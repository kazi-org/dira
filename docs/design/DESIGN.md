# dira — design system: "Bearing"

**Status:** direction locked 2026-07-29 (Phase 1 gate passed). Tokens locked.
Three screens built and gate-verified. Not yet implemented in product code.

---

## POV

> The chain tells you what the tool is. The prose tells you what was decided.

dira is a **navigation instrument, not a document viewer** — but its payload is
*prose reasoning*, and prose wants a serif and a measure. So the system is a
deliberate blend of two schools rather than a commitment to one:

- **The instrument grammar** (mono, the invocation line, the tree drawn in real
  selectable type) carries **the graph** — what relates to what, and how it resolved.
- **The typographic calm** (serif at a strict measure, refusals struck through with
  their grounds beneath) carries **the reasoning** — the payload a human actually reads.

**Canon lenses:** grug for the instrument half (total commitment; the bit never
breaks — the invocation line is real, not decorative), Craft for the prose half
(strict content width, chrome suppression, saturation reserved for content).

**Signature detail:** the **why-chain** — the decision graph rendered as
box-drawing characters that are *text*, so it is selectable, copyable, and
diff-able. It is the same output `dira why` prints in a terminal. Nothing is a
graphic that could be type.

**The 3-second interaction:** you land on a decision page from a link, read
`$ dira why elixir`, and the chain tells you what the tool does before you have
read a single sentence of prose.

---

## Direction rejected, and why

Three anchors were rendered and compared at 1440×900 in both schemes
(`renders/r1-*`). Recording the losers because the reasoning is reusable:

| Anchor | Lens | Why not |
|---|---|---|
| **A1 Chart Room** | Tide Guide — data made beautiful through restraint | The bearing dial is distinctive, but on a *decision* page it decorates rather than informs: the needle read as "12 o'clock", not as a bearing. **The dial was not discarded — it moved to s2-index**, where roll-ups are genuinely the content. |
| **A3 Instrument Terminal, straight** | grug — total commitment | The why-chain and invocation line were the best acquisition device of the three and were kept. Mono as the *display* face lost: the title set two cramped lines and a paragraph in mono is a wall. |
| **A2 Court Record, straight** | Craft + Things 3 | The most beautiful and most readable, and its struck-through refusals were kept wholesale. Lost alone because it shows no status and no graph — a stranger cannot tell there is a system behind the essay, which makes it the weakest acquisition surface. |

---

## Three checkable laws (this system's own)

Beyond TASTE.md and WEB.md, these are dira-specific and mechanically checkable:

1. **Red means the compass caught something. Nothing else.**
   `--caught` may only be used by drift, contradiction, and `dira check` failures.
   A rejected alternative is **a record, not an alarm**, and renders neutral
   (`--ink-low`). A **withheld** parent is likewise not an alarm — of the three
   resolution states, only `orphan` is drift.
   *Grep test:* `--caught` appears only in `.drift`, `.res-orphan`, `.st-orphan`
   and check states.
2. **The because outranks the title.**
   On any surface where a human disposes of an entry, the `because` is set larger
   than the title. What is being approved is the reasoning, not the label.
   *Measured test:* `.stage .because` font-size > `.stage .titleline` font-size.
