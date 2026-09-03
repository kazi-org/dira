# devlog

Session narrative and operational findings, newest first. Invariants belong in
`docs/lore.md`; decisions belong in `.dira/entries/`. This file is what happened.

---

## 2026-09-03 — T-BUG2.0: issue #29 does not reproduce as written; the frontmatter splitter is not the fault

**What ran.** Diagnosis only, per `docs/plan.md`'s Discovery Summary finding 4:
does issue #29's colon-in-a-quoted-title repro fail at the `internal/frontmatter`
boundary-splitter, or downstream at `yaml.v3`/`Entry.Validate`? No production code
changed.

**What was read.** `internal/frontmatter/frontmatter.go`'s `Split` (lines 39-61)
locates the two `---` delimiter lines with a literal line-by-line scan and returns
everything between them as opaque bytes. It never inspects that content for
colons, quotes, or any YAML syntax — its only failure modes are `ErrMissing` (no
opening `---`) and an unclosed block. It is not a naive colon-based splitter; it
does not parse frontmatter at all, only its boundary.

**The finding.** Issue #29's repro, copied byte for byte from the issue body
(confirmed via `gh issue view 29 --json body` piped through `od -c`: the quotes
around both example titles are plain ASCII `"`, `0x22`, not curly) does not
reproduce. Built the binary and ran it against this repo's real ledger:
`dira why dec-0032` renders cleanly and `dira reindex` reports zero invalid
entries across all 48 — `dec-0032`'s own title,
`"Ledger ids: persist a monotonic counter instead of allocating from a directory
scan"`, already carries exactly the reported shape. A throwaway `go test`
against `ledger.Decode` (`internal/ledger/decode.go`) with the issue's literal
title text, plus bisected variants (colon immediately after a word, colon+space
mid-title, colon at the very end, nested double quotes), all decoded without
error.

The failure does reproduce, but only for two input shapes, neither of which is
"a colon inside a properly double-quoted title":

1. An unquoted title containing `: ` (colon-space) — a bare YAML syntax error.
2. A title quoted with Unicode "smart" typographic quotes (`“`/`”`,
   `“ ”`) instead of straight ASCII `"` — YAML does not recognize curly quotes as
   a quoting mechanism, so the value parses as an unquoted plain scalar and hits
   the same error.

Both fail identically, at the same call site: `yaml.Unmarshal(front, &doc)` in
`decodeEntry`, **`internal/ledger/decode.go:62`** (inside `func decodeEntry`,
declared at `decode.go:55`) — with the error `yaml: line 3: mapping values are
not allowed in this context`. This is inside `yaml.v3` itself, reached
immediately after `frontmatter.Split` returns successfully (`decode.go:56`) and
strictly before `Entry.Validate` ever runs (only reached from `Decode` at
`decode.go:44-45`, which neither failing case gets to). `internal/frontmatter`'s
boundary-splitter is not implicated in either failure mode: it hands `yaml.v3`
the identical bytes whether the colon is safely quoted or not, because it has no
opinion on the content between the two `---` lines.

**Implication for T-BUG2.1.** The Discovery Summary's finding-4 fix — threading
`ix.store.Get`'s real error through `internal/index/sync.go` (the discard is at
line 120, `invalid = append(invalid, info.ID)`, dropping the `err` inspected at
line 104) — covers issue #29 for free, once the underlying file matches a shape
that actually reproduces (unquoted colon, or smart-quoted title). **No
`internal/frontmatter` patch is needed**: the boundary-splitter behaved correctly
on every input tried. The `yaml.v3` message once surfaced is genuinely useful (it
names the line and the syntax problem), though it won't itself say "you used
curly quotes" — a UX nicety, not a correctness gap, and out of scope here.

**Open question, not resolved here.** The issue's own repro text, as literally
written by its author, does not reproduce with the shipped code. Most likely
explanation: a copy/paste transcription that lost the actual failing bytes
(smart quotes are the leading suspect — the same paragraph's em dash in the
"parses" example suggests text that passed through a typography-substituting
editor before it reached the entry file). Recommend, if this recurs: capture the
exact file bytes (`od -c` or `git show`) rather than a re-typed example, so the
next diagnosis starts from ground truth instead of a plausible-looking retype.

**Verdict.** `yaml.v3` rejects it, not the frontmatter splitter —
`internal/ledger/decode.go:62` in `func decodeEntry`. The issue as literally
written does not reproduce; the real trigger is an unquoted colon or
smart/curly quotes around the title.

---

