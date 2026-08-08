# E2-L1 — `dira sniff`, the regex tier that may only ever stage

**Lane:** E2-L1 (`docs/plan/lanes/E2.md`) · **Status:** complete, unwired, uncommitted
**Date:** 2026-07-30

---

## 1. The wiring to add

`cmd/dira/main.go` is the integrator's file and I did not touch it. One line goes into
`newApp`'s `commands` slice, between `log` and `check`:

```go
{name: "sniff", summary: sniffSummary, run: runSniff, usage: writeSniffUsage},
```

That exact line is executed by `cmd/dira/sniff_test.go:newSniffRunner`, which appends it to
a real `newApp` registry and drives the command through `app.main`. It is not a line
described in prose here and never run.

Nothing else changes in `main.go`. `sniffSummary`, `runSniff` and `writeSniffUsage` all live
in `cmd/dira/sniff.go`.

---

## 2. What was built

**`internal/sniff`** — six source files, stdlib only.

| file | what it holds |
|---|---|
| `sniff.go` | package contract, `Candidate`, `Sniff`, `SniffTranscript`, the per-run bound |
| `transcript.go` | Claude Code JSONL parsing, `Scope` (LastTurn / Whole), plain-text fallback |
| `text.go` | fence/blockquote stripping, sentence splitting, title and excerpt bounding |
| `match.go` | the matcher: strong rules, commitment+contrast rules, 11 guard families |
| `redact.go` | what sniff refuses to capture, and why it drops rather than masks |
| `stage.go` | the write path and the `stagedOnly` store that makes acceptance inexpressible |

**`cmd/dira/sniff.go`** — the command. Reads the Claude Code hook payload from stdin
(`session_id`, `transcript_path`), falls back to `--transcript FILE`, falls back to reading
stdin as prose. `--stage` writes; without it the run is a dry listing. `--quiet` is the
Stop hook's flag. `--all` widens the scope from the last turn to the whole transcript, which
is what PreCompact wants.

**Fixtures** — `internal/sniff/testdata/transcripts/` holds five recorded transcripts in
Claude Code's real record shape: `stop-design-direction.jsonl`, `stop-storage-interface.jsonl`,
`pre-compact.jsonl` (the path E2-L2's acceptance line names), `no-decisions.jsonl` (the
negative fixture), `with-credentials.jsonl`.

**The corpus** — `internal/sniff/testdata/corpus.yaml`, 79 labelled rows drawn from a real
session, frozen by `corpus.sha256`.

---

## 3. The measured false-positive rate

### 3.1 The corpus and its digest

```
sha256(internal/sniff/testdata/corpus.yaml) = 440bc5d66bfa48405d9e7767c50e2fe809ae50984228b5cf317f42552fc5dbf5
```

79 rows: **25 `decision`**, **54 `none`**, of which **50 carry a `near_miss` label across 17
families** (citation, description, instruction, let-me, modal, deferral, commitment,
recommendation, past-action, commitment+contrast, conditional, question, option,
second-person, code, hypothetical, tool-output).

**Provenance.** Every row is drawn from one real Claude Code session — the session that built
this repository: 3,210 transcript records, 425 prose segments, 328KB of message text — and
then redacted (absolute paths, usernames, personal names, and anything from a private
instruction file removed or generalised). The labelling rule is stated at the top of the
corpus file, before the labels, so it can be argued with.

**Freeze honesty, stated because it matters.** The corpus was written and hashed before
`internal/sniff/match.go` existed. The digest was regenerated **once**, at
`2f2227b947b15fcbbe04a53a6ca6e7fad0f1365e49e231d54b5f3859fa578586` → the value above, before
any grading run was possible: ten `note:` values began with a double quote or contained
`": "`, which made the file unparseable YAML. Only `note:` lines changed; no row `id`, `text`,
`expect` or `near_miss` was touched. After that point nothing in the file changed, and
`assertCorpusFrozen` runs first in every grading test.

