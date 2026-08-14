package interview

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

// completeAnswers is the fixture answer set every other lane's tests reuse
// (E5-L5-T3, T4).
var completeAnswers = []string{
	"person",
	"ship the personal ledger before the workspace one asks for it",
	"nothing about where a session's time went ever leaves this machine",
	"how much of a week counts as focused before the drift report says so",
}

// TestBuild is E5-L5-T1's acceptance line.
func TestBuild(t *testing.T) {
	t.Run("a complete answer set returns one draft of each kind, valid and tier-tagged", func(t *testing.T) {
		tier, drafts, err := Build(completeAnswers)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if tier != "person" {
			t.Errorf("tier = %q, want person", tier)
		}
		if len(drafts) != 3 {
			t.Fatalf("got %d drafts, want 3", len(drafts))
		}

		byKind := map[ledger.Kind]*ledger.Entry{}
		for _, d := range drafts {
			byKind[d.Kind] = d
		}
		for _, want := range []struct {
			kind  ledger.Kind
			state ledger.State
		}{
			{ledger.KindIntent, ledger.StateActive},
			{ledger.KindConstraint, ledger.StateActive},
			{ledger.KindQuestion, ledger.StateOpen},
		} {
			d, ok := byKind[want.kind]
			if !ok {
				t.Fatalf("no draft of kind %s", want.kind)
			}
			if d.State != want.state {
				t.Errorf("%s: state = %q, want %q", want.kind, d.State, want.state)
			}
			if err := d.ValidateDraft(); err != nil {
				t.Errorf("%s draft does not validate: %v", want.kind, err)
			}
			if d.ID != "" {
				t.Errorf("%s draft carries an id %q before it has been written", want.kind, d.ID)
			}
		}

		// entry.schema.json has no tier field on any kind — the tier is
		// returned alongside the drafts, never baked into one.
		for _, d := range drafts {
			v := reflect.ValueOf(*d)
			if v.FieldByName("Tier").IsValid() {
				t.Fatalf("ledger.Entry now has a Tier field; drafts must not carry one")
			}
		}
	})

	t.Run("workspace is also a valid tier answer", func(t *testing.T) {
		answers := append([]string{}, completeAnswers...)
		answers[0] = "workspace"
		tier, drafts, err := Build(answers)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if tier != "workspace" {
			t.Errorf("tier = %q, want workspace", tier)
		}
		if len(drafts) != 3 {
			t.Errorf("got %d drafts, want 3", len(drafts))
		}
	})

	t.Run("fewer lines than prompts (stdin closing early) is a named error and zero drafts", func(t *testing.T) {
		short := completeAnswers[:2]
		tier, drafts, err := Build(short)
		if err == nil {
			t.Fatal("Build over an incomplete answer set returned no error")
		}
		if tier != "" {
			t.Errorf("tier = %q, want empty", tier)
		}
		if len(drafts) != 0 {
			t.Errorf("got %d drafts, want 0", len(drafts))
		}
	})

	t.Run("a required prompt answered with only whitespace is a named error naming which prompt", func(t *testing.T) {
		answers := append([]string{}, completeAnswers...)
		answers[2] = "   "
		tier, drafts, err := Build(answers)
		if err == nil {
			t.Fatal("Build over a blank required answer returned no error")
		}
		if tier != "" || len(drafts) != 0 {
			t.Fatalf("tier = %q, drafts = %v; want zero values", tier, drafts)
		}
		if !strings.Contains(err.Error(), Prompts[2]) {
			t.Errorf("error %q does not name the blank prompt %q", err.Error(), Prompts[2])
		}
	})

	t.Run("an invalid tier answer is a named error", func(t *testing.T) {
		answers := append([]string{}, completeAnswers...)
		answers[0] = "repo"
		_, drafts, err := Build(answers)
		if err == nil {
			t.Fatal("Build accepted an out-of-vocabulary tier")
		}
		if len(drafts) != 0 {
			t.Errorf("got %d drafts, want 0", len(drafts))
		}
	})

	t.Run("Build performs no I/O", func(t *testing.T) {
		empty := t.TempDir()
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		t.Chdir(empty)
		t.Cleanup(func() { _ = os.Chdir(wd) })

		if _, _, err := Build(completeAnswers); err != nil {
			t.Fatalf("Build failed with no .dira anywhere near the working directory: %v", err)
		}
	})

	t.Run("calling Build twice with the same answers produces identical drafts", func(t *testing.T) {
		_, first, err := Build(completeAnswers)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		_, second, err := Build(completeAnswers)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if len(first) != len(second) {
			t.Fatalf("draft counts differ: %d vs %d", len(first), len(second))
		}
		for i := range first {
			a, b := first[i], second[i]
			if a.Title != b.Title || a.Kind != b.Kind || a.State != b.State || a.Body != b.Body {
				t.Errorf("draft %d differs between runs: %+v vs %+v", i, a, b)
			}
		}
	})
}
