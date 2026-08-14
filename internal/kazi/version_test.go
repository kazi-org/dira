package kazi

import (
	"context"
	"testing"
)

// E4-L1-T5. schema_version is lockstep across kazi's whole --json surface, so
// a version other than PinnedSchemaVersion means kazi moved for reasons that
// may have nothing to do with portfolio's shape specifically — contract
// drift, not absence. Snapshot() must keep working and say so, never blank
// the result or return an error for this reason alone.

func TestContractDrift(t *testing.T) {
	t.Run("portfolio-schema-drift.json sets ContractDrift and keeps the fields", func(t *testing.T) {
		withRunner(t, fixtureRunner(t, "portfolio-schema-drift.json"))

		snap, err := Snapshot(context.Background())
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		if snap == nil {
			t.Fatal("Snapshot returned a nil *Portfolio with a nil error")
		}
		if !snap.ContractDrift {
			t.Error("ContractDrift = false, want true")
		}
		if snap.SchemaVersion == PinnedSchemaVersion {
			t.Fatalf("SchemaVersion = %d, same as PinnedSchemaVersion; the fixture is not testing drift", snap.SchemaVersion)
		}
		if snap.SchemaVersion != 5 {
			// Pinned to the fixture's actual value, per the acc line:
			// "never the pinned constant, so a caller can name what
			// it saw." Deliberately not PinnedSchemaVersion+1 (3) —
			// see testdata/kazi/README.md.
			t.Errorf("SchemaVersion = %d, want 5 (testdata/kazi/portfolio-schema-drift.json's actual value)", snap.SchemaVersion)
		}

		// The drift path is best-effort, not blank. Spot-checked, not
		// exhaustively — T3/T4 already prove the decoder itself.
		if len(snap.ByRepo) == 0 {
			t.Error("ByRepo is empty on a drifted snapshot; the drift path blanked the decode instead of decoding best-effort")
		}
		if len(snap.Planned) == 0 {
			t.Error("Planned is empty on a drifted snapshot")
		}
	})

	t.Run("portfolio-populated.json (pinned version) has ContractDrift false", func(t *testing.T) {
		withRunner(t, fixtureRunner(t, "portfolio-populated.json"))

		snap, err := Snapshot(context.Background())
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		if snap.SchemaVersion != PinnedSchemaVersion {
			t.Fatalf("SchemaVersion = %d, want the pinned %d; this fixture is supposed to be the untouched baseline",
				snap.SchemaVersion, PinnedSchemaVersion)
		}
		if snap.ContractDrift {
			t.Error("ContractDrift = true on the pinned version, want false")
		}
	})

	// The red control this task's "both sides" note asks for: a build that
	// hardcoded drift against the one committed number (3) rather than
	// genuinely comparing to PinnedSchemaVersion would pass the case above
	// and fail this one.
	t.Run("drift fires on PinnedSchemaVersion+1, not only on the committed fixture's 3", func(t *testing.T) {
		data := readFixture(t, "portfolio-populated.json")
		mutated := mutateJSON(t, data, func(doc map[string]any) {
			doc["schema_version"] = float64(PinnedSchemaVersion + 1)
		})
		withRunner(t, func(_ context.Context, _ []string) ([]byte, int, error) {
			return mutated, 0, nil
		})

		snap, err := Snapshot(context.Background())
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		if !snap.ContractDrift {
			t.Errorf("ContractDrift = false for schema_version %d, want true", PinnedSchemaVersion+1)
		}
		if snap.SchemaVersion != PinnedSchemaVersion+1 {
			t.Errorf("SchemaVersion = %d, want %d", snap.SchemaVersion, PinnedSchemaVersion+1)
		}
	})
}
