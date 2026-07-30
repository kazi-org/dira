// Package schema holds the dira entry contract, entry.schema.json, and the
// test that keeps the ledger from drifting away from it.
//
// The package currently has no non-test source. That is deliberate: E0-L1 owes
// the repo a gate, not an API. E0-L2 adds the go:embed of entry.schema.json
// here (go:embed cannot reach above its own directory, which is why the
// package lives beside the file rather than under internal/) and moves the
// parsing and validation below into exported functions the binary can call.
package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

const (
	schemaFile = "entry.schema.json"
	ledgerDir  = "../.dira/entries"
)

// errNoFrontmatter is returned for a file that is not an entry at all, as
// opposed to an entry that is wrong. The distinction matters: the first is a
// stray file, the second is ledger rot.
var errNoFrontmatter = errors.New("no YAML frontmatter")

// compileSchema loads entry.schema.json under its own $id, with format
// assertion switched on. Format assertion is annotation-only by default in
// draft 2020-12, so without this call `created: "yesterday"` validates.
func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()

	f, err := os.Open(schemaFile)
	if err != nil {
		t.Fatalf("opening %s: %v", schemaFile, err)
	}
	defer func() { _ = f.Close() }()

	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("parsing %s: %v", schemaFile, err)
	}

	// Register under the schema's own $id so the compiler never reaches the
	// network to resolve it — cst-0004, and a test that needs DNS is a test
	// that fails on a plane.
	url := schemaFile
	if obj, ok := doc.(map[string]any); ok {
		if id, ok := obj["$id"].(string); ok && id != "" {
			url = id
		}
	}

	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(url, doc); err != nil {
		t.Fatalf("adding %s as %s: %v", schemaFile, url, err)
	}
	sch, err := c.Compile(url)
	if err != nil {
		t.Fatalf("compiling %s: %v", schemaFile, err)
	}
	return sch
}

// splitFrontmatter returns the YAML frontmatter of a dira entry file. An entry
// opens with a `---` line and closes the block with another.
func splitFrontmatter(content []byte) ([]byte, error) {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, errNoFrontmatter
	}
	rest := text[len("---\n"):]

	// The closing delimiter is a line that is exactly "---".
	for offset := 0; offset < len(rest); {
		end := strings.IndexByte(rest[offset:], '\n')
		line := rest[offset:]
		next := len(rest)
		if end >= 0 {
			line = rest[offset : offset+end]
			next = offset + end + 1
		}
		if strings.TrimRight(line, " \t") == "---" {
			return []byte(rest[:offset]), nil
		}
		offset = next
	}
	return nil, fmt.Errorf("%w: frontmatter opened but never closed", errNoFrontmatter)
}

// jsonValue converts a yaml.v3-decoded value into the shape encoding/json
// would have produced, which is the only shape a JSON Schema validator can
// reason about.
//
// The load-bearing case is time.Time. yaml.v3 resolves an unquoted RFC3339
// scalar to the !!timestamp tag and hands back a time.Time, which is not a
// JSON type: a validator handed one reports `invalid jsonType time.Time` at
// /created, which says nothing about the actual problem and points at a field
// that is in fact correct. dira quotes timestamps on write and accepts both on
// read, so this converts rather than complains.
//
// The JSON round-trip in parseEntry would also flatten a time.Time, since
// time.Time implements json.Marshaler. Converting here anyway keeps the rule
// stated where it is read, and keeps it true for callers that skip the
// round-trip — which is what E0-L2's exported reader will do, for speed.
func jsonValue(v any) (any, error) {
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(time.RFC3339), nil

	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			converted, err := jsonValue(val)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", k, err)
			}
			out[k] = converted
		}
		return out, nil

	case map[any]any:
		// yaml.v3 falls back to this when a mapping has a non-string key.
		out := make(map[string]any, len(t))
		for k, val := range t {
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("non-string mapping key %v (%T)", k, k)
			}
			converted, err := jsonValue(val)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", key, err)
			}
			out[key] = converted
		}
		return out, nil

	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			converted, err := jsonValue(val)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			out[i] = converted
		}
		return out, nil

	default:
		return v, nil
	}
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

	front, err := splitFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	var raw any
	if err := yaml.Unmarshal(front, &raw); err != nil {
		return nil, fmt.Errorf("%s: parsing frontmatter: %w", filepath.Base(path), err)
	}
	if raw == nil {
		return nil, fmt.Errorf("%s: %w: frontmatter is empty", filepath.Base(path), errNoFrontmatter)
	}

	converted, err := jsonValue(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	// Round-tripping through JSON normalises the remaining numeric and
	// map types onto exactly what the validator expects.
	encoded, err := json.Marshal(converted)
	if err != nil {
		return nil, fmt.Errorf("%s: re-encoding frontmatter: %w", filepath.Base(path), err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("%s: decoding frontmatter: %w", filepath.Base(path), err)
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
		"intent-in-decision-state.md":      {atValidate, "state"},
		"missing-created.md":               {atValidate, "created"},
		"unknown-kind.md":                  {atValidate, "kind"},
		"unknown-edge-type.md":             {atValidate, "type"},
		"edge-without-target.md":           {atValidate, "to"},
		"title-too-short.md":               {atValidate, "minLength"},
		"uppercase-tag.md":                 {atValidate, "pattern"},
		"duplicate-tags.md":                {atValidate, "at '/tags': items at 0 and 1 are equal"},
		"alternative-without-why-not.md":   {atValidate, "why_not"},
		"bad-created-format.md":            {atValidate, "date-time"},
		"unknown-source-tier.md":           {atValidate, "tier"},
		"edges-not-a-list.md":              {atValidate, "array"},
		"no-frontmatter.md":                {atParse, "frontmatter"},
		"unterminated-frontmatter.md":      {atParse, "frontmatter"},
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
			front, err := splitFrontmatter(content)
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
