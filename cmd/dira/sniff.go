package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/sniff"
)

// `dira sniff` is capture tier 1 (dec-0003): pattern matching over the session
// transcript, proposing entries a human then disposes of.
//
// Two things about the surface are worth stating, because both are consequences
// of what the tier is rather than preferences:
//
// It reads Claude Code's hook payload from stdin. A Stop hook is invoked with no
// arguments and a JSON object on stdin carrying `session_id` and
// `transcript_path`, and that is where the transcript actually is — a `--transcript`
// flag the installer had to fill in would be an installer that has to know a path
// that changes every session. Input that is not a hook payload is read as prose,
// so `dira sniff < notes.md` works and a human can see what the tier would make
// of anything.
//
// It writes nothing without `--stage`. The default is a dry run that prints what
// it would propose, because the first thing anybody does with a capture tool is
// point it at their own transcript to see whether they trust it, and a tool that
// answered that question by writing eleven files into their ledger would not get
// a second try.
//
// Exit status is 0 for "ran", including "ran and found nothing". A non-zero exit
// means dira is broken, not that the session made no decisions — E2-L3's hooks
// have to be able to tell those apart to fail open.

// runSniff is the command. It is registered in newApp's slice as:
//
//	{name: "sniff", summary: sniffSummary, run: runSniff, usage: writeSniffUsage},
func runSniff(a *app, args []string) error {
	f := &sniffFlags{}
	fs := f.flagSet()
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeSniffUsage(a.stdout)
			return nil
		}
		return &usageError{err: err, usage: writeSniffUsage}
	}
	if rest := fs.Arg(0); rest != "" {
		return &usageError{
			err:   fmt.Errorf("sniff takes no arguments — the transcript arrives on stdin or through --transcript — but got %q", rest),
			usage: writeSniffUsage,
		}
	}

	input, session, err := a.transcriptSource(f)
	if err != nil {
		return err
	}
	if closer, ok := input.(io.Closer); ok && closer != nil {
		defer func() { _ = closer.Close() }()
	}

	scope := sniff.LastTurn
	if f.all {
		scope = sniff.Whole
	}
	candidates, err := sniff.SniffTranscript(input, scope)
	if err != nil {
		return err
	}

	if !f.stage {
		a.reportDryRun(f, candidates)
		return nil
	}

	store, _, err := openLedger(f.dir)
	if err != nil {
		return err
	}
	result, err := sniff.Stage(context.Background(), store, sniff.StageOptions{
		Hook:    ledger.Hook(f.hook),
		Session: session,
		Now:     a.now().UTC(),
	}, candidates)
	if err != nil {
		return err
	}
	a.reportStaged(f, result)
	return nil
}

// transcriptSource resolves where the session text comes from, and the session
// id that goes with it.
//
// The hook payload is tried first, then a `--transcript` path, then stdin as
// prose. A payload that names a transcript dira cannot open is not an error: a
// hook firing in a directory whose transcript has been rotated away should
// capture nothing quietly rather than print a failure into the session.
func (a *app) transcriptSource(f *sniffFlags) (io.Reader, string, error) {
	if f.transcript != "" {
		file, err := os.Open(f.transcript)
		if err != nil {
			return nil, "", fmt.Errorf("reading the transcript: %w", err)
		}
		return file, f.session, nil
	}

	data, err := io.ReadAll(a.stdin)
	if err != nil {
		return nil, "", fmt.Errorf("reading stdin: %w", err)
	}

	payload := parseHookPayload(data)
	if payload == nil {
		return strings.NewReader(string(data)), f.session, nil
	}

	session := payload.SessionID
	if f.session != "" {
		session = f.session
	}
	if payload.TranscriptPath == "" {
		return strings.NewReader(""), session, nil
	}
	file, err := os.Open(payload.TranscriptPath)
	if err != nil {
		// Quietly nothing: see the doc comment.
		return strings.NewReader(""), session, nil
	}
	return file, session, nil
}

// A hookPayload is the part of Claude Code's hook input dira reads. Everything
// else in it — cwd, permission mode, the tool name — is another hook's business.
type hookPayload struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	HookEventName  string `json:"hook_event_name"`
}

// parseHookPayload recognises a hook invocation, and returns nil for anything
// else so that stdin falls through to being read as prose.
//
// A transcript is also JSON, one object per line, so "it parsed as JSON" is not
// enough to tell them apart. What distinguishes a payload is that it is a single
// object carrying at least one of the two fields this command needs.
func parseHookPayload(data []byte) *hookPayload {
	trimmed := strings.TrimSpace(string(data))
	if !strings.HasPrefix(trimmed, "{") || strings.Contains(trimmed, "\n") {
		return nil
	}
	var p hookPayload
	if err := json.Unmarshal([]byte(trimmed), &p); err != nil {
		return nil
	}
	if p.TranscriptPath == "" && p.SessionID == "" {
		return nil
	}
	return &p
}

