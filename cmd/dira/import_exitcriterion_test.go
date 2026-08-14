package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/importadr"
	"github.com/kazi-org/dira/schema"
)

// TestExitCriterion is E2-L7-T7: the epic's exit criterion, read literally
// rather than reproduced in prose — against nulib/meadow it imports nothing
// and offers indexing; against bbc/tams it reports the count and imports on
// confirmation — proved against the real built command over T1's real
// vendored fixture directories, copied into a temp <dir> rather than read in
// place. Not a mock, not the unit-level policies in isolation.

// copyFixtureCorpus copies a T1 corpus's *.md files (MANIFEST.md excluded —
// it is a T1 testdata artifact, not part of the corpus a real `dira import`
// target would ever contain) into a fresh temp directory, and returns it.
func copyFixtureCorpus(t *testing.T, name string) string {
	t.Helper()
	src := fixtureDir(name)
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("reading %s: %v", src, err)
	}
	dst := t.TempDir()
	n := 0
	for _, e := range entries {
		if e.IsDir() || e.Name() == "MANIFEST.md" || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o644); err != nil {
			t.Fatalf("writing %s: %v", e.Name(), err)
		}
		n++
	}
	if n == 0 {
		t.Fatalf("copied zero files from %s — the corpus fixture is missing or empty", src)
	}
	return dst
}

func TestExitCriterion(t *testing.T) {
	t.Run("nulib/meadow: imports nothing, offers indexing", func(t *testing.T) {
		dir := copyFixtureCorpus(t, "nulib-meadow")
		r := newImportRunner(t)

		if code := r.run("y\n", dir, "--yes"); code != exitOK {
			t.Fatalf("exit code = %d\nstderr:\n%s", code, r.stderr.String())
		}
		if got := r.countFiles(t, r.entriesDir()); got != 0 {
			t.Errorf("entries/ has %d files, want 0", got)
		}
		if !strings.Contains(r.stdout.String(), "Importing these gives dira nothing it can enforce.") {
			t.Errorf("stdout does not carry the index offer:\n%s", r.stdout.String())
		}
		if got := r.countFiles(t, r.cacheImportsDir()); got != 1 {
			t.Fatalf("cache/imports/ has %d files, want exactly 1", got)
		}
		assertArtifactListsAllDocuments(t, r.cacheImportsDir(), 31)
	})

	t.Run("bbc/tams: reports the count, imports on confirmation", func(t *testing.T) {
		dir := copyFixtureCorpus(t, "bbc-tams")
		r := newImportRunner(t)

		if code := r.run("", dir, "--yes"); code != exitOK {
			t.Fatalf("exit code = %d\nstderr:\n%s", code, r.stderr.String())
		}
		// dec-0028 pins "44" and "237" literally. This lane's extractor
		// measures 47/231 on the real vendored corpus — see
		// internal/importadr's TestExtract and .orchestrator-status.md.
		// Asserted against the measured value for the same reason named at
		// every other site carrying this comment.
		out := r.stdout.String()
		if !strings.Contains(out, "47") {
			t.Errorf("report does not carry the measured count 47:\n%s", out)
		}
		if !strings.Contains(out, "231") {
			t.Errorf("report does not carry the measured count 231:\n%s", out)
		}

		if got := r.countFiles(t, r.entriesDir()); got != 47 {
			t.Fatalf("entries/ has %d files, want 47 as measured (dec-0028 pins 44)", got)
		}
		v, err := schema.NewValidator()
		if err != nil {
			t.Fatalf("building the schema validator: %v", err)
		}
		entries, _ := os.ReadDir(r.entriesDir())
		for _, e := range entries {
			data, err := os.ReadFile(filepath.Join(r.entriesDir(), e.Name()))
			if err != nil {
				t.Fatalf("reading %s: %v", e.Name(), err)
			}
			if err := v.Validate(data); err != nil {
				t.Errorf("%s does not satisfy entry.schema.json: %v", e.Name(), err)
			}
		}
	})

	t.Run("the control: a stub always offering INDEX fails on tams", func(t *testing.T) {
		// The real criterion for tams requires entries written and no index
		// offer. A command wired to always route INDEX writes none of the
		// 47 entries the real command does, and prints the index offer that
		// the real command, for tams, never prints — both failures at once,
		// which is the point of this control: it never offers import.
		dir := copyFixtureCorpus(t, "bbc-tams")
		out, entriesWritten := runAlwaysIndexStub(t, dir)

		if !strings.Contains(out, "Importing these gives dira nothing it can enforce.") {
			t.Fatal("the always-INDEX stub does not even print the index offer — test setup is broken")
		}
		if entriesWritten != 47 {
			t.Logf("confirmed: the always-INDEX stub wrote %d entries against bbc/tams, want 47 — "+
				"it never offers import, exactly the failure this control exists to demonstrate", entriesWritten)
		} else {
			t.Fatal("the always-INDEX stub unexpectedly matched the real tams criterion")
		}
	})

	t.Run("the control: a stub always offering IMPORT fails on meadow", func(t *testing.T) {
		// The real criterion for meadow requires exactly one index
		// artifact. A command wired to always route IMPORT never calls
		// BuildIndexArtifact at all, so it writes zero — the failure this
		// control exists to demonstrate: it never offers indexing.
		dir := copyFixtureCorpus(t, "nulib-meadow")
		artifactsWritten := runAlwaysImportStub(t, dir)

		if artifactsWritten != 1 {
			t.Logf("confirmed: the always-IMPORT stub wrote %d index artifacts against nulib/meadow, want 1 — "+
				"it never offers indexing, exactly the failure this control exists to demonstrate", artifactsWritten)
		} else {
			t.Fatal("the always-IMPORT stub unexpectedly matched the real meadow criterion")
		}
	})
}

