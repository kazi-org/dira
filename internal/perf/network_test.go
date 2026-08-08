//go:build perf

package perf

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger/fixture"
)

// TestTheBriefOpensNoSocket is cst-0004 asserted against the process rather than
// against its import graph.
//
// cst-0004 says dira never requires a network service, an account or a hosted
// tier, and int-0002's budget is only meaningful if the measured run was not
// quietly waiting on one. Two different claims live in that sentence and they
// need two different mechanisms, which is why this test has two halves:
//
//   - DEPENDENCE. The cold brief run with no network available produces the same
//     stdout, the same stderr and the same exit code, byte for byte, as the same
//     run with the network up. That says dira does not NEED a network.
//   - BEHAVIOUR. The same run, traced, makes no socket-creating syscall at all.
//     That says dira does not TOUCH one, and it is the only one of the two that
//     is evidence rather than inference — a program that opens a socket, fails,
//     and carries on regardless would pass the first check and fail this one.
//
// # Why the cheap version of this test is not written here
//
// The obvious implementation is a denylist over linked packages: assert
// `go list -deps ./cmd/dira` names no networking package. It would be one line,
// it would need no privileges, it would run everywhere — and it would be RED on
// a correct dira binary today. cmd/dira legitimately links net, net/http and
// crypto/tls through internal/ui, which serves the read-only localhost UI. A
// gate that cannot pass the case it exists to certify is docs/lore.md L-0001
// rule 2 in its purest form, and the subtest below pins that fact so a later
// "tighten this up" has to argue with a failing test rather than with a comment.
// internal/nomodel makes the same boundary explicit from the other side.
//
// Linkage is not evidence of abstinence. The runtime trace is, and where the
// platform will not provide one this test SKIPS with the reason named rather
// than asserting something weaker under the same name. NETWORK.md records which
// platforms those are and what the skip costs.
func TestTheBriefOpensNoSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and traces a spawned binary; skipped under -short")
	}

	argv, err := HookBriefArgv(hookSettings)
	if err != nil {
		t.Fatal(err)
	}

	h, err := NewHarness(t.TempDir(), fixture.Size)
	if err != nil {
		if errors.Is(err, ErrNoToolchain) {
			t.Skip(err)
		}
		t.Fatal(err)
	}

	// ---------------------------------------------------------------------
	// The parts that run on every platform, because they are about this
	// test's own instruments rather than about the process under test.
	// ---------------------------------------------------------------------

	t.Run("the linkage shortcut would be red on a correct binary", func(t *testing.T) {
		linked, err := linkedPackages(commandPackage)
		if err != nil {
			if errors.Is(err, ErrNoToolchain) {
				t.Skip(err)
			}
			t.Fatal(err)
		}
		// This is the assertion in the direction nobody writes it. If
		// all three of these ever stop being linked, the denylist
		// becomes writable and NETWORK.md's central claim becomes
		// false — so the doc has to change, and this failing test is
		// what makes that happen instead of the doc quietly rotting.
		var absent []string
		for _, pkg := range []string{"net", "net/http", "crypto/tls"} {
			if !linked[pkg] {
				absent = append(absent, pkg)
			}
		}
		if len(absent) > 0 {
			t.Errorf("cmd/dira no longer links %s.\nNETWORK.md says a denylist over linked networking packages "+
				"would be red on a correct binary, and that sentence rested on this. Re-read the doc before "+
				"changing this test: if the localhost UI has moved out of the command, a linkage check may now "+
				"be writable, and it is a different and stronger check than the one here.",
				strings.Join(absent, ", "))
		}
		// Green, for symmetry: a package cmd/dira genuinely does not
		// link is reported as absent, so the map is not simply true for
		// everything asked of it.
		if linked["net/http/httptest"] {
			t.Error("cmd/dira reports net/http/httptest as linked, which it is not; the dependency set is not being read")
		}
	})

	t.Run("NETWORK.md publishes what the method does and does not prove", func(t *testing.T) {
		// "Publish the method's limits" is the half of this task that
		// is prose, and prose is the half that rots. These are the
		// claims the test above and the trace below actually make; if
		// the doc stops carrying them, this goes red.
		doc, err := os.ReadFile("NETWORK.md")
		if err != nil {
			t.Fatal(err)
		}
		body := string(doc)
		for _, want := range []string{
			"## What this proves",
			"## What this does not prove",
			"## Where it does not run, and what that costs",
			"linkage",
			"net/http",
			"L-0001",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("NETWORK.md does not carry %q", want)
			}
		}
		// Green side of the same reader: a phrase that is not in the
		// document is reported as missing, so the loop above is
		// discriminating rather than always true.
		if strings.Contains(body, "this method proves the absence of all network activity") {
			t.Error("NETWORK.md claims the method proves more than it does")
		}
	})

	t.Run("the trace reader tells a socket line from a clean one", func(t *testing.T) {
		// The trace reader is the piece that decides the verdict, and
		// on a platform where the tracer cannot attach it is the ONLY
		// piece that can be exercised. Both sides, over transcripts in
		// each shape the two tracers emit.
		red := map[string]string{
			"strace, unthreaded":     "socket(AF_INET, SOCK_STREAM|SOCK_CLOEXEC, IPPROTO_TCP) = 3\n+++ exited with 0 +++\n",
			"strace, -f pid prefix":  "[pid  4711] connect(3, {sa_family=AF_INET, sin_port=htons(443)}, 16) = -1 EINPROGRESS\n",
			"strace, bare pid":       "4711  socket(AF_UNIX, SOCK_STREAM, 0) = 4\n",
			"strace, resumed":        "[pid  4711] <... connect resumed>) = 0\n",
			"strace, 32-bit i386":    "socketcall(SYS_SOCKET, [2, 1, 6]) = 3\n",
			"dtrace, printf shape":   "dtrace attached\n[pid 4711] socket()\n",
			"a socket among noise":   "strace: Process 4711 attached\nbrk(NULL) = 0x1\n[pid  4711] socket(AF_INET, SOCK_DGRAM, 0) = 5\n+++ exited with 0 +++\n",
			"connect with no socket": "[pid  4711] connect(9, {sa_family=AF_UNIX, sun_path=\"/var/run/nscd/socket\"}, 110) = -1\n",
		}
		for name, trace := range red {
			if found := socketEvidence(trace); len(found) == 0 {
				t.Errorf("%s: a trace containing a socket syscall read as clean:\n%s", name, excerpt(trace))
			}
		}

		green := map[string]string{
			"strace, nothing matched":    "+++ exited with 0 +++\n",
			"dtrace, attached and quiet": "dtrace attached\n",
			"attach noise only":          "strace: Process 4711 attached\nstrace: Process 4711 detached\n+++ exited with 0 +++\n",
			// The near misses. A reader that flagged these would
			// go red on a correct run and be deleted for being
			// noisy, which is how a real check dies.
			"a path that contains the word": "openat(AT_FDCWD, \"/var/run/nscd/socket\", O_RDONLY) = -1 ENOENT\n",
			"a syscall with a shared stem":  "socketpair_not_a_real_syscall(1) = 0\nunshare(CLONE_NEWNET) = 0\n",
			"a message about sockets":       "strace: could not trace socket syscalls on this kernel\n",
		}
		for name, trace := range green {
			if found := socketEvidence(trace); len(found) > 0 {
				t.Errorf("%s: a clean trace read as carrying a socket syscall: %v\n%s", name, found, excerpt(trace))
			}
		}
	})

	// ---------------------------------------------------------------------
	// The trace itself. Everything below needs a tracer that can actually
	// attach, and says so plainly when it cannot.
	// ---------------------------------------------------------------------

	control, err := buildSocketControl(t)
	if err != nil {
		if errors.Is(err, ErrNoToolchain) {
			t.Skip(err)
		}
		t.Fatal(err)
	}

	// The control has to be a control. Run it untraced first: if it does not
	// open a socket on its own, a traced run of it showing nothing would be
	// read as "the tracer is broken" when the truth is "the positive control
	// is". These two failures look identical from the trace file alone and
	// they route to opposite places, so they are separated here.
	if out, _, code := spawn(h, control); code != 0 || !strings.Contains(out, controlMarker) {
		t.Fatalf("the socket-opening control did not open a socket when run on its own (exit %d), so it cannot "+
			"prove anything about the tracer:\n%s", code, excerpt(out))
	}

	t.Run("the trace plumbing carries a verdict back, and fails loudly when it cannot", func(t *testing.T) {
		// The last piece that can be exercised where the real tracer
		// cannot attach: everything between "run something under a
		// tracer" and "read the verdict" — the output file, the traced
		// command actually running, and the two ways the tracer itself
		// can fail. A stand-in tracer writes a canned transcript and
		// then execs the command, which is the same shape strace and
		// dtrace have, so this covers the plumbing and leaves exactly
		// one thing unverified on darwin: whether a real tracer attaches.
		// NETWORK.md says so in those words.
		if runtime.GOOS == "windows" {
			t.Skip("the stand-in tracer is a /bin/sh script")
		}

		// `sh -c script name canned out argv...`: $0 is name, $1 the
		// canned transcript, $2 the output path, and everything after
		// the shift is the command to run.
		standIn := func(canned string) *tracer {
			return &tracer{
				name: "stand-in",
				tool: "/bin/sh",
				args: func(out string, argv []string) []string {
					return append([]string{
						"-c", `printf '%s\n' "$1" > "$2"; shift 2; exec "$@"`,
						"sh", canned, out,
					}, argv...)
				},
			}
		}

		// Green: a clean transcript comes back intact, is read as
		// clean, and — the part that matters — the traced command
		// really ran. It is traced over `dira reindex` with the cache
		// removed, so "it ran" is a fact on disk rather than a claim.
		if err := h.RemoveCache(); err != nil {
			t.Fatal(err)
		}
		clean, err := standIn("+++ exited with 0 +++").trace(t, h, h.Binary, "reindex")
		if err != nil {
			t.Fatalf("the stand-in tracer failed over a command that succeeds: %v", err)
		}
		if strings.TrimSpace(clean) != "+++ exited with 0 +++" {
			t.Errorf("the transcript did not come back intact: %q", clean)
		}
		if found := socketEvidence(clean); len(found) > 0 {
			t.Errorf("a clean transcript read as carrying a socket syscall: %v", found)
		}
		if err := h.CacheWasBuilt(); err != nil {
			t.Errorf("the traced command did not run, so a trace of it proves nothing: %v", err)
		}

		// Red 1: the same plumbing over a transcript that does carry a
		// socket syscall. Without this the green above is a check that
		// has only ever seen one input.
		dirty, err := standIn("[pid  4711] socket(AF_INET, SOCK_STREAM, IPPROTO_TCP) = 3").trace(t, h, h.Binary, "version")
		if err != nil {
			t.Fatal(err)
		}
		if found := socketEvidence(dirty); len(found) != 1 {
			t.Errorf("a transcript carrying one socket syscall read as %v", found)
		}

		// Red 2: a tracer that writes no trace file at all. This is
		// what strace looks like when it cannot attach, and reading it
		// as a clean run is the false green this whole test is built
		// around — so it must be an error, not an empty string.
		noFile := &tracer{name: "stand-in", tool: "/bin/sh", args: func(out string, argv []string) []string {
			return append([]string{"-c", `shift 1; exec "$@"`, "sh", out}, argv...)
		}}
		if got, err := noFile.trace(t, h, h.Binary, "version"); err == nil {
			t.Errorf("a tracer that wrote no trace file returned %q and no error", got)
		}

		// Red 3: a tracer that exits non-zero. It may still have
		// written a file, so the file alone cannot be the verdict.
		failing := &tracer{name: "stand-in", tool: "/bin/sh", args: func(out string, argv []string) []string {
			return append([]string{"-c", `printf 'partial\n' > "$1"; exit 3`, "sh", out}, argv...)
		}}
		if _, err := failing.trace(t, h, h.Binary, "version"); err == nil {
			t.Error("a tracer that exited non-zero was accepted")
		}
	})

	tr, why := newTracer()
	if tr == nil {
		t.Skip(why + " — see internal/perf/NETWORK.md; E1-L6-T4 owns running this where the method exists")
	}

	// The red side, and the reason it runs before the real subject rather
	// than after: a strace or dtrace invocation that silently failed to
	// attach produces a file with no socket lines in it, which is exactly what
	// a clean run produces. Without a positive control the two are
	// indistinguishable, and the test would certify every binary on earth.
	controlTrace, controlErr := tr.trace(t, h, control)
	if found := socketEvidence(controlTrace); len(found) == 0 {
		t.Skipf("%s produced no socket syscall for a program that provably opens one, so it is not attached on "+
			"this machine and its verdict about dira would be worthless (error: %v).\n"+
			"trace was:\n%s\nSee internal/perf/NETWORK.md.", tr.name, controlErr, excerpt(controlTrace))
	}
	t.Logf("%s is attached: the control program's socket syscalls were observed as %v", tr.name, socketEvidence(controlTrace))

	// And now the subject, cold, so the trace covers the cache build as well
	// as the read.
	if err := h.RemoveCache(); err != nil {
		t.Fatal(err)
	}
	if err := h.CheckCold(); err != nil {
		t.Fatal(err)
	}
	briefTrace, briefErr := tr.trace(t, h, append([]string{h.Binary}, argv...)...)
	if briefErr != nil {
		t.Fatalf("the traced dira run failed, so there is nothing to conclude from its trace: %v\ntrace:\n%s",
			briefErr, excerpt(briefTrace))
	}
	if strings.TrimSpace(briefTrace) == "" {
		t.Fatalf("%s produced no output at all for the dira run. An empty trace means tracing did not happen, "+
			"not that nothing happened, and reading it as a pass is the exact false green this test exists to "+
			"avoid.", tr.name)
	}
	t.Logf("traced `dira %s` with %s over the %d-entry fixture, cache removed first; trace:\n%s",
		strings.Join(argv, " "), tr.name, fixture.Size, excerpt(briefTrace))

	if found := socketEvidence(briefTrace); len(found) > 0 {
		t.Errorf("the measured dira run made %d socket-creating syscall(s), so cst-0004's claim that dira needs no "+
			"network service is not true of this binary, and int-0002's budget was measured over a run that "+
			"touched a socket:\n\t%s", len(found), strings.Join(found, "\n\t"))
	}

	// A traced run that produced no brief proves nothing about a brief.
	if err := h.CacheWasBuilt(); err != nil {
		t.Errorf("the traced cold run built no cache, so what was traced was not the work this claim is about: %v", err)
	}

	t.Run("the run does not depend on a network being available", func(t *testing.T) {
		isolate, why := networkIsolator()
		if isolate == nil {
			t.Skip(why + " — see internal/perf/NETWORK.md")
		}

		if err := h.RemoveCache(); err != nil {
			t.Fatal(err)
		}
		onOut, onErr, onCode := spawn(h, h.Binary, argv...)

		if err := h.RemoveCache(); err != nil {
			t.Fatal(err)
		}
		args := append(append([]string{}, isolate[1:]...), append([]string{h.Binary}, argv...)...)
		offOut, offErr, offCode := spawn(h, isolate[0], args...)

		switch {
		case onOut != offOut:
			t.Errorf("stdout differs between a run with the network up and the same run with no network at all.\n"+
				"with network:\n%s\nwithout:\n%s", excerpt(onOut), excerpt(offOut))
		case onErr != offErr:
			t.Errorf("stderr differs between a run with the network up and the same run with no network at all.\n"+
				"with network:\n%s\nwithout:\n%s", excerpt(onErr), excerpt(offErr))
		case onCode != offCode:
			t.Errorf("exit code differs: %d with the network up, %d with no network at all", onCode, offCode)
		}

		// Green side: the comparison is only meaningful if both runs
		// produced a real brief. Two identical empty strings compare
		// equal and prove nothing.
		if err := h.CheckBrief(offOut); err != nil {
			t.Errorf("the run with no network rendered no brief, so the comparison above was between two "+
				"non-answers: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// reading a trace
// ---------------------------------------------------------------------------

// socketSyscalls is what "opened a socket" means here.
//
// socket and connect are the two a process cannot reach the network without.
// socketcall is the multiplexed 32-bit i386 entry point that carries both, and
// it is included because a check that reads correctly on amd64 and silently
// reads nothing on i386 is worse than one that does not run there at all.
//
// AF_UNIX counts. dira has no business opening any socket at all, and a check
// phrased as "no INTERNET socket" would have to parse strace's argument
// rendering to find the address family — which is a parser that gets it wrong
// once and certifies a connection. The strict claim is both easier to check and
// the one worth making.
var socketSyscalls = []string{"socket", "connect", "socketcall"}

// tracePrefix strips the bookkeeping strace and dtrace put in front of a
// syscall: `[pid  4711] ` from strace -f, a bare `4711  ` from some builds, and
// `<... ` from a call reported as resumed after being interrupted.
var tracePrefix = regexp.MustCompile(`^(\[pid\s+\d+\]\s*|\d+\s+|<\.\.\.\s*)+`)

// socketEvidence returns the trace lines that show a socket-creating syscall,
// and the empty slice for a trace that shows none.
//
// It matches on the syscall NAME followed by `(` or by ` resumed`, after the
// prefixes above, and nowhere else in the line. Two near misses make that
// precision necessary rather than fussy: `openat(AT_FDCWD, "/var/run/nscd/socket"
// ...)` contains the word socket in a path, and strace's own diagnostics are
// prose that can mention any syscall by name. A reader that flagged either would
// be red on a correct run — and a check that is red on correct input is a check
// that gets deleted for being noisy, which is how the real assertion dies.
func socketEvidence(trace string) []string {
	var found []string
	for _, line := range strings.Split(trace, "\n") {
		stripped := tracePrefix.ReplaceAllString(strings.TrimSpace(line), "")
		for _, name := range socketSyscalls {
			if strings.HasPrefix(stripped, name+"(") || strings.HasPrefix(stripped, name+" resumed") {
				found = append(found, strings.TrimSpace(line))
				break
			}
		}
	}
	return found
}

// ---------------------------------------------------------------------------
// the tracers
// ---------------------------------------------------------------------------

// A tracer runs a command under a syscall tracer and returns what the tracer
// wrote, separately from what the command wrote.
type tracer struct {
	name string
	// args builds the tracer's argv around the traced command, writing the
	// trace to out.
	args func(out string, argv []string) []string
	tool string
}

// newTracer returns the tracer for this platform, or nil and the reason there is
// none. The reason is a sentence for a skip message, so it says what is missing
// and why rather than naming a GOOS.
func newTracer() (*tracer, string) {
	switch runtime.GOOS {
	case "linux":
		tool, err := exec.LookPath("strace")
		if err != nil {
			return nil, "no strace on PATH, so no syscall trace of the measured run is available"
		}
		return &tracer{
			name: "strace",
			tool: tool,
			args: func(out string, argv []string) []string {
				return append([]string{
					"-f",
					"-e", "trace=" + strings.Join(socketSyscalls, ","),
					"-o", out,
					"--",
				}, argv...)
			},
		}, ""

	case "darwin":
		tool, err := exec.LookPath("dtrace")
		if err != nil {
			return nil, "no dtrace on PATH, so no syscall trace of the measured run is available"
		}
		return &tracer{
			name: "dtrace",
			tool: tool,
			args: func(out string, argv []string) []string {
				var probes []string
				for _, name := range socketSyscalls {
					probes = append(probes, "syscall::"+name+":entry")
				}
				// BEGIN prints unconditionally so a clean run
				// still produces output — an empty file has to
				// mean "did not trace", and it cannot mean that
				// if a clean run produces one too.
				script := "BEGIN { printf(\"dtrace attached\\n\"); }\n" +
					strings.Join(probes, ",") +
					" /pid == $target/ { printf(\"[pid %d] %s()\\n\", pid, probefunc); }"
				return []string{"-q", "-n", script, "-o", out, "-c", strings.Join(argv, " ")}
			},
		}, ""

	default:
		return nil, "no syscall tracer is wired up for " + runtime.GOOS +
			", so the measured run's socket behaviour is unobserved here"
	}
}

// trace runs argv under the tracer from the harness's directory and returns the
// tracer's output.
//
// The traced command's own stdout and stderr are deliberately discarded: this
// function answers "what syscalls did it make", and a caller that also needs the
// output runs the command again untraced. Mixing the two is how a trace file
// ends up with a brief in the middle of it.
func (tr *tracer) trace(t *testing.T, h *Harness, argv ...string) (string, error) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "trace.txt")

	cmd := exec.Command(tr.tool, tr.args(out, argv)...)
	cmd.Dir = h.Root
	// The tracer needs the ambient environment (a tracer's own libraries,
	// and on macOS its privilege check), while the traced program must see
	// the harness's small one. dtrace -c and strace -- both inherit, so the
	// harness environment is what is set here and the tracer's own needs are
	// met by PATH being in it.
	cmd.Env = h.env
	var stderr strings.Builder
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	data, readErr := os.ReadFile(out)
	if readErr != nil {
		return "", fmt.Errorf("%s wrote no trace file (%v); its stderr was:\n%s", tr.name, readErr, excerpt(stderr.String()))
	}
	if runErr != nil {
		return string(data), fmt.Errorf("%s exited non-zero: %w; its stderr was:\n%s", tr.name, runErr, excerpt(stderr.String()))
	}
	return string(data), nil
}

// networkIsolator returns the argv prefix that runs a command with no network,
// or nil and the reason there is none.
//
// The check is a real one rather than a lookup: unshare is present on every
// Linux distribution and still fails on a kernel or container that forbids
// unprivileged user namespaces, which is a common enough runner configuration
// that discovering it as a mysterious non-zero exit inside the comparison would
// be a bad afternoon.
func networkIsolator() ([]string, string) {
	if runtime.GOOS != "linux" {
		return nil, "network-namespace isolation is a Linux facility and " + runtime.GOOS + " has no equivalent " +
			"that does not require taking the machine's network down"
	}
	tool, err := exec.LookPath("unshare")
	if err != nil {
		return nil, "no unshare on PATH, so the run cannot be given an empty network namespace"
	}
	probe := exec.Command(tool, "-rn", "--", "true")
	if out, err := probe.CombinedOutput(); err != nil {
		return nil, fmt.Sprintf("unshare -rn is not permitted here (%v: %s), so the run cannot be given an empty "+
			"network namespace", err, strings.TrimSpace(string(out)))
	}
	return []string{tool, "-rn", "--"}, ""
}

// ---------------------------------------------------------------------------
// the positive control
// ---------------------------------------------------------------------------

// controlMarker is what the control program prints once it has a socket open.
const controlMarker = "control: opened a socket to "

// controlSource is a program that provably opens a socket, built with the same
// toolchain and the same plain `go build` as the binary under test.
//
// It listens on loopback and dials itself rather than reaching a real address,
// so it needs no network, no DNS and no reachable host — the socket and connect
// syscalls happen either way, and those are the whole point. A control that
// needed the internet would be a control that skips on an offline runner, which
// is precisely the runner this test most wants to work on.
const controlSource = `package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "listen:", err)
		os.Exit(1)
	}
	defer ln.Close()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial:", err)
		os.Exit(1)
	}
	defer c.Close()
	fmt.Println("control: opened a socket to", c.RemoteAddr())
}
`

// buildSocketControl compiles controlSource and returns the path to it.
//
// It is written and built at test time rather than kept as a checked-in package
// because a _test-only main package inside internal/perf would be compiled by
// `go vet` and by the linter as production code, and because a binary that opens
// a socket should not be something the repository ships by accident.
func buildSocketControl(t *testing.T) (string, error) {
	t.Helper()

	goTool, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNoToolchain, err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(controlSource), 0o644); err != nil {
		return "", err
	}
	// Its own module, so it does not inherit dira's and cannot accidentally
	// be measuring dira. Nothing outside the standard library is imported,
	// so this resolves offline.
	init := exec.Command(goTool, "mod", "init", "diraperfcontrol")
	init.Dir = dir
	if out, err := init.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go mod init for the socket control: %w\n%s", err, out)
	}
	binary := filepath.Join(dir, "control")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command(goTool, "build", "-o", binary, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("building the socket control: %w\n%s", err, out)
	}
	return binary, nil
}

// spawn runs a command from the harness's directory and returns its stdout,
// stderr and exit code separately.
//
// Harness.Run returns stdout and folds everything else into an error, which is
// right for a timing sample and wrong here: the dependence check compares all
// three streams byte for byte, and an exit code folded into an error message is
// an exit code that cannot be compared.
func spawn(h *Harness, name string, args ...string) (stdout, stderr string, code int) {
	cmd := exec.Command(name, args...)
	cmd.Dir = h.Root
	cmd.Env = h.env

	var out, errs strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errs
	err := cmd.Run()

	code = 0
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	} else if err != nil {
		code = -1
	}
	return out.String(), errs.String(), code
}

// linkedPackages is `go list -deps` as a set: the linker's own answer to what
// the command contains, rather than a grep's guess.
func linkedPackages(pkg string) (map[string]bool, error) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoToolchain, err)
	}
	out, err := exec.Command(goTool, "list", "-deps", pkg).Output()
	if err != nil {
		return nil, fmt.Errorf("go list -deps %s: %w", pkg, err)
	}
	linked := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			linked[line] = true
		}
	}
	return linked, nil
}
