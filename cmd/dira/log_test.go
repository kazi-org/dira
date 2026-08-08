package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/schema"
)

// The tests in this file assert the write path's central claim in the form the
// lane's acceptance line states it: not "it looks right" but "exactly one path
// under .dira changed, and here it is". Every one of them snapshots the whole
// ledger directory by content hash before the command runs and again after, so a
// second file appearing anywhere — a cache, a lock, a stray temporary — fails
// the test that was watching something else.

// ledgerRoot creates an empty ledger and returns the directory containing it.
func ledgerRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".dira", "entries"), 0o755); err != nil {
		t.Fatalf("creating a ledger: %v", err)
	}
	return root
}

// seedEntry writes an entry file straight into the ledger, without going through
// the command under test.
func seedEntry(t *testing.T, root, id, content string) {
	t.Helper()

	path := filepath.Join(root, ".dira", "entries", id+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("seeding %s: %v", id, err)
	}
}

// seedRealEntry copies one of this repository's own entries into a test ledger.
//
// The hand-wrapped originals are the fixture that matters: dec-0002's promise
// that "a PR touching a decision shows a legible diff" is only tested by a file
// somebody actually hand-wrapped, and a synthetic entry written by the encoder
// would round-trip through it for reasons that say nothing about the real ones.
func seedRealEntry(t *testing.T, root, id string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", ".dira", "entries", id+".md"))
	if err != nil {
		t.Fatalf("reading this repository's %s: %v", id, err)
	}
	seedEntry(t, root, id, string(data))
	return string(data)
}

// minimalEntry is the smallest valid entry file, for seeding a ledger with ids
// the allocator has to work around.
func minimalEntry(id, kind, state string) string {
	entry := "---\nid: " + id + "\nkind: " + kind + "\ntitle: A seeded entry\nstate: " + state +
		"\ncreated: \"2026-07-29T20:00:00Z\"\n"
	if kind == "decision" {
		entry += "alternatives:\n  - option: Not doing it\n    why_not: the schema requires a decision to carry one\n"
	}
	return entry + "---\n\nSeeded.\n"
}

// result is one invocation of the command.
type result struct {
	code   int
	stdout string
	stderr string
}

// runDira runs the real command registry against the ledger at root, with a
// clock and a stdin a test controls. `-C` is appended, so a caller writes the
// arguments in the order a person would type them.
func runDira(t *testing.T, root, stdin string, args ...string) result {
	t.Helper()

	var out, errBuf bytes.Buffer
	a := newApp(&out, &errBuf)
	a.stdin = strings.NewReader(stdin)
	a.now = func() time.Time { return time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC) }

	full := append([]string{"log"}, args...)
	full = append(full, "-C", root)
	code := a.main(full)
	return result{code: code, stdout: out.String(), stderr: errBuf.String()}
}

// snapshot hashes every file under root, so a test can assert what changed
// rather than look at what it expected to change.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()

	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return out
}

// modifiedPaths is the set the acceptance line talks about: every path that was
// added, removed, or whose bytes changed.
func modifiedPaths(before, after map[string]string) []string {
	var paths []string
	for path, sum := range after {
		if before[path] != sum {
			paths = append(paths, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	return paths
}

// readEntry returns an entry file's bytes.
func readEntry(t *testing.T, root, id string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, ".dira", "entries", id+".md"))
	if err != nil {
		t.Fatalf("reading the entry the command wrote: %v", err)
	}
	return data
}

// validateAgainstSchema checks a file against entry.schema.json — the published
// contract, not dira's reading of it.
func validateAgainstSchema(t *testing.T, entryFile []byte) {
	t.Helper()

	v, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("building the schema validator: %v", err)
	}
	if err := v.Validate(entryFile); err != nil {
		t.Errorf("the entry dira wrote does not satisfy entry.schema.json: %v\n--- entry ---\n%s", err, entryFile)
	}
}

