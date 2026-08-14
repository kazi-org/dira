// distill-keys.mjs — E6-L3-T5's proof that y/n/e are progressive enhancement,
// not a second implementation of the write.
//
//   node docs/design/scripts/distill-keys.mjs             # the proof
//   node docs/design/scripts/distill-keys.mjs --probe-nav  # the reverted-navigation control (must FAIL)
//
// The claim under test: pressing y/n/e with JavaScript on produces the exact
// same ledger write a click on the same button produces with JavaScript OFF —
// because the keydown handler calls form.requestSubmit() on the same <form>
// element a click submits, so "the same file change" is true by construction.
// This script is the one place that is actually checked with a browser rather
// than read from the source: two Playwright browser contexts, one with
// javaScriptEnabled left on (dispatches a keydown) and one with it turned off
// (clicks the button, which becomes a genuine full-page form POST because
// there is no script to intercept it) — each against its own fresh, identical
// temp ledger — with the two resulting files compared.
//
// Frontmatter comparison drops the `updated:` line before hashing, the same
// normalisation internal/ui's own Go tests apply (frontmatterSHA in
// distill_test.go): both branches call time.Now() microseconds apart on the
// server, so the raw bytes can never be byte-identical even when every field
// that matters is.
//
// What this script does NOT re-derive: that the fetch calls are same-origin,
// loopback-relative and carry no cookie/auth header. That is a property of
// the script's SOURCE, checked once, statically, by
// go test ./internal/ui -run TestDistillHasExactlyOneScript — re-deriving it
// from network traffic here would just be a slower way to fail the same
// check on the same file.
//
// Exit codes: 0 pass, 1 fail, 2 harness could not run.

import { chromium } from 'playwright';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtemp, mkdir, writeFile, readFile, rm } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { tmpdir } from 'node:os';
import { join, resolve, dirname } from 'node:path';

const HERE = dirname(new URL(import.meta.url).pathname);
const ROOT = resolve(HERE, '../../..');
const PROBE_NAV = process.argv.includes('--probe-nav');

const die = (code, msg) => { console.error(msg); process.exit(code); };
const fails = [];
const fail = (msg) => { fails.push(msg); console.log(`  FAIL ${msg}`); };
const ok = (msg) => console.log(`  ok   ${msg}`);

// ---- 1. build ----------------------------------------------------------------
const bin = join(await mkdtemp(join(tmpdir(), 'dira-distillkeys-')), 'dira');
{
  const r = spawnSync('go', ['build', '-o', bin, './cmd/dira'], { cwd: ROOT, encoding: 'utf8' });
  if (r.status !== 0) die(2, `go build failed:\n${r.stderr || r.stdout}`);
}

// ---- 2. two identical, fresh temp ledgers -------------------------------------
// Sniff-shaped: a title and a regex-tier source, no confirmed_by — the same
// shape distill_test.go's distillEntry builds, so dec-0001 lands in
// Awaiting() and is the deck's one actionable card.
const ENTRY = (id, title) => `---
id: ${id}
kind: decision
title: "${title}"
state: staged
created: "2026-07-31T09:00:00Z"
source:
  hook: Stop
  tier: regex
---

The regex-staged because, before any keystroke.
`;

async function freshLedger() {
  const root = await mkdtemp(join(tmpdir(), 'dira-distillkeys-ledger-'));
  const entries = join(root, '.dira', 'entries');
  await mkdir(entries, { recursive: true });
  await writeFile(join(entries, 'dec-0001.md'), ENTRY('dec-0001', 'ship the thing without asking twice'));
  return { root, entryPath: join(entries, 'dec-0001.md') };
}

// frontmatterDigest hashes a file with its `updated:` line dropped — the
// same normalisation distill_test.go's frontmatterSHA applies, for the same
// reason: two independent server writes can never land on the same
// microsecond.
async function frontmatterDigest(path) {
  let raw;
  try {
    raw = await readFile(path, 'utf8');
  } catch {
    return null; // discarded — absence is the signal for `n`
  }
  const kept = raw.split('\n').filter(l => !l.startsWith('updated:')).join('\n');
  return createHash('sha256').update(kept).digest('hex');
}

// ---- 3. one server per ledger --------------------------------------------------
async function startServer(root) {
  const p = spawn(bin, ['ui', '-C', root, '-addr', '127.0.0.1:0'], { cwd: root });
  const errs = [];
  p.stderr.on('data', d => errs.push(String(d)));
  const base = await new Promise((res, rej) => {
    const t = setTimeout(() => rej(new Error('dira ui printed no URL within 20s')), 20000);
    p.stdout.on('data', d => {
      const m = String(d).match(/http:\/\/127\.0\.0\.1:\d+/);
      if (m) { clearTimeout(t); res(m[0]); }
    });
    p.on('exit', c => { clearTimeout(t); rej(new Error(`dira ui exited ${c}: ${errs.join('')}`)); });
  });
  return { base, stop: () => { try { p.kill('SIGINT'); } catch {} } };
}

const browser = await chromium.launch();

