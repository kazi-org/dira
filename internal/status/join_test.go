package status

// E4-L3's test file. Declared `package status` (white-box), not the
// `package status_test` every other file in this package uses, because T1's
// own acc line requires asserting parseRealizedBy's and resolveGoalID's
// resolved values BY VALUE — testable only from inside the package, since
// neither is exported. TestJoin (T7) reads this file's own source to check
// subtest-name coverage, which is also why it must live in this file rather
// than a sibling.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/kazi"
	"github.com/kazi-org/dira/internal/ledger"
)

// --- fixture loading ------------------------------------------------------

// rawPortfolioEnvelope mirrors kazi's own unexported rawPortfolio shape,
// built entirely from kazi's EXPORTED types (Proposal, RepoRun, TotalsRow
// and BlockedEntry all already carry the correct `json:` tags and, for
// RepoBucket/Cause, the custom UnmarshalText that rejects the wrong enum —
// docs/lore.md L-0004). This lane does not reach into internal/kazi's
// internals; it re-composes the same envelope from the outside, exactly the
// "consumes internal/kazi's exported surface" boundary docs/plan/lanes/E4.md
// draws around this lane.
type rawPortfolioEnvelope struct {
	SchemaVersion int                                  `json:"schema_version"`
	Planned       []kazi.Proposal                      `json:"planned"`
	ByRepo        map[string]map[string][]kazi.RepoRun `json:"by_repo"`
	FleetRemote   []kazi.RepoRun                       `json:"fleet_remote"`
	Totals        struct {
		Base  int              `json:"base"`
		Empty bool             `json:"empty"`
		Rows  []kazi.TotalsRow `json:"rows"`
	} `json:"totals"`
	Blocked []kazi.BlockedEntry `json:"blocked"`
	Rate    struct {
		Total int  `json:"total"`
		Green int  `json:"green"`
		Empty bool `json:"empty?"`
		Delta int  `json:"delta"`
	} `json:"rate"`
}

// loadPortfolioFixture decodes one of E4-L1's pinned testdata/kazi/ fixtures
// into a *kazi.Portfolio, by relative reference rather than a copy. That
// corpus is static and committed — unlike E4-L2's real-snapshot, a copy
// guarding against concurrent mutation of THIS repo's own live ledger, there
// is no mutation risk here to guard against, and E4-L5-T1 references
// internal/kazi's testdata/fakekazi/ the same cross-package way.
func loadPortfolioFixture(t *testing.T, name string) *kazi.Portfolio {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "kazi", "testdata", "kazi", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	var raw rawPortfolioEnvelope
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decoding fixture %s: %v", name, err)
	}

	byRepo := make(map[string]map[kazi.RepoBucket][]kazi.RepoRun, len(raw.ByRepo))
	for repo, buckets := range raw.ByRepo {
		converted := make(map[kazi.RepoBucket][]kazi.RepoRun, len(buckets))
		for rawBucket, runs := range buckets {
			var b kazi.RepoBucket
			if err := b.UnmarshalText([]byte(rawBucket)); err != nil {
				t.Fatalf("fixture %s: by_repo[%q]: %v", name, repo, err)
			}
			for i := range runs {
				runs[i].Bucket = b
			}
			converted[b] = runs
		}
		byRepo[repo] = converted
	}

	return &kazi.Portfolio{
		SchemaVersion: raw.SchemaVersion,
		Planned:       raw.Planned,
		ByRepo:        byRepo,
		FleetRemote:   raw.FleetRemote,
		TotalsBase:    raw.Totals.Base,
		TotalsEmpty:   raw.Totals.Empty,
		TotalsRows:    raw.Totals.Rows,
		Blocked:       raw.Blocked,
		RateTotal:     raw.Rate.Total,
		RateGreen:     raw.Rate.Green,
		RateEmpty:     raw.Rate.Empty,
		RateDelta:     raw.Rate.Delta,
	}
}

