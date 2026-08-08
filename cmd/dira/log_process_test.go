package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kazi-org/dira/internal/ledger"
)

// The tests in this file run the real binary as a process, because the two
// clauses they cover are about processes. Thirty-two goroutines calling into the
// same package share a heap; thirty-two `dira log` invocations from thirty-two
// Stop hooks share nothing but the directory, which is the case the design has
// to survive. And a failure "mid-write" is only honestly injected by killing
// something that is writing.

// buildDira compiles the command once for a test and returns the binary's path.
func buildDira(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("builds a binary; skipped under -short")
	}
	goBin := goTool(t)

	binary := filepath.Join(t.TempDir(), "dira")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if out, err := exec.Command(goBin, "build", "-o", binary, commandPackage).CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return binary
}

// entryFiles returns the names of everything in the ledger's entries directory,
// split into entry files and everything else.
func entryFiles(t *testing.T, root string) (entries, other []string) {
	t.Helper()

	names, err := os.ReadDir(filepath.Join(root, ".dira", "entries"))
	if err != nil {
		t.Fatalf("reading the entries directory: %v", err)
	}
	for _, name := range names {
		if strings.HasSuffix(name.Name(), ".md") {
			entries = append(entries, name.Name())
			continue
		}
		other = append(other, name.Name())
	}
	return entries, other
}

