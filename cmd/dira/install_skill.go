package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kazi-org/dira/internal/skill"
	"github.com/kazi-org/dira/skills"
)

// `dira install-skill` writes dira's capture tier 2 — skills/dira/SKILL.md —
// into a Claude Code configuration root, `~/.claude` by default.
//
// It exists because a skill nobody can install is not a tier. dec-0003 puts the
// semantic half of capture in the live session rather than in the binary, and
// the only thing that reaches the live session is a skill file on the machine;
// dec-0023 established that the handoff cannot ride PreCompact stdout, so the
// document the session reads at SessionStart is the whole delivery mechanism.
// Shipping it in the repository and leaving its installation to a copy command
// in a README would make the tier optional in practice.
//
// Three properties, each of them a deliberate choice:
//
// It is consent-first. No other dira command touches `~/.claude` — the hooks
// read a transcript and write inside `.dira/`, and nothing anywhere installs
// itself. This verb runs when a person types it, and mirrors
// Kazi.Teach.InstallSkill's contract for the same reason kazi has it.
//
// It never clobbers. A file that is there and is not the document dira ships is
// somebody's edit until proven otherwise, and is left exactly as it is, named,
// with `--force` offered. The already-correct case is reported as UNCHANGED
// rather than folded into the same refusal, because an installer that refused
// every second run would look identical to one that protects edits.
//
// It cannot write outside the root it was given. The root is opened as an
// *os.Root and every name below it is relative, so `..` and a symlink pointing
// out of the tree are both refused by the operating system rather than by
// careful string handling here. internal/skill, which decides what to do, has
// no filesystem import at all and could not name a path if it wanted to.
//
// Exit status is 0 whenever the command ran, including when it refused to
// overwrite an edited file. That refusal is the command working, not failing:
// exit 1 is dira being broken and exit 2 is the caller mistyping (L-0020), and
// a refusal is neither. It is reported on stderr with the file named and the
// remedy stated, so it cannot pass unnoticed.

// installSkillName is the verb. Named once because the command struct, the
// usage text and cmd/dira's tests all have to agree on it.
const installSkillName = "install-skill"

const installSkillSummary = "write dira's capture skill into ~/.claude for Claude Code to load"

// claudeDirName is the Claude Code configuration directory inside the user's
// home. It is a name inside a root rather than a path, which is what keeps this
// file clear of path/filepath (dec-0005's boundary check allows cmd/dira `os`
// and nothing else).
const claudeDirName = ".claude"

// installSkillCommand is this verb's registry entry. It is a function rather
// than a literal in newApp so that a test can register it against its own app
// without reaching into package-level state.
//
// Register it in newApp's slice in cmd/dira/main.go, after `reindex`:
//
//	{name: installSkillName, summary: installSkillSummary, run: runInstallSkill, usage: writeInstallSkillUsage},
//
// The help test iterates that registry rather than a literal list, so the entry
// is all it takes for `dira --help` to name the command and its summary.
func installSkillCommand() *command {
	return &command{
		name:    installSkillName,
		summary: installSkillSummary,
		run:     runInstallSkill,
		usage:   writeInstallSkillUsage,
	}
}

func runInstallSkill(a *app, args []string) error {
	f := &installSkillFlags{}
	fs := f.flagSet()
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeInstallSkillUsage(a.stdout)
			return nil
		}
		return &usageError{err: err, usage: writeInstallSkillUsage}
	}
	if rest := fs.Arg(0); rest != "" {
		return &usageError{
			err:   fmt.Errorf("%s takes no arguments, but got %q", installSkillName, rest),
			usage: writeInstallSkillUsage,
		}
	}

	root, shown, err := openClaudeRoot(f.root)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	outcome, err := skill.Install(claudeRoot{root: root}, skills.Dira, f.force)
	if err != nil {
		return err
	}

	// The outcome token and the file it refers to, on stdout, in every case:
	// it is the answer to the question that was asked, and a script reading
	// stdout gets the same word a person reads.
	file := shown + "/" + skill.Path
	_, _ = fmt.Fprintf(a.stdout, "%-9s %s\n", outcome, file)

	if outcome == skill.Refused {
		_, _ = fmt.Fprintf(a.stderr,
			"dira: %s is not the document dira ships, so it was left alone.\n"+
				"      Pass --force to replace it.\n", file)
	}
	return nil
}

type installSkillFlags struct {
	root  string
	force bool
}

