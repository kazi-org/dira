# E8-L2 report — landing page built and gated

**Lane:** E8-L2. **Executed by:** delegate agent, dispatched by the team lead,
2026-07-30. **Status:** done — all four `acc:` predicates green, verified live, not
self-reported.

---

## What was built

- `docs/design/landing/index.html` — the landing page, on `../tokens.css`, reusing
  the Bearing system's own components (`.invoke`, `.chain`, `.card`, `.chip`, `.btn`,
  `.made` from tokens.css; `.legend-key` and `.take` copied verbatim from
  `s2-index.html`, which are screen-local and not in tokens.css). Structure: hero
  (tagline + hook) → the `dira check` catch as the demo, in real selectable text →
  a decode legend for `dec-`/`int-` → three inversions (struck-through old way /
  emphasized dira way, reusing s1's refused/upheld contrast technique compacted to
  one row each) → two proof pillars → four objections → an honest status block →
  one restrained conversion block (star the repo — no "get started" on software that
  doesn't exist).
- `docs/design/landing/canonical.mjs` — five canonical strings (`HOOK`, `TAGLINE`,
  `NO_BINARY`, `INSTALL_LINE`, `CATEGORY`), each declaring which source doc(s) it must
  match verbatim.
- `docs/design/scripts/check-coherence.mjs` — new. Reads README.md,
  `.agents/product-marketing.md`, and the page; normalizes each (markdown
  blockquote/backtick/bold stripping for the docs, tag-stripping for the page) and
  asserts every canonical string appears verbatim in its declared source(s). Verified
  two-sided: edited `TAGLINE` in `canonical.mjs` to a wrong value, got a named
  `COHERENCE FAIL` citing all three surfaces; reverted, got `COHERENCE PASS`.
- `docs/design/scripts/render.mjs` — extended `TARGETS` to enumerate `landing/`
  (renamed `index` → `landing` so `render.mjs r5 landing` filters correctly). No fork.
- `docs/design/scripts/contrast.mjs` — **not touched.** Confirmed already committed
  and passing before starting (per the STALE-SPEC CORRECTION at the top of
  `docs/plan/prompts/L2-E8-L2.md`), run as an inherited gate.
- `docs/plan/tasks/E8-L2.md` — the 6-task leaf breakdown, written after the fact
  since this session executed directly rather than handing the plan to another agent
  (see the deviation note in that file).

## The four acc predicates, run and green

```
node docs/design/scripts/render.mjs r5 landing
  → GATE PASS — no console errors, no failed requests, no blank mounts, no fake dark, no layout shift.
    (6 png files written: mobile/laptop/wide × light/dark)

node docs/design/scripts/contrast.mjs
  → CONTRAST PASS — 42 pairs, 2 schemes, 0 failures, hover exceeds rest on all six surfaces.

grep -oE '#[0-9a-fA-F]{3,6}' docs/design/landing/index.html
  → #0f151c
  → #f7f4ed
  (only the two sanctioned theme-color hexes; nothing else)

node docs/design/scripts/check-coherence.mjs
  → COHERENCE PASS — hook, tagline, install line, no-binary status line, and
    category sentence all agree across README, product-marketing.md, and the page.
```

Both screens (light + dark, all three viewports) were also visually reviewed from the
rendered PNGs, not just gate-checked. No layout breaks; the demo chain block scrolls
horizontally at mobile width, matching s1's own existing precedent for the why-chain
tree rather than a new defect.

## A real finding: the repo moved under me mid-task

README.md's status blockquote changed **while this lane was in progress** — some
other concurrent session (E0-L1, by the timestamp) landed a Go-module bootstrap. The
version I read at the start of this session said:

> **Status: design phase.** There is no binary yet. ... `brew install` is not a
> thing yet.

The version on disk by the time I wrote `canonical.mjs` says:

> **Status: design phase.** What exists is ... a Go module you can build from source.
> The binary currently answers `--help` and `--version` and nothing else: no `log`,
> no `why`, no `brief` yet. ... `brew install` is not a thing yet.

This directly touches the team lead's absolute rule #1 ("the page must not claim
installable software exists... it carries an honest status line saying so"). That
instruction was accurate when written and became stale within the same session. I
updated `canonical.mjs`'s `NO_BINARY` export and the page's status copy to the
**current** README wording rather than hardcoding the now-false "there is no binary"
claim — a canonical string that no longer matched its own source would have been
exactly the kind of drift this gate exists to catch. The substance of the absolute
rule still holds: the page does not claim a working install exists (`brew install` is
still "not a thing yet"; `brew install kazi-org/tap/dira` still fails; the CLI still
does nothing beyond `--help`/`--version`). Only the specific wording changed to track
reality.

**This is worth flagging to whoever owns E0/E8 sequencing:** the coherence gate is
now load-bearing against a status line that will keep changing as E0 lands more
commands. Each time README's status blockquote gains a working verb (`log`, `why`,
`brief`, ...), `canonical.mjs`'s `NO_BINARY` string needs a matching edit or the gate
will start failing — correctly, but someone has to notice and fix both sides
together, exactly as the script's own error message says.

