# Serif bake-off — which face dira self-hosts

**Status:** evidence only. No winner picked, nothing under `screens/`, `landing/` or
`tokens.css` was touched, nothing committed.
**Run:** 2026-07-30. Built from a frozen snapshot of `s1-decision.html` sha1 `be7001a3`
and `tokens.css` sha1 `08b175f9` — **both now byte-identical to `a5d5e8d`**, the current
tip, so these renders are of the design as it actually stands.
**Everything below is measured.** Where a number is a prediction it says so, and the
prediction was then re-measured.

---

## The short version

1. **The brief's licence premise for candidate 1 is wrong, and it matters.** URW P052 is
   not "GPL with a font exception". It is **AGPL-3.0**, and its exception covers
   **PostScript and PDF documents only** — not a woff2 embedded in a binary that serves
   a web UI. Details and quoted text below. There is no "URW Palladio" on Google Fonts
   either; I checked (`ofl/palladio`, `ofl/p052`, `ofl/palatino`, `ofl/pagella` all 404).
2. **The Palatino-metrics bet is still available, from a different source.** **TeX Gyre
   Pagella** (GUST Font Licence / LPPL 1.3c) is a Palatino metric clone with the same
   prose advance widths as P052, a *closer* x-height to real Palatino than P052 has, a
   smaller subsetted payload, and a licence that is fine in an Apache-2.0 repo. It is
   rendered here as a fourth full page so this is not an assertion.
3. **"P052 preserves the ratios, the OFL faces will not" is half right.** P052 and
   Pagella preserve them exactly. **Source Serif 4 also preserves them exactly** — same
   characters-per-line, same line count, same page height — which the brief did not
   expect. Only Newsreader and Literata actually move the measure.
4. **Payload is not a differentiator.** All five candidates land within 7.5 KB of each
   other (62–70 KB for regular + italic + bold). Size will not decide this.

---

## 1. Payload — actual subsetted woff2 bytes

Subset: Basic Latin + Latin-1 Supplement + the punctuation this design's prose really
uses (curly quotes, en/em dash, ellipsis, bullet, primes, guillemets, ×, −, ≤ ≥, arrows).
Layout features kept: `kern liga calt ccmp locl onum lnum tnum pnum frac case mark mkmk`.

