// Package kazi is dira's read-only client over kazi's `--json` surface:
// `kazi portfolio --json` (Snapshot) and `kazi status <ref> --json` (Status).
//
// dec-0008 commits dira to integrating with kazi only through its published
// `--json` contract and hooks, never kazi's internals. `portfolio` is not
// registered in kazi's own command schema (qst-0005), so this package pins
// the shape it was recorded against rather than waiting on that upstream
// answer — the founder decision of 2026-08-14 (dec-0008's newest edge, and
// see qst-0005's own note). PinnedSchemaVersion is the version that pin
// covers; a client seeing a different one degrades to ContractDrift (T5)
// rather than guessing at a shape it was never tested against.
//
// dec-0004: this package derives, it never stores. Nothing here writes to
// `.dira/` — see internal/ledger/boundary_test.go's allowlist, which this
// package is not on, and does not need to be: os/exec is not a filesystem
// package (cst-0004's "shells out to a local binary and nothing else").
package kazi

import (
	"encoding/json"
	"fmt"
)

// PinnedSchemaVersion is the schema_version this client is built against.
// kazi's schema_version is lockstep across its whole --json surface
// (kazi's lib/kazi/cli.ex:95, @run_schema_version), so one pin covers
// portfolio and status alike.
const PinnedSchemaVersion = 2

// UnavailableReason is machine-readable and distinct per cause a caller
// (E4-L2, E4-L3, E4-L4) can switch on rather than parsing Detail.
type UnavailableReason string

const (
	ReasonNotOnPath     UnavailableReason = "not_on_path"
	ReasonNonZeroExit   UnavailableReason = "nonzero_exit"
	ReasonMalformedJSON UnavailableReason = "malformed_json"
	ReasonWrongKind     UnavailableReason = "wrong_kind"
	ReasonTimeout       UnavailableReason = "timeout"
)

// Unavailable is returned (never a bare error) whenever Snapshot or Status
// could not produce a trustworthy answer. dec-0004's degrade-without-guessing
// half: a caller that type-asserts to *Unavailable knows kazi could not be
// asked, and Reason says why.
type Unavailable struct {
	Reason UnavailableReason
	Detail string // exit code observed, the offending `kind`, etc. — for humans, never switched on
}

func (u *Unavailable) Error() string {
	if u.Detail == "" {
		return fmt.Sprintf("kazi unavailable: %s", u.Reason)
	}
	return fmt.Sprintf("kazi unavailable: %s (%s)", u.Reason, u.Detail)
}

// RepoBucket is portfolio.ex:38's three-value enum — by_repo and
// fleet_remote only. Never decoded into RowBucket or vice versa: the two
// enums are distinct Go types precisely so a value from one vocabulary
// landing where the other belongs fails to decode instead of silently
// becoming the zero value (docs/lore.md L-0004).
type RepoBucket string

const (
	RepoInProgress RepoBucket = "in_progress"
	RepoStuck      RepoBucket = "stuck"
	RepoComplete   RepoBucket = "complete"
)

// repoBuckets lists the closed set RepoBucket accepts.
var repoBuckets = []RepoBucket{RepoInProgress, RepoStuck, RepoComplete}

// UnmarshalText validates against the closed three-value set. Implementing
// this (rather than leaving RepoBucket a bare string alias) is what makes an
// out-of-vocabulary value — including a RowBucket value like "planned" — a
// decode error instead of an unnoticed RepoBucket("").
func (b *RepoBucket) UnmarshalText(text []byte) error {
	v, err := parseRepoBucket(string(text))
	if err != nil {
		return err
	}
	*b = v
	return nil
}

func parseRepoBucket(s string) (RepoBucket, error) {
	v := RepoBucket(s)
	for _, want := range repoBuckets {
		if v == want {
			return v, nil
		}
	}
	return "", fmt.Errorf("kazi: %q is not a valid RepoBucket (want one of %v)", s, repoBuckets)
}

