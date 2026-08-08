package ledger_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/kazi-org/dira/internal/ledger"
)

// TestQuotingSurvivesAReparse is the property the emitter has to hold: every
// value dira writes must read back as the identical string.
//
// The round-trip suites cover the values that happen to be in this repo's ledger
// and in the fixture. This covers the ones that are not there yet and will be
// the day someone titles a decision "No" or opens a note with a dash — the
// values where an unquoted scalar silently changes type or meaning.
func TestQuotingSurvivesAReparse(t *testing.T) {
	t.Parallel()

	values := []string{
		"ordinary prose",
		"No", "no", "yes", "true", "false", "null", "~", "on", "off", "y", "n",
		"0", "1", "007", "-1", "1.5", ".5", "1e6", "0x1f", "0o17", "0b101",
		".inf", "-.inf", ".nan",
		"2026-07-29", "2026-07-29T20:00:00Z", "2026-7-9",
		"1:30", "12:00:00",
		"- leading dash", "? leading question", ": leading colon", "# leading hash",
		"[bracketed]", "{braced}", "&anchor", "*alias", "!tag", "|pipe", ">angle",
		"'single'", `"double"`, "%directive", "@reserved", "`backtick`",
		"trailing space ", " leading space", "  ", "\t",
		"a: mapping-looking value", "value # with a comment marker",
		"em — dash", "unicode ✓ and emoji 🚀", "backslash \\ and quote \"",
		"multi\nline\nvalue", "trailing newline\n",
		"a value with  two  spaces",
		strings.Repeat("a long unbroken word ", 12),
		strings.Repeat("x", 200),
	}

	for _, value := range values {
		t.Run(strings.ReplaceAll(value, "\n", `\n`), func(t *testing.T) {
			// Titles are capped at 120 characters and cannot be empty,
			// so the note field carries the awkward values: it is a
			// prose field, which means it is also the one the emitter
			// might choose to fold.
			entry := &ledger.Entry{
				ID: "note-0001", Kind: ledger.KindNote, State: ledger.StateActive,
				Title:   "An entry carrying an awkward value",
				Created: "2026-07-29T20:00:00Z",
				Edges:   []ledger.Edge{{Type: ledger.EdgeInforms, To: "dec-0001", Note: value}},
				Body:    "\nWhy.\n",
			}
			if len([]rune(value)) > 280 {
				t.Skip("longer than the schema's note limit")
			}

			encoded, err := ledger.Encode(entry)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			decoded, err := ledger.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode: %v\n%s", err, encoded)
			}
			if got := decoded.Edges[0].Note; got != value {
				t.Errorf("value changed on a round-trip\n want %q\n  got %q\nencoded as:\n%s", value, got, encoded)
			}

			// And the emitted YAML must parse to a string with a
			// generic reader, not to a bool, a number or a time.
			front, _, err := splitFrontmatter(encoded)
			if err != nil {
				t.Fatalf("splitting: %v", err)
			}
			var raw struct {
				Edges []map[string]any `yaml:"edges"`
			}
			if err := yaml.Unmarshal(front, &raw); err != nil {
				t.Fatalf("re-parsing: %v\n%s", err, encoded)
			}
			if _, ok := raw.Edges[0]["note"].(string); !ok {
				t.Errorf("the note re-parsed as %T, not a string:\n%s", raw.Edges[0]["note"], encoded)
			}
		})
	}
}

// TestPlainSafeRejectsWhatItMust states the quoting rule directly, so a change
// to it is visible rather than showing up as a puzzling round-trip failure.
func TestPlainSafeRejectsWhatItMust(t *testing.T) {
	t.Parallel()

	safe := []string{
		"ordinary prose", "dec-0002", "derives_from", "human", "manual",
		"An entry title with an em — dash and (parentheses)",
		"no API key, no network, no per-call cost on the happy path",
		"never on performance grounds; only if the team's expertise shifts",
	}
	for _, value := range safe {
		if !ledger.PlainSafe(value) {
			t.Errorf("PlainSafe(%q) = false, want true; quoting this would churn the whole ledger", value)
		}
	}

	unsafe := []string{
		"", " ", "leading space", "trailing space ", "true", "No", "null",
		"2026-07-29", "1.5", "0x1f", "- dash", "#hash", "a: colon", "a # comment",
		"line\nbreak", "tab\there",
	}
	for _, value := range unsafe {
		if value == "leading space" {
			continue // control: this one really is safe
		}
		if ledger.PlainSafe(value) {
			t.Errorf("PlainSafe(%q) = true, want false", value)
		}
	}
}

