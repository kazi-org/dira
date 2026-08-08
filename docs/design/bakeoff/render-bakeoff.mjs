// Bake-off capture + measurement. Follows the harness pattern in
// docs/design/scripts/render.mjs (same static server, same viewport matrix, same
// mechanical gate) but writes into docs/design/bakeoff/renders/ and adds the one
// thing this decision actually turns on: measured characters-per-line.
//
//   node docs/design/bakeoff/render-bakeoff.mjs
//
// Why measurement and not judgement: every --m-* measure token in tokens.css is
// expressed in `ch`, and `ch` is the advance width of "0" IN THE RESOLVED FONT.
// Swapping the serif therefore moves the pixel width of every prose block AND the
// number of characters that fit in it, by different amounts. That is the ratio
// re-tuning question, and it is arithmetic, not taste.

import { chromium } from 'playwright';
import { createServer } from 'node:http';
import { readFile, mkdir, writeFile, readdir } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { extname, join, resolve } from 'node:path';

const HERE = import.meta.dirname;
const ROOT = resolve(HERE, '../../..');
const OUT = join(HERE, 'renders');
const VIEWPORTS = { mobile: [390, 844], laptop: [1024, 768], wide: [1440, 900] };
const SCHEMES = ['light', 'dark'];
const MIME = { '.html': 'text/html', '.css': 'text/css', '.js': 'text/javascript',
               '.woff2': 'font/woff2', '.png': 'image/png', '.json': 'application/json' };

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

const TARGETS = (await readdir(HERE))
  .filter(f => f.startsWith('s1-') && f.endsWith('.html')).sort()
  .map(f => [f.replace(/^s1-|\.html$/g, ''), `/docs/design/bakeoff/${f}`]);
await mkdir(OUT, { recursive: true });

// ---- the measurement, run inside the page ----------------------------------
const MEASURE = () => {
  // split a block into its real rendered lines by grouping client rects by top
  const lines = (el) => {
    const out = [];
    const walk = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
    let node;
    while ((node = walk.nextNode())) {
      const t = node.textContent;
      for (let i = 0; i < t.length; i++) {
        const r = document.createRange();
        r.setStart(node, i); r.setEnd(node, i + 1);
        const b = r.getBoundingClientRect();
        if (!b.width && !b.height) continue;
        const key = Math.round(b.top);
        const last = out[out.length - 1];
        if (last && Math.abs(last.top - key) <= 2) { last.text += t[i]; last.right = b.right; }
        else out.push({ top: key, text: t[i], left: b.left, right: b.right });
      }
    }
    return out.map(l => ({ ...l, text: l.text.replace(/\s+$/, '') }));
  };

  const stat = (sel, nth = 0) => {
    const el = document.querySelectorAll(sel)[nth];
    if (!el) return null;
    const cs = getComputedStyle(el);
    const ls = lines(el).filter(l => l.text.trim().length);
    const lens = ls.map(l => l.text.length);
    // ignore the last line: it is short by definition and drags the mean down
    const full = lens.slice(0, -1);
    return {
      selector: sel + (nth ? `[${nth}]` : ''),
      fontFamily: cs.fontFamily,
      fontSizePx: parseFloat(cs.fontSize),
      lineHeightPx: parseFloat(cs.lineHeight),
      maxWidthComputedPx: Math.round(parseFloat(cs.maxWidth) * 100) / 100,
      renderedWidthPx: Math.round(el.getBoundingClientRect().width * 100) / 100,
      renderedHeightPx: Math.round(el.getBoundingClientRect().height * 100) / 100,
      lineCount: ls.length,
      charsPerLine_mean: full.length ? Math.round(full.reduce((a, b) => a + b, 0) / full.length * 10) / 10 : null,
      charsPerLine_max: lens.length ? Math.max(...lens) : null,
      charsPerLine_perLine: lens,
    };
  };

  // resolved `ch` and `ex` for the serif -- the unit every measure token is in
  const probe = document.createElement('span');
  probe.style.cssText = 'position:absolute;visibility:hidden;font-family:var(--serif);font-size:100px';
  probe.textContent = '0';
  document.body.appendChild(probe);
  const chPx = probe.getBoundingClientRect().width / 100;
  const box = document.createElement('div');
  box.style.cssText = 'position:absolute;visibility:hidden;font-family:var(--serif);font-size:100px;width:1ex;height:1ex';
  document.body.appendChild(box);
  const exPx = box.getBoundingClientRect().height / 100;
  probe.remove(); box.remove();

  return {
    candidate: document.body.dataset.candidate,
    serifToken: getComputedStyle(document.documentElement).getPropertyValue('--serif').trim(),
    ch_em: Math.round(chPx * 10000) / 10000,   // advance of "0" as a fraction of em
    ex_em: Math.round(exPx * 10000) / 10000,   // x-height as a fraction of em
    blocks: {
      ruling:       stat('h1.ruling'),
      because:      stat('p.because'),
      grounds_long: stat('.arg .grounds', 1),   // the Elixir/OTP paragraph, the longest
      grounds_first: stat('.arg .grounds', 0),
    },
    pageHeight: document.body.scrollHeight,
  };
};

