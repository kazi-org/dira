package distill

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kazi-org/dira/internal/ledger"
)

// E2-L4-T8's renderer, checked against dec-0019 in both directions.
//
// Every assertion here has a second half, because the interesting claims are all
// claims about ABSENCE — no alternatives block, no invented summary, no `mirrors
// to adr/…` line — and an absence is only evidence when the renderer has a code
// path that could have produced it. A card that never emits `mirrors to` under
// any circumstances makes "this card carries no mirrors line" a fact about the
// template rather than about the entry, which is precisely the vacuous-green
// failure docs/lore.md L-0001 is about. So each of those three is asserted absent
// on the sniff-shaped fixture AND present on a fixture that records the prose it
// derives from.

// updateGolden rewrites the goldens instead of comparing against them:
//
//	go test ./internal/distill -run TestRender -update
//
// The goldens are the deliverable — this is the screen a demo screenshots
// (.agents/product-marketing.md §6) — so a diff in them is a change to the
// product's copy and has to be read as one in review.
var updateGolden = flag.Bool("update", false, "rewrite the card goldens from the renderer")

// goldenWidth is the width every golden is laid out at. Fixed rather than taken
// from the terminal, because a golden that depended on the window the test ran
// in would compare a card against whoever last regenerated it.
const goldenWidth = 76

// sniffCard is the entry `dira sniff` actually writes, as a card in a queue of
// three. sniffShaped (dispose_test.go) is the shared fixture for that shape, so
// the renderer and the transitions are held against one description of it.
func sniffCard() Item {
	return Item{Entry: sniffShaped("dec-0031"), Status: StatusAwaiting}
}

// fullCard is the other end of the range: everything dec-0019 permits a card to
// derive, recorded. It is what a semantic-tier capture or a hand-written entry
// looks like, and it exists so that every "renders nothing" assertion on
// sniffCard can be shown to be a measurement rather than an omission.
func fullCard() Item {
	return Item{
		Entry: &ledger.Entry{
			ID:      "dec-0011",
			Kind:    ledger.KindDecision,
			Title:   "Status is read from kazi at query time, never written into an entry",
			State:   ledger.StateStaged,
			Created: "2026-07-31T14:22:00Z",
			Edges: []ledger.Edge{
				{Type: ledger.EdgeDerivesFrom, To: "int-0003"},
			},
			Alternatives: []ledger.Alternative{{
				Option: "Store planned/in-progress/done as a field on each entry",
				WhyNot: "correct at the moment of writing and wrong within a day, " +
					"and the wrongness is invisible.",
			}},
			Source: &ledger.Source{
				Hook:    ledger.HookPreCompact,
				Session: "1f0c6a3e-0000-4000-8000-00000000000b",
				Excerpt: "status should come from kazi at query time, not be written down",
				Tier:    ledger.TierSemantic,
			},
			ADR:  "docs/adr/0011-status-is-derived.md",
			Body: "Because a hand-entered status field is exactly what went stale in Obsidian and Linear — a derived view cannot lie.",
		},
		Status: StatusAwaiting,
	}
}

// TestRender is the golden comparison, plus the three absences the acceptance
// line names, each shown against the fixture that produces them.
func TestRender(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		item   Item
		golden string
	}{
		{name: "the shape dira sniff writes", item: sniffCard(), golden: "sniff-shaped.golden"},
		{name: "everything dec-0019 permits", item: fullCard(), golden: "full.golden"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderCard(tc.item, 1, 3, goldenWidth)
			compareGolden(t, tc.golden, got)
		})
	}

	t.Run("the sniff-shaped card derives nothing it was not given", func(t *testing.T) {
		t.Parallel()

		bare := renderCard(sniffCard(), 1, 3, goldenWidth)
		full := renderCard(fullCard(), 1, 3, goldenWidth)

		// Three absences, each with the presence that proves the code path
		// exists. Without the right-hand column these are assertions about
		// a template; with it they are assertions about the entry.
		for _, probe := range []struct {
			what   string
			marker string
			why    string
		}{
			{
				what:   "an alternatives block",
				marker: "✗",
				why:    "the regex tier writes no alternatives, and dec-0003 forbids inventing one",
			},
			{
				what:   "an ADR mirror line",
				marker: "mirrors to",
				why:    "no transition in this package writes `adr`, and the mirror (dec-0009) is not in this lane",
			},
			{
				what: "a because",
				// The whole opening clause rather than the word, because
				// the key legend says "edit the because" on every card and
				// a one-word probe would be matching the legend.
				marker: "Because a hand-entered status field",
				why:    "dira sniff writes no body, and a card that supplied one would be inventing the rationale",
			},
		} {
			if strings.Contains(bare, probe.marker) {
				t.Errorf("the sniff-shaped card carries %s (%q): %s\n%s",
					probe.what, probe.marker, probe.why, bare)
			}
			if !strings.Contains(full, probe.marker) {
				t.Errorf("the fully recorded card carries no %s (%q), so its absence on the "+
					"sniff-shaped card measures the template rather than the entry, and the "+
					"assertion above is vacuous\n%s", probe.what, probe.marker, full)
			}
		}
	})

	t.Run("a card with no entry renders nothing", func(t *testing.T) {
		t.Parallel()

		if got := renderCard(Item{}, 1, 3, goldenWidth); got != "" {
			t.Errorf("an empty item rendered %q, want the empty string", got)
		}
	})
}

