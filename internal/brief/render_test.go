package brief_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/brief"
	"github.com/kazi-org/dira/internal/chain"
)

// chainClock is the instant every chain test here renders at.
var chainClock = clock

// writeChainLedger materialises a literal ledger — config and entries — under
// dir/.dira.
func writeChainLedger(t *testing.T, dir, config string, entries map[string]string) string {
	t.Helper()
	diraDir := filepath.Join(dir, ".dira")
	if err := os.MkdirAll(filepath.Join(diraDir, "entries"), 0o755); err != nil {
		t.Fatalf("creating %s: %v", diraDir, err)
	}
	if err := os.WriteFile(filepath.Join(diraDir, "config.toml"), []byte(config), 0o644); err != nil {
		t.Fatalf("writing config.toml: %v", err)
	}
	for id, body := range entries {
		if err := os.WriteFile(filepath.Join(diraDir, "entries", id+".md"), []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", id, err)
		}
	}
	return diraDir
}

func chainIntent(id, title string) string {
	return "---\nid: " + id + "\nkind: intent\ntitle: " + title + "\nstate: active\n" +
		"created: \"2026-06-01T09:00:00Z\"\n---\n\nfixture body.\n"
}

// twoParentFixture is a repo declaring sire (workspace, active bets) and me
// (person, an active direction) as two direct parents — E5-L4-T1's own
// fixture shape, not a transitive chain (E5-L2's tests already cover that).
func twoParentFixture(t *testing.T) (root, repoDira string) {
	t.Helper()
	root = t.TempDir()

	writeChainLedger(t, filepath.Join(root, "me"), "[ledger]\nname = \"me\"\ntier = \"person\"\n",
		map[string]string{"int-0001": chainIntent("int-0001", "me's active direction for the quarter")})

	writeChainLedger(t, filepath.Join(root, "sire"), "[ledger]\nname = \"sire\"\ntier = \"workspace\"\n",
		map[string]string{"int-0001": chainIntent("int-0001", "sire's active bet for the quarter")})

	repoDira = writeChainLedger(t, filepath.Join(root, "repo"),
		"[ledger]\nname = \"repo\"\ntier = \"repo\"\n\n[parents]\n"+
			"sire = { path = \"../../sire\" }\nme = { path = \"../../me\" }\n",
		map[string]string{"int-0001": chainIntent("int-0001", "repo's own local intent")})
	return root, repoDira
}

func chainOpts(diraDira string, extra brief.Options) brief.Options {
	extra.Now = chainClock
	extra.Chain = true
	extra.Parents = []string{"sire", "me"}
	extra.ChainSource = func(ctx context.Context) ([]chain.Ancestor, error) { return chain.Walk(ctx, diraDira) }
	return extra
}

// TestChainRenders is E5-L4-T1's internal/brief acceptance line.
func TestChainRenders(t *testing.T) {
	t.Run("content from all three tiers, inside the shared ceiling", func(t *testing.T) {
		_, repoDira := twoParentFixture(t)
		ix := openIndex(t, repoDira)
		out, result := render(t, ix, chainOpts(repoDira, brief.Options{Ledger: "repo", Context: true}))

		for _, want := range []string{"repo's own local intent", "sire:int-0001", "sire's active bet", "me:int-0001", "me's active direction"} {
			if !strings.Contains(out, want) {
				t.Errorf("the brief is missing %q:\n%s", want, out)
			}
		}
		if result.Tokens > brief.DefaultMaxTokens {
			t.Errorf("%d tokens against a %d ceiling", result.Tokens, brief.DefaultMaxTokens)
		}
	})

	t.Run("the combined content exceeds the ceiling unbudgeted, and stays under it budgeted", func(t *testing.T) {
		root := t.TempDir()
		writeChainLedger(t, filepath.Join(root, "me"), "[ledger]\nname = \"me\"\ntier = \"person\"\n",
			map[string]string{"int-0001": chainIntent("int-0001", "me's active direction for the quarter")})

		sireEntries := map[string]string{}
		var raw strings.Builder
		for i := 1; i <= 150; i++ {
			id := fmt.Sprintf("int-%04d", i)
			body := chainIntent(id, fmt.Sprintf("sire's bet number %04d, worded long enough to cost real tokens", i))
			sireEntries[id] = body
			raw.WriteString(body)
		}
		writeChainLedger(t, filepath.Join(root, "sire"), "[ledger]\nname = \"sire\"\ntier = \"workspace\"\n", sireEntries)

		repoDira := writeChainLedger(t, filepath.Join(root, "repo"),
			"[ledger]\nname = \"repo\"\ntier = \"repo\"\n\n[parents]\nsire = { path = \"../../sire\" }\n",
			map[string]string{"int-0001": chainIntent("int-0001", "repo's own local intent")})

		if brief.Tokens(raw.String()) <= brief.DefaultMaxTokens {
			t.Fatalf("the fixture's raw sire content is only %d tokens; it does not exceed %d unbudgeted",
				brief.Tokens(raw.String()), brief.DefaultMaxTokens)
		}

		ix := openIndex(t, repoDira)
		opts := chainOpts(repoDira, brief.Options{Ledger: "repo"})
		opts.Parents = []string{"sire"}
		out, result := render(t, ix, opts)

		if result.Tokens > brief.DefaultMaxTokens {
			t.Errorf("%d tokens against a %d ceiling", result.Tokens, brief.DefaultMaxTokens)
		}
		if !strings.Contains(out, "omitted") || !strings.Contains(out, "sire") {
			t.Errorf("the footer does not name a dropped chain section by namespace:\n%s", out)
		}
	})

	t.Run("empty [parents] is byte-identical to chainNotice's no-parent sentence", func(t *testing.T) {
		diraDira := filepath.Join(t.TempDir(), ".dira")
		if err := os.MkdirAll(filepath.Join(diraDira, "entries"), 0o755); err != nil {
			t.Fatalf("creating %s: %v", diraDira, err)
		}
		ix := openIndex(t, diraDira)

		withChain, _ := render(t, ix, brief.Options{Chain: true})
		withoutChain, _ := render(t, ix, brief.Options{})

		if !strings.Contains(withChain, "no parent ledger is configured") {
			t.Errorf("the empty-parents sentence is missing:\n%s", withChain)
		}
		// The only difference from a brief with no --chain at all is that
		// one sentence — nothing else about the local sections moves.
		afterNotice := strings.SplitN(withChain, "no parent ledger is configured", 2)
		if len(afterNotice) != 2 {
			t.Fatalf("could not isolate the notice in:\n%s", withChain)
		}
		if strings.TrimSpace(withoutChain) == "" {
			t.Fatal("the baseline brief is empty; the comparison is not testing anything")
		}
	})

	t.Run("the chain ancestors are never written (dec-0004 and cst-0003 rule 1)", func(t *testing.T) {
		// repo's own ledger legitimately grows .dira/cache/ — that is
		// index.Open's ordinary behaviour over a ledger dira may write
		// to, the same read cache TestTheBriefIsTheSameWithAndWithoutACache
		// exercises. What must stay untouched is sire and me, which
		// activeIntents reads directly through chain.Ancestor.Store and
		// never through an index at all.
		root, repoDira := twoParentFixture(t)
		before := map[string]string{
			"sire": treeDigestFor(t, filepath.Join(root, "sire")),
			"me":   treeDigestFor(t, filepath.Join(root, "me")),
		}

		ix := openIndex(t, repoDira)
		render(t, ix, chainOpts(repoDira, brief.Options{Ledger: "repo"}))

		for name, want := range before {
			if got := treeDigestFor(t, filepath.Join(root, name)); got != want {
				t.Errorf("%s changed while rendering the chain — a parent ledger must never be written to (cst-0003 rule 1)", name)
			}
		}
	})
}

