package cli_test

// TestNoLedgerWriter is E4-L5-T5's acc line.
//
// A resolved reading of the locked acc: line's phrase "the map command's
// package graph does not import the ledger writer package", recorded here
// per docs/plan/tasks/E4-L5.md's own instruction rather than left ambiguous.
// Taken completely literally this cannot hold: cmd/dira/map.go must
// construct a concrete ledger.Store to open the real ledger for reading at
// all, and internal/ledger/local is the ONLY concrete Store implementation
// in this repo (dec-0005) — every existing read command (brief, why, check,
// ui, reindex) imports it in its cmd/dira/*.go wrapper for exactly this
// reason. The implementable reading, and the one this test enforces:
// internal/cli, internal/status and internal/kazi — the packages that hold
// dira map's derivation and rendering logic — never import
// internal/ledger/local. Only cmd/dira/map.go, the thin wrapper that
// already holds write-capable filesystem access by construction, is
// exempted.
//
// This uses the full TRANSITIVE `go list -deps` closure, unlike
// internal/status/boundary_test.go's direct-imports-only check for
// os/exec — that check had to narrow to direct imports because
// modernc.org/sqlite (a genuine, third-party dependency of internal/index)
// itself imports os/exec, which would otherwise false-positive on a
// dependency's own filesystem use. No such false positive is possible here:
// internal/ledger/local is this module's own package, not something any
// third-party library could transitively depend on, so the transitive
// closure is exactly what "does not import the ledger writer package"
// means.

import (
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func TestNoLedgerWriter(t *testing.T) {
	const writer = "github.com/kazi-org/dira/internal/ledger/local"

	for _, pkg := range []string{
		"github.com/kazi-org/dira/internal/cli",
		"github.com/kazi-org/dira/internal/status",
		"github.com/kazi-org/dira/internal/kazi",
	} {
		t.Run(pkg, func(t *testing.T) {
			if len(moduleTransitiveDeps(t, pkg)) == 0 {
				t.Fatalf("go list -deps %s reported no dependencies at all; the check is not measuring anything", pkg)
			}
			if err := assertExcludes(t, pkg, writer); err != nil {
				t.Fatalf("%v — the map command's derivation and rendering logic must never be able to "+
					"write to the ledger", err)
			}
		})
	}

	t.Run("positive control: cmd/dira does import the writer", func(t *testing.T) {
		if !slices.Contains(moduleTransitiveDeps(t, "github.com/kazi-org/dira/cmd/dira"), writer) {
			t.Fatalf("go list -deps cmd/dira does not report %s at all — the go list -deps mechanism "+
				"itself is broken (or reports nothing for everything), which is exactly what would make "+
				"the exclusion checks above pass vacuously", writer)
		}
	})

	t.Run("both sides: the exclusion check itself trips on a package that does import the writer", func(t *testing.T) {
		// The same assertExcludes helper the three real checks above use
		// (not a separate, hand-rolled comparison), run against cmd/dira —
		// which does import the writer by construction — to prove the
		// check is capable of failing before trusting that it passed
		// cleanly for internal/cli/status/kazi.
		err := assertExcludes(t, "github.com/kazi-org/dira/cmd/dira", writer)
		if err == nil {
			t.Fatal("the both-sides control's own premise broke: cmd/dira must fail this exact check, " +
				"since docs/plan/tasks/E4-L2.md's own T6 precedent and every read command's cmd/dira/*.go " +
				"wrapper import internal/ledger/local directly")
		}
		t.Logf("OBSERVED  assertExcludes(cmd/dira, %s) correctly failed: %v", writer, err)
	})
}

// assertExcludes reports an error if pkg's transitive dependency closure
// contains dep, or nil if it does not.
func assertExcludes(t *testing.T, pkg, dep string) error {
	t.Helper()
	if slices.Contains(moduleTransitiveDeps(t, pkg), dep) {
		return fmt.Errorf("%s transitively imports %s", pkg, dep)
	}
	return nil
}

// moduleTransitiveDeps returns the full transitive dependency closure of
// pkg, via `go list -deps`.
func moduleTransitiveDeps(t *testing.T, pkg string) []string {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go toolchain on PATH: %v", err)
	}
	out, err := exec.Command(goBin, "list", "-deps", pkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pkg, err, out)
	}
	return strings.Fields(string(out))
}
