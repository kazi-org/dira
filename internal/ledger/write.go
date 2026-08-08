package ledger

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// maxAllocationAttempts bounds the id search so a pathological ledger or a
// misbehaving backend fails with an explanation rather than spinning. Every
// attempt past the first means a concurrent writer took the candidate id, so the
// bound is really a concurrency bound: it tolerates four thousand writers racing
// for the same kind at the same instant, against the 32 the design has to
// survive. A ledger will hit int-0002's latency budget long before it hits this.
const maxAllocationAttempts = 4096

// Add writes e as a new entry, giving it the lowest unused id for its kind.
//
// Id allocation is a shared-counter problem with unattended concurrent writers —
// `dira log` runs from Stop hooks in parallel sessions on one repository — and
// the naive form of it is a silent data-loss bug: scan for the highest id, add
// one, write, and two sessions that scanned at the same moment produce one entry
// where there should be two. Nothing reports it, because a write that overwrote
// something looks exactly like a write.
//
// So allocation is not a scan. The scan only proposes; Store.Create disposes.
// Create is exclusive (dec-0005 put it in the interface for this: O_EXCL on the
// local backend, a sha-less PUT on the GitHub Contents API), so a losing racer
// gets ErrExists instead of clobbering the winner, learns that the id is taken,
// and moves to the next candidate. Every candidate has exactly one winner, so N
// racers take N distinct ids after at most N-1 collisions each. There is no lock
// anywhere in this, which is what lets the same allocator work over a backend
// where no lock exists.
//
// The one List is a starting point rather than a source of truth. It is what
// makes the common case one round trip; correctness comes from Create alone, and
// a stale list costs a retry rather than an entry.
//
// On success e.ID holds the allocated id. On failure e.ID is left empty, because
// an entry carrying an id that was never written is a lie a caller would report.
func Add(ctx context.Context, s Store, e *Entry) error {
	if e == nil {
		return errors.New("nil entry")
	}
	if e.ID != "" {
		return fmt.Errorf("entry already carries id %q; Add allocates the id and Put replaces an existing entry", e.ID)
	}
	if e.Created == "" {
		// The clock belongs to the caller. This package has no business
		// reading one, and a default here would be a timestamp nobody
		// chose appearing in the permanent record.
		return errors.New("created is required: the caller stamps it, so a test and a hook agree on what time it is")
	}
	if err := e.ValidateDraft(); err != nil {
		return err
	}

	infos, err := s.List(ctx)
	if err != nil {
		return fmt.Errorf("reading the ledger to allocate an id: %w", err)
	}
	taken := make(map[int]bool, len(infos))
	for _, info := range infos {
		if n, ok := idNumber(info.ID, e.Kind); ok {
			taken[n] = true
		}
	}

	n := 1
	for attempt := 0; attempt < maxAllocationAttempts; attempt++ {
		for taken[n] {
			n++
		}
		e.ID = FormatID(e.Kind, n)

		err := s.Create(ctx, e)
		if err == nil {
			return nil
		}
		e.ID = ""
		if !errors.Is(err, ErrExists) {
			return err
		}
		// A concurrent writer got there first. That is the mechanism
		// working, not a failure: the id is now known to be taken.
		taken[n] = true
	}
	return fmt.Errorf("allocating an id for a %s: %d candidate ids in a row were taken by concurrent writers",
		e.Kind, maxAllocationAttempts)
}

// AddEdge appends edge to e and reports whether e changed.
//
// An edge identical to one already on the entry changes nothing and is not an
// error: `dira log` runs unattended from hooks that fire more than once for the
// same conclusion, and a hook that fails the second time it is right is a hook
// that gets uninstalled.
//
// An edge with the same type and target but a different note is a conflict, not
// an update. One of the two notes is a human's sentence about why the edge
// exists, and neither silently keeping it nor silently replacing it is defensible
// without knowing which — so the caller is told and decides.
func AddEdge(e *Entry, edge Edge) (bool, error) {
	for _, existing := range e.Edges {
		if existing.Type != edge.Type || existing.To != edge.To {
			continue
		}
		if existing.Note == edge.Note {
			return false, nil
		}
		return false, fmt.Errorf("%s already has a %s edge to %s carrying the note %q; "+
			"refusing to replace it with %q — edit the entry if the note is wrong",
			e.ID, edge.Type, edge.To, existing.Note, edge.Note)
	}
	e.Edges = append(e.Edges, edge)
	return true, nil
}

// AddTag appends tag to e and reports whether e changed. A tag already present
// changes nothing, for the same reason a duplicate edge does not.
func AddTag(e *Entry, tag string) bool {
	for _, existing := range e.Tags {
		if existing == tag {
			return false
		}
	}
	e.Tags = append(e.Tags, tag)
	return true
}

// FormatID renders the id of the nth entry of a kind: the schema's
// ^(int|dec|qst|cst|note)-[0-9]{4,}$, zero-padded to the four digits every entry
// in this ledger uses and widening past four rather than wrapping.
func FormatID(k Kind, n int) string {
	return fmt.Sprintf("%s-%04d", k.Prefix(), n)
}

// idNumber returns the numeric part of an id belonging to kind k. The second
// result is false for an id of another kind or a name that is not an id at all,
// both of which a ledger directory can legitimately contain.
func idNumber(id string, k Kind) (int, bool) {
	if !ValidID(id) {
		return 0, false
	}
	prefix, digits, ok := strings.Cut(id, "-")
	if !ok || prefix != k.Prefix() {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	return n, true
}
