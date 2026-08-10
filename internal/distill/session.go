package distill

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
)

// The session: everything that has to be decided *before* the keystroke loop is
// allowed to ask a human for a byte.
//
// # The failure this file exists to make structurally impossible
//
// `dira sniff` runs from the Stop hook, so `dira distill` will be reached for in
// the same places: a hook, a wrapper script, a CI step, a `dira distill` at the
// end of a pipeline whose stdin is a pipe carrying somebody else's JSON. A
// review loop that blocks on a keystroke in any of those is not a degraded
// experience, it is a hung agent session — the tool stops being a tool and
// becomes the reason the session never returns. That is worse than the command
// not existing.
//
// The guarantee here is therefore structural rather than careful. A keystroke
// can only be read through a KeySource, and the only way to obtain one is
// Terminal.Raw, and Run calls Raw only when Terminal.Interactive says a human is
// at the other end *and* the queue holds a card to spend a byte on. There is no
// path through this file that reads on a terminal that is not one, because there
// is no reader to read with.
//
// # Why the non-interactive answer is "read nothing", not "read what is there"
//
// E2-L4-T7's acceptance line describes stdin as "not a TTY and no input
// available", which invites a design that peeks: read if something is buffered,
// give up if not. That design is refused, and the refusal is the safer answer to
// the same question.
//
//   - "No input available" is not a state a pipe can be asked about without a
//     non-blocking read, and a non-blocking read that finds nothing this
//     microsecond says nothing about the next one. The check would be a race
//     whose losing side is the hang it was added to prevent.
//   - A hook's stdin is not empty, it is somebody's payload. This repository
//     already relies on that: `dira sniff` reads Claude Code's Stop payload as
//     JSON on stdin (cmd/dira/sniff.go). A distill that consumed a byte of one
//     would corrupt the thing that invoked it, and a `y` sitting inside that
//     JSON would dispose of a capture nobody looked at.
//   - Refusing to read at all is a superset of the acceptance line: a session
//     that never reads cannot block on a read, whatever is or is not buffered.
//
// So: not a terminal, one line, exit 0, nothing read and nothing written. Exit 0
// and not a failure code, for the reason `dira sniff` gives for the same choice:
// a non-zero exit means dira is broken, not that there was nothing to do.
// Captures waiting for a human in a context that has no human is the ordinary
// state of a machine, and failing there would fail the hook that called it.
//
// # Where the TTY check itself lives
//
// Not here. Deciding whether stdin is a terminal means asking the operating
// system about a file descriptor, and dec-0005's boundary (enforced by
// internal/ledger/boundary_test.go) does not put this package on the list of
// those that may import `os`. It does not need to be: whether a human is present
// is a fact about the surface, and this package takes it as an injected value
// exactly as it takes the clock and the editor. `cmd/dira` is on that list and
// owns the answer; E6's web screen answers it differently again, and neither has
// to reimplement what to do about it.

// ErrInterrupted is the session ending because something outside it said stop:
// a cancelled context, or a read the operating system cut short because a signal
// arrived.
//
// It is returned rather than swallowed. An interrupt is not a `q` — `q` is a
// human saying "leave the rest staged", and the loop records it as such — and a
// session that reported an interrupt as an ordinary quit would tell its caller
// the queue had been worked through when it had not. What the interrupt does
// *not* affect is the terminal: it is restored on this path exactly as on every
// other, which is the property the acceptance line pins.
var ErrInterrupted = errors.New("distill: interrupted")

// A Restore puts the terminal back the way it was found.
//
// It is returned by Raw rather than being a second method, so there is no way to
// enter raw mode without also being handed the thing that leaves it. A caller
// can still fail to call it; Run calls it from a deferred function, so the panic
// path, the error path and the ordinary path are the same path.
type Restore func() error

// A Terminal is the process's end of a human, as a value.
//
// # Two methods, and why keystrokes hang off the second one
//
// Raw returns the KeySource rather than the Terminal simply having a ReadKey.
// That is the whole anti-hang guarantee expressed as a type: reading a key
// requires a KeySource, obtaining one requires entering raw mode, and entering
// raw mode is something Run does only after Interactive has said yes. A Terminal
// that also satisfied KeySource directly would leave "do not read on a pipe" as
// a rule someone has to remember at every call site, which is precisely how the
// hang gets reintroduced later by a change that looks harmless.
//
// It is also faithful to what a real terminal requires. Reading one byte with no
// echo and no line discipline *is* raw mode; there is no cooked-mode read that
// delivers a single keystroke. Coupling the two in the interface says so.
//
// # No file descriptor anywhere in it
//
// Interactive returns a bool and Raw returns two values and an error. Nothing
// here is an io.Reader, an io.Writer or an *os.File, for the reason the package
// comment gives and for the narrower one this file adds: an *os.File in this
// interface would drag `os` into every implementation, including E6's web screen,
// which has no descriptors to hand over and no raw mode to enter.
type Terminal interface {
	// Interactive reports whether keystrokes can be read from this terminal
	// — in the ordinary implementation, whether stdin is a TTY.
	Interactive() bool

	// Raw switches the terminal into raw mode and returns the source of
	// keystrokes together with the call that switches it back. The Restore
	// is non-nil whenever the error is nil.
	Raw() (KeySource, Restore, error)
}

