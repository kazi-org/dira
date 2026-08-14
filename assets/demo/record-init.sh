#!/usr/bin/env bash
# assets/demo/record-init.sh — reproduce the secondary demo clip (E8-L3-T4).
#
# Same absence-guard contract as record-check.sh (E8-L3-T3), for the "value in
# 60 seconds" clip: `dira init` (or `dira import <dir>`, per dec-0028 — see
# below) against a repo with existing history, rendering the decision graph
# and flagging contradictions already present in it.
#
# Which repo, and whether the verb is `dira init` or a separate `dira import`,
# is NOT settled here — docs/plan/tasks/E8-L3.md assigns that call to E8-L4,
# once E0–E3 exist and dec-0028's dry-run behavior is wired. This script only
# builds the guard that must hold regardless of which repo is eventually
# chosen; the recording step below is a placeholder E8-L4 replaces.
#
# `dira` does not exist yet (see docs/plan.md). Every run of this script
# today takes the absence branch below and writes no cast.
#
#   ./assets/demo/record-init.sh                   # runs to a real terminal
#   asciinema rec assets/demo/init.cast \
#     --output-format asciicast-v2 --overwrite \
#     --window-size 92x20 -c "./assets/demo/record-init.sh"
#
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
DIRA="${DIRA:-dira}"

if ! command -v "$DIRA" >/dev/null 2>&1; then
  echo "record-init.sh: dira is not on PATH — nothing recorded, no cast written" >&2
  exit 1
fi

# TODO(E8-L4): resolve which repo and which verb — `dira init` vs
# `dira import <dir>` (dec-0028) — records this clip, then replace this
# placeholder with the real invocation record-check.sh uses as its model.
echo "record-init.sh: target repo and verb not yet decided (see E8-L4, dec-0028) — nothing recorded" >&2
exit 1
