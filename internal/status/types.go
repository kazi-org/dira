// Package status derives dira's six execution-status buckets from the
// ledger, and (E4-L3) from a join against kazi.
//
// This file owns the shapes both sides of that join return. E4-L2 (this lane)
// only ever populates the two ledger-only buckets, ToBePlanned and
// DecisionBlocked, plus the two terminal groups that predate any join at all —
// see ledger.go, blocked.go and terminal.go. E4-L3 adds join.go to this same
// package for the other four buckets and must not edit this file: every
// exported name here, and the Buckets slice's order and length, are fixed so
// that lane needs no conversation with this one.
package status

import "github.com/kazi-org/dira/internal/ledger"

// Bucket is dira's six-value derived-status vocabulary (dec-0004's join
// table, read top to bottom). It shares no value with ledger.State: a
// ledger.State describes the entry FILE (accepted, active, open); a Bucket
// describes where that entry sits once joined against kazi, or — for the two
// rows this lane derives — against nothing but the ledger itself.
type Bucket string

const (
	ToBePlanned      Bucket = "to_be_planned"
	Planned          Bucket = "planned"
	InProgress       Bucket = "in_progress"
	Completed        Bucket = "completed"
	ExecutionBlocked Bucket = "execution_blocked"
	DecisionBlocked  Bucket = "decision_blocked"
)

// Buckets lists the six in dec-0004's table order. len(Buckets) == 6 is
// asserted by TestTypes directly — the CLI (E4-L4) ranges over this slice
// rather than a literal switch, so a seventh bucket cannot be added without
// this test naming it.
var Buckets = []Bucket{ToBePlanned, Planned, InProgress, Completed, ExecutionBlocked, DecisionBlocked}

// Source names what produced a Row's bucket. This lane only ever sets
// SourceLedger; E4-L3's join.go sets the other two. Defined here, not there,
// so E4-L3 has no reason to touch this file.
type Source string

const (
	SourceLedger     Source = "ledger"         // to_be_planned / decision_blocked / terminal — no kazi call made
	SourcePortfolio  Source = "kazi portfolio" // E4-L3
	SourceKaziStatus Source = "kazi status"    // E4-L3
)

// A Row is one entry's derived status.
type Row struct {
	ID     string
	Kind   ledger.Kind
	Title  string
	Bucket Bucket
	Source Source

	// BlockedBy is set only when Bucket == DecisionBlocked: the blocking
	// question's id and title (the lane acc: line's "naming the question's
	// id and title" clause). An entry gated by more than one open question
	// is one Row per question, so BlockedBy is never a slice: two rows
	// sharing an ID is how "neither question is silently dropped" holds.
	BlockedBy *BlockingQuestion

	// Evidence, Ambiguous and Unresolved are E4-L3's fields (Completed
	// evidence-by-reference, and the two non-guess reports). This lane
	// never sets them; they are declared here so the struct shape is fixed
	// once, not extended by E4-L3 in a second file that redeclares Row.
	Evidence   *KaziEvidence
	Ambiguous  *AmbiguousDetail
	Unresolved *UnresolvedDetail
}

// BlockingQuestion names the open question gating a DecisionBlocked row.
type BlockingQuestion struct{ ID, Title string }

// KaziEvidence is E4-L3's Completed evidence, by reference only.
type KaziEvidence struct{ RunID, ReleaseRef string }

// AmbiguousDetail is E4-L3's report of conflicting kazi statuses for one ref.
type AmbiguousDetail struct{ Statuses []string }

// UnresolvedDetail is E4-L3's report of a realized_by ref kazi cannot resolve.
type UnresolvedDetail struct{ Ref, Reason string }

// TerminalGroup is a ledger-only lifecycle end-state that predates any kazi
// join: an intent achieved or abandoned by ledger record alone. Not one of
// dec-0004's six buckets — an achieved intent needs no goal and is not
// "to be planned," so it is reported separately rather than forced into
// either.
type TerminalGroup string

const (
	Achieved  TerminalGroup = "achieved"
	Abandoned TerminalGroup = "abandoned"
)

// A TerminalRow is one intent's terminal group.
type TerminalRow struct {
	ID, Title string
	Group     TerminalGroup
}
