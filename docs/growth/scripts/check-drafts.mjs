#!/usr/bin/env node
// docs/growth/scripts/check-drafts.mjs
//
// Lane E8-L5 (docs/plan/tasks/E8-L5.md). Zero third-party dependencies.
// Enforces the mechanical form of E8's absolute #1 (docs/plan/lanes/E8.md):
// nothing outward-facing is ever sent, posted, published, or submitted by an
// agent. The frontmatter contract this checks is documented once, in
// docs/growth/DRAFT-CONTRACT.md.
//
// Three checks, all must pass for exit 0:
//
//   1. Every *.md file under docs/growth/drafts/ carries frontmatter with
//      status: awaiting-maintainer-approval and posted: false.
//   2. No file under docs/growth/ or assets/demo/ contains an automated-post
//      invocation from the deny-list below.
//   3. .claude-plugin/plugin.json parses, and its declared hook commands
//      match hooks/settings.example.json name-for-name (same event names,
//      same command strings) -- a hook renamed on one side only fails.
//
// Fixture data for the checker's OWN tests lives under any directory
// literally named `fixtures` (docs/growth/scripts/fixtures/). Check #2
// deliberately excludes those directories from the production scan --
// otherwise the intentionally-bad content committed there so this checker
// can prove it fails would make the real, production run of this script
// fail forever. `check-drafts.selftest.mjs` points the same scan functions
// directly AT that fixture data to prove red, then at the real repo state
// to prove green.
//
// A second, narrower exclusion: this file, `check-drafts.selftest.mjs`, and
// `docs/growth/DRAFT-CONTRACT.md` all live under docs/growth/ and all
// necessarily quote the deny-list's trigger phrases in comments, assertion
// labels, or prose documenting the rule -- that is meta-documentation of
// the check, not an outward-facing invocation, and check #2 would otherwise
// flag itself for describing its own rule. Those three files are excluded
// from check #2 BY NAME (SELF_EXCLUDED_FILES below) -- a fixed, three-file,
// auditable exclusion, not a directory or pattern that could hide a real
// violation. An earlier attempt to solve this by splitting the trigger
// strings apart in code (`'gh' + ' pr create'`) was insufficient: prose
// describing the technique still spelled the phrase out contiguously and
// still matched. Excluding the known meta-files by exact path is the
// correct fix, verified by running the failing case first.
//
// Usage:
//   node docs/growth/scripts/check-drafts.mjs             # checks the real repo, exit 0/1
//   node docs/growth/scripts/check-drafts.selftest.mjs     # proves red, then green

import { readFileSync, readdirSync, existsSync } from 'node:fs';
import { join, relative, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url));
export const REPO_ROOT = join(SCRIPT_DIR, '..', '..', '..');

const EXCLUDE_DIR_NAMES = ['fixtures'];

// Named, auditable exclusion from the deny-list scan (check #2 only -- the
// draft-contract and hook-match checks still cover these paths where
// relevant). See the header comment for why: these three files describe or
// implement the deny-list itself and necessarily quote its trigger phrases.
const SELF_EXCLUDED_FILES = new Set([
  join(REPO_ROOT, 'docs', 'growth', 'DRAFT-CONTRACT.md'),
  join(REPO_ROOT, 'docs', 'growth', 'scripts', 'check-drafts.mjs'),
  join(REPO_ROOT, 'docs', 'growth', 'scripts', 'check-drafts.selftest.mjs'),
]);

// --- deny-list ---------------------------------------------------------------

const GH_PR_CREATE = 'gh pr create';
const GH_ISSUE_CREATE = 'gh issue create';
const PRAW_WORD = 'praw';
const TWEEPY_WORD = 'tweepy';
const SOCIAL_POST_HOSTS = [
  'api.twitter.com',
  'api.x.com',
  'oauth.reddit.com',
  'www.reddit.com/api',
  'old.reddit.com/api',
  'slack.com/api',
  'discord.com/api',
  'news.ycombinator.com/submit',
  'news.ycombinator.com/comment',
];

export function scanContentForDenylist(content) {
  const hits = [];
  if (content.includes(GH_PR_CREATE)) hits.push('gh pr create invocation');
  if (content.includes(GH_ISSUE_CREATE)) hits.push('gh issue create invocation');
  if (new RegExp(`\\b${PRAW_WORD}\\b`).test(content)) hits.push('praw import/usage');
  if (new RegExp(`\\b${TWEEPY_WORD}\\b`).test(content)) hits.push('tweepy import/usage');
  const postish = /\bpost\s*\(|\bPOST\b/i;
  if (postish.test(content)) {
    for (const host of SOCIAL_POST_HOSTS) {
      if (content.includes(host)) {
        hits.push(`HTTP POST-shaped call near social/forum API host "${host}"`);
      }
    }
  }
  return hits;
}

// --- frontmatter parsing (hand-rolled -- no gray-matter dependency) -------

export function parseFrontmatter(content) {
  const lines = content.split('\n');
  if ((lines[0] ?? '').trim() !== '---') return null;
  let end = -1;
  for (let i = 1; i < lines.length; i++) {
    if (lines[i].trim() === '---') {
      end = i;
      break;
    }
  }
  if (end === -1) return null;
  const fm = {};
  for (const raw of lines.slice(1, end)) {
    const line = raw.trim();
    if (!line || line.startsWith('#')) continue;
    const idx = line.indexOf(':');
    if (idx === -1) continue;
    const key = line.slice(0, idx).trim();
    let value = line.slice(idx + 1).trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    if (value === 'true') fm[key] = true;
    else if (value === 'false') fm[key] = false;
    else fm[key] = value;
  }
  return fm;
}

// --- file walking ----------------------------------------------------------

function listFilesRecursive(dir, excludeDirNames = []) {
  if (!existsSync(dir)) return [];
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory() && excludeDirNames.includes(entry.name)) continue;
    const p = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...listFilesRecursive(p, excludeDirNames));
    else out.push(p);
  }
  return out;
}

