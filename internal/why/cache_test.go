package why_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/index/indextest"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/fixture"
	"github.com/kazi-org/dira/internal/ledger/local"
	"github.com/kazi-org/dira/internal/why"
)

// errNoChains stops the differential harness passing on two empty renders.
var errNoChains = errors.New("no chains rendered; the comparison would be empty against empty")

// fixtureLedger writes the shared 200-entry fixture into a temporary ledger and
// returns the directory holding .dira.
func fixtureLedger(t *testing.T, n int) string {
	t.Helper()

	diraDir := filepath.Join(t.TempDir(), ".dira")
	if err := os.MkdirAll(filepath.Join(diraDir, "entries"), 0o755); err != nil {
		t.Fatalf("creating %s: %v", diraDir, err)
	}
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("local.Open: %v", err)
	}
	entries, err := fixture.Generate(fixture.Seed, n)
	if err != nil {
		t.Fatalf("fixture.Generate: %v", err)
	}
	if err := fixture.Write(context.Background(), store, entries); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	return diraDir
}

// TestTheChainIsTheSameWithAndWithoutACache adds `dira why`'s real rendered
// output to E1-L3's differential harness, which is the thing that package was
// left extensible for.
//
// The guarantee it buys is dec-0002's: deleting .dira/cache/ changes how long an
// answer takes and nothing else about it. It is worth having here specifically
// because `why` is the command that renders the most from the files — every
// why_not, every revisit_if, every parent's title — so it is the command a cache
// that had quietly become authoritative would lie through.
func TestTheChainIsTheSameWithAndWithoutACache(t *testing.T) {
	t.Parallel()

	diraDir := fixtureLedger(t, 200)

	// Chains over the twelve newest decisions, all rendered in one query so
	// the comparison is over a page of real output rather than one entry.
	chains := indextest.Query{
		Name: "why chains, rendered",
		Run: func(ctx context.Context, ix *index.Index) (string, error) {
			refs, err := ix.Select(ctx, index.Selector{
				Kinds: []ledger.Kind{ledger.KindDecision},
				Limit: 12,
			})
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for _, ref := range refs {
				chain, err := why.Build(ctx, ix, ref.ID, ref.ID)
				if err != nil {
					return "", err
				}
				if err := why.RenderText(&b, chain, why.DefaultWidth); err != nil {
					return "", err
				}
				b.WriteString("\n")
			}
			if b.Len() == 0 {
				return "", errNoChains
			}
			return b.String(), nil
		},
	}

	byTerm := indextest.Query{
		Name: "why by term, rendered",
		Run: func(ctx context.Context, ix *index.Index) (string, error) {
			nodes, err := why.Resolve(ctx, ix, "latency")
			if err != nil {
				return "", err
			}
			var b strings.Builder
			if err := why.RenderCandidates(&b, "latency", nodes, why.DefaultWidth); err != nil {
				return "", err
			}
			return b.String(), nil
		},
	}

	indextest.RunTwice(t, diraDir, chains, byTerm)
}
