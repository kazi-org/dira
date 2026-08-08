// Package why builds the why-chain: the entry a human asked about, the intents
// it arises from, the alternatives it refused and the grounds for refusing them.
//
// # One producer, two renderers
//
// The chain is a structure first and text second. Build returns a *Chain;
// RenderText turns one into the output `dira why` prints, and E6 turns the same
// *Chain into HTML. That split is not tidiness — docs/design/DESIGN.md makes the
// web why-chain load-bearing by claiming it is "the same output dira why prints
// in a terminal", and a claim like that survives exactly as long as there is one
// producer behind both renderers. If E6 re-derives the chain from the ledger,
// the signature detail becomes a lookalike and the claim becomes marketing.
//
// Every field a renderer needs is therefore on the Chain, including the ones the
// terminal does not print (the query the human typed, an entry's private flag),
// and the structs carry JSON tags so a consumer outside this module can have the
// same chain without re-walking the ledger.
//
// # What this package will not do
//
// It never writes. dec-0004 makes execution status derived and never stored, and
// dira does not embed a kazi client, so a `realized_by` target is carried
// verbatim as the external URI it is — never resolved, never annotated with a
// convergence state dira did not derive. A chain that said "converged" here
// would be dira asserting something it did not check.
package why

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
)

// Source is the read surface a chain is built from. *index.Index satisfies it.
//
// It is declared here, in the consumer, and sized to the three questions a chain
// asks: what did the human mean, what does this entry say, and what points at
// it. Reads go through it and nowhere else, so the files-win property the index
// guarantees (Entry reads the file, never the cache) is inherited rather than
// re-argued.
type Source interface {
	// Resolve turns a term into entry ids, newest first. A term matching
	// nothing yields an empty slice and no error.
	Resolve(ctx context.Context, term string) ([]string, error)

	// Entry returns the whole entry, read from its file. An id that is not
	// in the ledger yields an error wrapping ledger.ErrNotFound.
	Entry(ctx context.Context, id string) (*ledger.Entry, error)

	// In returns the edges pointing at an id.
	In(ctx context.Context, id string) ([]index.Backlink, error)
}

// Resolution says whether the ledger being read could open a ref.
//
// There are two values and not the three docs/design/DESIGN.md names, on
// purpose. `withheld` and `orphan` are tier states: they distinguish a ref into
// a parent ledger this repo declares but cannot show from a ref into no ledger
// at all, and that distinction needs the parent-ledger map, which is E5 and is
// blocked on qst-0001. Until it exists, dira cannot tell the two apart, and a
// renderer that guessed would be asserting a resolution nobody derived. So E1
// says only what it knows.
type Resolution string

// The resolutions.
const (
	// Oriented means the ref named an entry in this ledger and it was read.
	Oriented Resolution = "oriented"

	// Unresolved means this ledger cannot open the ref: either a namespaced
	// ref (sire:int-0002) whose ledger E1 has no map for, or a bare id
	// naming an entry that is not here.
	Unresolved Resolution = "unresolved"
)

// A Node is one entry as it appears in a chain.
//
// A node whose Resolution is Unresolved carries its Ref and nothing else: the
// point of the value is that dira could not read the entry, so every other field
// would be a guess.
type Node struct {
	// Ref is the entry reference as the ledger records it — a bare id
	// (dec-0002) or a namespaced one (sire:int-0002).
	Ref string `json:"ref"`

	Kind  ledger.Kind  `json:"kind,omitempty"`
	State ledger.State `json:"state,omitempty"`
	Title string       `json:"title,omitempty"`

	// Date is the entry's `updated` when it has one and its `created`
	// otherwise, as the RFC3339 string the file holds. A renderer decides
	// how much of it to show; this package does not shorten it, because a
	// date shortened here could not be lengthened by a renderer that wanted
	// the whole thing.
	Date string `json:"date,omitempty"`

	// Note is the note on the edge that put this node in the chain, if that
	// edge carried one. It is empty on the subject.
	Note string `json:"note,omitempty"`

	// Private is the entry's private flag, carried so a renderer that has to
	// honour cst-0003 can see it. E1 exports nothing and mirrors nothing, so
	// nothing in E1 acts on it; E3 and E6 do.
	Private bool `json:"private,omitempty"`

	Resolution Resolution `json:"resolution"`
}

