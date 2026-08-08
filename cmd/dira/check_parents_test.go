package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/config"
	"github.com/kazi-org/dira/internal/enforcer"
	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// Every ledger in this file is written into a real temp `.dira/` tree.
//
// docs/lore.md L-0014: internal/enforcer/testdata/ledgers/<name>/ is a flat pile
// of *.md and not a ledger dira can open, and local.Find walks *up* — so a
// command pointed at one grades against this repository's own .dira with no
// error at all. That failure is not hypothetical here: this file's whole subject
// is a command resolving a second ledger by path, and every assertion below
// would be green against the wrong one. The last case therefore pins
// enforced_entries to a number the fixtures themselves produce.

// The plan under test. It restates the parent's constraint and shares no
// distinctive vocabulary with the child, so a citation on it can only have come
// across the boundary.
const parentConflictPlan = "make an engineering hire next month even though runway is under twelve months"

// parentHiringConstraint is the parent's one active constraint — the entry a
// resolved parent contributes and an unresolved one does not.
const parentHiringConstraint = `---
id: cst-0001
kind: constraint
title: no engineering hire is made before the workspace holds twelve months of runway
state: active
created: "2026-06-01T09:00:00Z"
tags: [fixture, hiring]
source:
  hook: manual
  tier: human
confirmed_by: human
---

An engineering hire is the largest irreversible commitment this workspace makes.
No engineering hire is made while runway is under twelve months, whatever the
pipeline looks like.
`

// parentPayrollDecision is a parent entry that must contribute nothing: the
// enforcement table crosses the boundary for constraint/active and no other row.
// It is here so the count the last case asserts is a count of what inherited
// rather than of what the parent happens to hold.
const parentPayrollDecision = `---
id: dec-0001
kind: decision
title: the workspace runs its own payroll rather than an employer of record
state: accepted
created: "2026-05-02T09:00:00Z"
tags: [fixture, payroll]
alternatives:
  - option: an employer of record for every engineering hire
    why_not: the margin is a whole month of runway a year
source:
  hook: manual
  tier: human
confirmed_by: human
---

Payroll is run in house.
`

// decoySentinel is the string a parent resolved against the process working
// directory would put on stdout. It appears in no other fixture in this file.
const decoySentinel = "SENTINEL-DECOY-PARENT"

// decoyHiringConstraint is a *different* parent, reachable only by resolving the
// declared path against the working directory. It matches the same plan, so an
// implementation that resolved the wrong way would still exit 2 and still cite a
// constraint — the failure this fixture exists to make visible rather than
// invisible.
const decoyHiringConstraint = `---
id: cst-0003
kind: constraint
title: ` + decoySentinel + ` no engineering hire is made before the workspace holds twelve months of runway
state: active
created: "2026-06-01T09:00:00Z"
tags: [fixture, hiring]
source:
  hook: manual
  tier: human
confirmed_by: human
---

` + decoySentinel + ` an engineering hire is the largest irreversible commitment
this workspace makes. No engineering hire is made while runway is under twelve
months, whatever the pipeline looks like.
`

// The child's own entries. Two are enforcement substrate and one is not, so the
// enforced count below is a count and not a file tally. None of them shares
// vocabulary with the plan.
const (
	childConfigDecision = `---
id: dec-0001
kind: decision
title: the command path links no TOML library
state: accepted
created: "2026-06-10T09:00:00Z"
tags: [fixture, dependencies]
alternatives:
  - option: link a full TOML parser into the binary
    why_not: every dependency is paid for on every invocation in a hook's latency path
source:
  hook: manual
  tier: human
confirmed_by: human
---

Three scalars do not justify a dependency.
`

	childOfflineConstraint = `---
id: cst-0002
kind: constraint
title: the binary reaches no network to reach a verdict
state: active
created: "2026-06-11T09:00:00Z"
tags: [fixture, offline]
source:
  hook: manual
  tier: human
confirmed_by: human
---

Every number is computed in this process from the files on this disk.
`

	childNote = `---
id: note-0001
kind: note
title: the TOML reader was reviewed again this week
state: active
created: "2026-06-12T09:00:00Z"
tags: [fixture, dependencies]
source:
  hook: manual
  tier: human
---

Noted.
`
)

// childEnforceable and parentEnforceable are the entries each fixture ledger
// contributes to Verdict.Enforced, named rather than counted by hand.
var (
	childEnforceable  = []string{"dec-0001", "cst-0002"}
	parentEnforceable = []string{"cst-0001"}
)

