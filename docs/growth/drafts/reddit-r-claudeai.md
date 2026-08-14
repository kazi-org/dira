---
status: awaiting-maintainer-approval
posted: false
target: reddit.com/r/ClaudeAI
kind: reddit-post
owner: E8-L6
---

# r/ClaudeAI post — dira

**Do not post this until `status` above is flipped by a human. This file is a draft
awaiting maintainer approval, not a send.**

One earned subreddit, per the lane's own instruction: "one honest post in one earned
subreddit beats cross-posting to five." r/ClaudeAI, because the ICP — people already
running Claude Code agentically — is definitionally already there (`channels.md` row
19 / `EXP-002`'s audience).

## Subreddit self-promo rule, as read at authoring time

**Attempted read: 2026-08-14.** This draft's author tried to read r/ClaudeAI's live
sidebar/rules page directly (`reddit.com/r/ClaudeAI/about/rules`) and could not —
`reddit.com` is unreachable from this drafting environment (fetch blocked at the
domain level; confirmed via two access paths, both refused). A web search for the
subreddit's specific self-promotion rule did not surface its current text either, only
Reddit's general site-wide guidance.

**What is confirmed, and what is not.** Reddit's site-wide default norm is the 90/10
rule — roughly 90% genuine community participation to 10% or less self-promotion —
and individual subreddits are free to set stricter rules than that default.
r/ClaudeAI's own current rule text is **not independently confirmed by this draft**.
The 9:1 ledger below follows the stricter, commonly-cited ratio as a safe default, not
because it's been read on the live sidebar.

**Before this post goes out, the maintainer must open `reddit.com/r/ClaudeAI/about/
rules` directly and confirm the actual current rule** — record what it says and the
date read, right here, replacing this paragraph. Do not post against an unread rule.

## 9:1 ledger

| # | type | description | date |
|---|---|---|---|
| 1 | contribution | [maintainer: link + one-line summary of a real answer given in r/ClaudeAI] | [maintainer: fill in] |
| 2 | contribution | [maintainer: link + one-line summary of a real answer given in r/ClaudeAI] | [maintainer: fill in] |
| 3 | contribution | [maintainer: link + one-line summary of a real answer given in r/ClaudeAI] | [maintainer: fill in] |
| 4 | contribution | [maintainer: link + one-line summary of a real answer given in r/ClaudeAI] | [maintainer: fill in] |
| 5 | contribution | [maintainer: link + one-line summary of a real answer given in r/ClaudeAI] | [maintainer: fill in] |
| 6 | contribution | [maintainer: link + one-line summary of a real answer given in r/ClaudeAI] | [maintainer: fill in] |
| 7 | contribution | [maintainer: link + one-line summary of a real answer given in r/ClaudeAI] | [maintainer: fill in] |
| 8 | contribution | [maintainer: link + one-line summary of a real answer given in r/ClaudeAI] | [maintainer: fill in] |
| 9 | contribution | [maintainer: link + one-line summary of a real answer given in r/ClaudeAI] | [maintainer: fill in] |
| 10 | promotional | This post — introducing dira, built on the `dira check` demo clip | T0+5 (per `docs/growth/launch.md`) |

Nine genuine prior contributions, then the one promotional post. Not fabricated by this
draft — an agent has no access to the maintainer's Reddit history and will not invent
plausible-looking links or dates. Each row is a template the maintainer fills with a
real comment/answer before this post is approved; `posted: false` means nothing ships
until that happens.

## The post itself

**Title:** I built a git-native decision ledger so my coding agent stops relitigating
decisions I already made — feedback welcome

Hey r/ClaudeAI — sharing something I built for my own Claude Code workflow, in case
it's useful to anyone else running agents on real codebases.

The problem: I'd reach real agreement with the agent on a decision, then a few weeks
later it would suggest the exact thing I'd already rejected, because it has no memory
across sessions and I wasn't going to re-read old notes to check.

dira is a git-native ledger — one markdown file per entry, YAML frontmatter plus prose
— for intents, decisions, rejected alternatives, open questions, and constraints.
Capture happens through Claude Code hooks (`SessionStart`/`Stop`/`PreCompact`), so the
agent does the writing and I mostly just confirm.

The part that actually changes my workflow is `dira check`: before a plan gets built, I
can run it past the ledger and it refuses if the plan contradicts something I already
decided, quoting the original reason:

```
$ dira check "add a background daemon to track run state"
✗ conflicts with dec-0060 (accepted 2026-07-03)
    rejected alternative: "a daemon"
    why_not: violates the single-binary intent (int-0002)
    revisit_if: cold-start latency stops being the binding constraint
→ supersede dec-0060, or revise the plan
```

**Honest limitations, since this sub calls those out fast:**

- Capture quality depends on the agent driving it (`dec-0003`) — the offline regex tier
  catches decision language but not the *because*; that needs a live session.
- If you point `dira import` at an existing ADR corpus, the yield genuinely varies —
  I measured five real repos and it ranged from 90% of documents carrying a real
  rejected-option down to zero (`dec-0028`). It measures your corpus and tells you the
  number before writing anything, rather than importing a pile of entries with nothing
  in them.
- No team tier yet (`dec-0007`) — it's a single-player tool today.

Repo: https://github.com/kazi-org/dira. Genuinely interested in whether this is useful
outside my own workflow, and in what breaks it.

<!-- honest-limits:start -->
This is not pitched as going viral or as a growth hack — it's one honest post about a
tool I use daily, in the one subreddit where the audience already overlaps with the
problem.
<!-- honest-limits:end -->
