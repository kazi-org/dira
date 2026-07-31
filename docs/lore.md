# Project Lore

Append-only log of gotchas, invariants, and landmines. Unlike
docs/devlog.md (per-session investigation records), entries here
describe rules that must always hold or things that must never
happen again. Entries are never reordered and never pruned.

Retrieval: grep by tag, e.g. `grep -n "#gate" docs/lore.md`. New
entries are appended at the end and receive a stable L-NNNN ID.
See lore/SKILL.md for the entry format.

---

## L-0001: Watch the gate go red, then watch it go green -- the green side is the one nobody checks

**Tags:** #gate #testing #critical
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** A gate is evidence only once both sides have been observed: it goes red
on a constructed defect of the class it claims to catch, AND it goes green on the
untouched correct baseline.
**Why:** Two rules, and the second subsumes the first. (1) A check that cannot
fail the premise is not evidence for the premise. (2) A check that cannot *pass*
the correct case is equally worthless, and its failure is invisible from the red
cases alone -- a check that fails on everything looks identical to a check that
works. Rule 2 is the one nobody applies. E6-L1 proved it by breaking and
restoring `tokens-doc-sync` in four steps: the three red cases all looked
perfect, and step 1 -- the untouched baseline, which should have been green --
came back RED. The canonical sentence wraps across two lines in DESIGN.md, so a
literal `includes()` could never match it. That check would have failed a correct
document forever, and no amount of rewriting the paragraph would have fixed it.
The fix collapses whitespace before comparing (commit 28888fc; verbatim
transcript in `docs/decisions-pending/E6-L1-report.md` section 2d). A second,
cosmetic bug surfaced the same way. Rule 1 has five instances in this repo, each
a check that passed while blind to the exact thing it certified.
(1) `scripts/coverage.py` could not see that its own
input, `docs/plan.md`, was untracked -- see L-0005. (2) `contrast.mjs` certified
token pairs while every chip actually rendered on a `color-mix` tint of itself;
five chips sat at 3.0-4.3:1 with the gate green -- see L-0002. (3) The pixel
harness cannot see a serif fallback it will never experience on macOS, which
costs up to 100% of pixels on Linux -- see L-0006. (4)
`TestTheAllowlistIsNotStale` asked `go list -deps` about the host GOOS only, so
an allowlist exactly right on a Mac was stale on Linux; CI's first ubuntu run
caught it (commit d890bab) -- see L-0008. (5) `dira why`'s golden files agreed
byte-for-byte with a ledger that contradicted itself -- see L-0003. A check that
validates a declaration rather than a result is indistinguishable from no check.
**Trigger:** Any gate landed without running its supposed-to-pass baseline
explicitly; any string comparison against prose that wraps, without collapsing
whitespace first; any assertion of the shape "X survives transformation T" where
the input may contain no X; any check whose scope is "whatever machine ran it".
The current response is `docs/design/scripts/gates.mjs`, whose **exit code 3**
means "a gate PASSED but its negative control did not trip -- the gate is
blind", and `docs/design/fidelity/TOLERANCE.md`, which publishes the measurement
rather than the number. Note what exit 3 does *not* cover: it watches the red
side only, so rule 2 is still on the author.

## L-0002: color-mix(in oklab) cannot be approximated in sRGB

**Tags:** #css #contrast #gate #critical
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Resolve any composited colour by painting to a canvas and reading the
pixel; only parse literal `#rrggbb` out of `tokens.css` or DESIGN.md.
**Why:** Chromium returns `color-mix(in oklab, ...)` from `getComputedStyle` as
an `oklab(...)` string. A gate that parsed those three floats as if they were RGB
once reported a legible paragraph at **1.31:1** -- a number no real surface can
produce, passed straight through as a result. Every chip in this design system
sits on a `color-mix(in oklab, <own hue> N%, transparent)` tint of its own
colour, so the composited value is the only one a reader ever sees.
**Trigger:** Any new contrast or colour check that calls `getComputedStyle` and
assumes an `rgb()` return. `contrast.mjs` green is a token result, not a contrast
result; `contrast-rendered.mjs` is the authority for any surface with a tint.

