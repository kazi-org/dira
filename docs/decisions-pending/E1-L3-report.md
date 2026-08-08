# E1-L3 — the derived SQLite cache, `dira reindex`, and the version decision

**Status:** every `acc:` clause green. Tree left dirty and uncommitted.
The coverage gate is red, for exactly the expected reason (§9).

---

## 1. The decision E1-L1 handed me, and why the evidence reversed it

**`EntryInfo.Version` is now the entry file's git blob object id.** Recorded as
`.dira/entries/dec-0015.md` with five rejected alternatives and non-empty
`why_not`s.

E1-L1 handed this over with an estimate that a content hash would cost **~13ms
per full scan**. That number is wrong, and it is wrong in the direction that
decides the question. Measured over the 200-entry fixture on this machine:

| operation | median |
|---|---|
| `readdir` + `stat` × 200 (the floor) | 2.02ms |
| `List` with mtime+size — **E1-L1's version** | 2.15ms |
| `readdir` + read + hash × 200 — **the content hash** | 6.93ms |
| full read: `List` + `Get` × 200 (read *and* parse) | 22.0ms |

The marginal cost of hashing is **4.7ms**, not 13ms. The 13ms estimate had
folded in the YAML parse: the 200 entry files total 241KB, so reading and
hashing them is I/O plus a hash, and the expensive part of a full read is
`yaml.v3`, not the disk.

At 4.7ms against a 40ms budget, the heuristic is not worth defending, because
its hole is reachable rather than theoretical:

- `state: accepted` and `state: rejected` are the same eight characters. **The
  edit that reverses a decision is size-preserving.**
- Any restore that preserves modification times — `rsync -a`, `cp -p`, `tar -p`,
  a backup unpacked over a checkout — preserves mtime.
- Together those make the reversal invisible to `mtime+size`, permanently, with
  no reindex able to notice. That is dec-0002's one unsurvivable failure.

**Red-before-green, verified by reverting rather than asserted.** I put
E1-L1's `version(info fs.FileInfo)` back, along with the stat-based `List` and
`Get` call sites, and ran the acceptance test:

```
$ go test -count=1 -run TestAnEditBehindTheCacheResolvesToTheFile -v ./internal/index/
    index_test.go:197: dec-0001 is still selected as accepted after the file was changed to rejected.
    index_test.go:206: dec-0001 is not selected as rejected after the file was changed to rejected
--- FAIL: TestAnEditBehindTheCacheResolvesToTheFile
    --- PASS: TestAnEditBehindTheCacheResolvesToTheFile/an_ordinary_edit
    --- FAIL: TestAnEditBehindTheCacheResolvesToTheFile/an_edit_preserving_both_size_and_modification_time
```

Restored, and green.

**Why the git blob form rather than a bare sha256.** Two things it buys at
identical cost, and both are load-bearing:

1. E7's github backend gets blob shas free from the Contents API, so the two
   backends produce **equal** versions for equal content — a ledger can change
   backend without a full reindex. `mtime+size` has no counterpart on GitHub at
   all, so tuning to it tunes to a local-only property, which is the retrofit
   dec-0005 exists to prevent.
2. `git hash-object .dira/entries/dec-0002.md` reproduces dira's version with no
   dira installed.

Both are *checked*, not claimed: `internal/ledger/local/version_test.go`
shells out to `git hash-object` over three entries (one with a multi-paragraph
body, one with multibyte unicode, one with no body) and asserts `List` and `Get`
both agree with it.

sha1's collision weakness is out of this threat model and dec-0015 says so: this
detects accidental divergence, not forgery, and anyone who can write the entry
file can write whatever they like into it.

**Rejected, with the measurement that rejected it:**

- **ctime + inode.** I measured it working — after a size-preserving edit with
  `Chtimes` restoring mtime, ctime had moved by ~1s while size, mtime and inode
  were all bit-identical — and it is free, because the stat already happens. It
  still loses: it is a claim about when the inode was touched, not about what the
  file holds; it is absent on Windows and needs build-tagged `syscall.Stat_t`
  elsewhere. More code for a weaker guarantee than the 4.7ms option. Recorded as
  the right *pre-filter* if a very large ledger ever makes the scan the binding
  cost.
- **Bare sha256** — same cost, strictly less use (see above).

---

## 2. Does SQLite earn its place at 200 entries? Measured: yes warm, no cold

The lane brief invited me to say if it does not. The honest answer is
**it does, but by less than the brief assumed, and it makes one case worse.**

