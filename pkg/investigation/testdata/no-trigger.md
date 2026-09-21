---
id: INV-0011
title: "One parity fixture repository per built-in type"
status: Concluded
author: Donald Gifford
created: 2026-09-20
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0011: One parity fixture repository per built-in type

**Status:** Concluded
**Author:** Donald Gifford
**Date:** 2026-09-20

<!--docz:question:start-->
## Question

Does `test/parity` need one fixture repository per built-in document type, or
would a single repository carrying all six types record the same behaviour?
<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

One repository is enough. Every command under test takes a type name, so six
types in one repository walk the same code paths six repositories would, and the
suite would shed two thirds of its files.
<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

The suite was captured from the v1.2.2 release binary with seven fixture
repositories: one per built-in type, plus one carrying a custom `frameworks`
type. Each repository holds a template-faithful document and a messier
hand-written one, which is 14 documents to keep in step for a suite that only
replays goldens.
<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Count the distinct command invocations each fixture repository contributes.
2. Collapse the six type repositories into one and re-capture the goldens.
3. Diff the collapsed goldens against the ones captured from v1.2.2.
<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| Go | 1.25.1 |
| docz | v1.2.2 |
| Platform | darwin/arm64 |
<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### The file tree is most of every golden

A golden records stdout, stderr, the exit code, and the whole file tree. In a
collapsed repository every case's tree carries the other five types' documents,
so a change to the RFC index table moves a line in the ADR goldens too. The
per-type trees are what makes a failing diff name the command that broke.

### Collapsing does not change a single stdout

The 213 stdout and exit-code captures came back byte-identical. The hypothesis
is right about coverage and wrong about cost: nothing is gained in what the
suite measures, and the diffs stop being readable.
<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

Coverage is unchanged, so the collapse is possible; readability is not, so it is
not worth doing. The seven repositories stay.

**Answer:** No, one repository per type is not required for coverage, but the
per-type trees are what keeps a failing diff readable.
<!--docz:conclusion:end-->

<!--docz:references:start-->
## References

- `test/parity/README.md` — provenance of the captured goldens
- [ADR-0002](../adr/0002-v200-api-first-and-the-v2-module-path.md)
<!--docz:references:end-->
