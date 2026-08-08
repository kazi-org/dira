package enforcer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/kazi-org/dira/internal/config"
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

// The inherited fixture's two sentinels.
//
// They sit at the *front* of the title and of the body on purpose. docs/lore.md
// L-0024: a "the output contains no secret" assertion whose secret sits past the
// drawn width passes against a build with no redaction in it at all, because the
// secret was cut before the redactor was ever consulted. Nothing in Render
// truncates today; putting the sentinels first means nothing that starts to will
// quietly turn these assertions green.
//
// They are also screened verbatim and never through any normalising step
// (L-0023): normalisation is what destroys the shape a screen keys on, and a
// screen over a normalised string is a fail-open in the one path whose whole job
// is refusing text.
const (
	sentinelInheritedTitle = "SENTINEL-INHERITED-TITLE"
	sentinelInheritedBody  = "SENTINEL-INHERITED-BODY"
)

// inheritedPrivateGolden is the block a private inherited citation renders to.
const inheritedPrivateGolden = "testdata/golden/inherited-private.txt"

// sentinelConstraint is the parent's active constraint, carrying a distinctive
// string in each of the two places a renderer could leak one.
const sentinelConstraint = `---
id: cst-0001
kind: constraint
title: ` + sentinelInheritedTitle + ` no engineering hire is made before the workspace holds twelve months of runway
state: active
created: "2026-06-01T09:00:00Z"
tags: [fixture, hiring]
source:
  hook: manual
  tier: human
confirmed_by: human
---

` + sentinelInheritedBody + ` an engineering hire is the largest irreversible
commitment this workspace makes. No engineering hire is made while runway is
under twelve months, whatever the pipeline looks like.
`

