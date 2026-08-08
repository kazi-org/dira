# E3-L4 report — `dira supersede`, proven by a red-to-green enforcement flip

**Lane:** E3-L4, `docs/plan/lanes/E3.md`. **Status:** built, green, verified through a
real process. Not registered — see the wiring line first.

---

## 1. Wiring — landed

In `newApp`'s command slice in `cmd/dira/main.go`, in help order, immediately after the
`why` row:

```go
		{name: "supersede", summary: supersedeSummary, run: runSupersede, usage: writeSupersedeUsage},
```

**The coordinator wired this during review and it is in `main.go` at line 93.** It was
verified before that, too: the line was built into a real binary with `go build -overlay`
replacing `main.go` with a copy carrying exactly it, and every transcript in this report
came out of a real process, not a test harness.

`supersedeSummary` is a constant in `cmd/dira/supersede.go`, so the registry carries no
prose of its own. Nothing else in `main.go` changes — the `ExitCode() int` branch this
command needs is the one `dira check` already landed. See §4.1 for what the codes mean;
they were revised after review and are not the binary's default mapping.

The lane's own tests register the command only if `main.go` has not (`a.lookup` guards the
append), so they exercise the real registry now and would have worked before it landed.

---

## 2. What was built

| file | what it is |
|---|---|
| `cmd/dira/supersede.go` | the command: flags, refusals, the two writes, usage |
| `cmd/dira/supersede_test.go` | 11 tests, including the acceptance flip and the write-order test |
| `internal/enforcer/target.go` | `retiredSet` + `superseders`: the units a superseded decision is *reported* through |
| `internal/enforcer/enforcer.go` | `Notice`, `Verdict.Notices`, `matcher.notice` |
| `internal/enforcer/render.go` | `noticeLine` and the `notices` array in `--json` |
| `internal/enforcer/notice_test.go` | 5 tests over the redirect, including a schema negative control |
| `schema/check.schema.json` | `notices` added to the published `--json` contract |
| `internal/enforcer/testdata/ledgers/supersede/` | the fixture the acceptance line names: the daemon ledger + `dec-0061` |
| `docs/design.md` §7.1 | the surface documented where the enforcer is documented |
| `cmd/dira/check_test.go` | one refactor: `fixtureLedger` split so a second fixture can use it |

### 2.1 The command

`dira supersede <id> --with <id> [--note TEXT] [-C DIR]`. The id comes before the flags,
the same shape `dira log <id>` uses; `-C` after it. Both writes happen or the first one
is reported as having happened and the command tells you to run it again.

### 2.2 The two-file write, and the failure mode — decided, not discovered

dec-0002 puts edges on the subject entry so every dira mutation is one file. This command
is the exception, and `ledger.Store` has no transaction to make it atomic because none
exists over the GitHub Contents API (dec-0005). So the order was chosen for what a crash
between the two writes leaves:

* **edge first, then the state** (what shipped) — a crash leaves the retired entry still
  `accepted` and still enforced, with the replacement claiming to have replaced it. The
  firewall is unchanged, and the inconsistency is visible in the record.
* **state first, then the edge** — a crash leaves the entry superseded with nothing
  recorded as replacing it. `dira check` stops enforcing it, says nothing about where the
  thinking went, and the ledger looks fine.

The first failure is loud and safe; the second silently weakens the check. Both are
repaired by re-running the identical command, which is why the command *completes* a
half-finished supersession rather than refusing one.

This is pinned by `TestTheEdgeIsWrittenBeforeTheState`, which injects a failure into the
second write through a recording `ledger.Store`, asserts the write order was
`[dec-0061 dec-0060]`, and asserts that `dira check` still cites the retired entry
afterwards. Reversing the order in the source turns it red (mutation M3, §5).

### 2.3 The redirect

E3's enforcement table gives `decision`/`superseded` one row: matched against nothing,
"but a match is reported and redirected to its superseder". That is now a `Notice` —
informational, sorted, deduplicated per entry, and structurally incapable of changing the
exit code because notices are held in a separate slice from the units the citation loop
walks. `Verdict.Enforced` excludes superseded entries, so "no conflict with 6 enforced
entries" stays a claim about the firewall rather than a count of files.

