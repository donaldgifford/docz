# docz-api — the server's recipes
#
# Loaded by the root justfile as the optional module `api` (DESIGN-0016 OQ 1):
# run these as `just api <recipe>`, and a root gate reaches one as
# `api::<recipe>`. The file sits at the repository root, so every path below
# is root-relative. Release tagging, the licence check, and the ci/check gates
# are the root's (docz.just and justfile) — the module is one binary, one image,
# one chart.

set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

project_name      := "docz-api"
project_owner     := "donaldgifford"
go_package        := "github.com/" + project_owner + "/docz/v2"
build_dir         := "build"
bin_dir           := build_dir + "/bin"
coverage_out      := "coverage.out"
goimports_local   := "github.com/" + project_owner

# Version info derived from git; falls back to dev when not in a repo or tag-less.
commit_hash := `git rev-parse --short HEAD 2>/dev/null || echo unknown`
version     := `git describe --tags --always --dirty 2>/dev/null || echo dev`

# The server's packages: what test and lint cover here. The root gates cover
# the whole module.
server_pkgs       := "./cmd/docz-api/... ./internal/... ./api/..."

# Default: list this module's recipes
_default:
    @just --list api --unsorted

# ─── Build ──────────────────────────────────────────────────────────

# Build the server
[group('build')]
build: build-core

# Build the server binary into build/bin/docz-api
[group('build')]
build-core:
    @mkdir -p {{ bin_dir }}
    @go build -ldflags "-X main.version={{ version }} -X main.commit={{ commit_hash }}" \
        -o {{ bin_dir }}/{{ project_name }} ./cmd/{{ project_name }}
    @echo "✓ Core binaries built"

# Remove the server binary (the root's `just clean` owns build/ and the cache)
[group('build')]
clean:
    @rm -f {{ bin_dir }}/{{ project_name }}
    @echo "✓ Cleaned the docz-api binary"

# ─── Run ────────────────────────────────────────────────────────────

# Build then run the server
[group('run')]
run: build
    @{{ bin_dir }}/{{ project_name }}

# Build then run the server from the local bin
[group('run')]
run-local: build
    @{{ bin_dir }}/{{ project_name }}

# ─── Dev stack ──────────────────────────────────────────────────────

# Start the local dependency stack (Postgres, Redis, Meilisearch) and wait for health
[group('dev')]
dev-up:
    @docker compose up -d --wait
    @echo "✓ Dev stack up (postgres :5432, redis :6379, meilisearch :7700)"

# Stop the dev stack (tunnel included); volumes are kept (see dev-nuke)
[group('dev')]
dev-down:
    @docker compose --profile tunnel down
    @echo "✓ Dev stack stopped"

# Stop the dev stack (tunnel included) and wipe its volumes
[group('dev')]
dev-nuke:
    @docker compose --profile tunnel down -v
    @echo "✓ Dev stack stopped, volumes wiped"

# Show dev stack status and health
[group('dev')]
dev-ps:
    @docker compose --profile tunnel ps

# Follow dev stack logs
[group('dev')]
dev-logs:
    @docker compose --profile tunnel logs -f

# Expose host :8080 via ngrok for GitHub webhooks (needs NGROK_AUTHTOKEN in .env)
[group('dev')]
dev-tunnel:
    @docker compose --profile tunnel up -d ngrok
    @for i in 1 2 3 4 5 6 7 8 9 10; do \
        url=$(curl -fsS localhost:4040/api/tunnels 2>/dev/null | jq -r '.tunnels[0].public_url // empty'); \
        if [ -n "$url" ]; then echo "✓ Webhook URL: $url/webhooks/github"; exit 0; fi; \
        sleep 1; \
    done; \
    echo "✗ Tunnel not up after 10s — check 'just api dev-logs' and http://localhost:4040"; exit 1

# ─── Local environment (full stack in containers) ─────────────────

local_compose := "docker compose -f deploy/api/compose.local.yaml --env-file deploy/api/.env.local"

# Build + start the full local env: service, deps, ngrok (needs deploy/.env.local)
[group('local')]
local-up:
    @test -f deploy/api/.env.local || { echo "✗ deploy/api/.env.local missing — cp deploy/api/.env.local.example deploy/api/.env.local and fill it in"; exit 1; }
    @{{ local_compose }} up -d --build --wait
    @for i in 1 2 3 4 5 6 7 8 9 10; do \
        url=$(curl -fsS localhost:4040/api/tunnels 2>/dev/null | jq -r '.tunnels[0].public_url // empty'); \
        if [ -n "$url" ]; then echo "✓ Local env up — webhook URL: $url/webhooks/github"; exit 0; fi; \
        sleep 1; \
    done; \
    echo "✓ Local env up — tunnel still starting, check http://localhost:4040"

# Stop the local env; volumes are kept (see local-nuke)
[group('local')]
local-down:
    @{{ local_compose }} down
    @echo "✓ Local env stopped"

# Stop the local env and wipe its volumes
[group('local')]
local-nuke:
    @{{ local_compose }} down -v
    @echo "✓ Local env stopped, volumes wiped"

# Show local env status and health
[group('local')]
local-ps:
    @{{ local_compose }} ps

# Follow local env logs (all services)
[group('local')]
local-logs:
    @{{ local_compose }} logs -f

# ─── Monitoring stack ───────────────────────────────────────────────

monitor_compose := "docker compose -f deploy/api/compose.monitoring.yaml"

