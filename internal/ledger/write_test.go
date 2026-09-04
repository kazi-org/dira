package ledger_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// backends is every ledger.Store the allocator is tested against.
//
// Two rather than one, because the whole risk in id allocation is that it gets
// solved with a mechanism only the filesystem has. A test that passes on the
// local backend and on a map under a mutex is evidence that the retry loop is
// doing the work, not os.Link.
func backends() []struct {
	name string
	open func(t *testing.T) ledger.Store
} {
	return []struct {
		name string
		open func(t *testing.T) ledger.Store
	}{
		{"memory", func(*testing.T) ledger.Store { return newMemStore() }},
		{"local", func(t *testing.T) ledger.Store {
			t.Helper()
			store, err := local.Open(t.TempDir())
			if err != nil {
				t.Fatalf("opening a local ledger: %v", err)
			}
			return store
		}},
	}
}

// draft builds a valid entry with no id and no time-dependence, which is what a
// caller hands to Add.
func draft(kind ledger.Kind, title string) *ledger.Entry {
	e := &ledger.Entry{
		Kind:    kind,
		Title:   title,
		State:   kind.States()[0],
		Created: "2026-07-30T09:00:00Z",
		Source:  &ledger.Source{Hook: ledger.HookManual, Tier: ledger.TierHuman},
		Body:    "\nWhy this entry exists.\n",
	}
	if kind == ledger.KindDecision {
		e.Alternatives = []ledger.Alternative{{
			Option: "Not doing it",
			WhyNot: "a decision has to record at least the alternative of not doing it",
		}}
	}
	return e
}

// seed writes entries with the given ids straight through the store, so a test
// can describe the ledger the allocator is about to look at.
func seed(t *testing.T, s ledger.Store, ids ...string) {
	t.Helper()

	ctx := context.Background()
	for _, id := range ids {
		e := ledgerEntryFor(id)
		if err := s.Create(ctx, e); err != nil {
			t.Fatalf("seeding %s: %v", id, err)
		}
	}
}

func ledgerEntryFor(id string) *ledger.Entry {
	prefix, _, _ := strings.Cut(id, "-")
	kind, ok := ledger.KindForPrefix(prefix)
	if !ok {
		panic("test seed uses id " + id + ", which names no kind")
	}
	e := draft(kind, "A seeded entry")
	e.ID = id
	return e
}

func TestAddAllocatesTheLowestUnusedID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		existing []string
		kind     ledger.Kind
		want     string
	}{
		{
			name: "an empty ledger starts at one",
			kind: ledger.KindDecision,
			want: "dec-0001",
		},
		{
			name:     "the next number after a contiguous run",
			existing: []string{"dec-0001", "dec-0002", "dec-0003"},
			kind:     ledger.KindDecision,
			want:     "dec-0004",
		},
		{
			name: "a gap is filled rather than skipped",
			// This is what "lowest unused" means and it is the
			// clause that a max()+1 allocator quietly fails.
			existing: []string{"dec-0001", "dec-0003"},
			kind:     ledger.KindDecision,
			want:     "dec-0002",
		},
		{
			name:     "numbering is per kind",
			existing: []string{"dec-0001", "dec-0002", "note-0001"},
			kind:     ledger.KindIntent,
			want:     "int-0001",
		},
		{
			name:     "another kind's numbers do not shift this one",
			existing: []string{"int-0001", "int-0002", "qst-0001", "cst-0001"},
			kind:     ledger.KindNote,
			want:     "note-0001",
		},
		{
			name:     "a five-digit ledger keeps counting",
			existing: []string{"note-0001", "note-10000"},
			kind:     ledger.KindNote,
			want:     "note-0002",
		},
	}

	for _, backend := range backends() {
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					store := backend.open(t)
					seed(t, store, tc.existing...)

					e := draft(tc.kind, "The entry being allocated an id")
					if err := ledger.Add(context.Background(), store, e); err != nil {
						t.Fatalf("Add: %v", err)
					}
					if e.ID != tc.want {
						t.Errorf("allocated id = %q, want %q", e.ID, tc.want)
					}

					got, err := store.Get(context.Background(), tc.want)
					if err != nil {
						t.Fatalf("the allocated id does not read back: %v", err)
					}
					if got.Title != e.Title {
						t.Errorf("stored title = %q, want %q", got.Title, e.Title)
					}
				})
			}
		})
	}
}

