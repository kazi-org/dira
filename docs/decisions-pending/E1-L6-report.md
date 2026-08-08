# E1-L6-T5 — the cold run, attributed to phases, and the one redundant pass removed

**Lane:** E1-L6. **Task:** T5. **Status:** complete, unmerged, uncommitted.
**Verifies:** `int-0002`.

Two things are in this report and they are different kinds of claim.

**The attribution** is a measurement: one cold `dira brief --context --chain`,
broken into seven named phases that sum to within 1% of the same run's measured
median. It is taken on the maintainer's machine, where the spawn floor makes the
absolute numbers useless as a verdict but does not stop the decomposition from
being valid — every phase is inflated by the same contention, and what the
attribution is for is the *shape*.

**The optimisation** is a structural fact rather than a statistical one: a cold
build read every entry file **twice** — once to hash it, once to parse it — and
the hash pass decided nothing, because on an empty cache there is no cached
version for a hash to be compared against. That pass is now skipped on that one
path. It is asserted by counting calls, not by timing them, and both sides of
every assertion have been observed.

**int-0002's gate is green on CI and stays green.** No budget in
`internal/perf/budget.go` was touched, no single-run ceiling was reintroduced, and
nothing here is a daemon, a resident process, a warm-up trick or a network cache.

---

## 1. The numbers CI reports, which are the binding ones

`.github/workflows/ci.yml`'s `perf` job, latest run (`31230739083`, 2026-08-08),
`dira brief --context --chain` spawned over the 200-entry fixture:

| | ubuntu-latest | macos-latest |
|---|---|---|
| cold brief, median | **38.967 ms** | **55.294 ms** |
| cold brief, min | 36.987 ms | 43.245 ms |
| warm brief, median | 14.130 ms | 29.752 ms |
| cold `reindex`, median | 35.126 ms | 53.972 ms |

Against `int-0002`'s 100 ms that is 61 ms of headroom on linux and 45 ms on
darwin. **On the median, on a healthy runner, "well under 100ms" is met and it is
not a near miss.** Section 6 is where that sentence gets its qualifications.

---

## 2. Method, and why it is this one

`internal/perf`'s harness times a spawn from the outside and cannot see inside it.
The attribution needed the inside, so:

- **Phase marks in the process.** A throwaway copy of the tree — outside the
  repository, never committed, and reproducible from the description below —
  carries `time.Now()` marks at seven points on the `brief` path, plus three
  accumulators inside `index.sync` and `index.Entry`. A driver materialises the
  same `fixture.Seed`/`fixture.Size` ledger, removes `.dira/cache` before every
  run, takes one untimed warm-up and then twenty timed samples, and times the
  spawn exactly the way `perf.Measure` does — `exec.Command`, `cmd.Run()`, a
  `time.Now()` either side. The parent's pre-`Run` instant and the child's first
  instant in `main` are both wall clock on one machine, so their difference is
  process creation plus everything before `main`.
- **`GODEBUG=inittrace=1`** for the package-init split, over 31 runs of
  `dira version`, which is the same binary doing no ledger work.
- **`go test -bench`** for the one phase that was removed, because a benchmark
  repeats an operation under its own timer and is the only instrument on this
  machine with enough signal to resolve 10 ms.

**Nothing was instrumented in the repository.** T5's `files:` list does not
include `internal/perf`, and permanent timing instrumentation in the command path
would be a cost paid by every user to answer a question asked once.

**Why the local numbers are not a verdict, stated with its evidence.** Measured in
the same window as the runs below, on the same machine:

```
/usr/bin/true        min  81.81 ms   median  97.57 ms
dira version         min 156.04 ms   median 247.50 ms
```

A binary that does nothing at all takes 97.57 ms against a 100 ms ceiling, and a
`dira` invocation that opens no ledger takes 247.50 ms. The load average during
this work ranged from 6 to 322 on four cores, from a dozen sibling sessions
compiling. `hooks/pre-commit` already reports this condition as NOT RUN TO A
VERDICT rather than as red, and that is the correct reading of every absolute
number in sections 3 and 4. **CI is the authority. Section 1 is the verdict.**

---

## 3. The attribution

One cold `dira brief --context --chain`, n=20, cache removed before every timed
run, taken during the quietest window available (load average 6.3):

