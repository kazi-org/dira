package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// runReindex rebuilds the derived read cache from the entry files alone.
//
// The verb exists because dec-0002 promises it: SQLite is in the design "strictly
// as a derived read cache under .dira/cache/ — gitignored, rebuildable from the
// files at any time by dira reindex." Every other command already reconciles the
// cache before it reads it, so reindex is not how the cache stays correct. It is
// how a user throws it away: the existing database is discarded before anything
// is read, so a cache that has gone wrong in a way a version comparison cannot
// see — because something other than dira wrote it — is fixed by a command
// rather than by knowing which directory to delete.
//
// It reports what it indexed, because "rebuilt the cache" with no number is
// indistinguishable from having done nothing.
func runReindex(a *app, args []string) error {
	fs := flag.NewFlagSet("reindex", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	dir := fs.String("C", "", "run as if started in this directory")

	if err := fs.Parse(args); err != nil {
		return usagef("%w", err)
	}
	if fs.NArg() > 0 {
		return usagef("reindex takes no arguments, got %q", fs.Arg(0))
	}

	start := *dir
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("finding the working directory: %w", err)
		}
		start = cwd
	}

	diraDir, err := local.Find(start)
	if err != nil {
		return err
	}
	store, err := local.Open(diraDir)
	if err != nil {
		return err
	}

	ctx := context.Background()
	ix, err := index.OpenFresh(ctx, store, local.CacheDir(diraDir))
	if err != nil {
		return err
	}
	defer func() { _ = ix.Close() }()
	stats := ix.Stats()

	if notice := ix.Notice(); notice != "" {
		// An unusable cache directory is not a failure of this command,
		// and exiting non-zero here would be worse than useless: E2
		// installs dira in hooks, and a hook that fails on a read-only
		// checkout takes the session with it. So reindex says plainly
		// that it read the ledger and could not store the result, and
		// exits 0 — the same contract the read path has.
		_, _ = fmt.Fprintln(a.stderr, notice)
	}

	_, _ = fmt.Fprintf(a.stdout, "%s\n", summarise(diraDir, stats))
	return nil
}

// summarise is reindex's one line of output.
func summarise(diraDir string, stats index.Stats) string {
	var b strings.Builder
	fmt.Fprintf(&b, "indexed %d entries and %d edges from %s", stats.Entries, stats.Edges, diraDir)
	if stats.Cached {
		fmt.Fprintf(&b, " into %s", local.CacheDir(diraDir))
	} else {
		b.WriteString(" in memory; no cache was written")
	}
	if len(stats.Invalid) > 0 {
		fmt.Fprintf(&b, "\nskipped %d unreadable entry file(s): %s", len(stats.Invalid), strings.Join(stats.Invalid, ", "))
	}
	return b.String()
}
