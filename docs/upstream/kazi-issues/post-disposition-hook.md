---
status: awaiting-maintainer-approval
posted: false
target_repo: kazi-org/kazi
kind: enhancement
duplicate_check: >
  Run 2026-07-30. `gh issue list --repo kazi-org/kazi --state all --search` for
  "approve reject hook", "post-disposition", "dira". No results, no duplicate.
---

# Title

Optional post-disposition hook on `approve` / `reject`

# Body

`kazi approve <ref>` and `kazi reject <ref>` are the single front door through which a
proposal's fate is decided. There is currently no way for a third party to observe that
a disposition happened.

## The ask

An optional configured command, invoked synchronously after a successful disposition,
receiving the proposal ref and the new state. Something as small as:

```toml
[hooks]
post_disposition = "some-command"
```

invoked as `some-command <proposal-ref> <approved|rejected>`, with a non-zero exit
logged and ignored so a broken hook can never block a disposition or fail the command.

## Why synchronous and direct, explicitly NOT the session bus

An earlier draft of this ask proposed riding ADR-0067's session bus with a `bus post` on
disposition, on the reasoning that it needed no new infrastructure. That was wrong on
checking, and the reason is worth stating so it is not re-proposed:

- The bus is hosted by `kazi daemon`. ADR-0067 states that everything on it "degrades
  gracefully when the daemon is not running: bus surfaces report 'no daemon' cleanly and
  no-op." A disposition made with the daemon down would vanish silently, with nothing to
  indicate it had. For a consumer whose purpose is not losing decisions, a transport that
  silently no-ops is disqualifying.
- The bus is also the wrong shape and durability: its event/intent stream is age-bounded
  to roughly 24h, messages are capped near 1 KB, and reads return a digest rather than a
  transcript. Correct for ephemeral coordination chatter; wrong for a durable record.

A direct synchronous invocation has neither problem and requires no daemon.

## Why it might reasonably be refused

Stating this so the ask is not read as a blocker. kazi's README draws a deliberate
boundary — kazi will never "decide what to build — that's your judgment" — and a hook
inviting external tools to observe dispositions could read as another product colonising
kazi's surface with its own concerns. That is a fair objection.

**Nothing is blocked if this is declined.** The consumer's fallback is to wrap
`approve`/`reject` at the skill layer, or to diff proposal state between sessions. Worse
ergonomics, same outcome.

## Precedent already in the codebase

`kazi install-hooks` (`cli.ex:440`, ADR-0071/ADR-0076) already establishes that kazi
integrates with external hook consumers, with a careful contract — merge-never-clobber,
idempotent, operator keys preserved byte-identically, `--uninstall` reverting exactly.
This ask is smaller than that machinery.

## Scope

Additive, optional, non-breaking. Absent configuration, behaviour is unchanged.