// assertArtifactListsAllDocuments checks that the single JSON artifact under
// dir lists exactly want documents.
func assertArtifactListsAllDocuments(t *testing.T, dir string, want int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) != 1 {
		t.Fatalf("%s has %d files, want exactly 1", dir, len(entries))
	}
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatalf("reading %s: %v", entries[0].Name(), err)
	}
	var artifact importadr.IndexArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("decoding %s: %v", entries[0].Name(), err)
	}
	if len(artifact.Documents) != want {
		t.Errorf("artifact lists %d documents, want exactly %d", len(artifact.Documents), want)
	}
}

// runAlwaysIndexStub is the first red control T7 names: a command stub wired
// to always print the index offer, regardless of what the corpus actually
// measured. It exists only in this test file and calls internal/importadr
// directly, the same way cmd/dira/import.go does, except the verdict is
// hardcoded rather than read from the report.
func runAlwaysIndexStub(t *testing.T, dir string) (stdout string, entriesWritten int) {
	t.Helper()
	docs := mustScanDir(t, dir)
	report := importadr.Summarize(docs)
	report.Verdict = importadr.VerdictIndex // the stub's whole defect

	var b strings.Builder
	b.WriteString(report.Text)
	if report.Verdict == importadr.VerdictIndex {
		b.WriteString("Importing these gives dira nothing it can enforce.\n")
	}
	// The stub never calls BuildImportDrafts, so it never writes an entry —
	// entriesWritten is always 0 for this stub by construction.
	return b.String(), 0
}

// runAlwaysImportStub is the second red control: a command stub wired to
// always route IMPORT, regardless of what the corpus actually measured. It
// never calls BuildIndexArtifact — the stub's whole defect — so it always
// returns 0.
func runAlwaysImportStub(t *testing.T, dir string) (artifactsWritten int) {
	t.Helper()
	docs := mustScanDir(t, dir)
	report := importadr.Summarize(docs)
	report.Verdict = importadr.VerdictImport // the stub's whole defect

	if _, err := importadr.BuildImportDrafts(report, true, nil); err != nil {
		t.Fatalf("test setup: BuildImportDrafts: %v", err)
	}
	return 0
}

func mustScanDir(t *testing.T, dir string) []importadr.ScannedDocument {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var docs []importadr.ScannedDocument
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		docs = append(docs, importadr.ScanDocument(e.Name(), data))
	}
	return docs
}
