# dira.sire.run — the public website (standalone plan)

**Last updated:** 2026-08-14 (UTC) · **Owner:** maintainer · **Requested by:** founder, 2026-08-14

A standalone plan, deliberately outside the E0-E9 epic tree: E8 already carries its
six-lane bound, and this is founder-added scope with its own finish line. The roadmap
points here; `docs/plan.md` stays authoritative for the epic tree.

## Context

dira needs a public website at **dira.sire.run**, mirroring kazi's proven setup
(kazi.sire.run, kazi ADR-0018): an Astro + Tailwind static site in `site/`, built and
deployed by a GitHub Pages workflow, custom domain via `public/CNAME`, Playwright
smoke tests, and a README-to-site coherence gate so the two never drift.

Why a site at all: **dira is invisible by design** — it runs in hooks and prints to
terminals — so without a rendered artifact there is nothing to screenshot, link, or
land on (`dec-0010`: the public ledger renderer is the growth engine; `note-0001`:
the stranger path). The site's centerpiece is therefore **dira's own ledger, rendered**:
a stranger lands on a real decision page (`dec-0001`, Go-not-Elixir, with its rejected
alternatives and `revisit_if`) and understands what dira is by reading its output.

Settled constraints, cited not re-litigated:
- `dec-0012` — static output, no SPA behavior, no dira-operated host. GitHub Pages
  static hosting matches kazi's precedent exactly. Astro builds to static HTML.
- `cst-0004` — the CLI behaves identically whether the site exists or not. The site
  consumes dira's output; nothing in the binary knows about it.
- **Founder-gated:** DNS record creation, enabling the Pages deployment, and the
  first live deploy. Every task below stops at artifact-ready and locally verified.

Existing groundwork this plan builds on (never duplicates):
- `docs/growth/drafts/` — the gated landing copy (E8-L2). The site's landing page
  sources its copy from there; the drafts remain the single source of truth.
- `docs/plan/tasks/E8-L4.md` (demo recordings) and `E8-L6.md` (launch-readiness
  checker) — separate lanes, running in the main pool. The site links their
  artifacts when they exist; no task here waits on them.
- `dira ui` (E6-L2, shipped) serves read-only ledger pages from the binary; the
  `render` verb and the distill web surface are E6-L3, in flight. Until `render`
  ships, the site snapshots `dira ui` output at build time (T3); when E6-L3 lands,
  T3's build script swaps to `dira render` with no page changes.

## Discovery Summary

Verified against the kazi repo on disk (2026-08-14): `site/astro.config.mjs` bakes
the version badge from the release manifest so it cannot drift; `pages.yml` triggers
on site/, README.md, and the manifest, and is gated by the README-site coherence
check; smoke tests live in `site/tests` with their own workflow. dira today: 30 of 40
lanes merged, 14+ verbs working, no release tags yet (E0-L4/L5 in flight), so the
site's version badge reads "pre-release" until the first tag exists (T1 notes the
swap point). Playwright + Chromium are already installed on this machine for the
design gates.

## Use Case Summary

This repo keys `verifies:` on ledger entries, not UC- ids. The site serves:
`dec-0010` (rendered ledger as the acquisition surface), `note-0001` (the stranger
path), `dec-0012` (static, non-hosted), plus the deliverable-shaped launch assets.

## Checkable Work Breakdown

Wave 1 — the site exists and shows the ledger (parallel-safe after T1):

- [x] W1-T1 Scaffold `site/`: Astro + Tailwind + sitemap, `site: "https://dira.sire.run"`,
  `base: "/"`, `public/CNAME` = `dira.sire.run`, version badge sourced from git
  describe with a "pre-release" fallback until E0's release scaffolding tags. Record
  the architecture decision in dira's own ledger via `dira log` (mirror-kazi-site,
  citing dec-0012 and kazi ADR-0018) — the site plan dogfoods the product.
  `verifies: [dec-0012]` · `acc: [cd site && npm ci && npm run build emits dist/index.html and dist/CNAME containing dira.sire.run]`
- [x] W1-T2 Landing page `src/pages/index.astro` from the E8-L2 gated draft copy in
  `docs/growth/drafts/` — copy sourced, not rewritten; strapline identical to
  README's ("Never explain the same decision twice."). No analytics, no trackers.
  `verifies: [note-0001]` · `acc: [built dist/index.html contains the README strapline verbatim and no docs/growth draft-gate marker strings]`
- [x] W1-T3 The why pages: a build script that runs the locally built dira binary,
  serves `dira ui` against dira's own `.dira/` ledger, snapshots `/why/dec-0001` (and
  the index) to static HTML under `site/src/pages/why/`, styled by the site shell.
  Swap point to `dira render` documented inline for when E6-L3 merges.
  `verifies: [dec-0010]` · `acc: [dist/why/dec-0001/index.html exists and contains the rejected alternative "Elixir/OTP" with its why_not text]`
- [x] W1-T4 Docs page: install-from-source, the verb tour (one line + one real output
  block per verb, generated from the built binary so it cannot go stale), link map to
  repo docs. `verifies: [note-0001]` · `acc: [a site build with a verb missing from the generated tour fails; the complete tour passes — both sides proven]`

Wave 2 — gates, tests, and the founder handoff:

- [x] W2-T5 `.github/workflows/site.yml`: PR-triggered build job (always), deploy job
  present but `workflow_dispatch`-only behind a `pages-live` environment — the deploy
  exists, is reviewable, and CANNOT fire until the founder activates it.
  `verifies: [infrastructure]` · `acc: [CI on a PR touching site/ runs the build job green; the deploy job is absent from push-triggered runs]`
- [x] W2-T6 Playwright smoke in `site/tests/`: homepage renders with strapline, the
  dec-0001 why page shows all four rejection rows, zero broken internal links.
  `verifies: [dec-0010]` · `acc: [npx playwright test in site/ exits 0 locally; deleting the why snapshot makes the link test fail — both sides proven]`
- [x] W2-T7 README-site coherence gate (kazi's canonical.mjs pattern): strapline,
  verb list, and install instructions must match between README.md and the site
  source; wired into the site build. `verifies: [infrastructure]` ·
  `acc: [mutating the README strapline makes the coherence check exit non-zero; restoring it passes — both sides proven]`
- [x] W2-T8 Founder activation runbook `docs/growth/site-activation.md`: the exact
  DNS CNAME record, GitHub Pages custom-domain + HTTPS steps, environment approval,
  first-deploy command, and post-deploy verification (curl the live why page). Ends
  at "ready for David"; contains no action a session may take itself.
  `delivers: [one-sitting activation runbook for dira.sire.run]`

## Parallel Work

One worker owns this plan end-to-end (site/ is a new directory; zero collision with
the running pool lanes). If split, T2/T3/T4 parallelize after T1; wave 2 after wave 1.

## Risk Register

| risk | mitigation |
|---|---|
| A second renderer grows in the site and drifts from `dira ui` | T3 snapshots the binary's own output; the swap to `dira render` is a build-script change only |
| The site deploys before the founder wired DNS | deploy job is dispatch-only behind an environment until the runbook is executed |
| Copy drifts from the gated drafts / README | T2 sources copy; T7 gates coherence in CI both ways |
| No release tag exists for the version badge | explicit "pre-release" fallback in T1; swap noted for E0-L4/L5 |

## Progress Log

- 2026-08-14: plan created (W1-T1..W2-T8, 7 engineering tasks with acc: lines, 1
  deliverable task). No ADR files: this repo records decisions as ledger entries;
  the architecture decision lands via `dira log` in W1-T1.
