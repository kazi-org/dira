package index_test

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/index"

	_ "modernc.org/sqlite"
)

// This file reaches into the cache database directly, which no other test and no
// production code does. Two of the lane's guarantees cannot be demonstrated any
// other way: that a rebuilt cache holds the same rows as the one it replaced,
// and that a row corrupted in a field the version check does not cover is
// invisible to a reconcile and repaired by a reindex. Both are statements about
// the cache's contents rather than about its answers.

// openCacheFile opens the cache database read-write, as a second process would.
func openCacheFile(t *testing.T, cacheDir string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+index.Path(cacheDir)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("opening the cache at %s: %v", cacheDir, err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// cacheRows renders every row of the cache in a stable order.
//
// It compares the index's contents rather than a checksum of the database file,
// because a SQLite file is not byte-stable across rebuilds — page allocation,
// the freelist and the WAL all vary — and asserting on those bytes would be
// asserting on SQLite's internals rather than on dira's. What the acceptance
// line asks for is that a rebuilt cache produces an identical *result*, and this
// is that claim at its strongest: identical rows, not merely identical answers
// to the questions the harness happened to ask.
func cacheRows(t *testing.T, cacheDir string) string {
	t.Helper()

	db := openCacheFile(t, cacheDir)
	var lines []string

	rows, err := db.Query(`SELECT id, version, kind, state, title, created, updated, private, tags FROM entries`)
	if err != nil {
		t.Fatalf("reading entries: %v", err)
	}
	for rows.Next() {
		var id, version, kind, state, title, created, updated, tags string
		var private int
		if err := rows.Scan(&id, &version, &kind, &state, &title, &created, &updated, &private, &tags); err != nil {
			t.Fatalf("scanning an entry row: %v", err)
		}
		lines = append(lines, fmt.Sprintf("entry\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s",
			id, version, kind, state, title, created, updated, private, tags))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading entries: %v", err)
	}
	_ = rows.Close()

	edgeRows, err := db.Query(`SELECT src, seq, type, dst, note FROM edges`)
	if err != nil {
		t.Fatalf("reading edges: %v", err)
	}
	for edgeRows.Next() {
		var src, edgeType, dst, note string
		var seq int
		if err := edgeRows.Scan(&src, &seq, &edgeType, &dst, &note); err != nil {
			t.Fatalf("scanning an edge row: %v", err)
		}
		lines = append(lines, fmt.Sprintf("edge\t%s\t%d\t%s\t%s\t%s", src, seq, edgeType, dst, note))
	}
	if err := edgeRows.Err(); err != nil {
		t.Fatalf("reading edges: %v", err)
	}
	_ = edgeRows.Close()

	if len(lines) == 0 {
		t.Fatal("the cache holds no rows; a comparison against it would pass vacuously")
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// corruptTitle rewrites a row's title while leaving its version alone, which is
// the one kind of wrongness a reconcile is blind to by construction.
func corruptTitle(t *testing.T, cacheDir, id, title string) {
	t.Helper()

	db := openCacheFile(t, cacheDir)
	res, err := db.Exec(`UPDATE entries SET title = ? WHERE id = ?`, title, id)
	if err != nil {
		t.Fatalf("corrupting %s: %v", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		t.Fatalf("corrupting %s: %v", id, err)
	}
	if n != 1 {
		t.Fatalf("corrupting %s changed %d rows, want 1", id, n)
	}
}

// setSchemaVersion writes a schema version the running build does not
// understand, which is what a cache left behind by a different dira looks like.
func setSchemaVersion(t *testing.T, cacheDir string, version int) {
	t.Helper()

	db := openCacheFile(t, cacheDir)
	if _, err := db.Exec(`UPDATE meta SET value = ? WHERE key = 'schema_version'`, version); err != nil {
		t.Fatalf("setting the schema version: %v", err)
	}
}
