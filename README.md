<h1 align="center">dira</h1>

<p align="center"><b>Never explain the same decision twice.</b></p>

<p align="center">
  Your coding agent has amnesia. You keep re-explaining decisions you already made —<br>
  and it keeps suggesting the thing you rejected in July. dira remembers why.
</p>

<p align="center">
  <a href="https://dira.sire.run">Website</a> &nbsp;&middot;&nbsp;
  <a href="#quick-start">Quick start</a> &nbsp;&middot;&nbsp;
  <a href="#the-core-verbs">Commands</a> &nbsp;&middot;&nbsp;
  <a href="https://github.com/kazi-org/dira/releases/latest">Releases</a> &nbsp;&middot;&nbsp;
  <a href=".dira/entries">Example ledger</a>
</p>

**dira is decision memory for AI coding agents, stored in Git.** It records what
you chose, why you chose it, which alternatives you rejected, and what would make
you reconsider. Claude Code hooks bring that context into new sessions and stage
new decisions for review.

Each entry is a Markdown file with YAML frontmatter in `.dira/entries/`, committed
alongside your code. The CLI runs locally without an account, API key, or model
client. Semantic capture uses the coding agent already in your session.

*dira* is Swahili for *compass*.

## Quick start

Install with Homebrew:

```sh
brew install kazi-org/tap/dira
dira --version
```

