# dira — design

> **Status:** founding design, v2. Supersedes the v1 design doc drafted 2026-07-29,
> folding in derived status and `dira map`, the lifecycle walkthrough, the workspace
> and personal tiers with the privacy constraint, `init --interview`, the surfaces
> tier list, and the business model.
>
> Every decision in this document is also a real entry in `.dira/entries/` — dira's
> first user is dira. Where this prose and an entry disagree, **the entry wins**.

---

## 1. The problem, stated precisely

Hundreds of hours of brainstorming with a coding agent produce genuine agreement.
The agreement is then lost.

It is not lost for lack of writing it down. It was written down — 83 ADRs in kazi
alone. It is lost because **retrieval is pull-based and iteration outruns reading**.
Nobody re-reads a decision record, so week 12 relitigates week 1, and the agent — who
has no memory across sessions at all — cheerfully helps.

The failure has a shape worth naming, because it determines the fix:

| What exists | What's missing |
|---|---|
| Capture (ADRs, spec docs, plan files) | Retrieval that costs zero effort |
| What's done (kazi, trackers) | Why we chose that definition of done |
| Per-repo context | Cross-project and personal direction |
| Records of what we chose | Records of what we **rejected**, and why |

So dira is not a better ADR format. It is a **direction layer**: the thing that
remembers why, surfaces it unbidden, and refuses to let it be quietly contradicted.

### 1.1 Prior art, and the gap

The neighbourhood is crowded and the gap is specific:

- **Beads** (git-backed graph issue tracker as agent memory) is closest in spirit and
  solves the same "agent wakes with no memory" problem — but it is *work-item
  centric*, which is kazi's territory, not this gap.
- **Twining MCP** and similar give agents persistent project memory across context
  resets. Closest to the raw pain, but framed as MCP session memory rather than a
  git-native ledger with lifecycle semantics.
- **ADR skills** for Claude Code auto-detect decision moments and write records. They
  fix *capture*. The failure mode here is *retrieval*. Still files.
- **Spec-driven tooling** (Spec Kit, OpenSpec, BMAD, Superpowers) captures per-change
  intent as workflow ceremony, not a living why-graph.

Unclaimed: **a decision/intent ledger where the agent does the writing, the human
never opens a file, and the system actively enforces consistency.**

---

## 2. Charter: the boundary with kazi

kazi's README draws the line itself — kazi will never *"decide what to build — that's
your judgment."*

That sentence is dira's charter.

```
        ┌──────────────────────────────────────────┐
        │  dira — upstream of declaration          │
        │  intents · alternatives weighed          │
        │  rejections & why · open questions       │
        │  superseded thinking · constraints       │
        └───────────────────┬──────────────────────┘
                            │  a declared goal
        ┌───────────────────▼──────────────────────┐
        │  kazi — downstream of declaration        │
        │  predicates · convergence · evidence     │
        │  "is it objectively done?"               │
        └──────────────────────────────────────────┘
```

**kazi proves it's done. dira remembers why you wanted it.**

Neither depends on the other to function. kazi runs with no dira installed; dira
degrades to its ledger-side views with no kazi installed (§6.3).

---

## 3. The three inversions

dira only works if it inverts three defaults. Each one is a place where every
predecessor tool failed.

**1. Capture is the agent's job, not yours.** Hooks (`SessionStart`, `Stop`,
`PreCompact`) extract decisions, rejections, and open questions from the transcript
automatically. Your entire clerical workload is confirming or ignoring a one-line
prompt. → §4

**2. Review is push, not pull.** You never open a file. `dira brief` renders one
screen at session start and is injected into the agent's context at the same time.
This is the single fix for "I never read the ADRs." → §5

**3. It is an enforcer, not a notebook.** Before work is planned, `dira check`
compares the plan against accepted and rejected decisions and blocks contradictions:
*"You rejected a daemon on July 3 because it violates int-0002 — this plan
reintroduces it. Supersede or revise?"* This is the AI-native part. ADRs never did
this. → §7

---

## 4. The model

Five entry kinds. The set is **closed** (cst-0002) — every proposed sixth kind is a
proposal for dira to become Obsidian.

