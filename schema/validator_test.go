package schema

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidatorAcceptsTheLedger drives the exported entry point over the same
// corpus the unexported helpers are tested on. The two must not be able to
// disagree: Validate is what internal/ledger's fixture is checked with, so a
// Validator that accepts more than the schema would make that check vacuous.
func TestValidatorAcceptsTheLedger(t *testing.T) {
	t.Parallel()

	v, err := NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	for _, path := range markdownFiles(t, ledgerDir) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			if err := v.Validate(content); err != nil {
				t.Errorf("%s: %v", path, err)
			}
		})
	}
}

// TestValidatorRejectsInvalidFixtures is the half that makes the exported API
// mean something. Every fixture E0 wrote to be rejected must still be rejected
// through Validate.
func TestValidatorRejectsInvalidFixtures(t *testing.T) {
	t.Parallel()

	v, err := NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	dir := filepath.Join("testdata", "invalid")
	for _, path := range markdownFiles(t, dir) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			if err := v.Validate(content); err == nil {
				t.Errorf("%s validated successfully; it must be rejected", path)
			}
		})
	}
}

// TestSplitFrontmatterReturnsTheBodyVerbatim pins the half of SplitFrontmatter
// that entry_test.go never exercises. internal/ledger's codec relies on the
// body arriving byte for byte, trailing newlines included — the body is the
// entry's prose "because", not a field, and a codec that trimmed it would lose
// content on every round-trip.
func TestSplitFrontmatterReturnsTheBodyVerbatim(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		content   string
		wantFront string
		wantBody  string
		wantErr   error
	}{
		{
			name:      "body after a blank line",
			content:   "---\nid: dec-0001\n---\n\nProse here.\n",
			wantFront: "id: dec-0001\n",
			wantBody:  "\nProse here.\n",
		},
		{
			name:      "empty body",
			content:   "---\nid: dec-0001\n---\n",
			wantFront: "id: dec-0001\n",
			wantBody:  "",
		},
		{
			name:      "body containing its own --- line",
			content:   "---\nid: dec-0001\n---\n\nfirst\n---\nsecond\n",
			wantFront: "id: dec-0001\n",
			wantBody:  "\nfirst\n---\nsecond\n",
		},
		{
			name:    "no frontmatter at all",
			content: "just prose\n",
			wantErr: ErrNoFrontmatter,
		},
		{
			name:    "frontmatter never closed",
			content: "---\nid: dec-0001\n",
			wantErr: ErrNoFrontmatter,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			front, body, err := SplitFrontmatter([]byte(tc.content))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			if string(front) != tc.wantFront {
				t.Errorf("front = %q, want %q", front, tc.wantFront)
			}
			if string(body) != tc.wantBody {
				t.Errorf("body = %q, want %q", body, tc.wantBody)
			}
		})
	}
}

// TestSchemaIsEmbedded guards against the embed silently resolving to nothing,
// which would leave NewValidator compiling an empty document that accepts
// everything.
func TestSchemaIsEmbedded(t *testing.T) {
	t.Parallel()

	if len(Schema) == 0 {
		t.Fatal("embedded schema is empty")
	}
	onDisk, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("reading %s: %v", schemaFile, err)
	}
	if string(Schema) != string(onDisk) {
		t.Errorf("embedded schema differs from %s on disk", schemaFile)
	}
	if !strings.Contains(string(Schema), `"additionalProperties": false`) {
		t.Error("embedded schema does not close the object; unknown keys would be accepted")
	}
}
