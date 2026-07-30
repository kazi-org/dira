package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// commandPackage is the import path of the binary under test. Using the import
// path rather than a relative one means these tests do not care what directory
// `go test` chose to run them in.
const commandPackage = "github.com/kazi-org/dira/cmd/dira"

// goTool locates the toolchain, skipping rather than failing where there is no
// `go` on PATH (a binary-only test environment).
func goTool(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go toolchain on PATH: %v", err)
	}
	return path
}

// TestCommandPathHasNoThirdPartyDependencies is the cobra-exclusion check from
// the E0-L1 acceptance line, made a test rather than a habit.
//
// dec-0001 chose Go over Elixir on cold-start latency and int-0002 budgets a
// hook invocation at well under 100ms. A CLI framework linked into this binary
// spends that budget, and nothing else in the repo would notice. Test-only
// dependencies are invisible to `go list -deps` by construction, which is why
// the schema validator's YAML and JSON Schema libraries do not trip this.
//
// This clause is an E0 predicate, not a constitutional one: E1 legitimately
// adds a YAML parser and a SQLite driver to the command path. Superseding it
// there is expected — deleting it silently is not.
func TestCommandPathHasNoThirdPartyDependencies(t *testing.T) {
	t.Parallel()

	out, err := exec.Command(goTool(t), "list", "-deps",
		"-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", commandPackage).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", commandPackage, err, out)
	}

	const ownPrefix = "github.com/kazi-org/dira/"
	var foreign, own []string
	for _, line := range strings.Split(string(out), "\n") {
		pkg := strings.TrimSpace(line)
		if pkg == "" {
			continue
		}
		if strings.HasPrefix(pkg, ownPrefix) || pkg == strings.TrimSuffix(ownPrefix, "/") {
			own = append(own, pkg)
			continue
		}
		foreign = append(foreign, pkg)
	}

	// Without this the test passes just as happily on an empty or broken
	// listing, which is the vacuous-green failure it exists to prevent.
	if len(own) == 0 {
		t.Fatalf("go list -deps reported no packages from this module; the check is not measuring anything\n%s", out)
	}
	if len(foreign) > 0 {
		t.Errorf("the dira command path must be stdlib-only (dec-0001, int-0002); found %d non-stdlib dependencies:\n\t%s",
			len(foreign), strings.Join(foreign, "\n\t"))
	}
}

// TestVersionIsSetByLdflags builds the real binary twice and runs it, because
// the linker stamp is the one thing an in-process test cannot observe. E0-L4's
// release pipeline depends on this exact mechanism.
func TestVersionIsSetByLdflags(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("builds a binary; skipped under -short")
	}
	goBin := goTool(t)

	binary := filepath.Join(t.TempDir(), "dira")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	cases := []struct {
		name    string
		ldflags []string
		want    string
	}{
		{name: "plain build", want: "dev\n"},
		{name: "stamped build", ldflags: []string{"-ldflags", "-X main.version=1.2.3"}, want: "1.2.3\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"build", "-o", binary}, tc.ldflags...)
			args = append(args, commandPackage)
			if out, err := exec.Command(goBin, args...).CombinedOutput(); err != nil {
				t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
			}

			for _, flag := range []string{"--version", "version"} {
				out, err := exec.Command(binary, flag).Output()
				if err != nil {
					t.Fatalf("%s %s: %v", binary, flag, err)
				}
				if string(out) != tc.want {
					t.Errorf("%s %s printed %q, want %q", binary, flag, out, tc.want)
				}
			}
		})
	}
}

// TestUnknownCommandExitsTwoFromTheRealBinary checks the exit-code contract
// through a process boundary. The in-process test covers the mapping; this
// covers that os.Exit actually carries it, which is what a hook observes.
func TestUnknownCommandExitsTwoFromTheRealBinary(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("builds a binary; skipped under -short")
	}
	goBin := goTool(t)

	binary := filepath.Join(t.TempDir(), "dira")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if out, err := exec.Command(goBin, "build", "-o", binary, commandPackage).CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	cmd := exec.Command(binary, "nosuchcommand")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("running %s nosuchcommand: err = %v, want an exit error", binary, err)
	}
	if got := exit.ExitCode(); got != exitUsage {
		t.Errorf("exit code = %d, want %d", got, exitUsage)
	}
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Errorf("stderr carries no usage block:\n%s", stderr.String())
	}
}
