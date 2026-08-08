package nomodel

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// the enumerator
//
// It lives in the test file rather than in scan.go because
// internal/ledger/boundary_test.go enforces dec-0005 over every non-test package
// in this module: nothing above the storage backend may import `os`, `io/fs`,
// `path` or `path/filepath`. This package has no runtime consumer — its only
// caller is this file — so the policy stays pure in scan.go and the I/O stays
// here. See the "Why nothing here touches the filesystem" section of the package
// comment.
// ---------------------------------------------------------------------------

// moduleFiles lists the module's own files, relative to root and
// slash-separated.
//
// It asks git first, and asks for tracked AND untracked-but-not-ignored files.
// All three halves are load-bearing:
//
//   - tracked, because "the module's source" is what the module ships;
//   - untracked-not-ignored, because a check that only sees committed files
//     cannot fail the diff that introduces the problem — it would go red one
//     commit after the author needed it;
//   - and `--exclude-standard` rather than a hand-written ignore list, because
//     .gitignore already says which paths are derived (`dist/`, `node_modules`,
//     the mockup renders) and a second copy of that answer would drift.
//
// Whatever git returns is then filtered by nomodel.InModule.
//
// Where there is no git (a release tarball) it falls back to a filesystem walk
// applying the same filters.
func moduleFiles(root string) ([]string, error) {
	files, ok := gitFiles(root)
	if !ok {
		var err error
		if files, err = walkFiles(root); err != nil {
			return nil, err
		}
	}

	// Memoised so the go.mod stat is one syscall per directory rather than
	// one per file.
	nested := map[string]bool{}
	hasGoMod := func(dir string) bool {
		if seen, ok := nested[dir]; ok {
			return seen
		}
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir), "go.mod"))
		nested[dir] = err == nil
		return err == nil
	}

	out := files[:0]
	for _, rel := range files {
		if InModule(rel, hasGoMod) {
			out = append(out, rel)
		}
	}
	return out, nil
}

// gitFiles returns the tracked and untracked-not-ignored files under root, or
// ok=false where git cannot answer — no binary on PATH, or root is not inside a
// work tree.
func gitFiles(root string) (files []string, ok bool) {
	git, err := exec.LookPath("git")
	if err != nil {
		return nil, false
	}
	out, err := exec.Command(git, "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, false
	}
	seen := map[string]bool{}
	for _, name := range strings.Split(string(out), "\x00") {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		files = append(files, name)
	}
	// An empty listing is not an answer — it is git succeeding on a
	// directory with nothing in it. Fall back rather than report a clean
	// scan of nothing.
	if len(files) == 0 {
		return nil, false
	}
	return files, true
}

// walkFiles is the no-git fallback.
func walkFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			if rel == "." {
				return nil
			}
			for _, skip := range SkippedDirs {
				if d.Name() == skip.Path {
					return fs.SkipDir
				}
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			// A symlink out of the tree is somebody else's file, and
			// following one is how a walk leaves the module.
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// scanTree is the whole of check 2 wired together: enumerate, then apply the
// rule. Every caller below goes through it, so the red and green observations
// are made on the same path the real check uses.
func scanTree(root string) (violations []FileViolation, scanned int, err error) {
	files, err := moduleFiles(root)
	if err != nil {
		return nil, 0, err
	}
	return ScanFiles(files, func(rel string) ([]byte, error) {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrSkipFile
		}
		return content, err
	})
}

// commandPackage is the binary dec-0003 is about. Named by import path so the
// test does not care which directory `go test` chose to run it from.
const commandPackage = "github.com/kazi-org/dira/cmd/dira"

// A Target is one GOOS/GOARCH pair.
type Target struct{ GOOS, GOARCH string }

// BuildTargets is the matrix the linkage check runs over.
//
// docs/lore.md L-0008 is exactly this bug and this repository has already paid
// for it once: TestTheAllowlistIsNotStale asked `go list -deps` about the host
// GOOS only, so an allowlist exactly right on a Mac was stale on Linux, and
// CI's first ubuntu run caught it (commit d890bab). A dependency check that asks
// only the host is a report on whoever ran it.
//
// For a DENYLIST the union is strictly safer than any single platform: a model
// SDK pulled in behind a build tag for one GOOS is invisible from every other.
// So this is wider than the two pairs goreleaser publishes.
var BuildTargets = []Target{
	{GOOS: "darwin", GOARCH: "arm64"},
	{GOOS: "darwin", GOARCH: "amd64"},
	{GOOS: "linux", GOARCH: "amd64"},
	{GOOS: "linux", GOARCH: "arm64"},
}

