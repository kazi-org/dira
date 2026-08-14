package installhooks

// Install: merge-never-clobber, idempotent, ownership by command prefix.
//
// The package cannot write anything -- Install takes the current bytes and
// whether the file exists, and returns what should be written plus an
// Outcome, the same split internal/skill.Install makes for the same reason
// (E2-L2-T8's precedent). cmd/dira performs the write.
//
// Ownership is decided by the OwnerPrefix T2 declared for each registration's
// event, following kazi's kazi_entry?/2: an event whose array already holds an
// entry whose command starts with that event's prefix contributes no edit.
// Position is never used, and neither is the exact command string -- a later
// task growing an argument (as --all was added to PreCompact's command) must
// not orphan an entry already installed.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// jsonString renders s as a JSON string literal WITHOUT json.Marshal's
// default HTML-escaping of '<', '>' and '&'. Every command this package
// writes contains '>' (the "2>/dev/null" guard), and encoding/json's default
// encoder would escape it to ">" -- valid JSON, but bytes that no longer
// contain the command string a caller might reasonably search for, and unlike
// every other command string this repository writes by hand.
func jsonString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	// A string always encodes cleanly; the error is unreachable.
	_ = enc.Encode(s)
	return strings.TrimRight(buf.String(), "\n")
}

// An Outcome is what one Install did. The values are the words the command
// prints, mirroring internal/skill.Outcome.
type Outcome string

const (
	// Installed means bytes were produced that the caller should write --
	// either a fresh document (the file was absent) or the input with edits
	// spliced in.
	Installed Outcome = "INSTALLED"

	// Unchanged means every registration was already present under its own
	// event, recognised by its owner prefix. No edit was computed, so Data is
	// nil: the caller writes nothing at all, and the file's bytes -- and its
	// sha256 -- are untouched.
	Unchanged Outcome = "UNCHANGED"

	// Removed means Uninstall computed a removal -- either bytes with dira's
	// spans spliced out, or (see UninstallResult.DeleteFile) the whole file.
	Removed Outcome = "REMOVED"
)

// An InstallResult is what Install decided. Data is meaningful only when
// Outcome is Installed; a caller must not write it otherwise.
type InstallResult struct {
	Outcome Outcome
	Data    []byte
}

// Install computes what `dira install-hooks` should write into a Claude Code
// settings file.
//
// exists tells Install whether there is a file to merge into at all. When
// false, data is ignored (there is nothing to preserve) and the result is a
// fresh, minimal settings document holding exactly the three registrations
// and nothing else -- the same bytes uninstall's deletion decision (T5)
// compares a file against to know whether install created it.
//
// The returned error is a refusal, never a guess: a document Scan cannot make
// sense of, or whose "hooks" holds a non-array value under an event dira
// registers, produces a named error and a zero InstallResult. Nothing is
// computed to write in that case.
func Install(data []byte, exists bool) (InstallResult, error) {
	regs, err := Registrations()
	if err != nil {
		return InstallResult{}, err
	}

	if !exists {
		return InstallResult{Outcome: Installed, Data: freshSettings(regs)}, nil
	}

	root, err := Scan(data)
	if err != nil {
		return InstallResult{}, err
	}

	edits, err := installEdits(data, root, regs)
	if err != nil {
		return InstallResult{}, err
	}
	if len(edits) == 0 {
		return InstallResult{Outcome: Unchanged}, nil
	}
	return InstallResult{Outcome: Installed, Data: Insert(data, edits)}, nil
}

// freshSettings is the exact bytes Install writes when no settings file
// exists: a minimal, valid Claude Code settings object holding only dira's
// registrations, one line per event in the order Registrations() declares
// them. Uninstall (T5) pins the same canonical form to decide whether a file
// is exactly what a fresh install would have produced.
func freshSettings(regs []Registration) []byte {
	return []byte("{\n  \"hooks\": " + hooksValueText(regs) + "\n}\n")
}

// installEdits is the insertion set an install needs, ordered by the
// structural gap found: no "hooks" key at all -> ONE member insertion
// carrying every event; a "hooks" object missing an event -> a member
// insertion into it; an event array with no dira entry -> an element appended
// to it. An event that already holds a dira entry (by owner prefix)
// contributes NO edit, so a full set yields nil and Install reports
// Unchanged. Mirrors install_hooks.ex:266-298 (install_edits/event_edits).
func installEdits(data []byte, root *Node, regs []Registration) ([]Insertion, error) {
	hooksMember := root.Member("hooks")
	if hooksMember == nil {
		return []Insertion{insertMember(root, "hooks", hooksValueText(regs), "  ")}, nil
	}
	// Scan already refused a non-object "hooks" value, so hooksMember.Value
	// is an object here.
	hooksObj := hooksMember.Value

	var edits []Insertion
	for _, r := range regs {
		eventMember := hooksObj.Member(r.Event)
		switch {
		case eventMember == nil:
			edits = append(edits, insertMember(hooksObj, r.Event, eventArrayText(r), "    "))

		case eventMember.Value.Kind != KindArray:
			return nil, fmt.Errorf(`%w: "hooks".%q is not a JSON array`, ErrShape, r.Event)

		default:
			arr := eventMember.Value
			if !anyEntryOwnedByPrefix(data, arr.Elements, r.OwnerPrefix) {
				edits = append(edits, appendElement(arr, entryText(r), "      "))
			}
		}
	}
	return edits, nil
}

