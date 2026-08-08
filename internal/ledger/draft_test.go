package ledger_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

// draftDocument is an entry as an agent would hand it to `dira log --stdin`:
// the file format dira stores, minus the id it does not get to choose. The
// wrapping is deliberately hand-done and deliberately not what a greedy wrapper
// would produce, because reproducing it is the point.
const draftDocument = `---
kind: decision
title: Vendor the tokenizer rather than fetch a BPE file at runtime
state: accepted
tags: [brief, offline]
edges:
  - type: derives_from
    to: int-0002
    note: a hook in the latency path cannot wait on a download
alternatives:
  - option: Fetch the BPE file on first run and cache it
    why_not: >
      It puts a network call in the first invocation of a tool
      that promises to work offline, and the failure mode is a
      hook that hangs rather than one that is slow.
    revisit_if: dira grows a network path for some other reason
source:
  hook: Stop
  session: 01J8Z
  tier: semantic
confirmed_by: agent:claude-code
---

The counter has to be in the binary, so the tokenizer has to be too.
`

func TestDecodeDraftReadsAnEntryThatHasNoIDYet(t *testing.T) {
	t.Parallel()

	e, err := ledger.DecodeDraft([]byte(draftDocument))
	if err != nil {
		t.Fatalf("DecodeDraft: %v", err)
	}

	if e.ID != "" {
		t.Errorf("ID = %q, want empty until it is allocated", e.ID)
	}
	if e.Kind != ledger.KindDecision {
		t.Errorf("Kind = %q", e.Kind)
	}
	if e.Created != "" {
		t.Errorf("Created = %q, want empty so the caller stamps it", e.Created)
	}
	if len(e.Alternatives) != 1 || e.Alternatives[0].RevisitIf == "" {
		t.Errorf("Alternatives = %+v", e.Alternatives)
	}
	if len(e.Edges) != 1 || e.Edges[0].To != "int-0002" {
		t.Errorf("Edges = %+v", e.Edges)
	}
	if e.Source == nil || e.Source.Tier != ledger.TierSemantic {
		t.Errorf("Source = %+v", e.Source)
	}
	if !strings.HasPrefix(e.Body, "\nThe counter has to be in the binary") {
		t.Errorf("Body = %q", e.Body)
	}
}

func TestDecodeDraftRefusesACallerChosenID(t *testing.T) {
	t.Parallel()

	// An entry that picks its own id is how two concurrent writers end up
	// with one entry, which is exactly what allocation exists to prevent.
	document := strings.Replace(draftDocument, "kind: decision", "id: dec-0400\nkind: decision", 1)

	_, err := ledger.DecodeDraft([]byte(document))
	if err == nil {
		t.Fatal("DecodeDraft accepted an entry carrying its own id")
	}
	if !strings.Contains(err.Error(), "dec-0400") {
		t.Errorf("error does not name the id it refused: %v", err)
	}
}

func TestADraftIsHeldToTheSameRulesAsAStoredEntry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		document string
		want     string
	}{
		{
			name:     "a sixth kind is refused by name",
			document: strings.Replace(draftDocument, "kind: decision", "kind: task", 1),
			want:     "cst-0002",
		},
		{
			name: "a decision with no alternatives",
			document: `---
kind: decision
title: A decision that is really just an assertion
state: accepted
---

Because.
`,
			want: "alternative",
		},
		{
			name: "an unknown field is not silently dropped",
			document: `---
kind: note
title: A note carrying a field from the future
state: active
priority: high
---
`,
			want: "unknown field",
		},
		{
			name: "an alternative with no why_not",
			document: `---
kind: decision
title: A decision listing a bare option
state: accepted
alternatives:
  - option: The other way
---
`,
			want: "why_not is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := ledger.DecodeDraft([]byte(tc.document))
			if err == nil {
				t.Fatal("DecodeDraft accepted an entry a stored one could not be")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error does not name the problem; want substring %q, got %v", tc.want, err)
			}
		})
	}
}

