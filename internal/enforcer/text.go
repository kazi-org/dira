package enforcer

import (
	"math"
	"strings"
)

// This file is the whole linguistic surface of `dira check`. It is deliberately
// small, deliberately stupid, and deliberately offline: dec-0014 fixes lexical
// matching computed fresh in the binary as the mechanism, and dec-0003 and
// cst-0004 forbid the alternative that would make it cleverer. Nothing here
// downloads, embeds or trains anything.
//
// Three ideas carry all of it:
//
//  1. A text is a sequence of *spans* — clause-sized runs of words, cut at
//     punctuation and at the coordinators that end a clause. Spans exist so
//     that a negation scopes to a clause instead of to a sentence.
//  2. Every content word carries the polarity of the span position it was
//     written at. "no cache at all" and "cache nothing" both produce a negated
//     `cache`, and "cache query results" produces a positive one. That
//     distinction is the difference between a plan that proposes the thing a
//     decision rejected and one that disclaims it, and it is not recoverable
//     from a bag of words.
//  3. Words are normalised by a small suffix stripper, not a real stemmer.
//     `running`/`runs`/`run` must agree or a paraphrase stops matching; nothing
//     here needs `usage` and `use` to agree.

// A term is one normalised content word together with the polarity of the
// clause position it appeared in. Two texts share a term only when both the
// word and the polarity agree, except where the caller asks otherwise (see
// Text.Bag).
type term struct {
	word    string
	negated bool
}

// A span is one clause's worth of terms, in order. Order is kept because a
// contiguous multiword hit is an independent match signal under dec-0014, and a
// bag cannot express contiguity.
type span []term

// A Text is a normalised, span-segmented piece of prose: a candidate plan, a
// rejected alternative's option, a constraint's body sentence.
type Text struct {
	spans []span

	// raw is what was parsed, kept for diagnostics only. Nothing matches
	// against it.
	raw string
}

// Parse normalises prose into a Text.
func Parse(s string) Text {
	t := Text{raw: s}
	for _, chunk := range splitSpans(s) {
		if sp := toSpan(chunk); len(sp) > 0 {
			t.spans = append(t.spans, sp)
		}
	}
	return t
}

// Empty reports whether the text carries no content words at all.
func (t Text) Empty() bool { return len(t.spans) == 0 }

// Bag returns the text's distinct terms.
//
// polarised selects the comparison rule. With it set, `cache` and `not cache`
// are different terms, which is what makes a plan that disclaims a rejected
// alternative stop matching it. With it clear, polarity is discarded and only
// the plan side's polarity is consulted by the caller — the rule constraints
// need, because a constraint is a prohibition rather than a proposal: cst-0004
// says dira "never requires ... an account", and a plan conflicts with it by
// proposing an account, not by also saying "never".
func (t Text) Bag(polarised bool) map[term]bool {
	out := make(map[term]bool)
	for _, sp := range t.spans {
		for _, tm := range sp {
			if !polarised {
				tm.negated = false
			}
			out[tm] = true
		}
	}
	return out
}

// PositiveBag returns the terms the text asserts rather than disclaims, with
// polarity stripped. It is the plan side of a constraint comparison.
func (t Text) PositiveBag() map[term]bool {
	out := make(map[term]bool)
	for _, sp := range t.spans {
		for _, tm := range sp {
			if tm.negated {
				continue
			}
			out[term{word: tm.word}] = true
		}
	}
	return out
}

// Phrases returns every contiguous run of n content words inside a single span
// whose members share a polarity, keyed by the joined words.
//
// dec-0014 makes an exact multiword hit an independent match signal, on the
// reasoning that two texts using the same two distinctive words *adjacent to
// each other* are talking about the same thing even when the surrounding
// vocabulary diverges. Runs are taken within a span so that a phrase never
// straddles a clause boundary, where adjacency means nothing.
func (t Text) Phrases(n int, polarised bool) map[string]bool {
	out := make(map[string]bool)
	if n < 2 {
		return out
	}
	for _, sp := range t.spans {
		for i := 0; i+n <= len(sp); i++ {
			window := sp[i : i+n]
			polarity := window[0].negated
			words := make([]string, 0, n)
			uniform := true
			for _, tm := range window {
				if tm.negated != polarity {
					uniform = false
					break
				}
				words = append(words, tm.word)
			}
			if !uniform {
				continue
			}
			key := strings.Join(words, " ")
			if polarised && polarity {
				key = "!" + key
			}
			out[key] = true
		}
	}
	return out
}

