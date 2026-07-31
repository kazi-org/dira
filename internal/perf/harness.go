//go:build perf

package perf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kazi-org/dira/internal/ledger/fixture"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// commandPackage is the binary under measurement, named by import path rather
// than by a relative one so the harness does not care which directory `go test`
// chose to run it in. Same reasoning as cmd/dira/build_test.go.
const commandPackage = "github.com/kazi-org/dira/cmd/dira"

// samples is N. The lane's acceptance line says N>=20; this is the 20.
//
// Twenty is enough for a median to be a median and cheap enough that the whole
// package still runs in well under a minute at roughly a tenth of a second per
// spawn. It is not enough for a stable p99, which is why nothing here asserts
// one — the single-run maximum is asserted instead, over exactly these samples.
const samples = 20

// hookSettings is the file the measured argv is read out of, relative to this
// package. It is read rather than transcribed on purpose: see HookBriefArgv.
const hookSettings = "../../hooks/settings.example.json"

// briefHeader is the first section cst-0001's keep order renders, and the
// cheapest proof that what came back on stdout is a brief at all.
const briefHeader = "open blockers"

// logUnit is what durations are rounded to when reported. Microseconds, not
// milliseconds, because a distribution printed as "97ms 98ms 98ms 121ms" hides
// the spread that explains it — and because every millisecond-valued constant in
// this package lives in budget.go and nowhere else.
const logUnit = time.Microsecond

// ErrNoToolchain is returned where there is no `go` on PATH. Callers skip on it
// rather than failing, matching goTool in cmd/dira/build_test.go: a binary-only
// test environment is not a broken one.
var ErrNoToolchain = errors.New("no go toolchain on PATH")

// SkipReason returns a non-empty string when this machine cannot honestly take
// the measurement, and the empty string when it can.
//
// short: these tests spawn a binary a hundred-odd times and build it first.
//
// race: the detector costs several times the work being measured, so the number
// it would report is about the detector rather than about dira — the reason
// internal/index/latency_test.go states for the same skip. It is detected from
// the build settings the toolchain stamps into the test binary rather than from a
// `race`-tagged file, because this task owns four files and none of them can
// carry a second build constraint.
func SkipReason(short bool) string {
	if short {
		return "spawns and times a built binary; skipped under -short"
	}
	if raceEnabled() {
		return "timing test; the race detector costs several times the work being measured"
	}
	return ""
}

// raceEnabled reports whether this test binary was built with -race.
func raceEnabled() bool {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, setting := range info.Settings {
		if setting.Key == "-race" {
			return setting.Value == "true"
		}
	}
	return false
}

// build is the one `go build` this package performs, memoised: the acceptance
// line says once per test run, and a rebuild per test would put the compiler in
// the samples of whichever test triggered it.
var build struct {
	once   sync.Once
	dir    string
	binary string
	err    error
}

// Binary builds ./cmd/dira and returns the path to it, building at most once per
// process.
func Binary() (string, error) {
	build.once.Do(func() {
		goTool, err := exec.LookPath("go")
		if err != nil {
			build.err = fmt.Errorf("%w: %w", ErrNoToolchain, err)
			return
		}
		build.dir, build.err = os.MkdirTemp("", "dira-perf-bin-")
		if build.err != nil {
			return
		}
		binary := filepath.Join(build.dir, "dira")
		if runtime.GOOS == "windows" {
			binary += ".exe"
		}
		// The plain build, with no ldflags and no tags: the binary a
		// user runs is the binary that is timed. A release build is
		// stamped with -X main.version, which changes no code path this
		// measurement touches.
		out, err := exec.Command(goTool, "build", "-o", binary, commandPackage).CombinedOutput()
		if err != nil {
			build.err = fmt.Errorf("go build -o %s %s: %w\n%s", binary, commandPackage, err, out)
			return
		}
		build.binary = binary
	})
	return build.binary, build.err
}

// CleanupBinary removes the built binary. TestMain calls it; nothing else should.
func CleanupBinary() {
	if build.dir != "" {
		_ = os.RemoveAll(build.dir)
	}
}

