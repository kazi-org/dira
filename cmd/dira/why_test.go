package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/why"
)

// update rewrites the golden files instead of comparing against them.
//
//	go test ./cmd/dira -run TestWhy -update
//
// The golden files are the shared contract between this renderer and E6's: a
// renderer regression has to show up as a diff a human can read, which is the
// whole reason the expected output is a file rather than a string literal
// buried in an assertion.
var update = flag.Bool("update", false, "rewrite the golden files under testdata/why")

// exerciseWhy runs `dira why` through the real dispatcher and the real
// exit-code mapping.
//
// The command is appended to the registry here rather than in main.go because
// three other lanes are editing that file concurrently; registering it is one
// line the lead merges. Appending only when it is absent means this test keeps
// working unchanged once that line lands, and it exercises a.main either way —
// so the exit codes asserted below are the contract E2's hooks depend on, not a
// local re-derivation of it.
func exerciseWhy(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errBuf bytes.Buffer
	a := newApp(&out, &errBuf)
	if a.lookup("why") == nil {
		a.commands = append(a.commands, &command{
			name:    "why",
			summary: whySummary,
			run:     runWhy,
			usage:   writeWhyUsage,
		})
	}
	code = a.main(append([]string{"why"}, args...))
	return code, out.String(), errBuf.String()
}

// repoRoot is this repository, whose .dira/ is the ledger the golden files are
// taken over. Not a fixture: a fixture's chains are generated and a regression
// in them reads as noise, while a regression in dec-0002's chain reads as a
// sentence that changed.
func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("locating the repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".dira", "entries")); err != nil {
		t.Fatalf("no ledger at %s/.dira/entries: %v", root, err)
	}
	return root
}

// goldenChains are the chains pinned against this repository's real ledger,
// chosen so that between them they cover every element the acceptance line
// names that this ledger can express.
var goldenChains = []struct {
	name  string
	query string
	// covers is what this case is here to pin, so a case deleted for being
	// "redundant" takes a visible obligation with it.
	covers string
}{
	{
		name:   "dec-0002",
		query:  "dec-0002",
		covers: "three alternatives with why_nots, one carrying revisit_if; a derives_from parent's title; an outgoing informs edge",
	},
	{
		name:   "daemon",
		query:  "daemon",
		covers: "resolution by term; an entry with no alternatives; nine incoming derives_from edges",
	},
	{
		name:   "int-0002",
		query:  "int-0002",
		covers: "the id form of the case above, which must be the same chain",
	},
	{
		name:   "dec-0012",
		query:  "dec-0012",
		covers: "a superseded-by relation, an outgoing supersedes edge, and revisit_if on two of three alternatives",
	},
	{
		name:   "dec-0015",
		query:  "dec-0015",
		covers: "five alternatives, four of them carrying revisit_if — the long-content case this ledger has",
	},
}

func TestWhyOnTheRealLedgerMatchesItsGoldenFile(t *testing.T) {
	root := repoRoot(t)

	for _, tc := range goldenChains {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := exerciseWhy(t, "-C", root, tc.query)
			if code != exitOK {
				t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
			}
			if stdout == "" {
				t.Fatalf("no output for %q (%s)", tc.query, tc.covers)
			}

			path := filepath.Join("testdata", "why", tc.name+".golden")
			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("creating %s: %v", filepath.Dir(path), err)
				}
				if err := os.WriteFile(path, []byte(stdout), 0o644); err != nil {
					t.Fatalf("writing %s: %v", path, err)
				}
				return
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v\nrun `go test ./cmd/dira -run TestWhy -update` to create it", path, err)
			}
			if stdout != string(want) {
				t.Errorf("`dira why %s` no longer renders %s.\n"+
					"This file is the contract the web renderer is held to as well; if the change is "+
					"intended, re-run with -update and read the diff.\n--- want ---\n%s\n--- got ---\n%s",
					tc.query, path, want, stdout)
			}
		})
	}
}

