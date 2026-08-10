package installhooks

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/kazi-org/dira/hooks"
)

// wantExample is what hooks/settings.example.json says, written out literally.
//
// A literal is the wrong tool almost everywhere in this lane — the registrations
// are derived from the embedded file precisely so that nobody has to keep a
// second copy of them correct. This is the one place it is the right tool: the
// three command strings are the contract between this repository and a Claude
// Code session, and an edit to any of them should have to be made twice, here
// and in the file, by somebody who read this test's failure message first.
//
// The failure messages below are therefore not "want X got Y". Each one says
// what the string is for, because the reader of the failure is somebody who is
// midway through changing it.
var wantExample = []Registration{
	{
		Event:       "SessionStart",
		Command:     "dira brief --context --chain 2>/dev/null || true",
		Timeout:     5,
		OwnerPrefix: "dira brief",
	},
	{
		Event:       "Stop",
		Command:     "dira sniff --stage --quiet 2>/dev/null || true",
		Timeout:     10,
		OwnerPrefix: "dira sniff --stage",
	},
	{
		Event:       "PreCompact",
		Command:     "dira sniff --deep --stage --all 2>/dev/null || true",
		Timeout:     60,
		OwnerPrefix: "dira sniff --deep",
	},
}

// maxTimeout is the longest any installed hook may block a session for. It is
// PreCompact's, which is the one worth waiting for and the one that fires
// rarely; nothing may exceed it.
const maxTimeout = 60

