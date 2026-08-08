package sniff

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/brief"
	"github.com/kazi-org/dira/internal/ledger"
)

// goldenFile is the pinned handoff block. It is a constant so that a change of
// path has to be written down rather than typed into a call site.
const goldenFile = "testdata/handoff.golden"

// updateGolden rewrites the golden instead of comparing against it.
//
//	go test ./internal/sniff -run TestHandoffGolden -update
//
// It exists because the alternative is a human transcribing a 30-line block by
// hand and getting a trailing space wrong, and because a golden nobody can
// regenerate is a golden people delete. Every run without the flag is a
// comparison.
var updateGolden = flag.Bool("update", false, "rewrite testdata/handoff.golden from the renderer")

// TestHandoffGolden pins the block tier 1 hands to tier 2.
//
// The fixture is the real one. pre-compact.jsonl is the transcript the PreCompact
// hook is designed around and it yields exactly three candidates, so the "three
// staged ids" the acceptance asks for are three ids the sniffer actually
// produced rather than three strings this test made up.
//
// L-0001 rule 2 is served by TestGoldenComparisonCanFail below: a byte-for-byte
// comparison that has never been shown to reject anything is indistinguishable
// from `if false`.
func TestHandoffGolden(t *testing.T) {
	t.Parallel()

	items, staged := stagedHandoffItems(t, "pre-compact.jsonl")
	if len(staged) != 3 {
		t.Fatalf("pre-compact.jsonl staged %d entries, want the 3 the golden is pinned to", len(staged))
	}

	block := Handoff(items)

	// Non-vacuity first, and before the golden comparison, because every
	// assertion after this one is a "contains" or a "does not contain" and
	// all of them pass on the empty string. L-0014 is the shape.
	if strings.TrimSpace(block) == "" {
		t.Fatal("the rendered block is empty; every assertion below would pass vacuously")
	}
	ids := entryID.FindAllString(block, -1)
	if len(ids) == 0 {
		t.Fatal("the rendered block names no entry id, so it hands off nothing")
	}
	// Each id has to appear in the listing, not merely somewhere in the
	// block: the fixed prose cites dec-0003 and dec-0023 by name, so a bare
	// Contains would report a staged dec-0003 as named whether it was
	// listed or not.
	for _, e := range staged {
		if !strings.Contains(block, "  "+e.ID+"  ") {
			t.Errorf("the block does not list staged id %s:\n%s", e.ID, block)
		}
	}

	// The golden itself.
	if *updateGolden {
		if err := os.WriteFile(goldenFile, []byte(block), 0o644); err != nil {
			t.Fatalf("writing %s: %v", goldenFile, err)
		}
		t.Logf("OBSERVED  rewrote %s (%d bytes)", goldenFile, len(block))
		return
	}
	want, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("reading %s: %v (regenerate with -update)", goldenFile, err)
	}
	if len(want) == 0 {
		t.Fatalf("%s is empty; a comparison against it proves nothing", goldenFile)
	}
	if diff := diffBlocks(string(want), block); diff != "" {
		t.Errorf("the rendered block no longer matches %s:\n%s", goldenFile, diff)
	}

	// The three field names the semantic tier has to fill in, literally.
	// Named one at a time so a failure says which one went missing.
	for _, field := range []string{"alternatives", "why_not", "derives_from"} {
		if !strings.Contains(block, field) {
			t.Errorf("the block never names %q, so the reader is not told which field to supply", field)
		}
	}

	// The callback shape, using only flags cmd/dira/log.go defines. This
	// asserts the literal text; the mechanical check that every flag here
	// is registered is E2-L2-T5's, over the skill, and duplicating it as a
	// hand-listed slice would validate a declaration rather than a result.
	for _, part := range []string{
		"dira log --kind decision --state staged --tier semantic --hook PreCompact",
		"--title '", "--body '", "--alternative '", "--why-not '", "--excerpt '",
		"--edge derives_from=" + staged[0].ID,
	} {
		if !strings.Contains(block, part) {
			t.Errorf("the callback shape is missing %q", part)
		}
	}

	// cst-0001's counter, on cst-0001's terms.
	if got := brief.Tokens(block); got > handoffMaxTokens {
		t.Errorf("the block is %d tokens by brief.Tokens, want at most %d", got, handoffMaxTokens)
	} else {
		t.Logf("OBSERVED  the 3-candidate block is %d tokens by brief.Tokens (bound %d)", got, handoffMaxTokens)
	}

	assertNoSecretsOrURLs(t, "the golden block", block)

	// dec-0023: PreCompact stdout reaches the compaction summariser, which
	// is forbidden from calling tools. A block that only addressed a reader
	// able to run `dira log` would be addressing a reader it is not
	// guaranteed to have.
	for _, part := range []string{"If you can call tools", "If you cannot call tools", "dec-0023"} {
		if !strings.Contains(block, part) {
			t.Errorf("the block does not reflect dec-0023's finding: missing %q", part)
		}
	}
}