3. **The graph is type, never a picture.**
   The why-chain is selectable text at every viewport. It may re-form (the mobile
   `.chain-stack` is a stacked list rather than a clipped tree) but it may never
   become an image, a canvas, or an SVG of glyphs.
   *Test:* select-all on any chain yields the ids and reasons as text.

   **Amended 2026-07-30, and the amendment narrows the claim.** The vertical rules
   are now a CSS `border-left`, not box-drawing glyphs. Every id, reason, mark and
   status remains selectable text; the *rule* does not, because a rule is not
   content — it is punctuation for a fixed-width medium, and a border is the honest
   way to draw it in one that is not.

   Why it had to change: at `line-height: 1.9` a 13px box-drawing glyph sat in a
   24.7px line box, leaving ~12px of dead space between every pair of rows. The
   column rendered as a **dashed run of tick marks**. In a real terminal those rules
   are unbroken, so the element was violating the byte-identity it claimed, visibly,
   at 4x zoom and invisibly at 1x.

   **The cost, stated rather than buried.** A reviewer argued that byte-identity is
   the only thing keeping this element on the right side of the retro-terminal
   cliché — that it is not a *reference to* a terminal but an *artifact of* one, and
   the moment it stops being byte-identical it becomes styling with no terminal
   behind it. That argument is real and this amendment weakens it. What survives is
   narrower and still true: copy the chain and you get the ids and the reasoning,
   which is the content. What is lost is that the copied text no longer reproduces
   the tree drawing character-for-character.

   The trade was taken because the alternative was shipping an element that renders
   broken, and because a nested structure takes `<details>`/`<summary>` for
   collapse-at-depth with no JavaScript — which answers the chain-at-scale question
   that had no answer.

---

## Recorded law breaks

| Law | Break | Reason |
|---|---|---|
| TASTE 5 — "display face never below ~24pt or in chrome" | The serif is used at 17.5px for alternative names and 16.5px for grounds | The alternatives *are* the prose payload, not chrome. Setting them in the UI sans made the page read as a form; the whole point of the Court Record half is that refusals read as testimony. |
| TASTE 11 — density bimodality, one mode per screen | s2-index is DENSE (tree + roll-ups + dial); s1-decision and s3-distill are CALM | Each screen commits to one mode; the *system* spans both. This is the endorsed reading of the law, not a break of it — noted here only because a reviewer comparing screens side by side will see the shift. |

---

## Tokens

`tokens.css` is the single source. A hardcoded hex or px in a screen is a defect,
with exactly one sanctioned exception: `<meta name="theme-color">` cannot reference a
CSS custom property, so each screen repeats the two ground values literally. If a
ground token changes, those two meta tags change with it — the one place the token
system cannot enforce itself.

*Test:* `grep -oE '#[0-9a-fA-F]{3,6}' screens/*.html` must return only `#0f151c` and
`#f7f4ed`.

**Hues — three, each with exactly one meaning.** A fourth requires a documented role.

| Token | Meaning | Light | Dark |
|---|---|---|---|
| `--bearing` | the instrument: current focus, brand, links, active intents — **and the `withheld` resolution state** (one documented exception, below) | `#8a5f18` | `#c99a3e` |
| `--bearing-lift` | hover/focus — deliberately *more* contrast than rest (WEB.md 2). **Inverts direction per scheme:** darker in light, lighter in dark | `#6d4a12` | `#e2b95f` |
| `--converged` | runtime state only: accepted, converged, achieved | `#1f6d5b` | `#45a189` |
| `--caught` | **drift and contradiction only** | `#a83828` | `#d97060` |

All 42 fg/bg pairs clear 4.5:1 in both schemes and hover exceeds rest on every
surface — 0 failures. The values above are the post-fix ones, not the originals
(see r3 → r4 below).

**The one hue-budget exception, and it is a break rather than a widening.**
`--bearing` carries a second meaning: `withheld`. The rule is one meaning per
hue and this breaks it. Recorded here rather than quietly redefined.

The defence is that withheld is the instrument pointing at something it can name
and cannot open — the needle is still on it. The alternatives were all worse:
`--caught` is forbidden (withheld is not drift, Law 1), `--converged` would claim
a resolution that did not happen, and a fourth hue would have to earn a permanent
slot for a state whose entire design brief is to be *unremarkable*. The cost,
stated rather than buried: on `s1-decision-withheld.html` the page carries
bearing in four roles at once — the invocation argument, the id chips, the links,
and the withheld mark. A reviewer will notice that, and should.

**Two matrices, because one was not enough.**
`node docs/design/scripts/contrast.mjs` checks token against token. That is necessary
and it is NOT sufficient, and the gap shipped: every chip sits on a `color-mix` tint of
its own colour, so the pair that actually renders is fg-on-tint, never fg-on-surface.
The token matrix reported "0 failures" while five chips rendered between 3.0:1 and
4.3:1 in light.

