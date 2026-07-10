# Mirrors package.json scripts as `just` recipes so commands are short
# and composable. Underlying tooling (Bun, react-router, vitest) is the
# source of truth — recipes here just call into it.

set shell := ["zsh", "-cu"]

default:
    @just --list

# Install dependencies
install:
    bun install

# Vite dev server (SSR + HMR)
dev:
    bun run dev

# Vite dev server with MSW handlers backing the API surface.
# No rfc-api / Postgres / GitHub webhook required. See IMPL-0002.
dev-msw:
    bun run dev:msw

# Production build
build:
    bun run build

# Serve the production build
start:
    bun run start

# Regenerate RR7 route types in .react-router/types/
typegen:
    bun run typegen

# Strict TS check (runs typegen first)
typecheck:
    bun run typecheck

# Lint
lint:
    bun run lint

# Lint with autofix
lint-fix:
    bun run lint:fix

# Prettier write
format:
    bun run format

# Prettier check (no write)
format-check:
    bun run format:check

# Vitest single run
test:
    bun run test

# Vitest watch
test-watch:
    bun run test:watch

# Composite: run all the static checks (CI parity)
check: typecheck lint format-check test

# Regenerate the rfc-api TS client (orval -> src/portal/api/__generated__)
gen-api:
    bun run gen-api

# CI drift check: regenerate the client twice and fail if outputs diverge
gen-api-check:
    ./scripts/gen-api-check.sh