Or download a binary for macOS or Linux (ARM64 or AMD64) from
[GitHub Releases](https://github.com/kazi-org/dira/releases/latest). Releases include
`checksums.txt` for verification. Linux ARM64 users should use the release archive;
the Homebrew formula covers macOS ARM64/AMD64 and Linux AMD64.

In the repository where you want to keep decisions:

```sh
dira init --interview
```

The short interview seeds an intent, a constraint, and an open question. Choose
`workspace` for a project ledger. Then record a decision:

```sh
dira log --kind decision --title "Use SQLite for local storage" \
  --alternative "PostgreSQL" \
  --why-not "Local storage must work offline without a database server" \
  --revisit-if "Multiple users need concurrent remote writes"

dira why SQLite
dira check "use PostgreSQL for local storage"
```

`why` shows the decision and its reasoning. `check` reports the conflict, cites
the rejected alternative, and exits with code `2`. Commit `.dira/` with your code;
the derived cache is disposable.

### Connect Claude Code

With `dira` on your `PATH`, install the hooks and capture skill:

```sh
dira install-hooks
dira install-skill
```

These commands merge hooks into `~/.claude/settings.json` and install the skill at
`~/.claude/skills/dira/SKILL.md`. Existing settings and hooks are preserved. Start
a new Claude Code session in your repository to receive the brief.

```sh
dira brief       # Read the current focus, blockers, and recent decisions
dira distill     # Review staged captures in an interactive terminal
dira ui          # Browse the ledger in a local, read-only web view
```

The CLI also works directly from your terminal or another agent's workflow. The
bundled hooks and skill target Claude Code.

## How it works

**Capture → review → recall → check.** dira keeps the reasoning close to the code
and makes it available when the next decision is being made.

### Capture and review

The installed hooks run at three points:

| Claude Code event | Command | Purpose |
|---|---|---|
| `SessionStart` | `dira brief --context --chain` | Supply decision context before work begins. |
| `Stop` | `dira sniff --stage --quiet` | Stage candidate decisions from the session transcript. |
| `PreCompact` | `dira sniff --deep --stage --all` | Capture candidates before the transcript is compacted. |

`sniff` uses regular expressions. It stages candidates; it does not invent reasons
or accept decisions. In `dira distill`, you can confirm, ignore, edit, or undo.
Confirming a capture marks it for semantic extraction and leaves it staged until
its reasoning is supplied. The capture skill lets the session agent fill in that
reasoning from the conversation.

The PreCompact hook preserves candidates, but its output alone does not guarantee
that the agent completes semantic extraction. Review pending captures with
`dira distill`. See the [hook configuration](hooks/settings.example.json) for the
exact commands and delivery constraints.

### Recall and check

`dira brief` summarizes blockers, active intents, and recent decisions in at most
1,500 tokens. `dira why QUERY` follows an entry's reasoning chain, including rejected
alternatives and the conditions that would reopen them.

Run a proposed plan through `dira check` before implementing it. For example,
inside this repository:

```sh
dira check "rewrite dira in Elixir using OTP"
```

The ledger cites `dec-0001`, which records the choice of Go and the reasons for
rejecting Elixir/OTP. Matching is **lexical and offline**: it detects matches to
recorded rejected alternatives, not every possible semantic contradiction. A
clean result means no conflict was detected by that matcher.

For `check`, exit `0` means no conflict, `2` means a conflict, and `1` means the
check failed to run, including invalid arguments. Use these codes in your own
planning workflow; the installed capture hooks do not automatically gate plans.
Other commands generally use `2` for usage errors; consult `dira help COMMAND`.

When circumstances change, record the replacement decision and link it explicitly:

```sh
dira supersede dec-0001 --with dec-0002 --note "Requirements have changed"
```

Use the IDs from your own ledger. Superseding preserves the history, retires the
old entry, and makes the replacement the decision `check` consults.

## The core verbs

| Command | Purpose |
|---|---|
| `init` | Seed a ledger through a short interview. |
| `log` | Write an entry or add a relationship to an existing one. |
| `sniff` | Stage candidate decisions from a session transcript. |
| `distill` | Review staged captures interactively. |
| `brief` | Summarize blockers, focus, and recent decisions. |
| `why` | Show the reasoning behind an entry and its rejected alternatives. |
| `check` | Check a plan against settled decisions. |
| `map` | Group intents and decisions, with execution status from kazi when available. |
| `supersede` | Replace a settled entry while preserving its history. |
| `ui` | Serve a read-only ledger browser on localhost. |
| `import` | Measure existing ADRs and offer to import or index them. |
| `install-hooks` | Merge Claude Code hook registrations into settings. |
| `install-skill` | Install the Claude Code capture skill. |
| `reindex` | Rebuild the derived SQLite cache from entry files. |
| `version` | Print the binary version. |

This table describes the current source tree; an installed release may have fewer
commands. Run `dira --help` or `dira help COMMAND` for your binary's options.

## Bring existing ADRs

```sh
dira import docs/adr
```

Import scans the directory and reports how many documents contain a rejected
option with a reason. It asks before writing; `--yes` confirms noninteractively.
Imported decisions are staged for review, with source filenames and content hashes.

If no document contains a reasoned rejection, dira offers to index the documents
in its cache instead. It does not manufacture rejected alternatives to make an ADR
look enforceable.

## Your ledger is plain files

```text
.dira/
├── config.toml          # Ledger configuration
├── entries/             # Version-controlled Markdown with YAML frontmatter
│   ├── int-0001.md
│   └── dec-0001.md
└── cache/                # Derived SQLite cache; rebuild with dira reindex
```

The five entry kinds are **intent, decision, question, constraint, and note**.
Rejected alternatives belong to a decision, with a reason and an optional
`revisit_if` condition. Relationships connect entries into a reasoning chain.

You can read, diff, and review the files with ordinary Git tools. The cache is
rebuildable from those files. Browse [this repository's ledger](.dira/entries) for
real examples, or consult the [entry schema](schema/entry.schema.json).

Parent ledgers declared in `.dira/config.toml` let you organize personal, workspace,
and repository context. `dira brief --chain` surfaces parent context; namespaced
references connect entries across ledgers. See the [design document](docs/design.md)
for the tier model and privacy rules.

## Relationship to kazi

[kazi](https://github.com/kazi-org/kazi) checks whether a declared goal is complete.
dira preserves why that goal matters and which choices led to it. Both work
independently.

`dira map` groups active intents and accepted decisions and reads execution status
through kazi's public JSON interface. If kazi is unavailable, the ledger groups
still render and dira explains why execution status is missing. Execution status
is derived at read time, never stored in the ledger.

## Build and contribute

Use the Go version specified in [go.mod](go.mod):

```sh
git clone https://github.com/kazi-org/dira.git
cd dira
go build ./cmd/dira
./dira --help
```

A source build reports `dev`; release builds embed their version at link time.
Before contributing, install the repository's pre-commit gates and run the tests:

```sh
sh hooks/install.sh
go test ./...
```

The suite includes validation of this repository's ledger against the entry schema.
See the [design](docs/design.md), [roadmap](docs/roadmap.md), and
[work plan](docs/plan.md) for architecture and ongoing work. Report bugs or suggest
improvements through [GitHub Issues](https://github.com/kazi-org/dira/issues).

Capture, review, conflict checks, ADR import, and a read-only ledger browser are available in dira.
See [releases](https://github.com/kazi-org/dira/releases) for versioned changes.

## License

[Apache License 2.0](LICENSE). See [NOTICE](NOTICE) for attribution.
