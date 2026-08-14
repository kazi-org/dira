# Activating dira.sire.run — founder runbook

**Status: ready for David.** Every step below is founder-gated on purpose
(docs/plan/website.md): no session may create a DNS record, enable GitHub Pages,
approve the `pages-live` environment, or run the deploy dispatch. This document is
where that activation lives, in one sitting, once you decide to do it.

Nothing in `site/` or `.github/workflows/site.yml` does any of this automatically.
The deploy job in `site.yml` is `workflow_dispatch`-only and additionally gated
behind the `pages-live` GitHub Environment — it structurally cannot fire from a
push or a merge (see that workflow's header comment for the mechanism).

Estimated time: 15–20 minutes, plus DNS propagation (usually minutes, occasionally
up to 24h) and GitHub's own HTTPS certificate issuance (usually a few minutes after
DNS is verified).

## 0. Prerequisites

- You need DNS control over `sire.run` (wherever its records are managed today —
  check kazi.sire.run's existing DNS provider first; dira.sire.run is almost
  certainly best added at the same provider).
- You need admin access to `kazi-org/dira` on GitHub (to enable Pages and create
  the environment).
- The `site` branch (or whatever branch carries this plan's work) should be merged
  to `main` first — GitHub Pages serves from a build artifact this workflow
  produces on `main`.

## 1. DNS: add the CNAME record

At your DNS provider, add:

| Type | Host | Value | TTL |
|---|---|---|---|
| CNAME | `dira` (i.e. `dira.sire.run`) | `kazi-org.github.io` | 3600 (or provider default) |

This is the same pattern kazi.sire.run already uses (kazi ADR-0018) — `dira`
becomes a subdomain of `sire.run` pointed at GitHub's Pages hosting. GitHub's own
CNAME-verification step (next section) will tell you if anything is wrong with
this record.

Do **not** point it at an A record / GitHub's IP addresses — CNAME to
`kazi-org.github.io` is what GitHub's Pages custom-domain flow expects for this
setup, and it's what `site/public/CNAME` (already committed, contains
`dira.sire.run`) tells the Pages build to serve.

## 2. GitHub Pages: enable the custom domain

1. Go to `https://github.com/kazi-org/dira/settings/pages`.
2. Under **Build and deployment**, source should already be **GitHub Actions**
   (this repo's `site.yml` publishes via `actions/deploy-pages`, not a branch) —
   if it shows "Deploy from a branch" instead, switch it to **GitHub Actions**.
3. Under **Custom domain**, enter `dira.sire.run` and save. GitHub will check the
   CNAME record from step 1; if DNS hasn't propagated yet this can take a few
   minutes to a few hours — retry the save if it fails immediately after adding
   the record.
4. Once GitHub verifies the domain, check **Enforce HTTPS**. This may be greyed
   out for a few minutes after domain verification while GitHub issues the
   certificate — wait and refresh, then check it.

## 3. Create and gate the `pages-live` environment

1. Go to `https://github.com/kazi-org/dira/settings/environments`.
2. Click **New environment**, name it exactly `pages-live` (the workflow's
   `deploy` job references this name — see `.github/workflows/site.yml`).
3. Under **Deployment protection rules**, add **Required reviewers** and name
   yourself (David). This is what makes `workflow_dispatch` alone insufficient to
   deploy — even someone with write access who runs the dispatch has to wait for
   your approval before the `deploy` job actually executes.
4. Save.

## 4. Run the first deploy

1. Go to `https://github.com/kazi-org/dira/actions/workflows/site.yml`.
2. Click **Run workflow**, choose `main`, click the green **Run workflow** button.
3. The `build` job runs first (coherence check, Go build, Astro build, Playwright
   smoke). If it's red, stop here — do not approve the deploy job on a red build.
4. Once `build` is green, the `deploy` job will show as **Waiting** for your
   environment review (step 3 above). Approve it from the run's page.
5. Watch the `deploy` job finish. Its summary links the live Pages URL.

## 5. Verify the live site

From a terminal, once step 4 finishes:

```
curl -sI https://dira.sire.run/ | head -1
curl -sI https://dira.sire.run/why/dec-0001/ | head -1
curl -s https://dira.sire.run/why/dec-0001/ | grep -o "Elixir/OTP" | head -1
curl -s https://dira.sire.run/ | grep -o "Never explain the same decision twice\."
```

Expect: two `HTTP/2 200` (or `HTTP/1.1 200`) headers, `Elixir/OTP` present on the
decision page, and the strapline present on the home page. If any of these fail,
the deploy artifact is wrong or DNS/HTTPS from steps 1–2 hasn't finished
propagating — re-check those before re-running the workflow.

Also worth a manual look in a browser: `https://dira.sire.run/`,
`https://dira.sire.run/docs/`, and `https://dira.sire.run/why/`, on both a desktop
and a phone.

## 6. After it's live

- Update `docs/roadmap.md` (integrator-owned — ask, don't edit directly) to mark
  the site shipped and record the live URL.
- Consider whether `docs/growth/drafts/marketplace-listing.md` and friends are
  ready to un-gate now that there's a real artifact to link to — that's a separate,
  human decision (`status: awaiting-maintainer-approval` in each draft), not
  something this runbook does for you.