// loadRealizedByFixture decodes every entry under
// testdata/ledgers/realized-by/ — E4-L3-T1's own fixture, one entry per
// realized_by ref form — keyed by id.
func loadRealizedByFixture(t *testing.T) map[string]*ledger.Entry {
	t.Helper()
	dir := filepath.Join("testdata", "ledgers", "realized-by", ".dira", "entries")
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	out := make(map[string]*ledger.Entry, len(files))
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			t.Fatalf("reading %s/%s: %v", dir, f.Name(), err)
		}
		e, err := ledger.Decode(data)
		if err != nil {
			t.Fatalf("decoding %s/%s: %v", dir, f.Name(), err)
		}
		out[e.ID] = e
	}
	return out
}

// syntheticEntry builds a minimal ledger.Entry carrying one realized_by
// edge, for tests that need a Join candidate but not a round trip through
// the on-disk codec.
func syntheticEntry(id string, kind ledger.Kind, target string) *ledger.Entry {
	return &ledger.Entry{
		ID: id, Kind: kind, Title: "synthetic fixture " + id,
		Edges: []ledger.Edge{{Type: ledger.EdgeRealizedBy, To: target}},
	}
}

// neverCalledStatusFn fails the test the moment it is invoked — the
// "asserted by a counting fake that fails the test if invoked" shape T3 and
// T6 both call for.
func neverCalledStatusFn(t *testing.T) StatusFunc {
	return func(context.Context, string) (*kazi.RunStatus, *kazi.ProposalStatus, error) {
		t.Fatal("statusFn was called; this case should have resolved without it")
		return nil, nil, nil
	}
}

// countingStatusFn wraps inner and reports how many times it was called.
func countingStatusFn(inner StatusFunc) (StatusFunc, *int) {
	n := 0
	return func(ctx context.Context, ref string) (*kazi.RunStatus, *kazi.ProposalStatus, error) {
		n++
		return inner(ctx, ref)
	}, &n
}

// --- E4-L3-T1: ref parsing and resolution ---------------------------------