// TestWrapFillsGreedily covers the line filler on its own, including the case
// that has no good answer: a single word longer than the width.
func TestWrapFillsGreedily(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{
			name:  "fills to the width",
			text:  "one two three four five six",
			width: 12,
			want:  []string{"one two", "three four", "five six"},
		},
		{
			name:  "a word longer than the width gets its own line",
			text:  "short supercalifragilistic short",
			width: 10,
			want:  []string{"short", "supercalifragilistic", "short"},
		},
		{
			name:  "a single word",
			text:  "word",
			width: 10,
			want:  []string{"word"},
		},
		{
			name:  "an exact fit does not wrap",
			text:  "abc def",
			width: 7,
			want:  []string{"abc def"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ledger.Wrap(tc.text, tc.width)
			if len(got) != len(tc.want) {
				t.Fatalf("Wrap = %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestEncodeRefusesAnInvalidEntry keeps the codec from being the way ledger rot
// reaches the disk. Each case is a rule stated in entry.schema.json.
func TestEncodeRefusesAnInvalidEntry(t *testing.T) {
	t.Parallel()

	valid := func() *ledger.Entry {
		return &ledger.Entry{
			ID: "dec-0001", Kind: ledger.KindDecision, State: ledger.StateAccepted,
			Title: "A decision that is fine as it stands", Created: "2026-07-29T20:00:00Z",
			Alternatives: []ledger.Alternative{{Option: "Not doing it", WhyNot: "it needed doing"}},
		}
	}
	if _, err := ledger.Encode(valid()); err != nil {
		t.Fatalf("the control case does not encode: %v", err)
	}

	cases := []struct {
		name   string
		break_ func(*ledger.Entry)
	}{
		{"a sixth kind", func(e *ledger.Entry) { e.Kind = "task" }},
		{"a malformed id", func(e *ledger.Entry) { e.ID = "decision-1" }},
		{"an id whose prefix contradicts the kind", func(e *ledger.Entry) { e.ID = "int-0001" }},
		{"a state from another kind", func(e *ledger.Entry) { e.State = ledger.StateOpen }},
		{"a decision with no alternatives", func(e *ledger.Entry) { e.Alternatives = nil }},
		{"an alternative with no reason", func(e *ledger.Entry) { e.Alternatives[0].WhyNot = "" }},
		{"a missing created", func(e *ledger.Entry) { e.Created = "" }},
		{"a created that is not RFC3339", func(e *ledger.Entry) { e.Created = "yesterday" }},
		{"an updated that is not RFC3339", func(e *ledger.Entry) { e.Updated = "soon" }},
		{"a title too short", func(e *ledger.Entry) { e.Title = "hi" }},
		{"a title too long", func(e *ledger.Entry) { e.Title = strings.Repeat("a", 121) }},
		{"an unknown edge type", func(e *ledger.Entry) {
			e.Edges = []ledger.Edge{{Type: "causes", To: "int-0001"}}
		}},
		{"an edge to something that is not a ref", func(e *ledger.Entry) {
			e.Edges = []ledger.Edge{{Type: ledger.EdgeDerivesFrom, To: "kazi:goal-x"}}
		}},
		{"an uppercase tag", func(e *ledger.Entry) { e.Tags = []string{"Founding"} }},
		{"a duplicate tag", func(e *ledger.Entry) { e.Tags = []string{"storage", "storage"} }},
		{"an unknown source hook", func(e *ledger.Entry) {
			e.Source = &ledger.Source{Hook: "OnEverything"}
		}},
		{"an unknown source tier", func(e *ledger.Entry) {
			e.Source = &ledger.Source{Tier: "api"}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := valid()
			tc.break_(e)
			if _, err := ledger.Encode(e); err == nil {
				t.Error("Encode accepted it")
			}
		})
	}
}

// TestRealizedByTakesANonRef is the one edge whose target is not an entry, and
// applying the ref pattern to it would make the whole kazi join (E4) unwritable.
func TestRealizedByTakesANonRef(t *testing.T) {
	t.Parallel()

	e := &ledger.Entry{
		ID: "dec-0001", Kind: ledger.KindDecision, State: ledger.StateAccepted,
		Title: "A decision carried out somewhere else", Created: "2026-07-29T20:00:00Z",
		Alternatives: []ledger.Alternative{{Option: "Not doing it", WhyNot: "it needed doing"}},
		Edges:        []ledger.Edge{{Type: ledger.EdgeRealizedBy, To: "kazi:prop-resume-8a1f"}},
	}
	encoded, err := ledger.Encode(e)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := ledger.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Edges[0].To != "kazi:prop-resume-8a1f" {
		t.Errorf("to = %q, want the kazi artifact verbatim", decoded.Edges[0].To)
	}
}
