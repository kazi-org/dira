package distill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
)

// loopNow is the clock every case here stamps with, for the reason disposeNow is
// one: `updated` goes into the permanent record and a test that cannot say what
// time it is can only assert that the field looks vaguely like a timestamp.
var loopNow = time.Date(2026, 8, 1, 9, 15, 0, 0, time.UTC)

// TestKeystrokes is E2-L4-T5.
//
// # The clause in the acceptance line that cannot be met, and what is asserted
// # instead
//
// E2-L4-T5's `acc:` asks that after the scripted `y`, `n`, `e` "the ledger then
// holds zero `staged` entries". That is unreachable, and not because of anything
// this loop does. dec-0022 ruled that `y` *promotes* a regex-staged decision to
// the semantic tier rather than accepting it, and dec-0025 ruled what that
// writes: `confirmed_by: human` and a bumped `updated`, with `state: staged`
// left exactly where it was, because dec-0021 requires an alternative with a
// why_not the moment a decision leaves `staged` and a keystroke supplies none.
// `e` supplies none either — a body is prose, not an alternative. So two of the
// three cards are still `staged` when the loop is done, whatever the loop does,
// and a predicate demanding zero of them could only be made green by writing a
// file the published schema rejects or by inventing a why_not that dec-0003
// forbids and dec-0019 names as invention.
//
// dec-0025 §"A clause this falsifies" records the corrected predicate, and it is
// the one asserted below: **no entry in the ledger is `staged` without
// `confirmed_by`**. It is what the written clause was reaching for — that the
// queue empties, that nothing is left awaiting a human — and unlike the written
// one it is a real two-sided test: red against a loop that skips a card, green
// against a fully disposed queue. Both sides are shown in the first case.
//
// The unreachable clause is not quietly dropped. The first case asserts what the
// ledger really holds — exactly two `staged` entries, both carrying
// `confirmed_by` — so the falsification is on the record as an observation
// rather than as a comment.
//
// Every other case here is two-sided as well (docs/lore.md L-0001): where the
// assertion is "nothing was written", the same digest is shown moving under a
// write that did happen; where it is "this byte was ignored", the same script is
// run with a byte that is not.
func TestKeystrokes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("three bytes dispose three cards", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"), sniffShaped("dec-0002"), sniffShaped("dec-0003"))

		keys := scripted("yne")
		screen := &screen{}
		edits := 0

		result, err := Loop(ctx, Options{
			Store:   store,
			Keys:    keys,
			Display: screen,
			Now:     func() time.Time { return loopNow },
			Edit:    bodyEditor("we chose the storage interface so E7 can add a backend.", &edits),
		})
		if err != nil {
			t.Fatalf("Loop: %v", err)
		}

		// Three cards, three acts, in the order the bytes arrived.
		if got := acts(result); !reflect.DeepEqual(got, []string{"dec-0001 confirm", "dec-0002 discard", "dec-0003 edit"}) {
			t.Fatalf("the loop performed %v, want a confirm, a discard and an edit in that order", got)
		}
		if result.Remaining != 0 || result.Quit {
			t.Errorf("result = %+v; three cards were disposed of, so nothing remains and the session did not quit", result)
		}
		if edits != 1 {
			t.Errorf("the editor ran %d times, want exactly once — `e` is the only key that opens one", edits)
		}

		// Exactly three bytes, and the driver was never asked for a fourth.
		// A loop that wanted a newline after each key would have asked for
		// six and been told there was nothing left.
		if keys.calls != 3 || keys.requested != 3 {
			t.Errorf("the driver was asked %d times for %d bytes, want 3 and 3", keys.calls, keys.requested)
		}
		if keys.exhausted != 0 {
			t.Errorf("the loop asked for %d bytes beyond the three it was given", keys.exhausted)
		}
		// The negative control for that counter: the driver really does
		// report being asked for more than it holds, so the zero above is
		// an observation and not a field nobody sets.
		if _, err := keys.ReadKey(); !errors.Is(err, io.EOF) {
			t.Fatalf("the exhausted driver returned %v, want io.EOF", err)
		}
		if keys.exhausted != 1 {
			t.Fatal("the driver does not record being asked for a byte it does not have, so the assertion above proves nothing")
		}

		// dec-0025's corrected predicate, green: nothing is left awaiting a
		// human.
		if got := stagedWithoutConfirmation(t, entriesDir); len(got) != 0 {
			t.Errorf("the ledger still holds %v staged without confirmed_by; a card was skipped", got)
		}

		// The written clause, observed rather than asserted: two entries are
		// still `staged`, and both carry `confirmed_by`. This is the
		// unreachable clause, reported.
		staged := stagedEntries(t, entriesDir)
		if !reflect.DeepEqual(staged, []string{"dec-0001", "dec-0003"}) {
			t.Fatalf("staged entries after the loop = %v, want [dec-0001 dec-0003]: dec-0022 keeps a confirmed "+
				"capture staged until extraction supplies the alternative dec-0021 requires", staged)
		}
		for _, id := range staged {
			entry := decodeFile(t, filepath.Join(entriesDir, id+".md"))
			if entry.ConfirmedBy != ConfirmedByHuman {
				t.Errorf("%s is staged with confirmed_by %q, want %q", id, entry.ConfirmedBy, ConfirmedByHuman)
			}
			validateEntryFile(t, readFile(t, filepath.Join(entriesDir, id+".md")))
		}
		// `n` deleted its card, and `e` wrote the body it was given.
		if _, err := os.Stat(filepath.Join(entriesDir, "dec-0002.md")); !os.IsNotExist(err) {
			t.Errorf("the discarded entry is still on disk: %v", err)
		}
		if body := decodeFile(t, filepath.Join(entriesDir, "dec-0003.md")).Body; !strings.Contains(body, "so E7 can add a backend") {
			t.Errorf("the edited entry's body is %q; `e` did not keep what the editor wrote", body)
		}

		// The corrected predicate's red side: one untouched staged capture
		// and it must report it. Without this the green above is green
		// against a scanner that always returns nothing.
		put(t, store, sniffShaped("dec-0004"))
		if got := stagedWithoutConfirmation(t, entriesDir); !reflect.DeepEqual(got, []string{"dec-0004"}) {
			t.Errorf("with one untouched capture in the ledger the predicate reported %v, want [dec-0004]; "+
				"it is not reading the files", got)
		}

		// Three cards were shown and nothing else, and none of them asked a
		// second question.
		if screen.count() != 3 {
			t.Errorf("the display was written to %d times for 3 cards:\n%s", screen.count(), screen.all())
		}
		assertNoSecondPrompt(t, screen.all())
	})

	t.Run("u after an n restores the file byte-identically", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"), sniffShaped("dec-0002"), sniffShaped("dec-0003"))
		path := filepath.Join(entriesDir, "dec-0001.md")
		before := fileSHA(t, path)

		// The digest has to be able to tell two files apart, or every
		// comparison below is a tautology.
		if before == fileSHA(t, filepath.Join(entriesDir, "dec-0002.md")) {
			t.Fatal("two different entry files have the same digest; the comparison proves nothing")
		}

		keys := scripted("nuuq")
		screen := &screen{}
		result, err := Loop(ctx, Options{
			Store:   store,
			Keys:    keys,
			Display: screen,
			Now:     func() time.Time { return loopNow },
		})
		if err != nil {
			t.Fatalf("Loop: %v", err)
		}

		if after := fileSHA(t, path); after != before {
			t.Fatalf("the restored file is not the file that was deleted:\n  before %s\n  after  %s\n%s",
				before, after, readFile(t, path))
		}
		restored := decodeFile(t, path)
		if restored.State != ledger.StateStaged || restored.ConfirmedBy != "" {
			t.Errorf("the restored entry is %s / confirmed_by %q, want staged and unconfirmed",
				restored.State, restored.ConfirmedBy)
		}
		// And it is a card again, not merely a file: the queue offers it.
		if got := itemIDs(read(t, store).Awaiting()); !reflect.DeepEqual(got, []string{"dec-0001", "dec-0002", "dec-0003"}) {
			t.Errorf("after the undo the queue offers %v, want all three", got)
		}

		if result.Undone != 1 || len(result.Dispositions) != 0 {
			t.Errorf("result = %+v; the one disposition was taken back, so none stands", result)
		}
		if result.Remaining != 3 || !result.Quit {
			t.Errorf("result = %+v; every card is still awaiting and the session ended on q", result)
		}

		// Single level, and the second `u` says so rather than reaching
		// further back.
		if got := strings.Count(screen.all(), "nothing to undo"); got != 1 {
			t.Errorf("the second undo produced %d 'nothing to undo' lines, want exactly 1:\n%s", got, screen.all())
		}
		// The restored card is presented again, so the human sees what came
		// back rather than being left looking at the card after it.
		if got := screen.shown[0]; !strings.HasPrefix(got, "1 of 3") {
			t.Fatalf("the first thing shown was %q, want the first card", firstLine(got))
		}
		firstCard := 0
		for _, shown := range screen.shown {
			if strings.HasPrefix(shown, "1 of 3") {
				firstCard++
			}
		}
		if firstCard != 2 {
			t.Errorf("the first card was shown %d times, want 2 — once before the discard and once after the "+
				"undo brought it back:\n%s", firstCard, screen.all())
		}
		assertNoSecondPrompt(t, screen.all())

		// The green side of the digest: a write to that same file does move
		// it, so "unchanged" above is a fact about this file and not about
		// the helper.
		confirmed, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("re-reading dec-0001: %v", err)
		}
		if _, err := Confirm(ctx, store, confirmed, loopNow); err != nil {
			t.Fatalf("Confirm: %v", err)
		}
		if fileSHA(t, path) == before {
			t.Error("the digest is unchanged after a write that did happen; it is not measuring the file")
		}
	})

	t.Run("u after a y restores the file byte-identically", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"), sniffShaped("dec-0002"))
		path := filepath.Join(entriesDir, "dec-0001.md")
		before := fileSHA(t, path)

		keys := scripted("yuq")
		result, err := Loop(ctx, Options{
			Store: store,
			Keys:  keys,
			Now:   func() time.Time { return loopNow },
		})
		if err != nil {
			t.Fatalf("Loop: %v", err)
		}
		if after := fileSHA(t, path); after != before {
			t.Fatalf("the un-confirmed file is not the file that was confirmed:\n  before %s\n  after  %s\n%s",
				before, after, readFile(t, path))
		}
		if entry := decodeFile(t, path); entry.ConfirmedBy != "" || entry.Updated != "" {
			t.Errorf("the restored entry carries confirmed_by %q and updated %q; the confirm was not taken back",
				entry.ConfirmedBy, entry.Updated)
		}
		if result.Undone != 1 || len(result.Dispositions) != 0 || result.Remaining != 2 {
			t.Errorf("result = %+v; the confirm was undone and both cards are awaiting again", result)
		}
	})

	t.Run("undo refuses an entry that has moved on since the disposition", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"))
		path := filepath.Join(entriesDir, "dec-0001.md")

		card, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		disposition, err := Confirm(ctx, store, card, loopNow)
		if err != nil {
			t.Fatalf("Confirm: %v", err)
		}

		// The green side first: with the file exactly as the disposition
		// left it, the undo restores it.
		restored, err := undo(ctx, store, disposition)
		if err != nil {
			t.Fatalf("undo on an untouched file: %v", err)
		}
		if restored.ID != "dec-0001" || restored.ConfirmedBy != "" {
			t.Errorf("undo returned %+v, want the entry as it was before the confirm", restored)
		}

		// And the red side: someone else edits the file between the
		// disposition and the undo, and the undo refuses rather than
		// silently discarding their work.
		again, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("re-reading dec-0001: %v", err)
		}
		disposition, err = Confirm(ctx, store, again, loopNow)
		if err != nil {
			t.Fatalf("Confirm: %v", err)
		}
		meanwhile := decodeFile(t, path)
		meanwhile.Body = "a hand edit in another window\n"
		if err := store.Put(ctx, meanwhile); err != nil {
			t.Fatalf("writing the concurrent edit: %v", err)
		}
		concurrent := fileSHA(t, path)

		if _, err := undo(ctx, store, disposition); err == nil {
			t.Error("undo overwrote a file that had changed since the disposition")
		}
		if fileSHA(t, path) != concurrent {
			t.Error("a refused undo wrote to the ledger anyway")
		}
	})

	t.Run("q leaves the rest staged and returns no error", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"), sniffShaped("dec-0002"), sniffShaped("dec-0003"))
		untouched := map[string]string{
			"dec-0002": fileSHA(t, filepath.Join(entriesDir, "dec-0002.md")),
			"dec-0003": fileSHA(t, filepath.Join(entriesDir, "dec-0003.md")),
		}

		keys := scripted("yq")
		result, err := Loop(ctx, Options{
			Store: store,
			Keys:  keys,
			Now:   func() time.Time { return loopNow },
		})
		if err != nil {
			t.Fatalf("Loop after a q: %v", err)
		}
		if !result.Quit || result.Remaining != 2 || len(result.Dispositions) != 1 {
			t.Fatalf("result = %+v; one card was disposed of and two were left", result)
		}
		if keys.requested != 2 {
			t.Errorf("the loop read %d bytes, want 2 — `q` is the last byte it asks for", keys.requested)
		}

		// "Left staged" is asserted as the files being untouched, not merely
		// as the state string being right: a `q` that rewrote them with the
		// same state would still have written to the permanent record.
		for id, want := range untouched {
			if got := fileSHA(t, filepath.Join(entriesDir, id+".md")); got != want {
				t.Errorf("%s was written to by a session that quit before reaching it", id)
			}
		}
		if got := stagedWithoutConfirmation(t, entriesDir); !reflect.DeepEqual(got, []string{"dec-0002", "dec-0003"}) {
			t.Errorf("after q the ledger holds %v awaiting a human, want exactly the two that were not reached", got)
		}
	})

	t.Run("an unrecognised byte is ignored without advancing the queue", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"), sniffShaped("dec-0002"))
		second := fileSHA(t, filepath.Join(entriesDir, "dec-0002.md"))

		// An up-arrow, which is what a queue driven by muscle memory
		// actually receives: three bytes, none of them a key.
		keys := scripted("\x1b[Ayq")
		screen := &screen{}
		result, err := Loop(ctx, Options{
			Store:   store,
			Keys:    keys,
			Display: screen,
			Now:     func() time.Time { return loopNow },
		})
		if err != nil {
			t.Fatalf("Loop: %v", err)
		}

		if got := acts(result); !reflect.DeepEqual(got, []string{"dec-0001 confirm"}) {
			t.Fatalf("the loop performed %v, want only the confirm the `y` asked for", got)
		}
		if got := fileSHA(t, filepath.Join(entriesDir, "dec-0002.md")); got != second {
			t.Error("the second card was written to; an ignored byte advanced the queue")
		}
		if keys.requested != 5 {
			t.Errorf("the loop read %d bytes, want all 5 — an ignored byte is still consumed", keys.requested)
		}
		// Ignored means ignored: no line, no re-render, nothing on the
		// display at all. Two cards were reached, so two cards were shown.
		if screen.count() != 2 {
			t.Errorf("the display was written to %d times, want 2 — one per card reached:\n%s", screen.count(), screen.all())
		}

		// The control: the same script with a recognised byte in place of
		// the escape does advance, so "ignored" is a fact about that byte
		// and not about a loop that never advances on anything.
		store2, _, entries2 := tempLedger(t)
		put(t, store2, sniffShaped("dec-0001"), sniffShaped("dec-0002"))
		result2, err := Loop(ctx, Options{
			Store: store2,
			Keys:  scripted("nyq"),
			Now:   func() time.Time { return loopNow },
		})
		if err != nil {
			t.Fatalf("Loop with recognised bytes: %v", err)
		}
		if got := acts(result2); !reflect.DeepEqual(got, []string{"dec-0001 discard", "dec-0002 confirm"}) {
			t.Fatalf("with recognised bytes the loop performed %v, want both cards disposed of", got)
		}
		if got := stagedWithoutConfirmation(t, entries2); len(got) != 0 {
			t.Errorf("the control ledger still holds %v awaiting a human", got)
		}
	})

	t.Run("the n path costs one byte and one ledger mutation", func(t *testing.T) {
		t.Parallel()

		store, _, _ := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"))
		counted := &countingStore{inner: store}
		keys := scripted("n")
		screen := &screen{}
		edits := 0

		if _, err := Loop(ctx, Options{
			Store:   counted,
			Keys:    keys,
			Display: screen,
			Now:     func() time.Time { return loopNow },
			Edit:    bodyEditor("nothing here should ever run", &edits),
		}); err != nil {
			t.Fatalf("Loop: %v", err)
		}

		// One list and one read to build the queue, then the delete. No
		// second read of the entry between the card and the write, no put,
		// and nothing else.
		want := []string{"list", "get dec-0001", "delete dec-0001"}
		if !reflect.DeepEqual(counted.ops, want) {
			t.Errorf("the reject path performed %v, want %v", counted.ops, want)
		}
		if keys.calls != 1 || keys.requested != 1 {
			t.Errorf("the reject path was asked %d times for %d bytes, want 1 and 1 — no prompt, no newline",
				keys.calls, keys.requested)
		}
		if edits != 0 {
			t.Error("the reject path opened an editor")
		}
		if screen.count() != 1 {
			t.Errorf("the reject path wrote to the display %d times, want the one card:\n%s", screen.count(), screen.all())
		}
		assertNoSecondPrompt(t, screen.all())

		// The op log's control: a key that does write differently produces a
		// different tail, so the assertion above is reading the store rather
		// than a list that is always three entries long.
		store2, _, _ := tempLedger(t)
		put(t, store2, sniffShaped("dec-0001"))
		counted2 := &countingStore{inner: store2}
		if _, err := Loop(ctx, Options{
			Store: counted2,
			Keys:  scripted("y"),
			Now:   func() time.Time { return loopNow },
		}); err != nil {
			t.Fatalf("Loop: %v", err)
		}
		if want := []string{"list", "get dec-0001", "put dec-0001"}; !reflect.DeepEqual(counted2.ops, want) {
			t.Errorf("the confirm path performed %v, want %v; the op log cannot tell the two paths apart",
				counted2.ops, want)
		}
	})

	t.Run("an empty queue reads no key at all", func(t *testing.T) {
		t.Parallel()

		store, _, _ := tempLedger(t)
		put(t, store, decisionIn("dec-0001", "run payroll in house", ledger.StateAccepted))

		keys := scripted("y")
		screen := &screen{}
		result, err := Loop(ctx, Options{
			Store:   store,
			Keys:    keys,
			Display: screen,
			Now:     func() time.Time { return loopNow },
		})
		if err != nil {
			t.Fatalf("Loop over an empty queue: %v", err)
		}
		if keys.calls != 0 {
			t.Errorf("the loop read %d keys over an empty queue, want none", keys.calls)
		}
		if result.Remaining != 0 || len(result.Dispositions) != 0 || screen.count() != 0 {
			t.Errorf("result = %+v with %d lines shown; an empty queue disposes of nothing", result, screen.count())
		}

		// The control: the same loop over the same store reads exactly one
		// key once there is a card, so "no key" is an answer about this
		// queue and not a loop that never reads.
		put(t, store, sniffShaped("dec-0002"))
		keys = scripted("y")
		if _, err := Loop(ctx, Options{Store: store, Keys: keys, Now: func() time.Time { return loopNow }}); err != nil {
			t.Fatalf("Loop: %v", err)
		}
		if keys.calls != 1 {
			t.Errorf("the loop read %d keys for one card, want 1", keys.calls)
		}
	})

	t.Run("a card that cannot be disposed of stays where it is", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"))
		before := fileSHA(t, filepath.Join(entriesDir, "dec-0001.md"))

		// `e` with no editor configured: one line, no write, no advance.
		screen := &screen{}
		result, err := Loop(ctx, Options{
			Store:   store,
			Keys:    scripted("eq"),
			Display: screen,
			Now:     func() time.Time { return loopNow },
		})
		if err != nil {
			t.Fatalf("Loop: %v", err)
		}
		if len(result.Dispositions) != 0 || result.Remaining != 1 {
			t.Errorf("result = %+v; a refused edit disposes of nothing", result)
		}
		if fileSHA(t, filepath.Join(entriesDir, "dec-0001.md")) != before {
			t.Error("a refused edit wrote to the ledger")
		}
		if !strings.Contains(screen.all(), "dec-0001") {
			t.Errorf("the refusal does not name the entry it refused:\n%s", screen.all())
		}
		assertNoSecondPrompt(t, screen.all())
	})

	t.Run("an unreadable entry file is reported without costing the other cards", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"), sniffShaped("dec-0002"))
		broken := "---\nid: dec-0002\nkind: decision\ntitle: \"unterminated\nstate: staged\n---\n"
		if err := os.WriteFile(filepath.Join(entriesDir, "dec-0002.md"), []byte(broken), 0o644); err != nil {
			t.Fatalf("corrupting dec-0002.md: %v", err)
		}

		screen := &screen{}
		result, err := Loop(ctx, Options{
			Store:   store,
			Keys:    scripted("y"),
			Display: screen,
			Now:     func() time.Time { return loopNow },
		})
		if err != nil {
			t.Fatalf("Loop: %v", err)
		}
		if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "dec-0002") {
			t.Fatalf("warnings = %v, want one naming dec-0002", result.Warnings)
		}
		if got := screen.shown[0]; got != result.Warnings[0] {
			t.Errorf("the first thing shown was %q, want the warning", got)
		}
		if got := acts(result); !reflect.DeepEqual(got, []string{"dec-0001 confirm"}) {
			t.Errorf("the loop performed %v; one bad file cost the human the other card", got)
		}
	})

	t.Run("the exported API takes no reader, writer or terminal handle", func(t *testing.T) {
		t.Parallel()

		// The signature, pinned by compilation as T4 pinned its own: a
		// conversion between two function types only compiles while their
		// parameters and results are identical.
		_ = loopSignature(Loop)

		surface := map[string]reflect.Type{
			"Loop":      reflect.TypeOf(Loop),
			"Options":   reflect.TypeOf(Options{}),
			"Result":    reflect.TypeOf(Result{}),
			"Editor":    reflect.TypeOf(Editor(nil)),
			"Renderer":  reflect.TypeOf(Renderer(nil)),
			"KeySource": reflect.TypeOf((*KeySource)(nil)).Elem(),
			"Display":   reflect.TypeOf((*Display)(nil)).Elem(),
		}
		for name, typ := range surface {
			if offending := terminalTypesDeep(typ); len(offending) != 0 {
				t.Errorf("%s reaches %v; a terminal handle anywhere in this surface forces E6's web screen "+
					"to fake one", name, offending)
			}
		}

		// The decoy, and it is a struct field rather than a parameter on
		// purpose. T4's check walks parameters and results only, so a
		// terminal smuggled in on an options struct would pass it — the
		// check below has to be the deeper one or the rule is only enforced
		// where it was already convenient to obey.
		type decoy struct {
			Store ledger.Store
			Out   io.Writer
			File  *os.File
		}
		if offending := terminalTypesDeep(reflect.TypeOf(func(decoy) error { return nil })); len(offending) != 2 {
			t.Errorf("the deep check found %v in a function taking a struct with an io.Writer and an *os.File; "+
				"it is not walking the fields", offending)
		}
		if offending := terminalTypes(reflect.TypeOf(func(decoy) error { return nil })); len(offending) != 0 {
			t.Errorf("T4's parameter-only check reported %v for that decoy; this test's premise — that a struct "+
				"field escapes it — is wrong and the deep walk is not the reason this passes", offending)
		}
	})
}

