// Package index is dira's derived read cache: a SQLite database under
// .dira/cache/ that holds an index of the ledger and nothing else.
//
// # The cache is never authoritative
//
// dec-0002 puts SQLite in the design "strictly as a derived read cache …
// rebuildable from the files at any time by dira reindex. If the cache and the
// files ever disagree, the files win." That is not a preference about
// precedence, it is the property the product cannot survive losing: a stale row
// that outlives the file contradicting it makes dira lie about a decision.
//
// Two mechanisms hold it, and both are structural rather than careful:
//
//  1. **Nothing dira prints comes from the cache.** Entry and Entries go
//     straight to ledger.Store and read the file. The cache answers "which
//     entries", never "what does this entry say".
//
//  2. **No query runs against an unreconciled cache.** Open reconciles before
//     it returns, by listing the ledger and comparing each entry's content
//     hash — not its modification time — against the cached row. There is no
//     method that reads the database without that having happened, so a caller
//     cannot skip the check for speed, and there is no fast path that bypasses
//     it. See dec-0015 for why the version is a content hash and what the
//     alternative cost.
//
// # It is disposable
//
// Deleting .dira/cache/ is always safe and never changes an answer. An
// unwritable or read-only cache directory is not an error either: the same
// queries run against an in-memory database built from the files, which is
// slower and identical. The whole read path therefore has one implementation,
// not a cached one and a fallback one that can drift apart.
//
// # It stays out of git
//
// cst-0003 makes the gitignore a privacy mechanism rather than housekeeping: a
// private: true entry's title lands in this database, and a derived artifact is
// exactly the path by which private context reaches a public commit.
// TestTheCacheIsGitignored asserts it rather than trusting the convention.
package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kazi-org/dira/internal/ledger"

	// modernc.org/sqlite is the pure-Go SQLite, chosen over the cgo
	// mattn/go-sqlite3 so that goreleaser can cross-compile darwin-arm64 and
	// linux-amd64 from one runner without a C toolchain per target (dec-0015,
	// dec-0001, int-0002).
	_ "modernc.org/sqlite"
)

// dbFile is the cache database's name inside the cache directory. The path in
// .gitignore and in the acceptance line is .dira/cache/index.db.
const dbFile = "index.db"

// Path returns the cache database's path inside a cache directory.
//
// It exists so that the thing .gitignore names, the thing a command reports and
// the thing this package opens are provably the same string rather than three
// copies of it — see TestTheCacheIsGitignored, which asserts git ignores exactly
// this path (cst-0003).
func Path(cacheDir string) string { return filepath.Join(cacheDir, dbFile) }

// An Index answers the read path's questions over a ledger.
//
// It is cheap to construct only in the sense that constructing it is the whole
// cost: Open does the reconciling, so an Index that exists is an Index whose
// every row already agrees with the files. Queries after that are database
// reads plus, for anything that renders, a file read.
//
// An Index is not safe for concurrent use by multiple goroutines. dira is a
// short-lived process that opens one, asks its questions and exits (int-0002);
// several dira processes over the same cache are safe, and are what WAL mode and
// the busy timeout are for.
type Index struct {
	store  ledger.Store
	db     *sql.DB
	notice string
	stats  Stats
}

// Stats is what a reconcile did, and is what dira reindex reports.
type Stats struct {
	// Entries and Edges are the index's contents after reconciling.
	Entries int
	Edges   int

	// Indexed is how many entry files were read and parsed this run:
	// everything on a cold cache, nothing on a cache nobody has invalidated,
	// and exactly the changed set in between.
	Indexed int

	// Removed is how many rows named an entry that is no longer in the
	// ledger.
	Removed int

	// Invalid names the entries whose file could not be parsed. They are
	// skipped rather than fatal — dec-0002 chose one file per entry so that a
	// malformed edit takes down one entry instead of the ledger — but they
	// are reported, because an entry silently missing from every answer is
	// the same failure as a stale one.
	Invalid []string

	// Cached is false when the cache directory could not be used and the
	// index was built in memory instead. Notice says why.
	Cached bool

	// Rebuilt is true when the on-disk cache was created from nothing or
	// discarded and recreated, rather than updated in place.
	Rebuilt bool
}