The line names the **replacement only**:

```
ⓘ this plan matches thinking dec-0061 replaced; dec-0061 is enforced in its place
```

Three forms, because they are three different truths and one form would have to lie
about two of them:

| record | line |
|---|---|
| replacement is enforcement substrate | `… dec-0061 is enforced in its place` |
| replacement is staged / superseded / not substrate | `… dec-0061 is not enforced either, so nothing here is` |
| state flipped with no `supersedes` edge | `… the ledger records nothing that replaced it, so nothing here is enforced` |

The third is the shape of the defect `qst-0006` records, reported rather than papered
over with an invented successor.

---

## 3. Acceptance criteria, with observed evidence

Every transcript below is stdout/stderr of the **real binary** (built via overlay, §1),
run against `internal/enforcer/testdata/ledgers/supersede` copied into a scratch
directory. `exit=` is `$?` of the process.

### 3.1 `dira check "add a background daemon"` exits 2 citing dec-0060 — before

```
$ dira check "add a background daemon to track run state"
✗ conflicts with dec-0060 (accepted 2026-07-03)
    rejected alternative: "a daemon"
    why_not: violates the single-binary intent (int-0002)
    revisit_if: cold-start latency stops being the binding constraint
→ supersede dec-0060, or revise the plan
exit=2
```

### 3.2 The command

```
$ dira supersede dec-0060 --with dec-0061
dec-0060 is superseded by dec-0061; dira check now cites dec-0061 in its place   [stderr]
dec-0060                                                                        [stdout]
exit=0
```

### 3.3 The identical command exits 0, and no output cites dec-0060

```
$ dira check "add a background daemon to track run state"
ⓘ this plan matches thinking dec-0061 replaced; dec-0061 is enforced in its place
✓ no conflict with 6 enforced entries
exit=0
```

Observed: exit 0; no `✗`; the string `dec-0060` appears nowhere in stdout or stderr; the
informational line names `dec-0061`. The in-process test asserts all four, including the
absence of the retired id across `stdout+stderr`.

### 3.4 A conflict fabricated against dec-0061's own alternatives is cited instead

```
$ dira check "replay an append-only journal at startup to rebuild run state"
✗ conflicts with dec-0061 (accepted 2026-07-31)
    rejected alternative: "replay an append-only journal at startup"
    why_not: replay cost grows with the length of the run, which is the cost a rebuildable cache pays once and then stops paying
    revisit_if: a run outgrows what can be rebuilt inside the startup budget
→ supersede dec-0061, or revise the plan
exit=2
```

**Honest limit, stated rather than implied.** `dec-0061` is an `accepted` decision in the
fixture before the flip as well, so this plan conflicts with it before *and* after. What
the supersede changes is which entry is cited for the *daemon* plan, and that `dec-0060`
stops being citable at all. Making the fabricated plan compliant-then-conflicting would
have required `supersede` to also disposition `dec-0061` from `staged` to `accepted`,
which is E2's flow and which this command deliberately refuses to do (§4).

### 3.5 The record afterwards

```
$ grep -n "^state:\|^updated:" dec-0060.md
5:state: superseded
7:updated: "2026-07-31T05:33:06Z"

$ sed -n '/^edges:/,/^alternatives:/p' dec-0061.md
edges:
  - type: supersedes
    to: dec-0060
alternatives:
```

