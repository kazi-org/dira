package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/enforcer"
	"github.com/kazi-org/dira/internal/ledger"
)

// supersedeFixture is the ledger E3-L4's acceptance line names. It is the
// daemon fixture plus dec-0061, the entry that replaces dec-0060.
const supersedeFixture = "../../internal/enforcer/testdata/ledgers/supersede"

// The two plans the flip is measured with.
//
// daemonPlan is the one dec-0060 refuses; journalPlan proposes dec-0061's own
// first alternative back to it and is the fabricated conflict the acceptance
// line asks for. Neither is a near-miss: both are checked in both directions
// below, so a matcher that stopped detecting either would fail here rather than
// make this test quietly vacuous.
const (
	daemonPlan  = "add a background daemon to track run state"
	journalPlan = "replay an append-only journal at startup to rebuild run state"
)

// supersedeRegistryLine is the line cmd/dira/main.go must gain, and this is the
// test that says the line works.
//
// main.go belongs to the integrator, not to this lane, so the command is
// registered here instead — on the same *app, through the same registry field,
// dispatched by the same (*app).main. What that buys is that everything below
// exercises the real command surface including the exit-code mapping, so the
// only thing left unproven when the line lands is the line's own spelling:
//
//	{name: "supersede", summary: supersedeSummary, run: runSupersede, usage: writeSupersedeUsage},
func newSupersedeApp(t *testing.T, stdout, stderr *bytes.Buffer) *app {
	t.Helper()

	a := newApp(stdout, stderr)
	a.now = func() time.Time { return time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC) }
	if a.lookup("supersede") == nil {
		a.commands = append(a.commands, &command{
			name:    "supersede",
			summary: supersedeSummary,
			run:     runSupersede,
			usage:   writeSupersedeUsage,
		})
	}
	return a
}

// runCLI is one invocation of the whole binary: registry, dispatch, exit code.
//
// The arguments are written out in full, `-C` included and in the position a
// person would type it, because where it goes is not the same for every command
// — stdlib flag stops at the first non-flag argument, so `dira check` takes it
// before the plan and `dira supersede` after the id — and a helper that hid that
// would be testing a command line nobody can type.
func runCLI(t *testing.T, args ...string) result {
	t.Helper()

	var out, errBuf bytes.Buffer
	a := newSupersedeApp(t, &out, &errBuf)
	code := a.main(args)
	return result{code: code, stdout: out.String(), stderr: errBuf.String()}
}