| # | phase | min | median | share of min |
|---|---|---|---|---|
| 1 | process start, dynamic loading, Go runtime init, package init | 107.632 ms | 118.563 ms | 60.8% |
| 2 | subcommand dispatch and flag parse | 0.012 ms | 0.014 ms | 0.0% |
| 3 | `local.Find` + `local.Open` (locate the ledger) | 0.077 ms | 0.087 ms | 0.0% |
| 4 | config read (`.dira/config.toml`, parse) | 0.022 ms | 0.028 ms | 0.0% |
| 5 | cache open — `MkdirAll`, SQLite attach, WAL pragmas, schema create | 3.811 ms | 6.016 ms | 2.2% |
| 6 | cold build — list, read, parse, insert, commit | 52.061 ms | 63.985 ms | 29.4% |
| 7 | render — select, read the entries that print, fill, write stdout | 5.352 ms | 7.559 ms | 3.0% |
| 8 | exit — `ix.Close()`, WAL checkpoint, process teardown, `wait4` | 4.363 ms | 6.280 ms | 2.5% |
| | **sum of phases** | **173.330 ms** | **202.532 ms** | |
| | **measured total** | **176.951 ms** | **200.745 ms** | |

**Reconciliation: the phases sum to 98.0% of the measured minimum and 100.9% of
the measured median.** The acceptance line asks for 10%; this is inside 1%. The
median row exceeds 100% because a sum of per-phase medians is not the median of
the sums — different samples supply each phase's middle value — which is why the
minimum column is the one to read.

Three accumulators inside phases 6 and 7, which is where the finding is:

| accumulator | min | median | what it is |
|---|---|---|---|
| `sync.list` | **10.431 ms** | 13.835 ms | `store.List` — open, read and SHA-1 **all 200** entry files |
| `sync.entry_reads` | 28.125 ms | 33.002 ms | `store.Get` × 200 — open, read and **parse** the same 200 files |
| `render.entry_reads` | 3.997 ms | 5.861 ms | `index.Entry` — the entries the brief actually prints |

So of the 69.3 ms of in-process work on the minimum (total 176.951 less phase 1's
107.632), the cold build is 75% of it, and **15% of the whole in-process cost was
a second read of files the next 28 ms was about to read again.**

### Phase 1, split further

`GODEBUG=inittrace=1`, `dira version`, n=31:

| | min | median |
|---|---|---|
| all package init, 135 packages, 69,559 allocations before `main` | **14.42 ms** | 23.57 ms |
| ├ `modernc.org/libc/honnef.co/go/netdb` | 7.400 ms | 12.000 ms |
| ├ `github.com/santhosh-tekuri/jsonschema/v6` | 5.300 ms | 8.900 ms |
| └ everything else, 133 packages | ~1.7 ms | ~2.7 ms |

Phase 1 therefore splits into **~93 ms of process creation and Go runtime start —
this machine's spawn floor, and the same ~95 ms the earlier harness attribution
reported — and ~14.4 ms of package init**, of which **88% is two packages the
`brief` path never calls** (section 9).

`embed.FS` is named in the acceptance line and is worth reporting as a
non-finding: `schema.Schema` and the skill assets are `[]byte`/`embed.FS` package
variables, which are data-section references rather than init work. No embed
appears in the init trace at a measurable cost. The cost attributed to the schema
package is `jsonschema`'s compiler registry, not the embedded document.

---

## 4. Headroom per phase, and what breaks first

Read against `int-0002`'s 100 ms and the CI medians in section 1, in the order a
regression would arrive:

1. **The cold build (phase 6) — 75% of in-process work, and the only phase that
   scales with the ledger.** It is linear in entry count: 200 files opened, read,
   parsed, and inserted. dira's own ledger is 40 entries today; the fixture is
   200. A ledger of 600 would add roughly twice the current cold build — on
   ubuntu-latest, roughly 39 ms → 80 ms, still inside the ceiling; at ~1,000
   entries linux crosses it and darwin crosses it sooner. **This is the phase
   that will break the budget first, and the number to watch is entries, not
   milliseconds.**
2. **Package init (part of phase 1) — flat, and paid by every invocation warm or
   cold.** It cannot be outgrown but it also never shrinks, and on the *warm*
   budget it is a much larger share: ubuntu's warm median is 14.130 ms in total.
   A new dependency with an eager init is the second-most-likely regression here
   and the least visible, because it will not show up in any diff to
   `internal/brief` or `internal/index`.
