#!/usr/bin/env node
// docs/growth/scripts/check-cast-drift.mjs
//
// E8-L4-T3 (docs/plan/tasks/E8-L4.md). Extends E8-L3-T6's verbatim-binding
// idea one hop further: where check-demo-script.mjs binds record-check.sh's
// canonical output block to .agents/product-marketing.md §6, this binds a
// *recorded cast's* captured stdout to that same canonical block, by
// importing extractCanonicalBlock rather than re-deriving a second copy of
// the §6 text (docs/lore.md L-0019: one canonical source, not several loose
// substring searches). A hand-recorded or hand-edited cast that has drifted
// from the harness fails loudly here rather than shipping unnoticed.
//
// Usage: node check-cast-drift.mjs <path-to-cast> [record-check.sh path]
//   (record-check.sh path defaults to the real repo file)
// Exit 0: every non-blank line of the canonical block appears verbatim,
//         somewhere in the cast's concatenated "o" (output) event text.
//         Prints the file and the line count checked.
// Exit 1: the canonical block could not be extracted, or the first missing
//         line -- named, from the canonical block -- is not found in the cast.
// Exit 2: usage error.

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join, resolve } from 'node:path';
import { extractCanonicalBlock } from './check-demo-script.mjs';

const HERE = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(HERE, '..', '..', '..');
const DEFAULT_SCRIPT = join(ROOT, 'assets', 'demo', 'record-check.sh');

// Concatenates every "o" (output) event's data field, in file order. "i"
// (input) events and the header line are ignored -- what matters is what the
// terminal displayed, not what was typed at it.
export function concatOutputEvents(castText) {
  const lines = castText.split('\n').filter((l) => l.trim() !== '');
  let out = '';
  for (const line of lines.slice(1)) {
    let event;
    try {
      event = JSON.parse(line);
    } catch {
      continue; // not a parseable event line; skip rather than crash
    }
    if (Array.isArray(event) && event[1] === 'o' && typeof event[2] === 'string') {
      out += event[2];
    }
  }
  return out;
}

export function checkCastDrift(castPath, scriptPath = DEFAULT_SCRIPT) {
  const scriptText = readFileSync(scriptPath, 'utf8');
  const block = extractCanonicalBlock(scriptText);
  if (block.error) return { ok: false, reason: `${scriptPath}: ${block.error}` };

  const expectedLines = block.content.filter((l) => l.trim() !== '');
  if (expectedLines.length === 0) {
    return { ok: false, reason: `${scriptPath}: canonical block has no non-blank lines to check` };
  }

  const castText = readFileSync(castPath, 'utf8');
  const castOutput = concatOutputEvents(castText);

  for (const line of expectedLines) {
    if (!castOutput.includes(line)) {
      return { ok: false, reason: `missing expected line: ${JSON.stringify(line)}` };
    }
  }
  return { ok: true, expectedLines };
}

// ---- CLI -----------------------------------------------------------------
if (import.meta.url === `file://${process.argv[1]}`) {
  const castArg = process.argv[2];
  const scriptArg = process.argv[3];
  if (!castArg) {
    console.error('usage: node check-cast-drift.mjs <path-to-cast> [record-check.sh path]');
    process.exit(2);
  }
  const castPath = resolve(process.cwd(), castArg);
  const scriptPath = scriptArg ? resolve(process.cwd(), scriptArg) : DEFAULT_SCRIPT;

  const result = checkCastDrift(castPath, scriptPath);
  if (!result.ok) {
    console.error(`FAIL: ${result.reason}`);
    process.exit(1);
  }
  console.log(
    `check-cast-drift PASS — ${result.expectedLines.length} expected line(s) all found verbatim in ${castArg}.`
  );
  process.exit(0);
}
