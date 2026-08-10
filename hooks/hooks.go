// Package hooks carries the Claude Code hook configuration dira installs.
//
// It sits here, beside settings.example.json, for the reason skills/skills.go
// sits beside skills/dira/SKILL.md and schema/schema.go sits beside
// entry.schema.json: go:embed cannot reach above its own directory, so a
// package under internal/ cannot embed a file at the repository root.
//
// The alternative was a pinned table in Go — the three events, their command
// strings and their timeouts — with a test asserting it matches this file.
// That is one declaration checked against another, and it fails in the way
// every drift test fails: the day somebody edits the file and the table
// together and gets one of them subtly wrong, or the day the check is deleted
// because it "only ever fails on the example file". internal/installhooks
// derives the registrations by PARSING these bytes instead, so the file every
// reader is pointed at is the file that gets installed. A file embedded where
// it already lives cannot drift from itself, and needs no drift test to say so.
//
// Nothing here reads a file at run time. The bytes are compiled into the
// binary, so `dira install-hooks` works from any working directory, from a
// released artifact with no checkout anywhere near it, and with the network
// unplugged (cst-0004). Reading them starts no process and contacts nothing
// (int-0002).
package hooks

import _ "embed"

// SettingsExample is hooks/settings.example.json: the hook registrations
// `dira install-hooks` merges into a Claude Code settings file, together with
// the documentation keys that explain each one to whoever opens the file.
//
// What is registered on PreCompact is tier-1 capture that also prints a block
// (dec-0023). PreCompact stdout does not reach the model — on exit 0 it is
// appended to the compaction summariser's prompt, and that summariser cannot
// call a tool — so nothing installed there is a working tier-2 handoff, and
// nothing in this package or its callers may describe it as one.
//
//go:embed settings.example.json
var SettingsExample []byte
