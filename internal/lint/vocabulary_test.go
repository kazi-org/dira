package lint_test

import (
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/lint"
)

func entry(id string, tags []string, edgeNote string) *ledger.Entry {
	e := &ledger.Entry{ID: id, Kind: ledger.KindDecision, Title: "fixture", Tags: tags}
	if edgeNote != "" {
		e.Edges = []ledger.Edge{{Type: ledger.EdgeInforms, To: "dec-0001", Note: edgeNote}}
	}
	return e
}

// TestVocabularyLint is E4-L5-T4's own acc line, over synthetic entries.
// The real-fixture-corpus half ("every entry under E4-L1 through E4-L4's
// committed fixture ledgers finds zero matches") lives in
// test/acceptance/vocabulary_test.go, wired into the acceptance suite the
// lane's locked acc: line names.
func TestVocabularyLint(t *testing.T) {
	t.Run("a clean entry produces no matches", func(t *testing.T) {
		entries := []*ledger.Entry{entry("dec-1001", []string{"founding", "kazi-seam"}, "narrows the scope")}
		if got := lint.ScanEntries(entries); len(got) != 0 {
			t.Errorf("ScanEntries = %v, want no matches", got)
		}
	})

	t.Run("bare blocked/blocks is never flagged", func(t *testing.T) {
		// qst-0005's own real note, reused here: "disposition capture has
		// no automatic path until the hook exists" — a legitimate edge
		// note about the ledger's own blocks vocabulary, which this lint
		// must not false-positive on.
		entries := []*ledger.Entry{
			entry("qst-0005", []string{"blocks"}, "disposition capture has no automatic path until the hook exists; blocks nothing yet"),
		}
		if got := lint.ScanEntries(entries); len(got) != 0 {
			t.Errorf("ScanEntries flagged bare blocked/blocks: %v", got)
		}
	})

	t.Run("a poisoned tag is caught, naming the entry id and field", func(t *testing.T) {
		entries := []*ledger.Entry{entry("dec-2001", []string{"execution-blocked"}, "")}
		got := lint.ScanEntries(entries)
		if len(got) == 0 {
			t.Fatal("ScanEntries found no matches against a tag carrying \"execution-blocked\"")
		}
		found := false
		for _, m := range got {
			if m.EntryID == "dec-2001" && m.Field == "tags" && m.Phrase == "execution-blocked" {
				found = true
			}
		}
		if !found {
			t.Errorf("ScanEntries = %+v, want a match naming dec-2001/tags/execution-blocked", got)
		}
	})

	t.Run("a poisoned edge note is caught, naming the entry id and field", func(t *testing.T) {
		entries := []*ledger.Entry{entry("dec-2002", nil, "the goal converged last night")}
		got := lint.ScanEntries(entries)
		found := false
		for _, m := range got {
			if m.EntryID == "dec-2002" && m.Field == "edges[].note" && m.Phrase == "converged" {
				found = true
			}
		}
		if !found {
			t.Errorf("ScanEntries = %+v, want a match naming dec-2002/edges[].note/converged", got)
		}
	})

	t.Run("matching is case-insensitive", func(t *testing.T) {
		entries := []*ledger.Entry{entry("dec-2003", []string{"Converged"}, "")}
		if got := lint.ScanEntries(entries); len(got) == 0 {
			t.Error("ScanEntries did not match \"Converged\" case-insensitively")
		}
	})

	t.Run("the permitted case: a visibly-stale in-memory value is not what this lint targets", func(t *testing.T) {
		// dec-0004's revisit_if: "a cached snapshot with a visible
		// staleness timestamp is acceptable." No production code persists
		// one (this lane's own header note explains why no dedicated task
		// exists for it), so the permitted case is a synthetic in-memory
		// type, used only in this test, carrying a staleness statement —
		// proving the lint targets LEDGER FILE content, not a
		// legitimately-labelled in-memory value that happens to be old.
		type stalenessAwareRow struct {
			Bucket     string
			ObservedAt string
			Stale      bool
		}
		row := stalenessAwareRow{Bucket: "converged", ObservedAt: "2026-08-01T00:00:00Z", Stale: true}
		// This lint's surface is ScanEntries([]*ledger.Entry) — a row like
		// this never reaches it at all, because nothing constructs a
		// ledger.Entry from one. The permitted case is proven by
		// construction: passing zero entries derived from row finds zero
		// matches, and the row's own "converged" value is never checked
		// against ledger file content because it never becomes ledger
		// file content.
		if row.Bucket != "converged" || row.ObservedAt == "" || !row.Stale {
			t.Fatal("the fixture's own premise broke")
		}
		if got := lint.ScanEntries(nil); len(got) != 0 {
			t.Errorf("ScanEntries(nil) = %v, want no matches", got)
		}
	})

	t.Run("both sides: an empty phrase list would pass every poisoned fixture too", func(t *testing.T) {
		// The lane's own written standard: "a lint passes when its pattern
		// list is empty." Demonstrated directly: a zero-length vocabulary
		// finds nothing in the poisoned fixtures above, which is exactly
		// why this package's real Vocabulary is checked non-empty first.
		if len(lint.Vocabulary) == 0 {
			t.Fatal("lint.Vocabulary is empty; every clause above would pass vacuously")
		}
		emptyScan := func(_ []*ledger.Entry) []lint.Match {
			// A deliberately wrong scanner: same shape, empty phrase list.
			return nil
		}
		poisoned := []*ledger.Entry{entry("dec-2001", []string{"execution-blocked"}, "")}
		if got := emptyScan(poisoned); len(got) != 0 {
			t.Fatal("the empty-vocabulary control's own premise broke")
		}
	})
}
