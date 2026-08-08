# dira — growth experiments

**Lane:** E8-L1 · **Instrument:** `/growth-experiments`' ICE backlog + pre-registered
spec shape. State lives here per that skill's convention. Checked mechanically by
[`scripts/check-growth-plan.mjs`](scripts/check-growth-plan.mjs).

**Status:** nothing here has run. `docs/growth/corpus.md` stays empty until the first
experiment reports a number. Every threshold below was set **before** its channel runs,
per absolute #3 — none of them were tuned to whatever the channel is likely to produce.

<!-- honest-limits:start -->
No experiment below measures or targets virality, an invite mechanic, or a k-factor.
Every threshold is a rate (a percentage or a ratio) with an n-minimum and a deadline —
never a raw count — because a raw count cannot distinguish a compounding channel from
noise, and this product has no viral fallback if a rate disappoints.
<!-- honest-limits:end -->

---

## ICE backlog

One row per channel from `docs/growth/channels.md`. `ICE = Impact × Confidence × Ease`,
each 1–10. Confidence is evidence-based, not vibes: a channel with a documented outside
precedent for dev tools scores higher than one with none. Re-score whenever a real
result lands. The three inner-ring channels below are the top three by ICE among the
channels that are actually runnable now (SEM/social/offline/email/affiliate/sales rows
are excluded from spend by Gate Zero and scored low on Ease for exactly that reason, not
because the idea is bad).

