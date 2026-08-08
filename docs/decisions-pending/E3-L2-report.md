# E3-L2 report — `dira check` end to end over a single ledger

**Lane:** E3-L2, `docs/plan/lanes/E3.md`. **Status:** built, green, verified through a
real process. Not registered — see the wiring block first.

---

## 1. Wiring you own: what to add at merge

### 1.1 The registry line

In `newApp`'s command slice in `cmd/dira/main.go`, in help order (before `reindex`):

```go
{name: "check", summary: "refuse a plan that contradicts a settled decision", run: runCheck, usage: writeCheckUsage},
```

### 1.2 A five-line exit-code branch — **required, not optional**

This one is not cosmetic. `main.go`'s documented contract maps a *usage* error onto
exit **2**, and `docs/plan/lanes/E3.md` fixes exit **2** as "at least one cited
conflict". Those are the same number meaning two different things, and the whole point
of E3's exit codes is that a hook can tell "you contradicted yourself" from "dira is
broken". Without this branch a conflict returns 1 and a mistyped flag returns 2 —
exactly inverted.

Add to `(*app).main`, immediately after the `if err == nil { return exitOK }` return:

```go
	// A command that has already rendered its own verdict selects its exit
	// code directly, and nothing more is printed: the verdict is the output.
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
```

`errors` is already imported. Nothing else in the binary implements `ExitCode() int`,
so no existing command changes behaviour.

`dira check` handles its *own* misuse rather than raising a `usageError`, precisely so
it never lands on 2: `checkMisuse` in `cmd/dira/check.go` prints the message and the
command's own usage to stderr and selects 1. That is a deliberate local divergence from
the binary-wide convention, and it is documented in `runCheck`'s doc comment and in
`dira check -h`.

### 1.3 `allowedModules`

**No entry needed.** `go list -deps github.com/kazi-org/dira/cmd/dira` returns exactly
the fifteen modules already listed in `cmd/dira/build_test.go`; `internal/enforcer`
imports only stdlib and `internal/ledger`. Verified after the change.

### 1.4 One file outside my stated ownership

`schema/check.schema.json` — **new file**, required by the lane's `acc:` line ("`dira
check --json` validates against a committed `schema/check.schema.json`"). Nothing
existing in `schema/` was touched; `schema/entry.schema.json` is byte-identical
(`git diff --stat schema/entry.schema.json` is empty). Flagging it because `schema/`
was not on my list either way.

---

## 2. Measured results

**Corpus checksum verified unchanged, before and after:**

```
$ shasum -a 256 internal/enforcer/testdata/corpus.yaml
aa3a0245b107a3fc86073b63142536c3dd2d0779e804185489cf745618e09b8c
$ cat internal/enforcer/testdata/corpus.sha256
aa3a0245b107a3fc86073b63142536c3dd2d0779e804185489cf745618e09b8c
```

Not one byte of `corpus.yaml`, `corpus.sha256` or `testdata/ledgers/daemon/` changed.
`assertCorpusFrozen` runs first and fatally in every test that grades the matcher.

**Precision and recall at the shipped constants** (`matchThreshold = 0.38`,
`phraseShare = 0.82`):

| | |
|---|---|
| Recall | **91.7%** — 22 of 24 conflict rows, with the correct entry id cited |
| False positives | **0** of 19 compliant near-misses |
| Bar | ≥90% recall, zero false positives |

**The curve** (`TestPrecisionRecallCurve`, printed on every run, so the plateau claim is
re-measured rather than remembered):

```
threshold  recall   detected  false positives
   0.30    91.7%   22/24       4
   0.32    91.7%   22/24       1
   0.34    91.7%   22/24       0   <- plateau starts
   0.36    91.7%   22/24       0
   0.38    91.7%   22/24       0   <- shipped
   0.40    91.7%   22/24       0
   0.42    91.7%   22/24       0   <- plateau ends
   0.44    87.5%   21/24       0
   0.46    83.3%   20/24       0
   0.48    75.0%   18/24       0
