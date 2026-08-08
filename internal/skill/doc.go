// Package skill holds the checks that keep dira's agent-facing skill honest.
//
// # Why a skill needs a test package at all
//
// skills/dira/SKILL.md is prose that instructs an agent to run commands. That
// makes it the one artifact in this repository that can drift from the binary
// without anything failing: a verb that was renamed, a flag that was never
// added, an instruction that quietly contradicts a decision in the ledger — none
// of it breaks a build, and the symptom at the far end is a capture that did not
// happen, which looks exactly like a session that settled nothing. kazi already
// paid for this lesson (test/kazi/teach/skill_covers_commands_test.exs) and the
// failure it names is the one this package exists to make loud.
//
// The package is deliberately thin. Everything about the skill that can be
// checked mechanically is checked here; everything that cannot — whether the
// instructions are good ones — is a review question and this package does not
// pretend to answer it.
//
// # Where the pieces live
//
// E2-L2-T4 owns the skill document and the shape check over it (shape_test.go).
// E2-L2-T5 adds the invocation extractor and the assertion that every command
// and flag the skill names is one the binary registers; that assertion cannot
// live here, because the command registry is `newApp(...).commands` in package
// main, so the extractor is this package's and the check against the registry is
// cmd/dira's. E2-L2-T6 replays a recorded tier-2 response through the built
// binary. E2-L2-T8 installs the file.
//
// This file carries no code on purpose: a package whose only Go files are tests
// is not a package `go build ./...` will accept, and inventing a function for
// the compiler's benefit would be inventing a function.
package skill
