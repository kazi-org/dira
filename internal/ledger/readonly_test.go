package ledger_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/ledgertest"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// recordingStore is a ledger.Store that remembers every call made against it and
// then makes the call for real.
//
// Both halves matter. The refusal tests assert this store received *zero* calls,
// and a store that recorded nothing because it can do nothing would satisfy that
// assertion no matter what the wrapper did — so it delegates to a working
// in-memory store, and the same writes are then made against it unwrapped to
// show they land. Zero calls is only evidence when a call would have worked.
type recordingStore struct {
	inner *memStore
	calls []string
}

func newRecordingStore() *recordingStore { return &recordingStore{inner: newMemStore()} }

func (r *recordingStore) Get(ctx context.Context, id string) (*ledger.Entry, error) {
	r.calls = append(r.calls, "get "+id)
	return r.inner.Get(ctx, id)
}

func (r *recordingStore) List(ctx context.Context) ([]ledger.EntryInfo, error) {
	r.calls = append(r.calls, "list")
	return r.inner.List(ctx)
}

func (r *recordingStore) Create(ctx context.Context, e *ledger.Entry) error {
	r.calls = append(r.calls, "create "+idOf(e))
	return r.inner.Create(ctx, e)
}

func (r *recordingStore) Put(ctx context.Context, e *ledger.Entry) error {
	r.calls = append(r.calls, "put "+idOf(e))
	return r.inner.Put(ctx, e)
}

func (r *recordingStore) Delete(ctx context.Context, id string) error {
	r.calls = append(r.calls, "delete "+id)
	return r.inner.Delete(ctx, id)
}

func idOf(e *ledger.Entry) string {
	if e == nil {
		return "<nil>"
	}
	return e.ID
}

// TestReadOnlyRefusesEveryWriteBeforeTheStoreIsTouched is cst-0003 rule 1 as a
// property of the type: the refusal happens in the wrapper, so the inner store
// never hears about the write at all.
func TestReadOnlyRefusesEveryWriteBeforeTheStoreIsTouched(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	inner := newRecordingStore()
	parent := ledger.ReadOnly(inner)

	writes := []struct {
		name string
		run  func(ledger.Store) error
	}{
		{"Create", func(s ledger.Store) error { return s.Create(ctx, ledgertest.Entry("dec-0001")) }},
		{"Put", func(s ledger.Store) error { return s.Put(ctx, ledgertest.Entry("dec-0001")) }},
		{"Delete", func(s ledger.Store) error { return s.Delete(ctx, "dec-0001") }},
	}

	for _, w := range writes {
		err := w.run(parent)
		if err == nil {
			t.Fatalf("%s through a read-only store returned no error", w.name)
		}
		if !errors.Is(err, ledger.ErrReadOnly) {
			t.Errorf("%s: err = %v, want one wrapping ledger.ErrReadOnly so a caller can tell a refusal from a storage failure", w.name, err)
		}
		// The message is asserted, not just the sentinel: a hook that
		// fails has to tell its reader which rule stopped it.
		if !strings.Contains(err.Error(), "cst-0003 rule 1") {
			t.Errorf("%s: the refusal does not name the constraint it enforces: %v", w.name, err)
		}
	}

	if len(inner.calls) != 0 {
		t.Errorf("the inner store received %v; a read-only store refuses before the parent is touched, "+
			"so a backend that logged, locked or opened a connection on the way to failing would already have written", inner.calls)
	}

	// The other side of "zero calls": the same three writes against the same
	// store, unwrapped, do land. Without this the assertion above passes just
	// as happily against a store that cannot do anything.
	if err := inner.Create(ctx, ledgertest.Entry("dec-0001")); err != nil {
		t.Fatalf("Create against the unwrapped store: %v", err)
	}
	if err := inner.Put(ctx, ledgertest.Entry("dec-0001")); err != nil {
		t.Fatalf("Put against the unwrapped store: %v", err)
	}
	if err := inner.Delete(ctx, "dec-0001"); err != nil {
		t.Fatalf("Delete against the unwrapped store: %v", err)
	}
	if want := []string{"create dec-0001", "put dec-0001", "delete dec-0001"}; !slices.Equal(inner.calls, want) {
		t.Errorf("the unwrapped store recorded %v, want %v — if these writes cannot reach it either, "+
			"the zero-call assertion above is measuring a broken store rather than a working guard", inner.calls, want)
	}
}

