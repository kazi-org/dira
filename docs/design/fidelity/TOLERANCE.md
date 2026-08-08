# The pixel tolerance, and the measurement that produced it

**Measured 2026-07-31 on darwin x64, Node v24.5.0, headless Chromium via Playwright,
`deviceScaleFactor: 2`.** Regenerate with:

```
node docs/design/scripts/measure-tolerance.mjs --write
```

Raw data: `tolerance-evidence.json` (180 measurements). Enforced values:
`tolerance.json`, which `pixeldiff.mjs` reads at run time — the number published
here and the number the gate applies are the same number, by construction.

| figure | value | what it means |
|---|---|---|
| channel threshold | `0/255` | **any** per-channel difference counts. Nothing is filtered. |
| pixel tolerance | `0.00055%` | share of differing pixels allowed |
| block tolerance | `2.5%` | share of any one 16×16 block allowed to differ |

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

Every input (`screens/*.html`, `*.css`) is SHA-256 fingerprinted before the run
and again after it. If any of them moved, the run is **void** and no tolerance is
emitted — captures taken before an edit are not comparable with captures taken
after it. Not hypothetical: a concurrent edit to `s1-decision.html` landed during
an earlier run of this script, in a tree with several agents working in it.

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

**All 60 noise measurements are bit-identical.** Not "small" — zero, with nothing
filtered, since the channel threshold is 0.

### Signal — the smallest real defects the gate must catch

Each is one line of CSS, injected by serving a modified `tokens.css` from memory.
Nothing on disk is edited: `docs/design/tokens.css` and `docs/design/screens/`
stay read-only throughout, so the reference is never adjusted to make the gate
agree with it.

| arm | the defect | min % px | peak Δ | worst block |
|---|---|---|---|---|
| `radius-2px` | `--r-card: 7px → 5px` — four corners, no reflow | **0.002214%** | 109 | **10.16%** |
| `chip-hue` | `.chip-id` colour → `--ink-mid` — one component, no reflow | 0.007904% | 122 | 35.16% |
| `hairline` | the card hairline removed | 0.064248% | 48 | 18.36% |
| `ink-2` | `--ink` hex off by **2/255** in one channel, both schemes | 0.497667% | **2** | 53.13% |
| `ink-swap` | `--ink` takes the value of `--ink-mid` | 0.569673% | 67 | 58.20% |
| `opacity-1pct` | `.stage.next` opacity `.58 → .57` | 0.769956% | **3** | 55.08% |
| `spacing-1px` | `--s4: 18px → 19px` | dimension change | 255 | — |
| `type-0.5px` | `--t-body: 16.5px → 17px` | dimension change | 255 | — |

The two reflow arms change the full-page capture *height*, which `pixeldiff.mjs`
refuses to reconcile at all: a size change is the regression, not something to
crop past.

`hairline` is inert on `s2-index` and `s3-distill`, `chip-hue` on `s2-index`, and
`opacity-1pct` on everything but `s3-distill` — those screens do not use the thing
the mutation changes. Inert rows are excluded from the signal floor. A screen a
defect cannot reach is not evidence that the defect is invisible; counting its
zero drags the floor to zero, which it did on the first run of the harness.

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

- pixel tolerance: `0.002214% ÷ 4 = 0.000553…` → **0.00055%** (4.0× below the floor)
- block tolerance: `10.16% ÷ 4 = 2.54` → **2.5%** (4.1× below the floor)

### The channel threshold is 0, and that is the whole point

Every pair is measured at 0, 4, 8 and 16/255. A candidate is admissible only if,
at that threshold, every noise row still measures zero, every signal row is still
visible, and the weakest signal arm's peak delta is still ≥4× the threshold:

| threshold | noise all zero | every signal visible | weakest signal peak Δ | verdict |
|---|---|---|---|---|
| **0/255** | yes | yes | 2/255 | **admissible — chosen** |
| 4/255 | yes | **no** | 2/255 | rejected |
| 8/255 | yes | **no** | 2/255 | rejected |
| 16/255 | yes | **no** | 2/255 | rejected |

**The SMALLEST admissible value wins.** An earlier version of this file took the
largest, and justified it as leaving the gate "as robust as the evidence allows".
That was backwards, and backwards in a way that read as a safety argument: a
larger channel threshold makes the gate **less sensitive**, not more robust. The
only thing a threshold above zero buys is immunity to per-pixel noise, and
per-pixel noise here is exactly zero — the same evidence used to reject
*tolerance = noise × k*. Applying that evidence in one dimension while ignoring it
in the other was incoherent.

The correct principle: **the threshold is the smallest value that filters all
measured noise.** Where noise is zero, that is zero. On a machine where it is not,
this selects the least desensitisation that does the job, rather than the most the
signal can survive.

### What the wrong threshold was hiding

4/255 looked defensible because every signal arm then in the set was *loud per
pixel* (peak deltas 16–255). A signal set made only of loud defects cannot
distinguish one channel threshold from another, because all of them survive any
threshold.

Two arms were added that are **quiet per pixel and large in area** — precisely the
class a channel threshold swallows. Both are ordinary mistakes rather than
synthetic ones: a hex off by two is a copy-paste error, a stepped opacity is a
routine CSS edit.

| arm | peak Δ | at threshold 0 | at threshold 4 | ratio to tolerance |
|---|---|---|---|---|
| `ink-2` | 2/255 | 0.497667% of pixels | **0.000000%** | 905× |
| `opacity-1pct` | 3/255 | 0.769956% of pixels | **0.000000%** | 1400× |

At 4/255 both are **completely invisible** — zero pixels counted — while each moves
roughly a thousand times the pixel tolerance. That is not a marginal blind spot;
it is a hole a whole wrong stylesheet fits through. And `opacity-1pct` is covered
by **no other gate**: `contrast.mjs` and `tokens-doc-sync.mjs` read declared hex
values, so a wrong opacity on a rule in a page's own stylesheet is visible to the
pixel gate or to nothing.

The direction of the trade, stated plainly: moving to threshold 0 made the *count*
tolerance slightly more permissive (0.00033% → 0.00055%), because at threshold 0
the reference defect `radius-2px` also counts its own faint antialiased edge
pixels, which raises the floor the tolerance is derived from. The 4× safety factor
is unchanged. The gate is net far stronger, because an entire defect class went
from invisible to caught.

---

## What this tolerance is NOT

**It is not a cross-machine allowance.** Two arms were measured for information
only — they gate nothing, and they quantify what the same-environment rule buys:

| arm | what varies | % px | worst block |
|---|---|---|---|
| `rasterization` | `-webkit-font-smoothing: auto` — glyph rasterization changes, layout does not | 1.23% – 4.64% | 44.92% |
| `serif-fallback` | the Palatino stack falls through to the generic serif, which is what a stock Linux install actually renders | 1.50% – 100% | 92.97% |

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

1. **The floor is only as low as the weakest arm we built.** A defect quieter than
   `radius-2px` (0.002214% of pixels; 158 device pixels on `s3-distill` at
   1440×900) may pass. The remedy is another signal arm, not a smaller number —
   which is exactly how the channel threshold got fixed.
2. **Measured on one platform.** Every figure here is darwin x64. On a different
   OS the noise arms may not be zero, and the derivation must be re-run rather
   than assumed — which is what `--write` is for.

**No longer a blind spot:** sub-threshold colour. At the previous 4/255 threshold,
a change where every channel moved by ≤4/255 was invisible *by design*. At 0/255
no such class exists — any difference of any magnitude is counted.
