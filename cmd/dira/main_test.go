package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// exercise runs the real command registry against buffers and returns the exit
// code alongside what each stream received.
func exercise(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = newApp(&out, &errBuf).main(args)
	return code, out.String(), errBuf.String()
}

func TestHelpNamesEveryRegisteredCommand(t *testing.T) {
	t.Parallel()

	// The registry, not a hardcoded list: a command added without a help
	// entry must fail this test rather than ship undocumented.
	registered := newApp(nil, nil).commands
	if len(registered) == 0 {
		t.Fatal("no commands registered; help-lists-every-command would be vacuous")
	}

	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}, {}} {
		name := "bare"
		if len(args) > 0 {
			name = args[0]
		}
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := exercise(t, args...)
			if code != exitOK {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
			}
			for _, c := range registered {
				if !strings.Contains(stdout, c.name) {
					t.Errorf("help does not name registered command %q\n--- stdout ---\n%s", c.name, stdout)
				}
				if !strings.Contains(stdout, c.summary) {
					t.Errorf("help does not carry summary for %q\n--- stdout ---\n%s", c.name, stdout)
				}
			}
		})
	}
}

func TestUnknownCommandIsAUsageError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown subcommand", args: []string{"nosuchcommand"}, want: `unknown command "nosuchcommand"`},
		{name: "unknown flag", args: []string{"--nosuchflag"}, want: "not defined"},
		{name: "unknown command to help", args: []string{"help", "nosuchcommand"}, want: `unknown command "nosuchcommand"`},
		{name: "version with arguments", args: []string{"version", "extra"}, want: "takes no arguments"},
		{name: "help with two arguments", args: []string{"help", "a", "b"}, want: "at most one command name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := exercise(t, tc.args...)
			if code != exitUsage {
				t.Errorf("exit code = %d, want %d", code, exitUsage)
			}
			// A caller parsing stdout must never be handed usage text.
			if stdout != "" {
				t.Errorf("stdout = %q, want empty on a usage error", stdout)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr does not explain the mistake; want substring %q\n--- stderr ---\n%s", tc.want, stderr)
			}
			// Usage errors carry the usage, otherwise exit 2 is unhelpful.
			if !strings.Contains(stderr, "usage:") {
				t.Errorf("stderr carries no usage block\n--- stderr ---\n%s", stderr)
			}
		})
	}
}

func TestRuntimeErrorExitsOneAndDoesNotPrintUsage(t *testing.T) {
	t.Parallel()

	// A test-only command, because no real command can fail at E0 yet. The
	// exit-code contract is depended on by E2's hooks, so it is tested now.
	var out, errBuf bytes.Buffer
	a := newApp(&out, &errBuf)
	a.commands = append(a.commands, &command{
		name:    "boom",
		summary: "test-only failing command",
		run:     func(*app, []string) error { return errors.New("ledger is on fire") },
	})

	if code := a.main([]string{"boom"}); code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty on a runtime error", out.String())
	}
	if !strings.Contains(errBuf.String(), "ledger is on fire") {
		t.Errorf("stderr does not carry the error: %q", errBuf.String())
	}
	if strings.Contains(errBuf.String(), "usage:") {
		t.Errorf("runtime error printed usage; that is reserved for exit %d\n%s", exitUsage, errBuf.String())
	}
}

func TestVersionPrintsBareVersionAndNothingElse(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"--version"}, {"-version"}, {"version"}} {
		t.Run(args[0], func(t *testing.T) {
			code, stdout, stderr := exercise(t, args...)
			if code != exitOK {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
			}
			// Bare, so `dira --version` is directly usable in a shell
			// substitution and E0-L4 can assert an exact tag string.
			if stdout != version+"\n" {
				t.Errorf("stdout = %q, want %q", stdout, version+"\n")
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
		})
	}
}

func TestDefaultVersionIsDev(t *testing.T) {
	t.Parallel()

	// Guards the ldflags contract from the other side: if the default is
	// ever changed to a real-looking version, an unstamped build would lie
	// about which release it is.
	if version != "dev" {
		t.Errorf("version = %q, want %q for an unstamped build", version, "dev")
	}
}

func TestHelpForOneCommand(t *testing.T) {
	t.Parallel()

	code, stdout, stderr := exercise(t, "help", "version")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
	}
	if !strings.Contains(stdout, "print the dira version") {
		t.Errorf("stdout does not carry the command summary: %q", stdout)
	}
}

func TestCommandNamesAreUnique(t *testing.T) {
	t.Parallel()

	// lookup returns the first match, so a duplicate name would silently
	// shadow a command rather than fail.
	seen := map[string]bool{}
	for _, c := range newApp(nil, nil).commands {
		if seen[c.name] {
			t.Errorf("command %q is registered twice", c.name)
		}
		seen[c.name] = true
		if c.summary == "" {
			t.Errorf("command %q has no summary and would render blank in help", c.name)
		}
		if c.run == nil {
			t.Errorf("command %q has no run function and would panic on dispatch", c.name)
		}
	}
}
