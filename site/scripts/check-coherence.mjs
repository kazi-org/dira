// W2-T7 README <-> website coherence drift-check (mirrors kazi's
// scripts/check-coherence.mjs, ADR-0018): the canonical strings the site
// shows MUST appear verbatim in ../README.md, and every `dira <verb>`
// invocation example in README's own fenced code blocks must be a verb the
// CURRENTLY BUILT binary actually has — so the marketing site and the
// README can never silently diverge, in either direction. Reads README.md
// fresh on every run (never a cached copy), which is what lets this run
// regardless of merge order against a sibling lane that is also editing it.
//
// Run: `npm --prefix site run check:coherence` (or `node site/scripts/check-coherence.mjs`).
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { STRAPLINE, INSTALL_CMD } from "../src/canonical.mjs";
import { buildBinary, runDira, parseCommands, REPO_ROOT } from "./lib/dira.mjs";

export function extractDiraInvocations(readme) {
  const fences = [...readme.matchAll(/```[\s\S]*?```/g)].map((m) => m[0]);
  const verbs = new Set();
  for (const fence of fences) {
    for (const m of fence.matchAll(/(?:^|\n)\$?\s*dira ([a-z][a-z-]*)/g)) {
      verbs.add(m[1]);
    }
  }
  return [...verbs].sort();
}

export function checkCoherence(readme, liveVerbs) {
  const problems = [];

  if (!readme.includes(STRAPLINE)) {
    problems.push(`strapline not found verbatim in README.md: ${JSON.stringify(STRAPLINE)}`);
  }
  if (!readme.includes(INSTALL_CMD)) {
    problems.push(`install command not found verbatim in README.md: ${JSON.stringify(INSTALL_CMD)}`);
  }

  const liveSet = new Set(liveVerbs);
  for (const verb of extractDiraInvocations(readme)) {
    if (!liveSet.has(verb)) {
      problems.push(
        `README shows \`dira ${verb}\` as a runnable example, but the currently built binary has no ` +
          `such command (it has: ${liveVerbs.join(", ")})`,
      );
    }
  }

  return problems;
}

function main() {
  const readmePath = join(REPO_ROOT, "README.md");
  const readme = readFileSync(readmePath, "utf8");

  buildBinary();
  const liveVerbs = parseCommands(runDira(["--help"])).map((c) => c.name);

  const problems = checkCoherence(readme, liveVerbs);

  if (problems.length > 0) {
    console.error("README <-> website coherence check FAILED.");
    for (const p of problems) console.error(`  - ${p}`);
    console.error("\nFix: make README.md and site/src/canonical.mjs (or the CLI) agree.");
    process.exit(1);
  }

  console.log("README <-> website coherence OK.");
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main();
}