Over the 200-entry fixture, in process, spawn excluded
(`TestTheCacheBeatsReadingTheFiles`, which fails if warm ever stops beating
no-cache):

| | median |
|---|---|
| **no cache** — list, read and parse all 200 | **30.1ms** |
| **cache warm** — hash scan, open, reconcile finding nothing, render the brief | **15.1ms** |
| **cache cold** — the same, plus building the database | **55.5ms** |
| `dira reindex` over 200 entries (`BenchmarkReindex`) | 51.1ms |
| warm open + one selection, no rendering (`BenchmarkWarmBrief`) | 10.1ms |

**The brief's bar was "beat 28.3ms cold-read". Warm does (15.1ms). Cold does
not (55.5ms).** The first invocation after `rm -rf .dira/cache` costs roughly
twice what having no cache at all would cost, and it breaks even on the second
invocation. Since hooks fire several times a session, the steady state is the
one that matters — but E1-L6 needs this number, see §10.

The binary cost, measured by building both:

| | size |
|---|---|
| `dira` before this lane (stdlib only) | 2.76MB |
| + the ledger codec (yaml.v3, and jsonschema + x/text via `schema`) | 5.27MB |
| + `modernc.org/sqlite` | **11.79MB** |

So SQLite is **+6.52MB** and the codec chain is **+2.51MB**. None of it costs
start-up time, measured two ways: process entry to `main` is 3µs with and
without, and the median wall clock of `dira version` over 12 runs is 0.10s for
the 11.79MB binary against 0.11s for the 2.76MB one — indistinguishable, because
process spawn alone (`/usr/bin/true`) is 0.06s on this machine and swamps it.

**What decides it in favour of keeping SQLite:** the part the cache removes is
the YAML parse of every entry — 15ms of the 30.1 at 200 entries, growing
linearly — while the hash scan it cannot remove is only 6.9ms. The saving scales
with the ledger; the 6.52MB does not. `dec-0002` and the L0 acceptance line both
name SQLite, so dropping it would need a superseding entry; dec-0015 records the
measurement that would justify one if the binary size ever becomes a real
complaint.

**cgo was not needed.** `modernc.org/sqlite` (pure Go) is used, so goreleaser
can still cross-compile darwin-arm64 and linux-amd64 from one runner. The
measured database work is 2.1ms of the 15.1ms warm read, so SQLite's own speed
is not the binding constraint and there is nothing cgo would buy.

---

## 3. The query API — the contract `E1-L4` and `E1-L5` are blocked on

`internal/index`. Six methods and three types. `Open` reconciles before it
returns, so an `*Index` that exists is one whose every row already agrees with
the files.

```go
func Open(ctx context.Context, store ledger.Store, cacheDir string) (*Index, error)
func OpenFresh(ctx context.Context, store ledger.Store, cacheDir string) (*Index, error)
func Path(cacheDir string) string

func (ix *Index) Close() error
func (ix *Index) Notice() string   // "" when the cache is doing its job
func (ix *Index) Stats() Stats

func (ix *Index) Select(ctx context.Context, sel Selector) ([]Ref, error)
func (ix *Index) In(ctx context.Context, id string) ([]Backlink, error)
func (ix *Index) Resolve(ctx context.Context, term string) ([]string, error)
func (ix *Index) Entry(ctx context.Context, id string) (*ledger.Entry, error)
func (ix *Index) Entries(ctx context.Context, ids []string) ([]*ledger.Entry, error)

type Ref struct{ ID string; Kind ledger.Kind; State ledger.State; Title, Created, Updated string; Private bool }
type Backlink struct{ From string; Type ledger.EdgeType; Note string }
type Selector struct{ Kinds []ledger.Kind; States []ledger.State; WithEdge ledger.EdgeType; Limit int }
```

**What each is for, and what you get:**

- **`Select`** — the only listing verb. Fields are conjunctive; a zero `Selector`
  matches everything. `WithEdge` selects entries declaring at least one outgoing
  edge of that type, which is what makes cst-0001's *open blockers* — open
  questions carrying a `blocks` edge — a query rather than a post-filter over 200
  parsed files. **Order is `created DESC, id ASC` and it is total**, so `Limit`
  takes a reproducible prefix and E1-L5 can drop from the low-priority end
  deterministically.
- **`In`** — incoming edges. Outgoing edges are already on the entry
  (`Entry(id).Edges`, dec-0002 puts edges on the subject), so there is
  deliberately no `Out`. `In` is what a why-chain walks: the decisions deriving
  from an intent, the questions blocking a decision, the entry that superseded
  this one. An id nothing points at yields an empty slice, not an error.
