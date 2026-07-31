# E6-L1 — the fidelity gate. Report.

**Status:** delivered and green. `node docs/design/scripts/gates.mjs` → **9 gates run · 7 passed ·
2 negative controls tripped · 0 failed · 0 blind.**

Committed by the lead as `e82c77e`; nothing was lost in that commit.

**Revision 2 (2026-07-31), after the lead's two challenges — both landed.** The
channel threshold moved from `4/255` to **`0/255`** and the derivation rule was
inverted. Numbers throughout this report are the post-challenge ones. See §1a for
what the old threshold was hiding; it was worse than either of us argued.

---

## 1. The tolerance, and the evidence for it

**`0.00055%` of pixels, at a channel threshold of `0/255`, with no 16×16 block more
than `2.5%` changed.** A comparison also fails outright if the two captures differ
in size — `pixeldiff.mjs` refuses to crop or scale, because a size change *is* the
regression.

The channel threshold is **zero**: any per-channel difference of any magnitude counts.
There is no filtered band, so no class of change this gate is blind to by construction.

Recorded in three places that cannot drift apart: `docs/design/fidelity/tolerance.json`
(which `pixeldiff.mjs` reads at run time), `docs/design/fidelity/TOLERANCE.md` (method
and full evidence), and `DESIGN.md` — where `tokens-doc-sync.mjs` asserts the numbers
appear *with their units*, so the published figure and the enforced figure are the
same figure.

### The headline finding: this harness is bit-deterministic

180 measurements, 3 screens × 2 viewports × 2 schemes. Five noise arms, each a
difference E6-L2 will genuinely have between a mockup baseline and a Go-served page:

| noise arm | what varies | max % px | worst block |
|---|---|---|---|
| `same-page` | same live page screenshotted twice, no reload | 0.000000% | 0.00% |
| `fresh-context` | reload in a new browser context | 0.000000% | 0.00% |
| `fresh-browser` | a second Chromium **process** | 0.000000% | 0.00% |
| `other-origin` | a second HTTP server on a different port | 0.000000% | 0.00% |
| `reserialized` | markup round-tripped through the DOM serializer — normalized attribute quoting, entity encoding, boolean attributes, i.e. what `html/template` output differs from hand-written HTML by (`dec-0012`) | 0.000000% | 0.00% |

**All 60 noise measurements are zero**, measured at channel threshold 0 where nothing
is filtered. Not "small" — identical.

### Which broke the obvious derivation, and that matters

The natural rule is *tolerance = noise × k*. With noise at exactly zero it yields zero
for every *k*. So the number is **not** absorbing observed variance; there is none. It
reserves headroom, and headroom only means something relative to what it must not
swallow. The rule therefore reads from the signal side:

> tolerance = smallest measured real defect ÷ 4, truncated (never rounded up) to two
> significant figures, and required to be at least the noise ceiling.

Eight signal arms, each one line of injected CSS (served from memory — `tokens.css` and
`screens/` stay read-only throughout, so the reference is never edited to make the gate
agree with it):

| signal arm | the defect | min % px | peak Δ | worst block |
|---|---|---|---|---|
| `radius-2px` | `--r-card: 7px → 5px`, four corners, no reflow | **0.002214%** | 109 | **10.16%** |
| `chip-hue` | `.chip-id` colour → `--ink-mid`, no reflow | 0.007904% | 122 | 35.16% |
| `hairline` | the card hairline removed | 0.064248% | 48 | 18.36% |
| `ink-2` | `--ink` hex off by **2/255** in one channel | 0.497667% | **2** | 53.13% |
| `ink-swap` | `--ink` takes `--ink-mid`'s value, both schemes | 0.569673% | 67 | 58.20% |
| `opacity-1pct` | `.stage.next` opacity `.58 → .57` | 0.769956% | **3** | 55.08% |
| `spacing-1px` | `--s4: 18px → 19px` | dimension change | 255 | — |
| `type-0.5px` | `--t-body: 16.5px → 17px` | dimension change | 255 | — |

