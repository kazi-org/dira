package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kazi-org/dira/internal/cli"
	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/kazi"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// `dira map` is E4's join surface: the ledger's decisions and intents,
// grouped by why (their derives_from parent) rather than by kazi's own
// per-goal grouping, with execution status derived at read time and never
// stored (dec-0004). It is the one command in this binary that shells kazi —
// through internal/kazi's published Snapshot/Status functions, never
// directly — and it degrades honestly when kazi cannot be asked rather than
// guessing (E4-L3/E4-L5).

// mapSummary is the one-line description in `dira help`.
const mapSummary = "join the ledger to kazi's execution status: what is planned, running, blocked, or done"

// kaziCallTimeout bounds every individual call this command makes into
// internal/kazi — the portfolio snapshot and, per E4-L3-T3's bounded
// fan-out, up to MaxStatusCalls `kazi status` calls. kazi calls measure
// ≈0.65s typically (docs/plan/lanes/E4.md point 4); 3s gives a loaded
// machine real headroom (~4.6x) without leaving a human waiting on a hung
// terminal command indefinitely — and without a deadline at all,
// kazi.Snapshot and kazi.Status block on ctx.Background() forever, which is
// what a real hang (a wedged kazi process, not just a slow one) would
// otherwise do to this command with no way to ever report ReasonTimeout.
const kaziCallTimeout = 3 * time.Second

// mapUsagef reports a mistake in this command's own flags with this
// command's help.
func mapUsagef(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...), usage: writeMapUsage}
}

// runMap renders the map.
func runMap(a *app, args []string) error {
	fs := flag.NewFlagSet("map", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	dir := fs.String("C", "", "run as if started in this directory")
	jsonOut := fs.Bool("json", false, "emit the documented JSON shape instead of text")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeMapUsage(a.stdout)
			return nil
		}
		return mapUsagef("%w", err)
	}
	if fs.NArg() > 0 {
		return mapUsagef("map takes no arguments, got %q", fs.Arg(0))
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

	// Every other read command (brief, why) treats an index notice as
	// something to print and carry on past — a partial answer beats none.
	// dira map refuses instead: E4-L4-T2's whole point is that no entry
	// silently vanishes from the tree (count conservation), and an entry
	// the index could not even read would vanish with no trace beyond a
	// line other commands are free to let a reader skim past. One line on
	// stderr, naming the file, and exit 1 — not a panic, not a silent
	// incomplete tree.
	if notice := ix.Notice(); notice != "" {
		return fmt.Errorf("%s", strings.TrimPrefix(notice, "dira: "))
	}

	snapCtx, cancelSnap := context.WithTimeout(ctx, kaziCallTimeout)
	defer cancelSnap()
	snap, snapErr := kazi.Snapshot(snapCtx)
	observedAt := a.now().UTC().Format(time.RFC3339)

	// Each fan-out call to kazi status (E4-L3-T3, bounded at
	// status.MaxStatusCalls) gets its own fresh deadline off the same
	// budget, rather than sharing one deadline across the whole fan-out —
	// a single slow-but-working call must not starve the calls after it.
	statusFn := func(callCtx context.Context, ref string) (*kazi.RunStatus, *kazi.ProposalStatus, error) {
		c, cancel := context.WithTimeout(callCtx, kaziCallTimeout)
		defer cancel()
		return kazi.Status(c, ref)
	}

	return cli.Render(ctx, a.stdout, ix, snap, snapErr, statusFn, *jsonOut, observedAt)
}

// writeMapUsage renders `dira map`'s own help.
func writeMapUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira map - join the ledger to kazi's execution status.\n\n")
	b.WriteString("usage:\n\n\tdira map [flags]\n\n")
	b.WriteString("Groups accepted decisions and active intents by their derives_from\n")
	b.WriteString("parent, one level, with a roll-up per parent. An entry with no parent\n")
	b.WriteString("appears under an explicit unparented group. Status is derived from\n")
	b.WriteString("kazi at read time and never stored (dec-0004): when kazi cannot be\n")
	b.WriteString("asked, the ledger-side buckets still render and one line names why\n")
	b.WriteString("execution status is unavailable.\n\n")

	b.WriteString("flags:\n\n")
	b.WriteString("\t-C <dir>   run as if started in this directory\n")
	b.WriteString("\t--json     emit the documented JSON shape instead of text\n\n")

	b.WriteString("examples:\n\n")
	b.WriteString("\tdira map\n")
	b.WriteString("\tdira map --json\n\n")

	b.WriteString("exit codes:\n\n")
	b.WriteString("\t0  the map, however kazi answered\n")
	b.WriteString("\t1  the ledger could not be read\n")
	b.WriteString("\t2  usage error\n")

	_, _ = io.WriteString(w, b.String())
}
