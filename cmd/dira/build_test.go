package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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

// allowedModules is every non-stdlib module the dira binary may link.
//
// E0-L1's acceptance line said "no non-stdlib module" outright. E1 supersedes
// that clause here, deliberately and in writing, because E1 legitimately adds a
// YAML parser to the command path and E1-L3 may add a SQLite driver. The
// replacement is an allowlist rather than a deletion: the clause is the only
// thing standing between this binary and a dependency tree that spends
// int-0002's cold-start budget, and a check removed the first time it is
// inconvenient was never a check.
//
// Adding an entry is a design decision. It should read like one in the diff, and
// it should arrive with a measurement of what it costs a cold start.
//
// E1-L3 is the change that fills it, and here is that measurement. `dira` was a
// 2.76MB stdlib-only binary; wiring `dira reindex` to the ledger and its derived
// cache makes it 11.79MB. Attributed:
//
//	+2.51MB  the ledger codec: yaml.v3, and the JSON Schema validator that
//	         internal/ledger reaches into for SplitFrontmatter, which drags
//	         santhosh-tekuri/jsonschema and golang.org/x/text along with it
//	+6.52MB  modernc.org/sqlite and its transpiled libc — the derived read
//	         cache (dec-0002, dec-0015)
//
// None of it costs measurable *start-up* time, which is the thing int-0002 is
// actually about. Measured two ways: from process entry to main is 3µs with
// these linked and 3µs without, because Go initialises none of them eagerly;
// and the median wall clock of `dira version` over twelve runs is 0.10s for the
// 11.79MB binary against 0.11s for the 2.76MB one — indistinguishable, because
// on this machine process spawn alone (`/usr/bin/true`) is 0.06s and swamps it.
// The cost is download size and supply-chain surface, and the benefit is a read
// path that answers in 15.1ms warm against 30.1ms with no cache at all, over a
// 200-entry ledger. That trade is recorded in full, with its rejected
// alternatives, in dec-0015.
//
// Two of these are worth a second look by whoever owns them, and neither is
// E1-L3's to change:
//
//   - jsonschema and x/text are in the *binary* only because internal/ledger
//     imports the schema package for two small helpers. Moving SplitFrontmatter
//     out of a package that also embeds and compiles a JSON Schema document
//     would take roughly 2MB out of the release for no behaviour change.
//   - golang.org/x/exp and github.com/mattn/go-isatty arrive through
//     modernc.org/sqlite, not through anything dira asked for.
var allowedModules = []string{
	// The ledger codec (E1-L1). Frontmatter is YAML; there is no stdlib YAML.
	"gopkg.in/yaml.v3",

	// The schema package's validator, reached because internal/ledger
	// imports that package for SplitFrontmatter. See the note above.
	"github.com/santhosh-tekuri/jsonschema/v6",
	"golang.org/x/text",

	// The derived read cache (E1-L3, dec-0015). modernc.org/sqlite is the
	// pure-Go SQLite; the rest of this block is its dependency tree, not
	// dira's choices. cgo (mattn/go-sqlite3) was rejected because goreleaser
	// has to cross-compile darwin-arm64 and linux-amd64 from one runner.
	"modernc.org/sqlite",
	"modernc.org/libc",
	"modernc.org/mathutil",
	"modernc.org/memory",
	"github.com/dustin/go-humanize",
	"github.com/google/uuid",
	"github.com/mattn/go-isatty",
	"github.com/ncruces/go-strftime",
	"github.com/remyoudompheng/bigfft",
	"golang.org/x/exp",
	"golang.org/x/sys",
}

// TestCommandPathLinksOnlyAllowedModules is the cobra-exclusion check from the
// E0-L1 acceptance line, superseded by E1 into an allowlist and still a test
// rather than a habit.
//
// dec-0001 chose Go over Elixir on cold-start latency and int-0002 budgets a
// hook invocation at well under 100ms. A CLI framework linked into this binary
// spends that budget, and nothing else in the repo would notice. Test-only
// dependencies are invisible to `go list -deps` by construction, which is why
// the schema validator's YAML and JSON Schema libraries do not trip this.
func TestCommandPathLinksOnlyAllowedModules(t *testing.T) {
	t.Parallel()

	own, foreign := commandDependencies(t)

	// Without this the test passes just as happily on an empty or broken
	// listing, which is the vacuous-green failure it exists to prevent.
	if len(own) == 0 {
		t.Fatal("go list -deps reported no packages from this module; the check is not measuring anything")
	}

	for module, packages := range foreign {
		if slices.Contains(allowedModules, module) {
			continue
		}
		t.Errorf("the dira command path links %s, which is not in allowedModules (dec-0001, int-0002).\n"+
			"It arrives through:\n\t%s\n"+
			"Either route the dependency out of the command path, or add the module to allowedModules "+
			"in cmd/dira/build_test.go with the cold-start cost it adds.",
			module, strings.Join(packages, "\n\t"))
	}
}

// TestTheAllowlistIsNotStale is the other half. An allowlist that accumulates
// entries nobody needs stops being a list of what is permitted and becomes a
// list of what was once convenient, and the next dependency slips in under a
// name that is already there.
func TestTheAllowlistIsNotStale(t *testing.T) {
	t.Parallel()

	_, foreign := commandDependencies(t)
	for _, module := range allowedModules {
		if _, ok := foreign[module]; !ok {
			t.Errorf("allowedModules permits %s, which the command path does not link. "+
				"Remove it: an unused exemption is a hole waiting for something to walk through.", module)
		}
	}
}

// commandDependencies returns the packages the binary links, split into this
// module's own and everything else. The foreign half is keyed by module path,
// because "which modules does the binary link" is the question the budget cares
// about — a package count says nothing about what it costs to start.
func commandDependencies(t *testing.T) (own []string, foreign map[string][]string) {
	t.Helper()

	out, err := exec.Command(goTool(t), "list", "-deps",
		"-f", `{{if not .Standard}}{{.ImportPath}}{{"\t"}}{{if .Module}}{{.Module.Path}}{{end}}{{end}}`,
		commandPackage).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", commandPackage, err, out)
	}

	const ownModule = "github.com/kazi-org/dira"
	foreign = map[string][]string{}
	for _, line := range strings.Split(string(out), "\n") {
		pkg, module, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok || pkg == "" {
			continue
		}
		if module == ownModule {
			own = append(own, pkg)
			continue
		}
		if module == "" {
			// No module of its own: nothing to allowlist by name, so
			// it is attributed to its import path and reported.
			module = pkg
		}
		foreign[module] = append(foreign[module], pkg)
	}
	return own, foreign
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
