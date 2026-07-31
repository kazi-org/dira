# The pixel tolerance, and the measurement that produced it

**Measured 2026-07-31 on darwin x64, Node v24.5.0, headless Chromium via Playwright,
`deviceScaleFactor: 2`.** Regenerate with:

```
node docs/design/scripts/measure-tolerance.mjs --write
```

Raw data: `tolerance-evidence.json` (156 measurements). Enforced values:
`tolerance.json`, which `pixeldiff.mjs` reads at run time — the number published
here and the number the gate applies are the same number, by construction.

| figure | value | what it means |
|---|---|---|
| channel threshold | `4/255` | a per-channel delta at or below this does not count as a differing pixel |
| pixel tolerance | `0.00033%` | share of pixels over that threshold that may differ |
| block tolerance | `1.6%` | share of any one 16×16 block that may differ |

A comparison fails if **either** percentage is exceeded, or if the two images
differ in size at all.

---

## Why there are two percentages

Legitimate variance and real regressions have opposite *shapes*, and a single
frame-wide percentage cannot tell them apart.

Antialiasing and subpixel text positioning are **diffuse**: a thin scatter of
small deltas along every glyph edge in the frame. A real regression is
**clustered**: one element moves, one border disappears, one hue is wrong, and a
compact region goes dense.

A tolerance loose enough to absorb diffuse noise across a 2880×5000 capture is
also loose enough to swallow an entire missing card, because a percentage is
area-weighted. The worst-block figure is scale-free — it does not care how tall
the page is — so it catches the clustered case at a threshold the global figure
could never afford. `pixeldiff.mjs --self-test` includes the two-sided proof:
100 changed pixels arranged diffusely and 100 arranged in one patch produce
**identical** global percentages and worst-block figures that differ by 100×.

---

## The measurement

Two populations, captured under identical conditions across
**3 screens × 2 viewports × 2 schemes**: `s1-decision`, `s2-index`, `s3-distill`
at 390×844 and 1440×900, light and dark.

### Noise — differences that must NOT fail the gate

Each arm is a difference E6-L2 will genuinely have between a mockup baseline and
a Go-served page.

| arm | what varies | max % px | worst block |
|---|---|---|---|
| `same-page` | the same live page screenshotted twice — no reload | 0.000000% | 0.00% |
| `fresh-context` | a reload in a new browser context | 0.000000% | 0.00% |
| `fresh-browser` | a second Chromium **process** | 0.000000% | 0.00% |
| `other-origin` | served from a second HTTP server on a different port | 0.000000% | 0.00% |
| `reserialized` | markup round-tripped through the DOM serializer — normalized attribute quoting, entity encoding, boolean attributes, which is what `html/template` output differs from hand-written HTML by (`dec-0012`) | 0.000000% | 0.00% |

**All 60 noise measurements are bit-identical.** Not "small" — zero, at a channel
threshold of 0, where nothing is filtered.

### Signal — the smallest real defects the gate must catch

Each is one line of CSS, injected by serving a modified `tokens.css` from memory.
Nothing on disk is edited: `docs/design/tokens.css` and `docs/design/screens/`
stay read-only throughout, so the reference is never adjusted to make the gate
agree with it.

| arm | the defect | min % px | worst block |
|---|---|---|---|
| `radius-2px` | `--r-card: 7px → 5px` — four corners, no reflow | **0.001345%** | **6.64%** |
| `chip-hue` | `.chip-id` colour → `--ink-mid` — one component wrong, no reflow | 0.006328% | 28.13% |
| `hairline` | the card hairline removed | 0.056339% | 16.02% |
| `ink-swap` | `--ink` takes the value of `--ink-mid` in both schemes | 0.522857% | 55.86% |
| `spacing-1px` | `--s4: 18px → 19px` | dimension change | — |
| `type-0.5px` | `--t-body: 16.5px → 17px` | dimension change | — |

The two reflow arms change the full-page capture *height*, which `pixeldiff.mjs`
refuses to reconcile at all: a size change is the regression, not something to
crop past.

