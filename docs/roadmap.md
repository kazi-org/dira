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
| **E9 is complete — both upstream kazi asks filed.** `kazi-org/kazi#1681` (register `portfolio` in `Kazi.CLI.Schema`) and `#1682` (post-disposition hook on approve/reject), both open, both enhancement, no duplicates across 9 searches. Two claims were corrected before filing: `kazi teach --hooks` does not exist, and the "silently lost" characterisation of a bus-carried disposition was an over-claim. | E9-L1/L2, 2026-07-31 |
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

## Wave 1 shipped (pool, 2026-07-31)

Seven tasks, five agents in isolated worktrees, all merged and CI-green. The first
run that actually dispatched from the pool — see `docs/lore.md` L-0021 for why no
earlier one could.

| task | what landed |
|---|---|
| cold-start harness (E1-L6-T1) | `internal/perf` spawns the built binary and measures it as a hook does. **Its budget clause is reported NOT MET, not fixed:** `dira version` — a binary that opens nothing — has a 155.4ms median through the same harness, already over the 150ms ceiling, so this machine cannot certify it either way. Routed to E1-L6-T4 and idle hardware. |
| hook contract (E2-L2-T1) | `dec-0023`. `PreCompact` stdout is **not** discarded — it becomes custom instructions for the compaction summariser, a one-turn call structurally forbidden from calling tools. A seam that looks alive and is inert. `Stop` and `SessionStart(compact)` are the seams that work, both already used here. |
| no-model invariant (E2-L2-T7) | `internal/nomodel`. Module-scoped by necessity — `cmd/dira` links `net/http` today, so a stdlib-scoped rule would be red on a correct binary, and a test pins that. |
| `n` semantics (E2-L4-T1) | `dec-0024`. `n` disposes of the **capture**, not the option; "we decided against it" is a `y`. Found a real rejection loop: `sniff` dedupes by title, so deleting a rejected entry lets the next `Stop` re-stage the same sentence. |
| staged queue (E2-L4-T3) | `internal/distill`. Reads through the Store, never the index, so a stale cache cannot hide a staged entry. Pending-extraction is derived, not a new field. |
| `[parents]` parsing (E3-L3-T1) | `internal/config` declarations. Falls **closed** on an unreadable visibility, and no parent value ever reaches an error message. |
| read-only parent (E3-L3-T2) | `ledger.ReadOnly`. States plainly it **cannot** stop `index.Open` writing to a parent — the cache is opened beside the store, not through it — rather than implying a guarantee it lacks. |

**Three variants of one bug, all found in this wave:** a check reporting a verdict
it never reached. `privacy-lint.py` could not see a private parent whose
declaration carried a trailing comment — the exact shape shipped as the documented
example — and read `visibility = "privat"` as public. The pre-commit hook reported
"golangci-lint found issues" when the linter was blocked by a **machine-global**
lock held by an unrelated repo. And `golangci-lint` exited non-zero reporting
"0 issues" on `internal/perf`, a build-tagged package it could not typecheck at
all — so nothing had ever linted it. All three fixed, each proved on the green
side too.

## Wave 5 shipped (2026-08-10)

| what | result |
|---|---|
| **The JSON-schema library is off the command path.** `decode.go` imported `schema` for a string split and a sentinel, dragging in `jsonschema/v6` and 16 text packages on every invocation. | **-1.20 MiB binary, -21,710 allocations per run** — 31% of everything allocated before `main`. The timing was NOT confirmed: the machine sat at load 390-420 and its median run was 20x its minimum, so the lane reported the load-independent number and refused the millisecond figure. |
| **`dira distill` cannot hang a hook.** `Terminal` has no read method; the only route to a `KeySource` is `Raw()`, called only when stdin is a terminal *and* a card exists. | No code path can block on a pipe. Raw mode restored across 7 exit paths including a panic in the renderer. |
| **An unbounded spin in merged code**, found by mutation: a failed disposition did not advance the card — right for a human retrying, infinite against an endless key source and a failing store. | Bounded at 3 attempts. The test would HANG rather than fail without the bound; removing it is observed timing out. |
| **`dec-0016` implemented** — the embedded serif was accepted, committed, and referenced nowhere for weeks while nine gates passed, because all of them measure mockups using the system stack. | Wired and proved to RENDER (Pagella measures 252.14px vs 254.38 Palatino, 236.06 fallback). The swap moved **zero pixels** — every capture height identical on both sides. |
| **`dec-0029`** records the measurement discipline: spawned subprocess, median asserts, minimum decides whether the machine can judge. | Fourth absolute budget (`internal/why`) given the same discriminator. |

**Three structural guards added, all for defects that had already shipped.**
`cmd/dira/registered_test.go` — a command built and merged but absent from the
registry answers "unknown command" while its tests pass; that happened to
`dira sniff`, `dira why` and `dira install-skill`. `docs/design/scripts/fonts.mjs` —
a committed font nothing references. And `gates.mjs` was scoring a control that
crashed at `import` as "CONTROL TRIPPED": the harness built to catch blind gates
was itself blind.

**Corrections to the record, all mine.** I relayed four unverified figures in
briefs, and lanes caught every one: `schema.NewValidator()` is not on the write
path (all 19 call sites are tests, which is why the cut was possible); the spawn
floor figures I quoted appear in no source, one being a hypothetical in a comment
I wrote. And my own correction to `dec-0026` was wrong — it mixed ubuntu and macOS
medians into one list because I grepped the first match per run, and the platforms
differ by 17ms in the same run. Corrected again, per platform.

