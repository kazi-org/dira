package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/enforcer"
)

// The fixture E3-L1 froze, and the demo block E8 records.
const (
	daemonFixture = "../../internal/enforcer/testdata/ledgers/daemon"
	daemonGolden  = "../../internal/enforcer/testdata/golden/daemon.txt"
)

// exitCodeOf is cmd/dira's error mapping, applied to what runCheck returned.
//
// It mirrors (*app).main, and it mirrors it because `check` is not registered
// in newApp's command table yet — E3-L2 does not own main.go, and the registry
// line plus the five-line ExitCode branch below are wired by whoever merges
// this lane. The branch main.go needs, verbatim, immediately after its
// `if err == nil` return:
//
//	// A command that has already rendered its own verdict selects its exit
//	// code directly, and nothing more is printed: the verdict is the output.
//	var coded interface{ ExitCode() int }
//	if errors.As(err, &coded) {
//		return coded.ExitCode()
//	}
//
// Until that lands, this function is the thing under test rather than the real
// mapping, and these tests say so rather than implying otherwise.
func exitCodeOf(err error) int {
	if err == nil {
		return exitOK
	}
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	var ue *usageError
	if errors.As(err, &ue) {
		return exitUsage
	}
	return exitError
}

// runner is one invocation of `dira check`, captured.
type runner struct {
	stdout, stderr bytes.Buffer
	app            *app
}

func newRunner(t *testing.T) *runner {
	t.Helper()

	r := &runner{}
	r.app = newApp(&r.stdout, &r.stderr)
	return r
}

func (r *runner) run(args ...string) int { return exitCodeOf(runCheck(r.app, args)) }

// fixtureLedger copies the frozen fixture entries into a fresh .dira under
// t.TempDir and returns the directory to run `dira check -C` from.
//
// The fixture entries live as a flat directory of markdown files, because that
// is the shape internal/enforcer/testdata/corpus.yaml references them in and
// the corpus is checksummed. A ledger dira can open is a .dira with an entries/
// inside it, so the two shapes do not coincide, and the fixture is the one that
// cannot move.
//
// Copying rather than teaching the check to read a directory of loose entry
// files is the whole point: this way the command runs through the real read
// path — local.Find walking up for a .dira, the same store, the same index —
// instead of growing the second reader E3's lane file forbids. The copy also
// makes the read-only assertion in TestCheckLeavesTheLedgerAlone safe to make
// against a fixture the corpus is checksummed against.
//
// Files are copied byte for byte: nothing here parses or re-encodes an entry.
func fixtureLedger(t *testing.T) string {
	t.Helper()

	return fixtureLedgerFrom(t, daemonFixture)
}

// fixtureLedgerFrom is fixtureLedger over any of the frozen fixture ledgers.
// E3-L4 needs the same materialisation for testdata/ledgers/supersede, and a
// second copy of it would be a second place for the "no entry files" guard
// below to be forgotten.
func fixtureLedgerFrom(t *testing.T, fixture string) string {
	t.Helper()

	names, err := os.ReadDir(fixture)
	if err != nil {
		t.Fatalf("reading the fixture ledger %s: %v", fixture, err)
	}

	root := t.TempDir()
	entries := filepath.Join(root, ".dira", "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatalf("creating %s: %v", entries, err)
	}

	copied := 0
	for _, name := range names {
		if name.IsDir() || filepath.Ext(name.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(fixture, name.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", name.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(entries, name.Name()), data, 0o644); err != nil {
			t.Fatalf("writing %s: %v", name.Name(), err)
		}
		copied++
	}
	if copied == 0 {
		// A fixture that materialised nothing would make every test over it
		// pass by having nothing to contradict, which is the vacuous-green
		// failure this lane exists to avoid.
		t.Fatalf("no entry files in %s; the fixture ledger is empty", fixture)
	}
	return root
}

// TestCheckExitsTwoAndMatchesTheGolden is the lane's headline predicate.
//
// The block on stdout is the asset .agents/product-marketing.md §6 pre-registers
// for E8's launch clip and that README.md and docs/design.md §7 both print, so
// it is compared byte for byte rather than by substring. The exit code is
// checked alongside it because a demo that prints the right words and exits 0
// has not caught anything.
func TestCheckExitsTwoAndMatchesTheGolden(t *testing.T) {
	t.Parallel()

	want, err := os.ReadFile(daemonGolden)
	if err != nil {
		t.Fatalf("reading %s: %v", filepath.Clean(daemonGolden), err)
	}

	r := newRunner(t)
	code := r.run("-C", fixtureLedger(t), "add a background daemon to track run state")

	if code != enforcer.ExitConflict {
		t.Errorf("exit code is %d, want %d (a cited conflict)", code, enforcer.ExitConflict)
	}
	if r.stdout.String() != string(want) {
		t.Errorf("stdout does not match %s byte for byte.\n--- got ---\n%s\n--- want ---\n%s",
			daemonGolden, r.stdout.String(), want)
	}
	if r.stderr.Len() != 0 {
		t.Errorf("a conflict wrote to stderr, which a caller parsing stdout should never need to read:\n%s", r.stderr.String())
	}
}

// TestCheckExitsZeroOnACompliantPlan covers the half of the contract that makes
// the other half worth having.
func TestCheckExitsZeroOnACompliantPlan(t *testing.T) {
	t.Parallel()

	r := newRunner(t)
	code := r.run("-C", fixtureLedger(t), "write the checkpoint file atomically")

	if code != enforcer.ExitCompliant {
		t.Errorf("exit code is %d, want %d\nstdout:\n%s\nstderr:\n%s",
			code, enforcer.ExitCompliant, r.stdout.String(), r.stderr.String())
	}
	if strings.Contains(r.stdout.String(), "✗") {
		t.Errorf("a compliant plan printed a ✗:\n%s", r.stdout.String())
	}
}

// TestCheckJSONCarriesTheVerdict checks the machine surface end to end. The
// document's own conformance to schema/check.schema.json is asserted in
// internal/enforcer, which is where the schema is compiled; what is asserted
// here is that the flag reaches it and that the code on the process agrees with
// the code in the document.
func TestCheckJSONCarriesTheVerdict(t *testing.T) {
	t.Parallel()

	ledgerDir := fixtureLedger(t)
	cases := []struct {
		name string
		plan string
		want int
	}{
		{"conflict", "add a background daemon to track run state", enforcer.ExitConflict},
		{"compliant", "write the checkpoint file atomically", enforcer.ExitCompliant},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRunner(t)
			code := r.run("-C", ledgerDir, "-json", tc.plan)

			var doc struct {
				Plan      string `json:"plan"`
				Verdict   string `json:"verdict"`
				ExitCode  int    `json:"exit_code"`
				Conflicts []struct {
					Entry string `json:"entry"`
				} `json:"conflicts"`
			}
			if err := json.Unmarshal(r.stdout.Bytes(), &doc); err != nil {
				t.Fatalf("stdout is not JSON: %v\n%s", err, r.stdout.String())
			}
			if code != tc.want {
				t.Errorf("exit code is %d, want %d", code, tc.want)
			}
			if doc.ExitCode != code {
				t.Errorf("the document says exit_code %d and the process exited %d", doc.ExitCode, code)
			}
			if doc.Plan != tc.plan {
				t.Errorf("the document reports the plan as %q, want %q", doc.Plan, tc.plan)
			}
			if tc.want == enforcer.ExitConflict && (len(doc.Conflicts) == 0 || doc.Conflicts[0].Entry != "dec-0060") {
				t.Errorf("the document does not cite dec-0060: %+v", doc.Conflicts)
			}
		})
	}
}

