#!/usr/bin/env bash
# assets/demo/record-init.sh — reproduce the secondary demo clip (E8-L3-T4,
# recorded by E8-L4-T5).
#
# Same absence-guard contract as record-check.sh (E8-L3-T3): the committed
# cast (assets/demo/init.cast) is producible only by this script, it runs the
# REAL `dira` binary, and there is no post-processing — every line the cast
# shows is verbatim `dira import` output.
#
# The verb and the target repo, left open by E8-L3 pending dec-0028 and a
# real binary, are resolved here: the verb is `dira import` (`dira init`
# seeds an empty personal/workspace ledger by interview — dec-0003's fixed
# questions, nothing about reading an existing repo's history — so it cannot
# be the "value in 60 seconds from existing ADRs" clip; only `import` reads a
# directory the user already has). The target is `internal/importadr/testdata/
# corpora/bbc-tams`, the real bbc/tams ADR corpus vendored read-only at
# E2-L7-T1 (MANIFEST.md there records provenance: commit 8cd1ca5, 49 files).
# It is one of dec-0028's own evidence-table entries at 90% yield — not
# assumed worth importing, but the corpus that answer's evidence table names,
# and TestExitCriterion (cmd/dira/import_exitcriterion_test.go) proves the
# same fixture against the built binary already: 47 entries, 231 reasons.
#
# The block below is read verbatim by check-cast-drift.mjs and asserted
# against this script's own real output — do not hand-edit one side without
# re-running the recording.
# --- BEGIN CANONICAL DEMO OUTPUT ---
# $ dira import bbc-tams-adrs --yes
#
# 49 documents scanned
# 47 record a rejected option with a reason
# 231 reasons found
# Import the 47 entries that carry a reason?
# --- END CANONICAL DEMO OUTPUT ---
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
DIRA="${DIRA:-dira}"
CORPUS_SRC="$ROOT/internal/importadr/testdata/corpora/bbc-tams"

if ! command -v "$DIRA" >/dev/null 2>&1; then
  echo "record-init.sh: dira is not on PATH — nothing recorded, no cast written" >&2
  exit 1
fi

# DEMO_DIR lets a caller pin the working directory for post-recording
# inspection (check-init-ledger.mjs, E8-L4-T5) instead of a mktemp path that
# is gone once this script returns. Nothing about the recorded output
# changes either way.
DEMO="${DEMO_DIR:-$(mktemp -d)/demo}"
mkdir -p "$DEMO/.dira/entries"
mkdir -p "$DEMO/bbc-tams-adrs"
cp "$CORPUS_SRC"/*.md "$DEMO/bbc-tams-adrs/"
rm -f "$DEMO/bbc-tams-adrs/MANIFEST.md"

clear
printf '$ dira import bbc-tams-adrs --yes\n'
cd "$DEMO"
exec "$DIRA" import bbc-tams-adrs --yes
