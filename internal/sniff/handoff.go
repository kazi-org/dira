package sniff

import (
	"fmt"
	"strings"

	"github.com/kazi-org/dira/internal/brief"
	"github.com/kazi-org/dira/internal/ledger"
)

// The handoff block: what tier 1 hands to tier 2, and the only artifact the two
// tiers share.
//
// # Why it is a golden file rather than prose
//
// dec-0003 puts the semantic tier in a live session rather than in the binary,
// so the interface between them is a block of text one side prints and the other
// side reads. That is a contract, and a contract nobody diffs is a contract that
// drifts: a reworded instruction is invisible in review, and the skill that
// depended on the old wording fails silently — the run exits 0, the entry is
// never written, and nothing about it looks wrong. testdata/handoff.golden is
// the diff, and handoff_test.go is what makes a reword announce itself.
//
// # What dec-0023 changed about the shape
//
// This block was designed on the assumption that `PreCompact` stdout reaches the
// live session. It does not. dec-0023 establishes where it actually goes: stdout
// from a `PreCompact` hook becomes *custom instructions for the compaction
// summariser*, a one-turn call whose own prompt forbids it from calling tools.
// So the block has exactly one reader it is guaranteed to reach, and that reader
// cannot run `dira log`.
//
// The block is therefore written for two readers at once, and says so in its own
// text. A reader that can call tools gets the invocation. A reader that cannot —
// the summariser — gets an instruction it *can* obey: carry this block into the
// summary, so the ids survive the compaction and the next turn, delivered
// through a seam that does reach the model, can act on them. Which seam that is
// belongs to the task that wires `--deep`; this file's job is that the block
// works at either one, and asserts nothing about delivery that dec-0023 falsified.
//
// # Bounded, because the receiver is a token budget
//
// This text lands in a context window that is being compacted precisely because
// it is full. A handoff that grew with the candidate count would be largest
// exactly when there was least room for it. So it degrades: past a fixed number
// of staged entries it drops the titles and names ids only, and the whole block
// is measured with brief.Tokens — dira's own conservative counter, the one
// cst-0001 is enforced with (dec-0020) — rather than a second estimate that
// could disagree with it.
//
// # Redacted, by the same rule the excerpt path uses
//
// Everything rendered here goes through carriesCredential, and an item that
// trips it is dropped whole rather than masked. That is redact.go's rule, called
// rather than reimplemented: a masked line is evidence a reviewer cannot audit,
// and two redaction implementations are two things to keep in step. It is a
// second layer — collect() already refused these sentences before they were ever
// staged — and it is here because this block leaves the machine's ledger for a
// model's context, which is a boundary worth checking twice.
//
// The check reads the verbatim excerpt as well as the title, and HandoffItem
// says why: a title has already had `_` stripped out of it, which is the exact
// character the published-prefix patterns depend on. Redacting a normalised
// string is redacting the wrong string.

// handoffMaxTokens is the ceiling, in brief.Tokens, on the whole block.
//
// 400 against cst-0001's 1,500-token brief: the handoff is a message about work
// in flight, not a substitute for the ledger, and the session it arrives in has
// already run out of room. The number is enforced by Handoff itself and not only
// by the test, so a future block that grows past it degrades rather than ships.
const handoffMaxTokens = 400

// handoffDetail is how many staged entries the block will name with their
// titles. Past this it names ids only.
//
// Five is a reading bound rather than a token bound. The block exists to be
// acted on in one pass; a list longer than this is skimmed, and a skimmed list
// is worth no more than its ids.
const handoffDetail = 5

// handoffTitle bounds a title inside the block. Titles are stored at up to
// maxTitle (120); here they are shorter, because the block pays for them at the
// worst moment and the id is what the reader needs to act.
const handoffTitle = 48

// A HandoffItem is one staged entry as the handoff names it.
//
// The block renders the id and the title. The excerpt is carried but never
// drawn: the reader of this block is a session that still has the transcript, so
// quoting the sentence back at it costs tokens it does not have, and what tier 2
// is missing is the reasoning, which is not in the excerpt either.
//
// The excerpt is here for the credential check, and that is not belt-and-braces
// — it is the only field on which the check is reliable. Title is normalised
// before it is stored: text.go's markup rule strips `*`, `_` and backticks,
// which turns a `ghp_`-prefixed token into a `ghp`-prefixed one and an
// underscored key-variable assignment into an unbroken run of capitals. Both are
// still recoverable secrets, and both have lost exactly the character the
// published-prefix patterns in redact.go key on. A credential check that saw
// only the title would therefore pass on a title that leaks. It sees the
// verbatim sentence too, and that is the one that fires.
type HandoffItem struct {
	ID      string
	Title   string
	Excerpt string
}

