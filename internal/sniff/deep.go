package sniff

import (
	"context"

	"github.com/kazi-org/dira/internal/ledger"
)

// The deep run: tier 1 first, always, and the handoff afterwards.
//
// # `--deep` is a modifier on the write path, not a second tier
//
// The word "deep" invites a reading in which a deeper pass finds more, or finds
// better, or replaces the shallow one. None of those is true here and the code
// is arranged so that none of them can quietly become true. Deep calls Stage —
// the same function `dira sniff --stage` calls, with the same StageOptions,
// through the same stagedOnly wrapper — and then renders a block of text about
// what Stage wrote. There is no second matcher, no second writer, and no path
// through this file on which a candidate is staged differently because `--deep`
// was passed.
//
// That ordering is the whole design. `--deep` runs from `PreCompact`, which
// fires immediately before the session's lossiest moment, and dec-0023
// established that the block it prints reaches only the compaction summariser —
// a one-turn call whose prompt forbids it from calling tools. So the handoff is
// a best-effort message to a reader that may not act, while the staging is a
// guarantee. A design in which the guarantee depended on the best-effort part
// would be an insurance policy that lapses exactly when it is claimed on.
//
// # Which is why a broken renderer costs nothing
//
// Stage returns before anything is rendered, and renderSafely swallows every way
// the render can go wrong — an error, a panic, a future renderer that reaches
// for something that is not there. The hook's job is to not lose the session. A
// handoff that failed to render has cost the run one block of text; a hook that
// failed because a handoff failed to render has cost it the session, which is
// the thing dira exists to prevent.
//
// # What `--deep` does NOT change, and where the caller has to be careful
//
// It does not change the scope. `--deep` reads exactly what the caller asked
// for — the last turn by default, the whole transcript under `--all` — because
// the acceptance this task is held to is that a deep run writes byte-identical
// entries to a plain staged run over the same input, and a flag that silently
// widened the read would break that on any transcript longer than one turn.
// transcript.go's Scope comment records that PreCompact wants the whole file,
// and the flag that says so is `--all`; `cmd/dira/sniff.go`'s usage names the
// full invocation. See the report accompanying this change for the correction
// hooks/settings.example.json needs, which is not this package's file to make.
//
// It does not change the tier. Everything staged here is `source.tier: regex`,
// because a regular expression found it and dec-0025 calls rewriting that field
// what it is: forging provenance. The semantic tier's output arrives later and
// separately, as its own entry, through `dira log`.

// A DeepOptions is what a deep run needs.
//
// It embeds StageOptions rather than restating it so that there is exactly one
// description of a capture's provenance in this package, and so that a field
// added there cannot be silently missing here.
type DeepOptions struct {
	StageOptions

	// render is the handoff renderer, and it is unexported on purpose.
	//
	// The acceptance requires observing that a forced render failure still
	// stages, which needs an injection point; an exported one would be an
	// injection point production callers could use, and handoff_test.go
	// already states this package's rule about that — a production code
	// path that can be asked to skip its redaction is a production code
	// path somebody will ask. Unexported, only this package's own tests can
	// reach it, and cmd/dira cannot express a deep run that renders
	// anything other than the golden-pinned block.
	//
	// Nil means Handoff.
	render func([]HandoffItem) (string, error)
}

// Deep stages the regex candidates and returns the handoff block for what it
// wrote.
//
// The block is the empty string when there was nothing to hand off, when every
// candidate was refused by the credential check, and when rendering failed. All
// three are the same instruction to the caller: print nothing. A hook that
// emitted an empty header on a session that settled nothing would be a hook that
// gets uninstalled, and dec-0023 makes the emptier case worse than useless —
// this text lands in a compaction summariser's prompt, so an empty block is
// noise in a budget that is already exhausted.
//
// The error is a staging error and only ever a staging error. Rendering the
// handoff cannot produce one.
func Deep(ctx context.Context, store ledger.Store, opts DeepOptions, candidates []Candidate) (*Result, string, error) {
	// Tier 1 first, and unconditionally. Everything below this line is
	// additive by construction rather than by intention.
	result, err := Stage(ctx, store, opts.StageOptions, candidates)
	if err != nil {
		return result, "", err
	}
	return result, renderSafely(opts.render, StagedItems(result.Staged)), nil
}

// renderSafely renders the handoff and refuses to fail.
//
// Both failure shapes are covered because both are real. An error is what a
// future renderer that reads something would return; a panic is what a nil map,
// a slice bound or a formatting bug produces today, and a panic out of a
// PreCompact hook is a non-zero exit that `hooks/settings.example.json` only
// survives because of a `|| true` nobody should be relying on for correctness.
func renderSafely(render func([]HandoffItem) (string, error), items []HandoffItem) (block string) {
	if render == nil {
		render = defaultRender
	}
	defer func() {
		if recover() != nil {
			// Deliberately silent, and deliberately not an error. The
			// caller's remaining job is to print what it has, and it
			// has nothing.
			block = ""
		}
	}()

	out, err := render(items)
	if err != nil {
		return ""
	}
	return out
}

// defaultRender is the shipped renderer: T2's golden-pinned block, unchanged.
func defaultRender(items []HandoffItem) (string, error) { return Handoff(items), nil }
