# E1-L1 — storage interface, entry codec, 200-entry fixture

**Status:** all four `acc:` clauses green. Tree left dirty and uncommitted, as instructed.

---

## 1. The interface, and why it is this shape

`internal/ledger/store.go`:

```go
type Store interface {
	Get(ctx context.Context, id string) (*Entry, error)
	List(ctx context.Context) ([]EntryInfo, error)
	Create(ctx context.Context, e *Entry) error
	Put(ctx context.Context, e *Entry) error
	Delete(ctx context.Context, id string) error
}

type EntryInfo struct {
	ID      string
	Version string // opaque, backend-defined
}
```

Every method is expressed in entries. No path, no `*os.File`, no `fs.FS`, no
transaction, no batch, no lock — all of which are local-only concepts that the
`github` backend could not implement.

Three shape decisions worth your attention, because two of them go beyond what
the lane brief listed:

**`List` returns `[]EntryInfo`, not `[]string`.** The brief said "list … plus
whatever E1-L3 needs to enumerate for a reindex". What a reindex needs is
staleness detection, and the naive form of that is re-reading all 200 files to
compare them — the whole of `int-0002`'s budget spent to discover nothing
changed. `Version` is an opaque token that changes iff the content changed, so a
reindex is one directory read. It also happens to be exactly the shape the GitHub
Contents API returns for a directory (name + blob sha per file), which is the
best evidence available that the abstraction is drawn in the right place.

Measured: `List` over 200 entries is **2.7ms**; a full read of all 200 is
**28ms**. That ratio is the reason the field exists.

**`Create` was added, and is not in the brief's list.** E1-L2's acceptance line
requires 32 concurrent `dira log` invocations to produce 32 distinct ids with
zero collisions. That is unachievable without a compare-and-swap primitive at the
storage layer, and it is a native operation on both backends — `os.Link` locally,
a sha-less `PUT` on the Contents API. Leaving it out would have forced E1-L2 to
add a method to the interface, which is precisely the "E7 needs no change above
the interface" failure `dec-0005` was written to prevent. `TestConcurrentCreateHasExactlyOneWinner`
runs 16 racers and asserts exactly one winner.

**`Put` is unconditional; it does not take a version.** I considered
compare-and-swap on `Put` (GitHub's `PUT` needs the file's sha). I did not build
it, because I could not test it honestly on the local backend without a
read-modify-write window I would have had to hand-wave. Instead `Entry.Version()`
carries the token through `Get`, so E7's backend has the sha it needs to write
safely **with no signature change**. If E1-L3 or E7 decides `Put` must be
conditional, that is an added method or an added field, not a reshape.

`internal/ledger/ledgertest` holds the contract as a reusable suite. E7 adds one
call (`ledgertest.RunStoreContract`) and nothing else. That package exists
specifically so "github implements Store" cannot degrade to "github compiles".

---

## 2. The finding that shaped the codec, and one you should push back on if you disagree

**A canonical YAML emitter cannot reproduce this repo's ledger, and I measured
that before designing around it.**

The 26 entries in `.dira/entries/` contain 46 folded scalars (`why_not: >`),
hand-wrapped. I greedy-re-wrapped all 46 at every width from 74 to 95 columns:

| width | reproduced |
|---|---|
| 80 | 8 / 46 |
| 81 | 10 / 46 |
| **82** | **19 / 46** *(best)* |
| 83 | 18 / 46 |
| 85 | 16 / 46 |

I also tested `yaml.v3`'s own node-preserving round-trip (decode to `yaml.Node`,
re-encode with indent 2): it preserves the folded *style* but emits the whole
scalar on one long line, so it reproduces none of them.

So byte-identical round-trip over the real ledger was reachable in exactly three
ways: rewrite `.dira/entries/` into canonical form (forbidden, and it would
reflow every paragraph in the repo), cache raw bytes (a cheat that makes the
acceptance line vacuous), or **preserve presentation on read**.

I built the third. `internal/ledger/style.go` records, per scalar value, how that
value was written: quoting style, block indicator, and the block's source lines
dedented. `Encode` reuses it when the value is unchanged and falls through to
canonical emission when it is not.

**This is a product requirement, not a test accommodation.** `dec-0002` sells
"a PR touching a decision shows a legible diff". Without the memo, `dira log
--edge` adding one edge to `dec-0002` would re-fold every paragraph in the file
and produce a forty-line diff. `TestEditingOneFieldRewritesOnlyThatField`
asserts a title change alters exactly one line and an added edge inserts exactly
three lines with every original line surviving verbatim and in order.

