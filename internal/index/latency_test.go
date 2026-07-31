package index_test

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/fixture"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// A derived cache that does not beat reading the files has not earned the 6.5MB
// of binary and the eleven modules it costs (dec-0015). This measures whether it
// does, over the 200-entry fixture, in process, with spawn excluded — E1-L6 owns
// the end-to-end budget and measures the built binary.
//
// Three numbers, and the middle one is the point:
//
//	no cache at all   List + Get x 200, the whole ledger parsed
//	cache warm        List (hashing 200 files) + open + reconcile finding nothing
//	cache cold        the same, plus reading and indexing all 200
//
// The warm number is what a hook pays on every invocation after the first. The
// cold number is what it pays once after a clone or a `rm -rf .dira/cache`.

// warmBudget is the ceiling for answering with a warm cache. It is not the
// target — the target is E1-L6's — but a regression past it means the cache has
// stopped being an accelerator, and that is worth failing over rather than
// noticing a release later. Loose enough for CI hardware.
const warmBudget = 60 * time.Millisecond

const samples = 9

func TestTheCacheBeatsReadingTheFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test; skipped under -short")
	}
	if raceEnabled {
		t.Skip("timing test; the race detector costs several times the work being measured")
	}

	diraDir := ledgerDir(t)
	cacheDir := local.CacheDir(diraDir)
	ctx := context.Background()

	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatal(err)
	}

	direct := measure(t, samples, func() {
		list, err := store.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		read := 0
		for _, info := range list {
			e, err := store.Get(ctx, info.ID)
			if err != nil {
				t.Fatal(err)
			}
			read += len(e.Title) + len(e.Body)
		}
		if read == 0 {
			t.Fatal("read nothing; the measurement is measuring nothing")
		}
	})

	cold := measureWithSetup(t, samples, func() {
		// Deleting the cache is the setup, not the work. Timing the
		// unlink of three files would inflate the number this test
		// exists to report.
		if err := os.RemoveAll(cacheDir); err != nil {
			t.Fatal(err)
		}
	}, func() {
		ix, err := index.Open(ctx, store, cacheDir)
		if err != nil {
			t.Fatal(err)
		}
		if ix.Stats().Indexed != fixture.Size {
			t.Fatalf("a cold open indexed %d entries, want %d", ix.Stats().Indexed, fixture.Size)
		}
		brief(t, ctx, ix)
		if err := ix.Close(); err != nil {
			t.Fatal(err)
		}
	})

	warm := measure(t, samples, func() {
		ix, err := index.Open(ctx, store, cacheDir)
		if err != nil {
			t.Fatal(err)
		}
		if ix.Stats().Indexed != 0 {
			t.Fatalf("a warm open re-read %d entry files; it is not warm", ix.Stats().Indexed)
		}
		brief(t, ctx, ix)
		if err := ix.Close(); err != nil {
			t.Fatal(err)
		}
	})

	t.Logf("over %d entries, in process, spawn excluded (median, uncontended min):", fixture.Size)
	t.Logf("  no cache (List + Get x %d) : %v  (min %v)", fixture.Size, direct.median.Round(time.Microsecond), direct.min.Round(time.Microsecond))
	t.Logf("  cache cold (built this run): %v  (min %v)", cold.median.Round(time.Microsecond), cold.min.Round(time.Microsecond))
	t.Logf("  cache warm                 : %v  (min %v)", warm.median.Round(time.Microsecond), warm.min.Round(time.Microsecond))

	// The BUDGET is absolute, so the MEDIAN decides the verdict — a ceiling read
	// off the minimum passes on one lucky sample. But the MINIMUM decides
	// whether this machine can reach a verdict at all, and both are needed.
	//
	// A best sample comfortably inside the ceiling is proof the code CAN meet
	// it, so a median outside it is a statement about the scheduler. Reporting
	// that as a regression is the error this repo keeps making in new costumes:
	// a check announcing a verdict it lacked the evidence to reach. Same
	// treatment as internal/ledger/fixture's full-read budget, and the same
	// reason dec-0026 dropped the cold single-run ceiling.
	switch {
	case warm.min > warmBudget:
		// Even the fastest warm read is over. No amount of quiet fixes that.
		t.Errorf("a warm cache answers in %v even at its BEST, over the %v budget — "+
			"the fastest sample is over the ceiling, so this is dira's own work and not contention",
			warm.min, warmBudget)
	case warm.median > warmBudget:
		t.Skipf("NOT MEASURABLE on this machine, and NOT recorded as a pass.\n"+
			"  warm median %v is over the %v budget, but the best sample is %v — comfortably\n"+
			"  inside it, so the cache can meet the ceiling and this box is too busy to show it.\n"+
			"  CI's dedicated runner gates this on every push and is the authority.",
			warm.median, warmBudget, warm.min)
	}
	// The COMPARISON is relative, so it reads the uncontended minimum: under a
	// full parallel suite the median warm figure inflates past the direct one and
	// fails a claim that is true.
	if warm.min >= direct.min {
		t.Errorf("a warm cache (%v) is no faster than reading every file (%v).\n"+
			"The cache costs 6.5MB of binary and eleven modules against int-0002; if it does not beat the "+
			"files it should be removed by superseding dec-0002, not kept out of politeness.", warm.min, direct.min)
	}
}

