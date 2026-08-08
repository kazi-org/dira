# E6-L2 — Serve the read-only surfaces from the Go binary. Report.

**Status: built, tested, and honestly short of one acceptance clause.**
`dira ui` serves `/` and `/e/<id>` as server-rendered `html/template` from
`embed.FS`, over the same query path the CLI uses. Everything in the lane's
`acc:` line passes **except the pixel-diff clause**, which does not fail on
layout — it fails because the mockups carry content no ledger can produce. That
is measured, not asserted, and §6 has the numbers.

The decision you routed to me is §1. It is the first section because it was the
first job.

---

## 1. The upheld card: option (c), and why the device survives without it

**Recorded as `.dira/entries/dec-0019.md`** — *"The upheld option is the ruling,
not an alternative — the renderer derives, never invents"* — `kind: decision`,
`state: accepted`, `source.tier: human`, validated against
`schema/entry.schema.json`. Edges: `derives_from → dec-0012`, `informs →
dec-0017`, `informs → cst-0002`, `informs → dec-0001`.

**The card is dropped. The ruling carries it.** The chain's matching `✓` row went
with it.

### Why not (b), add a field — the option that looks strongest

It needs **two** fields, not one. The mockup gives every alternative a `.one`
summary line as well as a mark, and neither has a home. And both would store
facts the entry already carries:

- **The chosen road is the `title`.** `dec-0001`'s title is *"Go, not Elixir/OTP,
  despite kazi's stack"* — the upheld option, written as a sentence. A second
  field naming the same road is two fields that can disagree about one fact, and
  `dira check` reads the title.
- **The regex tier cannot fill it.** `dira sniff` is lossy by construction
  (`dec-0003`) and cannot infer which option won, so a page's composition would
  vary with *how the entry happened to be captured*. That turns `source.tier`
  from an audit trail into a layout input.
- `cst-0002` closed the kind set on the argument that every plausible addition is
  a tool dira promised not to become. The field-level form of that argument is
  that every added field must be understandable **at every tier**, and this one
  is not.

### Why not (a), synthesize from title + body

It reprints, at 16.5px, the two largest elements on the page: the title is
already the `h1.ruling` at up to 52px, and the body is already the `.because` at
19px directly beneath it. The one-line summary has no source at all. And — as
you flagged — the card's grounds paragraph is **not** the `because`; on the
mockup they are different content about different things. So (a) is one part
duplication and one part invention.

### Why (c) does not break the r3 → r4 device — argued, not defaulted to

You asked for the argument rather than the cheapest option. Four parts:

**1. What r3 → r4 defends is the strike *with its grounds beneath*, not the
strike beside an upheld card.** The rejected fix was "collapse the alternatives
into a scannable comparison list", and the stated reason is that it "would
destroy the struck-through refusal with its grounds beneath". Every refusal keeps
both. `dec-0017`'s own account is identical and mentions no upheld card at all:
*"the summary line carries the grounds, not just the title."*

**2. The counterweight is not lost — it gets bigger.** A strike is legible
against unstruck type. After this change the nearest unstruck type is the
`h1.ruling` at up to 52px, which the same r3 → r4 paragraph made the frame's
anchor at a 3.15:1 ratio against the grounds. That anchor was chosen *instead of*
the comparison list, and its stated direction was "reducing element count and
strengthening the anchor". Deleting a 16.5px card that restated the 52px anchor
is that same move, taken once more — the same move that deleted the duplicate
"This ledger" stats card. Inside each row the strike also sits directly above its
own unstruck grounds, so the contrast exists per-row and not only per-page.

**3. The same record already flags the card as slop-adjacent.** r3 → r4 notes the
upheld card's coloured left border as "adjacent to the 'left-border accent
stripe' slop tell", kept *only because the state was also carried in text*
(WEB.md 12). That defence needs a state to carry. There is none in the data.

**4. Nothing was lost, because nothing was there.** This is the part I did not
expect to find, and it is the one that settled it. The real `dec-0001` records
the grounds the mockup put on the upheld card — **inside the Rust refusal they
answer**:

> *"...but slower to write and a smaller pool of contributors for an OSS tool
> that wants drive-by PRs. **Go's stdlib covers everything dira needs (files,
> JSON/YAML, an embedded HTTP server for `dira ui`, SQLite via a single
> dependency).**"*

