package enforcer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/kazi-org/dira/internal/config"
	"github.com/kazi-org/dira/internal/ledger"
)

// What a child inherits from a parent, and the three rules that bound it.
//
// # One row of the table crosses the boundary, and it is closed
//
// The enforcement table in docs/plan/lanes/E3.md is closed, and inheritance
// does not widen it: a parent contributes its active constraints and nothing
// else. Its decisions — accepted or rejected — its questions, intents, notes,
// staged captures and superseded entries produce no unit at all.
//
// That is narrower than the local set on purpose. A constraint is the one kind
// the ledger model says inherits downward ("Constraints inherit downward through
// derives_from and are evaluated by every `dira check` in every descendant
// ledger", entry.schema.json), and a decision is a choice a particular ledger
// made in its own context. A parent's rejected alternative is a road *it* did
// not take; refusing a child's plan on it would make every workspace decision
// binding on every repository under it, which is a firewall nobody would leave
// switched on.
//
// # The namespace is the child's word, not the parent's
//
// A citation reads `me:cst-0002`, where `me` is the key under [parents] in the
// *child's* config — never the parent's own [ledger].name. The two can disagree,
// dec-0011 makes the declared key the thing a ref resolves through, and
// scripts/privacy-lint.py P3 resolves committed refs the same way. This file
// therefore reads the parent's config for exactly one field, its tier, and never
// for its name.
//
// # Ref-only is a rule, not a mode
//
// cst-0003 is unconditional: an inherited citation from a private source is
// cited by ref and never by text, in every mode including --json. So the
// Private flag is set here, where the parent is known, rather than inferred
// downstream — and it is set in three cases rather than one:
//
//	the entry says private: true
//	the parent's [ledger].tier is person
//	the child's own declaration says visibility = "private"
//	(and, falling closed, whenever the parent's tier cannot be read at all)
//
// The third and fourth are stricter than E3-L3-T3's acceptance asks for. They
// are here because the cost of being too private is a citation that carries a
// ref and no prose, and the cost of being too public is a private strategic note
// in a public repository's git history, which cannot be un-published. internal/
// config already falls closed the same way on a visibility it cannot read, and
// this is the run-time counterpart of that.
//
// # Nothing here writes
//
// Reading a parent is allowed; writing one is not (cst-0003 rule 1). The store
// this file reads through is wrapped in ledger.ReadOnly whatever the caller
// passed, so the refusal does not depend on the caller having remembered. What
// this cannot do from here is stop a caller opening a parent through index.Open,
// which writes a SQLite file under .dira/cache/ beside the store rather than
// through it — that obligation stays with whoever wires the command, and the
// only honest test for it digests the parent's filesystem tree, because
// .dira/cache/ is gitignored and a git-based check would be vacuously green.

// parentTierPerson is the [ledger].tier whose every entry is private,
// regardless of what the entry itself says.
//
// It is spelled here rather than imported because internal/config reads the
// tier as an opaque string and exports no constant for it. The three tiers are
// named in .dira/config.toml and in dec-0006; only this one changes what a
// citation may print.
const parentTierPerson = "person"

// A Parent is one declared parent ledger, resolved and opened for reading.
//
// It is deliberately not a path. Resolving a declaration to a directory is the
// caller's job — dec-0005 keeps paths inside a storage backend, and this package
// is above the interface — so what arrives here is a store somebody already
// opened and the two documents that say what may be done with it.
type Parent struct {
	// Decl is the child's own declaration: the [parents] entry that named
	// this ledger. Decl.Name is the namespace every citation is qualified
	// with, and Decl.Visibility is the child's statement about whether this
	// parent is one whose text may be printed.
	Decl config.Parent

	// Config is the bytes of the parent's own .dira/config.toml, or nil
	// where it could not be read.
	//
	// Only [ledger].tier is read from it. A nil or unreadable config is not
	// an error: it means the tier is unknown, and an unknown tier is treated
	// as person — the direction that cannot leak.
	Config []byte

	// Store is the read path over the parent. It is wrapped in
	// ledger.ReadOnly before use whatever it already is, so a caller that
	// hands over a writable store still cannot write upward through this
	// package.
	Store ledger.Store
}

