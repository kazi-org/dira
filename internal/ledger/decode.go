package ledger

import (
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	// Not github.com/kazi-org/dira/schema, which is where these two names
	// were reached for until E1-L6: that package compiles entry.schema.json
	// with santhosh-tekuri/jsonschema, and importing it for a string split
	// put the validator's package init — milliseconds and ~21,700
	// allocations — in front of main on every dira run. The codec validates
	// against Entry.Validate, not against the JSON Schema document; the
	// document is still the contract, and schema_test.go is what holds the
	// two in agreement.
	"github.com/kazi-org/dira/internal/frontmatter"
)

// Decode reads one entry file — frontmatter and body — into an Entry.
//
// It walks the yaml.Node tree rather than unmarshalling into the struct, for
// three reasons that are all requirements rather than preferences:
//
//   - Each scalar's raw source text is available, so a `!!timestamp`-tagged
//     value is read as the string it was written as and never becomes a
//     time.Time. See timestamp.go.
//   - An unknown key can be rejected by name and line. entry.schema.json is
//     `additionalProperties: false`, and a codec that silently dropped an
//     unknown key would delete data on the first round-trip through a file
//     written by a newer dira.
//   - The presentation of each scalar can be recorded, so re-serialising an
//     entry nobody edited reproduces the file byte for byte and editing one
//     field produces a one-line diff. dec-0002 sells reviewable history; a
//     codec that reflowed every paragraph on every write would not deliver it.
//
// The returned entry is validated. A file that parses but violates the contract
// is ledger rot, and returning it would push the problem to every caller.
func Decode(data []byte) (*Entry, error) {
	e, err := decodeEntry(data)
	if err != nil {
		return nil, err
	}
	if err := e.Validate(); err != nil {
		return nil, fmt.Errorf("entry %s: %w", e.ID, err)
	}
	return e, nil
}

// decodeEntry is Decode without the validation step. It exists so DecodeDraft
// can apply the draft rules to the same parse rather than to a second one, and
// nothing else may call it: an unvalidated entry is the ledger rot Decode exists
// to keep out, and handing one to a caller pushes the problem downstream.
func decodeEntry(data []byte) (*Entry, error) {
	front, body, err := frontmatter.Split(data)
	if err != nil {
		return nil, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(front, &doc); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return nil, fmt.Errorf("%w: frontmatter is empty", frontmatter.ErrMissing)
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("frontmatter is %s, want a mapping", nodeKind(root))
	}

	d := &decoder{src: strings.Split(string(front), "\n"), memo: styleMemo{}}
	e := &Entry{Body: string(body), style: d.memo}

	if err := d.mapping(root, "", map[string]func(*yaml.Node) error{
		"id":           func(n *yaml.Node) error { return d.str(n, "id", &e.ID) },
		"kind":         func(n *yaml.Node) error { return d.enum(n, "kind", (*string)(&e.Kind)) },
		"title":        func(n *yaml.Node) error { return d.str(n, "title", &e.Title) },
		"state":        func(n *yaml.Node) error { return d.enum(n, "state", (*string)(&e.State)) },
		"created":      func(n *yaml.Node) error { return d.str(n, "created", &e.Created) },
		"updated":      func(n *yaml.Node) error { return d.str(n, "updated", &e.Updated) },
		"confirmed_by": func(n *yaml.Node) error { return d.str(n, "confirmed_by", &e.ConfirmedBy) },
		"adr":          func(n *yaml.Node) error { return d.str(n, "adr", &e.ADR) },
		"private":      func(n *yaml.Node) error { return d.bool(n, "private", &e.Private) },
		"tags":         func(n *yaml.Node) error { return d.tags(n, &e.Tags) },
		"edges":        func(n *yaml.Node) error { return d.edges(n, &e.Edges) },
		"alternatives": func(n *yaml.Node) error { return d.alternatives(n, &e.Alternatives) },
		"source":       func(n *yaml.Node) error { return d.source(n, &e.Source) },
	}); err != nil {
		return nil, err
	}
	return e, nil
}

// DecodeStored is Decode for a Store implementation: it stamps the entry with
// the backend's opaque version, so a later reader can tell whether what it
// cached is still what is stored. See EntryInfo.Version.
//
// Nothing outside a backend should call this. An entry carrying a version it did
// not come from is how a derived cache is talked into trusting stale data.
func DecodeStored(data []byte, version string) (*Entry, error) {
	e, err := Decode(data)
	if err != nil {
		return nil, err
	}
	e.version = version
	return e, nil
}

// decoder carries the frontmatter source lines, which are what block scalars
// are recovered from, and the style memo being filled in.
type decoder struct {
	src  []string
	memo styleMemo
}