The mockup's card reads *"The standard library already covers files, JSON and
YAML, an embedded HTTP server for `dira ui`, and SQLite through a single
dependency."* Same sentence, **moved out of the refusal it belongs to**. The case
for the chosen road is made where the schema puts it: inside the reasons the
other roads lost. A field would have created a second place to put it, and a
second place is how the two drift.

### The rule this establishes, which settles a second gap for free

> The renderer derives presentation from recorded prose. It never invents a
> field, and where nothing can be derived it renders nothing.

`dec-0017` makes the one-line ground load-bearing, and there is no `summary`
field either. There does not need to be one: **the first sentence of `why_not` is
the one-line ground, and the rest is the detail.** Nothing stored, nothing
duplicated, and a reader scanning twenty summaries reads the author's own opening
claim rather than an editor's paraphrase. Implemented in `splitGround`
(`internal/ui/view.go`), tested against six cases including the abbreviation
case (`"Costs 100 ms. and then…"` must not split mid-clause).

### What it cost, and what changed

| file | change |
|---|---|
| `.dira/entries/dec-0019.md` | new |
| `docs/design/screens/s1-decision.html` | upheld `<details>` and the chain's `✓ go` rows removed; "— 4" → "— 3" |
| `docs/design/screens/s1-decision-long.html` | same; "— 20" → "— 19"; the rail's `upheld 1` row removed; all 19 rows now closed |
| `docs/design/screens/s1-decision-withheld.html` | same; "— 3" → "— 2" |
| `docs/design/screens/decision.css` | `.alt.upheld` rules removed; `.alt:not(.upheld) .name` → `.alt .name`; degradation comment amended |
| `docs/design/DESIGN.md` | the `> 6` degradation row, the screens table, the `*Test:*` line, and the r3 → r4 "left as-is" paragraph |
| `docs/design/scripts/fixture-check.mjs` | the FINDING replaced by a hard assertion |
| `docs/design/fidelity/fixtures/README.md` | resolution recorded; the schema-gate section updated |

**`dec-0017`'s above-six rule loses its named anchor.** It said "only the upheld
one open". It now reads **above six, every `<details>` closed**, which is
`dec-0017`'s own description of the outcome ("the page becomes an index that
expands in place") with the exception removed. At or below six, everything stays
open, unchanged.

**What I did *not* change: `docs/design/landing/index.html`.** Its copy refers to
"the struck-through/upheld contrast from s1's alternatives". It is E8-L2's
surface and not a route `dira ui` serves, so it is flagged rather than edited.
`check-coherence.mjs` still passes.

---

## 2. What I built

```
internal/ui/
  assets.go            embed.FS + the AssetSources pin
  assets/tokens.css    copy of docs/design/tokens.css          (pinned)
  assets/decision.css  copy of docs/design/screens/decision.css (pinned)
  assets/index.css     copy of s2-index.html's <style> block    (pinned)
  server.go            routing, loopback refusal, graceful stop
  view.go              the decision page's view model, from a *why.Chain
  indexview.go         the ledger index's view model, all joins derived
  templates/decision.gohtml
  templates/index.gohtml
  templates/error.gohtml
  ui_test.go           28 assertions, every one verified red→green (§4)
  fixtures_test.go     the schema gate you asked for (§3)
cmd/dira/ui.go         the `dira ui` command
cmd/dira/ui_test.go    the CLI surface: flags, refusals, exit codes
docs/design/scripts/uigate.mjs   the 3×2 pixel gate (§6)
```

Design decisions worth naming:

- **The decision page is rendered from a `*why.Chain`.** `internal/why` is one
  producer behind both renderers, as its package comment requires. I re-derived
  nothing.
- **`why.noAlternatives` is now exported as `why.NoAlternatives`** and used by
  both renderers, so an empty record cannot be described two different ways.
  Two-line change; the call site was updated. **Note for the integrator:**
  `internal/why/render.go` is being rewritten concurrently by another lane (I
  found `p.row` → `p.Row` mid-edit), so this may conflict.
- **`--caught` never reaches the markup.** The alarm hue lives entirely in
  `tokens.css`'s `.drift` rules; a served page carries no colour token at all.
