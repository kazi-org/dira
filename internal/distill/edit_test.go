package distill

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/schema"
)

// TestEditBody is E2-L4-T6.
//
// Two things about this file are deliberate and are the difference between it
// proving something and looking as though it does.
//
// The editor is a real process. `e` shells out, and a test whose editor is a
// closure returning a string proves nothing about an exit status and cannot tell
// "the editor ran and changed nothing" from "the editor never ran" — which is the
// shape in which every assertion below passes trivially. So the fake editor is
// the test binary re-executed (see fakeEditor and TestFakeEditorIsTheEditor), it
// writes a marker file from inside the child process, and every case that expects
// an edit asserts that marker was written and holds the exact text the editor was
// handed.
//
// And "nothing else changed" is asserted over the file's bytes, not its fields.
// dec-0002 buys a legible diff with one file per entry, so a re-wrapped or
// re-ordered key is a change even though it decodes to the identical entry — and
// the case below shows the comparison catching exactly that, alongside a decoded
// comparison that calls the same two files the same. An invariant nobody watched
// fail is an invariant nobody has checked (docs/lore.md L-0001).
func TestEditBody(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("the frontmatter is byte-identical apart from updated", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		path := writeEntryFile(t, entriesDir, "dec-0001", handWrapped)
		before := readFile(t, path)

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		editor := newFakeEditor(t).writes("\nThe because, rewritten by a human who typed it themselves.\n")
		edited, err := EditBody(ctx, store, read, editor, disposeNow)
		if err != nil {
			t.Fatalf("EditBody: %v", err)
		}
		after := readFile(t, path)

		// The editor ran, in another process, and was handed the body and
		// only the body. Without this the rest of the case is green
		// against a function that never opened anything.
		if handed := editor.ran(t); handed != bodyOf(t, before) {
			t.Errorf("the editor was handed %q, want the entry's body %q", handed, bodyOf(t, before))
		}
		if strings.Contains(editor.ran(t), "id: dec-0001") {
			t.Error("the editor was handed the frontmatter; `e` opens the body alone")
		}
		if editor.calls != 1 {
			t.Errorf("the editor was called %d times, want exactly 1", editor.calls)
		}

		if !edited.Wrote() || edited.ID != "dec-0001" || edited.Note != "" {
			t.Errorf("EditBody reported %+v, want a write of dec-0001 with no note", edited)
		}
		if before == after {
			t.Fatal("the file did not change at all, so every comparison below is comparing a file with itself")
		}
		validateEntryFile(t, after)

		// …and the published contract has to be able to reject, or every
		// "it still validates" above is a call to a function that returns
		// nil whatever it is handed.
		published, err := schema.NewValidator()
		if err != nil {
			t.Fatalf("compiling entry.schema.json: %v", err)
		}
		if err := published.Validate([]byte(strings.Replace(after, "state: staged", "state: shipped", 1))); err == nil {
			t.Error("entry.schema.json accepts `state: shipped`, so it is not grading these files")
		}

		// The acceptance line itself: the frontmatter block, byte for
		// byte, with `updated` dropped.
		if strippedFront(t, before) != strippedFront(t, after) {
			t.Errorf("the edit changed the frontmatter.\n--- before ---\n%s\n--- after ---\n%s",
				strippedFront(t, before), strippedFront(t, after))
		}
		if want := `updated: "` + disposeNow.Format(time.RFC3339) + `"`; !strings.Contains(after, want) {
			t.Errorf("the edited file does not carry %s:\n%s", want, after)
		}
		if got := bodyOf(t, after); got != "\nThe because, rewritten by a human who typed it themselves.\n" {
			t.Errorf("the body is %q, want what the editor wrote", got)
		}

		// The author's hand-wrapping survived. This is the whole reason
		// the fixture is hand-written at a width the codec would never
		// choose: an edit that round-tripped the entry through anything
		// that re-folds prose would reflow these lines and produce the
		// forty-line diff dec-0002 exists to prevent.
		for _, line := range handWrappedLines {
			if !strings.Contains(after, line+"\n") {
				t.Errorf("the edit re-wrapped the frontmatter: %q is no longer a line of the file:\n%s", line, after)
			}
		}

		// --- the comparison has to be able to fail ------------------- //

		// A changed value.
		if strippedFront(t, before) == strippedFront(t, strings.Replace(after, "kind: decision", "kind: note", 1)) {
			t.Error("the frontmatter comparison cannot tell two different files apart")
		}
		// A re-ordered key.
		if strippedFront(t, after) == strippedFront(t, swapLines(after, "kind: decision", "title: ")) {
			t.Error("the frontmatter comparison cannot see a re-ordered key")
		}
		// A re-wrapped key: the same value, the same decoded entry,
		// different bytes. This is the case the acceptance line calls out
		// by name, and it is the one a comparison of decoded fields is
		// blind to — which the assertion below proves rather than asserts.
		rewrapped := strings.Replace(after, foldedPair, foldedJoined, 1)
		if rewrapped == after {
			t.Fatal("the re-wrap replaced nothing; the fixture moved and this case is no longer testing anything")
		}
		if strippedFront(t, after) == strippedFront(t, rewrapped) {
			t.Error("the frontmatter comparison cannot see a re-wrapped key, which is the change it exists to catch")
		}
		one, two := decodeBytes(t, after), decodeBytes(t, rewrapped)
		if !reflect.DeepEqual(one.Alternatives, two.Alternatives) {
			t.Errorf("the re-wrap changed the decoded value (%+v against %+v); it was meant to change only the bytes",
				one.Alternatives, two.Alternatives)
		}
	})

	t.Run("an entry with no updated gains one and nothing else", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"))
		path := filepath.Join(entriesDir, "dec-0001.md")
		before := readFile(t, path)
		if strings.Contains(before, "updated:") {
			t.Fatal("the fixture already carries updated; this case is about the field being inserted")
		}

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		editor := newFakeEditor(t).writes("A because for an entry that had none.\n")
		if _, err := EditBody(ctx, store, read, editor, disposeNow); err != nil {
			t.Fatalf("EditBody: %v", err)
		}
		after := readFile(t, path)

		if editor.ran(t) != "" {
			t.Errorf("the editor was handed %q; the entry the regex tier writes has no body", editor.ran(t))
		}
		validateEntryFile(t, after)
		if strippedFront(t, before) != strippedFront(t, after) {
			t.Errorf("inserting updated moved something else.\n--- before ---\n%s\n--- after ---\n%s",
				strippedFront(t, before), strippedFront(t, after))
		}
		if !strings.Contains(after, `updated: "`+disposeNow.Format(time.RFC3339)+`"`) {
			t.Errorf("the file gained no updated:\n%s", after)
		}
		if got := bodyOf(t, after); got != "\nA because for an entry that had none.\n" {
			t.Errorf("the body is %q, want the one blank line and the prose", got)
		}
	})

	t.Run("an editor that writes frontmatter into the body changes no frontmatter field", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"))
		path := filepath.Join(entriesDir, "dec-0001.md")
		before := readFile(t, path)

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		forged := "---\nstate: accepted\nconfirmed_by: human\nsource:\n  tier: semantic\nid: dec-9999\n---\n\nand some prose after it\n"
		if _, err := EditBody(ctx, store, read, newFakeEditor(t).writes(forged), disposeNow); err != nil {
			t.Fatalf("EditBody: %v", err)
		}
		after := readFile(t, path)

		// It landed in the body. Without this the case passes against a
		// function that threw the edit away, which is a different bug.
		if !strings.Contains(bodyOf(t, after), "state: accepted") {
			t.Fatalf("the editor's text is not in the body, so nothing was spliced:\n%s", after)
		}
		validateEntryFile(t, after)
		if strippedFront(t, before) != strippedFront(t, after) {
			t.Errorf("frontmatter-looking prose changed the frontmatter.\n--- before ---\n%s\n--- after ---\n%s",
				strippedFront(t, before), strippedFront(t, after))
		}

		// And the file still reads back as the entry it was: the check
		// above is over the bytes, this one is over what a reader of this
		// ledger would actually see.
		entry := decodeBytes(t, after)
		if entry.ID != "dec-0001" || entry.State != ledger.StateStaged || entry.ConfirmedBy != "" {
			t.Errorf("the entry now reads id=%s state=%s confirmed_by=%q; a keystroke forged provenance",
				entry.ID, entry.State, entry.ConfirmedBy)
		}
		if entry.Source == nil || entry.Source.Tier != ledger.TierRegex {
			t.Errorf("source is now %+v; the capture was still made by a regular expression", entry.Source)
		}
	})

	t.Run("an editor exiting non-zero leaves the file byte-identical and the entry staged", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		path := writeEntryFile(t, entriesDir, "dec-0001", handWrapped)
		before := readFile(t, path)
		digest := ledgerDigest(t, entriesDir)

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		// The editor writes a body *and then* fails, which is the case
		// that separates "the write was refused" from "there was nothing
		// to write".
		editor := newFakeEditor(t).writes("\nprose the human abandoned\n").exits(1)
		edited, err := EditBody(ctx, store, read, editor, disposeNow)
		if err == nil {
			t.Fatalf("EditBody returned %+v for an editor that exited non-zero", edited)
		}

		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("the failure is %v, not a process exit status; the editor was not a real process", err)
		}
		if exit.ExitCode() != 1 {
			t.Errorf("the editor exited %d, want 1", exit.ExitCode())
		}
		if editor.ran(t) != bodyOf(t, before) {
			t.Error("the editor never ran, so this case would pass against a function that does nothing")
		}
		if after := readFile(t, path); after != before {
			t.Errorf("a failed edit rewrote the file.\n--- before ---\n%s\n--- after ---\n%s", before, after)
		}
		if after := ledgerDigest(t, entriesDir); after != digest {
			t.Errorf("a failed edit wrote to the ledger:\n  before %s\n  after  %s", digest, after)
		}
		if entry := decodeBytes(t, readFile(t, path)); entry.State != ledger.StateStaged {
			t.Errorf("the entry is %s; a failed edit disposed of the card", entry.State)
		}

		// The digest has to be able to move, or the two assertions above
		// are tautologies — and it is shown moving under the same call
		// with the same fixture and an editor that exits 0.
		succeeds := newFakeEditor(t).writes("\nprose the human kept\n")
		if _, err := EditBody(ctx, store, read, succeeds, disposeNow); err != nil {
			t.Fatalf("EditBody with an editor that exits 0: %v", err)
		}
		if ledgerDigest(t, entriesDir) == digest {
			t.Error("the digest is unchanged after an edit that did write; it is not measuring the ledger")
		}
	})

	t.Run("an empty body leaves the file unchanged and says so in one line", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name    string
			written string
		}{
			{"nothing at all", ""},
			{"newlines only", "\n\n\n"},
			{"whitespace only", "   \n\t\n"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				store, _, entriesDir := tempLedger(t)
				path := writeEntryFile(t, entriesDir, "dec-0001", handWrapped)
				before := readFile(t, path)
				digest := ledgerDigest(t, entriesDir)

				read, err := store.Get(ctx, "dec-0001")
				if err != nil {
					t.Fatalf("reading the staged entry: %v", err)
				}
				editor := newFakeEditor(t).writes(tc.written)
				edited, err := EditBody(ctx, store, read, editor, disposeNow)
				if err != nil {
					t.Fatalf("EditBody: %v", err)
				}
				if editor.ran(t) != bodyOf(t, before) {
					t.Error("the editor never ran; an unchanged file proves nothing here")
				}
				if edited.Wrote() {
					t.Errorf("EditBody wrote %+v for an empty body", edited.After)
				}
				if !isOneLine(edited.Note) {
					t.Errorf("the note is %q, want exactly one non-empty line", edited.Note)
				}
				if !strings.Contains(edited.Note, "dec-0001") {
					t.Errorf("the note is %q and does not name the entry it is about", edited.Note)
				}
				if after := readFile(t, path); after != before {
					t.Errorf("an empty body rewrote the file:\n%s", after)
				}
				if after := ledgerDigest(t, entriesDir); after != digest {
					t.Errorf("an empty body wrote to the ledger:\n  before %s\n  after  %s", digest, after)
				}
			})
		}

		// The one-line check has to be able to fail, or "says so in one
		// line" is satisfied by anything at all.
		if isOneLine("two\nlines") {
			t.Error("the one-line check accepts a message with a newline in it")
		}
		if isOneLine("") {
			t.Error("the one-line check accepts an empty message, so a silent refusal would pass it")
		}
	})

	t.Run("an editor that writes CRLF leaves a file that reads back as it was written", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		path := writeEntryFile(t, entriesDir, "dec-0001", handWrapped)

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		if _, err := EditBody(ctx, store, read, newFakeEditor(t).writes("first line\r\nsecond line\r\n"), disposeNow); err != nil {
			t.Fatalf("EditBody: %v", err)
		}
		after := readFile(t, path)

		// The reader normalises line endings when it splits the
		// frontmatter, so a file written with CRLF decodes to a body it no
		// longer holds: the entry on disk and the entry in memory differ
		// by a byte per line, silently and forever.
		if strings.Contains(after, "\r") {
			t.Errorf("the file carries a carriage return, so it no longer matches the entry it decodes to:\n%q", after)
		}
		if got := bodyOf(t, after); got != "\nfirst line\nsecond line\n" {
			t.Errorf("the body is %q, want the same prose with the line endings the reader will see", got)
		}
		// The property behind both: what is on disk is what the codec
		// would write for what it reads back.
		encoded, err := ledger.Encode(decodeBytes(t, after))
		if err != nil {
			t.Fatalf("re-encoding the edited entry: %v", err)
		}
		if string(encoded) != after {
			t.Errorf("the file is not stable under a read and a write.\n--- on disk ---\n%q\n--- re-encoded ---\n%q", after, string(encoded))
		}
	})

	t.Run("a body that comes back unchanged writes nothing", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		path := writeEntryFile(t, entriesDir, "dec-0001", handWrapped)
		before := readFile(t, path)

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		// The editor opened, the human looked, the human changed nothing.
		// Bumping `updated` for that would be a change to the permanent
		// record with nothing behind it.
		editor := newFakeEditor(t)
		edited, err := EditBody(ctx, store, read, editor, disposeNow)
		if err != nil {
			t.Fatalf("EditBody: %v", err)
		}
		if editor.ran(t) != bodyOf(t, before) {
			t.Error("the editor never ran")
		}
		if edited.Wrote() {
			t.Errorf("EditBody wrote %+v for a body nobody changed", edited.After)
		}
		if !isOneLine(edited.Note) {
			t.Errorf("the note is %q, want exactly one non-empty line", edited.Note)
		}
		if after := readFile(t, path); after != before {
			t.Errorf("an unchanged body rewrote the file:\n%s", after)
		}
	})

	t.Run("no editor names the variable in one line and touches nothing", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		writeEntryFile(t, entriesDir, "dec-0001", handWrapped)
		digest := ledgerDigest(t, entriesDir)

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		// A surface with no `$EDITOR` has no BodyEditor to hand in. The error
		// is what makes that surface exit non-zero; the exit code itself
		// is cmd/dira's contract and is asserted there.
		edited, err := EditBody(ctx, store, read, nil, disposeNow)
		if err == nil {
			t.Fatalf("EditBody returned %+v with no editor", edited)
		}
		if !errors.Is(err, ErrNoEditor) {
			t.Errorf("the failure is %v; a surface cannot tell an unset variable from a broken ledger", err)
		}
		if !isOneLine(err.Error()) {
			t.Errorf("the refusal is %q, want exactly one line", err.Error())
		}
		if !strings.Contains(err.Error(), EditorVar) {
			t.Errorf("the refusal is %q and does not name %s, which is the thing the human can fix", err.Error(), EditorVar)
		}
		if EditorVar != "EDITOR" {
			t.Errorf("EditorVar is %q; the acceptance line names $EDITOR", EditorVar)
		}
		if after := ledgerDigest(t, entriesDir); after != digest {
			t.Errorf("a refused edit wrote to the ledger:\n  before %s\n  after  %s", digest, after)
		}
	})

	t.Run("an entry that changed while the editor was open is not reverted", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		path := writeEntryFile(t, entriesDir, "dec-0001", handWrapped)

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		// `dira sniff` runs unattended from a Stop hook while the human
		// is in vim. Writing back the copy that was read would revert
		// whatever it did — a frontmatter change undone by the keystroke
		// that promises not to touch one.
		editor := newFakeEditor(t).writes("\nprose typed while the file moved\n")
		editor.meanwhile = func() {
			confirmMe, err := store.Get(ctx, "dec-0001")
			if err != nil {
				t.Errorf("reading dec-0001 mid-edit: %v", err)
				return
			}
			if _, err := Confirm(ctx, store, confirmMe, disposeNow); err != nil {
				t.Errorf("Confirm mid-edit: %v", err)
			}
		}
		concurrent := readFile(t, path)

		edited, err := EditBody(ctx, store, read, editor, disposeNow)
		if err == nil {
			t.Fatalf("EditBody returned %+v after the entry changed underneath it", edited)
		}
		after := readFile(t, path)
		if after == concurrent {
			t.Fatal("the concurrent write did not land, so this case is not testing what it says")
		}
		if !strings.Contains(after, "confirmed_by: "+ConfirmedByHuman) {
			t.Errorf("the concurrent confirm was reverted by the refused edit:\n%s", after)
		}
		if strings.Contains(bodyOf(t, after), "prose typed while the file moved") {
			t.Error("the edit was written over a file that had moved")
		}
	})

	t.Run("editing an entry that is not staged writes nothing", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, decisionIn("dec-0002", "run payroll in house", ledger.StateAccepted))
		digest := ledgerDigest(t, entriesDir)

		read, err := store.Get(ctx, "dec-0002")
		if err != nil {
			t.Fatalf("reading the accepted entry: %v", err)
		}
		editor := newFakeEditor(t).writes("\na body somebody tried to give an accepted decision\n")
		if _, err := EditBody(ctx, store, read, editor, disposeNow); err == nil {
			t.Fatal("EditBody accepted an entry that is not staged; a keystroke aimed at a queue reached " +
				"enforcement substrate")
		}
		if editor.calls != 0 {
			t.Error("the editor was launched before the entry was refused; a refusal that opens vim is not a refusal")
		}
		if after := ledgerDigest(t, entriesDir); after != digest {
			t.Errorf("a refused edit wrote to the ledger:\n  before %s\n  after  %s", digest, after)
		}
	})

	t.Run("the frontmatter guard permits the body and updated and refuses everything else", func(t *testing.T) {
		t.Parallel()

		// onlyTheBody is the rule applied on the way to the disk rather
		// than only in this file. It fires only when something else has
		// already gone wrong, so it is exercised here directly: a guard
		// whose refusal nobody has watched is a guard nobody has checked.
		before := decodeBytes(t, handWrapped)

		body := *before
		body.Body = "\na different because\n"
		body.Updated = disposeNow.Format(time.RFC3339)
		if err := onlyTheBody(before, &body); err != nil {
			t.Errorf("the guard refused an edit of the body and updated, which is the edit it exists to permit: %v", err)
		}

		for name, mutate := range map[string]func(*ledger.Entry){
			"state":        func(e *ledger.Entry) { e.State = ledger.StateAccepted },
			"confirmed_by": func(e *ledger.Entry) { e.ConfirmedBy = ConfirmedByHuman },
			"title":        func(e *ledger.Entry) { e.Title = "something the human retyped" },
			"created":      func(e *ledger.Entry) { e.Created = "2020-01-01T00:00:00Z" },
			"source.tier": func(e *ledger.Entry) {
				source := *e.Source
				source.Tier = ledger.TierSemantic
				e.Source = &source
			},
			"alternatives": func(e *ledger.Entry) {
				e.Alternatives = append([]ledger.Alternative{{Option: "a road", WhyNot: "not taken"}}, e.Alternatives...)
			},
		} {
			forged := body
			mutate(&forged)
			if err := onlyTheBody(before, &forged); err == nil {
				t.Errorf("the guard permitted an edit that changed %s; a keystroke could forge provenance", name)
			}
		}
	})

	t.Run("the exported API takes no reader, writer or terminal handle", func(t *testing.T) {
		t.Parallel()

		// Pinned by compilation, as Confirm and Discard are: a conversion
		// between two function types only compiles while their parameters
		// and results are identical.
		_ = editBodySignature(EditBody)

		if offending := terminalTypes(reflect.TypeOf(EditBody)); len(offending) != 0 {
			t.Errorf("EditBody takes or returns %v; E6's surface would have to reimplement the semantics", offending)
		}

		// The BodyEditor interface is the seam an editor is injected through,
		// so it is the one place a terminal could enter this package
		// without appearing in a function signature at all.
		method, ok := reflect.TypeOf((*BodyEditor)(nil)).Elem().MethodByName("Edit")
		if !ok {
			t.Fatal("BodyEditor has no Edit method; the interface this package injects an editor through moved")
		}
		if offending := terminalTypes(method.Type); len(offending) != 0 {
			t.Errorf("BodyEditor.Edit takes or returns %v; an editor is handed text and returns text", offending)
		}
	})
}

