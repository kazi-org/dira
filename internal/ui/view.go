package ui

import (
	"context"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"unicode"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/why"
)

// Source is the read surface the surfaces are rendered from. *index.Index
// satisfies it.
//
// It embeds why.Source rather than restating it, because the decision page is
// rendered from a *why.Chain and must ask the ledger nothing the chain builder
// does not already ask. Select is the one addition, and it belongs to the index
// page: "every entry, grouped by the intent it arises from" is a query, not a
// walk over 200 files.
type Source interface {
	why.Source
	Select(ctx context.Context, sel index.Selector) ([]index.Ref, error)
}

// ---------------------------------------------------------------------------
// The decision page
// ---------------------------------------------------------------------------

// A ChainRow is one line of the rendered why-chain.
//
// The chain is drawn as nested rows with a CSS border for the vertical rule
// (DESIGN.md law 3, as amended 2026-07-30). Depth is carried as a number and the
// template nests accordingly; every id, mark, reason and status in it is
// selectable text.
type ChainRow struct {
	Depth int

	// Class is the row's leading span class: id (this ledger's own id),
	// id-q (an ancestor), id-w (a ref this ledger cannot open), or no (a
	// refusal). A refusal is drawn in --ink-low and never in --caught: it is
	// a record, not an alarm (DESIGN.md law 1).
	Class string
	Lead  string

	// Text is the middle column and Right the status margin. Right is empty
	// on rows that have no state to report.
	Text  string
	Right string

	// TextClass is txt (ink) for an entry title and dim (ink-mid) for the
	// reason under a refusal.
	TextClass string

	// opens and closes are how many .node wrappers this row opens before
	// itself and closes after itself. The vertical rule in the chain is a
	// CSS border on those wrappers (DESIGN.md law 3 as amended), so nesting
	// is what draws the tree — and a flat list with a depth number cannot be
	// nested by a template that has no recursion. Computed in Go, emitted as
	// constant markup, so no ledger value ever reaches the template unescaped.
	opens, closes int
}

// OpenTags and CloseTags are the .node wrappers around a chain row. They are
// template.HTML because they are generated from an integer and a constant
// string — no entry text passes through either.
func (r ChainRow) OpenTags() template.HTML {
	return template.HTML(strings.Repeat(`<div class="node">`, r.opens))
}

// CloseTags closes what OpenTags opened, for this row and any deeper ones that
// ended with it.
func (r ChainRow) CloseTags() template.HTML {
	return template.HTML(strings.Repeat(`</div>`, r.closes))
}

// Indent is the .chain-stack row's class: "", "sub" or "sub2".
func (s StackRow) Class() string {
	switch s.Indent {
	case 0:
		return "row"
	case 1:
		return "row sub"
	default:
		return "row sub2"
	}
}

// A StackRow is one line of the mobile .chain-stack: the same graph in a
// stacked typographic form, carrying the same data in the same order.
type StackRow struct {
	Indent   int // 0, 1 or 2 -> "", "sub", "sub2"
	Mark     string
	Key      string
	Value    string
	Withheld bool
}

// An Alt is one alternative as the page renders it.
//
// There is no upheld variant. `alternatives` records the roads not taken, the
// chosen road is the ruling, and dec-0019 removed the card the mockups used to
// carry for it. One and Grounds are both derived from why_not — the first
// sentence is the always-visible summary dec-0017 makes load-bearing, and the
// rest is what opening the row reveals. Nothing is stored to make that work.
type Alt struct {
	Option    string
	One       string
	Grounds   string
	RevisitIf string

	// Open is the degradation rule, decided here rather than in CSS because
	// CSS cannot count siblings honestly: at six or fewer alternatives every
	// row is open and nothing is hidden; above six every row is closed and
	// the page becomes an index that expands in place (dec-0017, amended by
	// dec-0019).
	Open bool
}

// A KV is one row of the rail's edges card.
type KV struct {
	Key   string
	Value string

	// Href is empty for a target this ledger cannot open. A ref that is not
	// a link is still the whole ref, in text, which is cst-0003's rule: cite
	// the ref, never the text.
	Href string
}

