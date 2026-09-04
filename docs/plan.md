# dira — program plan (L0)

**Level 0 of a three-level plan tree.** L0 defines epics. Each epic has a
dispatchable prompt at `docs/plan/prompts/L1-<id>.md` that decomposes it into lanes
*and emits the L2 prompts for those lanes*. L2 produces tasks.

```
L0  docs/plan.md              epics        (this file — authored by the lead)
L1  docs/plan/lanes/<id>.md   lanes        (written by an L1 agent per epic)
L2  docs/plan/tasks/<id>.md   tasks        (written by an L2 agent per lane)
```

**Last updated:** 2026-09-02 · **Owner:** maintainer

> **Why this tree is bounded.** dira exists because nobody re-reads files
> (`int-0001`). A plan tree large enough to become unreadable would reproduce the
> exact failure the product addresses. So: **≤10 epics, ≤6 lanes per epic, ≤8 tasks
> per lane**, and every leaf carries an `acc:` predicate that is objectively
> checkable by kazi. A node without a testable `acc:` line is not a plan, it is a
> wish — delete it or sharpen it.

> **Relationship to `docs/roadmap.md`.** The roadmap is the living status surface
> (Shipped / In progress / In flight / Planned / Blocked). This is the decomposition.
> Epics map onto roadmap milestones; **the roadmap stays authoritative for status.**
> Do not duplicate status here.

---

## Context

**dira is a memory of why, kept in the repo as plain files, written mostly by the
coding agent rather than by the human.** The problem: a decision made with an agent
on Monday is gone by Friday, so a new session proposes the option already rejected
and the argument is had again. Multiply by a dozen parallel sessions.

**All 40 original lanes are shipped, released and launched.** v0.1.1 is installed
on all three of the founder's machines via `brew install kazi-org/tap/dira`, the
Claude Code plugin is live, `dira.sire.run` serves the public renderer, and
`check-launch-readiness.mjs` reports READY. Full history: `docs/roadmap.md`. E7
(the paid apps) stays deliberately parked — `dec-0007` sequences it to ship only
when teams pull for it.

**This plan is now scoped to the post-release backlog: bugs found by using dira on
real ledgers.** Eight GitHub issues (`kazi-org/dira#27` through `#37`) were filed
between 2026-08-19 and 2026-09-02, all from dogfooding — none are hypothetical.
Triaged 2026-09-02; see Discovery Summary.

---

## Discovery Summary

**This pass's discovery was source-reading against the shipped binary, not
speculation from the issue text.** Three findings changed the shape of the backlog
from what the issues, read alone, would suggest.

**1. The exclusive-create fix issue #27 asks for is already shipped, and predates
every reported occurrence.** `internal/ledger/local/local.go`'s `Store.Create` uses
`os.Link` (atomic, fails if the target exists — the create half already does what
`O_CREAT|O_EXCL` would); `internal/ledger/write.go`'s `Add` retries correctly on
`ErrExists`, marking the id taken and advancing. Both `dira log` and the `dira
sniff` auto-capture hook route through this same `Add`. This has been true since
commit `4b7a0d9`/`b7c2f61` (2026-07-30) — before v0.1.0/v0.1.1 and before every
occurrence in the issue thread (2026-08-19 through 2026-09-02).

