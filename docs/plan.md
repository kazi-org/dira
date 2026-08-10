# dira — program plan (L0)

**Level 0 of a three-level plan tree.** L0 defines epics. Each epic has a
dispatchable prompt at `docs/plan/prompts/L1-<id>.md` that decomposes it into lanes
*and emits the L2 prompts for those lanes*. L2 produces tasks.

```
L0  docs/plan.md              epics        (this file — authored by the lead)
L1  docs/plan/lanes/<id>.md   lanes        (written by an L1 agent per epic)
L2  docs/plan/tasks/<id>.md   tasks        (written by an L2 agent per lane)
```

**Last updated:** 2026-08-10 · **Owner:** maintainer

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

**What works today**, all merged and CI-green against dira's own 44-entry ledger:

| verb | what it does |
|---|---|
| `dira sniff` | reads the session transcript and **stages** decisions; may never accept one |
| `dira distill` | the review screen: one keystroke per staged capture, with undo |
| `dira why` | prints the chain — what a decision arose from, every option refused, and why |
| `dira check` | refuses a plan contradicting a settled decision, quoting the original reason |
| `dira brief` | prints what is blocked and what was decided, for injection at session start |
| `dira supersede` | retires an entry in favour of its replacement, writing both sides |
| `dira log`, `ui`, `reindex`, `install-skill`, `version` | write by hand, browse, rebuild cache, install the skill |

**23 of 40 lanes are shipped.** This document is now scoped to **what remains**.

### The finish line, and the order

**Founder decision, 2026-08-09: all 40 lanes.** Order: *make it real on this machine
first* — dira used daily by its author, with everything else built against what
annoys him. Recorded in `docs/roadmap.md`.

### Founder decisions that create new work

- **`dec-0027`** — dira reads session metadata to derive attention drift, locally,
  never transmitted. **Taken against the recommendation** in `qst-0002` and against
  mine; recorded as such so it is not re-litigated as an oversight. It makes
  `cst-0003` and `cst-0004` load-bearing and adds three ship conditions: opt-in and
  off by default, the derived answer retained rather than a session timeline, and
  deletable in one documented command. **Scope for E5.**
- **`dec-0028`** — import measures a corpus before importing it and offers indexing
  when the yield is nothing. Evidence: five real ADR corpora, reasoned-alternative
  rates of 90%, 76%, 65%, 11% and **zero**. **New lane in E2.**
- **`dec-0026` + `dec-0029`** — the cold-start budget is a median, gated by the
  minimum. Closed E1-L6.
- **Notion portfolio mirror dropped** for this repo. `docs/roadmap.md` is the single
  progress surface.

---

## Discovery Summary

Five execution waves ran since the last plan, 30 pool tasks merged. Discovery for
this pass is the evidence those waves produced.

**The dominant finding, now at nine instances: a check that reports a verdict it
never reached.** Newly found and fixed this pass:

1. A command built, tested and merged but absent from `newApp`'s registry answers
   "unknown command" while its whole suite passes — `dira sniff` (dead for weeks
   while a hook installed it), `dira why`, `dira install-skill`.
2. A font committed, licensed and documented but referenced nowhere — `dec-0016`
   was `accepted` for weeks while nine design gates passed, because every one
   measures mockups that use the system stack.
3. `gates.mjs` scored a control that crashed at `import` as CONTROL TRIPPED. The
   harness built to catch blind gates was itself blind.
4. A contract corpus whose summary line hardcoded "0 problems" and printed it while
   failing with one — caught by its own author.

All four now have structural guards: `cmd/dira/registered_test.go`,
`docs/design/scripts/fonts.mjs`, the control's own marker requirement, and four
independent emptiness floors.

**The second finding is about me.** Four figures I relayed in agent briefs were
unverified, and every one was caught by the lane I gave it to — including a
"correction" to `dec-0026` that mixed two platforms' medians into one list. The
operating rule that follows: **a figure quoted in a brief must be traced to its log
first.**

**Latency is settled and no longer a risk.** CI medians are ~39ms (ubuntu) and
~55ms (macOS) against a 100ms ceiling. The developer machine cannot certify any of
it — `dira version`, which opens no ledger, measures 95–355ms depending on load —
so four absolute budgets now carry both statistics: the median asserts, the minimum
decides whether the machine can judge at all.