// A Decision is everything the decision page renders.
type Decision struct {
	// Title, Description and Ruling are the page's SEO surface. dec-0010
	// makes these pages the acquisition channel, so the <title> and the
	// description are the entry's own words rather than boilerplate.
	Title       string
	Description string

	Query   string
	ID      string
	Kind    ledger.Kind
	State   ledger.State
	Date    string
	Ruling  string
	Arising *KV

	Chain []ChainRow
	Stack []StackRow

	Because []Para

	Alternatives []Alt

	// NoAlternatives is the sentence a page shows when nothing was weighed.
	// It is the same sentence `dira why` prints, from the same function, so
	// the two renderers cannot describe an empty record differently.
	NoAlternatives string

	Edges []KV

	// Drift is the ledger-wide orphan report: active intents with no
	// derives_from edge. It is the only red on the page (DESIGN.md law 1),
	// and it is derived at read time from the edges, never read from a field.
	Drift []string

	Total int
}

// A Para is one paragraph of an entry's body.
//
// The body is markdown (dec-0002) and this renderer is not a markdown renderer:
// the command path is stdlib-only, and a CommonMark implementation is a
// dependency int-0002's budget has no room for. What it does is the honest
// subset — split on blank lines, and treat a leading run of `#` as a heading —
// so an entry whose body is prose renders as prose. Lists and tables render as
// their literal source text, which is stated here rather than discovered later.
type Para struct {
	Text    string
	Heading bool
}

// BuildDecision turns a chain and its entry into the decision page's view.
func BuildDecision(ctx context.Context, src Source, chain *why.Chain, entry *ledger.Entry) (*Decision, error) {
	d := &Decision{
		Query:  chain.Query,
		ID:     entry.ID,
		Kind:   entry.Kind,
		State:  entry.State,
		Date:   shortDate(chain.Subject.Date),
		Ruling: strings.TrimSpace(entry.Title),
	}

	d.Title = d.Ruling + " — decision record kept with dira"
	d.Description = describe(entry, chain)

	d.Because = paragraphs(entry.Body)

	// The alternatives. Six is the threshold dec-0017 measured, and it is
	// one comparison rather than a CSS trick.
	open := len(chain.Alternatives) <= 6
	for _, a := range chain.Alternatives {
		one, rest := splitGround(a.WhyNot)
		d.Alternatives = append(d.Alternatives, Alt{
			Option:    strings.TrimSpace(a.Option),
			One:       one,
			Grounds:   rest,
			RevisitIf: collapse(a.RevisitIf),
			Open:      open,
		})
	}
	if len(chain.Alternatives) == 0 {
		d.NoAlternatives = why.NoAlternatives(entry.Kind)
	}

	d.Chain, d.Stack = buildChain(chain)

	// The rail. Edges are read off the entry, in the order the file records
	// them, so what the rail shows and what `git diff` shows are the same
	// list. The ADR is a path and never a link: the entry is the record and
	// the ADR is exhaust (dec-0009).
	for _, e := range entry.Edges {
		kv := KV{Key: strings.ReplaceAll(string(e.Type), "_", " "), Value: e.To}
		if e.Type != ledger.EdgeRealizedBy && ledger.ValidID(e.To) {
			kv.Href = "/e/" + e.To
		}
		d.Edges = append(d.Edges, kv)
		if e.Type == ledger.EdgeDerivesFrom && d.Arising == nil {
			arising := KV{Key: e.To, Value: e.Note, Href: kv.Href}
			for _, gen := range chain.Arising {
				for _, n := range gen {
					if n.Ref == e.To && n.Title != "" {
						arising.Value = strings.TrimSpace(n.Title)
					}
				}
			}
			d.Arising = &arising
		}
	}
	if entry.ADR != "" {
		d.Edges = append(d.Edges, KV{Key: "mirrored", Value: entry.ADR})
	}

	drift, total, err := driftAndTotal(ctx, src)
	if err != nil {
		return nil, err
	}
	d.Drift, d.Total = drift, total

	return d, nil
}