// TestSupersedeFlipsWhatIsEnforced is the lane's acceptance clause, in one test,
// red to green across a single command.
//
// Every assertion here is made in both directions. The plan that conflicts
// before must not conflict after, *and* the plan that conflicts after must be
// shown conflicting — otherwise "no conflict" after the flip would be equally
// consistent with the check having stopped working, which is the failure this
// lane could most easily ship without noticing.
func TestSupersedeFlipsWhatIsEnforced(t *testing.T) {
	t.Parallel()

	root := fixtureLedgerFrom(t, supersedeFixture)
	entries := filepath.Join(root, ".dira", "entries")
	before := snapshot(t, entries)

	// Red: the daemon plan is refused, and dec-0060 is what refuses it.
	red := runCLI(t, "check", "-C", root, daemonPlan)
	if red.code != enforcer.ExitConflict {
		t.Fatalf("before the supersede, %q exited %d, want %d\n%s%s",
			daemonPlan, red.code, enforcer.ExitConflict, red.stdout, red.stderr)
	}
	if !strings.Contains(red.stdout, "✗ conflicts with dec-0060") {
		t.Fatalf("before the supersede, the citation is not dec-0060:\n%s", red.stdout)
	}

	// The command.
	got := runCLI(t, "supersede", "dec-0060", "--with", "dec-0061", "-C", root)
	if got.code != exitOK {
		t.Fatalf("`dira supersede dec-0060 --with dec-0061` exited %d\n%s%s", got.code, got.stdout, got.stderr)
	}

	// Green: the identical plan is now compliant, nothing cites dec-0060,
	// and no ✗ is printed at all.
	green := runCLI(t, "check", "-C", root, daemonPlan)
	if green.code != enforcer.ExitCompliant {
		t.Errorf("after the supersede, %q exited %d, want %d\n%s%s",
			daemonPlan, green.code, enforcer.ExitCompliant, green.stdout, green.stderr)
	}
	if strings.Contains(green.stdout, "✗") {
		t.Errorf("a superseded decision was still cited with a ✗:\n%s", green.stdout)
	}
	if out := green.stdout + green.stderr; strings.Contains(out, "dec-0060") {
		t.Errorf("the check still names the retired dec-0060:\n%s", out)
	}
	if !strings.Contains(green.stdout, "ⓘ") || !strings.Contains(green.stdout, "dec-0061") {
		t.Errorf("the check does not report the match and redirect it to dec-0061:\n%s", green.stdout)
	}

	// And the enforced entry has changed identity: a plan fabricated against
	// dec-0061's own alternatives is cited against dec-0061.
	fabricated := runCLI(t, "check", "-C", root, journalPlan)
	if fabricated.code != enforcer.ExitConflict {
		t.Errorf("after the supersede, %q exited %d, want %d\n%s%s",
			journalPlan, fabricated.code, enforcer.ExitConflict, fabricated.stdout, fabricated.stderr)
	}
	if !strings.Contains(fabricated.stdout, "✗ conflicts with dec-0061") {
		t.Errorf("the replacement is not what is cited now:\n%s", fabricated.stdout)
	}

	// The record, read back off disk rather than from what the command said.
	retired := readDecoded(t, root, "dec-0060")
	if retired.State != ledger.StateSuperseded {
		t.Errorf("dec-0060 reads state %q, want %q", retired.State, ledger.StateSuperseded)
	}
	if retired.Updated == "" {
		t.Error("dec-0060's updated was not bumped, so the record does not say when it was retired")
	}
	superseder := readDecoded(t, root, "dec-0061")
	if !hasEdge(superseder, ledger.EdgeSupersedes, "dec-0060") {
		t.Errorf("dec-0061 carries no supersedes edge to dec-0060: %+v", superseder.Edges)
	}
	if superseder.Updated == "" {
		t.Error("dec-0061's updated was not bumped, so the record does not say when it took over")
	}

	// Both files still satisfy the published contract, not dira's reading of it.
	validateAgainstSchema(t, readEntry(t, root, "dec-0060"))
	validateAgainstSchema(t, readEntry(t, root, "dec-0061"))

	// And nothing else in the ledger moved.
	changed := modifiedPaths(before, snapshot(t, entries))
	want := []string{"dec-0060.md", "dec-0061.md"}
	if !slices.Equal(changed, want) {
		t.Errorf("the supersede changed %v, want exactly %v", changed, want)
	}
}

