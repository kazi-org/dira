package enforcer

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// The message is not a matter of taste.
//
// .agents/product-marketing.md §6 pre-registers this exact terminal block as the
// single most important marketing asset — one clip, under twenty seconds, no
// narration — and README.md and docs/design.md §7 print the same block.
// internal/enforcer/testdata/golden/daemon.txt holds it byte for byte, and a
// diff against it is a break rather than a preference. The voice rules in §10
// apply to every string in this file: precise, unhyped, states its limits. No
// exclamation marks and nothing that sounds like a product page.
//
//	$ dira check "add a background daemon to track run state"
//	✗ conflicts with dec-0060 (accepted 2026-07-03)
//	    rejected alternative: "a daemon"
//	    why_not: violates the single-binary intent (int-0002)
//	    revisit_if: cold-start latency stops being the binding constraint
//	→ supersede dec-0060, or revise the plan

// indent is the four spaces the frozen block uses for a conflict's detail
// lines.
const indent = "    "

// Render writes the human-readable verdict.
//
// Three rules are built in here rather than left to callers, because the
// renderer is the last place they can be enforced and a caller that forgot one
// would leak or mislead silently:
//
//  1. A private entry is cited by ref and never by text (cst-0003), always.
//  2. A revisit_if line appears only where the schema can carry one — on a
//     matched alternative — because docs/design.md §7 is explicit that the
//     message must not imply a revisit condition exists where none can. A
//     constraint and a rejected decision's own subject have no such field, and
//     their escape hatch is the supersede line every block already ends with.
//  3. Where an alternative recorded no revisit_if, the line is printed saying
//     so. Omitting it reads as "never", which is the opposite of the truth.
//
// A fourth rule governs the informational lines: they come first, and they
// never displace the call to action. A notice is context for a verdict, so it
// is read before it; the line that says what to do about a conflict stays last,
// where the frozen block puts it.
//
// A fifth governs the remedy for something this ledger does not own. A citation
// whose id is namespaced came out of a parent, and `dira supersede` refuses a
// namespaced ref on either side at exit 2 before it opens anything
// (cmd/dira/supersede.go's checkRefs, cst-0003 rule 1). So "supersede
// me:cst-0001" would be advice the binary itself rejects, and the line for an
// inherited citation names the ledger that owns the entry instead. See
// inheritedRemedy.
func Render(w io.Writer, v *Verdict) error {
	var b strings.Builder
	for _, n := range v.Notices {
		b.WriteString(noticeLine(n))
	}

	if v.Compliant() {
		fmt.Fprintf(&b, "✓ no conflict with %d enforced %s\n", v.Enforced, plural(v.Enforced, "entry", "entries"))
		_, err := io.WriteString(w, b.String())
		return err
	}

	local := make([]string, 0, len(v.Conflicts))
	inherited := map[string][]string{}
	namespaces := make([]string, 0, len(v.Conflicts))
	for _, c := range v.Conflicts {
		b.WriteString(block(c))

		namespace := namespaceOf(c.Entry)
		if namespace == "" {
			local = append(local, c.Entry)
			continue
		}
		if _, seen := inherited[namespace]; !seen {
			namespaces = append(namespaces, namespace)
		}
		inherited[namespace] = append(inherited[namespace], c.Entry)
	}

	// The local line first and unchanged, because it is the frozen block
	// (.agents/product-marketing.md §6). Parents follow it in citation order,
	// one line each, so a verdict citing both says what to do about each half
	// rather than offering one remedy that is wrong for one of them.
	if len(local) > 0 {
		fmt.Fprintf(&b, "→ supersede %s, or revise the plan\n", joinIDs(local))
	}
	for _, namespace := range namespaces {
		b.WriteString(inheritedRemedy(namespace, inherited[namespace]))
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// namespaceOf returns the parent ledger a citation was inherited from, and the
// empty string for one this ledger owns.
//
// The namespace is the whole of what distinguishes the two here, and that is by
// design rather than for want of a field: internal/enforcer/inherit.go builds an
// inherited citation as `<declared key>:<id>`, and dec-0011 makes that declared
// key the thing a ref resolves through. A renderer that needed a second flag to
// tell them apart would be a renderer a caller could get wrong.
func namespaceOf(entry string) string {
	namespace, _, ok := strings.Cut(entry, ":")
	if !ok {
		return ""
	}
	return namespace
}

// inheritedRemedy is the call to action for citations that came out of a parent.
//
// It names the ledger that owns them, which is the only actionable thing this
// binary can say about them: cst-0003 rule 1 makes inheritance one-way, so
// retiring a parent's entry is done in the parent, and `dira supersede` run here
// refuses a namespaced ref at exit 2 rather than trying. The shape deliberately
// mirrors the local line — the same arrow, the same "or revise the plan" — so a
// reader learns one grammar and the difference between the two is the fact being
// reported rather than a change of voice.
//
// It carries no path, no label and no text from the parent. The namespace is the
// child's own word for that ledger and is already published in the citation
// above it; dec-0011 says a private parent's label must never ship, and a remedy
// line is an output like any other.
func inheritedRemedy(namespace string, ids []string) string {
	return fmt.Sprintf("→ %s %s enforced by the parent ledger %s; supersede %s there, or revise the plan\n",
		joinIDs(ids), plural(len(ids), "is", "are"), namespace, plural(len(ids), "it", "them"))
}

// noticeLine renders one redirect.
//
// It names the replacement and never the entry that was replaced. That is not
// squeamishness about a dead id: the replacement is the entry that is enforced
// now, carries the `supersedes` edge back, and is what `dira why` should be
// pointed at, so it is the only id in the line worth acting on.
//
// The three forms are three different truths, and collapsing them would make
// the line a guess. A replacement that is itself staged or superseded enforces
// nothing, and a state flipped to `superseded` with no `supersedes` edge
// pointing at it names nothing at all — which is exactly the shape of the
// inconsistency qst-0006 found in this repository's own ledger, so the message
// reports it rather than inventing a successor.
func noticeLine(n Notice) string {
	switch {
	case n.SupersededBy == "":
		return "ⓘ this plan matches superseded thinking; the ledger records nothing that replaced it, " +
			"so nothing here is enforced\n"
	case !n.Enforced:
		return fmt.Sprintf("ⓘ this plan matches thinking %s replaced; %s is not enforced either, "+
			"so nothing here is\n", n.SupersededBy, n.SupersededBy)
	default:
		return fmt.Sprintf("ⓘ this plan matches thinking %s replaced; %s is enforced in its place\n",
			n.SupersededBy, n.SupersededBy)
	}
}

// block renders one conflict.
func block(c Conflict) string {
	var b strings.Builder
	fmt.Fprintf(&b, "✗ conflicts with %s (%s)\n", c.Entry, header(c))
	if c.Private {
		// Ref only. The binary cannot tell whether its stdout is a terminal
		// or a pull-request body, so it never has the information that
		// would make quoting the text safe.
		return b.String()
	}

	switch c.Basis {
	case BasisAlternative:
		fmt.Fprintf(&b, "%srejected alternative: %q\n", indent, c.Option)
		fmt.Fprintf(&b, "%swhy_not: %s\n", indent, c.WhyNot)
		fmt.Fprintf(&b, "%srevisit_if: %s\n", indent, revisitLine(c))

	case BasisDecision:
		fmt.Fprintf(&b, "%srejected decision: %q\n", indent, c.Title)

	case BasisConstraint:
		fmt.Fprintf(&b, "%s%s\n", indent, c.Title)
	}
	return b.String()
}

// header is what follows the entry id in parentheses.
//
// A decision carries its date because when it was settled is what makes a
// citation checkable; docs/design.md §7 prints a constraint without one, and a
// standing rule has no equivalent moment to name.
func header(c Conflict) string {
	parts := string(c.State)
	if c.Basis != BasisConstraint && c.Date != "" {
		parts += " " + c.Date
	}
	if c.Private {
		parts += ", private — cited by reference only"
	}
	return parts
}

// revisitLine is the revisit condition, or the explicit statement that none was
// recorded and what to do instead.
func revisitLine(c Conflict) string {
	if c.RevisitIf != "" {
		return c.RevisitIf
	}
	return fmt.Sprintf("none recorded — supersede %s to reopen", c.Entry)
}

// joinIDs lists the entries a plan would have to supersede, in citation order.
func joinIDs(ids []string) string {
	switch len(ids) {
	case 0:
		return ""
	case 1:
		return ids[0]
	default:
		return strings.Join(ids[:len(ids)-1], ", ") + " or " + ids[len(ids)-1]
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// jsonVerdict is the --json contract, pinned by schema/check.schema.json.
//
// It is a separate type from Verdict on purpose. Verdict is dira's internal
// account of a check and may grow a field whenever a lane needs one; this is a
// published surface that a hook parses, so it changes only deliberately and the
// schema is what says so.
type jsonVerdict struct {
	Plan      string         `json:"plan"`
	Verdict   string         `json:"verdict"`
	ExitCode  int            `json:"exit_code"`
	Enforced  int            `json:"enforced_entries"`
	Conflicts []jsonConflict `json:"conflicts"`
	Notices   []jsonNotice   `json:"notices"`
}

// jsonNotice is the machine form of a redirect. It carries the same words the
// human line does and the same omission: the replacement is named, the retired
// entry is not, in both modes, so a hook and a human are looking at the same
// fact rather than at two versions of it.
type jsonNotice struct {
	// SupersededBy is null where the record names no replacement.
	SupersededBy *string `json:"superseded_by"`

	// ReplacementEnforced is false where the replacement is not itself
	// enforcement substrate — including, always, where there is none.
	ReplacementEnforced bool `json:"replacement_enforced"`

	Basis string  `json:"basis"`
	Score float64 `json:"score"`
}

type jsonConflict struct {
	Entry   string `json:"entry"`
	Kind    string `json:"kind"`
	State   string `json:"state"`
	Date    string `json:"date,omitempty"`
	Private bool   `json:"private"`
	Basis   string `json:"basis"`

	// Title, Alternative and WhyNot are absent for a private entry, which is
	// cited by ref and never by text — the same rule the human renderer
	// applies, for the same reason: nothing downstream of this can tell
	// whether the JSON is about to be pasted into a public issue.
	Title       string `json:"title,omitempty"`
	Alternative string `json:"rejected_alternative,omitempty"`
	WhyNot      string `json:"why_not,omitempty"`

	// RevisitIf is null where none is recorded, and null always for a
	// constraint or a rejected decision's own subject, whose kind has no such
	// field to carry one. Supersede is the remedy that always exists.
	RevisitIf *string `json:"revisit_if"`
	Supersede string  `json:"supersede"`

	Score float64 `json:"score"`
}

// RenderJSON writes the machine-readable verdict.
//
// The exit code is in the document as well as on the process, because a caller
// that has already captured stdout should not have to have also captured
// $? to know what it is looking at.
func RenderJSON(w io.Writer, v *Verdict) error {
	out := jsonVerdict{
		Plan:      v.Plan,
		Verdict:   "compliant",
		ExitCode:  ExitCompliant,
		Enforced:  v.Enforced,
		Conflicts: []jsonConflict{},
		Notices:   []jsonNotice{},
	}
	if !v.Compliant() {
		out.Verdict = "conflict"
		out.ExitCode = ExitConflict
	}

	for _, c := range v.Conflicts {
		jc := jsonConflict{
			Entry:     c.Entry,
			Kind:      string(c.Kind),
			State:     string(c.State),
			Date:      c.Date,
			Private:   c.Private,
			Basis:     string(c.Basis),
			Supersede: c.Entry,
			Score:     round(c.Score),
		}
		if !c.Private {
			jc.Title = c.Title
			if c.Basis == BasisAlternative {
				jc.Alternative = c.Option
				jc.WhyNot = c.WhyNot
				if c.RevisitIf != "" {
					revisit := c.RevisitIf
					jc.RevisitIf = &revisit
				}
			}
		}
		out.Conflicts = append(out.Conflicts, jc)
	}

	for _, n := range v.Notices {
		jn := jsonNotice{
			ReplacementEnforced: n.Enforced,
			Basis:               string(n.Basis),
			Score:               round(n.Score),
		}
		if n.SupersededBy != "" {
			by := n.SupersededBy
			jn.SupersededBy = &by
		}
		out.Notices = append(out.Notices, jn)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// round trims the score to three decimals. The value is advisory, and emitting
// a float's full mantissa would invite a caller to depend on a digit that the
// next threshold sweep moves.
func round(f float64) float64 {
	return float64(int64(f*1000+0.5)) / 1000
}
