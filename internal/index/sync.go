package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/kazi-org/dira/internal/ledger"
)

// sync reconciles the index against the ledger, and is the only thing standing
// between a derived cache and a cache that lies.
//
// The shape is deliberately the cheapest one that is still a proof rather than a
// guess:
//
//	read the index's own versions            → id and content hash for every row
//	list the ledger                          → id and content hash for every entry,
//	                                           or ids alone when there are no rows
//	re-read exactly the entries that differ  → nothing, on a cache nobody invalidated
//	drop rows for ids the ledger no longer has
//
// The first two steps are in that order because of the third word of the second
// one. A listing's hashes exist only to be compared against rows, so when there
// are no rows there is nothing to compare and the hash pass is a second read of
// every file in the ledger whose answer is discarded — E1-L6-T5 measured it at
// roughly a sixth of the in-process work of a cold `dira brief`. See list.
//
// After it returns, every row's version equals the hash of the bytes on disk as
// read in this process. Because the version is a content hash rather than a
// modification time (dec-0015), "equals" means the file is byte-for-byte what
// the row was built from — there is no window in which a file can have changed
// and the comparison not notice.
//
// It is called by Open before any query can run, so there is no code path that
// reaches the database without it. That is the constraint the lane brief states
// as "staleness detection cannot be skipped for speed": the check costs 6.9ms
// over a 200-entry ledger against 22ms to answer from files with no cache at
// all, so skipping it would save a quarter of what it protects.
func (ix *Index) sync(ctx context.Context) error {
	// The cache's own account is read FIRST, and the order is load-bearing
	// rather than cosmetic: it is what makes the listing below able to know
	// whether anyone will look at the versions it is about to compute.
	cached, err := ix.cachedVersions(ctx)
	if err != nil {
		return fmt.Errorf("index: reading the cache: %w", err)
	}

	infos, err := ix.list(ctx, len(cached) == 0)
	if err != nil {
		return &ledgerError{fmt.Errorf("index: listing the ledger: %w", err)}
	}

	tx, err := ix.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("index: starting a transaction: %w", err)
	}
	// Committed below; this covers every path that does not get there.
	defer func() { _ = tx.Rollback() }()

	putEntry, err := tx.PrepareContext(ctx, `INSERT INTO entries (id, version, kind, state, title, created, updated, private, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			version = excluded.version, kind = excluded.kind, state = excluded.state,
			title = excluded.title, created = excluded.created, updated = excluded.updated,
			private = excluded.private, tags = excluded.tags`)
	if err != nil {
		return fmt.Errorf("index: preparing the entry upsert: %w", err)
	}
	defer func() { _ = putEntry.Close() }()

	putEdge, err := tx.PrepareContext(ctx, `INSERT INTO edges (src, seq, type, dst, note) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("index: preparing the edge insert: %w", err)
	}
	defer func() { _ = putEdge.Close() }()

	dropEntry, err := tx.PrepareContext(ctx, `DELETE FROM entries WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("index: preparing the entry delete: %w", err)
	}
	defer func() { _ = dropEntry.Close() }()

	dropEdges, err := tx.PrepareContext(ctx, `DELETE FROM edges WHERE src = ?`)
	if err != nil {
		return fmt.Errorf("index: preparing the edge delete: %w", err)
	}
	defer func() { _ = dropEdges.Close() }()

	live := make(map[string]bool, len(infos))
	var invalid []string

	for _, info := range infos {
		live[info.ID] = true
		version, known := cached[info.ID]
		if known && version == info.Version {
			continue
		}

		entry, err := ix.store.Get(ctx, info.ID)
		if err != nil {
			// One unreadable file takes out one entry, not the
			// ledger — that is what dec-0002 bought by choosing a
			// file per entry over a single document. The row is
			// dropped rather than left behind, because a cached
			// answer for a file dira could not read is precisely
			// the stale row this package exists to prevent, and the
			// id is reported so the omission is stated rather than
			// silent.
			if _, err := dropEdges.ExecContext(ctx, info.ID); err != nil {
				return fmt.Errorf("index: %s: %w", info.ID, err)
			}
			if _, err := dropEntry.ExecContext(ctx, info.ID); err != nil {
				return fmt.Errorf("index: %s: %w", info.ID, err)
			}
			if !errors.Is(err, ledger.ErrNotFound) {
				invalid = append(invalid, info.ID)
			} else {
				// Deleted between the list and the read.
				delete(live, info.ID)
			}
			continue
		}

		// The version stored is the one the bytes we actually parsed
		// hash to, not the one the listing reported. They differ only if
		// the file changed between the two reads, and in that case the
		// row must describe what was parsed or the next run would think
		// it was current.
		stored := entry.Version()
		if stored == "" {
			stored = info.Version
		}

		if _, err := putEntry.ExecContext(ctx, entry.ID, stored, string(entry.Kind), string(entry.State),
			entry.Title, entry.Created, entry.Updated, boolToInt(entry.Private), tagsColumn(entry.Tags)); err != nil {
			return fmt.Errorf("index: writing %s: %w", entry.ID, err)
		}
		// An entry the index has never seen has no edges to clear, and a
		// cold rebuild is entirely made of those — skipping the delete
		// there is 200 fewer statements on the one path where the cache
		// costs more than it saves.
		if known {
			if _, err := dropEdges.ExecContext(ctx, entry.ID); err != nil {
				return fmt.Errorf("index: writing %s: %w", entry.ID, err)
			}
		}
		for i, edge := range entry.Edges {
			if _, err := putEdge.ExecContext(ctx, entry.ID, i, string(edge.Type), edge.To, edge.Note); err != nil {
				return fmt.Errorf("index: writing %s: %w", entry.ID, err)
			}
		}
		ix.stats.Indexed++
	}

	// Rows the ledger no longer has. Iterated in sorted order so a reindex is
	// deterministic down to the statement sequence, which is what makes two
	// rebuilds of the same ledger comparable.
	stale := make([]string, 0, len(cached))
	for id := range cached {
		if !live[id] {
			stale = append(stale, id)
		}
	}
	sort.Strings(stale)
	for _, id := range stale {
		if _, err := dropEdges.ExecContext(ctx, id); err != nil {
			return fmt.Errorf("index: dropping %s: %w", id, err)
		}
		if _, err := dropEntry.ExecContext(ctx, id); err != nil {
			return fmt.Errorf("index: dropping %s: %w", id, err)
		}
		ix.stats.Removed++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("index: committing: %w", err)
	}

	sort.Strings(invalid)
	ix.stats.Invalid = invalid
	if len(invalid) > 0 {
		ix.notice = strings.TrimSpace(ix.notice + fmt.Sprintf(
			"\ndira: %d entry file(s) could not be read and are missing from every answer: %s.",
			len(invalid), strings.Join(invalid, ", ")))
	}

	return ix.countRows(ctx)
}

