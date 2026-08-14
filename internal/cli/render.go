package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/kazi-org/dira/internal/status"
)

// RenderText writes tree in docs/design.md §6.4's shape: one line per
// derives_from parent with its roll-up, its children indented beneath it,
// then the unparented group. When snapErr is non-nil it is preceded by the
// one explicit, named-reason unavailability line E4-L5 checks for — and by
// nothing else: a succeeding kazi run must never print this line at all.
func RenderText(w io.Writer, tree *Tree, snapErr error) error {
	if snapErr != nil {
		if _, err := fmt.Fprintln(w, degradationLineFor(snapErr)); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	for _, g := range tree.Groups {
		if err := writeRow(w, "", g.Parent.ID, g.Parent.Title, rollupSuffix(g.Rollup)); err != nil {
			return err
		}
		for _, c := range g.Children {
			if err := writeRow(w, "  ", c.ID, c.Title, renderRowSuffix(c)); err != nil {
				return err
			}
		}
	}

	if len(tree.Unparented) > 0 {
		if _, err := fmt.Fprintln(w, "unparented"); err != nil {
			return err
		}
		for _, n := range tree.Unparented {
			if err := writeRow(w, "  ", n.ID, n.Title, renderRowSuffix(n)); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeRow prints one line: indent, id, title and (if non-empty) a suffix.
func writeRow(w io.Writer, indent, id, title, suffix string) error {
	if suffix == "" {
		_, err := fmt.Fprintf(w, "%s%s  %s\n", indent, id, title)
		return err
	}
	_, err := fmt.Fprintf(w, "%s%s  %s  %s\n", indent, id, title, suffix)
	return err
}

// renderRowSuffix is one node's own annotation — the arrow-and-label shape
// docs/design.md §6.4 shows, or the "⛔ blocks <id>" shape for an entry
// (typically an open question) that gates another rather than carrying a
// bucket of its own.
func renderRowSuffix(n *Node) string {
	switch {
	case n.Bucket == status.DecisionBlocked:
		return "→ " + blockedRowLabel(n.BlockedBy)
	case n.Bucket == status.Completed:
		return "→ " + Label(n.Bucket) + " ✓"
	case n.Bucket == status.ToBePlanned:
		return "→ " + Label(n.Bucket) + " (no goal yet)"
	case n.Bucket != "":
		return "→ " + Label(n.Bucket)
	case n.Ambiguous != nil:
		return "→ ambiguous (" + strings.Join(n.Ambiguous.Statuses, ", ") + ")"
	case n.Unresolved != nil:
		return "→ unresolved (" + n.Unresolved.Reason + ")"
	case n.BlocksTarget != "":
		return "⛔ blocks " + n.BlocksTarget
	default:
		return ""
	}
}

// rollupSuffix renders a parent's roll-up counts in dec-0004's bucket
// order, skipping any bucket with a zero count — "3 done · 1 running · 1
// stuck"-shaped, using T4's committed labels rather than kazi's own
// abbreviations.
func rollupSuffix(counts map[status.Bucket]int) string {
	var parts []string
	for _, b := range status.Buckets {
		if n := counts[b]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, Label(b)))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}
