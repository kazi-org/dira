// Package config reads the handful of settings dira takes from
// .dira/config.toml.
//
// # It is not a TOML parser, and it must not become one
//
// dira's command path links no CLI framework and no config library on purpose
// (dec-0001, int-0002, and the allowlist in cmd/dira/build_test.go): every
// dependency is paid for on every invocation of a binary that runs in a hook, in
// the latency path of a waiting human. What this file needs from that file is
// three values, and three values do not justify a dependency.
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
	Parents []string
}

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
			cfg.Parents = append(cfg.Parents, key)
		}
	}

	if len(problems) > 0 {
		return cfg, fmt.Errorf(".dira/config.toml: %s", strings.Join(problems, "; "))
	}
	return cfg, nil
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
