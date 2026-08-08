# E1-L2 — the write path: `dira log` writes exactly one file per mutation

**Status:** all four `acc:` clauses green. Tree left dirty and uncommitted, as instructed.

---

## 1. The surface, and the one design decision the lane brief asked for

`dira log` has two halves because dec-0003 names two callers.

```
dira log --kind KIND --title TEXT [flags]      a person, typing
dira log --stdin                               an agent, with a complete entry
dira log ID --edge TYPE=TARGET                 adding an edge to what exists
```

**The agent's interface is the entry file itself, on stdin.** The prompt says "an
agent assembling a nested structure on a command line is a design decision; state
it", so: it is not one dira asks for. Alternatives, `why_not`s and excerpts are
prose. They contain quotes, colons, newlines and equals signs, and every scheme
that encodes them into argv is a quoting scheme the prose eventually breaks — in
the one artifact that is supposed to outlive the tool. So `--stdin` takes the
same frontmatter-plus-markdown document dira stores, minus the `id`, which dira
allocates.

Three things fall out of that choice, and they are the reason for it:

- **No second wire format.** A JSON input would be a parallel expression of
  `entry.schema.json` to keep in step with it forever. There is one format, and
  the model writing it can already read `.dira/entries/*.md`.
- **What the author wrote is what lands.** The document is parsed by E1-L1's
  codec, so the style memo captures the caller's own wrapping and quoting. The
  file in the repository is byte-for-byte the file the agent composed, plus two
  lines. `TestLogReadsACompleteEntryFromStdin` asserts the hand-wrapped lines
  survive verbatim.
- **One validator.** The flag layer reports *syntax* (`--edge` that is not
  `TYPE=TARGET`) and nothing else. Every question of vocabulary and shape is
  answered by `Entry.Validate`, so `--kind task` is refused in cst-0002's own
  words rather than by a second copy of the rule in the CLI.

The flags cover the same fields for the entry small enough to type. The two
nested ones are built **positionally** rather than through a delimiter syntax:
`--alternative` opens one and `--why-not` / `--revisit-if` fill in the one before
them; `--edge TYPE=TARGET` with `--edge-note` after it. `flag` calls `Set` in
argument order, so this needs no parsing at all and cannot be broken by prose.
`--why-not` with no `--alternative` in front of it is a usage error naming the
problem.

`=` and not `:` for edges, because a namespaced ref carries a colon
(`sire:int-0002`) and no target carries an equals sign.

---

## 2. Id allocation

`internal/ledger/write.go`:

```go
func Add(ctx context.Context, s Store, e *Entry) error
```

One `List` to propose, `Store.Create` to dispose. The list is a starting point,
never a source of truth: correctness comes from `Create` alone, which is
exclusive, so a losing racer gets `ErrExists`, learns the id is taken, and steps
to the next unused candidate. Every candidate has exactly one winner, so N racers
take N distinct ids after at most N-1 collisions each, and a stale list costs a
retry rather than an entry.

There is **no lock anywhere in it**, which is the property that matters for E7:
the same allocator works over a backend where no lock exists. E1-L1 was right
that `Create` is what makes this reachable — without it this lane could not have
been built without adding a method to the interface.

"Lowest unused, per kind" means a gap gets filled: a ledger holding `dec-0001`
and `dec-0003` allocates `dec-0002`. That is the clause a `max()+1` allocator
silently fails, and it has its own test on both backends.

The retry budget is a constant (4096) rather than unbounded. Exhausting it is
reported as contention, not as a mystery.

---

## 3. Mutation is deliberately not disposition

`dira log <id>` adds **edges and tags**, and nothing else. It bumps `updated`
because the schema says that field is bumped on any field change, and that is the
only field it sets on the caller's behalf.

It cannot change `state`, `title` or the body, and refuses those flags by name
with a message pointing at where that work belongs. Accepting a staged entry and
superseding a decision are E2's disposition flow; a `--state` flag here would be
a second way to spell it that E2 would then have to reconcile with. **If E2 wants
`dira log --state` to be its mechanism, this is the decision to reverse, and it
is one line in `mutableFlags`.**

Two smaller calls inside it:

- **An edge that is already there is a no-op, exit 0, nothing written.** `dira
  log` runs unattended from hooks that fire more than once for the same
  conclusion. A command that fails the second time it is right is a command that
  gets taken out of the hook. Asserted: the second invocation changes zero paths.
- **The same edge with a *different* note is an error.** One of the two notes is
  a human's sentence about why the edge exists, and neither silently keeping nor
  silently replacing it is defensible without knowing which.

---

## 4. Defaults, each of which is a claim

| field | default | why |
|---|---|---|
| `state` | the first state valid for the kind — `active`, `accepted`, `open`, `active`, `active` | `Kind.States()` is in schema order and that order runs from the state an entry is born in to the states it ends in. `ledgertest.Entry` had already made the same assumption independently. |
| `source.hook` | `manual` | An invocation that says nothing about where it came from came from someone running the command. A hook passes `--hook Stop`. |
| `source.tier` | **none** | tier is a claim about *how* the content was extracted, and only the caller knows. Defaulting to `human` would let an agent's inference wear a human's confidence, which is the one thing the provenance block exists to prevent. |
| `confirmed_by` | none | disposition is E2's. |

`created` is stamped by the command, never by `internal/ledger` — the clock is
the caller's, so a hook and a test agree on what time it is. `Add` refuses an
entry with no `created` rather than inventing one.

**stdout is the allocated id and nothing else**, so `id=$(dira log …)` works and
a hook can act on what it just wrote. Anything conversational (the no-op notice)
goes to stderr.

---

## 5. `acc:` clause by clause

**(a) One new file, schema-valid, lowest unused id — GREEN.**
`TestLogWritesExactlyOneFile` runs the literal case: a decision with two
alternatives (one carrying `revisit_if`) and a `derives_from` edge. It hashes
every file under the ledger root before and after and asserts the modified-path
set is exactly `[.dira/entries/dec-0001.md]` — so a cache, a lock file or a stray
temporary appearing anywhere fails it. The written file goes through
`schema.NewValidator()`, the published contract rather than dira's reading of it.
`TestLogAllocatesTheLowestUnusedIDForTheKind` covers the gap and the per-kind
cases through the command; `TestAddAllocatesTheLowestUnusedID` covers six cases
against both backends.

**(b) 32 concurrent invocations, 32 distinct ids, 32 files — GREEN.**
`TestThirtyTwoConcurrentInvocationsProduceThirtyTwoEntries` builds the binary and
runs 32 real processes released from one gate. It asserts 32 distinct ids, that
they are exactly `dec-0001`…`dec-0032` (a skipped number would mean a candidate
was abandoned), 33 entry files including the seed, nothing but entry files in the
directory, the pre-existing entry byte-identical, and every file decoding and
passing the schema validator.
`TestThirtyTwoConcurrentAddsProduceThirtyTwoDistinctIDs` is the in-process form,
against both backends, and it is the one that runs under `-race`.
`TestAddRetriesTheNextIDWhenOneIsTakenUnderneathIt` is the race in slow motion:
two candidates are stolen inside `Create`, between the `List` and the write, and
it asserts the id lands on `dec-0003`, that `Create` was called exactly three
times, and that neither winner was clobbered — because a concurrency test that
passes for the wrong reason looks exactly like one that passes.

**(c) Adding an edge changes exactly one path — GREEN, and more than asserted.**
`TestAddingAnEdgeChangesOneFileAndLeavesEveryOtherLineAlone` runs against this
repository's real `dec-0002` **and** `dec-0005`, and asserts three things: the
modified-path set has cardinality one; every original line survives in its
original order (a greedy scan, which names the line if one goes missing); and
exactly four lines were added — the edge's three plus `updated`.

**(d) An injected failure mid-write leaves the pre-write state byte-identical —
GREEN.** Three angles, described in §7.

**cst-0002 — GREEN.** A sixth kind is refused with `kind "task" is not one of the
five (cst-0002 closes the set: intent, decision, question, constraint, note)`,
exit 2, ledger byte-identical.

**Every write goes through the interface — GREEN.** No `os.WriteFile` above the
backend; `cmd/dira` imports `os` only, and E1-L1's `TestNoFilesystemImportsAboveTheBackend`
still passes unchanged. `openLedger` returns `ledger.Store`, not `*local.Store`,
so it is the single place in the binary that picks an implementation and E7 adds
a case there and nowhere else.

