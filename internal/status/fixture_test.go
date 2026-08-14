package status_test

import (
	"os"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

// TestFixtureCorpus is E4-L2-T2's acc line.
func TestFixtureCorpus(t *testing.T) {
	// Proven able to fail against an empty testdata/ledgers directory first:
	// every "directory X exists" clause below would otherwise be unreached,
	// which looks like success right up until "at least one blocks edge" is
	// checked against nothing.
	top, err := os.ReadDir("testdata/ledgers")
	if err != nil {
		t.Fatalf("reading testdata/ledgers: %v", err)
	}
	if len(top) == 0 {
		t.Fatal("testdata/ledgers is empty; the completeness check below would pass over nothing")
	}

	for _, name := range fixtureNames {
		if _, err := os.Stat(fixtureEntriesDir(name)); err != nil {
			t.Fatalf("fixture %s: %v", name, err)
		}
		// Every .md file under it parses via the E1 codec without error.
		if entries := fixtureEntries(t, name); len(entries) == 0 {
			t.Fatalf("fixture %s: no entries decoded", name)
		}
	}

	t.Run("answered-question", func(t *testing.T) {
		entries := fixtureEntries(t, "answered-question")
		q := mustFind(t, entries, "qst-1001")
		if q.State != ledger.StateAnswered {
			t.Errorf("qst-1001 state = %q, want %q", q.State, ledger.StateAnswered)
		}
		if !hasEdge(q, ledger.EdgeBlocks) {
			t.Errorf("qst-1001 carries no %s edge", ledger.EdgeBlocks)
		}
	})

	t.Run("superseded-target", func(t *testing.T) {
		entries := fixtureEntries(t, "superseded-target")
		target := mustFind(t, entries, "dec-1002")
		if target.State != ledger.StateSuperseded {
			t.Errorf("dec-1002 state = %q, want %q", target.State, ledger.StateSuperseded)
		}
	})

	t.Run("achieved-intent", func(t *testing.T) {
		entries := fixtureEntries(t, "achieved-intent")
		intents := onlyKind(entries, ledger.KindIntent)
		if len(intents) != 1 {
			t.Fatalf("achieved-intent: %d intents, want exactly 1", len(intents))
		}
		if intents[0].State != ledger.StateAchieved {
			t.Errorf("achieved-intent's intent state = %q, want %q", intents[0].State, ledger.StateAchieved)
		}
	})

	t.Run("abandoned-intent", func(t *testing.T) {
		entries := fixtureEntries(t, "abandoned-intent")
		intents := onlyKind(entries, ledger.KindIntent)
		if len(intents) != 1 {
			t.Fatalf("abandoned-intent: %d intents, want exactly 1", len(intents))
		}
		if intents[0].State != ledger.StateAbandoned {
			t.Errorf("abandoned-intent's intent state = %q, want %q", intents[0].State, ledger.StateAbandoned)
		}
	})

	t.Run("no-realized-by", func(t *testing.T) {
		entries := fixtureEntries(t, "no-realized-by")
		decisions := onlyKind(entries, ledger.KindDecision)
		intents := onlyKind(entries, ledger.KindIntent)
		if len(decisions) != 1 || decisions[0].State != ledger.StateAccepted {
			t.Fatalf("no-realized-by: want exactly one accepted decision, got %d", len(decisions))
		}
		if len(intents) != 1 || intents[0].State != ledger.StateActive {
			t.Fatalf("no-realized-by: want exactly one active intent, got %d", len(intents))
		}
		if hasEdge(decisions[0], ledger.EdgeRealizedBy) {
			t.Errorf("no-realized-by's decision carries a %s edge", ledger.EdgeRealizedBy)
		}
		if hasEdge(intents[0], ledger.EdgeRealizedBy) {
			t.Errorf("no-realized-by's intent carries a %s edge", ledger.EdgeRealizedBy)
		}
	})

	t.Run("real-snapshot", func(t *testing.T) {
		entries := fixtureEntries(t, "real-snapshot")
		found := false
		for _, e := range entries {
			if hasEdge(e, ledger.EdgeBlocks) {
				found = true
				break
			}
		}
		if !found {
			t.Error("real-snapshot: no entry carries a blocks edge")
		}
	})
}

func mustFind(t *testing.T, entries []*ledger.Entry, id string) *ledger.Entry {
	t.Helper()
	for _, e := range entries {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("fixture does not contain %s", id)
	return nil
}

func onlyKind(entries []*ledger.Entry, k ledger.Kind) []*ledger.Entry {
	var out []*ledger.Entry
	for _, e := range entries {
		if e.Kind == k {
			out = append(out, e)
		}
	}
	return out
}

func hasEdge(e *ledger.Entry, t ledger.EdgeType) bool {
	for _, edge := range e.Edges {
		if edge.Type == t {
			return true
		}
	}
	return false
}
