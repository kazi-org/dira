package drift

import (
	"fmt"
	"sort"
	"strings"
)

// driftMarker is the only token that means drift — dec-0006 makes orphan the
// one state that counts as such, and it is a token the test can search for
// and find exactly once, on A's line and nowhere else.
const driftMarker = "◆ unexplained work"

// Render takes a Classify result and produces one line per intent, in a
// stable order (by id).
//
// It opens nothing and reads nothing further: every byte it prints comes out
// of the Classification the caller already has, which is the whole reason
// Classify carries a title and a namespace rather than a bare State — a
// Withheld or Broken classification carries no entry to render because
// chain.Resolve returned none for either, so there is nothing here Render
// could leak even if it tried.
func Render(result map[string]Classification) string {
	ids := make([]string, 0, len(result))
	for id := range result {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var b strings.Builder
	for _, id := range ids {
		b.WriteString(renderLine(id, result[id]))
		b.WriteString("\n")
	}
	return b.String()
}

// renderLine is one intent's line. dec-0018's rule governs the withheld
// branch specifically — "withheld reads as neither an error nor a warning" —
// and neither word appears anywhere in this function, in any branch, so a
// caller cannot introduce one by adding a state later without the wording
// review that rule asks for.
func renderLine(id string, c Classification) string {
	switch c.State {
	case Orphan:
		return fmt.Sprintf("%s  %s  %s", id, driftMarker, c.Title)
	case Oriented:
		return fmt.Sprintf("%s  oriented via %s — %s (%s)", id, c.Namespace, c.Title, c.TargetTitle)
	case Withheld:
		return fmt.Sprintf("%s  %s is declared but not readable from here right now — %s", id, c.Namespace, c.Title)
	case Broken:
		return fmt.Sprintf("%s  %s is not a namespace declared anywhere in the chain — %s", id, c.Namespace, c.Title)
	default:
		return fmt.Sprintf("%s  %s", id, c.Title)
	}
}
