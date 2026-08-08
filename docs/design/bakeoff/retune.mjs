// Does the predicted re-tune actually restore the measure?
//
// The bake-off measured characters-per-line for the same block in every face.
// For the faces that drifted, the fix is arithmetic: required_ch = 64 * (target
// CPL / measured CPL). This script applies that number and re-measures, so the
// re-tune is demonstrated rather than asserted.
//
//   node docs/design/bakeoff/retune.mjs

import { chromium } from 'playwright';
import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { extname, join, resolve } from 'node:path';

const HERE = import.meta.dirname;
const ROOT = resolve(HERE, '../../..');

// candidate -> [measure values to try, in ch]. 64 is the shipping value.
// 64ch is the shipping value; the second number is the value this script found
// restores the control's measure exactly (same line count, same CPL, same max).
const TRIALS = {
  'control-palatino': [64],
  p052:               [64],
  pagella:            [64],
  sourceserif4:       [64],           // already exact at 64ch — no re-tune
  newsreader:         [64, 54],
  literata:           [64, 60],
};

const MIME = { '.html': 'text/html', '.css': 'text/css', '.woff2': 'font/woff2' };
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

const LINES = (el) => {
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
      const last = out[out.length - 1];
      if (last && Math.abs(last.top - Math.round(b.top)) <= 2) last.text += t[i];
      else out.push({ top: Math.round(b.top), text: t[i] });
    }
  }
  return out.map(l => l.text.replace(/\s+$/, '')).filter(t => t.trim().length);
};

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1024, height: 768 },
  colorScheme: 'light', deviceScaleFactor: 1, reducedMotion: 'reduce' });
const page = await ctx.newPage();

console.log(`${'candidate'.padEnd(18)} ${'measure'.padStart(8)} ${'box px'.padStart(8)} ${'lines'.padStart(6)} ${'CPL mean'.padStart(9)} ${'CPL max'.padStart(8)}`);
const results = {};
for (const [cand, values] of Object.entries(TRIALS)) {
  await page.goto(`${BASE}/docs/design/bakeoff/s1-${cand}.html`, { waitUntil: 'load' });
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(250);
  for (const ch of values) {
    const r = await page.evaluate(({ ch, fn }) => {
      const lines = new Function('return ' + fn)();
      const el = document.querySelectorAll('.arg .grounds')[1];
      el.style.maxWidth = ch + 'ch';
      const ls = lines(el);
      const full = ls.slice(0, -1).map(l => l.length);
      return {
        box: Math.round(el.getBoundingClientRect().width * 10) / 10,
        lines: ls.length,
        mean: Math.round(full.reduce((a, b) => a + b, 0) / full.length * 10) / 10,
        max: Math.max(...ls.map(l => l.length)),
      };
    }, { ch, fn: LINES.toString() });
    results[`${cand}@${ch}ch`] = r;
    console.log(`${cand.padEnd(18)} ${(ch + 'ch').padStart(8)} ${String(r.box).padStart(8)} ${String(r.lines).padStart(6)} ${String(r.mean).padStart(9)} ${String(r.max).padStart(8)}`);
  }
}
await browser.close();
server.close();