- **`Resolve`** — what a human typed → ids. An exact id that exists resolves to
  itself *alone*, so `dira why dec-0002` is never ambiguous; anything else is a
  case-insensitive substring of a title or a **whole** tag (`latency` matches the
  tag, `atenc` does not), newest first. This is what makes `dira why daemon` land
  on the same entry as `dira why int-0002`. No match is an empty slice and no
  error — whether that is a failure is E1-L4's call and an empty section is
  E1-L5's.
- **`Entry` / `Entries`** — **read the file, never the cache.** This is the
  files-win property expressed as an API rather than a convention: no rendering
  path exists through which a cached value could reach a human.
  `TestEntryReadsTheFileNotTheCache` poisons a cached title in place, keeping its
  version so no reconcile would notice, and asserts `Entry` still returns the
  file's. `Entries` preserves the order asked for; a missing id is an error.

**The two invariants you can build on:**

1. Nothing dira prints comes from the cache.
2. No query can run against an unreconciled cache — `Open` does it, and there is
   no method that reaches the database without that having happened. There is no
   fast path to skip.

**`internal/index/indextest` is yours to extend.** `RunTwice(t, diraDir,
extra ...Query)` runs every query warm and then again with `.dira/cache/` deleted
before *each* one, asserting byte-identical rendered output. Add your real
`dira why` / `dira brief` output as an `indextest.Query` and you inherit the
guarantee for free. It ships with 13 queries covering every method.

**What the cache holds** (`internal/index/schema.go`): id, version, kind, state,
title, created, updated, private, tags; and edges (src, seq, type, dst, note).
No body, no alternatives, no `why_not` — those are what dira prints. **No
execution status, no bucket, no kazi state** (dec-0004), asserted by
`TestTheCacheHoldsNoExecutionState`, which greps the schema text out of the
database file itself.

---

## 4. `acc:` clause by clause

The lane's acceptance line has five clauses. All green.

**(1) The full read-path suite runs twice — cache-warm and with `.dira/cache/`
deleted before every query — byte-identical output for every query. GREEN.**
`TestTheSameAnswersComeBackWithAndWithoutACache` over the 200-entry fixture, via
`indextest.RunTwice`: 13 queries covering `Select` (six selector shapes),
`Resolve` (four), `In`, a why-chain render and a whole-ledger render. The cold
run deletes the cache before *each* query, so a query that only worked because an
earlier one warmed something is caught too, and it fails if a cold open reads 0
files (i.e. if the deletion did not take).
`TestTheHarnessWouldNoticeADifference` is the other half: it feeds `RunTwice` a
planted query that answers differently each call and asserts the harness fails,
and asserts the plant ran exactly twice.

**(2) `dira reindex` after deleting `.dira/cache/` reproduces a byte-identical
cache-derived query result. GREEN.**
`TestReindexFromNothingReproducesTheSameIndex` compares both the rendered
answers *and* every row of the rebuilt database against the one it replaced.
Rows, not a checksum of the file: a SQLite file is not byte-stable across
rebuilds (page allocation, freelist, WAL), so hashing it would assert on
SQLite's internals rather than dira's — comparing rows is the stronger claim
anyway. `TestReindexRebuildsAfterTheCacheIsDeleted` is the same thing through
the command. `TestReindexIsIdempotent` stops it passing by doing nothing.

**(3) Editing an entry file so it disagrees with an existing cache makes every
query return the file's value, without a manual reindex. GREEN.**
`TestAnEditBehindTheCacheResolvesToTheFile`, two subtests: an ordinary edit, and
the size-and-mtime-preserving edit that is red under E1-L1's version (§1). Both
assert the entry, the selection that should now exclude it, and the selection
that should now include it. Plus
`TestAnEntryAddedOrRemovedBehindTheCacheIsNoticed`,
`TestEntryReadsTheFileNotTheCache`,
`TestAFreshOpenRepairsACacheTheVersionCheckCannotSee`, and at the backend
`TestVersionSeesASizeAndTimePreservingEdit`.

**(4) `git check-ignore .dira/cache/index.db` exits 0 in a test. GREEN.**
`TestTheCacheIsGitignored` asserts on `index.Path(local.CacheDir(".dira"))` —
the path the code actually constructs, not a copy of the string — and then
asserts git does **not** ignore `.dira/entries/dec-0002.md`, because a rule that
ignores everything would pass the first check without meaning anything.
Verified live: `.gitignore:4:.dira/cache/ .dira/cache/index.db`.

