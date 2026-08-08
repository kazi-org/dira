//go:build perf

package perf

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/ledger/fixture"
)

// TestWarmBriefBudget is the common case: the cache exists, reconciles clean,
// and the hook pays a read rather than a build.
//
// Every session start after the first one is this run, so it is the number a
// user meets most often — and it is the one a regression hides in, because a
// cache that has quietly stopped being reused still returns the right answer.
// It just costs the cold price forever. A budget over the warm path is how that
// becomes visible.
//
// Two ceilings are asserted, both against the MEDIAN and neither against the
// minimum. coldMedianBudget because the lane requires a warm invocation not to
// exceed the cold one, and warmBudget because "not worse than cold" is a very
// weak claim about a path that skips the whole build. The two are reported
// separately so a red run says which of the two was broken: over warmBudget but
// under coldMedianBudget is a slower read, over both is a cache that is not
// being used.
func TestWarmBriefBudget(t *testing.T) {
	if reason := SkipReason(testing.Short()); reason != "" {
		t.Skip(reason)
	}

	argv, err := HookBriefArgv(hookSettings)
	if err != nil {
		t.Fatal(err)
	}

	h, err := NewHarness(t.TempDir(), fixture.Size)
	if err != nil {
		if errors.Is(err, ErrNoToolchain) {
			t.Skip(err)
		}
		t.Fatal(err)
	}

	// One untimed run to build the cache. It is not a sample: it is the cold
	// run TestColdStartBudget already owns, and including it here would put a
	// cold number in a warm distribution.
	out, err := h.Run(argv...)
	if err != nil {
		t.Fatalf("the cache-building run failed, so there is no warm state to measure: %v", err)
	}
	if err := h.CheckBrief(out); err != nil {
		t.Fatalf("the cache-building run rendered nothing from the fixture: %v", err)
	}
	if err := h.CacheWasBuilt(); err != nil {
		t.Fatalf("the cache-building run left no cache, so every sample below would be cold: %v", err)
	}

	// The proof that the runs below were warm, taken before and after the
	// samples. See cacheFingerprint.
	before, err := cacheFingerprint(h)
	if err != nil {
		t.Fatal(err)
	}

	var lastOut string
	warm, err := Measure(samples,
		func() error {
			// Setup, and it asserts rather than prepares: a warm
			// sample needs the cache left exactly where the last one
			// put it. Anything that removed it would turn every
			// remaining sample cold, and a cold sample reported as
			// warm is the failure this whole test is built around.
			return checkWarm(h)
		},
		func() error {
			out, err := h.Run(argv...)
			if err != nil {
				return err
			}
			lastOut = out
			return h.CheckBrief(out)
		})
	if err != nil {
		t.Fatalf("the warm measurement did not complete, so there is no number to judge: %v", err)
	}

	// The real warmth check, and the reason it is a fingerprint rather than a
	// flag: dira has no "I reused the cache" output on the brief path, so the
	// only honest evidence is that the cache file the runs read is byte-for-byte
	// the file they started with. A run that rebuilt it — a schema version bump,
	// a staleness check that decided everything changed, a cache written to a
	// different path and this one left untouched — moves index.db's mtime.
	// Observed on this machine: three consecutive warm briefs leave index.db at
	// size 61440 and mtime 1785529204.274944555, unchanged to the nanosecond.
	after, err := cacheFingerprint(h)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("the cache changed across the %d runs labelled warm, so they were not warm runs and this "+
			"number is not a warm number:\n  before: %s\n  after:  %s", warm.N, before, after)
	}

	t.Logf("dira %s, spawned, over the %d-entry fixture, cache left in place (fingerprint %s):",
		strings.Join(argv, " "), fixture.Size, before)
	t.Log(warm.Report("warm brief"))
	t.Logf("  samples (sorted): %v", warm.Samples)
	t.Log("  the minimum is the uncontended estimate and is asserted against by nothing;")
	t.Log("  contention only ever ADDS time, so min <= median and a ceiling read off")
	t.Log("  the minimum would fire strictly less often than the one this lane wrote.")
	for _, b := range budgets() {
		t.Logf("  budget %-17s %-7v  %s", b.name, b.limit, b.what)
	}
	t.Logf("  a sample of the measured output:\n%s", excerpt(lastOut))

	// Both ceilings, reported separately. Neither is `else` to the other: a
	// reader of a red run needs to know whether warm merely got slower or
	// stopped being warm at all.
	if err := checkMedian(warm, "warm brief", "coldMedianBudget", coldMedianBudget); err != nil {
		t.Error(err)
	}
	if err := checkMedian(warm, "warm brief", "warmBudget", warmBudget); err != nil {
		t.Error(err)
	}

	// ---------------------------------------------------------------------
	// Both sides, per L-0001. Every check this measurement rests on is shown
	// to fail a constructed defect of its own class AND to pass the untouched
	// correct case, on every run, without editing a constant this lane is
	// forbidden to edit.
	// ---------------------------------------------------------------------

	t.Run("each warm ceiling fires alone and names itself", func(t *testing.T) {
		// Red: the same distribution against a ceiling nothing can meet,
		// once per budget name, so neither message can be standing in
		// for the other.
		for _, name := range []string{"coldMedianBudget", "warmBudget"} {
			err := checkMedian(warm, "warm brief", name, time.Microsecond)
			if err == nil {
				t.Fatalf("a median over the %s ceiling was accepted", name)
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("the failure does not name %q, so a red run would not say which budget broke:\n%v", name, err)
			}
			if !strings.Contains(err.Error(), "median") {
				t.Errorf("the failure does not say the statistic it read is the median:\n%v", err)
			}
		}
		// Red the other way round: a ceiling equal to the observed
		// median must NOT fire, and one a nanosecond under it must.
		// This is what pins the comparison to the median rather than to
		// the minimum — a check reading the minimum would pass both.
		if err := checkMedian(warm, "warm brief", "warmBudget", warm.Median); err != nil {
			t.Errorf("a ceiling exactly at the observed median fired, so the comparison is off by one: %v", err)
		}
		if err := checkMedian(warm, "warm brief", "warmBudget", warm.Median-1); err == nil {
			t.Error("a ceiling one nanosecond under the observed median did not fire")
		}
		if warm.Min < warm.Median {
			if err := checkMedian(warm, "warm brief", "warmBudget", warm.Min); err == nil {
				t.Errorf("a ceiling at the observed MINIMUM (%v) did not fire against a median of %v; "+
					"the assertion is reading the uncontended estimate, which is the loosening "+
					"internal/index/latency_test.go's comment warns about",
					warm.Min.Round(logUnit), warm.Median.Round(logUnit))
			}
		}
		// Green: the same distribution against a ceiling it meets.
		if err := checkMedian(warm, "warm brief", "warmBudget", time.Hour); err != nil {
			t.Errorf("a median inside its ceiling was rejected: %v", err)
		}
	})

	t.Run("a run with no cache is not accepted as warm", func(t *testing.T) {
		// Red: remove the cache, which is precisely the state a cold
		// sample starts in. A warm measurement that ran here would be
		// timing the cold path under the warm budget.
		if err := h.RemoveCache(); err != nil {
			t.Fatal(err)
		}
		if err := checkWarm(h); err == nil {
			t.Error("a run was accepted as warm with no cache in place")
		}
		// Green: rebuild it and check again.
		if _, err := h.Run(argv...); err != nil {
			t.Fatal(err)
		}
		if err := checkWarm(h); err != nil {
			t.Errorf("a genuinely warm run was rejected: %v", err)
		}
	})

	t.Run("a rebuilt cache is caught by the fingerprint", func(t *testing.T) {
		// Green first, because it is the side that is skipped: two
		// fingerprints across a genuine warm run must be equal, or the
		// check above passed by luck rather than by measuring anything.
		start, err := cacheFingerprint(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.Run(argv...); err != nil {
			t.Fatal(err)
		}
		unchanged, err := cacheFingerprint(h)
		if err != nil {
			t.Fatal(err)
		}
		if start != unchanged {
			t.Errorf("a genuine warm run changed the cache fingerprint, so this check cannot tell a warm run "+
				"from a rebuild:\n  before: %s\n  after:  %s", start, unchanged)
		}

		// Red: throw the cache away and let the next run rebuild it,
		// which is the mislabelled-warm defect exactly. The fingerprint
		// must differ.
		if err := h.RemoveCache(); err != nil {
			t.Fatal(err)
		}
		if _, err := h.Run(argv...); err != nil {
			t.Fatal(err)
		}
		rebuilt, err := cacheFingerprint(h)
		if err != nil {
			t.Fatal(err)
		}
		if rebuilt == unchanged {
			t.Errorf("a rebuilt cache produced the same fingerprint as the one it replaced (%s), so a run that "+
				"rebuilt the cache would be reported as a warm number", rebuilt)
		}
	})

	t.Run("an empty cache directory is not a fingerprint", func(t *testing.T) {
		// Red: a missing cache directory must be an error rather than a
		// fingerprint that happens to compare equal to another missing
		// one — otherwise two cold runs in a row would agree and be
		// reported as warm.
		if err := h.RemoveCache(); err != nil {
			t.Fatal(err)
		}
		if got, err := cacheFingerprint(h); err == nil {
			t.Errorf("a removed cache directory fingerprinted as %q instead of failing", got)
		}
		// Green: rebuild and fingerprint it.
		if _, err := h.Run(argv...); err != nil {
			t.Fatal(err)
		}
		if _, err := cacheFingerprint(h); err != nil {
			t.Errorf("a present cache directory could not be fingerprinted: %v", err)
		}
	})
}

