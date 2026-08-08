# The design fixture ledger

The 18 entries here are the content the mockups in `docs/design/screens/` and
`docs/design/landing/` are drawn from. `scripts/fixture-check.mjs` asserts every
rendered string is byte-equal to its source, so that a pixel diff between a mockup
and a built page measures **layout**, and never prose that drifted.

## These ids do not mean what the same ids mean in `.dira/`

This is a standalone corpus, frozen at the moment the mockups were drawn. It is not
a copy of this repo's real ledger and does not track it. Ids overlap and the overlap
is not meaningful:

| id | here | in `.dira/` |
|---|---|---|
| `qst-0006` | should the brief inherit personal focus into a public repo session? | the edge set has no way to say "narrows without replacing" |

Do not reconcile them. Editing a fixture entry to match the real ledger breaks the
byte-equality the gate exists to enforce; editing it for any other reason changes
what the mockups are held to and must be done together with the mockup.

## Adding an entry

Add the file, render it into a mockup, then run the gate. An entry that no mockup
renders must be declared in `fixture-check.mjs`'s exception list with the reason it
is not shown — the gate re-checks those declarations on every run, so an exception
that stops being true fails rather than lingers.