**Open, measured-by-argument:** `tolerance.json` was not re-derived after the font
change. The reasoning is in `DESIGN.md` — an unchanged capture height is an
unchanged pixel-count denominator — and the command to close it properly is
`node docs/design/scripts/measure-tolerance.mjs --write`. A 45-minute run was
started and killed at 7 of 12 combinations.

## Order of work, set by the founder 2026-08-09

Finish line is **all 40 lanes**. The order is *make it real on this machine first* —
dira used daily by its author, then everything else built against what annoys him.

1. **Make it capture and review without ceremony.** The hooks that fire
   automatically, and the one-key review of what they caught. This is the shortest
   path to dira being used rather than built.
2. **Then the rest of the active lanes** — the cold-start entry, the non-interactive
   cases, the distill card.
3. **Then the parts that need someone else present**: the kazi execution join, the
   public renderer and launch, the personal and workspace tiers.

**Four founder decisions landed with this order** — attention drift IS built, reading
session metadata locally (`dec-0027`, taken against the recommendation and recorded as
such); import measures a corpus before importing it and offers indexing when the yield
is nothing (`dec-0028`); and the Notion portfolio mirror is **dropped for this repo** —
this file is the single progress surface, and a second one that cannot be written just
makes every report carry a caveat.

## Ready to claim (unblocked, 2026-07-31)

15 of 40 lanes shipped. These eight have every dependency satisfied. **None has an
executable task breakdown yet** — `docs/plan/tasks/` holds only the five lanes already
finished — so `/apply --pool` has nothing to claim until they are expanded to task
fidelity. That expansion is the current blocker on the loop, not the work itself.

| lane | what it is | why now |
|---|---|---|
| **E1-L6** | hold the cold-start budget: <100ms `dira brief` on a cold cache | M1's remaining exit criterion; `brief` measured 61.2ms cold in-process, but nothing pins the *spawned binary* |
| **E2-L2** | the semantic tier: the skill and the `--deep` handoff, no model client in the binary | blocks E2-L3, whose acceptance cannot pass without it |
| **E2-L4** | the disposition flow — one keystroke per staged entry | `sniff` stages today with nothing to dispose them |
| **E3-L3** | inherit constraints across ledger boundaries without leaking private text | the last M3 lane |
| **E6-L4** | close the three open design questions against hostile data | gates E6-L5's ship |
| **E0-L4** | (E0 tail) | unblocked since E0-L1 |
| **E5-L1** | (E5 head) | no dependencies |
| **E8-L3** | (E8) | no dependencies |

## Open challenges (raised, awaiting a lane's reply)

- **The fidelity gate's channel threshold may buy headroom twice.** E6-L1 measured noise
  at exactly zero across 60 shots (5 arms including a second browser process and a
  different origin), then set a 4× safety factor on the pixel count *and* chose a
  4/255 channel threshold when 0/255 was admissible with a 16× margin. The two
  compound. Challenged; if it stands, 0/255 is strictly stronger and free. Separately,
  TOLERANCE.md justifies the choice with "as robust as the evidence allows", which
  inverts — a larger threshold is less sensitive, not more robust.
- **Fixture schema validation is verified but not gated.** 18/18 fixture entries pass the
  Go validator and 17/17 invalid fixtures are correctly rejected, but nothing re-checks
  it. Assigned to E6-L2. A verified-once property that nothing re-checks is the exact
  failure mode this repo keeps shipping.

## Blocked



- **The global gitignore is a project gitignore in the global slot — needs the owner's
  call.** `core.excludesfile` points at a file carrying other projects' artifacts
  (`zonnx-converter.log`, `gemma-plan.md`, arXiv PDFs) alongside bare unanchored
  patterns that match at any depth in **every repo on this machine**: `plan.md`,
  `plan.done.md`, `todos.md`, `bugs.md`, `issues.md`, `30day.md`, `GEMINI.md`, and the
  directory names `artifacts`, `tmp`, `vendor`, `bin`, `model_data`. This is how
  `docs/plan.md` was written, referenced repeatedly, and never committed for this
  repo's entire history — `git status` shows nothing, so the file does not exist to
  anyone else. **In dira the current damage is limited to `.claude/`**, which matches
  practice; the hazard is latent elsewhere, where a `bin/` source directory or a Go
  `vendor/` would vanish silently. Proposed: move the project-specific lines back to
  the project that owns them, and anchor what remains (`/plan.md`, not `plan.md`).
  **Not done here** — editing a global config affects every repo and is the owner's
  call, not a session's. Detail in `docs/lore.md` L-0005.



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

All additive, none breaking. Tracked as [qst-0005](../.dira/entries/qst-0005.md).
**Both filed 2026-07-31** — kazi-org/kazi#1681 (ask 1) and #1682 (ask 2), both open,
both labelled enhancement. No duplicates existed; 9 searches across `--state all`. The
only near-hit for ask 1 was the closed issue that *built* `portfolio` — registration
was simply never done as part of it.

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

**Two claims did not survive verification and were corrected before filing.**
`kazi teach --hooks` does not exist — the command is `kazi install-hooks`, and the ask
would have cited a non-existent surface. And qst-0005's "the disposition would vanish,
and nothing would indicate it had" was an over-claim: every bus verb prints a no-daemon
error and exits 1, so a direct `bus post` fails loudly. The filed issue argues the
verifiable version instead, in kazi's own words — ADR-0067 states that convergence
never depends on the bus, so a consumer cannot distinguish "no disposition" from
"daemon was down". Stronger argument, and true.

Ask 1 also held up better than this table claimed: `portfolio_json/1` already emits the
*same* lockstep `schema_version` constant as the registered commands, so it is inside
the contract-version discipline while absent from the registry. Independently confirmed:
`@schemas` has exactly four keys and `portfolio` appears zero times in `schema.ex`.

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
