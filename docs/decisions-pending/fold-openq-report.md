# Folding the two chosen treatments into the real screens

**Date:** 2026-07-30. **Nothing committed, nothing pushed.**
Both chosen options are folded in and all four gates pass. Three things I
believe are wrong or unfinished are in the last section, and one of them is a
blocker for the Go renderer.

---

## What changed

| File | State | What |
|---|---|---|
| `docs/design/tokens.css` | modified | withheld promoted to a first-class state: `.chip-withheld`, `.chain .id-w`, `.chain .wh-tag`, `.res-oriented` / `.res-withheld` / `.res-orphan`, and the hue-budget exception written down where the budget is declared |
| `docs/design/screens/decision.css` | **new** | the decision page's layout, shared by all three `s1-*` screens. Includes the summary/detail alternatives component |
| `docs/design/screens/s1-decision.html` | modified | Q1 folded in at today's four alternatives |
| `docs/design/screens/s1-decision-long.html` | **new** | Q1 at twenty alternatives, one of them 405 words |
| `docs/design/screens/s1-decision-withheld.html` | **new** | Q2, `dec-0011`, both founder amendments applied |
| `docs/design/screens/s2-index.html` | modified | **one line**: `dec-0002` now links to the long screen and its label matches that page's ruling |
| `docs/design/DESIGN.md` | modified | both answers recorded, screens table, Law 1 grep test widened, open questions updated |

Untouched, as instructed: `.dira/entries/`, `docs/roadmap.md`, `docs/coverage.md`,
`scripts/`, `assets/logo/`, `--serif`, and `docs/design/scripts/`.

```
$ git status --porcelain   (renders/ elided)
 M docs/design/DESIGN.md
 M docs/design/screens/s1-decision.html
 M docs/design/screens/s2-index.html
 M docs/design/tokens.css
?? docs/design/screens/decision.css
?? docs/design/screens/s1-decision-long.html
?? docs/design/screens/s1-decision-withheld.html
```

---

## Q1 — the summary/detail split

Every alternative is a `<details>`. The summary carries the caret, the mark, the
struck name, the state tag and **one line of ground**. The full reasoning is
inside. Zero JavaScript.

**The refusal device survives, which was the condition.** The strike-through
lives on the *summary* line, which is always visible: a refused alternative reads
as refused whether the entry is open or closed, and its grounds are one keystroke
away rather than gone. The reviewer's warning was against a comparison list that
*replaces* the struck-refusal-with-grounds; this suspends it, and at four
alternatives it does not even do that.

### The degradation rule

Decided at render time, not in CSS — CSS cannot count siblings honestly.

| alternatives | emitted |
|---|---|
| **≤ 6** | every `<details open>`. Nothing hidden. The page is the argument it always was, with a hinge added. |
| **> 6** | only the upheld one open. The page becomes an index that expands in place. |

Six, because the study measured the reading thread breaking at roughly the sixth
refusal. It is one comparison in the Go renderer.

At today's four, compare `renders/d2-s1-decision-laptop-light.png` (before) with
`renders/f1-s1-decision-laptop-light.png` (after): same struck names, same
grounds directly beneath, plus a scan line and a caret.

### Measured

| | mobile 390 | laptop 1024 | wide 1440 |
|---|---|---|---|
| `s1-decision` (4, all open) | 3,257 px (3.9 screens) | 2,162 px (2.8) | 2,162 px (2.4) |
| `s1-decision-long` (20, one open) | 4,643 px (5.5) | 2,980 px (3.9) | 2,980 px (3.3) |
| `s1-decision-withheld` (3, all open) | 3,552 px (4.2) | 2,226 px (2.9) | 2,188 px (2.4) |

Twenty alternatives cost **818 px more than four** at laptop. Letting it run cost
5,316 px.

---

## Q2 — the declared state, with both amendments

**Amendment 1 applied.** The mark is `⊙` in `--bearing`, everywhere it appears:
the chain, the chip, the state panel, the edges rail.

**Amendment 2 applied.** The chain row was

```
sire:int-0002  private ledger — ref published, body not      ⊙ withheld
```

and is now

```
sire:int-0002  private ledger                                ⊙ withheld
```

The plain-language explanation appears exactly once, in
`Arising from sire:int-0002 — a parent in a ledger this repo declares but cannot
show you.` See `renders/f1-s1-decision-withheld-laptop-light.png`.

**I applied that same trim once more, unprompted, and you should know where.**
The study's withheld cell carried the state four times: the `WITHHELD` label, the
chip, a serif line reading *"a parent this repo names and does not publish"*, and
the note *"Edge present, parent namespace declared private. Not drift."* I
removed the serif line. The chip now occupies the slot where the other two cells
put the parent's title, which is the point being made — where a title would be,
there is a chip. If you want the serif line back, it is one `<div>`.

