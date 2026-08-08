// Headless capture for dira mockups. Per web-design/RENDERER.md.
//
//   node docs/design/scripts/render.mjs <iteration> [glob-substring]
//
// Captures every target x {mobile,laptop,wide} x {light,dark} and runs the
// mechanical gate: console errors, failed requests, non-loopback asset requests,
// blank mount, byte-identical light/dark pair (fake dark mode), and post-load
// layout shift.
//
// Serves the repo over http rather than file:// so relative paths and
// prefers-color-scheme behave exactly as they will in `dira ui`.
//
//   node docs/design/scripts/render.mjs <iteration> [glob-substring]
//   node docs/design/scripts/render.mjs <iteration> --probe-external
//
// --probe-external is the negative control for the loopback rule. See the
// NON-LOOPBACK section below.

import { chromium } from 'playwright';
import { createServer } from 'node:http';
import { readFile, mkdir, writeFile, readdir } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { networkInterfaces } from 'node:os';
import { extname, join, resolve } from 'node:path';

const ROOT = resolve(import.meta.dirname, '../../..');      // repo root
const OUT = resolve(import.meta.dirname, '../renders');
const ARGS = process.argv.slice(2);
const PROBE_EXTERNAL = ARGS.includes('--probe-external');
const POSITIONAL = ARGS.filter(a => !a.startsWith('--'));
const ITER = POSITIONAL[0] ?? 'r1';
const FILTER = POSITIONAL[1] ?? '';

const VIEWPORTS = { mobile: [390, 844], laptop: [1024, 768], wide: [1440, 900] };
const SCHEMES = ['light', 'dark'];
const MIME = { '.html': 'text/html', '.css': 'text/css', '.js': 'text/javascript',
               '.svg': 'image/svg+xml', '.json': 'application/json', '.png': 'image/png' };

// Paths served from memory rather than disk. Used only by --probe-external, so
// the negative control does not require writing a throwaway file into the repo.
const OVERRIDES = new Map();

