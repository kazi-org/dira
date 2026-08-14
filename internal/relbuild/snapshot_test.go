package relbuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// runSnapshotBuild runs `goreleaser build --snapshot --clean -f configPath`
// with its working directory at root, so `./cmd/dira` resolves against the
// real module regardless of where configPath's `dist:` output lands.
func runSnapshotBuild(t *testing.T, root, configPath string) {
	t.Helper()
	cmd := exec.Command(goTool(t), "run", "github.com/goreleaser/goreleaser/v2@"+goreleaserVersion,
		"build", "--snapshot", "--clean", "-f", configPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goreleaser build --snapshot --clean: %v\n%s", err, out)
	}
}

// goBuildBinary is one file under dist/ that go version -m recognises as a
// Go binary, plus the GOOS/GOARCH/CGO_ENABLED build settings embedded in it.
type goBuildBinary struct {
	Path       string
	GOOS       string
	GOARCH     string
	CGOEnabled string // "0", "1", or "" if the setting is absent
}

// buildInfoBinaries walks dist looking for files go version -m can read —
// this is what "located by walking dist/ rather than a hardcoded path"
// means: goreleaser's per-target subdirectory naming is its own
// implementation detail, not something this test hardcodes.
func buildInfoBinaries(t *testing.T, goBin, dist string) []goBuildBinary {
	t.Helper()
	var found []goBuildBinary
	err := filepath.WalkDir(dist, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		out, verr := exec.Command(goBin, "version", "-m", path).Output()
		if verr != nil {
			return nil // not a Go binary (an archive, checksums.txt, …) — skip
		}
		b := goBuildBinary{Path: path}
		for line := range strings.SplitSeq(string(out), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "build\t") {
				continue
			}
			field := strings.TrimPrefix(line, "build\t")
			key, val, ok := strings.Cut(field, "=")
			if !ok {
				continue
			}
			switch key {
			case "GOOS":
				b.GOOS = val
			case "GOARCH":
				b.GOARCH = val
			case "CGO_ENABLED":
				b.CGOEnabled = val
			}
		}
		if b.GOOS != "" && b.GOARCH != "" {
			found = append(found, b)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dist, err)
	}
	return found
}

func binaryFor(t *testing.T, bins []goBuildBinary, goos, goarch string) goBuildBinary {
	t.Helper()
	var matches []goBuildBinary
	for _, b := range bins {
		if b.GOOS == goos && b.GOARCH == goarch {
			matches = append(matches, b)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one %s/%s binary under dist/, found %d: %+v", goos, goarch, len(matches), matches)
	}
	return matches[0]
}

// TestSnapshotBuildProducesBothTargets is E0-L4-T2's acceptance line.
func TestSnapshotBuildProducesBothTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to goreleaser and go build; skipped under -short")
	}
	goBin := goTool(t)
	root := moduleRoot(t)
	cfgPath := filepath.Join(root, ".goreleaser.yaml")
	distDir := filepath.Join(root, "dist")

	// Leave dist/ as it was found: goreleaser's default output directory,
	// gitignored per .gitignore lines 13-14, and this test both writes and
	// deletes it as part of proving idempotence.
	t.Cleanup(func() { _ = os.RemoveAll(distDir) })

	runSnapshotBuild(t, root, cfgPath)
	bins := buildInfoBinaries(t, goBin, distDir)

	darwinBin := binaryFor(t, bins, "darwin", "arm64")
	linuxBin := binaryFor(t, bins, "linux", "amd64")

	t.Run("CGO_ENABLED=0 is baked into both binaries", func(t *testing.T) {
		if darwinBin.CGOEnabled != "0" {
			t.Errorf("darwin/arm64 binary %s: CGO_ENABLED=%q, want \"0\"", darwinBin.Path, darwinBin.CGOEnabled)
		}
		if linuxBin.CGOEnabled != "0" {
			t.Errorf("linux/amd64 binary %s: CGO_ENABLED=%q, want \"0\"", linuxBin.Path, linuxBin.CGOEnabled)
		}
	})

	t.Run("idempotent: deleting dist/ and rebuilding produces the same two-file set", func(t *testing.T) {
		if err := os.RemoveAll(distDir); err != nil {
			t.Fatalf("removing dist/: %v", err)
		}
		runSnapshotBuild(t, root, cfgPath)
		rebuilt := buildInfoBinaries(t, goBin, distDir)
		darwinAgain := binaryFor(t, rebuilt, "darwin", "arm64")
		linuxAgain := binaryFor(t, rebuilt, "linux", "amd64")
		if filepath.Base(darwinAgain.Path) != filepath.Base(darwinBin.Path) {
			t.Errorf("darwin/arm64 binary filename changed across a clean rebuild: %q vs %q", darwinBin.Path, darwinAgain.Path)
		}
		if filepath.Base(linuxAgain.Path) != filepath.Base(linuxBin.Path) {
			t.Errorf("linux/amd64 binary filename changed across a clean rebuild: %q vs %q", linuxBin.Path, linuxAgain.Path)
		}
	})

	t.Run("both sides: CGO_ENABLED=1 on darwin only shows up on darwin's binary alone", func(t *testing.T) {
		mutatedCfgPath, mutatedDist := darwinCGOEnabledConfig(t, cfgPath)
		t.Cleanup(func() { _ = os.RemoveAll(mutatedDist) })
		runSnapshotBuild(t, root, mutatedCfgPath)
		mutatedBins := buildInfoBinaries(t, goBin, mutatedDist)
		mutatedDarwin := binaryFor(t, mutatedBins, "darwin", "arm64")
		mutatedLinux := binaryFor(t, mutatedBins, "linux", "amd64")
		if mutatedDarwin.CGOEnabled == "0" {
			t.Fatalf("expected the mutated config's darwin/arm64 binary to NOT report CGO_ENABLED=0 (config set it to 1), got %q — the assertion cannot fail", mutatedDarwin.CGOEnabled)
		}
		if mutatedLinux.CGOEnabled != "0" {
			t.Errorf("mutated config's linux/amd64 binary: CGO_ENABLED=%q, want \"0\" (only darwin's build was mutated)", mutatedLinux.CGOEnabled)
		}
	})
}