// runCase drives one key (y/n/e) through one context (js on or off) against
// its own fresh ledger, and reports whether a navigation ('load' event) fired
// after the interaction and what the entry file looks like afterward.
async function runCase(key, { javaScriptEnabled }) {
  const { root, entryPath } = await freshLedger();
  const { base, stop } = await startServer(root);
  const ctx = await browser.newContext({ javaScriptEnabled });
  const page = await ctx.newPage();

  await page.goto(base + '/distill', { waitUntil: 'load' });

  let loadsAfterInteraction = 0;
  page.on('load', () => { loadsAfterInteraction++; });

  if (javaScriptEnabled) {
    if (key === 'e') {
      await page.keyboard.press('e');
      await page.fill('.edit-disclosure textarea', 'The rewritten because, from the keydown case.');
      await page.click('.edit-disclosure button[type="submit"]');
    } else {
      await page.keyboard.press(key);
    }
    // A fetch()-driven update has nothing to wait ON but time; give the
    // in-page DOM swap a moment to land before reading the file.
    await page.waitForTimeout(400);
  } else {
    // No script: the same visible controls, but a click on a submit button
    // is now a genuine, full-page <form> POST — the baseline this script
    // compares the keydown case against.
    if (key === 'y') {
      await page.click('form[action$="/confirm"] button[type="submit"]');
    } else if (key === 'n') {
      await page.click('form[action$="/discard"] button[type="submit"]');
    } else {
      // <details> opens with a plain click when JS is off — the whole point
      // of using <details>/<summary> rather than a script-driven toggle.
      await page.click('.edit-disclosure summary');
      await page.fill('.edit-disclosure textarea', 'The rewritten because, from the keydown case.');
      await page.click('.edit-disclosure button[type="submit"]');
    }
    await page.waitForLoadState('load');
  }

  const digest = await frontmatterDigest(entryPath);
  await ctx.close();
  stop();
  return { digest, loadsAfterInteraction: javaScriptEnabled ? loadsAfterInteraction : null };
}

// ---- 4/5. the three keys, or the --probe-nav control --------------------------
//
// --probe-nav is the negative control for the "no page load" assertion
// below: it proves the load-event listener the loop uses can actually see a
// full navigation, by staging one directly (a synthetic page whose 'y'
// binding does location.href = '/distill' — the mistake T5's real binding
// does not make, reverted here rather than ever landing in the served
// template) and asserting the SAME listener technique catches it. It does
// not run the real y/n/e cases below — a control proves the checker's
// teeth, not the feature, and running both under one exit code would make
// gates.mjs unable to tell which one failed.
if (PROBE_NAV) {
  const { root } = await freshLedger();
  const { base, stop } = await startServer(root);
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(base + '/distill', { waitUntil: 'load' });
  // A second, additive binding on top of the real page — bound to a key
  // (z) the real script never uses, so this proves the LISTENER can see a
  // full navigation, without needing to first defeat T5's real handler.
  // location.href resolves against the real running server, unlike a
  // synthetic about:blank page, where the same navigation would error out
  // before ever completing and this control would prove nothing.
  await page.evaluate(() => {
    document.addEventListener('keydown', function (ev) {
      if (ev.key === 'z') { location.href = '/distill'; } // the reverted mistake
    });
  });
  let loads = 0;
  page.on('load', () => { loads++; });
  await page.keyboard.press('z');
  // A timeout, not waitForLoadState('load'): the page is already IN the
  // 'load' state from the initial goto, so waitForLoadState would resolve
  // immediately without waiting for the new navigation the keypress
  // triggers — the same race a fixed-timeout wait avoids everywhere else in
  // this script.
  await page.waitForTimeout(1000);
  await ctx.close();
  await browser.close();
  stop();
  if (loads > 0) {
    console.log('PROBE OK — the load-event listener caught the reverted full-navigation binding');
    process.exit(1); // the failure IS the proof, per gates.mjs's control convention
  }
  console.log('PROBE FAILED — a full navigation occurred and the listener did not see it; the check below is blind');
  process.exit(3);
}

for (const key of ['y', 'n', 'e']) {
  const withJS = await runCase(key, { javaScriptEnabled: true });
  const withoutJS = await runCase(key, { javaScriptEnabled: false });

  if (withJS.loadsAfterInteraction > 0) {
    fail(`'${key}': ${withJS.loadsAfterInteraction} page load(s) fired after the keydown — a full navigation, not an in-place update`);
  } else {
    ok(`'${key}': no page load after the keydown`);
  }

  if (key === 'n') {
    // Discard: the file must be gone in both branches.
    if (withJS.digest !== null) fail(`'y'->'n' JS case: dec-0001.md still exists after discard`);
    else ok(`'n' (JS): dec-0001.md deleted`);
    if (withoutJS.digest !== null) fail(`'n' no-JS case: dec-0001.md still exists after discard`);
    else ok(`'n' (no JS): dec-0001.md deleted`);
  } else {
    if (withJS.digest === null || withoutJS.digest === null) {
      fail(`'${key}': one of the two cases left no file to compare (digest missing)`);
    } else if (withJS.digest !== withoutJS.digest) {
      fail(`'${key}': the keydown write and the form-click write produced different files (frontmatter, updated excluded)`);
    } else {
      ok(`'${key}': keydown write == form-click write`);
    }
  }
}

await browser.close();

if (fails.length) {
  console.log(`\ndistill-keys FAIL — ${fails.length} issue(s)`);
  process.exit(1);
}
console.log('\ndistill-keys PASS — y/n/e are progressive enhancement, proven both ways.');
