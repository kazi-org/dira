package render_test

import (
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/render"
)

// This package was extracted from internal/why so that `dira brief` and
// `dira why` wrap the same way rather than nearly the same way. The proof that
// the extraction changed nothing is cmd/dira/testdata/why/*.golden, which did
// not move; what is pinned here is the layout contract the second caller now
// depends on, so a later change made for the brief's sake cannot quietly widen
// the chain.

// TestRowsStayInsideTheWidth is the property both callers rely on: a terminal
// that is 72 columns wide gets output 72 columns wide.
func TestRowsStayInsideTheWidth(t *testing.T) {
	t.Parallel()

	const text = "One file per entry, not an append-only JSONL ledger, because every " +
		"concurrent writer appends at the same offset and git cannot merge that"

	for _, width := range []int{56, 64, 80, 120} {
		p := render.New(width, 24)
		p.Row("", "", text, "")
		p.EntryRow("  ", "  ", "dec-0002", text, "accepted 2026-07-29")
		p.EntryBlock("  ", "  ", "qst-0001", text, "open 2026-07-29", "blocks dec-0011 — "+text)
		p.Label("  ", "revisit if", text)

		for _, line := range strings.Split(p.String(), "\n") {
			if n := len([]rune(line)); n > width {
				t.Errorf("a line is %d columns at width %d: %q", n, width, line)
			}
		}
	}
}

// TestARightMarginTooTightMovesToItsOwnLine pins the narrow stacked form: the
// status column never pushes a row past the width, and never squeezes the title
// into a column of single words.
func TestARightMarginTooTightMovesToItsOwnLine(t *testing.T) {
	t.Parallel()

	p := render.New(40, 24)
	p.EntryRow("            ", "            ", "dec-0002", "One file per entry", "accepted 2026-07-29")
	out := p.String()

	if !strings.Contains(out, "accepted 2026-07-29") {
		t.Fatalf("the status was dropped rather than moved:\n%s", out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Errorf("the status stayed on the row it could not fit on:\n%s", out)
	}
	if strings.Contains(lines[0], "accepted") {
		t.Errorf("the status is still on the first row:\n%s", out)
	}

	// This case is deliberately past the point where the layout can hold the
	// width: a twelve-column indent plus an id plus a nineteen-column status
	// leaves the title below the floor, and the documented behaviour there is
	// to overflow rather than to render one word per line. What is pinned is
	// that the overflow is bounded — the status sits at the continuation
	// column and nothing is lost — not that it never happens.
	last := lines[len(lines)-1]
	if n := len([]rune(last)); n > 40+len("accepted 2026-07-29") {
		t.Errorf("the stacked status overflowed further than the column it hangs under: %q", last)
	}
}

// TestLinesReflowsRatherThanPreservingTheSourceWrapping. The ledger's prose
// arrives hand-wrapped inside folded YAML scalars, so its line breaks are an
// artifact of the file rather than of the meaning.
func TestLinesReflowsRatherThanPreservingTheSourceWrapping(t *testing.T) {
	t.Parallel()

	hand := "a decision\nwrapped by whoever\n   typed it"
	got := render.Lines(hand, 40)
	if len(got) != 1 {
		t.Errorf("Lines(%q, 40) = %q, want one reflowed line", hand, got)
	}
	if got[0] != "a decision wrapped by whoever typed it" {
		t.Errorf("Lines collapsed whitespace differently: %q", got[0])
	}

	if only := render.Lines("", 40); len(only) != 1 || only[0] != "" {
		t.Errorf("Lines of an empty string = %q, want one empty line", only)
	}
}

// TestTrailingSpaceIsNeverWritten. A right margin leaves padding behind, and
// trailing whitespace is invisible on a screen and loud in a diff — these lines
// are compared byte for byte by golden tests in two packages.
func TestTrailingSpaceIsNeverWritten(t *testing.T) {
	t.Parallel()

	p := render.New(80, 24)
	p.Row("", "", "a short line", "open 2026-07-29")
	p.Line("padded            ")
	for _, line := range strings.Split(p.String(), "\n") {
		if strings.HasSuffix(line, " ") {
			t.Errorf("a line ends in whitespace: %q", line)
		}
	}
}