## L-0003: `supersedes` flips the target's state; it does not mean "narrows"

**Tags:** #ledger #schema #invariant
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Use `supersedes` only when the target genuinely becomes
`state: superseded`; for narrowing, amending, or answering, use `informs`.
**Why:** `docs/design.md` section 4.1 makes `supersedes` flip its target to
`superseded`. Three entries in dira's own ledger carried the edge while the
target still read `accepted` or `answered` -- `dec-0012 -> dec-0005`,
`dec-0016 -> dec-0012`, `dec-0011 -> qst-0001`. In every case the author meant
*narrows* or *answers*. The record told two stories at once and `dira why`
rendered both: `accepted` on the subject row, `superseded by dec-0016`
underneath. `entry.schema.json` cannot catch this -- the edge and the state are
independently valid. All three are now `informs`, and the ledger currently holds
no `supersedes` edges at all.
**Trigger:** Reaching for `supersedes` because no edge type says "narrows". That
gap is real and tracked as the open `qst-0006`; `informs` is the honest
placeholder until it is closed. A question can never be superseded -- the schema
gives questions only `open` and `answered`.

## L-0004: kazi's `--json` has two bucket enums, and `done`/`running` are never arrays

**Tags:** #kazi #json #integration #gotcha
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Read `by_repo` as the three-value enum and `totals.rows[]` as the
five-value one; never expect `buckets.done` or `buckets.running` to exist.
**Why:** `portfolio.ex:38` defines `@type bucket :: :in_progress | :stuck |
:complete` and types `by_repo` against it. The five-value `five_bucket` governs
only `totals.rows[]` and the top-level `todo`/`blocked` arrays. `portfolio_json/1`
emits `buckets.todo` and `buckets.blocked` only -- done and running survive as
*counts*, not as lists. An earlier version of `dec-0008` claimed the enum was
"exactly `done | running | blocked | todo | planned`, which maps cleanly", and
that claim had to be corrected in `dec-0008`, `docs/design.md` and
`docs/plan.md` together. Two of dec-0004's six buckets are not derivable from
portfolio alone.
**Trigger:** Writing a join against kazi output from the prose description of the
contract rather than from `portfolio.ex`. Integration is only ever through the
public `--json` contract, never kazi's internals.

## L-0005: A global gitignore on this machine silently swallows real project files

**Tags:** #git #tooling #critical
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Before trusting that a written file is in the repo, confirm it with
`git ls-files --error-unmatch <path>`; never infer tracking from a clean
`git status`.
**Why:** `core.excludesfile` is set to `~/.gitignore`, which is a project-level
ignore file that migrated into the global slot. It contains bare, unanchored
patterns -- `plan.md`, `todos.md`, `bugs.md`, `issues.md`, `vendor`, `bin`,
`tmp`, `artifacts`, `.claude`, `GEMINI.md` -- each of which matches at any depth
in every repo on this machine. `docs/plan.md`, the L0 program plan, was written
and referenced repeatedly and was never committed for this repo's entire history.
The failure is invisible by construction: `git status` shows nothing, so the file
simply does not exist to anyone else. It is now rescued by a `!docs/plan.md`
negation in the repo `.gitignore` plus a force-add, and `scripts/coverage.py`
asserts its own sources are tracked (`untracked_sources()`).
**Trigger:** Any file named `plan.md`, `notes.md`, `todo.md`, `bugs.md`,
`issues.md`, or any directory named `bin`, `tmp`, `vendor`, `artifacts`, or
`.claude`, in any repo on this machine. `git check-ignore -v <path>` names the
file and line that did the ignoring -- that is the whole diagnosis.

## L-0006: Pixel baselines are not portable across machines

