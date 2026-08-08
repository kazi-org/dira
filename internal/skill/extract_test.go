package skill

import (
	"os"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/skill/skilltest"
)

// TestExtractInvocations is the extractor half of E2-L2-T5's acceptance.
//
// It asks two questions of the extractor, and the second one is the one that
// gets skipped everywhere else. Does it find the invocations the skill names —
// and does it leave alone the things in the same document that only look like
// invocations? An extractor that answers the first question by matching
// `\bdira\s+\w+` reads the handoff block's own delimiter, `=== dira handoff,
// tier 2 ===`, as the verb `dira handoff`, and then reports a correct skill as
// broken. That is docs/lore.md L-0001 rule 2, and the fixtures below carry it
// rather than trusting a comment about it.
func TestExtractInvocations(t *testing.T) {
	t.Parallel()

	path, err := skilltest.Locate()
	if err != nil {
		t.Fatalf("locating the skill: %v", err)
	}
	text, err := skilltest.ReadSkill()
	if err != nil {
		t.Fatalf("locating the skill: %v", err)
	}
	invocations, err := Extract(text)
	if err != nil {
		t.Fatalf("extracting the shipped skill: %v", err)
	}

	// --- non-vacuity, before anything else -------------------------------
	//
	// Every clause below this point is a statement about a set, and all of
	// them hold of the empty set. An extractor whose regex stopped matching
	// finds nothing and passes the lot, which is the failure this whole
	// package exists to make loud, so the count is fatal and comes first.
	if len(invocations) == 0 {
		t.Fatalf("extracted no invocations from %s; every assertion below would pass vacuously", path)
	}
	for _, want := range []string{"log", "check"} {
		if !namesCommand(invocations, want) {
			t.Fatalf("extracted %d invocations from %s but none names `dira %s`: %s",
				len(invocations), path, want, summarize(invocations))
		}
	}
	t.Logf("OBSERVED  %d invocations in %s: %s", len(invocations), path, summarize(invocations))

	// --- the thing that is not an invocation -----------------------------
	//
	// The skill quotes the handoff marker inside a fenced block. It is in
	// the file, it contains the word, and it is not a command.
	raw := mustRead(t, path)
	if !strings.Contains(raw, "=== dira handoff, tier 2 ===") {
		t.Fatal("the skill no longer quotes the handoff marker, so the assertion below proves nothing")
	}
	if namesCommand(invocations, "handoff") {
		t.Errorf("the handoff block's own delimiter was read as the verb `dira handoff`: %s", summarize(invocations))
	}

	// --- the flags, on the invocation that carries them -------------------
	extraction := findInvocation(invocations, "log", "--why-not")
	if extraction == nil {
		t.Fatalf("no `dira log` invocation carries --why-not; the extraction example is what T5 exists to check: %s",
			summarize(invocations))
	}
	wantFlags := []string{
		"kind", "state", "tier", "hook", "title", "body",
		"alternative", "why-not", "edge", "edge-note", "excerpt",
	}
	if got := flagNames(extraction.Flags); !equal(got, wantFlags) {
		t.Errorf("line %d flags = %v, want %v", extraction.Line, got, wantFlags)
	} else {
		t.Logf("OBSERVED  line %d names %d flags: %v", extraction.Line, len(got), got)
	}

	// A quoted body is one token however many words and hyphens it holds.
	// If it were split, `--why-not 'It violates the single-binary intent…'`
	// would contribute nothing wrong, but a body opening with a hyphen would
	// arrive as a flag nobody defined.
	for _, f := range extraction.Flags {
		if strings.Contains(f.Name, " ") {
			t.Errorf("flag %q carries a space, so tokenisation folded a value into a name", f.Name)
		}
	}
}