// TestTheGoldenFilesContainWhatTheAcceptanceLineNames is the other half of the
// golden test. A golden file proves the output has not changed; it does not
// prove the output was ever right, and a `-update` run over a broken renderer
// would pin the breakage. These assertions are about content rather than bytes,
// so they survive a legitimate re-layout and fail a lost field.
func TestTheGoldenFilesContainWhatTheAcceptanceLineNames(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	// Wide enough that nothing wraps, so a flattened comparison cannot
	// be broken up by the right-margin status column.
	_, chain, _ := exerciseWhy(t, "-C", root, "-width", "300", "dec-0002")

	// Every element docs/design.md §10.8 and the lane's acceptance line
	// require of `dira why dec-0002`, taken from the entry files rather
	// than transcribed, so a test that agrees with a renderer that dropped
	// a field is not possible.
	required := []struct {
		what string
		text string
	}{
		{"the decision's own title", "One file per entry, not an append-only JSONL ledger"},
		{"its state and date", "accepted 2026-07-29"},
		{"the first alternative", "Append-only .dira/ledger.jsonl with a SQLite cache, following Beads"},
		{"the second alternative", "A single YAML or TOML document holding all entries"},
		{"the third alternative", "SQLite as the source of truth, with git storing the .db file"},
		{"a why_not", "Every concurrent writer appends at the same offset"},
		{"another why_not", "A binary blob in git is unreviewable and unmergeable"},
		{"the revisit_if", "entry volume grows past roughly 10k per ledger"},
		{"the revisit_if label", "revisit if"},
		{"the derives_from parent's id", "int-0002"},
		{"the derives_from parent's title", "Zero-ceremony operation — one binary, no server, no daemon"},
		{"the parent's state", "active"},
		{"the edge note explaining the parent link", "files-in-git is what lets there be no server"},
		{"the outgoing informs edge", "dec-0005"},
	}

	flat := strings.Join(strings.Fields(chain), " ")
	for _, r := range required {
		if !strings.Contains(flat, strings.Join(strings.Fields(r.text), " ")) {
			t.Errorf("the chain for dec-0002 is missing %s (%q):\n%s", r.what, r.text, chain)
		}
	}
}

// ansiPattern matches an ANSI escape sequence: CSI, then parameter and
// intermediate bytes, then a final byte.
var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

func stripANSI(s string) string { return ansiPattern.ReplaceAllString(s, "") }

// TestTheANSIStripperHasTeeth stops the assertion below from passing because
// the stripper does nothing. A test that strips escapes from output containing
// none proves nothing about either.
func TestTheANSIStripperHasTeeth(t *testing.T) {
	t.Parallel()

	coloured := "\x1b[31m└─ \x1b[1;33mdec-0002\x1b[0m  refused\x1b[0m"
	want := "└─ dec-0002  refused"
	if got := stripANSI(coloured); got != want {
		t.Fatalf("stripANSI(%q) = %q, want %q", coloured, got, want)
	}
	if stripANSI("plain") != "plain" {
		t.Error("stripANSI altered text containing no escapes")
	}
}

// TestTheChainIsSelectableTextWithNoANSIGraphics is the acceptance line's
// "stripping ANSI leaves the tree intact" clause, asserted in the strong
// direction: the tree is drawn in box-drawing characters, and there is no ANSI
// to strip in the first place.
//
// DESIGN.md law 3 makes the chain type rather than a picture, and law 1 reserves
// the only colour this output would plausibly reach for. So the check is that
// the bytes are the tree — not that some colouring happens to survive removal.
func TestTheChainIsSelectableTextWithNoANSIGraphics(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, tc := range goldenChains {
		t.Run(tc.name, func(t *testing.T) {
			_, stdout, _ := exerciseWhy(t, "-C", root, tc.query)

			if i := strings.IndexByte(stdout, 0x1b); i >= 0 {
				t.Errorf("the chain contains an escape byte at offset %d; "+
					"DESIGN.md law 1 reserves colour for drift and contradiction, and a refusal is a record, not an alarm", i)
			}
			if got := stripANSI(stdout); got != stdout {
				t.Errorf("stripping ANSI changed the chain, so something was drawn with escapes:\n%s", got)
			}
			if !strings.ContainsAny(stdout, "└├│") {
				t.Errorf("the chain is drawn with no box-drawing characters:\n%s", stdout)
			}
		})
	}
}

// TestTermAndIdResolveToTheSameChain is the acceptance line's `dira why daemon`
// clause. "daemon" appears in exactly one title in this ledger — int-0002's —
// so the term form and the id form must produce the same bytes.
func TestTermAndIdResolveToTheSameChain(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	byTerm, byID := "daemon", "int-0002"

	termCode, termOut, _ := exerciseWhy(t, "-C", root, byTerm)
	idCode, idOut, _ := exerciseWhy(t, "-C", root, byID)

	if termCode != exitOK || idCode != exitOK {
		t.Fatalf("exit codes: %q -> %d, %q -> %d", byTerm, termCode, byID, idCode)
	}
	if termOut != idOut {
		t.Errorf("`dira why %s` and `dira why %s` differ.\n--- %s ---\n%s\n--- %s ---\n%s",
			byTerm, byID, byTerm, termOut, byID, idOut)
	}
	if !strings.Contains(idOut, "int-0002") {
		t.Errorf("the chain does not name the entry it resolved to:\n%s", idOut)
	}
}

// TestAnEntryWithNoAlternativesSaysSo is the acceptance line's empty-section
// clause. int-0002 is an intent and records none.
func TestAnEntryWithNoAlternativesSaysSo(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	code, stdout, stderr := exerciseWhy(t, "-C", root, "int-0002")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
	}
	if !strings.Contains(stdout, "no alternatives recorded") {
		t.Errorf("an entry with no alternatives renders no statement that it has none:\n%s", stdout)
	}
}