**Tags:** #design-fidelity #platform #gate #invariant
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Regenerate the baseline and take the capture in the same run and the
same environment; never widen the tolerance to cross a machine boundary.
**Why:** Two measured arms sit three to four orders of magnitude above the
enforced tolerance (`0.00033%` of pixels, `1.6%` of any 16x16 block).
`-webkit-font-smoothing: auto` alone costs 1.07-4.64%. The Palatino stack falling
through to the generic serif -- what a stock Linux install renders -- costs up to
**100%**. No number that still catches a 2px radius change could absorb either.
`docs/design/renders/` is gitignored deliberately so a baseline cannot be
hand-committed and compared across machines.
**Trigger:** Committing a render, or reusing a baseline captured on another host
or another Playwright version. Every figure in TOLERANCE.md is darwin x64;
elsewhere the noise arms may not be zero and the derivation must be re-run, not
assumed.

## L-0007: The pre-commit hook is not a CI proxy

**Tags:** #ci #hook #testing #gotcha
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Never treat a green `hooks/pre-commit` as evidence the change is CI-clean.
**Why:** The hook runs `go test ./...` and nothing else -- no `go build`, no
`go vet`, no `gofmt -l`, no `-race`, and neither contrast script.
`internal/index/race_{on,off}_test.go` and
`internal/ledger/fixture/race_{on,off}_test.go` are `//go:build race`, so they are
compiled out entirely under the hook. A green pre-commit certifies nothing
whatsoever about the files whose only purpose is race detection.
**Trigger:** Concluding "tests pass" from the hook before pushing. Run the full
gate set; CI is the first place the race-tagged files exist at all.

## L-0008: A dependency check must union across every platform the binary ships

**Tags:** #ci #platform #gate #critical
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Any `go list -deps` assertion must run once per shipped GOOS and
combine the results; asking about the host only makes the test a report on
whoever ran it.
**Why:** `modernc.org/sqlite` pulls `go-isatty` and `go-strftime` into the
command path on darwin and not on linux. `TestTheAllowlistIsNotStale` therefore
passed on macOS -- where everything in this repo had ever run -- and failed on
ubuntu the first time CI executed it (commit d890bab). A module genuinely
required on one GOOS must not be deleted from the allowlist because the machine
you happen to be on does not link it; that breaks the other platform.
`buildPlatforms` in `cmd/dira/build_test.go` is now `{"darwin", "linux"}` and the
staleness half fails only when a module is linked on none of them.
**Trigger:** Adding, removing, or auditing an entry in `allowedModules`, or
writing any new check whose answer depends on GOOS, GOARCH, or the installed
toolchain.

## L-0009: `mtime + size` is blind to the one ledger edit that matters

**Tags:** #ledger #cache #invariant
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** `EntryInfo.Version` is the git blob object id over the entry file.
Never optimise it back to a stat-based heuristic.
**Why:** `state: accepted` and `state: rejected` are the same number of bytes, and
`rsync -a`, `cp -p` and `tar -p` all preserve mtime. Under mtime+size the
reversal of a decision -- the single most important edit the ledger can carry --
is permanently invisible to the cache, with no reindex able to notice.
`dec-0002` promises the files win, not that they usually win. The cost estimate
that originally justified mtime+size was also wrong: it had folded in the YAML
parse, and the real marginal cost of hashing is 4.7ms over a 200-entry ledger.
**Trigger:** A latency lane looking for something to shave. The value is
reproducible with `git hash-object .dira/entries/<id>.md`, and E7's github
backend gets the same value free from the Contents API -- that equality is what
lets a ledger change backend without a reindex.

## L-0010: A roadmap edit creates a coverage obligation in the same change

**Tags:** #process #gate #gotcha
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Edit `docs/roadmap.md` and `docs/coverage.md` together, or the gate
goes red.
**Why:** `scripts/coverage.py` mechanically extracts obligations from three
places: `## Blocked` rows and upstream asks in `docs/roadmap.md`, and every
`revisit_if` in `.dira/entries/`. Each needs a disposition registered in
`docs/coverage.md` or the script exits non-zero. E1-L3 tripped this by landing
one new entry carrying five `revisit_if` triggers -- six uncovered obligations,
gate red, for a change that was otherwise correct.
**Trigger:** Adding a Blocked line, an upstream ask, or any entry with a
`revisit_if`. Both files are shared-write across a wave: take the claim locks or
make the lead the sole writer.

