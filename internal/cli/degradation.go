package cli

import (
	"errors"
	"fmt"

	"github.com/kazi-org/dira/internal/kazi"
)

// degradationLines names, in plain human-readable prose, each way kazi's
// whole-snapshot Snapshot() call can fail — dec-0004's "state the limit
// plainly" requirement, five distinct wordings so a reader can tell WHICH
// failure occurred rather than reading one generic disclaimer regardless of
// cause. Checked against .agents/product-marketing.md §10: precise,
// unhyped, no "revolutionary", "seamless", "supercharge", "10x" or
// "AI-powered".
var degradationLines = map[kazi.UnavailableReason]string{
	kazi.ReasonNotOnPath:     "kazi is not installed; execution status is unavailable.",
	kazi.ReasonNonZeroExit:   "kazi exited with an error; execution status is unavailable.",
	kazi.ReasonMalformedJSON: "kazi's output could not be parsed; execution status is unavailable.",
	kazi.ReasonWrongKind:     "kazi returned an unexpected response; execution status is unavailable.",
	kazi.ReasonTimeout:       "kazi did not respond in time; execution status is unavailable.",
}

// DegradationLine renders the one explicit, named-reason unavailability
// line dira map prints when reason names why kazi.Snapshot() failed. A
// reason this table does not recognise (a future UnavailableReason) still
// names itself in the sentence rather than falling back to one generic
// message, so the "distinct wording per reason" property degrades
// gracefully instead of silently collapsing.
func DegradationLine(reason kazi.UnavailableReason) string {
	if line, ok := degradationLines[reason]; ok {
		return line
	}
	return fmt.Sprintf("kazi reported an unrecognised problem (%s); execution status is unavailable.", reason)
}

// degradationReasonFor returns the machine-readable reason string --json
// carries in its "degraded" object — the same kazi.UnavailableReason value
// the text line names, never a paraphrase of it.
func degradationReasonFor(snapErr error) string {
	var unavailable *kazi.Unavailable
	if errors.As(snapErr, &unavailable) {
		return string(unavailable.Reason)
	}
	return "unknown"
}

// degradationLineFor renders snapErr — exactly the error kazi.Snapshot()
// returned — as the one line dira map prints under degradation. snapErr is
// always an *kazi.Unavailable in practice (kazi.Snapshot's own contract);
// anything else still produces a named line rather than a panic or a blank
// one.
func degradationLineFor(snapErr error) string {
	var unavailable *kazi.Unavailable
	if errors.As(snapErr, &unavailable) {
		return DegradationLine(unavailable.Reason)
	}
	return fmt.Sprintf("kazi could not be asked (%v); execution status is unavailable.", snapErr)
}
