---
id: bogus-0001
kind: habit
title: A kind the schema does not allow
state: active
created: "2026-08-01T00:00:00Z"
---

Fixture for E8-L3-T2: `kind: habit` is not one of the five values
`schema/entry.schema.json` closes `kind` to (`intent`, `decision`, `question`,
`constraint`, `note`). This file exists only to make `check-fixture-ledger.mjs`
fail loudly and by name; it is never copied into a real ledger.
