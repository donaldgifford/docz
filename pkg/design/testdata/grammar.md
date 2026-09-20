---
id: DESIGN-0001
title: Grammar fixture
status: Draft
author: Test Author
created: 2026-09-20
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0001: Grammar fixture

<!--toc:start-->
<!--toc:end-->

<!--docz:overview:start-->
## Overview

A design document that exercises every field of the design package in one
place, so the shared field rules are pinned by something a person can read.
<!--docz:overview:end-->

## Goals and Non-Goals

<!--docz:goals:start-->
### Goals

- Read every section of the DESIGN template into its own field
- Keep the detailed design region whole, numbered subsections and all
<!--docz:goals:end-->

<!--docz:non-goals:start-->
### Non-Goals

- A grammar for the numbered subsections
<!--docz:non-goals:end-->

<!--docz:background:start-->
## Background

Five type packages share one shape, and design is the one whose longest
section has no structure docz reads.
<!--docz:background:end-->

<!--docz:detailed-design:start-->
## Detailed Design

The package is a switch over region kinds and nothing else.

### 1. Parsing

Parse resolves the document's regions once and fills one field per kind,
rebasing every line onto the document as it goes.

### 2. Validating

Validate runs Parse and reports the three rules that need a typed model.
<!--docz:detailed-design:end-->

<!--docz:api-changes:start-->
## API / Interface Changes

Three exported functions — `Parse`, `Validate`, and `Headings` — plus the
`Doc` value they are about.
<!--docz:api-changes:end-->

<!--docz:data-model:start-->
## Data Model

`Doc` holds one field per section and nothing is written to disk, so there is
no schema to migrate.
<!--docz:data-model:end-->

<!--docz:testing:start-->
## Testing Strategy

A marked fixture, the same fixture with its markers stripped, and one case per
finding code.
<!--docz:testing:end-->

<!--docz:rollout:start-->
## Migration / Rollout Plan

The package ships with the v2 line, so no existing document has to change.
<!--docz:rollout:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Does the detailed design region keep its subsections?

> **Resolved 2026-09-20: (a)** The numbering is the author's, so a reader that
> split on it would be inventing a grammar.

- a. Report the region's body whole, headings included. *(recommendation)*
- b. Split it into sections keyed by the number.
- c. Other.

### 2. Where does an unresolved question's finding point?

- a. At the question's own heading. *(recommendation)*
- b. At the frontmatter status line that contradicts it.
- c. Other.
<!--docz:open-questions:end-->

<!--docz:decisions:start-->
## Decisions

| # | Question | Decision |
| --- | --- | --- |
| 1 | Does the region keep its subsections? | Yes, the body is reported whole |
<!--docz:decisions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0014](../design/0014-the-docz-api-as-one-unit.md)
<!--docz:references:end-->
