---
control: c5-madr-reasons-elsewhere
bare: 0
thin: 0
reasoned: 1
revisit: 0
label_expansion_needed: false
---

# Use event-driven ingestion

## Context and Problem Statement

How should new documents enter the search pipeline: on a schedule, or as they
arrive?

## Considered Options

* Nightly batch import job
* Event-driven ingestion pipeline

## Decision Outcome

Chosen option: "Event-driven ingestion pipeline", because it minimizes the
latency between document creation and searchability.

## Pros and Cons of the Options

### Nightly batch import job

* Bad, because it introduces up to twenty-four hours of latency before a new
  document is searchable.

### Event-driven ingestion pipeline

* Good, because documents become searchable within seconds of creation.
