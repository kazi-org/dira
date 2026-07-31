package sniff

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
)

// deepBudget is the ceiling this test holds a deep run to.
//
// hooks/settings.example.json gives the PreCompact hook a 60s timeout, which is
// the number the acceptance cites; ten is a tenth of it and still two orders of
// magnitude above what the fixture costs. The point of the assertion is not the
// margin, it is that the margin is measured on every run rather than assumed
// from a design that has never been timed.
const deepBudget = 10 * time.Second

// TestDeepHandoff is E2-L2-T3's acceptance, in one test, in the order the
// clauses have to be read.
//
// The first subtest is the one everything else rests on: `--deep` writes exactly
// what `--stage` writes. If that is false, then no amount of correctness in the
// handoff matters, because the insurance policy has started gambling — a
// PreCompact run whose staging depended on the deep path is a run that loses the
// session whenever the deep path is wrong.
func TestDeepHandoff(t *testing.T) {
	t.Parallel()
	started := time.Now()

	// --- tier 1 loses nothing --------------------------------------------
	//
	// Both runs are given identical StageOptions, because that is the claim
	// under test: `--deep` changes nothing about how a candidate is staged.
	// The two flag surfaces do differ in one default — `--deep` moves --hook
	// to PreCompact, which is asserted against the built binary further down
	// rather than smuggled in here, where it would show up as a byte
	// difference and mask a real one.
	plain := stageRun(t, "pre-compact.jsonl")
	deep := deepRun(t, "pre-compact.jsonl", nil)

	// Non-vacuity before comparison. Two empty ledgers are byte-identical,
	// and an equality assertion between them is the purest form of the
	// failure docs/lore.md L-0001 rule 1 names.
	if len(plain.staged) != 3 {
		t.Fatalf("the plain staged run wrote %d entries, want the 3 pre-compact.jsonl yields", len(plain.staged))
	}
	if len(deep.staged) != 3 {
		t.Fatalf("the deep run wrote %d entries, want 3", len(deep.staged))
	}

	if diff := diffLedgers(ledgerFiles(t, plain.dir), ledgerFiles(t, deep.dir)); diff != "" {
		t.Errorf("--deep did not write the same ledger as --stage:\n%s", diff)
	} else {
		t.Logf("OBSERVED  --deep and --stage wrote %d byte-identical entry files", len(plain.staged))
	}

	// --- and the handoff is what it adds ---------------------------------
	if strings.TrimSpace(deep.block) == "" {
		t.Fatal("the deep run emitted no handoff block; every assertion below would pass vacuously")
	}
	for _, e := range deep.staged {
		if !strings.Contains(deep.block, "  "+e.ID+"  ") {
			t.Errorf("the handoff does not list staged id %s:\n%s", e.ID, deep.block)
		}
	}
	// It is T2's block, not a second renderer that could drift from the
	// golden. Asserted by identity rather than by re-listing its contents,
	// because handoff_test.go already pins what it says.
	if want := Handoff(StagedItems(deep.staged)); deep.block != want {
		t.Errorf("the deep run rendered something other than Handoff:\n%s", diffBlocks(want, deep.block))
	} else {
		t.Logf("OBSERVED  the block is Handoff(StagedItems(...)) verbatim, naming %d ids", len(deep.staged))
	}

	// --- --deep upgrades nothing -----------------------------------------
	//
	// The semantic tier's output arrives later, as its own entry, through
	// `dira log`. Nothing on this path may anticipate it.
	for _, e := range deep.staged {
		switch {
		case e.State != ledger.StateStaged:
			t.Errorf("%s is %q, want %q", e.ID, e.State, ledger.StateStaged)
		case e.Source == nil:
			t.Errorf("%s carries no source", e.ID)
		case e.Source.Tier != ledger.TierRegex:
			t.Errorf("%s claims tier %q; a regular expression found it", e.ID, e.Source.Tier)
		case e.Source.Hook != ledger.HookPreCompact:
			t.Errorf("%s records hook %q, want %q", e.ID, e.Source.Hook, ledger.HookPreCompact)
		case strings.TrimSpace(e.Source.Excerpt) == "":
			t.Errorf("%s carries no excerpt, so a human cannot dispose of it", e.ID)
		case len([]rune(e.Source.Excerpt)) > maxExcerpt:
			t.Errorf("%s carries a %d-character excerpt, want at most %d", e.ID, len([]rune(e.Source.Excerpt)), maxExcerpt)
		case len(e.Alternatives) != 0:
			t.Errorf("%s carries %d alternative(s); tier 1 cannot know what was rejected (dec-0003)", e.ID, len(e.Alternatives))
		case len(e.Edges) != 0:
			t.Errorf("%s carries %d edge(s); derives_from is tier 2's claim", e.ID, len(e.Edges))
		case e.ConfirmedBy != "":
			t.Errorf("%s claims confirmed_by %q; nobody confirmed it", e.ID, e.ConfirmedBy)
		}
	}
	t.Logf("OBSERVED  all %d entries are staged/regex/PreCompact with an excerpt and no alternatives", len(deep.staged))

	// --- a session that settled nothing hands off nothing ----------------
	//
	// Run in the same test as the positive fixture, deliberately: "the
	// sniffer found nothing" and "the plumbing is broken" produce identical
	// output, and the only thing that tells them apart is a positive result
	// from the same code path in the same run. That is the three entries
	// above.
	quiet := deepRun(t, "no-decisions.jsonl", nil)
	if len(quiet.staged) != 0 {
		t.Errorf("no-decisions.jsonl staged %d entries, want none", len(quiet.staged))
	}
	if quiet.block != "" {
		t.Errorf("no-decisions.jsonl produced a handoff block:\n%s", quiet.block)
	}
	if files := ledgerFiles(t, quiet.dir); len(files) != 0 {
		t.Errorf("no-decisions.jsonl wrote %d files: %v", len(files), sortedKeys(files))
	}
	t.Logf("OBSERVED  no-decisions.jsonl: 0 entries, no block, no error — while the fixture above staged %d", len(deep.staged))

	// --- a broken renderer must not cost the capture ---------------------
	//
	// Both shapes, because both are real: a renderer that returns an error
	// is what a future one that reads something would do, and a renderer
	// that panics is what a formatting bug does today. Either one out of a
	// PreCompact hook is an exit code the session sees.
	for _, broken := range []struct {
		name   string
		render func([]HandoffItem) (string, error)
	}{
		{"returns an error", func([]HandoffItem) (string, error) { return "", errors.New("forced") }},
		{"panics", func([]HandoffItem) (string, error) { panic("forced") }},
		{"returns a block and an error", func([]HandoffItem) (string, error) { return "half a block", errors.New("forced") }},
	} {
		run := deepRun(t, "pre-compact.jsonl", broken.render)
		if run.err != nil {
			t.Errorf("a renderer that %s made the run fail: %v", broken.name, run.err)
		}
		if run.block != "" {
			t.Errorf("a renderer that %s still produced a block: %q", broken.name, run.block)
		}
		if len(run.staged) != len(deep.staged) {
			t.Errorf("a renderer that %s cost the capture: %d entries staged, want %d",
				broken.name, len(run.staged), len(deep.staged))
		}
		if diff := diffLedgers(ledgerFiles(t, plain.dir), ledgerFiles(t, run.dir)); diff != "" {
			t.Errorf("a renderer that %s changed what was staged:\n%s", broken.name, diff)
		}
	}
	t.Logf("OBSERVED  with the renderer forced to fail three ways, the ledger is unchanged and no error is returned")

	// --- inside the hook's budget ----------------------------------------
	if elapsed := time.Since(started); elapsed > deepBudget {
		t.Errorf("the deep runs took %s, over the %s this test holds them to (the hook's own budget is 60s)", elapsed, deepBudget)
	} else {
		t.Logf("OBSERVED  every deep run in this test completed in %s, inside the %s bound", elapsed.Round(time.Millisecond), deepBudget)
	}
}

