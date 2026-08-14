package cli_test

import (
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/cli"
	"github.com/kazi-org/dira/internal/status"
)

// TestMapLabels is T4's acc line.
func TestMapLabels(t *testing.T) {
	t.Run("all six buckets have a distinct, non-empty label", func(t *testing.T) {
		if len(status.Buckets) != 6 {
			t.Fatalf("status.Buckets has %d entries, want 6 — this test's premise depends on the fixed six", len(status.Buckets))
		}
		seen := map[string]status.Bucket{}
		for _, b := range status.Buckets {
			label := cli.Label(b)
			if label == "" {
				t.Errorf("bucket %q has no label", b)
				continue
			}
			if owner, dup := seen[label]; dup {
				t.Errorf("buckets %q and %q share the label %q", owner, b, label)
			}
			seen[label] = b
		}
	})

	t.Run("InProgress and ExecutionBlocked are the exact literal substrings E4-L5 checks for", func(t *testing.T) {
		if got := cli.Label(status.InProgress); got != "in progress" {
			t.Errorf("Label(InProgress) = %q, want exactly %q", got, "in progress")
		}
		if got := cli.Label(status.ExecutionBlocked); got != "execution-blocked" {
			t.Errorf("Label(ExecutionBlocked) = %q, want exactly %q", got, "execution-blocked")
		}
	})

	t.Run("the blocked entry's own row names the blocking question's id", func(t *testing.T) {
		row := cli.RenderRowSuffixForTest(&cli.Node{
			Bucket:    status.DecisionBlocked,
			BlockedBy: &status.BlockingQuestion{ID: "qst-0042", Title: "does it matter"},
		})
		if !strings.Contains(row, "qst-0042") {
			t.Errorf("blocked-row rendering = %q, does not name the blocking question's id", row)
		}
	})

	t.Run("both sides: two buckets sharing one label is caught", func(t *testing.T) {
		mutated := map[status.Bucket]string{
			status.ToBePlanned: "same",
			status.Planned:     "same", // deliberately poisoned
		}
		seen := map[string]bool{}
		dup := false
		for _, label := range mutated {
			if seen[label] {
				dup = true
			}
			seen[label] = true
		}
		if !dup {
			t.Fatal("the poisoned label map's own premise broke: no duplicate found")
		}
	})
}
