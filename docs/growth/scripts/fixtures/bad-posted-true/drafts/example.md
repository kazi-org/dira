---
status: awaiting-maintainer-approval
posted: true
target: a fixture, not a real submission target
kind: marketplace-listing
owner: E8-L5
---

# Bad fixture: `posted` flipped to `true`

This file exists only to prove `check-drafts.mjs` rejects a draft whose
`posted` field is `true`. It is never a real draft, is never scanned by the
production run of the checker (it lives under a `fixtures/` directory,
excluded by name — see `check-drafts.mjs`'s header comment), and is only
ever exercised directly by `check-drafts.selftest.mjs`.
