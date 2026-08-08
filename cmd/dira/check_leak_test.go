package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/enforcer"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// E3-L3-T6 — the lane's leak test, written as negative space.
//
// # What this file asserts, and why the shape is what it is
//
// Not "the output looked clean": that this byte sequence occurs zero times in
// everything the process emitted, paired with two digests, because a check that
// leaked nothing while quietly writing to the parent would satisfy only half of
// cst-0003.
//
// Three habits are load-bearing here, each of them a bug this repository has
// already shipped once:
//
//  1. **Nothing is asserted absent before something is asserted present**
//     (docs/lore.md L-0024). An empty output contains no secret, so a run that
//     resolved no parent, or exited on a flag mistake, or printed nothing at
//     all, would satisfy every absence assertion in this file. So each case
//     asserts the exit code, then the citation, then the redaction marker, and
//     only then searches for the sentinels. L-0024's other half is met by the
//     fixture: the secret sits inside the *matched* sentence, so the citation
//     itself is evidence the sentinel-bearing bytes reached the matcher —
//     "the private text is what the match landed on" removes that one line and
//     watches the verdict go compliant.
//
//  2. **A leak is searched for in every form it could arrive in**
//     (docs/lore.md L-0023). That entry is about screening a *normalised*
//     string and passing a live token, and the same trap is here from the other
//     end: internal/enforcer/text.go lowercases, drops every non-alphanumeric
//     rune and stems, so a leak of normalised text would read
//     `sentinel privat text` and a verbatim search for SENTINEL-PRIVATE-TEXT
//     would report zero occurrences of it. leakForms therefore searches the
//     verbatim sentinel, its case variants, its separator variants, and the
//     bare leading word — which no normalisation this module performs can
//     destroy.
//
//  3. **Every absence has a matching demonstration that the search can fire.**
//     "the parent's tier is the only variable" re-runs the identical fixtures
//     with the parent declared `repo` instead of `person` and requires the
//     title sentinel to appear; "each sentinel is caught when the renderer
//     prints it" runs all three through the real renderer and requires each to
//     be found. An absence assertion with no such pair proves nothing.
//
// # And not from the lint
//
// scripts/privacy-lint.py's P2, P3 and P4 are vacuously green in this
// repository today — there is no live private parent declared and no mirrored
// ADR — and P1 reads only .dira/entries/. Nothing below rests on any of them.
// The fixtures here are testdata, so P1 never sees them either; they are read
// by this test and by nothing else.
//
// # L-0014
//
// Every ledger in this file is materialised into a real temp .dira/ tree.
// internal/enforcer/testdata/ledgers/<name>/ is a flat pile of *.md, and
// local.Find walks *up*, so a command pointed at one grades against this
// repository's own .dira with no error at all — and this repository's ledger
// holds no sentinel, so that failure would make every assertion below green.

// The two fixture ledgers. A `.dira/config.toml` is not a `*.md` and so is not
// part of either fixture: the tier and the [parents] declaration are written by
// leakLedger below, which is also what lets "the parent's tier is the only
// variable" change one of them and nothing else.
const (
	leakParentFixture = "../../internal/enforcer/testdata/ledgers/parent-private"
	leakChildFixture  = "../../internal/enforcer/testdata/ledgers/child-inherits"
)

// leakPlan is the plan under test. It shares no vocabulary with any entry in
// the child fixture, and within the parent it matches exactly one sentence —
// the one carrying leakSentinelText.
const leakPlan = "start a second side project"

// leakDecisionPlan restates the rejected alternative in the parent's dec-0007.
// It is used only where that file is read as a *local* ledger, to show the
// renderer printing the third sentinel.
const leakDecisionPlan = "publish the compensation floor in the public repository"

// The three byte sequences that must never leave the process.
const (
	leakSentinelText     = "SENTINEL-PRIVATE-TEXT"
	leakSentinelTitle    = "SENTINEL-PRIVATE-TITLE"
	leakSentinelDecision = "SENTINEL-PRIVATE-DECISION"
)

// leakSentinels is what every absence assertion searches for.
var leakSentinels = []string{leakSentinelText, leakSentinelTitle, leakSentinelDecision}

// leakSentinelSentence is the one line of the parent's cst-0002 that leakPlan
// can match. Removing it is how this file proves the secret reached the matcher
// rather than being truncated, summarised or filtered away before it.
const leakSentinelSentence = "SENTINEL-PRIVATE-TEXT: no second side project is started here."

