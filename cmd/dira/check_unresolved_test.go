package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/config"
	"github.com/kazi-org/dira/internal/enforcer"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// E3-L3-T7: a parent that could not be read is reported, and the report is not
// a verdict.
//
// Every ledger here is a real temp `.dira/` tree built by childWithParent in
// check_parents_test.go (docs/lore.md L-0014), and every absence assertion below
// is preceded by an assertion that the legitimate content is present — an empty
// stderr contains no leak either, and a test that only looked for the leak would
// pass against a binary that printed nothing at all.
//
// The two facts this file separates are the two dec-0011 separates. A declared
// parent that is not here is *unresolved*. A parent declared
// `visibility = "private"` that is not here is *withheld*, which dec-0018 makes
// a designed state rather than a fault: it is what a public clone of a ledger
// with a private parent looks like when everything is working.

// The plan that conflicts only with the *child's* own ledger. It restates
// childConfigDecision's rejected alternative, so its exit 2 is owned entirely by
// the local ledger and cannot move when a parent fails to resolve.
const localConflictPlan = "link a full TOML parser into the binary"

// parentLabelSentinel is a `label` on the [parents] declaration.
//
// dec-0011 says a private parent omits its label precisely so the name never
// ships, and scripts/privacy-lint.py P2 asserts that on committed bytes. This is
// the run-time counterpart: the string is committed into a config the command
// really reads, and then searched for across every byte the process emitted.
const parentLabelSentinel = "SENTINEL-PARENT-LABEL"

// The declarations under test. The withheld and unresolved pair differ in
// exactly one field, which is what makes the difference in their reports
// attributable to `visibility` and to nothing else.
const (
	declPublicParent  = `me = { path = "../../parent", label = "` + parentLabelSentinel + `" }`
	declPrivateParent = `me = { path = "../../parent", visibility = "private", label = "` +
		parentLabelSentinel + `" }`
)

// TestUnresolvedParentDoesNotChangeTheExitCode is the first acceptance clause,
// asserted in both directions.
//
// `dira check`'s 2 means a cited conflict and nothing else. A parent that cannot
// be read may not manufacture one, and it may not suppress one either — the
// second is the failure that matters, because a firewall that fails open in the
// configuration a public clone has is a firewall nobody finds out about.
func TestUnresolvedParentDoesNotChangeTheExitCode(t *testing.T) {
	t.Parallel()

	t.Run("a plan that conflicts locally still exits 2", func(t *testing.T) {
		t.Parallel()

		child := childWithMissingParent(t, declPublicParent)

		r := newRunner(t)
		code := r.run("-C", child, localConflictPlan)

		if code != enforcer.ExitConflict {
			t.Fatalf("exit code is %d, want %d (the local conflict is still a conflict)\nstdout:\n%s\nstderr:\n%s",
				code, enforcer.ExitConflict, r.stdout.String(), r.stderr.String())
		}
		if !strings.Contains(r.stdout.String(), "dec-0001") {
			t.Errorf("the verdict cites no local entry, so the exit code above is not the local verdict:\n%s",
				r.stdout.String())
		}
		assertOneParentLine(t, r.stderr.String(), "unresolved")
	})

	t.Run("a plan that is locally clean still exits 0", func(t *testing.T) {
		t.Parallel()

		child := childWithMissingParent(t, declPublicParent)

		// parentConflictPlan contradicts the parent's constraint and shares
		// no vocabulary with the child, so this is the run in which a
		// missing parent could most easily have been turned into a
		// verdict — either by erroring out or by citing something it never
		// read.
		r := newRunner(t)
		code := r.run("-C", child, parentConflictPlan)

		if code != enforcer.ExitCompliant {
			t.Fatalf("exit code is %d, want %d (an unreadable parent is not a conflict and not a failure)"+
				"\nstdout:\n%s\nstderr:\n%s", code, enforcer.ExitCompliant, r.stdout.String(), r.stderr.String())
		}
		if strings.Contains(r.stdout.String(), "me:") {
			t.Errorf("the verdict cites an inherited entry from a parent that was never read:\n%s", r.stdout.String())
		}
		assertOneParentLine(t, r.stderr.String(), "unresolved")
	})

	t.Run("the same child with the parent present reports nothing and cites it", func(t *testing.T) {
		t.Parallel()

		// The other side of both assertions above. Without it, "stderr
		// names the unresolved parent" would hold of a binary that printed
		// the line unconditionally, and "the exit code is the local one"
		// would hold of a binary that never resolved a parent at all.
		child, _ := childWithParent(t, declPublicParent)

		r := newRunner(t)
		code := r.run("-C", child, parentConflictPlan)

		if code != enforcer.ExitConflict {
			t.Fatalf("with the parent present the same plan exited %d, want %d; the case above is measuring "+
				"a check that never inherited anything\nstdout:\n%s", code, enforcer.ExitConflict, r.stdout.String())
		}
		if !strings.Contains(r.stdout.String(), "me:cst-0001") {
			t.Fatalf("the parent's constraint was not cited, so its absence above is not what is being reported:\n%s",
				r.stdout.String())
		}
		if got := parentLines(r.stderr.String()); len(got) != 0 {
			t.Errorf("a parent that resolved was reported anyway:\n%s", strings.Join(got, "\n"))
		}
	})
}

