// fixture-schema.mjs — the one wrapper gates.mjs needed to run a `go test`
// invocation as a gate row with the same pass/control shape every other row
// already has (E6-L3-T7).
//
// docs/decisions-pending/E6-L2-report.md §8 finding 6 already established
// that 18/18 fixture entries validate and 18/18 invalid ones are correctly
// refused — internal/ui/fixtures_test.go proves it — but that is a `go test`
// invocation, and docs/design/scripts/gates.mjs is the one command
// docs/design/DESIGN.md names as the thing that stops a requirement from
// silently going unmet at a busy moment. This script is what lets
// TestDesignFixturesValidate be a gates.mjs row rather than a check nobody
// remembers to run.
//
//   node docs/design/scripts/fixture-schema.mjs                # the pass case
//   node docs/design/scripts/fixture-schema.mjs --probe-invalid # the control (must FAIL by name)
//
// Exit codes: 0 pass, 1 fail (or, for --probe-invalid, "correctly caught the
// corrupted file" — see below), 2 harness could not run.

import { spawnSync } from 'node:child_process';
import { mkdtemp, cp, readFile, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve, dirname } from 'node:path';

const HERE = dirname(new URL(import.meta.url).pathname);
const ROOT = resolve(HERE, '../../..');
const FIXTURE_ENTRIES = resolve(ROOT, 'docs/design/fidelity/fixtures/ledger-design/entries');
const PROBE = process.argv.includes('--probe-invalid');

const die = (code, msg) => { console.error(msg); process.exit(code); };

function runGoTest(env) {
  return spawnSync('go', ['test', './internal/ui', '-run', 'TestDesignFixturesValidate', '-v'], {
    cwd: ROOT, encoding: 'utf8', env: { ...process.env, ...env },
  });
}

if (!PROBE) {
  const r = runGoTest({});
  if (r.error) die(2, `could not run go test: ${r.error}`);
  process.stdout.write(r.stdout);
  process.stderr.write(r.stderr);
  process.exit(r.status === 0 ? 0 : 1);
}

// ---- --probe-invalid: corrupt a COPY, point the env var at it, expect a named refusal ----
const work = await mkdtemp(join(tmpdir(), 'dira-fixture-schema-probe-'));
await cp(FIXTURE_ENTRIES, work, { recursive: true });

// dec-0011.md is one of the two fixture entries T1 made actionable; any file
// works, but naming a real one (rather than a synthetic id) is what lets
// this probe's assertion below check the failure names the RIGHT file.
const target = 'dec-0011.md';
const path = join(work, target);
const before = await readFile(path, 'utf8');
if (!before.includes('title:')) die(2, `${target} has no title: field to strip; pick a different probe target`);
const corrupted = before
  .split('\n')
  .filter(line => !line.startsWith('title:'))
  .join('\n');
if (corrupted === before) die(2, 'stripping title: did not change anything; the control stages no defect');
await writeFile(path, corrupted);

const r = runGoTest({ DIRA_FIXTURE_ENTRIES_DIR: work });

if (r.status === 0) {
  console.log('PROBE FAILED — the validator accepted a fixture with no title:; the check above is blind');
  process.exit(3);
}
if (!r.stdout.includes(target) && !r.stderr.includes(target)) {
  console.log(`PROBE FAILED — the test failed, but did not name ${target} by name:\n${r.stdout}\n${r.stderr}`);
  process.exit(3);
}
console.log(`PROBE OK — the corrupted ${target} (no title:) was caught by name`);
process.exit(1); // the failure IS the proof, per gates.mjs's control convention
