# E8-L1 report — channel selection + pre-registered thresholds

**Lane:** `docs/plan/lanes/E8.md` §E8-L1. **Executed**, not just planned, per dispatch.

## What got written

| file | purpose |
|---|---|
| `docs/growth/channels.md` | Bullseye pass — all 19 channels, exactly 3 inner-ring, reconciled explicitly against product-marketing §7 |
| `docs/growth/experiments.md` | ICE backlog (19 rows) + 3 pre-registered specs, one per inner-ring channel |
| `docs/growth/corpus.md` | schema + empty table, per spec — nothing has run |
| `docs/growth/scripts/check-growth-plan.mjs` | zero-dependency checker, two-sided |
| `docs/growth/fixtures/{bad-missing-owner-cadence,bad-raw-count,bad-hype-term}/` + `README.md` | the three required bad fixtures |
| `docs/plan/tasks/E8-L1.md` | the 5-task L2 breakdown (bound ≤8), all marked done |

**Task count: 5.** All five `acc:` lines are green, verified by running the checker
directly — not asserted from reading the code. Output:

```
$ node docs/growth/scripts/check-growth-plan.mjs
growth plan OK: 19 channels rated, 3 inner-ring, 3 pre-registered spec(s), 0 banned-hype terms outside honest-limits blocks.

$ node docs/growth/scripts/check-growth-plan.mjs docs/growth/fixtures/bad-missing-owner-cadence
FAIL: channels.md inner-ring row "Existing Platforms" is missing an owner

$ node docs/growth/scripts/check-growth-plan.mjs docs/growth/fixtures/bad-raw-count
FAIL: EXP-002's threshold is a raw count — missing a denominator (rate/percentage) and an n-minimum: "≥50 upvotes, within 7 days of the post."

$ node docs/growth/scripts/check-growth-plan.mjs docs/growth/fixtures/bad-hype-term
FAIL: banned-hype term "revolutionary" found in channels.md outside any <!-- honest-limits:start/end --> block
```

## Any `acc:` line found already green

None. `docs/growth/` did not exist before this pass — matches `docs/plan/lanes/E8.md`'s
own note that "no `acc:` line above is already green." Everything here started red and
is green now because this lane built it, not because it was already satisfied.

## The inner ring, and the one place I disagree with product-marketing §7

**Picked:** Existing Platforms (Claude Code plugin marketplace/awesome-lists), Community
Building (Show HN), Content Marketing (build-in-public ship-notes).

§7 ranks five channels: (1) Claude Code ecosystem, (2) kazi's audience, (3) Show
HN/X post, (4) public-ledger SEO, (5) OSS credibility. My Bullseye pass agrees with #1
and #3 and **disagrees on the third slot**: I put Content Marketing in the inner ring
instead of the SEO/ledger-renderer channel §7 favored. Reason, stated plainly: the SEO
channel's entire mechanism is rendered `.dira/` pages, which requires `dira render`
(E6) — infrastructure that does not exist. Spending one of three test slots on a channel
that literally cannot start yet wastes it. Content Marketing needs nothing but this
repo's own git history and can start today, pre-binary. I'm not asserting §7 is wrong
about SEO's long-run importance — `channels.md` explicitly keeps it in `potential` with
an instruction to promote it the moment E6 ships — only that it's the wrong pick for
*this wave's three test slots*, on a build-dependency argument §7 didn't have occasion
to weigh when it was written (positioning, not sequencing, was §7's job).

§7's items 2 (kazi's audience) and 5 (OSS credibility) are deliberately **not** run as
independent experiments — they're folded into Business Development and PR respectively,
both `potential`, because neither has a terminal metric separable from whichever channel
actually produces the traffic. Calling them separate inner-ring channels would have
double-counted the same visitors under two experiment IDs. I flagged this as a real
design tension in `channels.md` rather than quietly picking one reading.

**Nothing else in `.agents/product-marketing.md` struck me as wrong.** Positioning,
ICP, messaging hierarchy, and the honest-limits stance are solid and I built on them
without modification. The one disagreement above is about channel-selection mechanics,
which is this lane's job, not about anything §1–§6 or §8–§10 settled.

## Self-check on the risk the lane spec named

`E8.md`'s risk callout: thresholds get set to whatever's likely rather than what would
justify continuing. Honest read of my own numbers: 8% / 5% / 1.5% conversion rates with
n-minimums of 100–200 aren't gimmes — a typical HN post's click-through-to-meaningful-
action rate for a niche dev tool is often well under 5%, so EXP-002 failing is a real,
plausible outcome, not a formality. I did not tune any of the three to a number I was
confident would already clear.

## Things worth flagging that aren't defects in the plan itself

1. **T0 gating is real for two of three experiments.** EXP-001 and EXP-002 are
   pre-registered now but cannot *run* until a binary exists (E0–E3) and, for EXP-002,
   until the demo clip exists (E8-L4). Only EXP-003 (Content Marketing) is startable
   today. This is stated in the files themselves, not hidden.
2. **The measurement mechanism (GitHub repo Traffic Insights) has a real operational
   limitation**: GitHub only retains a 14-day trailing referrer window and surfaces the
   top 10 referrers. Whoever runs EXP-001/002/003 needs to snapshot Insights weekly
   (matches the cadence I set) to accumulate data past 14 days — worth knowing before
   the first experiment launches, not a spec defect but a real execution gotcha.
3. **`qst-0003` (ADR-import quality) does not block this lane.** None of the three
   inner-ring experiments claim retrospective import or day-1 value — that claim lives
   in E8-L4's secondary clip and E8-L6, both already gated on the open question by their
   own lanes. Confirmed this stayed out of scope here.

## What I refused to plan, and why

Nothing. All five tasks in `docs/plan/tasks/E8-L1.md` were both plannable and
executable without a Go binary, without touching the roadmap/coverage files, and
without any outward-facing action — so nothing hit the honesty-rule's "unresolved
prerequisite" clause.

## Boundaries respected

Did not touch `docs/roadmap.md`, `docs/coverage.md`, `go.mod`, `cmd/`, any `.go` file,
or `docs/design/scripts/contrast.mjs`. Committed nothing, pushed nothing, posted
nothing, sent nothing. Every file above is new; no existing file outside `docs/growth/`,
`docs/plan/tasks/`, and this report was modified.
