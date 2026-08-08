# L1 contract — how to decompose an epic into lanes, and emit L2

Every `L1-E<n>.md` prompt references this file. Read it first, then the epic prompt.
It defines the mechanics; the epic prompt supplies the scope.

---

## Before you plan anything

Read, in this order. Do not skip and do not re-derive what these already settle:

1. `docs/plan.md` — L0, for your epic's `acc:` line and its place on the critical path.
2. `.dira/entries/` — **the binding constraints.** These are decisions, not
   suggestions. If your plan contradicts one, you have two honest options: change the
   plan, or write a lane that supersedes the entry with a stated reason. Silently
   violating one is the single worst outcome of this exercise.
3. `docs/roadmap.md` — current status. Authoritative for what is done. Never
   duplicate status into a plan file.
4. `.agents/product-marketing.md` — only if your epic touches user-facing surfaces
   or copy.

The constraints that most often bite: `cst-0001` (brief ≤1500 tokens, enforced in
binary), `cst-0002` (five entry kinds, closed), `cst-0003` (inheritance one-way and
read-time only — a violation is a security bug), `cst-0004` (never requires a network
service, account, or hosted tier), `dec-0002` (one file per entry), `dec-0003` (no
model client in the binary), `dec-0004` (status derived, never stored).

---

## What a lane is

A lane is a unit of work **one agent or one person can own end to end without
coordinating mid-flight.** Not a category, not a phase. If two lanes must be worked
in lockstep, they are one lane. If a lane has an internal handoff, split it.

**Bound: ≤6 lanes per epic.** More than six means the epic was really two epics —
say so in your output rather than exceeding the bound.

Each lane needs, and is incomplete without:

| Field | Rule |
|---|---|
| `id` | `<EPIC>-L<n>`, e.g. `E1-L2` |
| `title` | one line, imperative |
| `owner` | `unassigned` unless L0 says otherwise |
| `depends_on` | lane ids, or `none`. Be honest — a false `none` produces a wave that deadlocks |
| `acc:` | **one machine-checkable predicate.** See below. This is the field that matters |
| `risk` | the one thing most likely to make this lane wrong, in a sentence |
| `entries` | the `.dira/` entries this lane implements or is constrained by |

## The `acc:` line is the whole point

`acc:` must be objectively checkable by a command, not by opinion. It becomes a kazi
goal, so it has to survive a cheap model grinding against it without being able to
declare false victory.

**Good** — a command and an observable outcome:
- `acc: go test ./internal/ledger -run TestReindex passes, and after deleting .dira/cache/ a reindex reproduces byte-identical query output`
- `acc: dira brief --context on fixtures/200-entry emits <=1500 tokens per the binary's own counter, and omitting entries is reported rather than silent`

**Bad** — unfalsifiable, or true before any work is done:
- `acc: the ledger works correctly` (opinion)
- `acc: code is clean and well documented` (opinion)
- `acc: dira log exists` (vacuously true once a stub exists — this is the failure
  mode kazi is designed to catch, so do not hand it one)

A predicate that is already green before the lane starts is a **defect in the plan.**
Check each one against the current repo state and say so if it passes already.

---

## Then emit your L2 prompts — this is not optional

For **each** lane you produced, write `docs/plan/prompts/L2-<lane-id>.md`. That file
must be dispatchable on its own by an agent with no memory of you, so it carries its
own context rather than referring to "the above".

Each L2 prompt contains:

1. **Context** — the epic, the lane, its `acc:` line verbatim, its `depends_on`.
2. **Read-first list** — `docs/plan.md`, the specific `.dira/` entries that bind this
   lane, and the specific source files if any exist yet.
3. **The task bound** — ≤8 tasks, each with: `id` (`<LANE>-T<n>`), `title`, `files`
   (the actual paths it will touch), `acc:` (same rules as above, at task
   granularity), and `depends_on`.
4. **Where to write** — `docs/plan/tasks/<lane-id>.md`.
5. **The stop rule**, verbatim:
   > Do not decompose further. L2 is the leaf level. If a task still feels too large
   > to converge in one kazi run, say so explicitly in your output and propose the
   > split — do not silently create an L3.
6. **The honesty rule**, verbatim:
   > If this lane cannot be planned because a prerequisite is unresolved, write that
   > as your output instead of inventing tasks. A plan built on an unresolved question
   > is worse than an admission that the question blocks it.

## Your output

Write `docs/plan/lanes/<EPIC>.md` containing the lane table plus, per lane, its
`acc:`, risk, and entry references. Then write one `L2-<lane-id>.md` per lane.

Report back: the lane count, the L2 prompt paths you wrote, any `acc:` line you found
already-green, any L0 constraint you believe is wrong, and anything you refused to
plan and why. **Be blunt about problems** — a plan that hides a known issue costs
more than one that names it.

## What not to do

- Do not write code. This is planning only.
- Do not create a parallel status surface — `docs/roadmap.md` owns status.
- Do not exceed the bounds (≤6 lanes, ≤8 tasks). The bounds exist because an
  unreadable plan reproduces the failure dira was built to fix (`int-0001`).
- Do not pad. Four sharp lanes beat six vague ones.