// TestNoPrivateTextLeaks is E3-L3-T6.
func TestNoPrivateTextLeaks(t *testing.T) {
	t.Parallel()

	t.Run("the fixtures still carry what is searched for", func(t *testing.T) {
		t.Parallel()

		// The vacuity guard for this whole file. A fixture that lost its
		// sentinels — to a rewrite, a merge, or a well-meant tidy — would
		// make every zero-occurrence assertion below green while proving
		// nothing whatsoever.
		parent := leakFixtureText(t, leakParentFixture)
		for _, sentinel := range leakSentinels {
			if !strings.Contains(parent, sentinel) {
				t.Errorf("%s carries no %s; every absence assertion in this file is now vacuous",
					leakParentFixture, sentinel)
			}
		}
		if !strings.Contains(parent, leakSentinelSentence) {
			t.Errorf("%s no longer carries the matched sentence %q, so nothing proves the secret "+
				"reaches the matcher (L-0024)", leakParentFixture, leakSentinelSentence)
		}

		child := leakFixtureText(t, leakChildFixture)
		for _, sentinel := range leakSentinels {
			if strings.Contains(child, sentinel) {
				t.Errorf("%s carries %s; a sentinel in the child would be printable text and the "+
					"test would be measuring the wrong ledger", leakChildFixture, sentinel)
			}
		}
	})

	t.Run("the child cites the parent's private constraint and emits neither sentinel", func(t *testing.T) {
		t.Parallel()

		child, _ := leakWorkspace(t, leakOpts{})

		r := newRunner(t)
		code := r.run("-C", child, leakPlan)

		// Present, then present, then present — and only then absent.
		if code != enforcer.ExitConflict {
			t.Fatalf("exit code is %d, want %d (a cited conflict)\nstdout:\n%s\nstderr:\n%s",
				code, enforcer.ExitConflict, r.stdout.String(), r.stderr.String())
		}
		if !strings.Contains(r.stdout.String(), "me:cst-0002") {
			t.Fatalf("stdout does not cite me:cst-0002, so no parent was resolved and the search "+
				"below would be searching an output that never had anything to leak:\n%s", r.stdout.String())
		}
		if !strings.Contains(r.stdout.String(), "private — cited by reference only") {
			t.Fatalf("the citation is not marked private, so the redaction branch was never taken "+
				"and the absence below is an accident:\n%s", r.stdout.String())
		}

		assertNoLeak(t, "the combined stdout and stderr of `dira check`", r.stdout.String()+r.stderr.String())
	})

	t.Run("the --json document cites it by ref and carries no text", func(t *testing.T) {
		t.Parallel()

		child, _ := leakWorkspace(t, leakOpts{})

		r := newRunner(t)
		code := r.run("-C", child, "-json", leakPlan)
		if code != enforcer.ExitConflict {
			t.Fatalf("exit code is %d, want %d\nstdout:\n%s\nstderr:\n%s",
				code, enforcer.ExitConflict, r.stdout.String(), r.stderr.String())
		}

		// Decoded into a map, and asserted by key ABSENCE. A key present
		// with an empty value is still a statement — `"title": ""` says
		// this entry has no title — and a substring search for the
		// sentinel would pass over it either way.
		var doc map[string]any
		if err := json.Unmarshal(r.stdout.Bytes(), &doc); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, r.stdout.String())
		}
		cited := leakConflict(t, doc, "me:cst-0002")
		if private, _ := cited["private"].(bool); !private {
			t.Fatalf("the document does not mark me:cst-0002 private, so the redaction branch was "+
				"never taken:\n%s", r.stdout.String())
		}
		for _, key := range []string{"title", "rejected_alternative", "why_not"} {
			if v, present := cited[key]; present {
				t.Errorf("the document carries %q (%v) for a private inherited citation; "+
					"cst-0003 rule 3 cites the ref and never the text", key, v)
			}
		}
		if v, present := cited["revisit_if"]; !present || v != nil {
			t.Errorf("revisit_if is %v (present: %t), want present and null", v, present)
		}

		assertNoLeak(t, "the combined --json stdout and stderr of `dira check`",
			r.stdout.String()+r.stderr.String())
	})

	t.Run("neither ledger is written to", func(t *testing.T) {
		t.Parallel()

		child, parent := leakWorkspace(t, leakOpts{})
		childEntries := filepath.Join(child, ".dira", "entries")

		// cst-0003 rule 2 over the child's own record, and rule 1 over
		// everything under the parent. The child's digest is over
		// entries/ because `dira check` legitimately writes its own
		// .dira/cache/; the parent's is over the whole checkout for the
		// opposite reason — .dira/cache/ is gitignored, so a git-based
		// check would be vacuously green against exactly the write that
		// opening a parent through index.Open would perform.
		beforeChild := parentTreeSHA256(t, childEntries)
		beforeParent := parentTreeSHA256(t, parent)

		for _, args := range [][]string{{"-C", child, leakPlan}, {"-C", child, "-json", leakPlan}} {
			r := newRunner(t)
			if code := r.run(args...); code != enforcer.ExitConflict {
				t.Fatalf("`dira check %s` exited %d, want %d; the run under measurement did not "+
					"resolve the parent\nstdout:\n%s", strings.Join(args, " "), code,
					enforcer.ExitConflict, r.stdout.String())
			}
		}

		if after := parentTreeSHA256(t, childEntries); after != beforeChild {
			t.Errorf("the child's entries changed during a check:\n  before %s\n  after  %s\n"+
				"cst-0003 rule 2: inherited context is read at check time and never persisted.",
				beforeChild, after)
		}
		if after := parentTreeSHA256(t, parent); after != beforeParent {
			t.Errorf("the parent tree changed during a check:\n  before %s\n  after  %s\n"+
				"cst-0003 rule 1: a child never writes to a parent.", beforeParent, after)
		}
		if cache := filepath.Join(parent, ".dira", "cache"); !leakMissing(cache) {
			t.Errorf("%s exists after a check; the parent was opened through index.Open", cache)
		}

		// Both digests are shown moving under a real entry written
		// through the real backend, or the two assertions above are
		// green lights wired to nothing.
		leakDigestSees(t, child, filepath.Join(".dira", "entries"), "the child's entries digest")
		leakDigestSees(t, parent, "", "the parent's whole-tree digest")
	})

	t.Run("the parent's tier is the only variable", func(t *testing.T) {
		t.Parallel()

		// The red side of the leak assertion, and the one that matters
		// most: identical fixtures, identical command, identical plan —
		// one word of the parent's config changed. The parent's cst-0002
		// carries no `private: true` precisely so that this is a
		// single-variable experiment. If the title does not appear here,
		// its absence above was never evidence of redaction.
		child, _ := leakWorkspace(t, leakOpts{tier: "repo"})

		for _, args := range [][]string{{"-C", child, leakPlan}, {"-C", child, "-json", leakPlan}} {
			r := newRunner(t)
			if code := r.run(args...); code != enforcer.ExitConflict {
				t.Fatalf("`dira check %s` exited %d, want %d\nstdout:\n%s", strings.Join(args, " "),
					code, enforcer.ExitConflict, r.stdout.String())
			}
			assertLeaks(t, fmt.Sprintf("a repo-tier parent's output for `%s`", strings.Join(args, " ")),
				r.stdout.String(), leakSentinelTitle)
		}
	})

	t.Run("each sentinel is caught when the renderer prints it", func(t *testing.T) {
		t.Parallel()

		// The search itself, shown firing on all three strings through
		// the real binary. leakSentinelText has no rendering path in
		// `dira check` at all — a constraint's body is matched against
		// and never quoted — so its absence above would be green against
		// a build with no redaction whatsoever. What makes that absence
		// mean something is this case plus "the private text is what the
		// match landed on": the string is provably findable when it is
		// printed, and provably reaches the matcher when it is not.
		for _, sentinel := range leakSentinels {
			t.Run(sentinel, func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				writeLedgerTree(t, root, "[ledger]\nname = \"leak-fire\"\ntier = \"repo\"\n",
					map[string]string{"cst-0002": fmt.Sprintf(leakFireConstraint, sentinel)})

				for _, args := range [][]string{{"-C", root, leakPlan}, {"-C", root, "-json", leakPlan}} {
					r := newRunner(t)
					if code := r.run(args...); code != enforcer.ExitConflict {
						t.Fatalf("`dira check %s` exited %d, want %d\nstdout:\n%s",
							strings.Join(args, " "), code, enforcer.ExitConflict, r.stdout.String())
					}
					assertLeaks(t, "`dira check "+strings.Join(args, " ")+"`", r.stdout.String(), sentinel)
				}
			})
		}
	})

	t.Run("the fixture's own bytes are printable when nothing redacts them", func(t *testing.T) {
		t.Parallel()

		// The same two fixture files, read as one local ledger. It shows
		// the renderer emitting the fixture's real title and its real
		// rejected alternative and why_not — so the sentinels above are
		// absent because something removed them, not because the fixture
		// happens to hold text no renderer would ever reach.
		root := t.TempDir()
		leakLedger(t, root, "[ledger]\nname = \"parent-as-local\"\ntier = \"repo\"\n", leakParentFixture, nil)

		for _, tc := range []struct{ plan, sentinel string }{
			{leakPlan, leakSentinelTitle},
			{leakDecisionPlan, leakSentinelDecision},
		} {
			r := newRunner(t)
			if code := r.run("-C", root, tc.plan); code != enforcer.ExitConflict {
				t.Fatalf("`dira check %q` over the fixture as a local ledger exited %d, want %d\n"+
					"stdout:\n%s", tc.plan, code, enforcer.ExitConflict, r.stdout.String())
			}
			assertLeaks(t, fmt.Sprintf("the local verdict on %q", tc.plan), r.stdout.String(), tc.sentinel)
		}
	})

	t.Run("the citation comes from the parent and nowhere else", func(t *testing.T) {
		t.Parallel()

		// Without this, every assertion above holds of a check that
		// exited 2 on the child's own ledger and never crossed a
		// boundary at all.
		child, _ := leakWorkspace(t, leakOpts{declaration: `# me = { path = "../../parent" }`})

		r := newRunner(t)
		if code := r.run("-C", child, leakPlan); code != enforcer.ExitCompliant {
			t.Fatalf("the child with its parent commented out exited %d, want %d; the citation above "+
				"did not come from the parent\nstdout:\n%s", code, enforcer.ExitCompliant, r.stdout.String())
		}
		if strings.Contains(r.stdout.String(), "me:") {
			t.Errorf("a commented-out declaration still resolved a parent:\n%s", r.stdout.String())
		}
	})

	t.Run("the private text is what the match landed on", func(t *testing.T) {
		t.Parallel()

		// L-0024's first half, applied. A fixture whose secret never
		// reaches the code under test passes an absence assertion
		// against a build with no redaction in it at all. Here the
		// secret is *inside* the sentence the plan matches, so the
		// citation is itself the proof it arrived — and removing that
		// one line has to take the verdict with it.
		edited := 0
		child, _ := leakWorkspace(t, leakOpts{parentEdit: func(text string) string {
			out := strings.Replace(text, leakSentinelSentence+"\n", "", 1)
			if out != text {
				edited++
			}
			return out
		}})
		if edited != 1 {
			t.Fatalf("the sentinel sentence was removed from %d parent entries, want exactly 1; "+
				"this case is not measuring what it claims", edited)
		}

		r := newRunner(t)
		if code := r.run("-C", child, leakPlan); code != enforcer.ExitCompliant {
			t.Fatalf("with the sentinel-bearing sentence deleted the check still exited %d, want %d — "+
				"so the conflict above did not depend on the secret reaching the matcher, and its "+
				"absence from the output proves nothing (L-0024)\nstdout:\n%s",
				code, enforcer.ExitCompliant, r.stdout.String())
		}
	})
}

