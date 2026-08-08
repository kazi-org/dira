//go:build perf

package perf

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/ledger/fixture"
)

// TestReindexBudget is `dira reindex` over the 200-entry fixture with the cache
// thrown away before every run: every entry file read, parsed and written into a
// database created from nothing.
//
// It is the most work dira ever does in one invocation, and deliberately the
// loosest budget in the lane. Nothing on the hook path runs it — a user types it
// when they want the cache rebuilt — so reindexBudget buys regression detection
// rather than latency, and budget.go states that reasoning beside the number
// along with the measurement it was derived from.
//
// The measurement asserts the MEDIAN and nothing else, per E1-L6-T2's acceptance
// line. The minimum is reported beside it as the uncontended estimate and is
// asserted against by nothing: min <= median by construction, so a ceiling read
// off the minimum fires strictly less often than the one this lane wrote.
func TestReindexBudget(t *testing.T) {
	if reason := SkipReason(testing.Short()); reason != "" {
		t.Skip(reason)
	}

	h, err := NewHarness(t.TempDir(), fixture.Size)
	if err != nil {
		if errors.Is(err, ErrNoToolchain) {
			t.Skip(err)
		}
		t.Fatal(err)
	}

	var lastOut string
	cold, err := Measure(samples,
		func() error {
			// Setup, outside the timer: removing three files is not
			// the work reindex is being budgeted for.
			if err := h.RemoveCache(); err != nil {
				return err
			}
			return h.CheckCold()
		},
		func() error {
			out, err := h.Run("reindex")
			if err != nil {
				return err
			}
			lastOut = out
			// The whole reason a reindex budget can pass vacuously:
			// a run that indexed nothing is very fast and exits
			// zero. Every sample is checked, not just the last.
			return checkReindexed(out, fixture.Size)
		})
	if err != nil {
		t.Fatalf("the reindex measurement did not complete, so there is no number to judge: %v", err)
	}

	if err := h.CacheWasBuilt(); err != nil {
		t.Fatalf("the runs were labelled cold reindexes but built no cache: %v", err)
	}

	t.Logf("dira reindex, spawned, over the %d-entry fixture, cache removed before every run:", fixture.Size)
	t.Log(cold.Report("cold reindex"))
	t.Logf("  samples (sorted): %v", cold.Samples)
	t.Log("  the minimum is the uncontended estimate and is asserted against by nothing;")
	t.Log("  contention only ever ADDS time, so min <= median and a ceiling read off")
	t.Log("  the minimum would fire strictly less often than the one this lane wrote.")
	for _, b := range budgets() {
		t.Logf("  budget %-17s %-7v  %s", b.name, b.limit, b.what)
	}
	t.Logf("  what the measured runs reported:\n%s", excerpt(lastOut))

	if err := checkMedian(cold, "cold reindex", "reindexBudget", reindexBudget); err != nil {
		t.Error(err)
	}

	// ---------------------------------------------------------------------
	// Both sides, per L-0001.
	// ---------------------------------------------------------------------

	t.Run("the reindex ceiling fires and names itself", func(t *testing.T) {
		// Red: a ceiling nothing can meet.
		err := checkMedian(cold, "cold reindex", "reindexBudget", time.Microsecond)
		if err == nil {
			t.Fatal("a median over the reindexBudget ceiling was accepted")
		}
		for _, want := range []string{"reindexBudget", "cold reindex", "median"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the failure does not mention %q, so a red run would not be diagnosable:\n%v", want, err)
			}
		}
		// And it reads the median, not the minimum: a ceiling at the
		// observed minimum must still fire when the median is above it.
		if cold.Min < cold.Median {
			if err := checkMedian(cold, "cold reindex", "reindexBudget", cold.Min); err == nil {
				t.Errorf("a ceiling at the observed MINIMUM (%v) did not fire against a median of %v",
					cold.Min.Round(logUnit), cold.Median.Round(logUnit))
			}
		}
		if err := checkMedian(cold, "cold reindex", "reindexBudget", cold.Median-1); err == nil {
			t.Error("a ceiling one nanosecond under the observed median did not fire")
		}
		// Green: a ceiling the same distribution meets, and the exact
		// boundary, which must pass rather than fire.
		if err := checkMedian(cold, "cold reindex", "reindexBudget", cold.Median); err != nil {
			t.Errorf("a ceiling exactly at the observed median fired, so the comparison is off by one: %v", err)
		}
		if err := checkMedian(cold, "cold reindex", "reindexBudget", time.Hour); err != nil {
			t.Errorf("a median inside its ceiling was rejected: %v", err)
		}
	})

	t.Run("an empty ledger fails as indexed 0 of 200", func(t *testing.T) {
		// Red: a reindex over a ledger with no entries. It exits zero,
		// prints a well-formed line, writes a cache and takes almost no
		// time — the fastest possible way for this budget to pass while
		// measuring nothing.
		empty, err := NewHarness(t.TempDir(), 0)
		if err != nil {
			t.Fatal(err)
		}
		out, err := empty.Run("reindex")
		if err != nil {
			t.Fatalf("dira could not reindex an empty ledger at all, so this subtest is testing the wrong failure: %v", err)
		}
		err = checkReindexed(out, fixture.Size)
		if err == nil {
			t.Fatalf("a reindex of an empty ledger was accepted as a measurement:\n%s", excerpt(out))
		}
		if want := fmt.Sprintf("indexed 0 of %d", fixture.Size); !strings.Contains(err.Error(), want) {
			t.Errorf("the failure does not say %q, so a vacuous run would not be diagnosable:\n%v", want, err)
		}
		// Green: the same check over the measured output.
		if err := checkReindexed(lastOut, fixture.Size); err != nil {
			t.Errorf("the untouched fixture reindex was rejected: %v", err)
		}
	})

	t.Run("output that reports nothing is a failure, not a pass", func(t *testing.T) {
		// Red: a reindex whose line dira did not print — an empty
		// stdout, or a line in some other shape — must fail as
		// unreadable rather than be waved through. Both are what a
		// changed output format looks like from here, and a budget over
		// a command whose report nobody can read is a budget over
		// nothing.
		for _, out := range []string{
			"",
			"rebuilt the cache\n",
			"indexed lots of entries and some edges\n",
		} {
			if err := checkReindexed(out, fixture.Size); err == nil {
				t.Errorf("stdout %q was accepted as a reindex report", out)
			}
		}
		// Green: the real line, and the real line with the optional
		// second line dira adds when an entry file will not parse.
		good := fmt.Sprintf("indexed %d entries and 401 edges from /x/.dira into /x/.dira/cache\n", fixture.Size)
		if err := checkReindexed(good, fixture.Size); err != nil {
			t.Errorf("a well-formed report was rejected: %v", err)
		}
		if err := checkReindexed(good+"skipped 1 unreadable entry file(s): dec-0001\n", fixture.Size); err != nil {
			t.Errorf("a well-formed report carrying the unreadable-file notice was rejected: %v", err)
		}
	})

	t.Run("a partial reindex fails rather than being timed", func(t *testing.T) {
		// Red: the realistic version of the vacuous pass, which is not
		// zero entries but fewer than all of them — a reindex that
		// stopped early is faster and still prints a plausible line.
		out := fmt.Sprintf("indexed %d entries and 12 edges from /x/.dira into /x/.dira/cache\n", fixture.Size-1)
		err := checkReindexed(out, fixture.Size)
		if err == nil {
			t.Fatal("a reindex one entry short of the fixture was accepted")
		}
		if want := fmt.Sprintf("indexed %d of %d", fixture.Size-1, fixture.Size); !strings.Contains(err.Error(), want) {
			t.Errorf("the failure does not say %q:\n%v", want, err)
		}
		// Green: exactly the fixture size passes, and so does more —
		// a ledger that grew is not this check's business.
		if err := checkReindexed(strings.Replace(out, fmt.Sprint(fixture.Size-1), fmt.Sprint(fixture.Size), 1), fixture.Size); err != nil {
			t.Errorf("a complete reindex was rejected: %v", err)
		}
	})
}

