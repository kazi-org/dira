// W2-T6: smoke tests against the built site, served by `astro preview`.
import { test, expect } from "@playwright/test";
import { STRAPLINE } from "../src/canonical.mjs";

test("homepage renders with the strapline", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(STRAPLINE);
});

test("the dec-0001 why page shows all four rejected alternatives", async ({ page }) => {
  await page.goto("/why/dec-0001/");
  const names = [
    "Elixir/OTP, reusing kazi's Burrito + Homebrew tap + release-please pipeline",
    "Rust",
    "A shell script or Python",
    "A TypeScript CLI on Node/Bun",
  ];
  const rows = page.locator("details.alt");
  await expect(rows).toHaveCount(4);
  for (const name of names) {
    await expect(page.getByText(name, { exact: false }).first()).toBeVisible();
  }
  // Every one is REFUSED — dec-0001 upheld none of its alternatives.
  await expect(page.getByText("refused", { exact: false })).toHaveCount(4);
});

// Crawls the pages a visitor can actually reach from the two entry points
// (the marketing home page and the ledger index) and asserts every
// root-relative link on them resolves. This is the test that catches a
// snapshot going missing: docs/plan/website.md's acceptance line for this
// task is proved by literally deleting site/public/why/dec-0001/index.html
// and observing this test fail on the index page's own link to it, then
// restoring the snapshot and observing it pass again.
test("zero broken internal links", async ({ page, request }) => {
  const seedPages = ["/", "/docs/", "/why/", "/why/dec-0001/"];
  const found = new Set();

  for (const path of seedPages) {
    await page.goto(path);
    const hrefs = await page.locator("a[href^='/']").evaluateAll((els) => els.map((e) => e.getAttribute("href")));
    for (const href of hrefs) found.add(href);
  }

  expect(found.size).toBeGreaterThan(0);

  const broken = [];
  for (const href of found) {
    const res = await request.get(href);
    if (!res.ok()) broken.push(`${href} -> ${res.status()}`);
  }
  expect(broken, `broken internal links:\n${broken.join("\n")}`).toEqual([]);
});
