package sniff

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
	"github.com/kazi-org/dira/schema"
)

// tempLedger builds an empty ledger in a temp directory and returns the store
// and the entries directory, so a test can assert on the files rather than only
// on what the API said it wrote.
func tempLedger(t *testing.T) (ledger.Store, string) {
	t.Helper()

	root := t.TempDir()
	dira := filepath.Join(root, ".dira")
	if err := os.MkdirAll(filepath.Join(dira, "entries"), 0o755); err != nil {
		t.Fatalf("creating the temp ledger: %v", err)
	}
	store, err := local.Open(dira)
	if err != nil {
		t.Fatalf("opening the temp ledger: %v", err)
	}
	return store, filepath.Join(dira, "entries")
}

func stamp(t *testing.T) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, "2026-07-30T09:15:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return at
}

// TestStagedEntriesAreTheOnlyThingWritten is this lane's acceptance line,
// asserted against the files on disk rather than against the values passed in.
//
// It reads every entry back through dira's own codec and validates it against
// the published schema, because the claim is not "Stage returned entries with
// state staged" — it is that nothing else can be in the ledger afterwards.
func TestStagedEntriesAreTheOnlyThingWritten(t *testing.T) {
	t.Parallel()

	validator, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	positives := []string{
		"stop-design-direction.jsonl",
		"stop-storage-interface.jsonl",
		"pre-compact.jsonl",
	}

	total := 0
	for _, name := range positives {
		t.Run(name, func(t *testing.T) {
			store, dir := tempLedger(t)
			candidates, err := SniffTranscript(openTranscript(t, name), Whole)
			if err != nil {
				t.Fatalf("SniffTranscript: %v", err)
			}
			if len(candidates) == 0 {
				t.Fatalf("%s yielded no candidates; the acceptance line requires at least one staged entry "+
					"from a recorded transcript containing decision language", name)
			}

			result, err := Stage(context.Background(), store, StageOptions{
				Hook: ledger.HookStop, Session: "fixture", Now: stamp(t),
			}, candidates)
			if err != nil {
				t.Fatalf("Stage: %v", err)
			}
			if len(result.Staged) == 0 {
				t.Fatal("Stage wrote nothing")
			}
			total += len(result.Staged)

			files, err := filepath.Glob(filepath.Join(dir, "*.md"))
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != len(result.Staged) {
				t.Fatalf("%d files on disk, %d entries reported staged (dec-0002: one file per entry)",
					len(files), len(result.Staged))
			}

			for _, path := range files {
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := validator.Validate(content); err != nil {
					t.Errorf("%s does not validate against entry.schema.json: %v", filepath.Base(path), err)
				}
				e, err := ledger.Decode(content)
				if err != nil {
					t.Fatalf("%s: %v", filepath.Base(path), err)
				}

				if e.State != ledger.StateStaged {
					t.Errorf("%s: state = %q, want %q", e.ID, e.State, ledger.StateStaged)
				}
				for _, forbidden := range []ledger.State{ledger.StateAccepted, ledger.StateActive, ledger.StateOpen} {
					if e.State == forbidden {
						t.Errorf("%s: state = %q, which the regex tier may never write", e.ID, forbidden)
					}
				}
				if e.Source == nil {
					t.Fatalf("%s: no source block, so nothing records that a regex inferred it", e.ID)
				}
				if e.Source.Tier != ledger.TierRegex {
					t.Errorf("%s: source.tier = %q, want %q", e.ID, e.Source.Tier, ledger.TierRegex)
				}
				if e.Source.Hook != ledger.HookStop {
					t.Errorf("%s: source.hook = %q, want %q", e.ID, e.Source.Hook, ledger.HookStop)
				}
				if strings.TrimSpace(e.Source.Excerpt) == "" {
					t.Errorf("%s: empty excerpt — a human cannot dispose of an entry with no evidence", e.ID)
				}
				if n := len([]rune(e.Source.Excerpt)); n > maxExcerpt {
					t.Errorf("%s: excerpt is %d characters, want at most %d", e.ID, n, maxExcerpt)
				}
				if len(e.Alternatives) != 0 {
					t.Errorf("%s: carries %d alternative(s); a regex may not assert what was rejected", e.ID, len(e.Alternatives))
				}
				for i, alt := range e.Alternatives {
					if alt.WhyNot != "" {
						t.Errorf("%s: alternatives[%d].why_not = %q, which no regex can know", e.ID, i, alt.WhyNot)
					}
				}
				if e.ConfirmedBy != "" {
					t.Errorf("%s: confirmed_by = %q, but nobody confirmed it", e.ID, e.ConfirmedBy)
				}
				if e.Kind != ledger.KindDecision {
					t.Errorf("%s: kind = %q; the regex tier writes decisions only", e.ID, e.Kind)
				}
			}
		})
	}

	if total == 0 {
		t.Fatal("no entry was staged by any transcript; every assertion above ran zero times")
	}
	t.Logf("OBSERVED  %d entries staged across %d recorded transcripts, every one staged/regex/Stop", total, len(positives))
}

