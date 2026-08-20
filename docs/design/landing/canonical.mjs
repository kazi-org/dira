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
// this lane started, false a few weeks later. E0's Go-module bootstrap landed
// in README concurrently (a `go build` now produces a binary that answers
// `--help`/`--version`, nothing else, buildable from source). Updated then to
// that live wording; see docs/decisions-pending/E8-L2-report.md for that note.
//
// NOTE 2026-08-14: false again. README was rewritten (7f0e66a/09684cd/723eee3)
// to describe what dira does today rather than what it did on 2026-07-29 —
// 14 verbs now ship and are tested, so the "answers --help and --version and
// nothing else" sentence no longer exists anywhere truthful to quote. Kept the
// export name (an E8-L2 acc predicate checks for its presence, not its
// wording) but retargeted it at the 14-verb claim: still the single sentence
// whose falsehood is the most consequential — the moment a verb ships or
// breaks without a README update, this stops matching and the gate catches
// it, same job the old string did for the binary's existence.
export const NO_BINARY =
  "14 verbs are real and tested against this repo's own 43-entry ledger — capture, review, enforcement, cross-project tiers, ADR import, and a read-only web surface all run today.";

// The install-line admission. Was "there is no brew install yet." — true
// until v0.1.1 shipped a real Homebrew tap (kazi-org/homebrew-tap) on
// 2026-08-18. Retargeted to the command itself: the sentence that is both
// true today and the one that would catch drift the other direction — if the
// tap is ever pulled or renamed, this line stops appearing and the gate
// fails until the page is told. Backtick-free because both normalizers
// (normalizeMarkdown, normalizeHtml) strip backticks/tags before comparing —
// a canonical string that still has them can never match.
export const INSTALL_LINE = 'brew install kazi-org/tap/dira';

// The category bet (product-marketing.md §1). README never uses these words
// — it describes the product instead of naming its shelf — so this is
// checked against product-marketing.md only, not README.
export const CATEGORY = 'decision memory for AI coding agents';
