package brief_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/brief"
	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/index/indextest"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/fixture"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// clock is the instant every test here renders at.
//
// Fixed, because the note window is measured against now and a brief that
// expired overnight would make these cases fail on a Tuesday for no reason. The
// date is inside the fixture's own span — its entries run from 2026-01-05 — so
// that "fresh notes" is a section with something in it rather than one the
// window empties, which is what makes the drop-order case measure four bands
// instead of three.
var clock = time.Date(2026, time.January, 20, 9, 0, 0, 0, time.UTC)

func fixtureLedger(t *testing.T, n int) string {
	t.Helper()

	diraDir := filepath.Join(t.TempDir(), ".dira")
	if err := os.MkdirAll(filepath.Join(diraDir, "entries"), 0o755); err != nil {
		t.Fatalf("creating %s: %v", diraDir, err)
	}
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("local.Open: %v", err)
	}
	entries, err := fixture.Generate(fixture.Seed, n)
	if err != nil {
		t.Fatalf("fixture.Generate: %v", err)
	}
	if err := fixture.Write(context.Background(), store, entries); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	return diraDir
}

func openIndex(t *testing.T, diraDir string) *index.Index {
	t.Helper()

	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("local.Open: %v", err)
	}
	ix, err := index.Open(context.Background(), store, local.CacheDir(diraDir))
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	return ix
}

func render(t *testing.T, src brief.Source, opts brief.Options) (string, *brief.Result) {
	t.Helper()

	if opts.Now.IsZero() {
		opts.Now = clock
	}
	var b strings.Builder
	result, err := brief.Render(context.Background(), &b, src, opts)
	if err != nil {
		t.Fatalf("brief.Render: %v", err)
	}
	return b.String(), result
}

// TestTheBriefIsTheSameWithAndWithoutACache adds the real brief to E1-L3's
// differential harness, which is what that package was left extensible for.
//
// dec-0002's promise is that deleting .dira/cache/ changes how long an answer
// takes and nothing else about it. The brief is the command where that promise
// is most load-bearing, because it is the one a session start runs on a cache
// that may have been thrown away by a `git clean` five seconds earlier.
func TestTheBriefIsTheSameWithAndWithoutACache(t *testing.T) {
	t.Parallel()

	diraDir := fixtureLedger(t, 200)

	forms := []brief.Options{
		{Now: clock, Ledger: "fixture"},
		{Now: clock, Ledger: "fixture", Context: true, Chain: true},
		{Now: clock, Ledger: "fixture", MaxTokens: 400},
	}

	queries := make([]indextest.Query, 0, len(forms))
	for _, opts := range forms {
		name := "brief"
		if opts.Context {
			name += " --context --chain"
		}
		if opts.MaxTokens > 0 {
			name += " at a lowered ceiling"
		}
		queries = append(queries, indextest.Query{
			Name: name,
			Run: func(ctx context.Context, ix *index.Index) (string, error) {
				var b strings.Builder
				result, err := brief.Render(ctx, &b, ix, opts)
				if err != nil {
					return "", err
				}
				if result.Tokens == 0 {
					return "", errors.New("the brief is empty; the comparison would be empty against empty")
				}
				return b.String(), nil
			},
		})
	}

	indextest.RunTwice(t, diraDir, queries...)
}

// keepOrder is cst-0001's order of importance, named here rather than read back
// out of the code under test.
//
// This is the difference between a gate and a mirror. Asserting that sections
// degrade consistently with whatever order brief.sections() declares passes just
// as happily when that order is reversed — which is what an earlier version of
// this test did, and a mutation that inverted the order ran green through it.
var keepOrder = []string{"open blockers", "current focus", "recent decisions", "fresh notes"}

// TestTheDropOrderIsTheKeepOrderReversed is cst-0001's priority rule at the
// package level, where the arithmetic is visible.
//
// The property is that what survives the ceiling is a prefix of the keep order:
// once a section holds fewer entries than the ledger has for it, every section
// after it holds none.
func TestTheDropOrderIsTheKeepOrderReversed(t *testing.T) {
	t.Parallel()

	ix := openIndex(t, fixtureLedger(t, 200))

	_, whole := render(t, ix, brief.Options{MaxTokens: 100000})
	total := map[string]int{}
	for _, s := range whole.Sections {
		total[s.Name] = s.Kept
	}
	for _, name := range keepOrder {
		if total[name] == 0 {
			t.Fatalf("the fixture has no entries for %q, so the priority order is not under test", name)
		}
	}

	for _, ceiling := range []int{1500, 1200, 900, 700, 500, 400, 300} {
		_, result := render(t, ix, brief.Options{MaxTokens: ceiling})
		if result.Tokens > ceiling {
			t.Errorf("ceiling %d produced %d tokens", ceiling, result.Tokens)
		}

		kept := map[string]int{}
		for _, s := range result.Sections {
			kept[s.Name] = s.Kept
			if s.Total != total[s.Name] {
				t.Errorf("at ceiling %d, %q reports %d entries in the ledger and the uncapped brief found %d",
					ceiling, s.Name, s.Total, total[s.Name])
			}
		}

		short := ""
		for _, name := range keepOrder {
			switch {
			case short != "" && kept[name] > 0:
				t.Errorf("at ceiling %d, %q kept %d entries although %q kept only %d of %d — "+
					"the low-priority end is not what is being dropped",
					ceiling, name, kept[name], short, kept[short], total[short])
			case kept[name] < total[name]:
				short = name
			}
		}
	}
}

