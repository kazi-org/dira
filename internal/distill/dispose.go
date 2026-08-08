package distill

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
)

// The two dispositions, and the two decisions that settled what they write.
//
// # `y` — dec-0025
//
// Confirm writes `confirmed_by: human` and a bumped `updated`, and nothing else.
// The state stays `staged`. That is not a half-finished transition: dec-0022
// ruled that confirming a regex-staged decision hands it to extraction rather
// than accepting it, and dec-0025 ruled what lands on disk while it waits.
// `entry.schema.json` requires ≥1 `alternatives` the moment a decision leaves
// `staged` (dec-0021) and `dira sniff` writes none, so a bare state flip on `y`
// would produce a file the published schema rejects. `confirmed_by` closes the
// gap without touching the schema: a staged decision carrying it is one a human
// stood behind, a staged decision without it is one nobody has looked at.
//
// # `n` — dec-0024
//
// Discard deletes the entry. `n` says *the regex misread me, this was never a
// decision* — it disposes of the capture, not of an option. The other reading,
// *we considered that and decided against it*, is a real decision and goes to
// `y`; it is unwritable from a keystroke anyway, because it would need a
// `why_not` that only invention could supply (dec-0003 forbids it, dec-0019
// names it).
//
// # What neither of them does
//
// Neither transition composes text. dec-0019's rule — the renderer derives
// presentation from recorded prose, never invents a field, and where nothing can
// be derived renders nothing — is applied here to a write: no alternative, no
// why_not, no body and no state this package chose for the human appears in
// anything either function returns. Where a rule says refuse, the transition
// refuses and writes nothing rather than filling in a field the human did not
// write.
//
// # No terminal in any signature
//
// Both take values and return values, as everything exported from this package
// does (see the package comment). `dira distill` is one surface over them and
// E6's web screen is another; an io.Reader, an io.Writer or a TTY here would
// push the semantics up into whichever called first and let the two drift.

// ConfirmedByHuman is what Confirm records in `confirmed_by`.
//
// It names the *kind* of authority rather than a person: the field's own
// description in entry.schema.json is "Who dispositioned this entry", and what
// the keystroke supplies is that a human did — not which one. dira has no
// identity, no account and no network (cst-0004), so a name here would have to
// be guessed from the environment, and a guess in the permanent record is
// exactly what dec-0019 forbids.
const ConfirmedByHuman = "human"

// An Act names which disposition was performed.
type Act string

// The two acts a keystroke can perform.
const (
	// ActConfirm is `y`: a human stood behind the capture.
	ActConfirm Act = "confirm"

	// ActDiscard is `n`: the capture was wrong and the entry is gone.
	ActDiscard Act = "discard"
)

// A Disposition is what one transition did, for a surface that has to report it
// or take it back.
//
// Before is the entry exactly as it was, and it is on the result rather than
// left for the caller to have remembered. A discard deletes a file that is very
// often minutes old and was never committed — `dira sniff` runs from the Stop
// hook — so the single-level undo the queue is required to offer (E2-L4-T5) has
// no other source for what it would restore. Handing it back here is what makes
// that undo possible without a second read of a file that no longer exists.
type Disposition struct {
	// ID is the entry the act was performed on.
	ID string

	// Act is which disposition ran.
	Act Act

	// Before is the entry as it was before the write, unmodified.
	Before *ledger.Entry

	// After is the entry as it now stands, and nil for a discard — where
	// there is no longer an entry to describe. That nil is the record of
	// dec-0024's ruling in the type: a discarded capture leaves nothing a
	// later reader could cite as rationale.
	After *ledger.Entry
}

// errNotStaged is the refusal both transitions share.
//
// Only a staged entry is a card. Everything else in the ledger is either
// enforcement substrate or somebody's standing record, and a keystroke aimed at
// a queue must not be able to reach it: `dira check` reads accepted and rejected
// decisions and active constraints, and a `y` or an `n` that landed on one would
// edit the record it is enforced by.
var errNotStaged = errors.New("distill: not a staged entry")