// Open returns an Index over store, caching it in cacheDir.
//
// It reconciles before returning, so the Index it hands back already agrees with
// the ledger. That is the whole point: an Open that deferred the check would let
// a caller query a cache nobody had validated.
//
// cacheDir is created if it does not exist. If it cannot be created, cannot be
// written to, or holds a database this build cannot use, Open does not fail: it
// builds the same index in memory, sets Notice to the reason and returns an
// Index whose answers are identical. An error comes back only when the ledger
// itself cannot be read, which is a real failure and not a degradation.
//
// Close it.
func Open(ctx context.Context, store ledger.Store, cacheDir string) (*Index, error) {
	return open(ctx, store, cacheDir, false)
}

// OpenFresh discards whatever cache is there and builds a new one from the entry
// files alone. It is what dira reindex runs.
//
// Open already reconciles, so this is not how the cache stays correct — it is
// how a user throws it away. A cache that has gone wrong in a way a version
// comparison cannot see, because something other than dira wrote it, is fixed by
// a command rather than by knowing which directory to delete.
//
// It is a second constructor rather than a method on an open Index so that a
// reindex reads each entry file once. A method would have had to undo the
// reconcile Open just did, which over 200 entries is the whole job twice.
func OpenFresh(ctx context.Context, store ledger.Store, cacheDir string) (*Index, error) {
	return open(ctx, store, cacheDir, true)
}

func open(ctx context.Context, store ledger.Store, cacheDir string, fresh bool) (*Index, error) {
	if store == nil {
		return nil, errors.New("index: nil store")
	}

	ix := &Index{store: store}
	db, rebuilt, err := openCache(cacheDir, fresh)
	if err == nil {
		ix.db = db
		ix.stats.Cached = true
		ix.stats.Rebuilt = rebuilt

		err = ix.sync(ctx)
		if err == nil {
			return ix, nil
		}
		_ = db.Close()
		ix.db = nil
		if ledgerErr(err) {
			// The ledger itself is unreadable. Answering from a
			// cache would be answering from the one source that is
			// allowed to be wrong, so this is a failure.
			return nil, err
		}
		// The ledger read fine; the cache would not take the result.
	}

	// Degrade. Everything below this line answers the same questions with the
	// same SQL over the same rows built from the same files; only the
	// database's home has changed. One implementation, so a cached answer and
	// an uncached one cannot drift apart.
	ix.stats = Stats{Rebuilt: true}
	ix.notice = degradedNotice(cacheDir, err)

	mem, _, memErr := openMemory()
	if memErr != nil {
		return nil, fmt.Errorf("index: no usable cache directory (%v) and no in-memory database either: %w", err, memErr)
	}
	ix.db = mem
	if err := ix.sync(ctx); err != nil {
		_ = mem.Close()
		return nil, err
	}
	return ix, nil
}

// Close releases the database.
func (ix *Index) Close() error {
	if ix.db == nil {
		return nil
	}
	db := ix.db
	ix.db = nil
	return db.Close()
}

// Notice is the empty string when the cache is doing its job, and a sentence
// naming what went wrong when it is not.
//
// A command prints it to stderr and exits 0. An unwritable cache directory is a
// slower dira, not a broken one — a read-only checkout, a container with a
// read-only mount and a ledger on a CD are all cases where the right behaviour
// is to answer the question — so it is stated rather than raised.
func (ix *Index) Notice() string { return ix.notice }

// Stats reports what the last reconcile did.
func (ix *Index) Stats() Stats { return ix.stats }

