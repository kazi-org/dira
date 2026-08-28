package ledger

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// wrapWidth is the column a folded scalar dira composes itself is wrapped at.
// It is the width that best matches the hand-written entries in this repo, and
// it governs only new text: text dira read from a file is re-emitted from the
// style memo exactly as it was found. See style.go.
const wrapWidth = 82

// Encode serialises an entry to its on-disk file: `---`, the frontmatter, `---`,
// then the markdown body byte for byte.
//
// It validates first. A codec that could write a file violating
// entry.schema.json is how ledger rot gets committed, and the ledger is the one
// thing in dira that has to be trustworthy.
//
// The key order below is not arbitrary and is not remembered per file. Every one
// of the six key orderings in this repo's own ledger is a subsequence of it, so
// writing in this order reproduces all 26 hand-written entries without the codec
// carrying an ordering per file. adr, private, and applies_when appear in no
// entry yet; they sit after confirmed_by with the other disposition metadata.
func Encode(e *Entry) ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, fmt.Errorf("refusing to encode entry %s: %w", e.ID, err)
	}

	w := &writer{memo: e.style}
	w.buf.WriteString("---\n")

	w.field(0, "id", e.ID, false)
	w.field(0, "kind", string(e.Kind), false)
	w.field(0, "title", e.Title, false)
	w.field(0, "state", string(e.State), false)
	w.timestamp(0, "created", e.Created)
	if e.Updated != "" {
		w.timestamp(0, "updated", e.Updated)
	}
	if len(e.Tags) > 0 {
		w.tags(e.Tags)
	}
	if len(e.Edges) > 0 {
		w.buf.WriteString("edges:\n")
		for _, edge := range e.Edges {
			w.item(2, []field{
				{key: "type", value: string(edge.Type)},
				{key: "to", value: edge.To},
				{key: "note", value: edge.Note, prose: true, omitEmpty: true},
			})
		}
	}
	if len(e.Alternatives) > 0 {
		w.buf.WriteString("alternatives:\n")
		for _, alt := range e.Alternatives {
			w.item(2, []field{
				{key: "option", value: alt.Option},
				{key: "why_not", value: alt.WhyNot, prose: true},
				{key: "revisit_if", value: alt.RevisitIf, prose: true, omitEmpty: true},
			})
		}
	}
	if e.Source != nil {
		w.buf.WriteString("source:\n")
		w.field(2, "hook", string(e.Source.Hook), false)
		if e.Source.Session != "" {
			w.field(2, "session", e.Source.Session, false)
		}
		if e.Source.Excerpt != "" {
			w.field(2, "excerpt", e.Source.Excerpt, true)
		}
		if e.Source.Tier != "" {
			w.field(2, "tier", string(e.Source.Tier), false)
		}
	}
	if e.ConfirmedBy != "" {
		w.field(0, "confirmed_by", e.ConfirmedBy, false)
	}
	if e.ADR != "" {
		w.field(0, "adr", e.ADR, false)
	}
	if e.Private {
		w.buf.WriteString("private: true\n")
	}
	if e.AppliesWhen != nil {
		w.buf.WriteString("applies_when:\n")
		w.appliesWhen(e.AppliesWhen)
	}

	w.buf.WriteString("---\n")
	w.buf.WriteString(e.Body)
	return []byte(w.buf.String()), nil
}

// field is one key/value pair queued for emission inside a sequence item.
type field struct {
	key       string
	value     string
	prose     bool
	omitEmpty bool
}

type writer struct {
	buf  strings.Builder
	memo styleMemo
}

// field writes `<indent>key: value` at the top level or inside a mapping.
func (w *writer) field(indent int, key, value string, prose bool) {
	w.buf.WriteString(strings.Repeat(" ", indent))
	w.keyValue(indent, key, value, prose)
}

// item writes a block-sequence item: the first field carries the `- `, the rest
// line up under it.
func (w *writer) item(indent int, fields []field) {
	first := true
	for _, f := range fields {
		if f.omitEmpty && f.value == "" {
			continue
		}
		if first {
			w.buf.WriteString(strings.Repeat(" ", indent))
			w.buf.WriteString("- ")
			first = false
		} else {
			w.buf.WriteString(strings.Repeat(" ", indent+2))
		}
		w.keyValue(indent+2, f.key, f.value, f.prose)
	}
}

