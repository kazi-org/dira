package kazi

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
)

// E4-L1-T4. Every case here goes through the injected runFunc seam
// (withRunner) rather than a real process — the seam T7's harness proves
// actually works end to end. The one exception is "no kazi on PATH", which
// is inherently about PATH resolution and so exercises the real default
// runner (execRunner) against a real, emptied PATH — no process is spawned
// either way, since LookPath fails before exec would even try.
//
// Nothing in this file runs t.Parallel(): kaziRunner and PATH are both
// process-global, and this file's whole job is swapping them out from under
// Snapshot() one case at a time.

// withRunner swaps kaziRunner for fn for the duration of the calling test.
func withRunner(t *testing.T, fn runFunc) {
	t.Helper()
	old := kaziRunner
	kaziRunner = fn
	t.Cleanup(func() { kaziRunner = old })
}

// fixtureRunner returns a runFunc that always succeeds with the named
// fixture's bytes on stdout — the "control" shape most cases below start
// from and then break one way.
func fixtureRunner(t *testing.T, name string) runFunc {
	data := readFixture(t, name)
	return func(_ context.Context, _ []string) ([]byte, int, error) {
		return data, 0, nil
	}
}

// TestSnapshot is E4-L1-T4's acceptance gate.
func TestSnapshot(t *testing.T) {
	t.Run("portfolio-populated.json", func(t *testing.T) {
		withRunner(t, fixtureRunner(t, "portfolio-populated.json"))

		snap, err := Snapshot(context.Background())
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		if snap == nil {
			t.Fatal("Snapshot returned a nil *Portfolio with a nil error")
		}
		// Spot-checked, not exhaustively — T3 already proves the
		// decoder over this exact fixture.
		if snap.SchemaVersion != 2 {
			t.Errorf("SchemaVersion = %d, want 2", snap.SchemaVersion)
		}
		if len(snap.Planned) == 0 {
			t.Error("Planned is empty")
		}
		if len(snap.ByRepo) == 0 {
			t.Error("ByRepo is empty")
		}
		if len(snap.Blocked) == 0 {
			t.Error("Blocked is empty")
		}
		if len(snap.TotalsRows) != 5 {
			t.Errorf("TotalsRows has %d entries, want 5", len(snap.TotalsRows))
		}
	})

	t.Run("portfolio-empty.json", func(t *testing.T) {
		withRunner(t, fixtureRunner(t, "portfolio-empty.json"))

		snap, err := Snapshot(context.Background())
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		if snap == nil {
			t.Fatal("Snapshot returned a nil *Portfolio with a nil error")
		}
		if !snap.TotalsEmpty || !snap.RateEmpty {
			t.Errorf("TotalsEmpty=%t RateEmpty=%t, want both true", snap.TotalsEmpty, snap.RateEmpty)
		}
	})

	// reasons collects every Unavailable.Reason actually observed across the
	// cases below, checked at the end of the test. A build that returned a
	// single catch-all Unavailable{Reason: "error"} for every case would
	// pass each t.Run individually if their assertions were loose, but
	// cannot pass this: the acc line requires four *distinct* reasons.
	reasons := map[UnavailableReason]bool{}

	t.Run("no kazi on PATH", func(t *testing.T) {
		// The real default runner, a real (emptied) PATH — this is the
		// one case in this file about PATH resolution itself, not
		// about a fake process's behaviour, so it does not go through
		// withRunner/fixtureRunner.
		t.Setenv("PATH", "")

		snap, err := Snapshot(context.Background())
		if snap != nil {
			t.Errorf("Snapshot returned a non-nil *Portfolio with no kazi on PATH")
		}
		unavail := requireUnavailable(t, err)
		if unavail.Reason != ReasonNotOnPath {
			t.Errorf("Reason = %q, want %q", unavail.Reason, ReasonNotOnPath)
		}
		reasons[unavail.Reason] = true
	})

	t.Run("stub exiting 2", func(t *testing.T) {
		withRunner(t, func(_ context.Context, _ []string) ([]byte, int, error) {
			return []byte("usage: kazi ...\n"), 2, nil
		})

		snap, err := Snapshot(context.Background())
		if snap != nil {
			t.Errorf("Snapshot returned a non-nil *Portfolio for a stub exiting 2")
		}
		unavail := requireUnavailable(t, err)
		if unavail.Reason != ReasonNonZeroExit {
			t.Errorf("Reason = %q, want %q", unavail.Reason, ReasonNonZeroExit)
		}
		if unavail.Detail == "" || !strings.Contains(unavail.Detail, strconv.Itoa(2)) {
			t.Errorf("Detail = %q, want it to name the observed exit code (2)", unavail.Detail)
		}
		reasons[unavail.Reason] = true
	})

	t.Run("stub printing non-JSON", func(t *testing.T) {
		withRunner(t, func(_ context.Context, _ []string) ([]byte, int, error) {
			return []byte("not json at all\n"), 0, nil
		})

		snap, err := Snapshot(context.Background())
		if snap != nil {
			t.Errorf("Snapshot returned a non-nil *Portfolio for a stub printing non-JSON")
		}
		unavail := requireUnavailable(t, err)
		if unavail.Reason != ReasonMalformedJSON {
			t.Errorf("Reason = %q, want %q", unavail.Reason, ReasonMalformedJSON)
		}
		reasons[unavail.Reason] = true
	})

	t.Run("stub printing JSON whose kind is not portfolio", func(t *testing.T) {
		withRunner(t, func(_ context.Context, _ []string) ([]byte, int, error) {
			return []byte(`{"schema_version":2,"kind":"status"}`), 0, nil
		})

		snap, err := Snapshot(context.Background())
		if snap != nil {
			t.Errorf("Snapshot returned a non-nil *Portfolio for a stub printing kind: \"status\"")
		}
		unavail := requireUnavailable(t, err)
		if unavail.Reason != ReasonWrongKind {
			t.Errorf("Reason = %q, want %q", unavail.Reason, ReasonWrongKind)
		}
		if unavail.Detail != "status" {
			t.Errorf("Detail = %q, want it to name the observed kind (%q)", unavail.Detail, "status")
		}
		reasons[unavail.Reason] = true
	})

	if len(reasons) < 4 {
		t.Errorf("only %d distinct Unavailable.Reason value(s) observed across this test (%v), want at least 4 — "+
			"a single catch-all Unavailable{} would satisfy every case above individually but fail here",
			len(reasons), reasonList(reasons))
	}
}

// requireUnavailable fails the test if err is nil or not an *Unavailable,
// and returns it otherwise.
func requireUnavailable(t *testing.T, err error) *Unavailable {
	t.Helper()
	if err == nil {
		t.Fatal("Snapshot returned a nil error; want a non-nil *Unavailable")
	}
	var unavail *Unavailable
	if !errors.As(err, &unavail) {
		t.Fatalf("Snapshot's error is %T (%v), want *Unavailable", err, err)
	}
	return unavail
}

// reasonList renders the observed reasons for a failure message.
func reasonList(reasons map[UnavailableReason]bool) []UnavailableReason {
	out := make([]UnavailableReason, 0, len(reasons))
	for r := range reasons {
		out = append(out, r)
	}
	return out
}
