# dira — launch plan

**Lane:** E8-L6 · phase sequence per `/distribution`'s playbook, adapted to dira's own
channels rather than a generic taxonomy — every channel named below id-matches a row in
[`docs/growth/channels.md`](channels.md) and a threshold in
[`docs/growth/experiments.md`](experiments.md); this file does not re-derive or re-rank
either. Precedent: kazi's own `docs/marketing/launch-plan.md`, adapted rather than
copied.

## T0, defined once

**T0 = the day a stranger can install `dira` and have it work** (the same definition
`docs/plan/lanes/E8.md` uses). Every step below is an offset from that day —
`T0`, `T0-N`, `T0+N` — **never** an absolute calendar date. `T0` is not fixed yet: it is
set by E0–E3 shipping, not by this document, and the checker
(`docs/growth/scripts/check-launch-readiness.mjs`) fails on the first absolute date it
finds, so an editor cannot quietly slip a real date back in once one is known. Every
step line below carries its own owner and offset, on the same line, so the checker can
verify each one mechanically rather than trust the prose around it.

<!-- honest-limits:start -->
**The honest limit, carried from `dec-0010`.** This plan buys compounding organic
distribution, not virality. There is no invite mechanic, no social graph, and no
k-factor in a single-player developer tool — nothing below assumes otherwise, and no
step's own success criterion is a raw share count.
<!-- honest-limits:end -->

**No step below assumes a hosted renderer is required to launch (`cst-0004`).** The
renderer (`dira render`, per `dec-0012`) is additive; every step here works with only
the CLI, the repo, and channels the maintainer already controls.

---

## Phase 0 — polish the conversion surface (T0-14 to T0)

The landing page (`E8-L2`) and demo clip (`E8-L4`) are gated by their own lanes; this
phase is the packaging around them, not a re-listing of their work.

- [ ] Confirm the landing page build matches `main` — @maintainer, T0-7
- [ ] Set the GitHub social-preview image to the demo clip's first frame — @maintainer, T0-5
- [ ] Set GitHub repo topics (`developer-tools`, `claude-code`, `cli`) — @maintainer, T0-5
- [ ] Confirm README's status line matches `dira --help`'s real verb list — @maintainer, T0-3
- [ ] Dry-run every landing-page and README link against `main` — @maintainer, T0-1

## Phase 1 — warm-audience presence (T0-3 to T0)

Per product-marketing §7 item 2: kazi's existing audience is the exact ICP, already
warm, already trusting the author. This is cross-linked docs, not an independent
inner-ring experiment — `channels.md`'s own reconciliation note folds this into row 12
(Business Development) rather than scoring it separately, so this phase does not open a
new channel row.

- [ ] Add a dira mention to kazi's README, linking back here — @maintainer, T0-3
- [ ] Confirm dira's README credits kazi as sibling, not dependency — @maintainer, T0-2
- [ ] Log the cross-link go-live to `docs/growth/corpus.md` — @maintainer, T0

## Phase 2 — the one sharp artifact (T0 to T0+7)

`EXP-002` (Community Building, channel #19): one Show HN post built on the `dira check`
demo clip. Not a launch campaign — a single sharp artifact, per the lane's own
instruction that one honest post beats a cross-posted campaign.

- [ ] Post `show-hn.md` to Show HN once the maintainer flips its `status` — @maintainer, T0
- [ ] Hold reply-debt at zero for 48h per `EXP-002`'s registered cadence — @maintainer, T0+2
- [ ] Log `EXP-002`'s referred/clone numbers to `docs/growth/corpus.md` — @maintainer, T0+7

## Phase 3 — the cascade (T0+2 to T0+10)

`EXP-001` (Existing Platforms, channel #15) and the directory submissions from `E8-L5`
— placement that compounds once the sharp artifact has run, not before it.

- [ ] Submit the plugin-marketplace listing once approved — @maintainer, T0+2
- [ ] Open the awesome-list submissions once approved — @maintainer, T0+3
- [ ] Submit the directory listings once approved — @maintainer, T0+3
- [ ] Post `reddit-r-claudeai.md` to r/ClaudeAI once approved — @maintainer, T0+5
- [ ] Post `x-thread.md` once approved — @maintainer, T0+5
- [ ] Log each submission's live-or-rejected status to corpus.md — @maintainer, T0+10

## Phase 4 — week+1 measurement (T0+7 to T0+21)

Every threshold below is a rate with an n-minimum and a deadline, pre-registered in
`docs/growth/experiments.md` — this phase reads those thresholds, it does not restate
or re-derive them.

- [ ] Check `EXP-002` against its threshold (≥5%, n≥150, 7 days) — @maintainer, T0+7
- [ ] Check `EXP-001` against its threshold (≥8%, n≥100, 21 days) — @maintainer, T0+21
- [ ] Re-score the ICE backlog against numbers actually observed — @maintainer, T0+21
- [ ] Write both experiments' pass/fail disposition to corpus.md — @maintainer, T0+21

---

## What this file does not own

Channel selection and thresholds (`docs/growth/channels.md`,
`docs/growth/experiments.md`, `E8-L1`) — every channel named above id-matches a row
there. The draft copy itself (`E8-L5`, `E8-L6-T2`/`T3`/`T4`) — this file schedules
sending it, it does not contain it. Whether the day-1 import story is safe to lead with
is `dec-0028`'s call, stated in `docs/growth/drafts/show-hn.md`'s known-limitations
section, not repeated here.
