package enforcer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/kazi-org/dira/internal/ledger"
)

// A corpusRow is one labelled candidate plan in the adversarial corpus.
type corpusRow struct {
	ID              string   `yaml:"id"`
	Plan            string   `yaml:"plan"`
	Expect          string   `yaml:"expect"` // "conflict" | "compliant"
	Entry           string   `yaml:"entry,omitempty"`
	WhyNotSubstring string   `yaml:"why_not_substring,omitempty"`
	NearMissOf      string   `yaml:"near_miss_of,omitempty"`
	SharedTerms     []string `yaml:"shared_terms,omitempty"`
	Note            string   `yaml:"note"`
}

type corpus struct {
	Rows []corpusRow `yaml:"rows"`
}

// TestCorpusWellFormed checks that the corpus is a usable grading instrument
// before anything is graded against it.
//
// It was specified in prose by E3-L1, which produced data and no Go, and it is
// implemented here verbatim from docs/decisions-pending/E3-L1-report.md.
//
// # What it deliberately does not check
//
// This test never runs the matcher, and asserts nothing whatsoever about
// detection rate or false-positive rate. That is TestMatchesTheCorpus's job,
// and the separation is what keeps this one honest: the freeze check below is
// the reason the detection test cannot be made to pass by editing the corpus
// out from under it, so the two must not be able to fail for each other's
// reasons. Do not "fix" this test to check recall.
func TestCorpusWellFormed(t *testing.T) {
	t.Parallel()

	// Runs first and fatally: every assertion below is a statement about a
	// specific 43 rows, and none of them means anything if the file is not
	// the one that was frozen.
	t.Run("freeze", func(t *testing.T) { assertCorpusFrozen(t) })

	rows := loadCorpus(t)
	matchable := map[string]string{}
	entryExists := func(t *testing.T, id string) bool {
		t.Helper()
		if _, ok := matchable[id]; ok {
			return true
		}
		path := filepath.Join(daemonLedger, id+".md")
		if _, err := os.Stat(path); err != nil {
			return false
		}
		matchable[id] = matchableText(fixtureEntry(t, path))
		return true
	}

	t.Run("row count", func(t *testing.T) {
		if len(rows) < 40 {
			t.Errorf("corpus holds %d rows, want at least 40", len(rows))
		}
	})

	t.Run("row shape", func(t *testing.T) {
		seen := map[string]bool{}
		for _, r := range rows {
			switch {
			case r.ID == "":
				t.Errorf("a row has no id: %+v", r)
			case seen[r.ID]:
				t.Errorf("%s: duplicate row id", r.ID)
			}
			seen[r.ID] = true

			if strings.TrimSpace(r.Plan) == "" {
				t.Errorf("%s: plan is empty", r.ID)
			}
			if r.Expect != "conflict" && r.Expect != "compliant" {
				t.Errorf("%s: expect is %q, want \"conflict\" or \"compliant\"", r.ID, r.Expect)
			}
			if strings.TrimSpace(r.Note) == "" {
				t.Errorf("%s: note is empty — a row with no stated purpose is a row nobody can audit", r.ID)
			}
		}
	})

	t.Run("conflict rows", func(t *testing.T) {
		for _, r := range rows {
			if r.Expect != "conflict" {
				continue
			}
			if r.NearMissOf != "" || len(r.SharedTerms) > 0 {
				t.Errorf("%s: a conflict row carries near_miss_of/shared_terms, which are compliant-row fields", r.ID)
			}
			if r.Entry == "" {
				t.Errorf("%s: conflict row names no entry", r.ID)
				continue
			}
			if r.WhyNotSubstring == "" {
				t.Errorf("%s: conflict row quotes no why_not substring", r.ID)
				continue
			}
			if !entryExists(t, r.Entry) {
				t.Errorf("%s: names entry %s, which has no file under %s", r.ID, r.Entry, daemonLedger)
				continue
			}
			if !strings.Contains(matchable[r.Entry], r.WhyNotSubstring) {
				t.Errorf("%s: why_not_substring %q does not appear in %s's matchable text",
					r.ID, r.WhyNotSubstring, r.Entry)
			}
		}
	})

	t.Run("compliant rows", func(t *testing.T) {
		for _, r := range rows {
			if r.Expect != "compliant" {
				continue
			}
			if r.Entry != "" || r.WhyNotSubstring != "" {
				t.Errorf("%s: a compliant row carries entry/why_not_substring, which are conflict-row fields", r.ID)
			}
		}
	})

	t.Run("distinct entries", func(t *testing.T) {
		referenced := map[string]bool{}
		for _, r := range rows {
			if r.Entry != "" {
				referenced[r.Entry] = true
			}
			if r.NearMissOf != "" {
				referenced[r.NearMissOf] = true
			}
		}
		if len(referenced) < 8 {
			t.Errorf("the corpus references %d distinct entries, want at least 8", len(referenced))
		}
		for id := range referenced {
			if !entryExists(t, id) {
				t.Errorf("the corpus references %s, which has no file under %s", id, daemonLedger)
			}
		}
	})

	t.Run("near misses", func(t *testing.T) {
		count := 0
		for _, r := range rows {
			if r.Expect != "compliant" || r.NearMissOf == "" {
				continue
			}
			if len(r.SharedTerms) < 2 {
				t.Errorf("%s: near-miss of %s shares %d terms, want at least 2",
					r.ID, r.NearMissOf, len(r.SharedTerms))
				continue
			}
			if !entryExists(t, r.NearMissOf) {
				t.Errorf("%s: near_miss_of %s has no file under %s", r.ID, r.NearMissOf, daemonLedger)
				continue
			}
			ok := true
			for _, word := range r.SharedTerms {
				if !containsWord(r.Plan, word) {
					t.Errorf("%s: shared term %q does not appear as a whole word in the plan", r.ID, word)
					ok = false
				}
				if !containsWord(matchable[r.NearMissOf], word) {
					t.Errorf("%s: shared term %q does not appear as a whole word in %s", r.ID, word, r.NearMissOf)
					ok = false
				}
			}
			if ok {
				count++
			}
		}
		if count < 15 {
			t.Errorf("the corpus holds %d valid near-miss rows, want at least 15 — "+
				"without them a matcher that flags everything scores 100%% recall", count)
		}
	})
}

