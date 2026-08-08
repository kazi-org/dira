---
status: awaiting-maintainer-approval
posted: false
target: a Claude Code plugin marketplace (e.g. a future kazi-org/claude-plugins-style repo)
kind: marketplace-listing
owner: E8-L5
---

# Marketplace listing draft — dira

**Do not submit.** No `dira` binary exists (`E0` unbuilt), so this plugin has
nothing to install yet. This draft exists so the copy is ready the day E0 and
E2 ship and the release pipeline can pin a real version into
`.claude-plugin/plugin.json` — see `.claude-plugin/BUNDLE.md` for the
versioning contract this listing depends on.

## Marketplace entry fields

**Name:** `dira`

**Tagline** (≤10 words):
> Never explain the same decision twice.

**Short description** (≤60 chars):
> A memory for why, in your Claude Code sessions.

**Long description** (~150 words, lead with dira, not kazi):

> Your coding agent has amnesia. You keep re-explaining decisions you already
> made — and it keeps suggesting the thing you rejected in July. dira is a
> git-native ledger of the intents, decisions, rejected alternatives, and open
> questions behind your codebase, captured automatically as you work with
> Claude Code. It surfaces that context at the start of every session, so you
> never re-explain a decision you already made — and it blocks plans that
> contradict decisions you already rejected, quoting your own past reasoning
> back at you.
>
> This plugin installs the dira skill, the `SessionStart`/`Stop`/`PreCompact`
> hooks that keep the ledger current at zero effort, and nothing else — no
> account, no hosted service, no network call required to function.
>
> **Status: pre-release.** There is no installable `dira` binary yet. This
> listing will go live once one exists.

**Category:** Memory & Context Persistence (matching the category
`hesreallyhim/awesome-claude-code` uses for this kind of tool — see
`awesome-list-prs.md` for why that's the closest existing fit).

**Keywords:** `dira`, `decision-memory`, `adr`, `hooks`, `context`, `agent`

**Author:** kazi-org · **License:** Apache-2.0 · **Repository:**
`https://github.com/kazi-org/dira`

**Install line** (do not publish until true):
```
/plugin marketplace add kazi-org/claude-plugins
/plugin install dira@kazi
```
This mirrors kazi's own install line (`kazi/README.md` "Install via the Claude
Code plugin"). It is aspirational copy — `kazi-org/claude-plugins` does not
yet carry a `dira` entry, and creating that entry is out of this lane's scope
(a separate repo plus a release-pipeline hook in `E0`, not a file this
checkout can produce).

## Banned-lexicon check (manual, until this listing is wired into a shared
coherence gate)

No use of "revolutionary," "seamless," "supercharge," "10x," or "AI-powered"
above (`.agents/product-marketing.md` §10). No virality or growth-rate claim.
No claim that the hosted renderer is required (`cst-0004`) — none is
mentioned because none exists yet to misrepresent.

## What must be true before this is postable

1. `E0` ships a binary a stranger can install.
2. `E2` ships the skill referenced above.
3. The release pipeline (`E0`'s scope) can render and pin
   `.claude-plugin/plugin.json`'s `version` to the release tag, per
   `.claude-plugin/BUNDLE.md`.
4. A maintainer reviews this copy, flips `posted` here to reflect reality
   *after* actually submitting it by hand, and edits `status` accordingly.
