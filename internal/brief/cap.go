package brief

import "fmt"

// idiomaticMax is dec-0006's own idiomatic range for how many active intents
// a legible ledger holds: "three to seven of them and never more" is the
// idiom, not an enforced ceiling. A ledger holding more is the drift case
// dec-0006 says dira's job is to show, not fix: this package names it with a
// warning and renders every one of them anyway — nothing here rejects or
// hides the entry past the idiom.
const idiomaticMax = 7

// capWarningMarker is a token the warning line always carries, distinct from
// any per-intent line, so a caller can find it without depending on the exact
// wording.
const capWarningMarker = "⚠ idiomatic range"

// capWarning is the one line named above idiomaticMax active intents for one
// ancestor, or "" when its count is within the idiom.
func capWarning(namespace string, count int) string {
	if count <= idiomaticMax {
		return ""
	}
	return fmt.Sprintf("%s  %s — %d active, three to seven is idiomatic, not a limit\n", namespace, capWarningMarker, count)
}
