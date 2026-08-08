package enforcer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/kazi-org/dira/internal/config"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// Every fixture in this file is written into a real temp `.dira/` tree and
// opened through local.Open, never pointed at testdata/ledgers/<name>/.
//
// docs/lore.md L-0014: those directories are flat piles of *.md and not ledgers
// dira can open. local.Find walks *up*, so a command pointed at one silently
// grades against this repository's own .dira with no error at all — a test that
// did that would pass while measuring nothing. The cost is that each case builds
// its ledger from strings; the benefit is that when a case says the parent held
// two constraints, it held two constraints.

// The parent every case inherits from unless it says otherwise: a person-tier
// ledger that calls itself something other than the key the child declares it
// under, holding two active constraints and one of every kind that must not
// cross the boundary.
const parentOwnName = "sire-workspace"

// parentConfig is the parent's own .dira/config.toml. Its [ledger].name is
// deliberately not the namespace the child declares — dec-0011 makes the
// declared key the thing a ref resolves through, and the only way to prove that
// is to make the two disagree.
func parentConfig(tier string) string {
	return fmt.Sprintf("[ledger]\nname = %q\ntier = %q\n", parentOwnName, tier)
}

// hiringConstraint is the parent's public-ish active constraint: not marked
// private, so it is the entry that proves the person-tier rule is unconditional.
const hiringConstraint = `---
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

// privateConstraint is the parent's second active constraint, marked private in
// the entry itself.
const privateConstraint = `---
id: cst-0002
kind: constraint
title: the partnership conversation stays inside this ledger until it is signed
state: active
created: "2026-06-02T09:00:00Z"
tags: [fixture, partnership]
private: true
source:
  hook: manual
  tier: human
confirmed_by: human
---

Nothing about the partnership conversation is written into a downstream ledger,
quoted into a public artefact, or named in a commit message.
`

// noUnitEntries are one of every entry the enforcement table stops at the
// boundary: a parent's decisions in all four states, its questions, intents,
// notes, and a superseded constraint.
func noUnitEntries() map[string]string {
	return map[string]string{
		"cst-0003": `---
id: cst-0003
kind: constraint
title: the workspace publishes a quarterly letter to its advisers
state: superseded
created: "2026-05-01T09:00:00Z"
tags: [fixture, reporting]
source:
  hook: manual
  tier: human
confirmed_by: human
---

Superseded; the letter became a monthly note.
`,
		"dec-0001": `---
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
`,
		"dec-0002": `---
id: dec-0002
kind: decision
title: opening a second office was refused
state: rejected
created: "2026-05-03T09:00:00Z"
tags: [fixture, offices]
alternatives:
  - option: a second office in another city
    why_not: it doubles the fixed cost against no measured gain
source:
  hook: manual
  tier: human
confirmed_by: human
---

Refused.
`,
		"dec-0003": `---
id: dec-0003
kind: decision
title: we will move the engineering hire forward rather than wait for the runway
state: staged
created: "2026-05-04T09:00:00Z"
tags: [fixture, hiring]
source:
  hook: Stop
  session: 1f0c6a3e-0000-4000-8000-000000000009
  excerpt: we will move the engineering hire forward rather than wait for the runway
  tier: regex
---
`,
		"dec-0004": `---
id: dec-0004
kind: decision
title: the quarterly adviser letter was chosen over an open metrics page
state: superseded
created: "2026-05-05T09:00:00Z"
tags: [fixture, reporting]
alternatives:
  - option: an open metrics page anyone could read
    why_not: it commits the workspace to a cadence it has not tested
source:
  hook: manual
  tier: human
confirmed_by: human
---

Replaced.
`,
		"qst-0001": `---
id: qst-0001
kind: question
title: does the workspace need a second engineering hire before the runway rebuilds
state: open
created: "2026-05-06T09:00:00Z"
tags: [fixture, hiring]
source:
  hook: manual
  tier: human
---

Open.
`,
		"int-0001": `---
id: int-0001
kind: intent
title: the workspace reaches twelve months of runway without an engineering hire
state: active
created: "2026-05-07T09:00:00Z"
tags: [fixture, runway]
source:
  hook: manual
  tier: human
---

Direction.
`,
		"note-0001": `---