// editBodySignature is the exported signature this package commits to.
type editBodySignature = func(context.Context, ledger.Store, *ledger.Entry, BodyEditor, time.Time) (*Edit, error)

// --- the fake editor ------------------------------------------------------- //

// The environment the fake editor is scripted through. fakeEditorMarker doubles
// as the switch: the re-executed test binary is the editor when it is set, and
// the ordinary suite when it is not.
const (
	fakeEditorMarker = "DIRA_TEST_FAKE_EDITOR_MARKER"
	fakeEditorWrite  = "DIRA_TEST_FAKE_EDITOR_WRITE"
	fakeEditorExit   = "DIRA_TEST_FAKE_EDITOR_EXIT"
)

// TestFakeEditorIsTheEditor is not a test. It is the fake `$EDITOR`: this test
// binary, re-executed as a child process with fakeEditorMarker set.
//
// A child process rather than a function is what makes the cases above mean what
// they say. "The editor exited non-zero" is a real wait status; "the editor was
// invoked" is proved by a file this process writes, which a fake that was never
// called cannot have produced; and "the editor writes an empty body" is a real
// truncated file rather than a string constant. Re-executing the test binary is
// the portable way to get one — a shell script would not run on Windows, and
// compiling a helper would cost a `go build` per case.
func TestFakeEditorIsTheEditor(t *testing.T) {
	marker := os.Getenv(fakeEditorMarker)
	if marker == "" {
		t.Skip("not the fake editor: this is the ordinary suite")
	}

	args := os.Args[len(os.Args)-1:]
	path := args[0]
	handed, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake editor: reading %s: %v\n", path, err)
		os.Exit(3)
	}
	// Written before anything else, so a case that fails still knows the
	// editor ran and what it was given.
	if err := os.WriteFile(marker, handed, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "fake editor: writing the marker: %v\n", err)
		os.Exit(3)
	}

	if text, ok := os.LookupEnv(fakeEditorWrite); ok {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "fake editor: writing %s: %v\n", path, err)
			os.Exit(3)
		}
	}
	if code, _ := strconv.Atoi(os.Getenv(fakeEditorExit)); code != 0 {
		os.Exit(code)
	}
}

