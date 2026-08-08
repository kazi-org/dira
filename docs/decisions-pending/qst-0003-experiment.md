# qst-0003 — measured against kazi's 80 ADRs

**Answer: the corpus is far better than the question feared, and the import is worth
building — but not for the reason the question was worried about, and not in the shape
it proposed.**

qst-0003 predicted the failure mode as *"77 entries with empty `alternatives` arrays."*
That is not what the corpus looks like. **Zero of 218 extracted alternatives are bare
names.** The real constraint is somewhere else entirely, it was not in the question, and
it decides how an importer should be built.

| | measured |
|---|---|
| ADRs (excluding `README.md`) | **80** |
| carrying ≥1 rejected alternative **with a reason** | **63 / 80 (79%)** — 61 under a heading, 2 in prose |
| alternatives extractable | **218** |
| …carrying a real `why_not` | **218 / 218 (100%)** after hand-check |
| …bare name, no reason | **0** |
| `why_not` length | median **23 words** (15 content words) |
| …carrying a `revisit_if` condition | **2 / 218 (0.9%)** |
| ADRs recording no alternative at all | **17 / 80 (21%)** |
| alternatives a lexical matcher can reach | see §4 — this is the finding |

---

## 1. Method, and the two-sided controls

Throwaway scripts in the session scratchpad, not shipped, nothing added to `cmd/dira/`
or `internal/`:

- `extract.py` — finds each ADR's alternatives section, splits it into bullet items,
  splits each item into `option` / `why_not`, and classifies the reason as **bare**
  (0 words), **thin** (1–7), or **reasoned** (≥8).
- `check_sim.py` — a crude content-word-overlap matcher, standing in for `dira check`
  (§4). Explicitly **not** a copy of `internal/enforcer`, which is another lane's live
  work; it is here to measure the corpus, not to grade that implementation.

**A check that fails on everything is indistinguishable from one that works.** So the
extractor was run against a hand-written control corpus with a known answer *before* its
verdict on kazi was believed:

| control | contains | extractor said |
|---|---|---|
| `0001-bare-names` | 5 alternatives, names only (2 of them bolded) | 5 **bare**, 0 reasoned ✓ |
| `0002-thin-reasons` | 3 alternatives, label-length reasons ("too slow") | 3 **thin**, 0 reasoned ✓ |
| `0003-rich-with-revisit` | 3 alternatives, full grounds + revisit conditions | 3 **reasoned**, 3 **revisit** ✓ |
| `0004-no-section` | no alternatives heading | 0 ✓ |

So a heading does **not** count as a hit — `0001` has the section and scores zero — and
the `revisit_if` detector is proven able to find one, which matters because its verdict
on the real corpus is 0.9% and an undetectable-by-construction 0.9% would be worthless.

**Hand-reading found a defect the script did not.** The first version matched `## `
headings only, and missed ADR-0080's `### Considered and rejected: opt-in sealing`.
Widened to any level; re-verified against the controls afterwards. Without the hand pass
this report would have said 60/80 and 214 alternatives.

**Hand-read in full or in relevant part:** 0001, 0003, 0006, 0007, 0022, 0032, 0045,
0049, 0055, 0059, 0064, 0067, 0080 — 13 ADRs, chosen to span the best cases, every
"thin" case, both `revisit_if` hits, and the no-section group.

---

## 2. Where the script over-counted, corrected by hand

**The 3 "bare" items are extractor artifacts, not corpus defects.** All three come from
ADR-0080's amendment section, and none is an alternative:

```
"It keeps `goal_drifted` reachable in production, instead of making the field nearly vestigial"
"It still covers the #1520 incident class in full, because the goal in that incident declares `sealed_inputs`"
"It is strictly backward-compatible for every pre-ADR-0080 goal-file."
```

Those are bullets *arguing for* the chosen option, swept up by the widened heading match.
**Corrected count: 218 real alternatives, 0 bare.**

**The 5 "thin" items are terse but real** — the 8-word threshold is conservative and
undercounts:

- *"**Build a new harness from scratch.** Same competition problem, larger."* (0001)
- *"**Whole-repo lock per agent** — correct but kills parallelism, the entire point."* (0006)
- *"**Rename only one verb.** Half-consistent; do both or neither."* (0032)