```

The shipped value sits in the middle of a five-point plateau, not on its edge. The test
fails if the shipped threshold falls outside the viable range, **and** fails if only one
threshold in the sweep meets the bar — a single viable point is a fit to this corpus
rather than a working matcher, and that distinction should not depend on anyone
noticing it in a log.

**The two rows it misses, named rather than averaged away:**

- `row-013` "publish decisions to a paid hosted dashboard that everyone must sign into"
  → `cst-0004`. The only word it shares with the constraint is `hosted`. There is no
  lexical signal here; catching it needs to know a dashboard you sign into is a hosted
  tier.
- `row-018` "add a plugin API so third parties can extend dira with native modules at
  runtime" → `dec-0081`. Shares `plugin` with the alternative "a plugin architecture
  loaded via cgo or a scripting VM" and nothing else; `API`, `native modules` and
  `third parties` appear nowhere in the entry.

Both are the case dec-0014's own second rejected alternative anticipated (per-entry
trigger terms) and its first (agent-assist above the deterministic floor). Neither is
reachable by widening the current signal without buying false positives back — the
curve above shows what loosening costs.

**Executed assertion counts.** 169 passing tests and subtests across the two packages
(`go test -v ./internal/enforcer/ ./cmd/dira/`), of which:

| Test | What it pins |
|---|---|
| `TestCorpusWellFormed` + 7 subtests | E3-L1's prose spec, implemented verbatim: freeze, ≥40 rows, row shape, conflict-row shape with `why_not` substring presence, compliant-row shape, ≥8 distinct entries, ≥15 valid near-misses |
| `TestMatchesTheCorpus` | 43 rows graded; recall and false positives asserted separately |
| `TestPrecisionRecallCurve` | 31 threshold points; plateau membership and plateau width |
| `TestTheStagedDecisionIsCitedByNothing` | all 43 rows × the staged decision and the three never-enforced kinds |
| `TestTheDemoBlockIsByteForByte` | golden equality plus the five required lines individually |
| `TestTheCompliantPlanEmitsNoCross` | exit 0, no `✗`, non-zero enforced count |
| `TestRenderStatesAMissingRevisitCondition` | the "none recorded — supersede … to reopen" line |
| `TestRenderOffersNoRevisitWhereNoneCanExist` (2) | constraint and rejected-decision blocks contain no `revisit_if` at all |
| `TestPrivateEntriesAreCitedByRefOnly` | sentinel absent from human **and** JSON output, ref present in both |
| `TestJSONValidatesAgainstTheSchema` (4) | every conditional branch of `check.schema.json` exercised, with a coverage assertion so an unreached branch fails |
| `TestTheSchemaRejectsALeak` | the schema *rejects* a private entry cited by text |
| `TestTheMatcherCannotReachTheNetwork` | `net`, `net/http`, `crypto/tls`, `os/exec` unreachable from the enforcer's dependency graph |
| `TestCheckReadsThroughTheLedgerInterface` (4) | a conflict is not an error; an unreadable ledger is |
| `TestCheck*` in `cmd/dira` (12) | exit codes 0/1/2, golden through the command, `--json`, help, ledger left byte-identical |

**Gates.** `go build ./...`, `go vet ./...`, `golangci-lint run ./...` (0 issues),
`gofmt -l` (clean), `go test -count=1 ./...` and `go test -race -count=1 ./...` all
clean **except** one failure that is not this lane's — see §6.

**Verified through a real process**, not only in-process. I copied the tree to a
scratch directory, applied §1.1 and §1.2 there, built, and ran the binary:

```
$ dira check -C <fixture> "add a background daemon to track run state"   → exit 2, stdout == golden byte-for-byte
$ dira check -C <fixture> "write the checkpoint file atomically"         → exit 0
$ dira check -C <fixture> --nope "x"                                     → exit 1
$ dira check -C <empty dir> "add a background daemon"                    → exit 1
$ dira check -C <fixture> --json "add a background daemon ..."           → exit 2, exit_code: 2 in the document
```

The working tree was not modified to do this.

---

## 3. Things in the brief that turned out to be wrong

### 3.1 dec-0014's phrase rule inverts on the two rows that matter most

dec-0014 says an exact multiword phrase hit is an independent match signal, and gives
the worked example: this "makes `add a background daemon to track run state` catch
dec-0060's `a daemon` alternative trivially".

Read as raw strings, that is false, and false in the worst possible direction:

- The substring `"a daemon"` **does not occur** in `"add a background daemon to track
  run state"` — `background` sits between the words. The canonical demo row would not
  fire.
