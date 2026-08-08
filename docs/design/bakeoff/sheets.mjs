// Builds the two comparison sheets, then renders them.
//
//   sheet-compare.png   laptop width (1024), the same real paragraph in every
//                       candidate at the real tokens and the real 64ch measure,
//                       so the measure difference is visible and not just tabulated
//   sheet-letterforms.png  the same single line of .grounds prose re-rendered at
//                       3x (49.5px) so the actual letterforms are legible
//
//   node docs/design/bakeoff/sheets.mjs

import { chromium } from 'playwright';
import { createServer } from 'node:http';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { extname, join, resolve } from 'node:path';

const HERE = import.meta.dirname;
const ROOT = resolve(HERE, '../../..');
const OUT = join(HERE, 'renders');
await mkdir(OUT, { recursive: true });

// control first: it is the baseline, not a candidate.
const FACES = [
  { id: 'control', label: 'CONTROL — Palatino stack as it ships today',
    note: 'macOS only. Stock Linux gets DejaVu Serif.', stack: '"Palatino","Palatino Linotype","Book Antiqua",Georgia,serif' },
  { id: 'p052', label: 'URW P052', note: 'Palatino metric clone · AGPL-3.0 + PS/PDF-only exception' },
  { id: 'pagella', label: 'TeX Gyre Pagella', note: 'Palatino metric clone · GUST Font Licence (LPPL 1.3c)' },
  { id: 'sourceserif4', label: 'Source Serif 4', note: 'OFL 1.1 · screen-designed, contemporary' },
  { id: 'newsreader', label: 'Newsreader', note: 'OFL 1.1 · long-form screen reading, editorial' },
  { id: 'literata', label: 'Literata', note: 'OFL 1.1 · long-form screen reading, sturdier' },
];

// real content off the page, not lorem
const GROUNDS = `BEAM start-up costs tens to hundreds of milliseconds before any work happens, and a Burrito-wrapped binary pays a first-run unpacking cost besides. Free CI and free distribution is a real saving — and it loses anyway, because paying BEAM&rsquo;s start-up for a process with no concurrency and no uptime buys the one thing that does not apply.`;
const RULING = `Go, not Elixir/OTP, despite kazi&rsquo;s stack.`;
const LINE = `Free CI and free distribution is a real saving`;

const fontFaces = FACES.filter(f => !f.stack).map(f => `
@font-face { font-family: "BO-${f.id}"; src: url("fonts/${f.id}-regular.core.woff2") format("woff2"); font-weight: 400; font-style: normal; font-display: block; }
@font-face { font-family: "BO-${f.id}"; src: url("fonts/${f.id}-italic.core.woff2") format("woff2"); font-weight: 400; font-style: italic; font-display: block; }
@font-face { font-family: "BO-${f.id}"; src: url("fonts/${f.id}-bold.core.woff2") format("woff2"); font-weight: 600 700; font-style: normal; font-display: block; }`).join('');

// One class per face rather than an inline style: the family names are quoted,
// and a quoted family inside a style="" attribute terminates the attribute.
const stackOf = f => f.stack ?? `"BO-${f.id}", serif`;
const faceClasses = FACES.map(f => `.face-${f.id} { font-family: ${stackOf(f)}; }`).join('\n');

const shell = (title, body, width) => `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>${title}</title>
<link rel="stylesheet" href="_snapshot/tokens.css">
<style>
${fontFaces}
${faceClasses}
body { background: var(--ground); color: var(--ink); width: ${width}px; }
.sheet { padding: var(--s6) var(--s5) var(--s7); }
h1.sheet-title { font-family: var(--ui); font-size: 15px; font-weight: 600; letter-spacing: .02em;
  color: var(--ink); margin-bottom: var(--s2); }
p.sheet-sub { font-family: var(--ui); font-size: 12.5px; color: var(--ink-low); max-width: 78ch;
  margin-bottom: var(--s6); line-height: 1.6; }
.spec { border-top: 1px solid var(--rule); padding-top: var(--s3); margin-bottom: var(--s6); }
.spec:last-child { margin-bottom: 0; }
.spec .who { font-family: var(--ui); font-size: 11px; letter-spacing: .13em; text-transform: uppercase;
  font-weight: 600; color: var(--bearing); margin-bottom: 2px; }
.spec .note { font-family: var(--mono); font-size: 11px; color: var(--ink-low); margin-bottom: var(--s4); }
.spec .metric { font-family: var(--mono); font-size: 11px; color: var(--ink-mid); margin-top: var(--s2); }
${body.css ?? ''}
</style></head><body>${body.html}</body></html>`;

