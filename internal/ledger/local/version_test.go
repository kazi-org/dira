package local_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// dec-0015 makes EntryInfo.Version the entry file's git blob object id, and
// rests two claims on that choice: that a human can reproduce dira's version
// with `git hash-object` and no dira, and that E7's github backend — which gets
// blob shas free from the Contents API — will produce the same value for the
// same content, so a ledger can change backend without a full reindex.
//
// Both are claims about an external tool's output, so they are checked against
// that tool rather than restated.

func TestVersionIsTheGitBlobId(t *testing.T) {
	t.Parallel()

	git, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("no git on PATH: %v", err)
	}

	diraDir := filepath.Join(t.TempDir(), ".dira")
	entriesDir := filepath.Join(diraDir, "entries")
	if err := os.MkdirAll(entriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatal(err)
	}

	entries := []*ledger.Entry{
		{
			ID: "dec-0001", Kind: ledger.KindDecision, State: ledger.StateAccepted,
			Title: "A decision with an alternative", Created: "2026-07-30T09:00:00Z",
			Alternatives: []ledger.Alternative{{Option: "Do nothing", WhyNot: "the problem does not go away"}},
			Body:         "\nA body with a blank line follows.\n\nAnd a second paragraph.\n",
		},
		{
			ID: "int-0001", Kind: ledger.KindIntent, State: ledger.StateActive,
			Title: "An intent with unicode: naïve café — ümlaut", Created: "2026-07-30T09:00:00Z",
			Tags: []string{"latency", "storage"},
			Body: "\nBytes, not runes: a multibyte body is where a length-prefixed hash goes wrong.\n",
		},
		{
			ID: "note-0001", Kind: ledger.KindNote, State: ledger.StateActive,
			Title: "A note with no body at all", Created: "2026-07-30T09:00:00Z",
		},
	}
	ctx := context.Background()
	for _, e := range entries {
		if err := store.Create(ctx, e); err != nil {
			t.Fatalf("writing %s: %v", e.ID, err)
		}
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(entries) {
		t.Fatalf("List returned %d entries, want %d", len(list), len(entries))
	}

	for _, info := range list {
		out, err := exec.Command(git, "hash-object", filepath.Join(entriesDir, info.ID+".md")).Output()
		if err != nil {
			t.Fatalf("git hash-object %s: %v", info.ID, err)
		}
		want := strings.TrimSpace(string(out))
		if info.Version != want {
			t.Errorf("List reports version %q for %s; `git hash-object` says %q.\n"+
				"dec-0015 rests on these being the same value: it is what lets a human debug a cache without "+
				"dira, and what lets E7's github backend reuse the blob sha the Contents API already returns.",
				info.Version, info.ID, want)
		}

		// And Get agrees with List, so the value a caller carries around
		// is the same one a reindex compares against.
		entry, err := store.Get(ctx, info.ID)
		if err != nil {
			t.Fatal(err)
		}
		if entry.Version() != want {
			t.Errorf("Get reports version %q for %s, want %q", entry.Version(), info.ID, want)
		}
	}
}

// TestVersionSeesASizeAndTimePreservingEdit is dec-0015's whole argument, at the
// backend rather than through the cache. The modification-time-and-size version
// this backend shipped with cannot see this edit; a content hash cannot miss it.
func TestVersionSeesASizeAndTimePreservingEdit(t *testing.T) {
	t.Parallel()

	diraDir := filepath.Join(t.TempDir(), ".dira")
	if err := os.MkdirAll(filepath.Join(diraDir, "entries"), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	e := &ledger.Entry{
		ID: "dec-0001", Kind: ledger.KindDecision, State: ledger.StateAccepted,
		Title: "A decision that is about to be reversed behind dira's back", Created: "2026-07-30T09:00:00Z",
		Alternatives: []ledger.Alternative{{Option: "Do nothing", WhyNot: "the problem does not go away"}},
		Body:         "\nThe state field below is what changes.\n",
	}
	if err := store.Create(ctx, e); err != nil {
		t.Fatal(err)
	}

	before, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(diraDir, "entries", "dec-0001.md")
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// accepted and rejected are both eight characters.
	edited := strings.Replace(string(original), "state: accepted", "state: rejected", 1)
	if edited == string(original) {
		t.Fatal("the entry does not carry `state: accepted`; the test is aimed at the wrong field")
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, stat.ModTime(), stat.ModTime()); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() != stat.Size() || !after.ModTime().Equal(stat.ModTime()) {
		t.Fatalf("the edit was meant to preserve size and modification time; size %d -> %d, mtime %v -> %v",
			stat.Size(), after.Size(), stat.ModTime(), after.ModTime())
	}

	changed, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if changed[0].Version == before[0].Version {
		t.Errorf("Version is %q both before and after a decision was reversed in place.\n"+
			"The file's size and modification time are unchanged by construction, so any version derived from "+
			"metadata reports the entry as current — and every cache keyed on it serves `accepted` for a file "+
			"that says `rejected` (dec-0002, dec-0015).", changed[0].Version)
	}
}
