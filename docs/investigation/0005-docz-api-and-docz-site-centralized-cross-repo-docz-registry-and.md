---
id: INV-0005
title: "docz-api and docz-site: centralized cross-repo docz registry and viewer"
status: Concluded
author: Donald Gifford
created: 2026-06-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0005: docz-api and docz-site: centralized cross-repo docz registry and viewer

**Status:** Concluded
**Author:** Donald Gifford
**Date:** 2026-06-23

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1 — docz hands the ingester a manifest, not a haystack](#observation-1--docz-hands-the-ingester-a-manifest-not-a-haystack)
  - [Observation 2 — the rfc-api/rfc-site model maps cleanly, minus the hard parsing](#observation-2--the-rfc-apirfc-site-model-maps-cleanly-minus-the-hard-parsing)
  - [Observation 3 — proposed architecture](#observation-3--proposed-architecture)
  - [Observation 4 — the docz-side enabler: don't re-implement the parser](#observation-4--the-docz-side-enabler-dont-re-implement-the-parser)
  - [Observation 5 — the hard part is visibility/auth, and it is not docz-specific](#observation-5--the-hard-part-is-visibilityauth-and-it-is-not-docz-specific)
  - [Observation 6 — why docz-site is not "just MkDocs per repo"](#observation-6--why-docz-site-is-not-just-mkdocs-per-repo)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. How does docz-api fetch repo content?](#1-how-does-docz-api-fetch-repo-content)
  - [2. Where is markdown rendered to HTML?](#2-where-is-markdown-rendered-to-html)
  - [3. Which renderer?](#3-which-renderer)
  - [4. What does "version" mean for a doc?](#4-what-does-version-mean-for-a-doc)
  - [5. Which branch(es) are ingested?](#5-which-branches-are-ingested)
  - [6. What is the visibility / access model?](#6-what-is-the-visibility--access-model)
  - [7. How does the API obtain the doc list without re-implementing docz?](#7-how-does-the-api-obtain-the-doc-list-without-re-implementing-docz)
  - [8. Scope of the first deliverable?](#8-scope-of-the-first-deliverable)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Question

Can we build a small companion service — **docz-api** — that uses a GitHub App
to onboard repositories already using docz, ingests each repo's `.docz.yaml` to
build a registry of documents (mapped to repos and doc types), caches their
rendered content, and refreshes via webhooks on push/merge/release — paired with
a **docz-site** front end that lets a team browse, search, and read *all* their
docz documents from a single web URL?

And specifically: does docz's existing structure (`.docz.yaml`, standardized
directory layout, typed YAML frontmatter, stable IDs) make this materially
*simpler* than a general system like rfc-api / rfc-site?

## Hypothesis

**Yes — and meaningfully simpler than rfc-api.** rfc-api spends most of its
complexity on *discovery* and *normalization*: figuring out where docs live in
an arbitrary repo and coercing inconsistent metadata into a common shape. docz
already solves both problems at the source:

- `.docz.yaml` is a **manifest** — it declares `docs_dir` and every type's
  `dir`, `id_prefix`, `statuses`, and `plural_label`. The API never has to guess
  where docs are.
- Every document carries **typed, guaranteed frontmatter** (`id`, `title`,
  `status`, `author`, `created`), so registry rows and search fields map almost
  one-to-one with no heuristic parsing.
- IDs are **stable and namespaced** (`RFC-0001`, `FW-0003`), giving a natural
  per-repo primary key and a clean cross-repo identity (`<repo>/<id>`).

So docz-api is mostly "fetch → parse known structure → upsert → index → render
cache," with webhooks driving incremental refresh. The genuinely hard part is
not docz-specific — it is the same problem rfc-api has: **mapping who-can-see-
what** from GitHub repo access to site access.

## Context

docz today produces beautifully structured docs *per repo*, and the `wiki`
command can render a single repo's docs as MkDocs/TechDocs. What's missing is a
**cross-repo** view: a team with docz in 15 repos has 15 separate doc trees and
no single place to search "all our ADRs" or "every Draft design across the org."
This investigation validates whether a thin aggregation service modeled on
rfc-api / rfc-site (Postgres registry + Meilisearch + a viewer) is the right
shape, and what — if anything — docz itself must expose to support it cleanly.

**Triggered by:** the desire for one URL across all docz repos; relationship to
the existing per-repo `wiki` (MkDocs/TechDocs) integration and the mdp renderer
explored in [[0004-v1-release-plan-tui-markdown-preview-and-cli-parity]].

## Approach

This is a **desk / feasibility investigation** (architecture validation), not an
empirical spike with running code. The steps below are how the conclusion was
reached and how a follow-on design would be de-risked:

1. Enumerate exactly what docz already standardizes that an ingester can rely on
   (`.docz.yaml` schema, directory layout, frontmatter fields, index markers).
2. Map the rfc-api / rfc-site component model onto docz and identify which parts
   collapse or disappear because docz removes discovery/normalization.
3. Sketch the data model (Postgres), the ingestion + webhook refresh flow, the
   search index (Meilisearch), the render/cache strategy, and the docz-site
   navigation (repos → types → documents).
4. Identify the docz-side prerequisite: how the API gets a doc list without
   re-implementing docz's parser (shared library vs. JSON export vs. shelling
   out).
5. Surface the unavoidable risks (visibility/auth, versioning semantics, fetch
   strategy) as Open Questions for the design phase.

## Environment

Proposed stack (assumed, to be confirmed in design):

| Component            | Proposed choice                                        |
|----------------------|--------------------------------------------------------|
| Ingestion/onboarding | GitHub App (installation tokens, webhooks)             |
| Site user auth       | Pluggable OIDC: GitHub (default), Okta, Keycloak       |
| API service          | Go (reuses docz's own parsing packages)                |
| Registry store       | PostgreSQL (caches raw markdown + metadata at ingest)  |
| Search               | Meilisearch (full-text + faceting)                     |
| Markdown render      | JS renderer in docz-site (e.g. markdown-it)            |
| Front end            | docz-site (repos → types → docs nav + search + reader) |

## Findings

### Observation 1 — docz hands the ingester a manifest, not a haystack

The single biggest simplification: `.docz.yaml` *is* the per-repo manifest. To
ingest a repo, docz-api reads one file and learns:

- `docs_dir` — the root to scan.
- For each type: `dir`, `id_prefix`, `id_width`, `statuses`, `plural_label`,
  and (since IMPL-0012) `aliases`. This includes **custom types**, so the
  registry must be type-agnostic and driven entirely by the repo's config —
  never a hardcoded list of the six built-ins.

Then for each enabled type it scans `<docs_dir>/<dir>/*.md` and parses
frontmatter — the exact job docz's own `internal/document.ScanDocuments`
already does. The output (`id`, `title`, `status`, `author`, `created`, path)
maps directly onto a registry row and a search document. No discovery, no
metadata coercion. This is the work rfc-api cannot avoid and docz-api gets for
free.

### Observation 2 — the rfc-api/rfc-site model maps cleanly, minus the hard parsing

| rfc-api / rfc-site concern | docz-api equivalent | Notes |
|---|---|---|
| Discover where docs live | Read `.docz.yaml` | Eliminated as a problem |
| Normalize metadata | Read typed frontmatter | Eliminated; schema is fixed |
| Registry store | Postgres | Same |
| Full-text search | Meilisearch | Same |
| Render markdown | mdp → cached HTML | Reuse docz's renderer |
| Onboarding | GitHub App install | Simpler: one app, opt-in repos |
| Refresh on change | Webhooks (push/release) | Same pattern |
| **Access control** | **Map GH repo access → site** | **Unchanged; the hard part** |

### Observation 3 — proposed architecture

**Onboarding.** A GitHub App is installed on selected repos/orgs. Required
permissions are minimal: `contents: read`, `metadata: read`; webhook events:
`installation` / `installation_repositories`, `push`, and `release`. On install,
docz-api enumerates installed repos and ingests any that contain a `.docz.yaml`
at the repo root.

**Ingestion (docz-api).** Per repo: fetch `.docz.yaml` + the doc files, parse,
upsert into Postgres (caching the raw markdown + metadata), and index into
Meilisearch. Markdown→HTML is **not** done here — docz-site renders client-side
(Decision 3). Fetching can use the GitHub Git Trees/Contents API (no checkout)
for small doc sets, or a shallow clone for large ones.

**Data model (Postgres), sketch:**

- `repos(id, installation_id, owner, name, default_branch, docs_dir,
  config_snapshot jsonb, last_synced_sha, updated_at)`
- `doc_types(id, repo_id, name, dir, id_prefix, plural_label, statuses jsonb)`
- `documents(id, repo_id, type, doc_id, title, status, author, created_at,
  path, git_sha, content_hash, raw_md, updated_at)` — unique on
  `(repo_id, doc_id)`. `raw_md` is the cached markdown; docz-site renders it to
  HTML client-side (Decision 3), so no `rendered_html` column is stored.

**Refresh via webhooks.** A `push` to the default branch touching `docs_dir`
(or `.docz.yaml`) triggers a diff-based re-ingest of only the changed files;
`.docz.yaml` changes re-sync the type set. A `release`/tag can optionally
snapshot a version. Events are debounced. `content_hash` gates re-ingest so
unchanged docs are not re-processed or re-indexed.

**Search (Meilisearch).** One index of documents with searchable `title`/`body`
and facets on `repo`, `type`, `status`, `author`. Drives both global search and
filtered views ("all Approved designs across repos").

**docz-site.** Navigation mirrors the mental model the request describes:
**repos list → a repo → its doc types → a document**, plus a global search box
and a rendered reader (HTML + ToC + a status badge from frontmatter). Cross-repo
views (by type or status) fall out of the Meilisearch facets for free.

### Observation 4 — the docz-side enabler: don't re-implement the parser

docz-api should not re-implement frontmatter/config parsing — that would drift
from the CLI. Three options, in order of preference:

1. **Extract a shared Go module.** Promote the relevant pieces of
   `internal/config` (Load/Validate, type resolution) and `internal/document`
   (`ScanDocuments`, `LoadFrontmatter`) into a reusable `pkg/…` surface that both
   the docz CLI and docz-api import. One source of truth for "what docz docs
   exist in this tree."
2. **Machine-readable export.** docz already has `docz list --format json`;
   a small `docz export --json` (whole-repo manifest: config + all types + all
   docs) would let the API consume docz's own output if a checkout is available.
3. **Shell out** to the docz binary — simplest but couples deployment to having
   the CLI installed and a checkout present.

Option 1 is the cleanest and turns docz-api into a true peer of the CLI. This is
a real, additive docz roadmap item, not a blocker for a prototype.

### Observation 5 — the hard part is visibility/auth, and it is not docz-specific

The one piece that is *not* simplified: a viewer aggregating private-repo content
must enforce that a site user only sees docs they are allowed to see. This is
exactly rfc-api's hardest problem, and it splits into two layers:

- **Authentication (who is this user?)** is **pluggable** (Decision 6). The site
  supports three providers behind one OIDC-shaped abstraction: **GitHub**
  (default and preferred — the GitHub App already in play for ingestion doubles
  as the OAuth identity provider), **Okta**, and **Keycloak**. GitHub uses
  OAuth; Okta and Keycloak use standard OIDC. A single provider interface
  (`issuer`, `client_id/secret`, scopes/claims) keeps the three configurable
  rather than forked.
- **Authorization (what may they see?)** is where the providers diverge and must
  be designed explicitly:
  - *GitHub provider* — mirror GitHub access: a user sees a repo's docs only if
    their token can read that repo (checked/cached per session). This is the
    natural, tightest mapping.
  - *Okta / Keycloak providers* — there is no GitHub repo-permission signal, so
    authorization comes from **OIDC group/role claims** mapped to repos or repo
    groups in docz-api config (e.g. `group:platform → repos[…]`), or a coarser
    "any authenticated org member sees all onboarded repos" for an internal
    deployment.

Note the **asymmetry**: ingestion is *always* via the GitHub App (that is how
docz-api reads repo content), independent of how site *users* authenticate. So
"auth provider" is strictly about the human reading the site, not about how the
service pulls content. This must be decided early because it shapes the API,
the session model, and the config surface.

### Observation 6 — why docz-site is not "just MkDocs per repo"

docz's existing `wiki` integration renders **one repo** as MkDocs/TechDocs.
docz-site is deliberately different: it is **cross-repo aggregation + unified
search + a single URL**. The two are complementary — per-repo TechDocs for deep
single-project browsing, docz-site for the org-wide index. Worth stating in the
design so the two features don't appear to overlap.

## Conclusion

**Answer: Yes — feasible, and simpler than rfc-api.** Because docz standardizes
location (`.docz.yaml`), structure (typed dirs), and metadata (frontmatter), the
ingestion side of docz-api is largely mechanical, and the rfc-api/rfc-site
architecture (GitHub App → Postgres registry → Meilisearch → viewer, refreshed by
webhooks) maps cleanly onto it. The dominant remaining complexity is the
visibility/auth model — now scoped to a **pluggable provider abstraction**
(GitHub by default, plus Okta and Keycloak via OIDC; Decision 6) — which is
inherent to any cross-repo viewer of private content and not unique to docz. A
clean implementation also motivates a small, additive docz change: exposing the
existing config/document parsing as a shared library so the API and CLI never
diverge (Decision 7).

## Recommendation

The open questions are resolved (see [Decisions](#decisions)). Next steps:

1. **Proceed to a DESIGN** for docz-api (the service: GitHub App, ingestion,
   data model, webhook refresh, search) and either a combined or sibling DESIGN
   for docz-site (the viewer: nav, search, reader). One service design with the
   site as a section is likely enough to start.
2. **Design the pluggable auth layer first** (Decision 6) — the GitHub /
   Okta / Keycloak provider abstraction and, critically, the per-provider
   *authorization* mapping (GitHub repo access vs. OIDC group claims). It gates
   the API and the session model.
3. **Scope the docz prerequisite** (Observation 4 / Decisions 7): extract a
   shared `pkg/…` parsing library from `internal/config` + `internal/document`
   so docz-api and the CLI never diverge.
4. **Build a thin vertical slice first** (Decision 8): one repo onboarded by
   hand → ingest → one type rendered in a bare docz-site, deferring auth and
   webhooks, to prove the fetch→parse→render→serve loop before the full pipeline.

## Open Questions

> **Resolved 2026-06-23** — see the [Decisions](#decisions) table below for the
> chosen option per question. The menu of alternatives is kept for the record.

Each question is numbered; option `a` is the recommendation, later letters are
alternatives, and "other" is free-form for review.

### 1. How does docz-api fetch repo content?

- **a. (Recommended)** GitHub Git Trees / Contents API — no checkout, ideal for
  small doc sets and simple ops.
- b. Shallow `git clone` per refresh — better for large doc trees, heavier ops.
- c. Hybrid: Contents API by default, clone above a size threshold.
- d. Other.

### 2. Where is markdown rendered to HTML?

- **a. (Recommended)** Render at ingest time and cache `rendered_html` in
  Postgres; invalidate on `content_hash` change.
- b. Store raw markdown only; render in docz-site at request time.
- c. Render client-side in the browser.
- d. Other.

### 3. Which renderer?

- **a. (Recommended)** Reuse docz's mdp-based renderer for visual parity with the
  CLI preview.
- b. A server-side Go renderer in the API (e.g. goldmark) independent of mdp.
- c. A JS renderer in docz-site.
- d. Other.

### 4. What does "version" mean for a doc?

- **a. (Recommended)** Track the default-branch HEAD as the single current
  version (MVP); store `git_sha` per doc.
- b. Snapshot versions on git tags / GitHub releases.
- c. Keep full per-doc history.
- d. Other.

### 5. Which branch(es) are ingested?

- **a. (Recommended)** Default branch only.
- b. Configurable per repo.
- c. All branches (preview/PR builds).
- d. Other.

### 6. What is the visibility / access model?

- **a. (Recommended)** Mirror GitHub repo access: GitHub App for ingestion +
  user OAuth on the site, checking the viewer's repo access. Correct, and the
  main complexity.
- b. Org-internal single-tenant: trust any authenticated org member (simplest
  MVP).
- c. Public read-only (open-source docs only).
- d. Other.

### 7. How does the API obtain the doc list without re-implementing docz?

- **a. (Recommended)** Extract docz's `internal/config` + `internal/document`
  scanning into a shared `pkg/…` library imported by both CLI and API.
- b. Add a `docz export --json` whole-repo manifest the API consumes (requires a
  checkout / the binary).
- c. Re-implement frontmatter + config parsing in the API (risks drift).
- d. Other.

### 8. Scope of the first deliverable?

- **a. (Recommended)** A thin vertical slice: one hand-onboarded repo →
  ingest → render one type in a minimal docz-site, no auth/webhooks yet.
- b. Full ingestion pipeline + webhooks first, site second.
- c. Site/UX prototype first against mocked data.
- d. Other.

## Decisions

Resolved by user review on 2026-06-23. Recommendations accepted except
Decisions 3 and 6, which were adjusted per reviewer guidance.

| # | Topic | Choice | Rationale |
|---|-------|--------|-----------|
| 1 | Repo content fetch | (a) GitHub Git Trees / Contents API | No checkout; simplest ops for typical doc-set sizes |
| 2 | Cache point | (a) Cache at ingest | Cache the raw markdown + metadata in Postgres at ingest (HTML itself is rendered client-side per Decision 3) |
| 3 | Markdown renderer | **(c) JS renderer in docz-site** | mdp is intentionally small for single-user neovim/terminal use and isn't a fit for a server-side multi-tenant renderer; render in the site instead |
| 4 | "Version" semantics | (a) Default-branch HEAD as current version | Simplest correct MVP; store `git_sha` per doc |
| 5 | Branch scope | (a) Default branch only | Avoids preview/PR-branch noise |
| 6 | Auth model | **(a) GitHub default + pluggable OIDC** | GitHub App OAuth is default/preferred; a generic OIDC provider abstraction must also support Okta and Keycloak. All three are required; authorization mapping differs per provider (see Observation 5) |
| 7 | Doc-list source | (a) Shared `pkg/…` library from docz | Single source of truth; API and CLI never drift |
| 8 | First deliverable | (a) Thin vertical slice | Prove fetch→parse→render→serve on one repo before the full pipeline |

## References

- docz `.docz.yaml` schema and type model — `internal/config`
- docz document scanning / frontmatter — `internal/document`
  (`ScanDocuments`, `LoadFrontmatter`)
- docz custom document types — DESIGN-0006 / IMPL-0012 (registry must be
  type-agnostic, driven by each repo's config)
- docz per-repo wiki (MkDocs/TechDocs) — the per-repo counterpart to docz-site
- mdp markdown renderer — [[0004-v1-release-plan-tui-markdown-preview-and-cli-parity]]
  (considered for server-side rendering but rejected in Decision 3 — it is
  intentionally scoped to single-user neovim/terminal use)
- Prior art: rfc-api / rfc-site (the general system docz-api simplifies)