**2. macbook-chief's git archaeology on hq's ledger (the incident #27 cites) rules
out a cross-branch merge collision and best fits a single shared working tree.**
`dec-0542` has exactly one commit, already holding the winning content, no merge,
no discarded parent anywhere in history or reflog — a merge/rebase landing one side
of two independently-created files would leave a two-parent commit or a reachable
loser; neither exists. `dec-0544`'s second commit is a routine explicit-id
amendment, not a second collision. hq's own CLAUDE.md confirms direct-commit on one
shared checkout for exactly this kind of write, which fits the evidence.

**3. Given (1) and (2), the best-supported mechanism is issue #35's bug, not a live
`Add`-vs-`Add` race: an entry was created (successfully, atomically) but never
committed, then deleted before commit — freeing its number — and a later session's
`Add` legitimately (per today's code) reused it for unrelated content.** This is
not provable from git alone (the deletion, if it happened, left no trace by
construction), so it is recorded as the working hypothesis, not a closed case —
`T-BUG1.1` below is a stress test designed to positively confirm or refute a
residual live-race gap in `Add` itself, independent of the hypothesis.

**Two staged decisions came out of this and are recorded in the ledger, not just
this document**, per `int-0001` — `dira` capturing its own reasoning is the point:

- `dec-0032` — persist a monotonic id counter; `Add` allocates from it instead of a
  directory scan. Closes issue #35 (id reuse after deletion) unconditionally
  regardless of which hypothesis above is right, and as a side effect turns two
  sessions independently advancing the same counter into a loud git merge conflict
  on the counter file instead of a silent duplicate id.
- `dec-0033` — add a reject/tombstone disposition that retains an entry's id.
  Closes issues #35 and #36 together: today deleting a bad auto-capture is the only
  retirement path, and it is exactly the mechanism that frees an id for reuse.

**Both are `state: staged`** — proposed by this planning pass, not self-confirmed.
Confirm or reject via `dira distill` before `T-BUG1.2`/`T-BUG1.3` land; each is
tagged `blocked:` in the work breakdown below until then. `dira check` was run
against this plan's direction before writing it (`int-0001`'s own tool, used on
itself): **no conflict with 35 enforced entries.**

**4. Issues #28, #29, #30 and #31 share one confirmed root cause**, verified
directly in `internal/index/sync.go`: when `ix.store.Get` fails to decode or
validate an entry, only the id is appended to the `invalid` list — the real error
`Get` returned is discarded before the notice string is built. `ledger.Decode` and
`Entry.Validate` already produce exactly the field-level detail issue #31's repro
table shows by hand (a `ledger.Decode` harness, cloned and built locally); the gap
is that no CLI command threads it through. All six consumers (`brief`, `check`,
`map`, `reindex`, `ui`, `why`) read the same notice string, so one fix point covers
all of them. **Issue #29's specific case (a colon inside a properly double-quoted
title) is not yet confirmed to be the same mechanism** — `Decode` first calls a
separate `internal/frontmatter` boundary-splitter before `yaml.v3` ever sees the
text, and that splitter's behavior on a quoted colon has not been read. `T-BUG2.0`
below diagnoses this before `T-BUG2.1` assumes it.

