import { execSync } from "node:child_process";
import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";
import tailwindcss from "@tailwindcss/vite";

// The displayed version comes from `git describe`, evaluated once at config
// load (plain Node, resolves relative to the repo this file is checked out
// in — the worker builds from a local worktree, CI from a full checkout).
// dira has no release-please manifest yet (no tags exist: E0-L4/L5 are the
// lanes that will cut the first one) so there is nothing to read a pinned
// version out of. `git describe` on a tagless repo throws; that failure IS
// the untagged state, not an error to route around, so the catch's fallback
// is the honest label rather than a guess. Swap point: once E0-L4/L5 lands a
// tag, `git describe` starts returning it with zero code change here.
function diraVersion() {
  try {
    return execSync("git describe --tags --always", {
      cwd: new URL("..", import.meta.url),
      stdio: ["ignore", "pipe", "ignore"],
    })
      .toString()
      .trim();
  } catch {
    return "pre-release";
  }
}
const DIRA_VERSION = diraVersion();

// Served at the custom domain dira.sire.run (mirrors kazi.sire.run, kazi
// ADR-0018 — see docs/plan/website.md). `base` stays "/" because the custom
// domain is the root; the default kazi-org.github.io/dira path only matters
// before DNS is wired (docs/growth/site-activation.md, founder-gated).
export default defineConfig({
  site: "https://dira.sire.run",
  base: "/",
  // @astrojs/sitemap emits sitemap-index.xml + sitemap-0.xml at build time by
  // walking the generated route set — a new static page is picked up with no
  // config change.
  integrations: [sitemap()],
  vite: {
    plugins: [tailwindcss()],
    define: { __DIRA_VERSION__: JSON.stringify(DIRA_VERSION) },
  },
});