Each is a usable `why_not`. So every one of the 218 carries a reason.

**Both `revisit_if` hits are genuine**, checked by hand:

- ADR-0003, on Go: *"chosen as the fallback if OTP is overruled."*
- ADR-0045: *"Opt-in until measured (ADR-0046), then reconsider the default."*

**The no-section group is genuinely thin, with two exceptions.** Of the 17 ADRs with no
alternatives anywhere, most are bug-closure or mechanical ADRs where there was no choice
to record (ADR-0049 closes a broken `approve → apply` seam; there were no options). Two
carry a real rejection in prose that a heading-based extractor misses:

- **ADR-0007** — *"The naive way to build it is horizontally… That order cannot be
  dogfooded: a phase built in isolation has nothing behind it to converge against."*
- **ADR-0055** — *"**Rejected.** Goal-files are reviewable declarative contracts; per-goal
  prose process rules would drift, bloat every proposal, and re-introduce the subjective
  layer kazi exists to remove."*

A regex importer gets 61; a semantic one gets 63. That 2-ADR gap is small here and is
the single strongest argument for option 2 over a mechanical import.

---

## 3. What the corpus is rich in, and what it is missing

**Rich in `why_not`.** Median 23 words, mean 25, and the shape is consistently
*option — grounds*, often naming the competing ADR:

> **Filesystem blackboard (append-only jsonl + per-session cursors + hook injection).**
> Zero infrastructure and workable at low volume, but it fails the stated scale
> requirement structurally: no server-side aggregation means token cost grows linearly
> with message volume; no TTL or last-value semantics without a reaper; delivery depends
> on every client's discipline. Rejected as the end-state; its delivery-via-hooks idea is
> retained (point 6). — ADR-0067

That is a better `why_not` than most entries in dira's own ledger.

**Missing `revisit_if`, near-totally: 2 of 218 (0.9%).** This is the half of qst-0003's
fear that is confirmed, and **import cannot fix it** — no extractor invents a condition
the author never wrote. `design.md` §4.2 calls `revisit_if` the field that *"distinguishes
a closed door from a locked one, and gives the enforcer something better to say than no."*
An imported corpus can refuse; it essentially cannot offer a way forward. Every imported
entry will need its `revisit_if` written by a human later or never.

---

## 4. The decisive test: could `dira check` refuse anything?

`dec-0014` makes conflict detection **lexical and in-binary** — no model, no agent. So a
rich `why_not` is not sufficient on its own: the **`option` text** has to contain terms a
human would reuse when proposing the same thing again, or the entry is unreachable
however well it is written.

Four real relitigations of kazi decisions, plus a negative control, against all 218
extracted alternatives:

| plan proposed | overlap ≥2 (precise) | overlap ≥1 (loose) |
|---|---|---|
| *"coordinate sessions with an append-only jsonl blackboard and per-session cursors"* | **cites ADR-0067** ✓ (5 shared terms) | ✓ |
| *"let the fixer author the pins it is graded against"* | **cites ADR-0064** ✓ (4 shared terms) | ✓ |
| *"rewrite the controller in Go so we ship a single static binary"* | **no citation** ✗ | fires, but top hit is ADR-0035 *"Static tiering only"* — **wrong entry**, 17 hits |
| *"add an llm-as-judge predicate so the agent can certify the goal converged"* | **no citation** ✗ | fires, but top hit is ADR-0071 *"Single aggregate predicate per feature"* — **wrong entry**, 17 hits |
| *"add a dark mode toggle to the dashboard header"* (NEGATIVE CONTROL) | **silent** ✓ | fires 7 times ✗ |

**Two of four, at perfect precision.** Loosening the threshold to catch the other two
turns the matcher into noise — it starts citing the wrong ADR and refusing a dark-mode
toggle.

**The cause is option-name length, and it is the finding of this experiment.** Matchable
content words in the `option` field:

```
      0 words    1  ( 0.5%)
      1 word     4  ( 1.8%)
      2 words   16  ( 7.3%)
    3-4 words   88  (40.4%)
    5-8 words  105  (48.2%)
   9-99 words    4  ( 1.8%)
   median 4.5
```

