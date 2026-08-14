---
control: 0004-no-section
bare: 0
thin: 0
reasoned: 0
revisit: 0
label_expansion_needed: false
---

# ADR 4: Use PostgreSQL as the primary datastore

## Status

Accepted

## Context

The service needs a primary datastore for structured records with
transactional guarantees. The team already operates PostgreSQL for two other
services, and nothing about this workload looks unusual enough to warrant a
different engine.

## Decision

We will use PostgreSQL as the primary datastore, with a single writable
instance and read replicas added if read load ever justifies them.

## Consequences

Operational knowledge already exists in the team, so onboarding cost is low.
A future workload that needs a fundamentally different data model — a graph
of relationships, or a document store with no fixed schema — is not served
well by this choice and would need its own datastore alongside this one.