// ---- static server ----------------------------------------------------------
const server = createServer(async (req, res) => {
  const url = decodeURIComponent(req.url.split('?')[0]);
  if (OVERRIDES.has(url)) {
    res.writeHead(200, { 'content-type': MIME[extname(url)] ?? 'text/plain' });
    return res.end(OVERRIDES.get(url));
  }
  try {
    const p = join(ROOT, url);
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
const landingDir = resolve(import.meta.dirname, '../landing');
try {
  for (const f of (await readdir(landingDir)).filter(f => f.endsWith('.html')).sort()) {
    TARGETS.push([f.replace(/\.html$/, '') === 'index' ? 'landing' : f.replace(/\.html$/, ''), `/docs/design/landing/${f}`]);
  }
} catch {}
if (FILTER) TARGETS = TARGETS.filter(([n]) => n.includes(FILTER));

// ---- NON-LOOPBACK: the negative control ------------------------------------
// cst-0004 and dec-0010 both hinge on the rendered surfaces fetching nothing from
// anywhere. The pre-existing gate catches FAILED requests, which means it would
// pass a page that successfully loads a webfont from a CDN — the exact failure
// mode that turns "your data never touches our servers" into a false statement.
// A gate that only sees failures cannot see a success it should have refused.
//
// So the control has to stage a request that SUCCEEDS from a host that is not
// 127.0.0.1. It serves a real stylesheet from this machine's LAN address: HTTP
// 200, no internet involved, and invisible to the requestfailed check. If the
// machine has no non-loopback interface, it falls back to `localhost` — still a
// host that is not the literal 127.0.0.1, but a weaker demonstration, and it says
// so rather than pretending the strong one ran.
let probeServer = null, probeHost = null, probeStrong = false;
if (PROBE_EXTERNAL) {
  const lan = Object.values(networkInterfaces()).flat()
    .find(a => a && a.family === 'IPv4' && !a.internal)?.address;
  probeStrong = Boolean(lan);
  const host = lan ?? '127.0.0.1';
  probeServer = createServer((req, res) => {
    res.writeHead(200, { 'content-type': 'text/css' });
    res.end('/* an asset from off this host. It loads. That is the point. */\n.probe-external{letter-spacing:0}\n');
  });
  await new Promise(r => probeServer.listen(0, host, r));
  probeHost = `http://${lan ?? 'localhost'}:${probeServer.address().port}`;

  const url = `${probeHost}/probe.css`;
  OVERRIDES.set('/docs/design/fidelity/probes/external-asset.html', `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>external asset probe</title>
<link rel="stylesheet" href="/docs/design/tokens.css">
<link rel="stylesheet" href="${url}">
</head><body>
<main style="padding:var(--s6);max-width:var(--m-prose)">
<h1 style="font-family:var(--serif)">External asset probe</h1>
<p class="probe-external">This page loads a stylesheet from ${url}, which is not 127.0.0.1.
The request succeeds, so the failed-request check stays silent. The loopback check must
not. If this page passes the gate, the gate is decorative.</p>
</main></body></html>`);
  TARGETS = [['probe-external-asset', '/docs/design/fidelity/probes/external-asset.html']];
  console.log(`--probe-external: serving an asset from ${url}` +
    (probeStrong ? ' (a real non-loopback host; the request will SUCCEED)'
                 : ' (no non-loopback interface available — falling back to the `localhost` alias, a weaker control)'));
}

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

      // Every request URL, recorded at request time so it is seen whether or not
      // it succeeds. `data:`, `blob:` and `about:` carry no host and reach no
      // network; anything else must be the literal loopback address.
      const offHost = new Map();
      page.on('request', r => {
        const u = r.url();
        if (/^(data|blob|about):/.test(u)) return;
        let host;
        try { host = new URL(u).hostname; } catch { return; }
        if (host !== '127.0.0.1') offHost.set(u, (offHost.get(u) ?? 0) + 1);
      });

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

      // non-loopback probe — named URLs, not a count
      for (const [url, n] of offHost) {
        errs.push(`NON-LOOPBACK asset (${n}x): ${url} — host is not 127.0.0.1 (cst-0004, dec-0010)`);
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
probeServer?.close();

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

if (PROBE_EXTERNAL) {
  // Under the probe the EXPECTED result is a failure, and specifically a
  // non-loopback failure. Reporting "gate failed" would not distinguish the check
  // firing from the page merely being broken, so the probe asserts on the reason.
  const nonLoopback = gate.flatMap(g => g.errs).filter(e => e.startsWith('NON-LOOPBACK'));
  const failedReq = gate.flatMap(g => g.errs).filter(e => e.startsWith('FAILED'));
  console.log(`\nPROBE — external asset served from ${probeHost}`);
  console.log(`  non-loopback findings: ${nonLoopback.length}`);
  console.log(`  failed-request findings: ${failedReq.length}` +
    (probeStrong ? '  <- expected 0: the asset LOADS, which is why the old check cannot see it' : ''));
  for (const e of [...new Set(nonLoopback)]) console.log(`    ${e}`);
  if (!nonLoopback.length) {
    console.log('\nPROBE BROKEN — an asset was served from a non-loopback host and the gate did not notice.');
    process.exit(3);
  }
  if (probeStrong && failedReq.length) {
    console.log('\nPROBE INCONCLUSIVE — the external request failed, so this run does not prove the\n' +
      '  check catches a SUCCESSFUL non-loopback load; it may only be re-detecting the failure.');
    process.exit(3);
  }
  console.log('\nPROBE OK — the loopback check fired on a request the failed-request check could not see.');
  process.exit(1);
}

if (gate.length) {
  console.log(`\nGATE FAIL (${gate.length}):`);
  for (const g of gate) console.log(`  ${g.name} ${g.vp} ${g.colorScheme}\n    - ${g.errs.join('\n    - ')}`);
  process.exitCode = 1;
} else {
  console.log('GATE PASS — no console errors, no failed requests, no non-loopback assets, no blank mounts, no fake dark, no layout shift.');
}