`node docs/design/scripts/contrast-rendered.mjs` measures every text node **as
composited in a real browser**, walking transparency up the ancestor chain. It is the
authority, for two reasons found the hard way: `color-mix(in oklab, ...)` cannot be
approximated in sRGB (an sRGB estimate passed pairs the browser fails), and colour
resolution must be done by painting rather than string-parsing, because computed
`color-mix` values come back as `oklab(...)` and reading those floats as RGB reported a
legible paragraph at 1.31:1.

Content marked `aria-hidden="true"` is exempt: decorative text carries no information,
so there is nothing to fail to read. It must be *marked* decorative, not assumed to be. It parses `tokens.css` rather than
carrying its own palette, so it cannot drift from the tokens it checks — and it is
verified to catch the original defect: restoring the pre-r4 `--bearing-lift`
(`#b8862f`) produces 6 failures including the hover-inversion. Until this script
existed, this document required a test that did not exist.

Surfaces are hued and never pure — light ground `#f7f4ed`, dark ground `#0f151c`.
No `#000`, no `#fff`, in either scheme. `color-scheme: light dark` is declared so
native scrollbars and form controls follow, and `<meta name="theme-color">` is set
per scheme so browser chrome matches.

**Type — two families plus mono.** `--serif` (Palatino stack) for display and prose;
`--ui` (system-ui) for chrome and labels, never display; `--mono` (SF Mono/Menlo)
for ids, the chain, and numerics. `font-variant-numeric: tabular-nums` is set on
`body`, so every roll-up and count is monospaced-digit by default.

**Why system fonts — and where that reasoning currently FAILS.** dira embeds its UI in
a Go binary that must work with the network unplugged (`int-0002`, `cst-0004`), so a
*network* webfont is out. That much holds.

**The serif stack does not deliver what this paragraph used to claim.**
`"Palatino", "Palatino Linotype", "Book Antiqua", Georgia, serif` resolves on macOS and
mostly on Windows, and on a stock Linux install resolves to **none of them** — it falls
through to DejaVu Serif, whose metrics are nothing like Palatino's. A git-native
developer tool has a large Linux audience. Every type ratio, measure and vertical rhythm
in this system was tuned by rendering on macOS with Palatino loaded.

That is the *same* failure this constraint was adopted to eliminate — an earlier pass
rendered with its intended faces silently falling back, making every spacing judgement
wrong — relocated to where the render harness structurally cannot catch it, because the
harness runs on macOS. A gate that cannot see the failure is not evidence against it.

No serif ships by default across macOS, Windows and Linux, so **no stack can fix this**;
only self-hosting can. And self-hosting is compatible with the offline constraint —
`embed.FS` puts a subsetted woff2 (~30-80KB) inside the binary with no network call. The
original trade ("bloat for zero functional gain") was miscounted: the gain is
cross-platform typographic determinism, which is the thing the constraint existed to buy.

**Open, and it is a real decision, not a cleanup:** self-host one serif, or accept that
a third of the audience sees a different design from the one that was reviewed. Recorded
rather than quietly fixed because it changes a documented constraint.

**Scale:** spacing `4 / 8 / 12 / 18 / 28 / 40 / 60 / 96`. Radii `14 / 7 / 4`,
concentric — inner ≈ half outer, so curves align.

**Motion:** two curves (`--ease`, `--ease-out`), `--dur: .28s`. No
`transition: all` anywhere; only `transform`, `opacity`, `background`, and
`border-color` are animated. `prefers-reduced-motion` collapses durations, and the
visible end state is the base style, so print, reduced-motion, and no-JS all show
real content rather than a pre-animation `opacity: 0`.

---

## Screens

