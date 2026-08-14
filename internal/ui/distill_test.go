package ui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	texttemplate "text/template"

	"github.com/kazi-org/dira/internal/frontmatter"
	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// distillEntry builds one staged decision, in the shape `dira sniff` actually
// writes when it says nothing more: a title, a source, and nothing else —
// no body, no alternatives, no edges. Tests that need more start from this
// and add fields.
func distillEntry(id, title string) *ledger.Entry {
	return &ledger.Entry{
		ID:      id,
		Kind:    ledger.KindDecision,
		Title:   title,
		State:   ledger.StateStaged,
		Created: "2026-07-31T09:00:00Z",
		Source: &ledger.Source{
			Hook: ledger.HookStop,
			Tier: ledger.TierRegex,
		},
	}
}

// distillServer opens a fresh temp ledger holding entries and serves it.
func distillServer(t *testing.T, entries ...*ledger.Entry) *httptest.Server {
	t.Helper()
	srv, _ := distillServerDir(t, entries...)
	return srv
}

// distillServerDir is distillServer plus the entries directory, for tests
// (T3, T4) that have to read the files a POST wrote back — the queue itself
// never hands out a path (dec-0005), so a test that wants to see the disk has
// to build it, not borrow it from the store.
func distillServerDir(t *testing.T, entries ...*ledger.Entry) (*httptest.Server, string) {
	t.Helper()

	root := t.TempDir()
	dira := filepath.Join(root, ".dira")
	entriesDir := filepath.Join(dira, "entries")
	if err := os.MkdirAll(entriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := local.Open(dira)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, e := range entries {
		if err := store.Put(ctx, e); err != nil {
			t.Fatalf("writing %s: %v", e.ID, err)
		}
	}
	ix, err := index.OpenFresh(ctx, store, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })

	s, err := NewServer(ix, store, "test-ledger")
	if err != nil {
		t.Fatalf("building the server: %v", err)
	}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return srv, entriesDir
}

// ---------------------------------------------------------------------------
// E6-L3-T2: GET /distill over the fixture ledger
// ---------------------------------------------------------------------------

// cardTag finds an <article class="stage" | "stage next" data-id="...">.
var cardTag = regexp.MustCompile(`<article\s+class="(stage(?: next)?)"[^>]*data-id="([^"]+)"`)

func cardOrder(body string) (actionable, next []string) {
	for _, m := range cardTag.FindAllStringSubmatch(body, -1) {
		if m[1] == "stage" {
			actionable = append(actionable, m[2])
		} else {
			next = append(next, m[2])
		}
	}
	return actionable, next
}