// TestLogWritesExactlyOneFile is the lane's first acceptance clause, verbatim: a
// decision with two alternatives and a derives_from edge, one new file, schema
// valid, lowest unused id.
func TestLogWritesExactlyOneFile(t *testing.T) {
	t.Parallel()

	root := ledgerRoot(t)
	before := snapshot(t, root)

	got := runDira(t, root, "",
		"--kind", "decision",
		"--title", "Allocate ids with a compare-and-swap rather than a scan",
		"--alternative", "Scan for the highest id and add one",
		"--why-not", "two writers that scan at the same moment produce one entry where there should be two, and nothing reports it",
		"--alternative", "Take a lock file around the write",
		"--why-not", "there is no lock to take over the GitHub Contents API, so the allocator would need replacing for E7",
		"--revisit-if", "the ledger only ever has one writer",
		"--edge", "derives_from=int-0002",
		"--edge-note", "unattended concurrent capture is the normal case",
		"--tag", "storage",
		"--tier", "human",
		"--confirmed-by", "human",
		"--body", "Create is exclusive, so a losing racer retries instead of clobbering.",
	)

	if got.code != exitOK {
		t.Fatalf("exit code = %d, want %d\n--- stderr ---\n%s", got.code, exitOK, got.stderr)
	}
	if got.stdout != "dec-0001\n" {
		t.Errorf("stdout = %q, want the allocated id and nothing else", got.stdout)
	}

	paths := modifiedPaths(before, snapshot(t, root))
	if len(paths) != 1 {
		t.Fatalf("the write touched %d paths, want exactly 1 (dec-0002): %v", len(paths), paths)
	}
	if paths[0] != ".dira/entries/dec-0001.md" {
		t.Errorf("the write touched %q, want .dira/entries/dec-0001.md", paths[0])
	}

	written := readEntry(t, root, "dec-0001")
	validateAgainstSchema(t, written)

	for _, want := range []string{
		"kind: decision",
		"state: accepted",
		`created: "2026-07-30T09:00:00Z"`,
		"type: derives_from",
		"to: int-0002",
		"note: unattended concurrent capture is the normal case",
		"option: Scan for the highest id and add one",
		"revisit_if: the ledger only ever has one writer",
		"tags: [storage]",
		"hook: manual",
		"tier: human",
		"confirmed_by: human",
		"Create is exclusive, so a losing racer retries instead of clobbering.",
	} {
		if !strings.Contains(string(written), want) {
			t.Errorf("the written entry does not carry %q\n--- entry ---\n%s", want, written)
		}
	}
}

func TestLogAllocatesTheLowestUnusedIDForTheKind(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		existing map[string]string
		kind     string
		want     string
	}{
		{
			name: "an empty ledger",
			kind: "note",
			want: "note-0001",
		},
		{
			name:     "the gap, not the next number after the highest",
			existing: map[string]string{"note-0001": "note", "note-0003": "note"},
			kind:     "note",
			want:     "note-0002",
		},
		{
			name:     "another kind's numbering does not move this one",
			existing: map[string]string{"dec-0001": "decision", "dec-0002": "decision"},
			kind:     "intent",
			want:     "int-0001",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := ledgerRoot(t)
			for id, kind := range tc.existing {
				state := map[string]string{"note": "active", "decision": "accepted", "intent": "active"}[kind]
				seedEntry(t, root, id, minimalEntry(id, kind, state))
			}

			got := runDira(t, root, "", "--kind", tc.kind, "--title", "The entry being allocated an id")
			if got.code != exitOK {
				t.Fatalf("exit code = %d\n--- stderr ---\n%s", got.code, got.stderr)
			}
			if got.stdout != tc.want+"\n" {
				t.Errorf("allocated %q, want %q", strings.TrimSpace(got.stdout), tc.want)
			}
		})
	}
}

