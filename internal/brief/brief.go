// Package brief renders the session brief: what is blocked, what is being
// worked towards, and what was decided recently, in one screen and under a hard
// token ceiling.
//
// # It is the whole of inversion 2
//
// int-0001's target is not better capture, it is retrieval that costs zero
// effort: "if reading dira is ever a task on someone's list, dira has failed."
// The brief is the push half of that — injected by the SessionStart hook into a
// session's context, and read by a human at a glance, before anyone asks
// anything. `dira why` is the pull half.
//
// # The ceiling is constitutional, and it is enforced here
//
// cst-0001 caps the brief at 1,500 tokens *in the binary*, "not by editorial
// judgment", and requires that going over drops whole entries by priority rather
// than truncating mid-render, naming what it omitted and the verb to see the
// rest. That is this package's central mechanism rather than a check bolted onto
// the end of it: every block of output is measured before it is written, and a
// block that would not fit stops the fill instead of being cut in half.
//
// The drop order is cst-0001's keep order read from the bottom: open blockers
// are the last thing to go, then current focus, then recent decisions, then
// fresh notes. docs/plan/lanes/E1.md pins what those mean in E1, where there is
// no kazi join and no tier resolution — open blockers are open questions
// carrying a `blocks` edge, current focus is active intents, recent decisions
// are accepted decisions newest first.
//
// # What it will not say
//
// Nothing about execution. dec-0004 derives status from kazi at read time and
// dira embeds no kazi client, so there is no bucket, no "in progress", and no
// mark against a `realized_by` target claiming it converged. E4 owns the join;
// until it lands the slot is absent rather than guessed, because a brief that
// guessed would be wrong in exactly the way hand-entered status is wrong.
//
// A `private: true` entry appears by ref and state, never by title. The reasoning
// is internal/enforcer's, unchanged: the binary cannot tell whether its stdout is
// a terminal or a pull-request body, so it never has the information that would
// make quoting the text safe (cst-0003).
package brief

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kazi-org/dira/internal/chain"
	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/render"
)

// DefaultMaxTokens is the ceiling when .dira/config.toml does not set one.
//
// It is cst-0001's number, and it is the default rather than "no limit" on
// purpose: a ledger with no config file, or with a config file dira could not
// read, still gets the constitutional cap. A cap that switches itself off when
// something is missing is not a cap.
const DefaultMaxTokens = 1500

// NoteWindow is how long a note stays in the brief.
//
// cst-0002 makes `note` the pressure valve and says notes "surface once in the
// next brief and then decay, so the valve cannot silently become a sixth store".
// Surfacing exactly once needs a memory of what was already surfaced, and dira
// has nowhere honest to keep one: the brief is a read verb, and a read verb that
// wrote a "seen" marker into the ledger would be storing derived state in the
// files (dec-0002, dec-0004) and would make two sessions started in parallel
// disagree about what they had seen.
//
// So decay is by age instead: a note surfaces in the briefs of the week it was
// written and then stops. That is weaker than "once" and stronger than "forever",
// it needs no write, and it is stated here rather than implied so that whoever
// owns the session-level marker later (E2 installs the hooks; nothing in E1 owns
// per-session state) can see exactly what it would be replacing.
const NoteWindow = 7 * 24 * time.Hour

// A Source is the read surface a brief is composed from.
//
// It is declared in the consumer and sized to the two questions a brief asks:
// which entries belong in it, and what does each of them say. The split is the
// index's files-win property inherited rather than re-argued — Select answers
// from the cache and Entry reads the file, so nothing a brief prints has come
// out of the cache (dec-0002). *index.Index satisfies it.
type Source interface {
	Select(ctx context.Context, sel index.Selector) ([]index.Ref, error)
	Entry(ctx context.Context, id string) (*ledger.Entry, error)
}

// Options is everything the renderer needs that it must not go and find for
// itself.
type Options struct {
	// MaxTokens is the ceiling, from brief.max_tokens. Zero means
	// DefaultMaxTokens.
	MaxTokens int

	// Width is the column the brief wraps at. Zero means the same default
	// `dira why` uses, because they are read on the same screen.
	Width int

	// Now is the clock the note window is measured against. The zero time
	// means time.Now, and a test that pins it gets a brief that does not
	// expire overnight.
	Now time.Time

	// Ledger is the ledger's name from .dira/config.toml, for the heading.
	Ledger string

	// Context selects the agent-facing form injected by the SessionStart
	// hook. The human-facing form is the same selection with a different
	// heading and no instructions to a model.
	Context bool

	// Chain asks for the tier chain: each parent ledger's own active bets
	// (workspace tier) or directions (person tier), rendered inside the
	// same token ceiling as the local brief (cst-0001 — the ceiling applies
	// to the chain, not just the local ledger). Resolution is
	// internal/chain (E5); this package turns what it finds into content.
	Chain bool

	// Parents are the namespaces declared under [parents] in
	// .dira/config.toml, so --chain can tell "none configured" from
	// "configured, and its content follows below".
	Parents []string

	// ChainSource resolves the ledger's full parent chain (chain.Walk) when
	// Chain is set. It is a func rather than an already-resolved slice so a
	// caller that did not ask for --chain never pays for a walk it will not
	// use — the same laziness Source.Select buys for the local ledger. Nil
	// is treated as no parents configured.
	ChainSource func(ctx context.Context) ([]chain.Ancestor, error)

	// Notices are things the caller already knows and the reader should:
	// an unusable cache, entry files that could not be parsed. They are in
	// the brief rather than only on stderr because the hook sends stderr to
	// /dev/null, and an entry silently missing from the brief is the failure
	// mode this is here to prevent.
	Notices []string
}

