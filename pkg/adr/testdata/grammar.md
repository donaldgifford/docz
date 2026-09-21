---
id: ADR-0002
title: "Read ADR sections by region rather than by heading"
status: Proposed
author: Test Author
created: 2026-09-20
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0002: Read ADR sections by region rather than by heading

<!--docz:summary:start-->
## Summary

Locate an ADR's sections by their docz region markers, and fall back to the
heading table only for a document that carries none. What a reader sees does
not change; what changes is that renaming a section stops silently emptying a
field.
<!--docz:summary:end-->

<!--docz:context:start-->
## Context

<!-- What is the issue that we're seeing that motivates this decision? -->

Every field of the typed model used to be found by matching heading text, so a
document whose author wrote "## The Decision" parsed with an empty decision
and said nothing about it.
<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

Regions are authoritative. A document that carries markers is read by them,
and one that carries none is read by the heading table the type package ships.

### Supporting Data

Of the 41 documents in the corpus, 38 spell every heading the way the template
does and 3 do not.
<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- A renamed section becomes a finding instead of a silently empty field
- One parser reads a marked document and a legacy one
<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- Every document in the fleet eventually wants markers, which is a migration
  nobody has scheduled
<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- The heading table is written out in the type package rather than derived
<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **a. Match on heading text only.** Ship the table and stop. Rejected: a
  renamed section stays invisible, which is the problem.
- **b. Require markers everywhere.** Every legacy document would have to be
  rewritten before it could be read at all.
- **c. Infer from position.** The third section is the decision. Rejected: a
  document that adds a section moves every field after it.
<!--docz:alternatives:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Where does the fallback heading table live?

- a. In the type package, written out and pinned to the template by a test
  *(recommendation)*
- b. In the core, derived from the embedded template at run time

> **Resolved 2026-09-20: (a).** A type package's production imports stop at
> the core, so deriving the table would mean importing the template embed.

### 2. Is an unmarked region a warning or an error?

- a. A warning, since inference is permanent rather than a migration aid
- b. An error once every document in the corpus carries markers
<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [ADR-0001](0001-pkgdoczcore-as-the-single-public-core.md) — the layer rules
  this decision works inside
- [DESIGN-0014](../design/0014-the-docz-api-as-one-unit.md) — the field rules
  every type package follows
<!--docz:references:end-->
