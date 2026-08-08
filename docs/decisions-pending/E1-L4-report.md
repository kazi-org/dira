# E1-L4 — `dira why`: the chain, the alternatives, the why_nots

**Status:** every `acc:` clause green. Tree left dirty and uncommitted.
The one red test in the module is another lane's (§10).

---

## 0. What you need to wire in

**The registry line for `cmd/dira/main.go`** — insert in `newApp`'s slice, between
`log` and `reindex` so help stays alphabetical:

```go
{name: "why", summary: whySummary, run: runWhy, usage: writeWhyUsage},
```

`whySummary`, `runWhy` and `writeWhyUsage` are all in `cmd/dira/why.go`.

**`allowedModules` in `cmd/dira/build_test.go`: no change needed.** `internal/why`
imports `context errors fmt io sort strings unicode/utf8` plus `internal/index` and
`internal/ledger`. Every module it reaches was already on the allowlist when E1-L3
filled it in, so this lane adds nothing to the dependency tree and nothing to the
binary beyond its own code. Verified with `go list -f '{{join .Imports " "}}'`.

**No change needed to `internal/ledger/boundary_test.go` either.** `internal/why`
imports no `os`, no `path/filepath`, no `io/fs` — everything reaches the ledger
through `ledger.Store` via `index`. `cmd/dira/why.go` uses `openLedger`, so it adds
no filesystem import to the command either.

Files added, all of them new and none shared with another lane:

| path | |
|---|---|
| `internal/why/why.go` | the chain builder: `Source`, `Chain`, `Node`, `Resolve`, `Build` |
| `internal/why/render.go` | the text renderer: `RenderText`, `RenderCandidates` |
| `internal/why/why_test.go`, `cache_test.go`, `latency_test.go`, `race_{on,off}_test.go` | |
| `cmd/dira/why.go`, `cmd/dira/why_test.go` | the subcommand |
| `cmd/dira/testdata/why/{dec-0002,daemon,int-0002,dec-0012,dec-0015}.golden` | |

I did not touch `cmd/dira/main.go`, `cmd/dira/build_test.go`, `internal/enforcer/`,
`.github/`, `docs/design/`, `.dira/entries/`, `docs/roadmap.md`, `docs/coverage.md`,
or any `scripts/*`. Nothing is committed or pushed.

---

## 1. What it looks like

`dira why dec-0002`, verbatim from `cmd/dira/testdata/why/dec-0002.golden`:

```
int-0002  Zero-ceremony operation — one binary, no server, no  active 2026-07-29
          daemon
            files-in-git is what lets there be no server
└─ dec-0002  One file per entry, not an append-only JSONL    accepted 2026-07-29
             ledger
   ├─ ✗ Append-only .dira/ledger.jsonl with a SQLite cache, following Beads
   │    Every concurrent writer appends at the same offset, so two sessions
   │    logging in the same minute produce a conflict on the same line of the
   │    same file — and the merge resolution for an append log is exactly the
   │    case git handles worst. It also forces a whole-file read-modify-write
   │    through the GitHub API, which makes the phone a second-class client for
   │    no gain. The mobile and multi-session designs both quietly assume
   │    per-entry files; JSONL was the earlier sketch and does not survive them.
   │    revisit if  entry volume grows past roughly 10k per ledger, where
   │                per-file overhead starts to dominate — at which point
   │                compaction into periodic archive files is additive, not a
   │                schema change
   ├─ ✗ A single YAML or TOML document holding all entries
   │    Same line-level conflict problem as JSONL, plus the whole ledger has to
   │    be parsed to read one entry, and a malformed edit takes down the entire
   │    ledger instead of one file.
   └─ ✗ SQLite as the source of truth, with git storing the .db file
        A binary blob in git is unreviewable and unmergeable. It also breaks the
        property that makes dira trustworthy — that the record is human-readable
        text you can `git log -p` and read without dira installed.

edges
  informs     dec-0005  A storage interface (local FS | GitHub API) is committed
                        to before the first surface ships
                          a single-file mutation is what makes the GitHub API
                          backend viable
  derived by  dec-0015  The derived cache stays honest by content hash, not by
                        modification time
                          "if the cache and the files ever disagree, the files
                          win" is a guarantee, and this is what makes it one
```

