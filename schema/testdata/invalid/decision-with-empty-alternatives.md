---
id: dec-9004
kind: decision
title: An accepted decision with an empty alternatives list must be rejected
state: accepted
created: "2026-07-29T20:00:00Z"
alternatives: []
---

`alternatives: []` used to satisfy the schema — `required` with no `minItems` —
while `Entry.Validate` rejected the same file. That disagreement meant a decision
could be written that the published contract accepted and the binary would not
read back.

The list is empty, which is the assertion the field exists to prevent: not "the
roads not taken are recorded here" but "there were none". A staged decision is
exempt, because nobody has decided yet; this one is accepted.
