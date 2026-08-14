package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/schema"
)

// TestInitEntriesValidateAgainstSchema is E5-L5-T4's acceptance line. L-0015
// exists for exactly this gap: ValidateDraft is dira's own Go rules, and
// entry.schema.json is the published contract; this proves a freshly-written
// entry satisfies both, the write path's counterpart to
// internal/ledger/schema_test.go's read-path check.
func TestInitEntriesValidateAgainstSchema(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	a := newApp(&out, &errBuf)
	a.stdin = strings.NewReader(answerLines(completeInterviewAnswers...))
	if code := a.main([]string{"init", "-C", root, "--interview"}); code != exitOK {
		t.Fatalf("dira init --interview: exit %d, stderr %s", code, errBuf.String())
	}

	validator, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("schema.NewValidator: %v", err)
	}

	entriesDir := filepath.Join(root, ".dira", "entries")
	paths, err := filepath.Glob(filepath.Join(entriesDir, "*.md"))
	if err != nil {
		t.Fatalf("globbing %s: %v", entriesDir, err)
	}
	// Without this the loop below passes just as happily over zero files.
	if len(paths) < 3 {
		t.Fatalf("init wrote %d entry files, want at least 3 — the intent, the constraint and the question", len(paths))
	}

	var corrupted []byte
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		if err := validator.Validate(content); err != nil {
			t.Errorf("%s does not validate against entry.schema.json: %v", filepath.Base(path), err)
		}
		if strings.Contains(string(content), "kind: constraint") {
			corrupted = corruptStateForKind(content)
		}
	}
	if corrupted == nil {
		t.Fatal("no constraint entry was found to corrupt; the red control below did not run")
	}

	// The red control: proving the compiler is actually checking these
	// bytes, not returning a cached success.
	//
	// entry.schema.json's own allOf does not, in fact, bind id's prefix to
	// kind — that agreement is Go's Entry.Validate, not the published JSON
	// contract (checked directly: a "kind: intent" entry carrying a
	// cst-prefixed id validates cleanly against entry.schema.json, because
	// no rule in the document says otherwise). The allOf block the schema
	// does enforce is state-per-kind, so the corruption below moves a
	// constraint to a state ("accepted") only decisions may carry —
	// exactly the class of defect L-0015 exists to catch, and the one this
	// schema document actually rejects.
	if err := validator.Validate(corrupted); err == nil {
		t.Fatal("the validator accepted a constraint carrying a state no constraint may hold")
	}
}

// corruptStateForKind rewrites a constraint entry's state to "accepted" —
// valid for a decision, not for a constraint (entry.schema.json's allOf
// restricts a constraint's state to active/superseded).
func corruptStateForKind(content []byte) []byte {
	return []byte(strings.Replace(string(content), "state: active", "state: accepted", 1))
}