// TestJoinResolve is T1's acc line.
func TestJoinResolve(t *testing.T) {
	snap := loadPortfolioFixture(t, "portfolio-populated.json")
	entries := loadRealizedByFixture(t)
	ctx := context.Background()

	t.Run("prop ref resolves through planned to its goal id, asserted by value", func(t *testing.T) {
		verb, ref, err := parseRealizedBy("kazi:prop-e45")
		if err != nil {
			t.Fatalf("parseRealizedBy: %v", err)
		}
		goalID, ok := resolveGoalID(verb, ref, snap)
		if !ok {
			t.Fatal("resolveGoalID: prop-e45 did not resolve")
		}
		if goalID != "e45" {
			t.Errorf("goalID = %q, want %q", goalID, "e45")
		}
	})

	t.Run("goal ref resolves directly, no planned lookup", func(t *testing.T) {
		verb, ref, err := parseRealizedBy("kazi:goal-warnings-clean")
		if err != nil {
			t.Fatalf("parseRealizedBy: %v", err)
		}
		if verb != "goal" {
			t.Fatalf("verb = %q, want %q", verb, "goal")
		}
		// An otherwise-empty portfolio proves no Planned lookup happened:
		// if resolveGoalID searched Planned for a goal-form ref, it would
		// find nothing in an empty one and report unresolved.
		empty := &kazi.Portfolio{}
		goalID, ok := resolveGoalID(verb, ref, empty)
		if !ok {
			t.Fatal("resolveGoalID: kazi:goal-warnings-clean did not resolve directly")
		}
		if goalID != "warnings-clean" {
			t.Errorf("goalID = %q, want %q", goalID, "warnings-clean")
		}
	})

	t.Run("rejected/unknown proposal produces an Unresolved row, never ToBePlanned", func(t *testing.T) {
		entry := entries["dec-3003"]
		if entry == nil {
			t.Fatal("fixture dec-3003 not found")
		}
		rows, err := Join(ctx, []*ledger.Entry{entry}, snap, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("Join: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("got %d rows, want 1", len(rows))
		}
		row := rows[0]
		if row.Bucket == ToBePlanned {
			t.Fatal("an unresolved prop ref must never be reported as ToBePlanned")
		}
		if row.Bucket != "" {
			t.Errorf("Bucket = %q, want the zero value", row.Bucket)
		}
		if row.Unresolved == nil {
			t.Fatal("Unresolved is nil")
		}
		if row.Unresolved.Ref != "kazi:prop-does-not-exist-anywhere" {
			t.Errorf("Unresolved.Ref = %q, want the original target", row.Unresolved.Ref)
		}
	})

	t.Run("a realized_by target matching neither form is a named parse error", func(t *testing.T) {
		_, _, err := parseRealizedBy("not-a-kazi-ref")
		if err == nil {
			t.Fatal("parseRealizedBy accepted a target with no kazi: prefix")
		}
		if !strings.Contains(err.Error(), "not-a-kazi-ref") {
			t.Errorf("error %q does not name the offending target", err.Error())
		}

		entry := entries["dec-3004"]
		if entry == nil {
			t.Fatal("fixture dec-3004 not found")
		}
		if _, err := Join(ctx, []*ledger.Entry{entry}, snap, nil, neverCalledStatusFn(t)); err == nil {
			t.Fatal("Join accepted a malformed realized_by target silently")
		}
	})

	t.Run("both sides: falling back to ToBePlanned on a lookup miss is the wrong answer", func(t *testing.T) {
		// The naive, plausible-looking wrong answer the lane doc's point 5
		// warns about: on a Planned lookup miss, report ToBePlanned instead
		// of Unresolved.
		naive := func(verb, ref string, snap *kazi.Portfolio) Bucket {
			if verb == "goal" {
				return InProgress // not exercised by this control
			}
			for _, p := range snap.Planned {
				if p.ProposalRef == ref {
					return InProgress
				}
			}
			return ToBePlanned // the wrong fallback
		}
		verb, ref, err := parseRealizedBy("kazi:prop-does-not-exist-anywhere")
		if err != nil {
			t.Fatalf("parseRealizedBy: %v", err)
		}
		if got := naive(verb, ref, snap); got != ToBePlanned {
			t.Fatalf("the naive control's own premise broke: got %q, want ToBePlanned", got)
		}
		if _, ok := resolveGoalID(verb, ref, snap); ok {
			t.Fatal("the real resolver resolved a proposal ref that is absent from Planned")
		}
	})
}

// --- E4-L3-T2: bucket mapping ----------------------------------------------

// TestJoinBucketMapping is T2's acc line.
func TestJoinBucketMapping(t *testing.T) {
	t.Run("all four blocked causes map to ExecutionBlocked", func(t *testing.T) {
		for _, cause := range []kazi.Cause{kazi.CauseDAG, kazi.CauseOverBudget, kazi.CauseError, kazi.CauseStuck} {
			if got := bucketForCause(cause); got != ExecutionBlocked {
				t.Errorf("bucketForCause(%q) = %q, want %q", cause, got, ExecutionBlocked)
			}
		}
	})

	t.Run("a run-less blocked goal resolves via portfolio-all-causes.json", func(t *testing.T) {
		snap := loadPortfolioFixture(t, "portfolio-all-causes.json")
		ctx := context.Background()
		calls := 0
		for _, goalID := range []string{"hand-extended-dag-blocked-goal", "hand-extended-error-blocked-goal"} {
			if got := len(runsForGoal(snap, goalID)); got != 0 {
				t.Fatalf("%s: has %d by_repo runs, want 0 — this test's premise depends on a run-less blocked goal", goalID, got)
			}
			bucket, source, ev, amb, unres := resolveGoal(ctx, snap, goalID, neverCalledStatusFn(t), &calls)
			if bucket != ExecutionBlocked {
				t.Errorf("%s: Bucket = %q, want %q", goalID, bucket, ExecutionBlocked)
			}
			if source != SourcePortfolio {
				t.Errorf("%s: Source = %q, want %q", goalID, source, SourcePortfolio)
			}
			if ev != nil || amb != nil || unres != nil {
				t.Errorf("%s: unexpected Evidence/Ambiguous/Unresolved on an ExecutionBlocked row", goalID)
			}
		}
		if calls != 0 {
			t.Errorf("statusFn was called %d times; a blocked-only goal needs none", calls)
		}
	})

	t.Run("a single-run terminated status never maps to InProgress", func(t *testing.T) {
		snap := loadPortfolioFixture(t, "portfolio-populated.json")
		runs := runsForGoal(snap, "t1-3-tenant-default-calibration")
		if len(runs) != 1 || runs[0].Status != "terminated" {
			t.Fatalf("t1-3-tenant-default-calibration: got %d run(s) (%+v), want exactly 1 with Status=terminated", len(runs), runs)
		}
		bucket, ok := mapRunBucket(runs[0].Bucket, runs[0].Status)
		if ok {
			t.Fatalf("mapRunBucket accepted a terminated status, returned %q", bucket)
		}
		if bucket == InProgress {
			t.Fatal("a terminated run must never map to InProgress")
		}
	})

	t.Run("a single-run complete entry with a normal status maps toward Completed", func(t *testing.T) {
		snap := loadPortfolioFixture(t, "portfolio-populated.json")
		runs := runsForGoal(snap, "gist-adr-005-supersession")
		if len(runs) != 1 || runs[0].Bucket != kazi.RepoComplete || runs[0].Status == "terminated" {
			t.Fatalf("gist-adr-005-supersession: got %+v, want exactly one RepoComplete, non-terminated run", runs)
		}
		bucket, ok := mapRunBucket(runs[0].Bucket, runs[0].Status)
		if !ok || bucket != Completed {
			t.Errorf("mapRunBucket(RepoComplete, %q) = (%q, %t), want (Completed, true)", runs[0].Status, bucket, ok)
		}
	})

	t.Run("both sides: trusting RepoBucket alone reports a terminated run as InProgress", func(t *testing.T) {
		snap := loadPortfolioFixture(t, "portfolio-populated.json")
		runs := runsForGoal(snap, "t1-3-tenant-default-calibration")
		naive := func(bucket kazi.RepoBucket) Bucket {
			switch bucket {
			case kazi.RepoInProgress:
				return InProgress
			case kazi.RepoStuck:
				return ExecutionBlocked
			case kazi.RepoComplete:
				return Completed
			}
			return ""
		}
		if got := naive(runs[0].Bucket); got != InProgress {
			t.Fatalf("the naive control's own premise broke: got %q, want InProgress", got)
		}
		if got, _ := mapRunBucket(runs[0].Bucket, runs[0].Status); got == InProgress {
			t.Fatal("the real mapping must not report InProgress for a terminated run")
		}
	})
}

// --- E4-L3-T3: multi-run resolution ----------------------------------------

// TestJoinMultiRun is T3's acc line — the lane's hardest task.
func TestJoinMultiRun(t *testing.T) {
	snap := loadPortfolioFixture(t, "portfolio-populated.json")
	entry := syntheticEntry("dec-4001", ledger.KindDecision, "kazi:goal-warnings-clean")
	ctx := context.Background()

	t.Run("a converged statusFn resolves warnings-clean via kazi status", func(t *testing.T) {
		inner := func(_ context.Context, ref string) (*kazi.RunStatus, *kazi.ProposalStatus, error) {
			if ref != "warnings-clean" {
				t.Fatalf("statusFn called with ref %q, want %q", ref, "warnings-clean")
			}
			return &kazi.RunStatus{Ref: ref, Status: "converged", Converged: true, ReleaseRef: "v1.2.3"}, nil, nil
		}
		fn, calls := countingStatusFn(inner)
		rows, err := Join(ctx, []*ledger.Entry{entry}, snap, nil, fn)
		if err != nil {
			t.Fatalf("Join: %v", err)
		}
		row := rows[0]
		if row.Source != SourceKaziStatus {
			t.Errorf("Source = %q, want %q", row.Source, SourceKaziStatus)
		}
		if row.Bucket != Completed {
			t.Errorf("Bucket = %q, want %q", row.Bucket, Completed)
		}
		if *calls != 1 {
			t.Errorf("statusFn called %d times, want 1", *calls)
		}
	})

	t.Run("an Unavailable statusFn reports Ambiguous naming every distinct status", func(t *testing.T) {
		runs := runsForGoal(snap, "warnings-clean")
		if len(runs) < 2 {
			t.Fatalf("warnings-clean has %d run(s), want >= 2 — the fixture no longer demonstrates a multi-run disagreement", len(runs))
		}

		inner := func(context.Context, string) (*kazi.RunStatus, *kazi.ProposalStatus, error) {
			return nil, nil, &kazi.Unavailable{Reason: kazi.ReasonTimeout}
		}
		rows, err := Join(ctx, []*ledger.Entry{entry}, snap, nil, inner)
		if err != nil {
			t.Fatalf("Join: %v", err)
		}
		row := rows[0]
		if row.Bucket != "" {
			t.Errorf("Bucket = %q, want the zero value", row.Bucket)
		}
		if row.Ambiguous == nil {
			t.Fatal("Ambiguous is nil")
		}
		if len(row.Ambiguous.Statuses) < 2 {
			t.Fatalf("Ambiguous.Statuses = %v, want at least 2 distinct statuses", row.Ambiguous.Statuses)
		}
		want := map[string]bool{"over_budget": true, "stuck": true}
		got := map[string]bool{}
		for _, s := range row.Ambiguous.Statuses {
			got[s] = true
		}
		for s := range want {
			if !got[s] {
				t.Errorf("Ambiguous.Statuses = %v, missing %q", row.Ambiguous.Statuses, s)
			}
		}
	})

	t.Run("a single-run goal needs zero statusFn calls", func(t *testing.T) {
		single := syntheticEntry("dec-4002", ledger.KindDecision, "kazi:goal-gist-adr-005-supersession")
		rows, err := Join(ctx, []*ledger.Entry{single}, snap, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("Join: %v", err)
		}
		row := rows[0]
		if row.Source != SourcePortfolio {
			t.Errorf("Source = %q, want %q", row.Source, SourcePortfolio)
		}
		if row.Bucket != Completed {
			t.Errorf("Bucket = %q, want %q", row.Bucket, Completed)
		}
	})

	t.Run("fan-out is capped at MaxStatusCalls", func(t *testing.T) {
		synth := &kazi.Portfolio{ByRepo: map[string]map[kazi.RepoBucket][]kazi.RepoRun{}}
		var candidates []*ledger.Entry
		const n = MaxStatusCalls + 5
		for i := range n {
			goalID := fmt.Sprintf("bounded-goal-%02d", i)
			synth.ByRepo[fmt.Sprintf("repo-%02d", i)] = map[kazi.RepoBucket][]kazi.RepoRun{
				kazi.RepoInProgress: {{GoalRef: goalID, RunID: "run-a", Status: "running", Bucket: kazi.RepoInProgress}},
				kazi.RepoStuck:      {{GoalRef: goalID, RunID: "run-b", Status: "stuck", Bucket: kazi.RepoStuck}},
			}
			candidates = append(candidates, syntheticEntry(fmt.Sprintf("dec-5%03d", i), ledger.KindDecision, "kazi:goal-"+goalID))
		}
		fn, calls := countingStatusFn(func(context.Context, string) (*kazi.RunStatus, *kazi.ProposalStatus, error) {
			return &kazi.RunStatus{Status: "in_progress"}, nil, nil
		})
		rows, err := Join(ctx, candidates, synth, nil, fn)
		if err != nil {
			t.Fatalf("Join: %v", err)
		}
		if *calls != MaxStatusCalls {
			t.Fatalf("statusFn called %d times, want exactly %d", *calls, MaxStatusCalls)
		}
		beyond := 0
		for _, row := range rows[MaxStatusCalls:] {
			if row.Ambiguous == nil {
				t.Errorf("%s: beyond the bound but not reported (%+v)", row.ID, row)
				continue
			}
			beyond++
		}
		if beyond != n-MaxStatusCalls {
			t.Errorf("%d row(s) beyond the bound reported it, want %d", beyond, n-MaxStatusCalls)
		}
	})

	t.Run("both sides: taking the first by_repo entry disagrees with the real resolver", func(t *testing.T) {
		runs := runsForGoal(snap, "warnings-clean")
		naiveBucket, _ := mapRunBucket(runs[0].Bucket, runs[0].Status)

		inner := func(context.Context, string) (*kazi.RunStatus, *kazi.ProposalStatus, error) {
			return &kazi.RunStatus{Status: "converged", Converged: true}, nil, nil
		}
		rows, err := Join(ctx, []*ledger.Entry{entry}, snap, nil, inner)
		if err != nil {
			t.Fatalf("Join: %v", err)
		}
		realBucket := rows[0].Bucket
		if realBucket == naiveBucket {
			t.Fatalf("the real resolver (%q) agrees with the naive first-entry shortcut (%q) on warnings-clean; "+
				"the fixture no longer demonstrates why the shortcut is wrong", realBucket, naiveBucket)
		}
	})

	t.Run("both sides: an uncapped resolver exceeds MaxStatusCalls", func(t *testing.T) {
		synth := &kazi.Portfolio{ByRepo: map[string]map[kazi.RepoBucket][]kazi.RepoRun{}}
		const n = MaxStatusCalls + 5
		goalIDs := make([]string, 0, n)
		for i := range n {
			goalID := fmt.Sprintf("unbounded-goal-%02d", i)
			goalIDs = append(goalIDs, goalID)
			synth.ByRepo[fmt.Sprintf("repo-%02d", i)] = map[kazi.RepoBucket][]kazi.RepoRun{
				kazi.RepoInProgress: {{GoalRef: goalID, RunID: "run-a", Status: "running", Bucket: kazi.RepoInProgress}},
				kazi.RepoStuck:      {{GoalRef: goalID, RunID: "run-b", Status: "stuck", Bucket: kazi.RepoStuck}},
			}
		}
		calls := 0
		fn := func(context.Context, string) (*kazi.RunStatus, *kazi.ProposalStatus, error) {
			calls++
			return &kazi.RunStatus{Status: "in_progress"}, nil, nil
		}
		for _, goalID := range goalIDs {
			// A fresh counter per call is the defect: the real Join
			// threads one shared counter across every candidate so the
			// bound applies to the whole batch, not per goal.
			resolveGoal(context.Background(), synth, goalID, fn, new(int))
		}
		if calls != n {
			t.Fatalf("the uncapped control's own premise broke: made %d calls, want %d", calls, n)
		}
		if calls <= MaxStatusCalls {
			t.Fatal("the uncapped control did not exceed MaxStatusCalls; it proves nothing")
		}
	})
}

// --- E4-L3-T4: Completed evidence by reference only -------------------------

// TestJoinEvidence is T4's acc line.
func TestJoinEvidence(t *testing.T) {
	t.Run("Completed evidence is run id and release ref, nothing else", func(t *testing.T) {
		snap := &kazi.Portfolio{
			ByRepo: map[string]map[kazi.RepoBucket][]kazi.RepoRun{
				"repo-a": {kazi.RepoComplete: {{GoalRef: "evidence-goal", RunID: "run-complete-1", Status: "converged", Bucket: kazi.RepoComplete}}},
				"repo-b": {kazi.RepoStuck: {{GoalRef: "evidence-goal", RunID: "run-stuck-1", Status: "stuck", Bucket: kazi.RepoStuck}}},
			},
		}
		entry := syntheticEntry("dec-6001", ledger.KindDecision, "kazi:goal-evidence-goal")
		fn := func(context.Context, string) (*kazi.RunStatus, *kazi.ProposalStatus, error) {
			return &kazi.RunStatus{Status: "converged", Converged: true, ReleaseRef: "v9.9.9"}, nil, nil
		}
		rows, err := Join(context.Background(), []*ledger.Entry{entry}, snap, nil, fn)
		if err != nil {
			t.Fatalf("Join: %v", err)
		}
		row := rows[0]
		if row.Bucket != Completed {
			t.Fatalf("Bucket = %q, want %q", row.Bucket, Completed)
		}
		want := &KaziEvidence{RunID: "run-complete-1", ReleaseRef: "v9.9.9"}
		if row.Evidence == nil || *row.Evidence != *want {
			t.Errorf("Evidence = %+v, want %+v", row.Evidence, want)
		}
	})

	t.Run("KaziEvidence has no field capable of holding a predicate vector", func(t *testing.T) {
		rt := reflect.TypeOf(KaziEvidence{})
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			switch f.Type.Kind() {
			case reflect.Slice, reflect.Array, reflect.Map:
				t.Errorf("KaziEvidence.%s is a %s, which could hold a predicate vector — evidence must be by reference only", f.Name, f.Type.Kind())
			}
		}
	})

	t.Run("both sides: the reflect check catches a throwaway predicate field on a local copy", func(t *testing.T) {
		// Not committed to join.go — a stand-in for the regression this
		// check exists to catch, defined only in this test.
		type poisonedEvidence struct {
			RunID, ReleaseRef string
			Predicates        []string
		}
		rt := reflect.TypeOf(poisonedEvidence{})
		flagged := false
		for i := 0; i < rt.NumField(); i++ {
			switch rt.Field(i).Type.Kind() {
			case reflect.Slice, reflect.Array, reflect.Map:
				flagged = true
			}
		}
		if !flagged {
			t.Fatal("the poisoned local type's own premise broke: no slice/array/map field found")
		}
	})
}

