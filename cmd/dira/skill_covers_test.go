package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/skill/skilltest"

	"github.com/kazi-org/dira/internal/skill"
)

// E2-L2-T5's acceptance lives here rather than in internal/skill because the
// command set is `newApp(...).commands` — unexported, package main — and every
// command's flag surface is built inside its own run function. A check in
// internal/skill could only re-list the verbs in a slice of its own, which would
// assert that the list matches itself: a declaration checked against a
// declaration, and the defect docs/lore.md L-0001 closes on. The extractor is
// internal/skill's; the registry is this package's; this file joins them.
//
// kazi already paid for the failure this prevents
// (test/kazi/teach/skill_covers_commands_test.exs). A skill that tells a model
// to run a verb the binary does not have produces no error anyone sees — the
// model runs it, the shell reports exit 2 into a hook that fails open, and the
// capture that was supposed to happen simply did not. The symptom, three weeks
// later, is a ledger with a hole in it that reads like a session where nothing
// was decided.

// notAFlag is a flag name no command defines. It is the control: before any
// flag from the skill is believed to be real, the same probe is run with this
// one, and it has to come back refused. A probe that cannot produce a red is
// indistinguishable from a flag surface that accepts everything.
const notAFlag = "e2-l2-t5-no-such-flag"

// unknownFlagReport is the stdlib flag package's own words for a flag nobody
// defined. Matching on it rather than on an exit code is deliberate: `dira log
// --stdin` and `dira log --nonsense` both exit 2, and only the message
// distinguishes "you used this wrongly" from "this does not exist".
const unknownFlagReport = "flag provided but not defined: -"

// TestSkillCoversCommands is the green half: every `dira …` the shipped skill
// names is a command the registry has, with flags that command's own flag set
// accepts.
func TestSkillCoversCommands(t *testing.T) {
	t.Parallel()

	path, invocations := readSkillInvocations(t)

	// The registry, read rather than restated. An empty one would make
	// every lookup below fail, so it is checked before it is trusted.
	registered := newApp(nil, nil).commands
	if len(registered) == 0 {
		t.Fatal("newApp registers no commands, so nothing below is a check")
	}

	for _, problem := range skillProblems(invocations) {
		t.Errorf("%s", problem)
	}

	t.Logf("OBSERVED  %s: %d invocations against %d registered commands (%s)",
		path, len(invocations), len(registered), strings.Join(commandNames(), ", "))
	for _, inv := range invocations {
		t.Logf("OBSERVED  line %3d  dira %s  flags: %v", inv.Line, inv.Command, flagList(inv))
	}
}

// TestSkillCoverageCanFail is the other half, and the reason the green above is
// evidence rather than a green light with nothing behind it.
//
// Each case splices one defect into a copy of the real skill and requires the
// check to name the offending token. The last two cases are the vacuity ones:
// an extractor that finds nothing satisfies every "all of them resolve" clause
// there is, which is the exact shape of the rule-1 instances docs/lore.md
// L-0001 lists.
func TestSkillCoverageCanFail(t *testing.T) {
	t.Parallel()

	_, realInvocations := readSkillInvocations(t)
	real := skillSource(t)

	cases := []struct {
		name string
		src  string
		want string // a substring the report must carry
		deny string // a substring the report must not carry
	}{
		{
			name: "an invented verb",
			src:  real + "\n```\ndira frobnicate --stage\n```\n",
			want: "frobnicate",
		},
		{
			name: "an invented flag on a real command",
			src:  real + "\n```\ndira log --magic\n```\n",
			want: "--magic",
		},
		{
			name: "an invented flag inside an otherwise real invocation",
			src:  strings.Replace(real, "dira log --stdin", "dira log --stdin --magic", 1),
			want: "--magic",
		},
		{
			name: "a flag that belongs to a different command",
			src:  real + "\n```\ndira check --stdin \"a plan\"\n```\n",
			want: "--stdin",
		},
		{
			name: "an empty document",
			src:  "",
			want: "found no",
		},
		{
			// The document that would break a bare `\bdira\s+\w+`
			// scan. It contains the word twice and names no command,
			// so the honest report is the vacuity one — and it must
			// not be a complaint about a verb called `handoff`.
			name: "a document carrying only the handoff marker",
			src:  "```\n=== dira handoff, tier 2 ===\ndec-0060 dec-0061\n=== end dira handoff ===\n```\n",
			want: "found no",
			deny: "handoff",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.src == real {
				t.Fatal("the splice changed nothing, so a red below would be about the real file")
			}

			invocations, err := skill.Extract(tc.src)
			if err != nil {
				t.Fatalf("extracting the spliced document: %v", err)
			}
			problems := skillProblems(invocations)
			if len(problems) == 0 {
				t.Fatalf("the check accepted a skill carrying exactly the defect it names")
			}
			report := strings.Join(problems, "\n")
			if !strings.Contains(report, tc.want) {
				t.Fatalf("the report never names %q, so it does not say what is wrong:\n%s", tc.want, report)
			}
			if tc.deny != "" && strings.Contains(report, tc.deny) {
				t.Fatalf("the report names %q, which is not a command in this document:\n%s", tc.deny, report)
			}
			t.Logf("OBSERVED  rejected: %s", strings.SplitN(report, "\n", 2)[0])

			// And the same check still accepts the untouched skill,
			// so a checker that refuses everything is not mistaken
			// for one that works.
			if problems := skillProblems(realInvocations); len(problems) != 0 {
				t.Fatalf("the check also rejects the real skill, so its red above means nothing:\n%s",
					strings.Join(problems, "\n"))
			}
		})
	}
}