// TestADraftKeepsTheAuthorsFormatting is the property that makes the document
// the right interface for an agent: what the caller wrote is what the ledger
// holds. If dira reflowed it, the file in the repository would not be the file
// the author reviewed.
func TestADraftKeepsTheAuthorsFormatting(t *testing.T) {
	t.Parallel()

	e, err := ledger.DecodeDraft([]byte(draftDocument))
	if err != nil {
		t.Fatalf("DecodeDraft: %v", err)
	}
	e.ID = "dec-0400"
	e.Created = "2026-07-30T09:00:00Z"

	out, err := ledger.Encode(e)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	written := string(out)

	// The hand-wrapped folded scalar, line for line.
	for _, line := range []string{
		"      It puts a network call in the first invocation of a tool",
		"      that promises to work offline, and the failure mode is a",
		"      hook that hangs rather than one that is slow.",
	} {
		if !strings.Contains(written, line+"\n") {
			t.Errorf("the written entry does not carry the author's line %q\n--- written ---\n%s", line, written)
		}
	}

	// The two fields dira supplied, and the timestamp quoted (yaml.v3
	// resolves an unquoted RFC3339 scalar to a time.Time).
	if !strings.Contains(written, "id: dec-0400\n") {
		t.Errorf("the allocated id is not in the written entry:\n%s", written)
	}
	if !strings.Contains(written, `created: "2026-07-30T09:00:00Z"`) {
		t.Errorf("created is not written as a quoted string:\n%s", written)
	}

	// And it is still the same entry.
	back, err := ledger.Decode(out)
	if err != nil {
		t.Fatalf("the written entry does not read back: %v", err)
	}
	if back.Alternatives[0].WhyNot != e.Alternatives[0].WhyNot {
		t.Errorf("why_not changed on the round trip:\n got %q\nwant %q", back.Alternatives[0].WhyNot, e.Alternatives[0].WhyNot)
	}
}

// TestValidateDraftAndValidateAgreeOnEveryRealEntry is the anti-drift check.
//
// ValidateDraft is Entry.Validate minus the id and created, and the way that
// claim rots is someone adding a rule to one and not the other. So every entry
// in this repository's ledger — which Validate accepts by definition — has those
// two fields removed and is put through ValidateDraft, which must also accept
// it.
func TestValidateDraftAndValidateAgreeOnEveryRealEntry(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(filepath.Join("..", "..", ".dira", "entries", "*.md"))
	if err != nil {
		t.Fatalf("globbing the ledger: %v", err)
	}
	if len(paths) < 20 {
		t.Fatalf("found %d entries in .dira/entries; this check is not measuring anything", len(paths))
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			e, err := ledger.Decode(data)
			if err != nil {
				t.Fatalf("decoding %s: %v", path, err)
			}

			e.ID = ""
			e.Created = ""
			if err := e.ValidateDraft(); err != nil {
				t.Errorf("Validate accepts this entry but ValidateDraft rejects it: %v", err)
			}
		})
	}
}

// TestValidateDraftStillCatchesWhatValidateCatches is the other half: a rule
// that accepts everything would pass the test above.
func TestValidateDraftStillCatchesWhatValidateCatches(t *testing.T) {
	t.Parallel()

	e, err := ledger.DecodeDraft([]byte(draftDocument))
	if err != nil {
		t.Fatalf("DecodeDraft: %v", err)
	}

	breaks := []struct {
		name   string
		break_ func(*ledger.Entry)
		want   string
	}{
		{"title too long", func(e *ledger.Entry) { e.Title = strings.Repeat("x", 121) }, "title"},
		{"a state from another kind", func(e *ledger.Entry) { e.State = ledger.StateOpen }, "state"},
		{"a tag that is not a tag", func(e *ledger.Entry) { e.Tags = []string{"Not A Tag"} }, "tags[0]"},
		{"an edge type outside the five", func(e *ledger.Entry) { e.Edges[0].Type = "causes" }, "edges[0]"},
		{"a source hook that does not exist", func(e *ledger.Entry) { e.Source.Hook = "OnTuesday" }, "source.hook"},
		{"created present but malformed", func(e *ledger.Entry) { e.Created = "last thursday" }, "created"},
	}

	for _, tc := range breaks {
		t.Run(tc.name, func(t *testing.T) {
			broken, err := ledger.DecodeDraft([]byte(draftDocument))
			if err != nil {
				t.Fatalf("DecodeDraft: %v", err)
			}
			tc.break_(broken)

			err = broken.ValidateDraft()
			if err == nil {
				t.Fatalf("ValidateDraft accepted an entry Validate would reject")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error does not name the field; want substring %q, got %v", tc.want, err)
			}
		})
	}

	// And the unbroken draft is still accepted, so the cases above are
	// failing for the reason they claim.
	if err := e.ValidateDraft(); err != nil {
		t.Errorf("ValidateDraft rejects the draft every case starts from: %v", err)
	}
}