// TestThirtyTwoConcurrentAddsProduceThirtyTwoDistinctIDs is the lane's
// acceptance clause in its in-process form: 32 writers, one ledger, zero
// collisions and zero overwrites.
//
// The subprocess form — 32 actual `dira log` invocations — is
// TestThirtyTwoConcurrentInvocations in cmd/dira. This one runs under -race,
// which that one cannot usefully do, and it runs against both backends.
//
// This is also T-BUG1.1's stress test: dec-0032 asked whether a genuine
// live create-vs-create race exists in Add beyond the id-reuse-after-deletion
// bug it fixes. This test already answers that, and has since it was written
// (commit b7c2f61, 2026-07-30) — no race found under -race across 32 real
// goroutines against the local backend, so dec-0032's revisit_if does not
// fire. A reader who lands on T-BUG1.1 or dec-0032 looking for the stress
// test they describe should stop here rather than write a second one.
func TestThirtyTwoConcurrentAddsProduceThirtyTwoDistinctIDs(t *testing.T) {
	t.Parallel()

	const writers = 32

	for _, backend := range backends() {
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()

			store := backend.open(t)
			ctx := context.Background()

			// A gate, so the writers race rather than queue: every
			// goroutine is already scheduled and blocked when the
			// channel closes.
			start := make(chan struct{})
			ids := make([]string, writers)
			errs := make([]error, writers)

			var wg sync.WaitGroup
			for i := range writers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					e := draft(ledger.KindDecision, fmt.Sprintf("Concurrent decision %d", i))
					<-start
					if err := ledger.Add(ctx, store, e); err != nil {
						errs[i] = err
						return
					}
					ids[i] = e.ID
				}()
			}
			close(start)
			wg.Wait()

			seen := map[string]int{}
			for i, err := range errs {
				if err != nil {
					t.Errorf("writer %d: %v", i, err)
					continue
				}
				if prev, dup := seen[ids[i]]; dup {
					t.Errorf("writers %d and %d were both allocated %s", prev, i, ids[i])
				}
				seen[ids[i]] = i
			}
			if len(seen) != writers {
				t.Fatalf("%d distinct ids from %d writers", len(seen), writers)
			}

			// Every id landed as its own entry, and nothing else did.
			infos, err := store.List(ctx)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(infos) != writers {
				t.Fatalf("the ledger holds %d entries after %d writes; something was overwritten", len(infos), writers)
			}

			// Contiguous from one: with an empty ledger and no id
			// wasted, the winners take exactly dec-0001..dec-0032.
			// A gap here would mean a candidate was abandoned, which
			// is how an allocator drifts away from "lowest unused".
			for n := 1; n <= writers; n++ {
				want := ledger.FormatID(ledger.KindDecision, n)
				if _, ok := seen[want]; !ok {
					t.Errorf("%s was never allocated, so the ids are not the lowest unused", want)
				}
			}

			// And every file is a whole entry, not a torn one.
			for _, info := range infos {
				if _, err := store.Get(ctx, info.ID); err != nil {
					t.Errorf("%s does not read back: %v", info.ID, err)
				}
			}
		})
	}
}

// TestAddRetriesTheNextIDWhenOneIsTakenUnderneathIt is the race in slow motion.
// The concurrent test proves the outcome; this one proves the mechanism, because
// a concurrency test that passes for the wrong reason looks exactly like one that
// passes.
func TestAddRetriesTheNextIDWhenOneIsTakenUnderneathIt(t *testing.T) {
	t.Parallel()

	store := newMemStore()
	ctx := context.Background()

	// The first two candidates are taken between the List and the Create,
	// which is precisely the window a scan-and-write allocator loses in.
	stolen := 0
	store.beforeCreate = func(id string) error {
		if stolen >= 2 {
			return nil
		}
		stolen++
		other := ledgerEntryFor(id)
		other.Title = "An entry a concurrent writer got in first with"
		// Written directly into the map: beforeCreate runs under the
		// store's lock, so this is what "someone else won" looks like
		// from inside Create.
		data, err := ledger.Encode(other)
		if err != nil {
			return err
		}
		store.files[id] = data
		return nil
	}

	e := draft(ledger.KindDecision, "The entry that keeps losing the race")
	if err := ledger.Add(ctx, store, e); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if e.ID != "dec-0003" {
		t.Errorf("allocated id = %q, want dec-0003 after losing dec-0001 and dec-0002", e.ID)
	}
	if got := store.createCalls(); got != 3 {
		t.Errorf("Create was called %d times, want 3: one per candidate", got)
	}

	// The winners were not clobbered.
	for _, id := range []string{"dec-0001", "dec-0002"} {
		got, err := store.Get(ctx, id)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if got.Title != "An entry a concurrent writer got in first with" {
			t.Errorf("%s holds %q; the retry overwrote the writer that won it", id, got.Title)
		}
	}
}

