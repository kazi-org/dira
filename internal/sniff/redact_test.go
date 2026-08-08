package sniff

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

// TestCredentialsAreRefusedWholesale is the privacy half of the acceptance.
//
// with-credentials.jsonl carries two sentences that are otherwise perfect
// decision language — "we're going with", "let's go with" — each with a
// credential in it, plus one refusal that carries none. The refusal is the
// control: the rule is per sentence, not per file, so a session that mentions a
// key once does not stop being captured, and a test where nothing at all was
// captured would prove nothing about which rule did the refusing.
func TestCredentialsAreRefusedWholesale(t *testing.T) {
	t.Parallel()

	// Control one: with the credential stripped, the same sentence matches.
	clean := "We're going with the environment variable rather than a flag, because a flag lands in shell history."
	if len(Sniff(clean)) == 0 {
		t.Fatal("the credential-free control sentence does not match, so the refusal below proves nothing")
	}

	candidates, err := SniffTranscript(openTranscript(t, "with-credentials.jsonl"), Whole)
	if err != nil {
		t.Fatalf("SniffTranscript: %v", err)
	}

	// Control two: something was captured, so the assertions below are not
	// passing because the parser returned nothing.
	if len(candidates) == 0 {
		t.Fatal("nothing at all was captured from the fixture; the refusals below would pass vacuously")
	}

	secrets := []string{"sk-ant-", "ghp_", "ANTHROPIC_API_KEY=", "REDACTED", "[redacted]"}
	forbidden := []string{"environment variable", "deploy token"}
	for _, c := range candidates {
		if carriesCredential(c.Excerpt) || carriesCredential(c.Title) {
			t.Errorf("captured a sentence carrying a credential:\n  title:   %q\n  excerpt: %q", c.Title, c.Excerpt)
		}
		for _, s := range secrets {
			if strings.Contains(c.Excerpt, s) || strings.Contains(c.Title, s) {
				t.Errorf("captured text contains %q — the rule is to drop the candidate, never to mask it:\n  %q", s, c.Excerpt)
			}
		}
		for _, s := range forbidden {
			if strings.Contains(strings.ToLower(c.Excerpt), s) {
				t.Errorf("captured the decision sentence that carried the secret: %q", c.Excerpt)
			}
		}
	}

	store, dir := tempLedger(t)
	if _, err := Stage(context.Background(), store, StageOptions{Hook: ledger.HookStop, Now: stamp(t)}, candidates); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no file was written, so the on-disk check below reads nothing")
	}
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, marker := range []string{"sk-ant-", "ghp_", "REDACTED", "[redacted]"} {
			if strings.Contains(string(content), marker) {
				t.Errorf("%s contains %q", filepath.Base(path), marker)
			}
		}
	}
}

// TestCredentialShapes grades the refusal patterns one at a time, with a
// matching negative for each so that a pattern which fires on everything is
// caught here rather than by a lane whose captures silently stopped.
func TestCredentialShapes(t *testing.T) {
	t.Parallel()

	// Every value here is invented, but a credential fixture's whole job is to
	// look real enough to exercise the matcher — which means it also looks real
	// enough to trip a secret scanner. GitHub's push protection rejected this
	// file over the Slack row, and it was right to: a scanner that trusted a
	// filename would be no scanner at all. So the shapes that scanners flag are
	// assembled at run time and never appear as literals. Keep it that way; a
	// tidy-up that inlines them will block the next push.
	slack := "xoxb-" + "1234567890" + "-" + "ABCDEFGHIJKLMNOP"

	refuse := []struct{ name, text string }{
		{"anthropic key", "sk-ant-api03-Zx9QwErTyUiOpAsDfGhJkLzXcVbNm1234567890"},
		{"github token", "ghp_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789"},
		{"github pat", "github_pat_11ABCDEFG0abcdefghijklmnop"},
		{"slack token", slack},
		{"aws key id", "AKIAIOSFODNN7EXAMPLE"},
		{"google api key", "AIzaSyA1234567890abcdefghijklmnopqrstuvw"},
		{"private key", "-----BEGIN OPENSSH PRIVATE KEY-----"},
		{"assignment", `api_key: "8f3d9a1c4b7e2f60"`},
		{"bearer header", "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.abc"},
	}
	for _, tc := range refuse {
		t.Run(tc.name, func(t *testing.T) {
			if !carriesCredential(tc.text) {
				t.Errorf("not refused: %q", tc.text)
			}
		})
	}

	keep := []struct{ name, text string }{
		{"a word starting sk", "sketching the layout before writing any code"},
		{"password: yes", "password: yes"},
		{"a token count", "the brief is capped at 1500 tokens forever"},
		{"an entry id", "dec-0014 settled that lexical matching decides conflicts"},
		{"a path", "internal/ledger/local/local.go holds every path walk"},
		{"a sha", "the commit is fc48e11 and the tree is clean"},
	}
	for _, tc := range keep {
		t.Run(tc.name, func(t *testing.T) {
			if carriesCredential(tc.text) {
				t.Errorf("wrongly refused: %q", tc.text)
			}
		})
	}
}
