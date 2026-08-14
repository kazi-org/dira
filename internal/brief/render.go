package brief

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kazi-org/dira/internal/chain"
	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/render"
)

// compose fills the ceiling and returns what fitted.
//
// The whole of cst-0001 is in the shape of this function, so it is worth being
// explicit about it:
//
//   - Every piece of output is painted whole, measured, and then either written
//     or not written. There is no path on which half a piece reaches the page,
//     which is what "drops entries rather than truncating mid-render" means when
//     it is a mechanism instead of an intention.
//   - The pieces are offered in keep order, and the fill *stops* at the first one
//     that does not fit rather than skipping it and trying the next. Skipping
//     would keep a later, smaller entry over an earlier, more important one, and
//     the omission would no longer be a tail — which is the property the drop
//     order is.
//   - The footer that names the omission has to fit *inside* the ceiling too, and
//     what it says depends on what was dropped — so it is written last and paid
//     for by giving entries back until it fits. A cap that could not afford to
//     say what it dropped would drop things silently, and cst-0001 requires the
//     opposite in the same sentence.
//
// The footer was reserved up front in the first version of this function, priced
// at its worst case. That is simpler and it is wrong at small ceilings: the worst
// case is around seventy tokens, so a `max_tokens = 60` produced an empty brief
// with a footer over the ceiling — the one number the whole mechanism exists to
// hold. Giving entries back instead costs a few more measurements and is exact.
func compose(ctx context.Context, src Source, opts Options, loaded []loadedSection) (string, *Result) {
	result := &Result{MaxTokens: opts.MaxTokens}
	for _, l := range loaded {
		result.Sections = append(result.Sections, SectionResult{Name: l.section.name, Total: len(l.refs)})
	}

	var written []piece
	spent := 0
	add := func(text string, section int) bool {
		cost := Tokens(text)
		if spent+cost > opts.MaxTokens {
			return false
		}
		spent += cost
		written = append(written, piece{text: text, cost: cost, section: section})
		return true
	}

	filling := true
	for _, block := range preamble(opts) {
		if !add(block, noSection) {
			filling = false
			break
		}
	}

sections:
	for i, l := range loaded {
		if !filling {
			break
		}
		heading := "\n" + l.section.name + "\n"

		if len(l.refs) == 0 {
			p := painter(opts)
			p.Row(itemIndent, itemIndent, l.section.empty, "")
			// noSection: a section with nothing in it holds no entry,
			// so giving it back would not reduce any omission count.
			if !add(heading+p.String(), noSection) {
				filling = false
			}
			continue
		}

		// The heading rides with the first entry that fits, so a section
		// whose every entry was dropped leaves no orphan heading behind —
		// and giving that entry back later takes the heading with it.
		pending := heading
		for _, ref := range l.refs {
			// The first entry that does not fit ends the fill — not just
			// this section's. Everything after it in the keep order is
			// less important than what was just refused, so continuing
			// would keep a lower-priority entry over a higher-priority
			// one and the omission would stop being a tail.
			if !add(pending+item(ctx, src, opts, ref), i) {
				break sections
			}
			pending = ""
			result.Sections[i].Kept++
		}
	}

	if filling && opts.Chain && opts.ChainSource != nil {
		fillChain(ctx, opts, result, add)
	}

	// Make room for the footer by giving entries back, cheapest-priority
	// first, until what is left plus the notice fits under the ceiling.
	tail := ""
	for result.Omitted() > 0 {
		tail = footer(opts, result)
		if spent+Tokens(tail) <= opts.MaxTokens {
			break
		}
		tail = ""
		if len(written) == 0 {
			// A ceiling too small to hold even the notice. What comes
			// out is nothing at all, which is what a ceiling of five
			// tokens asked for; it is not a truncated brief.
			break
		}
		last := written[len(written)-1]
		written = written[:len(written)-1]
		spent -= last.cost
		if last.section != noSection {
			result.Sections[last.section].Kept--
		}
	}

	var doc strings.Builder
	for _, p := range written {
		doc.WriteString(p.text)
	}
	doc.WriteString(tail)

	out := doc.String()
	result.Tokens = Tokens(out)
	return out, result
}

// A piece is one measured block of the document, remembered so it can be given
// back if the omission notice needs the room.
type piece struct {
	text string
	cost int

	// section indexes Result.Sections, or noSection for a block that is not
	// an entry — the heading, the guidance, a notice, an empty section.
	section int
}

const noSection = -1

// itemIndent is the two columns everything under a heading sits in.
const itemIndent = "  "

