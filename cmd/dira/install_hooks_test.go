package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/installhooks"
)

// E2-L3-T6's acceptance.
//
// Every test here injects a root via --dir, or Chdir's into a temp directory
// for --local, or redirects $HOME for the default-root case -- never once
// does a call in this file reach the real ~/.claude. claudeRoot's escape
// confinement (`..`, a symlink out of the root) is install-skill's adapter,
// reused rather than redeclared, and is already proven in
// install_skill_test.go's TestInstallSkillCommandWritesNothingOutsideTheRoot;
// this file does not re-run that proof against the identical code.

// withInstallHooks returns a on which `dira install-hooks` is reachable.
func withInstallHooks(a *app) *app {
	if a.lookup(installHooksName) == nil {
		a.commands = append(a.commands, installHooksCommand())
	}
	return a
}

func runHooks(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errBuf bytes.Buffer
	a := withInstallHooks(newApp(&out, &errBuf))
	code = a.main(append([]string{installHooksName}, args...))
	return code, out.String(), errBuf.String()
}

func TestInstallHooksCommand(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	installed := filepath.Join(dir, "settings.json")

	t.Run("a first install writes the user-level file, prints INSTALLED and the path, and exits 0", func(t *testing.T) {
		code, stdout, stderr := runHooks(t, "--dir", dir)
		if code != exitOK {
			t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
		}
		if !strings.Contains(stdout, string(installhooks.Installed)) {
			t.Errorf("stdout does not report %s:\n%s", installhooks.Installed, stdout)
		}
		if !strings.Contains(stdout, installed) {
			t.Errorf("stdout does not name the file it wrote:\n%s", stdout)
		}
		data, err := os.ReadFile(installed)
		if err != nil {
			t.Fatalf("reading %s: %v", installed, err)
		}
		if !json.Valid(data) {
			t.Fatalf("the written file is not valid JSON:\n%s", data)
		}
		t.Logf("OBSERVED  exit %d, %d bytes at %s", code, len(data), installed)
	})

	t.Run("a second install prints UNCHANGED, exits 0, and the sha256 is unchanged", func(t *testing.T) {
		before := shaOf(t, installed)

		code, stdout, stderr := runHooks(t, "--dir", dir)
		if code != exitOK {
			t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
		}
		if !strings.Contains(stdout, string(installhooks.Unchanged)) {
			t.Errorf("stdout does not report %s:\n%s", installhooks.Unchanged, stdout)
		}
		if after := shaOf(t, installed); after != before {
			t.Errorf("the file's sha256 changed on a no-op install: %x then %x", before, after)
		}
	})

	t.Run("every success path prints the exact --uninstall invocation, including --dir", func(t *testing.T) {
		_, stdout, _ := runHooks(t, "--dir", dir)
		want := "dira install-hooks --uninstall --dir " + dir
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not print the exact revert invocation %q:\n%s", want, stdout)
		}
	})

	t.Run("--uninstall after a fresh install into an absent file leaves no file on disk", func(t *testing.T) {
		freshDir := t.TempDir()
		file := filepath.Join(freshDir, "settings.json")

		if code, _, stderr := runHooks(t, "--dir", freshDir); code != exitOK {
			t.Fatalf("fixture setup: install exit %d (stderr: %s)", code, stderr)
		}
		if _, err := os.Stat(file); err != nil {
			t.Fatalf("fixture setup: install did not create %s: %v", file, err)
		}

		code, stdout, stderr := runHooks(t, "--dir", freshDir, "--uninstall")
		if code != exitOK {
			t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
		}
		if !strings.Contains(stdout, string(installhooks.Removed)) {
			t.Errorf("stdout does not report %s:\n%s", installhooks.Removed, stdout)
		}
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("%s still exists after uninstalling a fresh install: %v", file, err)
		}
	})
}

// TestInstallHooksLocal is not parallel: it Chdirs the process, which
// t.Chdir refuses to combine with parallel tests or parallel ancestors.
func TestInstallHooksLocal(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	local := filepath.Join(dir, claudeDirName, installHooksLocalFile)
	committed := filepath.Join(dir, claudeDirName, installHooksUserFile)

	code, stdout, stderr := runHooks(t, "--local")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
	}
	if _, err := os.Stat(local); err != nil {
		t.Fatalf("--local did not create %s: %v", local, err)
	}
	// The clause this test exists for: the committed file must never appear
	// alongside the local one.
	if _, err := os.Stat(committed); !os.IsNotExist(err) {
		t.Fatalf("the committed %s exists after a --local install; that file must never be created by this command", committed)
	}
	want := "dira install-hooks --uninstall --local"
	if !strings.Contains(stdout, want) {
		t.Errorf("stdout does not print the exact revert invocation %q:\n%s", want, stdout)
	}

	t.Run("--uninstall --local targets the local file only", func(t *testing.T) {
		code, stdout, stderr := runHooks(t, "--uninstall", "--local")
		if code != exitOK {
			t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
		}
		if !strings.Contains(stdout, string(installhooks.Removed)) {
			t.Errorf("stdout does not report %s:\n%s", installhooks.Removed, stdout)
		}
		if _, err := os.Stat(local); !os.IsNotExist(err) {
			t.Errorf("%s still exists after --uninstall --local", local)
		}
		if _, err := os.Stat(committed); !os.IsNotExist(err) {
			t.Error("the committed file was created by --uninstall --local")
		}
	})
}

