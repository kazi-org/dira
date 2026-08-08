package why_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
	"github.com/kazi-org/dira/internal/why"
)

// sourceOver materialises a ledger of literal entry files and returns the index
// over it, which is the Source a chain is built from.
//
// The real index rather than a stand-in, because half of what this package
// promises is inherited from it: Entry reads the file, Resolve's exact-id rule
// is what makes `dira why dec-0002` unambiguous, and a fake would let this
// package's tests agree with a version of those rules that does not ship.
func sourceOver(t *testing.T, files map[string]string) (*index.Index, string) {
	t.Helper()

	root := t.TempDir()
	diraDir := filepath.Join(root, ".dira")
	entries := filepath.Join(diraDir, "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatalf("creating %s: %v", entries, err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(entries, name), []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("opening the ledger: %v", err)
	}
	ix, err := index.Open(context.Background(), store, local.CacheDir(diraDir))
	if err != nil {
		t.Fatalf("opening the index: %v", err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	return ix, diraDir
}

// entryFile writes one entry's file text. Frontmatter is assembled rather than
// pasted so a case reads as the shape it is testing.
func entryFile(id, kind, state string, lines ...string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + id + "\n")
	b.WriteString("kind: " + kind + "\n")
	b.WriteString("title: entry " + id + "\n")
	b.WriteString("state: " + state + "\n")
	b.WriteString("created: \"2026-01-01T00:00:00Z\"\n")
	for _, line := range lines {
		b.WriteString(line + "\n")
	}
	b.WriteString("---\n\nBody of " + id + ".\n")
	return b.String()
}

func build(t *testing.T, src why.Source, id string) *why.Chain {
	t.Helper()

	chain, err := why.Build(context.Background(), src, id, id)
	if err != nil {
		t.Fatalf("Build(%s): %v", id, err)
	}
	return chain
}

func render(t *testing.T, c *why.Chain) string {
	t.Helper()

	var b strings.Builder
	if err := why.RenderText(&b, c, why.DefaultWidth); err != nil {
		t.Fatalf("RenderText: %v", err)
	}
	return b.String()
}

// TestTheADRPathIsPrintedAndTheFileIsNeverRead covers the acceptance line's
// `adr` clause, which this repository's own ledger cannot: nothing in .dira/
// sets the field, because no epic owns the mirror writer yet (E1's lane file
// flags it, and dec-0009 makes the ADR exhaust rather than a source).
func TestTheADRPathIsPrintedAndTheFileIsNeverRead(t *testing.T) {
	t.Parallel()

	const path = "docs/adr/0084-checkpoint-resume.md"
	src, diraDir := sourceOver(t, map[string]string{
		"dec-0100.md": entryFile("dec-0100", "decision", "accepted",
			"adr: "+path,
			"alternatives:",
			"  - option: not doing it",
			"    why_not: the run would not resume",
		),
	})

	chain := build(t, src, "dec-0100")
	if chain.ADR != path {
		t.Errorf("Chain.ADR = %q, want %q", chain.ADR, path)
	}
	out := render(t, chain)
	if !strings.Contains(out, path) {
		t.Errorf("the chain does not print the adr path:\n%s", out)
	}

	// dec-0009: the entry is the record and the ADR is exhaust, safe to
	// delete. A renderer that read it would invert that authority — and
	// would also break on the overwhelmingly common case, which is that the
	// path names a file that was never written.
	if _, err := os.Stat(filepath.Join(diraDir, "..", path)); !os.IsNotExist(err) {
		t.Fatalf("the test's own premise is wrong: %s exists", path)
	}
	if strings.Contains(out, "checkpoint") && !strings.Contains(out, path) {
		t.Error("the chain rendered ADR content rather than the ADR path")
	}
}

// TestRealizedByTargetsArePrintedVerbatimAndNothingIsClaimedAboutThem covers the
// acceptance line's realized_by clause, which this repository's ledger also
// cannot: it has no realized_by edge.
//
// The stronger half is the second one. README.md's worked example shows
// `realized_by kazi:prop-resume-8a1f → converged ✓`, and that arrow is E4's to
// draw, not E1's: dec-0004 makes status derived rather than stored and dira
// embeds no kazi client, so a chain that printed "converged" would be asserting
// something dira did not check.
func TestRealizedByTargetsArePrintedVerbatimAndNothingIsClaimedAboutThem(t *testing.T) {
	t.Parallel()

	const target = "kazi:prop-resume-8a1f"
	src, _ := sourceOver(t, map[string]string{
		"dec-0100.md": entryFile("dec-0100", "decision", "accepted",
			"edges:",
			"  - type: realized_by",
			"    to: "+target,
			"    note: three predicates",
			"alternatives:",
			"  - option: not doing it",
			"    why_not: the run would not resume",
		),
	})

	chain := build(t, src, "dec-0100")
	if len(chain.Realized) != 1 || chain.Realized[0].Target != target {
		t.Fatalf("Chain.Realized = %+v, want one artifact %q", chain.Realized, target)
	}

	out := render(t, chain)
	if !strings.Contains(out, target) {
		t.Errorf("the chain does not print the realized_by target verbatim:\n%s", out)
	}
	for _, claim := range []string{"converged", "in progress", "completed", "planned", "✓", "3/3"} {
		if strings.Contains(out, claim) {
			t.Errorf("the chain says %q about a kazi artifact it never asked about (dec-0004):\n%s", claim, out)
		}
	}
}

// TestADerivesFromCycleIsReportedRatherThanFollowed is the cycle-safety clause.
// A loop is a malformed ledger, and the failure mode a walk must not have is
// hanging: the test would time out rather than fail if it did.
func TestADerivesFromCycleIsReportedRatherThanFollowed(t *testing.T) {
	t.Parallel()

	derives := func(to string) []string {
		return []string{"edges:", "  - type: derives_from", "    to: " + to}
	}

	cases := []struct {
		name  string
		files map[string]string
		start string
	}{
		{
			name:  "an entry arising from itself",
			start: "int-0100",
			files: map[string]string{
				"int-0100.md": entryFile("int-0100", "intent", "active", derives("int-0100")...),
			},
		},
		{
			name:  "two entries arising from each other",
			start: "int-0100",
			files: map[string]string{
				"int-0100.md": entryFile("int-0100", "intent", "active", derives("int-0101")...),
				"int-0101.md": entryFile("int-0101", "intent", "active", derives("int-0100")...),
			},
		},
		{
			name:  "a three-entry loop entered from outside it",
			start: "int-0100",
			files: map[string]string{
				"int-0100.md": entryFile("int-0100", "intent", "active", derives("int-0101")...),
				"int-0101.md": entryFile("int-0101", "intent", "active", derives("int-0102")...),
				"int-0102.md": entryFile("int-0102", "intent", "active", derives("int-0103")...),
				"int-0103.md": entryFile("int-0103", "intent", "active", derives("int-0101")...),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			src, _ := sourceOver(t, tc.files)
			chain := build(t, src, tc.start)

			if len(chain.Cycle) == 0 {
				t.Fatalf("the walk terminated but reported no cycle; a loop dira followed silently is a loop nobody fixes")
			}
			out := render(t, chain)
			if !strings.Contains(out, "loops back to") {
				t.Errorf("the rendered chain does not say the ledger loops:\n%s", out)
			}
			for _, ref := range chain.Cycle {
				if !strings.Contains(out, ref) {
					t.Errorf("the cycle report does not name %s:\n%s", ref, out)
				}
			}
		})
	}
}

// TestTheWalkTerminatesOnAWideDiamond is the other half of cycle safety. A
// cycle guard that worked by refusing to visit any ref twice would pass every
// case above and silently place a shared ancestor at the wrong generation; one
// that re-walks without a bound would still terminate here but is worth
// measuring, because the ancestry of a real ledger is a DAG and not a chain.
func TestTheWalkTerminatesOnAWideDiamond(t *testing.T) {
	t.Parallel()

	// Twelve stacked lopsided diamonds. Each layer's base arises from two
	// entries and both routes reconverge on the layer's top, one route a
	// step longer than the other:
	//
	//	base ─┬─ short ──────────── top
	//	      └─ long1 ── long2 ─── top
	//
	// So 2^12 = 4096 distinct paths from the bottom to the apex over 37
	// entries, and every shared node is reachable at two different depths —
	// which is what makes "a generation is the longest path" a claim with
	// something at stake rather than a description of a straight line.
	const layers = 12
	files := map[string]string{}
	id := func(n int) string { return "int-" + pad(100+n) }

	base := 0
	for i := 0; i < layers; i++ {
		short, long1, long2, top := base+1, base+2, base+3, base+4
		files[id(base)+".md"] = entryFile(id(base), "intent", "active",
			"edges:",
			"  - type: derives_from",
			"    to: "+id(short),
			"  - type: derives_from",
			"    to: "+id(long1),
		)
		files[id(short)+".md"] = entryFile(id(short), "intent", "active",
			"edges:", "  - type: derives_from", "    to: "+id(top))
		files[id(long1)+".md"] = entryFile(id(long1), "intent", "active",
			"edges:", "  - type: derives_from", "    to: "+id(long2))
		files[id(long2)+".md"] = entryFile(id(long2), "intent", "active",
			"edges:", "  - type: derives_from", "    to: "+id(top))
		base = top
	}
	apex := id(base)
	files[apex+".md"] = entryFile(apex, "intent", "active")

	src, _ := sourceOver(t, files)
	chain := build(t, src, id(0))

	if len(chain.Cycle) != 0 {
		t.Errorf("a diamond was reported as a cycle: %v", chain.Cycle)
	}
	// Each layer's top sits three steps above its base along the long
	// route, so the apex is 3*layers generations up. A walk that took the
	// first depth it found would report 2*layers.
	if got, want := len(chain.Arising), layers*3; got != want {
		t.Errorf("the ancestry is %d generations deep, want %d — a shared ancestor was placed at its shallowest depth rather than its deepest",
			got, want)
	}
	if first := chain.Arising[0]; len(first) != 1 || first[0].Ref != apex {
		t.Errorf("the outermost generation is %+v, want just %s", first, apex)
	}
	// The bottom layer's two immediate parents share a generation, which is
	// the case the by-generation flattening exists to handle.
	if last := chain.Arising[len(chain.Arising)-1]; len(last) != 2 {
		t.Errorf("the generation nearest the subject holds %d entries, want its 2 parents: %+v", len(last), last)
	}
}

func pad(n int) string {
	s := ""
	for _, d := range []int{1000, 100, 10, 1} {
		s += string(rune('0' + (n/d)%10))
	}
	return s
}

// TestARefThisLedgerCannotOpenIsStatedRatherThanGuessedAt covers both shapes of
// unresolvable ref, neither of which this repository's ledger contains.
//
// It is deliberately not called "withheld" or "orphan". Those are the tier
// states in docs/design/DESIGN.md, and telling them apart needs the
// parent-ledger map, which is E5 and is blocked on qst-0001. What dira can say
// today is that it cannot open the ref, so that is what it says — and it says it
// in words rather than with an alarm, because of the three resolution states
// only orphan is drift (law 1).
func TestARefThisLedgerCannotOpenIsStatedRatherThanGuessedAt(t *testing.T) {
	t.Parallel()

	src, _ := sourceOver(t, map[string]string{
		"dec-0100.md": entryFile("dec-0100", "decision", "accepted",
			"edges:",
			"  - type: derives_from",
			"    to: sire:int-0002",
			"  - type: informs",
			"    to: dec-9999",
			"alternatives:",
			"  - option: not doing it",
			"    why_not: nothing would arise from anywhere",
		),
	})

	chain := build(t, src, "dec-0100")

	if len(chain.Arising) != 1 || len(chain.Arising[0]) != 1 {
		t.Fatalf("Chain.Arising = %+v, want one unresolvable parent", chain.Arising)
	}
	parent := chain.Arising[0][0]
	if parent.Ref != "sire:int-0002" || parent.Resolution != why.Unresolved {
		t.Errorf("parent = %+v, want sire:int-0002 unresolved", parent)
	}
	if parent.Title != "" || parent.State != "" {
		t.Errorf("parent carries %q/%q for an entry dira never read", parent.Title, parent.State)
	}

	out := render(t, chain)
	for _, want := range []string{"sire:int-0002", "dec-9999", "not in this ledger"} {
		if !strings.Contains(out, want) {
			t.Errorf("the chain does not mention %q:\n%s", want, out)
		}
	}
	for _, alarm := range []string{"error", "ERROR", "missing", "broken", "invalid"} {
		if strings.Contains(out, alarm) {
			t.Errorf("an unopenable ref reads as an alarm — it contains %q (DESIGN.md law 1):\n%s", alarm, out)
		}
	}
}

// TestASupersededEntryShowsWhatSupersededIt covers the acceptance line's
// superseded clause on an entry whose state was actually flipped, which this
// repository's ledger does not contain either: dec-0012 carries an incoming
// supersedes edge while its own state still reads `accepted`.
func TestASupersededEntryShowsWhatSupersededIt(t *testing.T) {
	t.Parallel()

	src, _ := sourceOver(t, map[string]string{
		"dec-0100.md": entryFile("dec-0100", "decision", "superseded",
			"alternatives:",
			"  - option: not doing it",
			"    why_not: the earlier reasoning, now replaced",
		),
		"dec-0101.md": entryFile("dec-0101", "decision", "accepted",
			"edges:",
			"  - type: supersedes",
			"    to: dec-0100",
			"    note: the constraint it turned on stopped applying",
			"alternatives:",
			"  - option: leaving dec-0100 in place",
			"    why_not: it is wrong now",
		),
	})

	chain := build(t, src, "dec-0100")
	if len(chain.SupersededBy) != 1 || chain.SupersededBy[0].Ref != "dec-0101" {
		t.Fatalf("Chain.SupersededBy = %+v, want dec-0101", chain.SupersededBy)
	}

	out := render(t, chain)
	for _, want := range []string{"superseded 2026-01-01", "superseded by", "dec-0101", "the constraint it turned on stopped applying"} {
		if !strings.Contains(out, want) {
			t.Errorf("the chain does not show %q:\n%s", want, out)
		}
	}
	// It must be in the chain, above the fold, not in the edges footer:
	// being superseded changes how every other line should be read.
	body, _, found := strings.Cut(out, "\nedges\n")
	if found && !strings.Contains(body, "superseded by") {
		t.Errorf("the supersession is only in the edges list, not in the chain:\n%s", out)
	}
}

// TestAnEntryWithNoParentsRendersWithoutAChain is the remaining empty case: an
// entry nothing arises from and which arises from nothing.
func TestAnEntryWithNoParentsRendersWithoutAChain(t *testing.T) {
	t.Parallel()

	src, _ := sourceOver(t, map[string]string{
		"note-0100.md": entryFile("note-0100", "note", "active"),
	})

	chain := build(t, src, "note-0100")
	if len(chain.Arising) != 0 {
		t.Errorf("Chain.Arising = %+v for an entry with no parents", chain.Arising)
	}
	if len(chain.Related) != 0 {
		t.Errorf("Chain.Related = %+v for an entry with no edges", chain.Related)
	}

	out := render(t, chain)
	// The subject is the first line and carries no branch glyph: there is
	// nothing above it for a branch to hang from.
	if first, _, _ := strings.Cut(out, "\n"); !strings.HasPrefix(first, "note-0100") {
		t.Errorf("the first line is %q; an entry with no parents should open with itself", first)
	}
	if !strings.Contains(out, "note-0100") || !strings.Contains(out, "no alternatives recorded") {
		t.Errorf("the entry did not render:\n%s", out)
	}
	if strings.Contains(out, "edges") {
		t.Errorf("an entry with no edges rendered an edges section:\n%s", out)
	}
}

// TestResolveListsEveryMatchAndBuildsNoneOfThem pins the disambiguation rule at
// the API, since the command's behaviour depends on it.
func TestResolveListsEveryMatchAndBuildsNoneOfThem(t *testing.T) {
	t.Parallel()

	src, _ := sourceOver(t, map[string]string{
		"int-0100.md": entryFile("int-0100", "intent", "active"),
		"int-0101.md": entryFile("int-0101", "intent", "active"),
		"dec-0100.md": entryFile("dec-0100", "decision", "accepted",
			"alternatives:", "  - option: no", "    why_not: because"),
	})
	ctx := context.Background()

	// "entry" is in every generated title.
	many, err := why.Resolve(ctx, src, "entry")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(many) != 3 {
		t.Fatalf("Resolve(%q) returned %d entries, want 3", "entry", len(many))
	}

	// An exact id resolves to itself alone even though its title also
	// matches the term above.
	one, err := why.Resolve(ctx, src, "int-0100")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(one) != 1 || one[0].Ref != "int-0100" {
		t.Fatalf("Resolve(%q) = %+v, want exactly int-0100", "int-0100", one)
	}

	none, err := why.Resolve(ctx, src, "nothing-matches-this")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("Resolve of an absent term returned %+v, want nothing", none)
	}

	var b strings.Builder
	if err := why.RenderCandidates(&b, "entry", many, why.DefaultWidth); err != nil {
		t.Fatalf("RenderCandidates: %v", err)
	}
	for _, want := range []string{"3 entries match", "int-0100", "int-0101", "dec-0100"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("the candidate list is missing %q:\n%s", want, b.String())
		}
	}
}

