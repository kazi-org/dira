// Package engine exposes Dira use cases to in-process consumers without a Dira
// executable. This first slice provides lexical checks against an explicitly
// supplied ledger. It delegates to the same matcher and privacy-preserving JSON
// renderer as the standalone CLI; it does not implement another algorithm.
//
// Check does not discover ledgers, open caches, resolve parent declarations,
// write entries, confirm decisions, or grant authority. The caller supplies an
// authorized read path. Parent inheritance, brief, why and the remaining Dira
// use cases are not yet exposed by this package. The CLI remains unchanged and
// continues to use that same internal engine, including its parent resolution.
//
// Results preserve the CLI JSON vocabulary, rounded advisory scores and private
// citation redaction. They do not expose the internal matcher's raw citations,
// which can contain private entry text. The caller's own plan is echoed just as
// in CLI JSON and may itself be sensitive; consumers must not log it blindly.
// A compliant result means this matcher detected no conflict in the supplied
// ledger, not that every policy or unprovided parent ledger was checked.
package engine
