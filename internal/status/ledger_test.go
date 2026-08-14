package status_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/status"
)

// TestToBePlanned is E4-L2-T3's acc line.
func TestToBePlanned(t *testing.T) {
	t.Run("no-realized-by lands both entries", func(t *testing.T) {
		ix := openFixtureIndex(t, "no-realized-by")
		rows, err := status.DeriveToBePlanned(context.Background(), ix)
		if err != nil {
			t.Fatalf("DeriveToBePlanned: %v", err)
		}
		requireRow(t, rows, "dec-1003", status.ToBePlanned, status.SourceLedger)
		requireRow(t, rows, "int-1003", status.ToBePlanned, status.SourceLedger)
	})

	t.Run("real-snapshot carries no realized_by edge to exclude", func(t *testing.T) {
		// T3's acc line, as docs/plan/tasks/E4-L2.md fixes it, expects
		// real-snapshot to hold at least one entry carrying a realized_by
		// edge ("this project's own .dira/entries/dec-0060-style edges")
		// and asks that ToBePlanned's output be checked to exclude those
		// ids. That premise is false: grepping the real ledger this
		// fixture was copied from
		// (`grep -rl "type: realized_by" .dira/entries/*.md`) at copy
		// time found zero matches across all 43 entries, and there is no
		// dec-0060 in this repository's ledger at all — see
		// testdata/ledgers/README.md. This sub-test records that fact
		// directly, so the absence is asserted rather than assumed, and
		// the real exclusion BEHAVIOUR is proven instead by
		// TestLedgerRealizedByEdgeExcludesEntry below, against
		// no-realized-by mutated to carry the edge the real snapshot does
		// not have.
		entries := fixtureEntries(t, "real-snapshot")
		var withRealizedBy []string
		for _, e := range entries {
			if hasEdge(e, ledger.EdgeRealizedBy) {
				withRealizedBy = append(withRealizedBy, e.ID)
			}
		}
		if len(withRealizedBy) != 0 {
			t.Fatalf("real-snapshot now carries realized_by edges on %v; "+
				"the deviation note above and testdata/ledgers/README.md need updating, "+
				"and this sub-test should assert those ids are absent from DeriveToBePlanned's output", withRealizedBy)
		}

		ix := openFixtureIndex(t, "real-snapshot")
		rows, err := status.DeriveToBePlanned(context.Background(), ix)
		if err != nil {
			t.Fatalf("DeriveToBePlanned: %v", err)
		}
		for _, id := range withRealizedBy {
			if rowFor(rows, id) != nil {
				t.Errorf("%s carries a realized_by edge and must not appear in ToBePlanned", id)
			}
		}
	})

	t.Run("superseded decision is absent", func(t *testing.T) {
		ix := openFixtureIndex(t, "superseded-target")
		rows, err := status.DeriveToBePlanned(context.Background(), ix)
		if err != nil {
			t.Fatalf("DeriveToBePlanned: %v", err)
		}
		if rowFor(rows, "dec-1002") != nil {
			t.Error("dec-1002 is superseded and must not appear in ToBePlanned")
		}
	})

	t.Run("achieved intent is absent", func(t *testing.T) {
		ix := openFixtureIndex(t, "achieved-intent")
		rows, err := status.DeriveToBePlanned(context.Background(), ix)
		if err != nil {
			t.Fatalf("DeriveToBePlanned: %v", err)
		}
		if rowFor(rows, "int-1001") != nil {
			t.Error("int-1001 is achieved and must not appear in ToBePlanned")
		}
	})

	t.Run("abandoned intent is absent", func(t *testing.T) {
		ix := openFixtureIndex(t, "abandoned-intent")
		rows, err := status.DeriveToBePlanned(context.Background(), ix)
		if err != nil {
			t.Fatalf("DeriveToBePlanned: %v", err)
		}
		if rowFor(rows, "int-1002") != nil {
			t.Error("int-1002 is abandoned and must not appear in ToBePlanned")
		}
	})

	t.Run("order is total", func(t *testing.T) {
		ix := openFixtureIndex(t, "real-snapshot")
		ctx := context.Background()
		first, err := status.DeriveToBePlanned(ctx, ix)
		if err != nil {
			t.Fatalf("DeriveToBePlanned: %v", err)
		}
		second, err := status.DeriveToBePlanned(ctx, ix)
		if err != nil {
			t.Fatalf("DeriveToBePlanned: %v", err)
		}
		if len(first) == 0 {
			t.Fatal("real-snapshot produced zero ToBePlanned rows; the order check needs at least one")
		}
		if len(first) != len(second) {
			t.Fatalf("two runs produced %d and %d rows", len(first), len(second))
		}
		for i := range first {
			if first[i].ID != second[i].ID {
				t.Fatalf("row %d differs between runs: %q vs %q", i, first[i].ID, second[i].ID)
			}
		}
		for i := 1; i < len(first); i++ {
			if first[i-1].ID >= first[i].ID {
				t.Fatalf("rows are not ID-ascending at index %d: %q then %q", i, first[i-1].ID, first[i].ID)
			}
		}
	})
}

