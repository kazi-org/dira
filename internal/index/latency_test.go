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

	t.Logf("over %d entries, in process, spawn excluded:", fixture.Size)
	t.Logf("  no cache (List + Get x %d) : %v", fixture.Size, direct.Round(time.Microsecond))
	t.Logf("  cache cold (built this run): %v", cold.Round(time.Microsecond))
	t.Logf("  cache warm                 : %v", warm.Round(time.Microsecond))

	if warm > warmBudget {
		t.Errorf("a warm cache answers in %v, over the %v budget", warm, warmBudget)
	}
	if warm >= direct {
		t.Errorf("a warm cache (%v) is no faster than reading every file (%v).\n"+
			"The cache costs 6.5MB of binary and eleven modules against int-0002; if it does not beat the "+
			"files it should be removed by superseding dec-0002, not kept out of politeness.", warm, direct)
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

func measure(t *testing.T, n int, run func()) time.Duration {
	t.Helper()
	return measureWithSetup(t, n, func() {}, run)
}

func measureWithSetup(t *testing.T, n int, setup, run func()) time.Duration {
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
	return took[len(took)/2]
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
