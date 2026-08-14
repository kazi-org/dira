package index_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/index/indextest"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/fixture"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// ledgerDir materialises the shared 200-entry fixture and returns its .dira
// directory.
func ledgerDir(t *testing.T) string {
	t.Helper()

	entries, err := fixture.Generate(fixture.Seed, fixture.Size)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return indextest.Materialise(t, entries)
}

func openIndex(t *testing.T, diraDir string) *index.Index {
	t.Helper()

	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("local.Open: %v", err)
	}
	ix, err := index.Open(context.Background(), store, local.CacheDir(diraDir))
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	return ix
}

// entryPath is the file a test edits behind the cache's back. Tests are allowed
// to know this; production code above the backend is not (dec-0005).
func entryPath(diraDir, id string) string {
	return filepath.Join(diraDir, "entries", id+".md")
}

// ---------------------------------------------------------------------------
// acc: the read-path suite runs twice, warm and cold, byte-identical
// ---------------------------------------------------------------------------

// TestTheSameAnswersComeBackWithAndWithoutACache is the lane's strongest
// predicate: every query in the harness, run once against a warm cache and once
// with .dira/cache/ deleted immediately beforehand, byte for byte the same.
func TestTheSameAnswersComeBackWithAndWithoutACache(t *testing.T) {
	t.Parallel()
	indextest.RunTwice(t, ledgerDir(t))
}

// TestTheHarnessWouldNoticeADifference is the other half. A differential test
// that cannot fail is worth nothing, so this feeds it a query whose answer
// really does depend on whether a cache was there and asserts it fails.
func TestTheHarnessWouldNoticeADifference(t *testing.T) {
	t.Parallel()

	calls := 0
	fake := &testing.T{}
	indextest.RunTwice(fake, ledgerDir(t), indextest.Query{
		Name: "a query that answers differently every time it is asked",
		Run: func(_ context.Context, _ *index.Index) (string, error) {
			calls++
			return fmt.Sprintf("call %d\n", calls), nil
		},
	})
	if calls != 2 {
		t.Errorf("the planted query ran %d times, want 2 (once warm, once cold)", calls)
	}
	if !fake.Failed() {
		t.Error("RunTwice passed a query whose output differs between the warm and cold runs; " +
			"the harness is not comparing anything")
	}
}

// ---------------------------------------------------------------------------
// acc: a cache/file disagreement resolves to the file, with no manual reindex
// ---------------------------------------------------------------------------

// TestAnEditBehindTheCacheResolvesToTheFile is dec-0002's "if the cache and the
// files ever disagree, the files win", exercised by making them disagree.
//
// The second case is the one that matters and the reason dec-0015 exists.
// `state: accepted` and `state: rejected` are the same length, so an edit that
// flips a decision changes no file size — and a restore that puts modification
// times back (rsync -a, cp -p, tar -p) changes no mtime either. Under the
// modification-time-and-size version this backend originally shipped, that edit
// is invisible and dira reports the old state forever. Under a content hash it
// cannot be.
func TestAnEditBehindTheCacheResolvesToTheFile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		edit func(t *testing.T, path string, original []byte) []byte
	}{
		{
			name: "an ordinary edit",
			edit: func(t *testing.T, path string, original []byte) []byte {
				t.Helper()
				edited := bytesReplace(t, original, "state: accepted", "state: rejected")
				edited = append(edited, []byte("\nA paragraph the cache has never seen.\n")...)
				write(t, path, edited)
				return edited
			},
		},
		{
			name: "an edit preserving both size and modification time",
			edit: func(t *testing.T, path string, original []byte) []byte {
				t.Helper()
				before, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				edited := bytesReplace(t, original, "state: accepted", "state: rejected")
				write(t, path, edited)
				if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
					t.Fatal(err)
				}

				after, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				// If these ever stop holding, this case has
				// stopped testing what it was written for.
				if after.Size() != before.Size() {
					t.Fatalf("the edit changed the file size (%d -> %d); it was supposed to be size-preserving",
						before.Size(), after.Size())
				}
				if !after.ModTime().Equal(before.ModTime()) {
					t.Fatalf("the modification time was not restored (%v -> %v)", before.ModTime(), after.ModTime())
				}
				return edited
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diraDir := ledgerDir(t)
			ctx := context.Background()

			// Warm a cache that believes dec-0001 is accepted.
			target := "dec-0001"
			func() {
				ix := openIndex(t, diraDir)
				refs, err := ix.Select(ctx, index.Selector{States: []ledger.State{ledger.StateAccepted}})
				if err != nil {
					t.Fatal(err)
				}
				if !containsID(refs, target) {
					t.Fatalf("%s is not accepted in the fixture; this test is aimed at the wrong entry", target)
				}
			}()

			path := entryPath(diraDir, target)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			tc.edit(t, path, original)

			// No reindex. Just another query, as a hook would run.
			ix := openIndex(t, diraDir)

			entry, err := ix.Entry(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			if entry.State != ledger.StateRejected {
				t.Errorf("Entry(%s).State = %q, want %q — the file says rejected", target, entry.State, ledger.StateRejected)
			}

			accepted, err := ix.Select(ctx, index.Selector{States: []ledger.State{ledger.StateAccepted}})
			if err != nil {
				t.Fatal(err)
			}
			if containsID(accepted, target) {
				t.Errorf("%s is still selected as accepted after the file was changed to rejected.\n"+
					"dec-0002: if the cache and the files ever disagree, the files win. This one did not.", target)
			}

			rejected, err := ix.Select(ctx, index.Selector{States: []ledger.State{ledger.StateRejected}})
			if err != nil {
				t.Fatal(err)
			}
			if !containsID(rejected, target) {
				t.Errorf("%s is not selected as rejected after the file was changed to rejected", target)
			}
		})
	}
}