**How I kept it from being a bytes cache** — because a style memo is one bad
decision away from being exactly that:

- The memo holds presentation only. Values, keys, ordering, indentation,
  sequence layout and the markdown body all come from the parsed model.
- It is keyed by value, so a caller's edit no longer matches and falls to
  canonical emission.
- `TestEditingOneFieldRewritesOnlyThatField` cannot pass for a raw-bytes cache.
- `TestCanonicalEmissionReparsesToTheSameEntry` runs all 26 real entries through
  the write path with the memo *deleted* and asserts every value survives. Bytes
  differ (that is the point); meaning does not.
- Captured blocks are only kept if reconstructing them provably reproduces the
  parser's value (`reconstruct` in `decode.go`). A block with a corner the
  capture does not model — explicit indentation indicator, interior blank line,
  more-indented line — records nothing and re-wraps instead. It reflows text; it
  never changes it.

**Push back here if you disagree:** the alternative I rejected was accepting
semantic round-trip instead of byte round-trip and letting dira reflow prose on
every write. That is a smaller codec. It also makes `dec-0002`'s reviewable-diff
claim false in the first release, so I did not take it.

---

## 3. Verbatim command output

```
$ go build ./...
  exit=0

$ go vet ./...
  exit=0

$ golangci-lint run ./...
0 issues.
  exit=0

$ go test -count=1 ./...
ok  	github.com/kazi-org/dira/cmd/dira	5.894s
ok  	github.com/kazi-org/dira/internal/ledger	3.178s
ok  	github.com/kazi-org/dira/internal/ledger/fixture	2.677s
?   	github.com/kazi-org/dira/internal/ledger/ledgertest	[no test files]
ok  	github.com/kazi-org/dira/internal/ledger/local	0.471s
ok  	github.com/kazi-org/dira/schema	0.366s
  exit=0
```

```
$ go test -race -count=1 ./...
ok  	github.com/kazi-org/dira/cmd/dira	7.305s
ok  	github.com/kazi-org/dira/internal/ledger	5.009s
ok  	github.com/kazi-org/dira/internal/ledger/fixture	4.554s
?   	github.com/kazi-org/dira/internal/ledger/ledgertest	[no test files]
ok  	github.com/kazi-org/dira/internal/ledger/local	1.793s
ok  	github.com/kazi-org/dira/schema	1.607s
```

```
$ python3 scripts/coverage.py
obligations extracted : 54
registered            : 54
uncovered             : 0
COVERAGE PASS — every obligation has a disposition.
  exit=0

$ python3 scripts/privacy-lint.py
PRIVACY LINT PASS — cst-0003 enforced by 4 checks.
  exit=0
```

The acceptance command, with executed counts rather than a bare `ok`:

```
$ go test -count=1 -v ./internal/ledger/... | grep -cE '^(    )*--- PASS'
286
$ go test -count=1 -v ./internal/ledger/... | grep -cE '^(    )*--- (FAIL|SKIP)'
0
```

36 top-level tests, 286 assertions/subtests, zero failures, zero skips.

---

## 4. `acc:` clause by clause

**(a) Byte-identical round-trip over every `.dira/entries/*.md` plus every
fixture entry — GREEN.**
`TestRoundTripIsByteIdentical` (26 subtests, one per real entry, guarded to fail
if fewer than 20 files are found) and `TestFixtureRoundTrips` (200 entries).
Supported by `TestDecodeIsIdempotent`, `TestBodyIsPreservedVerbatim` (7 awkward
bodies including one containing its own `---`), and `TestUnknownFieldIsRejected`.

**(b) `created`/`updated` survive as RFC3339 strings — GREEN.**
`TestTimestampsSurviveAsStrings`, six subtests: an unquoted input parses to the
string, the written form is quoted, the written form re-reads as a `string` and
not a `time.Time`, a quoted input stays quoted, E0's
`schema/testdata/valid/unquoted-timestamp.md` fixture, and a malformed timestamp
is rejected. The `time.Time` assertion decodes the emitted frontmatter into
`map[string]any` — what every naive reader does, including the schema validator —
so the coercion has nowhere to hide.

Structurally, `Decode` walks the `yaml.Node` tree and reads each scalar's raw
source text, so a `!!timestamp`-tagged node is observed and ignored rather than
resolved. A `time.Time` is never constructed anywhere in the package.

