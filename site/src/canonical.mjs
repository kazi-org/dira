// Canonical strings shared by the website AND ../README.md (mirrors kazi's
// site/src/canonical.mjs, ADR-0018). The drift-check
// (scripts/check-coherence.mjs, W2-T7) asserts each one appears verbatim in
// the README, so the two surfaces can only diverge if someone edits this
// file and the README apart on purpose.
export const STRAPLINE = "Never explain the same decision twice.";
export const INSTALL_CMD = "go build ./cmd/dira";
