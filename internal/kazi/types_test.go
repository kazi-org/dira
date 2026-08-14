package kazi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// E4-L1-T3. decodeSnapshot is not exercised through Snapshot() here — that is
// T4's process-invocation layer — this file is the decode layer alone,
// against T2's fixtures.

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "kazi", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return data
}

// TestTypes is the lane's own name for this task's acc line
// (`go test ./internal/kazi -run TestTypes`).
func TestTypes(t *testing.T) {
	t.Parallel()

	t.Run("portfolio-populated.json decodes cleanly", func(t *testing.T) {
		t.Parallel()

		snap, err := decodeSnapshot(readFixture(t, "portfolio-populated.json"))
		if err != nil {
			t.Fatalf("decodeSnapshot: %v", err)
		}
		if snap.SchemaVersion != 2 {
			t.Errorf("SchemaVersion = %d, want 2", snap.SchemaVersion)
		}
		if len(snap.ByRepo) == 0 {
			t.Fatal("ByRepo is empty; the fixture is not being read")
		}
		if len(snap.TotalsRows) != 5 {
			t.Fatalf("TotalsRows has %d entries, want 5 (done, running, blocked, todo, planned)", len(snap.TotalsRows))
		}

		// Every by_repo and fleet_remote bucket value decodes as a
		// RepoBucket constant — spot-checked, not exhaustively (T2's
		// completeness test already proves the corpus itself is
		// complete; this proves the decoder reads what it is given).
		seenRepoBuckets := map[RepoBucket]bool{}
		for repo, buckets := range snap.ByRepo {
			for bucket, runs := range buckets {
				if !repoBucketValid(bucket) {
					t.Errorf("by_repo[%q] carries bucket %q, not one of %v", repo, bucket, repoBuckets)
				}
				seenRepoBuckets[bucket] = true
				for _, run := range runs {
					if run.Bucket != bucket {
						t.Errorf("by_repo[%q][%q]: run %s carries Bucket %q, want %q",
							repo, bucket, run.RunID, run.Bucket, bucket)
					}
				}
			}
		}
		if len(seenRepoBuckets) == 0 {
			t.Fatal("no by_repo bucket was ever read; the spot check above ran over nothing")
		}

		// Every totals.rows[].bucket value decodes as a RowBucket
		// constant; all five must appear, since portfolio.ex's
		// @bucket_order always emits one row per bucket.
		gotRowBuckets := map[RowBucket]bool{}
		for _, row := range snap.TotalsRows {
			if !rowBucketValid(row.Bucket) {
				t.Errorf("totals.rows carries bucket %q, not one of %v", row.Bucket, rowBuckets)
			}
			gotRowBuckets[row.Bucket] = true
		}
		for _, want := range rowBuckets {
			if !gotRowBuckets[want] {
				t.Errorf("totals.rows never carries bucket %q", want)
			}
		}
	})

	t.Run("rate.empty? and totals.empty are read from two different keys", func(t *testing.T) {
		t.Parallel()

		// The natural recording has both false, which does not by
		// itself prove the two fields are independently sourced — a
		// decoder that copied one into the other, or read neither and
		// left both at their zero value, would look identical. This
		// fixture deliberately disagrees the two keys, so a decoder
		// that conflated them (or hardcoded either) is caught: it
		// would report the same value for both, which is wrong here
		// either way.
		mismatched := mutateJSON(t, readFixture(t, "portfolio-populated.json"), func(doc map[string]any) {
			totals := doc["totals"].(map[string]any)
			totals["empty"] = true
			rate := doc["rate"].(map[string]any)
			rate["empty?"] = false
		})
		snap, err := decodeSnapshot(mismatched)
		if err != nil {
			t.Fatalf("decodeSnapshot: %v", err)
		}
		if !snap.TotalsEmpty {
			t.Error("TotalsEmpty = false, want true (read from totals.empty)")
		}
		if snap.RateEmpty {
			t.Error("RateEmpty = true, want false (read from rate.empty?, a different key from totals.empty)")
		}

		// And the reverse assignment, so this is not "TotalsEmpty
		// always true, RateEmpty always false" by construction.
		reversed := mutateJSON(t, readFixture(t, "portfolio-populated.json"), func(doc map[string]any) {
			doc["totals"].(map[string]any)["empty"] = false
			doc["rate"].(map[string]any)["empty?"] = true
		})
		snap, err = decodeSnapshot(reversed)
		if err != nil {
			t.Fatalf("decodeSnapshot: %v", err)
		}
		if snap.TotalsEmpty {
			t.Error("TotalsEmpty = true, want false")
		}
		if !snap.RateEmpty {
			t.Error("RateEmpty = false, want true")
		}
	})

	t.Run("portfolio-empty.json decodes with both empty flags true", func(t *testing.T) {
		t.Parallel()

		snap, err := decodeSnapshot(readFixture(t, "portfolio-empty.json"))
		if err != nil {
			t.Fatalf("decodeSnapshot: %v", err)
		}
		if !snap.TotalsEmpty || !snap.RateEmpty {
			t.Errorf("TotalsEmpty=%t RateEmpty=%t, want both true", snap.TotalsEmpty, snap.RateEmpty)
		}
	})

	// The red control. Both directions of the swap are inline, not
	// committed to testdata/ — their only purpose is proving the decoder
	// rejects a value from the wrong enum, per this task's "both sides"
	// note.
	t.Run("a RepoBucket value in a totals.rows[].bucket slot fails to decode", func(t *testing.T) {
		t.Parallel()

		swapped := mutateJSON(t, readFixture(t, "portfolio-populated.json"), func(doc map[string]any) {
			rows := doc["totals"].(map[string]any)["rows"].([]any)
			row0 := rows[0].(map[string]any)
			before := row0["bucket"]
			row0["bucket"] = "complete" // a RepoBucket value, never a RowBucket one
			if before == "complete" {
				t.Fatal("totals.rows[0].bucket was already \"complete\"; this fixture proves nothing")
			}
		})
		snap, err := decodeSnapshot(swapped)
		if err == nil {
			t.Fatalf("decodeSnapshot succeeded on a swapped bucket; want a decode error, got snapshot with "+
				"TotalsRows[0].Bucket = %q", snap.TotalsRows[0].Bucket)
		}
		if !strings.Contains(err.Error(), "RowBucket") {
			t.Errorf("decode error does not name RowBucket, so this may be failing for the wrong reason: %v", err)
		}
	})

	t.Run("a RowBucket value in a by_repo bucket key fails to decode", func(t *testing.T) {
		t.Parallel()

		swapped := mutateJSON(t, readFixture(t, "portfolio-populated.json"), func(doc map[string]any) {
			byRepo := doc["by_repo"].(map[string]any)
			renamed := false
			for repo, raw := range byRepo {
				buckets := raw.(map[string]any)
				for bucket, runs := range buckets {
					delete(buckets, bucket)
					buckets["planned"] = runs // a RowBucket value, never a RepoBucket one
					renamed = true
					_ = repo
					break
				}
				if renamed {
					break
				}
			}
			if !renamed {
				t.Fatal("by_repo is empty; there was no bucket key to rename")
			}
		})
		snap, err := decodeSnapshot(swapped)
		if err == nil {
			t.Fatalf("decodeSnapshot succeeded on a by_repo bucket renamed to \"planned\"; want a decode "+
				"error, got a snapshot with %d by_repo entries", len(snap.ByRepo))
		}
		if !strings.Contains(err.Error(), "RepoBucket") {
			t.Errorf("decode error does not name RepoBucket, so this may be failing for the wrong reason: %v", err)
		}
	})
}

func repoBucketValid(b RepoBucket) bool {
	for _, want := range repoBuckets {
		if b == want {
			return true
		}
	}
	return false
}

func rowBucketValid(b RowBucket) bool {
	for _, want := range rowBuckets {
		if b == want {
			return true
		}
	}
	return false
}

// mutateJSON decodes data into a generic map, applies mutate, and
// re-encodes. Used only to build the inline red-control fixtures above — the
// untouched fixtures are read as bytes directly, never round-tripped through
// this.
func mutateJSON(t *testing.T, data []byte, mutate func(map[string]any)) []byte {
	t.Helper()

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("mutateJSON: decoding: %v", err)
	}
	mutate(doc)
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("mutateJSON: encoding: %v", err)
	}
	return out
}
