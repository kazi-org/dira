<h1 align="center">dira</h1>

<p align="center">
  <b>Never explain the same decision twice.</b>
</p>

<p align="center">
  Your coding agent has amnesia. You keep re-explaining decisions you already made —<br>
  and it keeps suggesting the thing you rejected in July. dira remembers why.
</p>

<p align="center">
  <a href="docs/design.md">Design</a> &nbsp;&middot;&nbsp;
  <a href="schema/entry.schema.json">Entry schema</a> &nbsp;&middot;&nbsp;
  <a href=".dira/entries">Our own ledger</a> &nbsp;&middot;&nbsp;
  <a href="https://github.com/kazi-org/kazi">kazi</a>
</p>

<p align="center">
  <i>dira</i> — Swahili for <i>compass</i>.
</p>

---

> **Status: design phase.** What exists is the founding design
> ([`docs/design.md`](docs/design.md)), the entry schema, dira's own ledger under
> [`.dira/`](.dira/entries) — the tool's first user is itself — and a Go module you
> can [build from source](#build-from-source). The binary currently answers `--help`
> and `--version` and nothing else: no `log`, no `why`, no `brief` yet. Stars and
> issues welcome; `brew install` is not a thing yet.

## The problem

You spend hundreds of hours brainstorming with a coding agent. You reach real
agreement. Then you lose it.

Not for lack of writing it down — there are 83 ADRs in the sibling repo. You lose it
because **reading is pull-based and iteration outruns reading**. So week 12
relitigates week 1, and the agent, who remembers nothing across sessions, helps you do
it.

Existing tools each miss a different half:

- **Trackers** (Linear, Jira, Beads) know *what* and *when*. Not *why*, and never
  *what you rejected*.
- **ADRs and spec docs** capture rationale into files nobody reopens. They fix
  capture. The broken thing is retrieval.
- **Notes apps** (Obsidian) hold everything and therefore own nothing.
- **kazi** proves a goal is objectively done — and deliberately refuses to decide what
  to build.

Unclaimed: a ledger where **the agent does the writing, you never open a file, and the
system actively refuses to let decisions be quietly contradicted.**

## What dira is

A git-native ledger of **intents, decisions, rejected alternatives, open questions,
and constraints**, sitting one layer above execution.

```
        dira  ─── intents · why · why-not · open questions        (upstream of a goal)
          │
          ▼  a declared goal
        kazi  ─── predicates · convergence · evidence             (downstream of a goal)
```

Three inversions make it work, and each is where a predecessor failed:

**1. Capture is the agent's job.** Hooks on `SessionStart`, `Stop`, and `PreCompact`
pull decisions out of the transcript. Your whole clerical workload:

```
dira: staged dec-0060 "Checkpoint file for run resume (no daemon)"
      — confirm or ignore
```

**2. Review is push, not pull.** You never open a file. One screen at session start,
injected into the agent's context at the same moment — hard-capped at 1,500 tokens
forever, because a brief that grows without bound is just the ADR pile again.

**3. It's an enforcer, not a notebook.** Before work gets planned:

```
$ dira check "add a background daemon to track run state"
✗ conflicts with dec-0060 (accepted 2026-07-03)
    rejected alternative: "a daemon"
    why_not: violates the single-binary intent (int-0002)
    revisit_if: cold-start latency stops being the binding constraint
→ supersede dec-0060, or revise the plan
```

That is the part ADRs never did.

## What it answers

**"What's planned, in progress, blocked?"** — without a single hand-entered status
field. dira stores none of it; it joins its ledger against `kazi portfolio --json`
across `realized_by` edges at read time. A derived view can't go stale.

Two buckets matter most, and one of them no execution tracker can see:

- **To be planned** — decided, but no goal exists yet. The gap list.
- **Decision-blocked** — an open question gating the work. The work stalled *before*
  any goal existed, because a human never answered something. That appears in no
  tracker anywhere.

**"Why did we do it this way?"**

Real output, from this repo's own ledger — abridged, because each rejected
alternative carries its full grounds:

```
$ dira why elixir
int-0002  Zero-ceremony operation — one binary, no server, no  active 2026-07-29
          daemon
            cold-start latency is the UX, and that is a runtime property
└─ dec-0001  Go, not Elixir/OTP, despite kazi's stack        accepted 2026-07-29
   ├─ ✗ Elixir/OTP, reusing kazi's Burrito + Homebrew tap + release-please
   │    pipeline
   │    dira is a short-lived, hook-invoked CLI that runs several times per
   │    session in the latency path of a human waiting on a prompt. BEAM
   │    start-up is tens to hundreds of milliseconds before any work happens...
   │    revisit if  dira grows a genuinely long-lived component
   ├─ ✗ Rust
   ├─ ✗ A shell script or Python
   └─ ✗ A TypeScript CLI on Node/Bun
```

The spine reads top-down: the intent it serves, then the decision, then every
alternative that was rejected *and why*. Only rejections are listed — what was
chosen is the decision itself. `revisit if` is the condition that would reopen
it, which is the thing a chat log never records.

**"What are we actually doing across all my projects?"** — the model is fractal. One
entry model, one join rule, one drift flag, at every level:

```
~/.dira/            you       — private, syncs across machines
└─ sire/.dira/      venture   — bets
   └─ kazi/.dira/   repo
```

Any active intent with no link to a parent gets flagged as **orphan work** — so
unexplained work becomes structurally visible instead of a feeling you reconstruct.
And `SessionStart` injects the whole chain, so the agent in one repo knows your
current focus is elsewhere and can say so.

## Design commitments

These are constitutional. Each is a real entry in [`.dira/`](.dira/entries) with its
rejected alternatives recorded, so overruling one is a one-line supersede rather than
an argument.

| | |
|---|---|
| **One binary, no server, no daemon** | Invoked from hooks in a human's latency path. Works with the network unplugged. |
| **No model client in the binary** | No API key, no vendor lock-in. Semantic extraction is delegated to the session that's already running. |
| **Status is derived, never stored** | Hand-entered status is exactly what went stale in Obsidian and Linear. |
| **Five entry kinds, closed set** | A sixth kind is how a direction tool becomes a second brain. |
| **The brief never exceeds 1,500 tokens** | Enforced by the binary, not by taste. |
| **Private context never enters a public ledger** | Inheritance is downward-only and read-time-only. A violation is a security bug. |
| **Never requires an account or a hosted tier** | Your data never touches our servers — literally, because there are no servers. |

## Where the data lives

One file per entry — YAML frontmatter for the machine, markdown body for the *because*:

```
.dira/
├── config.toml          # the only hand-edited file
├── entries/             # the ledger, committed to your repo
│   ├── int-0001.md
│   └── dec-0001.md
└── cache/               # derived SQLite, gitignored, rebuildable
```

Per-entry files rather than an append-only log, because capture is automatic and
unattended: two sessions logging at once create two files, which git merges without a
conflict. It also means one write per mutation — and therefore one GitHub `PUT`, which
is what lets a phone be a first-class client with **no dira server anywhere**. GitHub
is already the sync layer; it was already paid for.

Full rationale: [dec-0002](.dira/entries/dec-0002.md).

## Status & roadmap

| Stage | Contents |
|---|---|
| **M1** | entry schema · storage interface · `log` `why` `brief` `reindex` · `SessionStart` injection |
| **M2** | `dira sniff` · Claude Code skill · `Stop` + `PreCompact` hooks |
| **M3** | `dira check` · `supersede` · constraint inheritance |
| **M4** | kazi join · `dira map` · decision-blocked detection |
| **M5** | workspace + personal tiers · orphan drift · `init --interview` |
| **M6** | `dira ui` · **the public ledger renderer** (the growth engine — [dec-0010](.dira/entries/dec-0010.md)) |
| **M7** | GitHub storage backend · PWA · paid apps |

M1 alone is already more useful than any of the alternatives above. Detail and open
questions: [`docs/design.md`](docs/design.md), [`docs/roadmap.md`](docs/roadmap.md).

## Relationship to kazi

Siblings, neither depending on the other. kazi's README draws the line itself: it will
never *"decide what to build — that's your judgment."* That sentence is dira's
charter — dira owns everything upstream of a declared goal.

kazi runs fine with no dira installed. dira degrades to its ledger-side views with no
kazi installed, and says so rather than guessing. Integration is only through kazi's
public `--json` contract ([dec-0008](.dira/entries/dec-0008.md)) — never its
internals.

## Build from source

Go only — the toolchain version is the one pinned in [`go.mod`](go.mod), and there is
nothing else to install.

```
git clone https://github.com/kazi-org/dira
cd dira
go build ./cmd/dira
./dira --help
```

`dira --version` prints `dev` from a plain build. Release builds stamp the tag in at
link time:

```
go build -ldflags "-X main.version=1.2.3" ./cmd/dira
```

Run the tests, which include the gate that validates every entry in
[`.dira/entries/`](.dira/entries) against
[`schema/entry.schema.json`](schema/entry.schema.json):

```
go test ./...
```

The command path is stdlib-only and a test enforces it, because dira runs inside a
hook while you wait for a prompt ([int-0002](.dira/entries/int-0002.md),
[dec-0001](.dira/entries/dec-0001.md)). Exit codes are a contract: `0` success,
`1` runtime error, `2` usage error.

## License

Apache License 2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).
