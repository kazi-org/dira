// Package fixture generates a deterministic ledger to measure against.
//
// Every later lane in E1 needs a ledger bigger than the 26 entries dira keeps
// about itself: E1-L3 rebuilds a cache over one, E1-L4 and E1-L5 take golden
// output from one, and E1-L6 holds a cold-start budget over one. Those lanes
// need the same ledger every time or their golden files are noise.
//
// # Generated, not committed
//
// The choice was 200 committed files against a seeded generator. This is the
// generator, for two reasons and against one real cost.
//
// The reasons: 200 fixture files add review noise to every future diff and to
// every grep of the repository — a maintainer looking for a real entry would
// wade through two hundred synthetic ones forever — and a committed fixture
// drifts, because nothing forces it to still be what the code that reads it
// expects. The cost: a generator can change output silently in a way a committed
// file cannot, which would move a golden file under E1-L4 without anyone
// deciding to.
//
// That cost is paid off by Digest and by TestDigestIsStable, which pins the
// generator's exact output to a constant in the test. Changing the generator
// fails that test, and updating the constant is a deliberate line in a diff —
// the same review signal a committed fixture gives, in one line instead of two
// hundred files.
package fixture

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
)

// Size is the entry count E1's acceptance lines are written against.
const Size = 200

// Seed is the seed those lanes generate with. Using a different one is fine for
// a test that only needs volume; using this one is what makes two lanes talk
// about the same ledger.
const Seed = 0x11A5D1A

// base is the instant the generated ledger starts at. Fixed, because "now"
// would make every golden file expire overnight.
var base = time.Date(2026, time.January, 5, 9, 0, 0, 0, time.UTC)

// mix is how many entries of each kind a ledger of Size holds, in the order they
// are generated. The order matters: an edge may only point at an entry that
// already exists, so intents come before what derives from them and decisions
// before the questions that block them.
//
// The proportions are meant to look like a real ledger rather than to be even.
// Decisions dominate because they are what accumulates; constraints are rare
// because a constraint is constitutional; notes are the pressure valve and there
// are plenty of them (cst-0002).
var mix = []struct {
	kind  ledger.Kind
	count int
}{
	{ledger.KindIntent, 16},
	{ledger.KindConstraint, 14},
	{ledger.KindDecision, 90},
	{ledger.KindQuestion, 30},
	{ledger.KindNote, 50},
}

// Generate returns n entries derived from seed. The same seed and n always
// produce the same entries, byte for byte once encoded.
//
// n is scaled against Size: asking for fewer entries yields the same ledger
// truncated proportionally across the five kinds, so a test that wants twenty
// entries still gets decisions with alternatives and questions that block them.
func Generate(seed uint64, n int) ([]*ledger.Entry, error) {
	if n <= 0 {
		return nil, fmt.Errorf("fixture size %d, want a positive count", n)
	}

	// Two independent streams from one seed: rand.NewPCG is deterministic
	// and the sequence is consumed in a fixed order, so nothing here depends
	// on map iteration, wall-clock time or goroutine scheduling.
	r := rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))

	g := &generator{rand: r, byKind: map[ledger.Kind][]string{}}
	for _, group := range mix {
		count := scale(group.count, n)
		for i := 1; i <= count; i++ {
			g.add(group.kind, i)
		}
	}
	g.applySupersessions()

	if err := g.validate(); err != nil {
		return nil, err
	}
	return g.entries, nil
}

// scale apportions a kind's share of n, keeping the total exactly n by giving
// the remainder to the largest group.
func scale(count, n int) int {
	if n == Size {
		return count
	}
	scaled := count * n / Size
	if scaled < 1 {
		scaled = 1
	}
	return scaled
}

// Write materialises entries into a store, one Create per entry.
//
// It uses Create rather than Put so a fixture written into a ledger that is not
// empty fails loudly instead of overwriting whatever was there.
func Write(ctx context.Context, s ledger.Store, entries []*ledger.Entry) error {
	for _, e := range entries {
		if err := s.Create(ctx, e); err != nil {
			return fmt.Errorf("writing %s: %w", e.ID, err)
		}
	}
	return nil
}

