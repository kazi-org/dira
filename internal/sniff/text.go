package sniff

import (
	"regexp"
	"strings"
)

// fencedCode matches a ``` block, including an unterminated one at the end of a
// message. Everything inside is removed before a sentence ever exists: code is
// full of the words the matcher keys on ("go", "over", "not"), carries none of
// their meaning, and is the one part of a transcript most likely to hold a path
// or a token.
var fencedCode = regexp.MustCompile("(?s)```.*?(```|$)")

// quotedLine matches a markdown blockquote. Quoted text is somebody else's
// sentence — a pasted instruction, a citation, a message from another session —
// and capturing it would record their words as this session's decision.
var quotedLine = regexp.MustCompile(`(?m)^\s*>.*$`)

// markup is the inline markdown the title rendering removes. The characters
// carry emphasis, not content, and a ledger title reading `**Decision**` is a
// title that renders wrong everywhere dira prints it.
var markup = regexp.MustCompile("[*_`]+")

// listMarker matches the bullet or number a line may open with.
var listMarker = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+`)

// heading matches an ATX heading's hashes.
var heading = regexp.MustCompile(`^\s*#{1,6}\s+`)

// discourse is the connective a sentence may open with, which is meaningful in a
// paragraph and noise in a title.
var discourse = regexp.MustCompile(`^(?:So|And|But|Then|Now|Also|Well),?\s+`)

// whitespace collapses any run of spacing, including the newlines a hard-wrapped
// paragraph puts in the middle of a sentence.
var whitespace = regexp.MustCompile(`\s+`)

// strip removes what the matcher must never see.
func strip(text string) string {
	text = fencedCode.ReplaceAllString(text, "\n")
	text = quotedLine.ReplaceAllString(text, "")
	return text
}

// sentenceEnd splits on terminal punctuation followed by whitespace. The
// lookahead-free form Go's regexp allows keeps the punctuation with the sentence
// it closes, which matters: the question-mark guard needs to see it.
var sentenceEnd = regexp.MustCompile(`([.!?])\s+`)

// sentences splits prose into the units the matcher grades.
//
// Newlines split unconditionally, because transcript prose is markdown: a bullet
// list is a list of separate statements that no amount of punctuation analysis
// will separate, and a heading followed by a paragraph is two thoughts on two
// lines. Within a line, terminal punctuation splits.
//
// The split is deliberately crude. A sentence splitter good enough to handle
// "e.g." correctly would be a dependency, and the cost of getting it wrong here
// is one candidate whose excerpt begins mid-clause — visible to the human
// disposing of it, and recoverable by them.
func sentences(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, s := range splitSentences(line) {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func splitSentences(line string) []string {
	locs := sentenceEnd.FindAllStringSubmatchIndex(line, -1)
	if len(locs) == 0 {
		return []string{line}
	}
	var out []string
	start := 0
	for _, loc := range locs {
		// loc[3] is the end of the punctuation group: keep it, drop the
		// whitespace that follows.
		out = append(out, line[start:loc[3]])
		start = loc[1]
	}
	if start < len(line) {
		out = append(out, line[start:])
	}
	return out
}

// titleFor renders a matched sentence as an entry title.
//
// The schema wants one legible line with no trailing period, at most 120
// characters. A regex tier cannot write a better title than the sentence it
// found, and inventing one would be the tier asserting a summary it has no basis
// for — so this is a cleanup, not a rewrite. The human who disposes of the entry
// is the one who gets to improve it.
func titleFor(sentence string) string {
	s := heading.ReplaceAllString(sentence, "")
	s = listMarker.ReplaceAllString(s, "")
	s = markup.ReplaceAllString(s, "")
	s = whitespace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	s = discourse.ReplaceAllString(s, "")
	s = strings.TrimRight(s, " .!?,;:—-")
	s = strings.TrimSpace(s)

	if len([]rune(s)) < 3 {
		return ""
	}
	return bound(s, maxTitle)
}

// bound truncates to at most n runes, on a word boundary where there is one, and
// marks the truncation. An excerpt that stops mid-word reads as corruption; one
// that stops at "…" reads as a bound.
func bound(s string, n int) string {
	s = strings.TrimSpace(whitespace.ReplaceAllString(s, " "))
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	cut := string(runes[:n-1])
	if i := strings.LastIndexAny(cut, " \t"); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " .,;:—-") + "…"
}
