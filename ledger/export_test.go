package ledger

// This file exposes internals to the external test package. It is _test.go, so
// none of it is part of the package API.

// WithoutRecordedStyle returns a copy of e that has forgotten how it was
// written on disk. Encoding it exercises the canonical write path — the one
// every entry dira composes itself takes — over entries a human wrote.
func WithoutRecordedStyle(e *Entry) *Entry {
	clone := *e
	clone.style = nil
	return &clone
}

// WithVersion returns a copy of e carrying the given backend version, so a
// backend's contract test can construct the state a read would have produced.
func WithVersion(e *Entry, version string) *Entry {
	clone := *e
	clone.version = version
	return &clone
}

// PlainSafe reports whether value can be written as an unquoted scalar. The
// emitter's quoting rule is worth testing directly as well as through a
// round-trip, because a false positive here corrupts a file.
func PlainSafe(value string) bool { return plainSafe(value) }

// Wrap exposes the greedy line filler used for folded scalars dira composes.
func Wrap(text string, width int) []string { return wrap(text, width) }