| Kind | Is | Lifecycle states |
|---|---|---|
| `intent` | a direction or bet — the why above every why | `active` `achieved` `abandoned` |
| `decision` | a choice, **with its rejected alternatives** | `accepted` `rejected` `superseded` `staged` |
| `question` | something genuinely undecided | `open` `answered` |
| `constraint` | a standing rule that inherits downward | `active` `superseded` |
| `note` | a thought not yet any of the above; decays | `active` `abandoned` |

Note what is **absent**: no state here means planned, in-progress, done, or blocked.
Those are *derived* (§6), never stored — that is dec-0004, and it is the difference
between dira and the trackers it replaces.

### 4.1 Edges

Five typed, directed edge types, stored on the subject entry so any mutation is a
single-file write:

- `supersedes` — replaces an earlier entry, flipping it to `superseded`
- `derives_from` — serves a parent intent, possibly in a **parent ledger**; its
  absence on an active intent is the orphan-work drift signal (§8)
- `informs` — weaker context link, no lifecycle effect
- `blocks` — gates another entry; typically from an open question
- `realized_by` — the only edge whose target is external: `kazi:prop-…` / `kazi:goal-…`

### 4.2 Why `alternatives` is the load-bearing field

A decision without recorded alternatives is an assertion. The `alternatives` array —
`option`, `why_not`, and optionally `revisit_if` — is what makes `dira why` worth
reading and `dira check` possible at all. `revisit_if` matters more than it looks: it
distinguishes a closed door from a locked one, and gives the enforcer something
better to say than *no*.

Full field-level schema: [`schema/entry.schema.json`](../schema/entry.schema.json).

### 4.3 Storage

One entry per file: `.dira/entries/<id>.md` — YAML frontmatter (machine fields) plus
a markdown body (the prose *because*).

This is **not** an append-only JSONL log, and the reversal is deliberate (dec-0002):
concurrent unattended capture from two sessions would conflict on the same line of
the same file, and a whole-file read-modify-write makes the phone a second-class
client for no gain. Per-entry files give conflict-free concurrent capture, reviewable
`git log -p` history, one write per mutation — and therefore one GitHub `PUT` per
mutation, which is what makes §9's mobile tier possible.

SQLite stays in the design as a **derived** read cache under `.dira/cache/`,
gitignored and rebuildable by `dira reindex`. If cache and files disagree, files win.

```
.dira/
├── config.toml          # the only hand-edited file
├── entries/             # the ledger — one file per entry, committed
│   ├── int-0001.md
│   ├── dec-0001.md
│   └── qst-0005.md
└── cache/               # derived, gitignored
    └── index.db
```

---

## 5. Capture: two tiers, two trust levels

dira **never embeds a model client** (dec-0003). No API key, no network on the happy
path, no provider lock-in — BYOM-neutral, like kazi.

**Tier 1 — `dira sniff`, in the binary.** Regex over the transcript. Cheap, offline,
deterministic, lossy. Everything it produces is written `state: staged`,
`source.tier: regex`. A regex has no business asserting rationale.

```
dira: staged dec-0060 "Checkpoint file for run resume (no daemon)"
      — confirm or ignore
```

**Tier 2 — the skill, in the live session.** The already-running Claude session fills
in what the sniffer cannot: the because, the alternatives with their why_nots, the
`derives_from` edge. Then it calls `dira log` with a complete entry. This costs no
extra model call, because the model is already there with the transcript in context.

The honest consequence: **dira's capture quality is a function of the agent driving
it.** With no agent, capture degrades to `dira log` typed by hand.

### 5.1 Hook points

| Hook | Does | Why there |
|---|---|---|
| `SessionStart` | inject `dira brief --context` | both parties oriented before the first prompt |
| `Stop` | `dira sniff` the turn, stage candidates | catch decisions while the language is fresh |
| `PreCompact` | deep extraction via the skill | **the insurance policy** — fires immediately before Claude Code's lossiest moment, so a four-hour brainstorm cannot evaporate into a compaction summary |

### 5.2 The brief

`dira brief` is the whole of inversion 2, and it is **hard-capped at 1,500 tokens
forever** (cst-0001, enforced by the binary). When it would exceed the cap, dira
**drops by priority rather than truncating**: open blockers, then current focus, then
recent decisions — and states what it omitted plus the verb to see the rest.