---

## 6. The thing you should check me on first

**dec-0002 is a bad fixture for the diff-preservation property, and the brief
pointed me at it.**

The brief says adding one edge to `dec-0002` must alter exactly 3 lines with
every original line surviving, and that there is a test asserting it. Both true.
But I disabled the style memo and re-ran E1-L1's `TestRoundTripIsByteIdentical`
to find out which entries actually depend on it:

```
cst-0001  dec-0001  dec-0003  dec-0004  dec-0005  dec-0006  dec-0007
dec-0008  dec-0009  dec-0010  dec-0011  dec-0012  dec-0013  dec-0014
dec-0015  dec-0016  dec-0017  dec-0018  qst-0003
```

**`dec-0002` is not in that list.** Its wrapping is one a canonical emitter
happens to reproduce, so a command-level test written only against `dec-0002`
passes just as happily with the style memo deleted — I confirmed that, and it is
why the test is now table-driven over `dec-0002` (the entry the property was
bought by) and `dec-0005` (the entry that notices). With the memo disabled,
`dec-0002` passes and `dec-0005` fails on
`"      The query and mutation paths would grow filesystem assumptions everywhere —"`.

E1-L1's own test is unaffected — it covers all 26 entries — but anyone writing a
*new* test for this property against `dec-0002` alone would be writing a test
that cannot fail.

---

## 7. Crash safety, and what each test can actually prove

The acceptance line says "an injected failure mid-write". I built three checks
and measured which of them can discriminate, because a test that cannot fail for
the reason it exists is decoration.

**1. A failure in the middle of the write — deterministic, and it bites.**
`TestAFailureInTheMiddleOfTheWriteLeavesThePreWriteStateByteIdentical` runs the
real binary under a 1KB file-size limit (`ulimit -f 2`) with a half-megabyte
body. The write to the backend's temporary file fails a kilobyte in — a real
short write, reported by the kernel, after the entry was validated and encoded,
at a point no application-level check could have caught. Result: exit 1, stdout
empty, the entries directory holding only what it held before, byte for byte, and
nothing left behind.

**2. A concurrent reader — the atomicity check with teeth.**
`TestAConcurrentReaderNeverSeesAHalfWrittenEntry` polls the entry path with
`os.Stat` in a tight loop while a `dira log` with an 8.6MB body runs, and asserts
every size it ever observes equals the finished file's size. On a good run the
reader looks ~30,000 times and sees the file ~750 times, always whole.

Note the trap this test walks around: **`ledger.Decode` accepts a truncated
entry.** The body is trailing text, so an entry cut off inside its prose is a
structurally valid entry with prose missing. My first version of this check
compared decodability and passed against a deliberately broken backend. It
compares bytes now, and so does the kill sweep.

**3. A real crash — honest, but it cannot discriminate.**
`TestKillingDiraLeavesNoPartialEntry` kills 24 invocations at delays swept across
the command's lifetime, checking after every one that the new entry is absent or
whole, that the pre-existing entry is byte-identical, and that nothing looking
like an entry file was left behind.

It does not prove atomicity, and I am saying so rather than letting the name
imply it. I replaced the temp-file-and-link with a write straight to the entry
path and **this test still passed**, three times, at body sizes up to 8.6MB. The
write is a single syscall a signal does not interrupt, against a process that
spends ~90ms of its ~95ms starting up. The same substitution turns checks 1 and 2
red on every run.

**Red-before-green, all verified by breaking the code and restoring it:**

| break | result |
|---|---|
| allocator uses `max()+1` instead of lowest unused | `FAIL` — *allocated id = "dec-0004", want "dec-0002"* (both backends, and through the command) |
| allocator returns `ErrExists` instead of retrying | `FAIL` — *17 distinct ids from 32 writers* (memory), *8 from 32* (local), and 5 of 32 subprocesses exit 1 |
| `Encode` ignores the style memo | `FAIL` — `dec-0005` loses a hand-wrapped line; `TestLogReadsACompleteEntryFromStdin` reports the reflow |
| `local.write` writes straight to the entry path | `FAIL` — the reader sees the file at 0 bytes against a finished 8,640,150; the ulimit case leaves a partial `note-0001.md` |