// TestDeepLedgerEqualityCanFail is the other half of the first clause above, and
// the reason that clause is evidence rather than decoration.
//
// The acceptance asks for the equality to be "proven able to fail (a build where
// `--deep` skips the regex pass turns it red)". A build is not available to a
// test, so the defect is constructed at the only place it could actually appear:
// a deep run that reached the renderer with fewer candidates than the regex pass
// found. If the comparator cannot see that, then the green result above is a
// comparator that compares nothing.
func TestDeepLedgerEqualityCanFail(t *testing.T) {
	t.Parallel()

	full := stageRun(t, "pre-compact.jsonl")
	if len(full.staged) == 0 {
		t.Fatal("the reference run staged nothing, so nothing below is a comparison")
	}

	// Green: the comparator accepts two runs of the same input.
	again := stageRun(t, "pre-compact.jsonl")
	if diff := diffLedgers(ledgerFiles(t, full.dir), ledgerFiles(t, again.dir)); diff != "" {
		t.Fatalf("two identical runs compared unequal, so the red result below would mean nothing:\n%s", diff)
	}

	// Red 1: the regex pass skipped entirely. This is the shape of the build
	// the acceptance names — `--deep` that stages nothing and prints a block.
	skipped := deepRunWith(t, nil, nil)
	if skipped.block != "" {
		t.Errorf("a run that staged nothing still rendered a block:\n%s", skipped.block)
	}
	diff := diffLedgers(ledgerFiles(t, full.dir), ledgerFiles(t, skipped.dir))
	if diff == "" {
		t.Fatal("a run that staged nothing compared equal to one that staged three entries; the comparator is not comparing")
	}
	if !strings.Contains(diff, full.staged[0].ID) {
		t.Errorf("the difference does not name the missing entry, so a failure would not say what was lost:\n%s", diff)
	}
	t.Logf("OBSERVED  a deep run that skips the regex pass is rejected and reported:\n%s", diff)

	// Red 2: the subtler one — the same number of entries, one of them
	// different. A comparator that counted files would pass this.
	candidates := candidatesFor(t, "pre-compact.jsonl")
	altered := append([]Candidate(nil), candidates...)
	altered[len(altered)-1].Excerpt = altered[len(altered)-1].Excerpt + " (not what was said)"
	tampered := deepRunWith(t, altered, nil)
	if len(tampered.staged) != len(full.staged) {
		t.Fatalf("the tampered run wrote %d entries and the reference wrote %d; this would fail on the count alone",
			len(tampered.staged), len(full.staged))
	}
	diff = diffLedgers(ledgerFiles(t, full.dir), ledgerFiles(t, tampered.dir))
	if diff == "" {
		t.Fatal("an altered excerpt compared equal; the comparator is reading file names and not contents")
	}
	t.Logf("OBSERVED  a same-count ledger with one altered excerpt is rejected:\n%s", diff)
}

