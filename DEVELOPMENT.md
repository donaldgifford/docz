# Developing docz-api

Everything a new developer needs to build, run, and test docz-api locally. For
deploying the full stack to a real host, see
[deploy/README.md](deploy/README.md).

## Prerequisites

- **[mise](https://mise.jdx.dev/)** — manages the entire toolchain (Go, `just`,
  linters, `sqlc`, `docz`, …) at the versions pinned in `mise.toml`. Nothing
  else needs a manual install; do not install Go separately.
- **Docker** — required for the local dependency stack (`compose.yaml`) and the
  integration tests (testcontainers-go spins up real Postgres, Redis, and
  Meilisearch containers).
- **git**.

## First-time setup

```sh
git clone https://github.com/donaldgifford/docz-api.git
cd docz-api
mise trust && mise install   # installs the pinned toolchain
just                         # prints the task menu — the map of everything below
```

`just` is the single entry point for project automation. Any instruction
elsewhere that says `make <target>` maps to the `just` recipe of the same name.

## Build

```sh
just build        # binary at build/bin/docz-api
just clean        # remove build artifacts + Go build cache
```

Version metadata (`version`, `commit`) is derived from git and injected via
`-ldflags`; `build/bin/docz-api -version` prints it.

## Run locally

The service needs Postgres, Redis, and Meilisearch. The repo-root `compose.yaml`
brings up exactly those three (dev-only credentials that mirror `.env.example` —
it does **not** run the service itself):

```sh
just dev-up     # postgres :5432, redis :6379, meilisearch :7700; waits for health
just dev-ps     # status, if you want to double-check
just dev-logs   # follow the dependency logs
```

Configuration is **environment-only** (no config file). Copy the template and
fill in the placeholders:

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

On startup the service applies database migrations automatically, ensures the
Meilisearch index exists, starts the in-process ingest worker, and serves on
`:8080` (`HTTP_ADDR`). Config validation reports **all** problems in one error,
so a first run with placeholder values tells you everything that still needs
filling in.

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

