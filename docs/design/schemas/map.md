# `dira map --json` — documented shape

E4-L4-T5's copy of the pattern `docs/schemas/collective-result.md` uses on the
kazi side (cited in `dec-0008`'s own source comments): the shape is written
down once here, and `internal/cli/json_test.go` checks the actual emitted
key set against this file's own text so the two cannot drift apart silently
(`docs/lore.md`'s dominant-finding pattern — a declaration nobody checks
against its result).

`dira map --json` emits **dira's own six bucket names** (`status.Buckets`) —
never kazi's three-value `by_repo` enum (`in_progress | stuck | complete`) or
its five-value `totals.rows[]` enum (`done | running | blocked | todo |
planned`). A `bucket` field is omitted entirely (rather than set to an empty
string) for an entry dec-0004's join table does not cover — a question, a
constraint, a note, or a decision/intent already at rest.

## Top-level keys

| key | type | meaning |
|---|---|---|
| `observed_at` | string (RFC 3339) | when this run executed — the ONE field that legitimately differs between two otherwise-identical runs; strip it before diffing for determinism |
| `groups` | array of `group` | one entry per distinct `derives_from` target that at least one other entry names |
| `unparented` | array of `entry` | every entry naming no `derives_from` target, or naming one absent from the ledger (a dangling edge) |
| `degraded` | `degraded` object, present only when kazi could not be asked | mirrors the text renderer's own degradation line |

## `group`

| key | type | meaning |
|---|---|---|
| `parent` | `entry` | the group's header entry |
| `children` | array of `entry` | this parent's direct children, one level only — a child that is itself a parent elsewhere contributes to `parent`'s own `rollup` as one unit, not by flattening its own children upward |
| `rollup` | object, bucket name → count | this group's children counted by bucket, skipping any child with no bucket at all |

## `entry`

| key | type | meaning |
|---|---|---|
| `id` | string | the ledger entry id |
| `kind` | string | one of dira's five kinds |
| `title` | string | the entry's title |
| `bucket` | string, omitted if not applicable | one of dira's six `status.Bucket` values |
| `blocked_by` | object `{id, title}`, present only when `bucket == "decision_blocked"` | the open question gating this entry |
| `evidence` | object `{run_id, release_ref}`, present only for some `"completed"` rows | by-reference-only evidence (`dec-0004`) — never a predicate vector |
| `ambiguous_statuses` | array of strings, present only when the join could not resolve conflicting runs | every distinct raw status kazi reported across the conflicting runs |
| `unresolved` | object `{ref, reason}`, present only when the join could not answer at all | why |
| `blocks` | string, omitted if absent | the id this entry's own `blocks` edge names — typically an open question's row, independent of `bucket` |

## `degraded`

| key | type | meaning |
|---|---|---|
| `reason` | string | the machine-readable `kazi.UnavailableReason` — `not_on_path`, `nonzero_exit`, `malformed_json`, `wrong_kind`, or `timeout` |
| `message` | string | the same human-readable line the text renderer prints |
