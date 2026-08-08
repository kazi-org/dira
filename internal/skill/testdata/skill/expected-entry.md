---
id: dec-0002
kind: decision
title: Lexical matching in the binary, not an agent adjudicating the exit code
state: staged
created: "2026-08-08T18:27:54Z"
edges:
  - type: derives_from
    to: dec-0001
    note: the regex capture this extraction supplies the reasoning for
alternatives:
  - option: An agent adjudicating the exit code
    why_not: >
      A verdict that needs a live session produces nothing at a terminal with the
      network unplugged.
source:
  hook: PreCompact
  excerpt: >
    We settled on lexical matching in the binary rather than an agent adjudicating
    the exit code, because the non-zero exit is the product and a verdict that
    needs a live session produces nothing at a terminal with the network
    unplugged.
  tier: semantic
---

The non-zero exit is the product, and a verdict that needs a live session produces nothing at a terminal with the network unplugged.
