package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/kazi-org/dira/internal/enforcer"
	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// runCheck is the relitigation firewall at the command line.
//
// It runs *before* predicates are drafted — from a pre-commit hook, a CI step,
// or a human at a terminal — and its exit code is the product. Everything about
// how it decides lives in internal/enforcer; what lives here is the contract a
// caller sees:
//
//	0  the plan contradicts nothing this ledger enforces
//	2  at least one cited conflict
//	1  dira's own failure: an unreadable ledger, a bad flag, no plan given
//
// Telling 1 from 2 is what makes the check safe to install. A hook that cannot
// distinguish "you are contradicting yourself" from "dira is broken" has to
// choose between ignoring real contradictions and taking the session down when
// a checkout has no ledger, and both of those end with the check being switched
// off (int-0001).
//
// Note what that costs against the rest of this binary: dira's other commands
// map a *usage* mistake onto exit 2, and this command cannot, because 2 is
// already a verdict here. So a mistake in this command's own flags is reported
// by this command, with its own usage, at exit 1 — see checkUsage below.
func runCheck(a *app, args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	dir := fs.String("C", "", "run as if started in this directory")
	asJSON := fs.Bool("json", false, "write the verdict as JSON (schema/check.schema.json)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeCheckUsage(a.stdout)
			return nil
		}
		return a.checkMisuse("%s", err)
	}
	switch fs.NArg() {
	case 1:
	case 0:
		return a.checkMisuse("check needs the plan to check, as one quoted argument")
	default:
		return a.checkMisuse("check takes one argument, got %d — quote the whole plan as a single string", fs.NArg())
	}
	plan := fs.Arg(0)
	if strings.TrimSpace(plan) == "" {
		return a.checkMisuse("the plan to check is empty")
	}

	store, diraDir, err := openLedger(*dir)
	if err != nil {
		return err
	}

	ctx := context.Background()
	ix, err := index.Open(ctx, store, local.CacheDir(diraDir))
	if err != nil {
		return err
	}
	defer func() { _ = ix.Close() }()

	if notice := ix.Notice(); notice != "" {
		// An unusable cache directory is a slower check, not a broken one,
		// and the verdict is identical either way. Saying so on stderr
		// keeps stdout parseable and keeps a read-only checkout from
		// producing a failure a hook would have to fail open on.
		_, _ = fmt.Fprintln(a.stderr, notice)
	}

	verdict, err := enforcer.Check(ctx, indexLedger{ix}, plan)
	if err != nil {
		return err
	}

	if *asJSON {
		err = enforcer.RenderJSON(a.stdout, verdict)
	} else {
		err = enforcer.Render(a.stdout, verdict)
	}
	if err != nil {
		return fmt.Errorf("writing the verdict: %w", err)
	}

	if verdict.Compliant() {
		return nil
	}
	// The verdict is already on stdout. This carries nothing but the exit
	// code, so a conflict prints exactly the block README.md documents and
	// not a word more.
	return &codedError{code: verdict.ExitCode()}
}

// indexLedger wires the enforcer to dira's read path.
//
// The adapter lives here rather than in internal/enforcer because that package
// asks for one method and this is the package that knows which reader answers
// it. It is the same two calls every other read surface makes — Select decides
// which entries an answer is about, Entries reads each one from its file
// (dec-0002: the cache never says what an entry contains) — so `dira check` and
// `dira why` share one query path rather than growing a second.
//
// Every entry is selected, not just the enforceable ones. The matcher measures
// how distinctive a word is across the ledger under test (dec-0014), and a word
// that appears in every intent and note here is not distinctive merely because
// no decision happens to use it.
type indexLedger struct{ ix *index.Index }

func (l indexLedger) Entries(ctx context.Context) ([]*ledger.Entry, error) {
	refs, err := l.ix.Select(ctx, index.Selector{})
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(refs))
	for i, ref := range refs {
		ids[i] = ref.ID
	}
	return l.ix.Entries(ctx, ids)
}

// A codedError names the process exit code for an outcome the command has
// already reported itself. It prints nothing further.
//
// It exists because a cited conflict is not an error — it is this command's
// successful answer, which happens to exit non-zero — and because a mistake in
// `dira check`'s own flags must not land on the code that means "you
// contradicted yourself".
type codedError struct {
	code int
	err  error
}

func (e *codedError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *codedError) Unwrap() error { return e.err }

// ExitCode is the interface cmd/dira's error mapping looks for.
func (e *codedError) ExitCode() int { return e.code }

// checkMisuse reports a caller mistake in this command's own arguments, with
// this command's usage, and selects exit 1.
//
// Not exit 2: for `dira check` that code is a verdict about the plan, and a
// hook must never read a typo in a flag as a contradiction. The message is
// written here rather than raised as a usageError because a usageError is
// exactly the thing that would select 2.
func (a *app) checkMisuse(format string, args ...any) error {
	_, _ = fmt.Fprintf(a.stderr, "%s check: %v\n\n", a.name, fmt.Errorf(format, args...))
	writeCheckUsage(a.stderr)
	return &codedError{code: enforcer.ExitDiraError}
}

// writeCheckUsage renders `dira check`'s own flag surface. `dira help check`
// and `dira check -h` show this same text.
func writeCheckUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira check - refuse a plan that contradicts a settled decision\n\n")
	b.WriteString("usage:\n\n\tdira check [flags] \"<plan or idea>\"\n\n")
	b.WriteString("Matches the plan against this ledger's accepted decisions' rejected\n")
	b.WriteString("alternatives, its rejected decisions, and its active constraints. The\n")
	b.WriteString("matching is lexical and runs entirely in this binary: no model, no\n")
	b.WriteString("network, and no agent is involved in reaching the exit code.\n\n")

	b.WriteString("flags:\n\n")
	b.WriteString("\t-C <dir>   run as if started in this directory\n")
	b.WriteString("\t-json      write the verdict as JSON (schema/check.schema.json)\n\n")

	b.WriteString("exit codes:\n\n")
	b.WriteString("\t0  the plan contradicts nothing this ledger enforces\n")
	b.WriteString("\t2  at least one cited conflict\n")
	b.WriteString("\t1  dira's own error — an unreadable ledger, a bad flag\n\n")
	b.WriteString("A caller must never treat 1 as a verdict: it means the check did not run.\n")

	_, _ = io.WriteString(w, b.String())
}
