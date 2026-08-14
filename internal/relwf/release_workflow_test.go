// Package relwf is a test-only package (no non-test .go file — see
// docs/plan/tasks/E0-L4.md's boundary note) that parses
// .github/workflows/release.yml with gopkg.in/yaml.v3 and asserts its
// structure, the same shape docs/plan/tasks/E0-L4.md's T3 and
// docs/plan/tasks/E0-L5.md's T3 both describe.
package relwf

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// moduleRoot is the directory holding go.mod, found from this file's own
// path rather than from the working directory — same pattern as
// internal/nomodel/nomodel_test.go's moduleRoot and internal/ui/ui_test.go's
// repoRoot.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate this test file")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file))) // internal/relwf -> internal -> root
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("no go.mod at the computed module root %s: %v", root, err)
	}
	return root
}

func releaseWorkflowPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), ".github", "workflows", "release.yml")
}

// --- typed shape, just enough of it to assert the structure below ---

type workflowFile struct {
	Permissions any                    `yaml:"permissions"`
	On          map[string]any         `yaml:"on"`
	Jobs        map[string]workflowJob `yaml:"jobs"`
}

type workflowJob struct {
	RunsOn      any               `yaml:"runs-on"`
	Needs       any               `yaml:"needs"`
	Permissions map[string]string `yaml:"permissions"`
	Steps       []workflowStep    `yaml:"steps"`
}

type workflowStep struct {
	Name string         `yaml:"name"`
	ID   string         `yaml:"id"`
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	With map[string]any `yaml:"with"`
}

func parseWorkflow(t *testing.T, raw []byte) workflowFile {
	t.Helper()
	var wf workflowFile
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("parsing workflow YAML: %v", err)
	}
	return wf
}

// --- assertion functions, each proven both ways below ---

var shaPinPattern = regexp.MustCompile(`^[^@]+@[0-9a-f]{40}$`)

// validateSHAPins returns an error naming the first `uses:` value in the
// workflow that is not pinned to a 40-character commit SHA.
func validateSHAPins(wf workflowFile) error {
	for jobName, job := range wf.Jobs {
		for i, step := range job.Steps {
			if step.Uses == "" {
				continue
			}
			if !shaPinPattern.MatchString(step.Uses) {
				return errorf("job %q step %d: uses %q is not pinned to a 40-character commit SHA", jobName, i, step.Uses)
			}
		}
	}
	return nil
}

// stepIsSmokeTest reports whether a step's run: block is the version smoke
// test — the one thing docs/plan/tasks/E0-L4.md's T3 requires to run before
// publish.
func stepIsSmokeTest(s workflowStep) bool {
	return strings.Contains(s.Run, "--version")
}

// goreleaserReleasePattern matches the goreleaser invocation that creates
// and uploads the GitHub Release, as opposed to `check` or `build`.
var goreleaserReleasePattern = regexp.MustCompile(`goreleaser/v2@\S+\s+release\b`)

// stepPublishesRelease reports whether a step creates or uploads a Release.
func stepPublishesRelease(s workflowStep) bool {
	return goreleaserReleasePattern.MatchString(s.Run) || strings.Contains(s.Uses, "action-gh-release")
}

// validateSmokeBeforePublish returns an error if no smoke-test step exists,
// no publish step exists, or the first smoke-test step's index is not
// strictly lower than the first publish step's index. Index-based, so
// reordering the steps trips it rather than a search finding both anywhere
// in the file (docs/plan/tasks/E0-L4.md's own words for this assertion).
func validateSmokeBeforePublish(steps []workflowStep) error {
	smokeIdx, publishIdx := -1, -1
	for i, s := range steps {
		if smokeIdx == -1 && stepIsSmokeTest(s) {
			smokeIdx = i
		}
		if publishIdx == -1 && stepPublishesRelease(s) {
			publishIdx = i
		}
	}
	if smokeIdx == -1 {
		return errorf("no step's run: contains --version (no smoke test found)")
	}
	if publishIdx == -1 {
		return errorf("no step creates or uploads a Release")
	}
	if smokeIdx >= publishIdx {
		return errorf("smoke test step (index %d) does not precede the publish step (index %d)", smokeIdx, publishIdx)
	}
	return nil
}

func errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// --- the real test ---