// fakeEditor runs the fake editor as a process, the way a surface runs the
// human's: write the body to a scratch file, hand the file's name to a command,
// read back whatever is there when it exits.
type fakeEditor struct {
	t *testing.T

	// write is what the child writes into the scratch file. A nil pointer
	// means it writes nothing at all, which is an editor the human closed
	// without touching anything.
	write *string

	// exit is the status the child exits with.
	exit int

	// marker is the file the child writes the text it was handed into. It is
	// the proof of invocation, and it is written by the other process.
	marker string

	// meanwhile runs while the editor "has the terminal", for the case where
	// something else writes to the ledger during the session.
	meanwhile func()

	// calls counts Edit calls, so a case can assert `e` opens one editor.
	calls int
}

func newFakeEditor(t *testing.T) *fakeEditor {
	t.Helper()
	return &fakeEditor{t: t, marker: filepath.Join(t.TempDir(), "invoked")}
}

func (f *fakeEditor) writes(text string) *fakeEditor { f.write = &text; return f }

func (f *fakeEditor) exits(code int) *fakeEditor { f.exit = code; return f }

func (f *fakeEditor) Edit(ctx context.Context, body string) (string, error) {
	f.calls++

	scratch := filepath.Join(f.t.TempDir(), "body.md")
	if err := os.WriteFile(scratch, []byte(body), 0o600); err != nil {
		return "", fmt.Errorf("writing the scratch file: %w", err)
	}
	if f.meanwhile != nil {
		f.meanwhile()
	}

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFakeEditorIsTheEditor$", "--", scratch)
	cmd.Env = append(os.Environ(),
		fakeEditorMarker+"="+f.marker,
		fakeEditorExit+"="+strconv.Itoa(f.exit))
	if f.write != nil {
		cmd.Env = append(cmd.Env, fakeEditorWrite+"="+*f.write)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s: %w (%s)", filepath.Base(os.Args[0]), err, oneLine(string(out)))
	}

	edited, err := os.ReadFile(scratch)
	if err != nil {
		return "", fmt.Errorf("reading the scratch file back: %w", err)
	}
	return string(edited), nil
}

