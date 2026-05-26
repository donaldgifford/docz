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
- `internal/config/` — Config structs, Load(), Validate(), DefaultConfig(), WikiConfig, ToCConfig; centralized file-mode (`FileMode`, `DirMode`) and filename constants (`ConfigFileName`, `IndexFileName`, `WikiIndexName`, `MkDocsFileName`, `TemplatesDir`) in `constants.go`. Defaults are sourced exclusively from `DefaultConfig()`; `Load`/`loadFromFile` unmarshal Viper output onto a pre-populated `DefaultConfig()`, so sibling fields are preserved without `SetDefault` registrations. `Config.ValidateType(name)` is the single source of "unknown document type" errors (wraps the `ErrUnknownType` sentinel); `Config.EnabledTypes()` returns the sorted enabled-type slice for scaffolding/update loops
- `internal/document/` — Frontmatter parsing, document creation, ID scanning
- `internal/index/` — README index table generation with marker-based splicing. `DocEntry.Content []byte` caches the raw file bytes during `ScanDocuments` so downstream callers (notably `cmd/update.go`'s ToC pass) do not re-read each document
- `internal/template/` — Embedded templates, resolution, rendering; includes `docz_yaml.tmpl` consumed by `cmd/init` to render `.docz.yaml` from `config.DefaultConfig()` (single source of defaults)
- `internal/toc/` — Table of contents generation with marker-based splicing (toc.go). `UpdateToC` returns an `UpdateResult{Updated, Headings, Found}` struct so callers (e.g. `docz update --dry-run`) reuse the parsed `[]Heading` without calling `ParseHeadings` a second time
- `internal/wiki/` — MkDocs nav generation (titles.go, wiki.go, mkdocs.go). `wiki.CreateMkDocs(path, *MkDocsConfig)` builds the initial `mkdocs.yml` (cmd/ no longer constructs YAML strings inline)