// RowBucket is portfolio.ex:55's five-value enum — totals.rows[].bucket and
// the top-level todo/blocked arrays only.
type RowBucket string

const (
	RowDone    RowBucket = "done"
	RowRunning RowBucket = "running"
	RowBlocked RowBucket = "blocked"
	RowTodo    RowBucket = "todo"
	RowPlanned RowBucket = "planned"
)

// rowBuckets lists the closed set RowBucket accepts.
var rowBuckets = []RowBucket{RowDone, RowRunning, RowBlocked, RowTodo, RowPlanned}

// UnmarshalText is RowBucket's half of the two-enum split; see RepoBucket's.
func (b *RowBucket) UnmarshalText(text []byte) error {
	v, err := parseRowBucket(string(text))
	if err != nil {
		return err
	}
	*b = v
	return nil
}

func parseRowBucket(s string) (RowBucket, error) {
	v := RowBucket(s)
	for _, want := range rowBuckets {
		if v == want {
			return v, nil
		}
	}
	return "", fmt.Errorf("kazi: %q is not a valid RowBucket (want one of %v)", s, rowBuckets)
}

// Cause is blocked[].cause's closed vocabulary. cause: dag never occurs on a
// machine with no :starmap_roadmap_goal app-env goal configured (lane doc
// point 9), which is why testdata/kazi/portfolio-all-causes.json hand-extends
// a natural recording rather than relying on one to occur.
type Cause string

const (
	CauseDAG        Cause = "dag"
	CauseOverBudget Cause = "over_budget"
	CauseError      Cause = "error"
	CauseStuck      Cause = "stuck"
)

// causes lists the closed set Cause accepts.
var causes = []Cause{CauseDAG, CauseOverBudget, CauseError, CauseStuck}

// UnmarshalText validates against the closed four-value set.
func (c *Cause) UnmarshalText(text []byte) error {
	v := Cause(text)
	for _, want := range causes {
		if v == want {
			*c = v
			return nil
		}
	}
	return fmt.Errorf("kazi: %q is not a valid Cause (want one of %v)", text, causes)
}

// A RepoRun is one entry of by_repo[repo][bucket] or fleet_remote — carries
// Status, the RAW persisted run status string (which can be "terminated" and
// other values bucket() folds into RepoInProgress — lane doc point 3), so a
// caller can see past the bucket when the bucket lies.
//
// Bucket carries no `json` tag: by_repo's run objects do not name their own
// bucket (it is the enclosing map key), while fleet_remote's do. decodeSnapshot
// fills Bucket in for the by_repo case from the key it was read under, after
// that key has been validated as a RepoBucket.
type RepoRun struct {
	GoalRef string     `json:"goal_ref"`
	RunID   string     `json:"run_id"`
	Status  string     `json:"status"`
	Bucket  RepoBucket `json:"bucket,omitempty"`
}

// A Proposal is one entry of the top-level planned[] array — the prop -> goal
// bridge dec-0008 §2 describes.
type Proposal struct {
	ProposalRef string `json:"proposal_ref"`
	GoalID      string `json:"goal_id"`
	Idea        string `json:"idea"`
	Status      string `json:"status"`
}

// A TotalsRow is one entry of totals.rows[] — the headline percentage table,
// keyed by the five-value RowBucket.
type TotalsRow struct {
	Bucket RowBucket `json:"bucket"`
	Count  int       `json:"count"`
	Pct    float64   `json:"pct"`
}

// A BlockedEntry is one entry of the top-level blocked[] array. kazi's own
// entries carry more fields than this (red_predicates, iterations, cap,
// blocked_by — portfolio.ex:153-163), but GoalRef/Cause/Blocker are the ones
// this lane's decode needs; the rest are not load-bearing here (E4-L3's
// evidence-by-reference concern, not this package's).
type BlockedEntry struct {
	GoalRef string `json:"goal_ref"`
	Cause   Cause  `json:"cause"`
	Blocker string `json:"blocker"`
}

