# E1-L5 — `dira brief`, the in-binary token cap, and SessionStart injection

**Lane:** E1-L5. **Status:** complete, unmerged, uncommitted. All gates green on
this machine (`go build`, `go vet`, `golangci-lint`, `go test ./...`,
`scripts/coverage.py`, `scripts/privacy-lint.py`).

Two deliverables: `dira brief` with cst-0001's ceiling enforced in the binary,
and the `Resolve` widening in `internal/index/query.go`.

---

## 1. The wiring the integrator must add

One line in `cmd/dira/main.go`'s registry, after `why`:

```go
{name: "brief", summary: briefSummary, run: runBrief, usage: writeBriefUsage},
```

Nothing else. `briefSummary`, `runBrief` and `writeBriefUsage` are all in
`cmd/dira/brief.go`. `cmd/dira/brief_test.go` appends the same command when it is
absent from the registry, so the tests pass before and after that line lands, and
they exercise `a.main` either way.

One register edit: `docs/coverage.md` line 156 registers `impl:dec-0020` as
`blocked:E1` with the note "in flight as I register". It is no longer in flight —
flip it to `done`. The three `trigger:dec-0020:*` rows are correct as they stand.

---

## 2. What was built

| path | what |
|---|---|
| `internal/brief/brief.go` | selection, priority order, the note window, `Options`/`Result` |
| `internal/brief/render.go` | the fill, the two forms, `--chain`'s degradation, the omission notice |
| `internal/brief/tokens.go` | `Tokens` — the counter cst-0001 is enforced with |
| `internal/config/config.go` | reads `[brief] max_tokens`, `[ledger] name/tier`, `[parents]` |
| `internal/render/painter.go` | the column layout, extracted from `internal/why` so both renderers share it |
| `cmd/dira/brief.go` | the command, its flags, its help, its fail-open wiring |
| `internal/ledger/local/local.go` | `+ ReadConfig(diraDir)` — the config file is a path, and paths are the backend's (dec-0005) |
| `internal/index/query.go` | `Resolve`: all-tokens matching with a contiguous tier |
| `.dira/entries/dec-0020.md` | the token-counter decision, with its three rejected alternatives |

Tests: `internal/brief/brief_test.go`, `internal/brief/tokens_test.go`,
`internal/config/config_test.go`, `internal/render/painter_test.go`,
`internal/ledger/local/config_test.go`, `internal/index/resolve_test.go`,
`cmd/dira/brief_test.go`.

### The renderer was reused, not reimplemented

`internal/why/render.go`'s `painter` — the wrapping, the hanging title column,
the right margin that stacks when the terminal is too narrow — moved to
`internal/render` unchanged. `why`'s `painter` is now `struct{ *render.Painter }`
plus the chain's own box-drawing grammar, which stayed in `why` because nothing
else draws a tree.

**Observed proof the extraction changed nothing:** the `dira` binary built from
HEAD and the binary built from this tree were run over all 32 entries of the
ledger at five widths each — 160 renders — and compared:

```
compared 160 renders, 0 differ
```

---

## 3. Acceptance criteria, with observed evidence

> `dira brief --context` over the 200-entry fixture emits ≤1500 tokens by the
> binary's own counter

Observed, on the shared fixture (`fixture.Seed`, `fixture.Size`), human form:

```
--- stderr: dira: 1452 tokens of 1500; 77 entries omitted
```

1452 ≤ 1500, and it is *filling* the ceiling rather than honouring it by printing
nothing — `TestTheBriefStaysUnderTheDefaultCeiling` fails below half the ceiling
for exactly that reason. The `--context` and `--context --chain` forms are under
it too (same test, three sub-cases).

On this repository's own ledger (33 entries), nothing is omitted: 3,637 bytes /
**1,271 dira tokens** human, 3,998 bytes / **1,401** for `--context --chain`.

