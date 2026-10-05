---
id: INV-0016
title: "One ADR per built-in document type"
status: Open
author: Donald Gifford
created: 2026-10-02
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0016: One ADR per built-in document type

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: where each type is recorded today](#observation-1-where-each-type-is-recorded-today)
  - [Observation 2: the removal criterion exists, the inclusion criterion is implicit](#observation-2-the-removal-criterion-exists-the-inclusion-criterion-is-implicit)
  - [Observation 3: the evidence is uneven](#observation-3-the-evidence-is-uneven)
  - [Observation 4: the ADR template fits without changes](#observation-4-the-adr-template-fits-without-changes)
  - [Observation 5: the drift risk is in listing kinds](#observation-5-the-drift-risk-is-in-listing-kinds)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [1. One ADR per type, or one for the catalogue?](#1-one-adr-per-type-or-one-for-the-catalogue)
  - [2. How is the structure described?](#2-how-is-the-structure-described)
  - [3. Order and status](#3-order-and-status)
  - [4. What evidence does each cite?](#4-what-evidence-does-each-cite)
  - [5. One PR or six?](#5-one-pr-or-six)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Should each of the six built-in document types get an ADR recording what it
is for, why it is built in rather than custom, and why its structure is the
one it has, and what shape must those ADRs take to add a record the DESIGN
documents and the templates do not already hold?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Yes. The catalogue has a decision record for the one type that was removed
(ADR-0003) and for the rule that every built-in is structured (ADR-0002 R7),
but not for any type that is in it. The purpose of each type is a table row
in DESIGN-0001 or a paragraph in a later design, and the structure is in the
template and its marker skeleton, which are executable but say nothing about
why. Six short ADRs fill that gap, and the risk is the obvious one: an ADR
that restates the skeleton drifts from it, and `docz validate` cannot see
prose. So the ADRs should say what each type is for and what its structure
is meant to do, and link the skeleton for what it is.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** issue
[#146](https://github.com/donaldgifford/docz/issues/146), from DESIGN-0019
Open Question 13.

DESIGN-0019 asked whether runbook needed an ADR and resolved (d): no ADR for
runbook alone, because the better record is one ADR per built-in type,
stating what each is for and why it is built in. The design stays runbook's
record until that lands. This investigation works out what the six ADRs
contain, in what order, and with what evidence, before anyone writes them.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. For each type, find where its purpose and its structure are recorded
   today, and whether any decision record exists.
2. Read ADR-0003's removal argument and invert it: what is the criterion for
   a type to be built in?
3. Fit the content to the ADR template: Summary, Context, Decision with
   Supporting Data, Consequences, Alternatives, References.
4. Gather the evidence an ADR can cite: document counts per type in this
   repository, and across the repositories docz-api indexes.
5. Decide granularity, order, status, and how the structure is described
   without duplicating the skeleton.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| Registry | `pkg/doczcore/config/doctype.go`, six entries: rfc, adr, design, impl, investigation, runbook (disabled) |
| Existing ADRs | ADR-0001 to ADR-0004; the next is ADR-0005 |
| Documents here | rfc 0, adr 4, design 19, impl 21, investigation 19, runbook 2 (2026-10-02) |
| Templates and skeletons | `pkg/doczcore/doctemplate/templates/<type>.md` and `templates/schema/<type>.md` |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: where each type is recorded today

| Type | Purpose recorded in | Structure recorded in | Decision record |
| ---- | ------------------- | --------------------- | --------------- |
| rfc | DESIGN-0001 Document Types table, one row | `rfc.md`, `schema/rfc.md`, `pkg/rfc` | none |
| adr | DESIGN-0001, one row | `adr.md`, `schema/adr.md`, `pkg/adr` | none |
| design | DESIGN-0001, one row | `design.md`, `schema/design.md`, `pkg/design` | none |
| impl | DESIGN-0001, one row | `impl.md`, `schema/impl.md`, `pkg/impl` | none |
| investigation | added in 0.0.3 (2026-03-07) with no design; the registry's help text | `investigation.md`, `schema/investigation.md`, `pkg/investigation` | none |
| runbook | DESIGN-0019 §1 to §4 | `runbook.md`, `schema/runbook.md`, `pkg/runbook` | DESIGN-0019 OQ 13, deferred to this |
| plan | DESIGN-0001, one row | removed | ADR-0003 |

The registry's `HelpDescription` is the only place all six purposes sit
together, and it is one clause each. DESIGN-0014 §2 and DESIGN-0015 record
why every built-in is structured and what a region is, for the set rather
than per type.

### Observation 2: the removal criterion exists, the inclusion criterion is implicit

ADR-0003 removed `plan` on evidence: nobody had written one, phased planning
happened in IMPL documents, and the name collided with IMPL's vocabulary.
The inverse — what earns a type its place in the catalogue — was never
written down. DESIGN-0019 argued it for runbook (a distinct purpose, a shape
no other type has, a fleet that would use it) and that argument generalises:
a built-in has a purpose no other type serves, a structure worth a typed
reader, and documents in the fleet. Each ADR can state those three for its
type, and that is the content the DESIGN documents lack.

### Observation 3: the evidence is uneven

This repository has no RFC and never has: the RFC index table is empty. ADR
has four, all written since July 2026. The argument ADR-0003 made against
`plan` would, on this repository's numbers alone, apply to `rfc` as well.
The difference is the fleet: `rfc` was the first type in DESIGN-0001, the
repositories docz-api indexes may carry RFCs, and docz-api's own `rfc-api`
lineage says the name predates this tool. So the Supporting Data for each
ADR needs counts from the fleet, not from here, and the `rfc` ADR in
particular has to make its case or honestly record that the type is kept for
the fleet rather than for this repository.

### Observation 4: the ADR template fits without changes

Summary states the type in two sentences. Context is the need the type
meets and what people wrote before it existed. Decision is "built in,
enabled by default or not, with this structure", and Supporting Data is the
fleet counts and the documents that shaped the template. Consequences:
positive (a typed reader, validation, a site view), negative (one more
template and skeleton to keep in step, one more EXPERIMENTAL package),
neutral (a custom type with the same regions parses with the same package,
ADR-0002 R7). Alternatives are the ones every type has: a custom type in each
repository, or folding into a neighbour (runbook into impl, investigation
into design). The ADR about `adr` describes its own form, which is fine and
worth one sentence.

### Observation 5: the drift risk is in listing kinds

If an ADR enumerates its type's region kinds, the list is a second copy of
the skeleton with no test behind it. The template and skeleton are already
pinned to each other by the derivation test, and the type package's
`headings` table is pinned to the template. The ADR should state the intent
of the structure (an IMPL is phases of tasks with criteria; a runbook is
procedures of addressable steps) and link `templates/schema/<type>.md` and
the package for the exact shape. Then a skeleton change needs no ADR
amendment unless the intent changes, which is when an amendment is right
anyway.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes, provisionally. Six ADRs, one per type, are the right
record (Observation 1 shows none exists), each stating the inclusion
criterion ADR-0003 only implied (Observation 2), with fleet evidence
(Observation 3), on the unchanged template (Observation 4), and describing
the structure's intent rather than its kinds (Observation 5). The questions
below settle the shape; the investigation concludes when they are resolved
and the first ADR, runbook's, is written against them.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Write ADR-0005 for runbook first, from DESIGN-0019, and use it as the form
for the other five. Gather the fleet counts once, through docz-api, and cite
them in every ADR's Supporting Data. Land all six in one documentation PR.

### 1. One ADR per type, or one for the catalogue?

- a. **One ADR per type, six in all**, as DESIGN-0019 Open Question 13
  resolved. Each is short and can be superseded on its own, the way
  ADR-0003 superseded `plan`'s row. *(recommendation)*
- b. One ADR, "The built-in catalogue", with a section per type. Fewer
  documents, but removing or changing one type means amending a record
  about all of them.
- c. No ADRs; amend DESIGN-0001 and DESIGN-0019 with the inclusion criterion.
- d. Other.

### 2. How is the structure described?

- a. **Intent in prose, exact shape by link**: what the structure is for,
  then links to `templates/schema/<type>.md` and `pkg/<type>`. No list of
  region kinds. *(recommendation)*
- b. List the region kinds and their rules, accepting the drift.
- c. Other.

### 3. Order and status

- a. **Runbook first (ADR-0005), then rfc, adr, design, impl, investigation
  in registry order (ADR-0006 to ADR-0010), all `Accepted` on landing**,
  since each records a decision already in effect. *(recommendation)*
- b. `Proposed` on landing, accepted after review.
- c. Registry order throughout, runbook last.
- d. Other.

### 4. What evidence does each cite?

- a. **Document counts per type across the repositories docz-api indexes**,
  gathered once and dated, plus the documents that shaped the template
  (the corpus in each `pkg/<type>/testdata/README.md`). *(recommendation)*
- b. This repository's counts only.
- c. No counts; purpose and structure only.
- d. Other.

### 5. One PR or six?

- a. **One `docs/` PR with all six**, labelled `dont-release`, so the set is
  reviewed as a set. *(recommendation)*
- b. One PR per ADR.
- c. Other.

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [#146](https://github.com/donaldgifford/docz/issues/146): this issue
- [DESIGN-0019](../design/0019-runbook-a-sixth-built-in-document-type-disabled-by-default.md)
  Open Question 13: the resolution that asks for one ADR per type
- [ADR-0003](../adr/0003-remove-plan-from-the-built-in-document-types.md):
  the removal criterion
- [ADR-0002](../adr/0002-docz-is-an-api-package-whose-first-consumer-is-the-cli.md):
  Decision 7 and R7, every built-in is a structured type
- [DESIGN-0001](../design/0001-docz-cli-design.md): the original four types;
  [DESIGN-0014](../design/0014-the-docz-api-as-one-unit-packages-types-functions-and-the-cmd.md)
  and [DESIGN-0015](../design/0015-structured-regions-and-docz-validate.md):
  type packages and regions
- [`pkg/doczcore/config/doctype.go`](../../pkg/doczcore/config/doctype.go):
  the registry

<!--docz:references:end-->