// An Artifact is a `realized_by` target: an external execution artifact, carried
// verbatim.
//
// realized_by is the one edge type whose target is not a dira ref
// (entry.schema.json), and dira neither resolves it nor asks kazi about it
// (dec-0004). What a chain can honestly say is "this decision names that goal",
// and that is all this type holds.
type Artifact struct {
	Target string `json:"target"`
	Note   string `json:"note,omitempty"`
}

// A Relation is an edge the chain shows beside the subject rather than inside
// it: everything except the `derives_from` spine, which is Chain.Arising, and
// `realized_by`, which is Chain.Realized.
type Relation struct {
	Type ledger.EdgeType `json:"type"`

	// Incoming is true when the edge is declared by the other entry and
	// points at the subject. Edges live on the subject entry (dec-0002), so
	// an incoming edge is one nothing in the subject's own file records.
	Incoming bool `json:"incoming"`

	Node Node `json:"node"`
}

// A Chain is the whole answer to one `dira why`.
type Chain struct {
	// Query is what the human typed, verbatim. The terminal does not print
	// it — the invocation is already on the screen above the output — but a
	// rendered page has to show what produced it, which is the invocation
	// line in docs/design/screens/s1-decision.html.
	Query string `json:"query"`

	// Subject is the entry asked about.
	Subject Node `json:"subject"`

	// Arising is the `derives_from` ancestry, grouped by generation with the
	// outermost first. The subject is not in it; its generation is
	// len(Arising).
	//
	// Grouping by generation rather than nesting per branch is a deliberate
	// loss, and the only one in this structure. An entry with two parents in
	// different branches puts both at the same generation, so the chain
	// shows how far above the subject an ancestor sits but not which parent
	// it hangs from. A generation is the *longest* path from the subject, so
	// an ancestor reachable two ways appears once, at its deepest.
	Arising [][]Node `json:"arising,omitempty"`

	// Alternatives are the subject's roads not taken, in the order the entry
	// records them. Empty is a fact a renderer must state rather than skip:
	// "no alternatives" and "an empty section" read very differently to
	// someone deciding whether to relitigate.
	Alternatives []ledger.Alternative `json:"alternatives,omitempty"`

	// Realized are the `realized_by` targets, verbatim.
	Realized []Artifact `json:"realized_by,omitempty"`

	// ADR is the subject's `adr` field: a path to the mirrored ADR, which is
	// exhaust (dec-0009). The chain prints the path and never reads the file
	// — the entry is the record, and an ADR consulted as a source of truth
	// would invert the one-way authority dec-0009 turns on.
	ADR string `json:"adr,omitempty"`

	// SupersededBy holds the entries declaring a `supersedes` edge at the
	// subject. It is separated from Related because it changes how every
	// other line on the chain should be read.
	SupersededBy []Node `json:"superseded_by,omitempty"`

	// Related is every other edge touching the subject, in a deterministic
	// order, so two runs and two renderers agree.
	Related []Relation `json:"related,omitempty"`

	// Cycle names the refs where the ancestry walk found a `derives_from`
	// loop and stopped. A loop is a malformed ledger rather than a crash, so
	// it is reported and the rest of the chain still renders.
	Cycle []string `json:"cycle,omitempty"`
}

