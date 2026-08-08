// Package distill is the disposition flow: the queue of staged entries awaiting
// a human, and the transitions that dispose of them.
//
// # No terminal in any signature
//
// Everything exported here takes and returns values. `dira distill` is one
// surface over this package and E6's web distill screen is another, and a TTY,
// an io.Reader or an io.Writer in a signature here would push the semantics up
// into whichever surface happened to call first — leaving the other to
// reimplement them and drift. That is why a malformed entry produces a warning
// *string on the Queue* rather than a line printed to stderr: the queue reports
// what it skipped, and the surface decides how a human hears about it.
//
// # The queue reads files, never the cache
//
// dec-0002 says that where the derived cache and the entry files disagree, the
// files win. For most read paths that is a property the index already
// guarantees, because index.Open reconciles before it answers. This queue does
// not use the index at all, and that is deliberate rather than an omission: the
// queue's whole job is to put a card in front of a human and then write to the
// file behind it, so the one thing it must never do is offer to dispose of an
// entry whose file no longer says what the answer said it did. It costs one read
// per entry, which is the price of the guarantee (L-0009).
package distill

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kazi-org/dira/internal/ledger"
)

// Status is where a staged entry sits between capture and a usable record.
//
// Two values, and the second one exists because dec-0022 created a state that
// nothing on disk names directly. Confirming a regex-staged decision promotes it
// to the semantic tier for extraction — it does not accept it — so a confirmed
// entry stays `state: staged` and is indistinguishable, by state alone, from one
// nobody has looked at. dec-0022 says what that costs if it is left invisible:
// "the queue therefore has to show pending extraction as a state a human can
// see, or entries will silently pile up in staged looking rejected."
//
// Nothing new is stored to carry this. dec-0019's rule applies to a queue as
// much as to a renderer — derive from what is recorded, never invent a field —
// and the promotion is already recorded twice over, in `confirmed_by` and in
// `source.tier`. Status is that derivation, named.
type Status string

// The two places a staged entry can be.
const (
	// StatusAwaiting means nobody has disposed of this yet. It is the queue's
	// actionable set: a regex-tier proposal with no human's mark on it.
	StatusAwaiting Status = "awaiting"

	// StatusPendingExtraction means a human confirmed it and the reasoning
	// that would let it leave `staged` has not arrived. dec-0021 requires at
	// least one alternative with a why_not the moment a decision stops being
	// staged, and dec-0022 routed that to the semantic tier rather than to
	// the keystroke. Until extraction lands, this is where the entry waits.
	StatusPendingExtraction Status = "pending extraction"
)

// An Item is one card: the entry as its file reads, and where it sits.
type Item struct {
	// Entry is read from the file, never from the cache, so every field a
	// card renders is the field the disposition will be written against.
	Entry *ledger.Entry

	// Status is derived from Entry, not stored. See Status.
	Status Status
}

// ID is the entry's id, for callers that only want to name the card.
func (i Item) ID() string {
	if i.Entry == nil {
		return ""
	}
	return i.Entry.ID
}

// A Queue is every staged entry in a ledger, in a fixed order, plus an account
// of what could not be read.
type Queue struct {
	// Items are the staged entries in id order, ascending. See Staged for
	// why that order and not another.
	Items []Item

	// Warnings is one line per entry file the queue could not read. They are
	// values rather than something printed, because this package has no
	// terminal (see the package comment), and they are on the Queue rather
	// than folded into an error because a ledger with one bad file still has
	// a queue — dec-0002 chose one file per entry precisely so a malformed
	// edit takes down one entry instead of all of them.
	//
	// Empty on a ledger that reads cleanly.
	Warnings []string
}

// Len is how many cards the queue holds, including those pending extraction.
func (q *Queue) Len() int { return len(q.Items) }

// Awaiting is the subset a human can act on: everything not already confirmed
// and waiting on extraction.
//
// The split is here rather than in Staged because both halves have to stay
// visible. A surface that filtered pending entries out at read time would show
// an empty queue while promoted entries accumulated behind it, which is the
// failure dec-0022 named; a surface that offered them as cards would ask the
// human to confirm the same entry twice.
func (q *Queue) Awaiting() []Item {
	out := make([]Item, 0, len(q.Items))
	for _, item := range q.Items {
		if item.Status == StatusAwaiting {
			out = append(out, item)
		}
	}
	return out
}