// --- the search ------------------------------------------------------------ //

// leakForms enumerates the byte sequences one sentinel could arrive in.
//
// A verbatim search alone is not enough, and docs/lore.md L-0023 is why: this
// module normalises text on the way in — internal/enforcer/text.go lowercases,
// replaces every non-alphanumeric rune with a space and stems what is left —
// so a leak that travelled through a normalising path arrives as
// `sentinel privat text` and a search for SENTINEL-PRIVATE-TEXT finds nothing.
// L-0023 is the same failure in the redactor: screening a normalised title
// passes a live token, because normalisation destroyed the exact characters the
// pattern was anchored on. A leak detector anchored on exact characters has the
// identical blind spot.
//
// The last form is the one that closes it. `SENTINEL` survives every
// transformation this module performs — it has no punctuation to strip and no
// suffix the stemmer touches — so a case-insensitive search for it catches a
// leak in any casing, any separator and any stemmed form. It is safe to search
// for because no legitimate output of `dira check` contains the word: the
// verdict is built from the plan, the citations and the fixed message strings,
// and "the fixtures still carry what is searched for" pins the child fixture as
// sentinel-free.
func leakForms(sentinel string) []string {
	words := strings.Split(sentinel, "-")

	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		for _, v := range []string{s, strings.ToLower(s)} {
			if v == "" || seen[v] {
				continue
			}
			seen[v] = true
			out = append(out, v)
		}
	}
	for _, sep := range []string{"-", " ", "", "_"} {
		add(strings.Join(words, sep))
	}
	add(words[0])
	return out
}