// TestAddingAnEdgeChangesOneFileAndLeavesEveryOtherLineAlone is the third
// acceptance clause, against the entry the constraint was written about.
//
// The file is this repository's real dec-0002, hand-wrapped. Both halves matter:
// the modified-path set has cardinality one (asserted, not eyeballed), and
// inside that one file every original line survives, in order — a codec that
// reflowed the paragraphs would still touch one path and would still make
// dec-0002's "a PR touching a decision shows a legible diff" false.
func TestAddingAnEdgeChangesOneFileAndLeavesEveryOtherLineAlone(t *testing.T) {
	t.Parallel()

	// Two entries, and the second one is not decoration. dec-0002 is the
	// decision this whole property was bought by, but it happens to be one
	// of the entries whose wrapping a canonical emitter reproduces by luck,
	// so a codec that had thrown its formatting away would still pass on it.
	// dec-0005's does not survive canonical emission, so it is the one that
	// notices.
	for _, id := range []string{"dec-0002", "dec-0005"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			root := ledgerRoot(t)
			original := seedRealEntry(t, root, id)
			before := snapshot(t, root)

			got := runDira(t, root, "", id,
				"--edge", "informs=dec-0016",
				"--edge-note", "the write path is where one-file-per-mutation is honoured or lost",
			)
			if got.code != exitOK {
				t.Fatalf("exit code = %d\n--- stderr ---\n%s", got.code, got.stderr)
			}
			if got.stdout != id+"\n" {
				t.Errorf("stdout = %q, want the id of the entry it wrote", got.stdout)
			}

			wantPath := ".dira/entries/" + id + ".md"
			paths := modifiedPaths(before, snapshot(t, root))
			if len(paths) != 1 || paths[0] != wantPath {
				t.Fatalf("adding an edge touched %v, want exactly [%s]", paths, wantPath)
			}

			written := string(readEntry(t, root, id))
			validateAgainstSchema(t, []byte(written))

			oldLines := strings.Split(original, "\n")
			newLines := strings.Split(written, "\n")

			// Every original line, in its original order. A greedy
			// scan is the right check precisely because nothing may
			// be removed: if a line is, the scan runs off the end
			// and says which one went missing.
			i := 0
			for _, line := range oldLines {
				for i < len(newLines) && newLines[i] != line {
					i++
				}
				if i == len(newLines) {
					t.Fatalf("the line %q did not survive the write, or moved\n--- written ---\n%s", line, written)
				}
				i++
			}

			// Three lines for the edge, one for updated. Anything
			// more is a reflow, and a reflow is the forty-line diff
			// dec-0002 promises a reviewer will not see.
			const wantAdded = 4
			if added := len(newLines) - len(oldLines); added != wantAdded {
				t.Errorf("the write added %d lines, want %d (the edge's three, plus updated)\n--- written ---\n%s",
					added, wantAdded, written)
			}
			for _, want := range []string{
				"  - type: informs\n    to: dec-0016\n    note: the write path is where one-file-per-mutation is honoured or lost\n",
				`updated: "2026-07-30T09:00:00Z"`,
			} {
				if !strings.Contains(written, want) {
					t.Errorf("the written entry does not carry %q", want)
				}
			}
		})
	}
}

// TestAddingAnEdgeThatIsAlreadyThereWritesNothing keeps the unattended caller in
// mind: a Stop hook that reaches the same conclusion twice must not fail, and
// must not rewrite the file either.
func TestAddingAnEdgeThatIsAlreadyThereWritesNothing(t *testing.T) {
	t.Parallel()

	root := ledgerRoot(t)
	seedRealEntry(t, root, "dec-0002")

	first := runDira(t, root, "", "dec-0002", "--edge", "informs=dec-0015")
	if first.code != exitOK {
		t.Fatalf("first invocation: exit %d\n%s", first.code, first.stderr)
	}

	before := snapshot(t, root)
	second := runDira(t, root, "", "dec-0002", "--edge", "informs=dec-0015")
	if second.code != exitOK {
		t.Fatalf("second invocation: exit %d\n%s", second.code, second.stderr)
	}

	if paths := modifiedPaths(before, snapshot(t, root)); len(paths) != 0 {
		t.Errorf("repeating the same edge rewrote %v", paths)
	}
	if second.stdout != "dec-0002\n" {
		t.Errorf("stdout = %q, want the id even when nothing was written", second.stdout)
	}
	if !strings.Contains(second.stderr, "nothing written") {
		t.Errorf("stderr does not say the write was a no-op: %q", second.stderr)
	}
}

