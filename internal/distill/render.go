package distill

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/kazi-org/dira/internal/ledger"
)

// The card: one staged entry, laid out for a human who is about to spend a
// single byte on it.
//
// # dec-0019 is the whole specification
//
//	The renderer derives presentation from recorded prose. It never invents a
//	field, and where nothing can be derived it renders nothing.
//
// Both halves are load-bearing here, and the second one is the one that shapes
// the code. `dira sniff` writes a title, a state, a created stamp and a
// source{hook, session, excerpt, tier} — no body, no alternatives, no adr
// (docs/decisions-pending/E2-L1-report.md §4). So for the entry this queue
// actually holds, the card has no because to lead with, no struck refusal to
// show and no ADR to point at, and every one of those blocks is simply absent.
// Not a placeholder, not an em dash, not "no rationale recorded" — absent.
//
// That is why there is no template here and no fixed set of lines. Every block
// below is emitted if and only if the entry records the prose it is made of, and
// the card for a regex capture is four lines shorter than the card for a
// semantic one because it knows four fewer things.
//
// # Where the layout comes from
//
// docs/design/screens/s3-distill.html, read as an ordering rather than as
// pixels:
//
//   - the meta line first — id, state, and the provenance the screen puts in
//     `.src` as `PreCompact · 14:22 · semantic`;
//   - the **because** next and above the title, because the because is what is
//     actually being approved (the screen's own comment on `.because`);
//   - the title after it as the secondary label, not the substance;
//   - the struck refusal with its grounds beneath it, which is the device
//     DESIGN.md's r3 → r4 record defends and dec-0019 explicitly preserved;
//   - the edges last, as the screen's `.edge` line.
//
// Two things on that screen are deliberately not here. There is no upheld-option
// card, because dec-0019 dropped it from every surface — the ruling carries the
// chosen road, and a card for it would be one part duplication of the title and
// one part invention. And there are no keyboard hints *as a promise of what the
// buttons do*: the legend at the foot is dec-0024's register, naming what each
// key performs, and it asks no question.
//
// # What the progress indicator promises
//
// `1 of 3` is a claim that the second invocation shows the same card second.
// This file only prints the numbers; the promise is Staged's, which orders by id
// and consults no cache (see queue.go).

// DefaultWidth is the width a card is laid out for when the caller reports none.
//
// 80 because that is what a terminal that has never been resized is, and because
// a card wider than that stops being scannable regardless of what the terminal
// can fit — the design's `--m-ui` measure exists for the same reason on the web.
const DefaultWidth = 80

// minWidth is the narrowest layout this file will attempt. Below it the label
// column and the text would fight for the same characters and every line would
// be one word, so the card is laid out at minWidth and allowed to overflow a
// terminal that small. A card is not the thing to degrade gracefully into a
// 12-column window.
const minWidth = 32

// labelColumn is the width of the leading label on a labelled row, including its
// trailing gap. The labels are `decides` and `excerpt` at 7 runes and `requires`
// at 8, so 10 leaves every one of them at least one space of air and lines the
// text up in a single column.
const labelColumn = 10

// Card returns the Renderer `dira distill` shows, laid out for width columns.
//
// A width of zero or less means DefaultWidth, so a caller that could not ask the
// terminal how wide it is still gets a card rather than a decision to make.
//
// It is a constructor rather than a plain Renderer because Renderer's signature
// is (Item, position, total) and has no room for a width — deliberately, since
// the loop has no business knowing about columns. The width is the surface's
// fact, captured here once when the surface is built.
func Card(width int) Renderer {
	return func(item Item, position, total int) string {
		return renderCard(item, position, total, width)
	}
}

// renderCard is Card's body, taking the width as an argument so tests can drive
// it directly at several widths without building a closure per case.
func renderCard(item Item, position, total, width int) string {
	if item.Entry == nil {
		// Nothing recorded, nothing rendered. The loop never asks for this,
		// but the rule holds at the boundary too.
		return ""
	}
	if width <= 0 {
		width = DefaultWidth
	}
	if width < minWidth {
		width = minWidth
	}
	entry := item.Entry

	// Each element is one block; blocks are separated by a blank line. A block
	// that has nothing to say is never appended, which is how "where nothing
	// can be derived it renders nothing" is expressed structurally rather than
	// by a run of `if` statements around a template.
	var blocks []string
	add := func(text string) {
		if text != "" {
			blocks = append(blocks, text)
		}
	}

	add(metaBlock(item, position, total, width))
	add(paragraph(entry.Body, width))
	add(labelled(ruleLabel(entry.Kind), entry.Title, width))
	add(refusals(entry.Alternatives, width))
	add(provenanceEdges(entry, width))
	add(labelled("excerpt", excerptOf(entry), width))
	add(legendBlock(width))

	return strings.Join(blocks, "\n\n") + "\n"
}

