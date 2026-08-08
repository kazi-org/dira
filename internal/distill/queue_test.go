package distill

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// Every test here is written to fail on a false premise as well as pass on the
// true one (docs/lore.md L-0001). Where an assertion is "X is included", the
// same test also observes the state in which X would be excluded; where it is
// "the order is identical", it also shows that the comparison can tell two
// orders apart. A green half nobody watched is the half that is usually broken.

// tempLedger builds an empty ledger and returns it as the interface value the
// queue actually takes, plus the paths a test needs to reach behind the API.
func tempLedger(t *testing.T) (ledger.Store, string, string) {
	t.Helper()

	root := t.TempDir()
	dira := filepath.Join(root, ".dira")
	entries := filepath.Join(dira, "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatalf("creating the temp ledger: %v", err)
	}
	store, err := local.Open(dira)
	if err != nil {
		t.Fatalf("opening the temp ledger: %v", err)
	}
	return store, dira, entries
}

func stagedDecision(id, title string) *ledger.Entry {
	return &ledger.Entry{
		ID:      id,
		Kind:    ledger.KindDecision,
		Title:   title,
		State:   ledger.StateStaged,
		Created: "2026-07-31T09:00:00Z",
		Source: &ledger.Source{
			Hook:    ledger.HookStop,
			Excerpt: "we are going with " + title + " rather than the other thing",
			Tier:    ledger.TierRegex,
		},
	}
}

func decisionIn(id, title string, state ledger.State) *ledger.Entry {
	e := stagedDecision(id, title)
	e.State = state
	e.Alternatives = []ledger.Alternative{{
		Option: "do nothing at all",
		WhyNot: "the problem does not go away on its own",
	}}
	e.ConfirmedBy = "human"
	e.Source.Tier = ledger.TierHuman
	e.Source.Hook = ledger.HookManual
	return e
}

func put(t *testing.T, store ledger.Store, entries ...*ledger.Entry) {
	t.Helper()
	for _, e := range entries {
		if err := store.Put(context.Background(), e); err != nil {
			t.Fatalf("writing %s: %v", e.ID, err)
		}
	}
}

// setState rewrites an entry's state through the store, leaving every other
// field as it was on disk.
//
// Moving a decision out of `staged` requires an alternative, which is dec-0021's
// minItems rule and the whole reason this lane exists — the codec refuses the
// write otherwise, so the fixture has to supply one. That is a fact about the
// schema, not about the queue: nothing here is asserting on alternatives.
func setState(t *testing.T, store ledger.Store, id string, state ledger.State) {
	t.Helper()
	entry, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("reading %s: %v", id, err)
	}
	entry.State = state
	if entry.Kind == ledger.KindDecision && state != ledger.StateStaged && len(entry.Alternatives) == 0 {
		entry.Alternatives = []ledger.Alternative{{
			Option: "do nothing at all",
			WhyNot: "the problem does not go away on its own",
		}}
	}
	if err := store.Put(context.Background(), entry); err != nil {
		t.Fatalf("rewriting %s: %v", id, err)
	}
}

func ids(q *Queue) []string {
	out := make([]string, 0, len(q.Items))
	for _, item := range q.Items {
		out = append(out, item.ID())
	}
	return out
}

func itemIDs(items []Item) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID())
	}
	return out
}

func read(t *testing.T, store ledger.Store) *Queue {
	t.Helper()
	q, err := Staged(context.Background(), store)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	return q
}

// eightEntries is the acceptance line's ledger: three staged and five that are
// not. The five cover every other kind as well as the two non-staged decision
// states, so "returns exactly the three" is not an accident of there being only
// decisions in the directory.
func eightEntries(t *testing.T, store ledger.Store) {
	t.Helper()
	put(t, store,
		stagedDecision("dec-0001", "postgres over sqlite"),
		stagedDecision("dec-0002", "one binary rather than two"),
		stagedDecision("dec-0003", "vendor the parser"),

		decisionIn("dec-0004", "server-rendered templates", ledger.StateAccepted),
		decisionIn("dec-0005", "an SPA for the console", ledger.StateRejected),
		&ledger.Entry{ID: "int-0001", Kind: ledger.KindIntent, Title: "keep cold start under 100ms",
			State: ledger.StateActive, Created: "2026-07-31T09:00:00Z"},
		&ledger.Entry{ID: "qst-0001", Kind: ledger.KindQuestion, Title: "who owns the parent ledger map",
			State: ledger.StateOpen, Created: "2026-07-31T09:00:00Z"},
		&ledger.Entry{ID: "cst-0001", Kind: ledger.KindConstraint, Title: "no daemon, ever",
			State: ledger.StateActive, Created: "2026-07-31T09:00:00Z"},
	)
}