`0.002214 ÷ 4 = 0.000553…` → **0.00055%** (4.0× below the floor).
`10.16 ÷ 4 = 2.54` → **2.5%** (4.1× below the floor).

`measure-tolerance.mjs` **refuses to emit a number at all** if the populations fail to
separate, or if any signal arm is inert everywhere, or if a design input changes mid-run.
It has refused on all three grounds during this lane (see §4).

### 1a. The channel threshold — the lead's challenge, and what it turned up

**The challenge:** my own admissibility table showed `0/255` admissible, so what does
`4/255` buy that the pixel-count safety factor does not? "Robustness against noise"
cannot be the answer, because noise measured zero — the same evidence used to reject
*tolerance = noise × k*.

**It stands, and the reality was worse than either of us argued.** Not because the
headroom was merely redundant, but because the signal set that made `4/255` look
admissible was defective: **every arm in it was loud per pixel** (peak deltas 16–255).
A signal set of only loud defects cannot distinguish one channel threshold from
another, since all of them survive any threshold. The derivation was choosing between
options it had no instrument to tell apart.

So rather than concede by argument, I added the missing instrument — two arms that are
**quiet per pixel and large in area**, both ordinary mistakes rather than synthetic
ones:

| arm | the defect | peak Δ | at `0/255` | at `4/255` | ratio to tolerance |
|---|---|---|---|---|---|
| `ink-2` | a token hex off by 2 in one channel — a copy-paste typo | 2/255 | 0.497667% of pixels | **0.000000%** | 905× |
| `opacity-1pct` | `.stage.next` opacity `.58 → .57` — a routine CSS edit | 3/255 | 0.769956% of pixels | **0.000000%** | 1400× |

At `4/255` both are **completely invisible** — zero pixels counted — while each moves
roughly a thousand times the pixel tolerance. That is not a marginal blind spot; it is
a hole a whole wrong stylesheet fits through. And `opacity-1pct` is covered by **no
other gate in the repo**: `contrast.mjs` and `tokens-doc-sync.mjs` read declared hex
values, so a wrong opacity on a rule in a page's own stylesheet is visible to the pixel
gate or to nothing.

With those arms present the admissibility table decides on its own — `4`, `8` and `16`
all fail "every signal still visible":

| threshold | noise all zero | every signal visible | verdict |
|---|---|---|---|
| **0/255** | yes | yes | **admissible — chosen** |
| 4/255 | yes | **no** | rejected |
| 8/255 | yes | **no** | rejected |
| 16/255 | yes | **no** | rejected |

**The trade, stated plainly rather than buried:** moving to `0/255` made the *count*
tolerance slightly more permissive (`0.00033% → 0.00055%`), because at threshold 0 the
reference defect `radius-2px` also counts its own faint antialiased edge pixels, which
raises the floor the tolerance is derived from. So this is not strictly free in every
dimension — a hypothetical defect producing only large deltas over a small area now
needs marginally more pixels to trip the count. The 4× safety factor is unchanged, and
the gate is net far stronger, because an entire defect class went from invisible to
caught.

### 1b. Challenge 2 — the sentence that argued against itself

**Conceded without qualification**, and it went deeper than wording. The sentence read
"the largest admissible value wins, so the gate is as robust as the evidence allows and
no more" — but a larger channel threshold makes the gate *less* sensitive, so the
sentence justified the opposite of what it concluded.

Fixing only the wording would have left the defect, because **the rule itself was
inverted**, not just its description. Taking the largest admissible value imports an
unstated premise that sensitivity is costly and must be traded away as far as the
signal will bear. Nothing in the measurement supports that premise.

The rule is now: **the threshold is the smallest value that filters all measured
noise.** Where noise is zero, that is zero. Where it is not, this selects the least
desensitisation that does the job rather than the most the signal can survive. Changed
in `measure-tolerance.mjs`, in `TOLERANCE.md`, and in `DESIGN.md` — the lead was right
that the old sentence is what a future reader would have used to justify widening it
again.

### Two percentages, not one — and why

