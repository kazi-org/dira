#!/usr/bin/env node
// docs/growth/scripts/check-launch-readiness.mjs
//
// Lane E8-L6-T7. Zero third-party dependencies.
//
// The ordered launch-readiness aggregation checker. It will be RED for months --
// dira has no binary on PATH, no release, and no recorded demo clip on this branch
// today -- and a checker that is only useful once everything passes is not a
// checklist, it's a single "not ready" line. So every gate below runs and reports in
// FIXED ORDER, every time, regardless of whether an earlier gate failed: the first
// unmet gate is what blocks launch, but the full ordered list is what makes the
// checker useful while red.
//
// Composes its siblings rather than re-implementing them:
//   - launch.md's T0-offset/no-absolute-date rule (this file's own scanLaunchMd,
//     exported so a future edit can't quietly fork a second copy of the regex --
//     E8-L6-T1 has no script of its own, so the rule named in T1's own acc line is
//     defined once, here, rather than duplicated).
//   - check-drafts.mjs (E8-L5-T6) -- invoked as a subprocess, not reimplemented.
//   - check-launch-accuracy.mjs (E8-L6-T5) -- imported, not reimplemented.
//   - check-cast-duration.mjs (E8-L4-T1) -- imported dynamically if present; this repo
//     state does not have it yet (E8-L4 is still an open, unmerged PR), so the gate
//     reports SKIP with a named reason rather than silently passing or crashing.
//   - check-growth-plan.mjs's BANNED_HYPE_TERMS / stripHonestLimitsBlocks (E8-L1) --
//     imported, not a second hand-copied list.
//
// A gate is PASS, FAIL, or SKIP. SKIP is not a failure -- it names a real precondition
// that is honestly absent today (no cast recorded yet) rather than asserting a verdict
// the gate never actually reached (docs/lore.md's recurring defect in this repo).
// Overall exit is 0 iff there are zero FAIL gates; a SKIP can coexist with exit 0.
//
// Usage:
//   node check-launch-readiness.mjs

import { readFileSync, existsSync } from 'node:fs';
import { join, delimiter, dirname } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url));
export const REPO_ROOT = join(SCRIPT_DIR, '..', '..', '..');

const LAUNCH_MD = join(REPO_ROOT, 'docs', 'growth', 'launch.md');
const EXPERIMENTS_MD = join(REPO_ROOT, 'docs', 'growth', 'experiments.md');
const ROADMAP_MD = join(REPO_ROOT, 'docs', 'roadmap.md');
const SHOW_HN_MD = join(REPO_ROOT, 'docs', 'growth', 'drafts', 'show-hn.md');
const CHECK_DRAFTS = join(REPO_ROOT, 'docs', 'growth', 'scripts', 'check-drafts.mjs');
const CHECK_ACCURACY = join(REPO_ROOT, 'docs', 'growth', 'scripts', 'check-launch-accuracy.mjs');
const CHECK_CAST_DURATION = join(REPO_ROOT, 'docs', 'growth', 'scripts', 'check-cast-duration.mjs');
const CHECK_CAST = join(REPO_ROOT, 'assets', 'demo', 'check.cast');
const DEMO_DURATION_BOUND = '20.0';

// --- gate 1: a dira binary on PATH -------------------------------------------------

export function findDiraOnPath(pathEnv = process.env.PATH ?? '') {
  for (const dir of pathEnv.split(delimiter)) {
    if (!dir) continue;
    const candidate = join(dir, 'dira');
    if (existsSync(candidate)) return candidate;
  }
  return null;
}

function gateBinary() {
  const found = findDiraOnPath();
  if (!found) {
    // Exact wording asserted by this lane's own top-level acc line — first gate,
    // first printed line, true by construction until E0-E3 ship.
    return { name: 'dira binary on PATH', status: 'FAIL', reason: 'no dira binary found on PATH — E0–E3 have not shipped' };
  }
  return { name: 'dira binary on PATH', status: 'PASS', reason: `found at ${found}` };
}

