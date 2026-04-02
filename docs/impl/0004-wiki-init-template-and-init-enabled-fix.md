---
id: IMPL-0004
title: "Wiki Init Template and Init Enabled Fix"
status: Draft
author: Donald Gifford
created: 2026-04-02
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0004: Wiki Init Template and Init Enabled Fix

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-04-02

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Init Enabled Fix and Config Changes](#phase-1-init-enabled-fix-and-config-changes)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Wiki Index Template](#phase-2-wiki-index-template)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Documentation, Polish, and CI Readiness](#phase-3-documentation-polish-and-ci-readiness)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Open Questions](#open-questions)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

## Objective

Implement the three changes identified in INV-0001: (1) a customizable template
for the wiki `docs/index.md` homepage, (2) configurable MkDocs plugins via
`.docz.yaml`, and (3) fix `docz init` to skip types with `enabled: false`.

**Implements:** INV-0001

## Scope

### In Scope

- Embedded `wiki_index.md` template with `{{ .SiteName }}` and `{{ .Types }}`
  variables, overridable at `<docs_dir>/templates/wiki_index.md`
- `EmbeddedWikiIndex()` accessor in `internal/template/embed.go`
- Rewrite `ensureDocsIndex()` in `cmd/wiki.go` to render the template
- Add `Plugins []string` field to `WikiConfig`, default `["techdocs-core"]`
- Wire plugins into `writeMkDocsYAML()` and `setDefaults()`
- Update `cmd/init.go` default config to include `wiki.plugins`
- Fix `runInit()` to skip disabled types
- Fix `ensureDocsIndex()` to skip disabled types in the type list
- Tests for all changes
- Update README.md and DEVELOPMENT.md

### Out of Scope

- `wiki update` syncing plugins from config (INV-0001 Decision 4)
- New config key for wiki index template path (uses override pattern)
- Changes to `writeDefaultConfig()` type listing (INV-0001 Decision 1)

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

### Phase 1: Init Enabled Fix and Config Changes

Fix the `docz init` bug and add `Plugins` field to `WikiConfig`. These are
foundational changes that the wiki template work depends on.

#### Tasks

- [x] Fix `cmd/init.go` `runInit()` to skip types with `enabled: false`:
  - Add `tc, ok := appCfg.Types[typeName]; if !ok || !tc.Enabled { continue }`
    before creating directories and READMEs
  - Verify `writeDefaultConfig()` still lists all types (no change needed —
    it's a hardcoded config string, not driven by enabled state)
- [x] Add `Plugins []string` field to `WikiConfig` in
  `internal/config/config.go`:
  - Default in `DefaultConfig()`: `Plugins: []string{"techdocs-core"}`
  - Wire into `setDefaults()`: `v.SetDefault("wiki.plugins", cfg.Wiki.Plugins)`
- [x] Update `cmd/init.go` `writeDefaultConfig()` to include `wiki.plugins`
  in the generated `.docz.yaml`:
  ```yaml
  wiki:
    auto_update: true
    mkdocs_path: mkdocs.yml
    plugins:
      - techdocs-core
    exclude:
      - templates
      - examples
  ```
- [x] Update `writeMkDocsYAML()` in `cmd/wiki.go` to write all configured
  plugins instead of hardcoding `techdocs-core`:
  - Build the plugins YAML block from `appCfg.Wiki.Plugins`
  - Handle empty plugins list (omit plugins section entirely)
- [x] Add tests:
  - `TestInitSkipsDisabledTypes`: set a type to `enabled: false`, run
    `runInit`, verify its directory and README are not created
  - `TestDefaultConfig` — extend to verify `Wiki.Plugins` default
  - `TestLoad_WikiPlugins` — round-trip config test for plugins
  - `TestWikiInit_Plugins` — verify `mkdocs.yml` includes configured plugins
  - `TestWikiInit_MultiplePlugins` — verify multiple plugins render correctly

#### Success Criteria

- `docz init` does not create directories for disabled types
- `docz wiki init` writes all configured plugins to `mkdocs.yml`
- `go test ./...` passes
- `make lint` passes

---

### Phase 2: Wiki Index Template

Add the embedded `wiki_index.md` template with override support and rewrite
`ensureDocsIndex()` to use it. This phase delivers the customizable homepage.

#### Tasks

- [x] Create `internal/template/templates/wiki_index.md` with template content:
  ```markdown
  # {{ .SiteName }}

  Welcome to the documentation for {{ .SiteName }}.

  ## Document Types

  {{ range .Types }}- [{{ .NavTitle }}]({{ .Dir }}/README.md)
  {{ end }}
  ```
- [x] Add `EmbeddedWikiIndex()` function to `internal/template/embed.go`:
  ```go
  func EmbeddedWikiIndex() (string, error) {
      data, err := templateFS.ReadFile("templates/wiki_index.md")
      if err != nil {
          return "", fmt.Errorf("reading embedded wiki index template: %w", err)
      }
      return string(data), nil
  }
  ```
- [x] Add `WikiIndexData` struct and `ResolveWikiIndex()` function to
  `internal/template/template.go`:
  - `WikiIndexType` struct: `Name string`, `NavTitle string`, `Dir string`
  - `WikiIndexData` struct: `SiteName string`, `Types []WikiIndexType`
  - `ResolveWikiIndex(docsDir string) (string, error)` — checks for local
    override at `<docsDir>/templates/wiki_index.md`, falls back to embedded
  - `RenderWikiIndex(tmplContent string, data *WikiIndexData) (string, error)`
    — executes the template
- [x] Rewrite `ensureDocsIndex()` in `cmd/wiki.go`:
  - Resolve the wiki index template via `template.ResolveWikiIndex()`
  - Build `WikiIndexData` with site name and only enabled types
  - Render and write the result to `docs/index.md`
  - Skip writing if file already exists (preserve existing behavior)
- [x] Add tests:
  - `TestEmbeddedWikiIndex` — verify embedded template loads
  - `TestResolveWikiIndex_Embedded` — no local override, returns embedded
  - `TestResolveWikiIndex_LocalOverride` — local file takes precedence
  - `TestRenderWikiIndex` — verify template renders with site name and types
  - `TestWikiInit_IndexTemplate` — integration test: verify `docs/index.md`
    contains only enabled types
  - `TestWikiInit_IndexSkipsDisabledTypes` — disable a type, verify it's not
    in the generated `docs/index.md`
  - `TestWikiInit_IndexTemplateOverride` — place a custom template at
    `docs/templates/wiki_index.md`, verify it's used instead of the embedded one

#### Success Criteria

- `docz wiki init` renders `docs/index.md` from the template
- Disabled types are excluded from the generated homepage
- Local override at `<docs_dir>/templates/wiki_index.md` takes precedence
- `go test ./...` passes
- `make lint` passes

---

### Phase 3: Documentation, Polish, and CI Readiness

Update docs, verify edge cases, and ensure CI passes.

#### Tasks

- [x] Update `README.md`:
  - Add `wiki.plugins` to the wiki config example
  - Document the wiki index template override at
    `docs/templates/wiki_index.md`
  - Note that `docz init` skips disabled types
- [x] Update `DEVELOPMENT.md`:
  - Add `wiki_index.md` to the templates listing in the project layout
  - Note the `ResolveWikiIndex` resolution pattern
- [ ] Verify edge cases:
  - All types disabled — `docz init` creates no type directories,
    `docs/index.md` has empty type list
  - Empty plugins list — `mkdocs.yml` has no plugins section
  - Wiki index template override with custom variables (only declared
    variables work, no errors on extra template content)
- [ ] Run `make ci` and ensure it passes cleanly
- [ ] Clean up any TODO/FIXME comments

#### Success Criteria

- `make ci` passes with zero errors
- README documents wiki plugins config and index template override
- DEVELOPMENT.md reflects the new template file
- All edge cases produce correct output

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/template/templates/wiki_index.md` | Create | Embedded wiki homepage template |
| `internal/template/embed.go` | Modify | Add `EmbeddedWikiIndex()` |
| `internal/template/template.go` | Modify | Add `WikiIndexData`, `ResolveWikiIndex()`, `RenderWikiIndex()` |
| `internal/config/config.go` | Modify | Add `Plugins` field to `WikiConfig`, defaults, `setDefaults()` |
| `internal/config/config_test.go` | Modify | Add plugins config tests |
| `cmd/init.go` | Modify | Skip disabled types in `runInit()`, add `plugins` to default config |
| `cmd/wiki.go` | Modify | Rewrite `ensureDocsIndex()` to use template, update `writeMkDocsYAML()` for plugins |
| `cmd/wiki_test.go` | Modify | Add tests for plugins, disabled types, index template |
| `internal/template/template_test.go` | Modify | Add wiki index template tests |
| `README.md` | Modify | Document plugins config and index template override |
| `DEVELOPMENT.md` | Modify | Add `wiki_index.md` to project layout |

## Testing Plan

- [ ] Unit test for `EmbeddedWikiIndex()` loading
- [ ] Unit tests for `ResolveWikiIndex()` — embedded and local override
- [ ] Unit test for `RenderWikiIndex()` — template rendering
- [ ] Config tests for `Wiki.Plugins` default and round-trip
- [ ] Integration test for `docz init` skipping disabled types
- [ ] Integration test for `docz wiki init` writing configured plugins
- [ ] Integration test for `docz wiki init` rendering index template
- [ ] Integration test for index template skipping disabled types
- [ ] Integration test for index template local override

## Decisions

1. **`docz init --force` does not recreate directories for disabled types.**
   `--force` means "overwrite existing files", not "ignore config". Disabled
   types are always skipped.

2. **No ToC markers in the wiki index template.** The template is a useful
   starting point, not prescriptive. Keep it minimal so users can customize
   freely.

## Dependencies

- No new external dependencies
- Builds on existing template resolution pattern in `internal/template/`
- Builds on existing `WikiConfig` in `internal/config/`

## References

- INV-0001: Wiki Init Template and Init Enabled Fix
- DESIGN-0002: Wiki Command for MkDocs TechDocs Integration
- `internal/template/embed.go` — existing `EmbeddedIndexHeader()` pattern
- `internal/template/template.go` — existing `Resolve()` and `Render()` pattern
- `cmd/wiki.go` — `ensureDocsIndex()`, `writeMkDocsYAML()`
- `cmd/init.go` — `runInit()`
