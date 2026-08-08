package sniff

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
)

// The tests here are about the one invariant in this lane's title: sniff may
// only ever stage. Each of them fails against a deliberately illegitimate entry,
// which is the point — a check that cannot fail its premise is not evidence for
// it.

// TestTheStoreRefusesEverythingButStaging drives the wrapper directly with
// entries a regex has no business writing.
//
// Every case here is a valid ledger entry: Entry.Validate accepts all of them,
// and `dira log` would write any of them without complaint. That is what makes
// the test worth having. The wrapper is not a second validator; it encodes what
// THIS TIER may write, and the difference between a legitimate accepted decision
// and an accepted decision a regex wrote is invisible to the schema.
func TestTheStoreRefusesEverythingButStaging(t *testing.T) {
	t.Parallel()

	staged := func() *ledger.Entry {
		return &ledger.Entry{
			Kind:    ledger.KindDecision,
			Title:   "We are going with the derived cache",
			State:   ledger.StateStaged,
			Created: "2026-07-30T09:15:00Z",
			Source: &ledger.Source{
				Hook:    ledger.HookStop,
				Excerpt: "We're going with the derived cache rather than a status field on the entry.",
				Tier:    ledger.TierRegex,
			},
		}
	}

	// The control. If this is refused, every refusal below is meaningless
	// because the wrapper refuses everything.
	t.Run("a staged regex entry is accepted", func(t *testing.T) {
		if err := mustStage(staged()); err != nil {
			t.Fatalf("the legitimate entry was refused: %v", err)
		}
	})

	cases := []struct {
		name string
		want string
		mut  func(*ledger.Entry)
	}{
		{
			name: "accepted", want: "state",
			mut: func(e *ledger.Entry) {
				e.State = ledger.StateAccepted
				e.Alternatives = []ledger.Alternative{{Option: "A status field", WhyNot: "It goes stale"}}
			},
		},
		{name: "rejected", want: "state", mut: func(e *ledger.Entry) {
			e.State = ledger.StateRejected
			e.Alternatives = []ledger.Alternative{{Option: "A status field", WhyNot: "It goes stale"}}
		}},
		{name: "superseded", want: "state", mut: func(e *ledger.Entry) {
			e.State = ledger.StateSuperseded
			e.Alternatives = []ledger.Alternative{{Option: "A status field", WhyNot: "It goes stale"}}
		}},
		{name: "an open question", want: "kind", mut: func(e *ledger.Entry) {
			e.Kind = ledger.KindQuestion
			e.State = ledger.StateOpen
		}},
		{name: "an active intent", want: "kind", mut: func(e *ledger.Entry) {
			e.Kind = ledger.KindIntent
			e.State = ledger.StateActive
		}},
		{name: "a human tier", want: "source.tier", mut: func(e *ledger.Entry) { e.Source.Tier = ledger.TierHuman }},
		{name: "a semantic tier", want: "source.tier", mut: func(e *ledger.Entry) { e.Source.Tier = ledger.TierSemantic }},
		{name: "no source at all", want: "source.tier", mut: func(e *ledger.Entry) { e.Source = nil }},
		{name: "an import hook", want: "source.hook", mut: func(e *ledger.Entry) { e.Source.Hook = ledger.HookImport }},
		{name: "no excerpt", want: "excerpt", mut: func(e *ledger.Entry) { e.Source.Excerpt = "  " }},
		{name: "an unbounded excerpt", want: "excerpt", mut: func(e *ledger.Entry) {
			e.Source.Excerpt = strings.Repeat("a", maxExcerpt+1)
		}},
		{name: "confirmed by a human", want: "confirmed_by", mut: func(e *ledger.Entry) { e.ConfirmedBy = "human" }},
		{name: "confirmed by an agent", want: "confirmed_by", mut: func(e *ledger.Entry) { e.ConfirmedBy = "agent:dira-sniff" }},
		{name: "an invented alternative", want: "alternative", mut: func(e *ledger.Entry) {
			e.Alternatives = []ledger.Alternative{{Option: "A status field", WhyNot: "It would go stale"}}
		}},
		{name: "an inferred edge", want: "edge", mut: func(e *ledger.Entry) {
			e.Edges = []ledger.Edge{{Type: ledger.EdgeDerivesFrom, To: "int-0001"}}
		}},
		{name: "a mirrored adr", want: "adr", mut: func(e *ledger.Entry) { e.ADR = "docs/adr/0001-cache.md" }},
		{name: "marked private", want: "private", mut: func(e *ledger.Entry) { e.Private = true }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := staged()
			tc.mut(e)

			// The entry is legitimate as far as the ledger is
			// concerned; only this tier may not write it. Asserting
			// that here is what stops the wrapper from quietly
			// degrading into a copy of Entry.Validate.
			if tc.name != "no source at all" && tc.name != "an unbounded excerpt" {
				probe := *e
				probe.ID = ledger.FormatID(probe.Kind, 1)
				if err := probe.Validate(); err != nil {
					t.Fatalf("the fixture is not a valid ledger entry, so refusing it proves nothing: %v", err)
				}
			}

			err := mustStage(e)
			if err == nil {
				t.Fatalf("the regex tier was allowed to write %s", tc.name)
			}
			if !errors.Is(err, errRefused) {
				t.Errorf("error does not wrap errRefused: %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not name %q, so a reader cannot tell what was wrong: %v", tc.want, err)
			}
		})
	}
}

