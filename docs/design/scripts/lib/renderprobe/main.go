// Command renderprobe renders one of dira's served pages over the design
// fixture ledger and prints the raw HTML to stdout, so a mockup can be brought
// to byte-truth against the real template without hand-transcribing it.
//
// It is a throwaway aid for authoring docs/design/screens/*.html, not a gate
// itself — docs/design/scripts/fixture-check.mjs and uigate.mjs are the gates.
//
// Usage: go run ./docs/design/scripts/lib/renderprobe <path>
// e.g.:  go run ./docs/design/scripts/lib/renderprobe /
//
//	go run ./docs/design/scripts/lib/renderprobe /e/dec-0001
package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"

	"github.com/kazi-org/dira/internal/index"
	"github.com/kazi-org/dira/internal/ledger/local"
	"github.com/kazi-org/dira/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "renderprobe:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: renderprobe <path>")
	}
	path := os.Args[1]

	store, err := local.Open("docs/design/fidelity/fixtures/ledger-design")
	if err != nil {
		return err
	}
	ctx := context.Background()
	cacheDir, err := os.MkdirTemp("", "renderprobe-cache-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(cacheDir) }()

	ix, err := index.Open(ctx, store, cacheDir)
	if err != nil {
		return err
	}
	defer func() { _ = ix.Close() }()

	// "dira" is what local.Name() actually returns for this repository — the
	// basename of the directory holding .dira in a real checkout. It was
	// "kazi-org/dira" until E6-L3-T8 traced 12/18 pixel-diff pairs to this
	// literal: Name() takes the last path segment only, so a two-segment
	// org/repo string is a value it can never produce, and every mockup
	// rendered from it inherited a header no served page could ever match.
	srv, err := ui.NewServer(ix, store, "dira")
	if err != nil {
		return err
	}

	req := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	fmt.Println(rec.Body.String())
	return nil
}
