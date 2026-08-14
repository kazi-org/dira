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

> **Status: working, pre-release.** 14 verbs are real and tested against this repo's
> own 43-entry ledger — capture, review, enforcement, cross-project tiers, ADR
> import, and a read-only web surface all run today. Build it from source (below);
> there is no `brew install` yet. Stars and issues welcome.

## What dira is

**A memory of why, kept in the repo as plain files, written mostly by the coding
agent rather than by the human.** Every entry is a markdown file with YAML
frontmatter under `.dira/entries/` — an intent, a decision, a rejected alternative,
an open question, or a constraint — committed alongside the code it governs.

## The problem it solves

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

```
        dira  ─── intents · why · why-not · open questions        (upstream of a goal)
          │
          ▼  a declared goal
        kazi  ─── predicates · convergence · evidence             (downstream of a goal)
```

## The core verbs

Every line below is a real run against the binary built from this worktree
(`go build ./cmd/dira`), output trimmed where noted.

| verb | what it does | example |
|---|---|---|
| `init` | seeds a new ledger from a short fixed interview; all-or-nothing | `dira init --interview` → `seeded a new workspace ledger at .dira` |
| `log` | writes a new entry, or adds an edge to an existing one | `dira log --kind decision --title "…" --alternative "…" --why-not "…"` → `dec-0001` |
| `sniff` | reads the session transcript and **stages** candidate decisions; never accepts one | `dira sniff --stage --quiet` → `dira: staged dec-0002 "Let's go with X instead of Y" — confirm or ignore` |
| `distill` | the review screen: one keystroke per staged capture (`y`/`n`/`e`/`u`/`q`), with undo | run without a terminal on stdin: `dira distill: 1 capture awaiting a human; stdin is not a terminal, so nothing was read and nothing was changed` |
| `check` | refuses a plan that contradicts a settled decision, quoting the original reason | `dira check "rewrite dira in Elixir using OTP"` → `✗ conflicts with dec-0001 …` (exit 2) |
| `why` | prints the chain: what an entry arose from, every alternative it refused, and why | `dira why elixir` → the full spine below |
| `brief` | the session-start screen: open blockers, current focus, recent decisions — capped at 1,500 tokens | `dira brief` |
| `supersede` | retires an entry in favour of the one that replaces it, writing both sides | `dira supersede dec-0001 --with dec-0003 --note "…"` → `dec-0001 is superseded by dec-0003; dira check now cites dec-0003 in its place` |
| `ui` | serves the ledger index and per-entry pages on loopback, no JS required | `dira ui -addr 127.0.0.1:8942` → `serving the ledger read-only; ctrl-c to stop` (`curl` returns `200`) |
| `install-hooks` | merges dira's Claude Code hook registrations (`SessionStart`/`Stop`/`PreCompact`) into a settings file, merge-never-clobber; defaults to `~/.claude/settings.json` | `dira install-hooks --dir DIR` → `INSTALLED DIR/settings.json` |
| `install-skill` | writes dira's tier-2 capture skill into `~/.claude` for Claude Code to load; defaults to `~/.claude` | `dira install-skill --root DIR` → `INSTALLED DIR/skills/dira/SKILL.md` |
| `reindex` | rebuilds the derived SQLite cache from the entry files alone | `dira reindex` → `indexed 43 entries and 87 edges from .dira into .dira/cache` |
| `import` | measures a directory of ADRs, reports the yield, and asks before writing; see below | `dira import DIR` → `2 documents scanned` / `2 record a rejected option with a reason` |
| `version` | prints the binary's version | `dira version` → `dev` on a plain build |

Run `dira --help` for the authoritative, current list — that is the source this
table was built from, not the other way around.

Real `why` output, from this repo's own ledger — abridged, because each rejected
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

## How capture works

Capture is the agent's job, not yours. `dira install-hooks` writes three
registrations into Claude Code's settings — this is the literal command each one
runs, taken from `hooks/settings.example.json`:

| hook | command | job |
|---|---|---|
| `SessionStart` | `dira brief --context --chain` | inject the brief into the agent's context before the first prompt — review is push, not pull |
| `Stop` | `dira sniff --stage --quiet` | tier-1 capture: a regular expression reads the last turn and **stages** anything that looks like a decision |
| `PreCompact` | `dira sniff --deep --stage --all` | the insurance policy — fires before Claude Code's lossiest moment so a long session can't evaporate into a compaction summary |

`sniff` is deliberately underpowered: `dec-0003` gives a regular expression no
business asserting rationale, so everything it finds is written `state: staged`
with `source.tier: regex` and nothing else — no because, no alternative, no ADR.
Confirming a capture (`distill`, pressing `y`) does not accept the decision; it
hands the entry to the semantic tier — a Claude Code skill (`dira install-skill`)
that has the actual conversation in context and can fill in the *why* a regex
cannot see.

## How enforcement works

Before work gets planned, run the plan past the ledger:

```
$ dira check "rewrite dira in Elixir using OTP"
✗ conflicts with dec-0001 (accepted 2026-07-29)
    rejected alternative: "Elixir/OTP, reusing kazi's Burrito + Homebrew tap +
    release-please pipeline"
    why_not: dira is a short-lived, hook-invoked CLI that runs several times
    per session in the latency path of a human waiting on a prompt. BEAM
    start-up is tens to hundreds of milliseconds before any work happens...
    revisit_if: dira grows a genuinely long-lived component — a watch daemon
    or a hosted multi-tenant service
→ supersede dec-0001, or revise the plan
```

Real output from this repo's own ledger, trimmed; exit code `2`. The matching is
lexical and runs entirely inside the binary — no model, no network, no agent in
the loop, which is what makes it usable from a hook. The exit codes are a
contract dira holds everywhere: `0` no conflict, `2` a verdict against the plan,
`1` dira's own error (an unreadable ledger, a bad flag) — a caller must never read
`1` as a verdict. `dira supersede` is the only way past a settled decision: it
writes both sides — the replacement gains a `supersedes` edge, the retired entry's
state becomes `superseded` — and `check` starts citing the replacement instead.

## How import works

An existing pile of ADRs is not thrown away — `dira import DIR` measures it before
writing anything. Real output, run against a scratch ledger in a temp directory
seeded with two documents from the vendored `bbc/tams` corpus
(`internal/importadr/testdata/corpora/bbc-tams`), never against this repo's own
`.dira`:

```
$ dira import /tmp/dira-import-demo/adrs
2 documents scanned
2 record a rejected option with a reason
23 reasons found
Import the 2 entries that carry a reason?
```

That's the report-and-ask mode `dira import DIR` runs by default — nothing is
written until the prompt is answered yes, either interactively or with `--yes`.
Confirming imports one staged `decision` entry per document, each `alternatives`
list built from the document's own rejected options and `source.hook: import`
recording which file and content hash it came from — real output, trimmed to one
alternative:

```yaml
alternatives:
  - option: "Option 1: Add a Source representation"
    why_not: >
      Source data are potentially duplicated between this API and other systems
      (e.g. external MAM/PAM systems)
source:
  hook: import
  excerpt: "imported from 0002-add-sources-to-api.md (sha256:97bdb1b4…)"
  tier: regex
```

When a corpus yields nothing — no document records a reasoned rejection — import
offers to index it instead: a manifest under `.dira/cache/imports/`, never a
ledger entry, because a `decision` entry `dira check` can't cite anything against
is worse than not having it. `dec-0028` records the evidence this rests on: five
real corpora measured before the importer was built, from 90% of documents
carrying a reason (`bbc/tams`) down to 0% (`nulib/meadow`) — the importer has to
behave correctly at both ends of that range, not just the rich end.

## What it answers

**"What's planned, in progress, blocked?"** — `dira brief` prints open blockers
(unanswered questions gating work), current focus (active intents), and recent
decisions, hard-capped at 1,500 tokens (`brief.max_tokens` in `.dira/config.toml`)
so the brief can never become the ADR pile it exists to replace.

**Cross-project awareness is real but still narrow.** `.dira/config.toml` can
declare parent ledgers under `[parents]` — a personal ledger above a workspace
ledger above a repo, so the model is fractal:

```
~/.dira/            you       — private, syncs across machines
└─ sire/.dira/      venture   — bets
   └─ kazi/.dira/   repo
```

`dira brief --chain` names the declared parents so the agent knows more context
exists elsewhere (`chain: one parent ledger is configured as parent.`, verified
against a real two-ledger setup in this worktree). Edges can point across
ledgers with a namespaced ref (`derives_from=parent:dec-0001`), and
`scripts/privacy-lint.py` enforces that every namespaced edge resolves to a
declared parent namespace — an undeclared one is always a typo, never treated as
private. What is **not** wired to any command yet: automatic orphan-work flagging
and full cross-ledger citation inside `dira why`. The classification logic exists
and is tested (`internal/drift`), but no verb surfaces it today — that lands with
`dira map` (epic E4), which is still an outline, not built.

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
└── cache/                # derived SQLite, gitignored, rebuildable
```

Per-entry files rather than an append-only log, because capture is automatic and
unattended: two sessions logging at once create two files, which git merges without a
conflict. It also means one write per mutation — and therefore one GitHub `PUT`, which
is what lets a phone be a first-class client with **no dira server anywhere**. GitHub
is already the sync layer; it was already paid for.

Full rationale: [dec-0002](.dira/entries/dec-0002.md).

## Install from a Release

Each tagged release publishes a darwin-arm64 archive, a linux-amd64 archive, and a
`checksums.txt`, built by [`.goreleaser.yaml`](.goreleaser.yaml) with
`CGO_ENABLED=0` and no account needed to fetch them:

```
VERSION=0.0.1-rc.1  # the released tag, without the leading v
curl -LO "https://github.com/kazi-org/dira/releases/download/v${VERSION}/dira_${VERSION}_darwin_arm64.tar.gz"
curl -LO "https://github.com/kazi-org/dira/releases/download/v${VERSION}/checksums.txt"
sha256sum --ignore-missing -c checksums.txt
tar -xzf "dira_${VERSION}_darwin_arm64.tar.gz"
./dira --version
```

Swap `dira_${VERSION}_darwin_arm64.tar.gz` for `dira_${VERSION}_linux_amd64.tar.gz` on
linux-amd64.

## Build from source

Go only — the toolchain version is the one pinned in [`go.mod`](go.mod), and there is
nothing else to install.

```
git clone https://github.com/kazi-org/dira
cd dira
go build ./cmd/dira
./dira --help
```

If you intend to commit, install the pre-commit hook once:

```
sh hooks/install.sh
```

A fresh clone's `.git/hooks` is never tracked by git, so without this step the
coverage, privacy, lint and test gates simply do not run locally — CI still enforces
them on push, so the failure is late rather than silent.

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

## Status

**Working, pre-release.** All 14 verbs above are shipped, tested, and enforced
against this repo's own ledger — `go test ./...` is green. What that status means
concretely:

- No `brew install`. `docs/plan/tasks/E0-L5.md` (the Homebrew tap lane) is
  decomposed but **not executed** — there is no `Formula/dira.rb` anywhere yet.
  Build from source (above) until that lands.
- `dira --version` reports `dev` on a plain build; release builds stamp a tag in
  at link time (above). No tag has shipped yet.
- Detail beyond this line — what's in progress, what's next, what's blocked —
  lives in [`docs/roadmap.md`](docs/roadmap.md) and [`docs/plan.md`](docs/plan.md),
  which this README does not duplicate on purpose: a status table copied into two
  places is exactly the kind of thing that goes stale in one of them.

## Relationship to kazi

Siblings, neither depending on the other. kazi's README draws the line itself: it will
never *"decide what to build — that's your judgment."* That sentence is dira's
charter — dira owns everything upstream of a declared goal.

kazi runs fine with no dira installed. dira degrades to its ledger-side views with no
kazi installed, and says so rather than guessing (`.dira/config.toml`'s `[kazi]`
block). Integration is only through kazi's public `--json` contract
([dec-0008](.dira/entries/dec-0008.md)) — never its internals.

## License

Apache License 2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).
