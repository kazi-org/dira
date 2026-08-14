package ui

import (
	"context"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	texttemplate "text/template"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// repoRoot walks up from this test file to the module root. Tests must not
// depend on the directory `go test` happened to choose, and the alternative —
// a relative path from the package — breaks the moment the package moves.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate this test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// serve opens a ledger and returns a live test server over it.
func serve(t *testing.T, diraDir, name string) *httptest.Server {
	t.Helper()
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("opening %s: %v", diraDir, err)
	}
	ix, err := index.OpenFresh(context.Background(), store, t.TempDir())
	if err != nil {
		t.Fatalf("indexing %s: %v", diraDir, err)
	}
	t.Cleanup(func() { _ = ix.Close() })

	s, err := NewServer(ix, store, name)
	if err != nil {
		t.Fatalf("building the server: %v", err)
	}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return srv
}

// realLedger serves this repository's own .dira/. Several acceptance clauses are
// about the real ledger rather than about the design fixture, and a surface that
// only works on its fixture is a surface that works on nothing.
func realLedger(t *testing.T) *httptest.Server {
	t.Helper()
	return serve(t, filepath.Join(repoRoot(t), ".dira"), "kazi-org/dira")
}

// designFixture serves the 18-entry fixture ledger the mockups are drawn from.
func designFixture(t *testing.T) *httptest.Server {
	t.Helper()
	return serve(t, filepath.Join(repoRoot(t), "docs", "design", "fidelity", "fixtures", "ledger-design"), "kazi-org/dira")
}

func get(t *testing.T, srv *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return resp.StatusCode, string(body)
}

func mustGet(t *testing.T, srv *httptest.Server, path string) string {
	t.Helper()
	code, body := get(t, srv, path)
	if code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200\n%s", path, code, body)
	}
	return body
}

// ---------------------------------------------------------------------------
// The routes exist, over the same query path the CLI uses
// ---------------------------------------------------------------------------

func TestBothRoutesServeTheRealLedger(t *testing.T) {
	t.Parallel()
	srv := realLedger(t)

	for _, path := range []string{"/", "/e/dec-0001", "/tokens.css", "/decision.css", "/index.css"} {
		if code, body := get(t, srv, path); code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200\n%s", path, code, body)
		}
	}
}

// TestEveryEntryAppearsOnTheIndex is the acceptance clause about the real
// ledger: every id under .dira/entries/ is on the page, and no id is quietly
// dropped because it did not fit a group.
func TestEveryEntryAppearsOnTheIndex(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	body := mustGet(t, realLedger(t), "/")

	files, err := filepath.Glob(filepath.Join(root, ".dira", "entries", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 20 {
		// Without this the test passes on an empty glob, which is the
		// vacuous green it exists to prevent.
		t.Fatalf("found %d entry files; the check is not measuring anything", len(files))
	}
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".md")
		if !strings.Contains(body, ">"+id+"<") {
			t.Errorf("%s is in .dira/entries/ and not on the served index", id)
		}
	}
}

// TestEveryIndexLinkResolves walks the index's own links. A page listing ids it
// cannot serve is worse than one that omits them, because the omission is
// visible and the dead link is not.
func TestEveryIndexLinkResolves(t *testing.T) {
	t.Parallel()
	srv := realLedger(t)
	body := mustGet(t, srv, "/")

	links := regexp.MustCompile(`href="(/e/[a-z]+-[0-9]{4,})"`).FindAllStringSubmatch(body, -1)
	if len(links) < 20 {
		t.Fatalf("the index carries %d entry links; the check is not measuring anything", len(links))
	}
	seen := map[string]bool{}
	for _, m := range links {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		if code, _ := get(t, srv, m[1]); code != http.StatusOK {
			t.Errorf("the index links %s, which serves %d", m[1], code)
		}
	}
}

func TestDecisionPageRendersTheRealEntry(t *testing.T) {
	t.Parallel()
	body := mustGet(t, realLedger(t), "/e/dec-0001")

	want := []string{
		"Go, not Elixir/OTP, despite kazi&#39;s stack",      // the ruling, escaped
		"Elixir/OTP, reusing kazi&#39;s Burrito",            // an alternative's option
		"BEAM start-up is tens to hundreds of milliseconds", // its why_not
		"revisit if",   // the revisit condition's label
		"derives from", // the rail
		"int-0002",     // the parent, in the chain and the rail
		"Alternatives on record — 4",
	}
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("the served decision page does not carry %q", w)
		}
	}
}