// --- gate 2: launch.md is T0-relative only, owner+offset on every step line -------
// This is the rule named in E8-L6-T1's own acc line. T1 produces no script of its
// own (its Files: line is launch.md only), so the scan lives here -- once, not
// duplicated -- and T1's acc is proven by pointing this same function at a fixture
// copy of launch.md with an absolute date inserted.

const STEP_LINE_RE = /^-\s*\[ \]\s+.+$/;
const OWNER_RE = /@[A-Za-z][\w-]*/;
const T0_RE = /\bT0(?:[+-]\d+)?\b/;
const ABS_DATE_RE = /\b\d{4}-\d{2}-\d{2}\b/;
const MONTH_DATE_RE =
  /\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2}\b/i;

export function scanLaunchMd(text) {
  const failures = [];
  const lines = text.split('\n');
  lines.forEach((line, i) => {
    const lineNo = i + 1;
    if (STEP_LINE_RE.test(line)) {
      if (!OWNER_RE.test(line)) {
        failures.push(`line ${lineNo}: step has no owner token (@maintainer or a named role): "${line.trim()}"`);
      }
      if (!T0_RE.test(line)) {
        failures.push(`line ${lineNo}: step has no T0 offset (T0, T0+N, T0-N): "${line.trim()}"`);
      }
    }
    if (ABS_DATE_RE.test(line) || MONTH_DATE_RE.test(line)) {
      failures.push(`line ${lineNo}: absolute date found, must be T0-relative: "${line.trim()}"`);
    }
  });
  return failures;
}

function gateLaunchMdDates() {
  if (!existsSync(LAUNCH_MD)) {
    return { name: 'launch.md is T0-relative only', status: 'FAIL', reason: 'docs/growth/launch.md not found' };
  }
  const failures = scanLaunchMd(readFileSync(LAUNCH_MD, 'utf8'));
  if (failures.length > 0) {
    return { name: 'launch.md is T0-relative only', status: 'FAIL', reason: failures[0] };
  }
  return { name: 'launch.md is T0-relative only', status: 'PASS', reason: 'every step line carries an owner and a T0 offset, zero absolute dates' };
}

// --- gate 3: every channel launch.md names id-matches an experiments.md spec -------
// Scoped to EXP-NNN ids specifically -- launch.md "id-matches" a threshold row by
// citing its id, which is how docs/growth/experiments.md's own specs are addressed
// elsewhere in this repo. This checks one direction only (a channel launch.md names
// must be registered) -- the reverse (every registered experiment must appear in the
// phased launch sequence) is not required, because EXP-003 (build-in-public
// ship-notes) is explicitly registered as running from "now", with no T0 dependency,
// and is correctly absent from a T0-phased launch sequence; treating that as an
// orphan would fail launch.md for correctly leaving out something that isn't a launch
// step.