---

## Use Case Summary

Manifest: `.claude/scratch/usecases-manifest.json`. Engineering tasks carry
`verifies:` naming the ledger entries they discharge — this repo has no `UC-`
register, and entry ids are what `scripts/coverage.py` keys on.

---

## Scope and Deliverables

**In scope:** the 17 remaining lanes, plus two created by founder decisions (the
importer, attention drift) and one measurement debt. **E7's five lanes are parked
rather than remaining** — see the epic index for the trigger that revives them.

**Out of scope, deliberately:** anything in "Epics deliberately absent" below.

---

## Epic index

Every epic is named here whether or not it has remaining work. **This section is
load-bearing:** `scripts/coverage.py` extracts one obligation per epic from these
headings, and rewriting the plan without them orphaned ten register rows — which is
how the E7 omission below was caught rather than shipped.

### E0 — Foundations
Binary, schema, CI, gates. **Shipped except E0-L4 and E0-L5**, both outlined below.

### E1 — The ledger
Storage, `log`, `why`, `brief`, `reindex`, the cold-start budget. **Complete.**

### E2 — Capture
`sniff`, the semantic tier, the skill, `install-hooks`, the review queue.
**E2-L3 is the frontier**; the importer (`dec-0028`) is a new outline lane.

### E3 — The enforcer
`check`, `supersede`, constraint inheritance across ledger boundaries. **Complete.**

### E4 — Derived status
The kazi execution join, `dira map`, decision-blocked detection. **Outline.**

### E5 — Tiers
Personal and workspace ledgers, orphan drift, attention drift (`dec-0027`). **Outline.**

### E6 — Surfaces
The public renderer and the distill web surface. **E6-L3 remains, outlined.**

### E7 — Apps
The GitHub storage backend, OAuth device flow, the PWA, the paid bundle.
**PARKED, and this is a decision rather than a backlog.** `dec-0007` sequences the
paid apps deliberately: the team tier ships *when teams pull for it, not before*,
because the apps are bought by people who already use dira and therefore cannot
create the audience that produces those people.

**Revival trigger:** E7-L1 encodes the start gate as an enforceable ledger entry.
Until that entry exists and its condition is met, E7 is not decomposed and no L2
prompts are written for it. Five lanes are named in `docs/plan/lanes/E7.md` at lane
level only.