// TestUnknownIdIsARenderedPage covers the shape of a miss, which on the surface
// a stranger lands on is most of what they will judge the tool by.
func TestUnknownIdIsARenderedPage(t *testing.T) {
	t.Parallel()
	srv := realLedger(t)

	for _, path := range []string{"/e/dec-9999", "/e/not-an-id", "/e/", "/nope"} {
		code, body := get(t, srv, path)
		if code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, code)
		}
		for _, want := range []string{`class="topbar"`, "written by an agent, read by one", `<title>`} {
			if !strings.Contains(body, want) {
				t.Errorf("the 404 for %s is not a rendered page: missing %q", path, want)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// The three checkable laws, and the token contract
// ---------------------------------------------------------------------------

// hexLiteral is the grep from DESIGN.md's Tokens section, run over the served
// document rather than over the mockup.
var hexLiteral = regexp.MustCompile(`#[0-9a-fA-F]{3,6}`)

func TestOnlyTheTwoSanctionedHexLiteralsAreServed(t *testing.T) {
	t.Parallel()
	srv := realLedger(t)

	// The one sanctioned exception: <meta name="theme-color"> cannot
	// reference a CSS custom property, so each page repeats the two ground
	// values literally. Everything else must be a token.
	allowed := map[string]bool{"#0f151c": true, "#f7f4ed": true}

	for _, path := range []string{"/", "/e/dec-0001", "/e/int-0002", "/e/nope"} {
		_, body := get(t, srv, path)
		for _, hex := range hexLiteral.FindAllString(body, -1) {
			if !allowed[strings.ToLower(hex)] {
				t.Errorf("GET %s serves the hex literal %s; a hardcoded colour in a page is a defect", path, hex)
			}
		}
		for want := range allowed {
			if !strings.Contains(body, want) {
				t.Errorf("GET %s is missing the sanctioned theme-color %s", path, want)
			}
		}
	}
}

// TestNoScriptOnAnySurface is dec-0012 as a test rather than as a promise:
// the crawlable pages are crawlable because there is nothing to run, not
// because a runtime happens to be fast. Explicitly the two surfaces
// dec-0012's crawler argument covers — / and /e/<id> — and not /distill,
// which E6-L3-T5 licenses to carry a <script> for the reasons
// TestDistillHasExactlyOneScript checks. Narrowed rather than renamed:
// "on any surface" already only ever named these two, since /distill did
// not exist when this test was written; this comment states the boundary
// the test enforced by construction all along.
func TestNoScriptOnAnySurface(t *testing.T) {
	t.Parallel()
	srv := realLedger(t)

	banned := []string{"<script", "javascript:", " onclick=", " onload=", " onerror="}
	for _, path := range []string{"/", "/e/dec-0001", "/e/nope"} {
		_, body := get(t, srv, path)
		lower := strings.ToLower(body)
		for _, b := range banned {
			if strings.Contains(lower, b) {
				t.Errorf("GET %s contains %q; every surface must render complete with JavaScript disabled (dec-0012)", path, b)
			}
		}
	}
}

// TestDistillHasExactlyOneScript is /distill's counterpart: the one
// dira route licensed to carry a <script> (E6-L3-T5) carries exactly one,
// and it reaches nowhere but this same origin.
//
// Both sides: the "exactly one script, /distill only" claim is proven able
// to fail by TestASecondScriptOnTheDecisionPageIsCaught below, which patches
// a SECOND inline <script> onto decision.gohtml's own source and asserts
// TestNoScriptOnAnySurface's own check (the substring scan this test reuses)
// would catch it — before trusting that scan's silence on the real pages.
func TestDistillHasExactlyOneScript(t *testing.T) {
	t.Parallel()
	srv := distillServer(t, distillEntry("dec-0001", "one"))
	body := mustGet(t, srv, "/distill")

	if got := strings.Count(strings.ToLower(body), "<script"); got != 1 {
		t.Fatalf("GET /distill contains %d <script> tags, want exactly 1", got)
	}
	scriptStart := strings.Index(strings.ToLower(body), "<script")
	scriptEnd := strings.Index(strings.ToLower(body[scriptStart:]), "</script>")
	if scriptEnd < 0 {
		t.Fatal("the <script> block is never closed")
	}
	block := body[scriptStart : scriptStart+scriptEnd]

	for _, banned := range []string{"http://", "https://"} {
		if strings.Contains(block, banned) {
			t.Errorf("the script block contains %q; every fetch() target must be same-origin and loopback-relative (cst-0004)", banned)
		}
	}
	for _, banned := range []string{"document.cookie", "authorization", "cors", "no-cors"} {
		if strings.Contains(strings.ToLower(block), banned) {
			t.Errorf("the script block contains %q; the fetch calls must carry no cookie, no auth header and no cross-origin request (cst-0004)", banned)
		}
	}
}

// TestASecondScriptOnTheDecisionPageIsCaught is
// TestNoScriptOnAnySurface's negative control: it patches a second, harmless
// inline <script> onto a COPY of decision.gohtml's source (never the real
// file), renders it, and asserts the same substring scan that test uses
// would flag it — the narrowed test's own teeth, checked directly rather
// than assumed from the real pages' silence.
func TestASecondScriptOnTheDecisionPageIsCaught(t *testing.T) {
	t.Parallel()

	src, err := templates.ReadFile("templates/decision.gohtml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "</body>") {
		t.Fatal("decision.gohtml has no </body>; update this control to match its current shape")
	}
	patched := strings.Replace(string(src), "</body>", "<script>1;</script></body>", 1)
	if patched == string(src) {
		t.Fatal("the patch did not change anything; the control stages no defect")
	}

	tpl, err := template.New("decision.gohtml").Parse(patched)
	if err != nil {
		t.Fatalf("parsing the patched template: %v", err)
	}
	view := &Decision{Title: "t", Ruling: "r", ID: "dec-0001"}
	var b strings.Builder
	if err := tpl.Execute(&b, view); err != nil {
		t.Fatalf("executing the patched template: %v", err)
	}
	if !strings.Contains(strings.ToLower(b.String()), "<script") {
		t.Fatal("the patched template does not stage a <script> tag; the control is broken")
	}
}

// TestRedMeansTheCompassCaughtSomething is DESIGN.md law 1, measured on the
// served markup: --caught may appear only in a drift block.
func TestRedMeansTheCompassCaughtSomething(t *testing.T) {
	t.Parallel()
	srv := realLedger(t)

	for _, path := range []string{"/", "/e/dec-0001"} {
		_, body := get(t, srv, path)
		if strings.Contains(body, "--caught") {
			t.Errorf("GET %s uses --caught in its markup; the alarm hue belongs to tokens.css's .drift rules alone (law 1)", path)
		}
		// A refusal must not be marked with the alarm class either.
		for _, m := range regexp.MustCompile(`<details class="alt[^"]*"`).FindAllString(body, -1) {
			if strings.Contains(m, "caught") || strings.Contains(m, "drift") {
				t.Errorf("GET %s draws a refusal as an alarm: %s", path, m)
			}
		}
	}
}

// TestTheGraphIsTypeNeverAPicture is law 3. The chain may re-form between
// viewports but it may never become an image, a canvas or an SVG of glyphs.
func TestTheGraphIsTypeNeverAPicture(t *testing.T) {
	t.Parallel()
	body := mustGet(t, realLedger(t), "/e/dec-0001")

	chain := between(t, body, `<section class="chain"`, `</section>`)
	stack := between(t, body, `<section class="chain-stack"`, `</section>`)
	for name, block := range map[string]string{"chain": chain, "chain-stack": stack} {
		for _, banned := range []string{"<svg", "<canvas", "<img", "background-image"} {
			if strings.Contains(block, banned) {
				t.Errorf("the %s block contains %q; the graph is type, never a picture (law 3)", name, banned)
			}
		}
		if !strings.Contains(block, "int-0002") {
			t.Errorf("the %s block does not carry the parent id as text", name)
		}
	}
}

// TestNoUpheldCardIsEverRendered is dec-0019 in Go, beside the same assertion in
// fixture-check.mjs. The mockups and the template have to agree, and two checks
// on the two sides is how they stay agreed when only one side is edited.
func TestNoUpheldCardIsEverRendered(t *testing.T) {
	t.Parallel()
	srv := realLedger(t)

	for _, path := range []string{"/e/dec-0001", "/e/dec-0002", "/e/dec-0019"} {
		_, body := get(t, srv, path)
		for _, banned := range []string{`class="alt upheld"`, `<span class="tag">upheld`, `<span class="mark">✓`, `<span class="ok">✓`} {
			if strings.Contains(body, banned) {
				t.Errorf("GET %s renders %q — `alternatives` records the roads NOT taken, and the chosen "+
					"road is the ruling (dec-0019)", path, banned)
			}
		}
		if !strings.Contains(body, `<span class="mark">✗`) {
			t.Errorf("GET %s renders no refusal mark at all; the struck refusal is the page's device", path)
		}
	}
}

// TestNoExecutionStatusIsInvented is dec-0004. dira owns ledger states and kazi
// owns run states; no join exists here, so nothing on these pages may claim one.
func TestNoExecutionStatusIsInvented(t *testing.T) {
	t.Parallel()
	srv := realLedger(t)

	// "converged" is the load-bearing word: it is kazi's verdict, and a page
	// that prints it without the join is dira asserting something it never
	// checked.
	banned := []string{"converged", "in progress", "predicates green"}
	for _, path := range []string{"/", "/e/dec-0001"} {
		_, body := get(t, srv, path)
		text := stripTags(body)
		for _, b := range banned {
			if strings.Contains(strings.ToLower(text), b) {
				t.Errorf("GET %s prints %q as visible text; execution status is a kazi join dira has not made (dec-0004)", path, b)
			}
		}
	}
}

// TestTheIndexSaysTheJoinIsUnavailable is the other half of dec-0004: degrading
// silently is the failure, not degrading.
func TestTheIndexSaysTheJoinIsUnavailable(t *testing.T) {
	t.Parallel()
	body := mustGet(t, realLedger(t), "/")
	for _, want := range []string{"Execution status", "never stored", "ledger&#39;s own states"} {
		if !strings.Contains(body, want) {
			t.Errorf("the index does not state that the kazi join is unavailable: missing %q", want)
		}
	}
}

// TestTheDialAgreesWithTheLegend keeps the picture and the numbers together. A
// dial whose aria-label reports different counts from the visible legend is two
// answers to one question, and the screen-reader user gets the wrong one.
func TestTheDialAgreesWithTheLegend(t *testing.T) {
	t.Parallel()
	body := mustGet(t, realLedger(t), "/")

	label := between(t, body, `<svg viewBox="0 0 120 120" role="img" aria-label="`, `"`)
	legend := stripTags(between(t, body, `<div class="legend">`, `</div>`))

	numbers := regexp.MustCompile(`\d+`).FindAllString(label, -1)
	if len(numbers) != 5 {
		t.Fatalf("the dial label %q does not carry five numbers", label)
	}
	for _, n := range numbers[1:] {
		if !strings.Contains(legend, n) {
			t.Errorf("the dial label reports %s, which the visible legend %q does not", n, legend)
		}
	}
}

// ---------------------------------------------------------------------------
// A ledger entry is untrusted text
// ---------------------------------------------------------------------------

// TestHostileProseIsEscaped is the one test here that could hide a real
// vulnerability if it were vacuous, so it is written against a ledger built for
// the purpose rather than against prose that happens to contain a bracket.
//
// A decision body can contain anything: an agent writes it from a transcript,
// and a transcript can contain a code block containing a script tag.
func TestHostileProseIsEscaped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	entries := filepath.Join(dir, "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatal(err)
	}

	const payload = `<script>alert(1)</script>`
	// Escaped for YAML: the decoded scalar carries a bare double quote, which
	// is what an attribute breakout needs.
	const attrPayloadYAML = `\" onmouseover=\"alert(2)`
	const attrPayload = `" onmouseover="alert(2)`
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(entries, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("int-0001.md", "---\nid: int-0001\nkind: intent\ntitle: \"Hostile parent "+payload+"\"\n"+
		"state: active\ncreated: \"2026-01-01T00:00:00Z\"\n---\n\nA body.\n")
	write("dec-0001.md", "---\nid: dec-0001\nkind: decision\ntitle: \"Hostile title "+payload+"\"\n"+
		"state: accepted\ncreated: \"2026-01-01T00:00:00Z\"\n"+
		"edges:\n  - type: derives_from\n    to: int-0001\n"+
		"alternatives:\n  - option: \"Option "+payload+"\"\n"+
		"    why_not: \"First sentence "+payload+". Second sentence "+attrPayloadYAML+" trailing.\"\n"+
		"    revisit_if: \"Revisit "+payload+"\"\n---\n\n"+
		"Body paragraph "+payload+"\n\n## Heading "+payload+"\n\nAnother "+attrPayload+" paragraph.\n")

	srv := serve(t, dir, "hostile"+payload)

	for _, path := range []string{"/", "/e/dec-0001", "/e/int-0001"} {
		_, body := get(t, srv, path)
		if strings.Contains(body, payload) {
			t.Errorf("GET %s emits %q verbatim — a ledger entry is untrusted text", path, payload)
		}
		if strings.Contains(strings.ToLower(body), "<script") {
			t.Errorf("GET %s emits a <script> tag from ledger prose", path)
		}
		// The breakout shape, not the word: the payload is rendered as
		// visible text, so "onmouseover" appears on the page by design.
		// What must never appear is the unescaped quote that would end an
		// attribute and start a handler.
		if strings.Contains(body, attrPayload) {
			t.Errorf("GET %s lets ledger prose break out of an attribute", path)
		}
		// The escaped form must still be PRESENT: a renderer that dropped
		// the text entirely would pass every assertion above and lose the
		// record, which is the failure this whole product is against.
		if !strings.Contains(body, "&lt;script&gt;") && !strings.Contains(body, "&lt;script&gt") {
			t.Errorf("GET %s escaped the prose away instead of escaping it", path)
		}
	}
}

// TestEscapingCatchesAnUnescapedTemplate is the negative control for the test
// above, and it is the reason that test is evidence rather than a hope.
//
// It renders THE SAME template source, with THE SAME view data, through
// text/template — which is html/template minus the contextual escaping, and
// therefore exactly the mistake a future edit could make. The assertions from
// TestHostileProseIsEscaped are then run against that output and must all fire.
// A checker nobody has watched fail is indistinguishable from one that always
// prints ok.
func TestEscapingCatchesAnUnescapedTemplate(t *testing.T) {
	t.Parallel()

	const payload = `<script>alert(1)</script>`

	src, err := templates.ReadFile("templates/decision.gohtml")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := texttemplate.New("decision.gohtml").Parse(string(src))
	if err != nil {
		t.Fatalf("parsing the template as text/template: %v", err)
	}

	view := &Decision{
		Title:        "Hostile " + payload,
		Ruling:       "Hostile " + payload,
		ID:           "dec-0001",
		Because:      []Para{{Text: "Body " + payload}},
		Alternatives: []Alt{{Option: "Option " + payload, One: "One " + payload, Open: true}},
	}
	var b strings.Builder
	if err := tpl.Execute(&b, view); err != nil {
		t.Fatalf("executing the unescaped template: %v", err)
	}

	if !strings.Contains(b.String(), payload) {
		t.Fatal("the control produced no raw payload, so it stages nothing and the escaping test proves nothing")
	}
	if strings.Contains(b.String(), "&lt;script&gt;") {
		t.Fatal("the control output is already escaped; text/template is not behaving as the control assumes")
	}

	// And the same view through the real, escaping template must not.
	real, err := template.New("decision.gohtml").ParseFS(templates, "templates/decision.gohtml")
	if err != nil {
		t.Fatal(err)
	}
	var safe strings.Builder
	if err := real.ExecuteTemplate(&safe, "decision.gohtml", view); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(safe.String(), payload) {
		t.Error("the served template emits ledger prose unescaped")
	}
	if !strings.Contains(safe.String(), "&lt;script&gt;") {
		t.Error("the served template dropped the prose instead of escaping it")
	}
}

// ---------------------------------------------------------------------------
// The embedded assets
// ---------------------------------------------------------------------------

// TestEmbeddedAssetsMatchTheDesignSource pins every served stylesheet to the
// design file it is a copy of. Go's embed cannot reach outside its package
// directory, so the copy is unavoidable; a copy nothing compares is drift with a
// delay on it.
func TestEmbeddedAssetsMatchTheDesignSource(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	for name, source := range AssetSources {
		got, err := asset(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := readSource(t, root, source)
		if string(got) != string(want) {
			t.Errorf("%s has drifted from %s (%d bytes embedded, %d in the source).\n"+
				"tokens.css and the screen stylesheets are the single source of colour and spacing truth; "+
				"copy the design file over the embedded one rather than editing the copy.",
				name, source, len(got), len(want))
		}
	}
}

// TestTheDriftTestSeesAOneCharacterChange is that check's negative control, in
// both directions: it edits a COPY of each side and asserts the comparison
// fails. Editing the real files to test the checker is how a reference gets
// quietly rewritten to make a gate pass.
func TestTheDriftTestSeesAOneCharacterChange(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	for name, source := range AssetSources {
		embedded, err := asset(name)
		if err != nil {
			t.Fatal(err)
		}
		design := readSource(t, root, source)
		if string(embedded) != string(design) {
			t.Fatalf("%s already differs from %s; the control cannot prove anything from red", name, source)
		}

		// Direction one: the design file moves.
		movedDesign := flipOneByte(t, source, design)
		if string(embedded) == movedDesign {
			t.Errorf("%s: a one-byte change in the design file is invisible to the comparison", source)
		}

		// Direction two: the embedded copy moves.
		movedEmbed := flipOneByte(t, name, embedded)
		if movedEmbed == string(design) {
			t.Errorf("%s: a one-byte change in the embedded copy is invisible to the comparison", name)
		}
	}
}

// flipOneByte stages the smallest possible difference. A byte and not a token
// because AssetSources now covers the woff2 faces as well as the stylesheets,
// and "replace the first `px`" cannot stage a change in a binary — it would
// fail the control as unmeasurable on exactly the files that were added last,
// which is the wrong direction for a check to give up in.
func flipOneByte(t *testing.T, what string, b []byte) string {
	t.Helper()
	if len(b) == 0 {
		t.Fatalf("%s is empty; the control is not measuring anything", what)
	}
	moved := append([]byte(nil), b...)
	i := len(moved) / 2
	moved[i]++
	if string(moved) == string(b) {
		t.Fatalf("%s: could not stage a change; the control is not measuring anything", what)
	}
	return string(moved)
}

// TestAssetSourcesCoversEveryEmbeddedAsset stops an asset being added without a
// source to be pinned to, which would be a stylesheet nothing compares.
//
// It walks the whole embedded tree rather than its top level: the fonts sit in
// assets/fonts/, and a top-level listing sees that directory as one entry it can
// tick off without looking inside — which is how three unchecked files get into
// a binary under a check that reports "ok".
func TestAssetSourcesCoversEveryEmbeddedAsset(t *testing.T) {
	t.Parallel()

	embedded := embeddedAssets(t)
	if len(embedded) == 0 {
		t.Fatal("no embedded assets found; the check is not measuring anything")
	}
	for _, name := range embedded {
		if _, ok := AssetSources[name]; !ok {
			t.Errorf("%s is embedded and served but is pinned to no source", name)
		}
	}
	if len(AssetSources) != len(embedded) {
		t.Errorf("AssetSources has %d entries for %d embedded assets", len(AssetSources), len(embedded))
	}
	// A tree walk that found only the three stylesheets would pass every clause
	// above while the fonts were absent from the binary entirely.
	var fonts int
	for _, name := range embedded {
		if strings.HasSuffix(name, ".woff2") {
			fonts++
		}
	}
	if fonts == 0 {
		t.Error("no font is embedded. dec-0016 self-hosts the serif because the system stack " +
			"resolves to a different typeface on Linux than the one this design was tuned against; " +
			"a binary with no face in it has not implemented that decision.")
	}
}

// ---------------------------------------------------------------------------
// The fonts (dec-0016)
// ---------------------------------------------------------------------------

// TestEveryCommittedFontIsEmbeddedAndServed is the census, from the repository
// side inward. dec-0016 was accepted, the three woff2 subsets were committed
// under assets/fonts/, NOTICE and its README were written to satisfy the GUST
// Font Licence — and nothing referenced any of it for the entire time the entry
// read `accepted`. Every design gate measured the mockups, the mockups used the
// system stack, and so no gate could fail.
//
// This is the clause that makes that unrepeatable in the binary: a font in
// assets/fonts/ that `dira ui` will not hand a browser fails the build.
func TestEveryCommittedFontIsEmbeddedAndServed(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	srv := realLedger(t)

	committed, err := filepath.Glob(filepath.Join(root, "assets", "fonts", "*.woff2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(committed) == 0 {
		t.Fatal("no font in assets/fonts/; the check is not measuring anything")
	}

	for _, path := range committed {
		name := filepath.Base(path)
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		// Served, as a real request, and byte-identical to the file the
		// licence record describes. A binary serving different bytes from the
		// ones NOTICE names is serving something NOTICE does not cover.
		url := "/" + FontDir + "/" + name
		code, body := get(t, srv, url)
		if code != http.StatusOK {
			t.Errorf("GET %s = %d: assets/fonts/%s is committed, and carries a GUST Font Licence "+
				"obligation, but `dira ui` will not serve it", url, code, name)
			continue
		}
		if body != string(want) {
			t.Errorf("GET %s is not byte-identical to assets/fonts/%s (%d bytes served, %d on disk)",
				url, name, len(body), len(want))
		}
	}
}

// TestTheServedTokensReferenceEveryServedFont closes the loop the other way:
// take the stylesheet the browser is actually handed, resolve every url() in it
// the way a browser would against /tokens.css, and require the server to answer
// each one. This is a result and not a declaration — it does not read the CSS
// source or the route table, it asks the running server the questions a browser
// asks it.
//
// It also catches the inverse defect the census cannot see: a face embedded,
// routed and served that no stylesheet ever asks for.
func TestTheServedTokensReferenceEveryServedFont(t *testing.T) {
	t.Parallel()
	srv := realLedger(t)

	css := mustGet(t, srv, "/tokens.css")
	// Comments stripped first: a commented-out @font-face is not a reference,
	// and counting one would let the gate pass on prose.
	bare := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")

	asked := map[string]bool{}
	for _, m := range regexp.MustCompile(`url\(\s*"([^"]+)"\s*\)`).FindAllStringSubmatch(bare, -1) {
		ref, err := url.Parse(m[1])
		if err != nil {
			t.Errorf("tokens.css contains url(%q), which is not a URL: %v", m[1], err)
			continue
		}
		if ref.IsAbs() || ref.Host != "" {
			t.Errorf("tokens.css fetches %s from off this process. cst-0004 and int-0002 say the "+
				"served surfaces reach no network; the faces are in the binary.", m[1])
			continue
		}
		// Exactly what a browser does: resolve against the stylesheet's own
		// URL. `..` segments that would climb above the root are discarded,
		// which is why the relative form in tokens.css works both here and
		// when the mockups are read out of the working tree.
		resolved := (&url.URL{Path: "/tokens.css"}).ResolveReference(ref).Path
		asked[resolved] = true

		if code, _ := get(t, srv, resolved); code != http.StatusOK {
			t.Errorf("tokens.css asks for %s, which resolves to %s, and GET %s = %d.\n"+
				"A src that 404s renders the fallback while the stylesheet claims otherwise — and the "+
				"fallback's metrics are the thing dec-0016 exists to stop shipping.",
				m[1], resolved, resolved, code)
		}
	}
	if len(asked) == 0 {
		t.Fatal("the served tokens.css references no font at all. dec-0016 self-hosts the serif; " +
			"a stylesheet that asks for nothing is the state that decision was left in.")
	}

	for _, f := range embeddedFonts(t) {
		if !asked["/"+f] {
			t.Errorf("%s is embedded and routed, and the served tokens.css never asks for it. "+
				"A face nothing references is dead weight in the binary and a licence obligation "+
				"carried for no reason.", f)
		}
	}
}

// TestTheServedSerifLeadsWithTheEmbeddedFace is the clause a request-level check
// cannot reach. Every font could load, every route could answer, and the pages
// would still render in Palatino on the machine that has Palatino — which is
// the machine this design was tuned on, and therefore the machine every gate
// runs on. The embedded face has to be FIRST, and dec-0016 requires the old
// stack be kept behind it so a build without the font still renders.
func TestTheServedSerifLeadsWithTheEmbeddedFace(t *testing.T) {
	t.Parallel()
	srv := realLedger(t)
	css := mustGet(t, srv, "/tokens.css")

	families := map[string]bool{}
	for _, m := range regexp.MustCompile(`@font-face\s*\{[^}]*\}`).FindAllString(css, -1) {
		if f := regexp.MustCompile(`font-family\s*:\s*"([^"]+)"`).FindStringSubmatch(m); f != nil {
			families[f[1]] = true
		}
	}
	if len(families) != 1 {
		t.Fatalf("the served tokens.css declares %d @font-face families, want exactly 1: %v",
			len(families), families)
	}
	var embedded string
	for f := range families {
		embedded = f
	}

	serif := regexp.MustCompile(`--serif\s*:\s*([^;]+);`).FindStringSubmatch(css)
	if serif == nil {
		t.Fatal("the served tokens.css declares no --serif")
	}
	stack := strings.Split(serif[1], ",")
	first := strings.Trim(strings.TrimSpace(stack[0]), `"'`)
	if first != embedded {
		t.Errorf("--serif leads with %q, not with the embedded face %q. A self-hosted face behind a "+
			"system font never renders on a machine that has the system font.", first, embedded)
	}
	if len(stack) < 3 {
		t.Errorf("--serif is %q — dec-0016 keeps the old stack behind the embedded face explicitly, "+
			"so a build that somehow ships without the font still renders.", serif[1])
	}
	if last := strings.TrimSpace(stack[len(stack)-1]); last != "serif" {
		t.Errorf("--serif ends in %q, not the generic `serif`; the fallback chain has no floor", last)
	}
}

// TestTheFontCensusSeesAnUnwiredFace is the negative control for the two checks
// above, and it is the shape of the actual defect rather than a convenient one:
// a face present in the binary that the stylesheet never names.
//
// It runs against a COPY of the served stylesheet and the real embedded
// listing. Editing tokens.css to test the checker is how a reference gets
// quietly rewritten to make a gate pass.
func TestTheFontCensusSeesAnUnwiredFace(t *testing.T) {
	t.Parallel()
	srv := realLedger(t)
	css := mustGet(t, srv, "/tokens.css")

	// Direction one: a face is embedded and the stylesheet does not name it.
	// Staged by stripping the @font-face blocks — which is precisely the file
	// dec-0016 left behind.
	stripped := regexp.MustCompile(`(?s)@font-face\s*\{[^}]*\}`).ReplaceAllString(css, "")
	if stripped == css {
		t.Fatal("the served tokens.css has no @font-face block to strip; the control is not measuring anything")
	}
	asked := referencedFonts(stripped)
	unwired := 0
	for _, f := range embeddedFonts(t) {
		if !asked["/"+f] {
			unwired++
		}
	}
	if unwired == 0 {
		t.Error("with every @font-face removed, the census still finds no unwired face — it is not " +
			"reading the stylesheet it claims to read, and would have passed the tree dec-0016 left")
	}

	// Direction two: the stylesheet names a face that is not there. The
	// comparison must notice a reference the server cannot answer.
	renamed := strings.ReplaceAll(css, "pagella-italic", "pagella-oblique")
	if renamed == css {
		t.Fatal("could not stage a renamed reference; the control is not measuring anything")
	}
	var dangling int
	for path := range referencedFonts(renamed) {
		if code, _ := get(t, srv, path); code != http.StatusOK {
			dangling++
		}
	}
	if dangling == 0 {
		t.Error("a src: url() pointing at a face the server does not have went unnoticed; " +
			"the check would pass a stylesheet that renders entirely in the fallback")
	}
}

// referencedFonts resolves every url() in a stylesheet the way a browser would
// against /tokens.css. Shared by the check and its control so the control
// cannot pass by exercising different code.
func referencedFonts(css string) map[string]bool {
	bare := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	out := map[string]bool{}
	for _, m := range regexp.MustCompile(`url\(\s*"([^"]+)"\s*\)`).FindAllStringSubmatch(bare, -1) {
		ref, err := url.Parse(m[1])
		if err != nil {
			continue
		}
		out[(&url.URL{Path: "/tokens.css"}).ResolveReference(ref).Path] = true
	}
	return out
}

// embeddedAssets lists every file in the embedded tree, at any depth.
func embeddedAssets(t *testing.T) []string {
	t.Helper()
	var out []string
	// fs.WalkDir would be the obvious call. io/fs is on dec-0005's banned list
	// and internal/ui deliberately holds no exemption, so this walks embed.FS's
	// own ReadDir instead — the same reasoning as asset() in assets.go.
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := Assets.ReadDir(dir)
		if err != nil {
			t.Fatalf("listing %s: %v", dir, err)
		}
		for _, e := range entries {
			name := dir + "/" + e.Name()
			if dir == "." {
				name = e.Name()
			}
			if e.IsDir() {
				walk(name)
				continue
			}
			out = append(out, name)
		}
	}
	walk("assets")
	slices.Sort(out)
	return out
}

// embeddedFonts is Fonts() with the error turned into a test failure.
func embeddedFonts(t *testing.T) []string {
	t.Helper()
	fonts, err := Fonts()
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) == 0 {
		t.Fatal("no font is embedded; the check is not measuring anything")
	}
	return fonts
}

// TestTokensAreServedByteForByte is the acceptance clause stated as a request
// rather than as a file comparison: what the browser receives must be what the
// design file says.
func TestTokensAreServedByteForByte(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	srv := realLedger(t)

	for path, source := range map[string]string{
		"/tokens.css":   "docs/design/tokens.css",
		"/decision.css": "docs/design/screens/decision.css",
		"/index.css":    "docs/design/screens/s2-index.html#style",
	} {
		body := mustGet(t, srv, path)
		if want := string(readSource(t, root, source)); body != want {
			t.Errorf("GET %s is not byte-identical to %s (%d bytes served, %d expected)",
				path, source, len(body), len(want))
		}
	}
}

// TestServedFromEmbedFS proves the assets are in the binary and not read off
// disk: the process working directory is moved outside the repository first, so
// a handler that fell back to a file read would fail rather than pass quietly.
//
// It is deliberately not parallel — it changes process state.
func TestServedFromEmbedFS(t *testing.T) {
	root := repoRoot(t)
	store, err := local.Open(filepath.Join(root, ".dira"))
	if err != nil {
		t.Fatal(err)
	}
	ix, err := index.OpenFresh(context.Background(), store, t.TempDir())
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

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := t.TempDir()
	if err := os.Chdir(elsewhere); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	if _, err := os.Stat("docs"); err == nil {
		t.Fatal("the temporary directory contains repo files; the check is not measuring anything")
	}
	paths := []string{"/tokens.css", "/decision.css", "/index.css", "/e/dec-0001"}
	// The fonts belong in this list and not in a separate test: dec-0012 is the
	// reason they are embedded rather than fetched, and "works with the network
	// unplugged" is worth nothing if it holds for the stylesheets and not for
	// the faces the stylesheets ask for.
	for _, f := range embeddedFonts(t) {
		paths = append(paths, "/"+f)
	}
	for _, path := range paths {
		if code, _ := get(t, srv, path); code != http.StatusOK {
			t.Errorf("GET %s = %d with the working directory outside the repo; the asset is not embedded", path, code)
		}
	}
}

// ---------------------------------------------------------------------------
// Binding
// ---------------------------------------------------------------------------

func TestListenRefusesAnythingButLoopback(t *testing.T) {
	t.Parallel()

	refused := []string{"0.0.0.0:0", ":8080", "192.168.1.10:0", "example.com:80", "[::]:0"}
	for _, addr := range refused {
		ln, err := Listen(addr)
		if err == nil {
			_ = ln.Close()
			t.Errorf("Listen(%q) succeeded; a ledger reachable off this machine is a ledger published by accident (cst-0004)", addr)
			continue
		}
		if !strings.Contains(err.Error(), "cst-0004") {
			t.Errorf("Listen(%q) refused without naming the constraint: %v", addr, err)
		}
	}

	// The other side: loopback must actually work, or the check above is
	// satisfied by a function that refuses everything.
	for _, addr := range []string{"", "127.0.0.1:0", "[::1]:0"} {
		ln, err := Listen(addr)
		if err != nil {
			t.Errorf("Listen(%q) = %v, want a listener", addr, err)
			continue
		}
		_ = ln.Close()
	}
}

// ---------------------------------------------------------------------------
// The design fixture ledger renders too
// ---------------------------------------------------------------------------

func TestTheDesignFixtureLedgerServes(t *testing.T) {
	t.Parallel()
	srv := designFixture(t)

	body := mustGet(t, srv, "/e/dec-0001")
	for _, want := range []string{
		"Alternatives on record — 3",
		"An equivalent start-up and single-binary story",
		"kazi:prop-8a1f",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the fixture decision page does not carry %q", want)
		}
	}

	index := mustGet(t, srv, "/")
	for _, want := range []string{"int-0001", "int-0002", "int-0003", "int-0004"} {
		if !strings.Contains(index, ">"+want+"<") {
			t.Errorf("the fixture index does not carry %q", want)
		}
	}
	// int-0004 is the orphan the fixture exists to render.
	if !strings.Contains(index, "Drift") {
		t.Error("the fixture index does not render the drift row, which is the state int-0004 exists to produce")
	}
}

// ---------------------------------------------------------------------------
// Derived text
// ---------------------------------------------------------------------------

func TestSplitGround(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, one, rest string }{
		{"", "", ""},
		{"One sentence only.", "One sentence only.", ""},
		{"First. Second one follows.", "First.", "Second one follows."},
		{
			"BEAM start-up is tens of ms. OTP's value is supervision.",
			"BEAM start-up is tens of ms.",
			"OTP's value is supervision.",
		},
		{
			// A lowercase continuation is not a new sentence, so the
			// summary must not break mid-clause.
			"Costs 100 ms. and then some more text here.",
			"Costs 100 ms. and then some more text here.",
			"",
		},
		{
			// Folded YAML arrives hand-wrapped; the wrap is an artifact
			// of the file's width, not of the text.
			"A claim\nwrapped over lines. And the\nrest of it.",
			"A claim wrapped over lines.",
			"And the rest of it.",
		},
	}
	for _, c := range cases {
		one, rest := splitGround(c.in)
		if one != c.one || rest != c.rest {
			t.Errorf("splitGround(%q) = (%q, %q), want (%q, %q)", c.in, one, rest, c.one, c.rest)
		}
	}
}

