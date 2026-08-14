#!/usr/bin/env node
// docs/growth/scripts/check-demo-script.mjs
//
// E8-L3-T6 (docs/plan/tasks/E8-L3.md). Binds assets/demo/record-check.sh's
// canonical demo output block (the typed command plus the exact lines dira
// check will print) to .agents/product-marketing.md §6's fenced code block,
// verbatim. Both sides are read fresh every run -- neither is a second,
// hand-copied literal in this script -- so editing one and not the other
// fails loudly instead of drifting apart silently (docs/lore.md L-0019: one
// canonical string, one assertion, not several loose substring searches).
//
// Usage: node check-demo-script.mjs [record-check.sh path] [product-marketing.md path]
//   (both default to the real repo files)
// Exit 0: the canonical block matches §6's fenced block, line for line.
//         Prints the files and 1-based line ranges checked.
// Exit 1: a marker/fence could not be found, or a line differs -- the
//         offending line, from both sides, is printed.

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join, resolve, relative } from 'node:path';

const HERE = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(HERE, '..', '..', '..');

const DEFAULT_SCRIPT = join(ROOT, 'assets', 'demo', 'record-check.sh');
const DEFAULT_MARKETING = join(ROOT, '.agents', 'product-marketing.md');

const BEGIN_MARK = '# --- BEGIN CANONICAL DEMO OUTPUT';
const END_MARK = '# --- END CANONICAL DEMO OUTPUT';

// Every comment line between the BEGIN/END sentinels in record-check.sh, with
// its "# " (or bare "#" for a blank line) prefix stripped back to raw text,
// plus the 1-based line range (in the source file) the content occupies.
export function extractCanonicalBlock(scriptText) {
  const lines = scriptText.split('\n');
  const beginIdx = lines.findIndex((l) => l.startsWith(BEGIN_MARK));
  const endIdx = lines.findIndex((l) => l.startsWith(END_MARK));
  if (beginIdx === -1 || endIdx === -1 || endIdx <= beginIdx) {
    return { error: 'could not find BEGIN/END CANONICAL DEMO OUTPUT markers' };
  }
  const raw = lines.slice(beginIdx + 1, endIdx);
  const content = [];
  for (const l of raw) {
    if (l === '#') { content.push(''); continue; }
    if (l.startsWith('# ')) { content.push(l.slice(2)); continue; }
    return { error: `canonical block line is not a "# " comment: ${JSON.stringify(l)}` };
  }
  return { content, startLine: beginIdx + 2, endLine: endIdx };
}

// The first ``` ... ``` fenced block after the "## 6." heading in
// product-marketing.md, plus its 1-based line range.
export function extractSection6Block(marketingText) {
  const lines = marketingText.split('\n');
  const headingIdx = lines.findIndex((l) => l.trim().startsWith('## 6.'));
  if (headingIdx === -1) return { error: 'could not find a "## 6." heading' };
  const fenceStart = lines.findIndex((l, i) => i > headingIdx && l.trim() === '```');
  if (fenceStart === -1) return { error: 'could not find an opening ``` fence after "## 6."' };
  const fenceEnd = lines.findIndex((l, i) => i > fenceStart && l.trim() === '```');
  if (fenceEnd === -1) return { error: 'could not find the closing ``` fence for §6' };
  return { content: lines.slice(fenceStart + 1, fenceEnd), startLine: fenceStart + 2, endLine: fenceEnd };
}

export function compareBlocks(scriptContent, marketingContent) {
  const max = Math.max(scriptContent.length, marketingContent.length);
  for (let i = 0; i < max; i++) {
    if (scriptContent[i] !== marketingContent[i]) {
      return { ok: false, line: i + 1, script: scriptContent[i], marketing: marketingContent[i] };
    }
  }
  return { ok: true };
}

export function checkDemoScript(scriptPath, marketingPath) {
  const scriptText = readFileSync(scriptPath, 'utf8');
  const marketingText = readFileSync(marketingPath, 'utf8');

  const scriptBlock = extractCanonicalBlock(scriptText);
  if (scriptBlock.error) return { ok: false, reason: `${scriptPath}: ${scriptBlock.error}` };

  const marketingBlock = extractSection6Block(marketingText);
  if (marketingBlock.error) return { ok: false, reason: `${marketingPath}: ${marketingBlock.error}` };

  const cmp = compareBlocks(scriptBlock.content, marketingBlock.content);
  if (!cmp.ok) {
    return {
      ok: false,
      reason:
        `line ${cmp.line} of the canonical block does not match §6 verbatim:\n` +
        `  record-check.sh:      ${JSON.stringify(cmp.script ?? '<missing>')}\n` +
        `  product-marketing.md: ${JSON.stringify(cmp.marketing ?? '<missing>')}`,
    };
  }

  return { ok: true, scriptBlock, marketingBlock };
}

// ---- CLI -----------------------------------------------------------------
if (import.meta.url === `file://${process.argv[1]}`) {
  const scriptPath = process.argv[2] ? resolve(process.cwd(), process.argv[2]) : DEFAULT_SCRIPT;
  const marketingPath = process.argv[3] ? resolve(process.cwd(), process.argv[3]) : DEFAULT_MARKETING;

  const result = checkDemoScript(scriptPath, marketingPath);
  if (!result.ok) {
    console.error(`FAIL: ${result.reason}`);
    process.exit(1);
  }

  const { scriptBlock, marketingBlock } = result;
  console.log(
    `check-demo-script PASS — ${relative(ROOT, scriptPath)}:${scriptBlock.startLine}-${scriptBlock.endLine} ` +
    `matches ${relative(ROOT, marketingPath)}:${marketingBlock.startLine}-${marketingBlock.endLine} ` +
    `verbatim (${scriptBlock.content.length} line(s)).`
  );
  process.exit(0);
}
