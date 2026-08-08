---
status: awaiting-maintainer-approval
posted: false
target_repo: kazi-org/kazi
kind: enhancement
duplicate_check: >
  Run 2026-07-30. `gh issue list --repo kazi-org/kazi --state all --search` for
  "portfolio schema", "CLI.Schema OR json contract", "dira". No duplicate. Nearest
  related is #1427 (CLOSED) — the issue that built `portfolio` itself; this completes
  that work rather than duplicating it. The closed JSON-contract issues (#288, #291,
  #344, #395) all concern `run`/`apply`, not `portfolio`.
---

# Title

`portfolio --json` emits a versioned result but is absent from `Kazi.CLI.Schema`

# Body

`kazi portfolio --json` produces a stable, versioned JSON object, but it is the only
`--json`-emitting command that is not registered in the schema module. That makes the
shape real but undiscoverable, and unpinned by the lockstep-version rule that governs
every other documented result.

## What exists today

`portfolio_json/1` (`lib/kazi/cli.ex:4249`, flag parsed at `cli.ex:1254`) emits:

```
schema_version, kind: "portfolio", planned, by_repo, fleet_remote,
totals { base, empty, rows: [{bucket, count, pct}] },
todo, blocked [{…, cause, blocker}], rate
```

It stamps `@run_schema_version`, so it is already versioned in practice.

But `Kazi.CLI.Schema`'s registry (`lib/kazi/cli/schema.ex`) documents only `apply`,
`plan`, `bus`, and `status`. `schema.ex:443` explicitly describes any command "with no
documented `--json` result" as outside the contract — which currently includes
`portfolio`, despite it emitting one.

## Why this is worth fixing on kazi's own terms

This is not primarily a request from a consumer. An emitted-but-undocumented `--json`
shape is a gap in kazi's public interface whether or not anything reads it:

- The lockstep rule at `schema.ex:24` — "a breaking change to any `--json` result bumps
  both" — cannot protect a result the registry does not know about. `portfolio`'s shape
  can change without anything noticing.
- `--json` exists so harnesses can drive kazi (ADR-0023, #288). `portfolio` answers
  "where are we", which is exactly the question a harness most wants, and it is the one
  result a harness cannot rely on.
- Nothing in the test suite pins it as a public contract today.

## Two things a consumer discovered, offered as evidence the shape needs documenting

Both were found by reading the source rather than the docs, which is the point:

1. **There are two bucket enums and the difference is easy to miss.**
   `portfolio.ex:38` defines `@type bucket :: :in_progress | :stuck | :complete`, and
   `by_repo` is typed `%{String.t() => %{bucket() => [map()]}}`. The five-value
   `five_bucket` taxonomy governs only `totals.rows[]` and the top-level `todo`/`blocked`
   arrays. A reader who finds `@bucket_order` first will assume one enum and be wrong
   about `by_repo` and `fleet_remote[].bucket`.

2. **`done` and `running` are never emitted as arrays.** They survive only as counts
   inside `totals.rows[]`. So `portfolio --json` can report how many goals converged but
   not which — per-goal status has to come from `status <ref> --json`. That is a
   reasonable design; it is simply not discoverable without reading `portfolio_json/1`.

## Proposed contract

Add a `"portfolio"` entry to `Kazi.CLI.Schema`'s registry describing the fields above,
with both enums named explicitly and a note that per-goal run state comes from
`status <ref> --json` rather than from `portfolio`. No behaviour change; the emitted
object is already correct.

## Scope

Additive, non-breaking, documentation of an existing shape.
