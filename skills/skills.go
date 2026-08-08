// Package skills carries the skill documents the dira binary installs.
//
// It sits here, beside skills/dira/, for the reason the schema package sits
// beside entry.schema.json: go:embed cannot reach above its own directory. The
// alternative is a second copy of SKILL.md somewhere a package under internal/
// can reach, and that copy is a document that drifts from the one every other
// check in this repository reads — internal/skill's shape check and E2-L2-T5's
// invocation extractor both open skills/dira/SKILL.md by path. A file embedded
// where it already lives cannot drift from itself, and needs no drift test to
// say so.
//
// Nothing here reads a file at run time. The bytes are compiled into the
// binary, so `dira install-skill` works from any working directory, from a
// released artifact with no checkout anywhere near it, and with the network
// unplugged (cst-0004).
package skills

import _ "embed"

// Dira is skills/dira/SKILL.md — dira's capture tier 2, the document
// `dira install-skill` writes into a Claude Code configuration root.
//
//go:embed dira/SKILL.md
var Dira []byte
