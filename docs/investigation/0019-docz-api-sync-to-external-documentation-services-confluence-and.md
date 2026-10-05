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
  - [Observation 8: the libraries, read rather than searched](#observation-8-the-libraries-read-rather-than-searched)
  - [Observation 9: what the corpus actually contains](#observation-9-what-the-corpus-actually-contains)
  - [Observation 10: the one hard failure class is raw HTML, not markdown](#observation-10-the-one-hard-failure-class-is-raw-html-not-markdown)
  - [Observation 11: what the prototype got wrong, for the next pass](#observation-11-what-the-prototype-got-wrong-for-the-next-pass)
  - [Observation 12: Notion's Go converters bring a second parser](#observation-12-notions-go-converters-bring-a-second-parser)
  - [Observation 13: the prototype's shape is the package's shape](#observation-13-the-prototypes-shape-is-the-packages-shape)
  - [Observation 14: a free Cloud site is enough for the push](#observation-14-a-free-cloud-site-is-enough-for-the-push)
  - [Observation 15: Jira is a link target, not a page target](#observation-15-jira-is-a-link-target-not-a-page-target)
  - [Observation 16: a DESIGN is a Jira story and its IMPLs' phases are its sub-tasks, from fields docz already parses](#observation-16-a-design-is-a-jira-story-and-its-impls-phases-are-its-sub-tasks-from-fields-docz-already-parses)
  - [Observation 17: Linear is the cheapest target, and its free plan shapes the mapping](#observation-17-linear-is-the-cheapest-target-and-its-free-plan-shapes-the-mapping)
  - [Observation 18: the free plan as the floor](#observation-18-the-free-plan-as-the-floor)
  - [Observation 19: the Confluence push, live](#observation-19-the-confluence-push-live)
  - [Observation 20: the Jira projection, live](#observation-20-the-jira-projection-live)
  - [Observation 21: the mermaid apps, chosen before their storage is known](#observation-21-the-mermaid-apps-chosen-before-their-storage-is-known)
  - [Observation 22: the Confluence app keeps the source in the macro, and the sync writes it](#observation-22-the-confluence-app-keeps-the-source-in-the-macro-and-the-sync-writes-it)
  - [Observation 23: the Atlassian Labs viewer draws the code panel the sync already writes](#observation-23-the-atlassian-labs-viewer-draws-the-code-panel-the-sync-already-writes)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [1. What is the first target and format?](#1-what-is-the-first-target-and-format)
  - [2. How is the body rendered?](#2-how-is-the-body-rendered)
  - [3. Where does it live, and in what order?](#3-where-does-it-live-and-in-what-order)
  - [4. How does a repository opt in, and who holds the credentials?](#4-how-does-a-repository-opt-in-and-who-holds-the-credentials)
  - [5. How is a page identified across runs?](#5-how-is-a-page-identified-across-runs)
  - [6. What happens to a page edited in Confluence?](#6-what-happens-to-a-page-edited-in-confluence)
  - [7. Notion](#7-notion)
  - [8. Jira](#8-jira)
  - [9. Linear](#9-linear)
  - [10. Is the free plan the floor?](#10-is-the-free-plan-the-floor)
  - [11. Mermaid in Confluence](#11-mermaid-in-confluence)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Can the documents docz-api indexes be synced, all or per repository, to an
external documentation service — Confluence first, Notion second — and what
would that take: which API and body format, which Go libraries exist, what in
docz markdown has to be converted, and where the capability belongs between
a `docz` CLI command, a `pkg/` package, and docz-api? Review on
2026-10-04 widened it: can the same mechanism project a DESIGN and the IMPLs
that implement it into work items in Jira or Linear, so a team gets docz's
types there without defining them, and does every part of this work on each
service's free plan?

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

For the work items: yes, and more cheaply than the pages, because the
projection reads fields the type packages already parse (`design.Doc`,
`impl.Doc.Implements`, `impl.Phase`, `impl.Task`) rather than rendering a
body, and both trackers accept the shapes docz has. The free plan is a floor
that shapes the mapping rather than blocks it, with Notion the exception.

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
7. Map a DESIGN and the IMPLs that implement it onto Jira's issue hierarchy
   and onto Linear's, from the fields `pkg/design` and `pkg/impl` parse, and
   find what each service's free plan allows and forbids.
8. Tabulate the free plan of all four services as the support floor.
9. Fix what the eye found on the live pages that the API could not: wide
   tables, and mermaid through the Marketplace apps the review chose.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| Confluence | Cloud REST API v2, `POST /wiki/api/v2/pages`; body `representation: storage` (XHTML) or `atlas_doc_format` (ADF JSON); v1 Content API deprecated, its content-property endpoint still current |
| Go client | `github.com/ctreminiom/go-atlassian/v2` v2.12.0 (verified 2026-10-02): 5 direct requires; `confluence/v2.New(httpClient, site)`; `Page.Create/Update/Get`; `PropertyService.Create/Get(ctx, contentID, …)` |
| Markdown parser | `github.com/yuin/goldmark` v1.8.6, **zero dependencies** (its `go.mod` is the module line and `go 1.22`) |
| Markdown → Confluence | `kovetskiy/mark`: master at 2026-03-20, pseudo-versions only (no semver tags), `go 1.25.0`, **50 requires** including `chromedp` (headless Chrome, for mermaid); `markdown.CompileMarkdown` and its per-node renderers are exported |
| Notion | public API, 100 blocks per append request; `jomei/notionapi` v1.13.3 (no direct deps), `dstotijn/go-notion` v0.11.0 (`go-cmp` only, pre-1.0); `brittonhayes/notionmd` v0.9.0 on `go-notion` **and `gomarkdown/markdown`**; `wiremind/markdown-to-notionapi` CLI |
| Prototype | `/tmp/inv0019/`, five files, 1,437 lines: `render.go` (storage format, goldmark with eleven node overrides), `adf.go` (the ADF subset), `confluence.go` (v2 pages, v1 properties, a CQL probe, `export_view` read-back), `jira.go` (story, sub-tasks, labels, properties, transitions), `main.go`; `-mode render\|confluence\|jira`. The first build (585 lines, census and XML check) ran over `docs/` in 0.45 s and was lost with `/tmp` |
| Scratch site (live) | `dgifford06.atlassian.net`, Free plan, created 2026-10-04; Confluence space `DOCZ` (id 98334), five pages under a `docz` parent; Jira project `DOCZ`, company-managed, nine issues; one unscoped API token over basic auth, kept outside the repository |
| Mermaid apps | `Mermaid Chart for Confluence` (key `mermaid-chart-app-for-confluence`, listing 1234056) and `Mermaid for Jira` (key `mermaid-chart-app-for-jira`, listing 1234810), both by Mermaid Chart Inc, both listed free, installed on the scratch site 2026-10-05; the Marketplace lists 50 cloud apps matching `mermaid`. The Confluence app is a Connect app with a `mermaid` structured macro whose `diagramCode` parameter holds the source (Observation 22, specimen in the prototype's `testdocs/`); uninstalled 2026-10-05 in favour of **Mermaid diagrams viewer** (Atlassian Labs, listing 1232887, key `com.atlassian.confluence.plugins.mermaid-diagrams-viewer`, Forge, Apache-2.0, `github.com/atlassian-labs/mermaid-diagrams-viewer`, version 2.81.0, 7,532 installs), which draws from a code block on the page (Observation 23); its production extension key is `23392b90-4271-4239-98ca-a3e96c663cbb/63d4d207-ac2f-4273-865c-0240d37f044a/static/mermaid-diagram`, written by the prototype when `MERMAID_VIEWER_EXTENSION_KEY` names it |
| docz | `v2.0.0-beta.6`; `pkg/doczcore/docparse` is stdlib-only and extracts facts; nothing in `pkg/` renders markdown |
| Scratch site | Confluence Cloud **Free** plan: 10 users, 2 GB, REST v1 and v2, API tokens (basic auth); or the Cloud Developer Bundle (`go.atlassian.com/cloud-dev`), 5 users; either is one `.atlassian.net` site with Jira pre-linked (verified 2026-10-04) |
| Jira | Cloud REST v3 remote issue links, `/rest/api/3/issue/{key}/remotelink`, in `go-atlassian` v2.12.0 as `RemoteLinkService`; the `jira` macro in storage format for the page side |
| Jira work items | v3 `Issue.Create`, `Issue.Transitions`, `Issue.Move`, `Issue.Property.Set`, `Issue.SearchADF` (JQL) in the same client; description is ADF (`CommentNodeScheme`) on v3, a wiki-markup string on v2, capped at 32,767 characters either way, of serialised JSON on v3; `parent` sets an Epic's child and a Sub-task's parent alike (Epic Link removed from the API 2025-06-13). **Free**: 10 users, company-managed projects, custom workflows, issue types, and fields; every user is an admin |
| Linear | GraphQL only, `api.linear.app/graphql`, personal API key or OAuth; 2,500 requests and 3,000,000 complexity points an hour per user on every plan; **Free**: unlimited members, 2 teams, 250 non-archived issues, 10 MB uploads, every member an admin; issue descriptions and documents are markdown; official SDK is TypeScript, no official Go client (verified 2026-10-04) |
| Corpus for the projection | 19 DESIGNs: median 39 KB, largest 99 KB, 11 over 32 KB; 21 IMPLs: median 26 KB, largest 101 KB, 7 over 32 KB; 127 phases (at most 11 in one), 1,084 checkbox lines; 20 of 21 IMPLs carry `**Implements:**` (14 a DESIGN, 6 an INV) |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

Observations 1 to 7 are the survey. Observations 8 to 13 are from running
the prototype in Approach step 5 on 2026-10-02, up to and not including the
push to a Confluence space, which needs a scratch Cloud site and an API
token this run did not have. Observations 14 and 15 were added on
2026-10-04 for two review questions: whether a free site exists for that
push, and where Jira fits. Observations 16 to 18 followed the same day,
when review widened the scope to work items in Jira and Linear with the
free plan as the floor. Observations 19 and 20 are the live runs of the
same day against the Free site, Approach step 5 and the Jira half of
step 7.

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
`<!-- Title: -->`) and renders mermaid to images. The original is still
moving (master is at 2026-03-20) and forks carry on beside it. It is an
existence proof that the conversion is tractable in Go, a reference for how
each construct maps, and possibly a dependency if its renderer is
importable; Observation 8 checks that.

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

### Observation 8: the libraries, read rather than searched

`go-atlassian` v2.12.0 has five direct requires. Its v2 page surface is
`Page.Create(ctx, *PageCreatePayloadScheme{SpaceID, Status, Title,
ParentID, Body *PageBodyRepresentationScheme{Representation, Value}})`,
`Page.Update(ctx, pageID, *PageUpdatePayloadScheme)`, and
`Page.Get(ctx, pageID, format, draft, version)`; authentication is basic
(email and API token) or bearer. Content properties are on the v1
`PropertyService` (`Create`, `Get`, `Gets` by content id), and a page keeps
one id across v1 and v2, so Observation 7's docz-id and hash properties are
one call each. `goldmark` v1.8.6 has no dependencies at all.

`mark` is importable: `markdown.CompileMarkdown(markdown, stdlib, path,
cfg) (string, []Attachment, error)` and its per-node renderers
(`NewConfluenceFencedCodeBlockRenderer`, `NewConfluenceBlockQuoteRenderer`,
`GHAlertsBlockQuoteClassifier`, …) are exported from top-level packages
with no `internal/`. But its `go.mod` lists 50 modules, among them
`chromedp` for rendering mermaid in a headless Chrome, it has no semver tags
(pseudo-versions only), and it finds pages by space and title through a CQL
search. That is the wrong weight for a `pkg/` package a library consumer
would import, and the title keying is what Observation 7 argues against. Its
renderer is about 1.5k lines, which also sizes the work of writing one.

### Observation 9: what the corpus actually contains

The prototype parsed every markdown file under `docs/`, archive included:
134 documents, 3.2 MB in, 4.1 MB of storage format out.

| Construct | Count | Converter's job |
| --------- | ----- | --------------- |
| Headings | 3,180 (h2 1,135, h3 1,381, h4 510, h5 19, h6 1) | `<hN>` as is |
| Tables | 323 | `<table>` as is |
| Task lists | 258, with 1,810 items | `ac:task-list` / `ac:task` |
| Ordered lists | 222, of which 3 nested under another ordered list (the runbooks) | `<ol>` as is |
| Bullet lists | 2,195 | `<ul>` as is |
| Fenced code | 438 in 24 languages: go 117, mermaid 56, text 44, yaml 41, sh/bash 57, markdown 24, ts 17, json 15, sql 7, others under 5 | `code` macro; **mermaid is 13 % of fences** |
| Indented code | 6 | `code` macro |
| HTML comments | 2,384 blocks and 6 inline | dropped: the region markers and the ToC pair |
| Raw HTML, other | 1 block (`<details>`), 63 inline | see Observation 10 |
| Links | 2,945 in-page anchors, 240 external, 357 relative to another document, 18 relative to source files, 5 unresolved | resolver for the 357; a repository blob URL for the 18; anchors are Observation 11 |
| GitHub alerts | 6 (one of each kind, plus one) | panel macros |
| Plain blockquotes | 254 | `<blockquote>` as is |
| Images | 2 (one remote, one local) | `ac:image` |
| Footnotes, strikethrough, autolinks | 2, 3, 40 | HTML as is |

So the shape the hypothesis guessed is right, with one correction in
emphasis: tables and task lists are everywhere, alerts and images are rare,
and mermaid is common enough that its rendering decides whether a sync is
useful.

### Observation 10: the one hard failure class is raw HTML, not markdown

On the first pass 129 of 134 outputs were well-formed XML (checked with
`encoding/xml` after declaring the `ac:` and `ri:` prefixes). All five
failures were raw HTML passed through as written: `<br>` without a close in
a table (39 of them in one archived IMPL), and angle-bracket placeholders in
prose that goldmark reads as inline tags — `<status>`, `<type>`,
`<impl-id>`, `<path>`, `<old>`, `<new>`. Markdown forgives these; XML does
not. Allowing a short list (`br` rewritten self-closing, `kbd`, `sub`,
`sup`, `span`, `details`, `summary`) and escaping everything else made it
134 of 134 — and escaping is what the authors meant, since `<status>` in
prose is a placeholder, not markup. docz-site reaches the same answer from
the other side: its sanitizer strips what it does not allow.

### Observation 11: what the prototype got wrong, for the next pass

- **The ToC body survives.** Dropping the `<!--toc:…-->` comments leaves the
  list of anchor links between them as an ordinary list at the top of the
  page. The converter has to drop the span, or replace it with Confluence's
  `toc` macro, which is what `docz update` would want anyway.
- **In-page anchors are untested.** 2,945 links are `#fragment`s, most of
  them in the ToC blocks; the rest are cross-references within a document.
  Confluence does not use markdown's heading slugs, so these either become
  the `anchor` macro beside each heading or follow whatever heading anchors
  the Cloud editor generates. This needs the live push to settle.
- **Mermaid.** 56 fences. The prototype kept them as code blocks, which is
  readable but not a diagram. The choices are rendering to an attachment at
  sync time (`mark` runs headless Chrome; a remote renderer such as kroki is
  the lighter way), or leaving them as code. Notion renders `mermaid` code
  blocks itself.
- **Alert bodies keep their `[!NOTE]` marker.** The prototype opened the
  right panel macro but did not cut the marker text; `mark`'s
  `GHAlertsBlockQuoteClassifier` shows how.
- **Source-file links.** 18 relative links point at `.go`, `.ts`, `.yaml`
  files. The resolver needs a second rule mapping a repository path to its
  blob URL at the ingested commit, which docz-api has (`git_sha`).
- **Five broken links in the corpus**, found because the resolver treats a
  missing target as unresolved: `docs/index.md` still links
  `plan/README.md` (gone since ADR-0003); DESIGN-0016 links DESIGN-0015 by a
  filename that was renamed; `docs/examples/README.md` links a
  `docs/MIGRATION.md` that does not exist, twice; and an archived docz-api
  IMPL links a sibling by a path that moved in the graft. `docz validate`
  does not check link targets; a sync would, as a side effect, and so could
  the site's link graph (INV-0013).

### Observation 12: Notion's Go converters bring a second parser

`notionmd` v0.9.0 converts markdown to Notion blocks on top of `go-notion`
and `gomarkdown/markdown`, a different markdown parser from goldmark. A docz
converter that used it would parse every document twice with two grammars.
The better shape is the one the hypothesis gave: one goldmark parse, and a
Notion block mapper as a second renderer over the same AST. `jomei/notionapi`
has no direct dependencies; `go-notion` has one. Both lag the current Notion
API version, which is a risk to note in the Notion phase, not now.

### Observation 13: the prototype's shape is the package's shape

The renderer is the goldmark HTML renderer with ten overrides registered at
a higher priority: fenced and indented code, HTML block and inline HTML,
link, image, blockquote, list, list item, and task checkbox. The link
override takes a resolver callback and never knows the page tree. Frontmatter
is cut before parsing, as `document.ParseFrontmatter` would cut it. That is
about 250 lines, with no state across documents, and it converted the whole
corpus in under half a second. Everything the census needed beyond that was
the AST walk.

### Observation 14: a free Cloud site is enough for the push

Approach step 5 needs a Confluence site nobody has to pay for, and there
are two, both checked on 2026-10-04.

The **Free plan** is a product, not a trial: up to 10 users, 2 GB of
attachment storage, unlimited pages and spaces, and the REST API, v1 and v2
alike, with the same authentication as every paid plan. An API token is
minted per Atlassian account and sent as basic auth (email and token), which
is the shape `go-atlassian` takes. What Free lacks is space and page
permissions, audit logs, and any support beyond the community forums, and it
sits in the lowest rate-limit tier: the per-tenant pool for apps is 65,000
points an hour with no per-user scaling, while API-token traffic stays under
the older burst limits. None of that touches a sync that writes 134 pages
once and then only what changed. Atlassian caps an API token's life at a
year (since January 2025), so whatever holds the credential (question 4)
rotates it.

The **Cloud Developer Bundle** (`go.atlassian.com/cloud-dev`) provisions a
free development site with Confluence and every Jira product, 5 users each,
meant for building apps. It is the same `.atlassian.net` tenancy with the
same API, so it would serve too. The reports against it are that signing up
while not signed in to an Atlassian account yields a 7-day Standard trial
instead, and one 2020 report of a bundle site being downgraded to Free
mid-development. The Free plan is the better scratch site here: it is the
plan a small team would actually run the sync against, so its rate-limit
tier and its missing permissions are the real conditions rather than a
developer-program approximation.

Either way one site carries Jira and Confluence already linked (the
"System Jira" application link), which is what Observation 15 needs.

Notion has the same answer with one trap. The API is on every plan at 180
requests a minute per connection on Free (600 on Business and Enterprise),
and a first sync of this corpus at 100 blocks per append is a few hundred
requests, so a few minutes. But from 2026-09-08 a Free workspace with two or
more members is capped at 1,000 lifetime blocks through the API: the write
that crosses the line fails `403 restricted_resource`, and deleting blocks
does not restore capacity. 134 documents are tens of thousands of blocks. A
single-member Free workspace has no cap and is the scratch target for
question 7; a team on Free Notion is not a sync target at all.

### Observation 15: Jira is a link target, not a page target

Jira has no page body to render into, so it is not a page target like
Confluence and Notion. It can be a link target, which this observation
covers, and a work-item target, which Observation 16 does. Linking has two
directions, both reachable with what is already in hand.

**Page to issue.** The Jira issues macro in storage format is
`<ac:structured-macro ac:name="jira">` with `key`, `serverId`, and `server`
parameters; `serverId` is the application link's UUID, one per site, read
once from any page that already carries the macro. On a single Cloud site
the link exists from the start and must not be created by hand (a second,
manual link breaks the automatic one). When the page renders the macro,
Confluence writes the back-link on the issue itself, so one macro gives both
directions. In the prototype this is a `LinkResolver` case: a link whose
target is an issue URL on the configured site becomes the macro instead of
an anchor.

**Issue to document, without Confluence.** Jira remote issue links
(`POST /rest/api/3/issue/{key}/remotelink`) attach any URL to an issue with
a title, summary, and icon, and `globalId` makes the call an upsert. The URL
can be the docz-site page for the document, so a repository that never syncs
to Confluence can still have `IMPL-0018` appear on `PROJ-12`. `go-atlassian`
v2.12.0 already ships `RemoteLinkService` (`Gets`, `Get`, `Create`,
`Update`, `DeleteByID`, `DeleteByGlobalID` in
`jira/internal/remote_link_impl.go`), so Jira adds no dependency beyond the
one Confluence brings.

**The trap is the grammar.** A Jira issue key is a project key and a
number, `PROJ-12`, and every docz id matches it: `RFC-0001`, `IMPL-0018`,
`RUNBOOK-0001`, `INV-0019`. Auto-linking by pattern over document text,
which is how Jira-Confluence integrations usually find issues, would send
every docz cross-reference to Jira, and a Jira project keyed `INV` would
collide outright. So Jira references in docz documents have to be explicit:
a set of project keys declared in the `sync:` block, below which a bare key
or an issue URL is a Jira reference and everything else is a docz id, or a
frontmatter field. docz has no structured issue field today (issues appear
in prose as `#142`), so this would be a type-neutral frontmatter addition
rather than anything in a type package, and it belongs with the work-item
question (question 8), not in the first converter.

### Observation 16: a DESIGN is a Jira story and its IMPLs' phases are its sub-tasks, from fields docz already parses

The idea from review is a projection rather than a page: a DESIGN becomes a
story, each IMPL that implements it becomes work under that story, and a
team gets docz's types in Jira without defining them there. Three things
make it cheap. The link is already parsed: `impl.Doc.Implements` reads the
template's `**Implements:**` field, and 20 of the 21 IMPLs here carry it
(14 name a DESIGN, 6 an INV). The structure is already parsed:
`impl.Phase{Token, Title, Tasks, Criteria}` and `impl.Task{ID, Text,
Checked, Verify}` are the shapes a tracker wants, and `design.Doc{Overview,
Goals, NonGoals, OpenQuestions}` is the shape of a story's description. And
the client is the one Confluence already brings: `go-atlassian`'s v3 client
has `Issue.Create`, `Issue.Transitions` and `Issue.Move`,
`Issue.Property.Set`, and `Issue.SearchADF` over JQL.

Jira's vocabulary bends the mapping, and how a team uses Jira bends it
again. The hierarchy is Epic above Story, Task, and Bug, which sit at one
level, with Sub-task below them, and a sub-task cannot have children. Read
as docz's own ladder, a DESIGN as an **Epic** with each IMPL a Story and each
phase a Sub-task uses every rung, but it makes an epic of every feature, and
that is not what an epic is where this would be used: an epic collects
features across two-week sprints, a Story is planned work, a Task is
unplanned work, and each of those has sub-tasks. Nothing in docz is an epic.
An RFC is the nearest shape, a broad proposal that spawns designs, but
`design.Doc` has no field naming its RFC and this repository has none, so
there is nothing to derive one from, and that is fine: it is the rung the
team keeps.

A DESIGN is a **feature**, and a feature is a **Story**. Its sub-tasks are
the **phases** of the IMPLs that implement it, one sub-task per phase with
the IMPL id in the summary (`IMPL-0022 Phase 3: CI readiness`), and each
task a checkbox in that sub-task's description. The IMPL itself gets no
issue: it is the set of its sub-tasks plus its objective and link in the
story's description, which avoids a hollow story with one sub-task for the
common case of one IMPL per DESIGN (14 of the 20 here that carry
`**Implements:**`). An IMPL whose `**Implements:**` names no synced DESIGN
(the 6 here that implement an INV) becomes a Story of its own with the same
sub-tasks, so no work is dropped; whether that is a Story or a Task is a
type name in the `sync:` block. The epic is the team's: the block may name
one per repository or none, and otherwise the sync **never writes a story's
`parent`**, so an epic assigned in Jira survives every run. The general
rule is that the sync owns the fields it derives (summary, description,
labels, issue type, a sub-task's parent, and the transitions it is
configured for) and no others; sprint, assignee, priority, and the story's
epic stay the team's. Both parent relations are the one `parent` field,
since the Epic Link field left the REST API on 2025-06-13. Nothing here is
custom: Story, Task, and Sub-task are default types, and Jira Free allows
company-managed projects, custom workflows, issue types, and fields in any
case, because every Free user is a Jira admin.

The description is the constraint. Jira caps a description at 32,767
characters, and on the v3 API that is the length of the serialised ADF JSON,
not of the visible text, with `CONTENT_LIMIT_EXCEEDED` on breach and a side
effect that burns an issue number. 11 of the 19 DESIGNs here exceed that as
raw markdown (median 39 KB, largest 99 KB), and 7 of the 21 IMPLs do,
before JSON roughly doubles them. So the full body never goes into Jira: the
story carries the Overview, the Goals and Non-Goals, the open questions, a
link to the document on docz-site or its Confluence page, and one short
section per implementing IMPL with its Objective and link; each phase's
sub-task carries the phase description, its tasks as checkboxes, and a link
to the phase heading. The parsed fields make that a selection, not a
truncation. What does go in has to be ADF: the
v2 API takes a wiki-markup string instead, but wiki markup has no
checkboxes, while ADF's `taskList` renders as real ones, so a phase's tasks
are tasks only over v3. ADF is then a third renderer output beside storage
format and Notion blocks, over the same goldmark walk, and the subset needed
(paragraph, heading, bullet and ordered lists, task list, code block, table,
link and code marks) is small; `go-atlassian`'s `CommentNodeScheme` is a
generic node tree that carries it, so Observation 2's verdict that no Go ADF
generator is worth building on costs nothing here.

Status is a mapping, not a field. docz statuses are per type and per
repository; Jira statuses are per workflow and change by transition
(`Issue.Transitions` lists what is allowed, `Issue.Move` takes one). The
`sync:` block names the transition for each docz status that should move an
issue (Implemented and Completed to Done, Abandoned and Cancelled to Done
with a Won't Do resolution, In Progress to In Progress), a phase's
sub-task moves to Done when every task is checked, and an IMPL marked
Completed closes every sub-task it owns. The direction stays one-way for
what the sync owns: git is the source, and a story closed in Jira reopens on
the next run unless the policy says otherwise. The reverse, a sub-task closed in Jira
checking the docz task through `docwrite.SetTaskStateBytes` and a commit, is
possible and is a question, not a plan.

Identity needs a label, not only a property. Issue properties can hold the
docz id and content hash (32 KB a value), but JQL cannot search a property
set over REST unless an app has declared it in a `jiraEntityProperties`
index, which an API token cannot do. A label `docz-IMPL-0018` is
exact-match searchable (`labels = docz-IMPL-0018`) on every plan with no
app, so the label finds the issue and the property carries the hash.
Observation 15's grammar collision applies with more force here: a Jira
project keyed `IMPL` or `INV` on the same site would make `IMPL-0018` a real
issue key, so the sync never puts a bare docz id where Jira resolves keys,
and the declared project keys are the only Jira references docz recognises.

The scale is small: this repository is 19 stories for its DESIGNs, 6 for
the IMPLs that implement an INV, and 127 sub-tasks, about 150 issues, well
inside the burst limits, and the Free plan has no issue cap.

### Observation 17: Linear is the cheapest target, and its free plan shapes the mapping

Linear is a work tracker with one API, GraphQL at `api.linear.app/graphql`,
authenticated by a personal API key or OAuth. Its rate limits do not vary by
plan: 2,500 requests and 3,000,000 complexity points an hour per user,
10,000 points a query. The Free plan has unlimited members, 2 teams, 250
non-archived issues (Done and Canceled count, sub-issues count, archived do
not), 10 MB uploads, and every member an admin; issues, projects, cycles,
initiatives, documents, the API, and webhooks are all on it. Sub-initiatives
are Enterprise and team initiatives Business, and neither is needed.

Two things make it the cheapest target. It is markdown-native:
`IssueCreateInput.description` and `Document.content` are markdown strings,
and the editor renders tables, task lists, `mermaid` fences as diagrams, and
`+++` collapsible sections, so there is no renderer, only the docz-specific
pass that drops markers and the ToC and rewrites links. That makes it the
first target where mermaid, 13 % of the corpus's fences, renders without a
decision (Observation 11). What is not documented is whether markdown
supplied through the API renders tables and mermaid as the editor does when
they are pasted; that is a live check. And there is nothing to depend on:
Linear's official SDK is TypeScript, the Go clients are community-generated,
and a client for the handful of mutations a sync needs is `net/http` and
`encoding/json`, so it is the only target with zero dependencies.

The hierarchy is Initiative, Project, Milestone, Issue, Sub-issue, and an
issue cannot attach to an initiative directly. The natural projection is a
DESIGN as a **Project** with a Document carrying the full body (no size cap
is documented) and each IMPL an Issue in the project. Phases could be
sub-issues or milestones and tasks a checklist, and this is where the free
plan bites: this repository alone is 21 IMPLs and 127 phases, 148 issues against
a cap of 250 shared by the whole workspace, so phases as sub-issues spend
the Free plan on one repository. Phases as headings with checklists inside
one issue per IMPL (21 issues) fit, and a team on Basic can switch to
sub-issues. The floor decides the default.

Identity is better than in Jira: every create input takes a client-supplied
`id` in UUID v4 format, so an id derived from the repository and the docz id
(a hash with the version and variant bits set) makes creation idempotent and
lookup by `issue(id:)` exact, with no label and no property. What a retried
create with an existing id returns is undocumented and is a live check.
Status maps by workflow state *type* (backlog, unstarted, started,
completed, canceled), which every team's states carry, so the mapping is
stable across teams where Jira's needs a transition per workflow. An
`Attachment` carries the link back to docz-site.

### Observation 18: the free plan as the floor

| Service | Free plan | API on Free | What the floor does to the sync |
| ------- | --------- | ----------- | ------------------------------- |
| Confluence | 10 users, 2 GB, no page permissions, lowest rate tier | v1 and v2, API token | Nothing: 134 pages once, then only changes |
| Jira | 10 users, every user an admin, custom workflows and types | v2 and v3, the same token as Confluence | Nothing: no issue cap, about 150 issues for this repository |
| Linear | unlimited members, 2 teams, 250 non-archived issues | GraphQL, the same limits as paid | One issue per IMPL; phases as checklists, not sub-issues |
| Notion | one member unlimited; two or more members 1,000 lifetime blocks over the API | yes, 180 requests a minute | Fails for a team: 134 documents are tens of thousands of blocks |

Three of the four meet the floor as they are. Notion does not for any
workspace with two members, and the limit is lifetime, so a team cannot
work around it by deleting. "Anyone can use it free" therefore holds for
Confluence, Jira, and Linear, and for Notion only as a personal workspace;
that is a reason to place Notion last and to say so in its documentation,
not to drop it.

### Observation 19: the Confluence push, live

Approach step 5 ran on 2026-10-04 against a Free Cloud site (space `DOCZ`),
with the prototype rebuilt around a push and an unscoped API token over
basic auth. Four documents went up under a `docz` parent page: RUNBOOK-0001,
IMPL-0022, DESIGN-0019, and docz-site's archived markdown specimen, chosen
because it carries every construct the corpus has (seven alerts, in-page
links, a `<details>` block), plus a five-line test page for anchors and
alerts.

What rendered, read back through the `export_view` body: every code fence is
a code panel (17 on RUNBOOK-0001, 18 on the specimen), every task list is a
Confluence task list (nine on IMPL-0022, 70 items, all complete), every
table is a table, the ToC region became the `toc` macro, and the alert kinds
became `info`, `tip`, `note`, and `warning` panels. Mermaid is a code panel
showing its source, by choice (question 11). Nothing came back as an unknown
macro, and four pages with their properties took under ten seconds.

What the push settled that the desk review could not:

- **Page identity is the title; the property is metadata.** The v2 pages
  endpoint finds a page by title immediately after creation, and the second
  run reported all four pages unchanged from the `docz` content property's
  hash with no write. CQL cannot search a property set over REST
  (`content.property[docz].id = …` is `400 invalid payload`, the app-index
  rule Observation 16 found for Jira), and CQL by title lags its index: no
  result right after creation, one a minute later. So the lookup is the v2
  endpoint by title, the property carries the id and hash, and a store that
  remembers the page id beats both (question 5).
- **Cross-page links by title are lazy.** IMPL-0022's `ri:page` link to
  DESIGN-0019 was written before that page existed and showed as a broken
  `createlink`; once DESIGN-0019 was created it resolved with no rewrite.
  Creation order does not matter.
- **In-page anchors work, with one trap.** Confluence Cloud gives a heading
  the id `<title><heading>` with spaces removed
  (`TEST-0001:Anchorsandalerts-Sectiontwo`), and a link to it has to be an
  `<a href="#…">` carrying that id: the storage-format `ac:anchor` form
  rendered as `href="#Section two"`, which matches nothing. The trap is
  docz's own title scheme. `ID: Title` puts a colon in the id,
  `href="#TEST-0001:…"` reads as a URL scheme, and the sanitizer drops the
  href silently. Percent-encoding the colon survives the sanitizer and
  decodes to the id, and every in-page link on the test page and the
  specimen now matches a heading id in the view HTML. Whether the browser
  scrolls is the one thing the API cannot show; it is the click to make on
  the test page.
- **A wide table needs two things the body does not carry.** Visual review
  on 2026-10-05 found the wide tables clamped to the content column, which
  the `export_view` counts could not see. Confluence Cloud reads page width
  from two content properties, `content-appearance-draft` and
  `content-appearance-published`, and a page created over REST has neither,
  so it renders narrow; and the editor's own serialisation of a table
  carries `data-layout` (`default`, `wide`, or `full-width`), without which
  a table is clamped to the column even on a full-width page. Both took
  over the API: the v2 properties endpoint accepted the pair
  (`POST /wiki/api/v2/pages/{id}/properties`; a value already present
  needs a PUT with a bumped version), and `<table data-layout="full-width">`
  in the storage body came back unchanged in both the stored and the
  exported view. The renderer now writes the attribute on every table and
  the push sets the pair on every page it creates. To learn which of the
  two the eye needs, the three pushed pages carry one each: RUNBOOK-0001 the
  properties alone, IMPL-0022 the attribute alone, the specimen both, and
  the test page both through the prototype. Neither page width nor table
  layout is in the storage-format documentation; the attribute is what the
  editor emits, and the properties are what the community found.
- **Raw HTML needs a block-level allow-list.** The specimen's `<details>`
  block arrives as two HTML blocks, the opening tag with its `<summary>` and
  then the closing tag, and a per-tag rule passed the second while escaping
  the first, which was malformed XML. The rule is now per block: pass it when
  every tag in it is allowed, else escape it whole.
- **goldmark splits the alert marker.** `[!NOTE]` parses as `[` and `!NOTE]`
  because the bracket opens a link candidate, so matching the first text node
  found no alerts on the first push. Joining the first line's text nodes
  finds all five kinds, and the marker is cut from the AST before either
  renderer walks it.

Sizes: 12 KB of RUNBOOK-0001 became 14 KB of storage format and 19 KB of
export HTML; 46 KB of DESIGN-0019 became 55 KB and 70 KB.

### Observation 20: the Jira projection, live

DESIGN-0019 and IMPL-0022 went into project `DOCZ` the same afternoon
through `go-atlassian`'s v3 client, as Observation 16 describes: one Story
for the DESIGN (`DOCZ-1`), eight Sub-tasks for IMPL-0022's phases (`DOCZ-2`
to `DOCZ-9`), each task an ADF `taskItem` with its state from
`impl.Task.Checked`, a `docz-<id>` label per issue, and a `docz` issue
property carrying id, hash, phase, and counts. `impl.Doc.Implements` named
DESIGN-0019, so the link was read, not configured. The second run found every
issue by label and updated it in place: nine issues before, nine after. The
DESIGN is Implemented and every IMPL-0022 task is checked, so the run moved
all nine to Done through `Issue.Transitions` and `Issue.Move`, and left the
story's `parent` unset, as designed.

The description budget held. The story's ADF is 16,699 bytes of the 32,767
allowed with the Overview, Goals, Non-goals, five open questions with their
resolutions, and the IMPL's objective and phase list; the largest sub-task is
13,377 bytes. It reads as a summary of the design that links to the document
for the rest, which is what a story should be.

Two things the run corrected. `go-atlassian` v2.12.0's `Issue.Search.Post`
calls `POST /rest/api/3/search`, which Cloud has removed ("The requested API
has been removed. Please migrate to the /rest/api/3/search/jql API",
CHANGE-2046); the client's `SearchJQL` method calls the new endpoint and
works, so the library is current but a consumer has to pick the right method,
and Observation 8's point about a dependency's freshness stands. And JQL
`issue.property[docz].id = "DESIGN-0019"` returned no error and no match,
the app-index rule from Observation 16 confirmed in the field, while
`labels = docz-DESIGN-0019` found `DOCZ-1` at once. A transition to a status
the issue already holds is accepted and costs a call, so a real sync reads
the status before moving.

### Observation 21: the mermaid apps, chosen before their storage is known

Visual review of the live pages chose question 11's Marketplace option over
the code panel and installed the two official Mermaid Chart apps on the
scratch site (Environment). What the listings and the API say before a
diagram exists:

- **Both are listed free**, so they sit inside the floor question 10 set;
  the paid tiers are Mermaid Chart's own account (Plus, Pro), which the
  Confluence app does not require: its built-in editor works without an
  account and an account adds sync with mermaidchart.com. The Jira app's
  listing says the opposite, that it needs a Mermaid Chart account and
  keeps diagrams in the vendor's central storage, and that it moved to
  Forge in its 2.0.0 (November 2025). The Marketplace lists 50 cloud apps
  matching `mermaid`; the official vendor is the one reason to choose these
  two, since nothing else distinguishes them from the API's side yet.
- **Neither listing nor the vendor's documentation says how a diagram is
  stored**, which is the only thing a sync cares about. The docs pages for
  both apps are one paragraph and a download link. Installing them changed
  nothing the API can see: Jira has no field whose name mentions mermaid,
  DOCZ-1 carries only the `docz` property, and no page in the space uses a
  macro named `mermaid`, `mermaid-chart`, `mermaidchart`, or the app key
  (CQL `macro =`, four probes, zero results).
- **The question is whether REST can write what the app renders.** A Forge
  macro is stored as an `ac:adf-extension` whose parameters the app
  defines. If the mermaid source is one of them, the renderer emits it in
  place of the code panel and the sync owns the diagram like any other
  region. If the source lives in the app's own Forge storage or at
  mermaidchart.com under a diagram id, the API cannot put it there, the
  macro would reference a diagram nobody created, and the code panel stays
  the fallback for a synced page, with the app serving hand-made diagrams
  only. The Jira listing's wording points at the second shape for Jira; the
  Confluence app's built-in editor leaves the first possible.
- **One specimen settles it.** The way to learn the representation is to
  insert one diagram by hand with each app, on TEST-0001 and on DOCZ-1, and
  read the page's storage body and the issue's properties back. That is a
  UI action the API cannot take, so it is the one step of this
  investigation that waits on a person. Observation 22 reads the Confluence
  one back; the Jira one is still to be inserted.

### Observation 22: the Confluence app keeps the source in the macro, and the sync writes it

A diagram inserted by hand on TEST-0001 with the Confluence app's built-in
editor, no account, read back on 2026-10-05 from the page's draft over the
v2 API (`get-draft=true`; the edit was never published, so the published
body still showed the push and CQL `macro = "mermaid"` found nothing), is a
classic structured macro and not a Forge extension. The specimen is saved
beside the prototype as `testdocs/mermaid-macro-specimen.xml`:

- `ac:name="mermaid"`, schema version 1, `data-layout="default"`, plus the
  `ac:macro-id` and `ac:local-id` Confluence assigns.
- Ten parameters: `diagramType` (`mermaid`), `mcDiagramType`
  (`flowchart`), **`diagramCode` with the source verbatim**, XML-escaped
  (`&quot;`, `&gt;`, `&lt;br/&gt;`), `size` (`large`), `isEditable`
  (`true`), `theme` (`default`), an empty `caption`, `lastEdited`, and
  `updatedAt`; the ADF form adds a `_parentId`.
- A 56 KB plain-text body: the base64 of a 41 KB, 600 by 472 pixel PNG the
  editor rendered. That cached raster is the picture the review found
  poor, and it is the editor's, not the app's only way to draw.

So the source is one of the macro's own parameters, the first of the two
shapes Observation 21 laid out, and the renderer now emits the macro for
every mermaid fence: the source in `diagramCode`, no PNG body,
`mcDiagramType` guessed from the first word (`flowchart`, `class`,
`sequence`), the editor's other parameters copied. The push took it.
Confluence stored each macro as written and assigned the id, and the
`export_view` renders every one as the app's container (`ap-container`)
rather than the `unknown-macro` a missing app would give: one on TEST-0001,
three on DESIGN-0019, with no code panel left for them. Once published, CQL
`macro = "mermaid"` finds both pages.

What the container shows about the app: it is a **Connect** app
(`moduleType: dynamicContentMacros`, `addon_key:
mermaid-chart-app-for-confluence`) whose viewer is an iframe loaded from
`https://confluence.mermaidchart.com/viewer` with a JWT, and the page
hands the macro's parameters, source included, to that frame in the
browser. Rendering is the vendor's code from the vendor's host at view
time; whether the source then leaves the browser is not visible from here.
Question 11's objection to a hosted renderer thus reaches the chosen app
in a weaker form: the source sits in the page either way, the app draws it
client-side from a vendor-hosted viewer, and kroki would have sent it to a
third party at sync time. Whether a macro written without the cached PNG
draws is the one look the API cannot take.

The fourth DESIGN-0019 diagram did not land, and the renderer is not at
fault. The runbook skeleton shown earlier in that section is a
three-backtick fence whose body carries a fence of its own, a `sh` block
indented three spaces under a list item. CommonMark lets a closing fence be
indented up to three spaces and forbids it an info string, so the inner
block's opener is content but its closer closes the skeleton fence, the
author's closing fence then opens a new block, and that block swallows "The
regions nest like this:" and the mermaid fence after it into one code panel,
closed by the diagram's own closer. GitHub reads the document the same way. The fix is a four-backtick
outer fence in DESIGN-0019, not a cleverer sync; it joins the three broken
links of Observation 11 as a document fix worth its own change.

The Jira app was never read back and went with its sibling when review
moved to the viewer (Observation 23). Its listing (Observation 21)
describes the other shape, an account and the vendor's storage, and a Jira
issue description has no slot for an app's macro in any case, so the
projection's descriptions keep mermaid as a code block whatever app a site
installs.

### Observation 23: the Atlassian Labs viewer draws the code panel the sync already writes

Review then chose **Mermaid diagrams viewer** over the Mermaid Chart app
and uninstalled the latter. What follows is read from the viewer's source
(`atlassian-labs/mermaid-diagrams-viewer`, Apache-2.0), which is a better
record than any listing:

- **A Forge app with one macro and one scope.** `app/manifest.yml` declares
  a `macro` module with key `mermaid-diagram`, title "Mermaid diagram", a
  custom UI, and the single scope `read:page:confluence`; the listing adds
  that it stores and egresses nothing. The UI fetches the page's own ADF
  over the Forge bridge (`/wiki/api/v2/pages/{id}?body-format=atlas_doc_format`,
  the draft while editing) and renders with `mermaid.js` in the browser,
  so no diagram source leaves the site. That removes the vendor-hosted
  viewer Observation 22 recorded against the Mermaid Chart app.
- **It draws from a code block, chosen by order or by index.** The macro's
  only setting is `index`, a position among *all* code blocks on the page.
  Without it (the default, "Auto detect") the app walks the ADF once,
  collects every `mermaid-diagram` extension and every code block that
  looks like mermaid, meaning `attrs.language` is `mermaid` or
  `mermaid.detectType` accepts the text, and pairs the n-th macro with the
  n-th such block in document order, retrying for thirty seconds while a
  freshly inserted macro is not yet in the saved body. A macro with no
  partner shows "Can't find codeblock to render automatically".
- **So the sync's mermaid output is the code panel it writes today plus one
  macro after it.** The renderer now emits each mermaid fence as a `code`
  macro with `language` set to `mermaid` (it used to map it to `none`) and
  the viewer macro directly after it; order does the pairing, and the
  source stays readable as code on a site without the app, which is the
  fallback question 11 wants for free. Explicit `index` was rejected: it
  counts every code block on the page, so a reader pasting one block above
  a diagram would shift every index below it, while order-pairing moves
  only if the pasted block looks like mermaid. TEST-0001 and DESIGN-0019
  now carry a panel and a macro per diagram.
- **A Forge macro's storage form carries an environment id the manifest
  does not, and the editor's specimen gave it up.** The macro is an
  `ac:adf-extension` whose `extension-key` is
  `<app-id>/<environment-id>/static/mermaid-diagram`; the app id is in
  the manifest (`23392b90-4271-4239-98ca-a3e96c663cbb`) and the
  production environment id (`63d4d207-ac2f-4273-865c-0240d37f044a`) came
  from a macro inserted by hand on TEST-0001 and read out of the page's
  version history, since the published version had already lost it again
  (`/rest/api/content/{id}/version/{n}?expand=content.body.storage`; the
  specimen is `testdocs/mermaid-viewer-specimen.xml`). The editor writes
  the node twice, once as the extension and once as its own
  `ac:adf-fallback`, with `forge-environment`, an `embedded-macro-context`
  naming the page, space, account, and cloud id, and an empty
  `guest-params`. None of that is needed: the prototype emits the node
  with `extension-type`, `extension-key`, the `local-id`, `extension-id`,
  and `extension-title` parameters, `text`, `layout`, a fresh UUID for
  `local-id`, and a one-line fallback, and Confluence stored it as written
  and gave the ADF node the `parameters.localId` the viewer pairs on. The
  key is one value per site and app, so it is configuration, not code.
- **The export view does not know the macro exists.** A Forge macro leaves
  no trace in `export_view`: no container, no placeholder, no name, where
  a Connect macro had its iframe and a missing app its `unknown-macro`.
  So the API cannot watch it render, exports built from that body show the
  code panel and no diagram, and the page in the browser is the proof. CQL
  cannot find it either (`macro = "mermaid-diagram"` returns nothing),
  where the Connect macro was searchable by name.
- **One macro draws one block.** Review tried two mermaid code blocks and
  one macro on auto detect: the macro drew the first block and the second
  stayed a code panel, which is the n-th-with-n-th rule exactly. The sync
  writes one macro per fence, directly after it, so the pairing is one to
  one by construction; a reader adding a diagram by hand adds a block and a
  macro, as the app's own instructions say.
- **Uninstalling an app orphans its macros.** After the Mermaid Chart app
  was removed, every `mermaid` macro the push had written rendered as
  Confluence's `unknown-macro` placeholder with nothing of the diagram
  left, until the re-push replaced them. The viewer's failure mode is the
  opposite: remove it and the code panel still shows the source. For a
  sync that writes pages other people read, that asymmetry is a reason on
  its own.
- **Pricing on the Marketplace is per user tier, and the first tier is
  free.** Of the 50 cloud apps matching `mermaid`, 10 are listed free and
  the rest paid through Atlassian, but every paid one checked prices its
  up-to-10-users tier at $0.00, with charges beginning at 15 users ($34.50
  to $105 a month on the three sampled). On a Free-plan site, which caps
  at 10 users, a paid app costs nothing; the free floor of question 10
  therefore does not constrain the choice of app, only an enterprise's
  bill.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes. Confluence Cloud accepts storage format through a
maintained, light Go client (Observations 1 and 8), no ADF generator is
needed (Observation 2), and a 250-line goldmark renderer converted all 134
documents under `docs/` to well-formed storage format in under half a second
(Observations 9, 10, 13). The corpus is tables, task lists, and code
(Observation 9); the only construct that broke was raw HTML, and escaping
fixed it (Observation 10). The live push has run (Observation 19): the
macros render, in-page anchors work once the colon in docz's titles is
encoded, the title is the lookup and the property the metadata, and the only
renderer bugs were a per-tag HTML rule and goldmark's split of the alert
marker. The Jira projection has run too (Observation 20): one story, eight
sub-tasks, idempotent, all moved to Done, 17 KB of ADF for the largest
description. The review of those pages found the wide tables clamped, which two
API-settable things fix (Observation 19), and sent mermaid, 13 % of all
fences (Observation 11), to a Marketplace app (question 11): the Mermaid Chart app proved the
source can live in a macro the push writes (Observation 22), and review
then settled on Atlassian Labs' viewer, which draws from the code panel
the sync writes anyway, in the browser, with the source still readable
where the app is absent (Observation 23). What is left is the questions
below, which choose the shape. The Free plan carried all of it (Observation 14). Jira is a link target rather than
a page target, reachable in both directions with the client already chosen
(Observation 15).

The widened question holds too. A DESIGN is a Jira story and the phases of
the IMPLs that implement it are its sub-tasks, from fields `pkg/design` and
`pkg/impl` already parse, with a small ADF subset for the descriptions and
the epic left to the team (Observation 16). Linear would take the same
projection with no rendering at all (Observation 17), and review deferred
it: no official Go client and no experience with it here. Review dropped
Notion: its free plan fails the floor for any team (Observation 18), and a
target that cannot be tested for free is out. Jira is the second target
after Confluence pages, and the two share one client and one credential.

The investigation stays open for the questions. The three
broken links Observation 11 turned up in live documents are worth fixing on
their own.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Take the prototype's renderer into `pkg/export/confluence`, with the ToC
span, alert markers, raw-HTML allow-list, and source-file links from
Observation 11 fixed, and drive it first by a `docz export confluence`
command over a checkout, then by a post-ingest job in docz-api. Find pages
by title through the v2 endpoint, carry the docz id and hash in a content
property, and remember the page id in docz-api's store. Before any of that, fix the three broken
links in live documents. Create the scratch site on the Free plan rather
than the developer bundle. Two targets: Confluence pages (the prototype),
then Jira work items with a DESIGN as a story and its IMPLs' phases as
sub-tasks, over the client Confluence already brings. Linear is deferred
and Notion dropped (Decisions). The free plan is the floor.

### 1. What is the first target and format?

Evidence: Observation 19, four pages live in storage format.

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
- b. Import `mark`'s renderer as a library. Its packages allow it
  (Observation 8), but they bring 50 modules and headless Chrome into
  `pkg/`, with no semver tag to pin.
- c. Shell out to the `mark` binary with generated header comments.
- d. Other.

### 3. Where does it live, and in what order?

- a. **`pkg/export/confluence` as a pure converter; `docz export
  confluence --space --parent` in the CLI to test over a checkout; then a
  docz-api post-ingest sync job on the asynq queue**, the sequence the
  issue proposes. The work-item target adds `pkg/export/jira` over a
  type-aware projection that reads `design.Doc` and `impl.Doc`; export
  packages are siblings of the type packages, like `pkg/wiki`, and may
  import them where the core may not.
  *(recommendation)*
- b. docz-api only, no CLI.
- c. A separate tool outside this repository.
- d. Other.

### 4. How does a repository opt in, and who holds the credentials?

- a. **A `sync:` block in `.docz.yaml`** naming the service, space, and
  parent page per repository, dormant unless enabled, with the credentials
  in docz-api's environment like every other secret. The CLI takes the same
  block and its own token flag. An Atlassian API token lives at most a
  year, so it is a rotated secret. The live runs used an **unscoped**
  token over basic auth against the site URL; Atlassian's scoped tokens,
  the kind its token page offers first, are documented against
  `api.atlassian.com/ex/{jira|confluence}/{cloudId}` instead, so the
  design has to say which kind it takes, or take both and pick the base
  URL from the kind. *(recommendation)*
- b. Server-side configuration per repository, nothing in the repository.
- c. Other.

### 5. How is a page identified across runs?

Evidence: Observation 19. The v2 pages endpoint finds by title at once; a
content property cannot be searched without an app; CQL lags.

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
  shape, using the block limit and native mermaid noted above, and tested
  against a single-member Free workspace, since a Free workspace with more
  members is capped at 1,000 blocks through the API (Observation 14), and
  last of the four targets, because its free plan fails the floor for a
  team (Observation 18). *(recommendation)*
- b. In parallel with Confluence.
- c. Not planned.
- d. Other.

### 8. Jira

- a. **A work-item target after Confluence: a DESIGN is a Story (a
  feature), each phase of each IMPL that implements it a Sub-task named with
  the IMPL id, each task an ADF checkbox; an IMPL with no synced DESIGN is a
  Story of its own; the epic is the team's, named in the `sync:` block or
  left alone, and the sync never writes a story's `parent`.** The full body
  stays on docz-site or Confluence; statuses move by configured transitions;
  a `docz-<id>` label finds the issue and an issue property carries the
  hash; project keys are declared and nothing is inferred from `KEY-123`
  patterns (Observations 15 and 16). *(recommendation)*
- b. docz's ladder as Jira's: a DESIGN as an Epic, each IMPL a Story, each
  phase a Sub-task. Every feature becomes an epic.
- c. A DESIGN as a Story and each IMPL one Sub-task, with phases flattened
  into its description.
- d. Links only: the Jira macro on the Confluence page and remote issue
  links on the issue, no work items.
- e. Two-way as well: a Sub-task closed in Jira checks the docz task through
  docz-api and a commit.
- f. Not planned.
- g. Other.

### 9. Linear

- a. **The second target, before Jira: a DESIGN is a Project with a Document
  carrying the full body, each IMPL an Issue in it, phases as headed
  checklists on Free and sub-issues where the plan allows, ids derived from
  the docz id, status by state type, a stdlib GraphQL client**
  (Observation 17). *(recommendation)*
- b. Phases as sub-issues always, accepting the Free cap.
- c. Not planned.
- d. Other.

### 10. Is the free plan the floor?

- a. **Yes: every feature works on each service's free plan, the defaults
  are what the free plan allows, and Notion is documented as
  personal-workspace only** (Observation 18). *(recommendation)*
- b. Free for Confluence, Jira, and Linear; Notion paid only.
- c. No floor; each target assumes a paid plan where that is simpler.
- d. Other.

### 11. Mermaid in Confluence

- a. **A Marketplace app that draws from the code panel the sync writes**:
  Atlassian Labs' Mermaid diagrams viewer (Observation 23), free, Forge,
  read-only page scope, rendering in the browser, paired with the panel by
  order. The sync writes the code panel with `language: mermaid` and one
  viewer macro after it; on a site without the app the panel stands alone,
  which is the fallback for free. Review first chose the Mermaid Chart apps
  (Observation 22: the source in the macro, but a vendor-hosted viewer and
  nothing left when the app is removed) and then this. *(chosen in review)*
- b. A code panel with the source, as the live run did: Confluence Cloud
  has no native mermaid, and this is what a page gets with no app
  installed.
- c. Render to SVG at sync time with a bundled headless browser and attach
  it (the weight Observation 8 refused in `mark`).
- d. Render through a hosted service (kroki, mermaid.ink) and attach it,
  which sends diagram source to a third party an enterprise floor cannot
  assume.
- e. Other.

<!--docz:recommendation:end-->

<!--docz:decisions:start-->
## Decisions

Resolved by user review on 2026-10-04 and 2026-10-05. Questions 1 to 6
and 8 stay open; the first review reshaped question 8's recommendation (a
DESIGN is a story, not an epic), which the live Jira run then carried out.

| # | Question | Choice | Notes |
| --- | --- | --- | --- |
| 7 | Notion | (c) not planned | its free plan caps a team's workspace at 1,000 lifetime blocks over the API (Observation 18), so it cannot be tested for free; Observations 4, 12, and 14 stay as the record |
| 9 | Linear | (d) deferred | no official Go client and no experience with it here; Observation 17 stays as the record, and the projection is the one Jira uses |
| 10 | Is the free plan the floor? | (a) yes | every feature works on each service's free plan and the free plan sets the defaults; a target that cannot be tested free is out |
| 11 | Mermaid in Confluence | (a) Atlassian Labs' Mermaid diagrams viewer | chosen 2026-10-05 after the Mermaid Chart app was tried first (Observation 22) and uninstalled: the viewer draws from the code panel the sync writes, renders in the browser with no egress, and leaves the source readable without it (Observation 23); the macro's storage form came from a hand-inserted specimen's version history and the sync writes it with five attributes and a fresh local id; Mermaid for Jira is dropped with its sibling, and a Jira description keeps mermaid as a code block |

<!--docz:decisions:end-->

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
- Atlassian plans and site: [Confluence Free and Standard](https://www.atlassian.com/software/confluence/standard),
  [Confluence Cloud rate limiting](https://developer.atlassian.com/cloud/confluence/rate-limiting/),
  [Cloud Developer Bundle sign-up](https://developer.atlassian.com/cloud/confluence/getting-set-up-with-ace/)
  (`go.atlassian.com/cloud-dev`)
- Jira work items: [the Free Jira Cloud plan](https://support.atlassian.com/jira-cloud-administration/docs/what-is-the-free-jira-cloud-plan/),
  [Epic Link and Parent Link removal](https://community.atlassian.com/forums/Jira-Cloud-Admins-articles/The-fields-quot-Epic-Link-quot-and-quot-Parent-Link-quot-will-be/ba-p/2995787),
  [entity properties and JQL](https://developer.atlassian.com/cloud/jira/platform/jira-entity-properties/),
  [description limit, `CONTENT_LIMIT_EXCEEDED`](https://jira.atlassian.com/browse/JRACLOUD-78553)
- Linear: [GraphQL getting started](https://linear.app/developers/graphql),
  [rate limiting](https://linear.app/developers/rate-limiting),
  [billing and plans](https://linear.app/docs/billing-and-plans),
  [conceptual model](https://linear.app/docs/conceptual-model),
  [editor markdown](https://linear.app/docs/editor),
  [documents](https://linear.app/docs/documents)
- [`pkg/design/doc.go`](../../pkg/design/doc.go) and
  [`pkg/impl/doc.go`](../../pkg/impl/doc.go): the fields the projection reads
- Jira: [remote issue links](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-remote-links/),
  [Jira issues macro](https://confluence.atlassian.com/doc/jira-issues-macro-139380.html),
  [the System Jira application link](https://support.atlassian.com/confluence/kb/how-to-check-which-application-link-the-jira-macro-repair-should-be-mapped-after-an-import/)
- Notion plans: [request limits](https://developers.notion.com/reference/request-limits),
  [workspace block limits](https://developers.notion.com/reference/workspace-block-limits)
- [ADR-0001](../adr/0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md)
  and [ADR-0004](../adr/0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md):
  the layering a converter has to respect
- [DESIGN-0011](../design/0011-api-config-block-index-page-and-additionaldocs-for-docz-api-and.md):
  the `api:` block, the precedent for a dormant opt-in block in `.docz.yaml`

<!--docz:references:end-->
