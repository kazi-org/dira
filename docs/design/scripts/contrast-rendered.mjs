// contrast-rendered.mjs — measures contrast AS DRAWN, not as declared.
//
// contrast.mjs parses tokens.css and checks token against token. That is
// necessary and it is not sufficient: every chip in this system sits on a
// `color-mix(in oklab, <its own hue> N%, transparent)` tint OF ITS OWN COLOUR,
// so the pair that ships is fg-on-tint, not fg-on-surface. The token matrix
// structurally cannot see that pair, and reported "0 failures" while five chips
// were rendering between 3.0:1 and 4.3:1 in the light scheme.
//
// Two further reasons the browser is the only authority here:
//   - `color-mix(in oklab, ...)` cannot be approximated in sRGB. An sRGB
//     interpolation of the same declaration reported PASSES for pairs the
//     browser renders as failures.
//   - transparency composites up the ancestor chain, so the effective background
//     depends on where an element sits, not only on what it declares.
//
//   node docs/design/scripts/contrast-rendered.mjs [-v]
//
// Exits non-zero on any text under 4.5:1 as composited.

import { chromium } from 'playwright';
import { readdir } from 'node:fs/promises';
import { resolve, dirname } from 'node:path';

const HERE = dirname(new URL(import.meta.url).pathname);
const ROOT = resolve(HERE, '../../..');
const VERBOSE = process.argv.includes('-v');
const FLOOR = 4.5;

const targets = [];
for (const [dir, prefix] of [['docs/design/screens', ''], ['docs/design/landing', '']]) {
  try {
    for (const f of (await readdir(resolve(ROOT, dir)))) {
      if (f.endsWith('.html')) targets.push(`${dir}/${f}`);
    }
  } catch {}
}

const probe = () => {
  const lin = c => { c /= 255; return c <= .03928 ? c / 12.92 : Math.pow((c + .055) / 1.055, 2.4); };
  const L = ([r, g, b]) => .2126 * lin(r) + .7152 * lin(g) + .0722 * lin(b);
  // Resolve ANY CSS colour to sRGB by painting it. getComputedStyle returns
  // color-mix() results as oklab(...) in Chromium, and naively reading those
  // three floats as 0-255 RGB produces garbage - it reported the drift card at
  // 1.31:1 when the text is plain ink at ~13:1. Painting is the only parser that
  // is correct for every colour space the CSS may use.
  const _cv = document.createElement('canvas'); _cv.width = _cv.height = 1;
  const _cx = _cv.getContext('2d', { willReadFrequently: true });
  const parse = str => {
    if (!str || str === 'transparent') return [0, 0, 0, 0];
    _cx.clearRect(0, 0, 1, 1);
    _cx.fillStyle = '#000';
    _cx.fillStyle = str;              // invalid values leave the previous fill
    _cx.clearRect(0, 0, 1, 1);
    _cx.fillRect(0, 0, 1, 1);
    const d = _cx.getImageData(0, 0, 1, 1).data;
    return [d[0], d[1], d[2], d[3] / 255];
  };
  // composite semi-transparent backgrounds up the ancestor chain
  const effectiveBg = el => {
    let acc = null, n = el;
    while (n) {
      const bg = parse(getComputedStyle(n).backgroundColor);
      const a = bg.length > 3 ? bg[3] : 1;
      if (a > 0) {
        const c = [bg[0], bg[1], bg[2]];
        if (acc === null) acc = [c, a];
        else {
          const [pc, pa] = acc, na = pa + a * (1 - pa);
          acc = [[0,1,2].map(i => (pc[i]*pa + c[i]*a*(1-pa)) / na), na];
        }
        if (acc[1] >= 0.999) break;
      }
      n = n.parentElement;
    }
    return acc ? acc[0] : [255, 255, 255];
  };
  const out = [];
  document.querySelectorAll('*').forEach(el => {
    if (!el.childNodes.length) return;
    // only elements that directly own visible text
    const text = [...el.childNodes].filter(n => n.nodeType === 3 && n.textContent.trim()).length;
    if (!text) return;
    const cs = getComputedStyle(el);
    if (cs.visibility === 'hidden' || cs.display === 'none' || +cs.opacity === 0) return;
    // decorative content is exempt: it carries no information, so there is
    // nothing for a reader to fail to read. It must be MARKED decorative,
    // not merely assumed to be.
    if (el.closest('[aria-hidden="true"]')) return;
    const fg = parse(cs.color).slice(0, 3);
    const bg = effectiveBg(el);
    const l1 = L(fg), l2 = L(bg);
    const ratio = (Math.max(l1, l2) + .05) / (Math.min(l1, l2) + .05);
    out.push({
      sel: el.tagName.toLowerCase() + (el.className && typeof el.className === 'string' ? '.' + el.className.trim().split(/\s+/).join('.') : ''),
      size: parseFloat(cs.fontSize),
      weight: cs.fontWeight,
      ratio: +ratio.toFixed(2),
      sample: el.textContent.trim().slice(0, 32),
    });
  });
  return out;
};

const browser = await chromium.launch();
const failures = [];
let checked = 0;

for (const t of targets) {
  for (const colorScheme of ['light', 'dark']) {
    const ctx = await browser.newContext({ colorScheme, viewport: { width: 1024, height: 768 } });
    const page = await ctx.newPage();
    await page.goto('file://' + resolve(ROOT, t));
    await page.evaluate(() => document.fonts.ready);
    await page.waitForTimeout(250);
    for (const r of await page.evaluate(probe)) {
      checked++;
      // WCAG large-text exemption: >=24px, or >=18.66px at >=700 weight
      const large = r.size >= 24 || (r.size >= 18.66 && +r.weight >= 700);
      const floor = large ? 3.0 : FLOOR;
      if (r.ratio < floor) failures.push({ ...r, t, colorScheme, floor });
      else if (VERBOSE) console.log(`  ok   ${colorScheme} ${r.sel} ${r.ratio}`);
    }
    await ctx.close();
  }
}
await browser.close();

console.log(`\n${checked} text nodes measured as composited across ${targets.length} surfaces x 2 schemes.`);
if (failures.length) {
  console.log(`\nRENDERED CONTRAST FAIL — ${failures.length}:`);
  for (const f of failures)
    console.log(`  ${f.t.split('/').pop()} ${f.colorScheme}  ${f.sel}  ${f.size}px  ${f.ratio}:1 (floor ${f.floor})  "${f.sample}"`);
  process.exit(1);
}
console.log('RENDERED CONTRAST PASS — every text node clears its floor as actually drawn.');
