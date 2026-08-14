package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/brief"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/fixture"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// exerciseBrief runs `dira brief` through the real dispatcher and the real
// exit-code mapping.
//
// The command is appended to the registry here rather than in main.go because
// several lanes are editing that file concurrently; registering it is one line
// the integrator merges. Appending only when it is absent means this test keeps
// working unchanged once that line lands, and it exercises a.main either way —
// so the exit codes asserted below are the contract the SessionStart hook
// depends on rather than a local re-derivation of it.
func exerciseBrief(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errBuf bytes.Buffer
	a := newApp(&out, &errBuf)
	if a.lookup("brief") == nil {
		a.commands = append(a.commands, &command{
			name:    "brief",
			summary: briefSummary,
			run:     runBrief,
			usage:   writeBriefUsage,
		})
	}
	code = a.main(append([]string{"brief"}, args...))
	return code, out.String(), errBuf.String()
}

// fixtureLedger materialises the shared 200-entry fixture — the ledger E1's
// acceptance lines are written against — and returns the directory to run in.
//
// config is written verbatim into .dira/config.toml, so a test can lower the
// ceiling the way a user would rather than through a flag the command does not
// have.
func briefLedger(t *testing.T, config string) string {
	t.Helper()

	root := t.TempDir()
	diraDir := filepath.Join(root, ".dira")
	if err := os.MkdirAll(filepath.Join(diraDir, "entries"), 0o755); err != nil {
		t.Fatalf("creating the ledger: %v", err)
	}
	if config != "" {
		if err := os.WriteFile(filepath.Join(diraDir, "config.toml"), []byte(config), 0o644); err != nil {
			t.Fatalf("writing config.toml: %v", err)
		}
	}

	entries, err := fixture.Generate(fixture.Seed, fixture.Size)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("local.Open: %v", err)
	}
	if err := fixture.Write(t.Context(), store, entries); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	return root
}

// ---------------------------------------------------------------------------
// acc: `dira brief --context` over the 200-entry fixture emits <=1500 tokens by
// the binary's own counter, with the ceiling read from brief.max_tokens
// ---------------------------------------------------------------------------

func TestTheBriefStaysUnderTheDefaultCeiling(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "")
	for _, form := range [][]string{{}, {"--context"}, {"--context", "--chain"}} {
		name := "human"
		if len(form) > 0 {
			name = strings.Join(form, " ")
		}
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := exerciseBrief(t, append([]string{"-C", root}, form...)...)
			if code != exitOK {
				t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
			}
			tokens := brief.Tokens(stdout)
			if tokens > brief.DefaultMaxTokens {
				t.Errorf("the brief is %d tokens, over the %d ceiling cst-0001 sets", tokens, brief.DefaultMaxTokens)
			}
			// A ceiling honoured by printing almost nothing would pass
			// the line above and fail the product.
			if tokens < brief.DefaultMaxTokens/2 {
				t.Errorf("the brief is only %d tokens against a %d ceiling over a 200-entry ledger; "+
					"it is not filling the space it has:\n%s", tokens, brief.DefaultMaxTokens, stdout)
			}
		})
	}
}

// TestALoweredCeilingIsHonoured is the config half of the acceptance line: the
// number comes out of .dira/config.toml, not out of the binary.
func TestALoweredCeilingIsHonoured(t *testing.T) {
	t.Parallel()

	// Every one of these is a ceiling that still admits content. The
	// degenerate ones — where not even the omission notice fits — are their
	// own case below, because what "honoured" means there is different.
	for _, ceiling := range []int{1200, 800, 400, 250, 120} {
		t.Run(strconv.Itoa(ceiling), func(t *testing.T) {
			root := briefLedger(t, fmt.Sprintf("[ledger]\nname = \"fixture\"\n\n[brief]\nmax_tokens = %d\n", ceiling))
			code, stdout, stderr := exerciseBrief(t, "-C", root, "--context")
			if code != exitOK {
				t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
			}
			if tokens := brief.Tokens(stdout); tokens > ceiling {
				t.Errorf("brief.max_tokens = %d and the brief is %d tokens:\n%s", ceiling, tokens, stdout)
			}
			if !strings.Contains(stdout, "fixture") {
				t.Errorf("the ledger name from config.toml is not in the heading:\n%s", stdout)
			}
		})
	}
}

