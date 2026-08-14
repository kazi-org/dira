#!/usr/bin/env node
// docs/growth/scripts/check-launch-accuracy.mjs
//
// Lane E8-L6-T5. Zero third-party dependencies.
//
// The pre-send accuracy gate -- the anti-vaporware step run before the maintainer is
// ever asked to approve a draft. Every `dira <verb>` mentioned in a draft under
// docs/growth/drafts/ must name a verb that actually appears in README.md's own
// documented verb table ("## The core verbs") -- the same document
// docs/design/scripts/check-coherence.mjs (E8-L2) already treats as canonical. This
// script reads that table itself rather than hand-maintaining a second allowlist that
// could silently drift from what README claims is real.
//
// Only verb-claims written the way this repo's own docs demonstrate a command was
// actually run -- inside backticks (`dira <verb> ...`) or a shell prompt
// (`$ dira <verb> ...`) inside a fenced code block -- count as a claim. Plain prose
// ("dira is a ledger", "dira can also...") is not a command invocation and is not
// scanned; scanning it would flag words like "is"/"can" as unverifiable "verbs" and
// make the checker useless on real copy.
//
// Usage:
//   node check-launch-accuracy.mjs [drafts-dir]   (default: docs/growth/drafts/)
//
// Exit 0 and a report of every verb-claim checked (file, verb, source line range) if
// every claim resolves to a README-documented verb. Exit 1, naming the file and the
// unverifiable verb, on the first claim that does not.

import { readFileSync, readdirSync, existsSync } from 'node:fs';
import { join, relative, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url));
export const REPO_ROOT = join(SCRIPT_DIR, '..', '..', '..');
const README_PATH = join(REPO_ROOT, 'README.md');
const VERB_TABLE_HEADING = '## The core verbs';

// --- canonical verb source: README.md's own "## The core verbs" table ------------

// Returns { verbs: Set<string>, lineRange: [startLine, endLine] } (1-indexed, inclusive)
// so a report can cite exactly where each verb came from.
export function extractCanonicalVerbs(readmeText) {
  const lines = readmeText.split('\n');
  const headingIdx = lines.findIndex((l) => l.trim() === VERB_TABLE_HEADING);
  if (headingIdx === -1) {
    return { verbs: new Set(), lineRange: null, error: `README.md has no "${VERB_TABLE_HEADING}" heading` };
  }
  // Find the table: the first line after the heading that starts with '|', then every
  // contiguous '|'-prefixed line after it (header, separator, then data rows).
  let tableStart = -1;
  for (let i = headingIdx + 1; i < lines.length; i++) {
    if (lines[i].trim().startsWith('|')) {
      tableStart = i;
      break;
    }
    if (lines[i].trim().startsWith('#')) break; // hit the next section first
  }
  if (tableStart === -1) {
    return { verbs: new Set(), lineRange: null, error: `no table found under "${VERB_TABLE_HEADING}"` };
  }
  let tableEnd = tableStart;
  while (tableEnd + 1 < lines.length && lines[tableEnd + 1].trim().startsWith('|')) tableEnd++;

  const verbs = new Set();
  // Skip the header row (tableStart) and the separator row (tableStart + 1); data
  // rows start at tableStart + 2.
  for (let i = tableStart + 2; i <= tableEnd; i++) {
    const cells = lines[i].trim().slice(1, -1).split('|');
    const first = (cells[0] ?? '').trim();
    const m = first.match(/^`([a-zA-Z][\w-]*)`$/);
    if (m) verbs.add(m[1]);
  }
  return { verbs, lineRange: [tableStart + 1, tableEnd + 1] }; // 1-indexed for humans
}

// --- verb-claim extraction from a draft -------------------------------------------

// Only counts a `dira <verb>` mention as a claim when it is written the way this
// repo's own docs show a real, run command: backtick-wrapped, or a `$ dira` shell
// prompt. Returns [{verb, index}] with the character index of the match, for line
// lookup.
export function extractVerbClaims(text) {
  const claims = [];
  for (const m of text.matchAll(/`dira ([a-zA-Z][\w-]*)/g)) {
    claims.push({ verb: m[1], index: m.index });
  }
  for (const m of text.matchAll(/\$\s*dira\s+([a-zA-Z][\w-]*)/g)) {
    claims.push({ verb: m[1], index: m.index });
  }
  return claims;
}

function lineNumberAt(text, index) {
  return text.slice(0, index).split('\n').length;
}

// --- the check ---------------------------------------------------------------------

function listMarkdownFiles(dir) {
  if (!existsSync(dir)) return [];
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...listMarkdownFiles(p));
    else if (entry.name.endsWith('.md')) out.push(p);
  }
  return out;
}

// draftsDir: a directory of *.md files to scan (recursively).
// readmeText: README.md's content, read by the caller so this stays pure and testable
// against a fixture README without touching the filesystem a second time.
export function checkLaunchAccuracy(draftsDir, readmeText) {
  const { verbs, lineRange, error } = extractCanonicalVerbs(readmeText);
  if (error) {
    return { ok: false, checked: [], failures: [{ file: README_PATH, reason: error }], verbSource: null };
  }

  const files = listMarkdownFiles(draftsDir);
  const checked = [];
  const failures = [];

  for (const file of files) {
    const content = readFileSync(file, 'utf8');
    for (const { verb, index } of extractVerbClaims(content)) {
      const line = lineNumberAt(content, index);
      checked.push({ file, verb, line });
      if (!verbs.has(verb)) {
        failures.push({
          file,
          reason: `claims verb "dira ${verb}" at line ${line}, not found in README.md's core-verbs table`,
        });
      }
    }
  }

  return { ok: failures.length === 0, checked, failures, verbSource: lineRange };
}

// --- entry point ---------------------------------------------------------------------

function main() {
  const draftsDir = process.argv[2]
    ? join(process.cwd(), process.argv[2])
    : join(REPO_ROOT, 'docs', 'growth', 'drafts');

  const readmeText = readFileSync(README_PATH, 'utf8');
  const { ok, checked, failures, verbSource } = checkLaunchAccuracy(draftsDir, readmeText);

  const sourceLabel = verbSource
    ? `README.md:${verbSource[0]}-${verbSource[1]}`
    : 'README.md (no core-verbs table found)';

  if (!ok) {
    console.error(`check-launch-accuracy: FAIL (${failures.length} issue(s)), verb source: ${sourceLabel}`);
    for (const f of failures) console.error(`  - ${relative(REPO_ROOT, f.file) || f.file}: ${f.reason}`);
    process.exitCode = 1;
    return;
  }

  console.log(`check-launch-accuracy: PASS, verb source: ${sourceLabel}`);
  console.log(`${checked.length} verb-claim(s) checked against ${relative(REPO_ROOT, draftsDir)}:`);
  for (const c of checked) {
    console.log(`  - ${relative(REPO_ROOT, c.file)}:${c.line}: "dira ${c.verb}"`);
  }
}

const isMain = process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1];
if (isMain) main();
