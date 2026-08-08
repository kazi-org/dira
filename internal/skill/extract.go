package skill

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// This file is the extraction half of E2-L2-T5. It turns SKILL.md back into the
// command lines it tells an agent to run, so that something can check them
// against the binary that has to accept them. The assertion against the command
// registry is cmd/dira's, because `newApp(...).commands` is unexported and in
// package main; re-listing the verbs here would check a declaration rather than
// a result, which docs/lore.md L-0001 closes on.
//
// # What counts as an invocation, and why the rule is anchored
//
// Two things in this document look like `dira <word>` and are not commands:
//
//	=== dira handoff, tier 2 ===
//
// is the literal marker the handoff block is delimited with, quoted inside a
// fenced example; and the prose says things like "a dira handoff block appears
// in context". A `\bdira\s+\w+` scan over the file reads both as the verb
// `dira handoff` and turns red on a document that is entirely correct — L-0001
// rule 2 in its purest form, and the specific defect this extractor is written
// to avoid rather than discover later.
//
// So an invocation is one of exactly two shapes, both delimited by the document
// rather than inferred from the prose around them:
//
//   - a line inside a fenced code block whose first word is `dira`. The marker
//     above is inside a fence but its first word is `===`, so the anchor is what
//     excludes it, not a special case naming it.
//   - an inline code span whose first word is `dira` — `dira why`, `dira log <id>`.
//     A span is delimited by backticks the author typed, so a sentence that
//     merely says the word cannot become one.
//
// Everything else in the document is prose about dira and is left alone.
//
// # What it does not do
//
// It does not run a shell. Tokenisation covers the quoting the skill actually
// uses — single quotes, double quotes, backslash continuation, backslash escapes
// — and reports anything it cannot tokenise rather than guessing, because a
// command line with an unbalanced quote is broken whoever reads it. Expansion,
// substitution and pipelines are out of scope and would be out of place in a
// document whose commands an agent types verbatim.

// Invocation is one `dira …` command line the skill tells its reader to run.
type Invocation struct {
	// Command is the verb, e.g. "log". It is empty for an invocation that
	// carries only top-level flags (`dira --version`), whose flag set is
	// dira's own rather than a subcommand's.
	Command string

	// Flags are the flags named, in the order they appear.
	Flags []Flag

	// Args are the positional arguments after the verb, quotes removed.
	Args []string

	// Line is the 1-based line the invocation starts on, for a failure
	// message that names a place in the file rather than a string.
	Line int

	// Text is the invocation as written, continuations joined.
	Text string

	// Inline records that this came from an inline code span rather than
	// from a fenced block.
	Inline bool
}

// Flag is one flag named by an invocation.
type Flag struct {
	// Name is the flag without its leading dashes and without any =value,
	// which is the spelling flag.FlagSet.Lookup uses.
	Name string

	// Text is the token as written, so a report can quote the file.
	Text string
}

