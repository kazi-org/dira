---
status: awaiting-maintainer-approval
posted: false
target: news.ycombinator.com
kind: show-hn
owner: E8-L6
---

**Do not post this until `status` above is flipped by a human. This file is a draft
awaiting maintainer approval, not a send.**

## Title

Show HN: dira – a git-native decision ledger your coding agent writes for you

## First comment

Hi HN, I'm the author. I built dira because I kept relitigating decisions I'd already
made with my coding agent — week 12 relitigating week 1, because the agent remembers
nothing across sessions and I wasn't going to re-read 40 ADRs to check.

dira is a git-native ledger of intents, decisions, rejected alternatives, open
questions, and constraints — one markdown file per entry, YAML frontmatter for the
machine, prose for the *because*. It sits one layer above execution, upstream of a
declared goal (my other project, kazi, sits downstream — proves a goal is objectively
done, and deliberately refuses to decide what to build).

Three things it does that ADRs and issue trackers don't:

```
$ dira check "add a background daemon to track run state"
✗ conflicts with dec-0060 (accepted 2026-07-03)
    rejected alternative: "a daemon"
    why_not: violates the single-binary intent (int-0002)
    revisit_if: cold-start latency stops being the binding constraint
→ supersede dec-0060, or revise the plan
```

`dira check` refuses a plan that contradicts a settled decision, quoting the original
reason, before the plan gets built — not after someone notices in review.

```
$ dira why elixir
int-0002  Zero-ceremony operation — one binary, no server, no daemon
└─ dec-0001  Go, not Elixir/OTP, despite kazi's stack
   ├─ ✗ Elixir/OTP, reusing kazi's Burrito + Homebrew tap + release-please pipeline
   ├─ ✗ Rust
   ├─ ✗ A shell script or Python
   └─ ✗ A TypeScript CLI on Node/Bun
```

`dira why` prints the chain: the intent a decision serves, and every alternative it
refused, with the reason. Only rejections are listed — what was chosen is the decision
itself.

**Architecture, and the tradeoffs I made on purpose:**

- **No model client in the binary.** dira never holds an API key and never makes a
  network call to extract anything. Semantic capture (the *because*, the rejected
  alternatives) is delegated to the coding-agent session that's already running, via a
  Claude Code skill — the model is already there, in context, for free, at the exact
  moment the decision is made. The binary itself only ever does cheap, offline,
  deterministic regex staging. The honest tradeoff: **capture quality is a function of
  the agent driving it.** With no agent, dira degrades to `dira log` typed by hand —
  still useful, but the automatic-capture story belongs to the agent-driven path.
- **One file per entry, not an append-only log.** Two sessions logging concurrently
  produce two files git merges without a conflict, and a reviewer sees one small diff
  per decision instead of a line inside a growing ledger file. The cost: more files on
  disk than a single log would have. I think that's the right trade for something meant
  to be read in git history and in PR diffs.
- **No server, no daemon, no account.** Every core command works with the network
  unplugged. There is no dira-operated host anywhere — GitHub is already the sync layer
  for anyone using this, so I didn't build a second one.

**Known limitations, stated plainly — not softened:**

- **Capture quality depends on the agent driving it** (`dec-0003`). The regex tier that
  runs without an agent present catches decision *language* but can't fill in the
  *because* — that's the semantic tier's job, and it needs a live session to run in.
- **Import quality depends on how your ADRs are written.** `dira import <dir>` measures
  a corpus before writing anything, and the yield genuinely varies: across five real
  repositories I measured, the share of documents recording a reasoned rejection ranged
  from 90% down to **zero** (`dec-0028`). Some corpora yield nothing dira can enforce,
  and it says so before writing anything, rather than importing 30 empty entries and
  calling it a win. If your ADRs read like textbook Nygard with no rejected options
  recorded, import will tell you that upfront and offer to index the documents instead
  of pretending to enforce something they never contained.
- **No team tier yet.** The OSS core (CLI, hooks, the skill, `dira ui`) is free forever;
  a per-seat team tier is planned but not built (`dec-0007`). If you need shared review
  queues or an org dashboard today, dira isn't that yet.

Repo: https://github.com/kazi-org/dira — Apache 2.0, `go build ./cmd/dira` and go, no
`brew install` yet. Happy to answer anything.

<!-- honest-limits:start -->
This buys compounding organic distribution, not virality — there's no invite mechanic,
no social graph, no k-factor in a single-player developer tool, and this post isn't
claiming otherwise.
<!-- honest-limits:end -->