// An idLister can enumerate the ledger without hashing it. It is satisfied by
// ledger/local's Store and is deliberately an OPTIONAL interface rather than a
// method on ledger.Store: a backend that cannot list more cheaply than it can
// version — the GitHub Contents API returns a sha with the listing, so it is one
// — should not be made to implement a second method that saves it nothing.
type idLister interface {
	ListIDs(ctx context.Context) ([]ledger.EntryInfo, error)
}

// list enumerates the ledger, with versions unless the cache is empty.
//
// The versions List computes exist for exactly one purpose: to decide, per
// entry, whether the cached row is still current. When the cache holds no rows
// there is nothing to be current — every entry is read and parsed below whatever
// its hash is — so on that path the version pass is a second open, read and
// SHA-1 of every file in the ledger whose answer is discarded. E1-L6-T5
// attributed it at roughly a sixth of the in-process work of a cold
// `dira brief`, and this is the whole of that lane's optimisation.
//
// Nothing about staleness detection is weakened, and the reason is that the
// version STORED was never the one this listing computed. sync writes
// entry.Version() — the hash of the bytes it actually parsed — and falls back to
// info.Version only when the store left the entry's own version empty. So the
// rows this path writes are byte-for-byte the rows the versioned path would have
// written, and the next run's warm reconcile compares against exactly the same
// values. What is skipped is a question nobody asked, not a check.
//
// The condition is `no rows at all`, not `Rebuilt`, and that is the strict
// version: one surviving row is enough to take the full versioned listing.
func (ix *Index) list(ctx context.Context, cold bool) ([]ledger.EntryInfo, error) {
	if cold {
		if l, ok := ix.store.(idLister); ok {
			return l.ListIDs(ctx)
		}
	}
	return ix.store.List(ctx)
}

// cachedVersions is the index's own account of what it holds.
func (ix *Index) cachedVersions(ctx context.Context) (map[string]string, error) {
	rows, err := ix.db.QueryContext(ctx, `SELECT id, version FROM entries`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := map[string]string{}
	for rows.Next() {
		var id, version string
		if err := rows.Scan(&id, &version); err != nil {
			return nil, err
		}
		out[id] = version
	}
	return out, rows.Err()
}

func (ix *Index) countRows(ctx context.Context) error {
	if err := ix.db.QueryRowContext(ctx, `SELECT count(*) FROM entries`).Scan(&ix.stats.Entries); err != nil {
		return fmt.Errorf("index: counting entries: %w", err)
	}
	if err := ix.db.QueryRowContext(ctx, `SELECT count(*) FROM edges`).Scan(&ix.stats.Edges); err != nil {
		return fmt.Errorf("index: counting edges: %w", err)
	}
	return nil
}

// tagsColumn renders an entry's tags for the LIKE that Resolve runs.
//
// Space-delimited *and* space-surrounded, so "% cache %" matches the tag `cache`
// and not the tag `cache-warm`. Tags are already lowercase by schema
// (^[a-z0-9][a-z0-9-]*$), so no folding is needed here; Resolve lowercases the
// term because the term comes from a human.
func tagsColumn(tags []string) string {
	if len(tags) == 0 {
		return " "
	}
	return " " + strings.Join(tags, " ") + " "
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// scanRefs reads a Ref listing.
func scanRefs(rows *sql.Rows) ([]Ref, error) {
	defer func() { _ = rows.Close() }()

	var out []Ref
	for rows.Next() {
		var r Ref
		var kind, state string
		var private int
		if err := rows.Scan(&r.ID, &kind, &state, &r.Title, &r.Created, &r.Updated, &private); err != nil {
			return nil, err
		}
		r.Kind = ledger.Kind(kind)
		r.State = ledger.State(state)
		r.Private = private != 0
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Ref{}
	}
	return out, nil
}
