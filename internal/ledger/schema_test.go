package ledger_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/schema"
)

// Entry.Validate is a second expression of entry.schema.json, and a second
// expression of anything drifts. It exists because the schema is the published
// contract but a JSON Schema compile is too expensive to run inside int-0002's
// budget on every invocation, so the binary needs a native gate.
//
// The tests here stop the two from disagreeing. They read the schema document
// rather than restating it, so extending an enum in entry.schema.json without
// extending the Go constants fails here rather than three lanes later.

// schemaDoc is entry.schema.json, decoded far enough to read its enums and
// patterns.
type schemaDoc struct {
	Properties map[string]struct {
		Enum    []string `json:"enum"`
		Pattern string   `json:"pattern"`
		Items   *struct {
			Pattern string `json:"pattern"`
		} `json:"items"`
	} `json:"properties"`
	AllOf []conditional `json:"allOf"`
	Defs  map[string]struct {
		Pattern    string `json:"pattern"`
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
	} `json:"$defs"`
}

// A conditional is one if/then/else rule. The schema nests them one level: the
// per-kind rules at the top, and inside the decision rule a second rule that
// exempts a staged decision from carrying alternatives (dec-0003 — the regex
// tier cannot know what was rejected). The nesting is modelled here rather than
// flattened, because the exemption is the part most worth pinning.
type conditional struct {
	If struct {
		Properties map[string]struct {
			Const string `json:"const"`
		} `json:"properties"`
	} `json:"if"`
	Then *branch `json:"then"`
	Else *branch `json:"else"`
}

type branch struct {
	Properties map[string]struct {
		Enum     []string `json:"enum"`
		MinItems *int     `json:"minItems"`
	} `json:"properties"`
	Required []string      `json:"required"`
	AllOf    []conditional `json:"allOf"`
}

func loadSchema(t *testing.T) schemaDoc {
	t.Helper()

	var doc schemaDoc
	if err := json.Unmarshal(schema.Schema, &doc); err != nil {
		t.Fatalf("decoding entry.schema.json: %v", err)
	}
	if len(doc.Properties) == 0 || len(doc.AllOf) == 0 {
		t.Fatal("entry.schema.json decoded to nothing useful; this test would pass without checking anything")
	}
	return doc
}

// TestClosedSetsMatchTheSchema pins every enum the Go types restate. cst-0002
// closes the kind set and the edge set deliberately; if the schema grows one and
// the constants do not, dira rejects entries its own published contract accepts.
func TestClosedSetsMatchTheSchema(t *testing.T) {
	t.Parallel()

	doc := loadSchema(t)

	cases := []struct {
		name   string
		schema []string
		go_    []string
	}{
		{"kind", doc.Properties["kind"].Enum, strs(ledger.Kinds)},
		{"state", doc.Properties["state"].Enum, allStates()},
		{"edge type", doc.Defs["edge"].Properties["type"].Enum, strs(ledger.EdgeTypes)},
		{"source.hook", doc.Defs["source"].Properties["hook"].Enum, strs(ledger.Hooks)},
		{"source.tier", doc.Defs["source"].Properties["tier"].Enum, strs(ledger.Tiers)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.schema) == 0 {
				t.Fatalf("entry.schema.json declares no enum for %s; the check is vacuous", tc.name)
			}
			want, got := slices.Clone(tc.schema), slices.Clone(tc.go_)
			slices.Sort(want)
			slices.Sort(got)
			if !slices.Equal(want, got) {
				t.Errorf("%s\n  schema: %v\n  Go:     %v", tc.name, want, got)
			}
		})
	}
}

// TestKindStatesMatchTheSchema checks the per-kind allOf rules, which are the
// part most likely to drift because they are stated five times in the schema and
// once in Kind.States.
func TestKindStatesMatchTheSchema(t *testing.T) {
	t.Parallel()

	doc := loadSchema(t)

	checked := 0
	for _, rule := range doc.AllOf {
		kind, ok := rule.If.Properties["kind"]
		if !ok || kind.Const == "" {
			continue
		}
		if rule.Then == nil {
			continue
		}
		states, ok := rule.Then.Properties["state"]
		if !ok {
			continue
		}
		checked++

		t.Run(kind.Const, func(t *testing.T) {
			want := slices.Clone(states.Enum)
			got := strs(ledger.Kind(kind.Const).States())
			slices.Sort(want)
			slices.Sort(got)
			if !slices.Equal(want, got) {
				t.Errorf("states for %s\n  schema: %v\n  Go:     %v", kind.Const, want, got)
			}

			if kind.Const == string(ledger.KindDecision) {
				assertAlternativesRule(t, rule.Then)
			}
		})
	}

	if checked != len(ledger.Kinds) {
		t.Errorf("checked %d per-kind rules, but there are %d kinds", checked, len(ledger.Kinds))
	}
}

