---
id: dec-9001
kind: decision
title: A canonical decision fixture exercising every optional field
state: accepted
created: "2026-07-29T20:00:00Z"
updated: "2026-07-30T02:00:00Z"
tags: [fixture, schema]
edges:
  - type: derives_from
    to: int-9001
    note: fixture edge
  - type: informs
    to: sire:int-0002
  - type: realized_by
    to: kazi:prop-fixture-0001
alternatives:
  - option: Not writing a canonical fixture
    why_not: a schema test whose positive half is only real entries stops catching
      regressions the moment the real entries stop using a field.
    revisit_if: the real ledger exercises every optional field on its own
source:
  hook: manual
  session: fixture-session
  excerpt: a bounded transcript fragment
  tier: human
confirmed_by: human
adr: docs/adr/9001-canonical-fixture.md
private: false
---

The positive control. If this file ever fails validation, the validator has
regressed rather than the ledger.
