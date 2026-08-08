package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/skill"
)

// The command half of E2-L2-T8.
//
// Every test here injects a root, and the two that exercise the default root
// redirect $HOME first, so nothing in this file can reach the machine's real
// `~/.claude`. That is not politeness: a test suite that installs a skill into
// the operator's configuration while proving it installs skills would be a
// worse bug than the one it was checking for.
//
// The registry line this command needs in newApp is a one-liner
// (installSkillCommand's doc comment carries it verbatim). Until it lands,
// `withInstallSkill` registers the same *command value against a test app, so
// what is measured below is the command as it will be registered rather than a
// function called directly — the flag parsing, the exit-code mapping and the
// help rendering are all in the path. It registers only when the verb is not
// already there, so adding the line to newApp does not turn any of this red.

// withInstallSkill returns a on which `dira install-skill` is reachable.
func withInstallSkill(a *app) *app {
	if a.lookup(installSkillName) == nil {
		a.commands = append(a.commands, installSkillCommand())
	}
	return a
}

// runInstall runs the verb against buffers and reports what each stream saw.
func runInstall(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errBuf bytes.Buffer
	a := withInstallSkill(newApp(&out, &errBuf))
	code = a.main(append([]string{installSkillName}, args...))
	return code, out.String(), errBuf.String()
}

// shippedSkill is the document in this repository, read from disk. The command
// installs the embedded copy, and this is the other side of that comparison.
func shippedSkill(t *testing.T) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "skills", "dira", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading the shipped skill: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("the shipped skill is empty; every comparison below would be vacuous")
	}
	return data
}

func TestInstallSkillCommand(t *testing.T) {
	t.Parallel()

	want := shippedSkill(t)
	root := t.TempDir()
	installed := filepath.Join(root, "skills", "dira", "SKILL.md")

	t.Run("a first install writes the shipped document and exits 0", func(t *testing.T) {
		code, stdout, stderr := runInstall(t, "--root", root)
		if code != exitOK {
			t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
		}
		if !strings.Contains(stdout, string(skill.Installed)) {
			t.Errorf("stdout does not report %s:\n%s", skill.Installed, stdout)
		}
		if !strings.Contains(stdout, skill.Path) {
			t.Errorf("stdout does not name the file it wrote:\n%s", stdout)
		}
		onDisk, err := os.ReadFile(installed)
		if err != nil {
			t.Fatalf("reading %s: %v", installed, err)
		}
		if !bytes.Equal(onDisk, want) {
			t.Errorf("the installed file is not skills/dira/SKILL.md (%d bytes installed, %d shipped)",
				len(onDisk), len(want))
		}
		t.Logf("OBSERVED  exit %d, %d bytes at <root>/%s, stdout %q", code, len(onDisk), skill.Path, strings.TrimSpace(stdout))
	})

	t.Run("a second install reports UNCHANGED and exits 0", func(t *testing.T) {
		before := statOf(t, installed)

		code, stdout, stderr := runInstall(t, "--root", root)
		if code != exitOK {
			t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
		}
		if !strings.Contains(stdout, string(skill.Unchanged)) {
			t.Errorf("stdout does not report %s:\n%s", skill.Unchanged, stdout)
		}
		if after := statOf(t, installed); after != before {
			t.Errorf("the file changed on a no-op install: %v then %v", before, after)
		}
		t.Logf("OBSERVED  exit %d, stdout %q, file untouched", code, strings.TrimSpace(stdout))
	})

	t.Run("a locally edited file is named and left alone", func(t *testing.T) {
		edited := append(bytes.Clone(want), []byte("\n<!-- a note the operator added -->\n")...)
		if err := os.WriteFile(installed, edited, 0o644); err != nil {
			t.Fatalf("editing the installed file: %v", err)
		}

		code, stdout, stderr := runInstall(t, "--root", root)
		if code != exitOK {
			t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
		}
		if !strings.Contains(stdout, string(skill.Refused)) {
			t.Errorf("stdout does not report %s:\n%s", skill.Refused, stdout)
		}
		// Named, in the report a person reads, with the remedy. A
		// refusal nobody can act on is a silent failure with extra steps.
		if !strings.Contains(stderr, skill.Path) {
			t.Errorf("stderr does not name the file that was left alone:\n%s", stderr)
		}
		if !strings.Contains(stderr, "--force") {
			t.Errorf("stderr does not say how to replace it:\n%s", stderr)
		}
		onDisk, err := os.ReadFile(installed)
		if err != nil {
			t.Fatalf("reading %s: %v", installed, err)
		}
		if !bytes.Equal(onDisk, edited) {
			t.Error("the operator's edit did not survive the refusal")
		}
		t.Logf("OBSERVED  exit %d, stdout %q, stderr %q", code, strings.TrimSpace(stdout), strings.TrimSpace(stderr))
	})

	t.Run("--force replaces it", func(t *testing.T) {
		code, stdout, stderr := runInstall(t, "--root", root, "--force")
		if code != exitOK {
			t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
		}
		if !strings.Contains(stdout, string(skill.Installed)) {
			t.Errorf("stdout does not report %s:\n%s", skill.Installed, stdout)
		}
		onDisk, err := os.ReadFile(installed)
		if err != nil {
			t.Fatalf("reading %s: %v", installed, err)
		}
		if !bytes.Equal(onDisk, want) {
			t.Error("--force did not restore the shipped document")
		}
		t.Logf("OBSERVED  exit %d, stdout %q", code, strings.TrimSpace(stdout))
	})

	t.Run("an argument is a usage error and prints nothing to stdout", func(t *testing.T) {
		code, stdout, stderr := runInstall(t, "--root", root, "somewhere")
		if code != exitUsage {
			t.Errorf("exit code = %d, want %d", code, exitUsage)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty on a usage error", stdout)
		}
		if !strings.Contains(stderr, "takes no arguments") {
			t.Errorf("stderr does not explain the mistake:\n%s", stderr)
		}
	})
}

