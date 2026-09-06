# Dira website

The site is generated as static HTML by Astro and published to
https://dira.sire.run through GitHub Pages. The homepage adapts the supplied
`cg.html` reference: Pagella type and a visible decision record. The current
[True North identity](BRAND.md) adds cobalt, cool slate, and a D-shaped compass mark.
Shared theme tokens, header, footer, and progressive enhancements live in `src/`.

## Build and verify

Use Node 22.12+ and the Go version in the repository's `go.mod`.

```sh
cd site
npm ci
npm run check:coherence
npm run build
npx playwright install chromium
npm test
```

`dist/` is the complete deployable artifact. No application server is required.
The guide and homepage are authored in Astro; the command reference is generated
from the built CLI's help. `scripts/snapshot-why.mjs` snapshots the CLI's own
read-only UI and all entry pages reachable through its rendered links. It clears
previous snapshots on each build. It never enumerates private entry files.

The site uses local fonts, no analytics, and no external runtime dependencies.
Content and navigation work with JavaScript disabled. JavaScript adds clipboard
copying, mobile-menu behavior, and URL-backed record emphasis.

## Publish

After committing and pushing the intended site revision to `main`:

```sh
gh workflow run site.yml --repo kazi-org/dira --ref main
gh run list --repo kazi-org/dira --workflow site.yml --limit 1
```

The existing workflow builds, runs browser tests, uploads `site/dist`, and deploys
to the `pages-live` environment. Deployment is manually dispatched, not triggered
by every push. `public/CNAME` preserves the custom domain. Check the completed
workflow and the live homepage, guide, reference, and ledger after publishing.