// checkReindexed reads `dira reindex`'s one line of output and fails a run that
// did not index the whole fixture.
//
// reindex reports what it did — "indexed 200 entries and 401 edges from ... into
// ..." — precisely because, as cmd/dira/reindex.go's comment puts it, "rebuilt
// the cache" with no number is indistinguishable from having done nothing. That
// applies twice over to a budget: a reindex over an empty ledger exits zero,
// prints a well-formed line, writes a cache and returns in a couple of
// milliseconds, which is a green light on nothing.
//
// Unparseable output is a failure rather than a pass. If dira's report changes
// shape, this test must go red and be updated deliberately; a parser that
// shrugged and returned nil would silently stop checking the thing it exists for.
func checkReindexed(stdout string, want int) error {
	line, _, _ := strings.Cut(strings.TrimSpace(stdout), "\n")
	var entries, edges int
	if n, err := fmt.Sscanf(line, "indexed %d entries and %d edges", &entries, &edges); n != 2 || err != nil {
		return fmt.Errorf("dira reindex did not report what it indexed, so there is no evidence the timed run did "+
			"any work; its first line of stdout was %q", line)
	}
	if entries < want {
		return fmt.Errorf("dira reindex indexed %d of %d fixture entries, so the timed run did less work than the "+
			"budget is asserted over", entries, want)
	}
	return nil
}
