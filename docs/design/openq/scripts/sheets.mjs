// sheets.mjs — one comparison image per open question.
//
//   node docs/design/openq/scripts/sheets.mjs
//
// The founder asked to decide from pictures. Flipping between six files is not
// deciding from pictures, so each question collapses to ONE png: the options
// side by side at laptop width, with the trade-off under each.
//
// Q1 is compared at full page height and a single scale, because the length of
// the page IS the thing being judged.
// Q2 is compared twice — full page in both schemes, and a tight crop of the
// three-state row, because the whole question is how withheld reads against the
// orphan sitting beside it.

import { chromium } from 'playwright';
import { createServer } from 'node:http';
import { readFile, writeFile } from 'node:fs/promises';
import { extname, join, resolve } from 'node:path';

const ROOT = resolve(import.meta.dirname, '../../../..');
const OUT = resolve(import.meta.dirname, '../renders');
const MIME = { '.html': 'text/html', '.css': 'text/css', '.png': 'image/png' };

const server = createServer(async (req, res) => {
  try {
    const p = join(ROOT, decodeURIComponent(req.url.split('?')[0]));
    const body = await readFile(p);
    res.writeHead(200, { 'content-type': MIME[extname(p)] ?? 'application/octet-stream' });
    res.end(body);
  } catch { res.writeHead(404); res.end('not found'); }
});
await new Promise(r => server.listen(0, '127.0.0.1', r));
const BASE = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch();

// ---- 1. detail crops of the three-state row and the chain (q2 only) ---------
const Q2 = ['q2-a-plain', 'q2-b-declared', 'q2-c-sealed'];
for (const name of Q2) {
  for (const colorScheme of ['light', 'dark']) {
    const ctx = await browser.newContext({
      viewport: { width: 1024, height: 768 }, colorScheme, deviceScaleFactor: 2, reducedMotion: 'reduce',
    });
    const page = await ctx.newPage();
    await page.goto(`${BASE}/docs/design/openq/${name}.html`, { waitUntil: 'load' });
    await page.evaluate(() => document.fonts.ready);
    await page.waitForTimeout(300);
    await page.locator('.stgrid').screenshot({ path: join(OUT, `detail-states-${name}-${colorScheme}.png`) });
    await page.locator('.chain').screenshot({ path: join(OUT, `detail-chain-${name}-${colorScheme}.png`) });
    await ctx.close();
  }
}

// ---- 1b. the refusal device at close range (q1) -----------------------------
// The struck-through name with its grounds beneath is the strongest device in
// the system and the reason this direction beat the terminal anchor. Whether it
// survives twenty repetitions is not visible in a full-page thumbnail, so each
// option gets a fixed 980px window onto its alternatives block at one scale.
const Q1 = ['q1-a-run-long', 'q1-b-progressive', 'q1-c-summary-detail'];
for (const name of Q1) {
  const ctx = await browser.newContext({
    viewport: { width: 1024, height: 768 }, colorScheme: 'light', deviceScaleFactor: 2, reducedMotion: 'reduce',
  });
  const page = await ctx.newPage();
  await page.goto(`${BASE}/docs/design/openq/${name}.html`, { waitUntil: 'load' });
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(300);
  const box = await page.locator('.args').boundingBox();
  await page.screenshot({
    path: join(OUT, `detail-args-${name}.png`),
    fullPage: true,   // clip is page-relative, and .args starts below the fold
    clip: { x: box.x, y: box.y, width: box.width, height: Math.min(box.height, 980) },
  });
  await ctx.close();
}

