# E0-L1 report — Go module and `dira` command skeleton

**Status:** complete, tree left dirty and uncommitted as instructed.
**Toolchain:** `go version go1.26.2 darwin/amd64`. **Module:** `github.com/kazi-org/dira`.

---

## What was built

| path | what it is |
|---|---|
| `go.mod` / `go.sum` | module `github.com/kazi-org/dira`, `go 1.26.2`. Two direct requires, both test-only: `github.com/santhosh-tekuri/jsonschema/v6`, `gopkg.in/yaml.v3` (+ `golang.org/x/text` indirect). |
| `cmd/dira/main.go` | entrypoint, `version` var, exit-code contract, command registry, dispatch. Stdlib `flag` only. |
| `cmd/dira/usage.go` | help rendering, driven off the registry. |
| `cmd/dira/main_test.go` | CLI surface: help/version/exit codes, in-process. |
| `cmd/dira/build_test.go` | the three predicates that need a real build: no-third-party-deps, `-ldflags` version stamping, exit 2 across a process boundary. |
| `schema/entry_test.go` | the ledger gate: frontmatter parse, yaml→JSON normalisation, JSON Schema validation. |
| `schema/testdata/valid/*.md` (3) | positive fixtures incl. the unquoted-timestamp case. |
| `schema/testdata/invalid/*.md` (17) | negative fixtures. |
| `README.md` | new "Build from source" section; status callout corrected. |
| `docs/plan/tasks/E0-L1.md` | the lane's leaf tasks, recorded as-built (added in round 2). |

### Design decisions worth the lead's attention

**No CLI framework.** Hand-written dispatch over `flag.FlagSet`. `go list -deps ./cmd/dira`
reports exactly one package — the command itself. This is enforced by
`TestCommandPathHasNoThirdPartyDependencies`, which also fails if the listing comes
back empty, so it cannot pass vacuously.

**Two commands registered: `help` and `version`.** Both fully implemented. I did *not*
register `log`/`why`/`brief` as "not implemented" stubs even though the lane brief
permits it — advertising commands in `--help` that refuse to run is worse UX than not
advertising them, and it would make the help-lists-every-command predicate green
against non-functional entries. Adding one in E1 is a single line in `newApp`.

**Exit-code contract.** `0` success, `1` runtime error, `2` usage error. Encoded as a
`usageError` wrapper type checked with `errors.As`; a usage error prints the offence
plus full usage to **stderr** and writes **nothing to stdout**, so a caller parsing
stdout never receives help text. Bare `dira` with no arguments prints help to stdout
and exits 0 — the acc: line does not specify this case; flag it if you want exit 2.

**Exit code 1 is tested with a test-only command** appended to the registry inside
`_test.go`, because no real command can fail at E0. That keeps a fake out of the
production path while still pinning the contract E2's hooks depend on.

**`schema/` is a test-only Go package.** `go:embed` cannot reach above its own
directory and `schema/entry.schema.json`'s `$id` is a public URL that must not move,
so the package lives beside the file. It has no non-test source today. E0-L2 adds
`schema.go` with the embed and promotes the parsing/validation helpers to exported
functions in the same package. `go build ./...`, `go vet ./...` and `gofmt` are all
clean on a test-only package — verified, not assumed.

---

## Verbatim output

```
$ go version
go version go1.26.2 darwin/amd64

$ go build ./...
exit=0

$ go vet ./...
exit=0

$ gofmt -l .
(no output)
exit=0

$ go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/dira
github.com/kazi-org/dira/cmd/dira
exit=0

$ go test ./...
ok  	github.com/kazi-org/dira/cmd/dira	(cached)
ok  	github.com/kazi-org/dira/schema	(cached)
exit=0
```

`go test ./... -v -count=1` — 15 top-level tests, 87 subtests, zero failures:

```
--- PASS: TestCommandNamesAreUnique (0.00s)
--- PASS: TestCommandPathHasNoThirdPartyDependencies (0.96s)
--- PASS: TestDefaultVersionIsDev (0.00s)
--- PASS: TestHelpForOneCommand (0.00s)
--- PASS: TestHelpNamesEveryRegisteredCommand (0.00s)
--- PASS: TestInvalidFixturesAreRejected (0.02s)
--- PASS: TestJSONValueFlattensTimestamps (0.00s)
--- PASS: TestLedgerValidates (0.03s)
--- PASS: TestRuntimeErrorExitsOneAndDoesNotPrintUsage (0.00s)
--- PASS: TestTimestampsAreQuotedOnDisk (0.00s)
--- PASS: TestUnknownCommandExitsTwoFromTheRealBinary (1.56s)
--- PASS: TestUnknownCommandIsAUsageError (0.00s)
--- PASS: TestValidFixturesValidate (0.01s)
--- PASS: TestVersionIsSetByLdflags (2.89s)
--- PASS: TestVersionPrintsBareVersionAndNothingElse (0.00s)
```

Binary behaviour, from a real `go build` (not `go run`, which masks exit codes):

```
$ go build -o dira ./cmd/dira && ./dira --version
dev
exit=0

$ go build -ldflags "-X main.version=1.2.3" -o dira ./cmd/dira && ./dira --version
1.2.3
exit=0

$ ./dira --help ; echo exit=$?
dira - a git-native ledger of intents, decisions, rejected
alternatives, open questions, and constraints.

usage:

	dira <command> [arguments]

commands:

	help     show usage for dira or one of its commands
	version  print the dira version

flags:

	-h, -help   show this help
	-version    print the version

exit codes:

	0  success
	1  runtime error
	2  usage error
exit=0

$ ./dira nosuchcommand 2>/dev/null ; echo exit=$?
exit=2                              # nothing on stdout

$ ./dira nosuchcommand 2>&1 1>/dev/null | head -2
dira: unknown command "nosuchcommand"
                                    # usage block follows, on stderr
```

Existing gates unaffected:

```
$ python3 scripts/privacy-lint.py
PRIVACY LINT PASS — cst-0003 enforced by 4 checks.   exit=0

$ python3 scripts/coverage.py
COVERAGE PASS — every obligation has a disposition.  exit=0
```

---

## Red-before-green proof

Every predicate was checked red in a throwaway copy of the tree
(`scratchpad/redcheck`, since deleted). The real `.dira/entries/` was never touched.

| mutation | result |
|---|---|
| `dec-0007.md` `kind: decision` → `kind: epic` | `TestLedgerValidates/dec-0007.md` FAIL — `at '/kind': value must be one of 'intent', 'decision', ...` |
| unquote `created:` in `int-0001.md` | `TestTimestampsAreQuotedOnDisk/int-0001.md` FAIL |
| give `decision-without-alternatives.md` an `alternatives` block | `TestInvalidFixturesAreRejected/decision-without-alternatives.md` FAIL — "validated successfully; it must violate entry.schema.json" |
| `import _ "gopkg.in/yaml.v3"` in `cmd/dira/usage.go` | `TestCommandPathHasNoThirdPartyDependencies` FAIL — "found 1 non-stdlib dependencies: gopkg.in/yaml.v3" |
| strip time.Time handling **and** the JSON round-trip from `jsonValue` | `TestValidFixturesValidate/unquoted-timestamp.md` FAIL — `at '/created': invalid jsonType time.Time` |

The last one is the landmine you named. It reproduces exactly. Note the nuance:
stripping *only* the explicit `case time.Time` is not enough to turn it red, because
`json.Marshal` also flattens a `time.Time` via its `MarshalJSON`. Two independent
mechanisms cover it. I kept the explicit case anyway — E0-L2's exported reader will
want to skip the JSON round-trip for speed, and the rule needs to survive that — and
added `TestJSONValueFlattensTimestamps`, which *is* red without it. The code comment
states this honestly rather than claiming the explicit case is load-bearing today.

**Write side:** `TestTimestampsAreQuotedOnDisk` asserts every `created:`/`updated:` in
every real entry is quoted. There is no writer yet, so "always quote on write" is
currently enforced only against the on-disk state; E1's writer must satisfy it.

---

## Negative fixtures — 17, all rejected for their stated reason

`TestInvalidFixturesAreRejected` matches each rejection against a required substring,
so a fixture that fails for an unrelated reason (a YAML typo, say) does not count as a
pass. It also separates **parse-stage** from **validate-stage** failures, and fails if
any fixture on disk lacks a case entry or vice versa.

