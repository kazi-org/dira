package enforcer

import (
	"strings"

	"github.com/kazi-org/dira/internal/ledger"
)

// Basis names what inside an entry a plan collided with. It exists because the
// three cases are not interchangeable in the message: only an alternative can
// carry a revisit_if, and a message that implied otherwise would tell a reader
// a door is ajar when the schema gives it no hinge (docs/design.md §7).
type Basis string

// The three bases, one per row of the enforcement-set table in
// docs/plan/lanes/E3.md that can produce a citation.
const (
	// BasisAlternative is a road a decision explicitly did not take. It is
	// the only basis that can offer a revisit condition.
	BasisAlternative Basis = "alternative"

	// BasisDecision is a rejected decision's own subject — the thing that
	// was said no to. Rejected decisions are permanent substrate; a check
	// that read only accepted decisions would miss half the point of the
	// firewall.
	BasisDecision Basis = "decision"

	// BasisConstraint is a standing rule. Its only escape hatch is
	// supersession, in writing.
	BasisConstraint Basis = "constraint"
)

// A Citation is everything a conflict message may say about what was hit. It is
// filled from the entry file, never from the derived cache, which is what keeps
// dec-0002's files-win property true of every rendered byte.
type Citation struct {
	Entry   string
	Kind    ledger.Kind
	State   ledger.State
	Date    string
	Private bool
	Basis   Basis

	// Title is the entry's own subject. It is the quoted text for
	// BasisDecision and BasisConstraint.
	Title string

	// Option, WhyNot and RevisitIf are the matched alternative, empty for
	// every other basis. RevisitIf is empty when the alternative recorded
	// none, which the renderer says out loud rather than omitting.
	Option    string
	WhyNot    string
	RevisitIf string
}

// A unit is one matchable stretch of an entry, paired with the citation a hit
// on it produces.
//
// An entry is several units rather than one document on purpose. A constraint's
// body is long enough that a plan overlapping one of its sentences would be
// diluted to nothing against the whole of it, and an accepted decision's
// alternatives are separate proposals that happen to share a file.
type unit struct {
	text     Text
	citation Citation

	// polarised selects whether the target's own negations take part in the
	// comparison. See Text.Bag.
	polarised bool

	// phraseOnly restricts a unit to the multiword signal, disabling the
	// overlap score for it. It marks the units that are argument prose
	// rather than a proposal — a why_not. See alternativeUnits.
	phraseOnly bool

	// prose selects which multiword signal applies. A proposal can be
	// restated whole; prose can only be quoted from. See matcher.score.
	prose bool

	// supersededBy and supersederEnforced are set only on a retired unit —
	// one this ledger no longer enforces — and carry the redirect a match on
	// it is reported through. See retiredSet.
	supersededBy       string
	supersederEnforced bool
}

// enforcementSet turns the entries of a ledger into the units a plan is checked
// against, applying the closed table in docs/plan/lanes/E3.md:
//
//	decision/accepted    each alternatives[].option (and its why_not)
//	decision/rejected    the entry's own title, and its alternatives
//	decision/staged      nothing — an unconfirmed regex-tier guess (dec-0003)
//	decision/superseded  nothing here; E3-L4 owns the redirect
//	constraint/active    title + tags + body
//	constraint/superseded, intent, question, note   nothing
//
// One deviation from dec-0014's restatement of that table, made deliberately
// and reported rather than buried: dec-0014 lists a rejected decision as
// matched against its title alone, but the frozen corpus (row-010) requires a
// rejected decision's *alternatives* to be matched too — that row proposes
// dec-0042's own "a compacted event log" alternative and quotes that
// alternative's why_not, which is unreachable from the title. The corpus was
// frozen before any matcher existed precisely so it would win an argument like
// this one, so it does. The change is additive: every citation dec-0014's rule
// produces, this one produces as well.
func enforcementSet(entries []*ledger.Entry) []unit {
	var units []unit
	for _, e := range entries {
		units = append(units, unitsFor(e)...)
	}
	return units
}