id: note-0001
kind: note
title: an engineering hire was discussed again over the runway numbers
state: active
created: "2026-05-08T09:00:00Z"
tags: [fixture, hiring]
source:
  hook: manual
  tier: human
---

Noted.
`,
	}
}

// conflictingPlan restates the parent's hiring constraint. It shares no
// distinctive vocabulary with the daemon fixture this package's other tests use,
// so a citation on it can only have come across the boundary.
const conflictingPlan = "make an engineering hire next month even though runway is under twelve months"

func TestInherit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("the enforcement set gains the parent's active constraints, namespaced", func(t *testing.T) {
		t.Parallel()

		inh := inheritFrom(t, ctx, personParent(t), config.Parent{Name: "me"})

		got := citedEntries(inh.units)
		want := []string{"me:cst-0001", "me:cst-0002"}
		if !equalStrings(got, want) {
			t.Errorf("inherited citations %v, want %v", got, want)
		}
		if n := inh.Parents[0].Evaluated; n != 2 {
			t.Errorf("reported %d constraints evaluated, want 2", n)
		}
	})

	t.Run("the declared key wins over the parent's own [ledger].name", func(t *testing.T) {
		t.Parallel()

		inh := inheritFrom(t, ctx, personParent(t), config.Parent{Name: "me"})

		for _, entry := range citedEntries(inh.units) {
			if strings.HasPrefix(entry, parentOwnName+":") {
				t.Errorf("citation %q is namespaced with the parent's own [ledger].name.\n"+
					"dec-0011 resolves a ref through the key the CHILD declares it under; the two disagree "+
					"in this fixture precisely so that reading the wrong one is visible.", entry)
			}
			if !strings.HasPrefix(entry, "me:") {
				t.Errorf("citation %q is not namespaced with the declared key `me`", entry)
			}
		}
	})

	t.Run("nothing but an active constraint crosses the boundary", func(t *testing.T) {
		t.Parallel()

		dir := personParent(t)
		recorder := &recordingStore{Store: openStore(t, dir)}
		inh, err := Inherit(ctx, []Parent{{
			Decl:   config.Parent{Name: "me"},
			Config: []byte(parentConfig("person")),
			Store:  recorder,
		}})
		if err != nil {
			t.Fatalf("Inherit: %v", err)
		}

		// The fixture has to actually hold the entries this is asserting
		// about, or the case is green against an empty parent.
		for id := range noUnitEntries() {
			if _, err := os.Stat(filepath.Join(dir, "entries", id+".md")); err != nil {
				t.Fatalf("fixture is missing %s, so this case is measuring nothing: %v", id, err)
			}
		}

		for _, entry := range citedEntries(inh.units) {
			id := strings.TrimPrefix(entry, "me:")
			if _, forbidden := noUnitEntries()[id]; forbidden {
				t.Errorf("%s produced an enforcement unit; the table in docs/plan/lanes/E3.md "+
					"crosses the boundary for constraint/active and nothing else", entry)
			}
		}

		// Stronger than "produced no unit": a parent's decisions, questions,
		// intents and notes are never read at all, so text this check may
		// not use never enters the process.
		for _, id := range recorder.fetched() {
			if !strings.HasPrefix(id, "cst-") {
				t.Errorf("the check fetched %s out of the parent; only constraints may be read across the boundary", id)
			}
		}
		if len(recorder.fetched()) != 3 {
			t.Errorf("fetched %v, want the three cst- entries (two active, one superseded)", recorder.fetched())
		}
	})

	t.Run("a person-tier parent makes every citation private", func(t *testing.T) {
		t.Parallel()

		inh := inheritFrom(t, ctx, personParent(t), config.Parent{Name: "me"})

		// cst-0001 is NOT marked private in the entry. cst-0003 says the
		// rule is unconditional, so the tier alone has to carry it.
		for _, u := range inh.units {
			if !u.citation.Private {
				t.Errorf("%s is cited with Private=false out of a person-tier parent.\n"+
					"cst-0003 is a rule and not a mode: every entry of a person-tier ledger is cited by ref only.",
					u.citation.Entry)
			}
		}
	})

	t.Run("a repo-tier parent's unmarked constraint is not private", func(t *testing.T) {
		t.Parallel()

		// The green side of the clause above. Without it, an implementation
		// that marked everything private would pass the private case and
		// prove nothing.
		inh := inheritFrom(t, ctx, repoParent(t), config.Parent{Name: "sire"})

		public, private := map[string]bool{}, map[string]bool{}
		for _, u := range inh.units {
			if u.citation.Private {
				private[u.citation.Entry] = true
			} else {
				public[u.citation.Entry] = true
			}
		}
		if !public["sire:cst-0001"] {
			t.Error("an unmarked constraint in a repo-tier parent was cited as private; " +
				"the private rule would then be unfalsifiable")
		}
		if !private["sire:cst-0002"] {
			t.Error("an entry marked `private: true` was cited as public in a repo-tier parent")
		}
	})

	t.Run("a parent the child declared private is private whatever its tier says", func(t *testing.T) {
		t.Parallel()

		inh := inheritFrom(t, ctx, repoParent(t), config.Parent{
			Name:       "me",
			Visibility: config.VisibilityPrivate,
		})
		for _, u := range inh.units {
			if !u.citation.Private {
				t.Errorf("%s is cited with Private=false out of a parent the child declared "+
					"`visibility = \"private\"`", u.citation.Entry)
			}
		}
	})

	t.Run("an unreadable parent config falls closed", func(t *testing.T) {
		t.Parallel()

		inh, err := Inherit(ctx, []Parent{{
			Decl:   config.Parent{Name: "me"},
			Config: nil, // the config could not be read
			Store:  openStore(t, repoParent(t)),
		}})
		if err != nil {
			t.Fatalf("Inherit: %v", err)
		}
		if len(inh.units) == 0 {
			t.Fatal("no units, so nothing is being asserted")
		}
		for _, u := range inh.units {
			if !u.citation.Private {
				t.Errorf("%s is cited with Private=false out of a parent whose tier could not be read.\n"+
					"internal/config falls closed on a visibility it cannot read; an unknown tier is the same bet.",
					u.citation.Entry)
			}
		}
	})

	t.Run("the order of inherited units is total", func(t *testing.T) {
		t.Parallel()

		local := fixtureEntries(t, daemonLedger)

		// Eight runs rather than two. One comparison cannot distinguish a
		// total order from a shuffle that happened to land the same way
		// twice: over a parent of two constraints a map-ordered
		// implementation agrees with itself about half the time, so a
		// two-run check would be a coin toss dressed as an assertion.
		var order [8][]string
		var rendered [8]string
		for i := range rendered {
			// Two separately built parents, opened through two stores,
			// so a stable order cannot come from a shared slice.
			inh := inheritFrom(t, ctx, personParent(t), config.Parent{Name: "me"})
			order[i] = unitOrder(inh.units)

			v, err := CheckInherited(ctx, stubLedger{entries: local}, conflictingPlan, inh)
			if err != nil {
				t.Fatalf("CheckInherited: %v", err)
			}
			var b bytes.Buffer
			if err := Render(&b, v); err != nil {
				t.Fatalf("Render: %v", err)
			}
			rendered[i] = b.String()
		}

		// The units themselves, not only what they rendered to. A
		// verdict sorts its conflicts before printing, so comparing two
		// renders would pass over a unit order that was shuffled on
		// every run — the assertion has to reach the sequence the
		// ordering claim is about.
		for i := 1; i < len(order); i++ {
			if !equalStrings(order[0], order[i]) {
				t.Fatalf("run %d produced a different unit sequence:\n first: %v\n  run%d: %v",
					i, order[0], i, order[i])
			}
			if rendered[i] != rendered[0] {
				t.Fatalf("run %d rendered different bytes:\n--- first ---\n%s\n--- run %d ---\n%s",
					i, rendered[0], i, rendered[i])
			}
		}
		if len(order[0]) < 2 {
			t.Fatalf("%d inherited units, so no ordering is being asserted", len(order[0]))
		}

		// Stable is not enough on its own: an implementation with a fixed
		// but arbitrary order would satisfy every comparison above. The
		// order is by id, ascending, so the sequence is predictable from
		// the fixture rather than merely reproducible.
		if got := entriesOf(order[0]); !equalStrings(got, []string{"me:cst-0001", "me:cst-0002"}) {
			t.Errorf("units appear in the order %v, want the ids ascending", got)
		}
		if !strings.Contains(rendered[0], "me:cst-0001") {
			t.Fatalf("the plan cited no inherited constraint, so the ordering case is comparing "+
				"two copies of a local-only verdict:\n%s", rendered[0])
		}
	})

	t.Run("Verdict.Enforced is local plus inherited", func(t *testing.T) {
		t.Parallel()

		local := fixtureEntries(t, daemonLedger)
		child := stubLedger{entries: local}

		alone, err := Check(ctx, child, conflictingPlan)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		inh := inheritFrom(t, ctx, personParent(t), config.Parent{Name: "me"})
		with, err := CheckInherited(ctx, child, conflictingPlan, inh)
		if err != nil {
			t.Fatalf("CheckInherited: %v", err)
		}

		inherited := len(citedEntries(inh.units))
		if inherited == 0 {
			t.Fatal("the parent contributed no entries, so the sum below is not a sum")
		}
		if want := alone.Enforced + inherited; with.Enforced != want {
			t.Errorf("Enforced = %d, want %d (%d local + %d inherited)",
				with.Enforced, want, alone.Enforced, inherited)
		}
		// The red side of the same clause: an inheritance that silently
		// contributed nothing would leave the two counts equal, and every
		// assertion above about units would still hold of an empty set.
		if with.Enforced == alone.Enforced {
			t.Errorf("Enforced is %d with and without parents; inheritance contributed nothing", with.Enforced)
		}

		if alone.Enforced == 0 {
			t.Fatal("the child ledger enforces nothing, so 'local plus inherited' is only 'inherited'")
		}
		if len(with.Conflicts) == 0 || with.Conflicts[0].Entry != "me:cst-0001" {
			t.Errorf("the plan cited %v, want the inherited me:cst-0001 first", citedConflicts(with))
		}
	})

	t.Run("a nil inheritance is exactly Check", func(t *testing.T) {
		t.Parallel()

		local := fixtureEntries(t, daemonLedger)
		alone, err := Check(ctx, stubLedger{entries: local}, demoPlan)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		with, err := CheckInherited(ctx, stubLedger{entries: local}, demoPlan, nil)
		if err != nil {
			t.Fatalf("CheckInherited: %v", err)
		}
		if with.Enforced != alone.Enforced || len(with.Conflicts) != len(alone.Conflicts) {
			t.Errorf("a nil inheritance changed the verdict: %d/%d conflicts over %d/%d enforced",
				len(with.Conflicts), len(alone.Conflicts), with.Enforced, alone.Enforced)
		}
	})

	t.Run("a parent that cannot be read is reported and changes no verdict", func(t *testing.T) {
		t.Parallel()

		want := errors.New("the parent is on another machine")
		inh, err := Inherit(ctx, []Parent{{
			Decl:  config.Parent{Name: "me"},
			Store: failingStore{err: want},
		}})
		if err != nil {
			t.Fatalf("Inherit turned an unreadable parent into an error: %v", err)
		}
		if len(inh.Parents) != 1 || !errors.Is(inh.Parents[0].Err, want) {
			t.Fatalf("the unreadable parent was not reported: %+v", inh.Parents)
		}
		if n := inh.Parents[0].Evaluated; n != 0 {
			t.Errorf("reported %d constraints evaluated for a parent nobody could open, want 0", n)
		}

		local := fixtureEntries(t, daemonLedger)
		alone, _ := Check(ctx, stubLedger{entries: local}, demoPlan)
		with, err := CheckInherited(ctx, stubLedger{entries: local}, demoPlan, inh)
		if err != nil {
			t.Fatalf("CheckInherited: %v", err)
		}
		if with.ExitCode() != alone.ExitCode() || with.Enforced != alone.Enforced {
			t.Errorf("an unreadable parent moved the verdict: exit %d/%d over %d/%d enforced",
				with.ExitCode(), alone.ExitCode(), with.Enforced, alone.Enforced)
		}
	})

	t.Run("a declaration with no namespace or no store is a caller's bug", func(t *testing.T) {
		t.Parallel()

		if _, err := Inherit(ctx, []Parent{{Decl: config.Parent{Name: "  "}}}); err == nil {
			t.Error("Inherit accepted a parent with no namespace; every citation would read \":cst-0001\"")
		}
		if _, err := Inherit(ctx, []Parent{{Decl: config.Parent{Name: "me"}}}); err == nil {
			t.Error("Inherit accepted a parent with no store")
		}
	})
}

