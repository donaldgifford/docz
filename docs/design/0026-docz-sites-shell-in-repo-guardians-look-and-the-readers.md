---
id: DESIGN-0026
title: "docz-site's shell in repo-guardian's look, and the reader's documents as docz-html pages"
status: Draft
author: Donald Gifford
created: 2026-11-01
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0026: docz-site's shell in repo-guardian's look, and the reader's documents as docz-html pages

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [1. Two looks on one page](#1-two-looks-on-one-page)
  - [2. The shell](#2-the-shell)
  - [3. The document pane](#3-the-document-pane)
  - [4. The fragment](#4-the-fragment)
  - [5. The scoped stylesheet](#5-the-scoped-stylesheet)
  - [6. The runtime, mounted](#6-the-runtime-mounted)
  - [7. The reader's document route](#7-the-readers-document-route)
  - [8. Per-type accents](#8-per-type-accents)
  - [9. The review page](#9-the-review-page)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. How do the reader and the review page get the export's layout?](#1-how-do-the-reader-and-the-review-page-get-the-exports-layout)
  - [2. What happens to the theme selector?](#2-what-happens-to-the-theme-selector)
  - [3. Where do the metadata table and the lifecycle go?](#3-where-do-the-metadata-table-and-the-lifecycle-go)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:overview:start-->
## Overview

docz-site splits into two looks. The **shell**, which is the topbar, sidebar,
directory, search, and palette, takes repo-guardian's design system in light
and dark. The **document** is the page `docz export html` writes, with its
tabs, phase cards, callouts, expandables, and widgets, rendered by
`pkg/export/html` on the server and embedded in the reader. Each type also
gets an accent colour of its own. The review page takes the same shell, and
its document area takes the export's theme; its layout follows IMPL-0031.

<!--docz:overview:end-->

## Goals and Non-Goals

<!--docz:goals:start-->
### Goals

- The site's chrome looks like repo-guardian's v2 UI, in a light and a
  dark mode, from one set of tokens.
- A docz document in the reader looks and behaves like its exported page,
  because the same Go code renders both.
- The type colours the directory's filter already uses carry through to
  that type's pages.
- The review page shares the shell and shows documents in the export's
  theme.

<!--docz:goals:end-->

<!--docz:non-goals:start-->
### Non-Goals

- Importing a design-system package. The tokens and styles are copied in
  and owned here.
- Porting `pkg/export/html`'s layout to TypeScript for the reader.
- Changing what an export lays out. The reader shows what the export
  shows.
- The review page's tabs and components. They are IMPL-0031.
- Rendering non-document markdown (pages, the index, the changelog) with
  the export. That markdown has no regions to lay out, so it keeps the
  React pipeline, styled like the export.

<!--docz:non-goals:end-->

<!--docz:background:start-->
## Background

IMPL-0030 gave docz one theme system: `pkg/theme`'s paper, charcoal, and
classic, applied to the whole page in docz-site, docz-review, and exported
pages alike. Seeing the three side by side, the exported page reads best.
Its layout is the reason: a tab per section and a component per region
kind, which the reader's single long render has no equivalent of. The chrome
around a document, on the other hand, reads best in repo-guardian's look,
which charcoal approximates in dark only.

The export's layout is not CSS that could be lifted. DESIGN-0025 §4's
table, the outline, and the components are about 2,000 lines of Go in
`pkg/export/html`, and the phase cards, procedures, verdicts, and
consequences read typed facts from `pkg/impl`, `pkg/runbook`, `pkg/adr`,
and `pkg/investigation`. A TypeScript port would be all of that again, held
to Go by goldens. IMPL-0031 Open Question 1 weighed embedding the export's
HTML for the review page and set it aside, because review anchors are
matched against the remark parse of the source, block by block, and the
export restructures text. The reader has no anchors, so for the reader the
embedding costs nothing that matters.

<!--docz:background:end-->

<!--docz:detailed-design:start-->
## Detailed Design

### 1. Two looks on one page

Both looks use the same token names (`--color-bg-base`, `--color-fg-*`,
`--color-accent`, …), which is what lets every existing utility keep
working. They differ by scope:

- `:root` carries the **shell** tokens, light by default and dark under
  `data-mode="dark"`, following `prefers-color-scheme` when no mode is set.
- `.docz-page[data-theme="<name>"]` carries a **document** theme's tokens,
  redefining the same names for everything inside it.

`<html>` no longer carries `data-theme`; the theme is an attribute of the
document element. One mode, `docz:mode` in local storage as today, drives
both: the shell's `data-mode` and the document's, since the document's
dark selectors are scoped under `:root[data-mode="dark"]`.

### 2. The shell

`ui/src/theme/shell.css` holds repo-guardian's tokens mapped onto docz's
names: graphite surfaces, the sky accent, IBM Plex Sans and JetBrains Mono
(self-hosted with `@fontsource`), a 3px radius, square badges, uppercase
tracked labels at 11px, a 14px base, and a 40px header. A comment at the
top records where the values came from. Light and dark are both full sets.

The shell's components keep their structure and change their styling to
match: the topbar at header height on the page background, the sidebar's
groups with their labels, square type and status badges, the palette and
menus with the overlay elevation. `ui/src/theme/tokens.css`'s `@theme`
values become the shell's dark set, since Tailwind needs one set to
generate utilities from, and `TestClassicMatchesUITokens` moves to
comparing classic with its own file only.

### 3. The document pane

Anything that is a document's content sits inside one
`<div class="docz-page" data-theme="…">`: the reader's document route,
its pages, index, and changelog, and the review page's document column.
The theme is the deployment's `DOCZ_THEME` (paper by default) on the
site, with no selector (Open Question 2), and the repository's
`html.theme` on the review page.

### 4. The fragment

`pkg/export/html.RenderOptions` gains `Fragment bool`. A fragment is the
page's `<div class="wrap">` and nothing around it: the header, the tab
bar, the panels, and the footer, wrapped in
`<div class="docz-page" data-theme="…" data-doc="…">`. It carries no
`<style>`, no `<script>` except the widgets' JSON islands, and no mermaid
loader; a diagram is the same `pre.mermaid` the embed mode writes, for the
host to draw. The header's mode toggle is omitted, since the host has one.
`Page.Hash` is the fragment's.

`docz export html --fragment` writes fragments, for a host other than the
site. docz-api serves one per document (§ API).

### 5. The scoped stylesheet

`pkg/theme`'s token files and `components.css` style a whole page: `:root`,
`html.js`, `body`, `h1`, `table`. `ui/scripts/gen-page-css.ts` rewrites
them for the pane into `ui/src/theme/__generated__/page.css`:

| In `pkg/theme` | In `page.css` |
| --- | --- |
| `:root[data-theme="x"]`, `html[data-theme="x"]` | `.docz-page[data-theme="x"]` |
| `:root[data-theme="x"][data-mode="dark"]` | `:root[data-mode="dark"] .docz-page[data-theme="x"]` |
| `:root[data-theme="x"]:not([data-mode])` | `:root:not([data-mode]) .docz-page[data-theme="x"]` |
| `html.js X`, `html:not(.js) X` | `.docz-page.js X`, `.docz-page:not(.js) X` |
| `body` | `.docz-page` |
| any other selector `S` | `.docz-page S` |

The rewriter handles the two shapes the files use, flat rules and rules
inside `@media`, and fails on anything else, so a new shape in
`pkg/theme` is a build error rather than a silent leak into the shell. The
output is committed and checked like the generated theme list, and the
`ui` job's path filter already covers `pkg/theme/**`.

### 6. The runtime, mounted

`runtime.js` gains `docz.mount(root)`: tabs, copy buttons, the task
filter, in-page links, expandables, and widgets, each looked up inside
`root`, with the tab memory keyed by `root`'s `data-doc`. A standalone page
calls it on `document` as it does today; it starts itself only when
`<html>` carries `data-doc`, so importing it into the site does nothing
until the site mounts it. Mode stays on `document.documentElement`, which
is where the site keeps it too. Hash handling is the host's on the site:
`mount` takes `{ hash: false }` there, and the route passes the tab in the
URL's search params instead.

### 7. The reader's document route

The route fetches the fragment and the document together. The fragment
goes into the pane with `dangerouslySetInnerHTML`, which is sound because
`pkg/export/html` escapes text, rebuilds raw HTML from its allow-list, and
is fuzzed for scripts, handlers, and unsafe URLs (`FuzzRender`), and the
site's XSS suite gains the fragment path. After it lands the route calls
`docz.mount`, draws `pre.mermaid` with the site's own lazy mermaid, and
intercepts clicks on links the server resolved to site routes, so they
navigate in-app.

The breadcrumb, the metadata table, the lifecycle, and the html/md/json
view switch stay in the shell above the pane, as today (Open Question 3);
"html" is the fragment. A fragment that fails to load falls back
to today's React render with a quiet note, so a newer site against an
older API still shows documents.

### 8. Per-type accents

`typeColor()` already resolves a type to `--color-t-<type>` (or a hashed
fallback). A type's routes set `--type-accent` from it, and the shell
reads it for the eyebrow, the breadcrumb's last part, the active sidebar
item, and the repository mark. Inside the pane, `--color-accent` takes the
same value, so the tab bar, links, and phase bars follow the type. Each
`--color-t-*` gets a value for each shell mode and each document theme,
and `contrast.test.ts` holds each to 4.5:1 as text on the surfaces it is
drawn on.

### 9. The review page

The review page takes the shell (§2), and its document column becomes a
`.docz-page` in the repository's theme. Its layout is IMPL-0031's: the
outline from `docz inspect`, computed in Go by the code the renderer uses,
and React components over the review render, so `data-line`, highlights,
and anchors keep working.

<!--docz:detailed-design:end-->

<!--docz:api-changes:start-->
## API / Interface Changes

- **`pkg/export/html`**: `RenderOptions.Fragment`, additive.
- **`pkg/theme`**: `runtime.js` exposes `docz.mount(root, opts)` and
  starts itself only on a page whose `<html>` has `data-doc`. Standalone
  pages behave as before.
- **CLI**: `docz export html --fragment`. The JSON report is unchanged.
- **docz-api** (spec 1.8.0, additive):
  `GET /api/v1/repos/{owner}/{name}/types/{type}/docs/{doc_id}/html`
  returns `{doc_id, html, hash, inferred, problems}`. The server renders
  from the stored markdown with the type's heading spec (an embedded
  template's for a built-in, markers only for a custom type) and resolves
  relative links to the site's routes for documents and pages it holds.
  The same authorization as the document route; 404 alike.
- **ui**: the theme selector goes and `docz:theme` is no longer read;
  `DOCZ_THEME` and the chart's `site.config.theme` now name the document
  theme. The light/dark toggle stays and drives both looks.

<!--docz:api-changes:end-->

<!--docz:data-model:start-->
## Data Model

None. The fragment is rendered per request from `documents.raw_md`. A
render is a goldmark parse of one document, a few milliseconds, so there
is no cache; one could key on the content hash later.

<!--docz:data-model:end-->

<!--docz:testing:start-->
## Testing Strategy

- **Go**: fragment goldens beside the page goldens (every corpus document,
  one per theme), `FuzzRender` over fragments, and the endpoint in the
  OpenAPI contract test.
- **Generated CSS**: a check that `page.css` is current, and unit tests
  of the rewriter on each selector shape and on a shape it must refuse.
- **ui**: the XSS suite over a hostile fragment, contrast for the shell
  and the type accents in both modes, and axe over the reader with the
  pane.
- **e2e**: the reader opens a document's tabs, filters an IMPL's tasks,
  opens a question, draws a widget and a diagram, follows a resolved link
  in-app, and keeps the shell's look in both modes.

<!--docz:testing:end-->

<!--docz:rollout:start-->
## Migration / Rollout Plan

1. The fragment, `--fragment`, and `docz.mount`.
2. The endpoint.
3. The shell tokens and fonts, and the scoped stylesheet.
4. The reader's document route on the fragment, and the type accents.
5. The review page's shell and document theme.

The site falls back to its React render while the API lacks the endpoint,
so the site and the API can ship in either order.

<!--docz:rollout:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. How do the reader and the review page get the export's layout?

- a. Split by surface: the reader embeds the fragment Go renders; the
  review page builds IMPL-0031's React components over its own render,
  with the outline from Go.
- b. React everywhere: both use IMPL-0031's components.
- c. Go everywhere: the review page embeds the fragment too, stamped with
  lines, and anchors are re-verified against it.

> **Resolved 2026-11-01: (a).** The reader has nothing to anchor, so one
> renderer is pure gain there; the review page keeps its anchors.

### 2. What happens to the theme selector?

- a. It stays and picks the document theme; the shell follows the mode
  toggle only.
- b. It goes, and the document is always the deployment's `DOCZ_THEME`.

> **Resolved 2026-11-01: (b).** The shell has one look; the reader picks
> only light or dark.

### 3. Where do the metadata table and the lifecycle go?

- a. The document rail, beside the table of contents, since the export's
  header already shows id, status, author, and date.
- b. Above the pane, as today.

> **Resolved 2026-11-01: (b).** The pane repeats id, status, author, and
> date in its header; the table and lifecycle add the rest.

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0025: docz-html](0025-docz-html-standalone-html-pages-from-docz-documents-and-one.md)
  §3 (the theme), §4 (layout), §5 (the runtime)
- [DESIGN-0024: docz-review](0024-docz-review-a-blocking-review-page-for-agent-written-documents.md)
- [IMPL-0030](../impl/0030-one-theme-system-across-docz-html-docz-review-and-docz-site.md)
- [IMPL-0031](../impl/0031-the-review-page-laid-out-like-docz-htmls-exported-page.md)
- [IMPL-0032](../impl/0032-the-repo-guardian-look-shell-the-readers-document-pane.md)

<!--docz:references:end-->
