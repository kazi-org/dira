package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

const (
	schemaFile = "entry.schema.json"
	ledgerDir  = "../.dira/entries"
)

// compileSchema returns the compiled schema behind a Validator. The tests below
// predate Validator and drive the compiled schema directly so they can separate
// a parse failure from a validation failure, which Validate deliberately does
// not do for its callers.
func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()

	v, err := NewValidator()
	if err != nil {
		t.Fatalf("compiling %s: %v", schemaFile, err)
	}
	return v.schema
}

// parseEntry reads an entry file and returns its frontmatter as a JSON value.
// A failure here is a parse failure, kept distinct from a schema failure so a
// fixture cannot pass the negative half of the suite by being unparseable YAML
// when it was meant to be testing a schema rule.
func parseEntry(path string) (any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	value, err := parseEntryFile(content)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return value, nil
}

func markdownFiles(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatalf("globbing %s: %v", dir, err)
	}
	if len(paths) == 0 {
		t.Fatalf("no *.md files under %s; this suite would pass without testing anything", dir)
	}
	return paths
}

// TestLedgerValidates is the gate. Every entry dira keeps about itself is
// checked against the contract dira publishes, on every `go test ./...`.
func TestLedgerValidates(t *testing.T) {
	t.Parallel()

	sch := compileSchema(t)
	paths := markdownFiles(t, ledgerDir)
	if len(paths) < 20 {
		t.Fatalf("found %d entries under %s; the ledger had 25 at E0 and shrinking silently is itself a failure", len(paths), ledgerDir)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			entry, err := parseEntry(path)
			if err != nil {
				t.Fatalf("%s is not a readable entry: %v", path, err)
			}
			if err := sch.Validate(entry); err != nil {
				t.Errorf("%s violates %s:\n%v", path, schemaFile, err)
			}
		})
	}
}

func TestValidFixturesValidate(t *testing.T) {
	t.Parallel()

	sch := compileSchema(t)
	for _, path := range markdownFiles(t, filepath.Join("testdata", "valid")) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			entry, err := parseEntry(path)
			if err != nil {
				t.Fatalf("%s should parse: %v", path, err)
			}
			if err := sch.Validate(entry); err != nil {
				t.Errorf("%s should validate but does not:\n%v", path, err)
			}
		})
	}
}

