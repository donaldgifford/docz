---
id: DESIGN-0001
title: "docz-site: cross-repo docz reader and search UI"
status: Draft
author: Donald Gifford
created: 2026-07-09
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0001: docz-site: cross-repo docz reader and search UI

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-07-09

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
  - [What docz-api actually provides](#what-docz-api-actually-provides)
  - [Inputs to this design](#inputs-to-this-design)
- [Detailed Design](#detailed-design)
  - [Decisions](#decisions)
  - [Tech stack](#tech-stack)
  - [Information architecture and routes](#information-architecture-and-routes)
  - [Directory view](#directory-view)
  - [Repos and repo pages](#repos-and-repo-pages)
  - [Command palette](#command-palette)
  - [Document reader pipeline](#document-reader-pipeline)
  - [Type and status color system](#type-and-status-color-system)
  - [Auth and session handling](#auth-and-session-handling)
  - [Loading, empty, error, and auth states](#loading-empty-error-and-auth-states)
  - [Deployment and dev workflow](#deployment-and-dev-workflow)
  - [Mockup coverage map](#mockup-coverage-map)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Overview

**docz-site** is the web front end for **docz-api**: one URL where a team
browses, searches, and reads every docz document across all repos onboarded to
the docz GitHub App. docz-api ingests each repo's `.docz.yaml` + `docs/` tree
into a Postgres registry, indexes it in Meilisearch, and serves an
access-scoped read + search API; docz-site is a static single-page app over
that API — it owns navigation, search UX, and a safe markdown reader, and
stores nothing.

This design replaces the exploratory draft now preserved at `docs/input.md`.
Two things ground it that the draft lacked: **docz-api is real** (its wire
contract is a hand-authored, contract-tested OpenAPI 3.1 spec at
`api/openapi.yaml`, v1.0.0), and **the visual direction is settled**
(`mockup.html` in this repo is the source of truth for look and surfaces).

## Goals and Non-Goals

### Goals

- **One URL for all docz documents**, presented as the mockup shows: a flat
  cross-repo **directory** with type chips and a repo picker, a **⌘K command
  palette** for search, a **repos grid** and TechDocs-style **repo home**
  with per-type pages, and a three-column **document view** with repo
  navigation, ToC, and metadata/lifecycle/formats rails.
- **A faithful, safe reader.** Fetch `raw_md` from docz-api, render
  client-side through a sanitizing AST pipeline (the content is user-authored
  markdown → stored-XSS surface), highlight code, build the ToC from rendered
  headings, and show status/type badges.
- **A generated, typed API client.** Vendor docz-api's `openapi.yaml` and
  generate the TanStack Query client and MSW mocks from it; the spec's SemVer
  is what the site pins against.
- **Type-agnostic with a curated core.** Render whatever types the API
  returns. Standard types (`rfc`, `adr`, `design`, `impl`, `investigation`,
  plus `mandate`, `guide`, `principle`, `policy`, `framework`) get curated
  colors; unknown custom types get deterministic fallback colors.
- **Same-origin deployment** with docz-api (the spec's declared model), so
  session-cookie auth works with zero token handling in JS and no CORS.
- **A thin vertical slice first**: the reader end-to-end (with its XSS test
  suite) before directory, palette, and auth wiring.

### Non-Goals

- **Authoring or editing documents.** Read-only; docs are created by the docz
  CLI in their source repos.
- **Storing or persisting source data.** No database; only the in-memory
  query cache and UI preferences in localStorage.
- **Talking to GitHub for content.** Content always comes from docz-api; the
  GitHub App and ingestion are docz-api's concern.
- **SSR.** SPA for now; revisit only if public/SEO docs become a requirement.
- **Per-user authorization UI.** docz-api's authorizer is currently
  all-or-nothing (any authenticated user sees every onboarded repo). The site
  treats 401/404 as first-class states and will inherit per-user scoping when
  docz-api ships it, but builds no UI around partial visibility today.
- **Features the API does not serve yet** — labels/tags, the cross-doc link
  graph (relationship banners, References/Referenced-by footer), lifecycle
  *dates* with commit/PR links, repo `index.md` / type `README.md` bodies,
  non-docz raw files (marked "not ingested" in the mockup's own words), PDF
  export, and the MCP and API marketing pages. See the
  [mockup coverage map](#mockup-coverage-map); each degrades gracefully in
  v1 and needs docz-api work when it becomes real.
- **Team docs repos** (Backstage-TechDocs-style, non-docz-format content).
  Future state, separate design.

## Background

### What docz-api actually provides

docz-api (local: `~/code/docz-api`) is a working Go/chi service: Postgres
registry, Meilisearch index, Redis sessions and job queue, GitHub App
ingestion via HMAC-verified webhooks, GitHub OAuth + OIDC (Okta, Keycloak)
login. The contract is `api/openapi.yaml` (OpenAPI 3.1, `info.version`
1.0.0), hand-authored and enforced by an in-process kin-openapi contract test
with `additionalProperties: false` on every response schema — code and spec
cannot drift. The spec is served publicly at `GET /openapi.yaml`.

The surface docz-site consumes:

| Operation    | Route                                                        | Returns |
|--------------|--------------------------------------------------------------|---------|
| `listRepos`  | `GET /api/v1/repos`                                          | `{repos: [RepoSummary]}` |
| `getRepo`    | `GET /api/v1/repos/{owner}/{name}`                           | `RepoDetail` incl. `config_snapshot`, `types` |
| `listTypes`  | `GET /api/v1/repos/{owner}/{name}/types`                     | `{types: [DocType]}` — name, dir, id_prefix, plural_label, statuses, aliases |
| `listDocs`   | `GET /api/v1/repos/{owner}/{name}/types/{type}/docs`         | `{docs: [Document]}` without `raw_md` |
| `getDoc`     | `GET .../types/{type}/docs/{doc_id}`                         | `Document` **with `raw_md`** |
| `searchDocs` | `GET /api/v1/search?q=&repo=&type=&status=&author=&offset=&limit=` | `{query, estimated_total_hits, hits, facets}` |
| `getSession` | `GET /api/v1/auth/session`                                   | `{provider, subject, email?, login?, groups?}` |
| `logout`     | `POST /api/v1/auth/logout`                                   | `{status}` |
| `login`      | `GET /auth/login?provider=` (public)                         | 302 to provider |

Contract facts the UI must respect:

- Every `/api/v1` route requires the `docz_session` httpOnly cookie; missing
  session → 401 JSON. Repos outside the caller's allowed set return **404**
  (existence hiding), so the UI needs no separate 403 treatment.
- `raw_md` is present **only** on `getDoc` and includes the frontmatter block.
- Search facets are `repo`, `type`, `status`, `author` (map of value →
  count); hits carry a `snippet` with `<em>` around matches — the snippet is
  an excerpt of user-authored body text and **must be treated as untrusted**.
- Pagination is offset/limit (default 20) with `estimated_total_hits`. The
  endpoint exposes **no sort parameter** (see the additive asks under
  API / Interface Changes).
- Timestamps are strings: `created` is `YYYY-MM-DD`, `updated_at` is RFC3339,
  `""` means unset. Arrays are `[]`, never null.
- **No CORS headers exist** — same-origin is the deployment contract
  (`servers: url: /` in the spec).
- `{type}` in paths resolves by canonical name, `id_prefix`, or alias.

### Inputs to this design

- `docs/input.md` — the original exploratory design (kept for its reasoning
  on reader safety, states, and rollout; its stack recommendations and IA are
  superseded here).
- `mockup.html` — the visual source of truth (supersedes
  `docz-site-mockup3.html`): dark IBM Plex Mono/Sans/Source Serif aesthetic,
  zero border-radius, flat directory with a repo picker, ⌘K palette with
  preview pane, a repos grid, a TechDocs-style repo home driven by
  `index.md` plus per-type pages, and a three-column document portal
  (repo nav · content · ToC/metadata/lifecycle/formats rails) with a
  references footer and in-body xref previews.
- docz-api's `api/README.md` — prescribes the vendor-and-generate consumption
  model this design adopts.
- This repo's prior contents are an `rfc-site` template (React Router v7 SSR,
  stale orval config, `charts/temp`); it is swept and replaced in Slice 0
  rather than reused.

## Detailed Design

### Decisions

1. **SPA, no SSR.** Static Vite build served same-origin with docz-api.
2. **Vendor + generate.** `api/openapi.yaml` is vendored into the repo;
   orval generates the TanStack Query hooks, types, and MSW mocks. No
   hand-written API layer.
3. **AST-based reader pipeline** (unified/remark/rehype) with
   `rehype-sanitize` — sanitization is structural, before anything renders,
   and nothing goes through `dangerouslySetInnerHTML`.
4. **Directory is a search view**: the home table is a `searchDocs` query
   with facets bound to the URL, so filtered views are shareable links and
   hit counts come free.
5. **Curated colors for standard types, deterministic fallback for custom
   ones**; status colors by case-insensitive convention with a neutral
   default.
6. **docz ToC markers are stripped** from the rendered body; the ToC rail is
   built from rendered headings, so it always matches what is displayed.
7. **Lifecycle rail v1 renders position only** — the type's `statuses` list
   with the doc's current status highlighted. Dates and commit/PR links wait
   on API support.
8. **Repo and type pages are synthesized from API data.** The repo home
   renders `index.md` when docz-api can serve it; until then (and whenever a
   repo has none) the page is generated client-side from repo metadata and
   type/doc lists — likewise the per-type pages, which mirror docz's
   generated README tables.
9. **Bun** is the package manager and script runner, pinned via mise.
10. **react-router in library mode** (`createBrowserRouter`) handles
    routing; a small typed helper wraps URL search params for facet state.
11. **Shiki** (`@shikijs/rehype`) highlights code, with a slim lazy-loaded
    grammar set.

### Tech stack

| Concern            | Choice                                                      | Notes |
|--------------------|-------------------------------------------------------------|-------|
| Build / app        | Vite + React 19 SPA, TypeScript strict                      | No server runtime |
| Routing            | react-router (library mode, `createBrowserRouter`)          | Decision 10 — no codegen, no server runtime |
| Data               | TanStack Query + orval-generated client (`httpClient: fetch`) | Generated from the vendored spec |
| Styling            | Tailwind CSS v4                                             | Mockup `:root` tokens ported into `@theme`; no component library |
| Palette            | `cmdk`                                                      | Keyboard nav / focus management for the ⌘K modal |
| Markdown           | unified: `remark-parse` + `remark-gfm` → `remark-rehype` → `rehype-raw` → **`rehype-sanitize`** → `rehype-slug` → highlighter → React | Pipeline order below |
| Highlighting       | Shiki (`@shikijs/rehype`), slim language bundle, lazy-loaded | Decision 11 |
| Unit / component   | Vitest + Testing Library + MSW (orval-generated handlers)   | XSS suite gates CI |
| E2E                | Playwright (later slice)                                    | Against MSW/fixture API |
| Tasks / toolchain  | Bun + mise + justfile (template tasks rewritten)            | Decision 9 |

### Information architecture and routes

The mockup pairs the flat cross-repo directory with a TechDocs-style
per-repo drill-down, plus an overlay palette.

```
/                                Directory: flat cross-repo doc table
                                 (?q=&repo=&type=&status=&author= — shareable)
/repos                           Repos grid: one card per onboarded repo
/:owner/:repo                    Repo home (rendered index.md, or generated)
/:owner/:repo/:type              Type page (docz README-style doc table)
/:owner/:repo/:type/:docId       Document reader (e.g. /acme/platform/design/DESIGN-0009)
/login                           Provider selection (post-MVP — Slice 5)
*                                Not found (also covers hidden/unauthorized repos)
```

- `(owner, repo, docId)` is the stable permalink; `:type` resolves by name,
  `id_prefix`, or alias exactly as the API does.
- Breadcrumbs are file-path shaped, as in the mockup —
  `repos / acme/platform / design / 0009-….md` — each segment linking to the
  level above (filenames derive from the `path` field).
- Repo home, type pages, and the reader share one left **repo nav**: repo
  identity (name, branch, `docz.yaml`), Home, each doc type with its count,
  and that type's docs nested beneath, current page highlighted.
- The ⌘K palette is an overlay available on every route, not a route itself;
  its state is ephemeral (Escape closes, selection navigates).

### Directory view

The home view is the mockup's doc table: type badge, doc id, title, status
badge, repo, updated-ago — filtered by a repo picker (dropdown with per-repo
doc counts) and type chips.

- Backed by `searchDocs` (Decision 4): chips and the picker map to `type=` /
  `repo=` params; the text box maps to `q=` (debounced ~200 ms); all bound
  to URL search params so every filtered view is shareable.
- Facet counts from the response drive the chip and picker counts ("showing
  16 of 16"; per-repo counts from the `repo` facet).
- Type chips render from the union of the `type` facet values — never a
  hardcoded list — with curated or fallback colors per the color system.
- Pagination: "load more" using offset/limit against
  `estimated_total_hits`.
- Ordering caveat: `searchDocs` has no sort parameter, and empty-`q` results
  come back in index order. Interim: client-side sort of the fetched page by
  `updated_at`. The right fix is a small additive `sort=` param in docz-api
  (minor spec bump); see the additive asks under API / Interface Changes.

### Repos and repo pages

The `/repos` grid and the TechDocs-style repo pages are the browse-by-repo
complement to the directory.

- **Repos grid** (`/repos`): one card per repo from `listRepos` — name,
  default branch, per-type doc counts, doc total, last-updated. `RepoSummary`
  carries no counts, so counts come from the search facets (`repo` facet for
  totals; a repo-filtered query's `type` facet for the per-type split),
  cached per repo.
- **Repo home** (`/:owner/:repo`): the mockup renders the `index.md` the
  repo's `.docz.yaml` points at. docz-api serves no repo-level page bodies
  yet, so v1 generates the home client-side from repo metadata and type
  sections (Decision 8) — the mockup's own "No index.md configured" fallback
  is the v1 default. When the API grows an index endpoint, the rendered
  `index.md` slots into the same frame through the reader pipeline.
- **Type pages** (`/:owner/:repo/:type`): synthesized from `listTypes` +
  `listDocs` in the shape of docz's generated README tables — plural label,
  blurb, `docz create …` hint, and the doc table (ID, title, status badge,
  date, filename from `path`). No API change needed; if docz-api later
  serves the actual type `README.md`, it renders in place of the synthesis.
- **Raw / not-ingested files**: the mockup lists non-docz files (e.g.
  `RECOMMENDATIONS.md`) under a "not ingested" nav section and itself marks
  arbitrary-file ingest as future docz/docz-api work. v1 omits the section.

### Command palette

`cmdk`-based modal, opened by ⌘K / `/`, styled per the mockup (input row,
filter pills, grouped results left / preview right, footer hints).

- Each keystroke (debounced) issues `searchDocs`; results group by repo, as
  in the mockup.
- The pills are facet shortcuts (all / per-repo / per-type) applied to the
  palette query only.
- **Snippets are untrusted.** Render by escaping the snippet text, then
  re-inserting only the `<em>` match markers (parse → escape → `<mark>`), so
  HTML that survives Meilisearch's excerpt can never execute. This is part of
  the XSS suite.
- Enter navigates to the reader; Tab focuses the preview pane, which shows
  title, status, and snippet from the hit (no extra fetch).
- "titles / body / labels" pills from the mockup: titles and body ship
  (Meilisearch already searches both); labels defer with the labels feature.

### Document reader pipeline

The reader is the thin vertical slice and the security-critical path.
`getDoc` returns metadata plus `raw_md`; everything else happens client-side
in this order:

```
1. fetch        getDoc → Document{..., raw_md}
2. pre-process  strip the YAML frontmatter block and the
                <!--toc:start-->…<!--toc:end--> marker block from raw_md
3. parse        remark-parse + remark-gfm → mdast
4. to hast      remark-rehype (allowDangerousHtml) + rehype-raw
                (inline HTML in the markdown becomes real nodes — parsed,
                 not smuggled)
5. SANITIZE     rehype-sanitize with an extended GitHub schema
                (structural allow-list on the full tree)
6. slug         rehype-slug → stable heading anchors; collect the ToC
                (h2–h4) in the same pass via a small plugin
7. highlight    @shikijs/rehype on the sanitized tree — Shiki's output is
                generated from inert text content, so highlighting after
                sanitization is safe and its spans/styles survive
8. render       hast → React elements (no dangerouslySetInnerHTML)
```

Two ordering rules are load-bearing:

- **Sanitize after `rehype-raw`, before anything else touches the tree.**
  Raw HTML embedded in markdown only becomes attackable once parsed into
  nodes; sanitizing the parsed tree removes scripts, event handlers,
  `javascript:` URLs, iframes, and hostile SVG/MathML structurally. The
  sanitize schema extends GitHub's default with the class/id attributes the
  slugger and highlighter need.
- **Highlight after sanitize.** Shiki transforms the *text content* of code
  blocks into spans it generates itself — trusted output from inert input —
  so running it post-sanitize is safe and avoids teaching the sanitizer to
  pass through highlighter markup.

Around the rendered body, the reader composes the mockup's three-column
portal from `Document` fields:

- **Left — repo nav**: the shared repo navigation (types and sibling docs
  from `listTypes`/`listDocs`), current doc highlighted.
- **Center**: file-path breadcrumb, id line (`DESIGN / 0009`), title, then
  status pill · author · updated, and the rendered body.
- **Right rail**: the "On this page" ToC (step 6, sticky on wide screens,
  a disclosure under the title on narrow ones); a trimmed **metadata card**
  (status, author, repo, omitting `""` fields, with an "all fields · json →"
  link to the document endpoint); the **lifecycle rail** — the type's
  `statuses` with the current status as the active stop (Decision 7 — no
  dates in v1); a **formats list** — "md · source" viewing the
  already-fetched `raw_md`, "json" linking the API endpoint (the mockup's
  PDF entry is deferred). The Labels card joins when labels exist in the
  API.
- **References footer** (References / Referenced by) and in-body **xref
  hover previews** need link data the API doesn't have. v1 ships a partial
  substitute: doc-id-shaped tokens in the rendered body that match a sibling
  doc become links (client-side pattern match against the repo's doc list);
  the full graph and hover cards wait on docz-api.
- Rendered output is memoized per `(doc_id, content_hash)` for the session;
  never persisted.

### Type and status color system

Both systems live in one module with the same shape: curated map first,
deterministic fallback second, neutral default last.

- **Types.** Curated tokens (from the mockup palette) for `rfc`, `adr`,
  `design`, `impl`, `investigation`/`inv`, `mandate`, `guide`, `principle`,
  `policy`, `framework`. Any other type name hashes into a fixed 8-color
  palette — stable across sessions, no configuration.
- **Statuses.** Case-insensitive convention over `.docz.yaml` status vocab:
  draft/open → amber; proposed/in review/in progress → blue;
  accepted/active/approved/adopted/completed/implemented/concluded → green;
  rejected/cancelled/abandoned → red; superseded/deprecated/archived/paused →
  purple/grey; anything unrecognized → neutral. (docz `.docz.yaml` statuses
  are capitalized, the mockup styles lowercase — hence case-insensitive.)

### Auth and session handling

docz-api owns the entire token exchange; the site never sees a token.

**Auth UX is post-MVP** (Slice 5 in the rollout). Because the cookie is
issued and honored entirely by docz-api, the MVP works against a real API
with no auth code: visit `/auth/login?provider=github` once to establish the
session; the MVP's only concession is a bare 401 panel linking there. The
flow and behaviors below describe the Slice 5 target.

```
/login          provider buttons (github default | okta | keycloak)
   │  window.location = /auth/login?provider=github
   ▼
provider        authenticates, redirects with code
   ▼
/auth/callback  docz-api verifies state, exchanges code, sets the
                docz_session httpOnly cookie, 302 → /
```

- Same-origin means fetches carry the cookie with default credentials; no
  `Authorization` headers, nothing in JS-readable storage.
- On any 401 the query layer redirects to `/login`, stashing the intended
  path in sessionStorage first — the API's callback always lands on `/`, so
  the app restores the stashed destination on first authenticated load.
- `getSession` populates the avatar menu (`login` for GitHub users, `email`
  for OIDC); `logout` clears the TanStack Query cache and returns to
  `/login`.
- 404 covers both "doesn't exist" and "not allowed to know it exists"
  (docz-api hides unauthorized repos), so the not-found state is worded
  neutrally: "Not found — or not visible to you."

### Loading, empty, error, and auth states

Every data-bound view defines four states, driven by TanStack Query status:

| State   | Trigger                        | UI |
|---------|--------------------------------|----|
| Loading | request in flight              | skeleton rows (directory) / skeleton article (reader) |
| Empty   | 200 with zero results          | contextual message + next action ("No documents yet — onboard a repo with the docz GitHub App"; "No matches — clear filters") |
| Error   | network / 5xx                  | inline error + retry; layout and URL preserved |
| Auth    | 401 → session-required panel (MVP) / `/login` redirect (Slice 5); 404 → not-found panel | destination preserved via sessionStorage once Slice 5 lands |

### Deployment and dev workflow

- **Prod:** static assets in a small nginx/caddy container, deployed behind
  the same host as docz-api — `/api/*`, `/auth/*`, `/webhooks/*`,
  `/openapi.yaml` route to docz-api; everything else falls through to
  `index.html`. The template's Dockerfile/compose/chart get rewritten for
  this shape in Slice 0 (chart work happens when a deploy target exists).
- **Dev:** `vite dev` with `server.proxy` forwarding `/api`, `/auth`, and
  `/openapi.yaml` to a local docz-api (`just dev`), or MSW mode with the
  orval-generated handlers when working UI-only (`just dev-msw`).

### Mockup coverage map

| Mockup element | v1 | Notes |
|---|---|---|
| Directory table, type chips, repo picker | ✅ | search-backed, URL-bound |
| ⌘K palette with preview pane | ✅ | snippet sanitization required |
| Repos grid | ✅ | `listRepos`; counts via search facets |
| Repo nav (types + nested docs, counts) | ✅ | `listTypes` + `listDocs` |
| Type pages (README-style doc tables) | ✅ | synthesized client-side (Decision 8) |
| Repo home from `index.md` | ◐ | generated fallback in v1; rendered `index.md` needs an API endpoint |
| Reader: body, ToC, metadata rail, status pill | ✅ | from `getDoc` + pipeline |
| Lifecycle rail | ◐ | position only; dates/commit links need API fields |
| Formats list (md · json · pdf) | ◐ | md from `raw_md`, json links the API; pdf deferred |
| In-body xref links + hover preview cards | ◐ | client-side doc-id matching in v1; previews/graph need API link data |
| References / Referenced-by footer | ⏸ | needs a cross-doc link graph in docz-api |
| Relationship banners (`instantiates →`) | ⏸ | same link-graph dependency |
| Labels (`#ai #llm`) + label search group | ⏸ | no labels in the API; future docz/docz-api feature |
| "Not ingested" raw files in repo nav | ⏸ | mockup itself marks arbitrary-file ingest as future |
| MCP page, API reference page | ⏸ | future state; docz-mcp is its own deliverable |

## API / Interface Changes

docz-site defines no public API. Its contract with docz-api is the vendored
spec:

- `api/openapi.yaml` is copied from docz-api (pinned by `info.version`,
  starting 1.0.0) into `api/` in this repo.
- `orval.config.ts` generates into `src/api/__generated__/` (gitignored):
  TanStack Query hooks per operation, model types, and MSW handlers
  (`client: react-query`, `httpClient: fetch`, `mock: true`), with a thin
  fetch mutator for base-URL and the 401-redirect behavior.
- CI runs the generator and `git diff --exit-code`s the output (drift
  check), and fails if the vendored spec's `info.version` differs from the
  version the running docz-api serves at `/openapi.yaml` (re-vendor is a
  deliberate, reviewed bump; majors are reconciled before upgrading).

The mockup also implies a short list of **additive docz-api asks**, each
with a graceful v1 degradation (none block the build): a `sort=` param on
`searchDocs`; serving repo `index.md` / type `README.md` bodies; per-status
lifecycle dates; a cross-doc link graph (references + referenced-by);
labels. Each becomes a docz-api proposal rather than silent scope creep
here.

## Data Model

The site is stateless; the generated OpenAPI types **are** the data model —
no hand-maintained view-model interfaces (the old draft's `Repo`/`DocType`/
`Document`/`SearchHit` shapes now come from codegen and cannot drift).

- **Query cache** (TanStack Query): keys come from the generated hooks,
  parameterized by route params. Sibling-doc lists and type metadata are
  cached per repo; rendered/sanitized doc bodies are memoized per
  `(doc_id, content_hash)`. The cache is cleared on logout. Nothing
  access-scoped is written to durable storage.
- **localStorage**: UI preferences only (last-used auth provider, future
  theme choice).
- `status` and `type` stay `string` everywhere — the sets are per-repo,
  driven by `.docz.yaml`; the color system, not the type system, interprets
  them.

## Testing Strategy

- **Sanitization suite (gates CI, ships with the first slice).** A table of
  XSS payloads rendered through the *real* pipeline must neutralize:
  `<script>`, `<img onerror>`, `javascript:` links (markdown and raw HTML),
  event-handler attributes, `<iframe>`/`<object>`, hostile inline SVG/MathML,
  and — specific to this design — **hostile search snippets** through the
  palette's escape-then-mark renderer. Companion tests assert benign markdown
  (GFM tables, fenced code, footnotes, images) survives, and that ToC slugs
  are stable.
- **Component (Vitest + Testing Library + MSW).** The four-state matrix per
  view; facet/chip selection ↔ URL round-tripping; color system (curated,
  fallback determinism, unknown-status neutral); lifecycle rail positioning;
  metadata table omission of `""` fields; 401 redirect with destination
  restore.
- **Contract.** Generated-client drift check plus spec-version pin
  (API / Interface Changes above) — the site's equivalent of docz-api's
  kin-openapi test.
- **E2E (Playwright, later slice).** Directory → filter → open doc; palette
  search → open doc; login redirect flow against MSW.
- **Accessibility.** axe checks in component tests; keyboard paths for
  palette, chips, and ToC; focus-visible styling (already in the mockup CSS);
  contrast checks on badge colors.

## Migration / Rollout Plan

Greenfield; "rollout" is build order, and **slices 0–4 are the MVP** — auth
UX is deliberately post-MVP. docz-api owns the whole OAuth flow and the
session cookie, so the site needs no auth code to work against a real
docz-api: establish a session once via `/auth/login?provider=github` and
every same-origin fetch carries the cookie. Each slice ships independently.

1. **Slice 0 — sweep and scaffold.** Remove the rfc-site template leftovers
   (React Router SSR config, stale orval config, `rfc-site` naming,
   `charts/temp`, dead scripts); scaffold Vite + React + TS strict +
   Tailwind v4 (mockup tokens into `@theme`); vendor `openapi.yaml`; wire
   orval codegen + drift check; rewrite justfile/mise tasks; CI (lint,
   typecheck, test, gen-api:check).
2. **Slice 1 — the reader, end-to-end.** One route hardcoded to a document,
   the full pipeline (strip → parse → sanitize → slug → highlight → render),
   metadata rail, ToC, status pill — against MSW fixtures. The XSS suite
   lands here, first.
3. **Slice 2 — directory + palette.** Search-backed table, chips and repo
   picker bound to URL params, cmdk palette with sanitized snippets.
4. **Slice 3 — repo pages.** Repos grid, the shared repo nav, generated repo
   home and type pages, reader wired into the full three-column portal.
5. **Slice 4 — polish.** Lifecycle rail, xref linking, responsive
   breakpoints, a11y hardening, code-split routes, Playwright, deploy
   container + chart.
6. **Slice 5 — auth UX (post-MVP).** `/login` provider page, session query +
   avatar/logout, 401 redirect with destination restore. Until this lands
   the site ships no login UI: dev runs against MSW, and against a real
   docz-api a 401 renders a bare "session required" panel linking to
   `/auth/login?provider=github`.

## Open Questions

None open. The four questions carried by earlier drafts were resolved
2026-07-09 in favor of their recommendations — now Decisions 9–11 (the
directory question confirmed Decision 4):

1. **Package manager / runtime** → **Bun**, pinned via mise (`bunfig.toml`
   was already present); Vite/Vitest run fine under it.
2. **Router** → **react-router in library mode** — boring, known, no codegen
   or server runtime; a small typed helper wraps URL search params for the
   facet state.
3. **Directory data source** → **`searchDocs` with empty `q`** (Decision 4
   confirmed) — one data path powers directory, palette, and every shared
   filtered view. Ordering interim is a client-side sort by `updated_at`;
   the proper fix is the additive `sort=` param proposed to docz-api (see
   API / Interface Changes; Meilisearch already declares
   `created`/`updated_at` sortable).
4. **Syntax highlighter** → **Shiki via `@shikijs/rehype`** with a slim
   grammar set (yaml, go, ts/js, bash, json, hcl, sql, python), lazy-loaded.

## References

- `docs/input.md` — the superseded exploratory design (reader-safety
  reasoning, state matrix, and rollout shape carried forward from it).
- `mockup.html` — visual source of truth (supersedes
  `docz-site-mockup3.html`).
- docz-api `api/openapi.yaml` (v1.0.0) + `api/README.md` — the wire contract
  and its prescribed vendor-and-generate consumption model.
- docz-api `docs/design/0002-openapi-contract-for-docz-api-and-the-docz-site.md`
  and `docs/impl/0002-…` — how the contract is produced and kept honest.
- orval, TanStack Query — client generation and data layer.
- unified / remark / rehype, `rehype-sanitize`, `rehype-slug`, Shiki — reader
  pipeline.
- Tailwind CSS v4, cmdk — styling and palette primitives.
- MSW, Vitest, Testing Library, Playwright — test stack.