## L-0011: The L2 prompt files instruct an agent to write tasks, not code

**Tags:** #process #agents #gotcha
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** When dispatching from `docs/plan/prompts/L2-*.md`, state explicitly
whether you want a task file or an implementation, and re-check the prompt's
factual claims against the tree first.
**Why:** Every one of those files opens with "You are an L2 planner. You produce
**tasks**, not code, and not lanes." An agent handed one verbatim produces a task
file and no implementation. Several are also stale: they assign the writing of
gates that already exist and pass, so an agent following `L2-E8-L2.md` literally
will rewrite a green `contrast.mjs`. `docs/plan/lanes/E3.md:26` lists E3-L1's
prerequisite as "nothing buildable" while its own `acc:` line requires
`go test ./internal/enforcer`.
**Trigger:** Pasting a planner prompt into an execution lane. The prompts are a
planning artifact and were written before the tree they describe.

## L-0012: dec-0014's phrase rule inverts if read as raw substrings

**Tags:** #enforcer #matching #gotcha
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** The enforcer's multiword signal compares normalised **content words**,
never characters.
**Why:** `dec-0014` offers "add a background daemon to track run state" as the
case its exact-phrase rule catches trivially against dec-0060's alternative
"a daemon". Read as raw strings that is false and false in the worst direction:
the substring "a daemon" does not occur in that sentence -- `background` sits
between the words -- while it does occur verbatim in corpus row-039, "a daemon
was considered and rejected", a compliant near-miss. A character-level matcher
fires on the sentence documenting the decision and stays silent on the one
relitigating it. The entry's prose still describes the literal reading.
**Trigger:** Reimplementing or "simplifying" the matcher from dec-0014's prose.
The frozen corpus at `internal/enforcer/testdata/corpus.yaml`, checksummed before
any matcher existed, is the arbiter; it may never be edited to make a failing row
pass.

## L-0013: `dira check` exit 2 means a verdict only because check routes its own errors to 1

**Tags:** #cli #exit-codes #enforcer #critical
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** `dira check` must never return a `usageError`; its own misuse goes to
exit 1 via `checkMisuse`.
**Why:** `cmd/dira/main.go` maps usage errors to exit **2** for every command,
and E3 fixes exit 2 as "at least one cited conflict". Two halves were needed and
only one is obvious. The `ExitCode() int` branch in `(*app).main` makes 2
*reachable* for a verdict; `checkMisuse` sending this command's own flag errors
to 1 is what makes 2 mean *only* a verdict. With the branch alone, a cited
conflict and a mistyped flag would both exit 2, and a hook would block a commit
for a typo. The whole point of the code split is that a caller can tell "you
contradicted yourself" from "dira is broken" and fail open on the second.
**Trigger:** Adding a flag to `check`, or refactoring its argument handling to
use the shared `usagef` helper "for consistency". A caller must never treat 1 as
a verdict.

## L-0014: The enforcer fixture ledger is not a ledger dira can open

**Tags:** #enforcer #testing #gotcha
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Copy `internal/enforcer/testdata/ledgers/<name>/` into a temporary
`.dira/entries/` before pointing a command at it.
**Why:** The fixtures are a flat directory of `*.md`, because that is the shape
`corpus.yaml` references and the corpus is frozen by sha256. `local.Open` wants a
`.dira` with an `entries/` inside it, and `local.Find` walks *up* -- so
`dira check -C internal/enforcer/testdata/ledgers/daemon` silently grades against
**this repository's own `.dira`**, not the fixture. There is no error; the answer
is just about the wrong ledger.
**Trigger:** Any new test or script that points a dira command at a testdata
ledger path directly. Copying also keeps the checksummed fixture safe from a
command that writes a cache.

