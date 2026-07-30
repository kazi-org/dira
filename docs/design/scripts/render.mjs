// Headless capture for dira mockups. Per web-design/RENDERER.md.
//
//   node docs/design/scripts/render.mjs <iteration> [glob-substring]
//
// Captures every target x {mobile,laptop,wide} x {light,dark} and runs the
// mechanical gate: console errors, failed requests, blank mount, byte-identical
// light/dark pair (fake dark mode), and post-load layout shift.
//
// Serves the repo over http rather than file:// so relative paths and
// prefers-color-scheme behave exactly as they will in `dira ui`.

import { chromium } from 'playwright';
import { createServer } from 'node:http';
import { readFile, mkdir, writeFile, readdir } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { extname, join, resolve } from 'node:path';

const ROOT = resolve(import.meta.dirname, '../../..');      // repo root
const OUT = resolve(import.meta.dirname, '../renders');
const ITER = process.argv[2] ?? 'r1';
const FILTER = process.argv[3] ?? '';

const VIEWPORTS = { mobile: [390, 844], laptop: [1024, 768], wide: [1440, 900] };
const SCHEMES = ['light', 'dark'];
const MIME = { '.html': 'text/html', '.css': 'text/css', '.js': 'text/javascript',
               '.svg': 'image/svg+xml', '.json': 'application/json', '.png': 'image/png' };

// ---- static server ----------------------------------------------------------
const server = createServer(async (req, res) => {
  try {
    const p = join(ROOT, decodeURIComponent(req.url.split('?')[0]));
    const body = await readFile(p);
    res.writeHead(200, { 'content-type': MIME[extname(p)] ?? 'application/octet-stream' });
    res.end(body);
  } catch {
    res.writeHead(404); res.end('not found');
  }
});
await new Promise(r => server.listen(0, '127.0.0.1', r));
const BASE = `http://127.0.0.1:${server.address().port}`;

// ---- targets ----------------------------------------------------------------
const anchorDir = resolve(import.meta.dirname, '../anchors');
let TARGETS = [];
try {
  for (const f of (await readdir(anchorDir)).filter(f => f.endsWith('.html')).sort()) {
    TARGETS.push([f.replace(/\.html$/, ''), `/docs/design/anchors/${f}`]);
  }
} catch {}
const screensDir = resolve(import.meta.dirname, '../screens');
try {
  for (const f of (await readdir(screensDir)).filter(f => f.endsWith('.html')).sort()) {
    TARGETS.push([f.replace(/\.html$/, ''), `/docs/design/screens/${f}`]);
  }
} catch {}
if (FILTER) TARGETS = TARGETS.filter(([n]) => n.includes(FILTER));

if (!TARGETS.length) { console.error('no targets found'); process.exit(1); }
await mkdir(OUT, { recursive: true });

// ---- capture ----------------------------------------------------------------
const browser = await chromium.launch();
const gate = [];
const hashes = {};

for (const [name, path] of TARGETS) {
  for (const [vp, [width, height]] of Object.entries(VIEWPORTS)) {
    for (const colorScheme of SCHEMES) {
      const ctx = await browser.newContext({
        viewport: { width, height }, colorScheme, deviceScaleFactor: 2,
        reducedMotion: 'reduce',
      });
      const page = await ctx.newPage();
      const errs = [];
      page.on('console', m => m.type() === 'error' && errs.push(`console: ${m.text()}`));
      page.on('pageerror', e => errs.push(`pageerror: ${e.message}`));
      page.on('requestfailed', r => errs.push(`FAILED ${r.url()}`));

      await page.goto(BASE + path, { waitUntil: 'load' });
      await page.evaluate(() => document.fonts.ready);
      await page.waitForTimeout(400);

      // layout-shift probe: measure body height now and at 1500ms
      const h1 = await page.evaluate(() => document.body.scrollHeight);
      const file = join(OUT, `${ITER}-${name}-${vp}-${colorScheme}.png`);
      await page.screenshot({ path: file, fullPage: true });
      await page.waitForTimeout(1100);
      const h2 = await page.evaluate(() => document.body.scrollHeight);
      if (Math.abs(h2 - h1) > 4) errs.push(`layout shift ${h1}px -> ${h2}px after load`);

      // blank-mount probe
      const ink = await page.evaluate(() => document.body.innerText.trim().length);
      if (ink < 40) errs.push(`near-blank capture (${ink} chars of text)`);

      hashes[`${name}-${vp}-${colorScheme}`] =
        createHash('sha1').update(await readFile(file)).digest('hex');

      if (errs.length) gate.push({ name, vp, colorScheme, errs });
      await ctx.close();
    }
  }
}
await browser.close();
server.close();

// ---- fake-dark check --------------------------------------------------------
for (const [name] of TARGETS) {
  for (const vp of Object.keys(VIEWPORTS)) {
    const l = hashes[`${name}-${vp}-light`], d = hashes[`${name}-${vp}-dark`];
    if (l && d && l === d) {
      gate.push({ name, vp, colorScheme: 'both', errs: ['light/dark pair is byte-identical — dark scheme is fake'] });
    }
  }
}

// ---- contact sheet ----------------------------------------------------------
const shots = (await readdir(OUT)).filter(f => f.startsWith(ITER + '-') && f.endsWith('.png')).sort();
const byTarget = {};
for (const f of shots) {
  const m = f.match(new RegExp(`^${ITER}-(.+)-(mobile|laptop|wide)-(light|dark)\\.png$`));
  if (m) (byTarget[m[1]] ??= []).push({ f, vp: m[2], scheme: m[3] });
}
const order = { mobile: 0, laptop: 1, wide: 2 };
await writeFile(join(OUT, `${ITER}-index.html`), `<!doctype html>
<meta charset="utf-8"><title>dira mockups ${ITER}</title>
<style>
:root{color-scheme:light dark}
body{background:#14181d;color:#e6e9ed;font:14px/1.5 system-ui;margin:0;padding:28px}
h1{font-size:17px;letter-spacing:.02em;margin:0 0 20px}
h2{font-size:14px;margin:34px 0 10px;color:#d9a13f;font-family:ui-monospace,monospace}
.row{display:flex;gap:14px;overflow-x:auto;padding-bottom:8px}
figure{margin:0;flex:none}
figcaption{font:11px ui-monospace,monospace;color:#8b939c;margin:6px 0 0}
img{display:block;border:1px solid #262e39;border-radius:6px;background:#fff;height:420px;width:auto}
</style>
<h1>dira mockups — ${ITER}</h1>
${Object.entries(byTarget).map(([t, list]) => `<h2>${t}</h2><div class="row">${
  list.sort((a,b) => (order[a.vp]-order[b.vp]) || a.scheme.localeCompare(b.scheme))
      .map(s => `<figure><img src="${s.f}" alt="${t} ${s.vp} ${s.scheme}"><figcaption>${s.vp} · ${s.scheme}</figcaption></figure>`).join('')
}</div>`).join('\n')}
`);

// ---- report -----------------------------------------------------------------
console.log(`\ncaptured ${shots.length} shots for ${TARGETS.length} target(s) -> docs/design/renders/${ITER}-index.html`);
if (gate.length) {
  console.log(`\nGATE FAIL (${gate.length}):`);
  for (const g of gate) console.log(`  ${g.name} ${g.vp} ${g.colorScheme}\n    - ${g.errs.join('\n    - ')}`);
  process.exitCode = 1;
} else {
  console.log('GATE PASS — no console errors, no failed requests, no blank mounts, no fake dark, no layout shift.');
}
