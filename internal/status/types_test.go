package status_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/status"
)

// TestTypes is E4-L2-T1's acc line.
func TestTypes(t *testing.T) {
	if got := len(status.Buckets); got != 6 {
		t.Fatalf("len(status.Buckets) = %d, want 6", got)
	}

	seen := make(map[status.Bucket]bool, len(status.Buckets))
	for _, b := range status.Buckets {
		if seen[b] {
			t.Errorf("status.Buckets contains %q twice", b)
		}
		seen[b] = true
	}

	// Both sides of the collision check, in order: first prove it can fail,
	// by running it against a poisoned bucket list that aliases one value to
	// a real ledger.State ("open") — entirely a local variable here, never a
	// change to types.go. Only then is it trusted to certify the real
	// constants clean.
	poisoned := []status.Bucket{status.ToBePlanned, status.Bucket(ledger.StateOpen)}
	if err := noBucketStateCollision(poisoned); err == nil {
		t.Fatal("the collision check passed against a bucket list that aliases \"open\" (a real ledger.State); " +
			"it is not able to catch the thing it exists to catch")
	}

	if err := noBucketStateCollision(status.Buckets); err != nil {
		t.Errorf("status.Buckets collides with a ledger.State value: %v", err)
	}

	imports := moduleDirectImports(t, "github.com/kazi-org/dira/internal/status")
	if len(imports) == 0 {
		t.Fatal("go list reported no imports for internal/status; the check is not measuring anything")
	}
	for _, imp := range imports {
		if imp == "os/exec" {
			t.Fatal("internal/status directly imports os/exec; this package must never shell out (see docs/plan/tasks/E4-L2.md)")
		}
	}
}

// noBucketStateCollision fails the first time a candidate Bucket's underlying
// string equals a ledger.State's underlying string, for any state legal for
// any kind. The two vocabularies must never be confusable by value even
// though both are string-backed types.
func noBucketStateCollision(buckets []status.Bucket) error {
	for _, k := range ledger.Kinds {
		for _, s := range k.States() {
			for _, b := range buckets {
				if string(b) == string(s) {
					return &collisionError{bucket: b, state: s}
				}
			}
		}
	}
	return nil
}

type collisionError struct {
	bucket status.Bucket
	state  ledger.State
}

func (e *collisionError) Error() string {
	return "status.Bucket(\"" + string(e.bucket) + "\") equals ledger.State(\"" + string(e.state) + "\")"
}

// moduleDirectImports runs `go list` for pkg and returns the packages it
// imports directly — not go list -deps's full transitive closure.
//
// This is deliberately narrower than the "go list -deps ./internal/status"
// check docs/plan/tasks/E4-L2.md's T1 and T6 acc lines specify verbatim, and
// the narrowing is load-bearing, not a convenience: internal/index (which
// this lane is explicitly told to build on rather than duplicate) links
// modernc.org/sqlite for its pure-Go, cross-compilable driver (dec-0015),
// and that driver's libc shim — modernc.org/libc, in both libc_darwin.go and
// libc_linux.go, the two platforms dira ships — imports os/exec directly,
// for its own libc-emulation purposes and unrelated to dira shelling out to
// anything. So "go list -deps ./internal/status" contains os/exec on every
// platform dira ships the moment this package imports internal/index at
// all, which T3 and T4 are explicitly instructed to do ("consumes it
// through *index.Index... needs no allowlist entry of its own",
// docs/plan/tasks/E4-L2.md). The transitive check the acc lines specify is
// unpassable as written; see the status report filed alongside this lane.
//
// What actually matters — the property "This lane never shells out" is
// checking for — is that internal/status's OWN code never calls
// exec.Command, which a direct-import check answers exactly, and which
// mirrors this repository's own precedent for the identical class of
// problem: internal/ledger/boundary_test.go's TestNoFilesystemImportsAboveTheBackend
// checks direct imports only, for the same reason (a vendored dependency's
// own filesystem use is not this codebase's concern; only its own code's
// use is).
func moduleDirectImports(t *testing.T, pkg string) []string {
	t.Helper()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go toolchain on PATH: %v", err)
	}
	out, err := exec.Command(goBin, "list", "-f", `{{join .Imports "\n"}}`, pkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list %s: %v\n%s", pkg, err, out)
	}
	return strings.Fields(string(out))
}