// TestACeilingTooSmallForAnythingStillHonoursItself is the degenerate end of the
// same rule, and it is here because it is where the first implementation of the
// cap was wrong: the omission notice was reserved out of the budget at its worst
// case, so a `max_tokens = 60` produced a brief consisting of a notice that was
// itself over the ceiling. Whatever else it does, this command does not exceed
// the number in the config file.
func TestACeilingTooSmallForAnythingStillHonoursItself(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "")
	for _, ceiling := range []int{1, 5, 20, 60, 90} {
		writeCeiling(t, root, ceiling)
		code, stdout, stderr := exerciseBrief(t, "-C", root, "--context")
		if code != exitOK {
			t.Fatalf("exit code = %d at ceiling %d\nstderr: %s", code, ceiling, stderr)
		}
		if tokens := brief.Tokens(stdout); tokens > ceiling {
			t.Errorf("brief.max_tokens = %d produced %d tokens:\n%s", ceiling, tokens, stdout)
		}
		// And what does come out is whole lines, never half of one.
		if strings.HasSuffix(stdout, " ") {
			t.Errorf("ceiling %d produced output ending mid-line:\n%q", ceiling, stdout)
		}
	}
}

// TestTheCeilingIsHonouredAtEveryCeiling is the fuzz over the same property.
// The two cases above pin the numbers a human would choose; this one asks
// whether the mechanism holds everywhere, including the degenerate ceilings
// where nothing fits.
func TestTheCeilingIsHonouredAtEveryCeiling(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "")
	for ceiling := 1; ceiling <= 400; ceiling += 7 {
		cfg := filepath.Join(root, "ceiling", strconv.Itoa(ceiling))
		if err := os.MkdirAll(cfg, 0o755); err != nil {
			t.Fatal(err)
		}
		writeCeiling(t, root, ceiling)
		_, stdout, _ := exerciseBrief(t, "-C", root, "--context", "--chain")
		if tokens := brief.Tokens(stdout); tokens > ceiling {
			t.Fatalf("brief.max_tokens = %d produced %d tokens:\n%s", ceiling, tokens, stdout)
		}
	}
}

func writeCeiling(t *testing.T, root string, ceiling int) {
	t.Helper()
	path := filepath.Join(root, ".dira", "config.toml")
	body := fmt.Sprintf("[brief]\nmax_tokens = %d\n", ceiling)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// ---------------------------------------------------------------------------
// acc: whole entries are dropped from the low-priority end, every rendered
// entry block is structurally complete, and the output names what it omitted
// plus the verb to see the rest
// ---------------------------------------------------------------------------

// TestOverflowDropsWholeEntriesFromTheLowPriorityEnd is the heart of cst-0001.
//
// It reads the rendered brief back into the entries it names and asserts three
// separate things a truncating renderer would fail: every entry the brief names
// is rendered whole, the entries that survive a lower ceiling are a subset of
// those that survive a higher one, and what disappears first is the bottom of
// the priority order.
func TestOverflowDropsWholeEntriesFromTheLowPriorityEnd(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "")

	kept := map[int]map[string]bool{}
	sections := map[int]map[string]int{}
	ceilings := []int{1500, 1000, 700, 500, 350, 250}

	for _, ceiling := range ceilings {
		writeCeiling(t, root, ceiling)
		code, stdout, stderr := exerciseBrief(t, "-C", root)
		if code != exitOK {
			t.Fatalf("exit code = %d at ceiling %d\nstderr: %s", code, ceiling, stderr)
		}
		kept[ceiling] = idsIn(stdout)
		sections[ceiling] = perSection(stdout)

		if len(kept[ceiling]) == 0 {
			t.Fatalf("ceiling %d rendered no entry at all:\n%s", ceiling, stdout)
		}
		assertEntriesAreWhole(t, root, stdout, ceiling)
	}

	// Monotonic: a lower ceiling never keeps an entry a higher one dropped.
	for i := 1; i < len(ceilings); i++ {
		high, low := ceilings[i-1], ceilings[i]
		if len(kept[low]) >= len(kept[high]) {
			t.Errorf("ceiling %d kept %d entries and ceiling %d kept %d; lowering the ceiling kept no fewer entries",
				high, len(kept[high]), low, len(kept[low]))
		}
		for id := range kept[low] {
			if !kept[high][id] {
				t.Errorf("%s survives at ceiling %d but not at ceiling %d, so dropping is not from one end",
					id, low, high)
			}
		}
	}

	// Priority, asserted against the order cst-0001 names rather than against
	// whatever order the code happens to use.
	//
	// The property is that what survives is a *prefix*: a section that lost
	// even one entry is followed by sections holding none. Stated as
	// "something lower still has entries while this one shrank" it passes
	// vacuously when the important section was empty from the start — which
	// is exactly how an earlier version of this check let a deliberately
	// inverted drop order through.
	order := []string{"open blockers", "current focus", "recent decisions", "fresh notes"}
	full := sections[ceilings[0]]
	for _, ceiling := range ceilings {
		exhausted := ""
		for _, name := range order {
			switch {
			case exhausted != "" && sections[ceiling][name] > 0:
				t.Errorf("at ceiling %d, %q holds %d entries although %q was already short of the %d it has — "+
					"cst-0001 keeps open blockers first and drops the low-priority end",
					ceiling, name, sections[ceiling][name], exhausted, full[exhausted])
			case sections[ceiling][name] < full[name]:
				exhausted = name
			}
		}
	}

	// And the priority order is only meaningful if the ceilings under test
	// actually bite the top section at some point.
	if sections[ceilings[len(ceilings)-1]]["open blockers"] >= full["open blockers"] {
		t.Errorf("even the lowest ceiling kept every open blocker; the priority order is not under test")
	}
}

