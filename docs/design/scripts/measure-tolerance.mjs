// measure-tolerance.mjs — derives the pixel tolerance from measurement.
//
//   node docs/design/scripts/measure-tolerance.mjs [--write] [--quick]
//
// docs/plan.md §E6 required fidelity "within the pixel tolerance recorded in
// DESIGN.md" while no tolerance was recorded anywhere. A number invented to close
// that gap would be worse than the gap: an unfalsifiable clause replaced by a
// falsifiable-looking one. So this script measures both sides of the question and
// only then picks a number.
//
// THE METHOD. Two populations, captured under identical conditions:
//
//   NOISE  — the same page, rendered twice, varying only things that MUST NOT
//            count as a regression. Every one of these is a difference E6-L2 will
//            genuinely have between a mockup baseline and a Go-served page.
//   SIGNAL — the same page with one small, deliberate, real defect. These are the
//            smallest regressions the gate is expected to catch. Anything the gate
//            lets through that is smaller than these is a stated blind spot, not
//            an accident.
//
// The tolerance is then placed in the gap between the two distributions, and the
// script FAILS if there is no gap. A tolerance that cannot separate its own noise
// from its own smallest signal is not a tolerance; it is a decoration, and the
// honest output in that case is the overlap, not a number.
//
// Nothing on disk is modified to produce the signal arms. The static server holds
// an in-memory override map, so docs/design/tokens.css and docs/design/screens/
// are read-only throughout — the reference is not edited to make the gate agree
// with it.

import { chromium } from 'playwright';
import { createServer } from 'node:http';
import { readFile, mkdir, writeFile, readdir } from 'node:fs/promises';
import { extname, join, resolve, dirname } from 'node:path';
import { createHash } from 'node:crypto';
import { decodePNG } from './lib/png.mjs';
import { diff } from './pixeldiff.mjs';

const HERE = dirname(new URL(import.meta.url).pathname);
const ROOT = resolve(HERE, '../../..');
const FIDELITY = resolve(HERE, '../fidelity');
const QUICK = process.argv.includes('--quick');
const WRITE = process.argv.includes('--write');

const MIME = { '.html': 'text/html; charset=utf-8', '.css': 'text/css', '.js': 'text/javascript',
               '.svg': 'image/svg+xml', '.json': 'application/json', '.png': 'image/png',
               '.woff2': 'font/woff2' };

const SCREENS = ['s1-decision', 's2-index', 's3-distill'];
const VIEWPORTS = QUICK ? { wide: [1440, 900] } : { mobile: [390, 844], wide: [1440, 900] };
const SCHEMES = QUICK ? ['light'] : ['light', 'dark'];

// ---- static server, with an in-memory override map --------------------------
function serve(overrides = {}) {
  const server = createServer(async (req, res) => {
    const url = decodeURIComponent(req.url.split('?')[0]);
    try {
      if (overrides[url]) {
        res.writeHead(200, { 'content-type': MIME[extname(url)] ?? 'text/plain' });
        return res.end(overrides[url]);
      }
      const p = join(ROOT, url);
      const body = await readFile(p);
      res.writeHead(200, { 'content-type': MIME[extname(p)] ?? 'application/octet-stream' });
      res.end(body);
    } catch { res.writeHead(404); res.end('not found'); }
  });
  return new Promise(r => server.listen(0, '127.0.0.1', () =>
    r({ server, base: `http://127.0.0.1:${server.address().port}` })));
}

// ---- one capture -------------------------------------------------------------
// open() and shoot() are separate so the `same-page` arm can genuinely re-shoot a
// LIVE page rather than reloading it. The first version of this file had them
// fused, which made the "same context" arm silently identical to the "fresh
// context" arm — two arms, one measurement, and a determinism claim resting on a
// duplicate. Worth naming: an arm that measures something other than what it is
// labelled is the same defect class as a gate that checks the declaration.
async function open(browser, base, path, width, height, colorScheme) {
  const ctx = await browser.newContext({
    viewport: { width, height }, colorScheme, deviceScaleFactor: 2, reducedMotion: 'reduce',
  });
  const page = await ctx.newPage();
  await page.goto(base + path, { waitUntil: 'load' });
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(400);
  return { ctx, page };
}
const shoot = async (page, label) => decodePNG(await page.screenshot({ fullPage: true }), label);