## L-0015: jsonschema/v6 has format assertion off by default

**Tags:** #schema #validation #gotcha
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Call `c.AssertFormat()` on every `jsonschema.Compiler`, and keep the
non-RFC3339 negative fixture that pins it.
**Why:** Format is annotation-only in draft 2020-12, so without the call
`created: "yesterday"` validates cleanly against a schema declaring
`"format": "date-time"`. The only thing holding the call in place is the negative
fixture that fails without it -- delete the fixture and the assertion can be
removed with every test still green.
**Trigger:** Compiling a new schema (for example `schema/check.schema.json`), or
pruning "redundant" invalid fixtures.

## L-0016: An unquoted YAML timestamp is a `time.Time`, and its guard is double-masked

**Tags:** #ledger #yaml #codec #gotcha
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** `Entry.Created` and `Entry.Updated` are strings; always quote
timestamps on write.
**Why:** yaml.v3 resolves an unquoted RFC3339 scalar to the `!!timestamp` tag and
hands back a `time.Time`, which is not a JSON type -- a validator handed one
reports `invalid jsonType time.Time` at `/created`, pointing at a field that is
in fact correct. This has broken validation in this repo once already. The guard
is double-masked: deleting the explicit `case time.Time` in `jsonValue` does not
turn the suite red, because `json.Marshal` flattens it through `MarshalJSON`
anyway. Two independent mechanisms cover one rule, so removing either looks safe.
**Trigger:** "Simplifying" the codec's type switch, or storing a timestamp as
`time.Time` because that is the natural Go type.
`TestJSONValueFlattensTimestamps` is the one test that is genuinely red without
the explicit case.

## L-0017: The browser gates do not run from a fresh clone

**Tags:** #tooling #design-fidelity #gotcha
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Do not treat any Playwright-backed gate as reproducible until this repo
has a `package.json` and a lockfile.
**Why:** There is no `package.json` and no lockfile. `node_modules` at the repo
root is a **symlink into a session scratchpad** holding Playwright 1.62.0, and
`node_modules` is gitignored. `contrast-rendered.mjs`, `pixeldiff.mjs` and the
render harness therefore run on exactly one machine; CI restates
`PLAYWRIGHT_VERSION: "1.62.0"` in YAML rather than resolving it from a manifest.
A fresh clone gets nothing, and the failure looks like a missing module rather
than a missing dependency declaration.
**Trigger:** Adding a gate that shells out to node, or citing a browser-measured
number as reproducible. Add `package.json` plus `npm ci` first.

## L-0018: `gh api git/ref/tags/<tag>` returns an annotated tag SHA, not a commit

**Tags:** #ci #github-actions #gotcha
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Pin a GitHub Action with
`gh api repos/<owner>/<repo>/commits/<tag> --jq .sha`.
**Why:** For an annotated tag, `git/ref/tags/<tag>` returns an object of type
`tag`, whose SHA is the tag object rather than the commit. That value is not a
valid `uses:` target and the workflow fails at resolution, not at run.
`golangci/golangci-lint-action` v9 is annotated this way; it is pinned in
`.github/workflows/ci.yml:141` as
`golangci/golangci-lint-action@ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a # v9.3.0`.
**Trigger:** Adding or bumping any pinned third-party action.

## L-0019: Three loose substring searches are not one assertion

**Tags:** #gate #testing #invariant
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** Generate one canonical string from the machine-readable source and
assert the document carries it; never check that N figures each appear
*somewhere*.
**Why:** `tokens-doc-sync` used to verify that `0.00033%`, `1.6%` and `4/255`
each appeared somewhere in DESIGN.md. Three independent searches are satisfied
by a document that states all three figures in a sentence meaning something else
entirely, so the clause was unfalsifiable. It now builds one sentence from
`docs/design/fidelity/tolerance.json` and asserts the document carries it
(commit 28888fc), which gives a single truth condition and lets the failure print
the exact string required. The same file records how weak a bare search can get:
`design.includes("4")` passed trivially, because "r4", "4.5:1" and "42 pairs" all
contain it -- figures are now matched with their units.
**Trigger:** Any "the docs mention X" check. The upgrade also separates *deleted*
from *drifted*: previously a document stating a tolerance the gate does not
enforce was indistinguishable from one that omitted it, and drifted is the
dangerous case -- it reads as authoritative and is wrong.