### The three states are first-class

In `tokens.css`, with the law restated on this axis:

```
oriented   parent present and readable here        ✓  --converged
withheld   parent declared, body not published     ⊙  --bearing
orphan     no parent recorded at all               ⚠  --caught   ← drift
```

**Only orphan is drift and only orphan uses `--caught`.** Withheld is drawn at
chip weight and never gets a tinted card surface; orphan is the only cell on the
page that recolours its own ground. Law 1's grep test in DESIGN.md now names
`.res-orphan` and `.st-orphan` alongside `.drift`, and it passes:

```
$ grep -rn 'var(--caught)' docs/design/tokens.css docs/design/screens/*.css docs/design/screens/*.html
  tokens.css:325   .res-orphan .mk, .res-orphan .nm
  tokens.css:329   .drift border
  tokens.css:330   .drift background
  tokens.css:333   .drift h3, .drift .label
  tokens.css:338   .drift p b
  tokens.css:339   .drift b
  decision.css:208 .st-orphan border-color
  decision.css:209 .st-orphan background
  s2-index.html:122 .drift a
```

### `--bearing` now carries a second documented meaning

Stated in three places — the hue-budget comment in `tokens.css`, the hue table in
DESIGN.md, and here.

The defence is that withheld is the instrument pointing at something it can name
and cannot open: the needle is still on it. Every alternative was worse —
`--caught` is forbidden, `--converged` would claim a resolution that did not
happen, and a fourth hue would have to earn a permanent slot for a state whose
entire brief is to be unremarkable.

**The cost, stated:** `s1-decision-withheld.html` carries bearing in four roles at
once — the invocation argument, the id chips, the links, and the withheld mark.
That is the thing the one-meaning-per-hue rule exists to prevent, and it is now
visible on a real screen rather than only in a study.

### Verified by looking, not by markup

- `f1-s1-decision-withheld-laptop-light.png` / `-wide-dark.png` — the withheld
  row is **not** the heaviest thing in the chain. `dec-0011`'s title outweighs
  it, because the title is `--ink` and the withheld row is two quiet bearing
  spans on `--sunk`.
- The three-state row in both schemes — the orphan cell is unmistakably the only
  one raising anything; the withheld cell sits on the same untinted panel as
  oriented. It reads as neither error nor warning.
- The drift card is two columns away in the rail, still the loudest thing on the
  page. That adjacency is the test and it holds.

---

## Gates — verbatim

```
$ node docs/design/scripts/contrast.mjs

42 ink/surface pairs checked across 2 schemes, plus 6 hover>rest assertions.
CONTRAST PASS — every pair clears 4.5:1 and hover exceeds rest in both schemes.
  -> exit 0

$ node docs/design/scripts/contrast-rendered.mjs

1150 text nodes measured as composited across 6 surfaces x 2 schemes.
RENDERED CONTRAST PASS — every text node clears its floor as actually drawn.
  -> exit 0

$ node docs/design/scripts/check-coherence.mjs
5 canonical strings checked across 3 surfaces.
COHERENCE PASS — hook, tagline, install line, no-binary status line, and category sentence all agree across README, product-marketing.md, and the landing page.
  -> exit 0

$ node docs/design/scripts/render.mjs f1 s1-

captured 18 shots for 3 target(s) -> docs/design/renders/f1-index.html
GATE PASS — no console errors, no failed requests, no blank mounts, no fake dark, no layout shift.
  -> exit 0

$ node docs/design/scripts/render.mjs f1 s2-

captured 24 shots for 1 target(s) -> docs/design/renders/f1-index.html
GATE PASS — no console errors, no failed requests, no blank mounts, no fake dark, no layout shift.
  -> exit 0

$ node docs/design/scripts/render.mjs f1

captured 54 shots for 9 target(s) -> docs/design/renders/f1-index.html
GATE PASS — no console errors, no failed requests, no blank mounts, no fake dark, no layout shift.
  -> exit 0
```

Token discipline, checked by hand because no screens-scoped lint exists:

```
$ grep -ohE '#[0-9a-fA-F]{3,6}' docs/design/screens/*.html docs/design/screens/*.css
#0f151c
#f7f4ed          (the two sanctioned <meta name="theme-color"> values only)
```

Every `font-size` in the new CSS and the three screens is one of the nine scale
tokens — no tenth size. Every text `max-width` is an `--m-*` ceiling, with two
carried-over exceptions that predate this change and are not text measures:
`h1.ruling { max-width: 24ch }` and `.wrap { max-width: 1100px }`.

### PNGs

Contact sheet: `docs/design/renders/f1-index.html`