id pattern · unknown top-level field · decision with no alternatives · intent in a
decision-only state · missing `created` · sixth `kind` · unregistered edge type · edge
with no `to` · title under `minLength` · uppercase tag · duplicate tags · alternative
with no `why_not` · non-RFC3339 `created` (requires `AssertFormat()`, which is off by
default — that fixture is also the test that it was switched on) · `source.tier: api`
(dec-0003 forbids it) · `edges` as a scalar · no frontmatter · unterminated
frontmatter.

---

## acc: line status

Lane acc: — **all green.**

| clause | state |
|---|---|
| `go build ./...` and `go vet ./...` succeed on the version in `go.mod` | GREEN |
| `gofmt -l .` prints nothing | GREEN |
| `go list -deps ./cmd/dira` contains no non-stdlib module | GREEN, and enforced by a test |
| `dira --help` exits 0 and names every registered subcommand | GREEN, driven off the registry |
| unknown subcommand exits 2, usage on stderr, nothing on stdout | GREEN, verified through a process boundary |
| `dira --version` prints `dev` plain, exactly `1.2.3` under `-ldflags` | GREEN, verified by building twice |

E0 L0 acc: — the two clauses this lane touches:

| clause | state |
|---|---|
| `go build ./...` succeeds | GREEN |
| `go test ./...` passes with a schema-validation test that fails when a fixture entry violates `entry.schema.json` | GREEN — 25 real entries validate, 17 negative fixtures rejected |
| tagged push → downloadable darwin-arm64 + linux-amd64 binary | NOT THIS LANE (E0-L4) |
| `brew install kazi-org/tap/dira` | NOT THIS LANE (E0-L5) |
| CI runs `python3 scripts/coverage.py` and fails on non-zero exit | NOT THIS LANE (E0-L3) — script verified still passing |

---

## Left unimplemented, deliberately

- **No ledger commands.** No `log`, `why`, `brief`, `reindex`, `init`. E1 owns them.
- **No `internal/ledger`, `internal/ui`, `internal/check`.** The lane brief forbids
  scaffolding future epics and I agree: an empty package is a lie about progress that
  `go vet` cannot flag.
- **No `go:embed` of the schema.** E0-L2's job. The test reads
  `schema/entry.schema.json` from disk relative to its own package directory.
- **No CI, no goreleaser, no Homebrew formula.** E0-L3/L4/L5.
- **Semantic ledger checks are out of scope and stay red for E0-L2**: id-prefix vs
  `kind` agreement, filename vs `id` agreement, and `derives_from` edges pointing at
  entries that do not exist. None of these are visible to `entry.schema.json` as
  written — `$defs/ref` is defined and never referenced, so `edge.to` has no pattern
  at all. I deliberately did **not** add fixtures for them, because a fixture asserted
  to fail that the schema cannot reject would have to be either deleted or faked. The
  E0.md lane note already calls this out as E0-L2's central risk; it is still live.

---

## Corrections to the brief

1. **The brief at `docs/plan/prompts/L2-E0-L1.md` is an L2 *planner* prompt** — "You
   are an L2 planner. You produce **tasks**, not code," writing to
   `docs/plan/tasks/E0-L1.md`. **Resolved in round 2:** the lead confirmed the
   implementation instruction stands and the prompt's framing does not, and asked for
   the tasks file as well. `docs/plan/tasks/E0-L1.md` now exists, recording seven leaf
   tasks as-built (bound is eight) under the field contract the prompt specifies —
   id, title, files, `acc:`, `depends_on` — plus a green column. Every `acc:` there
   was run before it was written down.

2. **There are 25 entries in `.dira/entries/`, not 24** — cst 4, dec 13, int 3, qst 5.
   `scripts/privacy-lint.py` independently reports 25. `TestLedgerValidates` fails if
   the count drops below 20, so silent shrinkage is caught.

3. **`go list -deps` needs an explicit non-stdlib filter.** In module mode every
   package including stdlib carries module information, so "has a module" is not a
   usable stdlib test. The test uses `-f '{{if not .Standard}}...'`.

