package cli_test

// TestMap is E4-L4-T7's acc line: assembles T1-T6 into the golden-file
// suite `go test ./internal/cli -run TestMap` is graded on as one
// invocation — which it already is, since every TestMap* function above
// matches that pattern by substring, the same way internal/status's TestJoin
// umbrella works. This file adds the golden-file scenarios and the two
// completeness checks (all six labels present; the unparented scenario's
// rendered count matches its fixture).
//
// update rewrites the golden files instead of comparing against them:
//
//	go test ./internal/cli -run TestMap -update

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/cli"
	"github.com/kazi-org/dira/internal/kazi"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/status"
)

var update = flag.Bool("update", false, "rewrite the golden files under testdata/map")

// realSnapshotEntries returns a frozen copy of this repository's own real
// ledger — the same rationale docs/plan/tasks/E4-L2.md's fixture note
// gives: a live pointer at the working ledger is fragile against concurrent
// worktree edits, so this reads a point-in-time copy captured alongside
// this lane rather than the live .dira/ tree.
func realSnapshotEntries(t *testing.T) []*ledger.Entry {
	t.Helper()
	dir := filepath.Join("testdata", "map", "real-snapshot", "entries")
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var out []*ledger.Entry
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			t.Fatalf("reading %s/%s: %v", dir, f.Name(), err)
		}
		e, err := ledger.Decode(data)
		if err != nil {
			t.Fatalf("decoding %s/%s: %v", dir, f.Name(), err)
		}
		out = append(out, e)
	}
	return out
}

// unparentedScenarioEntries covers Completed, InProgress, ExecutionBlocked
// and Planned (the four buckets realSnapshotEntries cannot, since this
// repo's real ledger carries no realized_by edge — internal/status's own
// fixture README records the same finding), plus one entry with no
// derives_from parent.
func unparentedScenarioEntries() []*ledger.Entry {
	root := entry("int-7001", nil) // itself unparented
	completed := entry("dec-7002", combine(derivesFrom("int-7001"), realizedBy("kazi:goal-golden-complete")))
	inProgress := entry("dec-7003", combine(derivesFrom("int-7001"), realizedBy("kazi:goal-golden-progress")))
	blocked := entry("dec-7004", combine(derivesFrom("int-7001"), realizedBy("kazi:goal-golden-blocked")))
	planned := entry("dec-7005", combine(derivesFrom("int-7001"), realizedBy("kazi:prop-golden-planned")))
	lone := entry("dec-7006", nil) // no parent, no children: the unparented entry itself
	return []*ledger.Entry{root, completed, inProgress, blocked, planned, lone}
}

func unparentedScenarioPortfolio() *kazi.Portfolio {
	snap := singleRunPortfolio(map[string]kazi.RepoBucket{
		"golden-complete": kazi.RepoComplete,
		"golden-progress": kazi.RepoInProgress,
		"golden-blocked":  kazi.RepoStuck,
	})
	snap.Planned = []kazi.Proposal{{ProposalRef: "prop-golden-planned", GoalID: "golden-planned", Status: "proposed"}}
	return snap
}

// goldenScenarios is the golden-file suite T7's acc line requires: at least
// three, covering the real ledger, an unparented entry, and kazi
// unavailable.
var goldenScenarios = []struct {
	name    string
	entries func(t *testing.T) []*ledger.Entry
	snap    *kazi.Portfolio
	snapErr error
}{
	{
		name:    "real-snapshot",
		entries: realSnapshotEntries,
		snap:    &kazi.Portfolio{},
	},
	{
		name:    "unparented",
		entries: func(*testing.T) []*ledger.Entry { return unparentedScenarioEntries() },
		snap:    unparentedScenarioPortfolio(),
	},
	{
		name:    "kazi-unavailable",
		entries: func(*testing.T) []*ledger.Entry { return unparentedScenarioEntries() },
		snapErr: &kazi.Unavailable{Reason: kazi.ReasonNotOnPath, Detail: "kazi: executable file not found in $PATH"},
	},
}

