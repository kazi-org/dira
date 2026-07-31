package enforcer

import "github.com/kazi-org/dira/internal/ledger"

// The scoring rule, and the two numbers that make it a verdict.
//
// dec-0014 fixes the shape — idf-weighted content-word overlap, with an exact
// multiword hit as an independent signal, and a match declared when the score
// clears a threshold OR a phrase hits — and explicitly leaves the threshold
// itself to this lane, to be tuned against the frozen corpus in testdata. The
// measured result of that tuning is recorded on the constants below.

// matchThreshold is the overlap score at which a plan is declared to conflict.
//
// Tuned by sweep against internal/enforcer/testdata/corpus.yaml, whose sha256
// freeze is asserted before any of this runs. TestPrecisionRecallCurve prints
// the whole curve, so this number is falsifiable rather than asserted: the
// corpus's 24 conflict rows and 19 compliant near-misses are jointly satisfied
// across a plateau, and this value sits inside it rather than on its edge.
const matchThreshold = 0.38

// phraseShare is how distinctive a shared multiword phrase must be to count as
// a match on its own, as a fraction of the most distinctive a phrase of that
// length could be in this ledger.
//
// A floor is needed because "the same" and "run state" are contiguous word
// pairs too. It is a *share* rather than an absolute idf mass because idf
// scales with the ledger: in a nine-entry fixture the rarest term scores 2.6
// and in a two-hundred-entry ledger it scores 5.6, so a fixed floor that means
// "both words are rare here" in the first means "almost any pair" in the
// second. Tying it to the ledger's own ceiling keeps the rule the same
// statement at every size.
const phraseShare = 0.82

// phraseLength is the shortest run of content words treated as a phrase. Two
// is the minimum that can carry contiguity at all; longer runs are found by the
// same rule, since a shared trigram contains two shared bigrams.
const phraseLength = 2

// A matcher holds the ledger-wide statistics a score depends on, and the two
// numbers that turn a score into a verdict.
//
// The thresholds are fields rather than references to the constants so that
// TestPrecisionRecallCurve can sweep them and print what this matcher can and
// cannot do. They are set from the constants by newMatcher and by nothing else:
// no flag, no environment variable and no ledger config reaches them, because a
// check whose strictness a repository could configure is a check that can be
// configured into agreeing with you.
type matcher struct {
	weights *idf
	units   []unit

	threshold   float64
	phraseFloor float64
}

// newMatcher reads the enforcement set out of entries and measures the ledger.
func newMatcher(entries []*ledger.Entry) *matcher {
	m := &matcher{
		weights:   newIDF(),
		units:     enforcementSet(entries),
		threshold: matchThreshold,
	}
	for _, e := range entries {
		m.weights.observe(documentText(e))
	}
	m.phraseFloor = phraseShare * phraseLength * m.weights.rarest()
	return m
}

// hit is one unit's score against a plan.
type hit struct {
	score  float64
	phrase bool

	// scores is false for a unit whose overlap is advisory only — a why_not,
	// which can be cited on an exact phrase but never on scattered shared
	// vocabulary. The score is still computed and still reported, because it
	// is what the citation's Score carries.
	scores bool
}

// matched reports whether a hit is a conflict under dec-0014's rule: the score
// clears the threshold, or an exact multiword phrase landed.
func (m *matcher) matched(h hit) bool {
	return (h.scores && h.score >= m.threshold) || h.phrase
}

// score compares a plan against one unit.
//
// The measure is an overlap coefficient — shared idf mass over the *smaller*
// of the two sides — rather than the more obvious "how much of the target did
// the plan cover". Coverage alone cannot serve both target shapes this check
// has: an alternative is three words, so coverage is exactly right for it,
// while a constraint's body sentence is thirty, and a four-word plan matching
// its two distinctive words would score 0.1 and slip through. Dividing by the
// smaller side asks the question that fits whichever side is short: how much of
// this alternative does the plan propose, or how much of this plan is about
// this constraint.
func (m *matcher) score(plan Text, u unit) hit {
	var planBag, unitBag map[term]bool
	if u.polarised {
		planBag, unitBag = plan.Bag(true), u.text.Bag(true)
	} else {
		planBag, unitBag = plan.PositiveBag(), u.text.Bag(false)
	}

	var shared float64
	for tm := range unitBag {
		if planBag[tm] {
			shared += m.weights.weight(tm.word)
		}
	}
	if shared == 0 {
		return hit{}
	}

	unitMass, planMass := m.weights.mass(unitBag), m.weights.mass(planBag)
	denominator := unitMass
	if planMass < denominator {
		denominator = planMass
	}
	if denominator <= 0 {
		return hit{}
	}

	h := hit{score: shared / denominator, scores: !u.phraseOnly}
	if u.prose {
		h.phrase = m.sharesPhrase(plan, u)
	} else {
		h.phrase = m.restates(plan, u)
	}
	return h
}

