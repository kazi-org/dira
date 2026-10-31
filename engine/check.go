package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/kazi-org/dira/internal/enforcer"
	"github.com/kazi-org/dira/ledger"
)

// Ledger supplies entries from the caller's already authorized read path.
// The facade selects no backend and creates no files or network clients.
type Ledger interface {
	Entries(context.Context) ([]*ledger.Entry, error)
}

const (
	ExitCompliant = enforcer.ExitCompliant
	ExitDiraError = enforcer.ExitDiraError
	ExitConflict  = enforcer.ExitConflict
)

// CheckResult is the privacy-filtered CLI JSON result, not a raw matcher verdict.
// Operational errors return no result; conflicts are successful results with
// ExitCode == ExitConflict. Parents is empty in this initial ledger-only API.
type CheckResult struct {
	Plan      string         `json:"plan"`
	Verdict   string         `json:"verdict"`
	ExitCode  int            `json:"exit_code"`
	Enforced  int            `json:"enforced_entries"`
	Conflicts []Conflict     `json:"conflicts"`
	Notices   []Notice       `json:"notices"`
	Parents   []ParentReport `json:"parents"`
}

// Conflict cites a contradiction. Private citations omit their source text.
type Conflict struct {
	Entry       string       `json:"entry"`
	Kind        ledger.Kind  `json:"kind"`
	State       ledger.State `json:"state"`
	Date        string       `json:"date,omitempty"`
	Private     bool         `json:"private"`
	Basis       string       `json:"basis"`
	Title       string       `json:"title,omitempty"`
	Alternative string       `json:"rejected_alternative,omitempty"`
	WhyNot      string       `json:"why_not,omitempty"`
	RevisitIf   *string      `json:"revisit_if"`
	Supersede   string       `json:"supersede"`
	Score       float64      `json:"score"`
}

// Notice redirects retired thinking without changing the verdict.
type Notice struct {
	SupersededBy        *string `json:"superseded_by"`
	ReplacementEnforced bool    `json:"replacement_enforced"`
	Basis               string  `json:"basis"`
	Score               float64 `json:"score"`
}

// ParentReport mirrors safe CLI reporting; it contains no path or label.
// This initial API does not resolve or evaluate parent declarations.
type ParentReport struct {
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Evaluated int    `json:"evaluated"`
}

// Check evaluates a nonblank plan using the existing offline lexical matcher.
// Like CLI usage validation, an absent plan is an error, never a clean verdict.
// It passes context to the supplied reader, whose I/O and cancellation behavior
// remain the reader's responsibility. No inherited parents are inferred.
func Check(ctx context.Context, reader Ledger, plan string) (*CheckResult, error) {
	if ctx == nil {
		return nil, errors.New("check: context is required")
	}
	if strings.TrimSpace(plan) == "" {
		return nil, errors.New("check: plan is required")
	}
	verdict, err := enforcer.Check(ctx, reader, plan)
	if err != nil {
		return nil, err
	}
	// Reuse the established renderer rather than exporting raw private text or
	// maintaining a second privacy filter and score-rounding implementation.
	var wire bytes.Buffer
	if err := enforcer.RenderJSON(&wire, verdict); err != nil {
		return nil, fmt.Errorf("encode check result: %w", err)
	}
	var result CheckResult
	decoder := json.NewDecoder(&wire)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode check result: %w", err)
	}
	return &result, nil
}