Both files re-validate against `schema/entry.schema.json` (through `schema.NewValidator`,
the published contract, not dira's runtime reading of it) — asserted in
`TestSupersedeFlipsWhatIsEnforced`.

### 3.6 No other entry file changed

`TestSupersedeFlipsWhatIsEnforced` snapshots every file under `.dira/entries` by sha256
before and after and asserts the modified set is exactly `[dec-0060.md dec-0061.md]` —
not "at least", not "these two changed". A third file appearing anywhere fails it.

Stronger than the acceptance line asks:
`TestSupersedeChangesTwoLinesAndReflowsNothing` asserts the *line-level* diff of each
file. dec-0060: `-state: accepted`, `+state: superseded`, `+updated: …`. dec-0061:
`+updated: …`, `+edges:`, `+  - type: supersedes`, `+    to: dec-0060`. Nothing else
moves — the hand-wrapped `why_not` blocks are re-emitted byte for byte, which is
dec-0002's legible-diff promise and which no other assertion in the file would have
caught, since they all read the parsed entry.

### 3.7 An informational line naming dec-0061 and no `✗`

Shown in §3.3. Also carried by `--json`, so a hook and a human see the same fact:

```
$ dira check --json "add a background daemon to track run state"
{
  "plan": "add a background daemon to track run state",
  "verdict": "compliant",
  "exit_code": 0,
  "enforced_entries": 6,
  "conflicts": [],
  "notices": [
    {
      "superseded_by": "dec-0061",
      "replacement_enforced": true,
      "basis": "alternative",
      "score": 1
    }
  ]
}
exit=0
```

### 3.8 `dira supersede me:cst-0002 --with cst-0005` exits non-zero, writes nothing, and
leaves the parent byte-identical

Real fixture: a `tier = "person"` parent ledger holding `cst-0002`, a child whose
`.dira/config.toml` declares `[parents] me = { path = …, ref = "main" }`. Digest is
sha256 over every file under the parent, path and contents.

```
parent digest before: a101a3516e5074828d6fce136cd1ef08de273966e96cd5a9ea60aaa7ce90890d

$ dira supersede me:cst-0002 --with cst-0005      (run from the child)
dira: me:cst-0002 belongs to another ledger, and dira never writes to one (cst-0003 rule 1). Supersede it in the ledger that owns it
exit=2
parent digest after : a101a3516e5074828d6fce136cd1ef08de273966e96cd5a9ea60aaa7ce90890d

$ dira supersede cst-0005 --with me:cst-0002      (the other direction)
exit=2
parent digest after : a101a3516e5074828d6fce136cd1ef08de273966e96cd5a9ea60aaa7ce90890d
```

Both directions are refused because both are upward writes: retiring a parent's entry
writes that parent's `state`, and being replaced *by* a parent's entry writes the edge
onto the parent's file. The refusal happens in `checkRefs`, before the ledger is opened —
before the command knows where any ledger is — so "writes nothing" is a property of the
control flow rather than of a cleanup path.

**The digest is a check that can fail, and that is shown rather than claimed.** The test
finishes by making the very write cst-0003 forbids — `dira supersede cst-0002 --with
cst-0009 -C <parent>`, legitimate when run *from* the parent, a violation if it had come
from the child — and asserting the digest moves. Same bytes, same digest.

---

## 4. What was refused, and why

### 4.1 Exit codes — revised after review, and now documented

**The gap was real.** As first written this command routed *everything* invalid to 2 —
a wrong argument order, a cross-ledger refusal and a mistyped flag alike — and its usage
block documented no exit codes at all. `dira check` documents three and routes its own
flag errors to 1 precisely so that its 2 can only mean a verdict (`docs/lore.md`
L-0013). A caller scripting both would have read a typo in a `supersede` flag as a
policy refusal.

**Changed, and here is the reasoning.** The coordinator offered either answer; this one
routes argument mistakes to 1, because it makes the two E3 verbs agree on what 2 means
rather than merely documenting a disagreement:

| code | meaning | cases |
|---|---|---|
| 0 | the supersession is recorded | this run wrote it, or it was already there |
| 2 | **the ledger refuses it** — a rule in the record says no | a parent's entry (cst-0003), a kind with no `superseded` state, a cross-kind pairing, an entry already replaced, a `staged` replacement, an entry superseding itself |
| 1 | **dira did not get that far** | unusable command line, malformed id, missing `--with`, entry not found, unreadable ledger, failed write |

Across `dira check` and `dira supersede`, **2 is the record's answer and 1 is never a
verdict**. A hook can fail open on 1 and surface 2 without knowing which verb it called.
That is the property E3's lane file asks for ("hook callers must never treat 1 as a
verdict") and it was not true of this command before.

**The cost, stated rather than smoothed over.** `dira log` maps *its* own flag errors to
2 through the shared `usagef`, so `supersede` is now consistent with `check` and
inconsistent with `log`. Both cannot hold: `log` and `check` already disagree. `log` has
no policy refusals — everything non-zero there is syntax or a failed write — so it never
needs 2 to mean anything else, and the verb that shares a caller with `check` is this
one. Recorded as `docs/lore.md` **L-0020** so that a later "refactor supersede to use
`usagef` for consistency" is stopped by the same tripwire L-0013 set for `check`.

Observed from the real binary, one process per row:

```
code | case
1 | wrong argument order
1 | unknown flag
1 | malformed id
1 | no --with
1 | entry does not exist
1 | unreadable ledger (no .dira)
---
2 | cross-ledger refusal (cst-0003)
2 | superseding a question
2 | cross-kind
2 | self-supersede
2 | staged replacement
---
0 | the supersession itself
0 | re-run, nothing to do
2 | a second superseder
```

`dira supersede -h` now ends with:

```
exit codes:

	0  the supersession is recorded — this run wrote it, or it was already
	2  the ledger refuses it: a rule in the record says no
	1  dira did not get that far — an unusable command line, an entry
	   that is not there, an unreadable ledger, a write that failed

A caller may read 2 as "refused on policy", and only that: an entry in
a parent ledger, a kind the schema gives no superseded state, an entry
already replaced, a staged replacement. A bad flag, a missing entry and
a failed write are all 1, so 2 never means dira is broken. This is the
same split `dira check` makes, where 2 is a verdict about the plan and
never a mistake in the flags — across both commands 2 is the record's
answer and 1 is never a verdict.
```

The refusal table in `TestSupersedeRefusesAndWritesNothing` now asserts the **specific
code** per row rather than "non-zero", `TestSupersedeHelpGoesToStdout` asserts the block
is in the help, and three further mutations confirm all of it can fail (M14–M16, §5).

`docs/design.md` §7.1 states the same split, so the contract is in the surface docs and
not only in `-h`.

### 4.2 The refusals themselves

Each of these is a refusal the command makes, each with a test, each verified red by
mutation (§5).

| refused | why |
|---|---|
| a namespaced ref on either side | cst-0003 rule 1: both directions are an upward write |
| a question, an intent, a note | the schema gives them no `superseded` state; asked of `ledger.Kind.States`, not of a list kept in the command |
| a cross-kind supersession | a `supersedes` edge replaces an entry with another of its own kind; a decision "replaced" by a constraint leaves the record saying something false |
| an entry superseding itself | — |
| a **staged** replacement | a staged decision is an unconfirmed regex-tier capture (dec-0003); letting one retire an accepted decision would let a guess silently switch off part of the firewall |
| a replacement that is itself superseded | retiring one entry in favour of another that is already retired |
| a **second** superseder | an entry is replaced once and by one entry; two `supersedes` edges at one target is the record telling two stories, and the check would have to pick one to redirect to |

And three things it deliberately does **not** do:

1. **It does not disposition.** `dira supersede` never moves an entry from `staged` to
   `accepted`, and never touches any field but `state`, `updated` and the one edge.
   `dira log <id>` refuses field rewrites for the same reason, and E2 owns the
   disposition flow; a second way to spell it here is a thing E2 would have to reconcile.
2. **It does not roll back.** A failure of the second write is reported with the ledger
   left in the safe half-state and an instruction to re-run. A rollback would be a third
   write that can fail in its turn, and it would discard the edge note.
3. **It does not widen the enforcement table.** See §6.

---

## 5. Every assertion verified red→green

The repo's most expensive recurring lesson is the checker that cannot fail, so each
assertion was checked by mutating the implementation and observing which test breaks.
Thirteen mutations, applied one at a time to a clean tree and reverted after each. Every
one turned at least one of this lane's tests red.

| # | mutation | tests that went red |
|---|---|---|
| M1 | the state is never flipped | `SupersedeFlipsWhatIsEnforced`, `…ChangesTwoLines…`, `…NeverLeavesTheTwoSidesDiverged`, `…FinishesAHalfFinishedFlip`, `…RedirectReachesTheJSON`, `…RefusesASecondSuperseder`, `…RepeatedIsANoOp` |
| M2 | the `supersedes` edge is never written | `SupersedeFlipsWhatIsEnforced`, `…ChangesTwoLines…`, `…NeverLeavesTheTwoSidesDiverged`, `…RedirectReachesTheJSON`, `…RepeatedIsANoOp`, `TheEdgeIsWrittenBeforeTheState` |
| M3 | the state is written **before** the edge | `TheEdgeIsWrittenBeforeTheState` (observed: write order `[dec-0060]`, want `[dec-0061 dec-0060]`) |
| M4 | the redirect names the retired entry instead of its replacement | `ANoticeNeverNamesTheRetiredEntry` (human **and** json) |
| M5 | a superseded entry counts as enforced and can be cited | `ASupersededDecisionIsReportedAndNotCited`, `ANoticeNeverNamesTheRetiredEntry`, `TheRedirectSaysOnlyWhatTheRecordSupports` (×3), `SupersedeFlipsWhatIsEnforced`, `…FinishesAHalfFinishedFlip`, `…RedirectReachesTheJSON` |
| M6 | a notice changes the verdict | same set as M5 |
| M7 | any kind may be superseded | `SupersedeRefusesAndWritesNothing/a_question`, `/an_intent` |
| M8 | a staged entry may supersede | `SupersedeRefusesAndWritesNothing/a_staged_replacement` |
| M9 | an entry may be superseded twice | `SupersedeRefusesASecondSuperseder` |
| M10 | the cross-ledger guard is removed | `SupersedeNeverWritesToAParentLedger` |
| M11 | the write reflows prose it did not change | `SupersedeChangesTwoLinesAndReflowsNothing` |
| M12 | the redirect is widened to superseded constraints | `ASupersededConstraintProducesNothing` |
| M13 | the notice is not rendered at all | `ANoticeNeverNamesTheRetiredEntry`, `SupersedeFlipsWhatIsEnforced`, `TheRedirectSaysOnlyWhatTheRecordSupports` (×3) |
| M14 | a bad flag routes to 2 again (the L-0013 regression) | `SupersedeRefusesAndWritesNothing/{unknown_flag,not_an_id,no_target,no_replacement,target_after_the_flags}` |
| M15 | a policy refusal routes to 1 | `SupersedeNeverWritesToAParentLedger`, `SupersedeRefusesASecondSuperseder`, `SupersedeRefusesAndWritesNothing/{a_question,an_intent,across_kinds,itself,a_staged_replacement}` |
| M16 | the usage block documents no exit codes | `SupersedeHelpGoesToStdout` |

Two of these caught real weaknesses in the tests as first written, and both are worth
recording because the tests passed before they were found:

* **M10 initially failed to go red.** `TestSupersedeNeverWritesToAParentLedger` asserted
  that stderr contained `cst-0003` — and `dira supersede`'s *usage block*, printed under
  every refusal, contains `cst-0003` too. The assertion was true whatever the command
  said. Fixed by asserting against the message paragraph only (`refusal()` splits on the
  blank line `(*app).main` writes), which also strengthened every row of the refusal
  table. M10 is red with the fix.
* **M12 initially failed to go red**, because widening `retiredSet` to constraints is a
  no-op on its own: a constraint has no `alternatives`, so it produces no units either
  way. The mutation was corrected to also route constraints through `constraintUnits`,
  which is what a real widening would have to do, and the test is red against it
  (observed: a notice with `Basis:constraint` for `cst-0004`).

Three further checks were built specifically so they could fail:

* `TestSupersedeNeverLeavesTheTwoSidesDiverged` walks the whole ledger after the command
  and checks both directions of the implication (an edge implies a superseded target; a
  superseded entry implies an edge). Its **negative control** runs the identical walk over
  a ledger seeded in the half-finished state and requires it to report the divergence — so
  the walk cannot pass by being blind.
* `TestSupersedeRefusesAndWritesNothing` attaches the byte-digest assertion to **every**
  row, not to the interesting-looking ones. A refusal that half-wrote would otherwise be
  invisible in the exit code.
* `TestASupersededConstraintProducesNothing` first asserts that the plan it uses *does*
  conflict with `cst-0004` while the constraint is active, so "no conflict after
  superseding it" is not a fact about the plan.

---

## 6. Contradictions and gaps found

### 6.1 The enforcement table's superseded rows are asymmetric — reported, not fixed

`docs/plan/lanes/E3.md` gives `decision`/`superseded` a reported-and-redirected match and
`constraint`/`superseded` a flat "nothing". Nothing in the table explains the difference,
and a superseded constraint is exactly as informative to redirect: the reader is
proposing something a standing rule used to forbid, and the rule was replaced.

**Not fixed here.** The table is the closed contract this epic is graded against, and a
lane widening it in passing is a decision nobody could later find. The current behaviour
is pinned by `TestASupersededConstraintProducesNothing` so that changing it has to be
deliberate. If E3-L1 (or the L1) wants the symmetry, the change is small: route a
superseded constraint through `constraintUnits` in `retiredSet` — the mutation in §5 is
the patch.

### 6.2 "No output cites dec-0060" versus "a match is reported"

The acceptance line requires, of the same command, that "no output cites dec-0060" and
that a match against the now-superseded dec-0060 "emits an informational line naming
dec-0061". Read loosely — with "cite" meaning the `✗` block and the `conflicts` array —
both hold even if the notice names the retired entry. Read strictly, the notice may not
name it at all.

**The strict reading shipped**, in both modes, because it satisfies both readings and
because the two clauses are coherent under it: the second says the line names *dec-0061*,
not both. The cost is that `--json`'s `notices` array does not carry the retired id, so a
machine consumer that wants it resolves `dec-0061`'s `supersedes` edge (one entry read,
or `dira why dec-0061`). No information is lost from the record — only from this one
message. If the integrator prefers the loose reading, the change is confined to
`noticeLine` and `jsonNotice`, and `TestANoticeNeverNamesTheRetiredEntry` is the test
that would have to change with it.