| File | Is | Density | Notes |
|---|---|---|---|
| `screens/s1-decision.html` | the page a stranger lands on from a link | CALM | invocation + chain, then the ruling and its grounds. Three alternatives, so every one is open. Mobile swaps the tree for `.chain-stack`. |
| `screens/s1-decision-long.html` | the same page at 19 alternatives, one of them 405 words | CALM | the long-content case, and the reason it is a file rather than a note: WEB.md 9 requires it be *checked*, and a fixture in `screens/` is checked by all four gates automatically. |
| `screens/s1-decision-withheld.html` | the same page when the chain leaves the repo | CALM | `dec-0011`. The withheld state, shown adjacent to oriented and orphan because it can only be judged against the alarm it must not resemble. |
| `screens/s2-index.html` | the ledger index — **groups by why, not by goal** | DENSE | the dial lives here. Status is derived, never stored (`dec-0004`). The drift row is the only red. |
| `screens/s3-distill.html` | the daily habit — agents propose, you dispose | CALM | the because is the hero. Desktop shows keyboard hints; mobile becomes the swipe deck with full-width thumb targets. |

The three `s1-*` screens are one page with three payloads, so their layout lives
in `screens/decision.css` rather than three times in three `<style>` blocks.
That file consumes `tokens.css` and adds no colours of its own.

Attribution appears on every rendered page as one quiet line, never a badge:
*"Decision record kept with dira · written by an agent, read by one."* It is the
acquisition loop (`dec-0010`) and it must stay a sentence.

---

## Verification

`scripts/render.mjs` captures every screen at **3 viewports × 2 schemes** and runs
a mechanical gate that fails the round before any judgement:

- console errors, page errors, or failed requests
- near-blank capture (a mount that never rendered)
- **byte-identical light/dark pair** — proves the dark scheme is real, not a token
  that forgot to be scheme-aware
- layout shift (body height compared at 400 ms and 1500 ms)

```
node docs/design/scripts/render.mjs r4            # everything
node docs/design/scripts/render.mjs r4 s2-        # one screen
```

Playwright is not a repo dependency — it lives in the session scratchpad and is
symlinked in as `node_modules` (gitignored), so the repo stays free of a Node
toolchain it does not otherwise need.

### One command

```
node docs/design/scripts/gates.mjs          # every gate, plus every negative control
node docs/design/scripts/gates.mjs --fast   # skip the browser captures
node docs/design/scripts/gates.mjs --list   # what runs, and what each one proves
```

Four gates and two negative controls, run together. This document requires the
contrast matrix be re-run "whenever a colour token moves"; a requirement whose
invocation nobody remembers is a requirement that stops being met at the first
busy moment. `gates.mjs` is that invocation.

Every gate that has a negative control runs it in the same pass, and a control
that **fails to trip** is reported as `BLIND` with its own exit code (3), not as
a pass — a checker that cannot fail is indistinguishable from one that always
prints "ok".

| gate | proves | negative control |
|---|---|---|
| `pixeldiff.mjs --self-test` | the comparator's own arithmetic | 13 assertions, each with a known-bad counterpart |
| `contrast.mjs` | 42 token pairs clear 4.5:1; hover exceeds rest on all six surface × scheme combinations | `--probe-regression` restores the pre-r4 `--bearing-lift` `#b8862f` and must report both a floor violation and a hover inversion |
| `contrast-rendered.mjs` | every text node clears its floor **as composited**, including `color-mix` tints | — |
| `tokens-doc-sync.mjs` | this document agrees with `tokens.css` value for value, and states the measured tolerance | `--design` / `--tokens` accept copies, so the checker can be tested without editing what it guards |
| `render.mjs` | 3 viewports × 2 schemes, plus **no asset from any host but `127.0.0.1`** | `--probe-external` serves a stylesheet from this machine's LAN address; it returns 200, so the failed-request check stays silent and the loopback check must not |

### The pixel tolerance

**`0.00055%` of pixels, at a channel threshold of `0/255`, with no 16×16 block
more than `2.5%` changed.** A comparison fails if either percentage is exceeded,
or if the two captures differ in size at all.

