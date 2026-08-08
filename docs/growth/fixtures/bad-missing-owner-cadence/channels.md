# dira — channel plan (Bullseye pass)

**Lane:** E8-L1 · **Instrument:** `/distribution`'s Bullseye 19-channel selection.
Positioning, ICP, and the §7 channel priority list are settled in
[`.agents/product-marketing.md`](../../.agents/product-marketing.md) — this file builds
on that, it does not re-derive it. Checked mechanically by
[`scripts/check-growth-plan.mjs`](scripts/check-growth-plan.mjs).

**Status:** every row below is a plan, not a result. `docs/growth/corpus.md` is empty
because nothing has run yet. **Nothing in this file has been posted, sent, or
submitted — every outward artifact any of these rows implies is a draft under
`docs/growth/drafts/` (E8-L5) awaiting the maintainer.**

---

## Gate zero — this is pre-PMF

dira has no binary, no users, no retention curve. Per `/growth-experiments`' gate zero,
acquisition spend is not warranted yet. Every row in this file is one of two things,
named explicitly so nobody mistakes one for the other:

- **A pre-PMF learning experiment** — cheap, organic, zero spend, testing whether the
  hook and the artifact resonate at all. All three inner-ring channels are this.
- **Deferred until PMF / until T0** — paid channels (SEM, social/display ads) and
  anything requiring an installed base (email marketing) are explicitly **not**
  scheduled. Rating them long-shot here is not an oversight; it is Gate Zero applied.

No channel below schedules paid acquisition against an unretained product.

<!-- honest-limits:start -->
**The honest limit (verbatim from `dec-0010` / product-marketing §7).** This buys
compounding organic distribution, not virality. There is no invite mechanic, no social
graph, and no k-factor in a single-player developer tool. Published artifacts inside a
fast-growing ecosystem compound; they do not go viral. Every threshold in
`docs/growth/experiments.md` is a rate with an n-minimum and a deadline for exactly this
reason — a raw count cannot distinguish compounding from noise, and virality is not on
the menu to fall back on when a rate disappoints.
<!-- honest-limits:end -->

---

## The 19 channels

Traction's canonical 19 (Weinberg/Mares), fit ratings carried over verbatim from
`/distribution`'s table. `Ring` is exactly one of `inner` / `potential` / `long-shot`.
Exactly 3 rows are `inner`. `Owner` and `Cadence` are required for inner-ring rows —
they are the rows actually running; potential/long-shot are backlog, not commitments.

<!-- growth-plan:channels-table:start -->
| # | Channel | Fit | Ring | dira-specific idea | Owner | Cadence |
|---|---|---|---|---|---|---|
| 1 | Viral Marketing | ◐ | long-shot | Structural non-starter, not a deprioritized idea — dira is single-player, per absolute #2 (see the honest limit above). Listed for completeness, not as a backlog item. | — | — |
| 2 | Public Relations (PR) | ◐ | potential | Pitch agentic-coding newsletters/press using dira's own dogfooded `.dira/` ledger as the proof artifact (product-marketing §7 item 5, "OSS credibility") | maintainer | ad hoc, after EXP-001/002 have data to pitch with |
| 3 | Unconventional PR | ◐ | long-shot | No concrete idea clears the bar yet; revisit case-by-case if one surfaces | — | — |
| 4 | Search Engine Marketing (SEM) | ◐ | long-shot | Deferred by Gate Zero — no converting page exists yet and no ad budget is being committed pre-PMF | — | — |
| 5 | Social & Display Ads | ◐ | long-shot | Same Gate Zero deferral as SEM; also poor fit for a single-player dev tool with no visual hook beyond the demo clip | — | — |
| 6 | Offline Ads | ○ | long-shot | Poor solo-founder ROI per `/distribution`'s own rating; not pursued | — | — |
| 7 | SEO | ⬤ | potential | Rendered `.dira/` ledgers as long-tail "why did project X choose Y" search pages — the mechanism `dec-0010` names as the growth engine | maintainer | promote to inner the moment E6 ships `dira render` |
| 8 | Content Marketing | ⬤ | **inner** | Build-in-public ship-notes: repurpose real commits/decisions from this repo into short posts (frustration → discovery → new way). Needs no unshipped infrastructure and no binary — startable today. | maintainer | weekly, one ship-note per notable commit/decision |
| 9 | Email Marketing | ⬤ | long-shot | No install base to email yet; revisit once an opted-in list exists | — | — |
| 10 | Engineering as Marketing | ⬤ | potential | A free "paste an ADR, see what dira would flag" grader tool — David's strongest unfair-advantage channel, but it is itself a build task larger than a channel test. Deliberately not inner this pass: Bullseye's focus rule says pour effort into what already works, not build a second tool before the first one has data. Best candidate for round 2. | maintainer | revisit after EXP-001/002/003 report |
| 11 | Target Market Blogs | ◐ | potential | Pitch niche newsletters covering Claude Code / agentic tooling once the demo clip exists | maintainer | ad hoc |
| 12 | Business Development | ◐ | potential | Cross-link with kazi's docs/audience — shared tap, joint story, a mention once dira ships. Amplifies whichever channel already ships; its own terminal metric would double-count Existing Platforms' referred traffic, so it is not run as an independent experiment. | maintainer | at T0, alongside EXP-001 |
| 13 | Sales (cold outreach + calls-based) | ◐ | long-shot | ICP self-serves via docs/demo; cold outreach to indie devs reads as spam and is a disqualifying move against this ICP, not just a low-yield one | — | — |
| 14 | Affiliate Programs | ◐ | long-shot | No revenue model exists to fund a commission — the core is free forever (`dec-0007`). Revisit only if/when the team tier ships. | — | — |
| 15 | Existing Platforms | ⬤ | **inner** | Claude Code plugin marketplace listing + relevant awesome-list placement, riding kazi's existing `kazi-org/claude-plugins` rails (product-marketing §7 item 1) | — | — |
| 16 | Trade Shows | ○ | long-shot | Poor fit, not pursued | — | — |
| 17 | Offline Events | ○ | long-shot | Poor fit, not pursued | — | — |
| 18 | Speaking Engagements | ○ | long-shot | Real-time performance; only if the maintainer opts in, which has not happened | — | — |
| 19 | Community Building (async/written) | ◐ | **inner** | One Show HN post built on the `dira check` demo clip (product-marketing §7 item 3) | maintainer | reply-debt zero for 48h post-launch, then weekly for the 7-day window |
<!-- growth-plan:channels-table:end -->

