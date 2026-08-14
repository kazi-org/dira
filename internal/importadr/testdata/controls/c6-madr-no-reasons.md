---
control: c6-madr-no-reasons
bare: 2
thin: 0
reasoned: 0
revisit: 0
label_expansion_needed: false
---

# Use event-driven ingestion

## Context and Problem Statement

How should new documents enter the search pipeline: on a schedule, or as they
arrive?

## Considered Options

* Nightly batch import job
* Weekly bulk reconciliation pass
* Event-driven ingestion pipeline

## Decision Outcome

Chosen option: "Event-driven ingestion pipeline", because it minimizes the
latency between document creation and searchability.
