package status

// E4-L3: the kazi join. Populates the four buckets E4-L2 leaves alone
// (ToBePlanned and DecisionBlocked are ledger.go/blocked.go's; the two
// terminal groups predate any join at all), plus Evidence, Ambiguous and
// Unresolved — declared in types.go, first set here.
//
// The deviation the integrator's brief records: the lane doc's fixed
// signature names the payload type kazi.Snapshot; the real E4-L1 type is
// kazi.Portfolio (Go does not allow a type and its constructing function,
// kazi.Snapshot(ctx), to share one identifier in the same package — see that
// package's own doc comment on Portfolio). Every reference below uses
// *kazi.Portfolio.

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/kazi-org/dira/internal/kazi"
	"github.com/kazi-org/dira/internal/ledger"
)

// StatusFunc is kazi.Status's shape, injected rather than called directly so
// this lane's own tests never shell a binary and so a caller (E4-L4) can wrap
// it with a call counter to enforce a fan-out bound.
type StatusFunc func(ctx context.Context, ref string) (*kazi.RunStatus, *kazi.ProposalStatus, error)

// MaxStatusCalls bounds Join's fan-out. At kazi's measured ≈0.65s per Status
// call (docs/plan/lanes/E4.md point 4), an unbounded join over N realized_by
// edges is O(N) wall-clock against a human waiting on a terminal command.
const MaxStatusCalls = 25

// realizedByPattern is schema/entry.schema.json's $defs/edge shape for a
// realized_by target's two legal forms: "kazi:prop-<ref>" and
// "kazi:goal-<id>".
var realizedByPattern = regexp.MustCompile(`^kazi:(prop|goal)-(.+)$`)

// parseRealizedBy splits a realized_by edge target into its verb (prop or
// goal) and the ref/id following it. A target matching neither shape — no
// kazi: prefix, or a third verb — is a named parse error, never a
// silently-empty result.
func parseRealizedBy(target string) (verb, ref string, err error) {
	m := realizedByPattern.FindStringSubmatch(target)
	if m == nil {
		return "", "", fmt.Errorf("status: realized_by target %q is neither kazi:prop-<ref> nor kazi:goal-<id>", target)
	}
	return m[1], m[2], nil
}

// resolveGoalID turns a parsed realized_by target into the kazi goal id it
// names. A goal-<id> target names its goal directly, no lookup required —
// "goal-" is dira's own schema marker and by_repo's GoalRef values carry no
// such prefix (e.g. "warnings-clean"), so it is stripped before use as the
// join key. A prop-<ref> target resolves through snap.Planned's
// proposal_ref -> goal_id bridge, and the comparison is against the WHOLE
// "prop-<ref>" string, not ref alone: unlike goal ids, kazi's own
// proposal_ref values already carry "prop-" as a literal part of their own
// value (verified against the real fixture: ProposalRef is "prop-e45", never
// bare "e45"), so dira's schema marker and kazi's own ref happen to read
// identically here — reconstructing "prop-"+ref is what makes the match land
// on kazi's actual field. A proposal absent from Planned — rejected
// upstream, or never proposed; portfolio drops rejected proposals entirely
// (lane doc point 5) — is reported unresolved by the caller rather than
// silently treated as ToBePlanned: that would conflate "kazi never heard of
// this ref" with "no realized_by edge exists at all", the two things E4-L2
// and this lane exist to keep distinct.
func resolveGoalID(verb, ref string, snap *kazi.Portfolio) (goalID string, ok bool) {
	if verb == "goal" {
		return ref, true
	}
	full := "prop-" + ref
	for _, p := range snap.Planned {
		if p.ProposalRef == full {
			return p.GoalID, true
		}
	}
	return "", false
}

// runsForGoal flattens every by_repo run, across every repo and bucket key,
// whose GoalRef matches goalID. by_repo is undeduped and carries no
// timestamp (lane doc point 2), so this is every run kazi has ever recorded
// for the goal — never fewer, because nothing on the wire says which is
// current.
func runsForGoal(snap *kazi.Portfolio, goalID string) []kazi.RepoRun {
	var out []kazi.RepoRun
	for _, buckets := range snap.ByRepo {
		for _, runs := range buckets {
			for _, r := range runs {
				if r.GoalRef == goalID {
					out = append(out, r)
				}
			}
		}
	}
	return out
}

// blockedForGoal returns every top-level blocked[] entry naming goalID.
func blockedForGoal(snap *kazi.Portfolio, goalID string) []kazi.BlockedEntry {
	var out []kazi.BlockedEntry
	for _, b := range snap.Blocked {
		if b.GoalRef == goalID {
			out = append(out, b)
		}
	}
	return out
}

