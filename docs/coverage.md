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

## Gaps this check found, and their state

Registering every obligation forced three admissions no plan document had surfaced.

1. ~~**`cst-0003` has no enforcement point anywhere.**~~ **CLOSED 2026-07-30** by
   `scripts/privacy-lint.py`. The privacy constraint whose violation is a *security*
   bug was a sentence in a markdown file with nothing checking it. It now has four
   invariants — no `private: true` entry in a public ledger, no private-parent label
   leaking into any committed surface, every namespaced ref resolving to a declared
   parent, and no foreign prose in a mirrored ADR — each verified red→green except P4,
   which is **vacuously green because no ADRs exist yet** and so remains untested
   against real data.

   **Rule 2 is still not covered.** The lint cannot verify that inherited context was
   never *persisted* into a child ledger, because by construction it has no access to
   the private parent. That needs a runtime assertion in the brief-injection path and
   is tracked as `blocked:0a05aa` rather than quietly folded into "covered".

2. **The ADR import has no owner.** `dec-0010` promoted `qst-0003` from a deferred
   question to a **launch blocker** — import is the day-one acquisition moment, so a
   ledger of entries with empty `alternatives` arrays would be the fatal first
   impression. But no epic in `docs/plan.md` explicitly owns it. It needs adding to
   E2's scope, and the E2 lane plan should be re-read against it.