// TestTheOmissionIsNamedWithTheVerbToSeeTheRest is the other half of cst-0001's
// sentence: "states what it omitted plus the verb to see the rest".
func TestTheOmissionIsNamedWithTheVerbToSeeTheRest(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "[brief]\nmax_tokens = 600\n")
	code, stdout, stderr := exerciseBrief(t, "-C", root)
	if code != exitOK {
		t.Fatalf("exit code = %d\nstderr: %s", code, stderr)
	}

	for _, want := range []string{"omitted", "recent decisions", "cst-0001", "dira why"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the brief omitted entries without naming %q:\n%s", want, stdout)
		}
	}
	// The count in the notice has to be the real one, not a placeholder.
	if !omissionCountIsReal(t, root, stdout) {
		t.Errorf("the omission notice does not agree with what was actually left out:\n%s", stdout)
	}
	// Raising the cap is not offered: cst-0001 says raising it requires
	// superseding the constraint, so a footer suggesting it would undermine
	// the rule the same output is enforcing.
	if strings.Contains(stdout, "max_tokens") {
		t.Errorf("the brief suggests raising the ceiling it is enforcing:\n%s", stdout)
	}
}

// TestASingleSectionCeilingStillYieldsTheBlockers is the acceptance line's
// "a ceiling low enough to admit only one section still yields a well-formed
// brief containing the open blockers".
func TestASingleSectionCeilingStillYieldsTheBlockers(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "[brief]\nmax_tokens = 200\n")
	code, stdout, stderr := exerciseBrief(t, "-C", root)
	if code != exitOK {
		t.Fatalf("exit code = %d\nstderr: %s", code, stderr)
	}

	if !strings.Contains(stdout, "open blockers") {
		t.Fatalf("a ceiling admitting one section dropped the open blockers:\n%s", stdout)
	}
	if strings.Contains(stdout, "recent decisions\n  dec-") {
		t.Errorf("a 200-token ceiling still rendered decisions; this case is not exercising a single-section brief:\n%s", stdout)
	}
	if !strings.HasPrefix(stdout, "dira brief") {
		t.Errorf("the brief lost its heading before it lost its content:\n%s", stdout)
	}
	if !strings.Contains(stdout, "omitted") {
		t.Errorf("a brief that dropped whole sections did not say so:\n%s", stdout)
	}
	assertEntriesAreWhole(t, root, stdout, 200)
}

// ---------------------------------------------------------------------------
// acc: --chain with no [parents] configured exits 0 and says so
// ---------------------------------------------------------------------------