func TestParagraphs(t *testing.T) {
	t.Parallel()

	got := paragraphs("Lede paragraph.\n\n## A heading\n\nSecond\nparagraph.\n")
	want := []Para{
		{Text: "Lede paragraph."},
		{Text: "A heading", Heading: true},
		{Text: "Second paragraph."},
	}
	if len(got) != len(want) {
		t.Fatalf("paragraphs() returned %d paragraphs, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("paragraph %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestTheDegradationRuleIsSix pins dec-0017's threshold, amended by dec-0019: at
// six or fewer every row is open, above six every row is closed.
func TestTheDegradationRuleIsSix(t *testing.T) {
	t.Parallel()
	srv := realLedger(t)

	count := func(path string) (alts, open int) {
		_, body := get(t, srv, path)
		return strings.Count(body, `<details class="alt"`),
			strings.Count(body, `<details class="alt" open>`)
	}

	// dec-0001 has four alternatives; dec-0010 has four; dec-0012 has three.
	if alts, open := count("/e/dec-0001"); alts != open || alts == 0 {
		t.Errorf("/e/dec-0001 has %d alternatives and %d open; at six or fewer every row is open", alts, open)
	}

	// The above-six case is built rather than assumed, because no entry in
	// this ledger currently has seven.
	dir := t.TempDir()
	entries := filepath.Join(dir, "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("---\nid: dec-0001\nkind: decision\ntitle: \"Seven roads not taken\"\nstate: accepted\n" +
		"created: \"2026-01-01T00:00:00Z\"\nalternatives:\n")
	for i := 0; i < 7; i++ {
		b.WriteString("  - option: \"Option ")
		b.WriteString(string(rune('A' + i)))
		b.WriteString("\"\n    why_not: \"A first sentence. And the rest of the ground.\"\n")
	}
	b.WriteString("---\n\nA body.\n")
	if err := os.WriteFile(filepath.Join(entries, "dec-0001.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	long := serve(t, dir, "long")
	body := mustGet(t, long, "/e/dec-0001")
	if got := strings.Count(body, `<details class="alt"`); got != 7 {
		t.Fatalf("the seven-alternative page renders %d rows", got)
	}
	if got := strings.Count(body, `<details class="alt" open>`); got != 0 {
		t.Errorf("the seven-alternative page opens %d rows; above six the page is an index that expands in place", got)
	}
	if !strings.Contains(body, "Alternatives on record — 7") {
		t.Error("the seven-alternative page does not count its own rows")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// readSource reads a design file, or — for the "#style" suffix — the inline
// <style> block of a mockup that keeps its rules in the page.
func readSource(t *testing.T, root, source string) []byte {
	t.Helper()
	path, fragment, _ := strings.Cut(source, "#")
	raw, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatalf("reading %s: %v", source, err)
	}
	if fragment != "style" {
		return raw
	}
	block := regexp.MustCompile(`(?s)<style>\n(.*?)</style>`).FindSubmatch(raw)
	if block == nil {
		t.Fatalf("%s has no <style> block", path)
	}
	return block[1]
}

func between(t *testing.T, s, open, close string) string {
	t.Helper()
	i := strings.Index(s, open)
	if i < 0 {
		t.Fatalf("the document has no %q", open)
	}
	rest := s[i+len(open):]
	j := strings.Index(rest, close)
	if j < 0 {
		t.Fatalf("the document has %q with no %q after it", open, close)
	}
	return rest[:j]
}

var tagPattern = regexp.MustCompile(`<[^>]*>`)

func stripTags(s string) string { return tagPattern.ReplaceAllString(s, " ") }
