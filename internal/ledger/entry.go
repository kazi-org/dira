// Package ledger is dira's model of a ledger: the Entry type, the codec that
// maps an entry to and from its on-disk file, and the Store interface every
// read and write goes through.
//
// Nothing in this package knows what a path is. dec-0005 commits dira to a
// storage interface with two implementations — local (the filesystem, this
// repo's internal/ledger/local) and github (the Contents API, E7) — decided
// before the first surface ships, because retrofitting one later is the refactor
// that never happens. The mechanical form of that commitment is that this
// package imports no os, no io/fs and no path/filepath, enforced by
// TestNoFilesystemImportsAboveTheBackend rather than by care.
package ledger

import (
	"fmt"
	"regexp"
	"strings"
)

// Kind is the entry vocabulary. It is closed at five, and cst-0002 names the
// specific tool dira becomes for each plausible sixth: a task kind makes it a
// tracker, an event kind a calendar, a journal kind an Obsidian. Adding one
// requires superseding that constraint.
type Kind string

// The five kinds.
const (
	KindIntent     Kind = "intent"
	KindDecision   Kind = "decision"
	KindQuestion   Kind = "question"
	KindConstraint Kind = "constraint"
	KindNote       Kind = "note"
)

// Kinds lists the vocabulary in schema order.
var Kinds = []Kind{KindIntent, KindDecision, KindQuestion, KindConstraint, KindNote}

// State is an entry's lifecycle state. No state here describes execution
// progress: planned, in-progress, completed and blocked are derived by joining
// kazi (E4) and are never stored (dec-0004).
type State string

// The states. Which of them a given entry may carry depends on its kind; see
// Kind.States.
const (
	StateActive     State = "active"
	StateAchieved   State = "achieved"
	StateAbandoned  State = "abandoned"
	StateAccepted   State = "accepted"
	StateRejected   State = "rejected"
	StateSuperseded State = "superseded"
	StateOpen       State = "open"
	StateAnswered   State = "answered"
	StateStaged     State = "staged"
)

// States returns the states valid for this kind, mirroring the per-kind allOf
// rules in entry.schema.json. The two are held in agreement by
// TestKindStatesMatchTheSchema, which reads the schema rather than trusting
// this list.
//
// A kind outside the vocabulary has no valid states, which is what makes
// Validate reject it rather than accept anything.
func (k Kind) States() []State {
	switch k {
	case KindIntent:
		return []State{StateActive, StateAchieved, StateAbandoned}
	case KindDecision:
		return []State{StateAccepted, StateRejected, StateSuperseded, StateStaged}
	case KindQuestion:
		return []State{StateOpen, StateAnswered}
	case KindConstraint:
		return []State{StateActive, StateSuperseded}
	case KindNote:
		return []State{StateActive, StateAbandoned}
	default:
		return nil
	}
}

// Prefix returns the id prefix that encodes this kind.
func (k Kind) Prefix() string {
	switch k {
	case KindIntent:
		return "int"
	case KindDecision:
		return "dec"
	case KindQuestion:
		return "qst"
	case KindConstraint:
		return "cst"
	case KindNote:
		return "note"
	default:
		return ""
	}
}

// KindForPrefix maps an id prefix back to its kind. The second result is false
// for a prefix outside the vocabulary.
func KindForPrefix(prefix string) (Kind, bool) {
	for _, k := range Kinds {
		if k.Prefix() == prefix {
			return k, true
		}
	}
	return "", false
}

// EdgeType is the closed set of typed, directed relationships between entries.
type EdgeType string

// The edge types. realized_by is the only one whose target is not a dira ref.
const (
	EdgeSupersedes  EdgeType = "supersedes"
	EdgeDerivesFrom EdgeType = "derives_from"
	EdgeInforms     EdgeType = "informs"
	EdgeBlocks      EdgeType = "blocks"
	EdgeRealizedBy  EdgeType = "realized_by"
)

// EdgeTypes lists the edge vocabulary in schema order.
var EdgeTypes = []EdgeType{EdgeSupersedes, EdgeDerivesFrom, EdgeInforms, EdgeBlocks, EdgeRealizedBy}

// Hook names the capture point that produced an entry.
type Hook string

// The capture points.
const (
	HookSessionStart    Hook = "SessionStart"
	HookStop            Hook = "Stop"
	HookPreCompact      Hook = "PreCompact"
	HookManual          Hook = "manual"
	HookImport          Hook = "import"
	HookKaziDisposition Hook = "kazi-disposition"
)

// Hooks lists the capture points in schema order.
var Hooks = []Hook{HookSessionStart, HookStop, HookPreCompact, HookManual, HookImport, HookKaziDisposition}

// Tier is how an entry was extracted. There is no "api" tier because dira never
// embeds a model client (dec-0003).
type Tier string

// The extraction tiers.
const (
	TierRegex    Tier = "regex"
	TierSemantic Tier = "semantic"
	TierHuman    Tier = "human"
)

