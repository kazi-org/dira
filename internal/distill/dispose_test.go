package distill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/schema"
)

// disposeNow is the clock every case stamps with. It is a constant for the
// reason Confirm takes a clock at all: `updated` goes into the permanent record,
// and a test that cannot say what time it is can only assert that the field
// looks vaguely like a timestamp.
var disposeNow = time.Date(2026, 7, 31, 14, 22, 0, 0, time.UTC)

// TestDispose is E2-L4-T4.
//
// Every case is two-sided. Where the assertion is "nothing was written", the
// same digest is shown moving under a write that did happen; where it is "this
// field is absent", the same scan is shown finding a seeded one. A ledger-wide
// invariant nobody watched fail is an invariant nobody has actually checked
// (docs/lore.md L-0001).
func TestDispose(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("confirm writes confirmed_by and updated and nothing else", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		staged := sniffShaped("dec-0001")
		put(t, store, staged)
		path := filepath.Join(entriesDir, "dec-0001.md")
		before := readFile(t, path)

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		d, err := Confirm(ctx, store, read, disposeNow)
		if err != nil {
			t.Fatalf("Confirm: %v", err)
		}
		after := readFile(t, path)

		// The published contract, not Entry.Validate alone. The Go
		// validator is the runtime gate; entry.schema.json is what a
		// consumer of this ledger reads.
		validateEntryFile(t, after)

		if d.Act != ActConfirm || d.ID != "dec-0001" {
			t.Errorf("Confirm reported %+v, want a confirm of dec-0001", d)
		}
		for _, want := range []string{
			"confirmed_by: " + ConfirmedByHuman,
			`updated: "` + disposeNow.Format(time.RFC3339) + `"`,
			"state: " + string(ledger.StateStaged),
		} {
			if !strings.Contains(after, want) {
				t.Fatalf("the confirmed file does not contain %q:\n%s", want, after)
			}
		}

		// dec-0025: the state stays staged, and staged is a legal state
		// for a decision. It is asked of Kind.States() rather than
		// spelled out, because that list is held in agreement with the
		// schema by internal/ledger's own tests.
		if !slicesContain(ledger.KindDecision.States(), d.After.State) {
			t.Errorf("the confirmed entry is %q, which is not one of %v", d.After.State, ledger.KindDecision.States())
		}
		if d.After.State != ledger.StateStaged {
			t.Errorf("the confirmed entry is %q; dec-0025 leaves it staged until extraction supplies the "+
				"alternative dec-0021 requires", d.After.State)
		}
		if before == after {
			t.Fatal("the file did not change at all, so the comparison below is comparing a file with itself")
		}
		if got := withoutFields(before, "updated", "confirmed_by"); got != withoutFields(after, "updated", "confirmed_by") {
			t.Errorf("confirm changed a field other than updated and confirmed_by.\n--- before ---\n%s\n--- after ---\n%s",
				got, withoutFields(after, "updated", "confirmed_by"))
		}
		// The comparison has to be able to see a change, or the line
		// above passes against a transition that rewrote the file
		// wholesale.
		if withoutFields(before, "updated", "confirmed_by") == withoutFields(strings.Replace(after, "kind: decision", "kind: note", 1), "updated", "confirmed_by") {
			t.Error("the field-stripping comparison cannot tell two different files apart")
		}

		// dec-0025's fifth alternative, refused: the write may not forge
		// provenance. source.tier records that a regular expression made
		// the capture, and it is not a progress field.
		if d.After.Source == nil || d.After.Source.Tier != ledger.TierRegex {
			t.Errorf("confirm rewrote source.tier to %v; the capture was still made by a regex", d.After.Source)
		}
		if d.Before.ConfirmedBy != "" || d.Before.Updated != "" {
			t.Errorf("Confirm modified the entry it was given: %+v", d.Before)
		}
	})

	t.Run("confirm moves updated forward on an entry that already had one", func(t *testing.T) {
		t.Parallel()

		store, _, _ := tempLedger(t)
		staged := sniffShaped("dec-0001")
		staged.Updated = "2026-07-01T09:00:00Z"
		put(t, store, staged)

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		d, err := Confirm(ctx, store, read, disposeNow)
		if err != nil {
			t.Fatalf("Confirm: %v", err)
		}
		if d.After.Updated == staged.Updated {
			t.Errorf("updated is still %q; a confirm that leaves it is a confirm nothing records", d.After.Updated)
		}
		if want := disposeNow.Format(time.RFC3339); d.After.Updated != want {
			t.Errorf("updated is %q, want %q", d.After.Updated, want)
		}
	})

	t.Run("confirming an entry that is not staged writes nothing", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		accepted := decisionIn("dec-0002", "run payroll in house", ledger.StateAccepted)
		accepted.ConfirmedBy = ""
		put(t, store, accepted)
		staged := sniffShaped("dec-0001")
		put(t, store, staged)
		before := ledgerDigest(t, entriesDir)

		read, err := store.Get(ctx, "dec-0002")
		if err != nil {
			t.Fatalf("reading the accepted entry: %v", err)
		}
		if _, err := Confirm(ctx, store, read, disposeNow); err == nil {
			t.Fatal("Confirm accepted an entry that is not staged; a keystroke aimed at a queue reached " +
				"enforcement substrate")
		}
		if after := ledgerDigest(t, entriesDir); after != before {
			t.Errorf("a refused confirm wrote to the ledger:\n  before %s\n  after  %s", before, after)
		}

		// The digest has to be able to move, or the assertion above is a
		// tautology — and it is shown moving under the write this same
		// transition makes when it is allowed to.
		stagedRead, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		if _, err := Confirm(ctx, store, stagedRead, disposeNow); err != nil {
			t.Fatalf("Confirm on a staged entry: %v", err)
		}
		if ledgerDigest(t, entriesDir) == before {
			t.Error("the digest is unchanged after a confirm that did write; it is not measuring the ledger")
		}
	})

	t.Run("confirming twice writes nothing the second time", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"))

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		if _, err := Confirm(ctx, store, read, disposeNow); err != nil {
			t.Fatalf("Confirm: %v", err)
		}
		confirmed, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("re-reading the confirmed entry: %v", err)
		}
		after := ledgerDigest(t, entriesDir)

		later := disposeNow.Add(time.Hour)
		if _, err := Confirm(ctx, store, confirmed, later); err == nil {
			t.Error("Confirm bumped updated for an act already on the record")
		}
		if ledgerDigest(t, entriesDir) != after {
			t.Error("a refused second confirm rewrote the file")
		}
	})

	t.Run("discard deletes the entry and leaves nothing behind", func(t *testing.T) {
		t.Parallel()

		store, dira, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"), sniffShaped("dec-0002"))

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		d, err := Discard(ctx, store, read)
		if err != nil {
			t.Fatalf("Discard: %v", err)
		}

		if d.Act != ActDiscard || d.ID != "dec-0001" {
			t.Errorf("Discard reported %+v, want a discard of dec-0001", d)
		}
		if d.After != nil {
			t.Errorf("Discard returned an entry that still exists: %+v", d.After)
		}
		if d.Before == nil || d.Before.ID != "dec-0001" {
			t.Error("Discard did not hand back what it deleted; a single-level undo has no other source " +
				"for a file that is now gone")
		}
		if _, err := os.Stat(filepath.Join(entriesDir, "dec-0001.md")); !os.IsNotExist(err) {
			t.Errorf("the discarded entry's file is still there: %v", err)
		}

		// dec-0024 refused a note tombstone and a marked state in turn,
		// so "deleted" has to mean the ledger gained nothing at all —
		// not one file, not one field on another entry.
		names := entryFiles(t, entriesDir)
		if len(names) != 1 || names[0] != "dec-0002.md" {
			t.Errorf("the ledger holds %v after one discard, want only dec-0002.md; a discard left exhaust behind", names)
		}
		if strings.Contains(readFile(t, filepath.Join(entriesDir, "dec-0002.md")), "dec-0001") {
			t.Error("another entry now references the discarded capture")
		}
		if _, err := os.Stat(filepath.Join(dira, "entries", "dec-0001.md")); !os.IsNotExist(err) {
			t.Error("the file reappeared under the ledger directory")
		}
	})

	t.Run("no accepted or rejected regex-tier decision survives a discard unconfirmed", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"), sniffShaped("dec-0002"),
			decisionIn("dec-0003", "run payroll in house", ledger.StateAccepted))

		read, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("reading the staged entry: %v", err)
		}
		if _, err := Discard(ctx, store, read); err != nil {
			t.Fatalf("Discard: %v", err)
		}
		if violations := unconfirmedRegexRulings(t, entriesDir); len(violations) != 0 {
			t.Errorf("after a discard the ledger holds %v: an accepted or rejected decision that a regular "+
				"expression wrote and no human stood behind (dec-0003, dec-0024)", violations)
		}

		// The scan has to be able to find one, or the line above is
		// green against a scanner that always returns nothing. This is
		// a legal entry file — it carries the alternative dec-0021
		// requires — that is nevertheless the thing the invariant
		// forbids.
		seeded := decisionIn("dec-0004", "ship the rewrite this quarter", ledger.StateAccepted)
		seeded.ConfirmedBy = ""
		seeded.Source.Tier = ledger.TierRegex
		seeded.Source.Hook = ledger.HookStop
		put(t, store, seeded)
		validateEntryFile(t, readFile(t, filepath.Join(entriesDir, "dec-0004.md")))

		if violations := unconfirmedRegexRulings(t, entriesDir); len(violations) != 1 || violations[0] != "dec-0004" {
			t.Errorf("the scan reported %v over a ledger holding a seeded violation; it is not reading the files", violations)
		}
	})

	t.Run("discard refuses anything that is not an unexamined capture", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		accepted := decisionIn("dec-0002", "run payroll in house", ledger.StateAccepted)
		put(t, store, accepted)
		confirmed := sniffShaped("dec-0003")
		confirmed.ConfirmedBy = ConfirmedByHuman
		put(t, store, confirmed)
		before := ledgerDigest(t, entriesDir)

		for _, id := range []string{"dec-0002", "dec-0003"} {
			read, err := store.Get(ctx, id)
			if err != nil {
				t.Fatalf("reading %s: %v", id, err)
			}
			if _, err := Discard(ctx, store, read); err == nil {
				t.Errorf("Discard deleted %s (%s, confirmed_by %q); dec-0024 disposes of captures nobody "+
					"has stood behind", id, read.State, read.ConfirmedBy)
			}
		}
		if after := ledgerDigest(t, entriesDir); after != before {
			t.Errorf("a refused discard changed the ledger:\n  before %s\n  after  %s", before, after)
		}
	})

	t.Run("no transition produces an alternative the caller did not supply", func(t *testing.T) {
		t.Parallel()

		supplied := []ledger.Alternative{{
			Option: "keep staging the sentence every session",
			WhyNot: "the queue then never empties",
		}}

		cases := []struct {
			name  string
			given []ledger.Alternative
		}{
			{"none supplied", nil},
			{"one supplied", supplied},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				store, _, entriesDir := tempLedger(t)
				staged := sniffShaped("dec-0001")
				staged.Alternatives = tc.given
				discarded := sniffShaped("dec-0002")
				discarded.Alternatives = tc.given
				put(t, store, staged, discarded)

				confirmMe, err := store.Get(ctx, "dec-0001")
				if err != nil {
					t.Fatalf("reading dec-0001: %v", err)
				}
				d, err := Confirm(ctx, store, confirmMe, disposeNow)
				if err != nil {
					t.Fatalf("Confirm: %v", err)
				}
				if !reflect.DeepEqual(d.After.Alternatives, tc.given) {
					t.Errorf("confirm produced alternatives %+v, want exactly the %d supplied",
						d.After.Alternatives, len(tc.given))
				}

				discardMe, err := store.Get(ctx, "dec-0002")
				if err != nil {
					t.Fatalf("reading dec-0002: %v", err)
				}
				if _, err := Discard(ctx, store, discardMe); err != nil {
					t.Fatalf("Discard: %v", err)
				}

				// Read from the files rather than from the returned
				// values: what a caller cites later is what is on
				// disk, and a transition that composed an
				// alternative into the file only would pass every
				// assertion made against the struct.
				for _, name := range entryFiles(t, entriesDir) {
					entry := decodeFile(t, filepath.Join(entriesDir, name))
					if !reflect.DeepEqual(entry.Alternatives, tc.given) {
						t.Errorf("%s holds alternatives %+v, want exactly the %d supplied (dec-0019: "+
							"where nothing can be derived, record nothing)", name, entry.Alternatives, len(tc.given))
					}
				}
			})
		}
	})

	t.Run("the exported API takes no reader, writer or terminal handle", func(t *testing.T) {
		t.Parallel()

		// The signatures, pinned by compilation. A conversion between two
		// function types only compiles while their parameters and results
		// are identical, so a fourth parameter — or a *os.File in place of
		// a ledger.Store — stops this file building. The reflection below
		// is the complement: it catches a shape this package must not take
		// even where the signature was changed here to match.
		_ = confirmSignature(Confirm)
		_ = discardSignature(Discard)
		_ = stagedSignature(Staged)

		for name, fn := range map[string]any{"Confirm": Confirm, "Discard": Discard, "Staged": Staged} {
			if offending := terminalTypes(reflect.TypeOf(fn)); len(offending) != 0 {
				t.Errorf("%s takes or returns %v; a TTY in a signature here forces E6's surface to "+
					"reimplement the semantics and lets the two drift", name, offending)
			}
		}

		// The check has to be able to fail, or it is green against a
		// package that grew a terminal yesterday.
		decoy := func(io.Writer, *os.File) error { return nil }
		if offending := terminalTypes(reflect.TypeOf(decoy)); len(offending) != 2 {
			t.Errorf("the signature check found %v in a function taking an io.Writer and an *os.File; "+
				"it is not looking at the parameters", offending)
		}
	})
}

