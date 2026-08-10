package installhooks

// E2-L3-T1 — kazi's install-hooks case list, ported as a table plus a recorded
// settings file per case, BEFORE the installer exists.
//
// Why this is a task of its own rather than a paragraph inside T4: an installer
// that also owns its own case list decides its own bar. kazi's suite
// (test/kazi/teach/install_hooks_test.exs) is a list somebody else already paid
// for — five describe groups covering absent-file install, merge-never-clobber,
// uninstall, malformed input and install targets — and transcribing it first is
// what stops T3/T4/T5 from quietly narrowing the problem to whatever they
// happen to implement.
//
// WHAT LIVES HERE, and what deliberately does not. This file is the corpus and
// its well-formedness check. It asserts nothing about installing, because there
// is nothing to install yet: T3 (spans), T4 (install), T5 (uninstall), T7
// (command reality) and T8 (fail-open) each iterate contractCases() and bring
// their own assertions. So the fields below are the *declared expectation* per
// case — does the file exist at all, do the bytes parse, must the installer
// refuse them and why — and every one of those declarations is checked here
// against the actual bytes on disk. A declaration nobody checks is exactly the
// shape docs/lore.md L-0001 calls "a check that validates a declaration rather
// than a result".
//
// A FIXTURE CORPUS IS THE EASIEST ARTEFACT IN THE WORLD TO PASS VACUOUSLY.
// Every clause of the form "every case names a fixture that exists" and "every
// fixture parses" is true of an empty table and true of an empty directory. So
// contractCorpus.problems() puts the two floors FIRST, refuses a zero-value
// field on any case (the enums below have no valid zero value, on purpose),
// refuses a zero-byte fixture, refuses two fixtures with the same bytes, and
// requires at least one case per verdict and per syntax. TestContractTableCanFail
// then drives twenty-two mutants through the same function and observes each one
// go red — and each is asserted to name the thing that broke, because a checker
// answering "something is wrong" to everything is useless in a failure. This
// repository has now found seven checks that reported a verdict they never
// reached; the point of that second test is to not be the eighth.
//
// IMPORT NOTE (dec-0005). This file imports os and path/filepath. That is
// allowed and load-bearing: internal/ledger/boundary_test.go reads `go list`'s
// .Imports, which covers non-test files only, so *_test.go in this package may
// touch the filesystem while the shipped code may not. internal/installhooks
// is on nobody's allowlist and must not be added to one.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Where the corpus lives, and the two floors under it.
const (
	// contractExampleFile is this project's own documented example, named at
	// its REAL location rather than copied into testdata. A copy would drift
	// from the file `dira install-hooks` actually installs, and the drift
	// would be invisible: both files would parse, both would round-trip, and
	// the suite would be green about a document nobody ships. The corpus
	// check below also refuses any fixture whose bytes equal this file's, so
	// a later "fix" that copies it in is caught rather than accepted.
	contractExampleFile = "../../hooks/settings.example.json"

	// contractFixtureDir holds every recorded settings file except the one
	// above. Slash-separated; filepath.FromSlash is applied at the one place
	// bytes are actually read.
	contractFixtureDir = "testdata/settings"

	// The two floors. Eight is T1's acceptance number for cases; the fixture
	// floor is set to the same value rather than to the fixture count, so
	// deleting fixtures to make a failing suite pass is itself a failure.
	contractMinCases    = 8
	contractMinFixtures = 8

	// At least this many cases must trace to a kazi TEST rather than to
	// kazi's implementation. Without it, "every case names a kazi reference"
	// could be satisfied by pointing every case at one source file, which
	// would lose exactly the transcription this task exists to perform.
	contractMinKaziTestRefs = 8

	// The two kazi files a reference may cite, each rendered repo-relative.
	contractKaziTestFile   = "test/kazi/teach/install_hooks_test.exs"
	contractKaziSourceFile = "lib/kazi/teach/install_hooks.ex"
)

// contractPresence is whether the settings file exists at all before the
// installer runs. Absence is a case, not a missing case: kazi's first describe
// group is entirely about it, and it is the only case that ever licenses
// `--uninstall` to delete the file (T5).
//
// A string type with no valid zero value, so a case somebody added without
// thinking about presence fails the table rather than defaulting to something
// plausible. Same for contractSyntax and contractVerdict below.
type contractPresence string

const (
	contractFilePresent contractPresence = "present"
	contractFileAbsent  contractPresence = "absent"
)

// contractSyntax is what encoding/json makes of the recorded bytes.
//
// Kept SEPARATE from contractVerdict because kazi's "malformed settings file"
// group holds three different things and only one of them is a syntax error:
// `{ this is not json` does not parse, but `["an","array"]` and
// `{"hooks":"nope"}` parse perfectly and must still be refused. One boolean
// cannot carry both, and collapsing them would make the non-object-root fixture
// untestable under one clause or the other.
type contractSyntax string

const (
	contractWellFormed contractSyntax = "well-formed JSON"
	contractBadBytes   contractSyntax = "not JSON at all"
	contractNoBytes    contractSyntax = "no bytes — the file is absent"
)

