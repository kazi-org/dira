// render-openq.mjs — the existing harness pattern (docs/design/scripts/render.mjs)
// pointed at docs/design/openq/ so the studies never write into the real renders
// directory and never require editing shared tooling.
//
//   node docs/design/openq/scripts/render-openq.mjs [glob-substring]
//
// Same mechanical gate as the real harness: console errors, failed requests,
// blank mount, byte-identical light/dark pair, post-load layout shift.
//
// Plus one gate the real harness does not have, because the withheld study
// introduces colour treatments the token-level contrast matrix cannot see:
// every element carrying data-contrast is measured AS RENDERED — computed text
// colour against its effective background, compositing every semi-transparent
// color-mix layer up the ancestor chain — and must clear WCAG 4.5:1 in BOTH
// schemes. contrast.mjs proves the tokens; this proves what was built from them.

import { chromium } from 'playwright';
import { createServer } from 'node:http';
import { readFile, mkdir, writeFile, readdir } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { extname, join, resolve } from 'node:path';

const ROOT = resolve(import.meta.dirname, '../../../..');   // repo root
const OUT = resolve(import.meta.dirname, '../renders');
const SRC = resolve(import.meta.dirname, '..');
const FILTER = process.argv[2] ?? '';

const VIEWPORTS = { mobile: [390, 844], laptop: [1024, 768], wide: [1440, 900] };
const SCHEMES = ['light', 'dark'];
const MIME = { '.html': 'text/html', '.css': 'text/css', '.js': 'text/javascript',
               '.svg': 'image/svg+xml', '.json': 'application/json', '.png': 'image/png' };

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

let TARGETS = (await readdir(SRC))
  .filter(f => f.endsWith('.html'))
  .sort()
  .map(f => [f.replace(/\.html$/, ''), `/docs/design/openq/${f}`]);
if (FILTER) TARGETS = TARGETS.filter(([n]) => n.includes(FILTER));
if (!TARGETS.length) { console.error('no targets found'); process.exit(1); }
await mkdir(OUT, { recursive: true });

// ---- the as-rendered contrast probe, run inside the page --------------------
const PROBE = () => {
  const parse = c => {
    const m = c.match(/[\d.]+/g);
    if (!m) return null;
    return [+m[0], +m[1], +m[2], m[3] === undefined ? 1 : +m[3]];
  };
  const over = (fg, bg) => fg.slice(0, 3).map((c, i) => c * fg[3] + bg[i] * (1 - fg[3]));
  const chan = c => { c /= 255; return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4; };
  const lum = ([r, g, b]) => 0.2126 * chan(r) + 0.7152 * chan(g) + 0.0722 * chan(b);
  const ratio = (a, b) => {
    const [x, y] = [lum(a), lum(b)];
    return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
  };
  const effectiveBg = el => {
    const stack = [];
    for (let n = el; n; n = n.parentElement) {
      const c = parse(getComputedStyle(n).backgroundColor);
      if (c && c[3] > 0) { stack.push(c); if (c[3] === 1) break; }
    }
    let base = stack.length && stack[stack.length - 1][3] === 1
      ? stack.pop().slice(0, 3) : [255, 255, 255];
    while (stack.length) base = over(stack.pop(), base);
    return base;
  };
  return [...document.querySelectorAll('[data-contrast]')].map(el => {
    const fg = parse(getComputedStyle(el).color);
    const bg = effectiveBg(el);
    return {
      what: el.className || el.tagName.toLowerCase(),
      text: el.textContent.trim().slice(0, 34),
      size: getComputedStyle(el).fontSize,
      ratio: +ratio(over(fg, bg), bg).toFixed(2),
    };
  });
};

const browser = await chromium.launch();
const gate = [];
const hashes = {};
const contrast = [];
const heights = {};

