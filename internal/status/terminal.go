package status

import (
	"context"
	"fmt"
	"sort"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
)

// Terminal returns one TerminalRow per achieved or abandoned intent.
//
// Achieved and abandoned intents are their own groups, reported once and
// never folded into ToBePlanned: an intent that already succeeded or was
// dropped is not waiting on planning. DeriveToBePlanned only ever selects
// active intents, so the two are disjoint by construction — see
// TestTerminal's cross-check, which runs both over the same fixture and
// asserts neither intent this function reports ever appears in the other's
// output.
func Terminal(ctx context.Context, ix *index.Index) ([]TerminalRow, error) {
	var out []TerminalRow
	for _, group := range []struct {
		state ledger.State
		group TerminalGroup
	}{
		{ledger.StateAchieved, Achieved},
		{ledger.StateAbandoned, Abandoned},
	} {
		refs, err := ix.Select(ctx, index.Selector{
			Kinds:  []ledger.Kind{ledger.KindIntent},
			States: []ledger.State{group.state},
		})
		if err != nil {
			return nil, fmt.Errorf("status: selecting %s intents: %w", group.state, err)
		}
		for _, ref := range refs {
			out = append(out, TerminalRow{ID: ref.ID, Title: ref.Title, Group: group.group})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