func (f *installSkillFlags) flagSet() *flag.FlagSet {
	fs := flag.NewFlagSet(installSkillName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	fs.StringVar(&f.root, "root", "", "install under this directory instead of ~/"+claudeDirName)
	fs.BoolVar(&f.force, "force", false, "replace the installed file even if it was edited locally")
	return fs
}

// openClaudeRoot opens the directory the skill is installed into, and returns
// the way to refer to it in output alongside it.
//
// The displayed form is the tilde one for a default install. That is not
// cosmetic: the absolute path carries the operator's username, and this repo
// treats every output as potentially public.
//
// The default root is reached by opening the home directory as a root and
// descending into `.claude` from there, rather than by joining two strings into
// a path. Descending means the operating system does the confinement — a
// `.claude` that is a symlink out of the home directory is refused rather than
// followed — and it keeps this file free of path/filepath, which dec-0005's
// import boundary does not grant cmd/dira.
func openClaudeRoot(dir string) (root *os.Root, shown string, err error) {
	if dir != "" {
		// An explicit --root is a path the caller typed, so it is opened
		// as given. It is created first: an installer that fails because
		// the directory it was told to use does not exist yet is an
		// installer somebody has to run mkdir for.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, "", fmt.Errorf("creating %s: %w", dir, err)
		}
		opened, err := os.OpenRoot(dir)
		if err != nil {
			return nil, "", fmt.Errorf("opening %s: %w", dir, err)
		}
		return opened, strings.TrimSuffix(dir, "/"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", fmt.Errorf("finding the home directory: %w", err)
	}
	homeRoot, err := os.OpenRoot(home)
	if err != nil {
		return nil, "", fmt.Errorf("opening the home directory: %w", err)
	}
	defer func() { _ = homeRoot.Close() }()

	if err := homeRoot.MkdirAll(claudeDirName, 0o755); err != nil {
		return nil, "", fmt.Errorf("creating ~/%s: %w", claudeDirName, err)
	}
	opened, err := homeRoot.OpenRoot(claudeDirName)
	if err != nil {
		return nil, "", fmt.Errorf("opening ~/%s: %w", claudeDirName, err)
	}
	return opened, "~/" + claudeDirName, nil
}

// claudeRoot adapts *os.Root onto skill.Root.
//
// This is the whole of the filesystem in this feature. internal/skill decides
// what should happen and this does it, inside one directory, with names that
// are relative by construction.
type claudeRoot struct{ root *os.Root }

func (c claudeRoot) ReadFile(name string) ([]byte, bool, error) {
	data, err := c.root.ReadFile(name)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("reading %s: %w", name, err)
	}
	return data, true, nil
}

func (c claudeRoot) WriteFile(name string, data []byte) error {
	// name is slash-separated by contract — it is a name inside a root, the
	// way io/fs uses the word — so its parent is everything before the last
	// slash. This is the one place that has to know that, and it is string
	// handling over a name rather than a path built for the filesystem.
	if at := strings.LastIndex(name, "/"); at > 0 {
		if err := c.root.MkdirAll(name[:at], 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", name[:at], err)
		}
	}
	if err := c.root.WriteFile(name, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	return nil
}

// writeInstallSkillUsage renders `dira install-skill -h`. Assembled in memory
// and written once, for the same reason writeSniffUsage is.
func writeInstallSkillUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira " + installSkillName + " - " + installSkillSummary + "\n\n")
	b.WriteString("usage:\n\n")
	b.WriteString("\tdira " + installSkillName + "                 install into ~/" + claudeDirName + "\n")
	b.WriteString("\tdira " + installSkillName + " --force         replace a locally edited copy\n")
	b.WriteString("\tdira " + installSkillName + " --root DIR      install somewhere else\n\n")

	b.WriteString("Writes " + skill.Path + " — dira's capture tier 2. Tier 1 is the\n")
	b.WriteString("regular expression inside this binary; tier 2 is the session that has\n")
	b.WriteString("the conversation in context, and this document is what tells it how to\n")
	b.WriteString("turn a staged capture into an entry it can cite (dec-0003).\n\n")

	b.WriteString("flags:\n\n")
	for _, line := range [][2]string{
		{"--root DIR", "install under DIR instead of ~/" + claudeDirName},
		{"--force", "replace the installed file even if it was edited"},
	} {
		fmt.Fprintf(&b, "\t%-16s  %s\n", line[0], line[1])
	}

	b.WriteString("\nIt writes one file and only inside the root, and it reports which of\n")
	b.WriteString("three things happened:\n\n")
	fmt.Fprintf(&b, "\t%-9s  the file was written\n", skill.Installed)
	fmt.Fprintf(&b, "\t%-9s  it was already exactly this document; nothing was written\n", skill.Unchanged)
	fmt.Fprintf(&b, "\t%-9s  it exists, differs, and was left alone; pass --force\n", skill.Refused)

	b.WriteString("\nExit status is 0 whenever the command ran, a refusal included: leaving\n")
	b.WriteString("an edited file alone is this command working. Nothing else in dira\n")
	b.WriteString("writes to ~/" + claudeDirName + ", and this only does so when you run it.\n")

	_, _ = io.WriteString(w, b.String())
}