// --- the two predicates ----------------------------------------------------- //

// stagedWithoutConfirmation is dec-0025's corrected predicate: every entry whose
// file says `staged` and that carries no `confirmed_by`.
//
// Empty means the queue emptied. Non-empty names the captures still awaiting a
// human, which is what the acceptance line's unreachable "zero staged entries"
// was reaching for.
func stagedWithoutConfirmation(t *testing.T, entriesDir string) []string {
	t.Helper()

	var found []string
	for _, name := range entryFiles(t, entriesDir) {
		entry := decodeFile(t, filepath.Join(entriesDir, name))
		if entry.State == ledger.StateStaged && entry.ConfirmedBy == "" {
			found = append(found, entry.ID)
		}
	}
	return found
}

// stagedEntries is the acceptance line's clause as written: every entry in
// `state: staged`, whatever else it carries. It exists so the falsification is
// observed rather than asserted in a comment. See TestKeystrokes' doc comment.
func stagedEntries(t *testing.T, entriesDir string) []string {
	t.Helper()

	var found []string
	for _, name := range entryFiles(t, entriesDir) {
		if entry := decodeFile(t, filepath.Join(entriesDir, name)); entry.State == ledger.StateStaged {
			found = append(found, entry.ID)
		}
	}
	return found
}

// --- drivers ---------------------------------------------------------------- //