// TestAnEntryAddedOrRemovedBehindTheCacheIsNoticed covers the other two ways the
// ledger and the cache can disagree: a file that appeared and a file that went.
func TestAnEntryAddedOrRemovedBehindTheCacheIsNoticed(t *testing.T) {
	t.Parallel()

	diraDir := ledgerDir(t)
	ctx := context.Background()

	before := len(selectAll(t, openIndex(t, diraDir)))

	if err := os.Remove(entryPath(diraDir, "note-0001")); err != nil {
		t.Fatal(err)
	}
	added := &ledger.Entry{
		ID: "note-9001", Kind: ledger.KindNote, Title: "An entry the cache has never seen",
		State: ledger.StateActive, Created: "2026-07-30T09:00:00Z", Body: "\nAdded behind the cache's back.\n",
	}
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, added); err != nil {
		t.Fatal(err)
	}

	ix := openIndex(t, diraDir)
	after := selectAll(t, ix)
	if len(after) != before {
		t.Errorf("the ledger has %d entries after one removal and one addition, the index reports %d", before, len(after))
	}
	if containsID(after, "note-0001") {
		t.Error("note-0001 was deleted from the ledger and is still in the index")
	}
	if !containsID(after, "note-9001") {
		t.Error("note-9001 was added to the ledger and is not in the index")
	}
	if got := ix.Stats().Removed; got != 1 {
		t.Errorf("Stats().Removed = %d, want 1", got)
	}
}

// TestNothingChangedMeansNothingIsRead is the reason the cache exists at all: a
// second invocation over an untouched ledger must read no entry file. If this
// fails the cache is doing the work twice rather than saving it.
func TestNothingChangedMeansNothingIsRead(t *testing.T) {
	t.Parallel()

	diraDir := ledgerDir(t)
	first := openIndex(t, diraDir)
	if got := first.Stats().Indexed; got != fixture.Size {
		t.Fatalf("a cold index read %d entry files, want %d", got, fixture.Size)
	}

	second := openIndex(t, diraDir)
	if got := second.Stats().Indexed; got != 0 {
		t.Errorf("a warm index re-read %d entry files with nothing changed, want 0", got)
	}
	if got := second.Stats().Entries; got != fixture.Size {
		t.Errorf("the warm index holds %d entries, want %d", got, fixture.Size)
	}
}