// Words returns every distinct normalised word in the text, polarity and all
// positions discarded. It is what document frequency is counted over.
func (t Text) Words() map[string]bool {
	out := make(map[string]bool)
	for _, sp := range t.spans {
		for _, tm := range sp {
			out[tm.word] = true
		}
	}
	return out
}

// spanBreakers are the words that end a clause. A negation reaches the end of
// its clause and no further: "skip the cache entirely and reparse every entry
// file" proposes the reparse, and a negation that ran to the end of the
// sentence would read it as proposing neither.
//
// `or` is not here, and its absence is load-bearing. It nearly always joins a
// list under one operator rather than starting a new clause — "never creates a
// user account or hosted tier", "no network service, an account, or a hosted
// tier" — so breaking on it strands the second half of the list outside the
// negation that governs it, and a plan promising *not* to add a hosted tier
// gets read as proposing one. Corpus row-041 is exactly that sentence.
var spanBreakers = map[string]bool{
	"and": true, "but": true, "so": true, "then": true,
	"while": true, "which": true, "that": true, "though": true,
	"although": true, "because": true, "when": true, "whereas": true,
	"plus": true, "yet": true, "however": true, "whether": true,
}

// negationCues flip the polarity of the rest of their clause — or, when they
// close the clause, of what came before them, because "cache nothing" and "no
// cache" are the same proposal written from opposite ends.
//
// The second half of this list is not grammatical negation. "a daemon was
// considered and rejected" contains no negative particle at all, and yet the
// sentence is the clearest possible statement that a daemon is *not* being
// proposed — it is someone writing down that the decision already exists, which
// is the single most common way a real plan mentions a rejected option without
// relitigating it. Treating those verbs as disclaimers is what separates
// corpus row-039 from row-001; without them a substring matcher fires on the
// document *about* the decision.
var negationCues = map[string]bool{
	"no": true, "not": true, "never": true, "none": true, "nothing": true,
	"without": true, "cannot": true, "cant": true, "dont": true,
	"doesnt": true, "isnt": true, "arent": true, "wont": true, "nor": true,
	"neither": true, "non": true, "instead": true, "rather": true,

	"reject": true, "rejects": true, "rejected": true, "forbid": true,
	"forbids": true, "forbidden": true, "prohibit": true, "prohibits": true,
	"avoid": true, "avoids": true, "refuse": true, "refuses": true,
	"declined": true, "exclude": true, "excludes": true, "skip": true,
	"skips": true, "disclaim": true, "disclaims": true, "ruled": true,
}

// stopwords are dropped after polarity is resolved, so a cue can scope a
// clause and still contribute nothing to the match.
var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "to": true, "in": true,
	"on": true, "at": true, "by": true, "for": true, "from": true,
	"with": true, "as": true, "is": true, "are": true, "was": true,
	"were": true, "be": true, "been": true, "being": true, "it": true,
	"its": true, "this": true, "these": true, "those": true, "we": true,
	"you": true, "your": true, "our": true, "us": true, "they": true,
	"them": true, "their": true, "he": true, "she": true, "i": true,
	"my": true, "me": true, "will": true, "would": true, "can": true,
	"could": true, "should": true, "shall": true, "may": true,
	"might": true, "must": true, "do": true, "does": true, "did": true,
	"done": true, "have": true, "has": true, "had": true, "if": true,
	"else": true, "than": true, "there": true, "here": true, "what": true,
	"who": true, "whom": true, "whose": true, "how": true, "why": true,
	"all": true, "any": true, "some": true, "each": true, "both": true,
	"more": true, "most": true, "other": true, "another": true,
	"such": true, "only": true, "own": true, "same": true, "too": true,
	"very": true, "just": true, "into": true, "over": true, "under": true,
	"out": true, "up": true, "down": true, "about": true, "after": true,
	"before": true, "between": true, "through": true, "during": true,
	"per": true, "via": true, "let": true, "lets": true, "get": true,
	"gets": true, "one": true, "two": true, "also": true, "still": true,
	"already": true, "ever": true, "even": true, "much": true, "many": true,
}