func loadCorpus(t *testing.T) []corpusRow {
	t.Helper()

	data, err := os.ReadFile(corpusFile)
	if err != nil {
		t.Fatalf("reading %s: %v", corpusFile, err)
	}
	var c corpus
	if err := yaml.Unmarshal(data, &c); err != nil {
		t.Fatalf("parsing %s: %v", corpusFile, err)
	}
	if len(c.Rows) == 0 {
		t.Fatalf("%s holds no rows", corpusFile)
	}
	return c.Rows
}

// matchableText is what "literally appears in the entry" means.
//
// It is not the file's raw bytes, and that is deliberate. A YAML folded scalar
// and a hard-wrapped markdown paragraph both split sentences across newlines
// that carry no meaning, so a raw-bytes substring check would fail on prose
// that a reader would say plainly contains the quote — dec-0042 and dec-0082 in
// this fixture set both wrap a why_not across lines. Collapsing whitespace is
// also what the matcher itself does, so the corpus and the matcher agree about
// what an entry says.
func matchableText(e *ledger.Entry) string {
	parts := []string{collapse(e.Title)}
	for _, alt := range e.Alternatives {
		parts = append(parts, collapse(alt.Option), collapse(alt.WhyNot))
		if alt.RevisitIf != "" {
			parts = append(parts, collapse(alt.RevisitIf))
		}
	}
	parts = append(parts, collapse(e.Body))
	return strings.Join(parts, " | ")
}

func containsWord(haystack, word string) bool {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
	return re.MatchString(haystack)
}
