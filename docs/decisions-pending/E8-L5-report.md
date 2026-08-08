# E8-L5 report — Claude Code ecosystem placement, packaged as approval-gated drafts

**Lane:** `docs/plan/lanes/E8.md` §E8-L5. **Executed**, not just planned, per dispatch
(the L2 prompt at `docs/plan/prompts/L2-E8-L5.md` reads "planning only" — the team lead
explicitly overrode that and asked for the real artifacts; noted here so the discrepancy
between the prompt and what was built is visible, not silently resolved).

## What got written

| file | purpose |
|---|---|
| `.claude-plugin/plugin.json` | the plugin manifest — metadata + hook declarations, no `mcpServers` (dira has none) |
| `.claude-plugin/BUNDLE.md` | bundle layout, the E2 ownership boundary, the versioning contract |
| `docs/growth/DRAFT-CONTRACT.md` | the frontmatter contract, documented once, outside `drafts/` |
| `docs/growth/drafts/marketplace-listing.md` | marketplace copy |
| `docs/growth/drafts/awesome-list-prs.md` | 3 real, live-verified awesome-list/hook-showcase targets |
| `docs/growth/drafts/directory-submissions.md` | dev-tool directories, with explicit exclusions named |
| `docs/growth/scripts/check-drafts.mjs` | the zero-dependency, three-part checker |
| `docs/growth/scripts/check-drafts.selftest.mjs` | proves red-then-green against committed fixtures |
| `docs/growth/scripts/fixtures/{bad-posted-true,bad-denylist,hook-mismatch}/` | the three required bad fixtures |
| `docs/plan/tasks/E8-L5.md` | the 7-task L2 breakdown (bound ≤8), all marked done |

**Task count: 7.** Both the production checker and its self-test were run directly, not
asserted from reading the code:

```
$ node docs/growth/scripts/check-drafts.mjs
check-drafts: PASS

$ node docs/growth/scripts/check-drafts.selftest.mjs
red  ok: fixtures/bad-posted-true (posted: true) -- 1 issue(s), e.g. "posted must be boolean false, got true"
green ok: docs/growth/drafts/ (the real, committed drafts)
red  ok: fixtures/bad-denylist (gh pr create, gh issue create, praw, tweepy, social POST) -- 5 issue(s), e.g. "gh issue create invocation"
green ok: docs/growth/ + assets/demo/ (the real repo content, fixtures/ excluded)
red  ok: fixtures/hook-mismatch (Stop command renamed on the plugin.json side only) -- 1 issue(s), e.g. hook "Stop" command mismatch
green ok: hooks/settings.example.json vs .claude-plugin/plugin.json (the real files)

check-drafts.selftest: PASS (every red case failed, every green case passed)
```

## A guard that actually failed, then was fixed — not just claimed

The first version of `check-drafts.mjs` tried to keep its own source from tripping its
own deny-list scan by splitting trigger strings apart in code (`'gh' + ' pr create'`).
Running the self-test immediately caught this as insufficient: the header comment
*describing* the technique, and the self-test's own assertion labels, still spelled the
phrases out contiguously (`` `gh pr create` ``, `praw`, `tweepy`), and the checker
correctly flagged its own files. Fixed with a three-file, named, auditable exclusion
(`SELF_EXCLUDED_FILES`: `check-drafts.mjs`, `check-drafts.selftest.mjs`,
`DRAFT-CONTRACT.md` — the only three files that are *about* the deny-list rather than
outward content) instead. This is the "verify it fires red on a planted violation, then
green" instruction working as intended, not a formality — the first design was wrong
and the failure was real.

## Any `acc:` line found already green

None. `docs/growth/` had only two empty placeholder directories
(`docs/growth/fixtures/`, `docs/growth/scripts/`, created by a concurrent lane, left
untouched) and no files before this pass. `.claude-plugin/` did not exist. Everything
here started red and is green now because this lane built it.

## Where the `.claude-plugin/plugin.json` boundary against E2 falls

**This lane owns `.claude-plugin/plugin.json` and `BUNDLE.md`. E2 owns everything under
`skills/dira/`.** Nothing under `skills/` was created or touched by this lane — it does
not exist in the repo yet, and the plugin manifest needs no `skills` field to declare it
(Claude Code discovers `skills/dira/SKILL.md` by directory convention, same as kazi's
own plugin). If E2's lane also writes `.claude-plugin/plugin.json`, one of the two
silently wins; the lead should assign this file to E8-L5 explicitly, since the
hook-matching check needs exactly one `plugin.json` to check against and this lane
already owns the checker.

**The versioning collision is real and unresolved by this lane, on purpose.**
`plugin.json`'s `"version": "0.0.0-unreleased"` is a placeholder. There is no release
pipeline (`E0` unbuilt) to render and pin it the way kazi's `mix kazi.plugin` does. This
lane documents the contract (`BUNDLE.md`) but does not and cannot build the pinning step
— that belongs to `E0`, in this repo, plus an actual publish to a
`kazi-org/claude-plugins`-style marketplace repo, which is a **separate GitHub
repository** this checkout cannot touch (confirmed: no `.claude-plugin/` exists in the
sibling `kazi-org/kazi` checkout either — kazi's bundle is generated by
`mix kazi.plugin` and published by CI, never committed to the source repo). Scoping
that second-repo work into this lane would have been wrong; it is named, not silently
absorbed.

## Something in `.agents/product-marketing.md` worth flagging

Nothing in the doc is *wrong*, but §7's channel-1 framing ("kazi already ships as a
Claude Code plugin and has the distribution pipeline built; dira should ride the same
rails") reads, on a literal check of the sibling repo, as slightly more built-out than
it is: the "rails" are a rendering module (`lib/kazi/plugin/manifest.ex`) and an ADR
(ADR-0077), not yet a running, publishing `kazi-org/claude-plugins` marketplace with a
`kazi` entry live in it that dira could simply add a sibling entry beside — I did not
verify whether `kazi-org/claude-plugins` is populated, only that it exists as a distinct
repo from `kazi-org/kazi`. Worth a one-line correction in product-marketing (or leaving
as-is if the marketplace is in fact already populated) rather than treating "the rails
already exist" as fully checked.

## What was refused, and why

Nothing was refused. Every deliverable in the L2 prompt's "what this lane must
deliver" list was buildable without a binary, without touching `docs/roadmap.md` or
`docs/coverage.md`, without touching Go, and without creating anything in
`kazi-org/claude-plugins`. No task felt too large for one kazi run; no L3 was needed.

## What this lane does not claim

`check-drafts.mjs` does not gate on `E0` having shipped — that mechanical gate is
`E8-L6`'s `check-launch-readiness.mjs`, named explicitly in `directory-submissions.md`
so the two checks aren't confused for one another. This lane's checker enforces the
draft-approval contract and the deny-list; it does not (and per its own scope, should
not) re-implement the "is there a binary yet" check a sibling lane already owns.
