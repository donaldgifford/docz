---
id: INV-0013
title: "docz-site deferred features after the move: link graph, lifecycle, labels"
status: Open
author: Donald Gifford
created: 2026-09-23
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0013: docz-site deferred features after the move: link graph, lifecycle, labels

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Of the docz-site features docz-api INV-0003 found blocked on the API surface,
which are still blocked now that docz, docz-api, and (soon) docz-site share one
module — and does sharing a module change where each belongs, in `pkg/`, in
`internal/`, or in the site?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

The ordering INV-0003 recommended still holds, but the move collapses its
"upstream in docz" step: labels and typed relationships were blocked on a docz
release that docz-api then pinned, and in one module they are a `pkg/` change
the server picks up in the same pull request.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** docz-api INV-0003, archived at
[`docs/archive/api/investigation/0003-docz-site-deferred-features-and-the-docz-api-surface-to-unblock.md`](../archive/api/investigation/0003-docz-site-deferred-features-and-the-docz-api-surface-to-unblock.md),
which this continues (ADR-0004, DESIGN-0016 OQ 6, IMPL-0019 Phase 4).

INV-0003 sequenced five items. Its first, the repo home rendered from
`index.md`, shipped (docz-api DESIGN-0003 and IMPL-0003, archived). Still open
when it was archived:

- **The cross-doc link graph** (its F2), the keystone behind
  References/Referenced-by, relationship banners, and hover previews; it called
  for its own investigation and design, which were never opened.
- **Lifecycle commit metadata** (F3), to follow the link graph.
- **Labels** (F4) and **typed relationships** (F2 tier 2), both blocked on
  frontmatter fields docz did not have.
- Its non-goals (pdf, raw-file ingest, the MCP page) were to be recorded in the
  docz-site design so the coverage map pointed at owned dispositions.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Re-check each open item against the merged module: what `internal/` already
   stores (`raw_md`, `id_prefix`), and what `pkg/` already extracts
   (`docparse`, `kinds.References`).
2. For the link graph, decide whether edge extraction is a `pkg/` fact
   extractor (reusable by the CLI's `docz validate`) or server-only.
3. For labels and typed relationships, draft the frontmatter fields as a docz
   change rather than a feature request to another repository.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value                                                      |
| --------- | -------------------------------------------------------------------- |
| Module    | `github.com/donaldgifford/docz/v2` (docz + docz-api since IMPL-0019) |
| Site      | docz-site, not yet moved                                             |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

Not started: this successor records what was still open when docz-api's
investigation was archived.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Inconclusive — not yet investigated. This successor carries the
question forward; the verdict comes with the docz-site move.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Open the link-graph design first, as INV-0003 recommended; it gates the largest
cluster of deferred site features.

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- docz-api INV-0003 (archived):
  [`docs/archive/api/investigation/0003-docz-site-deferred-features-and-the-docz-api-surface-to-unblock.md`](../archive/api/investigation/0003-docz-site-deferred-features-and-the-docz-api-surface-to-unblock.md)
- docz-api DESIGN-0003 and IMPL-0003 (archived):
  [`docs/archive/api/design/`](../archive/api/design/),
  [`docs/archive/api/impl/`](../archive/api/impl/)
- [ADR-0004](../adr/0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md)
- [DESIGN-0016](../design/0016-move-docz-api-in-internal-cmddocz-api-api-charts-and.md)
- [INV-0012](0012-docz-site-consumes-the-openapi-contract-from-the-same-repository.md)

<!--docz:references:end-->
