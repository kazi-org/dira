package sniff

import (
	"strings"
	"testing"
)

// The two bars, pre-registered.
//
// They were written into this file before the matcher was run against the
// corpus even once, because a threshold chosen after seeing the result is not a
// threshold, it is a description. They are deliberately asymmetric: a decision
// this tier misses is caught by tier 2, by the next turn's Stop hook, or by a
// human typing `dira log` — while a decision it invents costs the human a
// disposition keystroke and, repeated, costs dira the human. int-0001 dies of
// noise, not of gaps.
const (
	// maxFalsePositiveRate is the share of `none` rows that may be staged.
	// One in twenty is the point where a weekly distill queue of ~20 cards
	// carries about one piece of junk — noticeable, and not yet a reason to
	// stop reading.
	maxFalsePositiveRate = 0.05

	// minDetectionRate is the share of `decision` rows that must be caught.
	// Set low on purpose. The tier is allowed to be lossy; dec-0003 says so
	// twice.
	minDetectionRate = 0.60
)

// TestCorpusWellFormed checks that the corpus is a usable grading instrument
// before anything is graded against it.
//
// It never runs the matcher and asserts nothing about detection or false
// positives. That separation is what keeps it honest: the freeze check is the
// reason TestMatchesTheCorpus cannot be made to pass by editing the corpus, so
// the two must not be able to fail for each other's reasons.
func TestCorpusWellFormed(t *testing.T) {
	t.Parallel()

	// Fatally, and first: every assertion below is a statement about a
	// specific set of rows, and none of them means anything if the file is
	// not the one that was frozen.
	t.Run("freeze", func(t *testing.T) { assertCorpusFrozen(t) })

	rows := loadCorpus(t)

	var decisions, nones, nearMisses int
	families := map[string]int{}
	seen := map[string]bool{}

	for _, r := range rows {
		switch {
		case r.ID == "":
			t.Errorf("a row has no id: %+v", r)
		case seen[r.ID]:
			t.Errorf("%s: duplicate row id", r.ID)
		}
		seen[r.ID] = true

		if strings.TrimSpace(r.Text) == "" {
			t.Errorf("%s: text is empty", r.ID)
		}
		if strings.TrimSpace(r.Note) == "" {
			t.Errorf("%s: note is empty — a row with no stated purpose is a row nobody can audit", r.ID)
		}

		switch r.Expect {
		case "decision":
			decisions++
			if r.NearMiss != "" {
				t.Errorf("%s: a decision row carries near_miss, which is a `none`-row field", r.ID)
			}
		case "none":
			nones++
			if r.NearMiss != "" {
				nearMisses++
				families[r.NearMiss]++
			}
		default:
			t.Errorf("%s: expect is %q, want \"decision\" or \"none\"", r.ID, r.Expect)
		}
	}

	t.Run("size", func(t *testing.T) {
		if len(rows) < 60 {
			t.Errorf("corpus holds %d rows, want at least 60", len(rows))
		}
		if decisions < 20 {
			t.Errorf("corpus holds %d decision rows, want at least 20 — a detection rate over fewer is noise", decisions)
		}
		if nones < 2*decisions {
			t.Errorf("corpus holds %d none rows against %d decision rows, want at least twice as many: "+
				"a false-positive rate is only meaningful against the text a session is actually made of, "+
				"and a session is overwhelmingly not decisions", nones, decisions)
		}
	})

	t.Run("near misses", func(t *testing.T) {
		if nearMisses < 30 {
			t.Errorf("the corpus holds %d near-miss rows, want at least 30 — "+
				"without them a matcher that fires on nothing scores a perfect false-positive rate", nearMisses)
		}
		if len(families) < 10 {
			t.Errorf("the near misses cover %d families (%v), want at least 10: "+
				"one family repeated is one mistake tested many times", len(families), families)
		}
	})

	t.Run("rows are one statement", func(t *testing.T) {
		// The matcher grades sentences, so a row holding several would
		// be graded on whichever fired and the label would stop
		// describing what was measured. Two is the allowance and it
		// exists for exactly one row: n50 is a shell line, and the
		// splitter mangling it into two pieces is the thing that row is
		// there to test.
		for _, r := range rows {
			n := len(sentences(strip(r.Text)))
			if n == 0 {
				t.Errorf("%s: splits into no sentences at all: %q", r.ID, r.Text)
			}
			if n > 2 {
				t.Errorf("%s: splits into %d sentences, want 1: %q", r.ID, n, r.Text)
			}
		}
	})
}

