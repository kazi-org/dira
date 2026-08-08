package sniff

import (
	"strings"
	"testing"
)

// TestToolOutputNeverReachesTheMatcher is a precision rule and a privacy rule at
// once, so it is tested from both sides.
func TestToolOutputNeverReachesTheMatcher(t *testing.T) {
	t.Parallel()

	line := func(s string) string { return s + "\n" }
	transcript := line(`{"type":"assistant","message":{"role":"assistant","content":[`+
		`{"type":"text","text":"Running the gates."},`+
		`{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"env | grep KEY"}},`+
		`{"type":"thinking","thinking":"We could go with the daemon instead."}]}}`) +
		line(`{"type":"user","message":{"role":"user","content":[`+
			`{"type":"tool_result","tool_use_id":"toolu_1","content":"We're going with the daemon rather than the checkpoint file."}]}}`)

	segments, err := ParseTranscript(strings.NewReader(transcript))
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	if len(segments) != 1 {
		t.Fatalf("got %d segments, want 1 (the text block only): %+v", len(segments), segments)
	}
	if segments[0].Text != "Running the gates." {
		t.Errorf("segment = %q", segments[0].Text)
	}

	// The control: that same sentence, presented as prose, does match. Without
	// it this test would pass against a parser that returned nothing at all.
	if len(Sniff("We're going with the daemon rather than the checkpoint file.")) == 0 {
		t.Fatal("the control sentence does not match as prose, so dropping it from tool output proves nothing")
	}

	candidates, err := SniffTranscript(strings.NewReader(transcript), Whole)
	if err != nil {
		t.Fatalf("SniffTranscript: %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("captured %d candidate(s) from tool output or thinking: %+v", len(candidates), candidates)
	}
}

// TestFencedCodeIsStripped covers the same rule for code a model writes inline,
// which the block structure cannot separate.
func TestFencedCodeIsStripped(t *testing.T) {
	t.Parallel()

	text := "Here is the change:\n\n```go\n// We're going with a mutex rather than a channel.\nvar mu sync.Mutex\n```\n\nThat is all."
	if got := Sniff(text); len(got) != 0 {
		t.Errorf("captured %d candidate(s) from inside a fenced block: %+v", len(got), got)
	}
	if len(Sniff("We're going with a mutex rather than a channel.")) == 0 {
		t.Fatal("the control sentence does not match outside a fence, so stripping it proves nothing")
	}
}

// TestQuotedTextIsNotThisSessionsDecision covers markdown blockquotes: a pasted
// instruction, a message from another session, a citation.
func TestQuotedTextIsNotThisSessionsDecision(t *testing.T) {
	t.Parallel()

	quoted := "The other session said:\n\n> We're going with the daemon rather than the checkpoint file.\n\nI disagree."
	if got := Sniff(quoted); len(got) != 0 {
		t.Errorf("captured %d candidate(s) from a blockquote: %+v", len(got), got)
	}
}

// TestLastTurnIsTheDefaultScope pins the difference between the two hooks.
//
// Stop fires after every turn against a growing file, so its scope is what has
// been said since the human last spoke. PreCompact fires once, before the
// session's lossiest moment, and wants the whole thing.
func TestLastTurnIsTheDefaultScope(t *testing.T) {
	t.Parallel()

	whole, err := SniffTranscript(openTranscript(t, "pre-compact.jsonl"), Whole)
	if err != nil {
		t.Fatalf("SniffTranscript(Whole): %v", err)
	}
	turn, err := SniffTranscript(openTranscript(t, "pre-compact.jsonl"), LastTurn)
	if err != nil {
		t.Fatalf("SniffTranscript(LastTurn): %v", err)
	}

	if len(whole) == 0 {
		t.Fatal("the whole-transcript scope found nothing; the comparison below is vacuous")
	}
	if len(turn) >= len(whole) {
		t.Errorf("the last turn yielded %d candidates and the whole transcript %d; "+
			"the narrower scope must be narrower or Stop re-proposes the session on every turn",
			len(turn), len(whole))
	}
}

// TestPlainTextIsNotAnError keeps `dira sniff < notes.md` working, and keeps a
// truncated transcript from being a failure a hook has to survive.
func TestPlainTextIsNotAnError(t *testing.T) {
	t.Parallel()

	got, err := SniffTranscript(strings.NewReader("We're going with the derived cache rather than a status field."), Whole)
	if err != nil {
		t.Fatalf("plain text was an error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d candidates from one decision sentence", len(got))
	}

	// A half-written JSONL line: one good record, one truncated.
	partial := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"We're going with the derived cache rather than a status field."}]}}` + "\n" + `{"type":"assis`
	got, err = SniffTranscript(strings.NewReader(partial), Whole)
	if err != nil {
		t.Fatalf("a truncated transcript was an error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d candidates from a transcript with one readable record", len(got))
	}
}

// TestEmptyInputIsSilent covers the common case: a hook fires, nothing was said,
// and the right amount of output is none.
func TestEmptyInputIsSilent(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "   \n\n  ", "\n"} {
		got, err := SniffTranscript(strings.NewReader(in), Whole)
		if err != nil {
			t.Errorf("%q: %v", in, err)
		}
		if len(got) != 0 {
			t.Errorf("%q yielded %d candidates", in, len(got))
		}
	}
}

// TestTitlesFitTheSchema checks the two bounds a candidate has to respect before
// an Entry exists, over every recorded transcript.
func TestTitlesFitTheSchema(t *testing.T) {
	t.Parallel()

	seen := 0
	for _, name := range recordedTranscripts(t) {
		candidates, err := SniffTranscript(openTranscript(t, name), Whole)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, c := range candidates {
			seen++
			if n := len([]rune(c.Title)); n < 3 || n > maxTitle {
				t.Errorf("%s: title is %d characters: %q", name, n, c.Title)
			}
			if strings.HasSuffix(c.Title, ".") {
				t.Errorf("%s: title ends with a period, which the schema's description forbids: %q", name, c.Title)
			}
			if n := len([]rune(c.Excerpt)); n == 0 || n > maxExcerpt {
				t.Errorf("%s: excerpt is %d characters", name, n)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no candidate was produced by any transcript; every assertion above ran zero times")
	}
}

// TestLongSentencesAreBoundedOnAWordBoundary is the truncation rule, driven past
// the bound so it actually runs.
func TestLongSentencesAreBoundedOnAWordBoundary(t *testing.T) {
	t.Parallel()

	long := "We're going with the derived cache " + strings.Repeat("and a considered reason for it ", 30) + "rather than a status field."
	got := Sniff(long)
	if len(got) != 1 {
		t.Fatalf("got %d candidates", len(got))
	}
	c := got[0]
	if n := len([]rune(c.Title)); n > maxTitle {
		t.Errorf("title is %d characters, want at most %d", n, maxTitle)
	}
	if n := len([]rune(c.Excerpt)); n > maxExcerpt {
		t.Errorf("excerpt is %d characters, want at most %d", n, maxExcerpt)
	}
	if !strings.HasSuffix(c.Excerpt, "…") {
		t.Errorf("a truncated excerpt does not say so: %q", c.Excerpt)
	}
	if strings.Contains(c.Excerpt, "consid…") {
		t.Errorf("truncated mid-word: %q", c.Excerpt)
	}
}
