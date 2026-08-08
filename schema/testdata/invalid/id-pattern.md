---
id: dec-1
kind: decision
title: An id with too few digits must be rejected
state: accepted
created: "2026-07-29T20:00:00Z"
alternatives:
  - option: Allowing short ids
    why_not: ids are permanent and sort lexically in the brief
---

`^(int|dec|qst|cst|note)-[0-9]{4,}$` requires at least four digits.
