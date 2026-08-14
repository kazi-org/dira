# internal/status fixture corpus

Six small ledgers, each `<name>/.dira/entries/*.md`, openable with
`internal/ledger/local.Open` exactly like a real ledger. `TestFixtureCorpus`
(`fixture_test.go`) asserts every directory below exists and every entry in it
decodes and validates.

## real-snapshot/

A **verbatim, unedited copy** of this repository's own `.dira/entries/` at
authoring time — not a live pointer at it, because other worktrees' concurrent
lanes mutate the real one and a frozen copy is what keeps this lane's tests
reproducible (the same pattern `testdata/kazi/` uses in E4-L1).

Copied 2026-08-14T20:46:15Z with:

```
cp .dira/entries/*.md internal/status/testdata/ledgers/real-snapshot/.dira/entries/
```

Carries genuine `blocks` edges: `qst-0005 -> dec-0008` (open question) and
`qst-0003 -> int-0003` (now `answered`). It also carries genuine `rejected` and
`superseded` decisions (`dec-0003`, `dec-0015`, `dec-0021`, `dec-0025`), used by
`ledger_test.go` to assert they never appear in `ToBePlanned`.

**Verified absent from this snapshot: any `realized_by` edge.** Checked by
`grep -rl "type: realized_by" .dira/entries/*.md` against the live ledger at
copy time — zero matches, across all 43 entries. E4-L2.md's T3 task text
expected at least one ("this project's own `.dira/entries/dec-0060`-style
edges"), but no such id or edge exists in this repository's real ledger; there
is no `dec-0060` here at all. `ledger_test.go` records this and proves the
`realized_by`-exclusion behaviour instead against `no-realized-by/`, mutated
in memory to add the edge the real snapshot does not have — see that file's
`TestLedgerRealizedByEdgeExcludesEntry` and the lane status report.

## answered-question/

`qst-1001` (`state: answered`) carries a `blocks -> dec-1001` edge. Proves an
*answered* question's `blocks` edge must not produce a `DecisionBlocked` row.

## superseded-target/

`qst-1002` (`state: open`) carries a `blocks -> dec-1002` edge, but `dec-1002`
itself is `state: superseded`. Proves a live blocking question is not enough —
the blocked entry's own state must also be live.

## achieved-intent/

One intent, `int-1001`, `state: achieved`. Proves `Terminal` reports it and
`ToBePlanned` does not.

## abandoned-intent/

One intent, `int-1002`, `state: abandoned`. Proves `Terminal` reports it and
`ToBePlanned` does not.

## no-realized-by/

One accepted decision (`dec-1003`) and one active intent (`int-1003`), neither
carrying a `realized_by` edge. Proves the baseline positive case for
`ToBePlanned`, and is the fixture `ledger_test.go` mutates (adding a
`realized_by` edge to a copy) to prove the exclusion both ways.
