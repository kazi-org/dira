// pixeldiff.mjs — the pixel half of the fidelity gate.
//
//   node docs/design/scripts/pixeldiff.mjs <a.png> <b.png> [options]
//   node docs/design/scripts/pixeldiff.mjs --self-test
//
// Compares two captures and reports THREE numbers, because one is not enough and
// the reason is the whole design of this gate:
//
//   1. differing pixels at delta 0            — includes every antialiasing wobble
//   2. differing pixels over the channel      — the headline figure the tolerance
//      threshold (default 8/255)                is set against
//   3. the worst 16x16 block                  — the CLUSTERING figure
//
// Why (3) exists. A percentage of the whole frame is area-weighted, and the two
// things it must distinguish have opposite shapes. Legitimate variance
// (antialiasing, subpixel text positioning) is DIFFUSE: a thin scatter of small
// deltas along every glyph edge in the frame. A real regression is CLUSTERED: one
// element moves, one border vanishes, one hue shifts, and a compact region goes
// dense. A tolerance loose enough to absorb diffuse noise across a 2880x5000
// capture is also loose enough to swallow a whole missing card, so a global
// percentage alone would be a gate you could drive a bus through. The worst-block
// figure is scale-free: it does not care how big the page is.
//
// The channel threshold is a stated blind spot rather than a hidden one: a change
// smaller than the threshold on every channel is invisible here BY DESIGN. Token
// drift of that size is caught by contrast.mjs and tokens-doc-sync.mjs, which read
// the hex values directly. Do not widen the threshold to paper over a token gate.
//
// Options:
//   --tolerance <pct>   max differing-pixel % over the channel threshold
//   --channel <0-255>   per-channel delta that counts as a difference
//   --block-tolerance <pct>  max share of any one 16x16 block that may differ
//   --out <file.png>    write a diff visualisation (red = over threshold)
//   --json              machine-readable result on stdout
//   --quiet             suppress the human report
//
// Defaults come from docs/design/fidelity/tolerance.json, which is measured
// (docs/design/scripts/measure-tolerance.mjs) rather than asserted. The gate reads
// the same file the document quotes, so the number enforced and the number
// published cannot drift apart.

import { readFileSync, writeFileSync } from 'node:fs';
import { resolve, dirname, basename } from 'node:path';
import { decodePNG, encodePNG } from './lib/png.mjs';

const HERE = dirname(new URL(import.meta.url).pathname);
const TOLERANCE_FILE = resolve(HERE, '../fidelity/tolerance.json');
export const BLOCK = 16;

export function loadTolerance() {
  try {
    const t = JSON.parse(readFileSync(TOLERANCE_FILE, 'utf8'));
    return {
      channel: t.channel_threshold,
      tolerance: t.pixel_tolerance_pct,
      blockTolerance: t.block_tolerance_pct,
      source: 'docs/design/fidelity/tolerance.json',
    };
  } catch {
    // No measured file: refuse to invent one. A default pulled out of the air is
    // exactly the unfalsifiable clause this lane exists to delete.
    return null;
  }
}

/**
 * Compare two decoded RGBA images.
 * Throws on a dimension mismatch — never crops, never scales. Two captures of
 * different sizes are not "mostly the same picture", they are a layout change,
 * and silently cropping one to fit is how a gate reports 0.00% on a broken page.
 */
export function diff(a, b, channel) {
  if (a.width !== b.width || a.height !== b.height) {
    throw new Error(
      `dimension mismatch: ${a.width}x${a.height} vs ${b.width}x${b.height} — ` +
      `refusing to crop or scale. A size change IS the regression.`);
  }
  const { width: w, height: h } = a;
  const mask = new Uint8Array(w * h);
  let anyDelta = 0, overThreshold = 0, maxDelta = 0;
  let minX = w, minY = h, maxX = -1, maxY = -1;

  for (let p = 0, i = 0; p < w * h; p++, i += 4) {
    const dr = Math.abs(a.data[i] - b.data[i]);
    const dg = Math.abs(a.data[i + 1] - b.data[i + 1]);
    const db = Math.abs(a.data[i + 2] - b.data[i + 2]);
    const da = Math.abs(a.data[i + 3] - b.data[i + 3]);
    const d = Math.max(dr, dg, db, da);
    if (d === 0) continue;
    anyDelta++;
    if (d > maxDelta) maxDelta = d;
    if (d > channel) {
      overThreshold++;
      mask[p] = 1;
      const x = p % w, y = (p / w) | 0;
      if (x < minX) minX = x;
      if (x > maxX) maxX = x;
      if (y < minY) minY = y;
      if (y > maxY) maxY = y;
    }
  }

  // worst 16x16 block: the clustering figure
  let worstBlock = 0, worstAt = null;
  if (overThreshold) {
    for (let by = 0; by < h; by += BLOCK) {
      for (let bx = 0; bx < w; bx += BLOCK) {
        const yh = Math.min(BLOCK, h - by), xw = Math.min(BLOCK, w - bx);
        let n = 0;
        for (let y = by; y < by + yh; y++) {
          for (let x = bx; x < bx + xw; x++) n += mask[y * w + x];
        }
        const frac = n / (yh * xw);
        if (frac > worstBlock) { worstBlock = frac; worstAt = [bx, by]; }
      }
    }
  }

  const total = w * h;
  return {
    width: w, height: h, totalPixels: total,
    anyDelta, anyDeltaPct: (anyDelta / total) * 100,
    overThreshold, overThresholdPct: (overThreshold / total) * 100,
    maxDelta,
    worstBlockPct: worstBlock * 100,
    worstBlockAt: worstAt,
    bbox: maxX < 0 ? null : { x: minX, y: minY, w: maxX - minX + 1, h: maxY - minY + 1 },
    mask,
  };
}