### 6.3 `qst-0006` is now partly enforceable, and nothing enforces it

`qst-0006` records that nothing detected the edge/state divergence until a *reader* was
built: the schema validated all three broken entries, and no check tested whether entries
were **consistent with each other**. `dira supersede` closes the hole going forward — it
cannot create the divergence — but it does not detect an existing one, and neither does
anything else in the binary. The walk in
`TestSupersedeNeverLeavesTheTwoSidesDiverged.divergence` is about thirty lines, has a
negative control, and is exactly the check that was missing.

**Where it belongs — a `dira lint` verb, run by CI.** Asked for a recommendation, this is
it, with the alternatives and why they lose:

* **Not inside `dira check`.** Tempting, because `check` already reads every entry, so the
  walk is free there. But `check`'s exit 2 is a verdict about *a plan*, and a ledger that
  contradicts itself is not a contradiction of the plan — folding it in would make 2 mean
  two things, which is the exact defect §4.1 just removed from this command. Routing it to
  `check`'s stderr instead is worse: `check` runs on every plan from a pre-plan hook, so
  the same ledger-wide warning would print on every invocation until someone silenced the
  hook, and int-0001 is the epic about checks that get switched off.
* **Not a script under `scripts/`.** `coverage.py` and `privacy-lint.py` are
  dependency-free by design and each carries its own frontmatter parser. A third one would
  be a third reader of the entry format, and this check is about *semantics* — edge types,
  states, ref resolution — where a regex reader drifts from the codec silently. The gate
  should be the verb; CI should run the binary.
