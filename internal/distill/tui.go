package distill

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
)

// The one-keystroke loop: one card at a time, one byte to dispose of it.
//
// # What the keys do, and which decision settled each
//
//	y  Confirm  — dec-0022 and dec-0025. The human stood behind the capture;
//	              `confirmed_by: human` and a bumped `updated` land, and the
//	              entry stays `staged` until extraction supplies the alternative
//	              dec-0021 requires.
//	n  Discard  — dec-0024. The regex misread the transcript; the entry is
//	              deleted, and nothing is left behind that a later reader could
//	              cite as rationale.
//	e  Edit     — the human writes the because themselves and the card is
//	              disposed of on the strength of it. See "Why `e` also confirms".
//	u  Undo     — takes back the last of those three, byte for byte.
//	q  Quit     — leaves every remaining card staged and untouched.
//
// Any other byte is ignored: it advances nothing, writes nothing and prints
// nothing. A queue driven by muscle memory will be handed stray bytes — an
// arrow key arrives as three of them — and the alternative to ignoring them is
// disposing of a card the human did not look at.
//
// # No prompt, ever
//
// There is no confirmation step and no menu. The byte that arrives is the
// disposition; the loop does not ask "are you sure?" and does not wait for a
// newline. That is what makes the destructive key safe to press quickly: the
// safety is `u`, not a second question. A prompt would also cost a byte per
// card on a path where roughly one capture in four is a reject
// (docs/decisions-pending/E2-L1-report.md §3.3), which is the entire budget the
// design is spending.
//
// # Why `e` also confirms
//
// `e` opens the body, and a body is not enough to move a decision out of
// `staged` — dec-0021 wants an alternative with a why_not, and a body is prose,
// not an alternative. So an `e` that only edited would leave its card exactly as
// it found it: `staged`, unmarked, and presented again on the next run as though
// nobody had looked at it. A human who has just written the reasoning by hand
// has stood behind the capture at least as firmly as one who pressed `y`, so the
// loop records that the same way dec-0025 records `y`: `confirmed_by: human`.
//
// The two writes stay two functions on purpose. The body edit is E2-L4-T6's
// (Options.Edit), and its own contract is that the frontmatter comes back
// byte-identical apart from `updated`; the mark is Confirm's. Folding the mark
// into the editor would break that contract, and folding the edit into Confirm
// would put an $EDITOR on `y`, which dec-0022 and dec-0025 each refused.
//
// # No terminal in any signature
//
// Loop takes a KeySource and a Display, not an io.Reader and an io.Writer, for
// the reason the package comment gives: E6's web surface drives this package
// too, and it has no file descriptors to hand over. Reading one *key* rather
// than a stream of bytes is also what makes "one keystroke" a property the
// package can hold rather than a promise its callers have to keep — there is no
// buffer here that could have swallowed a second byte.

// ActEdit is `e`: the human wrote the because and the card was disposed of on
// the strength of it. It is one act even though it lands as two writes, because
// it is one keystroke and `u` has to take all of it back.
const ActEdit Act = "edit"

// The five keys. They are bytes rather than runes because a keystroke loop reads
// bytes; a multi-byte key (an arrow, an accented letter) arrives as bytes none
// of which is one of these and is therefore ignored, which is the correct
// handling of it.
const (
	KeyConfirm byte = 'y'
	KeyDiscard byte = 'n'
	KeyEdit    byte = 'e'
	KeyUndo    byte = 'u'
	KeyQuit    byte = 'q'
)

// A KeySource yields one keystroke at a time.
//
// One byte per call, and no way to ask for more than one, so the loop cannot
// require a newline even by accident: there is no read here that could block
// waiting for the rest of a line. io.EOF ends the session as though `q` had been
// pressed — a driver with nothing left to say is a human who has stopped
// answering, and the remaining cards stay staged.
//
// E2-L4-T7 supplies the implementation that reads a real terminal, including
// what to do when stdin is not one. Nothing in this file knows what a TTY is.
type KeySource interface {
	ReadKey() (byte, error)
}

// A Display is where a rendered card goes. One call is one thing to show: a
// card, or a single line.
//
// It is not an io.Writer, and the difference is the point rather than an
// affectation. A web surface has a screen, not a stream, and a Show(string) it
// can implement by setting a field; an io.Writer here would make E6 fake one.
type Display interface {
	Show(text string)
}