**Box-drawing was checked and is not needed.** The current screens contain **zero**
U+2500–257F — the chain was rebuilt on 2026-07-30 to draw its rules with CSS borders
instead of glyphs. (Minor, noted in passing: `DESIGN.md` records that change at line 68,
but the POV paragraph at line 25 still calls the chain "box-drawing characters that are
*text*". One of the two is now stale. Not mine to fix.)

Three faces per candidate: regular 400, italic 400, and the bold that serves the
`font-weight: 600` on `.arg.upheld .name`.

| Candidate | regular | italic | bold | **total (core)** | total (+ Latin Ext-A) |
|---|---:|---:|---:|---:|---:|
| TeX Gyre Pagella | 20,380 | 21,456 | 20,220 | **62,056 B (60.6 KB)** | 80,272 B |
| Source Serif 4 | 21,348 | 21,512 | 22,744 | **65,604 B (64.1 KB)** | 78,264 B |
| Literata | 21,372 | 21,888 | 22,384 | **65,644 B (64.1 KB)** | 79,060 B |
| URW P052 | 22,388 | 23,716 | 22,684 | **68,788 B (67.2 KB)** | 85,744 B |
| Newsreader | 22,116 | 24,008 | 23,408 | **69,532 B (67.9 KB)** | 90,808 B |

Spread from best to worst: **7,476 bytes.** Against `int-0002`'s "binary must stay
small", a 7 KB difference is not a decision input. The *ext* column is the option worth
thinking about: +14–21 KB buys Latin Extended-A, i.e. European names in real decision
records rendering correctly rather than tofu.

**Coverage gaps found while subsetting** (these are real, not rounding):

| Candidate | Missing from the intended set |
|---|---|
| Pagella | none |
| Source Serif 4 | U+2011 (non-breaking hyphen) |
| Literata | U+2011, U+2197 ↗, U+2198 ↘ |
| P052 | U+2010 (hyphen), U+2011 |
| **Newsreader** | U+2011 **and every arrow** — U+2190 ← U+2191 ↑ U+2192 → U+2193 ↓ U+2197 ↗ U+2198 ↘ |

Newsreader has no arrows at all. The design uses → today ("Keep your own →"), currently
in a `--ui` sans context, so nothing breaks right now. But dira renders *arbitrary*
agent-written prose, and a serif that cannot set → is a standing constraint.

---

## 2. Licences — plainly, and with the text quoted

### URW P052 — **AGPL-3.0 with a PostScript/PDF-only exception. Do not ship this.**

The exception, quoted in full from `fonts/LICENSE` in `ArtifexSoftware/urw-base35-fonts`
(copy at `docs/design/bakeoff/licences/urw-LICENSE.txt`):

> The font and related files in this directory are distributed under the
> GNU AFFERO GENERAL PUBLIC LICENSE Version 3 (see the file COPYING), with
> the following exemption:
>
> As a special exception, permission is granted to include these font
> programs in a **Postscript or PDF file** that consists of a document that
> contains text to be displayed or printed using this font, regardless
> of the conditions or license applying to the document itself.

(emphasis mine). Debian's independent record for `fonts-urw-base35` says the same thing —
`License: AGPL-3 with Font exception`, same exception paragraph.

Read it against what dira would actually do: embed the font in a **Go binary** via
`embed.FS` and serve it from `dira ui` over HTTP. That is not a PostScript file and not
a PDF file, so **the exception does not reach it.** What is left is bare AGPL-3.0, and
AGPL §13 is triggered by exactly the thing dira does — offering interaction with users
over a network. The risk is that shipping P052 this way argues dira's own source must be
offered under AGPL, which contradicts the Apache-2.0 repo.

I also checked whether an older, more permissive URW release exists. It does not help:
Debian's `gsfonts` (the pre-Artifex URW++ set) is recorded as plain **GPL-2 with no font
exception at all**. The URW exception has been PS/PDF-only for its whole life.

I am not a lawyer and this is not legal advice. But "needs care" understates it — this
is a licence whose exception, on its face, does not cover the intended use. If the
founder wants Palatino metrics, the answer is Pagella, not a judgement call about P052.

### TeX Gyre Pagella — **GUST Font Licence = LPPL 1.3c. Fine, with two chores.**

Quoted in full (`licences/gust-font-license.txt`):

> This work may be distributed and/or modified under the conditions
> of the LaTeX Project Public License, either version 1.3c of this
> license or (at your option) any later version.
>
> Please also observe the following clause:
> 1) it is **requested, but not legally required**, that derived works be
>    distributed only after changing the names of the fonts comprising this
>    work …

LPPL is not a copyleft on surrounding software — it constrains the font files, not the
Go program that embeds them. Subsetting makes a Derived Work, so LPPL 1.3c clause 6
applies. The two obligations that actually bite:

- **6b** — "Every component of the Derived Work contains prominent notices detailing the
  nature of the changes to that component". A `NOTICE` line saying "subsetted from TeX
  Gyre Pagella 2.501; glyph coverage reduced" satisfies this.
- **6d** — distribute a complete unmodified copy of the Work, *or* offer equivalent
  access to it from the same place. A link in the NOTICE satisfies this.

The rename is **requested, not required** — GUST says so explicitly. Renaming to
something like "Dira Serif" is still the tidier choice and costs nothing.

### Source Serif 4, Newsreader, Literata — **OFL 1.1. The easiest of the three.**

OFL 1.1 condition 2 is the operative one:

> Original or Modified Versions of the Font Software may be **bundled,
> redistributed and/or sold with any software**, provided that each copy
> contains the above copyright notice and this license.

Bundling into an Apache-2.0 binary is expressly permitted, no copyleft reaches dira's
code, and the only obligations are: ship the OFL text, ship the copyright line, don't
sell the font by itself (condition 1). **None of the three declares a Reserved Font
Name** — I checked each OFL header — so subsetting does not force a rename either.