// restates reports whether the plan says a whole distinctive stretch of the
// unit back, contiguously and with the same polarity.
//
// This is dec-0014's exact-multiword-phrase rule, read as it has to be read to
// work. Taken as raw strings it inverts on the two rows that matter most:
// dec-0014 offers "add a background daemon to track run state" as the case its
// phrase rule catches trivially against the alternative "a daemon", and the
// literal substring "a daemon" is not in that sentence at all — `background`
// sits between the words. It *is* in corpus row-039, "a daemon was considered
// and rejected", which must not conflict. Comparing content words instead of
// characters fixes both: `daemon` matches through the intervening adjective,
// and row-039's is disclaimed so its polarity differs.
//
// The run has to be distinctive enough to mean something on its own, by the
// same floor a shared phrase uses, and it has to be a *whole* uniform-polarity
// run rather than any fragment of one. That second condition is what separates
// corpus row-010, which proposes dec-0042's "a compacted event log", from
// row-030, which keeps an in-memory event log for --verbose output: both
// contain "event log", and only one contains "compacted event log".
func (m *matcher) restates(plan Text, u unit) bool {
	for _, sp := range u.text.spans {
		for _, run := range uniformRuns(sp) {
			var mass float64
			for _, tm := range run {
				mass += m.weights.weight(tm.word)
			}
			if mass < m.phraseFloor {
				continue
			}
			if containsRun(plan, run, u.polarised) {
				return true
			}
		}
	}
	return false
}

// uniformRuns cuts a span into its maximal stretches of one polarity.
func uniformRuns(sp span) []span {
	var runs []span
	start := 0
	for i := 1; i <= len(sp); i++ {
		if i < len(sp) && sp[i].negated == sp[start].negated {
			continue
		}
		runs = append(runs, sp[start:i])
		start = i
	}
	return runs
}

// containsRun reports whether run appears as a contiguous stretch of some span
// of plan.
//
// polarised selects the comparison: an alternative is a proposal, so the plan
// must agree with it about what is and is not being proposed, while a
// constraint is a prohibition and only the plan's own polarity matters — a plan
// conflicts with it by asserting the thing, never by disclaiming it.
func containsRun(plan Text, run span, polarised bool) bool {
	for _, sp := range plan.spans {
		for i := 0; i+len(run) <= len(sp); i++ {
			ok := true
			for j, want := range run {
				got := sp[i+j]
				if got.word != want.word {
					ok = false
					break
				}
				if polarised && got.negated != want.negated {
					ok = false
					break
				}
				if !polarised && got.negated {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
	}
	return false
}

// sharesPhrase reports whether the plan and the unit use the same distinctive
// words adjacent to each other.
//
// It is the weaker of the two multiword rules and applies only to argument
// prose, where restates could never fire: nobody restates a why_not's whole
// sentence, and the evidence a why_not can offer is that it named the same
// mechanism. dec-0082's first why_not says "phoning home by default", and a
// plan saying "dira should phone home" is proposing exactly that (corpus
// row-021), while sharing no distinctive vocabulary with the alternative's own
// text.
func (m *matcher) sharesPhrase(plan Text, u unit) bool {
	unitPhrases := u.text.Phrases(phraseLength, u.polarised)
	if len(unitPhrases) == 0 {
		return false
	}
	var planPhrases map[string]bool
	if u.polarised {
		planPhrases = plan.Phrases(phraseLength, true)
	} else {
		planPhrases = positivePhrases(plan, phraseLength)
	}

	for phrase := range unitPhrases {
		if !planPhrases[phrase] {
			continue
		}
		var mass float64
		for _, word := range splitPhrase(phrase) {
			mass += m.weights.weight(word)
		}
		if mass >= m.phraseFloor {
			return true
		}
	}
	return false
}

// positivePhrases is Phrases restricted to what the plan asserts. A plan that
// writes "no hosted sync service" has the phrase and is disclaiming it.
func positivePhrases(t Text, n int) map[string]bool {
	out := make(map[string]bool)
	for phrase := range t.Phrases(n, true) {
		if len(phrase) > 0 && phrase[0] == '!' {
			continue
		}
		out[phrase] = true
	}
	return out
}

func splitPhrase(phrase string) []string {
	words := []string{}
	start := 0
	for i := 0; i <= len(phrase); i++ {
		if i == len(phrase) || phrase[i] == ' ' {
			if w := phrase[start:i]; w != "" && w != "!" {
				words = append(words, trimBang(w))
			}
			start = i + 1
		}
	}
	return words
}

func trimBang(w string) string {
	if len(w) > 0 && w[0] == '!' {
		return w[1:]
	}
	return w
}
