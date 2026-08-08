package local_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/ledgertest"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// TestStoreContract runs the shared ledger.Store suite against the filesystem
// backend. When E7 adds the github backend it adds this same call and nothing
// else, which is the whole point of dec-0005's interface.
func TestStoreContract(t *testing.T) {
	t.Parallel()

	ledgertest.RunStoreContract(t, func(t *testing.T) ledger.Store {
		s, err := local.Open(t.TempDir())
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		return s
	})
}

func newStore(t *testing.T) (*local.Store, string) {
	t.Helper()

	dir := t.TempDir()
	s, err := local.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s, dir
}

// TestOpenRefusesWhatIsNotALedger covers the failure a user actually hits:
// running dira somewhere there is no .dira. Open must say so rather than create
// a second ledger in the wrong place.
func TestOpenRefusesWhatIsNotALedger(t *testing.T) {
	t.Parallel()

	t.Run("missing directory", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope", ".dira")
		if _, err := local.Open(missing); err == nil {
			t.Fatal("Open succeeded on a directory that does not exist")
		}
	})

	t.Run("a file, not a directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".dira")
		if err := os.WriteFile(path, []byte("not a ledger"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
		if _, err := local.Open(path); err == nil {
			t.Fatal("Open succeeded on a regular file")
		}
	})

	t.Run("creates nothing", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := local.Open(dir); err != nil {
			t.Fatalf("Open: %v", err)
		}
		names, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("reading %s: %v", dir, err)
		}
		if len(names) != 0 {
			t.Errorf("Open created %v; it must create nothing", names)
		}
	})
}