// TestQueueReturnsExactlyTheStagedEntries is the acceptance line's first clause.
//
// The green side is the three. The red side is run twice in the same test, from
// both directions: a non-staged entry made staged must appear, and a staged
// entry made non-staged must vanish. Without those, a queue that returned the
// first three ids in the directory would pass.
func TestQueueReturnsExactlyTheStagedEntries(t *testing.T) {
	t.Parallel()
	store, _, _ := tempLedger(t)
	eightEntries(t, store)

	want := []string{"dec-0001", "dec-0002", "dec-0003"}
	if got := ids(read(t, store)); !reflect.DeepEqual(got, want) {
		t.Fatalf("staged queue = %v, want %v", got, want)
	}

	// Red side one: an accepted entry moved to staged must join the queue.
	// A queue that hardcoded three, or that read the id range, passes the
	// assertion above and fails this one.
	setState(t, store, "dec-0004", ledger.StateStaged)
	want = []string{"dec-0001", "dec-0002", "dec-0003", "dec-0004"}
	if got := ids(read(t, store)); !reflect.DeepEqual(got, want) {
		t.Fatalf("after staging dec-0004, queue = %v, want %v", got, want)
	}

	// Red side two: a staged entry that leaves staged must drop out.
	setState(t, store, "dec-0001", ledger.StateAccepted)
	want = []string{"dec-0002", "dec-0003", "dec-0004"}
	if got := ids(read(t, store)); !reflect.DeepEqual(got, want) {
		t.Fatalf("after accepting dec-0001, queue = %v, want %v", got, want)
	}

	// Green side restored: put it back and the original answer returns.
	setState(t, store, "dec-0004", ledger.StateAccepted)
	setState(t, store, "dec-0001", ledger.StateStaged)
	want = []string{"dec-0001", "dec-0002", "dec-0003"}
	if got := ids(read(t, store)); !reflect.DeepEqual(got, want) {
		t.Fatalf("restored queue = %v, want %v", got, want)
	}
}

// TestQueueOrderIsIdenticalAcrossReadsAndACacheRebuild is the `1 of 3` promise:
// the second invocation shows the same card second.
//
// The comparison itself is checked before it is trusted. Two empty slices are
// equal, and so are two slices from a queue that always returns the same wrong
// thing, so the test first requires the queue to be non-empty and then requires
// the comparison to reject a permutation of that same queue.
func TestQueueOrderIsIdenticalAcrossReadsAndACacheRebuild(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, dira, _ := tempLedger(t)
	eightEntries(t, store)

	first := ids(read(t, store))
	if len(first) != 3 {
		t.Fatalf("expected 3 staged entries to compare, got %d — an equal-order assertion over an empty queue proves nothing", len(first))
	}

	// The negative control for the comparison, not for the queue: reversing
	// the answer must be detected. If DeepEqual could not tell these apart,
	// every assertion below would be vacuous.
	reversed := append([]string(nil), first...)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	if reflect.DeepEqual(first, reversed) {
		t.Fatalf("the order comparison cannot distinguish %v from its reverse", first)
	}

	if second := ids(read(t, store)); !reflect.DeepEqual(first, second) {
		t.Fatalf("second read = %v, first read = %v", second, first)
	}

	// A cold cache built from nothing.
	ix, err := index.Open(ctx, store, local.CacheDir(dira))
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	if err := ix.Close(); err != nil {
		t.Fatalf("closing the index: %v", err)
	}
	if got := ids(read(t, store)); !reflect.DeepEqual(first, got) {
		t.Fatalf("after building the cache, queue = %v, want %v", got, first)
	}

	// And a rebuild: the cache discarded and made again, which is what
	// `dira reindex` runs.
	fresh, err := index.OpenFresh(ctx, store, local.CacheDir(dira))
	if err != nil {
		t.Fatalf("index.OpenFresh: %v", err)
	}
	if !fresh.Stats().Rebuilt {
		t.Fatalf("OpenFresh reported no rebuild, so this test did not exercise one")
	}
	if err := fresh.Close(); err != nil {
		t.Fatalf("closing the rebuilt index: %v", err)
	}
	if got := ids(read(t, store)); !reflect.DeepEqual(first, got) {
		t.Fatalf("after a cache rebuild, queue = %v, want %v", got, first)
	}
}