# Start the local observability backends (prometheus/grafana/jaeger/loki/otel/alloy)
[group('monitor')]
monitor-up:
    @{{ monitor_compose }} up -d --wait
    @echo "✓ Monitoring stack up:"
    @echo "    grafana     http://localhost:3000  (anonymous admin)"
    @echo "    prometheus  http://localhost:9090"
    @echo "    jaeger      http://localhost:16686"
    @echo "    loki        http://localhost:3100"
    @echo "    otel OTLP   http://localhost:4318  (set OTEL_EXPORTER_OTLP_ENDPOINT to this)"
    @echo "    alloy       http://localhost:12345"

# Start the monitoring stack with keycloak for local OIDC login testing
[group('monitor')]
monitor-auth-up:
    @{{ monitor_compose }} --profile auth up -d --wait
    @echo "✓ Monitoring stack up (+ keycloak):"
    @echo "    grafana     http://localhost:3000  (anonymous admin)"
    @echo "    prometheus  http://localhost:9090"
    @echo "    jaeger      http://localhost:16686"
    @echo "    keycloak    http://localhost:8180  (realm docz-api; user dev/dev-password)"

# Stop the monitoring stack (keycloak included); volumes are kept
[group('monitor')]
monitor-down:
    @{{ monitor_compose }} --profile auth down
    @echo "✓ Monitoring stack stopped"

# Follow monitoring stack logs (all services)
[group('monitor')]
monitor-logs:
    @{{ monitor_compose }} --profile auth logs -f

# ─── Docker ─────────────────────────────────────────────────────────
# Image builds via docker buildx bake; docker-bake.hcl builds Dockerfile.api.
# Folded in from docker.just (IMPL-0019 OQ 4).

# Local single-arch build via docker buildx bake
[group('docker')]
docker-build:
    @docker buildx bake

# Multi-arch build and push via docker buildx bake
[group('docker')]
docker-buildx:
    @docker buildx bake release

# Push the image (requires prior build)
[group('docker')]
docker-push:
    @docker buildx bake --push

# CI build (linux/amd64 only, for fast PR validation)
[group('docker')]
docker-ci:
    @docker buildx bake ci

# ─── Helm ───────────────────────────────────────────────────────────

# Lint the chart (ci-values supplies the required values with no defaults)
[group('helm')]
helm-lint:
    @helm lint charts/docz-api -f charts/docz-api/ci/ci-values.yaml

# Render the chart with the ci values (fast "does it template" check)
[group('helm')]
helm-template:
    @helm template docz-api charts/docz-api -f charts/docz-api/ci/ci-values.yaml

# Run the chart's helm-unittest suite (needs the helm-unittest plugin)
[group('helm')]
helm-unittest:
    @helm unittest charts/docz-api

# Regenerate the chart README from README.md.gotmpl + values.yaml
[group('helm')]
helm-docs:
    @helm-docs --chart-search-root=charts

# ─── Test ───────────────────────────────────────────────────────────

# Run all tests with the race detector
[group('test')]
test:
    @go test -v -race {{ server_pkgs }}

# Run tests for a single package: just api test-pkg ./internal/ingest
[group('test')]
test-pkg pkg:
    @go test -v -race {{ pkg }}

# Run integration tests (build tag `integration`; needs Docker for testcontainers)
[group('test')]
test-integration:
    @go test -tags=integration -count=1 {{ server_pkgs }}

# Run tests with a coverage profile written to coverage.out
[group('test')]
test-coverage:
    @go test -v -race -coverprofile={{ coverage_out }} {{ server_pkgs }}

# Run tests and open the HTML coverage report
[group('test')]
test-report:
    @go test -coverprofile={{ coverage_out }} {{ server_pkgs }}
    @go tool cover -html={{ coverage_out }}

# ─── Lint & format ─────────────────────────────────────────────────

# Run golangci-lint
[group('lint')]
lint:
    @golangci-lint run {{ server_pkgs }}

# Run golangci-lint with --fix
[group('lint')]
lint-fix:
    @golangci-lint run --fix {{ server_pkgs }}

# Verify the golangci-lint configuration
[group('lint')]
lint-config:
    @golangci-lint config verify

# Lint GitHub Actions workflows
[group('lint')]
lint-actions:
    @actionlint

# Validate the Prometheus alert rules (contrib pack)
[group('lint')]
lint-alerts:
    @promtool check rules contrib/prometheus/alerts.yaml

# Lint the OpenAPI spec (vacuum ruleset) and verify its canonical formatting
[group('lint')]
lint-openapi:
    @vacuum lint -d -n warn -r api/vacuum-ruleset.yaml api/openapi.yaml
    @yamlfmt -lint api/openapi.yaml api/vacuum-ruleset.yaml

# Format code with gofmt + goimports; canonicalize the OpenAPI spec with yamlfmt
[group('lint')]
fmt:
    @gofmt -s -w cmd/docz-api internal api
    @goimports -w -local {{ goimports_local }} cmd/docz-api internal api
    @yamlfmt api/openapi.yaml api/vacuum-ruleset.yaml

# ─── Codegen ────────────────────────────────────────────────────────

# Regenerate typed DB access from SQL (sqlc reads internal/store/queries)
[group('codegen')]
generate:
    @sqlc generate
    @echo "✓ sqlc generate complete"

# Verify committed sqlc output is up to date with the SQL sources
[group('codegen')]
generate-check:
    @sqlc diff
    @echo "✓ sqlc output is up to date"

# ─── Release ────────────────────────────────────────────────────────

# Validate the goreleaser config
[group('release')]
release-check:
    @goreleaser check

# Snapshot release locally (no publish, no sign)
[group('release')]
release-local:
    @goreleaser release --snapshot --clean --skip=publish --skip=sign
