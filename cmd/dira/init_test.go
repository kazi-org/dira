package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/interview"
)

// completeInterviewAnswers is the exact fixture internal/interview's own
// TestBuild uses, reused here so the CLI is exercised against the same
// scripted answer set that package proves is complete and valid.
var completeInterviewAnswers = []string{
	"person",
	"ship the personal ledger before the workspace one asks for it",
	"nothing about where a session's time went ever leaves this machine",
	"how much of a week counts as focused before the drift report says so",
}

func runInitCmd(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	a := newApp(&out, &errBuf)
	a.stdin = strings.NewReader(stdin)
	code = a.main(append([]string{"init"}, args...))
	return code, out.String(), errBuf.String()
}

func answerLines(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

// TestInitInterview is E5-L5-T3's acceptance line.
func TestInitInterview(t *testing.T) {
	t.Run("a complete answer set exits 0, prints the path, and a real brief run finds it", func(t *testing.T) {
		root := t.TempDir()
		code, stdout, stderr := runInitCmd(t, answerLines(completeInterviewAnswers...), "-C", root, "--interview")
		if code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
		}
		if !strings.Contains(stdout, root) {
			t.Errorf("stdout does not name the path it created:\n%s", stdout)
		}

		var briefOut, briefErr bytes.Buffer
		b := newApp(&briefOut, &briefErr)
		if b.lookup("brief") == nil {
			b.commands = append(b.commands, &command{name: "brief", summary: briefSummary, run: runBrief, usage: writeBriefUsage})
		}
		briefCode := b.main([]string{"brief", "-C", root})
		if briefCode != exitOK {
			t.Fatalf("brief exit code = %d, want %d\nstderr: %s", briefCode, exitOK, briefErr.String())
		}
		if strings.TrimSpace(briefOut.String()) == "" {
			t.Fatal("brief over the freshly-seeded ledger printed nothing")
		}
		if !strings.Contains(briefOut.String(), "ship the personal ledger") {
			t.Errorf("brief does not name the seeded intent:\n%s", briefOut.String())
		}
	})

	t.Run("an incomplete answer set exits non-zero and leaves no .dira at all", func(t *testing.T) {
		root := t.TempDir()
		// Only the tier and the first entry prompt are answered; stdin
		// closes before the constraint and question prompts.
		code, _, _ := runInitCmd(t, answerLines("person", completeInterviewAnswers[1]), "-C", root, "--interview")
		if code == exitOK {
			t.Fatal("an incomplete interview exited 0")
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("reading %s: %v", root, err)
		}
		if len(entries) != 0 {
			t.Errorf("%s is not empty after an incomplete interview: %v", root, entries)
		}
	})

	t.Run("a --tier mismatch is rejected before any entry prompt is asked, at exit 2", func(t *testing.T) {
		root := t.TempDir()
		code, stdout, _ := runInitCmd(t, answerLines(completeInterviewAnswers...), "-C", root, "--interview", "--tier", "workspace")
		if code != exitUsage {
			t.Fatalf("exit code = %d, want %d (usage)", code, exitUsage)
		}
		for _, prompt := range interview.Prompts[1:] {
			if strings.Contains(stdout, prompt) {
				t.Errorf("an entry prompt was printed after the --tier mismatch:\n%s", stdout)
			}
		}
		if entries, _ := os.ReadDir(root); len(entries) != 0 {
			t.Errorf("%s is not empty after a rejected --tier mismatch", root)
		}
	})

	t.Run("an invalid --tier value is rejected at exit 2 before any prompt at all", func(t *testing.T) {
		root := t.TempDir()
		code, stdout, _ := runInitCmd(t, answerLines(completeInterviewAnswers...), "-C", root, "--interview", "--tier", "repo")
		if code != exitUsage {
			t.Fatalf("exit code = %d, want %d", code, exitUsage)
		}
		if strings.Contains(stdout, interview.Prompts[0]) {
			t.Errorf("a prompt was printed for an invalid --tier value:\n%s", stdout)
		}
	})

	t.Run("running a second time against an existing .dira exits non-zero and writes nothing", func(t *testing.T) {
		root := t.TempDir()
		if code, _, stderr := runInitCmd(t, answerLines(completeInterviewAnswers...), "-C", root, "--interview"); code != exitOK {
			t.Fatalf("first init: exit %d, stderr %s", code, stderr)
		}

		before := digestPath(t, filepath.Join(root, ".dira"))
		code, _, stderr := runInitCmd(t, answerLines(completeInterviewAnswers...), "-C", root, "--interview")
		if code == exitOK {
			t.Fatal("a second init over an existing .dira exited 0")
		}
		if strings.TrimSpace(stderr) == "" {
			t.Error("a second init over an existing .dira wrote nothing to stderr")
		}
		if after := digestPath(t, filepath.Join(root, ".dira")); after != before {
			t.Error("the existing .dira's contents changed on a second init")
		}
	})

	t.Run("init is reachable through the registry", func(t *testing.T) {
		a := newApp(&bytes.Buffer{}, &bytes.Buffer{})
		if a.lookup("init") == nil {
			t.Fatal("dira init is not registered in newApp's command list")
		}
	})

	t.Run("nothing under the real working directory is touched", func(t *testing.T) {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		before, err := os.ReadDir(wd)
		if err != nil {
			t.Fatalf("reading %s: %v", wd, err)
		}
		beforeNames := dirNames(before)

		root := t.TempDir()
		runInitCmd(t, answerLines(completeInterviewAnswers...), "-C", root, "--interview")

		after, err := os.ReadDir(wd)
		if err != nil {
			t.Fatalf("reading %s: %v", wd, err)
		}
		if got := dirNames(after); !equalNameSets(beforeNames, got) {
			t.Errorf("the real working directory %s changed: %v -> %v", wd, beforeNames, got)
		}
	})
}

func dirNames(entries []os.DirEntry) map[string]bool {
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		out[e.Name()] = true
	}
	return out
}

func equalNameSets(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func digestPath(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return rerr
		}
		b.WriteString(rel)
		b.WriteString(string(data))
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return b.String()
}
