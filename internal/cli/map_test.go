package cli_test

import (
	"context"
	"testing"

	"github.com/kazi-org/dira/internal/cli"
	"github.com/kazi-org/dira/internal/kazi"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/status"
)

// --- E4-L4-T2: grouping ------------------------------------------------

// TestMapGrouping is T2's acc line.
func TestMapGrouping(t *testing.T) {
	ctx := context.Background()

	t.Run("every entry carrying derives_from produces zero unparented", func(t *testing.T) {
		// A and B derive from each other, so both carry a derives_from
		// edge and both resolve to a real target — nothing falls through
		// to Unparented.
		a := entry("int-9101", derivesFrom("dec-9102"))
		b := entry("dec-9102", derivesFrom("int-9101"))
		ix := openTree(t, []*ledger.Entry{a, b})

		tree, err := cli.BuildTree(ctx, ix, &kazi.Portfolio{}, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("BuildTree: %v", err)
		}
		if len(tree.Unparented) != 0 {
			t.Errorf("Unparented = %v, want empty", tree.Unparented)
		}
		if tree.TotalCount() != 2 {
			t.Errorf("TotalCount = %d, want 2", tree.TotalCount())
		}
	})

	t.Run("one entry lacking derives_from is unparented, distinct from an empty child list", func(t *testing.T) {
		// x and y derive from each other, so BOTH carry a derives_from edge
		// (neither is itself unparented) while y also serves as the group
		// header for x — exercising "a parent's own entry is not free-
		// floating" without confusing this fixture's ONE lacking-entry
		// premise. z is the fixture's only entry with no derives_from edge
		// at all.
		x := entry("int-9201", derivesFrom("dec-9202"))
		y := entry("dec-9202", derivesFrom("int-9201"))
		z := entry("dec-9203", nil)
		ix := openTree(t, []*ledger.Entry{x, y, z})

		tree, err := cli.BuildTree(ctx, ix, &kazi.Portfolio{}, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("BuildTree: %v", err)
		}
		if len(tree.Unparented) != 1 || tree.Unparented[0].ID != "dec-9203" {
			t.Fatalf("Unparented = %v, want exactly [dec-9203]", tree.Unparented)
		}
		if len(tree.Groups) != 2 {
			t.Fatalf("Groups = %+v, want exactly 2 (x heads a group for y, y heads one for x)", tree.Groups)
		}
	})

	t.Run("count conservation: rendered total equals ledger total, and grows by exactly one", func(t *testing.T) {
		parent := entry("int-9301", nil)
		child := entry("dec-9302", derivesFrom("int-9301"))
		lone := entry("dec-9303", nil)
		ix := openTree(t, []*ledger.Entry{parent, child, lone})

		tree, err := cli.BuildTree(ctx, ix, &kazi.Portfolio{}, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("BuildTree: %v", err)
		}
		if tree.TotalCount() != 3 {
			t.Fatalf("TotalCount = %d, want 3", tree.TotalCount())
		}

		extra := entry("dec-9304", derivesFrom("int-9301"))
		ix2 := openTree(t, []*ledger.Entry{parent, child, lone, extra})
		tree2, err := cli.BuildTree(ctx, ix2, &kazi.Portfolio{}, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("BuildTree: %v", err)
		}
		if got, want := tree2.TotalCount(), tree.TotalCount()+1; got != want {
			t.Errorf("TotalCount after adding one entry = %d, want %d", got, want)
		}
	})

	t.Run("both sides: a dangling derives_from target must not be dropped", func(t *testing.T) {
		dangling := entry("dec-9401", derivesFrom("dec-9999")) // dec-9999 is never written to this fixture ledger
		ix := openTree(t, []*ledger.Entry{dangling})

		// The naive, plausible-looking wrong answer: skip any entry whose
		// derives_from target is not a real, resolved parent.
		naiveCount := func(entries []*ledger.Entry, resolvable map[string]bool) int {
			n := 0
			for _, e := range entries {
				for _, edge := range e.Edges {
					if edge.Type == ledger.EdgeDerivesFrom {
						if resolvable[edge.To] {
							n++
						}
						goto next
					}
				}
				n++ // no derives_from edge: unparented, always counted
			next:
			}
			return n
		}
		if got := naiveCount([]*ledger.Entry{dangling}, map[string]bool{}); got != 0 {
			t.Fatalf("the naive control's own premise broke: got %d, want 0 (dropped)", got)
		}

		tree, err := cli.BuildTree(ctx, ix, &kazi.Portfolio{}, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("BuildTree: %v", err)
		}
		if tree.TotalCount() != 1 {
			t.Fatalf("the real implementation lost the dangling-target entry too: TotalCount = %d, want 1", tree.TotalCount())
		}
		if len(tree.Unparented) != 1 || tree.Unparented[0].ID != "dec-9401" {
			t.Errorf("Unparented = %v, want exactly [dec-9401]", tree.Unparented)
		}
	})
}

// --- E4-L4-T3: per-intent roll-ups --------------------------------------

