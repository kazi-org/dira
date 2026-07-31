# qst-0003 — does the kazi finding survive on corpora nobody here wrote?

**Partly. The corpus-quality finding does not generalise — it ranges from 0% to 90%.
The `revisit_if` gap does generalise, completely. And the label problem, which was my
load-bearing conclusion last time, does *not* generalise: it was largely kazi house
style, and I was wrong to put it in the importer's spec.**

The result that matters most for launch is **nulib/meadow: 31 ADRs, textbook Nygard,
zero recorded alternatives.** Importing it produces exactly the second pile qst-0003
feared. Nygard is the original and most-copied ADR template, so this is not an edge
case.

---

## 1. Corpora measured

All read-only, shallow-cloned into the session scratchpad. None forked, starred, or
interacted with.

| corpus | URL | commit | house style |
|---|---|---|---|
| **kazi** (baseline) | local | `b0b01576c10dea09134c25d6992b2c2c8beb5c39` | bespoke: `## Alternatives rejected`, bullet-per-option with inline grounds |
| **bbc/tams** | https://github.com/bbc/tams | `8cd1ca536322ce0941e58d2c67210b2c7cd3ee80` | **MADR**: `## Considered Options` (names) + `## Pros and Cons of the Options` (reasons, in a *different* section) |
| **Sylius/Sylius** | https://github.com/Sylius/Sylius | `e76516d17fb3a0b6164a41e6196d0b103f292bff` | **MADR variant**: `## Considered Options` with an H3 and prose per option |
| **nulib/meadow** | https://github.com/nulib/meadow | `10f6ac2cb3f3c4e2894c4cf5dbed67544516faf1` | **pure Nygard**: Status / Context / Decision / Consequences, nothing else |
| **IRS-Public/direct-file** | https://github.com/IRS-Public/direct-file | `e365f9f43446010af36cccf7edecc52953c00fe9` | **freeform / mixed**: Background, Evaluation Criteria, OSS Landscape; some MADR, some Nygard |

Four organisations, four unrelated domains (broadcast standards, digital preservation
at a university library, PHP e-commerce, US federal tax filing), and four house styles
including one that is not ADR-shaped at all. Deliberately **not** three descendants of
one template.

*(`IRS-Public/direct-file` was fetched file-by-file through the API rather than cloned —
the repo is large enough that a sparse clone timed out. Same commit, same content.)*

---

## 2. The instrument, and why the first one could not be reused

**The kazi experiment measured one corpus with an extractor shaped like that corpus.**
Pointed at MADR it reports a wall of bare names, because MADR puts the option *names*
under `## Considered Options` and the *reasons* under `## Pros and Cons of the Options`
as `* Bad, because …` bullets. That is a false negative in the direction that would
have wrongly killed the import.

So everything here, kazi included, was re-measured with one corpus-agnostic extractor
(`extract2.py`) handling four shapes: inline-reason bullets, sub-heading-per-option,
names-here-reasons-elsewhere, and no-section-at-all. **kazi's numbers in this report are
its re-measured numbers, not the ones from the first report**, so all five columns are
comparable.

### The controls, extended for the new shapes

The four hand-written controls carried over, plus two new ones for the format that
could produce the false negative:

| control | contains | extractor said |
|---|---|---|
| `0001-bare-names` | 5 options, names only | 5 **bare**, 0 reasoned ✓ |
| `0002-thin-reasons` | 3 options, label-length reasons | 3 **thin**, 0 reasoned ✓ |
| `0003-rich-with-revisit` | 3 options, full grounds + revisit conditions | 3 **reasoned**, 3 **revisit** ✓ |
| `0004-no-section` | no alternatives heading | 0 ✓ |
| **`c5-madr-reasons-elsewhere`** | MADR: bare option list, reasons in Pros-and-Cons | **1 reasoned** ✓ — the join works, and the chosen option is correctly dropped |
| **`c6-madr-no-reasons`** | MADR: bare option list, **no** Pros-and-Cons | **2 bare** ✓ — so C5 is a real join, not an always-pass |

C5 and C6 are the pair that matters: without C6, a join that returned a reason for
everything would look identical to a working one.

### Three bugs the controls and hand-reading caught

Reported because each would have changed a headline number:

1. **C5 failed on the first run** — MADR's `### Option 1: …` headings *inside*
   Pros-and-Cons were each treated as their own options section, so their
   "Good, because" / "Bad, because" bullets became alternatives. 6 alternatives
   extracted where there was 1.
2. **`Option 1: Assume DELETE requests will be mediated by other systems` was split at
   the colon** (found by hand on `bbc/tams` 0004), making the option `Option 1` and the
   descriptive name the *reason*. This alone inflated tams' unmatchable-label rate from
   1% to 20% — i.e. my headline answer to the lead's question 1 was, briefly, my own bug.
3. **IRS sub-sub-headings inside an option's body** (`#### Branches`,
   `##### Error workflow`) were counted as options.

Controls re-run green after each fix.

---

## 3. The numbers, one instrument, five corpora

| | **kazi** | **bbc/tams** | **Sylius** | **nulib/meadow** | **IRS direct-file** |
|---|---|---|---|---|---|
| ADRs | 80 | 49 | 26 | **31** | 35 |
| with an options/alternatives heading | 76% | 98% | 85% | **0%** | 34% |
| **with ≥1 reasoned rejected alternative** | **76%** | **90%** | **65%** | **0%** | **11%** |
| with nothing to extract | 19 | 2 | 4 | **31** | 30 |
| rejected alternatives extracted | 221 | 237 | 40 | **0** | 24 |
| …carrying a real `why_not` (≥8w) | 213 (96%) | 202 (85%)\* | 30 (75%) | — | 21 (88%) |
| …bare name, no reason | 3 | 29\* | **0** | — | 3 |
| …carrying a `revisit_if` | **2 (0.9%)** | **0 (0%)** | **0 (0%)** | — | **0 (0%)** |
| median `why_not` length | 23w | 27w | 21w | — | 64w |
| median option-label length | 6w | 11w | 6w | — | 8w |
| **options ≤2 words (unmatchable)** | **6%** | **1%** | **0%** | — | **8%†** |

