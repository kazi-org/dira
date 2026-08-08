# Wave shape + upstream issues — decided under quiet hours

**Decided by:** delegate agent, on David's behalf, 2026-07-30 ~00:15 local.
**Authority:** quiet-hours exception in `~/.claude/CLAUDE.md` rule 1. David has not seen this.
**Surface at 8am.** Both decisions are reversible; neither has been executed.

**Evidence base:** read in full — `docs/plan.md`, `docs/plan/lanes/{E0,E1,E3,E8}.md`,
`docs/plan/prompts/{L2-E0-L1,L2-E8-L2}.md`,
`.dira/entries/{int-0002,int-0003,dec-0010,dec-0012,dec-0013,qst-0003}.md`,
`.agents/product-marketing.md` §§1–7, `docs/roadmap.md`, `git log`. Ran all three gates.

---

## DECISION 1 — shape of the next agent wave

### Pick: **(a), with three amendments.** E0-L1 in flight + E8-L1, E8-L2, E8-L5, and E3-L1 **re-scoped**.

**Amendment 1 — E3-L1 must not touch Go.** Its lane `acc:` requires
`go test ./internal/enforcer -run TestCorpusWellFormed`. That needs `go.mod` and a Go
package at `internal/enforcer/`, created while E0-L1 is under an explicit
*"do not scaffold future epics — no empty `internal/ledger`, `internal/ui`, or
`internal/check` packages"* constraint (`docs/plan/prompts/L2-E0-L1.md:48`). Dispatch it
with: author `corpus.yaml`, `corpus.sha256`, the fixture ledgers, and the
conflict-detection decision entry; **specify** the Go well-formedness test (path +
assertions) as prose for E0/E3-L2 to land; create no `.go` file and no `go.mod`. The lane
is startable now; it is not *green* until E0-L1's layout exists. Say that out loud rather
than letting an agent invent a module to satisfy its own predicate.

**Amendment 2 — resolve where the dec-0060 fixture lives before dispatch.** Three files
claim it: `docs/plan.md` §E3 ("E3 **owns** `fixtures/demo-ledger/`"),
`docs/plan/lanes/E8.md` §E8-L3 (deliverable: `fixtures/demo-ledger/`), and
`docs/plan/lanes/E3.md`'s own `acc:` lines (`internal/enforcer/testdata/ledgers/daemon`).
E8.md's notes flag the E3/E8 half of this; nobody flagged that E3 contradicts *itself*.
Pick one path in the dispatch prompt. If E3-L1 authors it under a fourth path, the
enforcer test and the demo clip diverge and one of them is lying — the exact failure
E3.md predicts.

**Amendment 3 — one guardrail line in E8-L2's prompt:** *the page may not claim import or
day-1 retrospective value; `qst-0003` is open. The hero demo is the `dira check` catch
(§6 primary), never the `init` clip.* See wrong-assumption #2 — the lane spec already
excludes this, so it is a drift guard, not a fix.

### The three reasons that actually drove it

1. **Three of the four companions never touch a Go file, and the fourth can be made not
   to.** E8-L1 writes `docs/growth/{channels,experiments,corpus}.md` + a node checker.
   E8-L2 writes `docs/design/landing/` + node scripts. E8-L5 writes `.claude-plugin/` +
   `docs/growth/drafts/` + a node checker. Zero overlap with each other beyond E8-L1 and
   E8-L5 both creating `docs/growth/scripts/` (different files; trivial). The go.mod race
   option (c) describes **cannot occur** in this set once E3-L1 is scoped. Serializing
   behind E0-L1 pays a coordination cost for a conflict that is not there.

2. **The qst-0003 objection is real but points at lanes that are not in this wave.** It
   binds `E8-L6` (launch sequencing — which already names `qst-0003` in its `entries:` list
   and is gated on E0–E3 anyway) and `E8-L4`'s *secondary* `init` clip (BLOCKED, no
   binary). It does not bind the landing page as specified. Building GTM assets now is
   efficient parallelism for E8-L1 and E8-L5 — those are infrastructure and checkers, not
   pitch. For E8-L2 it is closer to the line, and I am saying so rather than pretending
   otherwise: its strongest justification 90 minutes ago was that it owned `contrast.mjs`,
   which has since landed and passes. What remains is `check-coherence.mjs` + the
   `render.mjs` extension — a gate binding README ↔ marketing ↔ page, much cheaper to build
   *before* there are three surfaces to drift. That is enough. Barely.

