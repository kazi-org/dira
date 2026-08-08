# The socket claim, and the limits of how it is checked

`cst-0004` says dira never requires a network service, an account or a hosted
tier. `int-0002` budgets a hook invocation at well under 100ms, and that budget
is only about dira if the run it measured was not quietly waiting on something
else. `TestTheBriefOpensNoSocket` in this package is those two sentences turned
into a check.

This document is the other half of that task. A check that is not honest about
its own reach is a check people over-read, and the reach here is narrower than
the test name suggests.

## What this proves

Two claims, deliberately kept apart, because they are different claims and a
single mechanism cannot make both.

**Behaviour — the run made no socket-creating syscall.** The cold
`dira brief --context --chain` run is traced, and the test fails if any
`socket`, `connect` or `socketcall` is attributed to the process. This is direct
observation of the process under measurement. It is the only part of this
document that is evidence rather than inference.

Three things make the verdict worth something:

- **A positive control runs first, on every run, under the same tracer
  invocation.** A short program that listens on loopback and dials itself is
  built with the same toolchain and traced the same way. If its socket syscalls
  are not observed, the tracer is not attached, and the test SKIPS rather than
  reporting dira as clean. Without this, a tracer that failed to attach and a
  binary that opened nothing produce byte-identical trace files, and the test
  would certify every program on earth.
- **The control is itself checked untraced first.** It has to print that it
  opened a socket when run on its own. A broken control and a broken tracer look
  the same from the trace file and route to opposite places, so they are
  separated.
- **An empty trace is a failure, not a pass.** An empty file means tracing did
  not happen. The darwin script prints an attach marker unconditionally so that
  a clean run still produces output and the rule holds on both tracers.

**Dependence — the run does not need a network.** On Linux the same cold run is
repeated inside an empty network namespace (`unshare -rn`), and the test fails if
stdout, stderr or the exit code differ from the same run with the network up by a
single byte. Both runs are additionally required to have rendered a real brief:
two identical empty outputs compare equal and prove nothing.

This half is weaker than the first and is kept only because it catches a
different failure. A program that opens a socket, fails, and carries on anyway
passes the namespace comparison and fails the trace.

## What this does not prove

**It is not a linkage check, and a linkage check is not available here.** The
tempting one-line version of this test — assert `go list -deps ./cmd/dira` names
no networking package — would be RED on a correct dira binary today. `cmd/dira`
legitimately links `net`, `net/http` and `crypto/tls`, reached through
`internal/ui`, which serves the read-only localhost UI:

```
$ go list -deps ./cmd/dira | grep -E '^(net|net/http|crypto/tls)$'
net
crypto/tls
net/http
```

A gate that cannot pass the case it exists to certify is `docs/lore.md` L-0001
rule 2 in its purest form, and it is the rule that gets skipped. Linking `net` is
not evidence of guilt and not linking it would not be evidence of innocence
either — a program can reach the network through a subprocess, and dira's
`--deep` handoff exists to spawn one. **Linkage is not evidence of abstinence.
The runtime trace is.** `internal/nomodel` states the same boundary from the
other side, and pins it with a test so a later tightening has to argue with a red
run rather than with a comment. `TestTheBriefOpensNoSocket`'s first subtest
asserts those three packages are still linked, so if the localhost UI ever moves
out of the command, this document goes red and has to be rewritten rather than
quietly becoming false.

The rest of what a green run does not cover:

- **One command, one ledger, one machine.** It traces
  `dira brief --context --chain` over the 200-entry fixture. It says nothing
  about `dira ui` — which is a localhost HTTP server and *does* open a listening
  socket, correctly — nor about `dira sniff`, `dira check`, or a `--deep` handoff
  that spawns an agent. Those are other claims and would need their own traces.
- **It does not follow children.** `strace -f` traces forks, so a child dira
  spawned itself would be caught, but the trace ends when the process does. A
  socket opened by something dira merely asked another process to do later is
  outside it.