// TestAnUnknownRefExitsNonZeroWithoutLookingLikeACrash is the acceptance line's
// error clause.
func TestAnUnknownRefExitsNonZeroWithoutLookingLikeACrash(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, query := range []string{"nonexistent-0000", "dec-9999", "no-such-term-anywhere"} {
		t.Run(query, func(t *testing.T) {
			code, stdout, stderr := exerciseWhy(t, "-C", root, query)

			if code == exitOK {
				t.Fatalf("exit code = %d for %q, want non-zero", code, query)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty so a caller parsing it never sees an error", stdout)
			}
			if !strings.Contains(stderr, query) {
				t.Errorf("the message does not name what was looked for:\n%s", stderr)
			}

			// "Does not resemble a crash" made checkable: no panic
			// vocabulary, no goroutine dump, no Go type names, and a
			// suggestion of what to do instead.
			for _, tell := range []string{"panic", "goroutine", "runtime error", "nil pointer", "0x", "*ledger.", "index:"} {
				if strings.Contains(stderr, tell) {
					t.Errorf("the message reads like a crash — it contains %q:\n%s", tell, stderr)
				}
			}
			if strings.Count(stderr, "\n") > 1 {
				t.Errorf("the message is %d lines; a failed lookup is one sentence:\n%s", strings.Count(stderr, "\n"), stderr)
			}
			if !strings.Contains(stderr, "try") {
				t.Errorf("the message does not suggest anything to try:\n%s", stderr)
			}
		})
	}
}

// TestAnAmbiguousTermListsTheCandidates pins the disambiguation rule. "founding"
// is a tag on most of this ledger, so it is the ambiguous case the design warns
// about: a resolver that picked one of them would render something
// indistinguishable from the right answer.
func TestAnAmbiguousTermListsTheCandidates(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	code, stdout, stderr := exerciseWhy(t, "-C", root, "founding")

	if code != exitOK {
		t.Fatalf("exit code = %d, want %d — several matches is an answer, not a failure\nstderr: %s", code, exitOK, stderr)
	}
	if strings.ContainsAny(stdout, "└├") {
		t.Errorf("an ambiguous term rendered a chain; it must list candidates instead:\n%s", stdout)
	}
	for _, want := range []string{"entries match", "dira why "} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the candidate list is missing %q:\n%s", want, stdout)
		}
	}
	if n := strings.Count(stdout, "dec-"); n < 5 {
		t.Errorf("only %d decisions listed for a tag most of this ledger carries:\n%s", n, stdout)
	}
}

// TestWhyWritesNothing is dec-0004 asserted rather than intended: status is
// derived and never stored, so the command that renders it must leave the
// ledger byte-identical — including the cache, which it is allowed to build but
// not to be believed.
func TestWhyWritesNothing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	diraDir := filepath.Join(root, ".dira")
	writeWhyLedger(t, diraDir, map[string]string{
		"int-0100.md": `---
id: int-0100
kind: intent
title: A direction that a decision below it serves
state: active
created: "2026-01-01T00:00:00Z"
---

Body.
`,
		"dec-0100.md": `---
id: dec-0100
kind: decision
title: A decision that arises from an intent
state: accepted
created: "2026-01-02T00:00:00Z"
edges:
  - type: derives_from
    to: int-0100
alternatives:
  - option: Not doing it
    why_not: it would not have served the intent
---

Body.
`,
	})

	before := whySnapshot(t, filepath.Join(diraDir, "entries"))
	for _, query := range []string{"dec-0100", "int-0100", "nothing-at-all"} {
		exerciseWhy(t, "-C", root, query)
	}
	after := whySnapshot(t, filepath.Join(diraDir, "entries"))

	if len(before) == 0 {
		t.Fatal("the ledger snapshot is empty; the comparison would pass on an empty directory")
	}
	for name, want := range before {
		got, ok := after[name]
		if !ok {
			t.Errorf("%s is gone after `dira why`", name)
			continue
		}
		if got != want {
			t.Errorf("%s changed after `dira why`", name)
		}
	}
	for name := range after {
		if _, ok := before[name]; !ok {
			t.Errorf("`dira why` created %s", name)
		}
	}
}

