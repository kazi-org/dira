package main

import (
	"fmt"
	"os"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// openLedger resolves the ledger a command should act on and opens it.
//
// dir is the `-C` flag: run as if started somewhere else. Empty means the
// working directory. From there local.Find walks up for a `.dira`, which is what
// makes dira work from a subdirectory of a repository — which is where a hook
// actually runs.
//
// Every command that touches a ledger goes through here so `-C` cannot come to
// mean two slightly different things in two commands. The path handling itself
// stays in the backend (dec-0005): this passes a string down and never builds
// one.
//
// It returns the interface rather than the backend on purpose. This function is
// the single place in the binary that picks an implementation, and E7's github
// backend is meant to be a second case here and nothing else; a command holding
// a *local.Store would be a command E7 has to change.
func openLedger(dir string) (store ledger.Store, diraDir string, err error) {
	start := dir
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("finding the working directory: %w", err)
		}
		start = cwd
	}

	diraDir, err = local.Find(start)
	if err != nil {
		return nil, "", err
	}
	backend, err := local.Open(diraDir)
	if err != nil {
		return nil, "", err
	}
	return backend, diraDir, nil
}
