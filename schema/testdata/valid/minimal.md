---
id: qst-9001
kind: question
title: Does an entry carrying only the required fields validate
state: open
created: "2026-07-29T20:00:00Z"
---

The floor. Everything except id, kind, title, state, and created is optional,
and a validator that quietly requires more than the schema does would make
`dira log` demand fields a human never typed.
