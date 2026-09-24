package ledger

import "gopkg.in/yaml.v3"

// The style memo is how dira writes a ledger file back the way it found it.
//
// The problem it solves is concrete. This repo's own 26 entries are hand-wrapped:
// 46 of their scalars are folded blocks (`why_not: >`), and no single greedy
// wrap width reproduces more than 19 of them — the best fit, 82 columns, gets 19.
// A codec that re-folded from the parsed value would therefore rewrite the
// wrapping of every paragraph in an entry the moment anything in that entry
// changed. `dira log --edge` adding one edge to dec-0002 would produce a diff
// touching forty lines, and dec-0002's own promise of reviewable history —
// "a PR touching a decision shows a legible diff" — would be false in the first
// release.
//
// So the codec records presentation on read and reuses it on write. What is
// recorded is only how a value was written: quoting style, block indicator, and
// the block's source lines. Values, keys, ordering, indentation, sequence layout
// and the markdown body all come from the parsed model and are re-emitted from
// it. The memo is keyed by the value it describes, so a value the caller changed
// no longer matches and falls through to canonical emission — which is what
// makes the round-trip test something other than a bytes cache echoing itself,
// and what TestEditingOneFieldRewritesOnlyThatField pins.

// blockStyle is a block scalar's source, kept verbatim.
type blockStyle struct {
	// indicator is the header as written: ">", "|", ">-", "|+" and so on.
	indicator string

	// lines are the block's content lines, dedented by the block's own
	// content indent so the text re-emits correctly at any nesting depth.
	lines []string
}

// scalarStyle is how one scalar was written on disk.
type scalarStyle struct {
	style yaml.Style
	block *blockStyle
}

// styleMemo maps a scalar's value to how that value was written. A nil memo is
// usable: it records nothing and looks nothing up, which is exactly what an
// entry dira composed itself should do.
type styleMemo map[string]scalarStyle

func (m styleMemo) record(value string, style scalarStyle) {
	if m == nil {
		return
	}
	// First writer wins. Two scalars with identical text and different
	// wrapping would otherwise flip-flop; keeping the first is stable, and
	// either choice re-parses to the same value.
	if _, ok := m[value]; !ok {
		m[value] = style
	}
}

func (m styleMemo) lookup(value string) (scalarStyle, bool) {
	if m == nil {
		return scalarStyle{}, false
	}
	style, ok := m[value]
	return style, ok
}