// brief is the read path a hook actually runs: cst-0001's three sections,
// rendered from the entry files the index selected.
func brief(t *testing.T, ctx context.Context, ix *index.Index) {
	t.Helper()

	selectors := []index.Selector{
		{Kinds: []ledger.Kind{ledger.KindQuestion}, States: []ledger.State{ledger.StateOpen}, WithEdge: ledger.EdgeBlocks},
		{Kinds: []ledger.Kind{ledger.KindIntent}, States: []ledger.State{ledger.StateActive}},
		{Kinds: []ledger.Kind{ledger.KindDecision}, States: []ledger.State{ledger.StateAccepted}, Limit: 10},
	}

	rendered := 0
	for _, sel := range selectors {
		refs, err := ix.Select(ctx, sel)
		if err != nil {
			t.Fatal(err)
		}
		for _, ref := range refs {
			entry, err := ix.Entry(ctx, ref.ID)
			if err != nil {
				t.Fatal(err)
			}
			rendered += len(entry.Title) + len(entry.Body)
		}
	}
	if rendered == 0 {
		t.Fatal("the brief rendered nothing; the measurement is measuring nothing")
	}
}

// timing carries both statistics because the two questions this file asks need
// different ones, and conflating them silently weakens an assertion. See the
// comment in measureWithSetup.
type timing struct {
	min    time.Duration // the uncontended estimate — for COMPARATIVE claims
	median time.Duration // what a run actually costs — for ABSOLUTE budgets
}

func measure(t *testing.T, n int, run func()) timing {
	t.Helper()
	return measureWithSetup(t, n, func() {}, run)
}

func measureWithSetup(t *testing.T, n int, setup, run func()) timing {
	t.Helper()

	// One untimed run so the first one's page faults and lazily-initialised
	// driver state do not land in the sample.
	setup()
	run()

	took := make([]time.Duration, 0, n)
	for range n {
		setup()
		start := time.Now()
		run()
		took = append(took, time.Since(start))
	}
	slices.Sort(took)

	// Two statistics, because this file asks two different questions and one
	// number cannot answer both honestly.
	//
	// MIN is the uncontended estimate. `go test ./...` runs packages in
	// parallel, so these samples compete for CPU and disk, and contention only
	// ever ADDS time — it cannot make an operation finish sooner. The smallest
	// sample is therefore the closest estimate of what the code costs, and it
	// is the right input to a COMPARATIVE claim like "the cache beats reading
	// the files", where contention would otherwise inflate both arms unequally
	// and flip a true result.
	//
	// MEDIAN is what a run actually costs on this machine, and it is the only
	// honest input to an ABSOLUTE budget. Since min <= median by construction,
	// asserting a ceiling against the minimum fires strictly LESS often than
	// against the median — a run whose median is 80ms passes a 60ms budget on
	// the strength of one lucky sample. That is a weakened assertion wearing a
	// robustness argument, and this comment exists because it was briefly
	// exactly that.
	return timing{min: took[0], median: took[len(took)/2]}
}

// BenchmarkWarmBrief is the number to quote for a hook invocation after the
// first: reconciling a 200-entry ledger against an untouched cache and
// rendering the brief's three sections from the files.
func BenchmarkWarmBrief(b *testing.B) {
	diraDir := benchLedger(b)
	store, err := local.Open(diraDir)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	cacheDir := local.CacheDir(diraDir)

	ix, err := index.Open(ctx, store, cacheDir)
	if err != nil {
		b.Fatal(err)
	}
	_ = ix.Close()

	b.ResetTimer()
	for b.Loop() {
		ix, err := index.Open(ctx, store, cacheDir)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := ix.Select(ctx, index.Selector{Kinds: []ledger.Kind{ledger.KindDecision}, Limit: 10}); err != nil {
			b.Fatal(err)
		}
		_ = ix.Close()
	}
}

// BenchmarkReindex is what dira reindex costs over 200 entries: every file read,
// parsed and written into a database created from nothing.
func BenchmarkReindex(b *testing.B) {
	diraDir := benchLedger(b)
	store, err := local.Open(diraDir)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	cacheDir := local.CacheDir(diraDir)

	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		if err := os.RemoveAll(cacheDir); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()

		ix, err := index.OpenFresh(ctx, store, cacheDir)
		if err != nil {
			b.Fatal(err)
		}
		if ix.Stats().Indexed != fixture.Size {
			b.Fatalf("a reindex read %d entry files, want %d", ix.Stats().Indexed, fixture.Size)
		}
		_ = ix.Close()
	}
}

func benchLedger(b *testing.B) string {
	b.Helper()

	entries, err := fixture.Generate(fixture.Seed, fixture.Size)
	if err != nil {
		b.Fatal(err)
	}
	diraDir := b.TempDir() + "/.dira"
	if err := os.MkdirAll(diraDir+"/entries", 0o755); err != nil {
		b.Fatal(err)
	}
	store, err := local.Open(diraDir)
	if err != nil {
		b.Fatal(err)
	}
	if err := fixture.Write(context.Background(), store, entries); err != nil {
		b.Fatal(err)
	}
	return diraDir
}
