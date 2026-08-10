package distill

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/schema"
)

// sessionNow is the clock every case here stamps with, for the reason loopNow
// and disposeNow are: `updated` goes into the permanent record.
var sessionNow = time.Date(2026, 8, 2, 11, 5, 0, 0, time.UTC)

// budget is the acceptance line's "within 1 second". It is the deadline the
// non-interactive case is measured against and the deadline the deliberately
// hanging control is required to blow.
const budget = time.Second

// TestNonInteractive is E2-L4-T7.
//
// # What a "does not hang" test has to prove before it proves anything
//
// A test that runs a command and finishes is not evidence that the command
// cannot hang: a test harness with no deadline finishes on a hang too, by
// waiting for it, and a harness with a deadline that nothing has ever tripped is
// indistinguishable from one whose deadline does not work. Both of those pass
// happily against the exact bug this task exists to prevent.
//
// So the first case below runs the *same* harness twice over the *same*
// deliberately blocking key source, changing one thing: whether the injected
// terminal says a human is present. With Interactive false the session returns
// inside the budget having read nothing; with Interactive true the session
// blocks and the harness reports the timeout. The second half is the load-bearing
// one. It shows that this harness detects a hang, and it shows that the injected
// answer is what decides — the green half is not a session that was never going
// to read on any input, it is a session that would have read and did not
// (docs/lore.md L-0001).
//
// Every other case here is two-sided in the same way. Where the assertion is
// "nothing was written", the same digest is shown moving under a write that did
// happen; where it is "the terminal was restored", the same matched-restore
// check is shown reporting a terminal that was not; where it is "every file
// still validates", the same validator is shown rejecting a half-written one.
func TestNonInteractive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("stdin is not a terminal, so nothing is read and nothing is changed", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"), sniffShaped("dec-0002"), sniffShaped("dec-0003"))
		before := ledgerDigest(t, entriesDir)

		keys := blocking()
		term := newTerminal(keys).offline()
		display := &safeScreen{}

		var result *SessionResult
		var err error
		finished, done := completesWithin(budget, func() {
			result, err = Run(ctx, SessionOptions{
				Store:    store,
				Terminal: term,
				Display:  display,
				Now:      func() time.Time { return sessionNow },
			})
		})
		if !finished {
			keys.unblock()
			<-done
			t.Fatal("the session did not return within the budget on a stdin that is not a terminal; " +
				"this is the hang that makes dira unusable from a hook")
		}
		<-done

		if err != nil {
			t.Fatalf("Run on a non-interactive stdin: %v; the exit has to be 0 or the hook that called it fails", err)
		}
		if result.Interactive || result.EnteredRaw {
			t.Errorf("result = %+v; the terminal was never a terminal, so raw mode must never have been entered", result)
		}
		if moves := term.moveLog(); len(moves) != 0 {
			t.Errorf("the terminal recorded %v; a non-interactive session must not touch the terminal's mode", moves)
		}
		if got := keys.count(); got != 0 {
			t.Errorf("the session asked for %d keystrokes; on a pipe it must ask for none — a hook's stdin is "+
				"somebody else's payload, not an empty queue of bytes", got)
		}
		if result.Waiting != 3 {
			t.Errorf("result.Waiting = %d, want 3; the session did not read the queue it declined to work", result.Waiting)
		}

		// Nothing was written. The digest's own control is at the end of
		// this case.
		if after := ledgerDigest(t, entriesDir); after != before {
			t.Errorf("the ledger changed on a path that reads no input:\n  before %s\n  after  %s", before, after)
		}
		assertLedgerIntact(t, entriesDir)

		// Exactly one line, and it is the line the result reports.
		assertExactlyOneLine(t, display.shown(), result.Summary)
		if !strings.Contains(result.Summary, "3 captures") {
			t.Errorf("the line is %q; it does not say how many captures are waiting", result.Summary)
		}

		// --- the control that makes the case above mean something ------ //
		//
		// The same ledger shape, the same blocking key source, the same
		// harness and the same budget. One thing changes: the terminal
		// says a human is present. If this finishes, the harness cannot
		// detect a hang and every "does not hang" assertion in this file
		// is decoration.
		store2, _, _ := tempLedger(t)
		put(t, store2, sniffShaped("dec-0001"), sniffShaped("dec-0002"), sniffShaped("dec-0003"))
		hanging := blocking()
		online := newTerminal(hanging)
		finished, done = completesWithin(budget, func() {
			_, _ = Run(ctx, SessionOptions{
				Store:    store2,
				Terminal: online,
				Display:  &safeScreen{},
				Now:      func() time.Time { return sessionNow },
			})
		})
		if finished {
			t.Fatal("the deliberately blocking session returned within the budget, so this harness cannot tell a " +
				"hang from a clean exit and the case above proves nothing")
		}
		if got := hanging.count(); got != 1 {
			t.Errorf("the blocking driver was asked for %d keystrokes, want 1; the control is not blocked on a read", got)
		}
		if moves := online.moveLog(); len(moves) != 1 || moves[0] != rawMode {
			t.Errorf("the hanging session's terminal recorded %v, want exactly [%s]; it is blocked somewhere other "+
				"than a raw-mode read", moves, rawMode)
		}
		hanging.unblock()
		<-done
		if moves := online.moveLog(); len(moves) != 2 || moves[1] != cookedMode {
			t.Errorf("after the read was released the terminal recorded %v; even the hanging path has to be "+
				"restored once it ends", moves)
		}

		// The digest's control: a write to this ledger does move it, so
		// "unchanged" above is a fact about the session and not about the
		// helper.
		card, err := store.Get(ctx, "dec-0001")
		if err != nil {
			t.Fatalf("re-reading dec-0001: %v", err)
		}
		if _, err := Confirm(ctx, store, card, sessionNow); err != nil {
			t.Fatalf("Confirm: %v", err)
		}
		if ledgerDigest(t, entriesDir) == before {
			t.Error("the digest is unchanged after a write that did happen; it is not measuring the ledger")
		}
	})

	t.Run("a zero-length queue costs one line and no input at all", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name  string
			setup func(t *testing.T, store ledger.Store)
		}{
			{"an empty ledger", func(*testing.T, ledger.Store) {}},
			{"a ledger with nothing staged", func(t *testing.T, store ledger.Store) {
				put(t, store,
					decisionIn("dec-0001", "postgres over sqlite", ledger.StateAccepted),
					decisionIn("dec-0002", "run payroll in house", ledger.StateRejected))
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				store, _, entriesDir := tempLedger(t)
				tc.setup(t, store)
				before := ledgerDigest(t, entriesDir)

				// A terminal that *is* interactive, so "read no
				// input" is a fact about the empty queue rather
				// than about there being no human.
				keys := blocking()
				term := newTerminal(keys)
				display := &safeScreen{}

				var result *SessionResult
				var err error
				finished, done := completesWithin(budget, func() {
					result, err = Run(ctx, SessionOptions{
						Store:    store,
						Terminal: term,
						Display:  display,
						Now:      func() time.Time { return sessionNow },
					})
				})
				if !finished {
					keys.unblock()
					<-done
					t.Fatal("the session blocked on an empty queue")
				}
				<-done

				if err != nil {
					t.Fatalf("Run over an empty queue: %v", err)
				}
				if result.Waiting != 0 || result.Pending != 0 {
					t.Errorf("result = %+v; this ledger holds no staged entry at all", result)
				}
				if got := keys.count(); got != 0 {
					t.Errorf("the session asked for %d keystrokes over an empty queue, want none", got)
				}
				if result.EnteredRaw {
					t.Error("raw mode was entered for a queue with no card in it")
				}
				if moves := term.moveLog(); len(moves) != 0 {
					t.Errorf("the terminal recorded %v over an empty queue", moves)
				}
				assertExactlyOneLine(t, display.shown(), result.Summary)
				if result.Summary != "dira distill: nothing staged" {
					t.Errorf("the line is %q", result.Summary)
				}
				if after := ledgerDigest(t, entriesDir); after != before {
					t.Error("an empty queue wrote to the ledger")
				}
				assertLedgerIntact(t, entriesDir)
			})
		}

		// The control for both: the same terminal and the same harness
		// over a ledger that does hold a card enters raw mode and reads
		// exactly one key. Without it, "reads no input" is green against
		// a session that never reads anything.
		store, _, _ := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"))
		keys := scripted("q")
		term := newTerminal(keys)
		result, err := Run(ctx, SessionOptions{
			Store:    store,
			Terminal: term,
			Display:  &safeScreen{},
			Now:      func() time.Time { return sessionNow },
		})
		if err != nil {
			t.Fatalf("Run over a queue with one card: %v", err)
		}
		if !result.EnteredRaw || keys.calls != 1 {
			t.Errorf("with one card the session entered raw mode %v and read %d keys, want true and 1; "+
				"the empty-queue assertions above are measuring nothing", result.EnteredRaw, keys.calls)
		}
		assertMatchedRestore(t, term.moveLog())
	})

	t.Run("an empty queue still names what is waiting on extraction", func(t *testing.T) {
		t.Parallel()

		store, _, _ := tempLedger(t)
		put(t, store, confirmedCapture("dec-0001"))

		// One promoted entry: no card, but dec-0022 requires the state to
		// be visible or entries pile up in `staged` looking rejected.
		result := runQuietly(t, ctx, store, newTerminal(blocking()))
		if result.Waiting != 0 || result.Pending != 1 {
			t.Fatalf("result = %+v, want no cards and one entry pending extraction", result)
		}
		if !strings.Contains(result.Summary, "1 entry confirmed and waiting on extraction") {
			t.Errorf("the line is %q; a promoted entry it does not mention is one that silently piles up", result.Summary)
		}

		// And the plural, because "1 entries" in the line a demo
		// screenshots is a line somebody has to fix later.
		put(t, store, confirmedCapture("dec-0002"))
		result = runQuietly(t, ctx, store, newTerminal(blocking()))
		if !strings.Contains(result.Summary, "2 entries confirmed and waiting on extraction") {
			t.Errorf("with two pending the line is %q", result.Summary)
		}
	})

	t.Run("a warning is additional to the one line, and the line is still one line", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, decisionIn("dec-0001", "postgres over sqlite", ledger.StateAccepted))
		broken := "---\nid: dec-0002\nkind: decision\ntitle: \"unterminated\nstate: staged\n---\n"
		if err := os.WriteFile(filepath.Join(entriesDir, "dec-0002.md"), []byte(broken), 0o644); err != nil {
			t.Fatalf("corrupting dec-0002.md: %v", err)
		}

		display := &safeScreen{}
		result, err := Run(ctx, SessionOptions{
			Store:    store,
			Terminal: newTerminal(blocking()),
			Display:  display,
			Now:      func() time.Time { return sessionNow },
		})
		if err != nil {
			t.Fatalf("Run over a ledger with an unreadable file: %v", err)
		}

		// The acceptance clause counts one line, and it is one line: the
		// warning is an extra, and it is an extra deliberately. An entry
		// nobody can parse is an entry missing from every answer, and a
		// session that swallowed the warning to keep its output to one
		// line would be hiding exactly that.
		lines := display.shown()
		if len(lines) != 2 {
			t.Fatalf("the session showed %d lines, want the warning and the summary:\n%v", len(lines), lines)
		}
		if !strings.Contains(lines[0], "dec-0002") {
			t.Errorf("the first line is %q, want the warning naming the file that could not be read", lines[0])
		}
		if lines[1] != result.Summary || strings.Contains(result.Summary, "\n") {
			t.Errorf("the last line is %q and the summary is %q; the answer must be last and must be one line",
				lines[1], result.Summary)
		}
		if len(result.Warnings) != 1 {
			t.Errorf("result.Warnings = %v, want one", result.Warnings)
		}
	})

	t.Run("raw mode is restored on every exit path", func(t *testing.T) {
		t.Parallel()

		// The check has to be able to fail before any of its passes mean
		// anything, and the failing input is a real observation rather
		// than a hand-written list: a terminal put into raw mode by the
		// same fake, with the Restore it handed back never called.
		forgotten := newTerminal(scripted("q"))
		if _, _, err := forgotten.Raw(); err != nil {
			t.Fatalf("Raw on the fake terminal: %v", err)
		}
		if matchedRestore(forgotten.moveLog()) == nil {
			t.Fatal("the matched-restore check accepts a terminal left in raw mode; every use of it below is " +
				"green against a session that never restores anything")
		}
		if matchedRestore([]string{rawMode, cookedMode}) != nil {
			t.Fatal("the matched-restore check rejects a matched pair, so it cannot pass on a correct session")
		}

		readError := errors.New("input/output error")

		for _, tc := range []struct {
			name string
			// staged is how many captures the ledger holds.
			staged int
			// keys drives the session.
			keys func(cancel context.CancelFunc) KeySource
			// render is nil except where the exit path is a panic.
			render Renderer
			// check grades the error the session ended on.
			check func(t *testing.T, err error)
		}{
			{
				name:   "the queue empties",
				staged: 1,
				keys:   func(context.CancelFunc) KeySource { return scripted("y") },
				check:  func(t *testing.T, err error) { requireNoError(t, err) },
			},
			{
				name:   "the human presses q",
				staged: 2,
				keys:   func(context.CancelFunc) KeySource { return scripted("yq") },
				check:  func(t *testing.T, err error) { requireNoError(t, err) },
			},
			{
				name:   "the driver reaches EOF",
				staged: 2,
				keys:   func(context.CancelFunc) KeySource { return scripted("") },
				check:  func(t *testing.T, err error) { requireNoError(t, err) },
			},
			{
				name:   "the read fails",
				staged: 1,
				keys:   func(context.CancelFunc) KeySource { return failingKeys(readError) },
				check: func(t *testing.T, err error) {
					if !errors.Is(err, readError) {
						t.Errorf("the session ended on %v, want the read error", err)
					}
					if errors.Is(err, ErrInterrupted) {
						t.Error("an ordinary read failure was reported as an interrupt; the two are not " +
							"distinguishable and the interrupt case below proves nothing")
					}
				},
			},
			{
				name:   "a signal cuts the read short",
				staged: 2,
				keys: func(context.CancelFunc) KeySource {
					return afterOneKey('y', fmt.Errorf("read: %w", ErrInterrupted))
				},
				check: func(t *testing.T, err error) {
					if !errors.Is(err, ErrInterrupted) {
						t.Errorf("the session ended on %v, want an interrupt", err)
					}
				},
			},
			{
				name:   "the context is cancelled mid-session",
				staged: 2,
				keys:   func(cancel context.CancelFunc) KeySource { return cancellingKeys(cancel, 'y') },
				check: func(t *testing.T, err error) {
					if !errors.Is(err, ErrInterrupted) || !errors.Is(err, context.Canceled) {
						t.Errorf("the session ended on %v, want an interrupt naming the cancelled context", err)
					}
				},
			},
			{
				name:   "the renderer panics",
				staged: 1,
				keys:   func(context.CancelFunc) KeySource { return scripted("y") },
				render: func(Item, int, int) string { panic("a renderer that cannot render") },
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				store, _, entriesDir := tempLedger(t)
				for i := 1; i <= tc.staged; i++ {
					put(t, store, sniffShaped(fmt.Sprintf("dec-%04d", i)))
				}

				caseCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				term := newTerminal(tc.keys(cancel))

				var err error
				panicked := false
				func() {
					defer func() {
						if r := recover(); r != nil {
							panicked = true
						}
					}()
					_, err = Run(caseCtx, SessionOptions{
						Store:    store,
						Terminal: term,
						Display:  &safeScreen{},
						Now:      func() time.Time { return sessionNow },
						Render:   tc.render,
					})
				}()

				if (tc.render != nil) != panicked {
					t.Fatalf("panicked = %v for a case whose renderer is %v; the exit path under test is not "+
						"the one that ran", panicked, tc.render != nil)
				}
				if tc.check != nil {
					tc.check(t, err)
				}

				moves := term.moveLog()
				if len(moves) == 0 {
					t.Fatalf("the terminal recorded nothing; this case never entered raw mode, so it is not "+
						"testing an exit path out of it (%d staged)", tc.staged)
				}
				assertMatchedRestore(t, moves)

				// The other half of this task's last clause: no
				// exit path leaves a half-written entry behind.
				assertLedgerIntact(t, entriesDir)
			})
		}
	})

	t.Run("a refused write leaves the terminal restored and the ledger intact", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"))
		before := ledgerDigest(t, entriesDir)

		refused := errors.New("the ledger is read-only")
		term := newTerminal(scripted("yq"))
		display := &safeScreen{}
		result, err := Run(ctx, SessionOptions{
			Store:    refusingStore{Store: store, putErr: refused},
			Terminal: term,
			Display:  display,
			Now:      func() time.Time { return sessionNow },
		})
		if err != nil {
			t.Fatalf("Run: %v; a write the ledger refused is reported on the display, not by ending the session", err)
		}
		if len(result.Loop.Dispositions) != 0 {
			t.Errorf("the session recorded %v; the write was refused", acts(result.Loop))
		}
		if !strings.Contains(strings.Join(display.shown(), "\n"), refused.Error()) {
			t.Errorf("the refusal was not shown:\n%v", display.shown())
		}
		if ledgerDigest(t, entriesDir) != before {
			t.Error("a refused write changed the ledger")
		}
		assertMatchedRestore(t, term.moveLog())
		assertLedgerIntact(t, entriesDir)
	})

	t.Run("a terminal that cannot enter raw mode is an error, not a hang", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"))
		before := ledgerDigest(t, entriesDir)

		refused := errors.New("inappropriate ioctl for device")
		keys := blocking()
		term := newTerminal(keys)
		term.rawErr = refused

		var err error
		var result *SessionResult
		finished, done := completesWithin(budget, func() {
			result, err = Run(ctx, SessionOptions{
				Store:    store,
				Terminal: term,
				Display:  &safeScreen{},
				Now:      func() time.Time { return sessionNow },
			})
		})
		if !finished {
			keys.unblock()
			<-done
			t.Fatal("a terminal that refused raw mode left the session blocked")
		}
		<-done

		if !errors.Is(err, refused) {
			t.Errorf("Run returned %v, want the terminal's refusal", err)
		}
		if result.EnteredRaw {
			t.Error("result says raw mode was entered by a Raw that failed")
		}
		if got := keys.count(); got != 0 {
			t.Errorf("the session read %d keystrokes through a terminal it never put into raw mode; the "+
				"KeySource is reachable without Raw succeeding", got)
		}
		if moves := term.moveLog(); len(moves) != 0 {
			t.Errorf("the terminal recorded %v for a mode change that failed", moves)
		}
		if ledgerDigest(t, entriesDir) != before {
			t.Error("a session that never reached the loop wrote to the ledger")
		}
		assertLedgerIntact(t, entriesDir)
	})

	t.Run("a failed restore is reported rather than dropped", func(t *testing.T) {
		t.Parallel()

		store, _, _ := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"))

		broken := errors.New("could not put the terminal back")
		term := newTerminal(scripted("q"))
		term.restoreErr = broken

		if _, err := Run(ctx, SessionOptions{
			Store:    store,
			Terminal: term,
			Display:  &safeScreen{},
			Now:      func() time.Time { return sessionNow },
		}); !errors.Is(err, broken) {
			t.Errorf("Run returned %v; a terminal left in raw mode is the caller's business", err)
		}

		// And it does not displace the error that explains why the
		// session ended, which is the one worth reading.
		readError := errors.New("input/output error")
		term2 := newTerminal(failingKeys(readError))
		term2.restoreErr = broken
		_, err := Run(ctx, SessionOptions{
			Store:    store,
			Terminal: term2,
			Display:  &safeScreen{},
			Now:      func() time.Time { return sessionNow },
		})
		if !errors.Is(err, readError) {
			t.Errorf("Run returned %v, want the read failure the session ended on", err)
		}
	})

	t.Run("the validator can tell a half-written entry from a whole one", func(t *testing.T) {
		t.Parallel()

		store, _, entriesDir := tempLedger(t)
		put(t, store, sniffShaped("dec-0001"), sniffShaped("dec-0002"))
		path := filepath.Join(entriesDir, "dec-0001.md")
		whole := readFile(t, path)

		// Green: this is what a ledger the session left alone looks like.
		if faults := ledgerFaults(t, entriesDir); len(faults) != 0 {
			t.Fatalf("the scan reports %v over a ledger written through the store", faults)
		}

		// Red, on disk rather than in a string: a write cut off partway
		// through is exactly what "partially written" means, and the same
		// scan every case above uses has to name it. Without this, every
		// assertLedgerIntact in this file is green against a scan that
		// walks nothing or accepts everything.
		if err := os.WriteFile(path, []byte(whole[:len(whole)/2]), 0o644); err != nil {
			t.Fatalf("truncating %s: %v", path, err)
		}
		if faults := ledgerFaults(t, entriesDir); !reflect.DeepEqual(faults, []string{"dec-0001.md"}) {
			t.Fatalf("the scan reports %v over a ledger holding one file truncated halfway, want "+
				"[dec-0001.md]", faults)
		}
	})

	t.Run("the copy is one line, and in the register the design asks for", func(t *testing.T) {
		t.Parallel()

		lines := map[string]string{
			"nothing staged":           nothingWaiting(0),
			"one pending":              nothingWaiting(1),
			"several pending":          nothingWaiting(2),
			"one capture, no tty":      notATerminal(1),
			"several captures, no tty": notATerminal(3),
		}
		for name, line := range lines {
			if line == "" || strings.Contains(line, "\n") {
				t.Errorf("%s: %q is not exactly one line", name, line)
			}
			if got := unhyped.FindString(line); got != "" {
				t.Errorf("%s: %q contains %q", name, line, got)
			}
		}
		// The pattern's control: it matches what it is looking for, or
		// its silence above is not a finding.
		for _, decoy := range []string{"successfully staged!", "AI-powered review", "a seamless 10x flow"} {
			if !unhyped.MatchString(decoy) {
				t.Fatalf("the register pattern does not match %q, so it cannot have found one above", decoy)
			}
		}

		// The plurals, spelled out because the wrong one is the kind of
		// thing that survives into a screenshot.
		for line, want := range map[string]string{
			lines["one pending"]:              "1 entry ",
			lines["several pending"]:          "2 entries ",
			lines["one capture, no tty"]:      "1 capture ",
			lines["several captures, no tty"]: "3 captures ",
		} {
			if !strings.Contains(line, want) {
				t.Errorf("%q does not contain %q", line, want)
			}
		}
	})

	t.Run("the session surface takes no reader, writer or terminal handle", func(t *testing.T) {
		t.Parallel()

		// Pinned by compilation, as T4 and T5 pin theirs: a conversion
		// between two function types compiles only while their parameters
		// and results are identical.
		_ = runSignature(Run)

		surface := map[string]reflect.Type{
			"Run":            reflect.TypeOf(Run),
			"SessionOptions": reflect.TypeOf(SessionOptions{}),
			"SessionResult":  reflect.TypeOf(SessionResult{}),
			"Terminal":       reflect.TypeOf((*Terminal)(nil)).Elem(),
			"Restore":        reflect.TypeOf(Restore(nil)),
		}
		for name, typ := range surface {
			if offending := terminalTypesDeep(typ); len(offending) != 0 {
				t.Errorf("%s reaches %v. A terminal handle here would be the whole of dec-0005's boundary "+
					"broken in the one package that is allowed to know a human exists — and E6's web "+
					"screen, which has no descriptors, would have to fake one", name, offending)
			}
		}
		// The deep walk's control, so the silence above is a finding: a
		// Terminal that did carry an *os.File is reported.
		type withHandle interface {
			Interactive() bool
			Raw() (*os.File, error)
		}
		if offending := terminalTypesDeep(reflect.TypeOf((*withHandle)(nil)).Elem()); len(offending) != 1 {
			t.Errorf("the deep walk found %v in an interface whose method returns an *os.File; it is not "+
				"walking this shape and the check above is vacuous", offending)
		}
	})
}