// TestWritesLandInOneFilePerEntry is dec-0002 checked rather than assumed: any
// mutation must be a single-file change, because that is what makes it a single
// GitHub PUT later and what keeps concurrent capture conflict-free.
func TestWritesLandInOneFilePerEntry(t *testing.T) {
	t.Parallel()

	s, dir := newStore(t)
	ctx := context.Background()

	if err := s.Create(ctx, ledgertest.Entry("dec-0001")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	before := snapshot(t, dir)

	entry, err := s.Get(ctx, "dec-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	entry.Edges = append(entry.Edges, ledger.Edge{Type: ledger.EdgeDerivesFrom, To: "int-0001"})
	if err := s.Put(ctx, entry); err != nil {
		t.Fatalf("Put: %v", err)
	}
	after := snapshot(t, dir)

	var changed []string
	for name, content := range after {
		if before[name] != content {
			changed = append(changed, name)
		}
	}
	for name := range before {
		if _, ok := after[name]; !ok {
			changed = append(changed, name+" (removed)")
		}
	}
	if len(changed) != 1 || changed[0] != filepath.Join("entries", "dec-0001.md") {
		t.Errorf("adding an edge changed %v, want exactly entries/dec-0001.md", changed)
	}
}

// TestNoTemporaryFilesSurvive guards the write path's own litter. A temp file
// left in .dira/entries would be committed to the repository.
func TestNoTemporaryFilesSurvive(t *testing.T) {
	t.Parallel()

	s, dir := newStore(t)
	ctx := context.Background()

	if err := s.Create(ctx, ledgertest.Entry("dec-0001")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Put(ctx, ledgertest.Entry("dec-0001")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// A Create that loses the race still writes a temp file first.
	if err := s.Create(ctx, ledgertest.Entry("dec-0001")); !errors.Is(err, ledger.ErrExists) {
		t.Fatalf("Create on a taken id: %v", err)
	}
	// So does one that is rejected for being invalid.
	bad := ledgertest.Entry("dec-0002")
	bad.Alternatives = nil
	if err := s.Create(ctx, bad); err == nil {
		t.Fatal("an invalid entry was written")
	}

	for name := range snapshot(t, dir) {
		if !strings.HasSuffix(name, ".md") {
			t.Errorf("%s survived a write; only entry files belong in the ledger", name)
		}
	}
}

// TestConcurrentCreateHasExactlyOneWinner is the property E1-L2's id allocator
// is built on: `dira log` runs unattended from Stop hooks in parallel sessions,
// so two of them will race for the same id, and the loser has to find out.
func TestConcurrentCreateHasExactlyOneWinner(t *testing.T) {
	t.Parallel()

	s, _ := newStore(t)
	ctx := context.Background()

	const racers = 16
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners int
		other   []error
	)
	start := make(chan struct{})

	for i := range racers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			entry := ledgertest.Entry("dec-0001")
			entry.Title = "Racer number " + string(rune('a'+i))
			<-start
			err := s.Create(ctx, entry)

			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				winners++
			case errors.Is(err, ledger.ErrExists):
			default:
				other = append(other, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if len(other) > 0 {
		t.Fatalf("Create failed for a reason other than the id being taken: %v", other)
	}
	if winners != 1 {
		t.Errorf("%d of %d concurrent Creates succeeded, want exactly 1", winners, racers)
	}

	// And the survivor is a complete, readable entry rather than a blend.
	got, err := s.Get(ctx, "dec-0001")
	if err != nil {
		t.Fatalf("Get after the race: %v", err)
	}
	if !strings.HasPrefix(got.Title, "Racer number ") {
		t.Errorf("title = %q, want one racer's title intact", got.Title)
	}
}

// TestListIgnoresWhatIsNotAnEntry covers the ledger being a directory in a
// repository that people also put files in.
func TestListIgnoresWhatIsNotAnEntry(t *testing.T) {
	t.Parallel()

	s, dir := newStore(t)
	ctx := context.Background()
	if err := s.Create(ctx, ledgertest.Entry("dec-0001")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	entries := filepath.Join(dir, "entries")
	for _, name := range []string{"README.md", "notes.txt", "dec-1.md", "DEC-0002.md", ".hidden.md"} {
		if err := os.WriteFile(filepath.Join(entries, name), []byte("---\nnot an entry\n"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(entries, "subdir.md"), 0o755); err != nil {
		t.Fatalf("creating subdir.md: %v", err)
	}

	got, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != "dec-0001" {
		t.Errorf("List = %+v, want only dec-0001", got)
	}
}

// TestGetRejectsAMisnamedFile covers the case where a human renamed a file. The
// id in the frontmatter and the file it lives in must agree, or `dira why
// dec-0002` resolves to whatever happens to be in dec-0002.md.
func TestGetRejectsAMisnamedFile(t *testing.T) {
	t.Parallel()

	s, dir := newStore(t)
	ctx := context.Background()
	if err := s.Create(ctx, ledgertest.Entry("dec-0001")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	entries := filepath.Join(dir, "entries")
	if err := os.Rename(filepath.Join(entries, "dec-0001.md"), filepath.Join(entries, "dec-0002.md")); err != nil {
		t.Fatalf("renaming: %v", err)
	}

	if _, err := s.Get(ctx, "dec-0002"); err == nil {
		t.Fatal("Get returned an entry whose id does not match its file name")
	}
}

// TestReadsTheRepositoryLedger points the backend at dira's own .dira and reads
// all of it. It is the only test here that runs against entries a human wrote.
func TestReadsTheRepositoryLedger(t *testing.T) {
	t.Parallel()

	s, err := local.Open("../../../.dira")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ctx := context.Background()

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) < 20 {
		t.Fatalf("List returned %d entries; dira's own ledger had 26 at E1", len(list))
	}

	for _, info := range list {
		entry, err := s.Get(ctx, info.ID)
		if err != nil {
			t.Errorf("Get %s: %v", info.ID, err)
			continue
		}
		if entry.Version() != info.Version {
			t.Errorf("%s: Get reports version %q, List reports %q", info.ID, entry.Version(), info.Version)
		}
	}
}

// snapshot reads every file under dir, keyed by path relative to dir.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()

	out := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out[rel] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return out
}