## L-0020: Exit 2 means "the record refused you" in E3's verbs and "you mistyped" everywhere else

**Tags:** #cli #exit-codes #enforcer #critical
**Date:** 2026-07-30
**Repo:** kazi-org/dira

**Rule:** `dira check` and `dira supersede` both route their own argument mistakes
to exit **1**, so exit **2** from either means only that the ledger said no.
`dira log` does the opposite on purpose and must not be "fixed" to match.
**Why:** L-0013 recorded half of this for `check`. `supersede` needed the same
split for the same reason and the split is now the *pair's* contract, not one
command's quirk: a hook wrapping both can fail open on 1 and surface 2 without
knowing which verb it called. Concretely — a cross-ledger supersede (cst-0003), a
kind with no `superseded` state, an entry already replaced and a staged
replacement are 2; a bad flag, a malformed id, a missing entry, an unreadable
ledger and a failed write are 1. Both cannot be made consistent with `log`, which
maps its own flag errors to 2 via the shared `usagef`: `log` has no policy
refusals, so it never needs 2 to mean anything else, and the verb that shares a
caller with `check` is `supersede`. The command's `-h` states this so a script
author does not have to read the source.
**Trigger:** Adding a flag or a refusal to `supersede`, or routing either
command's errors through `usagef` "for consistency with the rest of the binary".
Ask first which of the two things the new refusal is: a rule in the record saying
no (2), or dira never getting far enough to ask (1).

## L-0021: This repo's task ids are not claimable without a `T-` prefix

**Tags:** #tooling #apply #pool #gotcha
**Date:** 2026-07-31
**Repo:** kazi-org/dira

**Rule:** Claim pool tasks as `T-<task-id>`, e.g. `T-E1-L6-T1`, never the bare
`E1-L6-T1`.
**Why:** `claim.sh` validates the id against a fixed set of shapes —
`T<n>.<n>`, `T<AA><n>.<n>`, `S<n>.<n>`, `S-*`, `T-*`, `R-<slug>`, `M-<slug>`.
This repo's plan uses an epic-lane-task scheme (`E1-L6-T1`) that matches none of
them, so **every** task returns `BLOCKED`, and `BLOCKED` means "do not dispatch".
`/apply --pool` therefore appears to run correctly and claims nothing. The
failure is quiet: a wave of seven candidates produced seven `BLOCKED` lines and
no work, which reads as "the pool is empty" rather than "the ids are invalid".
The `T-*` shape is already accepted, so prefixing satisfies the validator with no
script change and no plan rewrite.
**Trigger:** Any `/apply --pool` run in this repo, or any repo whose plan does
not use kazi's `T<n>.<n>` numbering. Check for `BLOCKED: id must look like a
task` in the claim output — a run that dispatches nothing is the symptom.

## L-0022: A claim push takes 1-2 minutes, so claim in small batches

**Tags:** #tooling #apply #pool
**Date:** 2026-07-31
**Repo:** kazi-org/dira

**Rule:** Claim at most three tasks per shell invocation, and give the call a
generous timeout.
**Why:** Each `claim.sh claim` mints a commit and pushes a ref to `origin`, so it
costs a full network round trip. Six sequential claims exceeded a 120-second
command timeout and left the wave half-claimed — one task locked, five not, with
no error to distinguish "the push failed" from "the command was killed". A
half-claimed wave is worse than an unclaimed one: the locks that did land block
other sessions while nothing is executing against them.
**Trigger:** Claiming a wave of more than about three tasks in one loop. Verify
what actually landed with `git for-each-ref refs/remotes/origin-claims/` after
any timeout — do not infer it from the loop's output.

