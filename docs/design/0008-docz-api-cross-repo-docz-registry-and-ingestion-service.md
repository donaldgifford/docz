---
id: DESIGN-0008
title: "docz-api: cross-repo docz registry and ingestion service"
status: Approved
author: Donald Gifford
created: 2026-06-23
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0008: docz-api: cross-repo docz registry and ingestion service

**Status:** Approved **Author:** Donald Gifford **Date:** 2026-06-23

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [Service shape and package layout](#service-shape-and-package-layout)
  - [Consuming the docz parsing library (pkg/doczcore)](#consuming-the-docz-parsing-library-pkgdoczcore)
  - [GitHub App setup and onboarding](#github-app-setup-and-onboarding)
  - [Ingestion pipeline](#ingestion-pipeline)
  - [Auth and session model](#auth-and-session-model)
  - [Search](#search)
  - [Component / sequence overview](#component--sequence-overview)
- [Requirements for the docz repo (what docz-api needs from docz)](#requirements-for-the-docz-repo-what-docz-api-needs-from-docz)
- [API / Interface Changes](#api--interface-changes)
  - [HTTP / JSON endpoints](#http--json-endpoints)
  - [Reconciling the API contract with docz-site](#reconciling-the-api-contract-with-docz-site)
  - [Service config surface (environment variables)](#service-config-surface-environment-variables)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. Reconcile the API path shape and versioning with docz-site?](#1-reconcile-the-api-path-shape-and-versioning-with-docz-site)
  - [2. How is the docz parsing library pinned while building?](#2-how-is-the-docz-parsing-library-pinned-while-building)
  - [3. HTTP router / framework?](#3-http-router--framework)
  - [4. Postgres access layer?](#4-postgres-access-layer)
  - [5. Database migration tool?](#5-database-migration-tool)
  - [6. REST or GraphQL for the JSON API?](#6-rest-or-graphql-for-the-json-api)
  - [7. Synchronous ingest or a background worker?](#7-synchronous-ingest-or-a-background-worker)
  - [8. Where are sessions stored?](#8-where-are-sessions-stored)
  - [9. Webhook retry / idempotency strategy?](#9-webhook-retry--idempotency-strategy)
  - [10. How is Okta/Keycloak group to repo authorization configured?](#10-how-is-oktakeycloak-group-to-repo-authorization-configured)
  - [11. Meilisearch API-key scoping for any direct site access?](#11-meilisearch-api-key-scoping-for-any-direct-site-access)
  - [12. Tag/release version snapshots now, or stay HEAD-only?](#12-tagrelease-version-snapshots-now-or-stay-head-only)
  - [13. Multi-org / multi-tenant model?](#13-multi-org--multi-tenant-model)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Overview

**docz-api** is a small Go backend service that aggregates the documentation of
many repositories — all of which already use the **docz** CLI — into one
searchable registry behind a single JSON API. A team running docz in fifteen
repos has fifteen separate doc trees and no single place to search "all our
ADRs" or "every Draft design across the org." docz-api closes that gap; its
companion front end, **docz-site** (DESIGN-0009), renders the registry into a
browsable, searchable web app.

The service has four responsibilities:

1. **Onboard** repositories through a GitHub App. Installed repos are expected
   to contain a root `.docz.yaml` (the docz manifest).
2. **Ingest** each repo's `.docz.yaml` plus its docz documents into a Postgres
   registry and a Meilisearch index.
3. **Cache** the raw markdown plus parsed frontmatter/metadata — _not_ rendered
   HTML. Rendering happens client-side in docz-site.
4. **Refresh** incrementally via webhooks (`push` / `release`) on the default
   branch, and **serve** a JSON API to docz-site: repos → types → docs, plus
   search and auth.

This design derives from **INV-0005** (the feasibility investigation that
concluded docz-api is buildable and meaningfully simpler than a general system
like rfc-api, because docz standardizes location, structure, and metadata at the
source). It honors the eight decisions locked there. Crucially, docz-api does
**not** re-implement docz's parser: it consumes the shared Go parsing library
extracted from the docz CLI in **DESIGN-0007** (`pkg/doczcore/config` +
`pkg/doczcore/document`), so the registry never drifts from the CLI's own notion
of "what docz docs exist in this tree."

> **Self-contained note.** This document seeded the new `docz-api` repository,
> where it now lives as that repo's own canonical, **Approved** design (docz-api
> `DESIGN-0001`). This copy is the docz-repo mirror, updated to reflect the
> design as actually decided so the docz repo stays accurate. It additionally
> carries one section the service copy does not —
> **[Requirements for the docz repo](#requirements-for-the-docz-repo-what-docz-api-needs-from-docz)**
> — the contract docz-api places on this repo (chiefly DESIGN-0007). Two
> upstream documents remain load-bearing: **DESIGN-0007** defines the shared
> parsing library this service imports; **INV-0005** is the investigation whose
> locked decisions this design implements.

## Goals and Non-Goals

### Goals

- **Cross-repo registry.** A Postgres store keyed by `(repo, doc_id)` holding
  every docz document across every onboarded repo, with its frontmatter, cached
  raw markdown, and `git_sha`.
- **Type-agnostic ingestion.** The registry is driven entirely by each repo's
  `.docz.yaml`. Custom docz types (e.g. `frameworks` / `FW-0001`) ingest
  identically to the six built-ins, and a type is addressable by its canonical
  name, its `id_prefix`, or a declared `alias` — the same resolution the CLI
  uses (DESIGN-0006). The six built-in type names are **never** hardcoded
  anywhere in the service.
- **GitHub App onboarding + incremental refresh.** Repos opt in by installing
  the app; `push` / `release` webhooks on the default branch drive diff-based
  partial re-ingest; `content_hash` gates redundant work.
- **Raw-markdown caching, not HTML.** The API serves raw markdown plus metadata
  (Decisions 2 + 3); docz-site renders to HTML client-side.
- **Pluggable site-user authentication (Decision 6).** Authentication is
  provider-based: GitHub (default/preferred), Okta, and Keycloak — all three
  required. _Authorization_ (which repos a user may read) is out of scope for
  this iteration and deferred to a future SpiceDB-backed middleware (Decision
  10); the enforcement seam is built now so it slots in additively.
- **Full-text + faceted search.** A Meilisearch index over `title`/`body` with
  facets on `repo` / `type` / `status` / `author`.
- **A stable, versioned JSON contract** that docz-site (DESIGN-0009) consumes
  unchanged — the site is built against these endpoints from the first slice.
- **A thin vertical slice first (Decision 8).** One hand-onboarded repo → ingest
  → serve one type, deferring full auth and webhooks, to prove the fetch → parse
  → upsert → serve loop end to end.

### Non-Goals

- **Rendering markdown to HTML.** No `rendered_html` column, no server-side
  renderer. Rendering is docz-site's job (Decision 3). docz's mdp renderer is
  intentionally scoped to single-user neovim/terminal use and is not a fit for a
  server-side multi-tenant renderer.
- **Non-default branches and PR/preview builds.** The default branch HEAD is the
  single current version (Decisions 4 + 5). Per-doc version history is a later
  extension: the plan is to consume each repo's `CHANGELOG.md` as the source of
  truth for an audit/versions log and surface it in the UI, rather than snapshot
  on git tags (Decision 12).
- **Authorization (for now).** This iteration ships authentication only;
  deciding _which_ repos/docs a user may read is deferred to a future
  SpiceDB-backed middleware (Decision 10). Until then every authenticated user
  sees all onboarded repos.
- **Re-implementing docz's parser.** Frontmatter and config parsing come from
  the DESIGN-0007 shared library only (Decision 7).
- **Building docz-site.** The viewer (nav, reader, client-side rendering) is
  DESIGN-0009. docz-api is strictly the backend.
- **Per-repo MkDocs/TechDocs.** docz's existing `wiki` command renders one repo
  as MkDocs; docz-api is deliberately the complementary cross-repo aggregator,
  not a replacement for per-repo TechDocs.
- **Write-back to repos.** docz-api is read-only against GitHub content; it
  never opens PRs or mutates a source repo.

## Background

docz produces structured docs _per repo_. Each repo has a root `.docz.yaml`
manifest that declares `docs_dir` and, for every enabled type, its `dir`,
`id_prefix`, `id_width`, `statuses`, `plural_label`, and `aliases`. Every
document carries typed, guaranteed frontmatter:

```yaml
---
id: RFC-0001
title: "Some title"
status: Draft
author: Jane Dev
created: 2026-01-15
---
```

These three properties make ingestion mechanical rather than heuristic:

- **`.docz.yaml` is a manifest, not a haystack.** The service reads one file and
  learns exactly where docs live and what types exist — no discovery, and custom
  types are first-class because the type set is data, not code.
- **Frontmatter is typed and fixed-shape.** `id`, `title`, `status`, `author`,
  `created` map almost one-to-one onto a registry row and a search document — no
  metadata coercion.
- **IDs are stable and namespaced.** `RFC-0001`, `FW-0003` give a natural
  per-repo primary key and a clean cross-repo identity `<repo>/<doc_id>`.

The hard part — established in INV-0005 — is **not** docz-specific: a viewer
aggregating private-repo content must enforce that a site user sees only the
docs they are allowed to see. INV-0005 split that into **authentication** (who
is the user, pluggable) and **authorization** (what may they see). This
iteration delivers authentication; authorization is a deliberately deferred
follow-on (Decision 10). Note the asymmetry that drives the whole design:
**ingestion is always via the GitHub App** (that is how docz-api reads repo
content), independent of how site _users_ authenticate.

The DESIGN-0007 shared library is what makes the "don't re-implement the parser"
decision concrete. It promotes the docz CLI's `internal/config` (`Load`,
`Validate`, type resolution) and `internal/document` (`ScanDocuments`,
`LoadFrontmatter`, `ParseFrontmatter`) into an importable `pkg/doczcore/…`
surface. Both the CLI and docz-api depend on it, so the registry's view of a
repo is byte-for-byte the CLI's view.

## Detailed Design

### Service shape and package layout

docz-api is a single Go binary (`cmd/docz-api/`) over library code in
`internal/`, consistent with the repo's conventions (`slog` for logs, no
`init()` for behavior, dependencies wired in `main()`, `internal/` as a hard
wall). The proposed internal decomposition:

```text
cmd/docz-api/        # main: parse flags, configure slog, load config, wire deps,
                     #   run migrations, start HTTP server + ingest worker
internal/
  config/            # service configuration loaded from the environment
  githubapp/         # GitHub App client: app JWT, installation tokens (cached),
                     #   Git Trees / Blobs / Contents fetch
  webhook/           # HMAC-SHA256 verification + event routing
  queue/             # Redis-backed ingest job queue (enqueue + worker pool)
  ingest/            # the pipeline: fetch → parse → diff/gate → upsert → index
  store/             # Postgres access (sqlc + pgx): installations/repos/
                     #   doc_types/documents/users; goose migrations
  session/           # Redis-backed session store (issue / lookup / revoke)
  search/            # Meilisearch index management + query
  auth/              # Provider interface + github/okta/keycloak authN impls
                     #   (authZ is a future SpiceDB-backed middleware — see below)
  httpapi/           # chi router, handlers, middleware (session + authz seam)
```

Two `config` packages coexist: docz-api's `internal/config` (service env config)
and the imported `github.com/donaldgifford/docz/pkg/doczcore/config` (the parsed
`.docz.yaml`). The docz one is imported with an alias (e.g. `doczcfg`) wherever
both are in scope, to keep call sites unambiguous.

**Infrastructure dependencies:** Postgres (durable registry + users), Redis
(ingest job queue + session store), and Meilisearch (search index). Redis is
adopted up front (Decisions 7 + 8): the same instance backs the worker queue
that decouples webhook delivery from ingestion _and_ the session store with
natural TTL eviction, and it sets the service up to scale horizontally without a
later migration.

### Consuming the docz parsing library (pkg/doczcore)

Per DESIGN-0007 (Decision 7), docz-api imports the promoted parsing core and
never parses `.docz.yaml` or frontmatter itself:

```go
import (
    doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
    doczdoc "github.com/donaldgifford/docz/pkg/doczcore/document"
)
```

The two entry points the service relies on most:

- `doczcfg.Load` + `(*Config).Validate` + `(*Config).EnabledTypes` +
  `(*Config).TypeDir` — resolve the manifest and the type set to scan, including
  custom types (the type-agnostic contract from DESIGN-0006).
- `doczdoc.ParseFrontmatter(content []byte)` — parse frontmatter from
  **already-fetched bytes**. Because docz-api fetches over the GitHub Contents
  API and never has a local checkout (Decision 1), this byte-oriented entry
  point — not the disk-reading `ScanDocuments`/`LoadFrontmatter` — is the one
  used in the hot path. `doczcfg`'s type resolution + `doczdoc.IsDoczFile` tell
  the fetcher _which_ tree paths to pull.

> **Resolved inconsistency (library path).** An earlier draft of this doc
> referenced the promoted packages as `pkg/config` + `pkg/document`.
> DESIGN-0007's accepted recommendation (its Open Question 1a) is the
> `doczcore`-namespaced `pkg/doczcore/config` + `pkg/doczcore/document`, which
> this design adopts. The exact import path is still gated on DESIGN-0007
> shipping; see Open Question 2 and the
> [Requirements](#requirements-for-the-docz-repo-what-docz-api-needs-from-docz)
> section.

Because this surface is semver-governed (DESIGN-0007), docz-api pins a specific
docz tag in `go.mod` so a CLI change can never silently alter ingestion; bumps
are deliberate.

### GitHub App setup and onboarding

docz-api is a GitHub App (not an OAuth app for ingestion — see the auth section
for the separate site-login concern). The app is installed on selected
repos/orgs and reads content using short-lived installation tokens.

**Required permissions (minimal):**

| Permission | Level | Why                                                             |
| ---------- | ----- | --------------------------------------------------------------- |
| `contents` | read  | Fetch `.docz.yaml` and doc files via the Git Trees/Contents API |
| `metadata` | read  | List installed repos, read the default branch                   |

**Webhook events subscribed:**

| Event                       | Triggers                                                                                          |
| --------------------------- | ------------------------------------------------------------------------------------------------- |
| `installation`              | App installed/uninstalled → enumerate or purge repos                                              |
| `installation_repositories` | Repos added/removed from an install → onboard/offboard                                            |
| `push`                      | Commit to a branch → re-ingest if it is the default branch and touches `docs_dir` or `.docz.yaml` |
| `release`                   | Release published → reserved for tag/release snapshots (Open Question 12)                         |

**Installation-token auth flow.** docz-api authenticates to GitHub per request
chain as follows:

1. Sign a short-lived **app JWT** (RS256) with the App ID and the app private
   key (`iss` = app id, `exp` ≤ 10 min).
2. Exchange it for an **installation access token** via
   `POST /app/installations/{installation_id}/access_tokens`. The token is
   scoped to that installation's repos and expires in ~1 hour.
3. Cache the installation token per `installation_id` until just before expiry;
   refresh on demand.
4. Use the installation token as a `Bearer` credential on Git Trees / Contents
   API calls.

**Onboarding.** On `installation` / `installation_repositories`, docz-api
enumerates the installation's repos (`GET /installation/repositories`), and for
each repo checks for a root `.docz.yaml`. If present, it inserts an
`installations` row and a `repos` row and enqueues a full ingest. If absent, the
repo is recorded but marked unconfigured (no docz manifest) and skipped.

**Webhook HMAC verification.** Every inbound webhook is verified before any
work:

- The app is configured with a webhook secret.
- GitHub signs the raw request body with HMAC-SHA256 and sends
  `X-Hub-Signature-256: sha256=<hex>`.
- docz-api recomputes `HMAC-SHA256(secret, raw_body)` and compares with
  `hmac.Equal` (constant-time). A mismatch returns `401` and the payload is
  dropped. The `X-GitHub-Delivery` id is logged and recorded for idempotency
  (see Open Question 9).

### Ingestion pipeline

The pipeline is one logical flow with several triggers (initial onboard, webhook
refresh, manual re-sync). It is type-agnostic from end to end: the set of types
comes from the parsed `.docz.yaml`, never a constant. Triggers do not run the
pipeline inline — they **enqueue an ingest job onto the Redis-backed queue**
(Decision 7) and return immediately (a webhook gets `202`); a pool of workers
drains the queue and runs the stages below. This keeps webhook handling fast,
centralizes debounce/coalesce, and lets ingestion scale independently of the
HTTP front end.

```text
 trigger (onboard | push | release | manual)
        │
        ▼
 ┌──────────────────┐   fetch via GitHub Git Trees API (recursive=1)
 │  Fetcher         │   → resolve default-branch HEAD sha
 │  (githubapp)     │   → list tree, pull .docz.yaml + docs blobs (no checkout)
 └────────┬─────────┘
          │ raw bytes: .docz.yaml + matched *.md
          ▼
 ┌──────────────────┐   doczcfg.Load(.docz.yaml) + (*Config).Validate
 │  Parser          │   doczdoc.ParseFrontmatter per fetched doc blob
 │  (pkg/doczcore)  │   → []DocEntry{Frontmatter, Filename, Content}
 └────────┬─────────┘
          │ config snapshot + parsed doc entries
          ▼
 ┌──────────────────┐   compute content_hash per doc (sha256 of raw_md)
 │  Differ / Gate   │   compare to stored content_hash → changed/new/deleted set
 └────────┬─────────┘
          │ changed set only
          ▼
 ┌──────────────────┐   upsert repos / doc_types / documents (one tx)
 │  Postgres Writer │   delete documents absent from this HEAD
 └────────┬─────────┘
          │ changed doc ids
          ▼
 ┌──────────────────┐   add/replace/delete Meilisearch documents
 │  Search Indexer  │   keyed by composite "<repo_id>:<doc_id>"
 └──────────────────┘
```

**Step detail:**

1. **Fetch (Git Trees API, no checkout — Decision 1).** Resolve the default
   branch HEAD sha (`GET /repos/{owner}/{name}` → `default_branch`, then the
   ref). Pull the recursive tree
   (`GET /repos/{owner}/{name}/git/trees/{sha}?recursive=1`), filter to
   `.docz.yaml` and blobs under `docs_dir/<type.dir>/`, and fetch each blob
   (`GET /repos/{owner}/{name}/git/blobs/{blob_sha}`, base64-decoded). This is
   ideal for typical doc-set sizes; a shallow-clone path is deferred (folded
   into the background-worker / scaling question).

2. **Parse (DESIGN-0007 library).** `doczcfg.Load` + `Validate` produce the
   resolved `Config` (which already merges defaults and resolves type aliases /
   `id_prefix`). For each enabled type, the fetched blobs are parsed with
   `doczdoc.ParseFrontmatter` into `DocEntry` values carrying `Frontmatter` and
   the raw `Content`. No parsing logic lives in docz-api.

3. **content_hash gating.** For each doc, `content_hash = sha256(raw_md)`. If a
   stored row has the same `(repo_id, doc_id, content_hash)`, the doc is
   unchanged and is skipped for both Postgres and Meilisearch. This is the
   primary cost gate.

4. **Diff-based partial re-ingest on push.** A `push` webhook carries the list
   of added/modified/removed paths across its commits. docz-api intersects that
   list with `docs_dir` to decide _whether_ to ingest, and to narrow blob
   fetches to changed files when possible (still validating against the full
   tree for deletions). Docs present in the registry but absent from the new
   HEAD tree are **deleted** from Postgres and Meilisearch.

5. **`.docz.yaml` change re-syncs the type set.** If the push touches
   `.docz.yaml`, docz-api re-parses it and reconciles `doc_types`: types added
   to the config are created, removed types (and their orphaned `documents`) are
   deleted, and changed `statuses` / `plural_label` / `dir` / `aliases` are
   updated. Because the type set is config-driven, a repo adding a custom
   `frameworks` type "just works" on the next push.

6. **Debounce.** Rapid successive pushes to the same repo (e.g. a squash-merge
   followed by a tag) are coalesced: an ingest job for a repo that already has a
   pending job is collapsed, and a per-repo debounce window (`INGEST_DEBOUNCE`,
   default a few seconds) batches bursts so the latest HEAD wins.

7. **Upsert + index, Postgres first.** The `documents` upsert and `doc_types`
   reconcile run in one DB transaction; the Meilisearch update follows commit.
   If indexing fails after commit, the doc's changed state is recorded so a
   reconcile job can re-index without a full re-ingest (eventual consistency;
   the search index trails Postgres briefly).

### Auth and session model

Two independent identity concerns, kept strictly separate (INV-0005 Decision 6):

- **Ingestion identity** is **always** the GitHub App installation token,
  regardless of how site users log in. Nothing in this section changes that.
- **Site-user identity** is **pluggable** across three providers, behind one
  abstraction.

**Authentication now; authorization later (Decision 10).** This design ships
**authentication only**. The pluggable providers establish _who_ a user is and
docz-api issues a session. Per-user **authorization** — _which_ repos and docs a
user may read — is intentionally **deferred to a future feature**: a middleware
that consults an external authorization service backed by **SpiceDB**. Until
that lands, the deployment is an internal tool and **any authenticated user can
read all onboarded repos** (consistent with the single-tenant model in Decision
13). The enforcement seam is built now so the future layer is purely additive —
see _Authorization seam_ below.

**Provider abstraction (authN).** A single Go interface keeps the three
providers configurable rather than forked. Note it no longer carries an
`AuthorizedRepos` method — that was the authZ half, now deferred:

```go
// Provider authenticates a site user (the "who"). Authorization (the "what")
// is handled separately by a future SpiceDB-backed middleware.
type Provider interface {
    Name() string                    // "github" | "okta" | "keycloak"
    AuthCodeURL(state string) string // begin OAuth/OIDC
    Exchange(ctx context.Context, code string) (*Identity, error)
}

type Identity struct {
    Provider string
    Subject  string   // stable per-provider user id
    Email    string
    Login    string   // github login, when present
    Groups   []string // OIDC group/role claims — retained for the future authZ layer
}
```

**OIDC / OAuth flow.** Standard authorization-code flow on three endpoints (see
API section): `/auth/login?provider=…` redirects to the provider's `AuthCodeURL`
with a signed `state`; the provider redirects back to `/auth/callback`; docz-api
calls `Exchange`, upserts a `users` row, issues a session (stored in Redis), and
sets an httpOnly session cookie. GitHub uses OAuth; Okta and Keycloak use
standard OIDC discovery (`issuer`, `client_id`, `client_secret`, scopes). The
`Groups` claims are captured on the session so the future authZ layer has them
without a re-login.

**Authorization seam (the future SpiceDB layer).** Every read endpoint passes
through a single `authorize` middleware that resolves an _allowed-repo set_ for
the request. Today that resolver returns "all onboarded repos" for any valid
session. The planned feature replaces only this resolver with one that queries a
**SpiceDB**-backed authorization service — modeling GitHub repo access and/or
OIDC group claims as SpiceDB relationships — and returns the user's actual
allowed set. When it lands, the behaviors already wired behind the seam switch
on with no handler changes:

- list endpoints filter to authorized repos/docs;
- a doc fetch for an unauthorized-but-existing repo returns `404` (existence
  hiding), `403` for an authenticated-but-explicitly-denied case, `401` for
  no/invalid session;
- search injects a `repo IN (allowed…)` Meilisearch filter server-side so the
  index can never leak across the boundary.

> **Note on docz-site's 401 vs 403 behavior.** DESIGN-0009 treats `401` as "log
> in" and `403` as "access denied" UI states. The taxonomy above is compatible —
> the site renders whatever the API returns — and is worth confirming when the
> two are wired together (and again when the authZ layer introduces real
> `403`/`404` distinctions).

### Search

Meilisearch holds one `documents` index. Each Postgres document maps to one
Meilisearch document (shape in the Data Model section). Configuration:

- **Searchable attributes:** `title`, `body` (the raw markdown, indexed as
  text). `title` is ranked above `body`.
- **Filterable attributes (facets):** `repo`, `type`, `status`, `author`. These
  drive cross-repo filtered views — "all Approved designs across repos" is a
  facet query, not custom code — and back the per-session `repo IN (…)`
  authorization filter.
- **Sortable attributes:** `created`, `updated_at`.
- **Primary key:** a composite string `id = "<repo_id>:<doc_id>"` so the same
  `RFC-0001` in two repos never collides.

Indexing is keyed off the same content_hash gate as Postgres: unchanged docs are
not re-indexed; deleted docs are removed by primary key. Search results return
enough metadata (repo, type, doc_id, title, status, snippet) for docz-site to
render a result list and link to the full doc fetch.

All search goes **through docz-api**, not directly to Meilisearch (Decision 11):
the API owns the Meilisearch key and is the single point where the future
authorization filter is injected. A side benefit, mirroring rfc-api, is that the
search endpoint is a plain HTTP/JSON surface usable straight from `curl` — which
makes adding a **search tool to a future MCP server** trivial, since it is just
another consumer of the same endpoint.

### Component / sequence overview

```text
                          ┌───────────────────────────────────────────┐
   GitHub  ── webhook ───▶ │                docz-api (Go)               │
   (App)   ◀─ Trees API ── │                                           │
                          │  ┌──────────┐   ┌──────────┐  ┌─────────┐  │
                          │  │ webhook  │──▶│ ingest   │─▶│ search  │──┼─▶ Meilisearch
                          │  │ handler  │   │ pipeline │  │ indexer │  │
                          │  └──────────┘   └────┬─────┘  └─────────┘  │
                          │                      │                     │
                          │                      ▼                     │
                          │                 ┌─────────┐                │
                          │                 │ Postgres│◀──── reads ────┤
                          │                 │ registry│                │
   docz-site ── HTTP ────▶ │  ┌──────────┐  └─────────┘                │
   (DESIGN-0009)         │  │ httpapi  │──── auth (Provider) ──────────┼─▶ GitHub/Okta/Keycloak
                          │  └──────────┘                              │
                          └───────────────────────────────────────────┘
```

**Ingest sequence (push on default branch):**

```text
GitHub        Webhook        Ingest          Parser(pkg)    Postgres   Meili
  │  push       │              │                  │            │         │
  ├────────────▶│ verify HMAC  │                  │            │         │
  │             ├─ default br? touches docs_dir? ─┤            │         │
  │             ├─────────────▶│ debounce/coalesce│            │         │
  │             │              ├─ fetch tree+blobs│            │         │
  │             │              ├─────────────────▶│ Load/Parse │         │
  │             │              │◀── []DocEntry ───┤            │         │
  │             │              ├─ content_hash diff ──────────▶│ upsert  │
  │             │              │                  │            ├─ commit │
  │             │              ├─ index changed ──┼────────────┼────────▶│
  │             │◀─ 202 ───────┤                  │            │         │
```

Redis (not shown above) sits between the webhook handler and the ingest worker
as the job queue, and separately holds the session store; Postgres and
Meilisearch are the durable stores.

## Requirements for the docz repo (what docz-api needs from docz)

docz-api depends on the **docz repo** for exactly one thing at runtime: the
shared parsing library (DESIGN-0007). This section is the consumer-side contract
— the concrete surface, behavior, and release mechanics docz-api needs — written
so it can be treated as acceptance criteria for DESIGN-0007, plus two
forward-looking asks. Everything here is consistent with DESIGN-0007's own
goals; it just states them from docz-api's side.

- **R1 — A public, importable parsing package (DESIGN-0007).** Promote
  `internal/config` + `internal/document` to `pkg/doczcore/config` +
  `pkg/doczcore/document` (DESIGN-0007 OQ1a). docz-api imports these by their
  public paths; the `internal/` originals are unreachable from another module.
  No permanent re-export shim is needed on docz-api's behalf.

- **R2 — The exact exported surface docz-api calls.** The minimum public
  symbols (a consumer subset — `TypeConfig` has additional fields, e.g.
  `Template` and `StatusField`, that docz-api does not read):

  | Package             | Symbols docz-api imports                                                                                                                                                                                                                                                                                                                                                               |
  | ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
  | `doczcore/config`   | `Load(configFile, repoRoot) (Config, error)`; `(*Config).Validate() ([]string, error)`; `(*Config).EnabledTypes() []string`; `(*Config).TypeDir(string) string`; `(*Config).ValidateType(string) (string, error)`; types `Config`, `TypeConfig` (fields `Enabled`, `Dir`, `IDPrefix`, `IDWidth`, `Statuses`, `PluralLabel`, `Aliases`), `DocType`, `Status`; sentinel `ErrUnknownType` |
  | `doczcore/document` | `ParseFrontmatter(content []byte) (Frontmatter, error)`; `IsDoczFile(name string) bool`; types `Frontmatter` (fields `ID`, `Title`, `Status`, `Author`, `Created`), `DocEntry` (`Frontmatter` + `Filename` + `Content []byte`); sentinel `ErrNoFrontmatter`. Nice-to-have (tests / a future checkout path): `ScanDocuments`, `LoadFrontmatter`                                         |

- **R3 — `ParseFrontmatter` (bytes-in) is load-bearing, not just
  `ScanDocuments`.** docz-api fetches blobs over the GitHub API and never has a
  checkout, so it needs the content-from-bytes entry point, not only the
  disk-reading scanner. DESIGN-0007 already promotes it; this flags it as a hard
  requirement rather than an incidental export.

- **R4 — Type resolution by name / `id_prefix` / alias must be preserved
  (DESIGN-0006).** `ValidateType` (and its underlying `resolveType`) must
  canonicalize a token via name → alias (built-in registry + per-type
  `aliases:`) → `id_prefix`, exactly as the CLI does. docz-api exposes `{type}`
  URL segments that accept any of these forms, so `TypeConfig.Aliases` must be
  part of the public struct and the resolution order must match the CLI
  byte-for-byte.

- **R5 — `DocEntry.Content` carries the raw file bytes.** The cached `raw_md`
  the registry stores _is_ `DocEntry.Content`; docz-api persists it directly
  with no second read. The field must stay populated unconditionally (it is
  today).

- **R6 — A semver-governed surface and a real tag to pin.** The promoted surface
  becomes public API under semver. docz-api needs: (a) a tagged release `vX.Y.0`
  that introduces `pkg/doczcore`; (b) additive-only evolution within a major
  (adding fields is fine; renaming/removing a field or changing a signature is a
  major bump); (c) the sentinels (`ErrUnknownType`, `ErrNoFrontmatter`) treated
  as a supported `errors.Is` contract. **Concrete ask:** cut that tag before
  docz-api's first non-prototype release. Until it exists, docz-api prototypes
  against a local `replace` (Open Question 2), but the `replace` must not
  outlive the tag.

- **R7 — Stable `.docz.yaml` and frontmatter contract.** The five typed
  frontmatter fields and the `.docz.yaml` type schema are docz-api's column
  contract. DESIGN-0007 already commits to no schema change; the requirement
  here is that any _future_ breaking change to either be coordinated with a
  docz-api major bump, not shipped silently.

- **R8 — Not required: `docz export --json`.** docz-api consumes the Go library,
  not a JSON export. DESIGN-0007 OQ3's optional `docz export --json` is
  explicitly **not** a docz-api dependency; if it ships, it is for other
  (non-Go) consumers. Stated so it is never built or blocked on docz-api's
  behalf.

- **R9 — (Forward-looking) `CHANGELOG.md` as the versions/audit source.**
  Decision 12 stays HEAD-only now but plans to consume each onboarded repo's
  `CHANGELOG.md` as the source of truth for an audit/versions log, surfaced in
  docz-site. This is **not** a DESIGN-0007 ask and needs no action now; it is
  flagged so that if docz later standardizes a changelog location/format (or
  grows a `docz changelog` concept), docz-api is a known downstream consumer.

When R1–R7 are met and the tag is cut, docz-api drops its `replace` directive
and pins the published version; nothing else in the docz repo is required for
docz-api to build and run.

## API / Interface Changes

This is a greenfield service, so "interface changes" means the initial HTTP/JSON
surface plus the config surface. All responses are JSON; read endpoints are
filtered through the authorization seam (currently a pass-through; see Auth).

### HTTP / JSON endpoints

The base path is **`/api/v1`** (see Open Question 1 and the reconciliation note
below). Routes:

| Method | Path                                                      | Purpose                                                   |
| ------ | --------------------------------------------------------- | --------------------------------------------------------- |
| `GET`  | `/healthz`                                                | Liveness/readiness (DB + Meilisearch reachable)           |
| `GET`  | `/api/v1/repos`                                           | List onboarded repos the session may read                 |
| `GET`  | `/api/v1/repos/{owner}/{name}`                            | Repo detail (config snapshot, last_synced_sha, doc types) |
| `GET`  | `/api/v1/repos/{owner}/{name}/types`                      | List doc types for a repo                                 |
| `GET`  | `/api/v1/repos/{owner}/{name}/types/{type}/docs`          | List docs of a type                                       |
| `GET`  | `/api/v1/repos/{owner}/{name}/types/{type}/docs/{doc_id}` | Get one doc: raw markdown + metadata                      |
| `GET`  | `/api/v1/search?q=&repo=&type=&status=&author=`           | Faceted full-text search                                  |
| `GET`  | `/api/v1/auth/session`                                    | Current session/user, or `401`                            |
| `GET`  | `/auth/login?provider={github\|okta\|keycloak}`           | Begin OAuth/OIDC                                          |
| `GET`  | `/auth/callback`                                          | OAuth/OIDC redirect target; issues session                |
| `POST` | `/api/v1/auth/logout`                                     | Invalidate session                                        |
| `POST` | `/webhooks/github`                                        | GitHub App webhook receiver (HMAC-verified)               |

Notes:

- `{type}` accepts the type's canonical name **or** its `id_prefix` / alias,
  resolved by the same DESIGN-0007 `Config` type-resolution the CLI uses, so
  `…/types/frameworks/docs` and `…/types/FW/docs` are equivalent (DESIGN-0006).
- `{doc_id}` is the frontmatter id (`RFC-0001`), case-sensitive, matching the
  CLI's convention. It is unique within a repo.
- The doc fetch returns **raw markdown** in `raw_md`; docz-site renders it.

**Example — get a document:**

```http
GET /api/v1/repos/acme/platform/types/rfc/docs/RFC-0001 HTTP/1.1
Host: docz-api.internal
Cookie: docz_session=…
```

```json
{
  "repo": "acme/platform",
  "doc_id": "RFC-0001",
  "type": "rfc",
  "title": "Adopt structured logging",
  "status": "Accepted",
  "author": "Jane Dev",
  "created": "2026-01-15",
  "path": "docs/rfc/0001-adopt-structured-logging.md",
  "git_sha": "9f1c2ab…",
  "content_hash": "sha256:7b4e…",
  "updated_at": "2026-06-22T18:04:11Z",
  "raw_md": "---\nid: RFC-0001\ntitle: \"Adopt structured logging\"\n…\n---\n\n# RFC 0001 …"
}
```

**Example — search:**

```http
GET /api/v1/search?q=logging&type=rfc&status=Accepted HTTP/1.1
Host: docz-api.internal
Cookie: docz_session=…
```

```json
{
  "query": "logging",
  "estimated_total_hits": 2,
  "hits": [
    {
      "repo": "acme/platform",
      "doc_id": "RFC-0001",
      "type": "rfc",
      "title": "Adopt structured logging",
      "status": "Accepted",
      "author": "Jane Dev",
      "snippet": "…adopt <em>structured logging</em> across services…"
    }
  ],
  "facets": {
    "repo": { "acme/platform": 2 },
    "type": { "rfc": 2 },
    "status": { "Accepted": 1, "Draft": 1 }
  }
}
```

### Reconciling the API contract with docz-site

This doc's original draft and docz-site (DESIGN-0009) described the same API two
slightly different ways. The differences are small but real, and the site is the
consumer, so this design adopts the **superset that satisfies the consumer**:

| Concern        | Original draft           | DESIGN-0009 (site)                     | This design                                         |
| -------------- | ------------------------ | -------------------------------------- | --------------------------------------------------- |
| Version prefix | `/api/…` (none)          | `/api/v1/…`                            | **`/api/v1/…`** — explicit versioning               |
| Doc fetch path | `…/docs/{doc_id}` (flat) | `…/types/{type}/docs/{docId}` (nested) | **Nested** under type; matches the site's route map |
| Session check  | implicit                 | `GET /api/v1/auth/session`             | **Included**                                        |

The nested doc path is canonical. Because `{doc_id}` is already unique within a
repo, a flat `…/repos/{owner}/{name}/docs/{doc_id}` alias is trivial to offer
too if a non-site consumer wants it, but the site's nested form is the one
guaranteed. This reconciliation is recorded as Open Question 1 / Decision 1.

### Service config surface (environment variables)

```env
# Postgres
DATABASE_URL=postgres://docz:secret@db:5432/docz_api?sslmode=require

# Redis (ingest job queue + session store)
REDIS_URL=redis://redis:6379/0

# Meilisearch
MEILI_HOST=http://meili:7700
MEILI_API_KEY=…                # admin/index key; never reaches the browser (OQ 11)

# GitHub App (ingestion — always GitHub, regardless of site auth)
GITHUB_APP_ID=123456
GITHUB_APP_PRIVATE_KEY=/run/secrets/docz_app.pem   # PEM path or PEM body
GITHUB_WEBHOOK_SECRET=…
GITHUB_API_BASE=https://api.github.com             # override for GHES

# Site auth providers (pluggable; enable one or more)
AUTH_PROVIDERS=github,okta,keycloak                # comma-separated; default "github"
SESSION_SECRET=…                                   # signs session cookies

# GitHub OAuth (site login)
GITHUB_OAUTH_CLIENT_ID=…
GITHUB_OAUTH_CLIENT_SECRET=…

# Okta (OIDC) — authentication only; group→repo authorization is a future
# SpiceDB-backed feature (Decision 10), so no group mapping is configured here.
OKTA_ISSUER=https://acme.okta.com/oauth2/default
OKTA_CLIENT_ID=…
OKTA_CLIENT_SECRET=…

# Keycloak (OIDC) — authentication only (see Okta note above).
KEYCLOAK_ISSUER=https://kc.acme.com/realms/acme
KEYCLOAK_CLIENT_ID=…
KEYCLOAK_CLIENT_SECRET=…

# Session
SESSION_TTL=720h               # session lifetime in Redis (e.g. 30d)

# Ingestion tuning
INGEST_DEBOUNCE=5s
```

No group→repo authorization mapping appears here: authorization is deferred to a
future SpiceDB-backed middleware (Decision 10). When that feature lands, its
configuration (an endpoint/credentials for the authZ service) is added then.

## Data Model

The Postgres schema refines the INV-0005 sketch. All timestamps are
`timestamptz`. JSONB columns hold the config snapshot and per-type `statuses`
exactly as parsed, so the registry can answer "what types/statuses did this repo
declare" without a second source of truth.

```sql
-- A GitHub App installation (one per org/account that installed the app).
CREATE TABLE installations (
    id              BIGINT PRIMARY KEY,            -- GitHub installation id
    account_login   TEXT        NOT NULL,          -- org or user that installed
    account_type    TEXT        NOT NULL,          -- 'Organization' | 'User'
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- An onboarded repository. config_snapshot is the parsed .docz.yaml.
CREATE TABLE repos (
    id               BIGSERIAL PRIMARY KEY,
    installation_id  BIGINT      NOT NULL REFERENCES installations(id) ON DELETE CASCADE,
    owner            TEXT        NOT NULL,
    name             TEXT        NOT NULL,
    default_branch   TEXT        NOT NULL,
    docs_dir         TEXT        NOT NULL,          -- from .docz.yaml
    config_snapshot  JSONB       NOT NULL,          -- full parsed .docz.yaml
    last_synced_sha  TEXT,                          -- default-branch HEAD last ingested
    last_synced_at   TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner, name)
);

-- Per-repo doc types, driven entirely by .docz.yaml (custom types included).
CREATE TABLE doc_types (
    id            BIGSERIAL PRIMARY KEY,
    repo_id       BIGINT NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    name          TEXT   NOT NULL,                  -- canonical, e.g. 'rfc','frameworks'
    dir           TEXT   NOT NULL,                  -- e.g. 'rfc'
    id_prefix     TEXT   NOT NULL,                  -- e.g. 'RFC','FW'
    plural_label  TEXT   NOT NULL,                  -- display label
    statuses      JSONB  NOT NULL,                  -- ["Draft","Accepted",…]
    aliases       JSONB  NOT NULL DEFAULT '[]',     -- per-type CLI shorthands
    UNIQUE (repo_id, name)
);

-- One row per docz document at the default-branch HEAD.
CREATE TABLE documents (
    id            BIGSERIAL PRIMARY KEY,
    repo_id       BIGINT NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    type          TEXT   NOT NULL,                  -- canonical type name
    doc_id        TEXT   NOT NULL,                  -- frontmatter id, e.g. 'RFC-0001'
    title         TEXT   NOT NULL,
    status        TEXT,
    author        TEXT,
    created       DATE,                             -- frontmatter created
    path          TEXT   NOT NULL,                  -- repo-relative path
    git_sha       TEXT   NOT NULL,                  -- blob sha of the file
    content_hash  TEXT   NOT NULL,                  -- sha256 of raw_md (re-ingest gate)
    raw_md        TEXT   NOT NULL,                  -- cached markdown (NOT html)
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (repo_id, doc_id)
);

CREATE INDEX documents_repo_type_idx ON documents (repo_id, type);
CREATE INDEX documents_status_idx    ON documents (status);

-- Site users (one durable row per provider identity that has logged in;
-- used for audit. Sessions themselves live in Redis, see below).
CREATE TABLE users (
    id           BIGSERIAL PRIMARY KEY,
    provider     TEXT NOT NULL,                     -- 'github' | 'okta' | 'keycloak'
    subject      TEXT NOT NULL,                     -- stable per-provider id
    email        TEXT,
    login        TEXT,                              -- github login when present
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, subject)
);

-- Processed webhook deliveries, for idempotency (see Open Question 9).
CREATE TABLE webhook_deliveries (
    delivery_id   TEXT PRIMARY KEY,                 -- X-GitHub-Delivery
    event         TEXT        NOT NULL,
    received_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

There is **no `sessions` table**: sessions live in Redis (Decision 8). There is
no per-provider group→repo authorization table either, because authorization is
deferred to a future SpiceDB-backed layer (Decision 10), which owns its own
relationship store rather than a row in this schema.

**Redis structures.** Redis backs two concerns:

- **Session store** — key `sess:<id>` → a small JSON value
  `{ user_id, provider, login, email, groups[], created_at }` with a TTL of
  `SESSION_TTL`. The opaque `<id>` is the httpOnly cookie value; lookup is O(1),
  expiry is automatic, and logout is a single `DEL` (server-side revocation).
  `groups` is carried so the future authZ layer needs no re-login.
- **Ingest job queue** — webhook/onboard/manual triggers enqueue a job
  `{ repo_id, reason, head_sha? }`; a worker pool consumes it (Decision 7). A
  per-repo debounce/coalesce key collapses bursts so the latest HEAD wins. A
  durable task-queue library over Redis (e.g. `asynq`) or Redis streams provides
  the at-least-once delivery and retry the worker relies on; the `content_hash`
  gate and `X-GitHub-Delivery` idempotency (Decision 9) make redelivery cheap
  and safe. The short-lived GitHub installation tokens may also be cached in
  Redis (keyed by `installation_id`) so all replicas share one token rather than
  each minting its own.

**Meilisearch document shape.** One Meilisearch document per `documents` row:

```json
{
  "id": "42:RFC-0001",
  "repo": "acme/platform",
  "repo_id": 42,
  "doc_id": "RFC-0001",
  "type": "rfc",
  "title": "Adopt structured logging",
  "status": "Accepted",
  "author": "Jane Dev",
  "created": "2026-01-15",
  "body": "# RFC 0001 … full raw markdown body for full-text search …",
  "updated_at": 1750615451
}
```

- `id` is the composite primary key `<repo_id>:<doc_id>`.
- `title` + `body` are searchable; `repo` / `type` / `status` / `author` are
  filterable facets; `created` / `updated_at` are sortable.

## Testing Strategy

- **Unit — parsing / ingest mapping.** Given fixture bytes for `.docz.yaml` and
  a set of `*.md` docs, assert the parse (via the DESIGN-0007 library) → row
  mapping is correct: frontmatter → `documents` columns, types → `doc_types`,
  `content_hash` stable for identical bytes and changed when content changes.
  Include a **custom type** fixture (`frameworks` / `FW-0001`, addressable by
  prefix/alias) to prove no built-in is hardcoded, and a doc _missing_
  frontmatter to prove it is skipped without aborting the repo.

- **Integration — Postgres + Meilisearch + webhook handlers.** Spin Postgres and
  Meilisearch with **testcontainers-go** (or an equivalent harness). Cover: full
  ingest of a fixture tree; a second ingest with one changed doc (only the
  changed row re-written, content_hash gate proven); a `.docz.yaml` change that
  adds/removes a type (reconcile path); a doc deletion (row + index entry
  removed). Drive these through the webhook handler with synthetic `push`
  payloads so the trigger path is exercised, not just the inner pipeline.

- **Webhook signature tests.** Table-driven HMAC-SHA256 cases: a correct
  `X-Hub-Signature-256` passes; a wrong secret, a tampered body, and a missing
  header all return `401` and perform no DB writes. Assert constant-time
  comparison is used (no early-exit on first byte). A replayed
  `X-GitHub-Delivery` is a no-op (idempotency, Open Question 9).

- **End-to-end onboarding.** A fixture repo (committed under `testdata/`, served
  through a recorded/replayed GitHub client) is onboarded start to finish:
  `installation` event → enumerate → detect `.docz.yaml` → full ingest → assert
  `/api/v1/repos`, `/api/v1/repos/.../types`, and a doc fetch return the
  expected shapes. The GitHub Trees/Contents calls are replayed from recorded
  fixtures so the test is hermetic.

- **Auth enforcement.** With a stub authorization resolver returning a fixed
  allowed-repo set, assert list endpoints filter to authorized repos, an
  unauthorized doc fetch returns `404`, and the search filter injects
  `repo IN (allowed…)` so an out-of-scope doc never appears in results. (These
  exercise the seam that the future SpiceDB layer plugs into.)

- **Contract check against docz-site.** Keep a small set of golden response
  fixtures that match the shapes DESIGN-0009 consumes, so a breaking change to
  the JSON contract fails CI here before it breaks the site.

- **Golden / fixture discipline.** Reuse the docz convention: fixture trees and
  expected JSON under `testdata/`, regenerated with an `-update` flag, never
  hand-edited.

## Migration / Rollout Plan

docz-api is a **greenfield repository**; this document is its seed design. There
is no in-place migration of an existing system — "rollout" is the order in which
the service is built and shipped.

1. **Tagged docz library dependency (DESIGN-0007).** docz-api's `go.mod` depends
   on the docz module exposing `pkg/doczcore/config` + `pkg/doczcore/document`.
   This is a hard prerequisite for parsing. During prototyping the dependency
   may be a local `replace` against a docz checkout; before the first
   non-prototype release it is pinned to a published tag so a CLI change cannot
   silently alter ingestion (Open Question 2; see also the Requirements section,
   R6).

2. **Thin vertical slice first (Decision 8).** Ship the smallest end-to-end
   loop: one **hand-onboarded** repo (its `installation_id` / `owner` / `name`
   seeded directly, no GitHub App install UX) → fetch via Trees API → parse →
   upsert Postgres → serve `/api/v1/repos`, one `/types`, and a doc fetch.
   **Auth is stubbed** (the authorize seam returns "all repos") and **webhooks
   are deferred** (re-ingest is a manual endpoint or CLI subcommand). This
   proves fetch → parse → upsert → serve before any of the harder subsystems.

3. **DB migrations.** Schema is managed by `goose` (Decision 5), checked into
   the repo and embeddable so the binary runs `migrate up` on deploy. Migrations
   are forward-only and additive where possible so a binary rollback does not
   require a destructive down-migration.

4. **Layer in the rest.** In order: the Redis-backed ingest queue + worker
   (Decision 7); real GitHub App onboarding + installation-token flow; the
   webhook receiver (HMAC + `push`/`release` + `X-GitHub-Delivery` idempotency);
   the content_hash-gated diff/debounce pipeline; the Meilisearch indexer; then
   the pluggable **authentication** providers (GitHub first, then Okta/Keycloak)
   with Redis-backed sessions. Authorization (the SpiceDB middleware) is a
   **later** feature behind the seam, not part of this rollout.

5. **Container / deploy.** A single Go binary in a distroless image, plus
   Postgres, Redis, and Meilisearch (managed services or sidecar containers).
   The service is stateless — all state lives in Postgres, Redis, and the search
   index — so it scales horizontally behind a load balancer; webhook delivery to
   any replica is fine because the job goes onto the shared Redis queue and
   ingest reconciles against the stored `last_synced_sha`. Secrets (app private
   key, webhook secret, OIDC client secrets, Meilisearch key) come from the
   platform's secret store via the env vars above.

6. **Then docz-site (DESIGN-0009)** consumes the JSON API. The site is built
   against the slice's endpoints from the start so the contract is exercised
   early.

## Open Questions

> **Resolved 2026-06-30** — see the [Decisions](#decisions) table below for the
> chosen option per question. The menu of alternatives is kept for the record.
> Two answers refine their recommendation: **7 + 8** adopt **Redis now** (worker
> queue _and_ session store) rather than deferring it, and **10** is answered
> _"Other"_ — ship **authentication only**, with authorization deferred to a
> future SpiceDB-backed middleware.

Each question is numbered; option `a` is the recommendation, later letters are
alternatives, and **Other** is free-form for review.

### 1. Reconcile the API path shape and versioning with docz-site?

An earlier draft of this doc and docz-site (DESIGN-0009) disagreed on the URL
shape. docz-site is the consumer, so the consumer's shape should win.

- **a. (Recommended)** Adopt `/api/v1/…` with the **type-nested** doc path
  `…/repos/{owner}/{name}/types/{type}/docs/{doc_id}` exactly as DESIGN-0009
  expects, and add `GET /api/v1/auth/session`. Optionally also expose a flat
  `…/repos/{owner}/{name}/docs/{doc_id}` alias for non-site consumers.
- b. Keep an unversioned `/api/…` with a flat doc path and instead change
  DESIGN-0009 to match the server.
- c. Versioned `/api/v1/…` but flat doc path (`…/docs/{doc_id}`), pushing a
  small change onto docz-site's route map.
- Other.

### 2. How is the docz parsing library pinned while building?

DESIGN-0007 must ship `pkg/doczcore` before docz-api can import it; the slice
should not be blocked waiting for a tag.

- **a. (Recommended)** Local `replace github.com/donaldgifford/docz => ../docz`
  during prototyping, then pin a published tag (`require … vX.Y.Z`) before the
  first non-prototype release; no `replace` survives release.
- b. Wait for DESIGN-0007 to cut the tag first and only ever consume a published
  version (no `replace`), accepting that ingestion work is blocked until then.
- c. Vendor a docz checkout (`go mod vendor`) for hermetic builds, still via the
  public import path.
- Other.

### 3. HTTP router / framework?

- **a. (Recommended)** `chi` — lightweight, idiomatic `net/http` middleware and
  URL params, no framework lock-in, plays well with `slog` and stdlib handlers.
- b. Standard-library `net/http` with the Go 1.22+ pattern router — zero
  dependencies, at the cost of hand-rolling some middleware.
- c. `echo` or `gin` — batteries-included routing/binding, heavier and more
  opinionated.
- Other.

### 4. Postgres access layer?

- **a. (Recommended)** `sqlc` (compile-time-checked SQL → typed Go) over `pgx` —
  explicit SQL, no ORM surprises, fits the small fixed schema and the JSONB
  columns well.
- b. `pgx` with hand-written queries and a thin repository layer — maximum
  control, more boilerplate.
- c. `sqlx` — light struct-scanning over `database/sql`, simplest to start.
- d. `gorm` — full ORM; fastest to scaffold, least transparent for the
  transactional upsert/reconcile path.
- Other.

### 5. Database migration tool?

- **a. (Recommended)** `goose` — simple, Go-native, embeddable so the binary can
  run `migrate up` on deploy; SQL or Go migrations.
- b. `golang-migrate` — widely used, CLI + library, large driver set.
- c. `atlas` — declarative schema-as-source with generated migrations; more
  power, more concepts.
- Other.

### 6. REST or GraphQL for the JSON API?

- **a. (Recommended)** Plain REST/JSON as specified above. The resource shape
  (repos → types → docs + search) is shallow and well-bounded; REST keeps the
  server simple, is trivially cacheable, and is enough for docz-site's needs.
- b. GraphQL — one flexible endpoint lets docz-site shape exactly the data it
  needs and avoids over-fetching across the nav tree.
- c. REST now, add a GraphQL gateway later only if the site's query patterns
  prove awkward.
- Other.

### 7. Synchronous ingest or a background worker?

- **a. (Recommended)** In-process background worker. Webhooks enqueue an ingest
  job and return `202` immediately; a worker does fetch/parse/upsert/index.
  Keeps webhook handling fast and lets debounce/coalesce live in one place. Also
  where a future shallow-clone path for very large repos would land. Start with
  an in-process queue; promote to a durable queue only if needed.
- b. Fully synchronous ingest inside the webhook handler — simplest for the thin
  slice, but risks GitHub webhook timeouts on large repos.
- c. External/durable queue (Postgres-backed job table, NATS, or SQS) from day
  one for durability and retry.
- Other.

### 8. Where are sessions stored?

- a. Postgres `sessions` table. One store to operate and supports server-side
  revocation, at the cost of manual expiry sweeps.
- **b. (Chosen)** Redis — faster session reads and natural TTL eviction. A
  second datastore, but it is being added anyway for the ingest worker queue
  (Decision 7), so sessions ride along on the same instance.
- c. Stateless signed JWT cookies — no session store, but revocation and
  refreshing the cached allowed-repo set become awkward.
- Other.

### 9. Webhook retry / idempotency strategy?

- **a. (Recommended)** Idempotency on `X-GitHub-Delivery` (recorded in
  `webhook_deliveries`) plus reconcile against `last_synced_sha`, so a replayed
  or duplicate delivery is a no-op. Rely on GitHub's own redelivery for
  transient failures.
- b. A durable job table with explicit retry/backoff and a dead-letter for
  poison deliveries.
- c. At-least-once with no dedup, accepting that the content_hash gate makes
  re-ingest cheap and mostly harmless.
- Other.

### 10. How is Okta/Keycloak group to repo authorization configured?

- a. A service-config mapping file per provider (`*_GROUP_REPO_MAP`) of
  `group → [repos…]`, hot-reloadable, plus a coarse "any authenticated member
  sees all" toggle for internal deployments.
- b. Database-backed `repo_groups` tables managed through an admin API — more
  operable at scale, more to build.
- c. Encode the mapping directly in OIDC claims (a custom claim listing repo
  slugs) so the provider owns it entirely.
- **Other (Chosen).** Ship **authentication only** for now; defer authorization
  entirely to a future middleware that calls a **SpiceDB**-backed authorization
  service. Until then every authenticated user sees all onboarded repos. The
  enforcement seam is built now so the SpiceDB resolver slots in additively.

### 11. Meilisearch API-key scoping for any direct site access?

- **a. (Recommended)** docz-site never talks to Meilisearch directly; all search
  goes through docz-api with a server-side `repo IN (allowed…)` filter, and only
  the API holds the Meilisearch admin/index key. No key reaches the browser.
  (Also yields a curl-friendly search surface that eases a future MCP search
  tool.)
- b. Issue Meilisearch **tenant tokens** (scoped, short-lived, with an embedded
  filter) so docz-site can query Meilisearch directly for lower latency, at the
  cost of trusting the embedded filter.
- c. A separate read-only Meilisearch key with no embedded filter, relying on
  the API to never expose it — weakest option.
- Other.

### 12. Tag/release version snapshots now, or stay HEAD-only?

- **a. (Recommended)** Stay HEAD-only for now (honors Decision 4): the default
  branch HEAD is the single current version, `git_sha` is stored per doc, and
  the `release` webhook is wired but only logged. The intended later approach is
  to consume each repo's `CHANGELOG.md` as the audit/versions source of truth
  and present it in the UI, behind a schema extension.
- b. Add a `doc_versions` table now and snapshot on `release`, paying the
  storage/complexity cost up front.
- c. Keep full per-doc history (every HEAD change) rather than just tagged
  snapshots.
- Other.

### 13. Multi-org / multi-tenant model?

- **a. (Recommended)** Single logical tenant per deployment; multiple GitHub
  installations (orgs) coexist in one registry, separated only by authorization.
  Simplest, and fits the "one team, many repos" target. As an internal tool, the
  expectation is several GitHub orgs configured — each org's GitHub App creds
  are simply how the API pulls that org's repo content, which is transparent to
  end users.
- b. Hard multi-tenancy with a `tenant_id` on every table and per-tenant
  isolation — needed only if docz-api is offered as a shared/hosted service.
- c. One deployment per org, no cross-org concept at all.
- Other.

## Decisions

Resolved by user review on 2026-06-30. Recommendations accepted except where
noted (7 + 8 adopt Redis up front; 10 is an "Other").

| #   | Topic                 | Choice                                                         | Rationale / notes                                                                                                                              |
| --- | --------------------- | -------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | API path + versioning | (a) `/api/v1` + type-nested doc path                           | Matches the docz-site consumer (DESIGN-0009); flat alias optional                                                                              |
| 2   | docz library pinning  | (a) `replace` while prototyping → pinned tag before release    | Unblocks the slice without waiting on DESIGN-0007's tag                                                                                        |
| 3   | HTTP router           | (a) `chi`                                                      | Idiomatic `net/http`, no framework lock-in                                                                                                     |
| 4   | Postgres access       | (a) `sqlc` over `pgx`                                          | Compile-checked SQL, fits the small fixed schema + JSONB                                                                                       |
| 5   | Migrations            | (a) `goose`                                                    | Go-native, embeddable, runs `migrate up` on deploy                                                                                             |
| 6   | API style             | (a) REST/JSON                                                  | Shallow resource shape; simple, cacheable, enough for the site                                                                                 |
| 7   | Ingest execution      | (a) background worker, **Redis-backed queue now**              | Adopt the durable queue up front (not in-process-first) for clean horizontal scaling                                                           |
| 8   | Sessions              | (b) **Redis**                                                  | TTL eviction + O(1) lookup; shares the instance added for Decision 7                                                                           |
| 9   | Webhook idempotency   | (a) `X-GitHub-Delivery` dedup + reconcile vs `last_synced_sha` | Lean on GitHub redelivery; `content_hash` makes replays cheap                                                                                  |
| 10  | Authorization         | **(Other) authN only now; authZ later via SpiceDB**            | Ship authentication; a future middleware calls a SpiceDB-backed authZ service. Seam built now; all authenticated users currently see all repos |
| 11  | Search access         | (a) proxy through docz-api                                     | Single key holder + future authZ filter point; curl-friendly surface eases a later MCP search tool                                             |
| 12  | Versioning            | (a) HEAD-only now                                              | Later: consume each repo's `CHANGELOG.md` as the audit/versions source of truth and present it in the UI                                       |
| 13  | Tenancy               | (a) single logical tenant                                      | Multiple GitHub orgs/installations coexist; per-org GitHub App creds are just how the API pulls data — transparent to end users                |

## References

- **docz-api `DESIGN-0001`** — the service's own canonical, **Approved** copy of
  this design (in the `docz-api` repo). This doc is the docz-repo mirror kept in
  sync with it; the
  [Requirements](#requirements-for-the-docz-repo-what-docz-api-needs-from-docz)
  section here is the part that does not appear there.
- **INV-0005** — _docz-api and docz-site: centralized cross-repo docz registry
  and viewer._ The feasibility investigation and source of the eight locked
  decisions this design implements.
- **DESIGN-0007** — the shared docz parsing library (`pkg/doczcore/config` +
  `pkg/doczcore/document`) extracted from the docz CLI; the hard dependency
  docz-api imports so its registry never drifts from the CLI (Decision 7). The
  Requirements section above is, in effect, docz-api's acceptance criteria for
  it.
- **DESIGN-0009** — _docz-site_, the front-end consumer of this API (nav,
  search, client-side markdown rendering — Decision 3); the source of the
  consumed-endpoint shapes reconciled above.
- **DESIGN-0006 / IMPL-0012** — docz custom document types; why the registry
  must be type-agnostic and address types by name, `id_prefix`, or alias.
- **SpiceDB** — the relationship-based authorization service the deferred authZ
  middleware (Decision 10) will consult: <https://authzed.com/spicedb>.
- **GitHub Apps** — installation tokens, permissions, and webhook delivery:
  <https://docs.github.com/en/apps>.
- **GitHub Git Trees API** — recursive tree + blob fetch without a checkout
  (Decision 1): <https://docs.github.com/en/rest/git/trees>.
- **GitHub webhook signature verification** — `X-Hub-Signature-256` /
  HMAC-SHA256:
  <https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries>.
- **Meilisearch** — searchable/filterable/sortable attributes, facets, and
  tenant tokens: <https://www.meilisearch.com/docs>.
- **OpenID Connect (OIDC)** — authorization-code flow for the Okta/Keycloak
  providers: <https://openid.net/developers/how-connect-works/>.
