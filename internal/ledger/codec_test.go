package ledger_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

// ledgerDir is this repo's own ledger. It is the round-trip corpus because it is
// the only entry corpus dira did not write: 26 hand-authored files carrying 46
// hand-wrapped folded scalars, six different key orderings, quoted timestamps,
// nested alternatives and prose bodies. A codec that reproduces these has been
// tested against something.
const ledgerDir = "../../.dira/entries"

func ledgerFiles(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(ledgerDir, "*.md"))
	if err != nil {
		t.Fatalf("globbing %s: %v", ledgerDir, err)
	}
	if len(paths) < 20 {
		t.Fatalf("found %d entries under %s; this suite would pass without testing anything", len(paths), ledgerDir)
	}
	return paths
}

// TestRoundTripIsByteIdentical is E1-L1's acceptance line (a) over the real
// ledger. Read every entry, write it back, and require the bytes to match.
func TestRoundTripIsByteIdentical(t *testing.T) {
	t.Parallel()

	for _, path := range ledgerFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}

			entry, err := ledger.Decode(want)
			if err != nil {
				t.Fatalf("decoding %s: %v", path, err)
			}
			got, err := ledger.Encode(entry)
			if err != nil {
				t.Fatalf("encoding %s: %v", path, err)
			}
			if string(got) != string(want) {
				t.Errorf("%s does not round-trip:\n%s", path, lineDiff(string(want), string(got)))
			}
		})
	}
}

// TestDecodeIsIdempotent proves the model, not the bytes, carries the entry.
// Decoding the re-encoded file must give back an identical Entry — including
// its recorded presentation, since that is what the second encode would use.
func TestDecodeIsIdempotent(t *testing.T) {
	t.Parallel()

	for _, path := range ledgerFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			first, err := ledger.Decode(original)
			if err != nil {
				t.Fatalf("decoding %s: %v", path, err)
			}
			encoded, err := ledger.Encode(first)
			if err != nil {
				t.Fatalf("encoding %s: %v", path, err)
			}
			second, err := ledger.Decode(encoded)
			if err != nil {
				t.Fatalf("re-decoding %s: %v", path, err)
			}

			if first.ID != second.ID || first.Kind != second.Kind || first.Title != second.Title ||
				first.State != second.State || first.Created != second.Created || first.Updated != second.Updated ||
				first.ConfirmedBy != second.ConfirmedBy || first.ADR != second.ADR || first.Private != second.Private ||
				first.Body != second.Body {
				t.Errorf("scalar fields changed across a round-trip\nfirst:  %+v\nsecond: %+v", first, second)
			}
			if len(first.Tags) != len(second.Tags) {
				t.Fatalf("tags: %v then %v", first.Tags, second.Tags)
			}
			for i := range first.Tags {
				if first.Tags[i] != second.Tags[i] {
					t.Errorf("tags[%d] = %q then %q", i, first.Tags[i], second.Tags[i])
				}
			}
			if len(first.Edges) != len(second.Edges) {
				t.Fatalf("edges: %d then %d", len(first.Edges), len(second.Edges))
			}
			for i := range first.Edges {
				if first.Edges[i] != second.Edges[i] {
					t.Errorf("edges[%d] = %+v then %+v", i, first.Edges[i], second.Edges[i])
				}
			}
			if len(first.Alternatives) != len(second.Alternatives) {
				t.Fatalf("alternatives: %d then %d", len(first.Alternatives), len(second.Alternatives))
			}
			for i := range first.Alternatives {
				if first.Alternatives[i] != second.Alternatives[i] {
					t.Errorf("alternatives[%d] = %+v then %+v", i, first.Alternatives[i], second.Alternatives[i])
				}
			}
			switch {
			case (first.Source == nil) != (second.Source == nil):
				t.Errorf("source presence changed: %v then %v", first.Source, second.Source)
			case first.Source != nil && *first.Source != *second.Source:
				t.Errorf("source = %+v then %+v", *first.Source, *second.Source)
			}
		})
	}
}

// TestEditingOneFieldRewritesOnlyThatField is what stops the round-trip test
// above from being a bytes cache echoing itself.
//
// If Decode kept the original file and Encode returned it, the round-trip test
// would pass and this one could not: changing the title must change the title
// line and nothing else. It is also the product requirement behind the style
// memo — `dira log --edge` (E1-L2) adds one edge to an existing entry, and
// dec-0002 promises that shows up as a legible diff rather than a reflow of
// every paragraph in the file.
func TestEditingOneFieldRewritesOnlyThatField(t *testing.T) {
	t.Parallel()

	const path = ledgerDir + "/dec-0002.md"
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	t.Run("changed title", func(t *testing.T) {
		entry, err := ledger.Decode(original)
		if err != nil {
			t.Fatalf("decoding: %v", err)
		}
		oldTitle := entry.Title
		entry.Title = "One file per entry, and nothing else"

		got, err := ledger.Encode(entry)
		if err != nil {
			t.Fatalf("encoding: %v", err)
		}

		before, after := strings.Split(string(original), "\n"), strings.Split(string(got), "\n")
		if len(before) != len(after) {
			t.Fatalf("line count changed from %d to %d; only the title should have moved\n%s",
				len(before), len(after), lineDiff(string(original), string(got)))
		}
		changed := 0
		for i := range before {
			if before[i] != after[i] {
				changed++
				if !strings.HasPrefix(after[i], "title: ") {
					t.Errorf("line %d changed but is not the title: %q -> %q", i+1, before[i], after[i])
				}
			}
		}
		if changed != 1 {
			t.Errorf("%d lines changed, want exactly 1\n%s", changed, lineDiff(string(original), string(got)))
		}
		if strings.Contains(string(got), oldTitle) {
			t.Error("the old title survived the edit")
		}
	})

	t.Run("added edge", func(t *testing.T) {
		entry, err := ledger.Decode(original)
		if err != nil {
			t.Fatalf("decoding: %v", err)
		}
		entry.Edges = append(entry.Edges, ledger.Edge{
			Type: ledger.EdgeInforms,
			To:   "dec-0013",
			Note: "the entry file is what the SQLite cache is derived from",
		})

		got, err := ledger.Encode(entry)
		if err != nil {
			t.Fatalf("encoding: %v", err)
		}
		if !strings.Contains(string(got), "    to: dec-0013\n") {
			t.Fatalf("the new edge is missing:\n%s", got)
		}

		// Every line of the original must survive verbatim and in order:
		// adding an edge may insert lines, never rewrite existing ones.
		before, after := strings.Split(string(original), "\n"), strings.Split(string(got), "\n")
		j := 0
		for _, line := range before {
			for j < len(after) && after[j] != line {
				j++
			}
			if j == len(after) {
				t.Fatalf("adding an edge rewrote existing content; %q no longer appears in order\n%s",
					line, lineDiff(string(original), string(got)))
			}
			j++
		}
		if extra := len(after) - len(before); extra != 3 {
			t.Errorf("adding a three-field edge changed the line count by %d, want 3", extra)
		}
	})
}

