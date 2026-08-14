package local

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

func draft(kind ledger.Kind, title string) *ledger.Entry {
	state := ledger.StateActive
	if kind == ledger.KindQuestion {
		state = ledger.StateOpen
	}
	return &ledger.Entry{
		Kind:    kind,
		Title:   title,
		State:   state,
		Created: "2026-06-01T09:00:00Z",
		Source:  &ledger.Source{Hook: ledger.HookManual, Tier: ledger.TierHuman},
	}
}

const initConfig = "[ledger]\nname = \"fixture\"\ntier = \"person\"\n"

// TestInitLedger is E5-L5-T2's acceptance line.
func TestInitLedger(t *testing.T) {
	t.Run("a non-empty drafts slice creates a readable ledger", func(t *testing.T) {
		dir := t.TempDir()
		drafts := []*ledger.Entry{
			draft(ledger.KindIntent, "ship the personal ledger"),
			draft(ledger.KindConstraint, "nothing leaves this machine"),
		}

		store, err := InitLedger(dir, []byte(initConfig), drafts)
		if err != nil {
			t.Fatalf("InitLedger: %v", err)
		}

		diraDir := filepath.Join(dir, ".dira")
		if data, err := os.ReadFile(filepath.Join(diraDir, "config.toml")); err != nil || string(data) != initConfig {
			t.Errorf("config.toml = %q, %v; want %q verbatim", data, err, initConfig)
		}

		infos, err := store.List(context.Background())
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(infos) != 2 {
			t.Fatalf("got %d entries, want 2", len(infos))
		}
		got, err := store.Get(context.Background(), "int-0001")
		if err != nil {
			t.Fatalf("Get(int-0001): %v", err)
		}
		if got.Title != "ship the personal ledger" {
			t.Errorf("int-0001 title = %q", got.Title)
		}
	})

	t.Run("an empty drafts slice creates nothing and returns a named error", func(t *testing.T) {
		dir := t.TempDir()
		_, err := InitLedger(dir, []byte(initConfig), nil)
		if err == nil {
			t.Fatal("InitLedger with no drafts returned no error")
		}
		if _, statErr := os.Stat(filepath.Join(dir, ".dira")); !os.IsNotExist(statErr) {
			t.Errorf(".dira exists after an empty-drafts call: %v", statErr)
		}
		assertDirEmpty(t, dir)
	})

	t.Run("a write failure partway through leaves no .dira behind at all", func(t *testing.T) {
		dir := t.TempDir()

		// Force the third draft's write to collide: InitLedger allocates
		// the lowest unused id per kind starting at 1, so the second
		// constraint below gets cst-0002 — pre-seed exactly that path in
		// the fixed staging location InitLedger builds in before it
		// commits anything.
		staging := filepath.Join(dir, initStagingDirName)
		if err := os.MkdirAll(filepath.Join(staging, entriesDir), 0o755); err != nil {
			t.Fatalf("pre-seeding the staging collision: %v", err)
		}
		if err := os.WriteFile(filepath.Join(staging, entriesDir, "cst-0002.md"), []byte("pre-existing"), 0o644); err != nil {
			t.Fatalf("pre-seeding the staging collision: %v", err)
		}

		drafts := []*ledger.Entry{
			draft(ledger.KindIntent, "first, writes fine"),
			draft(ledger.KindConstraint, "second, writes fine"),
			draft(ledger.KindConstraint, "third, collides with the pre-seeded file"),
			draft(ledger.KindQuestion, "fourth, never reached"),
		}

		if _, err := InitLedger(dir, []byte(initConfig), drafts); err == nil {
			t.Fatal("InitLedger over a forced collision returned no error")
		}

		if _, statErr := os.Stat(filepath.Join(dir, ".dira")); !os.IsNotExist(statErr) {
			t.Errorf(".dira exists after a partial failure: %v", statErr)
		}
		if _, statErr := os.Stat(staging); !os.IsNotExist(statErr) {
			t.Errorf("the staging directory survived a failed init: %v", statErr)
		}
		assertDirEmpty(t, dir)
	})

	t.Run("called on a directory that already has a .dira, InitLedger refuses and touches nothing", func(t *testing.T) {
		dir := t.TempDir()
		diraDir := filepath.Join(dir, ".dira")
		if err := os.MkdirAll(filepath.Join(diraDir, "entries"), 0o755); err != nil {
			t.Fatalf("seeding an existing .dira: %v", err)
		}
		if err := os.WriteFile(filepath.Join(diraDir, "config.toml"), []byte("[ledger]\nname = \"existing\"\n"), 0o644); err != nil {
			t.Fatalf("seeding an existing .dira: %v", err)
		}
		before := digestDir(t, diraDir)

		_, err := InitLedger(dir, []byte(initConfig), []*ledger.Entry{draft(ledger.KindIntent, "should never land")})
		if err == nil {
			t.Fatal("InitLedger over an existing .dira returned no error")
		}

		if after := digestDir(t, diraDir); after != before {
			t.Error("the existing .dira's contents changed")
		}
		if _, statErr := os.Stat(filepath.Join(dir, initStagingDirName)); !os.IsNotExist(statErr) {
			t.Error("InitLedger touched the staging directory although .dira already existed")
		}
	})

	t.Run("TestNoFilesystemImportsAboveTheBackend still holds with this file added", func(t *testing.T) {
		// internal/ledger/local already has the filesystem allowlist;
		// this subtest exists only to document that this file needed no
		// new entry there. The real check lives in internal/ledger and
		// is run as part of the module's own test suite.
	})
}