// contractVerdict is what the installer owes the case: proceed, or refuse with
// a named reason. T3, T4 and T5 read this field; it is the whole reason the
// corpus is consumable rather than decorative.
type contractVerdict string

const (
	contractAccept         contractVerdict = "accept"
	contractRejectBadBytes contractVerdict = "refuse: not valid JSON"
	contractRejectRoot     contractVerdict = "refuse: the root is not a JSON object"
	contractRejectHooks    contractVerdict = `refuse: "hooks" is not a JSON object`
)

// contractVerdicts and contractSyntaxes exist so the coverage check below
// iterates the taxonomy instead of restating it. Adding a verdict without a
// case that exercises it turns the table red.
func contractVerdicts() []contractVerdict {
	return []contractVerdict{contractAccept, contractRejectBadBytes, contractRejectRoot, contractRejectHooks}
}

func contractSyntaxes() []contractSyntax {
	return []contractSyntax{contractWellFormed, contractBadBytes, contractNoBytes}
}

// A contractCase is one recorded input and everything declared about it.
//
// Kazi is a DATA field rather than a comment because T1's acceptance says so,
// and the reason is worth restating: a provenance comment cannot be checked for
// emptiness, cannot be checked for duplication, and cannot be counted. This one
// can, and is.
type contractCase struct {
	// Name identifies the case in failure messages. Unique across the table.
	Name string

	// Kazi is where this input came from: `<file> :: <group> / <test>` for a
	// test, `<file> :: <function>` for kazi's implementation. Non-empty and
	// unique across the table.
	Kazi string

	// Why says what this case is for in one sentence — in particular where
	// the mapping to Kazi is not one-to-one. Non-empty, because a case whose
	// purpose nobody wrote down is a case the next task cannot use.
	Why string

	// File is the recorded bytes, relative to this package's directory.
	// Empty for, and only for, the absent case.
	File string

	Present contractPresence
	Syntax  contractSyntax
	Verdict contractVerdict
}

