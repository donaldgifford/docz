# docz-api — task runner
#
# Project automation via just. Use either the Makefile or this justfile —
# both expose the same target set with equivalent behavior.

set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

project_name      := "docz-api"
project_owner     := "donaldgifford"
go_package        := "github.com/" + project_owner + "/" + project_name
build_dir         := "build"
bin_dir           := build_dir + "/bin"
coverage_out      := "coverage.out"
allowed_licenses  := "Apache-2.0,MIT,BSD-2-Clause,BSD-3-Clause,ISC,MPL-2.0"
goimports_local   := "github.com/" + project_owner

# Version info derived from git; falls back to dev when not in a repo or tag-less.
commit_hash := `git rev-parse --short HEAD 2>/dev/null || echo unknown`
version     := `git describe --tags --always --dirty 2>/dev/null || echo dev`

# Default: list recipes
_default:
    @just --list --unsorted

# ─── Build ──────────────────────────────────────────────────────────

# Build everything (core)
[group('build')]
build: build-core

# Build the core CLI binary into build/bin/docz-api
[group('build')]
build-core:
    @mkdir -p {{ bin_dir }}
    @go build -ldflags "-X main.version={{ version }} -X main.commit={{ commit_hash }}" \
        -o {{ bin_dir }}/{{ project_name }} ./cmd/{{ project_name }}
    @echo "✓ Core binaries built"

# Remove build artifacts and the Go build cache
[group('build')]
clean:
    @rm -rf {{ bin_dir }}/
    @rm -f {{ coverage_out }}
    @go clean -cache
    @find . -name "*.test" -delete
    @echo "✓ Cleaned build artifacts"

# ─── Run ────────────────────────────────────────────────────────────

# Build then run the CLI
[group('run')]
run: build
    @{{ bin_dir }}/{{ project_name }}

# Build then run the CLI from the local bin
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
    echo "✗ Tunnel not up after 10s — check 'just dev-logs' and http://localhost:4040"; exit 1

# ─── Local environment (full stack in containers) ─────────────────

local_compose := "docker compose -f deploy/compose.local.yaml --env-file deploy/.env.local"

# Build + start the full local env: service, deps, ngrok (needs deploy/.env.local)
[group('local')]
local-up:
    @test -f deploy/.env.local || { echo "✗ deploy/.env.local missing — cp deploy/.env.local.example deploy/.env.local and fill it in"; exit 1; }
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

# ─── Test ───────────────────────────────────────────────────────────

# Run all tests with the race detector
[group('test')]
test:
    @go test -v -race ./...

# Run all tests (core + plugins)
[group('test')]
test-all: test

# Run tests for a single package: just test-pkg ./pkg/foo
[group('test')]
test-pkg pkg:
    @go test -v -race {{ pkg }}

# Run integration tests (build tag `integration`; needs Docker for testcontainers)
[group('test')]
test-integration:
    @go test -tags=integration -count=1 ./...

# Run tests with a coverage profile written to coverage.out
[group('test')]
test-coverage:
    @go test -v -race -coverprofile={{ coverage_out }} ./...

# Run tests and open the HTML coverage report
[group('test')]
test-report:
    @go test -coverprofile={{ coverage_out }} ./...
    @go tool cover -html={{ coverage_out }}

# ─── Lint & format ─────────────────────────────────────────────────

# Run golangci-lint
[group('lint')]
lint:
    @golangci-lint run ./...

# Run golangci-lint with --fix
[group('lint')]
lint-fix:
    @golangci-lint run --fix ./...

# Verify the golangci-lint configuration
[group('lint')]
lint-config:
    @golangci-lint config verify

# Lint GitHub Actions workflows
[group('lint')]
lint-actions:
    @actionlint

# Lint the OpenAPI spec (vacuum ruleset) and verify its canonical formatting
[group('lint')]
lint-openapi:
    @vacuum lint -d -n warn -r api/vacuum-ruleset.yaml api/openapi.yaml
    @yamlfmt -lint api/openapi.yaml api/vacuum-ruleset.yaml

# Format code with gofmt + goimports; canonicalize the OpenAPI spec with yamlfmt
[group('lint')]
fmt:
    @gofmt -s -w .
    @goimports -w -local {{ goimports_local }} .
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

# ─── License compliance ─────────────────────────────────────────────

# Check dependency licenses against the allow list
[group('license')]
license-check:
    @go-licenses check ./... --allowed_licenses={{ allowed_licenses }}

# Generate CSV report of all dependency licenses
[group('license')]
license-report:
    @go-licenses report ./... --template=.github/licenses-csv.tpl

# ─── Release ────────────────────────────────────────────────────────

# Validate the goreleaser config
[group('release')]
release-check:
    @goreleaser check

# Snapshot release locally (no publish, no sign)
[group('release')]
release-local:
    @goreleaser release --snapshot --clean --skip=publish --skip=sign

# Tag and push a new release: just release v0.1.0
[group('release')]
release tag:
    @git tag -a {{ tag }} -m "Release {{ tag }}"
    @git push origin {{ tag }}

# ─── Composite gates ────────────────────────────────────────────────

# Pre-commit gate: lint + test
[group('gate')]
check: lint test
    @echo "✓ Pre-commit checks passed"

# Full CI gate: lint + test + build + license-check
[group('gate')]
ci: lint test build license-check
    @echo "✓ CI pipeline complete"
