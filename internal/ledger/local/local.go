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
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kazi-org/dira/internal/ledger"
)

// diraDirName is the ledger directory's name, the thing Find walks up looking
// for.
const diraDirName = ".dira"

// entriesDir is the subdirectory of .dira holding one file per entry
// (dec-0002).
const entriesDir = "entries"

// cacheDirName is the subdirectory holding the derived read cache. It is named
// here rather than in the cache package because it is a path, and a path is the
// backend's business (dec-0005). Nothing in it is authoritative: it is
// gitignored and rebuildable from entries/ at any time, and if it and the files
// disagree the files win (dec-0002).
const cacheDirName = "cache"

// configFile is the ledger's only hand-edited file. Everything under entries/ is
// written by dira and everything under cache/ is derived, which is why this one
// is read leniently and never rewritten (see internal/config).
const configFile = "config.toml"

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
	data, err := read(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s: %w", id, ledger.ErrNotFound)
		}
		return nil, fmt.Errorf("reading %s: %w", id, err)
	}

	entry, err := ledger.DecodeStored(data, version(data))
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
// It reads each entry file but decodes none of them: the version is the file's
// git blob id (see version), and hashing 200 files costs 6.9ms against 22ms to
// also parse them. That ratio is the derived cache's entire reason to exist —
// the cache saves the parse, and List is what proves the cache still matches the
// files (dec-0002, dec-0015).
//
// Files that are not named like an entry are skipped in silence: a ledger is a
// directory in a repository people also keep notes in, and a stray README.md is
// not ledger rot.
func (s *Store) List(ctx context.Context) ([]ledger.EntryInfo, error) {
	return s.list(ctx, true)
}

// ListIDs returns every entry in the ledger, sorted by id, with every
// EntryInfo.Version left EMPTY — it reads the directory and no entry file at
// all.
//
// It exists for one caller and one situation: a cache with no rows in it, which
// index.sync reaches on a cold build and on `dira reindex`. There, every entry
// is going to be read and parsed whatever its hash says, and the row written for
// it takes its version from the bytes that parse produced — so the version pass
// List performs decides nothing and costs a second open, read and SHA-1 of every
// file in the ledger. E1-L6-T5 measured that pass at roughly a sixth of the
// in-process work of a cold `dira brief`.
//
// It is deliberately NOT an optimisation of List. List's hash is the whole
// staleness check on a warm cache (dec-0015): it is what proves a cached row
// still matches the file, and skipping it there would be exactly the "fast path
// that bypasses the check" internal/index's package comment says does not exist.
// This method is only sound where there is nothing to compare against.
//
// The empty Version is a fact callers must read, not an oversight, and it fails
// in the safe direction: a row stored with an empty version can never equal a
// real file hash, so the worst a caller that misuses this can cause is a re-read
// on the next run. It can never make a stale row look current.
func (s *Store) ListIDs(ctx context.Context) ([]ledger.EntryInfo, error) {
	return s.list(ctx, false)
}

// list is List and ListIDs, which differ only in whether each file is opened to
// hash it. One implementation so the two cannot disagree about which files are
// entries — a listing that included a file the other skipped would make the
// cold and warm paths index different ledgers.
func (s *Store) list(ctx context.Context, versions bool) ([]ledger.EntryInfo, error) {
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
		if !versions {
			out = append(out, ledger.EntryInfo{ID: id})
			continue
		}
		data, err := read(filepath.Join(dir, name.Name()))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// Removed between the directory read and this one.
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", name.Name(), err)
		}
		out = append(out, ledger.EntryInfo{ID: id, Version: version(data)})
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

