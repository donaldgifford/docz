# Developing docz-api

Everything a new developer needs to build, run, and test docz-api locally.
For deploying the full stack to a real host, see [deploy/README.md](deploy/README.md).

## Prerequisites

- **[mise](https://mise.jdx.dev/)** — manages the entire toolchain (Go,
  `just`, linters, `sqlc`, `docz`, …) at the versions pinned in `mise.toml`.
  Nothing else needs a manual install; do not install Go separately.
- **Docker** — required for the local dependency stack (`compose.yaml`) and
  the integration tests (testcontainers-go spins up real Postgres, Redis,
  and Meilisearch containers).
- **git**.

## First-time setup

```sh
git clone https://github.com/donaldgifford/docz-api.git
cd docz-api
mise trust && mise install   # installs the pinned toolchain
just                         # prints the task menu — the map of everything below
```

`just` is the single entry point for project automation. Any instruction
elsewhere that says `make <target>` maps to the `just` recipe of the same
name.

## Build

```sh
just build        # binary at build/bin/docz-api
just clean        # remove build artifacts + Go build cache
```

Version metadata (`version`, `commit`) is derived from git and injected via
`-ldflags`; `build/bin/docz-api -version` prints it.

## Run locally

The service needs Postgres, Redis, and Meilisearch. The repo-root
`compose.yaml` brings up exactly those three (dev-only credentials that
mirror `.env.example` — it does **not** run the service itself):

```sh
just dev-up     # postgres :5432, redis :6379, meilisearch :7700; waits for health
just dev-ps     # status, if you want to double-check
just dev-logs   # follow the dependency logs
```

Configuration is **environment-only** (no config file). Copy the template
and fill in the placeholders:

```sh
cp .env.example .env
```

The justfile does not auto-load `.env`, so export it into your shell before
running:

```sh
set -a && source .env && set +a
just run                          # build + run build/bin/docz-api
# or, without the build step:
go run ./cmd/docz-api
```

On startup the service applies database migrations automatically, ensures
the Meilisearch index exists, starts the in-process ingest worker, and
serves on `:8080` (`HTTP_ADDR`). Config validation reports **all** problems
in one error, so a first run with placeholder values tells you everything
that still needs filling in.

Useful flags (see `cmd/docz-api`):

```sh
docz-api -version                      # print version info and exit
docz-api -migrate                      # apply migrations and exit (CI/ops)
docz-api -onboard owner/name@<instID>  # seed + enqueue one repo ingest, then exit
```

Quick smoke checks once it is up:

```sh
curl localhost:8080/healthz        # liveness
curl localhost:8080/readyz         # per-dependency readiness (postgres/redis/meili)
curl localhost:8080/openapi.yaml   # the served API contract
curl localhost:8080/metrics        # Prometheus metrics (METRICS_ENABLED)
```

Note that everything under `/api/v1` sits behind the session gate: you need
a real auth provider configured (e.g. a GitHub OAuth app in
`GITHUB_OAUTH_CLIENT_ID`/`_SECRET`) and a browser login via
`/auth/login?provider=github` to exercise those routes. The probes, spec,
and metrics endpoints above are public. Ingestion (webhooks) additionally
needs a GitHub App (`GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY`,
`GITHUB_WEBHOOK_SECRET`); the `-onboard` flag is the manual fallback that
skips webhooks.

When you are done:

```sh
just dev-down    # stop the dependencies (volumes kept)
just dev-nuke    # stop AND wipe the data volumes — fresh databases next dev-up
```

## Run with Docker

Build the image (multi-stage, distroless, runs as `nonroot`):

```sh
docker build -t docz-api:dev \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) .
```

The first build is cold; BuildKit cache mounts (`/go/pkg/mod`,
`/root/.cache/go-build`) make subsequent builds fast. The image has no
shell — health checks must probe `/healthz` over HTTP from outside.

To run the **entire stack in compose** (service container + the three
dependencies on a private network, only `:8080` published), use the
reference deployment:

```sh
cd deploy
cp .env.production.example .env.production   # fill in real values
mkdir -p secrets && cp /path/to/app.pem secrets/github-app.pem
docker compose up -d --build
```

`deploy/README.md` covers that layout, secrets handling, and
health/observability endpoints in detail.

## Test

```sh
just test               # all unit tests, race detector
just test-pkg ./internal/ingest   # one package
just test-coverage      # writes coverage.out
just test-report        # opens the HTML coverage report
just test-integration   # build tag `integration`; needs Docker (testcontainers)
```

Integration tests are hermetic: testcontainers starts throwaway Postgres,
Redis, and Meilisearch containers per suite — they do not touch the
`compose.yaml` dev stack. Tests use the standard-library `testing` package
only (no assertion libraries); prefer table-driven tests.

## Lint and format

```sh
just lint           # golangci-lint
just lint-fix       # golangci-lint --fix
just fmt            # gofmt + goimports + yamlfmt (canonicalizes the OpenAPI spec)
just lint-openapi   # vacuum ruleset + yamlfmt -lint on api/openapi.yaml
just lint-actions   # actionlint on .github/workflows
just check          # pre-commit gate: lint + test
just ci             # full local CI gate: lint + test + build + license-check
```

## Code generation

Typed database access is generated by sqlc from `internal/store/queries/`:

```sh
just generate         # regenerate after editing SQL
just generate-check   # verify committed output matches the SQL (CI runs this)
```

Migrations live in `internal/store/migrations/` (goose, embedded into the
binary). Add a new timestamped `.sql` file with paired `-- +goose Up` /
`-- +goose Down` sections; startup (or `-migrate`) applies it.

## Project documentation

Design docs, implementation plans, and investigations live under `docs/`
and are managed with the [docz](https://github.com/donaldgifford/docz) CLI
(installed by mise):

```sh
docz list             # what exists
docz create adr "Use X for Y"
docz update           # regenerate the README index tables after edits
```

## Conventions worth knowing before your first PR

- `internal/` is a hard wall — nothing outside this module can import it.
- Structured logs via `slog`; never log raw credentials (`config.Secret`
  redacts, unwrap only via `.Reveal()`).
- Errors wrap with `%w`; no `init()` for behavior; wire dependencies in
  `main()`.
- Do **not** run a bare `go mod tidy` — promote/settle dependencies with
  targeted `go get` (see CLAUDE.md for why).
- Conventional commits; every PR needs a semver label
  (`major`/`minor`/`patch`/`dont-release`), and CI checks changelog drift
  (`git-cliff`).
- CI (GitHub Actions) runs lint, tests with coverage, security scans
  (govulncheck/Trivy/CodeQL), license checks, and a goreleaser snapshot
  build on every PR.
