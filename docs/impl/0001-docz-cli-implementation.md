---
id: IMPL-0001
title: "docz CLI Implementation"
status: Draft
author: Donald Gifford
created: 2026-02-22
---

# IMPL 0001: docz CLI Implementation

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-02-22

## Objective

Implement the `docz` CLI tool as described in
[DESIGN-0001](../../docs/design/0001-docz-cli-design.md).

**Implements:** DESIGN-0001

## Scope

### In Scope

- All four built-in document types: RFC, ADR, DESIGN, IMPL
- Embedded default templates via `//go:embed`
- Configuration loading (repo root `.docz.yaml` with `~/.docz.yaml` fallback)
- Commands: `init`, `create`, `update`, `list`, `template show/export/override`,
  `config`, `version`
- YAML frontmatter parsing for index generation
- Auto-generated README index tables with marker-based preservation
- Template override resolution (explicit config > local file > embedded)
- Unit tests, integration tests, golden file tests

### Out of Scope

- Claude Code plugin / skills (deferred to later phase)
- Custom user-defined document types
- `docz status` command for status transitions
- Cross-reference linking between documents
- Git hooks integration
- Template partials and includes

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

### Phase 1: Project Foundation

Set up the internal package structure, embedded templates, and configuration
system. No CLI commands yet beyond the existing root command -- this phase is
purely the internal libraries that all commands depend on.

#### Tasks

- [x] Fix `go.mod`: move `cobra` and `viper` from indirect to direct
      dependencies (run `go mod tidy`)
- [x] Create directory structure:
      `internal/config/`, `internal/document/`, `internal/index/`,
      `internal/template/`, `internal/template/templates/`
- [x] Write the four default document template files under
      `internal/template/templates/` (`rfc.md`, `adr.md`, `design.md`,
      `impl.md`) using the templates defined in DESIGN-0001
- [x] Write the four default index header template files under
      `internal/template/templates/` (`index_rfc.md`, `index_adr.md`,
      `index_design.md`, `index_impl.md`)
- [x] Implement `internal/template/embed.go`: use `//go:embed templates/*.md`
      to embed all template files; export functions to retrieve a document
      template or index header template by type name
- [x] Implement `internal/template/template.go`: template resolution logic
      (config path > local override > embedded default), rendering a template
      with a `TemplateData` struct via `text/template`
- [x] Define `TemplateData` struct with fields: `Number`, `Title`, `Date`,
      `Author`, `Status`, `Type`, `Prefix`, `Slug`, `Filename`
- [x] Implement slug generation: title to kebab-case (lowercase, spaces to
      hyphens, strip non-alphanumeric except hyphens, trim leading/trailing
      hyphens)
- [x] Implement `internal/config/config.go`: define `Config` struct matching
      the `.docz.yaml` schema from DESIGN-0001, including `TypeConfig` for
      each document type; implement `Load()` using two separate Viper
      instances -- one for `~/.docz.yaml` (global), one for repo root
      `.docz.yaml` (local). Load global first, then deep-merge local on
      top via `MergeConfigMap`. Deep merge means the repo config overrides
      only the keys it explicitly sets; unset keys inherit from global.
      Apply built-in defaults for any keys missing from both files
- [x] Implement `internal/document/document.go`: define `Frontmatter` struct,
      implement `ParseFrontmatter(fileContent []byte) (Frontmatter, error)`
      that splits on `---` delimiters and unmarshals YAML
- [x] Write unit tests for `internal/template/`: template resolution order,
      rendering with all placeholder variables, slug generation edge cases
- [x] Write unit tests for `internal/config/`: default values, loading from
      file, deep merge behavior (repo config key overrides global, unset keys
      inherit from global, both missing returns defaults)
- [x] Write unit tests for `internal/document/`: frontmatter parsing with
      valid YAML, missing fields, no frontmatter, malformed delimiters

#### Success Criteria

- `go build ./...` succeeds with no errors
- `go test ./internal/...` passes with all tests green
- Template resolution returns embedded default when no config or local override
  exists