Legitimate variance and real regressions have opposite *shapes*. Antialiasing and
subpixel text positioning are **diffuse** — a thin scatter along every glyph edge.
A real regression is **clustered** — one element moves and a compact region goes dense.
A frame-wide percentage is area-weighted, so a tolerance loose enough to absorb diffuse
noise across a 2880×5000 capture is also loose enough to hide an entire missing card.
The worst-block figure is scale-free and catches that. `pixeldiff.mjs --self-test`
proves the separation: 100 changed pixels arranged diffusely and 100 arranged in one
patch produce **identical** global percentages and worst-block figures differing by 100×.

### What the tolerance is NOT: a cross-machine allowance

Two arms measured for information only, gating nothing:

| info arm | what varies | % px | worst block |
|---|---|---|---|
| `rasterization` | `-webkit-font-smoothing: auto` — glyphs rasterize differently, layout identical | 1.23 – 4.64% | 44.92% |
| `serif-fallback` | the Palatino stack falls through to the generic serif — what a stock Linux install renders | 1.50 – 100% | 92.97% |

Three to four orders of magnitude above the tolerance. No number that still catches a
2px radius change could absorb them, and one widened until it could would be measuring
nothing. **This is the evidence for the protocol E6-L2 already commits to**: regenerate
the baseline in the same run and the same environment as the capture.
`docs/design/renders/` is gitignored precisely so a baseline cannot be hand-committed
and compared across machines. It also puts a number on the macOS blind spot `DESIGN.md`
describes in prose — the Linux serif fallback costs up to **100%** of pixels.

### Stated blind spots

1. **The floor is only as low as the weakest arm built.** Something quieter than
   `radius-2px` (158 device pixels on `s3-distill` at 1440×900) may pass. The remedy is
   another signal arm, not a smaller number — which is exactly how the channel threshold
   got fixed.
2. **One platform.** Every figure is darwin x64. Elsewhere the noise arms may not be
   zero and the derivation must be re-run, not assumed.

**No longer a blind spot:** sub-threshold colour. At `4/255` a change where every
channel moved ≤4/255 was invisible by design. At `0/255` no such class exists.

---

## 2. Red-then-green, verbatim

Every check below was watched failing before it was trusted passing.

### 2a. Contrast — the pre-r4 regression restored

```
$ node docs/design/scripts/contrast.mjs --probe-regression
--probe-regression: light --bearing-lift restored to the pre-r4 value #b8862f.

42 ink/surface pairs checked across 2 schemes, plus 6 hover>rest assertions.
  hover < rest  INVERTED  light on --ground rest 5.12:1 -> hover 2.95:1
  hover < rest  INVERTED  light on --panel  rest 5.54:1 -> hover 3.18:1
  hover < rest  INVERTED  light on --sunk   rest 4.56:1 -> hover 2.62:1
  hover > rest  dark  on --ground rest 7.14:1 -> hover 9.91:1
  hover > rest  dark  on --panel  rest 6.54:1 -> hover 9.08:1
  hover > rest  dark  on --sunk   rest 7.38:1 -> hover 10.24:1

6 failures

PROBE RESULT — 3 contrast-floor violation(s), 3 hover inversion(s):
PROBE OK — the known regression is caught, on both the floor and the hover direction.
exit=1
```

**2.95 / 3.18 / 2.62 and six failures reproduce `DESIGN.md`'s r3→r4 record exactly** —
independent corroboration that the script measures what the document says it measures.
Restored: `0 failures`, `CONTRAST PASS`, exit 0.

### 2b. The loopback rule — an asset that *loads*

The pre-existing gate caught **failed** requests, so it would have passed a page that
successfully loads a CDN webfont — the exact shape that makes "your data never touches
our servers" false. The control had to stage a request that **succeeds** from a
non-loopback host, so it serves a real stylesheet from this machine's LAN address:

```
$ node docs/design/scripts/render.mjs probe --probe-external
--probe-external: serving an asset from http://192.168.86.29:65145/probe.css
  (a real non-loopback host; the request will SUCCEED)

PROBE — external asset served from http://192.168.86.29:65145
  non-loopback findings: 6
  failed-request findings: 0  <- expected 0: the asset LOADS, which is why the old check cannot see it
    NON-LOOPBACK asset (1x): http://192.168.86.29:65145/probe.css — host is not 127.0.0.1 (cst-0004, dec-0010)

PROBE OK — the loopback check fired on a request the failed-request check could not see.
exit=1
```

