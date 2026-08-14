package importadr

import (
	"errors"
	"fmt"
)

// index.go is E2-L7-T4: the write policy for "index instead" — what a
// zero-yield corpus's dry run writes when confirmed, and what it must never
// write. It returns data; cmd/dira/import.go (T6) performs the write, the
// same split internal/skill.Install makes for the same reason (skill/install.go's
// own doc comment).

// ErrWrongVerdict marks a policy called against a report it does not own —
// the index policy against an IMPORT-routed report, or the import policy
// against an INDEX-routed one. It is a caller error, not a silent no-op: a
// no-op here would look identical to a correct empty index on a corpus
// nobody measured as empty.
var ErrWrongVerdict = errors.New("importadr: report is not routed to the verdict this policy handles")

// DocumentRef names one scanned document by path and title, and nothing
// else. Its shape is checked mechanically (index_test.go's reflection
// assertion) to guarantee it can never grow a field that overlaps an entry's
// kind, state or alternatives — cst-0002's closed set does not get violated
// by a later edit here without that edit also touching the check that
// enforces it.
type DocumentRef struct {
	Path  string
	Title string
}

// IndexArtifact is what "index instead" writes: a manifest of the documents a
// zero-yield corpus scan found, headed for .dira/cache/imports/ — regenerable
// by re-running `dira import` over the same directory, never committed, and
// never confused with a ledger entry.
type IndexArtifact struct {
	Documents []DocumentRef
}

// BuildIndexArtifact decides what to write for report's INDEX case.
//
// confirmed false returns (nil, nil): no writes of any kind. report routed to
// IMPORT is a caller error — this policy has no path for it, silent or
// otherwise.
func BuildIndexArtifact(report Report, confirmed bool) (*IndexArtifact, error) {
	if report.Verdict != VerdictIndex {
		return nil, fmt.Errorf("%w: BuildIndexArtifact called with a %s-routed report", ErrWrongVerdict, report.Verdict)
	}
	if !confirmed {
		return nil, nil
	}

	artifact := &IndexArtifact{Documents: make([]DocumentRef, 0, len(report.Documents))}
	for _, d := range report.Documents {
		artifact.Documents = append(artifact.Documents, DocumentRef{Path: d.Path, Title: d.Title})
	}
	return artifact, nil
}
