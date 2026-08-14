// Package chain resolves a ledger's full parent chain, transitively and
// read-only.
//
// dec-0006's own diagram is two hops deep (kazi/.dira -> derives_from:
// sire:int-... -> derives_from: me:int-...), and a repository does not, in the
// common case, declare its grandparent directly — sire declares me, not kazi.
// Walk recurses through every ledger's own [parents] declarations so a caller
// gets one Ancestor per namespace discovered at any depth, not just the ones a
// ledger names directly.
//
// # Read-only at every hop
//
// cst-0003 rule 1 is absolute: dira has no verb that writes upward. Every
// Ancestor.Store this package hands back is wrapped in ledger.ReadOnly before
// it is returned, at every depth, not only the leaf — see writesafety_test.go
// for the adversarial proof.
//
// # No path arithmetic here
//
// Only internal/ledger/local may know what a path is (dec-0005). This package
// calls local.ParentDira to join a [parents] declaration onto the ledger that
// declared it, local.Open to open the result, and local.ReadConfig to read its
// config.toml; it never imports os or path/filepath itself, and
// internal/ledger/boundary_test.go's filesystemPackages check runs over every
// package in the module and would fail this one the moment it did.
package chain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kazi-org/dira/internal/config"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// An Ancestor is one parent ledger discovered while walking a chain of
// [parents] declarations, at whatever depth declared it.
type Ancestor struct {
	// Namespace is the key this ancestor was declared under, at the hop
	// that declared it — the word a ref like `me:int-0002` is qualified
	// with from wherever it is cited.
	Namespace string

	// Tier is the ancestor's own [ledger].tier, read from its config.toml.
	// Empty when the ancestor could not be opened or declared none.
	Tier string

	// Store is the read path into this ancestor, always wrapped in
	// ledger.ReadOnly. Nil when the ancestor could not be opened or read —
	// see Err.
	Store ledger.Store

	// Err is why Store is nil: the declared path does not exist, is
	// unreadable, or names a directory that is not a ledger. Nil when
	// Store is set.
	//
	// A namespace with a non-nil Err is still "declared" for a caller
	// deciding oriented vs. withheld vs. undeclared: the namespace is
	// known, it is only unreachable from here.
	Err error
}

// walkFunc is the indirection Resolve calls through, so a test can prove it
// never runs Walk against a malformed ref by substituting a counting wrapper.
// It is unexported: only this package's own tests may reassign it.
var walkFunc = Walk

// Walk opens every ancestor diraDir's [parents] declarations name, directly or
// transitively, and returns one Ancestor per namespace discovered at any
// depth.
//
// A hop that cannot be opened degrades that ancestor — Store nil, Err set —
// and its own further ancestors are unknowable, not absent, so Walk does not
// recurse past it. That is not the same as failing the call: an unreachable
// ancestor is reported like any other, and Walk's error return is reserved for
// a structural problem in the declarations themselves — a namespace that would
// revisit a .dira directory already in the chain, refused rather than walked
// forever.
//
// An empty [parents] section, at any depth, contributes nothing and is not an
// error: the base case of "nothing to do" is zero ancestors and a nil error,
// not a nil slice standing in for a walk that never ran.
func Walk(ctx context.Context, diraDir string) ([]Ancestor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(diraDir) == "" {
		return nil, errors.New("chain: walk needs a .dira directory to start from")
	}

	data, err := local.ReadConfig(diraDir)
	if err != nil {
		return nil, fmt.Errorf("chain: reading %s: %w", diraDir, err)
	}
	cfg, _ := config.Parse(data)

	return walkParents(ctx, diraDir, cfg.ParentDecls, map[string]bool{diraDir: true})
}

// walkParents is Walk's recursive step: one ledger's own [parents]
// declarations, resolved one hop at a time.
//
// visited is the set of .dira directories already on the path from the root to
// ownDira, inclusive. It is copied rather than shared before each recursive
// call, so a cycle is caught against the path that led here and not against
// every hop discovered anywhere else in the tree — which would refuse a
// perfectly acyclic diamond (two children sharing one grandparent) for no
// reason.
func walkParents(ctx context.Context, ownDira string, decls []config.Parent, visited map[string]bool) ([]Ancestor, error) {
	var out []Ancestor
	for _, decl := range decls {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ns := strings.TrimSpace(decl.Name)
		if decl.Path == "" {
			// Declared but not locatable — exactly the shape a private
			// parent takes in a public clone. It is reported, not
			// walked.
			out = append(out, Ancestor{Namespace: ns, Err: errors.New("chain: no path is declared for this parent, so it cannot be located")})
			continue
		}

		parentDira := local.ParentDira(ownDira, decl.Path)
		if visited[parentDira] {
			return nil, fmt.Errorf("chain: cycle detected — %s (declared under %s) is already in this chain", parentDira, ownDira)
		}

		backend, openErr := local.Open(parentDira)
		if openErr != nil {
			out = append(out, Ancestor{Namespace: ns, Err: openErr})
			continue
		}
		cfgData, readErr := local.ReadConfig(parentDira)
		if readErr != nil {
			// The directory opened (os.Stat succeeded) but its contents
			// could not be read — a distinct failure from "absent",
			// produced by a different code path (chmod 000 on the
			// directory itself, caught here rather than by local.Open).
			out = append(out, Ancestor{Namespace: ns, Err: readErr})
			continue
		}
		parentCfg, _ := config.Parse(cfgData)

		out = append(out, Ancestor{Namespace: ns, Tier: parentCfg.Tier, Store: ledger.ReadOnly(backend)})

		childVisited := make(map[string]bool, len(visited)+1)
		for k := range visited {
			childVisited[k] = true
		}
		childVisited[parentDira] = true

		grand, err := walkParents(ctx, parentDira, parentCfg.ParentDecls, childVisited)
		if err != nil {
			return nil, err
		}
		out = append(out, grand...)
	}
	return out, nil
}