> with the ceiling read from `brief.max_tokens` in `.dira/config.toml` and a test
> proving a lowered ceiling is honored

`TestALoweredCeilingIsHonoured` writes a real `config.toml` and checks the
rendered token count at 1200, 800, 400, 250, 120 — and checks that
`[ledger] name` reaches the heading, so the file is provably being read rather
than a default being reproduced. `TestTheCeilingIsHonouredAtEveryCeiling` sweeps
every ceiling from 1 to 400 in steps of 7. There is **no flag** that raises the
cap; see §6.

> against a fixture engineered to overflow, the omitted content is whole entries
> dropped from the low-priority end (recent decisions dropped first, then current
> focus, open blockers last to go)

The 200-entry fixture overflows the default ceiling by itself (77 of 102 eligible
entries dropped at 1,500), so no second fixture was needed — which is what the
lane asked for ("derived from E1-L1's generator rather than a second generator").

`TestOverflowDropsWholeEntriesFromTheLowPriorityEnd` walks ceilings 1500 → 250
and asserts: what survives is a *prefix* of the keep order; a lower ceiling never
keeps an entry a higher one dropped; and the lowest ceiling really does bite the
top section, so the order is under test rather than trivially satisfied.

Observed at ceiling 250 (human form), the whole brief:

```
dira brief — this ledger · 2026-07-31

open blockers
  qst-0029  Who owns the config loader before the brief still    open 2026-01-31
            fits on one screen
              blocks dec-0079 — Standardise on the derived cache over the drift
              check
  qst-0028  Whether anyone needs the storage backend before a    open 2026-01-15
            reviewer can read the diff
              blocks dec-0032 — Prefer the drift check over the token counter

omitted  14 open blockers, 16 current focus and 70 recent decisions — over the
         250-token ceiling (cst-0001)
         the oldest of each go first; `dira why <id>` prints any entry in full
```

> every rendered entry block is structurally complete (no entry is cut
> mid-render)

`incompleteEntries` in `cmd/dira/brief_test.go` splits the rendered brief into
one block per entry, strips the right-margin status column (which sits inside a
wrapped title and otherwise looks like the title changing halfway through), and
checks each block against the **entry file**: the whole title, and every `blocks`
edge target the file declares. See §5 for the two mutations that prove it can
fail.

> the output names both what it omitted and the verb to see the rest

The `omitted` block above. `TestTheOmissionIsNamedWithTheVerbToSeeTheRest` also
cross-checks the arithmetic against the ledger — omitted + rendered = the number
of accepted decisions on disk — so the count cannot be a placeholder.

> a ceiling low enough to admit only one section still yields a well-formed brief
> containing the open blockers

`TestASingleSectionCeilingStillYieldsTheBlockers` at `max_tokens = 200`: the
heading survives, `open blockers` survives with a whole entry under it, no
decision is rendered, the omission is stated, and the entry is whole.

> `dira brief --context --chain` with no `[parents]` configured exits 0 and
> states that no parent ledger is configured

Observed, run as the hook runs it, in this repository:

```
$ dira brief --context --chain 2>/dev/null || true
dira brief — dira · 2026-07-31

Settled records from this repository's ledger, injected at session start. Treat
them as decided: run `dira check "<plan>"` before planning something that may
contradict one, and `dira why <id>` for the reasoning behind any line.

chain: no parent ledger is configured ([parents] in .dira/config.toml is empty),
so this brief is this repository's ledger alone.

open blockers
  qst-0003  Does bulk-importing an existing ADR corpus produce   open 2026-07-29
            a useful ledger, or a second pile?
              blocks int-0003 — Replace the tool pile — kazi + dira + a subset
              of skills is the whole stack
...
exit=0
```

A commented-out `[parents]` line is **not** a declaration (the rule
`scripts/privacy-lint.py` already applies to the same file), and a ledger that
*does* declare one is told what dira cannot do rather than having its
configuration silently ignored:

```
chain: one parent ledger is configured as sire, but resolving a parent ledger is
not in this release, so this brief is this repository's ledger alone.
```

### Fail-open, which the hook's `2>/dev/null || true` makes load-bearing

- One malformed entry file: exit 0, the brief renders without it and **names it
  in the brief** (not only on stderr, which the hook discards).
  `TestOneMalformedEntryDegradesToABriefWithoutIt`.
- A `max_tokens` dira cannot parse: exit 0, the brief says so and falls back to
  cst-0001's 1,500 rather than to no ceiling.
  `TestAConfigDiraCannotUnderstandStillYieldsABrief`.
- An empty ledger: exit 0, every section says "none — …" rather than rendering
  an empty page. `TestABriefOverAnEmptyLedgerIsStillABrief`.
- No ledger at all (run from `/tmp`): exit 1 with a message, swallowed by the
  hook's `|| true`. Observed `exit=0` for the whole hook command.
- A read surface that cannot answer at all is still an **error** and writes
  nothing (`TestASourceThatFailsIsAnError`) — an unreadable ledger must not be
  rendered as an empty one.

### Warm and cold answer identically

The brief is now part of E1-L3's differential harness, which is what that package
was left extensible for: `TestTheBriefIsTheSameWithAndWithoutACache` runs three
forms through `indextest.RunTwice` (warm, then with `.dira/cache/` deleted before
every query) and compares bytes.

---

## 4. Measurements

Median of 25 runs each, on a machine running a dozen parallel agent sessions —
absolute numbers are inflated, the deltas are what to read.

**In-process, 200-entry fixture** (`local.Open` + `index.Open` + `brief.Render`):

| | median |
|---|---|
| warm cache | **17.3 ms** |
| cold cache (`.dira/cache/` removed before each run) | **61.2 ms** |

That is E1-L3's measured read path (15.1 ms warm / 55.5 ms cold) plus 2–6 ms of
brief. **The cold-cache database build dominates**, exactly as E1-L3's report
predicted, and it is the number E1-L6 has to decide about — not anything in this
lane's rendering.

**Through a process boundary** (same fixture, same machine):

| | median | p90 |
|---|---|---|
| `dira version` (spawn baseline) | 92.8 ms | 95.6 ms |
| `dira brief --context --chain`, warm | 122.7 ms | 132.4 ms |
| `dira brief --context --chain`, cold | 184.0 ms | 226.2 ms |
| `dira why dec-0002`, warm | 109.8 ms | 112.3 ms |

dira's own work is ~30 ms warm and ~91 ms cold; the rest is process spawn on a
loaded machine.

**Entry files opened per brief** (200 in the ledger), which is where the cold
budget is actually spent or saved:

| ceiling | entry-file reads | entries rendered |
|---|---|---|
| 1500 | 43 | 24 |
| 700 | 22 | 8 |
| 300 | 8 | 2 |

The ceiling decides which entries get read, so ~170 of 200 files are never
opened. Reads exceed rendered entries because a blocker's `blocks` target is read
for its title, and because the first entry that does not fit is built before it
is refused.

**The counter, measured on this repository's own brief:** 1,271 dira tokens for
3,637 bytes — **1.40×** the four-characters-per-token rule of thumb. The margin
is the price of not shipping a vocabulary, and it is in dec-0020 rather than
hidden.

---

## 5. Every new gate, proven able to fail

Four mutations, applied to the shipped code, reverted after each. All were run
against the full brief suite.

| mutation | caught by |
|---|---|
| **M1** the ceiling is ignored (`add` always accepts) | 7 tests, incl. all three ceiling tests, the omission test and the single-section test |
| **M2** the keep order is reversed | `TestTheDropOrderIsTheKeepOrderReversed`, `TestOverflowDropsWholeEntriesFromTheLowPriorityEnd`, `TestASingleSectionCeilingStillYieldsTheBlockers` |
| **M3b** titles are cut to 30 characters instead of wrapped | `TestTheWholenessCheckHasTeeth`, `TestOverflow…`, `TestASingleSection…` |
| **M3c** the `blocks` sub-line is dropped for long rows | the same three |
| **M4** `--chain` prints nothing | both chain tests |