## 2026-09-02 — backlog triage: a fix already shipped, and a bug misdiagnosed as its opposite

**What ran.** `/plan` to refine the backlog and triage all 8 open GitHub issues
(`#27` through `#37`). No code changed; `docs/plan.md`, `docs/roadmap.md` and
`docs/coverage.md` did, plus two staged ledger entries (`dec-0032`, `dec-0033`).

**The finding worth keeping.** Issue #27's own comment thread records that David
and three reviewers already decided the fix: make `dira log`'s creation path
exclusive-create, refuse if the id exists. Reading `internal/ledger/local/local.go`
and `internal/ledger/write.go` directly shows that fix already shipped —
`Store.Create` uses `os.Link` (atomic, fails on an existing target) and `Add`
retries correctly on `ErrExists`, marking the id taken and advancing. Both `dira
log` and the `sniff` auto-capture hook route through the same `Add`. Git blame
puts this at commit `4b7a0d9`/`b7c2f61`, 2026-07-30 — one day into the project,
before v0.1.0/v0.1.1, before every reported occurrence (2026-08-19 through
2026-09-02). A decision reached by reasoning about a bug can be correct about the
fix and wrong about whether it is still needed; nobody had re-read the source
since deciding.

**How the real mechanism was pinned down.** A peer session (macbook-chief) did the
git archaeology the local repo couldn't: hq's `dec-0542` has exactly one commit,
already holding the winning content, no merge, no discarded parent anywhere in
history or reflog. A merge/rebase landing one side of two independently-created
files at the same path would leave a two-parent commit or a reachable loser;
neither exists. That rules out a cross-branch merge collision (the first
alternative hypothesis raised) and best fits issue #35's bug instead — the entry
was created once, successfully and atomically, then deleted before it was
committed, freeing its number for a later session's legitimate reuse. This is not
fully provable (a pre-commit deletion leaves no trace by construction), which is
why the plan adds a stress test (`T-BUG1.1`) rather than closing the question by
assertion.

**Second, smaller finding.** Issues #28, #29, #30 and #31 all report the same
generic "N entry file(s) could not be read" notice with no field-level detail.
`internal/index/sync.go` confirmed as the single point of loss: `ix.store.Get`'s
error is checked (`errors.Is(err, ledger.ErrNotFound)`) and then discarded — only
the id is kept. `ledger.Decode`/`Entry.Validate` already produce the exact
field-and-limit detail issue #31's reporter extracted by hand with a throwaway
harness; the fix is threading it through, not writing new validation. One
exception noted rather than assumed: issue #29's colon-in-a-quoted-title case
passes through a separate `internal/frontmatter` boundary-splitter before
`yaml.v3` ever runs, and that layer's behavior on this input has not been read —
flagged as a diagnose-first task instead of folding it into the same fix
unverified.

**Process note.** Two design decisions from this pass went into the ledger as
`state: staged`, not self-confirmed by the planning session — `dira distill` is
where David disposes them, matching the tool's own designed workflow rather than
an agent asserting a decision on his behalf. `dira check` was run against the
plan's direction before writing anything: no conflict with 35 enforced entries.

---

## 2026-08-11 — six execution waves, and one defect found nine times

**What ran.** Six pool waves as coordinator, 30 tasks merged, 23 of 40 lanes
shipped. The capture-and-review loop works end to end: `dira sniff` stages from a
real transcript, `dira distill` renders the card through a pty and declines safely
on a pipe.

### The finding that outranks the code

**A check reporting a verdict it never reached — nine instances, four of them new
this session.** The shape is always the same: the check runs, returns cleanly, and
its answer is about something other than what it claims to measure.

- A **coverage gate blind to its own untracked source** — it graded a file git had
  never seen.
- A **contrast matrix blind to a composited tint**, because `color-mix(in oklab)`
  cannot be approximated in sRGB and the gate parsed `oklab()` floats as RGB. It
  reported legible text at 1.31:1.
- A **platform allowlist that only ever saw its own host** — green on macOS, red on
  ubuntu the first time CI ran it.
- A **privacy lint that could not see a private parent** declared in the exact shape
  shipped as its own documented example, because its regex anchored the closing
  brace to end-of-line and the example carries a trailing comment. It also read
  `visibility = "privat"` as public.
- A **pre-commit hook calling a machine-global lint lock a lint failure** — the lock
  was held by an unrelated repo in another session.
- A **linter exiting non-zero while printing "0 issues"** on a build-tagged package
  it could not typecheck, meaning nothing had ever linted `internal/perf`.
