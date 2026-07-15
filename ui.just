# Mirrors package.json scripts as `just` recipes so commands are short
# and composable. Underlying tooling (Bun, Vite, vitest, orval) is the
# source of truth — recipes here just call into it.

# bash, not zsh — CI runners don't ship zsh.
set shell := ["bash", "-cu"]

default:
    @just --list

# Install dependencies
install:
    bun install

# Vite dev server, proxying /api,/auth,/openapi.yaml to local docz-api
dev:
    bun run dev

# Vite dev server against MSW fixtures — no docz-api required
dev-msw:
    bun run dev:msw

# Regenerate the docz-api TS client (orval -> src/api/__generated__)
gen-api:
    bun run gen-api

# Drift gate: snapshot, regenerate, diff the generated client
gen-api-check:
    ./scripts/gen-api-check.sh

# Lint (eslint flat config)
lint:
    bun run lint

# Lint with autofix
lint-fix:
    bun run lint:fix

# Prettier write
fmt:
    bun run format

# Prettier check (no write)
fmt-check:
    bun run format:check

# Strict TS check (tsc -b over the project references)
typecheck:
    bun run typecheck

# Vitest single run
test:
    bun run test

# Vitest watch
test-watch:
    bun run test:watch

# Production build (tsc -b + vite build)
build:
    bun run build

# Entry-chunk size budget (requires a fresh `just build`)
bundle-budget:
    bun scripts/bundle-budget.ts

# Playwright e2e against an MSW-enabled preview build (dist-msw/)
e2e:
    bun run e2e

# Serve the production build locally
preview:
    bun run preview

local_compose := "docker compose -f deploy/compose.local.yaml"

# Build + start docz-site (:8090) against the running docz-api local stack
local-up:
    @docker network inspect docz-api-local_default >/dev/null 2>&1 || { echo "✗ docz-api local stack not running — run 'just local-up' in ../docz-api first"; exit 1; }
    @{{ local_compose }} up -d --build --wait
    @echo "✓ docz-site up at http://localhost:8090 (proxying to docz-api-local)"

# Stop the local docz-site container
local-down:
    @{{ local_compose }} down
    @echo "✓ docz-site stopped"

# ─── Helm chart ─────────────────────────────────────────────────────

# Lint the chart (ci-values supplies the required values with no defaults)
helm-lint:
    @helm lint charts/docz-site -f charts/docz-site/ci/ci-values.yaml

# Render the chart with the ci values (fast "does it template" check)
helm-template:
    @helm template docz-site charts/docz-site -f charts/docz-site/ci/ci-values.yaml

# Run the chart's helm-unittest suite (needs the helm-unittest plugin)
helm-unittest:
    @helm unittest charts/docz-site

# Regenerate the chart README from README.md.gotmpl + values.yaml
helm-docs:
    @helm-docs --chart-search-root=charts

# CI parity: everything the ci workflow runs, in order
ci: gen-api lint fmt-check typecheck test build bundle-budget e2e gen-api-check
