package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kazi-org/dira/internal/interview"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// `dira init --interview` seeds a fresh personal or workspace ledger by
// asking a short, fixed set of questions on stdout and reading one answer
// per question from stdin (dec-0003: dira embeds no model client, so this is
// the whole interview — a live session asks better questions and pipes the
// answers in, dira never generates one itself).
//
// Everything about the all-or-nothing guarantee (dec-0010: no successful
// init produces an empty ledger, and no unsuccessful one produces a
// non-empty one) lives in internal/interview.Build (validation, no I/O) and
// internal/ledger/local.InitLedger (the write, staged and committed once).
// This file is the seam between them and the terminal: it owns the prompt
// loop and nothing about what makes an answer valid.
//
// A repo-tier `.dira` is not seeded by this command — `dira log --stdin`
// already seeds a repo ledger's first entry, and the lane this command
// belongs to scopes itself to person and workspace.

const initSummary = "seed a new personal or workspace ledger by answering a short interview"

func initUsagef(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...), usage: writeInitUsage}
}

// runInit runs the interview and, on a complete answer set, writes the
// ledger it describes.
func runInit(a *app, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	dir := fs.String("C", "", "run as if started in this directory")
	asInterview := fs.Bool("interview", false, "seed a ledger by answering a short interview")
	tierFlag := fs.String("tier", "", "the tier being seeded (person or workspace) — must match the interview's own answer")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeInitUsage(a.stdout)
			return nil
		}
		return initUsagef("%w", err)
	}
	if fs.NArg() > 0 {
		return initUsagef("init takes no arguments, got %d", fs.NArg())
	}
	if !*asInterview {
		return initUsagef("init only supports --interview in this release")
	}
	if *tierFlag != "" && *tierFlag != interview.TierPerson && *tierFlag != interview.TierWorkspace {
		return initUsagef("--tier must be %q or %q, got %q", interview.TierPerson, interview.TierWorkspace, *tierFlag)
	}

	root := *dir
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("finding the working directory: %w", err)
		}
		root = cwd
	}

	answers, err := askPrompts(a, *tierFlag)
	if err != nil {
		// A --tier mismatch is a usage error (exit 2, L-0020): the
		// operator told dira two different things about the same run.
		// Every other reason answers were incomplete is real work not
		// completing (exit 1), the same distinction `dira check` draws
		// between a usage mistake and a verdict.
		return err
	}

	tier, drafts, err := interview.Build(answers)
	if err != nil {
		return err
	}

	now := a.now().UTC().Format(time.RFC3339)
	for _, d := range drafts {
		d.Created = now
	}
	cfg := []byte(fmt.Sprintf("[ledger]\ntier = %q\n", tier))

	if _, err := local.InitLedger(root, cfg, drafts); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(a.stdout, "seeded a new %s ledger at %s/.dira\n", tier, root)
	return nil
}

// askPrompts asks interview.Prompts in order, printing each to stdout and
// reading one line of the answer from stdin. It returns as many answers as
// were collected before stdin closed — interview.Build is what decides
// whether that is a complete set.
//
// A --tier flag is checked against the interview's own first answer as soon
// as that answer is read, before any further prompt is printed: an operator
// who told dira two different tiers in the same run is told so immediately
// rather than after answering the rest of the interview for nothing.
func askPrompts(a *app, tierFlag string) ([]string, error) {
	scanner := bufio.NewScanner(a.stdin)
	answers := make([]string, 0, len(interview.Prompts))

	for i, prompt := range interview.Prompts {
		if _, err := fmt.Fprintln(a.stdout, prompt); err != nil {
			return nil, err
		}
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()

		if i == 0 && tierFlag != "" && strings.TrimSpace(line) != tierFlag {
			return nil, initUsagef("--tier=%s does not match the interview's own answer %q", tierFlag, strings.TrimSpace(line))
		}
		answers = append(answers, line)
	}
	return answers, scanner.Err()
}

// writeInitUsage renders `dira init`'s own help.
func writeInitUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira init - seed a new personal or workspace ledger.\n\n")
	b.WriteString("usage:\n\n\tdira init --interview [flags]\n\n")
	b.WriteString("Asks a short, fixed set of questions on stdout and reads one answer per\n")
	b.WriteString("question from stdin: which tier you are seeding, one intent, one\n")
	b.WriteString("constraint, and one open question. dira asks; it never generates a\n")
	b.WriteString("question or interprets an answer (dec-0003).\n\n")
	b.WriteString("Either every answer is written or none are: an aborted or unanswered\n")
	b.WriteString("interview leaves no .dira behind at all (dec-0010).\n\n")

	b.WriteString("flags:\n\n")
	b.WriteString("\t-C <dir>        run as if started in this directory\n")
	b.WriteString("\t-interview      seed a ledger by answering the interview\n")
	b.WriteString("\t-tier <tier>    person or workspace — must match the interview's own answer\n\n")

	b.WriteString("exit codes:\n\n")
	b.WriteString("\t0  a new ledger was seeded\n")
	b.WriteString("\t1  the interview was incomplete, or the ledger could not be written\n")
	b.WriteString("\t2  usage error\n")

	_, _ = io.WriteString(w, b.String())
}