func TestReleaseWorkflowStructure(t *testing.T) {
	path := releaseWorkflowPath(t)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	wf := parseWorkflow(t, raw)

	var rawMap map[string]any
	if err := yaml.Unmarshal(raw, &rawMap); err != nil {
		t.Fatalf("parsing workflow YAML into a raw map: %v", err)
	}

	t.Run("on block declares push.tags, workflow_dispatch and workflow_call each with a required string tag input", func(t *testing.T) {
		push, ok := wf.On["push"].(map[string]any)
		if !ok {
			t.Fatal("on.push is missing or not a mapping")
		}
		tags, ok := push["tags"].([]any)
		if !ok {
			t.Fatal("on.push.tags is missing or not a sequence")
		}
		if !containsString(tags, "v*") {
			t.Errorf("on.push.tags = %v, want it to contain \"v*\"", tags)
		}

		for _, triggerName := range []string{"workflow_dispatch", "workflow_call"} {
			trigger, ok := wf.On[triggerName].(map[string]any)
			if !ok {
				t.Fatalf("on.%s is missing or not a mapping", triggerName)
			}
			inputs, ok := trigger["inputs"].(map[string]any)
			if !ok {
				t.Fatalf("on.%s.inputs is missing or not a mapping", triggerName)
			}
			tagInput, ok := inputs["tag"].(map[string]any)
			if !ok {
				t.Fatalf("on.%s.inputs.tag is missing", triggerName)
			}
			if required, _ := tagInput["required"].(bool); !required {
				t.Errorf("on.%s.inputs.tag.required = %v, want true", triggerName, tagInput["required"])
			}
			if typ, _ := tagInput["type"].(string); typ != "string" {
				t.Errorf("on.%s.inputs.tag.type = %q, want \"string\"", triggerName, typ)
			}
		}
	})

	t.Run("every uses is SHA-pinned", func(t *testing.T) {
		if err := validateSHAPins(wf); err != nil {
			t.Error(err)
		}
	})

	t.Run("no top-level permissions key", func(t *testing.T) {
		if _, ok := rawMap["permissions"]; ok {
			t.Error("release.yml declares a top-level permissions: key; permissions must be scoped per job")
		}
	})

	releaseJob, ok := wf.Jobs["release"]
	if !ok {
		t.Fatal(`release.yml has no "release" job`)
	}

	t.Run("the release job declares permissions: contents: write at job scope", func(t *testing.T) {
		if got := releaseJob.Permissions["contents"]; got != "write" {
			t.Errorf("release job permissions.contents = %q, want \"write\"", got)
		}
	})

	t.Run("the Go setup step uses go-version-file, never a literal go-version", func(t *testing.T) {
		var setupGo *workflowStep
		for i := range releaseJob.Steps {
			if strings.HasPrefix(releaseJob.Steps[i].Uses, "actions/setup-go@") {
				setupGo = &releaseJob.Steps[i]
				break
			}
		}
		if setupGo == nil {
			t.Fatal("release job has no actions/setup-go step")
		}
		if v, _ := setupGo.With["go-version-file"].(string); v != "go.mod" {
			t.Errorf("actions/setup-go with.go-version-file = %q, want \"go.mod\"", v)
		}
		if _, present := setupGo.With["go-version"]; present {
			t.Error("actions/setup-go declares a literal go-version:, which drifts from go.mod")
		}
	})

	t.Run("the smoke test step precedes the publish step", func(t *testing.T) {
		if err := validateSmokeBeforePublish(releaseJob.Steps); err != nil {
			t.Error(err)
		}
	})

	// docs/plan/tasks/E0-L5.md's E0-L5-T3: the tap-bump job's LOCALLY
	// VERIFIABLE half. Its live half — a real tag push causing this job to
	// run to completion and push Formula/dira.rb — cannot go green until
	// HOMEBREW_TAP_TOKEN exists on kazi-org/dira, and is left for E0-L5-T4
	// to observe, not duplicated here.
	t.Run("the tap-bump job exists, needs release, references HOMEBREW_TAP_TOKEN, and claims no contents:write", func(t *testing.T) {
		tapBump, ok := wf.Jobs["tap-bump"]
		if !ok {
			t.Fatal(`release.yml has no "tap-bump" job`)
		}
		needsRelease := false
		switch needs := tapBump.Needs.(type) {
		case string:
			needsRelease = needs == "release"
		case []any:
			needsRelease = containsString(needs, "release")
		}
		if !needsRelease {
			t.Errorf("tap-bump job needs = %v, want it to include \"release\"", tapBump.Needs)
		}
		if !strings.Contains(string(raw), "secrets.HOMEBREW_TAP_TOKEN") {
			t.Error("release.yml does not reference secrets.HOMEBREW_TAP_TOKEN")
		}
		if got := tapBump.Permissions["contents"]; got == "write" {
			t.Errorf("tap-bump job declares permissions.contents = %q; it writes to a different repo via a PAT and needs no elevated permissions on this one", got)
		}
	})

	// --- both sides: each structural assertion above is proven able to
	// fail against an actual mutated COPY OF THE REAL FILE, per the
	// acc: line's own wording, not a synthetic minimal fixture. ---

	t.Run("both sides: SHA-pin assertion catches a floating tag", func(t *testing.T) {
		mutated := oneSHAPinReplacedWithFloatingTag(t, raw)
		brokenWF := parseWorkflow(t, mutated)
		if err := validateSHAPins(brokenWF); err == nil {
			t.Fatal("expected validateSHAPins to reject a copy with one uses: reverted to @v5, got nil")
		}
		// and it must still pass on the untouched real file — this subtest
		// is worthless if that one is not also observed green in the same
		// run (docs/lore.md L-0001).
		if err := validateSHAPins(wf); err != nil {
			t.Fatalf("validateSHAPins rejected the real, correctly-pinned file: %v", err)
		}
	})

	t.Run("both sides: step-ordering assertion catches a smoke test moved after publish", func(t *testing.T) {
		mutated := smokeTestStepMovedAfterPublish(t, raw)
		brokenWF := parseWorkflow(t, mutated)
		brokenRelease, ok := brokenWF.Jobs["release"]
		if !ok {
			t.Fatal(`mutated copy has no "release" job`)
		}
		if err := validateSmokeBeforePublish(brokenRelease.Steps); err == nil {
			t.Fatal("expected validateSmokeBeforePublish to reject a copy with the smoke test moved after publish, got nil")
		}
		if err := validateSmokeBeforePublish(releaseJob.Steps); err != nil {
			t.Fatalf("validateSmokeBeforePublish rejected the real, correctly-ordered file: %v", err)
		}
	})
}

