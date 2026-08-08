# Fixtures for `check-growth-plan.mjs`

Each subdirectory is a full `channels.md` + `experiments.md` pair, copied from the real
plan with exactly one deliberate defect, so the checker's negative path is proven rather
than assumed. Run with:

```
node docs/growth/scripts/check-growth-plan.mjs docs/growth/fixtures/<name>
```

| fixture | defect | expected |
|---|---|---|
| `bad-missing-owner-cadence/` | the "Existing Platforms" inner-ring row's Owner and Cadence cells are blanked | exit 1, naming the row and the missing field |
| `bad-raw-count/` | EXP-002's threshold is rewritten as `≥50 upvotes, within 7 days of the post` — a raw count with no denominator and no n-minimum | exit 1, naming both missing parts |
| `bad-hype-term/` | a sentence of banned-hype/virality-claim language is inserted into `channels.md`'s status line, outside any `honest-limits` block | exit 1, naming the first matched term |

Each must exit non-zero. `node docs/growth/scripts/check-growth-plan.mjs` with no
argument (the real, committed plan) must exit 0.
