// Package ledgertest holds the contract every ledger.Store implementation has
// to satisfy, as a suite an implementation's own test can run against itself.
//
// It exists because of the claim dec-0005 makes: the github backend arriving in
// E7 must need no change above the interface. That claim is only checkable if
// there is a single definition of what the interface means, applied to both
// implementations — otherwise "it implements Store" degrades to "it compiles",
// and the two backends drift apart in exactly the places that are hard to see:
// what Create does when the id is taken, whether List sorts, whether Delete on a
// missing entry is an error or a no-op.
//
// A new backend adds one test that calls RunStoreContract and nothing else.
package ledgertest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

// NewStore builds an empty store for one subtest. It is called once per case, so
// cases cannot see each other's writes.
type NewStore func(t *testing.T) ledger.Store

// RunStoreContract runs every implementation-independent requirement of
// ledger.Store against the store newStore builds.
func RunStoreContract(t *testing.T, newStore NewStore) {
	t.Helper()

	cases := []struct {
		name string
		run  func(t *testing.T, s ledger.Store)
	}{
		{"an empty ledger lists nothing", emptyListsNothing},
		{"create then get", createThenGet},
		{"create refuses an id that is taken", createRefusesTakenID},
		{"put replaces", putReplaces},
		{"put creates when absent", putCreatesWhenAbsent},
		{"get reports a missing entry", getReportsMissing},
		{"delete removes", deleteRemoves},
		{"delete reports a missing entry", deleteReportsMissing},
		{"list is sorted by id", listIsSortedByID},
		{"version changes when content changes", versionChangesWithContent},
		{"a read entry carries a version", readEntryCarriesVersion},
		{"an invalid entry is not written", invalidEntryIsNotWritten},
		{"an invalid id is rejected", invalidIDIsRejected},
		{"a cancelled context is honoured", cancelledContextIsHonoured},
		{"the body survives storage", bodySurvivesStorage},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, newStore(t))
		})
	}
}

func emptyListsNothing(t *testing.T, s ledger.Store) {
	got, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List on an empty ledger: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List = %v, want empty", got)
	}
}

func createThenGet(t *testing.T, s ledger.Store) {
	ctx := context.Background()
	want := Entry("dec-0001")
	if err := s.Create(ctx, want); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, "dec-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != want.ID || got.Title != want.Title || got.State != want.State || got.Created != want.Created {
		t.Errorf("Get returned %+v, want the entry that was written", got)
	}
	if len(got.Alternatives) != len(want.Alternatives) {
		t.Fatalf("alternatives: %d, want %d", len(got.Alternatives), len(want.Alternatives))
	}
	if got.Alternatives[0] != want.Alternatives[0] {
		t.Errorf("alternatives[0] = %+v, want %+v", got.Alternatives[0], want.Alternatives[0])
	}
}

func createRefusesTakenID(t *testing.T, s ledger.Store) {
	ctx := context.Background()
	first := Entry("dec-0001")
	if err := s.Create(ctx, first); err != nil {
		t.Fatalf("Create: %v", err)
	}

	second := Entry("dec-0001")
	second.Title = "The entry that must not land"
	err := s.Create(ctx, second)
	if !errors.Is(err, ledger.ErrExists) {
		t.Fatalf("Create on a taken id: err = %v, want ErrExists", err)
	}

	// The point of Create is that the loser changes nothing.
	got, err := s.Get(ctx, "dec-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != first.Title {
		t.Errorf("the losing Create overwrote the entry: title = %q, want %q", got.Title, first.Title)
	}
}

func putReplaces(t *testing.T, s ledger.Store) {
	ctx := context.Background()
	if err := s.Create(ctx, Entry("dec-0001")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	replacement := Entry("dec-0001")
	replacement.Title = "The replacement decision"
	if err := s.Put(ctx, replacement); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := s.Get(ctx, "dec-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != replacement.Title {
		t.Errorf("title = %q, want %q", got.Title, replacement.Title)
	}
	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("Put created a second entry: %v", list)
	}
}

func putCreatesWhenAbsent(t *testing.T, s ledger.Store) {
	ctx := context.Background()
	if err := s.Put(ctx, Entry("int-0001")); err != nil {
		t.Fatalf("Put on a missing id: %v", err)
	}
	if _, err := s.Get(ctx, "int-0001"); err != nil {
		t.Fatalf("Get after Put: %v", err)
	}
}

func getReportsMissing(t *testing.T, s ledger.Store) {
	_, err := s.Get(context.Background(), "dec-9999")
	if !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("Get on a missing entry: err = %v, want ErrNotFound", err)
	}
}

func deleteRemoves(t *testing.T, s ledger.Store) {
	ctx := context.Background()
	if err := s.Create(ctx, Entry("note-0001")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Delete(ctx, "note-0001"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "note-0001"); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("Get after Delete: err = %v, want ErrNotFound", err)
	}
	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List after Delete = %v, want empty", list)
	}
}

func deleteReportsMissing(t *testing.T, s ledger.Store) {
	err := s.Delete(context.Background(), "note-9999")
	if !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("Delete on a missing entry: err = %v, want ErrNotFound", err)
	}
}

func listIsSortedByID(t *testing.T, s ledger.Store) {
	ctx := context.Background()
	// Written out of order, so a store that returns insertion order fails.
	for _, id := range []string{"qst-0002", "dec-0010", "int-0001", "dec-0002", "note-0003", "cst-0001"} {
		if err := s.Create(ctx, Entry(id)); err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
	}

	got, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"cst-0001", "dec-0002", "dec-0010", "int-0001", "note-0003", "qst-0002"}
	if len(got) != len(want) {
		t.Fatalf("List returned %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Errorf("List[%d] = %q, want %q", i, got[i].ID, want[i])
		}
	}
}