**(5) A read-only or unwritable `.dira/cache/` degrades to direct-from-files
answers with a stated notice and exit 0, never an error. GREEN.**
`TestAnUnusableCacheDirectoryDegradesRatherThanFailing`, four cases: read-only
parent, read-only cache directory, cache directory that is a file, and a
read-only cache directory that already holds a cache (the read-only checkout).
Each asserts `Open` returns no error, `Stats().Cached` is false, `Notice()` is
non-empty, and the degraded answers are identical to the cached ones.
`TestReindexOnAnUnwritableCacheExitsZero` is the same at the command: exit 0,
notice on stderr, `no cache was written` plus the real entry count on stdout.

**Degradation has one implementation, not two.** When the cache directory is
unusable the same schema is built in an in-memory SQLite database and the same
SQL runs over it. A cached answer and an uncached one cannot drift apart —
which is a stronger statement than the differential test that also checks it.

---

## 5. Verbatim command output

```
$ go build ./...
exit=0

$ go vet ./...
exit=0

$ golangci-lint run ./...
0 issues.
exit=0

$ go test -count=1 ./...
ok  	github.com/kazi-org/dira/cmd/dira	10.382s
ok  	github.com/kazi-org/dira/internal/index	10.684s
?   	github.com/kazi-org/dira/internal/index/indextest	[no test files]
ok  	github.com/kazi-org/dira/internal/ledger	3.078s
ok  	github.com/kazi-org/dira/internal/ledger/fixture	2.385s
?   	github.com/kazi-org/dira/internal/ledger/ledgertest	[no test files]
ok  	github.com/kazi-org/dira/internal/ledger/local	0.861s
ok  	github.com/kazi-org/dira/schema	0.209s
exit=0

$ go test -race -count=1 ./...
ok  	github.com/kazi-org/dira/cmd/dira	12.728s
ok  	github.com/kazi-org/dira/internal/index	30.293s
?   	github.com/kazi-org/dira/internal/index/indextest	[no test files]
ok  	github.com/kazi-org/dira/internal/ledger	5.182s
ok  	github.com/kazi-org/dira/internal/ledger/fixture	5.148s
?   	github.com/kazi-org/dira/internal/ledger/ledgertest	[no test files]
ok  	github.com/kazi-org/dira/internal/ledger/local	2.370s
ok  	github.com/kazi-org/dira/schema	1.901s
exit=0
```

Executed counts rather than a bare `ok`:

```
./internal/index/...         pass=40   fail/skip=0
./cmd/dira/                  pass=36   fail/skip=0
./internal/ledger/...        pass=292  fail/skip=0
```

```
$ python3 scripts/privacy-lint.py
  ok   [P1] no private:true entries in 27 entries of a public ledger
  ok   [P2] no private parent declares a label — nothing to leak
  ok   [P3] every namespaced edge target resolves to a declared parent namespace
  ok   [P4] no mirrored ADRs exist yet — nothing to check

PRIVACY LINT PASS — cst-0003 enforced by 4 checks.
exit=0
```

Verified live against this repo's own ledger, not just in tests:

```
$ rm -rf .dira/cache && dira reindex
indexed 27 entries and 45 edges from /…/dira/.dira into /…/dira/.dira/cache
$ dira reindex
indexed 27 entries and 45 edges from /…/dira/.dira into /…/dira/.dira/cache
$ (cd internal/index && dira reindex)
indexed 27 entries and 45 edges from /…/dira/.dira into /…/dira/.dira/cache
$ git check-ignore -v .dira/cache/index.db
.gitignore:4:.dira/cache/	.dira/cache/index.db
$ git status --short | grep -c '\.dira/cache'
0
```

---

## 6. Files

**New:**

| path | |
|---|---|
| `internal/index/index.go` | `Index`, `Open`, `OpenFresh`, degradation, cache lifecycle |
| `internal/index/schema.go` | the DDL and the schema version |
| `internal/index/sync.go` | the reconcile — the whole files-win mechanism |
| `internal/index/query.go` | `Select`, `In`, `Resolve`, `Entry`, `Entries` |
| `internal/index/indextest/indextest.go` | the dual-run harness E1-L4/L5 extend |
| `internal/index/{index,cachefile,latency}_test.go`, `race_{on,off}_test.go` | |
| `cmd/dira/reindex.go`, `cmd/dira/reindex_test.go` | the verb |
| `internal/ledger/local/version_test.go` | the `git hash-object` proof |
| `.dira/entries/dec-0015.md` | the decision |

