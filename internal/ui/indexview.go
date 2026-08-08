package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
)

// A Row is one entry as the index lists it.
type Row struct {
	ID    string
	Title string

	// Status is a LEDGER state or a ledger-side join, never an execution
	// state. See Index.StatusNote.
	Status string

	// StatusClass is done (--converged, for the states tokens.css sanctions
	// that hue for: accepted and achieved), gap (--ink-mid) for a settled
	// question or an abandoned entry, or empty for everything else.
	StatusClass string
}

// A Group is one intent and the entries that arise from it. The index groups by
// WHY, not by goal — the view a goal-shaped tracker structurally cannot give.
type Group struct {
	ID    string
	Title string

	// Settled and Moving are the roll-ups, and they are joins rather than
	// fields: nothing in any entry file records them (dec-0004).
	Settled int
	Moving  int

	Rows []Row
}

// An Index is everything the ledger index renders.
type Index struct {
	Title  string
	Ledger string
	Total  int
	Latest string

	Groups []Group

	// Unparented holds entries that arise from no intent in this ledger.
	// Every id under .dira/entries/ appears on this page, and an entry with
	// no recorded parent is exactly the thing this surface exists to make
	// visible rather than the thing it quietly drops.
	Unparented []Row

	// The dial and its legend. Percent is Settled over Tracked.
	Settled int
	Moving  int
	Staged  int
	Open    int
	Tracked int
	Percent int

	// SettledArc and MovingArc are stroke-dasharray values over a
	// circumference of 314.16 (r=50), and MovingRotate is the second arc's
	// start angle in degrees. They are computed rather than written down so
	// the picture and the counts cannot disagree — the dial's aria-label
	// reports the same numbers as the visible legend.
	SettledArc   string
	MovingArc    string
	MovingRotate int

	// StatusNote is dec-0004's degradation sentence. dira owns ledger states
	// and kazi owns run states; every execution bucket is the join of the
	// two across realized_by edges, and dira embeds no kazi client (dec-0003)
	// so there is no join to make here yet. The page says so rather than
	// filling the gap with a guess.
	StatusNote string

	Drift []string
}

// The dial's geometry. r=50 in a 120x120 viewBox, so the circumference is
// 2*pi*50. The mockup writes 314.16 literally; this computes from the same
// radius so a token change to the dial cannot silently desynchronise the arcs.
const dialCircumference = 314.159265358979

