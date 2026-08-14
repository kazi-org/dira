#!/usr/bin/env bash
#
# ci-local.sh — every gate .github/workflows/ci.yml runs, run here, in one command.
#
# This exists because of cst-0004: a check that only passes in CI is not a gate on
# anything a contributor can reproduce. The workflow and this script must always run
# the same set. If you add a gate to one, add it to the other — and to
# hooks/pre-commit, which runs the subset that is fast enough for every commit.
#
#     scripts/ci-local.sh
#
# Two versions (Playwright, golangci-lint) are READ OUT OF THE WORKFLOW rather than
# restated here, so the workflow stays the single source of truth for them.
#
# Deliberately NOT `set -e`: every gate runs even after an earlier one goes red, and
# the summary at the end names all of them. That mirrors what CI does with parallel
# jobs, and it is the difference between one round trip and four — a push carrying an
# unformatted file AND a failing test should tell you about both at once.
#
# There is no flag to skip a gate. A "ci-local passed" that means different things on
# different machines is worse than no script.

set -uo pipefail

cd "$(dirname "$0")/.." || exit 1
WF=".github/workflows/ci.yml"

if [ -t 1 ]; then B=$'\033[1m'; R=$'\033[0m'; RED=$'\033[31m'; GRN=$'\033[32m'
else B=''; R=''; RED=''; GRN=''; fi

PASSED=()
FAILED=()

run_gate() {
  local name=$1; shift
  printf '\n%s=== %s ===%s\n' "$B" "$name" "$R"
  if "$@"; then PASSED+=("$name"); else FAILED+=("$name"); fi
}

# Pull a workflow-level `env:` value. Fails loudly rather than defaulting: a silent
# default would let this script measure something CI does not.
wf_env() {
  local key=$1 val
  val=$(sed -n "s/^  ${key}: *\"\(.*\)\"\$/\1/p" "$WF")
  if [ -z "$val" ]; then
    echo "could not read ${key} from ${WF}" >&2
    return 1
  fi
  printf '%s' "$val"
}

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required tool: $1${2:+  ($2)}" >&2
    return 1
  fi
}

# --- Go -------------------------------------------------------------------- #

gate_build() { need go && go build ./...; }

# The second vet is not redundant. internal/perf's files all carry `//go:build
# perf`, so `go vet ./...` does not compile them and a syntax error in that package
# is invisible to every other gate in this script. .golangci.yml carries the same
# tag for the same reason.
gate_vet()   { need go && go vet ./... && go vet -tags perf ./internal/perf; }

# `gofmt -l` exits 0 whether or not it found anything — it reports on stdout. A gate
# that only checks the exit code here can never fail.
gate_gofmt() {
  need go || return 1
  local unformatted
  unformatted=$(gofmt -l .)
  if [ -n "$unformatted" ]; then
    echo "gofmt would rewrite these files — run: gofmt -w <file>"
    printf '  %s\n' "$unformatted"
    return 1
  fi
  echo "gofmt: clean"
}

gate_golangci() {
  need golangci-lint "https://golangci-lint.run/welcome/install/" || return 1
  local want have
  want=$(wf_env GOLANGCI_LINT_VERSION) || return 1
  have=$(golangci-lint --version 2>/dev/null | sed -n 's/.*has version \([^ ]*\).*/\1/p')
  if [ "$have" != "${want#v}" ]; then
    echo "note: CI pins golangci-lint ${want}, this machine has ${have:-unknown}."
    echo "      Findings may differ from the run that gates the push."
  fi
  golangci-lint run ./...
}

# -race because internal/index and internal/ledger/fixture carry race_on / race_off
# build-tagged tests that only mean anything under the detector. -count=1 because a
# cached PASS is not a run. The count assertion is the point of the wrapper:
# `go test ./...` over a module with no test files exits 0 and prints "no test
# files" — green, and proving nothing.
gate_test() {
  need go || return 1
  local log status ran
  log=$(mktemp)
  go test -race -count=1 -v ./... 2>&1 | tee "$log"
  status=${PIPESTATUS[0]}
  ran=$(awk '/^=== RUN/ { n++ } END { print n+0 }' "$log")
  rm -f "$log"
  echo "top-level tests executed: $ran"
  [ "$status" -eq 0 ] || return 1
  if [ "$ran" -eq 0 ]; then
    echo "go test executed 0 tests — a green run over an empty suite is not a gate."
    return 1
  fi
}

