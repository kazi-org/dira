// uigate.mjs — does the Go-served page match the mockup it was built from?
//
//   node docs/design/scripts/uigate.mjs            # capture, gate, diff
//   node docs/design/scripts/uigate.mjs --keep     # leave the temp ledger in place
//
// The protocol, and the reason it is a protocol rather than a comparison:
// E6-L1 measured that changing only glyph rasterization costs 1.07-4.64% of
// pixels and that a serif fallback costs up to 100%, against a tolerance of
// 0.00033%. No number that still catches a 2px radius change can absorb those.
// So the baseline is REGENERATED IN THIS RUN, in this process, on this machine,
// from docs/design/screens/ — never read from a committed PNG.
// docs/design/renders/ is gitignored precisely so that cannot be circumvented.
//
// What it does:
//   1. builds the dira binary from source
//   2. starts `dira ui` over the 18-entry design fixture ledger
//   3. captures the mockup and the served page for each route, at 3 viewports
//      x 2 schemes, in the same browser process
//   4. runs the mechanical gate on the SERVED pages — console errors, page
//      errors, failed requests, non-loopback assets, blank mount, fake dark,
//      layout shift
//   5. pixel-diffs each served capture against its freshly regenerated mockup
//
// Exit codes:
//   0  the mechanical gate passed and every pair is within tolerance
//   1  the mechanical gate failed, or a pair exceeds tolerance
//   2  the harness could not run (no build, no browser, no server)

