package status_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/index/indextest"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/status"
)

// TestDecisionBlocked is E4-L2-T4's acc line, the load-bearing row.
func TestDecisionBlocked(t *testing.T) {
	ctx := context.Background()

	t.Run("real-snapshot's open block names the question", func(t *testing.T) {
		ix := openFixtureIndex(t, "real-snapshot")
		rows, err := status.DeriveDecisionBlocked(ctx, ix)
		if err != nil {
			t.Fatalf("DeriveDecisionBlocked: %v", err)
		}
		row := rowFor(rows, "dec-0008")
		if row == nil {
			t.Fatal("dec-0008 is gated by qst-0005 (open, blocks -> dec-0008) and must appear")
		}
		if row.Bucket != status.DecisionBlocked || row.Source != status.SourceLedger {
			t.Errorf("dec-0008: Bucket=%q Source=%q, want %q/%q", row.Bucket, row.Source, status.DecisionBlocked, status.SourceLedger)
		}
		if row.BlockedBy == nil || row.BlockedBy.ID != "qst-0005" {
			t.Fatalf("dec-0008: BlockedBy = %+v, want qst-0005", row.BlockedBy)
		}
		// The title is read from qst-0005's own entry, not hardcoded here.
		wantTitle := entryTitle(t, ix, "qst-0005")
		if row.BlockedBy.Title != wantTitle {
			t.Errorf("dec-0008: BlockedBy.Title = %q, want %q (qst-0005's real title)", row.BlockedBy.Title, wantTitle)
		}
	})

	t.Run("answered question blocks nothing, until it is open again", func(t *testing.T) {
		diraDir := copyFixture(t, "answered-question")
		qstPath := filepath.Join(diraDir, "entries", "qst-1001.md")

		decBefore := decodeFile(t, filepath.Join(diraDir, "entries", "dec-1001.md"))
		qstBefore := decodeFile(t, qstPath)

		ix := openLedgerDir(t, diraDir)
		rows, err := status.DeriveDecisionBlocked(ctx, ix)
		if err != nil {
			t.Fatalf("DeriveDecisionBlocked: %v", err)
		}
		if got := rowFor(rows, "dec-1001"); got != nil {
			t.Fatalf("qst-1001 is answered; dec-1001 must not appear, got %+v", got)
		}

		setState(t, qstPath, "answered", "open")

		ix2 := openLedgerDir(t, diraDir)
		rows2, err := status.DeriveDecisionBlocked(ctx, ix2)
		if err != nil {
			t.Fatalf("DeriveDecisionBlocked (reopened): %v", err)
		}
		row := rowFor(rows2, "dec-1001")
		if row == nil {
			t.Fatal("qst-1001 is now open; dec-1001 must reappear")
		}
		// No other diff: the row's own fields trace back to the entries as
		// they stood before the state flip (Kind and Title untouched by it),
		// and BlockedBy names exactly the question that changed.
		if row.Kind != decBefore.Kind || row.Title != decBefore.Title {
			t.Errorf("dec-1001's row changed beyond the flip: Kind=%q Title=%q, want %q/%q",
				row.Kind, row.Title, decBefore.Kind, decBefore.Title)
		}
		if row.BlockedBy == nil || row.BlockedBy.ID != qstBefore.ID || row.BlockedBy.Title != qstBefore.Title {
			t.Errorf("BlockedBy = %+v, want {%s %s}", row.BlockedBy, qstBefore.ID, qstBefore.Title)
		}
	})

	t.Run("superseded target is never blocked", func(t *testing.T) {
		ix := openFixtureIndex(t, "superseded-target")
		rows, err := status.DeriveDecisionBlocked(ctx, ix)
		if err != nil {
			t.Fatalf("DeriveDecisionBlocked: %v", err)
		}
		if got := rowFor(rows, "dec-1002"); got != nil {
			t.Fatalf("dec-1002 is superseded; it must not appear even though qst-1002 is open, got %+v", got)
		}
	})

	t.Run("blocked by two open questions produces two rows", func(t *testing.T) {
		diraDir := indextest.Materialise(t, twoBlockersFixture())
		ix := openLedgerDir(t, diraDir)
		rows, err := status.DeriveDecisionBlocked(ctx, ix)
		if err != nil {
			t.Fatalf("DeriveDecisionBlocked: %v", err)
		}
		var matched []status.Row
		for _, r := range rows {
			if r.ID == "dec-2001" {
				matched = append(matched, r)
			}
		}
		if len(matched) != 2 {
			t.Fatalf("dec-2001 is gated by two open questions; got %d rows, want 2: %+v", len(matched), matched)
		}
		seen := map[string]bool{}
		for _, r := range matched {
			if r.BlockedBy == nil {
				t.Fatalf("row %+v has no BlockedBy", r)
			}
			seen[r.BlockedBy.ID] = true
		}
		if !seen["qst-2001"] || !seen["qst-2002"] {
			t.Errorf("expected rows naming both qst-2001 and qst-2002, got %v", seen)
		}
	})

	t.Run("naive resolver control", func(t *testing.T) {
		// The naive-implementation control run inline, over every fixture in
		// the corpus, not just the one with a blocking edge: it must find
		// zero rows on the fixtures with no live block too, for the boring
		// reason (no live block) — which is what distinguishes "the naive
		// resolver is wrong" from "the naive resolver happens to agree by
		// accident on the trivial fixtures."
		for _, name := range fixtureNames {
			ix := openFixtureIndex(t, name)
			naive, err := naiveDecisionBlockedByOwnEdges(ctx, ix)
			if err != nil {
				t.Fatalf("%s: naiveDecisionBlockedByOwnEdges: %v", name, err)
			}
			if len(naive) != 0 {
				t.Errorf("%s: naive resolver (own edges only) found %d rows, want 0: %+v", name, len(naive), naive)
			}
		}

		// Contrasted with the real implementation, which finds at least one
		// on real-snapshot (qst-0005 -> dec-0008).
		ix := openFixtureIndex(t, "real-snapshot")
		real, err := status.DeriveDecisionBlocked(ctx, ix)
		if err != nil {
			t.Fatalf("DeriveDecisionBlocked: %v", err)
		}
		if len(real) == 0 {
			t.Fatal("the real implementation found 0 rows on real-snapshot; the naive-vs-real contrast proves nothing without this")
		}
	})
}

