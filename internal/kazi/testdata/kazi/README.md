# testdata/kazi — the fixture corpus E4-L1 pins against

Per the founder decision recorded in dec-0008/qst-0005 (2026-08-14): dira does not
shell a live `kazi` in any test. Every fixture below is a byte-for-byte recording
(or a clearly marked hand-extension of one), and `internal/kazi` decodes against
these files and only these files. Re-recording is legitimate — a kazi upgrade, a
new bucket value — but must preserve every load-bearing case this README names,
or the task that needed it (cited per file) goes back to red.

## Natural recordings

- **`portfolio-populated.json`** — `kazi portfolio --json` on this machine
  (kazi 1.275.0, `/usr/local/bin/kazi`), captured 2026-08-14, unmodified except
  for pretty-printing (`json.dump(..., indent=2, sort_keys=True)`) for a legible
  diff — no field added, removed or reordered in value. `schema_version: 2`.
  Carries the **multi-run case E4-L3 needs**: `by_repo` holds `warnings-clean`
  four times, once per worktree-shaped key
  (`kazi-worktrees/p-warnings-clean-9479baf7-*`), with four distinct `run_id`s
  and statuses `over_budget` / `stuck` / `stuck` / `stuck` — verified present at
  capture time by grepping the raw output for `"goal_ref":"warnings-clean"`
  (4 hits). Also carries a `by_repo` entry with `"status":"terminated"` filed
  under the `in_progress` bucket (`exit-decouple-1407`), which is lane doc
  point 3's exact case, present here for free.
  `blocked[]` carries `stuck` and `over_budget` only — this machine has no
  `:starmap_roadmap_goal` app-env goal configured, so `cause: dag` never
  occurs, and no `error`-cause run was live at capture time either.
  `fleet_remote` is `[]` — no other machine had posted bus facts at capture
  time. Used by T3 (decode), T4 (`Snapshot()` happy path).

- **`status-run.json`** — `kazi status warnings-clean --json`, captured
  2026-08-14. `warnings-clean` is a real, currently-stuck goal on this
  machine (see `portfolio-populated.json`'s `by_repo`); this repo's own
  `.dira/entries/` carries no `realized_by` edge yet, so this is the
  "synthetic-but-real kazi goal" the task allows in that case — a real ref,
  chosen rather than invented. `kind: "run"`, `status: "in_progress"`. Used
  by T6.

- **`status-proposal.json`** — `kazi status prop-e45 --json`, captured
  2026-08-14. `prop-e45` is a real, currently-proposed proposal ref on this
  machine (see `portfolio-populated.json`'s `planned`). `kind: "proposal"`,
  `status: "proposed"`. Used by T6.

## Hand-extended

Both start from `portfolio-populated.json` and add exactly the field(s) named
below — everything else is byte-identical to that file (verified by diffing
before the addition).

- **`portfolio-all-causes.json`** — adds two entries to `blocked[]`:
  - `cause: "dag"` — `{goal_ref, cause, blocked_by, blocker}`, no `run_id` and
    no `red_predicates`, matching `blocker_label/1`'s `cause: :dag` clause in
    `portfolio.ex:207` (`"blocked by: #{dep}"`). This cause cannot occur
    naturally on this machine (lane doc point 9).
  - `cause: "error"` — shaped like the naturally-occurring `stuck` entries
    (`{cause, status, run_id, goal_ref, blocker, red_predicates}`), which
    `portfolio.ex:159` shows share a shape with `stuck`. No `error`-cause run
    was live at capture time.

  Result: `blocked[].cause` now covers all four of `dag | over_budget | error |
  stuck` — `over_budget` and `stuck` already present in the natural recording.
  Used by T2's own completeness test and by T3/T4 decode coverage.

- **`portfolio-fleet-remote.json`** — adds one entry to `fleet_remote`:
  `{goal_ref, bucket: "in_progress", machine}`, matching the shape
  `portfolio.ex`'s `remote_entries/1` and `parse_remote_fact/1` produce (no
  other machine had posted bus facts at capture time, so this array is `[]`
  in the natural recording). Used by T2's completeness test and T4.

## Synthesized (no natural recording is possible)

- **`portfolio-empty.json`** — `portfolio` is machine-global (lane doc point
  7: byte-identical output regardless of cwd), so there is no workspace this
  machine can be pointed at to get a genuinely empty portfolio short of
  wiping kazi's own state, which this task does not do. This file is
  constructed to match the exact envelope `portfolio_json/1` /
  `portfolio_totals_json/1` / `fleet_rate/1` emit for the empty case, read
  directly from `lib/kazi/portfolio.ex` (`totals/1`: `base: 0` implies
  `empty?: true` and `rows: []`, since `largest_remainder(_counts, 0)`
  returns `[]`; `fleet_rate/1`: no running entries with a recorded rate
  yields `%{green: 0, total: 0, delta: 0, empty?: true}`) and
  `lib/kazi/cli.ex:4249` (`portfolio_json/1`, `portfolio_totals_json/1`).
  `totals.empty: true`, `rate."empty?": true`. Used by T4's empty-portfolio
  clause.

- **`portfolio-schema-drift.json`** — byte-identical to
  `portfolio-populated.json` except `schema_version: 3` in place of `2`.
  Used by T5.

## Format

Every file is pretty-printed (`indent=2, sort_keys=True`) for a legible diff.
kazi itself emits compact JSON on one line; `encoding/json.Valid` and every
decoder in this package are whitespace-insensitive, so the reformatting
changes nothing a test observes.

## fakekazi/

Executable stub `kazi` sources for T7 — five fakes plus one control, run
against a real (temp) `PATH` rather than an injected decoder seam. See that
task's own comments for what each one does.
