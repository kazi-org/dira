# dira — program plan (L0)

**Level 0 of a three-level plan tree.** L0 defines epics. Each epic has a
dispatchable prompt at `docs/plan/prompts/L1-<id>.md` that decomposes it into lanes
*and emits the L2 prompts for those lanes*. L2 produces tasks.

```
L0  docs/plan.md              epics        (this file — authored by the lead)
L1  docs/plan/lanes/<id>.md   lanes        (written by an L1 agent per epic)
L2  docs/plan/tasks/<id>.md   tasks        (written by an L2 agent per lane)
```

**Last updated:** 2026-07-29 · **Owner:** maintainer

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

## The critical path

E0 is the gap this exercise exposed: **the roadmap assumed a Go project that does
not exist.** `dec-0001` chose Go over Elixir and noted the cost — goreleaser instead
of Burrito, a second Homebrew formula, a separate release workflow — but no milestone
ever owned paying it. Nothing else can ship until it does.

```
E0 foundations ──► E1 ledger ──► E2 capture ──► E3 enforcer
                       │              │
                       │              └──► E4 derived status ──► E5 tiers*
                       │
                       └──► E6 surfaces & renderer ──► E7 apps

E8 go-to-market ──► runs alongside from E1; gates on E6 for launch assets
E9 upstream kazi ──► independent, unblocks part of E4

* E5 was blocked on qst-0001, ANSWERED 2026-07-30 by dec-0011. L2 may now be planned.
```

---

## Epics

### E0 — Foundations: a Go project that ships
**Maps to:** nothing in the roadmap — this is the exposed gap.
**Why:** `dec-0001` accepted the cost of leaving kazi's Burrito/Homebrew pipeline
and no milestone owns it. Every other epic assumes a buildable, releasable binary.
**Scope:** Go module layout, the `dira` cobra-or-stdlib command skeleton, CI on
push, goreleaser config, a formula in the existing `kazi-org/homebrew-tap`,
release-please or equivalent versioning, and the entry-schema validator wired as a
test so `.dira/` cannot drift from `schema/entry.schema.json`.
**acc:** `go build ./...` succeeds; `go test ./...` passes with a schema-validation
test that fails when a fixture entry violates `entry.schema.json`; **CI runs
`python3 scripts/coverage.py` and the build fails on a non-zero exit**; a tagged push
produces a downloadable darwin-arm64 + linux-amd64 binary; `brew install kazi-org/tap/dira` installs it.
**Scope note (2026-07-30):** E0-L2 ("make the ledger self-validating in Go") is
**partly landed already**. E0-L1 shipped `schema/entry_test.go` with 17 invalid
fixtures, and E1-L1 exported the validator (`schema/schema.go`: `NewValidator`,
`Validator.Validate`, `SplitFrontmatter`) because it was test-only and unreachable from
another package. What remains for E0-L2 is CI wiring, not the validator itself.
**Prompt:** `docs/plan/prompts/L1-E0.md`

### E1 — The ledger
**Maps to:** roadmap M1.
**Scope:** the storage interface with its `local` backend (`dec-0005`), entry
read/write as one file per entry (`dec-0002`), the SQLite derived cache under
`.dira/cache/`, `dira log`, `dira why`, `dira brief`, `dira reindex`, and
`SessionStart` brief injection with the 1,500-token cap enforced in-binary
(`cst-0001`).
**Measured 2026-07-30 by E1-L3, and it constrains E1-L5/L6:** the SQLite cache makes a
warm `brief` 15.1ms against a 30.1ms no-cache path — but a **cold** cache costs 55.5ms,
*worse than no cache at all*, because it pays for building the database. `dira brief`
runs on `SessionStart`, so the very first session after a clone or a `reindex` is the
slowest one. E1-L5 must decide what the brief does on a cold cache (build it
synchronously and eat 55ms, fall back to the uncached read path, or build in the
background and serve uncached once), and E1-L6's budget must state which case it
measures. A budget that only ever measures the warm path is not measuring the user's
first experience.

**Decision E1-L3 was handed and has now made (`dec-0015`): `EntryInfo.Version` is a
content hash, not mtime+size** — the heuristic's hole was judged reachable rather than
theoretical, and the acceptance line promised the files win, which a heuristic makes a
near-certainty rather than a guarantee. Original framing kept below for the record.

**Decision handed to E1-L3, do not inherit it silently:** `EntryInfo.Version` on the
local backend is `mtime-nanos + size` — the standard heuristic (git, make, rsync), and
what keeps `List` at 2.7ms rather than 13ms. It is a heuristic: an in-place edit
preserving both size and mtime is invisible to it. E1-L3's own acceptance says a
cache/file disagreement **must** resolve to the file. If that is a guarantee rather than
a near-certainty, `Version` becomes a content hash at ~13ms per full scan. The interface
supports either with no signature change — E1-L3 chooses and records which.