# The latency gate (int-0002, E1-L6): the same invocation .github/workflows/ci.yml's
# `perf` job runs, so a contributor can reproduce it rather than take CI's word for
# it. internal/perf is tagged out of `go test ./...` on purpose — it times the
# SHIPPED BINARY, spawned, and a timing test that competes with the rest of the
# suite reports the load average. `-p 1` and a run of its own is the whole method.
#
# It is also the ONE gate here whose verdict depends on the machine and not only on
# the tree. Every other gate is a function of the commit; this one is a function of
# the commit and the hardware, and on a busy machine it is red for reasons the
# commit did not cause — `dira version`, an invocation that opens no ledger at all,
# has been measured at a 95-107ms median through this harness on the maintainer's
# machine, above warmBudget's 80ms ceiling. Read the published MINIMUM before
# believing a red: a minimum far under the ceiling with the median over it is
# contention, and a minimum beside the median is dira. The binding verdict is CI's,
# taken on a runner that is doing nothing else; ci.yml's `perf` job comment carries
# the runner-variance policy and what it rejected.
#
# No skip flag, for the reason at the top of this file. A "ci-local passed" that
# quietly omitted the latency gate would mean something different from what CI
# means by it.
gate_perf() {
  need go || return 1
  local log status ran rc=0
  log=$(mktemp)
  go test -tags perf -count=1 -p 1 -v ./internal/perf 2>&1 | tee "$log"
  status=${PIPESTATUS[0]}

  # The same two assertions the CI job makes, and for the same reasons. `=== RUN`
  # count: a perf build tag that stops matching produces "no test files", exit 0
  # and a green gate over nothing, and this package is the only place in the repo
  # where that failure is invisible to everything else. Named verdicts: `=== RUN`
  # is printed BEFORE a test skips, so all three budgets can skip and still be
  # counted — a skipped budget is not a met budget.
  ran=$(awk '/^=== RUN/ { n++ } END { print n+0 }' "$log")
  echo "tests and subtests executed: $ran"
  if [ "$ran" -eq 0 ]; then
    echo "go test -tags perf executed 0 tests — either the perf build tag no longer matches"
    echo "internal/perf's files, or the package did not compile."
    rc=1
  fi
  # dec-0029's minimum-discriminator is the one exception, mirroring ci.yml's
  # perf job exactly: a test that SKIPPED naming "NOT MEASURABLE" means its own
  # MINIMUM decided this machine cannot reach a verdict at all, and that is not
  # a budget that was met but is also not this gate's business to fail — CI's
  # dedicated runner is the authority. Any other skip still fails here.
  for t in TestColdStartBudget TestWarmBriefBudget TestReindexBudget; do
    block=$(awk -v name="$t" '
      $0 ~ ("^=== RUN   " name "$") { capture=1 }
      capture { print }
      capture && $0 ~ ("^--- (PASS|FAIL|SKIP): " name " ") { exit }
    ' "$log")
    if printf '%s\n' "$block" | grep -qE "^--- (PASS|FAIL): ${t} "; then
      continue
    fi
    if printf '%s\n' "$block" | grep -qE "^--- SKIP: ${t} " \
      && printf '%s\n' "$block" | grep -q 'NOT MEASURABLE'; then
      echo "$t: NOT MEASURABLE here (dec-0029) — this machine's own minimum could not judge it; not a pass, not a failure."
      continue
    fi
    echo "$t did not run to a verdict — a budget that skipped without the minimum-discriminator's own reason is not a budget that was met."
    rc=1
  done
  # On linux the socket assertion is not allowed to skip: it is the only platform
  # where it is more than a skip, which is what internal/perf/NETWORK.md says the
  # CI job exists to fix. On darwin it skips (dtrace needs SIP disabled) and
  # NETWORK.md's table records that, so it is reported below rather than asserted.
  if [ "$(uname -s)" = "Linux" ] && ! grep -qE '^--- (PASS|FAIL): TestTheBriefOpensNoSocket ' "$log"; then
    echo "TestTheBriefOpensNoSocket did not run to a verdict — see internal/perf/NETWORK.md;"
    echo "a tracer that cannot attach certifies nothing."
    rc=1
  fi
  grep -E '^[[:space:]]*--- SKIP: ' "$log"
  rm -f "$log"
  [ "$status" -eq 0 ] && [ "$rc" -eq 0 ]
}

# --- repo gates ------------------------------------------------------------ #

gate_coverage() { need python3 && python3 scripts/coverage.py; }
gate_privacy()  { need python3 && python3 scripts/privacy-lint.py; }
gate_contrast() { need node && node docs/design/scripts/contrast.mjs; }

# The rendered-contrast gate needs a real browser: every chip sits on a
# `color-mix(in oklab, ...)` tint of its own colour, and that pair is invisible to
# the token matrix. Installing Playwright is part of the gate, not a precondition to
# it — if it cannot be installed, the gate is red, not skipped.
gate_rendered() {
  need node && need npm || return 1
  local want have
  want=$(wf_env PLAYWRIGHT_VERSION) || return 1
  have=$(node -p "require('playwright/package.json').version" 2>/dev/null)
  if [ "$have" != "$want" ]; then
    echo "installing playwright@${want} (found: ${have:-none})"
    npm install --no-save --no-package-lock --no-audit --no-fund "playwright@${want}" || return 1
    ./node_modules/.bin/playwright install chromium || return 1
  fi
  node docs/design/scripts/contrast-rendered.mjs
}

# --- run ------------------------------------------------------------------- #

run_gate "go build ./..."            gate_build
run_gate "go vet ./... +perf tag"    gate_vet
run_gate "gofmt -l ."                gate_gofmt
run_gate "golangci-lint run ./..."   gate_golangci
run_gate "go test -race -count=1"    gate_test
run_gate "perf budgets (int-0002)"   gate_perf
run_gate "coverage register"         gate_coverage
run_gate "privacy lint (cst-0003)"   gate_privacy
run_gate "token contrast"            gate_contrast
run_gate "rendered contrast"         gate_rendered

printf '\n%s=== summary ===%s\n' "$B" "$R"
for g in "${PASSED[@]:-}"; do [ -n "$g" ] && printf '  %sPASS%s  %s\n' "$GRN" "$R" "$g"; done
for g in "${FAILED[@]:-}"; do [ -n "$g" ] && printf '  %sFAIL%s  %s\n' "$RED" "$R" "$g"; done

if [ ${#FAILED[@]} -gt 0 ]; then
  printf '\n%s%d gate(s) red — this push would fail CI.%s\n' "$RED" "${#FAILED[@]}" "$R"
  exit 1
fi
printf '\n%sall %d gates green — same set %s runs.%s\n' "$GRN" "${#PASSED[@]}" "$WF" "$R"