`failed-request findings: 0` is the load-bearing line. The full run over the nine
committed targets still reports `GATE PASS` (54 shots), so the new check does not
false-positive.

### 2c. The pixel gate — a subtle deviation, end to end

Same screen, captured clean twice and once with one component's hue changed and **no
reflow at all**:

```
GREEN  baseline.png vs rerun-clean.png   2880x4324
  differing, any delta                 0   0.0000%
  worst 16x16 block                  0.00%
  PIXELDIFF PASS                                                        exit=0

RED    baseline.png vs deviated.png     (.chip-id colour -> --ink-mid)
  differing, any delta               870   0.0070%
  differing, delta >   0/255         870   0.0070%   (tolerance 0.00055%)
  max per-channel delta               53
  worst 16x16 block                 54.30%   (tolerance 2.5%)  at 480,880
  changed region              113x19 at 413,880
  PIXELDIFF FAIL — 0.0070% of pixels differ, over the 0.00055% tolerance; a 16x16
  block at 480,880 is 54.3% changed, over the 2.5% block tolerance — that is
  clustered, not antialiasing                                           exit=1
```

870 pixels out of 12.4 million, caught and **localized to a 113×19 region**, with a
diff PNG written. The lane's `acc:` cases also hold: identical file → `0.0000%` exit 0;
light vs its dark pair → `99.9995%` exit 1; differing dimensions → `dimension mismatch
… refusing to crop or scale` exit 2.

### 2d. The tolerance record in DESIGN.md — broken on the real file, then restored

Requested by the lead: own the paragraph, end green, and prove the sync check by
breaking it. The check asserts **one canonical sentence generated from
`tolerance.json`**, not three loose substring searches — three independent searches
pass as long as each number appears *somewhere*, which a document can satisfy while
stating them in a sentence that means something else. One generated line has a single
truth condition and can print the exact string the document must carry.

```
########## 1. BASELINE — as committed, untouched ##########

0 failures
TOKEN/DOC SYNC PASS — DESIGN.md and tokens.css agree value for value, and the measured tolerance is recorded.
exit=0

########## 2. RED — the tolerance line DELETED ##########

TOKEN/DOC SYNC FAIL:
  - DESIGN.md does not carry the canonical tolerance line from docs/design/fidelity/tolerance.json.
      expected: **`0.00055%` of pixels, at a channel threshold of `0/255`, with no 16×16 block more than `2.5%` changed.**
      found:    (no line of this shape anywhere in the file)
      -> the line was never written, or was deleted.
      docs/plan.md §E6 cites "the pixel tolerance recorded in DESIGN.md"; the enforced number
      lives in tolerance.json, so a document that disagrees with it makes that clause unfalsifiable again.
  - DESIGN.md never states the measured pixel tolerance (0.00055%), block tolerance (2.5%), channel threshold (0/255) anywhere in the file.
exit=1

########## 3. RED — line PRESENT, one number DRIFTED (0/255 -> 4/255) ##########

TOKEN/DOC SYNC FAIL:
  - DESIGN.md does not carry the canonical tolerance line from docs/design/fidelity/tolerance.json.
      expected: **`0.00055%` of pixels, at a channel threshold of `0/255`, with no 16×16 block more than `2.5%` changed.**
      found:    **`0.00055%` of pixels, at a channel threshold of `4/255`, with no 16×16 block more than `2.5%` changed.**
      -> the line is present but its numbers have drifted from the measured ones.
      docs/plan.md §E6 cites "the pixel tolerance recorded in DESIGN.md"; the enforced number
      lives in tolerance.json, so a document that disagrees with it makes that clause unfalsifiable again.
  - DESIGN.md never states the measured channel threshold (0/255) anywhere in the file.
exit=1

########## 4. GREEN — restored ##########

0 failures
TOKEN/DOC SYNC PASS — DESIGN.md and tokens.css agree value for value, and the measured tolerance is recorded.
exit=0
```

