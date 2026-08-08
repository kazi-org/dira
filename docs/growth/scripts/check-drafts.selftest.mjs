#!/usr/bin/env node
// docs/growth/scripts/check-drafts.selftest.mjs
//
// Proves check-drafts.mjs fires RED on each committed bad fixture, then
// GREEN on the real repo state -- a checker that has never been observed to
// fail is not a guard (team instruction for lane E8-L5). This file is itself
// a CI-safe gate: it exits non-zero if any assertion fails, not just when
// run manually.
//
// Usage: node docs/growth/scripts/check-drafts.selftest.mjs

import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  REPO_ROOT,
  checkDraftsContract,
  checkDenylist,
  checkHooksMatch,
} from './check-drafts.mjs';

const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url));
const FIXTURES = join(SCRIPT_DIR, 'fixtures');

let failed = 0;

function assertRed(label, failures) {
  if (failures.length === 0) {
    console.error(`FAIL (expected red, got green): ${label}`);
    failed++;
  } else {
    console.log(`red  ok: ${label} -- ${failures.length} issue(s), e.g. "${failures[0].reason}"`);
  }
}

function assertGreen(label, failures) {
  if (failures.length !== 0) {
    console.error(`FAIL (expected green, got red): ${label}`);
    for (const f of failures) console.error(`    - ${f.file}: ${f.reason}`);
    failed++;
  } else {
    console.log(`green ok: ${label}`);
  }
}

// --- 1. Draft frontmatter contract -----------------------------------------

assertRed(
  'fixtures/bad-posted-true (posted: true)',
  checkDraftsContract(join(FIXTURES, 'bad-posted-true', 'drafts'))
);

assertGreen(
  'docs/growth/drafts/ (the real, committed drafts)',
  checkDraftsContract(join(REPO_ROOT, 'docs', 'growth', 'drafts'))
);

// --- 2. Deny-list scan -------------------------------------------------------

assertRed(
  'fixtures/bad-denylist (gh pr create, gh issue create, praw, tweepy, social POST)',
  checkDenylist([join(FIXTURES, 'bad-denylist')])
);

assertGreen(
  'docs/growth/ + assets/demo/ (the real repo content, fixtures/ excluded)',
  checkDenylist([join(REPO_ROOT, 'docs', 'growth'), join(REPO_ROOT, 'assets', 'demo')])
);

// --- 3. plugin.json hooks match hooks/ name-for-name ------------------------

assertRed(
  'fixtures/hook-mismatch (Stop command renamed on the plugin.json side only)',
  checkHooksMatch(
    join(FIXTURES, 'hook-mismatch', 'hooks', 'settings.example.json'),
    join(FIXTURES, 'hook-mismatch', '.claude-plugin', 'plugin.json')
  )
);

assertGreen(
  'hooks/settings.example.json vs .claude-plugin/plugin.json (the real files)',
  checkHooksMatch(
    join(REPO_ROOT, 'hooks', 'settings.example.json'),
    join(REPO_ROOT, '.claude-plugin', 'plugin.json')
  )
);

// --- verdict -----------------------------------------------------------------

if (failed > 0) {
  console.error(`\ncheck-drafts.selftest: FAIL (${failed} assertion(s) did not hold)`);
  process.exitCode = 1;
} else {
  console.log('\ncheck-drafts.selftest: PASS (every red case failed, every green case passed)');
}
