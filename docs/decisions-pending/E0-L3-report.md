# E0-L3 — CI gate on every push

**Files landed (nothing committed, nothing pushed):**

- `.github/workflows/ci.yml` — the gate
- `scripts/ci-local.sh` — the same gate set, runnable on a laptop in one command

Nothing outside those two paths was touched.

---

## Status in one line

Every gate is wired and every gate has been observed producing a real verdict on this
machine, including going red on real findings. **The workflow itself has never run on
a GitHub runner.** It is validated (`actionlint` clean, shellcheck clean, YAML parses,
run-blocks proven two-sided in a sandbox) but it is unproven until a push executes it.

---

## What was wired

Four jobs, all required, none with `continue-on-error` and no `|| true` anywhere.

| job | runner | gates |
|---|---|---|
| `go` | `ubuntu-latest` **+** `macos-latest` | `go build ./...`, `go vet ./...`, `go test -race -count=1 ./...` + non-zero-test-count assertion |
| `lint` | `ubuntu-latest` | `gofmt -l .`, `golangci-lint run ./...` |
| `gates` | `ubuntu-latest` | `python3 scripts/coverage.py`, `python3 scripts/privacy-lint.py`, `node docs/design/scripts/contrast.mjs` |
| `rendered-contrast` | `ubuntu-latest` | `node docs/design/scripts/contrast-rendered.mjs` (Chromium) |

Triggers: `push` (all branches), `pull_request`, `workflow_dispatch`.
`permissions: contents: read` at the top level; no job widens it.
Concurrency cancels superseded runs on topic branches but **never on `main`** — a
cancelled run on main is a lost answer about whether main is broken.

**Why four jobs rather than one.** The lane's `acc:` requires that a push carrying an
unformatted file *and* a failing test produce a red run **naming both**. In a single
sequential job the first red masks the second. Split across parallel jobs, both are
named in the same run. `fail-fast: false` on the `go` matrix exists for the same
reason: a darwin-only failure must not hide the linux result.

**Toolchain.** `actions/setup-go` with `go-version-file: go.mod` — the version is never
restated in YAML. setup-go handles module and build caching itself, keyed on `go.sum`;
both Go caches are content-addressed, so a stale entry can only make a cold run slower,
never produce a wrong verdict.

**`macos-latest` is in the matrix.** The L2 prompt asked for a decision with the reason
recorded in a comment; it is in the file. Short version: `dira` is a public repo so
macOS runner minutes are free, darwin-arm64 is the primary user platform (`int-0002`),
and `internal/index` does filesystem, timestamp and cache work where darwin and linux
genuinely diverge. The cost is queue time, paid in parallel with the linux leg rather
than added to it. Lint runs linux-only — `gofmt` and `golangci-lint` are
platform-independent and running them twice buys nothing.

