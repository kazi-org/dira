---
id: note-9013
kind: note
title: Frontmatter with no closing delimiter must be an error
state: active
created: "2026-07-29T20:00:00Z"

The closing `---` is missing, so everything below is being parsed as YAML.
This is the shape a truncated write leaves behind.
