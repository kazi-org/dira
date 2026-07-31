package enforcer

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/kazi-org/dira/internal/ledger"
)

// goldenDaemon is the frozen demo asset.
const goldenDaemon = "testdata/golden/daemon.txt"

// demoPlan is the string .agents/product-marketing.md §6 records a clip of.
const demoPlan = "add a background daemon to track run state"

// TestTheDemoBlockIsByteForByte is the marketing freeze, enforced.
//
// .agents/product-marketing.md §6 pre-registers this exact terminal block as
// E8's primary launch asset — one clip, under twenty seconds, no narration —
// and README.md and docs/design.md §7 print it too. Three surfaces and a
// recorded video cannot be re-cut because someone preferred a different
// preposition, so a diff here is a break rather than a preference. If the block
// genuinely has to change, the golden, all three documents and the clip change
// together, deliberately, in one commit.
func TestTheDemoBlockIsByteForByte(t *testing.T) {
	t.Parallel()

	want, err := os.ReadFile(goldenDaemon)
	if err != nil {
		t.Fatalf("reading %s: %v", goldenDaemon, err)
	}

	var got bytes.Buffer
	v := check(demoPlan, fixtureEntries(t, daemonLedger))
	if err := Render(&got, v); err != nil {
		t.Fatalf("rendering: %v", err)
	}

	if got.String() != string(want) {
		t.Errorf("`dira check %q` no longer matches %s.\n--- got ---\n%s\n--- want ---\n%s",
			demoPlan, goldenDaemon, got.String(), want)
	}

	// The golden is only worth freezing if it is the whole message, so
	// assert what it must contain rather than trusting that it does.
	for _, line := range []string{
		"✗ conflicts with dec-0060 (accepted 2026-07-03)",
		`    rejected alternative: "a daemon"`,
		"    why_not: violates the single-binary intent (int-0002)",
		"    revisit_if: cold-start latency stops being the binding constraint",
		"→ supersede dec-0060, or revise the plan",
	} {
		if !strings.Contains(string(want), line+"\n") {
			t.Errorf("%s does not contain the line %q", goldenDaemon, line)
		}
	}
	if v.ExitCode() != ExitConflict {
		t.Errorf("the demo plan exits %d, want %d", v.ExitCode(), ExitConflict)
	}
}

// TestTheCompliantPlanEmitsNoCross is the other half of the demo: the check has
// to be quiet when there is nothing to say, or the one time it is loud means
// nothing.
func TestTheCompliantPlanEmitsNoCross(t *testing.T) {
	t.Parallel()

	const plan = "write the checkpoint file atomically"
	v := check(plan, fixtureEntries(t, daemonLedger))

	if !v.Compliant() {
		t.Fatalf("%q conflicts with %d entries; it is what dec-0060 chose", plan, len(v.Conflicts))
	}
	if v.ExitCode() != ExitCompliant {
		t.Errorf("exit code is %d, want %d", v.ExitCode(), ExitCompliant)
	}

	var out bytes.Buffer
	if err := Render(&out, v); err != nil {
		t.Fatalf("rendering: %v", err)
	}
	if strings.Contains(out.String(), "✗") {
		t.Errorf("a compliant check printed a ✗:\n%s", out.String())
	}
	if v.Enforced == 0 {
		t.Error("the verdict reports 0 enforced entries; \"no conflict\" against nothing is not an answer")
	}
	if !strings.Contains(out.String(), "6 enforced entries") {
		t.Errorf("the compliant message does not say what it checked against:\n%s", out.String())
	}
}

// TestRenderStatesAMissingRevisitCondition covers the rule that keeps the check
// from being merely obstructive.
//
// revisit_if is what distinguishes a closed door from a locked one. Where an
// alternative recorded none, omitting the line reads as "never", which is the
// opposite of the truth: superseding it in writing is always available. So the
// line is printed, saying exactly that.
func TestRenderStatesAMissingRevisitCondition(t *testing.T) {
	t.Parallel()

	out := renderOne(t, Conflict{Citation: Citation{
		Entry:  "dec-0042",
		Kind:   ledger.KindDecision,
		State:  ledger.StateRejected,
		Date:   "2026-07-02",
		Basis:  BasisAlternative,
		Title:  "Event-source every run mutation instead of snapshotting state",
		Option: "a compacted event log instead of full mutation replay",
		WhyNot: "compaction still needs a replay-and-fold step",
	}})

	const want = "    revisit_if: none recorded — supersede dec-0042 to reopen\n"
	if !strings.Contains(out, want) {
		t.Errorf("an alternative with no revisit_if did not say so:\n%s", out)
	}
}

