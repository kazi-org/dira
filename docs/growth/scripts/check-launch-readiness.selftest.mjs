#!/usr/bin/env node
// docs/growth/scripts/check-launch-readiness.selftest.mjs
//
// Proves check-launch-readiness.mjs's gates both ways -- red on a false premise, green
// on the correct case -- per docs/lore.md's own recurring-defect warning: a check that
// never demonstrably fails is not evidence, and a check that never demonstrably passes
// is equally worthless and its failure is invisible from the red cases alone.
//
// Three things proved here, none of them by inspection:
//   1. E8-L6-T1's own acc line: a fixture copy of launch.md with one absolute date
//      inserted flips the date-scan gate from pass to a named failure quoting the
//      offending line, before the real launch.md is shown to pass.
//   2. The channel-orphan gate: a fixture launch.md naming a channel absent from
//      experiments.md flips that gate to a named failure, before the real files are
//      shown to pass.
//   3. The aggregate-level two-sided proof T7's own acc demands: with no dira on
//      PATH (today's real state), the checker exits 1, first line naming the binary
//      gate. With a stub dira added to PATH -- nothing else in the repo touched -- it
//      exits 0, printing every gate as passed or honestly skipped in order. This
//      proves the checker is not structurally stuck red regardless of what else is
//      true.
//
// Usage: node check-launch-readiness.selftest.mjs

import { readFileSync, mkdtempSync, writeFileSync, chmodSync, rmSync } from 'node:fs';
import { join, delimiter } from 'node:path';
import { tmpdir } from 'node:os';
import { spawnSync } from 'node:child_process';
import {
  REPO_ROOT,
  scanLaunchMd,
  findOrphanChannels,
  findGtmSectionIssues,
} from './check-launch-readiness.mjs';

const SCRIPT_PATH = join(REPO_ROOT, 'docs', 'growth', 'scripts', 'check-launch-readiness.mjs');
let failed = false;

function report(label, ok, detail) {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${label}${detail ? ` -- ${detail}` : ''}`);
  if (!ok) failed = true;
}

// --- 1. T1's date-scan gate, red then green -----------------------------------------

const GOOD_LAUNCH_MD = `## Phase 0

- [ ] Do a thing — @maintainer, T0-7
- [ ] Do another thing — @maintainer, T0+2
`;

const BAD_LAUNCH_MD = `## Phase 0

- [ ] Do a thing — @maintainer, T0-7
- [ ] Ship it on 2026-09-01 — @maintainer, T0
`;

{
  const redFailures = scanLaunchMd(BAD_LAUNCH_MD);
  const redOk = redFailures.length > 0 && redFailures.some((f) => f.includes('absolute date') && f.includes('2026-09-01'));
  report('T1 date-scan: RED on an inserted absolute date, naming the offending line', redOk, redFailures[0]);

  const greenFailures = scanLaunchMd(GOOD_LAUNCH_MD);
  report('T1 date-scan: GREEN on a clean T0-relative fixture', greenFailures.length === 0, greenFailures[0]);

  const realFailures = scanLaunchMd(readFileSync(join(REPO_ROOT, 'docs', 'growth', 'launch.md'), 'utf8'));
  report('T1 date-scan: GREEN on the real docs/growth/launch.md', realFailures.length === 0, realFailures[0]);
}

// --- 2. channel-orphan gate, red then green -----------------------------------------

{
  const experimentsText = readFileSync(join(REPO_ROOT, 'docs', 'growth', 'experiments.md'), 'utf8');

  const orphanLaunch = 'See EXP-001 and also the brand new EXP-999 channel.';
  const redOrphans = findOrphanChannels(orphanLaunch, experimentsText);
  report('channel-orphan gate: RED on a channel absent from experiments.md', redOrphans.length === 1 && redOrphans[0] === 'EXP-999', JSON.stringify(redOrphans));

  const cleanLaunch = 'See EXP-001 and EXP-002.';
  const greenOrphans = findOrphanChannels(cleanLaunch, experimentsText);
  report('channel-orphan gate: GREEN when every id is registered', greenOrphans.length === 0, JSON.stringify(greenOrphans));

  const realLaunch = readFileSync(join(REPO_ROOT, 'docs', 'growth', 'launch.md'), 'utf8');
  const realOrphans = findOrphanChannels(realLaunch, experimentsText);
  report('channel-orphan gate: GREEN on the real launch.md', realOrphans.length === 0, JSON.stringify(realOrphans));
}

// --- 3. roadmap GTM-section gate, red then green ------------------------------------

{
  const badRoadmap = `## GTM\n\n| Lane | What | Owner | PR |\n|---|---|---|---|\n| E8-L1 | thing | | |\n`;
  const { issues: redIssues } = findGtmSectionIssues(badRoadmap);
  report('roadmap GTM gate: RED on a row missing owner and PR', redIssues.length > 0, redIssues[0]);

  const goodRoadmap = `## GTM\n\n| Lane | What | Owner | PR |\n|---|---|---|---|\n| E8-L1 | thing | @maintainer | no PR yet |\n`;
  const { issues: greenIssues } = findGtmSectionIssues(goodRoadmap);
  report('roadmap GTM gate: GREEN on a complete row', greenIssues.length === 0, greenIssues[0]);

  const realRoadmap = readFileSync(join(REPO_ROOT, 'docs', 'roadmap.md'), 'utf8');
  const { issues: realIssues, laneRowCount } = findGtmSectionIssues(realRoadmap);
  report(`roadmap GTM gate: GREEN on the real docs/roadmap.md (${laneRowCount} lane rows)`, realIssues.length === 0, realIssues[0]);
}

// --- 4. the aggregate checker itself, red then green --------------------------------

{
  const redRun = spawnSync(process.execPath, [SCRIPT_PATH], { encoding: 'utf8' });
  const redFirstLine = redRun.stdout.split('\n')[0];
  const redOk =
    redRun.status === 1 && redFirstLine === 'FAIL: no dira binary found on PATH — E0–E3 have not shipped';
  report('aggregate checker: RED today (no dira on PATH), exit 1, exact first line', redOk, redFirstLine);

  const tmpDir = mkdtempSync(join(tmpdir(), 'dira-stub-'));
  const stubPath = join(tmpDir, 'dira');
  try {
    writeFileSync(stubPath, '#!/bin/sh\necho "stub dira -- selftest only, not a real build"\n');
    chmodSync(stubPath, 0o755);
    const greenRun = spawnSync(process.execPath, [SCRIPT_PATH], {
      encoding: 'utf8',
      env: { ...process.env, PATH: `${tmpDir}${delimiter}${process.env.PATH ?? ''}` },
    });
    const firstLineIsPass = greenRun.stdout.split('\n')[0].startsWith('PASS:');
    report(
      'aggregate checker: GREEN with a stub dira on PATH, nothing else touched, exit 0',
      greenRun.status === 0 && firstLineIsPass && /^READY/m.test(greenRun.stdout),
      greenRun.stdout.split('\n').join(' | '),
    );
  } finally {
    rmSync(tmpDir, { recursive: true, force: true });
  }
}

console.log('');
console.log(failed ? 'check-launch-readiness.selftest: FAIL' : 'check-launch-readiness.selftest: PASS (every red case failed, every green case passed)');
process.exitCode = failed ? 1 : 0;