`dira why daemon` resolves to `int-0002` and prints the same bytes as
`dira why int-0002` — the intent, its nine `derived by` decisions, and the explicit
statement that it records no alternatives. That golden is `daemon.golden` and
`int-0002.golden`, and a test asserts they are identical rather than assuming it.

---

## 2. Four things it deliberately does not print, and why each is a decision

**The entry's body.** `docs/design.md` §10 step 8 enumerates what `dira why daemon`
must answer — "the decision, the rejected alternative and its why_not, the intent it
served, the converged goal that realized it, and the ADR path if you want the long
form" — and the body is not in that list; the ADR path is offered *instead of* it.
The bodies in this repo's own ledger run forty lines. Printing one turns a chain into
a document and the one-screen target into a scroll, and §4.2 already says the
alternatives are the load-bearing field. **If you disagree, this is the change to
make**, and it is one call site.

**Any colour, at all.** Not one ANSI byte is emitted. `DESIGN.md` law 1 reserves red
for drift, contradiction and `dira check`, so the one hue this output would plausibly
reach for is the one it may not have — and the acceptance line's "stripping ANSI
leaves the tree intact" is then true in the strong direction rather than the vacuous
one. See §5 for how that is tested without being a test of nothing.

**The invocation line.** `s1-decision.html` opens with `$ dira why elixir` because a
stranger arriving from a link has to be told what produced the page. In a terminal the
reader typed it and it is already on screen. `Chain.Query` carries it so E6's renderer
draws it; this renderer does not repeat it back.

**Anything about whether a `realized_by` goal converged.** `README.md`'s worked
example shows `realized_by kazi:prop-resume-8a1f → converged ✓`. **That arrow is E4's,
and E1 may not draw it** (`dec-0004`: status derived, never stored; no kazi client in
the binary). The target prints verbatim and nothing is claimed about it.
`TestRealizedByTargetsArePrintedVerbatimAndNothingIsClaimedAboutThem` asserts the
output contains none of `converged`, `in progress`, `completed`, `planned`, `✓` or
`3/3`. **This is a place the README currently promises more than M1 delivers**, and it
is the second such place (§9).

---

## 3. The structured chain — the contract handed to E6

`internal/why` produces a `*Chain`; `RenderText` is renderer one of two. Nothing in
the renderer reads the ledger, so E6 implements `RenderHTML(io.Writer, *why.Chain)`
and cannot drift. Every struct carries JSON tags, so a consumer outside this module
gets the same chain without re-walking anything.

```go
type Chain struct {
	Query        string               // what the human typed — the invocation line
	Subject      Node
	Arising      [][]Node             // derives_from ancestry, by generation, outermost first
	Alternatives []ledger.Alternative // Option / WhyNot / RevisitIf, in file order
	Realized     []Artifact           // realized_by targets, verbatim
	ADR          string
	SupersededBy []Node
	Related      []Relation           // every other edge, both directions, deterministically ordered
	Cycle        []string             // where a derives_from loop was cut
}

type Node struct {
	Ref, Title string
	Kind       ledger.Kind
	State      ledger.State
	Date       string      // full RFC3339 — the renderer decides how much to show
	Note       string      // the note on the edge that put this node here
	Private    bool
	Resolution Resolution  // Oriented | Unresolved
}

type Artifact struct{ Target, Note string }
type Relation struct {
	Type     ledger.EdgeType
	Incoming bool            // declared by the other entry (edges live on the subject, dec-0002)
	Node     Node
}
```

Three notes E6 should read before building on it:

**`Resolution` has two values, not the three `DESIGN.md` names.** `withheld` and
`orphan` are tier states: telling them apart needs the parent-ledger map, which is E5
and is blocked on `qst-0001`. Until that exists dira genuinely cannot distinguish "a
parent in a ledger this repo declares but cannot show" from "a ref into no ledger at
all", and a renderer that guessed would be asserting a resolution nobody derived. E1
says only what it knows: `Oriented` or `Unresolved`, rendered in words as
`not in this ledger`, never as an alarm. **When E5 lands, this enum widens to three
and `s1-decision-withheld.html` becomes reachable; nothing else in the chain changes.**