// retiredSet turns the entries supersession took *out* of the enforcement set
// into the units a match is reported through rather than cited against.
//
// The table in docs/plan/lanes/E3.md gives this exactly one row:
// decision/superseded is matched against nothing, "but a match is reported and
// redirected to its superseder". So this covers superseded *decisions* and
// nothing else — a superseded constraint's row says plainly "nothing", and
// widening the redirect to it would be this lane inventing a rule the table
// closed. The asymmetry is reported in docs/decisions-pending/E3-L4-report.md
// rather than quietly fixed here.
//
// The units are the ones the entry had while it was enforced — its
// alternatives — so the redirect fires on exactly the plans that used to be
// refused. Its title is not matched: a superseded decision's title describes
// what it *chose*, and matching it would make the successor's own design
// produce a notice about the design it replaced.
func retiredSet(entries []*ledger.Entry) []unit {
	replacement := superseders(entries)

	var units []unit
	for _, e := range entries {
		if e.Kind != ledger.KindDecision || e.State != ledger.StateSuperseded {
			continue
		}
		base := Citation{
			Entry:   e.ID,
			Kind:    e.Kind,
			State:   e.State,
			Date:    entryDate(e),
			Private: e.Private,
			Title:   e.Title,
		}
		by := replacement[e.ID]
		for _, u := range alternativeUnits(e, base) {
			u.supersededBy = by.id
			u.supersederEnforced = by.enforced
			units = append(units, u)
		}
	}
	return units
}

// superseder is what the ledger records as having replaced an entry.
type superseder struct {
	id string

	// enforced is whether that replacement is itself enforcement substrate,
	// asked of the enforcement set rather than restated from the table. A
	// replacement that is staged, or superseded in its turn, enforces
	// nothing, and a redirect that claimed otherwise would send a reader to
	// an entry that will not stop them either.
	enforced bool
}

// superseders indexes the `supersedes` edges by the entry they retire.
//
// The edge lives on the *new* entry (dec-0002: edges are stored on the subject,
// so a mutation is one file), which means the only way to ask "what replaced
// this?" is to read the whole ledger and reverse the direction. The check has
// every entry in hand already, so that costs a pass over a slice.
//
// Two entries claiming to have superseded the same one is a broken record, not
// a case with a right answer, so the lowest id wins: the report is then the same
// on every run and on every backend, rather than depending on the order the
// ledger happened to be read in.
func superseders(entries []*ledger.Entry) map[string]superseder {
	out := make(map[string]superseder)
	for _, e := range entries {
		for _, edge := range e.Edges {
			if edge.Type != ledger.EdgeSupersedes {
				continue
			}
			if held, taken := out[edge.To]; taken && held.id <= e.ID {
				continue
			}
			out[edge.To] = superseder{id: e.ID, enforced: len(unitsFor(e)) > 0}
		}
	}
	return out
}

func unitsFor(e *ledger.Entry) []unit {
	base := Citation{
		Entry:   e.ID,
		Kind:    e.Kind,
		State:   e.State,
		Date:    entryDate(e),
		Private: e.Private,
		Title:   e.Title,
	}

	switch {
	case e.Kind == ledger.KindDecision && e.State == ledger.StateAccepted:
		return alternativeUnits(e, base)

	case e.Kind == ledger.KindDecision && e.State == ledger.StateRejected:
		subject := base
		subject.Basis = BasisDecision
		units := []unit{{text: Parse(e.Title), citation: subject, polarised: true}}
		return append(units, alternativeUnits(e, base)...)

	case e.Kind == ledger.KindConstraint && e.State == ledger.StateActive:
		return constraintUnits(e, base)

	default:
		return nil
	}
}

