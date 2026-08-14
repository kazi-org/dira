package drift

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// states extracts just the State field of every classification, so a test
// can compare "the map" in one shot rather than key by key.
func states(result map[string]Classification) map[string]State {
	out := make(map[string]State, len(result))
	for id, c := range result {
		out[id] = c.State
	}
	return out
}

// TestClassify is E5-L3-T1's acceptance line.
func TestClassify(t *testing.T) {
	t.Run("orphan, oriented and withheld, in one Classify call", func(t *testing.T) {
		root := copyFixture(t, "abc")
		abcDira := filepath.Join(root, ".dira")
		// C's target (sire:int-0009) is declared and its ledger opens
		// fine; only the one entry file the ref names is unreadable —
		// which chain.Resolve reports as an error distinct from
		// ErrUndeclaredNamespace, and this package treats as withheld:
		// a real, declared reference it cannot currently see.
		chmodUnreadable(t, filepath.Join(root, "sire", ".dira", "entries", "int-0009.md"))

		// The ledger under test (abc) is never permission-restricted, so
		// its own digest is taken over the whole tree; sire's readable
		// entry is checked by name and bytes separately, since one of
		// its sibling files is deliberately unreadable for this subtest.
		beforeAbc := treeDigest(t, abcDira)
		beforeSireReadable, err := readFile(filepath.Join(root, "sire", ".dira", "entries", "int-0002.md"))
		if err != nil {
			t.Fatalf("reading sire's readable entry: %v", err)
		}
		beforeGit := gitStatus(t)

		got, err := Classify(context.Background(), abcDira)
		if err != nil {
			t.Fatalf("Classify: %v", err)
		}

		want := map[string]State{
			"int-0001": Orphan,
			"int-0002": Oriented,
			"int-0003": Withheld,
			"int-0004": Broken,
		}
		if got := states(got); !reflect.DeepEqual(got, want) {
			t.Fatalf("states = %+v, want %+v", got, want)
		}

		assertGitUnchanged(t, beforeGit)
		if after := treeDigest(t, abcDira); after != beforeAbc {
			t.Errorf("abc's own tree changed during Classify: %s -> %s", beforeAbc, after)
		}
		afterSireReadable, err := readFile(filepath.Join(root, "sire", ".dira", "entries", "int-0002.md"))
		if err != nil {
			t.Fatalf("reading sire's readable entry: %v", err)
		}
		if beforeSireReadable != afterSireReadable {
			t.Error("sire's readable entry changed during Classify")
		}
	})

	t.Run("re-run after restoring access moves C to Oriented; A, B, D unchanged", func(t *testing.T) {
		root := copyFixture(t, "abc")
		abcDira := filepath.Join(root, ".dira")
		target := filepath.Join(root, "sire", ".dira", "entries", "int-0009.md")

		if err := os.Chmod(target, 0o000); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		first, err := Classify(context.Background(), abcDira)
		if err != nil {
			t.Fatalf("Classify (first run): %v", err)
		}
		if first["int-0003"].State != Withheld {
			t.Fatalf("int-0003 = %v before restoring access, want Withheld", first["int-0003"].State)
		}

		if err := os.Chmod(target, 0o644); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		second, err := Classify(context.Background(), abcDira)
		if err != nil {
			t.Fatalf("Classify (second run): %v", err)
		}

		if second["int-0003"].State != Oriented {
			t.Errorf("int-0003 after restoring access = %v, want Oriented", second["int-0003"].State)
		}
		for _, id := range []string{"int-0001", "int-0002", "int-0004"} {
			if first[id].State != second[id].State {
				t.Errorf("%s changed between runs: %v -> %v, want unchanged", id, first[id].State, second[id].State)
			}
		}

		drift := map[string]bool{}
		for id, c := range second {
			if c.State == Orphan {
				drift[id] = true
			}
		}
		if want := map[string]bool{"int-0001": true}; !reflect.DeepEqual(drift, want) {
			t.Errorf("drift set = %v, want %v", drift, want)
		}
	})

	t.Run("a decision with no derives_from edge is absent from the result", func(t *testing.T) {
		root := copyFixture(t, "abc")
		got, err := Classify(context.Background(), filepath.Join(root, ".dira"))
		if err != nil {
			t.Fatalf("Classify: %v", err)
		}
		if _, ok := got["dec-0001"]; ok {
			t.Error("dec-0001 (a decision, not an intent) appears in Classify's result")
		}

		wantKeys := map[string]bool{"int-0001": true, "int-0002": true, "int-0003": true, "int-0004": true}
		gotKeys := map[string]bool{}
		for id := range got {
			gotKeys[id] = true
		}
		if !reflect.DeepEqual(gotKeys, wantKeys) {
			t.Errorf("result keys = %v, want exactly the active intents %v", gotKeys, wantKeys)
		}
	})

	t.Run("folding Broken into Withheld is caught by the four-state assertion", func(t *testing.T) {
		root := copyFixture(t, "abc")
		got, err := Classify(context.Background(), filepath.Join(root, ".dira"))
		if err != nil {
			t.Fatalf("Classify: %v", err)
		}

		// The red control: a deliberately collapsed view where Broken
		// reads as Withheld, the exact defect dec-0011's implementation
		// notes forbid.
		collapsed := states(got)
		for id, s := range collapsed {
			if s == Broken {
				collapsed[id] = Withheld
			}
		}
		want := map[string]State{"int-0001": Orphan, "int-0002": Oriented, "int-0004": Withheld}
		delete(collapsed, "int-0003") // this subtest is not about C
		if !reflect.DeepEqual(collapsed, want) {
			t.Fatalf("the collapsed view did not reproduce the defect it exists to catch: %+v", collapsed)
		}

		// The real Classify must not agree with the collapsed view.
		if got["int-0004"].State == Withheld {
			t.Fatal("Classify folded a broken (undeclared-namespace) ref into withheld")
		}
		if got["int-0004"].State != Broken {
			t.Fatalf("int-0004 = %v, want Broken", got["int-0004"].State)
		}
	})
}

// readFile is a small local wrapper so the test file needs only "os" for one
// purpose, kept next to its two call sites.
func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}