// The exported signatures this package commits to. They are aliases rather than
// defined types so the conversions above are exact rather than merely
// convertible.
type (
	confirmSignature = func(context.Context, ledger.Store, *ledger.Entry, time.Time) (*Disposition, error)
	discardSignature = func(context.Context, ledger.Store, *ledger.Entry) (*Disposition, error)
	stagedSignature  = func(context.Context, ledger.Store) (*Queue, error)
)

// --- fixtures and helpers -------------------------------------------------- //

// sniffShaped is a staged entry in the exact shape the regex tier writes: an id,
// a kind, a truncated title, `state: staged`, `created`, and a source carrying
// the hook, the session, the excerpt and the tier. No body, no alternatives, no
// confirmed_by (docs/decisions-pending/E2-L1-report.md §4, dec-0021).
//
// It is spelled out here rather than taken from queue_test.go's stagedDecision
// because this file's subject is what a transition writes, and the input it
// writes over has to be the shape the ledger really holds rather than a
// convenient near-miss.
func sniffShaped(id string) *ledger.Entry {
	return &ledger.Entry{
		ID:      id,
		Kind:    ledger.KindDecision,
		Title:   "we are moving the queue reader behind the storage interface",
		State:   ledger.StateStaged,
		Created: "2026-07-31T09:00:00Z",
		Source: &ledger.Source{
			Hook:    ledger.HookStop,
			Session: "1f0c6a3e-0000-4000-8000-000000000009",
			Excerpt: "we are moving the queue reader behind the storage interface rather than taking a path",
			Tier:    ledger.TierRegex,
		},
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func decodeFile(t *testing.T, path string) *ledger.Entry {
	t.Helper()

	entry, err := ledger.Decode([]byte(readFile(t, path)))
	if err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	return entry
}

// withoutFields drops the named top-level frontmatter lines, so two files can be
// compared byte for byte on everything else.
//
// Lines rather than decoded fields, which is the point: a re-ordered or
// re-wrapped key is a change to the file, and a comparison over decoded structs
// would call it identical.
func withoutFields(file string, keys ...string) string {
	var kept []string
	for _, line := range strings.Split(file, "\n") {
		drop := false
		for _, key := range keys {
			if strings.HasPrefix(line, key+":") {
				drop = true
			}
		}
		if !drop {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// ledgerDigest is a sha256 over every entry file: its name, its length and its
// bytes.
func ledgerDigest(t *testing.T, entriesDir string) string {
	t.Helper()

	h := sha256.New()
	for _, name := range entryFiles(t, entriesDir) {
		data := readFile(t, filepath.Join(entriesDir, name))
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00%s", name, len(data), data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func entryFiles(t *testing.T, entriesDir string) []string {
	t.Helper()

	names, err := os.ReadDir(entriesDir)
	if err != nil {
		t.Fatalf("reading %s: %v", entriesDir, err)
	}
	var out []string
	for _, name := range names {
		if !name.IsDir() {
			out = append(out, name.Name())
		}
	}
	sort.Strings(out)
	return out
}

// unconfirmedRegexRulings scans every file in the ledger for the thing a
// disposition flow must never leave behind: a decision that a regular expression
// wrote, that has been ruled on, and that no human stood behind.
//
// It reads the files rather than any derived view (dec-0002), and it names what
// it found rather than returning a boolean, so a failure says which entry.
func unconfirmedRegexRulings(t *testing.T, entriesDir string) []string {
	t.Helper()

	var found []string
	for _, name := range entryFiles(t, entriesDir) {
		entry := decodeFile(t, filepath.Join(entriesDir, name))
		if entry.Kind != ledger.KindDecision {
			continue
		}
		if entry.State != ledger.StateAccepted && entry.State != ledger.StateRejected {
			continue
		}
		if entry.Source == nil || entry.Source.Tier != ledger.TierRegex {
			continue
		}
		if entry.ConfirmedBy == ConfirmedByHuman {
			continue
		}
		found = append(found, entry.ID)
	}
	return found
}

// validateEntryFile grades a file against entry.schema.json — the published
// contract a consumer of this ledger reads — rather than against Entry.Validate,
// which is dira's own runtime gate over the same rules.
func validateEntryFile(t *testing.T, file string) {
	t.Helper()

	v, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("compiling entry.schema.json: %v", err)
	}
	if err := v.Validate([]byte(file)); err != nil {
		t.Errorf("the entry does not satisfy entry.schema.json: %v\n%s", err, file)
	}
}

// terminalTypes names the parameters and results of fn that are a reader, a
// writer or an operating-system file handle.
var (
	readerType = reflect.TypeOf((*io.Reader)(nil)).Elem()
	writerType = reflect.TypeOf((*io.Writer)(nil)).Elem()
	fileType   = reflect.TypeOf((*os.File)(nil))
)

func terminalTypes(fn reflect.Type) []string {
	var found []string
	types := make([]reflect.Type, 0, fn.NumIn()+fn.NumOut())
	for i := range fn.NumIn() {
		types = append(types, fn.In(i))
	}
	for i := range fn.NumOut() {
		types = append(types, fn.Out(i))
	}
	for _, typ := range types {
		switch {
		case typ == fileType, typ.Implements(readerType), typ.Implements(writerType):
			found = append(found, typ.String())
		}
	}
	return found
}

func slicesContain(states []ledger.State, want ledger.State) bool {
	for _, s := range states {
		if s == want {
			return true
		}
	}
	return false
}
