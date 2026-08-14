package cli_test

// Shared fixture-building helpers for internal/cli's tests. Building a
// ledger programmatically (indextest.Materialise + ledgertest.Entry) rather
// than hand-authoring markdown files keeps each test's premise legible next
// to its assertions, matching the pattern internal/status/join_test.go uses
// for its own synthetic fixtures. Test files may import
// internal/ledger/local for exactly this reason — `go list -deps
// ./internal/cli` (the boundary E4-L5-T5 checks) reports only the plain
// package's own imports, never a _test.go file's.

import (
	"context"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/index/indextest"
	"github.com/kazi-org/dira/internal/kazi"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/ledgertest"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// entry builds one fixture entry off ledgertest's shared shape (which
// already satisfies Entry.Validate() — alternatives for a non-staged
// decision, a valid title, a quoted timestamp) and applies mutate.
func entry(id string, mutate func(e *ledger.Entry)) *ledger.Entry {
	e := ledgertest.Entry(id)
	if mutate != nil {
		mutate(e)
	}
	return e
}

// realizedBy returns an edge mutator adding one realized_by edge.
func realizedBy(target string) func(*ledger.Entry) {
	return func(e *ledger.Entry) {
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeRealizedBy, To: target})
	}
}

// derivesFrom returns an edge mutator adding one derives_from edge.
func derivesFrom(target string) func(*ledger.Entry) {
	return func(e *ledger.Entry) {
		e.Edges = append(e.Edges, ledger.Edge{Type: ledger.EdgeDerivesFrom, To: target})
	}
}

// combine chains several entry mutators into one.
func combine(fns ...func(*ledger.Entry)) func(*ledger.Entry) {
	return func(e *ledger.Entry) {
		for _, fn := range fns {
			fn(e)
		}
	}
}

// openTree materialises entries into a fresh ledger and opens an *index.Index
// over it.
func openTree(t *testing.T, entries []*ledger.Entry) *index.Index {
	t.Helper()
	diraDir := indextest.Materialise(t, entries)
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("opening %s: %v", diraDir, err)
	}
	ix, err := index.Open(context.Background(), store, local.CacheDir(diraDir))
	if err != nil {
		t.Fatalf("opening index over %s: %v", diraDir, err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	return ix
}

// singleRunPortfolio builds a *kazi.Portfolio with one by_repo run per
// goalID -> bucket/status pair given, enough for most tests here that need
// a snapshot but not portfolio's full envelope.
func singleRunPortfolio(runs map[string]kazi.RepoBucket) *kazi.Portfolio {
	byRepo := map[string]map[kazi.RepoBucket][]kazi.RepoRun{}
	i := 0
	for goalID, bucket := range runs {
		i++
		status := "running"
		switch bucket {
		case kazi.RepoComplete:
			status = "converged"
		case kazi.RepoStuck:
			status = "stuck"
		}
		byRepo[repoName(i)] = map[kazi.RepoBucket][]kazi.RepoRun{
			bucket: {{GoalRef: goalID, RunID: "run-" + goalID, Status: status, Bucket: bucket}},
		}
	}
	return &kazi.Portfolio{ByRepo: byRepo}
}

func repoName(i int) string {
	return "repo-" + string(rune('a'+i))
}

// neverCalledStatusFn fails the test if it is ever invoked.
func neverCalledStatusFn(t *testing.T) func(context.Context, string) (*kazi.RunStatus, *kazi.ProposalStatus, error) {
	return func(context.Context, string) (*kazi.RunStatus, *kazi.ProposalStatus, error) {
		t.Fatal("statusFn was called; this fixture is single-run per goal and should not need it")
		return nil, nil, nil
	}
}
