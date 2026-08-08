package skill

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/kazi-org/dira/schema"
	"gopkg.in/yaml.v3"
)

// E2-L2-T6 replays a recorded tier-2 response end to end.
//
// # Why a recording
//
// Tier 2 is a Claude session, and there is no model client in this binary to
// stand one up (dec-0003, cst-0004). A test that mocked the session would assert
// that the mock agrees with the test, which is the shape of every gate
// docs/lore.md L-0001 lists. So the session's output is recorded once and
// replayed: testdata/skill/handoff-response.txt is what a session actually
// answered with, and the `dira log` call inside it is run against the built
// binary rather than through an in-process helper, because a helper would
// exercise a code path the skill does not use. What the skill instructs is an
// argv, and an argv is only real once a process has parsed it.
//
// # Provenance of the fixtures
//
// The handoff the response answers is the block `dira sniff --deep --stage
// --all` prints for internal/sniff/testdata/transcripts/pre-compact.jsonl —
// internal/sniff/testdata/handoff.golden, which names dec-0001, dec-0002 and
// dec-0003. The response was recorded on 2026-08-08 from a session (Claude Opus
// 5) holding that block, that transcript and skills/dira/SKILL.md, and it is
// stored verbatim: it is prose with one fenced invocation in it, exactly as it
// arrived, not a command line reverse-engineered from the entry we wanted.
//
// replayCapture below is likewise verbatim — it is the file tier 1 wrote for
// dec-0001 on that same run, so the derives_from edge in the replayed call
// points at a capture that exists and looks the way the regex tier leaves them.
//
// # What this asserts, and what it deliberately does not
//
// The entry that comes out is checked against schema/entry.schema.json, the
// published contract, and not only against internal/ledger's Go validator. The
// two are separate implementations of one rule and have disagreed before
// (`alternatives: []` on a staged decision: the schema permits it, deliberately
// — see the `if/then/else` at entry.schema.json's staged-decision branch), and a
// suite that only ever ran each against the other's happy path would not have
// noticed. `dira log` validates with the Go validator; this test validates the
// file it produced with the JSON Schema one.
//
// It does not assert that the extraction replaces the capture, because it must
// not. The two entries are one graph — internal/why renders INCOMING
// derives_from, so `dira why dec-0001` names the extraction and the extraction
// names the capture — and merging them would rewrite the capture's source.tier
// into a claim a regex never made. dec-0025's fifth rejected alternative refuses
// exactly that, and `dira supersede` already refuses to let a staged extraction
// retire its capture.
//
// Nor does it assert the entry left `staged`. dec-0022 keeps a promoted entry
// staged until a human disposes of it, and the schema lifts `alternatives`'
// `minItems: 1` for a staged decision precisely so the semantic tier can supply
// a why_not on an entry that has not left it. dec-0025: confirming writes
// `confirmed_by: human` and a bumped `updated`, nothing else — so an entry that
// arrived here already carrying `confirmed_by: human` would be tier 2 signing
// for the human, and that is asserted against.

const (
	replayResponsePath = "testdata/skill/handoff-response.txt"
	replayExpectedPath = "testdata/skill/expected-entry.md"
)

// replayCapture is the thin tier-1 entry the replayed call derives from, as
// `dira sniff --deep --stage --all` wrote it for pre-compact.jsonl. It is seeded
// directly rather than by running the sniffer: this test is about the tier-2
// half, and depending on tier 1's writer here would make a sniff regression
// surface as a confusing failure in the replay.
const replayCapture = `---
id: dec-0001
kind: decision
title: We settled on lexical matching in the binary rather than an agent adjudicating the exit code, because the non-zero…
state: staged
created: "2026-08-08T18:26:05Z"
source:
  hook: PreCompact
  excerpt: >
    We settled on lexical matching in the binary rather than an agent adjudicating
    the exit code, because the non-zero exit is the product and a verdict that
    needs a live session produces nothing at a terminal with the network
    unplugged.
  tier: regex
---
`