**(c) Import-boundary test — GREEN.**
`TestNoFilesystemImportsAboveTheBackend` fails when any package in this module
directly imports `os`, `io/fs`, `io/ioutil`, `path` or `path/filepath`, with an
allowlist of exactly two entries: `internal/ledger/local` (the backend) and
`cmd/dira` (`os` only, for stdio and exit — not `path/filepath`, because locating
a ledger on disk belongs in the backend).

`TestTheImportBoundaryHasTeeth` is the other half: it asserts the backend really
does import `os` and `path/filepath`, and that no allowlist entry is stale. A
rule that forbids something nobody does is indistinguishable from a broken rule.

**(d) Fixture byte-identical across two runs from the same seed, all 200 passing
E0's validator — GREEN.**
`TestGenerationIsReproducible` compares encoded bytes (not structs) across two
runs. `TestDifferentSeedsDiffer` stops that from passing vacuously.
`TestDigestIsStable` pins the exact output. `TestEveryEntryValidatesAgainstTheSchema`
runs all 200 through the JSON Schema validator — the published contract, not
dira's reading of it. Verified stable across separate processes, five runs.

---

## 5. Fixture: generator, not 200 committed files

**Decision: seeded generator, materialised into a temp dir, with a committed
digest constant.**

200 committed files add review noise to every future diff and every grep of the
repo, forever. The real cost of a generator is that its output can move silently
and drag E1-L3's cache tests and E1-L4's golden why-chains with it. That cost is
paid off by the digest:

```go
const wantDigest = "7e46702abd119e4b09355ececf86e80f798b28024dfe75df24e599101f200f2d"
```

Changing the generator fails `TestDigestIsStable`, and updating the constant is
one deliberate line in a diff — the same review signal a committed fixture gives,
in one line instead of two hundred files.

Composition (`TestTheLedgerIsRealistic` asserts all of this): 16 intents, 14
constraints, 90 decisions, 30 questions, 50 notes. Every kind present, every one
of the five edge types present, every non-`realized_by` edge target resolving to
an entry that exists, every `realized_by` target a `kazi:` artifact, every
decision carrying alternatives, some carrying `revisit_if`, open questions
carrying `blocks` edges, at least one entry `private: true`, at least one
carrying an `adr` path, and superseded entries whose state was actually flipped
by the `supersedes` edge pointing at them.

`private` and `adr` are placed by stride, not by random draw. A 1-in-23 chance
produced a fixture with neither at small sizes, which is a test that silently
stops testing — `TestSmallerFixturesAreStillRealistic` caught it.

---

## 6. Latency

Measured in-process, spawn excluded, over the 200-entry fixture on this machine:

| operation | median |
|---|---|
| `List` (200 entries, staleness only) | **2.7ms** |
| `Decode` × 200, no I/O | **13.1ms** |
| full read: `List` + `Get` × 200 | **28.3ms** |

`TestFullLedgerReadIsWithinBudget` fails above 150ms and logs the real number.
The ceiling is deliberately looser than E1's restated <40ms: this is the *whole*
ledger decoded, which is strictly more than any read path does, and CI hardware
is slower. It exists to catch the class of regression that makes 40ms
unreachable, not to be the target. E1-L6 owns the end-to-end budget.

One optimisation landed rather than being deferred, since `int-0002` calls
latency a design constraint: `Get` was doing `os.ReadFile` then `os.Stat`, two
path resolutions per entry. Now it opens once and fstats the handle — 31.4ms →
28.3ms over 200 entries.

The timing test skips under `-race` (via a build-tagged `raceEnabled` constant)
because instrumentation costs ~7× the work being measured — it reported 201ms.
Skipping with a stated reason beats either a lying budget or a silently
build-tagged-out test.

---

## 7. Two bugs the tests found, both real

**A value ending in `\n` lost its last character.** `Encode` wrote any
newline-containing value as a literal block; clip chomping plus the decoder's
trailing-newline strip meant `"trailing newline\n"` came back as
`"trailing newline"`. One character of silent data loss.
`TestQuotingSurvivesAReparse` found it. Fixed: `literalSafe` refuses the block
form for values a block cannot express exactly, and those fall through to
double quotes, which can express any string.

**Folding could eat whitespace.** A folded scalar rejoins its lines with single
spaces, so a value containing two consecutive spaces would come back changed.
`foldable` now refuses to fold any value where `strings.Join(strings.Fields(v), " ") != v`.
That is a silent edit of the user's text on a write they did not ask for.

---

## 8. Red-before-green verification