**acc:** `dira brief` completes its own work in **<40ms** on a cold cache over a
200-entry ledger, measured **excluding process spawn**.
*Clarified 2026-07-30 after measuring the E0-L1 skeleton:* the original "<100ms" was
ambiguous in a way that mattered. On this machine `/usr/bin/true` costs ~88ms of pure
process-spawn overhead, and the dira skeleton costs ~99ms — so the binary's own cost is
~11ms and ~88ms is a floor no implementation can beat. A wall-clock "<100ms" budget
would therefore be nearly unmeetable regardless of code quality, and would fail on a
slower machine for reasons unrelated to dira. E1-L6 must measure dira's own work
(in-process timing around the query, or wall-clock minus a `/usr/bin/true` baseline
captured on the same host) and state which.
`dira brief --context` output is ≤1500 tokens measured by the binary and drops by
priority rather than truncating mid-render; `dira reindex` rebuilds the cache from
files alone and a cache/file disagreement resolves to the file.
**Prompt:** `docs/plan/prompts/L1-E1.md`

### E2 — Capture
**Maps to:** roadmap M2.
**Scope:** `dira sniff` regex tier writing `state: staged` + `source.tier: regex`
only (`dec-0003`), the Claude Code skill for the semantic tier, `Stop` and
`PreCompact` hooks, `dira install-hooks` matching kazi's merge-never-clobber
contract (`dec-0008`), the staged-entry disposition flow, and — **added 2026-07-30** —
**invoking `dira check` before planning.** E3 owns the verb; E2 owns making it fire,
because E2 already holds all the hook-wiring knowledge. A perfect enforcer nothing
calls enforces nothing.
**acc:** a recorded transcript fixture containing decision language yields ≥1 staged
entry and zero accepted entries; `dira install-hooks` is idempotent and
`--uninstall` restores the settings file byte-identically; hooks fail open — a
non-zero `dira` exit never blocks a session; **a plan contradicting a recorded
rejection is blocked before predicates are drafted, demonstrated end to end rather
than by calling the verb by hand.**
**Prompt:** `docs/plan/prompts/L1-E2.md`

### E3 — The enforcer
**Maps to:** roadmap M3.
**Scope:** `dira check` reading `alternatives[].why_not` from rejected *and*
accepted decisions plus inherited constraints, `dira supersede`, constraint
inheritance down the tier chain, and `revisit_if` surfacing so the check can say more
than "no" **where one exists** — `revisit_if` lives on an alternative, so a constraint
conflict can only offer "supersede it in writing". That asymmetry is accepted, not a
schema gap; the message must not imply otherwise.
**Owns the dec-0060 fixture** (the rejected "a daemon"), at
**`internal/enforcer/testdata/ledgers/daemon`** — the path E3's own acceptance lines
already use. *Corrected 2026-07-30: an earlier version of this line said
`fixtures/demo-ledger/`, which contradicted E3's acc and would have produced exactly
the two-fixtures-diverging failure it was written to prevent.* E3 authors it, E8-L3
reuses that path and does not author its own.
**acc:** `dira check "add a background daemon"` exits non-zero against a fixture
ledger containing dec-0060 and names both the conflicting entry and its `why_not`;
`dira check` on a compliant plan exits 0; a superseded decision stops being
enforced and its superseder is enforced instead.
**Dispatch note (2026-07-30):** E3-L1's lane table records its dependency as `none` /
"nothing buildable", but its own `acc:` line requires
`go test ./internal/enforcer -run TestCorpusWellFormed`. That is a **false `none`** — in
a table whose preamble warns that a false `none` deadlocks a wave. E3-L1 is startable
now for the *data*: author the labeled corpus, its sha256 freeze, the fixture ledgers,
and the conflict-detection decision entry, and **specify** the Go well-formedness test
in prose. It creates no `.go` file and no `go.mod`, and it is not *green* until E0-L1's
layout exists.
**Prompt:** `docs/plan/prompts/L1-E3.md`

### E4 — Derived status
**Maps to:** roadmap M4.
**Scope:** the `kazi portfolio --json` join across `realized_by` edges, `dira map`
grouped by why, the six buckets from `dec-0004`, decision-blocked detection, and
graceful degradation when kazi is absent.
**Added 2026-07-30 (`dec-0013`) — a sixth lane, E4-L6: reduce `/sitrep` to a judgement
layer over `dira map`.** `~/.claude/skills/sitrep` is a hand-executed version of this
epic's output, tested across ~a dozen sessions, and it is `int-0003`'s first specific
tool-reduction target. Its four display invariants bind `dira map`: no bare refs
(every ref carries its title), provenance rendered not merely stored, a deferral with
no revival trigger reported as a defect, and contradictions surfaced never silently
resolved. **dira must not absorb the verdict, the weighted percentage, the risk list or
the trend** — those are judgement and would need a narrative field `cst-0002` forbids.
**acc:** against **recorded kazi fixtures** (`portfolio --json` *and* per-ref
`status --json` — portfolio alone cannot supply the running/done split, see dec-0008),
`dira map` reports every bucket correctly and distinguishes execution-blocked from
decision-blocked; with the
`kazi` binary absent `dira map` still renders ledger-side buckets and states that
execution status is unavailable rather than guessing; no status value is ever
persisted into an entry file; **every ref in `map` output is accompanied by its title
(a bare-ref grep over the output returns nothing), provenance renders for each entry,
and a deferred item with no revival trigger is reported as a defect rather than
omitted**; and `/sitrep`'s gather phase, currently five sources hand-glossed, reduces
to `dira map` plus `gh`.
**Prompt:** `docs/plan/prompts/L1-E4.md`