**`Arising` is grouped by generation, and that is the one lossy thing in the
structure.** An entry with two parents in different branches puts both at the same
generation, so the chain shows how far above the subject an ancestor sits but not
which parent it hangs from. A generation is the *longest* path from the subject, so an
ancestor reachable two ways appears once, at its deepest. `dec-0015` in this repo is
the real case (it arises from both `dec-0002` and `dec-0005`, and `dec-0005` arises
from `int-0002` too); look at `dec-0015.golden` and you will see the flattening. The
alternative was a per-branch nesting that duplicates the subject or the ancestor, and
both are worse to read. If E6 wants the exact edges it has them: `Node.Ref` plus the
subject's own `derives_from` edges reconstruct the graph.

**`Related` is sorted and stable.** Order is `supersedes`, `informs`, `blocks`
(outgoing), then `derived by`, `informed by`, `blocked by` (incoming), then by ref.
`TestRelatedEdgesComeBackInAStableOrder` builds the same chain three times and pins
every position, so neither the golden files nor E6's markup depends on how SQLite felt
about tie-breaking.

---

## 4. `acc:` clause by clause

The lane's acceptance line has six clauses. All green.

**(1) `dira why dec-0002` prints every element DESIGN.md and design.md §10.8
require, asserted by a golden-file test. GREEN.**
`TestWhyOnTheRealLedgerMatchesItsGoldenFile`, five chains over this repository's real
ledger. A golden file proves output has not changed; it does not prove it was ever
right, and a careless `-update` over a broken renderer would pin the breakage. So
`TestTheGoldenFilesContainWhatTheAcceptanceLineNames` is the other half: fourteen
content assertions taken from the entry files rather than transcribed — the decision's
title and state, all three alternative options, two why_nots, the revisit_if and its
label, the parent's id, title and state, the edge note explaining the parent link, and
the outgoing `informs` target. It runs at `-width 300` so a flattened comparison
cannot be broken up by the right-margin status column.

**(2) Box-drawing characters that are selectable text, no ANSI box graphics
(stripping ANSI leaves the tree intact). GREEN, and asserted in the strong
direction.** See §5.

**(3) `dira why daemon` resolves by term to the same entry set as its id form.
GREEN.** `TestTermAndIdResolveToTheSameChain` asserts the two are byte-identical.
"daemon" appears in exactly one title in this ledger — `int-0002`'s — so this is the
unambiguous case; the ambiguous case is clause (6)'s neighbour, below.

**(4) An entry with no alternatives states that explicitly. GREEN.**
`TestAnEntryWithNoAlternativesSaysSo`. The sentence names the kind, because the same
absence means two different things: a decision with no alternatives is an assertion
(§4.2, and the schema rejects it) while an intent has nothing to have chosen between.
`no alternatives recorded — an intent states a direction rather than choosing between
options`.

**(5) `dira why nonexistent-0000` exits non-zero with a message that does not
resemble a crash. GREEN.**
`TestAnUnknownRefExitsNonZeroWithoutLookingLikeACrash`, three queries (a well-formed
id in no ledger, a plausible id, a free-text term). "Does not resemble a crash" is made
checkable rather than left to taste: exit 1, empty stdout, the message names what was
looked for and where, it is one line, it suggests something to try, and it contains
none of `panic`, `goroutine`, `runtime error`, `nil pointer`, `0x`, `*ledger.` or
`index:`. Exit **1** and not 2: the command was used correctly, the ledger simply does
not hold this.

```
dira: no entry matches "nonexistent-0000" in /…/dira/.dira — try a word from its title, or one of its tags
```

**(6) Superseded, no-parents and unresolvable cases. GREEN**, and mostly not on this
repo's ledger — see §9 for what it cannot express.
`TestASupersededEntryShowsWhatSupersededIt` puts the supersession *in the chain*, as
the subject's first child, not in the edges footer, and asserts that placement: being
superseded changes how every other line should be read.
`TestAnEntryWithNoParentsRendersWithoutAChain` asserts a chainless entry opens with
itself and renders no empty `edges` section.