func TestChainWithNoParentsConfiguredSaysSo(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "[ledger]\nname = \"fixture\"\n\n[parents]\n# sire = { path = \"../sire\" }\n")
	code, stdout, stderr := exerciseBrief(t, "-C", root, "--context", "--chain")

	if code != exitOK {
		t.Fatalf("exit code = %d, want %d — the documented hook command must not fail\nstderr: %s", code, exitOK, stderr)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatal("--chain printed nothing; a silent no-op is the defect this case exists to catch")
	}
	if !strings.Contains(stdout, "no parent ledger is configured") {
		t.Errorf("--chain does not state that no parent ledger is configured:\n%s", stdout)
	}
	// A commented-out example is not a declaration, which is the same rule
	// scripts/privacy-lint.py applies to the same file.
	if strings.Contains(stdout, "sire") {
		t.Errorf("a commented-out parent was read as configured:\n%s", stdout)
	}
}

// TestChainWithAnUnresolvableParentDegradesRatherThanFails. A configured
// parent dira cannot locate (no such directory — the shape a private parent
// takes in a public clone) is named in the notice and contributes nothing:
// there is no ledger there to read, so the brief says so and moves on rather
// than failing the whole command over one bad or absent declaration.
func TestChainWithAnUnresolvableParentDegradesRatherThanFails(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "[parents]\nsire = { path = \"../sire\" }\n")
	code, stdout, _ := exerciseBrief(t, "-C", root, "--chain")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout, "sire") {
		t.Errorf("a configured parent is not mentioned:\n%s", stdout)
	}
	if strings.Contains(stdout, "not in this release") {
		t.Errorf("--chain still claims resolution is not in this release, which E5-L4 supersedes:\n%s", stdout)
	}
}

// TestBriefChain is E5-L4-T1's acceptance line: `--chain` prints content from
// all three tiers under the same ceiling, an ancestor the ceiling cuts is
// named in the footer by namespace, and an unreadable ancestor's title never
// appears.
func TestBriefChain(t *testing.T) {
	t.Parallel()

	root := chainFixture(t)
	code, stdout, stderr := exerciseBrief(t, "-C", filepath.Join(root, "repo"), "--context", "--chain")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
	}

	for _, want := range []string{"repo-own-intent", "sire:int-0001", "sire's own bet", "me:int-0001", "me's own direction"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the brief is missing %q:\n%s", want, stdout)
		}
	}
}

// TestBriefChainStaysUnderTheSharedCeiling is cst-0001's unconditional
// clause: the ceiling applies to the chain, not just the local brief.
func TestBriefChainStaysUnderTheSharedCeiling(t *testing.T) {
	t.Parallel()

	root := chainFixtureOversized(t)
	code, stdout, stderr := exerciseBrief(t, "-C", filepath.Join(root, "repo"), "--chain")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
	}
	if got := brief.Tokens(stdout); got > brief.DefaultMaxTokens {
		t.Errorf("--chain produced %d tokens against a %d ceiling", got, brief.DefaultMaxTokens)
	}
	if !strings.Contains(stdout, "omitted") || !strings.Contains(stdout, "sire") {
		t.Errorf("the footer does not name a dropped chain section by namespace:\n%s", stdout)
	}
}

// TestBriefChainNeverRendersAnUnreadableAncestorsText proves the withheld
// discipline holds at the brief layer too: an ancestor the ceiling never even
// reaches, because it could not be opened at all, contributes no title.
func TestBriefChainNeverRendersAnUnreadableAncestorsText(t *testing.T) {
	t.Parallel()

	root := chainFixture(t)
	if err := os.Chmod(filepath.Join(root, "me", ".dira"), 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "me", ".dira"), 0o755) })

	code, stdout, stderr := exerciseBrief(t, "-C", filepath.Join(root, "repo"), "--chain")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
	}
	if strings.Contains(stdout, "me's own direction") {
		t.Errorf("the brief rendered text from an unreadable ancestor:\n%s", stdout)
	}
}

// chainFixture builds repo -> sire (workspace, bets) -> me (person,
// directions), a small three-tier fixture sized to fit comfortably under the
// default ceiling.
func chainFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	writeChainLedger(t, filepath.Join(root, "me"), "[ledger]\nname = \"me\"\ntier = \"person\"\n",
		map[string]string{"int-0001": chainEntry("int-0001", "intent", "me's own direction for the quarter")})

	writeChainLedger(t, filepath.Join(root, "sire"),
		"[ledger]\nname = \"sire\"\ntier = \"workspace\"\n\n[parents]\nme = { path = \"../../me\" }\n",
		map[string]string{"int-0001": chainEntry("int-0001", "intent", "sire's own bet for the quarter")})

	writeChainLedger(t, filepath.Join(root, "repo"),
		"[ledger]\nname = \"repo\"\ntier = \"repo\"\n\n[parents]\nsire = { path = \"../../sire\" }\n",
		map[string]string{"int-0001": chainEntry("int-0001", "intent", "repo-own-intent for this checkout")})

	return root
}

