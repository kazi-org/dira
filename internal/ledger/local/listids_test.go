package local_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kazi-org/dira/internal/ledger/ledgertest"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// TestListIDsSeesTheSameEntriesAsListWithoutOpeningThem is the two-sided proof
// that ListIDs is a cheaper listing rather than a different one.
//
// The saving it claims is "no entry file is opened", and a timing test cannot
// assert that — a fast number is what every broken measurement returns. So the
// entry files are made unreadable and the two methods are run over them: List,
// which must open each file to hash it, fails; ListIDs, which must not, returns
// the same ids it returned when the files were readable. That is the same claim
// as a syscall count and it holds on any filesystem.
//
// Both sides, per docs/lore.md L-0001: the red side is List failing on the
// unreadable ledger (if it did not, the file permissions are not doing what this
// test assumes and the green side proves nothing), and the green side is ListIDs
// succeeding on that ledger AND agreeing with List on a readable one.
func TestListIDsSeesTheSameEntriesAsListWithoutOpeningThem(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which ignores the file mode this test uses to prove a file was not opened")
	}

	s, dir := newStore(t)
	ctx := context.Background()
	ids := []string{"dec-0001", "dec-0002", "int-0001"}
	for _, id := range ids {
		if err := s.Create(ctx, ledgertest.Entry(id)); err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
	}
	entries := filepath.Join(dir, "entries")

	// Not an entry, in the shapes TestListIgnoresWhatIsNotAnEntry uses: the
	// two listings must skip exactly the same files, or a cold build and a
	// warm reconcile would be indexing different ledgers.
	for _, name := range []string{"README.md", "notes.txt", "dec-1.md", "DEC-0009.md"} {
		if err := os.WriteFile(filepath.Join(entries, name), []byte("---\nnot an entry\n"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	// Green, part one: over a readable ledger the two agree on the id set,
	// and only on the id set — ListIDs reports no versions, deliberately.
	full, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	bare, err := s.ListIDs(ctx)
	if err != nil {
		t.Fatalf("ListIDs: %v", err)
	}
	if len(full) != len(ids) {
		t.Fatalf("List returned %d entries, want %d", len(full), len(ids))
	}
	if len(bare) != len(full) {
		t.Fatalf("ListIDs returned %d entries, List returned %d", len(bare), len(full))
	}
	for i := range full {
		if bare[i].ID != full[i].ID {
			t.Errorf("entry %d: ListIDs says %q, List says %q — the two listings disagree about what an entry is",
				i, bare[i].ID, full[i].ID)
		}
		if full[i].Version == "" {
			t.Errorf("List reported no version for %s, so this test cannot tell the two listings apart", full[i].ID)
		}
		if bare[i].Version != "" {
			t.Errorf("ListIDs reported version %q for %s; it must report none, because a caller that "+
				"believed it would be comparing against a hash of nothing", bare[i].Version, bare[i].ID)
		}
	}

	// Now take away read permission on every entry file. The directory stays
	// readable: this is "the names are there, the contents are not".
	for _, id := range ids {
		if err := os.Chmod(filepath.Join(entries, id+".md"), 0o000); err != nil {
			t.Fatalf("chmod %s: %v", id, err)
		}
	}
	t.Cleanup(func() {
		for _, id := range ids {
			_ = os.Chmod(filepath.Join(entries, id+".md"), 0o644)
		}
	})

	// Red: List opens every entry file, so it cannot survive this.
	if got, err := s.List(ctx); err == nil {
		t.Fatalf("List returned %d entries over a ledger whose files cannot be read, so this test is not "+
			"measuring what it claims and its green side proves nothing", len(got))
	}

	// Green, part two: ListIDs never opens one, so it does.
	bare, err = s.ListIDs(ctx)
	if err != nil {
		t.Fatalf("ListIDs failed over a ledger whose files cannot be read, so it opened one: %v", err)
	}
	if len(bare) != len(ids) {
		t.Fatalf("ListIDs returned %d entries, want %d", len(bare), len(ids))
	}
	for i, id := range ids {
		if bare[i].ID != id {
			t.Errorf("entry %d = %q, want %q", i, bare[i].ID, id)
		}
	}
}

// TestListIDsOverAnEmptyLedger covers the case List documents: a ledger with no
// entries directory is empty, not broken. The two listings must degrade the same
// way or a first run against a fresh `dira init` would fail on one path and not
// the other.
func TestListIDsOverAnEmptyLedger(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := local.Open(dir)
	if err != nil {
		t.Fatalf("local.Open: %v", err)
	}

	got, err := s.ListIDs(context.Background())
	if err != nil {
		t.Fatalf("ListIDs over a ledger with no entries directory: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListIDs = %+v, want empty", got)
	}
}