// Inherited is what a set of parents contributes to one check.
type Inherited struct {
	// units are the matchable stretches the parents added, in a total
	// order: parents in the order they were declared, entries within a
	// parent by id, units within an entry in the order the entry produces
	// them. Two runs over the same parents produce the same units in the
	// same sequence, which is what makes a rendered verdict reproducible.
	units []unit

	// documents is the text each contributing entry adds to the ledger's
	// document-frequency statistics. See Inherited.adopt.
	documents []Text

	// Parents reports what happened to each declared parent, in declared
	// order. It carries no path, no label and no entry text: dec-0011 says a
	// private parent's label must never ship, and a report is an output like
	// any other.
	Parents []ParentResult
}

// A ParentResult is one parent's outcome, for a caller that has to report it.
//
// It exists because a parent that cannot be read must not silently contribute
// nothing: a check that quietly drops inherited constraints is a firewall with a
// hole in it, and it fails silently in exactly the configuration a public clone
// has. Reporting it is E3-L3-T7's; recording it honestly is this file's.
type ParentResult struct {
	// Namespace is the key the parent was declared under.
	Namespace string

	// Evaluated is how many of the parent's entries produced enforcement
	// units — the number a report may state. It is zero for a parent that
	// could not be read, and that zero is the only count such a parent has:
	// a ledger nobody can open has no countable constraints.
	Evaluated int

	// Err is why this parent contributed nothing, or nil. It is the
	// backend's error, so a caller that prints it must decide what may be
	// shown; nothing in this package puts it into a message.
	Err error
}

// Inherit reads the parents and collects what the child inherits from them.
//
// A parent that cannot be read is recorded in Parents and contributes nothing;
// it is not an error, because an unreachable parent may not change a verdict —
// `dira check`'s exit 2 means a cited conflict and nothing else. The error this
// returns is a caller's bug: a declaration with no namespace, or one with no
// store to read.
func Inherit(ctx context.Context, parents []Parent) (*Inherited, error) {
	inh := &Inherited{}
	for _, p := range parents {
		namespace := strings.TrimSpace(p.Decl.Name)
		if namespace == "" {
			return nil, errors.New("inherit: a parent was declared with no namespace")
		}
		if p.Store == nil {
			return nil, fmt.Errorf("inherit: parent %s has no ledger to read", namespace)
		}

		result := ParentResult{Namespace: namespace}
		entries, err := readConstraints(ctx, ledger.ReadOnly(p.Store))
		if err != nil {
			result.Err = err
			inh.Parents = append(inh.Parents, result)
			continue
		}

		private := forcedPrivate(p)
		for _, e := range entries {
			units := constraintUnits(e, inheritedCitation(namespace, e, private))
			if len(units) == 0 {
				continue
			}
			inh.units = append(inh.units, units...)
			inh.documents = append(inh.documents, documentText(e))
			result.Evaluated++
		}
		inh.Parents = append(inh.Parents, result)
	}
	return inh, nil
}

// CheckInherited is Check over a child ledger and everything it inherits.
//
// A nil Inherited is exactly Check: the two are one code path so that a ledger
// with no parents cannot take a different route to a verdict from one whose
// parents resolved to nothing.
func CheckInherited(ctx context.Context, lg Ledger, plan string, inh *Inherited) (*Verdict, error) {
	if lg == nil {
		return nil, errors.New("check: no ledger")
	}
	entries, err := lg.Entries(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading the ledger: %w", err)
	}
	m := newMatcher(entries)
	inh.adopt(m)
	return m.verdict(plan), nil
}

