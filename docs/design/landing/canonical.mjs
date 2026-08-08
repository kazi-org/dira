// canonical.mjs — the strings the landing page is not allowed to improvise.
//
// Precedent: kazi/site/src/canonical.mjs + kazi/site/scripts/check-coherence.mjs
// (ADR-0018) — canonical copy lives in one module, imported by both the page's
// author and the CI check, so a positioning surface can drift from the README
// only if someone edits this file and the README apart on purpose.
//
// Each export below names its authoritative source(s). check-coherence.mjs
// verifies presence in those sources AND in docs/design/landing/index.html —
// three-way, not just page-vs-README, because product-marketing.md is where
// some of these (the category) are actually settled and README never restates
// them in those words.

// Pain-first hook (product-marketing.md §5 "Primary hook", quoted into
// README's own opening paragraph — present in both, just broken across a
// <br> in the README's centered layout).
export const HOOK =
  'Your coding agent has amnesia. You keep re-explaining decisions you already made — and it keeps suggesting the thing you rejected in July.';

// Tagline candidate #1, marked "Recommended default" in product-marketing.md
// §5, and already the README's headline.
export const TAGLINE = 'Never explain the same decision twice.';

// The current status admission. Verbatim in README's status blockquote.
//
// NOTE 2026-07-30: this was originally "There is no binary yet." — true when
// this lane started, false by the time this file was written. E0's Go-module
// bootstrap landed in README concurrently (a `go build` now produces a binary
// that answers `--help`/`--version`, nothing else, buildable from source).
// Updated to the live wording rather than leaving a canonical string that
// would itself fail the coherence gate it exists to enforce. See
// docs/decisions-pending/E8-L2-report.md for the full note.
export const NO_BINARY =
  'The binary currently answers --help and --version and nothing else: no log, no why, no brief yet.';

// The install-line admission. README states it without naming the tap;
// docs/plan/lanes/E0.md and E8.md name the tap (`kazi-org/tap/dira`) as the
// planned path once E0 ships — that plan language is not README/marketing
// canon, so it is not asserted here as a canonical fact, only used as
// supplementary detail on the page (see index.html's status line comment).
export const INSTALL_LINE = 'brew install is not a thing yet.';

// The category bet (product-marketing.md §1). README never uses these words
// — it describes the product instead of naming its shelf — so this is
// checked against product-marketing.md only, not README.
export const CATEGORY = 'decision memory for AI coding agents';
