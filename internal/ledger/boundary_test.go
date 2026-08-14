package ledger_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// modulePrefix is this module's import path. Only packages under it are
// examined: the rule is about what dira's own code knows, not about what its
// dependencies do.
const modulePrefix = "github.com/kazi-org/dira/"

// filesystemPackages are the stdlib packages that can name a path. A package
// that imports one of these can reach `.dira/` directly, which is exactly what
// dec-0005 exists to prevent above the storage interface.
var filesystemPackages = []string{"os", "io/fs", "io/ioutil", "path", "path/filepath"}

// allowed lists the packages that may import them, and which ones.
//
// This is the mechanical form of dec-0005. The github backend arriving in E7
// implements ledger.Store over an HTTP API, where there is no path, no
// *os.File and no fs.FS. Any filesystem concept that leaks above the interface —
// a method taking a path, a caller globbing a directory, a query walking a tree
// — is a change E7 would have to make above the interface, which is precisely
// the retrofit the decision was written to avoid. Nothing but a backend gets to
// know what a path is.
//
// The allowlist is deliberately small and each entry states its reason. Adding
// one is a design decision, and it should read like one in the diff.
var allowed = map[string][]string{
	// The filesystem backend. This is the point of the exercise.
	"internal/ledger/local": filesystemPackages,

	// The command itself, for os.Stdout, os.Stderr, os.Args and os.Exit, and
	// — since E2-L7 — path/filepath. `dira import <dir>` is the first command
	// in this repository that reads a directory the *user* names rather than
	// one dira owns (dec-0005 governs a ledger's own storage, not an
	// arbitrary corpus a human points the importer at), and walking it needs
	// to join a directory name with the entries os.ReadDir returns.
	// internal/importadr itself still imports neither: every function in it
	// takes bytes or another function's structured output, and cmd/dira is
	// the only thing in that lane that ever names a path.
	"cmd/dira": {"os", "path/filepath"},

	// The derived read cache (E1-L3). It is not a second ledger backend and
	// it never reads an entry: every entry it holds arrives through
	// ledger.Store, and every entry it renders is fetched back through
	// ledger.Store. What it does own is a SQLite database file under
	// .dira/cache/, and a database has a path whatever the ledger is stored
	// in — E7's github backend still caches locally, because the alternative
	// is a network call on the read path, which int-0002 forbids outright.
	// So this exemption covers the cache's own file and nothing else, and
	// path/filepath is on it only to name that file inside a directory the
	// caller supplied.
	"internal/index": {"os", "path/filepath"},

	// The differential harness for the cache, which has to be able to delete
	// .dira/cache/ before a query to prove the answer does not change.
	// Deleting the cache is the thing under test.
	"internal/index/indextest": {"os", "path/filepath"},

	// Locates the skill document this repository ships, for the two test
	// packages that check it from two different directories. Same shape as
	// indextest: a test harness, imported by nothing in the shipped binary.
	//
	// It exists so internal/skill itself stays pure policy — it takes a
	// document's TEXT and returns what it found — which is what let E2-L2-T8
	// give the installer a two-method Root port instead of a filesystem, and
	// is the same restructuring internal/nomodel made for the same rule.
	// Reading a file is a filesystem concern; parsing its contents is not.
	"internal/skill/skilltest": {"os", "path/filepath"},

	// The design-mockup ground-truth probe (E6-L3-T1). It renders one served
	// page over the fixture ledger and prints the HTML to stdout, so
	// docs/design/scripts/fixture-check.mjs can diff a mockup against the
	// real render instead of a hand-typed copy of what the template should
	// produce. A throwaway harness under docs/design/, in the same class as
	// indextest and skilltest above: it needs a temp directory for the read
	// cache and os.Stdout to print to, and it ships in no binary dira builds.
	"docs/design/scripts/lib/renderprobe": {"os"},
}

// TestNoFilesystemImportsAboveTheBackend is E1-L1's acceptance line (c).
func TestNoFilesystemImportsAboveTheBackend(t *testing.T) {
	t.Parallel()

	imports := moduleImports(t)

	// Without this the test passes just as happily on an empty listing.
	if len(imports) < 3 {
		t.Fatalf("go list reported %d packages in this module; the check is not measuring anything", len(imports))
	}
	if _, ok := imports["internal/ledger"]; !ok {
		t.Fatal("internal/ledger is not in the listing; the package this rule exists to constrain was not examined")
	}

	for pkg, list := range imports {
		for _, imported := range list {
			if !slices.Contains(filesystemPackages, imported) {
				continue
			}
			if slices.Contains(allowed[pkg], imported) {
				continue
			}
			t.Errorf("%s imports %q.\n"+
				"Only a storage backend may name a path (dec-0005): the github backend in E7 has no filesystem, "+
				"so anything above ledger.Store that reaches for one is a change E7 would have to make above the interface.\n"+
				"Route the access through ledger.Store, or — if this really is a new backend — add it to the allowlist in %s.",
				pkg, imported, "internal/ledger/boundary_test.go")
		}
	}
}

// TestTheImportBoundaryHasTeeth is the other half. A rule that forbids something
// nobody does is indistinguishable from a rule that is broken, so this asserts
// the allowlisted backend really does use the imports the rule bans everywhere
// else, and that every allowlist entry names a package that exists.
func TestTheImportBoundaryHasTeeth(t *testing.T) {
	t.Parallel()

	imports := moduleImports(t)

	backend := imports["internal/ledger/local"]
	if backend == nil {
		t.Fatal("internal/ledger/local is not in the listing; the backend the allowlist exempts does not exist")
	}
	for _, want := range []string{"os", "path/filepath"} {
		if !slices.Contains(backend, want) {
			t.Errorf("internal/ledger/local does not import %q; either the backend stopped touching the filesystem "+
				"or the listing is wrong, and in both cases this rule is measuring nothing", want)
		}
	}

	for pkg := range allowed {
		if _, ok := imports[pkg]; !ok {
			t.Errorf("the allowlist exempts %s, which is not a package in this module; a stale exemption is a hole", pkg)
		}
	}
}

// moduleImports returns each package in this module and the packages it imports
// directly, keyed by path relative to the module.
//
// Direct imports, not the transitive closure, and non-test files only. The rule
// is about what dira's own code reaches for: yaml.v3 imports os somewhere in its
// own tree and that is not dira's business, and a test that shells out to `go
// list` is itself allowed to touch the filesystem.
func moduleImports(t *testing.T) map[string][]string {
	t.Helper()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go toolchain on PATH: %v", err)
	}

	out, err := exec.Command(goBin, "list",
		"-f", `{{.ImportPath}}{{"\t"}}{{join .Imports " "}}`,
		modulePrefix+"...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}

	imports := map[string][]string{}
	for _, line := range strings.Split(string(out), "\n") {
		path, joined, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		pkg := strings.TrimPrefix(path, modulePrefix)
		if pkg == strings.TrimSuffix(modulePrefix, "/") {
			pkg = "."
		}
		imports[pkg] = strings.Fields(joined)
	}
	return imports
}