- The substring `"a daemon"` **does occur** in corpus row-039, `"note in the plan doc
  that a daemon was considered and rejected; …"` — a compliant near-miss. A
  raw-substring matcher fires on the sentence documenting the decision and stays silent
  on the sentence relitigating it.

I implemented the rule over **content words** rather than characters, which is the
reading under which dec-0014's own example is correct: `daemon` matches through the
intervening adjective, and row-039's `daemon` is disclaimed so its polarity differs.
This is a reading of dec-0014, not a change to it — but it is worth an amendment to the
entry's prose, because the next reader will implement the literal version.

### 3.2 A rejected decision must be matched on its alternatives, not only its title

dec-0014's table says `decision`/`rejected` is matched against "the entry's own
`title`". Corpus row-010 targets dec-0042's own alternative "a compacted event log" and
quotes *that alternative's* `why_not` — unreachable from the title. The corpus was
frozen before any matcher existed specifically so it would win this kind of argument,
so it did: rejected decisions are matched on title **and** alternatives. The change is
additive; every citation dec-0014's rule produces, this one produces too. Recorded in
`enforcementSet`'s doc comment.

### 3.3 `.agents/product-marketing.md` §6 and README.md/design.md §7 disagree on whitespace

§6's block has a blank line after the `$` line and another before the `→` line;
README.md and design.md §7 have neither. The golden follows README/design.md §7 — two
surfaces to one, and design.md §7 was reconciled to the README shape on 2026-07-30, the
same reconciliation E3-L1's report asked for. `internal/enforcer/testdata/golden/daemon.txt`
is byte-identical to README.md lines 85–89 (verified with `diff`). If §6's spacing is
the intended asset, the golden is a one-line change and the clip has not been recorded
yet — worth settling before E8 records it.

### 3.4 dec-0014's proposed top-level `revisit_if` schema field — deliberately not applied

dec-0014 nominates E3-L2 as the lane that should apply its proposed additive
`revisit_if` field to `schema/entry.schema.json`. I did not, for two reasons:

1. `schema/entry.schema.json` is a shared, high-collision file and three other agents
   are in this tree.
2. It would work against this lane's own brief. `docs/design.md` §7 and your framing
   both say a constraint conflict can only offer "supersede it in writing", and that
   the message must not imply a revisit condition exists where none can. Adding the
   field creates exactly the class of constraint that *can* carry one, and the renderer
   would then have to decide whether to print "none recorded" for every constraint that
   does not — reintroducing the ambiguity the asymmetry exists to avoid.

The proposal stands unapplied in dec-0014. It should be a decision made on its own
merits, not a side effect of this lane.

### 3.5 The fixture ledger is not a ledger dira can open

`internal/enforcer/testdata/ledgers/daemon/` is a flat directory of `*.md`. `local.Open`
wants a `.dira` with an `entries/` inside it, and `local.Find` walking up from that path
would land on **this repository's own `.dira`** — the check would silently grade against
the wrong ledger. The corpus references `ledgers/daemon/<id>.md` and is checksummed, so
the fixture cannot move.

Resolved by copying the fixture into a temporary `.dira` on the way into each test
(`fixtureLedger` in `cmd/dira/check_test.go`), byte for byte. That keeps the command on
the *real* read path — `local.Find`, the same store, the same index — rather than
growing a second reader that takes a directory of loose entry files, which is the second
read path E3's lane file forbids. It also makes `TestCheckLeavesTheLedgerAlone` safe to
run against a checksummed fixture.

### 3.6 `dec-0014`'s latency argument does not buy what it says it buys, here

dec-0014 bounds body reads to constraints "so int-0002's sub-100ms bar survives". The
matching rule is implemented as stated. But `ledger.Store` has no frontmatter-only read
— `Get` reads and decodes the whole file — so the file is read either way. What the rule
bounds is how much text is *tokenised*, not how much is *fetched*. Stated in
`documentText`'s doc comment rather than left as an implied saving.

---

## 4. Calls I made that you may want to overrule

- **Constraint block shape.** `docs/design.md` §7's (now-removed) `cst-0004` line is the
  only evidence of what a constraint conflict prints: the title, indented four spaces,
  unlabelled, with no date in the header. I reproduced it exactly, which makes the
  constraint block the one of the three that carries no label. Consistency would argue
  for `constraint: <title>`; the documented example won.