- **The chain's vertical rules are `.node` wrappers**, generated in Go from an
  integer and a constant string. That is the only `template.HTML` in the package
  — no ledger value ever reaches the template unescaped.
- **The breadcrumb name comes from `local.Name`, not from `path/filepath` in the
  command.** `dec-0005` puts every path concept in the backend, and taking the
  last segment of a path is naming one. See §8.5.
- **`dira ui` refuses a non-loopback bind** with exit 2 and a message naming
  `cst-0004`. `:8080` is refused too: a bare port binds every interface, which is
  the accident the check exists for.
- **No `<script>`, anywhere.** Asserted, not intended.

---

## 3. The schema gate you asked for

`internal/ui/fixtures_test.go`. Three tests, and the second is what makes the
first evidence:

```
=== RUN   TestDesignFixturesValidate        18 fixture entries, 0 failures
=== RUN   TestTheValidatorRejectsTheInvalidCorpus
    fixtures_test.go:87: 18 invalid fixtures, 18 refused
=== RUN   TestTheRealLedgerValidates        33 entries, 0 failures
--- PASS: TestTheValidatorRejectsTheInvalidCorpus (0.01s)
--- PASS: TestDesignFixturesValidate (0.12s)
--- PASS: TestTheRealLedgerValidates (0.13s)
```

`TestDesignFixturesValidate` asserts the **count** (18) as well as the validity —
without that it would pass just as happily on a directory that had been moved or
emptied. `schema/testdata/invalid/` now holds 18 files, not the 17 E6-L1
recorded; another lane added one. They are all still refused.

It lives in `internal/ui` rather than `schema` because `internal/ui` is what
serves those fixtures: the thing that consumes the fixture should refuse an
invalid one. It also keeps this lane out of `schema/`, which another lane is
editing.

---

## 4. Red-then-green, verbatim

Every assertion was watched failing against a deliberately broken premise, then
watched passing. The mutations were applied to working copies and reverted; no
reference file was edited to make a gate agree with it.

```
RED OK   asset drift (embedded tokens.css edited)
           ui_test.go:504: assets/tokens.css has drifted from docs/design/tokens.css
                           (16681 bytes embedded, 16681 in the source).
           ui_test.go:588: GET /tokens.css is not byte-identical to docs/design/tokens.css
RED OK   upheld card reinstated in the template
           ui_test.go:297: GET /e/dec-0001 renders "<span class=\"tag\">upheld" — `alternatives`
                           records the roads NOT taken, and the chosen road is the ruling (dec-0019)
RED OK   hardcoded hex in a template
           ui_test.go:216: GET /e/dec-0001 serves the hex literal #ff0000
RED OK   a script tag on a surface
           ui_test.go:240: GET /e/dec-0001 contains "<script"; every surface must render
                           complete with JavaScript disabled (dec-0012)
RED OK   the chain becomes a picture
           ui_test.go:277: the chain block contains "<svg"; the graph is type, never a picture (law 3)
RED OK   an invented execution status
           ui_test.go:322: GET / prints "converged" as visible text; execution status is a
                           kazi join dira has not made (dec-0004)
RED OK   an entry dropped from the index
           ui_test.go:127: cst-0003 is in .dira/entries/ and not on the served index
RED OK   --caught used outside a drift block
           ui_test.go:255: GET /e/dec-0001 uses --caught in its markup (law 1)
```

Three more controls are permanent tests rather than one-off mutations:

- **`TestTheDriftTestSeesAOneCharacterChange`** edits *copies* of both sides of
  each asset pin and asserts the comparison fails in both directions. The
  reference files are never touched.
- **`TestEscapingCatchesAnUnescapedTemplate`** parses the *same template source*
  with `text/template` — `html/template` minus the contextual escaping, i.e. the
  exact mistake a future edit could make — renders the same view, and asserts the
  payload comes out raw. Then it renders through the real template and asserts it
  does not. Without this, `TestHostileProseIsEscaped` would be a test that could
  not fail.
- **`TestTheValidatorRejectsTheInvalidCorpus`**, above.

### Escaping, specifically

A ledger entry is untrusted text and a decision body can contain anything.
`TestHostileProseIsEscaped` builds a ledger whose title, alternative option,
`why_not`, `revisit_if` and body all contain `<script>alert(1)</script>`, and
whose `why_not` also contains a bare `" onmouseover="alert(2)` attribute
breakout. Across `/`, `/e/dec-0001` and `/e/int-0001` it asserts:

1. the raw payload never appears,
2. no `<script` appears in any case,
3. the raw breakout sequence never appears,
4. **`&lt;script&gt;` *does* appear** — a renderer that dropped the text would
   pass 1–3 and lose the record, which is the failure this whole product is
   against.

Actual output, for the record:

```
<h1 class="ruling">Hostile title &lt;script&gt;alert(1)&lt;/script&gt;</h1>
<span class="name">Option &lt;script&gt;alert(1)&lt;/script&gt;</span>
<title>Hostile title &lt;script&gt;alert(1)&lt;/script&gt; — decision record kept with dira</title>
```

---

## 5. Every acceptance criterion, with observed evidence

| # | clause | verdict | evidence |
|---|---|---|---|
| 1 | `dira ui` serves `s2-index` at `/` and `s1-decision` at `/e/<id>` over the same query path the CLI uses | **PASS** | `TestBothRoutesServeTheRealLedger`; the page is built from `why.Build(...)` — the same `*why.Chain` `dira why` renders |
| 2 | against the repo's own `.dira/`, every id under `.dira/entries/` appears in the served index | **PASS** | `TestEveryEntryAppearsOnTheIndex` (33 files, all present; fails loudly if fewer than 20 are found). `TestEveryIndexLinkResolves` additionally GETs every `/e/<id>` the page links |
| 3 | both routes pixel-diff within tolerance from freshly regenerated baselines at 3 viewports × 2 schemes | **FAIL — and not on layout** | §6. All 12 pairs, real numbers |
| 3a | …with the `render.mjs` gate passing and zero non-loopback asset requests | **PASS** | `uigate.mjs` runs the same mechanical gate on the **served** pages: "PASS — no console errors, no page errors, no failed requests, no non-loopback assets, no blank mount, no fake dark, no layout shift" at 3 × 2 |
| 4 | the served `tokens.css` is byte-identical to `docs/design/tokens.css`, and a Go test fails on any drift | **PASS** | `TestTokensAreServedByteForByte` compares the HTTP response bytes; `TestEmbeddedAssetsMatchTheDesignSource` pins all three assets; both verified red (§4) |
| 5 | `grep -oE '#[0-9a-fA-F]{3,6}'` over every served HTML body returns only `#0f151c` and `#f7f4ed` | **PASS** | `TestOnlyTheTwoSanctionedHexLiteralsAreServed`, run over `/`, `/e/dec-0001`, `/e/int-0002` and the 404. It also asserts both sanctioned values are **present**, so a page that dropped its `theme-color` tags cannot pass by emitting no hex at all |
| 6 | both routes render their full content with JavaScript disabled | **PASS** | `TestNoScriptOnAnySurface` — no `<script`, no `javascript:`, no `on*=` handler on any surface. There is no JavaScript to disable; the alternatives collapse with `<details>`, which is markup |

Beyond the `acc:` line, from the lane's `entries:` list:

| entry | how it is enforced |
|---|---|
| `dec-0004` (status derived, never stored) | `TestNoExecutionStatusIsInvented` — "converged", "in progress" and "predicates green" must not appear as visible text. `TestTheIndexSaysTheJoinIsUnavailable` — the page must **say** the join is missing, because degrading silently is the failure, not degrading. `TestTheDialAgreesWithTheLegend` — the dial's `aria-label` and the visible legend report the same numbers |
| `cst-0004` / `int-0002` | `TestListenRefusesAnythingButLoopback` (5 refused addresses, each error naming `cst-0004`; and 3 loopback forms that must still work, so the check is not satisfied by a function that refuses everything). `TestServedFromEmbedFS` moves the working directory outside the repo first |
| law 1 | `TestRedMeansTheCompassCaughtSomething` |
| law 3 | `TestTheGraphIsTypeNeverAPicture` — no `<svg>`, `<canvas>`, `<img>` or `background-image` inside `.chain` or `.chain-stack`, and the parent id must be present as text |
| `dec-0017` + `dec-0019` | `TestTheDegradationRuleIsSix` — builds a 7-alternative ledger and asserts all 7 render closed |

Full suite:

```
$ go test ./internal/ui
ok  	github.com/kazi-org/dira/internal/ui	1.182s

$ go test ./internal/ledger -run 'Boundary|NoFilesystem'
  --- PASS: TestNoFilesystemImportsAboveTheBackend (7.10s)
  --- PASS: TestTheImportBoundaryHasTeeth (7.24s)

$ go vet ./internal/ui ./cmd/dira        (clean)
$ golangci-lint run ./internal/ui/...    0 issues.
$ python3 scripts/privacy-lint.py        PRIVACY LINT PASS — cst-0003 enforced by 4 checks.
$ python3 scripts/coverage.py            COVERAGE PASS — every obligation has a disposition.
$ node docs/design/scripts/gates.mjs
  9 gates run · 7 passed · 2 negative controls tripped · 0 failed · 0 blind
  GATES PASS — every gate green, every negative control tripped.
```

---

## 6. The pixel clause: measured, and why it fails

`node docs/design/scripts/uigate.mjs` builds the binary, copies the 18-entry
fixture ledger under a `.dira`, starts the server, **regenerates the mockup
baselines in the same run and the same browser process**, captures both sides at
3 viewports × 2 schemes, runs the mechanical gate on the served pages, and
pixel-diffs each pair. `docs/design/renders/` is gitignored, so no committed
baseline can be smuggled in.

```
MECHANICAL GATE (served pages)
  PASS — no console errors, no page errors, no failed requests, no non-loopback assets,
         no blank mount, no fake dark, no layout shift.

PIXEL DIFF (served vs freshly regenerated mockup)
  DIFF s2-index     mobile  light  dimension mismatch:  780x5608 vs  780x7004
  DIFF s2-index     mobile  dark   dimension mismatch:  780x5608 vs  780x7004
  DIFF s2-index     laptop  light  dimension mismatch: 2048x3282 vs 2048x4082
  DIFF s2-index     laptop  dark   dimension mismatch: 2048x3282 vs 2048x4082
  DIFF s2-index     wide    light  dimension mismatch: 2880x3282 vs 2880x4082
  DIFF s2-index     wide    dark   dimension mismatch: 2880x3282 vs 2880x4082
  DIFF s1-decision  mobile  light  dimension mismatch:  780x5848 vs  780x5984
  DIFF s1-decision  mobile  dark   dimension mismatch:  780x5848 vs  780x5984
  DIFF s1-decision  laptop  light  dimension mismatch: 2048x3822 vs 2048x4530
  DIFF s1-decision  laptop  dark   dimension mismatch: 2048x3822 vs 2048x4530
  DIFF s1-decision  wide    light  dimension mismatch: 2880x3822 vs 2880x4530
  DIFF s1-decision  wide    dark   dimension mismatch: 2880x3822 vs 2880x4530

12 pairs · 0 within tolerance · 12 over
```

**The widths are identical at every viewport.** The container, the grid, the
measure ceilings, the breakpoints and both schemes all agree. Only the heights
differ, and every height difference is *content length*.

### The cause: the mockups are illustrations, not renders

`fixture-check.mjs` asserts titles, bodies and alternative fields byte-equal. It
never looked at the chain block or at the `.one` summary lines, and three classes
of string in the mockups have no source in the fixture ledger at all. I verified
each by grepping the fixture directory; all return nothing:

1. **The entire `.chain` / `.chain-stack` content is hand-abbreviated.**
   `"BEAM start-up in a hook latency path"`, `"✗ elixir/otp"`,
   `"slower to write, smaller contributor pool"`,
   `"Node start-up matches BEAM; Bun is young"` — none exist in any entry, and no
   derivation produces them from the `why_not` they summarise. The renderer emits
   the real option name and the real first sentence, which are longer.
2. **Every alternative's `.one` line is editorial.**
   `"free CI and free distribution, paid for in a latency path with no
   concurrency to supervise"` is not in the fixture. The renderer derives this
   line honestly (first sentence of `why_not`) but it is not that sentence.
3. **Execution status has no source and may not be given one.** `"converged"`,
   `"3/3 predicates green"`, the dial's `"43%"`, the index's `"in motion"` /
   `"to be planned"` are `dec-0004` joins with kazi. dira embeds no kazi client
   (`dec-0003`) and **E4 does not exist**. The served index shows the ledger-side
   buckets and states the join is unavailable — `dec-0004`'s own degradation
   rule, and one extra line of copy the mockup does not have.

