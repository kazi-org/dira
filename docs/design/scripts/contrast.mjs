// contrast.mjs — the standing contrast matrix for dira's tokens.
//
// DESIGN.md requires this be re-run whenever a colour token moves. Until this file
// existed that requirement was unmeetable: the matrix had only ever been computed
// ad hoc in a shell during the design phase, so the document asserted a test that
// did not exist.
//
// It parses tokens.css rather than carrying its own copy of the values, so it cannot
// drift from the tokens it checks. A hardcoded palette here would be the same class
// of bug as the DESIGN.md table that kept listing pre-fix values.
//
//   node docs/design/scripts/contrast.mjs                     # non-zero on any failure
//   node docs/design/scripts/contrast.mjs -v                  # print the full matrix
//   node docs/design/scripts/contrast.mjs --probe-regression  # the negative control
//
// The negative control substitutes the PRE-r4 light --bearing-lift (#b8862f) and
// requires the script to catch it: at least one pair under the floor and at least
// one hover<rest inversion. Exit codes are distinct because "found failures" and
// "found nothing when it should have" are opposite outcomes that both exit
// non-zero, and a runner that cannot tell them apart cannot certify the gate:
//
//   0  no failures                     (a normal, passing run)
//   1  failures found                  (a normal failing run; the EXPECTED probe result)
//   3  the probe caught nothing        (the gate is broken — only reachable under --probe-regression)
//
// Two rules, both from WEB.md §2:
//   1. every ink/accent on every surface clears WCAG 4.5:1 for normal text
//   2. hover/focus carries MORE contrast than rest — and because "more contrast"
//      means darker in light and lighter in dark, --bearing-lift inverts direction
//      per scheme. A single lift direction cannot serve both; that inversion was a
//      real shipped defect (r3 -> r4) and this check is what pins it.

import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';

const HERE = dirname(new URL(import.meta.url).pathname);
const CSS = readFileSync(resolve(HERE, '../tokens.css'), 'utf8');
const VERBOSE = process.argv.includes('-v');

