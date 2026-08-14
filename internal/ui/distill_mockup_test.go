package ui

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/kazi-org/dira/internal/distill"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// TestDistillMockupMatchesTheQueue is E6-L3-T1's mechanical proof that
// docs/design/screens/s3-distill.html shows exactly what internal/distill.
// Staged returns for the fixture ledger — Awaiting() first, then
// PendingExtraction() — rather than a hand-picked illustration.
//
// It is deliberately blind to everything BUT the identity and order of the
// cards: fixture-check.mjs already pins each card's rendered text (title,
// because) to the same fixture file byte for byte. This test's only job is
// the one fixture-check.mjs cannot do, because it has no notion of "the
// queue" at all — asking the real disposition-flow package what it would
// hand a human, and comparing that against the mockup's own `data-id`
// attributes.
//
// Both sides of L-0001: this test is red against the untouched mockup before
// E6-L3-T1's edit — s3-distill.html carried three `data-id`-less cards
// (dec-0011, dec-0012, qst-0006, in that order, with dec-0011 marked
// actionable), which both mismatches the ids Staged returns (Awaiting() over
// the pre-T1 fixture is empty — see the package comment on
// docs/decisions-pending/E6-L2-report.md §6.3's finding, restated in
// docs/plan/tasks/E6-L3.md's "What is already known" section) and predates
// the `data-id` attribute this test reads, so it fails to find any cards at
// all. That run is recorded in this lane's commit history rather than
// reproduced here, because reproducing it would mean carrying a second, stale
// copy of the fixture and mockup in testdata for a test whose whole point is
// that there is exactly one copy of each.
func TestDistillMockupMatchesTheQueue(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	store, err := local.Open(filepath.Join(root, "docs", "design", "fidelity", "fixtures", "ledger-design"))
	if err != nil {
		t.Fatalf("opening the fixture ledger: %v", err)
	}

	queue, err := distill.Staged(context.Background(), store)
	if err != nil {
		t.Fatalf("distill.Staged: %v", err)
	}
	if len(queue.Warnings) != 0 {
		t.Fatalf("Staged reported warnings over a fixture ledger that should read cleanly: %v", queue.Warnings)
	}

	wantAwaiting := ids(queue.Awaiting())
	wantPending := ids(queue.PendingExtraction())
	if len(wantAwaiting) == 0 && len(wantPending) == 0 {
		t.Fatal("the fixture ledger has no staged entries at all; this test would pass against an empty mockup")
	}

	haveActionable, haveNext := mockupCards(t, filepath.Join(root, "docs", "design", "screens", "s3-distill.html"))

	if !equal(haveActionable, wantAwaiting) {
		t.Errorf("s3-distill.html's actionable card(s) (.stage, not .stage.next) are %v, want Staged's Awaiting() %v",
			haveActionable, wantAwaiting)
	}
	if !equal(haveNext, wantPending) {
		t.Errorf("s3-distill.html's dimmed card(s) (.stage.next) are %v, want Staged's PendingExtraction() %v",
			haveNext, wantPending)
	}

	// Awaiting() is defined as at most one actionable card at a time — "only
	// the top card is actionable" is the deck's own rule (s3-distill.html's
	// comment), and Staged's Awaiting() over this fixture happens to return
	// exactly one. If that ever changes, this assertion is the one that
	// should be revisited, not silently dropped.
	if len(wantAwaiting) > 1 {
		t.Fatalf("Staged returned %d awaiting entries; the mockup renders only one card as .stage (actionable) — "+
			"decide how the deck should show more than one before this test hides it", len(wantAwaiting))
	}
}

func ids(items []distill.Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID())
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// mockupCards reads s3-distill.html and returns the `data-id` of every
// actionable card (<article class="stage" ...>, not "stage next") and every
// dimmed one (<article class="stage next" ...>), in source order — the same
// order the deck presents them in.
var (
	stageOpenTag = regexp.MustCompile(`<article\s+class="([^"]*)"[^>]*data-id="([^"]+)"`)
)

func mockupCards(t *testing.T, path string) (actionable, next []string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	for _, m := range stageOpenTag.FindAllStringSubmatch(string(raw), -1) {
		class, id := m[1], m[2]
		switch class {
		case "stage":
			actionable = append(actionable, id)
		case "stage next":
			next = append(next, id)
		default:
			t.Fatalf("%s: <article> with data-id=%q carries class %q, want \"stage\" or \"stage next\"", path, id, class)
		}
	}
	return actionable, next
}
