# dira — roadmap

Living document. Updated on every merge, lane claim, new blocker, and decision.

**Last updated:** 2026-07-29 · **Owner:** maintainer

> **Nothing is forgotten by construction.** `python3 scripts/coverage.py` extracts
> every obligation mechanically from `.dira/entries/`, `DESIGN.md`, this file, and
> `docs/plan.md`, then fails if any lacks a disposition in
> [`docs/coverage.md`](coverage.md). Adding a Blocked row or an upstream ask here
> creates an obligation the check will demand you account for. It becomes a CI gate
> in E0.

---

## Shipped

| What | Landed |
|---|---|
| **A binary that builds and installs from source.** Go module, command skeleton, correct exit codes (0 ok / 1 error / 2 usage), and a test that mechanically forbids third-party packages in the command path — `dec-0001` chose Go for cold-start latency, and that reasoning is now a build failure rather than prose. | E0-L1 |
| **The ledger is self-validating.** 17 invalid fixtures covering the cases that actually bite, including the yaml timestamp coercion that broke this repo once. All 30 entries validate. | E0-L1 / E0-L2 |
| **`dira log` writes decisions.** 32 concurrent invocations produce 32 distinct ids with zero overwrites; adding an edge alters 4 lines with zero deletions, so a PR touching a record shows a legible diff. | E1-L2 |
| **`dira reindex` rebuilds a content-addressed cache.** A stale cache is impossible rather than improbable (`dec-0015`). | E1-L3 |
| **The storage layer.** Interface with a portable contract suite for the future GitHub backend, and a codec that preserves the author's hand-wrapped prose. | E1-L1 |
| **The design system, settled.** Direction validated by three independent critics, connectors reconnected, measure ceilings, type scale 16→9, a self-hosted serif chosen from rendered evidence (`dec-0016`), a compass mark, and both remaining open questions answered from pictures (`dec-0017`, `dec-0018`). | E6 pre-work |
| **The enforcer's corpus**, 43 labelled rows frozen by sha256 before any matcher existed. | E3-L1 |
| **Go-to-market groundwork.** 19 channels rated with pre-registered thresholds, a gated landing page, approval-gated ecosystem drafts. | E8-L1/L2/L5 |
| **`dira why` answers the question the product exists to answer.** The spine above an entry, every rejected alternative with its grounds, and the `revisit if` that would reopen it. Box-drawing tree on wide terminals, stacked form on narrow. It refuses to print kazi convergence — `dec-0003` gives it no client, so a test asserts the output contains no check mark and no predicate count. 16.4ms median, of which 15.9ms is opening the index. | E1-L4, 2026-07-31 |
| **`dira check` refuses a plan that contradicts a settled decision.** Lexical, in-process, no model and no network — the property that makes it usable from a hook. Exit 2 is a verdict, 1 always means the check did not run, and `check` routes its own usage errors to 1 so the two can never collide. Verified through a real process: conflict → 2 citing the `why_not` and `revisit_if`, compliant → 0, decision superseded → 0 with nothing citing it. | E3-L2, 2026-07-31 |
| **CI runs every gate on every push**, and refuses a green run over an empty suite. Its first run caught what one laptop structurally could not: the platform allowlist passed on macOS and failed on ubuntu, because it had only ever seen its own host. | E0-L3, 2026-07-31 |
| **The design fidelity gate** — nine gates behind one command, two of them negative controls, on an exit code that separates "a gate failed" from "a gate passed but its control did not trip". The tolerance it measures against is recorded evidence, not an assertion. Its `fixture-content` gate holds an 18-entry fixture ledger byte-equal to the mockups, so a pixel diff measures layout and never drifted prose. | E6-L1, 2026-07-31 |
| **Four machine gates**, all enforced by the pre-commit hook rather than by memory: coverage, privacy lint, contrast, and contrast-as-rendered. | — |

---

## In progress

