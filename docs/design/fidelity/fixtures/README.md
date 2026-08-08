# The design fixture ledger

`ledger-design/entries/` is a **fixture**, not a ledger. It exists so a pixel
comparison between a Go-served page and a mockup is comparing layout rather than
prose: both sides render the same eighteen entries. `.dira/` — the repo's real
ledger — is untouched by anything here.

```
node docs/design/scripts/fixture-check.mjs        # content equality vs the mockups
node docs/design/scripts/fixture-check.mjs -v     # every string, and where it matched
```

One file per entry (`dec-0002`). Eighteen entries reproduce the three screens:

| what | entries |
|---|---|
| the three `s2-index` group headings | `int-0001`, `int-0002`, `int-0003` |
| their children, as index rows | `dec-0001`–`dec-0010`, `cst-0001`, `qst-0003` |
| the full decision page (`s1-decision`) | `dec-0001`, with its three refusals, grounds and revisit conditions |
| the distill queue (`s3-distill`) | `dec-0011`, `dec-0012` (staged), `qst-0006` (open, agent-captured) |
| **the orphan behind the drift row** | `int-0004` |

`int-0004` has **no `derives_from` edge, deliberately.** That absence is what
renders the drift row on `s2-index` and the "dira flagged this" card on
`s1-decision`. Giving it a parent would delete the state the fixture exists to
render — which is exactly the shape of "editing the reference until the gate
passes".

## Status of the two checks

**Content equality: gated.** `fixture-check.mjs` runs in `gates.mjs`. It asserts
every shared string byte-equal after tag-stripping and entity decoding, and it
re-checks its own declared exceptions each run, so an exception that quietly
becomes checkable is reported as a failure rather than left as a hole.

**Schema validity: gated as of E6-L2.** `TestDesignFixturesValidate` in
`internal/ui/fixtures_test.go` validates all eighteen entries against
`schema/entry.schema.json` on every `go test`. It asserts the count as well as
the validity, because without that it would pass just as happily on a directory
that had been moved or emptied.

`TestTheValidatorRejectsTheInvalidCorpus` is its two-sided control: every file in
`schema/testdata/invalid/` must still be refused, so a validator that accepted
everything cannot pass the first test quietly. `TestTheRealLedgerValidates`
extends the same guarantee to `.dira/` itself.

```
18 files, 0 failures        # docs/design/fidelity/fixtures/ledger-design/entries
18 files, 18 refused        # schema/testdata/invalid  — the two-sided control
```

The tests live in `internal/ui` rather than in `schema` because `internal/ui` is
what serves these fixtures: the thing that consumes the fixture is the thing that
should refuse an invalid one.

## The upheld option had no schema field — closed by `dec-0019`

**Resolved. Kept as a record because the reasoning is reusable and because it is
the clearest example of what this fixture ledger is for.**

`s1-decision.html` used to render **four** alternative cards under "Alternatives
on record — 4". Three were refusals. The fourth was the *upheld* option — "Go —
one static binary, sub-100 ms cold start" — with a `tag` of `upheld`, its own
summary line, and its own grounds paragraph.

`entry.schema.json` models `alternatives` as *the roads not taken*: each item
requires `option` and `why_not`, documented as "the actual reason" a road was
rejected. The chosen option had nowhere to live — and its grounds ("The standard
library already covers files, JSON and YAML…") were not the decision's `because`
either, which on that page is a different paragraph about hook latency. So no
ledger could have produced that card, and the fixture carried three alternatives
against a mockup that drew four.

**`dec-0019` settled it: the upheld option is the ruling, not an alternative.**
The card is gone; the page now reads "Alternatives on record — 3" and the fixture
matches it exactly. The chosen road is the `<h1 class="ruling">` — the largest
element on the page — and the case for it is recorded inside the refusals it
answers. The chain's matching `✓` row went with it, on the grounds that `dira why`
prints `✗` lines and never a `✓`, so one producer stands behind both renderers.

That is a stronger argument than "drop the card", which is how this option was
originally written up here, with a warning that it risked the struck-refusal
device `DESIGN.md` defended in r3 → r4. It does not: the strike survives, because
a refusal is still struck through *against a ruling*, and the ruling simply moved
to where it was always largest. The two rejected alternatives are recorded for
the same reason any rejected alternative is:

1. **Synthesize the upheld card from the decision itself** — title as the name,
   `because` as the grounds. Cheapest, and it silently changes what the page says:
   the grounds paragraph on that card is not the `because` and would be lost.
2. **Add a field** (`upheld`, or an `alternative.state`). Requires superseding a
   schema decision, and `additionalProperties: false` means nothing works until
   the schema moves — a large change to make a rendering detail expressible.

`fixture-check.mjs` printed this as a FINDING on every run rather than failing,
because it was a modelling gap and not a content mismatch. **The finding is now an
assertion**, in both directions of the seam:

- `fixture-check.mjs` fails any screen carrying an `upheld` tag, an
  `class="alt upheld"` card, or a `✓` mark in an alternatives or chain block. It
  is scoped rather than a blanket ban on the character: `s1-decision-withheld.html`
  legitimately marks the *oriented* resolution state with a `✓` (`dec-0011`,
  `dec-0018`), which is a fact about an edge and not a claim about an alternative.
- `TestNoUpheldCardIsEverRendered` in `internal/ui` refuses the same on the served
  side, so the mockup and the template cannot drift apart when only one is edited.

Both were watched failing before they were trusted passing. A resolved finding
that nothing re-checks is a finding that comes back.

It surfaced at fixture-build time rather than halfway through writing the
renderer, which is the entire argument for building the fixture ledger before the
template rather than alongside it.

## Still open: the mockups are not reproducible from this fixture

Recorded because it is why the pixel half of E6-L2's `acc:` line does not pass,
and because it is the same shape as the closed finding above rather than a new
kind of problem. Three classes of string in the mockups have no source here:

1. **The `.chain` and `.chain-stack` blocks are hand-abbreviated.** "BEAM
   start-up in a hook latency path", "✗ elixir/otp", "slower to write, smaller
   contributor pool" — none appear in any fixture entry, and no derivation
   produces them from the `why_not` they summarise. `fixture-check.mjs` never
   looked at the chain: it checks titles, bodies and alternative fields.
2. **Every alternative's `.one` line is editorial.** "free CI and free
   distribution, paid for in a latency path with no concurrency to supervise" is
   not in the fixture either. The Go renderer derives that line as the *first
   sentence of `why_not`* — honest, stores nothing, duplicates nothing — but it
   is not the mockup's sentence, so the two do not match character for character.
3. **Execution status has no source, and may not be given one.** "converged",
   "3/3 predicates green", the dial's "43%" and the index's "in motion" are all
   `dec-0004` joins with kazi. dira embeds no kazi client (`dec-0003`) and E4 does
   not exist, so the served pages show the ledger-side buckets and state that the
   join is unavailable — `dec-0004`'s own degradation rule, and visibly different
   from the mockup.

The consequence is measured rather than asserted:
`node docs/design/scripts/uigate.mjs` regenerates the mockup baselines and
captures the served pages in the same run, and reports a **dimension mismatch on
all twelve pairs** — the widths agree exactly at every viewport, the heights do
not. See `docs/decisions-pending/E6-L2-report.md`.