// leakHits counts every form of every sentinel occurring in text.
func leakHits(text string) map[string]int {
	seen := map[string]bool{}
	hits := map[string]int{}
	for _, sentinel := range leakSentinels {
		for _, form := range leakForms(sentinel) {
			if seen[form] {
				continue
			}
			seen[form] = true
			if n := strings.Count(text, form); n > 0 {
				hits[form] = n
			}
		}
	}
	return hits
}

// assertNoLeak requires zero occurrences of every form of every sentinel.
func assertNoLeak(t *testing.T, where, text string) {
	t.Helper()

	if strings.TrimSpace(text) == "" {
		// An empty output contains no secret. Every caller asserts a
		// citation first, so this can only fire if one stops doing so.
		t.Fatalf("%s is empty; an absence assertion over nothing is not evidence", where)
	}
	for form, n := range leakHits(text) {
		t.Errorf("%s carries %d occurrence(s) of %q.\ncst-0003: a private entry is cited by "+
			"reference and never by text.\n--- output ---\n%s", where, n, form, text)
	}
}

// assertLeaks is assertNoLeak's opposite, and every absence in this file is
// paired with one. It requires the verbatim sentinel — not a normalised
// variant — so that a demonstration cannot be satisfied by the loose form the
// absence search adds for safety.
func assertLeaks(t *testing.T, where, text, sentinel string) {
	t.Helper()

	if n := strings.Count(text, sentinel); n == 0 {
		t.Errorf("%s does not contain %s. The search that reports this string absent elsewhere "+
			"cannot be shown to fire, so those absences prove nothing.\n--- output ---\n%s",
			where, sentinel, text)
	}
}