// chainFixtureOversized is chainFixture with sire holding enough active bets
// that the combined local-plus-chain content exceeds 1500 tokens unbudgeted —
// proving the in-budget clause is not vacuously true of a fixture too small
// to test it.
func chainFixtureOversized(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	writeChainLedger(t, filepath.Join(root, "me"), "[ledger]\nname = \"me\"\ntier = \"person\"\n",
		map[string]string{"int-0001": chainEntry("int-0001", "intent", "me's own direction for the quarter")})

	sireEntries := map[string]string{}
	for i := 1; i <= 150; i++ {
		id := fmt.Sprintf("int-%04d", i)
		// entry.schema.json caps a title at 120 characters; this is 92,
		// which is long enough that 150 of them add up to real tokens
		// without failing validation and being silently skipped.
		sireEntries[id] = chainEntry(id, "intent",
			fmt.Sprintf("sire's bet number %04d, worded long enough to cost real tokens once rendered in the brief", i))
	}
	writeChainLedger(t, filepath.Join(root, "sire"),
		"[ledger]\nname = \"sire\"\ntier = \"workspace\"\n\n[parents]\nme = { path = \"../../me\" }\n",
		sireEntries)

	writeChainLedger(t, filepath.Join(root, "repo"),
		"[ledger]\nname = \"repo\"\ntier = \"repo\"\n\n[parents]\nsire = { path = \"../../sire\" }\n",
		map[string]string{"int-0001": chainEntry("int-0001", "intent", "repo-own-intent for this checkout")})

	// The raw, unbudgeted size check: sire's own content alone has to
	// exceed the ceiling for the in-budget clause below to mean anything.
	var raw strings.Builder
	for _, body := range sireEntries {
		raw.WriteString(body)
	}
	if brief.Tokens(raw.String()) <= brief.DefaultMaxTokens {
		t.Fatalf("the oversized fixture's own sire content is only %d tokens; it does not exceed the %d ceiling unbudgeted",
			brief.Tokens(raw.String()), brief.DefaultMaxTokens)
	}

	return root
}

func writeChainLedger(t *testing.T, dir, config string, entries map[string]string) {
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
}

func chainEntry(id, kind, title string) string {
	return "---\nid: " + id + "\nkind: " + kind + "\ntitle: " + title + "\nstate: active\n" +
		"created: \"2026-06-01T09:00:00Z\"\n---\n\nfixture body.\n"
}

// ---------------------------------------------------------------------------
// fail open
// ---------------------------------------------------------------------------

// TestOneMalformedEntryDegradesToABriefWithoutIt is the lane's fail-open rule.
// The hook discards stderr, so the statement has to be in the brief itself.
func TestOneMalformedEntryDegradesToABriefWithoutIt(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "")
	broken := filepath.Join(root, ".dira", "entries", "dec-0003.md")
	if err := os.WriteFile(broken, []byte("---\nthis: [is not, an entry\n"), 0o644); err != nil {
		t.Fatalf("breaking an entry: %v", err)
	}

	code, stdout, stderr := exerciseBrief(t, "-C", root, "--context")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d — one bad file must not take the brief down\nstderr: %s", code, exitOK, stderr)
	}
	if !strings.Contains(stdout, "dec-0003") {
		t.Errorf("the brief does not name the entry it could not read:\n%s", stdout)
	}
	if !strings.Contains(stdout, "recent decisions") {
		t.Errorf("one unreadable entry cost the whole brief:\n%s", stdout)
	}
	if strings.Contains(stdout, "panic") || strings.Contains(stdout, "goroutine") {
		t.Errorf("the brief reads like a crash:\n%s", stdout)
	}
}