// darwinCGOEnabledConfig writes a copy of the real .goreleaser.yaml with the
// darwin/arm64 build's CGO_ENABLED set to 1, and its dist: output pointed at
// a fresh temp directory so it never touches the real dist/. Returns the
// copy's path and its dist directory.
func darwinCGOEnabledConfig(t *testing.T, realCfgPath string) (cfgPath, distDir string) {
	t.Helper()
	raw, err := os.ReadFile(realCfgPath)
	if err != nil {
		t.Fatalf("reading %s: %v", realCfgPath, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing .goreleaser.yaml as a yaml.Node: %v", err)
	}
	rootMap := doc.Content[0]

	buildsNode := mappingValueOrNilConfig(rootMap, "builds")
	if buildsNode == nil || buildsNode.Kind != yaml.SequenceNode {
		t.Fatal("builds: is missing or not a sequence")
	}

	mutated := false
	for _, buildNode := range buildsNode.Content {
		goosNode := mappingValueOrNilConfig(buildNode, "goos")
		if goosNode == nil || len(goosNode.Content) != 1 || goosNode.Content[0].Value != "darwin" {
			continue
		}
		envNode := mappingValueOrNilConfig(buildNode, "env")
		if envNode == nil {
			t.Fatal("darwin build has no env: list")
		}
		for _, envEntry := range envNode.Content {
			if envEntry.Value == "CGO_ENABLED=0" {
				envEntry.Value = "CGO_ENABLED=1"
				mutated = true
			}
		}
	}
	if !mutated {
		t.Fatal("did not find CGO_ENABLED=0 in the darwin build's env: list to mutate")
	}

	distDir = t.TempDir()
	setMappingValue(rootMap, "dist", distDir)

	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("marshalling mutated config: %v", err)
	}
	cfgPath = filepath.Join(t.TempDir(), "goreleaser-darwin-cgo1.yaml")
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		t.Fatalf("writing mutated config: %v", err)
	}
	return cfgPath, distDir
}

func mappingValueOrNilConfig(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// setMappingValue sets (or adds) a scalar string key on a mapping node.
func setMappingValue(m *yaml.Node, key, value string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1].Value = value
			m.Content[i+1].Tag = "!!str"
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: value, Tag: "!!str"}
	m.Content = append(m.Content, keyNode, valNode)
}
