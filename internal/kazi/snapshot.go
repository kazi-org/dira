package kazi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

// runFunc executes one kazi subcommand and reports what happened at the
// process level: stdout, the exit code kazi itself reported, and an error
// covering everything short of that — kazi not resolvable on PATH, or the
// caller's context ending the process before it exited. exitCode is
// meaningless when err is non-nil.
type runFunc func(ctx context.Context, args []string) (stdout []byte, exitCode int, err error)

// kaziRunner is the seam Snapshot and Status call through. Left at its
// default, execRunner, it shells the real `kazi` resolved off the process's
// own PATH at call time — which is what T7's harness needs, since a
// PATH-isolation proof is exactly what an in-process fake cannot exercise.
// T4 and T6's unit tests swap this for a fake that returns canned bytes with
// no process ever spawned, which is what keeps them from shelling a real
// kazi (this lane's whole point, per the founder decision).
var kaziRunner runFunc = execRunner

// execRunner is kaziRunner's production implementation. It resolves "kazi"
// through exec.LookPath rather than hardcoding a path, so PATH is the one and
// only seam a caller (or a test) has to control.
func execRunner(ctx context.Context, args []string) ([]byte, int, error) {
	path, err := exec.LookPath("kazi")
	if err != nil {
		return nil, 0, err
	}

	cmd := exec.CommandContext(ctx, path, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			// The context ended the process. Treat this as the
			// context's doing, not kazi's, regardless of what signal
			// or exit status the killed process happened to report.
			return stdout.Bytes(), 0, ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return stdout.Bytes(), exitErr.ExitCode(), nil
		}
		return stdout.Bytes(), 0, err
	}
	return stdout.Bytes(), 0, nil
}

// Snapshot shells `kazi portfolio --json`, validates the result before
// trusting anything in it, and decodes it.
//
// Exit code alone cannot tell a good result from a bad one: a bad argument or
// unknown command also exits non-zero (and a caller trusting exit 0 alone to
// mean "the JSON is good" is equally wrong, in the other direction — see the
// kind check below). So the checks run in a fixed order: the process itself
// (did it run, did it exit clean), then whether stdout parses as JSON at all,
// then whether it is the *right* JSON — kind: "portfolio" — before anything
// in it is trusted. A non-nil error here is always *Unavailable; a non-nil
// *Portfolio is only ever returned with a nil error (dec-0004: a caller cannot
// mistake "kazi could not be asked" for "kazi reported an empty portfolio").
func Snapshot(ctx context.Context) (*Portfolio, error) {
	stdout, exitCode, err := kaziRunner(ctx, []string{"portfolio", "--json"})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, &Unavailable{Reason: ReasonTimeout, Detail: err.Error()}
		}
		// The only other failure execRunner produces below the exec
		// call itself is exec.LookPath's — kazi is not resolvable on
		// PATH.
		return nil, &Unavailable{Reason: ReasonNotOnPath, Detail: err.Error()}
	}
	if exitCode != 0 {
		return nil, &Unavailable{Reason: ReasonNonZeroExit, Detail: fmt.Sprintf("exit %d", exitCode)}
	}
	if !json.Valid(stdout) {
		return nil, &Unavailable{Reason: ReasonMalformedJSON, Detail: "stdout does not parse as JSON"}
	}

	var probe struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(stdout, &probe); err != nil {
		// json.Valid already passed, so this can only be a shape
		// mismatch (stdout is valid JSON but not a JSON object) — still
		// malformed from this package's point of view.
		return nil, &Unavailable{Reason: ReasonMalformedJSON, Detail: err.Error()}
	}
	if probe.Kind != "portfolio" {
		return nil, &Unavailable{Reason: ReasonWrongKind, Detail: probe.Kind}
	}

	snap, err := decodeSnapshot(stdout)
	if err != nil {
		return nil, &Unavailable{Reason: ReasonMalformedJSON, Detail: err.Error()}
	}

	// schema_version is lockstep across kazi's whole --json surface
	// (kazi's lib/kazi/cli.ex:95), so it will bump for reasons unrelated
	// to portfolio specifically. The founder decision this package pins
	// against treats that as contract drift, not absence: dira keeps
	// working and says so, rather than blanking a snapshot it could still
	// mostly trust. decodeSnapshot already decoded best-effort regardless
	// of version; this is the one place that comparison happens.
	snap.ContractDrift = snap.SchemaVersion != PinnedSchemaVersion
	return snap, nil
}