A fourth, smaller one: the mockup's `.because` is a single lede paragraph, and
real entry bodies are multi-paragraph markdown. The renderer emits every
paragraph.

### What I refused to do about it

- **I did not fake the kazi join.** Printing "converged" without the join is dira
  asserting something it never checked, which `dec-0004` forbids and which
  `internal/why`'s package comment calls out by name.
- **I did not rewrite the mockups' chain block to match my output.** Deciding
  what the chain shows at length is `DESIGN.md`'s open question 1 (the chain at
  scale) and E6-L4's licence, not mine — and editing the reference until the gate
  passes is the failure mode this repo names most often. I removed only what you
  routed to me.
- **I did not gut `s2-index` of its roll-ups.** "No lane below redesigns
  anything."

### What would close it

Either the mockups become renders of the fixture (a content pass over the chain
and the `.one` lines, owned by whoever owns the design, and gated by extending
`fixture-check.mjs` to the chain block), or the clause is narrowed to what it can
honestly measure today — the mechanical gate plus a structural diff — until E4
lands. **My recommendation is the first**, done as a single content pass with
`uigate.mjs` in the loop, because until the mockups are reproducible the pixel
tolerance E6-L1 measured so carefully is guarding nothing.

---

## 7. The exact wiring to add

`cmd/dira/main.go` is yours, not mine — and it has moved since I started (a
`supersede` line landed). One line in `newApp`'s command slice, after `supersede`
and before `reindex`:

```go
{name: "ui", summary: uiSummary, run: runUI, usage: writeUIUsage},
```

Nothing else changes. `runUI`, `uiSummary` and `writeUIUsage` are in
`cmd/dira/ui.go`. Until that line lands:

- `golangci-lint run ./cmd/dira/...` reports 4 `unused` findings against `ui.go`
  — `uiUsagef`, `uiSummary`, `runUI`, `writeUIUsage`. All four disappear the
  moment the line is added. (`cmd/dira/sniff.go` and `brief.go` have the same
  shape from other lanes.)
- `docs/design/scripts/uigate.mjs` detects the missing subcommand, prints a
  `NOTE`, and runs through a temporary shim over the same `internal/ui` package,
  which it writes and deletes in the same run. Once `ui` is registered that
  branch is never taken. **If you ever see `internal/ui/uigate_shim/` in a diff,
  the script crashed — delete the directory.**

---

## 8. Contradictions and findings

**1. `cmd/dira/testdata/why/dec-0012.golden` moved, and here is the whole diff
before I accepted it.** That golden is the shared content contract between the
terminal and the web renderer, so it gets a section rather than a routine
`-update`. The entire change:

```
+  derived by   dec-0019  The upheld option is the ruling, not an alternative —
+                         the renderer derives, never invents
+                           dec-0012 settled that the surfaces are
+                           server-rendered templates; this settles what those
+                           templates are allowed to emit
```

The renderer's output for unchanged data is byte-identical. The golden moved
because `dec-0019` declares `derives_from → dec-0012`, which puts a backlink on
`dec-0012`'s chain — the golden working, not the renderer changing. Regenerated
with `go test ./cmd/dira -run TestWhy -update`; that subtest now passes.

**Two other goldens moved in the same `-update` run and I reverted them**, because
they are not mine: `int-0002.golden` and `daemon.golden` each gained a `dec-0020`
line (another lane's entry, `informs → int-0002`). `git checkout` restored them so
their owner kept the signal; they have since accepted them themselves, and
`go test ./cmd/dira` is now fully green. An `-update` that silently absorbs
someone else's ledger change is how a shared contract stops being shared.

**2. `dec-0016` is decided but not implemented, and my brief's premise is
currently false.** You told me "the fonts are embedded via `embed.FS` (dec-0016,
TeX Gyre Pagella subsets in `assets/fonts/`)". They are committed — three woff2
files, 60.6 KB — and **nothing references them**: `grep -rn "font-face"
docs/design/` returns nothing, and `tokens.css` still declares
`--serif: "Palatino", "Palatino Linotype", "Book Antiqua", Georgia, serif`. So
`dec-0016` has not landed in the CSS, and the Linux determinism problem
`DESIGN.md` describes is still live.