3. **Cache open (phase 5), 5.5%.** Fixed cost of creating a SQLite database and
   its schema. Bounded by dec-0015's design; nothing to win without changing it.
4. **Render (phase 7), 7.7%.** Already reads only the entries that will print —
   `dira brief`'s own comment claims "a few dozen file reads rather than 200" and
   the accumulator confirms it at 4.0 ms against the cold build's 52.1 ms.
   cst-0001's token cap bounds it, so it does not grow with the ledger.
5. **Exit (phase 8), 6.3%.** WAL checkpoint and process teardown after the brief
   has already been written to stdout. See section 9 for why it is left alone.
6. **Flags, ledger location, config (phases 2–4), 0.2% combined.** 111 µs of the
   whole invocation. There is nothing here.

---

## 5. What was cut

**`internal/ledger/local/local.go`** gains `ListIDs`, a listing that reads the
entries directory and opens no entry file. `List` and `ListIDs` are one
implementation with one flag, so the two cannot disagree about which files count
as entries.

**`internal/index/sync.go`** reads the cache's own versions *first*, and when
there are none — a cold build, and `dira reindex`, which discards the cache before
opening it — enumerates the ledger through `ListIDs` instead of `List`, via an
optional `idLister` interface that `ledger.Store` does not require.

### Why this is not a weakening of the staleness check

The version a row is stored with was **never** the one the listing computed.
`sync` writes `entry.Version()` — the hash of the bytes it actually parsed — and
falls back to `info.Version` only when the store leaves an entry's own version
empty. `List`'s hash exists for exactly one purpose: to decide, per entry, whether
a cached row is still current. **When there are no rows, there is nothing to be
current.** The rows this path writes are byte-for-byte the rows the versioned path
writes, and the next run's warm reconcile compares against identical values.

The condition is `no rows at all`, not `Rebuilt`: one surviving row is enough to
take the full versioned listing. On a warm cache nothing changed, and that is the
path dec-0015 is about.

### The measurement

Both arms are the same tree with the same instrumentation, differing only in this
change, run back to back on the same machine (load average ~39):

| | before | after |
|---|---|---|
| `sync.list` — min | 11.212 ms | **0.457 ms** |
| `sync.list` — median | 18.876 ms | **0.717 ms** |
| cold build (phase 6) — median | 89.273 ms | 71.070 ms |

And the removed work measured on its own, `-benchtime 30x -count 8`, minimum of
eight runs (the uncontended estimate `internal/perf/doc.go` names as the right
input to a comparative claim):

```
BenchmarkListVersioned   min 60.60 ms/op      (200 files opened, read, SHA-1'd)
BenchmarkListIDsOnly     min  0.70 ms/op      (one readdir)
```

**What this is worth on CI, stated as the projection it is.** The list pass was
15.0% of in-process cold work on the minimum. Applying that share to
ubuntu-latest's 38.967 ms cold median — of which perhaps 5 ms is spawn and init —
projects roughly **5 ms off a linux cold brief and 7 ms off a darwin one**, i.e.
about 13% of the cold invocation. That arithmetic is shown so it can be checked
rather than believed: **the binding number is this branch's first `perf` job run**,
and if it does not move, the projection was wrong and the report is wrong with it.

A whole-binary A/B through spawn timing was attempted and is reported as a
**failure of the instrument, not as a result**: interleaved, n=40 per arm, it
returned +1.4% on the minimum and −7.7% on the median against a machine whose
spawn floor was moving between 250 ms and 2.2 s. A 5 ms effect is not resolvable
there, which is exactly why the claim rests on call counts and a benchmark.

### Both sides, per `docs/lore.md` L-0001

`TestAColdBuildDoesNotHashTheLedgerTwice` (`internal/index/coldlist_test.go`)
counts `List`, `ListIDs` and `Get` through a wrapping store. Five subtests, each
with both sides; the green side is the load-bearing one.

