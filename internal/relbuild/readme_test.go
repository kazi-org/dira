package relbuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"gopkg.in/yaml.v3"
)

// configWithDist returns a copy of the config at cfgPath with its dist:
// output redirected to distDir, so a snapshot run never touches the real
// dist/ directory another test (or a concurrent run) may be using.
func configWithDist(t *testing.T, cfgPath, distDir string) string {
	t.Helper()
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading %s: %v", cfgPath, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing %s as a yaml.Node: %v", cfgPath, err)
	}
	setMappingValue(doc.Content[0], "dist", distDir)
	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("marshalling config with dist override: %v", err)
	}
	tmp := filepath.Join(t.TempDir(), "goreleaser-dist-override.yaml")
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		t.Fatalf("writing config with dist override: %v", err)
	}
	return tmp
}

// goreleaserSnapshotReleaseCmd runs the FULL snapshot pipeline — build,
// archive, checksum — unlike E0-L4-T2's `build --snapshot --clean`, which
// stops before archiving. Real archive names only exist after this.
func goreleaserSnapshotReleaseCmd(t *testing.T, root, cfgPath string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(goTool(t), "run", "github.com/goreleaser/goreleaser/v2@"+goreleaserVersion,
		"release", "--snapshot", "--clean", "-f", cfgPath)
	cmd.Dir = root
	return cmd
}

// assetNamePattern builds the ONE regex both a real dist/ archive name and
// README.md's example command must match, for a given goos/goarch. It
// accepts either a real version string or the literal `${VERSION}` shell
// placeholder README.md uses — the same regex certifying both is the point:
// a README naming convention that only "looks right" to a human is not what
// this test proves.
func assetNamePattern(goos, goarch string) *regexp.Regexp {
	return regexp.MustCompile(
		`dira_(?:\$\{VERSION\}|[0-9][0-9A-Za-z.\-]*)_` + goos + `_` + goarch + `\.tar\.gz`)
}

// realArchiveNames runs goreleaser's full snapshot pipeline (build, archive,
// checksum — `build` alone, which E0-L4-T2 uses, never reaches the archive
// step) into an isolated temp dist/ and returns the two archive basenames it
// wrote, located by walking rather than a hardcoded path, same as
// snapshot_test.go's own binaries.
func realArchiveNames(t *testing.T, root, cfgPath string) (darwin, linux string) {
	t.Helper()
	dist := t.TempDir()
	mutatedCfg := configWithDist(t, cfgPath, dist)
	cmd := goreleaserSnapshotReleaseCmd(t, root, mutatedCfg)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("goreleaser release --snapshot --clean: %v\n%s", err, out)
	}

	darwinRe := assetNamePattern("darwin", "arm64")
	linuxRe := assetNamePattern("linux", "amd64")

	entries, err := os.ReadDir(dist)
	if err != nil {
		t.Fatalf("reading %s: %v", dist, err)
	}
	for _, e := range entries {
		switch {
		case darwinRe.MatchString(e.Name()):
			darwin = e.Name()
		case linuxRe.MatchString(e.Name()):
			linux = e.Name()
		}
	}
	if darwin == "" || linux == "" {
		t.Fatalf("did not find both archive names under %s (darwin=%q, linux=%q); entries: %v", dist, darwin, linux, entries)
	}
	return darwin, linux
}

// TestReadmeMatchesArchiveNaming is E0-L4-T5's acceptance line.
func TestReadmeMatchesArchiveNaming(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to goreleaser; skipped under -short")
	}
	root := moduleRoot(t)
	cfgPath := filepath.Join(root, ".goreleaser.yaml")

	darwinName, linuxName := realArchiveNames(t, root, cfgPath)
	t.Logf("real snapshot archive names: darwin=%s linux=%s", darwinName, linuxName)

	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}

	t.Run("the real archive names match the shared naming pattern", func(t *testing.T) {
		if err := validateArchiveNaming(darwinName, linuxName); err != nil {
			t.Fatalf("the actual snapshot archive names do not match this test's own pattern: %v", err)
		}
	})

	t.Run("README.md's curl/tar block matches the same pattern for both targets and names kazi-org/dira", func(t *testing.T) {
		if err := validateReadmeAssetNaming(string(readme)); err != nil {
			t.Error(err)
		}
	})

	t.Run("README.md carries a new heading for this section", func(t *testing.T) {
		if !regexp.MustCompile(`(?m)^## Install from a Release\s*$`).Match(readme) {
			t.Error(`README.md has no "## Install from a Release" heading`)
		}
	})

	t.Run("both sides: the naming assertion rejects kazi's bare-binary convention", func(t *testing.T) {
		kaziStyle := "```\ncurl -LO https://github.com/kazi-org/dira/releases/download/v0.0.1/dira_darwin_aarch64\nchmod +x dira_darwin_aarch64\n```\n"
		if err := validateReadmeAssetNaming(kaziStyle); err == nil {
			t.Fatal("expected validateReadmeAssetNaming to reject kazi's bare-binary naming convention, got nil")
		}
		// And the real, corrected README must still pass — checked in the
		// same run so the negative case has a positive baseline.
		if err := validateReadmeAssetNaming(string(readme)); err != nil {
			t.Fatalf("validateReadmeAssetNaming rejected the real README: %v", err)
		}
	})
}

func validateArchiveNaming(darwinName, linuxName string) error {
	if !assetNamePattern("darwin", "arm64").MatchString(darwinName) {
		return errf("darwin archive name %q does not match the expected pattern", darwinName)
	}
	if !assetNamePattern("linux", "amd64").MatchString(linuxName) {
		return errf("linux archive name %q does not match the expected pattern", linuxName)
	}
	return nil
}

func validateReadmeAssetNaming(readme string) error {
	if !assetNamePattern("darwin", "arm64").MatchString(readme) {
		return errf("README.md has no darwin/arm64 asset filename matching the goreleaser naming pattern (dira_<version>_darwin_arm64.tar.gz)")
	}
	if !assetNamePattern("linux", "amd64").MatchString(readme) {
		return errf("README.md has no linux/amd64 asset filename matching the goreleaser naming pattern (dira_<version>_linux_amd64.tar.gz)")
	}
	if !regexp.MustCompile(`kazi-org/dira`).MatchString(readme) {
		return errf("README.md's release section does not name kazi-org/dira")
	}
	return nil
}