// TestQueueIncludesAnEntryTheCacheCallsAccepted is dec-0002's files-win rule,
// asserted where it bites: the file says staged and the cache does not.
//
// The cache is made genuinely stale rather than assumed to be. index.Open
// reconciles before it answers, so the only way to hold a cache that disagrees
// with the files is to keep one open across a write — and Select does not
// reconcile. The test therefore *observes* the disagreement (the cache omits the
// entry) before asserting that the queue includes it anyway. Without that
// observation the assertion would be green against a cache that happened to
// agree, and would prove nothing about precedence.
func TestQueueIncludesAnEntryTheCacheCallsAccepted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, dira, _ := tempLedger(t)

	put(t, store,
		decisionIn("dec-0001", "self-host the font", ledger.StateAccepted),
		stagedDecision("dec-0002", "one binary rather than two"),
	)

	ix, err := index.Open(ctx, store, local.CacheDir(dira))
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	defer func() { _ = ix.Close() }()

	cachedStaged := func() []string {
		refs, err := ix.Select(ctx, index.Selector{States: []ledger.State{ledger.StateStaged}})
		if err != nil {
			t.Fatalf("index.Select: %v", err)
		}
		out := make([]string, 0, len(refs))
		for _, ref := range refs {
			out = append(out, ref.ID)
		}
		return out
	}

	// Baseline: the cache agrees with the files, and knows dec-0001 is not
	// staged. This is the green side of the cache's own answer — without it,
	// a cache that returned nothing for every query would look identical to
	// a stale one.
	if got := cachedStaged(); !reflect.DeepEqual(got, []string{"dec-0002"}) {
		t.Fatalf("cache staged set before the write = %v, want [dec-0002]", got)
	}

	setState(t, store, "dec-0001", ledger.StateStaged)

	// The disagreement, observed rather than assumed: the open index still
	// says dec-0001 is not staged.
	if got := cachedStaged(); !reflect.DeepEqual(got, []string{"dec-0002"}) {
		t.Fatalf("the cache reconciled behind the test's back (staged set = %v); this test can no longer prove precedence", got)
	}

	// The files win.
	if got := ids(read(t, store)); !reflect.DeepEqual(got, []string{"dec-0001", "dec-0002"}) {
		t.Fatalf("queue = %v, want [dec-0001 dec-0002] — the file says staged, so the entry is in the queue", got)
	}
}

// TestQueueSkipsAMalformedEntryWithAWarning is the "one bad file must not cost a
// human the other cards" clause.
func TestQueueSkipsAMalformedEntryWithAWarning(t *testing.T) {
	t.Parallel()
	store, _, entries := tempLedger(t)
	eightEntries(t, store)

	// Green side first: a clean ledger warns about nothing. A queue that
	// warned unconditionally would pass every assertion below.
	if q := read(t, store); len(q.Warnings) != 0 {
		t.Fatalf("clean ledger produced warnings: %v", q.Warnings)
	}

	good, err := os.ReadFile(filepath.Join(entries, "dec-0002.md"))
	if err != nil {
		t.Fatalf("reading dec-0002.md: %v", err)
	}
	broken := "---\nid: dec-0002\nkind: decision\ntitle: \"unterminated\nstate: staged\n---\n"
	if err := os.WriteFile(filepath.Join(entries, "dec-0002.md"), []byte(broken), 0o644); err != nil {
		t.Fatalf("corrupting dec-0002.md: %v", err)
	}

	q := read(t, store)
	if got := ids(q); !reflect.DeepEqual(got, []string{"dec-0001", "dec-0003"}) {
		t.Fatalf("queue = %v, want [dec-0001 dec-0003] — the other cards survive one bad file", got)
	}
	if len(q.Warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", q.Warnings)
	}
	if !strings.Contains(q.Warnings[0], "dec-0002") {
		t.Fatalf("warning %q does not name the entry it skipped", q.Warnings[0])
	}
	if strings.Contains(q.Warnings[0], "\n") {
		t.Fatalf("warning is more than one line: %q", q.Warnings[0])
	}

	// Green side again: repair the file and both the card and the silence
	// come back. A skip that were permanent, or a warning that stuck, would
	// only show up here.
	if err := os.WriteFile(filepath.Join(entries, "dec-0002.md"), good, 0o644); err != nil {
		t.Fatalf("repairing dec-0002.md: %v", err)
	}
	q = read(t, store)
	if got := ids(q); !reflect.DeepEqual(got, []string{"dec-0001", "dec-0002", "dec-0003"}) {
		t.Fatalf("after repair, queue = %v, want all three", got)
	}
	if len(q.Warnings) != 0 {
		t.Fatalf("after repair, warnings = %v, want none", q.Warnings)
	}
}