// TestCheckResolvesParents is E3-L3-T4.
func TestCheckResolvesParents(t *testing.T) {
	t.Parallel()

	t.Run("the child cites the parent's active constraint and exits 2", func(t *testing.T) {
		t.Parallel()

		child, _ := childWithParent(t, `me = { path = "../../parent" }`)

		r := newRunner(t)
		code := r.run("-C", child, parentConflictPlan)

		if code != enforcer.ExitConflict {
			t.Fatalf("exit code is %d, want %d (a cited conflict)\nstdout:\n%s\nstderr:\n%s",
				code, enforcer.ExitConflict, r.stdout.String(), r.stderr.String())
		}
		for _, want := range []string{
			"me:cst-0001",                      // namespaced with the key the CHILD declared
			"no engineering hire is made",      // the parent's entry was really read
			"enforced by the parent ledger me", // the remedy names the ledger that owns it
		} {
			if !strings.Contains(r.stdout.String(), want) {
				t.Errorf("stdout does not contain %q:\n%s", want, r.stdout.String())
			}
		}
	})

	t.Run("the same child with no parent declared is compliant", func(t *testing.T) {
		t.Parallel()

		// The red side. Without it every assertion above would hold of a
		// check that cited the constraint out of the child's own ledger,
		// or of one that exited 2 for an unrelated reason.
		child, _ := childWithParent(t, `# me = { path = "../../parent" }`)

		r := newRunner(t)
		code := r.run("-C", child, parentConflictPlan)

		if code != enforcer.ExitCompliant {
			t.Fatalf("a child with its parent commented out exited %d, want %d; the citation in the case "+
				"above did not come from the parent\nstdout:\n%s", code, enforcer.ExitCompliant, r.stdout.String())
		}
		if strings.Contains(r.stdout.String(), "me:") {
			t.Errorf("a commented-out declaration still resolved a parent:\n%s", r.stdout.String())
		}
	})

	t.Run("the parent is opened through a store that refuses to write", func(t *testing.T) {
		t.Parallel()

		// The structural half of the no-upward-write claim. The digest
		// below measures what happened; this measures what *can*.
		child, parent := childWithParent(t, `me = { path = "../../parent" }`)
		store, resolved := openParent(filepath.Join(child, ".dira"), config.Parent{Name: "me", Path: "../../parent"})

		if want := filepath.Join(parent, ".dira"); resolved != want {
			t.Fatalf("the declaration resolved to %q, want %q", resolved, want)
		}
		ctx := context.Background()
		if _, err := store.List(ctx); err != nil {
			t.Fatalf("the read path is closed too: %v", err)
		}
		if _, err := store.Get(ctx, "cst-0001"); err != nil {
			t.Fatalf("reading the parent's constraint: %v", err)
		}

		entry := &ledger.Entry{ID: "cst-0009", Kind: ledger.KindConstraint, Title: "written from a child",
			State: ledger.StateActive, Created: "2026-06-01T09:00:00Z"}
		for name, err := range map[string]error{
			"Create": store.Create(ctx, entry),
			"Put":    store.Put(ctx, entry),
			"Delete": store.Delete(ctx, "cst-0001"),
		} {
			if !errors.Is(err, ledger.ErrReadOnly) {
				t.Errorf("%s on a parent returned %v, want an error wrapping ledger.ErrReadOnly "+
					"(cst-0003 rule 1)", name, err)
			}
		}
	})

	t.Run("the run writes no byte under the parent and leaves no cache", func(t *testing.T) {
		t.Parallel()

		child, parent := childWithParent(t, `me = { path = "../../parent" }`)

		// The digest is over every file under the parent's whole
		// checkout, not over git's view of it: .gitignore ignores
		// .dira/cache/ precisely because a derived artifact holds a
		// private entry's title, so a git-based check reports a clean
		// parent while a cache is being written into it.
		before := parentTreeSHA256(t, parent)

		r := newRunner(t)
		if code := r.run("-C", child, parentConflictPlan); code != enforcer.ExitConflict {
			t.Fatalf("exit code is %d, want %d; the run under measurement did not resolve the parent",
				code, enforcer.ExitConflict)
		}
		if !strings.Contains(r.stdout.String(), "me:cst-0001") {
			t.Fatalf("the run cited no inherited constraint, so it read less of the parent than a real "+
				"check does:\n%s", r.stdout.String())
		}

		if after := parentTreeSHA256(t, parent); after != before {
			t.Errorf("the parent tree changed during a check:\n  before %s\n  after  %s\n"+
				"cst-0003 rule 1: a parent is never written to by a child, and dira has no verb that does so.",
				before, after)
		}
		cache := filepath.Join(parent, ".dira", "cache")
		if _, err := os.Stat(cache); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s exists after a check; the parent was opened through index.Open", cache)
		}

		// The digest has to be able to move, or the assertion above is a
		// tautology — and it is shown moving under the exact write the
		// obligation is about, not under a hand-placed file. A second
		// copy of the same parent is opened the way `dira check` opens
		// its *own* ledger; the cache appears and the digest follows it.
		twin := t.TempDir()
		copyTree(t, parent, twin)
		twinBefore := parentTreeSHA256(t, twin)

		twinDira := filepath.Join(twin, ".dira")
		backend, err := local.Open(twinDira)
		if err != nil {
			t.Fatalf("opening the twin: %v", err)
		}
		ix, err := index.Open(context.Background(), backend, local.CacheDir(twinDira))
		if err != nil {
			t.Fatalf("index.Open over the twin: %v", err)
		}
		if _, err := ix.Select(context.Background(), index.Selector{}); err != nil {
			t.Fatalf("reading the twin through the index: %v", err)
		}
		_ = ix.Close()

		if _, err := os.Stat(filepath.Join(twinDira, "cache")); err != nil {
			t.Fatalf("index.Open wrote no cache under the twin, so this red side is measuring nothing: %v", err)
		}
		if parentTreeSHA256(t, twin) == twinBefore {
			t.Error("the digest is unchanged after index.Open wrote a cache under the ledger; " +
				"it is not measuring the tree, and the no-write assertion above is a green light wired to nothing")
		}
	})

	t.Run("enforced_entries is the fixtures' own count", func(t *testing.T) {
		t.Parallel()

		child, _ := childWithParent(t, `me = { path = "../../parent" }`)

		r := newRunner(t)
		if code := r.run("-C", child, "-json", parentConflictPlan); code != enforcer.ExitConflict {
			t.Fatalf("exit code is %d, want %d\nstderr:\n%s", code, enforcer.ExitConflict, r.stderr.String())
		}

		var doc struct {
			Enforced  int `json:"enforced_entries"`
			Conflicts []struct {
				Entry string `json:"entry"`
			} `json:"conflicts"`
		}
		if err := json.Unmarshal(r.stdout.Bytes(), &doc); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, r.stdout.String())
		}

		// This repository's own ledger holds tens of entries. A run that
		// resolved to it — the L-0014 failure, which produces no error —
		// cannot produce this number.
		want := len(childEnforceable) + len(parentEnforceable)
		if doc.Enforced != want {
			t.Errorf("enforced_entries is %d, want %d (%d from the child fixture, %d inherited).\n"+
				"A count larger than this means the check graded against a ledger that is not the fixture.",
				doc.Enforced, want, len(childEnforceable), len(parentEnforceable))
		}

		// Same guard from the other end: every citation names an id one
		// of the two fixtures actually holds.
		known := map[string]bool{}
		for _, id := range childEnforceable {
			known[id] = true
		}
		for _, id := range parentEnforceable {
			known["me:"+id] = true
		}
		if len(doc.Conflicts) == 0 {
			t.Fatal("the document cites nothing, so the ids below are not being checked")
		}
		for _, c := range doc.Conflicts {
			if !known[c.Entry] {
				t.Errorf("the document cites %s, which neither fixture holds", c.Entry)
			}
		}
	})

	t.Run("a declared parent that is not there changes no verdict", func(t *testing.T) {
		t.Parallel()

		// Reporting the unresolved parent is E3-L3-T7's, and nothing
		// here asserts a report. What is asserted is that resolution
		// itself survives the configuration a public clone has: the
		// command still runs, still exits on its local verdict, and the
		// missing parent contributes nothing rather than erroring.
		child, parent := childWithParent(t, `me = { path = "../../parent" }`)
		if err := os.RemoveAll(parent); err != nil {
			t.Fatalf("removing the parent: %v", err)
		}

		r := newRunner(t)
		if code := r.run("-C", child, parentConflictPlan); code != enforcer.ExitCompliant {
			t.Errorf("a plan that conflicts only with a missing parent exited %d, want %d\nstdout:\n%s\nstderr:\n%s",
				code, enforcer.ExitCompliant, r.stdout.String(), r.stderr.String())
		}

		localOnly := newRunner(t)
		if code := localOnly.run("-C", child, "link a full TOML parser into the binary"); code != enforcer.ExitConflict {
			t.Errorf("a plan that conflicts locally exited %d with a missing parent, want %d\nstdout:\n%s",
				code, enforcer.ExitConflict, localOnly.stdout.String())
		}
	})
}