// TestReadOnlyRefusesAWriteItCannotEvenDescribe. The refusal is a policy
// decision rather than a report from the backend, so it does not depend on the
// entry being well formed, on there being an inner store, or on the context
// still being live. Each of these would reach the parent if the wrapper deferred
// to anything.
func TestReadOnlyRefusesAWriteItCannotEvenDescribe(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	cases := []struct {
		name string
		run  func() error
	}{
		{"a nil entry", func() error { return ledger.ReadOnly(newRecordingStore()).Create(context.Background(), nil) }},
		{"an invalid entry", func() error {
			bad := ledgertest.Entry("dec-0001")
			bad.Alternatives = nil
			return ledger.ReadOnly(newRecordingStore()).Put(context.Background(), bad)
		}},
		{"a cancelled context", func() error {
			return ledger.ReadOnly(newRecordingStore()).Delete(cancelled, "dec-0001")
		}},
		{"no store at all", func() error { return ledger.ReadOnly(nil).Create(context.Background(), ledgertest.Entry("dec-0001")) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.run()
			if !errors.Is(err, ledger.ErrReadOnly) {
				t.Errorf("err = %v, want one wrapping ledger.ErrReadOnly", err)
			}
			if err != nil && !strings.Contains(err.Error(), "cst-0003 rule 1") {
				t.Errorf("the refusal does not name the constraint: %v", err)
			}
		})
	}
}

// TestReadOnlyPassesReadsThrough is the green side. A guard that refused
// everything, reads included, would satisfy every assertion in the refusal test
// above and be useless — so this asserts the wrapper answers exactly what the
// wrapped store answers, error semantics included.
func TestReadOnlyPassesReadsThrough(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	inner := newRecordingStore()
	for _, id := range []string{"cst-0002", "dec-0060", "note-0003"} {
		if err := inner.inner.Create(ctx, ledgertest.Entry(id)); err != nil {
			t.Fatalf("seeding %s: %v", id, err)
		}
	}
	parent := ledger.ReadOnly(inner)

	wrapped, err := parent.List(ctx)
	if err != nil {
		t.Fatalf("List through the wrapper: %v", err)
	}
	direct, err := inner.inner.List(ctx)
	if err != nil {
		t.Fatalf("List against the store: %v", err)
	}
	if !slices.Equal(wrapped, direct) {
		t.Fatalf("List through the wrapper = %v, want %v", wrapped, direct)
	}
	if len(wrapped) != 3 {
		t.Fatalf("List returned %d entries, want the 3 that were seeded — a wrapper that returned nothing "+
			"would pass an equality check against a store nobody wrote to", len(wrapped))
	}

	for _, info := range wrapped {
		got, err := parent.Get(ctx, info.ID)
		if err != nil {
			t.Fatalf("Get(%s) through the wrapper: %v", info.ID, err)
		}
		want, err := inner.inner.Get(ctx, info.ID)
		if err != nil {
			t.Fatalf("Get(%s) against the store: %v", info.ID, err)
		}
		if got.ID != want.ID || got.Title != want.Title || got.State != want.State || got.Body != want.Body {
			t.Errorf("Get(%s) through the wrapper returned %+v, want %+v", info.ID, got, want)
		}
		if got.Version() != want.Version() || got.Version() == "" {
			t.Errorf("Get(%s): version %q through the wrapper, %q against the store", info.ID, got.Version(), want.Version())
		}
	}

	// ErrNotFound has to survive the wrapper too: E3-L3-T7 tells an
	// unresolved parent apart from an unreadable one, and it does that with
	// errors.Is.
	if _, err := parent.Get(ctx, "dec-9999"); !errors.Is(err, ledger.ErrNotFound) {
		t.Errorf("Get on a missing entry: err = %v, want ErrNotFound", err)
	}

	// A read-only store over nothing is a caller's bug, and it says so
	// rather than reporting an empty ledger — a parent that silently read as
	// empty is the failure mode E3-L3 exists to prevent.
	if _, err := ledger.ReadOnly(nil).List(ctx); err == nil {
		t.Error("List over a nil store returned no error; an empty answer and an absent ledger must not look the same")
	}
}

