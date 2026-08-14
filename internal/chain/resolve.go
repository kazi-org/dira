package chain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kazi-org/dira/internal/ledger"
)

// State is what Resolve found for a namespaced ref, one of the three answers
// dec-0011 fixes: oriented (the target resolved and is readable), withheld
// (the namespace is declared somewhere in the chain but the ledger holding it
// could not be opened here), or — reported as a named error rather than a
// State value at all — undeclared, so a typo can never be mistaken for a
// privacy boundary.
type State string

const (
	// Oriented is a namespace found and readable: Resolve also returns the
	// entry.
	Oriented State = "oriented"

	// Withheld is a namespace declared somewhere in the walked chain whose
	// ledger could not be opened from here. Withheld is success, not
	// failure — dec-0018: "withheld reads as neither an error nor a
	// warning" — so Resolve returns it with a nil error.
	Withheld State = "withheld"
)

// Resolve looks up one namespaced ref — `me:int-0002` — through diraDir's full
// parent chain (chain.Walk) and reports one of dec-0011's three outcomes.
//
// It takes a ref already known to be well-formed by ledger.ValidRef; a
// malformed ref is the caller's problem and is rejected before Walk ever runs,
// so a caller that never gets past its own flag parsing pays nothing for a
// walk it will not use.
//
// Resolve returns data; what a caller does with it — a citation line in
// `dira why`, a drift flag, a chain block in a brief — is each consumer's own
// policy. This package never truncates or redacts what it returns.
func Resolve(ctx context.Context, diraDir, ref string) (State, *ledger.Entry, error) {
	if !ledger.ValidRef(ref) {
		return "", nil, fmt.Errorf("chain: %q is not a well-formed entry ref", ref)
	}

	namespace, id := splitRef(ref)

	ancestors, err := walkFunc(ctx, diraDir)
	if err != nil {
		return "", nil, err
	}

	declared := false
	for _, a := range ancestors {
		if a.Namespace != namespace {
			continue
		}
		declared = true
		if a.Store == nil {
			// This ancestor is unreachable, but a later one in the
			// slice might carry the same namespace name from a
			// different hop; keep looking before giving up on it.
			continue
		}
		entry, err := a.Store.Get(ctx, id)
		if err != nil {
			return "", nil, fmt.Errorf("chain: %s: %w", ref, err)
		}
		return Oriented, entry, nil
	}

	if declared {
		return Withheld, nil, nil
	}
	return "", nil, fmt.Errorf("%w: %q in %q from %s — a typo or an invention, never treated as withheld",
		ErrUndeclaredNamespace, namespace, ref, diraDir)
}

// ErrUndeclaredNamespace is what Resolve wraps when a ref's namespace is not
// declared anywhere in the walked chain — dec-0011's "a typo or an
// invention," told apart with errors.Is from every other reason Resolve can
// fail to return an entry (a declared parent's ledger open but one entry
// inside it unreadable, say), which a caller like internal/drift must not
// fold into the same bucket as a mistyped ref.
var ErrUndeclaredNamespace = errors.New("chain: namespace not declared anywhere in the parent chain")

// splitRef splits a namespaced ref into its namespace and bare id. A ref with
// no namespace splits to an empty namespace, which cannot match anything
// chain.Walk discovers and therefore resolves as undeclared — chain.Resolve is
// for namespaced refs; a bare local id is the caller's own ledger's business,
// not this package's.
func splitRef(ref string) (namespace, id string) {
	ns, rest, ok := strings.Cut(ref, ":")
	if !ok {
		return "", ref
	}
	return ns, rest
}