// TestAConfigDiraCannotUnderstandStillYieldsABrief covers the other fail-open
// path. A typo in max_tokens must not cost the session its orientation, and the
// ceiling it falls back to must be the constitutional one rather than none.
func TestAConfigDiraCannotUnderstandStillYieldsABrief(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "[brief]\nmax_tokens = \"fifteen hundred\"\n")
	code, stdout, stderr := exerciseBrief(t, "-C", root)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
	}
	if !strings.Contains(stdout, "max_tokens") {
		t.Errorf("the brief does not say the ceiling in config.toml could not be read:\n%s", stdout)
	}
	if tokens := brief.Tokens(stdout); tokens > brief.DefaultMaxTokens {
		t.Errorf("an unreadable ceiling produced an uncapped brief: %d tokens", tokens)
	}
}

// TestABriefOverAnEmptyLedgerIsStillABrief. A fresh `.dira/` is the first thing
// a new user has, and the first brief they see must not be an error.
func TestABriefOverAnEmptyLedgerIsStillABrief(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".dira", "entries"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := exerciseBrief(t, "-C", root)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
	}
	for _, want := range []string{"open blockers", "none"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("an empty ledger's brief is missing %q:\n%s", want, stdout)
		}
	}
}

// ---------------------------------------------------------------------------
// what the brief must never say
// ---------------------------------------------------------------------------

// TestTheBriefAssertsNoExecutionStatus is dec-0004 asserted rather than
// intended. dira embeds no kazi client and E4 owns the join, so any word here
// about a goal's progress would be dira asserting something it did not derive.
func TestTheBriefAssertsNoExecutionStatus(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "")
	_, stdout, _ := exerciseBrief(t, "-C", root, "--context")

	for _, forbidden := range []string{"converged", "in progress", "done", "planned", "running", "blocked on kazi", "goal-"} {
		if strings.Contains(strings.ToLower(stdout), forbidden) {
			t.Errorf("the brief says %q; execution status is derived from kazi at read time and never stored or guessed (dec-0004):\n%s",
				forbidden, stdout)
		}
	}
}

// TestAPrivateEntryIsCitedByRefOnly. cst-0003 and the precedent internal/enforcer
// set: the binary cannot tell whether its stdout is a terminal or a pull-request
// body, so it never quotes a private entry's text.
func TestAPrivateEntryIsCitedByRefOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	diraDir := filepath.Join(root, ".dira")
	writeWhyLedger(t, diraDir, map[string]string{
		"int-0100.md": `---
id: int-0100
kind: intent
title: A private direction nobody outside may read
state: active
created: "2026-07-01T00:00:00Z"
private: true
---

Body.
`,
	})

	code, stdout, stderr := exerciseBrief(t, "-C", root)
	if code != exitOK {
		t.Fatalf("exit code = %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "int-0100") {
		t.Errorf("a private entry vanished from the brief instead of being cited:\n%s", stdout)
	}
	if strings.Contains(stdout, "nobody outside may read") {
		t.Errorf("the brief quoted a private entry's title:\n%s", stdout)
	}
}

// TestTheBriefWritesNothing. A read verb that mutates the ledger is how a
// derived-status product acquires stored status by accident (dec-0004), and the
// brief is the read verb that runs most often.
func TestTheBriefWritesNothing(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "")
	entries := filepath.Join(root, ".dira", "entries")

	before := whySnapshot(t, entries)
	exerciseBrief(t, "-C", root)
	exerciseBrief(t, "-C", root, "--context", "--chain")
	after := whySnapshot(t, entries)

	if len(before) == 0 {
		t.Fatal("the ledger snapshot is empty; the comparison would pass on an empty directory")
	}
	if len(before) != len(after) {
		t.Fatalf("the ledger has %d entries after `dira brief` and had %d", len(after), len(before))
	}
	for name, want := range before {
		if after[name] != want {
			t.Errorf("%s changed after `dira brief`", name)
		}
	}
}

// ---------------------------------------------------------------------------
// the command's own surface
// ---------------------------------------------------------------------------

func TestBriefDocumentsItsOwnFlags(t *testing.T) {
	t.Parallel()

	code, stdout, stderr := exerciseBrief(t, "-h")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr: %s", code, exitOK, stderr)
	}
	for _, flagName := range []string{"-C", "-context", "-chain", "-width"} {
		if !strings.Contains(stdout, flagName) {
			t.Errorf("`dira brief -h` does not document %s:\n%s", flagName, stdout)
		}
	}
	for _, want := range []string{"usage:", "exit codes:", "cst-0001"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("`dira brief -h` is missing %q:\n%s", want, stdout)
		}
	}
}

