# dira — product marketing context

**Document version:** 1.0.0 · **Last updated:** 2026-07-29

The shared context every other marketing skill reads before acting. If this doc and a
campaign disagree, fix one of them — don't let them drift.

> **Provenance note.** dira is pre-product: no binary exists, so there are no users,
> no analytics, and no customer interviews behind this document. Everything below is
> **positioning hypothesis derived from the founder's own documented pain** (see
> `.dira/entries/int-0001.md`) and from verified facts about the sibling product kazi.
> Treat the ICP language as unvalidated until real users say it back. Where a claim
> needs evidence we don't have, it is marked **[UNVALIDATED]**.

---

## 1. Product overview

**One-liner:** dira gives your AI coding agent a memory for *why*.

**What it does (3 sentences):** dira is a git-native ledger of the intents, decisions,
rejected alternatives, and open questions behind a codebase — captured automatically by
your coding agent, not typed by you. It surfaces that context at the start of every
session, so you never re-explain a decision you already made. And it blocks plans that
contradict decisions you already rejected, quoting your own past reasoning back at you.

**Category:** *decision memory for AI coding agents.*

We are deliberately **not** filing under:
- "context management" — abstract, crowded, and nobody searches for it with intent
- "ADR tooling" — a dead category people associate with documents nobody reads
- "project management" / "issue tracking" — that is kazi's and Linear's shelf, and
  entering it invites a comparison we lose

New-category framing is a bet: it costs search volume today and buys ownership if it
lands. Chosen because the existing shelves are actively harmful to the pitch.

---

## 2. The problem, in the customer's own words

The pain is specific, frequent, and emotionally charged for anyone running a coding
agent hard. Phrasings to use verbatim in copy:

- "I've explained this to Claude four times."
- "It keeps suggesting the thing we already decided against."
- "We agreed on this last week and now it's arguing the opposite."
- "I have 80 ADRs and I've never re-read one."
- "I don't know what we decided, only that we decided something."
- "Every new session starts from zero."

**The mechanism behind the pain:** agents have no memory across sessions, and the
human's memory is worse than they think. So rationale has to live somewhere — and every
existing place to put it is a *file*, which means retrieval is pull-based. Iteration
outruns reading. The record exists and goes unread, which is the worst of both worlds:
you paid the cost of writing it and got none of the benefit.

