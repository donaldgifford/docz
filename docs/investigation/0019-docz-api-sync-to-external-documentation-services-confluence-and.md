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
| Confluence | Cloud REST API v2, `POST /wiki/api/v2/pages`; body `representation: storage` (XHTML) or `atlas_doc_format` (ADF JSON); v1 Content API deprecated, its content-property endpoint still current |
| Go client | `github.com/ctreminiom/go-atlassian/v2` v2.12.0 (verified 2026-10-02): 5 direct requires; `confluence/v2.New(httpClient, site)`; `Page.Create/Update/Get`; `PropertyService.Create/Get(ctx, contentID, …)` |
| Markdown parser | `github.com/yuin/goldmark` v1.8.6, **zero dependencies** (its `go.mod` is the module line and `go 1.22`) |
| Markdown → Confluence | `kovetskiy/mark`: master at 2026-03-20, pseudo-versions only (no semver tags), `go 1.25.0`, **50 requires** including `chromedp` (headless Chrome, for mermaid); `markdown.CompileMarkdown` and its per-node renderers are exported |
| Notion | public API, 100 blocks per append request; `jomei/notionapi` v1.13.3 (no direct deps), `dstotijn/go-notion` v0.11.0 (`go-cmp` only, pre-1.0); `brittonhayes/notionmd` v0.9.0 on `go-notion` **and `gomarkdown/markdown`**; `wiremind/markdown-to-notionapi` CLI |
| Prototype | `/tmp/inv0019/main.go`, 585 lines (renderer about 250, the rest a census and an XML check); goldmark HTML renderer with ten node overrides; run over `docs/` (134 files, archive included) in 0.45 s wall |
| docz | `v2.0.0-beta.6`; `pkg/doczcore/docparse` is stdlib-only and extracts facts; nothing in `pkg/` renders markdown |
| Scratch site | Confluence Cloud **Free** plan: 10 users, 2 GB, REST v1 and v2, API tokens (basic auth); or the Cloud Developer Bundle (`go.atlassian.com/cloud-dev`), 5 users; either is one `.atlassian.net` site with Jira pre-linked (verified 2026-10-04) |
| Jira | Cloud REST v3 remote issue links, `/rest/api/3/issue/{key}/remotelink`, in `go-atlassian` v2.12.0 as `RemoteLinkService`; the `jira` macro in storage format for the page side |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

Observations 1 to 7 are the survey. Observations 8 to 13 are from running
the prototype in Approach step 5 on 2026-10-02, up to and not including the
push to a Confluence space, which needs a scratch Cloud site and an API
token this run did not have. Observations 14 and 15 were added on
2026-10-04 for two review questions: whether a free site exists for that
push, and where Jira fits.

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

Jira has no page body to render into, so it is not a third service beside
Confluence and Notion. What #142's "eventually" can mean is linking, and
there are two directions, both reachable with what is already in hand.

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
rather than anything in a type package, and it is a follow-up (question 8),
not part of the first converter.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes. Confluence Cloud accepts storage format through a
maintained, light Go client (Observations 1 and 8), no ADF generator is
needed (Observation 2), and a 250-line goldmark renderer converted all 134
documents under `docs/` to well-formed storage format in under half a second
(Observations 9, 10, 13). The corpus is tables, task lists, and code
(Observation 9); the only construct that broke was raw HTML, and escaping
fixed it (Observation 10). What is left is not feasibility: it is the live
push to a Confluence space, which settles in-page anchors and shows how the
macros render; a decision on mermaid, which is 13 % of all fences
(Observation 11); and the questions below, which choose the shape. The push
is not blocked by cost: the Free plan carries the API in full, and one free
site covers Jira as well (Observation 14). Jira is a link target rather than
a page target, reachable in both directions with the client already chosen
(Observation 15). Notion follows as a second renderer over the same parse
(Observations 4 and 12), tested against a single-member workspace
(Observation 14).

The investigation stays open for the push and the questions. The three
broken links Observation 11 turned up in live documents are worth fixing on
their own.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Take the prototype's renderer into `pkg/export/confluence`, with the ToC
span, alert markers, raw-HTML allow-list, and source-file links from
Observation 11 fixed, and drive it first by a `docz export confluence`
command over a checkout, then by a post-ingest job in docz-api once a live
push has settled anchors and mermaid. Key pages by a content property
carrying the docz id and hash. Treat Notion as the second renderer over the
same parse. Before any of that, fix the three broken links in live
documents. Create the scratch site on the Free plan rather than the
developer bundle, and keep Jira out of the first converter (question 8).

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
- b. Import `mark`'s renderer as a library. Its packages allow it
  (Observation 8), but they bring 50 modules and headless Chrome into
  `pkg/`, with no semver tag to pin.
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
  block and its own token flag. An Atlassian API token lives at most a
  year, so it is a rotated secret. *(recommendation)*
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
  shape, using the block limit and native mermaid noted above, and tested
  against a single-member Free workspace, since a Free workspace with more
  members is capped at 1,000 blocks through the API (Observation 14).
  *(recommendation)*
- b. In parallel with Confluence.
- c. Not planned.
- d. Other.

### 8. Jira

- a. **Linking only, as a follow-up after the first converter**: an
  explicit reference, either a `jira:` list in frontmatter or a declared
  project-key set in the `sync:` block, rendered as the Jira macro on the
  Confluence page and written as a remote issue link on the issue, and never
  inferred from `KEY-123` patterns in prose (Observation 15).
  *(recommendation)*
- b. Remote issue links to docz-site only, with no Confluence macro, so a
  repository that never syncs to Confluence gets the same linking.
- c. Pattern-based auto-linking with the enabled types' `id_prefix` values
  as a deny-list.
- d. Not planned.
- e. Other.

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
- Atlassian plans and site: [Confluence Free and Standard](https://www.atlassian.com/software/confluence/standard),
  [Confluence Cloud rate limiting](https://developer.atlassian.com/cloud/confluence/rate-limiting/),
  [Cloud Developer Bundle sign-up](https://developer.atlassian.com/cloud/confluence/getting-set-up-with-ace/)
  (`go.atlassian.com/cloud-dev`)
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