// contractCases is the table. Fourteen cases: the eleven distinct input files
// T1 enumerates, plus the two prefix-ownership inputs the lane's own history
// demands (see pre-all-precompact and operator-dira-lookalike), plus the
// project's own documented example.
//
// Every fixture below uses DIRA's three events — SessionStart, Stop, PreCompact
// — and dira's command strings, not kazi's. The shapes are kazi's; the contents
// are this project's, because a corpus of kazi settings files would grade the
// installer against hooks it will never see.
func contractCases() []contractCase {
	return []contractCase{
		{
			Name:    "absent",
			Kazi:    contractKaziTestFile + " :: install into ABSENT settings / creates a valid settings file registering both events",
			Why:     "No settings file at all: the installer must create one holding exactly the three registrations. The only case that ever licenses --uninstall to delete the file (T5).",
			File:    "",
			Present: contractFileAbsent,
			Syntax:  contractNoBytes,
			Verdict: contractAccept,
		},
		{
			Name:    "empty-object",
			Kazi:    contractKaziTestFile + " :: uninstall / restores a `{}` original exactly (never deletes an operator file)",
			Why:     "An operator's own empty document. Install must add to it; uninstall must restore it byte-for-byte and must NOT delete it, which is the other side of the absent case.",
			File:    contractFixtureDir + "/empty-object.json",
			Present: contractFilePresent,
			Syntax:  contractWellFormed,
			Verdict: contractAccept,
		},
		{
			Name:    "operator-hooks",
			Kazi:    contractKaziTestFile + " :: merge, never clobber (R-E55-1) / an operator's own hooks and keys survive byte-identically",
			Why:     "kazi's @operator_settings ported to dira's events: the operator already owns SessionStart and Stop, plus an unrelated PreToolUse, in three-space indent the installer would never emit itself.",
			File:    contractFixtureDir + "/operator-hooks.json",
			Present: contractFilePresent,
			Syntax:  contractWellFormed,
			Verdict: contractAccept,
		},
		{
			Name:    "unknown-top-level-keys",
			Kazi:    contractKaziTestFile + " :: uninstall / is a no-op when nothing is installed",
			Why:     "Top-level keys dira knows nothing about and NO `hooks` key at all, so the installer has to create the hooks object while every other key survives as a span.",
			File:    contractFixtureDir + "/unknown-top-level-keys.json",
			Present: contractFilePresent,
			Syntax:  contractWellFormed,
			Verdict: contractAccept,
		},
		{
			Name:    "documented-example",
			Kazi:    contractKaziTestFile + " :: merge, never clobber (R-E55-1) / install twice into a fresh (absent-created) file is a no-op",
			Why:     "This project's own hooks/settings.example.json, read where it lives: a fully-installed file whose keys include `//`-prefixed documentation arrays. Installing into it must be a byte-level no-op, and the doc arrays must survive — a decode/re-encode port would rewrite every one of them.",
			File:    contractExampleFile,
			Present: contractFilePresent,
			Syntax:  contractWellFormed,
			Verdict: contractAccept,
		},
		{
			Name:    "odd-whitespace",
			Kazi:    contractKaziTestFile + " :: merge, never clobber (R-E55-1) / install twice is a byte-level no-op (idempotent)",
			Why:     "Legal-but-unusual whitespace and key order: tabs, blank lines, spaces before colons, `hooks` in the middle, and a PreCompact key holding an EMPTY array so the installer must insert into an existing array rather than create one.",
			File:    contractFixtureDir + "/odd-whitespace.json",
			Present: contractFilePresent,
			Syntax:  contractWellFormed,
			Verdict: contractAccept,
		},
		{
			Name:    "partially-installed",
			Kazi:    contractKaziTestFile + " :: merge, never clobber (R-E55-1) / a partially-installed file gains only the missing event",
			Why:     "SessionStart already carries dira's exact brief command; Stop and PreCompact are missing. Exactly two entries may be added and the present one must not be touched.",
			File:    contractFixtureDir + "/partially-installed.json",
			Present: contractFilePresent,
			Syntax:  contractWellFormed,
			Verdict: contractAccept,
		},
		{
			Name: "pre-all-precompact",
			Kazi: contractKaziSourceFile + " :: kazi_entry?/2:365 (@command_prefix:71)",
			Why: "THE ORPHAN CASE, and it is not hypothetical: PreCompact holds `dira sniff --deep --stage`, the command as it was BEFORE --all was added. " +
				"kazi identifies its own entries by command prefix precisely so a later task can grow the arguments; an installer matching the whole string would not recognise this, would add a second entry, and the session would run sniff twice per compaction. " +
				"kazi has no test of its own for this because kazi's registrations never changed — dira's did, so the case is carried here against kazi's implementation rather than its suite.",
			File:    contractFixtureDir + "/pre-all-precompact.json",
			Present: contractFilePresent,
			Syntax:  contractWellFormed,
			Verdict: contractAccept,
		},
		{
			Name: "operator-dira-lookalike",
			Kazi: contractKaziSourceFile + " :: kazi_command?/1:382",
			Why: "The other side of prefix ownership: an operator's OWN `dira why dec-0003` hook under Stop. It starts with `dira ` and must still not be recognised as dira's, " +
				"because a prefix broad enough to swallow any `dira …` command would leave the session with no capture at all and report UNCHANGED while doing it.",
			File:    contractFixtureDir + "/operator-dira-lookalike.json",
			Present: contractFilePresent,
			Syntax:  contractWellFormed,
			Verdict: contractAccept,
		},
		{
			Name:    "mixed-entry",
			Kazi:    contractKaziTestFile + " :: uninstall / never removes an entry mixing an operator command with kazi's",
			Why:     "One entry whose `hooks` array holds dira's Stop command AND the operator's. Only a WHOLLY dira-owned entry may be removed, so uninstall must leave this alone and say so.",
			File:    contractFixtureDir + "/mixed-entry.json",
			Present: contractFilePresent,
			Syntax:  contractWellFormed,
			Verdict: contractAccept,
		},
		{
			Name:    "malformed-bytes",
			Kazi:    contractKaziTestFile + " :: a malformed settings file / fails with one clear line and writes NOTHING",
			Why:     "kazi's `{ this is not json` verbatim. Install must refuse with a named error and write nothing.",
			File:    contractFixtureDir + "/malformed-bytes.json",
			Present: contractFilePresent,
			Syntax:  contractBadBytes,
			Verdict: contractRejectBadBytes,
		},
		{
			Name:    "malformed-truncated",
			Kazi:    contractKaziTestFile + " :: a malformed settings file / uninstall on malformed JSON fails and writes nothing",
			Why:     "kazi's `{ broken` verbatim, carried separately because kazi asserts it on the UNINSTALL path: a refusal that only install performs would still lose an operator's file to a stray `--uninstall`.",
			File:    contractFixtureDir + "/malformed-truncated.json",
			Present: contractFilePresent,
			Syntax:  contractBadBytes,
			Verdict: contractRejectBadBytes,
		},
		{
			Name:    "non-object-root",
			Kazi:    contractKaziTestFile + " :: a malformed settings file / a non-object root fails and writes nothing",
			Why:     "`[\"an\",\"array\"]`. Parses perfectly and must still be refused, which is why syntax and verdict are separate fields.",
			File:    contractFixtureDir + "/non-object-root.json",
			Present: contractFilePresent,
			Syntax:  contractWellFormed,
			Verdict: contractRejectRoot,
		},
		{
			Name:    "non-object-hooks",
			Kazi:    contractKaziTestFile + " :: a malformed settings file / a non-object \"hooks\" value fails and writes nothing",
			Why:     "`{\"hooks\":\"nope\"}`. Also parses; the refusal is about the one key the installer edits, so a guess here would write hooks into a string.",
			File:    contractFixtureDir + "/non-object-hooks.json",
			Present: contractFilePresent,
			Syntax:  contractWellFormed,
			Verdict: contractRejectHooks,
		},
	}
}

// Input is how T3, T4, T5, T7 and T8 consume a case, and it is deliberately the
// same shape internal/skill.Install's caller uses: the bytes, and whether the
// file was there at all. Existence is a separate return rather than a nil check
// so "absent" can never be confused with "present and empty" — the difference
// decides whether uninstall may delete the file.
//
// Every case in the table is driven through this accessor by TestContractTable,
// so a case the consuming code could not reach fails here rather than sitting in
// the table looking covered.
func (c contractCase) Input(t *testing.T) (data []byte, exists bool) {
	t.Helper()

	data, exists, err := c.input(contractLoad)
	if err != nil {
		t.Fatalf("case %s: %v", c.Name, err)
	}
	return data, exists
}

