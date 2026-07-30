# Open questions 1 & 2 — rendered alternatives

**Status:** options built and gate-verified. **No recommendation made** — this is a
deliberate omission, the founder decides from the pictures.
**Date:** 2026-07-30. Nothing committed, nothing pushed.

---

## Look at these two files first

| Question | Comparison sheet |
|---|---|
| **1 — long content** | `/Users/dndungu/Code/kazi-org/dira/docs/design/openq/renders/sheet-q1.png` |
| **2 — the withheld state** | `/Users/dndungu/Code/kazi-org/dira/docs/design/openq/renders/sheet-q2.png` |

Each is one image with every option side by side at laptop width, the trade-off
and its cost written under each column. Everything below is detail behind those
two pictures.

Contact sheet for all 36 captures (3 viewports × 2 schemes × 6 options):
`/Users/dndungu/Code/kazi-org/dira/docs/design/openq/renders/index.html`

---

## Question 1 — long content

The test case is a real 20-wide decision — what a ledger entry physically **is** on
disk — with a **405-word `why_not`** on alternative 2 (SQLite as source of truth).
All three options are generated from one dataset by
`openq/scripts/build-q1.mjs`, so the prose is byte-identical across the three and
the only variable is the disclosure treatment.

### Option A — let it run

`/Users/dndungu/Code/kazi-org/dira/docs/design/openq/q1-a-run-long.html`

- **Trade-off:** nothing is hidden and nothing is summarised, so every refusal keeps
  its strike-through with its grounds directly beneath — the device this direction
  was chosen for survives untouched.
- **Costs:** the page becomes 5,316 px at laptop (**6.9 screens**) and 8,304 px at
  mobile (**9.8 screens**), with no map — a reader has no way to know twenty
  alternatives exist until they arrive at the twentieth.

**What actually breaks.** At close range (`renders/detail-args-q1-a-run-long.png`)
a full 980 px window holds **one and a half alternatives**. The 405-word refusal at
position 2 occupies more vertical space than alternatives 3–20 combined, so the
reading order the ledger records — upheld first, then refusals — is destroyed by
sheer length: whichever alternative happens to have the longest grounds becomes the
page. The eye loses the thread at roughly the sixth refusal, where the rhythm
(struck name → paragraph → struck name → paragraph) stops reading as testimony and
starts reading as a transcript. The refusal device itself does **not** break; it is
still legible and still forceful at item 20. What breaks is the reader's ability to
know where they are.

### Option B — progressive disclosure

`/Users/dndungu/Code/kazi-org/dira/docs/design/openq/q1-b-progressive.html`

- **Trade-off:** the first four alternatives read exactly as they do today and the
  remaining sixteen sit behind one `<details>` — zero JavaScript, the same collapse
  mechanism the rebuilt chain now uses.
- **Costs:** it fixes the count and not the length, and it asserts a ranking the
  ledger does not record — nothing in an entry says which four alternatives matter
  most, so the cut is either arbitrary or a new field someone has to write.

**What actually breaks.** Compare the top row of `sheet-q1.png`: **A and B render
the same first screen.** This is structural, not coincidence — B emits
`ALTS.slice(0, 4)` through the same block renderer A uses, so its opening is A's
opening unchanged. The 405-word refusal is in the visible four, so
the first screen-and-a-half of B is the same wall A has. Page height drops to
2,857 px (3.7 screens) — a real 46 % cut — but every pixel of that saving is below
the fold, where the problem was not. The `<details>` summary row also lands
immediately before the footer, so at laptop width the closed state reads as
"end of page, plus an accordion", and a stranger scanning for the alternative they
care about has to open a box to find out whether it is in there at all.

### Option C — summary / detail split

`/Users/dndungu/Code/kazi-org/dira/docs/design/openq/q1-c-summary-detail.html`

- **Trade-off:** all twenty are visible and scannable at once, each carrying a
  one-line ground and expanding in place to its full reasoning, with the
  strike-through preserved on the summary line.
- **Costs:** the grounds are closed by default — `r3 → r4` explicitly rejected a
  scannable comparison list because it destroys the struck-refusal-with-grounds
  device, and this is that same list with a hinge on it.

