package ledger_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/kazi-org/dira/internal/ledger"
)

// TestTimestampsSurviveAsStrings is E1-L1's acceptance line (b).
//
// The landmine is yaml.v3's YAML 1.1 !!timestamp resolution: an unquoted
// RFC3339 scalar decodes to a time.Time, which is not a JSON type, so a JSON
// Schema validator handed one reports `invalid jsonType time.Time` at /created
// — an error naming a field that is in fact correct. It has broken validation in
// this repo once already.
//
// Three things have to hold together, and each subtest below fails on its own if
// the coercion returns: an unquoted input parses to the string it was written
// as, the written form is quoted, and re-reading the written form yields a
// string rather than a time.Time.
func TestTimestampsSurviveAsStrings(t *testing.T) {
	t.Parallel()

	const unquoted = `---
id: note-9001
kind: note
title: Timestamps written without quotes must still validate
state: active
created: 2026-07-29T20:00:00Z
updated: 2026-07-30T02:00:00Z
---

Body.
`

	t.Run("an unquoted input parses to a string", func(t *testing.T) {
		entry, err := ledger.Decode([]byte(unquoted))
		if err != nil {
			t.Fatalf("decoding: %v", err)
		}
		if entry.Created != "2026-07-29T20:00:00Z" {
			t.Errorf("Created = %q, want %q", entry.Created, "2026-07-29T20:00:00Z")
		}
		if entry.Updated != "2026-07-30T02:00:00Z" {
			t.Errorf("Updated = %q, want %q", entry.Updated, "2026-07-30T02:00:00Z")
		}
	})

	t.Run("the written form is quoted", func(t *testing.T) {
		entry, err := ledger.Decode([]byte(unquoted))
		if err != nil {
			t.Fatalf("decoding: %v", err)
		}
		got, err := ledger.Encode(entry)
		if err != nil {
			t.Fatalf("encoding: %v", err)
		}
		for _, want := range []string{
			"created: \"2026-07-29T20:00:00Z\"\n",
			"updated: \"2026-07-30T02:00:00Z\"\n",
		} {
			if !strings.Contains(string(got), want) {
				t.Errorf("output does not contain %q:\n%s", want, got)
			}
		}
	})

	t.Run("the written form re-reads as a string, not a time.Time", func(t *testing.T) {
		entry, err := ledger.Decode([]byte(unquoted))
		if err != nil {
			t.Fatalf("decoding: %v", err)
		}
		got, err := ledger.Encode(entry)
		if err != nil {
			t.Fatalf("encoding: %v", err)
		}

		// This is the assertion that actually catches the coercion.
		// Decoding into `any` is what every naive reader does — including
		// the schema validator — so if the emitted scalar is unquoted,
		// this is where a time.Time shows up.
		front, _, err := splitFrontmatter(got)
		if err != nil {
			t.Fatalf("splitting frontmatter: %v", err)
		}
		var raw map[string]any
		if err := yaml.Unmarshal(front, &raw); err != nil {
			t.Fatalf("re-parsing frontmatter: %v", err)
		}
		for _, field := range []string{"created", "updated"} {
			switch v := raw[field].(type) {
			case string:
				if _, err := time.Parse(time.RFC3339, v); err != nil {
					t.Errorf("%s = %q, which is not RFC3339: %v", field, v, err)
				}
			case time.Time:
				t.Errorf("%s round-tripped as time.Time (%v); yaml.v3 resolved it to !!timestamp, so it was written unquoted", field, v)
			default:
				t.Errorf("%s is %T, want string", field, v)
			}
		}
	})

	t.Run("a quoted input stays quoted", func(t *testing.T) {
		quoted := strings.NewReplacer(
			"created: 2026-07-29T20:00:00Z", `created: "2026-07-29T20:00:00Z"`,
			"updated: 2026-07-30T02:00:00Z", `updated: "2026-07-30T02:00:00Z"`,
		).Replace(unquoted)

		entry, err := ledger.Decode([]byte(quoted))
		if err != nil {
			t.Fatalf("decoding: %v", err)
		}
		got, err := ledger.Encode(entry)
		if err != nil {
			t.Fatalf("encoding: %v", err)
		}
		if string(got) != quoted {
			t.Errorf("a quoted entry did not round-trip:\n%s", lineDiff(quoted, string(got)))
		}
	})

	t.Run("E0's unquoted-timestamp fixture", func(t *testing.T) {
		const path = "../../schema/testdata/valid/unquoted-timestamp.md"
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		entry, err := ledger.Decode(content)
		if err != nil {
			t.Fatalf("decoding %s: %v", path, err)
		}
		got, err := ledger.Encode(entry)
		if err != nil {
			t.Fatalf("encoding: %v", err)
		}
		if !strings.Contains(string(got), `created: "2026-07-29T20:00:00Z"`) {
			t.Errorf("the fixture's timestamp was not quoted on write:\n%s", got)
		}
		// The body is prose about this very rule; it must be untouched.
		if !strings.Contains(entry.Body, "dira quotes timestamps on write") {
			t.Error("the fixture's body did not survive decoding")
		}
	})

	t.Run("a malformed timestamp is rejected", func(t *testing.T) {
		bad := strings.Replace(unquoted, "created: 2026-07-29T20:00:00Z", `created: "yesterday"`, 1)
		if _, err := ledger.Decode([]byte(bad)); err == nil {
			t.Fatal("a non-RFC3339 created was accepted")
		}
	})
}

// splitFrontmatter is a local copy of the split, kept here so this test does not
// route its assertion through the same package it is checking.
func splitFrontmatter(content []byte) (front []byte, body []byte, err error) {
	text := string(content)
	if !strings.HasPrefix(text, "---\n") {
		return nil, nil, errNoFrontmatterInTest
	}
	rest := text[4:]
	i := strings.Index(rest, "\n---\n")
	if i < 0 {
		return nil, nil, errNoFrontmatterInTest
	}
	return []byte(rest[:i+1]), []byte(rest[i+len("\n---\n"):]), nil
}

var errNoFrontmatterInTest = errTest("no frontmatter")

type errTest string

func (e errTest) Error() string { return string(e) }