// TestCheckResolvesParentsFromAnotherDirectory is the half of E3-L3-T4's
// resolution clause that a parallel test cannot make: it changes the process
// working directory, and t.Chdir refuses to run under a parallel ancestor.
//
// It is a separate top-level test rather than a case inside TestCheckResolvesParents
// for that reason alone. `go test -run TestCheckResolvesParents` still selects it,
// because the flag matches on prefix.
func TestCheckResolvesParentsFromAnotherDirectory(t *testing.T) {
	child, _ := childWithParent(t, `me = { path = "../../parent" }`)

	// A second ledger reachable only by resolving `../../parent`
	// against the working directory. It matches the same plan, so an
	// implementation that resolved the wrong way still exits 2 and
	// still cites a constraint; only the id and the text differ.
	decoyRoot := t.TempDir()
	writeLedgerTree(t, filepath.Join(decoyRoot, "parent"),
		"[ledger]\nname = \"decoy\"\ntier = \"repo\"\n",
		map[string]string{"cst-0003": decoyHiringConstraint})
	cwd := filepath.Join(decoyRoot, "work", "here")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("building the third directory: %v", err)
	}
	t.Chdir(cwd)

	r := newRunner(t)
	code := r.run("-C", child, parentConflictPlan)

	if code != enforcer.ExitConflict {
		t.Fatalf("exit code is %d, want %d\nstdout:\n%s\nstderr:\n%s",
			code, enforcer.ExitConflict, r.stdout.String(), r.stderr.String())
	}
	if !strings.Contains(r.stdout.String(), "me:cst-0001") {
		t.Errorf("the real parent was not cited from a third working directory:\n%s", r.stdout.String())
	}
	combined := r.stdout.String() + r.stderr.String()
	if strings.Contains(combined, decoySentinel) || strings.Contains(combined, "me:cst-0003") {
		t.Errorf("the declared path resolved against the process working directory instead of the "+
			"child's .dira directory:\n%s", combined)
	}
}