// TestInheritWritesNothingUnderTheParent is cst-0003 rule 1 measured rather than
// asserted.
//
// The digest is over every file under the parent's whole directory, not over
// git's view of it: .dira/cache/ is gitignored precisely because a derived
// artifact holds a private entry's title, so a git-based check reports a clean
// parent while a cache is being written into it. The second half of the test
// moves the tree and requires the digest to move with it — a digest that cannot
// change is a green light wired to nothing.
func TestInheritWritesNothingUnderTheParent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dir := personParent(t)
	root := filepath.Dir(dir) // the whole parent checkout, not just .dira

	before := treeDigest(t, root)

	inh, err := Inherit(ctx, []Parent{{
		Decl:   config.Parent{Name: "me"},
		Config: []byte(parentConfig("person")),
		Store:  openStore(t, dir),
	}})
	if err != nil {
		t.Fatalf("Inherit: %v", err)
	}
	if len(inh.units) == 0 {
		t.Fatal("the parent contributed nothing, so no read happened to measure")
	}
	v, err := CheckInherited(ctx, stubLedger{entries: fixtureEntries(t, daemonLedger)}, conflictingPlan, inh)
	if err != nil {
		t.Fatalf("CheckInherited: %v", err)
	}
	if len(v.Conflicts) == 0 {
		t.Fatal("the plan cited nothing, so the read under measurement was shallower than a real check")
	}

	if after := treeDigest(t, root); after != before {
		t.Errorf("the parent tree changed during a read: %s -> %s\n"+
			"cst-0003 rule 1: a parent is never written to by a child, and dira has no verb that does so.",
			before, after)
	}

	// The digest has to be able to move, or the assertion above is a
	// tautology. This is the write index.Open would have made.
	cache := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatalf("seeding a cache write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cache, "index.db"), []byte("sqlite"), 0o644); err != nil {
		t.Fatalf("seeding a cache write: %v", err)
	}
	if moved := treeDigest(t, root); moved == before {
		t.Errorf("the digest is %s both before and after a file was written under the parent; "+
			"it is not measuring the tree", moved)
	}
}

