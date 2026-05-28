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

- `cmd/` — Cobra commands (root, init, create, update, list, template, config, wiki, version). `cmd.Runner` (runner.go) bundles the resolved Config with injectable dependencies (Out/Err writers, slog logger, time source, GitResolver); handlers are methods on `*Runner` and tests construct one directly with `bytes.Buffer` writers + stub Now/Git. `loadAndValidateConfig` (root.go) builds the production Runner and overwrites its Logger via `buildLogger` to honor `--verbose` / `--log-level` / `--log-format` flags. Resolution order: explicit `--log-level` wins, else `--verbose`→debug, else info. Invalid values fail at startup. `getRunner()` is the transitional accessor for tests that bypass PersistentPreRunE — wiki/list tests reset `runner = nil` in their setup helper so the previous test's writer doesn't leak across tests.
- `internal/config/` — Config structs, Load(), Validate(), DefaultConfig(), WikiConfig, TOCConfig (YAML key still `toc:` for back-compat); centralized file-mode (`FileMode`, `DirMode`) and filename constants (`ConfigFileName`, `IndexFileName`, `WikiIndexName`, `MkDocsFileName`, `TemplatesDir`) in `constants.go`. Defaults are sourced exclusively from `DefaultConfig()`; `Load`/`loadFromFile` unmarshal Viper output onto a pre-populated `DefaultConfig()`, so sibling fields are preserved without `SetDefault` registrations. `Config.ValidateType(name)` is the single source of "unknown document type" errors (wraps the `ErrUnknownType` sentinel); `Config.EnabledTypes()` returns the enabled-type slice in registry-declaration order for scaffolding/update loops. Per-type metadata (canonical name, aliases, default `TypeConfig`, nav title, plural label, template name) lives in the `allDocTypes` registry in `doctype.go` (`DocTypeDef` struct; `DefaultConfig` is a `func() TypeConfig` constructor so each lookup yields a fresh `Statuses` slice). `DefaultConfig().Types`, `DefaultNavTitles()`, the `typeAliases` map, and `DocTypeNames()` (the canonical type-name list) all derive from the registry — adding a new doc type is one registry entry plus two embedded templates. `DocTypeNames()` preserves registry-declaration order because `ValidateType`'s error string and existing tests depend on it. The old `ValidTypes()` shim is gone; callers use `DocTypeNames()` (built-in catalog) or `Config.EnabledTypes()` (user's effective set)
- `internal/document/` — Frontmatter parsing, document creation, ID scanning. `ScanDocuments(dir)` (scan.go) walks a type directory, parses frontmatter, and returns `[]DocEntry` with `Content []byte` cached so downstream callers (notably `cmd/update.go`'s ToC pass) do not re-read each document. `LoadFrontmatter(path)` (document.go) is the single read+parse helper; returns `(fm, content, ErrNoFrontmatter)` for files lacking frontmatter so callers (`scanner`, `wiki.DocTitle`) can fall back without a fatal error
- `internal/index/` — README index table generation with marker-based splicing only (`GenerateTable`, `UpdateReadme`, `DryRunReadme`). Returns typed `UpdateOutcome{Action, Path, Body}` (Action enum: `ActionCreated`, `ActionUpdated`, `ActionNoMarkers`, `ActionDryRunCreated`, `ActionDryRunUpdated`); cmd/ owns the user-facing wording. Scanning lives in `internal/document`; this package depends on `document.DocEntry` for its input type
- `internal/template/` — Embedded templates, resolution, rendering; includes `docz_yaml.tmpl` consumed by `cmd/init` to render `.docz.yaml` from `config.DefaultConfig()` (single source of defaults)
- `internal/toc/` — Table of contents generation with marker-based splicing (toc.go). `UpdateToC` returns an `UpdateResult{Updated, Headings, Found}` struct so callers (e.g. `docz update --dry-run`) reuse the parsed `[]Heading` without calling `ParseHeadings` a second time. `UpdateFiles([]FileInput, minHeadings, dryRun)` (update.go) walks a list of in-memory docs, performs the ToC splice + optional write-back, and returns a categorized `UpdateReport` (Updated / Unchanged / WouldUpdate / Skipped / WriteErrors); cmd/ owns all user-facing formatting
- `internal/wiki/` — MkDocs nav generation (titles.go, wiki.go, mkdocs.go). `wiki.CreateMkDocs(path, *MkDocsConfig)` builds the initial `mkdocs.yml` (cmd/ no longer constructs YAML strings inline)
