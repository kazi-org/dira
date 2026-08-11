# Handover — 2026-08-11, dira integrator session

## TL;DR

This session ran dira's execution pool as coordinator: six waves, 30 tasks merged,
23 of 40 lanes shipped, and the capture-and-review loop working end to end. It
stopped because the org protocol says an idle session banks and ends, not because
anything was left unfinished. **The single next action: claim `T-E2-L3-T3` (the
JSON scanner and splicer) and continue the hook installer** — but only if the seat
dispatches it, since the protocol now forbids self-claiming.

**There is no handover branch.** Nothing exists outside `origin/main`: clean tree,
no stash, no worktrees, no claims, no open PRs. A branch would have been a copy of
main.

## Done & VERIFIED

Every line below was observed, not inferred from an agent's report.

- **The capture loop runs.** `dira sniff --deep --stage --all` staged 3 entries from
  `internal/sniff/testdata/transcripts/pre-compact.jsonl`; `dira distill` rendered
  the first card through a pty at 72 columns; on a pipe it declines in one line and
  writes nothing. Observed directly, both paths.
- **CI green on main, 7 of 7 jobs**, run `31428583276` after a documented re-run.
- **11 verbs registered and reachable** — confirmed against `dira --help` from a
  fresh build, not from the registry source.
- **Four lanes complete**: E1-L6 (cold-start budget), E2-L2 (semantic tier + skill),
  E2-L4 (review queue), E3-L3 (parent-ledger inheritance).
- **The startup cut**: `jsonschema/v6` and 16 `x/text` packages off the command
  path. Binary 20,909,632 → 19,647,024 bytes; init allocations ~69,585 → ~47,875.
- **`dec-0016` implemented** — the embedded serif is now actually used. Verified by
  measured text width (Pagella 252.14px vs Palatino 254.38, fallback 236.06), so the
  browser rasterizes our face rather than substituting.

## Done but UNVERIFIED

- **`tolerance.json` was not re-derived after the font change.** The argument for why
  it should be a no-op is in `docs/design/DESIGN.md`: an unchanged capture height is
  an unchanged pixel-count denominator. **How to verify:**
  `node docs/design/scripts/measure-tolerance.mjs --write` on an idle machine. A
  45-minute run reached 7 of 12 combinations before being killed. This is the only
  item in the repo currently measured by argument rather than by run.

## In flight

**Nothing.** No agents running, no worktrees, no claims held.

## Blocked

- **E4, the kazi execution join** — waits on `kazi-org/kazi#1681`. `portfolio --json`
  is emitted and versioned but absent from kazi's documented schema registry. Filed
  by this project, unanswered. **Who owns unblocking:** kazi's maintainer. Do not
  decompose E4 until it is answered or a fallback is chosen; its planning task is
  gated accordingly in `docs/plan.md`.
- **E6-L3, the distill web surface** — inherits E6-L2's failing pixel clause. The
  cause is recorded and is not a bug: the mockups are illustrations, not renders of
  the fixture ledger, and the execution statuses they show are E4 joins that do not
  exist. **This is a content decision**, and E6-L3's planning task says to resolve it
  before decomposing.

## Running processes left alive

None. `kazi status` reports no live runs. No background tasks, no monitors, no
scheduled wakeups (the loop was stopped deliberately and never re-armed).

## Landmines & context

The full set is `docs/lore.md` (26 entries). The four that will bite a pickup
session soonest:

1. **Claim ids need a `T-` prefix.** `claim.sh` rejects this repo's `E1-L6-T1` shape
   outright and returns BLOCKED, which reads as "the pool is empty" rather than
   "your ids are invalid". Claim as `T-E1-L6-T1`. (L-0021)
2. **Claim in batches of three or fewer.** Each costs a network round trip of 1–2
   minutes; six sequential claims blew a 120s timeout and left a wave half-claimed,
   which is worse than unclaimed. (L-0022)
3. **`golangci-lint` takes a machine-global lock.** A run in an unrelated repo blocks
   this one. The pre-commit hook now distinguishes "could not run" from "found
   issues", but expect waits when siblings are grinding.
4. **Timing gates cannot judge on a loaded machine.** `dira version`, which opens no
   ledger, has measured 95–355ms here against a 100ms ceiling. Four budgets carry a
   minimum-discriminator so they skip-with-numbers rather than reporting a false red.
   **The CI perf gate does not** — see the open question below.

**The recurring defect, now nine instances: a check reporting a verdict it never
reached.** Three commands shipped unregistered, a font shipped unreferenced, a
control that crashed at `import` was scored as tripped, and a corpus hardcoded
"0 problems" while failing. All four classes now have structural guards. Assume the
tenth exists and has not been found.

## Open questions, routed to the seat (not to David)

Filed in `docs/roadmap.md` under `STATUS 2026-08-11`, flagged `FOUNDER:`, each with
a recommendation. Per the org protocol they default to the recommendation after 30
minutes of working hours and are then recorded as defaulted.

1. Does `dira install-skill` stay in E2 or get its own lane? *Recommend: stay.*
2. Does its no-clobber refusal keep exit 0? *Recommend: yes.*
3. Should the CI perf gate gain the minimum-discriminator the local budgets have?
   *Recommend: yes.* Today's red run measured min 26.8ms, median 144ms, max 897ms —
   a 33x spread inside one run — against a 100ms ceiling, and passed on re-run.

## Where the session's own record lives

- `docs/devlog.md` — the narrative: what ran, what was found, and a section on what
  I got wrong (four unverified figures relayed in briefs, two commits straight to
  main, three stale checkbox passes, and a plan rewrite that dropped an epic).
- `docs/lore.md` — 27 invariants. **L-0027 is new and is the one that cost most**:
  a command can be built, tested, merged and unreachable, because a lane's tests
  register the verb into their own app rather than into `newApp`.
- `.claude-checkpoint.md` — state at exit and the single next action.

## How to resume

```
git fetch origin && git checkout main && git pull --ff-only
```

1. Read `docs/plan.md` — scoped to what remains, rolling-wave. `E2-L3` is the only
   executable epic; everything else is an outline epic with a planning task.
2. Read `docs/roadmap.md` — status, the 2026-08-11 STATUS block, and the three open
   questions.
3. Read `docs/lore.md` before writing a gate. Nine of its entries exist because a
   check reported a verdict it had not reached.
4. **Do not self-claim.** The org protocol routes dispatch through the seat
   (`dndungu/hq`). Report at every lane boundary.
5. If dispatched onto the hook installer: `T-E2-L3-T3` is next, and its task file
   already records the two hazards — ownership matches by command *prefix* not whole
   string, and "prints nothing to stdout" is unsatisfiable because stdout is the
   payload.