func TestRegistrations(t *testing.T) {
	t.Run("the shipped example is the registrations", func(t *testing.T) {
		regs, err := Registrations()
		if err != nil {
			t.Fatalf("parsing the embedded hooks/settings.example.json: %v", err)
		}
		checkRegistrations(t.Errorf, regs)
	})

	// The red half of docs/lore.md L-0001. The subject of this test is a file
	// compiled into the binary, so the only way to observe the assertions
	// failing is to point them at a deliberately broken copy of it. Each case
	// below is a real regression this lane can suffer, not an invented one.
	t.Run("the assertions catch a changed example", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			from, to string
			// mentions are substrings the reported failures must contain,
			// so a case cannot be satisfied by some unrelated assertion
			// happening to fail.
			mentions []string
		}{
			{
				name:     "the stale prompt's command, without --chain",
				from:     "dira brief --context --chain 2>/dev/null",
				to:       "dira brief --context 2>/dev/null",
				mentions: []string{"SessionStart", "--chain"},
			},
			{
				name:     "PreCompact without --all, the regression that already happened once",
				from:     "dira sniff --deep --stage --all 2>/dev/null",
				to:       "dira sniff --deep --stage 2>/dev/null",
				mentions: []string{"PreCompact", "--all", "3 staged entries with it against 2 without"},
			},
			{
				name:     "the fail-open guard removed from the Stop command",
				from:     "dira sniff --stage --quiet 2>/dev/null || true",
				to:       "dira sniff --stage --quiet 2>/dev/null",
				mentions: []string{"Stop", "|| true"},
			},
			{
				name:     "a timeout raised past the bound",
				from:     `"timeout": 60`,
				to:       `"timeout": 600`,
				mentions: []string{"PreCompact", "600"},
			},
			{
				name:     "a command whose first word is not dira",
				from:     "dira brief --context --chain 2>/dev/null",
				to:       "sh -c dira brief --context --chain 2>/dev/null",
				mentions: []string{"SessionStart", `starts with "sh"`},
			},
			{
				name: "flags reordered so the owner prefix no longer matches",
				from: "dira sniff --deep --stage --all 2>/dev/null",
				to:   "dira sniff --stage --deep --all 2>/dev/null",
				// The command still does the same thing; what breaks is
				// dira's ability to find this entry again in somebody's
				// settings file. That has to fail here rather than there.
				mentions: []string{"PreCompact", "dira sniff --deep"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				regs, err := ParseRegistrations(mutateExample(t, tc.from, tc.to))
				if err != nil {
					t.Fatalf("the mutated example no longer parses (%v), so this case proves nothing about the assertions", err)
				}
				var got assertionFailures
				checkRegistrations(got.report, regs)

				if len(got.msgs) == 0 {
					t.Fatalf("no assertion failed against %q -> %q.\n"+
						"The example file could be changed this way and this test would stay green.", tc.from, tc.to)
				}
				joined := got.joined()
				for _, want := range tc.mentions {
					if !strings.Contains(joined, want) {
						t.Errorf("the failures never mention %q, so the assertion that fired was not the one this case is about.\nreported:\n%s", want, joined)
					}
				}
			})
		}

		// The vacuous case, which is L-0001 rule 1 rather than rule 2: every
		// "every command ..." assertion above ranges over the registrations,
		// and every one of them holds of no registrations at all.
		t.Run("no registrations at all", func(t *testing.T) {
			var got assertionFailures
			checkRegistrations(got.report, nil)
			if len(got.msgs) == 0 {
				t.Fatal("an empty set of registrations satisfied every assertion, so a parser that found nothing would look identical to a clean run")
			}
		})
	})

	// The parser refuses rather than guesses. Each case is an edit somebody
	// could plausibly make to the example file, and each one has to come back
	// as a named error with no registrations — a partial answer would be
	// installed unread.
	t.Run("the parser refuses what it cannot read", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			settings []byte
			want     error
		}{
			{
				name:     "a missing timeout",
				settings: mutateExample(t, ",\n            \"timeout\": 5", ""),
				want:     ErrShape,
			},
			{
				name:     "a missing command",
				settings: mutateExample(t, "\"command\": \"dira sniff --stage --quiet 2>/dev/null || true\",\n            ", ""),
				want:     ErrShape,
			},
			{
				name: "two commands under one entry",
				settings: mutateExample(t,
					"            \"timeout\": 5\n          }\n",
					"            \"timeout\": 5\n          },\n          {\n            \"type\": \"command\",\n            \"command\": \"dira brief --context 2>/dev/null || true\",\n            \"timeout\": 5\n          }\n"),
				want: ErrShape,
			},
			{
				name:     "a fourth event",
				settings: mutateExample(t, "  \"hooks\": {\n    \"SessionStart\"", "  \"hooks\": {\n    \"PostToolUse\": [],\n    \"SessionStart\""),
				want:     ErrUnknownEvent,
			},
			{
				name:     "an event renamed to one dira does not own",
				settings: mutateExample(t, "\"Stop\": [", "\"SessionEnd\": ["),
				want:     ErrUnknownEvent,
			},
			{
				name:     "an entry that is not a command",
				settings: mutateExample(t, "\"type\": \"command\",\n            \"command\": \"dira brief", "\"type\": \"prompt\",\n            \"command\": \"dira brief"),
				want:     ErrShape,
			},
			{
				name:     "malformed bytes",
				settings: []byte(`{"hooks": {`),
				want:     ErrMalformed,
			},
			{
				name:     "a non-object root",
				settings: []byte(`[]`),
				want:     ErrShape,
			},
			{
				name:     "a non-object hooks value",
				settings: []byte(`{"hooks": []}`),
				want:     ErrShape,
			},
			{
				name:     "no hooks at all",
				settings: []byte(`{"//": ["documentation and nothing else"]}`),
				want:     ErrShape,
			},
			{
				name:     "an event dira registers is absent",
				settings: []byte(`{"hooks": {"SessionStart": [{"hooks": [{"type": "command", "command": "dira brief --context --chain 2>/dev/null || true", "timeout": 5}]}]}}`),
				want:     ErrMissingEvent,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				regs, err := ParseRegistrations(tc.settings)
				if !errors.Is(err, tc.want) {
					t.Errorf("error is %v, want one wrapping %v", err, tc.want)
				}
				if regs != nil {
					t.Errorf("returned %d registrations alongside the error; a partial answer here is installed unread: %+v", len(regs), regs)
				}
			})
		}
	})

	// The other side of that refusal: a `//` documentation key is this
	// repository's own convention at every level of the example file, and one
	// appearing inside "hooks" must not be mistaken for a fourth event.
	t.Run("a documentation key inside hooks is not an event", func(t *testing.T) {
		settings := mutateExample(t,
			"  \"hooks\": {\n    \"SessionStart\"",
			"  \"hooks\": {\n    \"//\": [\"the same documentation convention, one level down\"],\n    \"SessionStart\"")
		regs, err := ParseRegistrations(settings)
		if err != nil {
			t.Fatalf("parsing: %v", err)
		}
		checkRegistrations(t.Errorf, regs)
	})

	// cst-0004 and int-0002: the bytes are compiled in, so nothing here needs
	// a checkout, a working directory or a network.
	t.Run("the bytes are compiled in, not read from the checkout", func(t *testing.T) {
		t.Chdir(t.TempDir())

		// The control. Without it this subtest would pass just as happily
		// having never left the package directory, which is where the file
		// it must not be reading actually is.
		if _, err := os.ReadFile("hooks/settings.example.json"); err == nil {
			t.Fatal("hooks/settings.example.json is readable from this working directory, so the chdir proved nothing")
		}

		regs, err := ParseRegistrations(hooks.SettingsExample)
		if err != nil {
			t.Fatalf("parsing the embedded example from a temp directory: %v", err)
		}
		checkRegistrations(t.Errorf, regs)
	})

	// dec-0005, asserted in the package that would break it as well as in
	// internal/ledger. A path in here is a path the E7 backend would have to
	// remove from above the storage interface.
	t.Run("nothing that ships names a path", func(t *testing.T) {
		const (
			hooksPkg = "github.com/kazi-org/dira/hooks"
			thisPkg  = "github.com/kazi-org/dira/internal/installhooks"
		)
		banned := []string{"os", "io/fs", "io/ioutil", "path", "path/filepath"}

		goBin, err := exec.LookPath("go")
		if err != nil {
			t.Skipf("no go toolchain on PATH: %v", err)
		}
		// .Imports is non-test files only, which is the distinction that
		// matters: this test file itself imports os and os/exec, and must.
		out, err := exec.Command(goBin, "list",
			"-f", `{{.ImportPath}}{{"\t"}}{{join .Imports " "}}`,
			hooksPkg, thisPkg).CombinedOutput()
		if err != nil {
			t.Fatalf("go list: %v\n%s", err, out)
		}

		imports := map[string][]string{}
		for _, line := range strings.Split(string(out), "\n") {
			path, joined, ok := strings.Cut(line, "\t")
			if !ok {
				continue
			}
			imports[path] = strings.Fields(joined)
		}
		for _, pkg := range []string{hooksPkg, thisPkg} {
			if _, ok := imports[pkg]; !ok {
				t.Fatalf("go list did not report %s, so this check examined nothing:\n%s", pkg, out)
			}
		}
		// Teeth: the embed package has to import something, and if the
		// listing were empty every assertion below would hold trivially.
		if !slices.Contains(imports[hooksPkg], "embed") {
			t.Errorf("%s does not import embed; either the file is no longer embedded or the listing is wrong, and in both cases this check is measuring nothing", hooksPkg)
		}

		for pkg, list := range imports {
			for _, imported := range list {
				if slices.Contains(banned, imported) {
					t.Errorf("%s imports %q. Deciding what to install is policy over bytes; opening a file is cmd/dira's (dec-0005), and internal/installhooks is on nobody's allowlist in internal/ledger/boundary_test.go.", pkg, imported)
				}
			}
		}
	})
}

