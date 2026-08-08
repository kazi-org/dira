# dira — growth corpus (measured winners and losers)

**Lane:** E8-L1. Per `/growth-experiments`: keep every headline, hook, page, or post
that beat its pre-registered threshold, with its measured rate — and the notable
losers. Feed this file to future copy/experiment generation as few-shot context. Decay
it: date every entry, prefer recent winners, and cap size so the voice doesn't ossify.

**Status: empty.** No experiment in `docs/growth/experiments.md` has run yet — there is
no binary (`docs/plan/lanes/E8.md`, "the dependency nobody may paper over"), so nothing
here can be measured. This file exists now with its schema settled so the first result
has somewhere honest to land, rather than inventing a format under deadline pressure.

Do not add a row until an experiment has actually run and reported a real number against
its pre-registered threshold. A row with no measured rate is not a corpus entry, it's a
prediction — those belong in `docs/growth/experiments.md`, not here.

---

## Schema

One row per **measured** experiment outcome (win or loss), append-only, most recent
first.

| field | rule |
|---|---|
| `date` | ISO date the result was read, not the date the channel launched |
| `exp_id` | the `EXP-NNN` id from `docs/growth/experiments.md` |
| `channel` | channel name + `#` from `docs/growth/channels.md` |
| `artifact` | the exact headline / hook / page / post copy that ran, verbatim |
| `metric` | the terminal metric, as defined in the pre-registered spec |
| `measured_rate` | the actual rate observed, in the same units as the threshold |
| `threshold` | the pre-registered threshold, copied verbatim for side-by-side comparison |
| `result` | `pass` or `fail` — mechanical comparison of `measured_rate` to `threshold`, not judgment |
| `note` | one line: what to reuse (if pass) or what not to repeat (if fail) |

## Table

<!-- growth-plan:corpus-table:start -->
| date | exp_id | channel | artifact | metric | measured_rate | threshold | result | note |
|---|---|---|---|---|---|---|---|---|
<!-- growth-plan:corpus-table:end -->

*(no rows yet — see Status above)*
