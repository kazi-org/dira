// Package installhooks decides what `dira install-hooks` writes into a Claude
// Code settings file, and derives what it installs from the example file this
// repository ships.
//
// The package names no path and opens no file: it takes bytes and returns
// bytes, the same split internal/skill makes for the same reason. Confining
// the write to a directory the operator chose is cmd/dira's job, and
// internal/ledger/boundary_test.go keeps os, io/fs, path and path/filepath out
// of everything here (dec-0005).
package installhooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/kazi-org/dira/hooks"
)

// A Registration is one hook dira installs.
//
// Event and Command and Timeout are read out of hooks/settings.example.json —
// they are not written down a second time here, which is the whole point of
// embedding the file rather than pinning a table beside it. OwnerPrefix is
// dira's own recognition rule and is declared below, because it is not a
// property of any one command string but of how dira finds its entry again in
// a settings file somebody else has since edited.
type Registration struct {
	// Event is the Claude Code hook event the command fires on.
	Event string

	// Command is the shell command string, exactly as it appears in the
	// example file — guard included, because the guard is the part that
	// makes a dira failure not a session failure.
	Command string

	// Timeout is the hard bound in seconds Claude Code applies to the
	// command. A hang is out of the shell guard's reach; this is the only
	// thing that bounds one.
	Timeout int

	// OwnerPrefix is the leading substring by which dira recognises this
	// entry as its own in a settings file it did not write.
	OwnerPrefix string
}

// ownership is the complete set of events dira registers, each with the prefix
// by which it claims its entry there. It mirrors kazi's `@command_prefix`
// (lib/kazi/teach/install_hooks.ex, kazi_entry?/2), which exists so that a
// later task can grow a command's arguments without orphaning the entries
// already installed.
//
// That is not hypothetical here. `--all` was added to the PreCompact command
// after the installed form had been measured losing a third of its captures.
// An installer matching on the whole command string would not recognise an
// older `dira sniff --deep --stage`, would add a second entry beside it, and
// the session would run sniff twice per compaction — with an uninstall then
// orphaning the older one. Position is never used either: an operator is free
// to reorder the entries in their own file.
//
// Each prefix is also narrow enough to exclude an operator's own dira hook. A
// prefix of "dira" would swallow `dira why dec-0003` under Stop, dira would
// conclude its capture hook was already installed, and the session would end
// up with no capture at all — a failure that looks exactly like success.
//
// The order here is not the installation order. That comes from the example
// file, which is the document a reader is pointed at.
var ownership = []struct {
	Event  string
	Prefix string
}{
	{Event: "SessionStart", Prefix: "dira brief"},
	{Event: "Stop", Prefix: "dira sniff --stage"},
	{Event: "PreCompact", Prefix: "dira sniff --deep"},
}

// ErrMalformed marks bytes that are not JSON at all.
var ErrMalformed = errors.New("installhooks: the settings bytes are not valid JSON")

// ErrShape marks valid JSON that is not shaped like hook registrations — a
// non-object root, a missing "hooks" key, an entry with no command or no
// timeout, more than one command under one entry.
//
// It is a refusal, deliberately, and not a best guess. An entry whose
// "timeout" is absent is bounded by Claude Code's own default rather than by
// the number this repository documents, so a parser that filled in a zero
// would install a hook with a bound nobody chose and say nothing about it.
var ErrShape = errors.New("installhooks: the settings are not shaped like hook registrations")

// ErrUnknownEvent marks a hook event dira declares no owner prefix for. dira
// cannot recognise its own entry under an event it has no prefix for, so it
// will not install one there.
var ErrUnknownEvent = errors.New("installhooks: hook event dira does not register")

// ErrMissingEvent marks a document that omits an event dira does register.
var ErrMissingEvent = errors.New("installhooks: hook event dira registers is absent")

// embedded parses the shipped example once, on first use.
//
// Not in an init function: schema/schema.go records what package-level
// initialisation costs a command that runs on every hook invocation, and while
// parsing four kilobytes of JSON is nowhere near that, a binary that pays for
// it whether or not `install-hooks` was the verb is paying for nothing.
var embedded = sync.OnceValues(func() ([]Registration, error) {
	return ParseRegistrations(hooks.SettingsExample)
})

// Registrations returns the hooks dira installs, in the order
// hooks/settings.example.json declares them.
//
// The bytes are compiled in (hooks.SettingsExample), so this reads nothing
// from disk, needs no checkout nearby and touches no network (cst-0004), and
// starts nothing (int-0002).
//
// The error is not decoration. It is returned rather than panicked because the
// only thing that can produce it is an edit to the example file, and the
// caller that will meet it is a test in this repository rather than an
// operator on a released binary.
func Registrations() ([]Registration, error) {
	regs, err := embedded()
	if err != nil {
		return nil, err
	}
	// A copy: the parse happens once and every caller would otherwise share
	// one slice with everyone else who asked.
	return slices.Clone(regs), nil
}