| subtest | what it asserts |
|---|---|
| a cold build lists the ledger without hashing it | `ListIDs` 1, `List` 0, `Get` 200, indexed 200 |
| **the rows it wrote satisfy the warm reconcile** | reopening re-reads **0** files and re-indexes **0** entries — the green side, and the only thing that proves the skipped pass changed no stored value |
| an edited entry is still the only thing re-read | one file changed → exactly 1 `Get`, 1 re-indexed |
| the cheap listing is what the cold build indexes | an id withheld from `ListIDs` alone yields 199 entries — if the cold build were still reading through `List`, the counters above would be counting a call nobody acts on |
| a backend with no cheap listing builds the same index | the fallback indexes 200 and its cache reopens clean through the fast path, so the two listings store identical versions |

**Both defects were constructed and observed red**, then reverted:

- *fast path disabled* (`ix.list(ctx, false)`) → `ListIDs called 0 times, want 1`
  and `indexed 200 entries with one withheld from ListIDs, want 199`.
- *fast path always taken* (`ix.list(ctx, true)`) → `re-read 200 entry files over
  a ledger nothing had changed, want 0`, `re-indexed 200 entries after one file
  changed, want 1`, and the fallback's cross-open re-indexing 200.

`TestListIDsSeesTheSameEntriesAsListWithoutOpeningThem`
(`internal/ledger/local/listids_test.go`) proves the "opens no file" claim without
timing anything: the entry files are chmod'd to `0o000` and the two methods are
run over them. **Red:** `List` fails, because it must open each file — and if it
did not, the test fails loudly, since a green side over a ledger whose
permissions are not doing what the test assumes proves nothing. **Green:**
`ListIDs` returns the same ids, and on a readable ledger the two agree on the id
set while `ListIDs` reports no versions at all. It skips under `root`, which
ignores the file mode the proof rests on.

---

## 6. Is `int-0002` met? The honest answer has two halves

**On the median, on a healthy runner: yes, and comfortably.** 38.967 ms and
55.294 ms against 100 ms is "well under", not "green with 6 ms of headroom".

**Across runs on darwin: not always.** Five CI runs, both platforms, every cold
sample harvested from the run logs — 200 samples, which is the measured
distribution `dec-0026`'s `revisit_if` asks for:

| | ubuntu-latest | macos-latest |
|---|---|---|
| per-run medians (5 runs) | 33.0, 38.8, 39.7, 39.8, 45.2 | 49.2, 50.4, 55.1, 61.0, **109.7** |
| pooled min | 32.0 ms | 39.4 ms |
| pooled p50 | 39.7 ms | 53.7 ms |
| pooled p90 | 45.5 ms | **111.1 ms** |
| pooled p95 | 47.0 ms | 132.2 ms |
| pooled p99 | 115.3 ms | 152.1 ms |
| pooled max | 191.0 ms | 163.9 ms |
| samples over 100 ms | 3 / 100 | **14 / 100** |
| **run medians over 100 ms** | 0 / 5 | **1 / 5** |

**One macOS run in five broke the median.** Run `30669750621`, 2026-07-31, cold
median 111.073 ms — seven minutes before `dec-0026` was written, and on the
platform `int-0002` is primarily written about. That is a red the lane has
recorded nowhere, and it is recorded here.

It matters *which* red it is. That run's **minimum was 49.678 ms**, against a
typical macOS minimum of 39–44 ms. The whole distribution had shifted: this was a
degraded runner, not a slower dira. `.github/workflows/ci.yml`'s perf-job comment
already names that reading — "a median over the ceiling with the minimum far under
it is a noisy runner" — and it is the correct one here.

**A correction the lane should carry.** `dec-0026`'s second rejected alternative
gives as its reason that "the median on CI passes at roughly 95ms against a 100ms
ceiling, so the cold path already meets the budget". No CI run supports that
figure. Every cold median observed on the perf job is between 33.0 ms and 109.7
ms, and the run that motivated `dec-0026` had a ubuntu median of 33.167 ms. The
conclusion — that a 191 ms single sample grades the scheduler rather than dira —
is right, and the measured evidence in this section supports it more strongly than
the number quoted in the entry does. T6 should carry the correction, not reopen
the decision.

### What this distribution says about a percentile ceiling

`dec-0026` parked a percentile ceiling until a measured distribution existed. It
now exists, and it says **a percentile ceiling still cannot be derived honestly**:

- macOS's pooled **p90 is 111.1 ms**, above the 100 ms budget it would be
  enforcing. Any p90 ceiling honest to this data would have to sit above
  `int-0002`'s own number, which is a gate that certifies nothing; and one set at
  100 ms would be red on a correct binary roughly one run in five — L-0001 rule 2
  in its purest form.
