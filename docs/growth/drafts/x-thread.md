---
status: awaiting-maintainer-approval
posted: false
target: x.com
kind: social-post
owner: E8-L6
---

# X/Bluesky thread — dira

**Do not post this until `status` above is flipped by a human. This file is a draft
awaiting maintainer approval, not a send.**

Built on the demo clip, not thread-bait: the first post links the recorded cast's
social cut directly — `assets/demo/check.gif` (or `assets/demo/check.mp4`, whichever
format the cut ends up in, per `E8-L4-T6`) — and the thread is the clip plus the same
candid limitations from the Show HN draft, not a separate pitch.

## Thread

**1/**
Your coding agent has amnesia. You keep re-explaining decisions you already made — and
it keeps suggesting the thing you rejected in July.

I built dira so it doesn't. 20 seconds, no narration:

[clip: assets/demo/check.gif]

**2/**
That's `dira check` — before a plan gets built, it runs the plan past a git-native
ledger of past decisions and refuses if it contradicts one, quoting the original
reason. No model in the binary, no network call: the matching is lexical and runs
entirely in-process.

**3/**
The ledger itself is one markdown file per decision — YAML frontmatter, prose for the
*because*, committed to your repo. The coding agent does the writing, via Claude Code
hooks; you mostly just confirm.

**4/**
Honest limits, since I'd rather say them than have someone else find them:

- Capture quality depends on the agent driving it — the offline regex tier catches
  decision language, not the *because*.
- Point it at an existing ADR corpus and the yield varies a lot: I measured five real
  repos and it ranged from 90% of documents carrying a real rejected option down to
  zero. It measures and tells you before writing anything.
- No team tier yet — single-player today.

**5/**
Apache 2.0, `brew install kazi-org/tap/dira`:
https://github.com/kazi-org/dira

<!-- honest-limits:start -->
Not claiming this goes viral or that it's a growth hack — it's one clip and one honest
thread about a tool I use daily.
<!-- honest-limits:end -->