func TestAddGivesUpRatherThanSpinning(t *testing.T) {
	t.Parallel()

	store := newMemStore()
	// Every candidate is taken, forever: a backend that has gone wrong, or a
	// ledger under impossible contention. Either way Add has to end.
	store.beforeCreate = func(id string) error {
		return fmt.Errorf("%s: %w", id, ledger.ErrExists)
	}

	err := ledger.Add(context.Background(), store, draft(ledger.KindNote, "An entry with nowhere to go"))
	if err == nil {
		t.Fatal("Add succeeded against a store where every id is taken")
	}
	if !strings.Contains(err.Error(), "concurrent writers") {
		t.Errorf("error does not explain what happened: %v", err)
	}
}

func TestAddRefusesToWriteAnInvalidEntry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		entry func() *ledger.Entry
		want  string
	}{
		{
			name: "a sixth kind names the constraint",
			entry: func() *ledger.Entry {
				e := draft(ledger.KindNote, "A task, which is not a kind")
				e.Kind = "task"
				return e
			},
			want: "cst-0002",
		},
		{
			name: "a decision with no alternatives",
			entry: func() *ledger.Entry {
				e := draft(ledger.KindDecision, "A decision that is really an assertion")
				e.Alternatives = nil
				return e
			},
			want: "alternative",
		},
		{
			name: "an alternative with no why_not",
			entry: func() *ledger.Entry {
				e := draft(ledger.KindDecision, "A decision with a bare option")
				e.Alternatives = []ledger.Alternative{{Option: "The other way"}}
				return e
			},
			want: "alternatives[0]: why_not is required",
		},
		{
			name: "a state the kind does not allow",
			entry: func() *ledger.Entry {
				e := draft(ledger.KindQuestion, "A question that claims to be accepted")
				e.State = ledger.StateAccepted
				return e
			},
			want: "state \"accepted\" is not valid for kind \"question\"",
		},
		{
			name: "an edge to something that is not a ref",
			entry: func() *ledger.Entry {
				e := draft(ledger.KindNote, "A note pointing at nothing in particular")
				e.Edges = []ledger.Edge{{Type: ledger.EdgeDerivesFrom, To: "the big idea"}}
				return e
			},
			want: "edges[0]",
		},
		{
			name: "a title too short to read",
			entry: func() *ledger.Entry {
				e := draft(ledger.KindNote, "hm")
				return e
			},
			want: "title",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := newMemStore()
			err := ledger.Add(context.Background(), store, tc.entry())
			if err == nil {
				t.Fatal("Add wrote an entry that violates entry.schema.json")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error does not name the problem; want substring %q, got %v", tc.want, err)
			}
			// Rejected before the ledger, not after: the store was
			// never asked to write anything.
			if got := store.createCalls(); got != 0 {
				t.Errorf("the store saw %d Create calls for an invalid entry, want 0", got)
			}
			if ids := store.ids(); len(ids) != 0 {
				t.Errorf("the ledger holds %v after a rejected write", ids)
			}
		})
	}
}

func TestAddRefusesAnEntryThatAlreadyCarriesAnID(t *testing.T) {
	t.Parallel()

	store := newMemStore()
	e := draft(ledger.KindNote, "An entry that named itself")
	e.ID = "note-0007"

	err := ledger.Add(context.Background(), store, e)
	if err == nil {
		t.Fatal("Add accepted a caller-chosen id; ids are the ledger's to hand out")
	}
	if !strings.Contains(err.Error(), "note-0007") {
		t.Errorf("error does not name the offending id: %v", err)
	}
}

func TestAddRequiresTheCallerToStampCreated(t *testing.T) {
	t.Parallel()

	store := newMemStore()
	e := draft(ledger.KindNote, "An entry with no created stamp")
	e.Created = ""

	err := ledger.Add(context.Background(), store, e)
	if err == nil {
		t.Fatal("Add invented a timestamp rather than requiring one")
	}
	if !strings.Contains(err.Error(), "created") {
		t.Errorf("error does not name the missing field: %v", err)
	}
}