// ran returns the text the editor process was handed, and fails the test if no
// editor process ever ran.
func (f *fakeEditor) ran(t *testing.T) string {
	t.Helper()

	handed, err := os.ReadFile(f.marker)
	if err != nil {
		t.Fatalf("no editor process ran: %v", err)
	}
	return string(handed)
}

// --- fixtures and helpers -------------------------------------------------- //

// handWrapped is an entry file as a human wrote it: two folded scalars wrapped at
// a width no greedy algorithm in this codebase would choose, and a body.
//
// The wrapping is the point. internal/ledger's style memo re-emits a scalar as it
// was found rather than re-folding it from the parsed value, and this fixture is
// what makes "the frontmatter is byte-identical" a claim about that rather than a
// claim about a file dira wrote itself and can trivially write again.
const handWrapped = `---
id: dec-0001
kind: decision
title: we are moving the queue reader behind the storage interface
state: staged
created: "2026-07-31T09:00:00Z"
alternatives:
  - option: keep taking a directory path and glob it here
    why_not: >
      E7's github backend has no directory to
      glob, so the path would have to be
      undone above the interface later.
source:
  hook: Stop
  session: 1f0c6a3e-0000-4000-8000-000000000009
  excerpt: >
    we are moving the queue reader behind the
    storage interface rather than taking a path
  tier: regex
---

The because as the human wrote it, hand-wrapped at
a width nobody's editor chose for them.
`