// keyValue writes `key: <scalar>` and the newline that ends it. keyIndent is the
// column the key starts at, which is what a block scalar's content is indented
// two further from.
func (w *writer) keyValue(keyIndent int, key, value string, prose bool) {
	w.buf.WriteString(key)
	w.buf.WriteString(":")

	if style, ok := w.memo.lookup(value); ok {
		if style.block != nil {
			w.buf.WriteString(" ")
			w.buf.WriteString(style.block.indicator)
			w.buf.WriteString("\n")
			w.writeBlockLines(keyIndent+2, style.block.lines)
			return
		}
		switch style.style {
		case yaml.DoubleQuotedStyle:
			w.buf.WriteString(" " + doubleQuote(value) + "\n")
			return
		case yaml.SingleQuotedStyle:
			w.buf.WriteString(" " + singleQuote(value) + "\n")
			return
		case 0:
			// The value came from a plain scalar at this depth in a
			// file that parsed, so re-emitting it plain is safe by
			// construction — no heuristic needed or wanted.
			w.buf.WriteString(" " + value + "\n")
			return
		}
	}

	w.buf.WriteString(" ")
	w.writeCanonical(keyIndent, value, prose)
}

// writeCanonical emits a value dira composed itself, choosing the presentation
// rather than recalling one.
func (w *writer) writeCanonical(keyIndent int, value string, prose bool) {
	switch {
	case literalSafe(value):
		// Line breaks are content. A folded block would collapse them.
		w.buf.WriteString("|\n")
		w.writeBlockLines(keyIndent+2, strings.Split(value, "\n"))

	case strings.Contains(value, "\n"):
		// A value a block scalar cannot express exactly. Double quotes
		// can express any string, so correctness wins over prettiness.
		w.buf.WriteString(doubleQuote(value) + "\n")

	case prose && foldable(value) && keyIndent+len(value) > wrapWidth:
		w.buf.WriteString(">\n")
		w.writeBlockLines(keyIndent+2, wrap(value, wrapWidth-(keyIndent+2)))

	case plainSafe(value):
		w.buf.WriteString(value + "\n")

	default:
		w.buf.WriteString(doubleQuote(value) + "\n")
	}
}

func (w *writer) writeBlockLines(indent int, lines []string) {
	pad := strings.Repeat(" ", indent)
	for _, line := range lines {
		if line == "" {
			w.buf.WriteString("\n")
			continue
		}
		w.buf.WriteString(pad)
		w.buf.WriteString(line)
		w.buf.WriteString("\n")
	}
}

// appliesWhen writes the applies_when mapping's two lines. Its params value
// is a free-form map[string]any rather than a string scalar the style-memo
// machinery above already knows how to reproduce byte for byte, so this
// method delegates to yaml.v3's own encoder instead of stretching
// writeCanonical/keyValue to a type they were never asked to model. That
// means a hand-formatted params map does not round-trip its original
// layout, only its values -- the same tradeoff params (decode.go) documents
// on the read side, and an open question for review rather than something
// papered over.
func (w *writer) appliesWhen(a *AppliesWhen) {
	out, err := yaml.Marshal(struct {
		Action string         `yaml:"action"`
		Params map[string]any `yaml:"params,omitempty"`
	}{a.Action, a.Params})
	if err != nil {
		// Action is a plain string and Params came from a decoded YAML
		// mapping (or was built by a caller from JSON-safe values); yaml.v3
		// only fails to marshal types it cannot represent, so this path is
		// unreached in practice. Kept so a future Params value this codec
		// was not written for fails loudly on the entry it belongs to,
		// rather than silently dropping the clause.
		w.buf.WriteString(fmt.Sprintf("  action: %s\n", doubleQuote(a.Action)))
		return
	}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			w.buf.WriteString("\n")
			continue
		}
		w.buf.WriteString("  ")
		w.buf.WriteString(line)
		w.buf.WriteString("\n")
	}
}

// timestamp writes created or updated, always double-quoted.
//
// This is the write half of the rule in timestamp.go and it deliberately does
// not consult plainSafe or the style memo: an unquoted RFC3339 scalar is read
// back as a !!timestamp by every YAML 1.1 reader, including ours, and the
// published schema says these fields are strings. There is no input for which
// the right answer here is "unquoted".
func (w *writer) timestamp(indent int, key, value string) {
	w.buf.WriteString(strings.Repeat(" ", indent))
	w.buf.WriteString(key)
	w.buf.WriteString(": ")
	w.buf.WriteString(doubleQuote(value))
	w.buf.WriteString("\n")
}

