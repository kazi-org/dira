package ledger

import "fmt"

// A draft is an entry that has everything except the two fields dira supplies
// itself: the id, which is allocated against the ledger at write time so it is
// the lowest unused number for the kind, and created, which is stamped by the
// command. Everything else — kind, title, state, edges, alternatives, source —
// comes from the caller and is held to exactly the rules a stored entry is held
// to.
//
// This file exists because `dira log` has to reject a bad entry *before* it
// touches the ledger, and at that moment the entry has no id. dec-0002 makes the
// ledger the record; an invalid entry reaching it is corruption in the one
// artifact meant to outlive the tool.

// DecodeDraft reads an entry file that has not been given an id yet: YAML
// frontmatter with no `id` field, followed by the markdown body.
//
// It is the parse behind `dira log --stdin`, and the format is deliberately the
// same one dira reads and writes on disk rather than a second wire format. An
// agent assembling an entry already knows this format — it can read the ledger —
// and a JSON or flag-encoded second representation would be a parallel schema to
// keep in step with entry.schema.json. The style memo is filled in by the same
// parse, so what the caller wrote is what lands: their line wrapping, their
// quoting, their block scalars.
//
// The returned entry is validated against every rule a stored entry obeys except
// the two fields it does not have yet. It is not written anywhere; the caller
// stamps created and hands it to Add.
func DecodeDraft(data []byte) (*Entry, error) {
	e, err := decodeEntry(data)
	if err != nil {
		return nil, err
	}
	if e.ID != "" {
		return nil, fmt.Errorf("this entry carries id %q, but a new entry does not choose its own id: "+
			"dira allocates the lowest unused number for the kind, which is what keeps concurrent writers from "+
			"colliding (remove the id field)", e.ID)
	}
	if err := e.ValidateDraft(); err != nil {
		return nil, err
	}
	return e, nil
}

// ValidateDraft checks everything Validate checks except the id and created.
//
// It is what a write path calls to reject a bad entry before it touches the
// ledger, at the point where the entry has no id yet — `dira log` reports the
// failing field and exits without a single write.
//
// It does that by validating a copy carrying stand-in values for exactly those
// two fields, rather than by restating the rules. A second copy of the rules
// would drift from the first, and the failure mode of that drift is an entry
// that passes the draft check and is then rejected — or, worse, accepted — by
// the real one. There is one set of entry rules in this package and Validate is
// it.
//
// The kind is checked first and by hand, because the stand-in id is derived from
// the kind's prefix: an unknown kind has no prefix, so it would otherwise fail
// with a message about the id rather than the message cst-0002 requires.
func (e *Entry) ValidateDraft() error {
	if e.Kind.Prefix() == "" {
		return fmt.Errorf("kind %q is not one of the five (cst-0002 closes the set: %s)", e.Kind, joinKinds())
	}

	probe := *e
	probe.ID = e.Kind.Prefix() + "-0000"
	if probe.Created == "" {
		probe.Created = "1970-01-01T00:00:00Z"
	}
	return probe.Validate()
}
