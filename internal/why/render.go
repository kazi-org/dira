package why

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/render"
)

// Width bounds for the text renderer.
const (
	// DefaultWidth is the column the text renderer wraps at.
	//
	// It is a constant rather than the terminal's real width, and that is
	// not a limitation dodged. Reading the window size means an ioctl, which
	// means os or golang.org/x/term in a package dec-0005 forbids to know
	// what a file descriptor is — and it would make `dira why` produce
	// different bytes in a pipe than on a screen, which is exactly what the
	// golden test exists to catch. `dira why --width` overrides it for a
	// narrower terminal.
	DefaultWidth = 80

	// MinWidth is the narrowest column the layout can hold, and it is
	// derived rather than chosen: the deepest fixed indent this renderer
	// produces is an edge note under the longest label ("superseded by",
	// then the id column, then the note's own two), which is 31 columns, and
	// minTextWidth is the measure below which prose stops being prose. A
	// narrower request is raised to this rather than refused, because a
	// terminal 30 columns wide is still a terminal someone is reading.
	MinWidth = 56

	// minTextWidth is the space a wrapped column keeps for its text however
	// deep the indent gets. It is the one case where a line may exceed the
	// width: a chain deep enough to squeeze the text below this overflows
	// rather than rendering one word per line, because a column of single
	// words is unreadable in a way an over-long line is not.
	//
	// At MinWidth the fixed indents all fit, so overflow needs an ancestry
	// about five generations deep — which is the chain-at-scale question
	// DESIGN.md leaves open for depth and E6 owns. This renderer does not
	// answer it: it does not collapse, elide or truncate anything, so what
	// it prints is always the whole chain.
	minTextWidth = 24
)

// The chain's box-drawing vocabulary.
//
// These are characters, not graphics: docs/design/DESIGN.md law 3 makes the
// chain "type, never a picture", so select-all over `dira why` output yields the
// tree. No ANSI is emitted anywhere in this file — not colour, not cursor
// movement — so the tree is intact whether or not anything strips escapes, and
// a refusal is drawn with a mark rather than with red (law 1: red means the
// compass caught something, and a rejected alternative is a record, not an
// alarm).
const (
	tee   = "├─ "
	elbow = "└─ "
	pipe  = "│  "
	stem  = "   "

	markRefused = "✗ "
)

// RenderText writes the chain as `dira why` prints it.
//
// width of zero means DefaultWidth. This is renderer one of two: E6 renders the
// same *Chain as HTML, and neither renderer reads the ledger.
func RenderText(w io.Writer, c *Chain, width int) error {
	p := newPainter(width)
	p.chain(c)
	_, err := io.WriteString(w, p.String())
	if err != nil {
		return fmt.Errorf("writing the chain: %w", err)
	}
	return nil
}

// RenderCandidates writes the disambiguation list for a term matching more than
// one entry.
//
// It is an answer, not an error message. A resolver that picked one of five
// entries and rendered its chain would answer a question nobody asked, and would
// look exactly like the right answer while doing it.
func RenderCandidates(w io.Writer, query string, nodes []Node, width int) error {
	p := newPainter(width)

	p.Line(fmt.Sprintf("%d entries match %q", len(nodes), query))
	p.Line("")
	for _, n := range nodes {
		p.EntryRow(stem, stem, n.Ref, n.Title, status(n))
	}
	if len(nodes) > 0 {
		p.Line("")
		p.Line("name one to see its chain: dira why " + nodes[0].Ref)
	}

	_, err := io.WriteString(w, p.String())
	if err != nil {
		return fmt.Errorf("writing the candidates: %w", err)
	}
	return nil
}

func clampWidth(width int) int {
	if width <= 0 {
		return DefaultWidth
	}
	if width < MinWidth {
		return MinWidth
	}
	return width
}

// painter is the shared column layout (internal/render) plus the chain's own
// grammar: the box-drawing spine, the refusal mark, the generation indents.
//
// The layout is shared rather than copied because `dira brief` draws the same
// rows at the same widths, and two implementations of "wrap at 80 columns" is
// how two surfaces that are meant to look like one product start disagreeing.
// What is *not* shared is everything below — a brief has no tree — so the split
// runs exactly where the second caller's needs stop.
type painter struct{ *render.Painter }

func newPainter(width int) *painter {
	return &painter{render.New(clampWidth(width), minTextWidth)}
}

// chain paints the whole answer.
func (p *painter) chain(c *Chain) {
	p.spine(c)
	p.subject(c)
	p.edges(c)
	if len(c.Cycle) > 0 {
		p.Line("")
		p.Row("", "", fmt.Sprintf(
			"the derives_from chain loops back to %s, so the walk stopped there; "+
				"an entry cannot arise from itself and this ledger says one does",
			strings.Join(c.Cycle, ", ")), "")
	}
}

// spine paints the derives_from ancestry, outermost generation first.
func (p *painter) spine(c *Chain) {
	for g, generation := range c.Arising {
		for i, n := range generation {
			prefix, cont := branch(g, i == len(generation)-1)
			p.EntryBlock(prefix, cont, n.Ref, n.Title, status(n), n.Note)
		}
	}
}