// --- the terminal fake ------------------------------------------------------ //

// The two mode transitions the fake records. They are the whole of the evidence
// for "restored on every exit path": a session that entered raw mode and did not
// leave it shows one of the first with no matching second.
const (
	rawMode    = "raw"
	cookedMode = "restore"
)

// fakeTerminal is a Terminal that records what was done to it.
//
// It is mutex-guarded because the timeout harness runs Run on another goroutine
// and the test reads the log while a deliberately blocked session is still
// holding it open — which is exactly the state the hang control inspects.
type fakeTerminal struct {
	mu          sync.Mutex
	interactive bool
	source      KeySource
	rawErr      error
	restoreErr  error
	moves       []string
}

func newTerminal(source KeySource) *fakeTerminal {
	return &fakeTerminal{interactive: true, source: source}
}

// offline is a terminal that is not one: a pipe, a hook's stdin, a CI step.
func (f *fakeTerminal) offline() *fakeTerminal {
	f.interactive = false
	return f
}

func (f *fakeTerminal) Interactive() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.interactive
}

// Raw records the transition only when it succeeds. A mode change that failed
// did not happen, and recording it would make the matched-restore check demand a
// restore for a terminal that was never touched.
func (f *fakeTerminal) Raw() (KeySource, Restore, error) {
	f.mu.Lock()
	rawErr, restoreErr, source := f.rawErr, f.restoreErr, f.source
	if rawErr == nil {
		f.moves = append(f.moves, rawMode)
	}
	f.mu.Unlock()

	if rawErr != nil {
		return nil, nil, rawErr
	}
	return source, func() error {
		f.mu.Lock()
		f.moves = append(f.moves, cookedMode)
		f.mu.Unlock()
		return restoreErr
	}, nil
}