// TestRenderOffersNoRevisitWhereNoneCanExist is the asymmetry docs/design.md §7
// is explicit about.
//
// revisit_if lives on an alternative in entry.schema.json, so a constraint and
// a rejected decision's own subject have no field to carry one. The message
// must not imply otherwise — not even by printing "none recorded", which would
// suggest someone forgot to fill it in. Their escape hatch is the supersede
// line every block already ends with.
func TestRenderOffersNoRevisitWhereNoneCanExist(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		c    Conflict
		want string
	}{
		{
			name: "constraint",
			c: Conflict{Citation: Citation{
				Entry: "cst-0004", Kind: ledger.KindConstraint, State: ledger.StateActive,
				Date: "2026-06-01", Basis: BasisConstraint,
				Title: "dira never requires a network service, an account, or a hosted tier to function",
			}},
			want: "✗ conflicts with cst-0004 (active)\n" +
				"    dira never requires a network service, an account, or a hosted tier to function\n" +
				"→ supersede cst-0004, or revise the plan\n",
		},
		{
			name: "rejected decision",
			c: Conflict{Citation: Citation{
				Entry: "dec-0042", Kind: ledger.KindDecision, State: ledger.StateRejected,
				Date: "2026-07-02", Basis: BasisDecision,
				Title: "Event-source every run mutation instead of snapshotting state",
			}},
			want: "✗ conflicts with dec-0042 (rejected 2026-07-02)\n" +
				`    rejected decision: "Event-source every run mutation instead of snapshotting state"` + "\n" +
				"→ supersede dec-0042, or revise the plan\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := renderOne(t, tc.c)
			if out != tc.want {
				t.Errorf("rendered:\n%s\nwant:\n%s", out, tc.want)
			}
			if strings.Contains(out, "revisit_if") {
				t.Errorf("a %s offered a revisit condition it cannot have:\n%s", tc.name, out)
			}
		})
	}
}

// TestPrivateEntriesAreCitedByRefOnly is a security test, not a formatting one.
//
// cst-0003 makes leaking a private entry's text a security bug, and E3's lane
// file tightens the rule to unconditional because the binary cannot classify
// its own stdout: a pipe into a pull-request body looks exactly like a
// terminal. The sentinel is checked against the combined human and JSON output,
// because a renderer that got one right and the other wrong would leak through
// whichever one a hook happened to use.
func TestPrivateEntriesAreCitedByRefOnly(t *testing.T) {
	t.Parallel()

	const sentinel = "SENTINEL-PRIVATE-TEXT"
	private := Conflict{Citation: Citation{
		Entry:     "cst-0002",
		Kind:      ledger.KindConstraint,
		State:     ledger.StateActive,
		Date:      "2026-06-01",
		Private:   true,
		Basis:     BasisConstraint,
		Title:     "never run more than one " + sentinel + " side project",
		Option:    sentinel + " option",
		WhyNot:    sentinel + " reason",
		RevisitIf: sentinel + " condition",
	}}

	v := &Verdict{Plan: "start a second side project", Conflicts: []Conflict{private}, Enforced: 3}

	var human, machine bytes.Buffer
	if err := Render(&human, v); err != nil {
		t.Fatalf("rendering: %v", err)
	}
	if err := RenderJSON(&machine, v); err != nil {
		t.Fatalf("rendering json: %v", err)
	}

	for name, out := range map[string]string{"human": human.String(), "json": machine.String()} {
		if strings.Contains(out, sentinel) {
			t.Errorf("%s output leaks private entry text (cst-0003):\n%s", name, out)
		}
		if !strings.Contains(out, "cst-0002") {
			t.Errorf("%s output does not cite the private entry at all:\n%s", name, out)
		}
	}
	if !strings.Contains(human.String(), "cited by reference only") {
		t.Errorf("the human output does not say why the text is missing:\n%s", human.String())
	}

	// Validating here as well as in the schema test is the point: the schema
	// encodes the privacy rule, so a leak has to fail validation and not
	// only this assertion.
	validate(t, machine.Bytes())
}

