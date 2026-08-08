package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/signal"
	"strings"
	"syscall"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger/local"
	"github.com/kazi-org/dira/internal/ui"
)

// `dira ui` serves the two read-only surfaces — the ledger index and the
// decision page — from the binary, on loopback, in the foreground.
//
// Three properties are load-bearing and each one is a decision rather than a
// preference:
//
//   - **Server-rendered HTML, no SPA** (dec-0012). The decision pages are
//     dec-0010's acquisition surface, and a page a crawler cannot read is
//     invisible to the one channel it exists to feed. Every page renders
//     complete with JavaScript disabled.
//   - **No daemon and no permanent port** (int-0002). This is a foreground
//     process that binds an ephemeral port by default and releases it on
//     Ctrl-C. Nothing is left running, and nothing needs to be.
//   - **Loopback only** (cst-0004, cst-0003). A ledger reachable from the LAN
//     is a ledger published to a network nobody asked to publish it to. The
//     refusal is in ui.Listen, so it holds for every caller and not only for
//     this one.
//
// It writes nothing. Every route is a GET over the same query path `dira why`
// uses, and the decision page is rendered from the same *why.Chain the terminal
// renderer prints — one producer behind both.

func uiUsagef(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...), usage: writeUIUsage}
}

// uiSummary is the one-line description in `dira help`.
const uiSummary = "serve the ledger index and the decision pages on localhost"

func runUI(a *app, args []string) error {
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	dir := fs.String("C", "", "run as if started in this directory")
	addr := fs.String("addr", "127.0.0.1:0", "loopback address to bind; port 0 picks a free one")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeUIUsage(a.stdout)
			return nil
		}
		return uiUsagef("%w", err)
	}
	if fs.NArg() > 0 {
		return uiUsagef("ui takes no arguments, got %d", fs.NArg())
	}

	store, diraDir, err := openLedger(*dir)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ix, err := index.Open(ctx, store, local.CacheDir(diraDir))
	if err != nil {
		return err
	}
	defer func() { _ = ix.Close() }()

	if notice := ix.Notice(); notice != "" {
		_, _ = fmt.Fprintln(a.stderr, notice)
	}

	server, err := ui.NewServer(ix, local.Name(diraDir))
	if err != nil {
		return err
	}

	ln, err := ui.Listen(*addr)
	if err != nil {
		// A refused bind is the caller asking for something dira will not
		// do, not a failure while doing it — so it is a usage error and
		// exit 2, and the message names the constraint.
		if errors.Is(err, ui.ErrNotLoopback) {
			return uiUsagef("%w", err)
		}
		return err
	}

	_, _ = fmt.Fprintf(a.stdout, "http://%s\n", ln.Addr())
	_, _ = fmt.Fprintln(a.stderr, "serving the ledger read-only; ctrl-c to stop")

	return ui.Serve(ctx, ln, server)
}

// writeUIUsage renders `dira ui`'s own help.
func writeUIUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira ui - serve the ledger index and the decision pages on localhost.\n\n")
	b.WriteString("usage:\n\n\tdira ui [-C dir] [-addr 127.0.0.1:port]\n\n")
	b.WriteString("Two read-only surfaces, server-rendered from the binary:\n\n")
	b.WriteString("\t/          the ledger index, grouped by the intent each entry serves\n")
	b.WriteString("\t/e/<id>    one entry: its chain, its ruling, and the roads it refused\n\n")
	b.WriteString("Both render completely with JavaScript disabled, and no page fetches\n")
	b.WriteString("anything from any host. It binds loopback only: a ledger reachable from\n")
	b.WriteString("the LAN is a ledger published by accident (cst-0004).\n\n")

	b.WriteString("flags:\n\n")
	b.WriteString("\t-C <dir>       run as if started in this directory\n")
	b.WriteString("\t-addr <a:p>    loopback address to bind (default 127.0.0.1:0)\n\n")

	b.WriteString("examples:\n\n")
	b.WriteString("\tdira ui\n")
	b.WriteString("\tdira ui -addr 127.0.0.1:7777\n\n")

	b.WriteString("exit codes:\n\n")
	b.WriteString("\t0  the server stopped on ctrl-c\n")
	b.WriteString("\t1  the ledger could not be read, or the port could not be bound\n")
	b.WriteString("\t2  usage error, including a request to bind a non-loopback address\n")

	_, _ = io.WriteString(w, b.String())
}
