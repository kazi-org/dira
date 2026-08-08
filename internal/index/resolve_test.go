package index_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/index"
)

// The all-tokens rule, and the ordering that keeps it from moving an answer
// anyone already has.
//
// Before this, Resolve matched a multi-word term only as one contiguous
// substring, so `dira why "read time"` found dec-0004 and `dira why "status
// derived"` found nothing — though dec-0004's title carries both words. dec-0014
// chose *lexical* matching, and a contiguous-substring test is narrower than
// lexical: it can only find a phrase typed in the order the title happens to
// use.
//
// The widening is bounded on both sides. Every word must still be present, so a
// term does not decay into an OR over common words; and a contiguous hit sorts
// ahead of a scattered one, so the entry a term resolved to yesterday is still
// the entry it resolves to today.

// realLedger is a copy of this repository's own ledger, which is where the case
// that motivated this lives: dec-0004's title is "Execution status is derived
// from kazi at read time, never stored in the ledger".
//
// A copy rather than the repository's own .dira, so these tests neither read a
// cache another test is writing nor leave one behind.
func realLedger(t *testing.T) *index.Index {
	t.Helper()

	src := filepath.Join("..", "..", ".dira", "entries")
	names, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("reading this repository's ledger: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("this repository's ledger is empty; these tests would measure nothing")
	}

	diraDir := filepath.Join(t.TempDir(), ".dira")
	entries := filepath.Join(diraDir, "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatalf("creating %s: %v", entries, err)
	}
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(src, name.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", name.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(entries, name.Name()), b, 0o644); err != nil {
			t.Fatalf("writing %s: %v", name.Name(), err)
		}
	}
	return openIndex(t, diraDir)
}

func TestScatteredTokensResolve(t *testing.T) {
	t.Parallel()

	ix := realLedger(t)
	ctx := context.Background()

	cases := []struct {
		term string
		want string
		// why names the property the case pins, so a case deleted as
		// "redundant" takes a visible obligation with it.
		why string
	}{
		{term: "read time", want: "dec-0004", why: "contiguous in the title — this is what worked before, and must still"},
		{term: "status derived", want: "dec-0004", why: "both words in the title, neither adjacent nor in order"},
		{term: "derived status", want: "dec-0004", why: "word order is not part of the match"},
		{term: "kazi   status", want: "dec-0004", why: "inner whitespace is collapsed rather than matched"},
	}

	for _, tc := range cases {
		t.Run(tc.term, func(t *testing.T) {
			got, err := ix.Resolve(ctx, tc.term)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(got, tc.want) {
				t.Errorf("Resolve(%q) = %v, which does not contain %s (%s)", tc.term, got, tc.want, tc.why)
			}
		})
	}
}

// TestEveryWordMustBePresent is the other side of the widening. A term whose
// words are not all in one entry must not resolve to the entries that carry some
// of them, or `dira why "the cache"` becomes a listing of the whole ledger.
func TestEveryWordMustBePresent(t *testing.T) {
	t.Parallel()

	ix := realLedger(t)
	ctx := context.Background()

	// "daemon" is in int-0002's title; "tokenizer" is in no title and no tag
	// in this ledger. Together they must resolve to nothing.
	got, err := ix.Resolve(ctx, "daemon tokenizer")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("Resolve(%q) = %v, want nothing: one word of it matches nothing in this ledger", "daemon tokenizer", got)
	}

	// And the half that does match must still match alone, or the case above
	// would pass for the wrong reason.
	alone, err := ix.Resolve(ctx, "daemon")
	if err != nil {
		t.Fatal(err)
	}
	if len(alone) == 0 {
		t.Fatal(`Resolve("daemon") found nothing; the test above proves nothing`)
	}
}