`hairline` is inert on `s2-index` and `s3-distill`, and `chip-hue` on `s2-index` —
those screens do not use the thing the mutation changes. Inert rows are excluded
from the signal floor. A screen a defect cannot reach is not evidence that the
defect is invisible; counting its zero drags the floor to zero, which it did on
the first run of the harness.

---

## How the numbers were derived

The obvious rule is *tolerance = noise × k*. It does not survive contact with this
measurement: noise is exactly zero on every arm, so *noise × k* is zero for every
*k*, and a zero-width gate is brittle for no stated reason.

What the measurement actually found is that **this harness is deterministic**. So
the tolerance is not absorbing observed variance — there is none to absorb. It is
reserving headroom, and headroom is only meaningful relative to the thing it must
not swallow. The rule therefore reads from the signal side:

> tolerance = smallest measured real defect ÷ 4, truncated (never rounded up) to
> two significant figures, and required to be at least the noise ceiling.

Truncation matters: rounding up would silently eat part of the safety factor that
is the entire content of the number.

- pixel tolerance: `0.001345% ÷ 4 = 0.000336…` → **0.00033%** (4.1× below the floor)
- block tolerance: `6.64% ÷ 4 = 1.66` → **1.6%** (4.2× below the floor)

The channel threshold is **selected**, not chosen. Every pair was measured at
0, 4, 8 and 16/255. A candidate is admissible only if, at that threshold, every
noise row still measures zero, every signal row is still visible, and the weakest
signal arm's peak delta is still ≥4× the threshold:

| threshold | noise all zero | every signal visible | weakest signal peak Δ | verdict |
|---|---|---|---|---|
| 0/255 | yes | yes | 16/255 = 16.0× | admissible |
| **4/255** | yes | yes | 16/255 = 4.0× | **admissible — chosen** |
| 8/255 | yes | yes | 16/255 = 2.0× | rejected |
| 16/255 | yes | **no** | 16/255 = 1.0× | rejected |

The largest admissible value wins, so the gate is as robust as the evidence
allows and no more.

`measure-tolerance.mjs` refuses to emit a tolerance at all if the two populations
fail to separate by the safety factor. It has done so: on its first run the
signal floor collapsed to zero (the inert-row bug above) and no number was
written.

---

## What this tolerance is NOT

**It is not a cross-machine allowance.** Two arms were measured for information
only — they gate nothing, and they quantify what the same-environment rule buys:

| arm | what varies | % px | worst block |
|---|---|---|---|
| `rasterization` | `-webkit-font-smoothing: auto` — glyph rasterization changes, layout does not | 1.07% – 4.64% | 46.88% |
| `serif-fallback` | the Palatino stack falls through to the generic serif, which is what a stock Linux install actually renders | 1.46% – 100% | 100.00% |

Either is **three to four orders of magnitude** above the tolerance. No tolerance
that still catches a 2px radius change could ever absorb them, and one widened
until it could would be measuring nothing.

That is the evidence for the protocol E6-L2 already commits to: the baseline must
be regenerated in the same run and the same environment as the capture.
`docs/design/renders/` is gitignored precisely so a baseline cannot be
hand-committed and compared across machines. Cross-environment variance is
designed out, not toleranced.

It is also the number behind a caveat `DESIGN.md` already states in prose: this
harness runs on macOS with Palatino present, and structurally cannot see the
Linux serif fallback. `serif-fallback` reaching 100% of pixels on some screens is
what that blind spot costs when it lands.

## Stated blind spots

1. **Sub-threshold colour.** A change where every channel moves by ≤4/255 is
   invisible here **by design**. Token drift of that size is caught by
   `contrast.mjs` and `tokens-doc-sync.mjs`, which read the hex values directly.
   Do not widen the channel threshold to paper over a token gate.
2. **The floor is only as low as the weakest arm we built.** A defect quieter than
   `radius-2px` (0.001345% of pixels; 96 device pixels on `s3-distill` at 1440×900)
   may pass. The remedy is another signal arm, not a smaller number.
3. **Measured on one platform.** Every figure here is darwin x64. On a different
   OS the noise arms may not be zero, and the derivation must be re-run rather
   than assumed — which is what `--write` is for.