// TestLedgerRealizedByEdgeExcludesEntry is the "Both sides" proof T3's acc
// line requires: a mutated copy of no-realized-by's decision, with a
// realized_by edge added, must not appear in DeriveToBePlanned's result —
// and the check is shown able to fail first, against a naive resolver that
// omits the realized_by filter entirely.
//
// Named with the TestLedger prefix (rather than folded into TestToBePlanned)
// so that `go test -run 'TestLedger|TestDecisionBlocked'` — the epic-level
// E4-L2 acc line in docs/plan/lanes/E4.md, which T6 re-runs verbatim under an
// emptied PATH — actually selects a test exercising this file's derivation,
// rather than matching nothing and passing over an empty listing.
func TestLedgerRealizedByEdgeExcludesEntry(t *testing.T) {
	diraDir := copyFixture(t, "no-realized-by")
	addRealizedByEdge(t, filepath.Join(diraDir, "entries", "dec-1003.md"))
	ix := openLedgerDir(t, diraDir)
	ctx := context.Background()

	// Red: a naive resolver that reads only state and kind, never the
	// entry's own edges, would still count dec-1003 a candidate and never
	// exclude it — the exact bug T3's acc line exists to catch.
	naive, err := naiveToBePlannedIgnoringRealizedBy(ctx, ix)
	if err != nil {
		t.Fatalf("naiveToBePlannedIgnoringRealizedBy: %v", err)
	}
	if rowFor(naive, "dec-1003") == nil {
		t.Fatal("the naive resolver (no realized_by filter) does not include dec-1003 either; " +
			"this fixture no longer demonstrates what the real filter guards against")
	}

	// Green: the real derivation excludes it.
	rows, err := status.DeriveToBePlanned(ctx, ix)
	if err != nil {
		t.Fatalf("DeriveToBePlanned: %v", err)
	}
	if got := rowFor(rows, "dec-1003"); got != nil {
		t.Errorf("dec-1003 now carries a realized_by edge and must not appear in ToBePlanned, got %+v", got)
	}
	// The untouched sibling, int-1003, is unaffected.
	if rowFor(rows, "int-1003") == nil {
		t.Error("int-1003 carries no realized_by edge and should still appear in ToBePlanned")
	}
}

// naiveToBePlannedIgnoringRealizedBy is the wrong implementation the "both
// sides" proof above needs, defined only in this test file: it selects the
// same candidates as DeriveToBePlanned but never reads their edges.
func naiveToBePlannedIgnoringRealizedBy(ctx context.Context, ix *index.Index) ([]status.Row, error) {
	var out []status.Row
	for _, sel := range []index.Selector{
		{Kinds: []ledger.Kind{ledger.KindDecision}, States: []ledger.State{ledger.StateAccepted}},
		{Kinds: []ledger.Kind{ledger.KindIntent}, States: []ledger.State{ledger.StateActive}},
	} {
		refs, err := ix.Select(ctx, sel)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			out = append(out, status.Row{ID: ref.ID, Kind: ref.Kind, Title: ref.Title, Bucket: status.ToBePlanned, Source: status.SourceLedger})
		}
	}
	return out, nil
}

// addRealizedByEdge rewrites an entry file in place to carry one realized_by
// edge, inserting the edges: block right after the created: line. It exists
// only to mutate a copied fixture, never a committed one.
func addRealizedByEdge(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines)+2)
	inserted := false
	for _, line := range lines {
		out = append(out, line)
		if !inserted && strings.HasPrefix(line, "created:") {
			out = append(out, "edges:", `  - type: realized_by`, `    to: "kazi:goal-test"`)
			inserted = true
		}
	}
	if !inserted {
		t.Fatalf("%s has no created: line to anchor the mutation on", path)
	}
	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// rowFor returns the row with the given id, or nil.
func rowFor(rows []status.Row, id string) *status.Row {
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i]
		}
	}
	return nil
}

// requireRow fails unless rows contains exactly the wanted id/bucket/source.
func requireRow(t *testing.T, rows []status.Row, id string, bucket status.Bucket, source status.Source) {
	t.Helper()
	row := rowFor(rows, id)
	if row == nil {
		t.Errorf("%s is missing from the result", id)
		return
	}
	if row.Bucket != bucket {
		t.Errorf("%s: Bucket = %q, want %q", id, row.Bucket, bucket)
	}
	if row.Source != source {
		t.Errorf("%s: Source = %q, want %q", id, row.Source, source)
	}
}