// TestMatchesTheCorpus is this lane's real gate, and the number in its log line
// is the lane's deliverable.
//
// Two rates, reported separately, because they fail for different reasons and a
// single score would let one pay for the other. The false-positive rate is over
// the `none` rows and is the one that matters: a sniffer that stages twenty
// candidates a session trains its human to reject reflexively, and a disposition
// queue nobody reads is int-0001's own failure mode wearing dira's syntax.
func TestMatchesTheCorpus(t *testing.T) {
	t.Parallel()
	assertCorpusFrozen(t)

	rows := loadCorpus(t)

	var decisions, detected, nones, falsePositives int
	for _, r := range rows {
		found := Sniff(r.Text)

		switch r.Expect {
		case "decision":
			decisions++
			if len(found) > 0 {
				detected++
				continue
			}
			t.Logf("MISS  %s  %q", r.ID, r.Text)

		case "none":
			nones++
			if len(found) == 0 {
				continue
			}
			falsePositives++
			t.Logf("FALSE POSITIVE  %s  [%s]  %q\n      staged: %q\n      %s",
				r.ID, found[0].Rule, r.Text, found[0].Title, r.Note)
		}
	}

	if decisions == 0 || nones == 0 {
		t.Fatalf("graded %d decision and %d none rows; a run that grades nothing passes vacuously", decisions, nones)
	}

	detection := float64(detected) / float64(decisions)
	fpRate := float64(falsePositives) / float64(nones)
	t.Logf("OBSERVED  detection %.1f%% (%d/%d)  ·  false positives %.1f%% (%d/%d)  ·  bars: detection ≥%.0f%%, false positives ≤%.0f%%",
		detection*100, detected, decisions, fpRate*100, falsePositives, nones,
		minDetectionRate*100, maxFalsePositiveRate*100)

	if fpRate > maxFalsePositiveRate {
		t.Errorf("false-positive rate is %.1f%% (%d of %d), want at most %.0f%%.\n"+
			"The corpus may not be edited to close this gap. Tighten a guard in match.go, or report the "+
			"rate honestly and propose a change to what the tier promises.",
			fpRate*100, falsePositives, nones, maxFalsePositiveRate*100)
	}
	if detection < minDetectionRate {
		t.Errorf("detection rate is %.1f%% (%d of %d), want at least %.0f%%.\n"+
			"The tier is allowed to be lossy, but not this lossy: at this rate the Stop hook is decoration.",
			detection*100, detected, decisions, minDetectionRate*100)
	}
}

