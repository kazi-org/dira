---
control: 0002-thin-reasons
bare: 0
thin: 3
reasoned: 0
revisit: 0
label_expansion_needed: false
---

# ADR 2: Choose how content gets re-indexed

## Context and Problem Statement

Search results lag behind edits by an amount nobody had measured until users
started complaining. Three approaches were considered, each dismissed with a
one-line reason that says no more than the option's own name already implied.

## Alternatives rejected

- Nightly batch processing job — too slow
- Continuous polling loop — resource heavy
- Manual reviewer queue — not scalable

## Decision

We chose to re-index on write, synchronously, and measure the latency cost
before optimising further.