// TestJSONValidatesAgainstTheSchema runs every verdict shape this check can
// produce through schema/check.schema.json.
//
// The schema is the published contract a hook parses, and a contract nothing
// validates against is a comment. The cases are chosen to reach each of its
// conditional branches — compliant, an alternative citation, a rejected
// decision's own subject, a constraint, and a private entry — because an
// `if/then` nobody exercises passes vacuously.
func TestJSONValidatesAgainstTheSchema(t *testing.T) {
	t.Parallel()

	entries := fixtureEntries(t, daemonLedger)
	plans := []struct {
		name string
		plan string
	}{
		{"alternative", demoPlan},
		{"compliant", "write the checkpoint file atomically"},
		{"rejected decision", "log every run mutation as an event so we can replay full history"},
		{"constraint", "spin up a small hosted sync service so ledgers stay in sync across devices"},
	}

	seen := map[string]bool{}
	for _, tc := range plans {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			v := check(tc.plan, entries)
			if err := RenderJSON(&out, v); err != nil {
				t.Fatalf("rendering json: %v", err)
			}
			validate(t, out.Bytes())

			var doc jsonVerdict
			if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
				t.Fatalf("re-reading the document: %v", err)
			}
			if doc.Verdict == "compliant" {
				seen["compliant"] = true
			}
			for _, c := range doc.Conflicts {
				seen[c.Basis] = true
			}
		})
	}

	for _, want := range []string{"compliant", "alternative", "decision", "constraint"} {
		if !seen[want] {
			t.Errorf("no case produced a %q document, so the schema branch covering it was never exercised", want)
		}
	}
}

// TestTheSchemaRejectsALeak is the negative half. A schema that accepts
// everything validates nothing, and the privacy rule is the one constraint in
// it whose failure is a security bug rather than a shape error.
func TestTheSchemaRejectsALeak(t *testing.T) {
	t.Parallel()

	leak := `{
	  "plan": "start a second side project",
	  "verdict": "conflict",
	  "exit_code": 2,
	  "enforced_entries": 3,
	  "conflicts": [{
	    "entry": "cst-0002",
	    "kind": "constraint",
	    "state": "active",
	    "private": true,
	    "basis": "constraint",
	    "title": "never run more than one side project",
	    "revisit_if": null,
	    "supersede": "cst-0002",
	    "score": 0.9
	  }]
	}`

	if err := validateErr(t, []byte(leak)); err == nil {
		t.Error("check.schema.json accepted a private entry cited by text; the privacy rule is not being checked")
	}
}

func renderOne(t *testing.T, c Conflict) string {
	t.Helper()

	var out bytes.Buffer
	if err := Render(&out, &Verdict{Plan: "a plan", Conflicts: []Conflict{c}, Enforced: 1}); err != nil {
		t.Fatalf("rendering: %v", err)
	}
	return out.String()
}

func validate(t *testing.T, doc []byte) {
	t.Helper()

	if err := validateErr(t, doc); err != nil {
		t.Errorf("the document does not satisfy check.schema.json: %v\n%s", err, doc)
	}
}

func validateErr(t *testing.T, doc []byte) error {
	t.Helper()

	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		t.Fatalf("parsing the document: %v", err)
	}
	return checkSchema(t).Validate(value)
}

// checkSchema compiles schema/check.schema.json off disk.
//
// Off disk rather than embedded because this package must not carry the schema
// into the binary: nothing in the command path compiles a JSON Schema document,
// since doing so on every invocation would spend int-0002's whole budget. The
// schema is a gate for tests and for whatever consumes the JSON, which is not
// dira.
func checkSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()

	const path = "../../schema/check.schema.json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", filepath.Clean(path), err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	const url = "https://github.com/kazi-org/dira/schema/check.schema.json"
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(url, doc); err != nil {
		t.Fatalf("registering %s: %v", url, err)
	}
	sch, err := c.Compile(url)
	if err != nil {
		t.Fatalf("compiling %s: %v", url, err)
	}
	return sch
}
