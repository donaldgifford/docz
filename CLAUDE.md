# CLAUDE.md

Per-repo orientation for `donaldgifford/docz-api`. This file is a Go-shaped
overlay on top of the universal homelab `CLAUDE.md` (see
[homelab/docs](https://github.com/donaldgifford/docs)); the universals apply
here too — only Go-specific guidance is captured below.

## What this is

`docz-api` is a Go binary maintained as part of the homelab fleet:

- Single binary under `cmd/docz-api/`; library code under `internal/` (private
  to the module).
- Built into a distroless container via `Dockerfile`; released as multi-arch
  (linux+darwin × amd64+arm64) archives via `goreleaser`.
- Hosted on GitHub (`github.com/donaldgifford/docz-api`) — that's the `origin`
  remote and the **only** active CI (GitHub Actions, under `.github/workflows/`).
  A Forgejo mirror is aspirational: there is **no `.forgejo/workflows/`** yet, and
  some release plumbing (`gitea_urls`, `GITEA_TOKEN`) is stubbed for when one
  exists. Don't assume Forgejo-primary.

## Layout

```text
cmd/docz-api/    # main package — keep thin, parse flags + call into internal/
internal/               # library code; not importable outside this module
Dockerfile              # multi-stage distroless build, cached layers
.goreleaser.yml         # release config (multi-arch archives + checksums)
mise.toml               # pinned go + golangci-lint + goreleaser + universal tools
justfile                # `just` task runner — `just` for the menu
.github/workflows/      # CI (GitHub Actions) — the only CI today
# .forgejo/workflows/   # NOT present — Forgejo mirror is aspirational
```

## Workflows

### Build + run

- `just build` — `go build -o bin/docz-api ./cmd/docz-api`
- `just run -- <args>` — runs via `go run` without building
- `just test` — race detector + coverage to `coverage.txt`

### Lint + format

- `just lint` — `golangci-lint run` + yamllint + markdownlint + prettier (covers
  the universal linters too).
- `just fmt` — `go fmt ./...` + yamlfmt + prettier `--write`.

### Release

- `just release v0.1.0` — tag + push. CI picks up the `v*` tag and runs
  `goreleaser release --clean`, producing multi-arch archives and a GitHub
  release (via `GITHUB_TOKEN`). The Forgejo path (`GITEA_TOKEN`) is stubbed for
  a future mirror.
- Version metadata is injected into the binary via `-ldflags`: `main.version`,
  `main.commit`, `main.date`. `--version`-style output should print these.

### Container build

Built locally with:

```bash
docker build -t docz-api:dev \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) .
```

The Dockerfile uses BuildKit `--mount=type=cache` for `/go/pkg/mod` and
`/root/.cache/go-build` — first build is cold, subsequent builds reuse the cache
layers.

## Go-specific conventions

- **`go.mod` go directive matches `mise.toml`** (currently `go 1.26.4`). Bump
  both together — Renovate's Go updater handles `go.mod`; bump `mise.toml` in
  the same commit.
- **No `vendor/`**. Modules are resolved at build time; the Docker cache mount
  handles offline-ish builds.
- **`internal/` is a hard wall** — packages there can't be imported by other
  modules. Use it liberally; promote to a separate module only when something
  outside this repo actually needs it.
- **`slog` for structured logs**, not `log` or third-party loggers. Set the
  default handler in `main()` so library code doesn't have to thread loggers.
- **No `init()` for behavior**. `init()` runs at import time — it breaks test
  isolation and surprises future-you. Wire dependencies in `main()`.
- **Tests live next to the code** (`foo_test.go` alongside `foo.go`).
  Integration tests that need external services go under a
  `// +build integration` (or `//go:build integration`) tag and run via
  `go test -tags=integration ./...`.
- **Errors wrap with `%w`**: `fmt.Errorf("loading config: %w", err)`. Top of the
  call stack handles via `errors.Is` / `errors.As`.

## CI matrix

- **GitHub Actions is the only CI** — everything lives under
  `.github/workflows/`. There is **no `.forgejo/workflows/`**; the Forgejo
  mirror is aspirational, so don't assume a Forgejo-primary setup.
- `.github/workflows/ci.yml` runs on every push/PR: `Lint` (golangci-lint via
  the action, **not** `just lint`), `Test Go` (`just test-coverage` + Codecov),
  `Security Scan` (govulncheck + Trivy), and `Build` (goreleaser `--snapshot`
  + SBOM scan). Sibling workflows add CodeQL, license-check (`go-licenses`,
  which needs a root `LICENSE`), trufflehog (verified-only), a changelog drift
  check + auto-regen (`git-cliff` via `cliff.toml`; sync commits match
  `^chore.*changelog` and are skipped to stay idempotent), and a required
  semver label (`major`/`minor`/`patch`/`dont-release`) on PRs.
- `release.yml` fires only on `v*` tag push; `goreleaser` consumes
  `.goreleaser.yml` with `GITHUB_TOKEN` (the `GITEA_TOKEN`/`gitea_urls` path is
  stubbed for a future Forgejo mirror).

## Gotchas

- **`go mod tidy` on first scaffold**: the post-create hook runs it
  automatically. If you skip hooks (`--no-hooks`), run it manually before the
  first `just build` or imports will be unresolved.
- **`goreleaser` v2 config**: the v1 → v2 migration moved `archives[].format` to
  `archives[].formats` (slice). If you copy a pre-v2 `.goreleaser.yml` from
  elsewhere, validate with `goreleaser check`.
- **Distroless `nonroot` UID is 65532**. If the binary needs to write state,
  mount a writable volume — the rootfs is read-only.
- **goreleaser + Forgejo**: the v6 action defaults to GitHub-shaped release
  URLs. The `gitea_urls` block in `.goreleaser.yml` is commented by default —
  uncomment for Forgejo releases, and ensure `GITEA_TOKEN` is set in repo
  Secrets (PAT with `write:repository`).

## Implementation (DESIGN-0001 / IMPL-0001)

The service is being built out per `docs/design/0001-*.md` (Approved) and
`docs/impl/0001-*.md` (the phased plan). Conventions established as the build
progresses:

- **Build/lint/test entry points are `just`** — `just build`, `just test`,
  `just lint`, `just fmt`. There is no `Makefile`; any "`make lint`/`make fmt`"
  instruction maps to the corresponding `just` recipe.
- **docz parsing library** is pinned at `github.com/donaldgifford/docz v1.0.0`
  (a plain `require`, no `replace`; bumped from `v0.5.0` — see INV-0001, the
  contract-guarded surface was unchanged). As of `v1.0.0` docz **no longer pulls
  `spf13/viper` transitively** (it moved `config.Load` to `yaml.v3`); `viper`
  (`v1.21.0`) is now a **standalone direct dep** used only by `internal/config`,
  so DESIGN-0001 Decision 2's "reuse viper from docz" rationale no longer holds.
  Import its packages with the **aliases `doczcfg` / `doczdoc`** everywhere, per
  DESIGN-0001 — this repo has its own `internal/config`, so the alias keeps "the
  docz library" unambiguous at every call site even in files that don't
  currently import `internal/config`. This deliberately overrides the generic
  "avoid import aliases" style rule.
- **`internal/doczcontract`** is a runtime-code-free package whose tests guard
  the pinned docz surface (R1–R5). If a docz bump breaks the contract, it fails
  here, not deep in ingest. Re-run after any docz version change.
- **Tests use the standard-library `testing` package only** — no testify or
  other assertion deps. Prefer table-driven tests; positional struct literals
  are fine for tables with ≤3 fields.
- **Secrets use `config.Secret`** (a `string` type that redacts on slog, `%s`,
  `%v`, `%+v`, `%#v`). Read the real value only via `.Reveal()` — every unwrap
  is explicit and greppable. Never log a raw credential.
- **`internal/config` is env-only** (`spf13/viper` + `AutomaticEnv`, no config
  file). `Load()` returns one `ErrInvalidConfig` listing **all** problems.
  Config value receivers are heavy — `AuthEnabled` uses a pointer receiver on
  purpose (gocritic `hugeParam`); don't flip it back to a value receiver.

### Phase progress

- **Phase 0 — Foundations: COMPLETE ✅** — docz `v0.5.0` pinned +
  `internal/doczcontract` smoke test; core deps pinned
  (chi/pgx/go-redis/asynq/meilisearch); `internal/config` (typed env config,
  validation, `Secret`, 100% cover); `main()` wiring (chi server, `/healthz`
  liveness, graceful shutdown, `-version`); `compose.yaml` + `.env.example`
  (Postgres/Redis/Meili, all healthy). All success criteria met; skeleton green
  (`build`/`test`/`lint`/`fmt`).
- **Phase 1 — Persistence: COMPLETE ✅** — initial goose migration (all 6
  tables + indexes, verified up/down); migrations embedded + `store.Migrate`/
  `MigrateDown` runner, `main()` auto-migrates on startup, `-migrate` flag for
  CI/ops (idempotent); sqlc config + query sets generated (typed access in
  package `store`; `just generate`/`generate-check`); `internal/store`
  `ReconcileRepo` tx (repo upsert + doc_types reconcile + documents
  content-hash gate + delete-absent, one tx) with plain-Go input DTOs + a
  `ReconcileResult` summary; `store.NewPool` + `main()` wires the runtime
  pgxpool/`Store` and serves `/readyz` (Postgres reachability via a narrow
  `readyChecker` interface, 200/503, unit-tested with a stub); testcontainers
  integration tests (`//go:build integration`, `just test-integration`) covering
  reconcile/gate/delete-absent against a real Postgres. All success criteria met.
  - Migrations run via goose's global-free
    `goose.NewProvider(DialectPostgres, db, migrations.FS)`; `db` is a
    `database/sql` conn from `sql.Open("pgx", …)` (pgx stdlib adapter),
    **separate from** the runtime pgxpool. `-migrate` applies + exits; normal
    startup applies then serves.
  - **Persistence conventions** (per go-architect): pgx v5 + pgxpool at runtime;
    goose runs migrations via the `pgx/v5/stdlib` `database/sql` adapter (never
    shares the pool); sqlc (`sql_package: pgx/v5`) generates typed queries into
    `internal/store`. Only JSONB is overridden (→ `json.RawMessage`); nullable
    TEXT/time/date stay as sqlc's `pgtype.Text`/`pgtype.Timestamptz`/`pgtype.Date`
    defaults (deviated from the architect's local `NullableText` — simpler; the
    `pgtype` values get mapped to clean DTOs at the Phase 2 boundary).
    `ReconcileRepo` is one tx:
    `pool.Begin` → `queries.WithTx(tx)` → deferred `Rollback` → explicit
    `Commit`; content-hash gate lives in Go, not SQL. Store constructor is
    `NewStore` (avoids colliding with sqlc's generated `New`). Integration tests
    behind `//go:build integration` with testcontainers.
  - `users` / `webhook_deliveries` Go code is **YAGNI until Phases 6/5** — the
    tables exist now; queries/methods come when first needed.
  - `cmd/docz-api` is the composition root — `run()`/`serve()` are covered by a
    live smoke test, not unit tests, so its statement coverage is low by design.
  - Local infra: `docker compose up -d` (Postgres 5432 / Redis 6379 / Meili
    7700); copy `.env.example` → `.env` for `just run`. CI uses testcontainers
    (later phases), not compose.
  - Core deps are still staged `// indirect` until their packages import them —
    **do not run a bare `go mod tidy`** while they're unused (it prunes them);
    use `go get`. `viper` is now direct (used by `internal/config`).
- **Phase 2 — Thin vertical slice: COMPLETE ✅** — synchronous hand-onboarded
  fetch→parse→upsert→serve, all 7 tasks done and all acceptance criteria proven
  by the `internal/e2e` integration test (five endpoints match DESIGN-0001, the
  custom type is addressable by name/prefix/alias, the content-hash gate makes an
  unchanged re-onboard a no-op, changed docs rewrite and removed docs delete).
  Architecture (per go-architect):
  - **`internal/ingest`** owns the consumer-side boundary: `RepoFetcher`
    interface (`Fetch(ctx, owner, name) (*RepoSnapshot, error)`) + `RepoSnapshot`
    {HeadSHA, DefaultBranch, ConfigYAML []byte, ChangelogMD []byte, ChangelogSHA,
    Blobs []BlobEntry{Path,GitSHA,Content}}. `Service` (`NewService(reconciler,
    RepoFetcher)`, `Run(ctx, installationID, owner, name) (ReconcileResult,
    error)`) does fetch → `loadConfig` → `Validate` → per-blob
    `doczdoc.ParseFrontmatter` (skip `ErrNoFrontmatter` with a warn, don't abort)
    → map → `store.ReconcileRepo`. Narrow `reconciler` interface (just
    `ReconcileRepo`).
  - **`loadConfig` bridge**: `doczcfg.Load` is disk-based, so write ONLY
    `.docz.yaml` to an `os.MkdirTemp` dir + point `HOME` at an empty temp dir
    (suppress the `$HOME/.docz.yaml` merge, like doczcontract tests), `Load("",
    tmp)`, deferred `RemoveAll`. Doc blobs never touch disk (byte-based
    `ParseFrontmatter`). `config_snapshot` stores the **raw `.docz.yaml` bytes**
    (`json.RawMessage(snap.ConfigYAML)`) — faithful to HEAD, no marshal risk.
  - **mapper** (`internal/ingest/mapper.go`): `TypeConfig`→`DocTypeInput`
    (Statuses/Aliases→`json.Marshal`), blob+`Frontmatter`→`DocumentInput`
    (DocID=`fm.ID`, Type=canonical name, ContentHash=`hex(sha256(raw))`,
    Created=`time.Parse("2006-01-02")` zero-on-empty, Status=`string(fm.Status)`).
  - **`internal/githubapp`**: concrete `Client` implementing `ingest.RepoFetcher`
    via `ghinstallation/v2` (App JWT→installation token transport, auto-refresh) +
    `google/go-github/v66`. `NewClient(appID, pemKey []byte, apiBase,
    installationID, httpClient)` — inject `*http.Client` (stub RoundTripper in
    tests). Fetch: get `.docz.yaml` blob first → parse for DocsDir/type dirs →
    resolve default-branch HEAD → recursive tree → filter to `.docz.yaml` +
    `docs_dir/<type.dir>/` via `doczdoc.IsDoczFile` → fetch blobs (base64) +
    optional root `CHANGELOG.md`.
  - **`internal/httpapi`**: chi `Handler.Mount(r, authzMiddleware)` at `/api/v1`.
    Response **DTOs** (own structs, map `pgtype` nullables → `string`/`YYYY-MM-DD`,
    never expose sqlc types). `{type}` resolved by `resolveType(types
    []store.DocType, input) (canonical, ok)` — pure match over name/id_prefix/
    aliases (no live doczcfg at serve time). Narrow `storeReader` interface.
  - **`internal/authorize`**: seam middleware. `Authorizer.Allowed(ctx, r)
    (AllowedRepos, error)`; `Middleware(a)` injects `AllowedRepos []int64` into
    ctx; `FromContext(ctx)` + `AllowedRepos.Contains(id)`. Phase 2 stub
    `AllReposAuthorizer` (narrow `repoLister`) returns all repo IDs; Phase 5 swaps
    impl only. Handlers use allowed-set for **existence hiding** (404 when a repo
    id isn't allowed).
  - **onboard**: `-onboard owner/name@installationID` flag on the binary (like
    `-migrate`); seeds installation+repo, runs one `Service.Run`. No admin HTTP
    surface in Phase 2.
  - **New store read methods/queries**: `ListRepos :many`, `ListDocumentsByType
    :many` (no `raw_md`), `GetDocumentByID :one` (with `raw_md`); reuse
    `ListDocTypes` for `GetDocTypesForRepo`.
  - **New deps**: `google/go-github/v66`, `bradleyfalzon/ghinstallation/v2` (add
    via `go get`, direct).
  - **Testing**: unit mapper tests (custom `frameworks`/`FW-0001` fixture +
    missing-frontmatter skip); hermetic e2e via an in-memory **fake
    `RepoFetcher`** at the ingest boundary (not a network VCR); `githubapp` token/
    tree-filter logic tested with a stub `http.RoundTripper` + `testdata/` JSON
    fixtures.
- **Phase 3 — Search: COMPLETE ✅** — Meilisearch indexer + faceted search, all
  5 tasks done and all success criteria proven. The headline criterion is
  proven end-to-end by `internal/e2e/search_integration_test.go`: onboarding a
  repo through the real ingest pipeline (real Postgres + real Meilisearch
  indexer) makes its docs searchable via `GET /api/v1/search`, returning hits,
  facet counts, and `<em>` snippets. Deletion removes from the index and the
  content-hash gate skips unchanged docs (proven by the search integration
  tests + ingest unit tests).
  Architecture (per go-architect):
  - **`internal/search`** wraps `meilisearch.ServiceManager` (meilisearch-go
    `v0.36.3`, now a direct dep). `Client` (`New(host, apiKey)`) satisfies the
    consumer-side `ingest.Indexer` and `httpapi.Searcher` interfaces. Boundary
    types in `types.go` keep meilisearch out of ingest/httpapi: `IndexDoc`
    (index schema, PK `id="<repo_id>:<doc_id>"`, `created` `YYYY-MM-DD`,
    `updated_at` Unix secs), `SearchParams` (`Query`, `AllowedRepoIDs` from the
    authorize seam, `Repo`/`Type`/`Status`/`Author` facet filters), `SearchHit`,
    `SearchResult` (matches DESIGN-0001 wire shape), `FacetMap`.
  - **`EnsureIndex(ctx)`** creates the `documents` index (PK `id`) + applies
    settings idempotently, called once at startup: searchable `title`,`body`
    (title first → higher relevance via the `attribute` ranking rule);
    filterable `repo`,`repo_id`,`type`,`status`,`author` (`repo_id` for the
    authorize `repo_id IN […]` filter); sortable `created`,`updated_at`. FIFO
    per-index task ordering means the enqueued create runs before the settings
    update (fresh index gets its PK); on an existing index the create task fails
    harmlessly (never waited on).
  - **meilisearch-go usage**: use the `…WithContext` API variants everywhere
    (`CreateIndexWithContext`, `UpdateSettingsWithContext`,
    `HealthWithContext`, `WaitForTaskWithContext`, later `AddDocumentsWithContext`
    /`DeleteDocumentsWithContext`/`SearchWithContext`) — `contextcheck` +
    revive `unused-parameter` require the ctx be threaded, not dropped.
    `WaitForTask` only errors on ctx-cancel/fetch-fail, so `waitTask` inspects
    `Task.Status != TaskStatusSucceeded` and surfaces `Task.Error.Message`.
    `Settings.SearchableAttributes` order sets relevance priority.
  - **content-hash-gated indexing** (task 2): the store reconcile is the single
    source of "what changed" — `ReconcileResult` now carries `UpsertedDocIDs`
    /`DeletedDocIDs`, populated by `reconcileDocuments` exactly where the
    content-hash gate decides. A new `GetDocumentsByIDs` store read (`= ANY
    (@doc_ids::text[])` → sqlc param `DocIds`, returns `[]Document`) fetches the
    changed rows. `ingest.Service` broadened its store interface to `repoStore`
    (`ReconcileRepo` + `GetDocumentsByIDs`) and gained a narrow `Indexer` dep
    (`IndexDocuments`/`DeleteDocuments`, satisfied by `*search.Client`). After
    the Postgres commit, `Run`→`indexSearch`→`syncIndex` deletes removed PKs
    then indexes upserted rows via `toIndexDoc` (`internal/ingest/indexmap.go`:
    PK `<repo_id>:<doc_id>`, repo label `owner/name`, `created` `YYYY-MM-DD`,
    `updated_at` Unix secs). **Indexing is best-effort**: an index failure logs
    at error and does NOT fail the ingest (Postgres is the source of truth; the
    next reconcile re-indexes — eventual consistency, Phase 4's queue makes it
    reliable). `NewService(st, fetcher, indexer)` — pass `nil` indexer to
    disable (e2e/unit paths that don't need Meili). `IndexDocuments`/
    `DeleteDocuments` wait on their tasks for read-after-write consistency.
  - **search endpoint** (task 3): `Client.Search(ctx, *SearchParams)
    SearchResult` uses `AttributesToCrop`+`AttributesToHighlight` on `body`
    (`<em>`/`</em>`, 40-word crop) so `_formatted.body` IS the snippet; facets
    `repo`/`type`/`status`/`author`. Decode hits via **`Hits.DecodeInto`** (NOT
    the deprecated `Hits.Decode` — staticcheck SA1019); it populates the nested
    `_formatted` struct field. `buildFilter` composes `repo_id IN [ids] AND
    field = "value"`, escaping `\` and `"` in user values; **nil** AllowedRepoIDs
    disables the repo scope (library/test convenience), **empty** slice matches
    nothing (`repo_id IN [-1]`, since ids are positive serials). Set
    `req.Filter` only when non-empty (empty-string filter is invalid). httpapi:
    `Searcher` seam + `NewHandlerWithSearch(st, s)`; `Mount` registers `GET
    /api/v1/search` only when a searcher is present (nil → route absent). The
    `searchDocs` handler injects `authorize.FromContext` as `AllowedRepoIDs`
    (the route is always behind `authorize.Middleware`, so the set is present).
    `main` wires `search.New(cfg.Meili…)` → `EnsureIndex` → both the onboard
    ingest indexer and `NewHandlerWithSearch`.
  - **/readyz multi-dep** (task 4): the single `readyChecker` interface is
    replaced by a `[]namedChecker` (`{name string; check func(ctx) error}`).
    `handleReadyz` runs each, reports a per-dependency status map (sorted keys
    via `json.Marshal` → deterministic body), and returns 503 if ANY fails so
    the body names the offender. `main` wires `postgres`→`st.Ping`,
    `meilisearch`→`searchClient.Health`. `newRouter` now takes `[]namedChecker`
    (pass `nil` when only `/healthz` matters). Body shape changed from
    `{"status":"ok"}` to `{"postgres":"ok","meilisearch":"ok"}`.
  - **integration tests** (task 5): `internal/search/search_integration_test.go`
    (`//go:build integration`) spins up `getmeili/meilisearch:v1.12` via
    testcontainers (generic container, `wait.ForHTTP("/health")` 200), shared
    across cases via TestMain. Covers index+search, facet counts, `<em>`
    snippet highlight, deletion, and the repo-scope filter seam.
  - **GOTCHA — Meilisearch document ids** allow only `[a-zA-Z0-9-_]`. The
    composite primary key uses `_` as the separator (`<repo_id>_<doc_id>`, e.g.
    `1_RFC-0001`), NOT the `:` DESIGN-0001 illustrates — a `:` id makes the add
    task fail with "Document identifier … is invalid". The PK is internal to the
    index and never appears in the search response, so this is a safe deviation;
    `repo_id` is numeric so the first `_` splits the two parts unambiguously.

- **Phase 4 — Async ingestion: COMPLETE ✅** — ingest moved off the request
  path onto an asynq + Redis job queue, in-process with the API binary. All
  acceptance criteria proven by `internal/queue/queue_integration_test.go`
  (real Redis via testcontainers): a job drains through the worker, a
  five-trigger burst coalesces to one run, and shutdown drains an in-flight
  job. Task 5 (installation-token cache) is deferred to Phase 5, where the
  webhook/App-auth work lives.
  Architecture (per go-architect):
  - **`internal/queue`** owns the job contract and both ends of the queue.
    `IngestJob{InstallationID, Owner, Name, Reason}` carries **no HeadSHA** —
    the worker refetches HEAD at process time, so a debounced burst always
    ingests the latest commit ("latest-HEAD-wins" is free). `job.go` marshals
    via `encoding/json`; `repoLabel()` = `owner/name` is the coalesce key.
  - **`Client`** (`client.go`) wraps both `asynq.Client` (enqueue) and a
    go-redis client (`Ping` for `/readyz`), parsing the one `redisURL` for
    both. `EnqueueIngest` sets **`TaskID = "ingest:" + owner/name`** (dedup key)
    + **`ProcessIn(debounce)`** (schedules into the future so repeat triggers
    within the window collapse onto the pending task) + `MaxRetry(5)` +
    `Retention(24h)`. `asynq.ErrTaskIDConflict`/`ErrDuplicateTask` are the
    coalesce signal → treated as success (nil error). `Enqueuer` interface +
    `var _ Enqueuer = (*Client)(nil)` so `main`/onboard depend on the seam.
    `Close()` joins the asynq + redis close errors via `errors.Join`.
  - **`Worker`** (`worker.go`) wraps `asynq.Server` (holds a pointer — must not
    be copied). `NewWorker(redisURL, concurrency, ing)`; `Start()` is
    non-blocking (registers `handleIngest` on a `ServeMux`, calls
    `srv.Start`), `Shutdown()` drains in-flight handlers. The `Ingestor`
    interface (`Run(ctx, installationID, owner, name) (store.ReconcileResult,
    error)`) matches `ingest.Service.Run` and is declared consumer-side so the
    worker tests with a fake. `handleIngest`: a malformed payload is unfixable
    → `asynq.SkipRetry`; any ingest error is returned so asynq retries with
    backoff (the content-hash gate makes retries idempotent). `isFailure`
    excludes `context.Canceled` so a shutdown re-queues the task instead of
    burning a retry.
  - **GOTCHA — `DelayedTaskCheckInterval` defaults to 5s.** asynq forwards
    scheduled (`ProcessIn`) and retry tasks to the pending queue only every 5s
    by default, so a debounced job could sit up to 5s past its window. Set to
    **`1 * time.Second`** in the worker `Config` — snappier ingestion in prod
    and it makes the debounce/drain integration tests pass within their waits.
  - **`ingestRunner`** (`cmd/docz-api/runner.go`) is the production `Ingestor`.
    It builds a **per-installation** `githubapp` client for each job (from
    `config.GitHubConfig` — `AppID`, `PrivateKey.Reveal()`, `APIBase`,
    `installationID`), then runs `ingest.NewService(store, ghClient,
    indexer).Run(...)`. This adapter avoids a `RepoFetcher.Fetch(installationID)`
    interface refactor: one worker serves every installation, building the
    client per job (cheap — ghinstallation caches the JWT; jobs are infrequent).
  - **in-process worker + wiring** (`cmd/docz-api/main.go`): `run()` builds one
    `queue.NewClient(cfg.Store.RedisURL, cfg.Ingest.Debounce)`. The
    `-onboard` path calls `runOnboard(ctx, st, enq queue.Enqueuer, spec)` —
    `UpsertInstallation` (synchronous) then `EnqueueIngest` (Reason `"onboard"`).
    The serve path builds the `ingestRunner`, `queue.NewWorker(..,
    workerConcurrency=2, ..)`, `worker.Start()`, and a router with **three**
    `namedChecker`s (`postgres`→`st.Ping`, `meilisearch`→`search.Health`,
    `redis`→`queueClient.Ping`). `serveWithWorker` drains in order: HTTP
    first (stop accepting requests/enqueues) → `worker.Shutdown()` (drain
    in-flight) → `closeQueueClient` — so no enqueue races the drain.

- **Phase 5 — GitHub App onboarding + webhooks: COMPLETE ✅** — install-driven
  onboarding and HMAC-verified webhooks now drive ingestion; the `-onboard` flag
  remains as a manual fallback. All acceptance criteria met; the new store SQL
  and index purge are proven by integration tests (real Postgres + Meilisearch).
  Architecture (per go-architect):
  - **`internal/webhook`** owns HMAC verification, event routing, and delivery
    idempotency only — business logic is delegated through unexported consumer
    interfaces (`webhookStore`/`enqueuer`/`indexPurger`, satisfied by
    `*store.Store`/`*queue.Client`/`*search.Client`). `Handler` is an
    `http.Handler`; `New(secret, store, enq, purger)` (nil purger disables index
    cleanup in tests). `ServeHTTP` flow: read raw body **once** (HMAC is over the
    exact bytes GitHub signed) with a 5 MiB `MaxBytesReader` cap → `verifyHMAC`
    (`hmac.Equal` constant-time, fails closed on bad/missing/malformed
    `X-Hub-Signature-256`) → `401` on mismatch **before any work** → dedupe via
    `RecordDelivery(X-GitHub-Delivery)` (`200` no-op on replay) → `ParseWebHook`
    (`go-github` v88 typed events; first arg is the `X-GitHub-Event` header, NOT
    the media type) → `route` → `202`.
  - **payloads via `go-github`** (`github.ParseWebHook`): `InstallationEvent`,
    `InstallationRepositoriesEvent`, `PushEvent`, `ReleaseEvent`. Onboarding
    reads the repo list **from the event payload** (`ev.Repositories` /
    `RepositoriesAdded`), deriving `owner`/`name` by splitting `full_name` — no
    `GET /installation/repositories` call (payload is complete at homelab scale).
    App auth is reused via the existing per-installation `githubapp.Client` that
    the worker builds per job.
  - **onboarding / offboarding** (`events.go`): `installation` created → upsert
    installation + enqueue an ingest per repo (Reason `onboard`); `deleted` →
    `ListRepoIDsByInstallation` (collect ids **before** delete) →
    `DeleteInstallation` (CASCADE wipes repos/doc_types/documents) → purge each
    repo from Meili by `repo_id` filter (best-effort). `installation_repositories`
    added → upsert + enqueue (Reason `repo_added`); removed → `DeleteRepo` +
    purge. `.docz.yaml` detection is left to the ingest worker (a repo with no
    manifest fails `Fetch` and is logged; no pre-check / `configured` flag).
  - **push handling**: `shouldIngest` requires the default branch
    (`refs/heads/<default_branch>`) AND a changed path (union of every commit's
    added/modified/removed — not just `head_commit`) equal to `.docz.yaml` or
    under `docs_dir/`. On a match it enqueues a **full** re-ingest (Reason
    `push`, no HeadSHA — worker refetches HEAD). The reconcile's content-hash
    gate + desired-state replace already do diff/delete/`doc_types` reconcile
    idempotently, so full re-ingest is correct; **narrow blob fetches are
    deferred** (a fetch-cost optimization only). A push for an un-onboarded repo
    (GetRepo → `ErrNoRows`) is skipped. `release` is **log-only** (`logRelease`;
    versions feature deferred).
  - **idempotency store surface** (`store.go` + `queries/deliveries.sql`):
    `RecordDelivery(ctx, id, event) (isNew bool, err error)` backed by `INSERT …
    ON CONFLICT (delivery_id) DO NOTHING RETURNING` — a conflicting insert
    returns no row (`pgx.ErrNoRows`) → mapped to `isNew=false`. Offboard surface:
    `DeleteInstallation`, `DeleteRepo(owner,name) (id, err)` (returns id for the
    index purge; `ErrNoRows` = already absent), `ListRepoIDsByInstallation`.
    `search.Client.DeleteRepoDocuments(repoID)` uses
    `DeleteDocumentsByFilterWithContext("repo_id = <id>")`.
  - **wiring** (`main.go`): `POST /webhooks/github` mounts on the **root** router
    (inherits `RequestID`/`Recoverer`, but NOT `/api/v1`'s `authorize`
    middleware — HMAC is the auth). `webhook.New([]byte(cfg.GitHub.WebhookSecret
    .Reveal()), st, queueClient, searchClient)`.
  - **GOTCHA — `github.ParseWebHook(event, body)`**: the first arg is the
    `X-GitHub-Event` header value (`"push"`, `"installation"`, …), NOT the
    Content-Type media type. `github.ValidatePayload`/`ValidateSignature` exist
    but we hand-roll `verifyHMAC` to satisfy the "constant-time via `hmac.Equal`"
    requirement and keep the taint out of a shared helper.
  - **GOTCHA — gosec G706** (log injection) fires on `slog` calls that log
    webhook payload fields. It is a false positive for structured logging (slog
    escapes attribute values); excluded globally in `.golangci.yml` with a
    justification, alongside a `_test.go` `unparam` relaxation (test helpers keep
    intent-documenting params).

- **Phase 6 — Authentication (pluggable providers + Redis sessions): COMPLETE
  ✅** — site users log in via one `Provider` abstraction (GitHub default, plus
  discovery-driven Okta/Keycloak) and carry an opaque Redis-backed session.
  Authorization stays the pass-through seam (Decision 10) — now keyed off a real
  identity. Architecture:
  - **`internal/auth`** — `Provider` interface (`Name`/`AuthCodeURL`/`Exchange`)
    returning an `Identity{Provider, Subject, Email, Login, Groups}`.
    `GitHubProvider` (OAuth via `golang.org/x/oauth2`; `Exchange` pulls the user
    with go-github and requires a **primary + verified** email). `OIDCProvider`
    backs **both** Okta and Keycloak — they differ only by issuer/credentials;
    discovery (`oidc.NewProvider`) runs **at startup** under a bounded context so
    a bad issuer fails the boot, not the first login, and `Exchange` verifies the
    `id_token` (JWKS signature + audience + issuer + expiry via go-oidc defaults)
    before reading claims, dropping an email the issuer marks
    `email_verified:false`. `Registry` is a name→provider map with sorted
    `Names()`.
  - **stateless CSRF via signed state** (`auth/state.go`): the OAuth `state` is
    `base64url(payload).hex(HMAC-SHA256(secret, payload))` where payload carries
    `{Provider, Nonce, ExpiresAt}` (5-min TTL). `VerifyState` is constant-time
    (`hmac.Equal`), fails closed on any tamper/expiry, and is checked **before**
    the payload is decoded. The signing secret is `SESSION_SECRET`. **No
    server-side state row** — the provider is recovered from the verified state.
  - **`internal/session`** — opaque **32-byte `crypto/rand`** session id (the id
    is the *only* credential; no identity is encoded in it). `sess:<id>` → JSON
    identity in Redis with a `SESSION_TTL` expiry. `Issue`/`Lookup`/`Revoke`
    (`redis.Nil` → `ErrSessionNotFound` via `errors.Is`). `SetCookie` is
    `HttpOnly` + `SameSite=Lax` (Lax, so the cookie survives the top-level
    provider redirect back to `/auth/callback`) + `Secure`-when-https +
    `Path:/`; `ClearCookie` mirrors it with `MaxAge:-1`. The store owns its
    **own** Redis client (separate from the queue's), closed via `closeSession`.
    `Middleware` resolves the cookie → `Lookup` → injects `Session` into the
    request context (or `401`); `FromContext` reads it back.
  - **`internal/authhttp`** — four endpoints over unexported consumer interfaces
    (`userUpserter`/`sessionStore`, satisfied by `*store.Store`/`*session.Store`).
    `MountPublic`: `GET /auth/login` (sign state → redirect to `AuthCodeURL`),
    `GET /auth/callback` (verify state → `Exchange` → `UpsertUser` → `Issue` →
    `SetCookie` → redirect `/`). `MountAPI` (behind the gate): `GET
    /api/v1/auth/session` (current user or `401`), `POST /api/v1/auth/logout`
    (`Revoke` + `ClearCookie`, idempotent). `UpsertUser` (`users.sql`) is `INSERT
    … ON CONFLICT (provider,subject) DO UPDATE`.
  - **the `/api/v1` gate** (`main.go` `runServer`): `session.Middleware` composed
    **over** `authorize.Middleware` — session runs **first** so authorization
    resolves behind a real identity. `authHandler.MountAPI` is threaded into the
    gated group via `httpapi.Handler.Mount(r, gate, extras…)`; `MountPublic` sits
    on the root router (login has no session yet; callback is authed by its
    state). The webhook receiver stays outside both (HMAC is its auth).
  - **config**: `AUTH_REDIRECT_BASE` (required, trailing-slash-trimmed) builds
    each provider's absolute `/auth/callback` `redirect_uri`; `cookiesSecure`
    keys `Secure` off its `https` scheme. Providers built by
    `cmd/docz-api/auth.go` `buildAuthProviders` (OIDC discovery bounded by
    `oidcDiscoveryTimeout=15s`).
  - **GOTCHA — funlen on `run()`**: wiring the auth stack + worker + router
    pushed `run()` over the 50-statement `funlen` limit; the serve path is
    extracted into `runServer(cfg, st, searchClient, queueClient)` (the
    `-onboard` early-return stays in `run()`).
  - **GOTCHA — `Issue(*auth.Identity)`**: `auth.Identity` is ~88 B, so gocritic
    `hugeParam` flags passing it by value; `session.Store.Issue` and the
    `sessionStore` interface take a **pointer**. Same for `session.Middleware`
    fakes (pointer receivers, `&fakeLookuper{}`).
  - **new deps**: `golang.org/x/oauth2` + `github.com/coreos/go-oidc/v3` (direct);
    `github.com/go-jose/go-jose/v4` (indirect, go-oidc's transitive). Promote via
    the go.mod direct-require block + `go mod edit -fmt`, never a bare
    `go mod tidy`.
  - **deferred (documented)**: OIDC `nonce` binding (the signed state already
    guards the code flow's CSRF; `statePayload` already carries a nonce, so it's a
    cheap hardening follow-up).

- **Phase 7 — Hardening, deploy, contract, observability: COMPLETE ✅** — the
  docz `v0.5.0` pin is confirmed (no `replace`); the read+search wire contract is
  frozen by golden fixtures (`internal/httpapi/contract_test.go`,
  `testdata/contract/*.json`, `-update` to regenerate); the distroless image
  builds/runs and `deploy/` is the reference stack; the full OQ 8 observability
  stack is wired; the error/TODO audit is clean; coverage reviewed (78% internal
  aggregate, 7/13 packages ≥80%; sub-80% is network-bound provider/fetch code +
  the cross-covered store + the composition root, no CI coverage gate).
  Observability architecture (per go-architect):
  - **`internal/telemetry`** is the single observability package.
    `Setup(ctx, Config) (shutdown, error)` installs the **global** W3C
    `TextMapPropagator` (TraceContext + Baggage) always, and — only when
    `OTEL_EXPORTER_OTLP_ENDPOINT` is set — a batching `sdktrace.TracerProvider`
    exporting over **OTLP/HTTP** (`otlptracehttp`, `WithInsecure`, host:port →
    `/v1/traces` on :4318). Empty endpoint ⇒ the global no-op tracer stays: spans
    are created but not exported, so a collector-less homelab pays ~zero overhead
    and needs no config. `Setup` does **no** network I/O (the HTTP exporter
    connects lazily on first export), so it never blocks startup.
  - **decision — metrics via `prometheus/client_golang`, OTel for traces only.**
    Metrics are package-level `promauto` instruments on the **default registry**
    (idiomatic; also exposes Go-runtime/process collectors). RED for HTTP
    (`docz_api_http_requests_total`, `..._request_duration_seconds`) and ingest
    (`docz_api_ingest_jobs_total{reason,status}`, `..._job_duration_seconds` with
    **wide buckets** up to 120s for slow GitHub-bound ingests). `ObserveIngest` is
    the one exported metric helper (called by the queue worker).
  - **HTTP middleware** (mounted in `newRouter`, order
    `RequestID → RequestLogger → Instrument → Recoverer` so a recovered panic is
    still logged + metered as 500): `RequestLogger` emits one structured slog line
    per request; `Instrument` starts a server span and records HTTP metrics. Both
    **skip** `/healthz`, `/readyz`, `/metrics` (`skipPaths`). The span name and
    the metric `route` label use chi's **matched route template**
    (`chi.RouteContext(r.Context()).RoutePattern()`, read **after**
    `next.ServeHTTP`), never the expanded URL — bounded cardinality; falls back to
    `"unmatched"`. 5xx ⇒ `span.SetStatus(codes.Error, …)`.
  - **trace propagation across the asynq boundary** (`internal/queue/job.go`):
    `IngestJob` carries `traceparent`/`tracestate` (`json:",omitempty"`).
    `injectTrace` (called in `EnqueueIngest` before marshal) writes the active
    span's W3C context via `otel.GetTextMapPropagator().Inject` into a
    `propagation.MapCarrier`; `extractTrace` (called in `handleIngest`) rebuilds
    the remote parent. The worker starts a `queue.ingest` **consumer** span; the
    ingest pipeline starts `ingest.run` with `ingest.fetch` / `ingest.reconcile`
    / `ingest.index` child spans. So webhook request → enqueue → worker → ingest
    is **one trace**.
  - **GOTCHA — global tracer/propagator are safe before `Setup`**: package-level
    `var tracer = otel.Tracer(...)` in `queue`/`ingest` and
    `otel.GetTextMapPropagator()` return **delegating** globals that retroactively
    bind to the real provider when `Setup` calls `SetTracerProvider` /
    `SetTextMapPropagator`. Never nil, so tests and the `-onboard` path (no
    `Setup`) just no-op.
  - **`/metrics`** is mounted in `runServer` (not `newRouter`) gated on
    `cfg.Telemetry.MetricsEnabled`, alongside the probes and **outside** the auth
    gate (pull-based, scraped internally). `run()` calls `telemetry.Setup` right
    after the logger and `defer shutdownTelemetry(...)` (bounded by
    `shutdownTimeout`), covering every return path.
  - **new deps** (all otel v1.43.0 line): `go.opentelemetry.io/otel/sdk`,
    `.../exporters/otlp/otlptrace/otlptracehttp`, `.../otel` + `.../otel/trace`
    (direct); `github.com/prometheus/client_golang`. `grpc` rides along
    transitively via `proto/otlp` even with the HTTP exporter. **Offline `go mod
    tidy` fails** (some dependencies' *test* deps aren't cached); settle go.sum
    with targeted `GOPROXY=off go get <pkg>@<ver>` instead.
  - **deploy** (`deploy/`): `deploy/compose.yaml` is the full reference stack
    (service + Postgres + Redis + Meili, health-gated, only `:8080` published);
    the repo-root `compose.yaml` stays deps-only for `just run`. Config is an
    env store (gitignored `deploy/.env.production`, template committed); the
    GitHub App private key is a mounted Docker **secret** referenced by path
    (`GITHUB_APP_PRIVATE_KEY` accepts a path or a PEM body). The distroless image
    has **no shell**, so there is no in-container healthcheck for the service —
    orchestrators probe `/healthz` + `/readyz` over HTTP.

## OpenAPI contract (DESIGN-0002 / IMPL-0002)

A machine-readable OpenAPI 3.1 contract for the `/api/v1` surface, kept honest by
an in-process `kin-openapi` test. Built per `docs/design/0002-*.md` (Draft) and
`docs/impl/0002-*.md` (the phased plan). Phase 1 (spec foundation + read/search
contract test) and Phase 2 (full auth/webhook surface + golden fixtures retired)
are **COMPLETE ✅**; Phases 3–4 (serve `/openapi.yaml`, consumer coordination)
are pending.

- **Spec at `api/openapi.yaml`** (OAS 3.1.0), hand-authored (not generated). It is
  embedded by the tiny **`api` package** (`api/spec.go`: `//go:embed openapi.yaml`
  → `var Spec []byte`) so both the runtime server (later) and the contract test
  consume the **same bytes**. Root-level `api/` is deliberate (fleet convention +
  the file the docz-site vendors); it is not under `internal/` because the wall
  governs Go-import visibility, and this artifact is consumed by file path/HTTP.
  It covers the **whole consumer-facing surface**: the read/search `/api/v1`
  routes, the auth endpoints (`/api/v1/auth/session` + `/logout`, public
  `/auth/login` + `/auth/callback` 302s), and the HMAC `/webhooks/github`
  receiver. Operational routes (`/openapi.yaml`, `/healthz`, `/readyz`,
  `/metrics`) stay **out** (OQ-6a).
- **Contract test** `internal/httpapi/openapi_contract_test.go` (`kin-openapi
  v0.135.0`, a **direct test-path dep**) — the **sole** wire-contract gate (the
  byte-frozen golden fixtures were retired at parity in Phase 2):
  `loadContractSpec` (`LoadFromData(api.Spec)` → `doc.Validate` →
  `gorillamux.NewRouter`) + `buildContractHandler` (wires the **full** stack
  exactly as `runServer` — read/search + gated auth behind the real
  `session.Middleware` ∘ `authorize.Middleware`, the public redirects, and the
  webhook — with in-package fakes) + `validateRoundTrip` (`openapi3filter`
  request + response, `MultiError`, snapshots the body so it survives both). **No
  build tag** — it rides `just test` / CI `Test Go`. Security is a no-op via
  `openapi3filter.NoopAuthenticationFunc` (the middleware runs for real with a
  fake session; the test asserts schemas, not the auth mechanism).
- **Schemas mirror the DTOs 1:1** (`internal/httpapi/dto.go`,
  `internal/search/types.go`, `internal/authhttp/handler.go`'s `sessionDTO`) with
  **`additionalProperties: false`** everywhere — that strictness is the drift
  detector (an added/renamed field fails the test). Nullable columns are empty
  strings (never `null`); JSONB arrays are `[]`. The read/search error envelope
  stays `{"error": string}` and list responses stay envelope objects
  (`{"repos":[…]}`); the **webhook + logout** use a separate `{"status": string}`
  envelope (`StatusResponse`). RFC 7807 + bare arrays are deferred (FU-1/FU-2).
- **Security model:** the top-level default is `sessionCookie` (apiKey in the
  `docz_session` cookie); `/api/v1/*` inherits it, and the three public routes
  (`login`, `callback`, `githubWebhook`) override with `security: []`. The webhook
  HMAC-SHA256 scheme is documented in the op `description` (OpenAPI has no
  first-class HMAC-body scheme).
- **Spec lint/format (OQ-7b):** `just lint-openapi` runs **`vacuum`** (`aqua:
  daveshanley/vacuum`, pinned in `mise.toml`) against `api/vacuum-ruleset.yaml`
  (`-n warn`, fails on warnings) + `yamlfmt -lint`; `just fmt` yamlfmt-
  canonicalizes the spec. The ruleset disables **`camel-case-properties`** (the
  wire contract is snake_case by design) plus two over-strict-for-us rules, with
  rationale in-file; the spec scores **100/100**. CI's Lint job runs
  `just lint-openapi` behind a mise step.
- **Dep-settling gotcha:** `go get kin-openapi@v0.135.0` alone does **not** pull
  the `openapi3`/`openapi3filter`/`routers/gorillamux` subpackages' transitive
  `go.sum` entries (nothing imports them yet). Settle them with targeted `go get
  <subpkg>@v0.135.0` (or a `go mod tidy` once the harness imports them, which also
  promotes `kin-openapi` to a direct require).

## Renovate

- `go.mod` updates are PR'd by Renovate's Go module manager.
- Container base images in `Dockerfile` are PR'd by the Docker manager.
- `mise.toml` versions are handled by a custom regex manager configured upstream
  in `donaldgifford/renovate-config`.