The two red cases are deliberately distinguished. *Deleted* and *drifted* need different
fixes, and calling a present-but-wrong line "missing" sends the reader to the wrong
place. Case 3 is the dangerous one — the document stating a tolerance the gate does not
enforce — and it prints expected and found side by side with the single differing figure
visible.

**This proof caught a real bug in the check itself.** On its first run, step 1 — the
untouched baseline — came back **red**. The canonical sentence wraps across two lines in
DESIGN.md, so a literal `includes()` could never match it: the check would have failed
on a document that was correct, and no amount of writing the paragraph would have fixed
it. Comparison is now done with whitespace collapsed, the same normalization
`check-coherence.mjs` uses for the same reason. A second, cosmetic bug surfaced in step
3: the excerpt regex was anchored on "any run of non-full-stops", and the figure it
quotes is a decimal, so it clipped the line to `0005%` and reported a corrupted excerpt
of a real sentence.

Both were only visible because the check was watched failing on a case that was supposed
to pass. Watching a gate go red is not sufficient — the *baseline green* has to be
watched too, or a check that fails on everything looks identical to a check that works.

### 2e. Token/doc sync — both hue-drift directions

Run against **copies** (the script takes `--design` / `--tokens`, so the checker can be
tested without editing what it guards):

```
edit one hex in the tokens.css copy only:
  DRIFT --caught  light #a83828 == #a83828   dark #d97060 != #d97050
  - --caught dark: DESIGN.md says #d97060, tokens.css says #d97050 — tokens.css is
    the declared single source, so DESIGN.md is the one that is wrong        exit=1

delete one row from the DESIGN.md copy only:
  - --converged is in the hue budget but has no row in DESIGN.md's hue table
    (a table can also drift by omission, which is why this is checked separately)  exit=1
```

It was also genuinely red on the real files until the tolerance was recorded — that red
is in §3.

### 2f. Fixture content

```
reword one why_not in the fixture:
  - dec-0001 alternatives[1].why_not: not present verbatim in any mockup   exit=1
restored:
  FIXTURE CONTENT PASS — every rendered fixture string is byte-equal to the mockups.
```

### 2g. The comparator's own arithmetic

`pixeldiff.mjs --self-test` — 13 assertions, each with a known-bad counterpart. It
failed on its first run and caught two real bugs in my own test (§4).

---

## 3. What I changed outside the two directories originally named

**One file: `docs/design/DESIGN.md`** — now explicitly granted, scoped to the tolerance
paragraph and the fidelity-gate section. Nothing else in that file was touched.

The edit is **purely additive**: two new subsections at the end of *Verification*
("One command" and "The pixel tolerance"). No existing prose was rewritten or deleted,
including the r3→r4 record, the law-3 amendment, and the hue table.

Everything else is inside `docs/design/scripts/**` and `docs/design/fidelity/**`.
`tokens.css`, `screens/`, `.dira/entries/`, all Go, `.github/`, `docs/roadmap.md`,
`docs/coverage.md` and both `scripts/*.py` are untouched — `git status` confirms the
only file I did not create or own is DESIGN.md.

**One deviation from the lane's stated path:** the fixture ledger is at
`docs/design/fidelity/fixtures/ledger-design/`, not `docs/design/fixtures/ledger-design/`
as E6-L2's `acc:` line cites, because `fidelity/**` is mine and `fixtures/**` was not
granted. One `git mv` either way — flagging it so E6-L2's predicate and the path agree
before that lane runs.

---

## 4. Gates that caught their own author

Recorded because "a gate you have not seen fail is not a gate" applies to the person
writing it too. Each of these was found by the harness, not by review:

1. **The self-test's own arithmetic was wrong twice.** A stray pixel in a *partial* edge
   block scored 6.25% instead of 0.39% (smaller denominator), and `XOR 0xff` — which
   looks like a maximal perturbation — produces a delta of **1** at value 127, so a
   "solid" patch measured 97.66% full instead of 100%. Both would have silently
   mis-calibrated the clustering metric.
