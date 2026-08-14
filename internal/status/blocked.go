package status

import (
	"context"
	"fmt"
	"sort"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
)

// DeriveDecisionBlocked returns one Row per entry gated by an open
// question's blocks edge, found via ix.In — never by reading the gated
// entry's own edges, which do not carry it: edges are stored on the
// declaring entry (dec-0002), and the entry that declares a blocks edge is
// the question, not the entry it blocks. An implementation that inspects
// only the candidate's own Edges field finds nothing and would report
// ToBePlanned instead — the precise failure this bucket exists to prevent.
// See TestDecisionBlocked's naiveDecisionBlockedByOwnEdges control.
//
// Named DeriveDecisionBlocked rather than DecisionBlocked — the name the
// lane's task doc fixes for it — for the same reason DeriveToBePlanned is:
// DecisionBlocked is already the Bucket constant in types.go (T1), and a
// function cannot share a top-level name with a constant in the same
// package.
//
// An entry gated by more than one open question produces one Row per
// question rather than one Row picking arbitrarily, so BlockedBy stays a
// single pointer (types.go) instead of a slice: two Rows sharing an ID is
// how "neither question is silently dropped" holds.
//
// The candidate's own state must also be live: an entry that is superseded,
// rejected, achieved or abandoned is not "blocked" by anything, whatever
// still points at it — it is already at rest. dec-0004's row names
// "superseded" explicitly; the others are the same terminal shape for the
// other kinds a blocks edge could in principle gate.
func DeriveDecisionBlocked(ctx context.Context, ix *index.Index) ([]Row, error) {
	refs, err := ix.Select(ctx, index.Selector{})
	if err != nil {
		return nil, fmt.Errorf("status: listing entries: %w", err)
	}

	var out []Row
	for _, ref := range refs {
		if isAtRest(ref.State) {
			continue
		}
		links, err := ix.In(ctx, ref.ID)
		if err != nil {
			return nil, fmt.Errorf("status: backlinks for %s: %w", ref.ID, err)
		}
		for _, link := range links {
			if link.Type != ledger.EdgeBlocks {
				continue
			}
			question, err := ix.Entry(ctx, link.From)
			if err != nil {
				return nil, fmt.Errorf("status: reading %s: %w", link.From, err)
			}
			if question.State != ledger.StateOpen {
				continue
			}
			out = append(out, Row{
				ID:        ref.ID,
				Kind:      ref.Kind,
				Title:     ref.Title,
				Bucket:    DecisionBlocked,
				Source:    SourceLedger,
				BlockedBy: &BlockingQuestion{ID: question.ID, Title: question.Title},
			})
		}
	}

	// Total order: by the blocked entry's id, then by the blocking
	// question's id, so two questions blocking the same entry come back in
	// the same order every run rather than in ix.In's incidental order.
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].BlockedBy.ID < out[j].BlockedBy.ID
	})
	return out, nil
}

// isAtRest reports whether a state describes an entry that has already
// reached a lifecycle end for its kind, independent of anything that once
// pointed at it.
func isAtRest(s ledger.State) bool {
	switch s {
	case ledger.StateSuperseded, ledger.StateRejected, ledger.StateAchieved, ledger.StateAbandoned:
		return true
	}
	return false
}
