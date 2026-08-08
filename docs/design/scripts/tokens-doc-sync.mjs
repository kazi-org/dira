// tokens-doc-sync.mjs — DESIGN.md must agree with tokens.css, value for value.
//
//   node docs/design/scripts/tokens-doc-sync.mjs
//   node docs/design/scripts/tokens-doc-sync.mjs --design <path> --tokens <path>
//
// tokens.css is the declared single source of colour truth, and DESIGN.md
// republishes four of its tokens in the *Hues* table. Two copies of a value, one
// of them prose, is a drift generator: the E6 lane notes recorded the table
// disagreeing with the stylesheet in three of four rows, one of which was the
// pre-r4 --bearing-lift the contrast fix had already replaced. The table has since
// been corrected. An unchecked agreement is a coincidence, so this pins it.
//
// It also enforces the second half of the same problem. docs/plan.md §E6 required
// fidelity "within the pixel tolerance recorded in DESIGN.md" while no tolerance
// was recorded anywhere, which made that clause unfalsifiable. The measured
// tolerance now lives in docs/design/fidelity/tolerance.json, and this script
// requires DESIGN.md to state the same numbers — so the published figure and the
// enforced figure cannot drift apart either.
//
// --design / --tokens exist so the negative control can run against copies. A
// checker that can only be tested by editing the files it guards is a checker
// nobody tests.

import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';

const HERE = dirname(new URL(import.meta.url).pathname);
const arg = (name, fallback) => {
  const i = process.argv.indexOf(name);
  return i >= 0 && process.argv[i + 1] ? process.argv[i + 1] : fallback;
};

const DESIGN_PATH = resolve(arg('--design', resolve(HERE, '../DESIGN.md')));
const TOKENS_PATH = resolve(arg('--tokens', resolve(HERE, '../tokens.css')));
const TOL_PATH = resolve(arg('--tolerance-file', resolve(HERE, '../fidelity/tolerance.json')));

const design = readFileSync(DESIGN_PATH, 'utf8');
const css = readFileSync(TOKENS_PATH, 'utf8');