// keyScript is the scripted stdin: it hands over one byte per call and records
// how many bytes it was asked for, so "the reject path consumes exactly one
// byte" is a measurement rather than an inference from the loop's shape.
type keyScript struct {
	rest      []byte
	calls     int
	requested int
	exhausted int
}

func scripted(keys string) *keyScript { return &keyScript{rest: []byte(keys)} }

func (k *keyScript) ReadKey() (byte, error) {
	k.calls++
	k.requested++
	if len(k.rest) == 0 {
		k.exhausted++
		return 0, io.EOF
	}
	b := k.rest[0]
	k.rest = k.rest[1:]
	return b, nil
}

// screen collects what the loop showed, in order.
type screen struct{ shown []string }

func (s *screen) Show(text string) { s.shown = append(s.shown, text) }
func (s *screen) count() int       { return len(s.shown) }
func (s *screen) all() string      { return strings.Join(s.shown, "\n") }

// countingStore records every operation performed through it, in order. It is
// the whole of the evidence for "exactly one ledger mutation, and no second
// read".
type countingStore struct {
	inner ledger.Store
	ops   []string
}

func (s *countingStore) Get(ctx context.Context, id string) (*ledger.Entry, error) {
	s.ops = append(s.ops, "get "+id)
	return s.inner.Get(ctx, id)
}