// openCache opens or creates the on-disk cache, reporting whether it had to
// create the schema from nothing. With fresh set, an existing cache is
// discarded rather than reused.
func openCache(cacheDir string, fresh bool) (*sql.DB, bool, error) {
	if cacheDir == "" {
		return nil, false, errors.New("no cache directory given")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, false, err
	}
	path := Path(cacheDir)

	if fresh {
		if err := discard(path); err != nil {
			return nil, false, err
		}
	}

	db, version, err := attach(path)
	if err != nil {
		// Corrupt, truncated, unreadable, or not a SQLite database at
		// all. There is nothing here worth recovering — it is derived
		// data, and the files it was derived from are still there — so
		// it is discarded and rebuilt rather than reported. If it cannot
		// even be discarded, the original reason is what the caller
		// hears, because "permission denied" is more use than "could not
		// delete the thing you could not read".
		if discardErr := discard(path); discardErr != nil {
			return nil, false, err
		}
		db, version, err = attach(path)
		if err != nil {
			return nil, false, err
		}
	}

	switch {
	case version == 0:
		if err := create(db); err != nil {
			_ = db.Close()
			return nil, false, err
		}
		return db, true, nil

	case version != schemaVersion:
		if err := recreate(db); err != nil {
			_ = db.Close()
			return nil, false, err
		}
		return db, true, nil

	default:
		return db, false, nil
	}
}

// attach opens the database and reads its schema version, reporting 0 for one
// that has never been written.
func attach(path string) (*sql.DB, int, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, 0, err
	}
	version, err := readSchemaVersion(db)
	if err != nil {
		_ = db.Close()
		return nil, 0, err
	}
	return db, version, nil
}

// discard removes a cache database and the two siblings WAL mode leaves beside
// it. A stale -wal against a fresh database is a corruption of its own, so all
// three go together or none do.
func discard(path string) error {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// openMemory builds the same schema in a private in-memory database.
func openMemory() (*sql.DB, bool, error) {
	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		return nil, false, err
	}
	// One connection, because every :memory: connection is a different
	// database and a pool would hand out empty ones.
	db.SetMaxOpenConns(1)
	if err := create(db); err != nil {
		_ = db.Close()
		return nil, false, err
	}
	return db, true, nil
}

// openDB connects to the database file.
//
// WAL so that a second dira reading the cache is not blocked by one writing it,
// which is the normal case when hooks fire in parallel sessions. synchronous
// NORMAL rather than FULL because losing the last write to a power cut costs a
// reindex and nothing else. busy_timeout so a concurrent writer waits rather
// than failing the query it was serving.
func openDB(path string) (*sql.DB, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func create(db *sql.DB) error {
	if _, err := db.Exec(ddl); err != nil {
		return err
	}
	_, err := db.Exec(`INSERT INTO meta (key, value) VALUES ('schema_version', ?)`, schemaVersion)
	return err
}

func recreate(db *sql.DB) error {
	if _, err := db.Exec(`DROP TABLE IF EXISTS edges; DROP TABLE IF EXISTS entries; DROP TABLE IF EXISTS meta;`); err != nil {
		return err
	}
	return create(db)
}

// readSchemaVersion returns the cache's schema version, 0 for a database that
// has never been written, and an error for one that cannot be read.
func readSchemaVersion(db *sql.DB) (int, error) {
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'meta'`).Scan(&count); err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, nil
	}
	var version int
	err := db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return version, nil
}

// degradedNotice is what the user is told when the cache could not be used.
//
// It names the directory, the reason and the consequence, and it says what is
// *not* wrong — because "cache" in an error message reads like data loss, and
// the one thing that has not happened is data loss.
func degradedNotice(cacheDir string, err error) string {
	where := cacheDir
	if where == "" {
		where = "the cache directory"
	}
	return fmt.Sprintf("dira: %s is not usable (%v), so this ran straight from the entry files. "+
		"The answer is the same, it just took longer; nothing is lost, and `dira reindex` will build the cache "+
		"once the directory is writable.", where, err)
}

// ledgerErr reports whether err came from reading the ledger rather than from
// the cache. A ledger that cannot be read is a failure; a cache that cannot be
// written is a degradation, and telling them apart is what keeps Open from
// answering "the disk is full" with "here are your entries".
func ledgerErr(err error) bool {
	var le *ledgerError
	return errors.As(err, &le)
}

type ledgerError struct{ err error }

func (e *ledgerError) Error() string { return e.err.Error() }
func (e *ledgerError) Unwrap() error { return e.err }

// placeholders builds the "?, ?, ?" of an IN list.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}
