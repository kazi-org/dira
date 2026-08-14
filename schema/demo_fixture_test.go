package schema

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDemoFixtureLedgerValidates registers fixtures/demo-ledger/ (E8-L3-T1 —
// a byte-identical copy of internal/enforcer/testdata/ledgers/daemon/
// {dec-0060,int-0002}.md) into this package's schema-validation corpus, per
// docs/plan/tasks/E8-L3.md's T7: a future schema change must be caught here,
// in Go, and not only by docs/growth/scripts/check-fixture-ledger.mjs's
// hand-rolled, zero-dependency node checker.
//
// T7 was written expecting no Go schema validator to exist yet. It already
// does (this package, shipped by E0-L2) — E0-L4/E0-L5, the still-outline E0
// lanes, are the release/tap lanes, not the validator. E8-L3-T1's copy makes
// this test startable without waiting on either.
func TestDemoFixtureLedgerValidates(t *testing.T) {
	t.Parallel()

	const dir = "../fixtures/demo-ledger"
	sch := compileSchema(t)

	// Named, not just counted: a check that only asserted len(paths) == 2
	// would report "1 file" on either fixture's removal without saying which
	// one is gone.
	for _, name := range []string{"dec-0060.md", "int-0002.md"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s is required in the demo fixture ledger and is missing: %v", path, err)
		}
	}

	for _, path := range markdownFiles(t, dir) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			entry, err := parseEntry(path)
			if err != nil {
				t.Fatalf("%s is not a readable entry: %v", path, err)
			}
			if err := sch.Validate(entry); err != nil {
				t.Errorf("%s violates %s:\n%v", path, schemaFile, err)
			}
		})
	}
}
