package enforcer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

// supersedeLedger is the fixture E3-L4's acceptance line names: the daemon
// ledger plus dec-0061, the entry that replaces dec-0060.
const supersedeLedger = "testdata/ledgers/supersede"

// retiredID is the entry the flip retires. It is a constant because half the
// assertions in this file are that this string does not appear in output, and a
// sentinel written out by hand in six places is a sentinel that gets one of them
// wrong.
const retiredID = "dec-0060"

// flipped is the supersede fixture with the supersession applied: dec-0060
// superseded, dec-0061 carrying the edge.
//
// The flip is applied to the decoded entries rather than to files on disk
// because what is under test here is the matcher's reading of a ledger in that
// state. That the command produces exactly this state on disk, through the real
// codec, is asserted in cmd/dira/supersede_test.go — the two halves are tested
// where each of them lives, and neither stands in for the other.
func flipped(t *testing.T, replacementState ledger.State, withEdge bool) []*ledger.Entry {
	t.Helper()

	entries := fixtureEntries(t, supersedeLedger)
	for _, e := range entries {
		switch e.ID {
		case retiredID:
			e.State = ledger.StateSuperseded
		case "dec-0061":
			e.State = replacementState
			if withEdge {
				e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeSupersedes, To: retiredID})
			}
		}
	}
	return entries
}

// TestASupersededDecisionIsReportedAndNotCited is the enforcement table's
// superseded row, which is the only row that produces output without producing
// a verdict.
func TestASupersededDecisionIsReportedAndNotCited(t *testing.T) {
	t.Parallel()

	before := check(demoPlan, fixtureEntries(t, supersedeLedger))
	if before.Compliant() || before.Conflicts[0].Entry != retiredID {
		t.Fatalf("the fixture does not cite %s before the flip: %+v", retiredID, before.Conflicts)
	}
	if len(before.Notices) != 0 {
		t.Errorf("an accepted decision produced a notice: %+v", before.Notices)
	}

	after := check(demoPlan, flipped(t, ledger.StateAccepted, true))
	if !after.Compliant() {
		t.Errorf("a superseded decision was still cited: %+v", after.Conflicts)
	}
	if after.ExitCode() != ExitCompliant {
		t.Errorf("exit code is %d, want %d — a notice is not a verdict", after.ExitCode(), ExitCompliant)
	}
	if len(after.Notices) != 1 {
		t.Fatalf("the match against the retired entry was not reported: %+v", after.Notices)
	}
	if got := after.Notices[0].SupersededBy; got != "dec-0061" {
		t.Errorf("the notice redirects to %q, want dec-0061", got)
	}
	if !after.Notices[0].Enforced {
		t.Error("the notice says dec-0061 is not enforced, but it is an accepted decision")
	}

	// The count is of what can be cited, not of what is on disk. Counting the
	// retired entry would make "no conflict with 7 enforced entries" a claim
	// about a file rather than about the firewall.
	if after.Enforced != before.Enforced-1 {
		t.Errorf("the check reports %d enforced entries after the flip and %d before; "+
			"a superseded entry is not enforced", after.Enforced, before.Enforced)
	}
}

// TestANoticeNeverNamesTheRetiredEntry.
//
// The redirect names the replacement and nothing else, in both modes. This is a
// contract assertion rather than a formatting preference: E3-L4's acceptance
// line requires that after the flip "no output cites dec-0060", and the
// information is not lost — dec-0061 carries the `supersedes` edge back, so
// `dira why dec-0061` still reaches it.
func TestANoticeNeverNamesTheRetiredEntry(t *testing.T) {
	t.Parallel()

	v := check(demoPlan, flipped(t, ledger.StateAccepted, true))
	if len(v.Notices) == 0 {
		t.Fatal("no notice was produced, so this test would pass against any renderer")
	}

	var human, machine bytes.Buffer
	if err := Render(&human, v); err != nil {
		t.Fatalf("rendering: %v", err)
	}
	if err := RenderJSON(&machine, v); err != nil {
		t.Fatalf("rendering json: %v", err)
	}

	for name, out := range map[string]string{"human": human.String(), "json": machine.String()} {
		if strings.Contains(out, retiredID) {
			t.Errorf("%s output names the retired entry:\n%s", name, out)
		}
		if !strings.Contains(out, "dec-0061") {
			t.Errorf("%s output does not name the replacement at all:\n%s", name, out)
		}
		if strings.Contains(out, "✗") {
			t.Errorf("%s output printed a ✗ for a superseded entry:\n%s", name, out)
		}
	}
	if !strings.HasPrefix(human.String(), "ⓘ ") {
		t.Errorf("the notice is not the first line of the human output:\n%s", human.String())
	}
	validate(t, machine.Bytes())
}

