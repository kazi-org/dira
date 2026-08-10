package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/distill"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// `dira distill` at the command boundary: the exit codes, the one-line answers,
// and that the surface is wired to the renderer rather than to the package's
// fallback.
//
// What is NOT here is the keystroke loop, the transitions and the card's own
// copy. Those are internal/distill's, and they are tested there against fakes
// that can script a terminal — a second copy of them driven through a real one
// would be slower, flakier and no more true.

// distillRunner is one `dira distill` invocation with everything captured.
type distillRunner struct {
	stdout, stderr bytes.Buffer
	app            *app
	stdin          *countingReader
	dir            string
}

// newDistillRunner builds an app that can reach `dira distill`, over an empty
// ledger in a temp directory.
//
// The registry line is added here when main.go does not already carry it,
// because main.go is the integrator's file and a lane may not edit it (see
// TestEveryRunFunctionIsRegistered, and newSniffRunner, which does the same
// thing for the same reason). Copying the line into a test rather than
// describing it in a report means the line this lane reports is a line that has
// been executed. The conditional matters: once the integrator adds it, this
// must not shadow the real registration with a second copy.
func newDistillRunner(t *testing.T) *distillRunner {
	t.Helper()

	r := &distillRunner{dir: t.TempDir(), stdin: &countingReader{}}
	if err := os.MkdirAll(filepath.Join(r.dir, ".dira", "entries"), 0o755); err != nil {
		t.Fatalf("creating the temp ledger: %v", err)
	}

	r.app = newApp(&r.stdout, &r.stderr)
	r.app.stdin = r.stdin
	r.app.now = func() time.Time { return time.Date(2026, 8, 1, 9, 15, 0, 0, time.UTC) }
	if r.app.lookup("distill") == nil {
		r.app.commands = append(r.app.commands, &command{
			name: "distill", summary: distillSummary, run: runDistill, usage: writeDistillUsage,
		})
	}
	return r
}

func (r *distillRunner) run(args ...string) int {
	return r.app.main(append([]string{"distill", "-C", r.dir}, args...))
}

// stage writes entries into the runner's ledger through the real backend, so the
// files under test are the files dira itself would produce.
func (r *distillRunner) stage(t *testing.T, entries ...*ledger.Entry) {
	t.Helper()

	store, err := local.Open(filepath.Join(r.dir, ".dira"))
	if err != nil {
		t.Fatalf("opening the temp ledger: %v", err)
	}
	for _, entry := range entries {
		if err := store.Create(context.Background(), entry); err != nil {
			t.Fatalf("staging %s: %v", entry.ID, err)
		}
	}
}

// digest is a sha256 over every entry file, so "nothing was written" is a
// comparison rather than an impression.
func (r *distillRunner) digest(t *testing.T) string {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(r.dir, ".dira", "entries", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)

	sum := sha256.New()
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		sum.Write([]byte(filepath.Base(file)))
		sum.Write(data)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// countingReader is a stdin that records whether anything ever asked it for a
// byte. `dira distill` must not read one when there is no human, and "did not
// read" is only checkable by asking the reader.
type countingReader struct{ reads int }

func (c *countingReader) Read([]byte) (int, error) {
	c.reads++
	return 0, io.EOF
}

// staged is a capture in the exact shape `dira sniff` writes: a title, a state,
// a created stamp and a source, with no body, no alternatives and no
// confirmed_by (docs/decisions-pending/E2-L1-report.md §4).
func staged(id, title string) *ledger.Entry {
	return &ledger.Entry{
		ID:      id,
		Kind:    ledger.KindDecision,
		Title:   title,
		State:   ledger.StateStaged,
		Created: "2026-07-31T09:00:00Z",
		Source: &ledger.Source{
			Hook:    ledger.HookStop,
			Session: "1f0c6a3e-0000-4000-8000-000000000009",
			Excerpt: title + " rather than the alternative",
			Tier:    ledger.TierRegex,
		},
	}
}

// TestDistillOnAnEmptyQueueCostsOneLine is the acceptance clause: a ledger with
// nothing staged exits 0 printing exactly one line.
//
// The second half is what makes it a measurement. A command that printed one
// fixed line whatever the ledger held would pass the first half forever, so the
// same runner is driven over a ledger that DOES hold captures and the line has
// to change — while still being one line, and while still writing nothing.
func TestDistillOnAnEmptyQueueCostsOneLine(t *testing.T) {
	t.Parallel()

	r := newDistillRunner(t)
	if code := r.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, r.stderr.String())
	}
	empty := onlyLine(t, r.stdout.String())
	if r.stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", r.stderr.String())
	}
	if r.stdin.reads != 0 {
		t.Errorf("stdin was read %d times over an empty queue; an empty queue costs no input", r.stdin.reads)
	}

	full := newDistillRunner(t)
	full.stage(t, staged("dec-0001", "we are moving the queue reader behind the storage interface"))
	before := full.digest(t)

	if code := full.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, full.stderr.String())
	}
	waiting := onlyLine(t, full.stdout.String())
	if waiting == empty {
		t.Errorf("a ledger with a capture waiting prints the same line as an empty one (%q); the line "+
			"is a constant and the assertion above measures nothing", waiting)
	}
	if after := full.digest(t); after != before {
		t.Errorf("the ledger changed with no human present:\n\tbefore %s\n\tafter  %s", before, after)
	}
	if full.stdin.reads != 0 {
		t.Errorf("stdin was read %d times with no terminal; nothing may be read from a pipe that "+
			"could be somebody else's hook payload", full.stdin.reads)
	}
}