- The variance is **between runs, not within them**. Four of five macOS runs have
  a p90 under 80 ms; the fifth has every sample shifted upward including its
  minimum. A within-run percentile cannot separate those two populations, so no
  choice of percentile fixes it.

**What the data does support, and what T6 should record as the shape of a future
gate:** the discriminator is not a percentile, it is the **minimum**. The one red
macOS run's minimum was inside the ceiling while its median was not, which is
precisely the "this machine cannot judge" signal that
`internal/index/latency_test.go` and `internal/ledger/fixture/latency_test.go`
already implement — compare the *best* sample to the ceiling to decide whether the
run is entitled to a verdict at all, and assert the median only when it is.
`internal/perf` reports the minimum today and asserts nothing with it. Making that
a verdict is a task, not a constant, and it needs the same both-sided treatment as
everything else here. It is **not** proposed as a loosening: a run whose minimum
is over the ceiling would go red on that alone.

---

## 7. What was refused, and why

- **No daemon, no resident process, no warm-up trick.** `int-0002` forbids them
  and the budget exists to protect the property they would violate. The 93 ms
  spawn floor on this machine is the largest single phase in section 3 and a
  pre-forked helper would remove most of it. It is refused outright.
- **No network cache**, and nothing here reaches the network (`cst-0004`).
- **No edit to `internal/perf/budget.go`.** Its header forbids raising a number to
  fix a red run, and nothing here needed one.
- **The single-run ceiling stays removed.** `dec-0026` removed `coldMaxBudget`;
  section 6's distribution supports that removal rather than reversing it — 1% of
  ubuntu samples exceeded 150 ms while every ubuntu run median passed.
- **The parsed entries are not memoised between the cold build and the render.**
  `sync` parses all 200 entries and `render` then re-reads about 20 of them from
  disk (`render.entry_reads`, 4.0 ms on the minimum). Keeping them in memory would
  save that, and it would put a value that did not come from a file read at render
  time on the rendering path — the exact property `internal/index`'s package
  comment holds structurally ("nothing dira prints comes from the cache"). Four
  milliseconds on a busy laptop, one or two on CI, is not worth trading a
  structural invariant for a careful one.
- **`ix.Close()` is not skipped on the happy path.** Phase 8 is 6.3% of in-process
  work and happens after stdout is already written, so skipping the WAL checkpoint
  would look free. It is not: the work moves to the next process's recovery, which
  is the *hook* invocation this budget is about, and "fast because it left a mess
  for the next run" is the kind of trick the budget exists to prevent.

---

## 8. What is out of `files:` and is a follow-up task, with its measurement

**`jsonschema` is linked into the command path, and package `schema`'s own doc
comment says it is not.** `schema/schema.go` opens with "Nothing in the dira
command path imports this package: JSON Schema validation costs a compile of the
schema document on every invocation, which int-0002's budget cannot absorb."

`internal/ledger/decode.go` imports it — for `schema.SplitFrontmatter` and
`schema.ErrNoFrontmatter`, two functions with no JSON Schema in them — and
`internal/ledger` is in the command path:

```
$ go list -deps ./cmd/dira | grep -E 'jsonschema|dira/schema'
github.com/santhosh-tekuri/jsonschema/v6/kind
github.com/santhosh-tekuri/jsonschema/v6
github.com/kazi-org/dira/schema
```

Measured by difference, building the same tree with and without that import (this
was done in the throwaway copy; **no such change is in this branch**):

| | with | without | saved |
|---|---|---|---|
| package init, min | 13.32 ms | 9.13 ms | **4.19 ms per invocation** |
| allocations before `main` | 69,564 | 47,852 | **21,712** |
| packages initialised | 136 | 128 | 8 |
| binary size | 20,905,440 B | 19,646,944 B | **1.26 MB** |

Every `dira` invocation pays it — `brief`, `sniff`, `check`, `log` — warm and
cold. It is the largest cuttable item found and it is **not cut here**, because
the fix is in `internal/ledger/decode.go` and `schema/`, and T5's `files:` list
grants `internal/index`, `internal/brief` and `internal/ledger/local` only. The
shape of the fix is one move: frontmatter splitting is not schema validation, so
it belongs in a package that does not import a validator, and `schema`'s doc
comment becomes true again.

