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

// `dira supersede` is how thinking is retired.
//
// It exists because the alternative is worse. Every conflict `dira check` prints
// ends with "→ supersede dec-0060, or revise the plan", and until this command
// existed the only way to take that first option was to open the entry file and
// edit `state:` by hand — which is to say, the remedy the firewall offers was a
// text editor and a convention. A convention is what produced the defect
// qst-0006 records: three entries in this repository's own ledger carried a
// `supersedes` edge whose target was still `accepted`, so the record said an
// entry had been replaced and simultaneously that it was still in force.
//
// So this command writes both sides or it writes neither useful half:
//
//	the new entry gains a `supersedes` edge to the old one
//	the old entry's state becomes `superseded`
//
// and it is the *pair* that changes what `dira check` enforces. The edge alone
// is a claim about the record; the state alone is an entry that stops being
// enforced with nothing saying what took its place. E3's enforcement table is
// explicit that a superseded decision is matched against nothing "but a match is
// reported and redirected to its superseder" — and there is no superseder to
// redirect to without the edge.
//
// # The two-file write, and what happens when it fails halfway
//
// dec-0002 puts edges on the subject entry precisely so that every dira mutation
// is one file, and this command is the exception that proves it needs a stated
// failure mode rather than an assumption. There is no transaction here and there
// cannot be one: ledger.Store has no batch and no lock, because neither exists
// over the GitHub Contents API (dec-0005).
//
// The order is therefore chosen for what a crash between the two writes leaves
// behind, and it is the edge first:
//
//   - Edge first, then the state. A crash leaves the old entry still `accepted`
//     and still enforced, with the new entry claiming to have replaced it. The
//     firewall is unchanged — nothing that was refused yesterday is allowed
//     today — and the inconsistency is visible in the record, which is how
//     qst-0006 was found in the first place.
//
//   - State first, then the edge. A crash leaves the old entry superseded with
//     nothing recorded as replacing it. `dira check` stops enforcing it, says
//     nothing about where the thinking went, and the ledger looks fine.
//
// The first failure is loud and safe; the second is silent and weakens the
// check. Both are recoverable by re-running the identical command, which is why
// this command completes a half-finished supersession rather than refusing one:
// running it twice is how a crash is repaired, and a command that failed on the
// second run would leave no way to finish the job but the text editor this
// command exists to replace.
//
// # What it refuses
//
// A namespaced ref on either side — `dira supersede me:cst-0002 --with cst-0005`
// — is refused before the ledger is even opened. Both directions are an upward
// write: superseding a parent's entry means writing that parent's `state`, and
// being superseded *by* a parent's entry means writing the edge onto the
// parent's file. cst-0003 rule 1 makes either one a security bug rather than a
// UX one, and the check is on the ref itself so that no resolution, no
// filesystem access and no partial write can happen first.
//
// A question is refused because the schema gives it only `open` and `answered`:
// there is no `superseded` state to move it to, and an answered question is
// answered, not retired. The same reasoning covers an intent and a note, and it
// is asked of ledger.Kind.States rather than of a list kept here, so the schema
// stays the single source of what a kind may be.

// supersedeSummary is the one line `dira help` lists this command with. It sits
// beside the command rather than inside main.go's registry so that the registry
// line is the only thing the integrator has to add.
const supersedeSummary = "retire an entry in favour of the one that replaces it"

// The exit codes, and why this command does not use the shared usagef.
//
// docs/lore.md L-0013 records the rule `dira check` had to learn: main.go maps a
// usageError onto exit 2 for every command, so a command that means something
// specific by 2 must never raise one. `dira check` means "at least one cited
// conflict" and routes its own flag errors to 1, which is what makes its 2 mean
// only a verdict.
//
// This command makes the same split for the same reason, and the two commands
// then agree on what a caller scripting both may conclude:
//
//	0  the supersession is recorded — either this run wrote it, or it was already
//	2  the ledger refuses the mutation. A rule in the record says no: a parent's
//	   entry (cst-0003), a kind with no superseded state, an entry already
//	   replaced, a staged replacement
//	1  dira did not get that far. A command line it could not act on, an entry
//	   that is not there, an unreadable ledger, a write that failed
//
// So across E3's two verbs 2 is always the record's answer and never dira's own
// mistake, and 1 is never a verdict. That is what lets a hook fail open on 1 and
// surface 2, which is the entire reason the split exists (docs/plan/lanes/E3.md).
//
// The cost, stated because it is a real inconsistency rather than a tidy one:
// `dira log` maps its own flag errors to 2, the binary's general contract, and
// this command does not. Both cannot be satisfied — log and check already
// disagree — and the verb that shares a caller with `check` is this one.

