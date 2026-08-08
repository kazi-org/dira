package enforcer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

// wantRecall is the bar E3-L2's acceptance line sets: at least 90% of the
// corpus's expected-conflict rows detected with the correct entry id cited, and
// zero compliant rows flagged.
const wantRecall = 0.90

// TestMatchesTheCorpus is the lane's real gate.
//
// It grades the matcher against the 43 rows E3-L1 froze before any matcher
// existed. The freeze assertion runs first, so the one cheat available here —
// deleting a row the matcher fails — fails the test rather than passing it.
//
// Two separate numbers, because they fail for different reasons and a single
// F-score would let one pay for the other: recall is over the 24 conflict rows
// and asks whether the correct entry was cited, and the false-positive count is
// over the 19 compliant near-misses and asks only whether anything was flagged
// at all. The false-positive bar is zero, not "low": int-0001 dies mechanically
// the first time this check cries wolf, because a developer who has been
// wrongly blocked once switches it off.
func TestMatchesTheCorpus(t *testing.T) {
	t.Parallel()
	assertCorpusFrozen(t)

	entries := fixtureEntries(t, daemonLedger)
	rows := loadCorpus(t)

	var conflicts, detected, compliant, falsePositives int
	for _, r := range rows {
		v := check(r.Plan, entries)

		switch r.Expect {
		case "conflict":
			conflicts++
			if citesEntry(v, r.Entry) {
				detected++
				continue
			}
			t.Logf("MISS  %s  %q\n      want a citation of %s, got %s",
				r.ID, r.Plan, r.Entry, describe(v))

		case "compliant":
			compliant++
			if v.Compliant() {
				continue
			}
			falsePositives++
			t.Errorf("FALSE POSITIVE  %s  %q\n      near-miss of %s, flagged %s\n      %s",
				r.ID, r.Plan, r.NearMissOf, describe(v), r.Note)
		}
	}

	if conflicts == 0 || compliant == 0 {
		t.Fatalf("the corpus graded %d conflict and %d compliant rows; a run that grades nothing passes vacuously",
			conflicts, compliant)
	}

	recall := float64(detected) / float64(conflicts)
	t.Logf("recall %.1f%% (%d/%d conflict rows cited correctly) · false positives %d/%d compliant rows · threshold %.2f · phrase share %.2f",
		recall*100, detected, conflicts, falsePositives, compliant, matchThreshold, phraseShare)

	if recall < wantRecall {
		t.Errorf("recall is %.1f%% (%d of %d), want at least %.0f%%.\n"+
			"The corpus may not be edited to close this gap (docs/plan/lanes/E3.md). "+
			"If the bar is unreachable, report the curve TestPrecisionRecallCurve prints and propose a contract change.",
			recall*100, detected, conflicts, wantRecall*100)
	}
	if falsePositives != 0 {
		t.Errorf("%d of %d compliant rows were flagged, want 0", falsePositives, compliant)
	}
}

// TestTheStagedDecisionIsCitedByNothing is the enforcement set's sharpest edge,
// tested on its own because it is the one row of the table that fails silently.
//
// dec-0075 is `state: staged`: a regex-tier guess from a Stop hook that no human
// has confirmed (dec-0003). The fixture includes a corpus row whose plan text
// matches it almost word for word, so a matcher that forgot to filter by state
// would cite it with a very high score and look, from the outside, like it was
// working especially well.
func TestTheStagedDecisionIsCitedByNothing(t *testing.T) {
	t.Parallel()
	assertCorpusFrozen(t)

	entries := fixtureEntries(t, daemonLedger)
	staged := map[string]bool{}
	for _, e := range entries {
		if e.State == ledger.StateStaged || e.Kind == ledger.KindIntent ||
			e.Kind == ledger.KindQuestion || e.Kind == ledger.KindNote {
			staged[e.ID] = true
		}
	}
	if !staged["dec-0075"] || !staged["int-0002"] || !staged["qst-0007"] {
		t.Fatalf("the fixture ledger no longer holds the staged decision and the never-enforced kinds it is meant to exercise")
	}

	for _, r := range loadCorpus(t) {
		v := check(r.Plan, entries)
		for _, c := range v.Conflicts {
			if staged[c.Entry] {
				t.Errorf("%s (%q) cited %s, which is %s/%s and is never enforcement substrate",
					r.ID, r.Plan, c.Entry, c.Kind, c.State)
			}
		}
	}
}

