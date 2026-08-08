package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger/local"
	"github.com/kazi-org/dira/internal/why"
)

// `dira why` is the on-demand half of int-0001. The brief pushes at session
// start; this answers when someone asks, and design.md §10 step 8 sets the bar:
// three weeks later, in a new session, `dira why daemon` returns "the decision,
// the rejected alternative and its why_not, the intent it served, the converged
// goal that realized it, and the ADR path if you want the long form" — in one
// screen.
//
// Four things it deliberately does not print, each for a reason that would
// otherwise have to be re-argued at a later date:
//
//   - **The entry's body.** §10's list does not include it and the ADR path is
//     offered instead ("if you want the long form"). The bodies in this repo's
//     own ledger run forty lines; printing one turns a chain into a document and
//     the one-screen target into a scroll. The alternatives *are* the reasoning
//     dira treats as load-bearing (design.md §4.2).
//   - **Any colour.** Not one ANSI byte is emitted. docs/design/DESIGN.md law 1
//     reserves red for drift, contradiction and `dira check`, and a refusal is a
//     record rather than an alarm — so the one hue this output could plausibly
//     want is the one it may not have.
//   - **The invocation line.** The rendered page in
//     docs/design/screens/s1-decision.html opens with `$ dira why elixir`
//     because a stranger landing from a link has to be told what produced the
//     page. In a terminal the reader typed it and it is on the screen already.
//     The Chain carries the query so E6's renderer can draw it; this one does
//     not repeat it back.
//   - **Anything about whether a `realized_by` goal converged.** dec-0004 makes
//     status derived and never stored, dira embeds no kazi client, and E4 owns
//     the join. The target is printed verbatim as the external URI it is.
//
// It writes nothing. There is no state for a query to update, and a read verb
// that mutates the ledger is how a derived-status product acquires stored status
// by accident.

// whyUsagef is usagef for a mistake inside `dira why`, so the help printed
// alongside the message is this command's rather than the list of commands.
func whyUsagef(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...), usage: writeWhyUsage}
}

// whySummary is the one-line description in `dira help`.
const whySummary = "print the chain behind an entry: what it arises from, and what it refused"

// runWhy renders the why-chain for one entry.
func runWhy(a *app, args []string) error {
	fs := flag.NewFlagSet("why", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	dir := fs.String("C", "", "run as if started in this directory")
	width := fs.Int("width", why.DefaultWidth, "wrap the chain at this column")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeWhyUsage(a.stdout)
			return nil
		}
		return whyUsagef("%w", err)
	}
	switch fs.NArg() {
	case 0:
		return whyUsagef("why needs an entry id or a term, e.g. `dira why dec-0002` or `dira why daemon`")
	case 1:
	default:
		return whyUsagef("why takes one entry id or term, got %d", fs.NArg())
	}
	query := strings.TrimSpace(fs.Arg(0))

	store, diraDir, err := openLedger(*dir)
	if err != nil {
		return err
	}

	ctx := context.Background()
	// index.Open reconciles before it returns, so this reads through a warm
	// cache when there is one and falls back to the files when there is not
	// — and either way the answer is identical, because everything rendered
	// below is fetched back through the store rather than out of the cache.
	ix, err := index.Open(ctx, store, local.CacheDir(diraDir))
	if err != nil {
		return err
	}
	defer func() { _ = ix.Close() }()

	if notice := ix.Notice(); notice != "" {
		// The notice goes to stderr and the exit code stays 0: a
		// read-only checkout is a place dira has to work, not a failure,
		// and stdout must stay the chain and nothing else.
		_, _ = fmt.Fprintln(a.stderr, notice)
	}

	candidates, err := why.Resolve(ctx, ix, query)
	if err != nil {
		return err
	}

	switch len(candidates) {
	case 0:
		// A runtime error rather than a usage error: the command was
		// used correctly and the ledger simply does not hold this. It
		// must not read like a crash, so it says what was looked for,
		// where, and what to try instead.
		return fmt.Errorf("no entry matches %q in %s — try a word from its title, or one of its tags", query, diraDir)
	case 1:
		chain, err := why.Build(ctx, ix, query, candidates[0].Ref)
		if err != nil {
			return err
		}
		return why.RenderText(a.stdout, chain, *width)
	default:
		// Listing beats guessing. Picking one of several silently would
		// produce something indistinguishable from the right answer.
		return why.RenderCandidates(a.stdout, query, candidates, *width)
	}
}

// writeWhyUsage renders `dira why`'s own help.
func writeWhyUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira why - print the chain behind an entry.\n\n")
	b.WriteString("usage:\n\n\tdira why <id|term>\n\n")
	b.WriteString("The chain is what an entry arises from, the alternatives it refused\n")
	b.WriteString("and why, anything that superseded it, and the goal or ADR it points at.\n\n")
	b.WriteString("An id resolves to itself. Any other term matches entry titles and tags,\n")
	b.WriteString("newest first; a term matching several entries lists them rather than\n")
	b.WriteString("choosing one.\n\n")

	b.WriteString("flags:\n\n")
	b.WriteString("\t-C <dir>       run as if started in this directory\n")
	b.WriteString("\t-width <n>     wrap the chain at this column (default 80)\n\n")

	b.WriteString("examples:\n\n")
	b.WriteString("\tdira why dec-0002\n")
	b.WriteString("\tdira why daemon\n\n")

	b.WriteString("exit codes:\n\n")
	b.WriteString("\t0  the chain, or the candidates a term matched\n")
	b.WriteString("\t1  nothing in the ledger matches\n")
	b.WriteString("\t2  usage error\n")

	_, _ = io.WriteString(w, b.String())
}
