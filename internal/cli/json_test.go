package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/cli"
	"github.com/kazi-org/dira/internal/index/indextest"
	"github.com/kazi-org/dira/internal/kazi"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/status"
)

// TestMapJSON is T5's acc line.
func TestMapJSON(t *testing.T) {
	ctx := context.Background()

	t.Run("every bucket value in --json output is one of dira's six", func(t *testing.T) {
		parent := entry("int-9801", nil)
		completed := entry("dec-9802", combine(derivesFrom("int-9801"), realizedBy("kazi:goal-json-complete")))
		ix := openTree(t, []*ledger.Entry{parent, completed})
		snap := singleRunPortfolio(map[string]kazi.RepoBucket{"json-complete": kazi.RepoComplete})

		var buf bytes.Buffer
		tree, err := cli.BuildTree(ctx, ix, snap, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("BuildTree: %v", err)
		}
		if err := cli.RenderJSON(&buf, tree, nil, "2026-08-14T00:00:00Z"); err != nil {
			t.Fatalf("RenderJSON: %v", err)
		}

		var decoded map[string]any
		if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
			t.Fatalf("decoding output: %v", err)
		}
		valid := map[string]bool{}
		for _, b := range validBucketStrings() {
			valid[b] = true
		}
		walkBucketFields(t, decoded, valid)
	})

	t.Run("two consecutive real invocations against an unchanged ledger agree, once observed_at is stripped", func(t *testing.T) {
		parent := entry("int-9901", nil)
		child := entry("dec-9902", derivesFrom("int-9901"))
		diraDir := indextest.Materialise(t, []*ledger.Entry{parent, child})

		binary := buildDiraForTest(t)
		first := runDiraMapJSON(t, binary, diraDir)
		second := runDiraMapJSON(t, binary, diraDir)

		delete(first, "observed_at")
		delete(second, "observed_at")
		firstBytes, _ := json.Marshal(first)
		secondBytes, _ := json.Marshal(second)
		if !bytes.Equal(firstBytes, secondBytes) {
			t.Errorf("two runs disagree once observed_at is removed:\nfirst:  %s\nsecond: %s", firstBytes, secondBytes)
		}
	})

	t.Run("the documented shape's key set matches what is actually emitted", func(t *testing.T) {
		parent := entry("int-9803", nil)
		child := entry("dec-9804", derivesFrom("int-9803"))
		ix := openTree(t, []*ledger.Entry{parent, child})
		tree, err := cli.BuildTree(ctx, ix, &kazi.Portfolio{}, nil, neverCalledStatusFn(t))
		if err != nil {
			t.Fatalf("BuildTree: %v", err)
		}
		var buf bytes.Buffer
		if err := cli.RenderJSON(&buf, tree, nil, "2026-08-14T00:00:00Z"); err != nil {
			t.Fatalf("RenderJSON: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
			t.Fatalf("decoding output: %v", err)
		}

		docPath := filepath.Join("..", "..", "docs", "design", "schemas", "map.md")
		doc, err := os.ReadFile(docPath)
		if err != nil {
			t.Fatalf("reading %s: %v", docPath, err)
		}
		for key := range decoded {
			if !strings.Contains(string(doc), "`"+key+"`") {
				t.Errorf("emitted top-level key %q is not documented in %s", key, docPath)
			}
		}
		// The reverse direction: every top-level key the doc claims must
		// actually appear at least once across a run that has both a
		// group and an unparented entry (the fixture above has neither
		// non-empty groups+unparented at once for some keys, so this walks
		// the documented top-level keys against this run's own key set,
		// which the fixture is built to cover).
		for _, key := range []string{"observed_at", "groups", "unparented"} {
			if !strings.Contains(string(doc), "`"+key+"`") {
				t.Errorf("documented shape is missing the top-level key %q", key)
			}
			if _, ok := decoded[key]; !ok {
				t.Errorf("emitted output is missing the documented top-level key %q", key)
			}
		}
	})

	t.Run("both sides: an encoder that copies kazi's own bucket strings through leaks a non-dira value", func(t *testing.T) {
		type wrongEntry struct {
			Bucket string `json:"bucket"`
		}
		// The deliberately wrong encoder: passes a RAW kazi.RepoBucket value
		// straight through instead of mapping it to a status.Bucket.
		leaked := wrongEntry{Bucket: string(kazi.RepoComplete)} // "complete" — never a status.Bucket value
		data, err := json.Marshal(leaked)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !bytes.Contains(data, []byte(`"complete"`)) {
			t.Fatal("the wrong-encoder control's own premise broke: it should leak \"complete\"")
		}
		if valid := validBucketStrings(); slices.Contains(valid, "complete") {
			t.Fatal("\"complete\" is a valid status.Bucket string; the control proves nothing")
		}
	})
}

// validBucketStrings returns every legal status.Bucket string value.
func validBucketStrings() []string {
	out := make([]string, 0, len(status.Buckets)+1)
	for _, b := range status.Buckets {
		out = append(out, string(b))
	}
	out = append(out, "") // omitted/zero value is legal (not applicable)
	return out
}

// walkBucketFields recursively checks every "bucket" key found anywhere in
// decoded against valid.
func walkBucketFields(t *testing.T, v any, valid map[string]bool) {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if k == "bucket" {
				s, ok := val.(string)
				if !ok {
					t.Errorf("bucket field is not a string: %v", val)
					continue
				}
				if !valid[s] {
					t.Errorf("bucket field %q is not one of dira's six status.Bucket values", s)
				}
			}
			walkBucketFields(t, val, valid)
		}
	case []any:
		for _, item := range x {
			walkBucketFields(t, item, valid)
		}
	}
}

// buildDiraForTest builds the dira binary once for this test file's process
// determinism check, which needs two real subprocess invocations rather
// than two in-process calls sharing state.
func buildDiraForTest(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "dira")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/kazi-org/dira/cmd/dira")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building dira: %v\n%s", err, out)
	}
	return bin
}

func runDiraMapJSON(t *testing.T, binary, diraDir string) map[string]any {
	t.Helper()
	cmd := exec.Command(binary, "map", "--json")
	cmd.Dir = filepath.Dir(diraDir) // the directory CONTAINING .dira
	// PATH cleared so both invocations agree on kazi being unavailable —
	// the determinism claim under test is about the command's own output,
	// not about whether this machine happens to have kazi installed.
	cmd.Env = envWithEmptyPath(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dira map --json: %v\n%s", err, out)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("decoding dira map --json output: %v\noutput:\n%s", err, out)
	}
	return decoded
}

// envWithEmptyPath returns env with PATH replaced by an empty value,
// wherever it appears.
func envWithEmptyPath(env []string) []string {
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			out = append(out, "PATH=")
			replaced = true
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, "PATH=")
	}
	return out
}

// repoRoot resolves this module's root via `go env GOMOD`, so the build
// above works regardless of the test binary's own working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("go env GOMOD: %v", err)
	}
	return filepath.Dir(strings.TrimSpace(string(out)))
}