// input is Input with the loader injected and the failure returned instead of
// fataled. It exists so the corpus check below reads every case through the
// SAME accessor the consuming tasks will, rather than through a second copy of
// the logic that could quietly disagree with it — and so a case the accessor
// cannot reach turns the corpus red rather than surfacing in T4.
func (c contractCase) input(load func(file string) ([]byte, error)) (data []byte, exists bool, err error) {
	if c.Present != contractFilePresent {
		return nil, false, nil
	}
	data, err = load(c.File)
	return data, true, err
}

// contractLoad reads one recorded file. The single place this package's tests
// touch the disk.
func contractLoad(file string) ([]byte, error) {
	return os.ReadFile(filepath.FromSlash(file))
}

// contractFixtureListing is what is ACTUALLY in testdata/settings, read rather
// than declared. Directories are included on purpose: anything in there that no
// case claims is an orphan, whatever it is.
func contractFixtureListing(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(filepath.FromSlash(contractFixtureDir))
	if err != nil {
		t.Fatalf("cannot list %s: %v", contractFixtureDir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// A contractCorpus is the table, the directory listing, and the loader, bound
// together so the whole check is one pure function of the three.
//
// That is what makes the red side of L-0001 reachable: TestContractTableCanFail
// can hand problems() a corpus with a case removed, a reference blanked, a
// fixture that is not there or a fixture nobody claims, without touching the
// repository. A check whose failure mode can only be produced by breaking the
// working tree does not get exercised, and therefore is not evidence.
type contractCorpus struct {
	cases []contractCase
	files []string
	load  func(file string) ([]byte, error)
}

func realContractCorpus(t *testing.T) contractCorpus {
	t.Helper()
	return contractCorpus{
		cases: contractCases(),
		files: contractFixtureListing(t),
		load:  contractLoad,
	}
}

// problems returns every way this corpus is not a corpus. Empty means sound.
//
// Ordering is deliberate: the two vacuity floors come first, because every
// clause after them is true of an empty table or an empty directory, and a
// suite that reports "all 0 cases valid" is the exact failure L-0001 catalogues.
func (c contractCorpus) problems() []string {
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }

	// --- vacuity floors, before anything else ----------------------------
	if len(c.cases) < contractMinCases {
		add("the case table holds %d case(s), fewer than the required %d — every per-case clause below is vacuously true of an empty table",
			len(c.cases), contractMinCases)
	}
	if len(c.files) < contractMinFixtures {
		add("the fixture directory %s yielded %d file(s), fewer than the required %d — every per-fixture clause below is vacuously true of an empty directory",
			contractFixtureDir, len(c.files), contractMinFixtures)
	}

	// --- per-case field validity -----------------------------------------
	seenName := map[string]bool{}
	seenKazi := map[string]string{}
	claimed := map[string][]string{}
	absentCases := 0
	exampleCases := 0
	kaziTestRefs := 0
	verdictSeen := map[contractVerdict]int{}
	syntaxSeen := map[contractSyntax]int{}

	for _, tc := range c.cases {
		label := tc.Name
		if label == "" {
			label = "<unnamed case>"
			add("a case has no Name; every failure message below would be unattributable")
		}
		if seenName[tc.Name] && tc.Name != "" {
			add("case name %q is used by more than one case", tc.Name)
		}
		seenName[tc.Name] = true

		switch {
		case strings.TrimSpace(tc.Kazi) == "":
			add("case %s carries no kazi reference; a case with no provenance was not ported from anything", label)
		case contractKaziRefKind(tc.Kazi) == "":
			add("case %s has kazi reference %q, which cites neither %s nor %s in `<file> :: <what>` form",
				label, tc.Kazi, contractKaziTestFile, contractKaziSourceFile)
		default:
			if owner, dup := seenKazi[tc.Kazi]; dup {
				add("case %s repeats the kazi reference already used by case %s (%q); two cases claiming one kazi test means one of them was not ported",
					label, owner, tc.Kazi)
			}
			seenKazi[tc.Kazi] = label
			if contractKaziRefKind(tc.Kazi) == "test" {
				kaziTestRefs++
			}
		}

		if strings.TrimSpace(tc.Why) == "" {
			add("case %s says nothing about why it exists; the tasks that consume this table cannot use a case whose purpose is undeclared", label)
		}

		switch tc.Present {
		case contractFilePresent, contractFileAbsent:
		default:
			add("case %s declares presence %q, which is not one of %q/%q (the zero value is not a valid answer)",
				label, tc.Present, contractFilePresent, contractFileAbsent)
		}
		if !slices.Contains(contractSyntaxes(), tc.Syntax) {
			add("case %s declares syntax %q, which is not in the taxonomy (the zero value is not a valid answer)", label, tc.Syntax)
		}
		if !slices.Contains(contractVerdicts(), tc.Verdict) {
			add("case %s declares verdict %q, which is not in the taxonomy (the zero value is not a valid answer)", label, tc.Verdict)
		}
		verdictSeen[tc.Verdict]++
		syntaxSeen[tc.Syntax]++

		// --- presence / syntax / verdict coherence ------------------------
		if tc.Present == contractFileAbsent {
			absentCases++
			if tc.File != "" {
				add("case %s says the file is absent but names fixture %s", label, tc.File)
			}
			if tc.Syntax != contractNoBytes {
				add("case %s says the file is absent but declares syntax %q; absent bytes are %q", label, tc.Syntax, contractNoBytes)
			}
			if tc.Verdict != contractAccept {
				add("case %s says the file is absent but expects %q; an absent file is what the installer creates", label, tc.Verdict)
			}
		} else {
			if tc.File == "" {
				add("case %s says the file is present but names no fixture", label)
			}
			if tc.Syntax == contractNoBytes {
				add("case %s says the file is present but declares %q", label, tc.Syntax)
			}
		}
		if (tc.Syntax == contractBadBytes) != (tc.Verdict == contractRejectBadBytes) {
			add("case %s pairs syntax %q with verdict %q; bytes that do not parse are refused for exactly that reason and no other",
				label, tc.Syntax, tc.Verdict)
		}

		// --- where the fixture is allowed to live -------------------------
		switch {
		case tc.File == "":
		case tc.File == contractExampleFile:
			exampleCases++
		case strings.HasPrefix(tc.File, contractFixtureDir+"/"):
			base := strings.TrimPrefix(tc.File, contractFixtureDir+"/")
			claimed[base] = append(claimed[base], label)
		default:
			add("case %s names %s, which is neither the project's own %s nor a file under %s",
				label, tc.File, contractExampleFile, contractFixtureDir)
		}
	}

	// --- the table's own shape -------------------------------------------
	if absentCases != 1 {
		add("the table holds %d absent-file case(s); exactly one is required, because that case alone licenses --uninstall to delete the file", absentCases)
	}
	if exampleCases != 1 {
		add("the table holds %d case(s) naming %s; exactly one is required, so the project's own documented example is covered by the suite that installs it",
			exampleCases, contractExampleFile)
	}
	if kaziTestRefs < contractMinKaziTestRefs {
		add("only %d case(s) trace to a kazi TEST (%d required); the rest cite %s, and a table that cites only the implementation has not ported the suite",
			kaziTestRefs, contractMinKaziTestRefs, contractKaziSourceFile)
	}
	for _, v := range contractVerdicts() {
		if verdictSeen[v] == 0 {
			add("no case exercises verdict %q; a verdict the corpus never produces is a branch nothing will grade", v)
		}
	}
	for _, s := range contractSyntaxes() {
		if syntaxSeen[s] == 0 {
			add("no case exercises syntax %q; a syntax the corpus never produces is a branch nothing will grade", s)
		}
	}

	// --- the directory and the table must name the same files -------------
	onDisk := map[string]bool{}
	for _, f := range c.files {
		onDisk[f] = true
	}
	for base, owners := range claimed {
		if !onDisk[base] {
			add("case %s names fixture %s/%s, which is not in the directory listing — a dangling reference",
				owners[0], contractFixtureDir, base)
		}
		if len(owners) > 1 {
			add("fixture %s/%s is named by %d cases (%s); each fixture belongs to exactly one case",
				contractFixtureDir, base, len(owners), strings.Join(owners, ", "))
		}
	}
	orphans := make([]string, 0, len(c.files))
	for _, f := range c.files {
		if len(claimed[f]) == 0 {
			orphans = append(orphans, f)
		}
	}
	slices.Sort(orphans)
	for _, f := range orphans {
		add("fixture %s/%s is on disk but no case names it — an orphan fixture is a file nobody tests", contractFixtureDir, f)
	}

	// --- the bytes must be what the table says they are -------------------
	digests := map[string]string{}
	exampleDigest := ""
	if data, err := c.load(contractExampleFile); err == nil {
		sum := sha256.Sum256(data)
		exampleDigest = hex.EncodeToString(sum[:])
	}

	for _, tc := range c.cases {
		// Read through the accessor the consuming tasks use, not around it.
		data, exists, err := tc.input(c.load)
		if exists != (tc.Present == contractFilePresent) {
			add("case %s declares presence %q, but the accessor T3/T4/T5 consume it through reports exists=%v", tc.Name, tc.Present, exists)
		}
		if !exists {
			if len(data) != 0 {
				add("case %s has no file, but the accessor yielded %d byte(s)", tc.Name, len(data))
			}
			continue
		}
		if err != nil {
			add("case %s names fixture %s, which could not be read: %v", tc.Name, tc.File, err)
			continue
		}
		if len(data) == 0 {
			add("case %s names fixture %s, which is zero bytes; an empty file satisfies \"fails to parse\" while testing nothing", tc.Name, tc.File)
			continue
		}

		sum := sha256.Sum256(data)
		digest := hex.EncodeToString(sum[:])
		if tc.File != contractExampleFile && digest == exampleDigest {
			add("case %s names fixture %s, whose bytes are a COPY of %s; the example must be read where it lives or it will drift from what the installer ships",
				tc.Name, tc.File, contractExampleFile)
		}
		if twin, dup := digests[digest]; dup {
			add("fixtures %s and %s hold identical bytes; two cases over one input is one case", twin, tc.File)
		}
		digests[digest] = tc.File

		var root any
		parseErr := json.Unmarshal(data, &root)

		switch tc.Syntax {
		case contractBadBytes:
			if parseErr == nil {
				add("case %s marks %s malformed, but encoding/json parses it; a fixture marked malformed that happens to parse is a case testing nothing",
					tc.Name, tc.File)
			}
			continue
		case contractWellFormed:
			if parseErr != nil {
				add("case %s marks %s well-formed, but encoding/json rejects it: %v", tc.Name, tc.File, parseErr)
				continue
			}
		default:
			continue
		}

		// Well-formed from here: the declared verdict is checked against the
		// document rather than taken on trust.
		obj, isObject := root.(map[string]any)
		hooksValue, hasHooks := any(nil), false
		if isObject {
			hooksValue, hasHooks = obj["hooks"]
		}
		_, hooksIsObject := hooksValue.(map[string]any)

		switch tc.Verdict {
		case contractAccept:
			if !isObject {
				add("case %s expects %q, but %s decodes to %T rather than a JSON object", tc.Name, contractAccept, tc.File, root)
			} else if hasHooks && !hooksIsObject {
				add("case %s expects %q, but %s has a non-object \"hooks\" value (%T)", tc.Name, contractAccept, tc.File, hooksValue)
			}
		case contractRejectRoot:
			if isObject {
				add("case %s expects %q, but %s decodes to a JSON object", tc.Name, contractRejectRoot, tc.File)
			}
		case contractRejectHooks:
			switch {
			case !isObject:
				add("case %s expects %q, but %s does not decode to an object at all", tc.Name, contractRejectHooks, tc.File)
			case !hasHooks:
				add("case %s expects %q, but %s has no \"hooks\" key to be wrong about", tc.Name, contractRejectHooks, tc.File)
			case hooksIsObject:
				add("case %s expects %q, but %s's \"hooks\" value decodes to an object", tc.Name, contractRejectHooks, tc.File)
			}
		case contractRejectBadBytes:
			add("case %s expects %q, but %s is well-formed JSON", tc.Name, contractRejectBadBytes, tc.File)
		}
	}

	slices.Sort(out)
	return out
}

// contractKaziRefKind classifies a provenance reference, or returns "" if it is
// not one. Two shapes are legal and the distinction is counted above.
func contractKaziRefKind(ref string) string {
	for file, kind := range map[string]string{contractKaziTestFile: "test", contractKaziSourceFile: "source"} {
		prefix := file + " :: "
		if strings.HasPrefix(ref, prefix) && strings.TrimSpace(strings.TrimPrefix(ref, prefix)) != "" {
			return kind
		}
	}
	return ""
}

// TestContractTable is T1's acceptance: the corpus is sound, and — first — it is
// large enough that "sound" means anything.
func TestContractTable(t *testing.T) {
	t.Parallel()

	corpus := realContractCorpus(t)

	// The floors are restated here as fatals rather than left to problems()
	// alone, because everything below this point iterates and every loop body
	// is skipped by an empty range.
	if len(corpus.cases) < contractMinCases {
		t.Fatalf("the table holds %d case(s); at least %d are required and every assertion below iterates it",
			len(corpus.cases), contractMinCases)
	}
	if len(corpus.files) < contractMinFixtures {
		t.Fatalf("%s holds %d file(s); at least %d are required and every assertion below iterates it",
			contractFixtureDir, len(corpus.files), contractMinFixtures)
	}
	if strings.Contains(contractExampleFile, "testdata") {
		t.Fatalf("contractExampleFile is %s, which is inside testdata; the point of the clause is that the example is read where it lives", contractExampleFile)
	}

	problems := corpus.problems()
	for _, problem := range problems {
		t.Error(problem)
	}

	// Reachability: every case is driven through the accessor T3/T4/T5 will
	// use, and the (data, exists) it yields must agree with what the case
	// declared. A case the consuming code cannot load is a case that looks
	// covered and is not.
	loaded, absent, totalBytes := 0, 0, 0
	for _, tc := range corpus.cases {
		data, exists := tc.Input(t)
		switch tc.Present {
		case contractFileAbsent:
			if exists || data != nil {
				t.Errorf("case %s declares the file absent but Input reported exists=%v, %d byte(s)", tc.Name, exists, len(data))
			}
			absent++
		default:
			if !exists {
				t.Errorf("case %s declares the file present but Input reported it absent", tc.Name)
				continue
			}
			if len(data) == 0 {
				t.Errorf("case %s loaded 0 bytes through Input", tc.Name)
				continue
			}
			loaded++
			totalBytes += len(data)
		}
	}
	if loaded == 0 {
		t.Fatal("not one case loaded any bytes through Input; the corpus is unreachable by the code meant to consume it")
	}

	// The count is read back from `problems` rather than written as a literal:
	// a summary line that says "0 problems" whatever it just found is its own
	// small instance of the class this file exists to avoid.
	t.Logf("OBSERVED  %d cases (%d fixtures loaded, %d bytes total; %d absent-file case), %d files in %s, %d problem(s)",
		len(corpus.cases), loaded, totalBytes, absent, len(corpus.files), contractFixtureDir, len(problems))
}

// TestContractTableCanFail is the other half of L-0001, and the reason the green
// above is evidence rather than decoration.
//
// Each mutant below is a corpus with one defect of a class T1's acceptance names,
// driven through the same problems() the real corpus is graded by. Note what is
// asserted: not merely "some problem", but a problem naming the thing that broke.
// A checker that answered "something is wrong" to every mutant would pass a
// weaker version of this test and be useless in a failure.
func TestContractTableCanFail(t *testing.T) {
	t.Parallel()

	base := realContractCorpus(t)

	// Rule 2 first: if the untouched corpus is already red, every mutant below
	// is red for a reason that has nothing to do with its mutation.
	if problems := base.problems(); len(problems) != 0 {
		t.Fatalf("the untouched corpus already reports %d problem(s); no mutation below would be evidence of anything:\n%s",
			len(problems), strings.Join(problems, "\n"))
	}

	// drop and edit panic rather than t.Fatal on an unknown case name: they run
	// inside subtests, the outer *testing.T is the wrong one to fail, and a
	// mutation aimed at a case that is not in the table is a defect in this
	// test rather than in the corpus. Silently doing nothing is the outcome
	// that must not happen — a no-op mutation would leave the corpus sound and
	// the subtest would report the check as blind.
	drop := func(c *contractCorpus, name string) {
		kept := c.cases[:0:0]
		for _, tc := range c.cases {
			if tc.Name != name {
				kept = append(kept, tc)
			}
		}
		if len(kept) == len(c.cases) {
			panic("mutation drops case " + name + ", which is not in the table")
		}
		c.cases = kept
	}
	edit := func(c *contractCorpus, name string, f func(*contractCase)) {
		for i := range c.cases {
			if c.cases[i].Name == name {
				f(&c.cases[i])
				return
			}
		}
		panic("mutation targets case " + name + ", which is not in the table")
	}

	mutants := []struct {
		name    string
		want    string
		mutate  func(*contractCorpus)
		defence string
	}{
		{
			name:    "an empty table",
			want:    "the case table holds 0 case(s)",
			mutate:  func(c *contractCorpus) { c.cases = nil },
			defence: "an empty corpus satisfies every \"every case …\" clause — rule 1's exact shape",
		},
		{
			name:    "an empty fixture directory",
			want:    "yielded 0 file(s)",
			mutate:  func(c *contractCorpus) { c.files = nil },
			defence: "an empty directory satisfies every \"every fixture …\" clause",
		},
		{
			name:    "a case with no kazi reference",
			want:    "carries no kazi reference",
			mutate:  func(c *contractCorpus) { edit(c, "mixed-entry", func(tc *contractCase) { tc.Kazi = "" }) },
			defence: "provenance is the field that proves the list was ported rather than invented",
		},
		{
			name: "two cases claiming one kazi test",
			want: "repeats the kazi reference",
			mutate: func(c *contractCorpus) {
				edit(c, "odd-whitespace", func(tc *contractCase) { tc.Kazi = contractCases()[2].Kazi })
			},
			defence: "a duplicated reference means one kazi case was silently dropped",
		},
		{
			name: "a kazi reference in no recognised form",
			want: "cites neither",
			mutate: func(c *contractCorpus) {
				edit(c, "empty-object", func(tc *contractCase) { tc.Kazi = "ported from kazi somewhere" })
			},
			defence: "prose in the provenance field cannot be checked for duplication",
		},
		{
			name:    "a case with no stated purpose",
			want:    "says nothing about why it exists",
			mutate:  func(c *contractCorpus) { edit(c, "empty-object", func(tc *contractCase) { tc.Why = "  " }) },
			defence: "the consuming tasks cannot use a case whose purpose is undeclared",
		},
		{
			name:    "a case with no expected result",
			want:    "declares verdict",
			mutate:  func(c *contractCorpus) { edit(c, "non-object-root", func(tc *contractCase) { tc.Verdict = "" }) },
			defence: "the zero value must not be a valid answer, or a half-written case looks complete",
		},
		{
			name:    "a case with no declared syntax",
			want:    "declares syntax",
			mutate:  func(c *contractCorpus) { edit(c, "operator-hooks", func(tc *contractCase) { tc.Syntax = "" }) },
			defence: "same, for the other half of the expectation",
		},
		{
			name:    "a case with no declared presence",
			want:    "declares presence",
			mutate:  func(c *contractCorpus) { edit(c, "operator-hooks", func(tc *contractCase) { tc.Present = "" }) },
			defence: "same, for the field that decides whether uninstall may delete the file",
		},
		{
			name: "a case pointing at a missing fixture",
			want: "settings/not-on-disk.json",
			mutate: func(c *contractCorpus) {
				edit(c, "partially-installed", func(tc *contractCase) { tc.File = contractFixtureDir + "/not-on-disk.json" })
			},
			defence: "a dangling reference must name the file it could not find",
		},
		{
			name:    "a fixture no case names",
			want:    "settings/orphan.json is on disk but no case names it",
			mutate:  func(c *contractCorpus) { c.files = append(slices.Clone(c.files), "orphan.json") },
			defence: "an orphan fixture is a recorded input nothing grades",
		},
		{
			name: "two cases over one fixture",
			want: "is named by 2 cases",
			mutate: func(c *contractCorpus) {
				edit(c, "partially-installed", func(tc *contractCase) { tc.File = contractFixtureDir + "/mixed-entry.json" })
			},
			defence: "one input claimed twice hides an input claimed not at all",
		},
		{
			name:    "the documented example dropped",
			want:    "0 case(s) naming ../../hooks/settings.example.json",
			mutate:  func(c *contractCorpus) { drop(c, "documented-example") },
			defence: "the file the installer ships must be in the suite that installs it",
		},
		{
			name: "the documented example copied into testdata",
			want: "a COPY of",
			mutate: func(c *contractCorpus) {
				real := c.load
				c.load = func(file string) ([]byte, error) {
					if file == contractFixtureDir+"/empty-object.json" {
						return real(contractExampleFile)
					}
					return real(file)
				}
			},
			defence: "a copy would drift from the shipped file invisibly — both halves would still parse",
		},
		{
			name:    "the absent-file case dropped",
			want:    "0 absent-file case(s)",
			mutate:  func(c *contractCorpus) { drop(c, "absent") },
			defence: "absence is the only case that licenses --uninstall to delete the file",
		},
		{
			name:    "a verdict no case exercises",
			want:    `no case exercises verdict "refuse: the root is not a JSON object"`,
			mutate:  func(c *contractCorpus) { drop(c, "non-object-root") },
			defence: "a refusal branch the corpus never produces is a branch nothing grades",
		},
		{
			name: "a fixture marked malformed that parses",
			want: "but encoding/json parses it",
			mutate: func(c *contractCorpus) {
				edit(c, "empty-object", func(tc *contractCase) { tc.Syntax, tc.Verdict = contractBadBytes, contractRejectBadBytes })
			},
			defence: "T1 names this one: a fixture marked malformed that happens to parse is a case testing nothing",
		},
		{
			name: "a fixture marked well-formed that does not parse",
			want: "but encoding/json rejects it",
			mutate: func(c *contractCorpus) {
				edit(c, "malformed-bytes", func(tc *contractCase) { tc.Syntax, tc.Verdict = contractWellFormed, contractAccept })
			},
			defence: "the green direction of the same clause, which is the one nobody checks",
		},
		{
			name: "a refusal claimed of a document that does not deserve it",
			want: "but " + contractFixtureDir + "/mixed-entry.json decodes to a JSON object",
			mutate: func(c *contractCorpus) {
				edit(c, "mixed-entry", func(tc *contractCase) { tc.Verdict = contractRejectRoot })
			},
			defence: "the declared verdict is checked against the bytes, not taken on trust",
		},
		{
			name: "a zero-byte fixture",
			want: "which is zero bytes",
			mutate: func(c *contractCorpus) {
				real := c.load
				c.load = func(file string) ([]byte, error) {
					if file == contractFixtureDir+"/malformed-bytes.json" {
						return []byte{}, nil
					}
					return real(file)
				}
			},
			defence: "an empty file fails to parse, so it satisfies the malformed clause while recording nothing",
		},
		{
			name: "two fixtures holding the same bytes",
			want: "hold identical bytes",
			mutate: func(c *contractCorpus) {
				real := c.load
				c.load = func(file string) ([]byte, error) {
					if file == contractFixtureDir+"/mixed-entry.json" {
						return real(contractFixtureDir + "/partially-installed.json")
					}
					return real(file)
				}
			},
			defence: "two cases over one input is one case wearing two names",
		},
		{
			name: "every case citing kazi's implementation instead of its suite",
			want: "trace to a kazi TEST",
			mutate: func(c *contractCorpus) {
				for i := range c.cases {
					c.cases[i].Kazi = fmt.Sprintf("%s :: invented_%d/0", contractKaziSourceFile, i)
				}
			},
			defence: "otherwise \"names a kazi reference\" is satisfiable without porting the suite at all",
		},
	}

	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			mutated := contractCorpus{cases: slices.Clone(base.cases), files: slices.Clone(base.files), load: base.load}
			m.mutate(&mutated)

			problems := mutated.problems()
			if len(problems) == 0 {
				t.Fatalf("the corpus check stayed GREEN on %s.\n%s", m.name, m.defence)
			}
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, m.want) }) {
				t.Fatalf("the corpus check went red on %s but named something else.\nwant a problem containing: %s\ngot:\n%s",
					m.name, m.want, strings.Join(problems, "\n"))
			}
			t.Logf("OBSERVED  red on %s: %s", m.name, contractFirstProblemContaining(problems, m.want))
		})
	}
}

func contractFirstProblemContaining(problems []string, want string) string {
	for _, p := range problems {
		if strings.Contains(p, want) {
			return p
		}
	}
	return ""
}