* **A verb.** `dira lint` (or `dira doctor`) reading the whole ledger and reporting what is
  internally inconsistent, with exit codes on the same rule §4.1 sets: `0` consistent,
  `2` the record contradicts itself, `1` dira could not read it. Human-runnable, CI-runnable,
  and its own exit code means nothing else has to be overloaded.

Seed it with the two-directional supersedes walk from this lane's test — an edge implies a
`superseded` target, a `superseded` entry implies an edge — and the obvious neighbours it
generalises to: a ref pointing at an entry that does not exist, an edge whose type the
target's kind cannot carry, a `derives_from` chain with a cycle. That set is the *class*
`qst-0006` says has no check, not just the one instance it found.

Sizing, honestly: the walk is thirty lines, the verb and its usage are another eighty, and
the tests are the real work — every rule needs a fixture that is broken in exactly that way
plus a negative control, which is where the corpus discipline of E3-L1 applies. Half a
lane. It also has a prerequisite worth naming: someone must decide whether an inconsistency
found in a *parent* ledger is this ledger's problem to report, which touches `qst-0001` and
should stay out of scope on the first pass.

Not done here because it is a new surface and this lane owns one; flagged for routing.

The partial mitigation that did ship: a redirect whose replacement is not itself
enforcement substrate says so (`… is not enforced either`), and a superseded entry with
no `supersedes` edge pointing at it says *that* (`… the ledger records nothing that
replaced it`). Both are visible in the output of a check rather than requiring an audit.