// TestCheckSeparatesItsOwnErrorsFromAVerdict is the exit-code contract's whole
// point, tested as a table because every one of these lands on 1 and a hook has
// to be able to trust that.
//
// This is where `dira check` differs deliberately from the rest of the binary:
// the top-level contract maps a usage mistake onto 2, and here 2 already means
// "you contradicted yourself". A hook that read a mistyped flag as a
// contradiction would block a commit for a reason that has nothing to do with
// the ledger.
func TestCheckSeparatesItsOwnErrorsFromAVerdict(t *testing.T) {
	t.Parallel()

	ledgerDir := fixtureLedger(t)
	cases := []struct {
		name string
		args []string
	}{
		{"unknown flag", []string{"-C", ledgerDir, "-nosuchflag", "a plan"}},
		{"no plan", []string{"-C", ledgerDir}},
		{"two plans", []string{"-C", ledgerDir, "one plan", "another plan"}},
		{"empty plan", []string{"-C", ledgerDir, "   "}},
		{"no ledger", []string{"-C", t.TempDir(), "add a background daemon to track run state"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRunner(t)
			code := r.run(tc.args...)

			if code != enforcer.ExitDiraError {
				t.Errorf("exit code is %d, want %d — a caller must be able to tell this from a conflict",
					code, enforcer.ExitDiraError)
			}
			if r.stdout.Len() != 0 {
				t.Errorf("dira's own failure wrote to stdout, where a caller looks for a verdict:\n%s", r.stdout.String())
			}
		})
	}
}

// TestCheckHelpGoesToStdoutAndExitsZero: help was asked for, so it is the
// answer rather than a failure, and it must not land on a verdict code.
func TestCheckHelpGoesToStdoutAndExitsZero(t *testing.T) {
	t.Parallel()

	r := newRunner(t)
	if code := r.run("-h"); code != exitOK {
		t.Errorf("`dira check -h` exited %d, want %d", code, exitOK)
	}
	for _, want := range []string{"usage:", "-json", "exit codes:", "2  at least one cited conflict"} {
		if !strings.Contains(r.stdout.String(), want) {
			t.Errorf("the help does not mention %q:\n%s", want, r.stdout.String())
		}
	}
}

// TestCheckLeavesTheLedgerAlone. The check is a read, and the only thing it may
// create is the derived cache dec-0002 already calls disposable. An entry file
// that changed during a check would be a check that edits the record it is
// enforcing.
func TestCheckLeavesTheLedgerAlone(t *testing.T) {
	t.Parallel()

	root := fixtureLedger(t)
	entries := filepath.Join(root, ".dira", "entries")
	before := treeDigest(t, entries)

	r := newRunner(t)
	if code := r.run("-C", root, "add a background daemon to track run state"); code != enforcer.ExitConflict {
		t.Fatalf("exit code is %d, want %d", code, enforcer.ExitConflict)
	}

	if after := treeDigest(t, entries); after != before {
		t.Errorf("the entries directory changed during a check:\n  before %s\n  after  %s", before, after)
	}
}

// treeDigest summarises a directory's files and their contents.
func treeDigest(t *testing.T, dir string) string {
	t.Helper()

	var b strings.Builder
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.WriteString(path)
		b.WriteString(":")
		b.WriteString(string(data))
		b.WriteString("\n")
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return b.String()
}