// TestFlagProbeTellsDefinedFromUndefined pins the primitive everything above
// rests on, in both directions and against every command the skill names.
//
// The probe's whole claim is that it can distinguish a flag the command defines
// from one it does not. If it could not — if, say, a command returned before
// parsing — every flag in the skill would come back clean and the test would be
// a very confident no-op.
func TestFlagProbeTellsDefinedFromUndefined(t *testing.T) {
	t.Parallel()

	cases := []struct {
		command string
		defined string
	}{
		{command: "log", defined: "kind"},
		{command: "log", defined: "stdin"},
		{command: "check", defined: "json"},
		{command: "why", defined: "width"},
		{command: "sniff", defined: "deep"},
		{command: "brief", defined: "C"},
		{command: "supersede", defined: "C"},
	}

	for _, tc := range cases {
		t.Run(tc.command+"/"+tc.defined, func(t *testing.T) {
			t.Parallel()

			if report := probeFlag(tc.command, tc.defined); report != "" {
				t.Errorf("the probe reports `dira %s --%s` as undefined, but the command defines it: %s",
					tc.command, tc.defined, report)
			}
			report := probeFlag(tc.command, notAFlag)
			if report == "" {
				t.Fatalf("the probe reports nothing for `dira %s --%s`, so it cannot produce a red at all",
					tc.command, notAFlag)
			}
			t.Logf("OBSERVED  --%s accepted, --%s refused: %s", tc.defined, notAFlag, report)
		})
	}
}

// ---- the check -------------------------------------------------------------