// TestReplay is the acceptance: the recorded response, replayed verbatim
// through the built binary, produces a valid semantic-tier entry — and a
// mutated one is refused.
//
// Both halves are subtests of one parent because they share the binary, and
// linking it is the expensive part of this file: on a loaded machine `go build
// ./cmd/dira` is tens of seconds, and building it twice to say two things about
// one artifact is a cost every future run pays.
func TestReplay(t *testing.T) {
	t.Parallel()

	binary := replayBuildDira(t)

	t.Run("the recorded response", func(t *testing.T) {
		t.Parallel()
		replayRecorded(t, binary)
	})
	t.Run("a mutated response with an empty why_not", func(t *testing.T) {
		t.Parallel()
		replayEmptyWhyNot(t, binary)
	})
}

// replayRecorded replays the recording as it was recorded.
func replayRecorded(t *testing.T, binary string) {
	call := replayTheCall(t, replayReadFixture(t, replayResponsePath))

	root := replayLedger(t)
	before := replayEntryFiles(t, root)

	result := replayRun(t, binary, root, call)
	if result.code != 0 {
		t.Fatalf("the recorded call exited %d; the recorded response is supposed to be the one that works\nstderr:\n%s",
			result.code, result.stderr)
	}

	id := strings.TrimSpace(result.stdout)
	if id == "" {
		t.Fatal("dira log printed no id on stdout, so there is nothing to look at")
	}
	t.Logf("OBSERVED  exit 0, allocated %s", id)

	// Exactly one file appeared, and it is the one whose id was printed. A
	// second file would mean the call did something besides write an entry.
	after := replayEntryFiles(t, root)
	added := replayAdded(before, after)
	if want := []string{id + ".md"}; !reflect.DeepEqual(added, want) {
		t.Fatalf("the call added %v under .dira/entries, want exactly %v", added, want)
	}

	produced, err := os.ReadFile(filepath.Join(root, ".dira", "entries", id+".md"))
	if err != nil {
		t.Fatalf("reading the entry the replay wrote: %v", err)
	}
	t.Logf("OBSERVED  %s:\n%s", id+".md", produced)

	// --- the published contract, not just the Go validator ----------------
	validator, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("compiling schema/entry.schema.json: %v", err)
	}
	if err := validator.Validate(produced); err != nil {
		t.Fatalf("the replayed entry violates the published schema: %v", err)
	}
	t.Log("OBSERVED  validates against schema/entry.schema.json")

	golden := replayReadFixture(t, replayExpectedPath)
	if err := validator.Validate([]byte(golden)); err != nil {
		t.Fatalf("%s violates the published schema, so comparing against it proves nothing: %v",
			replayExpectedPath, err)
	}

	front, body := replaySplit(t, produced)

	// --- the fields the acceptance names ----------------------------------
	if got := replayString(front, "source", "tier"); got != "semantic" {
		t.Errorf("source.tier = %q, want semantic", got)
	}
	if got := replayString(front, "source", "hook"); got != "PreCompact" {
		t.Errorf("source.hook = %q, want PreCompact", got)
	}

	alternatives, _ := front["alternatives"].([]any)
	if len(alternatives) == 0 {
		t.Fatal("the entry carries no alternatives; the whole point of the semantic tier is the road it names as refused")
	}
	for i, raw := range alternatives {
		alternative, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("alternatives[%d] is %T, not a mapping", i, raw)
		}
		whyNot, _ := alternative["why_not"].(string)
		if strings.TrimSpace(whyNot) == "" {
			t.Errorf("alternatives[%d].why_not is empty; an option with no reason is not an alternative", i)
		}
	}
	t.Logf("OBSERVED  %d alternative(s), each with a non-empty why_not", len(alternatives))

	if to := replayEdgeTarget(t, front, "derives_from"); to == "" {
		t.Error("the entry carries no derives_from edge, so the extraction is orphaned from the capture it extends")
	} else {
		t.Logf("OBSERVED  derives_from -> %s", to)
	}

	if strings.TrimSpace(body) == "" {
		t.Error("the entry has an empty markdown body; the because is the thing tier 2 exists to add")
	}

	// dec-0025: only a human disposition writes this, and the semantic tier
	// is not a human disposition.
	if confirmed, ok := front["confirmed_by"]; ok {
		if s, _ := confirmed.(string); s == "human" {
			t.Error("the replayed entry claims confirmed_by: human — tier 2 proposes and a person disposes (dec-0025)")
		}
	} else {
		t.Log("OBSERVED  no confirmed_by")
	}

	// --- and every field, against the recorded expectation ----------------
	replayCompare(t, produced, []byte(golden))
}

