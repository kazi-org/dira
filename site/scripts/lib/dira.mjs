// Shared helpers for talking to the locally built dira binary. Every
// generator under site/scripts/ builds and reads the SAME binary this way, so
// there is exactly one place that knows how to parse `dira --help` and how to
// find the binary on disk (docs/plan/website.md W1-T3/W1-T4: "do not write a
// second renderer" — the same discipline applies to a second command parser).
import { execFileSync, execSync } from "node:child_process";
import { existsSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
export const SITE_ROOT = join(here, "..", "..");
export const REPO_ROOT = join(SITE_ROOT, "..");
export const BIN_DIR = join(SITE_ROOT, ".bin");
export const BIN_PATH = join(BIN_DIR, "dira");

// Builds ./cmd/dira from the repo this worktree checked out, unconditionally
// (a stale binary from a previous commit is exactly the drift T3/T4 exist to
// prevent). Returns the path to the built binary.
export function buildBinary() {
  mkdirSync(BIN_DIR, { recursive: true });
  execSync(`go build -o ${JSON.stringify(BIN_PATH)} ./cmd/dira`, {
    cwd: REPO_ROOT,
    stdio: "inherit",
  });
  if (!existsSync(BIN_PATH)) {
    throw new Error(`dira: go build did not produce ${BIN_PATH}`);
  }
  return BIN_PATH;
}

// Runs the built binary with the given args and returns stdout. dira's own
// `-h` / usage text is written to stdout (verified against cmd/dira/main.go's
// help command), and a nonzero exit is expected for some flags (e.g. `-h`
// itself exits 0, but individual command usage screens can exit 2) — callers
// that need a specific exit code pass allowNonZero.
export function runDira(args, { allowNonZero = true } = {}) {
  try {
    return execFileSync(BIN_PATH, args, { encoding: "utf8" });
  } catch (err) {
    if (allowNonZero && typeof err.stdout === "string") {
      return err.stdout + (err.stderr ?? "");
    }
    throw err;
  }
}

// Parses the `commands:` table out of `dira --help`'s top-level usage text
// into [{ name, summary }], in the order the binary lists them. This is the
// single source of truth for "every verb dira has" — nothing here is a
// hand-maintained list of command names.
export function parseCommands(helpText) {
  const m = helpText.match(/commands:\n\n([\s\S]*?)\n\nflags:/);
  if (!m) {
    throw new Error("dira: could not find a commands: block in --help output");
  }
  return m[1]
    .split("\n")
    .filter((line) => line.trim().length > 0)
    .map((line) => {
      const lm = line.match(/^\t(\S+)\s+(.+)$/);
      if (!lm) throw new Error(`dira: unparsable command line: ${JSON.stringify(line)}`);
      return { name: lm[1], summary: lm[2].trim() };
    });
}