// TestMap is T7's acc line.
func TestMap(t *testing.T) {
	ctx := context.Background()
	rendered := make(map[string]string, len(goldenScenarios))

	for _, sc := range goldenScenarios {
		t.Run(sc.name, func(t *testing.T) {
			entries := sc.entries(t)
			ix := openTree(t, entries)

			// Every golden fixture is single-run per goal, so the fan-out
			// to kazi status is never exercised here — TestJoinMultiRun
			// (internal/status) already proves that path.
			tree, err := cli.BuildTree(ctx, ix, sc.snap, sc.snapErr, neverCalledStatusFn(t))
			if err != nil {
				t.Fatalf("BuildTree: %v", err)
			}
			var buf bytes.Buffer
			if err := cli.RenderText(&buf, tree, sc.snapErr); err != nil {
				t.Fatalf("RenderText: %v", err)
			}
			got := buf.String()
			rendered[sc.name] = got

			path := filepath.Join("testdata", "map", sc.name+".golden")
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatalf("writing %s: %v", path, err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v\nrun `go test ./internal/cli -run TestMap -update` to create it", path, err)
			}
			if got != string(want) {
				t.Errorf("%s does not match; re-run with -update if the change is intended.\n--- want ---\n%s\n--- got ---\n%s",
					path, want, got)
			}

			if sc.name == "unparented" {
				if got := countRenderedEntries(rendered[sc.name]); got != len(entries) {
					t.Errorf("rendered entry count = %d, want %d (the fixture's own entry count)", got, len(entries))
				}
			}
		})
	}

	t.Run("all six committed labels appear across the golden set combined", func(t *testing.T) {
		var all strings.Builder
		for _, sc := range goldenScenarios {
			all.WriteString(rendered[sc.name])
		}
		combined := all.String()
		for _, b := range status.Buckets {
			label := cli.Label(b)
			if !strings.Contains(combined, label) {
				t.Errorf("bucket %s's label %q does not appear anywhere in the golden set", b, label)
			}
		}
	})

	t.Run("both sides: a golden set missing one bucket's coverage is flagged", func(t *testing.T) {
		// A scratch copy of the combined text with one bucket's label
		// removed, used only inside this sub-test's own verification —
		// never written to the real golden files.
		var all strings.Builder
		for _, sc := range goldenScenarios {
			all.WriteString(rendered[sc.name])
		}
		poisoned := strings.ReplaceAll(all.String(), cli.Label(status.Completed), "REDACTED")
		if strings.Contains(poisoned, cli.Label(status.Completed)) {
			t.Fatal("the poisoned copy's own premise broke: the label must be fully removed")
		}
		missing := false
		for _, b := range status.Buckets {
			if !strings.Contains(poisoned, cli.Label(b)) {
				missing = true
			}
		}
		if !missing {
			t.Fatal("removing one bucket's label from the poisoned copy did not trip the completeness check")
		}
	})
}

// idRow matches an INDENTED row's leading id — a child or an unparented
// entry, the two categories Tree.TotalCount sums. A group header (no
// indent) intentionally has the same "id  title" shape and must NOT match
// here: a header line is a label for an entry already counted once via
// Unparented (when it has no derives_from edge of its own) or is not
// counted again just for heading its own group — see BuildTree's doc
// comment on why TotalCount never adds anything for being a Group.Parent.
var idRow = regexp.MustCompile(`(?m)^  (int|dec|qst|cst|note)-[0-9]+\s`)

// countRenderedEntries counts indented, id-bearing rows in rendered text —
// the golden-file-level re-assertion of Tree.TotalCount's property, so a
// regression in T2's grouping that only manifests through the full render
// pipeline is caught here too.
func countRenderedEntries(text string) int {
	return len(idRow.FindAllString(text, -1))
}