/** Red overlay on a dimmed copy of `a`, so a reviewer can see WHERE. */
function visualise(a, mask) {
  const out = Buffer.allocUnsafe(a.data.length);
  for (let p = 0, i = 0; p < a.width * a.height; p++, i += 4) {
    if (mask[p]) {
      out[i] = 255; out[i + 1] = 32; out[i + 2] = 32; out[i + 3] = 255;
    } else {
      out[i] = 255 - ((255 - a.data[i]) >> 2);
      out[i + 1] = 255 - ((255 - a.data[i + 1]) >> 2);
      out[i + 2] = 255 - ((255 - a.data[i + 2]) >> 2);
      out[i + 3] = 255;
    }
  }
  return { width: a.width, height: a.height, data: out };
}

// ============================ self-test ======================================
// Two-sided by construction: every assertion below has a known-good AND a
// known-bad case with an arithmetically predictable answer, so a pixeldiff that
// always returned 0.00% would fail here, and so would one that always failed.
function selfTest() {
  const fails = [];
  let asserted = 0;
  const ok = (name, cond, detail = '') => {
    asserted++;
    cond ? console.log(`  ok    ${name}`)
         : (console.log(`  FAIL  ${name}  ${detail}`), fails.push(name));
  };

  const make = (w, h, fill) => {
    const data = Buffer.allocUnsafe(w * h * 4);
    for (let p = 0; p < w * h; p++) {
      const [r, g, b, a] = fill(p % w, (p / w) | 0);
      data[p * 4] = r; data[p * 4 + 1] = g; data[p * 4 + 2] = b; data[p * 4 + 3] = a;
    }
    return { width: w, height: h, data };
  };

  // 160x160 = 25 600 px, an exact 10x10 grid of 16x16 blocks. Whole blocks
  // matter: a partial edge block has a smaller denominator, so a single stray
  // pixel in the corner scores 6.25% instead of 0.39% and the clustering figure
  // stops meaning what it says. (Found by this self-test on its first run,
  // which is the argument for having written it.)
  const W = 160, H = 160;                        // 1 px = 0.0039 %
  const base = make(W, H, (x, y) => [((x * 7 + y * 13) % 256), 120, 200, 255]);
  // +128 (mod 256) is the one perturbation whose per-channel delta is 128 for
  // EVERY starting value. XOR 0xff looks equivalent and is not: at value 127 it
  // produces a delta of 1, which the channel threshold correctly filters, so a
  // "solid" patch silently came out 97.66% full instead of 100%.
  const bump = (img, x, y) => { img.data[(y * W + x) * 4] = (img.data[(y * W + x) * 4] + 128) & 0xff; };

  // 1. identity
  const d0 = diff(base, base, 8);
  ok('identical images report 0 differing pixels',
     d0.anyDelta === 0 && d0.overThresholdPct === 0 && d0.worstBlockPct === 0,
     JSON.stringify({ any: d0.anyDelta, pct: d0.overThresholdPct }));

  // 2. a PNG round trip must be lossless, or every other number here is fiction
  const rt = decodePNG(encodePNG(base), '<roundtrip>');
  ok('encode->decode round trip is lossless', diff(base, rt, 0).anyDelta === 0);

  // 3. a KNOWN count of pixels changed by a KNOWN delta
  const nudged = { ...base, data: Buffer.from(base.data) };
  for (let p = 0; p < 250; p++) nudged.data[p * 4 + 1] = (nudged.data[p * 4 + 1] + 40) & 0xff;
  const d1 = diff(base, nudged, 8);
  ok('250 of 25 600 pixels over threshold reports exactly 0.9766 %',
     d1.overThreshold === 250 && Math.abs(d1.overThresholdPct - (250 / 25600) * 100) < 1e-9,
     `got ${d1.overThreshold} / ${d1.overThresholdPct}`);
  ok('max per-channel delta is reported exactly', d1.maxDelta === 40, `got ${d1.maxDelta}`);

  // 4. the channel threshold actually filters — the negative control for (3)
  const whisper = { ...base, data: Buffer.from(base.data) };
  for (let p = 0; p < 250; p++) whisper.data[p * 4 + 1] = (whisper.data[p * 4 + 1] + 5) & 0xff;
  const d2 = diff(base, whisper, 8);
  ok('a 5/255 delta is seen at delta 0 and filtered at threshold 8',
     d2.anyDelta === 250 && d2.overThreshold === 0,
     `any=${d2.anyDelta} over=${d2.overThreshold}`);
  ok('the same 5/255 delta IS counted when the threshold is 0',
     diff(base, whisper, 0).overThreshold === 250);

  // 5. clustering — the assertion the whole two-metric design rests on.
  //    EXACTLY 100 changed pixels each way. Same global percentage, opposite
  //    shape: one pixel in each of the hundred blocks, versus one solid 10x10
  //    patch inside a single block. If the global figure alone were the gate,
  //    these two would be indistinguishable — which is the point.
  const diffuse = { ...base, data: Buffer.from(base.data) };
  for (let by = 0; by < 10; by++) for (let bx = 0; bx < 10; bx++) bump(diffuse, bx * 16 + 3, by * 16 + 3);
  const clustered = { ...base, data: Buffer.from(base.data) };
  for (let y = 32; y < 42; y++) for (let x = 32; x < 42; x++) bump(clustered, x, y);
  const solid = { ...base, data: Buffer.from(base.data) };
  for (let y = 32; y < 48; y++) for (let x = 32; x < 48; x++) bump(solid, x, y);
  const dd = diff(base, diffuse, 8), dc = diff(base, clustered, 8), ds = diff(base, solid, 8);
  ok('both perturbations change exactly 100 pixels',
     dd.overThreshold === 100 && dc.overThreshold === 100,
     `diffuse ${dd.overThreshold} clustered ${dc.overThreshold}`);
  ok('a diffuse scatter keeps the worst block at one pixel (0.39 %)',
     Math.abs(dd.worstBlockPct - (1 / 256) * 100) < 1e-9, `worst block ${dd.worstBlockPct.toFixed(2)}%`);
  ok('a solid 16x16 patch drives the worst block to 100 %',
     ds.worstBlockPct === 100, `worst block ${ds.worstBlockPct.toFixed(2)}%`);
  ok('clustering separates the two at IDENTICAL global percentages',
     dd.overThresholdPct === dc.overThresholdPct && dc.worstBlockPct > dd.worstBlockPct * 25,
     `global ${dd.overThresholdPct.toFixed(4)}% vs ${dc.overThresholdPct.toFixed(4)}%; ` +
     `worst block ${dd.worstBlockPct.toFixed(2)}% vs ${dc.worstBlockPct.toFixed(2)}%`);

  // 6. bounding box names WHERE
  ok('the bounding box locates the change',
     ds.bbox && ds.bbox.x === 32 && ds.bbox.y === 32 && ds.bbox.w === 16 && ds.bbox.h === 16,
     JSON.stringify(ds.bbox));

  // 7. dimension mismatch fails loudly rather than cropping
  let threw = false;
  try { diff(base, make(W, H - 1, () => [0, 0, 0, 255]), 8); } catch { threw = true; }
  ok('differing dimensions throw instead of cropping', threw);

  // 8. a malformed PNG is rejected, not decoded into noise
  let threwPng = false;
  try { decodePNG(Buffer.from('not a png at all'), '<junk>'); } catch { threwPng = true; }
  ok('a non-PNG buffer is rejected', threwPng);

  console.log(fails.length
    ? `\nSELF-TEST FAIL — ${fails.length} of ${asserted}: ${fails.join(', ')}`
    : `\nSELF-TEST PASS — ${asserted} assertions, each with a known-bad counterpart.`);
  process.exit(fails.length ? 1 : 0);
}