// TestInvalidFixturesAreRejected is the half that makes the suite mean
// something. A validator that accepts everything passes TestLedgerValidates
// and proves nothing; every fixture here must be rejected, and rejected for
// the stated reason rather than incidentally.
func TestInvalidFixturesAreRejected(t *testing.T) {
	t.Parallel()

	// stage records where the rejection is expected. A fixture that is
	// meant to break a schema rule but is instead unparseable YAML would
	// otherwise pass this test while testing nothing.
	type stage int
	const (
		atParse stage = iota
		atValidate
	)

	cases := map[string]struct {
		where stage
		// want is a substring of the rejection, so a fixture that fails
		// for an unrelated reason is not silently counted as a pass.
		want string
	}{
		"id-pattern.md":                    {atValidate, "pattern"},
		"unknown-field.md":                 {atValidate, "additional properties 'priority' not allowed"},
		"decision-without-alternatives.md": {atValidate, "alternatives"},
		// E2-L1. `alternatives: []` used to validate — `required` with
		// no `minItems` — while Entry.Validate rejected the same file.
		// The staged exemption is what makes the minItems conditional,
		// and this fixture is the accepted case it does not cover.
		"decision-with-empty-alternatives.md": {atValidate, "minItems"},
		"intent-in-decision-state.md":         {atValidate, "state"},
		"missing-created.md":                  {atValidate, "created"},
		"unknown-kind.md":                     {atValidate, "kind"},
		"unknown-edge-type.md":                {atValidate, "type"},
		"edge-without-target.md":              {atValidate, "to"},
		"title-too-short.md":                  {atValidate, "minLength"},
		"uppercase-tag.md":                    {atValidate, "pattern"},
		"duplicate-tags.md":                   {atValidate, "at '/tags': items at 0 and 1 are equal"},
		"alternative-without-why-not.md":      {atValidate, "why_not"},
		"bad-created-format.md":               {atValidate, "date-time"},
		"unknown-source-tier.md":              {atValidate, "tier"},
		"edges-not-a-list.md":                 {atValidate, "array"},
		"no-frontmatter.md":                   {atParse, "frontmatter"},
		"unterminated-frontmatter.md":         {atParse, "frontmatter"},
	}

	dir := filepath.Join("testdata", "invalid")
	paths := markdownFiles(t, dir)

	// Every fixture on disk must be accounted for, so adding one without
	// stating what it proves fails the suite rather than joining it mutely.
	onDisk := make(map[string]bool, len(paths))
	for _, path := range paths {
		onDisk[filepath.Base(path)] = true
	}
	for name := range cases {
		if !onDisk[name] {
			t.Errorf("case %q has no fixture in %s", name, dir)
		}
	}
	for name := range onDisk {
		if _, ok := cases[name]; !ok {
			t.Errorf("fixture %s is not covered by a case; state the rule it violates", filepath.Join(dir, name))
		}
	}

	sch := compileSchema(t)
	for _, path := range paths {
		name := filepath.Base(path)
		tc, ok := cases[name]
		if !ok {
			continue
		}
		t.Run(name, func(t *testing.T) {
			entry, err := parseEntry(path)

			if tc.where == atParse {
				if err == nil {
					t.Fatalf("%s parsed successfully; it must be rejected as unreadable", path)
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Errorf("%s was rejected, but not for the stated reason\n got: %v\nwant substring: %q", path, err, tc.want)
				}
				return
			}

			if err != nil {
				t.Fatalf("%s must fail schema validation, but it failed to parse instead — the fixture is testing the wrong thing: %v", path, err)
			}
			verr := sch.Validate(entry)
			if verr == nil {
				t.Fatalf("%s validated successfully; it must violate %s", path, schemaFile)
			}
			if !strings.Contains(verr.Error(), tc.want) {
				t.Errorf("%s was rejected, but not for the stated reason\n got: %v\nwant substring: %q", path, verr, tc.want)
			}
		})
	}
}

// TestJSONValueFlattensTimestamps covers the yaml.v3 !!timestamp rule at the
// function that states it, rather than only through the fixture. Both an
// unquoted and a quoted timestamp must arrive at the validator as the same
// RFC3339 string.
func TestJSONValueFlattensTimestamps(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		yaml string
	}{
		{name: "unquoted", yaml: "created: 2026-07-29T20:00:00Z\n"},
		{name: "quoted", yaml: "created: \"2026-07-29T20:00:00Z\"\n"},
		{name: "unquoted with offset", yaml: "created: 2026-07-29T22:00:00+02:00\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var raw any
			if err := yaml.Unmarshal([]byte(tc.yaml), &raw); err != nil {
				t.Fatalf("unmarshalling %q: %v", tc.yaml, err)
			}
			converted, err := jsonValue(raw)
			if err != nil {
				t.Fatalf("jsonValue(%q): %v", tc.yaml, err)
			}
			got, ok := converted.(map[string]any)["created"].(string)
			if !ok {
				t.Fatalf("created is %T, want string", converted.(map[string]any)["created"])
			}
			if got != "2026-07-29T20:00:00Z" {
				t.Errorf("created = %q, want %q", got, "2026-07-29T20:00:00Z")
			}
		})
	}
}

// TestTimestampsAreQuotedOnDisk pins the write-side half of the timestamp
// rule. jsonValue accepts unquoted timestamps on read; entries must still be
// written quoted, so a consumer with a stricter YAML reader than ours can read
// this ledger.
func TestTimestampsAreQuotedOnDisk(t *testing.T) {
	t.Parallel()

	for _, path := range markdownFiles(t, ledgerDir) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			front, _, err := SplitFrontmatter(content)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			for _, line := range strings.Split(string(front), "\n") {
				for _, field := range []string{"created:", "updated:"} {
					value, ok := strings.CutPrefix(strings.TrimSpace(line), field)
					if !ok {
						continue
					}
					value = strings.TrimSpace(value)
					if !strings.HasPrefix(value, `"`) && !strings.HasPrefix(value, "'") {
						t.Errorf("%s: %s must be quoted, got %s", path, strings.TrimSuffix(field, ":"), value)
					}
				}
			}
		})
	}
}
