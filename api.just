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

# Format code with gofmt + goimports
[group('lint')]
fmt:
    @gofmt -s -w .
    @goimports -w -local {{ goimports_local }} .

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
