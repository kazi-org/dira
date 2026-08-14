package installhooks

// The byte-span JSON scanner and splicer.
//
// encoding/json is the authoritative malformed-JSON check, run first. Once
// bytes pass it, this file scans the SAME bytes into a span tree recording the
// byte offsets of every object member and array element — never decoding a
// value into a Go type it would later have to re-encode. An edit is then
// either an insertion of new text at a computed offset or a deletion of a
// span, so everything neither install nor uninstall touches survives
// byte-identically because it is never rewritten. This mirrors
// install_hooks.ex:243-259 (scan_value and friends) and is the only reason
// byte-identical preservation can be promised at all: a decode/json.Marshal
// round trip sorts map keys and discards formatting, which would rewrite every
// line of a file it was asked to merge into — including the `//`-prefixed
// documentation keys this project's own example file uses.
//
// The package names no path and opens no file. Everything here is a pure
// function of []byte in and []byte or a tree out.

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
)

// A Kind is what one node of the span tree represents.
type Kind int

const (
	KindObject Kind = iota
	KindArray
	KindString
	KindScalar
)

// A Node is one JSON value's byte span, plus its children when it has any.
// Start and Stop bound the value's own text: data[Start:Stop] is exactly the
// bytes that value occupies, unmodified.
type Node struct {
	Kind  Kind
	Start int
	Stop  int

	// Members holds Kind == KindObject's members, in the document's own
	// order. A duplicate key is kept as two members, matching encoding/json's
	// own decode -- Member below resolves it the same way encoding/json
	// resolves a duplicate key when decoding into a map: the last one wins.
	Members []Member

	// Elements holds Kind == KindArray's elements, in the document's own
	// order.
	Elements []*Node
}

// A Member is one object member: its key, the byte offset the key's opening
// quote sits at, and the member's value node.
type Member struct {
	Key      string
	KeyStart int
	Value    *Node
}

// Member returns the member of an object node named key, or nil. When key is
// declared more than once the LAST declaration wins, matching the semantics
// encoding/json applies when it decodes an object into a map -- the two must
// agree on which value a duplicate key resolves to, or a caller comparing this
// tree against a plain json.Unmarshal of the same bytes would see two
// different answers for the same document.
func (n *Node) Member(key string) *Member {
	if n == nil {
		return nil
	}
	var found *Member
	for i := range n.Members {
		if n.Members[i].Key == key {
			found = &n.Members[i]
		}
	}
	return found
}

// MemberIndex returns the index into Members of member, or -1. It exists so a
// caller holding a *Member found via Member above can locate its position in
// the slice without a second linear search re-implementing the same
// comparison.
func (n *Node) MemberIndex(member *Member) int {
	for i := range n.Members {
		if &n.Members[i] == member {
			return i
		}
	}
	return -1
}

