---
id: IMPL-0002
title: "Wiki Command for MkDocs TechDocs Integration"
status: Draft
author: Donald Gifford
created: 2026-03-11
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0002: Wiki Command for MkDocs TechDocs Integration

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-03-11

## Objective

Implement the `docz wiki` command group that generates and maintains a
`mkdocs.yml` file compatible with Backstage's TechDocs plugin.

**Implements:** DESIGN-0002

## Scope

### In Scope

- `WikiConfig` struct and integration into `Config` / `.docz.yaml`
- `internal/wiki/` package: nav scanning, title extraction, MkDocs YAML I/O
- `docz wiki init` command: create `mkdocs.yml`, `docs/index.md`, auto-run
  `docz init` if needed
- `docz wiki update` command: rebuild nav from docs directory contents
- Integration with `docz create` to auto-update nav when `mkdocs.yml` exists
- Unit tests, integration tests, golden file tests
- Update `init.go` default config to include `wiki` section with `plan` and
  `investigation` types

### Out of Scope

- Building or serving MkDocs sites (`mkdocs serve` / `mkdocs build`)
- MkDocs theme, plugin, or extension management beyond `techdocs-core`
- Non-markdown content tracking (images, PDFs)
- Custom MkDocs plugins

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

### Phase 1: Configuration and Wiki Package Foundation

Add the `WikiConfig` to the config system and build the core `internal/wiki/`
package with nav scanning, title extraction, and MkDocs YAML read/write. No
CLI commands yet — this phase is purely the internal libraries.

#### Tasks

- [ ] Add `WikiConfig` struct to `internal/config/config.go`:
      ```go
      type WikiConfig struct {
          AutoUpdate bool              `mapstructure:"auto_update" yaml:"auto_update"`
          MkDocsPath string            `mapstructure:"mkdocs_path" yaml:"mkdocs_path"`
          Exclude    []string          `mapstructure:"exclude"     yaml:"exclude"`
          NavTitles  map[string]string `mapstructure:"nav_titles"  yaml:"nav_titles"`
      }
      ```
- [ ] Add `Wiki WikiConfig` field to the `Config` struct
- [ ] Set defaults in `DefaultConfig()`: `AutoUpdate: true`,
      `MkDocsPath: "mkdocs.yml"`, `Exclude: ["templates", "examples"]`,
      `NavTitles` with docz-managed type mappings (`rfc` → `RFCs`,
      `adr` → `ADRs`, `design` → `Design`, `impl` → `Implementation Plans`,
      `plan` → `Plans`, `investigation` → `Investigations`)
- [ ] Wire `WikiConfig` defaults into `setDefaults()` for Viper
- [ ] Update `cmd/init.go` `writeDefaultConfig()` to include `wiki` section,
      and also include `plan` and `investigation` type blocks in the generated
      `.docz.yaml`
- [ ] Update config tests: `TestDefaultConfig` checks wiki defaults,
      `TestLoad_*` tests include wiki config round-trip
- [ ] Create `internal/wiki/titles.go`:
  - `DefaultNavTitles() map[string]string` — returns the default directory-to-
    nav-title mapping for docz types
  - `DirTitle(dir string, navTitles map[string]string) string` — resolve a
    directory name to a nav title (check navTitles map, then title-case the
    directory name, converting hyphens to spaces)
  - `DocTitle(filePath string) (string, error)` — extract nav title from a
    markdown file: parse YAML frontmatter for `id` and `title` fields,
    construct `"<ID>: <Title>"` for docz documents; fall back to first
    `# Heading`; fall back to filename title-cased
  - `FilenameTitle(filename string) string` — convert filename to title case
    (`system-overview.md` → `System Overview`)
- [ ] Create `internal/wiki/wiki.go`:
  - `NavEntry` struct: `Title string`, `Path string`, `Children []NavEntry`
  - `ScanDocs(docsDir string, exclude []string) ([]NavEntry, error)` — walk
    the docs directory recursively, build a tree of `NavEntry` structs.
    Skip excluded directories. For each directory: README.md/index.md becomes
    "Overview" entry (first); docz docs (`NNNN-*.md`) sorted by numeric ID;
    other `.md` files sorted alphabetically. Empty directories are skipped.
    Support arbitrary nesting depth
  - `SortEntries(entries []NavEntry) []NavEntry` — sort top-level: `index.md`
    (Home) first, then remaining entries alphabetically by title
- [ ] Create `internal/wiki/mkdocs.go`:
  - `ReadMkDocs(path string) (map[string]interface{}, error)` — read
    `mkdocs.yml` into a generic map, preserving all fields
  - `WriteMkDocs(path string, data map[string]interface{}) error` — write
    the map back to YAML, preserving non-nav fields
  - `NavToYAML(entries []NavEntry) []interface{}` — convert `[]NavEntry` to
    the MkDocs nav YAML structure (`[]interface{}` of single-key maps)
  - `ExistingNavOrder(data map[string]interface{}) []string` — extract the
    ordered list of top-level section titles from an existing nav
  - `MergeNavOrder(existing []string, newEntries []NavEntry) []NavEntry` —
    preserve existing section order, append new sections alphabetically
