package installhooks

// E2-L3-T7 — the command-string parser, and its own unit tests. The
// acceptance line that reads the file the installer actually WROTE lives in
// cmd/dira/install_hooks_commands_test.go (TestWrittenCommandsAreReal): the
// command registry (`newApp(...).commands`) is unexported, package main, so
// no internal/ package can enumerate it -- the correction T7's own prose
// names, matching E2-L2-T5's precedent.

import (
	"strings"
	"testing"
)

func TestParseHookCommand(t *testing.T) {
	t.Parallel()

	t.Run("every registered command parses to its own verb and flags", func(t *testing.T) {
		regs, err := Registrations()
		if err != nil {
			t.Fatalf("Registrations: %v", err)
		}
		if len(regs) < 3 {
			t.Fatalf("Registrations() returned %d, want at least 3", len(regs))
		}

		for _, r := range regs {
			parsed, err := ParseHookCommand(r.Command)
			if err != nil {
				t.Errorf("event %s: ParseHookCommand(%q): %v", r.Event, r.Command, err)
				continue
			}
			if parsed.Verb == "" {
				t.Errorf("event %s: empty verb from %q", r.Event, r.Command)
			}
			if !strings.HasPrefix(r.Command, "dira "+parsed.Verb) {
				t.Errorf("event %s: verb %q does not match the command it came from: %q", r.Event, parsed.Verb, r.Command)
			}
			t.Logf("OBSERVED  %s: verb=%q flags=%v", r.Event, parsed.Verb, flagNames(parsed))
		}
	})

	t.Run("the SessionStart command carries --context and --chain", func(t *testing.T) {
		regs, err := Registrations()
		if err != nil {
			t.Fatalf("Registrations: %v", err)
		}
		var sessionStart Registration
		found := false
		for _, r := range regs {
			if r.Event == "SessionStart" {
				sessionStart, found = r, true
			}
		}
		if !found {
			t.Fatal("no SessionStart registration")
		}
		parsed, err := ParseHookCommand(sessionStart.Command)
		if err != nil {
			t.Fatalf("ParseHookCommand: %v", err)
		}
		if parsed.Verb != "brief" {
			t.Errorf("verb = %q, want %q", parsed.Verb, "brief")
		}
		for _, want := range []string{"context", "chain"} {
			if !hasFlag(parsed, want) {
				t.Errorf("flags = %v, missing %q", flagNames(parsed), want)
			}
		}
	})

	t.Run("a missing shell guard is a named error", func(t *testing.T) {
		_, err := ParseHookCommand("dira sniff --stage")
		if err == nil {
			t.Fatal("expected an error for a command with no shell guard")
		}
		if !strings.Contains(err.Error(), "2>/dev/null || true") {
			t.Errorf("error does not name the missing guard: %v", err)
		}
	})

	t.Run("a string that does not begin with dira is a named error, never an empty result", func(t *testing.T) {
		parsed, err := ParseHookCommand("echo hi 2>/dev/null || true")
		if err == nil {
			t.Fatal("expected an error for a command that does not begin with \"dira\"")
		}
		if parsed.Verb != "" || len(parsed.Flags) != 0 {
			t.Errorf("a refused parse still returned a non-empty result: %+v", parsed)
		}
	})

	t.Run("an unbalanced quote is reported rather than guessed at", func(t *testing.T) {
		_, err := ParseHookCommand(`dira sniff --stage "unterminated 2>/dev/null || true`)
		if err == nil {
			t.Fatal("expected an error for an unbalanced quote")
		}
		t.Logf("OBSERVED  %v", err)
	})

	t.Run("dira with no verb at all is a named error", func(t *testing.T) {
		_, err := ParseHookCommand("dira 2>/dev/null || true")
		if err == nil {
			t.Fatal("expected an error for a command naming no verb")
		}
	})
}

func hasFlag(p ParsedCommand, name string) bool {
	for _, f := range p.Flags {
		if f.Name == name {
			return true
		}
	}
	return false
}

func flagNames(p ParsedCommand) []string {
	out := make([]string, 0, len(p.Flags))
	for _, f := range p.Flags {
		out = append(out, f.Name)
	}
	return out
}
