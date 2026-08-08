package sniff

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
)

// The write path, and the reason "sniff may only ever stage" is a property of
// the code rather than an assertion about it.
//
// Three things hold it, in increasing order of how hard they are to defeat:
//
//  1. Candidate has no state, confirmed_by, alternatives or adr field, so no
//     caller of this package can express a confirmed entry.
//
//  2. Stage builds every Entry itself, from constants. There is no parameter
//     through which state or tier arrives.
//
//  3. Stage wraps the caller's Store in a stagedOnly before touching it, and
//     that wrapper is the only Store any code in this file holds. stagedOnly
//     refuses a Create whose entry is not staged and regex-tier, and refuses
//     Put and Delete outright — so even a future bug in (2) produces an error
//     instead of a confirmed entry, and even a caller inside this package
//     cannot reach the real store to route around it.
//
// (3) is the one that matters. (1) and (2) are what the code currently does;
// (3) is what stays true after somebody edits it.

// errRefused is what stagedOnly returns. It wraps nothing: there is no
// underlying failure, the write was refused.
var errRefused = errors.New("sniff: refused")

// A StageOptions carries the provenance a candidate cannot know about itself.
type StageOptions struct {
	// Hook is the capture point. It is validated against the staging rule:
	// the regex tier runs from Stop and PreCompact, and from a human typing
	// `dira sniff` by hand, which is `manual`.
	Hook ledger.Hook

	// Session is the opaque session id, for tracing an entry back to the
	// conversation that produced it. Empty is fine and common.
	Session string

	// Now is the clock. It is a field for the same reason app.now is one in
	// cmd/dira: `created` goes into the permanent record, and a test that
	// cannot say what time it is can only assert that the field looks
	// vaguely like a timestamp.
	Now time.Time
}

// A Result is what one staging run did.
type Result struct {
	// Staged are the entries written, in the order they were written, with
	// their allocated ids.
	Staged []*ledger.Entry

	// Duplicates counts candidates skipped because the ledger already holds
	// an entry with that title. The Stop hook fires on every turn against a
	// transcript that only grows, so without this the same sentence would be
	// re-proposed for the rest of the session.
	Duplicates int

	// Dropped counts candidates discarded by the per-run bound. It is
	// reported rather than swallowed: silently discarding a decision is the
	// failure this whole tool exists to prevent.
	Dropped int
}

// Stage writes candidates to the ledger as staged, regex-tier entries.
//
// It returns the entries it wrote. Nothing it writes can be cited by
// `dira check` (a staged decision is never enforcement substrate) and nothing it
// writes carries an alternative or a why_not, because a regex has no basis for
// either — the schema's minItems on `alternatives` is relaxed for exactly this
// state, and only this state.
func Stage(ctx context.Context, store ledger.Store, opts StageOptions, candidates []Candidate) (*Result, error) {
	if store == nil {
		return nil, errors.New("sniff: no ledger")
	}
	if !stagingHook(opts.Hook) {
		return nil, fmt.Errorf("%w: %q is not a capture point the regex tier writes from (%s)",
			errRefused, opts.Hook, "Stop, PreCompact, manual")
	}
	if opts.Now.IsZero() {
		return nil, errors.New("sniff: no clock; created is stamped by the caller")
	}

	// The real store is not held anywhere below this line.
	guarded := stagedOnly{inner: store}

	known, err := existingTitles(ctx, guarded)
	if err != nil {
		return nil, err
	}

	result := &Result{}
	for _, c := range candidates {
		if known[titleKey(c.Title)] {
			result.Duplicates++
			continue
		}
		if len(result.Staged) >= maxCandidates {
			result.Dropped++
			continue
		}

		entry := stagedEntry(c, opts)
		if err := ledger.Add(ctx, guarded, entry); err != nil {
			return result, fmt.Errorf("staging %q: %w", c.Title, err)
		}
		known[titleKey(c.Title)] = true
		result.Staged = append(result.Staged, entry)
	}
	return result, nil
}

// stagedEntry is the only place in dira where a regex-tier entry is built, and
// every field it cannot honestly fill is left empty.
//
// state and tier are constants here rather than parameters. Alternatives are
// absent rather than invented: `alternatives: []` would satisfy the schema's
// letter while asserting that the roads not taken were considered and found to
// be none. confirmed_by is absent because nobody has confirmed it. The body is
// empty because the "because" is precisely what tier 1 cannot know — dec-0003's
// second rejected alternative says so in the ledger.
func stagedEntry(c Candidate, opts StageOptions) *ledger.Entry {
	return &ledger.Entry{
		Kind:    ledger.KindDecision,
		Title:   c.Title,
		State:   ledger.StateStaged,
		Created: opts.Now.UTC().Format(time.RFC3339),
		Source: &ledger.Source{
			Hook:    opts.Hook,
			Session: opts.Session,
			Excerpt: c.Excerpt,
			Tier:    ledger.TierRegex,
		},
	}
}

// stagingHook is the closed set of capture points this tier writes from.
func stagingHook(h ledger.Hook) bool {
	switch h {
	case ledger.HookStop, ledger.HookPreCompact, ledger.HookManual:
		return true
	default:
		return false
	}
}