**5. Issue #37 (a decision rejected with "must record at least one alternative"
after four earlier identical-shaped calls succeeded) could not be root-caused from
the issue text alone.** The reported error comes directly from
`entry.ValidateDraft()`, and the positional-id-misdetection hypothesis in the issue
does not hold for the one invocation shape that can be inferred (`runLog` only
inspects `args[0]`, and a `--kind`-first invocation never sets `id`). The actual
failing command was not preserved. `--dry-run` (the issue's own request) is the
right next step rather than guessing further.

Manifest: `.claude/scratch/usecases-manifest.json`. Engineering tasks carry
`verifies:` naming the ledger entries or GitHub issues they discharge — this repo
has no `UC-` register, and entry ids are what `scripts/coverage.py` keys on.

---

## Scope and Deliverables

**In scope:** the four bug clusters above (`T-BUG1` through `T-BUG3`), covering all
eight open GitHub issues. **Out of scope:** anything in "Epics deliberately absent"
below, and E7 (still parked on `dec-0007`'s trigger, unchanged by this pass).

---

## Epic index

Every epic is named here whether or not it has remaining work. **This section is
load-bearing:** `scripts/coverage.py` extracts one obligation per epic from these
headings, and rewriting the plan without them orphaned ten register rows once
already — which is how that failure mode was caught rather than shipped.

**E0 through E9 are SHIPPED.** Full history in `docs/roadmap.md`; no epic below
carries open work. Headings and the em dash after each id are kept verbatim —
`scripts/coverage.py`'s extractor is a literal regex on this exact shape, and
changing it silently orphans the epic's `lanes:` row in `docs/coverage.md`.

### E0 — Foundations
SHIPPED. Binary, schema, CI, gates, release pipeline (goreleaser, tap formula),
all four live targets installed and verified on three machines.

### E1 — The ledger
SHIPPED. Storage, `log`, `why`, `brief`, `reindex`, the cold-start budget.

### E2 — Capture
SHIPPED. `sniff`, the semantic tier, the skill, `install-hooks`, `install-skill`,
the review queue, the ADR importer.

### E3 — The enforcer
SHIPPED. `check`, `supersede`, constraint inheritance across ledger boundaries.

### E4 — Derived status
SHIPPED. The kazi execution join (against kazi's emitted `portfolio --json`,
version-pinned, degrading gracefully on drift or kazi-absent), `dira map`.

### E5 — Tiers
SHIPPED. Personal and workspace ledgers, orphan drift, attention drift (`dec-0027`,
its three ship conditions all met).

### E6 — Surfaces
SHIPPED. The public renderer, `dira ui`, the distill web surface.

### E7 — Apps
**PARKED, and this remains a decision rather than a backlog.** `dec-0007` sequences
the paid apps deliberately: the team tier ships *when teams pull for it, not
before*. Unchanged by this pass — see `docs/plan/lanes/E7.md` for the five
lane-level-only descriptions and the revival trigger.

- [ ] T-E7.0 PLAN: encode the E7 start gate as a ledger entry, then expand E7 · `kind: plan` · deps: [the gate's own condition, not a task]

### E8 — Go to market
SHIPPED. Channels, launch surfaces, distribution. `check-launch-readiness.mjs`
reports READY; launch itself is the founder's T0 call.

### E9 — Upstream kazi
**Complete on our side; not ours to close.** Both asks filed as
`kazi-org/kazi#1681` and `#1682`, both still open upstream. `dec-0030` unblocked
E4 in the meantime by building against kazi's emitted (if undocumented) contract.

---

## Checkable Work Breakdown

### Frontier — executable now

The post-release bug backlog. All eight open GitHub issues, triaged into three
clusters by shared root cause. `kazi` is on PATH — every task below carries an
`acc:` line; `/apply`'s kazi lane derives the goal from it just-in-time at
dispatch.

#### T-BUG1 — Ledger id integrity · `verifies: [dec-0032, dec-0033]` · gh: #27, #35, #36

- [x] T-BUG1.1 Add a concurrency stress test for `ledger.Add`, positively confirming or refuting a live create-vs-create race · Owner: TBD · Est: 1h · deps: [] · kind: agent · verifies: [dec-0032] · acc: [a test spawning N goroutines calling ledger.Add against one shared local.Store, each with distinct content, asserts N distinct ids exist on disk with zero ErrExists-driven silent overwrites, and fails loudly — not silently — if any two goroutines' entries ever land on the same final id] · **Done 2026-09-03: satisfied by pre-existing code, no new implementation.** `TestThirtyTwoConcurrentAddsProduceThirtyTwoDistinctIDs` (`internal/ledger/write_test.go`, commit `b7c2f61`, 2026-07-30) already meets this acc verbatim. Verified independently: 5x green under `-race`, and proved dispositive by fault-injecting the retry logic (test then failed loudly on both backends) and reverting. No race found; `dec-0032`'s `revisit_if` does not fire.
- [ ] T-BUG1.2 Persist a monotonic id counter; `Add` allocates from it instead of a directory scan · Owner: TBD · Est: 1.5h · deps: [T-BUG1.1] · kind: agent · verifies: [dec-0032] · blocked: awaiting confirmation of dec-0032 via `dira distill` (currently staged) · acc: [deleting a confirmed entry and then calling ledger.Add for the same kind never reissues the deleted entry's number; two ledgers independently advancing the same counter file, then merged with git, produce a merge conflict on the counter file rather than two entries sharing one id]
- [ ] T-BUG1.3 Add a reject/tombstone distill disposition that retains the id · Owner: TBD · Est: 1.5h · deps: [] · kind: agent · verifies: [dec-0033] · blocked: awaiting confirmation of dec-0033 via `dira distill` (currently staged) · acc: [dira distill's reject disposition on a staged entry leaves the entry's file and id in place, marks it disposed/rejected, and a subsequent ledger.Add call for the same kind never reallocates that id]
- [ ] T-BUG1.4 `dira supersede`: name the actual precondition instead of the current cryptic error · Owner: TBD · Est: 45m · deps: [] · kind: agent · verifies: [dec-0033] · acc: [dira supersede against a target whose state is staged prints a message naming the field and required state — e.g. "dec-NNNN is staged; supersede requires a confirmed entry" — instead of "answer and 1 is never a verdict", and exits 2, not 1]

**Both are `state: staged` in the ledger; do not implement T-BUG1.2 or T-BUG1.3
ahead of David confirming the underlying decision via `dira distill`.** T-BUG1.1
and T-BUG1.4 have no such gate and can run immediately.

#### T-BUG2 — Unreadable-entry diagnostics · `verifies: [gh-28, gh-29, gh-30, gh-31]` · gh: #28, #29, #30, #31

- [x] T-BUG2.0 Diagnose issue #29: does a colon inside a double-quoted title fail at the `internal/frontmatter` boundary-splitter, or at `yaml.v3`/`Entry.Validate`? · Owner: TBD · Est: 45m · deps: [] · kind: agent · lane: agent · acc: [a written report (as a devlog entry or task comment) states, with the exact failing line and function cited, which of the two layers rejects the repro from issue #29 — this determines whether T-BUG2.1's fix covers it for free or needs its own patch to internal/frontmatter] · Done: 2026-09-03, PR #38
- [ ] T-BUG2.1 Thread the per-file decode/validate error into the index's unreadable-entry notice · Owner: TBD · Est: 1.5h · deps: [T-BUG2.0] · kind: agent · acc: [reindex, why, check, brief, map and ui, run against a ledger containing one entry with an over-long edges[].note, one with an unknown edges[].type, and (if T-BUG2.0 finds the same layer at fault) one with a colon inside a quoted title, each print the real decode/validate error text — field name plus limit or enum, matching ledger.Decode/Validate's existing message shape — alongside the id, not just a bare id count] · Note (T-BUG2.0, PR #38): issue #29's own repro (a colon inside a *properly straight-double-quoted* title) does not reproduce and is not the frontmatter splitter's fault — it decodes cleanly. The failure only reproduces for an unquoted colon or a smart/curly-quoted title, and both fail at `yaml.v3` in `internal/ledger/decode.go:62`, the same layer #28/#30/#31 already share. No `internal/frontmatter` patch is needed; T-BUG2.1's existing acc: line (unchanged here) should cover #29 once its ledger fixture uses one of the shapes that actually reproduces.
- [ ] T-BUG2.2 `dira lint [id...]`: a verb that validates entries and prints field-level errors on demand · Owner: TBD · Est: 2h · deps: [T-BUG2.1] · kind: agent · acc: [dira lint run against a ledger with N unreadable entries exits non-zero and prints one line per entry naming the field, the offending value, and the limit or enum, at the same detail level as issue #31's own repro table; dira lint against a clean ledger exits 0 and prints a one-line summary]

#### T-BUG3 — `dira log` diagnosability · `verifies: [gh-37]` · gh: #37

- [ ] T-BUG3.1 Add `dira log --dry-run`: show how arguments were parsed, without writing · Owner: TBD · Est: 1h · deps: [] · kind: agent · acc: [dira log --dry-run --kind decision --title T --alternative A --why-not W prints the assembled entry (or the exact ValidateDraft error) to stdout, writes no file, allocates no id, and exits 0 on a valid draft or 2 on an invalid one — matching the exit code the non-dry-run path would have used]

**Root cause of #37's rejection is not established** (Discovery Summary, finding
5). `--dry-run` is the diagnostic tool the next occurrence needs, not a claimed
fix. If a repro command surfaces (via the GitHub issue), add a task then rather
than guessing at a fix now.

---

## Parallel Work

**Pool mode, claimed through `/claim`.** `docs/lore.md` L-0021: `claim.sh` accepts
the `T-*` id shape directly, and this backlog's task ids (`T-BUG1.1`, `T-BUG2.0`,
...) already carry it — no prefix rewriting needed, unlike the epic-lane-task ids
used elsewhere in this plan (`E1-L6-T1` had to be claimed as `T-E1-L6-T1`). Each
claim costs a network round trip of one to two minutes, so **claim in batches of
three or fewer** or a timeout leaves the wave half-claimed.

**Wave sizing is disk-bound, not agent-bound**, per prior waves: check free space
before spawning and remove worktrees immediately after merging.

### Waves

```
Wave 1: Diagnose + independent fixes (5 agents)
- [x] T-BUG1.1  verifies: [dec-0032]   — satisfied by pre-existing code (commit b7c2f61), no new PR
- [ ] T-BUG1.3  verifies: [dec-0033]   (blocked pending dec-0033 confirmation)
- [ ] T-BUG1.4  verifies: [dec-0033]
- [x] T-BUG2.0  verifies: [gh-29]      (lane: agent) — PR #38
- [ ] T-BUG3.1  verifies: [gh-37]

Wave 2: Depends on Wave 1 findings (2 agents)
- [ ] T-BUG1.2  deps: [T-BUG1.1]   (blocked pending dec-0032 confirmation)
- [ ] T-BUG2.1  deps: [T-BUG2.0]

Wave 3: Depends on Wave 2 (1 agent)
- [ ] T-BUG2.2  deps: [T-BUG2.1]
```

Nine tasks total, at most 5 concurrent. No two tasks in one wave own the same
package: `T-BUG1.*` is confined to `internal/ledger` and `internal/distill`;
`T-BUG2.*` to `internal/index`, `internal/frontmatter` and a new `cmd/dira/lint.go`;
`T-BUG3.1` to `cmd/dira/log.go`.

---

## Timeline and Milestones

**All of M1 through M6 from `docs/roadmap.md` are achieved** — the ledger, capture,
the enforcer, derived status, tiers, and the public renderer are shipped and
launch-ready. M7 (apps) stays parked on `dec-0007`. This plan adds no new
milestone: the backlog above is hardening against the shipped surface, not new
capability, and is sized to close within one pool wave (see Waves above). Full
milestone detail and exit criteria: `docs/roadmap.md`.

---

## Risk Register

| risk | evidence | mitigation |
|---|---|---|
| **A residual live-race gap in `ledger.Add` is unconfirmed either way.** | Source reading shows the exclusive-create+retry logic is correct as written; git archaeology on the one available incident is consistent with a different mechanism (id reuse after deletion) but does not conclusively rule out a race. | `T-BUG1.1`'s stress test is designed to be dispositive. If it finds a real collision, `dec-0032`'s `revisit_if` fires and the fix must also touch `Add`, not only allocation. |
| **Two load-bearing fixes are gated on staged decisions David has not yet confirmed.** | `dec-0032` and `dec-0033` are `state: staged`, not `accepted`. | `T-BUG1.2` and `T-BUG1.3` are marked `blocked:` in the work breakdown; `/apply --pool` should not claim them until `dira distill` confirms the underlying decision. |
| **Issue #29's root cause may differ from #28/#30/#31's confirmed one.** | The shared fix point (`internal/index/sync.go`) is verified for #28/#30/#31; #29's failure may originate one layer earlier, in `internal/frontmatter`. | `T-BUG2.0` diagnoses before `T-BUG2.1` assumes; the acc: line is conditional on the finding. |
| **Issue #37 has no confirmed repro.** | The reporter's own hypothesis (positional-id misdetection) does not hold against the code as read. | `T-BUG3.1` ships the diagnostic tool the issue itself asked for rather than a guessed fix; a real fix task is added only once a repro exists. |
| **Disk pressure silently degrades every gate.** | Prior waves measured `dira version` at 355ms under load against 15-16 GiB free at 97% full. | Check free space before a wave; remove worktrees on merge — unchanged guidance from the prior plan. |

---

## Operating Procedure

1. `/apply --pool` claims and dispatches. Claim ids take the `T-` prefix.
2. A lane may not edit `cmd/dira/main.go`; it reports the registry line and the
   integrator adds it. `cmd/dira/registered_test.go` makes forgetting it loud.
3. Every gate must be proved **both ways** — red on a false premise and green on the
   correct case. `docs/lore.md` L-0001. The green side is the one that gets skipped.
4. An `acc:` line is never edited by its own implementer. If it looks unpassable,
   report it.
5. Tick the checkboxes on merge.
6. Before `T-BUG1.2` or `T-BUG1.3` starts, confirm `dec-0032`/`dec-0033` via
   `dira distill` — do not implement ahead of the ledger's own disposition flow.
7. `python3 scripts/coverage.py` must stay green. A new ledger entry with a
   `revisit_if` field creates a new `trigger:` obligation immediately — register its
   disposition in `docs/coverage.md` in the same change that adds the entry, not
   after.

---

## Epics deliberately absent

Unchanged from the previous plan: no web app, no hosted service, no model client in
the binary, no analytics. See `docs/design.md` and `cst-0004`.

---

## Progress Log

- **2026-09-02** — Backlog refined and all 8 open GitHub issues (#27-#37) triaged.
  Trimmed: E0-E9's completed lane detail (all shipped; full history now lives only
  in `docs/roadmap.md` and the already-existing `docs/plan/lanes/*.md` /
  `docs/plan/tasks/*.md` files, which stay as the append-only record). Added three
  bug clusters at executable fidelity (T-BUG1, T-BUG2, T-BUG3) covering all eight
  issues. Two ledger decisions logged as `staged` (`dec-0032`, `dec-0033`) rather
  than self-confirmed. No new ADRs — this repo records decisions as ledger entries
  under `.dira/entries/`, not `docs/adr/` files. `docs/coverage.md` updated for the
  two new `trigger:` obligations the staged entries' `revisit_if` fields created;
  `python3 scripts/coverage.py` verified green after the edit.