// metaBlock is the two dim lines at the head of the card: what this card is, and
// where it came from.
//
// The progress counter shares the first line with the id and the state rather
// than taking a line of its own. It is orientation, not content, and the screen
// treats it the same way — a 3px rule in the header, not a heading.
func metaBlock(item Item, position, total, width int) string {
	entry := item.Entry

	parts := []string{fmt.Sprintf("%d of %d", position, total), entry.ID}
	if entry.State != "" {
		parts = append(parts, string(entry.State))
	}
	if item.Status == StatusPendingExtraction {
		// dec-0022 made this a state a human has to be able to see, or
		// promoted entries pile up in `staged` looking rejected. It is
		// derived from `confirmed_by` and `source.tier` (see statusOf) and
		// stored nowhere, which is exactly what dec-0019 permits.
		parts = append(parts, string(item.Status))
	}

	lines := []string{strings.Join(parts, " · ")}
	if line := sourceLine(entry); line != "" {
		// hook · time · tier, in that order, from sourceLine in tui.go —
		// one producer for the fallback card and this one, so the two
		// cannot come to disagree about what provenance looks like.
		lines = append(lines, line)
	}
	return paragraph(strings.Join(lines, "\n"), width)
}

// refusals renders the roads not taken: each struck option with its grounds
// beneath it.
//
// The `✗` is the mark `dira why` prints and the screen puts in `.against .m`.
// dec-0019 is explicit that there is no matching `✓` anywhere, because the
// upheld option is the ruling and has no card of its own — a mark the terminal
// cannot produce would make the "same output `dira why` prints" claim marketing.
//
// The whole of `why_not` is shown, not its first sentence. dec-0019 derives the
// screen's one-line summary from that first sentence because the screen has a
// summary slot to fill and a card that has to fit twenty of them; this card
// shows one alternative at a time to a human who is about to approve it, so
// abbreviating the reason would be hiding the thing being approved.
func refusals(alternatives []ledger.Alternative, width int) string {
	var out []string
	for _, alt := range alternatives {
		option := strings.TrimSpace(alt.Option)
		grounds := strings.TrimSpace(alt.WhyNot)
		if option == "" && grounds == "" {
			continue
		}

		var lines []string
		if option != "" {
			lines = append(lines, prefixed("✗ ", "  ", option, width)...)
		}
		if grounds != "" {
			lines = append(lines, prefixed("  ", "  ", grounds, width)...)
		}
		out = append(out, strings.Join(lines, "\n"))
	}
	return strings.Join(out, "\n\n")
}

// provenanceEdges is the screen's `.edge` line: what this entry hangs off, and
// the ADR it mirrors to if it mirrors to one.
//
// The ADR half will emit nothing for anything this lane can produce — no
// transition in internal/distill writes `adr`, and the mirror (dec-0009) is not
// in this lane at all. It is implemented anyway, and that is the point: an
// assertion that the card carries no `mirrors to …` line is worth nothing if the
// renderer has no code path that could ever emit one. With this here, the
// absence on a sniff-shaped card is a measurement of the entry rather than a
// property of the template, and the test shows it both ways.
func provenanceEdges(entry *ledger.Entry, width int) string {
	var parts []string
	for _, edge := range entry.Edges {
		if edge.Type == "" || edge.To == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", edge.Type, edge.To))
	}
	if adr := strings.TrimSpace(entry.ADR); adr != "" {
		parts = append(parts, "mirrors to "+adr)
	}
	return paragraph(strings.Join(parts, " · "), width)
}

