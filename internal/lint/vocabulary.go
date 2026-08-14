// Package lint holds standalone checkers over the ledger's on-disk shape —
// not a rendering concern, not a derivation, and reusable outside
// test/acceptance the way internal/skill's policy-over-bytes split keeps
// rendering pure (docs/plan/tasks/E2-L3.md's precedent for this shape).
//
// E4-L5-T4 is this package's first (and, for now, only) checker: an
// execution-status vocabulary lint over ledger content the schema cannot
// constrain. schema/entry.schema.json is additionalProperties: false with a
// closed properties list and a state enum carrying no execution value
// (docs/plan/lanes/E4.md point 11), so "grep entry files for a status
// field" is near-vacuous — green before this lane starts. What the schema
// cannot catch is status SMUGGLED into a field whose shape says nothing
// about its content: an entry's tags, or an edge's note.
package lint

import (
	"strings"

	"github.com/kazi-org/dira/internal/ledger"
)

// Vocabulary is the exact phrase list this lint scans for, recorded here
// rather than left implicit — an implicit list is unauditable, per this
// lane's own risk note. Drawn from internal/kazi's raw status/bucket
// strings (E4-L1) and internal/cli's committed CLI labels (E4-L4-T4).
//
// Deliberately excluded: bare "blocked" / "blocks". "blocks" is itself a
// legitimate edge TYPE name (ledger.EdgeBlocks) and an ordinary English
// word appearing in legitimate edge notes describing why one entry gates
// another. Linting on the bare word would false-positive on the ledger's
// own structural vocabulary — the false-alarm failure mode a near-vacuous
// check produces from the other direction. This list is the
// execution-status-specific phrases only.
var Vocabulary = []string{
	"converged",
	"in progress", "in_progress",
	"execution-blocked", "execution_blocked",
	"over_budget", "over-budget",
	"terminated",
	"stuck",
}

// Match is one vocabulary hit: which entry, which field, and the phrase
// that matched.
type Match struct {
	EntryID string
	Field   string // "tags" or "edges[].note"
	Phrase  string
}

// ScanEntries scans every entry's tags and every edge's note for
// Vocabulary's phrases, matched case-insensitively as whole phrases, and
// returns every match found — never short-circuiting on the first, so a
// caller can report every offending entry and field in one pass.
func ScanEntries(entries []*ledger.Entry) []Match {
	var out []Match
	for _, e := range entries {
		for _, tag := range e.Tags {
			for _, phrase := range Vocabulary {
				if strings.Contains(strings.ToLower(tag), phrase) {
					out = append(out, Match{EntryID: e.ID, Field: "tags", Phrase: phrase})
				}
			}
		}
		for _, edge := range e.Edges {
			if edge.Note == "" {
				continue
			}
			lower := strings.ToLower(edge.Note)
			for _, phrase := range Vocabulary {
				if strings.Contains(lower, phrase) {
					out = append(out, Match{EntryID: e.ID, Field: "edges[].note", Phrase: phrase})
				}
			}
		}
	}
	return out
}