// TestRenderPutsTheBecauseAboveTheTitle is the design's ordering: the because is
// what is being approved, the title is the label.
//
// Asserted by byte offsets and not by reading the golden, because "above" is a
// claim about position that a human comparing two blocks of text will confirm
// whichever way round they are.
func TestRenderPutsTheBecauseAboveTheTitle(t *testing.T) {
	t.Parallel()

	item := fullCard()
	card := renderCard(item, 2, 3, goldenWidth)

	// The first six words of each, so the probe survives wrapping: the
	// renderer joins wrapped lines with a newline, and a whole sentence would
	// not be found as a contiguous substring at any realistic width.
	because := opening(item.Entry.Body, 6)
	title := opening(item.Entry.Title, 6)

	at, titleAt := strings.Index(card, because), strings.Index(card, title)
	switch {
	case at < 0:
		t.Fatalf("the card does not carry the because %q\n%s", because, card)
	case titleAt < 0:
		t.Fatalf("the card does not carry the title %q\n%s", title, card)
	case at >= titleAt:
		t.Errorf("the because starts at byte %d and the title at byte %d; the because leads the card "+
			"(docs/design/screens/s3-distill.html: the because is what is being approved)\n%s",
			at, titleAt, card)
	}

	// The struck refusal and its grounds, which is the device dec-0019
	// preserved rather than the comparison list r3 → r4 rejected.
	alt := item.Entry.Alternatives[0]
	optionAt := strings.Index(card, opening(alt.Option, 5))
	groundsAt := strings.Index(card, opening(alt.WhyNot, 5))
	switch {
	case optionAt < 0:
		t.Errorf("the card does not carry the struck option %q\n%s", alt.Option, card)
	case groundsAt < 0:
		t.Errorf("the card does not carry the alternative's why_not %q\n%s", alt.WhyNot, card)
	case groundsAt < optionAt:
		t.Errorf("the grounds start at byte %d and the option at byte %d; the grounds sit BENEATH the "+
			"strike (dec-0019, on what the r3 → r4 record actually defends)\n%s", groundsAt, optionAt, card)
	}
}

// TestRenderSourceLineNamesHookTimeTier pins the order the acceptance line
// gives, which is the order docs/design/screens/s3-distill.html shows:
// `PreCompact · 14:22 · semantic`.
func TestRenderSourceLineNamesHookTimeTier(t *testing.T) {
	t.Parallel()

	for _, item := range []Item{sniffCard(), fullCard()} {
		entry := item.Entry
		card := renderCard(item, 1, 3, goldenWidth)

		line := lineContaining(card, string(entry.Source.Hook))
		if line == "" {
			t.Fatalf("no line of the card names the hook %q\n%s", entry.Source.Hook, card)
		}

		hook := strings.Index(line, string(entry.Source.Hook))
		clock := strings.Index(line, clockOf(entry.Created))
		tier := strings.Index(line, string(entry.Source.Tier))
		if clock < 0 || tier < 0 {
			t.Fatalf("the source line %q does not name the time and the tier", line)
		}
		if hook >= clock || clock >= tier {
			t.Errorf("the source line %q orders hook/time/tier at %d/%d/%d, want ascending",
				line, hook, clock, tier)
		}

		// The other half: the line carries the recorded time and not a
		// re-derived one. A renderer that parsed and re-formatted the
		// stamp would show an hour the file does not contain.
		if !strings.Contains(line, clockOf(entry.Created)) {
			t.Errorf("the source line %q does not carry the recorded time %q", line, clockOf(entry.Created))
		}
		if strings.Contains(card, entry.Source.Session) {
			t.Errorf("the card carries the session id %q; it is provenance for an auditor, not "+
				"something a human disposing of a card can act on\n%s", entry.Source.Session, card)
		}
	}
}

