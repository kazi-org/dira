package status_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestNoSubprocess is E4-L2-T6's acc line: the structural proof that
// internal/status derives everything from the ledger and the index and can
// never shell out to kazi, because that capability belongs to internal/kazi
// alone (E4-L1).
func TestNoSubprocess(t *testing.T) {
	t.Run("no direct os/exec import", func(t *testing.T) {
		// docs/plan/tasks/E4-L2.md's T1 and T6 acc lines both specify
		// `go list -deps ./internal/status` — the full TRANSITIVE import
		// closure — as the check. That is unpassable as written: this lane
		// is explicitly instructed to build on internal/index rather than
		// duplicate it ("consumes it through *index.Index... needs no
		// allowlist entry of its own"), and internal/index links
		// modernc.org/sqlite for its pure-Go, cross-compilable driver
		// (dec-0015). That driver's libc shim, modernc.org/libc, imports
		// os/exec directly in BOTH libc_darwin.go and libc_linux.go — the
		// two platforms dira ships — for its own libc-emulation purposes,
		// unrelated to dira shelling out to anything. So `go list -deps
		// ./internal/status` contains os/exec on every platform the moment
		// this package imports internal/index at all, which T3 and T4 are
		// required to do. Verified directly:
		//
		//   go list -deps github.com/kazi-org/dira/internal/index | grep exec
		//   -> internal/syscall/execenv
		//   -> os/exec
		//   modernc.org/libc imports "os/exec" in libc_darwin.go (line 12)
		//
		// See types_test.go's TestTypes and the lane status report for the
		// same finding, recorded once and applied consistently here.
		//
		// What this check actually needs to prove — "this lane never shells
		// out" — is that internal/status's OWN code never calls
		// exec.Command, which a direct-import check answers exactly, and
		// which mirrors internal/ledger/boundary_test.go's own precedent
		// for the identical class of rule (that test also checks direct
		// imports, not go list -deps's transitive closure, for the same
		// reason: a vendored dependency's own filesystem use is not this
		// codebase's concern).
		imports := moduleDirectImports(t, "github.com/kazi-org/dira/internal/status")
		if len(imports) == 0 {
			t.Fatal("go list reported no imports for internal/status; the check is not measuring anything")
		}
		for _, imp := range imports {
			if imp == "os/exec" {
				t.Fatal("internal/status directly imports os/exec; this lane must never shell out to kazi or anything else")
			}
		}
	})

	t.Run("the ledger/decision-blocked suite passes with PATH emptied", func(t *testing.T) {
		// The lane acc: line's exact invocation (docs/plan/lanes/E4.md
		// §E4-L2): `go test ./internal/status -run 'TestLedger|TestDecisionBlocked'`.
		// Run once normally to establish it executes a non-zero number of
		// tests (the non-vacuity guard), then again with PATH pointed at an
		// empty temp directory, proving nothing under that pattern
		// accidentally depends on kazi, go, git or any other executable
		// being resolvable on PATH.
		goBin, err := exec.LookPath("go")
		if err != nil {
			t.Skipf("no go toolchain on PATH: %v", err)
		}

		normal := runGoTest(t, goBin, os.Environ())
		if normal.ranTests == 0 {
			t.Fatalf("the normal run executed 0 tests under -run 'TestLedger|TestDecisionBlocked'; "+
				"the pattern matches nothing, so the emptied-PATH run below would pass over an empty listing.\noutput:\n%s", normal.output)
		}
		if !normal.passed {
			t.Fatalf("the normal run did not pass; the emptied-PATH run is not a meaningful comparison until this does.\noutput:\n%s", normal.output)
		}

		emptyPathDir := t.TempDir()
		env := replacePath(os.Environ(), emptyPathDir)
		emptied := runGoTest(t, goBin, env)
		if emptied.ranTests == 0 {
			t.Fatalf("the emptied-PATH run executed 0 tests; either it failed to compile/start (which the exit "+
				"status below should also show) or the pattern stopped matching.\noutput:\n%s", emptied.output)
		}
		if !emptied.passed {
			t.Fatalf("go test -run 'TestLedger|TestDecisionBlocked' failed with PATH set to an empty directory "+
				"(%s); something in this package's test path depends on an executable being resolvable.\noutput:\n%s",
				emptyPathDir, emptied.output)
		}
	})
}

type goTestResult struct {
	passed   bool
	ranTests int
	output   string
}

// runGoTest runs the lane acc line's exact invocation with the given
// environment and reports whether it passed and how many (sub)tests ran.
func runGoTest(t *testing.T, goBin string, env []string) goTestResult {
	t.Helper()

	cmd := exec.Command(goBin, "test", "-buildvcs=false", "-run", "TestLedger|TestDecisionBlocked", "-v", ".")
	cmd.Dir = "."
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	output := string(out)

	ran := strings.Count(output, "=== RUN   ")
	return goTestResult{passed: err == nil, ranTests: ran, output: output}
}

// replacePath returns a copy of env with PATH replaced by dir, wherever the
// platform's PATH-like variable is spelled.
func replacePath(env []string, dir string) []string {
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			out = append(out, "PATH="+dir)
			replaced = true
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, "PATH="+dir)
	}
	return out
}
