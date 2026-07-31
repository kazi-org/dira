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

**Schema validity: verified, not yet gated.** All eighteen entries validate
against `schema/entry.schema.json` through the repo's own Go validator, and the
same validator rejects all seventeen of `schema/testdata/invalid/`:

```
18 files, 0 failures        # docs/design/fidelity/fixtures/ledger-design/entries
17 files, 17 failures       # schema/testdata/invalid  — the two-sided control
```

That run used a throwaway module outside the repo so no `.go` file was added
here. Making it permanent is a **Go test, and belongs to E6-L2**, which owns the
Go surface. It is roughly:

```go
func TestDesignFixturesValidate(t *testing.T) {
	v, err := schema.NewValidator()
	// ... walk docs/design/fidelity/fixtures/ledger-design/entries/*.md,
	//     v.Validate(contents), t.Errorf on any error.
}
```

Until that test exists, schema validity of these fixtures is **checked but not
enforced**, and a future edit could break it silently.

## Finding for E6-L2: the upheld option has no schema field

`s1-decision.html` renders **four** alternative cards under the heading
"Alternatives on record — 4". Three are refusals. The fourth is the *upheld*
option — "Go — one static binary, sub-100 ms cold start" — with a `tag` of
`upheld`, its own one-line summary, and its own grounds paragraph.

`entry.schema.json` models `alternatives` as *the roads not taken*: each item
requires `option` and `why_not`, and `why_not` is documented as "the actual
reason" a road was rejected. The chosen option therefore has nowhere to live —
and its grounds ("The standard library already covers files, JSON and YAML…") are
not the decision's `because` either, which on that page is a different paragraph
about hook latency.

So the fixture carries **three** alternatives for `dec-0001`, and the upheld card
is currently unreproducible from ledger data. `fixture-check.mjs` prints this as
a FINDING on every run rather than failing, because it is a modelling gap, not a
content mismatch.

E6-L2 has to resolve it before writing the template. The options, none of them
free:

1. **Synthesize the upheld card from the decision itself** — title as the name,
   `because` as the grounds. Cheapest, and it changes what the page says: the
   grounds paragraph currently on that card would be lost.
2. **Add a field** (`upheld`, or an `alternative.state`). Requires superseding a
   schema decision, and `additionalProperties: false` means nothing works until
   the schema moves.
3. **Change the mockup** to drop the upheld card. Do not do this quietly — the
   struck-through refusal *against* an upheld option is the device `DESIGN.md`
   records defending in r3 → r4, and the design is locked.

This is the kind of thing the fixture ledger is for: it surfaced at fixture-build
time rather than halfway through writing the renderer.