// TestGoldenComparisonCanFail is the other half of the golden, and the reason
// the golden is evidence.
//
// A byte-for-byte comparison is the easiest gate in this repository to write
// wrongly: read a file that does not exist, compare against the empty string,
// pass forever. So the comparison is run against a block with one byte changed
// and is required to reject it, naming the line that moved.
func TestGoldenComparisonCanFail(t *testing.T) {
	t.Parallel()

	want, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("reading %s: %v", goldenFile, err)
	}

	// Green: the untouched golden compares equal to itself.
	if diff := diffBlocks(string(want), string(want)); diff != "" {
		t.Fatalf("the comparison rejects an identical block, so its red result below would mean nothing:\n%s", diff)
	}

	// Red: exactly one byte differs.
	mutated := []byte(string(want))
	at := strings.Index(string(want), "derives_from")
	if at < 0 {
		t.Fatalf("%s does not contain the token the mutation targets", goldenFile)
	}
	mutated[at] = 'D'

	diff := diffBlocks(string(want), string(mutated))
	if diff == "" {
		t.Fatal("a one-byte change compared equal; the golden comparison is not comparing")
	}
	if !strings.Contains(diff, "Derives_from") {
		t.Errorf("the diff does not name the changed text, so a failure would not say what moved:\n%s", diff)
	}
	t.Logf("OBSERVED  a one-byte change is rejected and reported:\n%s", diff)
}