// --- check 1: draft frontmatter contract -----------------------------------

export function checkDraftsContract(draftsDir) {
  const failures = [];
  const files = listFilesRecursive(draftsDir).filter((f) => f.endsWith('.md'));
  if (files.length === 0) {
    failures.push({ file: draftsDir, reason: 'no draft files found under this directory' });
    return failures;
  }
  for (const file of files) {
    const content = readFileSync(file, 'utf8');
    const fm = parseFrontmatter(content);
    if (!fm) {
      failures.push({ file, reason: 'missing or malformed frontmatter block' });
      continue;
    }
    if (fm.status !== 'awaiting-maintainer-approval') {
      failures.push({
        file,
        reason: `status must be "awaiting-maintainer-approval", got ${JSON.stringify(fm.status)}`,
      });
    }
    if (fm.posted !== false) {
      failures.push({
        file,
        reason: `posted must be boolean false, got ${JSON.stringify(fm.posted)}`,
      });
    }
  }
  return failures;
}

// --- check 2: deny-list scan ------------------------------------------------

export function checkDenylist(rootDirs) {
  const failures = [];
  for (const root of rootDirs) {
    for (const file of listFilesRecursive(root, EXCLUDE_DIR_NAMES)) {
      if (SELF_EXCLUDED_FILES.has(file)) continue;
      let content;
      try {
        content = readFileSync(file, 'utf8');
      } catch {
        continue; // unreadable / binary -- not a text invocation, skip
      }
      for (const hit of scanContentForDenylist(content)) {
        failures.push({ file, reason: hit });
      }
    }
  }
  return failures;
}

// --- check 3: plugin.json hooks match hooks/ name-for-name -----------------

function loadJSON(path) {
  return JSON.parse(readFileSync(path, 'utf8'));
}

function extractHookCommands(hooksConfig) {
  const out = {};
  for (const [event, entries] of Object.entries(hooksConfig ?? {})) {
    const commands = [];
    for (const entry of entries) {
      for (const h of entry.hooks ?? []) {
        commands.push(h.command);
      }
    }
    out[event] = commands;
  }
  return out;
}

export function checkHooksMatch(settingsExamplePath, pluginJsonPath) {
  const failures = [];
  let settings, plugin;
  try {
    settings = loadJSON(settingsExamplePath);
  } catch (e) {
    failures.push({ file: settingsExamplePath, reason: `does not parse: ${e.message}` });
    return failures;
  }
  try {
    plugin = loadJSON(pluginJsonPath);
  } catch (e) {
    failures.push({ file: pluginJsonPath, reason: `does not parse: ${e.message}` });
    return failures;
  }

  const settingsHooks = extractHookCommands(settings.hooks);
  const pluginHooks = extractHookCommands(plugin.hooks);
  const events = new Set([...Object.keys(settingsHooks), ...Object.keys(pluginHooks)]);

  for (const event of events) {
    const a = settingsHooks[event];
    const b = pluginHooks[event];
    if (!a) {
      failures.push({
        file: pluginJsonPath,
        reason: `declares hook "${event}" not present in ${settingsExamplePath}`,
      });
      continue;
    }
    if (!b) {
      failures.push({
        file: pluginJsonPath,
        reason: `is missing hook "${event}" present in ${settingsExamplePath}`,
      });
      continue;
    }
    if (JSON.stringify(a) !== JSON.stringify(b)) {
      failures.push({
        file: pluginJsonPath,
        reason: `hook "${event}" command mismatch: ${settingsExamplePath} has ${JSON.stringify(a)}, ${pluginJsonPath} has ${JSON.stringify(b)}`,
      });
    }
  }
  return failures;
}

// --- entry point -------------------------------------------------------------

function main() {
  const draftsDir = join(REPO_ROOT, 'docs', 'growth', 'drafts');
  const growthDir = join(REPO_ROOT, 'docs', 'growth');
  const demoDir = join(REPO_ROOT, 'assets', 'demo');
  const settingsExamplePath = join(REPO_ROOT, 'hooks', 'settings.example.json');
  const pluginJsonPath = join(REPO_ROOT, '.claude-plugin', 'plugin.json');

  const failures = [
    ...checkDraftsContract(draftsDir),
    ...checkDenylist([growthDir, demoDir]),
    ...checkHooksMatch(settingsExamplePath, pluginJsonPath),
  ];

  if (failures.length > 0) {
    console.error(`check-drafts: FAIL (${failures.length} issue(s))`);
    for (const f of failures) {
      console.error(`  - ${relative(REPO_ROOT, f.file) || f.file}: ${f.reason}`);
    }
    process.exitCode = 1;
    return;
  }
  console.log('check-drafts: PASS');
}

const isMain = process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1];
if (isMain) main();