// mapping walks a mapping node, dispatching each known key to its field
// handler and rejecting anything else. path is "" at the top level and the
// dotted field path inside a nested mapping, so an error names where it is.
func (d *decoder) mapping(n *yaml.Node, path string, fields map[string]func(*yaml.Node) error) error {
	seen := make(map[string]bool, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: mapping key is %s, want a string", key.Line, nodeKind(key))
		}
		name := key.Value
		if seen[name] {
			return fmt.Errorf("line %d: %s is set twice", key.Line, qualify(path, name))
		}
		seen[name] = true

		handle, ok := fields[name]
		if !ok {
			// entry.schema.json is additionalProperties: false. This is
			// the error, not a silent drop: dropping the key would
			// delete it on the next write.
			return fmt.Errorf("line %d: unknown field %q (entry.schema.json allows %s)",
				key.Line, qualify(path, name), strings.Join(sortedKeys(fields), ", "))
		}
		if err := handle(value); err != nil {
			return err
		}
	}
	return nil
}

// scalar returns a scalar node's value as the text it was written as, and
// records how it was written.
//
// n.Value is deliberately the only source of the value. For an unquoted RFC3339
// timestamp yaml.v3 tags the node !!timestamp, and decoding that node into any
// Go value would produce a time.Time; reading n.Value observes the tag and
// ignores it.
func (d *decoder) scalar(n *yaml.Node, field string) (string, error) {
	if n.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("line %d: %s is %s, want a scalar", n.Line, field, nodeKind(n))
	}
	if n.Tag == "!!null" {
		return "", fmt.Errorf("line %d: %s is null; omit the field instead", n.Line, field)
	}

	value := n.Value
	style := scalarStyle{style: n.Style}

	if n.Style&(yaml.FoldedStyle|yaml.LiteralStyle) != 0 {
		// Clip chomping — the default and the only form in this ledger —
		// leaves exactly one trailing newline on the value. The Entry
		// field holds the logical string without it; the block is
		// re-emitted from the recorded source, so nothing is lost.
		value = strings.TrimSuffix(value, "\n")
		block, err := d.block(n, field, value)
		if err != nil {
			return "", err
		}
		style.block = block
	}

	d.memo.record(value, style)
	return value, nil
}

// block recovers a block scalar's source verbatim. yaml.v3 hands back the
// folded value with its line breaks already collapsed to spaces, and re-folding
// it is not reversible: the ledger's own entries are hand-wrapped, and no single
// greedy width reproduces more than 19 of the 46 folded scalars in it. So the
// original lines are kept.
//
// The lines are dedented by the block's own content indent, so the same text
// re-emits correctly if the entry is written at a different nesting depth.
func (d *decoder) block(n *yaml.Node, field, value string) (*blockStyle, error) {
	// n.Line is 1-based and points at the line carrying the block header;
	// n.Column is 1-based and points at the header indicator itself.
	header := n.Line - 1
	if header < 0 || header >= len(d.src) {
		return nil, fmt.Errorf("line %d: %s: block scalar is outside the frontmatter", n.Line, field)
	}
	line := d.src[header]
	if n.Column-1 > len(line) {
		return nil, fmt.Errorf("line %d: %s: block header runs past the end of the line", n.Line, field)
	}
	indicator := strings.TrimRight(line[n.Column-1:], " \t")

	var lines []string
	contentIndent := -1
	for i := header + 1; i < len(d.src); i++ {
		text := d.src[i]
		if strings.TrimSpace(text) == "" {
			// A blank line only belongs to the block if the block
			// continues past it; trailing blanks are trimmed below.
			lines = append(lines, "")
			continue
		}
		indent := len(text) - len(strings.TrimLeft(text, " "))
		if contentIndent < 0 {
			contentIndent = indent
		}
		if indent < contentIndent {
			break
		}
		lines = append(lines, text[contentIndent:])
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("line %d: %s: block scalar has no content", n.Line, field)
	}

	// Only keep the capture if re-emitting it provably reproduces the value.
	//
	// The capture is a slice of source text, and the rules for turning block
	// source back into a string have corners this does not model: an
	// explicit indentation indicator, a blank line inside a folded block, a
	// more-indented line that keeps its own break. Rather than implement
	// them and be subtly wrong, the reconstruction is checked against what
	// the YAML parser actually produced, and a mismatch falls back to
	// canonical emission — which reflows the text but never changes it.
	if reconstruct(indicator, lines) != value {
		return nil, nil
	}
	return &blockStyle{indicator: indicator, lines: lines}, nil
}

// reconstruct renders a captured block back to the string it stands for, for
// the ordinary cases: a literal block joins its lines with newlines, a folded
// block joins them with spaces. It returns "" — which never matches a real
// value, since a block scalar with no content is rejected above — for anything
// outside those cases.
func reconstruct(indicator string, lines []string) string {
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			return ""
		}
	}
	switch {
	case strings.HasPrefix(indicator, "|"):
		return strings.Join(lines, "\n")
	case strings.HasPrefix(indicator, ">"):
		return strings.Join(lines, " ")
	default:
		return ""
	}
}

