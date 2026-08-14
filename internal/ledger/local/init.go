package local

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kazi-org/dira/internal/ledger"
)

// initStagingDirName is where InitLedger builds a new ledger before it is
// real. It is a fixed name — not a randomised temp directory — so that a
// process crash or kill between staging and commit leaves a
// recognisable, cleanable leftover rather than an anonymous one, and so a
// caller inspecting a failed init sees why nothing was created.
const initStagingDirName = ".dira.staging"

// InitLedger creates a new ledger at dir/.dira, seeded with drafts, all at
// once or not at all.
//
// # Why all-or-nothing has to be a property of this function, not of its caller
//
// Store.Create writes one file per call. An interview that wrote its intent,
// then its constraint, then failed before its question would leave
// .dira/entries/ holding two files: not empty, but not what dec-0010's
// guarantee promises either. So every draft is validated and staged before
// the first byte lands at dir/.dira: this function builds the whole ledger in
// initStagingDirName beside it, and only renames the staging directory onto
// .dira once every draft has written successfully. A rename is one syscall on
// the same filesystem, which is what makes "all" atomic; nothing else here
// needs to be.
//
// # Two refusals, and neither is a degenerate success
//
// An empty drafts slice creates nothing and returns an error: dec-0010's
// guarantee is exactly what an empty successful init would violate, so this
// is not "nothing to do," it is a caller's mistake. A dir that already has a
// .dira is refused before InitLedger writes anything at all, staging
// directory included — this command seeds a fresh ledger and never merges
// into or clobbers one that already exists.
//
// Each draft must already carry Created: the clock belongs to the caller,
// the same discipline Add already holds ledger writers to.
func InitLedger(dir string, cfg []byte, drafts []*ledger.Entry) (*Store, error) {
	if len(drafts) == 0 {
		return nil, errors.New("local: InitLedger needs at least one draft; " +
			"an empty successful init would violate dec-0010's guarantee that no successful init produces an empty ledger")
	}

	target := filepath.Join(dir, diraDirName)
	switch info, err := os.Stat(target); {
	case err == nil:
		if info.IsDir() {
			return nil, fmt.Errorf("local: %s already exists; InitLedger seeds a fresh ledger and never merges into one", target)
		}
		return nil, fmt.Errorf("local: %s exists and is not a directory", target)
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("local: checking %s: %w", target, err)
	}

	staging := filepath.Join(dir, initStagingDirName)
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()

	if err := os.MkdirAll(filepath.Join(staging, entriesDir), 0o755); err != nil {
		return nil, fmt.Errorf("local: staging the new ledger: %w", err)
	}
	if err := os.WriteFile(filepath.Join(staging, configFile), cfg, 0o644); err != nil {
		return nil, fmt.Errorf("local: staging the new ledger's config: %w", err)
	}

	stage, err := Open(staging)
	if err != nil {
		return nil, fmt.Errorf("local: opening the staged ledger: %w", err)
	}

	ctx := context.Background()
	taken := map[ledger.Kind]int{}
	for _, d := range drafts {
		if d == nil {
			return nil, errors.New("local: InitLedger was given a nil draft")
		}
		if d.Created == "" {
			return nil, fmt.Errorf("local: a %s draft carries no created timestamp; the caller stamps it before InitLedger is called", d.Kind)
		}
		n := taken[d.Kind] + 1
		d.ID = ledger.FormatID(d.Kind, n)
		if err := stage.Create(ctx, d); err != nil {
			d.ID = ""
			return nil, fmt.Errorf("local: writing a %s entry: %w", d.Kind, err)
		}
		taken[d.Kind] = n
	}

	if err := os.Rename(staging, target); err != nil {
		return nil, fmt.Errorf("local: committing the new ledger: %w", err)
	}
	committed = true

	return Open(target)
}
