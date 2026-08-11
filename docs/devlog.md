# devlog

Session narrative and operational findings, newest first. Invariants belong in
`docs/lore.md`; decisions belong in `.dira/entries/`. This file is what happened.

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