// goTool locates the toolchain, skipping rather than failing where there is no
// `go` on PATH — the same contract cmd/dira/build_test.go uses.
func goTool(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go toolchain on PATH: %v", err)
	}
	return path
}

// moduleRoot is the directory holding go.mod, found from this file's own path
// rather than from the working directory.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate this test file")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file))) // internal/nomodel -> internal -> root
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("no go.mod at the computed module root %s: %v", root, err)
	}
	return root
}

// ---------------------------------------------------------------------------
// check 1 — the command path links no model SDK
// ---------------------------------------------------------------------------

// TestCommandPathLinksNoModelSDK is the real check: the untouched tree, across
// the whole target matrix, links nothing on the denylist.
//
// This is L-0001's green side, and it is the half normally left unobserved. The
// red side is TestDenyModuleFlagsModelSDKs below, which fires the same predicate
// at constructed defects of the class this claims to catch.
func TestCommandPathLinksNoModelSDK(t *testing.T) {
	t.Parallel()

	for _, target := range BuildTargets {
		t.Run(target.GOOS+"/"+target.GOARCH, func(t *testing.T) {
			t.Parallel()

			modules, packages := linkedFor(t, target)

			// Without these the test passes just as happily on a
			// broken `go list`, which is the vacuous green
			// docs/lore.md L-0001 rule 1 lists five instances of.
			if len(modules) == 0 {
				t.Fatalf("go list -deps reported no foreign modules for %s/%s; the check measured nothing", target.GOOS, target.GOARCH)
			}
			if len(packages) == 0 {
				t.Fatalf("go list -deps reported no packages for %s/%s; the check measured nothing", target.GOOS, target.GOARCH)
			}

			found, err := DeniedModules(modules)
			if err != nil {
				t.Fatalf("DeniedModules: %v", err)
			}
			for _, v := range found {
				t.Errorf("the dira command path links a model-provider SDK on %s/%s (dec-0003, cst-0004):\n%s\n"+
					"dec-0003 says the binary contains no model client: tier 2 is the live session, which already has "+
					"the transcript in context and needs no key. If this dependency is genuinely not a model client, "+
					"say so by narrowing ModelVendors in internal/nomodel/scan.go — with the reason in the diff.",
					target.GOOS, target.GOARCH, v)
			}
		})
	}
}

// TestStdlibNetworkingIsLinkedAndNotDenied pins the boundary this check must
// never cross, by asserting the fact that would make a stricter check wrong.
//
// `cmd/dira` links `net`, `net/http` and `crypto/tls` today, through
// internal/ui's read-only localhost server. A denylist phrased over network
// packages instead of over model-provider modules would therefore be RED on a
// correct binary — L-0001 rule 2 in its purest form, and the single most likely
// way for a well-meaning later change to break this package.
//
// So the fact is asserted rather than commented. If internal/ui ever stops
// linking net/http this test fails, and whoever is here then decides
// deliberately whether the denylist may tighten, instead of tightening it by
// accident against a binary that has since changed.
func TestStdlibNetworkingIsLinkedAndNotDenied(t *testing.T) {
	t.Parallel()

	_, packages := linkedFor(t, Target{}) // the host's, which is where the claim is checkable
	if len(packages) == 0 {
		t.Fatal("go list -deps reported no packages; the check measured nothing")
	}

	for _, pkg := range []string{"net", "net/http", "crypto/tls"} {
		if !slices.Contains(packages, pkg) {
			t.Errorf("cmd/dira no longer links %s.\n"+
				"That is not a failure by itself — but this test exists to record that it DID, "+
				"which is why internal/nomodel denies model-provider MODULES and not network packages. "+
				"Re-read the package comment before changing the denylist's scope.", pkg)
		}
		if reason, denied := DenyModule(pkg); denied {
			t.Errorf("DenyModule(%q) = %q, true — the denylist has grown to cover stdlib networking. "+
				"dec-0003 forbids a model client, not a socket; the socket question is E1-L6-T3's, "+
				"measured at runtime. A check phrased this way is red on a correct binary.", pkg, reason)
		}
	}
}