// TestRenderShowsProgress covers the `1 of 3` the design promises, and that it
// is the position it was given rather than a constant.
func TestRenderShowsProgress(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		position, total int
		want            string
	}{
		{position: 1, total: 3, want: "1 of 3"},
		{position: 2, total: 3, want: "2 of 3"},
		{position: 7, total: 7, want: "7 of 7"},
		{position: 1, total: 1, want: "1 of 1"},
	} {
		card := renderCard(sniffCard(), tc.position, tc.total, goldenWidth)
		if !strings.Contains(card, tc.want) {
			t.Errorf("the card for position %d of %d does not carry %q\n%s",
				tc.position, tc.total, tc.want, card)
		}
		if tc.want != "1 of 3" && strings.Contains(card, "1 of 3") {
			t.Errorf("the card for position %d of %d still carries \"1 of 3\"; the counter is a "+
				"constant\n%s", tc.position, tc.total, card)
		}
	}
}

// slop is the register docs/design.md §10 rules out: precise, unhyped, real ids.
// The `!` is in the list because a review queue that shouts at a human once per
// card is a queue they stop reading.
var slop = regexp.MustCompile(`(?i)successfully|!|revolutionary|seamless|supercharge|10x|AI-powered`)

// TestRenderCarriesNoneOfTheSlop checks the copy, and then checks the checker.
//
// The second half matters more than the first here: a pattern that matched
// nothing — a typo in the alternation, a regexp that never compiled the way it
// reads — would pass silently on every card forever.
func TestRenderCarriesNoneOfTheSlop(t *testing.T) {
	t.Parallel()

	for _, item := range []Item{sniffCard(), fullCard()} {
		card := renderCard(item, 1, 3, goldenWidth)
		if found := slop.FindString(card); found != "" {
			t.Errorf("the card for %s carries %q, which docs/design.md §10's register rules out\n%s",
				item.ID(), found, card)
		}
	}

	for _, bad := range []string{
		"dira successfully staged your decision",
		"Confirmed!",
		"a seamless review queue",
		"AI-powered capture",
		"10x fewer keystrokes",
		"supercharge your ledger",
		"a revolutionary way to record decisions",
	} {
		if !slop.MatchString(bad) {
			t.Errorf("the slop pattern does not match %q, so the assertion above is measuring nothing", bad)
		}
	}
}

// TestRenderFitsTheWidth is the layout clause: within the width the harness
// reports, and never wrapping mid-word.
//
// "Never mid-word" is asserted by comparing the token sequence against the same
// card laid out at a width nothing can wrap at. If a wrap ever split a word, one
// token would become two and the sequences would diverge — which is a stronger
// statement than any single case, and it holds for every width at once.
func TestRenderFitsTheWidth(t *testing.T) {
	t.Parallel()

	items := []Item{sniffCard(), fullCard()}
	for _, item := range items {
		unwrapped := words(renderCard(item, 1, 3, 100000))

		for _, width := range []int{40, 56, 64, 72, 80, 100, 120} {
			card := renderCard(item, 1, 3, width)

			for _, line := range strings.Split(strings.TrimRight(card, "\n"), "\n") {
				if n := utf8.RuneCountInString(line); n > width && strings.Contains(strings.TrimSpace(line), " ") {
					t.Errorf("at width %d the card for %s has a %d-column line that could have been "+
						"broken:\n\t%q", width, item.ID(), n, line)
				}
			}

			if got := words(card); !equalTokens(got, unwrapped) {
				t.Errorf("at width %d the card for %s does not carry the same words as the unwrapped "+
					"card; a wrap broke a word\n%v\n%v", width, item.ID(), got, unwrapped)
			}
		}
	}

	// The other half. A word longer than the whole width is left whole and
	// allowed to overflow, because half of an id or a path is not the thing
	// the entry recorded — and the check above must not be passing merely
	// because nothing ever got close to the limit.
	long := "docs/adr/0011-status-is-read-from-kazi-at-query-time-and-never-written.md"
	item := fullCard()
	item.Entry.ADR = long
	card := renderCard(item, 1, 3, 40)
	if !strings.Contains(card, long) {
		t.Errorf("a %d-column path was broken across lines at width 40; recorded text is never "+
			"hyphenated by the renderer\n%s", utf8.RuneCountInString(long), card)
	}
}

