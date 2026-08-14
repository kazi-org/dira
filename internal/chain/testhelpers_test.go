package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// copyFixture copies a committed fixture tree into a fresh t.TempDir() and
// returns the copy's root, holding whichever sibling ledgers (me/, sire/,
// repo/) the fixture declares.
//
// docs/lore.md L-0014: local.Find walks *up* looking for the nearest .dira, so
// a test pointed at testdata/ directly risks silently resolving against
// whatever .dira happens to sit above the process's working directory. Every
// fixture in this package is copied first.
func copyFixture(t *testing.T, name string) string {
	t.Helper()

	src := filepath.Join("testdata", "fixtures", name)
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	dst := filepath.Join(t.TempDir(), name)

	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copying fixture %s: %v", name, err)
	}
	return dst
}

// treeDigest is a sha256 over every file under root: its path relative to
// root, and its bytes. It is what proves a read touched nothing, including
// .dira/cache/, which is gitignored and invisible to a git-based check.
func treeDigest(t *testing.T, root string) string {
	t.Helper()

	if _, err := os.Stat(root); err != nil {
		// A directory this package never created (the absent-parent
		// case) has nothing to digest; its absence is the assertion.
		return "absent:" + root
	}

	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	sort.Strings(paths)

	sum := sha256.New()
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("relativising %s: %v", path, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		_, _ = fmt.Fprintf(sum, "%s\n%d\n", rel, len(data))
		sum.Write(data)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// snapshot digests every named subdirectory of root, keyed by name — the
// before-half of the byte-identical assertion every TestWalk subtest makes.
func snapshot(t *testing.T, root string, names []string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(names))
	for _, name := range names {
		out[name] = treeDigest(t, filepath.Join(root, name))
	}
	return out
}

// assertUnchanged re-digests root's named subdirectories and fails the test if
// any of them moved since snapshot was taken — the whole run leaves every
// fixture ledger byte-identical, including a subtest that failed to open a
// hop.
func assertUnchanged(t *testing.T, before map[string]string, root string) {
	t.Helper()
	after := snapshot(t, root, names(before))
	for name, want := range before {
		if got := after[name]; got != want {
			t.Errorf("%s changed during a read: %s -> %s (cst-0003 rule 1: a parent is never written to by a child)", name, want, got)
		}
	}
}

func names(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// chmodTree makes every regular file and directory under root unreadable, and
// registers a cleanup that restores it — chmod 0o000 on a *file* is not
// enough to stop os.ReadFile inside that directory; the directory itself has
// to lose its execute bit.
func chmodUnreadable(t *testing.T, dir string) {
	t.Helper()

	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}