// An Editor is what `e` runs: it edits the body of a staged entry and returns
// the entry as it now stands.
//
// It takes the entry the card was rendered from rather than an id, so the editor
// does not re-read a file the loop has already read, and it returns an entry
// rather than writing through the caller, so the loop can hand the pre-edit
// value to `u`. E2-L4-T6 implements it; a nil Editor makes `e` a one-line
// refusal rather than a crash.
type Editor func(ctx context.Context, store ledger.Store, e *ledger.Entry, now time.Time) (*ledger.Entry, error)

// maxDispositionFailures bounds the retry loop for a card whose disposition
// keeps failing. See the KeyConfirm branch for why a bound is needed at all.
const maxDispositionFailures = 3

// A Renderer turns one card into the text shown for it. position is 1-based and
// total is the number of cards this session started with, which is the `1 of 3`
// the design promises.
//
// E2-L4-T8 supplies the real one, whose copy is part of the deliverable. The
// fallback in this file exists so the loop is testable and runnable before that
// lands, not to compete with it.
type Renderer func(item Item, position, total int) string

// Options is everything Loop needs. Store, Keys and Now are required; the rest
// have defined behaviour when they are nil.
type Options struct {
	// Store is the ledger, read for the queue and written for every
	// disposition.
	Store ledger.Store

	// Keys is where keystrokes come from.
	Keys KeySource

	// Display is where cards and lines go. A nil Display shows nothing,
	// which is a legitimate configuration for a caller that only wants the
	// writes.
	Display Display

	// Now stamps `updated`. It is a function rather than a time because a
	// session disposes of several cards and each one records when it was
	// disposed of, and it is a parameter at all for the reason ledger.Add
	// takes one: the value goes into the permanent record.
	Now func() time.Time

	// Edit is `e`. A nil Editor makes `e` refuse in one line and leave the
	// card where it is.
	Edit Editor

	// Render is how a card is shown. Nil uses this package's fallback.
	Render Renderer
}

// A Result is what one session did, for a surface that has to report it.
type Result struct {
	// Dispositions are the acts that stand, in the order they were
	// performed. An undone act is not here: `u` takes it off the record as
	// well as off the disk.
	Dispositions []Disposition

	// Undone counts the acts `u` took back.
	Undone int

	// Quit is true if the session ended on `q` or on the driver running out
	// of input, rather than on the queue emptying.
	Quit bool

	// Remaining is how many cards were left undisposed.
	Remaining int

	// Warnings is the queue's account of entry files it could not read,
	// carried through so a caller that passed no Display can still report
	// them.
	Warnings []string
}

