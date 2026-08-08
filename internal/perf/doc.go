//go:build perf

// Package perf measures what a hook actually pays: the shipped binary, spawned.
//
// # Why this package exists at all
//
// int-0002 budgets a hook invocation at well under 100ms. E1-L5 measured the
// read path and reported 17.3ms warm and 61.2ms cold over the 200-entry
// fixture, and internal/index/latency_test.go still measures that way. Both
// numbers are honest and neither is this one: they are taken *in process*, so
// everything between `execve` and `local.Open` is excluded — process creation,
// the dynamic loader, Go runtime start-up, package init including the embedded
// JSON Schema document, the config read, and the cache open. A hook pays all of
// it. Nothing in the repository measured it before this package.
//
// So the harness here builds `./cmd/dira` and runs it as a subprocess. Never
// `go run`, which would time the compiler. Never an in-process call, which is
// the measurement this lane exists because of.
//
// # Why it is behind a build tag
//
// `go test ./...` runs packages in parallel, and a timing test that competes
// with the rest of the suite for four cores and one disk reports the load
// average rather than the program. internal/ledger/fixture's
// TestFullLedgerReadIsWithinBudget has already failed once exactly that way —
// 254ms against a 150ms budget under full-suite load, green when run alone.
//
// The response to contention is to remove the contention, not to loosen the
// assertion. Hence `//go:build perf` on every file here: the package is invisible
// to `go test ./...` and is run deliberately, alone, with -p 1:
//
//	go test -tags perf -count=1 -p 1 ./internal/perf -v
//
// # Two statistics, and why both are reported
//
// internal/index/latency_test.go carries a minimum and a median deliberately,
// and the reasoning transfers here unchanged. Contention can only ever ADD time,
// so the smallest sample is the closest available estimate of what the code
// costs on an idle machine — that is the right input to a comparative claim.
// The median is what a run actually costs on the machine that took the sample,
// and since min <= median by construction, asserting a ceiling against the
// minimum fires strictly LESS often than against the median.
//
// This lane's budgets are absolute, so every assertion here reads the MEDIAN,
// which is the statistic int-0002 was written against. The minimum is reported
// beside it, labelled as the uncontended estimate, because a red run has to be
// diagnosable — "the median is 180ms and the minimum is 40ms" says the machine
// was busy, and "the median is 180ms and the minimum is 175ms" says the program
// is slow. Reporting the minimum is not asserting against it, and the two must
// not be unified.
//
// # What a green run here does not prove
//
// It proves the binary met the budget on the hardware that ran it, over the
// 200-entry fixture, with the page cache warm for the binary itself. It does not
// prove anything about a first run after a download, a cold filesystem cache, a
// ledger an order of magnitude larger, or a machine under different load. E1-L6-T4
// owns making the measurement happen on CI hardware, and E1-L6-T5 owns attributing
// the number to phases.
package perf
