// fixture-check.mjs — the design fixture ledger must render the mockups' content.
//
//   node docs/design/scripts/fixture-check.mjs [-v]
//
// A pixel comparison between a Go-served page and a mockup is only meaningful if
// both render THE SAME CONTENT. Otherwise the tolerance is measuring prose, and
// the first time somebody reworded a heading the gate would report a fidelity
// regression that is really a copy edit — or, worse, a real layout regression
// would be dismissed as "just the fixture text differing".
//
// So every string the fixture and the mockups share is asserted byte-equal here,
// after HTML tags are stripped and entities decoded. What is NOT asserted is
// listed explicitly below as a declared exception with its reason: an unstated
// exclusion is how a check quietly stops covering the thing it was written for.
//
// Exit 0 when every expected string matches; 1 on any mismatch or any exception
// that has silently become checkable (an exception nobody revisits is a hole).

import { readFileSync, readdirSync } from 'node:fs';
import { resolve, dirname, join } from 'node:path';

const HERE = dirname(new URL(import.meta.url).pathname);
const FIXTURES = resolve(HERE, '../fidelity/fixtures/ledger-design/entries');
const SCREENS = resolve(HERE, '../screens');
const VERBOSE = process.argv.includes('-v');

// ---- read the mockups, normalized to plain text ------------------------------
const normalize = html => html
  .replace(/<style[\s\S]*?<\/style>/gi, ' ')
  .replace(/<!--[\s\S]*?-->/g, ' ')
  .replace(/<[^>]+>/g, ' ')
  .replace(/&rsquo;/g, '’').replace(/&lsquo;/g, '‘')
  .replace(/&mdash;/g, '—').replace(/&ndash;/g, '–')
  .replace(/&nbsp;/g, ' ').replace(/&amp;/g, '&')
  .replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&quot;/g, '"')
  .replace(/\s+/g, ' ')
  .trim();

const screens = {};
for (const f of readdirSync(SCREENS).filter(f => f.endsWith('.html'))) {
  screens[f] = normalize(readFileSync(join(SCREENS, f), 'utf8'));
}

// ---- read the fixture entries ------------------------------------------------
// The emitter writes every string as a double-quoted flow scalar, so this reads
// exactly the shape it writes. It deliberately does NOT try to be a YAML parser:
// a half-correct one would fail open on the fields it did not understand.
const unq = s => s.replace(/\\"/g, '"').replace(/\\\\/g, '\\');
function readEntry(path) {
  const raw = readFileSync(path, 'utf8');
  const m = raw.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/);
  if (!m) throw new Error(`${path}: no frontmatter`);
  const [, front, body] = m;
  const e = { because: body.trim(), alternatives: [] };
  e.id = front.match(/^id:\s*(\S+)/m)?.[1];
  e.kind = front.match(/^kind:\s*(\S+)/m)?.[1];
  e.state = front.match(/^state:\s*(\S+)/m)?.[1];
  e.title = unq(front.match(/^title:\s*"((?:[^"\\]|\\.)*)"/m)?.[1] ?? '');
  // No /m flag, deliberately. Under /m the `$` alternative matches the end of
  // EVERY line, so the lazy quantifier stopped at the first newline and the block
  // came back one line long — every why_not silently empty, and an empty string
  // then "matches" nothing and reported as missing. Without /m, `$` means end of
  // input, which is what the alternation intends.
  const altBlock = ('\n' + front).match(/\nalternatives:\n([\s\S]*?)(?=\n[a-z_]+:|$)/)?.[1] ?? '';
  for (const chunk of altBlock.split(/\n(?=  - option:)/)) {
    const option = chunk.match(/option:\s*"((?:[^"\\]|\\.)*)"/)?.[1];
    if (!option) continue;
    e.alternatives.push({
      option: unq(option),
      why_not: unq(chunk.match(/why_not:\s*"((?:[^"\\]|\\.)*)"/)?.[1] ?? ''),
      revisit_if: unq(chunk.match(/revisit_if:\s*"((?:[^"\\]|\\.)*)"/)?.[1] ?? ''),
    });
  }
  return e;
}

const entries = readdirSync(FIXTURES).filter(f => f.endsWith('.md')).sort()
  .map(f => readEntry(join(FIXTURES, f)));

// ---- what the mockups actually render ----------------------------------------
// The mockups are three pages, not eighteen. Most entries appear only as an index
// row, which renders the title and nothing else. Only dec-0001 has a full
// decision page, and only the three distill entries have a card.
const FULL_PAGE = new Set(['dec-0001']);          // s1-decision.html
const DISTILL = new Set(['dec-0011', 'dec-0012', 'qst-0006']);  // s3-distill.html

// Declared exceptions. Each names WHY the string is not rendered anywhere. The
// script re-checks them: if an exception's string turns out to be present after
// all, that is reported as a failure, because a stale exception is a hole.
const EXCEPTIONS = [
  ['qst-0006', 'title',
   'the distill card shows the blocked target ("Blocks · dec-0006 — the fractal tier design") ' +
   'where a decision card shows a title, so this question\'s title is never rendered in the mockups'],
  ['int-0004', 'title',
   'the orphan is referenced by id only — s2-index\'s drift row and s1-decision\'s flag card both ' +
   'name int-0004 without its title, which is the point: it is work with no stated purpose'],
  ['int-0004', 'because',
   'same reason — the orphan\'s body is never shown, only the fact that it has no parent'],
];
const isException = (id, field) => EXCEPTIONS.find(e => e[0] === id && e[1] === field);