// linkedFor lists what cmd/dira links for a target. An empty Target means the
// host's. Returns foreign modules keyed by path with the packages that reach
// them, and the flat list of every linked package including stdlib.
func linkedFor(t *testing.T, target Target) (modules map[string][]string, packages []string) {
	t.Helper()

	cmd := exec.Command(goTool(t), "list", "-deps",
		"-f", `{{.ImportPath}}{{"\t"}}{{if .Standard}}std{{else if .Module}}{{.Module.Path}}{{end}}`,
		commandPackage)
	if target.GOOS != "" {
		cmd.Env = append(os.Environ(), "GOOS="+target.GOOS, "GOARCH="+target.GOARCH)
	}
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("go list -deps %s (GOOS=%s GOARCH=%s): %v\n%s", commandPackage, target.GOOS, target.GOARCH, err, exit.Stderr)
		}
		t.Fatalf("go list -deps %s (GOOS=%s GOARCH=%s): %v", commandPackage, target.GOOS, target.GOARCH, err)
	}

	const ownModule = "github.com/kazi-org/dira"
	modules = map[string][]string{}
	for _, line := range strings.Split(string(out), "\n") {
		pkg, module, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok || pkg == "" {
			continue
		}
		packages = append(packages, pkg)
		if module == "std" || module == ownModule {
			continue
		}
		if module == "" {
			// No module of its own; attribute it to its import path
			// so it is still visible to the denylist by name.
			module = pkg
		}
		modules[module] = append(modules[module], pkg)
	}
	return modules, packages
}

// TestDenyModuleFlagsModelSDKs is the red side. Each of these is the shape
// dec-0003's rejected alternative — "embed an LLM client so `dira sniff` can
// extract decisions semantically on its own" — would actually arrive in: as a
// `go get`. Every one must turn the check red.
func TestDenyModuleFlagsModelSDKs(t *testing.T) {
	t.Parallel()

	denied := []string{
		"github.com/anthropics/anthropic-sdk-go",
		"github.com/openai/openai-go",
		"github.com/sashabaranov/go-openai",
		"google.golang.org/genai",
		"github.com/google/generative-ai-go",
		"github.com/cohere-ai/cohere-go",
		"github.com/mistralai/client-go",
		"github.com/gage-technologies/mistral-go",
		"github.com/ollama/ollama",
		"github.com/aws/aws-sdk-go-v2/service/bedrockruntime",
		"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai",
	}
	for _, module := range denied {
		t.Run(module, func(t *testing.T) {
			t.Parallel()
			reason, ok := DenyModule(module)
			if !ok {
				t.Fatalf("DenyModule(%q) = false; a model-provider SDK walked past the denylist", module)
			}
			if reason == "" {
				t.Errorf("DenyModule(%q) denied it with an empty reason; the failure message would say nothing", module)
			}
		})
	}
}