**21 of 218 (10%) carry ≤2 matchable words** — and they are the famous ones, the ones most
likely to be relitigated: `Go`, `Rust`, `Redis`, `MCP-only.`, `Git refs`,
`LLM-as-judge for completion.`, `JetStream only (no SQLite)`.

The `why_not` beneath `Go` is three rich sentences. The **handle** is one word, which is
also an English verb. No lexical matcher distinguishes *"rewrite it in Go"* from *"go
ahead and ship"* on that surface. The corpus records the reasoning; it does not record it
in a form a lexical enforcer can find.

*(One honest caveat about my own numbers: my first run missed ADR-0003 entirely because
"go" was in my stopword list. Fixed before the table above. The real enforcer may also do
better than a crude overlap — stemming, phrase matching, weighting the option field. But
the structural point survives any matcher: a one-word option name carries one word of
signal.)*

---

## 5. Which of the three options the evidence supports

**Not the leaning in the entry.** qst-0003 leans **(1) index, don't import** plus
**(3) import only what is cited**, and both were reasoned from a premise the measurement
falsifies: that ADRs rarely record rejected alternatives with reasons. 79% of this corpus
does, at 23 words a piece. Downgrading 218 real `why_not`s to read-only links throws away
exactly the content dira exists to hold, and (3)'s "corpus stays mostly dark" cost is
being paid to avoid a problem that is not there.

**The evidence supports (2), semantic import via the skill, with the triage objection
answered by the numbers.** The entry rejects (2) because *"77 staged entries is a triage
queue nobody will finish."* Measured, the queue is not 77:

- **63 ADRs import with real content** — alternatives, `why_not`s, no human needed.
- **17 ADRs have nothing to extract** and are the only ones that need staging or skipping.

A 17-item queue is finishable in an afternoon. And (2) is the only option that gets
ADR-0007 and ADR-0055, whose rejections are in prose rather than under a heading.

**Two things the import must do that no option in the entry mentions**, both from §4:

1. **Expand short option names at extraction time.** The binding constraint on
   enforceability is a 1–2 word `option` label, not `why_not` quality. The extracting
   session has the surrounding ADR in context and can write `Go — a single static binary,
   NATS-native, no BEAM runtime` instead of `Go`. This is the difference between an entry
   that refuses and an entry that is merely readable, and it is free at import time and
   expensive later.
2. **Import `revisit_if` as absent, and say so.** 0.9% coverage means imported entries
   refuse without offering a way forward. That should be visible in the import summary,
   not discovered the first time `dira check` fires.

---

## 6. What would change this answer

- **A second corpus.** This is **n=1**, and the sample is not neutral: kazi's ADRs were
  written by the same person building dira, which is the best possible case for an
  importer and the worst possible evidence for generalising. 54 of 80 use one exact
  heading string; a repo whose ADRs are inconsistent, tabular, or discursive could
  produce a completely different number. **The pitch in the entry is "any repo with
  history", and this experiment does not support that claim — it supports it for one
  disciplined corpus.** Measuring 2–3 unrelated public ADR corpora is the cheapest thing
  that would move this, and I would not ship the import pitch without it.
- **A real run of `dira check`** against imported entries, once E3's enforcer lands.
  §4 uses a proxy; if the real matcher does materially better on short option names, the
  "expand the option text" requirement weakens.
- **Evidence that a live session extracts as accurately as the regex did.** I measured
  what is *in* the corpus, not whether a model reliably pulls it out. A semantic importer
  that hallucinates a `why_not` is worse than no import at all, and nothing here tests
  that.
- **The `revisit_if` gap turning out to matter more than assumed.** If `dira check`
  without `revisit_if` reads as a wall rather than a compass, an import that cannot supply
  it may not be worth having at scale.

---

## 7. Files

Everything is in the session scratchpad and nothing was added to the repo:

```
scratchpad/extract.py       the measurement
scratchpad/check_sim.py     the dira check proxy
scratchpad/dump.py          per-item extraction dump, for hand-checking
scratchpad/control/         four hand-written ADRs with known answers
```

Nothing under `cmd/dira/` or `internal/` was touched, nothing was committed, and
`docs/roadmap.md` / `docs/coverage.md` were not edited.