// supersedeRefuse reports a mutation the record's own rules forbid: exit 2.
func (a *app) supersedeRefuse(format string, args ...any) error {
	return a.supersedeSay(exitUsage, format, args...)
}

// supersedeMisuse reports a command line dira could not act on at all: exit 1.
// Not exit 2 — for this command that code says the ledger refused something, and
// a caller must never read a mistyped flag as a policy refusal.
func (a *app) supersedeMisuse(format string, args ...any) error {
	return a.supersedeSay(exitError, format, args...)
}

// supersedeSay writes the message and this command's usage to stderr and selects
// the exit code. The message is written here rather than raised as a usageError
// because a usageError is exactly the thing that would select 2 unconditionally.
func (a *app) supersedeSay(code int, format string, args ...any) error {
	_, _ = fmt.Fprintf(a.stderr, "%s supersede: %v\n\n", a.name, fmt.Errorf(format, args...))
	writeSupersedeUsage(a.stderr)
	return &codedError{code: code}
}

// runSupersede retires one entry in favour of another.
func runSupersede(a *app, args []string) error {
	// The target comes before the flags, the same shape `dira log <id>` uses,
	// so there is one spelling of "the entry this command acts on" in the
	// binary rather than two.
	var target string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		target, args = args[0], args[1:]
	}

	fs := flag.NewFlagSet("supersede", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	with := fs.String("with", "", "the entry that replaces it")
	note := fs.String("note", "", "one line on why the replacement happened")
	dir := fs.String("C", "", "run as if started in this directory")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeSupersedeUsage(a.stdout)
			return nil
		}
		return a.supersedeMisuse("%w", err)
	}
	if rest := fs.Arg(0); rest != "" {
		if ledger.ValidRef(rest) && target == "" {
			return a.supersedeMisuse("the entry to supersede goes before the flags: `dira supersede %s --with …`", rest)
		}
		return a.supersedeMisuse("supersede takes one argument — the entry to retire — but got %q after the flags", rest)
	}

	if err := a.checkRefs(target, *with); err != nil {
		return err
	}
	return a.supersede(target, *with, *note, *dir)
}

// checkRefs validates the two ids before anything is opened, read or written.
//
// Every refusal in this function is reachable without touching a filesystem, and
// that ordering is the point rather than an optimisation: the acceptance clause
// for a cross-ledger supersede is that it "writes nothing and leaves the parent
// ledger byte-identical", and the cheapest way to guarantee that is for the
// command to have refused before it knew where any ledger was.
func (a *app) checkRefs(target, with string) error {
	// Which of the two codes each refusal below selects is not a detail. A
	// missing argument and a malformed id are things dira could not act on;
	// everything after them is the record saying no to a well-formed request.
	switch {
	case target == "":
		return a.supersedeMisuse("supersede needs the entry to retire: `dira supersede dec-0060 --with dec-0061`")
	case with == "":
		return a.supersedeMisuse("supersede needs the entry that replaces %s: `dira supersede %s --with <id>`", target, target)
	}

	for _, ref := range []string{target, with} {
		if !strings.Contains(ref, ":") {
			continue
		}
		// A namespaced ref names another ledger, and both directions of
		// this command would write to it: retiring a parent's entry
		// writes its state, and being replaced by a parent's entry
		// writes the edge onto the parent's file.
		return a.supersedeRefuse("%s belongs to another ledger, and dira never writes to one (cst-0003 rule 1). "+
			"Supersede it in the ledger that owns it", ref)
	}
	for _, ref := range []string{target, with} {
		if !ledger.ValidID(ref) {
			return a.supersedeMisuse("%q is not an entry id (ids look like dec-0060)", ref)
		}
	}
	if target == with {
		return a.supersedeRefuse("%s cannot supersede itself", target)
	}

	kind, _ := ledger.KindForPrefix(strings.SplitN(target, "-", 2)[0])
	if !canBeSuperseded(kind) {
		return a.supersedeRefuse("a %s has no superseded state — the schema gives it only %s — so %s cannot be superseded. "+
			"A question is answered rather than retired", kind, joinStates(kind.States()), target)
	}
	if kind != mustKindOf(with) {
		return a.supersedeRefuse("%s is a %s and %s is a %s; a supersedes edge replaces an entry with another of its own kind, "+
			"and a cross-kind one would leave the record saying a %s is now a %s",
			target, kind, with, mustKindOf(with), kind, mustKindOf(with))
	}
	return nil
}