I deliberately did **not** fix it, because the fix must go in `tokens.css` — the
served copy is byte-pinned to it, so adding `@font-face` to only one side would
make the served page and the mockup use different fonts, which E6-L1 measured at
**up to 100% of pixels**. Whoever wires it must: add `@font-face` to
`tokens.css` pointing at `/assets/fonts/…`, copy the woff2 files under
`internal/ui/assets/fonts/`, add a route, and add them to `AssetSources` so the
existing drift test covers them. The `AssetSources` completeness test will fail
until the map is updated, which is the intended prompt.

**3. `internal/why/render.go` is being rewritten under me.** Mid-session it went
from 461 lines to 343 with the painter methods exported (`p.row` → `p.Row`). My
one change there — exporting `NoAlternatives` — is layered on top of that
in-flight work and may conflict.

**4. `docs/plan/tasks/E6-L2.md` does not exist.** The L2 planner prompt
(`docs/plan/prompts/L2-E6-L2.md`) was never executed, so I worked from the lane
`acc:` in `docs/plan/lanes/E6.md` as binding and treated the prompt's seven-task
sketch as guidance. That prompt also still cites
`docs/design/fixtures/ledger-design/` and `internal/ui/templates/decision.gohtml`
paths from before E6-L1's move; the lane file is corrected, the prompt is not.

**5. Two `dec-0005` boundary violations of mine, found by the coordinator and
fixed in place rather than allowlisted.** `TestNoFilesystemImportsAboveTheBackend`
caught both:

- `internal/ui/assets.go` imported `io/fs` for `fs.ReadFile(Assets, name)`. An
  `embed.FS` is a byte blob compiled into the binary, not the filesystem the rule
  protects — so the rule's *reasoning* does not reach it, but the mechanical check
  cannot tell the two apart. Rather than buy an allowlist entry, the call became
  `Assets.ReadFile(name)`, `embed.FS`'s own method, which needs no import at all.
  Spending the rule's credibility on a call that has a direct equivalent would be
  a bad trade.
- `cmd/dira/ui.go` imported `path/filepath` to derive the breadcrumb name from the
  ledger directory. That one is a real violation, and the allowlist says so
  explicitly: `cmd/dira` gets `os` and **not** `path/filepath`, because "locating
  a ledger on disk is a filesystem concern and belongs in the backend". So the
  derivation moved into the backend as `local.Name(diraDir)`, beside the existing
  `local.CacheDir(diraDir)` that `cmd/dira` already calls — same pattern, same
  reason, and E7's github backend will answer the question from the repository it
  talks to rather than from a directory. Covered by
  `internal/ledger/local/name_test.go`; both boundary tests pass.

**6. `schema/testdata/invalid/` has 18 files, not the 17 E6-L1 recorded.** All 18
are refused. Another lane added one; noted so the number in the fixtures README
is not read as a regression.

**7. The `s2-index` legend-key still glosses "converged" as "proven done by
tests".** I kept the mockup's copy verbatim, but the served index no longer shows
that word anywhere, because it cannot. It is a glossary entry for a concept the
page cannot currently display. Left as-is rather than edited, because changing it
widens the mockup/render gap in §6 rather than narrowing it — but it should be
revisited with E4.

---

## 9. What I did not do

- **`cmd/dira/main.go`, `docs/roadmap.md`, `docs/coverage.md`** — untouched, as
  instructed.
- **No commit, no push.**
- **E6-L3's `/distill` route** — out of scope; no write path exists in this
  package, and `internal/ui` has no handler that is not a GET.
- **A markdown renderer for entry bodies.** `internal/ui/view.go`'s `Para` type
  states the limitation rather than hiding it: bodies split on blank lines,
  a leading `#` becomes a bold lead-in, and lists and tables render as their
  literal source text. A CommonMark implementation is a dependency `int-0002`'s
  budget has no room for and `cmd/dira/build_test.go` would reject. It is
  cosmetic on the fixture ledger (single-paragraph bodies) and visible on the
  real one.
- **Nothing was added to `go.mod`.** The command path is still stdlib-only
  (`html/template`, `net/http`, `embed`), so `TestCommandPathLinksOnlyAllowedModules`
  is unaffected.