// ---- capture ---------------------------------------------------------------
const browser = await chromium.launch();
const gate = [], hashes = {}, measurements = {};

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

      // the font actually has to have loaded, or every number below is a lie
      // about the fallback rather than a measurement of the candidate
      const loaded = await page.evaluate(() =>
        [...document.fonts].map(f => `${f.family}|${f.weight}|${f.style}|${f.status}`));
      const unloaded = loaded.filter(f => f.startsWith('BakeoffSerif') && !f.endsWith('loaded'));
      if (name !== 'control-palatino' && !loaded.some(f => f.startsWith('BakeoffSerif')))
        errs.push('no BakeoffSerif face registered — page is rendering a fallback');
      if (unloaded.length) errs.push(`face(s) never loaded: ${unloaded.join(', ')}`);

      const h1 = await page.evaluate(() => document.body.scrollHeight);
      const file = join(OUT, `${name}-${vp}-${colorScheme}.png`);
      await page.screenshot({ path: file, fullPage: true });
      await page.waitForTimeout(1100);
      const h2 = await page.evaluate(() => document.body.scrollHeight);
      if (Math.abs(h2 - h1) > 4) errs.push(`layout shift ${h1}px -> ${h2}px after load`);

      const ink = await page.evaluate(() => document.body.innerText.trim().length);
      if (ink < 40) errs.push(`near-blank capture (${ink} chars of text)`);

      if (vp === 'laptop' && colorScheme === 'light') {
        measurements[name] = await page.evaluate(MEASURE);
        measurements[name].facesLoaded = loaded;
      }

      hashes[`${name}-${vp}-${colorScheme}`] =
        createHash('sha1').update(await readFile(file)).digest('hex');
      if (errs.length) gate.push({ name, vp, colorScheme, errs });
      await ctx.close();
    }
  }
}

// ---- fake-dark check + the bake-off's own gate ------------------------------
for (const [name] of TARGETS) {
  for (const vp of Object.keys(VIEWPORTS)) {
    const l = hashes[`${name}-${vp}-light`], d = hashes[`${name}-${vp}-dark`];
    if (l && d && l === d)
      gate.push({ name, vp, colorScheme: 'both', errs: ['light/dark pair byte-identical — dark scheme is fake'] });
  }
}
// two candidates rendering byte-identically means one of them silently fell back
for (const vp of Object.keys(VIEWPORTS)) {
  const seen = {};
  for (const [name] of TARGETS) {
    const h = hashes[`${name}-${vp}-light`];
    if (seen[h]) gate.push({ name, vp, colorScheme: 'light',
      errs: [`byte-identical to ${seen[h]} — one of these is not rendering its own face`] });
    seen[h] = name;
  }
}

await browser.close();
server.close();

await writeFile(join(HERE, 'measurements.json'), JSON.stringify(measurements, null, 1));

const shots = (await readdir(OUT)).filter(f => f.endsWith('.png')).sort();
console.log(`\ncaptured ${shots.length} shots for ${TARGETS.length} target(s)`);
console.log(`measurements -> docs/design/bakeoff/measurements.json`);
if (gate.length) {
  console.log(`\nGATE FAIL (${gate.length}):`);
  for (const g of gate) console.log(`  ${g.name} ${g.vp} ${g.colorScheme}\n    - ${g.errs.join('\n    - ')}`);
  process.exitCode = 1;
} else {
  console.log('GATE PASS — no console errors, no failed requests, no blank mounts, no fake dark, no layout shift, every face really loaded, no two candidates identical.');
}