// replayEmptyWhyNot is the other side, and the reason the green above is
// evidence rather than a description of one run.
//
// The mutation is the one the skill's hardest rule is about: an alternative
// offered with no reason. If `dira log` took it, the replay would go on passing
// while the ledger filled with options nobody refused for any stated reason —
// and the half above would look exactly the same.
func replayEmptyWhyNot(t *testing.T, binary string) {
	recorded := replayReadFixture(t, replayResponsePath)

	const whyNot = "--why-not 'A verdict that needs a live session produces nothing at a terminal with the network unplugged.'"
	mutated := strings.Replace(recorded, whyNot, "--why-not ''", 1)
	if mutated == recorded {
		t.Fatalf("the mutation changed nothing, so the rejection below would be about the recorded response.\n"+
			"%s no longer carries %s", replayResponsePath, whyNot)
	}

	call := replayTheCall(t, mutated)
	root := replayLedger(t)
	before := replayEntryFiles(t, root)

	result := replayRun(t, binary, root, call)
	if result.code == 0 {
		t.Fatalf("dira log accepted an alternative with an empty why_not and exited 0 (stdout %q)", result.stdout)
	}
	if !strings.Contains(result.stderr, "why_not") {
		t.Errorf("the refusal never names why_not, so it may be refusing something else:\n%s", result.stderr)
	}
	t.Logf("OBSERVED  exit %d: %s", result.code, replayFirstLine(result.stderr))

	// Refused is not enough. A command that reports a failure after writing
	// the file has still put an invalid entry in the record (dec-0002).
	after := replayEntryFiles(t, root)
	if added := replayAdded(before, after); len(added) != 0 {
		t.Errorf("the refused call still wrote %v under .dira/entries", added)
	}
	if result.stdout != "" {
		t.Errorf("the refused call printed %q on stdout, where an id would go", result.stdout)
	}
}

// TestReplaySchemaCheckCanFail proves the schema clause of TestReplay is capable
// of a red.
//
// Validating a file that the writer just produced is the easiest assertion in
// this package to get wrong: a validator that had failed to compile, or one
// reading a field that is no longer there, accepts everything and looks
// identical to a correct one. So the same validator is handed the same entry
// with one enum value corrupted, and has to reject it.
func TestReplaySchemaCheckCanFail(t *testing.T) {
	t.Parallel()

	validator, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("compiling schema/entry.schema.json: %v", err)
	}

	golden := replayReadFixture(t, replayExpectedPath)
	if err := validator.Validate([]byte(golden)); err != nil {
		t.Fatalf("the untouched %s is rejected, so the red below would mean nothing: %v", replayExpectedPath, err)
	}

	for _, tc := range []struct{ name, from, to string }{
		{"a tier outside the vocabulary", "tier: semantic", "tier: telepathic"},
		{"a hook outside the vocabulary", "hook: PreCompact", "hook: PreVacation"},
		{"an alternative with no why_not", "    why_not: >", "    unrelated_note: >"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			corrupted := strings.Replace(golden, tc.from, tc.to, 1)
			if corrupted == golden {
				t.Fatalf("%s no longer contains %q, so nothing was corrupted", replayExpectedPath, tc.from)
			}
			if err := validator.Validate([]byte(corrupted)); err == nil {
				t.Fatal("the published schema accepted it")
			} else {
				t.Logf("OBSERVED  rejected: %s", replayFirstLine(err.Error()))
			}
		})
	}
}

