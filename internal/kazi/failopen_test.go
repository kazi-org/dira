package kazi

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// E4-L1-T7. Everything in this file goes through the REAL default runner
// (execRunner) against a REAL, temporary PATH — never the injected runFunc
// seam T4/T6's unit tests use. That seam proves decode and classification;
// this file proves the process-invocation itself: LookPath resolution,
// argv, stdout capture, exit-code translation and context-deadline
// enforcement, none of which an in-process fake can certify.
//
// No case here ever shells a real kazi binary, per this lane's founding
// rule. The taxonomy has two members none of the five committed fakes can
// produce — NotOnPath (no kazi at all) and NotFound (Status()'s own
// not-found shape) — and this file derives both from an empty PATH
// directory and a small ad-hoc script written to t.TempDir() respectively,
// rather than by adding a sixth committed source or shelling anything real.
// Either way the process spawned is one this test wrote.

const fakeKaziDir = "testdata/fakekazi"

// installFake copies testdata/fakekazi/<name> into a fresh temp directory as
// "kazi", executable, and returns that directory — the one and only thing
// isolatedPATH below will put on PATH.
func installFake(t *testing.T, name string) string {
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

// installScript writes an ad-hoc script (not one of the five committed
// fakes) as an executable "kazi" in a fresh temp directory.
func installScript(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kazi"), []byte(body), 0o755); err != nil {
		t.Fatalf("installing ad-hoc kazi script: %v", err)
	}
	return dir
}

// isolatedPATH points PATH at exactly dir and nothing else, so whatever
// resolves is provably what this test put there.
func isolatedPATH(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir)
}

// useRealRunner makes sure this file exercises execRunner regardless of
// what a prior test in this package left kaziRunner pointed at, and restores
// whatever was there afterward.
func useRealRunner(t *testing.T) {
	t.Helper()
	old := kaziRunner
	kaziRunner = execRunner
	t.Cleanup(func() { kaziRunner = old })
}