// TestDistillRejectsAMistypedFlagWithTwo is the exit-code clause: the binary's
// general contract, not `check`/`supersede`'s 1/2 split (docs/lore.md L-0020).
func TestDistillRejectsAMistypedFlagWithTwo(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "a flag that does not exist", args: []string{"--nosuchflag"}},
		{name: "a flag with no value", args: []string{"--width"}},
		{name: "an argument it takes none of", args: []string{"dec-0001"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newDistillRunner(t)
			code := r.run(tc.args...)

			if code != exitUsage {
				t.Errorf("exit code = %d, want %d", code, exitUsage)
			}
			if r.stdout.String() != "" {
				t.Errorf("stdout = %q, want empty: a caller parsing stdout must never be handed help text",
					r.stdout.String())
			}
			if err := r.stderr.String(); !strings.Contains(err, "usage:") {
				t.Errorf("stderr carries no usage block:\n%s", err)
			}
			if err := r.stderr.String(); !strings.Contains(err, "dira distill") {
				t.Errorf("stderr shows the top-level usage rather than distill's own:\n%s", err)
			}
		})
	}

	// The other half. Without it, a command that returned a usage error for
	// every invocation would pass every case above.
	r := newDistillRunner(t)
	if code := r.run("--width", "72"); code != exitOK {
		t.Errorf("a well-formed invocation exits %d, want %d (stderr: %s)", code, exitOK, r.stderr.String())
	}
}

// TestDistillHelpIsTheSameTextBothWays is main.go's rule that `dira help <name>`
// and `dira <name> -h` show one text: a flag documented in one and not the other
// is a flag nobody finds.
func TestDistillHelpIsTheSameTextBothWays(t *testing.T) {
	t.Parallel()

	viaFlag := newDistillRunner(t)
	if code := viaFlag.app.main([]string{"distill", "-h"}); code != exitOK {
		t.Fatalf("`dira distill -h` exited %d", code)
	}
	viaHelp := newDistillRunner(t)
	if code := viaHelp.app.main([]string{"help", "distill"}); code != exitOK {
		t.Fatalf("`dira help distill` exited %d", code)
	}

	if viaFlag.stdout.String() != viaHelp.stdout.String() {
		t.Errorf("`dira distill -h` and `dira help distill` disagree:\n--- -h ---\n%s\n--- help ---\n%s",
			viaFlag.stdout.String(), viaHelp.stdout.String())
	}
	if viaFlag.stdout.String() == "" {
		t.Error("both printed nothing, so the comparison above is vacuous")
	}

	// The usage names every flag the command accepts. A flag that exists and
	// is undocumented is the failure this checks for, and it is checked from
	// the flag set rather than from a list somebody has to keep in step.
	help := viaFlag.stdout.String()
	(&distillFlags{}).flagSet().VisitAll(func(f *flag.Flag) {
		if !strings.Contains(help, "-"+f.Name) {
			t.Errorf("`dira distill -h` does not mention the %q flag:\n%s", f.Name, help)
		}
	})
}

