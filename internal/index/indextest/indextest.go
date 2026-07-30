// Package indextest is the read path's differential harness: every query dira
// can answer, run twice — once against a warm cache and once with .dira/cache/
// deleted immediately beforehand — asserting the two runs are byte-identical.
//
// This is E1-L3's strongest predicate, and it is a package rather than a test
// function because it is not finished. E1-L4 (`dira why`) and E1-L5 (`dira
// brief`) each add their real rendered output to Queries by calling RunTwice
// with their own, and the guarantee they get for free is the one dec-0002
// actually promises: that deleting the derived cache changes how long an answer
// takes and nothing else about it.
//
// A query here renders to text on purpose. Comparing structs would compare what
// the code chose to expose; comparing rendered bytes compares what a human would
// have read, which is where a stale cache does its damage.
package indextest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// A Query is one read-path question, rendered to deterministic text.
type Query struct {
	Name string
	Run  func(ctx context.Context, ix *index.Index) (string, error)
}

// RunTwice runs every query against the ledger in diraDir twice and fails on any
// difference between the runs.
//
// The warm run opens the index once and asks everything, which is the state a
// second dira invocation in a session finds. The cold run deletes .dira/cache/
// before *each* query and opens a fresh index, which is the state the first
// invocation after a clone finds — and, because the deletion is per query rather
// than per run, a query that only works because an earlier one warmed something
// is caught too.
//
// extra is appended to Queries, so a later lane adds its own rendered output to
// the same guarantee instead of re-deriving the harness.
func RunTwice(t *testing.T, diraDir string, extra ...Query) {
	t.Helper()

	queries := append(Queries(), extra...)
	if len(queries) == 0 {
		t.Fatal("no queries; the harness would pass without comparing anything")
	}

	ctx := context.Background()
	cacheDir := local.CacheDir(diraDir)

	warm := make(map[string]string, len(queries))
	func() {
		ix := open(t, diraDir)
		defer func() { _ = ix.Close() }()
		if !ix.Stats().Cached {
			t.Fatalf("the warm run is not using a cache (%s); the comparison would be cache-absent against cache-absent", ix.Notice())
		}
		for _, q := range queries {
			out, err := q.Run(ctx, ix)
			if err != nil {
				t.Fatalf("warm %s: %v", q.Name, err)
			}
			warm[q.Name] = out
		}
	}()

	for _, q := range queries {
		if err := os.RemoveAll(cacheDir); err != nil {
			t.Fatalf("removing %s: %v", cacheDir, err)
		}
		func() {
			ix := open(t, diraDir)
			defer func() { _ = ix.Close() }()
			if got := ix.Stats().Indexed; got == 0 {
				t.Fatalf("cold %s: the index read 0 entry files after the cache was deleted; "+
					"either the cache survived deletion or the ledger is empty", q.Name)
			}
			out, err := q.Run(ctx, ix)
			if err != nil {
				t.Fatalf("cold %s: %v", q.Name, err)
			}
			if out != warm[q.Name] {
				t.Errorf("%s differs between a warm cache and no cache at all.\n"+
					"dec-0002 makes the cache derived and disposable, so deleting it may change how long an "+
					"answer takes and nothing else about it.\n--- warm ---\n%s\n--- cold ---\n%s",
					q.Name, warm[q.Name], out)
			}
		}()
	}
}

func open(t *testing.T, diraDir string) *index.Index {
	t.Helper()

	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("opening the ledger at %s: %v", diraDir, err)
	}
	ix, err := index.Open(context.Background(), store, local.CacheDir(diraDir))
	if err != nil {
		t.Fatalf("opening the index: %v", err)
	}
	return ix
}

// Queries is the read-path coverage E1-L3 owns: every method of the query API,
// exercised in the shapes E1-L4 and E1-L5 will use them in.
//
// The selections are cst-0001's three brief sections as pinned by E1's lane file
// — open blockers are open questions carrying a blocks edge, current focus is
// active intents, recent decisions are accepted decisions newest first — so this
// harness is already asking the questions the brief will ask.
func Queries() []Query {
	return []Query{
		{"open blockers", selectQuery(index.Selector{
			Kinds:    []ledger.Kind{ledger.KindQuestion},
			States:   []ledger.State{ledger.StateOpen},
			WithEdge: ledger.EdgeBlocks,
		})},
		{"current focus", selectQuery(index.Selector{
			Kinds:  []ledger.Kind{ledger.KindIntent},
			States: []ledger.State{ledger.StateActive},
		})},
		{"recent decisions", selectQuery(index.Selector{
			Kinds:  []ledger.Kind{ledger.KindDecision},
			States: []ledger.State{ledger.StateAccepted},
			Limit:  20,
		})},
		{"live constraints", selectQuery(index.Selector{
			Kinds:  []ledger.Kind{ledger.KindConstraint},
			States: []ledger.State{ledger.StateActive},
		})},
		{"superseded decisions", selectQuery(index.Selector{
			Kinds:  []ledger.Kind{ledger.KindDecision},
			States: []ledger.State{ledger.StateSuperseded},
		})},
		{"everything", selectQuery(index.Selector{})},

		{"resolve a term", func(ctx context.Context, ix *index.Index) (string, error) {
			return resolved(ctx, ix, "cache")
		}},
		{"resolve a tag", func(ctx context.Context, ix *index.Index) (string, error) {
			return resolved(ctx, ix, "latency")
		}},
		{"resolve an id", func(ctx context.Context, ix *index.Index) (string, error) {
			return resolved(ctx, ix, "dec-0001")
		}},
		{"resolve nothing", func(ctx context.Context, ix *index.Index) (string, error) {
			return resolved(ctx, ix, "no-such-term-anywhere")
		}},

		{"backlinks", func(ctx context.Context, ix *index.Index) (string, error) {
			refs, err := ix.Select(ctx, index.Selector{})
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for _, ref := range refs {
				links, err := ix.In(ctx, ref.ID)
				if err != nil {
					return "", err
				}
				if len(links) == 0 {
					continue
				}
				fmt.Fprintf(&b, "%s\n", ref.ID)
				for _, l := range links {
					fmt.Fprintf(&b, "\t<- %s %s %q\n", l.From, l.Type, l.Note)
				}
			}
			return b.String(), nil
		}},

		{"why chains", func(ctx context.Context, ix *index.Index) (string, error) {
			// The shape E1-L4 walks: a decision, its alternatives
			// with their why_nots, the titles of what it derives
			// from, and what points back at it. Rendered from the
			// files, which is the whole point.
			refs, err := ix.Select(ctx, index.Selector{
				Kinds: []ledger.Kind{ledger.KindDecision},
				Limit: 25,
			})
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for _, ref := range refs {
				chain, err := renderChain(ctx, ix, ref.ID)
				if err != nil {
					return "", err
				}
				b.WriteString(chain)
			}
			return b.String(), nil
		}},

		{"whole ledger rendered", func(ctx context.Context, ix *index.Index) (string, error) {
			refs, err := ix.Select(ctx, index.Selector{})
			if err != nil {
				return "", err
			}
			ids := make([]string, len(refs))
			for i, ref := range refs {
				ids[i] = ref.ID
			}
			entries, err := ix.Entries(ctx, ids)
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for _, e := range entries {
				b.WriteString(renderEntry(e))
			}
			return b.String(), nil
		}},
	}
}