// TestNotesDecayOutOfTheBrief is cst-0002's pressure-valve clause, as far as a
// read verb can honestly implement it: a note surfaces while it is fresh and
// then stops, so the valve does not become a sixth store.
//
// It is written against a hand-built ledger rather than the fixture because the
// fixture's dates are fixed and this case is about the passage of time.
func TestNotesDecayOutOfTheBrief(t *testing.T) {
	t.Parallel()

	diraDir := filepath.Join(t.TempDir(), ".dira")
	writeEntries(t, diraDir, map[string]string{
		"note-0001.md": entryFile("note-0001", "note", "A thought from this morning", "active",
			clock.Add(-2*time.Hour)),
		"note-0002.md": entryFile("note-0002", "note", "A thought from a month ago", "active",
			clock.Add(-30*24*time.Hour)),
	})
	ix := openIndex(t, diraDir)

	out, _ := render(t, ix, brief.Options{})
	if !strings.Contains(out, "note-0001") {
		t.Errorf("a note written two hours ago is not in the brief:\n%s", out)
	}
	if strings.Contains(out, "note-0002") {
		t.Errorf("a note written a month ago is still in the brief; the valve has become a store (cst-0002):\n%s", out)
	}

	// And the window is a window rather than a filter that drops everything:
	// rendering as though it were a month ago brings the old note back.
	past, _ := render(t, ix, brief.Options{Now: clock.Add(-30 * 24 * time.Hour).Add(time.Hour)})
	if !strings.Contains(past, "note-0002") {
		t.Errorf("the note is missing from a brief rendered the day it was written:\n%s", past)
	}
}

// TestASectionWithNothingInItSaysSo. An absent section and a recorded absence
// read completely differently — internal/why made the same call for an entry
// with no alternatives, and for the same reason.
func TestASectionWithNothingInItSaysSo(t *testing.T) {
	t.Parallel()

	diraDir := filepath.Join(t.TempDir(), ".dira")
	writeEntries(t, diraDir, map[string]string{
		"int-0001.md": entryFile("int-0001", "intent", "The one thing this ledger is for", "active", clock),
	})
	ix := openIndex(t, diraDir)

	out, result := render(t, ix, brief.Options{})
	for _, want := range []string{"open blockers", "none", "current focus", "int-0001"} {
		if !strings.Contains(out, want) {
			t.Errorf("the brief is missing %q:\n%s", want, out)
		}
	}
	if result.Omitted() != 0 {
		t.Errorf("a brief that fitted reports %d omitted entries", result.Omitted())
	}
	if strings.Contains(out, "omitted") {
		t.Errorf("a brief that omitted nothing printed an omission notice:\n%s", out)
	}
}

// TestTheReportedCountIsTheWrittenCount. Result.Tokens is what `dira brief`
// prints to stderr and what E1-L6 will measure against; a count that described
// something other than the bytes written would make every assertion above
// self-referential.
func TestTheReportedCountIsTheWrittenCount(t *testing.T) {
	t.Parallel()

	ix := openIndex(t, fixtureLedger(t, 200))
	out, result := render(t, ix, brief.Options{MaxTokens: 900})

	if got := brief.Tokens(out); got != result.Tokens {
		t.Errorf("the brief reports %d tokens and its output counts %d", result.Tokens, got)
	}
	if result.Tokens > 900 {
		t.Errorf("the brief is %d tokens against a 900 ceiling", result.Tokens)
	}
}

// TestASourceThatFailsIsAnError. Fail-open is about the ledger's *contents* —
// a malformed entry, a missing title. A read surface that cannot answer at all
// is a real failure and must not be rendered as an empty brief, which would tell
// a session that the ledger is empty when it is unreadable.
func TestASourceThatFailsIsAnError(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	_, err := brief.Render(context.Background(), &b, brokenSource{}, brief.Options{Now: clock})
	if err == nil {
		t.Fatal("a source that cannot answer produced a brief instead of an error")
	}
	if b.Len() != 0 {
		t.Errorf("a failed render wrote %q to the page", b.String())
	}
}

type brokenSource struct{}

func (brokenSource) Select(context.Context, index.Selector) ([]index.Ref, error) {
	return nil, errors.New("the ledger cannot be read")
}

func (brokenSource) Entry(context.Context, string) (*ledger.Entry, error) {
	return nil, errors.New("the ledger cannot be read")
}

// writeEntries materialises literal entry files. Literal rather than generated,
// because these cases are about shapes the generator does not produce.
func writeEntries(t *testing.T, diraDir string, files map[string]string) {
	t.Helper()

	entries := filepath.Join(diraDir, "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatalf("creating %s: %v", entries, err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(entries, name), []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
}

func entryFile(id, kind, title, state string, created time.Time) string {
	body := "---\nid: " + id + "\nkind: " + kind + "\ntitle: " + title + "\nstate: " + state +
		"\ncreated: \"" + created.Format(time.RFC3339) + "\"\n"
	if kind == "decision" {
		body += "alternatives:\n  - option: Not doing it\n    why_not: it would not have served the intent\n"
	}
	return body + "---\n\nBody.\n"
}
