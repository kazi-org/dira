// Command fakedira stands in for the real `dira` binary on a PATH T8's tests
// build by hand, so the failure path the installed command strings guard
// against can be made to fire on demand and OBSERVED firing, rather than
// hoped for.
//
// Behaviour is entirely environment-controlled, because the installed
// command strings are FIXED bytes -- dira sniff --stage --quiet
// 2>/dev/null || true and its two siblings -- and nothing in this test suite
// may pass this program an argument the real dira would reject.
//
//	FAKEDIRA_STDOUT   written to stdout verbatim, if set
//	FAKEDIRA_STDERR   written to stderr verbatim, if set
//	FAKEDIRA_SIGNAL   "segv": the process kills itself with SIGSEGV
//	FAKEDIRA_EXIT     the exit code (default 0)
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func main() {
	if os.Getenv("FAKEDIRA_SIGNAL") == "segv" {
		// exec, not self-signal: the Go runtime installs its own SIGSEGV
		// handler (it uses the signal for stack-growth faults on some
		// platforms), so a self-directed kill from inside this process can
		// be caught and turned into a slow runtime crash dump rather than an
		// immediate signal death -- observed taking whole seconds per call.
		// Replacing this process with a bare shell that kills ITSELF removes
		// the Go runtime, and everything it might do with the signal, from
		// the picture entirely.
		path, err := exec.LookPath("sh")
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "fakedira: no sh on PATH: %v\n", err)
			os.Exit(91)
		}
		if err := syscall.Exec(path, []string{"sh", "-c", "kill -SEGV $$"}, os.Environ()); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "fakedira: exec sh: %v\n", err)
			os.Exit(92)
		}
	}

	if out := os.Getenv("FAKEDIRA_STDOUT"); out != "" {
		_, _ = fmt.Fprint(os.Stdout, out)
	}
	if errText := os.Getenv("FAKEDIRA_STDERR"); errText != "" {
		_, _ = fmt.Fprint(os.Stderr, errText)
	}

	code := 0
	if v := os.Getenv("FAKEDIRA_EXIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "fakedira: bad FAKEDIRA_EXIT %q: %v\n", v, err)
			os.Exit(90)
		}
		code = n
	}
	os.Exit(code)
}
