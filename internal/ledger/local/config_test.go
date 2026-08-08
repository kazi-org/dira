package local_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kazi-org/dira/internal/ledger/local"
)

// ReadConfig is the one path in dira that reads .dira/config.toml, and it lives
// on the backend because a path is the backend's business (dec-0005). What it
// has to get right is the difference between "there is no config file" — an
// ordinary state, since nothing in this release writes one — and "there is a
// config file and it could not be read", which is a real failure.

func TestReadConfigReturnsTheFile(t *testing.T) {
	t.Parallel()

	diraDir := t.TempDir()
	const body = "[brief]\nmax_tokens = 900\n"
	if err := os.WriteFile(filepath.Join(diraDir, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := local.ReadConfig(diraDir)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if string(got) != body {
		t.Errorf("ReadConfig = %q, want %q", got, body)
	}
}

func TestAMissingConfigIsNotAnError(t *testing.T) {
	t.Parallel()

	got, err := local.ReadConfig(t.TempDir())
	if err != nil {
		t.Fatalf("ReadConfig on a ledger with no config file: %v", err)
	}
	if got != nil {
		t.Errorf("ReadConfig = %q, want nil", got)
	}
}

// TestAnUnreadableConfigIsAnError is the other half, and it is what makes the
// case above mean something: absence is tolerated because it is ordinary, not
// because every failure is swallowed.
func TestAnUnreadableConfigIsAnError(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which can read a file with no permissions")
	}

	diraDir := t.TempDir()
	path := filepath.Join(diraDir, "config.toml")
	if err := os.WriteFile(path, []byte("[brief]\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	if _, err := local.ReadConfig(diraDir); err == nil {
		t.Error("a config file that could not be read came back as no config file")
	}
}