// subject paints the entry asked about and everything hanging under it.
func (p *painter) subject(c *Chain) {
	depth := len(c.Arising)
	prefix, cont := branch(depth, true)
	p.EntryRow(prefix, cont, c.Subject.Ref, c.Subject.Title, status(c.Subject))

	// Each child is a closure so the last one can be found before any of
	// them is painted: the branch glyph for a row depends on whether
	// anything follows it, and that is not knowable while emitting.
	var children []func(prefix, cont string)

	for _, n := range c.SupersededBy {
		children = append(children, func(prefix, cont string) {
			// No right margin: what matters on this row is that
			// something replaced the subject, not what state the
			// replacement is itself in — and at this indent the
			// status column costs the title a third of its width.
			const lead = "superseded by  "
			pad := cont + strings.Repeat(" ", render.RuneLen(lead))
			p.EntryBlock(prefix+lead, pad, n.Ref, n.Title, "", n.Note)
		})
	}

	for _, alt := range c.Alternatives {
		children = append(children, func(prefix, cont string) {
			// The mark and no colour. A rejected alternative is a
			// record, not an alarm (DESIGN.md law 1), and --caught is
			// reserved for drift, contradiction and `dira check`.
			p.Row(prefix, cont+"  ", markRefused+alt.Option, "")
			p.Row(cont+"  ", cont+"  ", alt.WhyNot, "")
			if alt.RevisitIf != "" {
				p.Label(cont+"  ", "revisit if", alt.RevisitIf)
			}
		})
	}
	if len(c.Alternatives) == 0 {
		children = append(children, func(prefix, cont string) {
			// Stated, not skipped. An absent section and a recorded
			// absence read completely differently to someone deciding
			// whether a choice was ever weighed.
			p.Row(prefix, cont, NoAlternatives(c.Subject.Kind), "")
		})
	}

	for _, a := range c.Realized {
		children = append(children, func(prefix, cont string) {
			// Verbatim. dira does not ask kazi whether this converged
			// and must not imply that it did (dec-0004).
			p.Row(prefix, cont, "realized_by  "+a.Target, "")
			if a.Note != "" {
				p.Row(cont+"  ", cont+"  ", a.Note, "")
			}
		})
	}

	if c.ADR != "" {
		children = append(children, func(prefix, cont string) {
			// The path, never the file's contents: the entry is the
			// record and the ADR is exhaust (dec-0009).
			p.Row(prefix, cont, "adr  "+c.ADR, "")
		})
	}

	base := cont
	for i, paint := range children {
		glyph, follow := tee, pipe
		if i == len(children)-1 {
			glyph, follow = elbow, stem
		}
		paint(base+glyph, base+follow)
	}
}

// edges paints the relations that sit beside the chain rather than in it,
// grouped so a label is written once however many rows it carries.
func (p *painter) edges(c *Chain) {
	if len(c.Related) == 0 {
		return
	}

	labels := make([]string, 0, len(relationGroups))
	rows := make(map[string][]Relation)
	for _, r := range c.Related {
		name := relationLabel(r)
		if _, ok := rows[name]; !ok {
			labels = append(labels, name)
		}
		rows[name] = append(rows[name], r)
	}
	sort.SliceStable(labels, func(i, j int) bool {
		return group(rows[labels[i]][0]) < group(rows[labels[j]][0])
	})

	column := 0
	for _, name := range labels {
		if n := render.RuneLen(name); n > column {
			column = n
		}
	}
	column += 2

	p.Line("")
	p.Line("edges")
	for _, name := range labels {
		for i, r := range rows[name] {
			cont := strings.Repeat(" ", 2+column)
			head := cont
			if i == 0 {
				head = "  " + name + strings.Repeat(" ", column-render.RuneLen(name))
			}
			// No right margin here. These rows are pointers to other
			// chains, not a status board: repeating every neighbour's
			// state turns a short list into a table and squeezes the
			// titles that are the reason to follow one.
			p.EntryBlock(head, cont, r.Node.Ref, r.Node.Title, "", r.Node.Note)
		}
	}
}

// branch returns the prefix and continuation for a row at generation g. The
// glyph says how deep the row is; last says whether anything sits beside it.
func branch(g int, last bool) (prefix, cont string) {
	if g == 0 {
		return "", ""
	}
	glyph := tee
	if last {
		glyph = elbow
	}
	return strings.Repeat(stem, g-1) + glyph, strings.Repeat(stem, g)
}

// status is the right-margin column: what state the entry is in, and when.
//
// A ref dira could not open says so in words rather than with a mark. Of the
// three resolution states DESIGN.md names, only orphan is drift; this is not
// that, and it must not read as an alarm.
func status(n Node) string {
	if n.Resolution != Oriented {
		return "not in this ledger"
	}
	date := n.Date
	if len(date) >= 10 {
		date = date[:10]
	}
	return strings.TrimSpace(string(n.State) + " " + date)
}

// NoAlternatives is the sentence a chain shows when nothing was weighed.
//
// It names the kind, because the same absence means two different things: a
// decision that recorded no alternatives is an assertion (design.md §4.2, and
// the schema rejects it), while an intent states a direction and has nothing to
// have chosen between.
//
// It is exported because E6's HTML renderer needs the same sentence. Two
// renderers describing an empty record differently would be two answers to one
// question, and the difference would show up on the surface a stranger lands on.
func NoAlternatives(kind ledger.Kind) string {
	switch kind {
	case ledger.KindIntent:
		return "no alternatives recorded — an intent states a direction rather than choosing between options"
	case ledger.KindConstraint:
		return "no alternatives recorded — a constraint states a rule rather than choosing between options"
	case ledger.KindQuestion:
		return "no alternatives recorded — nothing has been weighed against this question yet"
	default:
		return "no alternatives recorded on this entry"
	}
}

// relationLabel names an edge from the subject's point of view.
func relationLabel(r Relation) string {
	if !r.Incoming {
		return strings.ReplaceAll(string(r.Type), "_", " ")
	}
	switch r.Type {
	case ledger.EdgeDerivesFrom:
		return "derived by"
	case ledger.EdgeInforms:
		return "informed by"
	case ledger.EdgeBlocks:
		return "blocked by"
	case ledger.EdgeSupersedes:
		return "superseded by"
	default:
		return strings.ReplaceAll(string(r.Type), "_", " ") + " (incoming)"
	}
}
