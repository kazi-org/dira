//go:build perf

package perf

import "time"

// This file is the only place in internal/perf where a duration constant is
// written. The check is one line:
//
//	grep -l 'Millisecond' internal/perf/*.go   # must list budget.go and nothing else
//
// Two reasons, and the second is the load-bearing one. A budget scattered across
// the tests that assert it is a budget nobody can read off in one place, and a
// number that appears in three files gets edited in one of them. And E1-L6-T2
// asserts the warm and reindex budgets on this same harness: declaring all four
// here, up front, is what lets T2 add two test files without touching this one,
// so the two tasks cannot collide in a diff.
//
// Raising a number in this file is a decision about int-0002, not a fix for a red
// run. If the budget cannot be met, that is E1-L6-T5's finding and E1-L6-T6's
// ledger entry.

const (
	// coldMedianBudget is int-0002's number, unmodified: "a hook invocation
	// well under 100ms", measured as the median of N>=20 spawns of the
	// shipped binary over the 200-entry fixture with .dira/cache removed
	// before each one.
	//
	// It is not derived from a measurement and deliberately so — it is the
	// achievement condition the intent was written with, and the measurement
	// is what is judged against it rather than the other way round.
	coldMedianBudget = 100 * time.Millisecond

	// coldMaxBudget was REMOVED by dec-0026, after the perf gate's first run on
	// real CI hardware. It asserted that no single cold run may exceed 150ms,
	// and on ubuntu-latest the slowest of twenty spawns came in at 191ms while
	// the median passed.
	//
	// At n=20 the maximum IS the single worst observation, with no averaging
	// behind it — the noisiest statistic in the set, and the one most sensitive
	// to a descheduled spawn. A 191ms sample is not evidence that a hook
	// invocation is perceptible; it is evidence that one spawn in twenty was
	// descheduled. int-0002's number is unchanged; the statistic it is read from
	// is now the median alone.
	//
	// The cost is real and accepted, not overlooked: a tail regression can hide
	// behind a healthy median. What partly covers it is that the perf job
	// publishes the full sorted sample list to the CI step summary on every run,
	// so a widening tail is visible to anyone who looks even though nothing
	// fails on it. Do not reintroduce a single-run ceiling without the measured
	// distribution dec-0026's revisit_if asks for.

	// warmBudget is the ceiling for the common case: the cache exists and
	// reconciles clean, which is what every hook invocation after the first
	// one pays. E1-L6-T2 asserts it; it is declared here so T2 never edits
	// this file.
	//
	// Derivation, and it is a subtraction rather than a guess. E1-L6-T1
	// measured all three of these through the same harness on one machine, so
	// the spawn floor cancels: `dira version` — a binary that reads no ledger
	// at all — had a median of 155.4ms, warm `dira brief --context --chain`
	// 175.9ms, and cold 260.4ms. The ledger work a WARM invocation adds over
	// the spawn floor is therefore about 21ms, which agrees with E1-L5's
	// 17.3ms in-process figure (docs/decisions-pending/E1-L5-report.md §4).
	//
	// Those absolute numbers are from a machine at load average 38 on four
	// cores and are useless as ceilings; the 21ms difference is not, because
	// contention inflates both arms. So: a spawn floor of about 10ms on idle
	// hardware plus 21ms of warm read is roughly 30ms, and this budget is
	// that with a 2.5x factor. It stays well under coldMedianBudget on
	// purpose — a warm invocation that costs as much as a cold one has lost
	// the cache, and a warm ceiling equal to the cold one could not say so.
	warmBudget = 80 * time.Millisecond

	// reindexBudget is the ceiling for `dira reindex` over the 200-entry
	// fixture with the cache removed first — every entry file read, parsed and
	// written into a database created from nothing. E1-L6-T2 asserts it.
	//
	// Derivation, by the same subtraction. On the machine above, cold
	// `dira reindex` had a median of 231.6ms against the 155.4ms spawn floor,
	// so the indexing work itself is about 76ms — consistent with E1-L5's
	// 61.2ms in-process cold figure, which also included the brief render.
	// A spawn floor of about 10ms on idle hardware plus 76ms is roughly 86ms,
	// and this budget is that with a headroom factor of 4.5x.
	//
	// Deliberately looser than either brief budget, and the reason is worth
	// stating rather than being read as sloppiness: reindex is not on the hook
	// path. Nothing a user waits for runs it, so a tight ceiling here buys
	// regression detection rather than latency, and a tight number over a
	// command that walks 200 files would spend its life as a flake — and a
	// gate disabled after three false alarms protects nothing.
	reindexBudget = 400 * time.Millisecond
)

// A budget is one ceiling, its name and what it is asserted over. The table
// exists so a test can publish every number this lane holds without reaching for
// a constant it does not own, and so a constant declared here for a later task is
// referenced from the day it lands rather than sitting unused.
type budget struct {
	name  string
	limit time.Duration
	what  string
}

// budgets is the whole table, in declaration order.
func budgets() []budget {
	return []budget{
		{"coldMedianBudget", coldMedianBudget, "median of N>=20 cold `dira brief --context --chain` spawns"},
		{"warmBudget", warmBudget, "median of N>=20 warm `dira brief --context --chain` spawns (E1-L6-T2)"},
		{"reindexBudget", reindexBudget, "median of N>=20 cold `dira reindex` spawns (E1-L6-T2)"},
	}
}
