package kazi

import (
	"context"
	"errors"
	"testing"
)

// E4-L1-T6. Every case here goes through the injected runFunc seam — the
// real process-invocation path is T7's job, not this file's.

func TestStatus(t *testing.T) {
	t.Run("status-run.json decodes to RunStatus only", func(t *testing.T) {
		withRunner(t, fixtureRunner(t, "status-run.json"))

		run, prop, err := Status(context.Background(), "warnings-clean")
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if run == nil {
			t.Fatal("run is nil, want a *RunStatus")
		}
		if prop != nil {
			t.Errorf("prop is non-nil (%+v), want nil alongside a non-nil run", prop)
		}
		if run.Status != "in_progress" && run.Status != "converged" {
			t.Errorf("Status = %q, want \"converged\" or \"in_progress\"", run.Status)
		}
		if run.Ref == "" {
			t.Error("Ref is empty")
		}
	})

	t.Run("status-proposal.json decodes to ProposalStatus only", func(t *testing.T) {
		withRunner(t, fixtureRunner(t, "status-proposal.json"))

		run, prop, err := Status(context.Background(), "prop-e45")
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if prop == nil {
			t.Fatal("prop is nil, want a *ProposalStatus")
		}
		if run != nil {
			t.Errorf("run is non-nil (%+v), want nil alongside a non-nil prop", run)
		}
		if prop.Status == "" {
			t.Error("Status is empty")
		}
		if prop.Ref == "" {
			t.Error("Ref is empty")
		}
	})

	// The red control for the run-lifecycle vocabulary: a third value, per
	// lane doc point 4 — status <ref>'s own `status` field is documented as
	// only converged/in_progress, narrower than by_repo's raw persisted
	// status string.
	t.Run("a third run status value fails to decode", func(t *testing.T) {
		mutated := mutateJSON(t, readFixture(t, "status-run.json"), func(doc map[string]any) {
			doc["status"] = "stuck" // a real by_repo/blocked status, never a status<ref> lifecycle value
		})
		withRunner(t, func(_ context.Context, _ []string) ([]byte, int, error) {
			return mutated, 0, nil
		})

		run, prop, err := Status(context.Background(), "warnings-clean")
		if err == nil {
			t.Fatalf("Status succeeded on status: \"stuck\"; want a decode error, got run=%+v prop=%+v", run, prop)
		}
		var unavail *Unavailable
		if !errors.As(err, &unavail) {
			t.Fatalf("error is %T (%v), want *Unavailable", err, err)
		}
		if unavail.Reason != ReasonMalformedJSON {
			t.Errorf("Reason = %q, want %q", unavail.Reason, ReasonMalformedJSON)
		}
	})

	// notFoundStub is the exit-1 fixture: kazi's own status_not_found/2
	// message, ported verbatim (see statusNotFoundPrefix's doc comment for
	// why this is stdout and not stderr), with schema_version alongside it —
	// the real --json shape.
	notFoundBody := []byte(`{"error":"no run or proposal found for ref \"does-not-exist\" ` +
		`(a run appears once it has recorded an iteration; a proposal once proposed)","schema_version":2}`)

	t.Run("a stub exiting 1 with status_not_found's message unwraps to NotFound", func(t *testing.T) {
		withRunner(t, func(_ context.Context, _ []string) ([]byte, int, error) {
			return notFoundBody, 1, nil
		})

		run, prop, err := Status(context.Background(), "does-not-exist")
		if run != nil || prop != nil {
			t.Errorf("run=%+v prop=%+v, want both nil on a NotFound error", run, prop)
		}
		var nf *NotFound
		if !errors.As(err, &nf) {
			t.Fatalf("error is %T (%v), want *NotFound", err, err)
		}
		if nf.Ref != "does-not-exist" {
			t.Errorf("NotFound.Ref = %q, want %q", nf.Ref, "does-not-exist")
		}
		var unavail *Unavailable
		if errors.As(err, &unavail) {
			t.Errorf("errors.As also matched *Unavailable (%+v); NotFound and Unavailable must be mutually exclusive", unavail)
		}
	})

	t.Run("a stub exiting 2 with a malformed-flags message unwraps to Unavailable", func(t *testing.T) {
		withRunner(t, func(_ context.Context, _ []string) ([]byte, int, error) {
			return []byte("error: unknown option --bogus-flag\n\nusage: kazi status <ref> [--json]\n"), 2, nil
		})

		run, prop, err := Status(context.Background(), "warnings-clean")
		if run != nil || prop != nil {
			t.Errorf("run=%+v prop=%+v, want both nil on an Unavailable error", run, prop)
		}
		var unavail *Unavailable
		if !errors.As(err, &unavail) {
			t.Fatalf("error is %T (%v), want *Unavailable", err, err)
		}
		if unavail.Reason != ReasonNonZeroExit {
			t.Errorf("Reason = %q, want %q", unavail.Reason, ReasonNonZeroExit)
		}
		var nf *NotFound
		if errors.As(err, &nf) {
			t.Errorf("errors.As also matched *NotFound (%+v); the two error paths must be mutually exclusive", nf)
		}
	})
}