func TestAddingAnEdgeWithADifferentNoteIsRefused(t *testing.T) {
	t.Parallel()

	root := ledgerRoot(t)
	seedRealEntry(t, root, "dec-0002")
	before := snapshot(t, root)

	// dec-0002 already carries derives_from int-0002 with a note on it.
	got := runDira(t, root, "", "dec-0002",
		"--edge", "derives_from=int-0002",
		"--edge-note", "a completely different reason",
	)
	if got.code != exitError {
		t.Fatalf("exit code = %d, want %d\n--- stderr ---\n%s", got.code, exitError, got.stderr)
	}
	if !strings.Contains(got.stderr, "int-0002") {
		t.Errorf("stderr does not name the conflicting edge: %q", got.stderr)
	}
	if paths := modifiedPaths(before, snapshot(t, root)); len(paths) != 0 {
		t.Errorf("a refused edge still rewrote %v", paths)
	}
}

// TestAnInvalidEntryNeverReachesTheLedger is the validation clause: rejected
// with the failing field named, exit 2, and the ledger byte-identical.
func TestAnInvalidEntryNeverReachesTheLedger(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "a sixth kind names the constraint that closed the set",
			args: []string{"--kind", "task", "--title", "Ship the thing by Friday"},
			want: "cst-0002",
		},
		{
			name: "a decision with no alternatives",
			args: []string{"--kind", "decision", "--title", "A decision that is really an assertion"},
			want: "alternative",
		},
		{
			name: "an alternative with no why_not",
			args: []string{"--kind", "decision", "--title", "A decision with a bare option", "--alternative", "The other way"},
			want: "why_not is required",
		},
		{
			name: "a state the kind does not allow",
			args: []string{"--kind", "question", "--title", "A question claiming to be accepted", "--state", "accepted"},
			want: "not valid for kind",
		},
		{
			name: "a title too short to be legible",
			args: []string{"--kind", "note", "--title", "hm"},
			want: "title",
		},
		{
			name: "an edge target that is not a ref",
			args: []string{"--kind", "note", "--title", "A note pointing at a feeling", "--edge", "derives_from=the big idea"},
			want: "edges[0]",
		},
		{
			name: "an edge that is not TYPE=TARGET",
			args: []string{"--kind", "note", "--title", "A note with a malformed edge", "--edge", "derives_from int-0002"},
			want: "TYPE=TARGET",
		},
		{
			name: "a why-not with no alternative in front of it",
			args: []string{"--kind", "decision", "--title", "A decision with a loose reason", "--why-not", "because"},
			want: "no --alternative came first",
		},
		{
			name: "a hook outside the capture points",
			args: []string{"--kind", "note", "--title", "A note from a hook that does not exist", "--hook", "OnTuesday"},
			want: "source.hook",
		},
		{
			name: "neither --kind nor --stdin",
			args: []string{"--title", "An entry of no particular kind"},
			want: "--kind",
		},
		{
			name: "an id given after the flags rather than before them",
			args: []string{"--tag", "storage", "dec-0002"},
			want: "the id goes before the flags",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := ledgerRoot(t)
			seedEntry(t, root, "note-0001", minimalEntry("note-0001", "note", "active"))
			before := snapshot(t, root)

			got := runDira(t, root, "", tc.args...)
			if got.code != exitUsage {
				t.Errorf("exit code = %d, want %d for a caller mistake\n--- stderr ---\n%s", got.code, exitUsage, got.stderr)
			}
			if got.stdout != "" {
				t.Errorf("stdout = %q, want empty: a caller parsing it must never get help text", got.stdout)
			}
			if !strings.Contains(got.stderr, tc.want) {
				t.Errorf("stderr does not name the problem; want substring %q\n--- stderr ---\n%s", tc.want, got.stderr)
			}

			if paths := modifiedPaths(before, snapshot(t, root)); len(paths) != 0 {
				t.Errorf("a rejected entry still changed %v; the pre-write state must be byte-identical", paths)
			}
		})
	}
}