// Digest returns a hash over the encoded form of every entry, ordered by id. Two
// runs of the generator agree if and only if their digests agree, which is what
// makes "byte-identical across two runs" a single assertion rather than a
// file-by-file comparison.
func Digest(entries []*ledger.Entry) (string, error) {
	sorted := slices.Clone(entries)
	slices.SortFunc(sorted, func(a, b *ledger.Entry) int { return strings.Compare(a.ID, b.ID) })

	h := sha256.New()
	for _, e := range sorted {
		encoded, err := ledger.Encode(e)
		if err != nil {
			return "", fmt.Errorf("encoding %s: %w", e.ID, err)
		}
		// The id and length are hashed alongside the bytes so two
		// entries cannot swap content without changing the digest.
		// hash.Hash never returns an error, which is why these are
		// discarded rather than handled.
		_, _ = fmt.Fprintf(h, "%s\n%d\n", e.ID, len(encoded))
		_, _ = h.Write(encoded)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type generator struct {
	rand    *rand.Rand
	entries []*ledger.Entry

	// byKind indexes the ids generated so far, so an edge can pick a target
	// that exists. Slices, not a map iteration, because map order is random
	// and this has to be reproducible.
	byKind map[ledger.Kind][]string

	// adrs and privates count the two properties the acceptance line
	// requires at least one of, so validate can insist on them.
	adrs, privates int
}

func (g *generator) add(kind ledger.Kind, n int) {
	id := fmt.Sprintf("%s-%04d", kind.Prefix(), n)
	index := len(g.entries)

	e := &ledger.Entry{
		ID:      id,
		Kind:    kind,
		Title:   g.title(kind),
		Created: g.timestamp(index),
		Tags:    g.tags(kind),
		Body:    g.body(kind),
	}
	e.State = g.state(kind)

	// A third of entries have been touched since capture.
	if g.rand.IntN(3) == 0 {
		e.Updated = g.timestamp(index + 40 + g.rand.IntN(400))
	}

	e.Edges = g.edges(kind, id)
	if kind == ledger.KindDecision {
		e.Alternatives = g.alternatives()
	}
	e.Source, e.ConfirmedBy = g.provenance(e.State)

	// The next two are placed by position rather than by a random draw, and
	// deliberately so. Both are properties a later lane asserts the presence
	// of — cst-0003's never-export rule (E3, E6) and dec-0009's ADR pointer
	// (E1-L4) — and a 1-in-23 chance produces a fixture with none of them at
	// small sizes, which is a test that silently stops testing. A stride
	// guarantees at least one at every size the generator accepts.
	if index%23 == 2 {
		e.Private = true
		g.privates++
	}
	if kind == ledger.KindDecision && n%11 == 1 {
		e.ADR = fmt.Sprintf("docs/adr/%04d-%s.md", index, g.slug())
		g.adrs++
	}

	g.entries = append(g.entries, e)
	g.byKind[kind] = append(g.byKind[kind], id)
}

// applySupersessions flips the state of every entry that something supersedes.
// A ledger where dec-0004 supersedes dec-0002 but dec-0002 still reads
// `accepted` is not a ledger any reader would produce, and E1-L4's why-chain
// walks exactly these pairs.
func (g *generator) applySupersessions() {
	byID := make(map[string]*ledger.Entry, len(g.entries))
	for _, e := range g.entries {
		byID[e.ID] = e
	}
	for _, e := range g.entries {
		for _, edge := range e.Edges {
			if edge.Type != ledger.EdgeSupersedes {
				continue
			}
			target, ok := byID[edge.To]
			if !ok {
				continue
			}
			if slices.Contains(target.Kind.States(), ledger.StateSuperseded) {
				target.State = ledger.StateSuperseded
			}
		}
	}
}

// validate is the generator checking its own output, because a fixture that is
// subtly wrong is worse than no fixture: every lane measuring against it would
// be measuring the wrong thing, and the failure would surface as a puzzling
// result somewhere else.
func (g *generator) validate() error {
	ids := make(map[string]bool, len(g.entries))
	for _, e := range g.entries {
		if ids[e.ID] {
			return fmt.Errorf("duplicate id %s", e.ID)
		}
		ids[e.ID] = true
		if err := e.Validate(); err != nil {
			return fmt.Errorf("generated entry %s is invalid: %w", e.ID, err)
		}
	}
	for _, e := range g.entries {
		for _, edge := range e.Edges {
			if edge.Type == ledger.EdgeRealizedBy {
				continue
			}
			if !ids[edge.To] {
				return fmt.Errorf("%s has a %s edge to %s, which does not exist", e.ID, edge.Type, edge.To)
			}
		}
	}
	if g.privates == 0 {
		return fmt.Errorf("no entry is marked private; cst-0003's cases would go untested")
	}
	if g.adrs == 0 {
		return fmt.Errorf("no entry carries an adr path; E1-L4 reads that field")
	}
	return nil
}

// pick returns a deterministic element of a slice.
func pick[T any](r *rand.Rand, items []T) T {
	return items[r.IntN(len(items))]
}

func (g *generator) state(kind ledger.Kind) ledger.State {
	states := kind.States()
	// Weighted towards the live states: a ledger of mostly abandoned
	// intents and rejected decisions would not exercise the read path a
	// brief takes.
	if g.rand.IntN(4) > 0 {
		return states[0]
	}
	return pick(g.rand, states)
}

func (g *generator) timestamp(step int) string {
	// Deterministic and strictly derived from position, so an entry's
	// created time does not depend on how many random draws preceded it.
	return base.Add(time.Duration(step) * 97 * time.Minute).Format(time.RFC3339)
}

func (g *generator) title(kind ledger.Kind) string {
	switch kind {
	case ledger.KindIntent:
		return fmt.Sprintf("%s %s so %s", pick(g.rand, intentVerbs), pick(g.rand, subjects), pick(g.rand, outcomes))
	case ledger.KindDecision:
		return fmt.Sprintf("%s %s over %s", pick(g.rand, decisionVerbs), pick(g.rand, subjects), pick(g.rand, subjects))
	case ledger.KindQuestion:
		return fmt.Sprintf("%s %s before %s", pick(g.rand, questionOpeners), pick(g.rand, subjects), pick(g.rand, outcomes))
	case ledger.KindConstraint:
		return fmt.Sprintf("%s never %s", pick(g.rand, subjects), pick(g.rand, prohibitions))
	default:
		return fmt.Sprintf("%s about %s", pick(g.rand, noteOpeners), pick(g.rand, subjects))
	}
}

func (g *generator) slug() string {
	words := strings.Fields(strings.ToLower(pick(g.rand, subjects)))
	return strings.Join(words, "-")
}

func (g *generator) tags(kind ledger.Kind) []string {
	count := g.rand.IntN(4)
	if count == 0 {
		return nil
	}
	var tags []string
	for range count {
		tag := pick(g.rand, tagWords)
		if !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	return tags
}

func (g *generator) body(kind ledger.Kind) string {
	var b strings.Builder
	b.WriteString("\n")
	for i := range 1 + g.rand.IntN(3) {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(g.sentence())
		b.WriteString("\n")
	}
	return b.String()
}

func (g *generator) sentence() string {
	var parts []string
	for range 2 + g.rand.IntN(3) {
		parts = append(parts, pick(g.rand, clauses))
	}
	return strings.Join(parts, ", ") + "."
}

// edges builds a realistic edge set. Every type in the vocabulary occurs, and
// the shapes match what the read verbs will look for: a question that blocks a
// decision, a decision that derives from an intent, a decision realized by a
// kazi goal.
func (g *generator) edges(kind ledger.Kind, id string) []ledger.Edge {
	var edges []ledger.Edge

	addTo := func(edgeType ledger.EdgeType, targetKind ledger.Kind, note string) {
		candidates := g.byKind[targetKind]
		if len(candidates) == 0 {
			return
		}
		target := pick(g.rand, candidates)
		if target == id {
			return
		}
		edges = append(edges, ledger.Edge{Type: edgeType, To: target, Note: note})
	}

	switch kind {
	case ledger.KindIntent:
		if g.rand.IntN(3) == 0 {
			addTo(ledger.EdgeInforms, ledger.KindIntent, g.edgeNote())
		}

	case ledger.KindConstraint:
		// A constraint with no parent intent is a rule nobody can trace,
		// so most of them carry one.
		if g.rand.IntN(5) > 0 {
			addTo(ledger.EdgeDerivesFrom, ledger.KindIntent, g.edgeNote())
		}

	case ledger.KindDecision:
		addTo(ledger.EdgeDerivesFrom, ledger.KindIntent, g.edgeNote())
		if g.rand.IntN(4) == 0 {
			addTo(ledger.EdgeInforms, ledger.KindDecision, g.edgeNote())
		}
		if g.rand.IntN(9) == 0 {
			addTo(ledger.EdgeSupersedes, ledger.KindDecision, g.edgeNote())
		}
		if g.rand.IntN(6) == 0 {
			edges = append(edges, ledger.Edge{
				Type: ledger.EdgeRealizedBy,
				To:   fmt.Sprintf("kazi:goal-%s", g.slug()),
				Note: "execution lives in kazi; dira records only that it was delegated",
			})
		}

	case ledger.KindQuestion:
		// An open question carrying a blocks edge is the blockage kazi
		// structurally cannot see, and the first thing a brief renders.
		if g.rand.IntN(2) == 0 {
			addTo(ledger.EdgeBlocks, ledger.KindDecision, g.edgeNote())
		}

	case ledger.KindNote:
		if g.rand.IntN(3) == 0 {
			addTo(ledger.EdgeInforms, ledger.KindDecision, g.edgeNote())
		}
	}
	return edges
}

func (g *generator) edgeNote() string {
	if g.rand.IntN(3) == 0 {
		return ""
	}
	return pick(g.rand, clauses)
}

func (g *generator) alternatives() []ledger.Alternative {
	// The schema requires at least one; two or three is what a real
	// decision carries, and E1-L4 renders all of them.
	alts := make([]ledger.Alternative, 0, 3)
	for range 1 + g.rand.IntN(3) {
		alt := ledger.Alternative{
			Option: fmt.Sprintf("%s %s", pick(g.rand, decisionVerbs), pick(g.rand, subjects)),
			WhyNot: g.sentence() + " " + g.sentence(),
		}
		if g.rand.IntN(3) == 0 {
			alt.RevisitIf = pick(g.rand, clauses)
		}
		alts = append(alts, alt)
	}
	return alts
}

func (g *generator) provenance(state ledger.State) (*ledger.Source, string) {
	if g.rand.IntN(7) == 0 {
		return nil, ""
	}

	source := &ledger.Source{Hook: pick(g.rand, ledger.Hooks), Tier: pick(g.rand, ledger.Tiers)}
	if source.Tier == ledger.TierRegex {
		// dec-0003: a regex has no business asserting rationale, so
		// anything it produced is staged and attributed to the agent.
		source.Excerpt = g.sentence()
		source.Session = fmt.Sprintf("session-%08x", g.rand.Uint32())
		return source, "agent:dira-sniff"
	}
	if state == ledger.StateStaged {
		return source, "agent:claude-code"
	}
	return source, "human"
}

// The vocabularies below exist to make 200 entries read like a ledger rather
// than like lorem ipsum: a brief rendered over this fixture has to be legible
// enough that a human reviewing E1-L5's output can tell correct from wrong.
var (
	intentVerbs     = []string{"Keep", "Make", "Hold", "Reduce", "Preserve", "Shorten", "Remove", "Guarantee"}
	decisionVerbs   = []string{"Choose", "Prefer", "Adopt", "Standardise on", "Vendor", "Split", "Merge", "Defer"}
	questionOpeners = []string{"Whether to commit to", "How far to take", "Who owns", "When to retire", "Whether anyone needs"}
	noteOpeners     = []string{"An observation", "Something a user said", "A half-formed thought", "A pattern noticed twice"}

	subjects = []string{
		"the capture hook", "the derived cache", "the brief renderer", "the query engine",
		"the storage backend", "the id allocator", "the token counter", "the config loader",
		"the migration path", "the export format", "the ledger map", "the drift check",
		"the session transcript", "the edge index", "the release pipeline", "the install script",
	}
	outcomes = []string{
		"a cold start stays under budget", "the record survives the tool",
		"a fresh session starts oriented", "nothing has to be relitigated",
		"the phone is a first-class writer", "an offline machine is a working machine",
		"a reviewer can read the diff", "the brief still fits on one screen",
	}
	prohibitions = []string{
		"reaches the network on the read path", "requires a running process",
		"writes outside the entry it was given", "grows past what fits in a brief",
		"depends on a schema the user cannot read", "leaves a partial file behind",
	}
	tagWords = []string{
		"capture", "storage", "retrieval", "latency", "privacy", "cli", "hooks",
		"schema", "cache", "sync", "founding", "dx", "surfaces", "scope",
	}
	clauses = []string{
		"the cheapest version of this is the one that ships",
		"a check nobody runs is a check that does not exist",
		"the failure mode here is silent, which is what makes it expensive",
		"this was decided once already and the reasons have not changed",
		"the constraint is latency, and every option has to be priced against it",
		"a record that needs the tool to be readable is not a record",
		"two sessions writing at once is the normal case, not the edge case",
		"the alternative was rejected for a reason that still holds",
		"whatever is not enforced by a test is enforced by memory, which is to say not at all",
		"the boundary has to be drawn before the second implementation, not after",
	}
)