// assertAlternativesRule pins the conditional that E2-L1 introduced, from both
// sides: the schema must still carry it, and Entry.Validate must draw the line
// in the same place.
//
// The rule it pins: a decision carries at least one alternative, unless it is
// staged. Before E2-L1 the schema said `required: ["alternatives"]` with no
// minItems, so `alternatives: []` validated — the letter of "a decision without
// alternatives is an assertion" with none of its meaning — while Entry.Validate
// rejected the same file. That disagreement was real and untested; both halves
// are asserted here so it cannot return in either direction.
func assertAlternativesRule(t *testing.T, decision *branch) {
	t.Helper()

	var staged, other *branch
	for _, inner := range decision.AllOf {
		if inner.If.Properties["state"].Const == string(ledger.StateStaged) {
			staged, other = inner.Then, inner.Else
		}
	}
	if staged == nil || other == nil {
		t.Fatal("the decision rule no longer carries an if/then/else on state: staged. " +
			"Either the exemption was removed — in which case `dira sniff` cannot write anything (dec-0003) — " +
			"or it moved, and this test is no longer reading it.")
	}

	if !slices.Contains(other.Required, "alternatives") {
		t.Error("a non-staged decision is no longer required to carry alternatives")
	}
	if min := other.Properties["alternatives"].MinItems; min == nil || *min < 1 {
		t.Error("a non-staged decision's alternatives no longer carry minItems: 1, so `alternatives: []` validates again")
	}
	if slices.Contains(staged.Required, "alternatives") {
		t.Error("a staged decision is required to carry alternatives, which the regex tier cannot supply")
	}

	// The behavioural half. Two entries differing in exactly one field, so
	// a Validate that ignored state would fail one of them.
	base := func(state ledger.State) *ledger.Entry {
		return &ledger.Entry{
			ID: "dec-0001", Kind: ledger.KindDecision, State: state,
			Title: "A decision with nothing rejected", Created: "2026-07-29T20:00:00Z",
		}
	}
	if err := base(ledger.StateAccepted).Validate(); err == nil {
		t.Error("the schema requires an accepted decision to carry alternatives; Entry.Validate accepts one without")
	}
	if err := base(ledger.StateStaged).Validate(); err != nil {
		t.Errorf("the schema exempts a staged decision from carrying alternatives; Entry.Validate rejects one: %v", err)
	}
}

// TestPatternsMatchTheSchema pins the three regexps the Go code restates.
func TestPatternsMatchTheSchema(t *testing.T) {
	t.Parallel()

	doc := loadSchema(t)

	cases := []struct {
		name    string
		pattern string
		valid   []string
		invalid []string
		check   func(string) bool
	}{
		{
			name:    "id",
			pattern: doc.Properties["id"].Pattern,
			valid:   []string{"int-0001", "dec-0060", "qst-0007", "cst-0001", "note-0012", "dec-100000"},
			invalid: []string{"", "dec-1", "dec-001", "DEC-0001", "task-0001", "sire:int-0002", "dec-0001.md"},
			check:   ledger.ValidID,
		},
		{
			name:    "ref",
			pattern: doc.Defs["ref"].Pattern,
			valid:   []string{"dec-0060", "sire:int-0002", "me:cst-0002", "int-0001"},
			invalid: []string{"", "kazi:goal-resume", "Sire:int-0002", "dec-1", ":int-0001"},
			check:   ledger.ValidRef,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.pattern == "" {
				t.Fatalf("entry.schema.json declares no pattern for %s", tc.name)
			}
			for _, s := range tc.valid {
				if !tc.check(s) {
					t.Errorf("%q should match %s (%s)", s, tc.name, tc.pattern)
				}
			}
			for _, s := range tc.invalid {
				if tc.check(s) {
					t.Errorf("%q should not match %s (%s)", s, tc.name, tc.pattern)
				}
			}
		})
	}
}

// TestValidateAgreesWithTheSchema is the behavioural half: over E0's corpus of
// deliberately invalid fixtures, anything the schema rejects the codec must
// reject too. Anything the schema accepts, it must accept.
//
// The two are allowed to disagree about the reason — the schema reports a JSON
// Pointer, Entry.Validate reports a sentence — but never about the verdict.
func TestValidateAgreesWithTheSchema(t *testing.T) {
	t.Parallel()

	validator, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	corpora := []struct {
		dir        string
		wantValid  bool
		minFixture int
	}{
		{dir: ledgerDir, wantValid: true, minFixture: 20},
		{dir: "../../schema/testdata/valid", wantValid: true, minFixture: 3},
		{dir: "../../schema/testdata/invalid", wantValid: false, minFixture: 15},
	}

	for _, corpus := range corpora {
		paths, err := filepath.Glob(filepath.Join(corpus.dir, "*.md"))
		if err != nil {
			t.Fatalf("globbing %s: %v", corpus.dir, err)
		}
		if len(paths) < corpus.minFixture {
			t.Fatalf("%s holds %d fixtures, want at least %d; this corpus is not testing what it used to",
				corpus.dir, len(paths), corpus.minFixture)
		}

		for _, path := range paths {
			t.Run(filepath.Base(corpus.dir)+"/"+filepath.Base(path), func(t *testing.T) {
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("reading %s: %v", path, err)
				}

				schemaErr := validator.Validate(content)
				_, codecErr := ledger.Decode(content)

				if corpus.wantValid {
					if schemaErr != nil {
						t.Fatalf("the fixture is meant to be valid but the schema rejects it: %v", schemaErr)
					}
					if codecErr != nil {
						t.Errorf("the schema accepts this entry and the codec does not: %v", codecErr)
					}
					return
				}

				if schemaErr == nil {
					t.Fatalf("the fixture is meant to be invalid but the schema accepts it")
				}
				if codecErr == nil {
					t.Errorf("the schema rejects this entry and the codec accepts it; a file the contract forbids would be readable and re-writable")
				}
			})
		}
	}
}

func strs[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}

// allStates is the union of every kind's states, which is what the top-level
// state enum holds.
func allStates() []string {
	var out []string
	for _, kind := range ledger.Kinds {
		for _, state := range kind.States() {
			if !slices.Contains(out, string(state)) {
				out = append(out, string(state))
			}
		}
	}
	return out
}
