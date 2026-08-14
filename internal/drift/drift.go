// Package drift classifies every active intent in a ledger as oriented,
// withheld, orphan or broken — dec-0006's orphan-work flag, extended by
// dec-0011 from two states to three (plus the one dec-0011's implementation
// notes insist on keeping distinct from all of them: a typo is never
// withheld).
//
// # Scope: active intents, and only active intents
//
// dec-0006: "an active intent with no derives_from into a parent is surfaced
// as drift." A decision, question, constraint or note carries no such flag —
// a rejected decision citing no precedent is not unexplained work — so
// Classify reads kind == intent && state == active and nothing else.
//
// # Read-time, never stored
//
// dec-0004: status is never stored. Classify is recomputed from the ledger
// and internal/chain on every call and writes nothing, the same discipline
// internal/enforcer and internal/brief already hold.
package drift

import (
	"context"
	"errors"
	"strings"

	"github.com/kazi-org/dira/internal/chain"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// State is what Classify found for one active intent.
type State string

const (
	// Oriented is an intent whose derives_from edge resolved to a readable
	// entry.
	Oriented State = "oriented"

	// Withheld is an intent whose derives_from edge names a namespace this
	// package knows about but cannot currently read from here — whether
	// because the whole parent ledger is unreachable (chain.Withheld) or
	// because the declared parent opened but the one entry the edge names
	// could not be read. Both are "we know this is legitimate and cannot
	// see it right now," which is what dec-0011's withheld means; neither
	// is a typo.
	Withheld State = "withheld"

	// Orphan is an intent with no derives_from edge at all — dec-0006's
	// drift: work with no stated ancestry. This is the only state that
	// counts as drift.
	Orphan State = "orphan"

	// Broken is an intent whose derives_from edge names a namespace
	// declared nowhere in the walked chain — dec-0011's "a typo or an
	// invention," told apart from Withheld precisely so a real mistake
	// cannot hide behind the privacy boundary.
	Broken State = "broken"
)

// A Classification is what Classify found for one active intent — its own
// state plus everything a renderer needs to say so without opening a ledger
// a second time.
type Classification struct {
	// State is the drift state.
	State State

	// Title is the intent's own title, always set.
	Title string

	// Namespace is the derives_from edge's namespace — the word before the
	// colon in `sire:int-0002` — set for Withheld and Broken so a renderer
	// can name what it cannot show. Empty for Orphan (no edge to name) and
	// for Oriented (the resolved entry's own title says enough).
	Namespace string

	// TargetTitle is the resolved parent entry's title, set only when
	// State == Oriented. A Withheld or Broken classification carries no
	// entry to render — chain.Resolve returned none for either — so there
	// is nothing here for a renderer to leak even if it tried.
	TargetTitle string
}

// Classify reads every active intent in the ledger at diraDir and reports
// each one's classification.
//
// The result's keys are exactly the active intents' ids: a decision,
// question, constraint or note is never a key here, whether or not it
// carries a derives_from edge of its own. Classify opens the ledger and its
// parent chain to compute this and writes nothing (dec-0004).
func Classify(ctx context.Context, diraDir string) (map[string]Classification, error) {
	store, err := local.Open(diraDir)
	if err != nil {
		return nil, err
	}

	infos, err := store.List(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[string]Classification)
	for _, info := range infos {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		prefix, _, ok := strings.Cut(info.ID, "-")
		if !ok {
			continue
		}
		kind, known := ledger.KindForPrefix(prefix)
		if !known || kind != ledger.KindIntent {
			continue
		}
		e, err := store.Get(ctx, info.ID)
		if err != nil {
			return nil, err
		}
		if e.Kind != ledger.KindIntent || e.State != ledger.StateActive {
			continue
		}
		out[e.ID] = classifyOne(ctx, diraDir, e)
	}
	return out, nil
}

// classifyOne is one active intent's classification: orphan if it carries no
// derives_from edge, and otherwise whatever chain.Resolve reports about the
// edge's target, translated into drift's vocabulary.
func classifyOne(ctx context.Context, diraDir string, e *ledger.Entry) Classification {
	c := Classification{Title: e.Title}

	namespace, id := splitRef(e)
	if namespace == "" && id == "" {
		c.State = Orphan
		return c
	}
	c.Namespace = namespace

	state, entry, err := chain.Resolve(ctx, diraDir, namespace+":"+id)
	if err != nil {
		if errors.Is(err, chain.ErrUndeclaredNamespace) {
			c.State = Broken
			return c
		}
		// The namespace is declared — chain.Resolve got past the
		// undeclared-namespace check — but something stopped this
		// particular read: a real, legitimate reference this package
		// simply cannot see right now. That is exactly dec-0011's
		// withheld, not a typo.
		c.State = Withheld
		return c
	}
	switch state {
	case chain.Oriented:
		c.State = Oriented
		if entry != nil {
			c.TargetTitle = entry.Title
		}
	case chain.Withheld:
		c.State = Withheld
	default:
		c.State = Withheld
	}
	return c
}

// splitRef reads e's derives_from edge, if it has one, into a namespace and a
// bare id. Both return values are empty for an intent with no such edge —
// the orphan case.
func splitRef(e *ledger.Entry) (namespace, id string) {
	ref := derivesFromRef(e)
	if ref == "" {
		return "", ""
	}
	ns, rest, ok := strings.Cut(ref, ":")
	if !ok {
		// A bare, unnamespaced derives_from target is not this lane's
		// scenario (dec-0006's diagram is always namespaced across a
		// tier boundary) but is not nonsense either: treat the whole
		// ref as the id under an empty namespace, which chain.Resolve
		// will in turn report as an undeclared namespace.
		return "", rest
	}
	return ns, rest
}

// derivesFromRef returns the target of e's derives_from edge, or "" if it has
// none. entry.schema.json allows at most the edges a caller wrote; dec-0006's
// drift flag looks at whether one of that type exists at all, not at how
// many.
func derivesFromRef(e *ledger.Entry) string {
	for _, edge := range e.Edges {
		if edge.Type == ledger.EdgeDerivesFrom {
			return edge.To
		}
	}
	return ""
}