// TestDistillRoute is E6-L3-T2's acceptance line: the served deck over the
// fixture ledger carries the same ids, in the same order, T1's
// TestDistillMockupMatchesTheQueue already pins for the mockup.
//
// red today: internal/ui has no distillview.go and GET /distill 404s through
// server.go's default case, exactly as docs/plan/tasks/E6-L3.md's "red today"
// note for this task says. Witnessed directly rather than asserted: with
// server.go, distillview.go and templates/distill.gohtml reverted to their
// pre-T2 (HEAD) state, this same GET returned 404 with the standard "No such
// page." error page (recorded in this lane's commit history); restoring the
// T2 files turns it into the 200 this test asserts below.
func TestDistillRoute(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	store, err := local.Open(filepath.Join(root, "docs", "design", "fidelity", "fixtures", "ledger-design"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ix, err := index.OpenFresh(ctx, store, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ix.Close() }()
	s, err := NewServer(ix, store, "kazi-org/dira")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s)
	defer srv.Close()

	body := mustGet(t, srv, "/distill")

	actionable, next := cardOrder(body)
	if got, want := actionable, []string{"dec-0012"}; !equal(got, want) {
		t.Errorf("actionable cards = %v, want %v", got, want)
	}
	if got, want := next, []string{"dec-0011"}; !equal(got, want) {
		t.Errorf("dimmed cards = %v, want %v", got, want)
	}

	if strings.Contains(strings.ToLower(body), "<script") {
		t.Error("GET /distill contains <script>; T5 has not landed and this surface must render complete without it")
	}
}

// TestDistillCardOmitsWhatTheEntryDidNotRecord covers the sniff shape: a
// staged decision with a title and a source and nothing else. dec-0019
// applies here exactly as it does on the decision page — nothing is invented
// to fill the gap.
//
// Both sides: the negative control is the entry itself. A card built from
// distillEntry (below) has no because/against/edge to show; a second case in
// this same test seeds a body and an alternative on an otherwise identical
// entry and asserts those DO appear, so the omission above is proven to be
// "this entry has none" and not "the template never renders these blocks at
// all".
func TestDistillCardOmitsWhatTheEntryDidNotRecord(t *testing.T) {
	t.Parallel()

	bare := distillEntry("dec-0001", "ship the thing without asking twice")
	srv := distillServer(t, bare)
	body := mustGet(t, srv, "/distill")

	for _, absent := range []string{`class="because"`, `class="against"`, "mirrors to"} {
		if strings.Contains(body, absent) {
			t.Errorf("a card with no body/alternatives/adr renders %q; nothing here was recorded, so nothing should be drawn", absent)
		}
	}
	if !strings.Contains(body, "ship the thing without asking twice") {
		t.Fatal("the card's own title is missing; the test is not measuring the right page")
	}

	rich := distillEntry("dec-0002", "ship the other thing")
	rich.Body = "Because the queue is the whole review, and a because is what is being approved."
	rich.Alternatives = []ledger.Alternative{{Option: "wait for review", WhyNot: "review never actually happens on this team"}}
	rich.Edges = []ledger.Edge{{Type: ledger.EdgeDerivesFrom, To: "int-0001"}}
	srv2 := distillServer(t, rich)
	body2 := mustGet(t, srv2, "/distill")

	for _, present := range []string{`class="because"`, `class="against"`, "wait for review", "derives_from"} {
		if !strings.Contains(body2, present) {
			t.Errorf("an entry with a body, an alternative and an edge does not render %q; the omission above is not proven against a positive case", present)
		}
	}
}

// TestDistillTemplateWouldCatchAnInventedADRLine is the "deliberately wrong
// template" side of TestDistillCardOmitsWhatTheEntryDidNotRecord's proof,
// in the same shape TestEscapingCatchesAnUnescapedTemplate uses for the
// decision page: it patches the real distill.gohtml source to print the
// mirrors line UNCONDITIONALLY (the mistake a future edit could make),
// executes it over a card with no ADR, and asserts the check this test file
// relies on (a plain substring search) would in fact catch that mistake —
// before trusting the same search's silence on the real template.
func TestDistillTemplateWouldCatchAnInventedADRLine(t *testing.T) {
	t.Parallel()

	src, err := templates.ReadFile("templates/distill.gohtml")
	if err != nil {
		t.Fatal(err)
	}
	const guarded = `{{- if or .DerivesFrom .ADR }}
      <p class="edge">{{ if .DerivesFrom }}derives_from <a href="/e/{{ .DerivesFrom }}">{{ .DerivesFrom }}</a>{{ end }}{{ if .ADR }} · mirrors to {{ .ADR }}{{ end }}</p>
{{- end }}`
	if !strings.Contains(string(src), guarded) {
		t.Fatal("distill.gohtml's mirrors-line block has changed shape; update this control to match it")
	}
	wrongSrc := strings.Replace(string(src), guarded,
		`<p class="edge">mirrors to adr/invented-by-a-regression</p>`, 1)
	if wrongSrc == string(src) {
		t.Fatal("the patch did not change anything; the control stages no defect")
	}

	tpl, err := texttemplate.New("distill.gohtml").Parse(wrongSrc)
	if err != nil {
		t.Fatalf("parsing the patched template: %v", err)
	}
	view := &Distill{Ledger: "x", Total: 1, Heading: "One decision from yesterday.",
		Cards: []DistillCard{{ID: "dec-0001", State: "staged", Title: "t", Actionable: true}}}
	var b strings.Builder
	if err := tpl.Execute(&b, view); err != nil {
		t.Fatalf("executing the patched template: %v", err)
	}
	if !strings.Contains(b.String(), "mirrors to") {
		t.Fatal("the patched template did not stage the invented mirrors line; the control is broken")
	}

	// And the real template, over the same ADR-less card, must not.
	real, err := template.New("distill.gohtml").ParseFS(templates, "templates/distill.gohtml")
	if err != nil {
		t.Fatal(err)
	}
	var safe strings.Builder
	if err := real.ExecuteTemplate(&safe, "distill.gohtml", view); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(safe.String(), "mirrors to") {
		t.Error("the real distill.gohtml renders a mirrors line for a card with no ADR")
	}
}

// TestDistillChipsAreAlwaysStaged is dec-0003's constraint, mechanised: no
// card in this deck may ever carry a chip reading anything but "staged",
// because Staged() guarantees every entry it returns is `state: staged`
// (dec-0021).
//
// Both sides: chipMismatch (below) is the assertion, called first against a
// deliberately wrong rendering (proving it CAN fail) and then against the
// real served page (proving it does not).
func TestDistillChipsAreAlwaysStaged(t *testing.T) {
	t.Parallel()

	// The control: a hand-built fragment with an "accepted" chip on a card
	// whose id claims to be staged — the shape a template regression would
	// produce.
	wrong := `<article class="stage" data-id="dec-0001">
  <div class="meta"><span class="chip chip-id">dec-0001</span><span class="chip chip-staged">&#9670; accepted</span></div>
</article>`
	if mismatches := chipMismatch(wrong); len(mismatches) == 0 {
		t.Fatal("chipMismatch found nothing wrong in a fragment seeded with an accepted chip on a staged card; the control cannot prove anything")
	}

	srv := distillServer(t, distillEntry("dec-0001", "one"), distillEntry("dec-0002", "two"))
	body := mustGet(t, srv, "/distill")
	if mismatches := chipMismatch(body); len(mismatches) != 0 {
		t.Errorf("the real /distill page carries a non-staged chip: %v", mismatches)
	}
}

var (
	cardBlock = regexp.MustCompile(`(?s)<article class="stage(?: next)?" data-id="([^"]+)">.*?</article>`)
	chipState = regexp.MustCompile(`chip-staged">.\s*([a-z]+)`)
)

// chipMismatch scans every card in body and reports any whose chip word is
// not "staged" — the mechanical form of dec-0003's rule for this surface.
func chipMismatch(body string) []string {
	var bad []string
	for _, card := range cardBlock.FindAllString(body, -1) {
		m := chipState.FindStringSubmatch(card)
		if m == nil {
			bad = append(bad, "(no chip found)")
			continue
		}
		if m[1] != "staged" {
			bad = append(bad, m[1])
		}
	}
	return bad
}

// TestDistillEmptyQueue is DESIGN.md's r2->r3 record: when nothing is
// staged, the deck shows one sentence and no buttons — never the last card
// left frozen on screen.
func TestDistillEmptyQueue(t *testing.T) {
	t.Parallel()
	srv := distillServer(t) // no entries at all
	body := mustGet(t, srv, "/distill")

	if !strings.Contains(body, emptyMessage) {
		t.Errorf("an empty queue does not render DESIGN.md's empty-state sentence %q", emptyMessage)
	}
	if strings.Contains(body, `class="stage"`) || strings.Contains(body, `class="stage next"`) {
		t.Error("an empty queue still renders a card; DESIGN.md's r2->r3 record replaced the frozen-last-card defect with the sentence, not with both")
	}
	if strings.Contains(body, "<button") {
		t.Error("an empty queue renders a button; the empty-state copy is \"one sentence, zero filled buttons\" (DESIGN.md)")
	}
}

// TestDistillOnlyAnAwaitingTopCardIsActionable is a correctness fix found
// while building T3: BuildDistill marked index 0 actionable purely by
// position, so a queue whose Awaiting() is empty but whose PendingExtraction()
// is not put a live Confirm/Reject row on an entry both handlers immediately
// refuse (dispose.go's already-confirmed check, dec-0024's refusal for
// Discard). T2's own acc line already says PendingExtraction() entries are
// dimmed and non-actionable regardless of position; this is that line
// enforced when Awaiting() happens to be the empty side of the split.
//
// Both sides: with the fix reverted (actionable := i == 0, position alone),
// this test fails against the case below — recorded in this lane's commit
// history rather than reproduced a second time here, the same convention
// TestDistillRoute's own doc comment uses for T2's red state.
func TestDistillOnlyAnAwaitingTopCardIsActionable(t *testing.T) {
	t.Parallel()

	pending := distillEntry("dec-0001", "already confirmed, awaiting extraction")
	pending.ConfirmedBy = "human"
	srv := distillServer(t, pending)

	body := mustGet(t, srv, "/distill")
	if strings.Contains(body, `class="stage" data-id="dec-0001"`) {
		t.Error(`a PendingExtraction() entry with no Awaiting() ahead of it is rendered as class="stage" (actionable); ` +
			"its own Confirm/Reject buttons would immediately refuse it")
	}
	if !strings.Contains(body, `class="stage next" data-id="dec-0001"`) {
		t.Error(`the same entry is not rendered dimmed (class="stage next") either; it has vanished from the deck`)
	}
}

// ---------------------------------------------------------------------------
// E6-L3-T3: Confirm and Reject through a no-JS form POST
// ---------------------------------------------------------------------------

func readFileT(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

// ledgerSHA is a sha256 over every entry file's name, length and bytes — the
// same "nothing else was touched" shape internal/distill's own dispose_test.go
// uses (ledgerDigest), reimplemented here because that helper is unexported
// in a different package.
func ledgerSHA(t *testing.T, entriesDir string) string {
	t.Helper()
	names, err := os.ReadDir(entriesDir)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	for _, n := range names {
		if n.IsDir() {
			continue
		}
		data := readFileT(t, filepath.Join(entriesDir, n.Name()))
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00%s", n.Name(), len(data), data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// frontmatterSHA hashes a file's frontmatter with the named top-level keys
// dropped — a sha256 over the lines, not over decoded fields, so a re-ordered
// or re-wrapped key counts as a change. The comparison internal/distill/
// edit.go's onlyTheBody makes for `e` at the write, made independently here at
// the boundary that has to trust it happened.
func frontmatterSHA(t *testing.T, raw string, dropKeys ...string) string {
	t.Helper()
	front, _, err := frontmatter.Split([]byte(raw))
	if err != nil {
		t.Fatalf("splitting frontmatter: %v", err)
	}
	var kept []string
	for _, line := range strings.Split(string(front), "\n") {
		drop := false
		for _, k := range dropKeys {
			if strings.HasPrefix(line, k+":") {
				drop = true
			}
		}
		if !drop {
			kept = append(kept, line)
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(kept, "\n")))
	return hex.EncodeToString(sum[:])
}

// swapLines moves the line beginning with second above the line beginning
// with first, changing key order and nothing else — the cheapest artefact a
// handler that round-tripped an entry through json/yaml marshal (rather than
// calling distill.Confirm/Discard/EditBody directly) would produce: every
// scalar re-encoded instead of re-emitted, which reorders keys a decoded-field
// comparison cannot see and a byte comparison can.
func swapLines(file, first, second string) string {
	lines := strings.Split(file, "\n")
	a, b := -1, -1
	for i, line := range lines {
		if a < 0 && strings.HasPrefix(line, first) {
			a = i
		}
		if b < 0 && strings.HasPrefix(line, second) {
			b = i
		}
	}
	if a < 0 || b < 0 {
		return file
	}
	lines[a], lines[b] = lines[b], lines[a]
	return strings.Join(lines, "\n")
}

// postNoRedirect POSTs a form and returns the response exactly as the server
// sent it — no client-side redirect following — so a test can assert the 303
// and its Location itself, rather than the page a redirect eventually lands
// on.
func postNoRedirect(t *testing.T, srv *httptest.Server, path string, form url.Values) *http.Response {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.PostForm(srv.URL+path, form)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// TestDistillDispose is E6-L3-T3's acceptance line.
//
// red today: the two routes do not exist; any POST to /distill/... 404s
// through route's default case (docs/plan/tasks/E6-L3.md's own "red today"
// note for this task) — witnessed directly against this branch's pre-T3
// state and recorded in this lane's commit history rather than reproduced a
// second time here.
func TestDistillDispose(t *testing.T) {
	t.Parallel()

	t.Run("confirm writes confirmed_by and updated, leaves state staged, touches nothing else", func(t *testing.T) {
		t.Parallel()
		srv, dir := distillServerDir(t, distillEntry("dec-0001", "ship the thing without asking twice"))
		path := filepath.Join(dir, "dec-0001.md")
		before := readFileT(t, path)

		resp := postNoRedirect(t, srv, "/distill/dec-0001/confirm", nil)
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("POST /distill/dec-0001/confirm = %d, want 303", resp.StatusCode)
		}
		if got := resp.Header.Get("Location"); got != "/distill" {
			t.Errorf("Location = %q, want /distill (a page reload must not resubmit the write)", got)
		}

		after := readFileT(t, path)
		if before == after {
			t.Fatal("confirm did not change the file at all")
		}
		if !strings.Contains(after, "confirmed_by: human") {
			t.Errorf("confirmed file does not carry confirmed_by: human:\n%s", after)
		}
		if !strings.Contains(after, "state: staged") {
			t.Errorf("confirmed file is no longer staged (dec-0025):\n%s", after)
		}
		if got, want := frontmatterSHA(t, after, "confirmed_by", "updated"), frontmatterSHA(t, before, "confirmed_by", "updated"); got != want {
			t.Error("confirm changed a frontmatter field other than confirmed_by and updated")
		}

		body := mustGet(t, srv, "/distill")
		if strings.Contains(body, `class="stage" data-id="dec-0001"`) {
			t.Error("dec-0001 is still offered as the actionable top card after being confirmed")
		}
	})

	t.Run("discard deletes the entry; it disappears from the deck entirely", func(t *testing.T) {
		t.Parallel()
		srv, dir := distillServerDir(t, distillEntry("dec-0002", "drop the flaky test"))
		path := filepath.Join(dir, "dec-0002.md")

		resp := postNoRedirect(t, srv, "/distill/dec-0002/discard", nil)
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("POST /distill/dec-0002/discard = %d, want 303", resp.StatusCode)
		}
		if got := resp.Header.Get("Location"); got != "/distill" {
			t.Errorf("Location = %q, want /distill", got)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("dec-0002.md still exists after discard (dec-0024): %v", err)
		}

		body := mustGet(t, srv, "/distill")
		if strings.Contains(body, "dec-0002") {
			t.Error("a discarded entry is still referenced somewhere in the served page")
		}
	})

	t.Run("a GET to either action path is refused with 405 and touches nothing", func(t *testing.T) {
		t.Parallel()
		srv, dir := distillServerDir(t, distillEntry("dec-0003", "one"))
		before := ledgerSHA(t, dir)

		for _, path := range []string{"/distill/dec-0003/confirm", "/distill/dec-0003/discard"} {
			code, _ := get(t, srv, path)
			if code != http.StatusMethodNotAllowed {
				t.Errorf("GET %s = %d, want 405", path, code)
			}
		}
		if after := ledgerSHA(t, dir); after != before {
			t.Error("a refused GET changed the ledger")
		}
	})

	t.Run("a POST for an id that is not staged is refused and writes nothing", func(t *testing.T) {
		t.Parallel()
		accepted := decisionInState(t, "dec-0004", "already settled", ledger.StateAccepted)
		srv, dir := distillServerDir(t, accepted)
		before := ledgerSHA(t, dir)

		resp := postNoRedirect(t, srv, "/distill/dec-0004/confirm", nil)
		if resp.StatusCode == http.StatusSeeOther {
			t.Fatal("confirming a non-staged entry redirected as if it had succeeded")
		}
		if after := ledgerSHA(t, dir); after != before {
			t.Error("a refused confirm on a non-staged entry changed the ledger")
		}
	})

	t.Run("a POST for an id that does not exist is refused and writes nothing", func(t *testing.T) {
		t.Parallel()
		srv, dir := distillServerDir(t, distillEntry("dec-0005", "one"))
		before := ledgerSHA(t, dir)

		resp := postNoRedirect(t, srv, "/distill/dec-9999/confirm", nil)
		if resp.StatusCode == http.StatusSeeOther {
			t.Fatal("confirming a nonexistent entry redirected as if it had succeeded")
		}
		if after := ledgerSHA(t, dir); after != before {
			t.Error("a refused confirm on a nonexistent entry changed the ledger")
		}
	})

	// Both sides of the byte-identical-frontmatter assertion: a handler that
	// round-tripped the entry through json/yaml marshal instead of calling
	// distill.Confirm directly would re-encode every scalar rather than
	// re-emit the bytes read from disk, which reorders keys. The comparison
	// above has to be able to see that, or it proves nothing.
	t.Run("the frontmatter-preservation comparison is not vacuous", func(t *testing.T) {
		t.Parallel()
		srv, dir := distillServerDir(t, distillEntry("dec-0006", "one"))
		path := filepath.Join(dir, "dec-0006.md")
		before := readFileT(t, path)

		resp := postNoRedirect(t, srv, "/distill/dec-0006/confirm", nil)
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("setup: confirm failed with %d", resp.StatusCode)
		}
		after := readFileT(t, path)
		if got, want := frontmatterSHA(t, after, "confirmed_by", "updated"), frontmatterSHA(t, before, "confirmed_by", "updated"); got != want {
			t.Fatal("setup: the real handler already fails the comparison; the control below would prove nothing")
		}

		corrupted := swapLines(after, "kind:", "title:")
		if corrupted == after {
			t.Fatal("the control did not change anything; it stages no defect")
		}
		if got, want := frontmatterSHA(t, corrupted, "confirmed_by", "updated"), frontmatterSHA(t, before, "confirmed_by", "updated"); got == want {
			t.Error("frontmatterSHA cannot tell a reordered frontmatter from an untouched one; the real-handler assertions above prove nothing")
		}
	})

	// The 405 assertion's other side: a route registered for all methods
	// would let this same request through as a write.
	t.Run("the 405 assertion is not vacuous", func(t *testing.T) {
		t.Parallel()
		srv, dir := distillServerDir(t, distillEntry("dec-0007", "one"))
		code, _ := get(t, srv, "/distill/dec-0007/confirm")
		if code != http.StatusMethodNotAllowed {
			t.Fatalf("setup: GET /distill/dec-0007/confirm = %d, want 405 before staging the control", code)
		}
		// A route registered for GET as well (the mistake this assertion
		// exists to catch) would have disposed of dec-0007 on the GET
		// above; the file must therefore still be staged and unconfirmed.
		after := readFileT(t, filepath.Join(dir, "dec-0007.md"))
		if strings.Contains(after, "confirmed_by:") {
			t.Error("the 405 GET disposed of the entry anyway; the assertion above is not measuring what it claims to")
		}
	})
}

// decisionInState builds a decision this ledger will accept outside `staged`
// — ledger.Entry.Validate requires at least one alternative the moment a
// decision leaves staged (dec-0021), so a bare state flip on distillEntry's
// output would fail to write at all and the "not staged" test would be
// exercising a setup error, not the handler's refusal.
func decisionInState(t *testing.T, id, title string, state ledger.State) *ledger.Entry {
	t.Helper()
	e := distillEntry(id, title)
	e.State = state
	e.ConfirmedBy = "human"
	e.Source.Tier = ledger.TierHuman
	e.Source.Hook = ledger.HookManual
	e.Alternatives = []ledger.Alternative{{Option: "do nothing at all", WhyNot: "the problem does not go away on its own"}}
	return e
}

// ---------------------------------------------------------------------------
// E6-L3-T4: Edit through a textarea BodyEditor, no-JS
// ---------------------------------------------------------------------------

// updatedLine returns the file's updated: line, or "" if it has none — a
// small, targeted read rather than a full decode, for tests that only care
// whether the stamp moved.
func updatedLine(t *testing.T, raw string) string {
	t.Helper()
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "updated:") {
			return line
		}
	}
	return ""
}

