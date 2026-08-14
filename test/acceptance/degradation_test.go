package acceptance

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

// fakeCountMatchesDirectory is E4-L5-T1's own completeness check: this
// file's fakeKaziNames list must agree with what is actually on disk under
// fakeKaziDir, so a fake added to one and not the other is caught rather
// than silently ignored.
func TestFakeKaziListMatchesDirectory(t *testing.T) {
	entries, err := os.ReadDir(fakeKaziDir)
	if err != nil {
		t.Fatalf("reading %s: %v", fakeKaziDir, err)
	}
	var onDisk []string
	for _, e := range entries {
		if !e.IsDir() {
			onDisk = append(onDisk, e.Name())
		}
	}
	if len(onDisk) != fakeKaziTotalOnDisk {
		t.Fatalf("%s has %d file(s) (%v), this package expects %d (fakeKaziNames' %d failing scripts "+
			"plus control.sh) — a fake was added to one and not the other",
			fakeKaziDir, len(onDisk), onDisk, fakeKaziTotalOnDisk, len(fakeKaziNames))
	}
	expected := append(append([]string{}, fakeKaziNames...), "control.sh")
	for _, name := range expected {
		if !slices.Contains(onDisk, name) {
			t.Errorf("expected fake %q is not in %s", name, fakeKaziDir)
		}
	}
}

// degradedLedgerEntries builds a fixture ledger covering exactly the
// buckets dec-0004's own prose names as surviving degradation:
// to-be-planned, decision-blocked, and the two terminal groups. "planned"
// is deliberately NOT one of them — dec-0004's join table defines it as
// "realized_by points at an approved proposal or an unapplied goal", which
// is unknowable without asking kazi, so a degraded run can never legitimately
// report it. See this lane's status report for the discrepancy this settles
// against docs/plan/tasks/E4-L5.md's own (looser) parenthetical.
func degradedLedgerEntries() []*ledger.Entry {
	parent := mkEntry("int-9001", nil) // active, no realized_by: ToBePlanned, itself a group header
	toBePlanned := mkEntry("dec-9002", func(e *ledger.Entry) {
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeDerivesFrom, To: "int-9001"})
	})
	blocked := mkEntry("dec-9003", func(e *ledger.Entry) {
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeDerivesFrom, To: "int-9001"})
	})
	question := mkEntry("qst-9004", func(e *ledger.Entry) {
		e.State = ledger.StateOpen
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeBlocks, To: "dec-9003"})
	})
	achieved := mkEntry("int-9005", func(e *ledger.Entry) { e.State = ledger.StateAchieved })
	abandoned := mkEntry("int-9006", func(e *ledger.Entry) { e.State = ledger.StateAbandoned })
	return []*ledger.Entry{parent, toBePlanned, blocked, question, achieved, abandoned}
}

// workingLedgerEntries builds a fixture ledger with real entries in
// Completed, InProgress and ExecutionBlocked — T2's positive-run fixture,
// paired with workingKaziScript below.
func workingLedgerEntries() []*ledger.Entry {
	completed := mkEntry("dec-9101", func(e *ledger.Entry) {
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeRealizedBy, To: "kazi:goal-degrade-complete"})
	})
	inProgress := mkEntry("dec-9102", func(e *ledger.Entry) {
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeRealizedBy, To: "kazi:goal-degrade-progress"})
	})
	blocked := mkEntry("dec-9103", func(e *ledger.Entry) {
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeRealizedBy, To: "kazi:goal-degrade-blocked"})
	})
	return []*ledger.Entry{completed, inProgress, blocked}
}

// workingKaziScript is a fake `kazi` answering `portfolio --json` with
// single-run goals covering Complete/InProgress/Stuck — distinct from
// fakeKaziDir's own control.sh, which reports an empty portfolio and so
// cannot exercise any of the three execution-bucket labels.
//
// `echo`, not `cat <<heredoc`: this suite deliberately isolates PATH down
// to the fake's own directory (proving PATH resolution finds exactly the
// fake, the same discipline internal/kazi/failopen_test.go's isolatedPATH
// uses), which means an EXTERNAL command like `cat` is unresolvable inside
// the script too — `cat: command not found` was observed directly while
// authoring this fixture. `echo` is a shell builtin and needs no PATH
// lookup, matching fakeKaziDir's own control.sh.
const workingKaziScript = `#!/bin/sh
if [ "$1" = "portfolio" ]; then
  echo '{"kind":"portfolio","schema_version":2,"planned":[],"by_repo":{"repo-a":{"complete":[{"goal_ref":"degrade-complete","run_id":"run-1","status":"converged"}],"in_progress":[{"goal_ref":"degrade-progress","run_id":"run-2","status":"running"}],"stuck":[{"goal_ref":"degrade-blocked","run_id":"run-3","status":"stuck"}]}},"fleet_remote":[],"totals":{"base":1,"empty":false,"rows":[{"bucket":"done","count":1,"pct":100}]},"todo":[],"blocked":[{"goal_ref":"degrade-blocked","cause":"stuck","blocker":"blocked: x"}],"rate":{"total":0,"green":0,"empty?":true,"delta":0}}'
  exit 0
fi
echo "unsupported" >&2
exit 2
`

