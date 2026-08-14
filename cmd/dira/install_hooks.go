package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kazi-org/dira/internal/installhooks"
)

// `dira install-hooks` merges dira's three Claude Code hook registrations
// (SessionStart, Stop, PreCompact -- hooks/settings.example.json) into a
// settings file, mirroring cli.ex:5141-5195 and install-skill's contract for
// the same reason (E2-L2-T8's precedent, dec-0008): only this explicit
// command writes harness config, it never clobbers an operator's own file,
// and it cannot write outside the root it was given -- the root is opened as
// an *os.Root, reusing openClaudeRoot and claudeRoot from install_skill.go
// rather than declaring a second pair in this package.
//
// Merge-never-clobber, idempotence, ownership by command prefix, and the
// deletion decision all live in internal/installhooks; this file only opens
// the root, reads and writes through it, and reports the outcome.
//
// Exit status follows docs/lore.md L-0020 and install-skill's precedent:
// 0 whenever the command ran, including UNCHANGED and including a refusal to
// touch an operator-edited entry; 1 when dira could not do the work
// (unreadable file, malformed JSON, a failed write); 2 when the caller
// mistyped.

const installHooksName = "install-hooks"

const installHooksSummary = "merge dira's Claude Code hook registrations into a settings file"

// installHooksUserFile and installHooksLocalFile are the two settings file
// names install-hooks ever writes -- never the committed project
// `.claude/settings.json`, which in a public repo would publish the
// operator's own workflow (dec-0008, kazi's ADR-0034).
const (
	installHooksUserFile  = "settings.json"
	installHooksLocalFile = "settings.local.json"
)

// installHooksCommand is this verb's registry entry. Register it in newApp's
// slice in cmd/dira/main.go, after installSkillCommand:
//
//	{name: installHooksName, summary: installHooksSummary, run: runInstallHooks, usage: writeInstallHooksUsage},
func installHooksCommand() *command {
	return &command{
		name:    installHooksName,
		summary: installHooksSummary,
		run:     runInstallHooks,
		usage:   writeInstallHooksUsage,
	}
}

func runInstallHooks(a *app, args []string) error {
	f := &installHooksFlags{}
	fs := f.flagSet()
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeInstallHooksUsage(a.stdout)
			return nil
		}
		return &usageError{err: err, usage: writeInstallHooksUsage}
	}
	if rest := fs.Arg(0); rest != "" {
		return &usageError{
			err:   fmt.Errorf("%s takes no arguments, but got %q", installHooksName, rest),
			usage: writeInstallHooksUsage,
		}
	}

	root, shown, err := openHooksRoot(f.dir, f.local)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	filename := installHooksUserFile
	if f.local {
		filename = installHooksLocalFile
	}
	cr := claudeRoot{root: root}
	file := shown + "/" + filename
	revert := installHooksRevertFlags(f)

	if f.uninstall {
		return runInstallHooksUninstall(a, cr, filename, file, revert)
	}
	return runInstallHooksInstall(a, cr, filename, file, revert)
}

func runInstallHooksInstall(a *app, root claudeRoot, filename, file, revert string) error {
	data, exists, err := root.ReadFile(filename)
	if err != nil {
		return err
	}

	result, err := installhooks.Install(data, exists)
	if err != nil {
		return err
	}

	if result.Outcome == installhooks.Installed {
		if err := root.WriteFile(filename, result.Data); err != nil {
			return err
		}
	}

	_, _ = fmt.Fprintf(a.stdout, "%-9s %s\n", result.Outcome, file)
	if result.Outcome == installhooks.Installed {
		_, _ = fmt.Fprintln(a.stdout)
		_, _ = fmt.Fprintln(a.stdout, "SessionStart, Stop and PreCompact now run dira's own commands -- every one")
		_, _ = fmt.Fprintln(a.stdout, "guarded so a dira failure never blocks the session (2>/dev/null || true).")
	}
	_, _ = fmt.Fprintf(a.stdout, "Revert with: dira %s --uninstall%s\n", installHooksName, revert)
	return nil
}

func runInstallHooksUninstall(a *app, root claudeRoot, filename, file, revert string) error {
	data, exists, err := root.ReadFile(filename)
	if err != nil {
		return err
	}

	result, err := installhooks.Uninstall(data, exists)
	if err != nil {
		return err
	}

	switch {
	case result.Outcome == installhooks.Unchanged:
		_, _ = fmt.Fprintf(a.stdout, "%-9s %s (no dira hooks installed)\n", result.Outcome, file)
	case result.DeleteFile:
		if err := root.Remove(filename); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "%-9s %s (the file install-hooks created; deleted)\n", result.Outcome, file)
	default:
		if err := root.WriteFile(filename, result.Data); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.stdout, "%-9s %s (dira's hooks removed; everything else preserved)\n", result.Outcome, file)
	}

	// A refusal to touch an operator-edited entry is the command working,
	// not failing (L-0020): exit 0, named on stderr so it cannot pass
	// unnoticed, mirroring install-skill's REFUSED report.
	for _, u := range result.Untouched {
		_, _ = fmt.Fprintf(a.stderr,
			"dira: left %s's entry alone: %s\n"+
				"      it mixes an operator's own command with dira's, or no longer starts with dira's own command; edit the file by hand to remove it.\n",
			u.Event, strings.Join(u.Commands, " / "))
	}
	if result.Outcome != installhooks.Unchanged {
		_, _ = fmt.Fprintf(a.stdout, "Revert with: dira %s%s\n", installHooksName, revert)
	}
	return nil
}

