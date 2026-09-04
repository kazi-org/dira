# dira — roadmap

Living document. Updated on every merge, lane claim, new blocker, and decision.

**Last updated:** 2026-08-14 · **Owner:** maintainer

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

**Wave 1 dispatched 2026-09-03** (`/apply --pool`, post-release bug backlog, `docs/plan.md`):

| task | what | lane | status |
|---|---|---|---|
| T-BUG1.1 | concurrency stress test for `ledger.Add` | kazi | in flight — no proposal drafted after 1.5h+, pinged for status 2026-09-03 |
| T-BUG1.4 | `dira supersede` names the actual precondition | kazi | in flight — same stall as T-BUG1.1 |
| T-BUG2.0 | diagnose issue #29 (frontmatter vs yaml.v3 layer) | agent | **merged, PR #38** |
| T-BUG2.1 | thread decode/validate errors into 6 consumers | kazi | in flight — proposal approved, t0 RED confirmed (all 6 capability predicates), hit kazi infra crash (`kazi-org/kazi#1073`, `--parallel` on a flat/ungrouped goal), retrying serial |
| T-BUG3.1 | `dira log --dry-run` | kazi | in flight — same stall as T-BUG1.1 |

T-BUG1.3 stays `blocked:` pending David confirming `dec-0033` via `dira distill`.

**T-BUG2.0 verdict (PR #38, merged):** issue #29's own repro (a colon inside a
properly straight-quoted title) does not reproduce — it decodes cleanly. The
failure only reproduces for an unquoted colon or smart/curly quotes, and both
fail at `yaml.v3` in `internal/ledger/decode.go:62`, the same layer #28/#30/#31
already share. No `internal/frontmatter` patch needed; T-BUG2.1 covers it once
its fixture uses a shape that actually reproduces.

---

## GTM

One line per E8 lane, per `docs/plan/tasks/E8-L6.md`. Links to the plan rather than
restating status the sections above already own — this section exists so a reader can
see the whole go-to-market lane's shape in one place, not to duplicate it.

| Lane | What | Owner | PR |
|---|---|---|---|
| [E8-L1](plan/tasks/E8-L1.md) | Channel plan — 19 channels rated, 3 inner-ring, pre-registered thresholds | @maintainer | no PR yet |
| [E8-L2](plan/tasks/E8-L2.md) | Landing page, gated on the same honest-limits/no-hype rules as the drafts | @maintainer | no PR yet |
| [E8-L3](plan/tasks/E8-L3.md) | Demo fixture, recording harnesses, positioning-doc checkers | @maintainer | #10 |
| [E8-L4](plan/tasks/E8-L4.md) | Demo clips (`check.cast`, `init.cast`) and their probes | @maintainer | #19 |
| [E8-L5](plan/tasks/E8-L5.md) | Draft frontmatter contract, marketplace/awesome-list/directory drafts, `check-drafts.mjs` | @maintainer | no PR yet |
| [E8-L6](plan/tasks/E8-L6.md) | `launch.md`, Show HN/Reddit/X drafts, the pre-send accuracy gate, the launch-readiness checker | @maintainer | #20 |

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

> **Session ended 2026-08-11.** Handover notes: `docs/handover.md`. No handover
> branch — nothing existed outside `origin/main`. Claims released, worktrees removed.

## STATUS 2026-08-20 · fable-orchestrator · RELEASED, PUBLISHED, INSTALLED

**dira is released and in use on all three of the founder's machines.**

- **v0.1.0** (2026-08-17): first live run of the whole release pipeline —
  Release + checksums + tap publish all green. Its first REAL `brew install`
  then failed on the founder's own MacBook ("bad CPU type"): the machine is
  an Intel x86_64, the release had no darwin-amd64 target, and the generated
  formula served arm64 to every Mac unconditionally. Fixtures could never
  catch this; the live install did.
- **v0.1.1** (2026-08-18): four targets (darwin arm64+amd64, linux
  amd64+arm64 — the linux-arm64 added for the DGX by founder decision),
  arch-conditional `on_arm`/`on_intel` formula blocks, exact-token checksum
  greps (darwin_arm64 vs darwin_amd64 differ by one character). Verified by
  real installs, not assertion.
- **Plugin published**: kazi-org/claude-plugins now carries dira (manifest
  with the three hooks + the skill), second entry beside kazi.
- **Installed and verified on all 3 machines**: Intel MBP (brew, 0.1.1,
  `brief` renders), Mac mini (ssh: brew + plugin), DGX aitopatom (ssh as
  ndungu: linux-arm64 archive checksum-verified into ~/.local/bin + plugin).
- **Ops notes**: two wedged Site/CI Chromium-contrast runs cancelled per
  standing authorization (32094702954, 32094725254 — a ~2-min job stuck
  2h20m). Site badge showed "pre-release" post-tag because the Pages
  checkout was shallow/tagless — fetch-tags fix in PR #32.