// ============================ cli ============================================
// Only when run directly. measure-tolerance.mjs imports `diff` from here, and a
// module that calls process.exit() on import is a module nobody can reuse.
const IS_MAIN = process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname);
const argv = process.argv.slice(2);
if (!IS_MAIN) { /* imported as a library — export only */ }
else if (argv.includes('--self-test')) selfTest();
else main();

function main() {

const flag = (name, fallback) => {
  const i = argv.indexOf(name);
  return i >= 0 && argv[i + 1] !== undefined ? argv[i + 1] : fallback;
};
const positional = argv.filter((a, i) =>
  !a.startsWith('--') && !['--tolerance', '--channel', '--block-tolerance', '--out'].includes(argv[i - 1]));

if (positional.length !== 2) {
  console.error('usage: node docs/design/scripts/pixeldiff.mjs <a.png> <b.png> [--tolerance pct]' +
                ' [--channel n] [--block-tolerance pct] [--out diff.png] [--json] [--quiet]\n' +
                '       node docs/design/scripts/pixeldiff.mjs --self-test');
  process.exit(2);
}

const measured = loadTolerance();
const CHANNEL = Number(flag('--channel', measured?.channel));
const TOL = Number(flag('--tolerance', measured?.tolerance));
const BLOCK_TOL = Number(flag('--block-tolerance', measured?.blockTolerance));
const JSON_OUT = argv.includes('--json');
const QUIET = argv.includes('--quiet');

if (![CHANNEL, TOL, BLOCK_TOL].every(Number.isFinite)) {
  console.error('pixeldiff: no tolerance available. Either pass --tolerance/--channel/--block-tolerance,\n' +
    '  or generate the measured one: node docs/design/scripts/measure-tolerance.mjs\n' +
    '  (there is deliberately no built-in default — an unmeasured tolerance is the\n' +
    '   unfalsifiable clause this gate exists to replace).');
  process.exit(2);
}

const [pathA, pathB] = positional;
let a, b, r;
try {
  a = decodePNG(readFileSync(resolve(pathA)), basename(pathA));
  b = decodePNG(readFileSync(resolve(pathB)), basename(pathB));
  r = diff(a, b, CHANNEL);
} catch (e) {
  if (JSON_OUT) console.log(JSON.stringify({ ok: false, error: e.message }));
  else console.error(`PIXELDIFF FAIL — ${e.message}`);
  process.exit(2);
}

const outFile = flag('--out', null);
if (outFile) writeFileSync(resolve(outFile), encodePNG(visualise(a, r.mask)));

const overTol = r.overThresholdPct > TOL;
const overBlock = r.worstBlockPct > BLOCK_TOL;
const pass = !overTol && !overBlock;

if (JSON_OUT) {
  const { mask, ...rest } = r;
  console.log(JSON.stringify({ ok: pass, a: pathA, b: pathB, channel: CHANNEL, tolerance: TOL, blockTolerance: BLOCK_TOL, ...rest }));
} else if (!QUIET) {
  console.log(`\n${basename(pathA)}  vs  ${basename(pathB)}   ${r.width}x${r.height} device px`);
  console.log(`  differing, any delta        ${String(r.anyDelta).padStart(10)}   ${r.anyDeltaPct.toFixed(4)}%`);
  console.log(`  differing, delta > ${String(CHANNEL).padStart(3)}/255  ${String(r.overThreshold).padStart(10)}   ${r.overThresholdPct.toFixed(4)}%   (tolerance ${TOL}%)`);
  console.log(`  max per-channel delta       ${String(r.maxDelta).padStart(10)}`);
  console.log(`  worst ${BLOCK}x${BLOCK} block             ${r.worstBlockPct.toFixed(2).padStart(9)}%   (tolerance ${BLOCK_TOL}%)`
    + (r.worstBlockAt ? `  at ${r.worstBlockAt[0]},${r.worstBlockAt[1]}` : ''));
  if (r.bbox) console.log(`  changed region              ${r.bbox.w}x${r.bbox.h} at ${r.bbox.x},${r.bbox.y}`);
  if (outFile) console.log(`  diff image                  ${outFile}`);
  if (pass) {
    console.log('  PIXELDIFF PASS');
  } else {
    console.log('  PIXELDIFF FAIL — ' + [
      overTol && `${r.overThresholdPct.toFixed(4)}% of pixels differ, over the ${TOL}% tolerance`,
      overBlock && `a ${BLOCK}x${BLOCK} block at ${r.worstBlockAt?.[0]},${r.worstBlockAt?.[1]} is ${r.worstBlockPct.toFixed(1)}% changed, over the ${BLOCK_TOL}% block tolerance — that is clustered, not antialiasing`,
    ].filter(Boolean).join('; '));
  }
}
process.exit(pass ? 0 : 1);
}