---

## 8. Verbatim command output

```
$ gofmt -l .
  exit=0

$ go build ./...
  exit=0

$ go vet ./...
  exit=0

$ golangci-lint run ./...
0 issues.
  exit=0

$ go test -count=1 ./...
ok  	github.com/kazi-org/dira/cmd/dira	18.548s
ok  	github.com/kazi-org/dira/internal/index	16.398s
?   	github.com/kazi-org/dira/internal/index/indextest	[no test files]
ok  	github.com/kazi-org/dira/internal/ledger	3.813s
ok  	github.com/kazi-org/dira/internal/ledger/fixture	2.851s
?   	github.com/kazi-org/dira/internal/ledger/ledgertest	[no test files]
ok  	github.com/kazi-org/dira/internal/ledger/local	1.583s
ok  	github.com/kazi-org/dira/schema	0.460s
  exit=0

$ go test -race -count=1 ./...
ok  	github.com/kazi-org/dira/cmd/dira	21.647s
ok  	github.com/kazi-org/dira/internal/index	38.173s
ok  	github.com/kazi-org/dira/internal/ledger	5.932s
ok  	github.com/kazi-org/dira/internal/ledger/fixture	5.421s
ok  	github.com/kazi-org/dira/internal/ledger/local	2.504s
ok  	github.com/kazi-org/dira/schema	1.973s
  exit=0

$ python3 scripts/coverage.py
obligations extracted : 65
registered            : 65
uncovered             : 0
invalid disposition   : 0
orphaned register rows: 0
unverified dispositions: 0
untracked sources      : 0
COVERAGE PASS — every obligation has a disposition.
  exit=0

$ python3 scripts/privacy-lint.py
  ok   [P1] no private:true entries in 30 entries of a public ledger
  ok   [P2] no private parent declares a label — nothing to leak
  ok   [P3] every namespaced edge target resolves to a declared parent namespace
  ok   [P4] no mirrored ADRs exist yet — nothing to check
PRIVACY LINT PASS — cst-0003 enforced by 4 checks.
  exit=0
```

Executed counts rather than a bare `ok`:

```
$ go test -count=1 -v ./cmd/dira/ | grep -cE '^--- PASS'                42
$ go test -count=1 -v ./cmd/dira/ | grep -cE '^( *)--- PASS'            78
$ go test -count=1 -v ./cmd/dira/ | grep -cE '^( *)--- (FAIL|SKIP)'      0

$ go test -count=1 -v ./internal/ledger/... | grep -cE '^--- PASS'       56
$ go test -count=1 -v ./internal/ledger/... | grep -cE '^( *)--- PASS'  402
$ go test -count=1 -v ./internal/ledger/... | grep -cE '^( *)--- (FAIL|SKIP)'  0
```

42 top-level tests and 78 assertions in `cmd/dira` (was 24 / 42 before this
lane); 56 and 402 in `internal/ledger/...` (was 36 / 286). Zero failures, zero
skips, full suite green three consecutive times and `-race` green twice.

---

## 9. Two tests I had to make robust, and why it matters

Both process-level tests passed individually and then **failed on the first full
`go test ./...` run** — under the load of the whole suite running in parallel,
their vacuity guards, not their invariants, went red:

- the kill sweep required at least one killed round to have completed, and on a
  loaded machine all 24 were killed;
- the reader required at least one sighting of the file, and a starved poller got
  none.

Both guards were assertions about *timing*, which is a fact about machine load
rather than about the code under test. They are now assertions about *effort*: a
control invocation at the end of the sweep proves the harness would have noticed
a broken entry, and the reader asserts it looked at least 1,000 times rather than
that it found anything. The invariants themselves are unchanged and still bite.

Flagging it because the same shape is easy to reproduce: **a vacuity guard phrased
as a timing assertion is a flaky test**, and a flaky test in this repo's gate is
worse than the vacuity it was guarding against.

---

## 10. Changes outside this lane's own files

- **`cmd/dira/main.go`** — registered `log`; added `stdin` and a `now` clock to
  `app` (a test that cannot say what time it is can only assert that `created`
  looks vaguely like a timestamp); added an optional `usage` renderer to
  `command` so `dira help log` and `dira log -h` print the same text. No existing
  test changed.
