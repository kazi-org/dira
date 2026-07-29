# dira — roadmap

Living document. Updated on every merge, lane claim, new blocker, and decision.

**Last updated:** 2026-07-29 · **Owner:** maintainer

---

## Shipped

Nothing. There is no binary yet — dira is at the end of its design phase.

---

## In progress

| Item | Owner | Notes |
|---|---|---|
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

### M6 — surfaces
`dira ui` embedded SPA · `github` storage backend · PWA · paid iOS + desktop apps
(dec-0007).

**Exit:** a decision confirmed on a phone appears in the next session's brief on the
laptop, with no dira server involved.

---

## Blocked

| Item | Blocked on |
|---|---|
| **M5 — workspace tier** | [qst-0001](../.dira/entries/qst-0001.md) — how a public repo ledger resolves a parent ref living in a *private* ledger. Three candidate answers, none obviously right; needs a decision on who the public `.dira/` is for. |
| **Automatic disposition capture** (part of M2/M4) | [qst-0005](../.dira/entries/qst-0005.md) issue 2 — kazi has no post-disposition hook on `approve`/`reject`. Fallback exists (the skill wraps the commands), so this degrades ergonomics rather than blocking the milestone. |
| **ADR back-catalogue import** | [qst-0003](../.dira/entries/qst-0003.md) — needs validating against kazi's real 83-ADR corpus before committing to an approach. Leaning index-everything, promote-on-first-citation. |

---

## Upstream asks (kazi-org/kazi)

All additive, none breaking. Tracked as [qst-0005](../.dira/entries/qst-0005.md);
issues not yet filed.

| # | Ask | Why |
|---|---|---|
| 1 | Register `portfolio` in `Kazi.CLI.Schema` | It already emits `schema_version` and a stable shape, but is absent from the documented `--json` registry (which covers only `apply`, `plan`, `bus`, `status`). Registering it makes dira's join surface a real contract instead of an implementation detail dira happens to read. |
| 2 | Post-disposition hook on `approve` / `reject` | The highest-leverage capture point in the design — an approval *is* a decision event. May be a `bus post` on the existing session bus (ADR-0067) rather than new infrastructure. |
| 3 | Optional `why` field on goal-files | Free text, ignored by kazi, carrying `dira:dec-NNNN`. Closes traceability downward. |

Risk worth stating: asks 2 and 3 could read as dira colonising kazi's surface with its
own concerns, and ask 3 puts a dira-shaped string into kazi's file format. kazi may
reasonably refuse; fallbacks for all three are recorded in qst-0005.

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