---

## 5. The ANSI clause, and how it avoids being a test of nothing

The brief's clause is "stripping ANSI leaves the tree intact". Over output that
contains no ANSI, that assertion passes without measuring anything — which is exactly
the vacuous-predicate shape the brief warns about. So it is asserted as three claims,
and the stripper is given a positive control:

- `TestTheANSIStripperHasTeeth` feeds `stripANSI` a coloured sample
  (`"\x1b[31m└─ \x1b[1;33mdec-0002\x1b[0m  refused\x1b[0m"`) and asserts it returns
  `"└─ dec-0002  refused"`, and that it leaves escape-free text alone. A stripper that
  did nothing fails here.
- `TestTheChainIsSelectableTextWithNoANSIGraphics` then asserts, over all five golden
  chains: the output contains no `0x1b` byte at all, `stripANSI` is a no-op on it, and
  it contains box-drawing characters.

Red-before-green, verified by breaking it rather than asserted: colouring
`markRefused` red produced

```
--- FAIL: TestTheChainIsSelectableTextWithNoANSIGraphics/dec-0002
    why_test.go:228: the chain contains an escape byte at offset 272; DESIGN.md law 1
    reserves colour for drift and contradiction, and a refusal is a record, not an alarm
```

**Law 1 is respected, and the check is a grep-equivalent rather than a promise.**

---

## 6. Term resolution — the rule, and why an ambiguous term is not an error

`index.Resolve` (E1-L3) already fixes the matching: an exact id resolves to itself
*alone*, anything else matches a case-insensitive title substring or a whole tag,
newest first. This lane owns what happens next, and the rule is:

| matches | behaviour | exit |
|---|---|---|
| 0 | one-line message naming the term and the ledger | 1 |
| 1 | the chain | 0 |
| >1 | the candidate list, and how to pick | 0 |

**Several matches exits 0 because it is an answer, not a failure.** `dira why founding`
over a tag most of this ledger carries is a real question with a real answer — *these
are the entries that mention it*. The brief's own framing is that "a fuzzy matcher that
silently picks one of five entries is worse than one that lists the candidates", and
listing is the whole point; giving it a failure exit code would push a caller toward
treating the list as an error and falling back to the guess.

Red-before-green: making the `default:` branch take `candidates[:1]` and render its
chain fails `TestAnAmbiguousTermListsTheCandidates` with *"an ambiguous term rendered
a chain; it must list candidates instead"*.

**This is the clause of the design I am least sure about**, and it is a one-line change
if you want exit 1 there. What I am sure about is that it must not guess.

---

## 7. Latency — measured, not assumed

In-process, spawn excluded, warm cache, median of 21 runs on this machine:

| | median |
|---|---|
| whole answer over the **200-entry fixture** — `index.Open` + resolve + build + render | **16.4ms** |
| whole answer over **this repo's ledger** (30 entries) | **5.1ms** |
| **build + render alone**, index already open | **0.49ms** |

Against E1's <40ms budget for dira's own work, `dira why` uses **16.4ms at 200
entries**, and `TestTheWholeAnswerFitsTheBudget` fails above 40ms and logs the real
number either way. The test skips under `-race` with a stated reason, following
E1-L3's precedent: instrumentation costs several times the work being measured.

**The attribution matters more than the total.** 15.9ms of the 16.4 is E1-L3's
reconcile inside `index.Open`; this lane's own code is **0.49ms**, because a chain
reads about ten entry files rather than two hundred. The second measurement is in the
test for exactly that reason — so a regression in either is not read as a regression
in the other. It also means `dira why` will track whatever E1-L6 does to the open
path, and nothing this lane could do would move the number much.

Process spawn is not included and is 60–90ms on this hardware (E1-L3 §10). That is
E1-L6's problem and it is a real one.

---

## 8. Chain at scale — the limit hit, stated rather than solved

I did not invent a collapse policy. Measured lengths at the default 80 columns:

| chain | lines | |
|---|---|---|
| `dec-0002` | 35 | 3 alternatives — typical |
| `daemon` / `int-0002` | 39 | an intent with 9 incoming derives_from edges |
| `dec-0012` | 49 | 3 alternatives, 2 with revisit_if, plus a supersession |
| **`dec-0015`** | **96** | 5 alternatives averaging ~130 words each |