- **`why_not` is phrase-evidence only.** An option is a proposal, so overlapping it means
  proposing it. A `why_not` is argument prose that necessarily discusses the *adopted*
  design too — dec-0060's fourth `why_not` says the checkpoint file "exists to avoid" the
  wasted work, so `"write the checkpoint file atomically"` (row-025, what dec-0060
  chose) overlaps it strongly while contradicting nothing. Requiring a distinctive
  contiguous phrase keeps `"phone home"` (row-021) and drops that.
- **Negation includes disclaimer verbs** (`rejected`, `ruled out`, `refuse`, `avoid`,
  `skip`, …), not only grammatical negation. "a daemon was considered and rejected"
  contains no negative particle and is the clearest possible statement that a daemon is
  not being proposed. This is the single largest contributor to zero false positives.
- **`score` is in the `--json` output**, documented as advisory and explicitly "do not
  branch on it". It is what an agent-assist mode would rank on. Easy to remove if you
  would rather not publish a number that moves when the matcher is retuned.
- **`revisit_if` is always present in JSON**, `null` where none is recorded *or* where
  none can exist, with `supersede` always present beside it. The schema makes the
  "cannot exist" case machine-checkable (`basis != "alternative"` ⟹ `revisit_if` is
  null). The human renderer omits the line entirely for those bases, which is where the
  "must not imply" rule actually bites.

---

## 5. Not done, and whose it is

- **README.md carries no exit-code table for `dira check`.** The block itself is already
  there and now matches byte for byte. I left the file alone: four agents are in this
  tree and README is a collision magnet. `dira check -h` documents the flags and all
  three exit codes.
- **`docs/coverage.md`, `docs/roadmap.md`** — untouched, per the brief.
- **Cross-ledger inheritance, `[parents]`, private entries read from a `tier = "person"`
  ledger** — E3-L3's. The *renderer* handles `private: true` unconditionally and is
  tested with a sentinel in both modes, so E3-L3 inherits a working rule rather than a
  TODO. Namespaced refs (`me:cst-0002`) are E3-L3's to add; the schema's `ref` pattern
  already accepts them.
- **Supersession** — E3-L4's. Superseded decisions are excluded from the enforcement set
  and no redirect is emitted.
- **Nobody still owns invoking `dira check`.** `docs/plan/lanes/E3.md`'s own problem #1
  is unresolved: E3 scopes the verb, E2 scopes the hooks, and "runs before predicates
  are drafted" is unowned. The verb works and nothing calls it.

---

## 6. The one failing test, and why it is not mine

`go test ./...` fails on `TestWhyOnTheRealLedgerMatchesItsGoldenFile/dec-0012` in
`cmd/dira/why_test.go` — E1-L4's in-flight lane. Its golden predates
`.dira/entries/dec-0016.md` (written 2026-07-30 21:36), so the rendered chain now
carries a `superseded by dec-0016` line the golden does not have.

Proven independent of this lane: I moved `cmd/dira/check.go` and
`cmd/dira/check_test.go` out of the tree and re-ran it; it fails identically with none
of my code present. Restored afterwards; `go build ./...` clean.

`internal/ledger/fixture`'s `TestFullLedgerReadIsWithinBudget` also failed once during a
full `./...` run (median 171ms against a 150ms budget) and passes in isolation every
time. That is contention from several agents building at once on this machine, not a
regression — nothing in this lane touches `internal/ledger`.

---

## 7. Files

| Path | |
|---|---|
| `internal/enforcer/enforcer.go` | package doc, `Ledger`, `Verdict`, `Conflict`, `Check` |
| `internal/enforcer/exit.go` | the exit-code contract, beside the verdict it describes |
| `internal/enforcer/text.go` | tokenisation, clause spans, polarity, the suffix stripper, idf |
| `internal/enforcer/target.go` | the enforcement set and the unit model |
| `internal/enforcer/match.go` | scoring, the two thresholds, the two multiword rules |
| `internal/enforcer/render.go` | the frozen human block and the `--json` document |
| `internal/enforcer/testdata/golden/daemon.txt` | the demo asset, byte-identical to README.md:85–89 |
| `internal/enforcer/{corpus,match,render,offline,fixture}_test.go` | the tests above |
| `cmd/dira/check.go` | the verb, its exit-code handling, the read-path adapter |
| `cmd/dira/check_test.go` | the command's contract |
| `schema/check.schema.json` | the `--json` contract, with the privacy rule as a schema constraint |