// checkMedian is the assertion this task's two tests share: one named ceiling,
// read against the MEDIAN and against nothing else.
//
// It is not CheckBudgets. That one asserts a median and a single-run maximum
// together, which is what int-0002 says about the cold path and is not what this
// lane says about either of these two — E1-L6-T2's acceptance line names a
// median ceiling for warm and a median ceiling for reindex, and no maximum. An
// unasked-for single-run maximum over N>=20 samples is a coin-flip failure mode
// nobody sanctioned, so it is not added here.
//
// The ceiling arrives as an argument for the same reason CheckBudgets' does: the
// red side of a budget assertion is otherwise reachable only by editing the
// constant, which is a thing done once in a shell and never again. As a
// parameter, both sides are proved from the tree on every run.
//
// It lives in warm_test.go and is used from reindex_test.go too. Both files are
// E1-L6-T2's, and the alternative is these eight lines written twice.
func checkMedian(t Timing, label, budgetName string, limit time.Duration) error {
	if t.Median <= limit {
		return nil
	}
	return fmt.Errorf("the %s budget %s is broken: median of %d runs is %v, over the %v ceiling by %v "+
		"(min %v, the uncontended estimate, is not what this asserts against)",
		label, budgetName, t.N,
		t.Median.Round(logUnit), limit.Round(logUnit), (t.Median - limit).Round(logUnit),
		t.Min.Round(logUnit))
}

