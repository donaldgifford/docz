---
id: DESIGN-0008
title: "docz-api: cross-repo docz registry and ingestion service"
status: Draft
author: Donald Gifford
created: 2026-06-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0008: docz-api: cross-repo docz registry and ingestion service

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
  - [GitHub App setup and onboarding](#github-app-setup-and-onboarding)
  - [Ingestion pipeline](#ingestion-pipeline)
  - [Auth and session model](#auth-and-session-model)
  - [Search](#search)
  - [Component / sequence overview](#component--sequence-overview)
- [API / Interface Changes](#api--interface-changes)
  - [HTTP / JSON endpoints](#http--json-endpoints)
  - [Service config surface (environment variables)](#service-config-surface-environment-variables)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. REST or GraphQL for the JSON API?](#1-rest-or-graphql-for-the-json-api)
  - [2. Synchronous ingest or a background worker / queue?](#2-synchronous-ingest-or-a-background-worker--queue)
  - [3. Where are sessions stored?](#3-where-are-sessions-stored)
  - [4. How is Okta/Keycloak group → repo authorization configured?](#4-how-is-oktakeycloak-group--repo-authorization-configured)
  - [5. Webhook retry / idempotency strategy?](#5-webhook-retry--idempotency-strategy)
  - [6. Add tag/release version snapshots now, or stay HEAD-only?](#6-add-tagrelease-version-snapshots-now-or-stay-head-only)
  - [7. Multi-org / multi-tenant model?](#7-multi-org--multi-tenant-model)
  - [8. Meilisearch API-key scoping for any direct site access?](#8-meilisearch-api-key-scoping-for-any-direct-site-access)
- [References](#references)
<!--toc:end-->

## Overview

**docz-api** is a small Go backend service that aggregates the documentation
of many repositories — all of which already use the **docz** CLI — into one
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
3. **Cache** the raw markdown plus parsed frontmatter/metadata — *not* rendered
   HTML. Rendering happens client-side in docz-site.
4. **Refresh** incrementally via webhooks (`push` / `release`) on the default
   branch, and **serve** a JSON API to docz-site: repos → types → docs, plus
   search and auth.

This design derives from **INV-0005** (the feasibility investigation that
concluded docz-api is buildable and meaningfully simpler than a general system
like rfc-api, because docz standardizes location, structure, and metadata at
the source). It honors the eight decisions locked there. Crucially, docz-api
does **not** re-implement docz's parser: it consumes the shared Go parsing
library extracted from the docz CLI in **DESIGN-0007** (`pkg/config` +
`pkg/document`), so the registry never drifts from the CLI's own notion of
"what docz docs exist in this tree."

> **Self-contained note.** This document is copied into the new `docz-api`
> repository to seed development, so the essential context is restated inline.
> Two upstream documents are load-bearing but not assumed open: **DESIGN-0007**
> defines the shared parsing library this service imports; **INV-0005** is the
> investigation whose locked decisions this design implements.

## Goals and Non-Goals

### Goals

- **Cross-repo registry.** A Postgres store keyed by `(repo, doc_id)` holding
  every docz document across every onboarded repo, with its frontmatter,
  cached raw markdown, and `git_sha`.
- **Type-agnostic ingestion.** The registry is driven entirely by each repo's
  `.docz.yaml`. Custom docz types (e.g. `frameworks` / `FW-0001`) ingest
  identically to the six built-ins. The six built-in type names are **never**
  hardcoded anywhere in the service.
- **GitHub App onboarding + incremental refresh.** Repos opt in by installing
  the app; `push` / `release` webhooks on the default branch drive diff-based
  partial re-ingest; `content_hash` gates redundant work.
- **Raw-markdown caching, not HTML.** The API serves raw markdown plus
  metadata (Decisions 2 + 3); docz-site renders to HTML client-side.
- **Pluggable site-user auth (Decision 6).** Authentication is provider-based:
  GitHub (default/preferred), Okta, and Keycloak — all three required. The
  GitHub provider mirrors GitHub repo read-access; Okta/Keycloak map OIDC
  group/role claims to repos via service config.
- **Full-text + faceted search.** A Meilisearch index over `title`/`body` with
  facets on `repo` / `type` / `status` / `author`.
- **A thin vertical slice first (Decision 8).** One hand-onboarded repo →
  ingest → serve one type, deferring full auth and webhooks, to prove the
  fetch → parse → upsert → serve loop end to end.

### Non-Goals

- **Rendering markdown to HTML.** No `rendered_html` column, no server-side
  renderer. Rendering is docz-site's job (Decision 3). The mdp renderer is
  intentionally scoped to single-user neovim/terminal use and is not a fit for
  a server-side multi-tenant renderer.
- **Non-default branches and PR/preview builds.** The default branch HEAD is
  the single current version (Decisions 4 + 5). Tag/release version snapshots
  are a possible later extension (Open Question 6).
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

docz produces structured docs *per repo*. Each repo has a root `.docz.yaml`
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

- **`.docz.yaml` is a manifest, not a haystack.** The service reads one file
  and learns exactly where docs live and what types exist — no discovery, and
  custom types are first-class because the type set is data, not code.
- **Frontmatter is typed and fixed-shape.** `id`, `title`, `status`, `author`,
  `created` map almost one-to-one onto a registry row and a search document —
  no metadata coercion.
- **IDs are stable and namespaced.** `RFC-0001`, `FW-0003` give a natural
  per-repo primary key and a clean cross-repo identity `<repo>/<doc_id>`.

The hard part — established in INV-0005 — is **not** docz-specific: a viewer
aggregating private-repo content must enforce that a site user sees only the
docs they are allowed to see. INV-0005 split that into **authentication** (who
is the user, pluggable) and **authorization** (what may they see, per-provider).
Note the asymmetry that drives the whole design: **ingestion is always via the
GitHub App** (that is how docz-api reads repo content), independent of how site
*users* authenticate.

The DESIGN-0007 shared library is what makes the "don't re-implement the
parser" decision concrete. It promotes the docz CLI's `internal/config`
(`Load`, `Validate`, type resolution) and `internal/document`
(`ScanDocuments`, `LoadFrontmatter`) into an importable `pkg/…` surface. Both
the CLI and docz-api depend on it, so the registry's view of a repo is
byte-for-byte the CLI's view.

## Detailed Design

### GitHub App setup and onboarding

docz-api is a GitHub App (not an OAuth app for ingestion — see the auth section
for the separate site-login concern). The app is installed on selected
repos/orgs and reads content using short-lived installation tokens.

**Required permissions (minimal):**

| Permission       | Level | Why |
|------------------|-------|-----|
| `contents`       | read  | Fetch `.docz.yaml` and doc files via the Git Trees/Contents API |
| `metadata`       | read  | List installed repos, read the default branch |

**Webhook events subscribed:**

| Event                       | Triggers |
|-----------------------------|----------|
| `installation`              | App installed/uninstalled → enumerate or purge repos |
| `installation_repositories` | Repos added/removed from an install → onboard/offboard |
| `push`                      | Commit to a branch → re-ingest if it is the default branch and touches `docs_dir` or `.docz.yaml` |
| `release`                   | Release published → reserved for tag/release snapshots (Open Question 6) |

**Installation-token auth flow.** docz-api authenticates to GitHub per request
chain as follows:

1. Sign a short-lived **app JWT** (RS256) with the App ID and the app private
   key (`iss` = app id, `exp` ≤ 10 min).
2. Exchange it for an **installation access token** via
   `POST /app/installations/{installation_id}/access_tokens`. The token is
   scoped to that installation's repos and expires in ~1 hour.
3. Cache the installation token per `installation_id` until just before
   expiry; refresh on demand.
4. Use the installation token as a `Bearer` credential on Git Trees / Contents
   API calls.

**Onboarding.** On `installation` / `installation_repositories`, docz-api
enumerates the installation's repos (`GET /installation/repositories`), and for
each repo checks for a root `.docz.yaml`. If present, it inserts an
`installations` row and a `repos` row and enqueues a full ingest. If absent,
the repo is recorded but marked unconfigured (no docz manifest) and skipped.

**Webhook HMAC verification.** Every inbound webhook is verified before any
work:

- The app is configured with a webhook secret.
- GitHub signs the raw request body with HMAC-SHA256 and sends
  `X-Hub-Signature-256: sha256=<hex>`.
- docz-api recomputes `HMAC-SHA256(secret, raw_body)` and compares with
  `hmac.Equal` (constant-time). A mismatch returns `401` and the payload is
  dropped. The `X-GitHub-Delivery` id is logged for idempotency (see Open
  Question 5).

### Ingestion pipeline

The pipeline is one logical flow with several triggers (initial onboard,
webhook refresh, manual re-sync). It is type-agnostic from end to end: the set
of types comes from the parsed `.docz.yaml`, never a constant.

```
 trigger (onboard | push | release | manual)
        │
        ▼
 ┌──────────────────┐   fetch via GitHub Git Trees API (recursive=1)
 │  Fetcher         │   → resolve default-branch HEAD sha
 │  (GitHub client) │   → list tree, pull .docz.yaml + docs blobs (no checkout)
 └────────┬─────────┘
          │ raw bytes: .docz.yaml + matched *.md
          ▼
 ┌──────────────────┐   pkg/config.Load(.docz.yaml) + pkg/config.Validate
 │  Parser          │   pkg/document.ScanDocuments per enabled type dir
 │  (DESIGN-0007)   │   → []DocEntry{frontmatter, content, path}
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

2. **Parse (DESIGN-0007 library).** `pkg/config.Load` + `Validate` produce the
   resolved `Config` (which already merges defaults and resolves type aliases /
   `id_prefix`). For each enabled type, `pkg/document.ScanDocuments(typeDir)`
   returns `[]DocEntry` with frontmatter and cached `Content`. No parsing logic
   lives in docz-api.

3. **content_hash gating.** For each doc, `content_hash = sha256(raw_md)`. If a
   stored row has the same `(repo_id, doc_id, content_hash)`, the doc is
   unchanged and is skipped for both Postgres and Meilisearch. This is the
   primary cost gate.

4. **Diff-based partial re-ingest on push.** A `push` webhook carries the list
   of added/modified/removed paths across its commits. docz-api intersects that
   list with `docs_dir` to decide *whether* to ingest, and to narrow blob
   fetches to changed files when possible (still validating against the full
   tree for deletions). Docs present in the registry but absent from the new
   HEAD tree are **deleted** from Postgres and Meilisearch.

5. **`.docz.yaml` change re-syncs the type set.** If the push touches
   `.docz.yaml`, docz-api re-parses it and reconciles `doc_types`: types added
   to the config are created, removed types (and their orphaned `documents`)
   are deleted, and changed `statuses` / `plural_label` / `dir` are updated.
   Because the type set is config-driven, a repo adding a custom `frameworks`
   type "just works" on the next push.

6. **Debounce.** Rapid successive pushes to the same repo (e.g. a squash-merge
   followed by a tag) are coalesced: an ingest job for a repo that already has
   a pending job is collapsed, and a per-repo debounce window
   (`INGEST_DEBOUNCE`, default a few seconds) batches bursts so the latest HEAD
   wins.

7. **Upsert + index transactionally on the Postgres side.** The `documents`
   upsert and `doc_types` reconcile run in one DB transaction; the Meilisearch
   update follows commit. If indexing fails after commit, the doc's
   `content_hash` is recorded so a reconcile job can re-index without a full
   re-ingest (eventual consistency; the search index trails Postgres briefly).

### Auth and session model

Two independent identity concerns, kept strictly separate (INV-0005 Decision 6):

- **Ingestion identity** is **always** the GitHub App installation token,
  regardless of how site users log in. Nothing in this section changes that.
- **Site-user identity** is **pluggable** across three providers, behind one
  abstraction.

**Provider abstraction.** A single Go interface keeps the three providers
configurable rather than forked:

```go
// Provider authenticates a site user (the "who") and reports which onboarded
// repos that user may read (the "what"). Both halves are provider-specific.
type Provider interface {
    Name() string                       // "github" | "okta" | "keycloak"
    AuthCodeURL(state string) string    // begin OAuth/OIDC
    Exchange(ctx context.Context, code string) (*Identity, error)
    // AuthorizedRepos returns the repo ids (or "*") this identity may read.
    AuthorizedRepos(ctx context.Context, id *Identity) ([]int64, error)
}

type Identity struct {
    Provider string
    Subject  string   // stable per-provider user id
    Email    string
    Login    string   // github login, when present
    Groups   []string // OIDC group/role claims (okta/keycloak)
    Token    string   // provider access token, for live repo checks
}
```

**OIDC / OAuth flow.** Standard authorization-code flow on three endpoints
(see API section): `/auth/login?provider=…` redirects to the provider's
`AuthCodeURL` with a signed `state`; the provider redirects back to
`/auth/callback`; docz-api calls `Exchange`, then `AuthorizedRepos`, issues a
session, and sets a session cookie. GitHub uses OAuth; Okta and Keycloak use
standard OIDC discovery (`issuer`, `client_id`, `client_secret`, scopes).

**Per-provider authorization** (the half that differs):

- **GitHub provider (default/preferred).** Mirror GitHub repo read-access. A
  user sees an onboarded repo only if their GitHub token can read it. docz-api
  checks `GET /repos/{owner}/{name}` (or the user's installation repos) per
  session and caches the allowed-repo set for the session TTL. This is the
  tightest, most natural mapping.
- **Okta / Keycloak providers.** There is no GitHub repo-permission signal, so
  authorization comes from **OIDC group/role claims** mapped to repos or
  repo-groups in service config — e.g. `group:platform → [repos…]`. For
  internal deployments a coarse mode is allowed: *any authenticated member sees
  all onboarded repos*. The mapping is service config, not code (see Open
  Question 4).

**Enforcement on read endpoints.** Every read endpoint resolves the session's
allowed-repo set and filters results to it. List endpoints return only
authorized repos/docs; a doc fetch for an unauthorized repo returns `404` (not
`403`, to avoid leaking existence). Search queries are constrained with a
`repo IN (allowed…)` Meilisearch filter injected server-side so the index can
never leak across the authorization boundary. In the thin vertical slice
(Decision 8) auth is stubbed to a single "all repos visible" provider; the
enforcement seam exists from day one so turning on real providers is additive.

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

Indexing is keyed off the same content_hash gate as Postgres: unchanged docs
are not re-indexed; deleted docs are removed by primary key. Search results
return enough metadata (repo, type, doc_id, title, status, snippet) for
docz-site to render a result list and link to the full doc fetch.

### Component / sequence overview

```
                          ┌───────────────────────────────────────────┐
   GitHub  ── webhook ───▶ │                docz-api (Go)               │
   (App)   ◀─ Trees API ── │                                           │
                          │  ┌──────────┐   ┌──────────┐  ┌─────────┐  │
                          │  │ Webhook  │──▶│ Ingest   │─▶│ Search  │──┼─▶ Meilisearch
                          │  │ handler  │   │ pipeline │  │ indexer │  │
                          │  └──────────┘   └────┬─────┘  └─────────┘  │
                          │                      │                     │
                          │                      ▼                     │
                          │                 ┌─────────┐                │
                          │                 │ Postgres│◀──── reads ────┤
                          │                 │ registry│                │
   docz-site ── HTTP ────▶ │  ┌──────────┐  └─────────┘                │
   (DESIGN-0009)         │  │ JSON API │──── auth (Provider) ──────────┼─▶ GitHub/Okta/Keycloak
                          │  └──────────┘                              │
                          └───────────────────────────────────────────┘
```

**Ingest sequence (push on default branch):**

```
GitHub        Webhook        Ingest          Parser(pkg)    Postgres   Meili
  │  push       │              │                  │            │         │
  ├────────────▶│ verify HMAC  │                  │            │         │
  │             ├─ default br? touches docs_dir? ─┤            │         │
  │             ├─────────────▶│ debounce/coalesce│            │         │
  │             │              ├─ fetch tree+blobs│            │         │
  │             │              ├─────────────────▶│ Load/Scan  │         │
  │             │              │◀── []DocEntry ───┤            │         │
  │             │              ├─ content_hash diff ──────────▶│ upsert  │
  │             │              │                  │            ├─ commit │
  │             │              ├─ index changed ──┼────────────┼────────▶│
  │             │◀─ 202 ───────┤                  │            │         │
```

## API / Interface Changes

This is a greenfield service, so "interface changes" means the initial HTTP/JSON
surface plus the config surface. All responses are JSON; all read endpoints are
authorization-filtered to the session's allowed repos.

### HTTP / JSON endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET`  | `/healthz` | Liveness/readiness (DB + Meilisearch reachable) |
| `GET`  | `/api/repos` | List onboarded repos the session may read |
| `GET`  | `/api/repos/{owner}/{name}` | Repo detail (config snapshot, last_synced_sha) |
| `GET`  | `/api/repos/{owner}/{name}/types` | List doc types for a repo |
| `GET`  | `/api/repos/{owner}/{name}/types/{type}/docs` | List docs of a type |
| `GET`  | `/api/repos/{owner}/{name}/docs/{doc_id}` | Get one doc: raw markdown + metadata |
| `GET`  | `/api/search?q=&repo=&type=&status=&author=` | Faceted full-text search |
| `GET`  | `/auth/login?provider={github\|okta\|keycloak}` | Begin OAuth/OIDC |
| `GET`  | `/auth/callback` | OAuth/OIDC redirect target; issues session |
| `POST` | `/auth/logout` | Invalidate session |
| `POST` | `/webhooks/github` | GitHub App webhook receiver (HMAC-verified) |

Notes:

- `{type}` accepts the type's canonical name **or** its `id_prefix` / alias,
  resolved by the same DESIGN-0007 `Config.resolveType` logic the CLI uses, so
  `…/types/frameworks/docs` and `…/types/FW/docs` are equivalent.
- `{doc_id}` is the frontmatter id (`RFC-0001`), case-sensitive, matching the
  CLI's convention.
- The doc fetch returns **raw markdown** in `raw_md`; docz-site renders it.

**Example — get a document:**

```http
GET /api/repos/acme/platform/docs/RFC-0001 HTTP/1.1
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
GET /api/search?q=logging&type=rfc&status=Accepted HTTP/1.1
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
    "repo":   { "acme/platform": 2 },
    "type":   { "rfc": 2 },
    "status": { "Accepted": 1, "Draft": 1 }
  }
}
```

### Service config surface (environment variables)

```env
# Postgres
DATABASE_URL=postgres://docz:secret@db:5432/docz_api?sslmode=require

# Meilisearch
MEILI_HOST=http://meili:7700
MEILI_API_KEY=…                # admin/index key; site uses a scoped key (see OQ 8)

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

# Okta (OIDC)
OKTA_ISSUER=https://acme.okta.com/oauth2/default
OKTA_CLIENT_ID=…
OKTA_CLIENT_SECRET=…
OKTA_GROUP_REPO_MAP=/etc/docz-api/okta-groups.yaml  # group -> repos[] mapping

# Keycloak (OIDC)
KEYCLOAK_ISSUER=https://kc.acme.com/realms/acme
KEYCLOAK_CLIENT_ID=…
KEYCLOAK_CLIENT_SECRET=…
KEYCLOAK_GROUP_REPO_MAP=/etc/docz-api/keycloak-groups.yaml

# Ingestion tuning
INGEST_DEBOUNCE=5s
```

The group→repo mapping files (referenced for Okta/Keycloak) hold the
authorization config; their exact shape is Open Question 4.

## Data Model

The Postgres schema refines the INV-0005 sketch. All timestamps are
`timestamptz`. JSONB columns hold the config snapshot and per-type `statuses`
exactly as parsed, so the registry can answer "what types/statuses did this
repo declare" without a second source of truth.

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

-- Site users (one row per provider identity that has logged in).
CREATE TABLE users (
    id           BIGSERIAL PRIMARY KEY,
    provider     TEXT NOT NULL,                     -- 'github' | 'okta' | 'keycloak'
    subject      TEXT NOT NULL,                     -- stable per-provider id
    email        TEXT,
    login        TEXT,                              -- github login when present
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, subject)
);

-- Server-side sessions (default storage; see Open Question 3 for alternatives).
CREATE TABLE sessions (
    id              TEXT PRIMARY KEY,               -- opaque session id (cookie value)
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        TEXT   NOT NULL,
    allowed_repos   JSONB  NOT NULL,                -- cached repo ids, or "*"
    groups          JSONB  NOT NULL DEFAULT '[]',   -- OIDC group/role claims
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_idx    ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);
```

Per-provider authorization config (the group→repo mapping for Okta/Keycloak) is
deliberately **not** a table in the thin slice — it is env-driven / file-driven
service config (the `*_GROUP_REPO_MAP` files above), so operators can manage it
with the rest of their deployment config. Promoting it to a table (e.g.
`repo_groups`) is reasonable later; it is folded into Open Question 4.

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
  Include a **custom type** fixture (`frameworks` / `FW-0001`) to prove no
  built-in is hardcoded, and a doc *missing* frontmatter to prove it is skipped
  without aborting the repo.

- **Integration — Postgres + Meilisearch + webhook handlers.** Spin Postgres
  and Meilisearch with **testcontainers-go** (or an equivalent harness). Cover:
  full ingest of a fixture tree; a second ingest with one changed doc (only the
  changed row re-written, content_hash gate proven); a `.docz.yaml` change that
  adds/removes a type (reconcile path); a doc deletion (row + index entry
  removed). Drive these through the webhook handler with synthetic `push`
  payloads so the trigger path is exercised, not just the inner pipeline.

- **Webhook signature tests.** Table-driven HMAC-SHA256 cases: a correct
  `X-Hub-Signature-256` passes; a wrong secret, a tampered body, and a missing
  header all return `401` and perform no DB writes. Assert constant-time
  comparison is used (no early-exit on first byte).

- **End-to-end onboarding.** A fixture repo (committed under `testdata/`, served
  through a recorded/replayed GitHub client) is onboarded start to finish:
  `installation` event → enumerate → detect `.docz.yaml` → full ingest → assert
  `/api/repos`, `/api/repos/.../types`, and a doc fetch return the expected
  shapes. The GitHub Trees/Contents calls are replayed from recorded fixtures so
  the test is hermetic.

- **Auth enforcement.** With a stub `Provider` returning a fixed allowed-repo
  set, assert list endpoints filter to authorized repos, an unauthorized doc
  fetch returns `404`, and the search filter injects `repo IN (allowed…)` so an
  out-of-scope doc never appears in results.

- **Golden / fixture discipline.** Reuse the docz convention: fixture trees and
  expected JSON under `testdata/`, regenerated with an `-update` flag, never
  hand-edited.

## Migration / Rollout Plan

docz-api is a **greenfield repository**; this document is its seed design. There
is no in-place migration of an existing system — "rollout" is the order in which
the service is built and shipped.

1. **Tagged docz library dependency (DESIGN-0007).** docz-api's `go.mod`
   depends on a **tagged release** of the docz module exposing `pkg/config` +
   `pkg/document`. This is a hard prerequisite: the slice cannot parse a repo
   without it. The dependency is version-pinned so a docz CLI change cannot
   silently alter ingestion; bumps are deliberate.

2. **Thin vertical slice first (Decision 8).** Ship the smallest end-to-end
   loop: one **hand-onboarded** repo (its `installation_id` / `owner` / `name`
   seeded directly, no GitHub App install UX) → fetch via Trees API → parse →
   upsert Postgres → serve `/api/repos`, one `/types`, and a doc fetch. **Auth
   is stubbed** to a single "all repos visible" provider and **webhooks are
   deferred** (re-ingest is a manual endpoint or CLI subcommand). This proves
   fetch → parse → upsert → serve before any of the harder subsystems.

3. **DB migrations.** Schema is managed by a migration tool (e.g. `goose` or
   `golang-migrate`) checked into the repo; `migrate up` runs on deploy.
   Migrations are forward-only and additive where possible so a binary rollback
   does not require a destructive down-migration.

4. **Layer in the rest.** In order: real GitHub App onboarding +
   installation-token flow; the webhook receiver (HMAC + `push`/`release`); the
   content_hash-gated diff/debounce pipeline; the Meilisearch indexer; then the
   pluggable auth providers (GitHub first, then Okta/Keycloak) with real
   per-provider authorization enforcement.

5. **Container / deploy.** A single Go binary in a minimal container image,
   plus Postgres and Meilisearch (managed services or sidecar containers). The
   service is stateless except for its DB and search index, so it scales
   horizontally behind a load balancer; webhook delivery to any replica is fine
   because ingest jobs reconcile against the stored `last_synced_sha`. Secrets
   (app private key, webhook secret, OIDC client secrets, Meilisearch key) come
   from the platform's secret store via the env vars above.

6. **Then docz-site (DESIGN-0009)** consumes the JSON API. The site is built
   against the slice's endpoints from the start so the contract is exercised
   early.

## Open Questions

### 1. REST or GraphQL for the JSON API?

- **a. (Recommended)** Plain REST/JSON as specified above. The resource shape
  (repos → types → docs + search) is shallow and well-bounded; REST keeps the
  server simple, is trivially cacheable, and is enough for docz-site's needs.
- b. GraphQL — one flexible endpoint lets docz-site shape exactly the data it
  needs and avoids over-fetching across the nav tree.
- c. REST now, add a GraphQL gateway later only if the site's query patterns
  prove awkward.
- Other.

### 2. Synchronous ingest or a background worker / queue?

- **a. (Recommended)** Background worker. Webhooks enqueue an ingest job and
  return `202` immediately; a worker (in-process queue first, durable queue
  later) does fetch/parse/upsert/index. Keeps webhook handling fast and lets
  debounce/coalesce live in one place. Also where a future shallow-clone path
  for very large repos would land.
- b. Fully synchronous ingest inside the webhook handler — simplest for the
  thin slice, but risks GitHub webhook timeouts on large repos.
- c. External queue (e.g. a Postgres-backed job table, NATS, or SQS) from day
  one for durability and retry.
- Other.

### 3. Where are sessions stored?

- **a. (Recommended)** Postgres `sessions` table (as schematized). One store to
  operate, supports server-side revocation, and the cached allowed-repo set
  lives next to it.
- b. Redis — faster session reads and natural TTL eviction, at the cost of a
  second datastore.
- c. Stateless signed JWT cookies — no session store, but revocation and
  refreshing the cached allowed-repo set become awkward.
- Other.

### 4. How is Okta/Keycloak group → repo authorization configured?

- **a. (Recommended)** A service-config mapping file per provider
  (`*_GROUP_REPO_MAP`) of `group → [repos…]`, hot-reloadable, plus a coarse
  "any authenticated member sees all" toggle for internal deployments.
- b. Database-backed `repo_groups` tables managed through an admin API — more
  operable at scale, more to build.
- c. Encode the mapping directly in OIDC claims (a custom claim listing repo
  slugs) so the provider owns it entirely.
- Other.

### 5. Webhook retry / idempotency strategy?

- **a. (Recommended)** Idempotency on `X-GitHub-Delivery`: record processed
  delivery ids and reconcile ingest against `last_synced_sha`, so a replayed or
  duplicate delivery is a no-op. Rely on GitHub's own redelivery for transient
  failures.
- b. A durable job table with explicit retry/backoff and a dead-letter for
  poison deliveries.
- c. At-least-once with no dedup, accepting that the content_hash gate makes
  re-ingest cheap and mostly harmless.
- Other.

### 6. Add tag/release version snapshots now, or stay HEAD-only?

- **a. (Recommended)** Stay HEAD-only for now (honors Decision 4): the default
  branch HEAD is the single current version, `git_sha` is stored per doc, and
  the `release` webhook is wired but only logged. Add snapshots later behind a
  schema extension.
- b. Add a `doc_versions` table now and snapshot on `release`, paying the
  storage/complexity cost up front.
- c. Keep full per-doc history (every HEAD change) rather than just tagged
  snapshots.
- Other.

### 7. Multi-org / multi-tenant model?

- **a. (Recommended)** Single logical tenant per deployment; multiple GitHub
  installations (orgs) coexist in one registry, separated only by
  authorization. Simplest, and fits the "one team, many repos" target.
- b. Hard multi-tenancy with a `tenant_id` on every table and per-tenant
  isolation — needed only if docz-api is offered as a shared/hosted service.
- c. One deployment per org, no cross-org concept at all.
- Other.

### 8. Meilisearch API-key scoping for any direct site access?

- **a. (Recommended)** docz-site never talks to Meilisearch directly; all
  search goes through docz-api with a server-side `repo IN (allowed…)` filter,
  and only the API holds the Meilisearch admin/index key. No key reaches the
  browser.
- b. Issue Meilisearch **tenant tokens** (scoped, short-lived, with an embedded
  filter) so docz-site can query Meilisearch directly for lower latency, at the
  cost of trusting the embedded filter.
- c. A separate read-only Meilisearch key with no embedded filter, relying on
  the API to never expose it — weakest option.
- Other.

## References

- **INV-0005** — *docz-api and docz-site: centralized cross-repo docz registry
  and viewer.* The feasibility investigation and source of the eight locked
  decisions this design implements.
- **DESIGN-0007** — the shared docz parsing library (`pkg/config` +
  `pkg/document`) extracted from the docz CLI; the hard dependency docz-api
  imports so its registry never drifts from the CLI (Decision 7).
- **DESIGN-0009** — *docz-site*, the front-end consumer of this API (nav,
  search, client-side markdown rendering — Decision 3).
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
- docz custom document types (registry must be type-agnostic, driven by each
  repo's `.docz.yaml`) — DESIGN-0006 / IMPL-0012.
