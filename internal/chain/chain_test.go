package chain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/config"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// TestWalk is E5-L2-T1's acceptance line. Every subtest copies a committed
// fixture into a fresh t.TempDir() first (L-0014) and re-digests every
// fixture ledger afterwards to prove the read touched nothing.
func TestWalk(t *testing.T) {
	t.Run("three-tier fixture: sire then me, both read-only, me's tier read from its own config", func(t *testing.T) {
		root := copyFixture(t, "tier3")
		before := snapshot(t, root, []string{"me", "sire", "repo"})

		ancestors, err := Walk(context.Background(), filepath.Join(root, "repo", ".dira"))
		if err != nil {
			t.Fatalf("Walk: %v", err)
		}
		if len(ancestors) != 2 {
			t.Fatalf("got %d ancestors, want 2: %+v", len(ancestors), ancestors)
		}
		if ancestors[0].Namespace != "sire" {
			t.Errorf("ancestors[0].Namespace = %q, want sire", ancestors[0].Namespace)
		}
		if ancestors[0].Store == nil || ancestors[0].Err != nil {
			t.Errorf("sire: Store=%v Err=%v, want an open store and no error", ancestors[0].Store, ancestors[0].Err)
		}
		if ancestors[1].Namespace != "me" {
			t.Fatalf("ancestors[1].Namespace = %q, want me", ancestors[1].Namespace)
		}
		if ancestors[1].Store == nil {
			t.Fatal("me's Store is nil; the grandparent hop was not opened")
		}
		if ancestors[1].Tier != "person" {
			t.Errorf("me's Tier = %q, want person (read from me's own config.toml)", ancestors[1].Tier)
		}

		// me is reachable only because sire's own config names it: repo's
		// own [parents] declares sire alone.
		repoCfgData, err := local.ReadConfig(filepath.Join(root, "repo", ".dira"))
		if err != nil {
			t.Fatalf("reading repo's own config: %v", err)
		}
		repoCfg, _ := config.Parse(repoCfgData)
		if len(repoCfg.ParentDecls) != 1 || repoCfg.ParentDecls[0].Name != "sire" {
			t.Fatalf("the fixture's own repo config declares %v; this subtest is not testing transitive resolution", repoCfg.ParentDecls)
		}

		assertUnchanged(t, before, root)
	})

	t.Run("me absent entirely degrades that ancestor without failing the walk", func(t *testing.T) {
		root := copyFixture(t, "tier3")
		if err := os.RemoveAll(filepath.Join(root, "me")); err != nil {
			t.Fatalf("removing me/: %v", err)
		}
		before := snapshot(t, root, []string{"sire", "repo"})

		ancestors, err := Walk(context.Background(), filepath.Join(root, "repo", ".dira"))
		if err != nil {
			t.Fatalf("Walk returned an error for one unreachable ancestor; it must degrade that ancestor instead: %v", err)
		}
		if len(ancestors) != 2 {
			t.Fatalf("got %d ancestors, want 2 (sire usable, me degraded): %+v", len(ancestors), ancestors)
		}
		if ancestors[0].Namespace != "sire" || ancestors[0].Store == nil {
			t.Errorf("sire's ancestor is affected by me's absence: %+v", ancestors[0])
		}
		if ancestors[1].Namespace != "me" {
			t.Fatalf("ancestors[1].Namespace = %q, want me", ancestors[1].Namespace)
		}
		if ancestors[1].Store != nil {
			t.Error("me's Store is non-nil for a directory that does not exist")
		}
		if ancestors[1].Err == nil {
			t.Error("me's Err is nil for a directory that does not exist")
		}

		assertUnchanged(t, before, root)
	})

	t.Run("me present but chmod 000 degrades that ancestor the same way, via a different code path", func(t *testing.T) {
		root := copyFixture(t, "tier3")
		meDira := filepath.Join(root, "me", ".dira")
		chmodUnreadable(t, meDira)
		before := snapshot(t, root, []string{"sire", "repo"})

		ancestors, err := Walk(context.Background(), filepath.Join(root, "repo", ".dira"))
		if err != nil {
			t.Fatalf("Walk returned an error for one unreachable ancestor; it must degrade that ancestor instead: %v", err)
		}
		if len(ancestors) != 2 {
			t.Fatalf("got %d ancestors, want 2: %+v", len(ancestors), ancestors)
		}
		if ancestors[0].Namespace != "sire" || ancestors[0].Store == nil {
			t.Errorf("sire's ancestor is affected by me's permissions: %+v", ancestors[0])
		}
		if ancestors[1].Store != nil {
			t.Error("me's Store is non-nil for a directory dira cannot read")
		}
		if ancestors[1].Err == nil {
			t.Error("me's Err is nil for a directory dira cannot read")
		}

		assertUnchanged(t, before, root)
	})

	t.Run("a two-node cycle is refused, named, and does not hang", func(t *testing.T) {
		root := copyFixture(t, "cycle")
		before := snapshot(t, root, []string{"me", "sire"})

		_, err := Walk(context.Background(), filepath.Join(root, "me", ".dira"))
		if err == nil {
			t.Fatal("Walk over a two-node cycle returned no error")
		}
		if !strings.Contains(err.Error(), "me") || !strings.Contains(err.Error(), "sire") {
			t.Errorf("cycle error %q does not name both directories", err.Error())
		}

		assertUnchanged(t, before, root)
	})

	t.Run("an empty [parents] section is zero ancestors and no error", func(t *testing.T) {
		root := copyFixture(t, "empty")
		before := snapshot(t, root, []string{"leaf"})

		ancestors, err := Walk(context.Background(), filepath.Join(root, "leaf", ".dira"))
		if err != nil {
			t.Fatalf("Walk over an empty [parents] section returned an error: %v", err)
		}
		if len(ancestors) != 0 {
			t.Fatalf("got %d ancestors over an empty [parents] section, want 0: %+v", len(ancestors), ancestors)
		}

		assertUnchanged(t, before, root)
	})
}
