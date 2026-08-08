package enforcer

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

// TestTheMatcherCannotReachTheNetwork is dec-0003 and cst-0004 as a test rather
// than a promise.
//
// `dira check` must reach a non-zero exit with no model, no network and no
// agent present — from a pre-commit hook, a CI step, a laptop on a plane. That
// is the binding rule this whole lane was built under, and it is the kind of
// rule that decays silently: nobody adds an HTTP client on purpose, they add a
// library that has one. So the guarantee is checked structurally, against what
// the package can link at all, and not by observing that it happens not to make
// a call today.
//
// The two `net/*` packages that are allowed are the URL and IP-address parsers
// the JSON Schema validator drags in through internal/ledger for its `format`
// keywords. Neither can open a socket. `net` itself, `net/http`, `crypto/tls`
// and `os/exec` can, and none of them is reachable from here.
func TestTheMatcherCannotReachTheNetwork(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("no go toolchain on PATH: %v", err)
	}

	out, err := exec.Command("go", "list", "-deps", "github.com/kazi-org/dira/internal/enforcer").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	packages := strings.Fields(string(out))

	// Without this the test passes just as happily on an empty listing.
	if !slices.Contains(packages, "github.com/kazi-org/dira/internal/ledger") {
		t.Fatalf("go list reported %d packages and not internal/ledger; the check is not measuring anything", len(packages))
	}

	// net/url and net/netip parse strings. Everything else under net can
	// open a connection.
	allowed := []string{"net/url", "net/netip"}
	banned := []string{"net/http", "crypto/tls", "os/exec"}

	for _, pkg := range packages {
		leaks := pkg == "net" ||
			(strings.HasPrefix(pkg, "net/") && !slices.Contains(allowed, pkg)) ||
			slices.Contains(banned, pkg)
		if leaks {
			t.Errorf("the enforcer links %q, so reaching a verdict can touch the network or spawn a process.\n"+
				"dec-0003 and cst-0004 make `dira check` work with the network unplugged, and dec-0014 makes that "+
				"binding for this command specifically: the exit code may never depend on anything outside this binary.",
				pkg)
		}
	}
}

// TestCheckReadsThroughTheLedgerInterface drives the exported entry point,
// which the corpus tests bypass in favour of the cheaper internal seam.
//
// It also pins the error contract: a ledger that cannot be read is an error,
// and a plan that contradicts four things is not.
func TestCheckReadsThroughTheLedgerInterface(t *testing.T) {
	t.Parallel()

	entries := fixtureEntries(t, daemonLedger)
	ctx := context.Background()

	t.Run("a conflict is not an error", func(t *testing.T) {
		v, err := Check(ctx, stubLedger{entries: entries}, demoPlan)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if v.Compliant() {
			t.Fatal("the demo plan was found compliant")
		}
		if v.Conflicts[0].Entry != "dec-0060" {
			t.Errorf("cited %s, want dec-0060", v.Conflicts[0].Entry)
		}
	})

	t.Run("an unreadable ledger is", func(t *testing.T) {
		want := errors.New("the disk is on fire")
		if _, err := Check(ctx, stubLedger{err: want}, demoPlan); !errors.Is(err, want) {
			t.Errorf("Check returned %v, want it to wrap %v", err, want)
		}
	})

	t.Run("no ledger at all is", func(t *testing.T) {
		if _, err := Check(ctx, nil, demoPlan); err == nil {
			t.Error("Check accepted a nil ledger")
		}
	})

	t.Run("an empty ledger enforces nothing", func(t *testing.T) {
		v, err := Check(ctx, stubLedger{}, demoPlan)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if !v.Compliant() || v.Enforced != 0 {
			t.Errorf("an empty ledger produced %d conflicts over %d enforced entries", len(v.Conflicts), v.Enforced)
		}
	})
}

type stubLedger struct {
	entries []*ledger.Entry
	err     error
}

func (s stubLedger) Entries(context.Context) ([]*ledger.Entry, error) {
	return s.entries, s.err
}
