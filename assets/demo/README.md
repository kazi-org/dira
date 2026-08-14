# Demo clips (E8-L3 harness, E8-L4 recordings)

Two clips back `.agents/product-marketing.md` §6, "The demo (the single most
important marketing asset)": a primary clip showing `dira check` catching a
plan that contradicts a past decision, and a secondary clip showing `dira
import` deliver value from a repo's existing history in the first minute of
adoption. Both are recorded now — `dira` ships (E0–E3), and this file states
the reproduction contract and every post-processing step, per clip, per
kazi's own `record.sh` convention of naming its de-noising line by line rather
than leaving a reader to diff the output.

## Status, today

Both `check.cast` and `init.cast` exist and are committed. Each is producible
only by its recording script (`record-check.sh` / `record-init.sh`) run
against a real `dira` binary — the `command -v dira` guard in both scripts
still fails loudly and writes no cast if `dira` is not on `PATH`, which is
the correct behavior for a script that must never fabricate its output.

## The two clips

### Primary — `dira check`, via `record-check.sh`

Reproduces the exact transcript quoted in `.agents/product-marketing.md` §6:
`dira check` is pointed at a plan that conflicts with `dec-0060` (from
`fixtures/demo-ledger/`, a byte-identical copy of `internal/enforcer`'s
`dec-0060`/`int-0002` fixture — `E8-L3-T1`), and prints the conflict, the
rejected alternative, its `why_not`, and its `revisit_if`.

**Bar:** under **20.0s** (20 seconds) — one command, one screen, no setup and
no narration. Measured: **0.851s** (`check-cast-duration.mjs`).

## check.cast

Recording command (`E8-L4-T4`):

```sh
asciinema rec assets/demo/check.cast \
  --output-format asciicast-v2 --overwrite \
  --window-size 92x14 -c "./assets/demo/record-check.sh"
```

`dira` was on `PATH` for the real, built binary; `record-check.sh` copies
`fixtures/demo-ledger/` into a fresh `.dira/entries/` first (`docs/lore.md`
L-0014 — the fixture is a flat directory, not a ledger `dira` can open in
place) and execs `dira check` directly. No post-processing applied — the
committed `.cast` is the unedited recording.

**Social cut (`E8-L4-T6`):** `check.gif`, a direct GIF encode via `agg`
(`agg 1.9.0`):

```sh
agg assets/demo/check.cast assets/demo/check.gif
```

No post-processing applied beyond that one encode — no trimming, no
re-timing, no cropping. `check.gif` is 15,681 bytes, under the 3 MiB
(3,145,728 byte) HN/X link-preview ceiling by two orders of magnitude, and
carries zero audio streams (`check-no-audio.mjs`).

### Secondary — `dira import`, via `record-init.sh`

Reproduces `dira import` against a real repo's existing ADR corpus,
rendering a non-empty ledger with real recorded contradictions already
present — the "value in 60 seconds" clip. `dec-0028` (semantic import, with a
mandatory pre-import dry run) settles which verb: `dira init` seeds an empty
personal/workspace ledger from a fixed interview (`dec-0003`) and never reads
an existing repo, so only `dira import` can be this clip. The target is
`internal/importadr/testdata/corpora/bbc-tams` — the real bbc/tams ADR
corpus, vendored read-only at `E2-L7-T1` (provenance in that directory's
`MANIFEST.md`: commit `8cd1ca5`, 49 files) — one of `dec-0028`'s own
evidence-table entries at 90% yield, not merely assumed worth importing.
`nulib/meadow` (0% yield, `dec-0028`'s documented negative example) was not
used.

**Bar:** under **60.0s** (60 seconds) — the clip has to make "worth installing
today, not in three months" felt inside a single minute. Measured: **3.313s**
(`check-cast-duration.mjs`).

## init.cast

Recording command (`E8-L4-T5`):

```sh
DEMO_DIR=<a-directory-to-inspect-afterward> \
asciinema rec assets/demo/init.cast \
  --output-format asciicast-v2 --overwrite \
  --window-size 92x20 -c "./assets/demo/record-init.sh"
```

`record-init.sh` copies the bbc/tams corpus (`MANIFEST.md` excluded — it is a
vendoring artifact, not an ADR) into a fresh `bbc-tams-adrs/` next to a fresh
`.dira/entries/`, and execs `dira import bbc-tams-adrs --yes` directly. No
post-processing applied — the committed `.cast` is the unedited recording.
`DEMO_DIR` only pins where the recording writes so it can be inspected after
the fact; it changes nothing about what the recording shows.

**Post-recording ledger check (`E8-L4-T5`):** `check-init-ledger.mjs`, run
against the directory `DEMO_DIR` named, confirms the recorded run actually
produced a non-empty ledger with real contradictions, not merely a plausible
transcript:

```
check-init-ledger PASS — 47 entry(ies) in <dir>/.dira/entries, 47 carrying a
non-empty alternatives[] array.
```

No social cut exists for `init.cast` in this lane — `E8-L4-T6` scopes the cut
to the primary clip only.

## No audio, either clip

Both `.cast` files are terminal recordings only: **zero audio streams**, no
narration, no music. `dira`'s own output carries the emotional beat
(`.agents/product-marketing.md` §6); anything layered on top of it would be
the fabricated production value this lane's absolutes rule out.
`check-no-audio.mjs` (`E8-L4-T2`) is the mechanical form of that rule, run
against `check.gif` above; `.cast` files are JSON, not media `ffprobe` can
open, so the check applies to the rendered cut, not the raw recording.

## What this lane does not own

The fixture (`fixtures/demo-ledger/`, `E8-L3-T1`), the recording scripts'
absence-guard skeleton (`E8-L3-T3`/`T4`), and the verbatim-binding checker
against `.agents/product-marketing.md` §6 (`check-demo-script.mjs`,
`E8-L3-T6`) are `E8-L3`'s. `E8-L4` filled in `record-init.sh`'s target
repo and verb (both left open by `E8-L3`, resolved above), ran both
recordings for real, and added the probes (`check-cast-duration.mjs`,
`check-no-audio.mjs`, `check-cast-drift.mjs`, `check-init-ledger.mjs`) that
verify them.
