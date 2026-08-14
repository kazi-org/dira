package ui

import (
	"context"
	"html/template"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	texttemplate "text/template"

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

	root := t.TempDir()
	dira := filepath.Join(root, ".dira")
	if err := os.MkdirAll(filepath.Join(dira, "entries"), 0o755); err != nil {
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
	return srv
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

// equal is shared with distill_mockup_test.go's TestDistillMockupMatchesTheQueue.