export function findOrphanChannels(launchText, experimentsText) {
  const launchIds = [...launchText.matchAll(/\bEXP-\d+\b/g)].map((m) => m[0]);
  const registeredIds = new Set([...experimentsText.matchAll(/^###\s+(EXP-\d+):/gm)].map((m) => m[1]));
  return [...new Set(launchIds)].filter((id) => !registeredIds.has(id));
}

function gateChannelOrphans() {
  if (!existsSync(LAUNCH_MD) || !existsSync(EXPERIMENTS_MD)) {
    return { name: 'launch.md channels id-match experiments.md', status: 'FAIL', reason: 'launch.md or experiments.md not found' };
  }
  const orphans = findOrphanChannels(readFileSync(LAUNCH_MD, 'utf8'), readFileSync(EXPERIMENTS_MD, 'utf8'));
  if (orphans.length > 0) {
    return {
      name: 'launch.md channels id-match experiments.md',
      status: 'FAIL',
      reason: `launch.md names ${orphans.join(', ')}, absent from experiments.md's registered specs`,
    };
  }
  return { name: 'launch.md channels id-match experiments.md', status: 'PASS', reason: 'every EXP-id in launch.md has a registered threshold row' };
}

// --- gate 4: check-drafts.mjs (E8-L5-T6) passes -------------------------------------

function gateCheckDrafts() {
  if (!existsSync(CHECK_DRAFTS)) {
    return { name: 'check-drafts.mjs passes', status: 'FAIL', reason: 'docs/growth/scripts/check-drafts.mjs not found' };
  }
  const result = spawnSync(process.execPath, [CHECK_DRAFTS], { encoding: 'utf8' });
  if (result.status !== 0) {
    const firstLine = (result.stderr || result.stdout || '').split('\n').find((l) => l.trim()) ?? 'check-drafts.mjs failed';
    return { name: 'check-drafts.mjs passes', status: 'FAIL', reason: firstLine };
  }
  return { name: 'check-drafts.mjs passes', status: 'PASS', reason: 'exit 0' };
}

// --- gate 5: check-launch-accuracy.mjs (T5) passes ----------------------------------

async function gateAccuracy() {
  if (!existsSync(CHECK_ACCURACY)) {
    return { name: 'pre-send accuracy gate (T5) passes', status: 'FAIL', reason: 'docs/growth/scripts/check-launch-accuracy.mjs not found' };
  }
  const { checkLaunchAccuracy } = await import(pathToFileURL(CHECK_ACCURACY));
  const draftsDir = join(REPO_ROOT, 'docs', 'growth', 'drafts');
  const readmeText = readFileSync(join(REPO_ROOT, 'README.md'), 'utf8');
  const { ok, failures } = checkLaunchAccuracy(draftsDir, readmeText);
  if (!ok) {
    return { name: 'pre-send accuracy gate (T5) passes', status: 'FAIL', reason: failures[0]?.reason ?? 'unverifiable verb claim' };
  }
  return { name: 'pre-send accuracy gate (T5) passes', status: 'PASS', reason: 'every dira <verb> claim in the drafts matches README.md' };
}

// --- gate 6: E8-L4-T1's cast-duration probe, composed not reimplemented ------------

async function gateCastDuration() {
  if (!existsSync(CHECK_CAST_DURATION)) {
    return {
      name: 'demo cast duration (E8-L4-T1)',
      status: 'SKIP',
      reason: 'E8-L4-T1 (docs/growth/scripts/check-cast-duration.mjs) is not present on this branch yet — E8-L4 has not merged',
    };
  }
  if (!existsSync(CHECK_CAST)) {
    return { name: 'demo cast duration (E8-L4-T1)', status: 'SKIP', reason: 'not yet recorded — assets/demo/check.cast does not exist' };
  }
  const result = spawnSync(process.execPath, [CHECK_CAST_DURATION, CHECK_CAST, DEMO_DURATION_BOUND], { encoding: 'utf8' });
  if (result.status !== 0) {
    const firstLine = (result.stderr || result.stdout || '').split('\n').find((l) => l.trim()) ?? 'check-cast-duration.mjs failed';
    return { name: 'demo cast duration (E8-L4-T1)', status: 'FAIL', reason: firstLine };
  }
  return { name: 'demo cast duration (E8-L4-T1)', status: 'PASS', reason: (result.stdout || '').trim() || 'within bound' };
}

// --- gate 7: docs/roadmap.md carries the T6 GTM section, every item named ---------

const LANE_RE = /\bE8-L[1-6]\b/;

export function findGtmSectionIssues(roadmapText) {
  const lines = roadmapText.split('\n');
  const headingIdx = lines.findIndex((l) => l.trim() === '## GTM');
  const headingCount = lines.filter((l) => l.trim() === '## GTM').length;
  if (headingCount === 0) return { issues: ['no "## GTM" heading found'], laneRowCount: 0 };
  if (headingCount > 1) return { issues: [`"## GTM" heading appears ${headingCount} times, expected exactly 1`], laneRowCount: 0 };

  let end = lines.length;
  for (let i = headingIdx + 1; i < lines.length; i++) {
    if (/^##\s/.test(lines[i])) { end = i; break; }
  }
  const section = lines.slice(headingIdx, end);
  const issues = [];
  let laneRowCount = 0;
  for (const line of section) {
    if (!line.trim().startsWith('|') || !LANE_RE.test(line)) continue;
    laneRowCount++;
    if (!OWNER_RE.test(line)) issues.push(`row missing an owner token: "${line.trim()}"`);
    if (!/#\d+|no PR yet/.test(line)) issues.push(`row missing a #NNN or "no PR yet": "${line.trim()}"`);
  }
  return { issues, laneRowCount };
}

function gateRoadmapGtm() {
  if (!existsSync(ROADMAP_MD)) {
    return { name: 'roadmap.md GTM section (T6)', status: 'FAIL', reason: 'docs/roadmap.md not found' };
  }
  const { issues, laneRowCount } = findGtmSectionIssues(readFileSync(ROADMAP_MD, 'utf8'));
  if (issues.length > 0) {
    return { name: 'roadmap.md GTM section (T6)', status: 'FAIL', reason: issues[0] };
  }
  return { name: 'roadmap.md GTM section (T6)', status: 'PASS', reason: `${laneRowCount} lane row(s), each with an owner and a PR reference` };
}

// --- gate 8: the Show HN title + zero hype terms ------------------------------------

const SHOW_HN_TITLE_RE = /^Show HN: dira – /m;

async function gateShowHn() {
  if (!existsSync(SHOW_HN_MD)) {
    return { name: 'show-hn.md title and hype scan', status: 'FAIL', reason: 'docs/growth/drafts/show-hn.md not found' };
  }
  const text = readFileSync(SHOW_HN_MD, 'utf8');
  if (!SHOW_HN_TITLE_RE.test(text)) {
    return { name: 'show-hn.md title and hype scan', status: 'FAIL', reason: 'no line matches ^Show HN: dira – ' };
  }
  const { BANNED_HYPE_TERMS, stripHonestLimitsBlocks } = await import(
    pathToFileURL(join(REPO_ROOT, 'docs', 'growth', 'scripts', 'check-growth-plan.mjs'))
  );
  const stripped = stripHonestLimitsBlocks(text).toLowerCase();
  const hit = BANNED_HYPE_TERMS.find((t) => stripped.includes(t));
  if (hit) {
    return { name: 'show-hn.md title and hype scan', status: 'FAIL', reason: `banned-hype term "${hit}" found outside an honest-limits block` };
  }
  return { name: 'show-hn.md title and hype scan', status: 'PASS', reason: 'title matches, zero banned-hype terms' };
}

function pathToFileURL(p) {
  return new URL(`file://${p}`);
}

// --- run every gate, in fixed order, unconditionally --------------------------------

export async function runGates() {
  const gates = [
    gateBinary(),
    gateLaunchMdDates(),
    gateChannelOrphans(),
    gateCheckDrafts(),
    await gateAccuracy(),
    await gateCastDuration(),
    gateRoadmapGtm(),
    await gateShowHn(),
  ];
  return gates;
}

function main() {
  runGates().then((gates) => {
    // Gate 1 prints as "FAIL: <reason>" so its exact wording (this lane's own
    // top-level acc line) is the literal first line when it fails. Every other gate
    // prints "<status>: <name> — <reason>" so the ordered list stays legible.
    gates.forEach((g, i) => {
      if (i === 0) console.log(`${g.status}: ${g.reason}`);
      else console.log(`${g.status}: ${g.name} — ${g.reason}`);
    });
    const firstFail = gates.find((g) => g.status === 'FAIL');
    const skipped = gates.filter((g) => g.status === 'SKIP');
    console.log('');
    if (firstFail) {
      console.log(`NOT READY — first blocking gate: ${firstFail.name}`);
      if (skipped.length) console.log(`(${skipped.length} gate(s) skipped, not counted as blocking)`);
      process.exitCode = 1;
      return;
    }
    console.log(`READY${skipped.length ? ` (${skipped.length} gate(s) skipped)` : ''} — every gate passed or was honestly skipped, none failed.`);
  });
}

const isMain = process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1];
if (isMain) main();