// TestTheStructuredChainStandsWithoutTheRenderer is the contract handed to E6.
//
// Everything the terminal draws has to be reachable from the *Chain without
// parsing text, or the second renderer ends up re-deriving from the ledger and
// the two drift — which is the risk this lane exists to close.
func TestTheStructuredChainStandsWithoutTheRenderer(t *testing.T) {
	t.Parallel()

	src, _ := sourceOver(t, map[string]string{
		"int-0100.md": entryFile("int-0100", "intent", "active"),
		"dec-0100.md": entryFile("dec-0100", "decision", "accepted",
			"adr: docs/adr/0001-x.md",
			"private: true",
			"edges:",
			"  - type: derives_from",
			"    to: int-0100",
			"    note: why it arises",
			"  - type: realized_by",
			"    to: kazi:goal-1",
			"  - type: informs",
			"    to: int-0100",
			"alternatives:",
			"  - option: the road not taken",
			"    why_not: it went nowhere",
			"    revisit_if: the terrain changes",
		),
	})

	chain, err := why.Build(context.Background(), src, "a term someone typed", "dec-0100")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	checks := []struct {
		what string
		ok   bool
	}{
		{"the query the human typed", chain.Query == "a term someone typed"},
		{"the subject's id", chain.Subject.Ref == "dec-0100"},
		{"the subject's kind", chain.Subject.Kind == ledger.KindDecision},
		{"the subject's state", chain.Subject.State == ledger.StateAccepted},
		{"the subject's date", chain.Subject.Date == "2026-01-01T00:00:00Z"},
		{"the subject's private flag", chain.Subject.Private},
		{"the subject's resolution", chain.Subject.Resolution == why.Oriented},
		{"one generation of ancestry", len(chain.Arising) == 1 && len(chain.Arising[0]) == 1},
		{"the parent's title", len(chain.Arising) == 1 && chain.Arising[0][0].Title == "entry int-0100"},
		{"the edge note that put the parent there", len(chain.Arising) == 1 && chain.Arising[0][0].Note == "why it arises"},
		{"the alternative", len(chain.Alternatives) == 1 && chain.Alternatives[0].Option == "the road not taken"},
		{"its why_not", len(chain.Alternatives) == 1 && chain.Alternatives[0].WhyNot == "it went nowhere"},
		{"its revisit_if", len(chain.Alternatives) == 1 && chain.Alternatives[0].RevisitIf == "the terrain changes"},
		{"the realized_by target", len(chain.Realized) == 1 && chain.Realized[0].Target == "kazi:goal-1"},
		{"the adr path", chain.ADR == "docs/adr/0001-x.md"},
		{"the informs edge", len(chain.Related) == 1 && chain.Related[0].Type == ledger.EdgeInforms && !chain.Related[0].Incoming},
		{"no cycle", len(chain.Cycle) == 0},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("the structured chain does not carry %s: %+v", c.what, chain)
		}
	}

	// derives_from is the spine and must not be duplicated into Related, or
	// a second renderer draws the same parent twice.
	for _, r := range chain.Related {
		if r.Type == ledger.EdgeDerivesFrom && !r.Incoming {
			t.Errorf("the derives_from spine is also in Related: %+v", r)
		}
		if r.Type == ledger.EdgeRealizedBy {
			t.Errorf("a realized_by target leaked into Related as though it were an entry: %+v", r)
		}
	}
}