// A Harness is a built dira binary plus a ledger for it to read, and the two
// checks that stop a run from reporting a fast number for having done nothing.
type Harness struct {
	// Binary is the built dira.
	Binary string

	// Root is the directory holding .dira, and the working directory every
	// run starts in — local.Find walks up from there, which is how dira
	// finds a ledger from inside a repository.
	Root string

	// DiraDir and CacheDir are the ledger and its derived cache.
	DiraDir  string
	CacheDir string

	// ids is every entry id the fixture generated, and the vocabulary
	// CheckBrief looks for in stdout.
	ids []string

	env []string
}

// NewHarness materialises a ledger of size entries under dir and returns a
// harness over it.
//
// size is normally fixture.Size — the 200 entries every E1 acceptance line is
// written against, generated from fixture.Seed so two lanes are talking about the
// same ledger. A size of zero materialises an EMPTY ledger, which no measurement
// should ever use: it exists so a test can prove that measuring nothing fails
// loudly instead of returning a very fast number.
func NewHarness(dir string, size int) (*Harness, error) {
	binary, err := Binary()
	if err != nil {
		return nil, err
	}

	root := filepath.Join(dir, "workspace")
	diraDir := filepath.Join(root, ".dira")
	if err := os.MkdirAll(filepath.Join(diraDir, "entries"), 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", diraDir, err)
	}

	h := &Harness{
		Binary:   binary,
		Root:     root,
		DiraDir:  diraDir,
		CacheDir: local.CacheDir(diraDir),
		// A deliberately small environment, so the measurement is not a
		// report on whoever ran it. PATH because the toolchain lookup
		// already used it; HOME inside the temp tree so nothing reaches
		// a real user's files; TMPDIR because macOS puts a per-user
		// directory there and the SQLite cache writes temporaries.
		env: []string{
			"PATH=" + os.Getenv("PATH"),
			"HOME=" + root,
		},
	}
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		h.env = append(h.env, "TMPDIR="+tmp)
	}

	if size <= 0 {
		return h, nil
	}

	entries, err := fixture.Generate(fixture.Seed, size)
	if err != nil {
		return nil, fmt.Errorf("generating a %d-entry fixture: %w", size, err)
	}
	store, err := local.Open(diraDir)
	if err != nil {
		return nil, err
	}
	if err := fixture.Write(context.Background(), store, entries); err != nil {
		return nil, err
	}
	for _, e := range entries {
		h.ids = append(h.ids, e.ID)
	}
	return h, nil
}

// Run executes the binary with args from the ledger's directory and returns its
// stdout.
//
// A non-zero exit is an error, not a fast sample. `dira brief` fails open by
// design — the hook discards stderr and swallows the exit code — so a harness
// that ignored the exit code would happily time a usage error at three
// milliseconds and call it a pass.
func (h *Harness) Run(args ...string) (string, error) {
	cmd := exec.Command(h.Binary, args...)
	cmd.Dir = h.Root
	cmd.Env = h.env

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("dira %s exited non-zero: %w\nstderr:\n%s",
			strings.Join(args, " "), err, excerpt(stderr.String()))
	}
	return stdout.String(), nil
}

// CheckBrief fails a sample whose stdout is not a brief rendered over THIS
// ledger.
//
// Two things are checked and both are needed. The section header proves the
// output is a brief rather than a usage block or an empty string. The entry id
// proves the brief was rendered from the fixture rather than from an empty
// ledger — `dira brief` over a ledger with no entries prints every section
// header with "none" under it, in a couple of milliseconds, and looks exactly
// like a very fast pass.
func (h *Harness) CheckBrief(stdout string) error {
	if !strings.Contains(stdout, briefHeader) {
		return fmt.Errorf("the measured run printed no %q section header, so what was timed is not a brief:\n%s",
			briefHeader, excerpt(stdout))
	}
	if len(h.ids) == 0 {
		return errors.New("the ledger under measurement has no entries, so a brief over it renders nothing; " +
			"this harness measures a fast number for doing no work")
	}
	for _, id := range h.ids {
		if strings.Contains(stdout, id) {
			return nil
		}
	}
	return fmt.Errorf("the measured run named none of the %d fixture entry ids, so the brief rendered nothing "+
		"from the ledger under measurement:\n%s", len(h.ids), excerpt(stdout))
}

