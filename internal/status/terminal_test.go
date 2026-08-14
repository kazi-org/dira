package status_test

import (
	"context"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/index/indextest"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/status"
)

// TestTerminal is E4-L2-T5's acc line.
func TestTerminal(t *testing.T) {
	ctx := context.Background()

	t.Run("achieved intent reported", func(t *testing.T) {
		ix := openFixtureIndex(t, "achieved-intent")
		rows, err := status.Terminal(ctx, ix)
		if err != nil {
			t.Fatalf("Terminal: %v", err)
		}
		requireTerminalRow(t, rows, "int-1001", status.Achieved)
	})

	t.Run("abandoned intent reported", func(t *testing.T) {
		ix := openFixtureIndex(t, "abandoned-intent")
		rows, err := status.Terminal(ctx, ix)
		if err != nil {
			t.Fatalf("Terminal: %v", err)
		}
		requireTerminalRow(t, rows, "int-1002", status.Abandoned)
	})

	t.Run("Terminal never reports a non-intent", func(t *testing.T) {
		// docs/plan/tasks/E4-L2.md's acc line names real-snapshot's
		// superseded decision as the example here. real-snapshot has none
		// — every decision in this repository's real ledger is
		// state: accepted, verified by direct grep at fixture-recording
		// time (see testdata/ledgers/README.md and
		// TestToBePlanned/real-snapshot_carries_no_realized_by_edge_to_exclude
		// for the same class of finding). superseded-target/'s dec-1002
		// supplies a genuine superseded decision instead, and this loop
		// covers every fixture in the corpus generically rather than
		// hardcoding just that one case.
		for _, name := range fixtureNames {
			ix := openFixtureIndex(t, name)
			rows, err := status.Terminal(ctx, ix)
			if err != nil {
				t.Fatalf("%s: Terminal: %v", name, err)
			}
			for _, row := range rows {
				kind := entryKind(t, ix, row.ID)
				if kind != ledger.KindIntent {
					t.Errorf("%s: Terminal reported %s, kind %q — Terminal must only ever report intents", name, row.ID, kind)
				}
			}
		}

		// None of the six fixtures happens to carry a non-intent entry in an
		// achieved/abandoned-shaped state (a note's legal states include
		// "abandoned" too — ledger.Kind.States() — so the clause above is
		// otherwise unproven able to fail). This fixture supplies one: an
		// abandoned note alongside an abandoned intent, both real candidates
		// for the state-only filter a wrong implementation might use.
		diraDir := indextest.Materialise(t, []*ledger.Entry{
			{ID: "int-3101", Kind: ledger.KindIntent, Title: "Retire the legacy import script", State: ledger.StateAbandoned, Created: "2026-08-01T15:00:00Z"},
			{ID: "note-3101", Kind: ledger.KindNote, Title: "Investigated switching to a message queue", State: ledger.StateAbandoned, Created: "2026-08-01T15:05:00Z"},
		})
		ix := openLedgerDir(t, diraDir)
		rows, err := status.Terminal(ctx, ix)
		if err != nil {
			t.Fatalf("Terminal: %v", err)
		}
		if got := terminalRowFor(rows, "note-3101"); got != nil {
			t.Errorf("Terminal reported the abandoned NOTE note-3101 (%+v); it must only ever report intents", got)
		}
		if terminalRowFor(rows, "int-3101") == nil {
			t.Error("int-3101 is an abandoned intent and must still appear")
		}
	})

	t.Run("disjoint from ToBePlanned over the same ledger", func(t *testing.T) {
		diraDir := indextest.Materialise(t, mixedLifecycleFixture())
		ix := openLedgerDir(t, diraDir)

		terminal, err := status.Terminal(ctx, ix)
		if err != nil {
			t.Fatalf("Terminal: %v", err)
		}
		planned, err := status.DeriveToBePlanned(ctx, ix)
		if err != nil {
			t.Fatalf("DeriveToBePlanned: %v", err)
		}

		if len(terminal) == 0 || len(planned) == 0 {
			t.Fatalf("fixture produced %d terminal and %d to-be-planned rows; both must be non-empty for the cross-check to mean anything", len(terminal), len(planned))
		}

		for _, tr := range terminal {
			if rowFor(planned, tr.ID) != nil {
				t.Errorf("%s appears in both Terminal (%s) and ToBePlanned", tr.ID, tr.Group)
			}
		}
		for _, pr := range planned {
			for _, tr := range terminal {
				if tr.ID == pr.ID {
					t.Errorf("%s appears in both ToBePlanned and Terminal (%s)", pr.ID, tr.Group)
				}
			}
		}

		requireTerminalRow(t, terminal, "int-3001", status.Achieved)
		requireTerminalRow(t, terminal, "int-3002", status.Abandoned)
		if rowFor(planned, "int-3003") == nil {
			t.Error("int-3003 is active and should appear in ToBePlanned")
		}
	})
}

// mixedLifecycleFixture is the "Both sides" cross-check fixture: one
// achieved intent, one abandoned intent and one active intent, so Terminal
// and DeriveToBePlanned run over the SAME ledger rather than over separate
// single-purpose fixtures, and their outputs can be checked disjoint rather
// than merely individually correct.
func mixedLifecycleFixture() []*ledger.Entry {
	return []*ledger.Entry{
		{ID: "int-3001", Kind: ledger.KindIntent, Title: "Ship the v1 export pipeline", State: ledger.StateAchieved, Created: "2026-08-01T14:00:00Z"},
		{ID: "int-3002", Kind: ledger.KindIntent, Title: "Build a browser extension companion", State: ledger.StateAbandoned, Created: "2026-08-01T14:05:00Z"},
		{ID: "int-3003", Kind: ledger.KindIntent, Title: "Give the export pipeline a retry policy", State: ledger.StateActive, Created: "2026-08-01T14:10:00Z"},
	}
}

func requireTerminalRow(t *testing.T, rows []status.TerminalRow, id string, group status.TerminalGroup) {
	t.Helper()
	for _, r := range rows {
		if r.ID == id {
			if r.Group != group {
				t.Errorf("%s: Group = %q, want %q", id, r.Group, group)
			}
			return
		}
	}
	t.Errorf("%s is missing from the result", id)
}

func terminalRowFor(rows []status.TerminalRow, id string) *status.TerminalRow {
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i]
		}
	}
	return nil
}

func entryKind(t *testing.T, ix *index.Index, id string) ledger.Kind {
	t.Helper()
	e, err := ix.Entry(context.Background(), id)
	if err != nil {
		t.Fatalf("reading %s: %v", id, err)
	}
	return e.Kind
}
