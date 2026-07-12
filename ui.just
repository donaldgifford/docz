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

# Serve the production build locally
preview:
    bun run preview

# CI parity: everything the ci workflow runs, in order
ci: gen-api lint fmt-check typecheck test build bundle-budget gen-api-check
