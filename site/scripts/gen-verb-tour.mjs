// W1-T4: generates the docs page's verb tour — one curated line plus one REAL
// output block per dira command — from the locally built binary, so the page
// can never list a command the binary doesn't have, or omit one it does.
//
// The verb list itself comes from `dira --help` (parseCommands), never from a
// hand-typed array — that is the part that cannot go stale. The one-line
// blurb is hand-curated prose (DESCRIPTIONS below) because the raw --help
// summary is written for a terminal, not a landing page; completeness of
// THAT map is what this script enforces: every verb dira --help reports MUST
// have a line here, or the build fails loudly instead of silently shipping a
// verb with no description.
//
// The output block is `dira help <verb>` for every verb, deliberately never
// `dira <verb> -h` or a real invocation — several verbs mutate state
// (`log`, `supersede`, `install-skill`, `install-hooks`, `reindex`) and this
// script must be safe to run against a real checkout, including the
// maintainer's own machine and CI. `dira help <verb>` is uniformly read-only
// for every command (verified: exit 0 for all fifteen, where `<verb> -h`
// alone is not — `reindex -h` falls through to the top-level usage screen
// with exit 2 instead of its own, an inconsistency worth flagging upstream
// but not this lane's file to fix).
import { writeFileSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { buildBinary, runDira, parseCommands } from "./lib/dira.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const OUT_PATH = join(here, "..", "src", "generated", "verb-tour.json");

// Hand-curated, one line each, friendlier than the raw --help summary.
// Completeness against the REAL command list is enforced below — this is
// the map that must never fall behind the binary.
export const DESCRIPTIONS = {
  help: "Show usage for dira itself or for one command.",
  init: "Seed a new personal or workspace ledger by answering a short interview.",
  log: "Write a new entry to the ledger, or add an edge to one that already exists.",
  sniff: "Read the current session transcript for decisions and stage them for review.",
  distill: "Review what sniff staged for you — one keystroke per entry: confirm or ignore.",
  check: "Refuse a plan that contradicts a settled decision, quoting the record back at you.",
  brief: "Print the session brief: what is blocked, the current focus, what was decided recently.",
  why: "Print the chain behind an entry — what it arises from, and every alternative it refused.",
  supersede: "Retire an entry in favour of the one that replaces it.",
  map: "Join the ledger to kazi's execution status at read time — what is planned, running, blocked, or done.",
  ui: "Serve the ledger index and the decision pages on localhost, read-only.",
  "install-skill": "Write dira's capture skill into ~/.claude for Claude Code to load.",
  "install-hooks": "Merge dira's SessionStart / Stop / PreCompact hook registrations into a settings file.",
  reindex: "Rebuild the derived SQLite read cache from the entry files — the files always win.",
  import: "Measure a directory of existing ADRs and offer to import or index them.",
  version: "Print the dira version this binary was built at.",
};

export function buildTour(helpText) {
  const commands = parseCommands(helpText);
  const missing = commands.filter((c) => !(c.name in DESCRIPTIONS));
  if (missing.length > 0) {
    throw new Error(
      `gen-verb-tour: ${missing.length} verb(s) reported by \`dira --help\` have no curated ` +
        `description in DESCRIPTIONS (site/scripts/gen-verb-tour.mjs): ` +
        `${missing.map((c) => c.name).join(", ")}. Add one line per verb before the site can build.`,
    );
  }
  return commands;
}

function main() {
  buildBinary();
  const helpText = runDira(["--help"]);
  const commands = buildTour(helpText);

  const tour = commands.map((c) => ({
    name: c.name,
    line: DESCRIPTIONS[c.name],
    output: runDira(["help", c.name]).trimEnd(),
  }));

  mkdirSync(dirname(OUT_PATH), { recursive: true });
  writeFileSync(OUT_PATH, JSON.stringify(tour, null, 2) + "\n");
  console.log(`gen-verb-tour: wrote ${tour.length} verbs to ${OUT_PATH}`);
}

// Only run when invoked directly (`node scripts/gen-verb-tour.mjs`), not when
// imported by the selftest below.
if (import.meta.url === `file://${process.argv[1]}`) {
  main();
}