**Modified — including three outside what the brief assigned me, each flagged
here rather than buried:**

- `cmd/dira/main.go` — one line, registering `reindex`. As authorised.
- `cmd/dira/build_test.go` — `allowedModules` filled in. As authorised. **Twelve
  entries, which is a loud diff and should be.** Each is annotated, and the
  block carries the measured binary and start-up cost the existing comment
  demands.
- `internal/ledger/local/local.go` — **E1-L1's file.** `version()` is the one
  function E1-L1 explicitly handed over. I also had to add `Find` (walk up for
  `.dira`, which no command could work without and which `cmd/dira` cannot do
  itself because it may not import `path/filepath`) and `CacheDir`. `List` now
  reads each file instead of stat-ing it, which is what the hash requires, and
  `Get`'s read was factored into a `read` helper it now shares.
- `internal/ledger/boundary_test.go` — **E1-L1's file.** Two allowlist entries,
  `internal/index` and `internal/index/indextest`, for `os` and `path/filepath`.
  Unavoidable: SQLite opens a file, and the cache is local even when the ledger
  is not (E7's github backend still caches on disk, because the alternative is a
  network call on the read path, which int-0002 forbids). Both entries carry
  their reason in the file.
- `go.mod` / `go.sum` — `modernc.org/sqlite` and its tree.

I did **not** touch `docs/roadmap.md`, `docs/coverage.md`, any gate script, or
any `.dira/entries/*` other than the one new `dec-0015.md`. I created nothing
under `internal/enforcer/`. Nothing is committed or pushed.

---

## 7. Design decisions inside the lane you should sanity-check

**The cache is an index, never a content store.** `Select` returns `Ref`s;
anything that renders goes through `Entry`, which reads the file. This is why
the schema has no bodies and no `why_not`s. It costs a file read per rendered
entry (~0.11ms) and buys an invariant that cannot be violated by a future
careless commit.

**Degradation is an in-memory SQLite database, not a second query path.** Costs
~9ms over the direct read; buys the guarantee that the two paths cannot diverge.
The alternative — hand-rolled in-memory filtering — would be faster and is
exactly the shape of code that drifts from the cached path and makes the
differential test the only thing standing between dira and a lie.

**A malformed entry file takes out one entry, not the ledger.** Its row is
dropped (serving a cached answer for a file dira could not read is the stale row
this package exists to prevent), it is skipped, and it is **named** in `Stats().Invalid`
and in `Notice()`. Silence there would be the same failure as staleness.
`TestAMalformedEntryIsSkippedAndReported`, `TestReindexReportsUnreadableEntries`.

**`dira reindex` exits 0 on an unwritable cache.** Its job is impossible and it
says so (`no cache was written`), but E2 installs dira in hooks, and a hook that
fails on a read-only checkout takes the session with it. It still reports the
entries it read, because it did genuinely read and validate them.

**`OpenFresh` rather than an `ix.Reindex()` method.** My first version had
`Reindex` as a method, which meant `dira reindex` read all 200 files twice —
once in `Open`'s reconcile and again after clearing. A second constructor makes
it one pass. `BenchmarkReindex` asserts it reads exactly 200 files, so the
regression cannot come back quietly.

---

## 8. Two things in the surrounding code that are wrong, neither mine to fix

**`fixture.Generate(seed, n)` does not return `n` entries for `n != 200`.** Its
doc says `scale` keeps "the total exactly n by giving the remainder to the
largest group"; `scale` does no such thing — it scales each group and clamps to
≥1, so `n=20` yields 19 and `n=40` yields 39. My `cmd/dira` tests assert against
`len(entries)` rather than `n` and say why. E1-L1's owner should either
implement the redistribution or correct the comment; right now the code and its
documentation disagree, which is how a later lane writes an off-by-one test that
looks like a real failure.

**`internal/ledger` imports `schema` for `SplitFrontmatter`, which drags a JSON
Schema compiler and `golang.org/x/text` into the release binary** — about 2.5MB
of the 11.79MB, for two small helper functions. Moving `SplitFrontmatter` out of
the package that also `//go:embed`s and compiles a schema document would take
roughly 2MB out of every download for no behaviour change. Noted in
`build_test.go` beside the allowlist entries it explains. Not mine: it is a
change to E1-L1's package boundary.

---

