package engine_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kazi-org/dira/engine"
	"github.com/kazi-org/dira/internal/enforcer"
	"github.com/kazi-org/dira/ledger"
	"gopkg.in/yaml.v3"
)

type entriesReader struct {
	entries []*ledger.Entry
	err     error
}

func (r entriesReader) Entries(ctx context.Context) ([]*ledger.Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.entries, r.err
}

func fixture(t *testing.T) entriesReader {
	t.Helper()
	paths, err := filepath.Glob("../internal/enforcer/testdata/ledgers/daemon/*.md")
	if err != nil || len(paths) == 0 {
		t.Fatalf("fixture entries unavailable: %v", err)
	}
	var reader entriesReader
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		entry, err := ledger.Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		reader.entries = append(reader.entries, entry)
	}
	return reader
}

func TestCheckMatchesCanonicalCorpusJSON(t *testing.T) {
	data, err := os.ReadFile("../internal/enforcer/testdata/corpus.yaml")
	if err != nil {
		t.Fatal(err)
	}
	wantHash, err := os.ReadFile("../internal/enforcer/testdata/corpus.sha256")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != strings.TrimSpace(string(wantHash)) {
		t.Fatal("canonical corpus changed")
	}
	var corpus struct {
		Rows []struct {
			ID   string `yaml:"id"`
			Plan string `yaml:"plan"`
		} `yaml:"rows"`
	}
	if err := yaml.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Rows) == 0 {
		t.Fatal("empty corpus")
	}
	reader := fixture(t)
	for _, row := range corpus.Rows {
		t.Run(row.ID, func(t *testing.T) {
			got, err := engine.Check(context.Background(), reader, row.Plan)
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := enforcer.Check(context.Background(), reader, row.Plan)
			if err != nil {
				t.Fatal(err)
			}
			var wire bytes.Buffer
			if err := enforcer.RenderJSON(&wire, canonical); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var actual, expected any
			if json.Unmarshal(encoded, &actual) != nil || json.Unmarshal(wire.Bytes(), &expected) != nil || !reflect.DeepEqual(actual, expected) {
				t.Fatalf("public result differs from canonical CLI JSON: %s", row.ID)
			}
		})
	}
}

func TestPrivateCitationsDoNotExposeSourceText(t *testing.T) {
	reader := fixture(t)
	for _, entry := range reader.entries {
		entry.Private = true
	}
	result, err := engine.Check(context.Background(), reader, "add a background daemon to track run state")
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != engine.ExitConflict || len(result.Conflicts) == 0 {
		t.Fatal("expected private conflict")
	}
	for _, citation := range result.Conflicts {
		if !citation.Private || citation.Entry == "" || citation.Title != "" || citation.Alternative != "" || citation.WhyNot != "" || citation.RevisitIf != nil {
			t.Fatal("private source text exposed through typed result")
		}
	}
}

func TestErrorsDoNotBecomeCompliantResults(t *testing.T) {
	sentinel := errors.New("reader unavailable")
	result, err := engine.Check(context.Background(), entriesReader{err: sentinel}, "add daemon")
	if result != nil || !errors.Is(err, sentinel) {
		t.Fatal("reader error became a verdict or lost its cause")
	}
	for _, plan := range []string{"", " \t\n"} {
		if result, err := engine.Check(context.Background(), entriesReader{}, plan); result != nil || err == nil {
			t.Fatal("blank plan became a verdict")
		}
	}
	if result, err := engine.Check(context.Background(), nil, "add daemon"); result != nil || err == nil {
		t.Fatal("missing reader became a verdict")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := engine.Check(ctx, entriesReader{}, "add daemon"); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("reader cancellation became a verdict")
	}
	//nolint:staticcheck // SA1012: deliberately verify the API rejects a nil context.
	if result, err := engine.Check(nil, entriesReader{}, "add daemon"); result != nil || err == nil {
		t.Fatal("missing context became a verdict")
	}
}
