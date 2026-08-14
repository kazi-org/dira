package kazi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// E4-L1-T2. This test lands before internal/kazi has any production code —
// T3 is what makes it meaningful — because the corpus is the contract every
// later task in this lane decodes against, and a corpus that silently loses a
// case (the founder decision's whole premise: pin a recorded fixture, not a
// live binary) would make every later task's green bar mean less than it
// claims.

// corpusFiles are the seven fixtures every other task in this lane (and
// E4-L3's pairing) decodes against. testdata/kazi/README.md documents how
// each was captured.
var corpusFiles = []string{
	"portfolio-populated.json",
	"portfolio-empty.json",
	"portfolio-all-causes.json",
	"portfolio-fleet-remote.json",
	"portfolio-schema-drift.json",
	"status-run.json",
	"status-proposal.json",
}

// corpusCauses are the four `blocked[].cause` values portfolio.ex's
// blocker_label/1 names — dag, over_budget, error and stuck — that
// portfolio-all-causes.json must carry all of.
var corpusCauses = []string{"dag", "over_budget", "error", "stuck"}

// checkCorpus runs every completeness clause against dir and returns every
// violation found, rather than stopping at the first. That is what lets the
// same function back both the positive assertion (the real corpus: zero
// problems) and the negative control (an empty directory: many problems) in
// TestFixtureCorpus below.
func checkCorpus(dir string) []string {
	var problems []string

	raw := map[string][]byte{}
	for _, name := range corpusFiles {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if !json.Valid(data) {
			problems = append(problems, fmt.Sprintf("%s: does not parse as JSON", name))
			continue
		}
		raw[name] = data
	}

	if data, ok := raw["portfolio-all-causes.json"]; ok {
		for _, cause := range corpusCauses {
			// A raw string search, per the acc line — "checked by string
			// search over the raw JSON rather than by eye" — so this
			// clause needs no decoder of its own and cannot be fooled by
			// a cause value sitting in the wrong field.
			needle := fmt.Appendf(nil, `"cause":"%s"`, cause)
			if !bytes.Contains(data, needle) && !bytes.Contains(data, fmt.Appendf(nil, `"cause": "%s"`, cause)) {
				problems = append(problems, fmt.Sprintf(
					"portfolio-all-causes.json: blocked[] carries no cause %q", cause))
			}
		}
	}

	if data, ok := raw["portfolio-fleet-remote.json"]; ok {
		var doc struct {
			FleetRemote []json.RawMessage `json:"fleet_remote"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			problems = append(problems, fmt.Sprintf("portfolio-fleet-remote.json: %v", err))
		} else if len(doc.FleetRemote) < 1 {
			problems = append(problems, "portfolio-fleet-remote.json: fleet_remote has length 0, want >= 1")
		}
	}

	driftData, haveDrift := raw["portfolio-schema-drift.json"]
	populatedData, havePopulated := raw["portfolio-populated.json"]
	if haveDrift && havePopulated {
		driftVersion, driftErr := schemaVersionOf(driftData)
		populatedVersion, popErr := schemaVersionOf(populatedData)
		switch {
		case driftErr != nil:
			problems = append(problems, fmt.Sprintf("portfolio-schema-drift.json: %v", driftErr))
		case popErr != nil:
			problems = append(problems, fmt.Sprintf("portfolio-populated.json: %v", popErr))
		case driftVersion == populatedVersion:
			problems = append(problems, fmt.Sprintf(
				"portfolio-schema-drift.json's schema_version (%d) does not differ from "+
					"portfolio-populated.json's (%d)", driftVersion, populatedVersion))
		}
	}

	if data, ok := raw["portfolio-populated.json"]; ok {
		multiRun, ref, err := multiRunGoalRef(data)
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("portfolio-populated.json: %v", err))
		case !multiRun:
			problems = append(problems, "portfolio-populated.json: by_repo has no goal_ref appearing "+
				"more than once across its buckets — the multi-run case README.md names and E4-L3 depends on")
		default:
			_ = ref // named in README.md, not asserted to a specific value here
		}
	}

	if data, ok := raw["status-run.json"]; ok {
		if kind, err := kindOf(data); err != nil {
			problems = append(problems, fmt.Sprintf("status-run.json: %v", err))
		} else if kind != "run" {
			problems = append(problems, fmt.Sprintf("status-run.json: kind = %q, want \"run\"", kind))
		}
	}
	if data, ok := raw["status-proposal.json"]; ok {
		if kind, err := kindOf(data); err != nil {
			problems = append(problems, fmt.Sprintf("status-proposal.json: %v", err))
		} else if kind != "proposal" {
			problems = append(problems, fmt.Sprintf("status-proposal.json: kind = %q, want \"proposal\"", kind))
		}
	}

	return problems
}

// schemaVersionOf reads the top-level schema_version field common to every
// fixture in this corpus.
func schemaVersionOf(data []byte) (int, error) {
	var doc struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return 0, err
	}
	return doc.SchemaVersion, nil
}

// kindOf reads the top-level kind field.
func kindOf(data []byte) (string, error) {
	var doc struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", err
	}
	return doc.Kind, nil
}

// multiRunGoalRef reports whether portfolio JSON's by_repo carries any
// goal_ref more than once across its repo/bucket keys — the shape
// lane doc point 2 names and E4-L3's join must handle rather than assume
// away.
func multiRunGoalRef(data []byte) (found bool, ref string, err error) {
	var doc struct {
		ByRepo map[string]map[string][]struct {
			GoalRef string `json:"goal_ref"`
		} `json:"by_repo"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return false, "", err
	}
	counts := map[string]int{}
	for _, buckets := range doc.ByRepo {
		for _, runs := range buckets {
			for _, run := range runs {
				counts[run.GoalRef]++
			}
		}
	}
	for goalRef, n := range counts {
		if n > 1 {
			return true, goalRef, nil
		}
	}
	return false, "", nil
}

// TestFixtureCorpus is E4-L1-T2's acceptance gate.
func TestFixtureCorpus(t *testing.T) {
	t.Parallel()

	t.Run("the real corpus has every load-bearing case", func(t *testing.T) {
		t.Parallel()

		for _, problem := range checkCorpus(filepath.Join("testdata", "kazi")) {
			t.Error(problem)
		}
	})

	// The red control, per docs/lore.md L-0001: an empty corpus trivially
	// satisfies every "file X exists" clause it never reaches, which is
	// exactly the vacuous-check shape the lane doc's "both sides" note
	// warns about. If checkCorpus finds nothing wrong with an empty
	// directory, it is not measuring the corpus at all.
	t.Run("an empty directory is caught, not vacuously accepted", func(t *testing.T) {
		t.Parallel()

		problems := checkCorpus(t.TempDir())
		if len(problems) == 0 {
			t.Fatal("checkCorpus reported no problems against an empty testdata/kazi/ directory; " +
				"the completeness check would pass on a corpus with nothing in it")
		}
		if len(problems) < len(corpusFiles) {
			t.Errorf("checkCorpus reported %d problem(s) against an empty directory, want at least "+
				"%d — one missing-file problem per fixture, which is what proves every clause was "+
				"actually reached rather than short-circuiting on the first miss",
				len(problems), len(corpusFiles))
		}
	})
}