// TestQueueWarningStaysOneLine checks the collapsing directly, because the YAML
// parser this ledger uses happens to produce single-line errors today and a
// warning that only stays one line by luck is not a property.
func TestQueueWarningStaysOneLine(t *testing.T) {
	t.Parallel()

	got := skipped("dec-0009", errors.New("parsing frontmatter:\n  line 4:\n  found a tab"))
	if strings.Contains(got, "\n") {
		t.Fatalf("warning spans lines: %q", got)
	}
	if !strings.Contains(got, "dec-0009") {
		t.Fatalf("warning %q does not name the entry", got)
	}
	// The negative control for the collapsing itself: the raw message it was
	// built from does contain newlines, so the assertion above is not
	// trivially true of any string.
	if !strings.Contains("parsing frontmatter:\n  line 4:\n  found a tab", "\n") {
		t.Fatal("the fixture error has no newlines, so this test proves nothing")
	}
	if oneLine("a b") != "a b" {
		t.Fatalf("oneLine mangled a string that was already one line: %q", oneLine("a b"))
	}
}

// TestQueueOnAnEmptyLedgerIsEmptyAndNotAnError covers both shapes of empty: a
// ledger with an entries directory holding nothing, and one with no entries
// directory at all, which is what a repository gets before its first capture.
func TestQueueOnAnEmptyLedgerIsEmptyAndNotAnError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store, _, entries := tempLedger(t)
	q, err := Staged(ctx, store)
	if err != nil {
		t.Fatalf("Staged over an empty ledger: %v", err)
	}
	if q.Len() != 0 || len(q.Warnings) != 0 {
		t.Fatalf("empty ledger produced %d items and %v", q.Len(), q.Warnings)
	}
	if q.Items == nil {
		t.Fatal("Items is nil; an empty queue is an empty slice, so a caller can range over it")
	}

	if err := os.RemoveAll(entries); err != nil {
		t.Fatalf("removing the entries directory: %v", err)
	}
	q, err = Staged(ctx, store)
	if err != nil {
		t.Fatalf("Staged over a ledger with no entries directory: %v", err)
	}
	if q.Len() != 0 {
		t.Fatalf("ledger with no entries directory produced %d items", q.Len())
	}

	// The green side: the same call over the same store returns a card once
	// there is one, so "empty" is an answer about this ledger and not a
	// constant.
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatalf("recreating the entries directory: %v", err)
	}
	put(t, store, stagedDecision("dec-0001", "one binary rather than two"))
	if got := ids(read(t, store)); !reflect.DeepEqual(got, []string{"dec-0001"}) {
		t.Fatalf("queue = %v, want [dec-0001]", got)
	}
}