// anyEntryOwnedByPrefix reports whether ANY command in ANY of the entries
// starts with prefix -- kazi's kazi_entry?/2. One matching command is enough
// for the whole event to count as installed, so install adds nothing beside
// it.
func anyEntryOwnedByPrefix(data []byte, entries []*Node, prefix string) bool {
	for _, entry := range entries {
		for _, command := range entryCommands(data, entry) {
			if strings.HasPrefix(command, prefix) {
				return true
			}
		}
	}
	return false
}

// entryWhollyOwnedByPrefix reports whether EVERY command in entry starts with
// prefix, and there is at least one -- kazi's wholly_kazi_entry?/2. Uninstall
// (T5) removes an entry only when this holds: one mixing an operator's own
// command with dira's is never removed.
func entryWhollyOwnedByPrefix(data []byte, entry *Node, prefix string) bool {
	commands := entryCommands(data, entry)
	if len(commands) == 0 {
		return false
	}
	for _, command := range commands {
		if !strings.HasPrefix(command, prefix) {
			return false
		}
	}
	return true
}

// entryCommands returns the command strings inside one event-array element --
// `{"hooks":[{"type":"command","command":"...","timeout":N}, ...]}` -- decoded
// from the node's own bytes. Mirrors kazi's node_value + entry_commands: the
// decode is delegated to encoding/json on the raw slice rather than read back
// out of the span tree a second time.
func entryCommands(data []byte, entry *Node) []string {
	if entry == nil || entry.Kind != KindObject {
		return nil
	}
	var parsed struct {
		Hooks []struct {
			Command string `json:"command"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data[entry.Start:entry.Stop], &parsed); err != nil {
		return nil
	}
	out := make([]string, 0, len(parsed.Hooks))
	for _, h := range parsed.Hooks {
		out = append(out, h.Command)
	}
	return out
}

// ---- the text install writes -----------------------------------------------
//
// One matcher-group entry per registration -- `{ "hooks": [{ "type":
// "command", "command": "...", "timeout": N }] }` -- matching the shape
// hooks/settings.example.json itself uses (minus its "//" documentation,
// which a generated entry has no use for).

// entryText is the full matcher-group object text for one registration: what
// gets appended as a new element of an event's array.
func entryText(r Registration) string {
	return fmt.Sprintf(`{ "hooks": [{ "type": "command", "command": %s, "timeout": %d }] }`,
		jsonString(r.Command), r.Timeout)
}

// eventArrayText is a whole event's array value holding exactly one
// registration: what gets written when the event key does not exist at all.
func eventArrayText(r Registration) string {
	return "[" + entryText(r) + "]"
}

// hooksValueText is the entire "hooks" object value holding every
// registration, one event per line in Registrations()' own order: what gets
// written when the document has no "hooks" key at all, and what freshSettings
// wraps into a whole document.
func hooksValueText(regs []Registration) string {
	var b strings.Builder
	b.WriteString("{\n")
	for i, r := range regs {
		fmt.Fprintf(&b, "    %s: %s", jsonString(r.Event), eventArrayText(r))
		if i != len(regs)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("  }")
	return b.String()
}

// ---- splice edits: insertions, matching kazi's insert_member/append_element -

// insertMember returns the Insertion that appends `"key": valueText` as a new
// member of obj. Empty object: right after `{`, compact, no added whitespace
// (uninstall's exact-inverse span is then trivial). Non-empty: after the last
// member's value, comma-separated on its own indented line.
func insertMember(obj *Node, key, valueText, indent string) Insertion {
	keyJSON := jsonString(key)
	if len(obj.Members) == 0 {
		return Insertion{At: obj.Start + 1, Text: fmt.Appendf(nil, "%s: %s", keyJSON, valueText)}
	}
	last := obj.Members[len(obj.Members)-1]
	return Insertion{At: last.Value.Stop, Text: fmt.Appendf(nil, ",\n%s%s: %s", indent, keyJSON, valueText)}
}

// appendElement returns the Insertion that appends text as a new element of
// arr, the same shape insertMember uses.
func appendElement(arr *Node, text, indent string) Insertion {
	if len(arr.Elements) == 0 {
		return Insertion{At: arr.Start + 1, Text: []byte(text)}
	}
	last := arr.Elements[len(arr.Elements)-1]
	return Insertion{At: last.Stop, Text: []byte(",\n" + indent + text)}
}
