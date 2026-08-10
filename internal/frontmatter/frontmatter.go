// Package frontmatter splits a dira entry file into its YAML frontmatter and
// its markdown body.
//
// It is a separate package for one reason, and the reason is a measurement.
// Splitting a file on `---` is string handling and costs nothing; the schema
// package next door embeds entry.schema.json and compiles it with
// santhosh-tekuri/jsonschema. When the ledger codec reached into schema for
// this one function, the validator came with it: linking jsonschema into the
// binary costs milliseconds of package init and ~21,700 allocations on every
// dira invocation, warm or cold, whether or not anything validates. int-0002
// budgets a hook invocation at well under 100ms, and a hook fires on every tool
// call, so that is a real share of the budget spent on a library the command
// path never calls.
//
// So the split lives here, where it depends on nothing, and schema forwards to
// it. schema.SplitFrontmatter and schema.ErrNoFrontmatter remain the published
// names — this is a dependency move, not an API change — but a package on the
// command path should import this one.
package frontmatter

import (
	"errors"
	"fmt"
	"strings"
)

// ErrMissing marks a file that is not an entry at all, as opposed to an entry
// that is wrong. The distinction matters: the first is a stray file, the second
// is ledger rot, and only the second should fail a ledger-wide gate.
//
// It is the value schema.ErrNoFrontmatter names, so errors.Is answers the same
// either way and no caller has to know which package it came from.
var ErrMissing = errors.New("no YAML frontmatter")

// Split returns the YAML frontmatter of a dira entry file and the markdown body
// that follows it. An entry opens with a `---` line and closes the block with
// another; everything after the closing delimiter is the body, returned byte for
// byte.
func Split(content []byte) (front, body []byte, err error) {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, nil, ErrMissing
	}
	rest := text[len("---\n"):]

	// The closing delimiter is a line that is exactly "---".
	for offset := 0; offset < len(rest); {
		end := strings.IndexByte(rest[offset:], '\n')
		line := rest[offset:]
		next := len(rest)
		if end >= 0 {
			line = rest[offset : offset+end]
			next = offset + end + 1
		}
		if strings.TrimRight(line, " \t") == "---" {
			return []byte(rest[:offset]), []byte(rest[next:]), nil
		}
		offset = next
	}
	return nil, nil, fmt.Errorf("%w: frontmatter opened but never closed", ErrMissing)
}