// TestQueueTakesAStoreNotAPath is the acceptance line's last clause. The reader
// is driven through local.Open's Store value and given no path of its own, and
// the signature is inspected so that a later edit adding a directory parameter
// fails here rather than in E7 when the github backend cannot supply one
// (dec-0005).
func TestQueueTakesAStoreNotAPath(t *testing.T) {
	t.Parallel()

	// Compile-time: Staged is assignable to exactly this function type, so a
	// parameter of any other type — a directory string most of all — stops
	// the package building rather than failing an assertion.
	reader := asReader(Staged)

	storeType := reflect.TypeOf((*ledger.Store)(nil)).Elem()
	signature := reflect.TypeOf(Staged)
	if signature.NumIn() != 2 || signature.In(1) != storeType {
		t.Fatalf("Staged takes %v; want (context.Context, ledger.Store)", signature)
	}
	if takesAPath(signature) {
		t.Fatalf("Staged takes a string parameter: %v", signature)
	}
	// Negative control for the predicate: it must report a violation on a
	// function that really does take a path, or its verdict on Staged means
	// nothing.
	if !takesAPath(reflect.TypeOf(func(context.Context, string) (*Queue, error) { return nil, nil })) {
		t.Fatal("takesAPath cannot see a path parameter, so its verdict on Staged is worthless")
	}

	// And drive it: tempLedger hands back local.Open's value already widened
	// to the ledger.Store interface, and that value plus a context is
	// everything the reader is given. No path is in scope here at all.
	store, _, _ := tempLedger(t)
	put(t, store, stagedDecision("dec-0001", "one binary rather than two"))

	q, err := reader(context.Background(), store)
	if err != nil {
		t.Fatalf("Staged through the interface: %v", err)
	}
	if got := ids(q); !reflect.DeepEqual(got, []string{"dec-0001"}) {
		t.Fatalf("queue = %v, want [dec-0001]", got)
	}
}

// asReader is the compile-time gate: only a function of exactly this type can
// be passed to it, so a Staged that grew a directory parameter would fail to
// build rather than fail an assertion at run time.
func asReader(fn func(context.Context, ledger.Store) (*Queue, error)) func(context.Context, ledger.Store) (*Queue, error) {
	return fn
}

// takesAPath reports whether any parameter is a string, which over this API is
// how a directory would arrive.
func takesAPath(fn reflect.Type) bool {
	for i := 0; i < fn.NumIn(); i++ {
		if fn.In(i).Kind() == reflect.String {
			return true
		}
	}
	return false
}

// TestQueueShowsPendingExtraction is dec-0022's constraint on this lane: a
// confirmed entry stays `staged`, so without a visible distinction "promoted,
// awaiting its reasoning" and "nobody has looked at this" are the same row.
//
// Both marks the promotion leaves are checked on their own, because a queue that
// keyed on only one of them would mislabel an entry the transition happened to
// write the other way round.
func TestQueueShowsPendingExtraction(t *testing.T) {
	t.Parallel()
	store, _, _ := tempLedger(t)

	untouched := stagedDecision("dec-0001", "postgres over sqlite")

	confirmed := stagedDecision("dec-0002", "one binary rather than two")
	confirmed.ConfirmedBy = "human"

	promoted := stagedDecision("dec-0003", "vendor the parser")
	promoted.Source.Tier = ledger.TierSemantic

	put(t, store, untouched, confirmed, promoted)

	q := read(t, store)

	// All three are still in the queue. Visibility is the requirement, not
	// removal: an entry dropped from the queue would pile up exactly as
	// invisibly as one shown with the wrong status.
	if q.Len() != 3 {
		t.Fatalf("queue holds %d items, want 3", q.Len())
	}
	want := map[string]Status{
		"dec-0001": StatusAwaiting,
		"dec-0002": StatusPendingExtraction,
		"dec-0003": StatusPendingExtraction,
	}
	for _, item := range q.Items {
		if got := item.Status; got != want[item.ID()] {
			t.Fatalf("%s has status %q, want %q", item.ID(), got, want[item.ID()])
		}
	}
	if got := itemIDs(q.Awaiting()); !reflect.DeepEqual(got, []string{"dec-0001"}) {
		t.Fatalf("Awaiting() = %v, want [dec-0001]", got)
	}
	if got := itemIDs(q.PendingExtraction()); !reflect.DeepEqual(got, []string{"dec-0002", "dec-0003"}) {
		t.Fatalf("PendingExtraction() = %v, want [dec-0002 dec-0003]", got)
	}

	// Red side: clear each mark in turn and the entry must fall back to
	// awaiting. Without this, a classifier that returned pending for every
	// staged entry would pass everything above except the dec-0001 row.
	confirmed.ConfirmedBy = ""
	promoted.Source.Tier = ledger.TierRegex
	put(t, store, confirmed, promoted)

	q = read(t, store)
	if got := itemIDs(q.Awaiting()); !reflect.DeepEqual(got, []string{"dec-0001", "dec-0002", "dec-0003"}) {
		t.Fatalf("with both marks cleared, Awaiting() = %v, want all three", got)
	}
	if got := q.PendingExtraction(); len(got) != 0 {
		t.Fatalf("with both marks cleared, PendingExtraction() = %v, want none", itemIDs(got))
	}
}