- **It is not a proof about the source.** It is a proof about one execution.
  Code on a path this fixture does not reach — a first-run path, an error path, a
  config option nobody set — is untraced and unjudged.
- **It says nothing about DNS or filesystem reads.** dira reading `/etc/hosts`
  would not be a socket. The claim is exactly "no socket", not "no I/O".
- **A socket is a socket.** AF_UNIX counts. Filtering to internet families would
  mean parsing strace's argument rendering to find the address family, and a
  parser that gets that wrong once certifies a real connection. The strict claim
  is easier to check and the one worth making.

## Where it does not run, and what that costs

The tracer is platform-specific and the test **skips with the reason named in the
skip message** where there is none. It never asserts a weaker thing under the
same test name: a skipped platform is honest, a renamed assertion is not.

| platform | tracer | dependence check | status |
|---|---|---|---|
| linux, `strace` on PATH | `strace -f -e trace=socket,connect,socketcall` | `unshare -rn`, skipped when unprivileged user namespaces are refused | the intended home of this check |
| darwin | `dtrace`, guarded by the positive control | none — network-namespace isolation is a Linux facility | **skips in practice**: DTrace needs SIP disabled and root, and the probe refuses with `DTrace requires additional privileges` |
| anything else | none wired up | none | skips |

**On the maintainer's machine (darwin, SIP enabled) the socket assertion skips,
and has therefore never been observed either red or green there.** That is not a
formality, and it is stated rather than left to be inferred from a green `ok`:
the socket claim itself is at present *unverified by execution anywhere*.

What the darwin run does verify is the instrument, not the subject. Four
subtests run on every platform and each carries both sides:

1. **The trace reader**, against recorded transcripts in every shape the two
   tracers emit — `strace` unthreaded, with `-f`'s `[pid  N]` prefix, with a bare
   pid prefix, a `<... connect resumed>` line, i386's multiplexed `socketcall`,
   and dtrace's `printf` shape — and against the near misses that would make a
   sloppier reader red on a correct run: `openat(..., "/var/run/nscd/socket")`,
   `unshare(CLONE_NEWNET)`, and strace's own prose about socket syscalls.
2. **The trace plumbing**, through a stand-in tracer that writes a canned
   transcript and then execs the command. It proves the transcript comes back
   intact, that the traced command genuinely ran (it is traced over
   `dira reindex` with the cache removed, so "it ran" is a fact on disk), that a
   tracer writing no file is an error rather than a clean verdict, and that a
   tracer exiting non-zero is an error even when it did write one.
3. **The positive control's own socket**, run untraced.
4. **The linkage assertion and this document's contents.**

So the single thing left unverified on darwin is whether a real tracer attaches
and reports faithfully. That is exactly the gap a Linux runner closes.

The darwin `dtrace` path is written but unexercised, and is fail-safe rather than
fail-open by construction: if the script is wrong or dtrace cannot attach, the
positive control shows no socket and the test skips instead of reporting a clean
trace. Observed here, verbatim:

```
dtrace produced no socket syscall for a program that provably opens one, so it
is not attached on this machine and its verdict about dira would be worthless
(error: dtrace wrote no trace file ...; its stderr was:
	dtrace: system integrity protection is on, some features will not be available
	dtrace: failed to initialize dtrace: DTrace requires additional privileges).
```

**Closing this is E1-L6-T4's job**, and it is a specific one rather than a
general aspiration: the perf job must include a Linux runner with `strace`
installed, because that is the only platform where this check is currently more
than a skip. A green CI run there is the first time the socket claim will have
been observed at all.

## Running it

```
go test -tags perf -count=1 -p 1 ./internal/perf -run TestTheBriefOpensNoSocket -v
```

`-p 1` and the `perf` build tag for the reason `doc.go` gives: this package is
kept out of `go test ./...` so it never competes with the rest of the suite. A
skip is reported with its reason in the test output; read it rather than reading
`ok` as a pass.