// TestHandoffRedactsCredentials is the privacy half.
//
// The order of the assertions is the point, and it is L-0014's lesson: no secret
// is asserted ABSENT from a block until something legitimate has been asserted
// PRESENT in it. A renderer that returned "" would satisfy every "does not
// contain" clause here and satisfy nothing else, and Handoff genuinely does
// return "" when every item is refused — so every absence check below runs
// against a block that still names the credential-free item.
//
// It renders from with-credentials.jsonl's raw sentences — the ones collect()
// already refuses — rather than from what the sniffer staged, because the point
// is to exercise this file's own redaction rather than to re-test redact.go's.
func TestHandoffRedactsCredentials(t *testing.T) {
	t.Parallel()

	raw := rawHandoffItems(t, "with-credentials.jsonl")
	if len(raw) != 3 {
		t.Fatalf("the fixture yielded %d raw items, want 3 (one clean, two carrying credentials)", len(raw))
	}
	clean, dirty := raw[0], raw[1:]

	// Red first: with the redaction skipped, the block names both
	// credential-carrying items — so their absence below is the redaction
	// doing something rather than the fixture being empty.
	leaked := renderHandoff(raw, handoffDetail, maxCandidates)
	for _, it := range dirty {
		if !strings.Contains(leaked, it.ID) {
			t.Fatalf("the unredacted renderer did not name %s, so its removal below is not evidence of redaction:\n%s", it.ID, leaked)
		}
	}
	t.Logf("OBSERVED  with redaction bypassed the block names %s and %s", dirty[0].ID, dirty[1].ID)

	// Red, second form, and the reason it is needed: this fixture's two
	// tokens sit late enough in their sentences that handoffTitle cuts them
	// off before redaction is ever reached. Truncation is not redaction —
	// it hides a secret only when the secret happens to be far enough
	// right — so the literal half of the proof uses an item whose token is
	// inside the drawn width. Without this, "the block contains no secret"
	// would be true of a build with no redaction at all.
	for _, secret := range fixtureSecrets {
		if strings.Contains(leaked, secret) {
			t.Logf("OBSERVED  the unredacted fixture render does carry %q", secret)
		}
	}
	near := HandoffItem{
		ID:      "dec-0109",
		Title:   leakToken + " goes in the env",
		Excerpt: "we're going with " + leakToken + " in the env rather than a flag",
	}
	if drawn := bound(near.Title, handoffTitle); !strings.Contains(drawn, leakToken) {
		t.Fatalf("the constructed title is truncated to %q, so it cannot demonstrate a leak", drawn)
	}

	// The clean item travels with it. Handoff over the leaking item alone
	// would return the empty string — every item dropped, no block — and
	// "the empty string does not contain the token" is the assertion this
	// whole test exists not to make.
	pair := []HandoffItem{clean, near}
	nearLeak := renderHandoff(pair, handoffDetail, maxCandidates)
	if !strings.Contains(nearLeak, leakToken) {
		t.Fatalf("the unredacted renderer did not emit the token, so the absence assertions prove nothing:\n%s", nearLeak)
	}
	nearSafe := Handoff(pair)
	if !strings.Contains(nearSafe, clean.ID) {
		t.Fatalf("the redacted block dropped the clean item too, so the token's absence proves nothing:\n%s", nearSafe)
	}
	if strings.Contains(nearSafe, leakToken) {
		t.Errorf("the redacted renderer emitted the token:\n%s", nearSafe)
	}
	if strings.Contains(nearSafe, near.ID) {
		t.Errorf("the leaking item %s was still named:\n%s", near.ID, nearSafe)
	}
	t.Logf("OBSERVED  a credential inside the drawn title width renders unredacted, and Handoff drops that item while keeping %s", clean.ID)

	// Green: the real path.
	block := Handoff(raw)
	if strings.TrimSpace(block) == "" {
		t.Fatal("the redacted block is empty; every absence assertion below would pass vacuously")
	}
	if !strings.Contains(block, clean.ID) {
		t.Fatalf("the credential-free item %s was dropped too, so this is not redaction, it is silence:\n%s", clean.ID, block)
	}
	if !strings.Contains(block, "I'm not going to put that key anywhere") {
		t.Fatalf("the legitimate title is absent, so the block carries no evidence at all:\n%s", block)
	}

	for _, it := range dirty {
		if strings.Contains(block, it.ID) {
			t.Errorf("%s carries a credential and was still named:\n%s", it.ID, block)
		}
	}
	for _, secret := range fixtureSecrets {
		if strings.Contains(block, secret) {
			t.Errorf("the redacted block contains %q — the rule is to drop the item, never to mask it:\n%s", secret, block)
		}
	}
	for _, mask := range []string{"REDACTED", "[redacted]", "***"} {
		if strings.Contains(block, mask) {
			t.Errorf("the block contains %q; redact.go drops whole and never masks", mask)
		}
	}

	assertNoSecretsOrURLs(t, "the redacted block", block)
	t.Logf("OBSERVED  the redacted block names %s only, and none of the fixture's secret literals", clean.ID)
}