The channel threshold is **zero**: any per-channel difference of any magnitude
counts. There is no filtered band, so there is no class of change this gate is
blind to by construction.

Measured, not chosen. `docs/design/fidelity/TOLERANCE.md` carries the method and
the evidence; `tolerance.json` carries the numbers and `pixeldiff.mjs` reads them
at run time, so the figure published here and the figure enforced are the same
figure.

```
node docs/design/scripts/pixeldiff.mjs <a.png> <b.png> [--out diff.png]
node docs/design/scripts/measure-tolerance.mjs --write     # re-derive it
```

The short version of the evidence: across 3 screens × 2 viewports × 2 schemes,
**all 60 noise measurements were bit-identical** — a second screenshot of the
same page, a fresh browser context, a second Chromium process, a different
origin, and markup round-tripped through the DOM serializer all produced zero
differing pixels. The tolerance is therefore not absorbing observed variance;
there is none. It sits a factor of four below the *smallest real defect that
could be constructed* — a 2px card-radius change, which moves 0.002214% of pixels.

Two percentages rather than one because legitimate variance is **diffuse** (a
scatter along glyph edges) and a real regression is **clustered** (one element,
gone wrong). A frame-wide percentage is area-weighted, so a tolerance loose
enough for diffuse noise is loose enough to hide a missing card; the block figure
is scale-free and catches that.

**Why the channel threshold is 0 and not something comfortable.** An earlier
version of this gate filtered differences of ≤4/255, on the reasoning that the
largest threshold the signal could survive left the gate "as robust as the
evidence allows". That was backwards: a larger threshold makes the gate *less*
sensitive, and the only thing it buys is immunity to per-pixel noise that was
measured at exactly zero. The rule is now the smallest threshold that filters all
*measured* noise — zero here.

The cost of the old one was not theoretical. Two defects that are quiet per pixel
and large in area — a token hex off by 2/255, and a stepped opacity off by one
hundredth — moved **0.50%** and **0.77%** of the page respectively, roughly a
thousand times the tolerance, and at 4/255 both registered **0.000000%**. The
opacity case is caught by no other gate in this repo, since `contrast.mjs` and
`tokens-doc-sync.mjs` read declared hex values and never a rule's opacity.

**This tolerance is not a cross-machine allowance.** Changing only glyph
rasterization costs 1.23–4.64% of pixels, and letting the Palatino stack fall
through to a generic serif — what a stock Linux install actually renders, the
failure noted above — costs up to 100%. Both are three to four orders of
magnitude above the tolerance, and no number that still catches a 2px radius
change could ever absorb them. So the baseline is regenerated in the same run and
the same environment as the capture, which is why `docs/design/renders/` is
gitignored: a hand-committed baseline compared across machines is the one thing
this gate cannot survive.

### Defects caught by the loop (kept as a record)

- **r1 → r2:** red was doing double duty — marking rejected alternatives *and*
  flagging drift. Two meanings for one hue (TASTE 1). Refusals went neutral.
- **r2 → r3:** the dial needle crossed the centre readout; shortened to ride
  inside the ring band.
- **r2 → r3:** the mobile swipe hint never rendered — the media query was declared
  *before* the base `display: none`, so source order let the base rule win at every
  width. A rule that silently loses is invisible in review and only shows in a render.
- **r2 → r3:** the empty state sat below three populated cards, reading as though
  the queue were empty. Removed; its copy is recorded below instead.

**Empty-state copy** (one sentence, zero filled buttons, per TASTE 14):
> When the queue is empty, nothing is waiting on you — and the ledger is current.

### r3 → r4: the fresh-eyes revision

A fresh-eyes pass returned `VERDICT: revise`. Acted on, with one rejection.