// bucketForCause maps every blocked[].cause value onto ExecutionBlocked. Kept
// as its own function, over a switch with a default, so TestJoinBucketMapping
// can drive it over all four Cause values in one table and a fifth cause
// added later to internal/kazi fails loudly here instead of falling through
// silently.
func bucketForCause(kazi.Cause) Bucket { return ExecutionBlocked }

// mapRunBucket maps a single by_repo run's RepoBucket and raw Status onto
// dec-0004's vocabulary. A "terminated" raw status is never trusted:
// portfolio.ex's bucket/2 folds it into RepoInProgress regardless of what
// actually happened (lane doc point 3), so this reports ok=false rather than
// InProgress and leaves the caller to decide the row's final resting bucket.
func mapRunBucket(bucket kazi.RepoBucket, rawStatus string) (b Bucket, ok bool) {
	if rawStatus == "terminated" {
		return "", false
	}
	switch bucket {
	case kazi.RepoComplete:
		return Completed, true
	case kazi.RepoStuck:
		return ExecutionBlocked, true
	case kazi.RepoInProgress:
		return InProgress, true
	}
	return "", false
}

// distinctStatuses returns the sorted, deduplicated set of raw Status
// strings across runs — sorted so two calls over the same set agree, not
// because the acc line's "asserted as a set (order-independent)" requires it.
func distinctStatuses(runs []kazi.RepoRun) []string {
	seen := make(map[string]bool, len(runs))
	for _, r := range runs {
		seen[r.Status] = true
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// runIDForComplete returns the RunID of the first run among runs whose
// RepoBucket is RepoComplete, or "" if none — the by-reference evidence
// dec-0004 allows a Completed row resolved via the multi-run fan-out to
// carry, since no single by_repo entry alone was trusted enough to resolve
// the bucket on its own.
func runIDForComplete(runs []kazi.RepoRun) string {
	for _, r := range runs {
		if r.Bucket == kazi.RepoComplete {
			return r.RunID
		}
	}
	return ""
}

// resolveGoal is Join's per-goal decision: given every fact the portfolio
// carries about goalID, and — only when genuinely needed — one bounded call
// to statusFn, produce the bucket, its source, and whatever evidence or
// non-answer accompanies it.
//
// The branches, in the order they are tried:
//
//  1. No by_repo runs at all. If the goal appears in blocked[], its cause
//     resolves the bucket directly (this is the DAG-blocked-before-a-run-ever-
//     started shape: a goal can be blocked by an unmet dependency with no
//     run_id yet). Otherwise kazi has nothing to say and the row is
//     unresolved.
//  2. Exactly one by_repo run, not "terminated": mapRunBucket resolves it
//     directly, at zero cost — a genuinely single-run goal has nothing to
//     disambiguate.
//  3. Exactly one by_repo run, "terminated": the raw status is untrustworthy
//     (mapRunBucket's ok=false) and there is nothing else recorded to check
//     it against, so the row is reported unresolved rather than guessed.
//  4. More than one by_repo run: the disagreement this lane exists to catch.
//     statusFn is the only trustworthy answer (never "take the first
//     entry" — see the lane doc's "specific thing not to do"), bounded by
//     MaxStatusCalls; an *kazi.Unavailable from statusFn itself (a real,
//     distinct failure mode from the whole-snapshot degradation T6 covers)
//     reports Ambiguous, naming every distinct raw Status observed.
func resolveGoal(ctx context.Context, snap *kazi.Portfolio, goalID string, statusFn StatusFunc, calls *int) (Bucket, Source, *KaziEvidence, *AmbiguousDetail, *UnresolvedDetail) {
	runs := runsForGoal(snap, goalID)

	if len(runs) == 0 {
		if blocked := blockedForGoal(snap, goalID); len(blocked) > 0 {
			return bucketForCause(blocked[0].Cause), SourcePortfolio, nil, nil, nil
		}
		return "", SourcePortfolio, nil, nil, &UnresolvedDetail{
			Ref: goalID, Reason: "kazi has no run or blocked record for this goal",
		}
	}

	if len(runs) == 1 {
		run := runs[0]
		if b, ok := mapRunBucket(run.Bucket, run.Status); ok {
			var ev *KaziEvidence
			if b == Completed {
				ev = &KaziEvidence{RunID: run.RunID}
			}
			return b, SourcePortfolio, ev, nil, nil
		}
		// The only false from mapRunBucket, given the closed three-value
		// RepoBucket enum, is the "terminated" raw status.
		return "", SourcePortfolio, nil, nil, &UnresolvedDetail{
			Ref: goalID, Reason: "the only recorded run terminated without completing",
		}
	}

	// Multi-run: the disagreement itself is the signal. Resolving it by
	// picking a by_repo entry (e.g. the first) would pass any fixture that
	// happens to be single-run and be wrong on the real machine's majority
	// multi-run case — the one thing this branch must never do.
	if *calls >= MaxStatusCalls {
		return "", SourcePortfolio, nil, &AmbiguousDetail{Statuses: []string{
			fmt.Sprintf("fan-out bound reached (MaxStatusCalls=%d); not queried", MaxStatusCalls),
		}}, nil
	}
	*calls++
	runStatus, propStatus, err := statusFn(ctx, goalID)
	if err != nil {
		var unavailable *kazi.Unavailable
		if errors.As(err, &unavailable) {
			return "", SourcePortfolio, nil, &AmbiguousDetail{Statuses: distinctStatuses(runs)}, nil
		}
		return "", SourcePortfolio, nil, nil, &UnresolvedDetail{Ref: goalID, Reason: err.Error()}
	}
	switch {
	case runStatus != nil && runStatus.Status == "converged":
		return Completed, SourceKaziStatus, &KaziEvidence{
			RunID:      runIDForComplete(runs),
			ReleaseRef: runStatus.ReleaseRef,
		}, nil, nil
	case runStatus != nil:
		return InProgress, SourceKaziStatus, nil, nil, nil
	default:
		_ = propStatus // a multi-run goal resolving to a proposal is not a shape this join expects
		return "", SourcePortfolio, nil, nil, &UnresolvedDetail{
			Ref: goalID, Reason: "kazi status reported a proposal for a goal with recorded runs",
		}
	}
}

// Join resolves every entry in candidates (accepted decisions and active
// intents carrying a realized_by edge — the caller, E4-L4, selects them via
// index.Selector{WithEdge: ledger.EdgeRealizedBy}) against snap. snapErr, when
// non-nil, is exactly the error kazi.Snapshot() returned (an *kazi.Unavailable)
// — Join never receives a snapshot it should not trust, because E4-L1 already
// enforces that at the source.
//
// When snapErr is non-nil, every candidate resolves to the unresolved zero
// value naming snapErr's reason, and statusFn is never called — E4-L2-T3
// already excludes realized_by-carrying entries from ToBePlanned for exactly
// this reason; this function's job is not backfilling them into something
// equally wrong once kazi cannot even be asked for a portfolio.
func Join(ctx context.Context, candidates []*ledger.Entry, snap *kazi.Portfolio, snapErr error, statusFn StatusFunc) ([]Row, error) {
	rows := make([]Row, 0, len(candidates))
	calls := 0

	for _, entry := range candidates {
		target, err := firstRealizedByTarget(entry)
		if err != nil {
			return nil, fmt.Errorf("status: %s: %w", entry.ID, err)
		}

		if snapErr != nil {
			reason := snapErr.Error()
			var unavailable *kazi.Unavailable
			if errors.As(snapErr, &unavailable) {
				reason = string(unavailable.Reason)
			}
			rows = append(rows, Row{
				ID: entry.ID, Kind: entry.Kind, Title: entry.Title,
				Unresolved: &UnresolvedDetail{Ref: target, Reason: reason},
			})
			continue
		}

		verb, ref, err := parseRealizedBy(target)
		if err != nil {
			return nil, fmt.Errorf("status: %s: %w", entry.ID, err)
		}
		goalID, ok := resolveGoalID(verb, ref, snap)
		if !ok {
			rows = append(rows, Row{
				ID: entry.ID, Kind: entry.Kind, Title: entry.Title,
				Unresolved: &UnresolvedDetail{
					Ref:    target,
					Reason: "proposal not found in kazi's planned list (rejected upstream, or never proposed)",
				},
			})
			continue
		}

		bucket, source, evidence, ambiguous, unresolved := resolveGoal(ctx, snap, goalID, statusFn, &calls)
		rows = append(rows, Row{
			ID: entry.ID, Kind: entry.Kind, Title: entry.Title,
			Bucket: bucket, Source: source,
			Evidence: evidence, Ambiguous: ambiguous, Unresolved: unresolved,
		})
	}
	return rows, nil
}

// firstRealizedByTarget returns the To field of e's first realized_by edge.
// Join's caller selects candidates that carry one; an entry with none is a
// caller error, reported rather than silently skipped.
func firstRealizedByTarget(e *ledger.Entry) (string, error) {
	for _, edge := range e.Edges {
		if edge.Type == ledger.EdgeRealizedBy {
			return edge.To, nil
		}
	}
	return "", fmt.Errorf("entry carries no realized_by edge")
}

// Totals reads dec-0004's roll-up counts directly from snap.TotalsRows and
// never re-derives them by summing Planned/Todo/Blocked buckets (dec-0008
// point 5 — that sum double-counts, because planned overlaps both todo and
// live runs).
func Totals(snap *kazi.Portfolio) []kazi.TotalsRow {
	return snap.TotalsRows
}
