package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kazi-org/dira/ledger"
)

// buildPublicationProbe overlays only the actual writer source at compile time.
// Normal buildDira callers and production source never acquire a probe hook.
// The source hash and unique anchors deliberately refuse writer drift: review
// the instrumentation again rather than silently instrumenting a different path.
func buildPublicationProbe(t *testing.T, mutant bool) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate publication probe source")
	}
	source := filepath.Join(filepath.Dir(here), "..", "..", "ledger", "local", "local.go")
	source, err := filepath.Abs(source)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	const expected = "75c52c9075cbf1472526c4d4ed3f61f7c206b3796469a10a46cece93d25cb3ab"
	if got := fmt.Sprintf("%x", sha256.Sum256(original)); got != expected {
		t.Fatalf("publication probe source drift: got %s, want %s", got, expected)
	}
	backing := string(original)
	replace := func(old, new string) {
		t.Helper()
		if strings.Count(backing, old) != 1 {
			t.Fatalf("publication probe requires exactly one anchor %q", old)
		}
		backing = strings.Replace(backing, old, new, 1)
	}
	replace("tmp.Write(data)", "publicationProbeWrite(tmp, data)")
	replace("\t\treturn nil\n\t}\n\tif err := os.Rename(tmpName, path);", "\t\tif err := publicationProbePhase(\"PUBLISHED\", len(data)/2, len(data), '2'); err != nil { return err }\n\t\treturn nil\n\t}\n\tif err := os.Rename(tmpName, path);")
	if mutant {
		// Deliberately unsafe disposable control: write directly to the final
		// name, retain it, and bypass Link. No copied writer implementation.
		replace("os.CreateTemp(dir, \".dira-*.tmp\")", "os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)")
		replace("defer func() { _ = os.Remove(tmpName) }()", "// Direct-write mutant keeps the final file.")
		replace("os.Link(tmpName, path)", "func() error { return nil }()")
	}
	backing += publicationProbeInstrumentation
	dir := t.TempDir()
	backingPath := filepath.Join(dir, "local.go")
	if err := os.WriteFile(backingPath, []byte(backing), 0o600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(struct {
		Replace map[string]string
	}{map[string]string{source: backingPath}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("instrumented publication probe: mutant=%t source=%x backing=%x", mutant, sha256.Sum256(original), sha256.Sum256([]byte(backing)))
	binary := filepath.Join(dir, "dira-publication-probe")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goTool(t), "build", "-overlay", overlayPath, "-o", binary, commandPackage)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build instrumented publication probe: %v\n%s", err, out)
	}
	return binary
}

// These declarations exist only in the temporary backing file. Each phase is
// emitted after a real write/link and blocks on its own exact acknowledgement.
const publicationProbeInstrumentation = `
var publicationProbeEvents = os.NewFile(3, "publication-events")
var publicationProbeAcks = os.NewFile(4, "publication-acks")
func publicationProbePhase(phase string, partial, total int, ack byte) error {
	if _, err := fmt.Fprintf(publicationProbeEvents, "%s %d %d\n", phase, partial, total); err != nil { return err }
	var b [1]byte
	if n, err := publicationProbeAcks.Read(b[:]); err != nil || n != 1 || b[0] != ack {
		return fmt.Errorf("publication probe acknowledgement %s: n=%d byte=%d err=%v", phase, n, b[0], err)
	}
	return nil
}
func publicationProbeWrite(f *os.File, data []byte) (int, error) {
	cut := len(data)/2
	if cut == 0 || cut == len(data) { return 0, fmt.Errorf("publication probe needs two nonempty chunks") }
	n, err := f.Write(data[:cut])
	if err != nil { return n, err }
	if n != cut { return n, fmt.Errorf("publication probe short first write") }
	if err := publicationProbePhase("PARTIAL_STAGED", cut, len(data), '1'); err != nil { return n, err }
	m, err := f.Write(data[cut:])
	if err == nil && m != len(data)-cut { err = fmt.Errorf("publication probe short second write") }
	return n+m, err
}
`

type publicationReport struct {
	partialStats, partialVisible, publishedStats int
	partial, total                               int64
}

type partialPublicationError struct{ message string }

func (e *partialPublicationError) Error() string { return e.message }

// publicationEntry checks the exact metadata/body as well as the encoded size.
func publicationEntry(root, title, body string) ([]byte, error) {
	path := filepath.Join(root, ".dira", "entries", "note-0001.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	entry, err := ledger.Decode(data)
	if err != nil {
		return nil, err
	}
	if entry.ID != "note-0001" || entry.Kind != ledger.KindNote || entry.Title != title || entry.Body != "\n"+strings.Trim(body, "\n")+"\n" {
		return nil, fmt.Errorf("published entry metadata/body differ from input")
	}
	return data, nil
}

// This control exercises the ordinary binary without claiming overlap coverage.
func runOrdinaryPublicationControl(t *testing.T, binary, root, title, body string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "log", "-C", root, "--kind", "note", "--title", title, "--body", "-")
	cmd.Stdin = strings.NewReader(body)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output() // CommandContext kills and Output reaps on deadline.
	if err != nil || strings.TrimSpace(string(out)) != "note-0001" {
		t.Fatalf("ordinary log: %v stdout=%q stderr=%q", err, out, stderr.String())
	}
	if _, err := publicationEntry(root, title, body); err != nil {
		t.Fatal(err)
	}
}