// TestTheRedirectSaysOnlyWhatTheRecordSupports covers the two shapes a redirect
// can take that a one-form message would have to lie about.
//
// Both are states a hand-edited ledger reaches and `dira supersede` refuses to
// create: a decision flipped to `superseded` with no `supersedes` edge pointing
// at it — the exact inconsistency qst-0006 found in this repository's own
// ledger — and a replacement that is itself not enforcement substrate. Sending a
// reader to an entry that will not stop them either is worse than saying so.
func TestTheRedirectSaysOnlyWhatTheRecordSupports(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		state    ledger.State
		withEdge bool
		by       string
		enforced bool
		want     string
	}{
		{
			name: "no replacement recorded", state: ledger.StateAccepted, withEdge: false,
			by: "", enforced: false,
			want: "the ledger records nothing that replaced it",
		},
		{
			name: "the replacement is staged", state: ledger.StateStaged, withEdge: true,
			by: "dec-0061", enforced: false,
			want: "dec-0061 is not enforced either",
		},
		{
			name: "the replacement is enforced", state: ledger.StateAccepted, withEdge: true,
			by: "dec-0061", enforced: true,
			want: "dec-0061 is enforced in its place",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := check(demoPlan, flipped(t, tc.state, tc.withEdge))
			if len(v.Notices) != 1 {
				t.Fatalf("got %d notices, want 1: %+v", len(v.Notices), v.Notices)
			}
			n := v.Notices[0]
			if n.SupersededBy != tc.by || n.Enforced != tc.enforced {
				t.Errorf("notice is {by:%q enforced:%v}, want {by:%q enforced:%v}",
					n.SupersededBy, n.Enforced, tc.by, tc.enforced)
			}

			var out bytes.Buffer
			if err := Render(&out, v); err != nil {
				t.Fatalf("rendering: %v", err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Errorf("the line does not say %q:\n%s", tc.want, out.String())
			}

			var machine bytes.Buffer
			if err := RenderJSON(&machine, v); err != nil {
				t.Fatalf("rendering json: %v", err)
			}
			validate(t, machine.Bytes())
		})
	}
}

// TestTheSchemaRejectsAnUnbackedRedirect is the negative half of the notice
// schema. A document claiming a replacement is enforced while naming none is
// the one shape of notice that is self-contradictory, and a schema that
// accepted it would be describing nothing.
func TestTheSchemaRejectsAnUnbackedRedirect(t *testing.T) {
	t.Parallel()

	doc := `{
	  "plan": "add a background daemon",
	  "verdict": "compliant",
	  "exit_code": 0,
	  "enforced_entries": 6,
	  "conflicts": [],
	  "notices": [{
	    "superseded_by": null,
	    "replacement_enforced": true,
	    "basis": "alternative",
	    "score": 1
	  }]
	}`

	if err := validateErr(t, []byte(doc)); err == nil {
		t.Error("check.schema.json accepted a redirect to a replacement that does not exist")
	}
}

// TestASupersededConstraintProducesNothing pins the enforcement table's
// asymmetry rather than quietly widening it.
//
// docs/plan/lanes/E3.md gives decision/superseded the redirect and
// constraint/superseded a flat "nothing". That asymmetry is very likely an
// oversight — a superseded constraint is exactly as informative to redirect —
// but the table is the closed contract this epic is graded against, and a lane
// that widened it in passing would be deciding for E3-L1 that nobody could
// later find. It is reported in docs/decisions-pending/E3-L4-report.md; this
// test is here so that changing it has to be deliberate.
func TestASupersededConstraintProducesNothing(t *testing.T) {
	t.Parallel()

	entries := fixtureEntries(t, supersedeLedger)
	for _, e := range entries {
		if e.ID == "cst-0004" {
			e.State = ledger.StateSuperseded
		}
		if e.ID == "dec-0061" {
			e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeSupersedes, To: "cst-0004"})
		}
	}

	// The plan that cites cst-0004 while it is active, so the case is not
	// vacuous: it has to be a plan the constraint would have caught.
	const plan = "spin up a small hosted sync service so ledgers stay in sync across devices"
	if before := check(plan, fixtureEntries(t, supersedeLedger)); before.Compliant() {
		t.Fatalf("%q does not conflict with the active cst-0004, so superseding it proves nothing", plan)
	}

	after := check(plan, entries)
	if !after.Compliant() {
		t.Errorf("a superseded constraint was still cited: %+v", after.Conflicts)
	}
	if len(after.Notices) != 0 {
		t.Errorf("a superseded constraint produced a notice, which the enforcement table does not give it: %+v",
			after.Notices)
	}
}