// TestEveryGuardSuppressesSomething is the two-sided check on the guard list.
//
// Each case is a sentence that the rule families DO fire on and the guard list
// DOES suppress, so every case fails in both directions: delete the guard and
// the sentence stages, break the rule families and the case stops proving
// anything because it never fired to begin with. Both halves are asserted.
//
// It is a hand-written table rather than a measurement over the corpus, and that
// is the honest shape. Measured over the frozen corpus, only 4 of 54 `none` rows
// fire with every guard removed, and over a real 3,966-sentence session only 6
// extra sentences do — because the rule families are already narrow. A test that
// demanded each guard change one of those numbers would call `question` inert
// while "Are we going with the daemon?" stages, which is a real shape that a
// real session simply did not happen to contain. What is asserted instead is
// reachability: for every guard there exists text it is the thing that stops.
//
// The table must cover every family. A guard added without a case fails here.
func TestEveryGuardSuppressesSomething(t *testing.T) {
	t.Parallel()

	cases := []struct{ family, text string }{
		{"question", "Are we going with the daemon, or the checkpoint file?"},
		{"modal", "Maybe we're going with the daemon instead of the checkpoint file."},
		{"conditional", "If we're going with the daemon, the no-servers promise breaks."},
		{"deferral", "Your call whether we're going with the daemon instead."},
		{"option", "One option is that we're going with the daemon instead."},
		{"recommendation", "I recommend we're not doing a daemon at all."},
		{"second-person", "You decided we're going with the daemon instead."},
		{"citation", "dec-0042 already says we're going with the daemon instead."},
		{"instruction", "Never write that we're going with a daemon instead."},
		{"code", `echo "we're going with the daemon instead" && exit 0`},
		{"tool-output", `$ dira log --title "we're going with the daemon instead"`},
	}

	families := map[string]bool{}
	for _, g := range guardPatterns {
		families[g.name] = false
	}

	for _, tc := range cases {
		t.Run(tc.family, func(t *testing.T) {
			if _, ok := families[tc.family]; !ok {
				t.Fatalf("no guard family named %q exists", tc.family)
			}
			families[tc.family] = true

			// Without guards it stages. If this stops holding, the
			// case below is asserting nothing.
			if _, fired := matchWith(tc.text, nil); !fired {
				t.Fatalf("the rule families do not fire on %q, so suppressing it proves nothing", tc.text)
			}
			// With guards it does not.
			if rule, fired := matchWith(tc.text, guardPatterns); fired {
				t.Fatalf("staged as %s: %q", rule, tc.text)
			}
			// And this family is the one doing it.
			suppressed := false
			for _, g := range guardPatterns {
				if g.name == tc.family && g.re.MatchString(tc.text) {
					suppressed = true
				}
			}
			if !suppressed {
				t.Errorf("something other than the %q guard suppressed %q, so this case does not exercise it",
					tc.family, tc.text)
			}
		})
	}

	for name, covered := range families {
		if !covered {
			t.Errorf("the %q guard family has no case in this table, so nothing shows it can suppress anything", name)
		}
	}
}

// TestGuardsAreMeasuredAgainstTheCorpus reports what the guards are worth on the
// frozen corpus. It is a measurement rather than a bar: see the comment on
// TestEveryGuardSuppressesSomething for why the bar lives there instead.
func TestGuardsAreMeasuredAgainstTheCorpus(t *testing.T) {
	t.Parallel()
	assertCorpusFrozen(t)

	rows := loadCorpus(t)

	var unguarded, guarded int
	for _, r := range rows {
		if r.Expect != "none" {
			continue
		}
		if firesWith(r, nil) {
			unguarded++
		}
		if firesWith(r, guardPatterns) {
			guarded++
		}
	}

	t.Logf("OBSERVED  of %d `none` rows: %d stage with the guards off, %d with them on",
		countExpect(rows, "none"), unguarded, guarded)

	if unguarded < guarded {
		t.Errorf("removing every guard reduced the number of staged `none` rows from %d to %d, "+
			"which means a guard is causing a match rather than suppressing one", guarded, unguarded)
	}
}

// firesWith reports whether a row would be staged under a given guard list. It
// is the seam that lets a guard be graded with the guards switched off, which
// running Sniff cannot do.
func firesWith(r corpusRow, guards []pattern) bool {
	for _, s := range sentences(strip(r.Text)) {
		if _, fired := matchWith(s, guards); fired {
			return true
		}
	}
	return false
}

func countExpect(rows []corpusRow, expect string) int {
	n := 0
	for _, r := range rows {
		if r.Expect == expect {
			n++
		}
	}
	return n
}