**Two of these initially passed, and both were my gates being too weak.** They
are the most useful result in this report:

- **M2 first ran green.** The priority test asserted that sections degraded
  consistently with whatever order `brief.sections()` declares — which is true
  when the order is reversed. It now names cst-0001's order as a literal
  (`keepOrder`) and asserts what survives is a *prefix* of it, which fails
  vacuously in neither direction. A gate that reads its expectation out of the
  code under test is a mirror, not a gate.
- **M3 first ran green** (truncate the last block to two thirds). Two reasons,
  and only one was a hole: the wholeness check looked at titles only, so a cut
  that removed a `blocks` line was invisible — now fixed, and M3c exists to keep
  it fixed. The other reason is benign: that mutation is *unobservable*, because
  the footer trim gives the last block back anyway, so the renderer still never
  emitted a truncated block. M3b/M3c replace it with truncation that persists.

Two more, from the second task:

| mutation | caught by |
|---|---|
| `Resolve` as it is at HEAD | `TestScatteredTokensResolve` (3 sub-cases), `TestAContiguousMatchIsTheWholeAnswer`, `TestScatteredMatchesAreReturnedWhenThePhraseIsNowhere` |

Observed red, before the change:

```
--- FAIL: TestScatteredTokensResolve
    Resolve("status derived") = [], which does not contain dec-0004
    Resolve("derived status") = [], which does not contain dec-0004
    Resolve("kazi   status") = [], which does not contain dec-0004
```

---

## 6. The second task: `Resolve`

`Resolve` now requires **every word** of a term to be a substring of the title or
a whole tag, and returns matches in two tiers: the term found contiguously, and
the term's words all found somewhere.

**The tiering is not a refinement; it is the property that makes the change
safe.** Ranking contiguous matches first — which is what the lane brief asked
for — is not sufficient, and I found that by running it. `dira why` renders a
disambiguation list rather than a chain the moment a term matches more than one
entry, so under a rank-only scheme:

```
$ dira why "read time"
2 entries match "read time"

   dec-0004  Execution status is derived from kazi at read   accepted 2026-07-29
             time, never stored in the ledger
   cst-0003  Inheritance is one-way and read-time only — …     active 2026-07-29
```

dec-0004 was still first, and the reader still lost the chain they used to get,
because cst-0003's "read-time" contains both words. So: **when the contiguous
tier has any member, it is the whole answer.** Scattered matching only runs when
the phrase is nowhere. Nothing a reader types today changes what they see; terms
that found nothing yesterday can now find something.

Observed after the change:

```
$ dira why "read time"        →  dec-0004's chain (as before)
$ dira why "status derived"   →  dec-0004's chain (found nothing before)
$ dira why "derived status"   →  dec-0004's chain (word order is not part of it)
$ dira why "daemon tokenizer" →  nothing (one word matches nothing here)
```

No embeddings, no new dependency, the exact-id path untouched.

**The goldens did not move.** Built from this tree and run against the ledger and
the golden files *as they are at HEAD*:

```
OK   dec-0002 (against the goldens as they were at HEAD)
OK   daemon
OK   int-0002
OK   dec-0012
OK   dec-0015
```

`TestASingleWordResolvesExactlyAsItDid` also compares single-word results against
a second, independent implementation of the old rule rather than against a
recorded list.

### But two goldens *were* regenerated, for a different reason — read this

`cmd/dira/testdata/why/daemon.golden` and `int-0002.golden` each gained four
lines. **Not from the ranking change** — from `dec-0020`, whose `informs
int-0002` edge appears as an incoming edge on int-0002's chain:

```
+               dec-0020  A "token" in brief.max_tokens is dira's own
+                         conservative estimate, not a model's tokenizer
+                           counting costs about a microsecond per rendered brief
+                           and adds nothing to the binary
```

That is a ledger addition doing what the golden test says it should
("if the change is intended, re-run with -update and read the diff"). The edge is
true and worth keeping — dec-0015 carries the same edge for the same reason — so
I regenerated rather than removing it. `dec-0012.golden` also differs from HEAD,
by five lines naming `dec-0019`; that is another lane's entry and another lane's
regeneration, already in the working tree when I arrived.

---

## 7. What I refused to do

- **No `--max-tokens` flag.** It is the obvious convenience and it is a hole in a
  constitutional constraint: cst-0001 says raising the ceiling requires
  superseding the constraint *in writing*. A flag would let any invocation opt
  out of the rule the same binary enforces. Lowering is possible through
  `.dira/config.toml`, which is a reviewable file in git.
- **The omission notice does not suggest raising `brief.max_tokens`**, for the
  same reason, and there is a test asserting the string `max_tokens` is absent
  from the brief. The verb it offers is `dira why <id>` — the only read verb E1
  ships. `dira map` is E4's and pointing at it now would point at nothing.
- **No execution status anywhere.** No bucket, no "in progress", no mark on a
  `realized_by` target. `TestTheBriefAssertsNoExecutionStatus` asserts the
  absence of `converged`, `in progress`, `done`, `planned`, `running` and
  `goal-` from the rendered brief (dec-0004). I held the line the earlier lane
  drew.
- **No parent-ledger traversal.** `--chain` states what it cannot do. Tier
  resolution is E5 and is blocked on qst-0001.
- **No TOML library.** `internal/config` reads three keys and ignores everything
  else, including syntax it does not understand. It is documented as not being a
  TOML parser. The command path's module allowlist is unchanged by this lane —
  `cmd/dira/build_test.go` needed no edit.

---

## 8. Findings, and things the integrator should decide