- Config loading returns correct defaults when no `.docz.yaml` exists
- Frontmatter parsing correctly extracts all five fields (id, title, status,
  author, created) from a well-formed document

---

### Phase 2: Core Commands -- `init`, `create`, `version`

Wire up the internal packages to the first three Cobra commands. After this
phase a user can initialize a repo and create documents.

#### Tasks

- [x] Move `main.go` to `cmd/docz/main.go` to follow the standard Go project
      layout (`cmd/<name>/main.go`). Update the `main` package to import
      `github.com/donaldgifford/docz/cmd`. The Makefile `build-core` target
      already points to `./cmd/$(PROJECT_NAME)` so this aligns with it
- [x] Update `cmd/root.go`: replace boilerplate descriptions with actual docz
      descriptions; integrate config loading via `internal/config` in
      `initConfig()`; register `--docs-dir` and `--verbose` persistent flags;
      remove the placeholder `--toggle` flag
- [x] Implement `cmd/version.go`: define `var Version` and `var Commit` in the
      `cmd` package; Makefile ldflags will target
      `-X github.com/donaldgifford/docz/cmd.Version=...` and
      `-X github.com/donaldgifford/docz/cmd.Commit=...`; register `version`
      as subcommand of root that prints these values
- [x] Implement `cmd/init.go`: create `.docz.yaml` at repo root if it doesn't
      exist (write default config); create `docs/{rfc,adr,design,impl}/`
      directories (mkdir -p semantics); write default `README.md` index files
      into each directory using embedded index header templates with an empty
      auto-generated table section; skip existing README files unless `--force`
      is passed; register `--force` flag
- [x] Implement `internal/document/create.go`: scan target directory for
      `NNNN-*.md` files to find next ID number; build `TemplateData` from
      inputs; resolve and render the template; write the output file; return
      the created file path
- [x] Implement `cmd/create.go`: accept `<type> <title>` arguments; validate
      type is one of rfc/adr/design/impl; resolve author from `--author` flag,
      then config, then `git config user.name`, then "Unknown"; call
      `internal/document` create logic; print confirmation with file path and
      next-steps hints; register `--status`, `--author`, `--no-update` flags
- [x] Add author resolution helper: check flag, then config `author.default`,
      then `git config user.name` (exec `git config user.name` and trim),
      then fallback string
- [ ] Write integration tests for `docz init`: test directory/file creation,
      idempotency (safe to run twice), `--force` overwrites READMEs
- [ ] Write integration tests for `docz create`: test file creation with
      correct filename, frontmatter content, auto-increment IDs, duplicate
      filename rejection, invalid type rejection
- [ ] Write golden file tests: render each of the four templates with sample
      data and compare against checked-in expected output files

#### Success Criteria

- `docz init` in an empty directory creates `.docz.yaml` and all four type
  directories with README.md files
- `docz init` run a second time produces no errors and does not overwrite
  READMEs
- `docz init --force` regenerates README files
- `docz create rfc "Test RFC"` produces `docs/rfc/0001-test-rfc.md` with
  correct frontmatter and template content
- `docz create adr "Second"` after a first create produces `0002-second.md`
- `docz create badtype "Title"` exits with a clear error
- `docz version` prints version and commit hash

---

### Phase 3: Index Generation -- `update`

Implement the index/README generation system that reads YAML frontmatter from
all documents in a type directory and produces an auto-generated table.

#### Tasks

- [ ] Implement `internal/index/index.go`:
  - `ScanDocuments(dir string) ([]Frontmatter, error)` -- find all
    `NNNN-*.md` files in a directory, parse frontmatter from each, skip files
    without valid frontmatter (no error, silent skip), return sorted by ID
  - `GenerateTable(docs []Frontmatter) string` -- produce a markdown table
    with columns: ID, Title, Status, Date, Author, Link
  - `UpdateReadme(readmePath string, tableContent string) error` -- read
    existing README, find `<!-- BEGIN DOCZ AUTO-GENERATED -->` and
    `<!-- END DOCZ AUTO-GENERATED -->` markers, replace content between
    them with new table. If markers don't exist and the file exists, **do
    not modify the file** -- print a warning telling the user to run
    `docz init --force` or manually add the markers. If the file doesn't
    exist, create it with the default index header template plus the
    markers and table
