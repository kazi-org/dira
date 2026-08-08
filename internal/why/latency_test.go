package why_test

import (
	"context"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
	"github.com/kazi-org/dira/internal/why"
)

// budget is what E1 gives `dira why` for its own work, spawn excluded.
//
// int-0002 budgets the whole invocation at well under 100ms and E1-L3 measured
// process spawn alone at 60–90ms on this hardware, so the code's share is what
// this can honestly hold. 40ms is E1's restated figure for a command's own work
// and the number this test fails above; the median is logged either way, because
// a budget nobody reads the real number of is a budget that drifts to its
// ceiling.
const budget = 40 * time.Millisecond

// TestTheWholeAnswerFitsTheBudget measures what `dira why` costs between process
// start and process exit: opening the index (which reconciles), resolving the
// term, building the chain and rendering it.
//
// It is measured, not assumed, and it is measured over the 200-entry fixture
// rather than over this repository's 30 entries, because the ledger that has to
// fit the budget is the one E1's acceptance line names.
func TestTheWholeAnswerFitsTheBudget(t *testing.T) {
	if raceEnabled {
		// The race detector costs several times the work being measured,
		// so the number it produces is about the instrumentation. Saying
		// so beats either a lying budget or a test quietly tagged out.
		t.Skip("timing under -race measures the detector, not the read path")
	}

	diraDir := fixtureLedger(t, 200)
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("local.Open: %v", err)
	}
	cacheDir := local.CacheDir(diraDir)
	ctx := context.Background()

	// Warm the cache first: a cold cache is E1-L3's measurement and E1-L6's
	// budget. What a hook pays on every invocation after the first is this.
	subject := warmAndPick(t, ctx, store, cacheDir)

	const runs = 21
	samples := make([]time.Duration, 0, runs)
	for i := 0; i < runs; i++ {
		start := time.Now()

		ix, err := index.Open(ctx, store, cacheDir)
		if err != nil {
			t.Fatalf("index.Open: %v", err)
		}
		nodes, err := why.Resolve(ctx, ix, subject)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if len(nodes) != 1 {
			t.Fatalf("Resolve(%s) returned %d entries; the measurement is not of a chain", subject, len(nodes))
		}
		chain, err := why.Build(ctx, ix, subject, nodes[0].Ref)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if err := why.RenderText(io.Discard, chain, why.DefaultWidth); err != nil {
			t.Fatalf("RenderText: %v", err)
		}

		samples = append(samples, time.Since(start))
		if err := ix.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}

	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	median, worst := samples[len(samples)/2], samples[len(samples)-1]
	t.Logf("dira why over a 200-entry ledger, warm cache, spawn excluded: median %v, slowest of %d %v", median, runs, worst)

	// The same runs with the index already open, which is what this lane's
	// own code costs. The difference between the two numbers is E1-L3's
	// reconcile, and attributing it here stops a regression in either being
	// read as a regression in the other.
	ix, err := index.Open(ctx, store, cacheDir)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	defer func() { _ = ix.Close() }()

	chainOnly := make([]time.Duration, 0, runs)
	for i := 0; i < runs; i++ {
		start := time.Now()
		chain, err := why.Build(ctx, ix, subject, subject)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if err := why.RenderText(io.Discard, chain, why.DefaultWidth); err != nil {
			t.Fatalf("RenderText: %v", err)
		}
		chainOnly = append(chainOnly, time.Since(start))
	}
	sort.Slice(chainOnly, func(i, j int) bool { return chainOnly[i] < chainOnly[j] })
	t.Logf("building and rendering the chain alone, index already open: median %v", chainOnly[len(chainOnly)/2])

	if median > budget {
		t.Errorf("the median run is %v, over E1's %v budget for dira's own work", median, budget)
	}
}

// warmAndPick builds the cache and returns the id of a decision with
// alternatives, which is the shape a chain costs the most to render.
func warmAndPick(t *testing.T, ctx context.Context, store ledger.Store, cacheDir string) string {
	t.Helper()

	ix, err := index.Open(ctx, store, cacheDir)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	defer func() { _ = ix.Close() }()

	if !ix.Stats().Cached {
		t.Fatalf("the cache was not written (%s); this would be measuring the degraded path", ix.Notice())
	}
	refs, err := ix.Select(ctx, index.Selector{
		Kinds:  []ledger.Kind{ledger.KindDecision},
		States: []ledger.State{ledger.StateAccepted},
		Limit:  1,
	})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(refs) == 0 {
		t.Fatal("the fixture holds no accepted decision to build a chain from")
	}
	return refs[0].ID
}