I broke each guarantee and confirmed the test fails, then restored:

| break | result |
|---|---|
| `timestamp()` emits unquoted | `FAIL` — *"created round-tripped as time.Time … so it was written unquoted"* |
| `Encode` ignores the style memo | `FAIL` — 15 of 26 entries stop round-tripping |
| add `os` import to `internal/ledger` | `FAIL` — *"internal/ledger imports "os""* |

`go test -count=1 ./...` green again after restoring.

---

## 9. Things I did that you should sanity-check

**I landed E0-L2's schema export.** `acc:` (d) requires the fixture to pass
"E0's schema validator", but that validator lived in `schema/entry_test.go` as
unexported test-only code — unreachable from another package. `schema/entry_test.go`'s
own package doc says E0-L2 will `go:embed` the schema and export the parsing.
I did that: `schema/schema.go` adds `//go:embed entry.schema.json`, `NewValidator()`,
`Validator.Validate([]byte) error` and `SplitFrontmatter`. The helpers moved out
of the test file into the source file in the same package, so **every existing
test body is unchanged** — the diff to `entry_test.go` is a deletion plus two
call-site adjustments. Nothing about the schema itself changed.
`schema/validator_test.go` covers the new exported surface, including that the
embed is not empty.

If E0-L2 is assigned to someone else, this is the collision.

**I touched `cmd/dira/build_test.go`.** The lane brief required replacing E0-L1's
no-non-stdlib clause with an allowlist. `TestCommandPathHasNoThirdPartyDependencies`
became `TestCommandPathLinksOnlyAllowedModules`, keyed by module path, plus
`TestTheAllowlistIsNotStale`.

**The allowlist is empty today, deliberately.** Nothing in `cmd/dira` imports
`internal/ledger` yet, so the command path genuinely still links nothing, and
pre-authorising `yaml.v3` before anything needs it is a hole rather than a
convenience. The staleness test would fail on a pre-added entry. **E1-L2 adds
`"gopkg.in/yaml.v3"` when it wires the first command** — flagging this so that
lands as a decision rather than a surprise. `cmd/dira/` may be another lane's;
the diff is confined to that one file.

**Package layout differs from the brief's provisional paths.** The fixture is at
`internal/ledger/fixture`, not `internal/fixture`, so `go test ./internal/ledger/...`
— the literal acceptance command — covers it. Added `internal/ledger/ledgertest`
for the shared Store contract.

**`Entry.Validate` is a second expression of the schema.** It has to be: the
binary cannot compile a JSON Schema document on every invocation inside
`int-0002`'s budget. `schema_test.go` holds the two in agreement by *reading*
`entry.schema.json` — enum sets, per-kind `allOf` state rules, `id` and `ref`
patterns — and by asserting that over E0's 17 invalid fixtures plus all valid
corpora, the schema and the codec never disagree on the verdict.

I did not touch `.dira/entries/`, `docs/roadmap.md`, `docs/coverage.md`, or any
gate script. I did not create a Go package under `internal/enforcer/testdata/`.

---

## 10. Left unimplemented, and one thing E1-L3 must decide

Not built, and deliberately: id allocation (E1-L2), ledger discovery / walking up
for `.dira` (no command needs it yet — building it now would be an unpopulated
API), any user-facing verb, the SQLite cache, ADR mirror writing, tier
resolution.

**One decision I am handing to E1-L3 rather than making silently.**
`EntryInfo.Version` on the local backend is `mtime-nanos + size`. That is the
standard staleness heuristic — git, make and rsync all use it — and it is what
keeps `List` at 2.7ms instead of 200 file reads. But it is a heuristic: an
in-place edit that preserves both size and mtime is invisible to it. On a
filesystem with coarse mtime granularity that window is not vanishingly small.

E1-L3's acceptance line says a cache/file disagreement must resolve to the file,
and `dec-0002` says the files win. If E1-L3 wants that to be a guarantee rather
than a near-certainty, `Version` must become a content hash — which costs ~13ms
per full ledger scan. **The interface supports either without change**
(`Version` is opaque and backend-defined; it is one function in `local.go`), but
E1-L3 should choose explicitly rather than inherit my default.

## 11. Nothing in the brief turned out to be wrong

The timestamp landmine, the "one file per entry", the "no path above the
interface" constraint and the latency framing were all accurate and all load-bearing.

The one instruction that could not be followed as literally written was
"reuse E0's schema validator; do not duplicate it" — it was not reusable from
another package until it was exported, which is §9.