// TestSupersedeChangesTwoLinesAndReflowsNothing is dec-0002's promise, applied
// to the one command that writes two files.
//
// dec-0002 exists so that "a PR touching a decision shows a legible diff". A
// mutation that re-wrapped the hand-written prose around it would technically
// record the same facts and destroy that property, and the destruction would be
// invisible to every other assertion in this file — all of which read the parsed
// entry, where a reflow does not show.
func TestSupersedeChangesTwoLinesAndReflowsNothing(t *testing.T) {
	t.Parallel()

	root := fixtureLedgerFrom(t, supersedeFixture)
	before := map[string][]string{
		"dec-0060": strings.Split(string(readEntry(t, root, "dec-0060")), "\n"),
		"dec-0061": strings.Split(string(readEntry(t, root, "dec-0061")), "\n"),
	}

	if got := runCLI(t, "supersede", "dec-0060", "--with", "dec-0061", "-C", root); got.code != exitOK {
		t.Fatalf("exit code = %d\n%s%s", got.code, got.stdout, got.stderr)
	}

	cases := map[string][]string{
		// The state line changes in place; updated is inserted.
		"dec-0060": {"-state: accepted", "+state: superseded", `+updated: "2026-07-30T09:00:00Z"`},
		// Three inserted lines for the edge, and updated.
		"dec-0061": {
			`+updated: "2026-07-30T09:00:00Z"`,
			"+edges:",
			"+  - type: supersedes",
			"+    to: dec-0060",
		},
	}
	for id, want := range cases {
		slices.Sort(want)
		after := strings.Split(string(readEntry(t, root, id)), "\n")
		got := lineDiff(before[id], after)
		if !slices.Equal(got, want) {
			t.Errorf("%s's diff is\n\t%s\nwant\n\t%s", id, strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
		}
	}
}

// TestSupersedeNeverLeavesTheTwoSidesDiverged is the invariant qst-0006 was
// opened about, asserted as an invariant rather than as an outcome.
//
// Three entries in this repository's own ledger once carried a supersedes edge
// whose target was still accepted — the record saying an entry had been
// replaced and that it was still in force at the same time. This walks the whole
// ledger after the command and checks both directions of the implication, and
// the negative control below proves the walk can fail.
func TestSupersedeNeverLeavesTheTwoSidesDiverged(t *testing.T) {
	t.Parallel()

	root := fixtureLedgerFrom(t, supersedeFixture)
	if got := runCLI(t, "supersede", "dec-0060", "--with", "dec-0061", "-C", root); got.code != exitOK {
		t.Fatalf("exit code = %d\n%s%s", got.code, got.stdout, got.stderr)
	}
	if diverged := divergence(t, root); len(diverged) != 0 {
		t.Errorf("after the supersede the record contradicts itself: %v", diverged)
	}

	// The negative control. The same walk over a ledger holding exactly the
	// half-finished state — the edge written, the state not — must report
	// it, or the assertion above is a check that cannot fail.
	half := fixtureLedgerFrom(t, supersedeFixture)
	seedEdgeOnly(t, half)
	if diverged := divergence(t, half); len(diverged) == 0 {
		t.Error("the divergence walk saw nothing wrong with an edge whose target is still accepted, " +
			"so it could not have detected one after the command either")
	}
}

// TestSupersedeFinishesAHalfFinishedFlip covers the failure mode this command
// had to choose rather than discover.
//
// The two writes are not atomic and cannot be (dec-0005: no transaction exists
// over the GitHub Contents API), so the order is chosen for what a crash between
// them leaves: the edge first, which leaves the retired entry still enforced —
// loud and safe — rather than unenforced with nothing recorded as replacing it.
// That choice is only honest if re-running the command repairs it, which is what
// this asserts against a ledger seeded in exactly that state.
func TestSupersedeFinishesAHalfFinishedFlip(t *testing.T) {
	t.Parallel()

	root := fixtureLedgerFrom(t, supersedeFixture)
	seedEdgeOnly(t, root)

	// The safety property of the chosen order: mid-flip, the old decision is
	// still enforced. A crash cannot open a door that was closed.
	mid := runCLI(t, "check", "-C", root, daemonPlan)
	if mid.code != enforcer.ExitConflict || !strings.Contains(mid.stdout, "dec-0060") {
		t.Errorf("a half-finished supersede stopped enforcing dec-0060; exit %d\n%s", mid.code, mid.stdout)
	}

	got := runCLI(t, "supersede", "dec-0060", "--with", "dec-0061", "-C", root)
	if got.code != exitOK {
		t.Fatalf("re-running the command did not finish the flip: exit %d\n%s%s", got.code, got.stdout, got.stderr)
	}
	if state := readDecoded(t, root, "dec-0060").State; state != ledger.StateSuperseded {
		t.Errorf("dec-0060 reads state %q after the repair run, want %q", state, ledger.StateSuperseded)
	}
	if after := runCLI(t, "check", "-C", root, daemonPlan); after.code != enforcer.ExitCompliant {
		t.Errorf("after the repair run, %q exited %d, want %d\n%s", daemonPlan, after.code, enforcer.ExitCompliant, after.stdout)
	}
}

// TestSupersedeRepeatedIsANoOp. `dira log` accepts being right twice because it
// runs unattended from hooks that fire more than once, and this command inherits
// the same reasoning from the repair path above: the second run after a crash
// and the second run after a success are the same invocation, and only one of
// them has anything left to do.
func TestSupersedeRepeatedIsANoOp(t *testing.T) {
	t.Parallel()

	root := fixtureLedgerFrom(t, supersedeFixture)
	entries := filepath.Join(root, ".dira", "entries")

	if got := runCLI(t, "supersede", "dec-0060", "--with", "dec-0061", "-C", root); got.code != exitOK {
		t.Fatalf("first run exited %d\n%s%s", got.code, got.stdout, got.stderr)
	}
	settled := snapshot(t, entries)

	again := runCLI(t, "supersede", "dec-0060", "--with", "dec-0061", "-C", root)
	if again.code != exitOK {
		t.Errorf("the second run exited %d, want %d\n%s%s", again.code, exitOK, again.stdout, again.stderr)
	}
	if changed := modifiedPaths(settled, snapshot(t, entries)); len(changed) != 0 {
		t.Errorf("the second run rewrote %v", changed)
	}
	if !strings.Contains(again.stderr, "nothing written") {
		t.Errorf("the second run did not say it wrote nothing: %q", again.stderr)
	}
}

// TestSupersedeRefusesAndWritesNothing is every refusal, with the same
// assertion attached to each: the ledger is byte-identical afterwards.
//
// A refusal that half-wrote is worse than no refusal, and the failure would be
// invisible in the exit code alone — which is why the digest is asserted for
// every row rather than for the interesting-looking ones.
func TestSupersedeRefusesAndWritesNothing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		// The record says no to a well-formed request: exit 2.
		{"a question", []string{"qst-0007", "--with", "dec-0061"}, exitUsage, "no superseded state"},
		{"an intent", []string{"int-0002", "--with", "dec-0061"}, exitUsage, "no superseded state"},
		{"across kinds", []string{"cst-0004", "--with", "dec-0061"}, exitUsage, "its own kind"},
		{"itself", []string{"dec-0060", "--with", "dec-0060"}, exitUsage, "cannot supersede itself"},
		{"a staged replacement", []string{"dec-0060", "--with", "dec-0075"}, exitUsage, "is staged"},

		// dira could not act on the command line at all: exit 1.
		{"no replacement", []string{"dec-0060"}, exitError, "needs the entry that replaces"},
		{"no target", []string{"--with", "dec-0061"}, exitError, "needs the entry to retire"},
		{"not an id", []string{"dec-60", "--with", "dec-0061"}, exitError, "not an entry id"},
		{"unknown flag", []string{"dec-0060", "--with", "dec-0061", "--force"}, exitError, "flag provided but not defined"},
		{"target after the flags", []string{"--with", "dec-0061", "dec-0060"}, exitError, "goes before the flags"},
		{"an entry that is not there", []string{"dec-0099", "--with", "dec-0061"}, exitError, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := fixtureLedgerFrom(t, supersedeFixture)
			entries := filepath.Join(root, ".dira", "entries")
			before := snapshot(t, entries)

			args := append([]string{"supersede"}, tc.args...)
			got := runCLI(t, append(args, "-C", root)...)
			if got.code != tc.code {
				t.Errorf("exit code = %d, want %d — %s\n%s%s", got.code, tc.code,
					map[int]string{exitUsage: "the record refused this", exitError: "dira could not act on this"}[tc.code],
					got.stdout, got.stderr)
			}
			if tc.want != "" && !strings.Contains(refusal(got.stderr), tc.want) {
				t.Errorf("the refusal does not say why (want %q):\n%s", tc.want, got.stderr)
			}
			if got.stdout != "" {
				t.Errorf("a refusal wrote to stdout: %q", got.stdout)
			}
			if changed := modifiedPaths(before, snapshot(t, entries)); len(changed) != 0 {
				t.Errorf("a refusal wrote %v", changed)
			}
		})
	}
}