// buildChain lays the chain out for both forms: the nested desktop tree and the
// stacked mobile list.
//
// Nothing here reads the ledger. Every value comes off the *why.Chain, which is
// the same structure `dira why` renders, and that is the whole point of the
// split — a second walk would make the "same output" claim a lookalike.
func buildChain(c *why.Chain) ([]ChainRow, []StackRow) {
	var rows []ChainRow
	var stack []StackRow

	depth := 0
	for _, generation := range c.Arising {
		for _, n := range generation {
			rows = append(rows, ChainRow{
				Depth: depth, Class: refClass(n), Lead: n.Ref,
				Text: strings.TrimSpace(n.Title), TextClass: "txt", Right: nodeStatus(n),
			})
			stack = append(stack, StackRow{
				Indent: min(depth, 2), Key: n.Ref,
				Value: strings.TrimSpace(n.Title), Withheld: n.Resolution != why.Oriented,
			})
		}
		depth++
	}

	rows = append(rows, ChainRow{
		Depth: depth, Class: "id", Lead: c.Subject.Ref,
		Text: strings.TrimSpace(c.Subject.Title), TextClass: "txt", Right: nodeStatus(c.Subject),
	})
	stack = append(stack, StackRow{Indent: min(depth, 2), Key: c.Subject.Ref, Value: strings.TrimSpace(c.Subject.Title)})
	depth++

	// The refusals. A mark and no colour: a rejected alternative is a record,
	// not an alarm, and --caught is reserved for drift and contradiction.
	// There is no ✓ row, because there is no upheld alternative to draw one
	// for and `dira why` prints none either (dec-0019).
	const chainRefusals = 4
	shown := c.Alternatives
	if len(shown) > chainRefusals {
		shown = shown[:chainRefusals]
	}
	for _, a := range shown {
		one, _ := splitGround(a.WhyNot)
		rows = append(rows, ChainRow{
			Depth: depth, Class: "no", Lead: "✗ " + strings.TrimSpace(a.Option),
			Text: one, TextClass: "dim",
		})
		stack = append(stack, StackRow{Indent: min(depth, 2), Mark: "✗", Value: strings.TrimSpace(a.Option)})
	}
	if rest := len(c.Alternatives) - len(shown); rest > 0 {
		// The same disclosure idiom the alternatives block uses, so the
		// instrument half and the prose half speak one grammar.
		label := fmt.Sprintf("%d further refusals on record", rest)
		rows = append(rows, ChainRow{Depth: depth, Class: "lo", Lead: "…", Text: label, TextClass: "lo"})
		stack = append(stack, StackRow{Indent: min(depth, 2), Mark: "…", Value: label})
	}

	// realized_by, verbatim. dira does not ask kazi whether this converged
	// and must not imply that it did (dec-0004), so the row carries the
	// target and the edge's own note and nothing else.
	for _, a := range c.Realized {
		rows = append(rows, ChainRow{
			Depth: depth + 1, Class: "id", Lead: a.Target,
			Text: a.Note, TextClass: "dim",
		})
		stack = append(stack, StackRow{Indent: 2, Key: a.Target, Value: a.Note})
	}

	return nest(rows), stack
}

// nest turns the flat depth-numbered rows into the .node wrappers that draw the
// tree. It is a separate pass so buildChain stays a statement about what the
// chain contains rather than about how it is boxed.
func nest(rows []ChainRow) []ChainRow {
	depth := 0
	for i := range rows {
		for depth < rows[i].Depth {
			rows[i].opens++
			depth++
		}
		for depth > rows[i].Depth {
			rows[i].opens-- // never negative: opens is only incremented above
			depth--
		}
		// A row shallower than the one before it closes the wrappers the
		// deeper rows opened. Walking forward cannot know that, so the
		// close count is settled in the second loop below.
	}
	depth = 0
	for i := range rows {
		for depth < rows[i].Depth {
			depth++
		}
		if depth > rows[i].Depth {
			rows[i-1].closes += depth - rows[i].Depth
			depth = rows[i].Depth
		}
	}
	if len(rows) > 0 {
		rows[len(rows)-1].closes += depth
	}
	return rows
}

// refClass picks the leading span's class for a chain row. A ref this ledger
// cannot open is drawn in the instrument hue like any other ref — the needle is
// still on it — and says so in words in the status column rather than with an
// alarm (DESIGN.md law 1; only orphan is drift).
func refClass(n why.Node) string {
	if n.Resolution != why.Oriented {
		return "id-w"
	}
	return "id-q"
}

// nodeStatus is the chain's right margin: the entry's ledger state and its date.
// It is never an execution state — dira does not have one to report (dec-0004).
func nodeStatus(n why.Node) string {
	if n.Resolution != why.Oriented {
		return "not in this ledger"
	}
	return strings.TrimSpace(string(n.State) + " " + shortDate(n.Date))
}