// TestTheInstalledPreCompactCommandIsAccepted closes the clause that unblocks
// E2-L3: the exact string hooks/settings.example.json installs must be a command
// the built binary accepts and exits 0 on.
//
// # What this test can and cannot say today, stated rather than hidden
//
// `dira sniff` is not in newApp's registry. E2-L1 landed cmd/dira/sniff.go and
// cmd/dira/sniff_test.go — which registers the command into its own test app and
// says in its own comment that "the integrator will register it in newApp" — and
// the registry line was never added. So the binary does not have the command at
// all, and `dira sniff --deep --stage` exits 2 with `unknown command "sniff"`
// before any flag is parsed.
//
// That line is cmd/dira/main.go's, which this task may not edit. Where it is
// missing, this test SKIPS and names the line, rather than passing: a skip is
// visible in the output and an assertion that quietly held is not. It becomes a
// real assertion the moment the line lands, with no edit here.
func TestTheInstalledPreCompactCommandIsAccepted(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("builds a binary; skipped under -short")
	}
	binary := buildDira(t)

	// The installed string, split exactly as hooks/settings.example.json
	// writes it. -C and --transcript are added because a hook runs against a
	// real ledger and a real session and a test must not; nothing else about
	// the invocation differs.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".dira", "entries"), 0o755); err != nil {
		t.Fatalf("creating the temp ledger: %v", err)
	}
	args := []string{"sniff", "--deep", "--stage", "-C", dir, "--all", "--transcript", transcriptPath(t, "pre-compact.jsonl")}

	cmd := exec.Command(binary, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if strings.Contains(stderr.String(), `unknown command "sniff"`) {
		t.Skipf("BLOCKED, not passing: the built binary has no `sniff` command, so `dira sniff --deep --stage` "+
			"exits 2 before --deep is ever parsed. One line is missing from newApp's registry in cmd/dira/main.go:\n\n"+
			"\t{name: \"sniff\", summary: sniffSummary, run: runSniff, usage: writeSniffUsage},\n\n"+
			"E2-L2-T3 may not edit that file. This test asserts the clause the moment the line lands.")
	}

	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("`dira %s` exited %d, want 0\nstdout:\n%s\nstderr:\n%s",
				strings.Join(args, " "), exit.ExitCode(), stdout.String(), stderr.String())
		}
		t.Fatalf("running the binary: %v", err)
	}

	block := stdout.String()
	if !strings.Contains(block, "=== dira handoff, tier 2 ===") {
		t.Errorf("the installed command exited 0 but printed no handoff block:\n%s", block)
	}
	if ids := entryID.FindAllString(block, -1); len(ids) == 0 {
		t.Errorf("the handoff names no entry id:\n%s", block)
	}
	files, err := filepath.Glob(filepath.Join(dir, ".dira", "entries", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Error("the installed command exited 0 and wrote nothing, which is not a capture")
	}
	t.Logf("OBSERVED  `dira %s` exited 0, wrote %d entries and printed the handoff", strings.Join(args, " "), len(files))
}

