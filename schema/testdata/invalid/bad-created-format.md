---
id: note-9010
kind: note
title: A created value that is not RFC3339 must be rejected
state: active
created: "yesterday"
---

`format: date-time`. Format assertion is off by default in most JSON Schema
libraries, so this fixture is also a check that the validator turned it on.