// --- fixtures ------------------------------------------------------------- //

// personParent writes the person-tier parent into a real temp .dira/ tree and
// returns its .dira directory.
func personParent(t *testing.T) string {
	t.Helper()

	entries := noUnitEntries()
	entries["cst-0001"] = hiringConstraint
	entries["cst-0002"] = privateConstraint
	return writeLedger(t, parentConfig("person"), entries)
}

// repoParent is the same two constraints in a repo-tier ledger — the fixture
// that keeps the person-tier rule falsifiable.
func repoParent(t *testing.T) string {
	t.Helper()

	return writeLedger(t, parentConfig("repo"), map[string]string{
		"cst-0001": hiringConstraint,
		"cst-0002": privateConstraint,
	})
}

func writeLedger(t *testing.T, cfg string, entries map[string]string) string {
	t.Helper()

	diraDir := filepath.Join(t.TempDir(), "parent", ".dira")
	if err := os.MkdirAll(filepath.Join(diraDir, "entries"), 0o755); err != nil {
		t.Fatalf("building the fixture ledger: %v", err)
	}
	if err := os.WriteFile(filepath.Join(diraDir, "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("building the fixture ledger: %v", err)
	}
	for id, text := range entries {
		if err := os.WriteFile(filepath.Join(diraDir, "entries", id+".md"), []byte(text), 0o644); err != nil {
			t.Fatalf("building the fixture ledger: %v", err)
		}
	}
	return diraDir
}

func openStore(t *testing.T, diraDir string) ledger.Store {
	t.Helper()

	store, err := local.Open(diraDir)
	if err != nil {
		t.Fatalf("opening the fixture ledger: %v", err)
	}
	return store
}

// inheritFrom reads one parent, with its own config read off disk rather than
// restated — the tier under test is the one the fixture actually committed.
func inheritFrom(t *testing.T, ctx context.Context, diraDir string, decl config.Parent) *Inherited {
	t.Helper()

	cfg, err := os.ReadFile(filepath.Join(diraDir, "config.toml"))
	if err != nil {
		t.Fatalf("reading the parent's config: %v", err)
	}
	inh, err := Inherit(ctx, []Parent{{Decl: decl, Config: cfg, Store: openStore(t, diraDir)}})
	if err != nil {
		t.Fatalf("Inherit: %v", err)
	}
	if len(inh.units) == 0 {
		t.Fatal("the parent contributed no units, so this case would pass against an empty ledger")
	}
	return inh
}

// --- stores --------------------------------------------------------------- //

// recordingStore remembers which entries were fetched out of a parent.
type recordingStore struct {
	ledger.Store

	mu  sync.Mutex
	got []string
}

func (r *recordingStore) Get(ctx context.Context, id string) (*ledger.Entry, error) {
	r.mu.Lock()
	r.got = append(r.got, id)
	r.mu.Unlock()
	return r.Store.Get(ctx, id)
}

func (r *recordingStore) fetched() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]string(nil), r.got...)
	sort.Strings(out)
	return out
}

