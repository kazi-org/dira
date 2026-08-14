package drift

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

// copyFixture copies a committed fixture tree into a fresh t.TempDir() and
// returns the copy's root. docs/lore.md L-0014: local.Find walks *up*, so a
// test pointed at testdata/ directly risks silently resolving against
// whatever .dira happens to sit above the process's working directory.
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
// root, and its bytes.
func treeDigest(t *testing.T, root string) string {
	t.Helper()

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

// chmodUnreadable makes path unreadable and restores it on cleanup.
func chmodUnreadable(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
}

// gitStatus is `git status --porcelain` in this repository, captured so a
// subtest can prove Classify or Render changed nothing about it. It is
// compared before/after rather than asserted empty on its own: a worktree
// mid-development legitimately holds staged, uncommitted work.
func gitStatus(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Skipf("no git available to check: %v", err)
	}
	return string(out)
}

func assertGitUnchanged(t *testing.T, before string) {
	t.Helper()
	after := gitStatus(t)
	if after != before {
		t.Errorf("git status --porcelain changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