// TestStubBinaryMatrix is E4-L1-T7's acceptance gate.
func TestStubBinaryMatrix(t *testing.T) {
	useRealRunner(t)

	// observed collects every distinct failure signature seen across this
	// test: an UnavailableReason string, or the literal "NotFound" for the
	// one case that is not a Reason at all. The acc line's numeric floor —
	// "failing if its size is less than five" — is checked at the end.
	observed := map[string]bool{}

	t.Run("resolves to the fake, not an ambient kazi", func(t *testing.T) {
		dir := installFake(t, "control.sh")
		isolatedPATH(t, dir)

		resolved, err := exec.LookPath("kazi")
		if err != nil {
			t.Fatalf("LookPath: %v", err)
		}
		if !strings.HasPrefix(resolved, dir) {
			t.Fatalf("LookPath resolved %s, which is not under %s — PATH isolation is not working, and "+
				"nothing else this test observes can be trusted for the reason it claims", resolved, dir)
		}

		snap, err := Snapshot(context.Background())
		if err != nil {
			t.Fatalf("Snapshot against the control fake: %v", err)
		}
		if snap == nil || snap.TotalsBase != 7 {
			t.Fatalf("Snapshot did not return the control fake's own content (got %+v); a machine with a "+
				"real kazi installed would make this suite pass for the wrong reason if this check were "+
				"missing", snap)
		}
	})

	// PATH isolation shown capable of failing, per this task's "both sides"
	// note — but without ever executing a real kazi, per this lane's rule.
	// LookPath alone (no process spawned) is enough to show the mechanism
	// can resolve elsewhere; skipped when this host has no real kazi to
	// resolve to, since there is then nothing for it to prove.
	t.Run("PATH resolution can find something other than the fake (LookPath only, nothing executed)", func(t *testing.T) {
		fakeDir := installFake(t, "control.sh")

		ambient, err := exec.LookPath("kazi")
		if err != nil {
			t.Skip("no kazi on this host's ambient PATH; nothing to prove the isolation mechanism can fail against")
		}
		if strings.HasPrefix(ambient, fakeDir) {
			t.Fatal("the ambient PATH resolved into this test's own fake directory, which cannot happen " +
				"since fakeDir was never added to PATH — the test setup is wrong, not the isolation mechanism")
		}
		t.Logf("OBSERVED  ambient PATH resolves kazi to %s, outside the isolated fake directory; "+
			"the isolation this suite relies on (restricting PATH to exactly one directory) is what "+
			"keeps every case above from resolving here instead. Not executed.", ambient)
	})

	// The control fake, proven against both entry points: the harness is
	// not merely broken in a way that always fails.
	t.Run("control succeeds through both Snapshot and Status", func(t *testing.T) {
		isolatedPATH(t, installFake(t, "control.sh"))

		snap, err := Snapshot(context.Background())
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		if snap == nil {
			t.Fatal("Snapshot returned a nil *Portfolio with a nil error against a well-formed fake")
		}

		run, prop, err := Status(context.Background(), "warnings-clean")
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if run == nil || prop != nil {
			t.Errorf("run=%+v prop=%+v, want a non-nil run and nil prop", run, prop)
		}
	})

	t.Run("exit2: NonZeroExit through both entry points", func(t *testing.T) {
		isolatedPATH(t, installFake(t, "exit2.sh"))

		_, err := Snapshot(context.Background())
		reason := requireUnavailableReason(t, err)
		if reason != ReasonNonZeroExit {
			t.Errorf("Snapshot: Reason = %q, want %q", reason, ReasonNonZeroExit)
		}
		observed[string(reason)] = true

		_, _, err = Status(context.Background(), "anything")
		reason = requireUnavailableReason(t, err)
		if reason != ReasonNonZeroExit {
			t.Errorf("Status: Reason = %q, want %q", reason, ReasonNonZeroExit)
		}
	})

	t.Run("nonjson: MalformedJSON through both entry points", func(t *testing.T) {
		isolatedPATH(t, installFake(t, "nonjson.sh"))

		_, err := Snapshot(context.Background())
		reason := requireUnavailableReason(t, err)
		if reason != ReasonMalformedJSON {
			t.Errorf("Snapshot: Reason = %q, want %q", reason, ReasonMalformedJSON)
		}
		observed[string(reason)] = true

		_, _, err = Status(context.Background(), "anything")
		reason = requireUnavailableReason(t, err)
		if reason != ReasonMalformedJSON {
			t.Errorf("Status: Reason = %q, want %q", reason, ReasonMalformedJSON)
		}
	})

	t.Run("wrongkind: WrongKind through both entry points", func(t *testing.T) {
		isolatedPATH(t, installFake(t, "wrongkind.sh"))

		_, err := Snapshot(context.Background())
		reason := requireUnavailableReason(t, err)
		if reason != ReasonWrongKind {
			t.Errorf("Snapshot: Reason = %q, want %q", reason, ReasonWrongKind)
		}
		observed[string(reason)] = true

		_, _, err = Status(context.Background(), "anything")
		reason = requireUnavailableReason(t, err)
		if reason != ReasonWrongKind {
			t.Errorf("Status: Reason = %q, want %q", reason, ReasonWrongKind)
		}
	})

	t.Run("sleepy: ReasonTimeout, measured within a bounded margin of the deadline", func(t *testing.T) {
		isolatedPATH(t, installFake(t, "sleepy.sh"))

		const deadline = 300 * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), deadline)
		defer cancel()

		start := time.Now()
		_, err := Snapshot(ctx)
		elapsed := time.Since(start)

		reason := requireUnavailableReason(t, err)
		if reason != ReasonTimeout {
			t.Fatalf("Reason = %q, want %q", reason, ReasonTimeout)
		}
		observed[string(reason)] = true

		// The fake sleeps 5s. A timeout implementation that merely waits
		// for the process and then reports ReasonTimeout late would take
		// close to 5s here; this bound (well under half that, generous
		// over the 300ms deadline) fails that implementation and passes
		// one that actually kills the process at the deadline.
		if elapsed > 2*time.Second {
			t.Errorf("Snapshot took %s to report ReasonTimeout against a %s deadline and a 5s sleep; "+
				"this looks like it waited for the process rather than enforcing the deadline", elapsed, deadline)
		}
		t.Logf("OBSERVED  deadline=%s elapsed=%s", deadline, elapsed)
	})

	t.Run("no kazi at all: NotOnPath", func(t *testing.T) {
		// An empty directory, not one of the five fakes — nothing named
		// "kazi" exists on this PATH at all.
		isolatedPATH(t, t.TempDir())

		_, err := Snapshot(context.Background())
		reason := requireUnavailableReason(t, err)
		if reason != ReasonNotOnPath {
			t.Errorf("Reason = %q, want %q", reason, ReasonNotOnPath)
		}
		observed[string(reason)] = true
	})

	t.Run("Status' own not-found shape: NotFound, not Unavailable", func(t *testing.T) {
		// kazi's real status_not_found/2 message (see status.go's
		// statusNotFoundPrefix), ported verbatim, from an ad-hoc script —
		// not one of the five committed fakes, since NotFound only ever
		// applies to Status(), never to Snapshot().
		script := "#!/bin/sh\n" +
			`echo '{"error":"no run or proposal found for ref \"does-not-exist\" ` +
			`(a run appears once it has recorded an iteration; a proposal once proposed)","schema_version":2}'` + "\n" +
			"exit 1\n"
		isolatedPATH(t, installScript(t, script))

		run, prop, err := Status(context.Background(), "does-not-exist")
		if run != nil || prop != nil {
			t.Errorf("run=%+v prop=%+v, want both nil", run, prop)
		}
		var nf *NotFound
		if !errors.As(err, &nf) {
			t.Fatalf("error is %T (%v), want *NotFound", err, err)
		}
		observed["NotFound"] = true
	})

	if len(observed) < 5 {
		t.Errorf("only %d distinct failure signature(s) observed across this suite (%v), want at least 5",
			len(observed), observedKeys(observed))
	}
}

// requireUnavailableReason fails the test if err is not an *Unavailable, and
// returns its Reason otherwise.
func requireUnavailableReason(t *testing.T, err error) UnavailableReason {
	t.Helper()
	if err == nil {
		t.Fatal("got a nil error, want a non-nil *Unavailable")
	}
	var unavail *Unavailable
	if !errors.As(err, &unavail) {
		t.Fatalf("error is %T (%v), want *Unavailable", err, err)
	}
	return unavail.Reason
}

func observedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