// TestAContiguousMatchIsTheWholeAnswer is the rule that makes this change safe
// rather than merely wider.
//
// Ordering contiguous matches first is not enough. `dira why` renders a
// disambiguation list instead of a chain the moment a term matches more than one
// entry, so a term that found one entry yesterday and finds two today has
// changed what a reader sees even with the old answer still at the top. That is
// what `dira why "read time"` did under a rank-only scheme: dec-0004's chain
// became a two-item menu, because cst-0003's title carries "read-time" and
// therefore both words. Scattered matching is a fallback for a phrase that is
// nowhere; when the phrase is somewhere, it is the whole answer.
func TestAContiguousMatchIsTheWholeAnswer(t *testing.T) {
	t.Parallel()

	ix := realLedger(t)
	ctx := context.Background()

	const term = "read time"
	got, err := ix.Resolve(ctx, term)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "dec-0004" {
		t.Errorf("Resolve(%q) = %v, want exactly [dec-0004]: a phrase that is found is the whole answer", term, got)
	}

	// And the entry a looser rule would have added really does satisfy the
	// all-words test, or the assertion above excludes nothing.
	loose, err := ix.Resolve(ctx, "read-time private")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(loose, "cst-0003") {
		t.Fatalf("cst-0003 does not match the words it was meant to; this case excludes nothing (got %v)", loose)
	}
}

// TestScatteredMatchesAreReturnedWhenThePhraseIsNowhere is the other side of the
// same rule: the fallback has to fire, or the widening is theoretical.
func TestScatteredMatchesAreReturnedWhenThePhraseIsNowhere(t *testing.T) {
	t.Parallel()

	ix := realLedger(t)
	ctx := context.Background()

	const term = "status derived"
	got, err := ix.Resolve(ctx, term)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatalf("Resolve(%q) found nothing, though dec-0004's title carries both words", term)
	}
	for _, id := range got {
		entry, err := ix.Entry(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(entry.Title), term) {
			t.Fatalf("%s matches %q contiguously, so this case is not testing the fallback", id, term)
		}
		for _, word := range strings.Fields(term) {
			if !strings.Contains(strings.ToLower(entry.Title), word) && !slices.Contains(entry.Tags, word) {
				t.Errorf("%s came back for %q but carries %q neither in its title nor as a tag", id, term, word)
			}
		}
	}
}

// TestASingleWordResolvesExactlyAsItDid is the no-regression half, and it is the
// reason the rank probe and the word predicate are the same SQL. A one-word term
// is contiguous with itself, so both its result set and its order are what they
// were before this change — which is what keeps cmd/dira/testdata/why/*.golden
// where it is.
func TestASingleWordResolvesExactlyAsItDid(t *testing.T) {
	t.Parallel()

	ix := realLedger(t)
	ctx := context.Background()

	for _, term := range []string{"daemon", "founding", "cache", "latency", "ledger"} {
		t.Run(term, func(t *testing.T) {
			got, err := ix.Resolve(ctx, term)
			if err != nil {
				t.Fatal(err)
			}
			want, err := resolveTheOldWay(ctx, ix, term)
			if err != nil {
				t.Fatal(err)
			}
			if len(want) == 0 {
				t.Fatalf("%q matched nothing under either rule; this case measures nothing", term)
			}
			if !slices.Equal(got, want) {
				t.Errorf("Resolve(%q) = %v, want %v — a single word must resolve exactly as it did before scattered matching",
					term, got, want)
			}
		})
	}
}

// resolveTheOldWay is the pre-change query, kept here as the oracle the case
// above compares against. It is deliberately a second implementation rather than
// a recorded list: a recorded list would agree with whatever the code did on the
// day it was recorded.
func resolveTheOldWay(ctx context.Context, ix *index.Index, term string) ([]string, error) {
	refs, err := ix.Select(ctx, index.Selector{})
	if err != nil {
		return nil, err
	}
	lowered := strings.ToLower(strings.TrimSpace(term))

	out := []string{}
	for _, ref := range refs {
		entry, err := ix.Entry(ctx, ref.ID)
		if err != nil {
			return nil, err
		}
		title := strings.Contains(strings.ToLower(entry.Title), lowered)
		tag := slices.Contains(entry.Tags, lowered)
		if title || tag {
			out = append(out, ref.ID)
		}
	}
	// Select is already `created DESC, id ASC`, which is Resolve's order for
	// a term whose matches are all contiguous.
	return out, nil
}