// Confirm records that a human stood behind a staged capture (dec-0025).
//
// It writes `confirmed_by: human` and `updated`, leaves `state: staged` and
// every other frontmatter field alone, and performs exactly one ledger write.
// The entry it was given is not modified.
//
// now is a parameter for the reason ledger.Add makes `created` one: the value
// goes into the permanent record, and a package that read a clock itself would
// put a timestamp nobody chose into a file and leave a test unable to say what
// time it is.
func Confirm(ctx context.Context, store ledger.Store, e *ledger.Entry, now time.Time) (*Disposition, error) {
	if err := disposable(store, e); err != nil {
		return nil, err
	}
	if e.ConfirmedBy != "" {
		// Already dispositioned. Writing again would bump `updated` for
		// an act that is already on the record, which is a change to the
		// permanent record with nothing behind it.
		return nil, fmt.Errorf("distill: %s already carries confirmed_by %q; confirming it again would record nothing and bump updated",
			e.ID, e.ConfirmedBy)
	}
	if now.IsZero() {
		return nil, errors.New("distill: no clock; updated is stamped by the caller")
	}

	// A copy, so a refusal further down cannot leave the caller holding a
	// half-applied entry and so Disposition.Before is genuinely the input.
	after := *e
	after.ConfirmedBy = ConfirmedByHuman
	after.Updated = now.UTC().Format(time.RFC3339)

	if err := store.Put(ctx, &after); err != nil {
		return nil, fmt.Errorf("confirming %s: %w", e.ID, err)
	}
	return &Disposition{ID: e.ID, Act: ActConfirm, Before: e, After: &after}, nil
}

// Discard deletes a staged capture (dec-0024).
//
// It performs exactly one ledger mutation and leaves nothing behind: no
// tombstone note, no marked state, no field recording that something was here.
// dec-0024 refused each of those in turn — a `note` tombstone makes the reject
// path two writes on the path taken roughly 27% of the time and re-surfaces the
// rejected capture in tomorrow's brief, and a `rejected_capture` mark is a
// schema change understood by exactly one surface.
//
// It refuses an entry that already carries `confirmed_by`. dec-0024's case for
// deletion being safe is a case about a *capture*: nothing citable is destroyed,
// because a staged decision is never enforcement substrate, and the excerpt is a
// quotation whose original is still on disk. None of that is a reason to delete
// something a human already stood behind, and the queue never offers one —
// Queue.Awaiting excludes them. The refusal is here so that the guarantee does
// not depend on every future surface having remembered.
func Discard(ctx context.Context, store ledger.Store, e *ledger.Entry) (*Disposition, error) {
	if err := disposable(store, e); err != nil {
		return nil, err
	}
	if e.ConfirmedBy != "" {
		return nil, fmt.Errorf("distill: refusing to discard %s: it carries confirmed_by %q, so it is no longer "+
			"an unexamined capture — dec-0024 disposes of captures, and a human has stood behind this one", e.ID, e.ConfirmedBy)
	}

	if err := store.Delete(ctx, e.ID); err != nil {
		return nil, fmt.Errorf("discarding %s: %w", e.ID, err)
	}
	return &Disposition{ID: e.ID, Act: ActDiscard, Before: e}, nil
}

// disposable is the precondition both transitions share, checked before either
// touches the store so that a refusal writes nothing.
func disposable(store ledger.Store, e *ledger.Entry) error {
	if store == nil {
		return errors.New("distill: no ledger")
	}
	if e == nil {
		return errors.New("distill: no entry")
	}
	if e.ID == "" {
		return errors.New("distill: the entry carries no id")
	}
	if e.State != ledger.StateStaged {
		return fmt.Errorf("%w: %s is %s, and only a staged entry is a card in this queue",
			errNotStaged, e.ID, e.State)
	}
	return nil
}
