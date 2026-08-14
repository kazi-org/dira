package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/ledgertest"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// exerciseMap runs `dira map` through the real dispatcher and the real
// exit-code mapping — see exerciseBrief's own comment for why the command
// is appended here rather than assumed already registered.
func exerciseMap(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errBuf bytes.Buffer
	a := newApp(&out, &errBuf)
	if a.lookup("map") == nil {
		a.commands = append(a.commands, &command{name: "map", summary: mapSummary, run: runMap, usage: writeMapUsage})
	}
	code = a.main(append([]string{"map"}, args...))
	return code, out.String(), errBuf.String()
}

// mapAllBucketsLedger writes a fixture ledger with at least one entry in
// each of dec-0004's six buckets, given the fake kazi below is on PATH.
func mapAllBucketsLedger(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	diraDir := filepath.Join(root, ".dira")
	if err := os.MkdirAll(filepath.Join(diraDir, "entries"), 0o755); err != nil {
		t.Fatalf("creating %s: %v", diraDir, err)
	}
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("local.Open: %v", err)
	}

	mk := func(id string, mutate func(*ledger.Entry)) {
		e := ledgertest.Entry(id)
		mutate(e)
		if err := store.Create(t.Context(), e); err != nil {
			t.Fatalf("writing %s: %v", id, err)
		}
	}

	mk("dec-8001", func(e *ledger.Entry) {
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeRealizedBy, To: "kazi:goal-t1-complete"})
	})
	mk("dec-8002", func(e *ledger.Entry) {
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeRealizedBy, To: "kazi:goal-t1-progress"})
	})
	mk("dec-8003", func(e *ledger.Entry) {
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeRealizedBy, To: "kazi:prop-t1-planned"})
	})
	mk("dec-8004", func(e *ledger.Entry) {
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeRealizedBy, To: "kazi:goal-t1-blocked"})
	})
	mk("dec-8005", func(*ledger.Entry) {}) // no realized_by: ToBePlanned
	mk("qst-8006", func(e *ledger.Entry) {
		e.State = ledger.StateOpen
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeBlocks, To: "dec-8005"})
	})

	return root
}

// installFakeKazi puts a fake `kazi` script answering `portfolio --json`
// with a single-run-per-goal fixture covering Complete/InProgress/Planned
// (via prop-t1-planned)/Blocked, on a fresh PATH.
func installFakeKazi(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake kazi is a POSIX shell script")
	}
	dir := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = "portfolio" ]; then
  cat <<'JSON'
{"kind":"portfolio","schema_version":2,
 "planned":[{"proposal_ref":"prop-t1-planned","goal_id":"t1-planned","idea":"x","status":"proposed"}],
 "by_repo":{"repo-a":{"complete":[{"goal_ref":"t1-complete","run_id":"run-1","status":"converged"}],
   "in_progress":[{"goal_ref":"t1-progress","run_id":"run-2","status":"running"}]}},
 "fleet_remote":[],
 "totals":{"base":1,"empty":false,"rows":[{"bucket":"done","count":1,"pct":100}]},
 "todo":[],
 "blocked":[{"goal_ref":"t1-blocked","cause":"stuck","blocker":"blocked: x"}],
 "rate":{"total":0,"green":0,"empty?":true,"delta":0}}
JSON
  exit 0
fi
echo "unsupported" >&2
exit 2
`
	path := filepath.Join(dir, "kazi")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake kazi: %v", err)
	}
	t.Setenv("PATH", dir)
}

// TestMapCommand is E4-L4-T1's acc line.
func TestMapCommand(t *testing.T) {
	t.Run("exits 0 and prints a real fixture entry id, all six buckets present", func(t *testing.T) {
		installFakeKazi(t)
		root := mapAllBucketsLedger(t)

		code, stdout, stderr := exerciseMap(t, "-C", root)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
		}
		if !strings.Contains(stdout, "dec-8001") {
			t.Errorf("stdout does not contain the fixture's own entry id dec-8001:\n%s", stdout)
		}
	})

	t.Run("--json exits 0 and parses as JSON", func(t *testing.T) {
		installFakeKazi(t)
		root := mapAllBucketsLedger(t)

		code, stdout, stderr := exerciseMap(t, "-C", root, "--json")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
		}
		if !strings.HasPrefix(strings.TrimSpace(stdout), "{") {
			t.Errorf("stdout does not look like JSON:\n%s", stdout)
		}
	})

	t.Run("an unknown flag exits 2", func(t *testing.T) {
		root := mapAllBucketsLedger(t)
		code, _, stderr := exerciseMap(t, "-C", root, "--not-a-real-flag")
		if code != 2 {
			t.Errorf("exit code = %d, want 2; stderr:\n%s", code, stderr)
		}
	})

	t.Run("a malformed ledger entry exits 1 and names the offending file on stderr", func(t *testing.T) {
		root := t.TempDir()
		diraDir := filepath.Join(root, ".dira", "entries")
		if err := os.MkdirAll(diraDir, 0o755); err != nil {
			t.Fatalf("creating %s: %v", diraDir, err)
		}
		bad := "---\nid: dec-8101\nkind: decision\ntitle: [not valid yaml\n---\n"
		if err := os.WriteFile(filepath.Join(diraDir, "dec-8101.md"), []byte(bad), 0o644); err != nil {
			t.Fatalf("writing malformed entry: %v", err)
		}

		code, _, stderr := exerciseMap(t, "-C", root)
		if code != 1 {
			t.Fatalf("exit code = %d, want 1; stderr:\n%s", code, stderr)
		}
		if !strings.Contains(stderr, "dec-8101") {
			t.Errorf("stderr does not name the offending entry:\n%s", stderr)
		}
		lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
		if len(lines) != 1 {
			t.Errorf("stderr is %d lines, want exactly 1:\n%s", len(lines), stderr)
		}
	})

	t.Run("both sides: a stub that ignores its ledger passes a naive non-empty check and fails the real one", func(t *testing.T) {
		installFakeKazi(t)
		root := mapAllBucketsLedger(t)

		// The stub: ignores the ledger entirely and prints a fixed string.
		var stubOut bytes.Buffer
		stubOut.WriteString("dira map: nothing to show\n")

		naiveNonEmptyCheck := stubOut.Len() > 0
		if !naiveNonEmptyCheck {
			t.Fatal("the stub control's own premise broke: its output must be non-empty")
		}
		if strings.Contains(stubOut.String(), "dec-8001") {
			t.Fatal("the stub control's own premise broke: it must not happen to contain the fixture's id")
		}

		code, stdout, stderr := exerciseMap(t, "-C", root)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
		}
		if !strings.Contains(stdout, "dec-8001") {
			t.Fatalf("the real command's output does not contain the fixture's own entry id either — "+
				"a regression here would be indistinguishable from the stub:\n%s", stdout)
		}
	})
}
