package status

import (
	"context"
	"fmt"
	"sort"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
)

// DeriveToBePlanned returns one Row per accepted decision or active intent
// carrying no outgoing realized_by edge.
//
// Named DeriveToBePlanned rather than ToBePlanned — the name the lane's task
// doc fixes for it — because that name is already taken in this package: the
// ToBePlanned Bucket constant (types.go, T1) is a top-level identifier in the
// same package, and Go does not allow a function and a constant to share one.
// DecisionBlocked (blocked.go) has the identical collision with its own
// Bucket constant and is renamed the same way; Terminal has no such collision
// and keeps the name the task doc gives it.
//
// An entry that DOES carry a realized_by edge is excluded even when nothing
// else in this lane can say what it resolves to. That is deliberate: it is
// not to-be-planned, it is unknown until E4-L3 joins it, and including it
// here because "nothing else claimed it yet" would make E4-L3's
// degraded-join guarantee — an entry with realized_by is never demoted to
// ToBePlanned when kazi is unavailable — impossible to hold, because this
// function would already have put it there.
func DeriveToBePlanned(ctx context.Context, ix *index.Index) ([]Row, error) {
	var refs []index.Ref
	for _, sel := range []index.Selector{
		{Kinds: []ledger.Kind{ledger.KindDecision}, States: []ledger.State{ledger.StateAccepted}},
		{Kinds: []ledger.Kind{ledger.KindIntent}, States: []ledger.State{ledger.StateActive}},
	} {
		got, err := ix.Select(ctx, sel)
		if err != nil {
			return nil, fmt.Errorf("status: selecting to-be-planned candidates: %w", err)
		}
		refs = append(refs, got...)
	}

	out := make([]Row, 0, len(refs))
	for _, ref := range refs {
		entry, err := ix.Entry(ctx, ref.ID)
		if err != nil {
			return nil, fmt.Errorf("status: reading %s: %w", ref.ID, err)
		}
		if hasEdgeType(entry, ledger.EdgeRealizedBy) {
			continue
		}
		out = append(out, Row{
			ID:     entry.ID,
			Kind:   entry.Kind,
			Title:  entry.Title,
			Bucket: ToBePlanned,
			Source: SourceLedger,
		})
	}

	// The order is total (ID ascending) rather than whatever order the two
	// Select calls happened to return, so calling this twice over the same
	// fixture gives the same answer both times.
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// hasEdgeType reports whether e declares an outgoing edge of type t.
func hasEdgeType(e *ledger.Entry, t ledger.EdgeType) bool {
	for _, edge := range e.Edges {
		if edge.Type == t {
			return true
		}
	}
	return false
}