3. **Nothing watches the revisit triggers.** Seven `revisit_if` conditions are recorded
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
| `enforce:cst-0003` | covered:privacy-lint | `scripts/privacy-lint.py` — 4 invariants, CI-gating, verified red→green on P1/P2/P3. **P4 untested against real data** (no ADRs exist yet). Rule 2 (never persist inherited context) is NOT covered — see `blocked:0a05aa` |
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
| `impl:dec-0006` | covered:E5 | unblocked by dec-0011 |
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
| `answer:qst-0002` | deferred:never | may never be built and that is acceptable. Closer to self-surveillance than navigation |
| `answer:qst-0003` | blocked:E2 | **Measured 2026-07-31, premise falsified** — 63/80 kazi ADRs carry real `why_not`s, not the predicted empty arrays. Evidence supports option (2) semantic import. Still open on neutrality: n=1 and the corpus shares an author with dira. Needs 2-3 unrelated public corpora before the import pitch ships |
| `answer:qst-0005` | covered:E9 | file the two issues; maintainer decides |
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
| `impl:dec-0011` | covered:E5/E6 | three-state resolution in the query layer (E5) + the withheld visual (E6). Config field documented in `.dira/config.toml` |
| `trigger:dec-0011:1a59c4` | deferred:trigger | fires only if a ledger must publish where the namespace itself is sensitive; the `label`-omission escape hatch already exists |
| `blocked:0a05aa` | blocked:E1/E5 | cst-0003 rule 2 needs a runtime assertion in the brief-injection path. privacy-lint cannot see it — it has no access to a private ledger by construction |
| `impl:dec-0012` | covered:E6 | server-rendered templates + `dira render`; crawlability is an E6 acceptance criterion |
| `trigger:dec-0012:78bb50` | deferred:trigger | fires if teams pull for zero-setup shared rendering (dec-0007 team tier) |
| `trigger:dec-0012:cdf95c` | deferred:trigger | fires if a surface needs genuine client-side state — a live-filtering graph explorer, not a document view |
| `impl:dec-0013` | covered:E4 | E4-L6 reduces /sitrep to a judgement layer over `dira map`; its four display invariants are in E4's acc line |
| `impl:dec-0014` | covered:E3 | lexical matcher in the binary; E3-L1 froze the labeled corpus by sha256, E3-L2 implements against it |
| `trigger:dec-0014:d477db` | deferred:trigger | fires if an additive agent-assist mode is layered over the lexical floor — the floor itself must stay model-free (dec-0003) |
| `trigger:dec-0014:554303` | deferred:trigger | fires if lexical recall plateaus below an acceptable bar; the honest response is a precision/recall curve, never editing the corpus |
| `impl:dec-0015` | covered:E1 | content-hash Version + the SQLite derived cache, landed by E1-L3 |
| `trigger:dec-0015:089e15` | deferred:trigger | condition to watch; no watch mechanism exists yet |
| `trigger:dec-0015:bee40d` | deferred:trigger | condition to watch; no watch mechanism exists yet |
| `trigger:dec-0015:c4ac11` | deferred:trigger | condition to watch; no watch mechanism exists yet |
| `trigger:dec-0015:3d524c` | deferred:trigger | condition to watch; no watch mechanism exists yet |
| `trigger:dec-0015:e334b1` | deferred:trigger | condition to watch; no watch mechanism exists yet |
| `impl:dec-0016` | covered:E6 | Pagella subsets in assets/fonts/ + NOTICE; @font-face wiring lands with the E6 surface work |
| `trigger:dec-0016:357dbd` | deferred:trigger | fires if a licence audit rules the GUST Font Licence unacceptable; Source Serif 4 is the recorded fallback |
| `impl:dec-0017` | done | summary/detail split folded into s1-decision; s1-decision-long.html is the 20-alternative proof |
| `trigger:dec-0017:24a043` | deferred:trigger | fires if a reader study shows people do scroll an untreated long decision |
| `impl:dec-0018` | done | withheld state in tokens.css + the chain; s1-decision-withheld.html is the proof |
| `trigger:dec-0018:bf198a` | deferred:trigger | fires if a hue-budget audit rejects --bearing carrying a second meaning |
| `design-q:9dcfd1` | covered:E6 | chain-at-scale; the nested-node structure takes `<details>`/`<summary>` for depth with no JS, and the long-content lane proved the idiom |
| `design-q:be981a` | blocked:E6 | **NEW, found while implementing** — s2-index has nowhere to put a cross-boundary entry. Surfaced by folding dec-0018; the index groups by intent and a withheld parent has no row to sit in |
| `blocked:4d0b4b` | blocked:E1 | E1-L5 must choose the cold-cache behaviour and E1-L6 must state which case its budget measures; measured numbers are in the row |
| `blocked:0841e6` | blocked:E6 | same finding as `design-q:be981a`, tracked from both the roadmap and DESIGN.md because it is both a blocker and an open design question |
| `answer:qst-0006` | blocked:E3 | edge vocabulary gap found by `dira why`; becomes urgent when `dira check` must reason about constraint amendment |
| `impl:dec-0019` | blocked:E6 | E6-L2 owns it; the entry is in flight as I register this. The renderer must derive the ruling from the decision title and never invent an upheld alternative the schema has no field for |
| `trigger:dec-0019:ec6b0e` | blocked:E6 | revisit trigger: a decision whose chosen option genuinely cannot be stated in its title. None exists today; if one arrives, dec-0019 is what it falsifies |
| `impl:dec-0020` | done:E1-L5 | E1-L5 owns it; in flight as I register. `brief.max_tokens` counts dira's own conservative structural estimate, never a model tokenizer |
| `trigger:dec-0020:a3e81d` | blocked:E1 | revisit trigger: dira ever has to price a real API call, where the estimate is money rather than a budget |
| `trigger:dec-0020:7f7d69` | blocked:E1 | revisit trigger: a measurement over real briefs shows the structural estimator is materially off |
| `trigger:dec-0020:e6365a` | blocked:E1 | revisit trigger: the estimator's two halves agree so consistently that one is redundant |
| `impl:dec-0021` | done:E2-L1 | built and measured: 3.7% FP on the frozen corpus, ~73% precision out of sample; `stagedOnly` wrapper makes accept structurally impossible |
| `trigger:dec-0021:3d4141` | blocked:E2 | revisit trigger: out-of-sample precision on decisions holds above 90% across several unrelated sessions; only then consider widening the kinds |
| `impl:dec-0022` | blocked:E2 | E2-L4 owns the keystroke, E2-L2 the extraction it hands to; the distill queue must show *pending extraction* as a visible state or entries pile up in staged looking rejected |
| `trigger:dec-0022:576dc1` | blocked:E2 | revisit trigger: semantic extraction proves unreliable enough that a human writes the why_not anyway, at which point the editor option returns |
| `impl:dec-0024` | blocked:E2 | E2-L4-T4 owns the transition: `n` deletes the staged entry, and `u` (E2-L4-T5) is the single-level undo the destructiveness requires. The reject signal is accepted as lost, not solved |
| `trigger:dec-0024:cc26e5` | blocked:E2 | revisit trigger: tuning `internal/sniff` needs a standing true-negative corpus that the transcripts cannot supply, at which point keeping rejects in the ledger returns |
| `trigger:dec-0024:a25d31` | blocked:E2 | revisit trigger: the semantic tier needs the expected verdict as an input before it will extract a rejected decision, which would make the split keystroke a parameter rather than a key |
| `impl:dec-0023` | blocked:E2 | E2-L2-T3 owns the delivery seam; `PreCompact` keeps the capture (it is the only point the doomed transcript still exists) and only the handoff moves, to `Stop`'s `additionalContext` or `SessionStart(compact)` |
| `impl:dec-0025` | done:E2-L4-T2 | `y` writes `confirmed_by: human` and a bumped `updated`; state stays `staged`, every other byte unchanged. No schema change |
| `trigger:dec-0025:6d5634` | blocked:E2 | revisit trigger: a surface needs to tell extraction-pending from human-pending in a way `confirmed_by`'s presence cannot express |
| `trigger:dec-0025:c8b987` | blocked:E2 | revisit trigger: semantic extraction proves unreliable enough that a human writes the why_not by hand anyway |
| `impl:dec-0026` | done | `coldMaxBudget` removed from `internal/perf`; the cold gate asserts the median alone. CI comments updated; the tail is still published to the step summary, asserted by nothing |
| `trigger:dec-0026:a08865` | blocked:E1 | revisit trigger: a measured distribution over many CI runs exists, so a percentile ceiling can be DERIVED rather than chosen. E1-L6-T5's phase attribution is the natural place |