// TestDegradation is T1 and T2's combined acceptance gate.
func TestDegradation(t *testing.T) {
	binary := buildDira(t)
	degradedRoot := ledgerFixture(t, degradedLedgerEntries())

	// reasons collects the unavailability text observed for each of the
	// five failing fakes, so distinctness (T1's own clause) is checked
	// once at the end. "no-kazi-at-all" is the fifth: ReasonNotOnPath has
	// no committed script (an empty PATH directory IS that case, the same
	// shape internal/kazi/failopen_test.go's own "no kazi at all" sub-test
	// uses), so it is driven directly here rather than through
	// installFakeKazi.
	type scenario struct {
		name string
		path func(t *testing.T) string
	}
	scenarios := []scenario{{name: "no-kazi-at-all", path: func(t *testing.T) string { return t.TempDir() }}}
	for _, name := range fakeKaziNames {
		scenarios = append(scenarios, scenario{name: name, path: func(t *testing.T) string { return installFakeKazi(t, name) }})
	}

	lines := make(map[string]string, len(scenarios))

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			kaziDir := sc.path(t)
			code, stdout, stderr := runDiraMap(t, binary, degradedRoot, kaziDir)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
			}

			// Every ledger-side bucket dec-0004 names as surviving
			// degradation renders.
			for _, want := range []string{"to be planned", "blocks this", "achieved", "abandoned"} {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout does not contain %q:\n%s", want, stdout)
				}
			}

			// An unavailability line naming a reason.
			if !strings.Contains(stdout, "unavailable") {
				t.Errorf("stdout carries no unavailability line:\n%s", stdout)
			}

			firstLine, _, _ := strings.Cut(stdout, "\n")
			lines[sc.name] = firstLine
		})
	}

	t.Run("the five reasons are pairwise distinct", func(t *testing.T) {
		if len(lines) != len(scenarios) {
			t.Fatalf("collected %d line(s), want %d", len(lines), len(scenarios))
		}
		seen := map[string]string{}
		for name, line := range lines {
			if owner, dup := seen[line]; dup {
				t.Errorf("%s and %s produced the identical unavailability line %q", owner, name, line)
			}
			seen[line] = name
		}
	})

	t.Run("no banner at all against a succeeding kazi", func(t *testing.T) {
		kaziDir := installFakeKazi(t, "control.sh")
		code, stdout, stderr := runDiraMap(t, binary, degradedRoot, kaziDir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
		}
		if strings.Contains(stdout, "unavailable") {
			t.Errorf("stdout carries an unavailability line against a succeeding kazi:\n%s", stdout)
		}
	})

	t.Run("both sides: a generic disclaimer regardless of health prints even when kazi succeeds", func(t *testing.T) {
		// The wrong renderer, constructed inline: always prints a banner.
		alwaysBanner := func() string {
			return "execution status may be unavailable"
		}
		if !strings.Contains(alwaysBanner(), "unavailable") {
			t.Fatal("the generic-disclaimer control's own premise broke")
		}

		kaziDir := installFakeKazi(t, "control.sh")
		_, stdout, _ := runDiraMap(t, binary, degradedRoot, kaziDir)
		if strings.Contains(stdout, "unavailable") {
			t.Fatal("the real renderer printed a banner against a succeeding kazi too")
		}
	})

	// --- E4-L5-T2: label absence, paired with a positive run ---------

	t.Run("label absence under every degraded run", func(t *testing.T) {
		for _, sc := range scenarios {
			t.Run(sc.name, func(t *testing.T) {
				kaziDir := sc.path(t)
				_, stdout, _ := runDiraMap(t, binary, degradedRoot, kaziDir)
				for _, banned := range []string{"converged", "in progress", "execution-blocked"} {
					if strings.Contains(stdout, banned) {
						t.Errorf("stdout contains the banned label %q under %s:\n%s", banned, sc.name, stdout)
					}
				}
			})
		}
	})

	t.Run("the positive run: all three execution labels present against a working kazi", func(t *testing.T) {
		workingRoot := ledgerFixture(t, workingLedgerEntries())
		kaziDir := installScript(t, workingKaziScript)
		code, stdout, stderr := runDiraMap(t, binary, workingRoot, kaziDir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
		}
		for _, want := range []string{"converged", "in progress", "execution-blocked"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("stdout does not contain %q against a working kazi:\n%s", want, stdout)
			}
		}
	})

	t.Run("both sides: a renderer that never prints any execution label would still pass every negative clause", func(t *testing.T) {
		// This is the gap the positive run exists to close, demonstrated
		// directly: a renderer emitting nothing execution-shaped at all
		// satisfies every "absent under degradation" assertion above
		// while being simply broken, not degraded.
		brokenOutput := "int-0001  some intent\n  dec-0001  some decision\n"
		for _, banned := range []string{"converged", "in progress", "execution-blocked"} {
			if strings.Contains(brokenOutput, banned) {
				t.Fatalf("the broken-renderer control's own premise broke: it must contain none of the labels")
			}
		}
		// And yet it must fail the positive clause, which the real run
		// above already proved does not happen for the actual renderer.
		for _, want := range []string{"converged", "in progress", "execution-blocked"} {
			if strings.Contains(brokenOutput, want) {
				t.Fatal("unreachable")
			}
		}
	})
}