func (f *fakeTerminal) moveLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.moves...)
}

// --- key sources ------------------------------------------------------------ //

// blockingKeys is the hang, made deliberate: a read that never returns. It is
// what the non-interactive path must never reach and what the control in the
// first case does reach, so that the harness is shown detecting one.
type blockingKeys struct {
	gate  chan struct{}
	reads atomic.Int64
	once  sync.Once
}

func blocking() *blockingKeys { return &blockingKeys{gate: make(chan struct{})} }

func (b *blockingKeys) ReadKey() (byte, error) {
	b.reads.Add(1)
	<-b.gate
	return 0, io.EOF
}

func (b *blockingKeys) count() int { return int(b.reads.Load()) }

// unblock releases a blocked read so the goroutine holding it can finish. A test
// that saw the timeout calls it and then waits, rather than leaving a goroutine
// wedged for the rest of the package's run.
func (b *blockingKeys) unblock() { b.once.Do(func() { close(b.gate) }) }

// keysFunc adapts a function to a KeySource, for the cases whose whole content
// is what the read does.
type keysFunc func() (byte, error)

func (f keysFunc) ReadKey() (byte, error) { return f() }

func failingKeys(err error) KeySource {
	return keysFunc(func() (byte, error) { return 0, err })
}

// cancellingKeys is Ctrl-C arriving while the human was pressing a key: the byte
// lands, the session is cancelled as it does, and the driver has nothing further
// to say.
//
// The second read being an ordinary EOF rather than another byte is what gives
// this case a failing side. A session that did not check the context would take
// that EOF as the human having stopped answering and report a clean quit, which
// is a different and wrong account of what happened — and it is the account the
// case gets if the check is removed.
func cancellingKeys(cancel context.CancelFunc, key byte) KeySource {
	delivered := false
	return keysFunc(func() (byte, error) {
		if delivered {
			return 0, io.EOF
		}
		delivered = true
		cancel()
		return key, nil
	})
}