// existingTitles reads the titles already in the ledger, so a sentence is staged
// once rather than once per turn.
//
// Only decision entries are read. That is the kind this tier writes, and it
// halves the cost of a check that runs on every Stop — the read is the one part
// of `dira sniff` that scales with ledger size, and int-0002 budgets the whole
// invocation at well under 100ms.
func existingTitles(ctx context.Context, store ledger.Store) (map[string]bool, error) {
	infos, err := store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading the ledger: %w", err)
	}
	out := make(map[string]bool, len(infos))
	for _, info := range infos {
		if !strings.HasPrefix(info.ID, ledger.KindDecision.Prefix()+"-") {
			continue
		}
		entry, err := store.Get(ctx, info.ID)
		if err != nil {
			// A file that vanished between the list and the read is
			// another writer's business, not a reason to capture
			// nothing. The cost of being wrong here is one duplicate
			// a human deletes.
			continue
		}
		out[titleKey(entry.Title)] = true
	}
	return out, nil
}

// titleKey is the identity two entries are compared on. Case and surrounding
// space are presentation; a sniffer that staged "We're going with X" alongside
// "we're going with x" would be a sniffer nobody trusts.
func titleKey(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}

// stagedOnly is a Store that can only be used to stage.
//
// It is not a validator. Entry.Validate already rejects an entry the schema
// forbids, and this is not a second copy of it: every rule here is about what
// THIS TIER may write, and every one of them describes an entry the schema would
// happily accept from `dira log`. A regex-tier entry that arrived `accepted`
// with a why_not attached is a perfectly valid ledger entry and a completely
// illegitimate one, and that distinction is what this type exists to hold.
type stagedOnly struct{ inner ledger.Store }

func (s stagedOnly) Get(ctx context.Context, id string) (*ledger.Entry, error) {
	return s.inner.Get(ctx, id)
}

func (s stagedOnly) List(ctx context.Context) ([]ledger.EntryInfo, error) {
	return s.inner.List(ctx)
}

func (s stagedOnly) Create(ctx context.Context, e *ledger.Entry) error {
	if err := mustStage(e); err != nil {
		return err
	}
	return s.inner.Create(ctx, e)
}

// Put is refused unconditionally. Replacing an entry is a mutation of something
// that already exists, and everything that already exists either was confirmed
// by a human or is waiting to be — neither is a regex's to overwrite. E2-L4's
// disposition flow owns that transition and holds an unwrapped store.
func (s stagedOnly) Put(context.Context, *ledger.Entry) error {
	return fmt.Errorf("%w: the regex tier creates staged entries and never replaces one", errRefused)
}

// Delete is refused for the same reason, one step stronger: dec-0002 makes the
// ledger the record, and nothing that infers may erase.
func (s stagedOnly) Delete(context.Context, string) error {
	return fmt.Errorf("%w: the regex tier never deletes an entry", errRefused)
}

// mustStage is the gate. Each rule names the thing it refuses to let a regex
// assert.
func mustStage(e *ledger.Entry) error {
	switch {
	case e == nil:
		return fmt.Errorf("%w: nil entry", errRefused)
	// Kind is checked before state, because a question's states are `open`
	// and `answered` and reporting "state is open" for one would name the
	// symptom instead of the mistake.
	case e.Kind != ledger.KindDecision:
		return fmt.Errorf("%w: kind is %q — the regex tier writes decisions only, and %q has no staged state to wait in",
			errRefused, e.Kind, e.Kind)
	case e.State != ledger.StateStaged:
		return fmt.Errorf("%w: state is %q — the regex tier writes %q and nothing else, because a pattern match is not evidence of intent (dec-0003)",
			errRefused, e.State, ledger.StateStaged)
	case e.Source == nil || e.Source.Tier != ledger.TierRegex:
		return fmt.Errorf("%w: source.tier must be %q, so a reader can always tell what a hook inferred from what a human confirmed",
			errRefused, ledger.TierRegex)
	case !stagingHook(e.Source.Hook):
		return fmt.Errorf("%w: source.hook is %q, which is not a capture point the regex tier writes from", errRefused, e.Source.Hook)
	case strings.TrimSpace(e.Source.Excerpt) == "":
		return fmt.Errorf("%w: no excerpt — a staged entry with no evidence is one a human cannot dispose of", errRefused)
	case len([]rune(e.Source.Excerpt)) > maxExcerpt:
		return fmt.Errorf("%w: excerpt is %d characters, want at most %d", errRefused, len([]rune(e.Source.Excerpt)), maxExcerpt)
	case e.ConfirmedBy != "":
		return fmt.Errorf("%w: confirmed_by is %q — nobody confirmed this, a regex found it", errRefused, e.ConfirmedBy)
	case len(e.Alternatives) != 0:
		return fmt.Errorf("%w: %d alternative(s) — a regex cannot know what was rejected or why, and a why_not it invented would be quoted back at the user as their own reasoning",
			errRefused, len(e.Alternatives))
	case len(e.Edges) != 0:
		return fmt.Errorf("%w: %d edge(s) — derives_from and supersedes are claims about the graph that tier 2 fills in (dec-0003)", errRefused, len(e.Edges))
	case e.ADR != "":
		return fmt.Errorf("%w: adr is set — the mirror is written when a decision is accepted, and this one is not", errRefused)
	case e.Private:
		return fmt.Errorf("%w: private is set — whether an entry is private is the ledger's tier and a human's judgement, not a pattern's", errRefused)
	}
	return nil
}