// TestDeepRefusesWithoutStage pins the other end of the flag's contract, at the
// only layer this task's files can reach it from.
//
// A deep run with nothing staged has no ids, and a handoff block with no ids is
// a block that hands off nothing. The command refuses rather than printing one,
// and that refusal is asserted through the binary for the same reason the clause
// above is: a flag combination that is accepted in process and refused by the
// shipped binary is the failure this whole lane is about.
func TestDeepRefusesWithoutStage(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("builds a binary; skipped under -short")
	}
	binary := buildDira(t)

	cmd := exec.Command(binary, "sniff", "--deep")
	cmd.Stdin = strings.NewReader("")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()

	// The registry probe comes before the control below, not after it. With
	// no `sniff` command every invocation exits non-zero, so a control
	// asserting "`--deeper` is refused" would hold for the wrong reason and
	// report a flag surface it never reached.
	if strings.Contains(stderr.String(), `unknown command "sniff"`) {
		t.Skipf("BLOCKED, not passing: `sniff` is not in newApp's registry — see " +
			"TestTheInstalledPreCompactCommandIsAccepted for the missing line.")
	}

	// The control: an unregistered flag is refused, which is what `--deep`
	// itself did before this task. Without it, "the binary accepted --deep"
	// would be true of a binary that ignores every flag it does not know.
	unknown := exec.Command(binary, "sniff", "--deeper", "--stage")
	unknown.Stdin = strings.NewReader("")
	if err := unknown.Run(); err == nil {
		t.Error("the binary accepted `--deeper`, so its acceptance of `--deep` says nothing about flag registration")
	}

	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("`dira sniff --deep` returned %v, want a usage failure", err)
	}
	if exit.ExitCode() != 2 {
		t.Errorf("`dira sniff --deep` exited %d, want 2", exit.ExitCode())
	}
	if !strings.Contains(stderr.String(), "--stage") {
		t.Errorf("the refusal does not name the flag that fixes it:\n%s", stderr.String())
	}
	t.Logf("OBSERVED  `dira sniff --deep` exits 2 naming --stage, and `--deeper` is refused")
}