**Post-release truth sweep (2026-08-20, PRs #32/#33, verified live):** the
site now shows the v0.1.1 badge and leads with `brew install kazi-org/tap/dira`
(the "no brew install yet" claim is gone from every surface — README, landing,
canonical coherence strings, growth drafts); the badge renders from git
describe with a tag-fetching checkout. The dira plugin is live in real
sessions — its skill appears in the session skill list on this machine.

**Correction (2026-08-20), mine:** two merges reported on 08-15/08-17 had
silently failed (gh output piped to tail, chain ran past the failure) — the
launch kit (PR #20) and the manifest bump (PR #24) were still open. #24 closed
as superseded by the truth sweep; #20 rebased against the released reality
(its readiness selftest also assumed a dira-free host — fixed deterministic)
and is now VERIFIABLY merged, state queried not inferred.

**Launch readiness, observed 2026-08-20:** `check-launch-readiness.mjs` →
**READY** — all 8 gates pass (binary on PATH via tap, T0-relative plan,
registered thresholds, drafts accurate vs README, demo cast in bound, GTM
section complete, zero hype terms). The launch is now purely the founder's
T0 call. Upstream kazi#1681 remains unanswered; E4's emitted-contract
fallback unaffected.

## STATUS 2026-09-02 · dira (self-triage) · BACKLOG TRIAGED, 2 DECISIONS STAGED

**All 8 open GitHub issues (`#27`-`#37`) triaged and folded into `docs/plan.md`'s
new frontier: three bug clusters, `T-BUG1`/`T-BUG2`/`T-BUG3`, all at executable
fidelity.** No new epic — this is hardening on the shipped surface, not new
capability, matching the T-DEBT.1 precedent (a bounded debt item, not a lane).

**Correction to issue #27's own premise, verified in source.** The exclusive-create
fix its comment thread already decided on (`os.Link`, refuse-if-exists) is already
shipped — `internal/ledger/local/local.go` and `internal/ledger/write.go`, live
since 2026-07-30, before every reported occurrence. macbook-chief's git
archaeology on the cited incident (hq's `dec-0542`/`dec-0544`) rules out a
cross-branch merge collision (one linear commit, no discarded parent anywhere) and
best fits issue #35's bug instead — an uncommitted entry deleted before commit,
freeing its number for later legitimate reuse — not a live `Add`-vs-`Add` race.
Not provable from git alone, so `T-BUG1.1` adds a stress test to settle it
either way rather than resting on the hypothesis.

**Two ledger decisions staged from this pass, awaiting confirmation via `dira
distill`, not self-confirmed:** `dec-0032` (persist a monotonic id counter, closes
#35 unconditionally and turns a cross-session double-allocation into a loud git
merge conflict instead of a silent duplicate id) and `dec-0033` (a reject/tombstone
disposition that retains an entry's id, closing #35 and #36 together). `dira
check` run against the plan direction first: no conflict with 35 enforced entries.

**Second confirmed root cause: issues #28/#29/#30/#31 share one fix point.**
`internal/index/sync.go` discards the real decode/validate error and reports only
an id count; `ledger.Decode`/`Entry.Validate` already produce the field-level
detail issue #31's own repro table shows by hand. One fix (`T-BUG2.1`) covers
`brief`, `check`, `map`, `reindex`, `ui` and `why` at once. Issue #29's specific
case is not yet confirmed to be the same layer — `T-BUG2.0` diagnoses first.

**Issue #37 has no confirmed repro** — the reporter's own hypothesis does not hold
against the code as read. `T-BUG3.1` ships the `--dry-run` flag the issue itself
asked for; a real fix waits on an actual repro command.

**GitHub labeling and comment drafts:** see `docs/plan.md`'s Discovery Summary for
full citations. Applying `bug` labels to all 8 issues and posting the #27
correction as a comment are pending David's go-ahead (public repo, visible action).

**Next:** David confirms or rejects `dec-0032`/`dec-0033` via `dira distill`
(`T-BUG1.2`/`T-BUG1.3` are `blocked:` on this); `/apply --pool` the three waves in
`docs/plan.md` once unblocked.

---

## STATUS 2026-08-14 (close) · fable-orchestrator · CODE COMPLETE

**The run is finished: 22 PRs merged, zero open, zero red, all 40 lanes closed
or founder-gated.** One orchestrator (planning/merging only) drove ~20 headless
Sonnet worker sessions across three waves plus targeted fixes. Everything below
was verified by observation, not worker claims.

**Landed today beyond the morning block:** E5 complete (tiers, attention drift
under dec-0027's three conditions, `dira init --interview`); E2-L7 (`dira
import`, measure-first per dec-0028, 47-entry real-corpus proof); E0-L4/L5
(release scaffolding: goreleaser config, release workflow + structural gates,
tap formula generator — live halves BLOCKED on founder, below); E6-L3 (distill
web surface; three real bugs found and fixed); E4 complete (contract layer on
the emitted kazi schema, join groundwork, and the L3→L5 `dira map` chain —
runs against this repo's real ledger, degrades gracefully with kazi absent);
E8-L3/L4/L6 (demo fixture + real asciinema recordings + launch kit with a
readiness checker that honestly fails until a release exists); the README
rewritten from the real 15-verb binary; the **dira.sire.run website** (PR #17,
per docs/plan/website.md — deploy exists but cannot fire until activated); and
the design-fidelity system fully green for the first time: 18/18 pixel pairs,
13/13 gates, coherence gate re-pointed at sentences that are true.

**Defect classes found and closed by the run itself:** headless workers dying
by "waiting" on backgrounded checks (brief hardened; two lanes resumed from
commit-graph inspection); a rebase that would have silently reverted the README
rewrite (caught at conflict); a fixture whose `sleep` outlived its killed shell
(`exec`); goreleaser's go-floor vs CI's pinned toolchain; a ledger-id collision
between two open PRs (renumbered with its coverage rows); `GIT_DIR` poisoning
the gitignore test under pre-commit (fixed; verified under a poisoned env);
`.btn` centering on `<button>` but not `<summary>` (2px height delta, measured
not guessed).

**FOUNDER — the only remaining actions, none code:**
1. ~~Mint `HOMEBREW_TAP_TOKEN`~~ **CORRECTED 2026-08-14:** the token already
   exists as a kazi-org **org-level secret with visibility "all"** (minted
   2026-06-23 for kazi's own release flow), so dira's release.yml resolves
   `secrets.HOMEBREW_TAP_TOKEN` with zero setup. The E0 lane checked only
   repo-level secrets and reported it missing; the orchestrator relayed that
   without an org-level check. Residual unknowns only a real run can prove:
   the PAT's expiry and that its fine-grained grant covers
   kazi-org/homebrew-tap — both exercised by the first release.
2. Push the first release tag (exercises E0-L4-T4's live half, including the
   tap publish via the org token; also swaps the site's version badge off
   "pre-release").
3. ~~Activate dira.sire.run~~ **DONE 2026-08-15, founder-directed, verified
   live:** DNS applied as IaC (sirerun/foundation#156, scoped `--target`
   production deploy, record confirmed on public resolvers), Pages custom
   domain set, first Site deploy green (after its own completeness gate
   correctly caught the missing `map` verb — fixed as PR #23), and observed
   serving: `https://dira.sire.run/` 200 with the strapline, `/why/dec-0001/`
   rendering the real rejection content, `https_enforced: true`.
4. Launch itself: `docs/growth/launch.md` is T0-relative;
   `check-launch-readiness.mjs` fails until a stranger can install dira —
   i.e. until 1–3 are done.

## STATUS 2026-08-14 · fable-orchestrator · for the seat

Founder-directed run (David, 2026-08-14): drive dira to code complete via
parallel headless Sonnet workers; this session plans, dispatches, merges only.

**Merged today, PRs #1–#9, all CI-green.** The whole remaining plan tree is at
task fidelity (E0 tail, E2-L7 importer, E4 ×5 lanes, E5 ×4, E6-L3, E8 ×3);
T-DEBT.1 closed by a full 12-combination run — every tolerance value
byte-identical, the last measured-by-argument item now measured by run; the CI
perf gate carries dec-0029's minimum-discriminator (plus three wrappers fixed
that would have hard-failed a legitimate skip); and **E2-L3 is complete — PR #9,
M-hooks: `dira install-hooks` shipped with T3–T8, every task's gate proved both
ways.** E5-L1 closed with zero work: qst-0001 was already answered by dec-0011.

**Founder decisions, 2026-08-14 (via AskUserQuestion, recorded here):**
1. E4 builds against kazi's EMITTED `portfolio --json` contract (lockstep
   schema_version 2, verified in kazi source), version-pinned, degrading
   gracefully on drift and on kazi-absent. Supersedes "wait for #1681".
2. E6 mockups become RENDERS of the fixture ledger, not illustrations — the
   fidelity gate measures what ships.
Also applied, defaulted per the 30-minute rule (routed 2026-08-11):
install-skill stays in E2; the no-clobber refusal keeps exit 0; the CI perf
gate gained the discriminator (merged as PR #7).

**In flight:** headless workers on E2-L7 (importer), E0-L4/L5, E8-L3, E5-L2..L5,
E6-L3, E4-L1, E4-L2. E4-L3→L4→L5 dispatch when L1/L2 merge; E8-L4/L6 when their
deps land. Two worker deaths (ended turn "waiting" on backgrounded checks —
headless sessions have no next turn) were caught by commit-graph inspection and
relaunched; the worker brief now forbids backgrounding.

**Watch items:** internal/index TestTheCacheIsGitignored flakes red under
concurrent-session load even on main (pre-existing; confirmed by the E2-L3
lane); the checkout's installed pre-commit hook is stale vs tracked
hooks/pre-commit. Both to be swept before this run closes.

**Founder-gated remainders:** minting HOMEBREW_TAP_TOKEN (E0-L5's live
release half); any actual external publishing in E8 (artifacts stop at
"ready + locally verified"); dira.sire.run DNS + Pages activation (see below).

**New scope, founder-requested 2026-08-14:** the public website at
**dira.sire.run**, mirroring kazi.sire.run's Astro + GitHub Pages setup —
standalone plan at `docs/plan/website.md` (8 tasks, two waves; deploy exists but
cannot fire until the founder runs the activation runbook). Also dispatched: a
README/docs refresh — the README still described the design-phase binary from
2026-07-29.

## STATUS 2026-08-11 · dira-integrator · for the seat

`SendMessage` to `seat` is not reachable from this machine, so this is the board
delivery per the org protocol. The seat sweeps origin.

**Done and verified.** 30 pool tasks merged across 6 waves; **23 of 40 lanes
shipped**; CI green on main, 7 of 7 jobs. dira has 11 verbs and the capture loop
works end to end — `sniff` stages 3 entries from a real transcript, `distill`
renders the card through a pty, and declines safely with one line on a pipe.
E1-L6, E2-L2, E2-L4 and E3-L3 are complete; E2-L3 (hooks) is 2 of 8.

**In flight:** nothing. No claims held, no worktrees, no agents running.
**Uncommitted or unpushed:** none. Tree clean, `origin/main == HEAD`.

**Blockers.** E4 (the kazi execution join) is blocked on `kazi-org/kazi#1681` —
`portfolio --json` is emitted and versioned but absent from the documented schema
registry. Filed, unanswered. Separately, this Mac is at 16 GiB free / 97% full, so
wave size is disk-bound rather than agent-bound.

### FOUNDER: three questions, routed to the seat as process-tier

None of these is Tier-3 (no money, nothing external-facing, nothing destructive), so
the seat should answer rather than escalate. Each carries a recommendation, and per
the protocol an unanswered question defaults to it after 30 minutes of working hours
and is recorded as defaulted.

1. **`dira install-skill` shipped, but `docs/plan.md`'s E2 scope names only
   `install-hooks`.** Keep it in E2 or give it its own lane?
   *Recommend: keep in E2* — installing the skill and installing the hooks are one
   job, wiring dira into the agent.
2. **Its refusal to overwrite a locally-edited skill exits 0.**
   *Recommend: keep 0* — leaving a file alone is neither a policy refusal (exit 2)
   nor dira being broken (exit 1).
3. **NEW, found while banking state: the CI perf gate has no minimum-discriminator.**
   A degraded runner turns main red. Today's failing run measured min 26.8ms, median
   144ms, max 897ms — a 33x spread — against a 100ms ceiling, and passed on re-run.
   Three *local* budgets already carry the discriminator (`dec-0029`); CI does not.
   *Recommend: apply it there too*, so a runner that cannot judge says so rather
   than failing the build.

**Artifacts, not content:** `docs/plan.md` (what remains), `docs/roadmap.md` (this
file, status), `docs/lore.md` (26 landmines), `.dira/entries/` (44 entries).

## Planned (from docs/plan.md, 2026-08-10)

**23 of 40 lanes shipped.** `docs/plan.md` is now scoped to what remains, on the
rolling-wave rule: only the frontier is decomposed, everything else carries a single
planning task that fires when its trigger completes.

| | |
|---|---|
| **Frontier, executable** | `E2-L3` — `dira install-hooks`, 6 of 8 tasks open. The "fires automatically" half of the founder's order. |
| **Debt, measured by argument** | re-derive `tolerance.json` after the font change. Needs an idle machine. |
| **Outline, one planning task each** | the ADR importer (`dec-0028`); the E0 tail; `E4` the kazi execution join; `E5` tiers and attention drift (`dec-0027`); `E6-L3` the distill web surface; `E8` go to market. |

**Two outline epics carry a named blocker rather than a date.** `E4` depends on a
kazi contract that is emitted but unpublished — `kazi-org/kazi#1681`, open and
unanswered — so it is not decomposed until that is answered or a fallback is chosen.
`E6-L3` inherits E6-L2's failing pixel clause, whose cause is that the mockups are
illustrations rather than renders, which is a content decision and not a bug.

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
