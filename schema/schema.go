// Package schema holds the dira entry contract, entry.schema.json, and the
// validator that keeps a ledger from drifting away from it.
//
// The schema is the contract; the Go types in internal/ledger are one
// implementation of it. Nothing in the dira command path imports this package:
// JSON Schema validation costs a compile of the schema document on every
// invocation, which int-0002's budget cannot absorb. It is a gate for tests and
// for `dira check`-shaped work that is already paying for a full ledger read.
package schema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// Schema is entry.schema.json, embedded so a validator never depends on the
// working directory or on the file surviving into a release artifact. go:embed
// cannot reach above its own directory, which is why this package lives beside
// the schema rather than under internal/.
//
//go:embed entry.schema.json
var Schema []byte

// ErrNoFrontmatter marks a file that is not an entry at all, as opposed to an
// entry that is wrong. The distinction matters: the first is a stray file, the
// second is ledger rot, and only the second should fail a ledger-wide gate.
var ErrNoFrontmatter = errors.New("no YAML frontmatter")

// schemaURL is the $id entry.schema.json declares. Registering the document
// under its own $id is what stops the compiler reaching the network to resolve
// it — cst-0004, and a test that needs DNS is a test that fails on a plane.
const schemaURL = "https://github.com/kazi-org/dira/schema/entry.schema.json"

// A Validator checks entry files against entry.schema.json. Compiling the
// schema costs milliseconds, so callers validating more than one entry build
// one Validator and reuse it.
//
// A Validator is read-only after construction and safe for concurrent use.
type Validator struct {
	schema *jsonschema.Schema
}

// NewValidator compiles the embedded schema with format assertion switched on.
// Format assertion is annotation-only by default in draft 2020-12, so without
// it `created: "yesterday"` validates.
func NewValidator() (*Validator, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(Schema))
	if err != nil {
		return nil, fmt.Errorf("parsing embedded entry.schema.json: %w", err)
	}

	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(schemaURL, doc); err != nil {
		return nil, fmt.Errorf("registering entry.schema.json as %s: %w", schemaURL, err)
	}
	sch, err := c.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("compiling entry.schema.json: %w", err)
	}
	return &Validator{schema: sch}, nil
}

// Validate checks one entry file — the whole file, frontmatter and body — and
// returns nil when the frontmatter satisfies entry.schema.json. The markdown
// body is not part of the contract and is not inspected.
//
// A file with no readable frontmatter fails with an error wrapping
// ErrNoFrontmatter, so a caller can tell a stray file from an invalid entry.
func (v *Validator) Validate(entryFile []byte) error {
	value, err := parseEntryFile(entryFile)
	if err != nil {
		return err
	}
	if err := v.schema.Validate(value); err != nil {
		return fmt.Errorf("entry violates entry.schema.json: %w", err)
	}
	return nil
}

// SplitFrontmatter returns the YAML frontmatter of a dira entry file and the
// markdown body that follows it. An entry opens with a `---` line and closes
// the block with another; everything after the closing delimiter is the body,
// returned byte for byte.
func SplitFrontmatter(content []byte) (front, body []byte, err error) {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, nil, ErrNoFrontmatter
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
			return []byte(rest[:offset]), []byte(rest[next:]), nil
		}
		offset = next
	}
	return nil, nil, fmt.Errorf("%w: frontmatter opened but never closed", ErrNoFrontmatter)
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

// parseEntryFile reads an entry file and returns its frontmatter as a JSON
// value. A failure here is a parse failure, kept distinct from a schema failure
// so a fixture cannot pass the negative half of a suite by being unparseable
// YAML when it was meant to be testing a schema rule.
func parseEntryFile(content []byte) (any, error) {
	front, _, err := SplitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var raw any
	if err := yaml.Unmarshal(front, &raw); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}
	if raw == nil {
		return nil, fmt.Errorf("%w: frontmatter is empty", ErrNoFrontmatter)
	}

	converted, err := jsonValue(raw)
	if err != nil {
		return nil, err
	}

	// Round-tripping through JSON normalises the remaining numeric and map
	// types onto exactly what the validator expects.
	encoded, err := json.Marshal(converted)
	if err != nil {
		return nil, fmt.Errorf("re-encoding frontmatter: %w", err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("decoding frontmatter: %w", err)
	}
	return value, nil
}