**Two of the three collapse questions are already answered by DESIGN.md and this
renderer satisfies them.** The long-content rule is "≤ 6 alternatives → everything
open; > 6 → only the upheld one"; every entry in this ledger is at or under six, so
"everything open" is the correct emission and that is what happens. Nothing is
collapsed, elided or truncated: what `dira why` prints is always the whole chain.

**The depth rule is still unwritten (DESIGN.md open question 1) and I did not write
it.** Two consequences worth having on record:

1. `dec-0015`'s 96 lines are not a renderer defect — they are five refusals with long
   grounds, and the design's own answer for that on the web is the `<details>` summary
   carrying *one line of ground*. A terminal has no `<details>`. The obvious analogue
   is a `--brief`/`--full` pair, which is a collapse policy, so it is not here. **If
   you want the flagship command's output to fit a screen on its worst entry in this
   ledger, that is the change, and it should be decided for terminal and web
   together.**
2. Line width has one documented escape hatch. A wrapped column keeps at least 24
   columns for text however deep the indent, so a chain about five generations deep
   will exceed the requested width rather than render one word per line.
   `MinWidth` is 56 and is *derived* — the deepest fixed indent this renderer produces
   is an edge note under the longest label, at 31 columns, plus the 24-column floor.
   `TestWidthIsHonoured` asserts no line exceeds the width across all five chains at
   56, 64, 80 and 120 columns, and also that the longest line is at least half the
   width, so a renderer wrapping at some other column cannot pass.

---

## 9. What this repository's ledger cannot express, and what I did about it

Three of the acceptance line's elements have **no instance in `.dira/entries/`**, which
I found by grepping before writing the tests rather than after:

- **No entry sets `adr:`.** E1's lane file already flags that nobody owns the mirror
  writer, so the field is never set by dira today and `mirror.adr` is a config key the
  binary does not keep. Covered by `TestTheADRPathIsPrintedAndTheFileIsNeverRead` over
  a synthetic ledger, which also asserts the ADR file does not exist — dec-0009 makes
  it exhaust, and a renderer that read it would invert the one-way authority.
- **No `realized_by` edge exists.** Covered synthetically, per §2.
- **No entry is in state `superseded`.** Covered synthetically.

And one inconsistency in the ledger itself, which I have **not** fixed because
`.dira/entries/` is not mine:

> **`dec-0012` and `dec-0005` both read `state: accepted` while carrying an incoming
> `supersedes` edge.** `dec-0016` supersedes `dec-0012`; `dec-0012` supersedes
> `dec-0005`. design.md §4.1 says `supersedes` "replaces an earlier entry, flipping it
> to `superseded`", and neither flip happened. So `dira why dec-0012` prints
> `accepted 2026-07-30` on the subject row *and* `superseded by dec-0016` beneath it.
> That is the honest rendering — dira reports the stored state and the edge as they
> are, and never asserts a status it did not derive — but the ledger is telling two
> stories and one of them is wrong. Whoever owns disposition (E2) should flip them, or
> the design should stop claiming the edge flips state.

**One more place the README over-promises**, on top of the `→ converged ✓` in §2: its
`dira why daemon` example is drawn from the kazi ledger (`qst-0007`, `dec-0060`,
`dec-0042`), none of which exist here, and it roots the chain at a *question* whose
answer is a decision. This renderer roots at the subject and walks `derives_from`
upward, so `dira why <question>` shows what blocks and what answers it through the
`edges` block rather than as an inline `✓ dec-0060` row. Recognisably the same
artifact; not the same shape. Worth an issue against `README.md` before launch, and
not something I would change in the renderer to match a mock.

---

## 10. Verbatim command output

