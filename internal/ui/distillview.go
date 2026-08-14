package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/kazi-org/dira/internal/distill"
	"github.com/kazi-org/dira/internal/ledger"
)

// A DistillCard is one card in the deck: a staged entry plus which parts of it
// are safe to draw. dec-0019 applies here exactly as it does on the decision
// page — a field is drawn when the entry recorded it and left out otherwise,
// never invented to fill the gap a regex-tier capture left open. A card built
// from an entry with no body and no alternatives (the shape `dira sniff`
// actually writes) therefore carries an empty Because, a nil Against and no
// DerivesFrom/ADR, and the template omits each block rather than rendering it
// hollow.
type DistillCard struct {
	ID string

	// State is always "staged" — Staged() guarantees it (dec-0021) — carried
	// as a field rather than a literal so the chip and the entry's own
	// `state:` can never read differently by construction.
	State string

	// Src is "hook · tier" (e.g. "Stop · regex"), joining only what the entry
	// actually carries a Source for — "" when it has none at all, never a
	// bare " · " for a half-populated one.
	Src string

	Because []Para

	// Body is the entry's raw markdown body, unrendered — the exact text
	// T4's edit textarea is seeded with. Because is the rendered form of the
	// same field (paragraphs(e.Body)); this is the source the human's
	// keystrokes overwrite, and it must be the bytes that would round-trip
	// through EditBody unchanged if resubmitted verbatim.
	Body string

	// Title is the ruling this card decides. "Decides" is what the mockup
	// labels it; every card in this deck is a decision (Staged only ever
	// returns `kind: decision`, dec-0021), so the label is not a per-card
	// field.
	Title string

	// Against is the one alternative shown struck through — the first on
	// record, the same role s3-distill.html's mockup always gave it. nil
	// when the entry has none yet.
	Against *DistillAgainst

	DerivesFrom string // the parent id, or "" when the edge is absent
	ADR         string // the mirrored path, or "" when the entry has none

	// Actionable is true for exactly one card: the deck's own rule, stated in
	// s3-distill.html's own comment, is that only the top card takes input.
	// It is computed once here rather than left for the template to infer
	// from position, so there is one place this can be wrong instead of two.
	Actionable bool
}

// DistillAgainst is the struck-through alternative a card shows for context.
type DistillAgainst struct {
	Option string
	WhyNot string
}

// A Distill is everything the /distill page renders.
type Distill struct {
	Ledger string
	Total  int
	Cards  []DistillCard

	// Empty is DESIGN.md's r2->r3 record: one sentence, zero filled buttons,
	// shown instead of the deck when there is nothing to review — never the
	// last card left frozen on screen (the defect that sentence replaced).
	Empty bool

	// EmptyMessage is that sentence, verbatim, so the template does not carry
	// its own second copy of DESIGN.md's copy.
	EmptyMessage string

	// Heading is the deck's h1, e.g. "Two decisions from yesterday.",
	// pluralised on Total. Empty when Empty is true — the template picks a
	// different heading for that case.
	Heading string

	// RestSegments is Total-1 long and carries no data — the template ranges
	// over it to draw the progress bar's remaining, unreviewed segments
	// without a template-side arithmetic hack.
	RestSegments []struct{}
}

// emptyMessage is DESIGN.md's r2->r3 record (the "Empty-state copy" block),
// verbatim.
const emptyMessage = "When the queue is empty, nothing is waiting on you — and the ledger is current."

// BuildDistill assembles the /distill deck from the live queue. It calls
// internal/distill.Staged and nothing else that reads a ledger: this lane
// calls the disposition package rather than reimplementing any part of it
// (see internal/distill's package comment and docs/plan/lanes/E6.md's risk).
// name is what the ledger is called on the page, the same string BuildIndex
// takes.
func BuildDistill(ctx context.Context, store ledger.Store, name string) (*Distill, error) {
	q, err := distill.Staged(ctx, store)
	if err != nil {
		return nil, err
	}

	// Awaiting() first, then PendingExtraction() — T1's
	// TestDistillMockupMatchesTheQueue pins this order against the fixture,
	// and it is the same split s3-distill.html's `.stage` / `.stage.next`
	// draws.
	awaiting := q.Awaiting()
	items := make([]distill.Item, 0, q.Len())
	items = append(items, awaiting...)
	items = append(items, q.PendingExtraction()...)

	d := &Distill{Ledger: name, Total: len(items)}
	if d.Total == 0 {
		d.Empty = true
		d.EmptyMessage = emptyMessage
		return d, nil
	}
	d.Heading = heading(d.Total)
	d.RestSegments = make([]struct{}, d.Total-1)
	for i, it := range items {
		// Actionable is position AND status: the first slot is only live
		// when it actually holds an Awaiting() item. If Awaiting() is
		// empty, index 0 is a PendingExtraction() entry — one Confirm and
		// Discard already refuse (dec-0024, dispose.go's own
		// already-confirmed check) — and rendering live buttons on it would
		// offer a keystroke the handlers behind it are going to reject.
		actionable := i == 0 && len(awaiting) > 0
		d.Cards = append(d.Cards, buildCard(it, actionable))
	}
	return d, nil
}

// buildCard turns one queue Item into a DistillCard. actionable is passed in
// rather than derived from it.Status, because "only the top card is
// actionable" is a statement about POSITION in the deck (s3-distill.html's own
// comment), not about an individual entry's disposition status — a queue with
// more than one Awaiting() item still shows only its first card as live.
func buildCard(it distill.Item, actionable bool) DistillCard {
	e := it.Entry
	c := DistillCard{
		ID:         e.ID,
		State:      string(e.State),
		Title:      strings.TrimSpace(e.Title),
		Because:    paragraphs(e.Body),
		Body:       e.Body,
		ADR:        e.ADR,
		Actionable: actionable,
	}
	if e.Source != nil {
		hook, tier := string(e.Source.Hook), string(e.Source.Tier)
		switch {
		case hook != "" && tier != "":
			c.Src = hook + " · " + tier
		case hook != "":
			c.Src = hook
		case tier != "":
			c.Src = tier
		}
	}
	if len(e.Alternatives) > 0 {
		a := e.Alternatives[0]
		c.Against = &DistillAgainst{Option: strings.TrimSpace(a.Option), WhyNot: collapse(a.WhyNot)}
	}
	for _, edge := range e.Edges {
		if edge.Type == ledger.EdgeDerivesFrom {
			c.DerivesFrom = edge.To
			break
		}
	}
	return c
}

// heading spells the deck's count the way s3-distill.html's mockup does for
// the small numbers a review session actually holds, falling back to digits
// above that rather than growing the word list forever.
func heading(n int) string {
	words := [...]string{
		"", "One", "Two", "Three", "Four", "Five", "Six",
		"Seven", "Eight", "Nine", "Ten", "Eleven", "Twelve",
	}
	word := fmt.Sprintf("%d", n)
	if n > 0 && n < len(words) {
		word = words[n]
	}
	noun := "decisions"
	if n == 1 {
		noun = "decision"
	}
	return fmt.Sprintf("%s %s from yesterday.", word, noun)
}