**What actually breaks.** `renders/detail-args-q1-c-summary-detail.png` shows
**eleven alternatives in the same 980 px** that held one and a half in A, and the
one-line grounds genuinely carry meaning rather than being labels. Two things give
way. First, twenty struck-through names stacked at even intervals stop reading as
*refused after argument* and start reading as *crossed off a list* — the strike
survives mechanically and loses its rhetoric. Second, the `REFUSED` tag repeats
nineteen times against a `✗` and a strike that already say the same thing; at
twenty rows that redundancy becomes visual noise in a way it never is at four.
On mobile C is **longer than B** (4,997 px vs 4,652 px) despite hiding every
paragraph, because each summary row wraps to three or four lines at 390 px — the
scan value that justifies the treatment is a desktop-only benefit.

### Measured

| | laptop | mobile | wide |
|---|---|---|---|
| A let it run | 5,316 px (6.9 screens) | 8,304 px | 5,316 px |
| B progressive | 2,857 px (3.7) | 4,652 px | 2,839 px |
| C summary/detail | 2,906 px (3.8) | 4,997 px | 2,888 px |

---

## Question 2 — the withheld state

Every option renders the identical page — a decision whose parent is
`sire:int-0002` in a private ledger — and every option shows **all three
resolution states adjacent** (oriented, withheld, orphan) with the real red drift
card in the rail. The constraint is that withheld must read as neither an error nor
a warning, and that cannot be judged in isolation: it has to be judged against the
alarm sitting two columns away. All three are rendered in both schemes.

### Option A — plain speech

`/Users/dndungu/Code/kazi-org/dira/docs/design/openq/q2-a-plain.html`

- **Trade-off:** the state spends nothing — no hue, no icon, no surface change — and
  where the parent's title would be, the page simply states what is true in the same
  neutral ink a refused alternative already uses.
- **Costs:** it borrows no alarm and no presence either; grey italic text in an
  otherwise empty slot is also exactly what a failed fetch looks like, so the
  misreading it risks is "still loading" rather than "broken".

Crops: `renders/detail-states-q2-a-plain-{light,dark}.png`. In both schemes the
withheld cell is the quietest of the three, and the orphan cell beside it is
unmistakably the only one raising anything. The word "withheld" is doing all the
work; if a reader skips the label they get no signal at all.

### Option B — declared state

`/Users/dndungu/Code/kazi-org/dira/docs/design/openq/q2-b-declared.html`

- **Trade-off:** withheld becomes a first-class resolution state with a mark, a name
  and a chip in the instrument hue, reusing the exact grammar `tokens.css` already
  gives `.chip-staged` — something that looks designed is hard to read as broken.
- **Costs:** it spends `--bearing` on a second meaning, and the hue budget says
  three hues with one meaning each and a fourth requires a documented role, so this
  either widens bearing's definition or opens that fourth slot.

Crops: `renders/detail-states-q2-b-declared-{light,dark}.png`. This is the only
option where the state is legible at thumbnail size in both schemes. It is also the
only one where withheld competes with `.chip-id` and the invocation line for the
same hue on the same screen — visible in the whole-page row of `sheet-q2.png`,
where the page now carries bearing in four distinct roles.

### Option C — sealed material

`/Users/dndungu/Code/kazi-org/dira/docs/design/openq/q2-c-sealed.html`

- **Trade-off:** no hue is spent at all — the absence is drawn as a hatched slab at
  hairline weight, on the theory that no error state in any interface is woven, so
  "covered on purpose" arrives before a word is read.
- **Costs:** it adds a texture primitive the system does not otherwise have, and it
  is the one place in the system where a graphic stands in for content, which a
  reviewer will raise against Law 3 even though there is no content to show.

**This one fails its own claim, and the failure is scheme-dependent — exactly the
case the brief predicted.** Compare
`renders/detail-states-q2-c-sealed-light.png` against
`renders/detail-states-q2-c-sealed-dark.png`:

- **Light:** the hatch (`--rule-soft` over `--sunk`) is too low-contrast to resolve
  as a weave. The slab reads as a flat grey bar — a **skeleton loader**. The
  treatment lands on precisely the meaning it was designed to avoid.
- **Dark:** the same hatch reads at high contrast as diagonal stripes, which is
  **hazard tape**. The treatment lands on the *other* meaning it was designed to
  avoid, in the other direction.

There is presumably a hatch contrast between those two that reads as "sealed" in
both schemes, but it is not the one built here, and the two schemes want it moved in
opposite directions — structurally the same problem `--bearing-lift` had at r3 → r4.
The option is rendered and reported rather than tuned, because tuning it would be
choosing it.