2. **The first tolerance derivation was fitted to invented constants** (`floor 0.01%`,
   `floor 25%`) that sat *above* the measured signal floor. Deleted; the rule now reads
   from the data.
3. **The signal floor collapsed to zero** because a mutation that cannot reach a screen
   (`s1-decision` has no `.card`) scored 0 and was averaged in. Inert rows are now
   excluded from the floor, counted, printed by name, and an arm inert *everywhere* is a
   hard failure rather than a free pass.
4. **A noise arm measured something other than its label.** `same-context` was
   indistinguishable from `fresh-context` because `capture()` always opened a new
   context — two arms, one measurement, and a determinism claim resting on a duplicate.
   Now `same-page` re-shoots a genuinely live page.
5. **The DESIGN.md tolerance check was trivially true.** `design.includes("4")` matches
   "r4", "4.5:1" and "42 pairs". Every figure is now searched *with its unit*
   (`0.00055%`, `2.5%`, `0/255`), and all three as one generated sentence (§2d).
6. **`$` under the `/m` flag** made the fixture parser read one line of the alternatives
   block, so every `why_not` came back empty — and an empty string then "matches"
   nothing and was reported as missing. Caught only because the check was two-sided.
7. **Float noise published `0.00033000000000000005%`** as the tolerance.

8. **The tolerance-record check failed on a correct document.** The canonical sentence
   wraps across two lines in DESIGN.md, so a literal `includes()` could never match it —
   found only by watching the *baseline green* case, not the red ones (§2d).
9. **The drift excerpt was clipped at a decimal point**, reporting `0005%` as if that
   were the line in the file.
10. **The signal set could not distinguish the thing it was being used to choose.** Every
    arm was loud per pixel, so the channel-threshold derivation was picking between
    options it had no instrument to tell apart — found only because the lead challenged
    the result and the fix was to add the missing arm rather than to argue (§1a).
11. **A signal arm lost on CSS source order.** `opacity-1pct` was injected by appending
    to `tokens.css`, but `.stage.next` is declared in `s3-distill`'s inline `<style>`,
    which the browser applies later — so the arm measured nothing and the harness
    correctly refused to emit a tolerance at all.
12. **The harness could not tell that its own reference had moved.** A concurrent edit to
    `s1-decision.html` landed mid-run, in a tree with several agents in it, silently
    mixing baselines captured before the edit with captures made after. Every design
    input is now SHA-256 fingerprinted before and after; a run whose inputs moved is
    VOID and emits no tolerance.

`gates.mjs` encodes the general lesson on its own exit code: **3 = a gate passed but its
negative control did not trip.** That is reported as `BLIND`, not as a pass. §2d adds the
converse, which cost two bugs to learn: **watching a gate go red is not sufficient.** A
check that fails on everything looks identical to a check that works, unless the case
that is supposed to pass is watched passing too.

---

## 5. Things in the brief that turned out to be wrong

1. **"The contrast matrix does not exist as an artifact."** It does —
   `docs/design/scripts/contrast.mjs`, written and passing. What it lacked was the
   `--probe-regression` negative control, now added. `contrast-rendered.mjs` (the
   as-composited matrix) also already existed and is stronger than the planner prompt's
   description of the problem.
2. **"DESIGN.md's token table contradicts tokens.css in three of four rows."** Stale —
   all four rows already agreed when I arrived (`--bearing` light `#8a5f18`,
   `--bearing-lift` light `#6d4a12`, `--caught` dark `#d97060`). Somebody fixed it
   between the E6 lane doc being written and this lane running. I wrote the checker
   anyway: an unchecked agreement is a coincidence, and it is now pinned in both
   directions.
3. **"A pixel tolerance must be measured … antialiasing, subpixel layout."** The premise
   that there is legitimate per-run variance to absorb **did not survive measurement**
   on this machine. There is none — 60 of 60 noise measurements are bit-identical. The
   tolerance is justified on entirely different grounds (headroom below the smallest
   constructible defect), and the antialiasing story turns out to belong to the
   *cross-environment* case, which the protocol forbids rather than tolerances.