// --- fixtures -------------------------------------------------------------- //

// leakOpts are the knobs the cases above turn, one at a time.
type leakOpts struct {
	// tier is the parent's [ledger].tier. Empty means "person", which is
	// what makes every entry of the parent ref-only.
	tier string

	// declaration is the child's [parents] line verbatim, so a case can
	// commit a live declaration or a commented-out one.
	declaration string

	// parentEdit rewrites each parent entry file on its way into the temp
	// tree. The fixture on disk is never modified.
	parentEdit func(string) string
}

// leakWorkspace materialises the two fixtures as sibling checkouts under one
// temp root and returns their directories.
//
// `../../parent` is relative to the child's *.dira*, which is what makes
// root/child/.dira/../../parent the sibling root/parent.
func leakWorkspace(t *testing.T, opts leakOpts) (child, parent string) {
	t.Helper()

	tier := opts.tier
	if tier == "" {
		tier = "person"
	}
	declaration := opts.declaration
	if declaration == "" {
		declaration = `me = { path = "../../parent" }`
	}

	root := t.TempDir()
	child = filepath.Join(root, "child")
	parent = filepath.Join(root, "parent")

	leakLedger(t, child,
		fmt.Sprintf("[ledger]\nname = \"child-repo\"\ntier = \"repo\"\n\n[parents]\n%s\n", declaration),
		leakChildFixture, nil)

	// The parent's own [ledger].name is not the key the child declares it
	// under: dec-0011 makes the declared key the thing a ref resolves
	// through, and the only way to keep that honest is to make the two
	// disagree.
	leakLedger(t, parent,
		fmt.Sprintf("[ledger]\nname = \"a-person\"\ntier = %q\n", tier),
		leakParentFixture, opts.parentEdit)

	return child, parent
}