**No `actions/setup-python`.** Both Python gates are stdlib-only by explicit design
(`coverage.py`: *"deliberately dependency-free so this runs anywhere, including CI
before any Go toolchain exists"*) and `ubuntu-latest` ships `python3`. One fewer pinned
third-party action in the supply chain.

---

## The browser-dependent gate: it goes in CI

**Decision: `contrast-rendered.mjs` runs in CI, in its own required job.** Reasoning,
since the brief asked for it honestly either way:

**For.**
1. It is the only gate that measures what actually ships. Every chip sits on a
   `color-mix(in oklab, <own hue> N%, transparent)` tint of its own colour, so the pair
   that reaches a reader is fg-on-tint. `contrast.mjs` structurally cannot see that
   pair and reported clean while five chips rendered between 3.0:1 and 4.3:1.
2. **All of its inputs are tracked.** I checked: `docs/design/screens/*.html` (6 files),
   `docs/design/landing/index.html`, `docs/design/tokens.css` are all in `git ls-files`.
   So it is reproducible from a cold clone and is not a CI-only check — the `cst-0004` /
   `int-0002` objection in the L2 prompt does not bite here.
3. There is no cheaper substitute. `color-mix(in oklab, ...)` has no correct sRGB
   approximation; the script's own header records that an sRGB approximation *passed*
   pairs the browser renders as failures. Dropping the browser means dropping the check.
4. What it catches is a shipped-to-a-reader defect, not a style opinion.

**Against, stated fairly.** It is the slowest and most fragile job here — a browser
download, apt system libraries, and a headless render. A Playwright or Chromium
infrastructure outage turns the gate red for a reason no author caused.

**How the downside is contained.** Its own job, so a browser-infra failure is legible
as itself and does not sit in front of the Go signal. Chromium cached at
`~/.cache/ms-playwright` keyed on the exact Playwright version, so a cache entry can
only ever serve the browser build that version would have downloaded — a cold miss
changes the duration, never the verdict. If the containment turns out to be
insufficient in practice, the honest remedy is to move it to a scheduled or
design-path-filtered trigger — **not** `continue-on-error`, which would leave a gate in
the file that gates nothing.

---

## Pinned SHAs

Every `uses:` is a 40-character commit SHA. Each was resolved and independently
re-verified via `gh api repos/<owner>/<repo>/commits/<tag> --jq .sha` — note that
`git/ref/tags/<tag>` returns an **annotated tag object** SHA for some of these
(`golangci-lint-action` v9 → `db9de0fc…`, type `tag`), which is *not* a valid `uses:`
target. The commits endpoint is the one that gives the right answer.

| action | version | commit SHA |
|---|---|---|
| `actions/checkout` | v5.1.0 | `fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09` |
| `actions/setup-go` | v6.5.0 | `924ae3a1cded613372ab5595356fb5720e22ba16` |
| `actions/setup-node` | v5.0.0 | `a0853c24544627f65ddf259abe73b1d18a591444` |
| `actions/cache` | v4.3.0 | `0057852bfaa89a56745cba8c7296529d2fc39830` |
| `golangci/golangci-lint-action` | v9.3.0 | `ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a` |

**Evidence that tag-pinning would have been wrong:** `kazi`'s workflows pin
`actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5`, which is **v4.3.1**. The
`v4` tag today resolves to `11d5960a…`. The tag moved under the pin; the SHA did not.
That is the whole argument, demonstrated in a sibling repo.

`golangci/golangci-lint-action` is the one non-`actions/` dependency. The alternative
was downloading a pinned binary with a hand-maintained SHA256, the way kazi installs
`nats-server`. Pinned to an immutable commit, the action is equivalent in supply-chain
exposure and I do not have to maintain a checksum by hand.

---

## What I actually verified, and what I did not

**Verified by running it:**

- `actionlint` exits 0 on `.github/workflows/ci.yml`. I confirmed its shellcheck
  integration is live rather than assuming it — a deliberately broken copy in the
  scratchpad produced `SC2086` findings and exit 1, the real file produces neither.
- `shellcheck scripts/ci-local.sh` and `bash -n` both clean.
- `scripts/ci-local.sh` ran end to end: 8 of 9 gates green, 1 red (see below).
- **The workflow's two novel run-blocks were proven two-sided**, using the script text
  extracted programmatically out of the shipped YAML (so I tested what ships, not a
  retyped copy), against throwaway modules in the scratchpad:

  | case | expected | observed |
  |---|---|---|
  | module with zero tests | red | exit 1, `go test executed 0 tests…` |
  | module with one test | green | exit 0, `top-level tests executed: 1` |
  | module with a failing test | red | exit 1 (`pipefail` carries the failure through `tee`) |
  | clean tree, gofmt | green | exit 0, `gofmt: clean` |
  | unformatted file, gofmt | red | exit 1, **names `ugly.go`** |

  That is the red half of the lane `acc:` demonstrated at the script level: an
  unformatted file is named, and a failing test is named, and both go red.

**Not verified — state this plainly:**

- **No GitHub Actions run has ever executed this workflow.** `act` is not installed and
  would need Docker regardless. Runner-level behaviour (setup-go's cache on a cold
  runner, `playwright install --with-deps` under the runner's apt, macOS arm64 legs,
  `$GITHUB_STEP_SUMMARY` rendering) is inferred from the action documentation, not
  observed. The lane's green-half and red-half `acc:` clauses both require an observed
  run and remain **unmet** until someone pushes.
- The `go` matrix has only ever been exercised on darwin here. The linux leg is unrun.

---

## The one red gate, and why it is not mine

`scripts/ci-local.sh` reported `FAIL golangci-lint run ./...` at 21:11 — 17 `unused`
findings across `internal/enforcer/{match,target,text}.go`. Those three files were
**untracked**, written minutes earlier by another lane working in this tree. Minutes
later the tree briefly failed to compile at all (`cmd/dira/why_test.go:497: snapshot
redeclared`, colliding with `log_test.go:102`), and by 21:13 both had cleared and
`golangci-lint run ./...` was back to `0 issues`.

Two things follow. First, my configuration is not the cause — this is another lane's
work in flight. Second, and more usefully: **the lint gate went green, then red on real
findings, then green again, with no change from me.** That is unforced two-sided proof
that it is a live gate and not a vacuous one.

Consequence for the lead: whoever pushes first should expect the first CI run to
reflect whatever state the tree is in at that moment, which is currently changing every
few minutes.

---

## Where the brief was wrong or incomplete

1. **`coverage.py` does not assert its own source file is tracked.** It asserts its
   *input* sources are — `.dira/entries`, `docs/design/DESIGN.md`, `docs/roadmap.md`,
   `docs/plan.md` (`SOURCES` at `scripts/coverage.py:175`). `scripts/coverage.py`
   itself is not in that list. The class of failure it catches is still exactly the one
   described, but on inputs, not on itself.

2. **The rendered-contrast gate is not currently reproducible from a cold clone by
   anyone, including the maintainer.** There is **no `package.json` and no lockfile** in
   this repo. `contrast-rendered.mjs` does `import { chromium } from 'playwright'`, and
   the only reason it runs on this machine is that `node_modules` at the repo root is a
   **symlink into a session scratchpad** (`/private/tmp/claude-501/…/scratchpad/
   node_modules`, Playwright 1.62.0) and `node_modules` is gitignored. A fresh clone
   gets nothing. CI is what makes this gate reproducible for the first time — but with
   no lockfile to read, the version has to live in the workflow as
   `PLAYWRIGHT_VERSION: "1.62.0"`, which is precisely the "restated version drifts"
   problem the L2 prompt warns about for the Go toolchain. I pinned it to what the
   maintainer runs today and flagged it in a comment. **Top follow-up below.**

3. **No `.golangci.yml` exists.** The linter runs on defaults, and the version it runs
   at is recorded nowhere in the repo — `hooks/pre-commit` just calls whatever
   `golangci-lint` is on `PATH`. I pinned CI to `v2.11.3`, matching this machine, rather
   than floating to latest (currently v2.12.2): a new linter release turning an
   unrelated push red is a gate failing for a reason the author did not cause.

4. **`hooks/pre-commit` is a good deal weaker than the brief implies.** The brief says
   CI must run the same gates as the local hook. The hook actually runs: `coverage.py`,
   `privacy-lint.py`, `golangci-lint --fix` over *staged packages only*, and
   `go test ./...` **without `-race`**. It does **not** run `go build`, `go vet`,
   `gofmt -l`, or either contrast script. CI is therefore a strict superset, which is
   the right direction — but `-race` matters specifically here, because
   `internal/index/race_{on,off}_test.go` and `internal/ledger/fixture/race_{on,off}_
   test.go` are build-tagged on the detector and mean nothing without it. Bringing the
   hook up to CI's set is `hooks/`-owned work, not mine.

5. **`on: push:` is unrestricted, per your brief, and this diverges from the L2 prompt**
   (which cites kazi's `push: branches: [main]` plus `pull_request`). Your instruction
   and the lane `acc:` — *"a push to **a branch** of `kazi-org/dira` produces a green
   Actions run"* — both want any-branch pushes to run, so unrestricted is right. The
   cost is that a same-repo PR gets two runs (one for `refs/heads/…`, one for
   `refs/pull/N/merge`); concurrency cannot dedupe them because the refs differ. Cheap
   on a public repo, and worth knowing rather than discovering.

---

## Follow-ups, none of them inside my ownership boundary

1. **Add `package.json` + `package-lock.json`** pinning `playwright@1.62.0`, then change
   the workflow's install step to `npm ci` and delete `PLAYWRIGHT_VERSION`. This is the
   single change that removes the last restated version from the YAML and makes the
   rendered-contrast gate reproducible from a cold clone by a human. Repo-root file, so
   not mine.
2. **Add `.golangci.yml`** carrying the golangci-lint version, so CI, the hook, and a
   contributor's laptop all agree on which linter is authoritative.
3. **Bring `hooks/pre-commit` up to CI's set** (`-race`, `go vet`, `gofmt -l`) or record
   deliberately why the fast path stays narrower.
4. **Consider a drift check** asserting the workflow and `scripts/ci-local.sh` invoke
   the same gate list. Right now that agreement is maintained by the comment at the top
   of each file, which is discipline, not architecture — the exact thing
   `hooks/pre-commit`'s own header says a guarantee must not depend on.