### 3.2 The observed numbers

Bars were written into `corpus_test.go` before the matcher was graded even once:
false positives ≤ 5%, detection ≥ 60%.

```
$ go test -count=1 ./internal/sniff/ -v
--- PASS: TestMatchesTheCorpus (0.02s)
    corpus_test.go:153: MISS  p16  "The page ships with two pillars instead of three, because the third asserts a capability that is still an open question."
    corpus_test.go:153: MISS  p18  "The lane keeps the frozen corpus and drops the second fixture ledger instead of maintaining both."
    corpus_test.go:161: FALSE POSITIVE  n09  [commit-first-person+contrast-rather]  "Two things run in parallel: I'll recon the surfaces so the contract is concrete rather than inferred, while I draft the docs."
    corpus_test.go:161: FALSE POSITIVE  n10  [commit-first-person+contrast-rather]  "I'll verify from a cleared cache rather than assume it is benign twice."
    corpus_test.go:172: OBSERVED  detection 92.0% (23/25)  ·  false positives 3.7% (2/54)  ·  bars: detection ≥60%, false positives ≤5%
```

**False-positive rate: 3.7% (2 of 54).** Precision on the corpus is 23/25 = **92%**.

Both false positives are the same structural trap and the corpus says so in their notes: a
first-person commitment and a contrast, where the contrast qualifies a *noun*
("concrete rather than inferred") rather than the *course of action*. A regular expression
cannot see which phrase the contrast attaches to. Both misses are the same deliberate
exclusion: a third-person commitment ("The page ships with…"), which was left out because
including it fires on ordinary description ("The script hardcodes three windows rather than
reading them from configuration") — four rows of the corpus, and far more in real text.

### 3.3 Out of sample — the number that decides whether this is usable

The corpus was both the design input and the grading set, so a corpus-only result is
optimistic by construction. I ran the finished matcher over four **real, uncommitted** Claude
Code transcripts on this machine, in whole-transcript scope:

| session | prose segments | sentences | candidates | rate |
|---|---:|---:|---:|---:|
| the session that built dira | 425 | 4,001 | **22** | 0.55% |
| a large product session (32MB) | 941 | 13,189 | **83** | 0.63% |
| a large product session (31MB) | 1,319 | 10,521 | **46** | 0.44% |
| a shorter session (26MB) | 264 | 2,713 | **4** | 0.15% |

I hand-labelled all 22 from the dira session against the corpus's stated rule: **16 are
decisions worth a ledger entry, 6 are not** — out-of-sample precision **≈73%**. The six are
all the same shape as n09/n10: `I'll verify from a cleared cache rather than assume`,
`I'm noting that rather than claiming it works`, `I'll say so in the brief rather than let
them assume`. State this plainly: **roughly a quarter of what a real session stages is not
worth keeping**, and the human's disposition keystroke is what that costs. That is the
honest cost of the tier, and it is the number to re-measure when E2-L4 ships.

The rate that governs the Stop hook is per-turn, not per-session: **0.09 candidates per prose
segment** on the dira session, so the overwhelming majority of turns stage nothing at all.
The 83-candidate session is a whole-transcript figure and is bounded to 12 by
`maxCandidates` on a real PreCompact run, with the overflow reported rather than swallowed.

### 3.4 Two bugs the out-of-sample run found

Both were real, both are fixed, and neither would have been caught by the corpus:

1. **`we'?re` matches the word "were".** Three sentences of ordinary past-tense narration
   were staged as decisions. The commitment patterns now require the apostrophe and list
   both the typewriter and typographic forms.
2. **The `Decision:` marker fired on `DECIDED — and on whose authority`**, a heading in a
   report template quoted into the session, twice. The marker now requires a colon.

Candidates on the dira session fell from 28 to 22 as a result.

---

## 4. Every acceptance criterion, with evidence

The lane's `acc:` line, clause by clause. All output is real.

> `go test ./internal/sniff` passes against `testdata/transcripts/` holding ≥3 recorded
> Claude Code transcripts

```
$ ls internal/sniff/testdata/transcripts
no-decisions.jsonl  pre-compact.jsonl  stop-design-direction.jsonl
stop-storage-interface.jsonl  with-credentials.jsonl

$ go test -count=1 ./internal/sniff/
ok  	github.com/kazi-org/dira/internal/sniff	0.573s
```

`recordedTranscripts` fails the suite below three files, and `transcriptPath` fails on an
empty one.

> asserting on the entries actually written to a temp ledger: every entry has `state: staged`,
> `source.tier: regex`, `source.hook: Stop`, and a non-empty bounded `source.excerpt`

`TestStagedEntriesAreTheOnlyThingWritten` writes to a temp `.dira` through the real
`local.Store`, globs the files off disk, decodes each through `ledger.Decode`, and asserts
per entry. It also asserts `len(files) == len(result.Staged)` — one file per entry, dec-0002.

```
--- PASS: TestStagedEntriesAreTheOnlyThingWritten (0.03s)
    --- PASS: TestStagedEntriesAreTheOnlyThingWritten/stop-design-direction.jsonl
    --- PASS: TestStagedEntriesAreTheOnlyThingWritten/stop-storage-interface.jsonl
    --- PASS: TestStagedEntriesAreTheOnlyThingWritten/pre-compact.jsonl
    stage_test.go:153: OBSERVED  5 entries staged across 3 recorded transcripts, every one staged/regex/Stop
```

A file it wrote, verbatim:

```
--- dec-0001.md ---
---
id: dec-0001
kind: decision
title: I'll go with the chain-and-serif blend rather than the third anchor, because the struck-through refusals are the…
state: staged
created: "2026-07-30T09:15:00Z"
source:
  hook: Stop
  session: 1f0c6a3e-0000-4000-8000-000000000001
  excerpt: >
    I'll go with the chain-and-serif blend rather than the third anchor, because
    the struck-through refusals are the strongest device in any of them and the
    third one loses them.
  tier: regex
---
```

No body, no alternatives, no `confirmed_by`. The "because" is exactly what tier 1 cannot
know, so the field is absent rather than guessed at.

> **zero** entries are in `accepted`, `active`, or `open`; **zero** entries carry a non-empty
> `alternatives[]` or any `why_not`

Asserted on the decoded files in the same test — and, more to the point, made structurally
impossible; see §5.

> every written file validates against `schema/entry.schema.json`

The same test compiles `schema.NewValidator()` and validates every file's bytes. That is the
published contract, not the Go validator.

> a fixture with no decision language writes no file and, under `--quiet`, prints nothing and
> exits 0

Two tests, at two levels.

```
--- PASS: TestTheNegativeFixtureWritesNothing (0.00s)          # internal/sniff
--- PASS: TestQuietFindsNothingAndSaysNothing (0.01s)          # cmd/dira, through app.main
```

`TestQuietFindsNothingAndSaysNothing` runs the real command registry over
`no-decisions.jsonl` and asserts exit code `0`, `stdout == ""`, `stderr == ""`, and zero files
in the ledger. `TestQuietStillAnnouncesWhatItStaged` is its control: on a transcript that does
carry decisions, `--quiet` still prints `dira: staged dec-0001 …`. Without that control a
`--quiet` that suppressed everything unconditionally would pass.

> **risk:** the regex fires on hypotheticals … The negative fixture … must contain deliberate
> near-misses, not merely unrelated text.

`no-decisions.jsonl` contains, by line: "we could go with", "suppose we don't do", "one option
is", a direct question, two citations of real ledger entries, a "let me", "this is the
phase-one gate — your call", "needs a call", "we will decide later", and a block of tool
output. It yields **0 candidates**. The corpus carries 50 more near-misses across 17 families.

---

## 5. "May only ever stage", made structural

The brief asked for a code path that cannot express acceptance rather than a test asserting it
never happens. Three layers, in increasing order of how hard they are to defeat:

1. **`Candidate` has no `State`, `ConfirmedBy`, `Alternatives` or `ADR` field.** No caller of
   this package can express a confirmed entry.
2. **`Stage` builds every `Entry` itself, from constants.** There is no parameter through
   which a state or a tier arrives.
3. **`Stage` wraps the caller's `Store` in a `stagedOnly` before touching it, and that
   wrapper is the only `Store` any code in `stage.go` holds.** `Create` refuses any entry that
   is not staged, decision-kind, regex-tier, from a capture hook, with a bounded non-empty
   excerpt, with no `confirmed_by`, no alternatives, no edges, no `adr`, and not `private`.
   `Put` and `Delete` are refused unconditionally.

Layer 3 is the one that survives somebody editing layers 1 and 2. It is graded red→green
against 16 deliberately illegitimate entries, **each of which `Entry.Validate` accepts** — the
test asserts that first, so the wrapper cannot quietly degrade into a second copy of the
schema validator. The distinction it holds is that a legitimate accepted decision and an
accepted decision a regex wrote are indistinguishable to the schema.

```
--- PASS: TestTheStoreRefusesEverythingButStaging (0.00s)
    --- PASS: .../a_staged_regex_entry_is_accepted     <- the control; without it every refusal below is vacuous
    --- PASS: .../accepted            .../rejected           .../superseded
    --- PASS: .../an_open_question    .../an_active_intent
    --- PASS: .../a_human_tier        .../a_semantic_tier    .../no_source_at_all
    --- PASS: .../an_import_hook      .../no_excerpt         .../an_unbounded_excerpt
    --- PASS: .../confirmed_by_a_human    .../confirmed_by_an_agent
    --- PASS: .../an_invented_alternative .../an_inferred_edge
    --- PASS: .../a_mirrored_adr      .../marked_private
--- PASS: TestTheStoreRefusesReplacementAndDeletion (0.00s)
--- PASS: TestStageRefusesAHookThatIsNotACapturePoint (0.01s)
--- PASS: TestTheSourceCannotExpressAConfirmedEntry (0.02s)
```

`TestTheSourceCannotExpressAConfirmedEntry` is the weakest of the four and is labelled as
such: it walks the package's non-test ASTs for `ledger.StateAccepted`, `TierHuman` and five
other names. It carries its own negative control — the same walk over the test file must
*find* `StateAccepted`, or the walk is looking in the wrong place.

---

## 6. The two schema conflicts, resolved

The lane named both as its first task.

### 6.1 `state: staged` is valid only on `kind: decision`

**Resolved by scope: `dira sniff` stages decisions and nothing else.** Enforced in
`mustStage`, which refuses a question or an intent by kind before it looks at state.

The alternative was to widen the schema so `staged` is legal for `question`. I rejected it,
and the reasoning is worth arguing with rather than accepting:

- dec-0003's own ledger entry scopes tier 1 to decision language, twice, and names the
  phrases ("let's go with", "we're not doing"). Nothing in it claims the tier can find a
  question.
- Widening a kind's state enum changes `Kind.States()`, `Entry.Validate`, `dira check`'s
  enforcement table, the index schema, and every downstream lane's assumption about what a
  question can be. That is a cross-epic change on an L1 lane's own authority.
- Precision. Question-shaped language is far more common in a session than decision language,
  and this lane's whole deliverable is a measured false-positive rate.

**The contradiction this leaves, flagged rather than absorbed:**
`docs/design/screens/s3-distill.html` line 180 shows `qst-0006` in the distill queue with
`chip-open ◆ open question` and the source line `Stop · 16:41 · regex`. That is precisely the
thing the lane calls dishonest — a regex-inferred question written straight to `open`, so the
ledger asserts an open question no human ever saw. With this resolution the sniffer will never
produce that card. **Either the screen needs its third card changed, or somebody with the
authority to widen the schema needs to decide that questions get a staged state.** It is
E2-L4's surface, so I have not touched it; the decision is above both of us.

### 6.2 `kind: decision` requires `alternatives`, and a regex may not assert a `why_not`

**Resolved by making the requirement conditional, exactly as the lane proposed:** `minItems: 1`
on `alternatives` for every decision **except** a staged one, which is not required to carry
the field at all.

`schema/entry.schema.json` — the decision rule now nests an if/then/else on `state: staged`.
`internal/ledger/entry.go` — `Validate` gains `&& e.State != StateStaged`, with the reasoning
in the comment.

**This also fixed a real, pre-existing, untested disagreement.** Before this change the schema
said `required: ["alternatives"]` with no `minItems`, so `alternatives: []` **validated**
against the published contract while `Entry.Validate` rejected the same file. A decision could
be written that dira's own contract accepted and dira's binary would not read back. Both
directions are now pinned:

- `schema/testdata/invalid/decision-with-empty-alternatives.md` — accepted decision,
  `alternatives: []`. Rejected by both engines (`minItems`).
- `schema/testdata/valid/staged-decision.md` — staged decision, no alternatives, no body.
  Accepted by both.
- `internal/ledger/schema_test.go:assertAlternativesRule` reads the nested rule out of the
  schema document and asserts the Go validator draws the line in the same place, in both
  directions. The previous version of that check keyed on `rule.Then.Required`; had I left it
  alone it would have silently stopped asserting anything once the requirement moved into the
  nested `else`. It now fails loudly if the exemption is removed or relocated.

**A note for E2-L2 and E2-L4:** the schema *permits* a staged decision to carry alternatives —
it is only the `minItems` floor that is lifted. That is deliberate, so the semantic tier can
fill in a `why_not` on an entry that is still staged. What forbids alternatives is
`mustStage`, which is this tier's rule, not the schema's.

---

## 7. What sniff refuses to capture, and why

Decisions I made, stated so they can be overruled:

**Refused outright**

- **Tool input and tool output.** `tool_use` and `tool_result` blocks never reach the matcher.
  This is a precision rule and a privacy rule at once: a `Bash` result is where an `env` dump,
  a config file, a customer record or a token actually appears in a transcript, and none of it
  is anybody's decision.
- **`thinking` blocks.** Reasoning the session did not say out loud is the densest source of
  exactly the hypotheticals that must not stage.
- **Fenced code and markdown blockquotes.** Code carries the matcher's keywords and none of
  their meaning; a blockquote is somebody else's sentence.
- **Any sentence carrying something shaped like a credential** — nine patterns
  (`sk-`, `ghp_`/`gho_`/`ghs_`, `github_pat_`, `xox[baprs]-`, `AKIA…`, `AIza…`, armoured
  private keys, `api_key|secret|token|password: <12+ chars>`, `Authorization: Bearer …`).

**Dropped, never masked.** `source.excerpt` exists so a human can audit what a hook inferred.
An excerpt reading `the key is [redacted], so we're going with X` is evidence with a hole in
it: the reviewer cannot tell whether the removed span changed the meaning, and cannot recover
it. A dropped candidate loses at most one decision, which the human can still type by hand and
the semantic tier may still catch. `TestCredentialsAreRefusedWholesale` asserts no captured
text contains a secret **or** a mask marker, and carries two controls — the same sentence with
the credential stripped does match, and something *was* captured from the fixture, so the
assertions are not passing on an empty list.

**Deliberately NOT refused, and why**

- **File paths, hostnames and personal names.** The matcher cannot tell a customer's name from
  a library's, and one that guessed would drop real decisions while still missing real leaks.
  The boundary that binds is cst-0003's, enforced at the export edge, plus
  `scripts/privacy-lint.py` over what is committed. Stating this as a limit rather than
  implying coverage.
- **`private: true` is never set.** Whether an entry is private is a property of the ledger's
  tier and a human's judgement; a regex asserting it is the same category error as a regex
  asserting `accepted`. `mustStage` refuses a private entry outright.
- **No parent ledger is ever written.** `Stage` holds exactly the store it was given, which
  `openLedger` resolves from the working directory. cst-0003 rule 1, by construction.

---

## 8. One product problem the acceptance line does not name

**The Stop hook fires after every turn against a transcript that only grows.** A sniffer with
no memory re-proposes turn one's decision on turns two, three and four, and the distill queue
fills with copies of one sentence — the same noise failure the lane's risk line describes,
arriving by a route the corpus cannot see. Two mechanisms:

1. **Scope.** The default is the last turn — what has been said since the human last spoke.
   `--all` is the PreCompact scope. `TestLastTurnIsTheDefaultScope` asserts the narrow scope is
   actually narrower.
2. **Ledger dedupe.** `Stage` reads the existing `dec-` entries and skips any candidate whose
   normalised title already exists. `TestTheSameTurnStagesOnce` runs the same candidates twice
   and asserts the second run writes nothing and reports every one as a duplicate.

The cost is one ledger read per `--stage` invocation. Only decision entries are read, and E1
measured a 200-entry full read at ~28ms against int-0002's sub-100ms budget, so this fits —
but it is the part of `dira sniff` that scales with ledger size, and E1-L6's budget work should
know it exists.

---

## 9. Gate results

Run from a clean build, `-count=1` throughout.

| gate | result |
|---|---|
| `gofmt -l .` | clean |
| `go vet ./...` | clean |
| `golangci-lint run` over `internal/sniff`, `cmd/dira`, `internal/ledger`, `schema` | **0 issues** |
| `go test -count=1 ./internal/sniff/` | ok |
| `go test -count=1 ./cmd/dira/` | ok |
| `go test -count=1 ./schema/` | ok |
| `python3 scripts/coverage.py` | **PASS** — 70 obligations, 70 registered, 0 uncovered |
| `python3 scripts/privacy-lint.py` | **PASS** — 4 checks, cst-0003 enforced |

**Three failures in the full-repo run that are not mine, reported rather than hidden.** All
come from untracked files other lanes landed while this one was running. `cmd/dira` and
`internal/ledger` were green at my baseline run and green again when only this lane's tests
are selected (`go test ./cmd/dira/ -run 'Sniff|Quiet|…'` → `ok 0.446s`).

```
--- FAIL: TestNoFilesystemImportsAboveTheBackend (internal/ledger)
    cmd/dira imports "path/filepath"      <- cmd/dira/ui.go (untracked, not this lane)
    internal/ui imports "io/fs"           <- internal/ui/ (untracked, not this lane)

golangci-lint: internal/brief/render.go:89:5: ineffectual assignment to filling
                                              <- internal/brief/ (untracked, not this lane)

--- FAIL: TestWhyOnTheRealLedgerMatchesItsGoldenFile/{daemon,int-0002,dec-0012}
    <- .dira/entries/dec-0019.md and dec-0020.md landed untracked mid-run and
       changed the real ledger's why-chains. This lane adds no ledger entry:
       `git diff --stat .dira` is empty for it.
```

Evidence they are not mine: `git status --porcelain cmd/dira/ui.go internal/ui internal/brief`
reports all three as `??` (untracked, i.e. created after my baseline run, in which all gates
were green), and `grep -l "path/filepath" cmd/dira/*.go | grep -v _test` returns only
`cmd/dira/ui.go`. `cmd/dira/sniff.go` imports `os` only, which is on the boundary test's
allowlist for `cmd/dira`. The boundary test reads `go list .Imports`, not `.TestImports`, so
`sniff_test.go`'s `path/filepath` does not contribute.

`internal/ledger/fixture`'s `TestFullLedgerReadIsWithinBudget` failed once at 168.9ms against a
150ms budget while many agents were compiling on this machine, and passed on its own
(`ok … 0.904s`) immediately afterwards. Load artefact, not a regression — but it is a
latency gate with no headroom, and worth someone's attention.

---

## 10. Contradictions and gaps found

1. **`docs/design/screens/s3-distill.html` shows a regex-sourced `qst-0006` as `open` in the
   distill queue.** With this lane's resolution the sniffer cannot produce that card. Needs a
   decision above E2-L1: change the screen, or widen the schema so questions can be staged.
   §6.1.
2. **`hooks/settings.example.json` writes `dira sniff --deep --stage` for PreCompact, and
   `--deep` does not exist.** It is E2-L2's handoff and I was told not to build it. Until
   E2-L2 lands, that command string exits 2 — which means **E2-L3's acceptance clause (f)**
   ("every command string the installer writes is accepted by the built binary") **cannot pass
   until E2-L2 ships.** E2-L3 depends on E2-L1 in the lane table, not on E2-L2; that
   dependency edge is wrong, or the installer must not write `--deep` yet.
3. **The schema and the Go validator disagreed about `alternatives: []` and nothing tested
   it.** Fixed, with fixtures in both directions. §6.2.
4. **`internal/ledger/schema_test.go`'s alternatives assertion would have silently stopped
   asserting** when the requirement moved into a nested rule — it keyed on `Then.Required`
   with no failure path if the key was absent. Rewritten to fail loudly. §6.2.
5. **The phrases dec-0003 names as tier 1's bread and butter do not occur in real sessions.**
   Over 328KB of real Claude Code text: "let's go with" 0, "we're not doing" 0, "going with" 0
   — against "I'll <verb>" 41 and "rather than" 160. The entry is not wrong about what a regex
   *can* catch, but a matcher built from its examples would catch nothing. Worth a note on
   dec-0003 rather than a supersession.
6. **A ledger entry recording this lane's schema resolution has not been written.** An
   `accepted` decision entry creates an `impl:dec-XXXX` obligation in `scripts/coverage.py`
   that must be registered in `docs/coverage.md`, and I was told not to edit that file. The
   entry and its register row are the integrator's to add; the reasoning is §6 of this
   document, ready to be lifted.

---

## 11. What I did not do

- Did not touch `cmd/dira/main.go`, `docs/roadmap.md` or `docs/coverage.md`.
- Did not commit or push. Everything is working-tree only.
- Did not build `--deep`, any handoff block, any skill, or any model client (E2-L2).
- Did not build `install-hooks` or touch `hooks/` (E2-L3).
- Did not build any disposition flow (E2-L4).
- Did not build the ADR mirror writer (`dec-0009`), the unowned scope item E2's lane file flags
  upward. Nothing in this lane's acceptance names it, and absorbing it would expand an epic's
  scope on an L1 lane's authority.

## 12. Files

**Added**

```
internal/sniff/sniff.go              internal/sniff/corpus_test.go
internal/sniff/transcript.go         internal/sniff/fixture_test.go
internal/sniff/text.go               internal/sniff/guard_test.go
internal/sniff/match.go              internal/sniff/redact_test.go
internal/sniff/redact.go             internal/sniff/stage_test.go
internal/sniff/stage.go              internal/sniff/transcript_test.go
internal/sniff/testdata/corpus.yaml
internal/sniff/testdata/corpus.sha256
internal/sniff/testdata/transcripts/{stop-design-direction,stop-storage-interface,pre-compact,no-decisions,with-credentials}.jsonl
cmd/dira/sniff.go                    cmd/dira/sniff_test.go
schema/testdata/invalid/decision-with-empty-alternatives.md
schema/testdata/valid/staged-decision.md
```

**Modified**

```
schema/entry.schema.json          the decision rule's conditional on state: staged
internal/ledger/entry.go          Validate exempts a staged decision from alternatives
internal/ledger/schema_test.go    assertAlternativesRule, and the nested-rule decoder
schema/entry_test.go              one case row for the new invalid fixture
```
