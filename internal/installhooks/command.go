package installhooks

// The command-string parser T7 needs: takes one installed command string --
// exactly the value dira writes into a settings file's "command" field -- and
// returns its verb and flag names, so a caller (cmd/dira's own test) can ask
// the real binary whether it accepts them. This package still names no path
// and starts nothing; it only tokenises a string.

import (
	"errors"
	"fmt"
	"strings"
)

// shellGuard is the suffix every command dira installs carries -- the part
// that makes a dira failure not a session failure (2>/dev/null discards
// stderr, || true neutralises the exit status).
const shellGuard = "2>/dev/null || true"

// ErrNoGuard marks a command missing the shell guard. A command without it is
// not a string this installer wrote.
var ErrNoGuard = errors.New(`installhooks: command does not end with the shell guard "2>/dev/null || true"`)

// ErrNotDira marks a command whose first token, after the guard is stripped,
// is not "dira".
var ErrNotDira = errors.New(`installhooks: command does not begin with "dira"`)

// ErrUnparseable marks a command this tokeniser cannot make sense of -- an
// unbalanced quote, or nothing at all after "dira". An unbalanced quote is a
// broken command whoever reads it, so it is reported rather than guessed at.
var ErrUnparseable = errors.New("installhooks: command could not be tokenised")

// A Flag is one flag token from a parsed command.
type Flag struct {
	// Name is the bare flag name a caller probes with: "stage" for "--stage".
	Name string
	// Text is exactly what appeared on the command line: "--stage" or
	// "--hook=Stop".
	Text string
}

// A ParsedCommand is what ParseHookCommand found in one installed command
// string: the verb dira was invoked with, and every flag after it, in order.
type ParsedCommand struct {
	Verb  string
	Flags []Flag
}

// ParseHookCommand tokenises one installed command string.
//
// It asserts the shell guard is present and strips it, then requires the
// remainder to begin with "dira" followed by a verb. It reports anything it
// cannot tokenise rather than guessing, and it returns an error -- never an
// empty ParsedCommand, which would look like a real command that simply
// carries no flags -- for a string that does not begin with "dira".
func ParseHookCommand(command string) (ParsedCommand, error) {
	if !strings.HasSuffix(command, shellGuard) {
		return ParsedCommand{}, fmt.Errorf("%w: %q", ErrNoGuard, command)
	}
	body := strings.TrimSpace(strings.TrimSuffix(command, shellGuard))

	tokens, err := tokenise(body)
	if err != nil {
		return ParsedCommand{}, fmt.Errorf("%w: %q: %v", ErrUnparseable, command, err)
	}
	if len(tokens) == 0 || tokens[0] != "dira" {
		return ParsedCommand{}, fmt.Errorf("%w: %q", ErrNotDira, command)
	}
	if len(tokens) < 2 {
		return ParsedCommand{}, fmt.Errorf("%w: %q names no verb after \"dira\"", ErrUnparseable, command)
	}

	parsed := ParsedCommand{Verb: tokens[1]}
	for _, tok := range tokens[2:] {
		if !strings.HasPrefix(tok, "--") {
			continue // a positional argument, not a flag
		}
		name := strings.TrimPrefix(tok, "--")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		parsed.Flags = append(parsed.Flags, Flag{Name: name, Text: tok})
	}
	return parsed, nil
}

// tokenise splits body on whitespace, honouring double-quoted substrings so a
// flag value containing a space is not split in two. An unbalanced quote is
// reported rather than silently closed at end of string.
func tokenise(body string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inQuote := false
	haveToken := false

	flush := func() {
		if haveToken {
			tokens = append(tokens, cur.String())
			cur.Reset()
			haveToken = false
		}
	}

	for _, r := range body {
		switch {
		case r == '"':
			inQuote = !inQuote
			haveToken = true
		case r == ' ' && !inQuote:
			flush()
		default:
			cur.WriteRune(r)
			haveToken = true
		}
	}
	if inQuote {
		return nil, errors.New("unbalanced quote")
	}
	flush()
	return tokens, nil
}
