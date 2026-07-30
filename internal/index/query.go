package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/kazi-org/dira/internal/ledger"
)

// Ref is the index's account of one entry: enough to select it, order it and
// name it, and deliberately not enough to render it.
//
// A caller that wants to print an entry's body, its alternatives or their
// why_nots calls Entry, which reads the file. That split is the files-win
// property expressed as an API rather than as a convention: the cache can only
// ever be wrong about *which* entries an answer contains, never about what any
// of them says — and because Open reconciles on content hashes before any query
// runs (dec-0015), it cannot be wrong about that either.
type Ref struct {
	ID      string
	Kind    ledger.Kind
	State   ledger.State
	Title   string
	Created string
	Updated string
	Private bool
}

// A Backlink is an edge pointing at an entry, seen from the entry it points at.
//
// Edges are stored on the subject entry (dec-0002), so an entry's outgoing edges
// are simply Entry(id).Edges and need no index. Incoming edges are the ones
// nothing on disk records, and they are what a why-chain walks: the decisions
// that derive from an intent, the questions that block a decision, the entry
// that superseded this one.
type Backlink struct {
	// From is the entry that declares the edge.
	From string
	Type ledger.EdgeType
	Note string
}

// A Selector narrows Select. A zero Selector matches every entry.
//
// The fields are conjunctive: an entry matches when its kind is in Kinds (or
// Kinds is empty), its state is in States (or States is empty), and it declares
// at least one edge of type WithEdge (or WithEdge is empty).
type Selector struct {
	Kinds  []ledger.Kind
	States []ledger.State

	// WithEdge selects only entries declaring at least one outgoing edge of
	// this type. It is what makes "open questions that block something" —
	// cst-0001's open blockers, the first thing a brief renders — a query
	// rather than a filter applied after reading 200 files.
	WithEdge ledger.EdgeType

	// Limit caps the result. Zero means no cap. The ordering is total, so a
	// limit takes a prefix that does not depend on how SQLite felt about
	// tie-breaking.
	Limit int
}

// Select returns the entries matching sel, newest first.
//
// The order is `created` descending then `id` ascending, and it is total: two
// entries created in the same second still come back in the same order every
// time, which is what lets E1-L5 drop from the low-priority end reproducibly and
// E1-L4 take golden output.
func (ix *Index) Select(ctx context.Context, sel Selector) ([]Ref, error) {
	var where []string
	var args []any

	if len(sel.Kinds) > 0 {
		where = append(where, "kind IN ("+placeholders(len(sel.Kinds))+")")
		for _, k := range sel.Kinds {
			args = append(args, string(k))
		}
	}
	if len(sel.States) > 0 {
		where = append(where, "state IN ("+placeholders(len(sel.States))+")")
		for _, s := range sel.States {
			args = append(args, string(s))
		}
	}
	if sel.WithEdge != "" {
		where = append(where, "EXISTS (SELECT 1 FROM edges WHERE edges.src = entries.id AND edges.type = ?)")
		args = append(args, string(sel.WithEdge))
	}

	query := `SELECT id, kind, state, title, created, updated, private FROM entries`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created DESC, id ASC"
	if sel.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, sel.Limit)
	}

	rows, err := ix.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("index: selecting: %w", err)
	}
	return scanRefs(rows)
}

// In returns the edges pointing at id, ordered by the entry that declares them
// and then by that entry's own edge order.
//
// An id that no entry points at yields an empty slice, not an error: "nothing
// derives from this" is an answer.
func (ix *Index) In(ctx context.Context, id string) ([]Backlink, error) {
	rows, err := ix.db.QueryContext(ctx,
		`SELECT src, type, note FROM edges WHERE dst = ? ORDER BY src ASC, seq ASC`, id)
	if err != nil {
		return nil, fmt.Errorf("index: backlinks for %s: %w", id, err)
	}
	defer func() { _ = rows.Close() }()

	out := []Backlink{}
	for rows.Next() {
		var b Backlink
		var edgeType string
		if err := rows.Scan(&b.From, &edgeType, &b.Note); err != nil {
			return nil, fmt.Errorf("index: backlinks for %s: %w", id, err)
		}
		b.Type = ledger.EdgeType(edgeType)
		out = append(out, b)
	}
	return out, rows.Err()
}

// Resolve turns what a human typed into entry ids.
//
// An exact id that exists resolves to itself and nothing else, so `dira why
// dec-0002` is never ambiguous. Anything else is matched as a case-insensitive
// substring of a title or as a whole tag, newest first — which is what makes
// `dira why daemon` land on the same entry as `dira why int-0002`.
//
// A term matching nothing yields an empty slice and no error. Deciding that "no
// such entry" is a failure is the caller's business: it is an error for `dira
// why` and an empty section for a brief.
func (ix *Index) Resolve(ctx context.Context, term string) ([]string, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return []string{}, nil
	}

	if ledger.ValidID(term) {
		var id string
		err := ix.db.QueryRowContext(ctx, `SELECT id FROM entries WHERE id = ?`, term).Scan(&id)
		if err == nil {
			return []string{id}, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("index: resolving %s: %w", term, err)
		}
		return []string{}, nil
	}

	lowered := strings.ToLower(term)
	rows, err := ix.db.QueryContext(ctx,
		`SELECT id FROM entries WHERE instr(lower(title), ?) > 0 OR instr(lower(tags), ?) > 0
		 ORDER BY created DESC, id ASC`,
		lowered, " "+lowered+" ")
	if err != nil {
		return nil, fmt.Errorf("index: resolving %q: %w", term, err)
	}
	defer func() { _ = rows.Close() }()

	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("index: resolving %q: %w", term, err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Entry returns the whole entry, read from its file.
//
// It does not consult the cache, and that is the point. Every value dira renders
// — a title, a why_not, a revisit_if, an adr path, the prose body — comes from a
// file read in this process, so no rendering path exists through which a cached
// value could reach a human. The cache decides which entries an answer is about;
// the files decide what it says.
//
// An id that is not in the ledger yields an error wrapping ledger.ErrNotFound.
func (ix *Index) Entry(ctx context.Context, id string) (*ledger.Entry, error) {
	return ix.store.Get(ctx, id)
}

// Entries returns entries in the order asked for, each read from its file.
//
// It is Entry in a loop, and exists so the read path has one place to become
// concurrent if E1-L6's budget ever needs it to. A missing id is an error: a
// caller that got its ids from Select or Resolve has already been told they
// exist, so a silent gap would mean the ledger changed underneath — which the
// caller should hear about, not paper over.
func (ix *Index) Entries(ctx context.Context, ids []string) ([]*ledger.Entry, error) {
	out := make([]*ledger.Entry, 0, len(ids))
	for _, id := range ids {
		entry, err := ix.Entry(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, nil
}