```
$ go build ./...
  exit=0

$ go vet ./...
  exit=0

$ golangci-lint run ./internal/why/...
0 issues.
  exit=0

$ golangci-lint run ./cmd/dira/
0 issues.
  exit=0

$ go test -count=1 ./...
ok  	github.com/kazi-org/dira/cmd/dira	26.574s
ok  	github.com/kazi-org/dira/internal/enforcer	4.987s
?   	github.com/kazi-org/dira/internal/enforcer/enforcertest	[no test files]
ok  	github.com/kazi-org/dira/internal/index	21.408s
?   	github.com/kazi-org/dira/internal/index/indextest	[no test files]
--- FAIL: TestNoFilesystemImportsAboveTheBackend (3.89s)
FAIL
FAIL	github.com/kazi-org/dira/internal/ledger	4.441s
ok  	github.com/kazi-org/dira/internal/ledger/fixture	2.969s
?   	github.com/kazi-org/dira/internal/ledger/ledgertest	[no test files]
ok  	github.com/kazi-org/dira/internal/ledger/local	1.954s
ok  	github.com/kazi-org/dira/internal/why	9.606s
ok  	github.com/kazi-org/dira/schema	0.482s
```

**The one failure is not this lane's.** It is:

```
boundary_test.go:81: internal/enforcer/enforcertest imports "os".
boundary_test.go:81: internal/enforcer/enforcertest imports "path/filepath".
```

E3's new `enforcertest` package needs two entries in `allowed` in
`internal/ledger/boundary_test.go`, which is E1-L1's file and not mine to edit. The
same failure is present with my work removed. `internal/why` imports neither.

