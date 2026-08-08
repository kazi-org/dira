package skill

import (
	"bytes"
	"errors"
	"fmt"
)

// Installing the skill, and why this file names no path and opens no file.
//
// A skill nobody can install is not a capture tier, so `dira install-skill`
// exists (cmd/dira/install_skill.go) and this is the part of it that decides
// what to do. The decision is small — write it, leave it alone because it is
// already the right bytes, or leave it alone because somebody edited it — and
// the whole of the risk is in the writing, because the directory being written
// to is the operator's own Claude Code configuration.
//
// So the writing is not done here. This package imports no filesystem package
// at all: os, io/fs, path and path/filepath are all absent, and
// TestNoFilesystemImportsAboveTheBackend in internal/ledger keeps them absent.
// There is no path this code could build and no file it could open. Everything
// it touches arrives through the Root interface below, named relative to a
// directory the caller chose.
//
// That is the same move internal/sniff makes with stagedOnly: "this code cannot
// do the dangerous thing" is worth having as a property of the code rather than
// as a sentence in a doc comment, because the sentence is what a later edit
// walks straight past. cmd/dira backs Root with *os.Root, which refuses an
// escaping name at the operating system, so the confinement holds through a
// symlink as well as through a `..`.

// Path is where the skill goes inside a Claude Code configuration root:
// `~/.claude/skills/dira/SKILL.md`, of which this is the second half.
//
// It is slash-separated and relative on purpose. This is a name inside a Root,
// in the sense io/fs uses the word, not a path on anybody's disk — this package
// is never told where the root is, which is what makes "it cannot write
// somewhere else" a fact about the code instead of a claim about its callers.
//
// The directory name matches the skill's frontmatter `name`, which is Claude
// Code's convention for how a skill is discovered.
const Path = "skills/dira/SKILL.md"

// A Root is a directory the installer may write inside, and nothing else.
//
// Two methods, because two is what installing one file takes. Anything more —
// listing, removing, renaming — would be capability the installer does not need
// and a later edit could reach for.
type Root interface {
	// ReadFile returns the contents of name, and whether it is there at all.
	//
	// Existence is a separate return rather than an error to compare against
	// so that an implementation cannot quietly report "not there" for a file
	// it merely failed to read. The difference matters here: "absent" means
	// install it, and "unreadable" must never mean that.
	ReadFile(name string) (data []byte, exists bool, err error)

	// WriteFile creates or replaces name with data, creating whatever
	// directories name sits inside.
	WriteFile(name string, data []byte) error
}

// An Outcome is what one install did. The values are the words the command
// prints, so a person reading the terminal and a script reading stdout are
// looking at the same token.
type Outcome string

const (
	// Installed means the file was written.
	Installed Outcome = "INSTALLED"

	// Unchanged means the file was already byte-for-byte the document dira
	// ships, so nothing was written at all.
	//
	// It is distinct from Refused, and the distinction is the point: an
	// installer that refused every second run would protect an operator's
	// edits and would also be indistinguishable from one that had simply
	// stopped working.
	Unchanged Outcome = "UNCHANGED"

	// Refused means the file is there, is not what dira ships, and was left
	// exactly as it was because no force was asked for.
	Refused Outcome = "REFUSED"
)

// ErrNoRoot is returned when Install is handed no directory to install into.
var ErrNoRoot = errors.New("skill: no root to install into")

// ErrEmpty is returned when Install is handed a document with nothing in it.
//
// An empty skill installs, exits 0 and captures nothing for the rest of the
// machine's life — the silently-absent capability dec-0023 names as worse than
// a missing one. It costs one comparison to refuse it here.
var ErrEmpty = errors.New("skill: refusing to install an empty document")

// Install writes content to Path inside root.
//
// force replaces a file that is there and is not content. Without it such a
// file is left alone and Refused is returned, so the caller can name the file
// rather than report a success that overwrote somebody's work.
//
// The already-correct case is checked before the modified case, which is what
// makes Unchanged meaningful: a re-install of an untouched file is a no-op and
// says so, and only a file that actually differs is ever refused.
func Install(root Root, content []byte, force bool) (Outcome, error) {
	if root == nil {
		return "", ErrNoRoot
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return "", fmt.Errorf("%w as %s", ErrEmpty, Path)
	}

	current, exists, err := root.ReadFile(Path)
	if err != nil {
		return "", fmt.Errorf("skill: reading %s: %w", Path, err)
	}
	switch {
	case exists && bytes.Equal(current, content):
		return Unchanged, nil
	case exists && !force:
		return Refused, nil
	}

	if err := root.WriteFile(Path, content); err != nil {
		return "", fmt.Errorf("skill: writing %s: %w", Path, err)
	}
	return Installed, nil
}