// A Result is what the renderer did, for a caller that has to report it and for
// tests that have to assert it.
type Result struct {
	// Tokens is the count of what was written, by Tokens.
	Tokens    int
	MaxTokens int

	Sections []SectionResult
}

// A SectionResult is one section's arithmetic: how many entries the ledger had
// for it and how many survived the ceiling.
type SectionResult struct {
	Name  string
	Kept  int
	Total int
}

// Omitted is how many entries the ceiling dropped.
func (r *Result) Omitted() int {
	n := 0
	for _, s := range r.Sections {
		n += s.Total - s.Kept
	}
	return n
}

// Render writes the brief and reports what it contained.
//
// It never fails on account of the ledger's contents. An entry that cannot be
// read is the index's business and arrives here as a notice; a section with
// nothing in it says so; a ceiling too small for anything at all yields a short
// brief rather than an error. The SessionStart hook runs this with `2>/dev/null
// || true`, so anything this function turns into an error is orientation the
// session silently does not get.
func Render(ctx context.Context, w io.Writer, src Source, opts Options) (*Result, error) {
	opts = opts.normalised()

	loaded, err := load(ctx, src, opts)
	if err != nil {
		return nil, err
	}

	doc, result := compose(ctx, src, opts, loaded)
	if _, err := io.WriteString(w, doc); err != nil {
		return nil, fmt.Errorf("writing the brief: %w", err)
	}
	return result, nil
}

func (o Options) normalised() Options {
	if o.MaxTokens <= 0 {
		o.MaxTokens = DefaultMaxTokens
	}
	if o.Width <= 0 {
		o.Width = defaultWidth
	}
	if o.Width < minWidth {
		o.Width = minWidth
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	if o.Ledger == "" {
		o.Ledger = "this ledger"
	}
	return o
}

// A section is one band of the brief: what it is called, what it selects, and
// what to say when the ledger has none of it.
//
// The order of the slice is the keep order. Everything about dropping follows
// from it, so changing priority is changing this list and nothing else.
type section struct {
	name  string
	empty string

	selector index.Selector

	// fresh filters the selection on something the cache can answer but a
	// selector cannot. Only notes use it, and only to decay.
	fresh func(index.Ref, Options) bool
}

// sections is cst-0001's order, most important first.
//
// "Open blockers, then current focus, then recent decisions" is the order of
// *importance* in the constraint, which makes it the order of keeping here: the
// fill runs down this list and stops, so what is dropped is always a tail.
func sections() []section {
	return []section{
		{
			name:  "open blockers",
			empty: "none — nothing in this ledger is waiting on an unanswered question",
			selector: index.Selector{
				Kinds:    []ledger.Kind{ledger.KindQuestion},
				States:   []ledger.State{ledger.StateOpen},
				WithEdge: ledger.EdgeBlocks,
			},
		},
		{
			name:     "current focus",
			empty:    "none — no intent is active",
			selector: index.Selector{Kinds: []ledger.Kind{ledger.KindIntent}, States: []ledger.State{ledger.StateActive}},
		},
		{
			name:     "recent decisions",
			empty:    "none — nothing has been accepted yet",
			selector: index.Selector{Kinds: []ledger.Kind{ledger.KindDecision}, States: []ledger.State{ledger.StateAccepted}},
		},
		{
			name:     "fresh notes",
			empty:    "none — no note was written in the last week",
			selector: index.Selector{Kinds: []ledger.Kind{ledger.KindNote}, States: []ledger.State{ledger.StateActive}},
			fresh: func(ref index.Ref, opts Options) bool {
				return within(stamp(ref.Updated, ref.Created), opts.Now, NoteWindow)
			},
		},
	}
}

// loaded is one section's selection: refs only, which is the cache's job, and
// deliberately not entries.
//
// Nothing here has been read from a file yet. That is what keeps a cold
// `dira brief` cheap on a 200-entry ledger: the ceiling decides which twenty or
// so entries get rendered, and only those get opened.
type loadedSection struct {
	section section
	refs    []index.Ref
}

func load(ctx context.Context, src Source, opts Options) ([]loadedSection, error) {
	specs := sections()
	out := make([]loadedSection, 0, len(specs))
	for _, spec := range specs {
		refs, err := src.Select(ctx, spec.selector)
		if err != nil {
			return nil, err
		}
		if spec.fresh != nil {
			kept := refs[:0]
			for _, ref := range refs {
				if spec.fresh(ref, opts) {
					kept = append(kept, ref)
				}
			}
			refs = kept
		}
		out = append(out, loadedSection{section: spec, refs: refs})
	}
	return out, nil
}

// stamp is the date an entry is judged by: when it was last touched, or when it
// was written if it never was.
func stamp(updated, created string) string {
	if updated != "" {
		return updated
	}
	return created
}

// within reports whether an RFC3339 timestamp is inside a window ending at now.
//
// A timestamp dira cannot parse counts as inside it. The schema validates the
// format, so this is a file somebody hand-edited into a shape dira did not
// expect, and dropping an entry out of the brief for it would be a silent
// omission where the alternative is a visible one.
func within(ts string, now time.Time, window time.Duration) bool {
	at, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return true
	}
	return !at.Before(now.Add(-window))
}

// day is the date without the time, which is all a brief shows.
func day(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

// status is the right-margin column: what state an entry is in, and when.
func status(e *ledger.Entry) string {
	return strings.TrimSpace(string(e.State) + " " + day(stamp(e.Updated, e.Created)))
}

// rule is the shared painter configuration. The brief and the chain are read on
// the same screen and wrap at the same column; internal/render is the layout
// both use, so a wrapped title looks the same in both.
const (
	defaultWidth = 80
	minWidth     = 56
	textFloor    = 24
)

func painter(opts Options) *render.Painter { return render.New(opts.Width, textFloor) }