| Item | Owner | Notes |
|---|---|---|
| **`dira brief` + SessionStart injection** (E1-L5) | agent | dispatched 2026-07-31. M1's exit criterion runs through this. Also owns the multi-word resolver fix. |
| **`dira supersede`** (E3-L4) | agent | dispatched 2026-07-31. The red-to-green enforcement flip, currently only exercised by hand. |
| **Serve the read-only surfaces from the binary** (E6-L2) | agent | dispatched 2026-07-31. First job is deciding where the upheld-option card comes from — the fidelity gate routed that question here. |
| **`dira sniff`, the regex capture tier** (E2-L1) | agent | dispatched 2026-07-31. May only ever stage, never accept. |
| Privacy enforcement + coverage gate | maintainer | 2026-07-30. `scripts/coverage.py` (nothing-forgotten gate, 47 obligations) and `scripts/privacy-lint.py` (cst-0003 enforcement, 4 invariants). Both verified red→green. |
| Founding design + repo scaffold | maintainer | 2026-07-29. Design doc v2, entry schema, founding ledger (21 entries — 3 intents, 9 decisions, 4 constraints, 5 questions; all validate against the schema, no dangling edges), hook config, license. |

---

## In flight (PRs open)

None.

---

## Planned

**Found by building the readers, 2026-07-31 — both on the stranger path, which is the growth engine (`note-0001`):**

- **Multi-word queries only match as a contiguous substring.** `dira why "read time"`
  resolves; `dira why "status derived"` finds nothing, though both words are in the
  title. A stranger types the question in their own word order, not the author's. Fix
  stays inside `dec-0014`: require all tokens present, rank contiguous matches first so
  today's top result never moves. No embeddings.
- **The `s1-decision` mockup renders an upheld alternative the schema has no room for.**
  It shows `✓ Go` as one of four; the real `dec-0001` records four *rejections* and no
  upheld row, because in this schema the chosen option is the decision title. Either
  the schema gains a way to mark it or the mockup drops the row — E6's call, and its
  first concrete test case.
- **The edge set cannot say "narrows without replacing"** (`qst-0006`), so authors reach
  for `supersedes` and contradict the target's own state. Three instances existed in
  this ledger. Becomes urgent when `dira check` must reason about constraint amendment.



Milestones, ordered so that **M1 alone is already more useful than any alternative**
surveyed in `docs/design.md` §1.1. Each milestone's exit criterion is behavioural, not
a checklist of files.

### M1 — the ledger
Entry schema (done) · storage interface with the `local` backend · `dira log`, `why`,
`brief`, `reindex` · SQLite derived cache · `SessionStart` brief injection.

**Exit:** `dira brief` renders in well under 100ms on a cold cache, and one real
decision made in week 1 correctly shapes a session in week 2 without anyone opening a
file.

### M2 — capture
`dira sniff` (regex tier) · the Claude Code skill (semantic tier) · `Stop` and
`PreCompact` hooks · staged-entry disposition flow.

**Exit:** a four-hour session's decisions land in the ledger without being typed, and
a compaction event loses nothing.

### M3 — the enforcer
`dira check` · `dira supersede` · constraint inheritance down the tier chain.

**Exit:** one genuine relitigation attempt is caught and cited back with its own
recorded why_not, before predicates are drafted.

### M4 — derived status
`kazi portfolio --json` join · `dira map` · decision-blocked detection · graceful
degradation when kazi is absent.

**Exit:** `dira map` matches reality with zero hand-entered status, and correctly
distinguishes execution-blocked from decision-blocked.

### M5 — tiers
Workspace and personal ledgers · namespaced ref resolution · orphan-work drift flag ·
`dira init --interview`.

**Exit:** "what are we actually doing on Sire?" is answered by a derived report rather
than reconstructed from memory. **Gated on qst-0001.**

### M6 — surfaces & distribution
`dira ui` as server-rendered Go templates + `embed.FS`, and `dira render` emitting
static HTML the user deploys — **not an SPA and no dira-operated host** (`dec-0012`).
The public ledger renderer is the growth engine (`dec-0010`).

Pulled ahead of the apps because it is the growth engine, not a nicety: dira is
invisible by design, so without a rendered artifact there is nothing to screenshot,
link, or land on — and a solo maintainer has no distribution other than organic.
Read-only and additive, so cst-0004 still holds: the CLI behaves identically whether
the renderer exists or not.

**Exit:** a stranger can read a public repo's why-graph in a browser, understand a
decision they had no context for, and find dira from it.

### M7 — apps
`github` storage backend · PWA · paid iOS + desktop apps (dec-0007).

**Exit:** a decision confirmed on a phone appears in the next session's brief on the
laptop, with no dira server involved.