func selectQuery(sel index.Selector) func(context.Context, *index.Index) (string, error) {
	return func(ctx context.Context, ix *index.Index) (string, error) {
		refs, err := ix.Select(ctx, sel)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		for _, ref := range refs {
			fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\t%s\tprivate=%v\n",
				ref.ID, ref.Kind, ref.State, ref.Created, ref.Updated, ref.Title, ref.Private)
		}
		return b.String(), nil
	}
}

func resolved(ctx context.Context, ix *index.Index, term string) (string, error) {
	ids, err := ix.Resolve(ctx, term)
	if err != nil {
		return "", err
	}
	return term + " -> " + strings.Join(ids, " ") + "\n", nil
}

// renderChain renders an entry with its parents' titles and its backlinks, which
// is the shape of `dira why`.
func renderChain(ctx context.Context, ix *index.Index, id string) (string, error) {
	entry, err := ix.Entry(ctx, id)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(renderEntry(entry))
	for _, edge := range entry.Edges {
		if edge.Type != ledger.EdgeDerivesFrom {
			continue
		}
		parent, err := ix.Entry(ctx, edge.To)
		if err != nil {
			fmt.Fprintf(&b, "\t^ %s (unresolved)\n", edge.To)
			continue
		}
		fmt.Fprintf(&b, "\t^ %s %s\n", parent.ID, parent.Title)
	}
	links, err := ix.In(ctx, id)
	if err != nil {
		return "", err
	}
	for _, l := range links {
		fmt.Fprintf(&b, "\tv %s %s\n", l.From, l.Type)
	}
	return b.String(), nil
}

// renderEntry writes every field a reader would see, so a difference anywhere in
// an entry shows up as a difference in the comparison.
func renderEntry(e *ledger.Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "== %s [%s/%s] %s\n", e.ID, e.Kind, e.State, e.Title)
	fmt.Fprintf(&b, "   created=%s updated=%s private=%v adr=%s confirmed_by=%s\n",
		e.Created, e.Updated, e.Private, e.ADR, e.ConfirmedBy)

	tags := append([]string(nil), e.Tags...)
	sort.Strings(tags)
	fmt.Fprintf(&b, "   tags=%s\n", strings.Join(tags, ","))

	for _, edge := range e.Edges {
		fmt.Fprintf(&b, "   -> %s %s %q\n", edge.Type, edge.To, edge.Note)
	}
	for _, alt := range e.Alternatives {
		fmt.Fprintf(&b, "   x %s\n     why_not: %s\n     revisit_if: %s\n", alt.Option, alt.WhyNot, alt.RevisitIf)
	}
	if e.Source != nil {
		fmt.Fprintf(&b, "   src hook=%s tier=%s session=%s excerpt=%q\n",
			e.Source.Hook, e.Source.Tier, e.Source.Session, e.Source.Excerpt)
	}
	fmt.Fprintf(&b, "   body=%q\n", e.Body)
	return b.String()
}

// Materialise writes n entries of the shared fixture into a fresh ledger and
// returns its .dira directory.
//
// Every test in this lane needs the same ledger on disk, and every one of them
// would otherwise build it slightly differently.
func Materialise(t *testing.T, entries []*ledger.Entry) string {
	t.Helper()

	diraDir := filepath.Join(t.TempDir(), ".dira")
	if err := os.MkdirAll(filepath.Join(diraDir, "entries"), 0o755); err != nil {
		t.Fatalf("creating %s: %v", diraDir, err)
	}
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("opening %s: %v", diraDir, err)
	}
	for _, e := range entries {
		if err := store.Create(context.Background(), e); err != nil {
			t.Fatalf("writing %s: %v", e.ID, err)
		}
	}
	return diraDir
}
