// Package cli holds `dira map`'s derivation and rendering logic —
// grouping by `derives_from`, per-intent roll-ups, the six CLI labels,
// `--json` encoding and the degradation line. cmd/dira/map.go is the thin
// wrapper: flag parsing, opening the ledger, calling kazi.Snapshot, and one
// call into this package, mirroring the command/logic split
// cmd/dira/brief.go and internal/brief already use.
//
// E4-L5-T5 draws a structural boundary around this package: it, and
// internal/status and internal/kazi beneath it, never import
// internal/ledger/local. Only cmd/dira/map.go — the wrapper that already
// holds write-capable filesystem access by construction — may.
package cli

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/kazi"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/status"
)

// Render is the single entry point cmd/dira/map.go calls: build the tree
// and write it, in text or --json form, including the one degradation line
// when kazi could not be asked. observedAt is the wall-clock time the
// caller stamps the run with (cmd/dira/map.go's a.now()) — the single
// top-level field --json's determinism clause (E4-L4-T5) is checked with
// removed before comparing two runs.
func Render(ctx context.Context, w io.Writer, ix *index.Index, snap *kazi.Portfolio, snapErr error, statusFn status.StatusFunc, jsonOut bool, observedAt string) error {
	tree, err := BuildTree(ctx, ix, snap, snapErr, statusFn)
	if err != nil {
		return err
	}
	if jsonOut {
		return RenderJSON(w, tree, snapErr, observedAt)
	}
	return RenderText(w, tree, snapErr)
}

// Node is one ledger entry's position in the map tree.
type Node struct {
	ID    string
	Kind  ledger.Kind
	Title string

	// Bucket is one of dec-0004's six values, or the zero value for an
	// entry this join table does not cover at all (a question, a
	// constraint, a note, or a decision/intent already at rest — rejected,
	// superseded, achieved, abandoned).
	Bucket status.Bucket

	// BlockedBy is set only when Bucket == status.DecisionBlocked.
	BlockedBy *status.BlockingQuestion

	// Evidence, Ambiguous and Unresolved mirror status.Row's own fields —
	// E4-L3's join output, carried through unchanged.
	Evidence   *status.KaziEvidence
	Ambiguous  *status.AmbiguousDetail
	Unresolved *status.UnresolvedDetail

	// BlocksTarget is the id this entry's own `blocks` edge names, if any
	// — typically an open question's row, rendered per docs/design.md
	// §6.4's "⛔ blocks <id>" line, independent of Bucket.
	BlocksTarget string
}

// Group is one derives_from parent's subtree — one level only, per
// docs/plan/lanes/E4.md's boundary: "this lane groups one level, using
// ix.In/Entry.Edges directly rather than internal/why's Build."
type Group struct {
	Parent   *Node
	Children []*Node

	// Rollup counts each of Parent's direct children by Bucket, excluding
	// children with the zero-value Bucket (not one of the six) and
	// excluding grandchildren entirely — a child that is itself a group's
	// Parent elsewhere contributes to THIS roll-up as one unit.
	Rollup map[status.Bucket]int
}

// Tree is the whole map: one Group per distinct derives_from target that at
// least one entry names, plus every entry that names none (or names a
// target absent from the ledger — a dangling edge) in Unparented.
type Tree struct {
	Groups     []*Group
	Unparented []*Node
}

