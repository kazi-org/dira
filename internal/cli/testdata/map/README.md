# testdata/map — E4-L4-T7's golden-file corpus

Three scenarios, matching the pattern `internal/status/testdata/ledgers/README.md`
and `internal/kazi/testdata/kazi/README.md` already use for this epic.

## real-snapshot/

A **verbatim, unedited copy** of this repository's own `.dira/entries/` — not
a live pointer at it, for the same reason `internal/status/testdata/ledgers/README.md`'s
`real-snapshot/` fixture gives: other worktrees' concurrent lanes mutate the
real ledger, and a frozen copy is what keeps this golden file reproducible.

Copied alongside this lane's authoring. Carries no `realized_by` edge (this
repository's own ledger has none — see
`internal/status/testdata/ledgers/README.md`'s identical finding), so this
scenario alone covers `ToBePlanned` and `DecisionBlocked` only; the other
four buckets are covered by the `unparented/` scenario below, combined per
the lane's own acc line ("all six ... appear across the golden set combined").

## unparented (built in Go, not committed as files)

`unparentedScenarioEntries` in `map_golden_test.go` builds a small synthetic
ledger covering `Completed`, `InProgress`, `ExecutionBlocked` and `Planned`
via a `singleRunPortfolio` fixture, plus one entry with no `derives_from`
parent (`dec-7006`) — the unparented-group scenario itself.

## kazi-unavailable (same entries, `snapErr` set)

Reuses `unparentedScenarioEntries`, run with `snapErr` set to
`*kazi.Unavailable{Reason: ReasonNotOnPath}` — proves the degradation line
renders and every ledger-side bucket still does, per E4-L3-T6/E4-L5.

Regenerate with `go test ./internal/cli -run TestMap -update` after any
intentional rendering change; a diff to a `.golden` file is a rendering
change and must be reviewed as one.