// Snapshot is the decoded portfolio. ContractDrift is set, not returned as an
// error, when SchemaVersion != PinnedSchemaVersion (T5) — the snapshot is
// still populated best-effort. decodeSnapshot itself never inspects
// SchemaVersion for this purpose: that comparison belongs to Snapshot() (T4,
// T5), so the decode layer is correct regardless of which kazi release
// produced the bytes.
type Snapshot struct {
	SchemaVersion int
	ContractDrift bool

	Planned     []Proposal
	ByRepo      map[string]map[RepoBucket][]RepoRun // undeduped, no timestamp — lane doc point 2
	FleetRemote []RepoRun
	TotalsBase  int
	TotalsEmpty bool // totals.empty
	TotalsRows  []TotalsRow
	Todo        []map[string]any // shape not load-bearing for this lane; carried opaque
	Blocked     []BlockedEntry
	RateTotal   int
	RateGreen   int
	RateEmpty   bool // rate.empty? — the DIFFERENT key from totals.empty
	RateDelta   int
}

// rawPortfolio is the wire shape decodeSnapshot reads. by_repo's inner map is
// keyed by a raw string here rather than RepoBucket, because the conversion
// has to happen after decode: each run in the slice also needs its Bucket
// field filled in from that same key, which a struct tag cannot express.
type rawPortfolio struct {
	SchemaVersion int                             `json:"schema_version"`
	Planned       []Proposal                      `json:"planned"`
	ByRepo        map[string]map[string][]RepoRun `json:"by_repo"`
	FleetRemote   []RepoRun                       `json:"fleet_remote"`
	Totals        rawTotals                       `json:"totals"`
	Todo          []map[string]any                `json:"todo"`
	Blocked       []BlockedEntry                  `json:"blocked"`
	Rate          rawRate                         `json:"rate"`
}

type rawTotals struct {
	Base  int         `json:"base"`
	Empty bool        `json:"empty"`
	Rows  []TotalsRow `json:"rows"`
}

type rawRate struct {
	Total int  `json:"total"`
	Green int  `json:"green"`
	Empty bool `json:"empty?"`
	Delta int  `json:"delta"`
}

// decodeSnapshot decodes a `kazi portfolio --json` document into a Snapshot.
//
// It is kind- and version-agnostic on purpose: whether the bytes came from a
// `kind: "portfolio"` document and whether schema_version is the pinned one
// are both Snapshot()'s concerns (T4 validates kind before calling this at
// all; T5 sets ContractDrift by comparing SchemaVersion after this returns),
// because a decode layer that refused an unpinned version could not support
// T5's best-effort promise.
func decodeSnapshot(data []byte) (*Snapshot, error) {
	var raw rawPortfolio
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("kazi: decoding portfolio: %w", err)
	}

	byRepo := make(map[string]map[RepoBucket][]RepoRun, len(raw.ByRepo))
	for repo, buckets := range raw.ByRepo {
		converted := make(map[RepoBucket][]RepoRun, len(buckets))
		for rawBucket, runs := range buckets {
			bucket, err := parseRepoBucket(rawBucket)
			if err != nil {
				return nil, fmt.Errorf("kazi: decoding portfolio: by_repo[%q]: %w", repo, err)
			}
			for i := range runs {
				runs[i].Bucket = bucket
			}
			converted[bucket] = runs
		}
		byRepo[repo] = converted
	}

	return &Snapshot{
		SchemaVersion: raw.SchemaVersion,
		Planned:       raw.Planned,
		ByRepo:        byRepo,
		FleetRemote:   raw.FleetRemote,
		TotalsBase:    raw.Totals.Base,
		TotalsEmpty:   raw.Totals.Empty,
		TotalsRows:    raw.Totals.Rows,
		Todo:          raw.Todo,
		Blocked:       raw.Blocked,
		RateTotal:     raw.Rate.Total,
		RateGreen:     raw.Rate.Green,
		RateEmpty:     raw.Rate.Empty,
		RateDelta:     raw.Rate.Delta,
	}, nil
}