---

## Blocked

*GROOMED 2026-07-30. Rows are added when implementation surfaces them, not only when
planning predicts them — three of the entries below were found by building, not by
thinking.*

| Item | Blocked on |
|---|---|
| **Automatic disposition capture** (part of M2/M4) | [qst-0005](../.dira/entries/qst-0005.md) issue 2 — kazi has no post-disposition hook on `approve`/`reject`. Fallback exists (the skill wraps the commands), so this degrades ergonomics rather than blocking the milestone. |
| **cst-0003 rule 2 has no runtime check** | `scripts/privacy-lint.py` enforces the marker, label-leak, ref-declaration and ADR-prose invariants, but it cannot verify that *inherited context was never persisted* into a child ledger — that needs an assertion in the brief-injection path, which lands with E1/E5. Until then rule 2 rests on care, which is what cst-0003 forbids. |
| **`dira brief` has no cold-cache answer** | Measured by E1-L3: a warm cache renders in 15.1ms against a 30.1ms uncached path, but a COLD cache costs 55.5ms — worse than no cache. The brief runs at `SessionStart`, so a user's first session after a clone is their slowest, and first impressions are what `dec-0010` says the acquisition moment depends on. E1-L5 must choose: build synchronously and eat it, fall back to uncached, or build in the background and serve uncached once. Its budget must then state which case it measures. |
| **The index screen has nowhere to put a cross-boundary entry** | Found while implementing `dec-0018`. `s2-index` groups by intent; a withheld parent has no row to sit in. Unowned. |
| **ADR back-catalogue import** | [qst-0003](../.dira/entries/qst-0003.md) — needs validating against kazi's real 83-ADR corpus before committing to an approach. Leaning index-everything, promote-on-first-citation. **Promoted to a launch blocker by [dec-0010](../.dira/entries/dec-0010.md):** import is the day-1 acquisition moment, so its quality is now a growth variable rather than a convenience. |

---

## Upstream asks (kazi-org/kazi)

All additive, none breaking. Tracked as [qst-0005](../.dira/entries/qst-0005.md);
issues not yet filed.

| # | Ask | Why |
|---|---|---|
| 1 | Register `portfolio` in `Kazi.CLI.Schema` | It already emits `schema_version` and a stable shape, but is absent from the documented `--json` registry (which covers only `apply`, `plan`, `bus`, `status`). Registering it makes dira's join surface a real contract instead of an implementation detail dira happens to read. |
| 2 | Post-disposition hook on `approve` / `reject` | The highest-leverage capture point in the design — an approval *is* a decision event. Must be a **direct synchronous hook, not** a `bus post`: ADR-0067's bus needs `kazi daemon` and no-ops when it is down, so dispositions would be silently lost, and its event stream is age-bounded to ~24h with a ~1 KB cap. Wrong shape and wrong durability for capture. |

**Was three asks; now two.** The third — an optional `why` field on goal-files — was
dropped on verification: goal-files already carry a free-form `metadata` table,
documented at `lib/kazi/goal/loader.ex:24` as a "string-keyed map, verbatim" and read
with no validation (`loader.ex:640`). dira writes `metadata.why = "dira:dec-NNNN"`
today. This also removes the only ask that would have put a dira-shaped string into
kazi's file format.

Risk worth stating: ask 2 could read as dira colonising kazi's surface with its own
concerns, and kazi may reasonably refuse — the fallback (the skill wraps the commands)
is recorded in qst-0005. Ask 1 is defensible on kazi's own terms regardless of dira:
an emitted-but-undocumented `--json` shape is a gap in kazi's public contract whether
or not anything consumes it.

---

## Not planned (deliberately)

Recorded so they are not rediscovered as ideas. Each is a tool dira has promised not to
become — see [cst-0002](../.dira/entries/cst-0002.md) and
[int-0003](../.dira/entries/int-0003.md).

- **Tasks / a tracker** — kazi owns execution and doneness.
- **Calendar, journal, CRM, dashboards** — the closed entry set exists to prevent this.
- **A required hosted service** — [cst-0004](../.dira/entries/cst-0004.md).
- **Attention-drift scoring** — [qst-0002](../.dira/entries/qst-0002.md). The one
  proposed feature closer to self-surveillance than navigation. May never be built,
  and that is an acceptable outcome.
