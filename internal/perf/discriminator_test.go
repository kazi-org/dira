//go:build perf

package perf

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// degradedRunnerIncident is the run the founder decision (recorded 2026-08-11,
// dec-0029) named as the reason the CI perf gate needed a minimum-discriminator:
// a real 20-sample cold measurement came back min 26.8ms, median 144ms, max
// 897ms -- a 33x spread inside one run -- against coldMedianBudget's 100ms
// ceiling, failed the build, then passed clean on re-run. The minimum was
// nearly four times under the ceiling, so the code demonstrably could meet it;
// the median that broke the build was a statement about the runner.
//
// Fabricated exactly rather than waited for, per L-0001: a gate is evidence
// only once its red and green sides have both been watched, and this
// incident's own numbers are that proof instead of a hope it recurs by chance.
var degradedRunnerIncident = Timing{
	N:      20,
	Min:    26800 * time.Microsecond,
	Median: 144 * time.Millisecond,
	Max:    897 * time.Millisecond,
}

// fakeJudger records what judgeMedian called on it, without any of it
// propagating to this test binary's own pass/fail state the way a real
// *testing.T's Fail() would. Proving "judgeMedian correctly fails a broken
// budget" needs a real Error() call to have happened; it must not need this
// package's own test suite to go red to show it.
type fakeJudger struct {
	failed, skipped bool
	message         string
}

func (f *fakeJudger) Helper() {}
func (f *fakeJudger) Error(args ...any) {
	f.failed = true
	f.message = fmt.Sprint(args...)
}
func (f *fakeJudger) Skip(args ...any) {
	f.skipped = true
	f.message = fmt.Sprint(args...)
}

// Red (of the skip kind): a degraded runner must be SKIPPED, with the numbers
// that disqualified it printed, and must NOT be recorded as a build failure.
func TestMinimumDiscriminatorSkipsADegradedRunner(t *testing.T) {
	verdict, msg := judgeBudgetVerdict(degradedRunnerIncident, "cold", "coldMedianBudget", coldMedianBudget)
	if verdict != budgetNotMeasurable {
		t.Fatalf("the degraded-runner incident (min %v, median %v against a %v ceiling) verdicted %v, want not measurable",
			degradedRunnerIncident.Min, degradedRunnerIncident.Median, coldMedianBudget, verdict)
	}
	for _, want := range []string{"NOT MEASURABLE", "26.8ms", "144ms", "100ms"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the skip message does not contain %q, so the skip is not auditable against the numbers "+
				"that disqualified the machine:\n%s", want, msg)
		}
	}

	fake := &fakeJudger{}
	judgeMedian(fake, degradedRunnerIncident, "cold", "coldMedianBudget", coldMedianBudget)
	if !fake.skipped {
		t.Error("judgeMedian did not skip a distribution whose minimum is comfortably inside the ceiling")
	}
	if fake.failed {
		t.Error("judgeMedian FAILED a degraded-runner distribution instead of skipping it -- a check " +
			"announcing a verdict this machine never reached is the defect docs/lore.md keeps recording")
	}
	if !strings.Contains(fake.message, "NOT MEASURABLE") {
		t.Errorf("the message judgeMedian handed to Skip does not say NOT MEASURABLE:\n%s", fake.message)
	}
}

// Green (of the fail kind): a genuine regression on a HEALTHY minimum must
// still fail the build. If the discriminator loosened this case it would be
// exactly the substitution dec-0029's second alternative rejects -- a ceiling
// read off the minimum instead of the median.
func TestMinimumDiscriminatorStillFailsAHealthyOverage(t *testing.T) {
	healthyButOver := Timing{N: 20, Min: 108 * time.Millisecond, Median: 112 * time.Millisecond, Max: 130 * time.Millisecond}

	verdict, msg := judgeBudgetVerdict(healthyButOver, "cold", "coldMedianBudget", coldMedianBudget)
	if verdict != budgetBroken {
		t.Fatalf("a distribution whose BEST sample (%v) is over the %v ceiling verdicted %v, want broken",
			healthyButOver.Min, coldMedianBudget, verdict)
	}
	if strings.Contains(msg, "NOT MEASURABLE") {
		t.Errorf("a genuine regression's message reads as a skip: %s", msg)
	}

	fake := &fakeJudger{}
	judgeMedian(fake, healthyButOver, "cold", "coldMedianBudget", coldMedianBudget)
	if !fake.failed {
		t.Error("judgeMedian did not fail a distribution whose minimum itself is over the ceiling")
	}
	if fake.skipped {
		t.Error("judgeMedian skipped a genuine regression instead of failing it")
	}
}

// The untouched correct case: a distribution comfortably inside the ceiling
// must neither fail nor skip. L-0001 rule 2 -- a check that cannot pass the
// correct case is invisible from the red side alone.
func TestMinimumDiscriminatorPassesAHealthyRun(t *testing.T) {
	healthy := Timing{N: 20, Min: 38 * time.Millisecond, Median: 41 * time.Millisecond, Max: 55 * time.Millisecond}

	verdict, _ := judgeBudgetVerdict(healthy, "cold", "coldMedianBudget", coldMedianBudget)
	if verdict != budgetMet {
		t.Fatalf("a distribution well inside the ceiling verdicted %v, want met", verdict)
	}

	fake := &fakeJudger{}
	judgeMedian(fake, healthy, "cold", "coldMedianBudget", coldMedianBudget)
	if fake.skipped || fake.failed {
		t.Errorf("judgeMedian flagged a healthy distribution: skipped=%v failed=%v", fake.skipped, fake.failed)
	}
}
