package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/kazi-org/dira/internal/brief"
	chainpkg "github.com/kazi-org/dira/internal/chain"
	"github.com/kazi-org/dira/internal/config"
	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// `dira brief` is inversion 2 — review is push, not pull (int-0001).
//
// It is the command nobody types. hooks/settings.example.json runs
// `dira brief --context --chain 2>/dev/null || true` on SessionStart with a 5s
// timeout, which sets three requirements this file exists to meet:
//
//   - **It fails open.** stderr goes to /dev/null and a non-zero exit is
//     swallowed, so anything this command turns into an error is orientation the
//     session silently does not get. A malformed entry, an unwritable cache, a
//     config file with a typo in it: all of these degrade to a brief that says
//     so, in the brief, where the reader will actually see it.
//   - **It obeys a ceiling it did not choose.** cst-0001 is constitutional and
//     enforced in the binary; `brief.max_tokens` may lower it, and there is
//     deliberately no flag that raises it — raising it requires superseding the
//     constraint in writing, and a flag would be a hole in a rule the same
//     binary enforces two lines later.
//   - **It is fast enough to run before a human's first prompt** (int-0002).
//     The cache decides which entries the answer is about and only the entries
//     that will be rendered are read from disk, so a 200-entry ledger costs a
//     few dozen file reads rather than 200.
//
// Two forms, one selection: `dira brief` for a human, `dira brief --context` for
// the agent. Both obey the ceiling, and neither says anything about execution
// status — dira does not ask kazi whether a goal converged and must not imply
// that it did (dec-0004).

// briefSummary is the one-line description in `dira help`.
const briefSummary = "print the session brief: what is blocked, the current focus, what was decided"

// briefUsagef reports a mistake in this command's own flags with this command's
// help.
func briefUsagef(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...), usage: writeBriefUsage}
}

// runBrief renders the brief.
func runBrief(a *app, args []string) error {
	fs := flag.NewFlagSet("brief", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	dir := fs.String("C", "", "run as if started in this directory")
	width := fs.Int("width", 0, "wrap at this column (default 80)")
	asContext := fs.Bool("context", false, "render the agent-facing form injected at session start")
	chain := fs.Bool("chain", false, "include the tier chain — the parent ledgers this one derives from")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeBriefUsage(a.stdout)
			return nil
		}
		return briefUsagef("%w", err)
	}
	if fs.NArg() > 0 {
		return briefUsagef("brief takes no arguments, got %d — did you mean `dira why %s`?", fs.NArg(), fs.Arg(0))
	}

	store, diraDir, err := openLedger(*dir)
	if err != nil {
		return err
	}

	opts := brief.Options{
		Width:   *width,
		Now:     a.now(),
		Context: *asContext,
		Chain:   *chain,
		// A real ancestor source, walked lazily: brief.fillChain only
		// calls this when --chain was actually asked for, so a plain
		// `dira brief` never opens a parent ledger it will not render.
		ChainSource: func(ctx context.Context) ([]chainpkg.Ancestor, error) {
			return chainpkg.Walk(ctx, diraDir)
		},
	}

	// The config is read before the index, because a ledger whose config
	// cannot be read still has a ceiling — cst-0001's — and the brief has to
	// say so rather than quietly rendering under a default nobody chose.
	cfg, cfgErr := readBriefConfig(diraDir)
	opts.Ledger = cfg.Name
	opts.MaxTokens = cfg.MaxTokens
	opts.Parents = cfg.Parents
	if cfgErr != nil {
		opts.Notices = append(opts.Notices, fmt.Sprintf("%v — the brief is capped at dira's default of %d tokens instead (cst-0001).",
			cfgErr, brief.DefaultMaxTokens))
	}

	ctx := context.Background()
	ix, err := index.Open(ctx, store, local.CacheDir(diraDir))
	if err != nil {
		return err
	}
	defer func() { _ = ix.Close() }()

	// The index's notice goes into the brief as well as to stderr. Every
	// other command can leave it on stderr, because a human ran them and is
	// looking at the terminal; this one is run by a hook that discards
	// stderr, and "these entries are missing from what you are reading" has
	// to reach the reader of the thing they are missing from.
	if notice := ix.Notice(); notice != "" {
		_, _ = fmt.Fprintln(a.stderr, notice)
		for _, line := range strings.Split(notice, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				opts.Notices = append(opts.Notices, line)
			}
		}
	}

	result, err := brief.Render(ctx, a.stdout, ix, opts)
	if err != nil {
		return err
	}

	// The measurement, on stderr, where it cannot pollute what a hook
	// injects. It is here so that "1,500 tokens by the binary's own counter"
	// is observable by whoever is holding the binary rather than only by a
	// test.
	if result.Omitted() > 0 {
		_, _ = fmt.Fprintf(a.stderr, "dira: %d tokens of %d; %d entries omitted\n",
			result.Tokens, result.MaxTokens, result.Omitted())
	}
	return nil
}

// readBriefConfig loads .dira/config.toml, degrading to defaults with a stated
// reason rather than failing.
//
// Both failure modes end the same way and for the same reason: a session start
// with no orientation is worse than one oriented under the default ceiling, and
// the default ceiling is the constitutional one.
func readBriefConfig(diraDir string) (config.Config, error) {
	data, err := local.ReadConfig(diraDir)
	if err != nil {
		return config.Config{}, err
	}
	return config.Parse(data)
}

// writeBriefUsage renders `dira brief`'s own help.
func writeBriefUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira brief - the session brief: what is blocked, what is being worked\n")
	b.WriteString("towards, and what was decided recently.\n\n")
	b.WriteString("usage:\n\n\tdira brief [flags]\n\n")
	b.WriteString("Ordered by what matters most: open blockers (open questions holding\n")
	b.WriteString("something up), then current focus (active intents), then recent\n")
	b.WriteString("decisions, then notes from the last week.\n\n")
	b.WriteString("The output is capped at brief.max_tokens in .dira/config.toml, 1500 by\n")
	b.WriteString("default (cst-0001). Over the cap, whole entries are dropped from the\n")
	b.WriteString("bottom of that order — never cut mid-entry — and the brief names what\n")
	b.WriteString("it left out. There is no flag to raise the cap: raising it means\n")
	b.WriteString("superseding cst-0001, in writing.\n\n")

	b.WriteString("flags:\n\n")
	b.WriteString("\t-C <dir>     run as if started in this directory\n")
	b.WriteString("\t-context     the agent-facing form, injected by the SessionStart hook\n")
	b.WriteString("\t-chain       include the parent ledgers this one derives from\n")
	b.WriteString("\t-width <n>   wrap at this column (default 80)\n\n")

	b.WriteString("examples:\n\n")
	b.WriteString("\tdira brief\n")
	b.WriteString("\tdira brief --context --chain\n\n")

	b.WriteString("exit codes:\n\n")
	b.WriteString("\t0  the brief, however much of the ledger fitted in it\n")
	b.WriteString("\t1  the ledger could not be read at all\n")
	b.WriteString("\t2  usage error\n")

	_, _ = io.WriteString(w, b.String())
}