This is constitutional because the failure it prevents is dira's specific death: a
brief that grows without bound is the 83-ADR pile with better syntax, and nobody
reads that either.

---

## 6. Derived status: what's planned, in progress, blocked

The question dira must answer, and the one hand-entered status fields always get
wrong. dira **stores none of it** (dec-0004). It joins its ledger against kazi's
execution truth across `realized_by` edges, at read time.

### 6.1 The join

dira owns ledger state. kazi owns run state. Composing them yields every bucket:

| Bucket | Join rule |
|---|---|
| **To be planned** | accepted decision / active intent with **no** `realized_by` — *decided, never planned. The gap list only dira can see.* |
| **Planned** | `realized_by` → an approved proposal, or a goal-file not yet applied |
| **In progress** | linked goal has a live run (`running`) |
| **Completed** | linked goal `done`, with kazi's evidence attached by reference |
| **Execution-blocked** | kazi reports the goal `blocked` (stuck / over-budget) |
| **Decision-blocked** | an **open question** with a `blocks` edge gating this entry |

The last two rows are the point. **Execution-blocked** kazi already knows.
**Decision-blocked** it structurally cannot: the work stalled before any goal
existed, because a human never answered a question. That blockage appears in no
execution tracker anywhere, and it is the most common reason things do not move.

### 6.2 The real join surface

Verified against kazi on 2026-07-29 (dec-0008). `kazi portfolio --json` exists
(`cli.ex:1254`, `portfolio_json/1` at `cli.ex:4249`) and emits:

```
schema_version, kind: "portfolio", planned, by_repo (repo → bucket → entries),
fleet_remote, totals {base, empty, rows: [{bucket, count, pct}]},
todo, blocked [{…, cause, blocker}], rate
```

**Corrected 2026-07-30 — the mapping is not clean, and the join is a hybrid.**
`portfolio.ex:38` carries a *second* enum, `:in_progress | :stuck | :complete`, which
is what `by_repo` and `fleet_remote[].bucket` use; the five-value taxonomy governs only
`totals.rows[]` and the top-level `todo`/`blocked` arrays. And `portfolio_json/1`
emits **no `done` or `running` array** — those survive only as counts in
`totals.rows[]`, so portfolio reports how many goals converged, never which.

The per-goal read therefore comes from **`kazi status <ref> --json`** (registered at
`schema.ex:345`, ~0.65s/call), while blocked attribution (`cause`, `blocker`) is
available only from portfolio. dira needs both.

One caveat to carry knowingly: `portfolio` is **not registered in `Kazi.CLI.Schema`**
(which documents only `apply`, `plan`, `bus`, `status`). The shape is versioned but
not discoverable, and nothing pins it as a public contract. dira depends on it *and*
asks upstream to register it (qst-0005, issue 1).

### 6.3 Degradation

With kazi absent or erroring, dira falls back to ledger-side buckets — to-be-planned,
decision-blocked, achieved/abandoned intents — **and says so**. It never fills the
gap with a guess. dira does not assert doneness; it inherits it, so "done" stays as
objective as kazi's predicates make it.

### 6.4 `dira map`

The intent tree with derived roll-ups. `kazi portfolio` groups by **goal**; `dira map`
groups by **why**. That is the structural difference, not a cosmetic one.

```
int-0001  The why survives the session          3 done · 1 running · 1 stuck
  dec-0051  SQLite read-model            → converged ✓
  dec-0058  Homebrew distribution        → to be planned (no goal yet)
  qst-0007  multi-repo story             ⛔ blocks dec-0060
```

---

## 7. The enforcer

`dira check "<idea or plan>"` runs **before** predicates are drafted, and exits
non-zero on contradiction:

```
$ dira check "add a background daemon to track run state"
✗ conflicts with dec-0060 (accepted 2026-07-03)
    rejected alternative: "a daemon"
    why_not: violates the single-binary intent (int-0002)
    revisit_if: cold-start latency stops being the binding constraint
✗ conflicts with cst-0004 (active)
    dira never requires a long-lived service
→ supersede dec-0060, or revise the plan
```