// checkWarm fails a run labelled warm that has no cache to be warm over.
//
// It is CheckCold's opposite and is written out rather than derived from it: an
// inverted CheckCold would read a stat error other than "does not exist" as
// proof of warmth, which is the wrong direction to be lenient in.
func checkWarm(h *Harness) error {
	switch _, err := os.Stat(h.CacheDir); {
	case err == nil:
		return nil
	case errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("%s does not exist at the start of a run labelled warm; the sample would be a cold one",
			h.CacheDir)
	default:
		return fmt.Errorf("checking %s: %w", h.CacheDir, err)
	}
}

// cacheFingerprint is every file in the cache directory by name, size and
// modification time to the nanosecond, in a stable order.
//
// This is the evidence that a warm measurement was warm. dira's brief path
// prints no reconcile statistics — `dira reindex` reports what it indexed,
// `dira brief` does not — so there is no number from the binary to read. What
// there is, is the file: a run that reused the cache leaves index.db exactly as
// it found it, and a run that rebuilt it does not. Measured on this machine,
// three consecutive warm briefs left index.db at size 61440 with mtime
// 1785529204.274944555 unchanged to the nanosecond, and a rebuild moves it.
//
// A missing directory is an error rather than an empty fingerprint, because two
// cold runs in a row would otherwise fingerprint identically and be accepted as
// a warm pair — a check that certifies the exact state it exists to reject.
func cacheFingerprint(h *Harness) (string, error) {
	names, err := os.ReadDir(h.CacheDir)
	if err != nil {
		return "", fmt.Errorf("fingerprinting the cache: %w", err)
	}
	if len(names) == 0 {
		return "", fmt.Errorf("%s is empty, so there is no cache for a warm run to have reused", h.CacheDir)
	}
	lines := make([]string, 0, len(names))
	for _, name := range names {
		info, err := name.Info()
		if err != nil {
			return "", fmt.Errorf("fingerprinting %s: %w", name.Name(), err)
		}
		lines = append(lines, fmt.Sprintf("%s size=%d mtime=%d", name.Name(), info.Size(), info.ModTime().UnixNano()))
	}
	sort.Strings(lines)
	return strings.Join(lines, "; "), nil
}