- **`cmd/dira/reindex.go`** — 16 lines of ledger discovery replaced by a call to
  the shared `openLedger`. `-C` must not come to mean two slightly different
  things in two commands.
- **`internal/ledger/decode.go`** — `Decode` split into `decodeEntry` + the
  existing validation, so `DecodeDraft` applies the draft rules to the same parse
  rather than to a second one. `Decode`'s behaviour is unchanged.
- **`.gitignore`** — added `.dira/entries/.dira-*.tmp`. See §11.
- **`cmd/dira/build_test.go`** — **not touched, deliberately.** The brief said I
  would need to add to `allowedModules`; I do not. `dira log` links
  `internal/ledger` and `internal/ledger/local`, both of which E1-L3 already
  linked through `dira reindex`, so the module set is unchanged and
  `TestTheAllowlistIsNotStale` would have failed on a pre-added entry. E1-L1's
  hand-off note ("E1-L2 adds `gopkg.in/yaml.v3` when it wires the first command")
  was overtaken by E1-L3 landing first.

I did not touch `.dira/entries/` (the real entries are read as test fixtures and
copied into temp ledgers; none is modified), `docs/roadmap.md`,
`docs/coverage.md`, `docs/design/**`, `assets/**`, or any `scripts/*`, and I
created nothing under `internal/enforcer/`.

---

## 11. The one change you may want to drop

**`.gitignore` now ignores `.dira/entries/.dira-*.tmp`.**

A killed process cannot run a deferred cleanup, so the one thing a crash can
leave behind is the backend's scratch file, in `.dira/entries/`. It is not an
entry — `List` skips it, no id is consumed, and
`TestALeftoverTemporaryFileIsNotAnEntry` asserts a planted one changes nothing
about the next allocation — but it is untracked, in a directory that is committed,
and one `git add -A` from being in the repository forever.

Three ways to close that, and I took the cheapest: ignore the pattern. The others
are moving the temporary out of `entries/` (relocates the litter rather than
removing it, and changes E1-L1's backend) or having dira sweep stale temporaries
(a garbage collector nobody asked for, which could race a concurrent writer's
temporary file). If you would rather this were a backend change or nothing at
all, it is three lines in `.gitignore`.

---

## 12. What I did not build, and why

- **The ADR mirror (dec-0009).** Not this lane; the trigger is acceptance, which
  is E2's disposition flow. Not absorbed silently, and I do not think it belongs
  here: `dira log` would have to write two files, and a lane whose whole point is
  one file per mutation is the wrong place to introduce the exception.
- **Disposition** — accepting, superseding, state changes of any kind. E2.
- **`dira init`, `dira sniff`, hook installation.** Still unowned; `dira log`
  reports `no .dira directory in …` and exits 1 rather than seeding one, which
  matches `local.Open`'s deliberate refusal to create anything.
- **Removing an edge or a tag.** Nothing asked for it and a ledger is a record;
  removal wants a decision about whether history is editable at all.
- **`--body-file`.** Reading a file by path in the command layer would be the
  first `path/filepath` above the backend. `--body -` covers the case.
- **Any cache interaction.** `dira log` never opens `.dira/cache/`; the read path
  reconciles against the files, and a write that also updated the cache would be
  a second path changed, which is the thing the acceptance line forbids.

## 13. Decisions here that deserve `.dira/` entries

Not written, because the brief forbids touching `.dira/entries/`. In rough
priority:

1. **The agent's input to `dira log` is the entry document on stdin** (§1) —
   alternatives: a flag per field with a delimiter syntax for the nested ones;
   JSON on stdin. This one is load-bearing for E2's skill and E9's docs.
2. **`dira log <id>` adds edges and tags and cannot disposition** (§3) — the
   boundary between this lane and E2, and the thing most likely to be
   relitigated.
3. **A duplicate edge is a no-op, a conflicting note is an error** (§3).
4. **`source.tier` has no default while `source.hook` defaults to `manual`** (§4)
   — a provenance rule, and the kind of thing that gets "simplified" later.
5. **Schema-invalid input exits 2, not 1** — it is a caller mistake, and E2's
   hooks need "dira told me no" to be distinguishable from "dira is broken".