// checkRegistrations is every assertion this test makes about a set of
// registrations, reporting through report rather than through *testing.T.
//
// The indirection is the point. The registrations are derived from bytes
// compiled into the binary, so there is no way to observe these assertions
// failing except by pointing them at a deliberately broken copy of the example
// — which is what the "the assertions catch a changed example" subtest does
// with the same function the green run uses. A gate is evidence only once both
// sides have been observed (docs/lore.md L-0001), and rule 2 — green on the
// untouched baseline — is the one that is usually skipped.
func checkRegistrations(report func(format string, args ...any), regs []Registration) {
	if len(regs) == 0 {
		report("no registrations at all. Every assertion below ranges over them and every one holds vacuously of an empty set, so a parser that found nothing would be indistinguishable from a clean run (docs/lore.md L-0001 rule 1)")
		return
	}
	if len(regs) != len(wantExample) {
		report("%d registrations, want exactly %d. hooks/settings.example.json installs %d hooks and this lane installs what that file says",
			len(regs), len(wantExample), len(wantExample))
	}

	// The literals, in the file's own order.
	for i, want := range wantExample {
		if i >= len(regs) {
			report("registration %d (%s) is missing entirely", i, want.Event)
			continue
		}
		got := regs[i]
		if got.Event != want.Event {
			report("registration %d fires on %q, want %q. The order is the example file's own, and dec-0023 decided which hook carries what: PreCompact is where the transcript still exists, SessionStart is where context reaches the model",
				i, got.Event, want.Event)
		}
		if got.Command != want.Command {
			report("the %s command is\n  %q\nwant\n  %q\nThis string is the contract between this repository and a Claude Code session. Changing it means changing hooks/settings.example.json and this line together, deliberately",
				want.Event, got.Command, want.Command)
		}
		if got.Timeout != want.Timeout {
			report("the %s timeout is %d, want %d. This is the only bound on a dira that hangs — the shell guard cannot catch one",
				want.Event, got.Timeout, want.Timeout)
		}
	}

	// The properties, asserted of every registration rather than of the three
	// named ones, so a fourth added later is held to the same bar.
	for _, reg := range regs {
		if verb, _, _ := strings.Cut(reg.Command, " "); verb != "dira" {
			report("the %s command starts with %q, not \"dira\". Claude Code runs this string through a shell; anything else here is this repository installing somebody else's program",
				reg.Event, verb)
		}
		for _, want := range []string{"2>/dev/null", "|| true"} {
			if !strings.Contains(reg.Command, want) {
				report("the %s command %q does not contain %q. A dira failure must never block a session: %q drops the noise and %q neutralises the exit status, and without both a broken dira is a broken session",
					reg.Event, reg.Command, want, "2>/dev/null", "|| true")
			}
		}
		if reg.Timeout <= 0 || reg.Timeout > maxTimeout {
			report("the %s timeout is %d, want a positive number of seconds no greater than %d. A hook Claude Code waits on for longer than that is a hook an operator notices",
				reg.Event, reg.Timeout, maxTimeout)
		}
		if reg.OwnerPrefix == "" {
			report("the %s registration declares no owner prefix, so dira could only recognise its own entry by position or by the whole command string — and both orphan the entry the next time the command grows an argument",
				reg.Event)
		} else if !strings.HasPrefix(reg.Command, reg.OwnerPrefix) {
			report("the %s command\n  %q\ndoes not start with the prefix dira claims its entry by\n  %q\nAn installed entry would stop being recognised: install would add a second one beside it and the session would run it twice",
				reg.Event, reg.Command, reg.OwnerPrefix)
		}
	}

	// --all by name, because it is the one flag here with a measured cost.
	found := false
	for _, reg := range regs {
		if reg.Event != "PreCompact" {
			continue
		}
		found = true
		if !strings.Contains(reg.Command, "--all") {
			report("the PreCompact command %q does not contain --all. It was added after the installed command was measured losing a third of its captures — 3 staged entries with it against 2 without, on the same test transcript",
				reg.Command)
		}
	}
	if !found {
		report("no PreCompact registration. PreCompact is where the about-to-be-compacted transcript still exists, which is why the capture is installed there (dec-0023)")
	}
}