Note that everything under `/api/v1` sits behind the session gate: you need a
real auth provider configured (`GITHUB_OAUTH_CLIENT_ID`/`_SECRET`) and a browser
login via `/auth/login?provider=github` to exercise those routes. The probes,
spec, and metrics endpoints above are public. Ingestion (webhooks) additionally
needs a GitHub App (`GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY`,
`GITHUB_WEBHOOK_SECRET`) — permissions, events, and setup steps are in
[deploy/README.md](deploy/README.md#github-app-setup-ingestion); the `-onboard`
flag is the manual fallback that skips webhooks.

For local dev, **use one GitHub App for both**: the same app that delivers
webhooks can be the OAuth login provider (callback URL + client secret + email
permission — see
[deploy/README.md](deploy/README.md#site-login-reuse-the-github-app-or-a-separate-oauth-app)),
so you only ever create and configure a single dev app.

When you are done:

```sh
just dev-down    # stop the dependencies (volumes kept)
just dev-nuke    # stop AND wipe the data volumes — fresh databases next dev-up
```

### Receiving GitHub webhooks locally (ngrok)

To develop against a **real GitHub App** — install/uninstall onboarding,
push-triggered re-ingest — GitHub must be able to deliver webhooks to your
machine. The compose file ships an [ngrok](https://ngrok.com/) service behind
the `tunnel` profile (it never starts with the normal stack):

```sh
# one-time: put NGROK_AUTHTOKEN in .env (see .env.example)
just dev-tunnel
# ✓ Webhook URL: https://<random>.ngrok-free.app/webhooks/github
```

Paste the printed URL into your GitHub App's **Webhook URL** setting (see
[deploy/README.md](deploy/README.md#github-app-setup-ingestion) for the full app
configuration). The tunnel targets the **host's** `:8080`, so start the service
(`just run`) for deliveries to land. Inspect and replay deliveries at
<http://localhost:4040>.

Free ngrok URLs are random per start; claim your free static domain and set
`NGROK_ARGS=--domain=<name>.ngrok-free.app` in `.env` so the GitHub App's
webhook URL survives restarts. `just dev-down` / `dev-nuke` stop the tunnel
along with the rest of the stack.

## Run with Docker

Build the image (multi-stage, distroless, runs as `nonroot`):

```sh
docker build -t docz-api:dev \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) .
```

The first build is cold; BuildKit cache mounts (`/go/pkg/mod`,
`/root/.cache/go-build`) make subsequent builds fast. The image has no shell —
health checks must probe `/healthz` over HTTP from outside.

### Full local environment (`just local-up`)

To test **everything containerized end to end** — the locally-built image,
webhooks arriving via ngrok, login, ingest — use the local environment compose
file (`deploy/compose.local.yaml`):

```sh
cp deploy/.env.local.example deploy/.env.local   # fill in your dev GitHub App
mkdir -p deploy/secrets && cp /path/to/dev-app.pem deploy/secrets/github-app.pem
just local-up
# ✓ Local env up — webhook URL: https://<domain>.ngrok-free.app/webhooks/github
```

`local-up` builds the image from the working tree, starts the service +
Postgres/Redis/Meilisearch + ngrok, waits for health, and prints the webhook URL
to paste into your dev GitHub App. The service is at `localhost:8080` (so the
OAuth callback stays `http://localhost:8080`), ngrok's inspector at
`localhost:4040`. Companions: `just local-ps`, `local-logs`, `local-down`,
`local-nuke`.

This environment and the host-run dev loop are **alternatives, not roommates** —
both claim `:8080`/`:4040`, so `just dev-down` before `just local-up` (and vice
versa). Rebuild + restart after code changes is just `just local-up` again.

There is also the production-shaped reference deployment in
`deploy/compose.yaml` (`.env.production`, restart policies, no tunnel) —
`deploy/README.md` covers that layout, secrets handling, and
health/observability endpoints in detail.

### Local monitoring stack (`just monitor-up`)

`deploy/compose.monitoring.yaml` runs the **observability backends only** — not
the service or its data dependencies. Pair it with the app running either on the
host (`just run`) or containerized (`just local-up`):

```sh
just monitor-up          # prometheus, grafana, jaeger, loki, otel-collector, alloy
just monitor-auth-up     # + keycloak, for local OIDC login testing
just monitor-logs        # follow all backend logs
just monitor-down        # stop everything (keycloak included); volumes kept
```

What runs where:

| Backend        | URL                      | Purpose                                           |
| -------------- | ------------------------ | ------------------------------------------------- |
| Grafana        | `http://localhost:3000`  | dashboards (anonymous admin; "docz-api overview") |
| Prometheus     | `http://localhost:9090`  | scrapes the app's `/metrics` via the host gateway |
| Jaeger         | `http://localhost:16686` | trace explorer                                    |
| Loki           | `http://localhost:3100`  | log store (fed by alloy + otel-collector)         |
| otel-collector | `http://localhost:4318`  | OTLP/HTTP ingest → jaeger (traces) + loki (logs)  |
| Alloy          | `http://localhost:12345` | tails container stdout JSON logs → loki           |
| Keycloak       | `http://localhost:8180`  | local OIDC IdP (`--profile auth` only)            |

To send the app's telemetry into the stack, set two env vars (both are in
`.env.example` under "Local monitoring stack"):

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318   # host:port, NO scheme (WithEndpoint)
LOG_FORMAT=json                              # so alloy can label trace_id/span_id
```

For the containerized app (`just local-up`), point the collector at the host
gateway instead: `OTEL_EXPORTER_OTLP_ENDPOINT=host.docker.internal:4318` in
`deploy/.env.local`. Prometheus already scrapes `host.docker.internal:8080`, so
metrics work for both run modes. After a few requests, the Grafana overview
dashboard shows request/latency/error/ingest panels and a traced request appears
in Jaeger.

**Keycloak login walkthrough** (`just monitor-auth-up`): the seeded realm
`docz-api` ships one confidential client (`docz-api` / `dev-docz-api-secret`,
redirect `http://localhost:8080/auth/callback`) and one verified user (`dev` /
`dev-password`). Enable the keycloak block in `.env` (`AUTH_PROVIDERS` includes
`keycloak`, `KEYCLOAK_ISSUER=http://localhost:8180/realms/docz-api`, matching
client id/secret), start the app, then visit
`http://localhost:8080/auth/login?provider=keycloak` and sign in as `dev` — the
callback exchanges the code, upserts the user, and issues a session cookie.

## Test

```sh
just test               # all unit tests, race detector
just test-pkg ./internal/ingest   # one package
just test-coverage      # writes coverage.out
just test-report        # opens the HTML coverage report
just test-integration   # build tag `integration`; needs Docker (testcontainers)
```

Integration tests are hermetic: testcontainers starts throwaway Postgres, Redis,
and Meilisearch containers per suite — they do not touch the `compose.yaml` dev
stack. Tests use the standard-library `testing` package only (no assertion
libraries); prefer table-driven tests.

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

Design docs, implementation plans, and investigations live under `docs/` and are
managed with the [docz](https://github.com/donaldgifford/docz) CLI (installed by
mise):

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
  (govulncheck/Trivy/CodeQL), license checks, and a goreleaser snapshot build on
  every PR.