func (d *decoder) str(n *yaml.Node, field string, dst *string) error {
	value, err := d.scalar(n, field)
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

// enum is str for a field whose value is one of a closed set. The set itself is
// checked by Entry.Validate, which can name the allowed values; this exists so
// the intent is legible at the call site.
func (d *decoder) enum(n *yaml.Node, field string, dst *string) error {
	return d.str(n, field, dst)
}

func (d *decoder) bool(n *yaml.Node, field string, dst *bool) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: %s is %s, want true or false", n.Line, field, nodeKind(n))
	}
	switch n.Value {
	case "true":
		*dst = true
	case "false":
		*dst = false
	default:
		return fmt.Errorf("line %d: %s is %q, want true or false", n.Line, field, n.Value)
	}
	d.memo.record(n.Value, scalarStyle{style: n.Style})
	return nil
}

func (d *decoder) tags(n *yaml.Node, dst *[]string) error {
	if n.Kind != yaml.SequenceNode {
		return fmt.Errorf("line %d: tags is %s, want a list", n.Line, nodeKind(n))
	}
	tags := make([]string, 0, len(n.Content))
	for i, item := range n.Content {
		if item.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: tags[%d] is %s, want a string", item.Line, i, nodeKind(item))
		}
		tags = append(tags, item.Value)
	}
	*dst = tags
	return nil
}

func (d *decoder) edges(n *yaml.Node, dst *[]Edge) error {
	if n.Kind != yaml.SequenceNode {
		return fmt.Errorf("line %d: edges is %s, want a list", n.Line, nodeKind(n))
	}
	edges := make([]Edge, 0, len(n.Content))
	for i, item := range n.Content {
		if item.Kind != yaml.MappingNode {
			return fmt.Errorf("line %d: edges[%d] is %s, want a mapping", item.Line, i, nodeKind(item))
		}
		var edge Edge
		path := fmt.Sprintf("edges[%d]", i)
		err := d.mapping(item, path, map[string]func(*yaml.Node) error{
			"type": func(v *yaml.Node) error { return d.enum(v, path+".type", (*string)(&edge.Type)) },
			"to":   func(v *yaml.Node) error { return d.str(v, path+".to", &edge.To) },
			"note": func(v *yaml.Node) error { return d.str(v, path+".note", &edge.Note) },
		})
		if err != nil {
			return err
		}
		edges = append(edges, edge)
	}
	*dst = edges
	return nil
}

func (d *decoder) alternatives(n *yaml.Node, dst *[]Alternative) error {
	if n.Kind != yaml.SequenceNode {
		return fmt.Errorf("line %d: alternatives is %s, want a list", n.Line, nodeKind(n))
	}
	alts := make([]Alternative, 0, len(n.Content))
	for i, item := range n.Content {
		if item.Kind != yaml.MappingNode {
			return fmt.Errorf("line %d: alternatives[%d] is %s, want a mapping", item.Line, i, nodeKind(item))
		}
		var alt Alternative
		path := fmt.Sprintf("alternatives[%d]", i)
		err := d.mapping(item, path, map[string]func(*yaml.Node) error{
			"option":     func(v *yaml.Node) error { return d.str(v, path+".option", &alt.Option) },
			"why_not":    func(v *yaml.Node) error { return d.str(v, path+".why_not", &alt.WhyNot) },
			"revisit_if": func(v *yaml.Node) error { return d.str(v, path+".revisit_if", &alt.RevisitIf) },
		})
		if err != nil {
			return err
		}
		alts = append(alts, alt)
	}
	*dst = alts
	return nil
}

func (d *decoder) source(n *yaml.Node, dst **Source) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: source is %s, want a mapping", n.Line, nodeKind(n))
	}
	var s Source
	err := d.mapping(n, "source", map[string]func(*yaml.Node) error{
		"hook":    func(v *yaml.Node) error { return d.enum(v, "source.hook", (*string)(&s.Hook)) },
		"session": func(v *yaml.Node) error { return d.str(v, "source.session", &s.Session) },
		"excerpt": func(v *yaml.Node) error { return d.str(v, "source.excerpt", &s.Excerpt) },
		"tier":    func(v *yaml.Node) error { return d.enum(v, "source.tier", (*string)(&s.Tier)) },
	})
	if err != nil {
		return err
	}
	*dst = &s
	return nil
}

func qualify(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func nodeKind(n *yaml.Node) string {
	switch n.Kind {
	case yaml.DocumentNode:
		return "a document"
	case yaml.SequenceNode:
		return "a list"
	case yaml.MappingNode:
		return "a mapping"
	case yaml.ScalarNode:
		return "a scalar"
	case yaml.AliasNode:
		return "an alias"
	default:
		return "empty"
	}
}

func sortedKeys(m map[string]func(*yaml.Node) error) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
