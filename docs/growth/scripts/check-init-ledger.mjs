#!/usr/bin/env node
// docs/growth/scripts/check-init-ledger.mjs
//
// E8-L4-T5 (docs/plan/tasks/E8-L4.md). The secondary clip's whole claim is
// "value in the first 60 seconds," and dec-0010 makes that concrete as no
// empty first run. This is the post-recording check dec-0028's dry run
// promised: it counts *.md files under the recorded target's
// .dira/entries/, fails named if that count is zero, and additionally
// requires at least one entry to carry a non-empty `alternatives` array, so
// the clip shows a real contradiction already present in the corpus rather
// than a titles-only pile.
//
// This is not a YAML parser, same tradeoff check-fixture-ledger.mjs (E8-L3-T2)
// makes: it reads only the unindented `alternatives:` key and the `- option:`
// block that follows it.
//
// Usage: node check-init-ledger.mjs <path-to-recorded-target>
//   <path> is the directory dira import/init wrote into -- both
//   "<path>" and "<path>/.dira/entries" are accepted; the entries directory
//   is resolved automatically.
// Exit 0: >=1 entry file, and at least one has a non-empty alternatives[].
//         Prints the entry count and how many carry alternatives.
// Exit 1: zero entries ("empty ledger — dec-0010 violation"), or every entry
//         present has an empty alternatives array.
// Exit 2: usage error, or <path> does not resolve to an entries directory.

import { readFileSync, readdirSync, existsSync } from 'node:fs';
import { join, basename, resolve } from 'node:path';

function resolveEntriesDir(target) {
  if (basename(target) === 'entries' && existsSync(target)) return target;
  const nested = join(target, '.dira', 'entries');
  if (existsSync(nested)) return nested;
  if (existsSync(target)) return target;
  return null;
}

// Same block-scalar-aware parse check-fixture-ledger.mjs uses: an
// `alternatives:` key is "non-empty" only if it is followed by at least one
// `  - option:` line.
function hasNonEmptyAlternatives(frontmatter) {
  const blockMatch = ('\n' + frontmatter).match(/\nalternatives:\n([\s\S]*?)(?=\n[a-z_]+:|$)/);
  if (!blockMatch) return false;
  return /^\s*- option:/m.test(blockMatch[1]);
}

export function checkInitLedger(target) {
  const entriesDir = resolveEntriesDir(target);
  if (!entriesDir) {
    return { ok: false, reason: `${target}: no .dira/entries/ directory found` };
  }

  const files = readdirSync(entriesDir).filter((f) => f.endsWith('.md')).sort();
  if (files.length === 0) {
    return { ok: false, reason: 'empty ledger — dec-0010 violation', entriesDir, count: 0 };
  }

  let withAlternatives = 0;
  for (const f of files) {
    const raw = readFileSync(join(entriesDir, f), 'utf8');
    const m = raw.match(/^---\n([\s\S]*?)\n---\n?/);
    if (m && hasNonEmptyAlternatives(m[1])) withAlternatives++;
  }

  if (withAlternatives === 0) {
    return {
      ok: false,
      reason: `${files.length} entries written, but none carry a non-empty alternatives[] — titles-only pile`,
      entriesDir,
      count: files.length,
    };
  }

  return { ok: true, entriesDir, count: files.length, withAlternatives };
}

// ---- CLI -----------------------------------------------------------------
if (import.meta.url === `file://${process.argv[1]}`) {
  const targetArg = process.argv[2];
  if (!targetArg) {
    console.error('usage: node check-init-ledger.mjs <path-to-recorded-target>');
    process.exit(2);
  }
  const target = resolve(process.cwd(), targetArg);
  const result = checkInitLedger(target);
  if (!result.ok) {
    console.error(`FAIL: ${result.reason}`);
    process.exit(result.entriesDir === undefined ? 2 : 1);
  }
  console.log(
    `check-init-ledger PASS — ${result.count} entry(ies) in ${result.entriesDir}, ` +
    `${result.withAlternatives} carrying a non-empty alternatives[] array.`
  );
  process.exit(0);
}
