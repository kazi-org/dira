// lint-openq.mjs — the token-discipline gate for the study files.
//
//   node docs/design/openq/scripts/lint-openq.mjs
//
// DESIGN.md's rule is that a hardcoded hex or px in a screen is a defect, with
// one sanctioned exception (<meta name="theme-color">, which cannot reference a
// custom property). It states a grep for hex but has never had one for the type
// scale, which is the rule this study is most likely to break: 20 alternatives
// and a 400-word paragraph is exactly the situation that tempts a one-off size.
//
// Three checks:
//   1. no hex outside the two theme-color values
//   2. every font-size is one of the nine scale tokens
//   3. every max-width is a measure token or a ch value (the ceilings apply to
//      the study just as they do to the screens)
//
// Hairline geometry in px (1px rules, 2px accent bars, 3px hatch stops) is
// allowed, exactly as it already is in screens/s1-decision.html — the rule is
// about the type scale and the palette, not about whether a border can be a
// border.

import { readFile, readdir } from 'node:fs/promises';
import { resolve, join } from 'node:path';

const DIR = resolve(import.meta.dirname, '..');
const ALLOWED_HEX = new Set(['#0f151c', '#f7f4ed']);
const SIZE_TOKENS = new Set([
  '--t-display', '--t-lede', '--t-body', '--t-small',
  '--t-ui', '--t-sub', '--t-label', '--t-mono', '--t-chip',
]);

const files = (await readdir(DIR)).filter(f => f.endsWith('.html') || f.endsWith('.css'));
const fails = [];

for (const f of files) {
  const src = await readFile(join(DIR, f), 'utf8');
  const lines = src.split('\n');

  lines.forEach((line, i) => {
    // (?<!&) so that HTML numeric entities — &#10007; is the refusal mark, and
    // it is TEXT, which is the whole point of Law 3 — are not read as colours.
    for (const hex of line.match(/(?<!&)#[0-9a-fA-F]{3,8}\b/g) ?? []) {
      if (!ALLOWED_HEX.has(hex.toLowerCase())) fails.push(`${f}:${i + 1} hardcoded hex ${hex}`);
    }
    if (line.includes('@media')) return;   // breakpoints are not text measures
    for (const m of line.matchAll(/font-size:\s*([^;{}"']+)/g)) {
      const v = m[1].trim();
      const tok = v.match(/var\((--[a-z-]+)\)/);
      if (!tok || !SIZE_TOKENS.has(tok[1])) fails.push(`${f}:${i + 1} off-scale font-size "${v}"`);
    }
    for (const m of line.matchAll(/max-width:\s*([^;{}"']+)/g)) {
      const v = m[1].trim();
      // 1100px is the page shell, identical to screens/s1-decision.html — a
      // container width, not a line length. Every TEXT block must be a ceiling.
      if (!/var\(--m-/.test(v) && !/^\d+(\.\d+)?ch$/.test(v) && !/^100%$/.test(v) && v !== '1100px') {
        fails.push(`${f}:${i + 1} max-width "${v}" is neither a measure token nor a ch ceiling`);
      }
    }
  });
}

console.log(`linted ${files.length} file(s) in docs/design/openq/`);
if (fails.length) {
  console.log(`\nLINT FAIL — ${fails.length}:`);
  for (const x of fails) console.log('  ' + x);
  process.exit(1);
}
console.log('LINT PASS — no stray hex, every font-size on the 9-size scale, every measure a ceiling.');