// Scan validates data with encoding/json -- the authoritative malformed-JSON
// check -- and, only once that passes, walks the SAME bytes into a span tree.
//
// It also enforces the two shape facts every caller in this package needs
// settled before it can decide anything about the document: the root must be
// a JSON object (a settings file is never anything else), and a "hooks" value,
// if the document declares one at all, must itself be a JSON object. A
// document that fails either check returns a named error and no tree --
// install and uninstall both refuse rather than guess at a document shaped
// like neither (T4, T5).
func Scan(data []byte) (*Node, error) {
	if !json.Valid(data) {
		return nil, fmt.Errorf("%w", ErrMalformed)
	}

	var probe any
	if err := json.Unmarshal(data, &probe); err != nil {
		// json.Valid already accepted data, so this branch is not reachable
		// in practice. It is a named error rather than a panic because
		// nothing in this package should ever crash on bytes it just
		// validated itself.
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if _, ok := probe.(map[string]any); !ok {
		return nil, fmt.Errorf("%w: the root is not a JSON object", ErrShape)
	}

	root, _, err := scanValue(data, 0)
	if err != nil {
		// Also not reachable on data json.Valid already accepted; named
		// rather than panicked for the same reason as above.
		return nil, fmt.Errorf("%w: %v", ErrShape, err)
	}

	if hooks := root.Member("hooks"); hooks != nil && hooks.Value.Kind != KindObject {
		return nil, fmt.Errorf(`%w: "hooks" is not a JSON object`, ErrShape)
	}
	return root, nil
}

func scanValue(b []byte, i int) (*Node, int, error) {
	i = skipWS(b, i)
	if i >= len(b) {
		return nil, i, fmt.Errorf("unexpected end of input at byte %d", i)
	}
	switch b[i] {
	case '{':
		return scanObject(b, i)
	case '[':
		return scanArray(b, i)
	case '"':
		return scanString(b, i)
	default:
		return scanScalar(b, i)
	}
}

func skipWS(b []byte, i int) int {
	for i < len(b) {
		switch b[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

func scanObject(b []byte, start int) (*Node, int, error) {
	i := skipWS(b, start+1)
	if i >= len(b) {
		return nil, i, fmt.Errorf("unterminated object starting at byte %d", start)
	}
	if b[i] == '}' {
		return &Node{Kind: KindObject, Start: start, Stop: i + 1}, i + 1, nil
	}

	var members []Member
	for {
		keyStart := skipWS(b, i)
		keyNode, j, err := scanString(b, keyStart)
		if err != nil {
			return nil, j, err
		}
		key, err := decodeString(b, keyNode)
		if err != nil {
			return nil, j, err
		}
		j = skipWS(b, j)
		if j >= len(b) || b[j] != ':' {
			return nil, j, fmt.Errorf("expected ':' at byte %d", j)
		}
		value, k, err := scanValue(b, j+1)
		if err != nil {
			return nil, k, err
		}
		members = append(members, Member{Key: key, KeyStart: keyStart, Value: value})
		k = skipWS(b, k)
		if k >= len(b) {
			return nil, k, fmt.Errorf("unterminated object starting at byte %d", start)
		}
		switch b[k] {
		case ',':
			i = k + 1
			continue
		case '}':
			return &Node{Kind: KindObject, Start: start, Stop: k + 1, Members: members}, k + 1, nil
		default:
			return nil, k, fmt.Errorf("expected ',' or '}' at byte %d", k)
		}
	}
}

func scanArray(b []byte, start int) (*Node, int, error) {
	i := skipWS(b, start+1)
	if i >= len(b) {
		return nil, i, fmt.Errorf("unterminated array starting at byte %d", start)
	}
	if b[i] == ']' {
		return &Node{Kind: KindArray, Start: start, Stop: i + 1}, i + 1, nil
	}

	var elements []*Node
	for {
		node, j, err := scanValue(b, i)
		if err != nil {
			return nil, j, err
		}
		elements = append(elements, node)
		j = skipWS(b, j)
		if j >= len(b) {
			return nil, j, fmt.Errorf("unterminated array starting at byte %d", start)
		}
		switch b[j] {
		case ',':
			i = j + 1
			continue
		case ']':
			return &Node{Kind: KindArray, Start: start, Stop: j + 1, Elements: elements}, j + 1, nil
		default:
			return nil, j, fmt.Errorf("expected ',' or ']' at byte %d", j)
		}
	}
}

func scanString(b []byte, start int) (*Node, int, error) {
	i := start + 1
	for i < len(b) {
		switch b[i] {
		case '\\':
			i += 2
		case '"':
			return &Node{Kind: KindString, Start: start, Stop: i + 1}, i + 1, nil
		default:
			i++
		}
	}
	return nil, i, fmt.Errorf("unterminated string starting at byte %d", start)
}

func scanScalar(b []byte, start int) (*Node, int, error) {
	i := start
	for i < len(b) {
		switch b[i] {
		case ',', '}', ']', ' ', '\t', '\n', '\r':
			return &Node{Kind: KindScalar, Start: start, Stop: i}, i, nil
		}
		i++
	}
	return &Node{Kind: KindScalar, Start: start, Stop: i}, i, nil
}

// decodeString decodes a scanned string node's actual value, delegating to
// encoding/json on the raw slice rather than re-implementing escape handling —
// the scanner's own job is only ever to find the span, matching kazi's
// scan_string, which hands the same slice back to Jason.
func decodeString(b []byte, n *Node) (string, error) {
	var s string
	if err := json.Unmarshal(b[n.Start:n.Stop], &s); err != nil {
		return "", fmt.Errorf("decoding string at byte %d: %w", n.Start, err)
	}
	return s, nil
}

// An Insertion is text to splice into data at a byte offset. The offset is
// computed against data's ORIGINAL coordinates -- Insert reorders internally
// so that every caller only ever has to think in terms of the tree Scan
// handed it, never in terms of what an earlier edit already shifted.
type Insertion struct {
	At   int
	Text []byte
}

// Insert splices every insertion into data and returns the result, leaving
// data itself untouched.
//
// Positions are computed against data's original coordinates. Insert applies
// them back-to-front (highest offset first) so that inserting at one offset
// never invalidates an offset computed for another edit.
//
// Ties -- two insertions computed at the same offset, which happens when an
// object or array starts with no members at all and more than one thing is
// added to it in the same edit set -- are resolved by processing edits in the
// REVERSE of the order the caller listed them, which is what makes same-offset
// edits land in the order the caller intended: each edit processed after
// another at the same offset is spliced in immediately before it, so the
// first-listed edit is processed last and ends up first in the result.
func Insert(data []byte, edits []Insertion) []byte {
	if len(edits) == 0 {
		return data
	}

	ordered := slices.Clone(edits)
	slices.Reverse(ordered)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].At > ordered[j].At })

	out := data
	for _, e := range ordered {
		next := make([]byte, 0, len(out)+len(e.Text))
		next = append(next, out[:e.At]...)
		next = append(next, e.Text...)
		next = append(next, out[e.At:]...)
		out = next
	}
	return out
}

// A Span is a byte range [From, To) of data to delete. Deletion is the exact
// inverse of an Insertion carrying the same range's bytes as its Text: Insert
// followed by Delete of the inserted range, or the reverse, restores the
// original bytes exactly.
type Span struct {
	From int
	To   int
}

// Delete removes every span from data and returns the result, leaving data
// itself untouched.
//
// Positions are computed against data's original coordinates; Delete applies
// them back-to-front for the same reason Insert does.
func Delete(data []byte, spans []Span) []byte {
	if len(spans) == 0 {
		return data
	}

	ordered := slices.Clone(spans)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].From > ordered[j].From })

	out := data
	for _, s := range ordered {
		next := make([]byte, 0, len(out)-(s.To-s.From))
		next = append(next, out[:s.From]...)
		next = append(next, out[s.To:]...)
		out = next
	}
	return out
}
