---
id: DESIGN-0009
title: "docz-site: cross-repo docz viewer and search UI"
status: Draft
author: Donald Gifford
created: 2026-06-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0009: docz-site: cross-repo docz viewer and search UI

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-06-23

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [Tech stack proposal](#tech-stack-proposal)
  - [Information architecture and route map](#information-architecture-and-route-map)
  - [Search UX and how it queries docz-api](#search-ux-and-how-it-queries-docz-api)
  - [Document reader pipeline](#document-reader-pipeline)
  - [Auth and session handling against docz-api](#auth-and-session-handling-against-docz-api)
  - [Loading, empty, and error states](#loading-empty-and-error-states)
  - [Responsive layout](#responsive-layout)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. Which front-end framework?](#1-which-front-end-framework)
  - [2. Which markdown renderer library?](#2-which-markdown-renderer-library)
  - [3. SSR or SPA?](#3-ssr-or-spa)
  - [4. How does the site integrate with Meilisearch search?](#4-how-does-the-site-integrate-with-meilisearch-search)
  - [5. HTML sanitization approach?](#5-html-sanitization-approach)
  - [6. Styling / design-system choice?](#6-styling--design-system-choice)
  - [7. Where does the OIDC/OAuth token exchange happen?](#7-where-does-the-oidcoauth-token-exchange-happen)
  - [8. How should docz's own ToC markers be handled?](#8-how-should-doczs-own-toc-markers-be-handled)
- [References](#references)
<!--toc:end-->

## Overview

**docz-site** is the web front end that lets a team browse, search, and read
*all* of their docz documents from a single URL. docz is a CLI that generates
and manages standardized per-repo documentation (RFCs, ADRs, design docs,
implementation plans, plans, investigations) with typed YAML frontmatter
(`id`, `title`, `status`, `author`, `created`), stable namespaced IDs
(`RFC-0001`, `FW-0003`), and a `.docz.yaml` manifest per repo. Today each repo
has its own doc tree and there is no single place to search "all our ADRs" or
"every Draft design across the org." docz-site is that single place.

docz-site is a **client** of **docz-api** (designed separately in DESIGN-0008).
docz-api is a Go service that uses a GitHub App to onboard repos, ingests each
repo's `.docz.yaml` + doc files into a Postgres registry, indexes them in
Meilisearch, and refreshes via webhooks. Critically for this design:

- docz-api **caches raw markdown + metadata** (Decision 2). It does *not* store
  rendered HTML and does *not* render server-side.
- docz-site therefore **renders markdown → HTML client-side** with a JS
  renderer (Decision 3). docz's own terminal renderer (`mdp`) is intentionally
  single-user neovim/terminal scope and is *not* reused here.
- docz-site **stores no source data** and **never talks to GitHub directly for
  content** — it reads everything from docz-api over HTTP/JSON.

This document derives from INV-0005 ("docz-api and docz-site: centralized
cross-repo docz registry and viewer") and honors its locked decisions. It is
written to seed a brand-new `docz-site` git repository, so it restates the
essential context rather than assuming the docz or docz-api repos are open.

## Goals and Non-Goals

### Goals

- **One URL for all docz documents.** Navigate repos → a repo → its doc types →
  the documents of a type → open and read one document, with breadcrumbs
  throughout.
- **Global search** (Meilisearch-backed, via docz-api) with facets on **repo**,
  **type**, **status**, and **author**, and cross-repo views that fall out of
  those facets (e.g. "all Approved designs across repos").
- **A faithful document reader.** Fetch raw markdown from docz-api, render it to
  HTML client-side, **sanitize** the result (it is user content → XSS surface),
  highlight code, build a table of contents from headings, and show a
  frontmatter **status badge**.
- **Pluggable authentication** (Decision 6) via OIDC/OAuth: **GitHub**
  (default/preferred), **Okta**, **Keycloak**. The user sees only the repos and
  docs they are authorized for; **authorization is enforced server-side by
  docz-api**, and the site reflects it.
- **A thin vertical slice first** (Decision 8): render one repo's one type
  end-to-end before building the full navigation, search, and auth.
- **A responsive layout** — sidebar navigation + content area — that degrades to
  a single column on small screens.

### Non-Goals

- **Authoring or editing documents.** docz-site is read-only; docs are created
  and mutated by the docz CLI in their source repos.
- **Storing or persisting source data.** No database; the site is a thin client
  over docz-api. (A bounded client-side query cache is in scope; durable storage
  is not.)
- **Talking to GitHub for content.** Repo content always comes from docz-api.
  The GitHub App is docz-api's concern (DESIGN-0008); the site only uses GitHub
  as one *identity* provider.
- **Server-side markdown rendering** (rejected by Decision 3) and **reusing
  `mdp`** (its scope is single-user terminal preview).
- **Replacing per-repo MkDocs/TechDocs.** docz's per-repo `wiki` output is for
  deep single-project browsing; docz-site is the org-wide cross-repo index. They
  are complementary.
- **Implementing the authorization policy.** docz-site renders what docz-api
  returns; the policy (GitHub repo access vs. OIDC group claims) lives in
  docz-api.

## Background

The viewer half of INV-0005 settled on a shape that this design fills in. The
load-bearing conclusions:

- **The site is a thin client.** All discovery, normalization, caching, and
  search live behind docz-api. The site's job is navigation, search UX, and a
  good reader.
- **Markdown is rendered in the browser** (Decision 3). docz-api hands the site
  raw markdown plus the parsed frontmatter; the site owns the
  parse → render → sanitize → highlight → ToC pipeline. This keeps docz-api free
  of an HTML rendering stack and lets the reader evolve independently of the
  ingest service.
- **Auth is pluggable, authZ is server-enforced** (Decision 6). The site lets a
  user log in through one of several providers, but it never decides *what* a
  user may see — every content endpoint on docz-api is already access-scoped, so
  an unauthorized repo simply does not appear in the API's responses. The site
  must therefore treat 401/403 as first-class states, not edge cases.
- **Cross-repo views are free.** Because Meilisearch facets on repo/type/status/
  author, a view like "all Approved designs" is just a faceted search query — no
  special endpoint or UI mode.

What this design adds on top of INV-0005 is the concrete front-end shape: tech
stack, information architecture and routes, search UX, the reader pipeline, auth
and session handling against docz-api, the standard loading/empty/error states,
and the responsive layout.

## Detailed Design

### Tech stack proposal

docz-site is a JavaScript/TypeScript single-page application (or SSR app — see
Open Question 3). The working recommendation, to be confirmed:

| Concern             | Recommendation                          | Notes                                                            |
|---------------------|------------------------------------------|------------------------------------------------------------------|
| Framework           | React + Vite SPA (or Next.js)            | Exact choice is Open Question 1; SSR vs SPA is Open Question 3    |
| Language            | TypeScript (strict)                      | View-model interfaces below are the contract with docz-api       |
| Routing             | File- or config-based router             | Route map below                                                  |
| Data fetching/cache | TanStack Query                           | Request dedup, caching, retries, stale-while-revalidate          |
| Markdown render     | `markdown-it` **or** `react-markdown`    | Open Question 2                                                   |
| HTML sanitization   | DOMPurify                                | Mandatory; see reader pipeline (Open Question 5)                 |
| Syntax highlight    | Shiki **or** highlight.js                | Build the highlighted HTML, then sanitize                        |
| Styling             | Tailwind + a small component set         | Open Question 6                                                  |
| E2E tests           | Playwright                               | Against a mocked/fixture docz-api                                |
| Unit/component      | Vitest + Testing Library                 | Includes the XSS sanitization suite                              |

The recommendation is a **Vite + React SPA** for the thin vertical slice: no
server runtime to operate, fastest path to "render one repo's one type," and a
clean separation where docz-api is the only backend. SSR (Next.js/SvelteKit) is
attractive later for SEO-able public docs, faster first paint, and a place to
run a BFF token exchange (see Open Question 7) — hence keeping the framework
choice open rather than locking it now.

### Information architecture and route map

Navigation mirrors the docz mental model from INV-0005: **repos → a repo → its
doc types → a document**, with a global search surface alongside.

```
/                               Landing / dashboard (recent + entry points)
/repos                          List of onboarded repos the user may see
/:owner/:repo                   Repo detail: the repo's doc types
/:owner/:repo/:type             List of documents of one type in the repo
/:owner/:repo/:type/:docId      Single document reader (e.g. .../design/DESIGN-0009)
/search                         Global faceted search (repo/type/status/author)
/login                          Provider selection / OIDC entry point
/auth/callback                  OAuth/OIDC redirect handler
*                               404 / not-found
```

Notes on the route shape:

- `:owner/:repo` together identify a repo (e.g. `acme/platform`); this matches
  docz-api's `{owner}/{repo}` addressing and reads naturally in breadcrumbs.
- `:type` is the docz type *name or id_prefix* as docz-api exposes it
  (`design`, `rfc`, or a custom `frameworks`). The registry is **type-agnostic**
  — driven entirely by each repo's `.docz.yaml`, never a hardcoded list of the
  six built-ins — so the site must render whatever types the API returns.
- `:docId` is the stable frontmatter id (`DESIGN-0009`). It is unique within a
  repo, so `(owner, repo, docId)` is a stable permalink.
- **Cross-repo views are URLs into `/search`**, not new routes. "All Approved
  designs across repos" is `/search?type=design&status=Approved` with no `repo`
  facet selected. This keeps the IA small and makes every view shareable.

**Breadcrumbs** are derived from the route everywhere below `/repos`:

```
Repos / acme/platform / Designs / DESIGN-0009
```

### Search UX and how it queries docz-api

Search is the primary cross-repo surface. The `/search` page has:

- A query box (debounced, ~200 ms) bound to the URL `?q=`.
- A **facets sidebar** with four groups — repo, type, status, author — each a
  multi-select list of values with hit counts, bound to URL params
  (`?repo=…&type=…&status=…&author=…`). Selecting facets is additive and
  reflected in the URL so a filtered view is a shareable link.
- A results list of **search hits**: title, repo, type badge, status badge,
  author, and a highlighted snippet; clicking a hit routes to the document
  reader.

Every search interaction is one request to docz-api's search endpoint
(`GET /api/v1/search`, below), which proxies to Meilisearch. **Whether the site
hits a docz-api proxy endpoint or talks to Meilisearch directly with a scoped
search key is Open Question 4.** The recommendation is to proxy through docz-api
so that (a) the same access scoping that filters content endpoints also filters
search results, (b) no Meilisearch key is exposed to the browser, and (c) the
site has exactly one backend. The direct-Meilisearch alternative buys lower
latency and Meilisearch's instant-search components at the cost of issuing
per-session scoped keys and trusting the browser with filter constraints.

Because facets come straight from Meilisearch, **cross-repo views are search
queries** (see IA above): the `/search` page *is* the cross-repo view engine,
and the repo/type pages are just pre-filtered entry points into it.

### Document reader pipeline

The reader is the heart of the site and the part the thin vertical slice proves
first. docz-api returns **raw markdown + parsed metadata**; the site does
everything else, in this fixed order:

```
1. fetch          GET .../:type/:docId  → { metadata, raw_md } from docz-api
2. parse FM       split frontmatter from body (api already parsed metadata;
                  the body is the markdown below the --- block)
3. render md      markdown → HTML via the chosen JS renderer (OQ 2)
4. sanitize       HTML → safe HTML via DOMPurify (MANDATORY; see below)
5. highlight      syntax-highlight code blocks (then ensure step 4 covered it)
6. build ToC      walk rendered headings → nested ToC; add anchor ids
7. status badge   render frontmatter status as a colored badge
```

Two ordering rules are load-bearing:

- **Sanitization is non-negotiable and runs after rendering.** The content is
  user-authored markdown that becomes HTML, which is a stored-XSS surface
  (`<script>`, `onerror=`, `javascript:` URLs, malicious SVG). The rendered
  HTML is passed through DOMPurify with a conservative allow-list before it ever
  reaches the DOM. If syntax highlighting (step 5) emits raw HTML, it must be
  produced *before* the final sanitize pass, or run through a highlighter that
  only emits classed `<span>`s the sanitizer allows. The sanitization approach
  is Open Question 5; *that* it happens is not negotiable.
- **The ToC is generated from rendered headings, not from docz's own ToC
  markers.** docz documents contain `<!--toc:start-->` / `<!--toc:end-->` marker
  blocks that the CLI maintains. Rather than parse or trust those markers, the
  reader builds the ToC by walking the rendered `<h1..h4>` nodes and assigning
  slugged anchor ids, so the ToC is always consistent with what is displayed
  (how to treat the literal marker block — strip it, ignore it, or render the
  CLI-generated list — is Open Question 8).

The **status badge** maps the frontmatter `status` value to a color. The set of
valid statuses is per-type and comes from each repo's `.docz.yaml`
(`TypeConfig.Statuses`), surfaced through docz-api's metadata, so the badge uses
a small status→color convention with a neutral default for unknown values
rather than a hardcoded enum.

### Auth and session handling against docz-api

Authentication is **pluggable** (Decision 6): the site supports GitHub (default,
preferred), Okta, and Keycloak behind one OIDC/OAuth-shaped flow. The site never
makes authorization decisions — docz-api enforces them — so the front-end auth
job is narrow: get the user signed in, attach the session to API calls, and
react to 401/403.

Login flow (provider-agnostic; GitHub default):

```
/login            user picks a provider (or is sent straight to the default)
   │  redirect to provider authorize URL (coordinated with docz-api)
   ▼
provider          user authenticates (GitHub OAuth / Okta or Keycloak OIDC)
   │  redirect back with code
   ▼
/auth/callback    code is exchanged for a session
   │  (where the token exchange happens — a site BFF vs. docz-api —
   │   is Open Question 7; recommendation: docz-api owns it)
   ▼
session           session cookie (httpOnly, SameSite) issued by docz-api;
                  site holds no long-lived token in JS
```

Session handling:

- The session is carried as an **httpOnly cookie** set by docz-api (preferred)
  so no access token sits in JavaScript-readable storage. The site sends
  credentialed requests (`credentials: 'include'`) to docz-api.
- On `401`, the site routes to `/login`, preserving the intended destination as
  a return URL.
- On `403`, the site shows a "you don't have access to this" state — this is
  expected when a user follows a link to a repo/doc they cannot read.
- Because authZ is server-side, the **repos list, type list, and search results
  are already filtered** by docz-api to what the user may see. The site does not
  filter content itself; it only renders what comes back. This means an
  enumeration of repos is safe to show directly.

### Loading, empty, and error states

Every data-bound view defines four states; the data layer (TanStack Query)
drives them:

| State    | Trigger                          | UI                                                       |
|----------|----------------------------------|----------------------------------------------------------|
| Loading  | request in flight                | skeletons matching the target layout (list rows / reader) |
| Empty    | request ok, zero results         | contextual empty message + a next action                 |
| Error    | network / 5xx                    | inline error with a retry button; non-destructive        |
| Auth     | 401 → `/login`; 403 → no-access  | redirect (401) or an access-denied panel (403)           |

Examples: `/repos` with no onboarded repos shows "No repos yet — onboard one
with the docz GitHub App"; a type page with no documents shows "No designs in
acme/platform yet"; a search with zero hits shows the query and a "clear facets"
action; a reader fetch error shows a retry without losing the breadcrumb.

### Responsive layout

The default layout is a persistent **left sidebar** (navigation/context) plus a
**content area**; the reader adds a right-hand ToC rail on wide screens.

```
┌──────────────────────────────────────────────────────────────────────┐
│  docz-site            [ global search…                 ]   ◍ user ▾    │  top bar
├───────────────┬────────────────────────────────────────┬─────────────┤
│  SIDEBAR      │  CONTENT                                 │  ON THIS    │
│               │                                          │  PAGE       │
│  Repos        │  Repos / acme/platform / Designs /       │             │
│   acme/...    │  DESIGN-0009                             │  • Overview │
│   acme/web    │                                          │  • Goals    │
│   org/infra   │  # DESIGN 0009: docz-site …  [Draft]     │  • Detailed │
│               │                                          │    Design   │
│  Types        │  Overview                                │  • API      │
│   Designs     │  docz-site is the web front end that …   │  • Data     │
│   RFCs        │                                          │    Model    │
│   ADRs        │  Goals and Non-Goals                     │  • …        │
│               │  …                                       │             │
└───────────────┴────────────────────────────────────────┴─────────────┘
```

Breakpoints:

- **Wide (≥1280px):** sidebar + content + ToC rail (three columns).
- **Medium (≥768px):** sidebar + content; ToC collapses into an "On this page"
  disclosure at the top of the reader.
- **Small (<768px):** sidebar collapses behind a hamburger; single content
  column; search moves into a full-screen overlay.

The reader content column is width-capped (~72ch) for readability regardless of
viewport.

## API / Interface Changes

docz-site is a new app and defines no public API of its own. This section
describes (1) the docz-api endpoints it **consumes** and (2) the site's own
client routes (already listed in the route map above). Endpoint paths are
illustrative and must be reconciled with DESIGN-0008.

**Consumed docz-api endpoints:**

```
GET  /api/v1/repos
       → [{ owner, name, default_branch, doc_type_count, updated_at }]
       Access-scoped: only repos the session may read.

GET  /api/v1/repos/:owner/:repo
       → { owner, name, default_branch, docs_dir, doc_types: [...] }

GET  /api/v1/repos/:owner/:repo/types
       → [{ name, plural_label, id_prefix, statuses, doc_count }]
       Type-agnostic: whatever .docz.yaml enables, incl. custom types.

GET  /api/v1/repos/:owner/:repo/types/:type/docs
       → [{ doc_id, title, status, author, created, path, updated_at }]

GET  /api/v1/repos/:owner/:repo/types/:type/docs/:docId
       → { metadata: { id, title, status, author, created, ... },
           raw_md: "…full markdown including frontmatter…",
           git_sha }
       The reader's source of truth: RAW markdown + parsed metadata.

GET  /api/v1/search?q=&repo=&type=&status=&author=&page=
       → { hits: [{ repo, doc_id, type, title, status, author,
                     snippet }],
           facets: { repo: {...}, type: {...},
                     status: {...}, author: {...} },
           total }
       Backed by Meilisearch; results access-scoped server-side.

GET  /api/v1/auth/session   → { user } | 401
POST /api/v1/auth/logout    → 204
     (login/callback redirect endpoints per the auth flow above)
```

All content/search endpoints are **access-scoped server-side**; the site
receives only authorized data and surfaces 401/403 as described.

**Example: fetch and render one document.** This is the thin-vertical-slice
path — fetch raw markdown for `acme/platform / design / DESIGN-0009` and render
it safely. Renderer-specific calls are illustrative.

```ts
import DOMPurify from "dompurify";
import { renderMarkdown } from "./markdown"; // wraps markdown-it / remark

async function loadDoc(owner: string, repo: string, type: string, docId: string) {
  const res = await fetch(
    `/api/v1/repos/${owner}/${repo}/types/${type}/docs/${docId}`,
    { credentials: "include" },
  );
  if (res.status === 401) throw new AuthError();      // → /login
  if (res.status === 403) throw new ForbiddenError(); // → access-denied panel
  if (!res.ok) throw new ApiError(res.status);

  const doc: Doc = await res.json();

  // 1. render markdown body → HTML (renderer also highlights code)
  const rawHtml = renderMarkdown(stripFrontmatter(doc.rawMd));

  // 2. SANITIZE — mandatory: this is user content → HTML
  const safeHtml = DOMPurify.sanitize(rawHtml, { USE_PROFILES: { html: true } });

  // 3. build ToC from rendered headings (not docz's toc markers)
  const toc = buildToc(safeHtml);

  return { metadata: doc.metadata, html: safeHtml, toc };
}
```

```tsx
function DocReader({ owner, repo, type, docId }: DocReaderProps) {
  const { data, status } = useQuery({
    queryKey: ["doc", owner, repo, type, docId],
    queryFn: () => loadDoc(owner, repo, type, docId),
  });

  if (status === "pending") return <ReaderSkeleton />;
  if (status === "error")   return <ReaderError owner={owner} repo={repo} />;

  return (
    <article>
      <Breadcrumbs owner={owner} repo={repo} type={type} docId={docId} />
      <StatusBadge status={data.metadata.status} />
      <TableOfContents toc={data.toc} />
      {/* html is already sanitized by loadDoc */}
      <div className="prose" dangerouslySetInnerHTML={{ __html: data.html }} />
    </article>
  );
}
```

## Data Model

The site is **essentially stateless** — a thin client with no database. The
"data model" is (1) the TypeScript view-model shapes that mirror docz-api's JSON
and (2) the client-side query cache.

**View-model interfaces** (the contract the site renders against):

```ts
interface Repo {
  owner: string;
  name: string;            // `${owner}/${name}` is the repo key
  defaultBranch: string;
  docsDir: string;
  docTypeCount: number;
  updatedAt: string;       // ISO-8601
}

interface DocType {
  name: string;            // canonical type name, e.g. "design" or "frameworks"
  pluralLabel: string;     // display label, e.g. "Designs"
  idPrefix: string;        // e.g. "DESIGN", "FW"
  statuses: string[];      // valid statuses for this type (from .docz.yaml)
  docCount: number;
}

interface DocSummary {     // a row in a type's document list
  docId: string;           // stable frontmatter id, e.g. "DESIGN-0009"
  title: string;
  status: string;
  author: string;
  created: string;         // ISO-8601 date
  path: string;            // repo-relative source path
  updatedAt: string;
}

interface Doc {            // the full reader payload
  metadata: {
    id: string;
    title: string;
    status: string;
    author: string;
    created: string;
    [k: string]: unknown;  // forward-compatible with extra frontmatter keys
  };
  rawMd: string;           // raw markdown incl. frontmatter; rendered client-side
  gitSha: string;
}

interface SearchHit {
  repo: string;            // `${owner}/${name}`
  docId: string;
  type: string;
  title: string;
  status: string;
  author: string;
  snippet: string;         // highlighted excerpt from Meilisearch
}

interface SearchResponse {
  hits: SearchHit[];
  facets: {
    repo: Record<string, number>;
    type: Record<string, number>;
    status: Record<string, number>;
    author: Record<string, number>;
  };
  total: number;
}
```

`status` and `type` are typed as `string` (not a closed enum) because the type
and status sets are **per-repo, driven by `.docz.yaml`** — the site must render
whatever docz-api returns, including custom types and custom statuses.

**Client-side caching.** The only persisted-ish state is the in-memory query
cache (TanStack Query): keyed by route params (`["repos"]`,
`["repo", owner, name]`, `["docs", owner, repo, type]`,
`["doc", owner, repo, type, docId]`, `["search", params]`), with
stale-while-revalidate so navigating back to a list is instant while a
background refetch runs. Cache entries are dropped on logout. Rendered/sanitized
HTML may be memoized per `docId` within a session to avoid re-parsing on
re-mount; it is never written to durable storage. No localStorage of document
content (it would duplicate access-scoped data outside docz-api's control); only
UI preferences (theme, last provider) may be persisted.

## Testing Strategy

- **Component / unit (Vitest + Testing Library).** Breadcrumbs derive correctly
  from route params; the status badge maps known statuses to colors and falls
  back gracefully for unknown ones; facet selection updates the URL and
  vice-versa; loading/empty/error/auth states render for each data-bound view;
  the four-state matrix is covered per route.
- **Markdown rendering + sanitization (the security-critical suite).** A
  dedicated table of **XSS payloads** must be *neutralized* by the
  render → sanitize pipeline: `<script>alert(1)</script>`,
  `<img src=x onerror=alert(1)>`, `[x](javascript:alert(1))`,
  `<a href="javascript:…">`, event-handler attributes, `<iframe>`/`<object>`,
  and malicious inline SVG/MathML. Each test asserts the sanitized output
  contains no executable vector. Companion tests assert *benign* markdown
  (headings, tables, fenced code, links, images) survives intact and that ToC
  generation produces stable slugged anchors. These tests gate CI.
- **End-to-end (Playwright) against a mocked/fixture docz-api.** Drive the real
  flows — `/repos` → repo → type → reader; faceted search; 401→`/login` and
  403→access-denied — against recorded fixture responses (and a contract check
  that fixtures match docz-api's documented schema). Cover the thin-vertical-
  slice happy path first.
- **Accessibility checks.** Automated axe scans in component and e2e tests;
  keyboard navigation through sidebar, search, facets, and the in-page ToC;
  visible focus states; correct landmark/heading structure in the reader;
  color-contrast on status badges.

## Migration / Rollout Plan

This is a **greenfield repository** with no existing users to migrate; "rollout"
means the build order, which follows Decision 8 (thin vertical slice first).

1. **Slice 1 — render one repo's one type, no auth/search/nav.** Stand up the
   SPA, a single hardcoded route to a document reader, and the full reader
   pipeline (fetch raw md → render → **sanitize** → highlight → ToC → status
   badge) against a **minimal docz-api** (or fixtures) serving one repo's one
   type. This proves the end-to-end fetch→render→display loop and ships the
   security-critical sanitization suite from day one.
2. **Slice 2 — full navigation.** Add `/repos`, repo detail, type list, and
   breadcrumbs; wire the route map and the loading/empty/error states.
3. **Slice 3 — search.** Add `/search`, the facet sidebar, and the cross-repo
   views (which are just faceted queries). Resolve the proxy-vs-scoped-key
   question (Open Question 4).
4. **Slice 4 — auth.** Add the pluggable login flow (GitHub default; Okta /
   Keycloak), session handling, and 401/403 behavior, coordinated with
   docz-api. Until this lands, the site runs against an unauthenticated or
   single-tenant docz-api.
5. **Slice 5 — polish.** Responsive refinements, accessibility hardening,
   performance (code-split routes, prefetch on hover), and any SSR adoption if
   Open Question 3 lands on SSR.

Each slice is independently shippable; nothing in a later slice is required for
an earlier one to be useful.

## Open Questions

Each question is numbered; option `a` is the recommendation, later letters are
alternatives, and "Other" is free-form for review.

### 1. Which front-end framework?

- **a. (Recommended)** Vite + React SPA — no server runtime, fastest path to the
  thin vertical slice, docz-api as the only backend.
- b. Next.js (React + SSR) — SEO-able public docs, faster first paint, and a
  natural home for a BFF token exchange.
- c. SvelteKit — smaller bundles, built-in SSR, less boilerplate.
- d. Other.

### 2. Which markdown renderer library?

- **a. (Recommended)** `markdown-it` + plugins (anchors, footnotes, highlight) —
  framework-agnostic, mature, easy to feed into DOMPurify as an HTML string.
- b. `react-markdown` / `remark` — renders to React nodes (no `dangerouslySet…`
  for the common path), rich remark/rehype plugin ecosystem.
- c. `marked` — smallest/fastest, fewer plugins.
- d. Other.

### 3. SSR or SPA?

- **a. (Recommended)** SPA for the MVP — operationally simplest, defers a server
  runtime; revisit once public/SEO docs are a requirement.
- b. SSR from the start — better first paint, SEO, and a server seam for auth.
- c. Static prerender + client hydration for public docs, SPA for private.
- d. Other.

### 4. How does the site integrate with Meilisearch search?

- **a. (Recommended)** Proxy through a docz-api `/search` endpoint — the same
  server-side access scoping filters search results, no Meilisearch key reaches
  the browser, one backend.
- b. Direct Meilisearch instant-search with a per-session **scoped tenant
  token** issued by docz-api — lowest latency, native instant-search components,
  at the cost of trusting browser-side filter constraints.
- c. Hybrid: proxy for access-scoped queries, direct for public docs only.
- d. Other.

### 5. HTML sanitization approach?

- **a. (Recommended)** DOMPurify with a conservative allow-list, run after
  render and after highlighting — battle-tested, browser-native, configurable.
- b. A `rehype-sanitize` step in a remark/rehype pipeline (pairs with Open
  Question 2b) — sanitizes the AST before serialization.
- c. Server-side sanitization in docz-api before it serves markdown/HTML — moves
  the trust boundary off the client but conflicts with Decision 3 (client-side
  render).
- d. Other.

### 6. Styling / design-system choice?

- **a. (Recommended)** Tailwind CSS + a headless component set (e.g. Radix) plus
  a `prose` typography preset for the reader — fast, consistent, accessible
  primitives.
- b. A batteries-included component library (MUI / Chakra / Mantine) — more
  out-of-the-box, heavier and more opinionated.
- c. Plain CSS modules / vanilla-extract — minimal deps, more hand-rolled UI.
- d. Other.

### 7. Where does the OIDC/OAuth token exchange happen?

- **a. (Recommended)** docz-api owns the exchange and issues an httpOnly session
  cookie — keeps tokens out of browser JS and centralizes auth with the access
  control that already lives in docz-api.
- b. A thin site BFF (requires SSR per Open Question 3b) performs the exchange
  and brokers the docz-api session — keeps provider secrets at the site edge.
- c. Public-client PKCE in the SPA with tokens in memory — no server seam, but
  the browser holds tokens.
- d. Other.

### 8. How should docz's own ToC markers be handled?

- **a. (Recommended)** Generate the ToC from rendered headings and **strip** the
  `<!--toc:start-->…<!--toc:end-->` marker block from the displayed body — one
  ToC, always consistent with what's shown.
- b. Render the CLI-generated ToC list verbatim (trust the marker block) and
  skip heading-walking.
- c. Render the marker block *and* a generated rail, accepting possible
  duplication.
- d. Other.

## References

- INV-0005 — "docz-api and docz-site: centralized cross-repo docz registry and
  viewer" (source investigation; locked Decisions 2, 3, 6, 8 honored here).
- DESIGN-0008 — docz-api (the service docz-site consumes: GitHub App ingestion,
  Postgres registry caching raw markdown + metadata, Meilisearch index, webhook
  refresh, access-scoped/auth endpoints).
- DESIGN-0007 — the docz shared parsing library (`pkg/…` extracted from
  `internal/config` + `internal/document`) that docz-api depends on so the
  registry never drifts from the CLI; consumed by docz-site transitively via
  docz-api.
- DESIGN-0006 — custom document type support (why the registry and the site must
  be **type-agnostic**, driven by each repo's `.docz.yaml`).
- Meilisearch — full-text search + faceting backing docz-api's `/search`.
- markdown-it / react-markdown / remark — candidate client-side markdown
  renderers (Open Question 2).
- DOMPurify — HTML sanitization for the reader's XSS surface (Open Question 5).
- TanStack Query — client-side request cache / data layer.
- Playwright — end-to-end testing against a mocked/fixture docz-api.
