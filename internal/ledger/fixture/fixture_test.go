package fixture_test

import (
	"context"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/fixture"
	"github.com/kazi-org/dira/internal/ledger/local"
	"github.com/kazi-org/dira/schema"
)

// wantDigest pins the generator's exact output.
//
// This constant is what a committed fixture would have been: the review signal
// that says the 200-entry ledger every other E1 lane measures against has
// changed. E1-L3's cache tests and E1-L4's golden why-chains are written against
// this exact ledger, so a change here invalidates theirs. Updating it is a
// deliberate line in a diff, and it should arrive with the reason in the commit
// message.
const wantDigest = "7e46702abd119e4b09355ececf86e80f798b28024dfe75df24e599101f200f2d"

func generate(t *testing.T) []*ledger.Entry {
	t.Helper()

	entries, err := fixture.Generate(fixture.Seed, fixture.Size)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(entries) != fixture.Size {
		t.Fatalf("Generate returned %d entries, want %d", len(entries), fixture.Size)
	}
	return entries
}

// TestGenerationIsReproducible is E1-L1's acceptance line (d), first half: the
// same seed produces byte-identical output across two runs.
//
// It compares the encoded bytes rather than the structs, because bytes are what
// the later lanes consume and what a golden file is taken from — two runs could
// agree on every field and still differ in output if the codec's own behaviour
// depended on anything nondeterministic, such as map iteration order.
func TestGenerationIsReproducible(t *testing.T) {
	t.Parallel()

	first, err := fixture.Generate(fixture.Seed, fixture.Size)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	second, err := fixture.Generate(fixture.Seed, fixture.Size)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("two runs produced %d and %d entries", len(first), len(second))
	}

	for i := range first {
		a, err := ledger.Encode(first[i])
		if err != nil {
			t.Fatalf("encoding %s: %v", first[i].ID, err)
		}
		b, err := ledger.Encode(second[i])
		if err != nil {
			t.Fatalf("encoding %s: %v", second[i].ID, err)
		}
		if string(a) != string(b) {
			t.Fatalf("entry %d differs between two runs of the same seed:\n--- first ---\n%s\n--- second ---\n%s", i, a, b)
		}
	}
}

// TestDifferentSeedsDiffer keeps the reproducibility test above honest. A
// generator that ignored its seed would satisfy it perfectly.
func TestDifferentSeedsDiffer(t *testing.T) {
	t.Parallel()

	a, err := fixture.Generate(fixture.Seed, fixture.Size)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	b, err := fixture.Generate(fixture.Seed+1, fixture.Size)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	digestA, err := fixture.Digest(a)
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	digestB, err := fixture.Digest(b)
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if digestA == digestB {
		t.Error("two different seeds produced the same ledger; the seed is not being used")
	}
}

// TestDigestIsStable is the review gate that replaces 200 committed files.
func TestDigestIsStable(t *testing.T) {
	t.Parallel()

	got, err := fixture.Digest(generate(t))
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if got != wantDigest {
		t.Errorf("the generated ledger changed.\n got %s\nwant %s\n\n"+
			"E1-L3's cache tests and E1-L4's golden why-chains are written against the previous ledger. "+
			"If this change is intended, update wantDigest and say why in the commit.", got, wantDigest)
	}
}

// TestEveryEntryValidatesAgainstTheSchema is acceptance line (d), second half.
// It runs E0's JSON Schema validator — the published contract — rather than
// Entry.Validate, so the fixture is checked against the schema itself and not
// against dira's reading of it.
func TestEveryEntryValidatesAgainstTheSchema(t *testing.T) {
	t.Parallel()

	validator, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	entries := generate(t)
	for _, e := range entries {
		encoded, err := ledger.Encode(e)
		if err != nil {
			t.Fatalf("encoding %s: %v", e.ID, err)
		}
		if err := validator.Validate(encoded); err != nil {
			t.Errorf("%s does not satisfy entry.schema.json: %v\n%s", e.ID, err, encoded)
		}
	}
}