// preamble is the head of the brief, in keep order: each element is dropped
// before anything below it is, and the first is what the brief is.
func preamble(opts Options) []string {
	var blocks []string

	p := painter(opts)
	p.Row("", "", fmt.Sprintf("dira brief — %s · %s", opts.Ledger, opts.Now.UTC().Format("2006-01-02")), "")
	blocks = append(blocks, p.String())

	if opts.Context {
		// The agent-facing form says what the reader is holding and what
		// to do with it. It is two lines because it is spending the same
		// ceiling as the content, and because an agent that needs more
		// than two lines of instruction to treat a record as settled will
		// not be fixed by a third.
		p := painter(opts)
		p.Line("")
		p.Row("", "", "Settled records from this repository's ledger, injected at session start. "+
			"Treat them as decided: run `dira check \"<plan>\"` before planning something that "+
			"may contradict one, and `dira why <id>` for the reasoning behind any line.", "")
		blocks = append(blocks, p.String())
	}

	if opts.Chain {
		p := painter(opts)
		p.Line("")
		p.Row("", "", chainNotice(opts), "")
		blocks = append(blocks, p.String())
	}

	for _, notice := range opts.Notices {
		if strings.TrimSpace(notice) == "" {
			continue
		}
		p := painter(opts)
		p.Line("")
		p.Row("", "", notice, "")
		blocks = append(blocks, p.String())
	}

	return blocks
}

// chainNotice is `--chain`'s lead-in line: which case this run is in — no
// parent configured, or a chain whose content (if any survived the ceiling)
// follows below it — so the reader is oriented before the chain's own
// content appears.
//
// The two cases are told apart deliberately. "No parent ledger is configured"
// is a fact about this repository; "parents are configured" is followed by
// their content, and a reader who wrote a [parents] section is shown what
// dira found there rather than told it cannot look (E5-L2 resolved
// qst-0001: dec-0011).
func chainNotice(opts Options) string {
	if len(opts.Parents) == 0 {
		return "chain: no parent ledger is configured ([parents] in .dira/config.toml is empty), " +
			"so this brief is this repository's ledger alone."
	}
	return fmt.Sprintf("chain: %s configured as %s.",
		plural(len(opts.Parents), "one parent ledger is", fmt.Sprintf("%d parent ledgers are", len(opts.Parents))),
		strings.Join(opts.Parents, ", "))
}

// fillChain resolves the ledger's parent chain and spends what is left of the
// shared ceiling on each ancestor's own active intents — bets for a
// workspace-tier ancestor, directions for a person-tier one (docs/design.md
// §8's mix per tier; both are the same entry kind, so this needs no new one).
//
// It is the chain's whole contribution to compose's budget: every ancestor's
// content goes through the same add closure the local sections use, so
// cst-0001's ceiling is unconditional — the constraint's own text, "the
// ceiling applies to the chain, not just the local brief" — and whatever it
// cuts is named in the footer by that ancestor's namespace, because
// result.Sections carries no distinction between a local section and a
// chain one.
//
// An ancestor chain.Walk could not open (Store == nil — declared but
// unreadable) contributes nothing: there is no ref to a specific entry to
// cite here, only the namespace chainNotice already named, so there is
// nothing this function could render even by ref. That is the same
// discipline internal/drift's Render holds for a withheld classification,
// arrived at the same way — by never having text to print in the first
// place, rather than by redacting it after the fact.
func fillChain(ctx context.Context, opts Options, result *Result, add func(string, int) bool) {
	ancestors, err := opts.ChainSource(ctx)
	if err != nil {
		return
	}
	for _, a := range ancestors {
		if ctx.Err() != nil {
			return
		}
		entries := activeIntents(ctx, a)

		secIdx := len(result.Sections)
		result.Sections = append(result.Sections, SectionResult{Name: a.Namespace, Total: len(entries)})
		if len(entries) == 0 {
			continue
		}

		heading := "\n" + chainLabel(a) + "\n" + capWarning(a.Namespace, len(entries))
		pending := heading
		for _, e := range entries {
			if !add(pending+chainItem(opts, a.Namespace, e), secIdx) {
				return
			}
			pending = ""
			result.Sections[secIdx].Kept++
		}
	}
}