| id | idea (channel #) | stage | Impact | Confidence | Ease | ICE | status |
|---|---|---|---|---|---|---|---|
| B01 | Viral Marketing (1) | — | 1 | 1 | 1 | 1 | excluded (structural non-starter) |
| B02 | PR (2) | Acquisition | 5 | 3 | 4 | 60 | potential |
| B03 | Unconventional PR (3) | Acquisition | 4 | 2 | 3 | 24 | potential |
| B04 | SEM (4) | Acquisition | 5 | 3 | 2 | 30 | deferred (Gate Zero) |
| B05 | Social & Display Ads (5) | Acquisition | 4 | 3 | 2 | 24 | deferred (Gate Zero) |
| B06 | Offline Ads (6) | Acquisition | 2 | 2 | 1 | 4 | long-shot |
| B07 | SEO (7) | Acquisition | 9 | 4 | 5 | 180 | potential — promote at E6 ship |
| B08 | Content Marketing (8) | Acquisition | 6 | 5 | 7 | 210 | **inner — EXP-003** |
| B09 | Email Marketing (9) | Acquisition/Retention | 4 | 3 | 3 | 36 | long-shot (no list yet) |
| B10 | Engineering as Marketing (10) | Acquisition | 8 | 5 | 4 | 160 | potential — round 2 candidate |
| B11 | Target Market Blogs (11) | Acquisition | 5 | 4 | 5 | 100 | potential |
| B12 | Business Development (12) | Acquisition | 6 | 5 | 6 | 180 | potential — amplifies EXP-001 |
| B13 | Sales (13) | Acquisition | 3 | 2 | 3 | 18 | long-shot |
| B14 | Affiliate Programs (14) | Revenue | 2 | 2 | 2 | 8 | long-shot |
| B15 | Existing Platforms (15) | Acquisition | 8 | 6 | 8 | 384 | **inner — EXP-001** |
| B16 | Trade Shows (16) | Acquisition | 1 | 1 | 1 | 1 | long-shot |
| B17 | Offline Events (17) | Acquisition | 1 | 1 | 1 | 1 | long-shot |
| B18 | Speaking Engagements (18) | Acquisition | 3 | 2 | 2 | 12 | long-shot |
| B19 | Community Building (19) | Acquisition | 7 | 6 | 7 | 294 | **inner — EXP-002** |

Weekly loop per `/growth-experiments`: review last week's numbers → log to
`docs/growth/corpus.md` → re-score this table → launch the top ranked not-yet-run item.
Track tests-launched-per-week; two weeks at zero is the signal to surface, per the
skill's own rule.

---

## Pre-registered specs — one per inner-ring channel

### EXP-001: Claude Code plugin marketplace + awesome-list placement (channel #15, Existing Platforms)

- **Owner:** maintainer
- **Cadence:** weekly check-in against the threshold date
- **Runs from:** T0 (the day a stranger can install dira) — pre-registered now, not run
  yet. No absolute date; `T0` is defined in `docs/plan/lanes/E8.md`, never a calendar date.
- **Depends on:** E8-L5 (the drafted listing/PR content) and E0–E3 (a binary to point
  the listing at)
- **Gate-zero status:** pre-PMF learning experiment. Zero spend — packaging and
  submission only.
- **Hypothesis:** listing dira in the Claude Code plugin marketplace plus 2 relevant
  awesome-lists drives referred traffic that converts to real interest (a clone), not
  just impressions.
- **Terminal metric:** unique cloners ÷ unique visitors referred from marketplace and
  awesome-list sources (GitHub repo Insights → Traffic → Referring sites, see
  `channels.md`'s measurement note — no telemetry involved).
- **Threshold:** ≥8% clone-through rate, n≥100 referred visitors, within 21 days of the
  listing(s) going live.
- **Effort cap:** 1 day to draft the listing submissions (already scoped as approval-gated
  drafts under E8-L5); no ongoing spend.
- **On pass:** submit to 3 more marketplaces/lists (scale rule — same mechanism, more
  surfaces).
- **On fail:** one retry with revised listing copy, per the one-iteration rule. If the
  retry also fails, deprioritize and log the numbers in `docs/growth/corpus.md` — do not
  re-run a third time without new evidence.

### EXP-002: Show HN post on the `dira check` demo clip (channel #19, Community Building)

- **Owner:** maintainer
- **Cadence:** reply-debt zero for the first 48 hours, then a single weekly check for the
  remainder of the 7-day window
- **Runs from:** T0 + the demo clip existing (`E8-L4`, itself blocked on E0–E3). No
  absolute date.
- **Depends on:** the ≤20s `dira check` cast (E8-L4)
- **Gate-zero status:** pre-PMF learning experiment. Zero spend — one post, no ads, no
  solicited votes (etiquette tripwire per `/distribution`).
- **Hypothesis:** a single Show HN post built on the demo clip converts front-page or
  tail traffic into real engagement with the repo, not curiosity clicks that bounce.
- **Terminal metric:** unique visitors referred from `news.ycombinator.com` ÷ unique
  cloners among them (GitHub repo Insights → Traffic → Referring sites).
- **Threshold:** ≥5% clone-through rate, n≥150 referred visitors, within 7 days of the
  post.
- **Effort cap:** the post + reply-debt zero for 48h. No paid promotion — soliciting
  votes is a ban tripwire, not just off-strategy.
- **On pass:** repurpose the post into a build-in-public thread and submit to one further
  aligned community, respecting the 9:1 value-to-promotion rule (scale rule).
- **On fail:** HN posts are not repeatable — the same submission does not get reposted.
  One retry only applies if the post reached the front page but didn't convert (revise
  the pinned first comment for a fresh, distinct post later); if it never reached the
  front page at all, log the loss in `docs/growth/corpus.md` and do not retry the same
  angle.

### EXP-003: Build-in-public ship-notes (channel #8, Content Marketing)

- **Owner:** maintainer
- **Cadence:** weekly — one ship-note per notable commit or accepted decision
- **Runs from:** now. This is the one inner-ring experiment with no T0 dependency — it
  needs no binary, only the repo's existing commits and `.dira/entries/`.
- **Depends on:** nothing outside this repo
- **Gate-zero status:** pre-PMF learning experiment. Zero spend — writing time only.
- **Hypothesis:** repurposing real commits/decisions into short posts (frustration →
  discovery → the new way, the Epiphany ordering) at a steady weekly cadence produces
  readers who show genuine interest in the project, not one-off traffic.
- **Terminal metric:** unique visitors referred from each ship-note's link ÷ repo stars
  gained from that referrer cohort within 7 days of the post (GitHub repo Insights →
  Traffic → Referring sites, cross-referenced against star-history timestamps via
  `gh api repos/:owner/:repo/stargazers`).
- **Threshold:** ≥1.5% of unique referred visitors star the repo within 7 days of their
  referring post, n≥200 cumulative referred visitors across the first 4 posts, within 45
  days of the first post.
- **Effort cap:** 4 posts, ≤1 hour each — repurposed from material that already exists,
  not new research.
- **On pass:** make ship-notes a standing weekly habit; no separate re-approval needed
  (scale rule folds into ongoing cadence).
- **On fail:** one retry with a sharper pain-first hook per post (product-marketing §5).
  If still failing after the retry, log the learning that the artifact needs the demo
  clip live before it converts, and route back to the `E8-L4` dependency rather than
  iterating a third time on copy alone.

---

## Rule reminders (so a future edit doesn't quietly break the plan)

- A threshold with no denominator is not a threshold. "≥50 upvotes" is rejected on sight
  by `check-growth-plan.mjs` — see `docs/growth/fixtures/bad-raw-count/`.
- Every inner-ring channel row in `channels.md` must carry an owner and a cadence, or the
  checker fails naming the missing field — see
  `docs/growth/fixtures/bad-missing-owner-cadence/`.
- No banned-hype term appears outside a marked `<!-- honest-limits:start/end -->` block —
  see `docs/growth/fixtures/bad-hype-term/`.
