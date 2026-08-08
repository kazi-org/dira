# E3-L1 report — conflict-detection contract + adversarial corpus

**Lane:** E3-L1, `docs/plan/lanes/E3.md`. **Status:** executed as data + prose
only, per the re-scope: no `.go` file, no `go.mod` change. Startable now,
**not green** until a later lane lands `internal/enforcer` as a Go package
(E0/E1 territory) and E3-L2 builds the matcher against the fixtures below.

## What shipped

| Path | What |
|---|---|
| `.dira/entries/dec-0014.md` | The decision: lexical matching in the binary, no model/agent gating the exit code, plus the two open sub-questions settled in prose |
| `internal/enforcer/testdata/ledgers/daemon/*.md` | 9 fixture entries exercising every row of the enforcement-set table |
| `internal/enforcer/testdata/corpus.yaml` | 43 labeled rows (24 conflict, 19 compliant near-miss) |
| `internal/enforcer/testdata/corpus.sha256` | sha256 freeze of the corpus, computed before any matcher exists |
| `docs/plan/tasks/E3-L1.md` | leaf tasks as-built |

Verified today, reproducibly, without writing any `.go` file:

```
$ go test ./schema/... -run 'TestLedgerValidates|TestTimestampsAreQuotedOnDisk'
ok  	github.com/kazi-org/dira/schema	0.1s
```

`schema/entry_test.go` already globs `.dira/entries/*.md`, so `dec-0014.md`
was validated against the **real**, already-wired schema harness — not an
approximation. It passed with no changes to `schema/entry.schema.json`.

The 9 fixture entries under `internal/enforcer/testdata/ledgers/daemon/` and
the corpus's internal cross-references (why_not substrings, near-miss shared
terms) were verified with `python3` (`jsonschema` + `pyyaml`, both present in
this environment), since no Go package exists yet at `internal/enforcer` to
run a real `go test` there. Exact reproduction commands:

```bash
# schema-validate every fixture entry
python3 - <<'EOF'
import json, yaml, re, glob, datetime
import jsonschema
schema = json.load(open("schema/entry.schema.json"))
validator = jsonschema.Draft202012Validator(schema)
def conv(v):
    if isinstance(v, (datetime.datetime, datetime.date)): return v.isoformat()
    if isinstance(v, dict): return {k: conv(x) for k, x in v.items()}
    if isinstance(v, list): return [conv(x) for x in v]
    return v
for p in sorted(glob.glob("internal/enforcer/testdata/ledgers/daemon/*.md")):
    fm = conv(yaml.safe_load(re.match(r'^---\n(.*?\n)---\n', open(p).read(), re.S).group(1)))
    errs = list(validator.iter_errors(fm))
    print(p, "OK" if not errs else errs)
EOF
```

The full corpus well-formedness check (row shape, entry existence,
why_not-substring presence, near-miss shared-term presence, distinct-entry
count, near-miss count) is the Python script whose logic is transcribed
exactly into the `TestCorpusWellFormed` spec below — I ran it against the
frozen `corpus.yaml` and it reports 43 rows, 9 distinct entries referenced,
19 valid near-miss rows, zero errors, before computing the sha256 freeze.

## `TestCorpusWellFormed` — specified in prose, for a later lane to implement

This lane creates no Go source. The test below must be written by whichever
lane next owns `internal/enforcer` (E3-L2, most likely, since its own `acc:`
line names this exact test).

**Path:** `internal/enforcer/corpus_test.go`
**Package:** `enforcer`
**Function:** `func TestCorpusWellFormed(t *testing.T)`

**Types** (unmarshal `corpus.yaml` with `gopkg.in/yaml.v3`, already a module
dependency):

```go
type corpusRow struct {
    ID              string   `yaml:"id"`
    Plan            string   `yaml:"plan"`
    Expect          string   `yaml:"expect"` // "conflict" | "compliant"
    Entry           string   `yaml:"entry,omitempty"`
    WhyNotSubstring string   `yaml:"why_not_substring,omitempty"`
    NearMissOf      string   `yaml:"near_miss_of,omitempty"`
    SharedTerms     []string `yaml:"shared_terms,omitempty"`
    Note            string   `yaml:"note"`
}
type corpus struct {
    Rows []corpusRow `yaml:"rows"`
}
```

**A "matchable text" helper** the test builds per referenced entry file
(this mirrors how `dira check` itself must read entries, per
`dec-0014`'s answer to "(b)" — frontmatter for decisions, frontmatter+body
for constraints — so the test and the eventual matcher agree on what
"literally appears" means): parse the entry's YAML frontmatter (reuse
`schema` package's `parseEntry`-equivalent logic, or a local equivalent),
take `title` + each `alternatives[].option` + each `alternatives[].why_not`
(YAML folded scalars already collapse internal newlines to spaces on
parse), then append the markdown body with every run of whitespace
collapsed to a single space. Concatenate with a separator. **Do not**
substring-match against the raw file bytes verbatim — a hard-wrapped
markdown paragraph splits a quoted sentence across a newline that carries no
semantic meaning, and a raw-bytes match would spuriously fail on
line-wrapped prose (this is not hypothetical: it is exactly why `dec-0042.md`
and `dec-0082.md` in this fixture set wrap their `why_not` prose across
multiple lines).

