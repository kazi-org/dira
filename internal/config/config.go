// Package config reads the handful of settings dira takes from
// .dira/config.toml.
//
// # It is not a TOML parser, and it must not become one
//
// dira's command path links no CLI framework and no config library on purpose
// (dec-0001, int-0002, and the allowlist in cmd/dira/build_test.go): every
// dependency is paid for on every invocation of a binary that runs in a hook, in
// the latency path of a waiting human. What this file needs from that file is a
// handful of scalars and one shape — the inline table each [parents] entry is
// written as — and that does not justify a dependency.
//
// So this reads what it needs and ignores everything else, including syntax it
// does not understand. A key it does not read cannot be mis-parsed, and a
// section it does not know about — [kazi], [mirror], anything a later epic adds
// — is skipped rather than rejected. The cost is honest and stated: this will
// not catch a typo in a key nobody reads yet, and a value it cannot make sense
// of is reported rather than guessed at.
//
// # Absence is not permission
//
// A missing file, an unreadable one, or one with no [brief] section all yield
// cst-0001's ceiling, not an unlimited one. The constitutional cap is the
// default; the config file can only lower it or restate it.
package config

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// A Config is the part of .dira/config.toml dira reads today.
type Config struct {
	// Name is [ledger].name — what this ledger is called when something
	// refers to it. It is the brief's heading.
	Name string

	// Tier is [ledger].tier: person, workspace or repo.
	Tier string

	// MaxTokens is [brief].max_tokens, the ceiling cst-0001 puts on the
	// session brief. Zero means the file did not set one, and the caller
	// applies the default rather than treating it as no ceiling.
	MaxTokens int

	// Parents are the namespaces declared under [parents], in the order the
	// file declares them.
	//
	// Only live declarations count. .dira/config.toml ships with commented-out
	// examples, and a reader that counted them would report parent ledgers
	// nobody configured — the same rule scripts/privacy-lint.py applies for
	// the same reason.
	//
	// This is the name-only view, kept because the brief renderer wants a list
	// of names to print and nothing more. It is derived from ParentDecls at the
	// end of Parse rather than accumulated beside it, so the two cannot drift:
	// same length, same order, always.
	Parents []string

	// ParentDecls are those same declarations read in full — what each
	// [parents] entry actually says about the ledger it names.
	//
	// A parent is a declaration, not a name: dec-0011 puts `visibility` on it,
	// and the committed shape carries `path` and `ref`. Reading only the key
	// would leave a caller unable to tell a parent it may read from one it may
	// not, which is the distinction the whole boundary rests on.
	ParentDecls []Parent
}

// Visibility values dira reads on a [parents] declaration. Anything else is
// reported rather than interpreted; see Parent.Visibility.
const (
	// VisibilityPrivate marks a parent that may not be readable from here.
	// dec-0011: such a parent resolves as *withheld* rather than as an orphan,
	// and its label must not ship.
	VisibilityPrivate = "private"

	// VisibilityPublic is the explicit form of the default.
	VisibilityPublic = "public"
)

// A Parent is one live declaration under [parents].
//
// The key is the namespace, not the parent's own [ledger].name. Those two can
// disagree, and dec-0011 makes the declared key the thing a namespaced ref
// resolves through — scripts/privacy-lint.py P3 resolves committed refs the
// same way.
type Parent struct {
	// Name is the key the declaration is written under, and the namespace a
	// ref like `me:cst-0002` resolves through.
	Name string

	// Path is `path`: where the parent ledger sits, relative to the .dira
	// directory of the ledger declaring it. Empty when the declaration gives
	// none — a parent can be declared without being locatable, which is
	// exactly the shape a private parent takes in a public clone.
	Path string

	// Ref is `ref`: the revision of the parent this ledger is written against.
	// Empty when unstated.
	Ref string

	// Visibility is `visibility`: VisibilityPrivate, VisibilityPublic, or
	// empty when unstated (which reads as public).
	//
	// A value dira cannot read is reported *and* falls closed to
	// VisibilityPrivate. That is not a guess about what the author meant; it
	// is the only direction that cannot leak. Reading an unrecognised
	// visibility as public would let one typo publish a label dec-0011 says
	// must never ship — the failure `scripts/privacy-lint.py` P2 exists to
	// catch on committed bytes, arriving instead at run time.
	Visibility string

	// Label is `label`, a human-facing name for the parent.
	//
	// dec-0011: a private parent omits it precisely so the name itself never
	// ships. Nothing in this package ever puts a declaration's value into an
	// error message, because a label reaching stderr is that same leak by
	// another route.
	Label string
}

// Private reports whether this parent is declared as one that may not be
// readable from here.
func (p Parent) Private() bool { return p.Visibility == VisibilityPrivate }

