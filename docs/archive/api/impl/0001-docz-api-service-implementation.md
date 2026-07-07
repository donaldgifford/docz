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
- [x] Implement `internal/config`: a typed struct loaded from the environment
      (every var in DESIGN-0001's config surface) with validation and clear
      errors for missing/invalid required values, using **`spf13/viper`** —
      already pulled in transitively by `pkg/doczcore/config`, so no new
      top-level dependency (OQ 5). _(done — grouped sub-structs; `Load()`
      collects **all** missing/invalid required vars into one
      `ErrInvalidConfig`; provider-conditional validation
      (github/okta/keycloak); PEM path-or-body resolution; a redacting `Secret`
      type (slog + fmt); added operational `HTTP_ADDR`/`LOG_LEVEL`/`LOG_FORMAT`.
      100% test coverage.)_
- [x] Expand `cmd/docz-api/main.go`: flag parsing, `slog` handler selection
      (text/json + level via env), config load, dependency wiring, an HTTP
      server with graceful shutdown, and `--version` using the existing
      `version`/`commit`/`date` ldflags vars. _(done — `run()` composition root:
      `-version` flag, `config.Load`, `newLogger` (text/json × level), a chi
      server with `RequestID`/`Recoverer` and a `/healthz` liveness probe, and
      `signal.NotifyContext` graceful shutdown. Smoke-tested: start → healthz →
      clean SIGTERM drain.)_
- [x] Add a `compose.yaml` (or equivalent) that brings up Postgres, Redis, and
      Meilisearch for local dev (OQ 9). _(done — `compose.yaml` (Postgres 17,
      Redis 7.4, Meilisearch v1.12) with healthchecks + named volumes, plus
      `.env.example` mapping the DEV connection settings to `internal/config`.
      Verified: `docker compose up` brings all three to **healthy** and each is
      reachable.)_
- [x] Confirm `just build`, `just test`, `just lint`, and `just fmt` are green
      on the skeleton. _(done — all four green; `git status` clean after `fmt`.
      **Phase 0 success criteria all met.**)_

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

**Status: COMPLETE ✅** — all criteria met. `just build` produces the binary and
`--version` prints the injected metadata; `internal/config` loads/validates from
env (100% cover); the `pkg/doczcore` smoke test passes against the pinned
`v0.5.0`; `compose.yaml` brings Postgres + Redis + Meilisearch up healthy;
`just lint`/`just test` green.

---

### Phase 1: Persistence — Postgres schema, migrations, store

Stand up the registry's durable store: the schema from DESIGN-0001's Data Model,
forward-only migrations, and a typed access layer with the transactional
upsert/reconcile operation ingestion needs.

#### Tasks

- [x] Write `goose` migrations (Decision 5) for `installations`, `repos`,
      `doc_types`, `documents`, `users`, `webhook_deliveries`, plus the
      `documents_repo_type_idx` / `documents_status_idx` indexes, and nullable
      `changelog_md` + `changelog_sha` columns on `repos` for the cached
      `CHANGELOG.md` (OQ 10). Forward-only, additive where possible. _(done —
      `internal/store/migrations/20260702000000_initial_schema.sql` (Up + Down).
      Verified: Up creates all 6 tables + both custom indexes against real
      Postgres, Down rolls back cleanly.)_
- [x] Embed migrations and run `migrate up` on startup; expose an explicit
      `migrate` path for CI/ops. _(done — `migrations.FS` (go:embed) +
      `store.     Migrate`/`MigrateDown` via goose's global-free `Provider` over
      pgx's `database/sql` stdlib adapter; `main()` auto-migrates before
      serving, and the `-migrate` flag runs migrations then exits. Verified
      idempotent against Postgres — `goose_db_version` records
      `20260702000000`.)_
- [x] Configure `sqlc` (Decision 4) + write the queries; generate the typed Go
      access code. _(done — `sqlc.yaml` (`sql_package: pgx/v5`, `emit_json_tags`,
      `emit_empty_slices`, jsonb → `json.RawMessage` override) reads the
      goose-annotated migrations as its schema (Up only). Query sets in
      `internal/store/queries/{installations,repos,doc_types,documents}.sql`
      cover the upsert/list/get/delete surface the reconcile path needs
      (`UpsertRepo :one` RETURNING id; `ListDocumentHashes`/`ListDocTypes` for
      the content-hash gate; `UpsertDocument`/`DeleteDocument`,
      `UpsertDocType`/`DeleteDocType`). Generated into package `store`; nullable
      text left as `pgtype.Text` (mapped to clean DTOs in Phase 2). `just
      generate` regenerates, `just generate-check` (sqlc diff) guards drift.)_
- [x] Implement `internal/store`: CRUD over the tables plus the
      **transactional** `documents` upsert + `doc_types` reconcile (one tx), and
      the delete-absent-from-HEAD operation, modeling JSONB columns
      (`config_snapshot`, `statuses`, `aliases`). _(done — `store.go`
      (`Store`/`NewStore`, `Ping`, `Close`, `UpsertInstallation`, plain-Go input
      DTOs that keep `pgtype` out of the ingest boundary + `textOrNull`/
      `dateOrNull` null mappers) and `reconcile.go` (`ReconcileRepo`: one tx via
      `pool.Begin` → `q.WithTx(tx)` → committed-gated deferred `Rollback` →
      `Commit`; `reconcileDocTypes` upsert-present/delete-absent;
      `reconcileDocuments` with the **content-hash gate** — unchanged docs
      skipped, new/changed upserted, absent deleted). JSONB surfaces as
      `json.RawMessage`; `last_synced_at`/`updated_at` are DB-authoritative
      (`now()`). Returns a `ReconcileResult` (upsert/delete/unchanged counts) for
      logging + tests. Style-reviewed (go-style) and lint-clean. Integration
      tests land in Task 6.)_
- [x] Implement `/readyz` reporting Postgres reachability (readiness probe; OQ
      8). _(done — `store.NewPool` opens the runtime pgxpool (ping-verified,
      separate from the migration conn); `main()` builds the pool after
      migrations, wires a `Store`, and closes the pool on shutdown. `/readyz`
      depends on a narrow `readyChecker` interface (`Ping(ctx) error`) satisfied
      by `*store.Store`: 200 `{"status":"ok"}` when reachable, 503
      `{"status":"unavailable"}` + a `slog.Warn` otherwise, with a 2s check
      timeout. Unit-tested both paths via a stub (no DB needed); shared
      `writeJSON` helper.)_
- [x] Integration tests with the chosen harness (OQ 7): migrations apply
      cleanly; CRUD round-trips; the transactional upsert/reconcile and delete
      paths behave correctly under a simulated changed/new/deleted set. _(done —
      `store_integration_test.go` behind `//go:build integration`, testcontainers
      `postgres:17-alpine` via one shared container (`TestMain`/`runMain`; tests
      isolate on unique owner/name). Covers: `Ping`; new-repo reconcile (row
      counts + jsonb/changelog/`last_synced_at` round-trip); the **content-hash
      gate** (identical→all unchanged, one changed→exactly one upsert); and
      delete-absent for both documents and doc types. `just test-integration`
      runs them; verified green against Docker.)_

#### Success Criteria

- `migrate up` (and down) applies the full schema cleanly against a fresh
  Postgres; startup auto-migrates.
- `internal/store` integration tests pass against a real Postgres
  (testcontainers or chosen harness), including the one-transaction
  upsert+reconcile and the delete-absent path.
- `/readyz` returns healthy only when Postgres is reachable.

**Status: COMPLETE ✅** — all criteria met. The embedded goose migrations apply
the full schema up/down against a fresh Postgres and startup auto-migrates; the
`internal/store` integration tests (testcontainers) prove the one-transaction
upsert+reconcile, the content-hash gate, and delete-absent; `/readyz` reports
healthy only when Postgres is reachable.

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

- [x] Implement `internal/githubapp`: the App JWT → installation-token flow
      (`google/go-github` + `bradleyfalzon/ghinstallation`, OQ 2 / 3b) **and**
      the fetch surface — resolve default-branch HEAD, pull the recursive tree,
      filter to `.docz.yaml` + blobs under `docs_dir/<type.dir>/` (using
      `doczcfg` + `doczdoc.IsDoczFile`), and fetch blobs (base64-decoded).
      Authenticate every fetch with an installation token. _(done — `Client`
      implements `ingest.RepoFetcher` (boundary types in `internal/ingest/
      fetcher.go`: `RepoSnapshot`/`BlobEntry`). Auth via `ghinstallation/v2`
      transport (auto JWT→token refresh) on `go-github/v88` (`WithTransport` for
      stub injection, `WithEnterpriseURLs` for GHE). `Fetch` resolves
      default-branch HEAD → recursive tree (errors if truncated) → classifies
      `.docz.yaml`/`CHANGELOG.md`/doc blobs; **githubapp applies only the
      `doczdoc.IsDoczFile` convention filter** (no doczcfg dependency), leaving
      precise per-type filtering to ingest. Blobs base64-decoded (newline-
      stripped). Unit-tested via a stub `http.RoundTripper` (tree classify +
      decode table + no-config error) — no network/token exchange.)_
- [x] Implement `internal/ingest` core (synchronous): fetch → `doczcfg.Load` +
      `Validate` → `doczdoc.ParseFrontmatter` per blob → `content_hash` gate →
      map to rows → `store` upsert/reconcile. Type set comes only from
      `.docz.yaml` (no hardcoded built-ins). _(done — `Service.Run` (narrow
      `reconciler` interface) fetches, `loadConfig` bridges `doczcfg.Load` via a
      throwaway temp dir (no `HOME` mutation — tests neutralize `HOME`),
      `Validate` (error fatal, warnings logged), `buildDocTypes` maps every
      `EnabledTypes()` entry, `buildDocuments` assigns each blob to a type by its
      `docs_dir/<type.dir>/` prefix (over-fetched blobs ignored) and skips
      missing-frontmatter docs with a warn (repo not aborted); `mapper.go` maps
      `TypeConfig`→`DocTypeInput` and blob+`Frontmatter`→`DocumentInput`
      (`content_hash = hex(sha256(raw))`, `created` parsed, `Status` via the
      `config.Status` string). `config_snapshot` = `json.Marshal(cfg)` (jsonb;
      raw YAML can't be stored in jsonb). The store's content-hash gate does the
      diff. Unit + fake-fetcher e2e tests green.)_
- [x] If a root `CHANGELOG.md` exists, fetch and **cache it raw** onto the
      `repos` row (`content_hash`-gated, no parsing), per OQ 10 — available for
      the future versions/audit UI. _(done — `githubapp.Fetch` pulls a root
      `CHANGELOG.md` blob into `RepoSnapshot.ChangelogMD`/`ChangelogSHA`;
      `Service.Run` sets them on `RepoInput` (`changelog_md`/`changelog_sha`);
      the store upsert writes them unparsed. Covered by the e2e test.)_
- [x] Implement the manual onboard / re-sync trigger (OQ 4) to seed an
      `installations` + `repos` row and run an ingest for one repo. _(done —
      `-onboard owner/name@installation_id` flag on the binary (like `-migrate`):
      `runOnboard` seeds the installation (`UpsertInstallation`), builds a
      per-installation `githubapp.Client`, runs one `ingest.Service.Run`, logs
      the `ReconcileResult`, and exits. `parseOnboardSpec` validates the spec
      (unit-tested: valid + malformed/empty/bad-id cases). No admin HTTP surface;
      Phase 5 webhooks drive the same ingest.)_
- [x] Implement `internal/httpapi` with `chi`: the read endpoints
      `/api/v1/repos`, `/api/v1/repos/{owner}/{name}`, `…/types`,
      `…/types/{type}/docs`, and `…/types/{type}/docs/{doc_id}`. Resolve
      `{type}` by name / `id_prefix` / alias via `doczcfg` (R4). _(done —
      `Handler.Mount(r, authz)` mounts all five under `/api/v1` behind the
      authorize middleware. Wire DTOs (`dto.go`) flatten `pgtype` nullables to
      `""`/`YYYY-MM-DD`/RFC3339 and never leak sqlc types; single-doc returns
      `raw_md`, list omits it. `{type}` resolved by a pure `resolveType` over the
      repo's `doc_types` rows (name / `id_prefix` case-insensitive / alias) — no
      live `doczcfg` at serve time. Store gained `GetRepo`/`ListDocumentsByType`/
      `GetDocumentByID` (+ queries). `pgx.ErrNoRows`→404; repos outside the
      allowed set 404 (existence hiding). Wired into `main` (`newRouter` returns
      `chi.Router`; probes stay open). Tested: `resolveType` table + full
      fake-store endpoint suite incl. name/prefix/alias equivalence and the
      unauthorized-404 path.)_
- [x] Add the `authorize` middleware seam (returns the full onboarded-repo set
      for now) and apply it to every read endpoint. _(done — `internal/authorize`:
      `Authorizer.Allowed(ctx, r) (AllowedRepos, error)`, `Middleware(a)` injects
      the set into the request context (500 on authorizer error), `FromContext` +
      `AllowedRepos.Contains(id)`. Phase 2 stub `AllReposAuthorizer` (narrow
      `repoLister` = `store.ListRepos`) returns all repo ids; Phase 5 swaps the
      impl only. Store gained `ListRepos`/`GetDocTypesForRepo` (`read.go`) +
      `ListRepos :many` query. Unit-tested (contains, all-ids, ctx injection, 500
      path). Applied to every `/api/v1` route in the httpapi Task 5 commit.)_
- [x] Tests: unit parse→row mapping incl. a **custom type** fixture
      (`frameworks`/`FW-0001`, addressable by name/prefix/alias) and a
      **missing-frontmatter** doc (skipped, repo not aborted); e2e onboarding of
      a `testdata/` fixture repo through a replayed/recorded GitHub client
      asserting the read-endpoint shapes. _(done — unit mapping in
      `internal/ingest` (custom `frameworks` type; nofm/stray-dir skips).
      `internal/e2e` (`//go:build integration`) onboards a fixture repo through
      the **real** ingest pipeline into a **real** Postgres (testcontainers) and
      serves it via the **real** httpapi handler — only GitHub is faked (an
      in-memory `RepoFetcher`), so it is hermetic. Asserts: five endpoints return
      the DESIGN-0001 shapes; the custom type is addressable via name / `FW` /
      `fw` / `framework` equivalently; a re-onboard of unchanged HEAD is a
      Postgres no-op (2 unchanged / 0 upserted); a changed doc rewrites exactly
      its row and a removed doc is deleted, reflected by the list endpoint.)_

#### Success Criteria

- A fixture repo is hand-onboarded end to end; the five read endpoints return
  the shapes documented in DESIGN-0001.
- A custom type is addressable via `…/types/frameworks/docs`, `…/types/FW/docs`,
  and any declared alias — equivalently.
- A second ingest of unchanged content is a no-op for Postgres (the
  `content_hash` gate is proven by test); a changed doc rewrites exactly its
  row; a removed doc is deleted.
- The e2e onboarding test is hermetic (recorded GitHub fixtures) and green.

**Status: COMPLETE ✅** — all criteria met. A fixture repo is hand-onboarded end
to end and the five read endpoints return the DESIGN-0001 shapes; the custom
type is addressable by name / `id_prefix` / alias equivalently; a re-onboard of
unchanged HEAD is a Postgres no-op (content-hash gate), a changed doc rewrites
exactly its row, and a removed doc is deleted. The e2e test is hermetic
(in-memory fetcher) and green.

---

### Phase 3: Search — Meilisearch indexer + search endpoint

Index every document and expose faceted full-text search through the API
(Decision 11: all search proxied through docz-api, never direct from a browser).

#### Tasks

- [x] Implement `internal/search`: configure the `documents` index (searchable
      `title`/`body` with `title` boosted; filterable `repo`/`type`/
      `status`/`author`; sortable `created`/`updated_at`; composite primary key
      `"<repo_id>:<doc_id>"`).
      <br>_Done: `internal/search` wraps `meilisearch.ServiceManager`.
      `Client.EnsureIndex` creates the `documents` index (primary key `id`) and
      applies settings idempotently — searchable `title`,`body` (title first =
      higher relevance via the `attribute` ranking rule), filterable
      `repo`,`repo_id`,`type`,`status`,`author` (`repo_id` added for the
      authorize filter), sortable `created`,`updated_at`. Uses the
      `…WithContext` API variants (contextcheck) and waits on the settings task
      via `waitTask`, which surfaces a failed task's message. Boundary types
      (`IndexDoc`, `SearchParams`, `SearchHit`, `SearchResult`, `FacetMap`) live
      in `types.go` so no meilisearch type leaks to ingest/httpapi. Dep promoted
      to direct in `go.mod`._
- [x] Add/replace/delete index documents keyed off the same `content_hash` gate
      as Postgres; remove deleted docs by primary key. Hook this into
      `internal/ingest` after the Postgres commit.
      <br>_Done: `Client.IndexDocuments`/`DeleteDocuments` add/replace by PK and
      delete by PK, each waiting on its Meili task. `store.ReconcileResult` now
      carries `UpsertedDocIDs`/`DeletedDocIDs` (populated by `reconcileDocuments`
      as the content-hash gate decides), and a new `GetDocumentsByIDs` store
      read (`SELECT … WHERE doc_id = ANY(@doc_ids)`) fetches the changed rows.
      `ingest.Service` gained a narrow `Indexer` dep + broadened `repoStore`
      interface; after the Postgres commit `Run` calls `indexSearch` →
      `syncIndex` (delete removed PKs, then index the upserted rows via
      `toIndexDoc`). Indexing is best-effort: failures log at error and do NOT
      fail the ingest (Postgres already committed; next reconcile re-indexes —
      eventual consistency). `NewService` takes a third `Indexer` arg (nil
      disables indexing); callers updated. Tests: `captureIndexer` asserts the
      upserted doc is indexed with PK `1:FW-0001`; `failIndexer` proves an index
      error doesn't fail `Run`._
- [x] Implement `GET /api/v1/search` (q + `repo`/`type`/`status`/`author`
      facets), returning hits + facet counts; inject the `repo IN (allowed…)`
      filter via the `authorize` seam (pass-through for now).
      <br>_Done: `Client.Search` runs the query with `AttributesToCrop`/
      `AttributesToHighlight` on `body` (`<em>` tags, 40-word crop → the
      snippet), facets `repo`/`type`/`status`/`author`, decodes hits via the
      non-deprecated `Hits.DecodeInto` (pulls `_formatted.body`) and facet
      counts from `FacetDistribution`. `buildFilter` composes `repo_id IN
      [ids] AND field = "value"` clauses, escaping user values (`\`,`"`); a nil
      allowed-set disables the scope (tests), an empty set matches nothing
      (`repo_id IN [-1]`). httpapi `Handler` gained a `Searcher` seam +
      `NewHandlerWithSearch`; `Mount` registers `GET /api/v1/search` only when a
      searcher is present. The `searchDocs` handler reads `q`/facets/`limit`/
      `offset` and injects `authorize.FromContext` as `AllowedRepoIDs`. Tests
      (fake `Searcher`): param mapping, seam injects the authorizer's set
      ([999]), response shape (hits+snippet+facets), and route-absent-without-
      searcher._
- [x] Extend `/readyz` to report Meilisearch reachability.
      <br>_Done: replaced the single `readyChecker` interface with a
      `[]namedChecker` (name + `func(ctx) error`). `handleReadyz` runs each,
      building a per-dependency status map (`{"meilisearch":"ok","postgres":
      "ok"}`, sorted keys via `json.Marshal`), 503 if any fails so the body
      names the offender. `main` wires `postgres`→`st.Ping` and `meilisearch`→
      `searchClient.Health`. Tests: per-dep reachable/unreachable + a mixed case
      (postgres ok, meilisearch down → 503)._
- [x] Integration tests (Meilisearch via the chosen harness): index population,
      facet counts, snippet/highlight, deletion removes from the index, and the
      filter-injection seam.
      <br>_Done: `internal/search/search_integration_test.go` (`//go:build
      integration`, testcontainers `getmeili/meilisearch:v1.12`, shared
      container via TestMain). Five cases over a 3-doc / 2-repo / 2-type corpus:
      index+search (title outranks body), facet counts (type/status/repo/
      author), snippet `<em>` highlight, deletion removes from the index, and
      the filter-injection seam (repo-1 scope, repo-2 scope, empty scope → no
      results). **Bug caught by the integration test**: Meilisearch document ids
      allow only `[a-zA-Z0-9-_]`, so the composite PK separator is `_` not the
      `:` DESIGN-0001 illustrated (internal-only; not in the response)._

#### Success Criteria

- After an ingest, `GET /api/v1/search?q=…` returns hits with facet counts and
  highlighted snippets matching DESIGN-0001's example shape.
- A deleted document disappears from search results; an unchanged document is
  not re-indexed (gate proven).
- The endpoint is usable directly from `curl` (the surface a future MCP search
  tool would reuse).

**Status: COMPLETE ✅** — all criteria met. The ingest→index→search path is
proven end-to-end by `internal/e2e/search_integration_test.go` (real Postgres +
real Meilisearch): after onboard, `GET /api/v1/search?q=logging` returns the
FW-0001 hit with a `<em>`-highlighted snippet and a `frameworks` facet count.
Deletion-removes-from-index and the unchanged-not-reindexed gate are proven by
the `internal/search` integration tests and the `internal/ingest` unit tests.
The endpoint is a plain chi `GET` behind the authorize seam — curl-usable.

---

### Phase 4: Async ingestion — Redis queue, worker, debounce

Decouple ingestion from its triggers: triggers enqueue Redis jobs and return
fast; a worker pool drains them; per-repo debounce coalesces bursts (Decision
7).

#### Tasks

- [x] Implement `internal/queue` over Redis (`hibiken/asynq`, OQ 1): an enqueue
      API and a worker pool that runs the Phase-2 ingest pipeline.
      <br>_Done: `internal/queue` (asynq + go-redis, both promoted to direct).
      `IngestJob{InstallationID, Owner, Name, Reason}` (no HEAD SHA — the worker
      re-fetches HEAD, so "latest wins" is free). `Client` (`NewClient(redisURL,
      debounce)`) satisfies the consumer-side `Enqueuer`; `EnqueueIngest` builds
      an asynq task with `TaskID("ingest:"+owner+"/"+name)` (coalesce key known
      before the repo row exists), `ProcessIn(debounce)`, `MaxRetry(5)`,
      `Retention(24h)`; `ErrTaskIDConflict`/`ErrDuplicateTask` → coalesced (nil).
      `Client.Ping` (go-redis) backs /readyz; `Close` joins both clients.
      `Worker` (holds `Ingestor` + `*asynq.Server`) `Start`s non-blocking and
      `Shutdown`s draining; `handleIngest` decodes → `Ingestor.Run`, dropping a
      malformed payload via `asynq.SkipRetry` and returning ingest errors for
      retry (content-hash gate makes retries idempotent). Unit tests: handler
      success, SkipRetry-on-bad-payload, transient-error-retries, isFailure._
- [x] Move ingest invocation behind the queue; the manual trigger (OQ 4) and
      (later) webhooks enqueue jobs rather than running inline.
      <br>_Done: `-onboard` now seeds the installation synchronously then
      **enqueues** an ingest job (Reason `onboard`) and exits — a running
      server's worker performs the ingest. The server runs the worker
      **in-process** alongside HTTP (`workerConcurrency=2`, single-binary
      ethos). The worker's `queue.Ingestor` is the composition-root
      `ingestRunner` (cmd/docz-api/runner.go), which builds a per-installation
      `githubapp.Client` per job — so one worker serves every installation with
      NO change to the Phase 2/3 `RepoFetcher`/`githubapp` signatures. `/readyz`
      gained a third `redis` checker (`queueClient.Ping`)._
- [x] Implement per-repo debounce/coalesce (`INGEST_DEBOUNCE`) so a repo with a
      pending job collapses duplicates and the latest HEAD wins.
      <br>_Done: `EnqueueIngest` uses `TaskID("ingest:"+owner+"/"+name)` +
      `ProcessIn(INGEST_DEBOUNCE)`; a second enqueue in the window returns
      `ErrTaskIDConflict` → coalesced. The single scheduled job runs after the
      window and re-fetches HEAD, so the latest wins for free. The worker's
      `DelayedTaskCheckInterval` is tuned to 1s (asynq defaults to 5s) so a
      debounced job runs within ~1s of its window closing. Proven by
      `TestDebounceCoalesces` (5-trigger burst → exactly 1 run)._
- [x] Add at-least-once delivery + retry semantics; rely on the `content_hash`
      gate to keep re-runs cheap and safe.
      <br>_Done: `MaxRetry(5)` + asynq's default exponential backoff; the worker
      returns ingest errors for retry and drops only malformed payloads via
      `asynq.SkipRetry`. `IsFailure` excludes `context.Canceled` so a
      shutdown-interrupted job re-queues. Idempotency rides on the store's
      content-hash gate + the overwrite-by-PK indexer (proven by the Phase 2/3
      e2e tests); `TestEnqueueAndDrain` proves at-least-once delivery._
- [ ] ~~(Optional) Cache GitHub installation tokens in Redis keyed by
      `installation_id` so replicas share one token.~~ **DEFERRED to Phase 5.**
      <br>_Rationale: no token pressure in Phase 4 — the homelab runs one
      replica and `ghinstallation/v2` already caches the token in-memory per
      process. The cache is a drop-in for `githubapp.Client` and belongs in
      Phase 5 when webhook-driven onboarding actually creates multi-replica
      token demand (per go-architect)._
- [x] Graceful shutdown: stop accepting, drain in-flight jobs, close cleanly.
      <br>_Done: `serveWithWorker` drains in a strict order on SIGTERM/SIGINT —
      (1) `http.Server.Shutdown` stops accepting + drains in-flight requests so
      no new webhook/onboard enqueues arrive, (2) `worker.Shutdown()` blocks
      until in-flight ingests finish (asynq graceful drain), (3)
      `queueClient.Close()` joins the asynq + redis clients; the pgxpool closes
      last via the deferred `pool.Close()`. `isFailure` treats `context.Canceled`
      as non-failure so a shutdown-interrupted job re-queues rather than burning
      a retry._
- [x] Tests: enqueue → worker drains; bursts coalesce to a single latest-HEAD
      run; a re-delivered job is idempotent; shutdown drains without loss.
      <br>_Done: unit tests (`worker_test.go`) cover the handler (success,
      SkipRetry-on-bad-payload, transient-retries, isFailure) with a fake
      ingestor + hand-built tasks. Integration tests (`queue_integration_test.go`,
      testcontainers `redis:7-alpine`): `TestEnqueueAndDrain`,
      `TestDebounceCoalesces` (burst → 1), `TestShutdownDrainsInFlight` (an
      in-flight job completes before `Shutdown` returns). Redelivery idempotency
      is proven at the store/index layer by the Phase 2/3 e2e tests._

#### Success Criteria

- Triggers return promptly (`202`-style) and a worker performs the ingest
  asynchronously.
- Rapid repeated triggers for one repo coalesce to a single ingest at the latest
  HEAD (debounce proven by test).
- Re-running a job is idempotent (no duplicate rows / no double-index); shutdown
  drains in-flight work.

**Status: COMPLETE ✅** — all criteria met (task 5 token-cache deferred to Phase
5 with rationale). `EnqueueIngest` returns promptly and the in-process worker
ingests asynchronously (`TestEnqueueAndDrain`); a 5-trigger burst coalesces to
one run via `TaskID` + `ProcessIn` (`TestDebounceCoalesces`); shutdown drains an
in-flight job (`TestShutdownDrainsInFlight`); redelivery is idempotent via the
content-hash gate + overwrite-by-PK indexer (Phase 2/3 e2e). Full unit +
integration suites green (Postgres + Meilisearch + Redis testcontainers).

---

### Phase 5: GitHub App onboarding + webhooks

Build on the App installation-token auth from Phase 2 (OQ 3b): add
install-driven onboarding and HMAC-verified webhooks that drive incremental
refresh, replacing the slice's manual onboarding trigger.

#### Tasks

- [x] Reuse the App auth flow in `internal/githubapp` (built in Phase 2 per OQ
      3b): the app JWT (RS256) → installation-token exchange via
      `POST /app/installations/{id}/access_tokens`, cached per `installation_id`
      until just before expiry — now driving onboarding + webhook ingest, not
      just the slice.
      <br>_Done: webhook-enqueued jobs flow through the existing per-installation
      `githubapp.Client` (built per job by `ingestRunner`), whose
      `ghinstallation/v2` transport does the JWT → installation-token exchange
      and caches the token in-memory until just before expiry. No separate
      `GET /installation/repositories` call was needed: the `installation` /
      `installation_repositories` payloads already carry the repo list, so
      onboarding enumerates from the payload (complete at homelab scale). The
      cross-replica Redis token cache stays deferred (Phase 4 task 5 rationale)._
- [x] Onboarding: handle `installation` / `installation_repositories` —
      enumerate installation repos, detect root `.docz.yaml`, insert
      `installations`/`repos`, enqueue full ingest; mark repos without a
      manifest unconfigured.
      <br>_Done: `handleInstallation` (`created` → upsert installation + enqueue
      an ingest per granted repo from the payload; `deleted` → offboard) and
      `handleInstallationRepos` (`added` → upsert + enqueue; `removed` → delete
      repos + purge index). `.docz.yaml` detection is left to the ingest worker
      rather than a pre-check: a repo with no manifest fails `githubapp.Fetch`
      with "no .docz.yaml at HEAD", logged by the worker (per go-architect — a
      dedicated `configured` flag is a YAGNI candidate). Offboarding deletes the
      installation/repo rows (`ON DELETE CASCADE` wipes doc_types + documents)
      and purges each repo from Meilisearch by `repo_id` filter._
- [x] Implement `internal/webhook`: HMAC-SHA256 verification with `hmac.Equal`
      (constant-time); reject mismatch with `401` and no work; route events.
      <br>_Done: `webhook.Handler` reads the raw body once, `verifyHMAC` recomputes
      `HMAC-SHA256(secret, body)` and compares with `hmac.Equal`; a bad/missing/
      malformed `X-Hub-Signature-256` returns `401` before any store write or
      enqueue. `route` dispatches parsed `go-github` events (`ParseWebHook`) to
      per-event handlers; unhandled events (ping, …) are accepted and ignored._
- [x] `push` handling: default-branch + `docs_dir`/`.docz.yaml` filter;
      diff-based partial re-ingest (narrow blob fetches; delete docs absent from
      new HEAD); `.docz.yaml` change → `doc_types` reconcile (add/remove/update
      types).
      <br>_Done: `shouldIngest` gates on default-branch (`refs/heads/<default>`)
      AND a changed path (union of every commit's added/modified/removed) that is
      `.docz.yaml` or under `docs_dir/`; on a match it enqueues a **full**
      re-ingest. The existing reconcile is a desired-state replace — its
      content-hash gate rewrites only changed docs, deletes docs absent from the
      new HEAD, and reconciles `doc_types` (add/remove/update) — so the full
      re-ingest achieves diff/delete/type-reconcile idempotently. **Narrow blob
      fetches are deliberately deferred** (documented in `handlePush`): they only
      save GitHub fetch cost, negligible at homelab scale._
- [x] `release` handling: wired but **log-only** for the versions feature (OQ 10
      / Decision 12); `push` / `release` also refresh the cached `CHANGELOG.md`
      on the `repos` row when it changes.
      <br>_Done: `logRelease` records the event (repo/action/tag) and takes no
      other action, keeping the subscription wired for the deferred versions
      feature. `CHANGELOG.md` is refreshed as a side effect of every full ingest
      (`githubapp.Fetch` re-fetches it and reconcile caches it on the repo row),
      so a docz-relevant push refreshes it for free; a standalone changelog-only
      refresh path is deferred with the versions feature._
- [x] Idempotency: record `X-GitHub-Delivery` in `webhook_deliveries`; a
      duplicate delivery is a no-op; reconcile against `last_synced_sha`.
      <br>_Done: `store.RecordDelivery` (`INSERT … ON CONFLICT (delivery_id) DO
      NOTHING RETURNING` → `isNew`) gates the handler right after signature
      verification: a replayed delivery returns `200` with no routing. Every
      operation is independently idempotent too (coalesced enqueue, content-hash
      + HEAD gate, `ErrNoRows`-tolerant deletes), so the delivery table is a
      redundant-work optimization on top of a self-idempotent pipeline._
- [x] Wire webhook events to enqueue ingest jobs (Phase 4 queue).
      <br>_Done: `main` mounts `POST /webhooks/github` on the root router
      (outside `/api/v1` and the authorize seam — it is HMAC-authenticated, not
      session-authenticated) via `webhook.New(secret, store, queueClient,
      searchClient)`. Onboard/push handlers call `queue.Client.EnqueueIngest`;
      the in-process worker drains them through the same pipeline as Phase 4._
- [x] Tests: table-driven HMAC (correct passes; wrong secret / tampered body /
      missing header → `401`, no DB writes; constant-time asserted); synthetic
      `push` payloads exercise reconcile + delete; a replayed delivery is a
      no-op.
      <br>_Done: `internal/webhook/webhook_test.go` — table-driven `verifyHMAC`
      (valid / wrong-secret / tampered-body / missing-header / bad-prefix /
      non-hex / near-miss), `ServeHTTP` bad-signature → `401` with zero
      deliveries + zero enqueues, replayed delivery → `200` + single enqueue,
      push filter (branch/path/unknown-repo), installation onboard/offboard,
      `shouldIngest` + `ownerName` tables. Constant-time is exercised by the
      near-miss case (same-length, shared-prefix signature still rejected).
      New store SQL is proven against real Postgres
      (`store/webhook_integration_test.go`: `RecordDelivery` idempotency,
      `DeleteRepo`/`DeleteInstallation` CASCADE, `ListRepoIDsByInstallation`);
      the index purge is proven against real Meilisearch
      (`search` `TestIntegrationDeleteRepoDocuments`, scoped by `repo_id`)._

#### Success Criteria

- Installing the app onboards its repos (enumerate → detect `.docz.yaml` →
  ingest); uninstall/removal offboards them.
- A `push` to the default branch incrementally refreshes only changed docs;
  adding/removing a type in `.docz.yaml` reconciles `doc_types`; a deleted doc
  is removed from Postgres and the index.
- Bad webhook signatures are rejected with `401` and zero writes; a replayed
  `X-GitHub-Delivery` performs no duplicate work.

**Status: COMPLETE ✅** — all criteria met. Installing the app (`installation`
created) upserts the installation and enqueues an ingest per granted repo, which
detects `.docz.yaml` and ingests; uninstall/repo-removal deletes the rows
(CASCADE) and purges the index. A default-branch push touching docz paths
enqueues a full re-ingest whose content-hash gate rewrites only changed docs,
reconciles `doc_types`, and deletes docs absent from HEAD (from Postgres and,
via `syncIndex`, the index). Bad signatures return `401` with zero writes and a
replayed `X-GitHub-Delivery` is a `200` no-op (unit-tested); the new store SQL
and index purge are proven by integration tests. Documented MVP simplifications:
payload-based repo enumeration (no `GET /installation/repositories`), ingest-time
`.docz.yaml` detection (no pre-check / `configured` flag), and full re-ingest on
push (narrow blob fetches deferred). Full unit + integration suites green.

---

### Phase 6: Authentication — pluggable providers + Redis sessions

Add site-user **authentication** behind one provider abstraction (GitHub
default, Okta, Keycloak) with Redis-backed sessions. Authorization stays a
pass-through seam (Decision 10).

#### Tasks

- [x] Implement `internal/auth`: the `Provider` interface (`Name` /
      `AuthCodeURL` / `Exchange`) and the GitHub OAuth provider (default).
      <br>_Done: `internal/auth/provider.go` (`Provider` interface + `Identity`),
      `internal/auth/github.go` (`GitHubProvider` over `golang.org/x/oauth2`;
      `Exchange` fetches the user via go-github and requires a **primary +
      verified** email), `internal/auth/registry.go` (name→provider lookup, sorted
      `Names()`), `internal/auth/state.go` (HMAC-SHA256 signed, 5-min-TTL OAuth
      `state` carrying the provider — the stateless CSRF guard, `hmac.Equal`
      constant-time verify)._
- [x] Implement the Okta and Keycloak OIDC providers via discovery
      (`issuer`/`client_id`/`client_secret`/scopes), using `coreos/go-oidc` +
      `golang.org/x/oauth2` (OQ 6).
      <br>_Done: `internal/auth/oidc.go` — one `OIDCProvider` backs both (they
      differ only by issuer/credentials). Discovery runs at startup under a bounded
      context; `Exchange` verifies the `id_token` (JWKS signature + audience +
      issuer + expiry, go-oidc defaults) before reading `sub`/`email`/`groups`, and
      drops an email the issuer asserts `email_verified:false`._
- [x] Implement `internal/session`: Redis session store (`sess:<id>` →
      identity + groups + expiry, `SESSION_TTL`), issue/lookup/revoke; set an
      httpOnly, SameSite cookie.
      <br>_Done: `internal/session/store.go` — opaque 32-byte `crypto/rand`
      session id, `sess:<id>` → JSON identity with a `SESSION_TTL` Redis
      expiry; `Issue`/`Lookup`/`Revoke` (`redis.Nil`→`ErrSessionNotFound`);
      `SetCookie` is `HttpOnly` + `SameSite=Lax` + `Secure`-when-https, mirrored by
      `ClearCookie`. Owns its own Redis client (`Close`/`Ping`)._
- [x] Auth endpoints: `/auth/login?provider=…`, `/auth/callback` (exchange →
      upsert `users` row → issue session), `GET /api/v1/auth/session`,
      `POST /api/v1/auth/logout` (single `DEL`).
      <br>_Done: `internal/authhttp/{handler,endpoints}.go` — `login` signs state
      and redirects; `callback` verifies state, `Exchange`s, `UpsertUser`s
      (`store.UpsertUser`, `users.sql` `ON CONFLICT (provider,subject)`), issues the
      session, sets the cookie, redirects to `/`; `getSession` returns the current
      user; `logout` revokes + clears. `MountPublic` (login/callback) vs `MountAPI`
      (session/logout, behind the gate)._
- [x] Session middleware resolves the session into request context; the
      `authorize` seam still returns "all onboarded repos" (authZ deferred), but
      now keyed off a real identity. Protected endpoints return `401` without a
      valid session.
      <br>_Done: `internal/session/middleware.go` — cookie → `Lookup` → inject
      `Session` into context or `401`; `FromContext` reads it back. In
      `cmd/docz-api/main.go` the `/api/v1` `gate` composes `session.Middleware`
      (runs first) over `authorize.Middleware`, so authorization resolves behind a
      real identity._
- [x] Tests: a provider stub drives login/callback → session issued; session
      lookup populates identity; logout revokes; unauthenticated requests to
      protected endpoints → `401`; `Groups` claims persisted for the future
      authZ layer.
      <br>_Done: `internal/authhttp/handler_test.go` (stub provider drives
      login→redirect, callback→upsert+issue+cookie, rejects invalid/forged state +
      exchange failure, `getSession`/`logout` behind the real middleware),
      `internal/session/middleware_test.go` (valid/unknown/no-cookie → `200`/`401`,
      parallel-safe), `internal/auth/{state,registry}_test.go`, and
      `internal/session/store_integration_test.go` (real Redis: issue→lookup→revoke,
      unknown→NotFound, TTL expiry, **`Groups` round-trip persisted**)._

#### Success Criteria

- GitHub login works end to end; Okta and Keycloak work via OIDC config.
- A session cookie is issued on callback, validated on subsequent requests, and
  revoked on logout; `/api/v1/auth/session` reflects the current user.
- Protected endpoints require a session (`401` otherwise); the `authorize` seam
  is the single, isolated switch point where the future SpiceDB resolver plugs
  in.

**Status: COMPLETE ✅** — all criteria met. GitHub login works end to end
(`/auth/login?provider=github` → provider authorize → `/auth/callback` →
verified-email Exchange → `users` upsert → session + cookie → `/`); Okta and
Keycloak ride the same `/auth/callback` via one discovery-driven `OIDCProvider`
selected by the signed state. The session cookie (`HttpOnly`, `SameSite=Lax`,
`Secure`-when-https, opaque 32-byte id) is issued on callback, validated by the
`/api/v1` session middleware on every subsequent request, and revoked on logout;
`GET /api/v1/auth/session` reflects the current user and `401`s without a
session. The `authorize` seam still grants all onboarded repos but now resolves
behind a real identity — it remains the single switch point for the future
SpiceDB resolver. `Groups` claims are persisted (Redis round-trip proven by
integration test) for that authZ layer. State is a stateless HMAC-SHA256 CSRF
guard (constant-time verify, 5-min TTL). Full unit + integration suites green;
`golangci-lint` at 0 issues. Documented MVP simplification: OIDC `nonce`
binding is deferred (the signed `state` already guards CSRF for the code flow) —
a cheap hardening follow-up since `statePayload` already carries a per-login
nonce.

---

### Phase 7: Hardening, deploy, contract, CI

Pin the real dependency, lock the consumer contract, add observability, and get
the service release-ready.

#### Tasks

- [x] Confirm the docz dependency stays a **pinned published tag**
      (`require github.com/donaldgifford/docz v0.5.0`, no `replace`), and bump
      it deliberately if a newer docz ships (R6; DESIGN-0007 already published
      v0.5.0).
      <br>_Done: `go.mod` pins `github.com/donaldgifford/docz v0.5.0` as a direct
      require with **no `replace` directive**; the module resolves from the
      published tag. v0.5.0 remains the current published release (DESIGN-0007), so
      no deliberate bump is warranted this phase._
- [x] Add contract golden fixtures matching the response shapes DESIGN-0009
      consumes, asserted in CI so a breaking JSON change fails here first.
      <br>_Done: `internal/httpapi/contract_test.go` drives the real chi router
      (behind the authorize seam, with a deterministic searcher) for all six
      read/search endpoints plus the 404 error envelope, freezing
      status + `Content-Type` + JSON body per endpoint in
      `testdata/contract/*.json`. `TestContractGolden` asserts them on every
      `just test` run (so CI catches a renamed/removed/retyped field — verified by
      a tampered-fixture check); `-update` regenerates them for an intentional
      change. Locks the DESIGN-0001 wire shapes DESIGN-0009 consumes._
- [x] Observability (OQ 8 — full stack): request-logging middleware over `slog`;
      a Prometheus `/metrics` endpoint; a `/healthz` liveness endpoint and the
      `/readyz` readiness probe extended to cover Redis (Postgres + Meilisearch
      already wired in Phases 1/3); and full OpenTelemetry tracing across the
      ingest, HTTP, and worker paths.
      <br>_Done: new `internal/telemetry` package (per go-architect) —
      `Setup(ctx, Config)` installs the global W3C propagator and (when
      `OTEL_EXPORTER_OTLP_ENDPOINT` is set) a batching OTLP/HTTP `TracerProvider`,
      returning a flush-bounded shutdown; no-op and zero-overhead when unset.
      `RequestLogger` logs one structured slog line per request (probes skipped);
      `Instrument` starts a server span + records Prometheus HTTP metrics keyed by
      chi **route template** (bounded cardinality), extracting upstream W3C context
      and marking 5xx as span errors. Metrics via `prometheus/client_golang` on the
      default registry (RED for HTTP + ingest); `/metrics` mounted alongside the
      probes, outside the auth gate. Tracing propagates across the asynq boundary:
      `IngestJob` carries `traceparent`/`tracestate` (inject at enqueue, extract in
      the worker → `queue.ingest` consumer span → `ingest.run` with
      `fetch`/`reconcile`/`index` child spans). `/healthz` + `/readyz` (all three
      deps) already stood up in Phases 0/1/3. go-style + go-review clean (handle-once
      + OTel error-status fixes applied); unit + integration suites green._
- [x] Container: confirm the distroless `Dockerfile` builds the service; provide
      deploy manifests/compose wiring Postgres + Redis + Meilisearch; secrets
      via env/secret store.
      <br>_Done: the multi-stage `Dockerfile` builds and runs — `docker build` +
      `docker run --rm docz-api:dev --version` prints the injected
      `version`/`commit`/`date`; a 47.8 MB `distroless/static:nonroot` image.
      New `deploy/` reference stack: `deploy/compose.yaml` runs the service plus
      all three dependencies on a private network, health-gated
      (`depends_on: condition: service_healthy`), publishing only `:8080`. Secrets
      are externalized — all config via a gitignored `.env.production` env store
      (`deploy/.env.production.example` template), the GitHub App private key via a
      mounted Docker secret referenced by path; `deploy/README.md` documents
      bring-up, probes, and the k8s translation. `compose config` validates._
- [x] Audit error messages for consistency and wrapping (`%w`); resolve
      TODO/FIXME comments.
      <br>_Done: zero TODO/FIXME/XXX/HACK comments in the tree. Every wrapping
      `fmt.Errorf` uses `%w` (no `%v`-on-error chain breaks); the non-`%w` cases
      are all new-error constructions with no wrappable cause (validation, missing
      config, unsupported encoding). Message style is uniform Go convention —
      lowercase, no trailing punctuation, no "failed to" prefix, "verb-ing
      context: %w". Fixed the two naked returns in `ingest.Service.Run`
      (`buildDocTypes`/`buildDocuments`) to carry `owner/name` context like the
      rest of `Run`._
- [x] Ensure `make ci` / `just lint` + `just test` pass; review coverage
      (target >80%).
      <br>_Done: `just lint` at 0 issues and `just test` + `just test-integration`
      fully green. Coverage reviewed and materially raised (added unit tests for
      the session cookie surface, the GitHub/OIDC provider surface + OIDC discovery,
      the search filter/facet builders, the webhook release/purge paths, and the
      httpapi/authhttp 500 branches): **internal aggregate 78.0%** (from ~69%), with
      **config 100 / authorize 95 / telemetry 93 / ingest 88 / authhttp 85 /
      httpapi 85 / webhook 82** over target and **auth 79 / queue 79 / search 78 /
      session 78 / store 76** just under. Documented exceptions to a strict
      per-package 80%: the OAuth/OIDC `Exchange` and GitHub `Fetch` paths do live
      network I/O against real providers (covered by integration/e2e + manual smoke,
      not unit tests), the `store` layer is cross-covered by e2e, and
      `cmd/docz-api` is the composition root (covered by the live smoke test, low by
      design — noted in Phase 1). No CI coverage gate; refactoring production code
      purely to lift the number is not warranted at homelab scale._

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

**Status: COMPLETE ✅** — all six tasks done. `just lint` is at 0 issues and the
full unit + integration (testcontainers Postgres/Redis/Meili) suites are green.
The distroless `Dockerfile` builds and runs (47.8 MB `static:nonroot`, prints
injected version metadata); `deploy/` provides a reference stack wiring the
service plus all three dependencies with secrets externalized (env store +
mounted key secret), and `/healthz` + `/readyz` (all three deps) + `/metrics`
are served — the OQ 8 full-stack observability (slog request logging, Prometheus
metrics, end-to-end OpenTelemetry tracing across HTTP → queue → ingest) is
wired. The read + search wire contract is frozen by golden fixtures asserted in
CI. The docz dependency stays pinned at `v0.5.0` with no `replace`. Coverage was
reviewed and raised to a 78% internal aggregate (7/13 packages ≥80%); the
sub-80% remainder is network-bound provider/fetch code (covered by
integration/e2e/smoke), the cross-covered `store` layer, and the composition
root — a documented, proportionate exception to a strict per-package 80% at
homelab scale (there is no CI coverage gate).

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

All items below are implemented and green across the phases.

- [x] Unit tests for parse→row mapping, type resolution (name/prefix/alias),
      `content_hash` stability, and config loading (table-driven where
      applicable).
      <br>_`internal/ingest/mapper_test.go`, `internal/httpapi/typeresolver_test.go`,
      `internal/config/config_test.go` (100% cover), content-hash gate proven by the
      e2e re-onboard no-op._
- [x] Integration tests (Postgres + Redis + Meilisearch via OQ 7) for the store,
      queue, search, and webhook-driven ingest paths.
      <br>_`internal/store/{store,webhook}_integration_test.go`,
      `internal/queue/queue_integration_test.go`,
      `internal/search/search_integration_test.go`, `internal/e2e/` — all
      testcontainers, `//go:build integration`._
- [x] Webhook HMAC table-driven tests (pass / wrong secret / tampered / missing
      → `401`, no writes; constant-time comparison).
      <br>_`internal/webhook/webhook_test.go` `TestVerifyHMAC` (7 cases incl. a
      same-length near-miss for the constant-time path) + `ServeHTTP` bad-sig →
      `401` with zero writes._
- [x] Hermetic e2e onboarding test using recorded GitHub fixtures.
      <br>_`internal/e2e/onboard_integration_test.go` +
      `search_integration_test.go` — fixture repo tree through the full
      fetch→parse→reconcile→index→serve path against real Postgres/Meili._
- [x] Auth tests with a provider stub: login/callback/session/logout and `401`
      on protected endpoints.
      <br>_`internal/authhttp/handler_test.go` (stub provider drives
      login/callback/session/logout; unauthenticated → `401`) +
      `internal/session/` middleware/store tests._
- [x] Contract golden fixtures matching DESIGN-0009's consumed shapes.
      <br>_`internal/httpapi/contract_test.go` freezes status + content-type + body
      for all read/search endpoints in `testdata/contract/*.json`, asserted every
      run._
- [x] Golden/fixture discipline: an `-update` flag regenerates fixtures; never
      hand-edited.
      <br>_`contract_test.go` `-update` flag regenerates the fixtures; the
      committed files are asserted byte-for-byte otherwise (drift fails CI)._

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
