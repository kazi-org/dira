// gates.mjs — run the design gates. One command, not four remembered ones.
//
//   node docs/design/scripts/gates.mjs              # every gate
//   node docs/design/scripts/gates.mjs --fast       # skip the browser captures
//   node docs/design/scripts/gates.mjs --iter r5    # capture under a given iteration tag
//   node docs/design/scripts/gates.mjs --list
//
// DESIGN.md requires the contrast matrix be re-run "whenever a colour token
// moves". A requirement whose invocation nobody remembers is a requirement that
// stops being met, quietly, at the first busy moment. This is the invocation.
//
// EVERY GATE HERE IS TWO-SIDED. Each one that has a negative control runs the
// control too, and a control that fails to trip is reported as a BROKEN GATE
// rather than a pass — because a checker that cannot fail is indistinguishable
// from a checker that always prints "ok", and this repo has already shipped three
// of those. The exit code separates the two outcomes:
//
//   0  every gate passed and every negative control tripped
//   1  a gate failed
//   3  a gate PASSED but its negative control did not trip — the gate is blind

import { spawn } from 'node:child_process';
import { resolve, dirname } from 'node:path';

const HERE = dirname(new URL(import.meta.url).pathname);
const ROOT = resolve(HERE, '../../..');
const argv = process.argv.slice(2);
const FAST = argv.includes('--fast');
const ITER = (() => { const i = argv.indexOf('--iter'); return i >= 0 ? argv[i + 1] : 'gates'; })();

// expect: 'pass'          — must exit 0
//         'control'       — must exit 1 (the failure IS the proof); exit 3 means blind
const GATES = [
  { id: 'pixeldiff-self-test', browser: false, expect: 'pass',
    is: 'the pixel comparator checks its own arithmetic, each assertion with a known-bad counterpart',
    cmd: ['pixeldiff.mjs', '--self-test'] },

  { id: 'contrast-tokens', browser: false, expect: 'pass',
    is: 'every ink/surface pair clears 4.5:1 and hover exceeds rest, read from tokens.css',
    cmd: ['contrast.mjs'] },
  { id: 'contrast-tokens:control', browser: false, expect: 'control',
    is: 'the pre-r4 --bearing-lift (#b8862f) is restored and must be caught, on the floor AND the hover direction',
    cmd: ['contrast.mjs', '--probe-regression'] },

  { id: 'contrast-rendered', browser: true, expect: 'pass',
    is: 'every text node clears its floor AS COMPOSITED in a browser, including color-mix tints the token matrix cannot see',
    cmd: ['contrast-rendered.mjs'] },

  { id: 'tokens-doc-sync', browser: false, expect: 'pass',
    is: 'DESIGN.md agrees with tokens.css value for value, and the measured tolerance is recorded',
    cmd: ['tokens-doc-sync.mjs'] },

  { id: 'fixture-content', browser: false, expect: 'pass',
    is: 'the design fixture ledger renders the mockups\' content byte for byte, so a pixel diff measures layout and not prose',
    cmd: ['fixture-check.mjs'] },

  { id: 'coherence', browser: false, expect: 'pass',
    is: 'README, product-marketing.md and the landing page still state the canonical strings verbatim',
    cmd: ['check-coherence.mjs'] },

  { id: 'render', browser: true, expect: 'pass',
    is: '3 viewports x 2 schemes: no console errors, no failed requests, no non-loopback assets, no blank mount, no fake dark, no layout shift',
    cmd: ['render.mjs', ITER] },
  { id: 'render:control', browser: true, expect: 'control',
    is: 'a page that SUCCESSFULLY loads an asset from a non-loopback host must be rejected by name',
    cmd: ['render.mjs', `${ITER}-probe`, '--probe-external'] },
];

if (argv.includes('--list')) {
  for (const g of GATES) console.log(`${g.id.padEnd(26)} ${g.browser ? '[browser] ' : '          '}${g.is}`);
  process.exit(0);
}

const run = (cmd) => new Promise(r => {
  const p = spawn(process.execPath, [resolve(HERE, cmd[0]), ...cmd.slice(1)], { cwd: ROOT });
  let out = '';
  p.stdout.on('data', d => (out += d));
  p.stderr.on('data', d => (out += d));
  p.on('close', code => r({ code, out }));
});

const selected = GATES.filter(g => !(FAST && g.browser));
const skipped = GATES.length - selected.length;

console.log(`design gates — ${selected.length} to run${skipped ? `, ${skipped} skipped (--fast)` : ''}\n`);

const results = [];
for (const g of selected) {
  const t0 = Date.now();
  const { code, out } = await run(g.cmd);
  const secs = ((Date.now() - t0) / 1000).toFixed(1);

  let status;
  if (g.expect === 'pass') status = code === 0 ? 'PASS' : 'FAIL';
  else status = code === 1 ? 'CONTROL TRIPPED' : code === 0 || code === 3 ? 'BLIND' : 'FAIL';

  results.push({ ...g, code, out, status, secs });
  const mark = status === 'PASS' || status === 'CONTROL TRIPPED' ? ' ok ' : status === 'BLIND' ? 'BLIND' : 'FAIL';
  console.log(`  ${mark.padEnd(6)} ${g.id.padEnd(26)} ${secs.padStart(6)}s   ${g.is}`);
  if (status === 'FAIL' || status === 'BLIND') {
    console.log(out.split('\n').map(l => `           | ${l}`).join('\n'));
  }
}

const failed = results.filter(r => r.status === 'FAIL');
const blind = results.filter(r => r.status === 'BLIND');

console.log(`\n${results.length} gates run · ${results.filter(r => r.status === 'PASS').length} passed · ` +
  `${results.filter(r => r.status === 'CONTROL TRIPPED').length} negative controls tripped · ` +
  `${failed.length} failed · ${blind.length} blind`);

if (blind.length) {
  console.log(`\nGATES BLIND — ${blind.map(b => b.id).join(', ')}\n` +
    '  A negative control did not trip. The gate it certifies may be reporting success\n' +
    '  unconditionally. Treat its most recent PASS as unproven.');
  process.exit(3);
}
if (failed.length) {
  console.log(`\nGATES FAIL — ${failed.map(f => f.id).join(', ')}`);
  process.exit(1);
}
if (skipped) console.log(`\nGATES PASS (--fast: ${skipped} browser gate(s) NOT run — this is not a full result)`);
else console.log('\nGATES PASS — every gate green, every negative control tripped.');
