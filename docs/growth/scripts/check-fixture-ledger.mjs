#!/usr/bin/env node
// docs/growth/scripts/check-fixture-ledger.mjs
//
// E8-L3-T2 (docs/plan/tasks/E8-L3.md). Zero third-party dependencies, per the
// lane's rule -- E0 owns real JSON-Schema validation in Go; this checker only
// verifies the required fields and the closed `kind` enum, read directly out
// of schema/entry.schema.json's literal values rather than hand-copied, plus
// the one property the demo depends on: dec-0060's alternatives[] each carry
// a non-empty why_not (without that, `dira check`'s output in
// .agents/product-marketing.md §6 quotes a field the fixture doesn't have).
//
// This does NOT try to be a YAML parser. It reads only unindented top-level
// `key: value` lines out of the frontmatter and one hand-rolled block for
// `alternatives`, the same tradeoff docs/design/scripts/fixture-check.mjs
// makes and explains: a half-correct general parser would fail open on the
// fields it did not understand.
//
// Usage: node check-fixture-ledger.mjs [dir]   (default: fixtures/demo-ledger)
// Exit 0: every *.md file in dir has the required fields, a kind in the
//         closed enum, and (if it is dec-0060) alternatives[].why_not all
//         non-empty. Prints the file count checked.
// Exit 1: at least one violation. Every violation is named: file + reason.

import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join, resolve, basename } from 'node:path';

const HERE = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(HERE, '..', '..', '..');
const SCHEMA_PATH = join(ROOT, 'schema', 'entry.schema.json');

export function loadSchemaContract(schemaPath = SCHEMA_PATH) {
  const schema = JSON.parse(readFileSync(schemaPath, 'utf8'));
  return {
    required: schema.required,
    kindEnum: schema.properties.kind.enum,
  };
}

// Parse only the unindented `key: value` lines of a frontmatter block -- an
// indented line belongs to a nested structure (edges, alternatives, source)
// and is deliberately not surfaced here as a top-level field.
function parseTopLevelFields(frontmatter) {
  const fields = {};
  for (const line of frontmatter.split('\n')) {
    if (/^\s/.test(line) || line.trim() === '') continue;
    const m = line.match(/^([a-z_]+):\s*(.*)$/);
    if (m) fields[m[1]] = m[2].trim();
  }
  return fields;
}

// alternatives[] entries start each with a `  - option:` line at 2-space
// indent. A why_not is "present" either as inline text after the colon, or
// (the actual shape dec-0060.md uses) a `>` folded block scalar followed by
// non-blank, more-indented continuation lines.
function parseAlternatives(frontmatter) {
  const blockMatch = ('\n' + frontmatter).match(/\nalternatives:\n([\s\S]*?)(?=\n[a-z_]+:|$)/);
  if (!blockMatch) return null; // key absent entirely
  const block = blockMatch[1];
  const chunks = block.split(/\n(?=  - option:)/).filter((c) => c.trim() !== '');
  return chunks.map((chunk) => {
    const optionMatch = chunk.match(/option:\s*(.*)$/m);
    const whyNotKeyMatch = chunk.match(/^\s*why_not:\s*(.*)$/m);
    let whyNotNonEmpty = false;
    if (whyNotKeyMatch) {
      const inline = whyNotKeyMatch[1].trim();
      if (inline && !/^[>|][+-]?\d*$/.test(inline)) {
        whyNotNonEmpty = true; // inline scalar, e.g. `why_not: some text`
      } else {
        // block scalar (`>` or `|`): look for a following line indented
        // deeper than `why_not:` itself with non-whitespace content.
        const keyIndent = whyNotKeyMatch[0].match(/^\s*/)[0].length;
        const afterKey = chunk.slice(chunk.indexOf(whyNotKeyMatch[0]) + whyNotKeyMatch[0].length);
        for (const l of afterKey.split('\n')) {
          if (l.trim() === '') continue;
          const indent = l.match(/^\s*/)[0].length;
          if (indent <= keyIndent) break;
          if (l.trim() !== '') { whyNotNonEmpty = true; break; }
        }
      }
    }
    return { option: optionMatch ? optionMatch[1].trim() : '', whyNotNonEmpty };
  });
}

function readEntry(path) {
  const raw = readFileSync(path, 'utf8');
  const m = raw.match(/^---\n([\s\S]*?)\n---\n?([\s\S]*)$/);
  if (!m) return { error: 'no YAML frontmatter (no --- ... --- block)' };
  const [, frontmatter] = m;
  return {
    fields: parseTopLevelFields(frontmatter),
    alternatives: parseAlternatives(frontmatter),
  };
}

export function checkFixtureLedger(dir, contract = loadSchemaContract()) {
  const files = readdirSync(dir).filter((f) => f.endsWith('.md')).sort();
  const violations = [];

  for (const f of files) {
    const path = join(dir, f);
    const entry = readEntry(path);
    if (entry.error) {
      violations.push(`${f}: ${entry.error}`);
      continue;
    }
    const { fields, alternatives } = entry;

    for (const req of contract.required) {
      if (fields[req] === undefined) {
        violations.push(`${f}: missing required field "${req}"`);
      }
    }

    if (fields.kind !== undefined && !contract.kindEnum.includes(fields.kind)) {
      violations.push(
        `${f}: kind "${fields.kind}" is not in the closed enum [${contract.kindEnum.join(', ')}]`
      );
    }

    const id = fields.id ?? basename(f, '.md');
    if (id === 'dec-0060') {
      const bad = !alternatives || alternatives.length === 0 ||
        alternatives.some((a) => !a.whyNotNonEmpty);
      if (bad) violations.push(`${f}: dec-0060 has no alternatives[].why_not`);
    }
  }

  return { files, violations };
}

// ---- CLI -----------------------------------------------------------------
if (import.meta.url === `file://${process.argv[1]}`) {
  const targetDir = process.argv[2]
    ? resolve(process.cwd(), process.argv[2])
    : join(ROOT, 'fixtures', 'demo-ledger');

  const { files, violations } = checkFixtureLedger(targetDir);

  console.log(`${files.length} file(s) checked in ${targetDir}, ${violations.length} error(s).`);
  if (violations.length) {
    for (const v of violations) console.error(`  - ${v}`);
    process.exit(1);
  }
  process.exit(0);
}
