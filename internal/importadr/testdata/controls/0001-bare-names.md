---
control: 0001-bare-names
bare: 5
thin: 0
reasoned: 0
revisit: 0
label_expansion_needed: false
---

# ADR 1: Choose a message queue for the ingest pipeline

## Context and Problem Statement

We need to select a message queue technology for the new document ingest
pipeline. Several options were discussed in a design review but none was
written up with its reasoning at the time.

## Alternatives rejected

- Migrate to a managed Kafka cluster
- Build a custom queue service in-house
- Use AWS SQS with Lambda triggers
- Adopt NATS JetStream for streaming
- Continue running the existing RabbitMQ cluster unmodified

## Decision

We chose to keep the existing PostgreSQL-backed job queue for now, and revisit
this list once ingest volume grows past what it comfortably handles.
