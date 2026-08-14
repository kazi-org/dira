#!/usr/bin/env node
// docs/growth/scripts/check-cast-duration.mjs
//
// E8-L4-T1 (docs/plan/tasks/E8-L4.md). Reads an asciicast v2 file's own event
// timestamps -- never wall-clock, never the file's mtime (docs/lore.md L-0009
// is the same class of bug: a filesystem timestamp is blind to what actually
// happened inside the recording) -- and compares the final event's timestamp
// against a bound passed on the command line. This is the ≤20s / ≤60s bar
// dec-0010 sets for the demo clips, made machine-checkable.
//
// Usage: node check-cast-duration.mjs <path-to-cast> <bound-seconds>
// Exit 0: measured duration <= bound. Prints the measured duration.
// Exit 1: measured duration > bound, or the file could not be parsed as
//         asciicast v2. Names the measured duration and the bound exceeded.
// Exit 2: usage error (missing argument, non-numeric bound).

import { readFileSync } from 'node:fs';

// asciicast v2 is one JSON header object followed by one JSON [time, type,
// data] array per line. The recording's own duration is the last event's
// time field -- nothing here reads process.hrtime, Date.now, or fs.statSync.
export function measureCastDuration(castText) {
  const lines = castText.split('\n').filter((l) => l.trim() !== '');
  if (lines.length === 0) {
    return { error: 'empty file: no header line' };
  }
  try {
    JSON.parse(lines[0]);
  } catch (e) {
    return { error: `header line is not valid JSON: ${e.message}` };
  }
  const eventLines = lines.slice(1);
  if (eventLines.length === 0) {
    return { error: 'no events after the header line' };
  }
  const lastLine = eventLines[eventLines.length - 1];
  let event;
  try {
    event = JSON.parse(lastLine);
  } catch (e) {
    return { error: `final event line is not valid JSON: ${e.message}` };
  }
  if (!Array.isArray(event) || typeof event[0] !== 'number') {
    return { error: `final event line is not a [time, type, data] array: ${lastLine}` };
  }
  return { duration: event[0] };
}

export function checkCastDuration(castPath, bound) {
  const castText = readFileSync(castPath, 'utf8');
  const { duration, error } = measureCastDuration(castText);
  if (error) return { ok: false, reason: `${castPath}: ${error}` };
  if (duration > bound) {
    return {
      ok: false,
      reason: `measured duration ${duration}s exceeds the ${bound}s bound`,
      duration,
    };
  }
  return { ok: true, duration };
}

// ---- CLI -----------------------------------------------------------------
if (import.meta.url === `file://${process.argv[1]}`) {
  const castPath = process.argv[2];
  const boundArg = process.argv[3];
  if (!castPath || boundArg === undefined) {
    console.error('usage: node check-cast-duration.mjs <path-to-cast> <bound-seconds>');
    process.exit(2);
  }
  const bound = Number(boundArg);
  if (!Number.isFinite(bound)) {
    console.error(`FAIL: bound "${boundArg}" is not a number`);
    process.exit(2);
  }

  const result = checkCastDuration(castPath, bound);
  if (!result.ok) {
    console.error(`FAIL: ${result.reason}`);
    process.exit(1);
  }
  console.log(`check-cast-duration PASS — measured duration ${result.duration}s <= ${bound}s bound (${castPath}).`);
  process.exit(0);
}