// Tiers lists the extraction tiers in schema order.
var Tiers = []Tier{TierRegex, TierSemantic, TierHuman}

// An Edge is a typed, directed relationship. Edges are stored on the subject
// entry, so any mutation is a single file write and therefore a single GitHub
// PUT (dec-0002). A design that needed two files written atomically would be
// the wrong design.
type Edge struct {
	Type EdgeType
	To   string
	Note string
}

// An Alternative is a road not taken, with the reason. A decision without
// alternatives is an assertion, and the schema requires at least one.
type Alternative struct {
	Option    string
	WhyNot    string
	RevisitIf string
}

// Source is provenance: how an entry came to exist, so a human can audit what a
// hook inferred against what a human confirmed.
type Source struct {
	Hook    Hook
	Session string
	Excerpt string
	Tier    Tier
}

// An Entry is one ledger entry: the YAML frontmatter fields plus the markdown
// body that follows them.
//
// Created and Updated are strings, not time.Time, and that is load-bearing.
// yaml.v3 resolves an unquoted RFC3339 scalar to the !!timestamp tag, and a
// codec that let the value become a time.Time would re-emit it in a shape the
// schema's date-time format rejects — which has already broken validation in
// this repo once. The codec reads the raw scalar text and always quotes on
// write; see TestTimestampsSurviveAsStrings.
type Entry struct {
	ID           string
	Kind         Kind
	Title        string
	State        State
	Created      string
	Updated      string
	Tags         []string
	Edges        []Edge
	Alternatives []Alternative
	Source       *Source
	ConfirmedBy  string
	ADR          string
	Private      bool

	// Body is everything after the closing frontmatter delimiter, byte for
	// byte including trailing newlines. It is the entry's prose "because",
	// content rather than a field, and the codec never reformats it.
	Body string

	// style remembers how the frontmatter scalars were written on disk, so
	// re-serialising an entry nobody edited reproduces the file exactly and
	// adding one edge to an entry produces a one-line diff rather than a
	// reflow of every paragraph in it. It holds presentation only — never
	// values, never raw bytes — and is dropped for any value that changed.
	style styleMemo

	// version is the backend's opaque content version, set on read and left
	// empty on an entry the caller built. See Store.
	version string
}

// Version returns the backend's opaque version for this entry, or "" for an
// entry that did not come from a Store. It changes if and only if the stored
// content changed, which is what lets a derived cache detect staleness (E1-L3)
// without re-reading every file, and what the github backend will pass back as
// the blob sha it needs to write safely (E7).
//
// The value is opaque: only the backend that produced it may interpret it.
func (e *Entry) Version() string { return e.version }

// idPattern is the `id` pattern from entry.schema.json. Ids are ledger-local:
// the namespaced form (sire:int-0002) is a ref, not an id.
var idPattern = regexp.MustCompile(`^(int|dec|qst|cst|note)-[0-9]{4,}$`)

// refPattern is the `$defs/ref` pattern: an id, optionally namespaced by a
// ledger name that resolves through the ledger map in .dira/config.toml.
var refPattern = regexp.MustCompile(`^([a-z][a-z0-9_-]*:)?(int|dec|qst|cst|note)-[0-9]{4,}$`)

// tagPattern is the `tags` item pattern.
var tagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ValidID reports whether s is a well-formed, ledger-local entry id.
func ValidID(s string) bool { return idPattern.MatchString(s) }

// ValidRef reports whether s is a well-formed entry reference, bare or
// namespaced.
func ValidRef(s string) bool { return refPattern.MatchString(s) }