// TestCanonicalEmissionReparsesToTheSameEntry runs the write path with the style
// memo removed, which is the path every entry dira composes itself takes. The
// bytes will differ from the hand-wrapped originals — that is the whole reason
// the memo exists — but the meaning may not.
func TestCanonicalEmissionReparsesToTheSameEntry(t *testing.T) {
	t.Parallel()

	for _, path := range ledgerFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			entry, err := ledger.Decode(original)
			if err != nil {
				t.Fatalf("decoding: %v", err)
			}

			canonical, err := ledger.Encode(ledger.WithoutRecordedStyle(entry))
			if err != nil {
				t.Fatalf("encoding without recorded style: %v", err)
			}
			back, err := ledger.Decode(canonical)
			if err != nil {
				t.Fatalf("re-decoding canonical output: %v\n%s", err, canonical)
			}

			if back.Title != entry.Title || back.Created != entry.Created || back.Body != entry.Body {
				t.Errorf("canonical emission changed a value")
			}
			for i := range entry.Alternatives {
				if back.Alternatives[i] != entry.Alternatives[i] {
					t.Errorf("alternatives[%d] changed:\n got %+v\nwant %+v", i, back.Alternatives[i], entry.Alternatives[i])
				}
			}
			for i := range entry.Edges {
				if back.Edges[i] != entry.Edges[i] {
					t.Errorf("edges[%d] changed:\n got %+v\nwant %+v", i, back.Edges[i], entry.Edges[i])
				}
			}
		})
	}
}

// TestUnknownFieldIsRejected covers the additionalProperties: false half of the
// contract. A codec that dropped the key instead would delete it from the file
// on the next write, which is data loss on a round-trip through a newer schema.
func TestUnknownFieldIsRejected(t *testing.T) {
	t.Parallel()

	const src = `---
id: dec-0001
kind: decision
title: A decision with a field from the future
state: accepted
created: "2026-07-29T20:00:00Z"
priority: high
alternatives:
  - option: Not doing it
    why_not: it needed doing
---
`
	_, err := ledger.Decode([]byte(src))
	if err == nil {
		t.Fatal("an unknown field was accepted")
	}
	if !strings.Contains(err.Error(), "priority") {
		t.Errorf("the error does not name the offending field: %v", err)
	}
}

// TestBodyIsPreservedVerbatim pins the rule that the markdown body is content,
// not a field.
func TestBodyIsPreservedVerbatim(t *testing.T) {
	t.Parallel()

	const head = `---
id: note-0001
kind: note
title: A note with an awkward body
state: active
created: "2026-07-29T20:00:00Z"
---
`
	cases := []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "blank line then prose", body: "\nProse.\n"},
		{name: "trailing blank lines", body: "\nProse.\n\n\n"},
		{name: "no trailing newline", body: "\nProse."},
		{name: "contains a frontmatter delimiter", body: "\nfirst\n---\nsecond\n"},
		{name: "contains yaml-looking lines", body: "\nid: not-a-field\nkind: prose\n"},
		{name: "trailing spaces", body: "\nline with trailing space   \n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := head + tc.body
			entry, err := ledger.Decode([]byte(src))
			if err != nil {
				t.Fatalf("decoding: %v", err)
			}
			if entry.Body != tc.body {
				t.Errorf("Body = %q, want %q", entry.Body, tc.body)
			}
			got, err := ledger.Encode(entry)
			if err != nil {
				t.Fatalf("encoding: %v", err)
			}
			if string(got) != src {
				t.Errorf("round-trip = %q, want %q", got, src)
			}
		})
	}
}

// lineDiff renders the first few differing lines, because a byte-equality
// failure over a 90-line file is unreadable without one.
func lineDiff(want, got string) string {
	a, b := strings.Split(want, "\n"), strings.Split(got, "\n")
	var out strings.Builder
	shown := 0
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y string
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x == y {
			continue
		}
		if shown == 10 {
			out.WriteString("  ...\n")
			break
		}
		shown++
		out.WriteString("  line " + itoa(i+1) + "\n")
		out.WriteString("    want " + quote(x) + "\n")
		out.WriteString("    got  " + quote(y) + "\n")
	}
	return out.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func quote(s string) string { return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\"" }