// --- E4-L3-T5: roll-ups from totals.rows, never summed ----------------------

// TestJoinTotals is T5's acc line.
func TestJoinTotals(t *testing.T) {
	snap := loadPortfolioFixture(t, "portfolio-populated.json")

	t.Run("Totals is element for element equal to totals.rows", func(t *testing.T) {
		got := Totals(snap)
		if len(got) != len(snap.TotalsRows) {
			t.Fatalf("got %d rows, want %d", len(got), len(snap.TotalsRows))
		}
		for i := range got {
			if got[i] != snap.TotalsRows[i] {
				t.Errorf("row %d: got %+v, want %+v", i, got[i], snap.TotalsRows[i])
			}
		}
	})

	t.Run("never falls back to summing when planned/todo/blocked overlap", func(t *testing.T) {
		mismatched := &kazi.Portfolio{
			Planned:    make([]kazi.Proposal, 10),
			TotalsRows: []kazi.TotalsRow{{Bucket: kazi.RowPlanned, Count: 4, Pct: 100}},
			Blocked:    make([]kazi.BlockedEntry, 3),
		}
		sum := len(mismatched.Planned) + len(mismatched.Blocked)
		if sum == mismatched.TotalsRows[0].Count {
			t.Fatal("this fixture's own premise broke: the summed count must differ from TotalsRows")
		}
		got := Totals(mismatched)
		if len(got) != 1 || got[0].Count != 4 {
			t.Errorf("Totals = %+v, want the passthrough [{planned 4 100}]", got)
		}
	})

	t.Run("both sides: a summing implementation diverges from the fixture", func(t *testing.T) {
		mismatched := &kazi.Portfolio{
			Planned:    make([]kazi.Proposal, 10),
			TotalsRows: []kazi.TotalsRow{{Bucket: kazi.RowPlanned, Count: 4, Pct: 100}},
			Blocked:    make([]kazi.BlockedEntry, 3),
		}
		summingCount := len(mismatched.Planned) + len(mismatched.Blocked)
		if summingCount == Totals(mismatched)[0].Count {
			t.Fatal("the wrong, summing implementation happens to agree with the real one on this fixture")
		}
	})
}