\* tams' 29 "bare" are a floor artifact, not a corpus property: they are mostly variant
options (`Option 1a: As above, without a hierarchy model`) that carry a full descriptive
name but no separate Pros-and-Cons block for the join to find. Its 85% reasoned is a
lower bound.
† IRS raw count was 21%; hand-checked, 6 of the 8 are the sub-heading artifacts from bug
3's family and only `Liquibase` and `Flyway` are genuine short labels — 2 of 24.

---

## 4. The two questions the lead asked

### Does the label problem generalise? **No — and this reverses my recommendation.**

Unmatchable (≤2-word) option labels: kazi **6%**, tams **1%**, Sylius **0%**, IRS **8%**.
Median label length runs 6–11 words everywhere.

**kazi's `Go` / `Rust` / `Redis` terseness is house style, not a property of ADRs.** MADR
corpora name options descriptively by convention (`Option 2: Make the Flow timeline
represent the presentation timeline`), and Sylius has literally zero short labels in 40
alternatives.

Confirmed by citation rather than inferred from label length — the same proxy matcher,
same overlap≥2 threshold, same negative control:

| corpus | plan proposed | result |
|---|---|---|
| tams | *"make the flow timeline represent the decode timeline"* | **cites 0013** ✓ |
| tams | *"add an is_deleted flag on sources and flows"* | **cites 0004** ✓ |
| tams | *"add a dark mode toggle to the dashboard header"* | **silent** ✓ |
| Sylius | *"use the FQCN as a service id"* | **cites services_naming_convention** ✓ |
| Sylius | *"add a dark mode toggle to the dashboard header"* | **silent** ✓ |

3 of 3 real conflicts cited at perfect precision — **better than kazi's 2 of 4**, and for
exactly the predicted reason.

**So I was wrong to write "extraction-time expansion belongs in the importer's spec."**
It is a *conditional repair* for corpora that write terse labels, which the importer can
detect (median label length is one pass) and apply where needed. It is not a universal
requirement, and stating it as one would have loaded the importer with work most corpora
do not need.

### Does heading uniformity generalise? **Within a corpus, yes. Across corpora, no — and that is the finding that settles the build shape.**

Every corpus is internally consistent: tams 98% under one heading, meadow 100% under the
same four Nygard headings, Sylius 85%, kazi 76%. But **the heading is different in every
corpus, and two of the five have no options heading at all.**

The direct evidence is this experiment itself: my kazi-shaped extractor needed a
structural rewrite plus three bug fixes to survive four more corpora, and two of those
bugs were found only by reading the files by hand. **A regex importer is viable per
corpus and not viable in general.** That is a much stronger argument for semantic import
(option 2) than anything in the first report, and it is empirical rather than
speculative.

---

## 5. What changes in the answer to qst-0003

**Option 2 (semantic import) still stands, and is now better supported** — by the
heading-diversity result rather than by corpus richness.

**But the recommendation gains a mandatory precondition it did not have: a pre-import
dry-run.** The measured yield across five corpora ranges from **218 enforceable
`why_not`s (kazi)** to **literally nothing (meadow)**. "Import what you already have" is
a promise dira cannot keep for a whole class of repos, and Nygard — the template that
class follows — is the original and the most widely copied.

So the import must report what it *will* produce before producing it:

> `31 ADRs found. 0 record a rejected alternative. Importing them gives you 31 entries
> dira cannot enforce with. Index them instead?`

That is option **(1) index, don't import** — the entry's own first option — surfacing as
the correct answer *for some corpora*, chosen by measurement rather than by policy. The
three options in qst-0003 are not mutually exclusive; which one is right is a property
of the corpus, and it is cheaply measurable before anything is written.

**The `revisit_if` gap is confirmed universal and is worse than the kazi run suggested:**
0.9% in kazi and **0% in all four public corpora**, over 301 alternatives. The detector is
proven able to fire (control C3). No import will ever supply this field; every imported
entry refuses without offering a way forward.

---

## 6. What would still change this answer

- **A corpus in the 20–60% band.** The five results cluster at the ends (0%, 11%, 65%,
  76%, 90%). Whether the middle is populated decides whether the dry-run's advice is a
  clean binary or a judgement call.
- **Testing extraction rather than presence.** Still unmeasured, and still the largest
  gap: I measured what is *in* these corpora, not whether a live session pulls it out
  faithfully. A semantic importer that hallucinates a `why_not` is worse than no import,
  and nothing here tests that. This is now the top open risk.
- **A real `dira check` run**, once E3's enforcer lands, against entries imported from a
  non-kazi corpus.
- **Corpora outside GitHub-hosted OSS.** All five are public repos with a docs culture.
  A private corporate ADR pile may look nothing like any of them.

---

## 7. Files

```
scratchpad/neutrality/extract2.py        the corpus-agnostic extractor
scratchpad/neutrality/control/           six hand-written ADRs with known answers
scratchpad/neutrality/{tams,sylius,meadow}/   shallow read-only clones
scratchpad/neutrality/irs_adr/           IRS ADRs fetched by API at the pinned SHA
scratchpad/check_sim.py                  the citation proxy, reused unchanged
```

Nothing was added to `cmd/dira/` or `internal/`, nothing committed or pushed, and
`docs/roadmap.md` / `docs/coverage.md` were not edited. No public repo was forked,
starred, or otherwise interacted with.