// TestExtractRules is the extractor against constructed documents, one per rule.
//
// Each case states what the extractor must find and what it must not, because
// the two failure directions cost different things: a missed invocation is a
// check that silently stops checking, and an invented one is a red result on a
// correct file.
func TestExtractRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		src      string
		want     []string // "command" or "command --flag --flag"
		wantErr  bool
		wantLine int // 0 to skip
	}{
		{
			name: "a fenced command line",
			src:  "text\n\n```\ndira check \"a plan\"\n```\n",
			want: []string{"check"},
			// The fence opens on line 3, so the command is on line 4.
			wantLine: 4,
		},
		{
			name: "the handoff marker is not a verb",
			src:  "```\n=== dira handoff, tier 2 ===\ndec-0060\n=== end dira handoff ===\n```\n",
			want: nil,
		},
		{
			name: "prose naming the product is not a verb",
			src:  "dira, the git-native ledger of decisions, stages what it finds.\nA dira handoff block appears in context.\n",
			want: nil,
		},
		{
			name: "a backslash continuation is one invocation",
			src:  "```\ndira log --kind decision \\\n  --title 'a title' \\\n  --why-not 'because'\n```\n",
			want: []string{"log --kind --title --why-not"},
		},
		{
			name: "a quoted argument is one token",
			src:  "```\ndira check \"revise the plan --with something\"\n```\n",
			want: []string{"check"},
		},
		{
			name: "a quoted value opening with a hyphen is not a flag",
			src:  "```\ndira log --kind decision --body '-'\n```\n",
			want: []string{"log --kind --body"},
		},
		{
			name: "a flag whose quoted value opens with a hyphen is still a flag",
			src:  "```\ndira log --kind decision --body='-'\n```\n",
			want: []string{"log --kind --body"},
		},
		{
			name: "an inline code span is an invocation",
			src:  "The pair is a graph: `dira why dec-0060` renders both.\n",
			want: []string{"why"},
		},
		{
			name: "a bare mention of the binary is not an invocation",
			src:  "`dira` is the binary, and `--tier semantic` is a flag.\n",
			want: nil,
		},
		{
			name: "a neighbouring command in the same block is left alone",
			src:  "```\ngrep -l 'tier: regex' .dira/entries/dec-*.md\nkazi approve prop-resume-8a1f\ndira log --stdin\n```\n",
			want: []string{"log --stdin"},
		},
		{
			name: "an invented verb is extracted so the registry can refuse it",
			src:  "```\ndira frobnicate --stage\n```\n",
			want: []string{"frobnicate --stage"},
		},
		{
			name: "an indented command line is still a command line",
			src:  "```\n    dira reindex\n```\n",
			want: []string{"reindex"},
		},
		{
			name:    "an unbalanced quote is reported rather than guessed at",
			src:     "```\ndira check \"a plan that never closes\n```\n",
			wantErr: true,
		},
		{
			name: "a fence that is never closed still yields its commands",
			src:  "```\ndira brief\n",
			want: []string{"brief"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := Extract(tc.src)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want an error, got %s", summarize(got))
				}
				t.Logf("OBSERVED  reported: %v", err)
				return
			}
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			if !equal(shapes(got), tc.want) {
				t.Fatalf("extracted %v, want %v", shapes(got), tc.want)
			}
			if tc.wantLine != 0 {
				if got[0].Line != tc.wantLine {
					t.Errorf("line = %d, want %d", got[0].Line, tc.wantLine)
				}
			}
		})
	}
}

// ---- helpers ---------------------------------------------------------------

// shapes renders each invocation as "command --flag --flag", which is the whole
// of what the registry check reads and therefore the whole of what these cases
// need to pin.
func shapes(invocations []Invocation) []string {
	var out []string
	for _, inv := range invocations {
		parts := []string{inv.Command}
		for _, f := range inv.Flags {
			parts = append(parts, "--"+f.Name)
		}
		out = append(out, strings.Join(parts, " "))
	}
	return out
}

// summarize is shapes, for a failure message.
func summarize(invocations []Invocation) string {
	if len(invocations) == 0 {
		return "(none)"
	}
	return "[" + strings.Join(shapes(invocations), " | ") + "]"
}

// mustRead reads a file the test cannot continue without.
func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func namesCommand(invocations []Invocation, command string) bool {
	for _, inv := range invocations {
		if inv.Command == command {
			return true
		}
	}
	return false
}

// findInvocation returns the first invocation naming command and flag.
func findInvocation(invocations []Invocation, command, flag string) *Invocation {
	for i, inv := range invocations {
		if inv.Command != command {
			continue
		}
		for _, f := range inv.Flags {
			if f.Text == flag {
				return &invocations[i]
			}
		}
	}
	return nil
}

func flagNames(flags []Flag) []string {
	var out []string
	for _, f := range flags {
		out = append(out, f.Name)
	}
	return out
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