## 9. The coverage gate is red, as predicted

```
$ python3 scripts/coverage.py
obligations extracted : 60
registered            : 54
uncovered             : 6

UNCOVERED — no disposition registered in docs/coverage.md:
  impl:dec-0015
  trigger:dec-0015:089e15   (ledger past ~2,000 entries)
  trigger:dec-0015:bee40d   (hash scan becomes the binding cost)
  trigger:dec-0015:c4ac11   (version needs to authenticate, not detect change)
  trigger:dec-0015:3d524c   (binary size becomes a real complaint)
  trigger:dec-0015:e334b1   (modernc.org/sqlite's libc becomes a problem)

COVERAGE FAIL — something is unaccounted for.
exit=1
```

Six obligations, all from `dec-0015`: its implementation plus the five
`revisit_if` triggers. I was told not to edit `docs/coverage.md`, so I have not.
Registering them is a one-time edit by whoever owns that file. The
implementation obligation is in fact satisfied — it is this lane — so it should
register as done rather than as pending.

---

## 10. What E1-L6 needs to know before it starts

Three things, and the first is the one that matters.

**Process spawn on this machine is 60–90ms, which eats the entire budget.**
Measured with `/usr/bin/time -p`:

```
/usr/bin/true       real 0.06 – 0.07
dira version        real 0.08 – 0.10   (no ledger work at all)
dira reindex        real 0.10          (26 entries)
```

E1-L6's `acc:` is "the median of N≥20 cold runs must not exceed 100ms wall
clock". On this hardware `dira version` — which reads nothing — already sits at
0.10s. The budget is not obviously reachable end-to-end here regardless of
dira's own code, and the E1-L1 note about `/usr/bin/true` costing ~88ms was in
the right ballpark. E1-L6 will need to decide whether it is measuring dira or
measuring macOS process launch, and its stated CI multiplier needs to account
for a spawn floor that is larger than the budget's headroom. This is not a
finding I can act on inside this lane, but the lane will fail for the wrong
reason if it is discovered at the end.

**The cold-cache-before-every-run methodology measures a state that occurs once
per clone.** Cold is 55.5ms of dira's own work, warm is 15.1ms, and no-cache is
30.1ms. Deleting the cache before every run measures the one invocation where
the cache costs more than it saves, and reports it as the steady state. If the
budget is meant to describe what a hook actually pays, it needs both numbers.

**The two levers if the cold path needs to come down**, neither of which I took
because both reach outside this lane:

1. `List` reads every file's bytes to hash them, then the reconcile calls `Get`,
   which reads the same bytes again — about 7ms of duplicated I/O on the cold
   path. Removing it means a bulk-read method on `ledger.Store`, which is a
   change to dec-0005's interface and has to be weighed against E7.
2. The reconcile reads and decodes 200 files serially. Parallelising it across
   `GOMAXPROCS` is contained inside `internal/index` and is the cheaper of the
   two. E1-L6 is explicitly authorised to make it.

---

## 11. What in the brief turned out to be wrong

- **"`Version` … costs ~13ms per full scan as a content hash"** (from E1-L1's
  report, repeated in the lane brief). It costs 4.7ms marginal, 6.9ms absolute.
  The estimate had folded the YAML parse into the hash. This one number inverted
  the decision, so it is worth being explicit that it was checked rather than
  inherited.
- **"A cache that does not beat 28.3ms cold-read has not earned its existence."**
  Warm does (15.1ms). Cold does not (55.5ms). Stated as a single threshold, the
  bar is not answerable — the cache is 2× faster in the steady state and 1.8×
  slower on the first invocation after deletion, and both are real.
- **The baselines are slightly optimistic on this machine today** — `List`
  2.15ms rather than 2.7ms, full cold read 22.0–30.1ms rather than 28.3ms,
  depending on how warm the OS page cache is. Same ballpark; the ratios hold.
- **"Prefer a cgo-free SQLite driver"** — correct and taken, and it cost nothing
  measurable: the database work is 2.1ms of a 15.1ms warm read.
- **"`allowedModules` … add only what the command path actually links"** —
  correct, and it links twelve modules, not one. Nine of them arrive through
  `modernc.org/sqlite` and two through `internal/ledger`'s import of `schema`.
  The list was computed from `go list -deps`, not guessed.
- Everything else in the brief — the derived-cache framing, the gitignore as a
  privacy mechanism rather than housekeeping, the latency posture, and the
  instruction to make the dual-run harness extensible — was accurate and
  load-bearing.
