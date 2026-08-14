package importadr

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/kazi-org/dira/internal/frontmatter"
)

// E2-L7-T1's acceptance line: the two named corpora and the seven controls are
// vendored, pinned, and checksummed — before the extractor exists to define its
// own bar. See docs/plan/tasks/E2-L7.md's T1 section.

// corpusManifestRowRe matches one row of a corpus MANIFEST.md's table:
// `| file | repo-relative path | sha256 |`.
var corpusManifestRowRe = regexp.MustCompile(`^\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*([0-9a-f]{64})\s*\|$`)

// manifestEntry is one row of a corpus's MANIFEST.md.
type manifestEntry struct {
	file   string
	sha256 string
}

// parseManifestData extracts a corpus MANIFEST.md's table rows straight from
// its bytes. It tolerates the prose header above the table — only lines
// shaped like a table row carrying a 64-character hex digest count.
func parseManifestData(data []byte) []manifestEntry {
	var entries []manifestEntry
	for _, line := range strings.Split(string(data), "\n") {
		m := corpusManifestRowRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		entries = append(entries, manifestEntry{file: m[1], sha256: m[3]})
	}
	return entries
}

// corpusProblems is the whole of TestFixtureCorpora's check, as a pure
// function over a directory: every problem it can report, named, rather than
// a boolean. It is used both to grade the real vendored corpus and — with a
// deliberately corrupted copy — to prove this check can fail at all
// (TestFixtureCorporaCatchesDrift), the "both sides" E2-L7-T1's acc demands.
func corpusProblems(dir string, want int) []string {
	var problems []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{fmt.Sprintf("reading %s: %v", dir, err)}
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == "MANIFEST.md" || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		files = append(files, e.Name())
	}
	if len(files) != want {
		problems = append(problems, fmt.Sprintf("%d vendored .md files (MANIFEST.md excluded), want exactly %d", len(files), want))
	}
	if len(files) == 0 {
		// An empty corpus satisfies every other check here vacuously
		// (docs/lore.md L-0001) — stop rather than report a false "all
		// sha256 matched".
		return append(problems, "no vendored files at all")
	}

	manifestData, err := os.ReadFile(filepath.Join(dir, "MANIFEST.md"))
	if err != nil {
		return append(problems, fmt.Sprintf("reading MANIFEST.md: %v", err))
	}
	manifest := parseManifestData(manifestData)
	if len(manifest) == 0 {
		return append(problems, "MANIFEST.md carries no parseable rows")
	}

	byFile := make(map[string]manifestEntry, len(manifest))
	for _, m := range manifest {
		byFile[m.file] = m
	}

	for _, f := range files {
		entry, ok := byFile[f]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s is vendored but has no MANIFEST.md row", f))
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			problems = append(problems, fmt.Sprintf("reading %s: %v", f, err))
			continue
		}
		sum := sha256.Sum256(data)
		got := hex.EncodeToString(sum[:])
		if got != entry.sha256 {
			problems = append(problems, fmt.Sprintf(
				"%s sha256 %s does not match MANIFEST.md's recorded %s — a hand-edited fixture invalidates "+
					"every literal number this lane pins downstream", f, got, entry.sha256))
		}
	}

	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f] = true
	}
	for _, m := range manifest {
		if !fileSet[m.file] {
			problems = append(problems, fmt.Sprintf("MANIFEST.md names %s, which is not a vendored file", m.file))
		}
	}

	return problems
}

// TestFixtureCorpora is E2-L7-T1's acceptance line.
func TestFixtureCorpora(t *testing.T) {
	for name, want := range map[string]int{"bbc-tams": 49, "nulib-meadow": 31} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", "corpora", name)
			for _, p := range corpusProblems(dir, want) {
				t.Error(p)
			}
		})
	}
}