async function capture(browser, base, path, width, height, colorScheme, { reserialize = false } = {}) {
  const { ctx, page } = await open(browser, base, path, width, height, colorScheme);
  const img = await shoot(page, path);
  const html = reserialize
    ? await page.evaluate(() => '<!doctype html>\n' + document.documentElement.outerHTML)
    : null;
  await ctx.close();
  return { img, html };
}

// ---- the two populations -----------------------------------------------------
// Every NOISE arm is a real difference E6-L2 will have. Every SIGNAL arm is a
// real defect stated in one line of CSS.
const TOKENS = '/docs/design/tokens.css';

const NOISE = [
  ['same-page',      'the SAME live page screenshotted a second time — no reload, no new context', { samePage: true }],
  ['fresh-context',  'a reload in a new browser context in the same browser process',             {}],
  ['fresh-browser',  'a second chromium process — the CI-vs-laptop case, minus the OS change',   { newBrowser: true }],
  ['other-origin',   'served from a second HTTP server on a different port — a Go binary and the mockup harness are never the same port',
                                                                                                 { otherOrigin: true }],
  ['reserialized',   'markup round-tripped through the DOM serializer — normalized attribute quoting, entity encoding and boolean attributes, which is exactly what html/template output differs from hand-written HTML by (dec-0012)',
                                                                                                 { reserialize: true }],
];