// CheckCold fails a run labelled cold whose cache survived the setup.
//
// This is the third way a timing harness reports a fast number for nothing: a
// removal that silently did not happen turns every "cold" sample into a warm one,
// and the budget passes with room to spare while measuring the opposite of what
// it claims.
func (h *Harness) CheckCold() error {
	_, err := os.Stat(h.CacheDir)
	switch {
	case err == nil:
		return fmt.Errorf("%s exists at the start of a run labelled cold; the sample would be a warm one", h.CacheDir)
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("checking %s: %w", h.CacheDir, err)
	}
}

// RemoveCache is the setup half of a cold sample: outside the timer, because
// timing the unlink of three files would inflate the number this package exists
// to report.
func (h *Harness) RemoveCache() error {
	if err := os.RemoveAll(h.CacheDir); err != nil {
		return fmt.Errorf("removing %s: %w", h.CacheDir, err)
	}
	return nil
}

// CacheWasBuilt reports whether the cache directory holds anything, which after a
// cold run it must: a cold run that left no cache behind did not do the cold work
// the budget is asserted over.
func (h *Harness) CacheWasBuilt() error {
	names, err := os.ReadDir(h.CacheDir)
	if err != nil {
		return fmt.Errorf("the cold run left no cache directory behind: %w", err)
	}
	if len(names) == 0 {
		return fmt.Errorf("%s is empty after a cold run; nothing was indexed", h.CacheDir)
	}
	return nil
}

// excerpt bounds a captured stream so a failure message stays readable.
func excerpt(s string) string {
	const limit = 400
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return "\t(nothing on stdout)"
	}
	if len(s) > limit {
		s = s[:limit] + "\n\t..."
	}
	return "\t" + strings.ReplaceAll(s, "\n", "\n\t")
}

// HookBriefArgv returns the argv of the SessionStart hook's dira command, read
// out of hooks/settings.example.json.
//
// The measured command is read from the hook configuration rather than
// transcribed into a test, so the two cannot drift: if somebody changes what the
// SessionStart hook installs, this package measures the new thing, and the
// caller's assertion that the argv is still `brief --context --chain` is what
// makes that change visible rather than silent. A budget held over a command
// nobody runs is a budget on nothing.
//
// The configured command is a shell line — `dira brief --context --chain
// 2>/dev/null || true` — so parsing stops at the first shell metacharacter. The
// redirection and the `|| true` are how the hook fails open; neither is part of
// what dira is asked to do.
func HookBriefArgv(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the hook configuration: %w", err)
	}

	// Only the fields this needs. The file also carries "//" comment keys,
	// which are arrays of strings and are ignored here by omission.
	var doc struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	for _, matcher := range doc.Hooks["SessionStart"] {
		for _, hook := range matcher.Hooks {
			argv, ok := diraArgv(hook.Command)
			if ok {
				return argv, nil
			}
		}
	}
	return nil, fmt.Errorf("%s registers no SessionStart hook that runs dira", path)
}

// diraArgv splits a configured shell command into the argv dira receives,
// stopping at the first shell metacharacter.
func diraArgv(command string) ([]string, bool) {
	var argv []string
	for _, field := range strings.Fields(command) {
		if strings.ContainsAny(field, "<>|&;()") {
			break
		}
		argv = append(argv, field)
	}
	if len(argv) < 2 || argv[0] != "dira" {
		return nil, false
	}
	return argv[1:], true
}

