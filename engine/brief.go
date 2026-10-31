package engine

import (
	"context"
	"io"

	"github.com/kazi-org/dira/internal/brief"
	"github.com/kazi-org/dira/internal/chain"
	"github.com/kazi-org/dira/internal/index"
)

// Public aliases let consumers implement the existing source contract without
// importing Dira's internal packages. They do not change selection semantics.
type Selector = index.Selector
type EntryRef = index.Ref
type BriefSource = brief.Source
type BriefOptions = brief.Options
type BriefAncestor = chain.Ancestor
type BriefResult = brief.Result
type BriefSectionResult = brief.SectionResult

const DefaultBriefTokens = brief.DefaultMaxTokens

// Brief delegates to the CLI's producer and renderer, preserving token, notice
// and omission accounting. Source and any Options.ChainSource must already be
// authorized by the caller; the facade discovers no paths and opens no caches.
// Output may include private source text. It is not a remotely publishable DTO
// or an authorization grant: transport disclosure policy remains separate.
func Brief(ctx context.Context, out io.Writer, src BriefSource, opts BriefOptions) (*BriefResult, error) {
	return brief.Render(ctx, out, src, opts)
}

// WalkChain explicitly invokes the CLI's existing parent-ledger walker. Unlike
// Brief itself, this operation reads configuration and declared parent paths.
// The caller must authorize those reads before invoking it; a root selected by
// untrusted hook/repository input is not sufficient remote authority. It opens
// parents read-only and selects no new record custody, writer, or cache.
func WalkChain(ctx context.Context, diraDir string) ([]BriefAncestor, error) {
	return chain.Walk(ctx, diraDir)
}