// TestMapRollups is T3's acc line.
func TestMapRollups(t *testing.T) {
	ctx := context.Background()

	t.Run("a parent's roll-up matches its children's buckets exactly", func(t *testing.T) {
		parent := entry("int-9501", nil)
		completed := entry("dec-9502", combine(derivesFrom("int-9501"), realizedBy("kazi:goal-roll-complete")))
		inProgress := entry("dec-9503", combine(derivesFrom("int-9501"), realizedBy("kazi:goal-roll-progress")))
		blocked := entry("dec-9504", combine(derivesFrom("int-9501"), realizedBy("kazi:goal-roll-blocked")))
		ix := openTree(t, []*ledger.Entry{parent, completed, inProgress, blocked})

		snap := singleRunPortfolio(map[string]kazi.RepoBucket{
			"roll-complete": kazi.RepoComplete,
			"roll-progress": kazi.RepoInProgress,
			"roll-blocked":  kazi.RepoStuck,
		})
		tree, err := cli.BuildTree(ctx, ix, snap, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("BuildTree: %v", err)
		}
		if len(tree.Groups) != 1 {
			t.Fatalf("Groups = %+v, want exactly 1", tree.Groups)
		}
		g := tree.Groups[0]
		want := map[status.Bucket]int{status.Completed: 1, status.InProgress: 1, status.ExecutionBlocked: 1}
		if len(g.Rollup) != len(want) {
			t.Fatalf("Rollup = %v, want %v", g.Rollup, want)
		}
		for b, n := range want {
			if g.Rollup[b] != n {
				t.Errorf("Rollup[%s] = %d, want %d", b, g.Rollup[b], n)
			}
		}
	})

	t.Run("the unparented group has no roll-up of its own, and grandchildren do not flatten upward", func(t *testing.T) {
		grandparent := entry("int-9601", nil) // unparented root
		parent := entry("dec-9602", combine(derivesFrom("int-9601"), realizedBy("kazi:goal-gp-complete")))
		child := entry("dec-9603", combine(derivesFrom("dec-9602"), realizedBy("kazi:goal-gp-progress")))
		ix := openTree(t, []*ledger.Entry{grandparent, parent, child})

		snap := singleRunPortfolio(map[string]kazi.RepoBucket{
			"gp-complete": kazi.RepoComplete,
			"gp-progress": kazi.RepoInProgress,
		})
		tree, err := cli.BuildTree(ctx, ix, snap, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("BuildTree: %v", err)
		}
		if len(tree.Unparented) != 1 || tree.Unparented[0].ID != "int-9601" {
			t.Fatalf("Unparented = %v, want exactly [int-9601]", tree.Unparented)
		}
		if len(tree.Groups) != 2 {
			t.Fatalf("Groups = %+v, want exactly 2 (one per parent)", tree.Groups)
		}
		var gpGroup, parentGroup *cli.Group
		for _, g := range tree.Groups {
			switch g.Parent.ID {
			case "int-9601":
				gpGroup = g
			case "dec-9602":
				parentGroup = g
			}
		}
		if gpGroup == nil || parentGroup == nil {
			t.Fatalf("expected groups headed by int-9601 and dec-9602, got %+v", tree.Groups)
		}
		// int-9601's roll-up counts dec-9602 (Completed) as ONE unit — it
		// must not also carry dec-9603's InProgress, which belongs to
		// dec-9602's own roll-up instead.
		if gpGroup.Rollup[status.Completed] != 1 || gpGroup.Rollup[status.InProgress] != 0 {
			t.Errorf("int-9601's Rollup = %v, want {Completed: 1} only", gpGroup.Rollup)
		}
		if parentGroup.Rollup[status.InProgress] != 1 {
			t.Errorf("dec-9602's Rollup = %v, want {InProgress: 1}", parentGroup.Rollup)
		}
	})

	t.Run("mutation: flipping one child's bucket changes only its own parent's roll-up", func(t *testing.T) {
		parentA := entry("int-9701", nil)
		childA := entry("dec-9702", combine(derivesFrom("int-9701"), realizedBy("kazi:goal-mut-a")))
		parentB := entry("int-9703", nil)
		childB := entry("dec-9704", combine(derivesFrom("int-9703"), realizedBy("kazi:goal-mut-b")))
		ix := openTree(t, []*ledger.Entry{parentA, childA, parentB, childB})

		before := singleRunPortfolio(map[string]kazi.RepoBucket{"mut-a": kazi.RepoComplete, "mut-b": kazi.RepoInProgress})
		beforeTree, err := cli.BuildTree(ctx, ix, before, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("BuildTree: %v", err)
		}
		beforeA := groupFor(t, beforeTree, "int-9701")
		beforeB := groupFor(t, beforeTree, "int-9703")
		if beforeA.Rollup[status.Completed] != 1 {
			t.Fatalf("before: int-9701's Rollup = %v, want {Completed: 1}", beforeA.Rollup)
		}
		if beforeB.Rollup[status.InProgress] != 1 {
			t.Fatalf("before: int-9703's Rollup = %v, want {InProgress: 1}", beforeB.Rollup)
		}

		// Flip goal-mut-a from Complete to InProgress — a mutated ledger
		// copy re-run through Join, not a hand-edited rendered string.
		after := singleRunPortfolio(map[string]kazi.RepoBucket{"mut-a": kazi.RepoInProgress, "mut-b": kazi.RepoInProgress})
		afterTree, err := cli.BuildTree(ctx, ix, after, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("BuildTree: %v", err)
		}
		afterA := groupFor(t, afterTree, "int-9701")
		afterB := groupFor(t, afterTree, "int-9703")
		if afterA.Rollup[status.Completed] != 0 || afterA.Rollup[status.InProgress] != 1 {
			t.Errorf("after: int-9701's Rollup = %v, want {InProgress: 1}", afterA.Rollup)
		}
		if afterB.Rollup[status.InProgress] != 1 || len(afterB.Rollup) != 1 {
			t.Errorf("after: int-9703's Rollup changed to %v, want it unchanged at {InProgress: 1}", afterB.Rollup)
		}
	})
}

func groupFor(t *testing.T, tree *cli.Tree, parentID string) *cli.Group {
	t.Helper()
	for _, g := range tree.Groups {
		if g.Parent.ID == parentID {
			return g
		}
	}
	t.Fatalf("no group headed by %s", parentID)
	return nil
}