// TestHandoffStaysBounded is the bound, on both sides.
//
// The degradation is what makes "at most 400 tokens" true of a block whose input
// is unbounded, so the test that matters is the one that removes it: rendering
// fifty worst-case items at full detail must blow the ceiling. If it does not,
// the ceiling is not being held by the degradation and the passing case says
// nothing about why it passed.
func TestHandoffStaysBounded(t *testing.T) {
	t.Parallel()

	// Red: the degradation removed. detail is set past the item count, so
	// every one of the fifty is drawn with its title.
	fifty := syntheticItems(50)
	undegraded := renderHandoff(fifty, len(fifty), len(fifty))
	if got := brief.Tokens(undegraded); got <= handoffMaxTokens {
		t.Fatalf("fifty items at full detail measured %d tokens, at or under the %d bound — "+
			"the fixture is too small to prove the degradation does anything", got, handoffMaxTokens)
	} else {
		t.Logf("OBSERVED  50 candidates with the degradation removed: %d tokens, over the %d bound", got, handoffMaxTokens)
	}

	// Green: the shipped path, over a range of counts rather than one.
	for _, n := range []int{1, 3, 5, 6, 12, 50, 500} {
		items := syntheticItems(n)
		block := Handoff(items)
		if strings.TrimSpace(block) == "" {
			t.Fatalf("n=%d rendered nothing", n)
		}
		if len(entryID.FindAllString(block, -1)) == 0 {
			t.Fatalf("n=%d rendered a block naming no id", n)
		}
		got := brief.Tokens(block)
		if got > handoffMaxTokens {
			t.Errorf("n=%d rendered %d tokens, want at most %d:\n%s", n, got, handoffMaxTokens, block)
			continue
		}
		t.Logf("OBSERVED  n=%-3d → %3d tokens, %d ids named", n, got, len(entryID.FindAllString(block, -1)))
	}

	// The degradation threshold is a property worth naming rather than
	// inferring: at handoffDetail items the titles are still there, and at
	// one more they are gone.
	drawn := bound(syntheticTitle, handoffTitle)
	withTitles := Handoff(syntheticItems(handoffDetail))
	if !strings.Contains(withTitles, drawn) {
		t.Errorf("%d items dropped the titles already; the detail listing is unreachable:\n%s", handoffDetail, withTitles)
	}
	withoutTitles := Handoff(syntheticItems(handoffDetail + 1))
	if strings.Contains(withoutTitles, drawn) {
		t.Errorf("%d items still carries titles; the block grows without bound:\n%s", handoffDetail+1, withoutTitles)
	}
}

// TestHandoffEmptyInput covers the case the hook hits most often: a session that
// settled nothing. No items means no block, so a caller can print the result
// unconditionally and a quiet session stays quiet.
func TestHandoffEmptyInput(t *testing.T) {
	t.Parallel()

	if got := Handoff(nil); got != "" {
		t.Errorf("Handoff(nil) = %q, want the empty string", got)
	}
	if got := Handoff([]HandoffItem{}); got != "" {
		t.Errorf("Handoff([]) = %q, want the empty string", got)
	}

	// An input that is entirely credentials is the same case, and it must
	// not render a header over an empty list.
	only := []HandoffItem{{ID: "dec-0001", Title: "we're going with it", Excerpt: "we're going with ghp_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789 rather than a password"}}
	if got := Handoff(only); got != "" {
		t.Errorf("an all-credential input rendered a block:\n%s", got)
	}
	// Control: the same item without the credential does render, so the
	// empty result above is the redaction and not a broken renderer.
	only[0].Excerpt = "we're going with a deploy token rather than a password"
	if got := Handoff(only); !strings.Contains(got, "dec-0001") {
		t.Errorf("the credential-free control rendered no block, so the empty result above proves nothing:\n%s", got)
	}
}