// SessionOptions is everything Run needs. Store, Terminal and Now are required;
// the rest behave as Options says when they are nil.
type SessionOptions struct {
	// Store is the ledger, read for the queue and written for every
	// disposition.
	Store ledger.Store

	// Terminal answers whether there is a human, and is where raw mode is
	// entered and left if there is.
	Terminal Terminal

	// Display is where the session's one line, the queue's warnings and
	// every card go. A nil Display shows nothing; SessionResult still
	// reports what would have been shown.
	Display Display

	// Now stamps `updated`. See Options.Now.
	Now func() time.Time

	// Edit is `e`. See Options.Edit.
	Edit Editor

	// Render is how a card is shown. See Options.Render.
	Render Renderer
}

// A SessionResult is what one invocation did, in the terms a surface has to
// report it in.
//
// Everything the session showed is also on this value. A caller that passed no
// Display — a test, or a surface that renders elsewhere — can still say what
// happened, and a caller that passed one can assert that what it received is
// what the session decided rather than a second string composed alongside it.
type SessionResult struct {
	// Waiting is how many cards were awaiting a human when the session
	// started: Queue.Awaiting, read once, before anything was decided.
	Waiting int

	// Pending is how many entries were confirmed and waiting on extraction
	// (dec-0022). They are not cards and no keystroke is offered for them,
	// but a session that never mentioned them would let them pile up in
	// `staged` looking rejected, which is the failure dec-0022 named.
	Pending int

	// Interactive is what the Terminal answered.
	Interactive bool

	// EnteredRaw records whether raw mode was entered. False on both of the
	// paths this task exists for, and it is on the result rather than left
	// implicit so "without entering raw mode" is a claim a caller can check
	// as well as a test.
	EnteredRaw bool

	// Summary is the one line the session printed instead of running the
	// loop, and "" when the loop ran. It never contains a newline.
	Summary string

	// Loop is what the keystroke loop did, and nil when it never ran.
	Loop *Result

	// Warnings is the queue's account of entry files it could not read,
	// carried through as Result.Warnings is.
	Warnings []string
}