// skillProblems reports everything wrong with a set of extracted invocations,
// as lines a reader can act on, and returns nothing at all when they are clean.
//
// It is a function returning findings rather than a body full of t.Errorf so
// that the same code produces both the verdict on the real file and the
// rejections in TestSkillCoverageCanFail. A red proven by a different code path
// than the green proves nothing about the green.
func skillProblems(invocations []skill.Invocation) []string {
	// Non-vacuity first, and it is a full stop rather than one finding
	// among several: every clause after this one is a statement about each
	// member of a set, and all of them are true of the empty set.
	if len(invocations) == 0 {
		return []string{"the extractor found no `dira …` invocation in the skill; " +
			"either the document stopped naming any command, or the extractor stopped matching — " +
			"and both look identical to a skill that is entirely correct"}
	}

	var problems []string
	for _, want := range []string{"log", "check"} {
		if !namesCommand(invocations, want) {
			problems = append(problems, fmt.Sprintf(
				"the skill names no `dira %s` invocation; the tier-2 loop cannot work without it (found: %s)",
				want, strings.Join(shapes(invocations), " | ")))
		}
	}

	registry := newApp(nil, nil)
	for _, inv := range invocations {
		if inv.Command != "" && registry.lookup(inv.Command) == nil {
			problems = append(problems, fmt.Sprintf(
				"SKILL.md:%d names `dira %s`, which newApp registers no command for; registered: %s",
				inv.Line, inv.Command, strings.Join(commandNames(), ", ")))
			continue
		}
		if len(inv.Flags) == 0 {
			continue
		}
		// The control, per command and per invocation: this flag set has
		// to be able to refuse something before its acceptances count.
		if control := probeFlag(inv.Command, notAFlag); control == "" {
			problems = append(problems, fmt.Sprintf(
				"SKILL.md:%d — `dira %s` did not refuse the invented flag --%s, so it cannot tell a real flag "+
					"from an invented one and the %d flags on this line were never checked",
				inv.Line, inv.Command, notAFlag, len(inv.Flags)))
			continue
		}
		for _, f := range inv.Flags {
			if report := probeFlag(inv.Command, f.Name); report != "" {
				problems = append(problems, fmt.Sprintf(
					"SKILL.md:%d names `dira %s %s`, and that command's own flag set refuses it: %s",
					inv.Line, inv.Command, f.Text, report))
			}
		}
	}
	return problems
}

// probeFlag asks the real binary whether a command defines a flag, and returns
// the binary's own complaint when it does not.
//
// It runs the actual registry — `newApp(...).main` — rather than reaching for a
// flag set, because no flag set is reachable: most commands build theirs inside
// their run function. What is reachable is the answer, which is the thing worth
// asserting anyway.
//
// The two arguments are chosen so the call is inert. `--name=value` keeps a bool
// flag from leaving its value behind as a positional argument, which would stop
// flag parsing early and skip the rest of the line; and a trailing `-h` stops
// parsing at help — after every flag before it has been looked up, and before
// the command opens a ledger, reads stdin or writes a file.
func probeFlag(command, name string) string {
	args := make([]string, 0, 3)
	if command != "" {
		args = append(args, command)
	}
	args = append(args, "--"+name+"=true", "-h")

	var stdout, stderr bytes.Buffer
	newApp(&stdout, &stderr).main(args)

	for _, line := range strings.Split(stderr.String(), "\n") {
		if strings.Contains(line, unknownFlagReport+name) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// ---- helpers ---------------------------------------------------------------

// readSkillInvocations locates and extracts the shipped skill, failing loudly
// rather than returning an empty set that would pass everything.
func readSkillInvocations(t *testing.T) (string, []skill.Invocation) {
	t.Helper()

	path, err := skilltest.Locate()
	if err != nil {
		t.Fatalf("locating the skill: %v", err)
	}
	text, err := skilltest.ReadSkill()
	if err != nil {
		t.Fatalf("locating the skill: %v", err)
	}
	invocations, err := skill.Extract(text)
	if err != nil {
		t.Fatalf("extracting the skill's invocations: %v", err)
	}
	return path, invocations
}

// skillSource reads the shipped skill verbatim, for the splices to build on.
func skillSource(t *testing.T) string {
	t.Helper()

	path, err := skilltest.Locate()
	if err != nil {
		t.Fatalf("locating the skill: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s is empty", path)
	}
	return string(data)
}

func namesCommand(invocations []skill.Invocation, command string) bool {
	for _, inv := range invocations {
		if inv.Command == command {
			return true
		}
	}
	return false
}

// shapes renders invocations as "command --flag --flag", for a report.
func shapes(invocations []skill.Invocation) []string {
	var out []string
	for _, inv := range invocations {
		out = append(out, strings.TrimSpace(inv.Command+" "+strings.Join(flagList(inv), " ")))
	}
	return out
}

func flagList(inv skill.Invocation) []string {
	var out []string
	for _, f := range inv.Flags {
		out = append(out, "--"+f.Name)
	}
	return out
}

// commandNames is the registry's names, read from the registry.
func commandNames() []string {
	var out []string
	for _, c := range newApp(nil, nil).commands {
		out = append(out, c.name)
	}
	return out
}