// installHooksRevertFlags renders exactly the flags this invocation was
// given, so the printed revert line is copy-pasteable rather than
// approximately right -- a script running it after a --dir install must not
// silently fall back to the default root.
func installHooksRevertFlags(f *installHooksFlags) string {
	var b strings.Builder
	if f.dir != "" {
		fmt.Fprintf(&b, " --dir %s", f.dir)
	}
	if f.local {
		b.WriteString(" --local")
	}
	return b.String()
}

// openHooksRoot resolves the settings directory. --dir always wins,
// regardless of --local, matching kazi's install_hooks_opts/settings_path:
// --local (T6's --project) chooses which FILE NAME is written inside
// whatever directory is resolved here, not which directory. The default,
// with neither flag, is exactly install-skill's default root: ~/.claude,
// reused rather than re-declared.
func openHooksRoot(dir string, local bool) (root *os.Root, shown string, err error) {
	if dir != "" {
		return openClaudeRoot(dir)
	}
	if local {
		return openLocalClaudeRoot()
	}
	return openClaudeRoot("")
}

// openLocalClaudeRoot opens <cwd>/.claude, the target --local selects with no
// --dir override. Descent from an opened root by NAME, the same move
// openClaudeRoot makes from the home directory, so a `.claude` that is a
// symlink out of the working directory is refused by the operating system
// rather than followed.
func openLocalClaudeRoot() (root *os.Root, shown string, err error) {
	cwdRoot, err := os.OpenRoot(".")
	if err != nil {
		return nil, "", fmt.Errorf("opening the current directory: %w", err)
	}
	defer func() { _ = cwdRoot.Close() }()

	if err := cwdRoot.MkdirAll(claudeDirName, 0o755); err != nil {
		return nil, "", fmt.Errorf("creating %s: %w", claudeDirName, err)
	}
	opened, err := cwdRoot.OpenRoot(claudeDirName)
	if err != nil {
		return nil, "", fmt.Errorf("opening %s: %w", claudeDirName, err)
	}
	return opened, claudeDirName, nil
}

type installHooksFlags struct {
	local     bool
	dir       string
	uninstall bool
}

func (f *installHooksFlags) flagSet() *flag.FlagSet {
	fs := flag.NewFlagSet(installHooksName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	fs.BoolVar(&f.local, "local", false, "target <dir>/"+claudeDirName+"/"+installHooksLocalFile+" instead of the user-level file")
	fs.StringVar(&f.dir, "dir", "", "install under this directory instead of ~/"+claudeDirName+" (or <cwd>/"+claudeDirName+" with --local)")
	fs.BoolVar(&f.uninstall, "uninstall", false, "remove exactly what a previous install added")
	return fs
}

// writeInstallHooksUsage renders `dira install-hooks -h`. Assembled in memory
// and written once, for the same reason writeInstallSkillUsage is.
func writeInstallHooksUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira " + installHooksName + " - " + installHooksSummary + "\n\n")
	b.WriteString("usage:\n\n")
	b.WriteString("\tdira " + installHooksName + "                    install into ~/" + claudeDirName + "/" + installHooksUserFile + "\n")
	b.WriteString("\tdira " + installHooksName + " --local            install into <cwd>/" + claudeDirName + "/" + installHooksLocalFile + "\n")
	b.WriteString("\tdira " + installHooksName + " --dir DIR          install somewhere else\n")
	b.WriteString("\tdira " + installHooksName + " --uninstall        remove exactly what a previous install added\n\n")

	b.WriteString("Merges three hook registrations -- SessionStart, Stop, PreCompact -- into\n")
	b.WriteString("a Claude Code settings file (hooks/settings.example.json documents the\n")
	b.WriteString("exact commands). It never clobbers: an operator's own hooks and keys, and\n")
	b.WriteString("even their formatting, survive byte-identically, and a second install is a\n")
	b.WriteString("byte-level no-op. It never touches the committed project\n")
	b.WriteString(claudeDirName + "/" + installHooksUserFile + " -- in a public repo that would publish your own workflow.\n\n")

	b.WriteString("flags:\n\n")
	for _, line := range [][2]string{
		{"--local", "target <dir>/" + claudeDirName + "/" + installHooksLocalFile + " instead of the user-level file"},
		{"--dir DIR", "install under DIR instead of ~/" + claudeDirName},
		{"--uninstall", "remove exactly what a previous install added"},
	} {
		fmt.Fprintf(&b, "\t%-13s  %s\n", line[0], line[1])
	}

	b.WriteString("\nIt writes one file and only inside the root, and it reports which of\n")
	b.WriteString("three things happened:\n\n")
	fmt.Fprintf(&b, "\t%-9s  the file was written\n", installhooks.Installed)
	fmt.Fprintf(&b, "\t%-9s  it already held exactly this; nothing was written\n", installhooks.Unchanged)
	fmt.Fprintf(&b, "\t%-9s  --uninstall removed dira's own spans (or the whole file, if\n", installhooks.Removed)
	b.WriteString("\t             install created it and nothing else has touched it since)\n")

	b.WriteString("\nExit status is 0 whenever the command ran, including a refusal to touch\n")
	b.WriteString("an entry a human has since edited -- that refusal is reported on stderr,\n")
	b.WriteString("named, and is this command working rather than failing. Nothing else in\n")
	b.WriteString("dira writes to ~/" + claudeDirName + ", and this only does so when you run it.\n")

	_, _ = io.WriteString(w, b.String())
}
