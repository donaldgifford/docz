---
id: IMPL-0008
title: "Move Stranded Business Logic Into Internal Packages"
status: Draft
author: Donald Gifford
created: 2026-05-15
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0008: Move Stranded Business Logic Into Internal Packages

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-05-15

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1 (PR A): Move writeMkDocsYAML into internal/wiki](#phase-1-pr-a-move-writemkdocsyaml-into-internalwiki)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2 (PR A): Move updateToCs into internal/toc](#phase-2-pr-a-move-updatetocs-into-internaltoc)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3 (PR A): Extract shared nav-building helper](#phase-3-pr-a-extract-shared-nav-building-helper)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4 (PR B): Split internal/index](#phase-4-pr-b-split-internalindex)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5 (PR B): Single LoadFrontmatter(path) helper](#phase-5-pr-b-single-loadfrontmatterpath-helper)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6 (PR B): Single DoczFilePattern regex](#phase-6-pr-b-single-doczfilepattern-regex)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7 (PR B): Rename Slugify functions](#phase-7-pr-b-rename-slugify-functions)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
  - [Phase 8 (PR B): Clean up DocTitle return contract](#phase-8-pr-b-clean-up-doctitle-return-contract)
    - [Tasks](#tasks-7)
    - [Success Criteria](#success-criteria-7)
  - [Phase 9 (PR B): Typed index.UpdateOutcome](#phase-9-pr-b-typed-indexupdateoutcome)
    - [Tasks](#tasks-8)
    - [Success Criteria](#success-criteria-8)
  - [Phase 10 (PR B): Rename TemplateData and ToCConfig](#phase-10-pr-b-rename-templatedata-and-tocconfig)
    - [Tasks](#tasks-9)
    - [Success Criteria](#success-criteria-9)
  - [Phase 11: Verify and ship](#phase-11-verify-and-ship)
    - [Tasks](#tasks-10)
    - [Success Criteria](#success-criteria-10)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Decisions](#decisions)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

## Objective

Move business logic out of `cmd/` into testable `internal/` packages, deduplicate
the two `Slugify` implementations and three docz-file regex variants, and rename
stuttering types. After this wave the `cmd/` package should be a thin
flag-parsing + wiring layer; all logic should be unit-testable without going
through Cobra.

**Implements:** INV-0002 (Wave 4 — Stranded business logic)

This wave is large enough to ship as **two sequential PRs**:

- **PR A** — Move functions out of `cmd/` (Phases 1–3)
- **PR B** — Restructure internal packages and rename types (Phases 4–10)

PR A can ship independently; PR B builds on it.

## Scope

### In Scope

- Move `writeMkDocsYAML` → `internal/wiki.CreateMkDocs` (F17)
- Move `updateToCs` → `internal/toc.UpdateFiles` (F18)
- Extract shared nav-building helper for the wiki update paths (F19)
- Split `internal/index`: move `ScanDocuments` + `DocEntry` → `internal/document` (F20)
- Single `document.LoadFrontmatter(path)` helper used by both index and wiki (F21)
- Single `document.DoczFilePattern` regex; delete duplicates (F22)
- Rename `template.Slugify` → `template.FilenameSlug`;
  `toc.Slugify` → `toc.AnchorSlug` (F23)
- Clean up `DocTitle` return contract (F11)
- Typed `index.UpdateOutcome` instead of `(msg, err)` (F13)
- Rename `template.TemplateData` → `template.Data` (F24)
- Consider renaming `template.WikiIndexType/Data` (F25)
- Rename `ToCConfig` → `TOCConfig`, field `ToC` → `TOC` (F26)

### Out of Scope

- Introducing a `Runner` struct or changing how `cmd/` handlers receive
  state (IMPL-0009)
- Changing `cmd/` global flags (IMPL-0009)
- Introducing typed `DocType` / `Status` (IMPL-0009)
- Adding logger abstraction (IMPL-0009)

## Implementation Phases

---

### Phase 1 (PR A): Move `writeMkDocsYAML` into `internal/wiki`

Promote the initial-mkdocs.yml generator from the cmd layer to the wiki
package alongside the existing `ReadMkDocs`/`WriteMkDocs`.

#### Tasks

- [x] Define `wiki.MkDocsConfig` struct in `internal/wiki/mkdocs.go` with
      fields: `SiteName`, `SiteDescription`, `DocsDir`, `RepoURL`, `SiteURL`,
      `Theme`, `Plugins`, `MarkdownExtensions`
- [x] Add `wiki.CreateMkDocs(path string, cfg *MkDocsConfig) error` that
      builds the YAML and writes it (pointer receiver per gocritic
      `hugeParam`; matches `template.RenderWikiIndex` pattern)
- [x] Move the string-building loop from `cmd/wiki.go:247-289` into
      `wiki.CreateMkDocs`
- [x] Update `cmd/wiki.go:runWikiInit` to populate `MkDocsConfig` from
      `appCfg.Wiki` and call `wiki.CreateMkDocs`
- [x] Delete `cmd/wiki.go:writeMkDocsYAML`
- [x] Add table-driven tests in `internal/wiki/mkdocs_test.go` covering:
      minimal config (only site_name), full config (all optional fields),
      plugins ordering, markdown_extensions presence
- [x] Add a golden file under `testdata/golden/wiki/mkdocs_full.yml` for
      the full-config case

#### Success Criteria

- `wiki.CreateMkDocs` is fully unit-testable without `appCfg`
- `cmd/wiki.go` no longer constructs YAML strings inline
- Golden files for `wiki init` unchanged

---

### Phase 2 (PR A): Move `updateToCs` into `internal/toc`

Promote the file-iteration ToC updater from cmd into the toc package.

#### Tasks

- [x] Add `toc.UpdateFiles(files []FileInput, minHeadings int, dryRun bool) (UpdateReport, error)`
      where `UpdateReport` carries `Updated`, `Unchanged`, `WouldUpdate`
      slices of `FileResult{Path, Headings}`, plus `Skipped []string` and
      `WriteErrors []FileError`
- [x] Move the loop from `cmd/update.go:updateToCs` into `toc.UpdateFiles`
- [x] Take `[]toc.FileInput{Path, Content}` as input per Decisions §1
      (preserves the IMPL-0007 byte-cache; no import cycle into
      `internal/document`)
- [x] Update `cmd/update.go:updateType` to build the FileInput list from
      the scan results and call `toc.UpdateFiles`
- [x] Move user-facing strings out of the library — the cmd layer formats
      messages from `report.Updated/Unchanged/WouldUpdate/Skipped/WriteErrors`
- [x] Add tests in `internal/toc/update_test.go` covering: dry-run, real
      update, no-markers (Skipped), idempotent re-run, write-error
      isolation, empty input

#### Success Criteria

- `toc.UpdateFiles` does not call `fmt.Println` directly
- `cmd/update.go:updateToCs` is deleted; the call site is one line
- `make test` green

---

### Phase 3 (PR A): Extract shared nav-building helper

`runWikiUpdateNav` and `runWikiUpdateDryRun` have ~80% overlapping bodies.
Extract the shared piece.

#### Tasks

- [x] Add `wiki.BuildNav(docsDir, exclude, navTitles, existingOrder) ([]NavEntry, error)`
      that encapsulates: `ScanDocs` → `ExistingNavOrder` decision →
      `MergeNavOrder` or `SortEntries`
- [x] Replace the bodies of `runWikiUpdateNav` and `runWikiUpdateDryRun`
      with calls to `wiki.BuildNav`; verbose logging hoisted to
      `logScan` / `logScanResult` helpers; only the "write vs. print"
      final step diverges
- [x] Add tests for `wiki.BuildNav` covering: empty existing order,
      partial existing order, no docs found, scan error

#### Success Criteria

- The two cmd functions are each <30 lines
- Nav-building logic is unit-testable without touching `cmd/`
- Wiki golden files unchanged

---

### Phase 4 (PR B): Split `internal/index`

Move `ScanDocuments` + `DocEntry` into `internal/document`. `internal/index`
keeps README splicing only.

#### Tasks

- [ ] Move `ScanDocuments`, `DocEntry`, and `docFilePattern` from
      `internal/index/index.go` into a new file `internal/document/scan.go`
- [ ] Update `DocEntry` to use the shared `document.DoczFilePattern`
      (introduced in Phase 6)
- [ ] Re-export `DocEntry` from `internal/index` as a type alias for one
      release to avoid breaking imports (or delete cleanly — see
      Decisions §2)
- [ ] Update `cmd/list.go`, `cmd/update.go` to import from
      `internal/document`
- [ ] Move `internal/index/index_test.go` tests that target `ScanDocuments`
      to `internal/document/scan_test.go`; keep tests that target
      `UpdateReadme`/`GenerateTable` in `internal/index`

#### Success Criteria

- `internal/index/index.go` contains only README-splicing logic
  (`UpdateReadme`, `DryRunReadme`, `GenerateTable`, `spliceMarkers`,
  `createNewReadme`)
- `internal/document` exports `ScanDocuments` and `DocEntry`
- Import graph: `index` → `document` (one-way; no cycles)

---

### Phase 5 (PR B): Single `LoadFrontmatter(path)` helper

Today both `internal/index` and `internal/wiki` open files, read them,
and call `ParseFrontmatter` with different error handling. Consolidate.

#### Tasks

- [ ] Add `document.LoadFrontmatter(path string) (Frontmatter, []byte, error)`
      that reads the file and parses; returns bytes alongside frontmatter
      so callers like the new `ScanDocuments` get both in one call
- [ ] Replace the file-read + parse block in `document.ScanDocuments`
      (after Phase 4 move) with `LoadFrontmatter`
- [ ] Replace the equivalent block in `wiki.DocTitle` with `LoadFrontmatter`
- [ ] Document the contract: "returns `ErrNoFrontmatter` for files without
      frontmatter (not an error); other errors are fatal"
- [ ] Update callers to use `errors.Is(err, document.ErrNoFrontmatter)`
      where they fall back to filename

#### Success Criteria

- Exactly one site in the codebase reads bytes + parses frontmatter
- Error contract documented and tested

---

### Phase 6 (PR B): Single `DoczFilePattern` regex

Three different regexes for "is this a docz file" today. Pick one.

#### Tasks

- [ ] Define `document.DoczFilePattern = regexp.MustCompile(...)` with the
      canonical shape `^\d+-.*\.md$`
- [ ] Add `document.IsDoczFile(name string) bool` convenience function
- [ ] Replace the regex at `internal/wiki/wiki.go:12` with
      `document.IsDoczFile(name)`
- [ ] Replace the regex at `internal/index/index.go:21` (moved to
      `internal/document/scan.go` in Phase 4) with `DoczFilePattern`
- [ ] Replace the regex at `internal/document/create.go:36` with
      `DoczFilePattern`
- [ ] Document the invariant in a single doc-comment

#### Success Criteria

- `grep -rn 'regexp\.MustCompile.*-\.\*\\.md' internal/` returns one
  match (the canonical definition)

---

### Phase 7 (PR B): Rename `Slugify` functions

Two `Slugify` functions today with different algorithms. Rename for clarity.

#### Tasks

- [ ] Rename `template.Slugify` → `template.FilenameSlug`; update doc
      comment to explain "kebab-case for filenames, max 64 chars,
      word-boundary truncation"
- [ ] Rename `toc.Slugify` → `toc.AnchorSlug`; update doc comment to
      reference the GitHub anchor algorithm
- [ ] Update all call sites (audit with `grep -rn '\.Slugify' .`)
- [ ] Update tests
- [ ] Confirm `nonAlphanumHyphen` regex variable name renamed to
      something clearer (`nonSlugChar` per INV-0002 note 11.2)

#### Success Criteria

- Two clearly-named slug functions, each with explicit docs
- `grep -rn 'Slugify' .` returns no results in production code

---

### Phase 8 (PR B): Clean up `DocTitle` return contract

`wiki.DocTitle` currently returns a fallback value alongside a non-nil
error. Decide the contract.

#### Tasks

- [ ] Change `DocTitle(filePath string) (string, error)` to never return
      a value with an error: on read failure return `"", err`
- [ ] Move the filename-fallback logic to the caller (`wiki.scanDir`),
      which already has the filename and can call `wiki.FilenameTitle`
      explicitly
- [ ] Or alternatively, return `Result{Title string, IsFallback bool}` —
      see Decisions §3
- [ ] Update tests

#### Success Criteria

- `DocTitle` follows the standard "value OR error" contract
- Callers explicitly choose the fallback path

---

### Phase 9 (PR B): Typed `index.UpdateOutcome`

`UpdateReadme` and `DryRunReadme` currently return user-facing strings as
the success value. Replace with a typed result.

#### Tasks

- [ ] Define `index.UpdateAction int` enum with values: `ActionCreated`,
      `ActionUpdated`, `ActionNoMarkers`, `ActionDryRunCreated`,
      `ActionDryRunUpdated`
- [ ] Define `index.UpdateOutcome struct { Action UpdateAction; Path string; Body string }`
      where `Body` carries the would-be content for dry-run
- [ ] Change `UpdateReadme` and `DryRunReadme` to return `UpdateOutcome` +
      `error`
- [ ] Update `cmd/update.go:updateType` to format messages from
      `UpdateOutcome.Action` (e.g., `switch outcome.Action { case ActionCreated: fmt.Printf("Created %s", outcome.Path) ... }`)
- [ ] Add tests

#### Success Criteria

- `internal/index` does not produce English user-facing strings
- The cmd layer is the only place that formats user-facing messages

---

### Phase 10 (PR B): Rename `TemplateData` and `ToCConfig`

Drop the stutter and fix the initialism casing.

#### Tasks

- [ ] Rename `template.TemplateData` → `template.Data`
- [ ] Update all call sites: `internal/document/create.go:61`, tests, golden
      generators
- [ ] Consider renaming `template.WikiIndexType`/`WikiIndexData` — they
      stutter only mildly (qualified with `template.` they read fine).
      See Decisions §4.
- [ ] Rename `config.ToCConfig` → `config.TOCConfig`
- [ ] Rename `config.Config.ToC` → `config.Config.TOC` (field)
- [ ] **Critical:** keep the YAML tag `toc:` and mapstructure tag `"toc"`
      unchanged so existing `.docz.yaml` files continue to work
- [ ] Update all call sites: `cmd/update.go:80`, `internal/config/config.go`,
      tests
- [ ] Add a back-compat test: a `.docz.yaml` with `toc:` parses identically

#### Success Criteria

- No `TemplateData` or `ToCConfig` identifiers remain
- Existing `.docz.yaml` files in user repos continue to work
- `make ci` green

---

### Phase 11: Verify and ship

#### Tasks

- [ ] Run `make ci`
- [ ] Smoke test full CLI surface against this repo
- [ ] Verify golden files: only intentional changes
- [ ] Open PR A with `dont-release` label (Phases 1–3)
- [ ] After PR A merges: rebase, open PR B (Phases 4–10) with `dont-release`
      label
- [ ] Update INV-0002 status

#### Success Criteria

- Two PRs merged with green CI
- Manual smoke test passes
- The `cmd/` package contains no business logic — only flag parsing,
  config wiring, and calls into `internal/`

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/wiki/mkdocs.go` | Modify | Add `MkDocsConfig`, `CreateMkDocs` |
| `cmd/wiki.go` | Modify | Delete `writeMkDocsYAML`; extract `BuildNav` callers |
| `internal/wiki/wiki.go` | Modify | Add `BuildNav`; use `document.DoczFilePattern` |
| `internal/toc/toc.go` | Modify | Add `UpdateFiles`, `UpdateReport`; rename `Slugify` |
| `cmd/update.go` | Modify | Delete `updateToCs`; consume `UpdateOutcome` and `UpdateReport` |
| `internal/document/scan.go` | Create | `ScanDocuments`, `DocEntry` (moved from index) |
| `internal/document/document.go` | Modify | Add `LoadFrontmatter`, `DoczFilePattern`, `IsDoczFile` |
| `internal/index/index.go` | Modify | Keep splicing only; return `UpdateOutcome` |
| `internal/template/template.go` | Modify | Rename `TemplateData` → `Data`; rename `Slugify` → `FilenameSlug` |
| `internal/config/config.go` | Modify | Rename `ToCConfig` → `TOCConfig`; field `ToC` → `TOC` |
| Tests + golden files | Modify | Mirror all of the above |

## Testing Plan

- [ ] Each moved function gets unit tests in its new location (most will
      be copied + adapted from existing tests)
- [ ] Back-compat test for `.docz.yaml` parsing with `toc:` key
- [ ] Smoke test: run docz against the docz repo, verify all generated
      files identical
- [ ] Golden file regen happens exactly once (renames + reorganization
      shouldn't change output)

## Decisions

Resolved during INV-0002 planning review.

1. **`toc.UpdateFiles` input type:** `[]toc.FileInput{Path string;
   Content []byte}` — a local input struct defined in `internal/toc`.
   Preserves the IMPL-0007 byte-cache gain without forcing an import
   cycle into `internal/document`.
2. **Type-alias for back-compat after the `internal/index` split:**
   clean delete. `internal/index` is internal — no external consumers.
   Document the move in the PR description.
3. **`DocTitle` return contract:** strict `(string, error)`. On read
   failure, return `"", err`. The single caller (`scanDir`) already
   has the filename and calls `FilenameTitle` explicitly when it
   wants a fallback.
4. **`WikiIndexType` / `WikiIndexData` rename:** leave as-is. The
   `Wiki` prefix carries useful meaning and distinguishes from a
   future doc-index template.
5. **`ToC` → `TOC` rename release-note treatment:** add a brief
   release-note line explaining the internal-only rename. The YAML
   key `toc:` is unchanged; user configs continue to work.
6. **PR A and PR B branching:** merge PR A first, then start PR B
   from fresh main. Simpler, lower merge-conflict risk.
7. **`UpdateReport` placement:** defined in `internal/toc` alongside
   `UpdateFiles`. Not used by other packages.

## Dependencies

- Builds on IMPL-0006 (`EnabledTypes`, `ValidateType` helpers in use)
- Builds on IMPL-0007 (`DocEntry.Content` field is the source for
  `toc.UpdateFiles` bytes)
- Blocks IMPL-0009 (the Runner refactor expects the slimmer `cmd/` layer
  this wave produces)

## References

- INV-0002 — Wave 4, findings F11, F13, F17–F26
- IMPL-0007 — `DocEntry.Content` field is required input for Phase 2
- Effective Go — package responsibility, doc comment placement