// Loop presents every card awaiting a human and disposes of each one on a single
// keystroke.
//
// It reads the queue once. The cards it shows are the entries as their files
// read at that moment, and every write is made against that value rather than
// against a re-read — which is what keeps the reject path to one keystroke and
// one ledger mutation with no second read in between. A concurrent writer that
// changes an entry mid-session is not a case this loop tries to win; the queue
// is a human's working set for the next few seconds, and `dira sniff` staging a
// new capture behind it is served by the next invocation.
//
// It reads no key when there is nothing to dispose of: an empty queue costs zero
// bytes of input, which is what lets E2-L4-T7 answer the non-interactive case
// without ever entering raw mode.
//
// Undo is offered while the queue still holds a card. When the last card is
// disposed of the session is over, and the loop returns rather than asking for a
// byte it has nothing to spend on — a loop that stayed open for a possible `u`
// would turn the scripted three keystrokes for three cards into four.
func Loop(ctx context.Context, opts Options) (*Result, error) {
	if opts.Store == nil {
		return nil, errors.New("distill: no ledger")
	}
	if opts.Keys == nil {
		return nil, errors.New("distill: no keys; the loop is driven by keystrokes")
	}
	if opts.Now == nil {
		return nil, errors.New("distill: no clock; updated is stamped by the caller")
	}

	queue, err := Staged(ctx, opts.Store)
	if err != nil {
		return nil, err
	}

	result := &Result{Warnings: queue.Warnings}
	show := func(text string) {
		if opts.Display != nil {
			opts.Display.Show(text)
		}
	}
	for _, warning := range queue.Warnings {
		show(warning)
	}

	cards := queue.Awaiting()
	result.Remaining = len(cards)
	if len(cards) == 0 {
		return result, nil
	}

	render := opts.Render
	if render == nil {
		render = plainCard
	}

	// The single level of undo. It is one value rather than a stack because
	// dec-0024's mitigation is "the last disposition", and a deeper history
	// would mean holding the pre-image of every entry disposed of in a
	// session in order to serve a keystroke nobody has asked for twice.
	var last *Disposition
	failures := 0

	position, redraw := 0, true
	for position < len(cards) {
		if redraw {
			show(render(cards[position], position+1, len(cards)))
			redraw = false
		}

		key, err := opts.Keys.ReadKey()
		if err != nil {
			if errors.Is(err, io.EOF) {
				result.Quit = true
				return result, nil
			}
			return result, fmt.Errorf("reading a keystroke: %w", err)
		}

		switch key {
		case KeyQuit:
			result.Quit = true
			return result, nil

		case KeyConfirm, KeyDiscard, KeyEdit:
			disposition, err := dispose(ctx, opts, key, cards[position].Entry)
			if err != nil {
				// A failed disposition does not advance, so a human can see the
				// reason and try again or quit. That is right for a human and
				// unbounded for anything else: a key source that never ends,
				// against a store failing persistently, spins here forever.
				// Found by E2-L4-T7 through mutation, in merged code.
				//
				// Three tries per card, then the error is returned. A human gets
				// room to retry; a runaway loop terminates. The counter resets on
				// any successful disposition or any move to another card, so
				// transient failures never accumulate across the session.
				failures++
				if failures >= maxDispositionFailures {
					return result, fmt.Errorf("disposing %s: %w (gave up after %d attempts on the same card)",
						cards[position].Entry.ID, err, failures)
				}
				show(oneLine(err.Error()))
				continue
			}
			failures = 0
			last = disposition
			result.Dispositions = append(result.Dispositions, *disposition)
			result.Remaining--
			position++
			redraw = true

		case KeyUndo:
			if last == nil {
				show("dira distill: nothing to undo")
				continue
			}
			restored, err := undo(ctx, opts.Store, last)
			if err != nil {
				show(oneLine(err.Error()))
				continue
			}
			position--
			cards[position].Entry = restored
			cards[position].Status = statusOf(restored)
			result.Dispositions = result.Dispositions[:len(result.Dispositions)-1]
			result.Undone++
			result.Remaining++
			last = nil
			redraw = true

		default:
			// Ignored: no advance, no write, no line. See the file
			// comment.
		}
	}
	return result, nil
}

// dispose performs the act one key names, and is where `e`'s two writes are held
// together as one act.
func dispose(ctx context.Context, opts Options, key byte, entry *ledger.Entry) (*Disposition, error) {
	switch key {
	case KeyConfirm:
		return Confirm(ctx, opts.Store, entry, opts.Now())

	case KeyDiscard:
		return Discard(ctx, opts.Store, entry)

	case KeyEdit:
		if opts.Edit == nil {
			return nil, fmt.Errorf("distill: no editor is configured, so %s cannot be edited here", entry.ID)
		}
		edited, err := opts.Edit(ctx, opts.Store, entry, opts.Now())
		if err != nil {
			return nil, fmt.Errorf("editing %s: %w", entry.ID, err)
		}
		if edited == nil {
			return nil, fmt.Errorf("editing %s: the editor returned no entry", entry.ID)
		}
		confirmed, err := Confirm(ctx, opts.Store, edited, opts.Now())
		if err != nil {
			return nil, err
		}
		// One act, so `u` restores the entry as it was before the body
		// was touched and not as it was between the two writes.
		return &Disposition{ID: entry.ID, Act: ActEdit, Before: entry, After: confirmed.After}, nil
	}
	return nil, fmt.Errorf("distill: %q is not a disposition", string(rune(key)))
}