---

## Reconciliation with product-marketing §7

§7 ranks five channels in priority order: (1) Claude Code ecosystem adjacency, (2)
kazi's existing audience, (3) Show HN/X post, (4) public ledgers as long-tail SEO, (5)
OSS credibility. This Bullseye pass **agrees with #1 and #3** (rows 15 and 19 above) and
**disagrees with §7 on the third inner-ring slot.**

**Where this pass disagrees, and why:**

§7 ranked "public ledgers as long-tail SEO" (row 7, SEO) ahead of build-in-public
content and didn't rank content marketing at all. This pass puts **Content Marketing
(row 8) in the inner ring instead of SEO**, for one concrete reason: the SEO channel's
mechanism — rendered `.dira/` pages — depends on `dira render` (E6), which does not
exist yet. Spending one of only three inner-ring test slots on a channel that cannot
start until another epic ships wastes the slot. Content Marketing needs nothing that
isn't already sitting in this repo's own commit and decision history, and can start
this week. ICE scoring bears this out without being tuned to fit the conclusion: Content
Marketing scores 210 (Impact 6 · Confidence 5 · Ease 7) against SEO's 180 (Impact 9 ·
Confidence 4 · Ease 5) — SEO's impact ceiling is genuinely higher (it is the
architecturally-free loop `dec-0010` describes), but its Confidence and Ease are both
lower today because the infrastructure isn't there. **SEO is demoted to `potential`,
not dropped** — the row says explicitly to promote it to inner the moment E6 ships.

§7's #2 (kazi's existing audience) and #5 (OSS credibility) are **not** run as
independent inner-ring experiments. Both are folded into rows that already exist —
row 12 (Business Development) and row 2 (PR) respectively — because neither has a
terminal metric separable from the channel that actually produces the traffic (kazi's
audience mostly finds dira via the same marketplace listing that is row 15; OSS
credibility is a proof-point cited inside a PR pitch, not an acquisition mechanism on
its own). Calling them separate inner-ring channels would double-count the same
visitors under two experiment IDs.

**Everything else in §7 is preserved as stated:** the priority order for rows 15, 19,
and (once E6 ships) 7 matches §7 exactly; nothing here reopens the positioning,
messaging, or voice decisions — only the channel-selection mechanics, which is what
this lane owns.

---

## Measurement note (why these metrics don't require telemetry)

`cst-0004` forbids the CLI phoning home; telemetry is opt-in and absent by default. None
of the terminal metrics in `docs/growth/experiments.md` need it — they all read GitHub's
own repo-owner-facing analytics (Insights → Traffic: unique visitors, clones, referring
sites) for **this repo**, which is standard GitHub functionality available to any repo
owner and carries no data from a user's installed copy of dira. If GitHub's traffic
insights ever became unavailable, the experiments would need a different measurement
plan — but they would not need to add telemetry to the binary to get one.
