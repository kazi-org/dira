package kazi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// NotFound is returned (as the error, via errors.As) when ref names neither a
// run nor a proposal kazi knows about — distinct from Unavailable, which
// means kazi itself could not be asked. E4-L3's UnresolvedRef vs
// Ambiguous/degraded-join reporting depends on telling the two apart.
type NotFound struct{ Ref string }

func (n *NotFound) Error() string {
	return fmt.Sprintf("kazi: no run or proposal found for %q", n.Ref)
}

// statusNotFoundPrefix is the load-bearing substring of kazi's own
// status_not_found/2 message (lib/kazi/cli.ex:4540) — ported, not
// paraphrased, so Status recognises kazi's actual refusal rather than a
// guess at its shape:
//
//	"no run or proposal found for ref #{inspect(ref)} " <>
//	  "(a run appears once it has recorded an iteration; a proposal once proposed)"
//
// Verified against a live kazi on this machine, 2026-08-14: under --json the
// message arrives as {"error": "...", "schema_version": 2} on STDOUT with
// exit 1 — status_not_found's json?(opts) branch calls emit_json_error/1,
// which IO.puts to stdout, not stderr. Only the non-JSON branch (IO.puts
// :stderr) writes to stderr, and dira always calls --json. This corrects the
// lane doc's "stub exiting 1 ... on stderr" — it describes the human-output
// path, not the one this client uses.
const statusNotFoundPrefix = "no run or proposal found for ref"

// RunStatus and ProposalStatus are the two `kazi status <ref> --json` shapes,
// distinguished by kind.
type RunStatus struct {
	Ref        string
	Status     string // "converged" | "in_progress" — status <ref>'s OWN computed field, narrower than RepoRun.Status
	Converged  bool
	ReleaseRef string
	ObservedAt string
}

// ProposalStatus is the "kind": "proposal" shape.
type ProposalStatus struct {
	Ref    string
	Status string // "proposed" | "approved" | "rejected"
}

// rawRunStatus is the wire shape for kind: "run", read verbatim from
// run_status_json/6 (cli.ex). It carries no run_id: unlike portfolio's
// by_repo/blocked entries, kazi's own status encoder never emits one — the
// "What is already known" surface in docs/plan/lanes/E4.md names a RunID
// field, but nothing on the wire populates it, so RunStatus above omits it
// rather than carry a field that would always read empty. Verified against
// lib/kazi/cli.ex:4564-4581 and a live recording (testdata/kazi/status-run.json).
type rawRunStatus struct {
	Ref        string `json:"ref"`
	Status     string `json:"status"`
	Converged  bool   `json:"converged"`
	ReleaseRef string `json:"release_ref"`
	ObservedAt string `json:"observed_at"`
}

type rawProposalStatus struct {
	Ref    string `json:"ref"`
	Status string `json:"status"`
}

// statusError is the {"error": "...", "schema_version": N} shape
// status_not_found/2 emits under --json.
type statusError struct {
	Error string `json:"error"`
}

// runLifecycleValues are the only values status <ref>'s own computed
// `status` field takes — lane doc point 4: "documented status values are
// only converged / in_progress" — narrower than by_repo's raw persisted
// Status string, which can be anything a run recorded (e.g. "terminated").
var runLifecycleValues = []string{"converged", "in_progress"}

func validRunLifecycle(s string) bool {
	for _, want := range runLifecycleValues {
		if s == want {
			return true
		}
	}
	return false
}

// Status shells `kazi status <ref> --json`, decodes kind: "run" into
// RunStatus and kind: "proposal" into ProposalStatus, and distinguishes
// NotFound (kazi answered: it knows no such ref) from Unavailable (kazi
// could not be asked). Exactly one of the two pointers is non-nil on a nil
// error.
func Status(ctx context.Context, ref string) (*RunStatus, *ProposalStatus, error) {
	stdout, exitCode, err := kaziRunner(ctx, []string{"status", ref, "--json"})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, nil, &Unavailable{Reason: ReasonTimeout, Detail: err.Error()}
		}
		return nil, nil, &Unavailable{Reason: ReasonNotOnPath, Detail: err.Error()}
	}

	// status_not_found/2 always exits 1 (cli.ex:4548) and, under --json,
	// puts its message in the error field on stdout rather than on
	// stderr. Checked before the general non-zero-exit branch below, so a
	// malformed-flags exit (2, or 1 for an unrelated reason) is never
	// misread as "ref not found".
	if exitCode == 1 {
		var se statusError
		if err := json.Unmarshal(stdout, &se); err == nil && strings.Contains(se.Error, statusNotFoundPrefix) {
			return nil, nil, &NotFound{Ref: ref}
		}
	}
	if exitCode != 0 {
		return nil, nil, &Unavailable{Reason: ReasonNonZeroExit, Detail: fmt.Sprintf("exit %d", exitCode)}
	}
	if !json.Valid(stdout) {
		return nil, nil, &Unavailable{Reason: ReasonMalformedJSON, Detail: "stdout does not parse as JSON"}
	}

	var probe struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(stdout, &probe); err != nil {
		return nil, nil, &Unavailable{Reason: ReasonMalformedJSON, Detail: err.Error()}
	}

	switch probe.Kind {
	case "run":
		var raw rawRunStatus
		if err := json.Unmarshal(stdout, &raw); err != nil {
			return nil, nil, &Unavailable{Reason: ReasonMalformedJSON, Detail: err.Error()}
		}
		if !validRunLifecycle(raw.Status) {
			return nil, nil, &Unavailable{Reason: ReasonMalformedJSON,
				Detail: fmt.Sprintf("status %q is neither converged nor in_progress", raw.Status)}
		}
		return &RunStatus{
			Ref:        raw.Ref,
			Status:     raw.Status,
			Converged:  raw.Converged,
			ReleaseRef: raw.ReleaseRef,
			ObservedAt: raw.ObservedAt,
		}, nil, nil

	case "proposal":
		var raw rawProposalStatus
		if err := json.Unmarshal(stdout, &raw); err != nil {
			return nil, nil, &Unavailable{Reason: ReasonMalformedJSON, Detail: err.Error()}
		}
		return nil, &ProposalStatus{Ref: raw.Ref, Status: raw.Status}, nil

	default:
		return nil, nil, &Unavailable{Reason: ReasonWrongKind, Detail: probe.Kind}
	}
}
