package installhooks

// E2-L3-T8 — the installed strings fail open, with the failure path observed
// firing.
//
// This is the task most at risk of proving nothing (its own words): a fake
// dira that exits 0, or a shell that never reached the fake because a real
// dira was found on PATH first, produces a green test that certifies
// nothing. Every scenario below therefore runs its own control FIRST --
// stripping the "2>/dev/null || true" guard and asserting the SAME command
// string exits non-zero without it -- and the suite fails immediately if any
// control does not trip.
//
// This runs the installed command strings through a REAL shell rather than
// calling any Go function; that is the only way to observe the property,
// which lives in the shell guard rather than in this package's own code.
//
// IMPORT NOTE (dec-0005): this file may import os, os/exec and path/filepath
// freely -- non-test files only are read by
// internal/ledger/boundary_test.go, so nothing here puts installhooks on any
// allowlist. spans.go, install.go, uninstall.go and command.go import none of
// them.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakediraMarker is written to stdout by the PATH-isolation probe, and by
// nothing else in this file -- so seeing it proves the shell actually
// resolved `dira` to the fake this test built, not to a real binary
// somewhere else on the machine's PATH.
const fakediraMarker = "FAKEDIRA-E2-L3-T8-fbb237e9"

// buildFakeDira compiles testdata/fakedira into dir/dira once, and returns a
// PATH holding only that directory -- so `dira` on PATH can only ever resolve
// to it.
func buildFakeDira(t *testing.T, dir string) (fakeDira, path string) {
	t.Helper()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go toolchain on PATH: %v", err)
	}
	name := "dira"
	if runtime.GOOS == "windows" {
		name = "dira.exe"
	}
	fakeDira = filepath.Join(dir, name)

	cmd := exec.Command(goBin, "build", "-o", fakeDira, "./testdata/fakedira")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("building the fake dira: %v\n%s", err, out)
	}
	return fakeDira, dir
}

// emptyPATHDir is a directory holding no binaries at all -- the "dira later
// moved or upgraded" scenario, the likeliest real failure this task names.
func emptyPATHDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// runShell runs command through a real shell with PATH and every FAKEDIRA_*
// variable set, isolated from the rest of the process environment so a real
// dira elsewhere, or a stray FAKEDIRA_* left by another test, cannot leak in.
func runShell(t *testing.T, command, path string, extraEnv ...string) (code int, stdout, stderr string) {
	t.Helper()

	cmd := exec.Command("sh", "-c", command)
	cmd.Env = append([]string{"PATH=" + path}, extraEnv...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err == nil {
		return 0, outBuf.String(), errBuf.String()
	}
	var exitErr *exec.ExitError
	if ok := asExitError(err, &exitErr); ok {
		return exitErr.ExitCode(), outBuf.String(), errBuf.String()
	}
	t.Fatalf("running %q: %v", command, err)
	return -1, "", ""
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

// stripGuard removes the trailing "2>/dev/null || true" a command was
// installed with, for the red control: the SAME command, unguarded, must
// fail loudly.
func stripGuard(command string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(command), shellGuard))
}

func installedCommands(t *testing.T) []Registration {
	t.Helper()
	regs, err := Registrations()
	if err != nil {
		t.Fatalf("Registrations: %v", err)
	}
	if len(regs) < 3 {
		t.Fatalf("Registrations() returned %d, want at least 3", len(regs))
	}
	return regs
}

