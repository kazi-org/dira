// Package enforcer is the relitigation firewall: it decides whether a plan
// written in free text contradicts something the ledger already settled.
//
// # Nothing here can be talked out of a verdict
//
// dec-0014 fixes the mechanism as lexical matching computed fresh, in the
// binary, over the ledger being checked — no model client (dec-0003), no
// network (cst-0004), no agent adjudicating (int-0001). That is not a
// preference about implementation: `dira check` runs from a pre-commit hook, a
// CI step, and a human at a terminal with the network unplugged, and in every
// one of those the non-zero exit *is* the product. A design whose exit code
// depended on a live session having rendered judgment would produce no verdict
// at all in the three places the verdict matters most. An agent-assist mode
// that enriches a citation on top of this floor stays admissible; a mode that
// gates the exit code does not.
//
// The consequence to hold on to when reading the rest of this package: every
// number here is computed from the ledger under test at check time. There is no
// shipped idf table, no embedded corpus, and nothing to keep in sync.
//
// # What it is allowed to enforce
//
// The enforcement set is closed, and the closure is the whole difference
// between a firewall and a wolf-crier. It is written out in target.go against
// the table in docs/plan/lanes/E3.md. Two rows of it are load-bearing and easy
// to get wrong: a rejected decision is enforced (it is permanent substrate, not
// a tombstone), and a staged decision never is (it is an unconfirmed regex-tier
// guess, and enforcing a guess is the fastest route to the check being switched
// off).
//
// # What it will not say
//
// A private entry is cited by ref and never by text — in every mode, including
// --json. docs/design.md §7 scopes that to "a public context", but the binary
// cannot classify its own stdout: a pipe into a PR body looks exactly like a
// terminal. So the rule is unconditional here, which is strictly tighter than
// cst-0003 requires and therefore supersedes nothing.
package enforcer

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/kazi-org/dira/internal/ledger"
)

// A Ledger is the read path this check runs over.
//
// One method, sized to what this consumer needs, and deliberately not
// *index.Index. Two things follow from that. The check can be graded against a
// slice of entries with no SQLite file anywhere near it, which is what lets the
// corpus test run 43 plans against one ledger read. And this package's
// dependency graph stays small enough that TestTheMatcherCannotReachTheNetwork
// can assert what dec-0003 and cst-0004 promise — that no part of reaching a
// verdict can dial anything — rather than leaving it as a claim in a comment.
//
// The caller wires it to the same query path `dira why` uses (see
// cmd/dira/check.go); dira has one read path and this lane did not grow a
// second one.
type Ledger interface {
	// Entries returns every entry in the ledger, each read from its file.
	Entries(ctx context.Context) ([]*ledger.Entry, error)
}

// A Conflict is one cited contradiction.
type Conflict struct {
	Citation

	// Score is the overlap that produced the citation. It is advisory — a
	// number for an agent-assist mode or a human tuning a ledger to reason
	// about — and it is not part of the verdict, which is already decided by
	// the time a Conflict exists.
	Score float64
}

// A Verdict is the whole result of a check.
type Verdict struct {
	// Plan is the candidate text, as typed.
	Plan string

	// Conflicts are the citations, strongest first, then by entry id so two
	// runs over the same ledger produce the same bytes.
	Conflicts []Conflict

	// Enforced is how many entries were eligible to be cited. It is
	// reported on the compliant path because "no conflict" against a ledger
	// the check failed to read is indistinguishable from "no conflict"
	// against one it read in full, and only one of those is an answer.
	Enforced int
}

// Compliant reports whether the plan contradicts nothing.
func (v *Verdict) Compliant() bool { return len(v.Conflicts) == 0 }

// ExitCode is the process exit code this verdict selects. It is never
// ExitDiraError: a verdict exists only where the check ran.
func (v *Verdict) ExitCode() int {
	if v.Compliant() {
		return ExitCompliant
	}
	return ExitConflict
}

// Check matches plan against everything lg enforces.
//
// It returns an error only when the ledger cannot be read. A plan that
// contradicts four decisions is a successful check with four conflicts in it,
// not a failure — the exit-code contract in cmd/dira keeps "you are
// contradicting yourself" and "dira is broken" apart, and folding the first
// into an error here would collapse them at the source.
func Check(ctx context.Context, lg Ledger, plan string) (*Verdict, error) {
	if lg == nil {
		return nil, errors.New("check: no ledger")
	}
	entries, err := lg.Entries(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading the ledger: %w", err)
	}
	return check(plan, entries), nil
}

// check is Check over entries already read. It is the seam the corpus test
// drives, so that grading the matcher costs one ledger read for 43 rows rather
// than 43.
func check(plan string, entries []*ledger.Entry) *Verdict {
	return newMatcher(entries).verdict(plan)
}

// verdict matches one plan against everything the matcher enforces.
func (m *matcher) verdict(plan string) *Verdict {
	v := &Verdict{Plan: plan, Enforced: countEnforced(m.units)}
	candidate := Parse(plan)
	if candidate.Empty() {
		return v
	}

	// One conflict per entry: an entry whose title and two alternatives all
	// match has still only been contradicted once, and printing it three
	// times would make the check look like it was padding its case.
	best := map[string]Conflict{}
	for _, u := range m.units {
		h := m.score(candidate, u)
		if !m.matched(h) {
			continue
		}
		if prior, ok := best[u.citation.Entry]; ok && prior.Score >= h.score {
			continue
		}
		best[u.citation.Entry] = Conflict{Citation: u.citation, Score: h.score}
	}

	for _, c := range best {
		v.Conflicts = append(v.Conflicts, c)
	}
	sort.Slice(v.Conflicts, func(i, j int) bool {
		if v.Conflicts[i].Score != v.Conflicts[j].Score {
			return v.Conflicts[i].Score > v.Conflicts[j].Score
		}
		return v.Conflicts[i].Entry < v.Conflicts[j].Entry
	})
	return v
}

// documentText is what an entry contributes to document frequency.
//
// Every entry counts, including the kinds that are never enforcement substrate:
// dec-0014 measures distinctiveness across the ledger under test, and a word
// that appears in every intent and note in this repository is not distinctive
// just because it happens to be absent from the decisions. What each entry
// contributes is the same text the matcher would match against it — frontmatter
// for a decision, frontmatter plus body for a constraint — so the statistics and
// the comparison agree about what an entry says.
//
// The body read stays bounded to constraints, which is dec-0014's answer to
// int-0002's latency posture. Note the honest limit: ledger.Store has no
// frontmatter-only read, so the file itself is read either way. What the rule
// bounds here is how much text is tokenised, not how much is fetched.
func documentText(e *ledger.Entry) Text {
	parts := e.Title
	for _, tag := range e.Tags {
		parts += " " + tag
	}
	for _, alt := range e.Alternatives {
		parts += " " + alt.Option + " " + alt.WhyNot
	}
	if e.Kind == ledger.KindConstraint {
		parts += " " + e.Body
	}
	return Parse(parts)
}

// countEnforced counts entries, not units: a decision with four alternatives is
// one entry the check could cite.
func countEnforced(units []unit) int {
	seen := map[string]bool{}
	for _, u := range units {
		seen[u.citation.Entry] = true
	}
	return len(seen)
}
