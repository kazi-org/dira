# dira — coverage register

**The guarantee that nothing is forgotten.** Not this file — the *check*.

`scripts/coverage.py` extracts every obligation **mechanically** from the repo's
structured sources and exits non-zero if any obligation has no disposition here, or
if a row here points at an obligation that no longer exists.

```
python3 scripts/coverage.py          # non-zero on any gap
python3 scripts/coverage.py --list   # what it extracted
python3 scripts/coverage.py --stub   # register rows for anything uncovered
```

The asymmetry is the whole design: **the register is hand-maintained, the obligation
list is not.** A human can forget to add a row; the checker catches it. If both sides
were hand-written this would guarantee nothing, which is exactly why a plan document
alone guarantees nothing.

## Where obligations come from

| Source | Rule |
|---|---|
| `.dira/entries/*.md` | accepted decision → must be implemented · open question → answered or parked · active constraint → must have an enforcement point · active intent → must be served · every `revisit_if` → a trigger to watch |
| `docs/design/DESIGN.md` | every open design question |
| `docs/roadmap.md` | every Blocked row, every upstream ask |
| `docs/plan.md` | every epic must have lanes |

Add an extractor to `coverage.py` and the checker immediately demands dispositions.

## Dispositions

`done` · `covered:<where>` · `deferred:<reason>` · `blocked:<what>`

A disposition is a claim someone can check. `blocked` is legitimate — a known blocker
is not a forgotten item — but it must name the blocker.

## The honest limit

This guarantees nothing is forgotten **from the sources it reads.** It cannot
guarantee something never written down anywhere is remembered. The mitigation is
structural: those sources are exactly where the capture hooks write (E2), so new
decisions enter scope automatically instead of needing to be remembered. Until E2
ships, that mitigation is a plan, not a fact.

---

## Gaps this check found on its first run

Registering all 46 obligations forced three admissions that no plan document had
surfaced. Each is `blocked` in the register rather than quietly `covered`.

1. **`cst-0003` has no enforcement point anywhere.** The privacy constraint —
   inheritance is one-way and read-time only, and a violation is a *security* bug
   rather than a UX bug — is currently a sentence in a markdown file with nothing
   checking it. A private strategic note committed into public git history cannot be
   un-published, so this is the highest-severity gap in the register. E3 covers the
   "cite the ref, never the text" half; the tier-crossing half waits on `qst-0001`.

2. **The ADR import has no owner.** `dec-0010` promoted `qst-0003` from a deferred
   question to a **launch blocker** — import is the day-one acquisition moment, so a
   ledger of entries with empty `alternatives` arrays would be the fatal first
   impression. But no epic in `docs/plan.md` explicitly owns it. It needs adding to
   E2's scope, and the E2 lane plan should be re-read against it.

3. **Nothing watches the revisit triggers.** Six `revisit_if` conditions are recorded
   across the decisions — "if entry volume passes 10k", "if dira grows a long-lived
   component", "if it escapes the terminal audience". They are inert text. The whole
   point of recording a revisit condition is that someone notices when it fires, and
   right now nobody would. This is arguably a *product* gap as much as a process one:
   `dira brief` surfacing fired triggers is the natural home for it, and no entry
   proposes that yet.

---

## Register