// TestFixtureCorporaCatchesDrift is the red side of E2-L7-T1's "both sides"
// requirement: a control naming an expected answer with no matching file, and
// a fixture with no control entry, both turn corpusProblems red, naming the
// file. It works against a copy of bbc-tams, tampered two ways, so the real
// fixture directory is never touched by a test that is allowed to fail.
func TestFixtureCorporaCatchesDrift(t *testing.T) {
	src := filepath.Join("testdata", "corpora", "bbc-tams")

	t.Run("file with no manifest row", func(t *testing.T) {
		dir := copyCorpus(t, src)
		if err := os.WriteFile(filepath.Join(dir, "9999-unmanifested.md"), []byte("# stray\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		problems := corpusProblems(dir, 49)
		if !anyContains(problems, "9999-unmanifested.md") {
			t.Fatalf("expected a problem naming the unmanifested file, got: %v", problems)
		}
	})

	t.Run("manifest row with no matching file", func(t *testing.T) {
		dir := copyCorpus(t, src)
		files, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if f.Name() != "MANIFEST.md" && strings.HasSuffix(f.Name(), ".md") {
				if err := os.Remove(filepath.Join(dir, f.Name())); err != nil {
					t.Fatal(err)
				}
				break
			}
		}
		problems := corpusProblems(dir, 49)
		if len(problems) == 0 {
			t.Fatal("expected a problem naming the manifest row with no matching file, got none")
		}
	})

	t.Run("edited file", func(t *testing.T) {
		dir := copyCorpus(t, src)
		files, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		var target string
		for _, f := range files {
			if f.Name() != "MANIFEST.md" && strings.HasSuffix(f.Name(), ".md") {
				target = f.Name()
				break
			}
		}
		if err := os.WriteFile(filepath.Join(dir, target), []byte("# hand-edited\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		problems := corpusProblems(dir, 49)
		if !anyContains(problems, target) {
			t.Fatalf("expected a problem naming the edited file %s, got: %v", target, problems)
		}
	})
}

// copyCorpus copies src into a fresh temp directory and returns it, so a
// destructive test never touches the real vendored fixtures.
func copyCorpus(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

func anyContains(problems []string, substr string) bool {
	for _, p := range problems {
		if strings.Contains(p, substr) {
			return true
		}
	}
	return false
}

// controlFrontmatter is a control fixture's expected-answer data field, read
// by TestFixtureControls and reused by TestExtract (T2) as the table this
// lane's extractor is graded against — one source of the expected numbers,
// not two copies that can drift apart.
type controlFrontmatter struct {
	Control              string `yaml:"control"`
	Bare                 int    `yaml:"bare"`
	Thin                 int    `yaml:"thin"`
	Reasoned             int    `yaml:"reasoned"`
	Revisit              int    `yaml:"revisit"`
	LabelExpansionNeeded bool   `yaml:"label_expansion_needed"`
	MedianLabelWords     int    `yaml:"median_label_words"`
}

// wantControls is the seven control fixtures E2-L7-T1 requires, named exactly.
var wantControls = []string{
	"0001-bare-names",
	"0002-thin-reasons",
	"0003-rich-with-revisit",
	"0004-no-section",
	"c5-madr-reasons-elsewhere",
	"c6-madr-no-reasons",
	"c7-terse-labels",
}

// loadControl reads one control fixture's frontmatter and body.
func loadControl(t *testing.T, name string) (controlFrontmatter, string) {
	t.Helper()
	path := filepath.Join("testdata", "controls", name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	front, body, err := frontmatter.Split(data)
	if err != nil {
		t.Fatalf("%s carries no YAML frontmatter: %v", path, err)
	}
	var fm controlFrontmatter
	if err := yaml.Unmarshal(front, &fm); err != nil {
		t.Fatalf("%s: unparseable frontmatter: %v", path, err)
	}
	if fm.Control != name {
		t.Errorf("%s: frontmatter control field is %q, want %q", path, fm.Control, name)
	}
	return fm, string(body)
}

// controlSetProblems compares the .md files actually present in dir against
// want, naming every mismatch in both directions: a file present but not
// named in want, and a name in want with no file present.
func controlSetProblems(dir string, want []string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{fmt.Sprintf("reading %s: %v", dir, err)}
	}
	present := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		present[strings.TrimSuffix(e.Name(), ".md")] = true
	}
	wantSet := make(map[string]bool, len(want))
	for _, name := range want {
		wantSet[name] = true
	}

	var problems []string
	for name := range present {
		if !wantSet[name] {
			problems = append(problems, fmt.Sprintf("%s.md is present but not one of the named controls", name))
		}
	}
	for _, name := range want {
		if !present[name] {
			problems = append(problems, fmt.Sprintf("%s.md is a named control but not present", name))
		}
	}
	return problems
}

// TestFixtureControls is the controls half of E2-L7-T1's acceptance line: the
// directory holds exactly the seven named fixtures, each carrying its expected
// answer as parseable data, and the test fails if the directory holds fewer
// than seven — an empty (or partial) control set would satisfy every "every
// control …" clause downstream vacuously (docs/lore.md L-0001).
func TestFixtureControls(t *testing.T) {
	dir := "testdata/controls"
	for _, p := range controlSetProblems(dir, wantControls) {
		t.Error(p)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) != len(wantControls) {
		t.Fatalf("%s holds %d files, want exactly %d", dir, len(entries), len(wantControls))
	}

	for _, name := range wantControls {
		t.Run(name, func(t *testing.T) {
			fm, body := loadControl(t, name)
			if strings.TrimSpace(body) == "" {
				t.Fatalf("%s carries no body — a control with no document text passes every extraction "+
					"assertion vacuously", name)
			}
			// The frontmatter itself is the acceptance surface here: every
			// field must be present as data. bare/thin/reasoned/revisit are
			// ints with a natural zero value, so their presence is checked
			// by requiring at least one of the four counts to be non-zero (a
			// control naming zero of everything would be indistinguishable
			// from a control whose frontmatter was never filled in) except
			// 0004-no-section, whose own point is that every count is zero.
			if name != "0004-no-section" {
				if fm.Bare+fm.Thin+fm.Reasoned+fm.Revisit == 0 {
					t.Errorf("%s: frontmatter carries no non-zero bare/thin/reasoned/revisit count", name)
				}
			}
		})
	}
}

// TestFixtureControlsCatchesMismatch is the red side of the controls'
// acceptance line: a control file named outside wantControls, or wantControls
// naming a file that is not there, must turn controlSetProblems red, naming
// which.
func TestFixtureControlsCatchesMismatch(t *testing.T) {
	seed := func(t *testing.T, dir string, names []string) {
		t.Helper()
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(dir, name+".md"),
				[]byte("---\ncontrol: "+name+"\nbare: 1\n---\nx\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("file present but not named", func(t *testing.T) {
		dir := t.TempDir()
		seed(t, dir, wantControls)
		seed(t, dir, []string{"zz-not-a-real-control"})
		problems := controlSetProblems(dir, wantControls)
		if !anyContains(problems, "zz-not-a-real-control") {
			t.Fatalf("expected a problem naming the stray file, got: %v", problems)
		}
	})

	t.Run("named control not present", func(t *testing.T) {
		dir := t.TempDir()
		seed(t, dir, wantControls[1:]) // drop the first
		problems := controlSetProblems(dir, wantControls)
		if !anyContains(problems, wantControls[0]) {
			t.Fatalf("expected a problem naming the missing control %s, got: %v", wantControls[0], problems)
		}
	})

	t.Run("the real set has no problems", func(t *testing.T) {
		problems := controlSetProblems("testdata/controls", wantControls)
		if len(problems) != 0 {
			t.Fatalf("the real control set reported problems: %v", problems)
		}
	})
}
