// check-coherence.mjs — README <-> .agents/product-marketing.md <-> landing page.
//
// Precedent: kazi/site/scripts/check-coherence.mjs (ADR-0018) — canonical strings
// live in one module and CI fails the moment a surface repeats them with a
// changed word. This is the same pattern, extended to two source-of-truth
// documents instead of one, because dira has a README (the product's own voice)
// and a separate marketing doc (the settled positioning) that must not silently
// diverge from each other OR from the page that quotes both.
//
// Each canonical string in ./landing/canonical.mjs declares which document(s)
// it must appear in verbatim. Not every string is checked against both source
// docs — some things (the category bet) are only ever stated in the marketing
// doc, and asserting them against README would be checking a document against
// words it was never written to contain. Asserting a false three-way match
// would be worse than the narrower, honest check this does instead.
//
//   node docs/design/scripts/check-coherence.mjs
//
import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';

import {
  HOOK, TAGLINE, NO_BINARY, INSTALL_LINE, CATEGORY,
} from '../landing/canonical.mjs';

const HERE = dirname(new URL(import.meta.url).pathname);
const ROOT = resolve(HERE, '../../..');

const README_PATH = resolve(ROOT, 'README.md');
const MARKETING_PATH = resolve(ROOT, '.agents/product-marketing.md');
const PAGE_PATH = resolve(ROOT, 'docs/design/landing/index.html');

// ---- normalizers -------------------------------------------------------------
// Markdown source: strip blockquote markers, collapse a hard <br> to a space
// (the README centers its hook across two visual lines with one), drop
// backticks and ** bold markers, then collapse all whitespace/newlines to
// single spaces so a sentence broken across lines still reads as one string.
function normalizeMarkdown(text) {
  return text
    .replace(/<br\s*\/?>/gi, ' ')
    .replace(/^\s*>\s?/gm, '')
    .replace(/`/g, '')
    .replace(/\*\*/g, '')
    .replace(/\s+/g, ' ')
    .trim();
}

// HTML source (the rendered page): strip every tag, decode the handful of
// entities this system's screens actually use, collapse whitespace. A phrase
// split across inline elements (e.g. a <b> inside the wordmark) must still
// read as continuous text, which is why tags are stripped rather than kept.
function normalizeHtml(text) {
  return text
    .replace(/<style[\s\S]*?<\/style>/gi, ' ')
    .replace(/<[^>]+>/g, ' ')
    .replace(/&rsquo;/g, '’')
    .replace(/&mdash;/g, '—')
    .replace(/&amp;/g, '&')
    .replace(/&nbsp;/g, ' ')
    .replace(/\s+/g, ' ')
    .replace(/\s+([,.;:])/g, '$1') // "log , no" -> "log, no" (an inline tag
    // like </code> stripped to a space before punctuation must not count as a
    // real word-break the source markdown never had)
    .trim();
}

let readme, marketing, page;
try {
  readme = normalizeMarkdown(readFileSync(README_PATH, 'utf8'));
} catch {
  console.error(`COHERENCE FAIL — cannot read ${README_PATH}`);
  process.exit(1);
}
try {
  marketing = normalizeMarkdown(readFileSync(MARKETING_PATH, 'utf8'));
} catch {
  console.error(`COHERENCE FAIL — cannot read ${MARKETING_PATH}`);
  process.exit(1);
}
try {
  page = normalizeHtml(readFileSync(PAGE_PATH, 'utf8'));
} catch {
  console.error(`COHERENCE FAIL — cannot read ${PAGE_PATH} (has the landing page been built yet?)`);
  process.exit(1);
}

const DOCS = { README: readme, MARKETING: marketing, PAGE: page };
const DOC_LABEL = { README: 'README.md', MARKETING: '.agents/product-marketing.md', PAGE: 'docs/design/landing/index.html' };

// ---- the checks ---------------------------------------------------------------
const CHECKS = [
  ['hook', HOOK, ['README', 'MARKETING', 'PAGE']],
  ['tagline', TAGLINE, ['README', 'MARKETING', 'PAGE']],
  ['no-binary status line', NO_BINARY, ['README', 'PAGE']],
  ['install line', INSTALL_LINE, ['README', 'PAGE']],
  ['category sentence', CATEGORY, ['MARKETING', 'PAGE']],
];

const failures = [];
for (const [name, value, sources] of CHECKS) {
  for (const source of sources) {
    if (!DOCS[source].includes(value)) {
      failures.push(`${name} — missing from ${DOC_LABEL[source]}\n      expected verbatim: ${JSON.stringify(value)}`);
    }
  }
}

console.log(`${CHECKS.length} canonical strings checked across ${Object.keys(DOC_LABEL).length} surfaces.`);
if (failures.length) {
  console.log(`\nCOHERENCE FAIL — ${failures.length} violation(s):`);
  for (const f of failures) console.log(`  - ${f}`);
  console.log('\nFix: edit canonical.mjs and the diverging surface together — never just the checker.');
  process.exit(1);
}
console.log('COHERENCE PASS — hook, tagline, install line, no-binary status line, and category sentence all agree across README, product-marketing.md, and the landing page.');