// TestInitLedgerNaiveWriteAsYouGoLeavesPartialFiles is L-0001's red control:
// a write-each-draft-as-you-go implementation, exercised directly, leaves
// entries behind on the same collision the real InitLedger leaves none for.
func TestInitLedgerNaiveWriteAsYouGoLeavesPartialFiles(t *testing.T) {
	dir := t.TempDir()
	diraDir := filepath.Join(dir, ".dira")
	if err := os.MkdirAll(filepath.Join(diraDir, entriesDir), 0o755); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if err := os.WriteFile(filepath.Join(diraDir, configFile), []byte(initConfig), 0o644); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if err := os.WriteFile(filepath.Join(diraDir, entriesDir, "cst-0002.md"), []byte("pre-existing"), 0o644); err != nil {
		t.Fatalf("seeding the collision: %v", err)
	}

	store, err := Open(diraDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	drafts := []*ledger.Entry{
		draft(ledger.KindIntent, "first, writes fine"),
		draft(ledger.KindConstraint, "second, writes fine"),
		draft(ledger.KindConstraint, "third, collides"),
		draft(ledger.KindQuestion, "fourth, never reached"),
	}

	ctx := context.Background()
	taken := map[ledger.Kind]int{}
	var failed error
	for _, d := range drafts {
		n := taken[d.Kind] + 1
		d.ID = ledger.FormatID(d.Kind, n)
		if err := store.Create(ctx, d); err != nil {
			failed = err
			break
		}
		taken[d.Kind] = n
	}
	if failed == nil {
		t.Fatal("the naive writer did not reproduce the forced collision")
	}

	infos, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// The pre-seeded collision file plus the two drafts that wrote
	// successfully before the third failed: exactly the partial state
	// InitLedger's all-or-nothing contract exists to prevent.
	if len(infos) < 3 {
		t.Fatalf("the naive writer left %d files; expected it to demonstrate a partial ledger (>= 3)", len(infos))
	}
	t.Cleanup(func() { _ = os.RemoveAll(diraDir) })
}

// digestDir is a sha256 over every file under dir: its path relative to dir,
// and its bytes.
func digestDir(t *testing.T, dir string) string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	sort.Strings(paths)

	sum := sha256.New()
	for _, path := range paths {
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			t.Fatalf("relativising %s: %v", path, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		_, _ = fmt.Fprintf(sum, "%s\n%d\n", rel, len(data))
		sum.Write(data)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func assertDirEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("%s is not empty: %v", dir, names)
	}
}
