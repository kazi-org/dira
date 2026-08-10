package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kazi-org/dira/internal/distill"
	"github.com/kazi-org/dira/internal/ledger"
)

// `dira distill` is the disposition flow: the entries `dira sniff` staged,
// presented one at a time, disposed of on a single keystroke.
//
// Everything about what the keys mean, what each one writes and what may be
// rendered lives in internal/distill — E6's web screen drives the same package,
// and semantics in this file would be semantics E6 has to reimplement. What is
// here is the three things only a command can own:
//
//   - the ledger, opened the way every other verb opens it;
//   - the terminal, which is the only thing in the flow that needs a file
//     descriptor: whether a human is present, raw mode, and how wide the card
//     may be;
//   - `$EDITOR`, because launching one needs a scratch file and the process's
//     own streams, and dec-0005 keeps both out of every package above the
//     storage backend.
//
// # Exit status
//
// The binary's general contract, not `check`/`supersede`'s (docs/lore.md
// L-0020): 0 for ran, 1 for broken, 2 for mistyped. Those two verbs route their
// own flag errors to 1 so that 2 can mean "the ledger refused you", and they are
// a pair a hook wraps together. `distill` makes no policy refusal — there is
// nothing for it to say no to — so 2 keeps its ordinary meaning here and a bad
// flag gets it.
//
// Exit 0 covers "there was nothing to review" and "stdin is not a terminal", for
// the reason `dira sniff` gives for the same choice: a non-zero exit means dira
// is broken, and a hook that treated an empty queue as a failure would fail the
// session that called it. internal/distill's session.go is where that is
// decided; this file only maps it onto a status code.

// runDistill is the command. It is registered in newApp's slice as:
//
//	{name: "distill", summary: distillSummary, run: runDistill, usage: writeDistillUsage},
func runDistill(a *app, args []string) error {
	f := &distillFlags{}
	fs := f.flagSet()
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeDistillUsage(a.stdout)
			return nil
		}
		return &usageError{err: err, usage: writeDistillUsage}
	}
	if rest := fs.Arg(0); rest != "" {
		return &usageError{
			err: fmt.Errorf("distill takes no arguments — the queue is whatever the ledger has staged — but got %q",
				rest),
			usage: writeDistillUsage,
		}
	}

	store, _, err := openLedger(f.dir)
	if err != nil {
		return err
	}

	term := newTerminal(a.stdin)
	width := f.width
	if width <= 0 {
		width = term.Width()
	}

	result, err := distill.Run(context.Background(), a.distillSession(store, term, width))
	if err != nil {
		return err
	}
	if result.Loop != nil {
		// Only after the loop actually ran. The two answers that skip it
		// have already printed their one line, and the acceptance line
		// counts that line.
		_, _ = fmt.Fprintln(a.stdout, distilled(result.Loop))
	}
	return nil
}

// distillSession is everything the flow needs, assembled from the command's
// pieces.
//
// It is a method rather than four lines inside runDistill so that the wiring is
// a value a test can hold. "The card is rendered by distill.Card and not by the
// package's fallback" and "--width reaches the renderer" are claims about this
// struct, and a test that could only observe them through a real terminal could
// not make them at all.
func (a *app) distillSession(store ledger.Store, term terminal, width int) distill.SessionOptions {
	return distill.SessionOptions{
		Store:    store,
		Terminal: term,
		Display:  &shownLines{to: a.stdout},
		Now:      func() time.Time { return a.now().UTC() },
		Edit:     editorFromEnvironment(term),
		Render:   distill.Card(width),
	}
}

// distilled is the closing line: what the session did, in whole numbers.
//
// It exists because raw mode has just been left and the last card is still on
// the screen above it, so without a line naming the outcome the human is looking
// at a card they have already disposed of. It says what happened and nothing
// about how it went.
func distilled(loop *distill.Result) string {
	parts := []string{fmt.Sprintf("dira distill: %s disposed of", plural(len(loop.Dispositions), "capture", "captures"))}
	if loop.Undone > 0 {
		parts = append(parts, fmt.Sprintf("%s undone", plural(loop.Undone, "disposition", "dispositions")))
	}
	if loop.Remaining > 0 {
		parts = append(parts, fmt.Sprintf("%d left staged", loop.Remaining))
	}
	return strings.Join(parts, ", ")
}

