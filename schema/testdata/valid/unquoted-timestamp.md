---
id: note-9001
kind: note
title: Timestamps written without quotes must still validate
state: active
created: 2026-07-29T20:00:00Z
updated: 2026-07-30T02:00:00Z
---

yaml.v3 resolves an unquoted RFC3339 scalar to the `!!timestamp` tag and hands
back a `time.Time`, which is not a JSON type and which a JSON Schema validator
rejects outright. That is a reader bug, not a ledger error: this file is valid.

dira quotes timestamps on write. It must not require them on read, because a
human editing an entry by hand will not quote them.