// TestDeniedModulesIsRedOnALinkageFixture runs the red side through the same
// entry point the real check uses, not just through DenyModule — so the
// filtering, not only the predicate, is proven able to fail.
func TestDeniedModulesIsRedOnALinkageFixture(t *testing.T) {
	t.Parallel()

	got, err := DeniedModules(map[string][]string{
		"gopkg.in/yaml.v3":                       {"gopkg.in/yaml.v3"},
		"modernc.org/sqlite":                     {"modernc.org/sqlite"},
		"github.com/anthropics/anthropic-sdk-go": {"github.com/kazi-org/dira/internal/sniff"},
	})
	if err != nil {
		t.Fatalf("DeniedModules: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("DeniedModules found %d violations, want exactly 1: %v", len(got), got)
	}
	if got[0].Module != "github.com/anthropics/anthropic-sdk-go" {
		t.Errorf("violation names %q, want the anthropic SDK", got[0].Module)
	}
	// A gate whose message does not say how the dependency arrived makes the
	// reader repeat the investigation.
	if !strings.Contains(got[0].String(), "internal/sniff") {
		t.Errorf("violation message does not name the package that reaches it:\n%s", got[0])
	}
}

// TestDenyModulePassesInnocuousModules is the green side of the same predicate,
// on controlled input rather than on the live dependency tree.
//
// The first block is every foreign module cmd/dira links today, listed
// literally: if the denylist ever starts flagging one of these, this fails with
// the name rather than with a mysterious red on the real check. The second block
// is the near-misses a plain substring match would have caught — which is the
// reason DenyModule tokenises instead.
func TestDenyModulePassesInnocuousModules(t *testing.T) {
	t.Parallel()

	allowed := []string{
		// linked by cmd/dira today
		"gopkg.in/yaml.v3",
		"github.com/santhosh-tekuri/jsonschema/v6",
		"golang.org/x/text",
		"golang.org/x/exp",
		"golang.org/x/sys",
		"modernc.org/sqlite",
		"modernc.org/libc",
		"modernc.org/mathutil",
		"modernc.org/memory",
		"github.com/dustin/go-humanize",
		"github.com/google/uuid",
		"github.com/mattn/go-isatty",
		"github.com/ncruces/go-strftime",
		"github.com/remyoudompheng/bigfft",
		"github.com/kazi-org/dira",

		// near-misses: none of these is a model client
		"github.com/google/go-cmp",
		"cloud.google.com/go/storage",
		"github.com/aws/smithy-go",
		"github.com/coreos/go-systemd",
	}
	for _, module := range allowed {
		t.Run(module, func(t *testing.T) {
			t.Parallel()
			if reason, denied := DenyModule(module); denied {
				t.Errorf("DenyModule(%q) = %q, true; the denylist is flagging something that is not a model client", module, reason)
			}
		})
	}
}

// TestDeniedModulesRefusesAnEmptyInput is rule 1 stated as a test: a filter over
// zero modules reports zero violations and looks exactly like a clean binary.
func TestDeniedModulesRefusesAnEmptyInput(t *testing.T) {
	t.Parallel()

	if _, err := DeniedModules(nil); !errors.Is(err, ErrNothingScanned) {
		t.Errorf("DeniedModules(nil) err = %v, want ErrNothingScanned", err)
	}
	if _, err := DeniedModules(map[string][]string{}); !errors.Is(err, ErrNothingScanned) {
		t.Errorf("DeniedModules(empty) err = %v, want ErrNothingScanned", err)
	}
	if got, err := DeniedModules(map[string][]string{"gopkg.in/yaml.v3": {"gopkg.in/yaml.v3"}}); err != nil || len(got) != 0 {
		t.Errorf("DeniedModules(one clean module) = %v, %v; want no violations and no error", got, err)
	}
}

// ---------------------------------------------------------------------------
// check 2 — no API key name in the source
// ---------------------------------------------------------------------------

// TestModuleNamesNoAPIKeyVariable is the real check on the untouched module.
func TestModuleNamesNoAPIKeyVariable(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	violations, scanned, err := scanTree(root)
	if err != nil {
		t.Fatalf("scanTree(%s): %v", root, err)
	}

	// A floor rather than an exact count: the point is that the scanner read
	// the repository and not three files, and an exact number would be a
	// maintenance tax that says nothing extra.
	const floor = 100
	if scanned < floor {
		t.Fatalf("scanTree read only %d files under %s; expected at least %d. "+
			"A scan this small is a broken enumerator, not a clean repo.", scanned, root, floor)
	}

	for _, v := range violations {
		t.Errorf("%s (dec-0003, cst-0004).\n"+
			"dira holds no API key: the semantic tier is the live session, which is already "+
			"running with the transcript in context. If this file legitimately has to name the "+
			"variable — a redaction fixture, a document about this rule — add it to "+
			"APIKeyExclusions in internal/nomodel/scan.go with the reason.", v)
	}
}

// TestAPIKeyNamesFindsWhatItClaimsTo is the red side of check 2.
//
// The literals are written out in full rather than assembled from fragments.
// `_test.go` files are excluded from the scan structurally, so there is nothing
// to hide from — and the E8-L5-T7 lesson recorded in scan.go is that the
// splitting trick makes the rule ungreppable for the next reader without buying
// anything.
func TestAPIKeyNamesFindsWhatItClaimsTo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "the env lookup a model client cannot avoid",
			content: `key := os.Getenv("ANTHROPIC_API_KEY")`,
			want:    "ANTHROPIC_API_KEY",
		},
		{
			name:    "another provider, in a comment",
			content: "// fall back to OPENAI_API_KEY if unset",
			want:    "OPENAI_API_KEY",
		},
		{
			name:    "a bare name in a shell snippet",
			content: "export API_KEY=hunter2",
			want:    "API_KEY",
		},
		{
			name:    "a provider nobody has thought of yet",
			content: `cfg.Token = env["SOME_NEW_PROVIDER_API_KEY"]`,
			want:    "SOME_NEW_PROVIDER_API_KEY",
		},
		{
			name:    "a YAML key in a workflow file",
			content: "env:\n  MISTRAL_API_KEY: ${{ secrets.MISTRAL }}\n",
			want:    "MISTRAL_API_KEY",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := APIKeyNames(tc.content)
			if !slices.Contains(got, tc.want) {
				t.Errorf("APIKeyNames(%q) = %v, want it to contain %q", tc.content, got, tc.want)
			}
		})
	}
}

