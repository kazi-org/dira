---
id: int-9002
kind: intent
title: An intent in a decision-only state must be rejected
state: accepted
created: "2026-07-29T20:00:00Z"
---

Intents are active, achieved, or abandoned. `accepted` belongs to decisions.
The state enum is global, so only the per-kind allOf branch catches this.