// undo restores what the last disposition changed, byte for byte.
//
// Byte-identical is not an aspiration here, it is a consequence of what is
// restored: Disposition.Before is the entry as it was decoded from the file,
// carrying the codec's memory of how every scalar was written (see
// ledger.Entry.style), so re-encoding it reproduces the original file rather
// than a re-formatted equivalent of it. Nothing in this function edits that
// value.
func undo(ctx context.Context, store ledger.Store, d *Disposition) (*ledger.Entry, error) {
	if d == nil || d.Before == nil {
		return nil, errors.New("distill: nothing to undo")
	}

	switch d.Act {
	case ActDiscard:
		// Create rather than Put: the entry is supposed to be gone, so
		// if anything has taken the id back the restore must fail loudly
		// instead of writing over it.
		if err := store.Create(ctx, d.Before); err != nil {
			return nil, fmt.Errorf("restoring %s: %w", d.ID, err)
		}

	case ActConfirm, ActEdit:
		// Only what this loop wrote may be taken back. If the file has
		// moved on since — a concurrent editor, a hand edit in another
		// window — undo would be a silent revert of somebody else's
		// work, which is a worse failure than refusing.
		current, err := store.Get(ctx, d.ID)
		if err != nil {
			return nil, fmt.Errorf("re-reading %s to undo it: %w", d.ID, err)
		}
		unchanged, err := sameFile(current, d.After)
		if err != nil {
			return nil, fmt.Errorf("comparing %s with what was written: %w", d.ID, err)
		}
		if !unchanged {
			return nil, fmt.Errorf("distill: refusing to undo %s: its file has changed since the disposition, "+
				"and restoring it would discard that change", d.ID)
		}
		if err := store.Put(ctx, d.Before); err != nil {
			return nil, fmt.Errorf("restoring %s: %w", d.ID, err)
		}

	default:
		return nil, fmt.Errorf("distill: %q is not an act this loop can undo", d.Act)
	}
	return d.Before, nil
}

// sameFile reports whether two entries would serialise to the same bytes.
//
// The comparison is over the encoded form rather than over the structs because
// what an undo would overwrite is a file, and two entries that differ only in
// how a scalar was quoted are two different files.
func sameFile(a, b *ledger.Entry) (bool, error) {
	if a == nil || b == nil {
		return a == b, nil
	}
	left, err := ledger.Encode(a)
	if err != nil {
		return false, err
	}
	right, err := ledger.Encode(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(left, right), nil
}

// plainCard is the fallback renderer: enough of a card to drive the loop, and
// nothing invented.
//
// dec-0019 applies to it as much as to E2-L4-T8's real one — the body, the
// excerpt and the source line render only where the entry records them, and
// there is no summary, no alternative and no ADR line, because a regex-staged
// entry has none of those and nothing here may supply them.
func plainCard(item Item, position, total int) string {
	if item.Entry == nil {
		return ""
	}
	entry := item.Entry

	var b strings.Builder
	fmt.Fprintf(&b, "%d of %d\n", position, total)
	if body := strings.TrimSpace(entry.Body); body != "" {
		b.WriteString(body + "\n\n")
	}
	b.WriteString(entry.Title + "\n")
	if line := sourceLine(entry); line != "" {
		b.WriteString(line + "\n")
	}
	if entry.Source != nil && entry.Source.Excerpt != "" {
		b.WriteString(entry.Source.Excerpt + "\n")
	}
	b.WriteString(keyLegend)
	return b.String()
}

// keyLegend names the five keys in the register dec-0024 settled: `n` is "not a
// decision", not "reject", because the two meanings collapse back together the
// moment the card uses the word that covers both.
//
// It is a legend and not a prompt. Nothing here asks a question, so nothing here
// waits for an answer to one.
const keyLegend = "y stand behind it · n not a decision · e edit the because · u undo · q leave the rest"

// sourceLine is the provenance line: the hook, the time and the tier, in that
// order, and only the parts the entry records.
func sourceLine(e *ledger.Entry) string {
	if e.Source == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if e.Source.Hook != "" {
		parts = append(parts, string(e.Source.Hook))
	}
	if stamp := clockOf(e.Created); stamp != "" {
		parts = append(parts, stamp)
	}
	if e.Source.Tier != "" {
		parts = append(parts, string(e.Source.Tier))
	}
	return strings.Join(parts, " · ")
}

// clockOf is the wall-clock part of an RFC3339 stamp, or "" if it does not look
// like one. It reads the recorded text rather than parsing to a time, because
// the stamp is stored as text for a reason (see ledger.Entry) and a card that
// re-rendered it through a parser would show an hour the file does not contain.
func clockOf(stamp string) string {
	_, rest, found := strings.Cut(stamp, "T")
	if !found || len(rest) < 5 {
		return ""
	}
	return rest[:5]
}
