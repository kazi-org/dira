# dira — roadmap

Living document. Updated on every merge, lane claim, new blocker, and decision.

**Last updated:** 2026-07-29 · **Owner:** maintainer

> **Nothing is forgotten by construction.** `python3 scripts/coverage.py` extracts
> every obligation mechanically from `.dira/entries/`, `DESIGN.md`, this file, and
> `docs/plan.md`, then fails if any lacks a disposition in
> [`docs/coverage.md`](coverage.md). Adding a Blocked row or an upstream ask here
> creates an obligation the check will demand you account for. It becomes a CI gate
> in E0.

---

## Shipped

Nothing. There is no binary yet — dira is at the end of its design phase.

---

## In progress

| Item | Owner | Notes |
|---|---|---|
| Privacy enforcement + coverage gate | maintainer | 2026-07-30. `scripts/coverage.py` (nothing-forgotten gate, 47 obligations) and `scripts/privacy-lint.py` (cst-0003 enforcement, 4 invariants). Both verified red→green. |
| Founding design + repo scaffold | maintainer | 2026-07-29. Design doc v2, entry schema, founding ledger (21 entries — 3 intents, 9 decisions, 4 constraints, 5 questions; all validate against the schema, no dangling edges), hook config, license. |

---

## In flight (PRs open)

None.

---

## Planned

Milestones, ordered so that **M1 alone is already more useful than any alternative**
surveyed in `docs/design.md` §1.1. Each milestone's exit criterion is behavioural, not
a checklist of files.

### M1 — the ledger
Entry schema (done) · storage interface with the `local` backend · `dira log`, `why`,
`brief`, `reindex` · SQLite derived cache · `SessionStart` brief injection.

**Exit:** `dira brief` renders in well under 100ms on a cold cache, and one real
decision made in week 1 correctly shapes a session in week 2 without anyone opening a
file.

### M2 — capture
`dira sniff` (regex tier) · the Claude Code skill (semantic tier) · `Stop` and
`PreCompact` hooks · staged-entry disposition flow.

**Exit:** a four-hour session's decisions land in the ledger without being typed, and
a compaction event loses nothing.

### M3 — the enforcer
`dira check` · `dira supersede` · constraint inheritance down the tier chain.

**Exit:** one genuine relitigation attempt is caught and cited back with its own
recorded why_not, before predicates are drafted.

### M4 — derived status
`kazi portfolio --json` join · `dira map` · decision-blocked detection · graceful
degradation when kazi is absent.

**Exit:** `dira map` matches reality with zero hand-entered status, and correctly
distinguishes execution-blocked from decision-blocked.

### M5 — tiers
Workspace and personal ledgers · namespaced ref resolution · orphan-work drift flag ·
`dira init --interview`.

**Exit:** "what are we actually doing on Sire?" is answered by a derived report rather
than reconstructed from memory. **Gated on qst-0001.**

### M6 — surfaces & distribution
`dira ui` as server-rendered Go templates + `embed.FS`, and `dira render` emitting
static HTML the user deploys — **not an SPA and no dira-operated host** (`dec-0012`).
The public ledger renderer is the growth engine (`dec-0010`).

Pulled ahead of the apps because it is the growth engine, not a nicety: dira is
invisible by design, so without a rendered artifact there is nothing to screenshot,
link, or land on — and a solo maintainer has no distribution other than organic.
Read-only and additive, so cst-0004 still holds: the CLI behaves identically whether
the renderer exists or not.

**Exit:** a stranger can read a public repo's why-graph in a browser, understand a
decision they had no context for, and find dira from it.

### M7 — apps
`github` storage backend · PWA · paid iOS + desktop apps (dec-0007).

**Exit:** a decision confirmed on a phone appears in the next session's brief on the
laptop, with no dira server involved.

---

## Blocked

| Item | Blocked on |
|---|---|
| **Automatic disposition capture** (part of M2/M4) | [qst-0005](../.dira/entries/qst-0005.md) issue 2 — kazi has no post-disposition hook on `approve`/`reject`. Fallback exists (the skill wraps the commands), so this degrades ergonomics rather than blocking the milestone. |
| **cst-0003 rule 2 has no runtime check** | `scripts/privacy-lint.py` enforces the marker, label-leak, ref-declaration and ADR-prose invariants, but it cannot verify that *inherited context was never persisted* into a child ledger — that needs an assertion in the brief-injection path, which lands with E1/E5. Until then rule 2 rests on care, which is what cst-0003 forbids. |
| **ADR back-catalogue import** | [qst-0003](../.dira/entries/qst-0003.md) — needs validating against kazi's real 83-ADR corpus before committing to an approach. Leaning index-everything, promote-on-first-citation. **Promoted to a launch blocker by [dec-0010](../.dira/entries/dec-0010.md):** import is the day-1 acquisition moment, so its quality is now a growth variable rather than a convenience. |

---

## Upstream asks (kazi-org/kazi)

All additive, none breaking. Tracked as [qst-0005](../.dira/entries/qst-0005.md);
issues not yet filed.

| # | Ask | Why |
|---|---|---|
| 1 | Register `portfolio` in `Kazi.CLI.Schema` | It already emits `schema_version` and a stable shape, but is absent from the documented `--json` registry (which covers only `apply`, `plan`, `bus`, `status`). Registering it makes dira's join surface a real contract instead of an implementation detail dira happens to read. |
| 2 | Post-disposition hook on `approve` / `reject` | The highest-leverage capture point in the design — an approval *is* a decision event. Must be a **direct synchronous hook, not** a `bus post`: ADR-0067's bus needs `kazi daemon` and no-ops when it is down, so dispositions would be silently lost, and its event stream is age-bounded to ~24h with a ~1 KB cap. Wrong shape and wrong durability for capture. |

**Was three asks; now two.** The third — an optional `why` field on goal-files — was
dropped on verification: goal-files already carry a free-form `metadata` table,
documented at `lib/kazi/goal/loader.ex:24` as a "string-keyed map, verbatim" and read
with no validation (`loader.ex:640`). dira writes `metadata.why = "dira:dec-NNNN"`
today. This also removes the only ask that would have put a dira-shaped string into
kazi's file format.

Risk worth stating: ask 2 could read as dira colonising kazi's surface with its own
concerns, and kazi may reasonably refuse — the fallback (the skill wraps the commands)
is recorded in qst-0005. Ask 1 is defensible on kazi's own terms regardless of dira:
an emitted-but-undocumented `--json` shape is a gap in kazi's public contract whether
or not anything consumes it.

---

## Not planned (deliberately)

Recorded so they are not rediscovered as ideas. Each is a tool dira has promised not to
become — see [cst-0002](../.dira/entries/cst-0002.md) and
[int-0003](../.dira/entries/int-0003.md).

- **Tasks / a tracker** — kazi owns execution and doneness.
- **Calendar, journal, CRM, dashboards** — the closed entry set exists to prevent this.
- **A required hosted service** — [cst-0004](../.dira/entries/cst-0004.md).
- **Attention-drift scoring** — [qst-0002](../.dira/entries/qst-0002.md). The one
  proposed feature closer to self-surveillance than navigation. May never be built,
  and that is an acceptable outcome.
