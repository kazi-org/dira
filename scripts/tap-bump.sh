#!/usr/bin/env bash
# scripts/tap-bump.sh — publishes Formula/dira.rb to kazi-org/homebrew-tap
# (or, in tests, to a local fixture standing in for it).
#
# Stages Formula/dira.rb BY NAME, never `git add -A`, and never force-pushes.
# This is the one property every task under docs/plan/tasks/E0-L5.md's
# E0-L5-T2 must be able to demonstrate failing: kazi's own tap-bump job
# (kazi-org/kazi's release-please.yml) does `git add Formula/kazi.rb` for
# exactly one reason — a generator or a force-push touching the whole
# Formula/ directory clobbers Formula/kazi.rb and breaks `brew install kazi`
# for every kazi user, a production failure in another product this script
# exists not to cause.
#
# Usage:
#   HOMEBREW_TAP_TOKEN=<pat> scripts/tap-bump.sh \
#     --version <version> \
#     --darwin-arm64-url <url> --darwin-arm64-sha256 <sha256> \
#     --darwin-amd64-url <url> --darwin-amd64-sha256 <sha256> \
#     --linux-amd64-url  <url> --linux-amd64-sha256  <sha256> \
#     --remote <git-remote-url-or-path>
#
# With HOMEBREW_TAP_TOKEN unset, this prints a warning naming the variable
# and exits 0 without touching the remote at all — the fine-grained PAT it
# needs (contents:write on kazi-org/homebrew-tap only) is mintable only by
# the repo owner, so a run with none configured is expected, not an error.

set -euo pipefail

version=""
darwin_arm64_url=""
darwin_arm64_sha256=""
darwin_amd64_url=""
darwin_amd64_sha256=""
linux_url=""
linux_sha256=""
remote=""

while [ $# -gt 0 ]; do
  case "$1" in
    --version) version="$2"; shift 2 ;;
    --darwin-arm64-url) darwin_arm64_url="$2"; shift 2 ;;
    --darwin-arm64-sha256) darwin_arm64_sha256="$2"; shift 2 ;;
    --darwin-amd64-url) darwin_amd64_url="$2"; shift 2 ;;
    --darwin-amd64-sha256) darwin_amd64_sha256="$2"; shift 2 ;;
    --linux-amd64-url) linux_url="$2"; shift 2 ;;
    --linux-amd64-sha256) linux_sha256="$2"; shift 2 ;;
    --remote) remote="$2"; shift 2 ;;
    *) echo "tap-bump.sh: unknown argument: $1" >&2; exit 1 ;;
  esac
done

for pair in "version:$version" "darwin-arm64-url:$darwin_arm64_url" "darwin-arm64-sha256:$darwin_arm64_sha256" \
            "darwin-amd64-url:$darwin_amd64_url" "darwin-amd64-sha256:$darwin_amd64_sha256" \
            "linux-amd64-url:$linux_url" "linux-amd64-sha256:$linux_sha256" "remote:$remote"; do
  name="${pair%%:*}"
  val="${pair#*:}"
  if [ -z "$val" ]; then
    echo "tap-bump.sh: --$name is required" >&2
    exit 1
  fi
done

if [ -z "${HOMEBREW_TAP_TOKEN:-}" ]; then
  echo "tap-bump.sh: HOMEBREW_TAP_TOKEN is not set; skipping the tap publish (no clone, no commit, no push)." >&2
  exit 0
fi

# DIRA_MODULE_ROOT lets internal/tapbump's tests point a MUTATED COPY of
# this script (run from a temp directory, to prove the byte-unchanged guard
# both ways) back at the real module, since BASH_SOURCE alone would resolve
# to the copy's own temp directory instead. Unset in every real invocation,
# where the normal BASH_SOURCE-relative resolution applies.
if [ -n "${DIRA_MODULE_ROOT:-}" ]; then
  repo_root="$DIRA_MODULE_ROOT"
else
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  repo_root="$(cd "$script_dir/.." && pwd)"
fi

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

git clone --quiet "$remote" "$workdir/tap"

formula="$(cd "$repo_root" && go run ./internal/tapformula/gen \
  --version "$version" \
  --darwin-arm64-url "$darwin_arm64_url" --darwin-arm64-sha256 "$darwin_arm64_sha256" \
  --darwin-amd64-url "$darwin_amd64_url" --darwin-amd64-sha256 "$darwin_amd64_sha256" \
  --linux-amd64-url "$linux_url" --linux-amd64-sha256 "$linux_sha256")"

mkdir -p "$workdir/tap/Formula"
# Command substitution above stripped the generator's trailing newline;
# restore exactly one so the published bytes match `go run
# ./internal/tapformula/gen`'s own output byte-for-byte.
printf '%s\n' "$formula" > "$workdir/tap/Formula/dira.rb"

cd "$workdir/tap"

# Idempotent: a second publish of unchanged bytes is a no-op, mirroring
# internal/skill.Install's UNCHANGED precedent (docs/plan/tasks/E2-L3.md).
# On a brand-new Formula/dira.rb, `git ls-files --error-unmatch` fails (not
# yet tracked), so the `&&` short-circuits to false and the commit proceeds
# regardless of what `git diff` reports for an untracked path.
if git ls-files --error-unmatch Formula/dira.rb >/dev/null 2>&1 && git diff --quiet -- Formula/dira.rb; then
  echo "tap-bump.sh: Formula/dira.rb is unchanged; nothing to publish."
  exit 0
fi

git add Formula/dira.rb
git -c user.name="dira release pipeline" -c user.email="noreply@kazi-org.dev" \
  commit --quiet -m "dira ${version}"

branch="$(git branch --show-current)"
git push --quiet origin "HEAD:refs/heads/${branch}"
