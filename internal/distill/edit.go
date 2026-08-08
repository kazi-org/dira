package distill

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/schema"
)

// `e` — edit the because, and nothing else.
//
// # What is being edited
//
// The markdown body: the prose a human is actually approving. `E` in the mockup
// is "Edit the because", and a because is the one part of an entry a keystroke
// cannot derive — dec-0019 says the flow may never invent a field, and this is
// the one place a human supplies one by hand rather than by pressing a letter.
//
// It is not a frontmatter editor. `state`, `source`, `created`, `confirmed_by`
// and the rest are the record of how the entry came to exist and who has stood
// behind it; a hand-edited `source.tier` would let `e` claim a semantic
// extraction that never ran, and a hand-edited `state` would move a decision out
// of `staged` with no alternative behind it — the two forgeries dec-0025 and
// dec-0021 spend most of their text refusing. So the frontmatter that goes in is
// the frontmatter that comes out, apart from `updated`, and that is checked on
// the way to the disk rather than trusted (see onlyTheBody).
//
// # Byte-identical, not field-identical
//
// "Unchanged" here means the bytes. dec-0002 buys one thing with one file per
// entry — a PR touching a decision shows a legible diff — and an edit that
// quietly reflowed a paragraph nobody touched would spend it: the diff would show
// forty lines and the reviewer would have to read all of them to find the one
// that matters. internal/ledger's style memo is what makes the byte-level promise
// reachable (it re-emits each scalar as it was found rather than re-folding it
// from the parsed value), and this package's job is not to defeat it by
// round-tripping the entry through anything else.
//
// # The editor is the surface's to launch, not this package's
//
// BodyEditor is an interface. Nothing here spawns a process, reads an environment
// variable or opens a file, for two reasons that agree:
//
//   - This package has no terminal in any signature (see the package comment),
//     and an editor session is the most terminal-shaped thing in the whole flow —
//     it takes the tty away from dira for minutes at a time.
//   - Running one needs a scratch file and the process's own standard streams,
//     which means importing os, and dec-0005 confines that to a storage backend
//     and to cmd/dira. TestNoFilesystemImportsAboveTheBackend enforces it: an
//     `$EDITOR` launcher written here would not compile past the suite.
//
// So `dira distill` reads EditorVar, builds the BodyEditor, and hands it in; E6's web
// surface hands in a textarea; a test hands in a fake. The rules about what an
// edit may change live here, once, where every surface gets them.
//
// # A minute passes between the read and the write
//
// Everywhere else in dira a read and its write are microseconds apart. Here a
// human is looking at a file in vim, `dira sniff` runs unattended from a Stop
// hook, and the entry can change underneath. Writing back the copy that was read
// would silently revert whatever changed — which is a frontmatter change made by
// something else, undone by a keystroke that promises to touch only the body. So
// the entry is re-read after the editor returns and the write is refused if the
// stored version moved.

// EditorVar is the environment variable naming the human's editor.
//
// The name lives in this package rather than in the surface that reads it so the
// refusal reads the same on every surface, and so there is one place to change if
// VISUAL is ever honoured as well.
const EditorVar = "EDITOR"

// ErrNoEditor is the refusal when no editor was supplied.
//
// It is a sentinel because a surface branches on it: `e` with no `$EDITOR` is a
// misconfiguration a human can fix in one line, not a broken ledger, and a
// keystroke loop should say so and keep the card rather than fail the run.
var ErrNoEditor = fmt.Errorf("distill: %s is not set, so `e` has no editor to open", EditorVar)

// A BodyEditor hands a human some text and returns what they wrote.
//
// Named BodyEditor, not Editor, because tui.go's Editor is a different thing at
// a different layer: that one is the loop's edit SEAM, taking an entry and
// returning an entry, and EditBody is what satisfies it. This one is the port
// underneath — it sees the body text and nothing else. Two lanes in one wave
// reached for the same name for the two concepts; they shared no file, which is
// what the wave boundary checks, and collided in the package namespace anyway.
//
// The text is the entry's markdown body and nothing else — not the file, not the
// frontmatter. An editor cannot be handed what it is not allowed to change.
type BodyEditor interface {
	// Edit is given the body as it stands and returns the body as the human
	// left it.
	//
	// An error means the human did not finish: an editor that exited
	// non-zero, a process that could not be started, a cancelled context.
	// Nothing is written in that case, so returning the original text and a
	// nil error is not the same thing and must not be substituted for it.
	Edit(ctx context.Context, body string) (string, error)
}

// An Edit is what one `e` did, for a surface that has to report it.
//
// The zero-write cases are results rather than errors, because they are ordinary
// things a human does — closing the editor without typing anything, or typing
// nothing new — and a queue should print one line and move on rather than treat
// them as failures. Note carries that line; After is nil whenever nothing
// reached the ledger.
type Edit struct {
	// ID is the entry that was opened.
	ID string

	// Before is the entry exactly as it was, unmodified.
	Before *ledger.Entry

	// After is the entry as it now stands, and nil when nothing was written.
	After *ledger.Entry

	// Note is the one line explaining why nothing was written. It is empty
	// when a write did happen: there is nothing to explain, and a surface
	// printing it unconditionally would print a blank line.
	Note string
}

// Wrote reports whether anything reached the ledger.
func (e *Edit) Wrote() bool { return e != nil && e.After != nil }