// TestSupersedeRefusesASecondSuperseder. An entry is replaced once and by one
// entry; two supersedes edges pointing at it is the record telling two stories
// again, and the check would have to pick one to redirect to.
func TestSupersedeRefusesASecondSuperseder(t *testing.T) {
	t.Parallel()

	root := fixtureLedgerFrom(t, supersedeFixture)
	if got := runCLI(t, "supersede", "dec-0060", "--with", "dec-0061", "-C", root); got.code != exitOK {
		t.Fatalf("the first supersede exited %d\n%s%s", got.code, got.stdout, got.stderr)
	}
	entries := filepath.Join(root, ".dira", "entries")
	settled := snapshot(t, entries)

	got := runCLI(t, "supersede", "dec-0060", "--with", "dec-0083", "-C", root)
	if got.code != exitUsage {
		t.Errorf("a second superseder exited %d, want %d — the record refused it\n%s%s",
			got.code, exitUsage, got.stdout, got.stderr)
	}
	if !strings.Contains(refusal(got.stderr), "already superseded") {
		t.Errorf("the refusal does not name the reason:\n%s", got.stderr)
	}
	if changed := modifiedPaths(settled, snapshot(t, entries)); len(changed) != 0 {
		t.Errorf("the refused second supersede wrote %v", changed)
	}
}