- [ ] Implement `cmd/update.go`: accept optional `[type]` argument; if no type
      given, update all four; for each type, resolve the directory path from
      config, call `ScanDocuments` and `UpdateReadme`; register `--dry-run`
      flag that prints what would change without writing
- [ ] Wire `docz create` to automatically call `update` after creating a
      document (unless `--no-update` is passed)
- [ ] Write unit tests for `internal/index/`: table generation with 0, 1, and
      multiple documents; README update with existing markers; README without
      markers is not modified (warning emitted); README creation from scratch
      when file doesn't exist; preservation of content above the begin marker
- [ ] Write integration tests for `docz update`: create several documents,
      run update, verify README contents; test `--dry-run` produces no file
      changes; test empty directory produces empty table
- [ ] Write golden file tests for index generation: compare generated README
      output against checked-in expected files for various scenarios

#### Success Criteria

- `docz update rfc` generates a correct `docs/rfc/README.md` with a table of
  all RFC documents found in the directory
- `docz update` (no argument) updates all four type directories
- Custom content above the `<!-- BEGIN DOCZ AUTO-GENERATED -->` marker is
  preserved across updates
- Files without frontmatter are silently skipped (no error output)
- `docz create rfc "Title"` automatically updates the RFC index
- `docz create rfc "Title" --no-update` skips the index update
- `docz update --dry-run` outputs what would change but writes nothing

---

### Phase 4: Listing and Template Management -- `list`, `template`

Add the remaining read-only commands for listing documents and managing
templates.

#### Tasks

- [ ] Implement `cmd/list.go`: accept optional `[type]` argument; if no type
      given, list across all four types; use `internal/index.ScanDocuments` to
      read documents; display as a formatted table to stdout (ID, Title,
      Status, Date, Author, Type); register `--status` flag to filter by
      status (case-insensitive match); register `--format` flag supporting
      `table` (default), `json`, `csv` output
- [ ] Implement table formatter: aligned columns using `text/tabwriter`
- [ ] Implement JSON output: marshal document list as JSON array to stdout
- [ ] Implement CSV output: write CSV with header row to stdout
- [ ] Implement `cmd/template.go` with three subcommands:
  - `template show <type>` -- resolve the template for the given type and
    print it to stdout (raw template text, not rendered)
  - `template export <type> [path]` -- resolve the template and write it to
    a file at the given path (default: `./<type>.md`)
  - `template override <type>` -- copy the resolved template into the local
    overrides directory (`<docs_dir>/templates/<type>.md`) so the user can
    edit it; fail if the override file already exists
- [ ] Implement `cmd/config.go`: print the fully resolved configuration
      (merged repo + global + defaults) as YAML to stdout
- [ ] Write unit tests for list formatting: table alignment, JSON structure,
      CSV correctness, status filtering
- [ ] Write integration tests for `docz list`: list with documents present,
      list with no documents, list filtered by type and status, JSON and CSV
      output formats
- [ ] Write integration tests for `docz template show/export/override`: verify
      correct template is printed/written, override creates file in correct
      location, override fails if file exists
- [ ] Write integration test for `docz config`: verify merged output matches
      expected resolved config

#### Success Criteria

- `docz list` prints a formatted table of all documents across all types
- `docz list rfc` prints only RFC documents
- `docz list --status accepted` filters correctly (case-insensitive)
- `docz list --format json` outputs valid JSON
- `docz list --format csv` outputs valid CSV with headers
- `docz template show rfc` prints the RFC template to stdout
- `docz template export design ./my-template.md` writes the file
- `docz template override adr` creates `docs/templates/adr.md`
- `docz config` prints the fully resolved YAML configuration