// TestRenderInventsNothing is dec-0019's first half, mechanised: every word on
// the card is either the renderer's own fixed vocabulary or a word the entry
// records. Nothing is summarised, nothing is paraphrased, nothing is supplied.
//
// It is a token-set check rather than an eyeball over the golden because the
// failure it exists to catch is a plausible sentence — "a decision about
// storage", "captured from your session" — which reads perfectly well in a
// golden and is exactly the invention dec-0019 refused a field for.
func TestRenderInventsNothing(t *testing.T) {
	t.Parallel()

	for _, item := range []Item{sniffCard(), fullCard()} {
		card := renderCard(item, 1, 3, goldenWidth)
		if invented := unaccountedFor(card, item); len(invented) > 0 {
			t.Errorf("the card for %s carries %v, which is neither the renderer's own vocabulary nor "+
				"anything the entry records (dec-0019: it never invents a field)\n%s",
				item.ID(), invented, card)
		}
	}

	// The other half, and the one that gives the check its teeth: the most
	// plausible inventions this renderer could have made are the two
	// dec-0019 names — a one-line summary, and a card for the upheld option.
	item := fullCard()
	card := renderCard(item, 1, 3, goldenWidth)
	for _, invention := range []string{
		"\n\nA decision about where status comes from.\n",
		"\n\n✓ Read status from kazi at query time\n  The road taken.\n",
		"\n\nCaptured automatically from your session.\n",
	} {
		if invented := unaccountedFor(card+invention, item); len(invented) == 0 {
			t.Errorf("the invention check accepts %q, so it is measuring nothing", invention)
		}
	}
}

// unaccountedFor returns the words on a card that neither the entry records nor
// the renderer is allowed to say on its own.
func unaccountedFor(card string, item Item) []string {
	allowed := map[string]bool{}
	for _, source := range recordedText(item) {
		for _, word := range strings.Fields(source) {
			allowed[word] = true
		}
	}
	// The renderer's own vocabulary, in full. Everything here is chrome: it
	// names a field or a key and asserts nothing about the entry. It is
	// spelled out rather than derived so that adding a word to the card is a
	// line in this list and therefore a line in review.
	for _, word := range append(strings.Fields(keyLegend), "of", "·", "✗", "mirrors", "to",
		"intends", "decides", "asks", "requires", "notes", "excerpt") {
		allowed[word] = true
	}

	var invented []string
	for _, word := range strings.Fields(card) {
		if allowed[word] || isNumber(word) {
			// A bare number is the progress counter; `1 of 3` is the
			// only place the card produces one.
			continue
		}
		invented = append(invented, word)
	}
	return invented
}

// recordedText is every piece of prose the entry actually holds, which is the
// whole of what a card is permitted to show.
func recordedText(item Item) []string {
	entry := item.Entry
	out := []string{entry.ID, string(entry.State), string(item.Status), entry.Title, entry.Body, entry.ADR}
	for _, alt := range entry.Alternatives {
		out = append(out, alt.Option, alt.WhyNot)
	}
	for _, edge := range entry.Edges {
		out = append(out, string(edge.Type), edge.To, edge.Note)
	}
	if entry.Source != nil {
		out = append(out, string(entry.Source.Hook), string(entry.Source.Tier), entry.Source.Excerpt)
	}
	out = append(out, clockOf(entry.Created))
	return out
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// opening is the first n words of s, for probing a card whose lines have been
// wrapped at an unknown column.
func opening(s string, n int) string {
	words := strings.Fields(s)
	if len(words) > n {
		words = words[:n]
	}
	return strings.Join(words, " ")
}

// lineContaining is the first line of text holding want, or "".
func lineContaining(text, want string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, want) {
			return line
		}
	}
	return ""
}

// words is the card's words, with the interpunct separators dropped.
//
// The separator is dropped rather than compared because the key legend is broken
// BETWEEN its segments when it does not fit, and the `·` that joined them goes
// with the break — which is what a separator is for. The claim under test is
// that no WORD is ever split, and this is the token stream that claim is about.
func words(card string) []string {
	var out []string
	for _, token := range strings.Fields(card) {
		if token == "·" {
			continue
		}
		out = append(out, token)
	}
	return out
}

func equalTokens(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// compareGolden holds the rendered card against testdata, or rewrites it under
// -update.
func compareGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
		t.Logf("rewrote %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v (regenerate with `go test ./internal/distill -run TestRender -update`)", path, err)
	}
	if got != string(want) {
		t.Errorf("the card does not match %s byte for byte.\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
	}
}