import { chromium } from 'playwright';
import { spawn, spawnSync } from 'node:child_process';
import { createServer } from 'node:http';
import { mkdtemp, mkdir, cp, rm, readFile, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { tmpdir } from 'node:os';
import { extname, join, resolve, dirname } from 'node:path';

const HERE = dirname(new URL(import.meta.url).pathname);
const ROOT = resolve(HERE, '../../..');
const OUT = resolve(HERE, '../renders');
const FIXTURE = resolve(ROOT, 'docs/design/fidelity/fixtures/ledger-design');
const KEEP = process.argv.includes('--keep');

const VIEWPORTS = { mobile: [390, 844], laptop: [1024, 768], wide: [1440, 900] };
const SCHEMES = ['light', 'dark'];

// The two routes this lane serves, each paired with the mockup it was built
// from. A route with no mockup would be a page nothing measures.
const ROUTES = [
  { name: 's2-index', served: '/', mockup: '/docs/design/screens/s2-index.html' },
  { name: 's1-decision', served: '/e/dec-0001', mockup: '/docs/design/screens/s1-decision.html' },
];

const die = (code, msg) => { console.error(msg); process.exit(code); };

// ---- 1. build ---------------------------------------------------------------
const bin = join(await mkdtemp(join(tmpdir(), 'dira-uigate-')), 'dira');
{
  const r = spawnSync('go', ['build', '-o', bin, './cmd/dira'], { cwd: ROOT, encoding: 'utf8' });
  if (r.status !== 0) die(2, `go build failed:\n${r.stderr || r.stdout}`);
}

// Is `ui` registered in cmd/dira/main.go? That file belongs to the integrator,
// so this gate must not edit it — and it must not silently measure nothing
// either. If the subcommand is absent, the gate builds a shim that calls the
// same runUI over the same package, says so loudly, and deletes the shim after.
// The shim is a scaffold, not a second entry point: the moment the registry line
// lands, this branch stops being taken.
const SHIM_DIR = resolve(ROOT, 'internal/ui/uigate_shim');
let shimmed = false;
{
  const probe = spawnSync(bin, ['help', 'ui'], { encoding: 'utf8' });
  if (probe.status !== 0) {
    console.log('NOTE — `dira ui` is not in cmd/dira/main.go\'s command registry yet.');
    console.log('       Running through a temporary shim over the same internal/ui package.');
    console.log('       The one line to add is in docs/decisions-pending/E6-L2-report.md.');
    await mkdir(SHIM_DIR, { recursive: true });
    await writeFile(join(SHIM_DIR, 'main.go'), `// Command uigate_shim exists only while docs/design/scripts/uigate.mjs runs.
// It is written and deleted by that script. If you are reading this in a diff,
// the script crashed - delete the directory.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger/local"
	"github.com/kazi-org/dira/internal/ui"
)

func main() {
	diraDir, err := local.Find(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	store, err := local.Open(diraDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ix, err := index.Open(ctx, store, local.CacheDir(diraDir))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = ix.Close() }()
	srv, err := ui.NewServer(ix, store, filepath.Base(filepath.Dir(diraDir)))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ln, err := ui.Listen("127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("http://%s\\n", ln.Addr())
	if err := ui.Serve(ctx, ln, srv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`);
    const r = spawnSync('go', ['build', '-o', bin, './internal/ui/uigate_shim'], { cwd: ROOT, encoding: 'utf8' });
    await rm(SHIM_DIR, { recursive: true, force: true });
    if (r.status !== 0) die(2, `building the shim failed:\n${r.stderr || r.stdout}`);
    shimmed = true;
  }
}

// ---- 2. the fixture ledger, under a .dira the CLI can find -------------------
// `dira ui -C dir` walks up for a `.dira`, which is how it works from a
// subdirectory of a repository — where a hook actually runs. The fixture is a
// ledger directory, not a `.dira`, so it is copied under one rather than the
// CLI being taught a second way to find a ledger for the benefit of a test.
const work = await mkdtemp(join(tmpdir(), 'dira-uigate-ledger-'));
await cp(FIXTURE, join(work, '.dira'), { recursive: true });

const uiArgs = shimmed ? [work] : ['ui', '-C', work, '-addr', '127.0.0.1:0'];
const ui = spawn(bin, uiArgs, { cwd: work });
let uiBase = null;
const uiErrors = [];
ui.stderr.on('data', d => uiErrors.push(String(d)));
uiBase = await new Promise((res, rej) => {
  const t = setTimeout(() => rej(new Error('dira ui printed no URL within 20s')), 20000);
  ui.stdout.on('data', d => {
    const m = String(d).match(/http:\/\/127\.0\.0\.1:\d+/);
    if (m) { clearTimeout(t); res(m[0]); }
  });
  ui.on('exit', c => { clearTimeout(t); rej(new Error(`dira ui exited ${c}: ${uiErrors.join('')}`)); });
}).catch(e => die(2, String(e)));

const stopUI = () => { try { ui.kill('SIGINT'); } catch {} };
process.on('exit', stopUI);

// ---- the static server for the mockups --------------------------------------
// Same shape as render.mjs: over http rather than file://, so relative paths and
// prefers-color-scheme behave exactly as they do in `dira ui`.
// .woff2 is load-bearing here rather than tidy: this server stands in for the
// mockup side of the comparison, and the Go side sends font/woff2. A pixel diff
// between two pages that loaded the same face over two different content-types
// is measuring the harness.
const MIME = { '.html': 'text/html', '.css': 'text/css', '.svg': 'image/svg+xml', '.png': 'image/png',
               '.woff2': 'font/woff2' };
const mockServer = createServer(async (req, res) => {
  const url = decodeURIComponent(req.url.split('?')[0]);
  try {
    const body = await readFile(join(ROOT, url));
    res.writeHead(200, { 'content-type': MIME[extname(url)] ?? 'application/octet-stream' });
    res.end(body);
  } catch { res.writeHead(404); res.end('not found'); }
});
await new Promise(r => mockServer.listen(0, '127.0.0.1', r));
const mockBase = `http://127.0.0.1:${mockServer.address().port}`;

// ---- 3. capture --------------------------------------------------------------
await mkdir(OUT, { recursive: true });
const browser = await chromium.launch();
const gate = [];
const hashes = {};

// capture returns the file it wrote. `which` is 'mock' or 'ui'; only the served
// pages are put through the mechanical gate, because the mockups already have
// their own gate (render.mjs) and failing them here would report the same defect
// twice under a different name.
async function capture(which, name, url, vp, colorScheme, [width, height]) {
  const ctx = await browser.newContext({
    viewport: { width, height }, colorScheme, deviceScaleFactor: 2, reducedMotion: 'reduce',
  });
  const page = await ctx.newPage();
  const errs = [];
  page.on('console', m => m.type() === 'error' && errs.push(`console: ${m.text()}`));
  page.on('pageerror', e => errs.push(`pageerror: ${e.message}`));
  page.on('requestfailed', r => errs.push(`FAILED ${r.url()}`));

  const offHost = new Map();
  page.on('request', r => {
    const u = r.url();
    if (/^(data|blob|about):/.test(u)) return;
    let host; try { host = new URL(u).hostname; } catch { return; }
    if (host !== '127.0.0.1') offHost.set(u, (offHost.get(u) ?? 0) + 1);
  });

  await page.goto(url, { waitUntil: 'load' });
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(400);

  const h1 = await page.evaluate(() => document.body.scrollHeight);
  const file = join(OUT, `ui-${which}-${name}-${vp}-${colorScheme}.png`);
  await page.screenshot({ path: file, fullPage: true });

  if (which === 'ui') {
    await page.waitForTimeout(1100);
    const h2 = await page.evaluate(() => document.body.scrollHeight);
    if (Math.abs(h2 - h1) > 4) errs.push(`layout shift ${h1}px -> ${h2}px after load`);
    const ink = await page.evaluate(() => document.body.innerText.trim().length);
    if (ink < 40) errs.push(`near-blank capture (${ink} chars of text)`);
    for (const [u, n] of offHost) {
      errs.push(`NON-LOOPBACK asset (${n}x): ${u} — host is not 127.0.0.1 (cst-0004, dec-0010)`);
    }
    hashes[`${name}-${vp}-${colorScheme}`] = createHash('sha1').update(await readFile(file)).digest('hex');
    if (errs.length) gate.push({ name, vp, colorScheme, errs });
  }
  await ctx.close();
  return file;
}

const pairs = [];
for (const route of ROUTES) {
  for (const [vp, size] of Object.entries(VIEWPORTS)) {
    for (const scheme of SCHEMES) {
      const mock = await capture('mock', route.name, mockBase + route.mockup, vp, scheme, size);
      const served = await capture('ui', route.name, uiBase + route.served, vp, scheme, size);
      pairs.push({ route: route.name, vp, scheme, mock, served });
    }
  }
}
await browser.close();
mockServer.close();
stopUI();

// ---- the fake-dark check on the SERVED pages --------------------------------
for (const route of ROUTES) {
  for (const vp of Object.keys(VIEWPORTS)) {
    const l = hashes[`${route.name}-${vp}-light`], d = hashes[`${route.name}-${vp}-dark`];
    if (l && d && l === d) {
      gate.push({ name: route.name, vp, colorScheme: 'both',
        errs: ['light/dark pair is byte-identical — the served dark scheme is fake'] });
    }
  }
}

// ---- 4/5. report -------------------------------------------------------------
console.log(`uigate — ${pairs.length} pairs, baselines regenerated in this run`);
console.log(`  served from ${uiBase} over ${FIXTURE.replace(ROOT + '/', '')}`);

console.log('\nMECHANICAL GATE (served pages)');
if (gate.length) {
  for (const g of gate) console.log(`  FAIL ${g.name} ${g.vp} ${g.colorScheme}\n    - ${g.errs.join('\n    - ')}`);
} else {
  console.log('  PASS — no console errors, no page errors, no failed requests, no non-loopback assets,');
  console.log('         no blank mount, no fake dark, no layout shift.');
}

console.log('\nPIXEL DIFF (served vs freshly regenerated mockup)');
const diffs = [];
for (const p of pairs) {
  const out = join(OUT, `ui-diff-${p.route}-${p.vp}-${p.scheme}.png`);
  const r = spawnSync(process.execPath, [join(HERE, 'pixeldiff.mjs'), p.mock, p.served, '--out', out],
    { encoding: 'utf8' });
  // pixeldiff writes its verdict to stderr on a hard refusal (a dimension
  // mismatch) and to stdout otherwise. Reading only one of them is how a gate
  // reports "(no output)" and calls it a result.
  const summary = ((r.stdout || '') + (r.stderr || '')).trim().split('\n').filter(Boolean);
  const verdict = summary.find(l => /PIXELDIFF (PASS|FAIL)|dimension mismatch/.test(l)) ?? summary.at(-1) ?? '(no output)';
  const reason = verdict.replace(/^PIXELDIFF (PASS|FAIL)\s*—?\s*/, '').trim();
  const pct = summary.find(l => /differing, delta/.test(l)) ?? summary.find(l => /differing, any delta/.test(l)) ?? '';
  diffs.push({ ...p, code: r.status, verdict, pct });
  const mark = r.status === 0 ? ' ok ' : 'DIFF';
  console.log(`  ${mark} ${p.route.padEnd(12)} ${p.vp.padEnd(7)} ${p.scheme.padEnd(6)} ${(pct.trim() || reason).slice(0, 110)}`);
}

await writeFile(join(OUT, 'ui-gate.json'), JSON.stringify({ gate, diffs }, null, 2));

const overTolerance = diffs.filter(d => d.code !== 0);
console.log(`\n${diffs.length} pairs · ${diffs.length - overTolerance.length} within tolerance · ${overTolerance.length} over`);
if (!KEEP) await rm(work, { recursive: true, force: true });

if (gate.length) { console.log('\nUIGATE FAIL — the served pages did not pass the mechanical gate.'); process.exit(1); }
if (overTolerance.length) {
  console.log('\nUIGATE FAIL — a served page differs from its mockup by more than the recorded tolerance.');
  for (const d of overTolerance) console.log(`  ${d.route} ${d.vp} ${d.scheme}: ${d.verdict.trim()}`);
  process.exit(1);
}
console.log('\nUIGATE PASS — every served page is within tolerance of its freshly regenerated mockup.');