func TestBriefRejectsMisuse(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "")
	cases := []struct {
		name string
		args []string
	}{
		{"an argument", []string{"-C", root, "dec-0001"}},
		{"unknown flag", []string{"-C", root, "-nope"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := exerciseBrief(t, tc.args...)
			if code != exitUsage {
				t.Errorf("exit code = %d, want %d\nstderr: %s", code, exitUsage, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty on a usage error", stdout)
			}
			if !strings.Contains(stderr, "dira brief") {
				t.Errorf("the usage printed is not this command's:\n%s", stderr)
			}
		})
	}
}

func TestBriefHonoursWidth(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "")
	for _, width := range []int{56, 72, 80, 120} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			_, stdout, _ := exerciseBrief(t, "-C", root, "-width", strconv.Itoa(width))
			longest := 0
			for _, line := range strings.Split(stdout, "\n") {
				if n := len([]rune(line)); n > longest {
					longest = n
				}
			}
			if longest > width {
				t.Errorf("a line is %d columns wide at -width %d:\n%s", longest, width, stdout)
			}
			if longest < width/2 {
				t.Errorf("the longest line is %d columns at -width %d; the text is not laid out to the requested width", longest, width)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// entryLine matches a rendered entry row: two spaces, an id, two spaces.
func idsIn(brief string) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(brief, "\n") {
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 && ledger.ValidID(fields[0]) {
			out[fields[0]] = true
		}
	}
	return out
}

// perSection counts the entries rendered under each heading.
func perSection(brief string) map[string]int {
	out := map[string]int{}
	section := ""
	for _, line := range strings.Split(brief, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "dira brief") && !strings.HasPrefix(line, "omitted"):
			section = trimmed
		default:
			fields := strings.Fields(line)
			if len(fields) > 0 && ledger.ValidID(fields[0]) && section != "" {
				out[section]++
			}
		}
	}
	return out
}

// statusColumn matches the right margin — a state and a date set hard against
// the width — which sits *inside* a wrapped title's lines and would otherwise
// look like the title changing halfway through.
var statusColumn = regexp.MustCompile(
	`\s+(active|achieved|abandoned|accepted|rejected|superseded|open|answered|staged) \d{4}-\d{2}-\d{2}\s*$`)

// incompleteEntries names every entry whose rendered block is missing part of
// what the entry file says it should carry.
//
// A block is the entry's row and everything indented under it, up to the next
// row or heading. Two things have to survive: the whole title, and every entry
// the `blocks` edges name. The second half is not decoration — the first version
// of this check looked only at titles, and a renderer deliberately mutated to cut
// a block's last line passed it, because what it cut was a `blocks` line. A check
// that can only see the first line of a block cannot tell "drops whole entries"
// from "truncates".
//
// It is written against the entry files rather than against the renderer's own
// output, because an assertion in terms of the renderer would be satisfied by a
// renderer that truncated consistently.
func incompleteEntries(t *testing.T, root, rendered string) []string {
	t.Helper()

	var missing []string
	for id, block := range blocksIn(rendered) {
		data, err := os.ReadFile(filepath.Join(root, ".dira", "entries", id+".md"))
		if err != nil {
			continue
		}
		private := strings.Contains(string(data), "\nprivate: true\n")

		// A private entry is cited by ref and never by title (cst-0003),
		// so its title being absent is the rule rather than a truncation.
		// TestAPrivateEntryIsCitedByRefOnly asserts that directly.
		if title := titleOf(string(data)); title != "" && !private {
			if !strings.Contains(block, strings.Join(strings.Fields(title), " ")) {
				missing = append(missing, id+": title "+strconv.Quote(title))
			}
		}
		for _, target := range blocksEdges(string(data)) {
			if !strings.Contains(block, "blocks "+target) {
				missing = append(missing, id+": the blocks edge to "+target)
			}
		}
	}
	sort.Strings(missing)
	return missing
}