// ---- 2. the sheets ----------------------------------------------------------
const CHROME = `
:root { color-scheme: dark }
* { box-sizing: border-box; margin: 0 }
body { background: #12161b; color: #e6e9ed; font: 13px/1.5 system-ui, sans-serif; padding: 30px 30px 44px }
h1 { font: 600 19px/1.3 system-ui; margin-bottom: 4px }
.sub { color: #8b939c; font-size: 13px; max-width: 96ch; margin-bottom: 26px }
h2 { font: 600 13px/1.4 ui-monospace, monospace; color: #d9a13f; letter-spacing: .04em;
     margin: 30px 0 12px; text-transform: uppercase }
.cols { display: grid; gap: 20px; align-items: start }
.c3 { grid-template-columns: repeat(3, 1fr) }
.c6 { grid-template-columns: repeat(6, 1fr) }
figure { margin: 0; min-width: 0 }
img { display: block; width: 100%; height: auto; border: 1px solid #2a323d; border-radius: 5px; background: #fff }
.cap { margin-bottom: 11px; min-height: 132px }
.ttl { font-weight: 650; font-size: 13px }
.ttl b { color: #d9a13f; font-family: ui-monospace, monospace; margin-right: 6px }
.trade { color: #aab2bc; font-size: 12.5px; margin-top: 4px }
.cost { color: #8b939c; font-size: 12.5px; margin-top: 4px }
.cost i { color: #e08a7a; font-style: normal; font-weight: 600 }
.meta { font: 11.5px ui-monospace, monospace; color: #6f7883; margin-top: 6px }
.note { color: #8b939c; font-size: 12.5px; margin: 10px 0 0; max-width: 110ch }
.scheme { font: 11px ui-monospace, monospace; color: #6f7883; margin-bottom: 6px }
`;

const q1 = [
  { f: 'q1-a-run-long-laptop-light.png', k: 'A', t: 'Let it run',
    trade: 'Nothing is hidden and nothing is summarised: every refusal keeps its strike-through and its grounds directly beneath, which is the strongest device in the system.',
    cost: 'The page becomes a scroll with no map — the reader has no idea 20 alternatives exist until they reach the twentieth, and the 400-word refusal at position 2 buries the 18 behind it.' },
  { f: 'q1-b-progressive-laptop-light.png', k: 'B', t: 'Progressive disclosure',
    trade: 'Four alternatives read exactly as they do today; the remaining sixteen sit behind one &lt;details&gt;, zero JavaScript, the same collapse mechanism the rebuilt chain uses.',
    cost: 'Fixes the count and not the length — the 400-word refusal is in the visible four, so the first screen is still dominated by one alternative. It also asserts a ranking the ledger does not record: which four?' },
  { f: 'q1-c-summary-detail-laptop-light.png', k: 'C', t: 'Summary / detail split',
    trade: 'All twenty are visible and scannable, each carrying a one-line ground, each opening to full reasoning. The strike-through survives on the summary line.',
    cost: 'The grounds — the testimony this direction was chosen for — are closed by default. r3&rarr;r4 rejected a comparison list for exactly this reason; this is that fix with a hinge on it.' },
];

const q2 = [
  { k: 'A', n: 'q2-a-plain', t: 'Plain speech',
    trade: 'Spends nothing — no hue, no icon, no surface change. Where the parent title would be, the page states what is true, in the same neutral ink a refused alternative uses.',
    cost: 'Borrows no alarm and no presence either. Grey italic text in an otherwise empty slot is also what a failed fetch looks like; the reading it risks is "loading", not "broken".' },
  { k: 'B', n: 'q2-b-declared', t: 'Declared state',
    trade: 'Withheld becomes a first-class resolution state with a mark, a name and a chip, in the instrument hue, reusing the grammar tokens.css already gives .chip-staged.',
    cost: 'Spends --bearing on a second meaning. The hue budget says three hues, one meaning each, and a fourth needs a documented role — this either widens bearing or opens that fourth.' },
  { k: 'C', n: 'q2-c-sealed', t: 'Sealed material',
    trade: 'No hue at all: the absence is drawn as a hatched slab at hairline weight. No error state in any interface is woven, so "covered on purpose" arrives before a word is read.',
    cost: 'Adds a texture primitive the system does not have, and it is the one place a graphic stands in for content — a reviewer will raise it against Law 3 even though there is no content to show.' },
];

// caption ABOVE the image: with a 6.9-screen column next to a 3.7-screen one,
// captions underneath land at wildly different heights and stop being comparable.
const cap = o => `<div class="cap">
  <div class="ttl"><b>${o.k}</b>${o.t}</div>
  <div class="trade">${o.trade}</div>
  <div class="cost"><i>costs:</i> ${o.cost}</div>
  ${o.meta ? `<div class="meta">${o.meta}</div>` : ''}
</div>`;

