// Package relbuild is a test-only package (no non-test .go file — see
// docs/plan/tasks/E0-L4.md's boundary note) that proves .goreleaser.yaml's
// shape and behaviour: E0-L4-T1 here, and E0-L4-T2 and T5 in
// snapshot_test.go and readme_test.go.
package relbuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// goreleaserVersion is the pinned goreleaser version this repo runs via
// `go run`, matching .github/workflows/release.yml's GORELEASER_VERSION.
// goreleaser is not vendored and nothing in this repo installs it — see
// docs/plan/tasks/E0-L4.md's "already known" section.
const goreleaserVersion = "v2.17.1"

// moduleRoot is the directory holding go.mod, found from this file's own
// path rather than from the working directory.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate this test file")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file))) // internal/relbuild -> internal -> root
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("no go.mod at the computed module root %s: %v", root, err)
	}
	return root
}

func goTool(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go toolchain on PATH: %v", err)
	}
	return path
}

// goreleaserCheck runs `goreleaser check -f <configPath>` and returns its
// exit code and combined output.
func goreleaserCheck(t *testing.T, configPath string) (exitCode int, output string) {
	t.Helper()
	cmd := exec.Command(goTool(t), "run", "github.com/goreleaser/goreleaser/v2@"+goreleaserVersion,
		"check", "-f", configPath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	var exitErr *exec.ExitError
	if ok := asExitError(err, &exitErr); ok {
		return exitErr.ExitCode(), string(out)
	}
	t.Fatalf("running goreleaser check: %v\n%s", err, out)
	return -1, string(out)
}

func asExitError(err error, target **exec.ExitError) bool {
	ee, ok := err.(*exec.ExitError)
	if ok {
		*target = ee
	}
	return ok
}

// --- the config's shape, just enough to assert the acceptance line ---

type goreleaserConfig struct {
	Builds   []buildTarget `yaml:"builds"`
	Archives []archive     `yaml:"archives"`
	Checksum checksum      `yaml:"checksum"`
}

type buildTarget struct {
	ID      string   `yaml:"id"`
	Env     []string `yaml:"env"`
	GOOS    []string `yaml:"goos"`
	GOARCH  []string `yaml:"goarch"`
	LDFlags []string `yaml:"ldflags"`
}

type archive struct {
	Formats []string `yaml:"formats"`
}

type checksum struct {
	NameTemplate string `yaml:"name_template"`
}

func parseGoreleaserConfig(t *testing.T, raw []byte) goreleaserConfig {
	t.Helper()
	var cfg goreleaserConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parsing .goreleaser.yaml: %v", err)
	}
	return cfg
}

// validateTwoTargets returns an error unless cfg declares exactly two builds
// entries whose (goos, goarch) pairs are (darwin, arm64) and (linux, amd64)
// and no others.
func validateTwoTargets(cfg goreleaserConfig) error {
	if len(cfg.Builds) != 2 {
		return errf("expected exactly 2 builds entries, got %d", len(cfg.Builds))
	}
	want := map[[2]string]bool{
		{"darwin", "arm64"}: false,
		{"linux", "amd64"}:  false,
	}
	for _, b := range cfg.Builds {
		if len(b.GOOS) != 1 || len(b.GOARCH) != 1 {
			return errf("build %q: expected exactly one goos and one goarch, got goos=%v goarch=%v", b.ID, b.GOOS, b.GOARCH)
		}
		pair := [2]string{b.GOOS[0], b.GOARCH[0]}
		if _, ok := want[pair]; !ok {
			return errf("build %q targets unexpected pair (%s, %s)", b.ID, pair[0], pair[1])
		}
		want[pair] = true
	}
	for pair, seen := range want {
		if !seen {
			return errf("no build entry targets (%s, %s)", pair[0], pair[1])
		}
	}
	return nil
}

func errf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// --- the real test ---

func TestGoreleaserConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to goreleaser; skipped under -short")
	}

	root := moduleRoot(t)
	cfgPath := filepath.Join(root, ".goreleaser.yaml")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading .goreleaser.yaml: %v", err)
	}
	cfg := parseGoreleaserConfig(t, raw)

	t.Run("goreleaser check passes on the real file", func(t *testing.T) {
		code, out := goreleaserCheck(t, cfgPath)
		if code != 0 {
			t.Fatalf("goreleaser check exited %d, want 0:\n%s", code, out)
		}
	})

	t.Run("both sides: goreleaser check fails on a copy with an unknown top-level key", func(t *testing.T) {
		mutated := append(append([]byte{}, raw...), []byte("\nnot_a_real_goreleaser_key: true\n")...)
		tmp := filepath.Join(t.TempDir(), "goreleaser-unknown-key.yaml")
		if err := os.WriteFile(tmp, mutated, 0o644); err != nil {
			t.Fatalf("writing mutated config: %v", err)
		}
		code, out := goreleaserCheck(t, tmp)
		if code == 0 {
			t.Fatalf("goreleaser check exited 0 on a config with an unknown top-level key, want non-zero:\n%s", out)
		}
	})

	t.Run("exactly two build targets: darwin/arm64 and linux/amd64", func(t *testing.T) {
		if err := validateTwoTargets(cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("both sides: two-target assertion fails on a copy with only one builds entry", func(t *testing.T) {
		// This copy is schema-valid (goreleaser check alone would accept a
		// single-target config), so it exercises this test's own count
		// check specifically, not goreleaser's.
		oneTarget := goreleaserConfig{Builds: cfg.Builds[:1]}
		if err := validateTwoTargets(oneTarget); err == nil {
			t.Fatal("expected validateTwoTargets to reject a config with only one builds entry, got nil")
		}
		// And the real, two-target config must still pass — proven in the
		// same run so this negative case is checked against a positive
		// baseline, not in isolation.
		if err := validateTwoTargets(cfg); err != nil {
			t.Fatalf("validateTwoTargets rejected the real two-target config: %v", err)
		}
	})

	t.Run("every build sets CGO_ENABLED=0 and the version ldflags", func(t *testing.T) {
		for _, b := range cfg.Builds {
			if !containsStr(b.Env, "CGO_ENABLED=0") {
				t.Errorf("build %q env = %v, want it to contain CGO_ENABLED=0", b.ID, b.Env)
			}
			if !anyContains(b.LDFlags, "-X main.version={{.Version}}") {
				t.Errorf("build %q ldflags = %v, want an entry containing \"-X main.version={{.Version}}\"", b.ID, b.LDFlags)
			}
		}
	})

	t.Run("archives declare tar.gz and checksum names a single file", func(t *testing.T) {
		if len(cfg.Archives) == 0 {
			t.Fatal("no archives entries")
		}
		for _, a := range cfg.Archives {
			if !containsStr(a.Formats, "tar.gz") {
				t.Errorf("archive formats = %v, want it to contain \"tar.gz\"", a.Formats)
			}
		}
		if cfg.Checksum.NameTemplate == "" {
			t.Fatal("checksum.name_template is empty")
		}
		if matched, _ := regexp.MatchString(`\{\{.*\.Os.*\}\}|\{\{.*\.Arch.*\}\}`, cfg.Checksum.NameTemplate); matched {
			t.Errorf("checksum.name_template = %q looks per-archive (contains an OS/Arch template variable), want a single shared file", cfg.Checksum.NameTemplate)
		}
	})
}

func containsStr(list []string, want string) bool {
	return slices.Contains(list, want)
}

func anyContains(list []string, substr string) bool {
	return slices.ContainsFunc(list, func(s string) bool { return strings.Contains(s, substr) })
}