// afterOneKey delivers one byte and then fails, which is what a signal arriving
// mid-session looks like from inside the loop.
func afterOneKey(key byte, err error) KeySource {
	delivered := false
	return keysFunc(func() (byte, error) {
		if !delivered {
			delivered = true
			return key, nil
		}
		return 0, err
	})
}

// --- store wrappers --------------------------------------------------------- //

// refusingStore is a ledger whose writes fail, for the exit path where a
// disposition cannot be performed. Everything else passes through, so the queue
// still reads and the card is still shown.
type refusingStore struct {
	ledger.Store
	putErr error
}

func (s refusingStore) Put(context.Context, *ledger.Entry) error { return s.putErr }

// --- the timeout harness ---------------------------------------------------- //

// completesWithin runs fn on its own goroutine and reports whether it returned
// inside d.
//
// The second return closes when fn eventually returns. A caller that saw a
// timeout releases whatever fn is blocked on and then waits on it, so a case
// that deliberately hangs does not leave a goroutine holding a store for the
// rest of the run.
func completesWithin(d time.Duration, fn func()) (bool, <-chan struct{}) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()

	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-done:
		return true, done
	case <-timer.C:
		return false, done
	}
}

// --- assertions ------------------------------------------------------------- //

// matchedRestore reports whether a terminal's transitions balance: every raw
// mode entered is left, and none is left that was not entered.
//
// It returns an error rather than failing, so a test can assert that it *does*
// fail on a terminal left in raw mode. A check nobody has watched reject
// anything is a check that might accept everything (L-0001).
func matchedRestore(moves []string) error {
	depth := 0
	for i, move := range moves {
		switch move {
		case rawMode:
			depth++
		case cookedMode:
			depth--
		default:
			return fmt.Errorf("unknown transition %q at %d in %v", move, i, moves)
		}
		if depth < 0 {
			return fmt.Errorf("the terminal was restored without having been put into raw mode: %v", moves)
		}
	}
	if depth != 0 {
		return fmt.Errorf("the terminal was left in raw mode: %v", moves)
	}
	return nil
}