// splitSpans cuts prose at punctuation. Word-level breakers are applied later,
// inside toSpan, because they have to be recognised after case folding.
func splitSpans(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case ',', ';', ':', '.', '!', '?', '\n', '(', ')', '[', ']', '{', '}', '"', '—', '–':
			return true
		}
		return false
	})
}

// toSpan turns one punctuation-delimited chunk into terms, applying the
// word-level clause breaks and the negation scoping.
func toSpan(chunk string) span {
	words := wordsOf(chunk)

	// Word-level breakers split the chunk further. Each sub-clause gets its
	// own negation scope, and the results are concatenated: a Text's spans
	// are a flat list, so a chunk that breaks into three clauses simply
	// contributes three of them.
	var out span
	start := 0
	flush := func(end int) {
		if end > start {
			out = append(out, scopeNegation(words[start:end])...)
		}
		start = end + 1
	}
	for i, w := range words {
		if !spanBreakers[w] {
			continue
		}
		// A breaker immediately followed by a negation cue is continuing
		// the clause, not ending it: "a daemon was considered and
		// rejected" is one statement, and cutting it at `and` leaves
		// `daemon` asserted in a sentence whose whole point is that it is
		// not. Corpus row-039 is that sentence, written the way a person
		// actually writes it in a plan.
		if i+1 < len(words) && negationCues[words[i+1]] {
			continue
		}
		flush(i)
	}
	flush(len(words))
	return out
}

// scopeNegation resolves one clause's polarity and drops its stopwords.
//
// A cue negates what follows it. A cue with nothing but cues after it negates
// what precedes it instead, which is the "cache nothing" shape. Both are the
// same rule seen from either end of the clause.
func scopeNegation(words []string) span {
	firstCue, lastContent := -1, -1
	for i, w := range words {
		if negationCues[w] {
			if firstCue < 0 {
				firstCue = i
			}
			continue
		}
		lastContent = i
	}

	negateFrom, negateBefore := len(words), -1
	switch {
	case firstCue < 0:
		// No cue: the clause is asserted as written.
	case firstCue > lastContent:
		// Every content word comes before the cue: "cache nothing".
		negateBefore = firstCue
	default:
		negateFrom = firstCue
	}

	var out span
	for i, w := range words {
		if negationCues[w] || stopwords[w] {
			continue
		}
		root := stem(w)
		if root == "" {
			continue
		}
		out = append(out, term{word: root, negated: i > negateFrom || i < negateBefore})
	}
	return out
}

// wordsOf lowercases a chunk and splits it into bare words. Apostrophes are
// dropped rather than split on, so "don't" survives as the cue "dont" instead
// of decomposing into a stopword and a fragment.
func wordsOf(chunk string) []string {
	var b strings.Builder
	b.Grow(len(chunk))
	for _, r := range strings.ToLower(chunk) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '\'' || r == '’':
			// Elided: don't -> dont.
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Fields(b.String())
}