// TestUnresolvedParentThatExistsButCannotBeRead is the acceptance clause about a
// parent that is *there* and unreadable: it prints the same shape with the same
// count, and no line claims a total the run could not have read.
//
// The fixture makes `.dira/entries` a regular file. local.Open still succeeds —
// the ledger directory is there — and the read fails inside List, which is a
// different code path from a path that does not exist and produces a different
// error. That error names the file it failed on, which is what makes the second
// half of this test worth asserting.
func TestUnresolvedParentThatExistsButCannotBeRead(t *testing.T) {
	t.Parallel()

	child, parent := childWithParent(t, declPublicParent)
	entries := filepath.Join(parent, ".dira", "entries")
	if err := os.RemoveAll(entries); err != nil {
		t.Fatalf("removing the parent's entries directory: %v", err)
	}
	if err := os.WriteFile(entries, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("making the parent's entries directory unreadable: %v", err)
	}

	// The backend's error really does carry the path. Without this the
	// zero-occurrence assertion below would be measuring a string that was
	// never in play — the L-0014 shape, arriving through an error message.
	store, _ := openParent(filepath.Join(child, ".dira"), config.Parent{Name: "me", Path: "../../parent"})
	if _, err := store.List(context.Background()); err == nil {
		t.Fatal("the parent read cleanly, so this fixture is not an unreadable parent at all")
	} else if !strings.Contains(err.Error(), parent) {
		t.Fatalf("the backend error does not name the parent's path (%v), so the search below proves nothing", err)
	}

	r := newRunner(t)
	code := r.run("-C", child, parentConflictPlan)

	if code != enforcer.ExitCompliant {
		t.Fatalf("exit code is %d, want %d\nstdout:\n%s\nstderr:\n%s",
			code, enforcer.ExitCompliant, r.stdout.String(), r.stderr.String())
	}
	assertOneParentLine(t, r.stderr.String(), "unresolved")

	combined := r.stdout.String() + r.stderr.String()
	if strings.Contains(combined, parent) {
		t.Errorf("the path to the parent reached the output:\n%s\n"+
			"A filesystem path identifies a parent ledger at least as well as the label dec-0011 keeps out "+
			"of every output, and ParentResult.Err is the backend's error, not a printable one.", combined)
	}
}

