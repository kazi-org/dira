// Package acceptance holds E4-L5's end-to-end proofs of dec-0004's two
// invariants: honest degradation (this lane's own TestDegradation) and
// status never stored (TestNeverStored). Every test here shells the real
// `dira` binary — the one place in this epic that is appropriate, since the
// binary's own process boundary (argv, PATH resolution, exit codes, stdout)
// is exactly what is under test. It never shells kazi: every kazi
// interaction goes through a fake script this package writes, reusing
// internal/kazi/testdata/fakekazi/ (E4-L1-T7) for the five failure shapes
// rather than recording a second set.
package acceptance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/ledgertest"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// fakeKaziDir is E4-L1-T7's own fixture directory, referenced rather than
// copied — the same cross-package pattern
// docs/plan/tasks/E4-L3.md's fixture loading and this lane's own task doc
// both use.
const fakeKaziDir = "../../internal/kazi/testdata/fakekazi"

// fakeKaziNames is every FAILING fake this lane's stub matrix drives `dira
// map` against — four of E4-L1-T7's five committed scripts. The fifth
// failure reason, ReasonNotOnPath, has no script at all (an empty PATH
// directory IS that case, exactly as internal/kazi/failopen_test.go's own
// "no kazi at all" sub-test handles it) and control.sh is the SIXTH,
// succeeding fixture used separately as the positive control — so
// fakeKaziDir's five files map to "four failing scripts plus one
// succeeding one", not "five failing fakes", matching what E4-L1-T7 itself
// committed. Kept explicit here (rather than only ever discovered by
// listing the directory) so a fake added to fakeKaziDir without this list
// being updated — or vice versa — is caught by
// TestFakeKaziListMatchesDirectory in degradation_test.go.
var fakeKaziNames = []string{"exit2.sh", "nonjson.sh", "sleepy.sh", "wrongkind.sh"}

// fakeKaziTotalOnDisk is fakeKaziNames' four failing scripts plus
// control.sh — every file fakeKaziDir is expected to hold.
const fakeKaziTotalOnDisk = 5

// buildDiraOnce builds cmd/dira exactly once for this whole test binary run.
var (
	diraBinaryOnce sync.Once
	diraBinaryPath string
	diraBinaryErr  error
)

// buildDira returns the path to a freshly-built dira binary, built once and
// shared across every test in this package.
func buildDira(t *testing.T) string {
	t.Helper()
	diraBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "dira-acceptance-bin")
		if err != nil {
			diraBinaryErr = err
			return
		}
		diraBinaryPath = filepath.Join(dir, "dira")
		cmd := exec.Command("go", "build", "-o", diraBinaryPath, "github.com/kazi-org/dira/cmd/dira")
		cmd.Dir = repoRoot(t)
		out, err := cmd.CombinedOutput()
		if err != nil {
			diraBinaryErr = fmt.Errorf("building dira: %v\n%s", err, out)
		}
	})
	if diraBinaryErr != nil {
		t.Fatalf("%v", diraBinaryErr)
	}
	return diraBinaryPath
}

// repoRoot resolves this module's root via `go env GOMOD`.
func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("go env GOMOD: %v", err)
	}
	return filepath.Dir(strings.TrimSpace(string(out)))
}

// installFakeKazi copies fakeKaziDir/name into a fresh temp directory as
// "kazi", executable, and returns that directory.
func installFakeKazi(t *testing.T, name string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(fakeKaziDir, name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kazi"), src, 0o755); err != nil {
		t.Fatalf("installing fake %s: %v", name, err)
	}
	return dir
}

// installScript writes an ad-hoc script as an executable "kazi" in a fresh
// temp directory.
func installScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kazi"), []byte(body), 0o755); err != nil {
		t.Fatalf("installing ad-hoc kazi script: %v", err)
	}
	return dir
}

// ledgerFixture materialises entries into a fresh .dira and returns its
// parent directory (where `dira -C <dir>` expects to find `.dira/`).
func ledgerFixture(t *testing.T, entries []*ledger.Entry) string {
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
	for _, e := range entries {
		if err := store.Create(t.Context(), e); err != nil {
			t.Fatalf("writing %s: %v", e.ID, err)
		}
	}
	return root
}

// mkEntry builds one fixture entry off ledgertest's shared shape (already
// satisfying Entry.Validate()) and applies mutate.
func mkEntry(id string, mutate func(*ledger.Entry)) *ledger.Entry {
	e := ledgertest.Entry(id)
	if mutate != nil {
		mutate(e)
	}
	return e
}

// runDiraMap runs `dira -C dir map [args...]` with PATH restricted to
// kaziPathDir (or unset if empty) and returns its exit code, stdout and
// stderr.
func runDiraMap(t *testing.T, binary, dir, kaziPathDir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	cmdArgs := append([]string{"map", "-C", dir}, args...)
	cmd := exec.Command(binary, cmdArgs...)
	env := envWithoutPath(os.Environ())
	if kaziPathDir != "" {
		env = append(env, "PATH="+kaziPathDir)
	} else {
		env = append(env, "PATH=")
	}
	cmd.Env = env
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("running dira map: %v", err)
		}
	}
	return code, outBuf.String(), errBuf.String()
}

// envWithoutPath returns env with every PATH entry removed, so the caller
// can set exactly the PATH it wants (a single directory, or none).
func envWithoutPath(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}
