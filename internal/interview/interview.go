// Package interview is `dira init --interview`'s fixed script and the
// builder that turns scripted answers into unwritten entry drafts.
//
// # The script is fixed, not generated
//
// dec-0003: dira embeds no model client. "The interview is conducted by the
// already-running session, not by dira" — dira's whole role is to ask a
// short, ordered list of prompts on stdout and read one answer per prompt
// from stdin, and to validate what comes back. Nothing here generates a
// question or interprets an answer semantically; a scripted answer fixture
// (piped stdin, no live model) is not a shortcut standing in for the real
// thing, because the real interview loop has no code path that reads a
// model at all.
//
// # This package does no I/O
//
// Prompting and reading are cmd/dira's job (the one command allowed `os` in
// the CLI package, per internal/ledger/boundary_test.go's allowlist). Build
// takes the answers already collected and returns drafts or a reason it
// could not; it never touches stdin, stdout or a file.
package interview

import (
	"fmt"
	"strings"

	"github.com/kazi-org/dira/internal/ledger"
)

// TierPerson and TierWorkspace are the two tiers this command seeds. A
// repo-tier `.dira` is not seeded by this command (docs/plan/lanes/E5.md);
// `dira log --stdin` already seeds a repo ledger's first entry.
const (
	TierPerson    = "person"
	TierWorkspace = "workspace"
)

// Prompts is the fixed, ordered script: one line asking which tier is being
// seeded, then one prompt per kind dec-0010's guarantee needs — at least one
// intent, one constraint, one question. The order is the order Build expects
// answers in.
var Prompts = []string{
	"Which ledger are you seeding — person or workspace?",
	"One thing you are actively trying to make true (an intent):",
	"One rule that must hold no matter what, whatever else changes (a constraint):",
	"One question you do not have an answer to yet:",
}

// entryScript is Prompts[1:], the prompts that each produce one draft.
var entryScript = []struct {
	kind  ledger.Kind
	state ledger.State
}{
	{ledger.KindIntent, ledger.StateActive},
	{ledger.KindConstraint, ledger.StateActive},
	{ledger.KindQuestion, ledger.StateOpen},
}

// Build turns a complete scripted answer set — one line per Prompts entry, in
// order — into the tier that was named and the drafts to write.
//
// It is deterministic and does no I/O: calling it twice with the same
// answers produces two draft sets with identical titles, bodies, kinds and
// states. created is stamped later, by the writer (internal/ledger/local's
// InitLedger), not here — this function has no clock to read.
//
// An incomplete answer set (fewer lines than Prompts, simulating stdin
// closing early) or a required answer that is blank after trimming returns a
// named error and a nil slice — never a partial one a caller might forget to
// check the error before using.
func Build(answers []string) (tier string, drafts []*ledger.Entry, err error) {
	if len(answers) < len(Prompts) {
		return "", nil, fmt.Errorf("interview: %d of %d prompts were answered before input ended", len(answers), len(Prompts))
	}

	tier = strings.TrimSpace(answers[0])
	switch tier {
	case TierPerson, TierWorkspace:
	default:
		return "", nil, fmt.Errorf("interview: %q answers %q, and dira only seeds %q or %q here",
			Prompts[0], answers[0], TierPerson, TierWorkspace)
	}

	drafts = make([]*ledger.Entry, 0, len(entryScript))
	for i, spec := range entryScript {
		prompt := Prompts[i+1]
		title := strings.TrimSpace(answers[i+1])
		if title == "" {
			return "", nil, fmt.Errorf("interview: %q was answered with nothing", prompt)
		}

		e := &ledger.Entry{
			Kind:  spec.kind,
			Title: title,
			State: spec.state,
			Source: &ledger.Source{
				Hook: ledger.HookManual,
				Tier: ledger.TierHuman,
			},
			ConfirmedBy: "human",
		}
		if err := e.ValidateDraft(); err != nil {
			return "", nil, fmt.Errorf("interview: %q's answer does not make a valid entry: %w", prompt, err)
		}
		drafts = append(drafts, e)
	}
	return tier, drafts, nil
}
