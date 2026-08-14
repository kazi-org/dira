// E8-L1 (docs/plan/lanes/E8.md) growth-plan checker. Zero third-party dependencies.
//
// Validates docs/growth/channels.md + docs/growth/experiments.md (or a fixture pair
// passed as a directory argument) against the lane's acc line:
//   - exactly 19 channels rated, exactly 3 in the inner ring
//   - every inner-ring channel row names an owner and a cadence
//   - every pre-registered threshold is a rate (has a denominator) with an n-minimum
//   - no banned-hype term appears outside a marked honest-limits block
//
// Usage: node check-growth-plan.mjs [dir]   (default: docs/growth, relative to repo root)
// Exit 0 on a clean plan; exit 1 with a single named reason on the first violation found.
import { readFileSync, existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join, resolve } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, "..", "..", "..");

const EXPECTED_TOTAL_CHANNELS = 19;
const EXPECTED_INNER_COUNT = 3;

// Generic marketing hype (product-marketing.md §10 "Don't") plus specific virality
// claims (absolute #2 / dec-0010's honest limit). Deliberately excludes the bare words
// "viral"/"virality" — Traction's own channel is named "Viral Marketing" and must be
// nameable in the channel taxonomy without tripping the check; what's banned is
// *claiming* virality, not naming the channel that doesn't apply here.
export const BANNED_HYPE_TERMS = [
  "revolutionary",
  "seamless",
  "supercharge",
  "10x",
  "ai-powered",
  "game-changing",
  "game changer",
  "disrupt",
  "disruptive",
  "paradigm shift",
  "best-in-class",
  "cutting-edge",
  "next-generation",
  "groundbreaking",
  "unicorn",
  "hockey stick",
  "go viral",
  "goes viral",
  "going viral",
  "viral growth",
  "viral loop",
  "k-factor",
  "network effect",
  "invite mechanic",
  "exponential growth",
];

function fail(reason) {
  console.error(`FAIL: ${reason}`);
  process.exit(1);
}

function readOrFail(path, label) {
  if (!existsSync(path)) fail(`${label} not found at ${path}`);
  return readFileSync(path, "utf8");
}

// --- honest-limits exemption ---
export function stripHonestLimitsBlocks(text) {
  return text.replace(
    /<!--\s*honest-limits:start\s*-->[\s\S]*?<!--\s*honest-limits:end\s*-->/g,
    "",
  );
}

function scanForBannedTerms(text, fileLabel) {
  const lower = stripHonestLimitsBlocks(text).toLowerCase();
  for (const term of BANNED_HYPE_TERMS) {
    if (lower.includes(term)) {
      fail(
        `banned-hype term "${term}" found in ${fileLabel} outside any ` +
          `<!-- honest-limits:start/end --> block`,
      );
    }
  }
}

// --- channels table ---
function extractMarked(text, marker, fileLabel) {
  const re = new RegExp(
    `<!--\\s*growth-plan:${marker}:start\\s*-->([\\s\\S]*?)<!--\\s*growth-plan:${marker}:end\\s*-->`,
  );
  const m = text.match(re);
  if (!m) fail(`could not find growth-plan:${marker} markers in ${fileLabel}`);
  return m[1];
}

function parseTableRows(block) {
  const lines = block
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => l.startsWith("|") && l.endsWith("|"));
  const rows = lines.map((l) =>
    l
      .slice(1, -1)
      .split("|")
      .map((c) => c.trim()),
  );
  // drop the markdown separator row (all cells match /^:?-+:?$/)
  return rows.filter((cells) => !cells.every((c) => /^:?-+:?$/.test(c)));
}

function isBlank(v) {
  const t = (v || "").trim().toLowerCase();
  return t === "" || t === "-" || t === "—" || t === "n/a";
}

