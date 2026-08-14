#!/usr/bin/env bash
# assets/demo/record-check.sh — reproduce the primary demo clip (E8-L3-T3).
#
# Mirrors kazi/priv/examples/hero_cast_demo/record.sh's contract: the
# committed cast (assets/demo/check.cast, built by E8-L4) is producible only
# by this script, it runs the REAL `dira` binary against a fresh copy of
# fixtures/demo-ledger/ (E8-L3-T1), and there is no post-processing — every
# line the cast shows is verbatim `dira check` output.
#
# `dira` does not exist yet (see docs/plan.md). Every run of this script
# today takes the absence branch below and writes no cast.
#
#   ./assets/demo/record-check.sh                  # runs to a real terminal
#   asciinema rec assets/demo/check.cast \
#     --output-format asciicast-v2 --overwrite \
#     --window-size 92x14 -c "./assets/demo/record-check.sh"
#
# The block below is read verbatim by check-demo-script.mjs and asserted
# against .agents/product-marketing.md §6 — do not hand-edit one side
# without the other.
# --- BEGIN CANONICAL DEMO OUTPUT ---
# $ dira check "add a background daemon to track run state"
#
# ✗ conflicts with dec-0060 (accepted 2026-07-03)
#     rejected alternative: "a daemon"
#     why_not: violates the single-binary intent (int-0002)
#     revisit_if: cold-start latency stops being the binding constraint
#
# → supersede dec-0060, or revise the plan
# --- END CANONICAL DEMO OUTPUT ---
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
DIRA="${DIRA:-dira}"
FIXTURE="$ROOT/fixtures/demo-ledger"
PLAN='add a background daemon to track run state'

if ! command -v "$DIRA" >/dev/null 2>&1; then
  echo "record-check.sh: dira is not on PATH — nothing recorded, no cast written" >&2
  exit 1
fi

# The fixture ledger is a flat directory of *.md, not a ledger `dira` can
# open directly (docs/lore.md L-0014) — copy it into a fresh .dira/entries/
# first, exactly like `dira check -C <path>` expects.
DEMO="$(mktemp -d)/demo"
mkdir -p "$DEMO/.dira/entries"
cp "$FIXTURE"/*.md "$DEMO/.dira/entries/"

clear
printf '$ dira check "%s"\n' "$PLAN"
cd "$DEMO"
exec "$DIRA" check "$PLAN"