// TestAnUnusableCacheStillAnswers is the read-through half of this lane's
// binding constraint: warm from the cache, fall back to the files when there is
// none, and never turn a cache problem into a failed answer.
//
// The answer must also be identical, which it is by construction — everything
// rendered is read from the entry files — but construction is what a test is
// for.
func TestAnUnusableCacheStillAnswers(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which can write to a read-only directory")
	}

	root := repoRoot(t)
	_, warm, _ := exerciseWhy(t, "-C", root, "dec-0002")

	// A copy of the ledger, because the repository's own cache directory is
	// shared with every other test in this package.
	copyRoot := t.TempDir()
	copyDira := filepath.Join(copyRoot, ".dira")
	entries, err := os.ReadDir(filepath.Join(root, ".dira", "entries"))
	if err != nil {
		t.Fatalf("reading the ledger: %v", err)
	}
	files := make(map[string]string, len(entries))
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(root, ".dira", "entries", e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		files[e.Name()] = string(b)
	}
	writeWhyLedger(t, copyDira, files)

	if err := os.Chmod(copyDira, 0o555); err != nil {
		t.Fatalf("making %s read-only: %v", copyDira, err)
	}
	t.Cleanup(func() { _ = os.Chmod(copyDira, 0o755) })

	code, stdout, stderr := exerciseWhy(t, "-C", copyRoot, "dec-0002")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d on an unwritable cache\nstderr: %s", code, exitOK, stderr)
	}
	if stderr == "" {
		t.Error("no notice on stderr; a degraded cache must be stated rather than silent")
	}
	if stdout != warm {
		t.Errorf("the chain differs when the cache cannot be written.\n--- with a cache ---\n%s\n--- without ---\n%s", warm, stdout)
	}
}

// TestWhyRejectsMisuse covers the usage half of the exit-code contract.
func TestWhyRejectsMisuse(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	cases := []struct {
		name string
		args []string
	}{
		{"no argument", []string{"-C", root}},
		{"two arguments", []string{"-C", root, "dec-0002", "dec-0001"}},
		{"unknown flag", []string{"-C", root, "-nope", "dec-0002"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := exerciseWhy(t, tc.args...)
			if code != exitUsage {
				t.Errorf("exit code = %d, want %d\nstderr: %s", code, exitUsage, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty on a usage error", stdout)
			}
			if !strings.Contains(stderr, "dira why") {
				t.Errorf("the usage printed is not this command's:\n%s", stderr)
			}
		})
	}
}

// TestWhyDocumentsItsOwnFlags. `dira why -h` and `dira help why` must show the
// same text, and a flag documented in neither is a flag nobody finds — so the
// help is asserted to name every flag the command actually parses.
func TestWhyDocumentsItsOwnFlags(t *testing.T) {
	t.Parallel()

	code, stdout, stderr := exerciseWhy(t, "-h")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d — help asked for is help, not an error\nstderr: %s", code, exitOK, stderr)
	}
	for _, flagName := range []string{"-C", "-width"} {
		if !strings.Contains(stdout, flagName) {
			t.Errorf("`dira why -h` does not document %s:\n%s", flagName, stdout)
		}
	}
	for _, want := range []string{"usage:", "exit codes:", "dira why dec-0002"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("`dira why -h` is missing %q:\n%s", want, stdout)
		}
	}
}

// TestWidthIsHonoured proves the wrap column is real, in both directions.
func TestWidthIsHonoured(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, width := range []int{why.MinWidth, 64, why.DefaultWidth, 120} {
		for _, tc := range goldenChains {
			t.Run(strconv.Itoa(width)+"/"+tc.name, func(t *testing.T) {
				_, stdout, _ := exerciseWhy(t, "-C", root, "-width", strconv.Itoa(width), tc.query)

				longest := 0
				for _, line := range strings.Split(stdout, "\n") {
					if n := len([]rune(line)); n > longest {
						longest = n
					}
				}
				if longest > width {
					t.Errorf("a line is %d columns wide at -width %d:\n%s", longest, width, stdout)
				}
				// A width honoured but never approached would pass
				// the check above while wrapping at some other column.
				if longest < width/2 {
					t.Errorf("the longest line is %d columns at -width %d; the text is not being laid out to the requested width", longest, width)
				}
			})
		}
	}
}

// TestNarrowWidthsAreRaisedRatherThanRefused pins the floor.
func TestNarrowWidthsAreRaisedRatherThanRefused(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	code, stdout, stderr := exerciseWhy(t, "-C", root, "-width", "5", "dec-0002")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
	}
	if !strings.Contains(stdout, "One file per entry") {
		t.Errorf("an absurd width lost the chain rather than being raised to the floor:\n%s", stdout)
	}
}

// writeWhyLedger materialises a ledger of literal entry files.
//
// Literal files rather than the generated fixture, because these tests are
// about shapes the real ledger does not contain, and a shape asserted against a
// generator is asserted against whatever the generator does next.
func writeWhyLedger(t *testing.T, diraDir string, files map[string]string) {
	t.Helper()

	entries := filepath.Join(diraDir, "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatalf("creating %s: %v", entries, err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(entries, name), []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
}

// whySnapshot reads every file in a directory, so a test can assert nothing in it
// moved.
func whySnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		out[e.Name()] = string(b)
	}
	return out
}