func versionChangesWithContent(t *testing.T, s ledger.Store) {
	ctx := context.Background()
	if err := s.Create(ctx, Entry("dec-0001")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	before, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(before) != 1 || before[0].Version == "" {
		t.Fatalf("List = %+v, want one entry carrying a version", before)
	}

	changed := Entry("dec-0001")
	changed.Body = "\nA body of a different length entirely, so no version scheme can miss it.\n"
	if err := s.Put(ctx, changed); err != nil {
		t.Fatalf("Put: %v", err)
	}

	after, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if after[0].Version == before[0].Version {
		t.Errorf("Version is %q both before and after a content change; a cache keyed on it would serve stale data", after[0].Version)
	}
}

func readEntryCarriesVersion(t *testing.T, s ledger.Store) {
	ctx := context.Background()
	if err := s.Create(ctx, Entry("dec-0001")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, "dec-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Version() == "" {
		t.Error("Get returned an entry with no version; E7's backend needs it to write safely and E1-L3 needs it to detect staleness")
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list[0].Version != got.Version() {
		t.Errorf("List reports version %q but Get reports %q for the same unchanged entry", list[0].Version, got.Version())
	}
}

func invalidEntryIsNotWritten(t *testing.T, s ledger.Store) {
	ctx := context.Background()
	// A decision with no alternatives: valid Go, invalid entry.
	bad := Entry("dec-0001")
	bad.Alternatives = nil

	if err := s.Create(ctx, bad); err == nil {
		t.Fatal("Create wrote an entry violating entry.schema.json")
	}
	if _, err := s.Get(ctx, "dec-0001"); !errors.Is(err, ledger.ErrNotFound) {
		t.Errorf("a rejected Create still left something behind: err = %v", err)
	}

	if err := s.Put(ctx, bad); err == nil {
		t.Fatal("Put wrote an entry violating entry.schema.json")
	}
	if _, err := s.Get(ctx, "dec-0001"); !errors.Is(err, ledger.ErrNotFound) {
		t.Errorf("a rejected Put still left something behind: err = %v", err)
	}
}

func invalidIDIsRejected(t *testing.T, s ledger.Store) {
	ctx := context.Background()
	for _, id := range []string{"", "dec", "dec-1", "../escape", "dec-0001/../../etc", "DEC-0001"} {
		if _, err := s.Get(ctx, id); err == nil {
			t.Errorf("Get(%q) succeeded; it is not an entry id", id)
		} else if errors.Is(err, ledger.ErrNotFound) {
			// Reporting "not found" for a malformed id is fine as
			// long as it never resolves to something.
			continue
		}
	}
}

func cancelledContextIsHonoured(t *testing.T, s ledger.Store) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := s.List(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("List with a cancelled context: err = %v, want context.Canceled", err)
	}
	if _, err := s.Get(ctx, "dec-0001"); !errors.Is(err, context.Canceled) {
		t.Errorf("Get with a cancelled context: err = %v, want context.Canceled", err)
	}
	if err := s.Create(ctx, Entry("dec-0001")); !errors.Is(err, context.Canceled) {
		t.Errorf("Create with a cancelled context: err = %v, want context.Canceled", err)
	}
	if err := s.Put(ctx, Entry("dec-0001")); !errors.Is(err, context.Canceled) {
		t.Errorf("Put with a cancelled context: err = %v, want context.Canceled", err)
	}
	if err := s.Delete(ctx, "dec-0001"); !errors.Is(err, context.Canceled) {
		t.Errorf("Delete with a cancelled context: err = %v, want context.Canceled", err)
	}
}

func bodySurvivesStorage(t *testing.T, s ledger.Store) {
	ctx := context.Background()
	want := Entry("note-0001")
	want.Kind = ledger.KindNote
	want.State = ledger.StateActive
	want.Alternatives = nil
	want.Body = "\nProse with a `---` in it\n---\nand a trailing blank line\n\n"

	if err := s.Create(ctx, want); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Get(ctx, "note-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Body != want.Body {
		t.Errorf("Body = %q, want %q", got.Body, want.Body)
	}
}

// Entry builds a valid entry with the given id, for a store under test to
// round-trip. The kind is taken from the id's prefix so a caller can write one
// of each without restating the rules.
func Entry(id string) *ledger.Entry {
	kind := ledger.KindNote
	if prefix, _, ok := strings.Cut(id, "-"); ok {
		if k, found := ledger.KindForPrefix(prefix); found {
			kind = k
		}
	}

	e := &ledger.Entry{
		ID:          id,
		Kind:        kind,
		Title:       "An entry written by the store contract suite",
		Created:     "2026-07-29T20:00:00Z",
		Tags:        []string{"contract"},
		Source:      &ledger.Source{Hook: ledger.HookManual, Tier: ledger.TierHuman},
		ConfirmedBy: "human",
		Body:        "\nWhy this entry exists.\n",
	}
	e.State = kind.States()[0]
	if kind == ledger.KindDecision {
		e.Alternatives = []ledger.Alternative{{
			Option: "Not doing it",
			WhyNot: "the contract suite needs a decision that carries an alternative, because the schema requires every decision to record at least the alternative of not doing it",
		}}
	}
	return e
}
