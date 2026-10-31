package engine

import (
	"context"
	"io"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/why"
)

type Backlink = index.Backlink
type WhySource = why.Source
type WhyNode = why.Node
type WhyChain = why.Chain
type WhyRelation = why.Relation
type WhyArtifact = why.Artifact
type WhyResolution = why.Resolution

const (
	WhyOriented     = why.Oriented
	WhyUnresolved   = why.Unresolved
	DefaultWhyWidth = why.DefaultWidth
)

// ResolveWhy preserves the CLI's candidate resolution without silently picking
// one match. Nodes/chains may contain private text, unlike redacted CheckResult
// citations. The caller must authorize sources and separately govern disclosure.
func ResolveWhy(ctx context.Context, src WhySource, query string) ([]WhyNode, error) {
	return why.Resolve(ctx, src, query)
}

// BuildWhy reuses the original chain producer, including cycles, unresolved
// references and supersession. It never fetches external artifact URLs or
// infers execution status from them.
func BuildWhy(ctx context.Context, src WhySource, query, id string) (*WhyChain, error) {
	return why.Build(ctx, src, query, id)
}

// RenderWhy preserves the CLI renderer. Rendering does not authorize publishing
// private source text.
func RenderWhy(out io.Writer, result *WhyChain, width int) error {
	return why.RenderText(out, result, width)
}

// RenderWhyCandidates preserves the CLI's disambiguation output.
func RenderWhyCandidates(out io.Writer, query string, nodes []WhyNode, width int) error {
	return why.RenderCandidates(out, query, nodes, width)
}
