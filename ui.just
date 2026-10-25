# Mirrors package.json scripts as `just` recipes so commands are short
# and composable. Underlying tooling (Bun, Vite, vitest, orval) is the
# source of truth — recipes here just call into it.

# bash, not zsh — CI runners don't ship zsh.
set shell := ["bash", "-cu"]

# A module of the root justfile (`mod? ui 'ui.just'`), invoked as
# `just ui <recipe>`. Bun, orval, Vite, Playwright, and bundle-budget.ts all
# resolve paths from the cwd, so every recipe runs in ui/.
set working-directory := "ui"

# Lists this module's recipes. ui/ has no justfile of its own, so a bare
# `just --list` here would climb to the root's.
default:
    @just --list ui

# Install dependencies
install:
    bun install

# Vite dev server, proxying /api,/auth,/openapi.yaml to local docz-api
dev:
    bun run dev

# Vite dev server against MSW fixtures — no docz-api required
dev-msw:
    bun run dev:msw

# docz-review's page against the in-process fake server over the committed
# fixture capture (e2e/fixtures/review-session.json); prints the page URL
dev-review:
    bun run dev:review

# Recapture the review fixture from a fresh docz build (just build first)
gen-review-fixture:
    bun run gen-review-fixture

# Regenerate the docz-api TS client (orval -> src/api/__generated__)
gen-api:
    bun run gen-api

# Drift gate: snapshot, regenerate, diff the generated client
gen-api-check:
    ./scripts/gen-api-check.sh

# Generate the CLI contract types and open schemas (../api/cli -> src/cli/__generated__)
gen-cli:
    bun run gen-cli

# Drift gate: snapshot, regenerate, diff the generated CLI types
gen-cli-check:
    ./scripts/gen-cli-check.sh

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

# docz-review: vitest over the page and the CLI contract, then bun test over
# the server (review/ is Bun-only, outside the vitest src/ graph)
test-review:
    bunx vitest run src/review src/cli
    bun test ./review/

# Bun test for the production server (server/serve.ts is Bun-only, outside
# the vitest src/ graph)
test-server:
    bun test server/

# Vitest watch
test-watch:
    bun run test:watch

# Production build (tsc -b + vite build)
build:
    bun run build

# Entry-chunk size budget (requires a fresh `just build`)
bundle-budget:
    bun scripts/bundle-budget.ts

# Bundle the server exactly as the image does (dist-server/serve.js)
build-server:
    bun build server/serve.ts --target=bun --outfile=dist-server/serve.js

# Run the bundled server against dist/ — what the container actually
# executes. Needs a prior `just build`.
serve-bundle: build-server
    bun dist-server/serve.js

# docz-review's four binaries (DESIGN-0024 section 10): the page built into
# dist-review, its assets embedded through the generated manifest module,
# then bun build --compile per target into dist-review-bin/ (outside the
# inputs, so a rebuild never embeds a previous binary), and the host's
# binary smoke-tested from an empty directory.
build-review version="":
    #!/usr/bin/env bash
    set -euo pipefail
    v="{{ version }}"
    [ -n "$v" ] || v="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
    bun run build:review
    bun scripts/embed-manifest.ts
    rm -rf dist-review-bin
    for target in linux-x64 linux-arm64 darwin-x64 darwin-arm64; do
      os="${target%-*}"; arch="${target#*-}"
      [ "$arch" = x64 ] && arch=amd64
      bun build review/compiled.ts --compile --minify --target="bun-$target" \
        --define "DOCZ_REVIEW_VERSION=\"$v\"" \
        --outfile "dist-review-bin/docz-review_${os}_${arch}"
    done
    host_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    host_arch="$(uname -m)"
    case "$host_arch" in x86_64) host_arch=amd64 ;; aarch64) host_arch=arm64 ;; esac
    (cd dist-review-bin && shasum -a 256 docz-review_* > docz-review_checksums.txt)
    bun review/smoke.ts "dist-review-bin/docz-review_${host_os}_${host_arch}"

# Playwright e2e against an MSW-enabled preview build (dist-msw/)
e2e:
    bun run e2e

# Playwright over docz-review alone (chromium): the page against the fake
# server, then through the real server over the fixture repository, which
# needs a built docz (just build, or DOCZ_REVIEW_DOCZ)
e2e-review:
    bun run build:review
    bunx playwright test e2e/review.spec.ts e2e/review-server.spec.ts

# Serve the production build locally
preview:
    bun run preview

# Build the image without bake: context ui/, with docz-api's spec as the
# named context the Dockerfile copies (DESIGN-0017 OQ 3). The bake spelling,
# from the repository root, is `docker buildx bake dev-ui`.
docker-build:
    docker build -f ../Dockerfile.ui --build-context spec=../api -t donaldgifford/docz-site:dev .

local_compose := "docker compose -f ../deploy/ui/compose.local.yaml"

# Build + start docz-site (:8090) against the running docz-api local stack
local-up:
    @docker network inspect docz-api-local_default >/dev/null 2>&1 || { echo "✗ docz-api local stack not running — run 'just api local-up' first"; exit 1; }
    @{{ local_compose }} up -d --build --wait
    @echo "✓ docz-site up at http://localhost:8090 (proxying to docz-api-local)"

# Stop the local docz-site container
local-down:
    @{{ local_compose }} down
    @echo "✓ docz-site stopped"

# CI parity: everything the ci workflow runs, in order
ci: gen-api gen-cli lint fmt-check typecheck test test-server build bundle-budget e2e gen-api-check gen-cli-check