func (s *countingStore) List(ctx context.Context) ([]ledger.EntryInfo, error) {
	s.ops = append(s.ops, "list")
	return s.inner.List(ctx)
}

func (s *countingStore) Create(ctx context.Context, e *ledger.Entry) error {
	s.ops = append(s.ops, "create "+e.ID)
	return s.inner.Create(ctx, e)
}

func (s *countingStore) Put(ctx context.Context, e *ledger.Entry) error {
	s.ops = append(s.ops, "put "+e.ID)
	return s.inner.Put(ctx, e)
}

func (s *countingStore) Delete(ctx context.Context, id string) error {
	s.ops = append(s.ops, "delete "+id)
	return s.inner.Delete(ctx, id)
}

// bodyEditor stands in for E2-L4-T6's `$EDITOR`: it writes a body and nothing
// else, which is the contract that task's `acc:` pins. The counter is here so a
// path that must not open an editor can be shown not to have opened one.
func bodyEditor(body string, calls *int) Editor {
	return func(ctx context.Context, store ledger.Store, e *ledger.Entry, now time.Time) (*ledger.Entry, error) {
		*calls++
		edited := *e
		edited.Body = body + "\n"
		edited.Updated = now.UTC().Format(time.RFC3339)
		if err := store.Put(ctx, &edited); err != nil {
			return nil, err
		}
		return &edited, nil
	}
}