// ---- check --------------------------------------------------------------------
const found = s => Object.entries(screens).find(([, text]) => text.includes(s))?.[0];

const results = [];
const check = (id, field, value) => {
  const ex = isException(id, field);
  const where = value ? found(value) : null;
  if (ex) {
    results.push({ id, field, value, where,
      status: where ? 'STALE-EXCEPTION' : 'exempt', reason: ex[2] });
  } else {
    results.push({ id, field, value, where, status: where ? 'ok' : 'MISSING' });
  }
};

for (const e of entries) {
  check(e.id, 'title', e.title);
  if (FULL_PAGE.has(e.id) || DISTILL.has(e.id)) check(e.id, 'because', e.because);
  if (isException(e.id, 'because')) check(e.id, 'because', e.because);
  if (FULL_PAGE.has(e.id)) {
    for (const [i, a] of e.alternatives.entries()) {
      check(e.id, `alternatives[${i}].option`, a.option);
      check(e.id, `alternatives[${i}].why_not`, a.why_not);
      if (a.revisit_if) check(e.id, `alternatives[${i}].revisit_if`, a.revisit_if);
    }
  }
}

// ---- report --------------------------------------------------------------------
const bad = results.filter(r => r.status === 'MISSING' || r.status === 'STALE-EXCEPTION');
const ok = results.filter(r => r.status === 'ok');
const exempt = results.filter(r => r.status === 'exempt');

if (VERBOSE) for (const r of ok) console.log(`  ok    ${r.id} ${r.field.padEnd(26)} -> ${r.where}`);

console.log(`${entries.length} fixture entries · ${results.length} strings checked against ${Object.keys(screens).length} mockups`);
console.log(`  ${ok.length} byte-equal · ${exempt.length} declared not-rendered · ${bad.length} failures`);

if (exempt.length) {
  console.log('\ndeclared exceptions (not rendered in any mockup, and re-checked every run):');
  for (const r of exempt) console.log(`  - ${r.id} ${r.field}: ${r.reason}`);
}

// ---- the unproducible-card check (dec-0019) ----------------------------------
// This was a FINDING printed on every run: s1-decision.html rendered a fourth
// alternative card for the UPHELD option, and entry.schema.json models
// `alternatives` as the roads NOT taken, so no ledger could ever produce it.
// dec-0019 resolved it by removing the card — the ruling carries the chosen road
// — and a resolved finding that nothing re-checks is a finding that comes back.
//
// So the finding is now an assertion, scoped to the two places the card lived:
// an alternative's state tag/mark, and the chain's refusal column. It is NOT a
// blanket ban on the character: `.res-oriented` on s1-decision-withheld.html
// legitimately marks the ORIENTED resolution state with a ✓ (dec-0011,
// dec-0018), which is a fact about an edge and not a claim about an alternative.
//
// Comments are stripped first, because the three screens explain in a comment
// why the card is gone and a checker that reads its own explanation as a
// violation would be unfixable.
const stripComments = html => html.replace(/<!--[\s\S]*?-->/g, ' ');
const UNPRODUCIBLE = [
  [/class="alt[^"]*\bupheld\b/, 'an <details class="alt upheld"> card'],
  [/<span class="tag">\s*upheld\s*<\/span>/, 'an "upheld" state tag on an alternative'],
  [/<span class="mark">\s*✓/, 'a ✓ mark on an alternative summary'],
  [/<span class="ok">\s*✓/, 'a ✓ row in the .chain block'],
  [/<span class="m ok">\s*✓/, 'a ✓ row in the .chain-stack block'],
];
const unproducible = [];
for (const f of readdirSync(SCREENS).filter(f => f.endsWith('.html'))) {
  const raw = stripComments(readFileSync(join(SCREENS, f), 'utf8'));
  for (const [re, what] of UNPRODUCIBLE) {
    if (re.test(raw)) unproducible.push(`  - ${f}: ${what} — no ledger field produces this (dec-0019)`);
  }
}
if (unproducible.length) {
  console.log('\nUNPRODUCIBLE MARKUP FAIL — a screen renders a card the schema has no field for:');
  for (const u of unproducible) console.log(u);
  console.log('  `alternatives` records the roads NOT taken. The chosen road is the <h1 class="ruling">,');
  console.log('  and `dira why` prints ✗ lines and never a ✓. See .dira/entries/dec-0019.md.');
  process.exit(1);
}
console.log(`  ${UNPRODUCIBLE.length} unproducible-markup patterns absent from all ` +
  `${Object.keys(screens).length} screens (dec-0019)`);

if (bad.length) {
  console.log('\nFIXTURE CONTENT FAIL:');
  for (const r of bad) {
    if (r.status === 'MISSING') {
      console.log(`  - ${r.id} ${r.field}: not present verbatim in any mockup\n      ${JSON.stringify(r.value.slice(0, 120))}`);
    } else {
      console.log(`  - ${r.id} ${r.field}: declared not-rendered, but it IS rendered in ${r.where}. ` +
        `The exception is stale — delete it rather than leaving a string unchecked.`);
    }
  }
  process.exit(1);
}
console.log('\nFIXTURE CONTENT PASS — every rendered fixture string is byte-equal to the mockups.');