// driftAndTotal derives the orphan report and the ledger size.
//
// An orphan is an ACTIVE INTENT with no derives_from edge: work with no stated
// purpose. That is the one drift this surface reports, and it is a join over
// edges computed here rather than a field anybody wrote down.
func driftAndTotal(ctx context.Context, src Source) ([]string, int, error) {
	all, err := src.Select(ctx, index.Selector{})
	if err != nil {
		return nil, 0, err
	}
	var orphans []string
	for _, ref := range all {
		if ref.Kind != ledger.KindIntent || ref.State != ledger.StateActive {
			continue
		}
		entry, err := src.Entry(ctx, ref.ID)
		if err != nil {
			return nil, 0, err
		}
		if !hasEdge(entry, ledger.EdgeDerivesFrom) {
			orphans = append(orphans, ref.ID)
		}
	}
	sort.Strings(orphans)
	return orphans, len(all), nil
}

func hasEdge(e *ledger.Entry, t ledger.EdgeType) bool {
	for _, edge := range e.Edges {
		if edge.Type == t {
			return true
		}
	}
	return false
}

// describe writes the meta description: the entry's own claim plus the count of
// roads it refused. dec-0010 makes this page the acquisition surface and a
// description nobody wrote is a description nobody clicks.
func describe(e *ledger.Entry, c *why.Chain) string {
	var b strings.Builder
	b.WriteString(e.ID)
	b.WriteString(" — ")
	b.WriteString(string(e.State))
	b.WriteString(". ")
	switch n := len(c.Alternatives); n {
	case 0:
		b.WriteString("No alternatives recorded.")
	case 1:
		b.WriteString("One alternative on record, with its grounds.")
	default:
		fmt.Fprintf(&b, "%d alternatives on record, each with its grounds.", n)
	}
	b.WriteString(" A decision record kept automatically by a coding agent.")
	return b.String()
}

// ---------------------------------------------------------------------------
// Derived text
// ---------------------------------------------------------------------------

// splitGround divides a why_not into the one-line summary the always-visible
// summary row carries and the rest, which opening the row reveals.
//
// dec-0017 makes the one-line ground load-bearing — "the summary line carries
// the grounds, not just the title" — and there is no field for it. There does
// not need to be one: the first sentence IS the author's own opening claim, and
// deriving it stores nothing and duplicates nothing (dec-0019).
//
// A why_not of one sentence yields that sentence and an empty detail, which is
// correct: there is nothing further to reveal, and a <details> that opens onto
// nothing is worse than one that opens onto the whole ground.
func splitGround(whyNot string) (one, rest string) {
	text := collapse(whyNot)
	if text == "" {
		return "", ""
	}
	runes := []rune(text)
	for i := 0; i < len(runes)-1; i++ {
		if runes[i] != '.' || runes[i+1] != ' ' {
			continue
		}
		// A sentence ends at ". " only when what follows starts a new
		// one. Without this an abbreviation or a version number splits
		// the summary mid-clause.
		j := i + 2
		for j < len(runes) && runes[j] == ' ' {
			j++
		}
		if j >= len(runes) || !unicode.IsUpper(runes[j]) {
			continue
		}
		return strings.TrimSpace(string(runes[:i+1])), strings.TrimSpace(string(runes[j:]))
	}
	return text, ""
}

// collapse normalises whitespace. Ledger prose arrives hand-wrapped inside
// folded YAML scalars, so its line breaks are an artifact of the width the file
// was written at rather than of what the text means.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// paragraphs splits an entry body on blank lines. See Para for what this
// deliberately does not do.
func paragraphs(body string) []Para {
	var out []Para
	for _, block := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n\n") {
		text := strings.TrimSpace(block)
		if text == "" {
			continue
		}
		heading := false
		if strings.HasPrefix(text, "#") {
			heading = true
			text = strings.TrimSpace(strings.TrimLeft(text, "#"))
		}
		out = append(out, Para{Text: collapse(text), Heading: heading})
	}
	return out
}

// shortDate takes the date part of an RFC3339 stamp. The chain does not shorten
// beyond this: a date shortened further could not be lengthened by a renderer
// that wanted the whole thing.
func shortDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