func TestInstallHooksDirOverridesLocal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	code, stdout, stderr := runHooks(t, "--dir", dir, "--local")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
	}
	local := filepath.Join(dir, "settings.local.json")
	nonLocal := filepath.Join(dir, "settings.json")
	if _, err := os.Stat(local); err != nil {
		t.Fatalf("--dir with --local did not create %s: %v", local, err)
	}
	if _, err := os.Stat(nonLocal); !os.IsNotExist(err) {
		t.Error("--dir with --local also created the non-local file name")
	}
	want := "dira install-hooks --uninstall --dir " + dir + " --local"
	if !strings.Contains(stdout, want) {
		t.Errorf("stdout does not print the exact revert invocation %q:\n%s", want, stdout)
	}
}

// TestInstallHooksDefaultRootUsesHomeClaude covers the path a person actually
// runs, with $HOME redirected under a temp directory so the real one is never
// reached -- the same guard TestInstallSkillCommandDefaultsToTheHomeClaudeDirectory
// uses for the identical reason.
func TestInstallHooksDefaultRootUsesHomeClaude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	code, stdout, stderr := runHooks(t)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
	}
	installed := filepath.Join(home, claudeDirName, installHooksUserFile)
	if _, err := os.ReadFile(installed); err != nil {
		t.Fatalf("the default install wrote no %s: %v", installed, err)
	}
	if !strings.Contains(stdout, "~/"+claudeDirName+"/"+installHooksUserFile) {
		t.Errorf("stdout does not report the tilde path:\n%s", stdout)
	}
	if strings.Contains(stdout, home) {
		t.Errorf("stdout carries the absolute home path, which names the operator:\n%s", stdout)
	}
}

func TestInstallHooksMalformedFileExitsOneWithOneLine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(file, []byte("{ this is not json"), 0o644); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	before := shaOf(t, file)

	code, stdout, stderr := runHooks(t, "--dir", dir)
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on a runtime error", stdout)
	}
	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("stderr has %d line(s), want exactly 1: %q", len(lines), stderr)
	}
	if after := shaOf(t, file); after != before {
		t.Error("the malformed file was modified")
	}
	t.Logf("OBSERVED  exit %d, stderr %q, file unmodified", code, strings.TrimSpace(stderr))
}

func TestInstallHooksUnknownFlagExitsTwo(t *testing.T) {
	t.Parallel()

	code, stdout, stderr := runHooks(t, "--e2-l3-t6-no-such-flag")
	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on a usage error", stdout)
	}
	if !strings.Contains(stderr, unknownFlagReport) {
		t.Errorf("stderr does not report the unknown flag:\n%s", stderr)
	}
}

func TestInstallHooksUninstallRefusalExitsZeroAndNamesTheEntry(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "settings.json")
	data := []byte(`{
  "hooks": {
    "Stop": [
      { "hooks": [{ "type": "command", "command": "dira sniff --stage --quiet --extra-flag 2>/dev/null || true", "timeout": 10 }] },
      { "hooks": [{ "type": "command", "command": "dira-sniffer-wrapper.sh", "timeout": 10 }] }
    ]
  }
}`)
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	code, stdout, stderr := runHooks(t, "--dir", dir, "--uninstall")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
	}
	if !strings.Contains(stdout, string(installhooks.Removed)) {
		t.Errorf("stdout does not report %s:\n%s", installhooks.Removed, stdout)
	}
	if !strings.Contains(stderr, "Stop") || !strings.Contains(stderr, "dira-sniffer-wrapper.sh") {
		t.Errorf("stderr does not name the untouched entry:\n%s", stderr)
	}
	onDisk, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("reading %s: %v", file, err)
	}
	if !strings.Contains(string(onDisk), "dira-sniffer-wrapper.sh") {
		t.Error("the entry reported as left alone was in fact removed")
	}
	if strings.Contains(string(onDisk), "--extra-flag") {
		t.Error("the flag-edited entry, which does carry dira's own prefix, was not removed")
	}
	t.Logf("OBSERVED  exit %d, stderr names the untouched entry: %s", code, strings.TrimSpace(stderr))
}

// TestInstallHooksAppearsInHelp mirrors TestInstallSkillCommandAppearsInHelp.
func TestInstallHooksAppearsInHelp(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	a := withInstallHooks(newApp(&out, &errBuf))

	if code := a.main([]string{"--help"}); code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, errBuf.String())
	}
	if !strings.Contains(out.String(), installHooksName) {
		t.Errorf("help does not name %q:\n%s", installHooksName, out.String())
	}
	if !strings.Contains(out.String(), installHooksSummary) {
		t.Errorf("help does not carry the summary for %q:\n%s", installHooksName, out.String())
	}

	out.Reset()
	errBuf.Reset()
	if code := a.main([]string{"help", installHooksName}); code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, errBuf.String())
	}
	for _, want := range []string{"--local", "--dir", "--uninstall", string(installhooks.Installed), string(installhooks.Unchanged), string(installhooks.Removed)} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("`dira help %s` does not mention %q:\n%s", installHooksName, want, out.String())
		}
	}
	viaHelp := out.String()

	out.Reset()
	errBuf.Reset()
	if code := a.main([]string{installHooksName, "-h"}); code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, errBuf.String())
	}
	if out.String() != viaHelp {
		t.Errorf("`dira %s -h` and `dira help %s` render different text", installHooksName, installHooksName)
	}
}

func shaOf(t *testing.T, path string) [32]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return sha256.Sum256(data)
}