// ---- parse the two explicit [data-theme] blocks -----------------------------
// Those blocks are authoritative: they exist precisely so a theme toggle wins in
// both directions, which means they must carry the full palette for each scheme.
function themeBlock(name) {
  const m = CSS.match(new RegExp(`:root\\[data-theme="${name}"\\]\\s*\\{([^}]*)\\}`, 's'));
  if (!m) throw new Error(`no [data-theme="${name}"] block in tokens.css`);
  const out = {};
  for (const decl of m[1].split(';')) {
    const d = decl.match(/--([a-z-]+)\s*:\s*(#[0-9a-fA-F]{3,8})/);
    if (d) out[d[1]] = d[2];
  }
  return out;
}

const SCHEMES = { light: themeBlock('light'), dark: themeBlock('dark') };
const SURFACES = ['ground', 'panel', 'sunk'];
const INKS = ['ink', 'ink-mid', 'ink-low', 'bearing', 'bearing-lift', 'converged', 'caught'];
const FLOOR = 4.5;

// ---- the negative control ---------------------------------------------------
// #b8862f is the value --bearing-lift actually shipped with before r4. It is not
// a made-up bad colour: it is the defect this script was written to have caught,
// restored. DESIGN.md's r3 -> r4 section records what it scored (2.95 / 3.18 /
// 2.62 in light) and that hover came out BELOW rest, inverting WEB.md 2.
const PROBE = process.argv.includes('--probe-regression');
const PRE_R4_BEARING_LIFT = '#b8862f';
if (PROBE) {
  SCHEMES.light['bearing-lift'] = PRE_R4_BEARING_LIFT;
  console.log(`--probe-regression: light --bearing-lift restored to the pre-r4 value ${PRE_R4_BEARING_LIFT}.`);
  console.log('  Expecting this script to report contrast failures AND a hover<rest inversion.');
}

// ---- WCAG 2.x relative luminance -------------------------------------------
const chan = c => { c /= 255; return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4; };
function lum(hex) {
  let h = hex.replace('#', '');
  if (h.length === 3) h = [...h].map(c => c + c).join('');
  const [r, g, b] = [0, 2, 4].map(i => parseInt(h.slice(i, i + 2), 16));
  return 0.2126 * chan(r) + 0.7152 * chan(g) + 0.0722 * chan(b);
}
const ratio = (a, b) => {
  const [x, y] = [lum(a), lum(b)];
  return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
};

// ---- run --------------------------------------------------------------------
const failures = [];
const inversions = [];
const hoverLines = [];
let pairs = 0;

for (const [scheme, T] of Object.entries(SCHEMES)) {
  if (VERBOSE) console.log(`\n=== ${scheme.toUpperCase()} ===`);
  for (const ink of INKS) {
    const cells = [];
    for (const surf of SURFACES) {
      if (!T[ink] || !T[surf]) {
        failures.push(`${scheme}: token --${!T[ink] ? ink : surf} missing from the [data-theme] block`);
        continue;
      }
      const r = ratio(T[ink], T[surf]);
      pairs++;
      if (r < FLOOR) failures.push(`${scheme}: --${ink} on --${surf} is ${r.toFixed(2)}:1 (floor ${FLOOR})`);
      cells.push(`${surf}:${r.toFixed(2)}${r < FLOOR ? ' FAIL' : ''}`);
    }
    if (VERBOSE) console.log(`  ${ink.padEnd(14)} ${cells.join('  ')}`);
  }
  // hover must exceed rest, in whichever direction this scheme needs.
  // Printed unconditionally, not behind -v: an assertion nobody sees the result
  // of is an assertion nobody checks. All six surface x scheme combinations are
  // named on their own line, with the ratio that produced the verdict.
  for (const surf of SURFACES) {
    const rest = ratio(T['bearing'], T[surf]);
    const hover = ratio(T['bearing-lift'], T[surf]);
    const held = hover > rest;
    if (!held) {
      const msg = `${scheme}: hover does NOT exceed rest on --${surf} `
        + `(rest ${rest.toFixed(2)} -> hover ${hover.toFixed(2)}) — WEB.md §2`;
      failures.push(msg);
      inversions.push(msg);
    }
    hoverLines.push(`  ${held ? 'hover > rest' : 'hover < rest  INVERTED'}  ${scheme.padEnd(5)} on --${surf.padEnd(6)} `
      + `rest ${rest.toFixed(2)}:1 -> hover ${hover.toFixed(2)}:1`);
  }
}

console.log(`\n${pairs} ink/surface pairs checked across 2 schemes, plus 6 hover>rest assertions.`);
console.log(hoverLines.join('\n'));
console.log(`\n${failures.length} failures`);

if (PROBE) {
  // Two-sided: the probe must produce BOTH a floor violation and an inversion.
  // A probe that only trips one of them would leave the other check uncertified.
  const floorFails = failures.length - inversions.length;
  console.log(`\nPROBE RESULT — ${floorFails} contrast-floor violation(s), ${inversions.length} hover inversion(s):`);
  for (const f of failures) console.log(`  ${f}`);
  if (floorFails < 1 || inversions.length < 1) {
    console.log('\nPROBE BROKEN — the pre-r4 regression was restored and this script did not catch it' +
      (floorFails < 1 ? ' (no floor violation)' : '') + (inversions.length < 1 ? ' (no inversion)' : '') +
      '.\n  The matrix is not measuring what it claims to measure.');
    process.exit(3);
  }
  console.log('\nPROBE OK — the known regression is caught, on both the floor and the hover direction.');
  process.exit(1);
}

if (failures.length) {
  console.log(`\nCONTRAST FAIL — ${failures.length} violation(s):`);
  for (const f of failures) console.log(`  ${f}`);
  process.exit(1);
}
console.log('CONTRAST PASS — every pair clears 4.5:1 and hover exceeds rest in both schemes.');