**Verdict on licence alone:** OFL trio easiest → Pagella straightforward with two
one-line chores → **P052 should be dropped.**

---

## 3. Do the existing ratios still hold? — measured, not asserted

### Why this is arithmetic and not taste

Every measure in `tokens.css` is in `ch`: `--m-prose: 64ch`, `--m-lede: 60ch`, and
`.arg .grounds { max-width: 64ch }`. **`ch` is the advance width of "0" in the resolved
font.** So changing the serif moves two things at once — the pixel width of the box, and
the average width of the letters going into it — and they do not move together. The
question is what happens to their ratio.

### The units each face resolves to

Browser-resolved at 1024px, light, laptop (`measurements.json`):

| Candidate | `ch` (adv of "0", em) | vs Palatino | `ex` (x-height, em) | vs Palatino |
|---|---:|---:|---:|---:|
| **Palatino (control)** | 0.5000 | — | 0.4711 | — |
| URW P052 | 0.5000 | **±0.0%** | 0.4689 | −0.5% |
| TeX Gyre Pagella | 0.5000 | **±0.0%** | 0.4689 | −0.5% |
| Source Serif 4 | 0.5191 | +3.8% | 0.4859 | +3.1% |
| Newsreader | 0.5666 | +13.3% | 0.4405 | **−6.5%** |
| Literata | 0.5820 | **+15.8%** | 0.5069 | **+7.6%** |

### Advance widths against macOS Palatino, the face this system was tuned on

Per-glyph, normalised to em (`metrics-report.json`, `scripts/metrics.py`):

| Candidate | lowercase a–z mean Δ | a–z max Δ | space Δ | worst ASCII glyph |
|---|---:|---:|---:|---|
| URW P052 | **0.0001 em** | 0.0002 em | 0.0000 | `#` (Δ 0.106) |
| TeX Gyre Pagella | **0.0002 em** | 0.0040 em | 0.0000 | `\|` (Δ 0.398) |
| Source Serif 4 | 0.0308 em | 0.0652 em | −0.0150 | `\|` (Δ 0.316) |
| Newsreader | 0.0299 em | 0.0910 em | −0.0210 | `\|` (Δ 0.354) |
| Literata | 0.0435 em | 0.0852 em | −0.0500 | `J` (Δ 0.224) |

**Read the "worst glyph" column carefully — it nearly misled me.** Pagella's whole-ASCII
mean delta (0.021 em) looks 18× worse than P052's (0.0012 em), which would suggest it is
a poorer clone. It is not. Every one of Pagella's large deltas is a *math or symbol*
glyph — `| < > = + ( )` — that TeX Gyre deliberately redrew for TeX. Across the letters
that actually set prose, Pagella is 0.0002 em from Palatino. **Both are true metric
clones for text.** Aggregate deltas over the full ASCII range are the wrong statistic
here and I have quoted the lowercase figure instead.

One more thing worth recording: **P052's declared x-height metadata is wrong.** Its OS/2
`sxHeight` says 0.449 em, but the browser resolves `ex` to 0.4689 — same as Pagella,
whose OS/2 says 0.469. The outlines agree; P052's metadata does not. Pagella fixed it.

### The measurement that decides it: characters per line

Same block in every page — `.arg .grounds` of the Elixir/OTP alternative, the longest
paragraph, `max-width: 64ch`, `--t-body: 16.5px`. Lines recovered from real client rects,
not estimated. Last line excluded from the mean (it is short by definition).

| Candidate | box px | lines | **CPL mean** | CPL max | block height | page height |
|---|---:|---:|---:|---:|---:|---:|
| **Palatino (control)** | 528.0 | 5 | **66.8** | 70 | 127.8 | 1899 |
| URW P052 | 528.0 | 5 | **66.8** | 70 | 127.8 | 1899 |
| TeX Gyre Pagella | 528.0 | 5 | **66.8** | 70 | 127.8 | 1899 |
| Source Serif 4 | 548.1 | 5 | **66.8** | 70 | 127.8 | 1899 |
| Newsreader | 598.2 | **4** | **83.3** | **87** | 102.2 | 1791 |
| Literata | 611.4 | 5 | **71.5** | 78 | 127.8 | 1873 |