// TestReadOnlyWritesNoByteUnderTheParent is the acceptance clause, and it is
// asserted the way `dira supersede`'s cross-boundary refusal is: over the
// parent's bytes rather than over an exit code or a call count.
//
// The digest is a filesystem walk, deliberately. .dira/cache/ is gitignored
// (internal/index's TestTheCacheIsGitignored asserts exactly that path), so a
// git-based check would report a spotless parent while the cache was being
// written into it — vacuously green against the one write that actually happens
// here.
//
// The second half of the test is the red side: the same fixture opened the
// ordinary way, through index.Open, which writes .dira/cache/index.db under the
// parent. If that leg did not move the digest, the byte-identity assertion above
// could not have caught an upward write and the test would be proving nothing.
func TestReadOnlyWritesNoByteUnderTheParent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Green: every read a caller can make through the wrapper, against a
	// tree that must come out byte-identical.
	guarded := parentLedger(t)
	before := treeDigest(t, guarded)

	backend, err := local.Open(filepath.Join(guarded, ".dira"))
	if err != nil {
		t.Fatalf("opening the parent: %v", err)
	}
	got := readWholeLedger(t, ledger.ReadOnly(backend))

	// The read has to have worked, or "nothing was written" is what a
	// wrapper that refuses reads would also report.
	if want := []string{"cst-0002", "dec-0060", "note-0003"}; !slices.Equal(got, want) {
		t.Fatalf("reading the parent through the wrapper returned %v, want %v", got, want)
	}

	if after := treeDigest(t, guarded); after != before {
		t.Errorf("reading the parent through a read-only store changed it (cst-0003 rule 1)\n  before %s\n  after  %s", before, after)
	}
	cacheDir := local.CacheDir(filepath.Join(guarded, ".dira"))
	if _, err := os.Stat(cacheDir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s exists after a read-only read (stat err = %v); the cache is a write, and it is a write into somebody else's ledger", cacheDir, err)
	}

	// Red: the ordinary read path, which is the write this task exists to
	// prevent. Same fixture, same reads, a different store.
	ordinary := parentLedger(t)
	diraDir := filepath.Join(ordinary, ".dira")
	beforeOrdinary := treeDigest(t, ordinary)

	plain, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("opening the parent: %v", err)
	}
	ix, err := index.Open(ctx, plain, local.CacheDir(diraDir))
	if err != nil {
		t.Fatalf("index.Open over the parent: %v", err)
	}
	if !ix.Stats().Cached {
		t.Fatalf("index.Open degraded to an in-memory cache (%s), so this leg is not exercising the on-disk write it is here to demonstrate", ix.Notice())
	}
	if err := ix.Close(); err != nil {
		t.Fatalf("closing the index: %v", err)
	}

	if treeDigest(t, ordinary) == beforeOrdinary {
		t.Fatal("opening the parent through index.Open left the tree byte-identical, so the digest cannot see " +
			"the cache write and the assertion above proves nothing")
	}
	if _, err := os.Stat(local.CacheDir(diraDir)); err != nil {
		t.Fatalf("index.Open did not create %s (%v); this leg is not the write it claims to be", local.CacheDir(diraDir), err)
	}
}

// TestReadOnlyTheDigestSeesAnEntryChange is the digest's own two-sided check.
// The test above rests entirely on the walk noticing a byte that moved, and a
// digest that never changes is indistinguishable from a ledger nobody wrote to.
func TestReadOnlyTheDigestSeesAnEntryChange(t *testing.T) {
	t.Parallel()

	root := parentLedger(t)
	backend, err := local.Open(filepath.Join(root, ".dira"))
	if err != nil {
		t.Fatalf("opening the ledger: %v", err)
	}
	before := treeDigest(t, root)

	changed := ledgertest.Entry("dec-0060")
	changed.Title = "A title nobody in the fixture has"
	if err := backend.Put(context.Background(), changed); err != nil {
		t.Fatalf("Put: %v", err)
	}
	edited := treeDigest(t, root)
	if edited == before {
		t.Fatal("the digest did not move after an entry was rewritten")
	}

	// A file added where no file was, which is the shape of a cache write.
	if err := os.WriteFile(filepath.Join(root, ".dira", "sentinel"), []byte("x"), 0o644); err != nil {
		t.Fatalf("writing the sentinel: %v", err)
	}
	withFile := treeDigest(t, root)
	if withFile == edited {
		t.Fatal("the digest did not move after a file was added")
	}

	// And an empty directory, because a cache directory dira created and
	// then failed to populate is still a write into somebody else's ledger.
	if err := os.Mkdir(filepath.Join(root, ".dira", "cache"), 0o755); err != nil {
		t.Fatalf("creating the directory: %v", err)
	}
	if treeDigest(t, root) == withFile {
		t.Error("the digest did not move after an empty directory was created; a walk that only hashes files " +
			"would call a half-made cache no change at all")
	}
}

