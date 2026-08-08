package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
)

// `dira log` is dira's write verb. It is not a history view: nothing in dira
// lists entries as a `git log`-style log, and a reading of the name that expects
// one is wrong. dec-0003 names this command twice — the skill "calls `dira log`
// with the complete entry", and with no agent driving, capture "degrades to
// `dira log` typed by hand" — and those two callers are why the surface has two
// halves.
//
// `dira log --stdin` is the agent's half. The entry arrives as the same
// frontmatter-plus-markdown document dira stores, because an agent assembling a
// nested structure on a command line is the design decision this lane had to
// make and this is the answer to it. Alternatives, edges and excerpts are prose:
// they contain quotes, colons, newlines and equals signs, and every encoding of
// them into argv is a quoting scheme that silently mangles the one artifact that
// is supposed to outlive the tool. A model that can read `.dira/entries/*.md`
// can already write the format, so the format is the interface, and there is no
// second wire schema to keep in step with entry.schema.json.
//
// The flags are the human's half, for the entry small enough to type. They cover
// the same fields, with the two nested ones built positionally — `--alternative`
// opens one and `--why-not` fills it in — rather than through a delimiter
// micro-syntax that prose would eventually break.
//
// Both halves converge on one Entry, one validation (ledger.ValidateDraft, which
// is Entry.Validate minus the two fields dira supplies itself) and one write.
// The flag layer reports syntax — a malformed `--edge` — and nothing else: every
// question of vocabulary and shape is answered by the ledger's validator, so
// `dira log --kind task` is refused with cst-0002's own words rather than with a
// second, drifting copy of the rule.
//
// `dira log <id>` is the mutation half: it adds an edge or a tag to an entry
// that already exists. Because dec-0002 put edges on the subject entry, that is
// a single file write — the same single write a creation is. This command can
// change no other field: disposition (accepting a staged entry, superseding a
// decision) is E2's flow, and a `--state` flag here would be a second way to do
// it that E2 would then have to reconcile with.

// logUsagef is usagef for a mistake inside `dira log`, so the help printed
// alongside the message is this command's rather than the list of commands.
func logUsagef(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...), usage: writeLogUsage}
}

// runLog writes one entry, or adds one edge to one entry. Either way it is one
// file (dec-0002).
func runLog(a *app, args []string) error {
	// A leading non-flag argument is the id of an entry to mutate. flag
	// stops at the first non-flag argument anyway, so accepting it only
	// here — before the flags rather than after them — keeps `dira log
	// dec-0002 --edge …` the only spelling and leaves no ambiguous form to
	// guess about.
	var id string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		id, args = args[0], args[1:]
	}

	f := &logFlags{}
	fs := f.flagSet()
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeLogUsage(a.stdout)
			return nil
		}
		return logUsagef("%w", err)
	}
	if rest := fs.Arg(0); rest != "" {
		if ledger.ValidID(rest) {
			// The likeliest way to get here is `dira log -C dir
			// dec-0002 …`, so say what to do rather than restate
			// the rule.
			return logUsagef("the id goes before the flags: `dira log %s …`, not after them", rest)
		}
		return logUsagef("log takes at most one argument — the id of an entry to add an edge to — but got %q after the flags", rest)
	}

	set := setFlags(fs)
	if id != "" {
		return a.mutate(id, f, set)
	}
	return a.create(f, set)
}

// create writes a new entry, with its id allocated against the ledger.
func (a *app) create(f *logFlags, set map[string]bool) error {
	entry, err := a.draft(f, set)
	if err != nil {
		return err
	}

	// Validate before opening the ledger, not after. An invalid entry must
	// never reach it (dec-0002: the ledger is the record), and reporting the
	// failing field is worth more than reporting it a few syscalls later.
	if err := entry.ValidateDraft(); err != nil {
		return logUsagef("%w", err)
	}

	store, _, err := openLedger(f.dir)
	if err != nil {
		return err
	}
	if err := ledger.Add(context.Background(), store, entry); err != nil {
		return err
	}

	// The id alone on stdout, so `id=$(dira log …)` works and a hook can
	// use what it just wrote. Everything conversational goes to stderr.
	_, _ = fmt.Fprintln(a.stdout, entry.ID)
	return nil
}