func assertMatchedRestore(t *testing.T, moves []string) {
	t.Helper()

	if err := matchedRestore(moves); err != nil {
		t.Errorf("%v. A shell left in raw mode is one the human has to `reset` by hand, and the exit path that "+
			"did it is the one nobody was watching", err)
	}
}

// assertExactlyOneLine is the acceptance clause "printing exactly one line",
// asserted over what the Display received and cross-checked against what the
// session says it printed.
func assertExactlyOneLine(t *testing.T, shown []string, summary string) {
	t.Helper()

	if len(shown) != 1 {
		t.Fatalf("the session showed %d things, want exactly one line:\n%v", len(shown), shown)
	}
	if strings.Contains(shown[0], "\n") {
		t.Errorf("the one thing shown is %d lines:\n%s", strings.Count(shown[0], "\n")+1, shown[0])
	}
	if shown[0] != summary {
		t.Errorf("the session showed %q and reports %q; the line and the result are two different strings",
			shown[0], summary)
	}
}

// assertLedgerIntact is the last clause: no exit path leaves a partially written
// entry file.
func assertLedgerIntact(t *testing.T, entriesDir string) {
	t.Helper()

	if faults := ledgerFaults(t, entriesDir); len(faults) != 0 {
		t.Errorf("the ledger holds %v after the session; an exit path left an entry a consumer of this ledger "+
			"cannot read", faults)
	}
}