// --- fixtures ------------------------------------------------------------- //

// childWithParent writes a child ledger and a parent ledger as siblings under
// one temp root, and returns the two checkout directories.
//
// The declaration is written verbatim into the child's [parents] section, so a
// case can commit a live declaration, a commented-out one, or a path that
// resolves nowhere. `../../parent` is relative to the child's *.dira*, which is
// what makes root/child/.dira/../../parent the sibling root/parent.
func childWithParent(t *testing.T, declaration string) (child, parent string) {
	t.Helper()

	root := t.TempDir()
	child = filepath.Join(root, "child")
	parent = filepath.Join(root, "parent")

	writeLedgerTree(t, child,
		fmt.Sprintf("[ledger]\nname = %q\ntier = \"repo\"\n\n[parents]\n%s\n", "child-repo", declaration),
		map[string]string{
			"dec-0001":  childConfigDecision,
			"cst-0002":  childOfflineConstraint,
			"note-0001": childNote,
		})

	// The parent's own [ledger].name is deliberately not the key the child
	// declares it under. dec-0011 makes the declared key the thing a ref
	// resolves through, and the only way to prove that is to make the two
	// disagree.
	writeLedgerTree(t, parent, "[ledger]\nname = \"sire-workspace\"\ntier = \"repo\"\n",
		map[string]string{
			"cst-0001": parentHiringConstraint,
			"dec-0001": parentPayrollDecision,
		})
	return child, parent
}

// writeLedgerTree builds a real .dira/ tree — a config and an entries directory
// — at root. It never points a command at a testdata directory (L-0014).
func writeLedgerTree(t *testing.T, root, cfg string, entries map[string]string) {
	t.Helper()

	dir := filepath.Join(root, ".dira", "entries")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("building the fixture ledger: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".dira", "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("building the fixture ledger: %v", err)
	}
	for id, text := range entries {
		if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(text), 0o644); err != nil {
			t.Fatalf("building the fixture ledger: %v", err)
		}
	}
}

// copyTree copies every file under src into dst, creating directories as it
// goes. It is how the red side of the digest gets a parent it may write to.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copying %s: %v", src, err)
	}
}

// parentTreeSHA256 digests every file under root: its path relative to root, its
// size, and its bytes.
//
// A sha256 over the filesystem rather than a git status, and over the whole
// checkout rather than over entries/. .dira/cache/ is gitignored, so a
// git-based check is vacuously green against exactly the write this is looking
// for; and a digest taken over entries/ alone would miss it for the same reason.
func parentTreeSHA256(t *testing.T, root string) string {
	t.Helper()

	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	sort.Strings(files)

	h := sha256.New()
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("relativising %s: %v", path, err)
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00", rel, len(data))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))
}
