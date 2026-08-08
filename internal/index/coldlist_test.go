package index_test

import (
	"context"
	"os"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/fixture"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// counter is a ledger.Store that records how it was asked for the ledger.
//
// The claim under test is about which calls happen, not how long they take, and
// a counter is the only instrument that can say so: a timing test cannot tell
// "the second read was skipped" from "the machine was quick", and every way a
// timing test breaks returns a fast number that looks like a pass.
type counter struct {
	inner *local.Store

	lists   int
	listIDs int
	gets    int

	// dropped is an id ListIDs pretends is not in the ledger. It is the
	// constructed defect for "the cold build actually uses this listing":
	// if the answer were ignored, removing an entry from it would change
	// nothing.
	dropped string
}

func (c *counter) Get(ctx context.Context, id string) (*ledger.Entry, error) {
	c.gets++
	return c.inner.Get(ctx, id)
}

func (c *counter) List(ctx context.Context) ([]ledger.EntryInfo, error) {
	c.lists++
	return c.inner.List(ctx)
}

func (c *counter) Create(ctx context.Context, e *ledger.Entry) error { return c.inner.Create(ctx, e) }
func (c *counter) Put(ctx context.Context, e *ledger.Entry) error    { return c.inner.Put(ctx, e) }
func (c *counter) Delete(ctx context.Context, id string) error       { return c.inner.Delete(ctx, id) }

func (c *counter) reset() { c.lists, c.listIDs, c.gets = 0, 0, 0 }

// plainStore is a backend with no cheap listing — the shape every other
// ledger.Store implementation has until it adds one. It must still work.
type plainStore struct{ *counter }

// fastStore is the same backend with ledger/local's ListIDs exposed.
type fastStore struct{ *counter }

func (f fastStore) ListIDs(ctx context.Context) ([]ledger.EntryInfo, error) {
	f.listIDs++
	infos, err := f.inner.ListIDs(ctx)
	if err != nil || f.dropped == "" {
		return infos, err
	}
	out := infos[:0]
	for _, info := range infos {
		if info.ID != f.dropped {
			out = append(out, info)
		}
	}
	return out, nil
}

// TestAColdBuildDoesNotHashTheLedgerTwice is E1-L6-T5's optimisation, asserted
// as behaviour rather than as a duration.
//
// Before it, a cold `dira brief` opened, read and SHA-1'd all 200 entry files to
// build a version listing, then opened, read and parsed all 200 again — because
// on an empty cache there is nothing for those versions to be compared against.
// After it, the listing costs one readdir and the files are read once.
//
// Every subtest carries both sides (docs/lore.md L-0001). The one that matters
// most is the second: it is the green side, and it is what proves the skipped
// hash pass changed no stored value. If the cold build wrote versions the warm
// reconcile does not accept, the whole ledger is re-read on the very next
// invocation and the optimisation is a regression wearing its own name.
func TestAColdBuildDoesNotHashTheLedgerTwice(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	diraDir := ledgerDir(t)
	cacheDir := local.CacheDir(diraDir)

	base, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("local.Open: %v", err)
	}
	c := &counter{inner: base}

	open := func(t *testing.T, store ledger.Store) index.Stats {
		t.Helper()
		ix, err := index.Open(ctx, store, cacheDir)
		if err != nil {
			t.Fatalf("index.Open: %v", err)
		}
		defer func() { _ = ix.Close() }()
		if notice := ix.Notice(); notice != "" {
			t.Fatalf("the cache degraded, so this test is measuring the in-memory path: %s", notice)
		}
		return ix.Stats()
	}

	t.Run("a cold build lists the ledger without hashing it", func(t *testing.T) {
		stats := open(t, fastStore{c})

		if c.listIDs != 1 {
			t.Errorf("ListIDs called %d times, want 1 — the cold build did not take the cheap listing", c.listIDs)
		}
		if c.lists != 0 {
			t.Errorf("List called %d times on a cold build; that is the version pass whose answer nothing reads, "+
				"and it is a second open, read and SHA-1 of every file in the ledger", c.lists)
		}
		if c.gets != fixture.Size {
			t.Errorf("Get called %d times, want %d — every entry is read exactly once on a cold build", c.gets, fixture.Size)
		}
		if stats.Indexed != fixture.Size || stats.Entries != fixture.Size {
			t.Errorf("indexed %d and holds %d entries, want %d of each", stats.Indexed, stats.Entries, fixture.Size)
		}
	})

	t.Run("the rows it wrote satisfy the warm reconcile", func(t *testing.T) {
		// The green side, and the load-bearing one. A cold build that
		// stored the wrong version — or none — passes the subtest above
		// and fails here, by re-reading a ledger nothing has touched.
		c.reset()
		stats := open(t, fastStore{c})

		if c.lists != 1 {
			t.Errorf("List called %d times on a warm cache, want exactly 1: the version pass IS the staleness "+
				"check (dec-0015) and must not be skipped where there are rows to compare against", c.lists)
		}
		if c.listIDs != 0 {
			t.Errorf("ListIDs called %d times on a WARM cache. That listing reports no versions, so a reconcile "+
				"reading it would be comparing every row against the empty string", c.listIDs)
		}
		if c.gets != 0 {
			t.Errorf("re-read %d entry files over a ledger nothing had changed, want 0 — the versions the cold "+
				"build stored are not the hashes of the files it read", c.gets)
		}
		if stats.Indexed != 0 {
			t.Errorf("re-indexed %d entries over an unchanged ledger, want 0", stats.Indexed)
		}
	})

	t.Run("an edited entry is still the only thing re-read", func(t *testing.T) {
		// Staleness detection after a cold build that skipped the hash
		// pass: the red side of the subtest above. One file changes, and
		// exactly one entry is re-read.
		const edited = "dec-0007"
		path := entryPath(diraDir, edited)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		t.Cleanup(func() { _ = os.WriteFile(path, before, 0o644) })
		if err := os.WriteFile(path, append(before, "\nEdited behind the cache's back.\n"...), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}

		c.reset()
		stats := open(t, fastStore{c})

		if stats.Indexed != 1 {
			t.Errorf("re-indexed %d entries after one file changed, want 1", stats.Indexed)
		}
		if c.gets != 1 {
			t.Errorf("read %d entry files after one changed, want 1", c.gets)
		}
	})

	t.Run("the cheap listing is what the cold build indexes", func(t *testing.T) {
		// Red, constructed: an entry is withheld from ListIDs and from
		// nothing else. If the cold build were still reading the ledger
		// through List, the withheld entry would be indexed anyway and
		// the count below would be unchanged — which is to say the
		// counters above would be measuring a call nobody acts on.
		fresh := t.TempDir()
		c.reset()
		c.dropped = "dec-0007"
		t.Cleanup(func() { c.dropped = "" })

		ix, err := index.Open(ctx, fastStore{c}, fresh)
		if err != nil {
			t.Fatalf("index.Open: %v", err)
		}
		defer func() { _ = ix.Close() }()

		if got := ix.Stats().Entries; got != fixture.Size-1 {
			t.Errorf("indexed %d entries with one withheld from ListIDs, want %d — the cold build is not "+
				"reading the ledger through the listing this test counts", got, fixture.Size-1)
		}
	})

	t.Run("a backend with no cheap listing builds the same index", func(t *testing.T) {
		// Green for the fallback. The optional interface must be an
		// optimisation for the backends that have it, never a
		// requirement: dec-0005 commits to a second implementation, and
		// the GitHub Contents API returns a sha with its listing, so it
		// has nothing to gain and should not be made to implement this.
		fresh := t.TempDir()
		c.reset()

		cold, err := index.Open(ctx, plainStore{c}, fresh)
		if err != nil {
			t.Fatalf("index.Open: %v", err)
		}
		coldStats := cold.Stats()
		_ = cold.Close()

		if c.listIDs != 0 {
			t.Errorf("ListIDs called %d times on a store that does not offer it", c.listIDs)
		}
		if c.lists != 1 {
			t.Errorf("List called %d times, want 1 — a backend with no cheap listing still gets listed", c.lists)
		}
		if coldStats.Indexed != fixture.Size || coldStats.Entries != fixture.Size {
			t.Errorf("indexed %d and holds %d entries through the fallback, want %d of each",
				coldStats.Indexed, coldStats.Entries, fixture.Size)
		}

		// And the rows it wrote are interchangeable with the fast path's:
		// reopening THIS cache through the fast-path store re-reads
		// nothing, which it could not do if the two paths stored
		// different versions.
		c.reset()
		warm, err := index.Open(ctx, fastStore{c}, fresh)
		if err != nil {
			t.Fatalf("index.Open: %v", err)
		}
		defer func() { _ = warm.Close() }()
		if got := warm.Stats().Indexed; got != 0 {
			t.Errorf("re-indexed %d entries reopening a cache the fallback built, want 0 — the two listings "+
				"store different versions for the same file", got)
		}
	})
}
