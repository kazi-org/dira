package enforcer

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
)

// The fixture ledger every test in this package grades against, and the corpus
// that grades the matcher. Paths are constants so that a test moving one of
// them has to say so.
const (
	daemonLedger = "testdata/ledgers/daemon"
	corpusFile   = "testdata/corpus.yaml"
	corpusFreeze = "testdata/corpus.sha256"
)

// assertCorpusFrozen fails unless corpus.yaml still hashes to corpus.sha256.
//
// It is called first by every test that grades the matcher, and that ordering
// is the point. E3-L1 wrote and checksummed the corpus before any matcher
// existed, precisely so that the cheapest way to make a failing detection test
// pass — deleting the rows it fails — stops working. This function is what
// makes that true rather than aspirational: an edited corpus fails here, loudly
// and by name, before any score is computed.
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
			"The corpus was frozen before any matcher existed so that a matcher which cannot reach the bar "+
			"reports an honest precision/recall result instead of deleting the rows it fails "+
			"(docs/plan/lanes/E3.md, .dira/entries/dec-0014.md). If the corpus genuinely needs to change, "+
			"regenerate %s by hand in the same commit and say why.",
			corpusFile, got, expected, corpusFreeze)
	}
}

// fixtureEntries reads a fixture ledger's entry files through dira's own codec.
func fixtureEntries(t *testing.T, dir string) []*ledger.Entry {
	t.Helper()

	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	var out []*ledger.Entry
	for _, name := range names {
		if name.IsDir() || filepath.Ext(name.Name()) != ".md" {
			continue
		}
		e := fixtureEntry(t, filepath.Join(dir, name.Name()))
		out = append(out, e)
	}
	if len(out) == 0 {
		t.Fatalf("no entry files in %s", dir)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func fixtureEntry(t *testing.T, path string) *ledger.Entry {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	e, err := ledger.Decode(data)
	if err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	return e
}
