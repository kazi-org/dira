package chain

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const canary = "CANARY-ME-INT-0002-BODY-MUST-NEVER-CROSS-THE-BOUNDARY"

// TestResolve is E5-L2-T2's acceptance line.
func TestResolve(t *testing.T) {
	t.Run("oriented: me readable resolves me:int-0002 to the entry", func(t *testing.T) {
		root := copyFixture(t, "tier3")
		repoDira := filepath.Join(root, "repo", ".dira")

		// The canary-absence clause is non-vacuous only if the canary is
		// genuinely present in the untouched fixture first.
		meEntry := filepath.Join(root, "me", ".dira", "entries", "int-0002.md")
		data, err := os.ReadFile(meEntry)
		if err != nil {
			t.Fatalf("reading the fixture entry: %v", err)
		}
		if !strings.Contains(string(data), canary) {
			t.Fatalf("the fixture entry does not carry the canary; this test would pass vacuously")
		}

		before := gitStatus(t)
		state, entry, err := Resolve(context.Background(), repoDira, "me:int-0002")
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if state != Oriented {
			t.Errorf("state = %q, want %q", state, Oriented)
		}
		if entry == nil || entry.ID != "int-0002" {
			t.Fatalf("entry = %+v, want int-0002", entry)
		}

		assertGitUnchanged(t, before)
		assertCanaryAbsent(t, filepath.Join(root, "repo", ".dira"))
	})

	t.Run("withheld: me's path absent, nil error, nil entry", func(t *testing.T) {
		root := copyFixture(t, "tier3")
		repoDira := filepath.Join(root, "repo", ".dira")
		if err := os.RemoveAll(filepath.Join(root, "me")); err != nil {
			t.Fatalf("removing me/: %v", err)
		}

		before := gitStatus(t)
		state, entry, err := Resolve(context.Background(), repoDira, "me:int-0002")
		if err != nil {
			t.Fatalf("Resolve returned an error for a withheld ancestor; withheld is success, not failure: %v", err)
		}
		if state != Withheld {
			t.Errorf("state = %q, want %q", state, Withheld)
		}
		if entry != nil {
			t.Errorf("entry = %+v, want nil", entry)
		}

		assertGitUnchanged(t, before)
		assertCanaryAbsent(t, repoDira)
	})

	t.Run("withheld: me present but chmod 000, nil error, nil entry", func(t *testing.T) {
		root := copyFixture(t, "tier3")
		repoDira := filepath.Join(root, "repo", ".dira")
		chmodUnreadable(t, filepath.Join(root, "me", ".dira"))

		before := gitStatus(t)
		state, entry, err := Resolve(context.Background(), repoDira, "me:int-0002")
		if err != nil {
			t.Fatalf("Resolve returned an error for a withheld ancestor: %v", err)
		}
		if state != Withheld {
			t.Errorf("state = %q, want %q", state, Withheld)
		}
		if entry != nil {
			t.Errorf("entry = %+v, want nil", entry)
		}

		assertGitUnchanged(t, before)
		assertCanaryAbsent(t, repoDira)
	})

	t.Run("undeclared namespace is a named error, never withheld", func(t *testing.T) {
		root := copyFixture(t, "tier3")
		repoDira := filepath.Join(root, "repo", ".dira")

		state, entry, err := Resolve(context.Background(), repoDira, "nowhere:int-0001")
		if err == nil {
			t.Fatal("Resolve over an undeclared namespace returned no error")
		}
		if !strings.Contains(err.Error(), "nowhere") {
			t.Errorf("error %q does not name the undeclared namespace", err.Error())
		}
		if state != "" {
			t.Errorf("state = %q, want the zero value", state)
		}
		if entry != nil {
			t.Errorf("entry = %+v, want nil", entry)
		}
	})

	t.Run("a malformed ref is rejected before Walk runs", func(t *testing.T) {
		root := copyFixture(t, "tier3")
		repoDira := filepath.Join(root, "repo", ".dira")

		calls := 0
		original := walkFunc
		walkFunc = func(ctx context.Context, diraDir string) ([]Ancestor, error) {
			calls++
			return original(ctx, diraDir)
		}
		t.Cleanup(func() { walkFunc = original })

		_, _, err := Resolve(context.Background(), repoDira, "not a valid ref")
		if err == nil {
			t.Fatal("Resolve over a malformed ref returned no error")
		}
		if calls != 0 {
			t.Errorf("Walk was called %d times for a malformed ref; it must not run at all", calls)
		}
	})

	t.Run("undeclared-namespace collapse into withheld is caught", func(t *testing.T) {
		// The red control this test's own package comment promises: a
		// build that folded "undeclared" into "withheld" would pass every
		// withheld-shaped assertion above and only this subtest would
		// catch it.
		root := copyFixture(t, "tier3")
		repoDira := filepath.Join(root, "repo", ".dira")

		ancestors, err := Walk(context.Background(), repoDira)
		if err != nil {
			t.Fatalf("Walk: %v", err)
		}

		// A deliberately broken resolver that collapses "namespace not
		// found" into Withheld instead of a named error.
		brokenResolve := func(namespace string) (State, error) {
			for _, a := range ancestors {
				if a.Namespace == namespace {
					return Withheld, nil
				}
			}
			return Withheld, nil // the defect: undeclared reads as withheld
		}

		state, err := brokenResolve("nowhere")
		if err != nil || state != Withheld {
			t.Fatalf("the broken resolver did not reproduce the defect it exists to catch")
		}

		// The real implementation must not agree with it.
		realState, _, realErr := Resolve(context.Background(), repoDira, "nowhere:int-0001")
		if realErr == nil {
			t.Fatal("the real Resolve collapsed an undeclared namespace into a non-error state")
		}
		if realState == Withheld {
			t.Fatal("the real Resolve reported an undeclared namespace as withheld")
		}
	})
}

// gitStatus is `git status --porcelain` in this repository — not the temp
// copy Resolve reads — captured so a subtest can prove Resolve changed
// nothing about it.
//
// It is compared before and after rather than asserted empty on its own: a
// worktree mid-development legitimately holds staged, uncommitted work (this
// very package, before its own commit), and asserting a pristine tree would
// make the check fail on every run that is not also the run that commits it.
// What E5-L2-T2's acc line is actually testing — "nothing under version
// control was touched" by the read — is exactly what a before/after diff
// proves, without depending on the tree being otherwise clean.
func gitStatus(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Skipf("no git available to check: %v", err)
	}
	return string(out)
}

// assertGitUnchanged fails the test if git status moved since before was
// captured.
func assertGitUnchanged(t *testing.T, before string) {
	t.Helper()
	after := gitStatus(t)
	if after != before {
		t.Errorf("git status --porcelain changed during Resolve:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// assertCanaryAbsent greps every file under dir for the canary string, which
// belongs only to me's entry body and must never be copied under a child's
// own tree.
func assertCanaryAbsent(t *testing.T, dir string) {
	t.Helper()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		if strings.Contains(string(data), canary) {
			t.Errorf("%s carries the canary string; a parent's private body leaked under a child's own tree", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
}