// leakLedger copies a fixture's entry files into a real .dira/ tree at root and
// writes the config that ledger is to be read with.
//
// The config is written here rather than committed beside the fixture for two
// reasons: the fixture is a flat directory of *.md by design (L-0014, and
// because that is the shape internal/enforcer/testdata/corpus.yaml references),
// and the tier is the single variable "the parent's tier is the only variable"
// turns.
func leakLedger(t *testing.T, root, cfg, fixture string, edit func(string) string) {
	t.Helper()

	entries := filepath.Join(root, ".dira", "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatalf("building the fixture ledger: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".dira", "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("building the fixture ledger: %v", err)
	}

	names, err := os.ReadDir(fixture)
	if err != nil {
		t.Fatalf("reading the fixture ledger %s: %v", fixture, err)
	}
	copied := 0
	for _, name := range names {
		if name.IsDir() || filepath.Ext(name.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(fixture, name.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", name.Name(), err)
		}
		text := string(data)
		if edit != nil {
			text = edit(text)
		}
		if err := os.WriteFile(filepath.Join(entries, name.Name()), []byte(text), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name.Name(), err)
		}
		copied++
	}
	if copied == 0 {
		// A fixture that materialised nothing would make every test over
		// it pass by having nothing to contradict and nothing to leak.
		t.Fatalf("no entry files in %s; the fixture ledger is empty", fixture)
	}
}

// leakFixtureText concatenates a fixture's entry files, for the guard that the
// sentinels are still there to be searched for.
func leakFixtureText(t *testing.T, fixture string) string {
	t.Helper()

	names, err := os.ReadDir(fixture)
	if err != nil {
		t.Fatalf("reading the fixture ledger %s: %v", fixture, err)
	}
	var b strings.Builder
	for _, name := range names {
		if name.IsDir() || filepath.Ext(name.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(fixture, name.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", name.Name(), err)
		}
		b.Write(data)
	}
	if b.Len() == 0 {
		t.Fatalf("no entry files in %s", fixture)
	}
	return b.String()
}

// leakFireConstraint is a constraint whose *title* carries the sentinel under
// test, so that "each sentinel is caught when the renderer prints it" exercises
// a path the human renderer and RenderJSON both print — a constraint's title —
// for every one of the three strings, including the one the check has no way to
// print out of a real fixture.
const leakFireConstraint = `---
id: cst-0002
kind: constraint
title: %s the founder holds one commitment at a time
state: active
created: "2026-06-14T09:00:00Z"
tags: [fixture, boundary]
source:
  hook: manual
  tier: human
confirmed_by: human
---

No second side project is started here.
`

// leakConflict finds one citation in a decoded --json document.
func leakConflict(t *testing.T, doc map[string]any, entry string) map[string]any {
	t.Helper()

	conflicts, ok := doc["conflicts"].([]any)
	if !ok || len(conflicts) == 0 {
		t.Fatalf("the document cites nothing, so there is no citation to inspect: %v", doc)
	}
	for _, raw := range conflicts {
		c, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if c["entry"] == entry {
			return c
		}
	}
	t.Fatalf("the document does not cite %s: %v", entry, conflicts)
	return nil
}

// leakMissing reports whether a path is absent, distinguishing that from a stat
// that failed for another reason.
func leakMissing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist)
}

// leakDigestSees shows a digest moving under a real write, so that the
// unchanged-digest assertions are measuring the tree rather than returning a
// constant.
//
// The write goes through internal/ledger/local — the same backend `dira` writes
// entries with — into a *copy* of the directory under measurement, so nothing
// here touches the tree the assertion is about.
// rel is the part of the tree the digest under test covers, relative to the
// checkout root: "" for the parent's whole-tree digest, ".dira/entries" for the
// child's.
func leakDigestSees(t *testing.T, root, rel, what string) {
	t.Helper()

	twin := t.TempDir()
	copyTree(t, root, twin)
	target := filepath.Join(twin, rel)
	before := parentTreeSHA256(t, target)

	store, err := local.Open(filepath.Join(twin, ".dira"))
	if err != nil {
		t.Fatalf("%s: opening the twin: %v", what, err)
	}
	entry := &ledger.Entry{
		ID:      "cst-0099",
		Kind:    ledger.KindConstraint,
		Title:   "a byte written under the ledger being measured",
		State:   ledger.StateActive,
		Created: "2026-06-01T09:00:00Z",
	}
	if err := store.Create(context.Background(), entry); err != nil {
		t.Fatalf("%s: writing an entry into the twin: %v", what, err)
	}

	if parentTreeSHA256(t, target) == before {
		t.Errorf("%s is unchanged after a real entry was written under it; it is not measuring the "+
			"tree, and the no-write assertion it backs is a green light wired to nothing", what)
	}
}
