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
   (`--ink-low`). *Grep test:* `--caught` appears only in `.drift` and check states.
2. **The because outranks the title.**
   On any surface where a human disposes of an entry, the `because` is set larger
   than the title. What is being approved is the reasoning, not the label.
   *Measured test:* `.stage .because` font-size > `.stage .titleline` font-size.
3. **The graph is type, never a picture.**
   The why-chain is selectable text at every viewport. It may re-form (the mobile
   `.chain-stack` is a stacked list rather than a clipped tree) but it may never
   become an image, a canvas, or an SVG of glyphs.
   *Test:* select-all on any chain yields the ids and reasons as text.

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
| `--bearing` | the instrument: current focus, brand, links, active intents | `#8a5f18` | `#c99a3e` |
| `--bearing-lift` | hover/focus — deliberately *more* contrast than rest (WEB.md 2). **Inverts direction per scheme:** darker in light, lighter in dark | `#6d4a12` | `#e2b95f` |
| `--converged` | runtime state only: accepted, converged, achieved | `#1f6d5b` | `#45a189` |
| `--caught` | **drift and contradiction only** | `#a83828` | `#d97060` |

All 42 fg/bg pairs clear 4.5:1 in both schemes and hover exceeds rest on every
surface — 0 failures. Re-run the matrix whenever a colour token moves; the values
above are the post-fix ones, not the originals (see r3 → r4 below).

Surfaces are hued and never pure — light ground `#f7f4ed`, dark ground `#0f151c`.
No `#000`, no `#fff`, in either scheme. `color-scheme: light dark` is declared so
native scrollbars and form controls follow, and `<meta name="theme-color">` is set
per scheme so browser chrome matches.

**Type — two families plus mono.** `--serif` (Palatino stack) for display and prose;
`--ui` (system-ui) for chrome and labels, never display; `--mono` (SF Mono/Menlo)
for ids, the chain, and numerics. `font-variant-numeric: tabular-nums` is set on
`body`, so every roll-up and count is monospaced-digit by default.

**Why system fonts, deliberately, not a webfont.** dira embeds its SPA in a Go
binary and must work with the network unplugged (`int-0002`, `cst-0004`). Embedding
webfonts would bloat the binary for zero functional gain. This also removes a real
failure mode: an earlier design pass rendered with its intended faces silently
falling back, which makes every spacing and measure judgement wrong. The renders
here use the same faces that ship.

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
| `screens/s1-decision.html` | the page a stranger lands on from a link | CALM | invocation + chain, then the ruling and its grounds. Mobile swaps the tree for `.chain-stack`. |
| `screens/s2-index.html` | the ledger index — **groups by why, not by goal** | DENSE | the dial lives here. Status is derived, never stored (`dec-0004`). The drift row is the only red. |
| `screens/s3-distill.html` | the daily habit — agents propose, you dispose | CALM | the because is the hero. Desktop shows keyboard hints; mobile becomes the swipe deck with full-width thumb targets. |

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

**Also noted and left as-is:** the upheld alternative carries a coloured left
border while refusals carry a hairline. Flagged as adjacent to the "left-border
accent stripe" slop tell. Kept because it is state-driven rather than decorative,
and the state is *also* carried by a text tag and the strike-through, so it
satisfies WEB.md 12. Recorded here so a future reviewer does not re-litigate it.

---

## Open design questions

1. **The chain at scale.** Eight lines is legible; a 40-entry intent with a deep
   supersession chain is not. Needs a collapse rule before the renderer ships.
2. **Long content.** Screens are verified against short, average, and the real
   entry text — but not against a 400-word `why_not` or a 20-alternative decision.
   WEB.md 9 requires that check.
3. **The withheld state needs designing** — `qst-0001` is now answered by
   `dec-0011`, so the model is settled and only the visual is open. Resolution reports
   three states: **oriented**, **withheld** (parent declared private, not readable
   here), **orphan** (no parent → drift). Only orphan is drift. Withheld is a
   legitimate designed state and must read as neither an error nor a warning — it is
   currently the only state with no token or treatment assigned.

---

## Hand-off

- Production hardening → `/web-craft`, plus Vercel's `web-interface-guidelines`
  skill (**not currently installed** — install before implementation; it is the
  authority for the implementation layer).
- Audit the built result → `/ui-review` (pixels) and `/ux-review` (experience).
- Theme variants on this kit → `/skin-gallery`.