// supersede performs the two writes, edge first.
func (a *app) supersede(target, with, note, dir string) error {
	store, _, err := openLedger(dir)
	if err != nil {
		return err
	}

	ctx := context.Background()
	superseder, err := store.Get(ctx, with)
	if err != nil {
		return err
	}
	retired, err := store.Get(ctx, target)
	if err != nil {
		return err
	}

	if err := a.checkStates(retired, superseder); err != nil {
		return err
	}

	wrote, err := a.writeSupersession(ctx, store, retired, superseder, note)
	if err != nil {
		return err
	}
	if !wrote {
		// Nothing to do, and that is not a failure: it is what the
		// second run after a crash looks like once the first one landed.
		_, _ = fmt.Fprintf(a.stderr, "%s is already superseded by %s; nothing written\n", retired.ID, superseder.ID)
		_, _ = fmt.Fprintln(a.stdout, retired.ID)
		return nil
	}

	_, _ = fmt.Fprintf(a.stderr, "%s is superseded by %s; dira check now cites %s in its place\n",
		retired.ID, superseder.ID, superseder.ID)
	// The retired id on stdout, so a caller can pipe it. It is the entry
	// whose enforcement changed, which is what this command is for.
	_, _ = fmt.Fprintln(a.stdout, retired.ID)
	return nil
}

// writeSupersession performs the two writes and reports whether either of them
// had anything to do.
//
// It takes the Store rather than finding one so that the order of the two
// writes, and what a failure of the second one leaves behind, can be observed
// directly by a test — see TestTheEdgeIsWrittenBeforeTheState. The order is the
// one decision in this command that only shows up in a crash, so leaving it to
// be verified by reading the code would leave it unverified.
func (a *app) writeSupersession(ctx context.Context, store ledger.Store, retired, superseder *ledger.Entry, note string) (bool, error) {
	// Write one: the edge onto the entry that replaces. See the failure-mode
	// note at the top of this file for why this write goes first.
	edge := ledger.Edge{Type: ledger.EdgeSupersedes, To: retired.ID, Note: note}
	added, err := ledger.AddEdge(superseder, edge)
	if err != nil {
		return false, err
	}
	if added {
		if err := a.put(ctx, store, superseder); err != nil {
			return false, err
		}
	}

	// Write two: the state on the entry that was replaced. Reaching here
	// with the state already flipped is a repeat run, not a failure — the
	// edge above was a no-op too, and both together are what a second
	// invocation after a crash has to be able to do.
	if retired.State == ledger.StateSuperseded {
		return added, nil
	}
	retired.State = ledger.StateSuperseded
	if err := a.put(ctx, store, retired); err != nil {
		return false, fmt.Errorf("%w\n\t%s now records that it supersedes %s, and %s is still enforced. "+
			"Run the same command again to finish", err, superseder.ID, retired.ID, retired.ID)
	}
	return true, nil
}

// checkStates refuses the pairings that would make the ledger say something
// false, once both entries have been read.
func (a *app) checkStates(retired, superseder *ledger.Entry) error {
	if superseder.State == ledger.StateSuperseded {
		return a.supersedeRefuse("%s is itself superseded, so putting it in %s's place would retire one entry "+
			"in favour of another that is already retired", superseder.ID, retired.ID)
	}
	if superseder.State == ledger.StateStaged {
		return a.supersedeRefuse("%s is staged — an unconfirmed capture, not a settled decision (dec-0003) — "+
			"so it cannot retire %s. Accept it first", superseder.ID, retired.ID)
	}
	if retired.State == ledger.StateSuperseded {
		for _, edge := range superseder.Edges {
			if edge.Type == ledger.EdgeSupersedes && edge.To == retired.ID {
				return nil
			}
		}
		return a.supersedeRefuse("%s is already superseded; `dira why %s` names what replaced it. "+
			"An entry is replaced once, and by one entry", retired.ID, retired.ID)
	}
	return nil
}