// ---- tokens.css: the two explicit [data-theme] blocks ------------------------
function themeBlock(name) {
  const m = css.match(new RegExp(`:root\\[data-theme="${name}"\\]\\s*\\{([^}]*)\\}`, 's'));
  if (!m) throw new Error(`no [data-theme="${name}"] block in ${TOKENS_PATH}`);
  const out = {};
  for (const decl of m[1].split(';')) {
    const d = decl.match(/--([a-z-]+)\s*:\s*(#[0-9a-fA-F]{3,8})/);
    if (d) out[d[1]] = d[2].toLowerCase();
  }
  return out;
}
const SCHEME = { light: themeBlock('light'), dark: themeBlock('dark') };

// ---- DESIGN.md: the Hues table ----------------------------------------------
// | `--token` | meaning | `#light` | `#dark` |
const ROW = /^\|\s*`--([a-z-]+)`\s*\|[^|]*\|\s*`?(#[0-9a-fA-F]{3,8})`?\s*\|\s*`?(#[0-9a-fA-F]{3,8})`?\s*\|/gm;
const documented = [...design.matchAll(ROW)].map(m => ({
  token: m[1], light: m[2].toLowerCase(), dark: m[3].toLowerCase(),
}));

const failures = [];

if (!documented.length) {
  failures.push(`DESIGN.md declares no hue table rows — the "| \`--token\` | meaning | light | dark |" ` +
    `shape this check reads is gone or was reformatted. That is a change to the document's contract, not a passing state.`);
}

for (const row of documented) {
  for (const scheme of ['light', 'dark']) {
    const inCss = SCHEME[scheme][row.token];
    if (!inCss) {
      failures.push(`--${row.token} is documented in DESIGN.md but absent from the [data-theme="${scheme}"] block in tokens.css`);
    } else if (inCss !== row[scheme]) {
      failures.push(`--${row.token} ${scheme}: DESIGN.md says ${row[scheme]}, tokens.css says ${inCss} ` +
        `— tokens.css is the declared single source, so DESIGN.md is the one that is wrong`);
    }
  }
}

// Hue-budget tokens must ALL be documented. Without this, deleting a row from the
// table would make the check pass by having nothing left to disagree about.
const HUES = ['bearing', 'bearing-lift', 'converged', 'caught'];
for (const h of HUES) {
  if (!documented.some(r => r.token === h)) {
    failures.push(`--${h} is in the hue budget but has no row in DESIGN.md's hue table ` +
      `(a table can also drift by omission, which is why this is checked separately)`);
  }
}

// ---- the recorded pixel tolerance -------------------------------------------
let tol = null;
try { tol = JSON.parse(readFileSync(TOL_PATH, 'utf8')); } catch {}

const tolerance = { checked: 0, missing: [] };
if (!tol) {
  failures.push(`no measured tolerance at ${TOL_PATH} — run: node docs/design/scripts/measure-tolerance.mjs --write`);
} else {
  // Each number must appear in DESIGN.md verbatim. Verbatim rather than "a
  // number is present": a document that says "about 0.1%" while the gate enforces
  // 0.00033% is the same defect as the stale hue table, one decimal place larger.
  // THE CANONICAL LINE. All three figures are checked as one exact sentence
  // generated from tolerance.json, rather than three independent substring
  // searches. Three separate searches pass as long as each number appears
  // SOMEWHERE, which a document can satisfy while still stating them in a
  // sentence that means something else. One generated line has a single truth
  // condition, and a failure can print the exact string the document must carry.
  const canonical = `**\`${tol.pixel_tolerance_pct}%\` of pixels, at a channel threshold of ` +
    `\`${tol.channel_threshold}/255\`, with no ${tol.block_px}×${tol.block_px} block more than ` +
    `\`${tol.block_tolerance_pct}%\` changed.**`;

  // Compared against the document with whitespace collapsed. Prose wraps: the
  // canonical sentence sits across two lines in DESIGN.md, so a literal
  // `includes` could never match it and the check failed on a document that was
  // in fact correct. Caught by running the break-and-restore proof, where step
  // one — the untouched baseline — came back red. Same normalization
  // check-coherence.mjs uses, and for the same reason.
  const flat = s => s.replace(/\s+/g, ' ');
  const designFlat = flat(design);

  tolerance.checked++;
  if (!designFlat.includes(flat(canonical))) {
    // Distinguish "never written" from "written and then drifted" — they need
    // different fixes, and saying "missing" about a line that is present but
    // wrong sends the reader looking in the wrong place.
    // Anchored on the opening `**`, not on "any run of non-full-stops": the
    // figure it is quoting is a decimal, so a [^.]* prefix clipped it to
    // "0005%" and reported a corrupted excerpt of a real line.
    const near = designFlat.match(/\*\*`[^`]*` of pixels, at a channel threshold of[^*]*\*\*/)?.[0];
    failures.push(
      `DESIGN.md does not carry the canonical tolerance line from docs/design/fidelity/tolerance.json.\n` +
      `      expected: ${canonical}\n` +
      (near ? `      found:    ${near.trim()}\n      -> the line is present but its numbers have drifted from the measured ones.\n`
            : `      found:    (no line of this shape anywhere in the file)\n      -> the line was never written, or was deleted.\n`) +
      `      docs/plan.md §E6 cites "the pixel tolerance recorded in DESIGN.md"; the enforced number\n` +
      `      lives in tolerance.json, so a document that disagrees with it makes that clause unfalsifiable again.`);
  }

  // Kept alongside the canonical line: each figure WITH ITS UNIT. A bare
  // `design.includes("4")` passed trivially — "r4", "4.5:1" and "42 pairs" all
  // satisfy it — which would have made the channel-threshold clause exactly the
  // kind of always-true predicate this lane exists to remove.
  const needed = [
    ['pixel tolerance', `${tol.pixel_tolerance_pct}%`],
    ['block tolerance', `${tol.block_tolerance_pct}%`],
    ['channel threshold', `${tol.channel_threshold}/255`],
  ];
  for (const [label, value] of needed) {
    tolerance.checked++;
    if (!design.includes(value)) tolerance.missing.push(`${label} (${value})`);
  }
  if (tolerance.missing.length) {
    failures.push(`DESIGN.md never states the measured ${tolerance.missing.join(', ')} anywhere in the file.`);
  }
  if (!/pixeldiff\.mjs/.test(design)) {
    failures.push(`DESIGN.md's Verification section does not mention pixeldiff.mjs — the tolerance is ` +
      `meaningless without the command that measures against it`);
  }
}

// ---- report ------------------------------------------------------------------
console.log(`${documented.length} documented hue rows checked against tokens.css (${documented.length * 2} values), ` +
  `plus ${tolerance.checked} recorded tolerance figures.`);
for (const row of documented) {
  const okL = SCHEME.light[row.token] === row.light, okD = SCHEME.dark[row.token] === row.dark;
  console.log(`  ${okL && okD ? 'ok  ' : 'DRIFT'} --${row.token.padEnd(13)} light ${row.light} ${okL ? '==' : '!='} ${SCHEME.light[row.token] ?? '(absent)'}` +
    `   dark ${row.dark} ${okD ? '==' : '!='} ${SCHEME.dark[row.token] ?? '(absent)'}`);
}

console.log(`\n${failures.length} failures`);
if (failures.length) {
  console.log('\nTOKEN/DOC SYNC FAIL:');
  for (const f of failures) console.log(`  - ${f}`);
  process.exit(1);
}
console.log('TOKEN/DOC SYNC PASS — DESIGN.md and tokens.css agree value for value, and the measured tolerance is recorded.');