// --- assertions and small helpers ------------------------------------------- //

// secondPrompt is the shape the acceptance line forbids between cards: a
// yes/no offer, or a question asking the human to say it again.
var secondPrompt = regexp.MustCompile(`(?i)\[y/n\]|confirm\?`)

func assertNoSecondPrompt(t *testing.T, output string) {
	t.Helper()

	if got := secondPrompt.FindString(output); got != "" {
		t.Errorf("the loop printed %q between cards; the byte is the disposition and there is no second "+
			"question:\n%s", got, output)
	}
	// The pattern's control: it must match the thing it is looking for, or
	// its silence above means nothing.
	for _, decoy := range []string{"delete this entry? [y/n]", "Are you sure — confirm?"} {
		if !secondPrompt.MatchString(decoy) {
			t.Fatalf("the prompt pattern does not match %q, so it cannot have found one in the output", decoy)
		}
	}
}

// acts renders a Result's dispositions as "id act", which is what the order and
// the identity of each disposition come down to.
func acts(r *Result) []string {
	out := make([]string, 0, len(r.Dispositions))
	for _, d := range r.Dispositions {
		out = append(out, d.ID+" "+string(d.Act))
	}
	return out
}

func fileSHA(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// loopSignature is the exported shape Loop commits to, as an alias so the
// conversion in the test is exact rather than merely convertible.
type loopSignature = func(context.Context, Options) (*Result, error)

// terminalTypesDeep is T4's check, walked through the types it reaches rather
// than stopped at the parameter list: struct fields, function types, element
// types and interface methods.
//
// It exists because this task adds an options struct, and a rule that a package
// takes no terminal handle is not a rule about parameters — an io.Writer on
// Options would be exactly the coupling the lane refused, and T4's check cannot
// see it (asserted, with a decoy, in the case above).
func terminalTypesDeep(typ reflect.Type) []string {
	return walkForTerminals(typ, map[reflect.Type]bool{})
}

func walkForTerminals(typ reflect.Type, seen map[reflect.Type]bool) []string {
	if typ == nil || seen[typ] {
		return nil
	}
	seen[typ] = true

	var found []string
	switch {
	case typ == fileType, typ.Implements(readerType), typ.Implements(writerType):
		return []string{typ.String()}
	}

	switch typ.Kind() {
	case reflect.Func:
		for i := range typ.NumIn() {
			found = append(found, walkForTerminals(typ.In(i), seen)...)
		}
		for i := range typ.NumOut() {
			found = append(found, walkForTerminals(typ.Out(i), seen)...)
		}
	case reflect.Struct:
		for i := range typ.NumField() {
			field := typ.Field(i)
			if !field.IsExported() {
				continue
			}
			found = append(found, walkForTerminals(field.Type, seen)...)
		}
	case reflect.Interface:
		for i := range typ.NumMethod() {
			found = append(found, walkForTerminals(typ.Method(i).Type, seen)...)
		}
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Chan:
		found = append(found, walkForTerminals(typ.Elem(), seen)...)
	case reflect.Map:
		found = append(found, walkForTerminals(typ.Key(), seen)...)
		found = append(found, walkForTerminals(typ.Elem(), seen)...)
	}
	return found
}