// put stamps updated and writes one entry.
//
// The whole entry is validated before the write rather than the field that
// changed, for the reason the creation path does it: a mutation that produces an
// entry the schema rejects is reported as the caller's mistake it is, with this
// command's usage, instead of as a failure of the write.
func (a *app) put(ctx context.Context, store ledger.Store, e *ledger.Entry) error {
	e.Updated = a.now().UTC().Format(time.RFC3339)
	if err := e.Validate(); err != nil {
		return a.supersedeMisuse("%w", err)
	}
	return store.Put(ctx, e)
}

// canBeSuperseded asks the schema, through ledger.Kind.States, whether a kind
// has a superseded state at all. A question does not — it is answered, and the
// schema gives it nowhere else to go — and neither does an intent or a note.
func canBeSuperseded(k ledger.Kind) bool {
	for _, s := range k.States() {
		if s == ledger.StateSuperseded {
			return true
		}
	}
	return false
}

// mustKindOf is the kind an id's prefix encodes. An id that reached here has
// already been through ledger.ValidID, so the prefix is one of the five.
func mustKindOf(id string) ledger.Kind {
	k, _ := ledger.KindForPrefix(strings.SplitN(id, "-", 2)[0])
	return k
}

// joinStates renders a kind's states for a message.
func joinStates(states []ledger.State) string {
	parts := make([]string, len(states))
	for i, s := range states {
		parts[i] = string(s)
	}
	return strings.Join(parts, " and ")
}

// writeSupersedeUsage renders `dira supersede -h`.
func writeSupersedeUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira supersede - retire an entry in favour of the one that replaces it\n\n")
	b.WriteString("usage:\n\n\tdira supersede <id> --with <id> [flags]\n\n")

	b.WriteString("Writes both sides of the supersession: the replacing entry gains a\n")
	b.WriteString("`supersedes` edge, and the replaced entry's state becomes `superseded`.\n")
	b.WriteString("`dira check` stops citing the retired entry and reports a match against\n")
	b.WriteString("it as a redirect to its replacement instead.\n\n")

	b.WriteString("flags:\n\n")
	for _, line := range [][2]string{
		{"--with ID", "the entry that replaces it; required"},
		{"--note TEXT", "one line on why the replacement happened"},
		{"-C DIR", "run as if started in this directory"},
	} {
		fmt.Fprintf(&b, "\t%-14s  %s\n", line[0], line[1])
	}

	b.WriteString("\nOnly a decision or a constraint can be superseded: the schema gives no\n")
	b.WriteString("other kind a `superseded` state. An entry in another ledger cannot be\n")
	b.WriteString("superseded from here — dira never writes upward (cst-0003).\n\n")

	b.WriteString("The two writes are not atomic and cannot be: dira's storage interface\n")
	b.WriteString("has no transaction, because none exists over the GitHub Contents API.\n")
	b.WriteString("The edge is written first, so an interruption leaves the retired entry\n")
	b.WriteString("still enforced rather than silently unenforced. Running the same\n")
	b.WriteString("command again finishes the job.\n\n")

	b.WriteString("exit codes:\n\n")
	b.WriteString("\t0  the supersession is recorded — this run wrote it, or it was already\n")
	b.WriteString("\t2  the ledger refuses it: a rule in the record says no\n")
	b.WriteString("\t1  dira did not get that far — an unusable command line, an entry\n")
	b.WriteString("\t   that is not there, an unreadable ledger, a write that failed\n\n")

	b.WriteString("A caller may read 2 as \"refused on policy\", and only that: an entry in\n")
	b.WriteString("a parent ledger, a kind the schema gives no superseded state, an entry\n")
	b.WriteString("already replaced, a staged replacement. A bad flag, a missing entry and\n")
	b.WriteString("a failed write are all 1, so 2 never means dira is broken. This is the\n")
	b.WriteString("same split `dira check` makes, where 2 is a verdict about the plan and\n")
	b.WriteString("never a mistake in the flags — across both commands 2 is the record's\n")
	b.WriteString("answer and 1 is never a verdict.\n")

	_, _ = io.WriteString(w, b.String())
}