Same story on `.because` (`60ch`, 19px): control/P052/Pagella/SS4 all 73.0 CPL over
4 lines; Newsreader 84.0 over 3; Literata 77.7 over 4.

`h1.ruling` (`24ch`, 52px) is **stable in all five** — 2 lines, 27 characters, 114.4px
tall. The display anchor does not need re-tuning for any candidate. Newsreader and
Literata's 24ch boxes are wider (707px, 723px) but the text only draws to 664px, so the
ceiling stops binding before it can do damage.

### So, per candidate, honestly

- **URW P052 — ratios hold exactly. Zero re-tuning.** Every measured value is identical
  to the control, including total page height to the pixel. This is what a metric clone
  buys, and it is real. It is also the one you cannot ship.
- **TeX Gyre Pagella — ratios hold exactly. Zero re-tuning.** Identical to P052 on every
  number in the table, with a marginally better x-height match to real Palatino and 6.7 KB
  less payload.
- **Source Serif 4 — ratios hold exactly, by coincidence rather than by design.** Its
  `ch` is 3.8% wider *and* its letters are ~7% wider, and the two effects cancel: the
  block grows 20px but sets the same 66.8 characters over the same 5 lines, and the page
  ends at the same 1899px. **The brief predicted this face would break the ratios. It
  does not.** What does change is the *pixel* width of every prose block (+3.8%), which
  matters for column fit next to the 244px rail, not for reading.
- **Newsreader — ratios break, and this is the worst of the five.** CPL goes 66.8 → 83.3
  (+25%), max line 70 → 87. 87 characters is well outside the comfortable range for
  continuous prose, and the paragraph collapses 5 lines → 4. Compounding it, its
  x-height is 6.5% *below* Palatino's, so 16.5px Newsreader reads visibly smaller and
  greyer than 16.5px Palatino — the page loses presence exactly where the design says
  the prose is the payload. Two separate re-tunes needed, not one.
- **Literata — ratios break modestly.** CPL 66.8 → 71.5 (+7%), max 70 → 78, line count
  and block height unchanged. Its x-height is 7.6% *above* Palatino's, so it reads
  slightly larger at the same px — the opposite problem to Newsreader's and the more
  forgiving direction.

### The re-tune, applied and re-measured

Predicted from the CPL ratio (`required_ch = 64 × 66.8 / measured_CPL`), then actually
set and re-measured — `docs/design/bakeoff/retune.mjs`:

| Candidate | shipping | predicted | **measured at the new value** | matches control? |
|---|---|---|---|---|
| P052 / Pagella | 64ch | 64ch | 528.0px · 5 lines · 66.8 · max 70 | **yes, unchanged** |
| Source Serif 4 | 64ch | 64ch | 548.1px · 5 lines · 66.8 · max 70 | **yes, unchanged** |
| Newsreader | 64ch | 51.3ch | **54ch** → 504.8px · 5 lines · 66.8 · max 70 | yes |
| Literata | 64ch | 59.8ch | **60ch** → 573.2px · 5 lines · 66.8 · max 70 | yes |

Both re-tunes land exactly on the control's measure. So the OFL faces that do move the
ratios are *fixable*, and the fix is one token. What the table does not capture:
`--m-prose` is shared across screens, so moving it moves `s2-index` and `s3-distill` too
— those were not re-measured here and would need the same check.

Size compensation to match Palatino's rendered x-height at 16.5px (7.77px), predicted
from `ex` and **not** verified by rendering:

| Candidate | `--t-body` would want to be |
|---|---|
| P052 / Pagella | 16.5px (no change) |
| Source Serif 4 | ~16.0px |
| Literata | ~15.3px |
| Newsreader | ~17.6px |

---

## 4. Newsreader or Literata — I rendered both