// naiveDecisionBlockedByOwnEdges is the wrong implementation T4's acc line
// exists to rule out: for each decision — "inspects only dec-X itself", in
// the acc line's own words — it looks for a signal of being blocked using
// only that entry's own Edges, never ix.In. There is no such signal to find:
// edges are stored on the entry that DECLARES them (dec-0002), and a blocks
// edge is declared by the blocking question, not by the decision it gates —
// dec-X's own Edges describe what dec-X points at, never what points at it.
//
// The check below is a self-reference (edge.To == the candidate's own id)
// rather than "any outgoing blocks edge, regardless of target", because
// real-snapshot's dec-0008 happens to declare a genuine blocks edge of its
// own — dec-0008 -> qst-0005, a real and unrelated relationship recorded in
// this repository's ledger — and a direction-blind check would collide with
// it by accident, hiding the naive resolver's actual failure (checking the
// wrong entity's edges) behind a coincidence. A self-loop can never occur in
// a real entry, so this stays true to "inspects only its own edges, never
// ix.In" while never matching by chance.
func naiveDecisionBlockedByOwnEdges(ctx context.Context, ix *index.Index) ([]status.Row, error) {
	refs, err := ix.Select(ctx, index.Selector{Kinds: []ledger.Kind{ledger.KindDecision}})
	if err != nil {
		return nil, err
	}
	var out []status.Row
	for _, ref := range refs {
		entry, err := ix.Entry(ctx, ref.ID)
		if err != nil {
			return nil, err
		}
		for _, edge := range entry.Edges {
			if edge.Type == ledger.EdgeBlocks && edge.To == entry.ID {
				out = append(out, status.Row{ID: entry.ID, Kind: entry.Kind, Title: entry.Title, Bucket: status.DecisionBlocked, Source: status.SourceLedger})
			}
		}
	}
	return out, nil
}

// twoBlockersFixture is the fixture T4 adds beyond T2's named set: one
// decision gated by two independent open questions.
func twoBlockersFixture() []*ledger.Entry {
	return []*ledger.Entry{
		{
			ID: "dec-2001", Kind: ledger.KindDecision, Title: "Serialize the plugin manifest as TOML",
			State: ledger.StateAccepted, Created: "2026-08-01T13:00:00Z",
			Alternatives: []ledger.Alternative{{Option: "JSON", WhyNot: "fixture only"}},
		},
		{
			ID: "qst-2001", Kind: ledger.KindQuestion, Title: "Does the plugin loader support TOML yet?",
			State: ledger.StateOpen, Created: "2026-08-01T13:05:00Z",
			Edges: []ledger.Edge{{Type: ledger.EdgeBlocks, To: "dec-2001", Note: "fixture only"}},
		},
		{
			ID: "qst-2002", Kind: ledger.KindQuestion, Title: "Do third-party plugins already ship TOML manifests?",
			State: ledger.StateOpen, Created: "2026-08-01T13:10:00Z",
			Edges: []ledger.Edge{{Type: ledger.EdgeBlocks, To: "dec-2001", Note: "fixture only"}},
		},
	}
}

// entryTitle reads an entry's title through the index, so an assertion
// against it is never a hardcoded second copy of the fixture text.
func entryTitle(t *testing.T, ix *index.Index, id string) string {
	t.Helper()
	e, err := ix.Entry(context.Background(), id)
	if err != nil {
		t.Fatalf("reading %s: %v", id, err)
	}
	return e.Title
}

// decodeFile reads and decodes one entry file directly, for capturing a
// "before" snapshot ahead of a mutation.
func decodeFile(t *testing.T, path string) *ledger.Entry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	e, err := ledger.Decode(data)
	if err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	return e
}

// setState rewrites an entry file's frontmatter state: line in place,
// changing it from `from` to `to`. It fails loudly if the exact line it
// expects is not present, so a fixture edit elsewhere cannot silently turn
// this into a no-op mutation.
func setState(t *testing.T, path, from, to string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	want := "state: " + from
	replacement := "state: " + to
	if strings.Count(string(data), want) != 1 {
		t.Fatalf("%s: expected exactly one %q line, cannot mutate safely", path, want)
	}
	mutated := strings.Replace(string(data), want, replacement, 1)
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
