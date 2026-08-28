package ledger_test

import (
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/schema"
)

// entryWithAppliesWhen is a minimal, otherwise-valid constraint entry carrying
// an applies_when clause, used as the base fixture across this file.
const entryWithAppliesWhen = `---
id: cst-9001
kind: constraint
title: A fixture constraint carrying an applies_when clause
state: active
created: "2026-08-28T00:00:00Z"
applies_when:
  action: fixture_action
  params:
    threshold: 500
    currency: usd
---

Fixture body text.
`

// TestAppliesWhenDecodes proves the new field parses into the Entry model
// with the right shape, including a nested params value.
func TestAppliesWhenDecodes(t *testing.T) {
	t.Parallel()

	e, err := ledger.Decode([]byte(entryWithAppliesWhen))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if e.AppliesWhen == nil {
		t.Fatal("AppliesWhen is nil, want a populated clause")
	}
	if e.AppliesWhen.Action != "fixture_action" {
		t.Errorf("Action = %q, want %q", e.AppliesWhen.Action, "fixture_action")
	}
	if got, want := e.AppliesWhen.Params["threshold"], 500; got != want {
		t.Errorf("Params[threshold] = %v (%T), want %v", got, got, want)
	}
	if got, want := e.AppliesWhen.Params["currency"], "usd"; got != want {
		t.Errorf("Params[currency] = %v, want %v", got, want)
	}
}

// TestAppliesWhenRoundTrips proves an entry carrying the field survives an
// encode/decode cycle with its values intact. Byte-identity is not claimed —
// see appliesWhen in encode.go for why params does not carry the style-memo
// guarantee the rest of this codec gives every scalar field — only that
// decoding what was just encoded gives back the same clause.
func TestAppliesWhenRoundTrips(t *testing.T) {
	t.Parallel()

	first, err := ledger.Decode([]byte(entryWithAppliesWhen))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	encoded, err := ledger.Encode(first)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	second, err := ledger.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode of re-encoded entry: %v\nre-encoded:\n%s", err, encoded)
	}
	if second.AppliesWhen == nil {
		t.Fatal("AppliesWhen is nil after round trip")
	}
	if second.AppliesWhen.Action != first.AppliesWhen.Action {
		t.Errorf("Action = %q after round trip, want %q", second.AppliesWhen.Action, first.AppliesWhen.Action)
	}
	if got, want := second.AppliesWhen.Params["threshold"], first.AppliesWhen.Params["threshold"]; got != want {
		t.Errorf("Params[threshold] = %v after round trip, want %v", got, want)
	}
}

// TestAppliesWhenOmittedByDefault proves the field's absence changes nothing
// for the entries that predate it: every entry this repo already ships
// decodes with a nil AppliesWhen and encodes with no applies_when line at
// all — the whole point of adding it as optional.
func TestAppliesWhenOmittedByDefault(t *testing.T) {
	t.Parallel()

	const noClause = `---
id: int-9002
kind: intent
title: A fixture intent carrying no trigger clause
state: active
created: "2026-08-28T00:00:00Z"
---

Fixture body text.
`
	e, err := ledger.Decode([]byte(noClause))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if e.AppliesWhen != nil {
		t.Fatalf("AppliesWhen = %+v, want nil", e.AppliesWhen)
	}
	encoded, err := ledger.Encode(e)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if strings.Contains(string(encoded), "applies_when") {
		t.Errorf("encoded entry mentions applies_when though none was set:\n%s", encoded)
	}
}

// TestAppliesWhenRequiresAction proves Validate rejects a clause with no
// action, matching the schema's appliesWhen $def, which requires it.
func TestAppliesWhenRequiresAction(t *testing.T) {
	t.Parallel()

	const missingAction = `---
id: cst-9002
kind: constraint
title: A fixture constraint with a malformed applies_when clause
state: active
created: "2026-08-28T00:00:00Z"
applies_when:
  params:
    threshold: 500
---

Fixture body text.
`
	if _, err := ledger.Decode([]byte(missingAction)); err == nil {
		t.Fatal("Decode succeeded on an applies_when clause with no action, want an error")
	}
}

// TestAppliesWhenRejectsUnknownField proves the field stays subject to the
// same additionalProperties: false discipline as every other mapping in this
// schema, per decode.go's mapping: an unrecognized key inside applies_when
// must fail loudly rather than be silently dropped on the next write.
func TestAppliesWhenRejectsUnknownField(t *testing.T) {
	t.Parallel()

	const unknownField = `---
id: cst-9003
kind: constraint
title: A fixture constraint with an unrecognized applies_when field
state: active
created: "2026-08-28T00:00:00Z"
applies_when:
  action: fixture_action
  operator: unexpected
---

Fixture body text.
`
	if _, err := ledger.Decode([]byte(unknownField)); err == nil {
		t.Fatal("Decode succeeded with an unrecognized applies_when field, want an error")
	}
}

// TestAppliesWhenAgreesWithSchema is TestValidateAgreesWithTheSchema's shape,
// narrowed to the one clause this proposal adds: the compiled JSON Schema and
// Entry.Validate must reach the same verdict on both the valid and the
// malformed fixture above.
func TestAppliesWhenAgreesWithSchema(t *testing.T) {
	t.Parallel()

	validator, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	cases := []struct {
		name  string
		entry string
		valid bool
	}{
		{"valid clause", entryWithAppliesWhen, true},
		{"missing action", `---
id: cst-9004
kind: constraint
title: A fixture constraint with a malformed applies_when clause
state: active
created: "2026-08-28T00:00:00Z"
applies_when:
  params:
    threshold: 500
---

Fixture body text.
`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schemaErr := validator.Validate([]byte(tc.entry))
			_, codecErr := ledger.Decode([]byte(tc.entry))

			if tc.valid {
				if schemaErr != nil {
					t.Errorf("schema rejects a valid clause: %v", schemaErr)
				}
				if codecErr != nil {
					t.Errorf("codec rejects a valid clause: %v", codecErr)
				}
				return
			}
			if schemaErr == nil {
				t.Error("schema accepts a clause with no action, want a rejection")
			}
			if codecErr == nil {
				t.Error("codec accepts a clause with no action, want a rejection")
			}
		})
	}
}