// draft assembles the entry the caller asked for, from either half of the
// surface.
func (a *app) draft(f *logFlags, set map[string]bool) (*ledger.Entry, error) {
	now := a.now().UTC().Format(time.RFC3339)

	if f.stdin {
		for name := range set {
			if name != "stdin" && name != "C" {
				return nil, logUsagef("--stdin carries the whole entry, so it cannot be combined with --%s", name)
			}
		}
		data, err := io.ReadAll(a.stdin)
		if err != nil {
			return nil, fmt.Errorf("reading the entry from stdin: %w", err)
		}
		if len(strings.TrimSpace(string(data))) == 0 {
			return nil, logUsagef("--stdin was given but stdin was empty; the entry is the input")
		}
		entry, err := ledger.DecodeDraft(data)
		if err != nil {
			return nil, logUsagef("%w", err)
		}
		if entry.Created == "" {
			entry.Created = now
		}
		return entry, nil
	}

	if !set["kind"] || !set["title"] {
		return nil, logUsagef("log needs at least --kind and --title, or --stdin carrying a complete entry (try `dira log -h`)")
	}
	if f.bodyFromStdin() {
		body, err := io.ReadAll(a.stdin)
		if err != nil {
			return nil, fmt.Errorf("reading the body from stdin: %w", err)
		}
		f.body = string(body)
	}

	kind := ledger.Kind(f.kind)
	state := ledger.State(f.state)
	if !set["state"] {
		state = defaultState(kind)
	}

	return &ledger.Entry{
		Kind:         kind,
		Title:        f.title,
		State:        state,
		Created:      now,
		Tags:         f.tags,
		Edges:        f.edges,
		Alternatives: f.alternatives,
		Source:       f.source(),
		ConfirmedBy:  f.confirmedBy,
		ADR:          f.adr,
		Private:      f.private,
		Body:         markdownBody(f.body),
	}, nil
}

// mutate adds edges and tags to an entry that already exists.
//
// One read, one write, one file. The edges live on this entry (dec-0002), so
// nothing else in the ledger is touched and there is no second file that would
// have to land atomically with it.
func (a *app) mutate(id string, f *logFlags, set map[string]bool) error {
	if !ledger.ValidID(id) {
		return logUsagef("%q is not an entry id (ids look like dec-0002)", id)
	}
	for name := range set {
		if !mutableFlags[name] {
			return logUsagef("`dira log %s` adds edges and tags to an existing entry, so it does not take --%s. "+
				"Editing an entry's text is a job for an editor, and dispositioning one — accepting it, "+
				"superseding it — is `dira`'s disposition flow rather than another way to spell it here", id, name)
		}
	}
	if len(f.edges) == 0 && len(f.tags) == 0 {
		return logUsagef("`dira log %s` was given nothing to add; pass --edge TYPE=TARGET or --tag TAG", id)
	}

	store, _, err := openLedger(f.dir)
	if err != nil {
		return err
	}
	ctx := context.Background()
	entry, err := store.Get(ctx, id)
	if err != nil {
		return err
	}

	changed := false
	for _, edge := range f.edges {
		added, err := ledger.AddEdge(entry, edge)
		if err != nil {
			return err
		}
		changed = changed || added
	}
	for _, tag := range f.tags {
		changed = ledger.AddTag(entry, tag) || changed
	}

	if !changed {
		// Already true is not a failure. `dira log` runs unattended from
		// hooks that fire more than once for the same conclusion, and a
		// command that fails the second time it is right is a command
		// that gets taken out of the hook.
		_, _ = fmt.Fprintf(a.stderr, "%s already carries all of that; nothing written\n", id)
		_, _ = fmt.Fprintln(a.stdout, entry.ID)
		return nil
	}

	// The schema says updated is bumped on any field change, and it is the
	// only field this command sets on the caller's behalf.
	entry.Updated = a.now().UTC().Format(time.RFC3339)

	// Validate the whole entry before writing it, not just the parts that
	// changed. The backend refuses an invalid entry anyway, so the ledger is
	// safe either way; what this buys is that a mistyped tag is reported as
	// the caller's mistake it is, with the same exit code it would get on
	// the creation path, rather than as a failure of the write.
	if err := entry.Validate(); err != nil {
		return logUsagef("%w", err)
	}
	if err := store.Put(ctx, entry); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(a.stdout, entry.ID)
	return nil
}

// mutableFlags are the flags `dira log <id>` accepts. Everything else on the
// creation surface would be a field rewrite, and this command deliberately
// cannot do one.
var mutableFlags = map[string]bool{"edge": true, "edge-note": true, "tag": true, "C": true}

// logFlags is the flag surface, collected. The repeatable ones are built as they
// are parsed, because flag calls each Set in the order the arguments appeared,
// which is what lets --why-not attach to the --alternative it follows.
type logFlags struct {
	kind  string
	title string
	state string
	body  string

	tags         []string
	edges        []ledger.Edge
	alternatives []ledger.Alternative

	hook    string
	session string
	excerpt string
	tier    string

	confirmedBy string
	adr         string
	private     bool

	stdin bool
	dir   string
}