4. **`127.0.0.2` is not usable as a non-loopback stand-in on macOS** (`EADDRNOTAVAIL`
   without a `lo0` alias, which needs sudo). The control uses the LAN address instead
   and degrades to the `localhost` alias with a stated warning when no interface exists.

---

## 6. Left undone, explicitly

1. ~~**Schema validation of the fixtures is verified but not gated.**~~ **Closed by
   E6-L2** while this revision was in progress, and verified rather than taken on
   trust — `go test ./internal/ui/` runs `TestDesignFixturesValidate`,
   `TestTheValidatorRejectsTheInvalidCorpus` (the two-sided control, now 18 known-bad
   files), and `TestTheRealLedgerValidates`, all PASS. They also converted the upheld
   finding into an assertion on both sides of the seam
   (`TestNoUpheldCardIsEverRendered` plus a check in `fixture-check.mjs`), which is
   the right disposition: a resolved finding that nothing re-checks is a finding that
   comes back.
2. **`web-interface-guidelines` (E6-L1-T1) not installed.** It writes to
   `~/.claude/skills/`, outside this repo entirely and outside anything you granted.
   Still genuinely uninstalled; `DESIGN.md`'s hand-off note remains accurate.
3. **A modelling gap E6-L2 must close before writing the template.**
   `s1-decision.html` renders **four** alternative cards, one of them the *upheld*
   option ("Go — one static binary, sub-100 ms cold start") with its own grounds
   paragraph. `entry.schema.json` models `alternatives` as the roads **not** taken —
   `why_not` is required — so the chosen option has no field to live in, and its grounds
   are not the decision's `because` either (that paragraph is about hook latency).
   The fixture therefore carries three alternatives, and `fixture-check.mjs` prints this
   as a FINDING on every run rather than failing. Three ways out, none free, are written
   up in `docs/design/fidelity/fixtures/README.md`. Option 3 (drop the card from the
   mockup) would break the struck-refusal device `DESIGN.md` explicitly defended in
   r3→r4, so it should not be taken quietly.

---

## 7. Colour resolution — checked against the `oklab` warning

Flagged by the lead: if the fidelity gate compares colour anywhere, resolve via canvas
rather than parsing strings, because `color-mix(in oklab, …)` comes back as `oklab(…)`
and reading those floats as RGB once reported a legible paragraph at 1.31:1.

**It does not compare colour by string anywhere.** Verified rather than assumed:

- `pixeldiff.mjs`, `lib/png.mjs`, `measure-tolerance.mjs` — zero occurrences of
  `getComputedStyle`. They read **decoded PNG pixel bytes**, which is the rendered
  result itself, downstream of every `color-mix` the browser composited. There is no
  colour string in the pixel path to parse.
- `contrast.mjs` and `tokens-doc-sync.mjs` do parse hex, but only **literal `#rrggbb`
  declarations** out of `tokens.css` and DESIGN.md's table. That is unambiguous by
  construction and involves no colour space conversion, no `color-mix`, and no computed
  value.
- `contrast-rendered.mjs` — unmodified. It remains the authority for composited colour,
  and `gates.mjs` runs it on every pass, describing it in exactly those terms.

So the pixel gate and the composited-contrast gate are complementary rather than
overlapping: one measures painted pixels, the other measures painted colour, and neither
reads a computed colour string.

## 8. The one command

```
node docs/design/scripts/gates.mjs          # everything, plus every negative control
node docs/design/scripts/gates.mjs --fast   # skip the three browser gates
node docs/design/scripts/gates.mjs --list   # what runs and what each one proves
```

```
9 gates run · 7 passed · 2 negative controls tripped · 0 failed · 0 blind
GATES PASS — every gate green, every negative control tripped.
```

Runtime ≈ 3.6 min full, ≈ 2 s with `--fast` (which prints that it is not a full
result). Exit codes: `0` all green · `1` a gate failed · `3` a gate passed but its
control did not trip.