// assertionFailures records what checkRegistrations reported.
type assertionFailures struct {
	msgs []string
}

func (f *assertionFailures) report(format string, args ...any) {
	f.msgs = append(f.msgs, fmt.Sprintf(format, args...))
}

func (f *assertionFailures) joined() string {
	return strings.Join(f.msgs, "\n")
}

// mutateExample returns the shipped example with from replaced by to, failing
// the test unless from appears exactly once.
//
// The uniqueness check is not tidiness. A mutation that silently did not apply
// leaves the case asserting something about the UNTOUCHED example: the
// refusal cases would report "no error" and look like a real finding, and the
// drift cases would report "nothing failed" and look like a broken assertion.
// Either way the reader would debug the wrong thing.
//
// Nothing here writes to hooks/settings.example.json. The mutations exist only
// in memory, for exactly as long as the case that made one.
func mutateExample(t *testing.T, from, to string) []byte {
	t.Helper()

	source := string(hooks.SettingsExample)
	if n := strings.Count(source, from); n != 1 {
		t.Fatalf("the mutation source\n  %q\nappears %d times in hooks/settings.example.json, want exactly 1. The mutation would not have applied cleanly and the case would be asserting something about the untouched file", from, n)
	}
	return []byte(strings.Replace(source, from, to, 1))
}