function checkChannels(channelsText) {
  const block = extractMarked(channelsText, "channels-table", "channels.md");
  const rows = parseTableRows(block);
  if (rows.length < 2) fail("channels.md channel table has no data rows");

  const header = rows[0];
  const dataRows = rows.slice(1);

  if (dataRows.length !== EXPECTED_TOTAL_CHANNELS) {
    fail(
      `channels.md rates ${dataRows.length} channels, expected exactly ${EXPECTED_TOTAL_CHANNELS}`,
    );
  }

  const idx = {
    channel: header.indexOf("Channel"),
    ring: header.indexOf("Ring"),
    owner: header.indexOf("Owner"),
    cadence: header.indexOf("Cadence"),
  };
  for (const [name, i] of Object.entries(idx)) {
    if (i === -1) fail(`channels.md table is missing the "${name}" column`);
  }

  let innerCount = 0;
  for (const row of dataRows) {
    const channelName = (row[idx.channel] || "").replace(/\*/g, "").trim();
    const ring = (row[idx.ring] || "").replace(/\*/g, "").trim().toLowerCase();

    if (!["inner", "potential", "long-shot"].includes(ring)) {
      fail(
        `channels.md row "${channelName}" has ring "${row[idx.ring]}", ` +
          `expected inner/potential/long-shot`,
      );
    }

    if (ring === "inner") {
      innerCount += 1;
      if (isBlank(row[idx.owner])) {
        fail(`channels.md inner-ring row "${channelName}" is missing an owner`);
      }
      if (isBlank(row[idx.cadence])) {
        fail(`channels.md inner-ring row "${channelName}" is missing a cadence`);
      }
    }
  }

  if (innerCount !== EXPECTED_INNER_COUNT) {
    fail(
      `channels.md has ${innerCount} inner-ring channels, expected exactly ${EXPECTED_INNER_COUNT}`,
    );
  }
}

// --- experiments.md thresholds ---
function checkExperiments(experimentsText) {
  const specHeaderRe = /^###\s+(EXP-\d+):/gm;
  const specStarts = [...experimentsText.matchAll(specHeaderRe)];
  if (specStarts.length === 0) {
    fail("experiments.md has no `### EXP-NNN:` pre-registered specs");
  }

  for (let i = 0; i < specStarts.length; i++) {
    const id = specStarts[i][1];
    const start = specStarts[i].index;
    const end = i + 1 < specStarts.length ? specStarts[i + 1].index : experimentsText.length;
    const specBody = experimentsText.slice(start, end);

    // The threshold value may wrap onto indented continuation lines in the source
    // markdown, so capture everything up to the next "- **" bullet or a blank line.
    const thresholdMatch = specBody.match(
      /-\s*\*\*Threshold:\*\*\s*([\s\S]*?)(?:\n\s*-\s*\*\*|\n\s*\n|$)/,
    );
    if (!thresholdMatch) {
      fail(`${id} has no "- **Threshold:**" line`);
    }
    const thresholdLine = thresholdMatch[1].replace(/\s+/g, " ").trim();

    const hasRate = /%|÷|\bper\b|\/\s*\S/.test(thresholdLine);
    const hasNMin = /n\s*[≥>=]+\s*\d+/i.test(thresholdLine);

    if (!hasRate || !hasNMin) {
      const missing = [];
      if (!hasRate) missing.push("a denominator (rate/percentage)");
      if (!hasNMin) missing.push("an n-minimum");
      fail(
        `${id}'s threshold is a raw count — missing ${missing.join(" and ")}: ` +
          `"${thresholdLine.trim()}"`,
      );
    }
  }

  return specStarts.length;
}

// --- run ---
// Guarded so importing this module for its exports (BANNED_HYPE_TERMS,
// stripHonestLimitsBlocks) never triggers a full run against argv/cwd --
// same pattern as check-drafts.mjs's isMain guard, added when E8-L6 needed
// to reuse the hype-term list rather than hand-maintain a second copy.
function main() {
  const targetDir = process.argv[2]
    ? resolve(process.cwd(), process.argv[2])
    : join(repoRoot, "docs", "growth");

  const channelsPath = join(targetDir, "channels.md");
  const experimentsPath = join(targetDir, "experiments.md");

  const channelsText = readOrFail(channelsPath, "channels.md");
  const experimentsText = readOrFail(experimentsPath, "experiments.md");

  checkChannels(channelsText);
  const specCount = checkExperiments(experimentsText);
  scanForBannedTerms(channelsText, "channels.md");
  scanForBannedTerms(experimentsText, "experiments.md");

  console.log(
    `growth plan OK: ${EXPECTED_TOTAL_CHANNELS} channels rated, ` +
      `${EXPECTED_INNER_COUNT} inner-ring, ${specCount} pre-registered spec(s), ` +
      `0 banned-hype terms outside honest-limits blocks.`,
  );
}

const isMain = process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1];
if (isMain) main();