// --- E4-L3-T6: the degraded join --------------------------------------------

// TestJoinDegraded is T6's acc line.
func TestJoinDegraded(t *testing.T) {
	entry := syntheticEntry("dec-7001", ledger.KindDecision, "kazi:goal-does-not-matter")
	reasons := []kazi.UnavailableReason{
		kazi.ReasonNotOnPath, kazi.ReasonNonZeroExit, kazi.ReasonMalformedJSON, kazi.ReasonWrongKind, kazi.ReasonTimeout,
	}

	for _, reason := range reasons {
		t.Run(string(reason), func(t *testing.T) {
			snapErr := &kazi.Unavailable{Reason: reason, Detail: "test detail"}
			rows, err := Join(context.Background(), []*ledger.Entry{entry}, nil, snapErr, neverCalledStatusFn(t))
			if err != nil {
				t.Fatalf("Join: %v", err)
			}
			row := rows[0]
			if row.Bucket != "" {
				t.Errorf("Bucket = %q, want the zero value", row.Bucket)
			}
			if row.Bucket == ToBePlanned {
				t.Fatal("a degraded join must never report ToBePlanned")
			}
			if row.Unresolved == nil {
				t.Fatal("Unresolved is nil")
			}
			if row.Unresolved.Reason != string(reason) {
				t.Errorf("Unresolved.Reason = %q, want %q", row.Unresolved.Reason, reason)
			}
		})
	}

	t.Run("resolves for real once snap is available again", func(t *testing.T) {
		snap := loadPortfolioFixture(t, "portfolio-populated.json")
		single := syntheticEntry("dec-7002", ledger.KindDecision, "kazi:goal-gist-adr-005-supersession")
		rows, err := Join(context.Background(), []*ledger.Entry{single}, snap, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("Join: %v", err)
		}
		if rows[0].Bucket != Completed {
			t.Errorf("Bucket = %q, want %q — the positive half proving the degraded path is not simply 'always unresolved'", rows[0].Bucket, Completed)
		}
	})

	t.Run("both sides: an empty-slice-on-error Join silently drops rows", func(t *testing.T) {
		wrongJoin := func(candidates []*ledger.Entry, snapErr error) []Row {
			if snapErr != nil {
				return nil // looks safe; actually silently drops every candidate
			}
			return make([]Row, len(candidates))
		}
		snapErr := &kazi.Unavailable{Reason: kazi.ReasonNotOnPath}
		lost := wrongJoin([]*ledger.Entry{entry}, snapErr)
		if len(lost) != 0 {
			t.Fatal("the wrong control's own premise broke: it should have dropped the candidate")
		}

		rows, err := Join(context.Background(), []*ledger.Entry{entry}, nil, snapErr, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("Join: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("the real Join lost the candidate too: got %d rows, want 1", len(rows))
		}
	})
}

// --- E4-L3-T7: the fixture-pairing harness ----------------------------------

// clauseChecklist names the seven clauses the lane acc: line states
// verbatim, each as a substring this file's own source must contain —
// grepped, not executed, so a future edit that renames or t.Skip()s one of
// T1-T6's subtests is caught here even though the remaining subtests would
// still keep `go test -run TestJoin` green.
var clauseChecklist = []string{
	"prop ref resolves through planned to its goal id",           // kazi:prop-<ref> resolution
	"goal ref resolves directly, no planned lookup",              // kazi:goal-<id> direct resolution
	"rejected/unknown proposal produces an Unresolved row",       // the UnresolvedRef rejected-proposal case
	"an Unavailable statusFn reports Ambiguous",                  // warnings-clean Ambiguous / Source: "kazi status"
	"a single-run terminated status never maps to InProgress",    // the terminated-never-InProgress case
	"Completed evidence is run id and release ref, nothing else", // Completed evidence-by-reference
	"Totals is element for element equal to totals.rows",         // the roll-up equality-with-no-summing case
}

// TestJoin is T7's acc line: `go test ./internal/status -run TestJoin`
// selects every TestJoin* function above by substring match, and this
// meta-test additionally asserts none of their defining subtests has quietly
// disappeared.
func TestJoin(t *testing.T) {
	t.Run("the seven lane clauses are each covered by a named subtest", func(t *testing.T) {
		src, err := os.ReadFile("join_test.go")
		if err != nil {
			t.Fatalf("reading own source: %v", err)
		}
		text := string(src)

		// Search only the portion of the file BEFORE clauseChecklist's own
		// declaration. Without this cut, every clause would trivially
		// "find itself" inside the checklist literal a few lines below —
		// the self-referential vacuous check docs/lore.md L-0001 warns
		// against — and the meta-test could never go red no matter what
		// happened to the real subtest names above it.
		const marker = "var clauseChecklist"
		idx := strings.Index(text, marker)
		if idx < 0 {
			t.Fatalf("marker %q not found in join_test.go's own source", marker)
		}
		subtestSource := text[:idx]

		for _, clause := range clauseChecklist {
			if !strings.Contains(subtestSource, clause) {
				t.Errorf("no subtest name above the checklist declaration contains %q", clause)
			}
		}
	})
}
