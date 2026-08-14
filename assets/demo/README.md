# Demo clips (E8-L3)

Two clips back `.agents/product-marketing.md` §6, "The demo (the single most
important marketing asset)": a primary clip showing `dira check` catching a
plan that contradicts a past decision, and a secondary clip showing `dira
init` deliver value in the first 60 seconds of adoption. Neither has been
recorded — **no `dira` binary exists yet** (see `docs/plan.md`); both scripts
below take their absence branch on every run today and write no `.cast` file.
This file states the reproduction contract and the numeric bars `E8-L4`'s
probes will measure, so the bar is written down once rather than invented
twice.

## Status, today

Honestly: neither `check.cast` nor `init.cast` exists in this repo. `dira`
does not exist as a binary, so there is nothing to record. Running either
script below fails loudly at a `command -v dira` guard and produces no cast —
that is the correct, current behavior, not a bug to work around. `E8-L4`
records the real clips once `dira check` and `dira init` actually run.

## The two clips

### Primary — `dira check`, via `record-check.sh`

Reproduces the exact transcript quoted in `.agents/product-marketing.md` §6:
`dira check` is pointed at a plan that conflicts with `dec-0060` (from
`fixtures/demo-ledger/`, a byte-identical copy of `internal/enforcer`'s
`dec-0060`/`int-0002` fixture — `E8-L3-T1`), and prints the conflict, the
rejected alternative, its `why_not`, and its `revisit_if`.

```sh
DIRA=dira ./assets/demo/record-check.sh
asciinema rec assets/demo/check.cast \
  --output-format asciicast-v2 --overwrite \
  --window-size 92x14 -c "./assets/demo/record-check.sh"
```

**Bar:** under **20.0s** (20 seconds) — one command, one screen, no setup and
no narration. This is deliberately the single most compressed clip in the
product: "it caught me" has to land in one frame.

### Secondary — `dira init`, via `record-init.sh`

Reproduces `dira init` on a real repo with existing history, rendering the
decision graph and flagging contradictions already present in it — the "value
in 60 seconds" clip. Which repo, and whether the resolving verb ends up being
`dira init` or a separate `dira import <dir>` (`dec-0028`), is not decided by
this lane; `record-init.sh` builds only the absence guard that holds
regardless of that later choice.

```sh
DIRA=dira ./assets/demo/record-init.sh
asciinema rec assets/demo/init.cast \
  --output-format asciicast-v2 --overwrite \
  --window-size 92x20 -c "./assets/demo/record-init.sh"
```

**Bar:** under **60.0s** (60 seconds) — the clip has to make "worth installing
today, not in three months" felt inside a single minute.

## No audio, either clip

Both `.cast` files are terminal recordings only: **zero audio streams**, no
narration, no music. `dira check`'s output alone carries the emotional beat
(`.agents/product-marketing.md` §6); anything layered on top of it would be
the fabricated production value `docs/plan/tasks/E8-L3.md`'s absolutes rule
out.

## What is NOT this lane's job

Actually recording the clips — that needs a real `dira` binary and is
`E8-L4`'s work, gated on E0–E3 shipping. This lane owns the fixture the demo
reads, the two recording harnesses (fail loudly, no fabricated cast, per the
lane's absolutes), and `check-demo-script.mjs`, which keeps the harness and
`.agents/product-marketing.md` §6 from drifting apart. No clip is fabricated
here to stand in for a real one — an imitation of `dira check`'s output would
misrepresent working software that does not yet work.