- [ ] Write unit tests for `internal/wiki/titles.go`: title extraction from
      frontmatter, H1 heading, filename fallback; directory title mapping
      with overrides and defaults
- [ ] Write unit tests for `internal/wiki/wiki.go`: nav tree building from
      mock directory structures (use `t.TempDir()`); entry sorting; empty
      directory skipping; exclusion filtering; arbitrary nesting
- [ ] Write unit tests for `internal/wiki/mkdocs.go`: YAML round-trip
      preserves unknown fields; nav serialization format matches MkDocs
      expectations; nav order merging preserves existing order and appends new

#### Success Criteria

- `go build ./...` succeeds with no errors
- `go test ./internal/...` passes with all tests green
- `WikiConfig` loads correctly from `.docz.yaml` with defaults applied
- Title extraction correctly handles docz frontmatter, H1 headings, and
  filename fallback
- Nav tree correctly represents a nested directory structure
- MkDocs YAML read/write preserves all non-nav fields

---

### Phase 2: CLI Commands — `wiki init` and `wiki update`

Wire the internal wiki package to Cobra commands and integrate with `docz
create`.

#### Tasks

- [ ] Create `cmd/wiki.go`:
  - `wiki` parent command with help text
  - `wiki init` subcommand:
    - Check if `docz init` has been run (`.docz.yaml` exists and docs dir
      exists); if not, call `runInit` automatically
    - Check if `mkdocs.yml` exists; fail unless `--force` is passed
    - Determine `site_name`: `--site-name` flag, or derive from current
      directory name (repo root)
    - Determine `site_description`: `--site-description` flag, or
      `"Documentation for <site_name>"`
    - Write `mkdocs.yml` with `site_name`, `site_description`,
      `plugins: [techdocs-core]`, and initial nav with `Home: index.md`
    - Create `docs/index.md` if it doesn't exist (minimal landing page with
      links to each docz type's README)
    - Run `wiki update` to populate the nav from existing docs
    - Register flags: `--force`, `--site-name`, `--site-description`
  - `wiki update` subcommand:
    - Verify `mkdocs.yml` exists; error with message to run `wiki init` if not
    - Read existing `mkdocs.yml`
    - Scan docs directory via `wiki.ScanDocs()`
    - Sort entries alphabetically for initial nav; for updates, use
      `MergeNavOrder` to preserve existing section order
    - Convert entries to MkDocs nav YAML format
    - Replace the `nav` key in the mkdocs data map
    - Write back `mkdocs.yml`
    - Print summary: `"Updated nav in mkdocs.yml (N pages)"`
    - Register flag: `--dry-run`
- [ ] Integrate with `docz create`: in `cmd/create.go`, after the existing
      index auto-update, check if `mkdocs.yml` exists at repo root and
      `appCfg.Wiki.AutoUpdate` is true; if so, call the wiki update logic
- [ ] Write integration tests for `wiki init`:
  - Init in empty directory (should auto-run `docz init`)
  - Init in already-initialized directory
  - Init with `--site-name` and `--site-description` flags
  - Init fails when `mkdocs.yml` already exists (without `--force`)
  - Init with `--force` overwrites existing `mkdocs.yml`
  - Verify `docs/index.md` is created with correct content
  - Verify `mkdocs.yml` has correct structure
- [ ] Write integration tests for `wiki update`:
  - Update with various directory structures (docz types, non-docz dirs,
    nested directories)
  - Update preserves existing section order
  - Update appends new sections alphabetically
  - `--dry-run` prints nav without writing
  - Error when `mkdocs.yml` doesn't exist
- [ ] Write integration tests for `docz create` wiki auto-update:
  - Create a document when `mkdocs.yml` exists → nav is updated
  - Create a document when `mkdocs.yml` doesn't exist → no wiki update
  - Create with `wiki.auto_update: false` → no wiki update

#### Success Criteria

- `docz wiki init` in an empty directory creates `.docz.yaml`, docs structure,
  `mkdocs.yml`, and `docs/index.md`
- `docz wiki init --site-name "My Service"` sets the correct site name
- `docz wiki init` fails with a clear error if `mkdocs.yml` already exists
- `docz wiki init --force` overwrites existing `mkdocs.yml`
- `docz wiki update` generates correct nav from docs directory contents
- `docz wiki update --dry-run` prints nav without writing to disk
- `docz wiki update` preserves existing section order and appends new sections
- `docz create rfc "Title"` auto-updates the nav when `mkdocs.yml` exists
- All non-nav fields in `mkdocs.yml` are preserved across updates

---

### Phase 3: Polish, Edge Cases, and CI Readiness

Harden the wiki commands with edge case handling, ensure all tests pass,
and prepare for merge.