| Obligation | Disposition | Note |
|---|---|---|
| `enforce:cst-0001` | covered:E1 | brief cap enforced in-binary; drop-by-priority test |
| `enforce:cst-0002` | covered:E0 | schema validator test pins the five-kind enum in CI |
| `enforce:cst-0003` | blocked:qst-0001 | **GAP** no enforcement exists. Privacy leak = security bug. E3 cites refs only; the tier-crossing case waits on qst-0001 |
| `enforce:cst-0004` | covered:E6 | network-unplugged test on CLI + renderer |
| `impl:dec-0001` | covered:E0 | Go module, goreleaser, tap formula |
| `trigger:dec-0001:0af862` | deferred:trigger | not work — a condition to watch. **No watch mechanism exists yet** (see gap list) |
| `impl:dec-0002` | covered:E1 | one-file-per-entry read/write |
| `trigger:dec-0002:8d9b6a` | deferred:trigger | not work — a condition to watch. **No watch mechanism exists yet** (see gap list) |
| `impl:dec-0003` | covered:E2 | regex tier stages only; no model client in binary |
| `trigger:dec-0003:b48028` | deferred:trigger | not work — a condition to watch. **No watch mechanism exists yet** (see gap list) |
| `impl:dec-0004` | covered:E4 | kazi join; test greps entries for status fields |
| `impl:dec-0005` | covered:E1 | local backend now; github backend is E7 |
| `trigger:dec-0005:d7ef58` | deferred:trigger | not work — a condition to watch. **No watch mechanism exists yet** (see gap list) |
| `impl:dec-0006` | blocked:qst-0001 | fractal tiers are E5, blocked |
| `trigger:dec-0006:0d9a33` | deferred:trigger | not work — a condition to watch. **No watch mechanism exists yet** (see gap list) |
| `impl:dec-0007` | deferred:E7/E8 | business model. Pricing needs no code until apps exist |
| `trigger:dec-0007:439d04` | deferred:trigger | not work — a condition to watch. **No watch mechanism exists yet** (see gap list) |
| `impl:dec-0008` | covered:E2/E4/E9 | install-hooks in E2, join in E4, upstream asks in E9 |
| `impl:dec-0009` | covered:E2 | ADR mirror on accept |
| `impl:dec-0010` | covered:E6 | the public renderer |
| `trigger:dec-0010:29bcee` | deferred:trigger | not work — a condition to watch. **No watch mechanism exists yet** (see gap list) |
| `serve:int-0001` | covered:E1/E2/E3 | brief + capture + enforcer are the whole intent |
| `serve:int-0002` | covered:E0/E1 | single binary, <100ms cold as an acc: line |
| `serve:int-0003` | covered:E4/E8 | derived status replaces the tracker; GTM proves adoption |
| `answer:qst-0001` | covered:E5 | E5-L1 is answering it; agent asked for an argued recommendation |
| `answer:qst-0002` | deferred:never | may never be built and that is acceptable. Closer to self-surveillance than navigation |
| `answer:qst-0003` | blocked:E2 | **GAP** dec-0010 made import a LAUNCH BLOCKER but no epic explicitly owns it. Needs adding to E2 scope |
| `answer:qst-0005` | covered:E9 | file the two issues; maintainer decides |
| `design-q:1` | covered:E6 | chain-at-scale collapse rule |
| `design-q:2` | covered:E6 | long-content verification |
| `design-q:3` | blocked:qst-0001 | private-parent state cannot be designed until the model is decided |
| `blocked:eeabdf` | blocked:qst-0001 | M5 workspace tier |
| `blocked:4784fa` | covered:E9 | automatic disposition capture = upstream ask 2; skill-wrap fallback exists |
| `blocked:2147c3` | blocked:E2 | ADR import — same GAP as answer:qst-0003 |
| `upstream:1` | covered:E9 | portfolio schema registration |
| `upstream:2` | covered:E9 | post-disposition hook, direct not bus |
| `lanes:E0` | covered:E0 | L1 agent dispatched 2026-07-29 |
| `lanes:E1` | covered:E1 | L1 agent dispatched 2026-07-29 |
| `lanes:E2` | covered:E2 | L1 agent dispatched 2026-07-29 |
| `lanes:E3` | covered:E3 | L1 agent dispatched 2026-07-29 |
| `lanes:E4` | covered:E4 | L1 agent dispatched 2026-07-29 |
| `lanes:E5` | covered:E5 | lane-level only by design; no L2 (blocked / gated) |
| `lanes:E6` | covered:E6 | L1 agent dispatched 2026-07-29 |
| `lanes:E7` | covered:E7 | lane-level only by design; no L2 (blocked / gated) |
| `lanes:E8` | covered:E8 | L1 agent dispatched 2026-07-29 |
| `lanes:E9` | covered:E9 | L1 agent dispatched 2026-07-29 |
