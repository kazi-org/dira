package status_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// fixtureNames is the fixture corpus T2 records, in the order its README
// documents them.
var fixtureNames = []string{
	"real-snapshot",
	"answered-question",
	"superseded-target",
	"achieved-intent",
	"abandoned-intent",
	"no-realized-by",
}

// fixtureEntriesDir returns the on-disk entries/ directory for a committed
// fixture, without copying anything — the read-only path TestFixtureCorpus
// uses to check the corpus itself.
func fixtureEntriesDir(name string) string {
	return filepath.Join("testdata", "ledgers", name, ".dira", "entries")
}

// fixtureEntries decodes every entry file under a fixture, exactly as
// TestFixtureCorpus's acc line requires: parsed via the E1 entry codec, no
// exceptions.
func fixtureEntries(t *testing.T, name string) []*ledger.Entry {
	t.Helper()

	dir := fixtureEntriesDir(name)
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	var out []*ledger.Entry
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			t.Fatalf("reading %s/%s: %v", dir, f.Name(), err)
		}
		e, err := ledger.Decode(data)
		if err != nil {
			t.Fatalf("decoding %s/%s: %v", dir, f.Name(), err)
		}
		out = append(out, e)
	}
	return out
}

// openFixtureIndex copies a committed fixture ledger into a fresh temp
// directory and opens an *index.Index over the copy.
//
// It never opens the committed testdata/ledgers tree directly: T2 records
// those as a frozen, verbatim corpus, and a test that wrote a cache database
// (or, worse, a mutated entry) into it would be exactly the kind of
// accidental edit the fixtures exist to be safe from.
func openFixtureIndex(t *testing.T, name string) *index.Index {
	t.Helper()
	return openLedgerDir(t, copyFixture(t, name))
}

// copyFixture copies a committed fixture's .dira directory into a fresh
// t.TempDir() and returns the copy's .dira path, ready to mutate.
func copyFixture(t *testing.T, name string) string {
	t.Helper()

	src := filepath.Join("testdata", "ledgers", name, ".dira")
	dst := filepath.Join(t.TempDir(), ".dira")
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copying fixture %s: %v", name, err)
	}
	return dst
}

// openLedgerDir opens a Store and Index over an already-materialised .dira
// directory (from copyFixture, a mutated copy, or indextest.Materialise).
func openLedgerDir(t *testing.T, diraDir string) *index.Index {
	t.Helper()

	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("opening ledger at %s: %v", diraDir, err)
	}
	ix, err := index.Open(context.Background(), store, local.CacheDir(diraDir))
	if err != nil {
		t.Fatalf("opening index over %s: %v", diraDir, err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	return ix
}

// copyDir recursively copies src to dst, both directories.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	return err
}