// Resolve turns what a human typed into the entries it names, newest first.
//
// It is separate from Build because the two outcomes it can have are different
// answers, not a success and a failure. One match is a chain. Several matches is
// a list of candidates — which is the whole reason this is not folded into
// Build: a resolver that silently picked one of five entries would answer a
// question the human did not ask, and would do it invisibly. No match is the
// caller's to call a failure, and `dira why` does.
func Resolve(ctx context.Context, src Source, query string) ([]Node, error) {
	ids, err := src.Resolve(ctx, query)
	if err != nil {
		return nil, err
	}
	nodes := make([]Node, 0, len(ids))
	for _, id := range ids {
		entry, err := src.Entry(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", id, err)
		}
		nodes = append(nodes, oriented(entry, ""))
	}
	return nodes, nil
}

// Build assembles the chain for one entry. query is what the human typed and is
// carried through for renderers; id is what it resolved to.
func Build(ctx context.Context, src Source, query, id string) (*Chain, error) {
	b := &builder{src: src, read: make(map[string]*ledger.Entry)}

	subject, err := b.entry(ctx, id)
	if err != nil {
		return nil, err
	}

	c := &Chain{
		Query:        query,
		Subject:      oriented(subject, ""),
		Alternatives: subject.Alternatives,
		ADR:          subject.ADR,
	}

	for _, edge := range subject.Edges {
		switch edge.Type {
		case ledger.EdgeRealizedBy:
			c.Realized = append(c.Realized, Artifact{Target: edge.To, Note: edge.Note})
		case ledger.EdgeDerivesFrom:
			// The spine, built below by the ancestry walk.
		default:
			node, err := b.node(ctx, edge.To, edge.Note)
			if err != nil {
				return nil, err
			}
			c.Related = append(c.Related, Relation{Type: edge.Type, Node: node})
		}
	}

	links, err := src.In(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		node, err := b.node(ctx, link.From, link.Note)
		if err != nil {
			return nil, err
		}
		if link.Type == ledger.EdgeSupersedes {
			c.SupersededBy = append(c.SupersededBy, node)
			continue
		}
		c.Related = append(c.Related, Relation{Type: link.Type, Incoming: true, Node: node})
	}

	sort.SliceStable(c.Related, func(i, j int) bool {
		gi, gj := group(c.Related[i]), group(c.Related[j])
		if gi != gj {
			return gi < gj
		}
		return c.Related[i].Node.Ref < c.Related[j].Node.Ref
	})

	if err := b.walk(ctx, subject, 0, map[string]bool{subject.ID: true}); err != nil {
		return nil, err
	}
	c.Arising = b.generations()
	c.Cycle = b.cycle

	return c, nil
}

// group is a relation's position in the fixed render order. It lives here
// rather than in the renderer because Related is sorted by it, and a JSON
// consumer that never calls RenderText still gets a stable order.
func group(r Relation) int {
	for i, g := range relationGroups {
		if g.Type == r.Type && g.Incoming == r.Incoming {
			return i
		}
	}
	return len(relationGroups)
}

// relationGroups is the order edges render in: what this entry says about
// others first, then what others say about it.
var relationGroups = []struct {
	Type     ledger.EdgeType
	Incoming bool
}{
	{ledger.EdgeSupersedes, false},
	{ledger.EdgeInforms, false},
	{ledger.EdgeBlocks, false},
	{ledger.EdgeDerivesFrom, true},
	{ledger.EdgeInforms, true},
	{ledger.EdgeBlocks, true},
}

// builder carries the per-call state of a chain build: the entries already read,
// the ancestry depths found so far, and any cycle the walk hit.
type builder struct {
	src   Source
	read  map[string]*ledger.Entry
	depth map[string]int
	nodes map[string]Node
	cycle []string
}

// entry reads an entry once per build. A chain reads the same ancestor several
// times over in a diamond, and int-0002's budget is not large enough to pay for
// the same file twice.
func (b *builder) entry(ctx context.Context, id string) (*ledger.Entry, error) {
	if e, ok := b.read[id]; ok {
		return e, nil
	}
	e, err := b.src.Entry(ctx, id)
	if err != nil {
		return nil, err
	}
	b.read[id] = e
	return e, nil
}

