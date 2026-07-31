package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kazi-org/dira/internal/config"
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

	parents, cfgErr := declaredParents(diraDir)
	if cfgErr != nil {
		// A config dira could not make sense of is a report, not a
		// failure — the same bargain `dira brief` makes with the same
		// reader. It goes to stderr so stdout stays the parseable
		// verdict, and it is safe to print because internal/config puts
		// no part of a declaration's value into a report: a private
		// parent's label reaching stderr would be the leak
		// scripts/privacy-lint.py P2 exists to catch, arriving by
		// another route.
		_, _ = fmt.Fprintf(a.stderr, "%s check: %v\n", a.name, cfgErr)
	}
	inherited, err := enforcer.Inherit(ctx, parents)
	if err != nil {
		return err
	}

	verdict, err := enforcer.CheckInherited(ctx, indexLedger{ix}, plan, inherited)
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

// declaredParents resolves every [parents] declaration in the child's config
// and opens each one for reading.
//
// # This function is where cst-0003 rule 1 is actually kept
//
// ledger.ReadOnly refuses Create, Put and Delete, and internal/enforcer re-wraps
// whatever it is handed in one anyway. Neither of those stops the write that
// really happens: index.Open creates a SQLite file at local.CacheDir(diraDir),
// *beside* the store rather than through it, so opening a parent the way this
// command opens its own ledger writes into that parent. No type can prevent that
// from below the call — which is why the obligation lives here, in the one place
// that decides which read path each ledger gets. `dira check` keeps the index for
// its own ledger and gives a parent the read-only store and nothing else.
//
// The proof standard that goes with it is a sha256 over every file under the
// parent's directory. A git-based check would be vacuously green: .gitignore
// ignores .dira/cache/ precisely because a derived artifact holds a private
// entry's title, so git reports a clean parent while the cache is being written
// into it.
//
// # Relative to the child's .dira, and never walked up from
//
// A declaration's path is relative to the .dira directory of the ledger that
// declared it, so it is joined onto diraDir — which openLedger has already made
// absolute — and not onto the process working directory. A parent declared
// `../../sire-run/sire` means the same ledger whether the human ran dira from
// the repository root, from a subdirectory, or with -C from somewhere else
// entirely.
//
// Nothing here calls local.Find. Find walks *up* until it meets a .dira, so
// pointing it at a path that does not exist silently resolves to whatever ledger
// happens to be above it — the shape of L-0014, arriving through the parent
// path instead of through a fixture. A parent is at <path>/.dira or it is not
// resolved.
//
// That this file joins a path at all is a departure from dec-0005's rule that
// paths live in a storage backend, and it is the one internal/enforcer's
// inherit.go names: "Resolving a declaration to a directory is the caller's job
// … so what arrives here is a store somebody already opened." cmd/dira is the
// binary's composition root — openLedger is already the single place that picks
// an implementation — so the join lands beside it rather than inside a package
// that must stay backend-agnostic. When E7 adds the github backend, this is the
// function that grows a second case, exactly as openLedger does.
//
// # An unresolved parent is a value, not a skip
//
// A declaration that does not resolve still becomes a Parent, backed by a store
// that reports it cannot be read. Inherit records it in Inherited.Parents,
// evaluates zero of its constraints and changes no verdict. Dropping it here
// instead would make a check with a missing parent indistinguishable from one
// with no parents at all, which is a firewall with a hole in it in exactly the
// configuration a public clone has.
//
// The error such a store reports carries no path. dec-0011 keeps a private
// parent's label out of every output, and a filesystem path to that ledger
// identifies it at least as well as its label does; ParentResult.Err is meant to
// be printed by whoever reports it, so it is written here as something that is
// always safe to print.
func declaredParents(diraDir string) ([]enforcer.Parent, error) {
	data, err := local.ReadConfig(diraDir)
	if err != nil {
		return nil, err
	}
	cfg, cfgErr := config.Parse(data)

	parents := make([]enforcer.Parent, 0, len(cfg.ParentDecls))
	for _, decl := range cfg.ParentDecls {
		store, parentDira := openParent(diraDir, decl)
		parents = append(parents, enforcer.Parent{
			Decl: decl,
			// The parent's own config, read for its [ledger].tier and
			// nothing else. Unreadable is not an error: internal/
			// enforcer treats an unknown tier as person, which is the
			// direction that cannot leak.
			Config: parentConfig(parentDira),
			Store:  store,
		})
	}
	return parents, cfgErr
}

// errNoParentLedger is what an unresolved parent's store reports.
//
// One sentence, no path, no label, no namespace: it is written to be printable
// verbatim by any caller without that caller having to decide what may be shown.
var errNoParentLedger = errors.New("no ledger at the declared path")

// openParent opens one declared parent for reading, and returns the store to
// read it through together with the .dira directory it resolved to.
//
// The store is a ledger.ReadOnly whatever happens. That is not redundant with
// Inherit's own wrapping: this function is the boundary the parent's writable
// backend must not cross, and holding *local.Store here — even briefly, even
// only to hand onward — would put the one object that can write to a parent in
// the same scope as the index that would cache it.
func openParent(diraDir string, decl config.Parent) (ledger.Store, string) {
	if decl.Path == "" {
		// A declaration with no path is not locatable, which is exactly
		// the shape a private parent takes in a public clone. It is
		// reported, not guessed at.
		return ledger.ReadOnly(unreadableParent{}), ""
	}

	parentDira := local.ParentDira(diraDir, decl.Path)
	info, err := os.Stat(parentDira)
	if err != nil || !info.IsDir() {
		return ledger.ReadOnly(unreadableParent{}), ""
	}
	backend, err := local.Open(parentDira)
	if err != nil {
		return ledger.ReadOnly(unreadableParent{}), ""
	}
	return ledger.ReadOnly(backend), parentDira
}

// parentConfig reads a parent's .dira/config.toml, or nil where there is none
// to read. The bytes go to internal/enforcer, which reads [ledger].tier out of
// them and falls closed on anything it cannot make sense of.
func parentConfig(parentDira string) []byte {
	if parentDira == "" {
		return nil
	}
	data, err := local.ReadConfig(parentDira)
	if err != nil {
		return nil
	}
	return data
}

// unreadableParent is a ledger that is declared and not there.
//
// It exists so that "this parent could not be read" is a value travelling the
// same path as a parent that could, rather than a branch taken before the
// journey starts. Its writes are refused by the ledger.ReadOnly it is always
// wrapped in; they are implemented here only because a Store has five methods.
type unreadableParent struct{}

func (unreadableParent) Get(context.Context, string) (*ledger.Entry, error) {
	return nil, errNoParentLedger
}

func (unreadableParent) List(context.Context) ([]ledger.EntryInfo, error) {
	return nil, errNoParentLedger
}

func (unreadableParent) Create(context.Context, *ledger.Entry) error { return errNoParentLedger }
func (unreadableParent) Put(context.Context, *ledger.Entry) error    { return errNoParentLedger }
func (unreadableParent) Delete(context.Context, string) error        { return errNoParentLedger }

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
