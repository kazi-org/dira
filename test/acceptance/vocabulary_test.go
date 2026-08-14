package acceptance

// TestNeverStoredVocabularyLint is E4-L5-T4's wiring half: `go test
// ./test/acceptance -run TestNeverStored` — the lane's single locked test
// command — matches this function too (substring, same convention every
// other umbrella in this epic uses), so the vocabulary lint is folded into
// the same named suite as T3 rather than run as a second, separately-named
// gate.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/lint"
)

// fixtureLedgerDirs are E4-L1 through E4-L4's own committed fixture
// ledgers, referenced by relative path rather than copied — the same
// cross-package pattern this epic's tests already use throughout.
var fixtureLedgerDirs = []string{
	"../../internal/status/testdata/ledgers/real-snapshot/.dira/entries",
	"../../internal/status/testdata/ledgers/answered-question/.dira/entries",
	"../../internal/status/testdata/ledgers/superseded-target/.dira/entries",
	"../../internal/status/testdata/ledgers/achieved-intent/.dira/entries",
	"../../internal/status/testdata/ledgers/abandoned-intent/.dira/entries",
	"../../internal/status/testdata/ledgers/no-realized-by/.dira/entries",
	"../../internal/status/testdata/ledgers/realized-by/.dira/entries",
	"../../internal/cli/testdata/map/real-snapshot/entries",
}

// loadLedgerDir decodes every *.md file directly under dir.
func loadLedgerDir(t *testing.T, dir string) []*ledger.Entry {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var out []*ledger.Entry
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			t.Fatalf("reading %s/%s: %v", dir, f.Name(), err)
		}
		e, err := ledger.Decode(data)
		if err != nil {
			t.Fatalf("decoding %s/%s: %v", dir, f.Name(), err)
		}
		out = append(out, e)
	}
	return out
}

func TestNeverStoredVocabularyLint(t *testing.T) {
	t.Run("every committed fixture ledger finds zero matches", func(t *testing.T) {
		for _, dir := range fixtureLedgerDirs {
			t.Run(dir, func(t *testing.T) {
				entries := loadLedgerDir(t, dir)
				if len(entries) == 0 {
					t.Fatalf("%s decoded zero entries; the completeness check below would pass over nothing", dir)
				}
				if matches := lint.ScanEntries(entries); len(matches) != 0 {
					t.Errorf("%s: found %d vocabulary match(es): %+v", dir, len(matches), matches)
				}
			})
		}
	})

	t.Run("a poisoned tag on a copy of a real fixture is caught", func(t *testing.T) {
		entries := loadLedgerDir(t, "../../internal/status/testdata/ledgers/no-realized-by/.dira/entries")
		if len(entries) == 0 {
			t.Fatal("fixture decoded zero entries")
		}
		poisoned := entries[0]
		poisoned.Tags = append(append([]string{}, poisoned.Tags...), "execution-blocked")

		matches := lint.ScanEntries(entries)
		found := false
		for _, m := range matches {
			if m.EntryID == poisoned.ID && m.Field == "tags" && m.Phrase == "execution-blocked" {
				found = true
			}
		}
		if !found {
			t.Errorf("matches = %+v, want one naming %s/tags/execution-blocked", matches, poisoned.ID)
		}
	})

	t.Run("a poisoned edge note on a copy of a real fixture is caught", func(t *testing.T) {
		entries := loadLedgerDir(t, "../../internal/status/testdata/ledgers/answered-question/.dira/entries")
		var poisoned *ledger.Entry
		for _, e := range entries {
			if len(e.Edges) > 0 {
				poisoned = e
				break
			}
		}
		if poisoned == nil {
			t.Fatal("no entry in this fixture carries an edge to poison")
		}
		poisoned.Edges[0].Note = "the goal converged"

		matches := lint.ScanEntries(entries)
		found := false
		for _, m := range matches {
			if m.EntryID == poisoned.ID && m.Field == "edges[].note" && m.Phrase == "converged" {
				found = true
			}
		}
		if !found {
			t.Errorf("matches = %+v, want one naming %s/edges[].note/converged", matches, poisoned.ID)
		}
	})

	t.Run("both sides: zero matches on real fixtures and at least one on poisoned copies are each other's control", func(t *testing.T) {
		// A lint with an empty phrase list would pass the real-fixture
		// clause above AND fail the poisoned-fixture clauses above — this
		// sub-test states that relationship directly, per the lane's own
		// written standard ("a lint passes when its pattern list is
		// empty").
		clean := loadLedgerDir(t, "../../internal/status/testdata/ledgers/no-realized-by/.dira/entries")
		if matches := lint.ScanEntries(clean); len(matches) != 0 {
			t.Fatalf("the clean fixture is not actually clean: %+v", matches)
		}
		clean[0].Tags = append(append([]string{}, clean[0].Tags...), "converged")
		if matches := lint.ScanEntries(clean); len(matches) == 0 {
			t.Fatal("poisoning the same in-memory fixture produced no matches; " +
				"the real-fixture clause and the poisoned clause are not actually testing the same lint")
		}
	})
}