// oneSHAPinReplacedWithFloatingTag returns a copy of raw with the first
// 40-character commit SHA pin replaced by a floating tag, the exact defect
// class validateSHAPins exists to catch.
func oneSHAPinReplacedWithFloatingTag(t *testing.T, raw []byte) []byte {
	t.Helper()
	pin := regexp.MustCompile(`@[0-9a-f]{40}( #[^\n]*)?`)
	loc := pin.FindIndex(raw)
	if loc == nil {
		t.Fatal("no SHA-pinned uses: found in the real file to mutate — nothing to prove the assertion against")
	}
	mutated := make([]byte, 0, len(raw))
	mutated = append(mutated, raw[:loc[0]]...)
	mutated = append(mutated, []byte("@v5")...)
	mutated = append(mutated, raw[loc[1]:]...)
	return mutated
}

// smokeTestStepMovedAfterPublish returns a copy of raw, parsed and
// re-serialised via yaml.Node so formatting is preserved as closely as
// gopkg.in/yaml.v3 allows, with the release job's smoke-test step and its
// goreleaser-release step swapped — the exact defect class
// validateSmokeBeforePublish exists to catch.
func smokeTestStepMovedAfterPublish(t *testing.T, raw []byte) []byte {
	t.Helper()

	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing workflow as a yaml.Node: %v", err)
	}
	if len(doc.Content) != 1 {
		t.Fatalf("expected exactly one document, got %d", len(doc.Content))
	}
	root := doc.Content[0]

	jobs := mappingValue(t, root, "jobs")
	release := mappingValue(t, jobs, "release")
	steps := mappingValue(t, release, "steps")

	smokeIdx, publishIdx := -1, -1
	for i, stepNode := range steps.Content {
		name := mappingValueOrNil(stepNode, "name")
		run := mappingValueOrNil(stepNode, "run")
		if name != nil && strings.Contains(name.Value, "Smoke test") {
			smokeIdx = i
		}
		if run != nil && goreleaserReleasePattern.MatchString(run.Value) {
			publishIdx = i
		}
	}
	if smokeIdx == -1 || publishIdx == -1 {
		t.Fatalf("could not locate both the smoke-test step (%d) and the publish step (%d) in the real file", smokeIdx, publishIdx)
	}

	steps.Content[smokeIdx], steps.Content[publishIdx] = steps.Content[publishIdx], steps.Content[smokeIdx]

	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("re-marshalling the mutated document: %v", err)
	}
	return out
}

func mappingValue(t *testing.T, m *yaml.Node, key string) *yaml.Node {
	t.Helper()
	v := mappingValueOrNil(m, key)
	if v == nil {
		t.Fatalf("key %q not found in mapping node", key)
	}
	return v
}

func mappingValueOrNil(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func containsString(items []any, want string) bool {
	for _, item := range items {
		if s, ok := item.(string); ok && s == want {
			return true
		}
	}
	return false
}