// TestUnresolvedParentIsWithheldWhenDeclaredPrivate is the withheld clause:
// dec-0011's second state, dec-0018's rule that it reads as neither an error nor
// a warning, and cst-0003's rule that the declared label never ships.
func TestUnresolvedParentIsWithheldWhenDeclaredPrivate(t *testing.T) {
	t.Parallel()

	t.Run("a private parent is withheld and a public one is unresolved", func(t *testing.T) {
		t.Parallel()

		// One field apart. Everything else — the path, the label, the
		// missing parent, the plan — is identical, so the difference in
		// the two lines is attributable to `visibility` alone.
		private := newRunner(t)
		private.run("-C", childWithMissingParent(t, declPrivateParent), parentConflictPlan)
		public := newRunner(t)
		public.run("-C", childWithMissingParent(t, declPublicParent), parentConflictPlan)

		withheld := assertOneParentLine(t, private.stderr.String(), "withheld")
		unresolved := assertOneParentLine(t, public.stderr.String(), "unresolved")

		if strings.Contains(withheld, "unresolved") {
			t.Errorf("the withheld line also calls the parent unresolved, so the two states are not "+
				"distinguished:\n%s", withheld)
		}
		if strings.Contains(unresolved, "withheld") {
			t.Errorf("a parent declared public reported as withheld:\n%s", unresolved)
		}
		if withheld == unresolved {
			t.Errorf("withheld and unresolved render identically:\n%s", withheld)
		}
	})

	t.Run("the withheld line is neither an error nor a warning", func(t *testing.T) {
		t.Parallel()

		r := newRunner(t)
		r.run("-C", childWithMissingParent(t, declPrivateParent), parentConflictPlan)
		line := assertOneParentLine(t, r.stderr.String(), "withheld")

		// dec-0018: a private parent that is not on this machine is the
		// configuration working, not a fault in it.
		for _, word := range []string{"error", "warning"} {
			if strings.Contains(strings.ToLower(line), word) {
				t.Errorf("the withheld line uses the word %q; dec-0018 makes withheld a declared state that "+
					"must render as neither an error nor a warning:\n%s", word, line)
			}
		}
	})

	t.Run("the declared label reaches no output stream", func(t *testing.T) {
		t.Parallel()

		child := childWithMissingParent(t, declPrivateParent)

		// The label is really in the file the command reads. Without this
		// the zero-occurrence searches below would pass against a run
		// that was never given a label to leak.
		cfg, err := os.ReadFile(filepath.Join(child, ".dira", "config.toml"))
		if err != nil {
			t.Fatalf("reading the child's config: %v", err)
		}
		if !strings.Contains(string(cfg), parentLabelSentinel) {
			t.Fatalf("the fixture config carries no label, so nothing below is being tested:\n%s", cfg)
		}

		human := newRunner(t)
		human.run("-C", child, parentConflictPlan)
		machine := newRunner(t)
		machine.run("-C", child, "-json", parentConflictPlan)

		// Present-first, on both runs, so an empty stream cannot pass.
		assertOneParentLine(t, human.stderr.String(), "withheld")
		assertOneParentLine(t, machine.stderr.String(), "withheld")
		if !strings.Contains(machine.stdout.String(), `"withheld"`) {
			t.Fatalf("the document carries no withheld parent, so the search over it proves nothing:\n%s",
				machine.stdout.String())
		}

		for _, stream := range []struct {
			name string
			text string
		}{
			{"human stdout", human.stdout.String()},
			{"human stderr", human.stderr.String()},
			{"--json stdout", machine.stdout.String()},
			{"--json stderr", machine.stderr.String()},
		} {
			if n := strings.Count(stream.text, parentLabelSentinel); n != 0 {
				t.Errorf("the private parent's label occurs %d times in %s (want 0):\n%s",
					n, stream.name, stream.text)
			}
		}
	})
}

// TestUnresolvedParentReportGoesToStderr is the stream clause: the report is a
// note about the check, and stdout stays the parseable verdict.
func TestUnresolvedParentReportGoesToStderr(t *testing.T) {
	t.Parallel()

	child := childWithMissingParent(t, declPublicParent)

	r := newRunner(t)
	if code := r.run("-C", child, "-json", parentConflictPlan); code != enforcer.ExitCompliant {
		t.Fatalf("exit code is %d, want %d\nstderr:\n%s", code, enforcer.ExitCompliant, r.stderr.String())
	}

	assertOneParentLine(t, r.stderr.String(), "unresolved")
	if got := parentLines(r.stdout.String()); len(got) != 0 {
		t.Errorf("the human report landed on stdout, in the middle of the document a hook parses:\n%s",
			strings.Join(got, "\n"))
	}

	var doc map[string]any
	if err := json.Unmarshal(r.stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not a JSON document: %v\n%s", err, r.stdout.String())
	}
}