---

### Phase 5: Polish, Error Handling, and CI Readiness

Harden the tool with edge case handling, improve user-facing output, ensure
the Makefile build pipeline works end-to-end, and prepare for release.

#### Tasks

- [ ] Audit all commands for consistent error messages: invalid arguments,
      missing directories, permission errors, file-already-exists errors
- [ ] Add `--verbose` flag behavior: when set, print additional context during
      operations (template resolution path chosen, config file locations
      checked, files scanned during update)
- [ ] Validate config on load: warn on unknown keys, error on invalid type
      names, error on empty statuses list
- [ ] Handle edge cases in ID assignment: directories with non-sequential IDs,
      directories with only non-matching files, empty directories
- [ ] Handle edge cases in slug generation: titles with unicode, titles that
      are all special characters, very long titles (truncate slug at a
      reasonable length), empty title after slug conversion
- [ ] Update `Makefile` `build-core` target: update ldflags to target
      `github.com/donaldgifford/docz/cmd.Version` and
      `github.com/donaldgifford/docz/cmd.Commit` instead of `main.version`
      and `main.commit` (build path `./cmd/$(PROJECT_NAME)` is now correct
      after the `main.go` move in Phase 2)
- [ ] Add `Makefile` targets for docz operations: `make docs-init`,
      `make docs-update`, etc. as convenience wrappers
- [ ] Ensure `make ci` passes: `lint`, `test`, `build`, `license-check` all
      succeed
- [ ] Ensure `make test-coverage` reports reasonable coverage across
      `internal/` packages (target: >80%)
- [ ] Review and clean up any TODO/FIXME comments left during implementation
- [ ] Verify `go vet ./...` and `golangci-lint run ./...` produce no warnings

#### Success Criteria

- `make ci` passes with zero errors
- `golangci-lint run ./...` produces no warnings
- `go vet ./...` produces no warnings
- Test coverage for `internal/` packages is >80%
- All error paths produce clear, actionable messages (no raw Go error dumps)
- `--verbose` flag produces useful debugging output for each command
- The built binary at `build/bin/docz` runs correctly for all commands

---

## File Changes

Key files that will be created or modified, organized by phase.

### Phase 1

| File | Action | Description |
|------|--------|-------------|
| `go.mod` | Modify | Fix indirect deps to direct |
| `internal/template/templates/rfc.md` | Create | Embedded RFC template |
| `internal/template/templates/adr.md` | Create | Embedded ADR template |
| `internal/template/templates/design.md` | Create | Embedded DESIGN template |
| `internal/template/templates/impl.md` | Create | Embedded IMPL template |
| `internal/template/templates/index_rfc.md` | Create | Default RFC index header |
| `internal/template/templates/index_adr.md` | Create | Default ADR index header |
| `internal/template/templates/index_design.md` | Create | Default DESIGN index header |
| `internal/template/templates/index_impl.md` | Create | Default IMPL index header |
| `internal/template/embed.go` | Create | `//go:embed` directives and accessors |
| `internal/template/template.go` | Create | Resolution, rendering, slug generation |
| `internal/template/template_test.go` | Create | Unit tests |
| `internal/config/config.go` | Create | Config structs, loading, defaults |
| `internal/config/config_test.go` | Create | Unit tests |
| `internal/document/document.go` | Create | Frontmatter struct and parsing |
| `internal/document/document_test.go` | Create | Unit tests |

### Phase 2

| File | Action | Description |
|------|--------|-------------|
| `main.go` | Delete | Remove root-level entry point |
| `cmd/docz/main.go` | Create | Entry point at standard Go layout location |
| `cmd/root.go` | Modify | Real descriptions, config integration, global flags |
| `cmd/version.go` | Create | Version command with ldflags vars |
| `cmd/init.go` | Create | Init command |
| `cmd/create.go` | Create | Create command |
| `internal/document/create.go` | Create | Document creation logic (ID scan, file write) |
| `internal/document/create_test.go` | Create | Unit tests |
| `testdata/golden/` | Create | Golden file test fixtures |