// ledgerFaults names every file in the ledger that does not satisfy
// entry.schema.json — the published contract a consumer reads, not dira's own
// runtime gate over the same rules.
//
// It returns the names rather than failing, so the scan itself can be pointed at
// a deliberately half-written file and shown to find it (see "the validator can
// tell a half-written entry from a whole one"). A scan that walked an empty
// directory, or that accepted anything, would make every use of
// assertLedgerIntact above a green with nothing behind it.
func ledgerFaults(t *testing.T, entriesDir string) []string {
	t.Helper()

	var faults []string
	for _, name := range entryFiles(t, entriesDir) {
		if err := entryFileError(t, readFile(t, filepath.Join(entriesDir, name))); err != nil {
			faults = append(faults, name)
		}
	}
	return faults
}

// entryFileError grades one file and returns the verdict rather than failing on
// it, which is what lets the validator itself be tested.
func entryFileError(t *testing.T, file string) error {
	t.Helper()

	v, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("compiling entry.schema.json: %v", err)
	}
	return v.Validate([]byte(file))
}

func requireNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("the session ended on %v, want a clean exit", err)
	}
}

// --- small helpers ---------------------------------------------------------- //

// safeScreen is a Display that can be written from the harness's goroutine while
// the test reads it.
type safeScreen struct {
	mu    sync.Mutex
	lines []string
}