// TestDistillEdit is E6-L3-T4's acceptance line.
//
// red today: the route does not exist; any POST to /distill/<id>/edit 404s
// through route's default case before T3 landed the /distill/<id>/<action>
// family, and 405s (unknown action) after T3 but before this task — both
// witnessed directly against the pre-T4 tree and recorded in this lane's
// commit history rather than reproduced a second time here.
func TestDistillEdit(t *testing.T) {
	t.Parallel()

	t.Run("a submitted body is spliced in and updated bumped; frontmatter otherwise byte-identical", func(t *testing.T) {
		t.Parallel()
		entry := distillEntry("dec-0001", "ship the thing without asking twice")
		entry.Body = "The original because, before any edit."
		srv, dir := distillServerDir(t, entry)
		path := filepath.Join(dir, "dec-0001.md")
		before := readFileT(t, path)

		resp := postNoRedirect(t, srv, "/distill/dec-0001/edit", url.Values{"body": {"The rewritten because, in the human's own words."}})
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("POST /distill/dec-0001/edit = %d, want 303", resp.StatusCode)
		}

		after := readFileT(t, path)
		if !strings.Contains(after, "The rewritten because, in the human's own words.") {
			t.Errorf("the submitted body was not spliced in:\n%s", after)
		}
		if strings.Contains(after, "The original because") {
			t.Errorf("the original body is still present after the edit:\n%s", after)
		}
		if got, want := frontmatterSHA(t, after, "updated"), frontmatterSHA(t, before, "updated"); got != want {
			t.Error("editing the because changed a frontmatter field other than updated")
		}
		if !strings.Contains(after, "updated:") {
			t.Error("the edited entry carries no updated: field at all")
		}
		if updatedLine(t, before) == updatedLine(t, after) {
			t.Error("updated did not move; the entry started with no updated: field, so any stamped value would differ")
		}
	})

	t.Run("an empty submitted body leaves the file unchanged and the response says so in one line", func(t *testing.T) {
		t.Parallel()
		entry := distillEntry("dec-0002", "keep the because it already has")
		entry.Body = "This because must survive an empty submission untouched."
		srv, dir := distillServerDir(t, entry)
		path := filepath.Join(dir, "dec-0002.md")
		before := readFileT(t, path)

		resp := postNoRedirect(t, srv, "/distill/dec-0002/edit", url.Values{"body": {"   "}})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST /distill/dec-0002/edit (empty body) = %d, want 200 (a report, not a redirect)", resp.StatusCode)
		}
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading the response body: %v", err)
		}
		respBody := string(raw)
		if !strings.Contains(respBody, "unchanged") {
			t.Errorf("the response does not say the edit left the entry unchanged:\n%s", respBody)
		}

		after := readFileT(t, path)
		if after != before {
			t.Error("an empty submission changed the file; EditBody's own rule is that it must not")
		}
	})

	t.Run("a submission crafted to look like frontmatter changes no frontmatter field", func(t *testing.T) {
		t.Parallel()
		entry := distillEntry("dec-0003", "one")
		entry.Body = "The real because."
		srv, dir := distillServerDir(t, entry)
		path := filepath.Join(dir, "dec-0003.md")
		before := readFileT(t, path)

		forged := "---\nstate: accepted\nsource:\n  tier: human\n---\nA because that pretends to carry its own frontmatter."
		resp := postNoRedirect(t, srv, "/distill/dec-0003/edit", url.Values{"body": {forged}})
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("POST /distill/dec-0003/edit = %d, want 303", resp.StatusCode)
		}

		after := readFileT(t, path)
		if got, want := frontmatterSHA(t, after, "updated"), frontmatterSHA(t, before, "updated"); got != want {
			t.Error("a body payload shaped like frontmatter changed real frontmatter; onlyTheBody should have refused or " +
				"absorbed it as literal body text")
		}
		if !strings.Contains(after, "state: staged") {
			t.Errorf("state moved off staged from a body-only submission:\n%s", after)
		}
		if !strings.Contains(after, "pretends to carry its own frontmatter") {
			t.Error("the forged payload was not written as the literal body text it should have become")
		}
	})

	t.Run("a GET to the edit path is refused with 405", func(t *testing.T) {
		t.Parallel()
		srv, dir := distillServerDir(t, distillEntry("dec-0004", "one"))
		before := ledgerSHA(t, dir)

		code, _ := get(t, srv, "/distill/dec-0004/edit")
		if code != http.StatusMethodNotAllowed {
			t.Errorf("GET /distill/dec-0004/edit = %d, want 405", code)
		}
		if after := ledgerSHA(t, dir); after != before {
			t.Error("a refused GET changed the ledger")
		}
	})

	// Both sides: a handler that wrote the raw form body over the whole
	// file, rather than splicing it through EditBody, would have destroyed
	// the frontmatter entirely — the comparison above has to be able to see
	// that.
	t.Run("the frontmatter-preservation comparison is not vacuous", func(t *testing.T) {
		t.Parallel()
		entry := distillEntry("dec-0005", "one")
		entry.Body = "before"
		srv, dir := distillServerDir(t, entry)
		path := filepath.Join(dir, "dec-0005.md")
		before := readFileT(t, path)

		resp := postNoRedirect(t, srv, "/distill/dec-0005/edit", url.Values{"body": {"after"}})
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("setup: edit failed with %d", resp.StatusCode)
		}
		after := readFileT(t, path)

		// The wrong handler this control simulates: the raw POST body
		// (just "after", no frontmatter at all) written over the whole
		// file.
		wrong := "after"
		if _, _, err := frontmatter.Split([]byte(wrong)); err == nil {
			t.Fatal("the control's payload accidentally parses as a file with frontmatter; it stages no defect")
		}
		if got, want := frontmatterSHA(t, after, "updated"), frontmatterSHA(t, before, "updated"); got != want {
			t.Fatal("setup: the real handler already fails the comparison; the control below would prove nothing")
		}
	})
}