func (f *logFlags) flagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	fs.StringVar(&f.kind, "kind", "", "intent, decision, question, constraint or note")
	fs.StringVar(&f.title, "title", "", "one line, legible with no context")
	fs.StringVar(&f.state, "state", "", "lifecycle state; defaults to the first state valid for the kind")
	fs.StringVar(&f.body, "body", "", "the prose because; - reads it from stdin")

	fs.Func("tag", "retrieval tag; repeatable", func(v string) error {
		f.tags = append(f.tags, v)
		return nil
	})
	fs.Func("edge", "TYPE=TARGET, e.g. derives_from=int-0002; repeatable", f.addEdge)
	fs.Func("edge-note", "one line on why the preceding --edge exists", f.addEdgeNote)
	fs.Func("alternative", "the road not taken; repeatable, each needs a --why-not", f.addAlternative)
	fs.Func("why-not", "the reason the preceding --alternative was not taken", f.addWhyNot)
	fs.Func("revisit-if", "what would legitimately reopen the preceding --alternative", f.addRevisitIf)

	fs.StringVar(&f.hook, "hook", "", "capture point: SessionStart, Stop, PreCompact, manual, import, kazi-disposition")
	fs.StringVar(&f.session, "session", "", "opaque session id this entry came from")
	fs.StringVar(&f.excerpt, "excerpt", "", "the transcript fragment the capture was inferred from")
	fs.StringVar(&f.tier, "tier", "", "how it was extracted: regex, semantic or human")

	fs.StringVar(&f.confirmedBy, "confirmed-by", "", "who dispositioned this entry, e.g. human")
	fs.StringVar(&f.adr, "adr", "", "path to a mirrored ADR file")
	fs.BoolVar(&f.private, "private", false, "never exportable to a downstream ledger")

	fs.BoolVar(&f.stdin, "stdin", false, "read a complete entry (frontmatter + body, no id) from stdin")
	fs.StringVar(&f.dir, "C", "", "run as if started in this directory")
	return fs
}

// bodyFromStdin reports whether --body asked for the body to be read from
// stdin. A bare `-` is the conventional spelling and cannot collide with a real
// body: a one-character body consisting of a hyphen is not prose.
func (f *logFlags) bodyFromStdin() bool { return f.body == "-" }

func (f *logFlags) addEdge(v string) error {
	edgeType, to, ok := strings.Cut(v, "=")
	// `=` rather than `:` because a namespaced ref carries a colon
	// (sire:int-0002) and a target never carries an equals sign.
	if !ok {
		return fmt.Errorf("%q is not TYPE=TARGET, e.g. derives_from=int-0002", v)
	}
	edgeType, to = strings.TrimSpace(edgeType), strings.TrimSpace(to)
	if edgeType == "" || to == "" {
		return fmt.Errorf("%q is not TYPE=TARGET, e.g. derives_from=int-0002", v)
	}
	// The type is not checked against the vocabulary here. Entry.Validate
	// owns that, and it can name every allowed value; a second copy of the
	// list in the flag layer is a second thing to keep in step.
	f.edges = append(f.edges, ledger.Edge{Type: ledger.EdgeType(edgeType), To: to})
	return nil
}

func (f *logFlags) addEdgeNote(v string) error {
	if len(f.edges) == 0 {
		return errors.New("--edge-note describes the --edge before it, and no --edge came first")
	}
	last := &f.edges[len(f.edges)-1]
	if last.Note != "" {
		return fmt.Errorf("the %s edge to %s already has a note", last.Type, last.To)
	}
	last.Note = v
	return nil
}

func (f *logFlags) addAlternative(v string) error {
	f.alternatives = append(f.alternatives, ledger.Alternative{Option: v})
	return nil
}

func (f *logFlags) addWhyNot(v string) error {
	last, err := f.lastAlternative("--why-not")
	if err != nil {
		return err
	}
	if last.WhyNot != "" {
		return fmt.Errorf("the alternative %q already has a --why-not", last.Option)
	}
	last.WhyNot = v
	return nil
}

func (f *logFlags) addRevisitIf(v string) error {
	last, err := f.lastAlternative("--revisit-if")
	if err != nil {
		return err
	}
	if last.RevisitIf != "" {
		return fmt.Errorf("the alternative %q already has a --revisit-if", last.Option)
	}
	last.RevisitIf = v
	return nil
}