### 6.4 `docs/design.md` §4.1 said what the tool now does — one sentence, no mechanism

§4.1 defines `supersedes` as "replaces an earlier entry, flipping it to `superseded`",
which is exactly the two-sided write. Until this lane there was no command that did the
flipping, so the sentence described a convention — and a convention is what produced the
three broken edges. §7.1 now documents the command, the redirect, the write order and the
refusals.

---

## 7. Gates

Run against the working tree after the review round. **This tree is shared with several
other in-flight lanes**, so it was re-run at the end rather than trusted from earlier in
the session.

| gate | result |
|---|---|
| `go vet ./...` | clean |
| `golangci-lint run ./...` | **0 issues**, whole repo |
| `gofmt -l cmd internal schema` | clean |
| `python3 scripts/coverage.py` | PASS — every obligation has a disposition |
| `python3 scripts/privacy-lint.py` | PASS — all four cst-0003 checks |
| `go test ./...` | **all 13 packages ok**, whole repo |

Earlier in this lane the full suite was red in four places, all of them other lanes'
uncommitted work rather than this one's, and all four are now resolved by their owners:
the `dira why` golden against a newly added `dec-0019`, `internal/ui` importing `io/fs`,
a `schema/entry.schema.json` fixture with no matching case, and a `cmd/dira/zz_dbg_test.go`
debug scratch that redeclared `readEntry` and stopped the package compiling. They are
recorded here only because this lane's earlier evidence was captured around them: while
that scratch file existed, `cmd/dira` was re-run with `go test -overlay` mapping it to an
empty file rather than deleting another lane's work in progress.

No obligation was added to `docs/coverage.md` and no entry was written to
`.dira/entries/`, so the coverage register is untouched. `docs/roadmap.md`,
`docs/coverage.md` and `cmd/dira/main.go` were not edited. `docs/lore.md` gained one
entry, **L-0020**, which is the tripwire for §4.1's exit-code split.

## 8. Not done

* **The command is not registered.** One line, §1.
* **`dira supersede` is not exercised on dira's own ledger.** dec-0001 says "overruling
  this is a one-line supersede" and E3's lane file calls dira's own ledger the acceptance
  test for this verb. Running it for real means retiring a real entry, which is a
  decision, not a lane task.
* **No consistency check ships.** §6.3 recommends where it belongs — a `dira lint` verb
  run by CI, seeded from this lane's divergence walk — and sizes it at half a lane.
* **`README.md` is unchanged.** Its `dira check` block is a frozen marketing asset and
  gains nothing from this lane; the `→ supersede dec-0060` line it already prints now has
  a command behind it.