#### Tasks

- [ ] Audit error messages across wiki commands for consistency with existing
      docz commands (format: `"doing X: %w"`)
- [ ] Handle edge cases:
  - Docs directory with only excluded directories → empty nav (just Home)
  - Markdown files with no frontmatter and no H1 heading → filename title
  - `mkdocs.yml` with no existing nav key → treat as fresh init
  - `mkdocs.yml` with empty nav → treat as fresh init
  - Directories containing only non-markdown files → skip
  - Symlinks in docs directory → follow or skip (decide and document)
- [ ] Add `--verbose` output to wiki commands: show directories scanned,
      files found, titles resolved, sections added/preserved
- [ ] Ensure `make ci` passes: `lint`, `test`, `build`
- [ ] Ensure `golangci-lint run ./...` produces no warnings for new code
- [ ] Verify test coverage for `internal/wiki/` is >80%
- [ ] Write golden file tests: create a known directory structure, run
      `wiki update`, compare generated `mkdocs.yml` nav against expected output
- [ ] Update `README.md` with `docz wiki` command documentation
- [ ] Update `DEVELOPMENT.md` if wiki package introduces new patterns

#### Success Criteria

- `make ci` passes with zero errors
- `golangci-lint run ./...` produces no warnings
- Test coverage for `internal/wiki/` is >80%
- All edge cases produce clear, actionable error messages
- `--verbose` output is useful for debugging wiki operations
- `README.md` documents the wiki commands
- Golden file tests validate nav output for a representative directory structure

---

## File Changes

Key files that will be created or modified, organized by phase.

### Phase 1

| File | Action | Description |
|------|--------|-------------|
| `internal/config/config.go` | Modify | Add `WikiConfig` struct and field to `Config`, defaults |
| `internal/config/config_test.go` | Modify | Test wiki config defaults and loading |
| `cmd/init.go` | Modify | Add wiki + plan + investigation to generated `.docz.yaml` |
| `internal/wiki/titles.go` | Create | Directory title mapping, doc title extraction |
| `internal/wiki/titles_test.go` | Create | Unit tests for title functions |
| `internal/wiki/wiki.go` | Create | `NavEntry`, `ScanDocs()`, `SortEntries()` |
| `internal/wiki/wiki_test.go` | Create | Unit tests for nav scanning |
| `internal/wiki/mkdocs.go` | Create | MkDocs YAML read/write, nav serialization |
| `internal/wiki/mkdocs_test.go` | Create | Unit tests for YAML handling |

### Phase 2

| File | Action | Description |
|------|--------|-------------|
| `cmd/wiki.go` | Create | `wiki init` and `wiki update` commands |
| `cmd/wiki_test.go` | Create | Integration tests for wiki commands |
| `cmd/create.go` | Modify | Wire wiki auto-update after document creation |

### Phase 3

| File | Action | Description |
|------|--------|-------------|
| `testdata/golden/wiki/` | Create | Golden file test fixtures for nav output |
| `README.md` | Modify | Add wiki command documentation |
| `DEVELOPMENT.md` | Modify | Document wiki package patterns (if needed) |
| Various `internal/wiki/*.go` | Modify | Edge case handling, verbose output |

## Testing Plan

- [ ] Unit tests for `internal/wiki/titles.go`: frontmatter title extraction,
      H1 fallback, filename fallback, directory title with overrides
- [ ] Unit tests for `internal/wiki/wiki.go`: nav tree from directory structure,
      sorting, exclusion, empty directory skipping, arbitrary nesting
- [ ] Unit tests for `internal/wiki/mkdocs.go`: YAML round-trip preserves
      unknown fields, nav serialization format, nav order merging
- [ ] Integration tests for `wiki init`: creates all expected files, auto-runs
      `docz init`, `--force` behavior, flag overrides
- [ ] Integration tests for `wiki update`: various directory layouts, order
      preservation, `--dry-run`, error on missing mkdocs.yml
- [ ] Integration tests for `docz create` auto-update: nav updated when
      mkdocs.yml present, skipped when absent or disabled
- [ ] Golden file tests for nav output given a representative directory tree
- [ ] Table-driven tests for title extraction edge cases

## Dependencies

- `go.yaml.in/yaml/v3` — already in `go.mod`, used for MkDocs YAML I/O
- `github.com/spf13/cobra` — already in `go.mod`, used for CLI commands
- No new external dependencies required
- `os`, `path/filepath`, `sort`, `strings`, `regexp`, `bufio` from stdlib

## References

- [DESIGN-0002: Wiki Command for MkDocs TechDocs Integration](../design/0002-wiki-command-for-mkdocs-techdocs-integration.md)
- [IMPL-0001: docz CLI Implementation](0001-docz-cli-implementation.md)
- [Backstage TechDocs](https://backstage.io/docs/features/techdocs/)
- [MkDocs Configuration](https://www.mkdocs.org/user-guide/configuration/)
