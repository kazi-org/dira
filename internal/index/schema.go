package index

// schemaVersion is bumped whenever ddl changes. An existing cache carrying a
// different value is dropped and rebuilt rather than migrated.
//
// There is no migration path and there will not be one. The cache is derived
// and disposable (dec-0002), so the cost of throwing it away is one reindex —
// and a migration is code that can be wrong about the shape of data it did not
// write, which is exactly how a derived cache becomes authoritative by accident.
const schemaVersion = 1

// ddl is the whole cache. Two tables and the indexes that make the two queries
// E1-L4 and E1-L5 need cheap.
//
// What is deliberately absent is as much of the design as what is present:
//
//   - No body, no alternatives, no why_not. Those are what dira prints, and
//     nothing dira prints comes from the cache — Entry and Entries read the
//     file. The cache narrows a query; the file answers it.
//   - No execution status, no kazi state, no bucket (dec-0004). That join is
//     E4's and is computed at read time from kazi, never stored here.
//   - No foreign key from edges.dst to entries.id. An edge may name an entry
//     that does not exist yet, or a kazi artifact that never will
//     (realized_by), and a cache that refused to hold the ledger's real shape
//     would be lying about it.
//
// title and tags are here because Select orders on them and Resolve searches
// them. They are content, so they carry the same guarantee as every other
// column: no query runs against a row whose version does not match the file's
// content hash as read in this same process (see sync.go).
const ddl = `
CREATE TABLE meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE entries (
	id      TEXT PRIMARY KEY,
	version TEXT NOT NULL,
	kind    TEXT NOT NULL,
	state   TEXT NOT NULL,
	title   TEXT NOT NULL,
	created TEXT NOT NULL,
	updated TEXT NOT NULL,
	private INTEGER NOT NULL,
	tags    TEXT NOT NULL
);

CREATE INDEX entries_kind_state ON entries (kind, state);
CREATE INDEX entries_created ON entries (created DESC, id);

CREATE TABLE edges (
	src  TEXT NOT NULL,
	seq  INTEGER NOT NULL,
	type TEXT NOT NULL,
	dst  TEXT NOT NULL,
	note TEXT NOT NULL,
	PRIMARY KEY (src, seq)
);

CREATE INDEX edges_dst ON edges (dst, type);
CREATE INDEX edges_type ON edges (type, src);
`