// TestFixtureRoundTrips is acceptance line (a) over the generated half of the
// corpus: every entry re-serializes to the bytes it was written as.
func TestFixtureRoundTrips(t *testing.T) {
	t.Parallel()

	for _, e := range generate(t) {
		want, err := ledger.Encode(e)
		if err != nil {
			t.Fatalf("encoding %s: %v", e.ID, err)
		}
		decoded, err := ledger.Decode(want)
		if err != nil {
			t.Fatalf("decoding %s: %v\n%s", e.ID, err, want)
		}
		got, err := ledger.Encode(decoded)
		if err != nil {
			t.Fatalf("re-encoding %s: %v", e.ID, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s does not round-trip:\n--- want ---\n%s\n--- got ---\n%s", e.ID, want, got)
		}
		if decoded.Body != e.Body {
			t.Errorf("%s: body = %q, want %q", e.ID, decoded.Body, e.Body)
		}
	}
}

// TestTheLedgerIsRealistic checks the fixture is worth measuring against. A
// ledger of 200 identical notes would satisfy every test above and be useless to
// E1-L3, L4 and L5, all of which read particular shapes out of it.
func TestTheLedgerIsRealistic(t *testing.T) {
	t.Parallel()

	entries := generate(t)

	kinds := map[ledger.Kind]int{}
	edgeTypes := map[ledger.EdgeType]int{}
	states := map[ledger.State]int{}
	var privates, adrs, withAlternatives, withRevisitIf, blockingQuestions int

	ids := make(map[string]bool, len(entries))
	for _, e := range entries {
		ids[e.ID] = true
	}

	for _, e := range entries {
		kinds[e.Kind]++
		states[e.State]++
		if e.Private {
			privates++
		}
		if e.ADR != "" {
			adrs++
		}
		if len(e.Alternatives) > 0 {
			withAlternatives++
		}
		for _, alt := range e.Alternatives {
			if alt.RevisitIf != "" {
				withRevisitIf++
			}
		}
		for _, edge := range e.Edges {
			edgeTypes[edge.Type]++
			if edge.Type == ledger.EdgeRealizedBy {
				if !strings.HasPrefix(edge.To, "kazi:") {
					t.Errorf("%s: realized_by points at %q, want a kazi execution artifact", e.ID, edge.To)
				}
				continue
			}
			if !ids[edge.To] {
				t.Errorf("%s: %s edge points at %s, which is not in the ledger", e.ID, edge.Type, edge.To)
			}
			if edge.Type == ledger.EdgeBlocks && e.Kind == ledger.KindQuestion {
				blockingQuestions++
			}
		}
	}

	for _, kind := range ledger.Kinds {
		if kinds[kind] == 0 {
			t.Errorf("the fixture has no %s entries", kind)
		}
	}
	for _, edgeType := range ledger.EdgeTypes {
		if edgeTypes[edgeType] == 0 {
			t.Errorf("the fixture has no %s edges", edgeType)
		}
	}
	if privates == 0 {
		t.Error("no entry is private; cst-0003's cases would go untested")
	}
	if adrs == 0 {
		t.Error("no entry carries an adr path; E1-L4 reads that field")
	}
	if withAlternatives != kinds[ledger.KindDecision] {
		t.Errorf("%d entries carry alternatives but there are %d decisions; every decision must record at least one",
			withAlternatives, kinds[ledger.KindDecision])
	}
	if withRevisitIf == 0 {
		t.Error("no alternative carries revisit_if; E1-L4 renders it when present")
	}
	if blockingQuestions == 0 {
		t.Error("no open question blocks anything; that shape is the first thing E1-L5's brief renders")
	}
	if len(states) < 5 {
		t.Errorf("the fixture uses %d states, want a spread across the lifecycle", len(states))
	}
	// A superseded entry only exists if a supersedes edge flipped one.
	if states[ledger.StateSuperseded] == 0 {
		t.Error("nothing is superseded, so the supersedes edges point at entries that do not know it")
	}
}

// TestFixtureSurvivesTheStore materialises the whole ledger through the storage
// interface and reads it back. It is the end-to-end check that the codec, the
// interface and the backend agree over 200 entries rather than over one.
func TestFixtureSurvivesTheStore(t *testing.T) {
	t.Parallel()

	store, err := local.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ctx := context.Background()

	entries := generate(t)
	if err := fixture.Write(ctx, store, entries); err != nil {
		t.Fatalf("Write: %v", err)
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != len(entries) {
		t.Fatalf("List returned %d entries, want %d", len(list), len(entries))
	}

	byID := make(map[string]*ledger.Entry, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
	}
	for _, info := range list {
		want, ok := byID[info.ID]
		if !ok {
			t.Errorf("the store holds %s, which was not generated", info.ID)
			continue
		}
		got, err := store.Get(ctx, info.ID)
		if err != nil {
			t.Fatalf("Get %s: %v", info.ID, err)
		}

		wantBytes, err := ledger.Encode(want)
		if err != nil {
			t.Fatalf("encoding %s: %v", want.ID, err)
		}
		gotBytes, err := ledger.Encode(got)
		if err != nil {
			t.Fatalf("re-encoding %s: %v", got.ID, err)
		}
		if string(gotBytes) != string(wantBytes) {
			t.Errorf("%s changed on its way through the store:\n--- want ---\n%s\n--- got ---\n%s", info.ID, wantBytes, gotBytes)
		}
	}
}

// TestSmallerFixturesAreStillRealistic covers the scaling path, which exists so
// a test that wants twenty entries does not have to pay for two hundred and does
// not end up with twenty notes.
func TestSmallerFixturesAreStillRealistic(t *testing.T) {
	t.Parallel()

	entries, err := fixture.Generate(fixture.Seed, 20)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("Generate(20) returned nothing")
	}

	kinds := map[ledger.Kind]int{}
	for _, e := range entries {
		kinds[e.Kind]++
	}
	for _, kind := range ledger.Kinds {
		if kinds[kind] == 0 {
			t.Errorf("a 20-entry fixture has no %s entries", kind)
		}
	}
}

// TestGenerateRejectsANonsenseSize covers the one input that has no sensible
// answer, so it fails rather than returning an empty ledger a caller would
// measure against.
func TestGenerateRejectsANonsenseSize(t *testing.T) {
	t.Parallel()

	for _, n := range []int{0, -1} {
		if _, err := fixture.Generate(fixture.Seed, n); err == nil {
			t.Errorf("Generate(seed, %d) succeeded, want an error", n)
		}
	}
}
