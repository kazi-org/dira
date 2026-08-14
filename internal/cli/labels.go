package cli

import "github.com/kazi-org/dira/internal/status"

// bucketLabels is the single source of truth for every literal string this
// package prints for a status.Bucket — E4-L5's absence assertions and this
// lane's own golden files both read from it rather than six inline literals
// scattered through the renderer.
//
// InProgress and ExecutionBlocked's labels are fixed by E4-L5's locked acc:
// line, not by this lane's preference — see docs/plan/tasks/E4-L4.md's "What
// is already known" table for the full citation of each choice.
var bucketLabels = map[status.Bucket]string{
	status.ToBePlanned:      "to be planned",
	status.Planned:          "planned",
	status.InProgress:       "in progress",
	status.Completed:        "converged",
	status.ExecutionBlocked: "execution-blocked",
	status.DecisionBlocked:  "blocks this",
}

// Label returns bucket's committed CLI string, or "" for the zero value (an
// entry dec-0004's table does not cover).
func Label(bucket status.Bucket) string {
	return bucketLabels[bucket]
}

// blockedRowLabel is the DecisionBlocked label for the BLOCKED entry's own
// row — as opposed to the blocking question's own line, which keeps
// docs/design.md §6.4's "⛔ blocks <id>" form unchanged. It names the
// blocking question's id so a reader lands on the blocked entry's row and
// can see which question without following a second edge by hand.
func blockedRowLabel(q *status.BlockingQuestion) string {
	if q == nil {
		return Label(status.DecisionBlocked)
	}
	return "blocked by " + q.ID
}