// alternativeUnits makes each rejected alternative two units: what was
// proposed, and why it was refused.
//
// The why_not is matched as well as the option because an option is often a
// noun phrase of three words while the reasoning names the mechanism. "phoning
// home by default" appears nowhere in "anonymous aggregate usage stats", and a
// plan saying "dira should phone home with anonymous telemetry" is
// contradicting that decision (corpus row-021). Both units cite the same
// alternative, so which one fired never reaches the reader.
//
// The why_not is phrase-only, and that restriction is what keeps it from
// costing more than it earns. An option is a proposal, so a plan overlapping it
// is proposing the same thing; a why_not is argument prose, and argument prose
// necessarily discusses the *adopted* design too. dec-0060's fourth why_not
// says the checkpoint file "exists to avoid" the wasted work — so a plan that
// says "write the checkpoint file atomically", which is precisely what dec-0060
// chose, overlaps it strongly while contradicting nothing (corpus row-025).
// Requiring a distinctive contiguous phrase keeps "phone home" and drops
// scattered shared vocabulary.
func alternativeUnits(e *ledger.Entry, base Citation) []unit {
	var units []unit
	for _, alt := range e.Alternatives {
		c := base
		c.Basis = BasisAlternative
		c.Option = collapse(alt.Option)
		c.WhyNot = collapse(alt.WhyNot)
		c.RevisitIf = collapse(alt.RevisitIf)

		if text := Parse(alt.Option); !text.Empty() {
			units = append(units, unit{text: text, citation: c, polarised: true})
		}
		if text := Parse(alt.WhyNot); !text.Empty() {
			units = append(units, unit{text: text, citation: c, polarised: true, phraseOnly: true, prose: true})
		}
	}
	return units
}

// constraintUnits matches a constraint on its title, its tags and its body,
// one sentence at a time.
//
// The body is read here and nowhere else in the check. dec-0014 bounds the
// expensive read to the constraint set for a reason: a constraint's operative
// reasoning has nowhere structural to live, because the schema gives it no
// alternatives array, while a decision's whole matchable surface is already
// parsed frontmatter. The constraint set is small and closed by construction —
// a new one requires superseding cst-0002's five-kind closure — so the cost
// does not grow with the ledger.
//
// Units here are not polarised. A constraint is a prohibition, not a proposal:
// cst-0004 says dira "never requires ... an account", and a plan conflicts with
// it by proposing an account, not by also refusing one. The plan side still
// carries its polarity, and a plan that disclaims the term contributes nothing.
func constraintUnits(e *ledger.Entry, base Citation) []unit {
	c := base
	c.Basis = BasisConstraint

	heading := e.Title
	if len(e.Tags) > 0 {
		heading += " " + strings.Join(e.Tags, " ")
	}

	units := []unit{{text: Parse(heading), citation: c, prose: true}}
	for _, s := range sentences(e.Body) {
		if text := Parse(s); !text.Empty() {
			units = append(units, unit{text: text, citation: c, prose: true})
		}
	}
	return units
}

// entryDate is the date a conflict message reports beside the state.
//
// updated wins over created where it exists, because the state is what the
// message names and the state is what updated records a change to. Saying an
// entry was "superseded 2026-06-01" when that is the day it was accepted would
// be a false statement about the record.
func entryDate(e *ledger.Entry) string {
	stamp := e.Created
	if e.Updated != "" {
		stamp = e.Updated
	}
	if len(stamp) >= 10 {
		return stamp[:10]
	}
	return stamp
}

// sentences cuts a markdown body into sentence-sized pieces.
//
// It is not a sentence tokeniser. It splits on blank lines and on a period
// followed by whitespace, which over prose written for humans is close enough
// that the difference never reaches a verdict: a mis-split produces two shorter
// units, and a unit's score is compared against the plan independently.
func sentences(body string) []string {
	clean := strings.ReplaceAll(body, "`", "")
	var out []string
	for _, para := range strings.Split(clean, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		start := 0
		for i := 0; i < len(para); i++ {
			if para[i] != '.' || i+1 >= len(para) {
				continue
			}
			if next := para[i+1]; next != ' ' && next != '\n' {
				continue
			}
			if s := strings.TrimSpace(para[start : i+1]); s != "" {
				out = append(out, s)
			}
			start = i + 1
		}
		if s := strings.TrimSpace(para[start:]); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// collapse folds the whitespace a YAML folded scalar leaves behind, so a
// why_not written across four wrapped lines renders as one.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }
