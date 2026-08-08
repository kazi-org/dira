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
	"strings"

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

// A Notice is a match against something the ledger has stopped enforcing,
// redirected to whatever replaced it.
//
// It is informational and never a verdict: a notice cannot make a check exit
// non-zero, because the entry it came from is superseded and a superseded entry
// enforces nothing. The row exists so that supersession *moves* enforcement
// rather than silently dropping it — a plan that would have been refused last
// month gets told where the thinking went, instead of getting a bare ✓ that
// looks identical to never having been considered.
//
// What it deliberately does not carry is the id of the superseded entry. The
// remedy and the record both live in the replacement: it is the entry that is
// enforced now, it is the entry that carries the `supersedes` edge, and
// `dira why <replacement>` walks back to what it replaced. Naming the retired
// id here would put a dead reference in the one line a reader is meant to act
// on.
type Notice struct {
	// SupersededBy is the entry that replaced the one the plan matched, or
	// empty where the record names none — a state flipped by hand with no
	// `supersedes` edge pointing at it, which is exactly the inconsistency
	// qst-0006 found in this repository's own ledger.
	SupersededBy string

	// Enforced reports whether that replacement is itself enforcement
	// substrate. It is false for a replacement that is staged, superseded in
	// its turn, or of a kind the check never matches — and the message says
	// so rather than promising an enforcement that is not there.
	Enforced bool

	// Basis is what inside the retired entry the plan matched.
	Basis Basis

	// Score is the overlap that produced the notice, on the same advisory
	// footing as a Conflict's.
	Score float64

	// entry is the superseded entry's id. It is unexported because nothing
	// may render it: it exists to hold one notice per retired entry rather
	// than one per matched alternative.
	entry string
}

// A Verdict is the whole result of a check.
type Verdict struct {
	// Plan is the candidate text, as typed.
	Plan string

	// Conflicts are the citations, strongest first, then by entry id so two
	// runs over the same ledger produce the same bytes.
	Conflicts []Conflict

	// Notices are the matches against retired thinking, strongest first.
	// They are reported alongside the verdict and are never part of it.
	Notices []Notice

	// Enforced is how many entries were eligible to be cited. It is
	// reported on the compliant path because "no conflict" against a ledger
	// the check failed to read is indistinguishable from "no conflict"
	// against one it read in full, and only one of those is an answer.
	//
	// A superseded entry is not counted: it cannot be cited, and counting it
	// would make the number a count of files rather than of what is enforced.
	Enforced int

	// Parents is what became of each declared parent ledger, in declared
	// order, or empty for a check with no parents.
	//
	// It is on the verdict rather than beside it because a caller that has the
	// answer must also have the account of how complete it is: a check that
	// quietly dropped a parent's constraints and a check that had none to drop
	// produce identical conflicts, and only one of those is the whole answer.
	// It is not part of the verdict — nothing here moves Compliant or
	// ExitCode, and an unreachable parent may not, because exit 2 means a
	// cited conflict and nothing else.
	Parents []ParentReport
}

// A ParentStatus is what became of one declared parent ledger.
//
// dec-0011 resolves a cross-boundary reference into three states — oriented,
// withheld, orphan — of which only the last is drift. Two of them are about a
// declared parent and are the two this type distinguishes; the third is about a
// ref with no declaration behind it, which is not a parent at all and has
// nothing to report here.
type ParentStatus string

const (
	// ParentResolved is a parent that was read. Its Evaluated count is a real
	// count of what that parent contributed.
	ParentResolved ParentStatus = "resolved"

	// ParentUnresolved is a declared parent that could not be read from here.
	//
	// It is reported rather than dropped: a check missing half its enforcement
	// set looks exactly like a compliant one, and it fails that way in exactly
	// the configuration a public clone has.
	ParentUnresolved ParentStatus = "unresolved"

	// ParentWithheld is a parent the child declared `visibility = "private"`
	// and could not read from here.
	//
	// dec-0018 makes this a designed state rather than a fault: a private
	// parent is expected to be absent wherever it has not been shared, so its
	// absence is the configuration working. The renderer says so by never
	// using the word error or the word warning about it, and by reporting it
	// under a different name from the parent that is simply missing — the two
	// are different facts, and collapsing them would either alarm a reader
	// about a working setup or quiet one about a broken one.
	ParentWithheld ParentStatus = "withheld"
)