**Assertions, each its own `t.Run` subtest so one failure names its cause:**

1. **Freeze** (`t.Run("freeze", ...)`): read `corpus.yaml` raw bytes, compute
   `sha256.Sum256`, format lowercase hex, compare (after `strings.TrimSpace`)
   to the contents of `corpus.sha256`. Must run and fail loud before any
   other assertion — a matcher change that edits the corpus to pass must
   regenerate this file by hand first.
2. **Row count**: `len(rows) >= 40`.
3. **Row shape**: for every row — `ID` non-empty and unique across the
   file; `Plan` non-empty; `Expect` is exactly `"conflict"` or
   `"compliant"`; `Note` non-empty.
4. **Conflict-row shape**: `Expect == "conflict"` rows must not set
   `NearMissOf` or `SharedTerms`; must set non-empty `Entry` and
   `WhyNotSubstring`; `ledgers/daemon/<Entry>.md` must exist;
   `WhyNotSubstring` must be a literal (case-sensitive) substring of that
   entry's matchable text as built above.
5. **Compliant-row shape**: `Expect == "compliant"` rows must not set
   `Entry` or `WhyNotSubstring`.
6. **Distinct entries**: the set formed by `Entry` (conflict rows) ∪
   `NearMissOf` (compliant rows, where set) must have cardinality `>= 8`,
   and every id in it must correspond to an existing
   `ledgers/daemon/<id>.md`.
7. **Near-miss rows**: a compliant row counts toward the near-miss quota iff
   `NearMissOf` is non-empty, `len(SharedTerms) >= 2`, and every term in
   `SharedTerms` appears as a whole word (case-insensitive; `\b<term>\b` or
   equivalent tokenization) in **both** `Plan` and the `NearMissOf` entry's
   matchable text. The count of rows satisfying this must be `>= 15`.

**Explicit non-goals**, stated in the test's doc comment so nobody "fixes"
it to check them: `TestCorpusWellFormed` never runs a matcher and never
asserts anything about detection rate or false-positive rate. That is
E3-L2's test (its `acc:` line names ≥90% recall / zero false positives),
and this test's freeze check (#1) is what keeps that test honest — it
cannot pass by editing the corpus out from under it.

## Two open questions settled (per the lane prompt's instruction)

Both are recorded in `dec-0014.md` directly, not duplicated here in full —
summary:

- **(a) Constraint-level `revisit_if`:** decided yes, as a proposed additive
  schema field (`revisit_if` at the top level, meaningful chiefly for
  `constraint`). **Not applied** — `schema/entry.schema.json` is untouched
  by this lane (verified: `git diff --stat schema/entry.schema.json`
  reports no output). The exact JSON diff is in `dec-0014.md`.
- **(b) Body reads vs. frontmatter, against int-0002's latency posture:**
  decided — decisions match on frontmatter only (already-parsed YAML, no
  body parse); constraints match on frontmatter + body (their reasoning has
  nowhere else to live). This bounds the expensive read to the small,
  closed constraint set rather than the unbounded decision set.

## A documentation divergence found while reading the source material, reported rather than fixed

`docs/design.md` §7's canonical `dira check` example block shows **two**
conflicts for the input `"add a background daemon to track run state"`:

```
✗ conflicts with dec-0060 (accepted 2026-07-03)
    ...
✗ conflicts with cst-0004 (active)
    dira never requires a long-lived service
→ supersede dec-0060, or revise the plan
```

`README.md`'s equivalent block and `.agents/product-marketing.md` §6 — the
block §6 explicitly freezes as **the** demo asset E8 records a clip of, and
that `docs/plan/lanes/E3.md` (E3-L2's lane doc) says is "frozen by
marketing, not by taste" — show only **one** conflict, `dec-0060`, with no
`cst-0004` line at all.

This lane did not resolve which is authoritative; it built the fixture and
corpus so the **frozen, one-conflict version wins**: the fixture
ledger's `cst-0004` entry does not share enough distinctive vocabulary with
`"add a background daemon to track run state"` to trigger under the lexical
design in `dec-0014` (no shared content words between that phrase and
`cst-0004`'s text), so `row-001`'s single expected conflict (`dec-0060`
only) is consistent with README/marketing, not with `design.md` §7's fuller
illustrative block. `cst-0004` is still exercised as an enforced entry — via
`row-011` through `row-014` and `row-032`/`row-041` — against different,
unrelated plan strings, so the constraint-conflict path has real test
coverage without touching the frozen golden block.

**Recommendation:** whoever owns `docs/design.md` should either drop the
`cst-0004` line from §7's example to match the frozen demo, or the frozen
demo needs revisiting — but that decision belongs to whoever owns
marketing/demo sign-off, not to this lane.

## Honesty statement

Per the lane's own stop rule: this lane is fully plannable and was fully
executable as data-and-prose. Nothing here is blocked on an unresolved
question. It is **not green** — `go test ./internal/enforcer -run
TestCorpusWellFormed` cannot run today because no Go package exists at that
path, and won't be exercised for real detection until E3-L2 builds the
matcher. Both facts are stated plainly rather than worked around.