// TestTheNegativeFixtureWritesNothing is the other half of the acceptance line.
//
// no-decisions.jsonl is not unrelated text. Every line in it is a deliberate
// near-miss: "we could go with", "suppose we don't do", "one option is", a
// question, a citation of two real ledger entries, a "let me", a deferral, and a
// block of tool output. That is what the lane's risk line asks for — a fixture
// full of hypotheticals is the guard, and one made of lorem ipsum would prove
// nothing.
func TestTheNegativeFixtureWritesNothing(t *testing.T) {
	t.Parallel()

	store, dir := tempLedger(t)
	candidates, err := SniffTranscript(openTranscript(t, "no-decisions.jsonl"), Whole)
	if err != nil {
		t.Fatalf("SniffTranscript: %v", err)
	}
	if len(candidates) != 0 {
		for _, c := range candidates {
			t.Errorf("staged a near-miss  [%s]  %q", c.Rule, c.Excerpt)
		}
		t.Fatalf("no-decisions.jsonl yielded %d candidates, want 0", len(candidates))
	}

	result, err := Stage(context.Background(), store, StageOptions{
		Hook: ledger.HookStop, Now: stamp(t),
	}, candidates)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if len(result.Staged) != 0 {
		t.Errorf("Stage wrote %d entries from a fixture with no decision language", len(result.Staged))
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("%d files written: %v", len(files), files)
	}
}

// TestTheSameTurnStagesOnce covers the failure the Stop hook would otherwise
// produce on every session.
//
// Stop fires after every turn, against a transcript that only grows. A sniffer
// with no memory would re-propose the first turn's decision on turn two, three
// and four, and the distill queue would fill with copies of one sentence — which
// is the noise failure this lane's risk line describes, arriving by a route the
// corpus cannot see.
func TestTheSameTurnStagesOnce(t *testing.T) {
	t.Parallel()

	store, dir := tempLedger(t)
	opts := StageOptions{Hook: ledger.HookStop, Now: stamp(t)}

	candidates, err := SniffTranscript(openTranscript(t, "pre-compact.jsonl"), Whole)
	if err != nil {
		t.Fatalf("SniffTranscript: %v", err)
	}

	first, err := Stage(context.Background(), store, opts, candidates)
	if err != nil {
		t.Fatalf("first Stage: %v", err)
	}
	if len(first.Staged) == 0 {
		t.Fatal("the first run staged nothing, so the second proves nothing")
	}

	second, err := Stage(context.Background(), store, opts, candidates)
	if err != nil {
		t.Fatalf("second Stage: %v", err)
	}
	if len(second.Staged) != 0 {
		t.Errorf("the second run staged %d entries the ledger already held", len(second.Staged))
	}
	if second.Duplicates != len(candidates) {
		t.Errorf("second run reported %d duplicates over %d candidates", second.Duplicates, len(candidates))
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(first.Staged) {
		t.Errorf("%d files after two runs, want %d", len(files), len(first.Staged))
	}
}

// TestEveryRecordedTranscriptStaysUnderTheBound is the volume check the lane's
// risk line asks for, stated as a number rather than an intention.
func TestEveryRecordedTranscriptStaysUnderTheBound(t *testing.T) {
	t.Parallel()

	for _, name := range recordedTranscripts(t) {
		candidates, err := SniffTranscript(openTranscript(t, name), Whole)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		t.Logf("OBSERVED  %-32s %d candidate(s)", name, len(candidates))
		if len(candidates) > maxCandidates {
			t.Errorf("%s yielded %d candidates, over the per-run bound of %d", name, len(candidates), maxCandidates)
		}
	}
}
