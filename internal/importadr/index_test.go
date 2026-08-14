package importadr

import (
	"errors"
	"reflect"
	"testing"
)

// TestIndexPolicy is E2-L7-T4's acceptance line.
func TestIndexPolicy(t *testing.T) {
	meadow := Summarize(scanCorpus(t, "nulib-meadow"))
	if meadow.Verdict != VerdictIndex {
		t.Fatalf("test setup: nulib-meadow routed %s, want INDEX", meadow.Verdict)
	}

	// Both sides in the same test run: a policy that always writes, or never
	// writes, cannot pass both.
	t.Run("confirmed true writes exactly one artifact listing every document", func(t *testing.T) {
		artifact, err := BuildIndexArtifact(meadow, true)
		if err != nil {
			t.Fatalf("BuildIndexArtifact: %v", err)
		}
		if artifact == nil {
			t.Fatal("BuildIndexArtifact returned no artifact for a confirmed INDEX report")
		}
		if len(artifact.Documents) != 31 {
			t.Errorf("artifact lists %d documents, want exactly 31", len(artifact.Documents))
		}
		want := make(map[string]bool, len(meadow.Documents))
		for _, d := range meadow.Documents {
			want[d.Path] = true
		}
		for _, ref := range artifact.Documents {
			if !want[ref.Path] {
				t.Errorf("artifact lists %q, which was not among the scanned documents", ref.Path)
			}
			delete(want, ref.Path)
		}
		if len(want) != 0 {
			t.Errorf("%d scanned documents are missing from the artifact: %v", len(want), want)
		}
	})

	t.Run("confirmed false writes nothing", func(t *testing.T) {
		artifact, err := BuildIndexArtifact(meadow, false)
		if err != nil {
			t.Fatalf("BuildIndexArtifact: %v", err)
		}
		if artifact != nil {
			t.Errorf("BuildIndexArtifact returned an artifact for confirmed=false: %+v", artifact)
		}
	})

	t.Run("an IMPORT-routed report is a caller error, not a silent no-op", func(t *testing.T) {
		tams := Summarize(scanCorpus(t, "bbc-tams"))
		if tams.Verdict != VerdictImport {
			t.Fatalf("test setup: bbc-tams routed %s, want IMPORT", tams.Verdict)
		}
		artifact, err := BuildIndexArtifact(tams, true)
		if err == nil {
			t.Fatal("BuildIndexArtifact accepted an IMPORT-routed report without error")
		}
		if !errors.Is(err, ErrWrongVerdict) {
			t.Errorf("error = %v, want it to wrap ErrWrongVerdict", err)
		}
		if artifact != nil {
			t.Errorf("BuildIndexArtifact returned a non-nil artifact alongside an error: %+v", artifact)
		}
	})
}

// TestIndexArtifactCannotCarryEntryFields is cst-0002's closed set, checked
// mechanically rather than by convention: IndexArtifact (and everything it
// carries) must have no field whose name could serialise into a ledger
// entry's kind, state or alternatives. A later edit that adds such a field
// fails this test by name, not by someone noticing in review.
func TestIndexArtifactCannotCarryEntryFields(t *testing.T) {
	forbidden := map[string]bool{
		"kind":         true,
		"state":        true,
		"alternatives": true,
		"confirmedby":  true,
		"confirmed_by": true,
		"adr":          true,
	}
	checkNoForbiddenFields(t, reflect.TypeOf(IndexArtifact{}), forbidden)
}

// checkNoForbiddenFields walks typ's fields recursively (structs and slices
// of structs), failing the test by name for any field whose lower-cased name
// is in forbidden.
func checkNoForbiddenFields(t *testing.T, typ reflect.Type, forbidden map[string]bool) {
	t.Helper()
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		lower := toLowerASCII(f.Name)
		if forbidden[lower] {
			t.Errorf("%s.%s is a forbidden field name — it could serialise into a ledger entry's %s, "+
				"which cst-0002 closes off for this artifact", typ.Name(), f.Name, f.Name)
		}
		checkNoForbiddenFields(t, f.Type, forbidden)
	}
}

func toLowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