// failingStore is a parent that is declared but cannot be read — the shape a
// public clone of a repository with a private parent has.
type failingStore struct{ err error }

func (s failingStore) Get(context.Context, string) (*ledger.Entry, error) { return nil, s.err }
func (s failingStore) List(context.Context) ([]ledger.EntryInfo, error)   { return nil, s.err }
func (s failingStore) Create(context.Context, *ledger.Entry) error        { return s.err }
func (s failingStore) Put(context.Context, *ledger.Entry) error           { return s.err }
func (s failingStore) Delete(context.Context, string) error               { return s.err }

// --- helpers -------------------------------------------------------------- //

// citedEntries is the distinct entries a set of units would cite, in order.
func citedEntries(units []unit) []string {
	seen := map[string]bool{}
	var out []string
	for _, u := range units {
		if seen[u.citation.Entry] {
			continue
		}
		seen[u.citation.Entry] = true
		out = append(out, u.citation.Entry)
	}
	return out
}

// unitOrder fingerprints a unit sequence, in order.
//
// The citation alone would not do it: a constraint becomes one unit per
// sentence and they all cite the same entry, so a fingerprint that stopped at
// the id could not tell a reordered body from an ordered one. The terms are
// taken span by span, in the order Text keeps them.
func unitOrder(units []unit) []string {
	out := make([]string, 0, len(units))
	for _, u := range units {
		var b strings.Builder
		fmt.Fprintf(&b, "%s|%s|", u.citation.Entry, u.citation.Basis)
		for _, sp := range u.text.spans {
			for _, tm := range sp {
				if tm.negated {
					b.WriteString("!")
				}
				b.WriteString(tm.word)
				b.WriteString(" ")
			}
			b.WriteString("/ ")
		}
		out = append(out, b.String())
	}
	return out
}

// entriesOf is the distinct entries a fingerprint sequence cites, in the order
// they first appear.
func entriesOf(fingerprints []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range fingerprints {
		entry, _, _ := strings.Cut(f, "|")
		if seen[entry] {
			continue
		}
		seen[entry] = true
		out = append(out, entry)
	}
	return out
}

func citedConflicts(v *Verdict) []string {
	out := make([]string, 0, len(v.Conflicts))
	for _, c := range v.Conflicts {
		out = append(out, c.Entry)
	}
	return out
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// treeDigest is a sha256 over every file under root: its path and its bytes.
//
// Every file, including the gitignored ones. The write this is watching for is
// index.Open's SQLite database under .dira/cache/, which git cannot see.
func treeDigest(t *testing.T, root string) string {
	t.Helper()

	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if len(paths) == 0 {
		t.Fatalf("no files under %s; the digest would be constant", root)
	}
	sort.Strings(paths)

	sum := sha256.New()
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("relativising %s: %v", path, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		_, _ = fmt.Fprintf(sum, "%s\n%d\n", rel, len(data))
		sum.Write(data)
	}
	return hex.EncodeToString(sum.Sum(nil))
}
