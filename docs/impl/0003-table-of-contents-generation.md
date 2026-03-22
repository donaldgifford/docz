---
id: IMPL-0003
title: "Table of Contents Generation"
status: Draft
author: Donald Gifford
created: 2026-03-22
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0003: Table of Contents Generation

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-03-22

## Objective

Implement automatic table of contents generation integrated into `docz update`,
as specified in DESIGN-0003. Documents with `<!--toc:start-->` / `<!--toc:end-->`
markers will have their ToC regenerated from document headings on each update.

**Implements:** DESIGN-0003

## Scope

### In Scope

- `internal/toc/` package: heading parsing, slug generation, ToC rendering,
  marker-based splicing
- Fenced code block awareness (skip headings inside ``` blocks)
- GitHub-compatible anchor slugs with duplicate suffix handling (`-1`, `-2`)
- `ToCConfig` struct and integration into `Config` / `.docz.yaml`
- Wire ToC generation into `cmd/update.go` (respecting `--dry-run`)
- Update all six embedded document templates to include ToC markers
- Update `cmd/init.go` default config to include `toc` section
- Unit tests, golden file tests, integration tests
- Update README.md and DEVELOPMENT.md

### Out of Scope

- Standalone `docz toc` command (deferred per DESIGN-0003 Decision 4)
- ToC in README index files
- ToC for non-docz markdown files
- Nested/collapsible ToC formats

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

### Phase 1: Core ToC Package (`internal/toc/`)

Build the `internal/toc/` package with all the pure logic: heading parsing,
slug generation, ToC rendering, and marker splicing. No CLI integration yet —
this phase is the self-contained library.

#### Tasks

- [x] Create `internal/toc/toc.go` with exported constants, types, and functions:
  - `BeginMarker` / `EndMarker` constants (`<!--toc:start-->` / `<!--toc:end-->`)
  - `Heading` struct: `Level int`, `Text string`, `Slug string`
  - `Slugify(text string) string` — GitHub-compatible anchor generation:
    lowercase, strip non-alphanumeric (keep letters, digits, spaces, hyphens),
    spaces to hyphens, collapse multiple hyphens, trim leading/trailing hyphens
  - `ParseHeadings(content string) []Heading` — extract `##` through `######`
    headings from content after the ToC end marker; skip H1 (`#`) headings;
    track fenced code blocks (`` ``` ``) and skip headings inside them; strip
    inline markdown (bold, italic, inline code, links) from heading text; apply
    `Slugify` to generate each heading's slug; apply duplicate slug suffixes
    (`-1`, `-2`, etc.) matching GitHub behavior
  - `GenerateToC(headings []Heading, minHeadings int) string` — build indented
    markdown list with links; use relative indentation based on shallowest
    heading level; 2-space indent per level; return empty string if
    `len(headings) < minHeadings`
  - `UpdateToC(content string, minHeadings int) (string, bool)` — find markers,
    call `ParseHeadings` on content after end marker, call `GenerateToC`, splice
    result between markers; return `(content, false)` if markers not found
- [x] Create `internal/toc/toc_test.go` with unit tests:
  - `TestSlugify`: basic text, special characters, unicode, colons, slashes,
    leading/trailing hyphens, empty string
  - `TestParseHeadings`: H2-H6 levels, skip H1, skip headings before end
    marker, skip headings inside fenced code blocks, inline markdown stripping
    (bold, code, links), duplicate slug suffixes
  - `TestGenerateToC`: single level, mixed levels, relative indentation,
    min_headings threshold (below threshold returns empty), empty input
  - `TestUpdateToC`: markers present with headings, markers present but below
    threshold, no markers returns original content, empty between markers,
    existing ToC content gets replaced
- [x] Create golden file tests:
  - `internal/toc/golden_test.go` — representative document with mixed heading
    levels, code blocks, inline formatting; compare output against
    `testdata/golden/toc/basic.md`
  - Use the `-update` flag pattern from the wiki package

#### Success Criteria

- `go build ./internal/toc/...` succeeds
- `go test ./internal/toc/...` passes with all tests green
- Test coverage for `internal/toc/` is >90%
- `Slugify` produces anchors matching GitHub rendering for all test cases
- Headings inside fenced code blocks are correctly skipped
- Duplicate headings get `-1`, `-2` suffixes

---

### Phase 2: Config, CLI Integration, and Template Updates

Wire the ToC package into the config system and the `docz update` command. Update
embedded templates to include ToC markers. Update `docz init` to include the
`toc` config section.

#### Tasks

- [x] Add `ToCConfig` struct to `internal/config/config.go`:
  ```go
  type ToCConfig struct {
      Enabled     bool `mapstructure:"enabled"      yaml:"enabled"`
      MinHeadings int  `mapstructure:"min_headings" yaml:"min_headings"`
  }
  ```
- [x] Add `ToC ToCConfig` field to the `Config` struct
- [x] Set defaults in `DefaultConfig()`: `Enabled: true`, `MinHeadings: 3`
- [x] Wire `ToCConfig` defaults into `setDefaults()` for Viper:
  `v.SetDefault("toc.enabled", ...)`, `v.SetDefault("toc.min_headings", ...)`
- [x] Add config tests: `TestDefaultConfig` checks toc defaults,
  `TestLoad_ToCConfig` round-trip test
- [x] Update `cmd/init.go` `writeDefaultConfig()` to include `toc` section in
  the generated `.docz.yaml`:
  ```yaml
  toc:
    enabled: true
    min_headings: 3
  ```
- [x] Modify `cmd/update.go` `updateType()` to call ToC update on each document
  before generating the README index table:
  - Read each document file in the type directory
  - If `appCfg.ToC.Enabled` is true, call `toc.UpdateToC(content, appCfg.ToC.MinHeadings)`
  - If the ToC was updated (markers found and content changed), write the file
  - Log updates in verbose mode; warn on errors but continue
  - Respect `--dry-run`: show what would change without writing
- [x] Update all six embedded templates to include ToC markers. Placement:
  - **rfc.md**: after `**Date:** {{ .Date }}`, before `## Summary`
  - **design.md**: after `**Date:** {{ .Date }}`, before `## Overview`
  - **impl.md**: after `**Date:** {{ .Date }}`, before `## Objective`
  - **plan.md**: after `**Date:** {{ .Date }}`, before `## Goal`
  - **investigation.md**: after `**Date:** {{ .Date }}`, before `## Question`
  - **adr.md**: after `# {{ .Number }}. {{ .Title }}`, before `## Status`
    (ADR has no metadata block — markers go between H1 and first H2)
  - Marker format in templates:
    ```
    <!--toc:start-->
    <!--toc:end-->
    ```
- [x] Regenerate golden files for template rendering: run
  `go test ./internal/template/... -update` and review the diffs
- [x] Add integration tests in `cmd/`:
  - `TestUpdateGeneratesToC`: create a doc with markers and headings, run
    `updateType`, verify ToC was generated between markers
  - `TestUpdateToCDisabled`: set `appCfg.ToC.Enabled = false`, verify no ToC
    generated
  - `TestUpdateToCDryRun`: verify dry-run shows ToC changes without writing
  - `TestUpdateToCNoMarkers`: doc without markers is untouched
  - `TestCreateIncludesToCMarkers`: verify `docz create` produces docs with
    `<!--toc:start-->` / `<!--toc:end-->` markers

#### Success Criteria

- `go test ./...` passes with all tests green
- `make lint` passes with no issues
- `docz update` regenerates ToC in documents that have markers
- `docz update --dry-run` shows ToC changes without modifying files
- `docz create <type> <title>` produces documents with ToC markers
- ToC generation is skipped when `toc.enabled: false`
- Documents below `min_headings` threshold have empty ToC markers

---

### Phase 3: Documentation, Polish, and CI Readiness

Update user-facing docs, verify edge cases, ensure CI passes, and clean up.

#### Tasks

- [ ] Update `README.md`:
  - Add ToC feature bullet to Features section
  - Add `toc` config section to the example `.docz.yaml`
  - Document ToC behavior in a new "Table of Contents" section near the
    Index Tables section
- [ ] Update `DEVELOPMENT.md`:
  - Add `internal/toc/` to the project layout tree
  - Add `internal/toc` package responsibility section describing the heading
    parser, slug generator, and marker splicing
- [ ] Update `CLAUDE.md` if any new conventions emerge
- [ ] Verify edge cases manually:
  - Document with headings only inside code blocks (should produce empty ToC)
  - Document with duplicate heading text at different levels
  - Document with markers but no headings after them
  - Very long document with many headings
- [ ] Run `make ci` and ensure it passes cleanly
- [ ] Review test coverage: target >90% for `internal/toc/`, >80% overall
- [ ] Clean up any TODO/FIXME comments introduced during implementation

#### Success Criteria

- `make ci` passes with zero errors
- Test coverage >90% for `internal/toc/`
- README.md documents the ToC feature and configuration
- DEVELOPMENT.md reflects the new package
- All edge cases produce correct output

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/toc/toc.go` | Create | Core ToC logic: parsing, slugify, generation, splicing |
| `internal/toc/toc_test.go` | Create | Unit tests for all toc functions |
| `internal/toc/golden_test.go` | Create | Golden file test for ToC output |
| `testdata/golden/toc/basic.md` | Create | Golden file fixture |
| `internal/config/config.go` | Modify | Add ToCConfig struct and defaults |
| `internal/config/config_test.go` | Modify | Add toc config tests |
| `cmd/update.go` | Modify | Wire ToC generation into updateType() |
| `cmd/init.go` | Modify | Add toc section to default config |
| `internal/template/templates/rfc.md` | Modify | Add ToC markers |
| `internal/template/templates/adr.md` | Modify | Add ToC markers |
| `internal/template/templates/design.md` | Modify | Add ToC markers |
| `internal/template/templates/impl.md` | Modify | Add ToC markers |
| `internal/template/templates/plan.md` | Modify | Add ToC markers |
| `internal/template/templates/investigation.md` | Modify | Add ToC markers |
| `testdata/golden/*.md` | Modify | Regenerated via `-update` (templates changed) |
| `README.md` | Modify | Document ToC feature and config |
| `DEVELOPMENT.md` | Modify | Add internal/toc package to layout |

## Testing Plan

- [ ] Unit tests for `Slugify` — table-driven with edge cases
- [ ] Unit tests for `ParseHeadings` — heading levels, code blocks, inline markdown
- [ ] Unit tests for `GenerateToC` — indentation, min_headings, empty input
- [ ] Unit tests for `UpdateToC` — marker splicing, missing markers, replacement
- [ ] Golden file test for representative document
- [ ] Config tests for ToCConfig defaults and round-trip loading
- [ ] Integration tests for `docz update` with ToC
- [ ] Integration test for `docz create` including markers

## Decisions

1. **Inline markdown stripping matches GitHub anchor behavior.** Strip bold
   (`**`/`__`), italic (`*`/`_`), inline code (`` ` ``), and links
   (`[text](url)` → keep text). Skip images and HTML tags — they're
   extremely rare in headings and can be added later if needed.

2. **Dry-run shows concise per-file summary.** `docz update --dry-run` prints
   a one-liner per file like `"Would update ToC in docs/rfc/0001-my-rfc.md
   (8 headings)"`. Full content output is available with `--verbose`.

3. **ToC formatting uses `-` list markers with no blank lines around marker
   content.** Matches `.markdownlint.yaml` (MD004 disabled but `-` is the
   standard marker). No blank line after `<!--toc:start-->` or before
   `<!--toc:end-->` unless the lazyvim plugin output requires it — verify
   exact format during Phase 1 implementation and match it.

## Dependencies

- No new external dependencies required
- Uses only Go standard library (`strings`, `regexp`, `os`, `fmt`)
- Builds on existing patterns from `internal/index/` (marker splicing) and
  `internal/wiki/` (golden file testing)

## References

- DESIGN-0003: Table of Contents Generation
- `internal/index/index.go` — `spliceMarkers()` pattern to reuse
- `internal/wiki/golden_test.go` — golden file test pattern to follow
- `internal/config/config.go` — config struct pattern for ToCConfig
