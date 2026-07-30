// Command dira is the CLI for the dira ledger.
//
// The command path is deliberately stdlib-only. dira is invoked from Claude
// Code hooks in a human's latency path, several times per session, and
// int-0002 budgets the whole invocation at well under 100ms; dec-0001 chose Go
// over Elixir for exactly that reason. A CLI framework in this path spends the
// benefit the language choice bought, so subcommand dispatch is hand-written
// over stdlib flag. See .dira/entries/dec-0001.md and int-0002.md.
//
// Exit codes are a contract that later epics depend on — E2's hooks must be
// able to distinguish "dira told me no" from "dira is broken" so they can fail
// open:
//
//	0  success
//	1  runtime error
//	2  usage error
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// Exit codes. Documented in the package comment because hooks depend on them.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

// version is the build version. It is "dev" for a plain `go build` and is
// replaced at link time by the release pipeline:
//
//	go build -ldflags "-X main.version=1.2.3" ./cmd/dira
var version = "dev"

func main() {
	os.Exit(newApp(os.Stdout, os.Stderr).main(os.Args[1:]))
}

// command is one dira subcommand. Adding one is a single entry in newApp's
// slice; nothing else here changes.
type command struct {
	name    string
	summary string
	run     func(a *app, args []string) error
}

// app is the command's runtime state. Everything the commands touch arrives
// through it so the whole CLI surface is testable without a subprocess.
type app struct {
	name     string
	version  string
	stdout   io.Writer
	stderr   io.Writer
	commands []*command
}

// newApp builds the command registry. Commands are listed in the order they
// render in help.
func newApp(stdout, stderr io.Writer) *app {
	a := &app{
		name:    "dira",
		version: version,
		stdout:  stdout,
		stderr:  stderr,
	}
	a.commands = []*command{
		{name: "help", summary: "show usage for dira or one of its commands", run: runHelp},
		{name: "reindex", summary: "rebuild the derived read cache from the entry files", run: runReindex},
		{name: "version", summary: "print the dira version", run: runVersion},
	}
	return a
}

// usageError marks a caller mistake rather than a failure while doing the
// work. It is what selects exit code 2.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

func usagef(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...)}
}

// main runs args and maps the outcome onto the exit-code contract. Usage
// errors print the offence and the full usage to stderr and leave stdout
// untouched, so a caller parsing stdout never sees help text.
func (a *app) main(args []string) int {
	err := a.run(args)
	if err == nil {
		return exitOK
	}

	var ue *usageError
	if errors.As(err, &ue) {
		// stderr is unactionable on failure; discard explicitly (see writeUsage).
		_, _ = fmt.Fprintf(a.stderr, "%s: %v\n\n", a.name, err)
		a.writeUsage(a.stderr)
		return exitUsage
	}

	_, _ = fmt.Fprintf(a.stderr, "%s: %v\n", a.name, err)
	return exitError
}

func (a *app) run(args []string) error {
	fs := flag.NewFlagSet(a.name, flag.ContinueOnError)
	// flag's own reporting writes usage to stderr and duplicates whatever
	// a.main prints. Silence it and route everything through a.main.
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	showVersion := fs.Bool("version", false, "print the version and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// Help was asked for, so it is the answer, not an error.
			a.writeUsage(a.stdout)
			return nil
		}
		return usagef("%w", err)
	}

	if *showVersion {
		_, _ = fmt.Fprintln(a.stdout, a.version)
		return nil
	}

	rest := fs.Args()
	if len(rest) == 0 {
		a.writeUsage(a.stdout)
		return nil
	}

	cmd := a.lookup(rest[0])
	if cmd == nil {
		return usagef("unknown command %q", rest[0])
	}
	return cmd.run(a, rest[1:])
}

func (a *app) lookup(name string) *command {
	for _, c := range a.commands {
		if c.name == name {
			return c
		}
	}
	return nil
}

func runHelp(a *app, args []string) error {
	switch len(args) {
	case 0:
		a.writeUsage(a.stdout)
		return nil
	case 1:
		cmd := a.lookup(args[0])
		if cmd == nil {
			return usagef("unknown command %q", args[0])
		}
		_, _ = fmt.Fprintf(a.stdout, "%s %s - %s\n", a.name, cmd.name, cmd.summary)
		return nil
	default:
		return usagef("help takes at most one command name")
	}
}

func runVersion(a *app, args []string) error {
	if len(args) > 0 {
		return usagef("version takes no arguments")
	}
	_, _ = fmt.Fprintln(a.stdout, a.version)
	return nil
}