// TestUnresolvedParentInJSON is the --json clause: the same report in a
// `parents` block the schema describes, and a document carrying an undescribed
// key still fails validation.
func TestUnresolvedParentInJSON(t *testing.T) {
	t.Parallel()

	t.Run("an unresolved parent is reported with the count it can know", func(t *testing.T) {
		t.Parallel()

		doc, raw := checkJSON(t, childWithMissingParent(t, declPublicParent), parentConflictPlan)
		validateCheckDoc(t, raw)

		if len(doc.Parents) != 1 {
			t.Fatalf("the document reports %d parents, want 1:\n%s", len(doc.Parents), raw)
		}
		got := doc.Parents[0]
		if got.Namespace != "me" || got.Status != "unresolved" || got.Evaluated != 0 {
			t.Errorf("the parents block is %+v, want {me unresolved 0}\n%s", got, raw)
		}
	})

	t.Run("a private parent is withheld in the document too", func(t *testing.T) {
		t.Parallel()

		doc, raw := checkJSON(t, childWithMissingParent(t, declPrivateParent), parentConflictPlan)
		validateCheckDoc(t, raw)

		if len(doc.Parents) != 1 || doc.Parents[0].Status != "withheld" {
			t.Fatalf("the document does not report a withheld parent: %+v\n%s", doc.Parents, raw)
		}
	})

	t.Run("a resolved parent carries its real count", func(t *testing.T) {
		t.Parallel()

		// The red side of the count: if `evaluated` were hard-wired to
		// zero, or the block were only ever emitted for a failure, this
		// case fails.
		child, _ := childWithParent(t, declPublicParent)
		doc, raw := checkJSON(t, child, parentConflictPlan)
		validateCheckDoc(t, raw)

		if len(doc.Parents) != 1 {
			t.Fatalf("the document reports %d parents, want 1:\n%s", len(doc.Parents), raw)
		}
		got := doc.Parents[0]
		if got.Status != "resolved" || got.Evaluated != len(parentEnforceable) {
			t.Errorf("the parents block is %+v, want status resolved and evaluated %d (the fixture's own "+
				"inheritable count)\n%s", got, len(parentEnforceable), raw)
		}
	})

	t.Run("a ledger with no parents carries an empty block", func(t *testing.T) {
		t.Parallel()

		// "No parents declared" and "a parent that contributed nothing"
		// are different facts, and a block that were omitted in the first
		// case would make them indistinguishable.
		child, _ := childWithParent(t, `# me = { path = "../../parent" }`)
		doc, raw := checkJSON(t, child, parentConflictPlan)
		validateCheckDoc(t, raw)

		if doc.Parents == nil {
			t.Errorf("the document omits `parents` entirely for a ledger that declares none:\n%s", raw)
		}
		if len(doc.Parents) != 0 {
			t.Errorf("a commented-out declaration produced %d parent reports:\n%s", len(doc.Parents), raw)
		}
	})

	t.Run("the schema rejects what the document may not carry", func(t *testing.T) {
		t.Parallel()

		_, raw := checkJSON(t, childWithMissingParent(t, declPrivateParent), parentConflictPlan)

		// The control. Every rejection below is a mutation of this exact
		// document, so a rejection means the mutation was rejected rather
		// than the document being invalid to begin with.
		validateCheckDoc(t, raw)
		validateCheckDoc(t, mutateCheckDoc(t, raw, func(doc map[string]any) {
			doc["plan"] = "a different plan entirely"
		}))

		for _, tc := range []struct {
			name   string
			mutate func(doc map[string]any)
		}{
			{
				// The runtime counterpart of privacy-lint P2, as a
				// shape rule: there is no key for a label, so a
				// renderer that grew one fails validation.
				name: "a label on a parent",
				mutate: func(doc map[string]any) {
					firstParent(t, doc)["label"] = parentLabelSentinel
				},
			},
			{
				name: "a path on a parent",
				mutate: func(doc map[string]any) {
					firstParent(t, doc)["path"] = "../../parent"
				},
			},
			{
				name: "an undescribed key on the document itself",
				mutate: func(doc map[string]any) {
					doc["parent_path"] = "../../parent"
				},
			},
			{
				// The honesty rule. A run that could not open a
				// ledger cannot have counted what is in it.
				name: "a withheld parent with a count",
				mutate: func(doc map[string]any) {
					firstParent(t, doc)["evaluated"] = 3
				},
			},
			{
				name: "a status the document does not define",
				mutate: func(doc map[string]any) {
					firstParent(t, doc)["status"] = "orphan"
				},
			},
			{
				name: "a parent with no status at all",
				mutate: func(doc map[string]any) {
					delete(firstParent(t, doc), "status")
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				mutated := mutateCheckDoc(t, raw, tc.mutate)
				if err := validateCheckDocErr(t, mutated); err == nil {
					t.Errorf("check.schema.json accepted a document carrying %s:\n%s", tc.name, mutated)
				}
			})
		}
	})
}

// --- fixtures ------------------------------------------------------------- //

// childWithMissingParent builds the child and parent of childWithParent and then
// removes the parent, which is the configuration a public clone of a ledger with
// a private parent has.
func childWithMissingParent(t *testing.T, declaration string) string {
	t.Helper()

	child, parent := childWithParent(t, declaration)
	if err := os.RemoveAll(parent); err != nil {
		t.Fatalf("removing the parent: %v", err)
	}
	return child
}