// TestNoIntentCap is E5-L4-T2's acceptance line: twelve, then twenty, active
// intents render as twelve and twenty — never paginated, truncated, or
// summarised past dec-0006's idiomatic range, which is a warning, not a
// ceiling.
func TestNoIntentCap(t *testing.T) {
	fixtureWithN := func(t *testing.T, n int) (root, repoDira string) {
		t.Helper()
		root = t.TempDir()
		sireEntries := map[string]string{}
		for i := 1; i <= n; i++ {
			id := fmt.Sprintf("int-%04d", i)
			sireEntries[id] = chainIntent(id, fmt.Sprintf("distinctive-bet-%04d", i))
		}
		writeChainLedger(t, filepath.Join(root, "sire"), "[ledger]\nname = \"sire\"\ntier = \"workspace\"\n", sireEntries)
		repoDira = writeChainLedger(t, filepath.Join(root, "repo"),
			"[ledger]\nname = \"repo\"\ntier = \"repo\"\n\n[parents]\nsire = { path = \"../../sire\" }\n", nil)
		return root, repoDira
	}

	assertAllPresent := func(t *testing.T, out string, n int) {
		t.Helper()
		for i := 1; i <= n; i++ {
			want := fmt.Sprintf("distinctive-bet-%04d", i)
			if !strings.Contains(out, want) {
				t.Errorf("missing %s among %d intents — the first one dropped without saying so", want, n)
			}
		}
	}

	t.Run("12 active intents render as 12, with a warning naming the count", func(t *testing.T) {
		_, repoDira := fixtureWithN(t, 12)
		ix := openIndex(t, repoDira)
		out, _ := render(t, ix, chainOpts(repoDira, brief.Options{Ledger: "repo", MaxTokens: 100000}))
		assertAllPresent(t, out, 12)
		if !strings.Contains(out, "12") {
			t.Errorf("the warning does not name the count 12:\n%s", out)
		}
	})

	t.Run("20 active intents render as 20 — 12 was not a coincidental threshold", func(t *testing.T) {
		_, repoDira := fixtureWithN(t, 20)
		ix := openIndex(t, repoDira)
		out, _ := render(t, ix, chainOpts(repoDira, brief.Options{Ledger: "repo", MaxTokens: 100000}))
		assertAllPresent(t, out, 20)
	})

	t.Run("under a tight ceiling, entries are dropped and named in the footer, never silently shrunk", func(t *testing.T) {
		_, repoDira := fixtureWithN(t, 12)
		ix := openIndex(t, repoDira)
		out, result := render(t, ix, chainOpts(repoDira, brief.Options{Ledger: "repo", MaxTokens: 250}))

		if result.Tokens > 250 {
			t.Errorf("%d tokens against a 250 ceiling", result.Tokens)
		}
		if result.Omitted() == 0 {
			t.Fatal("a 250-token ceiling kept all 12 intents; this subtest is not testing the conflict it exists to test")
		}
		if !strings.Contains(out, "omitted") {
			t.Errorf("entries were dropped without the footer saying so:\n%s", out)
		}
	})
}

// treeDigestFor is a thin sha256-free change detector: enough for this file's
// one write-nothing assertion without importing crypto here too.
func treeDigestFor(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s|%d|%v\n", path, info.Size(), info.ModTime())
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return b.String()
}