**Why now:** the population of people who feel this weekly went from ~zero to large in
about eighteen months, because agentic coding made "hours of brainstorming per day"
normal. The pain is new, sharp, and has no incumbent. **[UNVALIDATED — sizing is
inferred from Claude Code's visible ecosystem growth, not measured.]**

---

## 3. ICP

### Primary: the heavy agentic developer
- Runs Claude Code (or Cursor/Codex) daily, often several sessions at once
- Solo founder, indie dev, or tech lead on a small team
- Ships fast, iterates faster, and has *already tried* ADRs, Obsidian, Linear, or a
  `docs/decisions/` folder — and abandoned each
- Comfortable with a CLI, git, and installing a binary
- Already keeps context files (`CLAUDE.md`, `AGENTS.md`) — proof they believe in
  feeding context to agents

**Qualifying signal:** they can name a decision their agent re-litigated. If they
can't, they are not ready and no amount of copy will convert them.

**Disqualifying signal:** they want a task tracker. Send them to kazi or Linear.

### Secondary: the OSS maintainer
Wants contributors to stop asking "why is it built this way?" — a public dira ledger
answers it once. This segment matters disproportionately because they generate the
public artifacts the growth loop runs on (§7).

### Tertiary, later: small engineering teams
Shared decision memory across people, not just sessions. This is where the money
eventually is, and where a subscription becomes defensible (`dec-0007`). Not the
launch ICP — do not build for them yet.

---

## 4. Positioning

**For** developers who work with AI coding agents every day,
**who** are tired of re-explaining decisions the agent has forgotten and re-arguing
choices they already settled,
**dira is** a decision memory layer that lives in your repo,
**that** captures the *why* automatically as you work and pushes it back into every
session before you ask,
**unlike** ADRs, spec docs, and notes apps, which capture rationale into files nobody
ever reopens,
**dira** is written by the agent, read by the agent, and enforces itself — so the
record actually changes what gets built.

### The three inversions (the substance of the differentiation)

| Everyone else | dira |
|---|---|
| You write the record | **The agent writes it.** Your whole job is confirm/ignore |
| You go read the record | **The record comes to you** — one screen, every session start, capped forever |
| The record is a document | **The record is an enforcer** — it blocks contradictions before work is planned |

Inversion 3 is the one nobody else does at all, and it is the demo (§6).

---

## 5. Messaging hierarchy

**Primary hook (pain-first, always lead here):**
> Your coding agent has amnesia. You keep re-explaining decisions you already made —
> and it keeps suggesting the thing you rejected in July.

**Primary value statement:**
> dira remembers why, so you never explain it twice.

**Tagline candidates**, in order of preference:
1. **"Never explain the same decision twice."** — benefit-first, zero jargon, no kazi
   dependency. Recommended default.
2. "Your agent forgets. dira remembers why."
3. "Stop relitigating decisions with your AI."
4. "The compass for agentic development." — evocative, matches the Swahili meaning, but
   vague on its own. Use as a sub-line, never as the lead.

**Retire from front-line copy:** *"kazi proves it's done; dira remembers why you wanted
it."* It is an elegant line and it stays in the kazi-adjacent docs — but as a headline
it only lands for people who already know kazi, which at launch is nearly nobody. A
value prop that requires a second product to parse is not a value prop. This is a
change from the current README.

**Three proof pillars:**
1. **Zero clerical work** — hooks capture it; you press confirm. If it needed
   discipline, you'd have kept using ADRs.
2. **It reads what you already have** — point it at an existing repo and it builds the
   graph from your ADRs, commits, and plan files. Value before you've written anything.
   **[UNVALIDATED — DO NOT SHIP THIS CLAIM YET.]** `qst-0003` is open: nobody has
   established whether importing an ADR corpus yields a useful ledger or a second pile.
   If import produces entries with empty `alternatives` arrays, this pillar is false and
   the enforcer cannot enforce. `dec-0010` promotes it to a launch blocker precisely
   because this is the day-one acquisition moment. **Until qst-0003 is answered, ship
   two pillars, not three** — which is what the landing page does.
3. **It argues back** — the only tool that stops a bad plan by citing your own past
   reasoning.

---

## 6. The demo (the single most important marketing asset)

One clip, under 20 seconds, no narration needed:

```
$ dira check "add a background daemon to track run state"

✗ conflicts with dec-0060 (accepted 2026-07-03)
    rejected alternative: "a daemon"
    why_not: violates the single-binary intent (int-0002)
    revisit_if: cold-start latency stops being the binding constraint

→ supersede dec-0060, or revise the plan
```

Why this and not the brief or the map: it is the only moment that produces a felt
emotional beat — *"it caught me"* — in a single frame, with no setup and no prior
context. Briefs and maps require explaining what you're looking at. This does not.

**Secondary clip:** `dira init` on a real repo with existing ADRs, rendering the
decision graph and flagging contradictions already present in it. This is the "value in
60 seconds" clip and it is what makes the tool feel worth installing *today* rather
than in three months.

---

## 7. The growth engine

Three structural problems must be answered honestly before any channel work, because
each one kills growth on its own:

| Problem | Why it kills growth | The answer |
|---|---|---|
| **Invisible by design** — "you never open a file" | Nothing to screenshot, nothing to share, no surface to land on | The ledger **renders**. A read-only web view of any public `.dira/` turns every adopting repo into a shareable, linkable, indexable artifact |
| **Delayed value** — week 12 benefits from week 1 | Nobody adopts on a promise of future payoff | Lead with **retrospective** value: import what exists and report the contradictions already in it. Insight on day one, from data the user already has |
| **Fatal empty state** — a fresh ledger says nothing | First run is an anticlimax; users churn immediately | `dira init` must never produce an empty ledger. Import + interview are the *product*, not setup |

### The loop

```
dev points dira at a repo
   → gets their decision graph + contradiction report in 60s   (day-1 value)
      → hooks keep it current at zero effort                    (retention)
         → ledger is committed, public in OSS repos             (artifact)
            → renders as a browsable why-graph others can read  (distribution)
               → visitors see it, want it for their own repo    (acquisition)
```

This loop is architecturally free: the ledger is *already* committed to git by design
(`dec-0002`), and GitHub is *already* the sync layer (`dec-0005`). The only new piece is
a renderer.

**The renderer does not violate `cst-0004`.** That constraint says dira must never
*require* a hosted service to function. A read-only public renderer is additive — the
CLI works identically without it. This distinction must be stated explicitly wherever
the renderer is discussed, or it will read as a broken promise to exactly the audience
that checks.

### Channels, in priority order

1. **Claude Code ecosystem adjacency** — highest-leverage by a wide margin. The ICP is
   definitionally already there: skills/plugin marketplaces, the awesome-lists, hook
   showcases. kazi already ships as a Claude Code plugin and has the distribution
   pipeline built; dira should ride the same rails.
2. **kazi's existing audience** — the exact ICP, already warm, already trusting the
   author. Cross-linked docs, a shared tap, a joint story.
3. **One strong Show HN / X post built on the demo clip** — not a launch campaign, a
   single sharp artifact. The pain statement *is* the headline.
4. **Public ledgers as long-tail SEO** — every rendered decision page is a page
   answering "why did project X choose Y." This compounds and costs nothing per unit.
   **Demoted out of the tested inner ring, 2026-07-30**, on the E8-L1 planner's argument
   and I agree with it: this channel's mechanism is `dira render`, which does not exist
   (E6). Spending one of three concurrent test slots on a channel that cannot start
   wastes the slot. It stays a named candidate with an explicit promotion trigger — the
   day E6 ships. Build-in-public ship-notes took the slot instead (ICE 210 vs 180), being
   the only inner-ring experiment runnable before the binary exists.
5. **OSS credibility** — the repo's own `.dira/` ledger is a live demo of the product
   working on itself. Nobody else can show this.

### What we are honestly *not* claiming

Developer tools do not go viral the way consumer apps do. There is no invite mechanic,
no social graph, and no k-factor here. What is achievable is **compounding organic
distribution** off a sharp hook plus published artifacts, inside a fast-growing
ecosystem. Copy and internal targets should say that. Promising virality sets a bar
that will be missed and read as failure. **[UNVALIDATED — no channel has been tested.]**

---

## 8. Pricing & business model

Per `dec-0007`: OSS core free forever → one-off purchase for iOS + desktop apps → team
subscription only when teams pull for it. Data on-device or in the user's own GitHub.

**Two honest tensions to hold, not paper over:**

1. **The growth engine and the revenue engine are different things.** The CLI drives all
   adoption and earns nothing. A paid companion app for a CLI tool monetizes a thin
   slice of an already-niche audience. This model is right for trust and wrong for
   revenue scale — which is an acceptable trade for a solo maintainer, but it should be
   a *chosen* trade, not a discovered one.
2. **The team tier is where the money is, and we are deliberately not building it yet.**
   Shared decision memory across people is a stronger commercial product than
   single-player memory. Sequencing it last is defensible (validate the core first), but
   it means revenue is deferred well past launch.

**Never** charge for: the CLI, hooks, the skill, `dira ui`, or the public renderer.
These are the entire adoption engine.

---

## 9. Objections, with honest answers

| Objection | Answer |
|---|---|
| "I already have ADRs." | So does the author — 83 of them, unread. The problem was never capture, it was that nobody re-reads them. Whether dira can usefully *import* that back-catalogue is an open question (`qst-0003`) the project is working through in the open — do not answer this objection as though import ships today. |
| "Another tool to maintain." | It replaces tools rather than adding one. And you don't maintain it — the agent writes it. |
| "My agent will write garbage rationale." | Regex-tier capture is always *staged*, never accepted; you confirm. Quality does depend on the agent driving it — stated plainly, not hidden (`dec-0003`). |
| "Why not just a bigger CLAUDE.md?" | A context file has no lifecycle, no supersession, no rejected alternatives, and cannot enforce anything. It also grows until it stops being read — which is the original failure. |
| "Is this lock-in?" | Plain markdown in your own repo. Delete the binary; the record is still readable. Apache 2.0. |
| "Does my private strategy leak into public repos?" | Inheritance is one-way and read-time only; a violation is treated as a security bug (`cst-0003`). |

---

## 10. Voice

Match kazi's register: precise, unhyped, evidence-first, willing to state limits. This
audience is allergic to marketing language and rewards specificity — "83 ADRs, unread"
outperforms "better decision tracking" every time.

- **Do:** name real numbers, show real terminal output, state what the tool won't do
- **Do:** lead with the pain in the user's words
- **Don't:** say "revolutionary," "seamless," "supercharge," "10x," or "AI-powered"
- **Don't:** promise virality, or imply the hosted renderer is required
- **Don't:** lead with kazi. dira must stand alone first.

---

## Changelog

- **1.0.0** (2026-07-29) — Initial context. Key calls: new category *decision memory
  for AI coding agents*; pain-first hook replacing the kazi-dependent tagline as the
  lead; growth engine reframed around retrospective import (promoting `qst-0003` from
  deferred risk to the primary acquisition moment) plus a public ledger renderer;
  virality explicitly disclaimed in favour of compounding organic distribution.