This is the relitigation firewall. It reads `alternatives[].why_not` from rejected
and accepted decisions plus every `constraint` inherited down the tier chain (§8).
`revisit_if` is what keeps it from being merely obstructive — it tells you the
condition under which the door legitimately opens. **Note the asymmetry:** `revisit_if`
lives on an *alternative*, so a conflict with a rejected alternative can offer one,
while a conflict with a **constraint** can only ever offer "supersede it, in writing,
with a reason". That is deliberate — cst-0001 to cst-0004 are constitutional and the
heavy path is the correct one — but the check's message must not imply a revisit
condition exists where none can.

Where a check must cite a **private** constraint in a public context, it cites the
**ref only** (`me:cst-0002`), never the text (cst-0003).

---

## 8. Fractal tiers

One entry model, one join rule, one drift flag, at every level (dec-0006). The tier is
metadata, not a type system — the fact that the venture and personal tiers needed **no
new entry kinds** is the evidence the model is right.

```
~/.dira/                      you        — private git repo, syncs across machines
└─ sire/.dira/                venture    — derives_from: me:int-…
   ├─ kazi/.dira/             repo       — derives_from: sire:int-…
   └─ dira/.dira/             repo
```

What changes per tier is only which **mix** of the five kinds is idiomatic:

- **person** — intents as *directions* ("ship dira v0.1"); cross-project decisions
  ("parked X in March because Y"); personal constraints ("no new projects until dira
  ships"); portfolio-scope questions ("does gist become a product?").
- **workspace** — intents as *bets* ("kazi is the wedge product"), three to seven of
  them and never more, because **the cap is the clarity**; cross-cutting decisions no
  single repo owns; standing constraints every repo inherits.
- **repo** — the technical decisions, alternatives, and questions of one codebase.

### 8.1 Two mechanisms, same at every level

**Namespaced refs.** `derives_from: sire:int-0002`, resolved through `[parents]` in
`config.toml`. Since ledgers are just files in git, a parent is fetched read-only —
no server, same architecture.

**The orphan flag.** An active intent with no `derives_from` into a parent is surfaced
as drift. At the repo tier it asks *"why does this work exist?"* At the workspace tier
it asks the same question of a whole project.

```
◈ dira brief --workspace sire
BET   sire:int-0002  kazi is the wedge         kazi: 4 done · 2 running · 1 stuck
BET   sire:int-0003  gist = context substrate  gist: 1 running · qst-blocked
BET   sire:int-0005  dira = clarity layer      dira: to be planned
OPEN  sire:qst-0004  monetization model — blocks int-0006
⚠ DRIFT  gist/int-0004 has no sire: parent — why does this work exist?
```

That last line is the mechanism that answers *"what are we actually doing on Sire?"*
It stops being a feeling you reconstruct and becomes a derived report in which
**unexplained work is structurally visible**.

### 8.2 Inherited orientation

`SessionStart` in any repo injects **the chain**, not just the local brief: repo
context + the workspace bet it serves + your current personal focus. The agent in the
kazi repo now knows *"focus is dira v0.1; kazi is maintenance-mode this month"* — and
can say *"this refactor is interesting but conflicts with `me:cst-0002`; log it as a
note and stay on target?"*

Constraints cut both ways. dira pushing back on **you** drifting is precisely what
you want from a compass.

### 8.3 The privacy boundary

The personal ledger is private. Repo ledgers ship publicly. Three absolute rules
(cst-0003):

1. **Downward only.** A child reads its parents. No verb writes upward.
2. **Read-time only.** Inherited context is materialized into a brief in memory,
   never persisted into the child's `entries/`, so it cannot be committed by accident.
3. **No leakage through derived artifacts** — not into a mirrored ADR, an export, a
   hosted render, or a check message written to a public path.

A private strategic note committed into public git history cannot be un-published.
Any violation is a **security** bug, not a UX bug.

### 8.4 What this pointedly does not become

Not tasks. Not calendar. Not journal. Not a second brain. Obsidian was failing at the
direction layer, and the direction layer is the *only* layer dira takes (int-0003).
The closed entry set is the structural guard.

---

## 9. Surfaces

The insight that makes UI tractable: dira's UI is **not an editor**. It is a
*review-and-dispose surface*. Agents propose; you dispose. That is taps, not typing —
which is why it maps to mobile unusually well.

**Tier 1 — `dira ui`, embedded in the binary.** Server-rendered Go `html/template`
pages served on localhost over the same query engine the CLI uses — **not an SPA**
(`dec-0012`): the built screens contain zero JavaScript, and client-side rendering
would make the decision pages uncrawlable, defeating the long-tail channel the growth
loop depends on. No install, no server, works offline. This is
where "beautiful" lives first, and the data deserves it: the brief as a composed daily
artifact rather than terminal text; the intent tree as a zoomable map (you → sire →
kazi) with orphan work glowing at the edges; decisions as a why/alternatives/
supersession timeline; the edge graph as an actual navigable graph.

**Tier 1b — the public renderer.** `dira render` emits static HTML the user deploys
wherever they already publish (`dec-0012`). No dira-operated host exists, which is what
makes cst-0004 structural rather than a promise.

**Tier 2 — mobile, with GitHub as the backend.** The ledger is files in git, so GitHub
is *already* dira's sync server. A PWA (or thin native shell) reads ledgers through
the GitHub API over OAuth and writes by committing. Tap confirm on the phone → a
commit flips `dec-0058` to accepted → the laptop's next `dira reindex` picks it up →
the next session's brief contains it. **No dira server exists anywhere**; auth, sync,
offline caching, and audit history are problems GitHub already solved.

The mobile experience follows from dispose-surface framing:

- **The morning brief as the home screen** — the compass across all workspaces.
- **The distill queue as swipeable cards** — yesterday's staged decisions; swipe right
  to confirm, left to reject, tap to edit the because. Five minutes in a coffee line
  replaces the weekly triage nobody does at a desk.
- **Answer open questions from anywhere** — a question is the entry type that blocks
  work, and answering one is a sentence. Phone-shaped.
- **Capture on the go** — a thought at dinner becomes a `note`, waiting in the next
  brief instead of dying in Apple Notes.
- **Push via a small GitHub Action** on `.dira/` paths: *"kazi session staged 3
  decisions."*

**Tier 3 — optional hosted, much later.** `dira.sire.run` for teams wanting zero-setup
rendering and shared review. Fine as a product, and a natural monetization seam — but
cst-0004 holds absolutely: **dira must never require it.**

The architectural cost of all three is a single abstraction committed to on day one
(dec-0005): a storage interface (`local` | `github`) behind one query/write engine,
with every mutation expressed as an entry-file change. Then CLI, agent, web, and phone
are all just different lenses on the same commits.

---

## 10. Lifecycle of an idea, end to end

One feature, all the way through. Say it is *"runs should survive a laptop reboot."*

**1. Session start — you type nothing.** The `SessionStart` hook injects
`dira brief --context`. Claude already knows int-0002 (single-binary DX), dec-0042
(event-sourcing rejected), qst-0007 open. Both of you are oriented before the first
prompt.

**2. You brainstorm.** Twenty minutes in you say *"let's go with a resume-token
checkpoint file, not a daemon."* The `Stop` hook's `dira sniff` catches the decision
language and stages it. You say "confirm." **That is your entire clerical workload.**

**3. dira writes the record; the ADR is a side effect.** The skill fills in what the
sniffer cannot — the because, the rejected alternative (*daemon: violates the
single-binary intent*), the edge `derives_from: int-0002` — and because
`mirror.adr = true`, emits `docs/adr/0084-checkpoint-resume.md` from the same entry.
One capture, two artifacts. The entry is queryable truth; the ADR is familiar exhaust
for whoever wants it (dec-0009). **You open neither.**

**4. Drift check before planning.** The skill runs `dira check`. It passes — but had
you said *"daemon,"* it would have exited non-zero against dec-0060's rejection
rationale and cst-0004. Fired **before a single predicate is drafted**.

**5. Claude hands the what to kazi.**

```
kazi plan "runs resume after restart from a checkpoint file" --workspace . --adr --json
→ PROPOSED prop-resume-8a1f (3 predicates)
```

dira links `dec-0060 → realized_by: kazi:prop-resume-8a1f`, and the goal-file carries
the return pointer in the `metadata` table kazi already stores verbatim:

```toml
[metadata]
why = "dira:dec-0060"
```

**This is the seam**: rationale points down at the goal, the goal points up at the
rationale — and it needs no change to kazi (dec-0008).

**6. You approve — and the approval is itself a decision event.**
`kazi approve prop-resume-8a1f` flips the linkage from proposal to runnable goal, and
`dira map` now shows dec-0060 as *planned*. (Today the skill wraps this; a
post-disposition hook upstream would make it automatic — qst-0005, issue 2.)

**7. kazi grinds; dira watches, never duplicates.** `kazi apply` loops → *in
progress*. Converges → *completed*, kazi's evidence attached. Goes stuck →
*execution-blocked* under int-0002, sourced from `portfolio --json`, never
hand-entered.

**8. Three weeks later, a new session.** You have forgotten all of it. The brief shows
dec-0060 done under int-0002. You ask *"why didn't we just run a daemon?"* —
`dira why daemon` answers in one screen: the decision, the rejected alternative and
its why_not, the intent it served, the converged goal that realized it, and the ADR
path if you want the long form.

**Division of labour across one idea:** you judge and confirm; Claude brainstorms,
extracts, and plans; dira remembers why and enforces consistency; kazi proves done.
Each artifact is written **once**, by the layer that owns it, and every one is
reachable from every other through the edges.

---

## 11. Onboarding: `dira init --interview`

Capture-going-forward does nothing for the fog that already exists. So the workspace
and personal tiers bootstrap by **interview**: Claude asks what this is in one
sentence, what the live bets are, what has been explicitly decided against, and what
is genuinely undecided — then writes the founding intents, constraints, and open
questions as the seed ledger.

Thirty minutes of interview converts the fog in your head into the compass everything
else hangs from. The existing corpus (kazi's 83 ADRs, gist's docs) then imports
underneath it — though **how** is still open, and the honest risk is real: ADRs record
the choice but rarely the rejected alternatives, so a naive bulk import yields entries
with empty `alternatives` arrays and an enforcer that cannot enforce. Current leaning
is index-everything, promote-on-first-citation (qst-0003).

---

## 12. Business model

Free OSS core, one-off apps, team subscription — three different **customers**, not
competing pricing models (dec-0007).

1. **Now — free forever:** CLI, hooks, skill, `dira ui`. The adoption engine and the
   kazi-org flywheel. Charging here kills the reason the apps are worth buying.
2. **v1 apps — one-off purchase:** iOS + desktop bundle. Data on-device or in the
   user's own GitHub. Privacy-as-product is a genuine strength for this buyer, and it
   costs nothing to serve — no ops, no on-call, no security surface, which matters
   enormously for a solo maintainer already carrying kazi.
3. **When teams pull for it, not before — per-seat subscription:** shared review
   queues, members without GitHub, SSO, org dashboard, notifications. Real ongoing
   cost, real ongoing value, subscription-shaped.

**Why not individual subscription:** a subscription needs recurring cost or recurring
delivered service behind it. Hosted sync is the usual justification and it was
designed away — GitHub is the sync layer, and this audience already has it free.
Monthly rent on an app whose data sits in the user's own git repos reads as rent, and
developer audiences punish that hard.

**Honest caveats:** one-off revenue is a decaying curve per release, and App Store
discovery is brutal without the OSS funnel doing the marketing. Standard fix:
pay-once-per-major-version.

The ordering is the strategy — paid apps fund the OSS credibly rather than the OSS
reading as bait.

---

## 13. Roadmap

MVP order is chosen so that **shipping stage 1 alone already beats every alternative
above.**

| Stage | Contents | Ships when |
|---|---|---|
| **M1 — the ledger** | entry schema · storage interface (`local`) · `log` `why` `brief` `reindex` · SQLite cache · `SessionStart` injection | `dira brief` renders under 100ms cold and one real decision survives a week |
| **M2 — capture** | `dira sniff` · the Claude Code skill · `Stop` + `PreCompact` hooks · staged-entry disposition | a four-hour session's decisions land without being typed |
| **M3 — the enforcer** | `dira check` · `supersede` · constraint inheritance | one genuine relitigation attempt is caught |
| **M4 — derived status** | `kazi portfolio --json` join · `dira map` · decision-blocked detection | `dira map` matches reality with no hand-entry |
| **M5 — tiers** | workspace + personal ledgers · namespaced refs · orphan drift · `init --interview` | "what are we doing on Sire?" is answered by a report |
| **M6 — surfaces** | `dira ui` server-rendered templates · `dira render` static output (dec-0012) | — |
| **M7 — apps** | `github` storage backend · PWA · paid apps | — |

Upstream kazi asks — only **two**, both additive, neither breaking (qst-0005):
register `portfolio` in `Kazi.CLI.Schema`, and a post-disposition hook on
`approve`/`reject`. Downward traceability needs nothing: goal-files already carry a
verbatim `metadata` table (`loader.ex:24`), so `metadata.why` works today.

---

## 14. Open questions

Tracked as real entries, not a list that rots:

| Ref | Question | Blocks |
|---|---|---|
| [qst-0001](../.dira/entries/qst-0001.md) | How does a public repo ledger resolve a parent ref in a **private** ledger? | dec-0006 — the workspace tier cannot ship until settled |
| [qst-0002](../.dira/entries/qst-0002.md) | Attention drift from ledger writes only, or session metadata? *(or don't build it — it is the one proposed feature closer to self-surveillance than navigation)* | nothing |
| [qst-0003](../.dira/entries/qst-0003.md) | Does bulk-importing 83 ADRs produce a useful ledger or a second pile? | int-0003 |
| [qst-0005](../.dira/entries/qst-0005.md) | Will kazi accept the two upstream additions? | dec-0008 (fallbacks exist for both) |

---

## 15. Founding ledger index

| Ref | Title |
|---|---|
| [int-0001](../.dira/entries/int-0001.md) | The why of a decision survives the session that made it |
| [int-0002](../.dira/entries/int-0002.md) | Zero-ceremony operation — one binary, no server, no daemon |
| [int-0003](../.dira/entries/int-0003.md) | Replace the tool pile — kazi + dira + a subset of skills |
| [dec-0001](../.dira/entries/dec-0001.md) | Go, not Elixir/OTP, despite kazi's stack |
| [dec-0002](../.dira/entries/dec-0002.md) | One file per entry, not an append-only JSONL ledger |
| [dec-0003](../.dira/entries/dec-0003.md) | No model client in the binary |
| [dec-0004](../.dira/entries/dec-0004.md) | Execution status is derived from kazi, never stored |
| [dec-0005](../.dira/entries/dec-0005.md) | A storage interface committed to before the first surface |
| [dec-0006](../.dira/entries/dec-0006.md) | Tiers are fractal |
| [dec-0007](../.dira/entries/dec-0007.md) | OSS free; individuals buy once; teams subscribe |
| [dec-0008](../.dira/entries/dec-0008.md) | Integrate with kazi only through its `--json` contract and hooks |
| [dec-0009](../.dira/entries/dec-0009.md) | ADRs are mirrored exhaust; the entry is the record |
| [cst-0001](../.dira/entries/cst-0001.md) | The brief never exceeds 1,500 tokens |
| [cst-0002](../.dira/entries/cst-0002.md) | The entry set is closed at five kinds |
| [cst-0003](../.dira/entries/cst-0003.md) | Inheritance is one-way and read-time only |
| [cst-0004](../.dira/entries/cst-0004.md) | Never requires a network service, account, or hosted tier |
| [qst-0001](../.dira/entries/qst-0001.md) | Resolving a parent ref across the privacy boundary *(open)* |
| [qst-0002](../.dira/entries/qst-0002.md) | Attention drift — ledger writes or session metadata? *(open)* |
| [qst-0003](../.dira/entries/qst-0003.md) | Does importing an ADR corpus help or make a second pile? *(open)* |
| [qst-0004](../.dira/entries/qst-0004.md) | Is the name clear of collisions? *(answered — yes)* |
| [qst-0005](../.dira/entries/qst-0005.md) | Will kazi accept the two upstream additions? *(open)* |

21 entries: 3 intents, 9 decisions, 4 constraints, 5 questions. All validate against
[`schema/entry.schema.json`](../schema/entry.schema.json); no dangling edges.
