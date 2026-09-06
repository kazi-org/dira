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
  const seedPages = ["/", "/guide/", "/docs/", "/why/", "/why/dec-0001/", "/404.html"];
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

for (const width of [390, 1024, 1440, 2880]) {
  test(`pages fit a ${width}px viewport`, async ({ page }) => {
    await page.setViewportSize({ width, height: 1000 });
    for (const route of ['/', '/guide/', '/docs/', '/why/', '/why/dec-0001/']) {
      await page.goto(route);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), route).toBe(true);
    }
  });
}

test('mobile navigation closes with Escape and returns focus', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  const menu = page.getByRole('button', { name: 'Menu', exact: true });
  await menu.click();
  await expect(page.getByRole('navigation', { name: 'Mobile' })).toBeVisible();
  await page.getByRole('navigation', { name: 'Mobile' }).getByRole('link', { name: 'Guide', exact: true }).focus();
  await page.keyboard.press('Escape');
  await expect(menu).toBeFocused();
  await expect(menu).toHaveAttribute('aria-expanded', 'false');
});

test('record emphasis survives refresh and browser back', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'When to revisit' }).click();
  await expect(page).toHaveURL(/part=revisit/);
  await page.reload();
  await expect(page.getByRole('button', { name: 'When to revisit' })).toHaveAttribute('aria-pressed', 'true');
  await page.getByRole('button', { name: 'Decision', exact: true }).click();
  await page.goBack();
  await expect(page.getByRole('button', { name: 'When to revisit' })).toHaveAttribute('aria-pressed', 'true');
});

test('copy command writes the exact install command', async ({ page, context }) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.goto('/');
  await page.locator('.cmd-row .copy-btn').click();
  await expect(page.locator('.cmd-row .copy-btn')).toHaveText('Copied');
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe('brew install kazi-org/tap/dira');
});

test('no-JS mobile visitors retain content and navigation', async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } });
  const page = await context.newPage();
  await page.goto('http://127.0.0.1:4325/');
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  await expect(page.getByRole('navigation', { name: 'Mobile' })).toBeVisible();
  await expect(page.locator('#record-explain')).not.toBeEmpty();
  await context.close();
});

test('dark and reduced-motion modes keep content visible without runtime errors', async ({ page }) => {
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
  for (const route of ['/', '/guide/', '/docs/', '/404.html']) {
    await page.goto(route);
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  }
  expect(errors).toEqual([]);
});

test('all published ledger entries are reachable as static HTML', async ({ page, request }) => {
  await page.goto('/why/');
  const links = await page.locator("a[href^='/why/']").evaluateAll(els => [...new Set(els.map(el => el.getAttribute('href')))].filter(x => x !== '/why/'));
  expect(links.length).toBeGreaterThan(1);
  for (const link of links) {
    const response = await request.get(link);
    expect(response.status(), link).toBe(200);
    expect(await response.text()).toContain('</html>');
  }
});


test('skip link appears on keyboard focus and reaches main content', async ({ page }) => {
  await page.goto('/');
  const skip = page.getByRole('link', { name: 'Skip to content' });
  expect((await skip.boundingBox()).x).toBeLessThan(0);
  await page.keyboard.press('Tab');
  await expect(skip).toBeFocused();
  expect((await skip.boundingBox()).x).toBeGreaterThanOrEqual(0);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/#main$/);
});