// handWrappedLines are lines of handWrapped that only survive an edit if the
// author's wrapping was preserved. Any re-folding — at the codec's 82 columns or
// at any other width — joins or splits them.
var handWrappedLines = []string{
	"      E7's github backend has no directory to",
	"      glob, so the path would have to be",
	"    we are moving the queue reader behind the",
	"    storage interface rather than taking a path",
}

// foldedPair and foldedJoined are the same folded value written two ways: a
// folded block joins its lines with single spaces, so these decode identically
// and differ only in bytes. That is what makes them the demonstration that the
// comparison is over the file and not over the fields.
const (
	foldedPair   = "      E7's github backend has no directory to\n      glob, so the path would have to be\n"
	foldedJoined = "      E7's github backend has no directory to glob, so the path would have to be\n"
)

// writeEntryFile puts a file into the ledger directly, without going through the
// codec, so the test's "before" is the author's bytes rather than dira's.
func writeEntryFile(t *testing.T, entriesDir, id, content string) string {
	t.Helper()

	path := filepath.Join(entriesDir, id+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func decodeBytes(t *testing.T, file string) *ledger.Entry {
	t.Helper()

	entry, err := ledger.Decode([]byte(file))
	if err != nil {
		t.Fatalf("decoding the entry file: %v\n%s", err, file)
	}
	return entry
}

// strippedFront is an entry file's frontmatter block with the `updated` line
// dropped: everything the acceptance line says an edit may not change.
//
// Text, not decoded fields. A re-ordered or re-wrapped key decodes to the
// identical entry and is still a change to the file (dec-0002).
func strippedFront(t *testing.T, file string) string {
	t.Helper()

	front, _, err := schema.SplitFrontmatter([]byte(file))
	if err != nil {
		t.Fatalf("splitting the frontmatter: %v\n%s", err, file)
	}
	return withoutFields(string(front), "updated")
}

// bodyOf is everything after the closing delimiter, byte for byte.
func bodyOf(t *testing.T, file string) string {
	t.Helper()

	_, body, err := schema.SplitFrontmatter([]byte(file))
	if err != nil {
		t.Fatalf("splitting the frontmatter: %v\n%s", err, file)
	}
	return string(body)
}

// swapLines moves the line beginning with second above the line beginning with
// first, changing the key order and nothing else.
func swapLines(file, first, second string) string {
	lines := strings.Split(file, "\n")
	a, b := -1, -1
	for i, line := range lines {
		if a < 0 && strings.HasPrefix(line, first) {
			a = i
		}
		if b < 0 && strings.HasPrefix(line, second) {
			b = i
		}
	}
	if a < 0 || b < 0 {
		return file
	}
	lines[a], lines[b] = lines[b], lines[a]
	return strings.Join(lines, "\n")
}

// isOneLine is the "says so in one line" check: something, and one line of it.
func isOneLine(s string) bool { return s != "" && !strings.Contains(s, "\n") }