**Contrast was a genuine token-level defect, and worse than first reported.**
Computed WCAG ratios for all 42 fg/bg pairs across both schemes found seven
failures, not the three flagged. The serious one was missed entirely:
`--bearing-lift` scored 2.95 / 3.18 / 2.62 in light — and it is the *hover/focus*
token, which WEB.md 2 requires to have **more** contrast than rest. It had less.
The law was inverted.

Root cause was structural, not a bad swatch: **a single "lift" direction cannot
serve both schemes.** More contrast means *darker* in light and *lighter* in dark.
`--bearing-lift` now inverts per scheme. New values clear 4.5:1 on every surface
and hover exceeds rest on all six surface/scheme combinations — 0 failures.

*Test:* the contrast matrix must report 0 failures and `hover > rest` everywhere.
Re-run it whenever a colour token moves.

**The jargon gate and the missing conversion path were real, and were defects
against `dec-0010`.** If the index is the acquisition surface, then a stranger who
cannot decode `int-`/`dec-`/`qst-` and has no way to obtain the tool means the
growth loop never closes. Added: a permanently-visible legend (not a hover
tooltip — hover is never the only affordance, WEB.md 5; and not an onboarding
carousel, which TASTE 14 bans), and one restrained conversion block. A visitor is
not a user already inside the app, so an affordance here is legitimate rather than
the "Get started on a screen you're already in" slop tell.

**The density diagnosis was right; its proposed fix was rejected.** s1-decision was
correctly identified as sitting in the medium-density/medium-contrast slop zone
(TASTE 11) with nothing anchoring the frame. But the suggested fix — collapse the
alternatives into a scannable comparison list — would destroy the struck-through
refusal with its grounds beneath, which is the strongest device in the system and
the reason this direction was chosen over the terminal anchor. Resolved instead by
*reducing element count and strengthening the anchor*: the display went to 52px
(3.15:1 against the 16.5px grounds, clearing the 3:1 floor so one element clearly
wins), and the duplicate "This ledger" stats card was removed from the rail — its
count moved to the footer, where it costs no block.

**Also noted and left as-is at r4, and removed later:** the upheld alternative
carried a coloured left border while refusals carry a hairline. It was flagged as
adjacent to the "left-border accent stripe" slop tell and kept because it was
state-driven rather than decorative, with the state *also* carried by a text tag
and the strike-through (WEB.md 12). That defence needed a state to carry.
`dec-0019` found there is none in the data — `alternatives` records only the roads
not taken — so the card, its border and `.alt.upheld` are gone. Recorded here so a
future reviewer does not re-litigate either direction.

---

## Long content, and the withheld state — both answered 2026-07-30

Two of the three open questions below were answered from rendered comparisons
(`openq/`, six options across two questions). The founder chose from the
pictures; what follows is what shipped, not what was proposed.

### Long content — the summary/detail split

**Chosen: option C.** Every alternative is a `<details>`. The summary carries the
mark, the name, the state tag and **one line of ground** — enough to decide
whether to open it. The full reasoning is inside. Zero JavaScript, the same
collapse mechanism the rebuilt chain uses.

The device this was most at risk of destroying survives, and that was the
condition of choosing it. `r3 → r4` had already rejected "collapse the
alternatives into a scannable comparison list" precisely because it kills the
struck-through refusal with its grounds beneath. The strike lives on the
**summary** line, which is always visible: a refused alternative reads as refused
whether the entry is open or closed, and its grounds are one keystroke away
rather than gone.

**The degradation rule — decided at render time, not in CSS:**

| alternatives | emitted |
|---|---|
| ≤ 6 | every `<details open>`. Nothing is hidden; the page is the argument it always was, with a hinge added. |
| > 6 | every `<details>` closed. The page becomes an index that expands in place. |