---

## Method and verification

Work is confined to `/Users/dndungu/Code/kazi-org/dira/docs/design/openq/`.
`git status` confirms **zero** modifications to `tokens.css`, `screens/`,
`landing/`, `scripts/`, or `DESIGN.md`.

| Gate | Command | Result |
|---|---|---|
| Render harness (3 viewports × 2 schemes, all 6 options) | `node docs/design/openq/scripts/render-openq.mjs` | **PASS** — 36 shots, no console errors, no failed requests, no blank mounts, no byte-identical light/dark pair, no layout shift |
| Token contrast matrix | `node docs/design/scripts/contrast.mjs` | **PASS** — 42 pairs, 0 failures, hover > rest in both schemes |
| Token discipline | `node docs/design/openq/scripts/lint-openq.mjs` | **PASS** — no stray hex, every `font-size` on the 9-size scale, every `max-width` a measure ceiling |
| As-rendered contrast | built into `render-openq.mjs` | **PASS** — 26 probes, worst 4.56:1 |

`render-openq.mjs` is a copy of the real harness pointed at `openq/`, so the studies
never write into `docs/design/renders/` and no shared tooling was edited.

### One gate had to be added, and it caught a real defect

`contrast.mjs` checks tokens against tokens. Every chip in this system sits on a
`color-mix` **tint of** its surface, not on the surface, so the ratio it verifies is
not the ratio that ships. `render-openq.mjs` therefore measures each new treatment
**as rendered** in the browser, compositing every semi-transparent layer up the
ancestor chain.

It immediately failed option B: `--bearing` on a 13 % `--bearing` tint over
`--panel` measures **4.11:1** in light, under the 4.5 floor, where `--bearing` on
plain `--panel` measures 5.12:1. Option B was rebuilt at a 7 % tint (**4.74:1** in
both schemes) and now passes.

### The same defect is in the shipped screens

Running the same probe read-only against `docs/design/screens/` — **light scheme
only**, dark is clear throughout:

| Element | Screen | As rendered |
|---|---|---|
| `.chip-id` | s1-decision | **4.00:1** |
| `.arg .tag` | s1-decision | **4.09:1** |
| `.chip-staged` | s3-distill | **4.11:1** |
| `.chip-accepted` | s1-decision | **4.19:1** |
| `.chip-id` | s3-distill | **4.31:1** |

These are 11–11.5 px text, so 4.5:1 is the correct floor. DESIGN.md's claim that
"all 42 fg/bg pairs clear 4.5:1 in both schemes — 0 failures" is true of the tokens
and not true of what is drawn with them; the matrix structurally cannot see this
class. Out of scope here and **not fixed** — `tokens.css` and the screens were not
to be touched — but it wants an owner. The fix is mechanical (lower the tint
percentages) and the check already exists in `openq/scripts/render-openq.mjs`.

---

## Files

```
docs/design/openq/
  common.css                    s1-decision's layout, lifted so every option starts identical
  q2-common.css                 the three-state panel, identical across q2 options
  q1-a-run-long.html            q1 option A
  q1-b-progressive.html         q1 option B
  q1-c-summary-detail.html      q1 option C
  q2-a-plain.html               q2 option A
  q2-b-declared.html            q2 option B
  q2-c-sealed.html              q2 option C
  scripts/build-q1.mjs          the 20-alternative dataset + three emitters
  scripts/build-q2.mjs          the shared q2 page + three treatments
  scripts/render-openq.mjs      harness + the as-rendered contrast gate
  scripts/lint-openq.mjs        hex / type-scale / measure-ceiling gate
  scripts/sheets.mjs            the two comparison sheets and the detail crops
  renders/sheet-q1.png          ← question 1, one image
  renders/sheet-q2.png          ← question 2, one image
  renders/index.html            all 36 captures
  renders/detail-args-*.png     q1 refusal device at close range, one scale
  renders/detail-states-*.png   q2 three-state row, both schemes
  renders/detail-chain-*.png    q2 withheld inside the chain, both schemes
```

Rebuild everything:

```
node docs/design/openq/scripts/build-q1.mjs
node docs/design/openq/scripts/build-q2.mjs
node docs/design/openq/scripts/lint-openq.mjs
node docs/design/openq/scripts/render-openq.mjs
node docs/design/openq/scripts/sheets.mjs
```
