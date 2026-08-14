package chain

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kazi-org/dira/internal/config"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// TestNoHopCanWrite is E5-L2-T3's acceptance line: the adversarial proof that
// no hop Walk opens can acquire a write path, checked directly on every
// Ancestor's Store rather than inferred from an absence of side effects.
func TestNoHopCanWrite(t *testing.T) {
	probe := func(id string) *ledger.Entry {
		return &ledger.Entry{
			ID:      id,
			Kind:    ledger.KindNote,
			Title:   "a write this test must never let land",
			State:   ledger.StateActive,
			Created: "2026-01-01T00:00:00Z",
		}
	}

	t.Run("every ancestor Walk returns refuses Create, Put and Delete", func(t *testing.T) {
		root := copyFixture(t, "tier3")
		before := snapshot(t, root, []string{"me", "sire"})

		ancestors, err := Walk(context.Background(), filepath.Join(root, "repo", ".dira"))
		if err != nil {
			t.Fatalf("Walk: %v", err)
		}
		if len(ancestors) != 2 {
			t.Fatalf("got %d ancestors, want 2", len(ancestors))
		}

		for _, a := range ancestors {
			if a.Store == nil {
				t.Fatalf("%s: Store is nil, nothing to probe", a.Namespace)
			}
			if err := a.Store.Create(context.Background(), probe("note-9001")); !errors.Is(err, ledger.ErrReadOnly) {
				t.Errorf("%s: Create = %v, want an error wrapping ledger.ErrReadOnly", a.Namespace, err)
			}
			if err := a.Store.Put(context.Background(), probe("note-9001")); !errors.Is(err, ledger.ErrReadOnly) {
				t.Errorf("%s: Put = %v, want an error wrapping ledger.ErrReadOnly", a.Namespace, err)
			}
			if err := a.Store.Delete(context.Background(), "note-9001"); !errors.Is(err, ledger.ErrReadOnly) {
				t.Errorf("%s: Delete = %v, want an error wrapping ledger.ErrReadOnly", a.Namespace, err)
			}
		}

		assertUnchanged(t, before, root)
	})

	t.Run("the same directory opened directly, bypassing Walk, writes for real", func(t *testing.T) {
		// The control: proves the refusal above is ledger.ReadOnly's
		// doing, not a filesystem permission the test environment happens
		// to enforce.
		root := copyFixture(t, "tier3")
		meDira := filepath.Join(root, "me", ".dira")

		backend, err := local.Open(meDira)
		if err != nil {
			t.Fatalf("local.Open: %v", err)
		}

		e := probe("note-9002")
		if err := backend.Create(context.Background(), e); err != nil {
			t.Fatalf("Create on a directly-opened store: %v", err)
		}
		written := filepath.Join(meDira, "entries", "note-9002.md")
		if _, err := os.Stat(written); err != nil {
			t.Fatalf("Create reported success but %s does not exist: %v", written, err)
		}

		if err := backend.Put(context.Background(), e); err != nil {
			t.Errorf("Put on a directly-opened store: %v", err)
		}
		if err := backend.Delete(context.Background(), "note-9002"); err != nil {
			t.Errorf("Delete on a directly-opened store: %v", err)
		}
	})

	t.Run("a walker missing the ReadOnly wrapper on the grandparent hop is caught by this check", func(t *testing.T) {
		// L-0001's construction: the check must be shown able to catch
		// the exact defect it exists to catch, not just able to pass the
		// real implementation. This is a deliberately broken copy of
		// walkParents' body with the wrapper dropped on the grandparent
		// (me) hop, exercised in place of the real Walk.
		root := copyFixture(t, "tier3")
		repoDira := filepath.Join(root, "repo", ".dira")

		ancestors := brokenWalkNoReadOnlyOnGrandparent(t, repoDira)
		var me *Ancestor
		for i := range ancestors {
			if ancestors[i].Namespace == "me" {
				me = &ancestors[i]
			}
		}
		if me == nil || me.Store == nil {
			t.Fatalf("the broken walker did not reach me: %+v", ancestors)
		}

		e := probe("note-9003")
		err := me.Store.Put(context.Background(), e)
		if err != nil {
			t.Fatalf("the broken walker's me.Store still refused a write (%v); this subtest is not testing anything", err)
		}
		written := filepath.Join(root, "me", ".dira", "entries", "note-9003.md")
		if _, statErr := os.Stat(written); statErr != nil {
			t.Fatalf("Put reported success but %s does not exist: %v", written, statErr)
		}
		t.Cleanup(func() { _ = os.Remove(written) })
	})
}

// brokenWalkNoReadOnlyOnGrandparent is a deliberately defective copy of
// walkParents: it wraps the direct parent in ledger.ReadOnly but hands back
// the grandparent's raw, writable backend. It exists only to prove
// TestNoHopCanWrite's first subtest would fail against this shape, and is
// never called from production code.
func brokenWalkNoReadOnlyOnGrandparent(t *testing.T, diraDir string) []Ancestor {
	t.Helper()

	data, err := local.ReadConfig(diraDir)
	if err != nil {
		t.Fatalf("reading %s: %v", diraDir, err)
	}
	cfg, _ := config.Parse(data)

	var out []Ancestor
	for _, decl := range cfg.ParentDecls {
		parentDira := local.ParentDira(diraDir, decl.Path)
		backend, err := local.Open(parentDira)
		if err != nil {
			out = append(out, Ancestor{Namespace: decl.Name, Err: err})
			continue
		}
		// Direct parent: wrapped, as the real Walk does.
		out = append(out, Ancestor{Namespace: decl.Name, Store: ledger.ReadOnly(backend)})

		grandData, err := local.ReadConfig(parentDira)
		if err != nil {
			continue
		}
		grandCfg, _ := config.Parse(grandData)
		for _, grandDecl := range grandCfg.ParentDecls {
			grandDira := local.ParentDira(parentDira, grandDecl.Path)
			grandBackend, err := local.Open(grandDira)
			if err != nil {
				out = append(out, Ancestor{Namespace: grandDecl.Name, Err: err})
				continue
			}
			// The defect: the grandparent hop is handed back raw.
			out = append(out, Ancestor{Namespace: grandDecl.Name, Store: grandBackend})
		}
	}
	return out
}