// A ParentReport is what one declared parent contributed, for a caller that has
// to report it.
//
// Everything it carries is safe to print. That is the whole design: the
// declaration behind it holds a path and may hold a `label`, dec-0011 says a
// private parent's label must never ship, and a filesystem path to that ledger
// identifies it at least as well as its label does — so neither is copied into
// this type and no renderer can print what it was never given.
type ParentReport struct {
	// Namespace is the key the parent was declared under.
	//
	// It is the child's own word for that ledger, committed in the child's own
	// config and already published in every inherited citation and remedy line,
	// so naming it discloses nothing a resolved parent would not have.
	Namespace string

	// Status is what became of this parent.
	Status ParentStatus

	// Evaluated is how many of the parent's entries produced enforcement
	// units.
	//
	// It is zero for anything but ParentResolved, and that zero is the only
	// count such a parent has. How many constraints a ledger holds that nobody
	// could open is not knowable from here, so no field states it: a report
	// that named a total would be inventing the one number the failure it is
	// reporting made unreadable.
	Evaluated int
}

// ParentReports pairs each declaration with what became of it.
//
// The two halves arrive separately because they are two different facts.
// Inherited.Parents says whether a parent could be read; the declaration says
// whether the child expected to be able to. Only together do they tell an
// unresolved parent from a withheld one — the same unreadable ledger, and
// dec-0011 says a reader should hear about them differently.
//
// A parent declared private that *was* read is resolved, not withheld: it
// contributed, and its citations carry no text (inherit.go's forcedPrivate),
// which is where its privacy is kept. Withheld is about a ledger that is not
// here.
func ParentReports(parents []Parent, inh *Inherited) []ParentReport {
	if inh == nil {
		return nil
	}

	// Matched by namespace rather than by position. Inherit returns one result
	// per declaration in declared order today, but a report that silently
	// mislabelled one parent as another's kind is exactly the failure this is
	// meant to catch, and a name is what both sides actually agree on.
	private := make(map[string]bool, len(parents))
	for _, p := range parents {
		private[strings.TrimSpace(p.Decl.Name)] = p.Decl.Private()
	}

	reports := make([]ParentReport, 0, len(inh.Parents))
	for _, result := range inh.Parents {
		report := ParentReport{
			Namespace: result.Namespace,
			Status:    ParentResolved,
			Evaluated: result.Evaluated,
		}
		if result.Err != nil {
			// The error itself is deliberately not carried forward. It is
			// the backend's, and a backend's error names the file it
			// failed on — so the one field a reader would most want here
			// is the one that cannot be printed.
			report.Evaluated = 0
			report.Status = ParentUnresolved
			if private[result.Namespace] {
				report.Status = ParentWithheld
			}
		}
		reports = append(reports, report)
	}
	return reports
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

	v.Notices = m.notice(candidate)
	return v
}

// notice matches the plan against what the ledger has retired.
//
// It is the same comparison the verdict uses, run over the units supersession
// took out of the enforcement set. Sharing the comparison is the point: a
// redirect that fired on a different rule from the citation it replaces would
// mean supersession changed what the check *detects* rather than what it
// enforces, and the entry that used to be cited could quietly stop being
// noticed at all.
func (m *matcher) notice(candidate Text) []Notice {
	best := map[string]Notice{}
	for _, u := range m.retired {
		h := m.score(candidate, u)
		if !m.matched(h) {
			continue
		}
		if prior, ok := best[u.citation.Entry]; ok && prior.Score >= h.score {
			continue
		}
		best[u.citation.Entry] = Notice{
			SupersededBy: u.supersededBy,
			Enforced:     u.supersederEnforced,
			Basis:        u.citation.Basis,
			Score:        h.score,
			entry:        u.citation.Entry,
		}
	}

	notices := make([]Notice, 0, len(best))
	for _, n := range best {
		notices = append(notices, n)
	}
	sort.Slice(notices, func(i, j int) bool {
		if notices[i].Score != notices[j].Score {
			return notices[i].Score > notices[j].Score
		}
		return notices[i].entry < notices[j].entry
	})
	return notices
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
