package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/fixture"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// ledgerAt writes about n entries of the shared fixture into a fresh ledger
// under a temporary directory, returning the directory holding .dira — which is
// what `dira reindex -C` is pointed at — and how many entries it actually wrote.
//
// "About n" is not sloppiness on this side: fixture.Generate apportions n across
// the five kinds and its rounding does not always total n exactly, so the tests
// assert against what was written rather than against what was asked for.
func ledgerAt(t *testing.T, n int) (string, int) {
	t.Helper()

	root := t.TempDir()
	diraDir := filepath.Join(root, ".dira")
	if err := os.MkdirAll(filepath.Join(diraDir, "entries"), 0o755); err != nil {
		t.Fatalf("creating %s: %v", diraDir, err)
	}
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("local.Open: %v", err)
	}
	entries, err := fixture.Generate(fixture.Seed, n)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := fixture.Write(context.Background(), store, entries); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	return root, len(entries)
}

// indexedLine is the phrase reindex prints for a given entry count.
func indexedLine(n int) string {
	return fmt.Sprintf("indexed %d entries", n)
}

func cachePath(root string) string {
	return index.Path(local.CacheDir(filepath.Join(root, ".dira")))
}

func TestReindexBuildsTheCacheAndSaysWhatItIndexed(t *testing.T) {
	t.Parallel()

	root, n := ledgerAt(t, 40)
	code, stdout, stderr := exercise(t, "reindex", "-C", root)

	if code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty for a writable cache", stderr)
	}
	if !strings.Contains(stdout, indexedLine(n)) {
		t.Errorf("stdout does not report what was indexed:\n%s", stdout)
	}
	if !strings.Contains(stdout, "edges") {
		t.Errorf("stdout does not report the edge count:\n%s", stdout)
	}
	if _, err := os.Stat(cachePath(root)); err != nil {
		t.Errorf("no cache at %s after reindex: %v", cachePath(root), err)
	}
}

// TestReindexSaysSomethingDifferentForADifferentLedger stops the assertion above
// from passing on a hardcoded string.
func TestReindexSaysSomethingDifferentForADifferentLedger(t *testing.T) {
	t.Parallel()

	smallRoot, smallN := ledgerAt(t, 20)
	largeRoot, largeN := ledgerAt(t, 60)
	_, small, _ := exercise(t, "reindex", "-C", smallRoot)
	_, large, _ := exercise(t, "reindex", "-C", largeRoot)
	if small == large {
		t.Errorf("a 20-entry ledger and a 60-entry ledger both report %q", small)
	}
	if !strings.Contains(small, indexedLine(smallN)) || !strings.Contains(large, indexedLine(largeN)) {
		t.Errorf("the counts do not follow the ledger (%d and %d entries):\nsmall: %slarge: %s", smallN, largeN, small, large)
	}
}

func TestReindexIsIdempotent(t *testing.T) {
	t.Parallel()

	root, _ := ledgerAt(t, 40)
	_, first, _ := exercise(t, "reindex", "-C", root)
	_, second, _ := exercise(t, "reindex", "-C", root)
	if first != second {
		t.Errorf("two reindexes of the same ledger report differently:\n--- first ---\n%s--- second ---\n%s", first, second)
	}
}

// TestReindexRebuildsAfterTheCacheIsDeleted is the acceptance line's verb: the
// cache is derived and disposable, so deleting it and running reindex must put
// it back with no other input than the entry files.
func TestReindexRebuildsAfterTheCacheIsDeleted(t *testing.T) {
	t.Parallel()

	root, _ := ledgerAt(t, 40)
	_, before, _ := exercise(t, "reindex", "-C", root)

	if err := os.RemoveAll(local.CacheDir(filepath.Join(root, ".dira"))); err != nil {
		t.Fatal(err)
	}
	code, after, stderr := exercise(t, "reindex", "-C", root)
	if code != exitOK {
		t.Fatalf("exit code = %d after deleting the cache, want %d\nstderr: %s", code, exitOK, stderr)
	}
	if after != before {
		t.Errorf("a rebuilt cache reports differently:\n--- before ---\n%s--- after ---\n%s", before, after)
	}
	if _, err := os.Stat(cachePath(root)); err != nil {
		t.Errorf("reindex did not recreate the cache: %v", err)
	}
}

// TestReindexOnAnUnwritableCacheExitsZero is the degradation clause. E2 installs
// dira in hooks; a hook that fails on a read-only checkout takes the session with
// it, so this reports and exits 0 rather than raising.
func TestReindexOnAnUnwritableCacheExitsZero(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("permission bits do not work this way on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission bits do not deny root")
	}

	root, n := ledgerAt(t, 40)
	diraDir := filepath.Join(root, ".dira")
	if err := os.Chmod(diraDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(diraDir, 0o755) })

	code, stdout, stderr := exercise(t, "reindex", "-C", root)
	if code != exitOK {
		t.Errorf("exit code = %d, want %d — an unwritable cache directory is a slower dira, not a broken one\nstderr: %s",
			code, exitOK, stderr)
	}
	if !strings.Contains(stderr, "not usable") {
		t.Errorf("stderr does not state the degradation:\n%s", stderr)
	}
	if !strings.Contains(stdout, "no cache was written") {
		t.Errorf("stdout claims to have written a cache it could not write:\n%s", stdout)
	}
	if !strings.Contains(stdout, indexedLine(n)) {
		t.Errorf("stdout does not report the entries it did read:\n%s", stdout)
	}
}