// ---- sheet 1: same paragraph, real tokens, real measure ---------------------
const compare = {
  css: `
/* font-family comes from .face-*; setting it here too would fight it on equal
   specificity and the later rule would silently win for every candidate. */
.ruling-x { font-weight: 400; font-size: 52px; line-height: 1.1;
  letter-spacing: -.019em; max-width: 24ch; margin-bottom: var(--s4); }
.grounds-x { font-size: var(--t-body); color: var(--ink-mid);
  max-width: var(--m-prose); line-height: 1.55; }
.rule-edge { position: relative; }
/* the measure made visible: a hairline at the 64ch ceiling of THIS face */
.rule-edge::after { content: ""; position: absolute; top: 0; bottom: 0; left: var(--m-prose);
  width: 1px; background: color-mix(in oklab, var(--bearing) 45%, transparent); }`,
  html: `<div class="sheet">
<h1 class="sheet-title">dira serif bake-off — the same real content, laptop width (1024)</h1>
<p class="sheet-sub">Identical markup and identical tokens in every block below. The only variable is the serif.
The hairline marks where <code>--m-prose: 64ch</code> lands for that face — <code>ch</code> is the advance width of “0”
in the resolved font, so the measure ceiling itself moves when the serif moves. Display is 52px, prose is 16.5px, both straight off tokens.css.</p>
${FACES.map(f => `<section class="spec">
  <div class="who">${f.label}</div>
  <div class="note">${f.note}</div>
  <div class="face-${f.id}">
    <div class="ruling-x face-${f.id}">${RULING}</div>
    <div class="rule-edge"><p class="grounds-x face-${f.id}" data-face="${f.id}">${GROUNDS}</p></div>
  </div>
</section>`).join('\n')}
</div>`,
};

// ---- sheet 2: letterforms at 3x --------------------------------------------
const letterforms = {
  css: `
.big { font-size: 49.5px; line-height: 1.35; color: var(--ink); white-space: nowrap; }
.pangram { font-size: 26px; color: var(--ink-mid); margin-top: 6px; white-space: nowrap; }`,
  html: `<div class="sheet">
<h1 class="sheet-title">dira serif bake-off — letterforms at 3×</h1>
<p class="sheet-sub">One line of the real <code>.grounds</code> prose, re-rendered at 49.5px — three times the 16.5px it ships at.
This is a true optical enlargement, not a pixel zoom: what you are looking at is the letterform, not the rasteriser.
Second line in each block carries the characters that separate these faces — <b>a g y e R Q &amp; 1 0</b> — plus the italic and the 600.</p>
${FACES.map(f => `<section class="spec">
  <div class="who">${f.label}</div>
  <div class="note">${f.note}</div>
  <div class="face-${f.id}">
    <div class="big face-${f.id}">${LINE}</div>
    <div class="pangram face-${f.id}">agyeRQ&amp;10 · <i>italic grounds</i> · <b style="font-weight:600">upheld</b> · “curly” — dash ×</div>
  </div>
</section>`).join('\n')}
</div>`,
};

await writeFile(join(HERE, 'sheet-compare.html'), shell('Bake-off — comparison', compare, 1024));
await writeFile(join(HERE, 'sheet-letterforms.html'), shell('Bake-off — letterforms 3x', letterforms, 1240));

// ---- render ----------------------------------------------------------------
const MIME = { '.html': 'text/html', '.css': 'text/css', '.woff2': 'font/woff2', '.png': 'image/png' };
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
const problems = [];
for (const [file, width] of [['sheet-compare', 1024], ['sheet-letterforms', 1240]]) {
  for (const colorScheme of ['light', 'dark']) {
    const ctx = await browser.newContext({ viewport: { width, height: 900 }, colorScheme,
      deviceScaleFactor: 2, reducedMotion: 'reduce' });
    const page = await ctx.newPage();
    page.on('requestfailed', r => problems.push(`${file} ${colorScheme}: FAILED ${r.url()}`));
    page.on('pageerror', e => problems.push(`${file} ${colorScheme}: ${e.message}`));
    await page.goto(`${BASE}/docs/design/bakeoff/${file}.html`, { waitUntil: 'load' });
    await page.evaluate(() => document.fonts.ready);
    await page.waitForTimeout(500);
    // Check the faces this sheet actually uses, not every registered FontFace:
    // an unused @font-face legitimately stays 'unloaded' and would false-positive.
    const bad = await page.evaluate(() => {
      const ids = [...new Set([...document.querySelectorAll('[class*="face-"]')]
        .flatMap(e => [...e.classList]).filter(c => c.startsWith('face-'))
        .map(c => c.slice(5)))].filter(id => id !== 'control');
      return ids.filter(id => !document.fonts.check(`16px "BO-${id}"`))
                .map(id => `BO-${id}`);
    });
    if (bad.length) problems.push(`${file} ${colorScheme}: faces not usable — ${bad.join(', ')}`);
    // and every candidate must render at a different width, or one silently fell back
    const widths = await page.evaluate(() => Object.fromEntries(
      [...document.querySelectorAll('.spec')].map(s => {
        const probe = document.createElement('span');
        probe.style.cssText = 'position:absolute;visibility:hidden;white-space:nowrap;font-size:100px';
        probe.textContent = 'Handgloves 0123';
        s.querySelector('[class*="face-"]').appendChild(probe);
        const w = probe.getBoundingClientRect().width;
        probe.remove();
        return [s.querySelector('.who').textContent, Math.round(w * 100) / 100];
      })));
    const seen = new Map();
    for (const [who, w] of Object.entries(widths)) {
      if (seen.has(w)) problems.push(`${file} ${colorScheme}: "${who}" sets identically to "${seen.get(w)}" (${w}px) — one is falling back`);
      seen.set(w, who);
    }
    await page.screenshot({ path: join(OUT, `${file}-${colorScheme}.png`), fullPage: true });
    await ctx.close();
  }
}
await browser.close();
server.close();

console.log(problems.length ? 'SHEET PROBLEMS:\n  ' + problems.join('\n  ')
                            : 'sheets rendered clean — every face loaded');
if (problems.length) process.exitCode = 1;
