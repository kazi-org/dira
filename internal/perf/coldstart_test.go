//go:build perf

package perf

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/ledger/fixture"
)

func TestMain(m *testing.M) {
	code := m.Run()
	CleanupBinary()
	os.Exit(code)
}

// hookArgv is the argv this package measures, as
// hooks/settings.example.json's SessionStart block installs it. Changing what
// that hook runs must change what this budget is held over, or the budget stops
// being about anything a user pays for.
var hookArgv = []string{"brief", "--context", "--chain"}

// TestColdStartBudget is int-0002 asserted against the shipped binary.
//
// Cold means .dira/cache removed before every timed run, so each sample pays
// process start, Go runtime and package init, the config read, a cache built
// from nothing over 200 entries, and the render. That is what a user pays on the
// first session after a clone, and it is the number E1-L5's in-process 61.2ms
// does not contain.
//
// The subtests are not decoration. A timing harness fails safe in the wrong
// direction — every way it can break returns a very fast number that looks
// exactly like a pass — so each of the three ways this one could measure nothing
// is proved to fail, on every run, against a deliberately constructed defect.
// L-0001: a check that cannot fail a wrong input is worthless, and a check whose
// green side nobody has watched is worse, because it is trusted.
func TestColdStartBudget(t *testing.T) {
	if reason := SkipReason(testing.Short()); reason != "" {
		t.Skip(reason)
	}

	argv, err := HookBriefArgv(hookSettings)
	if err != nil {
		if errors.Is(err, ErrNoToolchain) {
			t.Skip(err)
		}
		t.Fatal(err)
	}

	// The sibling assertion. The measurement below uses whatever the hook
	// file says; this is what makes a change to it loud instead of silent.
	t.Run("the measured argv is the one the hook installs", func(t *testing.T) {
		if !slices.Equal(argv, hookArgv) {
			t.Errorf("hooks/settings.example.json's SessionStart hook now runs `dira %s`, not `dira %s`.\n"+
				"int-0002's budget is held over the command a session start actually pays for. Either restore the "+
				"hook or update hookArgv here deliberately, with the new command's own measurement.",
				strings.Join(argv, " "), strings.Join(hookArgv, " "))
		}
	})

	// And the other half of it: an assertion that only ever sees one input has
	// never been shown to discriminate. These are synthetic hook files rather
	// than an edit to the real one, so the red side is proved on every run
	// instead of once in somebody's shell.
	t.Run("a drifted hook configuration is read as drifted", func(t *testing.T) {
		dir := t.TempDir()

		write := func(name, body string) string {
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			return path
		}

		// Red: a SessionStart hook that installs a different command.
		drifted := write("drifted.json",
			`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"dira brief 2>/dev/null || true"}]}]}}`)
		got, err := HookBriefArgv(drifted)
		if err != nil {
			t.Fatal(err)
		}
		if slices.Equal(got, hookArgv) {
			t.Errorf("a hook installing `dira brief` parsed as %v, the same as the measured argv", got)
		}
		if want := []string{"brief"}; !slices.Equal(got, want) {
			t.Errorf("parsed %v, want %v — the shell redirection and `|| true` are not part of the argv", got, want)
		}

		// Red: a configuration that registers no dira SessionStart hook at
		// all must be an error, not an empty argv quietly measured.
		none := write("none.json",
			`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"dira sniff --stage --quiet"}]}]}}`)
		if got, err := HookBriefArgv(none); err == nil {
			t.Errorf("a configuration with no SessionStart dira hook parsed as %v, want an error", got)
		}

		// Green: the real file, which is what the measurement runs.
		got, err = HookBriefArgv(hookSettings)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, hookArgv) {
			t.Errorf("%s parsed as %v, want %v", hookSettings, got, hookArgv)
		}
	})

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
			// Setup, outside the timer: the removal is not the work.
			if err := h.RemoveCache(); err != nil {
				return err
			}
			return h.CheckCold()
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
		t.Fatalf("the cold measurement did not complete, so there is no number to judge: %v", err)
	}

	// A cold run that left nothing behind did not do the cold work.
	if err := h.CacheWasBuilt(); err != nil {
		t.Fatalf("the runs were labelled cold but built no cache: %v", err)
	}

	t.Logf("dira %s, spawned, over the %d-entry fixture, cache removed before every run:",
		strings.Join(argv, " "), fixture.Size)
	t.Log(cold.Report("cold brief"))
	t.Logf("  samples (sorted): %v", cold.Samples)
	t.Log("  the minimum is the uncontended estimate and is asserted against by nothing;")
	t.Log("  contention only ever ADDS time, so min <= median and a ceiling read off")
	t.Log("  the minimum would fire strictly less often than the one this lane wrote.")
	for _, b := range budgets() {
		t.Logf("  budget %-17s %-7v  %s", b.name, b.limit, b.what)
	}
	t.Logf("  a sample of the measured output:\n%s", excerpt(lastOut))

	if err := CheckBudgets(cold, "cold", coldMedianBudget, coldMaxBudget); err != nil {
		t.Error(err)
	}

	// ---------------------------------------------------------------------
	// Both sides of every check this measurement rests on. Each subtest
	// constructs the defect and asserts the check catches it, then presents
	// the correct case and asserts the check passes it.
	// ---------------------------------------------------------------------

	t.Run("a non-zero exit is a failure, not a fast sample", func(t *testing.T) {
		// Red: a deliberately broken argv. `dira brief` fails open by
		// design — the hook discards stderr and swallows the exit code —
		// so a usage error returns in a few milliseconds and would be
		// the fastest sample in the distribution.
		if out, err := h.Run("brief", "--no-such-flag"); err == nil {
			t.Errorf("a broken argv exited zero and was accepted as a sample:\n%s", excerpt(out))
		}
		// Green: the real argv.
		if _, err := h.Run(argv...); err != nil {
			t.Errorf("the untouched command was rejected: %v", err)
		}
	})

	t.Run("an empty ledger fails as measuring nothing", func(t *testing.T) {
		// Red: a ledger with no entries. dira renders every section
		// header with "none" under it, exits zero, and is very fast.
		empty, err := NewHarness(t.TempDir(), 0)
		if err != nil {
			t.Fatal(err)
		}
		out, err := empty.Run(argv...)
		if err != nil {
			t.Fatalf("dira could not read an empty ledger at all, so this subtest is testing the wrong failure: %v", err)
		}
		if !strings.Contains(out, briefHeader) {
			t.Fatalf("an empty ledger printed no %q header, so the id check is not what is being proved here:\n%s",
				briefHeader, excerpt(out))
		}
		if err := empty.CheckBrief(out); err == nil {
			t.Errorf("a brief over an empty ledger was accepted as a measurement:\n%s", excerpt(out))
		}
		// Green: the same check over the fixture output.
		if err := h.CheckBrief(lastOut); err != nil {
			t.Errorf("the untouched fixture brief was rejected: %v", err)
		}
	})

	t.Run("a surviving cache fails as mislabelled cold", func(t *testing.T) {
		// Red: skip the removal. After the measurement above the cache
		// exists, so calling the check without RemoveCache first is
		// exactly the defect — a removal that silently did not happen.
		if err := h.CheckCold(); err == nil {
			t.Error("a run was accepted as cold with the cache still in place")
		}
		// Green: remove it and check again.
		if err := h.RemoveCache(); err != nil {
			t.Fatal(err)
		}
		if err := h.CheckCold(); err != nil {
			t.Errorf("a genuinely cold run was rejected: %v", err)
		}
	})

	t.Run("both budget ceilings fire, and name themselves", func(t *testing.T) {
		// Red: the same distribution against ceilings nothing can meet.
		// This is the constructed defect for the budget assertion
		// itself, held in the tree rather than in a shell history — the
		// alternative is editing budget.go, which this lane forbids.
		err := CheckBudgets(cold, "cold", time.Microsecond, time.Microsecond)
		if err == nil {
			t.Fatal("a distribution over both ceilings was accepted")
		}
		for _, want := range []string{"cold MEDIAN budget is broken", "cold SINGLE-RUN MAXIMUM budget is broken"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the failure does not name %q, so a red run would not say which budget broke:\n%v", want, err)
			}
		}
		// And each fires alone, so neither message is standing in for
		// the other.
		if err := CheckBudgets(cold, "cold", time.Microsecond, time.Hour); err == nil ||
			strings.Contains(err.Error(), "SINGLE-RUN MAXIMUM") {
			t.Errorf("the median ceiling alone did not produce the median failure and nothing else: %v", err)
		}
		if err := CheckBudgets(cold, "cold", time.Hour, time.Microsecond); err == nil ||
			strings.Contains(err.Error(), "MEDIAN budget") {
			t.Errorf("the maximum ceiling alone did not produce the maximum failure and nothing else: %v", err)
		}
		// Green: the same distribution against ceilings it meets.
		if err := CheckBudgets(cold, "cold", time.Hour, time.Hour); err != nil {
			t.Errorf("a distribution inside both ceilings was rejected: %v", err)
		}
	})

	t.Run("a failing sample stops the measurement", func(t *testing.T) {
		// No spawns: this is about Measure itself. A harness that
		// swallowed a failing run would report a median over the
		// samples that happened to work.
		//
		// Measure takes one UNTIMED warm-up run before the samples, so
		// the fourth call is the third sample.
		calls := 0
		_, err := Measure(samples, func() error { return nil }, func() error {
			calls++
			if calls == 4 {
				return errors.New("constructed failure")
			}
			return nil
		})
		if err == nil {
			t.Fatal("a measurement whose third sample failed returned a distribution")
		}
		if !strings.Contains(err.Error(), "sample 3") {
			t.Errorf("the failure does not name the sample that broke: %v", err)
		}
		// Green: the same shape with nothing failing.
		if _, err := Measure(samples, func() error { return nil }, func() error { return nil }); err != nil {
			t.Errorf("a measurement with no failing sample returned an error: %v", err)
		}
	})
}
