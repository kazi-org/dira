---
control: 0003-rich-with-revisit
bare: 0
thin: 0
reasoned: 3
revisit: 3
label_expansion_needed: false
---

# ADR 3: Choose how the ingest queue is hosted

## Context and Problem Statement

The ingest pipeline needs a durable queue. Three options were weighed in full,
each with its own grounds and each with a condition under which the choice
should be revisited.

## Alternatives rejected

- Adopt a fully managed Kafka cluster — Operating our own broker fleet would
  consume roughly two engineers' worth of on-call time we do not currently
  have to spare, and a managed offering removes that burden entirely for a
  modest monthly premium. Revisit if the vendor's pricing model changes
  materially or self-hosting becomes clearly cheaper.
- Build a custom in-house queue service — Nothing on the market matches our
  exact latency and ordering guarantees today, but a bespoke service means
  every future engineer has to learn a system found nowhere else, and the
  maintenance burden compounds every year it survives. Revisit if a
  mainstream broker adds the ordering guarantee we need.
- Route everything through a single Postgres table as a queue — Postgres
  already runs everywhere in this stack, and adding no new infrastructure has
  real value, but polling a table under real load starts contending with the
  write path within the first few thousand jobs. Revisit if throughput
  requirements drop below one hundred jobs per minute.

## Decision

We chose to run our own broker on cloud VMs we already operate, accepting the
on-call cost in exchange for full control over ordering semantics.