// stem is a deliberately small suffix stripper.
//
// It is not a linguistic stemmer and does not try to be. Its whole job is that
// a paraphrase using `running`, `runs` or `run` matches an alternative that
// used another of them, because the corpus's conflict rows are paraphrases by
// construction and an exact-word matcher would miss most of them. Every rule
// here is one that pays for itself on that corpus; there is no rule for
// `-ation`, `-ise` or `-er`, because collapsing `user` to `us` or `mutation` to
// `mutate` costs precision and buys nothing.
//
// A word of three characters or fewer is left alone: `run`, `log`, `ui`, `db`.
func stem(w string) string {
	if len(w) <= 3 {
		return w
	}

	switch {
	case strings.HasSuffix(w, "sses"):
		w = w[:len(w)-2]
	case strings.HasSuffix(w, "ies") && len(w) > 4:
		w = w[:len(w)-3] + "y"
	case strings.HasSuffix(w, "ss"), strings.HasSuffix(w, "us"), strings.HasSuffix(w, "is"):
		// Not a plural: process, anonymous, basis.
	case strings.HasSuffix(w, "shes"), strings.HasSuffix(w, "ches"),
		strings.HasSuffix(w, "xes"), strings.HasSuffix(w, "zes"),
		strings.HasSuffix(w, "ses"):
		w = w[:len(w)-2]
	case strings.HasSuffix(w, "s"):
		w = w[:len(w)-1]
	}

	if len(w) >= 5 {
		switch {
		case strings.HasSuffix(w, "ence"), strings.HasSuffix(w, "ance"):
			w = w[:len(w)-4]
		case strings.HasSuffix(w, "ing"):
			w = undouble(w[:len(w)-3])
		case strings.HasSuffix(w, "edly"):
			w = undouble(w[:len(w)-4])
		case strings.HasSuffix(w, "ed"):
			w = undouble(w[:len(w)-2])
		case strings.HasSuffix(w, "ly"):
			w = w[:len(w)-2]
		}
	}

	// A trailing `e` is dropped so that `require`, `requires` and `required`
	// converge: the -ed rule above leaves `requir`, and without this the bare
	// form would not join it. Gated at five characters so `state` and `file`
	// keep their shape.
	if len(w) >= 5 && strings.HasSuffix(w, "e") {
		w = w[:len(w)-1]
	}
	return w
}

// undouble collapses the doubled final consonant an -ing or -ed suffix leaves
// behind: running -> runn -> run, snapshotting -> snapshott -> snapshot.
func undouble(w string) string {
	n := len(w)
	if n < 3 || w[n-1] != w[n-2] {
		return w
	}
	switch w[n-1] {
	case 'a', 'e', 'i', 'o', 'u', 's', 'l':
		return w
	}
	return w[:n-1]
}

// idf is inverse document frequency over the ledger being checked.
//
// dec-0014 is explicit that document frequency is measured across the entries
// in the ledger under test at check time, and not against a fixed external
// corpus: a shipped idf table would be an artifact to keep in sync, and dira
// ships one binary and no data files (int-0002). The consequence is that
// distinctiveness is relative to *this* ledger — `daemon` is unremarkable in a
// ledger about daemons — which is the behaviour a relitigation firewall wants.
//
// Smoothed both sides so a term in every entry still scores above zero: a
// vocabulary that is common in this ledger is weak evidence, not no evidence.
type idf struct {
	docs int
	df   map[string]int
}

func newIDF() *idf { return &idf{df: map[string]int{}} }

// observe counts one document.
func (f *idf) observe(t Text) {
	f.docs++
	for w := range t.Words() {
		f.df[w]++
	}
}

// weight returns a term's inverse document frequency.
func (f *idf) weight(word string) float64 {
	return math.Log(float64(f.docs+1)/float64(f.df[word]+1)) + 1
}

// rarest is the weight of a term appearing in exactly one entry — the most
// distinctive a word can be in this ledger, and the ceiling the phrase floor is
// expressed as a share of. A term absent from the ledger entirely scores higher
// still, but no such term can appear in a unit, and only units set the floor.
func (f *idf) rarest() float64 {
	return math.Log(float64(f.docs+1)/2) + 1
}

// mass sums the weights of a bag of terms.
func (f *idf) mass(bag map[term]bool) float64 {
	var total float64
	for tm := range bag {
		total += f.weight(tm.word)
	}
	return total
}
