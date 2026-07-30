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
//   node docs/design/scripts/contrast.mjs        # non-zero on any failure
//   node docs/design/scripts/contrast.mjs -v     # print the full matrix
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
  // hover must exceed rest, in whichever direction this scheme needs
  for (const surf of SURFACES) {
    const rest = ratio(T['bearing'], T[surf]);
    const hover = ratio(T['bearing-lift'], T[surf]);
    if (!(hover > rest)) {
      failures.push(`${scheme}: hover does NOT exceed rest on --${surf} `
        + `(rest ${rest.toFixed(2)} -> hover ${hover.toFixed(2)}) — WEB.md §2`);
    }
    if (VERBOSE) console.log(`    hover>rest on ${surf}: ${rest.toFixed(2)} -> ${hover.toFixed(2)}`
      + `${hover > rest ? '' : '  INVERTED'}`);
  }
}

console.log(`\n${pairs} ink/surface pairs checked across 2 schemes, plus 6 hover>rest assertions.`);
if (failures.length) {
  console.log(`\nCONTRAST FAIL — ${failures.length} violation(s):`);
  for (const f of failures) console.log(`  ${f}`);
  process.exit(1);
}
console.log('CONTRAST PASS — every pair clears 4.5:1 and hover exceeds rest in both schemes.');
