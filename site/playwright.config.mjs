// W2-T6: Playwright smoke test for the dira website. Runs against the BUILT
// site (site/dist), served by `astro preview`, so a broken page (missing
// strapline, a rejection row dropped from the dec-0001 snapshot, a dead
// internal link) fails CI before it can deploy. Build first: `npm run
// build`, then `npx playwright test` (mirrors kazi's site/playwright.config.mjs).
import { defineConfig, devices } from "@playwright/test";

const PORT = 4325;
const BASE_URL = `http://127.0.0.1:${PORT}`;

export default defineConfig({
  testDir: "./tests",
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: BASE_URL,
    trace: "on-first-retry",
  },
  projects: [
    {
      name: "desktop-chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  // Serves the already-built dist/ — assumes `npm run build` ran first.
  // scripts/serve-dist.mjs, not `astro preview`: see that file's header for
  // why (astro preview 7.2.2 is a shared background daemon, incompatible
  // with Playwright's one-process-per-run webServer lifecycle).
  webServer: {
    command: `node scripts/serve-dist.mjs --port ${PORT} --host 127.0.0.1`,
    url: BASE_URL,
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
});