// TestStagedItemsCarriesTheExcerpt guards the field the credential check depends
// on. A StagedItems that dropped the excerpt would leave withoutCredentials
// looking at a title with its underscores already stripped, which is the one
// input the published-prefix patterns cannot see a token in.
func TestStagedItemsCarriesTheExcerpt(t *testing.T) {
	t.Parallel()

	_, staged := stagedHandoffItems(t, "pre-compact.jsonl")
	items := StagedItems(staged)
	if len(items) != len(staged) {
		t.Fatalf("StagedItems returned %d items for %d entries", len(items), len(staged))
	}
	for i, it := range items {
		if it.ID == "" || it.Title == "" {
			t.Errorf("item %d has an empty id or title: %+v", i, it)
		}
		if strings.TrimSpace(it.Excerpt) == "" {
			t.Errorf("item %d (%s) carries no excerpt, so the credential check runs on the normalised title only", i, it.ID)
		}
	}

	// The normalisation this defends against, demonstrated rather than
	// asserted in a comment.
	token := "ghp_" + "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789"
	sentence := "Let's go with the deploy token " + token + " in CI rather than a personal account."
	title := titleFor(sentence)
	if strings.Contains(title, token) {
		t.Fatalf("the title still carries the token verbatim; this test is guarding a normalisation that no longer happens")
	}
	if carriesCredential(title) {
		t.Fatalf("the normalised title is still refused, so checking the excerpt would be redundant — re-read this test")
	}
	if !carriesCredential(sentence) {
		t.Fatal("the verbatim sentence is not refused either, which would mean redact.go stopped seeing this shape")
	}
	t.Logf("OBSERVED  title %q is NOT refused; the verbatim sentence is — the excerpt is what the check must read", title)
}

// ---- helpers ---------------------------------------------------------------

// entryID matches a dira entry id as it appears in the block.
var entryID = regexp.MustCompile(`\bdec-\d{4}\b`)

// syntheticTitle is the title every synthetic item carries: long enough to be
// the worst case the bound has to survive, at the schema's own title limit.
var syntheticTitle = strings.Repeat("we are going with the longer option rather than the shorter one ", 2)[:maxTitle]

// syntheticItems builds n worst-case items with allocated-looking ids.
func syntheticItems(n int) []HandoffItem {
	out := make([]HandoffItem, n)
	for i := range out {
		out[i] = HandoffItem{
			ID:      fmt.Sprintf("dec-%04d", i+1),
			Title:   syntheticTitle,
			Excerpt: syntheticTitle + " because the first one is cheaper to reverse.",
		}
	}
	return out
}

// fixtureSecrets are the literals with-credentials.jsonl carries, in both the
// verbatim form and the underscore-stripped form a title would hold.
var fixtureSecrets = []string{
	"sk-ant-",
	"ghp_",
	"ghpAbCdEfGh",
	"ANTHROPIC_API_KEY",
	"ANTHROPICAPIKEY",
	"Zx9QwErTyUiOp",
}

// leakToken is a credential-shaped literal short enough to sit inside the width
// the block draws a title at. It is assembled at run time rather than written
// out whole, for the reason redact_test.go gives: a fixture's job is to look
// real enough to exercise the matcher, which is also real enough to trip a push
// scanner.
var leakToken = "sk-" + "ant-api03-" + "Zx9QwErTyUiOpAsDf"

// apiKeyEnv matches an API-key environment variable name in any of the shapes
// E2-L2-T7 denylists.
var apiKeyEnv = regexp.MustCompile(`[A-Z][A-Z0-9_]*_API_KEY`)

// urlish matches anything a reader could follow off this machine. cst-0004 says
// dira fetches nothing at runtime; a handoff that told a model to go and look
// something up would be dira asking somebody else to break it.
var urlish = regexp.MustCompile(`(?i)\bhttps?://|\bwww\.\w|\b\w+\.(?:com|net|org|io|ai)\b`)

// modelNames are the names a block must not carry. The handoff is read by
// whatever session is running; naming a model in it is either an instruction to
// switch or a claim about which one is reading, and dira knows neither.
var modelNames = []string{
	"claude", "anthropic", "openai", "gpt-", "opus", "sonnet", "haiku",
	"gemini", "llama", "mistral", "ollama", "cohere",
}