// read returns a file's whole contents.
//
// One open, one fstat, one read. os.ReadFile followed by os.Stat would be a
// second path resolution and a second syscall per entry, which over a 200-entry
// ledger is a measurable slice of int-0002's budget spent asking the kernel
// something the open file already knows.
func read(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(make([]byte, 0, info.Size()+1))
	if _, err := buf.ReadFrom(f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

// version derives EntryInfo.Version from an entry file's bytes: the git blob
// object id, sha1 over "blob <len>\x00" followed by the content, which is
// exactly what `git hash-object` prints for the same file.
//
// This replaces the modification-time-and-size heuristic this backend shipped
// with, and the reasoning is recorded as dec-0015. In short: mtime+size is a
// heuristic with a reachable hole — `state: accepted` and `state: rejected` are
// the same length, so a restore that preserves modification times (rsync -a,
// cp -p, tar -p) can flip a decision invisibly — and dec-0002 promises that the
// files win, not that they usually win. Measured over the 200-entry fixture the
// hash costs 6.9ms against 2.2ms for the metadata form; 4.7ms is a cheap price
// for turning a near-certainty into a guarantee, and it is a sixth of what the
// interface's author estimated.
//
// The git blob format rather than a bare hash is not decoration. E7's github
// backend gets blob shas free from the Contents API, so the two backends produce
// *equal* versions for equal content and a ledger can change backend without a
// reindex — and a human debugging a cache can reproduce the value with
// `git hash-object .dira/entries/dec-0002.md` and no dira at all.
//
// sha1's collision weakness is not in this threat model: this detects accidental
// divergence, not forgery, and anyone able to write the entry file can simply
// write whatever they want into it. It is the same tradeoff git makes for the
// same job.
func version(data []byte) string {
	h := sha1.New()
	// hash.Hash never returns an error, which is why these are discarded.
	_, _ = fmt.Fprintf(h, "blob %d\x00", len(data))
	_, _ = h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// Find returns the .dira directory governing start, walking up from it until one
// is found or the filesystem root is reached.
//
// Walking up is what makes dira usable from a subdirectory of a repository,
// which is where a hook actually runs. It stops at the first .dira rather than
// the outermost: a nested ledger is the more specific answer, and dec-0006's
// tier design makes an outer ledger a parent to inherit from rather than a
// ledger to write into.
func Find(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", start, err)
	}
	for {
		candidate := filepath.Join(dir, diraDirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s directory in %s or any parent", diraDirName, start)
		}
		dir = parent
	}
}

// CacheDir returns the derived cache's directory inside a .dira directory.
//
// It is a function on the backend rather than a constant in the cache package
// because it is a path, and dec-0005 confines paths to a backend. The cache
// package receives the string and never constructs one.
func CacheDir(diraDir string) string { return filepath.Join(diraDir, cacheDirName) }

// Name is what this ledger is called on a rendered surface: the directory
// holding `.dira`, which is the repository name in every real checkout.
//
// It lives here for the same reason CacheDir does. dec-0005 says only a storage
// backend may name a path, and taking the last segment of a path is naming one —
// so a caller that wanted a display name and reached for path/filepath itself
// would be putting a filesystem concept above ledger.Store, which is exactly the
// retrofit E7's github backend would then have to undo. That backend will answer
// this question from the repository it talks to, not from a directory.
//
// A ledger with nothing above it to name yields a neutral label rather than an
// empty crumb.
func Name(diraDir string) string {
	name := filepath.Base(filepath.Dir(diraDir))
	switch name {
	case "", ".", string(filepath.Separator):
		return "this ledger"
	}
	return name
}

// ReadConfig returns the bytes of .dira/config.toml, and nil for a ledger that
// has none.
//
// A ledger with no config file is a working ledger: every value in it has a
// default, and `dira init` — which would write one — has no owner in this
// release (docs/plan/lanes/E1.md, "deliberately not in these lanes"). So absence
// is not an error here. A file that exists and cannot be read *is* one, because
// that is a permissions or a hardware problem rather than a shape dira supports,
// and the caller decides whether to degrade or to stop.
//
// It lives on the backend for the same reason CacheDir does: it is a path, and
// dec-0005 confines paths to a backend. internal/config takes the bytes.
func ReadConfig(diraDir string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(diraDir, configFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", configFile, err)
	}
	return data, nil
}

// ParentDira resolves a `[parents]` declaration's path — which is relative to
// the child's own .dira directory — to the parent's .dira directory.
//
// This lives here for the same reason Name does: dec-0005 says only a storage
// backend may name a path, and cmd/dira joining one directly is a change E7's
// GitHub backend would have to make ABOVE the interface. internal/enforcer's
// Inherit deliberately takes a Store rather than a path and says resolving the
// declaration is the caller's job; this is where that job belongs, in the
// backend for which "a directory" is a meaningful answer at all.
//
// A backend with no filesystem answers this differently or not at all, which is
// exactly the point of routing it through the interface.
func ParentDira(diraDir, declared string) string {
	return filepath.Join(diraDir, declared, ".dira")
}