// EditBody opens the entry's body in editor, splices what comes back, and bumps
// `updated`.
//
// It changes the body and `updated` and nothing else — not `state`, which stays
// `staged`, and not `confirmed_by`, which `e` does not supply. dec-0025 governs
// what `y` writes and this is deliberately not that: editing a because is not
// standing behind the capture, and a human who does both presses both keys.
//
// Nothing is written when the editor fails, when it comes back empty, or when it
// comes back with the body it was given. The first is an error; the other two are
// an *Edit carrying a Note, because they are not failures.
//
// The entry must be one read from the store — the shape Staged hands out — so the
// re-read below has a version to compare against.
//
// now is a parameter for the reason it is one on Confirm: `updated` goes into the
// permanent record, and a package that read a clock itself would put a timestamp
// nobody chose into a file and leave a test unable to say what time it is.
func EditBody(ctx context.Context, store ledger.Store, e *ledger.Entry, editor BodyEditor, now time.Time) (*Edit, error) {
	if err := disposable(store, e); err != nil {
		return nil, err
	}
	if editor == nil {
		// Not "no editor was passed": the human's problem is the unset
		// variable, and the message names it because that is the thing
		// they can fix.
		return nil, ErrNoEditor
	}
	if now.IsZero() {
		return nil, errors.New("distill: no clock; updated is stamped by the caller")
	}
	// A confirmed entry is deliberately *not* refused here, though Confirm and
	// Discard both refuse one. Those two dispose of a capture and a human has
	// already disposed of this one; editing the because is not a second
	// disposition, and the reasoning is exactly what a pending-extraction entry
	// is still missing.
	if e.Version() == "" {
		return nil, fmt.Errorf("distill: %s did not come from the ledger; `e` edits the entry that was read, "+
			"and there is no version to check it against", e.ID)
	}

	written, err := editor.Edit(ctx, e.Body)
	if err != nil {
		return nil, fmt.Errorf("editing %s: %w", e.ID, err)
	}

	body := spliced(written)
	if strings.TrimSpace(body) == "" {
		return &Edit{ID: e.ID, Before: e, Note: oneLine(fmt.Sprintf(
			"dira distill: %s is unchanged — the editor came back empty, and an empty because is not a because", e.ID))}, nil
	}
	if body == e.Body {
		return &Edit{ID: e.ID, Before: e, Note: oneLine(fmt.Sprintf(
			"dira distill: %s is unchanged — the body came back as it went in", e.ID))}, nil
	}

	// The entry may have moved while the editor had the terminal. Writing the
	// copy that was read would revert whatever changed, which is a frontmatter
	// change undone by the keystroke that promises not to touch one.
	current, err := store.Get(ctx, e.ID)
	if err != nil {
		return nil, fmt.Errorf("re-reading %s after the editor: %w", e.ID, err)
	}
	if current.Version() != e.Version() {
		return nil, fmt.Errorf("distill: refusing to write %s: it changed on disk while the editor was open, "+
			"and writing this body back would revert that change", e.ID)
	}

	after := *e
	after.Body = body
	after.Updated = now.UTC().Format(time.RFC3339)

	if err := onlyTheBody(e, &after); err != nil {
		return nil, err
	}
	if err := store.Put(ctx, &after); err != nil {
		return nil, fmt.Errorf("writing %s: %w", e.ID, err)
	}
	return &Edit{ID: e.ID, Before: e, After: &after}, nil
}

// spliced is the body as it goes back into the file: one blank line after the
// closing `---`, and exactly one newline at the end.
//
// Those two are the file's shape rather than the human's prose, and every entry
// in this ledger has them. Everything between the first word and the last is
// returned byte for byte — no re-wrapping, no trimming of the lines inside, no
// collapsing of the blank lines between paragraphs — because the body is content
// and the codec never reformats it (see ledger.Entry.Body).
//
// CRLF is folded to LF for the one reason that it is otherwise a silent
// round-trip failure: the reader normalises line endings when it splits the
// frontmatter, so a body written with CRLF reads back as LF and the file on disk
// stops matching the entry it decodes to.
func spliced(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.Trim(text, "\n")
	if text == "" {
		return ""
	}
	return "\n" + text + "\n"
}

// onlyTheBody refuses a write that would change any frontmatter except `updated`.
//
// This is the acceptance line's own rule applied where the write happens rather
// than only where it is tested. The comparison is over the bytes the codec would
// write, with the `updated` line dropped, so a re-ordered or a re-wrapped key
// counts as a change — which a comparison of decoded fields would call identical,
// and which is exactly the diff dec-0002 sells.
//
// What it cannot see, said plainly rather than left to be discovered: it compares
// two *encodings*, not the file on disk. A file whose layout the codec could not
// reproduce would pass this and still change on write; that is a codec bug, it is
// pinned by internal/ledger's TestEditingOneFieldRewritesOnlyThatField, and it is
// why TestEditBody's own comparison is over real files written by a hand rather
// than over what this function returns.
func onlyTheBody(before, after *ledger.Entry) error {
	was, err := frontmatterOf(before)
	if err != nil {
		return err
	}
	now, err := frontmatterOf(after)
	if err != nil {
		return err
	}
	if was != now {
		return fmt.Errorf("distill: refusing to write %s: the edit would change the frontmatter, and `e` edits "+
			"the body — a hand-edited state or source is a forged provenance (dec-0025)", before.ID)
	}
	return nil
}

// frontmatterOf is an entry's frontmatter as the codec would write it, with the
// `updated` line removed.
func frontmatterOf(e *ledger.Entry) (string, error) {
	data, err := ledger.Encode(e)
	if err != nil {
		return "", fmt.Errorf("encoding %s: %w", e.ID, err)
	}
	front, _, err := schema.SplitFrontmatter(data)
	if err != nil {
		return "", fmt.Errorf("reading %s's frontmatter: %w", e.ID, err)
	}

	// Line by line, on the raw text. Anything that parsed the YAML back into
	// values would be blind to the re-wrapping this exists to catch.
	lines := strings.Split(string(front), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "updated:") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n"), nil
}
