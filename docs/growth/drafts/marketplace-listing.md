---
status: awaiting-maintainer-approval
posted: false
target: a Claude Code plugin marketplace (e.g. a future kazi-org/claude-plugins-style repo)
kind: marketplace-listing
owner: E8-L5
---

# Marketplace listing draft — dira

**Superseded by the real listing.** `dira` released v0.1.1 (2026-08-18,
`brew install kazi-org/tap/dira` live) and the plugin is already published in
`kazi-org/claude-plugins` (second entry beside kazi). This draft's job — copy
ready for the day E0/E2 shipped — is done; kept as historical reference and
as the starting point if the listing ever needs a rewrite, not as a pending
submission. `posted` stays `false` here because this file itself was never
what got submitted — the marketplace repo's own manifest was — and a
maintainer, not this draft, owns that repo's copy going forward.

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
> **Status: released.** `brew install kazi-org/tap/dira` installs the binary
> this plugin depends on.

**Category:** Memory & Context Persistence (matching the category
`hesreallyhim/awesome-claude-code` uses for this kind of tool — see
`awesome-list-prs.md` for why that's the closest existing fit).

**Keywords:** `dira`, `decision-memory`, `adr`, `hooks`, `context`, `agent`

**Author:** kazi-org · **License:** Apache-2.0 · **Repository:**
`https://github.com/kazi-org/dira`

**Install line** (matches the real, published entry):
```
/plugin marketplace add kazi-org/claude-plugins
/plugin install dira@kazi
```
This mirrors kazi's own install line (`kazi/README.md` "Install via the Claude
Code plugin"). `kazi-org/claude-plugins` now carries a `dira` entry
(`.claude-plugin/marketplace.json` in that repo) — this file's copy is no
longer aspirational, it describes what is live.

## Banned-lexicon check (manual, until this listing is wired into a shared
coherence gate)

No use of "revolutionary," "seamless," "supercharge," "10x," or "AI-powered"
above (`.agents/product-marketing.md` §10). No virality or growth-rate claim.
No claim that the hosted renderer is required (`cst-0004`) — none is
mentioned because none exists yet to misrepresent.

## What was true before this went postable — all now satisfied

1. ~~`E0` ships a binary a stranger can install.~~ Done — v0.1.1, released
   2026-08-18.
2. ~~`E2` ships the skill referenced above.~~ Done — `skills/dira/SKILL.md`
   ships in this repo and in the plugin bundle.
3. ~~The release pipeline can render and pin `.claude-plugin/plugin.json`'s
   `version` to the release tag.~~ Not built in this checkout's pipeline
   (`.claude-plugin/BUNDLE.md`) — the marketplace repo's own manifest carries
   its own version instead, hand-maintained there.
4. A maintainer submitted the real listing to `kazi-org/claude-plugins`
   directly — this draft was reference copy, not the thing that got posted,
   which is why `posted` here stays `false` rather than being flipped to
   describe a submission this file never made.