// reportDryRun prints what the tier would propose. It goes to stdout because it
// is the answer to the question that was asked.
func (a *app) reportDryRun(f *sniffFlags, candidates []sniff.Candidate) {
	if len(candidates) == 0 {
		if !f.quiet {
			_, _ = fmt.Fprintln(a.stdout, "nothing to stage")
		}
		return
	}
	for _, c := range candidates {
		_, _ = fmt.Fprintf(a.stdout, "would stage  %s\n", c.Title)
	}
	if !f.quiet {
		_, _ = fmt.Fprintf(a.stdout, "\n%d candidate(s); nothing written. Add --stage to write them as staged entries.\n", len(candidates))
	}
}

// reportStaged prints what was written, in the shape docs/design.md §5 shows.
//
// --quiet suppresses everything except the staged lines themselves. It exists
// for the Stop hook, which fires after every turn: the common case is that
// nothing was found, and a line of output on every turn is a tool that gets
// uninstalled.
func (a *app) reportStaged(f *sniffFlags, result *sniff.Result) {
	for _, e := range result.Staged {
		_, _ = fmt.Fprintf(a.stdout, "dira: staged %s %q\n      — confirm or ignore\n", e.ID, e.Title)
	}
	if f.quiet {
		return
	}
	if len(result.Staged) == 0 {
		_, _ = fmt.Fprintln(a.stdout, "nothing to stage")
	}
	if result.Duplicates > 0 {
		_, _ = fmt.Fprintf(a.stderr, "%d candidate(s) were already in the ledger\n", result.Duplicates)
	}
	if result.Dropped > 0 {
		_, _ = fmt.Fprintf(a.stderr, "%d candidate(s) dropped: one run stages at most %d. Run `dira sniff` without --stage to see the rest.\n",
			result.Dropped, sniff.Bound())
	}
}

type sniffFlags struct {
	stage      bool
	quiet      bool
	all        bool
	hook       string
	session    string
	transcript string
	dir        string
}

func (f *sniffFlags) flagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("sniff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	fs.BoolVar(&f.stage, "stage", false, "write the candidates to the ledger as staged entries")
	fs.BoolVar(&f.quiet, "quiet", false, "print nothing unless something was staged")
	fs.BoolVar(&f.all, "all", false, "read the whole transcript rather than the last turn")
	fs.StringVar(&f.hook, "hook", string(ledger.HookStop), "capture point recorded on each entry: Stop, PreCompact or manual")
	fs.StringVar(&f.session, "session", "", "opaque session id; taken from the hook payload when absent")
	fs.StringVar(&f.transcript, "transcript", "", "read this file instead of stdin")
	fs.StringVar(&f.dir, "C", "", "run as if started in this directory")
	return fs
}

const sniffSummary = "read the session for decisions and stage them for review"

// writeSniffUsage renders `dira sniff -h`. Assembled in memory and written once,
// for the same reason writeLogUsage is.
func writeSniffUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira sniff - read the session for decisions and stage them for review\n\n")
	b.WriteString("usage:\n\n")
	b.WriteString("\tdira sniff                     show what the last turn would stage\n")
	b.WriteString("\tdira sniff --stage --quiet     stage it; the Stop hook's invocation\n")
	b.WriteString("\tdira sniff --transcript FILE   read a transcript from disk\n\n")

	b.WriteString("Everything it writes is `state: staged` with `source.tier: regex`, and\n")
	b.WriteString("waits for you. A regular expression has no business asserting rationale,\n")
	b.WriteString("so nothing here is ever accepted, carries an alternative, or claims a\n")
	b.WriteString("reason (dec-0003). Confirming one is a separate verb.\n\n")

	b.WriteString("flags:\n\n")
	for _, line := range [][2]string{
		{"--stage", "write the candidates; without it nothing is written"},
		{"--quiet", "print nothing unless something was staged"},
		{"--all", "read the whole transcript, not just the last turn"},
		{"--hook HOOK", "capture point recorded: Stop, PreCompact or manual"},
		{"--session ID", "session id; taken from the hook payload when absent"},
		{"--transcript FILE", "read this file instead of stdin"},
		{"-C DIR", "run as if started in this directory"},
	} {
		fmt.Fprintf(&b, "\t%-20s  %s\n", line[0], line[1])
	}

	b.WriteString("\nWith no --transcript, stdin is read as a Claude Code hook payload if it\n")
	b.WriteString("looks like one, and as prose otherwise.\n")
	b.WriteString("Exit status is 0 whenever the command ran, including when it found\n")
	b.WriteString("nothing: a hook must be able to tell that from dira being broken.\n")

	_, _ = io.WriteString(w, b.String())
}
