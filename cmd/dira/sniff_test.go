package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sniffTestdata is internal/sniff's fixture directory, reached from cmd/dira.
// The transcripts live there because that is the package that owns the format,
// and a second copy of five JSONL files would be a second thing to keep in step.
const sniffTestdata = "../../internal/sniff/testdata/transcripts"

// sniffRunner is one `dira sniff` invocation with everything captured.
type sniffRunner struct {
	stdout, stderr bytes.Buffer
	app            *app
	dir            string
}

// newSniffRunner builds an app with the sniff command registered exactly as the
// integrator will register it in newApp, over an empty ledger in a temp
// directory.
//
// The registration line is here rather than in main.go because main.go is the
// integrator's file. Copying it into this test rather than describing it in
// prose means the line in E2-L1's report is a line that has been executed.
func newSniffRunner(t *testing.T) *sniffRunner {
	t.Helper()

	r := &sniffRunner{dir: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(r.dir, ".dira", "entries"), 0o755); err != nil {
		t.Fatalf("creating the temp ledger: %v", err)
	}

	r.app = newApp(&r.stdout, &r.stderr)
	r.app.stdin = strings.NewReader("")
	r.app.now = func() time.Time { return time.Date(2026, 7, 30, 9, 15, 0, 0, time.UTC) }
	r.app.commands = append(r.app.commands, &command{
		name: "sniff", summary: sniffSummary, run: runSniff, usage: writeSniffUsage,
	})
	return r
}

func (r *sniffRunner) run(args ...string) int {
	return r.app.main(append([]string{"sniff", "-C", r.dir}, args...))
}

func (r *sniffRunner) entries(t *testing.T) []string {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(r.dir, ".dira", "entries", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func transcript(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(sniffTestdata, name)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("transcript fixture: %v", err)
	}
	return path
}

// TestQuietFindsNothingAndSaysNothing is the acceptance line's last clause,
// verbatim: a fixture with no decision language writes no file and, under
// --quiet, prints nothing and exits 0.
//
// The Stop hook fires after every turn of every session, so this is the common
// case rather than an edge case. A tool that printed one line per turn would be
// uninstalled inside a day, and one that exited non-zero would make every hook
// invocation look like a failure.
func TestQuietFindsNothingAndSaysNothing(t *testing.T) {
	t.Parallel()

	r := newSniffRunner(t)
	code := r.run("--stage", "--quiet", "--all", "--transcript", transcript(t, "no-decisions.jsonl"))

	if code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
	if r.stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", r.stdout.String())
	}
	if r.stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", r.stderr.String())
	}
	if files := r.entries(t); len(files) != 0 {
		t.Errorf("%d files written: %v", len(files), files)
	}
}

// TestQuietStillAnnouncesWhatItStaged is the control for the test above. Without
// it, a `--quiet` that suppressed everything unconditionally would pass, and the
// Stop hook would stage entries nobody was ever told about.
func TestQuietStillAnnouncesWhatItStaged(t *testing.T) {
	t.Parallel()

	r := newSniffRunner(t)
	code := r.run("--stage", "--quiet", "--all", "--transcript", transcript(t, "pre-compact.jsonl"))

	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, r.stderr.String())
	}
	out := r.stdout.String()
	if !strings.Contains(out, "dira: staged dec-0001") {
		t.Errorf("stdout does not announce the staged entry:\n%s", out)
	}
	if !strings.Contains(out, "confirm or ignore") {
		t.Errorf("stdout does not say what to do about it:\n%s", out)
	}
	if files := r.entries(t); len(files) == 0 {
		t.Error("nothing was written")
	}
}

// TestTheDefaultWritesNothing pins the dry run. The first thing anybody does
// with a capture tool is point it at their own transcript to see whether they
// trust it; answering that by writing files into their ledger is how a tool gets
// one try.
func TestTheDefaultWritesNothing(t *testing.T) {
	t.Parallel()

	r := newSniffRunner(t)
	code := r.run("--all", "--transcript", transcript(t, "pre-compact.jsonl"))

	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, r.stderr.String())
	}
	if !strings.Contains(r.stdout.String(), "would stage") {
		t.Errorf("stdout does not show what it would stage:\n%s", r.stdout.String())
	}
	if !strings.Contains(r.stdout.String(), "nothing written") {
		t.Errorf("stdout does not say that nothing was written:\n%s", r.stdout.String())
	}
	if files := r.entries(t); len(files) != 0 {
		t.Errorf("the dry run wrote %d files: %v", len(files), files)
	}
}

// TestTheHookPayloadResolvesTheTranscript covers how this command is actually
// invoked: no arguments, and a JSON object on stdin.
func TestTheHookPayloadResolvesTheTranscript(t *testing.T) {
	t.Parallel()

	abs, err := filepath.Abs(transcript(t, "pre-compact.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{
		"session_id":      "0193a1de-7b2c-7000-8000-abcdefabcdef",
		"transcript_path": abs,
		"hook_event_name": "Stop",
		"cwd":             t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}

	r := newSniffRunner(t)
	r.app.stdin = bytes.NewReader(payload)
	if code := r.run("--stage", "--quiet", "--all"); code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, r.stderr.String())
	}

	files := r.entries(t)
	if len(files) == 0 {
		t.Fatal("the hook payload staged nothing; the transcript path was not resolved")
	}
	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "0193a1de-7b2c-7000-8000-abcdefabcdef") {
		t.Errorf("the session id from the payload is not on the entry:\n%s", content)
	}
}

