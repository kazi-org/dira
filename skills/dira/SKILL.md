---
name: dira
description: >-
  Capture tier 2 for dira, the git-native ledger of decisions and the
  alternatives they rejected. Invoke when a dira handoff block appears in
  context; when a session resumes after a PreCompact compaction and regex-staged
  decisions are waiting for their reasoning; before drafting a plan, so a
  settled decision gets a chance to refuse it; and when a kazi proposal is
  dispositioned with approve or reject, which nothing else records.
---

# dira — tier 2

There are two capture tiers. Tier 1 is a regular expression inside the
binary: it reads the transcript on `Stop` and on `PreCompact`, and it stages
what looks like decision language. It records a title and the sentence it came
from, and nothing else, because a pattern match is not evidence of intent.

Tier 2 is you. You have the conversation in context, so you are the only party
that can say **why** a road was taken and **which** road was refused. That is
the whole of this skill: turn a thin capture into an entry that can be cited.

There is no model client in the binary and no network call anywhere in this
loop. Nothing here fetches anything, and nothing here needs an account, a key
or a hosted service.

## The one rule that outranks everything else

**Never write a `why_not` that was not said.**

An invented rationale is worse than a missing one. The ledger is quoted back at
the person three weeks later as their own reasoning, and a plausible sentence
you constructed is indistinguishable, at that distance, from one they actually
wrote. If the transcript does not say why an option lost, you have no
alternative to record — leave the entry alone and say so. A staged entry with
no extraction is a visible gap. A fabricated one is an invisible lie.

The same rule covers the body: the *because* is quoted or closely paraphrased
from what was said, not reconstructed from what would have been sensible.

## When this runs

**A handoff block in context.** The block looks like this, and names the ids
that were staged:

```
=== dira handoff, tier 2 ===
...
=== end dira handoff ===
```

**A session that just resumed after a compaction.** This is the common case,
and it needs saying because the block itself may never reach you. The
`PreCompact` hook prints the block on stdout, and stdout from that hook is not
delivered to the session — it becomes custom instructions for the compaction
summariser, a one-turn call that is forbidden from calling tools (`dec-0023`
records that contract and the evidence for it). The capture still happened: the
staged entries are on disk. `SessionStart` fires again after a compaction — its
matcher list includes `compact` — and that session is the first reader of this
block that can act on it. So when a session starts after a compaction, the
pending work is in the ledger, not in the block. Find it by reading the entry
files:

```
grep -l 'tier: regex' .dira/entries/dec-*.md
```

The ones that matter carry `state: staged` and no `alternatives:`. Of those, the
ones carrying `confirmed_by: human` are the ones a person has already stood
behind — do those first.

**Before planning.** See *Drift check* below.

**On a disposition.** See *Dispositions* below.

## Writing the extraction

One call per staged id. Every flag below is one `dira log` defines:

```
dira log --kind decision --state staged --tier semantic --hook PreCompact \
  --title 'Checkpoint file for resume, not a daemon' \
  --body 'Chosen because a daemon is a second process to install, and the intent was one binary.' \
  --alternative 'A long-lived resume daemon' \
  --why-not 'It violates the single-binary intent; you said you would not ship a second process.' \
  --edge derives_from=dec-0060 \
  --edge-note 'the regex capture this extraction supplies the reasoning for' \
  --excerpt 'let us go with a resume-token checkpoint file, not a daemon'
```

`--alternative` and `--why-not` are positional: each `--why-not` attaches to the
`--alternative` immediately before it. `--revisit-if` attaches the same way and
is worth adding whenever the transcript says what would reopen the question.

If the body runs long, or carries quotes and newlines that argv will mangle,
write the whole entry as a document instead:

```
dira log --stdin
```

which reads the same frontmatter-plus-markdown file dira stores.

Set `--tier semantic` on everything you write here. That field is the record of
how the entry was extracted, and it is the reason a reader can tell what a
pattern guessed from what a reader of the conversation established.

Do not set the field that records who dispositioned an entry. Tier 2 proposes;
a person disposes. Writing it yourself would put your extraction beyond the one
review it exists to receive.

## Why this creates a second entry, and what joins them

`dira log <id>` adds edges and tags to an entry that already exists, and
nothing else — no body, no alternatives, no state. That is deliberate: editing
an entry's text is a job for an editor, and changing its state is a
disposition, not a write.

So the extraction is a **new** entry that points at the capture with
`--edge derives_from=<staged id>`. The thin capture stays exactly as the regex
tier wrote it, which is correct rather than unfortunate: its `source.tier` is
the record that a pattern found it, and rewriting that to claim an extraction
would forge the provenance the two-tier split exists to preserve.

The pair is a graph, not two orphans. `dira why` renders incoming
`derives_from` edges, so the capture names its extraction and the extraction
names its capture:

```
dira why dec-0060
```

Retiring the thin capture in favour of the extraction is a person's move, not
yours, and the binary enforces that: `dira supersede` refuses a replacement
that is still staged. Leave both entries in place and say, in one line, that
the extraction is waiting on review.

## Drift check before planning

Run this before a single predicate is drafted, not after:

```
dira check "runs resume after restart from a checkpoint file"
```

It matches the plan against accepted decisions' rejected alternatives, against
rejected decisions, and against active constraints. Exit 0 means nothing in the
ledger refuses the plan. Exit 2 means at least one cited conflict — read the
citation, and either revise the plan or supersede the entry that refuses it.
Exit 1 means the check did not run at all and is never a verdict.

The matching is lexical and happens inside the binary. It is not asking you,
and you must not substitute your own judgement for its exit code.

## Dispositions

kazi has no post-disposition hook yet (`qst-0005`, ask 2). Until it does,
approving or rejecting a proposal is a decision event that nothing records. So
when you run either:

```
kazi approve prop-resume-8a1f
kazi reject prop-resume-8a1f
```

log the disposition in the same turn, from what was actually said about it:

```
dira log --kind decision --state accepted --tier semantic --hook manual \
  --title 'Approved prop-resume-8a1f' \
  --body 'Approved after the third predicate was narrowed to the restart path.' \
  --alternative 'Rejecting it and re-planning around a daemon' \
  --why-not 'Same objection as the original decision: a second process to install.' \
  --edge realized_by=kazi:prop-resume-8a1f
```

An accepted decision needs at least one alternative with a non-empty `why_not`,
so if the disposition was made without a stated reason, ask for one rather than
supplying it.

## What this skill never does

- It never invents a `why_not`, a body, or an alternative. See the top.
- It never records who confirmed an entry. That is a person's keystroke.
- It never changes an existing entry's state, kind, title or body.
- It never calls a network service, fetches a page, or reaches for a hosted
  model. dira's whole premise is that the record is local files and a binary.
- It never writes to a parent ledger. A namespaced ref belongs to somebody
  else's repository.
