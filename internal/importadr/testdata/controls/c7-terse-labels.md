---
control: c7-terse-labels
bare: 5
thin: 0
reasoned: 0
revisit: 0
label_expansion_needed: true
median_label_words: 1
---

# ADR: Choose a datastore for the search index

## Context and Problem Statement

The team ran a quick spike comparing five candidates by name, in a
stand-up, with no write-up of the reasoning at the time.

## Alternatives rejected

- Go
- Rust
- Redis
- PostgreSQL full-text search
- Elasticsearch cluster

## Decision

We chose to keep using the existing OpenSearch cluster.
