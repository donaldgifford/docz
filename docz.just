# docz — library and CLI recipes
#
# The Go half of the repository: build, test, the parity and consumer
# suites, corpus validation, lint, docs, licences, and release plumbing.
# Imported by the root justfile (ADR-0004 Decision 4). The api/ and ui/
# halves land as api.just and ui.just when those services move in.

## Project

project_name  := "docz"
project_owner := "donaldgifford"
go_package    := "github.com/" + project_owner + "/" + project_name

# module_path is the import path with its major-version suffix. The v2 line
# ships as .../docz/v2 (ADR-0002 Decision 3), and ldflags must name that path
# or the version injection silently no-ops — a `-X` on a path no package has
# is ignored without a word.
module_path := go_package + "/v2"

## Directories

build_dir := "build"
bin_dir   := build_dir + "/bin"

# bin is the binary the parity suite drives. Override it to replay the
# goldens against a release build instead of the local one:
#   just bin=/path/to/docz parity
bin := bin_dir / project_name

coverage_out := "coverage.out"

## Version injection

commit_hash := `git rev-parse --short HEAD 2>/dev/null || echo unknown`
version     := `git describe --tags --always --dirty 2>/dev/null || echo dev`

## Tooling

goimports_local  := "github.com/" + project_owner
allowed_licenses := "Apache-2.0,MIT,BSD-2-Clause,BSD-3-Clause,ISC,MPL-2.0"

# ─── Build ──────────────────────────────────────────────────────────

# Build everything (core)
[group('build')]
build: build-core

# Build the CLI binary into build/bin/docz
[group('build')]
build-core:
    @mkdir -p {{ bin_dir }}
    @go build -ldflags "-X {{ module_path }}/cmd.Version={{ version }} -X {{ module_path }}/cmd.Commit={{ commit_hash }}" \
        -o {{ bin_dir }}/{{ project_name }} ./cmd/{{ project_name }}
    @echo "✓ Core binaries built"

# Remove build artifacts and the Go build cache
[group('build')]
clean:
    @rm -rf {{ bin_dir }}/
    @rm -f {{ coverage_out }}
    @go clean -cache
    @find . -name "*.test" -delete
    @echo "✓ Build artifacts cleaned"

# ─── Run ────────────────────────────────────────────────────────────

# Build then run the CLI: just run list adr
[group('run')]
run *args: build
    @{{ bin }} {{ args }}

# ─── Test ───────────────────────────────────────────────────────────

# Run all tests with the race detector
[group('test')]
test:
    @go test -v -race ./...

# Run all tests (alias of test)
[group('test')]
test-all: test

# Run tests for a single package: just test-pkg ./pkg/impl
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

# Run the external-module consumer smoke test (separate go.mod)
[group('test')]
test-consumer:
    @cd test/consumer && go test ./...

# ─── Functional parity (IMPL-0018 Phase 0) ─────────────────────────

# The release the goldens are captured from. Goldens come from this tag and
# from nowhere else: regenerating them from a v2 build would make the suite
# agree with whatever it happens to be measuring.
parity_tag := "v1.2.2"

# Deliberately the unversioned path: the capture installs the v1.2.2 binary,
# and v1 tags live at github.com/donaldgifford/docz, not at /v2.
parity_pkg := go_package + "/cmd/" + project_name

# Replay the parity goldens (bin=<path> to drive another binary)
[group('parity')]
parity: build
    @DOCZ_PARITY_BIN="{{ absolute_path(bin) }}" go test -tags parity -count=1 ./test/parity/...
    @echo "✓ Parity goldens replay against {{ bin }}"

# Capture parity goldens from the v1.2.2 release binary
[group('parity')]
parity-capture:
    #!/usr/bin/env bash
    set -euo pipefail
    bin="${DOCZ_PARITY_BIN:-}"; tmp=""
    if [ -z "$bin" ]; then
        tmp=$(mktemp -d)
        echo "  installing {{ parity_pkg }}@{{ parity_tag }}"
        GOBIN="$tmp" GOFLAGS= go install {{ parity_pkg }}@{{ parity_tag }}
        bin="$tmp/{{ project_name }}"
    fi
    echo "  capturing from $bin ($("$bin" version))"
    DOCZ_PARITY_BIN="$bin" go test -tags parity -count=1 ./test/parity/... -update
    [ -n "$tmp" ] && rm -rf "$tmp" || true
    echo "✓ Goldens captured into test/parity/testdata (record provenance in test/parity/README.md)"

# ─── Corpus validation (IMPL-0018 Phase 5) ─────────────────────────
#
# Non-strict on purpose (DESIGN-0015, IMPL-0018 Open Question 7): errors fail
# the build, warnings print and do not. The older hand-written documents carry
# warnings this repository has not finished paying down, and gating on them
# would make every unrelated change wait for that work.

# Validate this repository's own docs against their schemas
[group('validate')]
validate: build
    @{{ bin }} validate
    @echo "✓ docs/ validates against its schemas"

# ─── Lint & format ─────────────────────────────────────────────────

# Run golangci-lint
[group('lint')]
lint:
    @golangci-lint run ./...

# Run golangci-lint with --fix
[group('lint')]
lint-fix:
    @golangci-lint run --fix ./...

# Format code with gofmt and goimports
[group('lint')]
fmt:
    @gofmt -s -w .
    @goimports -w -local {{ goimports_local }} .

# ─── Documentation ─────────────────────────────────────────────────

# Initialize docz in the repository
[group('docs')]
docs-init: build
    @{{ bin }} init

# Update all document indexes
[group('docs')]
docs-update: build
    @{{ bin }} update

# List all documents
[group('docs')]
docs-list: build
    @{{ bin }} list

# Show the resolved configuration
[group('docs')]
docs-config: build
    @{{ bin }} config

# ─── License compliance ────────────────────────────────────────────

# Check dependency licenses against the allow list
[group('license')]
license-check:
    @go-licenses check ./... --allowed_licenses={{ allowed_licenses }}

# Generate a CSV report of all dependency licenses
[group('license')]
license-report:
    @go-licenses report ./... --template=.github/licenses-csv.tpl

# ─── Release ───────────────────────────────────────────────────────

# Validate the goreleaser config
[group('release')]
release-check:
    @goreleaser check

# Snapshot release locally (no publish, no sign)
[group('release')]
release-local:
    @goreleaser release --snapshot --clean --skip=publish --skip=sign

# Every PR on the v2 line carries dont-release, so betas are cut by hand from
# a main merge commit (ADR-0002 Decision 6). prerelease.yml triggers on the
# v*-beta.* tag.

# Tag and push a release: just release v2.0.0-beta.2
[group('release')]
release tag:
    @git tag -a {{ tag }} -m "Release {{ tag }}"
    @git push origin {{ tag }}