**1. dira's own brief is at 93% of its ceiling with 33 entries.** The
`--context --chain` form measures 1,401 of 1,500 dira tokens today. Four or five
more accepted decisions and this repository's own brief starts dropping entries.
That is cst-0001 working as designed, and it is also a product signal worth
seeing early: the *first* thing that will be dropped is recent decisions, which
is the section design.md §10 step 1 leans on ("Claude already knows int-0002,
dec-0042, qst-0007 open").

**2. Strict priority makes big ledgers blocker-only.** On the 200-entry fixture,
16 open blockers and 9 active intents consume the whole 1,500-token budget and
*no decision is rendered at all*. That is exactly what the acceptance line
specifies ("recent decisions dropped first"), so I implemented it and did not
soften it. But the observable consequence is that on a ledger with many open
questions, the brief stops containing decisions entirely — and "review is push"
was mostly about decisions. A per-section floor (say, three of each before the
tail is dropped) would fix it and is **editorial judgment**, which cst-0001
forbids the binary to exercise. **This needs a decision above this lane**: either
cst-0001 is amended to describe a floor, or the behaviour stands. I have not
made that call.

**3. cst-0002's "notes surface once and then decay" is not implementable as
written by a read verb.** Surfacing exactly once needs a memory of what was
already surfaced; dira has nowhere honest to keep one (a read verb that wrote a
"seen" marker would be storing derived state in the files, and two parallel
sessions would disagree). Implemented as a **7-day window** instead:
`brief.NoteWindow`, documented at the constant, tested by
`TestNotesDecayOutOfTheBrief`. Weaker than "once", stronger than "forever", needs
no write. Whoever owns per-session state later (E2 installs the hooks; nothing in
E1 owns it) can replace it. **This is a knowing under-delivery against cst-0002's
wording and should be read as one.**

**4. A private entry is cited by ref and never by title, in the brief too.**
cst-0003 binds E1 in only one place per the pinned interpretations (the cache
stays gitignored), so this is a choice rather than an obligation. I made it
because `internal/enforcer` already made it, and its reasoning transfers exactly:
the binary cannot tell whether its stdout is a terminal or a pull-request body.
`TestAPrivateEntryIsCitedByRefOnly`. If the integrator thinks a locally-injected
brief is different from a check message, this is the line to argue about.

**5. `README.md`'s status paragraph is stale.** It still says "no `log`, no
`why`, no `brief` yet" — three commands that now exist. I did not edit it: it was
being modified by another session while I worked, and a conflicting edit to a
file someone else has open costs more than it fixes. It needs one sentence from
whoever integrates.

**6. This working tree is shared with several other lanes.** While I worked,
other sessions added `internal/sniff`, `internal/ui`, `cmd/dira/supersede.go`,
`.dira/entries/dec-0019.md` and edits across `internal/enforcer` and
`docs/design/`. Everything reported here was measured with those present, and
`go test ./...` is green across all of it. Two consequences worth knowing:
   - **`git stash@{0}` contains a duplicate of my `internal/index/query.go`
     change.** A `git stash push` of that single file hit `.git/index.lock`
     contention from another session and half-succeeded; the working tree has the
     change and is correct, and the stash is redundant. **Drop it, do not pop
     it.** The same accident left `internal/index/query.go` staged in the index
     rather than merely modified; the content is identical either way.
   - `dec-0020` is the id I took because `dec-0019` was already claimed by a
     concurrent lane. If a third lane also took `dec-0020`, renumber mine — its
     only inbound references are `internal/brief/tokens.go`'s doc comment and
     `docs/coverage.md`.

**7. One thing I could not measure honestly: the counter's accuracy.** There is
no tokenizer in this repository to compare against, and dec-0003 forbids
acquiring one. The claim I can defend is bounded and is the one dec-0020 makes:
the count is never *below* what any tokenizer needs (one per word, one per line —
`TestTokensNeverUnderCountsARealTokenizer` asserts that lower bound), and on real
briefs it is 1.40× the chars/4 rule of thumb. If someone with a tokenizer to hand
measures it and the ratio is above ~1.6, `trigger:dec-0020:7f7d69` is the row
that says what to do.

---

## 9. Gate output, verbatim

```
--- build
OK
--- vet
--- lint
0 issues.
--- tests
ok  github.com/kazi-org/dira/cmd/dira               16.636s
ok  github.com/kazi-org/dira/internal/brief         (cached)
ok  github.com/kazi-org/dira/internal/config        (cached)
ok  github.com/kazi-org/dira/internal/enforcer      3.576s
ok  github.com/kazi-org/dira/internal/index         (cached)
ok  github.com/kazi-org/dira/internal/ledger        3.717s
ok  github.com/kazi-org/dira/internal/ledger/fixture (cached)
ok  github.com/kazi-org/dira/internal/ledger/local  1.874s
ok  github.com/kazi-org/dira/internal/render        (cached)
ok  github.com/kazi-org/dira/internal/sniff         (cached)
ok  github.com/kazi-org/dira/internal/ui            (cached)
ok  github.com/kazi-org/dira/internal/why           (cached)
ok  github.com/kazi-org/dira/schema                 (cached)
--- privacy
PRIVACY LINT PASS — cst-0003 enforced by 4 checks.
--- coverage
uncovered             : 0
COVERAGE PASS — every obligation has a disposition.
```

One caveat on `go test ./...`: `internal/ledger/fixture`'s
`TestFullLedgerReadIsWithinBudget` failed once during a full-suite run under
heavy parallel load (median 254 ms against a 150 ms budget) and passes when run
alone (`ok`, 1.1s). It is not this lane's code — it measures the ledger read
path — but E1-L6 should know that this budget is sensitive to machine load
before it writes a CI multiplier.