// TestAPIKeyNamesPassesCleanSource is the green side. A matcher that fires on
// everything is as useless as one that fires on nothing, and its failure is
// invisible from the red cases alone.
func TestAPIKeyNamesPassesCleanSource(t *testing.T) {
	t.Parallel()

	clean := []string{
		"",
		"package sniff\n\nfunc Stage(ctx context.Context) error { return nil }\n",
		"// The api key question is answered by dec-0003: there is no key.",
		"apiKey := cfg.Value // lowercase, and not an environment variable name",
		"KEY_API is not the same thing",
		"MY_API_KEYRING",
		"github.com/santhosh-tekuri/jsonschema/v6",
	}
	for _, content := range clean {
		if got := APIKeyNames(content); len(got) != 0 {
			t.Errorf("APIKeyNames(%q) = %v, want none", content, got)
		}
	}
}

// TestScanTreeSeesAPlantedKeyAndAnUntouchedTree runs both sides of the file
// scanner over a real directory, one step apart, so the green result is
// observed on the same enumerator that produced the red one.
//
// The temp tree has no git repository in it, so this also exercises the
// walkFiles fallback moduleFiles uses where git cannot answer.
func TestScanTreeSeesAPlantedKeyAndAnUntouchedTree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/x\n\ngo 1.26.2\n")
	write(t, root, "internal/deep/deep.go", "package deep\n\nfunc Handoff() string { return \"\" }\n")
	write(t, root, "internal/deep/deep_test.go", "package deep\n\n// this fixture names ANTHROPIC_API_KEY on purpose\n")
	write(t, root, "docs/design.md", "The binary holds no credentials.\n")

	// Green first, deliberately. Observing the clean tree before the defect
	// is what proves the later red came from the defect rather than from a
	// scanner that reports everything.
	violations, scanned, err := scanTree(root)
	if err != nil {
		t.Fatalf("scanTree on the clean tree: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("scanTree on the clean tree found %v, want none — note deep_test.go names a key and must be skipped", violations)
	}
	if scanned != 3 {
		t.Fatalf("scanTree read %d files, want 3 (go.mod, deep.go, design.md; deep_test.go skipped)", scanned)
	}

	// Now the defect: the fallback extractor dec-0003 exists to forbid.
	write(t, root, "internal/deep/client.go",
		"package deep\n\nimport \"os\"\n\nfunc key() string { return os.Getenv(\"ANTHROPIC_API_KEY\") }\n")

	violations, scanned, err = scanTree(root)
	if err != nil {
		t.Fatalf("scanTree on the defective tree: %v", err)
	}
	if scanned != 4 {
		t.Errorf("scanTree read %d files after planting one, want 4", scanned)
	}
	if len(violations) != 1 {
		t.Fatalf("scanTree found %d violations, want exactly 1: %v", len(violations), violations)
	}
	if violations[0].Path != "internal/deep/client.go" {
		t.Errorf("violation path = %q, want internal/deep/client.go", violations[0].Path)
	}
	if !strings.Contains(violations[0].String(), "ANTHROPIC_API_KEY") {
		t.Errorf("violation message does not name the variable it found:\n%s", violations[0])
	}
}