- [ ] T-E7.0 PLAN: encode the E7 start gate as a ledger entry, then expand E7 · `kind: plan` · deps: [the gate's own condition, not a task]

### E8 — Go to market
Channels, launch surfaces, distribution. **E8-L3, E8-L4, E8-L6 remain, outlined.**

### E9 — Upstream kazi
Both asks filed as `kazi-org/kazi#1681` and `#1682`. **Complete** on our side;
whether kazi accepts them is not ours to close, and E4 depends on the answer to
#1681.

---

## Checkable Work Breakdown

### Frontier — executable now

#### E2-L3 — `dira install-hooks` · `fidelity: executable` · 6 of 8 tasks open

Tasks live in `docs/plan/tasks/E2-L3.md`. This is the **"fires automatically"** half
of the founder's stated priority and the only lane with a decomposed breakdown.

- [x] E2-L3-T1 — the contract corpus, 14 cases over 12 fixtures, four emptiness floors
- [x] E2-L3-T2 — the shipped example IS the registrations, by embedding it
- [ ] E2-L3-T3 — the byte-span JSON scanner and splicer · deps: [T1]
- [ ] E2-L3-T4 — install: merge-never-clobber, idempotent, ownership by prefix · deps: [T2, T3]
- [ ] E2-L3-T5 — uninstall: the exact inverse, including the deletion decision · deps: [T4]
- [ ] E2-L3-T6 — the CLI with `--local`, `--dir`, `--uninstall`, confined root · deps: [T4, T5]
- [ ] E2-L3-T7 — every command string the installer writes is accepted by the binary · deps: [T2, T6]
- [ ] E2-L3-T8 — the installed strings fail open, with the failure path observed firing · deps: [T6]

**Two hazards recorded in the task file, not to be rediscovered.** Ownership must
match by **command prefix, not whole string** — `--all` was added to the installed
sniff command after the fact, so a whole-string matcher would not recognise an
already-installed pre-`--all` entry, would add a second, and would run capture twice
per compaction. And "prints nothing to stdout" is satisfiable only by a fake `dira`
that prints nothing — stdout is the payload, since it carries the brief.

#### T-DEBT.1 — re-derive the pixel tolerance · `verifies: [dec-0016]` · `lane: agent`

`acc: [node docs/design/scripts/measure-tolerance.mjs --write leaves tolerance.json byte-identical, or the diff is explained in DESIGN.md]`

The one item currently **measured by argument rather than by run**. After the font
change, `tolerance.json` was not re-derived; the reasoning (an unchanged capture
height is an unchanged pixel-count denominator) is in `DESIGN.md`. A 45-minute run
reached 7 of 12 combinations before being killed. Needs an idle machine.

---

### Beyond the frontier — outline epics

Per the rolling-wave rule, these are **not** decomposed. Each carries one planning
task that expands it when its trigger completes. Decomposing them now would be waste:
it would be replanned once the frontier's learnings exist.

#### E2-L7 — the ADR importer · `fidelity: outline`

**Intent.** `dira import <dir>` measures a corpus and reports what it found before
writing anything, offering to index rather than import where the yield is nothing
(`dec-0028`).

**Exit criterion.** Against `nulib/meadow` (31 documents, zero recorded rejections)
it imports nothing and offers indexing; against `bbc/tams` (49 documents, 237
reasons) it reports the count and imports on confirmation.

- [ ] T-E2-L7.0 PLAN: expand E2-L7 to executable fidelity · `kind: plan` · deps: [E2-L3-T8]

#### E0 tail — E0-L4, E0-L5 · `fidelity: outline`

**Intent.** The two remaining foundation lanes. Read `docs/plan/lanes/E0.md`; they
predate five waves of learning and their acceptance lines need re-reading against
what now exists before being decomposed.

- [ ] T-E0.0 PLAN: expand E0-L4 and E0-L5, re-checking their acc: lines against the current tree · `kind: plan` · deps: [E2-L3-T8]

#### E4 — the kazi execution join · `fidelity: outline` · 5 lanes

**Intent.** `dira map` joins the ledger to kazi's execution status at read time, so
"decided but never planned" and "blocked on an unanswered question" become visible.
Status is never stored (`dec-0004`).

**Exit criterion.** `dira map` matches reality with zero hand-entered status and
distinguishes execution-blocked from decision-blocked.

**Known input risk, already measured:** kazi's `portfolio --json` is emitted and
versioned but absent from its documented schema registry — filed upstream as
`kazi-org/kazi#1681`, unanswered. E4 depends on a contract kazi has not published.

- [ ] T-E4.0 PLAN: expand E4 to lanes and tasks · `kind: plan` · deps: [E2-L3-T8]

#### E5 — personal and workspace tiers · `fidelity: outline` · 5 lanes

**Intent.** The fractal model above one repo: a personal ledger and a workspace
ledger, with constraints inherited downward and private context never leaking. Now
also carries attention drift (`dec-0027`).

**Exit criterion.** "What are we actually doing on this venture?" is answered by a
derived report rather than reconstructed from memory.

**The privacy work is the hard part and it is now larger.** `dec-0027` puts session
metadata inside dira. Three conditions must hold before any of it ships: opt-in and
off by default; the **derived answer** retained, never a session timeline that could
reconstruct a working day; deletable in one documented command.

- [ ] T-E5.0 PLAN: expand E5 to lanes and tasks, with dec-0027's three conditions as acceptance · `kind: plan` · deps: [E4 milestone]

#### E6-L3 — the distill web surface · `fidelity: outline`

**Intent.** Serve the review queue over `dira ui`, keyboard and no-JS alike.

**Blocking finding, already recorded:** E6-L2's pixel clause fails because the
mockups are illustrations rather than renders of the fixture ledger, and the
execution statuses they show are kazi joins E4 has not built. That is a content
decision, not a rendering bug.

- [ ] T-E6-L3.0 PLAN: expand E6-L3, resolving the illustration-vs-render question first · `kind: plan` · deps: [E2-L3-T8]

#### E8 — go to market · `fidelity: outline` · E8-L3, E8-L4, E8-L6

**Intent.** The public renderer is the growth engine (`dec-0010`): a stranger lands
on a decision page from a link and wants it for their own repo.

**Exit criterion.** A dira-rendered decision page is reachable from a link, and the
launch surfaces are live.

- [ ] T-E8.0 PLAN: expand E8-L3, E8-L4 and E8-L6 · `kind: plan` · deps: [E6-L3 milestone]

---

## Parallel Work

**Pool mode, claimed through `/claim` with a `T-` prefix.** Two landmines are
recorded in `docs/lore.md` and both cost a wave to learn: the claim script rejects
this repo's `E1-L6-T1` id shape outright — **claim as `T-E1-L6-T1`** — and each
claim costs a network round trip of one to two minutes, so **claim in batches of
three or fewer** or a timeout leaves the wave half-claimed, which is worse than
unclaimed.

**Wave sizing is currently disk-bound, not agent-bound.** Free space has been
15–16 GiB at 97% full; below ~20 GiB, parallel builds fail, retry, and write more.
Remove worktrees immediately after merging.

**A wave boundary does not prevent a name collision.** Two lanes once declared
`Editor` in one package without sharing a file. Brief each agent with the
identifiers its neighbours are introducing.

---

## Timeline and Milestones

| milestone | what it means | gated on |
|---|---|---|
| **M-hooks** | dira captures automatically; the founder is its first daily user | E2-L3-T8 |
| **M-import** | a repo with history can be read without producing a second pile | E2-L7 |
| **M-join** | execution status is derived, never stored | E4, and kazi answering #1681 |
| **M-tiers** | the fractal model works above one repo, privately | E5 |
| **M-public** | a stranger can land on a decision page and want it | E6-L3, E8 |

---

## Risk Register

| risk | evidence | mitigation |
|---|---|---|
| **A decision is recorded and never wired.** | Three commands and one font shipped unreferenced. | Structural guards now exist for both classes. Any new "decided, committed" artefact needs its own census gate. |
| **E4 depends on an unpublished kazi contract.** | `portfolio --json` is emitted but absent from the schema registry; `kazi-org/kazi#1681` is open and unanswered. | Do not decompose E4 until the issue is answered or a fallback is chosen. The planning task is gated accordingly. |
| **`dec-0027` puts behavioural data in a tool holding private strategy.** | The founder chose this against the recommendation. | Three ship conditions are acceptance criteria, not guidance. |
| **Disk pressure silently degrades every gate.** | 15 GiB free at 97%; a timing gate measured `dira version` at 355ms under load. | Check free space before a wave; remove worktrees on merge. |
| **A figure quoted in a brief is not verified.** | Four instances this pass, all caught by the receiving lane. | Trace to the log before quoting. |

---

## Operating Procedure

1. `/apply --pool` claims and dispatches. Claim ids take the `T-` prefix.
2. A lane may not edit `cmd/dira/main.go`; it reports the registry line and the
   integrator adds it. `cmd/dira/registered_test.go` now makes forgetting it loud.
3. Every gate must be proved **both ways** — red on a false premise and green on the
   correct case. `docs/lore.md` L-0001. The green side is the one that gets skipped.
4. An `acc:` line is never edited by its own implementer. If it looks unpassable,
   report it. That has happened four times and each honest report was correct.
5. Tick the checkboxes on merge. They have gone stale three times because lane
   agents are correctly forbidden from editing the files they are graded against.

---

## Epics deliberately absent

Unchanged from the previous plan: no web app, no hosted service, no model client in
the binary, no analytics. See `docs/design.md` and `cst-0004`.

---

## Progress Log

- **2026-08-10** — Rescoped to what remains. The first draft dropped E7 entirely;
  `scripts/coverage.py` caught it by orphaning ten `lanes:` rows, because the epic
  headings it extracts from had been rewritten away. The epic index is restored and
  now says so in its own text.
- **2026-08-10** — Rescoped to what remains. 23 of 40 lanes shipped; four lanes
  completed this session (E1-L6, E2-L2, E2-L4, E3-L3) and were ticked. E2-L3 is the
  only executable epic; six outline epics carry planning tasks. Added T-DEBT.1
  (tolerance re-derivation) and E2-L7 (the importer, from `dec-0028`). No ADRs
  created — this repo records decisions as ledger entries under `.dira/entries/`,
  not as `docs/adr/` files, and `dec-0026` through `dec-0029` already carry this
  pass's decisions.
