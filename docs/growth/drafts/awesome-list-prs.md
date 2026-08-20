---
status: awaiting-maintainer-approval
posted: false
target: three Claude Code awesome-lists / hook showcases (named below)
kind: awesome-list-pr
owner: E8-L5
---

# Awesome-list and hook-showcase submissions — draft

**Do not submit yet.** `E0`'s release gate is clear: dira v0.1.1 is released,
`brew install kazi-org/tap/dira` is a real, working install command — a
submission today would no longer be a vaporware claim. What still gates
these is launch sequencing (`E8-L6`, not this lane): a submission is a
one-shot, hard-to-undo outward-facing action, so it waits for the maintainer's
launch call rather than firing the moment a gate clears.

Each target's submission rules were read from the live repository, not
assumed, per the lane prompt's instruction not to invent a channel framework.
Sources checked 2026-07-30.

---

## 1. `hesreallyhim/awesome-claude-code`

The flagship curated list — largest audience of the three.

**Submission method (verified):** *not* a pull request. The repo's
contribution flow is a GitHub issue-form template ("Click here to submit a
new resource"); the maintainer explicitly asks contributors not to open PRs
directly, and states that only their own bot/process files PRs against the
list itself. Filing an issue is the correct action here — the file name
`awesome-list-prs.md` is the lane's shorthand, not a literal claim that every
target here takes a PR.

**Required elements (verified):**
- One-line description, written as descriptive prose — not sales language,
  no emoji, no addressing the reader directly ("Don't you hate...").
- A human-authored recommendation (the issue filer must be a person vouching
  for it, even though the tool itself is built with coding agents).
- Licensing is auto-discovered by their bot from the GitHub repo — nothing to
  supply manually beyond having a real LICENSE file (dira already ships
  Apache-2.0, `LICENSE`).
- Best-effort review, no guaranteed response, selective curation — "creating
  an issue does not represent any sort of contract."

**Best-fit category:** *Memory & Context Persistence* — the closest existing
section to "decision memory for AI coding agents" (`.agents/product-marketing.md`
§1's category framing). Re-verify the category list at submission time; it
was 2026-07 living data, not a frozen schema.

**Draft one-line description** (prose, no emoji, no "you"-address, matching
their style):
> dira is a git-native ledger of the decisions behind a codebase — captured
> automatically by Claude Code hooks, surfaced at the start of every session,
> and able to block a plan that contradicts a decision already rejected.

**Who files:** must be a human (the maintainer or David) — the list's own
rule requires a human-authored recommendation, which an agent cannot satisfy
on David's behalf even in draft form. This is a second, list-specific reason
this can never be agent-submitted, on top of the lane's absolute #1.

---

## 2. `ianymu/awesome-claude-code-hooks`

A hook-specific showcase — narrower audience, higher intent (people already
building/evaluating hooks).

**Submission method (verified):** pull request, or file an issue with the
hook's details — either is accepted.

**Required elements (verified):**
1. Public repository link (MIT/Apache/CC0 preferred — dira is Apache-2.0).
2. One-sentence description of what bug or issue the hook catches.
3. The lifecycle event(s) it fires on (`Stop`, `PreToolUse`, `PostToolUse`,
   `Notification`, `SubagentStop`).
4. Dependencies and caveats, including network calls — the list explicitly
   asks contributors not to submit hooks that make **undisclosed** network
   calls, or that require a `--permission` flag change.

**Why dira's hooks are a clean fit for this list's rules specifically:** all
three of dira's hooks (`hooks/settings.example.json`) fail open (`|| true`),
run entirely offline against the local repo, and need no permission-flag
change to install. `dira brief` does not "catch a bug" in the sense this
list's format expects (that framing fits `Stop`/`PreCompact` better than
`SessionStart`) — the entry below is written for the two that do.

**Draft entry** (their exact format, adapted):
> **[dira sniff --stage](https://github.com/kazi-org/dira)** — "Stages
> decision-shaped language from the transcript so context survives a
> `PreCompact` or session end instead of evaporating into a compaction
> summary." Apache-2.0, Go. Offline, no network calls, fails open.
> _(Stop, PreCompact)_

**Note for the maintainer:** this list is small and narrowly scoped (hook
authors evaluating other hooks) — it is inner-ring by intent-match, not by
audience size. If `E8-L1`'s channel ranking disagrees, defer to that ranking;
this draft does not re-litigate channel selection, only prepares the copy.

---

## 3. `rdmgator12/awesome-claude-plugins`

A plugin/marketplace-specific list — the right home once the plugin bundle
itself (`.claude-plugin/plugin.json`) is real and installable.

**Submission method (verified):** pull request against the list, per its own
`CONTRIBUTING.md`.

**Required format (verified):**
```
[Plugin Name](url) - Short description. _Use case: specific application._ (Surface)
```
- **Surface tag** must be one of `(Claude Code)`, `(Cowork)`, or
  `(Claude Code, Cowork)` — dira is `(Claude Code)` only; no Cowork surface
  exists or is planned.
- Entries sort into ~28 categories; re-check the live category list at
  submission time rather than trusting this draft's guess (categories drift
  faster than a quarterly-verified list should be trusted).
- List explicitly states it is community-maintained and not affiliated with
  Anthropic — no claim of official status belongs in the entry copy.

**Draft entry:**
> [dira](https://github.com/kazi-org/dira) - A git-native decision ledger for
> Claude Code: captures the *why* behind a codebase automatically, surfaces
> it every session, and blocks plans that contradict a rejected decision.
> _Use case: stop re-explaining and re-litigating decisions your agent has
> already forgotten._ (Claude Code)

**Gate specific to this one:** a plugin-marketplace list is the one target
here where submitting *before* `.claude-plugin/plugin.json` is real and
publishable is not just premature but actively wrong — the entry links to a
plugin that cannot be installed. This is the strictest of the three gates.

---

## What was deliberately not included

**Product Hunt** and general startup-launch directories are out of scope for
this file by design — they belong to `directory-submissions.md`. See that
file for the current reasoning (launch sequencing, not a missing binary).

## Common failure mode across all three

Every one of these lists explicitly punishes self-promotional framing
("Don't you hate...", sales language, addressing the reader). The drafts
above are written in the flat, evidence-first voice `.agents/product-marketing.md`
§10 already mandates for dira's own copy — the two constraints turned out to
be the same constraint, which is worth noting rather than treating as a
coincidence.