The brief asked for one of the two, chosen and justified. **I judge Literata the closer
fit** — but since asserting it and showing it cost about the same, both are built,
rendered at the full matrix, and measured, so this is overridable without new work.

**For Literata:** one token of re-tuning versus two for Newsreader. Its x-height error
runs in the forgiving direction (reads slightly large, not small). It holds up at 16.5px
on the warm `#f7f4ed` ground where Newsreader goes grey and recedes — visible in
`sheet-compare-light.png`, where Newsreader's page is the palest of the six. It has
arrows; Newsreader has none. And Newsreader's em dash is drawn conspicuously short,
which matters more here than it normally would: the em dash is load-bearing punctuation
in this design's voice, in nearly every `.grounds` paragraph.

**Against Literata, honestly:** it is the Google Play Books brand face, so it is
*recognisable* — a real cost for a tool that wants its own voice, and the exact thing
the brief was reaching for with "warmer and more distinctive". Newsreader is the more
distinctive face. Its tall ascenders (hhea 1.177 em vs Palatino's 0.823) are also a
latent risk at tight line-heights, though the 52px ruling at `line-height: 1.1` rendered
clean with no clipping.

If the founder wants distinctiveness over fit, Newsreader's page is already rendered and
its re-tune is already measured — it costs `--m-prose: 54ch`, `--t-body: ~17.6px`, and
accepting a serif with no arrows.

---

## 5. The trade-off, per candidate

| Candidate | Licence | Re-tuning | Payload | The trade |
|---|---|---|---|---|
| **URW P052** | **AGPL-3.0, exception is PS/PDF-only** | none | 67.2 KB | Perfect typographically, unusable legally. Everything the brief wanted from it, Pagella delivers under a licence that works. |
| **TeX Gyre Pagella** | LPPL 1.3c — NOTICE + link | none | 60.6 KB | The pure determinism fix. Zero re-tuning, smallest payload, closest x-height to real Palatino. Costs: no new expression, and Palatino's own weaknesses on screen come along with it. |
| **Source Serif 4** | OFL 1.1 — simplest | **none measured** | 64.1 KB | A contemporary face for free: same CPL, same line count, same page height, cleanest licence. Costs: every prose block gets 3.8% wider in pixels, and the page stops looking like Palatino — which is a change to the reviewed design, not a bug. |
| **Literata** | OFL 1.1 | `--m-prose: 60ch`, `--t-body: ~15.3px` | 64.1 KB | Most robust at small sizes, warmest of the OFL three. Costs: a recognisable brand face, and two tokens move. |
| **Newsreader** | OFL 1.1 | `--m-prose: 54ch`, `--t-body: ~17.6px` | 67.9 KB | The most distinctive bet. Costs: worst measure break (+25% CPL), reads small and grey at body size, ships no arrows, short em dash. |

**What I would flag if asked** (and was not — no winner is picked here): the real choice
is between *keeping the design that was reviewed* (Pagella) and *taking a free upgrade to
a screen-designed face that happens to cost nothing in ratios* (Source Serif 4). P052 is
out on licence. Newsreader and Literata are both real options but both ask for re-tuning
that the other two do not.

---

## 6. Artefacts — exact paths

All paths relative to the repo root `/Users/dndungu/Code/kazi-org/dira`.

### The two comparison sheets

```
docs/design/bakeoff/renders/sheet-compare-light.png       laptop 1024, all 6, real content + measure hairline
docs/design/bakeoff/renders/sheet-compare-dark.png
docs/design/bakeoff/renders/sheet-letterforms-light.png   3x (49.5px), one line of real .grounds prose
docs/design/bakeoff/renders/sheet-letterforms-dark.png
```

`sheet-compare` draws a hairline at each face's own `64ch` ceiling, so the measure
difference is visible and not just tabulated. `sheet-letterforms` is a true optical
enlargement (re-rendered at 49.5px), not a pixel zoom.

### Full pages — 6 targets × 3 viewports × 2 schemes = 36 PNGs

```
docs/design/bakeoff/renders/control-palatino-{mobile,laptop,wide}-{light,dark}.png
docs/design/bakeoff/renders/p052-{mobile,laptop,wide}-{light,dark}.png
docs/design/bakeoff/renders/pagella-{mobile,laptop,wide}-{light,dark}.png
docs/design/bakeoff/renders/sourceserif4-{mobile,laptop,wide}-{light,dark}.png
docs/design/bakeoff/renders/newsreader-{mobile,laptop,wide}-{light,dark}.png
docs/design/bakeoff/renders/literata-{mobile,laptop,wide}-{light,dark}.png
```

Viewports match `scripts/render.mjs`: mobile 390×844, laptop 1024×768, wide 1440×900.

### Sources, data and scripts

```
docs/design/bakeoff/_snapshot/            frozen s1-decision.html + tokens.css the pages were built from
docs/design/bakeoff/build.mjs             emits one page per candidate, identical except the serif
docs/design/bakeoff/render-bakeoff.mjs    capture + gate + measurement
docs/design/bakeoff/sheets.mjs            the two comparison sheets
docs/design/bakeoff/retune.mjs            applies the predicted measure and re-measures
docs/design/bakeoff/scripts/subset.py     subsetting (fontTools), both tiers
docs/design/bakeoff/scripts/metrics.py    advance-width comparison against macOS Palatino
docs/design/bakeoff/measurements.json     CPL, box widths, line counts, resolved ch/ex
docs/design/bakeoff/metrics-report.json   per-face metrics vs Palatino
docs/design/bakeoff/subset-report.json    byte sizes + coverage gaps
docs/design/bakeoff/fonts/*.core.woff2    the 15 subsetted faces actually rendered
docs/design/bakeoff/licences/             OFL ×3, URW LICENSE, GUST Font Licence
```

Font sources: P052 from `ArtifexSoftware/urw-base35-fonts`; Pagella 2.501 from
gust.org.pl; Source Serif 4 / Newsreader / Literata from `google/fonts` (variable,
instanced to wght 400/600 at opsz 16 before subsetting).

### Gate

`render-bakeoff.mjs` reproduces the existing mechanical gate and adds two checks this
exercise needs, because a bake-off where a face silently failed to load would produce
confident numbers about a fallback:

- every `@font-face` under test must report `loaded`, and
- no two candidates may render byte-identically at the same viewport.

**Result: GATE PASS** — no console errors, no failed requests, no blank mounts, no fake
dark, no layout shift, every face really loaded, no two candidates identical.

---

## 7. Caveats

- **The snapshot is frozen; the live screen is not — but they currently agree.** Another
  session was editing `s1-decision.html` and `tokens.css` throughout this run (the chain
  was rebuilt off box-drawing mid-way; the type scale collapsed to 9 tokens, landing as
  `a5d5e8d`). The snapshot was re-taken after those edits and **verified byte-identical
  to the live files**, so nothing here is stale. If the screen moves again, re-copy
  `_snapshot/` and re-run `build.mjs` before trusting these numbers.
- **One screen, not three.** Only `s1-decision` was rendered, per the brief. `--m-prose`
  is shared, so a re-tune for Newsreader or Literata moves `s2-index` and `s3-distill`
  too, and those were not measured.
- **The x-height size compensation is predicted, not rendered.** The measure re-tune was
  verified by re-measuring; the `--t-body` numbers were not.
- **`ch` under-counts on this design generally.** At `64ch`, Palatino sets 66.8
  characters, and at `60ch` `.because` sets 73 — because `ch` is the digit width
  (0.5 em) while average lowercase prose is 0.444 em. That is a property of the existing
  system, not something any candidate changes, but it means the `--m-*` numbers do not
  mean what they look like they mean.
- **Bold is 700, not 600, for P052 and Pagella.** Neither ships a 600, so the design's
  `font-weight: 600` is served by the 700 face (declared `font-weight: 600 700`) rather
  than synthesised. The OFL faces were instanced at a true 600. Visible on
  `.arg.upheld .name` in the sheets.
- **I am not a lawyer.** The licence section quotes primary text and Debian's record so
  the reasoning can be checked rather than trusted.