## Deliberate content decisions (drift guards honored, not just inherited)

1. **Never the `init` clip.** The page's only demo is the `dira check` catch from
   product-marketing §6. No `dira init` clip, no "point it at your existing repo"
   framing anywhere on the page.
2. **Proof pillar 2 dropped.** Product-marketing §5 lists three proof pillars; pillar
   2 ("it reads what you already have — point it at an existing repo and it builds
   the graph from your ADRs") is exactly the import/retrospective-value claim the
   team lead's absolute rule #2 excludes, since `qst-0003` (does importing an ADR
   corpus produce a useful ledger or a second pile?) is open and unowned. The page
   ships with two pillars, not three. This is a deliberate content gap, not an
   oversight — noted so nobody "fixes" it back to three without re-checking
   `qst-0003`'s status first.
3. **The "I already have ADRs" objection reworded.** Product-marketing §9's answer —
   "dira indexes them and pushes them to you instead" — asserts the same unresolved
   import capability as a settled fact. Reworded to: "Whether indexing an existing
   pile produces a useful ledger or just a second pile is an open question the
   project is still working through, not a shipped feature yet." Same honest number
   (83 ADRs), no functionality claim.
4. **cst-0004 additive framing:** not spelled out at length on the page itself (the
   page never mentions a hosted renderer at all, so the "additive, not required"
   distinction dec-0010 asks for doesn't come up — there's nothing on this page that
   could be mistaken for a hosted-tier claim). Flagging in case a future editor adds
   renderer copy here without re-reading `dec-0010`'s explicit instruction to state
   that distinction wherever the renderer is mentioned.

## Anything in the source docs I think is wrong

- `.agents/product-marketing.md` §5's proof pillar 2 and §9's ADR objection answer
  both quietly assume `qst-0003` will resolve in dira's favor. That's a real
  inconsistency between a "settled" doc (v1.0.0, no changelog entry acknowledging
  this) and a question the same doc's own dec-0010 cross-reference calls "OPEN and
  unowned." Whoever owns product-marketing.md should either resolve qst-0003 or add
  an explicit hedge to §5/§9 rather than leaving two sections that read as certain
  about something the ledger itself records as undecided.
- Everything else read (README, DESIGN.md, tokens.css, the three screens, dec-0010,
  cst-0004, dec-0007, int-0001) was internally consistent and did not need
  correction.

## What was not done, and why (in scope, correctly)

Nothing was left out of this lane's scope. Not touched, per the team lead's
instructions: `docs/roadmap.md`, `docs/coverage.md`, `go.mod`, `cmd/`, any `.go` file,
no deploy, no domain, no push, no commit.