// TestAFailedWriteLeavesNothingBehind is the injected-failure clause at the
// interface: when the write fails, the ledger is as it was and the entry does
// not come back carrying an id it never got.
func TestAFailedWriteLeavesNothingBehind(t *testing.T) {
	t.Parallel()

	store := newMemStore()
	seed(t, store, "dec-0001")
	before := store.ids()

	boom := errors.New("the disk went away mid-write")
	store.beforeCreate = func(string) error { return boom }

	e := draft(ledger.KindDecision, "The decision that never landed")
	err := ledger.Add(context.Background(), store, e)
	if !errors.Is(err, boom) {
		t.Fatalf("Add: err = %v, want the injected failure", err)
	}
	if e.ID != "" {
		t.Errorf("the entry came back carrying id %q for a write that failed", e.ID)
	}

	after := store.ids()
	if len(after) != len(before) || after[0] != before[0] {
		t.Errorf("the ledger holds %v, want %v unchanged", after, before)
	}
}

func TestAddEdgeIsAdditiveIdempotentAndRefusesToRewriteANote(t *testing.T) {
	t.Parallel()

	base := func() *ledger.Entry {
		e := draft(ledger.KindDecision, "A decision that acquires edges")
		e.ID = "dec-0001"
		e.Edges = []ledger.Edge{{Type: ledger.EdgeDerivesFrom, To: "int-0002", Note: "latency is the reason"}}
		return e
	}

	t.Run("a new edge is appended", func(t *testing.T) {
		e := base()
		changed, err := ledger.AddEdge(e, ledger.Edge{Type: ledger.EdgeInforms, To: "dec-0005"})
		if err != nil {
			t.Fatalf("AddEdge: %v", err)
		}
		if !changed {
			t.Error("AddEdge reported no change after adding an edge")
		}
		if len(e.Edges) != 2 || e.Edges[1].To != "dec-0005" {
			t.Errorf("edges = %+v", e.Edges)
		}
		if e.Edges[0].To != "int-0002" {
			t.Error("the existing edge moved; edges are appended, not reordered")
		}
	})

	t.Run("an identical edge changes nothing", func(t *testing.T) {
		e := base()
		changed, err := ledger.AddEdge(e, e.Edges[0])
		if err != nil {
			t.Fatalf("AddEdge: %v", err)
		}
		if changed {
			t.Error("AddEdge reported a change for an edge already present")
		}
		if len(e.Edges) != 1 {
			t.Errorf("edges = %+v, want the one it started with", e.Edges)
		}
	})

	t.Run("the same edge with a different note is a conflict", func(t *testing.T) {
		e := base()
		_, err := ledger.AddEdge(e, ledger.Edge{Type: ledger.EdgeDerivesFrom, To: "int-0002", Note: "a different reason"})
		if err == nil {
			t.Fatal("AddEdge replaced an existing note without being asked")
		}
		if !strings.Contains(err.Error(), "int-0002") {
			t.Errorf("error does not name the edge: %v", err)
		}
		if len(e.Edges) != 1 || e.Edges[0].Note != "latency is the reason" {
			t.Errorf("edges = %+v; the conflict still mutated the entry", e.Edges)
		}
	})
}

func TestAddTagIsIdempotent(t *testing.T) {
	t.Parallel()

	e := draft(ledger.KindNote, "A note that acquires tags")
	e.Tags = []string{"storage"}

	if !ledger.AddTag(e, "latency") {
		t.Error("AddTag reported no change after adding a tag")
	}
	if ledger.AddTag(e, "storage") {
		t.Error("AddTag reported a change for a tag already present")
	}
	want := []string{"storage", "latency"}
	if len(e.Tags) != len(want) {
		t.Fatalf("tags = %v, want %v", e.Tags, want)
	}
	for i := range want {
		if e.Tags[i] != want[i] {
			t.Errorf("tags = %v, want %v", e.Tags, want)
		}
	}
}

func TestFormatIDMatchesTheSchemaPattern(t *testing.T) {
	t.Parallel()

	cases := []struct {
		kind ledger.Kind
		n    int
		want string
	}{
		{ledger.KindDecision, 1, "dec-0001"},
		{ledger.KindIntent, 42, "int-0042"},
		{ledger.KindQuestion, 9999, "qst-9999"},
		{ledger.KindNote, 10000, "note-10000"},
		{ledger.KindConstraint, 123456, "cst-123456"},
	}

	for _, tc := range cases {
		got := ledger.FormatID(tc.kind, tc.n)
		if got != tc.want {
			t.Errorf("FormatID(%s, %d) = %q, want %q", tc.kind, tc.n, got, tc.want)
		}
		if !ledger.ValidID(got) {
			t.Errorf("FormatID(%s, %d) = %q, which is not a valid id", tc.kind, tc.n, got)
		}
	}
}