// TestAMissingTranscriptInAPayloadIsQuietlyNothing keeps a hook from printing a
// failure into a session because a transcript was rotated away. It is not the
// fail-open contract itself — that is E2-L3's, and it lives in the settings file
// — but a command that errored here would make that contract do more work than
// it should.
func TestAMissingTranscriptInAPayloadIsQuietlyNothing(t *testing.T) {
	t.Parallel()

	payload := `{"session_id":"abc","transcript_path":"/nonexistent/nowhere.jsonl","hook_event_name":"Stop"}`

	r := newSniffRunner(t)
	r.app.stdin = strings.NewReader(payload)
	if code := r.run("--stage", "--quiet"); code != exitOK {
		t.Errorf("exit code = %d, want %d (stderr: %s)", code, exitOK, r.stderr.String())
	}
	if r.stdout.String() != "" || r.stderr.String() != "" {
		t.Errorf("stdout=%q stderr=%q, want both empty", r.stdout.String(), r.stderr.String())
	}
}

// TestProseOnStdinIsRead keeps `dira sniff < notes.md` working, which is the
// only way a human can see what the tier makes of their own text.
func TestProseOnStdinIsRead(t *testing.T) {
	t.Parallel()

	r := newSniffRunner(t)
	r.app.stdin = strings.NewReader("We're going with the derived cache rather than a status field on the entry.\n")
	if code := r.run(); code != exitOK {
		t.Fatalf("exit code = %d (stderr: %s)", code, r.stderr.String())
	}
	if !strings.Contains(r.stdout.String(), "would stage") {
		t.Errorf("stdout:\n%s", r.stdout.String())
	}
}

// TestAHookThatDoesNotCaptureIsAnError covers the provenance rule from the
// command's side: SessionStart injects a brief and captures nothing, so an entry
// claiming it would misattribute where it came from.
func TestAHookThatDoesNotCaptureIsAnError(t *testing.T) {
	t.Parallel()

	r := newSniffRunner(t)
	code := r.run("--stage", "--all", "--hook", "SessionStart", "--transcript", transcript(t, "pre-compact.jsonl"))

	if code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
	if !strings.Contains(r.stderr.String(), "capture point") {
		t.Errorf("stderr does not explain the refusal: %q", r.stderr.String())
	}
	if files := r.entries(t); len(files) != 0 {
		t.Errorf("%d files written despite the refusal", len(files))
	}
}

// TestSniffRejectsMisuse pins the exit-code contract on the two mistakes a
// caller can make. Both are exit 2, and E2-L3's installer test reads this
// surface to check that every command string it writes is one the binary
// accepts.
func TestSniffRejectsMisuse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown flag", args: []string{"--nosuchflag"}, want: "not defined"},
		{name: "an argument", args: []string{"transcript.jsonl"}, want: "takes no arguments"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newSniffRunner(t)
			if code := r.run(tc.args...); code != exitUsage {
				t.Errorf("exit code = %d, want %d", code, exitUsage)
			}
			if r.stdout.String() != "" {
				t.Errorf("stdout = %q, want empty on a usage error", r.stdout.String())
			}
			if !strings.Contains(r.stderr.String(), tc.want) {
				t.Errorf("stderr does not explain the mistake; want %q\n%s", tc.want, r.stderr.String())
			}
			if !strings.Contains(r.stderr.String(), "dira sniff") {
				t.Errorf("stderr carries no usage block:\n%s", r.stderr.String())
			}
		})
	}
}

// TestSniffDocumentsItsOwnFlags is the same check `dira why` and `dira log`
// carry: a flag the binary accepts and the help does not mention is a flag
// nobody finds, and a flag the help promises and the binary rejects produces a
// hook that silently never captures.
func TestSniffDocumentsItsOwnFlags(t *testing.T) {
	t.Parallel()

	var help bytes.Buffer
	writeSniffUsage(&help)

	f := &sniffFlags{}
	fs := f.flagSet()
	seen := 0
	fs.VisitAll(func(fl *flag.Flag) {
		seen++
		spelling := "--" + fl.Name
		if len(fl.Name) == 1 {
			spelling = "-" + fl.Name
		}
		if !strings.Contains(help.String(), spelling) {
			t.Errorf("`dira sniff -h` does not mention %s", spelling)
		}
	})
	if seen == 0 {
		t.Fatal("the flag set is empty; this check is not measuring anything")
	}

	// And the other direction: every flag the hook contract in
	// hooks/settings.example.json writes must parse.
	for _, args := range [][]string{{"--stage", "--quiet"}, {"--stage"}} {
		if err := f.flagSet().Parse(args); err != nil {
			t.Errorf("%v is refused by the binary but written by the installer: %v", args, err)
		}
	}
}
