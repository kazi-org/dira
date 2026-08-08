// Package sniff is capture tier 1: pattern matching over session text that
// proposes ledger entries and is structurally incapable of confirming one.
//
// # Why it may only ever stage
//
// dec-0003 splits capture into two tiers with different trust levels. This is
// the cheap one: no model, no network, no key, no knowledge of what any sentence
// means. A regular expression that saw the words "we're going with X" has
// evidence that a sentence was written, and no evidence whatsoever that a
// decision was made — the words appear inside hypotheticals, quotations,
// instructions and reports of other people's choices, all of which read
// identically to a matcher. So everything this package produces is written
// `state: staged`, `source.tier: regex`, and waits for a human (E2-L4).
//
// That is not enforced by a test asserting the state field. It is enforced by
// stage.go: the only Store this package's writer will use is a stagedOnly, whose
// Create refuses any entry that is not staged and regex-tier, whose Put refuses
// everything, and which Stage wraps around the caller's store before the caller
// can reach it. There is no argument, flag, or option through which a caller of
// this package can write an accepted entry, and no code path inside it that
// constructs one. A test can only demonstrate that; the wrapper is what makes it
// so.
//
// # Precision over recall, as a measured number
//
// The failure this package can inflict on int-0001 is not missing a decision. It
// is staging twenty candidates a session, at which point the human learns to
// reject without reading and the disposition queue becomes the chore that
// int-0001 exists to abolish — the 77-ADR pile with new syntax. So the matcher
// is built to be quiet: two rule families, a long list of guards, and a graded
// corpus in testdata/corpus.yaml that was frozen before match.go existed. The
// observed false-positive rate is reported by TestMatchesTheCorpus on every run
// rather than claimed in a comment.
//
// # What it will not read
//
// Tool output, tool input, and fenced code never reach the matcher. That is a
// precision rule and a privacy rule at once: a `Bash` result is where an
// environment dump, a customer record, or a token actually appears in a
// transcript, and none of it is anybody's decision. On top of that, redact.go
// drops any candidate whose text carries something shaped like a credential —
// dropped whole, never masked, because a masked excerpt is evidence a reviewer
// cannot audit.
package sniff

import (
	"io"
	"strings"
)

// maxExcerpt bounds `source.excerpt`. The schema allows 1000 characters; this is
// far tighter because the excerpt is evidence for one disposition keystroke, not
// a transcript archive, and a staged card a human has to scroll is a staged card
// a human skips.
const maxExcerpt = 300

// maxTitle is the schema's own limit on `title`. Restated here because the
// candidate is trimmed to fit before an Entry exists, and being rejected by the
// validator for a title the sniffer chose would be dira's mistake reported as
// the caller's.
const maxTitle = 120

// maxCandidates bounds one invocation.
//
// A hook that writes sixty files gets uninstalled, and a PreCompact run sees a
// whole session rather than one turn. When more candidates than this are found,
// the earliest are kept — transcript order is the order they were said — and the
// caller is told how many were dropped, because silently discarding a decision
// is the failure this tool exists to prevent.
const maxCandidates = 12

// A Candidate is one proposed entry.
//
// It deliberately has no state, no confirmed_by, no alternatives and no adr
// field. Those are the four fields that would let a regex assert something it
// cannot know, and a type that cannot hold them is a type through which no
// caller can pass them.
type Candidate struct {
	// Title is the one-line rendering of the sentence that matched, trimmed
	// to the schema's limit.
	Title string

	// Excerpt is the sentence itself, bounded. It is the evidence a human
	// reads when disposing of the entry, so it is stored verbatim apart from
	// whitespace collapsing and the length bound.
	Excerpt string

	// Rule names the pattern family that fired. It is diagnostic: it renders
	// in the dry-run listing and in test failures, and it is never written
	// to the ledger, because the ledger records what was captured rather
	// than which regexp caught it.
	Rule string
}

// Sniff returns the candidates in a block of prose.
//
// It is the whole matcher, and it takes a string rather than a transcript so
// that the corpus test grades sentences directly and so that `dira sniff` can be
// pointed at any text a human pastes at it.
func Sniff(text string) []Candidate {
	return collect(sentences(strip(text)))
}

// SniffTranscript returns the candidates in a Claude Code transcript.
//
// Only prose written by a person or a model is considered: tool calls and tool
// results are dropped by the parser and never reach the matcher. A transcript
// that is not JSONL at all is read as plain prose, which is what makes
// `dira sniff < notes.md` work and what keeps a malformed transcript from being
// an error a hook has to survive.
func SniffTranscript(r io.Reader, scope Scope) ([]Candidate, error) {
	segments, err := ParseTranscript(r)
	if err != nil {
		return nil, err
	}
	if scope == LastTurn {
		segments = lastTurn(segments)
	}

	var all []Candidate
	for _, s := range segments {
		all = append(all, collect(sentences(strip(s.Text)))...)
	}
	return dedupe(all), nil
}

// collect runs the matcher over already-split sentences.
func collect(sents []string) []Candidate {
	var out []Candidate
	for _, s := range sents {
		rule, ok := match(s)
		if !ok {
			continue
		}
		if carriesCredential(s) {
			// Dropped whole. See redact.go.
			continue
		}
		title := titleFor(s)
		if title == "" {
			continue
		}
		out = append(out, Candidate{Title: title, Excerpt: bound(s, maxExcerpt), Rule: rule})
	}
	return dedupe(out)
}

// dedupe collapses candidates that would produce the same entry, keeping the
// first. One sentence quoted twice in a turn is one decision.
func dedupe(in []Candidate) []Candidate {
	seen := make(map[string]bool, len(in))
	out := make([]Candidate, 0, len(in))
	for _, c := range in {
		key := strings.ToLower(c.Title)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

// Bound reports how many candidates one invocation may stage, and is exported so
// the command can explain a truncation rather than perform one silently.
func Bound() int { return maxCandidates }
