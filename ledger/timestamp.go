package ledger

import (
	"fmt"
	"time"
)

// Timestamps are strings in this package, and this file is where the rule lives.
//
// The landmine: Go's yaml.v3 implements the YAML 1.1 !!timestamp resolution, so
// an unquoted `created: 2026-07-29T20:00:00Z` decodes to a time.Time rather than
// to a string. Two things then break. A JSON Schema validator handed a
// time.Time reports `invalid jsonType time.Time` at /created — an error that
// names a field which is in fact correct. And re-marshalling a time.Time emits
// it unquoted, so the next reader hits the same coercion, and any consumer with
// a stricter YAML reader than ours sees a typed timestamp where the published
// schema promised a string.
//
// dira's rule, in three parts:
//
//  1. On read, tolerate both forms. The codec walks the yaml.Node tree and takes
//     each scalar's raw source text, so the !!timestamp tag is observed and
//     ignored rather than resolved. A time.Time is never constructed.
//  2. On write, always quote. writeTimestamp is the only path that emits these
//     two fields and it does not consult the plain-scalar heuristic.
//  3. Keep the value textual end to end. Entry.Created is a string, so there is
//     no place in the type for a time.Time to hide.
//
// TestTimestampsSurviveAsStrings covers all three, and fails if the coercion
// ever returns.

// validTimestamp reports whether s is an RFC3339 timestamp, which is what the
// schema's `format: date-time` asserts.
//
// This parses to check the shape and throws the result away deliberately: the
// parsed time is never the value dira stores. Comparing timestamps is a matter
// for whoever needs an ordering, and for the two fields dira has, lexical order
// on RFC3339 UTC text is the same order.
func validTimestamp(field, s string) error {
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		return fmt.Errorf("%s %q is not an RFC3339 timestamp: %w", field, s, err)
	}
	return nil
}
