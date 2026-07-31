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
func Render(w io.Writer, v *Verdict) error {
	if v.Compliant() {
		_, err := fmt.Fprintf(w, "✓ no conflict with %d enforced %s\n", v.Enforced, plural(v.Enforced, "entry", "entries"))
		return err
	}

	var b strings.Builder
	ids := make([]string, 0, len(v.Conflicts))
	for _, c := range v.Conflicts {
		b.WriteString(block(c))
		ids = append(ids, c.Entry)
	}
	fmt.Fprintf(&b, "→ supersede %s, or revise the plan\n", joinIDs(ids))

	_, err := io.WriteString(w, b.String())
	return err
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