func (f *logFlags) lastAlternative(flagName string) (*ledger.Alternative, error) {
	if len(f.alternatives) == 0 {
		return nil, fmt.Errorf("%s describes the --alternative before it, and no --alternative came first", flagName)
	}
	return &f.alternatives[len(f.alternatives)-1], nil
}

// source is the provenance block.
//
// hook defaults to manual — an invocation that says nothing about where it came
// from came from someone running the command — and tier deliberately does not
// default. tier is a claim about how the content was extracted, and only the
// caller knows whether a human wrote it or a regex guessed at it; inventing
// `human` here would let an agent's inference wear a human's confidence, which
// is the one thing the provenance block exists to prevent.
func (f *logFlags) source() *ledger.Source {
	s := &ledger.Source{
		Hook:    ledger.Hook(f.hook),
		Session: f.session,
		Excerpt: f.excerpt,
		Tier:    ledger.Tier(f.tier),
	}
	if s.Hook == "" {
		s.Hook = ledger.HookManual
	}
	return s
}

// defaultState is the state a kind gets when the caller does not say. It is the
// first state the kind allows, which is not a coincidence dressed up as a rule:
// Kind.States lists them in the schema's order and that order runs from the
// state an entry is born in to the states it ends in — active, accepted, open,
// active, active.
func defaultState(k ledger.Kind) ledger.State {
	states := k.States()
	if len(states) == 0 {
		// An unknown kind. Leaving the state empty lets validation
		// report the kind, which is the actual mistake.
		return ""
	}
	return states[0]
}

// markdownBody shapes typed text into the body as the file stores it: a blank
// line after the closing frontmatter delimiter, then the prose, then exactly one
// trailing newline.
//
// Only the surrounding newlines are touched. Indentation, blank lines inside the
// text and trailing structure are content — the codec never reformats the body,
// and neither does this.
func markdownBody(text string) string {
	text = strings.Trim(text, "\n")
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return "\n" + text + "\n"
}

// setFlags is the set of flags actually given on the command line, which is not
// the same question as which flags are non-empty: `--title ""` is a mistake to
// report, not a title left unset.
func setFlags(fs *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return set
}

// writeLogUsage renders `dira log -h`. Assembled in memory and written once, for
// the same reason writeUsage is: a broken pipe here is real and unactionable.
func writeLogUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira log - write an entry to the ledger, or add an edge to one\n\n")
	b.WriteString("usage:\n\n")
	b.WriteString("\tdira log --kind KIND --title TEXT [flags]   write a new entry\n")
	b.WriteString("\tdira log --stdin                            write an entry read from stdin\n")
	b.WriteString("\tdira log ID --edge TYPE=TARGET              add an edge to an existing entry\n\n")

	b.WriteString("The id is allocated by dira: the lowest unused number for the kind.\n")
	b.WriteString("Each invocation writes exactly one file under .dira/entries/.\n\n")

	b.WriteString("entry flags:\n\n")
	for _, line := range [][2]string{
		{"--kind KIND", "intent, decision, question, constraint or note"},
		{"--title TEXT", "one line, legible with no context"},
		{"--state STATE", "defaults to the first state valid for the kind"},
		{"--body TEXT", "the prose because; - reads it from stdin"},
		{"--tag TAG", "repeatable"},
		{"--edge TYPE=TARGET", "repeatable, e.g. derives_from=int-0002"},
		{"--edge-note TEXT", "one line on why the preceding --edge exists"},
		{"--alternative TEXT", "repeatable; the road not taken"},
		{"--why-not TEXT", "why the preceding --alternative was not taken"},
		{"--revisit-if TEXT", "what would reopen the preceding --alternative"},
		{"--hook HOOK", "capture point; defaults to manual"},
		{"--session ID", "opaque session id this entry came from"},
		{"--excerpt TEXT", "the transcript fragment it was inferred from"},
		{"--tier TIER", "regex, semantic or human"},
		{"--confirmed-by WHO", "who dispositioned it, e.g. human"},
		{"--adr PATH", "path to a mirrored ADR file"},
		{"--private", "never exportable to a downstream ledger"},
		{"--stdin", "read the whole entry from stdin; excludes the flags above"},
		{"-C DIR", "run as if started in this directory"},
	} {
		fmt.Fprintf(&b, "\t%-20s  %s\n", line[0], line[1])
	}

	b.WriteString("\nA decision must record at least one alternative with its --why-not.\n")
	b.WriteString("`dira log ID` takes --edge, --edge-note and --tag: it adds to an entry,\n")
	b.WriteString("and never rewrites one.\n")

	_, _ = io.WriteString(w, b.String())
}