// TestThirtyTwoConcurrentInvocationsProduceThirtyTwoEntries is the lane's
// acceptance clause in its literal form: 32 concurrent `dira log` invocations
// against one ledger, 32 distinct ids, 32 files, zero overwrites, zero
// collisions.
func TestThirtyTwoConcurrentInvocationsProduceThirtyTwoEntries(t *testing.T) {
	t.Parallel()

	const writers = 32

	binary := buildDira(t)
	root := ledgerRoot(t)

	// A pre-existing entry of another kind, so the test also proves the
	// writers left what was already there alone.
	seedEntry(t, root, "note-0001", minimalEntry("note-0001", "note", "active"))
	before := snapshot(t, root)

	type outcome struct {
		id     string
		stderr string
		err    error
	}
	outcomes := make([]outcome, writers)

	// A gate, so the processes overlap. Starting them in a loop without one
	// would let the first finish before the last is spawned, and the test
	// would pass without ever having raced.
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			cmd := exec.Command(binary, "log",
				"-C", root,
				"--kind", "decision",
				"--title", fmt.Sprintf("Concurrent decision number %d", i),
				"--alternative", "Not doing it",
				"--why-not", "a decision has to record at least the alternative of not doing it",
				"--hook", "Stop",
				"--tier", "semantic",
			)
			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			<-start
			err := cmd.Run()
			outcomes[i] = outcome{id: strings.TrimSpace(stdout.String()), stderr: stderr.String(), err: err}
		}()
	}
	close(start)
	wg.Wait()

	seen := map[string]int{}
	for i, got := range outcomes {
		if got.err != nil {
			t.Errorf("writer %d failed: %v\n%s", i, got.err, got.stderr)
			continue
		}
		if !ledger.ValidID(got.id) {
			t.Errorf("writer %d printed %q, which is not an id", i, got.id)
			continue
		}
		if prev, dup := seen[got.id]; dup {
			t.Errorf("writers %d and %d were both given %s", prev, i, got.id)
		}
		seen[got.id] = i
	}
	if len(seen) != writers {
		t.Fatalf("%d distinct ids from %d invocations", len(seen), writers)
	}

	// One file each, plus the entry that was already there.
	entries, other := entryFiles(t, root)
	if len(entries) != writers+1 {
		t.Errorf("the ledger holds %d entry files after %d writes plus one seed; something was overwritten:\n%v",
			len(entries), writers, entries)
	}
	if len(other) != 0 {
		t.Errorf("the writers left %v in the entries directory", other)
	}

	// The ids are the lowest unused ones, with no number skipped.
	for n := 1; n <= writers; n++ {
		want := ledger.FormatID(ledger.KindDecision, n)
		if _, ok := seen[want]; !ok {
			t.Errorf("%s was never allocated; the ids are not the lowest unused", want)
		}
	}

	// Every file is a whole, schema-valid entry, and the one that was there
	// first is byte-identical.
	after := snapshot(t, root)
	if after[".dira/entries/note-0001.md"] != before[".dira/entries/note-0001.md"] {
		t.Error("the pre-existing entry changed during the concurrent writes")
	}
	for _, name := range entries {
		data, err := os.ReadFile(filepath.Join(root, ".dira", "entries", name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if _, err := ledger.Decode(data); err != nil {
			t.Errorf("%s is not a whole entry: %v", name, err)
		}
		validateAgainstSchema(t, data)
	}
}

// TestAWriteThatCannotStartLeavesTheLedgerByteIdentical is the injected-failure
// clause in its deterministic form: the write fails at the first syscall that
// would have created something, and nothing in the ledger moves.
func TestAWriteThatCannotStartLeavesTheLedgerByteIdentical(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which ignores the directory mode this test relies on")
	}

	root := ledgerRoot(t)
	seedRealEntry(t, root, "dec-0002")
	entriesDir := filepath.Join(root, ".dira", "entries")

	// Read and traverse, but no create: the failure lands inside the
	// backend's write, after the entry has been validated and encoded.
	if err := os.Chmod(entriesDir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	// Restored so t.TempDir can clean up, and so the snapshot below can read.
	t.Cleanup(func() { _ = os.Chmod(entriesDir, 0o755) })

	before := snapshot(t, root)
	got := runDira(t, root, "", "--kind", "note", "--title", "A note the disk would not take")
	if got.code != exitError {
		t.Errorf("exit code = %d, want %d for a write that failed\n--- stderr ---\n%s", got.code, exitError, got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q; nothing was written, so there is no id to report", got.stdout)
	}

	after := snapshot(t, root)
	if len(after) != len(before) {
		t.Errorf("the ledger holds %d files, want %d unchanged", len(after), len(before))
	}
	for path, sum := range before {
		if after[path] != sum {
			t.Errorf("%s changed during a write that failed", path)
		}
	}
	entries, other := entryFiles(t, root)
	if len(entries) != 1 {
		t.Errorf("entry files = %v, want only the one that was there", entries)
	}
	if len(other) != 0 {
		t.Errorf("a failed write left %v behind", other)
	}
}

// TestAFailureInTheMiddleOfTheWriteLeavesThePreWriteStateByteIdentical is the
// injected-failure clause with the failure landing where it matters: after the
// entry has been validated, encoded and partly written.
//
// The injection is a file-size limit on the process, so the write to the
// backend's temporary file fails a kilobyte into a half-megabyte entry — a real
// short write, reported by the kernel, at a point no application-level check
// could have caught. What the ledger has to show for it is nothing at all: the
// temporary file never becomes an entry, so the entry file the write was aiming
// at is never created, and every file that was there before is byte-identical.
func TestAFailureInTheMiddleOfTheWriteLeavesThePreWriteStateByteIdentical(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("ulimit is a POSIX shell builtin")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skipf("no POSIX shell to set a file-size limit with: %v", err)
	}

	binary := buildDira(t)
	root := ledgerRoot(t)
	seedRealEntry(t, root, "dec-0002")
	before := snapshot(t, root)

	// Half a megabyte of body against a 1KB limit: the kernel refuses the
	// write partway through, which is the failure this clause is about.
	body := strings.Repeat("The ledger is the record, and a half-written entry is corruption in it.\n", 7000)

	cmd := exec.Command(shell, "-c",
		`ulimit -f 2; exec "$1" log -C "$2" --kind note --title "An entry larger than the file size limit" --body -`,
		"sh", binary, root)
	cmd.Stdin = strings.NewReader(body)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr == nil {
		t.Fatalf("the write succeeded against a %d-byte file size limit; the failure was not injected", 1024)
	}
	var exit *exec.ExitError
	if !errors.As(runErr, &exit) {
		t.Fatalf("running dira under a file size limit: %v", runErr)
	}
	if exit.ExitCode() != exitError {
		t.Errorf("exit code = %d, want %d for a write that failed", exit.ExitCode(), exitError)
	}
	if stdout.String() != "" {
		t.Errorf("stdout = %q; nothing was written, so there is no id to report", stdout.String())
	}
	if !strings.Contains(stderr.String(), "note-0001") {
		t.Errorf("stderr does not name the entry that failed to land: %q", stderr.String())
	}

	after := snapshot(t, root)
	if len(after) != len(before) {
		t.Errorf("the ledger holds %d files, want the %d it started with", len(after), len(before))
	}
	for path, sum := range before {
		if after[path] != sum {
			t.Errorf("%s changed during a write that failed", path)
		}
	}
	entries, other := entryFiles(t, root)
	if len(entries) != 1 || entries[0] != "dec-0002.md" {
		t.Errorf("entry files = %v, want only the one that was there before", entries)
	}
	if len(other) != 0 {
		t.Errorf("the failed write left %v behind", other)
	}
}

// TestALeftoverTemporaryFileIsNotAnEntry covers what a killed writer leaves: the
// backend commits an entry by writing a temporary file and linking it into
// place, so a process that dies between those two steps leaves the temporary
// behind.
//
// That file is scratch, not ledger. It has no entry id, so nothing lists it,
// nothing reads it, and it takes no id out of circulation — which this asserts
// rather than assumes, because the failure mode if it did participate would be
// an id allocated around a file that is not an entry.
func TestALeftoverTemporaryFileIsNotAnEntry(t *testing.T) {
	t.Parallel()

	root := ledgerRoot(t)
	seedEntry(t, root, "note-0001", minimalEntry("note-0001", "note", "active"))

	// Half an entry, exactly as a killed writer would have left it.
	partial := filepath.Join(root, ".dira", "entries", ".dira-2130706433.tmp")
	if err := os.WriteFile(partial, []byte("---\nid: note-0002\nkind: note\ntitle: An entry that was b"), 0o644); err != nil {
		t.Fatalf("planting a leftover temporary file: %v", err)
	}
	before := snapshot(t, root)

	got := runDira(t, root, "", "--kind", "note", "--title", "The entry written after the crash")
	if got.code != exitOK {
		t.Fatalf("exit code = %d\n--- stderr ---\n%s", got.code, got.stderr)
	}
	if got.stdout != "note-0002\n" {
		t.Errorf("allocated %q, want note-0002: the leftover file holds no id", strings.TrimSpace(got.stdout))
	}

	paths := modifiedPaths(before, snapshot(t, root))
	if len(paths) != 1 || paths[0] != ".dira/entries/note-0002.md" {
		t.Fatalf("the write touched %v, want exactly [.dira/entries/note-0002.md]", paths)
	}
}

// TestKillingDiraLeavesNoPartialEntry is the same clause with a real crash
// rather than a reported failure.
//
// A killed process runs no deferred cleanup, so this is the strongest statement
// available: however the process dies, the ledger holds either the whole new
// entry or no trace of it, every entry that was already there is byte-identical,
// and no entry file is ever half-written. The backend earns that by writing to a
// temporary file and committing it with a link, so an entry file only ever
// appears complete.
//
// What this does not claim, because it was measured and is not true: that the
// kill lands inside the write. It almost never does — the write is a single
// syscall the signal does not interrupt, against a process that spends most of
// its life starting up — and replacing the temporary file and link with a write
// straight to the entry path survives this test. So this is the arbitrary-crash
// sweep, and its value is that the invariant is checked after every one of two
// dozen real kills. The two checks that do bite on atomicity are
// TestAFailureInTheMiddleOfTheWriteLeavesThePreWriteStateByteIdentical and
// TestAConcurrentReaderNeverSeesAHalfWrittenEntry, and both were confirmed red
// against exactly that substitution.
func TestKillingDiraLeavesNoPartialEntry(t *testing.T) {
	t.Parallel()

	binary := buildDira(t)
	root := ledgerRoot(t)
	original := seedRealEntry(t, root, "dec-0002")

	body := strings.Repeat("The ledger is the record, and a half-written entry is corruption in it.\n", 7000)
	wantBody := "\n" + strings.Trim(body, "\n") + "\n"

	invoke := func(delay time.Duration) (completed bool) {
		cmd := exec.Command(binary, "log", "-C", root,
			"--kind", "note",
			"--title", "An entry written while the process is being killed",
			"--body", "-")
		cmd.Stdin = strings.NewReader(body)

		if err := cmd.Start(); err != nil {
			t.Fatalf("starting dira: %v", err)
		}
		if delay > 0 {
			timer := time.AfterFunc(delay, func() { _ = cmd.Process.Kill() })
			defer timer.Stop()
		} else {
			_ = cmd.Process.Kill()
		}

		err := cmd.Wait()
		var exit *exec.ExitError
		return err == nil || (errors.As(err, &exit) && exit.ExitCode() == exitOK)
	}

	// A warm baseline. The first run of a freshly built binary pays for
	// paging it in and is an order of magnitude slower, so measuring that
	// one would scale every delay to a cost the later runs do not have and
	// the kills would all land after the process had finished.
	invoke(10 * time.Second)
	start := time.Now()
	invoke(10 * time.Second)
	baseline := time.Since(start)
	t.Logf("an uninterrupted `dira log` with a %d-byte body takes %v here", len(body), baseline)

	const rounds = 24
	killed, completed := 0, 0
	for i := range rounds {
		// Swept from nothing to a little past the whole command, so the
		// tail of the range brackets the write, which is the last thing
		// the command does.
		delay := time.Duration(int64(baseline) * 6 * int64(i) / (5 * int64(rounds)))
		if invoke(delay) {
			completed++
		} else {
			killed++
		}

		// The invariant, checked after every single kill rather than at
		// the end: the ledger is never observably broken.
		entries, other := entryFiles(t, root)
		for _, name := range entries {
			path := filepath.Join(root, ".dira", "entries", name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", name, err)
			}
			entry, err := ledger.Decode(data)
			if err != nil {
				t.Fatalf("round %d (delay %v) left %s partial or malformed: %v", i, delay, name, err)
			}
			if name == "dec-0002.md" {
				if string(data) != original {
					t.Fatalf("round %d (delay %v) changed the entry that was already there", i, delay)
				}
				continue
			}
			// Decoding is not enough on its own: the body is
			// trailing text, so an entry truncated inside it still
			// parses. What must hold is that the whole body is
			// there.
			if entry.Body != wantBody {
				t.Fatalf("round %d (delay %v) left %s holding %d bytes of body, want the whole %d",
					i, delay, name, len(entry.Body), len(wantBody))
			}
		}
		for _, name := range other {
			// A killed process cannot remove its scratch file. It
			// is not an entry and nothing reads it, but it must
			// never be mistaken for one.
			if strings.HasSuffix(name, ".md") {
				t.Fatalf("round %d left %s, which looks like an entry file", i, name)
			}
		}
	}

	t.Logf("%d invocations were killed before finishing, %d completed", killed, completed)
	if killed == 0 {
		t.Error("no invocation was actually killed; this test proved nothing")
	}

	// A control run, uninterrupted, proving the harness would have noticed a
	// broken entry: the invariant loop above only says something if a write
	// under these arguments produces a whole entry when left alone.
	//
	// This is a control rather than a requirement that one of the killed
	// rounds completed. The delays are scaled to a baseline measured on a
	// machine that is also running the rest of the test suite, so how many
	// rounds survive is a fact about load, and a test that asserts one is a
	// test that fails on a busy CI runner for no reason.
	if !invoke(60 * time.Second) {
		t.Fatal("the uninterrupted control invocation did not complete")
	}
	entries, _ := entryFiles(t, root)
	whole := 0
	for _, name := range entries {
		if name == "dec-0002.md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, ".dira", "entries", name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		entry, err := ledger.Decode(data)
		if err != nil {
			t.Fatalf("the control invocation left %s malformed: %v", name, err)
		}
		if entry.Body == wantBody {
			whole++
		}
	}
	if whole == 0 {
		t.Error("no complete entry was ever written, so the body check above could not have failed")
	}
}

// TestAConcurrentReaderNeverSeesAHalfWrittenEntry is the atomicity of the
// single write, observed rather than argued.
//
// A reader polls the entries directory while a `dira log` with a multi-megabyte
// body is running. The entry file has to be either absent or complete every
// single time it is looked at: the backend writes the content to a temporary
// file and commits it with a link, and a link either happens or does not.
//
// This is the check with teeth for that property. Replacing the temporary file
// and link with a write straight to the entry path fails here within one run,
// while the same substitution survives being killed at arbitrary moments,
// because the window a signal has to land in is microseconds wide and the window
// a reader has to look in is however long the write takes.
func TestAConcurrentReaderNeverSeesAHalfWrittenEntry(t *testing.T) {
	t.Parallel()

	binary := buildDira(t)
	root := ledgerRoot(t)
	path := filepath.Join(root, ".dira", "entries", "note-0001.md")

	// Big enough that writing it takes long enough to be caught looking.
	body := strings.Repeat("The ledger is the record, and a half-written entry is corruption in it.\n", 120000)

	cmd := exec.Command(binary, "log", "-C", root,
		"--kind", "note",
		"--title", "An entry read while it is being written",
		"--body", "-")
	cmd.Stdin = strings.NewReader(body)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	type reading struct {
		attempts int
		sizes    []int64
	}
	done := make(chan struct{})
	observations := make(chan reading, 1)

	// Sizes rather than decodability, and stat rather than read. A
	// half-written entry whose frontmatter happens to be complete still
	// parses — the body is trailing text, so a truncated entry is a valid
	// entry with prose missing, and that is exactly the failure being looked
	// for. Stat also costs microseconds against milliseconds for reading
	// megabytes, which is the difference between sampling the write
	// thousands of times and sampling it once.
	go func() {
		var got reading
		for {
			select {
			case <-done:
				observations <- got
				return
			default:
			}
			got.attempts++
			if info, err := os.Stat(path); err == nil {
				got.sizes = append(got.sizes, info.Size())
			}
		}
	}()

	if err := cmd.Run(); err != nil {
		close(done)
		t.Fatalf("dira log: %v\n%s", err, stderr.String())
	}
	close(done)

	final, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the finished entry: %v", err)
	}
	got := <-observations
	for _, size := range got.sizes {
		if size != int64(len(final)) {
			t.Fatalf("a reader saw %s at %d bytes while it was being written; the finished entry is %d bytes, "+
				"so a half-written entry was visible under its own name", path, size, len(final))
		}
	}

	// The vacuity guard is how hard the reader looked, not how often it
	// happened to find the file. Whether a sighting lands inside the write is
	// a matter of scheduling and machine load, and asserting one would make
	// this test fail on a busy runner rather than on a broken write; that the
	// reader ran thousands of times during a write it could have caught is
	// the part that must be true for the check above to mean anything.
	const wantAttempts = 1000
	if got.attempts < wantAttempts {
		t.Errorf("the reader only looked %d times during the write, want at least %d; it was starved, "+
			"so seeing nothing half-written says nothing", got.attempts, wantAttempts)
	}
	t.Logf("the reader looked %d times and saw the entry file %d times, always whole (%d bytes)",
		got.attempts, len(got.sizes), len(final))
}