### E5 — Tiers
**Maps to:** roadmap M5. **Unblocked 2026-07-30:** `qst-0001` is answered by
`dec-0011` — cross-boundary refs publish opaquely, resolution reports three states
(oriented / withheld / orphan), only orphan is drift.
**Scope:** workspace + personal ledgers, namespaced ref resolution, the orphan-work
drift flag, `dira init --interview`.
**Instruction:** lanes exist (`docs/plan/lanes/E5.md`) and L2 may now be emitted
against `dec-0011`'s model. Note that E3-L3 lands the `[parents]` reader — E5 extends
it rather than duplicating it.
**Prompt:** `docs/plan/prompts/L1-E5.md`

### E6 — Surfaces and the renderer
**Maps to:** roadmap M6. **Design is done** — `docs/design/` carries tokens, three
gate-verified screens, and DESIGN.md. This epic is implementation, not design.
**Scope:** `dira ui` as server-rendered Go `html/template` + `embed.FS` (**not an SPA** — `dec-0012`), `dira render` emitting static HTML the user deploys, the
read-only public ledger renderer (`dec-0010`), and the three open design questions
in DESIGN.md (chain-at-scale collapse, long-content verification, the private-parent
state).
**acc:** `dira ui` serves on localhost with the network unplugged and renders the
real `.dira/` ledger, not fixtures; the rendered pages reproduce
`docs/design/screens/` within a pixel tolerance **that E6 must first measure and
record in DESIGN.md** (none is recorded today, so the clause was unfalsifiable);
`docs/design/scripts/contrast.mjs` reports 0 failures with hover exceeding rest in
both schemes; a public ledger URL renders for a signed-out visitor.
**Prompt:** `docs/plan/prompts/L1-E6.md`

### E7 — Apps
**Maps to:** roadmap M7. **Do not plan below lane level yet** — gated on E6
shipping and on real adoption evidence, per `dec-0007`'s "when teams pull for it,
not before".
**Scope:** the `github` storage backend, the PWA, the paid iOS + desktop bundle.
**Prompt:** `docs/plan/prompts/L1-E7.md`

### E8 — Go-to-market
**Maps to:** roadmap has no GTM section — add one.
**Scope:** the distribution plan (channel selection for a solo engineer, Claude Code
ecosystem placement, the Show HN asset), the landing page, the `dira check` demo
clip, and the launch sequence. Positioning is **already done** in
`.agents/product-marketing.md` — read it first and do not re-derive it.
**acc:** every channel in the plan names its owner, its cadence, and a numeric
success threshold pre-registered before launch; the demo clip is ≤20s and needs no
narration; the landing page passes the same contrast matrix and 3×2 render gate as
`docs/design/`.
**Prompt:** `docs/plan/prompts/L1-E8.md`

### E9 — Upstream kazi
**Maps to:** roadmap "Upstream asks". Independent of everything else.
**Scope:** two issues at `kazi-org/kazi` — register `portfolio` in
`Kazi.CLI.Schema`, and a post-disposition hook on `approve`/`reject` that is a
direct synchronous invocation, **not** a `bus post` (the bus needs a daemon and
no-ops when it is down — see `qst-0005`).
**acc:** both issues filed with a reproduction and a proposed contract; neither
duplicates an existing issue.
**Prompt:** `docs/plan/prompts/L1-E9.md`

---

## Epics deliberately absent

- **A tasks/tracker epic.** kazi owns execution and doneness (`int-0003`, `cst-0002`).
- **Attention-drift scoring.** `qst-0002` — may never be built, and that is an
  acceptable outcome.
- **A hosted service anyone must use.** `cst-0004`.

---

## How to run the tree

```bash
# L1: decompose one epic into lanes AND emit its L2 prompts
#   feed docs/plan/prompts/L1-E1.md to an agent

# L2: decompose one lane into tasks
#   feed the emitted docs/plan/prompts/L2-<lane>.md to an agent

# then converge the leaves
kazi plan "<task acc line>" --workspace .
```

Every L1 prompt ends with an instruction to write its own L2 prompts, so the tree
propagates without the lead authoring each level by hand.