// Parse reads config from the bytes of a config.toml.
//
// It always returns a usable Config. The error is a report about a value that
// was written for a key dira reads and could not be made sense of — a
// max_tokens that is not a number, say. A caller shows it and carries on: a
// brief that refused to render because of a typo in a config file would be a
// session with no orientation at all, which is worse than a session oriented by
// the default ceiling.
func Parse(data []byte) (Config, error) {
	var cfg Config
	var problems []string

	section := ""
	for n, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(stripComment(value))

		switch {
		case section == "ledger" && key == "name":
			cfg.Name = unquote(value)
		case section == "ledger" && key == "tier":
			cfg.Tier = unquote(value)
		case section == "brief" && key == "max_tokens":
			tokens, err := strconv.Atoi(unquote(value))
			switch {
			case err != nil:
				problems = append(problems, fmt.Sprintf("line %d: brief.max_tokens is %q, which is not a number", n+1, value))
			case tokens <= 0:
				problems = append(problems, fmt.Sprintf("line %d: brief.max_tokens is %d; a ceiling has to be positive", n+1, tokens))
			default:
				cfg.MaxTokens = tokens
			}
		case section == "parents":
			parent, reports := parseParent(key, value, n+1)
			problems = append(problems, reports...)
			if slices.ContainsFunc(cfg.ParentDecls, func(p Parent) bool { return p.Name == parent.Name }) {
				// Two declarations of one namespace and no rule for which
				// wins. Merging them would be a guess, and last-one-wins is
				// the guess that can quietly drop a visibility.
				problems = append(problems, fmt.Sprintf("line %d: parents.%s is declared twice; dira reads the first declaration and ignores this one", n+1, parent.Name))
				continue
			}
			cfg.ParentDecls = append(cfg.ParentDecls, parent)
		}
	}

	for _, parent := range cfg.ParentDecls {
		cfg.Parents = append(cfg.Parents, parent.Name)
	}

	if len(problems) > 0 {
		return cfg, fmt.Errorf(".dira/config.toml: %s", strings.Join(problems, "; "))
	}
	return cfg, nil
}

// parseParent reads one `name = { … }` line from [parents].
//
// It returns a declaration whatever happens: a namespace that is written down
// is declared even when the rest of the line is unreadable, because dropping it
// would turn every ref through that namespace into an undeclared-namespace
// error (dec-0011) — punishing the refs for a typo in the config.
//
// No report it produces contains any part of the value. The values on this line
// include `label`, which dec-0011 says must not ship for a private parent, and
// an error message is an output like any other. Naming the key and the line is
// enough to fix the file, and it cannot leak.
func parseParent(name, value string, line int) (Parent, []string) {
	parent := Parent{Name: name}

	body, ok := inlineTable(value)
	if !ok {
		return parent, []string{fmt.Sprintf("line %d: parents.%s is not written as a { … } declaration, so dira read nothing from it", line, name)}
	}

	var problems []string
	for _, field := range splitFields(body) {
		if strings.TrimSpace(field) == "" {
			continue
		}
		key, fieldValue, ok := strings.Cut(field, "=")
		if !ok {
			problems = append(problems, fmt.Sprintf("line %d: parents.%s has a field that is not a key = value pair", line, name))
			continue
		}
		key = strings.TrimSpace(key)
		fieldValue = unquote(fieldValue)

		switch key {
		case "path":
			parent.Path = fieldValue
		case "ref":
			parent.Ref = fieldValue
		case "label":
			parent.Label = fieldValue
		case "visibility":
			switch fieldValue {
			case VisibilityPrivate, VisibilityPublic:
				parent.Visibility = fieldValue
			default:
				parent.Visibility = VisibilityPrivate
				problems = append(problems, fmt.Sprintf("line %d: parents.%s has a visibility dira does not read; it reads %q and %q, and treats anything else as %q so a typo cannot publish what it hides", line, name, VisibilityPrivate, VisibilityPublic, VisibilityPrivate))
			}
		}
		// A key this reader does not know is skipped, not rejected — the same
		// bargain the rest of the file makes with sections it does not read.
	}
	return parent, problems
}

// inlineTable returns the contents of a `{ … }` value.
func inlineTable(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '{' || value[len(value)-1] != '}' {
		return "", false
	}
	return value[1 : len(value)-1], true
}

// splitFields splits an inline table's body on its commas.
//
// Only outside quotes, for the reason stripComment is careful about `#`: a
// label reading "sire, the workspace" must not become two malformed fields, and
// a value silently cut in half is the failure mode a hand-rolled reader is
// actually at risk of.
func splitFields(body string) []string {
	var fields []string
	quoted := false
	start := 0
	for i, r := range body {
		switch r {
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				fields = append(fields, body[start:i])
				start = i + 1
			}
		}
	}
	return append(fields, body[start:])
}

// stripComment drops a trailing `# ...` from a value.
//
// Only outside quotes: `name = "dira # 1"` is a name, not a comment. This is the
// one piece of TOML this file is careful about, because getting it wrong turns a
// value into a shorter value rather than into an error, and a silently truncated
// value is the failure mode a hand-rolled reader is actually at risk of.
func stripComment(value string) string {
	quoted := false
	for i, r := range value {
		switch r {
		case '"':
			quoted = !quoted
		case '#':
			if !quoted {
				return value[:i]
			}
		}
	}
	return value
}

// unquote removes the surrounding quotes of a string value, leaving anything
// else alone.
func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	return value
}
