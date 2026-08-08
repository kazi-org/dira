// Package render is the column layout dira's text output is drawn with: wrap a
// paragraph to a width, hang a wrapped title under itself rather than under the
// id in front of it, and set a status hard against the right margin unless the
// terminal is too narrow to hold both.
//
// It exists because there are two text renderers and there will be more. E1-L4's
// why-chain solved the layout first (internal/why); E1-L5's brief needs the same
// rows at the same widths, and a second implementation of "wrap at 80 columns"
// is how two surfaces that are supposed to look like one product start
// disagreeing about what a wrapped line looks like. The chain's box-drawing
// vocabulary stays in internal/why, because that is the chain's own grammar and
// nothing else draws a tree; what lives here is the part both callers share.
//
// Nothing in this package emits ANSI — not colour, not cursor movement.
// docs/design/DESIGN.md law 3 makes dira's output type rather than a picture, so
// selecting it yields exactly what was on the screen, and law 1 reserves colour
// for drift and contradiction. A renderer that could colour would eventually
// colour something.
package render

import (
	"strings"
	"unicode/utf8"
)

// A Painter accumulates lines at a fixed column width.
//
// width is the column text wraps at. floor is the space a wrapped column keeps
// for its text however deep the indent gets, and it is the one case where a line
// may exceed the width: an indent deep enough to squeeze the text below the
// floor overflows rather than rendering one word per line, because a column of
// single words is unreadable in a way an over-long line is not.
type Painter struct {
	b     strings.Builder
	width int
	floor int
}

// New returns a Painter drawing at width columns, keeping at least floor columns
// for text.
func New(width, floor int) *Painter {
	if width < 1 {
		width = 1
	}
	if floor < 1 {
		floor = 1
	}
	return &Painter{width: width, floor: floor}
}

// Width is the column this painter wraps at.
func (p *Painter) Width() int { return p.width }

// String is everything painted so far.
func (p *Painter) String() string { return p.b.String() }

// Line writes one line, trimming the trailing spaces a right margin can leave
// behind. Trailing whitespace is invisible on a screen and loud in a diff, and
// these lines are compared byte for byte by golden tests.
func (p *Painter) Line(s string) {
	p.b.WriteString(strings.TrimRight(s, " "))
	p.b.WriteByte('\n')
}

// Row writes left wrapped into the space prefix leaves, with right set hard
// against the width on the first line. Continuation lines carry cont.
//
// right is placed on the first line rather than the last because that is where
// the design puts it: a state is a property of the row, and a reader scanning
// states down the right margin should find them level with the ids on the left.
func (p *Painter) Row(prefix, cont, left, right string) {
	avail := p.width - RuneLen(prefix)

	// A right margin that would squeeze the text below the floor moves to
	// its own line instead of pushing the row past the width. Overflowing
	// would break the one property the width has: that a narrow terminal
	// stops wrapping the text for you.
	trailing := ""
	if right != "" {
		if avail-RuneLen(right)-2 < p.floor {
			trailing, right = right, ""
		} else {
			avail -= RuneLen(right) + 2
		}
	}
	if avail < p.floor {
		avail = p.floor
	}

	for i, text := range Lines(left, avail) {
		switch {
		case i > 0:
			p.Line(cont + text)
		case right == "":
			p.Line(prefix + text)
		default:
			pad := p.width - RuneLen(prefix) - RuneLen(text) - RuneLen(right)
			if pad < 2 {
				pad = 2
			}
			p.Line(prefix + text + strings.Repeat(" ", pad) + right)
		}
	}

	if trailing != "" {
		pad := p.width - RuneLen(trailing)
		if pad < RuneLen(cont) {
			pad = RuneLen(cont)
		}
		p.Line(strings.Repeat(" ", pad) + trailing)
	}
}

// Label writes a labelled block with the text hanging under itself rather than
// under the label, so a wrapped `revisit if` reads as one thing.
func (p *Painter) Label(prefix, name, text string) {
	gap := name + strings.Repeat(" ", 2)
	p.Row(prefix+gap, prefix+strings.Repeat(" ", RuneLen(gap)), text, "")
}

// EntryRow writes one entry as `<ref>  <title>`, wrapping the title under itself
// rather than under the id, so the left margin of a wrapped title is a column of
// prose and not a column of ids.
func (p *Painter) EntryRow(prefix, cont, ref, title, right string) {
	p.Row(prefix+ref+"  ", cont+strings.Repeat(" ", RuneLen(ref)+2), title, right)
}

// EntryBlock is EntryRow plus a note belonging to the row rather than to the
// entry — the note on the edge that put it here, or the entry it blocks.
//
// The note is indented two columns past the title, because at the title's own
// indent a note and a wrapped title are the same thing to a reader.
func (p *Painter) EntryBlock(prefix, cont, ref, title, right, note string) {
	p.EntryRow(prefix, cont, ref, title, right)
	if note == "" {
		return
	}
	under := cont + strings.Repeat(" ", RuneLen(ref)+4)
	p.Row(under, under, note, "")
}

// Lines breaks text into lines of at most width columns.
//
// Whitespace is collapsed first. The ledger's prose arrives hand-wrapped inside
// folded YAML scalars, so its line breaks are an artifact of how the file was
// written at some other width, not of what the text means — re-wrapping to the
// requested width is the only way the same value renders the same at two widths.
func Lines(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	if width < 1 {
		width = 1
	}

	out := make([]string, 0, 4)
	current := words[0]
	for _, word := range words[1:] {
		if RuneLen(current)+1+RuneLen(word) <= width {
			current += " " + word
			continue
		}
		out = append(out, current)
		current = word
	}
	return append(out, current)
}

// RuneLen counts the columns a string occupies, approximating one column per
// rune. Every glyph dira draws — box-drawing, the refusal mark, Latin prose — is
// single-width; a CJK title would be measured a column short, which is a
// wrapping imperfection rather than a correctness one, and fixing it needs a
// width table this binary has no other reason to carry.
func RuneLen(s string) int { return utf8.RuneCountInString(s) }
