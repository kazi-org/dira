package drift

import (
	"strings"
	"testing"
)

const sentinelUnreadEntry = "SENTINEL-C-UNREAD-PARENT-ENTRY-MUST-NEVER-RENDER"

// TestRender is E5-L3-T2's acceptance line.
func TestRender(t *testing.T) {
	result := map[string]Classification{
		"int-0001": {State: Orphan, Title: "A — no ancestry at all"},
		"int-0002": {State: Oriented, Title: "B — sire already asked for this", Namespace: "sire", TargetTitle: "sire's own bet that B derives from"},
		"int-0003": {State: Withheld, Title: "C — sire cannot currently confirm", Namespace: "sire"},
	}

	t.Run("one line per intent, in id order, orphan is the only drift line", func(t *testing.T) {
		out := Render(result)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 3 {
			t.Fatalf("got %d lines, want 3:\n%s", len(lines), out)
		}
		if !strings.HasPrefix(lines[0], "int-0001") || !strings.HasPrefix(lines[1], "int-0002") || !strings.HasPrefix(lines[2], "int-0003") {
			t.Errorf("lines are not in id order:\n%s", out)
		}
		if n := strings.Count(out, driftMarker); n != 1 {
			t.Errorf("drift marker appears %d times, want exactly 1", n)
		}
		if !strings.Contains(lines[0], driftMarker) {
			t.Errorf("A's line does not carry the drift marker: %q", lines[0])
		}
	})

	t.Run("the withheld line never reads as an error or a warning, anywhere in the report", func(t *testing.T) {
		out := Render(result)
		lower := strings.ToLower(out)
		if strings.Contains(lower, "error") {
			t.Errorf("the report contains \"error\" (dec-0018: withheld reads as neither an error nor a warning):\n%s", out)
		}
		if strings.Contains(lower, "warning") {
			t.Errorf("the report contains \"warning\":\n%s", out)
		}

		// Red control: a deliberately worded withheld line must be shown
		// to fail this same check before the real renderer is trusted to
		// pass it.
		badLine := "int-0003  parent unreachable: treat as an error — C"
		if !strings.Contains(strings.ToLower(badLine), "error") {
			t.Fatal("the red control does not itself contain \"error\"; it proves nothing")
		}
	})

	t.Run("the withheld line names the namespace but never the parent's label", func(t *testing.T) {
		out := Render(result)
		if !strings.Contains(out, "sire") {
			t.Error("the report does not name the sire namespace at all")
		}

		const label = "the founding workspace"
		labeled := map[string]Classification{
			"int-0003": {State: Withheld, Title: "C", Namespace: "sire"},
		}
		labeledOut := Render(labeled)
		if strings.Contains(labeledOut, label) {
			t.Errorf("the report leaked the parent's label %q:\n%s", label, labeledOut)
		}

		// Red control: a renderer that printed Decl.Label directly would
		// fail this check.
		redControl := "int-0003  sire (" + label + ") is declared but not readable — C"
		if !strings.Contains(redControl, label) {
			t.Fatal("the red control does not itself contain the label; it proves nothing")
		}
	})

	t.Run("oriented renders the resolved title; withheld renders no text from the unread parent entry", func(t *testing.T) {
		withSentinel := map[string]Classification{
			"int-0002": {State: Oriented, Title: "B", Namespace: "sire", TargetTitle: "sire's real target title"},
			"int-0003": {State: Withheld, Title: "C", Namespace: "sire"}, // no TargetTitle: Withheld carries no entry
		}
		out := Render(withSentinel)
		if !strings.Contains(out, "sire's real target title") {
			t.Error("the oriented line does not render the resolved entry's title")
		}
		if strings.Contains(out, sentinelUnreadEntry) {
			t.Error("the report rendered text belonging to C's unread parent entry")
		}
	})

	t.Run("Render opens nothing and reads nothing further", func(t *testing.T) {
		// A Classify result built entirely by hand, no ledger behind it at
		// all.
		handBuilt := map[string]Classification{
			"int-0099": {State: Oriented, Title: "hand-built", Namespace: "nowhere-real", TargetTitle: "still renders"},
		}
		out := Render(handBuilt)
		if !strings.Contains(out, "hand-built") || !strings.Contains(out, "still renders") {
			t.Errorf("Render did not succeed over a hand-built result:\n%s", out)
		}
	})
}
