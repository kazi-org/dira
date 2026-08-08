package ledger

import (
	"context"
	"errors"
)

// Errors a Store returns for conditions callers branch on. Check them with
// errors.Is; a backend wraps them with whatever detail it has.
var (
	// ErrNotFound means no entry with that id exists in the ledger.
	ErrNotFound = errors.New("entry not found")

	// ErrExists means Create was asked for an id that is already taken.
	// It is the signal an id allocator retries on (E1-L2).
	ErrExists = errors.New("entry already exists")
)

// EntryInfo is what a Store can report about an entry without reading it.
//
// Version is what makes a derived cache possible without a daemon: a reindex
// (E1-L3) lists the ledger, compares each Version against what it cached, and
// re-reads only what changed. Over 200 entries that is one directory read
// rather than 200 file reads, which is where int-0002's budget goes if this
// field does not exist.
type EntryInfo struct {
	ID string

	// Version is an opaque, backend-defined token that changes if and only
	// if the stored entry changed. Only the backend that produced it may
	// interpret it: local derives it from the file's modification time and
	// size, github will use the blob sha. Nothing above the interface may
	// parse it, compare it for ordering, or persist an assumption about its
	// shape beyond equality.
	Version string
}

// A Store is every read and write dira performs against a ledger. It is the
// whole storage surface, and dec-0005 commits to it before the first surface
// ships: local (internal/ledger/local) is the only implementation today, and
// github (the Contents API, E7) must be addable with no change above this
// interface.
//
// The methods are therefore expressed in entries, not in files. No method takes
// or returns a path, an *os.File or an fs.FS, because none of those exist over
// an HTTP API. What each method does map onto, on both backends:
//
//	Get     read one file            GET    /contents/{path}
//	List    read the directory       GET    /contents/{dir}
//	Create  create, failing if taken PUT    with no sha
//	Put     replace                  PUT    with the entry's sha
//	Delete  remove one file          DELETE /contents/{path}
//
// Every mutation is a single-entry operation because dec-0002 put edges on the
// subject entry, so no dira write ever needs two files changed together. There
// is deliberately no transaction, no batch and no lock in this interface: they
// would all be local-only concepts, and a Store that needed them could not be
// implemented over the GitHub API at all.
//
// Implementations are safe for concurrent use by multiple goroutines.
type Store interface {
	// Get returns the entry with the given id, or an error wrapping
	// ErrNotFound.
	Get(ctx context.Context, id string) (*Entry, error)

	// List returns every entry in the ledger, id and version only, sorted
	// by id. It reads no entry bodies. An empty ledger is an empty slice
	// and a nil error — not an error.
	List(ctx context.Context) ([]EntryInfo, error)

	// Create writes an entry that must not already exist, returning an
	// error wrapping ErrExists if the id is taken.
	//
	// This is the only collision-free primitive in the interface, and it is
	// here because id allocation needs one: `dira log` runs unattended from
	// Stop hooks in parallel sessions, so "scan for the highest id, add
	// one, write" races, and the failure is silent — one entry overwriting
	// another. Create makes the winner observable, so an allocator can
	// retry the next id rather than lose an entry. It is a native operation
	// on both backends (O_EXCL locally, a sha-less PUT on the Contents API),
	// which is why it belongs here rather than above the interface.
	Create(ctx context.Context, e *Entry) error

	// Put writes an entry, replacing any existing one with the same id.
	// Callers that must not clobber a concurrent write use Create.
	Put(ctx context.Context, e *Entry) error

	// Delete removes an entry, returning an error wrapping ErrNotFound if
	// it is not there.
	Delete(ctx context.Context, id string) error
}
