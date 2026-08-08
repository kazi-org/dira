package enforcer

// The exit-code contract, kept beside the verdict it describes rather than in
// cmd/dira, because it is what a hook branches on and it must be one number in
// one place.
//
// docs/plan/lanes/E3.md fixes it: 0 compliant, 2 at least one cited conflict, 1
// reserved for dira's own errors. Telling 1 from 2 is the entire reason the
// pre-plan seam works — a hook has to distinguish "you are contradicting
// yourself", which is a verdict it should surface and stop on, from "dira is
// broken", which it must fail open on rather than take the session down with
// it. A check that returned 1 for a conflict would make every hook caller
// choose between ignoring real contradictions and breaking on a missing
// ledger.
const (
	// ExitCompliant is the plan contradicting nothing the ledger enforces.
	ExitCompliant = 0

	// ExitDiraError is dira's own failure: an unreadable ledger, a bad flag.
	// It is never a verdict about the plan.
	ExitDiraError = 1

	// ExitConflict is at least one cited conflict.
	ExitConflict = 2
)