// PendingExtraction is the complement of Awaiting: entries a human has already
// confirmed, still staged because their reasoning has not arrived.
func (q *Queue) PendingExtraction() []Item {
	out := make([]Item, 0, len(q.Items))
	for _, item := range q.Items {
		if item.Status == StatusPendingExtraction {
			out = append(out, item)
		}
	}
	return out
}

// Staged returns every entry in the ledger whose file says `state: staged`.
//
// It takes a ledger.Store and not a directory, so `dira distill` over the local
// backend and E7's github backend are the same code path, and so nothing in this
// package has to know what a path is (dec-0005).
//
// # The order, and why it is this one
//
// Ascending by id, which is what Store.List already guarantees and which this
// function only ever filters. The design's `1 of 3` progress indicator is a
// promise that the second invocation shows the same card second, so the order
// has to be total and it has to be derivable without parsing anything.
//
//   - Ids are unique, so the order is total: no tie-break is needed, and none
//     can be got wrong.
//   - Ids are allocated ascending by ledger.Add, and dec-0021 lets only
//     decisions be staged, so id order is capture order for everything in this
//     queue.
//   - Ordering by `created` instead would mean comparing RFC3339 text whose
//     offset form the schema does not pin, so two entries could sort by their
//     timezone rather than by their instant. Nothing here needs an instant.
//
// The order is also unchanged by a cache rebuild, for the strong reason rather
// than the lucky one: no cache is consulted.
//
// # What is skipped and what is reported
//
// An entry that vanished between the listing and the read is skipped in silence.
// That is a concurrent writer's business — `dira sniff` runs unattended from
// hooks — and it is not a fact about this ledger a human can act on.
//
// An entry whose file cannot be parsed is skipped with a warning naming it. It
// is not an error: one malformed file must not cost a human the other nine
// cards. It is also not silent, because an entry missing from every answer is
// the same failure as a stale one.
//
// An empty ledger yields an empty Queue and no error.
func Staged(ctx context.Context, store ledger.Store) (*Queue, error) {
	if store == nil {
		return nil, errors.New("distill: no ledger")
	}

	infos, err := store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading the ledger: %w", err)
	}

	queue := &Queue{Items: make([]Item, 0, len(infos))}
	for _, info := range infos {
		entry, err := store.Get(ctx, info.ID)
		if err != nil {
			if errors.Is(err, ledger.ErrNotFound) {
				continue
			}
			queue.Warnings = append(queue.Warnings, skipped(info.ID, err))
			continue
		}
		if entry.State != ledger.StateStaged {
			continue
		}
		queue.Items = append(queue.Items, Item{Entry: entry, Status: statusOf(entry)})
	}
	return queue, nil
}

// statusOf derives an Item's Status from the entry's own fields, and from
// nothing else.
//
// Either mark is enough on its own, and that is not belt-and-braces. dec-0022
// describes the promotion in terms of the tier ("promotes it to the semantic
// tier"), while the lane's acceptance line describes it in terms of the human
// ("the confirmed entry gains confirmed_by: human"). They are two records of one
// act, written by one transition, and a queue that keyed on only one of them
// would show a promoted entry as un-dispositioned if that transition ever
// recorded the other first. Reading either as the promotion costs nothing: a
// regex-tier entry carries neither, because internal/sniff is structurally
// incapable of writing them (see stagedOnly.mustStage).
func statusOf(e *ledger.Entry) Status {
	if e.ConfirmedBy != "" {
		return StatusPendingExtraction
	}
	if e.Source != nil && e.Source.Tier != "" && e.Source.Tier != ledger.TierRegex {
		return StatusPendingExtraction
	}
	return StatusAwaiting
}

// skipped is the one line a surface prints for an unreadable entry.
//
// The whitespace is collapsed because the reason comes from a YAML parser, and a
// parser that decides to wrap would turn one warning into three lines in the
// middle of a full-screen card. One entry, one line.
func skipped(id string, err error) string {
	return oneLine(fmt.Sprintf("dira distill: skipping %s (%v)", id, err))
}

// oneLine flattens any run of whitespace, including newlines, to single spaces.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
