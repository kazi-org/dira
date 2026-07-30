# The frontmatter contract for outward-facing drafts

Every file under `docs/growth/drafts/` is copy an agent wrote for a human to
send — never copy an agent sends itself. This file is the one place that
contract is documented, so every lane's drafts (this lane's marketplace
listing and awesome-list/directory copy; `E8-L6`'s Show HN post, Reddit post,
X/Bluesky thread; anything future) validate under the same checker:
`node docs/growth/scripts/check-drafts.mjs`.

This file itself is **not** a draft — it carries no frontmatter and is not
scanned for the contract below, because it lives outside `docs/growth/drafts/`
on purpose. Nothing about "documented in one place" should tempt a future lane
into putting contract docs inside the directory the checker treats as
100% outward copy.

## Required frontmatter keys

Every `*.md` file under `docs/growth/drafts/` must open with a YAML-ish
frontmatter block (`---` delimited) carrying at minimum:

```yaml
---
status: awaiting-maintainer-approval
posted: false
target: <the platform/list/directory this copy is destined for>
kind: marketplace-listing | awesome-list-pr | directory-submission | show-hn | reddit-post | social-post
owner: <lane id, e.g. E8-L5>
---
```

- **`status`** must be the literal string `awaiting-maintainer-approval`. No
  other value is accepted — there is no "approved" or "ready" state an agent
  can set. Only a human flips this, by editing the file (and at that point
  the maintainer is doing the sending, not the checker doing the approving).
- **`posted`** must be the literal boolean `false`. `true`, `"false"` (a
  string), or a missing key all fail the check. This is the single field the
  mechanical guard exists to protect: **no agent, ever, flips this to `true`.**
  A human posts the content by hand and may then update the record for their
  own tracking — the checker does not police that after-the-fact edit, only
  what an agent could have committed.
- **`target`** and **`kind`** are required so a reviewer (human or the next
  agent) can tell what a draft is for without opening it. `owner` names the
  lane that authored it, for traceability.

## What the checker actually enforces

`node docs/growth/scripts/check-drafts.mjs` (`docs/growth/scripts/check-drafts.mjs`) does two independent things:

1. **Every file under `docs/growth/drafts/`** parses with valid frontmatter
   and satisfies `status`/`posted` above. A fixture draft with `posted: true`
   is the committed red case (`docs/growth/scripts/fixtures/bad-posted-true/`).
2. **No file under `docs/growth/` or `assets/demo/`** (excluding the checker's
   own fixtures, see the script's header comment for the narrow, named
   exclusion) contains an automated-post invocation from a committed deny-list:
   an HTTP POST to a social/forum API host, `praw`, `tweepy`, `gh pr create`,
   `gh issue create`. The committed red case is
   `docs/growth/scripts/fixtures/bad-denylist/`.
3. **`.claude-plugin/plugin.json` parses**, and its declared hook commands
   match `hooks/settings.example.json` name-for-name — same event names, same
   command strings. The committed red case is
   `docs/growth/scripts/fixtures/hook-mismatch/`.

All three are exercised end to end (red, then green) by
`node docs/growth/scripts/check-drafts.selftest.mjs`.

## Why this is stricter than "the drafts look right"

A checker that only validates good input is decoration, not a guard — it has
never been observed to fail, so a regression that silently weakens the rule
(say, a future lane accepting `posted: "false"` as a string) would ship
unnoticed. Every rule above has a committed fixture proving the checker
rejects the bad case, not just accepts the good one.
