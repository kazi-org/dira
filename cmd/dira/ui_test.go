package main

import (
	"errors"
	"strings"
	"testing"
)

// These tests call runUI directly rather than going through the command
// registry, because `ui` is not registered in main.go yet — that file is the
// integrator's and this lane does not edit it. The registry line is one entry in
// newApp's slice; it is quoted verbatim in docs/decisions-pending/E6-L2-report.md.
//
// What is tested here is the CLI surface: what the flags mean, what a refusal
// looks like, and what exit code a hook would see. The serving path is covered
// in internal/ui, which runs a real server against a real ledger.

// runUIWith calls the command and maps its error the way app.main does, so the
// exit code asserted here is the one a hook would observe.
func runUIWith(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	out, errOut := &strings.Builder{}, &strings.Builder{}
	a := newApp(out, errOut)
	err := runUI(a, args)

	if err == nil {
		return exitOK, out.String(), errOut.String()
	}
	code = exitError
	var ue *usageError
	if errors.As(err, &ue) {
		code = exitUsage
	}
	errOut.WriteString(err.Error())
	return code, out.String(), errOut.String()
}

func TestUIRefusesToBindOffThisMachine(t *testing.T) {
	t.Parallel()

	// cst-0004 says dira never requires a network service, and cst-0003 says
	// a private ledger's text never leaves. A `dira ui` reachable from the
	// LAN breaks both by accident, from one flag typed once.
	for _, addr := range []string{"0.0.0.0:0", ":9999", "192.168.1.10:0", "example.com:80"} {
		code, stdout, stderr := runUIWith(t, "-C", "../..", "-addr", addr)
		if code != exitUsage {
			t.Errorf("dira ui -addr %s exited %d, want %d (a caller asking for something dira will not do)",
				addr, code, exitUsage)
		}
		if !strings.Contains(stderr, "cst-0004") {
			t.Errorf("dira ui -addr %s refused without naming the constraint:\n%s", addr, stderr)
		}
		if stdout != "" {
			t.Errorf("dira ui -addr %s wrote to stdout: %q", addr, stdout)
		}
	}
}

func TestUIHelpIsTheSameTextBothWays(t *testing.T) {
	t.Parallel()

	// A flag documented in one place and not the other is a flag nobody
	// finds, which is why the command carries its own usage rather than
	// letting the top-level list stand in for it.
	code, viaFlag, _ := runUIWith(t, "-h")
	if code != exitOK {
		t.Errorf("dira ui -h exited %d; asking for help is the answer, not an error", code)
	}
	viaHelp := &strings.Builder{}
	writeUIUsage(viaHelp)

	if viaFlag != viaHelp.String() {
		t.Error("`dira ui -h` and the command's own usage differ")
	}
	for _, want := range []string{"/e/<id>", "-addr", "cst-0004", "JavaScript disabled", "loopback"} {
		if !strings.Contains(viaFlag, want) {
			t.Errorf("dira ui's help does not mention %q", want)
		}
	}
	if uiSummary == "" {
		t.Error("uiSummary is empty; the registry line would render a blank description")
	}
}

func TestUIRejectsArguments(t *testing.T) {
	t.Parallel()

	code, _, stderr := runUIWith(t, "dec-0001")
	if code != exitUsage {
		t.Errorf("dira ui dec-0001 exited %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "no arguments") {
		t.Errorf("the message does not say the command takes no arguments:\n%s", stderr)
	}
}

func TestUIUsagefCarriesItsOwnHelp(t *testing.T) {
	t.Parallel()

	err := uiUsagef("something %s", "wrong")
	var ue *usageError
	if !errors.As(err, &ue) {
		t.Fatalf("uiUsagef returned %T, want a *usageError so main selects exit 2", err)
	}
	if ue.usage == nil {
		t.Error("a mistake inside `dira ui` must print ui's help, not the list of commands")
	}
}