const SIGNAL = [
  ['spacing-1px',  'one spacing step moved by 1px (--s4: 18px -> 19px)',
                   css => css.replace(/--s4:\s*18px/g, '--s4: 19px')],
  ['type-0.5px',   'body type moved by half a pixel (--t-body: 16.5px -> 17px)',
                   css => css.replace(/--t-body:\s*16\.5px/g, '--t-body: 17px')],
  ['hairline',     'the card hairline removed (.card border-color -> transparent)',
                   css => css + '\n.card{border-color:transparent}\n'],
  // The ink swap takes the value from the SAME block, so it is correct in both
  // schemes rather than pasting a light value into the dark palette. This is the
  // subtlest arm on purpose: no geometry moves, only the colour of every glyph.
  ['ink-swap',     'body ink set one step lighter (--ink takes the value of --ink-mid) in both schemes',
                   css => css.replace(/--ink:\s*#[0-9a-fA-F]{3,8}\s*;/g, (m, offset) => {
                     const mid = css.slice(offset, offset + 400).match(/--ink-mid:\s*(#[0-9a-fA-F]{3,8})/);
                     return mid ? `--ink: ${mid[1]};` : m;
                   })],
  // The two below reflow NOTHING. They are the genuine lower bound on AREA: a
  // single component's hue wrong, and a 2px geometry change confined to four
  // corners. If the tolerance cannot see these, that is a blind spot to publish,
  // not to discover later.
  ['chip-hue',     'one component hue wrong with no reflow (.chip-id colour -> --ink-mid)',
                   css => css + '\n.chip-id{color:var(--ink-mid)}\n'],
  ['radius-2px',   'card radius off by 2px (--r-card: 7px -> 5px) — four corners, no reflow',
                   css => css.replace(/--r-card:\s*7px/g, '--r-card: 5px')],

  // ---- the LOW-DELTA arms, and why they had to exist -------------------------
  // Every arm above is loud per pixel (peak deltas 16-255). A signal set made
  // only of loud defects cannot distinguish one channel threshold from another,
  // because all of them survive any threshold — which is exactly how 4/255 came
  // to look admissible on the first pass. These two are quiet per pixel and
  // large in area: the class a channel threshold silently swallows.
  //
  // Both are realistic rather than synthetic. A hex off by two in one channel is
  // an ordinary copy-paste error; a stepped opacity is an ordinary CSS edit. And
  // critically, NO OTHER GATE COVERS THE SECOND: contrast.mjs and
  // tokens-doc-sync.mjs read declared hex values, so a wrong opacity on a rule in
  // the page's own stylesheet is visible to the pixel gate or to nothing.
  ['ink-2',        'one token hex off by 2/255 in a single channel (--ink), both schemes — a copy-paste typo, quiet per pixel and everywhere at once',
                   css => css.replace(/--ink:\s*#23211d/g, '--ink: #23211f')
                             .replace(/--ink:\s*#e9e6df/g, '--ink: #e9e6e1')],
  // !important because the real rule lives in s3-distill's inline <style>, which
  // the browser applies AFTER tokens.css — without it the override loses on
  // source order and the arm measured nothing at all. The defect being simulated
  // is still "this element renders at .57 instead of .58"; the specificity is
  // only how the mutation reaches it from the one file this harness overrides.
  ['opacity-1pct', 'a stepped opacity off by one hundredth (.stage.next: .58 -> .57) — quiet per pixel over a large area, and covered by no other gate',
                   css => css + '\n.stage.next{opacity:.57 !important}\n'],
];

// INFO arms gate nothing. They quantify the thing the protocol forbids rather
// than tolerances: comparing captures made in different environments. DESIGN.md
// already records that this harness runs on macOS and structurally cannot see a
// Linux serif fallback. These two put a number on that, which is the evidence for
// why E6-L2 must regenerate its baseline in the same run as its capture instead
// of widening the tolerance until a cross-machine comparison fits under it.
const INFO = [
  ['rasterization',  'glyph rasterization changed with no layout change (-webkit-font-smoothing: auto) — a stand-in for a different machine\'s text renderer',
                     css => css + '\nbody{-webkit-font-smoothing:auto}\n'],
  ['serif-fallback', 'the Palatino stack falls through to the generic serif, which is what a stock Linux install actually renders (DESIGN.md, "where that reasoning currently FAILS")',
                     css => css.replace(/--serif:[^;]+;/, '--serif: serif;')],
];

// ---- run ---------------------------------------------------------------------
const rows = [];
const primary = await serve();
const secondary = await serve();
const browserA = await chromium.launch();
const browserB = await chromium.launch();

// Every pair is measured at all three candidate channel thresholds, so the one
// that ships is SELECTED from the data rather than picked and then justified.
// 0 hides nothing; the larger values are candidates only if they still leave
// every signal arm comfortably visible.
const CHANNEL_PROBES = [0, 4, 8, 16];

// ---- input fingerprint -------------------------------------------------------
// This tree has several agents in it, and the screens ARE the reference. A run
// that captures half its baselines before somebody edits a mockup and half after
// produces evidence that looks clean and is not comparable. Hash every input at
// the start and again at the end; if anything moved, the measurement is void.
// (Not hypothetical: a concurrent edit to s1-decision.html landed during the
// previous run of this script.)
//
// assets/fonts/ is in the set because dec-0016's faces are now an input to
// every capture. A regenerated subset changes glyph rasterization on both
// sides, which is precisely the kind of change that makes captures taken
// before it incomparable with captures taken after it.
async function fingerprint() {
  const files = [];
  for (const d of ['docs/design/screens', 'docs/design']) {
    for (const f of await readdir(join(ROOT, d))) {
      if (/\.(html|css)$/.test(f)) files.push(join(d, f));
    }
  }
  for (const f of await readdir(join(ROOT, 'assets/fonts'))) {
    if (/\.woff2$/.test(f)) files.push(join('assets/fonts', f));
  }
  files.sort();
  const h = createHash('sha256');
  for (const f of files) h.update(f).update(await readFile(join(ROOT, f)));
  return { hash: h.digest('hex'), count: files.length };
}

const probeAll = (a, b) => {
  const at = {};
  let maxDelta = 0, total = 0;
  for (const c of CHANNEL_PROBES) {
    const r = diff(a, b, c);
    maxDelta = r.maxDelta; total = r.totalPixels;
    at[c] = { pct: r.overThresholdPct, blockPct: r.worstBlockPct, px: r.overThreshold };
  }
  return { at, maxDelta, total, inert: at[0].px === 0 };
};

const fpBefore = await fingerprint();
console.log(`inputs fingerprinted: ${fpBefore.count} html/css/woff2 files, sha256 ${fpBefore.hash.slice(0,16)}`);
console.log(`measuring ${SCREENS.length} screens x ${Object.keys(VIEWPORTS).length} viewports x ${SCHEMES.length} schemes`);
console.log(`  ${NOISE.length} noise arms, ${SIGNAL.length} signal arms, at deviceScaleFactor 2\n`);

for (const screen of SCREENS) {
  const path = `/docs/design/screens/${screen}.html`;
  for (const [vp, [width, height]] of Object.entries(VIEWPORTS)) {
    for (const colorScheme of SCHEMES) {
      const tag = `${screen} ${vp} ${colorScheme}`;
      // The reference page stays OPEN so the same-page arm can re-shoot it live.
      const refHandle = await open(browserA, primary.base, path, width, height, colorScheme);
      const ref = {
        img: await shoot(refHandle.page, path),
        html: await refHandle.page.evaluate(() => '<!doctype html>\n' + document.documentElement.outerHTML),
      };

      for (const [name, , opts] of NOISE) {
        let got;
        if (opts.samePage) got = { img: await shoot(refHandle.page, path) };
        else if (opts.newBrowser) got = await capture(browserB, primary.base, path, width, height, colorScheme);
        else if (opts.otherOrigin) got = await capture(browserA, secondary.base, path, width, height, colorScheme);
        else if (opts.reserialize) {
          const s = await serve({ [path]: ref.html });
          got = await capture(browserA, s.base, path, width, height, colorScheme);
          s.server.close();
        } else got = await capture(browserA, primary.base, path, width, height, colorScheme);

        let at;
        try { at = probeAll(ref.img, got.img); }
        catch (e) { rows.push({ pop: 'noise', arm: name, tag, error: e.message }); continue; }
        rows.push({ pop: 'noise', arm: name, tag, ...at });
      }

      for (const [pop, arms] of [['signal', SIGNAL], ['info', INFO]]) {
        for (const [name, , mutate] of arms) {
          const css = await readFile(join(ROOT, TOKENS.slice(1)), 'utf8');
          const mutated = mutate(css);
          if (mutated === css) { rows.push({ pop, arm: name, tag, error: 'mutation did not change the CSS' }); continue; }
          const s = await serve({ [TOKENS]: mutated });
          const got = await capture(browserA, s.base, path, width, height, colorScheme);
          s.server.close();
          let at;
          try { at = probeAll(ref.img, got.img); }
          catch (e) {
            // A dimension change IS the loudest possible signal — record it as such.
            at = { maxDelta: 255, dimensionChange: true, note: e.message, inert: false,
                   at: Object.fromEntries(CHANNEL_PROBES.map(c => [c, { pct: 100, blockPct: 100, px: null }])) };
          }
          // "inert" means the mutation was valid but this particular screen does
          // not use the thing it changed (s1-decision has no .card). Excluded from
          // the signal floor: a screen a defect cannot reach is not evidence that
          // the defect is invisible. Counted and printed, so an arm that is inert
          // EVERYWHERE is caught as a broken arm rather than silently lowering the
          // floor to zero — which it did on the first run of this script.
          rows.push({ pop, arm: name, tag, ...at });
        }
      }
      await refHandle.ctx.close();
      console.log(`  ${tag.padEnd(30)} done`);
    }
  }
}

await browserA.close(); await browserB.close();
primary.server.close(); secondary.server.close();

const fpAfter = await fingerprint();
if (fpAfter.hash !== fpBefore.hash) {
  console.log('\nMEASUREMENT VOID — a design input changed while this run was in progress.');
  console.log(`  before: ${fpBefore.hash.slice(0, 16)} (${fpBefore.count} files)`);
  console.log(`  after:  ${fpAfter.hash.slice(0, 16)} (${fpAfter.count} files)`);
  console.log('  Baselines captured before the edit are not comparable with captures made after it,');
  console.log('  so no tolerance is emitted. Re-run once the tree is quiet.');
  process.exit(1);
}
console.log(`\ninputs unchanged through the run (sha256 ${fpAfter.hash.slice(0, 16)})`);

// ---- summarise ---------------------------------------------------------------
const SAFETY = 4;   // the tolerance sits this factor below the smallest real defect

// `keepInert` because inert means the opposite thing in the two populations. For
// SIGNAL, an inert row is a defect that could not reach that screen, and counting
// its zero would drag the floor to zero (it did, on the first run). For NOISE, an
// inert row IS the result — zero difference is exactly what the arm is testing
// for — so dropping it would leave the noise table empty and claim no evidence
// where there is twelve rows of it.
const armStats = (pop, c, keepInert = false) => {
  const arms = [...new Set(rows.filter(r => r.pop === pop).map(r => r.arm))];
  return arms.map(arm => {
    const all = rows.filter(r => r.pop === pop && r.arm === arm && !r.error);
    const rs = keepInert ? all : all.filter(r => !r.inert);
    const errs = rows.filter(r => r.pop === pop && r.arm === arm && r.error);
    const pcts = rs.map(r => r.at[c].pct), blocks = rs.map(r => r.at[c].blockPct);
    return {
      arm, n: all.length, effective: rs.length, inert: all.length - rs.length, errors: errs.length,
      maxPct: rs.length ? Math.max(...pcts) : null,
      minPct: rs.length ? Math.min(...pcts) : null,
      maxDelta: rs.length ? Math.max(...rs.map(r => r.maxDelta)) : null,
      minMaxDelta: rs.length ? Math.min(...rs.map(r => r.maxDelta)) : null,
      maxBlockPct: rs.length ? Math.max(...blocks) : null,
      minBlockPct: rs.length ? Math.min(...blocks) : null,
      inertOn: all.filter(r => r.inert).map(r => r.tag),
    };
  });
};

const noiseRows = rows.filter(r => r.pop === 'noise' && !r.error);
const signalRows = rows.filter(r => r.pop === 'signal' && !r.error && !r.inert);
const noiseMaxDelta = Math.max(...noiseRows.map(r => r.maxDelta), 0);
const signalMinMaxDelta = signalRows.length ? Math.min(...signalRows.map(r => r.maxDelta)) : NaN;

const deadArms = armStats('signal', 0).filter(a => a.effective === 0);
if (deadArms.length) {
  console.log(`\nMEASUREMENT FAIL — signal arm(s) inert on every screen: ${deadArms.map(a => a.arm).join(', ')}.\n` +
    '  An arm that changes nothing anywhere is not a signal; it is a typo in the mutation.');
  await mkdir(FIDELITY, { recursive: true });
  await writeFile(join(FIDELITY, 'tolerance-evidence.json'), JSON.stringify({ rows }, null, 2) + '\n');
  process.exit(1);
}

// ---- select the channel threshold from the data ------------------------------
// A candidate is admissible when, at that threshold, (a) every noise row still
// measures zero, (b) no signal row has gone invisible, and (c) the weakest signal
// arm's peak delta is still at least SAFETY times the threshold — so the
// threshold is nowhere near swallowing the quietest real defect.
//
// THE SMALLEST admissible value wins, and the first version of this file took the
// largest. That was wrong, and wrong in a way that read as a safety argument: a
// LARGER channel threshold makes the gate less sensitive, not more robust. The
// only thing a threshold above zero buys is immunity to per-pixel noise, and this
// harness measured per-pixel noise at exactly zero on all 60 rows — the same
// evidence used to reject `tolerance = noise x k`. Applying it in one dimension
// and not the other was incoherent.
//
// So the rule is: the threshold is the smallest value that filters all MEASURED
// noise. On a machine where noise is zero, that is zero, and the gate is as
// sensitive as the evidence allows. On a machine where it is not, this selects
// the least desensitisation that does the job, rather than the most the signal
// can survive.
const candidates = CHANNEL_PROBES.map(c => {
  const noiseZero = noiseRows.every(r => r.at[c].px === 0);
  const allVisible = signalRows.every(r => r.at[c].pct > 0);
  const deltaMargin = signalMinMaxDelta / (c || 1);
  return { c, noiseZero, allVisible, deltaMargin, admissible: noiseZero && allVisible && (c === 0 || signalMinMaxDelta >= SAFETY * c) };
});
console.log('\n== channel threshold candidates ==');
for (const k of candidates) console.log(
  `  ${String(k.c).padStart(3)}/255   noise all zero: ${k.noiseZero ? 'yes' : 'NO '}   every signal still visible: ${k.allVisible ? 'yes' : 'NO '}` +
  `   weakest signal peak delta ${signalMinMaxDelta}/255 = ${(signalMinMaxDelta / (k.c || 1)).toFixed(1)}x threshold   -> ${k.admissible ? 'admissible' : 'rejected'}`);

const admissible = candidates.filter(k => k.admissible);
const channel = admissible.length ? Math.min(...admissible.map(k => k.c)) : 0;
console.log(`  chosen: ${channel}/255 — the SMALLEST threshold that filters all measured noise, so the gate is as`);
console.log(`  sensitive as the evidence allows. A larger one would desensitise it to buy immunity to`);
console.log(`  per-pixel variance that was measured at zero.`);

// ---- the numbers, at the chosen threshold ------------------------------------
const noiseArms = armStats('noise', channel, true);
const signalArms = armStats('signal', channel);
const infoArms = armStats('info', channel);

const noiseMaxPct = Math.max(...noiseRows.map(r => r.at[channel].pct), 0);
const noiseMaxBlock = Math.max(...noiseRows.map(r => r.at[channel].blockPct), 0);
const signalMinPct = Math.min(...signalRows.map(r => r.at[channel].pct));
const signalMinBlock = Math.min(...signalRows.map(r => r.at[channel].blockPct));

const pad = s => String(s).padEnd(16);
const num = (v, d = 4) => (v === null || v === undefined || Number.isNaN(v) ? '—' : v.toFixed(d)).padStart(11);
const table = (title, arms, key, keyBlock, label) => {
  console.log(`\n== ${title} ==`);
  console.log(`  ${pad('arm')}${label.padStart(11)}${'peak Δ/255'.padStart(12)}${'block %'.padStart(11)}   n (inert)`);
  for (const a of arms) console.log(
    `  ${pad(a.arm)}${num(a[key], 6)}${num(a.maxDelta, 0).padStart(12)}${num(a[keyBlock], 2)}` +
    `   ${a.effective}${a.inert ? ` (${a.inert} inert: ${a.inertOn.join(', ')})` : ''}${a.errors ? ` [${a.errors} err]` : ''}`);
};
console.log(`\nall figures below are at the chosen channel threshold of ${channel}/255`);
table('NOISE — differences that must NOT fail the gate', noiseArms, 'maxPct', 'maxBlockPct', 'max % px');
table('SIGNAL — the smallest real defects the gate must catch', signalArms, 'minPct', 'minBlockPct', 'min % px');
table('INFO — not gated; quantifies what the same-environment rule buys', infoArms, 'minPct', 'minBlockPct', 'min % px');

console.log(`\n  noise  ceiling: ${noiseMaxPct.toFixed(6)}% of pixels, worst block ${noiseMaxBlock.toFixed(2)}%, peak channel delta ${noiseMaxDelta}`);
console.log(`  signal floor:   ${signalMinPct.toFixed(6)}% of pixels, worst block ${signalMinBlock.toFixed(2)}%`);

// ---- derive ------------------------------------------------------------------
// The rule, and why it reads from the SIGNAL side rather than the noise side.
//
// The obvious rule is "tolerance = noise x k". It does not survive contact with
// the measurement: noise here is exactly zero on every arm, so noise x k is zero
// for every k, and a zero-width gate is brittle for no stated reason. The
// measurement's actual finding is that this harness is deterministic, which means
// the tolerance is NOT absorbing observed variance — there is none to absorb. It
// is reserving headroom, and headroom is only meaningful relative to the thing it
// must not swallow. So:
//
//   tolerance = smallest measured real defect / SAFETY, truncated (never rounded
//   up) to two significant figures, and required to be >= the noise ceiling.
//
// Truncation matters: rounding up would silently eat part of the safety factor
// that is the whole point of the number.
const floor2sig = v => {
  if (!(v > 0)) return 0;
  const e = Math.floor(Math.log10(v)) - 1;
  // toPrecision, not `Math.floor(v / 10**e) * 10**e` — the multiply reintroduces
  // binary float error and published 0.00033000000000000005% as the tolerance.
  return Number((Math.floor(v / 10 ** e) * 10 ** e).toPrecision(2));
};
const pixelTolerance = floor2sig(signalMinPct / SAFETY);
const blockTolerance = floor2sig(signalMinBlock / SAFETY);

const gapPct = signalMinPct / pixelTolerance;
const gapBlock = signalMinBlock / blockTolerance;
const aboveNoise = pixelTolerance >= noiseMaxPct && blockTolerance >= noiseMaxBlock;

console.log(`\n  derived: channel ${channel}/255 · pixel tolerance ${pixelTolerance}% · block tolerance ${blockTolerance}%`);
console.log(`  separation: the smallest real defect is ${gapPct.toFixed(1)}x the pixel tolerance and ${gapBlock.toFixed(1)}x the block tolerance (need >=${SAFETY}x on both)`);
console.log(`  headroom:   the tolerance is ${aboveNoise ? 'at or above' : 'BELOW'} the measured noise ceiling`);

const separated = gapPct >= SAFETY && gapBlock >= SAFETY && aboveNoise;

await mkdir(FIDELITY, { recursive: true });
const evidence = {
  measured_at: new Date().toISOString(),
  platform: `${process.platform} ${process.arch}`, node: process.version,
  device_scale_factor: 2,
  screens: SCREENS, viewports: VIEWPORTS, schemes: SCHEMES,
  channel_probes: CHANNEL_PROBES,
  channel_candidates: candidates,
  safety_factor: SAFETY,
  // Recorded so a reader can confirm WHICH mockups produced these numbers, and
  // re-check them later. Reaching this line is already proof the inputs did not
  // move mid-run — the VOID branch exits before it — but that proof is only
  // available to whoever watched the run. This makes it durable.
  input_fingerprint: { sha256: fpAfter.hash, files: fpAfter.count },
  noise_arms: NOISE.map(([n, d]) => ({ arm: n, is: d })),
  signal_arms: SIGNAL.map(([n, d]) => ({ arm: n, is: d })),
  info_arms: INFO.map(([n, d]) => ({ arm: n, is: d })),
  noise_summary: noiseArms, signal_summary: signalArms, info_summary: infoArms,
  chosen_channel_threshold: channel,
  noise_ceiling: { pct: noiseMaxPct, block_pct: noiseMaxBlock, channel_delta: noiseMaxDelta },
  signal_floor: { pct: signalMinPct, block_pct: signalMinBlock, weakest_peak_delta: signalMinMaxDelta },
  derived: { channel_threshold: channel, pixel_tolerance_pct: pixelTolerance, block_tolerance_pct: blockTolerance },
  separation: { pct_multiple: gapPct, block_multiple: gapBlock, separated },
  rows,
};
await writeFile(join(FIDELITY, 'tolerance-evidence.json'), JSON.stringify(evidence, null, 2) + '\n');
console.log(`\n  evidence -> docs/design/fidelity/tolerance-evidence.json (${rows.length} measurements)`);

if (!separated) {
  console.log('\nMEASUREMENT FAIL — the noise and signal populations do not separate. No tolerance is emitted.\n' +
    '  A number chosen here would pass the noise AND the smallest real defect, which is a\n' +
    '  decorative gate. Fix the harness (determinism) or enlarge the signal arms before\n' +
    '  writing a tolerance into DESIGN.md.');
  process.exit(1);
}

if (WRITE) {
  await writeFile(join(FIDELITY, 'tolerance.json'), JSON.stringify({
    $comment: 'MEASURED, not chosen. Regenerate with: node docs/design/scripts/measure-tolerance.mjs --write. ' +
              'Evidence and method in tolerance-evidence.json and docs/design/fidelity/TOLERANCE.md.',
    channel_threshold: channel,
    pixel_tolerance_pct: pixelTolerance,
    block_tolerance_pct: blockTolerance,
    block_px: 16,
    device_scale_factor: 2,
    measured_at: evidence.measured_at,
    measured_on: evidence.platform,
    noise_ceiling_pct: +noiseMaxPct.toFixed(6),
    signal_floor_pct: +signalMinPct.toFixed(6),
  }, null, 2) + '\n');
  console.log('  tolerance -> docs/design/fidelity/tolerance.json');
}
console.log('\nMEASUREMENT OK — noise and signal are separated; the tolerance sits in the gap.');
