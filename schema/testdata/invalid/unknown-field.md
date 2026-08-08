---
id: note-9002
kind: note
title: An unknown top-level field must be rejected
state: active
created: "2026-07-29T20:00:00Z"
priority: high
---

`additionalProperties: false` is what keeps the entry model from growing a
status field by accident. dec-0004: no execution status is ever a stored state.