**`modernc.org/libc/honnef.co/go/netdb` costs more still** — 7.4 ms minimum,
12.0 ms median, 3.66 MB and 44,050 allocations of eager package init, for a
services/protocols database dira never consults. It arrives with
`modernc.org/sqlite`, which `dec-0015` and `dec-0001` chose deliberately for
cgo-free cross-compilation. It is not dira's code and is recorded as a cost of
that decision, not as a defect: the honest options are an upstream fix or a
different driver, and neither belongs in this lane.

---

## 9. Files changed

| path | what |
|---|---|
| `internal/index/sync.go` | read the cache's versions first; `list` + the optional `idLister`; the header comment now describes the order it actually runs in |
| `internal/ledger/local/local.go` | `ListIDs`; `List` and it share one `list` implementation |
| `internal/index/coldlist_test.go` | **new** — the five call-counting subtests above |
| `internal/ledger/local/listids_test.go` | **new** — the unreadable-ledger proof and the empty-ledger case |
| `docs/decisions-pending/E1-L6-report.md` | this file |

Nothing outside T5's `files:` list was touched. `cmd/dira/main.go`,
`docs/roadmap.md`, `docs/coverage.md` and `internal/perf/budget.go` are untouched.

---

## 10. Gates, on this tree

| gate | result |
|---|---|
| `go build ./...` | pass |
| `go vet ./...` | pass |
| `go vet -tags perf ./internal/perf` | pass |
| `golangci-lint run ./...` | pass, 0 issues |
| `gofmt -l internal/` | clean |
| `go test ./...` | pass, 18 packages |
| `python3 scripts/coverage.py` | pass — 87 obligations, 0 uncovered |
| `python3 scripts/privacy-lint.py` | pass — 4 checks |
| `go test -tags perf -count=1 -p 1 ./internal/perf` | **NOT RUN TO A VERDICT on this machine** — see below |

The perf package's verdict on this machine is not evidence and is not claimed as
any. `TestReindexBudget` passed. `TestColdStartBudget` and `TestWarmBriefBudget`
reported their absolute budgets broken — cold median 228.253 ms, warm median
565.617 ms — in a window where `/usr/bin/true` alone took a 97.57 ms median and
`dira version` took 247.50 ms against the same 100 ms ceiling. Every one of the
instrument's own subtests passed: the argv still matches the hook, a drifted hook
is read as drifted, a broken argv is rejected, an empty ledger fails as measuring
nothing, a surviving cache fails as mislabelled cold, the ceiling fires and names
itself, and a failing sample stops the measurement. `TestTheBriefOpensNoSocket`
skipped, as `internal/perf/NETWORK.md`'s table says it does on darwin.

**The binding perf verdict is this branch's CI run**, on both legs, against
`budget.go` unmodified.

---

## 11. What T6 should record

1. **The attribution** — section 3's seven phases with their reconciliation, and
   the finding that the cold build is 75% of in-process work and the only phase
   that scales with the ledger.
2. **The one optimisation and its argument** — that a listing's hashes exist only
   to be compared against rows, so an empty cache has nothing for them to answer;
   with the alternative of leaving it (`why_not`: it is a second read of the whole
   ledger on the one path where the cache costs more than it saves) and of
   memoising the parsed entries for the render (`why_not`: section 7).
3. **The refusals** — daemon, resident process, warm-up, network cache — recorded
   as refusals rather than as things nobody thought of.
4. **`dec-0026`'s "roughly 95ms" is not supported by any CI log**, and the measured
   distribution in section 6 supports its conclusion more strongly than the figure
   it cited. A correction, not a reversal.
5. **The distribution `dec-0026`'s `revisit_if` asked for now exists**, and its
   answer is that a percentile ceiling still cannot be derived — macOS's pooled
   p90 is above the budget it would enforce, and the variance is between runs
   rather than within them. The discriminator the data supports is the **minimum
   against the ceiling**, as a "can this machine judge?" test, in the shape
   `internal/index/latency_test.go` and `internal/ledger/fixture/latency_test.go`
   already use.
6. **`int-0002` is met and is not being superseded.** No `supersedes` edge. The
   budget's number is unchanged and this task did not need it changed.