// TestFailOpen is T8's acceptance.
func TestFailOpen(t *testing.T) {
	regs := installedCommands(t)
	fakeDir := t.TempDir()
	fakeDira, fakePATH := buildFakeDira(t, fakeDir)

	// PATH isolation, proven before any fake behaviour is trusted: bare
	// `dira` under the prepared PATH resolves to the fake, evidenced by its
	// own marker output -- so a machine with a real dira installed cannot
	// turn this suite green for the wrong reason.
	t.Run("PATH isolation: dira on PATH resolves to the fake", func(t *testing.T) {
		code, stdout, _ := runShell(t, "dira", fakePATH, "FAKEDIRA_STDOUT="+fakediraMarker)
		if code != 0 {
			t.Fatalf("the fake dira itself exited %d", code)
		}
		if stdout != fakediraMarker {
			t.Fatalf("stdout = %q, want the fake's marker %q -- PATH did not resolve to the fake we built", stdout, fakediraMarker)
		}
		t.Logf("OBSERVED  dira on PATH = %s, output = %q", fakeDira, stdout)
	})

	scenarios := []struct {
		name string
		env  []string
	}{
		{name: "the fake exits 1", env: []string{"FAKEDIRA_EXIT=1"}},
		{name: "the fake writes to stderr and exits 2", env: []string{"FAKEDIRA_EXIT=2", "FAKEDIRA_STDERR=fakedira: a deliberate failure\n"}},
		{name: "the fake is killed by SIGSEGV", env: []string{"FAKEDIRA_SIGNAL=segv"}},
		{name: "no dira on PATH at all", env: nil}, // PATH is the empty dir below, not fakePATH
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			path := fakePATH
			if sc.name == "no dira on PATH at all" {
				path = emptyPATHDir(t)
			}

			for _, r := range regs {
				t.Run(r.Event, func(t *testing.T) {
					unguarded := stripGuard(r.Command)
					if unguarded == r.Command {
						t.Fatalf("command for %s carries no shell guard to strip: %q", r.Event, r.Command)
					}

					// The control, asserted first and per fake and per
					// command string: if this does not trip, nothing below
					// is evidence of anything.
					controlCode, _, _ := runShell(t, unguarded, path, sc.env...)
					if controlCode == 0 {
						t.Fatalf("the CONTROL did not trip: %q exited 0 without its guard -- "+
							"the failure path did not fire, so nothing below is evidence", unguarded)
					}
					t.Logf("OBSERVED  control tripped: unguarded exit %d", controlCode)

					// The guarded string, which is what was actually
					// installed, must exit 0.
					code, stdout, stderr := runShell(t, r.Command, path, sc.env...)
					if code != 0 {
						t.Errorf("guarded command for %s exited %d, want 0: %q\nstderr: %s", r.Event, code, r.Command, stderr)
					}

					// 2>/dev/null did its job: nothing from the fake's own
					// stderr reaches the outer process.
					if wantStderr := envValue(sc.env, "FAKEDIRA_STDERR"); wantStderr != "" && stderr != "" {
						t.Errorf("guarded command for %s leaked stderr, want 2>/dev/null to have discarded it: %q", r.Event, stderr)
					}

					// stdout is deliberately NOT suppressed: it is the
					// product (the brief SessionStart injects, the staged
					// count Stop reports), so a fake writing to stdout must
					// have that output reach the caller.
					if wantStdout := envValue(sc.env, "FAKEDIRA_STDOUT"); wantStdout != "" && stdout != wantStdout {
						t.Errorf("guarded command for %s: stdout = %q, want %q (stdout must pass through)", r.Event, stdout, wantStdout)
					}
				})
			}
		})
	}

	// stdout passthrough, asserted as the contract on its own rather than
	// buried inside a failure scenario: a fake that succeeds AND writes to
	// stdout still has that output reach the caller.
	t.Run("stdout is not suppressed on success either", func(t *testing.T) {
		for _, r := range regs {
			code, stdout, _ := runShell(t, r.Command, fakePATH, "FAKEDIRA_STDOUT=the brief\n")
			if code != 0 {
				t.Errorf("%s: exit = %d, want 0", r.Event, code)
			}
			if stdout != "the brief\n" {
				t.Errorf("%s: stdout = %q, want %q", r.Event, stdout, "the brief\n")
			}
		}
	})

	// A hang is out of the shell guard's reach. Only Claude Code's own
	// timeout bounds one, so this asserts the timeouts FROM THE FILE THE
	// INSTALLER WROTE.
	t.Run("timeouts from the written file bound what the guard cannot", func(t *testing.T) {
		dir := t.TempDir()
		result, err := Install(nil, false)
		if err != nil {
			t.Fatalf("Install: %v", err)
		}
		written := filepath.Join(dir, "settings.json")
		if err := os.WriteFile(written, result.Data, 0o644); err != nil {
			t.Fatalf("writing the fixture: %v", err)
		}
		data, err := os.ReadFile(written)
		if err != nil {
			t.Fatalf("reading %s: %v", written, err)
		}
		writtenRegs, err := ParseRegistrations(data)
		if err != nil {
			t.Fatalf("parsing the written file: %v", err)
		}
		if len(writtenRegs) < 3 {
			t.Fatalf("read %d command string(s) from the written file, want at least 3", len(writtenRegs))
		}

		want := map[string]int{"SessionStart": 5, "Stop": 10, "PreCompact": 60}
		for _, r := range writtenRegs {
			if r.Timeout <= 0 {
				t.Errorf("%s: timeout = %d, want > 0", r.Event, r.Timeout)
			}
			if ceiling, ok := want[r.Event]; ok && r.Timeout > ceiling {
				t.Errorf("%s: timeout = %d, want <= %d", r.Event, r.Timeout, ceiling)
			}
			if !strings.Contains(r.Command, "|| true") {
				t.Errorf("%s: command %q lacks the || true guard", r.Event, r.Command)
			}
		}
		t.Logf("OBSERVED  %d timeout(s) read from the written file: %+v", len(writtenRegs), want)
	})
}

// envValue returns the value FAKEDIRA_STDOUT/FAKEDIRA_STDERR was set to in
// env, or "" if absent.
func envValue(env []string, key string) string {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			return v
		}
	}
	return ""
}
