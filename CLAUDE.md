# CLAUDE.md

## Project

`docz` is a Go CLI tool for generating and managing standardized documentation
(RFCs, ADRs, design docs, implementation plans, plans, investigations).

## Build & Test

```bash
make build          # build to build/bin/docz
make test           # run all tests
make lint           # golangci-lint + golines
make fmt            # gofmt + goimports
make ci             # full CI pipeline (lint + test + build + license-check)
```

## Code Conventions

- Go modules with Cobra/Viper for CLI, `text/template` for rendering
- `//go:embed` for bundled templates in `internal/template/templates/`
- Six built-in doc types: rfc, adr, design, impl, plan, investigation
- Type aliases: `implementation` -> `impl`, `inv` -> `investigation`
- Templates must have `<!-- markdownlint-disable-file MD025 MD041 -->` after frontmatter
- Lint: `golangci-lint` with `golines` for line length
- Tests: `t.TempDir()` for filesystem tests, golden files under `testdata/golden/`
- Golden files are regenerated with `go test ./... -update`, never hand-edited

## Git Workflow

- Always create feature branches + PRs, never push directly to main
- Branch naming: `feat/`, `fix/`, `chore/`, `docs/` prefixes
- Conventional commits required (e.g. `feat(wiki):`, `fix(config):`, `docs:`)
- `docs/examples/plans/` contains user reference material — never commit these files

## Architecture

- `cmd/` — Cobra commands (root, init, create, update, list, template, config, wiki, version)
- `internal/config/` — Config structs, Load(), Validate(), DefaultConfig(), WikiConfig, ToCConfig
- `internal/document/` — Frontmatter parsing, document creation, ID scanning
- `internal/index/` — README index table generation with marker-based splicing
- `internal/template/` — Embedded templates, resolution, rendering
- `internal/toc/` — Table of contents generation with marker-based splicing (toc.go)
- `internal/wiki/` — MkDocs nav generation (titles.go, wiki.go, mkdocs.go)