// ParseRegistrations derives the registrations from a Claude Code settings
// document, in the order the document declares them.
//
// It refuses rather than guesses. Anything it cannot read as a registration is
// a named error and NO registrations at all — a partial answer here would be
// installed unread.
func ParseRegistrations(settings []byte) ([]Registration, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(settings, &root); err != nil {
		if !json.Valid(settings) {
			return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
		}
		return nil, fmt.Errorf("%w: the root is not a JSON object: %v", ErrShape, err)
	}

	hooksRaw, ok := root["hooks"]
	if !ok {
		return nil, fmt.Errorf(`%w: the root has no "hooks" key`, ErrShape)
	}
	events, err := orderedMembers(hooksRaw)
	if err != nil {
		return nil, fmt.Errorf(`%w: "hooks" is not an object: %v`, ErrShape, err)
	}

	var regs []Registration
	seen := map[string]bool{}
	for _, event := range events {
		if strings.HasPrefix(event.key, "//") {
			// The documentation convention the example file itself uses,
			// at every level of the document. A comment is not an event.
			continue
		}
		prefix, registered := ownerPrefix(event.key)
		if !registered {
			return nil, fmt.Errorf("%w: %q. dira registers %s and nothing else, so it has no prefix by which it could recognise an entry there again",
				ErrUnknownEvent, event.key, strings.Join(registeredEvents(), ", "))
		}
		if seen[event.key] {
			return nil, fmt.Errorf("%w: %q is declared twice, and the second declaration would silently win", ErrShape, event.key)
		}
		seen[event.key] = true

		command, timeout, err := soleCommand(event.value)
		if err != nil {
			return nil, fmt.Errorf("%w: under %q: %v", ErrShape, event.key, err)
		}
		regs = append(regs, Registration{
			Event:       event.key,
			Command:     command,
			Timeout:     timeout,
			OwnerPrefix: prefix,
		})
	}

	for _, want := range registeredEvents() {
		if !seen[want] {
			return nil, fmt.Errorf("%w: %q. dira installs %d hooks and this document describes %d of them",
				ErrMissingEvent, want, len(ownership), len(regs))
		}
	}
	return regs, nil
}

// ownerPrefix returns the prefix by which dira claims its entry under event,
// and whether dira registers that event at all.
func ownerPrefix(event string) (string, bool) {
	for _, o := range ownership {
		if o.Event == event {
			return o.Prefix, true
		}
	}
	return "", false
}

// registeredEvents names the events dira registers, in a fixed order so a
// failure message reads the same on every run.
func registeredEvents() []string {
	out := make([]string, 0, len(ownership))
	for _, o := range ownership {
		out = append(out, o.Event)
	}
	return out
}

// A member is one object member and the bytes of its value.
type member struct {
	key   string
	value json.RawMessage
}

// orderedMembers returns an object's members in the order the bytes declare
// them.
//
// encoding/json decodes an object into a map, and a map has no order — so the
// registrations would come out in whatever order Go's map iteration happened
// to choose, "in the file's own order" could not be asserted at all, and the
// installer would insert its three entries in a different order on different
// runs. The token stream keeps the document's own order without giving up
// encoding/json as the authority on what is valid JSON.
func orderedMembers(raw json.RawMessage) ([]member, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	open, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := open.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("found %v where an object was expected", open)
	}

	var out []member
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("found %v where a member name was expected", tok)
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, fmt.Errorf("member %q: %w", key, err)
		}
		out = append(out, member{key: key, value: value})
	}
	return out, nil
}

// A matcherGroup is one element of an event's array in a Claude Code settings
// file. Its "matcher" and its "//" documentation array are somebody else's
// business and are deliberately not decoded here.
type matcherGroup struct {
	Hooks []hookEntry `json:"hooks"`
}

// A hookEntry is one command Claude Code runs.
//
// Every field is a pointer so that absent is distinguishable from zero. A
// missing "timeout" decoded as 0 would be indistinguishable from a timeout of
// zero seconds, and both would be installed without comment.
type hookEntry struct {
	Type    *string `json:"type"`
	Command *string `json:"command"`
	Timeout *int    `json:"timeout"`
}

// soleCommand returns the one command and timeout an event's value declares,
// and an error describing anything else it found.
func soleCommand(raw json.RawMessage) (command string, timeout int, err error) {
	var groups []matcherGroup
	if err := json.Unmarshal(raw, &groups); err != nil {
		return "", 0, fmt.Errorf("the value is not an array of matcher groups: %w", err)
	}
	if len(groups) != 1 {
		return "", 0, fmt.Errorf("%d matcher groups; dira registers exactly one", len(groups))
	}

	entries := groups[0].Hooks
	if len(entries) != 1 {
		return "", 0, fmt.Errorf(`%d commands under one "hooks" array; dira registers exactly one, and a second one here would be installed unread`, len(entries))
	}

	entry := entries[0]
	switch {
	case entry.Type == nil:
		return "", 0, errors.New(`the entry has no "type"`)
	case *entry.Type != "command":
		return "", 0, fmt.Errorf(`the entry's "type" is %q, not "command"`, *entry.Type)
	case entry.Command == nil:
		return "", 0, errors.New(`the entry has no "command"`)
	case strings.TrimSpace(*entry.Command) == "":
		return "", 0, errors.New(`the entry's "command" is empty`)
	case entry.Timeout == nil:
		return "", 0, errors.New(`the entry has no "timeout", so Claude Code would bound it by its own default rather than by the number this repository documents`)
	}
	return *entry.Command, *entry.Timeout, nil
}