// observePublication is shared unchanged by safe and mutant binaries. The 1000
// stats happen only after PARTIAL_STAGED and before its ack. Published reads do
// not contribute. Protocol/exit/coverage failures cannot masquerade as the
// expected partial-visibility error returned for the mutant.
func observePublication(binary, root, title, body string) (report publicationReport, result error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	eventsR, eventsW, err := os.Pipe()
	if err != nil {
		return report, err
	}
	// Best-effort cleanup also runs after explicit endpoint closes below.
	defer func() { _ = eventsR.Close() }()
	defer func() { _ = eventsW.Close() }()
	acksR, acksW, err := os.Pipe()
	if err != nil {
		return report, err
	}
	defer func() { _ = acksR.Close() }()
	defer func() { _ = acksW.Close() }()
	cmd := exec.CommandContext(ctx, binary, "log", "-C", root, "--kind", "note", "--title", title, "--body", "-")
	cmd.Stdin = strings.NewReader(body)
	cmd.ExtraFiles = []*os.File{eventsW, acksR}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		return report, err
	}
	_ = eventsW.Close()
	_ = acksR.Close()
	done := make(chan struct{})
	var waitErr error
	go func() { waitErr = cmd.Wait(); close(done) }()
	defer func() {
		_ = cmd.Process.Kill()
		<-done // Reap this child on every error, including protocol timeout.
		if result != nil {
			result = fmt.Errorf("%w; stdout=%q stderr=%q", result, stdout.String(), stderr.String())
		}
	}()
	type event struct {
		line string
		err  error
	}
	events := make(chan event)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		defer close(events)
		scanner := bufio.NewScanner(eventsR)
		for scanner.Scan() {
			select {
			case events <- event{line: scanner.Text()}:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case events <- event{err: err}:
			case <-ctx.Done():
			}
		}
	}()
	defer func() { cancel(); _ = eventsR.Close(); <-readerDone }()
	next := func(phase string) (int64, int64, error) {
		select {
		case e, ok := <-events:
			if !ok {
				return 0, 0, fmt.Errorf("missing %s: EOF", phase)
			}
			fields := strings.Fields(e.line)
			if e.err != nil || len(fields) != 3 || fields[0] != phase {
				return 0, 0, fmt.Errorf("expected %s, got %q: %v", phase, e.line, e.err)
			}
			partial, e1 := strconv.ParseInt(fields[1], 10, 64)
			total, e2 := strconv.ParseInt(fields[2], 10, 64)
			if e1 != nil || e2 != nil || partial <= 0 || total <= partial || partial != total/2 {
				return 0, 0, fmt.Errorf("invalid %s lengths: %q", phase, e.line)
			}
			return partial, total, nil
		case <-done:
			return 0, 0, fmt.Errorf("child exited before %s: %v", phase, waitErr)
		case <-ctx.Done():
			return 0, 0, fmt.Errorf("waiting for %s: %w", phase, ctx.Err())
		}
	}
	alive := func() error {
		select {
		case <-done:
			return fmt.Errorf("child exited during held write: %v", waitErr)
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	report.partial, report.total, err = next("PARTIAL_STAGED")
	if err != nil {
		return report, err
	}
	path := filepath.Join(root, ".dira", "entries", "note-0001.md")
	var sizes []int64
	for i := 0; i < 1000; i++ {
		if err := alive(); err != nil {
			return report, err
		}
		info, err := os.Stat(path)
		report.partialStats++
		switch {
		case err == nil:
			report.partialVisible++
			sizes = append(sizes, info.Size())
			if info.Size() != report.partial {
				return report, fmt.Errorf("held-write size %d differs from staged length %d", info.Size(), report.partial)
			}
		case errors.Is(err, os.ErrNotExist):
		default:
			return report, fmt.Errorf("held-write stat: %w", err)
		}
	}
	if err := alive(); err != nil {
		return report, err
	}
	if n, err := acksW.Write([]byte{'1'}); err != nil || n != 1 {
		return report, fmt.Errorf("partial ack: n=%d err=%v", n, err)
	}
	partial, total, err := next("PUBLISHED")
	if err != nil {
		return report, err
	}
	if partial != report.partial || total != report.total {
		return report, fmt.Errorf("publication lengths changed")
	}
	if err := alive(); err != nil {
		return report, err
	}
	info, err := os.Stat(path)
	report.publishedStats++
	if err != nil {
		return report, fmt.Errorf("published stat: %w", err)
	}
	sizes = append(sizes, info.Size())
	if info.Size() != total {
		return report, fmt.Errorf("published size=%d want=%d", info.Size(), total)
	}
	published, err := publicationEntry(root, title, body)
	if err != nil || int64(len(published)) != total {
		return report, fmt.Errorf("published entry length=%d want=%d: %v", len(published), total, err)
	}
	if n, err := acksW.Write([]byte{'2'}); err != nil || n != 1 {
		return report, fmt.Errorf("publication ack: n=%d err=%v", n, err)
	}
	_ = acksW.Close()
	select {
	case <-done:
		if waitErr != nil {
			return report, fmt.Errorf("log exit: %w", waitErr)
		}
	case <-ctx.Done():
		return report, ctx.Err()
	}
	// EOF is mandatory; a duplicate/out-of-order/trailing event is not success.
	select {
	case extra, ok := <-events:
		if ok {
			return report, fmt.Errorf("unexpected trailing event %q: %v", extra.line, extra.err)
		}
	case <-ctx.Done():
		return report, ctx.Err()
	}
	final, err := publicationEntry(root, title, body)
	if err != nil || !bytes.Equal(final, published) {
		return report, fmt.Errorf("entry changed after publication: %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "note-0001" {
		return report, fmt.Errorf("unexpected log output %q", stdout.String())
	}
	for _, size := range sizes {
		if size != int64(len(final)) {
			return report, &partialPublicationError{fmt.Sprintf("reader observed partial file: %d bytes; final=%d", size, len(final))}
		}
	}
	if report.partialStats < 1000 || report.partialVisible != 0 {
		return report, fmt.Errorf("insufficient or non-absent held-write coverage: %+v", report)
	}
	return report, nil
}