// Validate checks an entry against the rules entry.schema.json states, and
// returns the first violation.
//
// This is not a second copy of the schema for its own sake. The schema is the
// published contract and is checked by a JSON Schema validator in tests; this
// is the runtime gate, in the command path, where compiling a JSON Schema
// document on every invocation would spend int-0002's whole budget.
// TestValidateAgreesWithTheSchema holds the two in agreement over the ledger
// and over E0's invalid fixtures, so they cannot drift apart silently.
func (e *Entry) Validate() error {
	if !ValidID(e.ID) {
		return fmt.Errorf("id %q does not match %s", e.ID, idPattern)
	}
	if e.Kind.Prefix() == "" {
		return fmt.Errorf("kind %q is not one of the five (cst-0002 closes the set: %s)", e.Kind, joinKinds())
	}
	if prefix, _, _ := strings.Cut(e.ID, "-"); prefix != e.Kind.Prefix() {
		return fmt.Errorf("id %q carries the prefix %q but kind is %q, whose prefix is %q", e.ID, prefix, e.Kind, e.Kind.Prefix())
	}
	if n := len([]rune(e.Title)); n < 3 || n > 120 {
		return fmt.Errorf("title is %d characters, want 3 to 120", n)
	}
	if !validState(e.Kind, e.State) {
		return fmt.Errorf("state %q is not valid for kind %q, which allows %s", e.State, e.Kind, joinStates(e.Kind.States()))
	}
	if e.Created == "" {
		return fmt.Errorf("created is required")
	}
	if err := validTimestamp("created", e.Created); err != nil {
		return err
	}
	if e.Updated != "" {
		if err := validTimestamp("updated", e.Updated); err != nil {
			return err
		}
	}
	// A staged decision is exempt, and the exemption is the whole reason the
	// state exists. `dira sniff` captures decision language with a regular
	// expression (dec-0003); it can see that a choice was announced and has
	// no way to know what was rejected or why. Requiring an alternative here
	// would leave it two moves, both dishonest: write `alternatives: []`,
	// which satisfies the rule while asserting that the roads not taken were
	// considered and found to be none, or invent a why_not that gets quoted
	// back at the user as their own reasoning. So the requirement attaches at
	// the moment a human stands behind the entry, not before.
	if e.Kind == KindDecision && e.State != StateStaged && len(e.Alternatives) == 0 {
		return fmt.Errorf("a decision must record at least the alternative of not doing it (a staged one need not: nobody has decided yet)")
	}

	for i, edge := range e.Edges {
		if !validEdgeType(edge.Type) {
			return fmt.Errorf("edges[%d]: type %q is not one of %s", i, edge.Type, joinEdgeTypes())
		}
		if edge.To == "" {
			return fmt.Errorf("edges[%d]: to is required", i)
		}
		// realized_by is the one edge whose target is an external
		// execution artifact rather than a dira ref, so it is the one
		// edge whose target the ref pattern must not be applied to.
		if edge.Type != EdgeRealizedBy && !ValidRef(edge.To) {
			return fmt.Errorf("edges[%d]: to %q is not an entry ref", i, edge.To)
		}
		if n := len([]rune(edge.Note)); n > 280 {
			return fmt.Errorf("edges[%d]: note is %d characters, want at most 280", i, n)
		}
	}

	for i, alt := range e.Alternatives {
		if alt.Option == "" {
			return fmt.Errorf("alternatives[%d]: option is required", i)
		}
		if n := len([]rune(alt.Option)); n > 200 {
			return fmt.Errorf("alternatives[%d]: option is %d characters, want at most 200", i, n)
		}
		if alt.WhyNot == "" {
			return fmt.Errorf("alternatives[%d]: why_not is required — an option without a reason is not an alternative", i)
		}
	}

	seen := make(map[string]bool, len(e.Tags))
	for i, tag := range e.Tags {
		if !tagPattern.MatchString(tag) {
			return fmt.Errorf("tags[%d]: %q does not match %s", i, tag, tagPattern)
		}
		if seen[tag] {
			return fmt.Errorf("tags[%d]: %q is a duplicate", i, tag)
		}
		seen[tag] = true
	}

	if e.Source != nil {
		if e.Source.Hook != "" && !validHook(e.Source.Hook) {
			return fmt.Errorf("source.hook %q is not one of %s", e.Source.Hook, joinHooks())
		}
		if e.Source.Tier != "" && !validTier(e.Source.Tier) {
			return fmt.Errorf("source.tier %q is not one of %s", e.Source.Tier, joinTiers())
		}
		if n := len([]rune(e.Source.Excerpt)); n > 1000 {
			return fmt.Errorf("source.excerpt is %d characters, want at most 1000 — this is evidence for review, not a transcript archive", n)
		}
	}
	return nil
}

func validState(k Kind, s State) bool {
	for _, want := range k.States() {
		if s == want {
			return true
		}
	}
	return false
}

func validEdgeType(t EdgeType) bool {
	for _, want := range EdgeTypes {
		if t == want {
			return true
		}
	}
	return false
}

func validHook(h Hook) bool {
	for _, want := range Hooks {
		if h == want {
			return true
		}
	}
	return false
}

func validTier(t Tier) bool {
	for _, want := range Tiers {
		if t == want {
			return true
		}
	}
	return false
}

func joinKinds() string {
	parts := make([]string, len(Kinds))
	for i, k := range Kinds {
		parts[i] = string(k)
	}
	return strings.Join(parts, ", ")
}

func joinStates(states []State) string {
	parts := make([]string, len(states))
	for i, s := range states {
		parts[i] = string(s)
	}
	return strings.Join(parts, ", ")
}

func joinEdgeTypes() string {
	parts := make([]string, len(EdgeTypes))
	for i, t := range EdgeTypes {
		parts[i] = string(t)
	}
	return strings.Join(parts, ", ")
}

func joinHooks() string {
	parts := make([]string, len(Hooks))
	for i, h := range Hooks {
		parts[i] = string(h)
	}
	return strings.Join(parts, ", ")
}

func joinTiers() string {
	parts := make([]string, len(Tiers))
	for i, t := range Tiers {
		parts[i] = string(t)
	}
	return strings.Join(parts, ", ")
}