**Amended 2026-07-30 by `dec-0019`, and the amendment removes an element rather
than adding one.** The `> 6` row used to read *"only the upheld one open"*. There
is no upheld one. `entry.schema.json` models `alternatives` as the roads **not**
taken — `why_not` is required — so the chosen option has no field to live in, and
the fourth card `s1-decision.html` carried ("Go — one static binary, sub-100 ms
cold start") was unproducible from any ledger. It is gone from all three `s1-*`
payloads, and the chain's matching `✓` row with it, because `dira why` prints `✗`
lines and never a `✓` and one producer stands behind both renderers.

*What survives, because it was the condition of the change:* the struck refusal
with its grounds beneath. Every refusal keeps its strike and its one-line ground
on the always-visible summary. The counterweight a strike needs is unstruck type,
and the nearest unstruck type is now the `h1.ruling` at up to 52px — the anchor
r3 → r4 chose *instead of* the comparison list, at a 3.15:1 ratio against the
grounds. Removing a 16.5px card that restated that anchor is the same move r3 → r4
made when it deleted the duplicate stats card: reduce element count, strengthen
the anchor.

*What has no field either, and is derived rather than stored:* the one-line ground
`dec-0017` makes load-bearing. It is the **first sentence of `why_not`**, and the
detail is the rest. The rule the renderer follows is *derive presentation from
recorded prose; never invent a field; where nothing can be derived, render
nothing.*

Six, because the long-content study measured the reading thread breaking at
roughly the sixth refusal — before that the rhythm still reads as testimony;
after it, as a transcript. CSS cannot count siblings honestly, so this belongs to
the Go renderer, and it is one comparison.

*Test:* `s1-decision.html` (3, all open) and `s1-decision-long.html` (19, all
closed) are both in `screens/`, so both pass through the render gate and the
as-composited contrast gate on every run. `fixture-check.mjs` additionally
refuses any screen that reintroduces an `upheld` tag or a `✓` mark in an
alternatives or chain block.

### The withheld state — a declared state

**Chosen: option B**, with two amendments applied at fold-in.

Withheld is a first-class resolution state: the mark `⊙`, the name, and
`.chip-withheld`, all in `--bearing`. Something that looks *designed* is hard to
read as broken — which is the whole requirement, since withheld must read as
neither an error nor a warning.

The three states are now first-class in `tokens.css` as `.res-oriented`,
`.res-withheld`, `.res-orphan`. **Only orphan is drift, and only orphan may use
`--caught`.** Withheld is drawn at *chip* weight and never gets a tinted card
surface; orphan is the only state that recolours the ground it sits on.

**Amendment 1 — the mark stays `⊙` in `--bearing`.**

**Amendment 2 — the duplicated prose is gone.** The chain row read

```
sire:int-0002  private ledger — ref published, body not      ⊙ withheld
```

which states the same fact twice: the mark already says the body is not here. It
is now

```
sire:int-0002  private ledger                                ⊙ withheld
```

and the plain-language explanation lives **once**, in the `Arising from … — a
parent in a ledger this repo declares but cannot show you.` line.

---

## Open design questions

1. **The chain at scale.** Eight lines is legible; a 40-entry intent with a deep
   supersession chain is not. Partly answered — `s1-decision-long.html` shows the
   chain listing four refusals and counting the remaining sixteen — but the rule
   for *depth* (a long supersession chain) is still unwritten.
2. ~~**Long content.**~~ Answered above.
3. ~~**The withheld state needs designing.**~~ Answered above.
4. **s2-index has nowhere to put a cross-boundary entry.** Surfaced by folding
   the withheld state in, not previously recorded. The index groups by intent,
   and `dec-0011`'s parent is `sire:int-0002` — an intent in a private ledger,
   which by construction cannot be a group heading in this repo. So
   `s1-decision-withheld.html` is currently reachable only by direct link. Either
   the index grows a "from another ledger" group, or cross-boundary entries are
   accepted as index-invisible, which weakens the acquisition surface (`dec-0010`).

---

## Hand-off

- Production hardening → `/web-craft`, plus Vercel's `web-interface-guidelines`
  skill (**not currently installed** — install before implementation; it is the
  authority for the implementation layer).
- Audit the built result → `/ui-review` (pixels) and `/ux-review` (experience).
- Theme variants on this kit → `/skin-gallery`.