`golangci-lint run ./...` over the whole module reports 7 `unused` issues, all in
`cmd/dira/check.go` (E3's, landed unregistered and unreferenced). None are in my
files, and my own `runWhy` is referenced from `why_test.go`, so it does not have that
problem whether or not you have merged the registry line yet.

Executed counts rather than a bare `ok`:

```
$ go test -count=1 -v ./cmd/dira
  pass=127  fail=0  skip=0   (55 top-level tests)
  of which this lane's:  pass=49  fail=0   (13 top-level tests)

$ go test -count=1 -v ./internal/why
  pass=16   fail=0  skip=0   (13 top-level tests)
```

Race:

```
$ go test -race -count=1 ./internal/why ./cmd/dira
ok  	github.com/kazi-org/dira/internal/why	30.972s
ok  	github.com/kazi-org/dira/cmd/dira	23.882s
```

Repo gates, both unaffected by this lane (it adds no `.dira/` entry and therefore no
obligation):

```
$ python3 scripts/coverage.py
obligations extracted : 67
registered            : 67
uncovered             : 0
COVERAGE PASS — every obligation has a disposition.
  exit=0

$ python3 scripts/privacy-lint.py
PRIVACY LINT PASS — cst-0003 enforced by 4 checks.
  exit=0
```

---

## 11. Red-before-green, verified by breaking rather than asserted

| break | result |
|---|---|
| colour `markRefused` red | `FAIL` — *"the chain contains an escape byte at offset 272"* |
| stop rendering `revisit_if` | `FAIL` — 3 golden chains diff, and *"missing the revisit_if"* |
| ambiguous term renders `candidates[0]`'s chain | `FAIL` — *"an ambiguous term rendered a chain; it must list candidates instead"* |
| remove the `path[ref]` cycle guard from the ancestry walk | `fatal error: stack overflow` — the guard is load-bearing, not decorative |

All restored; `go test -count=1 ./internal/why ./cmd/dira` green afterwards.

---

## 12. Things inside the lane you should sanity-check

**Cycle safety is a path set plus a monotone depth, and termination is proven rather
than bounded by a counter.** `walk` cuts a `derives_from` loop where it closes and
records the ref; it re-walks a ref only when it is found at a strictly greater depth,
and depth is bounded by the entry count, so there is no visit budget to tune and no
untested guard clause. `TestADerivesFromCycleIsReportedRatherThanFollowed` covers a
self-loop, a two-entry loop and a three-entry loop entered from outside it.
`TestTheWalkTerminatesOnAWideDiamond` is the other half: twelve stacked *lopsided*
diamonds — 4096 distinct paths over 37 entries, every shared node reachable at two
different depths — which is what makes "a generation is the longest path" a claim with
something at stake rather than a description of a straight line. A walk that took the
first depth it found reports 24 generations instead of 36 and fails.

**The chain is added to E1-L3's differential harness rather than trusted.**
`TestTheChainIsTheSameWithAndWithoutACache` feeds `indextest.RunTwice` two real
rendered queries — twelve chains over the 200-entry fixture, and a candidate list —
so `dira why` inherits dec-0002's guarantee that deleting `.dira/cache/` changes how
long an answer takes and nothing else about it. `why` is the command that renders the
most from files (every why_not, every revisit_if, every parent title), so it is the
command a cache that had quietly become authoritative would lie through.

**`dira why` writes nothing, asserted.** `TestWhyWritesNothing` snapshots
`.dira/entries/` before and after three invocations including a failing one, and
compares contents file by file in both directions. `dec-0004` is the reason; a read
verb that mutates is how a derived-status product acquires stored status by accident.

**A degraded cache still answers, identically.** `TestAnUnusableCacheStillAnswers`
copies the ledger, makes `.dira/` read-only, and asserts exit 0, a notice on stderr,
and **byte-identical stdout** against the warm run. That is true by construction —
everything rendered comes back through `ledger.Store` — but construction is what a
test is for.

**`-width` is a real flag with a floor, not decoration.** It exists because reading
the terminal's real width means an ioctl, which means `os` or `golang.org/x/term` in a
package `dec-0005` forbids to know what a file descriptor is — and it would make
`dira why` produce different bytes in a pipe than on a screen, which is precisely what
the golden test exists to catch. A right-margin status that would squeeze the text
below the floor moves to its own right-aligned line rather than overflowing.

**Prose is re-wrapped, and that is deliberate.** The ledger's why_nots arrive
hand-wrapped inside folded YAML scalars, so their line breaks are an artifact of the
width someone typed at. Collapsing whitespace and re-wrapping is the only way the same
value renders the same at two widths — and it is also what lets E6 set the same text
at a different measure without the terminal's line breaks bleeding through.

**`runeLen` counts runes, not display columns.** Every glyph this chain draws is
single-width; a CJK title would be measured a column short. That is a wrapping
imperfection rather than a correctness one, and fixing it needs a width table the
binary has no other reason to carry. Recorded rather than left to be discovered.

---

## 13. What in the brief turned out to be wrong or incomplete

- **"README.md shows the intended `dira why daemon` output"** — it shows an output
  built from the *kazi* ledger, containing entries that do not exist here, rooted at a
  question rather than at the subject, and asserting a kazi convergence state that
  `dec-0004` forbids E1 to derive. Two of its five lines are things this lane may not
  print. I matched the *artifact* (box-drawing chain, `✗`/`✓` marks, the alternative
  with its why_not beneath, the right-margin state) and not the literal transcript, and
  I think the README needs an edit rather than the renderer. §9.
- **"`dira why dec-0002` … the `adr` path when the field is set"** — no entry in this
  ledger sets it, and none can until someone owns the mirror writer. Same for
  `realized_by` and for `state: superseded`. Three of the acceptance line's elements
  are therefore green on synthetic ledgers by necessity, and I have said which. §9.
- **"the collapse rule is an open design question owned by E6"** — partly stale.
  DESIGN.md's *long-content* question was answered on 2026-07-30 (the ≤6 / >6
  degradation rule) and this renderer satisfies it. Only *depth* is still open. §8.
- **"E1's budget is <40ms of dira's own work"** — comfortably met (16.4ms at 200
  entries), but the number is almost entirely E1-L3's reconcile: this lane's own code
  is 0.49ms. A brief that framed 40ms as this lane's to spend would be measuring
  someone else's package. §7.
- **"any new package under `internal/` that you create and populate"** — taken
  literally and `internal/why` is entirely new, but it is worth flagging that
  `internal/index/indextest` is *designed* to be extended by this lane (its package
  doc says so). I extended it from the outside, by passing extra queries to
  `RunTwice`, rather than editing the file. That is the seam E1-L3 left, and it needed
  no edit to a contended file.
- Everything else — the `dec-0004` boundary, law 1, law 3, the read-through-the-cache
  constraint, the one-producer-two-renderers framing, and "never assert a status dira
  did not derive" — was accurate and load-bearing.