const heights = JSON.parse(await readFile(join(OUT, 'heights.json'), 'utf8').catch(() => '{}'));
for (const o of q1) {
  const h = heights[o.f.replace('-laptop-light.png', '')];
  if (h) o.meta = `laptop page height ${h}px · ${(h / 768).toFixed(1)} screens`;
}

await writeFile(join(OUT, 'sheet-q1.html'), `<!doctype html><meta charset="utf-8">
<title>dira — open question 1: long content</title><style>${CHROME}</style>
<h1>Open question 1 — long content</h1>
<div class="sub">One decision, twenty alternatives, and a 405-word refusal on alternative 2. Identical content in all three columns — only the disclosure treatment differs. Laptop width (1024px), light scheme, full page, all at one scale, so column height is a real measurement and not a layout artifact.</div>
<h2>the refusal device at close range — same 980px window, same scale</h2>
<div class="cols c3">
${q1.map(o => `<figure><div class="scheme">${o.k} · top of the alternatives block</div><img src="detail-args-${o.f.replace('-laptop-light.png','')}.png" alt="${o.k} alternatives"></figure>`).join('\n')}
</div>

<h2>whole page — laptop, light, one scale</h2>
<div class="cols c3">
${q1.map(o => `<figure>${cap(o)}<img src="${o.f}" alt="option ${o.k}"></figure>`).join('\n')}
</div>
<p class="note">The dark scheme, the mobile and wide viewports, and every option at every size are in renders/index.html. What this sheet cannot show: whether a reader who has scrolled past ten struck-through names still reads the eleventh as testimony or as a list.</p>
`);

await writeFile(join(OUT, 'sheet-q2.html'), `<!doctype html><meta charset="utf-8">
<title>dira — open question 2: the withheld state</title><style>${CHROME}</style>
<h1>Open question 2 — the withheld state</h1>
<div class="sub">A parent that exists in a private ledger and is deliberately unreadable. Withheld must read as neither an error nor a warning; red belongs to drift and contradiction only. Every page shows all three resolution states adjacent — oriented, withheld, orphan — with the real red drift card in the rail, so withheld is judged against the alarm it must not resemble rather than on its own.</div>

<h2>the three-state row — the whole test, cropped, both schemes</h2>
<div class="cols c6">
${q2.flatMap(o => ['light', 'dark'].map(s => `<figure><div class="scheme">${o.k} · ${s}</div><img src="detail-states-${o.n}-${s}.png" alt="${o.k} ${s}"></figure>`)).join('\n')}
</div>

<h2>the same state inside the chain</h2>
<div class="cols c6">
${q2.flatMap(o => ['light', 'dark'].map(s => `<figure><div class="scheme">${o.k} · ${s}</div><img src="detail-chain-${o.n}-${s}.png" alt="${o.k} chain ${s}"></figure>`)).join('\n')}
</div>

<h2>whole page — laptop, both schemes</h2>
<div class="cols c6">
${q2.flatMap(o => ['light', 'dark'].map(s => `<figure><div class="scheme">${o.k} · ${s}</div><img src="${o.n}-laptop-${s}.png" alt="${o.k} page ${s}"></figure>`)).join('\n')}
</div>

<h2>the trade in each</h2>
<div class="cols c3">
${q2.map(o => `<figure>${cap(o)}</figure>`).join('\n')}
</div>
<p class="note">Mobile and wide are in renders/index.html. Every treatment clears 4.5:1 as rendered, measured in the browser with color-mix layers composited, in both schemes.</p>
`);

for (const q of ['q1', 'q2']) {
  const ctx = await browser.newContext({ viewport: { width: 1900, height: 1200 }, deviceScaleFactor: 1 });
  const page = await ctx.newPage();
  await page.goto(`${BASE}/docs/design/openq/renders/sheet-${q}.html`, { waitUntil: 'load' });
  await page.waitForTimeout(500);
  await page.screenshot({ path: join(OUT, `sheet-${q}.png`), fullPage: true });
  const h = await page.evaluate(() => document.body.scrollHeight);
  console.log(`sheet-${q}.png  1900x${h}`);
  await ctx.close();
}

await browser.close();
server.close();
