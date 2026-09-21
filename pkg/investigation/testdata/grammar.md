---
id: INV-0001
title: Grammar fixture
status: Concluded
author: Test Author
created: 2026-09-20
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0001: Grammar fixture

<!--toc:start-->
<!--toc:end-->

<!--docz:question:start-->
## Question

Can a type package report document line numbers for a region it read through
the kinds readers, without the offset being applied twice?
<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

It can. Every reader numbers lines from the start of the bytes it was handed,
so one addition of the region's start is enough at any depth.
<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

The five type packages all publish a Line a consumer acts on, and IMPL-0018
Phase 1 needs the arithmetic settled before any goldens are captured.

**Triggered by:** DESIGN-0014 / issue #101
<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Reproduce the drift with a fixture whose regions nest two deep.
2. Instrument every reader and record the line it reports.
3. Compare each reported line against the file the fixture was read from.
<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| Go | 1.25.1 |
| docz | v1.2.2 |
| Platform | darwin/arm64 |
|  |  |
<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### The offset belongs to the caller

Each reader reported a line one short of the file, because the region's start
was never added: the reader cannot add it, since it was handed bytes and not
a document.

### Adding it in the reader double-counts a nested region

A region nested inside another had its parent's start added once by the
walker and once by the reader, so the reported line sat two lines above the
bullet a consumer wanted to jump to.
<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

The arithmetic is right once the region's start is added exactly once, by the
caller that owns the region rather than by the reader that walked it.

**Answer:** Yes, with the shift applied by the caller.
<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Give kinds a Shift helper per value type and have every type package call it,
so the conversion has one definition instead of five.
<!--docz:recommendation:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Should the readers report document lines themselves?

- a. No, the caller shifts them. *(recommendation)*
- b. Yes, pass the region into every reader.

> **Resolved 2026-09-20: (a)** the readers stay bytes-in, values-out.

### 2. Does a third level of nesting need a rule of its own?

- a. No, the arithmetic is the same at any depth. *(recommendation)*
- b. Yes, cap the nesting at two.
<!--docz:open-questions:end-->

<!--docz:decisions:start-->
## Decisions

| # | Question | Decision |
| --- | --- | --- |
| 1 | Should the readers report document lines? | No, the caller shifts them |
<!--docz:decisions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0014](../design/0014-v200-the-docz-api-as-one-unit.md)
<!--docz:references:end-->
