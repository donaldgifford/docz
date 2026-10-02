---
id: INV-0019
title: "docz-api sync to external documentation services: Confluence and Notion"
status: Open
author: Donald Gifford
created: 2026-10-02
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0019: docz-api sync to external documentation services: Confluence and Notion

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: Confluence accepts two body formats, and storage is the portable one](#observation-1-confluence-accepts-two-body-formats-and-storage-is-the-portable-one)
  - [Observation 2: there is no Go generator for ADF worth building on](#observation-2-there-is-no-go-generator-for-adf-worth-building-on)
  - [Observation 3: markdown-to-Confluence in Go already exists](#observation-3-markdown-to-confluence-in-go-already-exists)
  - [Observation 4: Notion is a second target with a different shape](#observation-4-notion-is-a-second-target-with-a-different-shape)
  - [Observation 5: what docz markdown has to convert](#observation-5-what-docz-markdown-has-to-convert)
  - [Observation 6: where it lives in this repository](#observation-6-where-it-lives-in-this-repository)
  - [Observation 7: identity, idempotence, and direction](#observation-7-identity-idempotence-and-direction)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [1. What is the first target and format?](#1-what-is-the-first-target-and-format)
  - [2. How is the body rendered?](#2-how-is-the-body-rendered)
  - [3. Where does it live, and in what order?](#3-where-does-it-live-and-in-what-order)
  - [4. How does a repository opt in, and who holds the credentials?](#4-how-does-a-repository-opt-in-and-who-holds-the-credentials)
  - [5. How is a page identified across runs?](#5-how-is-a-page-identified-across-runs)
  - [6. What happens to a page edited in Confluence?](#6-what-happens-to-a-page-edited-in-confluence)
  - [7. Notion](#7-notion)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Can the documents docz-api indexes be synced, all or per repository, to an
external documentation service — Confluence first, Notion second — and what
would that take: which API and body format, which Go libraries exist, what in
docz markdown has to be converted, and where the capability belongs between
a `docz` CLI command, a `pkg/` package, and docz-api?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Yes. Confluence Cloud's v2 REST API accepts a page body as storage format
(XHTML) or as Atlassian Document Format (JSON), a maintained Go client
exists, and a mature Go tool already syncs markdown files to Confluence
pages. The work is not the HTTP calls. It is converting what docz documents
contain — GFM tables, task lists, nested ordered lists, fenced code, mermaid
blocks, GitHub-style alerts, region-marker comments, and relative links to
other documents — and mapping a repository's document tree onto a page tree
idempotently. The issue's sequence, a CLI command for testing before
auto-sync in the API, matches the repository's layering: the converter is a
`pkg/` package with bytes in and bytes out, the CLI drives it over a
checkout, and docz-api drives the same code from its store after an ingest.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** issue
[#142](https://github.com/donaldgifford/docz/issues/142).

Teams that keep their documents in git with docz may still be asked to
publish them where the rest of the organisation reads: a Confluence space or
a Notion workspace. docz-api already holds every indexed document's bytes,
its type, and the cross-repository id map, which is what a sync needs. The
issue asks whether this is possible, what it requires, whether a Go generator
exists for the Atlassian document format as a schema to convert docz types
into, and suggests starting with a CLI command.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Survey the Confluence Cloud API: which body formats it accepts, which Go
   clients wrap it, and whether a Go generator for the Atlassian Document
   Format exists.
2. Survey existing markdown-to-Confluence tools in Go for an existence proof
   and a possible dependency.
3. Survey the Notion API and its Go SDKs the same way.
4. List what docz markdown contains that a converter must handle, from the
   six templates and this repository's documents.
5. Prototype: render this repository's RUNBOOK-0001, one IMPL, and one
   DESIGN to storage format with a goldmark renderer, push them to a scratch
   Confluence space with `go-atlassian`, and score the fidelity of tables,
   task lists, code, mermaid, alerts, and cross-links.
6. Decide the converter, its home, the opt-in and mapping, page identity,
   and the conflict policy, then write a DESIGN.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| Confluence | Cloud REST API v2, `POST /wiki/api/v2/pages`; body `representation: storage` (XHTML) or `atlas_doc_format` (ADF JSON); v1 Content API deprecated |
| Go client | `github.com/ctreminiom/go-atlassian/v2`, v2.12.0 at the time of the survey |
| Markdown → Confluence | `kovetskiy/mark`, Go, Apache-2.0, about 1.1k stars, goldmark-based; last push to the original about May 2025; active forks `mrueg/mark`, `rfizzle/mark` |
| Notion | public API, 100 blocks per append request; `jomei/notionapi` v1.13.3 (2024-12), `dstotijn/go-notion` v0.11.0 (2023-02, pre-1.0); `brittonhayes/notionmd` markdown → blocks; `wiremind/markdown-to-notionapi` CLI |
| docz | `v2.0.0-beta.6`; `pkg/doczcore/docparse` is stdlib-only and extracts facts; nothing in `pkg/` renders markdown |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

The findings are a survey; the prototype in Approach step 5 is what turns
them into a verdict.

### Observation 1: Confluence accepts two body formats, and storage is the portable one

The v2 page API takes a body with `representation: storage`, an XHTML
dialect that Cloud, Data Center, and Server all accept, or
`atlas_doc_format`, which is Cloud-only. ADF has a quirk: the JSON document
is passed as a JSON-encoded *string* inside the body, so it is double
encoded on write and on read. Macros — code blocks with highlighting, the
table of contents, info and warning panels, task lists, status lozenges —
are `ac:structured-macro` elements in storage format and have no equivalent
in plain HTML, which is why tools that need them write storage format.
Storage format also accepts ordinary HTML for paragraphs, headings, lists,
tables, links, and inline code, so a converter is standard HTML plus a
handful of macros.

### Observation 2: there is no Go generator for ADF worth building on

Atlassian publishes the ADF schema as JSON Schema (`@atlaskit/adf-schema`),
and a Go code generator could in principle consume it, but the schema is
large and built from `oneOf` unions that generators handle badly.
`go-atlassian` hand-writes its ADF node structs instead and documents
building a body by marshalling them. The answer to the issue's question is
no, and it does not matter: storage format needs no schema, only a renderer.

### Observation 3: markdown-to-Confluence in Go already exists

`kovetskiy/mark` reads a markdown file, parses it with goldmark and custom
extensions, renders storage format, creates or updates the page through the
REST API, and uploads attachments; it binds a file to a page with HTML
comments at the top of the file (`<!-- Space: -->`, `<!-- Parent: -->`,
`<!-- Title: -->`) and renders mermaid to images. The original slowed in 2025
and forks carry on. It is an existence proof that the conversion is tractable
in Go, a reference for how each construct maps, and possibly a dependency if
its renderer is importable; otherwise its approach is what to copy, not its
binary to shell out to.

### Observation 4: Notion is a second target with a different shape

Notion's API appends blocks, at most 100 per request, so a long document is
chunked. Its block model has headings, paragraphs, bulleted and numbered
lists that nest through children, to-do blocks, code blocks with a language
(and it renders `mermaid` code blocks natively, which Confluence does not),
tables, and callouts. Two Go SDKs exist, both targeting the 2022-06-28 API
version and neither released in over a year; `notionmd` converts markdown to
blocks on top of one of them. Notion is a second renderer over the same
intermediate representation, not a second design.

### Observation 5: what docz markdown has to convert

From the six templates and this repository's documents: YAML frontmatter
(becomes a properties table or page metadata, not body); the
`<!--toc:start-->` block (a `toc` macro, or dropped); `<!--docz:…-->` region
markers (stripped); GFM tables (`<table>`); task lists (`ac:task-list`, or
plain checkboxes); nested ordered lists, which runbook steps depend on;
fenced code with a language (`code` macro); mermaid fences (an image
attachment for Confluence, native for Notion); `> [!NOTE]` alerts (panel
macros); and **relative links to other documents**, which must become the
target page's URL. That last one is the reason docz-api is the right driver:
its store has the id-to-document map across repositories, and the link
resolver is a callback the converter takes, not knowledge it has.

### Observation 6: where it lives in this repository

`pkg/` never imports `internal/`, and docz-api composes `pkg/`. A converter
`pkg/export/confluence` (and later `pkg/export/notion`) takes a document's
bytes and a link resolver and returns the body; it depends on a markdown
parser but on no HTTP client. The HTTP client, credentials, and page
bookkeeping sit in the callers: `cmd/docz` for a CLI that walks a checkout
through `repo.List`, and `internal/` for a post-ingest job on the existing
asynq queue. That keeps `go-atlassian` out of a library consumer's
dependency graph. A renderer is new ground for `pkg/`: `docparse` is
stdlib-only because it extracts facts, but rendering full markdown by hand
is a large job and goldmark is the standard Go parser, so this would be the
first markdown-rendering dependency under `pkg/`.

### Observation 7: identity, idempotence, and direction

docz ids are stable (`RFC-0001`), so a page title of `ID: Title` is natural,
but a title is a weak key: `mark` matches by title and a fork added page-id
binding because titles change. Confluence content properties can carry the
docz id and the content hash on the page itself, which gives a lookup that
survives renames and a skip-unchanged gate that mirrors the store's
reconcile. The sync is one-way: git is the source of truth, a Confluence edit
is overwritten on the next run, and the page should say so in a banner.
Detecting remote edits (compare the page's version or hash before writing)
is possible and worth deciding on.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes, provisionally. Confluence Cloud accepts storage format
through a maintained Go client (Observation 1), no ADF generator is needed
(Observation 2), and a Go tool already proves the conversion
(Observation 3). The open cost is fidelity on docz's constructs
(Observation 5), which the prototype measures, and the one real design
question is where the converter and its drivers live (Observation 6).
Notion follows as a second renderer (Observation 4). The investigation
concludes when the prototype has run and the questions below are resolved.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Prototype against Confluence Cloud in storage format with a goldmark
renderer in `pkg/export/confluence`, driven first by a `docz export
confluence` command over a checkout, then by a post-ingest job in docz-api
once the output is right. Key pages by a content property carrying the docz
id and hash. Treat Notion as the second renderer over the same shape.

### 1. What is the first target and format?

- a. **Confluence Cloud, storage format.** Portable to Data Center, needed
  for macros anyway, no double-encoded JSON. *(recommendation)*
- b. Confluence Cloud, ADF, hand-written node structs via `go-atlassian`.
- c. Notion first.
- d. Other.

### 2. How is the body rendered?

- a. **goldmark with a storage-format renderer of our own**, starting from
  goldmark's HTML renderer and overriding only the nodes storage format
  treats differently: code to the `code` macro, task items to a task list,
  alerts to panels, mermaid to an attachment, relative links through the
  resolver. *(recommendation)*
- b. Import `mark`'s renderer as a library, if its packages allow it.
- c. Shell out to the `mark` binary with generated header comments.
- d. Other.

### 3. Where does it live, and in what order?

- a. **`pkg/export/confluence` as a pure converter; `docz export
  confluence --space --parent` in the CLI to test over a checkout; then a
  docz-api post-ingest sync job on the asynq queue**, the sequence the
  issue proposes. *(recommendation)*
- b. docz-api only, no CLI.
- c. A separate tool outside this repository.
- d. Other.

### 4. How does a repository opt in, and who holds the credentials?

- a. **A `sync:` block in `.docz.yaml`** naming the service, space, and
  parent page per repository, dormant unless enabled, with the credentials
  in docz-api's environment like every other secret. The CLI takes the same
  block and its own token flag. *(recommendation)*
- b. Server-side configuration per repository, nothing in the repository.
- c. Other.

### 5. How is a page identified across runs?

- a. **Title `ID: Title` plus Confluence content properties carrying the
  docz id and content hash**, and the page id stored on the document row in
  docz-api; unchanged hash means no write. *(recommendation)*
- b. Title match only, as `mark` does by default.
- c. Other.

### 6. What happens to a page edited in Confluence?

- a. **Overwritten; the page carries a banner saying it is generated from
  the repository**, with a link to the source. One direction, no merge.
  *(recommendation)*
- b. Skip with a warning when the page's version moved since the last sync.
- c. Other.

### 7. Notion

- a. **After Confluence**, as `pkg/export/notion` over the same converter
  shape, using the block limit and native mermaid noted above.
  *(recommendation)*
- b. In parallel with Confluence.
- c. Not planned.
- d. Other.

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [#142](https://github.com/donaldgifford/docz/issues/142): this issue
- Confluence: [REST API v2 pages](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-page/);
  [create a page with `atlas_doc_format`](https://community.developer.atlassian.com/t/confluence-rest-api-v2-create-page-with-atlas-doc-format-representation/67565),
  the double-encoding note
- `go-atlassian`: [Confluence v2 Page docs](https://docs.go-atlassian.io/confluence-cloud/v2/page),
  [pkg.go.dev](https://pkg.go.dev/github.com/ctreminiom/go-atlassian/v2/service/confluence)
- `mark`: [kovetskiy/mark](https://github.com/kovetskiy/mark),
  [mrueg/mark](https://github.com/mrueg/mark),
  [rfizzle/mark](https://github.com/rfizzle/mark),
  [architecture overview](https://deepwiki.com/kovetskiy/mark)
- Notion: [jomei/notionapi](https://github.com/jomei/notionapi),
  [dstotijn/go-notion](https://github.com/dstotijn/go-notion),
  [brittonhayes/notionmd](https://github.com/brittonhayes/notionmd),
  [wiremind/markdown-to-notionapi](https://github.com/wiremind/markdown-to-notionapi)
- [ADR-0001](../adr/0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md)
  and [ADR-0004](../adr/0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md):
  the layering a converter has to respect
- [DESIGN-0011](../design/0011-api-config-block-index-page-and-additionaldocs-for-docz-api-and.md):
  the `api:` block, the precedent for a dormant opt-in block in `.docz.yaml`

<!--docz:references:end-->
