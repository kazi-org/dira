package ledger

import (
	"context"
	"errors"
	"fmt"
)

// The read path for a ledger that is not ours to write.
//
// cst-0003 rule 1 is absolute — "A child ledger may read its parents. A parent
// is never written to by a child, and dira has no verb that does so" — and the
// constraint says why it has to be architectural rather than careful: a private
// note published into a public repository's git history cannot be un-published,
// so a violation is a security bug. A rule held by every caller remembering it
// is a rule that survives until the first caller who does not.
//
// So it is held by a type. A parent ledger is opened through ReadOnly and the
// only Store the caller then holds refuses Create, Put and Delete outright,
// before the inner store is reached. There is no accessor for the inner store,
// deliberately: an Inner() method would be a documented way around the
// guarantee, and the guarantee is the whole point of the type.
//
// # The hazard is the cache, not Put
//
// Nothing in dira calls Put on a parent today, so a wrapper that only refused
// the three write methods would be guarding a door nobody walks through. The
// write that actually happens is index.Open's: it creates .dira/cache/index.db
// under the ledger it is caching, so opening a parent the ordinary way writes to
// that parent. This type does not prevent that on its own — it cannot, because
// the cache is opened beside the store rather than through it. What it does is
// make the read path for a parent a different, narrower thing than the read path
// for our own ledger, so a caller that reaches for index.Open over a parent is
// visibly doing something else.
//
// A test for this has to digest the filesystem tree rather than ask git: the
// .gitignore ignores .dira/cache/ precisely because a derived artifact holds a
// private entry's title, so a git-based check would report a clean parent while
// the cache was being written into it.

// ErrReadOnly is the error every refused write wraps. Callers that need to tell
// a refusal from a storage failure check it with errors.Is.
var ErrReadOnly = errors.New("ledger: read-only")

// errNoInnerStore is what a ReadOnly built over nothing reports when it is read.
// It is a caller's bug rather than a refusal, so it does not wrap ErrReadOnly:
// reporting "read-only" for a store that was never supplied would send whoever
// reads the message looking for a permissions problem.
var errNoInnerStore = errors.New("ledger: read-only store has no ledger to read")

// ReadOnly returns a Store that reads inner and refuses to write it.
//
// Get and List pass straight through. Create, Put and Delete return an error
// wrapping ErrReadOnly and naming cst-0003 rule 1, without calling inner at all —
// the refusal is a decision about what dira is allowed to do, not a report of
// what the backend would have said, so it does not depend on the backend being
// reachable, on the context still being live, or on the entry being valid.
//
// It is a Store because that is the seam dec-0005 commits to: a parent read
// through the GitHub backend (E7) needs the same guarantee, and it gets it here
// rather than in a second copy beside that backend.
func ReadOnly(inner Store) Store { return readOnly{inner: inner} }

// readOnly is unexported so that ReadOnly is the only way to build one and a
// caller cannot reach the field.
type readOnly struct{ inner Store }

// Get reads one entry from the wrapped ledger, unchanged.
func (r readOnly) Get(ctx context.Context, id string) (*Entry, error) {
	if r.inner == nil {
		return nil, errNoInnerStore
	}
	return r.inner.Get(ctx, id)
}

// List reads the wrapped ledger's contents, unchanged.
func (r readOnly) List(ctx context.Context) ([]EntryInfo, error) {
	if r.inner == nil {
		return nil, errNoInnerStore
	}
	return r.inner.List(ctx)
}

// Create is refused. An entry dira invented under someone else's ledger is the
// clearest form of the upward write cst-0003 forbids.
func (r readOnly) Create(_ context.Context, e *Entry) error {
	return refuseWrite("create", entryID(e))
}

// Put is refused. Replacing an entry in a parent rewrites a record whose owner
// is a different ledger and, in the tier design, frequently a different person.
func (r readOnly) Put(_ context.Context, e *Entry) error {
	return refuseWrite("put", entryID(e))
}

// Delete is refused, for the reason one step stronger than Put's: dec-0002
// makes the ledger the record, and erasing another ledger's record from here
// destroys context that was never ours to hold.
func (r readOnly) Delete(_ context.Context, id string) error {
	return refuseWrite("delete", id)
}

// refuseWrite is the one message all three refusals share. It names the
// operation, the entry where there is one, and the constraint — so the reader of
// a failing hook learns which rule stopped them rather than that "something is
// read-only".
func refuseWrite(op, id string) error {
	subject := ""
	if id != "" {
		subject = " " + id
	}
	return fmt.Errorf("%w: refusing to %s%s in a ledger opened for reading: "+
		"inheritance is one-way, and a parent is never written to by a child (cst-0003 rule 1)",
		ErrReadOnly, op, subject)
}

// entryID is the id to name in a refusal, and the empty string for an entry
// that has none or is not there at all. A nil entry is refused like any other
// write rather than reported as a nil pointer: it is still a write, and the
// caller learns the same thing either way.
func entryID(e *Entry) string {
	if e == nil {
		return ""
	}
	return e.ID
}