// BuildTree derives the whole map: every ledger entry's dec-0004 bucket
// (where applicable) and its position in the derives_from tree.
//
// snap/snapErr/statusFn are exactly status.Join's own parameters — the
// caller (cmd/dira/map.go) obtains them from kazi.Snapshot and kazi.Status
// and passes them straight through, so this package never imports
// internal/kazi's process-shelling seam itself.
func BuildTree(ctx context.Context, ix *index.Index, snap *kazi.Portfolio, snapErr error, statusFn status.StatusFunc) (*Tree, error) {
	refs, err := ix.Select(ctx, index.Selector{})
	if err != nil {
		return nil, fmt.Errorf("cli: listing entries: %w", err)
	}
	ids := make([]string, len(refs))
	for i, r := range refs {
		ids[i] = r.ID
	}
	entries, err := ix.Entries(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("cli: reading entries: %w", err)
	}
	entryByID := make(map[string]*ledger.Entry, len(entries))
	for _, e := range entries {
		entryByID[e.ID] = e
	}

	// The three ledger-and-kazi-derived sources of a Bucket, computed once
	// each rather than per entry.
	var candidates []*ledger.Entry
	for _, e := range entries {
		if isJoinCandidate(e) && hasEdgeType(e, ledger.EdgeRealizedBy) {
			candidates = append(candidates, e)
		}
	}
	joined, err := status.Join(ctx, candidates, snap, snapErr, statusFn)
	if err != nil {
		return nil, fmt.Errorf("cli: joining kazi execution status: %w", err)
	}
	joinedByID := make(map[string]status.Row, len(joined))
	for _, r := range joined {
		joinedByID[r.ID] = r
	}

	toBePlanned, err := status.DeriveToBePlanned(ctx, ix)
	if err != nil {
		return nil, fmt.Errorf("cli: deriving to-be-planned: %w", err)
	}
	toBePlannedByID := make(map[string]bool, len(toBePlanned))
	for _, r := range toBePlanned {
		toBePlannedByID[r.ID] = true
	}

	decisionBlocked, err := status.DeriveDecisionBlocked(ctx, ix)
	if err != nil {
		return nil, fmt.Errorf("cli: deriving decision-blocked: %w", err)
	}
	// An entry blocked by more than one open question produces one Row per
	// question (status.DeriveDecisionBlocked's own contract). This tree
	// renders one node per entry, so the first — status's own total order,
	// by blocking question id — wins; the others are not silently
	// contradicted, just not this lane's concern (E4-L4 groups by parent,
	// not by every gating question).
	blockedByID := make(map[string]*status.BlockingQuestion, len(decisionBlocked))
	for _, r := range decisionBlocked {
		if _, exists := blockedByID[r.ID]; !exists {
			blockedByID[r.ID] = r.BlockedBy
		}
	}

	nodes := make(map[string]*Node, len(entries))
	for _, e := range entries {
		n := &Node{ID: e.ID, Kind: e.Kind, Title: e.Title}
		switch {
		case blockedByID[e.ID] != nil:
			n.Bucket = status.DecisionBlocked
			n.BlockedBy = blockedByID[e.ID]
		default:
			if row, ok := joinedByID[e.ID]; ok {
				n.Bucket = row.Bucket
				n.Evidence = row.Evidence
				n.Ambiguous = row.Ambiguous
				n.Unresolved = row.Unresolved
			} else if toBePlannedByID[e.ID] {
				n.Bucket = status.ToBePlanned
			}
		}
		if target, ok := firstEdgeTarget(e, ledger.EdgeBlocks); ok {
			n.BlocksTarget = target
		}
		nodes[e.ID] = n
	}

	groupsByParent := make(map[string][]*Node)
	var unparented []*Node
	for _, e := range entries {
		n := nodes[e.ID]
		target, ok := firstEdgeTarget(e, ledger.EdgeDerivesFrom)
		if !ok {
			unparented = append(unparented, n)
			continue
		}
		if _, exists := entryByID[target]; !exists {
			// A dangling derives_from target — schema/entry.schema.json
			// does not enforce that it resolves. Routed to Unparented
			// rather than dropped: dropping it would break count
			// conservation, the exact defect T2's own control proves.
			unparented = append(unparented, n)
			continue
		}
		groupsByParent[target] = append(groupsByParent[target], n)
	}

	groups := make([]*Group, 0, len(groupsByParent))
	for parentID, children := range groupsByParent {
		sort.Slice(children, func(i, j int) bool { return children[i].ID < children[j].ID })
		groups = append(groups, &Group{
			Parent:   nodes[parentID],
			Children: children,
			Rollup:   rollup(children),
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Parent.ID < groups[j].Parent.ID })
	sort.Slice(unparented, func(i, j int) bool { return unparented[i].ID < unparented[j].ID })

	return &Tree{Groups: groups, Unparented: unparented}, nil
}

// rollup counts children by Bucket, skipping the zero value — an entry
// dec-0004's table does not cover contributes nothing to a parent's roll-up
// line.
func rollup(children []*Node) map[status.Bucket]int {
	counts := make(map[status.Bucket]int)
	for _, c := range children {
		if c.Bucket == "" {
			continue
		}
		counts[c.Bucket]++
	}
	return counts
}

// isJoinCandidate reports whether e is a kind/state status.Join concerns
// itself with — the same accepted-decision/active-intent restriction
// DeriveToBePlanned applies, so a realized_by edge on an entry already at
// rest (rejected, superseded, achieved, abandoned) is not joined against
// kazi at all.
func isJoinCandidate(e *ledger.Entry) bool {
	switch {
	case e.Kind == ledger.KindDecision && e.State == ledger.StateAccepted:
		return true
	case e.Kind == ledger.KindIntent && e.State == ledger.StateActive:
		return true
	}
	return false
}

// hasEdgeType reports whether e declares an outgoing edge of type t.
func hasEdgeType(e *ledger.Entry, t ledger.EdgeType) bool {
	_, ok := firstEdgeTarget(e, t)
	return ok
}

// firstEdgeTarget returns the To field of e's first outgoing edge of type t.
func firstEdgeTarget(e *ledger.Entry, t ledger.EdgeType) (string, bool) {
	for _, edge := range e.Edges {
		if edge.Type == t {
			return edge.To, true
		}
	}
	return "", false
}

// TotalCount returns the number of entries appearing anywhere in tree — the
// count-conservation property T2's acc line demands: it must equal the
// ledger's total entry count.
func (t *Tree) TotalCount() int {
	n := len(t.Unparented)
	for _, g := range t.Groups {
		n += len(g.Children)
	}
	return n
}
