---
id: INV-0018
title: "docz-site: a step-aware runbook view"
status: Open
author: Donald Gifford
created: 2026-10-02
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0018: docz-site: a step-aware runbook view

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: the rail and the anchors are half built](#observation-1-the-rail-and-the-anchors-are-half-built)
  - [Observation 2: code blocks have chrome but no copy button](#observation-2-code-blocks-have-chrome-but-no-copy-button)
  - [Observation 3: the markers do not survive the pipeline](#observation-3-the-markers-do-not-survive-the-pipeline)
  - [Observation 4: there is no type-specific rendering to extend](#observation-4-there-is-no-type-specific-rendering-to-extend)
  - [Observation 5: the parser is already shipped](#observation-5-the-parser-is-already-shipped)
  - [Observation 6: two grammars or one](#observation-6-two-grammars-or-one)
  - [Observation 7: the fixture needs fixing first](#observation-7-the-fixture-needs-fixing-first)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [1. Where does the step structure come from?](#1-where-does-the-step-structure-come-from)
  - [2. How is a step anchor spelled?](#2-how-is-a-step-anchor-spelled)
  - [3. Where does the copy button go?](#3-where-does-the-copy-button-go)
  - [4. What does the rail show for a runbook?](#4-what-does-the-rail-show-for-a-runbook)
  - [5. Where does the Last Verified badge read from?](#5-where-does-the-last-verified-badge-read-from)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Can docz-site give a runbook step anchors, a copy button on each step's
commands, procedure and scenario navigation in the right rail, and a Last
Verified badge, inside its existing markdown pipeline and without a second
parser, and should it read the step structure from the markdown itself or
from the API fields INV-0017 proposes?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Yes, within the pipeline that exists. The site already parses markdown to
an mdast on the client, assigns heading ids with rehype-slug, and has a
scroll-spy ToC in the rail, so steps are one remark plugin that finds the
ordered lists under a Steps heading, assigns ids from the grammar
`pkg/runbook` uses, and feeds the rail the way `rehypeCollectToc` does. Two
things complicate it: rehype-sanitize drops HTML comments, so the
`<!--docz:…-->` markers never reach the HAST and the plugin must work at the
remark stage or from headings; and a step grammar implemented twice, in Go
and in TypeScript, can drift unless it is pinned to the same fixtures.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** issue
[#144](https://github.com/donaldgifford/docz/issues/144), from DESIGN-0019
Open Question 12 and IMPL-0022.

DESIGN-0019 gave docz-site a colour and a blurb for runbook and deferred
everything step-aware. `pkg/runbook` gives every step a stable id
(`<token>.<n>`, `S<i>.<n>`, `<token>.R<n>`), its commands, and its Expected
line, which is what the four features in the issue need. The issue says the
view depends on the structured API fields (#145, INV-0017) or on the site
parsing the markdown itself; this investigation decides which.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Read the document route, the markdown processor, the rail, and the
   code-block and heading components to see what each feature can reuse.
2. Check whether the region markers survive the pipeline, and if not, where
   a plugin would have to read them.
3. Measure the parser cost: whether a remark plugin needs a new dependency,
   and where it lands against the bundle budget.
4. Compare client-side inference with API-served step ids, including how
   each would be tested against `pkg/runbook`'s goldens.
5. Fix the runbook fixture and add reader-level and e2e coverage for a
   runbook before any feature lands.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| Route | `ui/src/app/router.tsx` `:owner/:repo/:type/:docId` → `ui/src/routes/doc.tsx`, `useGetDoc` + `useRenderedMarkdown` |
| Pipeline | `ui/src/markdown/processor.ts`: remark-parse, remark-gfm, github-alerts, capture-code-meta, remark-rehype, rehype-raw, rehype-sanitize, rehype-slug, collect-toc, mermaid marker, shiki, wrap-codeblocks, wrap-tables |
| Rail | `ui/src/components/repo-frame.tsx` grid `250px / 1fr / 190px`, rail hidden below 1181px; `TocList` + `useActiveHeading` in `doc-rail.tsx` |
| Parser deps already shipped | `unified`, `remark-parse`, `remark-gfm`, `remark-rehype`, `unist-util-visit`, `unist-util-visit-parents`, `hast-util-to-string` |
| Bundle budget | `scripts/bundle-budget.ts`, 130 KB gzip for eager JS, about 123 KB today; routes are lazy |
| Fixture | `src/mocks/content/docz-site-runbook-0001.md`, 40 lines, titled "Rotate the session secret" under path `0001-cut-a-v2-beta-release.md` |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: the rail and the anchors are half built

`rehype-slug` already gives every heading an id and `rehypeCollectToc`
collects h2 to h4 into the rail's `TocList`, with `useActiveHeading`
highlighting the one in view. The heading components (`markdown-heading.tsx`)
carry a hover "Copy link to section" button that writes to the clipboard.
So procedure and scenario navigation is a change to what the collector
emits for a runbook, and step anchors are ids on list items that the same
scroll-spy can track.

### Observation 2: code blocks have chrome but no copy button

`MarkdownPre` renders a `<pre>` with a role and label, and
`wrap-codeblock.ts` adds a header with the language and caption. The only
clipboard use in `src/` is the heading button, so a copy button on code is
new, and nothing about it is runbook-specific: every code block on the site
would benefit.

### Observation 3: the markers do not survive the pipeline

The sanitize schema never mentions comments, so rehype-sanitize's default
drops every HTML comment, and `<!--docz:procedure:start-->`,
`<!--docz:steps:start-->`, and `<!--docz:last-verified:start-->` never reach
the HAST. A step plugin that wants the markers must run at the remark stage,
where they are `html` nodes, or infer from headings the way
`kinds.InferRegions` does for an unmarked document: `### Procedure N:`,
`#### Steps`, `### Scenario:`, `## Last Verified`. For a template-shaped
runbook the two give the same regions; markers are what make a non-template
heading still parse. The processor's comment calls its order a security
invariant (raw before sanitize, shiki after), so a plugin must slot in at
remark and leave the rehype order alone.

### Observation 4: there is no type-specific rendering to extend

`doc.tsx` never branches on `doc.type`. The nearest precedent is
`LifecycleRail`, keyed by `typeName`, and `MarkdownInput`, which only gives
task-list checkboxes an accessible name. A runbook view is the first
type-aware rendering in the reader, and it should be a branch in one place,
the rendered-markdown hook or the route, rather than conditions scattered
through the plugins.

### Observation 5: the parser is already shipped

`unified`, `remark-parse`, `remark-gfm`, and the unist visitors are in
`package.json`, and `github-alerts.ts` and `capture-code-meta.ts` show the
house pattern for a remark plugin. Locating `list[ordered=true]` nodes under
a Steps heading and the nested lists under each item needs no new
dependency, and the doc route is lazy, so the plugin lands in the reader
chunk rather than the eager set the budget measures.

### Observation 6: two grammars or one

A client-side port of the step grammar is small (ordered items are steps,
nested ordered items are children, the heading's token names the
procedure), but it is a second implementation of something `pkg/runbook`
tests thoroughly. The repository has the fixture to pin them together:
`pkg/runbook/testdata/*.golden.txt` record the step ids and lines for every
fixture, and a vitest test can read those files and compare. That crosses
the boundary `CLAUDE.md` says to watch — a UI test reading outside `ui/` —
so the `ui` CI path filter would need `pkg/runbook/testdata/**`. The other
route is INV-0017's metadata carrying `steps: [{id, line}]`, with the plugin
tagging the `listItem` whose `position.start.line` matches; one grammar, but
the view then waits on the API change and on a re-ingest.

### Observation 7: the fixture needs fixing first

The mock runbook is titled "Rotate the session secret" but sits at the path
`0001-cut-a-v2-beta-release.md`; it has the Last Verified table, one
procedure, one step with a fence and an Expected line. There is no
reader-level test and no e2e spec that opens a runbook (`e2e/mvp.spec.ts`,
`rendering.spec.ts`, and `a11y.spec.ts` all use DESIGN documents). Fixing the
title and adding both tests is the first task of any implementation.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes, provisionally. Every piece has a precedent in the pipeline
(Observations 1, 2, 5) and no new parser is needed; the costs are the
marker-stripping constraint (Observation 3), the first type-aware branch in
the reader (Observation 4), and the choice between a pinned client-side
grammar and API-served ids (Observation 6). The questions below choose; the
investigation concludes when a DESIGN is written against them.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Build the view client-side on a remark plugin pinned to `pkg/runbook`'s
goldens, so it ships without waiting on INV-0017, and switch the Last
Verified badge to the API field when that lands. Make the copy button a
site-wide code-block feature. Fix the fixture and add the tests first.

### 1. Where does the step structure come from?

- a. **A remark plugin on the client, pinned to
  `pkg/runbook/testdata/*.golden.txt` in a vitest test**, with the `ui` CI
  path filter widened to that directory. Ships independently of INV-0017.
  *(recommendation)*
- b. The API serves `steps: [{id, line}]` in INV-0017's metadata and the
  plugin tags list items by source line. One grammar, but blocked on the
  API change and a re-ingest.
- c. Both: (a) now, (b) when the field exists, with the plugin preferring
  server ids.
- d. Other.

### 2. How is a step anchor spelled?

- a. **`#step-<id>` with the id verbatim**: `#step-2.3`, `#step-S1.2`,
  `#step-2.R1`. Dots are legal in an id and in a fragment, and the anchor
  reads as the id a person sees in `docz validate` output.
  *(recommendation)*
- b. Hyphenated and lower-cased, `#step-2-3`, as DESIGN-0019 Open Question
  12 spelled it.
- c. Other.

### 3. Where does the copy button go?

- a. **On every code block site-wide**, in `MarkdownPre` beside the language
  chrome, so runbooks get it along with every other document.
  *(recommendation)*
- b. On step commands only.
- c. Other.

### 4. What does the rail show for a runbook?

- a. **Procedures and scenarios, with their steps nested**, replacing the
  generic h2–h4 list for this type; the scroll-spy follows steps.
  *(recommendation)*
- b. The generic ToC with procedures and scenarios added beneath.
- c. Other.

### 5. Where does the Last Verified badge read from?

- a. **The same client-side pass**, reading the table under
  `## Last Verified`, shown in `DocHeader` beside the status pill; replaced
  by INV-0017's `runbook.last_verified` when it exists. *(recommendation)*
- b. INV-0017's field only; no badge until then.
- c. Other.

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [#144](https://github.com/donaldgifford/docz/issues/144): this issue;
  [#145](https://github.com/donaldgifford/docz/issues/145) and
  [INV-0017](0017-docz-api-serve-runbook-metadata-as-structured-fields.md):
  the API fields
- [DESIGN-0019](../design/0019-runbook-a-sixth-built-in-document-type-disabled-by-default.md)
  Open Question 12 and §4, the step grammar
- [`ui/CLAUDE.md`](../../ui/CLAUDE.md) and
  [`ui/src/markdown/processor.ts`](../../ui/src/markdown/processor.ts):
  the pipeline and its ordering invariant
- [`ui/src/components/doc-rail.tsx`](../../ui/src/components/doc-rail.tsx),
  [`ui/src/markdown/markdown-pre.tsx`](../../ui/src/markdown/markdown-pre.tsx),
  [`ui/src/markdown/markdown-heading.tsx`](../../ui/src/markdown/markdown-heading.tsx)
- [`pkg/runbook/testdata/README.md`](../../pkg/runbook/testdata/README.md):
  the goldens a client grammar would be pinned to

<!--docz:references:end-->