// tags writes the tag list as a flow sequence, which is how every entry in this
// ledger writes it and the only sensible layout for a handful of short words.
// Tag syntax is constrained by the schema to `^[a-z0-9][a-z0-9-]*$`, and
// Entry.Validate has already checked it, so no tag can need quoting.
func (w *writer) tags(tags []string) {
	w.buf.WriteString("tags: [")
	w.buf.WriteString(strings.Join(tags, ", "))
	w.buf.WriteString("]\n")
}

// wrap greedily fills lines to width. Words longer than the width get their own
// line rather than being broken, which is what keeps a URL or a long identifier
// intact.
func wrap(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		line += " " + word
	}
	return append(lines, line)
}

// yamlIndicators are the characters that change meaning when they open a plain
// scalar. The check below is deliberately blunt — it rejects them anywhere at
// the start rather than reasoning about which are safe when not followed by a
// space — because the cost of a false negative is a quoted string that did not
// need quoting, and the cost of a false positive is a corrupt ledger file.
const yamlIndicators = "-?:,[]{}#&*!|>'\"%@`"

// nonStringPlain matches the plain scalars YAML 1.1 resolves to something other
// than a string: booleans, nulls, numbers in every base, the infinities, and
// anything date-shaped. Emitting one of these unquoted would change its type on
// the next read.
var nonStringPlain = regexp.MustCompile(`^(?i:` +
	`|~|null|true|false|yes|no|on|off|y|n` +
	`|[-+]?(0b[01_]+|0o?[0-7_]+|0x[0-9a-f_]+)` +
	`|[-+]?([0-9][0-9_]*)(\.[0-9_]*)?([e][-+]?[0-9]+)?` +
	`|[-+]?\.[0-9_]+([e][-+]?[0-9]+)?` +
	`|[-+]?\.(inf|nan)` +
	`|[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}.*` +
	`)$`)

// plainSafe reports whether value can be written as a plain scalar and read
// back as the identical string.
//
// It governs only text dira composed itself. Text read from a file carries its
// own recorded style, which is authoritative because the file parsed.
func plainSafe(value string) bool {
	if value == "" {
		return false
	}
	if strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	if strings.IndexByte(yamlIndicators, value[0]) >= 0 {
		return false
	}
	// A colon anywhere can start a mapping or a sexagesimal number
	// depending on what follows; a hash can start a comment. Neither is
	// worth a rule that has to be right about the context.
	if strings.ContainsAny(value, ":#") {
		return false
	}
	return !nonStringPlain.MatchString(value)
}

// literalSafe reports whether value can be written as a literal block and read
// back byte for byte.
//
// A literal block written with the default clip chomping always reads back
// ending in exactly one newline, and the codec's model value carries none, so a
// value that itself ends in a newline cannot be written this way — that was a
// real one-character data loss until a test went looking for it. A line that
// begins or ends with a space is excluded for the same kind of reason: the
// indentation of the first content line is what sets the block's indent, and a
// trailing space is invisible to the human who would have to preserve it.
func literalSafe(value string) bool {
	if !strings.Contains(value, "\n") || strings.HasSuffix(value, "\n") {
		return false
	}
	for _, line := range strings.Split(value, "\n") {
		if line != strings.TrimRight(line, " \t") || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			return false
		}
	}
	return true
}

// foldable reports whether value can be written as a folded block and read back
// unchanged.
//
// Folding is lossy in one specific way: reading a folded block joins its lines
// with a single space, so a run of two spaces, a tab, or a leading space on a
// line does not survive. Prose does not contain those; a value that does gets
// written on one line instead, where every byte is preserved. Getting this
// wrong would be a silent edit of the user's text on a write they did not ask
// for, which is worse than an ugly long line.
func foldable(value string) bool {
	if !plainSafe(value) {
		return false
	}
	return strings.Join(strings.Fields(value), " ") == value
}

// doubleQuote writes a YAML double-quoted scalar. Double quotes are the only
// YAML quoting that can express every string, which is why this is the fallback
// rather than single quotes.
func doubleQuote(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\x%02x`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// singleQuote writes a YAML single-quoted scalar, used only to reproduce a file
// that already used one. A single-quoted scalar cannot carry an escape, so a
// value containing anything unrepresentable falls back to double quotes.
func singleQuote(value string) string {
	if strings.ContainsAny(value, "\n\r\t") {
		return doubleQuote(value)
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
