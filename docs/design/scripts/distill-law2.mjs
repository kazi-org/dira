// distill-law2.mjs — measures design law 2 and the swipe hint on the SERVED
// DOM, not the source CSS. render.mjs and contrast-rendered.mjs already
// establish the pattern (composited values, not declared ones); this is one
// more script in that family, over a live `dira ui` process rather than a
// static mockup file, because E6-L3-T6's own acc line names the served page
// specifically.
//
//   node docs/design/scripts/distill-law2.mjs                 # the proof
//   node docs/design/scripts/distill-law2.mjs --probe-fontsize # law 2 control (must FAIL)
//   node docs/design/scripts/distill-law2.mjs --probe-swipehint # swipe-hint control (must FAIL)
//
// Both controls intercept the served /distill.css response (Playwright
// route interception) and splice in a defect, entirely in memory — nothing
// on disk is ever touched, so there is no scratch file to forget to revert.
//
// Exit codes: 0 pass, 1 fail, 2 harness could not run.

import { chromium } from 'playwright';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtemp, cp } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve, dirname } from 'node:path';

const HERE = dirname(new URL(import.meta.url).pathname);
const ROOT = resolve(HERE, '../../..');
const FIXTURE = resolve(ROOT, 'docs/design/fidelity/fixtures/ledger-design');
const VIEWPORTS = { mobile: [390, 844], laptop: [1024, 768], wide: [1440, 900] };

const PROBE_FONTSIZE = process.argv.includes('--probe-fontsize');
const PROBE_SWIPEHINT = process.argv.includes('--probe-swipehint');

const die = (code, msg) => { console.error(msg); process.exit(code); };

// ---- 1. build, and a temp copy of the fixture ledger under a findable .dira --
const bin = join(await mkdtemp(join(tmpdir(), 'dira-law2-')), 'dira');
{
  const r = spawnSync('go', ['build', '-o', bin, './cmd/dira'], { cwd: ROOT, encoding: 'utf8' });
  if (r.status !== 0) die(2, `go build failed:\n${r.stderr || r.stdout}`);
}
const work = await mkdtemp(join(tmpdir(), 'dira-law2-ledger-'));
await cp(FIXTURE, join(work, '.dira'), { recursive: true });

// ---- 2. start `dira ui`, same protocol uigate.mjs uses -----------------------
const p = spawn(bin, ['ui', '-C', work, '-addr', '127.0.0.1:0'], { cwd: work });
const errs = [];
p.stderr.on('data', d => errs.push(String(d)));
const base = await new Promise((res, rej) => {
  const t = setTimeout(() => rej(new Error('dira ui printed no URL within 20s')), 20000);
  p.stdout.on('data', d => {
    const m = String(d).match(/http:\/\/127\.0\.0\.1:\d+/);
    if (m) { clearTimeout(t); res(m[0]); }
  });
  p.on('exit', c => { clearTimeout(t); rej(new Error(`dira ui exited ${c}: ${errs.join('')}`)); });
}).catch(e => die(2, String(e)));
const stop = () => { try { p.kill('SIGINT'); } catch {} };

const browser = await chromium.launch();
const fails = [];

for (const [name, [width, height]] of Object.entries(VIEWPORTS)) {
  const ctx = await browser.newContext({ viewport: { width, height } });
  const page = await ctx.newPage();

  if (PROBE_FONTSIZE) {
    // The control this probe stages: .titleline set larger than .because,
    // the exact inversion law 2 forbids — appended, so it wins the cascade
    // over the real rule with equal specificity.
    await page.route('**/distill.css', async route => {
      const resp = await route.fetch();
      const body = await resp.text();
      await route.fulfill({
        response: resp,
        body: body + '\n.stage .titleline { font-size: 999px !important; }\n',
      });
    });
  }
  if (PROBE_SWIPEHINT) {
    // s3-distill.html's own r2->r3 regression, reproduced: the media query
    // that reveals the hint at mobile widths declared BEFORE the base
    // display:none, so the base rule wins the cascade at every viewport
    // (docs/design/DESIGN.md, "the mobile swipe hint never rendered").
    // Appended AFTER the real (correctly-ordered) rules, not before — equal
    // specificity means source order decides, and this has to win the
    // cascade to reproduce the regression rather than lose to the real CSS
    // already in body.
    await page.route('**/distill.css', async route => {
      const resp = await route.fetch();
      const body = await resp.text();
      await route.fulfill({
        response: resp,
        body:
          body +
          '\n@media (max-width: 767px) { .swipe-note { display: block; } }\n' +
          '.swipe-note { display: none; }\n',
      });
    });
  }

  await page.goto(base + '/distill', { waitUntil: 'load' });
  await page.evaluate(() => document.fonts.ready);
  await page.waitForLoadState('networkidle');

  const measured = await page.evaluate(() => {
    const because = document.querySelector('.stage .because');
    const title = document.querySelector('.stage .titleline');
    const swipe = document.querySelector('.swipe-note');
    return {
      becauseSize: because ? parseFloat(getComputedStyle(because).fontSize) : null,
      titleSize: title ? parseFloat(getComputedStyle(title).fontSize) : null,
      swipeDisplay: swipe ? getComputedStyle(swipe).display : null,
    };
  });

  if (measured.becauseSize == null || measured.titleSize == null) {
    fails.push(`${name}: .stage .because or .stage .titleline not found — is the fixture ledger's actionable card present?`);
  } else if (!(measured.becauseSize > measured.titleSize)) {
    fails.push(`${name}: .because ${measured.becauseSize}px is not strictly greater than .titleline ${measured.titleSize}px (law 2)`);
  } else {
    console.log(`  ok   ${name}: .because ${measured.becauseSize}px > .titleline ${measured.titleSize}px`);
  }

  const wantHidden = name === 'wide'; // 1440px
  const wantVisible = name === 'mobile'; // 390px
  if (wantHidden && measured.swipeDisplay === 'none') {
    console.log(`  ok   ${name}: .swipe-note is display:none at 1440px`);
  } else if (wantHidden) {
    fails.push(`${name}: .swipe-note is ${measured.swipeDisplay} at 1440px, want none`);
  }
  if (wantVisible && measured.swipeDisplay !== 'none') {
    console.log(`  ok   ${name}: .swipe-note is not display:none at 390px (${measured.swipeDisplay})`);
  } else if (wantVisible) {
    fails.push(`${name}: .swipe-note is none at 390px, want visible (the r2->r3 regression this checks for)`);
  }

  await ctx.close();
}

await browser.close();
stop();

if (PROBE_FONTSIZE || PROBE_SWIPEHINT) {
  if (fails.length) {
    console.log(`PROBE OK — the staged defect was caught:\n  - ${fails.join('\n  - ')}`);
    process.exit(1); // the failure IS the proof, per gates.mjs's control convention
  }
  console.log('PROBE FAILED — the staged defect was not caught; the check above is blind');
  process.exit(3);
}

if (fails.length) {
  console.log(`distill-law2 FAIL — ${fails.length} issue(s):\n  - ${fails.join('\n  - ')}`);
  process.exit(1);
}
console.log('distill-law2 PASS — law 2 holds and the swipe hint degrades correctly, at all 3 viewports, as served.');