// StagedItems renders a staging result as the items the handoff names.
//
// It takes the written entries rather than the candidates because the ids are
// the point — they are allocated by the ledger during the write, and a handoff
// that named candidates would name things the reader cannot address.
func StagedItems(staged []*ledger.Entry) []HandoffItem {
	out := make([]HandoffItem, 0, len(staged))
	for _, e := range staged {
		if e == nil || e.ID == "" {
			continue
		}
		item := HandoffItem{ID: e.ID, Title: e.Title}
		if e.Source != nil {
			item.Excerpt = e.Source.Excerpt
		}
		out = append(out, item)
	}
	return out
}

// Handoff renders the block for a set of staged entries.
//
// It returns the empty string when there is nothing to hand off, so a caller can
// print the result unconditionally and a session with no captured decisions gets
// no block rather than an empty header.
func Handoff(items []HandoffItem) string {
	items = withoutCredentials(items)
	if len(items) == 0 {
		return ""
	}

	// The ladder. Full detail first; then ids only; then fewer ids. Each
	// rung is measured rather than assumed, because a title is prose and
	// prose does not have a fixed cost.
	block := renderHandoff(items, handoffDetail, maxCandidates)
	for ids := maxCandidates; brief.Tokens(block) > handoffMaxTokens && ids > 1; ids-- {
		block = renderHandoff(items, 0, ids-1)
	}
	return block
}

// withoutCredentials drops any item whose rendered text carries something shaped
// like a credential. Dropped whole, never masked — redact.go says why.
func withoutCredentials(items []HandoffItem) []HandoffItem {
	out := make([]HandoffItem, 0, len(items))
	for _, it := range items {
		if carriesCredential(it.Title) || carriesCredential(it.Excerpt) || carriesCredential(it.ID) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// renderHandoff draws the block at a given verbosity.
//
// detail is how many items may be named with their titles; past it the listing
// degrades to ids. ids caps how many ids the degraded listing names before it
// reports a remainder. Both are parameters rather than constants so that a test
// can render the block with the degradation removed and observe the bound fail —
// a ceiling nobody has watched break is a ceiling nobody has tested.
//
// It performs no redaction. Handoff does that, before calling this.
func renderHandoff(items []HandoffItem, detail, ids int) string {
	var b strings.Builder

	b.WriteString("=== dira handoff, tier 2 ===\n\n")
	fmt.Fprintf(&b, "dira sniff staged %s. Tier 1 records no because, no alternative\n", plural(len(items), "decision", "decisions"))
	b.WriteString("and no edge (dec-0003):\n\n")

	b.WriteString(listing(items, detail, ids))

	b.WriteString("\nIf you can call tools, supply alternatives, why_not and derives_from\n")
	b.WriteString("from what was actually said. Never invent one. One call per id:\n\n")
	b.WriteString("  dira log --kind decision --state staged --tier semantic --hook PreCompact \\\n")
	b.WriteString("    --title '<one line>' --body '<the because, in the words used>' \\\n")
	b.WriteString("    --alternative '<the road not taken>' --why-not '<why it lost>' \\\n")
	b.WriteString("    --edge derives_from=" + items[0].ID + " --excerpt '<the sentence>'\n\n")
	b.WriteString("If you cannot call tools, you are summarising for compaction: carry\n")
	b.WriteString("this block into the summary unchanged. Nothing else carries the ids\n")
	b.WriteString("across it (dec-0023).\n\n")
	b.WriteString("=== end dira handoff ===\n")

	return b.String()
}

// listing draws the staged entries, with titles or without.
func listing(items []HandoffItem, detail, ids int) string {
	var b strings.Builder

	if len(items) <= detail {
		for _, it := range items {
			fmt.Fprintf(&b, "  %s  %s\n", it.ID, bound(it.Title, handoffTitle))
		}
		return b.String()
	}

	if ids < 1 {
		ids = 1
	}
	shown := items
	if len(shown) > ids {
		shown = shown[:ids]
	}
	names := make([]string, len(shown))
	for i, it := range shown {
		names[i] = it.ID
	}
	for i := 0; i < len(names); i += 6 {
		end := i + 6
		if end > len(names) {
			end = len(names)
		}
		fmt.Fprintf(&b, "  %s\n", strings.Join(names[i:end], ", "))
	}
	if rest := len(items) - len(shown); rest > 0 {
		fmt.Fprintf(&b, "  … and %d more, ids only: too many staged to carry titles.\n", rest)
	}
	return b.String()
}

// plural picks the noun. One staged decision is the common case and "1
// decisions" reads like a bug in the tool that wrote it.
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
