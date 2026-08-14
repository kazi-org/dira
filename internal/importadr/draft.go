package importadr

import (
	"fmt"

	"github.com/kazi-org/dira/internal/ledger"
)

// draft.go is E2-L7-T5: the write policy for import — one staged decision
// draft per document carrying at least one reasoned alternative, settled by
// docs/plan/tasks/E2-L7.md's "what is already known" section: `state:
// staged`, `source.tier: regex`, `source.hook: import`, never `confirmed_by`.
// dec-0003 states regex-tier output is never accepted, because a regex has no
// business asserting rationale — imported entries land as staged and flow
// through E2-L4's existing `dira distill` for individual disposition, the
// same as every other staged entry.

// DocumentKey is the natural identity of a scanned document for idempotence:
// its path plus a content hash, so a document already turned into an entry is
// recognised even if the corpus directory is rescanned. It mirrors
// ScannedDocument's Path/SHA256 pair rather than reusing that type directly,
// so a caller can build the exclusion set from whatever it persisted (T6's
// job) without needing a full ScannedDocument to do it.
type DocumentKey struct {
	Path   string
	SHA256 string
}

// keyOf is d's DocumentKey.
func keyOf(d ScannedDocument) DocumentKey {
	return DocumentKey{Path: d.Path, SHA256: d.SHA256}
}

// BuildImportDrafts decides what to write for report's IMPORT case: one
// ledger.Entry draft per document carrying ≥1 reasoned alternative, excluding
// any document whose DocumentKey is already in alreadyImported.
//
// confirmed false returns (nil, nil): no drafts of any kind. report routed to
// INDEX is a caller error, the mirror of BuildIndexArtifact's equivalent
// clause — this policy has no silent no-op path for the wrong input.
//
// Every returned entry is validated with ledger.ValidateDraft before this
// function returns it, so a caller can never receive a draft dira log's own
// validator would reject.
func BuildImportDrafts(report Report, confirmed bool, alreadyImported map[DocumentKey]bool) ([]*ledger.Entry, error) {
	if report.Verdict != VerdictImport {
		return nil, fmt.Errorf("%w: BuildImportDrafts called with a %s-routed report", ErrWrongVerdict, report.Verdict)
	}
	if !confirmed {
		return nil, nil
	}

	var drafts []*ledger.Entry
	for _, d := range report.Documents {
		if !d.WithReason() {
			continue
		}
		if alreadyImported[keyOf(d)] {
			continue
		}

		entry := &ledger.Entry{
			Kind:  ledger.KindDecision,
			Title: draftTitle(d),
			State: ledger.StateStaged,
			Source: &ledger.Source{
				Hook:    ledger.HookImport,
				Tier:    ledger.TierRegex,
				Excerpt: draftExcerpt(d),
			},
		}
		for _, alt := range d.Alternatives {
			if alt.Reason == "" {
				// A bare alternative has no why_not, and entry.Validate
				// requires one on every alternative it carries ("an option
				// without a reason is not an alternative") — this document
				// still qualifies for import because at least one OTHER
				// alternative is reasoned, but a bare one is never itself
				// written into the draft.
				continue
			}
			entry.Alternatives = append(entry.Alternatives, ledger.Alternative{
				Option:    alt.Option,
				WhyNot:    alt.Reason,
				RevisitIf: alt.RevisitIf,
			})
		}

		if err := entry.ValidateDraft(); err != nil {
			return nil, fmt.Errorf("importadr: building a draft for %s: %w", d.Path, err)
		}
		drafts = append(drafts, entry)
	}
	return drafts, nil
}

// titleMaxRunes is entry.schema.json's own ceiling (Entry.Validate: "want 3
// to 120"). draftTitle truncates rather than fails, because a document whose
// H1 runs long is not a document import should refuse to touch.
const titleMaxRunes = 120

// draftTitle is the entry's title: the document's own H1, or its path if it
// has none — never invented prose, because a regex has no business writing a
// sentence about what a document decided.
func draftTitle(d ScannedDocument) string {
	title := d.Title
	if title == "" {
		title = d.Path
	}
	runes := []rune(title)
	if len(runes) > titleMaxRunes {
		runes = runes[:titleMaxRunes]
	}
	if len(runes) < 3 {
		// entry.schema.json's floor. A path or title this short is not
		// realistic for a real corpus, but padding rather than failing
		// keeps this policy total over whatever a directory contains.
		for len(runes) < 3 {
			runes = append(runes, '.')
		}
	}
	return string(runes)
}

// excerptMaxRunes is entry.schema.json's own ceiling on source.excerpt.
const excerptMaxRunes = 1000

// draftExcerpt is the evidence a reviewer sees for where this entry came
// from: the document's own path, so `dira distill` can point back at the
// source file without this policy inventing commentary about it.
func draftExcerpt(d ScannedDocument) string {
	excerpt := "imported from " + d.Path
	runes := []rune(excerpt)
	if len(runes) > excerptMaxRunes {
		runes = runes[:excerptMaxRunes]
	}
	return string(runes)
}