// TestPrecisionRecallCurve reports what the matcher can and cannot do across
// the whole threshold range, rather than at the one value that shipped.
//
// This lane's stated risk is that zero false positives at 90% recall might not
// be jointly reachable by lexical matching at all, and that the honest response
// to that would be a curve and a proposal rather than a quietly edited corpus.
// This is that curve. It runs on every `go test ./internal/enforcer`, so the
// claim that the shipped threshold sits on a plateau is re-measured rather than
// remembered.
func TestPrecisionRecallCurve(t *testing.T) {
	t.Parallel()
	assertCorpusFrozen(t)

	entries := fixtureEntries(t, daemonLedger)
	rows := loadCorpus(t)

	var report strings.Builder
	report.WriteString("\nthreshold  recall   detected  false positives\n")

	viable := []float64{}
	for step := 20; step <= 80; step += 2 {
		threshold := float64(step) / 100
		detected, conflicts, falsePositives := 0, 0, 0
		for _, r := range rows {
			v := checkAt(r.Plan, entries, threshold)
			switch r.Expect {
			case "conflict":
				conflicts++
				if citesEntry(v, r.Entry) {
					detected++
				}
			case "compliant":
				if !v.Compliant() {
					falsePositives++
				}
			}
		}
		recall := float64(detected) / float64(conflicts)
		fmt.Fprintf(&report, "   %.2f   %5.1f%%   %2d/%-2d       %d\n",
			threshold, recall*100, detected, conflicts, falsePositives)
		if falsePositives == 0 && recall >= wantRecall {
			viable = append(viable, threshold)
		}
	}
	t.Log(report.String())

	if len(viable) == 0 {
		t.Fatalf("no threshold in [0.20, 0.80] reaches %.0f%% recall with zero false positives.\n"+
			"That is this lane's stated risk arriving. The corpus may not be edited: report the curve above "+
			"and propose either a different threshold target or a change to the matching contract "+
			"(.dira/entries/dec-0014.md), and supersede it in writing.", wantRecall*100)
	}

	t.Logf("thresholds meeting the bar: %v", viable)
	if matchThreshold < viable[0] || matchThreshold > viable[len(viable)-1] {
		t.Errorf("the shipped threshold %.2f is outside the viable range %v", matchThreshold, viable)
	}
	if len(viable) == 1 {
		t.Errorf("only one threshold in the sweep meets the bar (%v). A single viable point is a fit to this "+
			"corpus rather than a working matcher; widen the signal rather than pinning the number.", viable)
	}
}

// checkAt is check with the threshold overridden, for the sweep only. Nothing
// outside a test can move it: the field exists on the matcher rather than as a
// package variable so that no flag, no environment variable and no ledger
// config can talk the check into a different verdict.
func checkAt(plan string, entries []*ledger.Entry, threshold float64) *Verdict {
	m := newMatcher(entries)
	m.threshold = threshold
	return m.verdict(plan)
}

func citesEntry(v *Verdict, id string) bool {
	for _, c := range v.Conflicts {
		if c.Entry == id {
			return true
		}
	}
	return false
}

func describe(v *Verdict) string {
	if v.Compliant() {
		return "no conflict"
	}
	parts := make([]string, 0, len(v.Conflicts))
	for _, c := range v.Conflicts {
		parts = append(parts, fmt.Sprintf("%s@%.2f", c.Entry, c.Score))
	}
	return strings.Join(parts, " ")
}
