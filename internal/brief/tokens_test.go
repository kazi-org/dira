package brief_test

import (
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/brief"
)

// The counter is the whole of cst-0001's enforcement, so what it has to be is
// not "accurate" — no offline estimator can be — but **conservative and
// deterministic**. These cases pin exactly that, and nothing about a particular
// vendor's tokenizer, which dira has no way to consult (dec-0003, cst-0004).

// TestTokensNeverUnderCountsARealTokenizer is the property the cap rests on.
//
// The reference is a lower bound rather than a tokenizer: no BPE vocabulary can
// encode a string in fewer tokens than it has non-space runs of letters, digits
// and punctuation *of length one* — every merge covers at least one character,
// and no merge crosses a space in the middle of a line. So a count that is below
// that lower bound is under-counting for certain, and one above it is only
// possibly generous.
//
// It is deliberately not a comparison against a fixed number per string, which
// would pin this estimator's arithmetic rather than its direction of error.
func TestTokensNeverUnderCountsARealTokenizer(t *testing.T) {
	t.Parallel()

	cases := []string{
		"dec-0002",
		"the session brief never exceeds 1,500 tokens",
		"open blockers",
		"  qst-0001  How does a public repo ledger resolve a parent ref?    open 2026-07-29",
		"omitted  12 recent decisions — over the 1,500-token ceiling (cst-0001)",
		"→ supersede dec-0060, or revise the plan",
		"dira brief — dira · 2026-07-30",
		strings.Repeat("a", 400),
		"",
	}

	for _, text := range cases {
		t.Run(short(text), func(t *testing.T) {
			got := brief.Tokens(text)
			if floor := lowerBound(text); got < floor {
				t.Errorf("Tokens(%q) = %d, below the %d tokens any tokenizer needs for it", text, got, floor)
			}
		})
	}
}

// lowerBound counts what no tokenizer can go under: one per whitespace-delimited
// word, plus one per line.
func lowerBound(text string) int {
	if text == "" {
		return 0
	}
	n := 0
	for _, line := range strings.Split(text, "\n") {
		n++
		n += len(strings.Fields(line))
	}
	return n
}

// TestTokensIsDeterministic. A counter that varied would make the ceiling a
// coin toss and the golden output unstable.
func TestTokensIsDeterministic(t *testing.T) {
	t.Parallel()

	const text = "open blockers\n  qst-0001  A question that blocks a decision    open 2026-07-29\n"
	first := brief.Tokens(text)
	for range 100 {
		if got := brief.Tokens(text); got != first {
			t.Fatalf("Tokens returned %d and then %d for the same input", first, got)
		}
	}
}

// TestTokensIsSubadditiveOverBlocks is the property the fill depends on: the
// whole document never costs more than the sum of the blocks that were measured
// and paid for. If this were false the cap could be exceeded by a brief every
// piece of which fitted.
func TestTokensIsSubadditiveOverBlocks(t *testing.T) {
	t.Parallel()

	blocks := []string{
		"dira brief — dira · 2026-07-30\n",
		"\nopen blockers\n",
		"  qst-0001  A question that blocks a decision                open 2026-07-29\n" +
			"              blocks dec-0002 — One file per entry\n",
		"\nrecent decisions\n",
		"  dec-0015  The derived cache stays honest by content hash    accepted 2026-07-30\n",
	}

	sum, whole := 0, strings.Builder{}
	for _, b := range blocks {
		sum += brief.Tokens(b)
		whole.WriteString(b)
	}
	if got := brief.Tokens(whole.String()); got > sum {
		t.Errorf("the whole document is %d tokens and the pieces were paid for at %d; "+
			"the fill can exceed the ceiling", got, sum)
	}
}

// TestTokensGrowsWithText. A counter that saturated, or that ignored a
// dimension of the text, would let a brief grow underneath a ceiling that never
// moved.
func TestTokensGrowsWithText(t *testing.T) {
	t.Parallel()

	previous := 0
	line := "  dec-0002  One file per entry, not an append-only JSONL ledger    accepted 2026-07-29\n"
	for n := 1; n <= 20; n++ {
		got := brief.Tokens(strings.Repeat(line, n))
		if got <= previous {
			t.Fatalf("%d copies of a line count %d tokens, no more than the %d for one fewer", n, got, previous)
		}
		previous = got
	}
}

// TestTokensChargesForCharactersAWordCountWouldMiss. The estimator's second
// half exists because a word count is blind to long words, ids and punctuation
// density — the exact shape of a brief.
func TestTokensChargesForCharactersAWordCountWouldMiss(t *testing.T) {
	t.Parallel()

	short := "a b c d e f g h\n"
	long := "internationalisation counterproductively incomprehensibly disproportionately\n"

	if len(strings.Fields(short)) <= len(strings.Fields(long)) {
		t.Fatal("the two samples do not differ in the way this case needs")
	}
	if brief.Tokens(long) <= brief.Tokens(short) {
		t.Errorf("four long words count %d tokens and eight single letters count %d; "+
			"the counter is measuring words rather than text", brief.Tokens(long), brief.Tokens(short))
	}
}

func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "empty"
	}
	if len(s) > 24 {
		s = s[:24]
	}
	return s
}