func TestLogReadsACompleteEntryFromStdin(t *testing.T) {
	t.Parallel()

	const document = `---
kind: decision
title: Hand the whole entry to dira rather than encode it into argv
state: staged
tags: [capture]
edges:
  - type: derives_from
    to: dec-0003
alternatives:
  - option: A flag per field, with a delimiter syntax for the nested ones
    why_not: >
      Alternatives and excerpts are prose, and every delimiter
      that encodes them into argv is one the prose eventually
      contains.
source:
  hook: Stop
  session: 01J8Z
  tier: semantic
confirmed_by: agent:claude-code
---

The skill fills in the because and calls dira log with the complete entry.
`

	root := ledgerRoot(t)
	before := snapshot(t, root)

	got := runDira(t, root, document, "--stdin")
	if got.code != exitOK {
		t.Fatalf("exit code = %d\n--- stderr ---\n%s", got.code, got.stderr)
	}
	if got.stdout != "dec-0001\n" {
		t.Errorf("stdout = %q", got.stdout)
	}

	paths := modifiedPaths(before, snapshot(t, root))
	if len(paths) != 1 || paths[0] != ".dira/entries/dec-0001.md" {
		t.Fatalf("--stdin touched %v, want exactly [.dira/entries/dec-0001.md]", paths)
	}

	written := string(readEntry(t, root, "dec-0001"))
	validateAgainstSchema(t, []byte(written))

	// The author's own wrapping, not the encoder's.
	for _, line := range []string{
		"      Alternatives and excerpts are prose, and every delimiter",
		"      that encodes them into argv is one the prose eventually",
		"      contains.",
	} {
		if !strings.Contains(written, line+"\n") {
			t.Errorf("the entry was reflowed; %q is not in it\n--- entry ---\n%s", line, written)
		}
	}
	// The two fields dira supplies, and nothing else invented.
	if !strings.Contains(written, "id: dec-0001\n") || !strings.Contains(written, `created: "2026-07-30T09:00:00Z"`) {
		t.Errorf("id and created were not stamped:\n%s", written)
	}
	if !strings.Contains(written, "state: staged") || !strings.Contains(written, "tier: semantic") {
		t.Errorf("the caller's own fields did not survive:\n%s", written)
	}
	if strings.Contains(written, "hook: manual") {
		t.Errorf("the caller said the entry came from a Stop hook and dira overrode it:\n%s", written)
	}
}

func TestStdinAndTheFieldFlagsAreTwoWaysOfSayingTheSameThing(t *testing.T) {
	t.Parallel()

	root := ledgerRoot(t)
	before := snapshot(t, root)

	got := runDira(t, root, "---\nkind: note\ntitle: A note\nstate: active\n---\n", "--stdin", "--title", "Another title")
	if got.code != exitUsage {
		t.Errorf("exit code = %d, want %d", got.code, exitUsage)
	}
	if !strings.Contains(got.stderr, "--stdin") {
		t.Errorf("stderr does not explain the conflict: %q", got.stderr)
	}
	if paths := modifiedPaths(before, snapshot(t, root)); len(paths) != 0 {
		t.Errorf("a rejected invocation wrote %v", paths)
	}
}

func TestBodyCanComeFromStdin(t *testing.T) {
	t.Parallel()

	root := ledgerRoot(t)
	got := runDira(t, root, "A body with\n\ntwo paragraphs and a trailing newline.\n",
		"--kind", "note", "--title", "A note whose body arrived on stdin", "--body", "-")
	if got.code != exitOK {
		t.Fatalf("exit code = %d\n%s", got.code, got.stderr)
	}

	written := string(readEntry(t, root, "note-0001"))
	if !strings.HasSuffix(written, "---\n\nA body with\n\ntwo paragraphs and a trailing newline.\n") {
		t.Errorf("the body was not stored as written:\n%q", written)
	}
}

func TestMutationRefusesToRewriteFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "state is disposition, which is not this command",
			args: []string{"dec-0002", "--state", "superseded"},
			want: "--state",
		},
		{
			name: "a title rewrite is a job for an editor",
			args: []string{"dec-0002", "--title", "A better title"},
			want: "--title",
		},
		{
			name: "an id that is not an id",
			args: []string{"dec-2", "--tag", "storage"},
			want: "not an entry id",
		},
		{
			name: "nothing to add",
			args: []string{"dec-0002"},
			want: "nothing to add",
		},
		{
			name: "a tag the schema does not allow",
			args: []string{"dec-0002", "--tag", "Not A Tag"},
			want: "tags[",
		},
		{
			name: "an edge type outside the five",
			args: []string{"dec-0002", "--edge", "causes=dec-0005"},
			want: "edges[",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := ledgerRoot(t)
			seedRealEntry(t, root, "dec-0002")
			before := snapshot(t, root)

			got := runDira(t, root, "", tc.args...)
			if got.code != exitUsage {
				t.Errorf("exit code = %d, want %d\n--- stderr ---\n%s", got.code, exitUsage, got.stderr)
			}
			if !strings.Contains(got.stderr, tc.want) {
				t.Errorf("stderr does not explain the refusal; want %q\n--- stderr ---\n%s", tc.want, got.stderr)
			}
			if paths := modifiedPaths(before, snapshot(t, root)); len(paths) != 0 {
				t.Errorf("a refused mutation changed %v", paths)
			}
		})
	}
}

func TestMutatingAnEntryThatIsNotThereIsARuntimeError(t *testing.T) {
	t.Parallel()

	root := ledgerRoot(t)
	before := snapshot(t, root)

	got := runDira(t, root, "", "dec-0404", "--tag", "storage")
	if got.code != exitError {
		t.Errorf("exit code = %d, want %d", got.code, exitError)
	}
	if !strings.Contains(got.stderr, "dec-0404") {
		t.Errorf("stderr does not name the entry: %q", got.stderr)
	}
	if strings.Contains(got.stderr, "usage:") {
		t.Errorf("a missing entry printed usage; that is reserved for exit %d", exitUsage)
	}
	if paths := modifiedPaths(before, snapshot(t, root)); len(paths) != 0 {
		t.Errorf("a failed mutation changed %v", paths)
	}
}

func TestLogOutsideALedgerSaysSoAndWritesNothing(t *testing.T) {
	t.Parallel()

	// A directory with no .dira anywhere above it. t.TempDir is under
	// /tmp, which is not inside a repository.
	root := t.TempDir()
	before := snapshot(t, root)

	got := runDira(t, root, "", "--kind", "note", "--title", "A note with nowhere to go")
	if got.code != exitError {
		t.Errorf("exit code = %d, want %d", got.code, exitError)
	}
	if !strings.Contains(got.stderr, ".dira") {
		t.Errorf("stderr does not say what is missing: %q", got.stderr)
	}
	if paths := modifiedPaths(before, snapshot(t, root)); len(paths) != 0 {
		t.Errorf("dira created %v outside a ledger; Open deliberately creates nothing", paths)
	}
}

// TestLogHelpDocumentsEveryFlagItAccepts is the docs-land-with-the-code check,
// driven by the flag set rather than by a list someone has to remember to
// update. A flag that exists and is undocumented is a flag nobody finds.
func TestLogHelpDocumentsEveryFlagItAccepts(t *testing.T) {
	t.Parallel()

	var help bytes.Buffer
	writeLogUsage(&help)

	registered := 0
	(&logFlags{}).flagSet().VisitAll(func(f *flag.Flag) {
		registered++
		name := "--" + f.Name
		if f.Name == "C" {
			name = "-C"
		}
		if !strings.Contains(help.String(), name) {
			t.Errorf("`dira log -h` does not document %s\n--- help ---\n%s", name, help.String())
		}
	})
	if registered == 0 {
		t.Fatal("the flag set is empty; this check is not measuring anything")
	}

	// And the two ways of asking for it agree.
	var out, errBuf bytes.Buffer
	a := newApp(&out, &errBuf)
	if code := a.main([]string{"help", "log"}); code != exitOK {
		t.Fatalf("`dira help log` exited %d", code)
	}
	if out.String() != help.String() {
		t.Error("`dira help log` and `dira log -h` print different text")
	}
}

func TestLogHelpIsNotAnError(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	a := newApp(&out, &errBuf)
	if code := a.main([]string{"log", "-h"}); code != exitOK {
		t.Errorf("`dira log -h` exited %d, want %d", code, exitOK)
	}
	if !strings.Contains(out.String(), "--kind") {
		t.Errorf("`dira log -h` did not print the flags:\n%s", out.String())
	}
	if errBuf.String() != "" {
		t.Errorf("`dira log -h` wrote to stderr: %q", errBuf.String())
	}
}