// activeIntents reads one ancestor's active intents directly through its
// Store — chain.Ancestor's Store is always ledger.ReadOnly, at every depth
// (E5-L2-T3) — sorted by id, the same total order internal/enforcer.Inherit
// already holds its own inherited units to.
//
// An ancestor with no Store (unreachable) has nothing to list; a List or Get
// failure mid-read degrades to "nothing from this ancestor" rather than
// failing the whole brief, matching cst-0001's own promise that a bad ledger
// degrades to a brief that says so rather than no brief at all.
func activeIntents(ctx context.Context, a chain.Ancestor) []*ledger.Entry {
	if a.Store == nil {
		return nil
	}
	infos, err := a.Store.List(ctx)
	if err != nil {
		return nil
	}
	var out []*ledger.Entry
	for _, info := range infos {
		prefix, _, ok := strings.Cut(info.ID, "-")
		if !ok {
			continue
		}
		kind, known := ledger.KindForPrefix(prefix)
		if !known || kind != ledger.KindIntent {
			continue
		}
		e, err := a.Store.Get(ctx, info.ID)
		if err != nil || e.Kind != ledger.KindIntent || e.State != ledger.StateActive {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// chainLabel is one ancestor's section heading: its namespace and, per
// docs/design.md §8's mix, what its tier calls an active intent.
func chainLabel(a chain.Ancestor) string {
	switch a.Tier {
	case "workspace":
		return a.Namespace + " — bets"
	case "person":
		return a.Namespace + " — directions"
	default:
		return a.Namespace + " — active intents"
	}
}

// chainItem paints one ancestor's entry, the chain's counterpart to item():
// a namespaced id (dec-0011 — `sire:int-0002`, the only form a ref to a
// parent's own entry may take), title (or the private/withheld placeholder —
// title() already covers that), state and date. It reads nothing further;
// every field it prints came from the *ledger.Entry activeIntents already
// fetched.
func chainItem(opts Options, namespace string, e *ledger.Entry) string {
	p := painter(opts)
	p.EntryRow(itemIndent, itemIndent, namespace+":"+e.ID, title(e), status(e))
	return p.String()
}

// item paints one entry: its id, its title, its state and date, and — for an
// open question — what it is holding up.
//
// This is the only place a brief reads a file, and it reads exactly the entries
// that are going to be rendered. On a 200-entry ledger the ceiling admits a few
// dozen, so the other ~170 files are never opened; that is where the cold-start
// budget int-0002 sets is actually spent or saved.
func item(ctx context.Context, src Source, opts Options, ref index.Ref) string {
	p := painter(opts)

	entry, err := src.Entry(ctx, ref.ID)
	if err != nil {
		// The index said this entry exists and the file did not answer.
		// Saying so by ref costs one line and is the honest version of a
		// gap; dropping the row silently is the version that lets a
		// session be oriented by a brief with a hole in it.
		p.EntryRow(itemIndent, itemIndent, ref.ID, "(this entry could not be read)", "")
		return p.String()
	}

	p.EntryRow(itemIndent, itemIndent, entry.ID, title(entry), status(entry))

	under := itemIndent + strings.Repeat(" ", render.RuneLen(entry.ID)+4)
	for _, edge := range entry.Edges {
		if edge.Type != ledger.EdgeBlocks {
			continue
		}
		p.Row(under, under, "blocks "+describe(ctx, src, edge.To), "")
	}
	return p.String()
}

// title is what may be shown of an entry, which for a private one is nothing.
//
// internal/enforcer made this call first and the reasoning carries over
// unchanged: the binary cannot tell whether its stdout is a terminal or a
// pull-request body, so it never has the information that would make quoting a
// private entry's text safe (cst-0003). The ref and the state still appear,
// because "there is something here you cannot see" is orientation and silence is
// not.
func title(e *ledger.Entry) string {
	if e.Private {
		return "(private — cited by reference only)"
	}
	return e.Title
}

// describe names an entry a blocks edge points at, with its title when there is
// one to be had.
//
// A target dira cannot read is named by ref alone rather than dropped: an open
// question that blocks something is the one thing in this brief that cannot be
// reconstructed from anywhere else (dec-0004 — no execution tracker can see it),
// so the edge is worth more than the title on the end of it.
func describe(ctx context.Context, src Source, ref string) string {
	if !ledger.ValidID(ref) {
		return ref
	}
	target, err := src.Entry(ctx, ref)
	if err != nil {
		return ref
	}
	if target.Private {
		return ref
	}
	return ref + " — " + target.Title
}

// footer names what the ceiling dropped and the verb that reaches it.
//
// cst-0001 asks for both in one sentence: "states what it omitted plus the verb
// to see the rest". The verb is `dira why <id>`, which is the only read verb E1
// ships — `dira map` is E4's, and pointing at it now would be pointing at
// something that does not exist. Raising brief.max_tokens is deliberately not
// offered: cst-0001 says raising the ceiling requires superseding the constraint
// in writing, and a footer that suggested it would be the binary undermining the
// rule it is enforcing two lines further up.
func footer(opts Options, r *Result) string {
	var dropped []string
	for _, s := range r.Sections {
		if n := s.Total - s.Kept; n > 0 {
			dropped = append(dropped, fmt.Sprintf("%d %s", n, s.Name))
		}
	}
	if len(dropped) == 0 {
		return ""
	}

	p := painter(opts)
	p.Line("")
	p.Row("omitted  ", "         ",
		fmt.Sprintf("%s — over the %s-token ceiling (cst-0001)", commas(dropped), thousands(opts.MaxTokens)), "")
	p.Row("         ", "         ",
		"the oldest of each go first; `dira why <id>` prints any entry in full", "")
	return p.String()
}

// commas joins a list the way a sentence does.
func commas(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	}
}

// thousands writes a token count the way the constraint does: 1,500.
func thousands(n int) string {
	s := strconv.Itoa(n)
	if n < 0 {
		return s
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