// excerptOf is the span of the session the regex matched.
//
// It is labelled `excerpt` and never rendered in the because's position, which
// is the difference between deriving and inventing. The excerpt is what a
// pattern happened to match; the because is reasoning a human stood behind. A
// card that printed the first in the slot the second occupies would be asserting
// that a regular expression wrote the rationale, which dec-0003 forbids outright
// and dec-0019 names as invention.
func excerptOf(entry *ledger.Entry) string {
	if entry.Source == nil {
		return ""
	}
	return strings.TrimSpace(entry.Source.Excerpt)
}

// ruleLabel is the word in front of the title: what this entry does, taken from
// its kind.
//
// The screen labels the title row `Decides`, and every kind needs the same row
// for the same reason. Only decisions can be staged today (dec-0021), so four of
// these five are unreachable from this queue; they are here because the renderer
// takes an Item and not a decision, and a card that printed `decision` in front
// of a question's title would be worse than one that printed nothing.
func ruleLabel(kind ledger.Kind) string {
	switch kind {
	case ledger.KindIntent:
		return "intends"
	case ledger.KindDecision:
		return "decides"
	case ledger.KindQuestion:
		return "asks"
	case ledger.KindConstraint:
		return "requires"
	case ledger.KindNote:
		return "notes"
	}
	// An entry whose kind is not one of the five is not something this
	// renderer can name, so it names nothing and shows the title bare.
	return ""
}

// legendSeparator is what joins the key legend's segments, and what it is
// broken at when it does not fit.
const legendSeparator = " · "

// legendBlock is keyLegend, wrapped at its own separators rather than at spaces.
//
// The legend is 84 columns and an 80-column terminal is the common case, so it
// wraps on most cards rather than exceptionally. Wrapping it as prose strands
// two words of one key's description on a line of their own, which reads as a
// fragment of a sentence rather than as a key; breaking it between keys keeps
// every `x does y` pair whole. The text itself is dec-0024's register and is not
// this file's to restate — `n` is "not a decision" and never "reject", because
// the two meanings collapse back together the moment the card uses the word that
// covers both.
func legendBlock(width int) string {
	segments := strings.Split(keyLegend, legendSeparator)

	var lines []string
	current := segments[0]
	for _, segment := range segments[1:] {
		if runes(current)+runes(legendSeparator)+runes(segment) <= width {
			current += legendSeparator + segment
			continue
		}
		lines = append(lines, current)
		current = segment
	}
	return strings.Join(append(lines, current), "\n")
}

// labelled is a row with a label in the left column and text wrapped in the
// right one, the continuation lines lining up under the text rather than under
// the label.
//
// An empty text renders nothing at all, label included: a label with nothing
// after it is a field the card is claiming exists.
func labelled(label, text string, width int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if label == "" {
		return paragraph(text, width)
	}
	first := fmt.Sprintf("%-*s", labelColumn, label)
	return strings.Join(prefixed(first, strings.Repeat(" ", labelColumn), text, width), "\n")
}

// paragraph wraps text to width, preserving the line breaks already in it.
func paragraph(text string, width int) string {
	text = strings.TrimRight(strings.TrimLeft(text, "\n"), " \t\n")
	if strings.TrimSpace(text) == "" {
		return ""
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		out = append(out, wrapText(line, width)...)
	}
	return strings.Join(out, "\n")
}

// prefixed wraps text and puts first in front of its first line and rest in
// front of every other, so a marker or a label costs its width once and the
// text below it stays in one column.
func prefixed(first, rest, text string, width int) []string {
	indent := runes(rest)
	lines := wrapText(text, width-indent)
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if i == 0 {
			out = append(out, first+line)
			continue
		}
		out = append(out, rest+line)
	}
	return out
}

// wrapText breaks text at spaces so that no line exceeds width.
//
// A single word longer than width is left whole on a line of its own and
// overflows. Breaking it would be the one thing a card must not do to recorded
// prose: an id, a path or a flag cut in half is no longer the thing the entry
// records, and a reader cannot tell a hyphen the author wrote from one the
// renderer added.
//
// Width is counted in runes. The copy on this card is full of `·` and `—`, each
// three bytes and one column, and a byte count would wrap a card of em dashes at
// a third of the terminal.
func wrapText(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	lines := make([]string, 0, 4)
	current := words[0]
	for _, word := range words[1:] {
		if runes(current)+1+runes(word) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	return append(lines, current)
}

// runes is the column count of s, for the reason wrapText gives.
func runes(s string) int { return utf8.RuneCountInString(s) }
