---
id: RFC-0001
title: Grammar fixture
status: Draft
author: Test Author
created: 2026-09-20
---

<!-- markdownlint-disable-file MD025 MD041 -->

# RFC-0001: Grammar fixture

<!--toc:start-->
<!--toc:end-->

<!--docz:summary:start-->
## Summary

Publish a typed reader for RFC documents, so a consumer reads a proposal as a
value with one field per section rather than as markdown it has to walk itself.
<!--docz:summary:end-->

<!--docz:problem:start-->
## Problem Statement

Every consumer that wants the risks out of a proposal parses the table itself,
and each one disagrees about which column is which.

### Supporting Data

Three consumers ship a table reader of their own today, and two of them read
the mitigation column as the third rather than the fourth.
<!--docz:problem:end-->

<!--docz:proposal:start-->
## Proposed Solution

Ship `pkg/rfc` with one field per section of the template, the shared sections
read by the `kinds` package so they mean the same thing in every type.
<!--docz:proposal:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **A. Leave the parsing to each consumer.** Rejected: three readers already
  disagree about the risks table, which is the problem rather than the fix.
- **Ship a generic markdown model.** Rejected: a consumer would still have to
  know which heading held the risks.
- Wait for the schema work to land first.
<!--docz:alternatives:end-->

<!--docz:risks:start-->
## Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | ---------- | ---------- |
| The template is renamed | High | Low | A test pins the heading table to it |
| A legacy document carries no markers | Medium | High | Inference reads it by heading |
| The risks table grows a fifth column | Low | Low |  |
|      |        |            |            |
<!--docz:risks:end-->

<!--docz:criteria:start-->
## Success Criteria

- `go test ./pkg/rfc/...` passes
- Every RFC in the corpus parses with no finding its author did not earn
<!--docz:criteria:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Should the risks table be read by column name?

- a. No, by position *(recommendation)*: the corpus spells the headers four
  different ways.
- b. Yes, by name, and report a table whose headers do not match.

> **Resolved 2026-09-20: (a)** reading by name would drop every row of the two
> documents that write "Likelihood" as "Probability".

### 2. Should an empty mitigation be an error rather than a warning?

- a. Warning: a risk listed before anyone knows what to do about it is how the
  section gets written.
- b. Error, so an accepted proposal cannot carry one.
<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0014](../design/0014-the-docz-api-as-one-unit.md)
- The RFC template, whose sections this fixture mirrors
<!--docz:references:end-->