// blocksIn splits a rendered brief into one flattened block per entry: the row
// and every line indented under it, with the right-margin status removed so a
// wrapped title reads as one string.
func blocksIn(rendered string) map[string]string {
	out := map[string]string{}

	id := ""
	var current []string
	flush := func() {
		if id != "" {
			out[id] = strings.Join(strings.Fields(strings.Join(current, " ")), " ")
		}
		id, current = "", nil
	}

	for _, raw := range strings.Split(rendered, "\n") {
		line := statusColumn.ReplaceAllString(raw, "")
		fields := strings.Fields(line)

		switch {
		case len(fields) == 0, !strings.HasPrefix(line, "  "):
			flush()
		case ledger.ValidID(fields[0]) && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   "):
			flush()
			id = fields[0]
			current = append(current, strings.Join(fields[1:], " "))
		case id != "":
			current = append(current, line)
		}
	}
	flush()
	return out
}

// blocksEdges reads the `blocks` edge targets out of an entry file. Deliberately
// a second, cruder reader than internal/ledger's: a check that decoded with the
// same codec the renderer reads through would agree with it about a field they
// were both wrong about.
func blocksEdges(entry string) []string {
	var out []string
	lines := strings.Split(entry, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "- type: blocks" {
			continue
		}
		for _, next := range lines[i+1:] {
			trimmed := strings.TrimSpace(next)
			if rest, ok := strings.CutPrefix(trimmed, "to: "); ok {
				out = append(out, strings.TrimSpace(rest))
				break
			}
			if strings.HasPrefix(trimmed, "- ") || trimmed == "---" {
				break
			}
		}
	}
	return out
}

func assertEntriesAreWhole(t *testing.T, root, rendered string, ceiling int) {
	t.Helper()

	if missing := incompleteEntries(t, root, rendered); len(missing) > 0 {
		t.Errorf("at ceiling %d, %d entries are rendered without their whole title — an entry was cut mid-render:\n%s\n%s",
			ceiling, len(missing), strings.Join(missing, "\n"), rendered)
	}
}

// TestTheWholenessCheckHasTeeth. The assertion above is the only thing standing
// between "drops whole entries" and "truncates and nobody notices", and it is
// written in terms of a renderer that does the right thing — so it is worth
// proving it can tell. A brief with one title cut in half must be caught, and
// the untouched brief must not be.
func TestTheWholenessCheckHasTeeth(t *testing.T) {
	t.Parallel()

	root := briefLedger(t, "[brief]\nmax_tokens = 700\n")
	_, stdout, _ := exerciseBrief(t, "-C", root)

	if missing := incompleteEntries(t, root, stdout); len(missing) > 0 {
		t.Fatalf("the check reports the real brief as truncated, so its verdicts mean nothing:\n%s", strings.Join(missing, "\n"))
	}

	// Cut one rendered title in half, the way a truncating renderer would.
	cut := regexp.MustCompile(`(?m)^(  qst-\d{4}  \S+ \S+ \S+).*$`).ReplaceAllString(stdout, "$1")
	if cut == stdout {
		t.Fatal("nothing was cut, so this case measures nothing")
	}
	if missing := incompleteEntries(t, root, cut); len(missing) == 0 {
		t.Errorf("a brief with a title cut mid-render passed the wholeness check:\n%s", cut)
	}
}

func titleOf(entry string) string {
	for _, line := range strings.Split(entry, "\n") {
		if rest, ok := strings.CutPrefix(line, "title: "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// omissionCountIsReal cross-checks the footer's arithmetic against the ledger:
// the number omitted plus the number rendered has to be the number the ledger
// holds for that section.
func omissionCountIsReal(t *testing.T, root, rendered string) bool {
	t.Helper()

	// The notice wraps, so it is read flattened rather than line by line.
	flat := strings.Join(strings.Fields(rendered), " ")
	found := regexp.MustCompile(`omitted .*?(\d+) recent decisions`).FindStringSubmatch(flat)
	if found == nil {
		t.Errorf("the omission notice does not name a count of omitted decisions:\n%s", rendered)
		return false
	}
	omitted, err := strconv.Atoi(found[1])
	if err != nil || omitted == 0 {
		return false
	}

	accepted := 0
	entries, err := os.ReadDir(filepath.Join(root, ".dira", "entries"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(root, ".dira", "entries", e.Name()))
		if err != nil {
			continue
		}
		if strings.HasPrefix(e.Name(), "dec-") && strings.Contains(string(data), "\nstate: accepted\n") {
			accepted++
		}
	}
	shown := perSection(rendered)["recent decisions"]
	return omitted+shown == accepted
}