4. **The `time.Time` nuance above** — stripping the explicit case alone is not enough
   to reproduce the break. Detail in the red-before-green section.

---

## Round 2 — the `writeUsage` diagnostic is a false positive

The lead reported: "`a.writeUsage` is referenced at four call sites in `main.go` and is
undefined." It is defined, and the package compiles. `writeUsage` is a method on `*app`
declared in `cmd/dira/usage.go`, which is in `package main` alongside `main.go` — one
compilation unit, so the four call sites resolve normally.

```
$ grep -rn "func (a \*app) writeUsage" cmd/dira/
cmd/dira/usage.go:11:func (a *app) writeUsage(w io.Writer) {

$ grep -rn "a.writeUsage(" cmd/dira/ | grep -v "func "
cmd/dira/main.go:101:		a.writeUsage(a.stderr)
cmd/dira/main.go:120:			a.writeUsage(a.stdout)
cmd/dira/main.go:133:		a.writeUsage(a.stdout)
cmd/dira/main.go:156:		a.writeUsage(a.stdout)

$ go list -f '{{.Name}}: {{join .GoFiles " "}}' ./cmd/dira
main: main.go usage.go

$ go build ./...   exit=0
$ go vet ./...     exit=0
```

The diagnostic came from single-file analysis, which cannot see sibling files in the
same package. **No change was made** — implementing or deleting anything here would
have broken working code to satisfy a tool error. If the lead's editor keeps flagging
it, that is an LSP/workspace-root issue, not a code issue.

I did not merge `usage.go` into `main.go` to silence it. `main.go` is dispatch and
contract; `usage.go` is presentation. Splitting them is normal Go, and E1 will grow
help rendering further as commands are added.

## Round 2 — `docs/plan/tasks/E0-L1.md`

Seven leaf tasks (bound is eight), each with `id` / `title` / `files` / `acc:` /
`depends_on` and a green column. The one departure from the prompt's advisory
decomposition: its items 4 and 5 stay separate tasks, because one is an in-process
test of the CLI surface and the other shells out to `go list`, and they fail for
unrelated reasons.

The file states plainly that **T6 (the ledger gate) is E0-L2 work pulled forward**,
that only the schema-visible half landed, and that E0-L2's `acc:` is unchanged and
still red on the three rules the schema cannot express. The plan tree should not read
as though E0-L2 got smaller.

T7's `acc:` — "every command in the README build-from-source section runs from a clean
clone" — was verified by copying the tree without `.git` to a scratch directory and
running each command there:

```
$ go build ./cmd/dira        exit=0
$ ./dira --help              exit=0
$ ./dira --version           dev
$ go build -ldflags "-X main.version=1.2.3" ./cmd/dira && ./dira --version
1.2.3
$ go test ./...
ok  	github.com/kazi-org/dira/cmd/dira	1.740s
ok  	github.com/kazi-org/dira/schema	0.156s
```

The literal `git clone https://github.com/kazi-org/dira` line is the one step not
verifiable yet — nothing has been pushed. It becomes true when this lands.

## Notes for the lead before committing

**Exactly these paths are mine.** Everything else in `git status` belongs to other
lanes running in this session:

```
 M README.md
?? cmd/
?? go.mod
?? go.sum
?? schema/entry_test.go
?? schema/testdata/
?? docs/plan/tasks/E0-L1.md
?? docs/decisions-pending/E0-L1-report.md
```

Not mine, do not attribute to E0-L1: `docs/plan/prompts/L2-E8-L2.md` (modified),
`.claude-plugin/`, `docs/design/landing/`, `docs/design/scripts/check-coherence.mjs`,
`docs/growth/`, `docs/decisions-pending/wave-shape.md`.
- `.gitignore` already covers `/dira`, `/dist/`, `*.test`, `*.out`. Nothing added.
- `schema/testdata/` must be committed — the negative half of the gate lives there.
- `go.sum` must be committed.
- Two test-only dependencies enter the repo here. They are invisible to
  `go list -deps ./cmd/dira`, so the cobra-exclusion clause holds, but they are the
  first third-party code in the tree and worth a conscious nod at merge.