// parentLedger writes a small ledger into a fresh temp directory and returns the
// directory holding its `.dira`.
//
// A real temp tree, not internal/enforcer/testdata/ledgers/: L-0014 records that
// pointing dira at a fixture directory silently grades against this repository's
// own .dira, because local.Find walks up. A digest of a directory that is not a
// ledger would be identical before and after every run for the wrong reason.
func parentLedger(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	diraDir := filepath.Join(root, ".dira")
	if err := os.Mkdir(diraDir, 0o755); err != nil {
		t.Fatalf("creating %s: %v", diraDir, err)
	}
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("opening the fixture ledger: %v", err)
	}

	ctx := context.Background()
	for _, id := range []string{"cst-0002", "dec-0060", "note-0003"} {
		e := ledgertest.Entry(id)
		e.Title = "The parent ledger's " + id
		if err := store.Create(ctx, e); err != nil {
			t.Fatalf("seeding %s: %v", id, err)
		}
	}
	config := "[ledger]\nname = \"parent\"\ntier = \"person\"\n"
	if err := os.WriteFile(filepath.Join(diraDir, "config.toml"), []byte(config), 0o644); err != nil {
		t.Fatalf("writing the fixture config: %v", err)
	}
	return root
}

// readWholeLedger performs every read the interface offers and returns the ids
// it saw, in order. This is what "a full read of that parent" means: the list,
// every entry body, and a miss.
func readWholeLedger(t *testing.T, store ledger.Store) []string {
	t.Helper()

	ctx := context.Background()
	infos, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var ids []string
	for _, info := range infos {
		entry, err := store.Get(ctx, info.ID)
		if err != nil {
			t.Fatalf("Get(%s): %v", info.ID, err)
		}
		if entry.Title == "" {
			t.Errorf("%s came back with no title; the read did not reach the file", info.ID)
		}
		ids = append(ids, entry.ID)
	}
	if _, err := store.Get(ctx, "dec-9999"); !errors.Is(err, ledger.ErrNotFound) {
		t.Errorf("Get on a missing entry: err = %v, want ErrNotFound", err)
	}
	return ids
}

// treeDigest is one sha256 over every path under root — directories included,
// file contents included.
//
// Directories are in it because an empty .dira/cache/ is still a write into a
// ledger dira does not own, and a walk that only hashed files would score that
// as no change. Paths are hashed alongside contents so that a file moved, or a
// second copy of the same bytes appearing, both register.
func treeDigest(t *testing.T, root string) string {
	t.Helper()

	type node struct {
		path string
		dir  bool
	}
	var nodes []node
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		nodes = append(nodes, node{path: path, dir: d.IsDir()})
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	slices.SortFunc(nodes, func(a, b node) int { return strings.Compare(a.path, b.path) })

	sum := sha256.New()
	for _, n := range nodes {
		rel, err := filepath.Rel(root, n.path)
		if err != nil {
			t.Fatalf("relativising %s: %v", n.path, err)
		}
		// hash.Hash never returns an error, which is why these are
		// written without checking one.
		sum.Write([]byte(rel))
		sum.Write([]byte{0})
		if n.dir {
			sum.Write([]byte("dir\x00"))
			continue
		}
		data, err := os.ReadFile(n.path)
		if err != nil {
			t.Fatalf("reading %s: %v", n.path, err)
		}
		sum.Write(data)
		sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil))
}