func (s *safeScreen) Show(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines = append(s.lines, text)
}

func (s *safeScreen) shown() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.lines...)
}

// confirmedCapture is a regex-staged entry a human has already stood behind:
// still `staged`, carrying `confirmed_by: human`, and therefore pending
// extraction rather than awaiting one (dec-0022, dec-0025).
func confirmedCapture(id string) *ledger.Entry {
	e := sniffShaped(id)
	e.ConfirmedBy = ConfirmedByHuman
	e.Updated = "2026-08-01T09:15:00Z"
	return e
}

// runQuietly runs a session that is expected to answer in one line and return
// cleanly.
func runQuietly(t *testing.T, ctx context.Context, store ledger.Store, term Terminal) *SessionResult {
	t.Helper()

	display := &safeScreen{}
	result, err := Run(ctx, SessionOptions{
		Store:    store,
		Terminal: term,
		Display:  display,
		Now:      func() time.Time { return sessionNow },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertExactlyOneLine(t, display.shown(), result.Summary)
	return result
}

// unhyped is the register docs/design.md §10 asks for, as a pattern: no
// exclamation, no marketing vocabulary. It is applied to this file's copy for
// the same reason E2-L4-T8 applies it to the card — the lines here are the ones
// a demo of the common case actually screenshots.
var unhyped = regexp.MustCompile(`(?i)successfully|!|revolutionary|seamless|supercharge|10x|AI-powered`)

// runSignature is the exported shape Run commits to, as an alias so the
// conversion above is exact rather than merely convertible.
type runSignature = func(context.Context, SessionOptions) (*SessionResult, error)