```
docs/design/renders/f1-s1-decision-{mobile,laptop,wide}-{light,dark}.png
docs/design/renders/f1-s1-decision-long-{mobile,laptop,wide}-{light,dark}.png
docs/design/renders/f1-s1-decision-withheld-{mobile,laptop,wide}-{light,dark}.png
docs/design/renders/f1-s2-index-{mobile,laptop,wide}-{light,dark}.png
```

---

## Three defects in the chosen options, found by folding them in

All three passed the study's own gates. Two were invisible at desktop.

**1. `.st-orphan` fails as-composited in dark.** The study built the orphan cell
at a 7 % `--caught` tint over `--panel`. `--ink-low` on that surface measures
**4.41:1** in dark — under the floor — and `contrast-rendered.mjs` failed it on
the first run here (two nodes: `span.k`, `div.stnote`). Dropped to 3 %, which is
what `.drift` in `tokens.css` already uses. The visual difference is negligible;
the red border was always doing the work.

**2. `.alt .one` was an inline `<span>`, so `padding-left` indented only its first
line.** Invisible at desktop, where the one-liner is one line. At every mobile
width it wrapped and produced a hanging first line. Fixed with `display: block`.

**3. The study's headline mobile objection to option C was partly a layout bug,
not a cost of the treatment.** `.line1` is `display:flex; flex-wrap:wrap` and the
name had `flex-basis:auto`. Flexbox assigns items to lines using their
*hypothetical* size — max-content for `basis:auto` — **before** it shrinks
anything, so any title that did not fit was pushed to a new flex line, stranding
the caret and the mark alone on a line of their own. At 390 px roughly half the
twenty rows did this.

Fixed with `flex: 1 1 0` at ≤767px, so the name contributes nothing to the
line-fitting decision and then grows into whatever is left, wrapping inside its
own column with the tag held at the end.

The study reported option C at 4,997 px on mobile against option B's 4,652 px and
concluded "the scan value that justifies the treatment is a desktop-only
benefit". With the bug fixed the same page measures **4,643 px** — level with B.
That conclusion should be retired.

---

## What I think is wrong, or unfinished

**1. BLOCKER for the renderer: the schema has no field for the one-line ground.**
`schema/entry.schema.json` gives an alternative exactly three fields — `option`,
`why_not`, `revisit_if`. The chosen treatment requires a fourth, and the whole
value of the summary line depends on it being *authored*, not derived.

I hit this while writing this fold. My first draft of the four one-liners on
`s1-decision.html` paraphrased the opening of each `why_not`, and the open state
read the same claim twice in a row — the exact defect Amendment 2 exists to
prevent, reproduced elsewhere on the same page. I rewrote them as compressed
judgements ("the better guarantees, in the language fewer drive-by contributors
will open"). An agent that generates the field by truncating `why_not` will
reintroduce it on every entry.

So this needs a schema field with a description that says what it is for, and it
wants a `dira check` rule that fails when the summary is a prefix or a substring
of the grounds. I did not touch the schema — out of scope, and it is a real
decision rather than a mechanical addition.

**2. The `REFUSED` tag repeats nineteen times at twenty rows, and I shipped it
anyway.** The study flagged this and I did not act on it, because acting on it is
a redesign of what you chose. Look at `f1-s1-decision-long-laptop-light.png` and
judge: nineteen identical uppercase pills against a `✗` and a strike-through that
already say the same thing. If you want it gone, the safe form is to drop the tag
on *refused* rows only above the disclosure threshold and keep it on the upheld
one — `✗` plus the strike are two non-colour carriers, so WEB.md 12 still holds.
One CSS rule and one renderer condition. Your call, not mine.

**3. `s2-index` has nowhere to put a cross-boundary entry.** Surfaced by this
work, recorded as open question 4 in DESIGN.md. The index groups by intent, and
`dec-0011`'s parent is `sire:int-0002` — an intent in a private ledger, which by
construction cannot be a group heading in this repo. So `s1-decision-withheld.html`
is reachable only by direct link. Either the index grows a "from another ledger"
group, or cross-boundary entries are accepted as index-invisible — which weakens
the acquisition surface `dec-0010` depends on.

**4. Neither choice is wrong.** Q1 option C is the only one of the three that
gives a reader a map, and the objection that mattered — that it destroys the
refusal device — does not survive contact with `<details>` once the strike is on
the summary line. Q2 option B is the only option legible at thumbnail size in
both schemes, and option C's hatch genuinely fails in opposite directions per
scheme. I would have chosen the same two.

**5. One thing I did without being asked, flagged rather than buried.**
`s2-index.html`'s `dec-0002` row said "One file per entry, not an append-only
log" while the page it now links to rules "Entries are markdown files in the repo,
not rows in a database". Both are true of `dec-0002`; only one matches the detail
page. I changed the label and pointed the link at `s1-decision-long.html`. It is
one line and it is trivially revertible.