// TestRelatedEdgesComeBackInAStableOrder stops the golden files and E6's
// rendering from depending on map iteration or on the order SQLite felt like.
func TestRelatedEdgesComeBackInAStableOrder(t *testing.T) {
	t.Parallel()

	src, _ := sourceOver(t, map[string]string{
		"dec-0100.md": entryFile("dec-0100", "decision", "accepted",
			"edges:",
			"  - type: blocks",
			"    to: dec-0103",
			"  - type: informs",
			"    to: dec-0102",
			"  - type: supersedes",
			"    to: dec-0101",
			"alternatives:", "  - option: no", "    why_not: because"),
		"dec-0101.md": entryFile("dec-0101", "decision", "superseded",
			"alternatives:", "  - option: no", "    why_not: because"),
		"dec-0102.md": entryFile("dec-0102", "decision", "accepted",
			"edges:", "  - type: informs", "    to: dec-0100",
			"alternatives:", "  - option: no", "    why_not: because"),
		"dec-0103.md": entryFile("dec-0103", "decision", "accepted",
			"edges:", "  - type: derives_from", "    to: dec-0100",
			"alternatives:", "  - option: no", "    why_not: because"),
	})

	want := []struct {
		ref      string
		typ      ledger.EdgeType
		incoming bool
	}{
		{"dec-0101", ledger.EdgeSupersedes, false},
		{"dec-0102", ledger.EdgeInforms, false},
		{"dec-0103", ledger.EdgeBlocks, false},
		{"dec-0103", ledger.EdgeDerivesFrom, true},
		{"dec-0102", ledger.EdgeInforms, true},
	}

	for run := 0; run < 3; run++ {
		chain := build(t, src, "dec-0100")
		if len(chain.Related) != len(want) {
			t.Fatalf("Chain.Related has %d entries, want %d: %+v", len(chain.Related), len(want), chain.Related)
		}
		for i, w := range want {
			got := chain.Related[i]
			if got.Node.Ref != w.ref || got.Type != w.typ || got.Incoming != w.incoming {
				t.Errorf("Related[%d] = %s %s incoming=%v, want %s %s incoming=%v",
					i, got.Node.Ref, got.Type, got.Incoming, w.ref, w.typ, w.incoming)
			}
		}
	}
}

// TestTheRendererIsDeterministic guards the golden files against anything in
// the build that is not.
func TestTheRendererIsDeterministic(t *testing.T) {
	t.Parallel()

	src, _ := sourceOver(t, map[string]string{
		"int-0100.md": entryFile("int-0100", "intent", "active"),
		"dec-0100.md": entryFile("dec-0100", "decision", "accepted",
			"edges:",
			"  - type: derives_from",
			"    to: int-0100",
			"alternatives:", "  - option: no", "    why_not: because"),
		"dec-0101.md": entryFile("dec-0101", "decision", "accepted",
			"edges:",
			"  - type: derives_from",
			"    to: int-0100",
			"alternatives:", "  - option: no", "    why_not: because"),
	})

	first := render(t, build(t, src, "int-0100"))
	for run := 0; run < 20; run++ {
		if got := render(t, build(t, src, "int-0100")); got != first {
			t.Fatalf("run %d rendered differently:\n--- first ---\n%s\n--- got ---\n%s", run, first, got)
		}
	}
}