// parentLines returns the lines of text that report on a parent ledger.
func parentLines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "parent ledger me") {
			out = append(out, line)
		}
	}
	return out
}

// assertOneParentLine requires exactly one report line about the parent `me`,
// carrying the given state and the only count the run can know, and returns it.
//
// One line, not "at least one": the acceptance is a single line per parent, and
// a report that printed one per read attempt would be a check narrating itself.
func assertOneParentLine(t *testing.T, stderr, state string) string {
	t.Helper()

	lines := parentLines(stderr)
	if len(lines) != 1 {
		t.Fatalf("stderr carries %d lines naming the parent, want exactly 1:\n%s", len(lines), stderr)
	}
	line := lines[0]

	if !strings.Contains(line, state) {
		t.Errorf("the report does not call the parent %s:\n%s", state, line)
	}
	if !strings.Contains(line, "0 of its constraints were evaluated") {
		t.Errorf("the report does not state how many of the parent's constraints were evaluated:\n%s", line)
	}

	// The count it can know, and no other. The lane acceptance asks for "the
	// number of constraints not evaluated", and for a ledger nobody could open
	// that number does not exist — a run that reports one has invented it. So
	// the line is required to carry exactly one number, and that number is the
	// zero it did evaluate.
	if numbers := reportNumbers.FindAllString(line, -1); len(numbers) != 1 || numbers[0] != "0" {
		t.Errorf("the report states the numbers %v; the only count a run that could not open a ledger has is "+
			"the zero it evaluated, and a total it could not have read must not appear:\n%s", numbers, line)
	}
	return line
}

// reportNumbers finds every number in a report line. The namespaces in this
// file carry no digits, so anything it matches came from the report itself.
var reportNumbers = regexp.MustCompile(`[0-9]+`)

// checkJSON runs `dira check --json` and returns the document, decoded and raw.
func checkJSON(t *testing.T, child, plan string) (parsedCheckDoc, []byte) {
	t.Helper()

	r := newRunner(t)
	r.run("-C", child, "-json", plan)

	var doc parsedCheckDoc
	if err := json.Unmarshal(r.stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s\nstderr:\n%s", err, r.stdout.String(), r.stderr.String())
	}
	return doc, r.stdout.Bytes()
}

// parsedCheckDoc is the part of the document this file asserts over. Parents is
// a pointer-free slice so that a missing block and an empty one are told apart
// by nil rather than by length.
type parsedCheckDoc struct {
	Parents []struct {
		Namespace string `json:"namespace"`
		Status    string `json:"status"`
		Evaluated int    `json:"evaluated"`
	} `json:"parents"`
}

// mutateCheckDoc re-encodes a real document with one change applied.
func mutateCheckDoc(t *testing.T, raw []byte, mutate func(doc map[string]any)) []byte {
	t.Helper()

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decoding the document: %v\n%s", err, raw)
	}
	mutate(doc)

	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("re-encoding the document: %v", err)
	}
	return out
}

// firstParent returns the first entry of a decoded document's parents block.
func firstParent(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()

	parents, ok := doc["parents"].([]any)
	if !ok || len(parents) == 0 {
		t.Fatalf("the document has no parents block to mutate: %v", doc["parents"])
	}
	parent, ok := parents[0].(map[string]any)
	if !ok {
		t.Fatalf("the first parent is not an object: %v", parents[0])
	}
	return parent
}

func validateCheckDoc(t *testing.T, raw []byte) {
	t.Helper()

	if err := validateCheckDocErr(t, raw); err != nil {
		t.Errorf("the document does not satisfy check.schema.json: %v\n%s", err, raw)
	}
}

func validateCheckDocErr(t *testing.T, raw []byte) error {
	t.Helper()

	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parsing the document: %v\n%s", err, raw)
	}
	return compiledCheckSchema(t).Validate(value)
}

// compiledCheckSchema compiles schema/check.schema.json off disk.
//
// Off disk, and never embedded into this package: nothing in the command path
// compiles a JSON Schema document, because doing so on every invocation would
// spend int-0002's whole cold-start budget. A test-only import is invisible to
// `go list -deps`, which is what keeps TestCommandPathLinksOnlyAllowedModules
// green and honest at the same time.
//
// AssertFormat is called for docs/lore.md L-0015: format is annotation-only in
// draft 2020-12, so a compiler without it validates a document that violates a
// declared format.
func compiledCheckSchema(t *testing.T) *jsonschema.Schema {
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