// TestOnlyTheChangedEntryIsReread pins the incremental path. One edited file
// means one file re-parsed, not two hundred.
func TestOnlyTheChangedEntryIsReread(t *testing.T) {
	t.Parallel()

	diraDir := ledgerDir(t)
	_ = openIndex(t, diraDir)

	path := entryPath(diraDir, "note-0002")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, append(original, []byte("\nOne more line.\n")...))

	ix := openIndex(t, diraDir)
	if got := ix.Stats().Indexed; got != 1 {
		t.Errorf("one entry changed and the index read %d files, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// acc: dira reindex rebuilds from files alone, reproducibly
// ---------------------------------------------------------------------------

// TestReindexFromNothingReproducesTheSameIndex deletes the cache, rebuilds it,
// and asserts the rebuilt index is identical to the one that was thrown away —
// both in what it answers and in every row it holds.
func TestReindexFromNothingReproducesTheSameIndex(t *testing.T) {
	t.Parallel()

	diraDir := ledgerDir(t)
	ctx := context.Background()
	cacheDir := local.CacheDir(diraDir)

	first := openIndex(t, diraDir)
	wantAnswers := answers(t, first)
	if !first.Stats().Cached {
		t.Fatalf("the first index wrote no cache, so there is nothing to rebuild: %s", first.Notice())
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	wantRows := cacheRows(t, cacheDir)

	if err := os.RemoveAll(cacheDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(index.Path(cacheDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the cache survived deletion: %v", err)
	}

	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := index.OpenFresh(ctx, store, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Close() }()

	stats := second.Stats()
	if stats.Entries != fixture.Size {
		t.Errorf("Reindex indexed %d entries, want %d", stats.Entries, fixture.Size)
	}
	if stats.Indexed != fixture.Size {
		t.Errorf("Reindex read %d entry files, want %d — a reindex that skips files is not rebuilding from the files alone", stats.Indexed, fixture.Size)
	}
	if !stats.Cached {
		t.Errorf("Reindex did not write a cache: %s", second.Notice())
	}

	if got := answers(t, second); got != wantAnswers {
		t.Error("a rebuilt cache answers differently from the one it replaced")
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if got := cacheRows(t, cacheDir); got != wantRows {
		t.Errorf("a rebuilt cache holds different rows from the one it replaced.\n"+
			"`dira reindex` rebuilds from the entry files alone, so two rebuilds of the same files must agree "+
			"down to the row (dec-0002).\n%s", firstDifference(wantRows, got))
	}
}

// TestAFreshOpenRepairsACacheTheVersionCheckCannotSee is why `dira reindex`
// exists at all given that every query already reconciles. A row that is wrong
// in a field the version does not cover — because something other than dira
// wrote it — survives a reconcile and does not survive a rebuild.
func TestAFreshOpenRepairsACacheTheVersionCheckCannotSee(t *testing.T) {
	t.Parallel()

	diraDir := ledgerDir(t)
	ctx := context.Background()

	ix := openIndex(t, diraDir)
	before := answers(t, ix)
	if err := ix.Close(); err != nil {
		t.Fatal(err)
	}

	// Corrupt one row in place, keeping its version, which is exactly the
	// state a reconcile is blind to by construction.
	corruptTitle(t, local.CacheDir(diraDir), "dec-0002", "a title no file ever held")

	reconciled := openIndex(t, diraDir)
	refs, err := reconciled.Select(ctx, index.Selector{})
	if err != nil {
		t.Fatal(err)
	}
	if !titleIs(refs, "dec-0002", "a title no file ever held") {
		t.Skip("the reconcile already repaired the row, so there is nothing left for OpenFresh to prove")
	}
	if err := reconciled.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := index.OpenFresh(ctx, store, local.CacheDir(diraDir))
	if err != nil {
		t.Fatalf("OpenFresh: %v", err)
	}
	defer func() { _ = rebuilt.Close() }()

	if got := rebuilt.Stats().Indexed; got != fixture.Size {
		t.Errorf("OpenFresh read %d files, want %d", got, fixture.Size)
	}
	if got := answers(t, rebuilt); got != before {
		t.Error("the rebuild did not restore the index to what the files say")
	}
}

// ---------------------------------------------------------------------------
// acc: the cache is gitignored
// ---------------------------------------------------------------------------

// TestTheCacheIsGitignored asserts cst-0003's mechanism rather than its
// intention. A private: true entry's title is written into this database, so a
// cache that reached a public commit would publish it — and the gitignore is the
// only thing stopping that.
func TestTheCacheIsGitignored(t *testing.T) {
	t.Parallel()

	git, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("no git on PATH: %v", err)
	}

	// This repository's own pre-commit hook runs exactly this suite, and a
	// git hook's subprocess environment carries GIT_DIR (and friends,
	// GIT_INDEX_FILE among them) — see `env | grep ^GIT_` from inside
	// hooks/pre-commit. Inherited here, that makes `git rev-parse
	// --show-toplevel` skip its normal upward discovery from cwd and, with
	// no GIT_WORK_TREE set and cwd sitting in a package subdirectory
	// (internal/index, where `go test` actually runs this binary from),
	// misreport the CURRENT directory as the top level instead of the real
	// repository root. `-C <that wrong root> check-ignore` then evaluates
	// the target path against a work tree that does not contain the real
	// .gitignore at all, and reports it not ignored — a false red with
	// nothing wrong in the tree. Stripping the discovery-affecting GIT_*
	// vars reproduces ordinary, non-hook discovery every time. See
	// docs/lore.md.
	env := stripGitDiscoveryEnv(os.Environ())

	revParse := exec.Command(git, "rev-parse", "--show-toplevel")
	revParse.Env = env
	root, err := revParse.Output()
	if err != nil {
		t.Skipf("not in a git work tree: %v", err)
	}
	repo := strings.TrimSpace(string(root))

	// The path the code actually uses, not a copy of it written out here.
	target := index.Path(local.CacheDir(".dira"))
	if target != filepath.Join(".dira", "cache", "index.db") {
		t.Fatalf("the cache lives at %s, which is not the path .gitignore and the acceptance line name", target)
	}

	cmd := exec.Command(git, "-C", repo, "check-ignore", "-q", target)
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		t.Errorf("git check-ignore %s exited non-zero (%v): git would commit the derived cache.\n"+
			"cst-0003 makes that a security bug, not a housekeeping one — the cache holds the titles of "+
			"private: true entries, and a private entry in public git history cannot be un-published.", target, err)
	}

	// A rule that ignores everything would pass the check above without
	// meaning anything.
	notIgnored := exec.Command(git, "-C", repo, "check-ignore", "-q", filepath.Join(".dira", "entries", "dec-0002.md"))
	notIgnored.Env = env
	if err := notIgnored.Run(); err == nil {
		t.Error("git also ignores .dira/entries/dec-0002.md; the ignore rule is too broad to be evidence of anything")
	}
}

// stripGitDiscoveryEnv returns env with the GIT_* variables that override
// git's normal repository/work-tree discovery removed, so a git subprocess
// launched from within an already-running git hook (which sets several of
// these for its own children) discovers a git command's target repository
// from the given working directory exactly as it would outside any hook.
func stripGitDiscoveryEnv(env []string) []string {
	blocked := map[string]bool{
		"GIT_DIR":                 true,
		"GIT_WORK_TREE":           true,
		"GIT_INDEX_FILE":          true,
		"GIT_COMMON_DIR":          true,
		"GIT_CEILING_DIRECTORIES": true,
		"GIT_PREFIX":              true,
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, ok := strings.Cut(kv, "=")
		if ok && blocked[name] {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// ---------------------------------------------------------------------------
// acc: an unwritable cache directory degrades with a notice and exit 0
// ---------------------------------------------------------------------------

func TestAnUnusableCacheDirectoryDegradesRatherThanFailing(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("permission bits do not work this way on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission bits do not deny root")
	}

	cases := []struct {
		name    string
		breakIt func(t *testing.T, diraDir, cacheDir string)
	}{
		{
			name: "the parent directory is read-only",
			breakIt: func(t *testing.T, diraDir, _ string) {
				t.Helper()
				chmod(t, diraDir, 0o555)
			},
		},
		{
			name: "the cache directory is read-only",
			breakIt: func(t *testing.T, _, cacheDir string) {
				t.Helper()
				if err := os.MkdirAll(cacheDir, 0o755); err != nil {
					t.Fatal(err)
				}
				chmod(t, cacheDir, 0o555)
			},
		},
		{
			name: "the cache directory is a file",
			breakIt: func(t *testing.T, _, cacheDir string) {
				t.Helper()
				write(t, cacheDir, []byte("not a directory"))
			},
		},
		{
			// The read-only checkout: a cache is there, it may even
			// be valid, and nothing may be written to it. dira must
			// not trust a cache it cannot reconcile, so this is a
			// degradation and not a fast path.
			name: "the cache directory is read-only and already holds a cache",
			breakIt: func(t *testing.T, _, cacheDir string) {
				t.Helper()
				if err := os.MkdirAll(cacheDir, 0o755); err != nil {
					t.Fatal(err)
				}
				write(t, index.Path(cacheDir), []byte("a cache this build cannot use"))
				chmod(t, cacheDir, 0o555)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diraDir := ledgerDir(t)
			cacheDir := local.CacheDir(diraDir)

			// The answers a working cache gives, taken before the
			// directory is broken.
			want := answers(t, openIndex(t, diraDir))
			if err := os.RemoveAll(cacheDir); err != nil {
				t.Fatal(err)
			}

			tc.breakIt(t, diraDir, cacheDir)
			t.Cleanup(func() { _ = os.Chmod(diraDir, 0o755) })

			store, err := local.Open(diraDir)
			if err != nil {
				t.Fatal(err)
			}
			ix, err := index.Open(context.Background(), store, cacheDir)
			if err != nil {
				t.Fatalf("Open returned an error rather than degrading: %v.\n"+
					"An unwritable cache directory is a slower dira, not a broken one.", err)
			}
			defer func() { _ = ix.Close() }()

			if ix.Stats().Cached {
				t.Error("Stats().Cached is true, but the cache directory was not usable")
			}
			if ix.Notice() == "" {
				t.Error("Notice() is empty: the degradation happened silently, and the acceptance line requires it be stated")
			}
			if got := answers(t, ix); got != want {
				t.Error("the degraded index answers differently from the cached one")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The cache's own failure modes
// ---------------------------------------------------------------------------

func TestACorruptCacheIsDiscardedAndRebuilt(t *testing.T) {
	t.Parallel()

	diraDir := ledgerDir(t)
	cacheDir := local.CacheDir(diraDir)

	first := openIndex(t, diraDir)
	want := answers(t, first)
	// Closed first, so the write below corrupts the database rather than
	// racing a live handle that still has the real pages in memory.
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	// A file that is not a SQLite database at all, in a directory that is
	// perfectly writable.
	write(t, index.Path(cacheDir), []byte("this is not a database, it is a sandwich"))

	ix := openIndex(t, diraDir)
	if !ix.Stats().Cached {
		t.Errorf("a corrupt cache in a writable directory should have been replaced, not abandoned: %s", ix.Notice())
	}
	if !ix.Stats().Rebuilt {
		t.Error("Stats().Rebuilt is false; the corrupt database was apparently reused")
	}
	if got := answers(t, ix); got != want {
		t.Error("the rebuilt cache answers differently")
	}
}

func TestACacheFromAnotherSchemaIsDiscarded(t *testing.T) {
	t.Parallel()

	diraDir := ledgerDir(t)
	cacheDir := local.CacheDir(diraDir)
	want := answers(t, openIndex(t, diraDir))

	setSchemaVersion(t, cacheDir, 999)

	ix := openIndex(t, diraDir)
	if !ix.Stats().Rebuilt {
		t.Error("a cache written by a different schema version was reused rather than rebuilt")
	}
	if got := ix.Stats().Indexed; got != fixture.Size {
		t.Errorf("the rebuild read %d entry files, want %d", got, fixture.Size)
	}
	if got := answers(t, ix); got != want {
		t.Error("the rebuilt cache answers differently")
	}
}

func TestAMalformedEntryIsSkippedAndReported(t *testing.T) {
	t.Parallel()

	diraDir := ledgerDir(t)
	write(t, entryPath(diraDir, "dec-0003"), []byte("---\nthis: is not\n  valid: frontmatter\n---\n"))

	ix := openIndex(t, diraDir)
	stats := ix.Stats()
	if len(stats.Invalid) != 1 || stats.Invalid[0] != "dec-0003" {
		t.Errorf("Stats().Invalid = %v, want [dec-0003]", stats.Invalid)
	}
	if !strings.Contains(ix.Notice(), "dec-0003") {
		t.Errorf("Notice() does not name the unreadable entry: %q", ix.Notice())
	}
	if stats.Entries != fixture.Size-1 {
		t.Errorf("the index holds %d entries, want %d — one bad file should take out one entry, not the ledger", stats.Entries, fixture.Size-1)
	}
	if containsID(selectAll(t, ix), "dec-0003") {
		t.Error("dec-0003 is unreadable and is still being selected")
	}
}

// TestSeveralIndexesOverOneCacheAgree is the parallel-hooks case: two sessions
// firing SessionStart at the same moment against the same ledger. The processes
// are separate; here they are goroutines with their own database handles, which
// is the same contention through the same locking.
func TestSeveralIndexesOverOneCacheAgree(t *testing.T) {
	t.Parallel()

	diraDir := ledgerDir(t)
	if err := os.RemoveAll(local.CacheDir(diraDir)); err != nil {
		t.Fatal(err)
	}

	const racers = 8
	var wg sync.WaitGroup
	results := make([]string, racers)
	errs := make([]error, racers)

	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store, err := local.Open(diraDir)
			if err != nil {
				errs[i] = err
				return
			}
			ix, err := index.Open(context.Background(), store, local.CacheDir(diraDir))
			if err != nil {
				errs[i] = err
				return
			}
			defer func() { _ = ix.Close() }()
			refs, err := ix.Select(context.Background(), index.Selector{})
			if err != nil {
				errs[i] = err
				return
			}
			var b strings.Builder
			for _, ref := range refs {
				fmt.Fprintf(&b, "%s %s %s %s\n", ref.ID, ref.Kind, ref.State, ref.Title)
			}
			results[i] = b.String()
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("racer %d: %v", i, err)
		}
	}
	for i, got := range results {
		if got == "" {
			t.Fatalf("racer %d produced nothing", i)
		}
		if i > 0 && got != results[0] {
			t.Errorf("racer %d disagrees with racer 0:\n%s", i, firstDifference(results[0], got))
		}
	}
}

// ---------------------------------------------------------------------------
// The query API E1-L4 and E1-L5 are blocked on
// ---------------------------------------------------------------------------

func TestSelect(t *testing.T) {
	t.Parallel()

	ix := openIndex(t, ledgerDir(t))
	ctx := context.Background()

	t.Run("a zero selector matches everything", func(t *testing.T) {
		refs, err := ix.Select(ctx, index.Selector{})
		if err != nil {
			t.Fatal(err)
		}
		if len(refs) != fixture.Size {
			t.Errorf("got %d refs, want %d", len(refs), fixture.Size)
		}
	})

	t.Run("kind and state narrow conjunctively", func(t *testing.T) {
		refs, err := ix.Select(ctx, index.Selector{
			Kinds:  []ledger.Kind{ledger.KindDecision},
			States: []ledger.State{ledger.StateAccepted},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(refs) == 0 {
			t.Fatal("no accepted decisions in a fixture that is mostly accepted decisions")
		}
		for _, ref := range refs {
			if ref.Kind != ledger.KindDecision || ref.State != ledger.StateAccepted {
				t.Fatalf("%s is %s/%s, which the selector excluded", ref.ID, ref.Kind, ref.State)
			}
		}
	})

	t.Run("WithEdge selects only entries declaring that edge", func(t *testing.T) {
		refs, err := ix.Select(ctx, index.Selector{
			Kinds:    []ledger.Kind{ledger.KindQuestion},
			States:   []ledger.State{ledger.StateOpen},
			WithEdge: ledger.EdgeBlocks,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(refs) == 0 {
			t.Fatal("no open questions carrying a blocks edge; cst-0001's open blockers would render empty")
		}
		for _, ref := range refs {
			entry, err := ix.Entry(ctx, ref.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !declaresEdge(entry, ledger.EdgeBlocks) {
				t.Errorf("%s was selected for a blocks edge it does not have", ref.ID)
			}
		}

		// And the complement is non-empty, or the filter is a no-op.
		all, err := ix.Select(ctx, index.Selector{
			Kinds:  []ledger.Kind{ledger.KindQuestion},
			States: []ledger.State{ledger.StateOpen},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(all) <= len(refs) {
			t.Errorf("every open question carries a blocks edge (%d of %d), so WithEdge filtered nothing", len(refs), len(all))
		}
	})

	t.Run("the order is newest first and total", func(t *testing.T) {
		refs, err := ix.Select(ctx, index.Selector{})
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(refs); i++ {
			prev, cur := refs[i-1], refs[i]
			if cur.Created > prev.Created {
				t.Fatalf("%s (%s) sorts after %s (%s)", cur.ID, cur.Created, prev.ID, prev.Created)
			}
			if cur.Created == prev.Created && cur.ID < prev.ID {
				t.Fatalf("equal created times are not broken by id: %s before %s", prev.ID, cur.ID)
			}
		}
	})

	t.Run("Limit takes a prefix of the same order", func(t *testing.T) {
		all, err := ix.Select(ctx, index.Selector{})
		if err != nil {
			t.Fatal(err)
		}
		limited, err := ix.Select(ctx, index.Selector{Limit: 7})
		if err != nil {
			t.Fatal(err)
		}
		if len(limited) != 7 {
			t.Fatalf("got %d refs, want 7", len(limited))
		}
		for i, ref := range limited {
			if ref.ID != all[i].ID {
				t.Fatalf("limited[%d] = %s, unlimited[%d] = %s", i, ref.ID, i, all[i].ID)
			}
		}
	})

	t.Run("private is carried through", func(t *testing.T) {
		refs, err := ix.Select(ctx, index.Selector{})
		if err != nil {
			t.Fatal(err)
		}
		private := 0
		for _, ref := range refs {
			entry, err := ix.Entry(ctx, ref.ID)
			if err != nil {
				t.Fatal(err)
			}
			if entry.Private != ref.Private {
				t.Errorf("%s: index says private=%v, the file says %v", ref.ID, ref.Private, entry.Private)
			}
			if ref.Private {
				private++
			}
		}
		if private == 0 {
			t.Error("no private entry in the fixture; cst-0003's cases would go untested")
		}
	})
}

func TestResolve(t *testing.T) {
	t.Parallel()

	ix := openIndex(t, ledgerDir(t))
	ctx := context.Background()

	t.Run("an id resolves to itself alone", func(t *testing.T) {
		got, err := ix.Resolve(ctx, "dec-0001")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0] != "dec-0001" {
			t.Errorf("Resolve(dec-0001) = %v, want [dec-0001]", got)
		}
	})

	t.Run("an id that does not exist resolves to nothing", func(t *testing.T) {
		got, err := ix.Resolve(ctx, "dec-9999")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("Resolve(dec-9999) = %v, want []", got)
		}
	})

	t.Run("a term matches titles case-insensitively", func(t *testing.T) {
		lower, err := ix.Resolve(ctx, "derived cache")
		if err != nil {
			t.Fatal(err)
		}
		if len(lower) == 0 {
			t.Fatal(`nothing matches "derived cache", which the fixture's titles contain`)
		}
		upper, err := ix.Resolve(ctx, "DERIVED CACHE")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(lower, ",") != strings.Join(upper, ",") {
			t.Errorf("case changes the answer: %v vs %v", lower, upper)
		}
		for _, id := range lower {
			entry, err := ix.Entry(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.ToLower(entry.Title), "derived cache") {
				t.Errorf("%s was matched but its title is %q", id, entry.Title)
			}
		}
	})

	t.Run("a term matches a whole tag and not part of one", func(t *testing.T) {
		got, err := ix.Resolve(ctx, "latency")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == 0 {
			t.Fatal(`nothing matches the tag "latency"`)
		}
		partial, err := ix.Resolve(ctx, "atenc")
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range partial {
			entry, err := ix.Entry(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.ToLower(entry.Title), "atenc") {
				t.Errorf("%s matched %q through a tag substring; tags match whole or not at all", id, "atenc")
			}
		}
	})

	t.Run("an empty term resolves to nothing", func(t *testing.T) {
		got, err := ix.Resolve(ctx, "   ")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("Resolve(\"   \") = %v, want []", got)
		}
	})
}

func TestIn(t *testing.T) {
	t.Parallel()

	ix := openIndex(t, ledgerDir(t))
	ctx := context.Background()

	// Every backlink the index reports must be an edge the source file
	// really declares, and every edge a file declares must be reported.
	refs := selectAll(t, ix)
	want := map[string][]index.Backlink{}
	for _, ref := range refs {
		entry, err := ix.Entry(ctx, ref.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, edge := range entry.Edges {
			want[edge.To] = append(want[edge.To], index.Backlink{From: entry.ID, Type: edge.Type, Note: edge.Note})
		}
	}
	if len(want) == 0 {
		t.Fatal("the fixture declares no edges at all")
	}

	seen := 0
	for _, ref := range refs {
		got, err := ix.In(ctx, ref.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want[ref.ID]) {
			t.Errorf("In(%s) returned %d backlinks, the files declare %d", ref.ID, len(got), len(want[ref.ID]))
			continue
		}
		seen += len(got)
		for _, link := range got {
			if !hasBacklink(want[ref.ID], link) {
				t.Errorf("In(%s) reported %+v, which no file declares", ref.ID, link)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no backlinks were checked")
	}

	t.Run("an entry nothing points at has no backlinks", func(t *testing.T) {
		got, err := ix.In(ctx, "note-9999")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("In(note-9999) = %v, want []", got)
		}
	})
}

func TestEntryReadsTheFileNotTheCache(t *testing.T) {
	t.Parallel()

	diraDir := ledgerDir(t)
	ctx := context.Background()
	ix := openIndex(t, diraDir)

	// Poison the cached title without touching the file, keeping the row's
	// version so no reconcile would notice.
	corruptTitle(t, local.CacheDir(diraDir), "dec-0002", "a title no file ever held")

	entry, err := ix.Entry(ctx, "dec-0002")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Title == "a title no file ever held" {
		t.Error("Entry returned the cached title. Nothing dira renders may come from the cache (dec-0002).")
	}

	t.Run("a missing id is ErrNotFound", func(t *testing.T) {
		if _, err := ix.Entry(ctx, "dec-9999"); !errors.Is(err, ledger.ErrNotFound) {
			t.Errorf("Entry(dec-9999) error = %v, want one wrapping ledger.ErrNotFound", err)
		}
	})

	t.Run("Entries preserves the order asked for", func(t *testing.T) {
		ids := []string{"note-0003", "dec-0001", "int-0002"}
		entries, err := ix.Entries(ctx, ids)
		if err != nil {
			t.Fatal(err)
		}
		for i, e := range entries {
			if e.ID != ids[i] {
				t.Errorf("Entries()[%d] = %s, want %s", i, e.ID, ids[i])
			}
		}
	})
}

func TestAnEmptyLedgerIsNotAnError(t *testing.T) {
	t.Parallel()

	diraDir := indextest.Materialise(t, nil)
	ix := openIndex(t, diraDir)

	refs, err := ix.Select(context.Background(), index.Selector{})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("an empty ledger selected %d entries", len(refs))
	}
	if ix.Notice() != "" {
		t.Errorf("an empty ledger produced a notice: %q", ix.Notice())
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func chmod(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
}

func bytesReplace(t *testing.T, data []byte, from, to string) []byte {
	t.Helper()
	if len(from) != len(to) {
		t.Fatalf("replacing %q with %q changes the length; this helper exists to keep it constant", from, to)
	}
	out := strings.Replace(string(data), from, to, 1)
	if out == string(data) {
		t.Fatalf("%q does not occur in the entry; the test is aimed at the wrong file", from)
	}
	return []byte(out)
}

func selectAll(t *testing.T, ix *index.Index) []index.Ref {
	t.Helper()
	refs, err := ix.Select(context.Background(), index.Selector{})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return refs
}

func containsID(refs []index.Ref, id string) bool {
	for _, ref := range refs {
		if ref.ID == id {
			return true
		}
	}
	return false
}

func titleIs(refs []index.Ref, id, title string) bool {
	for _, ref := range refs {
		if ref.ID == id {
			return ref.Title == title
		}
	}
	return false
}

func declaresEdge(e *ledger.Entry, t ledger.EdgeType) bool {
	for _, edge := range e.Edges {
		if edge.Type == t {
			return true
		}
	}
	return false
}

func hasBacklink(links []index.Backlink, want index.Backlink) bool {
	for _, l := range links {
		if l == want {
			return true
		}
	}
	return false
}

// firstDifference reports the first line two renderings disagree on. Two 200-
// entry dumps differing in one field is otherwise an unreadable failure.
func firstDifference(want, got string) string {
	a, b := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := range max(len(a), len(b)) {
		var left, right string
		if i < len(a) {
			left = a[i]
		}
		if i < len(b) {
			right = b[i]
		}
		if left != right {
			return fmt.Sprintf("line %d:\n  want %q\n  got  %q", i+1, left, right)
		}
	}
	return "no line differs, but the strings do"
}

// answers renders every query in the harness, which is the comparison the
// acceptance line is written in terms of.
func answers(t *testing.T, ix *index.Index) string {
	t.Helper()

	ctx := context.Background()
	var b strings.Builder
	for _, q := range indextest.Queries() {
		out, err := q.Run(ctx, ix)
		if err != nil {
			t.Fatalf("%s: %v", q.Name, err)
		}
		b.WriteString("### " + q.Name + "\n")
		b.WriteString(out)
	}
	return b.String()
}
