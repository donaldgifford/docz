---
id: IMPL-0001
title: "docz-api service implementation"
status: Draft
author: Donald Gifford
created: 2026-06-30
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0001: docz-api service implementation

**Status:** Draft **Author:** Donald Gifford **Date:** 2026-06-30

<!--toc:start-->

- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 0: Foundations — config, dependencies, local infra](#phase-0-foundations--config-dependencies-local-infra)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 1: Persistence — Postgres schema, migrations, store](#phase-1-persistence--postgres-schema-migrations-store)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 2: Thin vertical slice — fetch → parse → upsert → serve](#phase-2-thin-vertical-slice--fetch--parse--upsert--serve)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 3: Search — Meilisearch indexer + search endpoint](#phase-3-search--meilisearch-indexer--search-endpoint)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 4: Async ingestion — Redis queue, worker, debounce](#phase-4-async-ingestion--redis-queue-worker-debounce)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 5: GitHub App onboarding + webhooks](#phase-5-github-app-onboarding--webhooks)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 6: Authentication — pluggable providers + Redis sessions](#phase-6-authentication--pluggable-providers--redis-sessions)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
  - [Phase 7: Hardening, deploy, contract, CI](#phase-7-hardening-deploy-contract-ci)
    - [Tasks](#tasks-7)
    - [Success Criteria](#success-criteria-7)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. Redis job-queue library?](#1-redis-job-queue-library)
  - [2. GitHub API / App client library?](#2-github-api--app-client-library)
  - [3. How does the Phase 2 slice authenticate to GitHub?](#3-how-does-the-phase-2-slice-authenticate-to-github)
  - [4. Manual onboard / re-sync mechanism?](#4-manual-onboard--re-sync-mechanism)
  - [5. Environment config loader?](#5-environment-config-loader)
  - [6. OIDC client library for Okta/Keycloak?](#6-oidc-client-library-for-oktakeycloak)
  - [7. Integration test harness?](#7-integration-test-harness)
  - [8. Observability scope for v1?](#8-observability-scope-for-v1)
  - [9. Local dev/test infrastructure?](#9-local-devtest-infrastructure)
  - [10. release webhook handling now?](#10-release-webhook-handling-now)
- [References](#references)
<!--toc:end-->

## Objective

Build **docz-api**, the Go backend service designed in DESIGN-0001: a GitHub App
that onboards docz repos, ingests each repo's `.docz.yaml` + documents into a
Postgres registry and a Meilisearch index, refreshes via webhooks, and serves a
versioned JSON API (`/api/v1`) to docz-site. This plan sequences that build into
eight phases, each independently shippable and verifiable, following the
thin-vertical-slice-first decision (DESIGN-0001 Decision 8) and the locked
decisions table.

**Implements:** DESIGN-0001 (with the consumer contract from DESIGN-0009 and the
parsing-library contract from DESIGN-0007 / its R1–R9 requirements).

## Scope

### In Scope

- The full docz-api service per DESIGN-0001: GitHub App ingestion, Postgres
  registry, Redis queue + sessions, Meilisearch search, the `/api/v1` JSON API,
  webhook-driven incremental refresh, and pluggable **authentication** (GitHub,
  Okta, Keycloak).
- The `authorize` middleware **seam** (currently a pass-through returning "all
  repos") that the future authorization layer plugs into.
- Migrations, integration/e2e tests, the container image, and CI.

### Out of Scope

- **Authorization** (which repos a user may read) — the future SpiceDB-backed
  middleware (Decision 10). Only the seam is built here.
- **Per-doc version history + `CHANGELOG.md` _parsing_ / audit store**
  (Decision 12) — the `release` webhook is wired but only logged. The raw
  `CHANGELOG.md` _is_ cached on ingest (OQ 10) so the future feature has its
  source, but nothing parses or serves it yet.
- **docz-site** (DESIGN-0009) — a separate repo/consumer.
- **The docz parsing library itself** (DESIGN-0007) — it lives in the `docz`
  repo and **shipped as docz `v0.5.0`**. This plan _consumes_ it by pinning that
  tag; its R1–R7 surface is verified present, so cutting/maintaining the tag
  stays a `docz`-repo task.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met. Phases 0–2 deliver the thin
vertical slice (Decision 8); Phases 3–6 layer in the remaining subsystems; Phase
7 hardens and ships.

---

### Phase 0: Foundations — config, dependencies, local infra

Establish the buildable skeleton every later phase depends on: typed
configuration, the docz parsing-library dependency, the core third-party
dependencies, a runnable `main()`, and local infra (Postgres + Redis +
Meilisearch) for development and tests.

#### Tasks

- [x] Pin the docz parsing library in `go.mod`:
      `require github.com/donaldgifford/docz v0.5.0` (DESIGN-0007 shipped; no
      `replace` needed — Decision 2 / OQ 2). Add a smoke test that imports
      `pkg/doczcore/config` + `pkg/doczcore/document`, parses a fixture, and
      resolves a type by name / `id_prefix` / alias via `ValidateType`. _(done —
      `internal/doczcontract` guards the R1–R5 surface: `config.Load` +
      `Validate` + `EnabledTypes` + `TypeDir` + `ValidateType`
      name/alias/id_prefix resolution, `document.ParseFrontmatter`,
      `ScanDocuments` → `DocEntry.Content`, and `IsDoczFile`.)_
- [x] Align the Go toolchain: `go.mod` and `mise.toml` both on **go 1.26.4**
      (docz `v0.5.0`'s minimum). _(done — `mise.toml` bumped)_
- [x] Add core dependencies: HTTP router (`chi`, Decision 3), Postgres driver
      (`pgx`, Decision 4), Redis client + `hibiken/asynq` (OQ 1), Meilisearch
      client. Note `spf13/viper` arrives transitively via `pkg/doczcore/config`.
      _(done — pinned `go-chi/chi/v5 v5.3.0`, `jackc/pgx/v5 v5.10.0`,
      `redis/go-redis/v9 v9.21.0`, `hibiken/asynq v0.26.0`,
      `meilisearch-go v0.36.3`; staged `// indirect`, promoted to direct as each
      package imports them in Phases 1–4.)_
- [ ] Implement `internal/config`: a typed struct loaded from the environment
      (every var in DESIGN-0001's config surface) with validation and clear
      errors for missing/invalid required values, using **`spf13/viper`** —
      already pulled in transitively by `pkg/doczcore/config`, so no new
      top-level dependency (OQ 5).
- [ ] Expand `cmd/docz-api/main.go`: flag parsing, `slog` handler selection
      (text/json + level via env), config load, dependency wiring, an HTTP
      server with graceful shutdown, and `--version` using the existing
      `version`/`commit`/`date` ldflags vars.
- [ ] Add a `compose.yaml` (or equivalent) that brings up Postgres, Redis, and
      Meilisearch for local dev (OQ 9).
- [ ] Confirm `just build`, `just test`, `just lint`, and `just fmt` are green
      on the skeleton.

#### Success Criteria

- `just build` produces `build/bin/docz-api`; `docz-api --version` prints the
  injected version/commit/date.
- `internal/config` loads from env, rejects an invalid/missing required var with
  an actionable error, and is unit-tested.
- The `pkg/doczcore` import smoke test passes against the pinned `v0.5.0` tag
  (proves the R1–R5 surface — parse, type resolution, `DocEntry.Content` — works
  end to end).
- `docker compose up` (or chosen tool) starts Postgres + Redis + Meilisearch
  locally; `just lint` and `just test` pass.

---

### Phase 1: Persistence — Postgres schema, migrations, store

Stand up the registry's durable store: the schema from DESIGN-0001's Data Model,
forward-only migrations, and a typed access layer with the transactional
upsert/reconcile operation ingestion needs.

#### Tasks

- [ ] Write `goose` migrations (Decision 5) for `installations`, `repos`,
      `doc_types`, `documents`, `users`, `webhook_deliveries`, plus the
      `documents_repo_type_idx` / `documents_status_idx` indexes, and nullable
      `changelog_md` + `changelog_sha` columns on `repos` for the cached
      `CHANGELOG.md` (OQ 10). Forward-only, additive where possible.
- [ ] Embed migrations and run `migrate up` on startup; expose an explicit
      `migrate` path for CI/ops.
- [ ] Configure `sqlc` (Decision 4) + write the queries; generate the typed Go
      access code.
- [ ] Implement `internal/store`: CRUD over the tables plus the
      **transactional** `documents` upsert + `doc_types` reconcile (one tx), and
      the delete-absent-from-HEAD operation, modeling JSONB columns
      (`config_snapshot`, `statuses`, `aliases`).
- [ ] Implement `/readyz` reporting Postgres reachability (readiness probe; OQ
      8).
- [ ] Integration tests with the chosen harness (OQ 7): migrations apply
      cleanly; CRUD round-trips; the transactional upsert/reconcile and delete
      paths behave correctly under a simulated changed/new/deleted set.

#### Success Criteria

- `migrate up` (and down) applies the full schema cleanly against a fresh
  Postgres; startup auto-migrates.
- `internal/store` integration tests pass against a real Postgres
  (testcontainers or chosen harness), including the one-transaction
  upsert+reconcile and the delete-absent path.
- `/readyz` returns healthy only when Postgres is reachable.

---

### Phase 2: Thin vertical slice — fetch → parse → upsert → serve

The Decision-8 proof loop: one **hand-onboarded** repo, fetched over the GitHub
Trees API, parsed via `pkg/doczcore`, upserted to Postgres, and served by the
read endpoints. GitHub is read over the **App installation-token flow, built
here** (OQ 3b) — the App is the mechanism for reading across many repos, so
there is no throwaway PAT path. **Authorization is stubbed** (the `authorize`
seam returns "all repos") and **webhooks/queue are deferred** (ingest is
synchronous, triggered manually).

#### Tasks

- [ ] Implement `internal/githubapp`: the App JWT → installation-token flow
      (`google/go-github` + `bradleyfalzon/ghinstallation`, OQ 2 / 3b) **and**
      the fetch surface — resolve default-branch HEAD, pull the recursive tree,
      filter to `.docz.yaml` + blobs under `docs_dir/<type.dir>/` (using
      `doczcfg` + `doczdoc.IsDoczFile`), and fetch blobs (base64-decoded).
      Authenticate every fetch with an installation token.
- [ ] Implement `internal/ingest` core (synchronous): fetch → `doczcfg.Load` +
      `Validate` → `doczdoc.ParseFrontmatter` per blob → `content_hash` gate →
      map to rows → `store` upsert/reconcile. Type set comes only from
      `.docz.yaml` (no hardcoded built-ins).
- [ ] If a root `CHANGELOG.md` exists, fetch and **cache it raw** onto the
      `repos` row (`content_hash`-gated, no parsing), per OQ 10 — available for
      the future versions/audit UI.
- [ ] Implement the manual onboard / re-sync trigger (OQ 4) to seed an
      `installations` + `repos` row and run an ingest for one repo.
- [ ] Implement `internal/httpapi` with `chi`: the read endpoints
      `/api/v1/repos`, `/api/v1/repos/{owner}/{name}`, `…/types`,
      `…/types/{type}/docs`, and `…/types/{type}/docs/{doc_id}`. Resolve
      `{type}` by name / `id_prefix` / alias via `doczcfg` (R4).
- [ ] Add the `authorize` middleware seam (returns the full onboarded-repo set
      for now) and apply it to every read endpoint.
- [ ] Tests: unit parse→row mapping incl. a **custom type** fixture
      (`frameworks`/`FW-0001`, addressable by name/prefix/alias) and a
      **missing-frontmatter** doc (skipped, repo not aborted); e2e onboarding of
      a `testdata/` fixture repo through a replayed/recorded GitHub client
      asserting the read-endpoint shapes.

#### Success Criteria

- A fixture repo is hand-onboarded end to end; the five read endpoints return
  the shapes documented in DESIGN-0001.
- A custom type is addressable via `…/types/frameworks/docs`, `…/types/FW/docs`,
  and any declared alias — equivalently.
- A second ingest of unchanged content is a no-op for Postgres (the
  `content_hash` gate is proven by test); a changed doc rewrites exactly its
  row; a removed doc is deleted.
- The e2e onboarding test is hermetic (recorded GitHub fixtures) and green.

---

### Phase 3: Search — Meilisearch indexer + search endpoint

Index every document and expose faceted full-text search through the API
(Decision 11: all search proxied through docz-api, never direct from a browser).

#### Tasks

- [ ] Implement `internal/search`: configure the `documents` index (searchable
      `title`/`body` with `title` boosted; filterable `repo`/`type`/
      `status`/`author`; sortable `created`/`updated_at`; composite primary key
      `"<repo_id>:<doc_id>"`).
- [ ] Add/replace/delete index documents keyed off the same `content_hash` gate
      as Postgres; remove deleted docs by primary key. Hook this into
      `internal/ingest` after the Postgres commit.
- [ ] Implement `GET /api/v1/search` (q + `repo`/`type`/`status`/`author`
      facets), returning hits + facet counts; inject the `repo IN (allowed…)`
      filter via the `authorize` seam (pass-through for now).
- [ ] Extend `/readyz` to report Meilisearch reachability.
- [ ] Integration tests (Meilisearch via the chosen harness): index population,
      facet counts, snippet/highlight, deletion removes from the index, and the
      filter-injection seam.

#### Success Criteria

- After an ingest, `GET /api/v1/search?q=…` returns hits with facet counts and
  highlighted snippets matching DESIGN-0001's example shape.
- A deleted document disappears from search results; an unchanged document is
  not re-indexed (gate proven).
- The endpoint is usable directly from `curl` (the surface a future MCP search
  tool would reuse).

---

### Phase 4: Async ingestion — Redis queue, worker, debounce

Decouple ingestion from its triggers: triggers enqueue Redis jobs and return
fast; a worker pool drains them; per-repo debounce coalesces bursts (Decision
7).

#### Tasks

- [ ] Implement `internal/queue` over Redis (`hibiken/asynq`, OQ 1): an enqueue
      API and a worker pool that runs the Phase-2 ingest pipeline.
- [ ] Move ingest invocation behind the queue; the manual trigger (OQ 4) and
      (later) webhooks enqueue jobs rather than running inline.
- [ ] Implement per-repo debounce/coalesce (`INGEST_DEBOUNCE`) so a repo with a
      pending job collapses duplicates and the latest HEAD wins.
- [ ] Add at-least-once delivery + retry semantics; rely on the `content_hash`
      gate to keep re-runs cheap and safe.
- [ ] (Optional) Cache GitHub installation tokens in Redis keyed by
      `installation_id` so replicas share one token.
- [ ] Graceful shutdown: stop accepting, drain in-flight jobs, close cleanly.
- [ ] Tests: enqueue → worker drains; bursts coalesce to a single latest-HEAD
      run; a re-delivered job is idempotent; shutdown drains without loss.

#### Success Criteria

- Triggers return promptly (`202`-style) and a worker performs the ingest
  asynchronously.
- Rapid repeated triggers for one repo coalesce to a single ingest at the latest
  HEAD (debounce proven by test).
- Re-running a job is idempotent (no duplicate rows / no double-index); shutdown
  drains in-flight work.

---

### Phase 5: GitHub App onboarding + webhooks

Build on the App installation-token auth from Phase 2 (OQ 3b): add
install-driven onboarding and HMAC-verified webhooks that drive incremental
refresh, replacing the slice's manual onboarding trigger.

#### Tasks

- [ ] Reuse the App auth flow in `internal/githubapp` (built in Phase 2 per OQ
      3b): the app JWT (RS256) → installation-token exchange via
      `POST /app/installations/{id}/access_tokens`, cached per `installation_id`
      until just before expiry — now driving onboarding + webhook ingest, not
      just the slice.
- [ ] Onboarding: handle `installation` / `installation_repositories` —
      enumerate installation repos, detect root `.docz.yaml`, insert
      `installations`/`repos`, enqueue full ingest; mark repos without a
      manifest unconfigured.
- [ ] Implement `internal/webhook`: HMAC-SHA256 verification with `hmac.Equal`
      (constant-time); reject mismatch with `401` and no work; route events.
- [ ] `push` handling: default-branch + `docs_dir`/`.docz.yaml` filter;
      diff-based partial re-ingest (narrow blob fetches; delete docs absent from
      new HEAD); `.docz.yaml` change → `doc_types` reconcile (add/remove/update
      types).
- [ ] `release` handling: wired but **log-only** for the versions feature (OQ 10
      / Decision 12); `push` / `release` also refresh the cached `CHANGELOG.md`
      on the `repos` row when it changes.
- [ ] Idempotency: record `X-GitHub-Delivery` in `webhook_deliveries`; a
      duplicate delivery is a no-op; reconcile against `last_synced_sha`.
- [ ] Wire webhook events to enqueue ingest jobs (Phase 4 queue).
- [ ] Tests: table-driven HMAC (correct passes; wrong secret / tampered body /
      missing header → `401`, no DB writes; constant-time asserted); synthetic
      `push` payloads exercise reconcile + delete; a replayed delivery is a
      no-op.

#### Success Criteria

- Installing the app onboards its repos (enumerate → detect `.docz.yaml` →
  ingest); uninstall/removal offboards them.
- A `push` to the default branch incrementally refreshes only changed docs;
  adding/removing a type in `.docz.yaml` reconciles `doc_types`; a deleted doc
  is removed from Postgres and the index.
- Bad webhook signatures are rejected with `401` and zero writes; a replayed
  `X-GitHub-Delivery` performs no duplicate work.

---

### Phase 6: Authentication — pluggable providers + Redis sessions

Add site-user **authentication** behind one provider abstraction (GitHub
default, Okta, Keycloak) with Redis-backed sessions. Authorization stays a
pass-through seam (Decision 10).

#### Tasks

- [ ] Implement `internal/auth`: the `Provider` interface (`Name` /
      `AuthCodeURL` / `Exchange`) and the GitHub OAuth provider (default).
- [ ] Implement the Okta and Keycloak OIDC providers via discovery
      (`issuer`/`client_id`/`client_secret`/scopes), using `coreos/go-oidc` +
      `golang.org/x/oauth2` (OQ 6).
- [ ] Implement `internal/session`: Redis session store (`sess:<id>` →
      identity + groups + expiry, `SESSION_TTL`), issue/lookup/revoke; set an
      httpOnly, SameSite cookie.
- [ ] Auth endpoints: `/auth/login?provider=…`, `/auth/callback` (exchange →
      upsert `users` row → issue session), `GET /api/v1/auth/session`,
      `POST /api/v1/auth/logout` (single `DEL`).
- [ ] Session middleware resolves the session into request context; the
      `authorize` seam still returns "all onboarded repos" (authZ deferred), but
      now keyed off a real identity. Protected endpoints return `401` without a
      valid session.
- [ ] Tests: a provider stub drives login/callback → session issued; session
      lookup populates identity; logout revokes; unauthenticated requests to
      protected endpoints → `401`; `Groups` claims persisted for the future
      authZ layer.

#### Success Criteria

- GitHub login works end to end; Okta and Keycloak work via OIDC config.
- A session cookie is issued on callback, validated on subsequent requests, and
  revoked on logout; `/api/v1/auth/session` reflects the current user.
- Protected endpoints require a session (`401` otherwise); the `authorize` seam
  is the single, isolated switch point where the future SpiceDB resolver plugs
  in.

---

### Phase 7: Hardening, deploy, contract, CI

Pin the real dependency, lock the consumer contract, add observability, and get
the service release-ready.

#### Tasks

- [ ] Confirm the docz dependency stays a **pinned published tag**
      (`require github.com/donaldgifford/docz v0.5.0`, no `replace`), and bump
      it deliberately if a newer docz ships (R6; DESIGN-0007 already published
      v0.5.0).
- [ ] Add contract golden fixtures matching the response shapes DESIGN-0009
      consumes, asserted in CI so a breaking JSON change fails here first.
- [ ] Observability (OQ 8 — full stack): request-logging middleware over `slog`;
      a Prometheus `/metrics` endpoint; a `/healthz` liveness endpoint and the
      `/readyz` readiness probe extended to cover Redis (Postgres + Meilisearch
      already wired in Phases 1/3); and full OpenTelemetry tracing across the
      ingest, HTTP, and worker paths.
- [ ] Container: confirm the distroless `Dockerfile` builds the service; provide
      deploy manifests/compose wiring Postgres + Redis + Meilisearch; secrets
      via env/secret store.
- [ ] Audit error messages for consistency and wrapping (`%w`); resolve
      TODO/FIXME comments.
- [ ] Ensure `make ci` / `just lint` + `just test` pass; review coverage
      (target >80%).

#### Success Criteria

- `make ci` (or `just lint` + `just test`) passes with zero errors;
  coverage >80% across packages.
- The container image runs the service against a compose/managed Postgres +
  Redis
  - Meilisearch; `/readyz` reports all three reachable and `/metrics` is
    scrapeable.
- Contract golden fixtures pass; the docz dependency is a pinned published tag
  (`v0.5.0` or a deliberately-bumped later release), with no `replace`
  directive.

---

## File Changes

| File / package               | Action | Description                                                                                                                                                                                                   |
| ---------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `go.mod` / `go.sum`          | Modify | Pin docz `v0.5.0` (no `replace`; brings `viper` transitively), chi, pgx, sqlc, goose, redis + `asynq`, meilisearch, `go-github` + `ghinstallation`, `go-oidc` + `oauth2`, OTel + Prometheus client; go 1.26.4 |
| `cmd/docz-api/main.go`       | Modify | Flags, slog config, config load, dependency wiring, server + graceful shutdown                                                                                                                                |
| `internal/config/`           | Create | Typed env configuration + validation                                                                                                                                                                          |
| `internal/store/`            | Create | sqlc-generated access + transactional upsert/reconcile; embedded goose migrations                                                                                                                             |
| `internal/store/migrations/` | Create | goose SQL migrations for the DESIGN-0001 schema                                                                                                                                                               |
| `internal/githubapp/`        | Create | App JWT, installation tokens, Trees/Blobs/Contents fetch                                                                                                                                                      |
| `internal/ingest/`           | Create | fetch → parse (`pkg/doczcore`) → content_hash gate → upsert/reconcile                                                                                                                                         |
| `internal/queue/`            | Create | Redis-backed ingest job queue + worker pool + debounce                                                                                                                                                        |
| `internal/webhook/`          | Create | HMAC verification + event routing (`push`/`release`/install) + idempotency                                                                                                                                    |
| `internal/search/`           | Create | Meilisearch index management + query                                                                                                                                                                          |
| `internal/auth/`             | Create | Provider interface + github/okta/keycloak authN                                                                                                                                                               |
| `internal/session/`          | Create | Redis session store (issue/lookup/revoke)                                                                                                                                                                     |
| `internal/httpapi/`          | Create | chi router, handlers, session + authorize middleware                                                                                                                                                          |
| `compose.yaml`               | Create | Local Postgres + Redis + Meilisearch (OQ 9)                                                                                                                                                                   |
| `testdata/`                  | Create | Fixture repo trees + recorded GitHub responses + golden JSON                                                                                                                                                  |

## Testing Plan

- [ ] Unit tests for parse→row mapping, type resolution (name/prefix/alias),
      `content_hash` stability, and config loading (table-driven where
      applicable).
- [ ] Integration tests (Postgres + Redis + Meilisearch via OQ 7) for the store,
      queue, search, and webhook-driven ingest paths.
- [ ] Webhook HMAC table-driven tests (pass / wrong secret / tampered / missing
      → `401`, no writes; constant-time comparison).
- [ ] Hermetic e2e onboarding test using recorded GitHub fixtures.
- [ ] Auth tests with a provider stub: login/callback/session/logout and `401`
      on protected endpoints.
- [ ] Contract golden fixtures matching DESIGN-0009's consumed shapes.
- [ ] Golden/fixture discipline: an `-update` flag regenerates fixtures; never
      hand-edited.

## Dependencies

- **docz `v0.5.0` (`docz` repo)** — the `pkg/doczcore/config` +
  `pkg/doczcore/document` library (its R1–R7 requirements). **Published and
  verified**, so it is a plain pinned `require` (no `replace`) from Phase 0
  onward. It transitively brings `spf13/viper` (+ ~12 indirect modules) and
  requires go 1.26.4.
- **Infrastructure** — Postgres, Redis, and Meilisearch (local via compose;
  managed/sidecar in deploy).
- **Third-party Go libraries** (now chosen in Open Questions): `chi`, `pgx` +
  `sqlc`, `goose`, `redis` + `hibiken/asynq` (OQ 1), the Meilisearch client,
  `google/go-github` + `bradleyfalzon/ghinstallation` (OQ 2), `coreos/go-oidc` +
  `golang.org/x/oauth2` (OQ 6), and the OpenTelemetry SDK + Prometheus client
  (OQ 8). Service config reuses the transitively-present `viper` (OQ 5).
- **Existing repo tooling** — `mise`, `just`, `golangci-lint`, `goreleaser`, the
  distroless `Dockerfile`, and the Forgejo/GitHub CI workflows (already
  present).

## Open Questions

Each question is numbered; option `a` is the recommendation, later letters are
alternatives, and **Other** is free-form for review. These are implementation
choices not already fixed by DESIGN-0001's Decisions table.

### 1. Redis job-queue library?

> **Resolved 2026-07-02.** Chose **(a) `hibiken/asynq`** — the mature
> Redis-backed queue (retries, scheduling, dead-letter, inspector) wired in
> Phase 4.

DESIGN-0001 picks Redis as the queue substrate but leaves the library open
("e.g. `asynq` or Redis streams").

- **a. (Chosen)** `hibiken/asynq` — a mature Redis-backed task queue with
  retries, scheduling, dead-letter, and a built-in inspector; minimal code to a
  robust worker.
- b. Hand-rolled over **Redis Streams** (`XADD` / consumer groups) — no extra
  dependency, full control, but you build retry/backoff/visibility yourself.
- c. `riverqueue/river` — excellent ergonomics, but it is **Postgres**-backed,
  which diverges from the Redis-queue decision (would mean no Redis for the
  queue).
- Other.

### 2. GitHub API / App client library?

> **Resolved 2026-07-02.** Chose **(a) `google/go-github` +
> `bradleyfalzon/ghinstallation`** — the REST surface plus App/installation auth
> with token caching, least custom crypto.

Needed for Trees/Blobs/Contents fetch and the App JWT → installation-token flow.

- **a. (Chosen)** `google/go-github` for the REST surface +
  `bradleyfalzon/ghinstallation` for App/installation auth — well-maintained,
  handles token caching/refresh, least custom crypto.
- b. A hand-rolled minimal `net/http` client for just the few endpoints used
  (trees, blobs, repo, installation tokens) — fewer deps, less surface, more
  code to maintain and test.
- c. `go-github` with a custom `http.RoundTripper` for auth (no
  `ghinstallation`) — one fewer dep, but you re-implement JWT/installation-token
  handling.
- Other.

### 3. How does the Phase 2 slice authenticate to GitHub?

> **Resolved 2026-07-02.** Chose **(b)**: build the GitHub App JWT →
> installation-token flow up front (in Phase 2) so the slice reads over real
> installation tokens from day one. The App is the intended mechanism for
> reading across many repos, so there is no value in a throwaway PAT path —
> Phases 2 and 5 share one auth implementation. This pulls the App-auth task
> from Phase 5 into Phase 2; Phase 5 then adds only install-driven onboarding
> and webhooks on top of it.

The full App flow lands in Phase 5, but the slice (Phase 2) needs to fetch a
repo first.

- a. A configurable token (a fine-grained PAT scoped to the one hand-onboarded
  repo) for the slice, swapped for installation tokens in Phase 5. Keeps the
  slice tiny and unblocks the proof loop.
- **b. (Chosen)** Build the App JWT → installation-token flow first (reorder
  Phase 5 auth before the slice) so the slice already uses real installation
  tokens.
- c. Use a stored/replayed HTTP fixture only (no live GitHub) for the slice and
  defer all real GitHub auth to Phase 5.
- Other.

### 4. Manual onboard / re-sync mechanism?

> **Resolved 2026-07-02.** Chose **(a)**, structured so **(b)** layers on for
> free. Implement onboard/re-sync as one internal service method, then expose it
> first as a `docz-api` CLI subcommand; the future admin/management HTTP API is
> a second thin adapter over the _same_ method — not a rewrite. Rejected
> **(c)**: migrations should carry schema, not operational/onboarding state, and
> coupling ingest-on-startup to a migration makes per-repo re-sync awkward and
> non-repeatable. Idempotency comes from the `content_hash` gate in the ingest
> path, not from where it is triggered.

The slice (and ongoing ops) needs a way to onboard/re-ingest a repo without the
webhook path.

- **a. (Chosen)** A `docz-api` CLI subcommand (e.g.
  `docz-api ingest --repo owner/name`) that seeds the rows and runs an ingest —
  doubles as a manual re-sync / operator tool later, and the future admin API
  wraps the same internal method.
- b. An authenticated admin HTTP endpoint (`POST /api/v1/admin/ingest`) — usable
  remotely, but adds an admin-auth surface early. _(Planned as the later second
  adapter over the same method.)_
- c. A seed migration / SQL fixture that inserts the `installations` + `repos`
  rows, with ingest triggered on startup.
- Other.

### 5. Environment config loader?

> **Resolved 2026-07-01.** Chose **(d) `spf13/viper`**: it is already pulled
> into the module graph transitively by `pkg/doczcore/config` (docz `v0.5.0`),
> so using it for service config adds **no new top-level dependency**. Its extra
> weight is already paid; a separate env-only lib would just be a second config
> mechanism.

- a. `caarlos0/env` — struct-tag env binding, defaults, required fields; tiny
  and explicit, no config-file machinery.
- b. `sethvargo/go-envconfig` — context-aware, mutators, no global state;
  similar weight.
- c. Standard-library `os.Getenv` + a hand-written loader — zero deps, more
  boilerplate.
- **d. (Chosen)** `spf13/viper` — files + env + flags; heavier than an env-only
  surface needs, but **already present transitively via `pkg/doczcore/config`**,
  so it is a zero-cost reuse.
- Other.

### 6. OIDC client library for Okta/Keycloak?

> **Resolved 2026-07-02.** Chose **(a) `coreos/go-oidc` +
> `golang.org/x/oauth2`** — the de-facto OIDC discovery + token-verification
> pair, wired in Phase 6.

- **a. (Chosen)** `coreos/go-oidc` + `golang.org/x/oauth2` — the de-facto
  standard for OIDC discovery + token verification; pairs naturally with the
  GitHub OAuth path.
- b. `zitadel/oidc` — a fuller-featured OIDC toolkit (client + server), heavier
  than a pure client need.
- c. Hand-rolled OIDC discovery + JWKS verification — no dep, but
  security-sensitive code to own.
- Other.

### 7. Integration test harness?

> **Resolved 2026-07-02.** Chose **(a) `testcontainers-go`** — hermetic,
> programmatic Postgres/Redis/Meilisearch containers per run (pairs with OQ 9a).

DESIGN-0001 names testcontainers-go; confirming for the build.

- **a. (Chosen)** `testcontainers-go` — programmatic Postgres/Redis/ Meilisearch
  containers per test run, hermetic and CI-friendly (matches the design).
- b. `ory/dockertest` — similar capability, lighter API, slightly less
  ecosystem.
- c. A long-lived `compose.yaml` the tests connect to (gated by a build tag) —
  fastest local loop, but shared mutable state and more CI wiring.
- Other.

### 8. Observability scope for v1?

> **Resolved 2026-07-02.** Chose **the full stack (an expanded (c))**:
> structured `slog` everywhere + a request-logging middleware, a Prometheus
> `/metrics` endpoint, `/healthz` (liveness) and `/readyz` (Postgres + Redis +
> Meilisearch readiness) endpoints, and **full OpenTelemetry tracing**. `slog`
> and the readiness probes stand up as the subsystems they cover land (Phases
> 0–3); `/metrics`, `/healthz`, and tracing are finalized in Phase 7.

- a. Structured `slog` everywhere + a request-logging middleware
  - a Prometheus `/metrics` endpoint (cheap, high value). Defer distributed
    tracing.
- b. `slog` logs only for v1; add metrics/tracing when an operational need
  appears.
- c. Full OpenTelemetry (logs + metrics + traces) from the start — most
  observable, most upfront wiring and deps.
- **Other (Chosen).** Full stack: `slog` + request-logging middleware,
  Prometheus `/metrics`, `/healthz` + `/readyz`, and full OTel tracing.

### 9. Local dev/test infrastructure?

> **Resolved 2026-07-02.** Chose **(a)**: a `compose.yaml` for local dev plus
> `testcontainers-go` for hermetic CI (pairs with OQ 7a).

- **a. (Chosen)** A `compose.yaml` for local dev **and** `testcontainers-go` for
  hermetic CI tests — best of both (pairs with OQ 7a).
- b. `compose.yaml` only; tests connect to it behind a build tag (no
  testcontainers).
- c. `testcontainers-go` only; no committed compose file for manual runs.
- Other.

### 10. `release` webhook handling now?

> **Resolved 2026-07-02.** Chose **(a) plus the caching half of (b)**: the
> `release` event stays **log-only** for the versions/audit feature
> (Decision 12) — no parsing, no audit store yet — **but** ingest now also
> fetches and **caches the repo's root `CHANGELOG.md` raw** (if present),
> `content_hash`- gated like docs and stored on the `repos` row, so the future
> versions/audit UI has the source ready without a re-crawl. Refreshed on the
> same `push` / `release` events that drive doc ingest.

- **a. (Chosen, log-only half)** Subscribe to `release` and **log only** for now
  (Decision 12); no `CHANGELOG.md` _parsing_ in this IMPL. Keeps the door open
  without building the versions/audit feature yet.
- **b. (Chosen, caching half)** Also fetch each repo's `CHANGELOG.md` — but only
  **cache it raw** now (no parsing, no audit/versions store), so it is available
  when that feature is designed.
- c. Do not subscribe to `release` at all until the versions feature is
  designed.
- Other.

## References

- **DESIGN-0001** — _docz-api: cross-repo docz registry and ingestion service_
  (Approved). The design this plan implements; its Decisions table fixes the
  stack (chi, sqlc, goose, Redis, REST, authN-only, etc.).
- **DESIGN-0007 / docz `v0.5.0`** — the docz parsing library
  (`pkg/doczcore/config` + `pkg/doczcore/document`) this service imports;
  published as docz `v0.5.0` with the R1–R7 surface verified present, pinned in
  `go.mod`.
- **DESIGN-0009** — _docz-site_, the consumer whose endpoint shapes the contract
  golden fixtures lock.
- **INV-0005** — the originating investigation and its eight locked decisions.
- **DESIGN-0006 / IMPL-0012 (docz)** — custom document types; the type-agnostic,
  name/`id_prefix`/alias resolution the registry preserves.
- **Tooling** — `goose`, `sqlc`, `chi`, `pgx`, Meilisearch, `testcontainers-go`,
  and the GitHub Apps / Git Trees / webhook-signature docs referenced in
  DESIGN-0001.
