package sniff

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The graded corpus and the recorded transcripts. Paths are constants so that a
// test moving one of them has to say so.
const (
	corpusFile   = "testdata/corpus.yaml"
	corpusFreeze = "testdata/corpus.sha256"
	transcripts  = "testdata/transcripts"
)

// A corpusRow is one labelled sentence.
type corpusRow struct {
	ID       string `yaml:"id"`
	Text     string `yaml:"text"`
	Expect   string `yaml:"expect"` // "decision" | "none"
	NearMiss string `yaml:"near_miss,omitempty"`
	Note     string `yaml:"note"`
}

type corpusFileT struct {
	Rows []corpusRow `yaml:"rows"`
}

// assertCorpusFrozen fails unless corpus.yaml still hashes to corpus.sha256.
//
// Every test that produces a number calls this first, and the ordering is the
// point. The corpus was written and frozen before internal/sniff/match.go
// existed, precisely so that the cheapest way to improve a false-positive rate —
// deleting the rows that produce it — stops working. This function is what makes
// that true rather than aspirational.
//
// The pattern is E3-L1's, deliberately: internal/enforcer/fixture_test.go does
// the same thing for the relitigation matcher, and two capture-quality gates
// that could be cheated in different ways would be two gates to reason about.
func assertCorpusFrozen(t *testing.T) {
	t.Helper()

	data, err := os.ReadFile(corpusFile)
	if err != nil {
		t.Fatalf("reading %s: %v", corpusFile, err)
	}
	want, err := os.ReadFile(corpusFreeze)
	if err != nil {
		t.Fatalf("reading %s: %v", corpusFreeze, err)
	}

	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if expected := strings.TrimSpace(string(want)); got != expected {
		t.Fatalf("%s has changed.\n  got  %s\n  want %s\n"+
			"The corpus was frozen before internal/sniff/match.go existed so that a matcher which cannot reach "+
			"the bar reports an honest number instead of deleting the rows it fails (docs/plan/lanes/E2.md, "+
			".dira/entries/dec-0003.md). If a row is genuinely mislabelled, change it and regenerate %s in the "+
			"same commit, naming the row and the reason.",
			corpusFile, got, expected, corpusFreeze)
	}
}

func loadCorpus(t *testing.T) []corpusRow {
	t.Helper()

	data, err := os.ReadFile(corpusFile)
	if err != nil {
		t.Fatalf("reading %s: %v", corpusFile, err)
	}
	var c corpusFileT
	if err := yaml.Unmarshal(data, &c); err != nil {
		t.Fatalf("parsing %s: %v", corpusFile, err)
	}
	if len(c.Rows) == 0 {
		t.Fatalf("%s holds no rows", corpusFile)
	}
	return c.Rows
}

// transcriptPath resolves a fixture by name and fails if it is missing, rather
// than letting a typo become a test that reads an empty file and passes.
func transcriptPath(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(transcripts, name)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("transcript fixture %s: %v", name, err)
	}
	if info.Size() == 0 {
		t.Fatalf("transcript fixture %s is empty; a test over it would prove nothing", name)
	}
	return path
}

// openTranscript opens a fixture for a test, closing it when the test ends.
func openTranscript(t *testing.T, name string) *os.File {
	t.Helper()

	f, err := os.Open(transcriptPath(t, name))
	if err != nil {
		t.Fatalf("opening %s: %v", name, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// recordedTranscripts is every fixture in testdata/transcripts.
func recordedTranscripts(t *testing.T) []string {
	t.Helper()

	names, err := filepath.Glob(filepath.Join(transcripts, "*.jsonl"))
	if err != nil {
		t.Fatalf("globbing %s: %v", transcripts, err)
	}
	if len(names) < 3 {
		t.Fatalf("%s holds %d transcripts, want at least 3", transcripts, len(names))
	}
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = filepath.Base(n)
	}
	return out
}