// BuildIndex assembles the ledger index.
//
// Every roll-up on this page is derived here, at read time, from states and
// edges. Nothing is read from a status field, because there is no status field
// — dec-0004 makes that structural rather than a convention.
func BuildIndex(ctx context.Context, src Source, name string) (*Index, error) {
	refs, err := src.Select(ctx, index.Selector{})
	if err != nil {
		return nil, err
	}

	ix := &Index{
		Title:  name + " — decision memory · dira",
		Ledger: name,
		Total:  len(refs),
		StatusNote: "Execution status is a join with kazi and is derived at read time, never stored. " +
			"No kazi join is available here, so these are the ledger's own states.",
	}

	byID := make(map[string]index.Ref, len(refs))
	parents := make(map[string][]string) // parent id -> child ids
	claimed := make(map[string]bool)
	blocked := make(map[string]bool)
	realized := make(map[string]bool)

	for _, ref := range refs {
		byID[ref.ID] = ref
		if ref.Updated > ix.Latest {
			ix.Latest = ref.Updated
		}
		if ref.Created > ix.Latest {
			ix.Latest = ref.Created
		}
	}

	// One pass over the files for the edges. Select answers which entries an
	// answer contains; the files answer what any of them says, and an edge is
	// something an entry says.
	for _, ref := range refs {
		entry, err := src.Entry(ctx, ref.ID)
		if err != nil {
			return nil, err
		}
		for _, e := range entry.Edges {
			switch e.Type {
			case ledger.EdgeDerivesFrom:
				if p, ok := byID[e.To]; ok && p.Kind == ledger.KindIntent {
					parents[e.To] = append(parents[e.To], ref.ID)
					claimed[ref.ID] = true
				}
			case ledger.EdgeBlocks:
				if ref.Kind == ledger.KindQuestion && ref.State == ledger.StateOpen {
					blocked[e.To] = true
				}
			case ledger.EdgeRealizedBy:
				realized[ref.ID] = true
			}
		}
		if ref.Kind == ledger.KindIntent && ref.State == ledger.StateActive && !hasEdge(entry, ledger.EdgeDerivesFrom) {
			ix.Drift = append(ix.Drift, ref.ID)
		}
	}
	sort.Strings(ix.Drift)

	row := func(ref index.Ref) Row {
		r := Row{ID: ref.ID, Title: strings.TrimSpace(ref.Title), Status: string(ref.State)}
		switch {
		case blocked[ref.ID]:
			r.Status, r.StatusClass = "blocked by a question", "gap"
		case ref.State == ledger.StateAccepted || ref.State == ledger.StateAchieved:
			r.StatusClass = "done"
			if !realized[ref.ID] {
				r.Status = string(ref.State) + " · to be planned"
			}
		case ref.State == ledger.StateStaged || ref.State == ledger.StateAbandoned ||
			ref.State == ledger.StateSuperseded || ref.State == ledger.StateRejected:
			r.StatusClass = "gap"
		}
		return r
	}

	// Intent headings, oldest first, so the ledger reads in the order it was
	// laid down rather than newest-noise first.
	var intents []index.Ref
	for _, ref := range refs {
		if ref.Kind == ledger.KindIntent {
			intents = append(intents, ref)
		}
	}
	sort.Slice(intents, func(i, j int) bool { return intents[i].ID < intents[j].ID })

	for _, in := range intents {
		g := Group{ID: in.ID, Title: strings.TrimSpace(in.Title)}
		kids := parents[in.ID]
		sort.Strings(kids)
		for _, id := range kids {
			r := row(byID[id])
			if r.StatusClass == "done" {
				g.Settled++
			} else {
				g.Moving++
			}
			g.Rows = append(g.Rows, r)
		}
		ix.Groups = append(ix.Groups, g)
	}

	for _, ref := range refs {
		if ref.Kind == ledger.KindIntent || claimed[ref.ID] {
			continue
		}
		ix.Unparented = append(ix.Unparented, row(ref))
	}
	sort.Slice(ix.Unparented, func(i, j int) bool { return ix.Unparented[i].ID < ix.Unparented[j].ID })

	for _, ref := range refs {
		switch {
		case ref.Kind == ledger.KindQuestion && ref.State == ledger.StateOpen:
			ix.Open++
		case ref.State == ledger.StateStaged:
			ix.Staged++
		case ref.State == ledger.StateAccepted || ref.State == ledger.StateAchieved || ref.State == ledger.StateAnswered:
			ix.Settled++
		case ref.State == ledger.StateActive:
			ix.Moving++
		}
	}
	ix.Tracked = ix.Settled + ix.Moving + ix.Staged + ix.Open
	if ix.Tracked > 0 {
		ix.Percent = ix.Settled * 100 / ix.Tracked
	}
	ix.SettledArc = arc(ix.Settled, ix.Tracked)
	ix.MovingArc = arc(ix.Moving, ix.Tracked)
	ix.MovingRotate = -90 + int(float64(ix.Settled)/max(float64(ix.Tracked), 1)*360)
	ix.Latest = shortDate(ix.Latest)

	return ix, nil
}

// arc renders one ring segment as a stroke-dasharray: the drawn length, then
// the gap that completes the circle.
func arc(n, total int) string {
	if total <= 0 {
		return "0 " + fmt.Sprintf("%.1f", dialCircumference)
	}
	drawn := dialCircumference * float64(n) / float64(total)
	return fmt.Sprintf("%.1f %.1f", drawn, dialCircumference-drawn)
}

// DialLabel is the dial's accessible description. It is built from the same
// fields the visible legend renders, so the two cannot report different numbers.
func (ix *Index) DialLabel() string {
	return fmt.Sprintf("Of %d tracked entries: %d settled, %d active, %d staged, %d open questions",
		ix.Tracked, ix.Settled, ix.Moving, ix.Staged, ix.Open)
}
