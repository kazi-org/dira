// Package engine exposes Dira use cases to in-process consumers without a Dira
// executable. Lexical checks, brief rendering and why-chain operations delegate
// to the same producers/renderers as the CLI, not another matcher or graph walk.
//
// Check does not discover ledgers, open caches, resolve parent declarations,
// write entries, confirm decisions, or grant authority. The caller supplies an
// authorized read path. Check parent inheritance is not yet exposed. Brief can
// consume an explicitly supplied ChainSource; it never discovers one itself.
// The CLI remains unchanged and continues to use that same internal engine.
// WalkChain is an explicit filesystem-reading operation for already authorized
// local roots/parents; no transport should infer access from a path alone.
//
// Check results preserve the CLI JSON vocabulary, rounded scores and private
// citation redaction. They do not expose the internal matcher's raw citations,
// which can contain private entry text. The caller's own plan is echoed just as
// in CLI JSON and may itself be sensitive; consumers must not log it blindly.
// A compliant result means this matcher detected no conflict in the supplied
// ledger, not that every policy or unprovided parent ledger was checked.
// Brief and why preserve their authorized-local-source semantics, including
// private text. They do not inherit CheckResult's citation redaction. A public
// or hosted adapter must enforce its own disclosure scopes before using them.
package engine