// TestInstallSkillCommandDefaultsToTheHomeClaudeDirectory covers the path a
// person actually runs, with $HOME redirected under a temp directory so the
// real one is never reached.
//
// It also asserts the reported path is the tilde form. The absolute path
// carries the operator's username, and this repository treats every output as
// potentially public.
func TestInstallSkillCommandDefaultsToTheHomeClaudeDirectory(t *testing.T) {
	// Not parallel: it sets HOME for the process.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	code, stdout, stderr := runInstall(t)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
	}

	installed := filepath.Join(home, ".claude", "skills", "dira", "SKILL.md")
	onDisk, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("the default install wrote no %s: %v", installed, err)
	}
	if !bytes.Equal(onDisk, shippedSkill(t)) {
		t.Error("the default install did not write the shipped document")
	}
	if !strings.Contains(stdout, "~/.claude/"+skill.Path) {
		t.Errorf("stdout does not report the tilde path:\n%s", stdout)
	}
	if strings.Contains(stdout, home) {
		t.Errorf("stdout carries the absolute home path, which names the operator:\n%s", stdout)
	}
	t.Logf("OBSERVED  exit %d, %d bytes at $HOME/.claude/%s, stdout %q",
		code, len(onDisk), skill.Path, strings.TrimSpace(stdout))
}

// TestInstallSkillCommandWritesNothingOutsideTheRoot is the confinement clause,
// measured on the production adapter rather than on a fixture.
//
// The escape is attempted through claudeRoot itself, because internal/skill
// cannot express one — it only ever passes skill.Path — so the only place the
// guarantee could fail is here.
func TestInstallSkillCommandWritesNothingOutsideTheRoot(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	inside := filepath.Join(parent, "claude")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatalf("creating the root: %v", err)
	}
	opened, err := os.OpenRoot(inside)
	if err != nil {
		t.Fatalf("opening the root: %v", err)
	}
	defer func() { _ = opened.Close() }()
	root := claudeRoot{root: opened}

	// Green first: the name the installer actually uses works, so the red
	// below is about the escape and not about a broken adapter.
	if err := root.WriteFile(skill.Path, []byte("in\n")); err != nil {
		t.Fatalf("writing %s inside the root: %v", skill.Path, err)
	}
	if _, err := os.ReadFile(filepath.Join(inside, "skills", "dira", "SKILL.md")); err != nil {
		t.Fatalf("the adapter reported success but wrote nothing: %v", err)
	}

	for _, name := range []string{"../escaped.md", "../../escaped.md", "skills/../../escaped.md"} {
		if err := root.WriteFile(name, []byte("out\n")); err == nil {
			t.Errorf("writing %q was allowed; it names a file outside the root", name)
		} else {
			t.Logf("OBSERVED  refused %q: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(parent, "escaped.md")); !os.IsNotExist(err) {
		t.Errorf("a file appeared outside the root: %v", err)
	}
}

// TestInstallSkillCommandAppearsInHelp is the acceptance clause about `dira
// --help`, checked through the same registry iteration E0-L1-T4's help test
// uses. Once the registry line is in newApp this measures the shipped binary;
// until then it measures the same *command value that line will register.
func TestInstallSkillCommandAppearsInHelp(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	a := withInstallSkill(newApp(&out, &errBuf))

	if code := a.main([]string{"--help"}); code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, errBuf.String())
	}
	stdout := out.String()
	if !strings.Contains(stdout, installSkillName) {
		t.Errorf("help does not name %q:\n%s", installSkillName, stdout)
	}
	if !strings.Contains(stdout, installSkillSummary) {
		t.Errorf("help does not carry the summary for %q:\n%s", installSkillName, stdout)
	}

	// And the command's own help, which is where the flags are documented.
	out.Reset()
	errBuf.Reset()
	if code := a.main([]string{"help", installSkillName}); code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, errBuf.String())
	}
	for _, want := range []string{"--force", "--root", skill.Path, string(skill.Unchanged)} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("`dira help %s` does not mention %q:\n%s", installSkillName, want, out.String())
		}
	}
	viaHelp := out.String()

	// `dira help <name>` and `dira <name> -h` must render the same text — the
	// contract on the command struct in main.go. A flag documented in one and
	// not the other is a flag nobody finds.
	out.Reset()
	errBuf.Reset()
	if code := a.main([]string{installSkillName, "-h"}); code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, errBuf.String())
	}
	if out.String() != viaHelp {
		t.Errorf("`dira %s -h` and `dira help %s` render different text", installSkillName, installSkillName)
	}
	t.Logf("OBSERVED  help names %q, documents --root, --force and all three outcomes, and both help routes render the same %d bytes",
		installSkillName, len(viaHelp))
}

// statOf returns the size and modification time of a file, as the pair a no-op
// install must not change.
//
// mtime, not just size: rewriting identical bytes leaves the size alone and is
// exactly the churn "byte-level no-op" rules out.
func statOf(t *testing.T, path string) string {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return fmt.Sprintf("%d bytes, modified %s", info.Size(), info.ModTime().Format(time.RFC3339Nano))
}
