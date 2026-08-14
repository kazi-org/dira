package acceptance

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

// nowriteEntries is a small, ordinary fixture ledger — the no-write proof
// does not need bucket coverage, only files to hash.
func nowriteEntries() []*ledger.Entry {
	parent := mkEntry("int-9201", nil)
	child := mkEntry("dec-9202", func(e *ledger.Entry) {
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeDerivesFrom, To: "int-9201"})
	})
	return []*ledger.Entry{parent, child}
}

// hashTree returns the sha256 of every regular file under dir, keyed by
// its path relative to dir.
func hashTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("hashing %s: %v", dir, err)
	}
	return out
}

// copyTreeReadOnly copies src to a fresh directory and chmods every file
// and directory under the copy to 0555 — this lane's recorded substitute
// for a read-only mount (root/container privileges a CI runner or this
// worktree's own sandbox does not have): removing write permission is the
// property "mounted read-only" is actually testing.
func copyTreeReadOnly(t *testing.T, src string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "dira-ro")
	if err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}); err != nil {
		t.Fatalf("copying %s to %s: %v", src, dst, err)
	}
	// chmod bottom-up so a directory's own permission does not block
	// descending into it before its children are chmoded.
	var paths []string
	if err := filepath.Walk(dst, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		t.Fatalf("walking %s: %v", dst, err)
	}
	for i := len(paths) - 1; i >= 0; i-- {
		if err := os.Chmod(paths[i], 0o555); err != nil {
			t.Fatalf("chmod %s: %v", paths[i], err)
		}
	}
	t.Cleanup(func() {
		// Restore write permission before TempDir's own cleanup tries to
		// remove the tree, or removal itself fails on a read-only dir.
		for _, p := range paths {
			_ = os.Chmod(p, 0o755)
		}
	})
	return dst
}

// TestNeverStored is E4-L5-T3's acceptance gate (T4's vocabulary lint folds
// into this same named suite, in vocabulary_test.go).
func TestNeverStored(t *testing.T) {
	binary := buildDira(t)
	kaziDir := installFakeKazi(t, "wrongkind.sh") // any fake; content is irrelevant to this proof

	root := ledgerFixture(t, nowriteEntries())
	// entriesDir, not the whole .dira/ tree: .dira/cache/ holds the
	// derived read cache (dec-0002, dec-0015), a documented, disposable
	// SQLite index that every dira read command legitimately writes or
	// rebuilds on open — observed directly while authoring this test:
	// dira map does write .dira/cache/index.db, and that is not the
	// invariant dec-0004/int-0003 protect. The invariant is that no
	// ENTRY FILE is ever written, so this proof is scoped to
	// .dira/entries/, the same boundary internal/ledger/boundary_test.go
	// draws around writer access.
	diraDir := filepath.Join(root, ".dira", "entries")

	t.Run("sha256 unchanged across a text and a --json run, file set unchanged too", func(t *testing.T) {
		before := hashTree(t, diraDir)
		if len(before) == 0 {
			t.Fatal("hashTree found no files; the fixture is empty")
		}

		code, _, stderr := runDiraMap(t, binary, root, kaziDir)
		if code != 0 {
			t.Fatalf("dira map: exit %d; stderr:\n%s", code, stderr)
		}
		code, _, stderr = runDiraMap(t, binary, root, kaziDir, "--json")
		if code != 0 {
			t.Fatalf("dira map --json: exit %d; stderr:\n%s", code, stderr)
		}

		after := hashTree(t, diraDir)
		if len(after) != len(before) {
			t.Fatalf("file set changed: before had %d file(s), after has %d", len(before), len(after))
		}
		for rel, sum := range before {
			gotSum, ok := after[rel]
			if !ok {
				t.Errorf("%s disappeared after dira map ran", rel)
				continue
			}
			if gotSum != sum {
				t.Errorf("%s: sha256 changed from %s to %s", rel, sum, gotSum)
			}
		}
		for rel := range after {
			if _, ok := before[rel]; !ok {
				t.Errorf("%s appeared after dira map ran, was not in the original set", rel)
			}
		}
	})

	t.Run("a read-only copy (0555) still exits 0 and renders the same content", func(t *testing.T) {
		writableCode, writableStdout, writableStderr := runDiraMap(t, binary, root, kaziDir)
		if writableCode != 0 {
			t.Fatalf("writable run: exit %d; stderr:\n%s", writableCode, writableStderr)
		}

		roRoot := copyTreeReadOnly(t, root)
		roCode, roStdout, roStderr := runDiraMap(t, binary, roRoot, kaziDir)
		if roCode != 0 {
			t.Fatalf("read-only run: exit %d; stderr:\n%s", roCode, roStderr)
		}
		if roStdout != writableStdout {
			t.Errorf("read-only run's output differs from the writable run's:\n--- writable ---\n%s\n--- read-only ---\n%s",
				writableStdout, roStdout)
		}
	})

	t.Run("both sides: a mid-run write would be caught — a mutation control on a scratch copy", func(t *testing.T) {
		scratch := filepath.Join(t.TempDir(), "scratch")
		if err := copyDirForNowriteTest(t, diraDir, scratch); err != nil {
			t.Fatalf("copying scratch ledger: %v", err)
		}
		before := hashTree(t, scratch)

		// The deliberately wrong shim: a stand-in for a regression, never
		// wired into internal/cli or cmd/dira, that writes one byte to one
		// entry file "mid-run".
		var target string
		for rel := range before {
			target = filepath.Join(scratch, rel)
			break
		}
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("reading %s: %v", target, err)
		}
		if err := os.WriteFile(target, append(data, '\n'), 0o644); err != nil {
			t.Fatalf("mutating %s: %v", target, err)
		}

		after := hashTree(t, scratch)
		rel, _ := filepath.Rel(scratch, target)
		if after[rel] == before[rel] {
			t.Fatal("the mutation control's own premise broke: the sha256 must change after a write")
		}
		t.Logf("OBSERVED  mutating %s changed its sha256 from %s to %s — the comparison above would catch this", rel, before[rel], after[rel])
	})
}

// copyDirForNowriteTest is a minimal recursive copy, used only by the
// mutation control above so it never touches the fixture the other two
// sub-tests compare against.
func copyDirForNowriteTest(t *testing.T, src, dst string) error {
	t.Helper()
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
