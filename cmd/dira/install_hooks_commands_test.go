package main

// E2-L3-T7's acceptance half that cannot live in internal/installhooks: the
// command registry is newApp(...).commands, unexported and package main, so
// no internal/ package can enumerate it -- the correction E2-L2-T5 already
// made and this task's prose names explicitly. This file reuses probeFlag,
// commandNames, notAFlag and unknownFlagReport from skill_covers_test.go,
// which already do exactly this job for the skill.

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/installhooks"
)

// writtenCommandProblems reads the settings file THE INSTALLER ACTUALLY
// WROTE, not the pinned table -- a check against the table would verify a
// declaration rather than a result (docs/lore.md L-0001). Every command it
// finds must parse, its verb must be a registered command, and every flag
// must resolve against that command's own flag set.
func writtenCommandProblems(data []byte) []string {
	regs, err := installhooks.ParseRegistrations(data)
	if err != nil {
		return []string{"parsing the written settings: " + err.Error()}
	}
	// Non-vacuity first: an extractor that finds nothing satisfies "every
	// extracted command resolves" and looks identical to a clean run -- the
	// exact shape of the five rule-1 instances L-0001 lists.
	if len(regs) < 3 {
		return []string{fmt.Sprintf(
			"extracted %d command(s) from the written file, want at least 3", len(regs))}
	}

	var problems []string
	for _, r := range regs {
		parsed, err := installhooks.ParseHookCommand(r.Command)
		if err != nil {
			problems = append(problems, fmt.Sprintf("event %s: %v", r.Event, err))
			continue
		}
		if !slices.Contains(commandNames(), parsed.Verb) {
			problems = append(problems, fmt.Sprintf(
				"event %s names `dira %s`, which newApp registers no command for; registered: %s",
				r.Event, parsed.Verb, strings.Join(commandNames(), ", ")))
			continue
		}
		// The control, per command: this flag set has to be able to refuse
		// something before its acceptances count -- probeFlag's own
		// two-sided behaviour is pinned by TestFlagProbeTellsDefinedFromUndefined
		// and is not re-proven here.
		if control := probeFlag(parsed.Verb, notAFlag); control == "" {
			problems = append(problems, fmt.Sprintf(
				"event %s: `dira %s` did not refuse the invented flag --%s, so its flags were never checked",
				r.Event, parsed.Verb, notAFlag))
			continue
		}
		for _, f := range parsed.Flags {
			if report := probeFlag(parsed.Verb, f.Name); report != "" {
				problems = append(problems, fmt.Sprintf(
					"event %s: `dira %s %s` is refused by that command's own flag set: %s",
					r.Event, parsed.Verb, f.Text, report))
			}
		}
	}
	return problems
}

// TestWrittenCommandsAreReal is the green half.
func TestWrittenCommandsAreReal(t *testing.T) {
	t.Parallel()

	data := installHooksWrittenFile(t)

	for _, p := range writtenCommandProblems(data) {
		t.Error(p)
	}

	regs, err := installhooks.ParseRegistrations(data)
	if err != nil {
		t.Fatalf("parsing the written file: %v", err)
	}

	// dira brief --context --chain is among the three and passes -- what
	// keeps E1-L5's --chain commitment honest. If it ever fails, that is a
	// contradiction to report, not a reason to drop --chain from the example
	// file to make this green.
	sawChain := false
	for _, r := range regs {
		if r.Event != "SessionStart" {
			continue
		}
		parsed, err := installhooks.ParseHookCommand(r.Command)
		if err == nil && parsed.Verb == "brief" && hasFlagName(parsed, "chain") {
			sawChain = true
		}
	}
	if !sawChain {
		t.Error("`dira brief --context --chain` was not among the written commands and passing")
	}
	t.Logf("OBSERVED  %d command(s) extracted from the written file, all resolved against the real registry", len(regs))
}

// TestWrittenCommandsCanFail is the red half. Each case splices a defect into
// a COPY of a real, installed settings file's bytes and requires
// writtenCommandProblems to name the offending token and its event.
func TestWrittenCommandsCanFail(t *testing.T) {
	t.Parallel()

	real := installHooksWrittenFile(t)
	if problems := writtenCommandProblems(real); len(problems) != 0 {
		t.Fatalf("the untouched written file already reports problem(s); no mutation below would be evidence of anything:\n%s",
			strings.Join(problems, "\n"))
	}
	regs, err := installhooks.ParseRegistrations(real)
	if err != nil {
		t.Fatalf("parsing the written file: %v", err)
	}

	cases := []struct {
		name   string
		event  string
		mutate func(command string) string
		want   string
	}{
		{
			name:  "a bogus flag spliced onto a real command",
			event: "PreCompact",
			mutate: func(c string) string {
				return strings.Replace(c, " 2>/dev/null", " --e2-l3-t7-no-such-flag 2>/dev/null", 1)
			},
			want: "--e2-l3-t7-no-such-flag",
		},
		{
			name:  "a bogus verb spliced onto a real command",
			event: "Stop",
			mutate: func(c string) string {
				return strings.Replace(c, "dira sniff", "dira frobnicate", 1)
			},
			want: "frobnicate",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var original string
			for _, r := range regs {
				if r.Event == tc.event {
					original = r.Command
				}
			}
			if original == "" {
				t.Fatalf("no %s registration in the written file", tc.event)
			}
			mutated := tc.mutate(original)
			if mutated == original {
				t.Fatal("the mutation changed nothing, so a red below would be about the untouched file")
			}
			spliced := strings.Replace(string(real), original, mutated, 1)
			if spliced == string(real) {
				t.Fatal("splicing the mutated command into the written bytes changed nothing")
			}

			problems := writtenCommandProblems([]byte(spliced))
			if len(problems) == 0 {
				t.Fatalf("the check accepted a written file carrying %q", tc.want)
			}
			report := strings.Join(problems, "\n")
			if !strings.Contains(report, tc.want) {
				t.Fatalf("the report never names %q:\n%s", tc.want, report)
			}
			if !strings.Contains(report, tc.event) {
				t.Fatalf("the report never names the event %q:\n%s", tc.event, report)
			}
			t.Logf("OBSERVED  red on %s: %s", tc.name, firstLineContaining(report, tc.want))
		})
	}
}

// installHooksWrittenFile installs into a fresh temp root and returns exactly
// the bytes on disk -- the file the installer wrote, read back rather than
// assumed.
func installHooksWrittenFile(t *testing.T) []byte {
	t.Helper()

	dir := t.TempDir()
	if code, _, stderr := runHooks(t, "--dir", dir); code != exitOK {
		t.Fatalf("fixture setup: install exit %d (stderr: %s)", code, stderr)
	}
	data, err := os.ReadFile(filepath.Join(dir, installHooksUserFile))
	if err != nil {
		t.Fatalf("reading the written file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("the written file is empty")
	}
	return data
}

func hasFlagName(p installhooks.ParsedCommand, name string) bool {
	for _, f := range p.Flags {
		if f.Name == name {
			return true
		}
	}
	return false
}

func firstLineContaining(report, want string) string {
	for _, line := range strings.Split(report, "\n") {
		if strings.Contains(line, want) {
			return line
		}
	}
	return ""
}
