// Package local implements ledger.Store over the filesystem.
//
// It is the only package in dira that knows what a path is. dec-0005 commits to
// a storage interface with the filesystem as one implementation and the GitHub
// Contents API (E7) as the other, so every path walk, glob, temp file and rename
// has to be confined to a backend or the github backend cannot be added without
// changing the code above it. That confinement is checked mechanically by
// TestNoFilesystemImportsAboveTheBackend in internal/ledger, not by convention.
package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kazi-org/dira/internal/ledger"
)

// entriesDir is the subdirectory of .dira holding one file per entry
// (dec-0002). Its siblings — cache/, config.toml — belong to other lanes.
const entriesDir = "entries"

// A Store reads and writes a ledger as a directory of markdown files.
//
// It holds no open file handles, no lock and no cached state, so it costs
// nothing to construct and nothing to keep: int-0002 forbids a daemon, and a
// backend that had to be warmed would be one.
//
// A Store is safe for concurrent use. Each operation touches a single file, and
// writes land by rename, so a concurrent reader sees either the old file or the
// new one and never a half-written one.
type Store struct {
	// dir is the .dira directory, not the entries directory. Keeping the
	// parent is what lets a later lane put .dira/cache beside the entries
	// without re-deriving it from a child path.
	dir string
}

// Open returns a Store over the ledger in diraDir, which must exist and be a
// directory — typically the `.dira` at the root of a repository.
//
// The entries directory inside it does not have to exist yet: a ledger with no
// entries reads as empty, and the first write creates it. Open deliberately
// creates nothing, so pointing dira at the wrong directory reports an error
// instead of quietly seeding a second ledger there.
func Open(diraDir string) (*Store, error) {
	info, err := os.Stat(diraDir)
	if err != nil {
		return nil, fmt.Errorf("opening ledger at %s: %w", diraDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("opening ledger at %s: not a directory", diraDir)
	}
	return &Store{dir: diraDir}, nil
}

// Get returns the entry with the given id.
func (s *Store) Get(ctx context.Context, id string) (*ledger.Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}

	// One open, one fstat, one read. os.ReadFile followed by os.Stat would
	// be a second path resolution and a second syscall per entry, which over
	// a 200-entry ledger is a measurable slice of int-0002's budget spent
	// asking the kernel something the open file already knows.
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s: %w", id, ledger.ErrNotFound)
		}
		return nil, fmt.Errorf("reading %s: %w", id, err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stating %s: %w", id, err)
	}
	data := make([]byte, 0, info.Size()+1)
	buf := bytes.NewBuffer(data)
	if _, err := buf.ReadFrom(f); err != nil {
		return nil, fmt.Errorf("reading %s: %w", id, err)
	}

	entry, err := ledger.DecodeStored(buf.Bytes(), version(info))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", id, err)
	}
	if entry.ID != id {
		return nil, fmt.Errorf("%s holds an entry with id %q; the file name and the id must agree", id+".md", entry.ID)
	}
	return entry, nil
}

// List returns every entry in the ledger, id and version only, sorted by id.
//
// It reads the directory and stats each file rather than opening any of them,
// which is what keeps a reindex over 200 entries affordable inside int-0002's
// budget. Files that are not named like an entry are skipped in silence: a
// ledger is a directory in a repository people also keep notes in, and a stray
// README.md is not ledger rot.
func (s *Store) List(ctx context.Context) ([]ledger.EntryInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	dir := filepath.Join(s.dir, entriesDir)
	names, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// A ledger with no entries directory is empty, not broken.
			return []ledger.EntryInfo{}, nil
		}
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}

	out := make([]ledger.EntryInfo, 0, len(names))
	for _, name := range names {
		if name.IsDir() {
			continue
		}
		id, ok := strings.CutSuffix(name.Name(), ".md")
		if !ok || !ledger.ValidID(id) {
			continue
		}
		info, err := name.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// Removed between the read and the stat.
				continue
			}
			return nil, fmt.Errorf("stating %s: %w", name.Name(), err)
		}
		out = append(out, ledger.EntryInfo{ID: id, Version: version(info)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Create writes an entry that must not already exist.
//
// Exclusivity comes from os.Link, which fails rather than replaces when the
// destination is taken — the filesystem's own compare-and-swap, and the local
// counterpart of a sha-less PUT to the GitHub Contents API. Writing the content
// to a temporary file first means a losing racer never leaves a partial entry
// behind, only a temporary file it then removes.
func (s *Store) Create(ctx context.Context, e *ledger.Entry) error {
	return s.write(ctx, e, true)
}

// Put writes an entry, replacing any existing one with the same id.
func (s *Store) Put(ctx context.Context, e *ledger.Entry) error {
	return s.write(ctx, e, false)
}

func (s *Store) write(ctx context.Context, e *ledger.Entry, exclusive bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if e == nil {
		return errors.New("nil entry")
	}
	path, err := s.path(e.ID)
	if err != nil {
		return err
	}
	// Encode validates, so a file violating entry.schema.json cannot reach
	// the disk through this backend.
	data, err := ledger.Encode(e)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".dira-*.tmp")
	if err != nil {
		return fmt.Errorf("writing %s: %w", e.ID, err)
	}
	tmpName := tmp.Name()
	// Any path out of here that has not renamed the file must remove it.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing %s: %w", e.ID, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", e.ID, err)
	}
	// CreateTemp is 0600; entries are committed to a repository and read by
	// everything from an editor to a hook, so they carry the repository's
	// ordinary file mode.
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", e.ID, err)
	}

	if exclusive {
		if err := os.Link(tmpName, path); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return fmt.Errorf("%s: %w", e.ID, ledger.ErrExists)
			}
			return fmt.Errorf("creating %s: %w", e.ID, err)
		}
		return nil
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("writing %s: %w", e.ID, err)
	}
	return nil
}

// Delete removes an entry.
func (s *Store) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s: %w", id, ledger.ErrNotFound)
		}
		return fmt.Errorf("deleting %s: %w", id, err)
	}
	return nil
}

// path maps an id to its file.
//
// The id is validated before it reaches the filesystem, which is what stops
// `..`, an absolute path or a separator from being read as one. Entry ids match
// ^(int|dec|qst|cst|note)-[0-9]{4,}$, so no valid id contains a path element at
// all and rejecting the rest here is not a heuristic.
func (s *Store) path(id string) (string, error) {
	if !ledger.ValidID(id) {
		return "", fmt.Errorf("%q is not an entry id", id)
	}
	return filepath.Join(s.dir, entriesDir, id+".md"), nil
}

// version derives EntryInfo.Version from a file's metadata.
//
// Modification time and size, not a content hash: a reindex over 200 entries
// must cost one directory read, and hashing would cost 200 file reads — the
// whole of int-0002's budget, spent to detect that nothing changed. The value is
// opaque above the interface, so replacing it with a hash later is a change to
// this function alone.
func version(info fs.FileInfo) string {
	return fmt.Sprintf("%d-%d", info.ModTime().UnixNano(), info.Size())
}