// TestSupersedeNeverWritesToAParentLedger is a security test (cst-0003 rule 1),
// so it is asserted against the parent's bytes rather than against the exit
// code.
//
// `dira supersede me:cst-0002 --with cst-0005` is refused in both readings of
// what it could mean: retiring a parent's entry would write that parent's
// `state`, and being replaced by a parent's entry would write the edge onto the
// parent's file. Both are upward writes, and dira has none.
func TestSupersedeNeverWritesToAParentLedger(t *testing.T) {
	t.Parallel()

	child, parent := ledgerWithParent(t)
	childEntries := filepath.Join(child, ".dira", "entries")
	beforeChild := snapshot(t, childEntries)
	beforeParent := treeSHA(t, parent)

	for _, args := range [][]string{
		{"supersede", "me:cst-0002", "--with", "cst-0005"},
		{"supersede", "cst-0005", "--with", "me:cst-0002"},
	} {
		got := runCLI(t, append(slices.Clone(args), "-C", child)...)
		if got.code != exitUsage {
			t.Errorf("`dira %s` exited %d, want %d — a cross-ledger supersede is refused on policy, "+
				"and a caller must be able to tell that from dira failing to run\n%s%s",
				strings.Join(args, " "), got.code, exitUsage, got.stdout, got.stderr)
		}
		if !strings.Contains(refusal(got.stderr), "cst-0003") {
			t.Errorf("the refusal does not name the constraint it enforces:\n%s", got.stderr)
		}
		if after := treeSHA(t, parent); after != beforeParent {
			t.Fatalf("the parent ledger changed\n  before %s\n  after  %s", beforeParent, after)
		}
		if changed := modifiedPaths(beforeChild, snapshot(t, childEntries)); len(changed) != 0 {
			t.Errorf("the refusal wrote %v in the child ledger", changed)
		}
	}

	// The digest is only evidence if it moves when those bytes do, and the
	// way to show that is to make the very write cst-0003 forbids and watch
	// it register. Run from inside the parent this is a legitimate local
	// supersession; run from the child it would be the violation. Same
	// bytes, same digest, so the assertions above are checks that can fail.
	seedEntry(t, parent, "cst-0009", minimalEntry("cst-0009", "constraint", "active"))
	withFile := treeSHA(t, parent)
	if withFile == beforeParent {
		t.Fatal("the parent digest did not change after a file was added to it, so it proves nothing")
	}
	if got := runCLI(t, "supersede", "cst-0002", "--with", "cst-0009", "-C", parent); got.code != exitOK {
		t.Fatalf("superseding inside the parent exited %d\n%s%s", got.code, got.stdout, got.stderr)
	}
	if treeSHA(t, parent) == withFile {
		t.Error("the parent digest did not change after an entry in it was superseded, " +
			"so the byte-identity assertions above could not have caught an upward write")
	}
}