// A Timing is the distribution of one measurement, not a single number.
//
// All five fields are reported because a red run has to be diagnosable without a
// re-run: a median over budget with a minimum far below it is a busy machine, and
// a median over budget with a minimum beside it is a slow program. Which of the
// five may be ASSERTED against is a separate question, answered in doc.go and in
// CheckBudgets.
type Timing struct {
	N                     int
	Min, Median, P90, Max time.Duration

	// Samples is the sorted distribution, kept so a caller can publish it.
	Samples []time.Duration
}

// Measure runs setup then run n times, timing only run.
//
// The split is deliberate and is the same one internal/index/latency_test.go
// makes: removing a cache is the SETUP for a cold sample, not the work, and
// timing it would inflate every number this package reports. One untimed
// setup+run precedes the samples so the first run's page faults and lazily
// initialised state do not land in the distribution.
func Measure(n int, setup, run func() error) (Timing, error) {
	if n < samples {
		return Timing{}, fmt.Errorf("asked for %d samples; the lane's acceptance line requires N>=%d", n, samples)
	}

	if err := setup(); err != nil {
		return Timing{}, fmt.Errorf("warm-up setup: %w", err)
	}
	if err := run(); err != nil {
		return Timing{}, fmt.Errorf("warm-up run: %w", err)
	}

	took := make([]time.Duration, 0, n)
	for i := range n {
		if err := setup(); err != nil {
			return Timing{}, fmt.Errorf("setup before sample %d of %d: %w", i+1, n, err)
		}
		start := time.Now()
		err := run()
		elapsed := time.Since(start)
		if err != nil {
			return Timing{}, fmt.Errorf("sample %d of %d: %w", i+1, n, err)
		}
		took = append(took, elapsed)
	}
	slices.Sort(took)

	return Timing{
		N:       n,
		Min:     took[0],
		Median:  took[n/2],
		P90:     took[nearestRank(n, 90)],
		Max:     took[n-1],
		Samples: took,
	}, nil
}

// nearestRank is the index of the pth percentile of n sorted samples, by the
// nearest-rank definition: ceil(p/100 * n), one-based, then zero-based here.
// With n=20 and p=90 that is the 18th of 20, which is a sample that was actually
// observed rather than an interpolation between two that were.
func nearestRank(n, p int) int {
	rank := (p*n + 99) / 100
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return rank - 1
}

// Report is the line a regression is diagnosed from. It names which statistic is
// the uncontended estimate and which one carries the assertion, because a reader
// finding a number in a CI log a year from now will not have doc.go open.
func (t Timing) Report(label string) string {
	return fmt.Sprintf("%-28s n=%d  min %v (uncontended estimate)  median %v (asserted)  p90 %v  max %v (asserted)",
		label+":", t.N,
		t.Min.Round(logUnit), t.Median.Round(logUnit), t.P90.Round(logUnit), t.Max.Round(logUnit))
}

// CheckBudgets fails a distribution that broke either ceiling, and names which
// one and by how much.
//
// The ceilings arrive as arguments rather than being read from budget.go
// directly, and that is not indirection for its own sake. L-0001 says a gate is
// evidence only once both sides have been observed, and the red side of a budget
// assertion is otherwise only reachable by editing the constant it asserts —
// which is a thing done once in a shell and never again. As parameters, both
// failure messages are provable from a test in the tree, on every run, without
// touching a number this lane is forbidden to touch.
func CheckBudgets(t Timing, label string, median, max time.Duration) error {
	var errs []error
	if t.Median > median {
		errs = append(errs, fmt.Errorf(
			"the %s MEDIAN budget is broken: median of %d runs is %v, over the %v ceiling by %v",
			label, t.N, t.Median.Round(logUnit), median.Round(logUnit), (t.Median-median).Round(logUnit)))
	}
	if t.Max > max {
		errs = append(errs, fmt.Errorf(
			"the %s SINGLE-RUN MAXIMUM budget is broken: slowest of %d runs is %v, over the %v ceiling by %v",
			label, t.N, t.Max.Round(logUnit), max.Round(logUnit), (t.Max-max).Round(logUnit)))
	}
	return errors.Join(errs...)
}
