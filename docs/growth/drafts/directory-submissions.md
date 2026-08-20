---
status: awaiting-maintainer-approval
posted: false
target: dev-tool directories (dofollow, dev audience) — named below
kind: directory-submission
owner: E8-L5
---

# Directory submissions — draft

**Do not submit yet.** `E0`'s gate is cleared — dira v0.1.1 is released,
`brew install kazi-org/tap/dira` is a real, working install command. What
still blocks submission is launch sequencing (`E8-L6`'s job, not this
lane's): these are outward-facing posts, and posting them is a maintainer
action gated on the launch decision, not on the binary existing.

Selected using the `/distribution` skill's channel framework and the
`directory-submissions` reference tiers — no new channel taxonomy invented.
Filtered to directories with a genuine dev audience and a real dofollow (or
equivalent high-intent) payoff; general consumer/startup directories with no
dev-specific angle were cut.

## Explicitly excluded, named rather than silently dropped

- **Product Hunt.** Per `/distribution`: PH rewards a live demo video,
  screenshots, and warm-audience momentum in the first two hours. `E0` has
  now shipped, but a PH launch is still a real launch event that needs its
  own sequencing (`E8-L6`'s job, not this lane's) — not something to fire
  off incidentally while fixing stale copy elsewhere.
- **Show HN, Reddit, X/Bluesky.** These are `E8-L6`'s deliverables
  (`docs/growth/lanes/E8.md`), not this lane's — listing them here would
  duplicate ownership. Not omitted by oversight; out of scope by design.
- **Tier 4 agent/MCP registries** (Glama, LF MCP Registry, AI Agents List,
  etc.). **Not applicable.** dira has no MCP server (`dec-0008`: integration
  with kazi runs entirely through kazi's public `--json` surface and Claude
  Code hooks — dira never exposes its own MCP server). Listing here would
  misrepresent what the product is. If that scope ever changes, this section
  should be revisited, not silently backfilled.

## Directories worth the time — E0's binary gate is now clear

| Directory | DR (approx.) | Dofollow | Dev-audience fit | Gate |
|---|---|---|---|---|
| **DevHunt** | ~35 | Yes | Dev-tool-specific launch directory; best fit of the launch-style directories for a CLI. | Launch sequencing (`E8-L6`) |
| **SourceForge** | 92 | Yes | Legacy but still high DR; trivial company/project listing, no demo required. | Launch sequencing (`E8-L6`) |
| **Slashdot** | ~88 | Yes | Legacy high-DR, dev-heavy audience; company/project profile submission. | Launch sequencing (`E8-L6`) |
| **Stackshare** | ~60 | Yes | Dev-centric "what's in your stack" profile — fits a CLI tool better than a general SaaS directory. | Launch sequencing (`E8-L6`) |
| **GitHub topics** (not a directory, same spirit) | n/a | n/a | Free, immediate, zero submission process — just repo metadata. | **None** — needs no binary, only a public repo, which already exists. See note below. |

**AlternativeTo — flagged, not recommended by default.** AlternativeTo works
by submitting "X is an alternative to Y," which requires picking an
incumbent category to sit inside. `.agents/product-marketing.md` §1
*deliberately* rejects filing dira under "ADR tooling," "context management,"
or "project management" — the exact categories AlternativeTo would need dira
filed under to be discoverable there. Listing dira as an "alternative to
Obsidian" or "alternative to a `docs/decisions/` folder" directly contradicts
the new-category bet the positioning doc is making. **Recommendation: skip,
or revisit only if the new-category bet is later abandoned** — this is a real
tension, not a checklist item to complete regardless.

### GitHub topics (do today, needs no gate)

The repo is already public. Adding topic tags is metadata, not a listing, and
needs no binary — but it is still outward-facing (a public repo setting), so
it stays maintainer-actioned, not agent-actioned, same as everything else
here. Recommended tags: `claude-code`, `claude-code-plugin`, `decision-memory`,
`adr`, `ai-agent-tools`, `developer-tools`. These are the tags the awesome-lists
and GitHub's own topic-browse surface use to discover repos in this space —
verified against the categories the three awesome-lists in
`awesome-list-prs.md` actually use, not guessed.

## What "worth the time" excluded

General Tier 2 startup/SaaS directories (Manta, Hotfrog, F6S, Gust, and the
~40 similar entries in the broader directory catalog) were cut: they have no
dev-specific audience and their value for a CLI tool is marginal compared to
the five above. Submitting to all of them would be volume for its own sake,
which `.agents/product-marketing.md` §7 already disclaims ("compounding
organic distribution," not spray-and-pray).

## Gate, stated once more plainly

`E0`'s binary gate is clear — dira v0.1.1 is released and `brew install
kazi-org/tap/dira` works. What still gates every row above (except GitHub
topics) is launch sequencing, not the binary. `docs/growth/scripts/check-drafts.mjs`
does not enforce that gate mechanically (it checks the draft-approval
contract and the deny-list, not launch readiness) — that belongs to
`E8-L6`'s `check-launch-readiness.mjs`, which does not exist in this repo yet
(only planned in `docs/plan/tasks/E8-L6.md`). This file's gate is asserted in
prose here; there is no mechanical check to read it alongside until that
script is built.