// TestReplayExtractorFindsNothingInProse is the non-vacuity half of the
// extraction.
//
// TestReplay fails when it finds no call, but "found one" is only evidence if
// the extractor is capable of finding none — an extractor that returned a
// hard-coded command would satisfy every clause above while the recorded
// response drifted into saying something else entirely.
func TestReplayExtractorFindsNothingInProse(t *testing.T) {
	t.Parallel()

	const prose = "The session read the handoff and decided the transcript never said why the\n" +
		"hosted renderer lost, so it wrote nothing. Running `dira log` here would mean\n" +
		"inventing the reason.\n\n```\ngrep -l 'tier: regex' .dira/entries/dec-*.md\n```\n"

	calls, err := replayExtract(prose)
	if err != nil {
		t.Fatalf("extracting from prose: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("the extractor found %d invocation(s) in a response that makes none: %v", len(calls), calls)
	}
	t.Log("OBSERVED  0 invocations in a response that declines to log")
}

// ---- replaying -------------------------------------------------------------

// replayResult is one execution of the built binary.
type replayResult struct {
	stdout string
	stderr string
	code   int
}

// replayRun executes an extracted call against the built binary, in a real
// working directory containing a real .dira.
//
// The working directory is what `dira` resolves the ledger from, and docs/lore.md
// L-0014 is about the alternative: a command pointed at a fixture directory with
// no .dira above it walks up and grades itself against this repository's own
// ledger, silently. root is a t.TempDir, so there is nothing above it to find.
func replayRun(t *testing.T, binary, root string, call []string) replayResult {
	t.Helper()

	if len(call) == 0 || call[0] != "dira" {
		t.Fatalf("the recorded call is %v; the first word has to be `dira`", call)
	}

	cmd := exec.Command(binary, call[1:]...)
	cmd.Dir = root
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Env is not inherited: a stray DIRA_* or a HOME pointing at a real
	// checkout would make this test a report on the machine that ran it.
	cmd.Env = []string{"HOME=" + root, "PATH=" + os.Getenv("PATH")}

	err := cmd.Run()
	if cmd.ProcessState == nil {
		// The process never ran at all, which is not a verdict about the
		// call: report it as the environment failure it is.
		t.Fatalf("running the recorded call: %v", err)
	}
	return replayResult{stdout: stdout.String(), stderr: stderr.String(), code: cmd.ProcessState.ExitCode()}
}

// replayLedger creates an empty ledger in a temp directory and seeds the tier-1
// capture the recorded call derives from.
func replayLedger(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	entries := filepath.Join(root, ".dira", "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatalf("creating a ledger: %v", err)
	}
	if err := os.WriteFile(filepath.Join(entries, "dec-0001.md"), []byte(replayCapture), 0o644); err != nil {
		t.Fatalf("seeding the capture: %v", err)
	}
	return root
}

// replayBuildDira builds the binary under test, skipping where there is no
// toolchain, as cmd/dira/build_test.go does.
func replayBuildDira(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("builds a binary; skipped under -short")
	}
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

// ---- extraction ------------------------------------------------------------

// replayTheCall returns the single `dira` invocation a recorded response
// carries, failing loudly when there is not exactly one.
//
// Exactly one, rather than "the first": the recorded response is a reply about
// one staged id, and a fixture that grew a second call would otherwise be
// replayed in part, with the untested half looking tested.
func replayTheCall(t *testing.T, response string) []string {
	t.Helper()

	calls, err := replayExtract(response)
	if err != nil {
		t.Fatalf("extracting the invocation from the recorded response: %v", err)
	}
	if len(calls) == 0 {
		t.Fatalf("no `dira` invocation was extracted from the recorded response, so nothing was replayed. "+
			"An extractor that finds nothing passes every assertion about what it found; see %s", replayResponsePath)
	}
	if len(calls) != 1 {
		t.Fatalf("the recorded response carries %d invocations, want exactly 1: %v", len(calls), calls)
	}
	if len(calls[0]) < 2 || calls[0][1] != "log" {
		t.Fatalf("the recorded invocation is %v, want a `dira log`", calls[0])
	}
	t.Logf("OBSERVED  extracted %d-argument invocation: dira %s …", len(calls[0]), calls[0][1])
	return calls[0]
}

// replayExtract pulls every `dira …` invocation out of a recorded response.
//
// Only fenced code blocks are read. A response is prose that mentions commands
// as well as issuing them — "`dira log <id>` adds edges and tags" is a sentence
// about the tool, not a call — and a scanner that could not tell the two apart
// would replay a sentence. Backslash-continued lines are joined first, because
// the shape the handoff hands the session is a multi-line one.
//
// This is deliberately not E2-L2-T5's extractor: T5 answers "does every command
// the skill names exist", over SKILL.md, and this answers "what did the session
// actually type", over a recording. Sharing one would couple a change in either
// question to the other.
func replayExtract(response string) ([][]string, error) {
	var calls [][]string

	inFence := false
	var pending string
	for _, line := range strings.Split(response, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			pending = ""
			continue
		}
		if !inFence {
			continue
		}

		trimmed := strings.TrimRight(line, " \t")
		if continued := strings.HasSuffix(trimmed, `\`); continued {
			pending += strings.TrimSuffix(trimmed, `\`)
			continue
		}
		command := strings.TrimSpace(pending + trimmed)
		pending = ""
		if !strings.HasPrefix(command, "dira ") {
			continue
		}

		words, err := replayWords(command)
		if err != nil {
			return nil, err
		}
		calls = append(calls, words)
	}
	if pending != "" {
		return nil, errReplay("a fenced command ends with a line continuation and never continues: " + pending)
	}
	return calls, nil
}

// replayWords splits a command line the way a shell would, for the subset of
// quoting the skill's invocations use: single quotes are literal, double quotes
// honour a backslash, and a backslash outside quotes escapes the next character.
//
// It refuses what it cannot split rather than guessing. A tokeniser that
// silently dropped an unterminated quote would hand `dira log` a truncated
// argument list and blame the binary for the result.
func replayWords(command string) ([]string, error) {
	var (
		words []string
		word  strings.Builder
		open  bool
	)
	flush := func() {
		if open {
			words = append(words, word.String())
			word.Reset()
			open = false
		}
	}

	for i := 0; i < len(command); i++ {
		switch c := command[i]; c {
		case ' ', '\t':
			flush()

		case '\'':
			open = true
			end := strings.IndexByte(command[i+1:], '\'')
			if end < 0 {
				return nil, errReplay("unterminated single quote in: " + command)
			}
			word.WriteString(command[i+1 : i+1+end])
			i += end + 1

		case '"':
			open = true
			closed := false
			for i++; i < len(command); i++ {
				if command[i] == '\\' && i+1 < len(command) {
					i++
					word.WriteByte(command[i])
					continue
				}
				if command[i] == '"' {
					closed = true
					break
				}
				word.WriteByte(command[i])
			}
			if !closed {
				return nil, errReplay("unterminated double quote in: " + command)
			}

		case '\\':
			if i+1 >= len(command) {
				return nil, errReplay("a trailing backslash in: " + command)
			}
			i++
			open = true
			word.WriteByte(command[i])

		default:
			open = true
			word.WriteByte(c)
		}
	}
	flush()
	return words, nil
}

// errReplay is a package-local error string, so this file needs no error type of
// its own and collides with nothing.
type errReplay string

func (e errReplay) Error() string { return string(e) }

// ---- comparing -------------------------------------------------------------

// replayCompare asserts two entry files agree on every field except the
// allocated id and the timestamps.
//
// Those three are excluded because they are the ledger's to set: the id is the
// lowest unused number for the kind, and `created` is the wall clock. Excluding
// anything else would be excluding the thing under test.
func replayCompare(t *testing.T, produced, expected []byte) {
	t.Helper()

	producedFront, producedBody := replaySplit(t, produced)
	expectedFront, expectedBody := replaySplit(t, expected)

	for _, field := range []string{"id", "created", "updated"} {
		delete(producedFront, field)
		delete(expectedFront, field)
	}

	// Before comparing: two documents that both decoded to nothing are equal,
	// and that equality would be reported as a pass.
	if len(expectedFront) == 0 {
		t.Fatalf("%s decoded to no fields at all, so the comparison below would compare nothing", replayExpectedPath)
	}

	if !reflect.DeepEqual(producedFront, expectedFront) {
		t.Errorf("the replayed entry's frontmatter differs from %s.\ngot:\n%s\nwant:\n%s",
			replayExpectedPath, replayYAML(producedFront), replayYAML(expectedFront))
	}
	if producedBody != expectedBody {
		t.Errorf("the replayed entry's body differs from %s.\ngot:\n%q\nwant:\n%q",
			replayExpectedPath, producedBody, expectedBody)
	}
}

// replaySplit decodes an entry file into its frontmatter and its body.
func replaySplit(t *testing.T, entry []byte) (map[string]any, string) {
	t.Helper()

	front, body, err := schema.SplitFrontmatter(entry)
	if err != nil {
		t.Fatalf("splitting the entry file: %v", err)
	}
	var fields map[string]any
	if err := yaml.Unmarshal(front, &fields); err != nil {
		t.Fatalf("parsing the entry's frontmatter: %v", err)
	}
	return fields, string(body)
}

// replayString reads a nested string field, returning "" when any step is
// missing. The caller reports what it wanted; a nil-deref here would report
// nothing useful.
func replayString(front map[string]any, path ...string) string {
	var cursor any = front
	for _, key := range path {
		mapping, ok := cursor.(map[string]any)
		if !ok {
			return ""
		}
		cursor = mapping[key]
	}
	value, _ := cursor.(string)
	return value
}

// replayEdgeTarget returns the target of the first edge of a type, or "".
func replayEdgeTarget(t *testing.T, front map[string]any, edgeType string) string {
	t.Helper()

	edges, _ := front["edges"].([]any)
	for i, raw := range edges {
		edge, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("edges[%d] is %T, not a mapping", i, raw)
		}
		if value, _ := edge["type"].(string); value == edgeType {
			to, _ := edge["to"].(string)
			return to
		}
	}
	return ""
}

// ---- small helpers ---------------------------------------------------------

// replayReadFixture reads a fixture and fails on an empty one, because every
// assertion downstream holds of the empty string.
func replayReadFixture(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		t.Fatalf("%s is empty", path)
	}
	return string(data)
}

// replayEntryFiles lists the entry files in a ledger, sorted.
func replayEntryFiles(t *testing.T, root string) []string {
	t.Helper()

	names, err := filepath.Glob(filepath.Join(root, ".dira", "entries", "*.md"))
	if err != nil {
		t.Fatalf("listing the ledger: %v", err)
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, filepath.Base(name))
	}
	return out
}

// replayAdded returns the names present in after and absent from before.
func replayAdded(before, after []string) []string {
	was := map[string]bool{}
	for _, name := range before {
		was[name] = true
	}
	var added []string
	for _, name := range after {
		if !was[name] {
			added = append(added, name)
		}
	}
	return added
}

// replayYAML re-renders decoded frontmatter for a failure message.
func replayYAML(fields map[string]any) string {
	out, err := yaml.Marshal(fields)
	if err != nil {
		return "<unrenderable: " + err.Error() + ">"
	}
	return string(out)
}

// replayFirstLine is the first line of a message, for a log that should stay one
// line.
func replayFirstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