// plural spells out both forms, because English does not derive `dispositions`
// from `disposition` in a way worth a rule and a line reading "1 captures" in a
// demo screenshot is a line somebody has to fix later.
func plural(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// shownLines is the session's Display: one Show is one line, or one card.
//
// The trailing newline the renderer puts on a card is trimmed before the line is
// written so that a Show of a one-line summary produces exactly one line, which
// is the acceptance clause for the empty queue.
type shownLines struct{ to io.Writer }

func (s *shownLines) Show(text string) {
	_, _ = fmt.Fprintln(s.to, strings.TrimRight(text, "\n"))
}

// editorFromEnvironment is `e`: the loop's edit seam, wired to `$EDITOR`.
//
// The rules about what an edit may change are distill.EditBody's and are not
// restated here. This adapts its result to the seam the loop wants: an entry, or
// an error. The two "nothing was written" outcomes — an empty body, and a body
// that came back as it went in — become errors on this path deliberately. They
// are not failures of dira, and EditBody is right to return them as results; but
// at the keystroke they mean the card was not disposed of, and a loop told
// otherwise would advance past a card the human has not dealt with.
func editorFromEnvironment(term terminal) distill.Editor {
	return func(ctx context.Context, store ledger.Store, e *ledger.Entry, now time.Time) (*ledger.Entry, error) {
		edit, err := distill.EditBody(ctx, store, e, &environmentEditor{term: term}, now)
		if err != nil {
			return nil, err
		}
		if !edit.Wrote() {
			return nil, errors.New(edit.Note)
		}
		return edit.After, nil
	}
}

// environmentEditor runs the human's editor over the body, and only the body.
//
// It is handed the body text and hands back what came out, which is the whole of
// distill.BodyEditor's contract: it never sees the frontmatter, so there is
// nothing here that could write one — the guarantee is structural rather than
// checked.
type environmentEditor struct{ term terminal }

// Edit writes the body to a scratch file, opens it, and reads back what was
// saved.
//
// A scratch file rather than the entry's own file, for the same reason: an
// editor pointed at the real file could be saved with a hand-edited `state` or
// `source`, and dec-0025 calls that a forged provenance. The editor is never
// given the chance.
func (x *environmentEditor) Edit(ctx context.Context, body string) (string, error) {
	command := strings.Fields(os.Getenv(distill.EditorVar))
	if len(command) == 0 {
		// The sentinel, not a new error: the loop and the web surface both
		// branch on it, and an unset variable is a misconfiguration a human
		// fixes in one line rather than a broken ledger.
		return "", distill.ErrNoEditor
	}

	file, err := os.CreateTemp("", "dira-because-*.md")
	if err != nil {
		return "", fmt.Errorf("opening a scratch file for the because: %w", err)
	}
	path := file.Name()
	defer func() { _ = os.Remove(path) }()

	if _, err := io.WriteString(file, body); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("writing the because to %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("closing %s: %w", path, err)
	}

	// The terminal goes back to the way dira found it for as long as the
	// editor holds it. An editor started while dira still has the line
	// discipline in raw mode inherits raw mode, and vim in raw mode is a
	// window nobody can type into.
	if err := x.term.Suspend(); err != nil {
		return "", fmt.Errorf("handing the terminal to %s: %w", command[0], err)
	}
	run := exec.CommandContext(ctx, command[0], append(command[1:], path)...)
	run.Stdin, run.Stdout, run.Stderr = os.Stdin, os.Stdout, os.Stderr
	runErr := run.Run()
	if err := x.term.Resume(); err != nil {
		return "", fmt.Errorf("taking the terminal back from %s: %w", command[0], err)
	}
	if runErr != nil {
		return "", fmt.Errorf("%s: %w", strings.Join(command, " "), runErr)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading back %s: %w", path, err)
	}
	return string(written), nil
}

// A terminal is cmd/dira's concrete distill.Terminal, plus the two things the
// command needs of it that the flow does not.
//
// Width is here rather than in distill.Renderer's signature because how wide a
// card may be is a fact about a surface: E6's screen has a measure in CSS and no
// columns at all, and a width in the Renderer signature would be a parameter it
// had to invent a value for.
//
// Suspend and Resume are here because `$EDITOR` is the one thing in the flow
// that takes the terminal away from dira and gives it back. internal/distill
// cannot own them — it has no terminal in any signature — and the alternative is
// an editor that runs in the raw mode dira put the terminal into.
type terminal interface {
	distill.Terminal

	// Width is the terminal's column count, or 0 when it cannot be asked.
	Width() int

	// Suspend puts the terminal back the way dira found it, leaving raw mode
	// re-enterable. It is a no-op when raw mode was never entered.
	Suspend() error

	// Resume restores raw mode after a Suspend. It is a no-op when raw mode
	// was never entered.
	Resume() error
}

// offlineTerminal is the terminal there is no human at: a piped stdin, a hook's
// JSON payload, a CI step, or a platform with no termios.
//
// Interactive answers false and Raw is never reached, which is the whole of the
// anti-hang guarantee expressed in the smallest possible object — distill.Run
// calls Raw only after Interactive has said yes, so this type cannot read a byte
// even if something asked it to.
type offlineTerminal struct{}

func (offlineTerminal) Interactive() bool { return false }

func (offlineTerminal) Raw() (distill.KeySource, distill.Restore, error) {
	return nil, nil, errors.New("dira distill: stdin is not a terminal, so there is no raw mode to enter")
}

func (offlineTerminal) Width() int     { return 0 }
func (offlineTerminal) Suspend() error { return nil }
func (offlineTerminal) Resume() error  { return nil }

type distillFlags struct {
	width int
	dir   string
}

func (f *distillFlags) flagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("distill", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	fs.IntVar(&f.width, "width", 0, "lay each card out for this many columns instead of asking the terminal")
	fs.StringVar(&f.dir, "C", "", "run as if started in this directory")
	return fs
}

const distillSummary = "review what was staged for you, one keystroke per entry"

// writeDistillUsage renders `dira distill -h`. Assembled in memory and written
// once, for the same reason writeSniffUsage is.
func writeDistillUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira distill - review what was staged for you, one keystroke per entry\n\n")
	b.WriteString("usage:\n\n")
	b.WriteString("\tdira distill                   review everything staged\n")
	b.WriteString("\tdira distill --width 72        lay the cards out for 72 columns\n")
	b.WriteString("\tdira distill -C path/to/repo   review another repository's ledger\n\n")

	b.WriteString("One card at a time, one byte to dispose of it. There is no prompt and no\n")
	b.WriteString("newline: the key you press is the disposition, and the safety is `u`.\n\n")

	b.WriteString("keys:\n\n")
	for _, line := range [][2]string{
		{"y", "stand behind it; the entry is promoted for extraction and stays staged"},
		{"n", "not a decision; the capture is deleted and leaves nothing behind"},
		{"e", "edit the because in $EDITOR, and stand behind it"},
		{"u", "undo the last of those, byte for byte"},
		{"q", "leave the rest staged and stop"},
	} {
		fmt.Fprintf(&b, "\t%-3s  %s\n", line[0], line[1])
	}

	b.WriteString("\nflags:\n\n")
	for _, line := range [][2]string{
		{"--width N", "columns to lay a card out for; asks the terminal when unset"},
		{"-C DIR", "run as if started in this directory"},
	} {
		fmt.Fprintf(&b, "\t%-20s  %s\n", line[0], line[1])
	}

	b.WriteString("\n`y` does not accept the decision. A regular expression cannot know why a\n")
	b.WriteString("road was refused, so confirming records that a human stood behind the\n")
	b.WriteString("capture and hands it on for its reasoning; the entry stays staged until\n")
	b.WriteString("something supplies the rejected alternative that makes it enforceable\n")
	b.WriteString("(dec-0021, dec-0022, dec-0025).\n\n")

	b.WriteString("Each card shows only what its entry records. A capture from the regex\n")
	b.WriteString("tier has no because, no alternative and no ADR, and the card renders\n")
	b.WriteString("nothing in their place (dec-0019).\n\n")

	b.WriteString("With stdin not a terminal it reads nothing, changes nothing and exits 0,\n")
	b.WriteString("so a hook or a CI step that reaches for it cannot hang. An empty queue\n")
	b.WriteString("costs one line. Exit status is 2 only for a mistyped flag.\n")

	_, _ = io.WriteString(w, b.String())
}
