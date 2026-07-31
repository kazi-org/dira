package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kazi-org/dira/schema"
)

// TestDesignFixturesValidate makes a verified-once property permanent.
//
// E6-L1 checked the 18 entries of docs/design/fidelity/fixtures/ledger-design/
// against entry.schema.json with a throwaway module outside the repo, so no Go
// file landed there, and recorded the result in the fixtures README. Nothing
// re-checked it. A property verified once and re-checked never is the exact
// failure mode this repo keeps shipping — the fixture is the reference a pixel
// gate measures against, and a reference that quietly stops being a valid ledger
// makes the gate measure something dira could not serve.
//
// This test lives in internal/ui rather than in schema because internal/ui is
// what serves those fixtures: the thing that consumes the fixture is the thing
// that should refuse an invalid one.
func TestDesignFixturesValidate(t *testing.T) {
	t.Parallel()

	v, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("compiling entry.schema.json: %v", err)
	}

	dir := filepath.Join(repoRoot(t), "docs", "design", "fidelity", "fixtures", "ledger-design", "entries")
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Without this the test passes just as happily on a directory that has
	// been moved or emptied.
	if len(files) != 18 {
		t.Fatalf("found %d fixture entries in %s, expected 18 — the fixture ledger has changed shape, "+
			"and the README's inventory has to change with it", len(files), dir)
	}

	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := v.Validate(raw); err != nil {
			t.Errorf("%s does not validate against entry.schema.json: %v", filepath.Base(f), err)
		}
	}
}

// TestTheValidatorRejectsTheInvalidCorpus is the other side, and it is what makes
// the test above evidence rather than an assertion.
//
// A validator that accepted everything would pass TestDesignFixturesValidate
// without noticing anything. E0's invalid corpus is seventeen files, each broken
// in one specific way, and every one of them must be refused.
func TestTheValidatorRejectsTheInvalidCorpus(t *testing.T) {
	t.Parallel()

	v, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("compiling entry.schema.json: %v", err)
	}

	dir := filepath.Join(repoRoot(t), "schema", "testdata", "invalid")
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no invalid fixtures in %s; the control is not measuring anything", dir)
	}

	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := v.Validate(raw); err == nil {
			t.Errorf("%s is in the invalid corpus and the validator accepted it", filepath.Base(f))
		}
	}
	t.Logf("%d invalid fixtures, %d refused", len(files), len(files))
}

// TestTheRealLedgerValidates extends the same guarantee to .dira/ itself. The
// served surfaces read this ledger, and an entry that stopped validating would
// render as something rather than fail loudly.
func TestTheRealLedgerValidates(t *testing.T) {
	t.Parallel()

	v, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("compiling entry.schema.json: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(repoRoot(t), ".dira", "entries", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 20 {
		t.Fatalf("found %d entries in .dira/entries/; the check is not measuring anything", len(files))
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := v.Validate(raw); err != nil {
			t.Errorf("%s does not validate: %v", filepath.Base(f), err)
		}
	}
}
