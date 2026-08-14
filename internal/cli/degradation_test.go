package cli_test

import (
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/cli"
	"github.com/kazi-org/dira/internal/kazi"
)

// bannedWords are .agents/product-marketing.md §10's list — precise,
// unhyped, no hype words.
var bannedWords = []string{"revolutionary", "seamless", "supercharge", "10x", "AI-powered"}

// TestMapDegradationCopy is T6's acc line.
func TestMapDegradationCopy(t *testing.T) {
	reasons := []kazi.UnavailableReason{
		kazi.ReasonNotOnPath, kazi.ReasonNonZeroExit, kazi.ReasonMalformedJSON, kazi.ReasonWrongKind, kazi.ReasonTimeout,
	}

	lines := make(map[kazi.UnavailableReason]string, len(reasons))
	for _, reason := range reasons {
		t.Run(string(reason), func(t *testing.T) {
			line := cli.DegradationLine(reason)
			if line == "" {
				t.Fatal("empty line")
			}
			if line == string(reason) {
				t.Errorf("line = %q, want human-readable prose, not the bare Go constant", line)
			}
			for _, banned := range bannedWords {
				if strings.Contains(strings.ToLower(line), strings.ToLower(banned)) {
					t.Errorf("line %q contains a banned word %q", line, banned)
				}
			}
			lines[reason] = line
		})
	}

	t.Run("every reason produces visibly different wording", func(t *testing.T) {
		if len(lines) != len(reasons) {
			t.Fatalf("collected %d line(s), want %d", len(lines), len(reasons))
		}
		for i, a := range reasons {
			for j, b := range reasons {
				if i >= j {
					continue
				}
				if lines[a] == lines[b] {
					t.Errorf("%s and %s produced the identical line %q", a, b, lines[a])
				}
			}
		}
	})

	t.Run("both sides: one generic template collapses all five to one string", func(t *testing.T) {
		generic := func(kazi.UnavailableReason) string { return "kazi is unavailable" }
		seen := map[string]bool{}
		for _, reason := range reasons {
			seen[generic(reason)] = true
		}
		if len(seen) != 1 {
			t.Fatal("the generic-template control's own premise broke: it should collapse to exactly one string")
		}
		real := map[string]bool{}
		for _, reason := range reasons {
			real[cli.DegradationLine(reason)] = true
		}
		if len(real) != len(reasons) {
			t.Errorf("the real, reason-aware renderer produced %d distinct line(s), want %d", len(real), len(reasons))
		}
	})
}