// Run is one invocation of the disposition flow, from reading the ledger to
// leaving the terminal as it was found.
//
// # The three answers
//
//	no cards            one line, no input read, raw mode never entered
//	cards, no human     one line, no input read, raw mode never entered, no write
//	cards and a human   raw mode, the keystroke loop, raw mode restored
//
// The first two return a nil error, which is `dira distill` exiting 0. Neither
// is a failure: an empty queue is the common case on most days, and captures
// waiting in a context with no human is what a hook looks like from the inside.
//
// # The queue is read before the terminal is touched, and that costs a read
//
// Run reads the queue, and then Loop reads it again. That second read is not an
// oversight and it is not free — one List and one Get per entry — and it is
// paid deliberately, because the alternative is worse in both directions. The
// decision Run has to make first is *whether to touch the terminal at all*, and
// that cannot be made without knowing whether there is a card; making it after
// entering raw mode would mean a session that flickers a terminal into raw mode
// on a day when there is nothing to review, which is the "empty queue costs a
// screen, not a line" failure this task was written against. Threading the
// already-read queue into Loop would mean changing Loop's signature, and Loop's
// contract — that it reads the files itself, at the moment it disposes of them
// (dec-0002, L-0009) — is the reason a disposition is written against the entry
// as its file actually reads. Loop stays the authority on what it writes to;
// Run's read is a preflight and is treated as one. A capture staged in between
// the two is simply picked up by the loop, and one deleted in between is simply
// not shown.
//
// # Restoring the terminal
//
// The Restore is deferred the moment it is obtained, so every way out of the
// keystroke loop — the queue emptying, `q`, the driver reaching EOF, a read
// error, a cancelled context, a panic in a renderer — leaves the terminal the
// way it was found. There is no exit path between Raw and that defer.
func Run(ctx context.Context, opts SessionOptions) (result *SessionResult, err error) {
	if opts.Store == nil {
		return nil, errors.New("distill: no ledger")
	}
	if opts.Terminal == nil {
		// Not defaulted to "assume no human". A surface that forgot to
		// wire this would then never review anything and would say
		// nothing about why, which is a worse failure than refusing.
		return nil, errors.New("distill: no terminal; whether a human is present is the caller's to answer")
	}
	if opts.Now == nil {
		return nil, errors.New("distill: no clock; updated is stamped by the caller")
	}

	queue, err := Staged(ctx, opts.Store)
	if err != nil {
		return nil, err
	}

	result = &SessionResult{
		Waiting:     len(queue.Awaiting()),
		Pending:     len(queue.PendingExtraction()),
		Interactive: opts.Terminal.Interactive(),
		Warnings:    queue.Warnings,
	}
	show := func(text string) {
		if opts.Display != nil {
			opts.Display.Show(text)
		}
	}

	// The warnings first, and before the summary rather than after it, so
	// the last line a human sees is the answer and not a footnote. They are
	// additional to the one line the acceptance clause counts: a ledger that
	// reads cleanly produces none, and a ledger holding a file nobody can
	// parse must say so or the entry is invisible in every answer.
	for _, warning := range queue.Warnings {
		show(warning)
	}

	switch {
	case result.Waiting == 0:
		result.Summary = nothingWaiting(result.Pending)
		show(result.Summary)
		return result, nil

	case !result.Interactive:
		result.Summary = notATerminal(result.Waiting)
		show(result.Summary)
		return result, nil
	}

	keys, restore, err := opts.Terminal.Raw()
	if err != nil {
		return result, fmt.Errorf("putting the terminal into raw mode: %w", err)
	}
	if keys == nil || restore == nil {
		// A Terminal that reported success and handed back nothing would
		// otherwise become a nil dereference inside the loop, with the
		// terminal already in raw mode and no way out of it.
		if restore != nil {
			_ = restore()
		}
		return result, errors.New("distill: the terminal entered raw mode without returning a way out of it")
	}
	result.EnteredRaw = true
	defer func() {
		if restoreErr := restore(); restoreErr != nil && err == nil {
			// Reported rather than dropped: a terminal left in raw
			// mode is a shell the human has to reset by hand, and it
			// is the caller's business that it happened. It does not
			// displace an error the session already has, which is
			// the one that explains why the session ended.
			err = fmt.Errorf("restoring the terminal: %w", restoreErr)
		}
	}()

	loop, err := Loop(ctx, Options{
		Store:   opts.Store,
		Keys:    interruptible{ctx: ctx, keys: keys},
		Display: opts.Display,
		Now:     opts.Now,
		Edit:    opts.Edit,
		Render:  opts.Render,
	})
	result.Loop = loop
	if err != nil {
		return result, err
	}
	return result, nil
}

// interruptible is the keystroke source Run hands the loop: the terminal's, plus
// the context.
//
// The context is checked before each read rather than raced against one. A read
// already blocked in the kernel is not something this wrapper can take back, and
// pretending otherwise — a select over a goroutine's channel — would leave that
// goroutine holding a raw-mode terminal after Run had restored it, which is a
// worse end state than a keystroke arriving late. What a real interrupt does to
// a real terminal read is end it with an error, and that error travels this path
// on its own; the check here is what makes a *cancelled context* end the session
// at the next keystroke instead of being noticed only when the human presses q.
type interruptible struct {
	ctx  context.Context
	keys KeySource
}

func (i interruptible) ReadKey() (byte, error) {
	if err := i.ctx.Err(); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInterrupted, err)
	}
	return i.keys.ReadKey()
}

// nothingWaiting is the empty-queue line: one line, and the whole of the output.
//
// It names the entries waiting on extraction when there are any, in the same
// line rather than a second one. dec-0022 requires that state to be visible or
// promoted entries accumulate in `staged` looking rejected; a session that
// printed "nothing staged" over the top of six of them would be the failure that
// decision named, and one that spent a second line on the common case would be
// the failure this task was written against.
func nothingWaiting(pending int) string {
	if pending == 0 {
		return "dira distill: nothing staged"
	}
	return fmt.Sprintf("dira distill: nothing awaiting you; %s confirmed and waiting on extraction",
		count(pending, "entry", "entries"))
}

// notATerminal is the non-interactive line.
//
// It says what is waiting, why nothing happened, and that nothing was touched,
// because all three are things the reader of a CI log or a hook transcript needs
// and none of them can be inferred from the exit code — which is 0, and has to
// be, so that a hook that called this does not fail.
func notATerminal(waiting int) string {
	return fmt.Sprintf("dira distill: %s awaiting a human; stdin is not a terminal, so nothing was read and nothing was changed",
		count(waiting, "capture", "captures"))
}

// count is "1 capture" or "3 captures". The plural is spelled out by the caller
// because English does not derive `entries` from `entry` by adding an s, and a
// line that says "1 entries" in a demo screenshot is a line somebody has to fix
// later.
func count(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}
