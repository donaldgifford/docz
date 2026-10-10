---
id: INV-0029
title: "Executable acceptance in docz documents"
status: Open
author: Donald Gifford
created: 2026-10-10
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0029: Executable acceptance in docz documents

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: what is executable today](#observation-1-what-is-executable-today)
  - [Observation 2](#observation-2)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. What does docz do with executable acceptance?](#1-what-does-docz-do-with-executable-acceptance)
  - [2. How is an expected result written?](#2-how-is-an-expected-result-written)
  - [3. Do DESIGN documents get a worked-example region?](#3-do-design-documents-get-a-worked-example-region)
  - [4. Who may run the commands?](#4-who-may-run-the-commands)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Should docz documents carry acceptance that a tool can run? That means
commands with stated expected results, which `docz` can list, run on
request, and report on, so a document says how to tell it was implemented
correctly and the claim can be checked. If so, what shape should that
take, given what IMPL tasks, success criteria, and runbook steps already
hold?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

docz is closer than it looks:

- IMPL tasks already carry verify commands;
- success criteria already distinguish an executable criterion (one that
  opens with a backtick span) from a statement;
- runbook steps already pair commands with an `**Expected:**` line.

What is missing is a command that collects them, runs them on request,
and compares the results with what the document said.

SMART's "reconciliation anchors" (expected outputs stated exactly and
enforced by generated tests) are a stronger form. They fit DESIGN
documents with worked examples, and need a region kind docz does not have
yet.

Running commands taken from a document is running code taken from text,
so it should be local, explicit, and never done by docz-api.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** [INV-0027](0027-interactive-mode.md) Observation 10,
the SMART paper ([arXiv 2609.05364](https://arxiv.org/abs/2609.05364)).

In SMART, "every number-bearing doc ends with a reconciliation anchor: a
small preset whose expected outputs are stated exactly and enforced by
generated tests". Worked examples in the documents act as demonstrations
for the agents that regenerate the code. That is what makes regeneration
from documents trustworthy.

docz's own loop is close to this already: IMPL documents drive the
implementation loop, and each phase ends with success criteria. But
whether a criterion holds is still checked by whoever, or whatever, reads
it.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. List what is executable in docz documents today and which reader
   exposes it.
2. Count, over this repository's IMPL documents, how many tasks have
   verify lines and how many criteria are executable. Sample whether
   those commands still run as written.
3. Compare runbook `**Expected:**` lines with SMART's exact expected
   outputs. Decide whether one shape can serve both.
4. Decide what a worked example or anchor region would look like in a
   DESIGN document, and what the validator should require of it.
5. Settle the safety rules for running commands taken from documents.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz | `v2.0.0-beta.8` |
| Corpus | this repository's `docs/impl/` and `docs/runbook/` |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: what is executable today

| Where | Shape | Reader |
| --- | --- | --- |
| IMPL task | A verify line naming a command | `impl.Task.Verify`; `impl.task.verify-no-command` flags a verify line with no command |
| Success criteria (IMPL phase, RFC) | A criterion opening with a backtick span, such as "`just ci` passes" | `kinds.Criterion{Executable, Command}`; a backtick span mid-sentence is deliberately not a command |
| Runbook step | Fenced commands under a step, and an `**Expected:**` line | `runbook.Step.Commands[]` and `Step.Expected` |

All three can be read today. None of them is run by docz, and only the
runbook states an expected result.

### Observation 2

<!-- Approach step 2: counts, and whether the commands still run. -->

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Inconclusive. The investigation is open, and Approach steps
2 to 5 decide it.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

<!--docz:open-questions:start-->
## Open Questions

### 1. What does docz do with executable acceptance?

- **a. `docz verify <id>` lists the document's commands and expected
  results, and runs them only with `--run`, locally, showing each command
  before it runs.** *(recommendation)*
- b. Only list them, for an agent or CI to run.
- c. Generate test files from them, as SMART does.
- d. Other.

### 2. How is an expected result written?

- **a. Reuse the runbook's `**Expected:**` line everywhere, with an exit
  code by default and exact output when the line says so.**
  *(recommendation)*
- b. A fenced `expected` block with exact output, compared byte for byte.
- c. Other.

### 3. Do DESIGN documents get a worked-example region?

- **a. Yes, an optional `example` region kind holding inputs, steps, and
  an expected result, which the validator checks for shape only.**
  *(recommendation)*
- b. Not yet: start with IMPL and runbooks, which already have the parts.
- c. Other.

### 4. Who may run the commands?

- **a. Only the local CLI, on an explicit flag. docz-api never runs
  them, and docz-review only displays them.** *(recommendation)*
- b. The local CLI and CI, where CI runs them in a sandbox.
- c. Other.

<!--docz:open-questions:end-->

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [INV-0027](0027-interactive-mode.md): docz-review, where the idea came
  up
- [Design Docs Are All You Need](https://arxiv.org/abs/2609.05364):
  worked examples and reconciliation anchors
- [DESIGN-0019](../design/0019-runbook-a-sixth-built-in-document-type-disabled-by-default.md):
  runbook steps, commands, and expected lines
- [DESIGN-0015](../design/0015-structured-regions-and-docz-validate.md):
  regions and the validator

<!--docz:references:end-->