// node resolves a ref to a Node, degrading to Unresolved rather than failing.
//
// A ref dira cannot open is a fact about the ledger, not an error in the
// command: an entry pointing at a parent in a ledger this repo has no map for
// (E5) or at an id that was deleted must still render the rest of its chain.
func (b *builder) node(ctx context.Context, ref, note string) (Node, error) {
	if !ledger.ValidID(ref) {
		// A namespaced ref. Resolving it needs the parent-ledger map,
		// which is E5. Probing the store for a file named `sire:int-0002`
		// would be a guess about the storage layout on top of that.
		return Node{Ref: ref, Note: note, Resolution: Unresolved}, nil
	}
	entry, err := b.entry(ctx, ref)
	if err != nil {
		if errors.Is(err, ledger.ErrNotFound) {
			return Node{Ref: ref, Note: note, Resolution: Unresolved}, nil
		}
		return Node{}, fmt.Errorf("reading %s: %w", ref, err)
	}
	return oriented(entry, note), nil
}

// walk records, for every ancestor reachable from entry through `derives_from`,
// the longest distance from the subject.
//
// It terminates on every ledger, including a malformed one. path holds the refs
// on the current descent, so a loop is cut where it closes and recorded rather
// than followed. A ref is re-walked only when it is found at a strictly greater
// depth, and depth is bounded by the number of entries in the ledger, so the
// walk is bounded too.
func (b *builder) walk(ctx context.Context, entry *ledger.Entry, depth int, path map[string]bool) error {
	for _, edge := range entry.Edges {
		if edge.Type != ledger.EdgeDerivesFrom {
			continue
		}
		ref := edge.To
		if path[ref] {
			b.recordCycle(ref)
			continue
		}

		node, err := b.node(ctx, ref, edge.Note)
		if err != nil {
			return err
		}
		if b.depth == nil {
			b.depth = make(map[string]int)
			b.nodes = make(map[string]Node)
		}
		known, seen := b.depth[ref]
		if seen && known >= depth+1 {
			continue
		}
		b.depth[ref] = depth + 1
		b.nodes[ref] = node

		if node.Resolution != Oriented {
			continue
		}
		parent := b.read[ref]
		path[ref] = true
		if err := b.walk(ctx, parent, depth+1, path); err != nil {
			return err
		}
		delete(path, ref)
	}
	return nil
}

func (b *builder) recordCycle(ref string) {
	for _, seen := range b.cycle {
		if seen == ref {
			return
		}
	}
	b.cycle = append(b.cycle, ref)
}

// generations groups the ancestry by distance from the subject, outermost
// first, each generation sorted by ref so the output is reproducible.
func (b *builder) generations() [][]Node {
	if len(b.depth) == 0 {
		return nil
	}
	deepest := 0
	for _, d := range b.depth {
		if d > deepest {
			deepest = d
		}
	}
	out := make([][]Node, deepest)
	for ref, d := range b.depth {
		// Generation 0 is the outermost, so the deepest ancestor comes
		// first and the subject's own parents come last.
		out[deepest-d] = append(out[deepest-d], b.nodes[ref])
	}
	for _, gen := range out {
		sort.Slice(gen, func(i, j int) bool { return gen[i].Ref < gen[j].Ref })
	}
	return out
}

// oriented builds the node for an entry dira read.
func oriented(e *ledger.Entry, note string) Node {
	date := e.Updated
	if date == "" {
		date = e.Created
	}
	return Node{
		Ref:        e.ID,
		Kind:       e.Kind,
		State:      e.State,
		Title:      e.Title,
		Date:       date,
		Note:       note,
		Private:    e.Private,
		Resolution: Oriented,
	}
}

// String renders a node the way an error message names it, which is the one
// place a Node becomes text outside the renderer.
func (n Node) String() string {
	if n.Title == "" {
		return n.Ref
	}
	return n.Ref + " " + strings.TrimSpace(n.Title)
}