for (const [name, path] of TARGETS) {
  for (const [vp, [width, height]] of Object.entries(VIEWPORTS)) {
    for (const colorScheme of SCHEMES) {
      const ctx = await browser.newContext({
        viewport: { width, height }, colorScheme, deviceScaleFactor: 2, reducedMotion: 'reduce',
      });
      const page = await ctx.newPage();
      const errs = [];
      page.on('console', m => m.type() === 'error' && errs.push(`console: ${m.text()}`));
      page.on('pageerror', e => errs.push(`pageerror: ${e.message}`));
      page.on('requestfailed', r => errs.push(`FAILED ${r.url()}`));

      await page.goto(BASE + path, { waitUntil: 'load' });
      await page.evaluate(() => document.fonts.ready);
      await page.waitForTimeout(400);

      const h1 = await page.evaluate(() => document.body.scrollHeight);
      const file = join(OUT, `${name}-${vp}-${colorScheme}.png`);
      await page.screenshot({ path: file, fullPage: true });
      await page.waitForTimeout(1100);
      const h2 = await page.evaluate(() => document.body.scrollHeight);
      if (Math.abs(h2 - h1) > 4) errs.push(`layout shift ${h1}px -> ${h2}px after load`);

      const ink = await page.evaluate(() => document.body.innerText.trim().length);
      if (ink < 40) errs.push(`near-blank capture (${ink} chars of text)`);

      if (vp === 'wide') {
        for (const r of await page.evaluate(PROBE)) contrast.push({ name, colorScheme, ...r });
      }

      // full-page height, recorded: the long-content question is partly a
      // question about how much page there is.
      if (vp === 'laptop' && colorScheme === 'light') {
        heights[name] = h2;
        console.log(`  ${name.padEnd(22)} laptop page height ${h2}px  (${(h2 / height).toFixed(1)} screens)`);
      }

      hashes[`${name}-${vp}-${colorScheme}`] =
        createHash('sha1').update(await readFile(file)).digest('hex');
      if (errs.length) gate.push({ name, vp, colorScheme, errs });
      await ctx.close();
    }
  }
}
await browser.close();
server.close();

for (const [name] of TARGETS) {
  for (const vp of Object.keys(VIEWPORTS)) {
    const l = hashes[`${name}-${vp}-light`], d = hashes[`${name}-${vp}-dark`];
    if (l && d && l === d) {
      gate.push({ name, vp, colorScheme: 'both', errs: ['light/dark pair is byte-identical — dark scheme is fake'] });
    }
  }
}

const FLOOR = 4.5;
const bad = contrast.filter(c => c.ratio < FLOOR);
for (const c of bad) {
  gate.push({ name: c.name, vp: 'wide', colorScheme: c.colorScheme,
    errs: [`as-rendered contrast ${c.ratio}:1 on .${c.what} ("${c.text}") — floor ${FLOOR}`] });
}

// ---- contact sheet ----------------------------------------------------------
const shots = (await readdir(OUT))
  .filter(f => f.endsWith('.png') && !f.startsWith('sheet-') && !f.startsWith('detail-')).sort();
const byTarget = {};
for (const f of shots) {
  const m = f.match(/^(.+)-(mobile|laptop|wide)-(light|dark)\.png$/);
  if (m) (byTarget[m[1]] ??= []).push({ f, vp: m[2], scheme: m[3] });
}
const order = { mobile: 0, laptop: 1, wide: 2 };
await writeFile(join(OUT, 'index.html'), `<!doctype html>
<meta charset="utf-8"><title>dira open-question studies</title>
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
<h1>dira open-question studies — long content &amp; the withheld state</h1>
${Object.entries(byTarget).map(([t, list]) => `<h2>${t}</h2><div class="row">${
  list.sort((a,b) => (order[a.vp]-order[b.vp]) || a.scheme.localeCompare(b.scheme))
      .map(s => `<figure><img src="${s.f}" alt="${t} ${s.vp} ${s.scheme}"><figcaption>${s.vp} · ${s.scheme}</figcaption></figure>`).join('')
}</div>`).join('\n')}
`);

await writeFile(join(OUT, 'heights.json'), JSON.stringify(heights, null, 2));

console.log(`\ncaptured ${shots.length} shots for ${TARGETS.length} target(s) -> docs/design/openq/renders/index.html`);
if (contrast.length) {
  console.log(`\nas-rendered contrast — ${contrast.length} probes across both schemes:`);
  const worst = {};
  for (const c of contrast) {
    const k = `${c.name} .${c.what} ${c.colorScheme}`;
    if (!worst[k] || c.ratio < worst[k]) worst[k] = c.ratio;
  }
  for (const [k, v] of Object.entries(worst).sort((a, b) => a[1] - b[1])) {
    console.log(`  ${v.toFixed(2).padStart(6)}:1  ${k}${v < FLOOR ? '  FAIL' : ''}`);
  }
}
if (gate.length) {
  console.log(`\nGATE FAIL (${gate.length}):`);
  for (const g of gate) console.log(`  ${g.name} ${g.vp} ${g.colorScheme}\n    - ${g.errs.join('\n    - ')}`);
  process.exitCode = 1;
} else {
  console.log('GATE PASS — no console errors, no failed requests, no blank mounts, no fake dark, no layout shift, no sub-4.5:1 as-rendered pair.');
}