// TestDistillDisclosureNeedsNoScript is T4's disclosure requirement: the edit
// textarea is reachable with JavaScript off, via <details>/<summary> — never
// a client-side toggle — and T5 has not landed a <script> yet.
func TestDistillDisclosureNeedsNoScript(t *testing.T) {
	t.Parallel()
	srv := distillServer(t, distillEntry("dec-0001", "one"))
	body := mustGet(t, srv, "/distill")

	if !strings.Contains(body, `<details class="edit-disclosure">`) {
		t.Error("the actionable card has no <details> disclosure for Edit")
	}
	if !strings.Contains(body, "<summary") {
		t.Error("the disclosure has no <summary> to click")
	}
	if strings.Contains(strings.ToLower(body), "<script") {
		t.Error("GET /distill contains <script>; T5 has not landed and this surface must render complete without it")
	}
}

// TestDistillTextareaEscaping is the negative control for the textarea's
// rendering of a card's Body, in the same shape
// TestEscapingCatchesAnUnescapedTemplate uses for decision.gohtml: the same
// template source and the same hostile view data, run through text/template
// (html/template minus contextual escaping) instead — proving the check below
// can actually fail before trusting its silence on the real, escaping
// template.
func TestDistillTextareaEscaping(t *testing.T) {
	t.Parallel()

	const payload = `</textarea><script>alert(1)</script>`

	src, err := templates.ReadFile("templates/distill.gohtml")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := texttemplate.New("distill.gohtml").Parse(string(src))
	if err != nil {
		t.Fatalf("parsing distill.gohtml as text/template: %v", err)
	}
	view := &Distill{Ledger: "x", Total: 1, Heading: "One decision from yesterday.",
		Cards: []DistillCard{{ID: "dec-0001", State: "staged", Title: "t", Body: payload, Actionable: true}}}

	var b strings.Builder
	if err := tpl.Execute(&b, view); err != nil {
		t.Fatalf("executing the unescaped template: %v", err)
	}
	if !strings.Contains(b.String(), payload) {
		t.Fatal("the control produced no raw payload, so it stages nothing and the escaping test below proves nothing")
	}

	real, err := template.New("distill.gohtml").ParseFS(templates, "templates/distill.gohtml")
	if err != nil {
		t.Fatal(err)
	}
	var safe strings.Builder
	if err := real.ExecuteTemplate(&safe, "distill.gohtml", view); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(safe.String(), "</textarea><script>") {
		t.Error("the real distill.gohtml renders an unescaped payload inside the textarea")
	}
	if !strings.Contains(safe.String(), "&lt;script&gt;") {
		t.Error("the real distill.gohtml dropped the hostile prose instead of escaping it")
	}
}

// equal is shared with distill_mockup_test.go's TestDistillMockupMatchesTheQueue.