func TestReindexReportsUnreadableEntries(t *testing.T) {
	t.Parallel()

	root, n := ledgerAt(t, 40)
	broken := filepath.Join(root, ".dira", "entries", "dec-0002.md")
	if err := os.WriteFile(broken, []byte("---\nnot: valid\n  frontmatter: at all\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := exercise(t, "reindex", "-C", root)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d — one bad file takes out one entry, not the ledger (dec-0002)\nstderr: %s",
			code, exitOK, stderr)
	}
	if !strings.Contains(stdout, "dec-0002") {
		t.Errorf("stdout does not name the entry it skipped:\n%s", stdout)
	}
	if !strings.Contains(stdout, indexedLine(n-1)) {
		t.Errorf("stdout does not report the %d entries it did index:\n%s", n-1, stdout)
	}
}

func TestReindexOutsideALedgerIsAnError(t *testing.T) {
	t.Parallel()

	code, stdout, stderr := exercise(t, "reindex", "-C", t.TempDir())
	if code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, ".dira") {
		t.Errorf("stderr does not say what was missing:\n%s", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on failure", stdout)
	}
}

func TestReindexFindsTheLedgerFromASubdirectory(t *testing.T) {
	t.Parallel()

	root, n := ledgerAt(t, 20)
	deep := filepath.Join(root, "internal", "pkg", "sub")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := exercise(t, "reindex", "-C", deep)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d — hooks run from wherever the session is\nstderr: %s", code, exitOK, stderr)
	}
	if !strings.Contains(stdout, indexedLine(n)) {
		t.Errorf("stdout:\n%s", stdout)
	}
	if _, err := os.Stat(cachePath(root)); err != nil {
		t.Errorf("the cache was not written beside the ledger it found: %v", err)
	}
}

func TestReindexRejectsArguments(t *testing.T) {
	t.Parallel()

	code, stdout, stderr := exercise(t, "reindex", "everything")
	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "takes no arguments") {
		t.Errorf("stderr:\n%s", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on a usage error", stdout)
	}
}

// TestReindexResolvesACacheFileDisagreementToTheFile is the acceptance line at
// the command boundary rather than inside the package: the file wins, and no
// reindex is needed to make it win.
func TestReindexResolvesACacheFileDisagreementToTheFile(t *testing.T) {
	t.Parallel()

	root, n := ledgerAt(t, 40)
	if code, _, stderr := exercise(t, "reindex", "-C", root); code != exitOK {
		t.Fatalf("exit code = %d: %s", code, stderr)
	}

	// Delete an entry behind the cache's back, then reindex without telling
	// it anything.
	if err := os.Remove(filepath.Join(root, ".dira", "entries", "note-0001.md")); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := exercise(t, "reindex", "-C", root)
	if code != exitOK {
		t.Fatalf("exit code = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, indexedLine(n-1)) {
		t.Errorf("the cache still believes in an entry the ledger no longer has:\n%s", stdout)
	}
}

// TestTheCacheHoldsNoExecutionState is dec-0004 as a check. Execution status is
// kazi's and is joined at read time by E4; a column for it here would make the
// ledger a tracker, which cst-0002 closes the vocabulary to prevent.
func TestTheCacheHoldsNoExecutionState(t *testing.T) {
	t.Parallel()

	for _, state := range []string{"planned", "in-progress", "in_progress", "completed", "blocked"} {
		for _, k := range ledger.Kinds {
			for _, s := range k.States() {
				if string(s) == state {
					t.Errorf("%q is a ledger state as well as an execution state; dec-0004's separation has already gone", state)
				}
			}
		}
	}

	root, _ := ledgerAt(t, 20)
	if code, _, stderr := exercise(t, "reindex", "-C", root); code != exitOK {
		t.Fatalf("exit code = %d: %s", code, stderr)
	}
	data, err := os.ReadFile(cachePath(root))
	if err != nil {
		t.Fatal(err)
	}
	// SQLite keeps the CREATE statements as text in the file header pages,
	// so a column named for execution status would be visible right here.
	for _, forbidden := range []string{"status", "bucket", "kazi", "progress", "completed"} {
		if strings.Contains(strings.ToLower(string(data[:min(len(data), 8192)])), forbidden) {
			t.Errorf("the cache schema mentions %q; the cache holds no execution status and no derived kazi state (dec-0004)", forbidden)
		}
	}
}