- **Three commands built, tested, merged and unreachable** (L-0027).
- **A font committed, licensed, documented and referenced nowhere** — `dec-0016` was
  `accepted` for weeks while nine design gates passed, because every one measures
  mockups that use the system stack.
- **`gates.mjs` scoring a control that crashed at `import` as CONTROL TRIPPED.** The
  harness whose entire purpose is catching gates that cannot fail was itself blind.

**The rule that came out of it, sharper than the one we started with.** We began
with "a check that cannot fail the premise is not evidence for the premise" — the
red→green rule. A lane produced the stronger form by breaking and restoring its own
check: **watching a gate go red is not sufficient; the case that is supposed to pass
must be watched passing too.** Two bugs lived in the green case where every red case
looked perfect — one where a wrapped sentence meant a literal `includes()` could
never match a correct document.

### Latency, settled

CI medians are ~39ms (ubuntu) and ~55ms (macOS) against `int-0002`'s 100ms ceiling.
The developer machine cannot certify any of it: `dira version`, which opens no
ledger, measured 95–355ms depending on load, and `/usr/bin/true` measured 70ms.

Four absolute budgets now carry two statistics. **The median asserts; the minimum
decides whether the machine can judge at all.** A best sample comfortably inside the
ceiling proves the code can meet it, so a median outside it is a statement about the
scheduler. `dec-0026` dropped the single-run maximum entirely after CI measured a
191ms outlier against a passing median — at n=20 the maximum *is* one observation.

**The cold path got genuinely faster**, found by phase attribution rather than
guessed: the build read every entry file **twice** — `List` hashed all 200, then
`Get` opened and parsed the same 200 — and on an empty cache the hash pass decides
nothing. That path went 11.2ms → 0.457ms. Separately, `decode.go` imported the
schema package for a string split and a sentinel, dragging `jsonschema/v6` and 16
text packages into every invocation: **−1.20 MiB and −21,710 allocations per run**,
31% of everything allocated before `main`.

### Operational findings

- **The pool could never dispatch.** `claim.sh` rejects this repo's `E1-L6-T1` id
  shape and returns BLOCKED, which reads as "the pool is empty". `T-` prefix fixes
  it (L-0021), and claims cost 1–2 minutes each so they must be batched (L-0022).
- **Wave size is disk-bound, not agent-bound.** 15–16 GiB free at 97% full; below
  ~20 GiB builds fail, retry, and write more.
- **A wave boundary does not prevent a name collision.** Two lanes declared `Editor`
  in one package without sharing a file — the boundary checks files, not namespaces.
- **An unbounded spin shipped and was found by mutation**, not by review: a failed
  disposition did not advance the card, which is right for a human retrying and
  infinite against an endless key source and a failing store.

### What I got wrong

**Four figures I relayed in agent briefs were unverified, and every one was caught
by the lane I gave it to.** The worst was a *correction* to `dec-0026` that mixed
ubuntu and macOS medians into one list, because I harvested with a grep taking the
first match per run — the platforms differ by 17ms in the same run. A correction
written to fix an unmeasured number introduced two more. I also told a lane that
`schema.NewValidator()` was on the write path; all 19 call sites are tests, which is
precisely why the cut was possible.

**I committed directly to `main` twice**, both times the same way: chaining
`commit && checkout main && merge`, where the commit fails on a lint lock but the
checkout still runs, so the staged work lands on main on the retry. Stop chaining.

**I forgot to tick task checkboxes three times.** Lane agents are correctly
forbidden from editing the files they are graded against, which makes ticking the
integrator's job, and a stale plan invites re-claiming finished work.

**A plan rewrite dropped an entire epic.** `scripts/coverage.py` caught it by
orphaning ten `lanes:` rows — it extracts one obligation per epic from `### E<n>`
headings, and the rewrite had removed them. E7 is five lanes of paid apps, parked on
a real trigger, and it would have vanished silently.

### What the lanes refused, and were right to

Several reported an acceptance clause **unreachable** rather than editing it: "zero
staged entries after y/n/e" cannot hold because `y` promotes rather than accepts;
"deletes the file iff install created it" is unimplementable because nothing records
who created a settings file; "prints nothing to stdout" is satisfiable only by a
fake that prints nothing, since stdout is the payload. Each honest report was
correct, and an `acc:` edited by its own implementer would have hidden all three.

One lane refused to confirm a timing it could not measure — the machine was at load
390–420 and its median run was twenty times its minimum — and reported the
load-independent allocation count instead. That is a better answer than a confident
millisecond figure.