// assertNoSecretsOrURLs is the pattern half of the acceptance, factored so that
// every block this file renders is held to it rather than only the golden.
func assertNoSecretsOrURLs(t *testing.T, what, block string) {
	t.Helper()

	if strings.TrimSpace(block) == "" {
		t.Fatalf("%s is empty; the patterns below would all pass vacuously", what)
	}
	if m := apiKeyEnv.FindString(block); m != "" {
		t.Errorf("%s names an API key variable %q", what, m)
	}
	if m := urlish.FindString(block); m != "" {
		t.Errorf("%s carries a URL %q — cst-0004 says nothing here fetches anything", what, m)
	}
	lower := strings.ToLower(block)
	for _, name := range modelNames {
		if strings.Contains(lower, name) {
			t.Errorf("%s names the model %q", what, name)
		}
	}
	for _, re := range credentialPatterns {
		if m := re.FindString(block); m != "" {
			t.Errorf("%s carries something shaped like a credential: %q", what, m)
		}
	}
}

// stagedHandoffItems runs the real path: sniff a recorded transcript, stage the
// candidates into a temp ledger, and take the handoff items from the entries
// that were written. The ids are the ledger's own, which is what a golden over
// them is worth pinning.
func stagedHandoffItems(t *testing.T, fixture string) ([]HandoffItem, []*ledger.Entry) {
	t.Helper()

	candidates, err := SniffTranscript(openTranscript(t, fixture), Whole)
	if err != nil {
		t.Fatalf("SniffTranscript(%s): %v", fixture, err)
	}
	if len(candidates) == 0 {
		t.Fatalf("%s yielded no candidates; a handoff over it would be empty", fixture)
	}

	store, _ := tempLedger(t)
	result, err := Stage(context.Background(), store, StageOptions{Hook: ledger.HookPreCompact, Now: stamp(t)}, candidates)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if len(result.Staged) == 0 {
		t.Fatalf("%s staged nothing", fixture)
	}
	return StagedItems(result.Staged), result.Staged
}

// rawHandoffItems builds items from a transcript's matched sentences with the
// credential refusal skipped, so a test can watch this file's own redaction
// work rather than inherit collect()'s.
//
// It is deliberately the same three steps collect() takes, minus one, rather
// than a call into collect() with a flag: a production code path that can be
// asked to skip its redaction is a production code path somebody will ask.
func rawHandoffItems(t *testing.T, fixture string) []HandoffItem {
	t.Helper()

	segments, err := ParseTranscript(openTranscript(t, fixture))
	if err != nil {
		t.Fatalf("ParseTranscript(%s): %v", fixture, err)
	}

	var out []HandoffItem
	for _, s := range segments {
		for _, sentence := range sentences(strip(s.Text)) {
			if _, ok := match(sentence); !ok {
				continue
			}
			title := titleFor(sentence)
			if title == "" {
				continue
			}
			out = append(out, HandoffItem{
				// Numbered away from dec-0001..dec-0099 so that no
				// item's id collides with an entry the block's own
				// prose cites — dec-0003 and dec-0023 are both in
				// there, and a "was this id dropped?" assertion that
				// matched the citation would never fail.
				ID:      fmt.Sprintf("dec-%04d", len(out)+101),
				Title:   title,
				Excerpt: bound(sentence, maxExcerpt),
			})
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s produced no raw items", fixture)
	}
	return out
}

// diffBlocks reports the first line on which two blocks differ, with enough
// context to see it. A byte offset is a true answer to the wrong question: the
// reader wants to know what changed.
func diffBlocks(want, got string) string {
	if want == got {
		return ""
	}

	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		w, g := "", ""
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w == g {
			continue
		}
		return fmt.Sprintf("  line %d\n  golden: %q\n  render: %q", i+1, w, g)
	}
	return fmt.Sprintf("  lengths differ: golden %d bytes, render %d bytes", len(want), len(got))
}