### Phase 3

| File | Action | Description |
|------|--------|-------------|
| `internal/index/index.go` | Create | Scan, table generation, README update |
| `internal/index/index_test.go` | Create | Unit tests |
| `cmd/update.go` | Create | Update command |
| `cmd/create.go` | Modify | Wire auto-update after create |

### Phase 4

| File | Action | Description |
|------|--------|-------------|
| `cmd/list.go` | Create | List command with formatters |
| `cmd/template.go` | Create | Template show/export/override commands |
| `cmd/config.go` | Create | Config display command |

### Phase 5

| File | Action | Description |
|------|--------|-------------|
| `Makefile` | Modify | Update ldflags to target `cmd` package, add docs targets |
| Various `*.go` | Modify | Error handling hardening, verbose output |

## Testing Plan

Tests are distributed across phases but follow a consistent strategy:

- [ ] Unit tests for every exported function in `internal/` packages
- [ ] Table-driven tests for functions with multiple input variations
      (slug generation, frontmatter parsing, config merging)
- [ ] Integration tests using `t.TempDir()` for filesystem operations
      (real filesystem, automatically cleaned up by the test framework)
- [ ] Golden file tests under `testdata/golden/` for template rendering
      and index generation output
- [ ] CLI tests using Cobra's test helpers (`cmd.SetArgs`, capture stdout/
      stderr) for argument parsing and flag validation
- [ ] No mocking of external commands except `git config user.name`
      (use env var or test flag to control author in tests)

## Dependencies

- All Go dependencies are already in `go.mod` (cobra, viper, yaml)
- No new external dependencies required
- `text/template`, `embed`, `text/tabwriter`, `encoding/json`,
  `encoding/csv`, `testing` (`t.TempDir()`) are all stdlib

## Resolved Decisions

1. **`t.TempDir()` for tests** -- Use `t.TempDir()` (real filesystem,
   automatically cleaned up) for all integration tests. Do not thread
   `afero.Fs` through production APIs. This keeps the production code simple
   and avoids an abstraction layer that exists solely for test convenience.

2. **Deep merge for config** -- Use deep merge via two separate Viper
   instances. Load global config (`~/.docz.yaml`) into one instance, load
   repo config (`.docz.yaml`) into another, then merge the repo config map
   on top of the global via `MergeConfigMap`. This is the idiomatic Viper
   pattern: the repo config overrides only the keys it explicitly sets,
   and unset keys inherit from global. Note: Viper deep-merges nested maps
   recursively but replaces slices entirely -- this means a repo config that
   overrides `types.rfc.statuses` replaces the entire list (not appending),
   which is the correct behavior for status lists.

3. **Move `main.go` to `cmd/docz/main.go`** -- Follow the standard Go
   project layout (`cmd/<name>/main.go`). This aligns with the existing
   Makefile `build-core` target which already points to
   `./cmd/$(PROJECT_NAME)`, matches the `golang-standards/project-layout`
   recommendation, and positions the project for potential future multi-binary
   support.

4. **Version vars in `cmd/version.go`** -- Define `var Version` and
   `var Commit` in the `cmd` package. Makefile ldflags target
   `-X github.com/donaldgifford/docz/cmd.Version=...` and
   `-X github.com/donaldgifford/docz/cmd.Commit=...`. This avoids passing
   values between packages and keeps version information co-located with
   the `version` subcommand.

5. **Do not touch non-docz README files** -- If `docz update` encounters a
   README that exists but has no `<!-- BEGIN DOCZ AUTO-GENERATED -->` marker,
   it does not modify the file. It prints a warning telling the user to run
   `docz init --force` or manually add the markers. This prevents `docz` from
   corrupting manually-maintained documentation.

## References

- [DESIGN-0001: docz CLI Tool](../design/0001-docz-cli-design.md)
- [Previous bash scripts](../examples/)