3. **Holding them buys nothing.** A settled Go module does not make the channel plan, the
   plugin packaging, or the landing page easier, different, or safer. Option (b)'s premise
   — that these lanes benefit from waiting — is not true of any of the four.

### Second choice: (b). Why rejected

(b) is the disciplined answer and would be correct if the companion lanes touched Go. They
don't. Rejected on that single fact. It also does not deliver what it promises: a "true
5-agent coding wave against a settled module" cannot happen after E0-L1 either, because
`docs/plan/tasks/` is empty (wrong-assumption #4) — there are no leaf tasks to converge.
(b) buys a serial step and still lands in the same place.

(c) rejected outright — E1, E3, and E4 lanes all define or consume the storage interface
and package layout, and `E1.md`/`E3.md` both mark their paths *provisional until E0 lands*.
Four agents guessing concurrently is the named failure.

(d) rejected **as stated**, because "leaving the binary unstarted" is counterfactual — but
its *shape* (broad non-Go parallelism) is what I am recommending. 5 not 8, because the
binding constraint is not agent slots: it is your review bandwidth and the two shared-write
files below.

### Was starting E0-L1 wrong? No — but not for the reason given

"Harmless under (d)" undersells it. E0-L1 is the only lane in the repo whose completion
unblocks anything else; every other lane's `depends_on` chain terminates at it. Starting it
first is the one sequencing call that is unambiguously correct under any wave shape.

**One caveat that needs checking.** `docs/plan/prompts/L2-E0-L1.md` opens *"You are an L2
planner. You produce **tasks**, not code."* If the E0-L1 agent was dispatched with that
prompt, it will write `docs/plan/tasks/E0-L1.md` and **no Go code** — and a second,
implementing dispatch is still required. Nothing is on disk yet (clean tree at `8732c3f`,
one worktree, no branches, no `*.go`), so this is still cheap to correct.

### Two operational notes for the dispatch

- **`docs/roadmap.md` and `docs/coverage.md` are shared-write across the whole wave.**
  `scripts/coverage.py` extracts obligations mechanically and fails if any lacks a
  disposition — so any agent adding a Blocked row or an upstream ask creates an obligation
  that must land in `coverage.md` in the same change. Five agents editing both files is a
  guaranteed conflict. Take `R-roadmap` and `R-coverage` claim locks, or make the lead the
  sole writer of both and have agents report deltas.
- **Collapse L2 into the dispatch for these four lanes.** The L2 prompts (`L2-E8-L1.md`,
  `L2-E8-L2.md`, `L2-E8-L5.md`) are excellent implementation briefs. Give each agent its L2
  prompt and instruct it to produce the artifacts **and** `docs/plan/tasks/<lane>.md`.
  Otherwise those prompts sit around to be run later and produce task files describing work
  already done.

---

## DECISION 2 — the two upstream kazi issues

### Pick: **(a) — draft to disk, complete, with the duplicate check already done. David files at 8am.**

Concretely: write `docs/upstream/kazi-issues/{portfolio-schema,post-disposition-hook}.md`,
each carrying the title, body, reproduction, proposed contract, and a recorded result of
`gh issue list --repo kazi-org/kazi --search <terms>` (read-only, safe to run now). That
closes every clause of E9's `acc:` except pressing the button.

### The three reasons

1. **Nothing waits on it.** `docs/plan.md` §E9: *"Independent of everything else."* Its only
   downstream is part of E4, which sits two epics behind a binary that does not exist.
   Filing at 8am instead of 01:00 costs literally zero. When the cost of waiting is zero, an
   irreversible public action does not get taken by a delegate.

2. **These are not friction reports, they are API-design proposals.** `CLAUDE.md`'s standing
   rule — *"Every kazi friction point gets a GitHub issue at kazi-org/kazi in the same
   session"* — is the genuine argument for (b), and it is real durable authorization. But it
   was written for *"I hit a bug in kazi while working."* These two carry **proposed
   contracts** for a sibling product's public interface, published under David's name on his
   product's public tracker. A proposed contract in a public issue is a soft commitment.
   When a standing rule and the reversibility rule point different ways on something that
   blocks nothing, the tie goes to the cheaper error.

3. **Drafting loses almost nothing.** The reproduction, the contract, and the duplicate
   check *are* the work. What's deferred is 30 seconds of clicking — in exchange for the
   review this content warrants.

### Second choice: (b). Why rejected

The "he owns both repos" argument is about **permission**, and permission was never the
question. The question is **review**. Him owning `kazi-org/kazi` is precisely why the issue
tracker is not a scratchpad. One thing I want to state accurately rather than overstate:
E8-L5's deny-list bans *committing a script* that runs `gh issue create` — it does not ban
an agent filing an issue. That check is not the argument against (b); reversibility is.

(c) rejected outright — the content exists; deferring entirely just loses it.

---

## Assumptions in the brief that were WRONG on checking

1. **"E3-L1 … no Go" — wrong.** Its `acc:` requires
   `go test ./internal/enforcer -run TestCorpusWellFormed` over
   `internal/enforcer/testdata/corpus.yaml`. `docs/plan/lanes/E3.md:26` lists its
   prerequisite as *"nothing buildable"* — a false `none`, in a table whose own preamble
   says *"Stated because a false `none` deadlocks a wave."* This was the most load-bearing
   error in the brief and it is why (a) needs amendment 1.

2. **"The landing page would advertise 'point dira at your repo and get your decision graph
   back'" — not supported by the lane spec.** `L2-E8-L2.md` specifies the page as:
   pain-first hook → the §6 `dira check` catch as selectable text → three inversions → proof
   pillars → objections → one honest status line. Its absolute #3 already forbids claiming
   installable software exists and requires the design-phase status line under a coherence
   gate. The import claim lives in product-marketing §7 (internal), in E8-L4's *secondary*
   clip (blocked), and in E8-L6 (which already cites `qst-0003`). The exposure is drift, not
   spec. That said — **`qst-0003` being open and unowned is a genuine defect and you have
   deferred owning it twice.** By `dec-0013`'s own invariant 3, an open question with no
   owner and no revival trigger is *"forgetting with better vocabulary."* It does not block
   this wave. It does block E8-L6, and the ledger's own rules say so.

3. **"three passing gates (… contrast.mjs)" — true now, but the prompts are stale.** I ran
   all three: `coverage.py` PASS (0 orphans, 0 unverified), `privacy-lint.py` PASS (4
   checks), `contrast.mjs` PASS (42 pairs × 2 schemes + 6 hover>rest assertions). But
   `docs/plan/lanes/E8.md:99` and `L2-E8-L2.md` both still say `contrast.mjs` *"does not
   exist in the repo today"* and assign **writing it** to E8-L2. It was committed at 18:58,
   after those files were written. An E8-L2 agent following its prompt literally will
   rewrite a passing gate. Fix the prompt before dispatch. E8.md's "shared dependency with
   E6, E8-L2 is assigned to write it" note is now moot.

4. **"37 L2 task prompts" — right count, wrong implication. `docs/plan/tasks/` is empty.**
   There are 37 L2 *prompts* and **zero** task files. The plan tree stops one level above
   what `docs/plan.md` §"How to run the tree" says kazi converges. Any lane-level coding
   dispatch today hands kazi a 4–6-clause compound predicate that L2 exists to split into ≤8
   tasks. **This is the highest-value thing missing from the wave** — 37 L2 planners write
   37 disjoint files, have zero conflict surface, and need no Go to exist. If you want more
   parallelism than 5, that is where it is free. (E5 and E7 have no L2 prompts emitted at
   all; E5 was unblocked by `dec-0011` and nobody owns emitting them.)

5. **"24 ledger entries" — there are 25.** 4 `cst`, 13 `dec`, 3 `int`, 5 `qst`. Not
   load-bearing; noted because the number appears in briefs.

6. **"Zero Go code, no go.mod" — confirmed.** No `*.go` anywhere, clean tree at `8732c3f`,
   single worktree, no branches beyond `main`. Also confirms E0-L1 has produced nothing on
   disk yet.

---

## What I did not do

Filed nothing, posted nothing, published nothing, executed nothing. Modified no file in
this repo other than this one.