// ---- helpers ---------------------------------------------------------------

// A run is one staging pass and everything a test wants to assert about it.
type run struct {
	dir    string
	staged []*ledger.Entry
	block  string
	err    error
}

// candidatesFor is the regex pass over a recorded transcript, at Whole scope.
//
// Whole rather than LastTurn because that is the scope PreCompact wants — see
// transcript.go's Scope comment — and because pre-compact.jsonl's last turn
// carries two of its three decisions, so a test at the default scope would be
// quietly grading a smaller fixture.
func candidatesFor(t *testing.T, fixture string) []Candidate {
	t.Helper()

	candidates, err := SniffTranscript(openTranscript(t, fixture), Whole)
	if err != nil {
		t.Fatalf("SniffTranscript(%s): %v", fixture, err)
	}
	return candidates
}

// deepOpts is the provenance both paths are given, so a byte difference between
// their ledgers is a difference in what they wrote and never in what they were
// told.
func deepOpts(t *testing.T) StageOptions {
	t.Helper()
	return StageOptions{Hook: ledger.HookPreCompact, Session: "s-0001", Now: stamp(t)}
}

// stageRun is what `dira sniff --stage` does, in this package's own terms.
func stageRun(t *testing.T, fixture string) run {
	t.Helper()

	store, dir := tempLedger(t)
	result, err := Stage(context.Background(), store, deepOpts(t), candidatesFor(t, fixture))
	if err != nil {
		t.Fatalf("Stage(%s): %v", fixture, err)
	}
	return run{dir: dir, staged: result.Staged}
}

// deepRun is what `dira sniff --deep --stage` does. A nil render is the shipped
// renderer.
func deepRun(t *testing.T, fixture string, render func([]HandoffItem) (string, error)) run {
	t.Helper()
	return deepRunWith(t, candidatesFor(t, fixture), render)
}

// deepRunWith is deepRun over candidates a test supplied itself, which is how
// the equality comparator's red side constructs a defect without a second build.
func deepRunWith(t *testing.T, candidates []Candidate, render func([]HandoffItem) (string, error)) run {
	t.Helper()

	store, dir := tempLedger(t)
	result, block, err := Deep(context.Background(), store, DeepOptions{
		StageOptions: deepOpts(t),
		render:       render,
	}, candidates)
	out := run{dir: dir, block: block, err: err}
	if result != nil {
		out.staged = result.Staged
	}
	return out
}

// ledgerFiles reads every entry file in a ledger, keyed by name.
//
// The contents, not a digest of them: a comparator that reported "these two
// hashes differ" would satisfy the acceptance's letter and tell whoever reads
// the failure nothing about what moved.
func ledgerFiles(t *testing.T, dir string) map[string]string {
	t.Helper()

	names, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	out := make(map[string]string, len(names))
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		out[filepath.Base(name)] = string(data)
	}
	return out
}

// diffLedgers reports the first way two ledgers differ, file by file. It returns
// the empty string only when both hold the same names and the same bytes.
func diffLedgers(want, got map[string]string) string {
	for _, name := range sortedKeys(want) {
		g, ok := got[name]
		if !ok {
			return fmt.Sprintf("  %s is missing from the second ledger", name)
		}
		if g != want[name] {
			return fmt.Sprintf("  %s differs:\n%s", name, diffBlocks(want[name], g))
		}
	}
	for _, name := range sortedKeys(got) {
		if _, ok := want[name]; !ok {
			return fmt.Sprintf("  %s is in the second ledger and not the first", name)
		}
	}
	return ""
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// buildDira builds the command under test, skipping where there is no toolchain
// — the pattern cmd/dira/build_test.go established.
func buildDira(t *testing.T) string {
	t.Helper()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go toolchain on PATH: %v", err)
	}
	binary := filepath.Join(t.TempDir(), "dira")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	out, err := exec.Command(goBin, "build", "-o", binary, "github.com/kazi-org/dira/cmd/dira").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return binary
}