// adopt folds the inherited units into a matcher built over the child.
//
// The statistics move with them, and that is not optional. idf.rarest states the
// invariant the phrase floor rests on — "A term absent from the ledger entirely
// scores higher still, but no such term can appear in a unit" — and a unit read
// out of a parent whose vocabulary was never observed would break it, making
// every distinctive word in that parent score above the ceiling the floor is a
// share of. So each contributing entry is observed and the floor is recomputed.
//
// Only the entries that contribute units are observed. documentText's rule for
// the local ledger is that every entry counts, including the kinds that are
// never enforcement substrate; across the boundary the rule narrows to what is
// actually enforced, because widening it would pull a parent's private notes and
// decisions into a number this binary publishes in --json. dec-0014 measures
// distinctiveness "across the ledger under test", and for a check with parents
// the ledger under test is the child plus what it inherits — not the child plus
// everything its parents happen to hold.
func (inh *Inherited) adopt(m *matcher) {
	if inh == nil || len(inh.units) == 0 {
		return
	}
	m.units = append(m.units, inh.units...)
	for _, doc := range inh.documents {
		m.weights.observe(doc)
	}
	m.phraseFloor = phraseShare * phraseLength * m.weights.rarest()
}

// inheritedCitation is the citation an inherited unit carries.
//
// The id is namespaced with the declared key, which is what makes the reference
// resolvable from the child and what privacy-lint P3 already checks on committed
// refs. Everything else is read from the entry, as it is locally: a citation is
// filled from the entry file and never from anywhere else, which is what keeps
// dec-0002's files-win property true of every rendered byte.
func inheritedCitation(namespace string, e *ledger.Entry, private bool) Citation {
	return Citation{
		Entry:   namespace + ":" + e.ID,
		Kind:    e.Kind,
		State:   e.State,
		Date:    entryDate(e),
		Private: e.Private || private,
		Title:   e.Title,
	}
}

// forcedPrivate reports whether every entry from this parent is cited by ref
// only, whatever the entries themselves say.
//
// Three of the four ways this is true are statements about the *ledger* rather
// than about an entry, which is why the question is asked once per parent: a
// person-tier ledger holds one person's thinking, a parent the child declared
// private is one the child has said may not be quoted, and a parent whose config
// could not be read is one we know nothing about. Only the fourth — an entry
// marked private: true — is per entry, and inheritedCitation applies it.
func forcedPrivate(p Parent) bool {
	if p.Decl.Private() {
		return true
	}
	return parentTier(p.Config) == parentTierPerson
}

// parentTier reads [ledger].tier out of a parent's config, falling closed.
//
// An absent config, an unreadable one, or one that declares no tier all read as
// person. That is the same bargain internal/config makes with a visibility it
// cannot parse, for the same reason: reading an unknown tier as a public one
// would let a missing file publish a private ledger's prose, and no error a
// reader might have ignored can undo that.
//
// The parse error is deliberately dropped. config.Parse always returns a usable
// Config, and a problem in a *parent's* [brief] section is not this child's to
// report — what matters here is whether the tier came back, and an unreadable
// tier is already handled by falling closed.
func parentTier(data []byte) string {
	if len(data) == 0 {
		return parentTierPerson
	}
	cfg, _ := config.Parse(data)
	switch cfg.Tier {
	case "repo", "workspace":
		return cfg.Tier
	default:
		return parentTierPerson
	}
}

// readConstraints reads a parent's active constraints and nothing else.
//
// It reads the ids first and fetches only those that could be a constraint at
// all. That is a privacy property before it is a performance one: a parent's
// decisions, questions, intents and notes are never fetched across the boundary,
// so text this check may not use never enters the process. The kind and state
// are still checked on the entry after it is read — the id prefix is a filter,
// and the entry is what decides.
//
// The order is by id, which with the parents in declared order makes the whole
// inherited sequence totally ordered. ledger.Store already promises List sorted
// by id; sorting again costs nothing over a constraint set and makes the
// property hold over any backend rather than over the one we have.
func readConstraints(ctx context.Context, store ledger.Store) ([]*ledger.Entry, error) {
	infos, err := store.List(ctx)
	if err != nil {
		return nil, err
	}

	var out []*ledger.Entry
	for _, info := range infos {
		prefix, _, ok := strings.Cut(info.ID, "-")
		if !ok {
			continue
		}
		if kind, known := ledger.KindForPrefix(prefix); !known || kind != ledger.KindConstraint {
			continue
		}
		e, err := store.Get(ctx, info.ID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", info.ID, err)
		}
		if e.Kind != ledger.KindConstraint || e.State != ledger.StateActive {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