// TestScanTreeSkipsANestedModule covers the enumerator rule that matters most in
// this repository: agent worktrees are full checkouts nested under the module
// root, and scanning them would report another session's files as this module's.
//
// The nested directory here is deliberately NOT named `.claude`, so this
// exercises the go.mod rule rather than the SkippedDirs name list.
func TestScanTreeSkipsANestedModule(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/x\n\ngo 1.26.2\n")
	write(t, root, "main.go", "package main\n\nfunc main() {}\n")
	write(t, root, "worktrees/agent-1/go.mod", "module example.com/x\n\ngo 1.26.2\n")
	write(t, root, "worktrees/agent-1/leak.go", "package main\n\nvar k = \"ANTHROPIC_API_KEY\"\n")

	violations, scanned, err := scanTree(root)
	if err != nil {
		t.Fatalf("scanTree: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("scanTree descended into a nested module and reported %v", violations)
	}
	if scanned != 2 {
		t.Errorf("scanTree read %d files, want 2 (go.mod, main.go)", scanned)
	}
}

// TestScanTreeSkipsNamedDirectories is the other half of the enumerator's
// filtering, and it is checked with a planted key in each skipped directory so a
// skip that silently stopped working would show up.
func TestScanTreeSkipsNamedDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/x\n\ngo 1.26.2\n")
	write(t, root, "main.go", "package main\n\nfunc main() {}\n")
	for _, skip := range SkippedDirs {
		write(t, root, skip.Path+"/planted.txt", "ANTHROPIC_API_KEY=sk-not-real\n")
		if strings.TrimSpace(skip.Reason) == "" {
			t.Errorf("SkippedDirs entry %q carries no reason", skip.Path)
		}
	}

	violations, scanned, err := scanTree(root)
	if err != nil {
		t.Fatalf("scanTree: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("scanTree descended into a skipped directory and reported %v", violations)
	}
	if scanned != 2 {
		t.Errorf("scanTree read %d files, want 2 (go.mod, main.go)", scanned)
	}
}

// TestScanTreeRefusesAnEmptyTree is rule 1 for the file half: a scan of zero
// files finds zero keys and is indistinguishable from a clean repository.
func TestScanTreeRefusesAnEmptyTree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, _, err := scanTree(root); !errors.Is(err, ErrNothingScanned) {
		t.Errorf("scanTree on an empty directory err = %v, want ErrNothingScanned", err)
	}
}

// TestExclusionsAreNotStale keeps the named exclusion list honest.
//
// An exclusion list that accumulates entries nobody needs stops being a list of
// what is permitted and becomes a list of what was once convenient — and the
// next file to name a key slips in under a path that is already there. So every
// entry must still exist, and must still contain the thing it is excused for.
func TestExclusionsAreNotStale(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	if len(APIKeyExclusions) == 0 {
		t.Fatal("APIKeyExclusions is empty; this test would then assert nothing")
	}

	for _, e := range APIKeyExclusions {
		t.Run(e.Path, func(t *testing.T) {
			t.Parallel()

			if strings.TrimSpace(e.Reason) == "" {
				t.Errorf("exclusion %s carries no reason; an unexplained exemption is a hole with a comment", e.Path)
			}
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(e.Path)))
			if err != nil {
				t.Fatalf("excluded file is gone: %v. Remove the entry.", err)
			}
			if names := APIKeyNames(string(content)); len(names) == 0 {
				t.Errorf("%s is excluded from the API-key scan but no longer names one. "+
					"Remove the exclusion: an unused exemption is a hole waiting for something to walk through.", e.Path)
			}
		})
	}
}

// TestExcludedFilesAreReachedByTheEnumerator proves the exclusions are doing
// work rather than decorating a scan that would have passed anyway. If a listed
// file stopped being reachable, the list would look effective while protecting
// nothing — and the scan's clean result would not be evidence about that file.
func TestExcludedFilesAreReachedByTheEnumerator(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	files, err := moduleFiles(root)
	if err != nil {
		t.Fatalf("moduleFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("moduleFiles listed nothing; the check measured nothing")
	}
	for _, e := range APIKeyExclusions {
		if !slices.Contains(files, e.Path) {
			t.Errorf("%s is excluded from the API-key scan but the enumerator never reaches it, "+
				"so the exclusion protects nothing.", e.Path)
		}
	}
}

// TestModuleFilesSeesAnUncommittedFile is the property that makes this check
// useful at the moment it is needed. An enumerator over committed files only
// would go red one commit AFTER the author introduced the problem — it could not
// fail the diff that caused it.
func TestModuleFilesSeesAnUncommittedFile(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	files, err := moduleFiles(root)
	if err != nil {
		t.Fatalf("moduleFiles: %v", err)
	}
	// This file and its package's source are the two the enumerator must
	// reach for the rest of this suite to mean anything.
	for _, want := range []string{"internal/nomodel/scan.go", "internal/nomodel/nomodel_test.go"} {
		if !slices.Contains(files, want) {
			t.Errorf("moduleFiles did not list %s", want)
		}
	}
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}