// TestDistillRendersThroughTheCard is the wiring: the command lays cards out
// with internal/distill's renderer at the width it resolved, not with the
// package's fallback and not at a constant.
//
// It is asserted against the session options rather than against a screen
// because a screen needs a terminal, and a test that needed one could not run in
// CI at all — which is how a surface ends up shipped with a renderer nobody ever
// saw it use.
func TestDistillRendersThroughTheCard(t *testing.T) {
	t.Parallel()

	r := newDistillRunner(t)
	item := distill.Item{Entry: staged("dec-0001", "the queue reader takes a store, not a path")}

	opts := r.app.distillSession(nil, offlineTerminal{}, 72)
	if opts.Render == nil {
		t.Fatal("the session has no renderer, so the loop would fall back to the package's plain card")
	}
	if got, want := opts.Render(item, 1, 3), distill.Card(72)(item, 1, 3); got != want {
		t.Errorf("the session's renderer is not distill.Card(72)\n--- session ---\n%s\n--- Card(72) ---\n%s",
			got, want)
	}

	// The other half: the width is carried rather than ignored. Two widths
	// that produced the same card would mean --width reaches nothing.
	narrow := r.app.distillSession(nil, offlineTerminal{}, 40)
	if narrow.Render(item, 1, 3) == opts.Render(item, 1, 3) {
		t.Error("the card is identical at 40 and 72 columns; the resolved width reaches no renderer")
	}
	if opts.Edit == nil {
		t.Error("the session has no editor, so `e` would refuse on every card")
	}
	if opts.Now == nil {
		t.Error("the session has no clock, so nothing could stamp `updated`")
	}
}

// TestDistillTerminalReportsNoHumanWithoutOne covers the anti-hang guarantee at
// the only place that can answer it: the concrete terminal.
//
// distill.Run calls Raw only after Interactive says yes, so a terminal that
// answers honestly is the whole of the guarantee. Both halves are here — a
// reader with no descriptor and a pipe that has one are both "not a human", and
// the second is the case a mode check would have got wrong.
func TestDistillTerminalReportsNoHumanWithoutOne(t *testing.T) {
	t.Parallel()

	if term := newTerminal(strings.NewReader("yyy")); term.Interactive() {
		t.Error("a reader with no file descriptor was reported interactive")
	}

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("opening a pipe: %v", err)
	}
	defer func() { _ = read.Close(); _ = write.Close() }()

	term := newTerminal(read)
	if term.Interactive() {
		t.Error("a pipe was reported interactive; nothing may read a keystroke from it")
	}
	if _, _, err := term.Raw(); err == nil {
		t.Error("raw mode was entered on a pipe")
	}
	if width := term.Width(); width != 0 {
		t.Errorf("a pipe reported a width of %d, want 0 so the card falls back to the default", width)
	}

	// Suspend and Resume outside raw mode are no-ops rather than errors: the
	// editor path calls them whether or not a terminal was ever raw.
	if err := term.Suspend(); err != nil {
		t.Errorf("Suspend outside raw mode: %v", err)
	}
	if err := term.Resume(); err != nil {
		t.Errorf("Resume outside raw mode: %v", err)
	}
}

// TestDistillClosingLineCountsWhatHappened checks the line printed after the
// loop, in both directions: it reports what the session did, and it says
// something different when the session did something different.
func TestDistillClosingLineCountsWhatHappened(t *testing.T) {
	t.Parallel()

	one := distilled(&distill.Result{
		Dispositions: []distill.Disposition{{ID: "dec-0001", Act: distill.ActConfirm}},
	})
	if !strings.Contains(one, "1 capture disposed of") {
		t.Errorf("the closing line for one disposition is %q", one)
	}
	if strings.Contains(one, "left staged") {
		t.Errorf("the closing line mentions entries left staged when none were: %q", one)
	}

	several := distilled(&distill.Result{
		Dispositions: []distill.Disposition{{ID: "dec-0001"}, {ID: "dec-0002"}},
		Undone:       1,
		Remaining:    3,
	})
	for _, want := range []string{"2 captures disposed of", "1 disposition undone", "3 left staged"} {
		if !strings.Contains(several, want) {
			t.Errorf("the closing line %q does not carry %q", several, want)
		}
	}
	if strings.Contains(several, "1 captures") || strings.Contains(one, "1 captures") {
		t.Errorf("the closing line spells a plural over a count of one: %q / %q", one, several)
	}
}

// onlyLine returns the single line in out, failing when there is not exactly
// one.
func onlyLine(t *testing.T, out string) string {
	t.Helper()

	if out == "" {
		t.Fatal("nothing was printed; the answer is one line, not none")
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("the output does not end in a newline: %q", out)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("%d lines printed, want exactly 1:\n%s", len(lines), out)
	}
	return lines[0]
}