// TestSupersedeHelpGoesToStdout. `dira supersede -h` and `dira help supersede`
// print the same text, and asking for help is an answer rather than a failure.
func TestSupersedeHelpGoesToStdout(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"supersede", "-h"}, {"help", "supersede"}} {
		got := runCLI(t, args...)
		if got.code != exitOK {
			t.Errorf("`dira %s` exited %d, want 0", strings.Join(args, " "), got.code)
		}
		for _, want := range []string{
			"usage:", "--with ID", "cst-0003", "The edge is written first",
			"exit codes:", "the ledger refuses it", "refused on policy", "1 is never a verdict",
		} {
			if !strings.Contains(got.stdout, want) {
				t.Errorf("`dira %s` does not mention %q:\n%s", strings.Join(args, " "), want, got.stdout)
			}
		}
	}
}

// TestSupersedeRedirectReachesTheJSON. A hook parses --json, and a redirect that
// existed only in the human output would be invisible to exactly the caller the
// pre-plan seam is built for.
func TestSupersedeRedirectReachesTheJSON(t *testing.T) {
	t.Parallel()

	root := fixtureLedgerFrom(t, supersedeFixture)
	if got := runCLI(t, "supersede", "dec-0060", "--with", "dec-0061", "-C", root); got.code != exitOK {
		t.Fatalf("exit code = %d\n%s%s", got.code, got.stdout, got.stderr)
	}

	got := runCLI(t, "check", "-C", root, "-json", daemonPlan)
	if got.code != enforcer.ExitCompliant {
		t.Fatalf("exit code = %d, want %d\n%s", got.code, enforcer.ExitCompliant, got.stdout)
	}

	var doc struct {
		Verdict   string `json:"verdict"`
		Conflicts []struct {
			Entry string `json:"entry"`
		} `json:"conflicts"`
		Notices []struct {
			SupersededBy *string `json:"superseded_by"`
			Enforced     bool    `json:"replacement_enforced"`
		} `json:"notices"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, got.stdout)
	}
	if doc.Verdict != "compliant" || len(doc.Conflicts) != 0 {
		t.Errorf("the document is not a compliant verdict: %s", got.stdout)
	}
	if len(doc.Notices) != 1 {
		t.Fatalf("the document carries %d notices, want 1: %s", len(doc.Notices), got.stdout)
	}
	if doc.Notices[0].SupersededBy == nil || *doc.Notices[0].SupersededBy != "dec-0061" {
		t.Errorf("the notice does not redirect to dec-0061: %s", got.stdout)
	}
	if !doc.Notices[0].Enforced {
		t.Errorf("the notice says the replacement is not enforced, but dec-0061 is accepted: %s", got.stdout)
	}
	if strings.Contains(got.stdout, "dec-0060") {
		t.Errorf("the machine surface still names the retired entry: %s", got.stdout)
	}
}

// TestTheEdgeIsWrittenBeforeTheState is the failure mode, decided rather than
// discovered.
//
// Two files change and there is no transaction to change them in — ledger.Store
// has neither, because neither exists over the GitHub Contents API (dec-0005).
// So the order is the whole answer, and it is only observable when the second
// write fails. Both halves are asserted here: that the edge goes first, and that
// a failure after it leaves the retired entry still enforced rather than
// silently unenforced.
func TestTheEdgeIsWrittenBeforeTheState(t *testing.T) {
	t.Parallel()

	root := fixtureLedgerFrom(t, supersedeFixture)
	store, _, err := openLedger(root)
	if err != nil {
		t.Fatalf("opening the fixture ledger: %v", err)
	}

	ctx := context.Background()
	retired, err := store.Get(ctx, "dec-0060")
	if err != nil {
		t.Fatalf("reading dec-0060: %v", err)
	}
	superseder, err := store.Get(ctx, "dec-0061")
	if err != nil {
		t.Fatalf("reading dec-0061: %v", err)
	}

	// The second write fails, exactly as a crash, a full disk or a rejected
	// PUT would make it fail.
	failing := &recordingStore{Store: store, failOn: "dec-0060"}
	var out, errBuf bytes.Buffer
	a := newSupersedeApp(t, &out, &errBuf)
	if _, err := a.writeSupersession(ctx, failing, retired, superseder, ""); err == nil {
		t.Fatal("the injected failure did not surface; the rest of this test would prove nothing")
	} else if !strings.Contains(err.Error(), "Run the same command again") {
		t.Errorf("the error does not tell the caller how to finish:\n%v", err)
	}

	if want := []string{"dec-0061", "dec-0060"}; !slices.Equal(failing.puts, want) {
		t.Fatalf("the writes went in the order %v, want %v — the edge is written first so that an "+
			"interruption leaves the retired entry still enforced", failing.puts, want)
	}

	// What the half-finished ledger enforces: still the old decision. The
	// other order would have left it unenforced with nothing recorded as
	// replacing it, which is the silent failure this order exists to avoid.
	mid := runCLI(t, "check", "-C", root, daemonPlan)
	if mid.code != enforcer.ExitConflict || !strings.Contains(mid.stdout, "dec-0060") {
		t.Errorf("after a failed second write, dec-0060 is no longer enforced; exit %d\n%s", mid.code, mid.stdout)
	}
	if !hasEdge(readDecoded(t, root, "dec-0061"), ledger.EdgeSupersedes, "dec-0060") {
		t.Error("the first write did not land, so the failure was injected in the wrong place")
	}
}

// recordingStore is a Store that records the order of writes and can fail one.
type recordingStore struct {
	ledger.Store

	failOn string
	puts   []string
}

func (s *recordingStore) Put(ctx context.Context, e *ledger.Entry) error {
	s.puts = append(s.puts, e.ID)
	if e.ID == s.failOn {
		return errors.New("injected write failure")
	}
	return s.Store.Put(ctx, e)
}

// ---- helpers --------------------------------------------------------------

// refusal is the message a refusal led with, without the usage block printed
// under it.
//
// Asserting against the whole of stderr would be a check that cannot fail:
// `dira supersede`'s usage text names cst-0003, questions and the write order
// itself, so every substring worth asserting is already down there whatever the
// command actually said. Splitting on the blank line (*app).main writes between
// the two is what makes these assertions about the reason given.
func refusal(stderr string) string {
	message, _, _ := strings.Cut(stderr, "\n\n")
	return message
}

// readDecoded reads an entry file back through dira's own codec.
func readDecoded(t *testing.T, root, id string) *ledger.Entry {
	t.Helper()

	e, err := ledger.Decode(readEntry(t, root, id))
	if err != nil {
		t.Fatalf("decoding %s after the command wrote it: %v", id, err)
	}
	return e
}

func hasEdge(e *ledger.Entry, edgeType ledger.EdgeType, to string) bool {
	for _, edge := range e.Edges {
		if edge.Type == edgeType && edge.To == to {
			return true
		}
	}
	return false
}

// divergence reports every place the ledger's edges and states disagree about
// what has been superseded, in both directions.
func divergence(t *testing.T, root string) []string {
	t.Helper()

	entries := map[string]*ledger.Entry{}
	dir := filepath.Join(root, ".dira", "entries")
	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, name := range names {
		if filepath.Ext(name.Name()) != ".md" {
			continue
		}
		id := strings.TrimSuffix(name.Name(), ".md")
		entries[id] = readDecoded(t, root, id)
	}

	claimed := map[string]bool{}
	var out []string
	for _, e := range entries {
		for _, edge := range e.Edges {
			if edge.Type != ledger.EdgeSupersedes {
				continue
			}
			claimed[edge.To] = true
			target, ok := entries[edge.To]
			if !ok {
				continue
			}
			if target.State != ledger.StateSuperseded {
				out = append(out, e.ID+" supersedes "+target.ID+", which still reads state "+string(target.State))
			}
		}
	}
	for id, e := range entries {
		if e.State == ledger.StateSuperseded && !claimed[id] {
			out = append(out, id+" is superseded and nothing records what replaced it")
		}
	}
	slices.Sort(out)
	return out
}

// seedEdgeOnly puts a ledger into the state a crash between the two writes
// leaves: dec-0061 records that it supersedes dec-0060, and dec-0060 has not
// been flipped.
//
// It writes the file through dira's own codec rather than by patching text, so
// the seeded state is one the command itself could have produced.
func seedEdgeOnly(t *testing.T, root string) {
	t.Helper()

	e := readDecoded(t, root, "dec-0061")
	e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeSupersedes, To: "dec-0060"})
	e.Updated = "2026-07-30T08:00:00Z"
	data, err := ledger.Encode(e)
	if err != nil {
		t.Fatalf("encoding the half-finished dec-0061: %v", err)
	}
	seedEntry(t, root, "dec-0061", string(data))
}

// ledgerWithParent builds a child ledger whose config declares a private parent,
// and the parent itself, and returns both roots.
func ledgerWithParent(t *testing.T) (child, parent string) {
	t.Helper()

	parent = ledgerRoot(t)
	seedEntry(t, parent, "cst-0002", "---\nid: cst-0002\nkind: constraint\n"+
		"title: Never run more than one side project at a time\nstate: active\n"+
		"created: \"2026-06-01T09:00:00Z\"\n---\n\nSENTINEL-PRIVATE-TEXT\n")
	if err := os.WriteFile(filepath.Join(parent, ".dira", "config.toml"),
		[]byte("[ledger]\nname = \"me\"\ntier = \"person\"\n"), 0o644); err != nil {
		t.Fatalf("writing the parent config: %v", err)
	}

	child = ledgerRoot(t)
	seedEntry(t, child, "cst-0005", minimalEntry("cst-0005", "constraint", "active"))
	config := "[ledger]\nname = \"child\"\ntier = \"repo\"\n\n[parents]\nme = { path = " +
		strconv.Quote(parent) + ", ref = \"main\" }\n"
	if err := os.WriteFile(filepath.Join(child, ".dira", "config.toml"), []byte(config), 0o644); err != nil {
		t.Fatalf("writing the child config: %v", err)
	}
	return child, parent
}

// treeSHA is one digest over every file under root, path and contents.
func treeSHA(t *testing.T, root string) string {
	t.Helper()

	sum := sha256.New()
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	slices.Sort(paths)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		sum.Write([]byte(path))
		sum.Write([]byte{0})
		sum.Write(data)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// lineDiff is the set of lines added and removed between two files, as `+line`
// and `-line`, sorted. It is a set difference rather than a real diff: what it
// has to answer is "which lines are not in both", which is exactly the question
// a reflow fails.
func lineDiff(before, after []string) []string {
	count := map[string]int{}
	for _, line := range before {
		count[line]--
	}
	for _, line := range after {
		count[line]++
	}

	var out []string
	for line, n := range count {
		switch {
		case n > 0:
			out = append(out, "+"+line)
		case n < 0:
			out = append(out, "-"+line)
		}
	}
	slices.Sort(out)
	return out
}
