---
id: INV-0028
title: "A document graph from the references docz already parses"
status: Concluded
author: Donald Gifford
created: 2026-10-10
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0028: A document graph from the references docz already parses

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: where relationships are written today](#observation-1-where-relationships-are-written-today)
  - [Observation 2: most edges are already declared, and none dangle](#observation-2-most-edges-are-already-declared-and-none-dangle)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. Where do edges come from?](#1-where-do-edges-come-from)
  - [2. Where does the graph live?](#2-where-does-the-graph-live)
  - [3. What does the CLI print?](#3-what-does-the-cli-print)
  - [4. Does the validator use it?](#4-does-the-validator-use-it)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:question:start-->
## Question

Can docz build a dependency graph of a repository's documents from what
it already parses? That means typed edges such as "triggered by",
"supersedes", "implements", and "references", with no new syntax and no
agent guessing. What would the graph serve: the CLI, the validator,
docz-site, and the orchestration in INV-0023?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Mostly yes. Every relationship between docz documents is already written
down somewhere docz reads:

- a "Triggered by" field;
- a reference list;
- a superseded ADR's pointer to its replacement;
- a document ID mentioned in prose.

What is missing is the step that collects them into one graph with typed
edges. The SMART paper infers its edges with read-only agents because its
documents have no structure to read. docz should not need to.

Some edges will be ambiguous. "IMPL-0024 implements DESIGN-0021" is
usually only a mention in prose, and a mention is a weaker edge than a
declared one. A small, optional frontmatter vocabulary may be worth adding
for the edges that matter most.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** [INV-0027](0027-interactive-mode.md) Observation 10,
the SMART paper ([arXiv 2609.05364](https://arxiv.org/abs/2609.05364)).

SMART's repository is a DAG of design documents. An orchestrator walks it
in topological order to regenerate code, and the edges are
"machine-discovered rather than hand-maintained": read-only agents read
the prose and infer them. docz's documents already link to each other
deliberately, and the lineage views in the docz deck draw those links by
hand. Nothing in docz computes them.

Uses for the graph, if it existed:

- **Impact:** "what depends on DESIGN-0015?" before changing it.
- **Validation:** a reference to an ID that does not exist, or to a
  superseded document as if it were current.
- **docz-site:** a lineage panel on each document, alongside the existing
  cross-reference links.
- **Orchestration:** INV-0023's workflows run in dependency order, as
  SMART's orchestrator does.
- **Agents:** read a document's ancestors before revising it, through
  `docz inspect` (INV-0027).

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. List every place a relationship is written today, and the reader (if
   any) that parses it.
2. Run a throwaway scan over this repository's `docs/` that extracts every
   candidate edge. Count how many are typed by structure and how many are
   bare mentions.
3. Sample the bare mentions by hand and classify them: implements,
   depends on, supersedes, or merely cites.
4. Decide whether structure plus mentions is enough, or whether a few
   frontmatter fields are needed.
5. Sketch the output (`docz graph`, JSON, Mermaid, DOT) and the
   validator findings it enables.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz | `v2.0.0-beta.8` |
| Corpus | this repository's `docs/` (investigation, design, impl, adr, rfc, runbook) |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: where relationships are written today

| Relationship | Where it is written | Parsed by |
| --- | --- | --- |
| Triggered by | INV `**Triggered by:**` field | `investigation.Doc.TriggeredBy` (text only) |
| Supersedes | A superseded ADR's reference to its replacement | `adr` checks it exists (`adr.superseded.no-reference`) but does not report the target |
| References | The `references` region's list | `kinds.References` gives `Reference{Text, URL}` |
| Mentions | Any `PREFIX-NNNN` token in prose | docz-site's `xrefs.ts` links tokens that resolve to an existing document; nothing in Go collects them |
| Frontmatter | `id`, `title`, `status`, `author`, `created`, `schema` | No relationship fields |

The raw material exists, but every reader stops at text. None of them
resolves a target to a document ID.

### Observation 2: most edges are already declared, and none dangle

A scan of this repository's `docs/` on 2026-10-15 (81 documents across
the six types, the archive excluded), counting each target once per
document and never a document's own id:

| Edge source | Count |
| --- | --- |
| `**Triggered by:**` fields | 29 of 29 investigations |
| Ids named in a `references` region | 334 |
| Ids mentioned in prose only, outside the references region | 235 |
| Superseded ADRs | 0 |
| Targets that resolve to no document | 0 |

So three in five cross-document links are declared in a list docz
already parses, and the rest are prose mentions. Not one target is
missing, which means the validator warning in question 4 would fire on
nothing today and exists to catch the first typo. Sampling the prose
mentions by hand (Approach step 3) was not done: with the reference list
this well kept, a mention is worth exactly what it says, a citation, and
a typed "implements" or "depends on" edge should come from structure
(the `**Triggered by:**` field, an IMPL's design reference) rather than
from guessing at a sentence.

One trap the counts expose: ids are not unique across the repository.
`docs/archive/api/` holds docz-api's records under the same prefixes, so
`IMPL-0001` names two files. A graph has to be built over the enabled
types' directories, as `repo.List` reads them, and never over every
markdown file under `docs/`.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes. Decided 2026-10-15. Every edge the graph needs is
already written where docz reads (Observation 1), and the corpus shows
the declared edges outnumber the prose mentions with no dangling target
(Observation 2). No new syntax is needed and no agent is involved.

Next: a short DESIGN for a public reader in `pkg/doczcore` with typed
edges from structure and untyped mentions from prose, a `docz graph`
command over it, and two validator warnings. The reader's structure
should ride `docz inspect --format json` from INV-0027, so one JSON
contract serves the review page, the graph, and docz-site's lineage
panel. Approach steps 3 and 4 are closed by Observation 2's reading.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

<!--docz:open-questions:start-->
## Open Questions

### 1. Where do edges come from?

- **a. Structure first, mentions second: typed edges from the fields and
  regions docz already reads, plus untyped "mentions" edges from IDs in
  prose, kept distinct.** *(recommendation)*
- b. (a) plus optional frontmatter fields (`supersedes:`, `implements:`,
  `depends_on:`) for edges that must be explicit.
- c. Frontmatter fields only.
- d. Agent inference, as SMART does.
- e. Other.

> **Resolved 2026-10-15: (a).** Structure first, mentions second, kept
> distinct. Frontmatter fields wait until the graph has been used and a
> missing edge is felt.

### 2. Where does the graph live?

- **a. A public reader in `pkg/doczcore` that takes the documents and
  returns nodes and typed edges, so the CLI, docz-api, and docz-site all
  use one definition.** *(recommendation)*
- b. In the CLI only, behind a `docz graph` command.
- c. In docz-api only, computed at ingest.
- d. Other.

> **Resolved 2026-10-15: (a).** One public reader, scoped to the enabled
> types' directories (Observation 2's archive trap).

### 3. What does the CLI print?

- **a. `docz graph [id] --format json|mermaid|dot`, with `--depth` and
  `--direction up|down` to scope it around one document.**
  *(recommendation)*
- b. JSON only; rendering is someone else's job.
- c. Other.

> **Resolved 2026-10-15: (a).** JSON, Mermaid, and DOT, scoped by depth
> and direction. The JSON shape is shared with `docz inspect` (INV-0027).

### 4. Does the validator use it?

- **a. Yes, as warnings: a reference to an ID that does not exist, and a
  current document depending on a superseded one.** *(recommendation)*
- b. Not until the graph has been used for a while.
- c. Other.

> **Resolved 2026-10-15: (a).** Two warnings. Both are silent on this
> repository today (Observation 2), which is the point of a warning.

<!--docz:open-questions:end-->

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [INV-0027](0027-interactive-mode.md): docz-review, where the idea came
  up
- [INV-0023](0023-a-temporal-control-plane-for-docz-with-docz-api-as-coordinator.md):
  the Temporal control plane that would walk the graph
- [Design Docs Are All You Need](https://arxiv.org/abs/2609.05364):
  SMART's machine-discovered DAG
- [DESIGN-0015](../design/0015-structured-regions-and-docz-validate.md):
  regions and the validator

<!--docz:references:end-->
