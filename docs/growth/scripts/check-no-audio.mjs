#!/usr/bin/env node
// docs/growth/scripts/check-no-audio.mjs
//
// E8-L4-T2 (docs/plan/tasks/E8-L4.md). Wraps `ffprobe -show_streams
// -print_format json`, but the parsing logic itself is tested directly
// against a committed ffprobe-shaped JSON fixture (--ffprobe-json), so the
// two-sided proof needs neither ffmpeg installed nor a real video file.
//
// Usage:
//   node check-no-audio.mjs --ffprobe-json <path-to-json>   (test mode, no ffmpeg)
//   node check-no-audio.mjs <path-to-cut>                   (real mode: shells
//                                                             out to ffprobe)
// Exit 0: zero audio streams. Prints "0 audio streams".
// Exit 1: at least one audio stream. Names the audio codec found.
// Exit 2: usage error, or ffprobe itself failed.

import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';

export function countAudioStreams(ffprobeJsonText) {
  const parsed = JSON.parse(ffprobeJsonText);
  const streams = Array.isArray(parsed.streams) ? parsed.streams : [];
  return streams.filter((s) => s.codec_type === 'audio');
}

export function checkNoAudio(ffprobeJsonText) {
  const audioStreams = countAudioStreams(ffprobeJsonText);
  if (audioStreams.length > 0) {
    const codec = audioStreams[0].codec_name ?? 'unknown codec';
    return {
      ok: false,
      reason: `${audioStreams.length} audio stream(s) found, codec: ${codec}`,
    };
  }
  return { ok: true, count: 0 };
}

// ---- CLI -----------------------------------------------------------------
if (import.meta.url === `file://${process.argv[1]}`) {
  const args = process.argv.slice(2);
  let ffprobeJsonText;
  let sourceLabel;

  if (args[0] === '--ffprobe-json') {
    const path = args[1];
    if (!path) {
      console.error('usage: node check-no-audio.mjs --ffprobe-json <path-to-json>');
      process.exit(2);
    }
    ffprobeJsonText = readFileSync(path, 'utf8');
    sourceLabel = path;
  } else {
    const path = args[0];
    if (!path) {
      console.error('usage: node check-no-audio.mjs <path-to-cut>');
      process.exit(2);
    }
    try {
      ffprobeJsonText = execFileSync(
        'ffprobe',
        ['-v', 'error', '-show_streams', '-print_format', 'json', path],
        { encoding: 'utf8' }
      );
    } catch (e) {
      console.error(`FAIL: ffprobe could not read ${path}: ${e.message}`);
      process.exit(2);
    }
    sourceLabel = path;
  }

  const result = checkNoAudio(ffprobeJsonText);
  if (!result.ok) {
    console.error(`FAIL: ${result.reason} (${sourceLabel})`);
    process.exit(1);
  }
  console.log(`check-no-audio PASS — 0 audio streams (${sourceLabel}).`);
  process.exit(0);
}