// Extract returns every `dira …` invocation the document names.
//
// The error is for a command line that cannot be read at all — an unbalanced
// quote — and never for one that names something the binary does not have. What
// a verb resolves to is the registry's question and this package cannot see the
// registry.
func Extract(src string) ([]Invocation, error) {
	lines := strings.Split(src, "\n")

	var out []Invocation
	fence := ""

	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

		if fence == "" {
			if marker := fenceMarker(trimmed); marker != "" {
				fence = marker
				continue
			}
			spans, err := inlineInvocations(lines[i], i+1)
			if err != nil {
				return nil, err
			}
			out = append(out, spans...)
			continue
		}

		if isFenceClose(trimmed, fence) {
			fence = ""
			continue
		}

		// Inside a fence. Join backslash continuations first, so the
		// indented `--title …` lines of a wrapped command belong to the
		// command above them rather than looking like lines of their own.
		start := i
		text := strings.TrimRight(lines[i], " \t")
		for strings.HasSuffix(text, `\`) && i+1 < len(lines) {
			next := lines[i+1]
			if isFenceClose(strings.TrimSpace(next), fence) {
				// A trailing backslash on the last line of a
				// block. Stop rather than swallow the fence.
				break
			}
			i++
			text = strings.TrimRight(strings.TrimSuffix(text, `\`), " \t") + " " + strings.TrimSpace(next)
		}

		rest, ok := cutDira(text)
		if !ok {
			continue
		}
		inv, err := parseInvocation(rest, start+1, strings.TrimSpace(text), false)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}

	return out, nil
}

// codeSpan matches an inline `…` span. Non-greedy and single-line, which is what
// a span is: a backtick pair does not survive a paragraph break.
var codeSpan = regexp.MustCompile("`([^`\n]+)`")

// inlineInvocations reads the `dira …` spans out of one prose line.
func inlineInvocations(line string, number int) ([]Invocation, error) {
	var out []Invocation
	for _, m := range codeSpan.FindAllStringSubmatch(line, -1) {
		rest, ok := cutDira(m[1])
		if !ok {
			continue
		}
		inv, err := parseInvocation(rest, number, strings.TrimSpace(m[1]), true)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, nil
}

// cutDira reports whether text begins with the word `dira`, and returns what
// follows it.
//
// The word has to be followed by whitespace: `dira` alone is the product's name
// and not a command, and a bare mention of it is what the prose is full of.
func cutDira(text string) (string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimLeft(text, " \t"), "dira")
	if !ok {
		return "", false
	}
	if rest == "" || !isSpace(rest[0]) {
		return "", false
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "", false
	}
	return rest, true
}

// parseInvocation splits the part of a command line after `dira`.
func parseInvocation(rest string, line int, text string, inline bool) (Invocation, error) {
	tokens, err := tokenize(rest)
	if err != nil {
		return Invocation{}, fmt.Errorf("line %d: %w: %s", line, err, text)
	}

	inv := Invocation{Line: line, Text: text, Inline: inline}
	for i, tok := range tokens {
		switch {
		case tok.isFlag():
			inv.Flags = append(inv.Flags, Flag{Name: flagName(tok.text), Text: tok.text})
		case i == 0:
			inv.Command = tok.text
		default:
			inv.Args = append(inv.Args, tok.text)
		}
	}
	return inv, nil
}

// flagName strips a token down to the name flag.FlagSet knows it by.
func flagName(token string) string {
	name := strings.TrimPrefix(strings.TrimPrefix(token, "-"), "-")
	name, _, _ = strings.Cut(name, "=")
	return name
}

// token is one word of a command line, with the one fact about it that its text
// no longer carries.
type token struct {
	text string

	// quoted records that the word arrived inside quotes, which is what
	// keeps a quoted body that opens with a hyphen — `--body '-'`, the
	// spelling that reads the body from stdin — from being read as a flag.
	quoted bool
}

// isFlag reports whether the token is a flag rather than a value.
func (t token) isFlag() bool {
	return !t.quoted && len(t.text) > 1 && t.text[0] == '-' && t.text != "--"
}

// tokenize splits a command line into words the way a POSIX shell would, for
// the quoting this document uses and no more.
func tokenize(s string) ([]token, error) {
	var (
		out      []token
		cur      strings.Builder
		open     bool
		quoted   bool
		inSingle bool
		inDouble bool
	)

	flush := func() {
		if open {
			out = append(out, token{text: cur.String(), quoted: quoted})
			cur.Reset()
			open, quoted = false, false
		}
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
				continue
			}
			cur.WriteByte(c)
		case inDouble:
			switch c {
			case '"':
				inDouble = false
			case '\\':
				if i+1 < len(s) {
					i++
					cur.WriteByte(s[i])
				}
			default:
				cur.WriteByte(c)
			}
		case c == '\'':
			// Only a word that *opens* with a quote is a quoted word.
			// `--body='-'` is a flag whose value happens to be quoted,
			// and reading it as a quoted value would drop the flag.
			quoted = quoted || !open
			inSingle, open = true, true
		case c == '"':
			quoted = quoted || !open
			inDouble, open = true, true
		case c == '\\':
			if i+1 < len(s) {
				i++
				cur.WriteByte(s[i])
				open = true
			}
		case isSpace(c):
			flush()
		default:
			cur.WriteByte(c)
			open = true
		}
	}

	if inSingle || inDouble {
		return nil, errUnbalancedQuote
	}
	flush()
	return out, nil
}

var errUnbalancedQuote = errors.New("the command line has an unbalanced quote")

func isSpace(c byte) bool { return c == ' ' || c == '\t' }

// fenceMarker returns the fence a line opens, or "".
func fenceMarker(trimmed string) string {
	for _, marker := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, marker) {
			return marker
		}
	}
	return ""
}

// isFenceClose reports whether a line closes the open fence. A closing fence
// carries the marker and nothing else; a line that opens with it and continues
// is an info string, which only ever opens.
func isFenceClose(trimmed, fence string) bool {
	return fence != "" && strings.HasPrefix(trimmed, fence) &&
		strings.TrimLeft(trimmed, string(fence[0])) == ""
}
