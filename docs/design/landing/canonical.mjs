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

// Shared capability summary. Keep the legacy export name for existing consumers.
export const NO_BINARY =
  'Capture, review, conflict checks, ADR import, and a read-only ledger browser are available in dira.';

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