// TestTheStoreRefusesReplacementAndDeletion covers the two verbs that are not
// creation. Put and Delete are refused unconditionally, so no future edit to
// this package can reach a confirmed entry through them either.
func TestTheStoreRefusesReplacementAndDeletion(t *testing.T) {
	t.Parallel()

	store, _ := tempLedger(t)
	guarded := stagedOnly{inner: store}
	ctx := context.Background()

	e := &ledger.Entry{
		ID: "dec-0001", Kind: ledger.KindDecision, State: ledger.StateStaged,
		Title: "Something already in the ledger", Created: "2026-07-30T09:15:00Z",
	}
	if err := store.Create(ctx, e); err != nil {
		t.Fatalf("seeding the ledger: %v", err)
	}

	if err := guarded.Put(ctx, e); !errors.Is(err, errRefused) {
		t.Errorf("Put returned %v, want a refusal", err)
	}
	if err := guarded.Delete(ctx, "dec-0001"); !errors.Is(err, errRefused) {
		t.Errorf("Delete returned %v, want a refusal", err)
	}

	// And the file is still there, unchanged — the refusal is not a report
	// about a write that happened anyway.
	if _, err := store.Get(ctx, "dec-0001"); err != nil {
		t.Errorf("the entry is gone after a refused Delete: %v", err)
	}
}

// TestStageRefusesAHookThatIsNotACapturePoint pins the other end of the
// provenance contract. SessionStart injects a brief and captures nothing;
// kazi-disposition and import are other lanes' provenance, and an entry claiming
// one of them would misattribute where it came from.
func TestStageRefusesAHookThatIsNotACapturePoint(t *testing.T) {
	t.Parallel()

	store, _ := tempLedger(t)
	candidates := Sniff("We're going with the derived cache rather than a status field on the entry.")
	if len(candidates) == 0 {
		t.Fatal("the fixture sentence no longer matches, so this test would pass for the wrong reason")
	}

	for _, hook := range []ledger.Hook{ledger.HookSessionStart, ledger.HookImport, ledger.HookKaziDisposition, ledger.Hook("")} {
		_, err := Stage(context.Background(), store, StageOptions{Hook: hook, Now: time.Now()}, candidates)
		if !errors.Is(err, errRefused) {
			t.Errorf("hook %q returned %v, want a refusal", hook, err)
		}
	}

	// And the legitimate ones are not refused, so the test above is not
	// passing because Stage refuses everything.
	for _, hook := range []ledger.Hook{ledger.HookStop, ledger.HookPreCompact, ledger.HookManual} {
		store, _ := tempLedger(t)
		if _, err := Stage(context.Background(), store, StageOptions{Hook: hook, Now: time.Now()}, candidates); err != nil {
			t.Errorf("hook %q was refused: %v", hook, err)
		}
	}
}

// TestTheSourceCannotExpressAConfirmedEntry is the belt to the wrapper's braces.
//
// It parses this package's non-test sources and fails if any of them mentions a
// state or a field the tier may not write. It is deliberately the weakest of the
// three guarantees — a string can always be assembled at runtime — and it is
// here because it fails at review time, in the diff, where the other two fail at
// run time.
func TestTheSourceCannotExpressAConfirmedEntry(t *testing.T) {
	t.Parallel()

	forbidden := map[string]string{
		"StateAccepted":   "the regex tier may never write an accepted entry (dec-0003)",
		"StateRejected":   "rejecting an option is a human's act, and E2-L4 owns the transition",
		"StateSuperseded": "supersession is a claim about two entries that a pattern cannot make",
		"StateActive":     "the tier writes decisions, and no decision is ever active",
		"StateOpen":       "an open question no human saw is the ledger asserting a blocker nobody has",
		"TierHuman":       "an inference wearing a human's confidence is what provenance exists to prevent",
		"TierSemantic":    "tier 2 runs in the live session, not in this package",
	}

	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		checked++
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if why, bad := forbidden[sel.Sel.Name]; bad {
				t.Errorf("%s references ledger.%s — %s", fset.Position(sel.Pos()), sel.Sel.Name, why)
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no non-test source files were parsed; this check is not measuring anything")
	}

	// The negative control: the same walk over this file must find the
	// names, or the walk is looking in the wrong place.
	src, err := os.ReadFile("guard_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "StateAccepted") {
		t.Fatal("the control string is gone; the AST walk above could no longer fail")
	}
	file, err := parser.ParseFile(fset, "guard_test.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "StateAccepted" {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("the AST walk does not find ledger.StateAccepted in a file that demonstrably contains it")
	}
}