// TestRenderPrivateInheritedCitation is E3-L3-T5.
//
// cst-0003 rule 3 is a rule and not a mode: a private entry is cited ref-only in
// every mode, so this asserts over the human block and the --json document
// together. What makes it different from TestPrivateEntriesAreCitedByRefOnly
// above is that the citation is not hand-built — it comes out of Inherit over a
// real parent ledger on disk, so what is under test is the path a `dira check`
// with a person-tier parent actually takes.
//
// Every absence assertion below is preceded by a presence assertion, and every
// one of them has a green side: the same citation rendered non-private has to
// print the very strings the private one must not. Without that pair, a renderer
// that printed nothing at all would pass the whole test (L-0001, and L-0024 for
// the security-specific form of it).
func TestRenderPrivateInheritedCitation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("the human block cites the ref and prints no text from the parent", func(t *testing.T) {
		t.Parallel()

		v := inheritedPrivateVerdict(t, ctx)
		var out bytes.Buffer
		if err := Render(&out, v); err != nil {
			t.Fatalf("rendering: %v", err)
		}
		got := out.String()

		// Present first. An empty block satisfies every absence
		// assertion below it, so the ones that say the citation is
		// actually there have to come first and have to fail loudly.
		for _, want := range []string{
			"me:cst-0001",              // the namespaced ref
			string(ledger.StateActive), // the state
			"cited by reference only",  // why there is no text
			"parent ledger me",         // the remedy names the owning ledger
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("the block does not contain %q, so the absence assertions below prove nothing:\n%s", want, got)
			}
		}

		assertNoParentText(t, "human", got)

		// The remedy must not be advice this binary rejects.
		// cmd/dira/supersede.go refuses a namespaced ref on either side
		// at exit 2 before it opens anything, so a line reading
		// "supersede me:cst-0001, or revise the plan" would send the
		// reader to a command that answers 2.
		if strings.Contains(got, "supersede me:cst-0001,") {
			t.Errorf("the block tells the reader to supersede an inherited entry from here, "+
				"which `dira supersede` refuses at exit 2 (cst-0003 rule 1):\n%s", got)
		}
	})

	t.Run("the same citation rendered non-private prints all four", func(t *testing.T) {
		t.Parallel()

		// The green side. Each field the private block must omit is
		// shown reaching the renderer and being printed, so "absent"
		// above means "withheld" rather than "never supplied".
		for _, tc := range []struct {
			name string
			c    Conflict
			want string
		}{
			{"title", publicConstraintCitation(), sentinelInheritedTitle},
			{"rejected alternative", publicAlternativeCitation(), "an " + sentinelInheritedBody + " option"},
			{"why_not", publicAlternativeCitation(), "the " + sentinelInheritedBody + " reason"},
			{"revisit_if", publicAlternativeCitation(), "the " + sentinelInheritedBody + " condition"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if got := renderOne(t, tc.c); !strings.Contains(got, tc.want) {
					t.Errorf("a non-private inherited citation does not print its %s %q, so the "+
						"assertion that a private one omits it is unfalsifiable:\n%s", tc.name, tc.want, got)
				}

				private := tc.c
				private.Private = true
				if got := renderOne(t, private); strings.Contains(got, tc.want) {
					t.Errorf("a private inherited citation printed its %s (cst-0003):\n%s", tc.name, got)
				}
			})
		}
	})

	t.Run("the --json document omits the text and says so", func(t *testing.T) {
		t.Parallel()

		v := inheritedPrivateVerdict(t, ctx)
		var out bytes.Buffer
		if err := RenderJSON(&out, v); err != nil {
			t.Fatalf("rendering json: %v", err)
		}

		// Validated against the published contract with AssertFormat on
		// the compiler (L-0015), because the schema encodes the privacy
		// rule and a leak has to fail validation rather than only this
		// test.
		validate(t, out.Bytes())
		assertNoParentText(t, "json", out.String())

		var doc struct {
			Conflicts []map[string]any `json:"conflicts"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatalf("re-reading the document: %v", err)
		}
		if len(doc.Conflicts) != 1 {
			t.Fatalf("the document carries %d conflicts, want the one inherited citation", len(doc.Conflicts))
		}
		conflict := doc.Conflicts[0]

		if got := conflict["entry"]; got != "me:cst-0001" {
			t.Fatalf("the document cites %v, not the inherited entry; nothing below is about a parent", got)
		}
		// Key absence, not substring absence: a key present with an
		// empty value is still a statement that dira knows the text and
		// chose to publish it as nothing.
		for _, key := range []string{"title", "rejected_alternative", "why_not"} {
			if _, present := conflict[key]; present {
				t.Errorf("the document carries %q for a private entry (cst-0003): %v", key, conflict)
			}
		}
		if private, _ := conflict["private"].(bool); !private {
			t.Errorf(`the document does not set "private": true, so a consumer cannot tell why the text is missing: %v`, conflict)
		}
		revisit, present := conflict["revisit_if"]
		if !present || revisit != nil {
			t.Errorf(`the document's "revisit_if" is %v (present=%t), want an explicit null`, revisit, present)
		}
	})

	t.Run("the --json document carries the text when the entry is not private", func(t *testing.T) {
		t.Parallel()

		// The green side of the three omissions above, on the machine
		// surface. Without it a RenderJSON that dropped those keys
		// unconditionally would pass.
		var out bytes.Buffer
		v := &Verdict{Plan: "a plan", Conflicts: []Conflict{publicAlternativeCitation()}, Enforced: 1}
		if err := RenderJSON(&out, v); err != nil {
			t.Fatalf("rendering json: %v", err)
		}
		validate(t, out.Bytes())

		var doc struct {
			Conflicts []map[string]any `json:"conflicts"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatalf("re-reading the document: %v", err)
		}
		for _, key := range []string{"title", "rejected_alternative", "why_not", "revisit_if"} {
			if value, present := doc.Conflicts[0][key]; !present || value == nil {
				t.Errorf("a non-private inherited citation omits %q, so the private case's omission "+
					"proves nothing: %v", key, doc.Conflicts[0])
			}
		}
	})

	t.Run("the block matches the golden byte for byte", func(t *testing.T) {
		t.Parallel()

		want, err := os.ReadFile(inheritedPrivateGolden)
		if err != nil {
			t.Fatalf("reading %s: %v", filepath.Clean(inheritedPrivateGolden), err)
		}

		var got bytes.Buffer
		if err := Render(&got, inheritedPrivateVerdict(t, ctx)); err != nil {
			t.Fatalf("rendering: %v", err)
		}
		if got.String() != string(want) {
			t.Errorf("the inherited private block no longer matches %s.\n--- got ---\n%s\n--- want ---\n%s",
				inheritedPrivateGolden, got.String(), want)
		}
		// A golden is worth freezing only if it is the whole message.
		for _, line := range []string{
			"✗ conflicts with me:cst-0001 (active, private — cited by reference only)",
			"→ me:cst-0001 is enforced by the parent ledger me; supersede it there, or revise the plan",
		} {
			if !strings.Contains(string(want), line+"\n") {
				t.Errorf("%s does not contain the line %q", inheritedPrivateGolden, line)
			}
		}
	})
}

// inheritedPrivateVerdict is one conflict, cited across the boundary out of a
// person-tier parent that is really on disk.
//
// The parent is written into a real temp .dira/ tree and opened through
// local.Open, never pointed at testdata/ledgers/<name>/ (L-0014): those are flat
// piles of *.md that local.Find would walk *past*, silently grading against this
// repository's own ledger.
func inheritedPrivateVerdict(t *testing.T, ctx context.Context) *Verdict {
	t.Helper()

	diraDir := writeLedger(t, parentConfig("person"), map[string]string{"cst-0001": sentinelConstraint})
	inh := inheritFrom(t, ctx, diraDir, config.Parent{Name: "me"})

	v, err := CheckInherited(ctx, stubLedger{entries: fixtureEntries(t, daemonLedger)}, conflictingPlan, inh)
	if err != nil {
		t.Fatalf("CheckInherited: %v", err)
	}
	if len(v.Conflicts) != 1 || v.Conflicts[0].Entry != "me:cst-0001" {
		t.Fatalf("the plan cited %v, want exactly the inherited me:cst-0001", citedConflicts(v))
	}
	if !v.Conflicts[0].Private {
		t.Fatal("the citation is not private, so this whole test is about the ordinary render path; " +
			"the parent is person-tier and cst-0003 makes that unconditional")
	}
	return v
}

// assertNoParentText is the zero-occurrence search, run over whole output rather
// than over a field.
func assertNoParentText(t *testing.T, surface, out string) {
	t.Helper()

	for _, sentinel := range []string{sentinelInheritedTitle, sentinelInheritedBody} {
		if strings.Contains(out, sentinel) {
			t.Errorf("the %s output carries %s from a private parent (cst-0003):\n%s", surface, sentinel, out)
		}
	}
	// The label as well as the value: a `revisit_if:` line with nothing after
	// it still tells a reader a door exists, and the schema makes the field
	// null for a private entry precisely so it says nothing at all.
	for _, label := range []string{"rejected alternative:", "why_not:", "revisit_if:"} {
		if strings.Contains(out, label) {
			t.Errorf("the %s output carries a %q line for a private entry:\n%s", surface, label, out)
		}
	}
}

// publicConstraintCitation is the inherited constraint as a citation that is not
// private — the falsifier for every absence assertion about a title.
func publicConstraintCitation() Conflict {
	return Conflict{Citation: Citation{
		Entry: "me:cst-0001",
		Kind:  ledger.KindConstraint,
		State: ledger.StateActive,
		Basis: BasisConstraint,
		Title: sentinelInheritedTitle + " no engineering hire is made before twelve months of runway",
	}, Score: 0.9}
}

// publicAlternativeCitation is an inherited citation on an alternative.
//
// Nothing in inherit.go produces one today: the enforcement table crosses the
// boundary for constraint/active and nothing else, so an inherited citation is
// always BasisConstraint. It is built here anyway, and that is deliberate rather
// than speculative — the ref-only rule is a property of the *renderer*, not of
// what happens to reach it, and this is the only way to show the three fields a
// constraint has no room for being withheld. If a later lane widens what
// inherits, this case already fails on the day the rule stops holding.
func publicAlternativeCitation() Conflict {
	return Conflict{Citation: Citation{
		Entry:     "me:dec-0001",
		Kind:      ledger.KindDecision,
		State:     ledger.StateAccepted,
		Date:      "2026-06-01",
		Basis:     BasisAlternative,
		Title:     sentinelInheritedTitle + " the workspace runs its own payroll",
		Option:    "an " + sentinelInheritedBody + " option",
		WhyNot:    "the " + sentinelInheritedBody + " reason",
		RevisitIf: "the " + sentinelInheritedBody + " condition",
	}, Score: 0.9}
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
