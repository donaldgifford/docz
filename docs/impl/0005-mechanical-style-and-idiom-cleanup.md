---
id: IMPL-0005
title: "Mechanical Style and Idiom Cleanup"
status: Draft
author: Donald Gifford
created: 2026-05-15
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0005: Mechanical Style and Idiom Cleanup

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-05-15

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Centralize scattered constants](#phase-1-centralize-scattered-constants)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Modernize Go stdlib idioms](#phase-2-modernize-go-stdlib-idioms)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Cobra and style polish](#phase-3-cobra-and-style-polish)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Verify and ship](#phase-4-verify-and-ship)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Decisions](#decisions)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

## Objective

Apply the low-risk mechanical fixes from INV-0002 Wave 1: centralize scattered
constants, modernize Go standard-library idioms, and polish Cobra and style
issues that have zero design risk. The goal is a single "style sweep" PR that
improves readability without changing any behavior.

**Implements:** INV-0002 (Wave 1 — Mechanical wins)

## Scope

### In Scope

- Centralize file-mode and filename constants (F36, F37, F35)
- Replace `os.IsNotExist` with `errors.Is(err, fs.ErrNotExist)` (F28)
- Replace `sort.Slice` with `slices.SortFunc` (F29)
- Replace `strings.NewReader(string(data))` with `bytes.NewReader(data)` (F30)
- Replace path string-concat with `filepath.Join` (F31)
- Replace hand-rolled `itoa` with `strconv.Itoa` (F32)
- Format `currentDate` via `time.DateOnly`; capture `timeNow()` once (F9)
- Add `defer enc.Close()` in `cmd/config.go` (F33)
- Switch `cmd/version.go` from `Run` to `RunE` (F43)
- Set `SilenceUsage: true` on root command (F44)
- Move package doc-comment from `internal/wiki/titles.go` to `wiki.go` (F45)
- Drop named return values in `config.Validate()` (F27)

### Out of Scope

- Any change that alters public API surface or YAML config shape
- Renaming `TemplateData` / `ToCConfig` (deferred to IMPL-0008 — has caller
  fanout that's larger than mechanical)
- Anything touching `cmd/` globals or output routing (deferred to IMPL-0009)
- Behavioral changes to defaults loading (deferred to IMPL-0006)

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

### Phase 1: Centralize scattered constants

Create a single source of truth for file modes, well-known filenames, and a
handful of magic numbers that appear inline today. This phase is purely
mechanical extraction — no logic changes.

#### Tasks

- [x] Decide constant home: extend `internal/config` or add `internal/doczfs`
      (see Decisions §1)
- [x] Define `FileMode os.FileMode = 0o644` and `DirMode os.FileMode = 0o750`
      constants in the chosen package
- [x] Replace all 13 inline `0o644` occurrences with `FileMode` (across
      `cmd/init.go:177,204`, `cmd/template.go:131`,
      `internal/index/index.go:109,166`, `internal/document/create.go:78`,
      and others)
- [x] Replace all 4 inline `0o750` occurrences with `DirMode` (across
      `cmd/init.go:46`, `cmd/template.go:127`,
      `internal/index/index.go:162`, `internal/document/create.go:43`)
- [x] Define filename constants: `ConfigFileName = ".docz.yaml"`,
      `IndexFileName = "README.md"`, `WikiIndexName = "index.md"`,
      `MkDocsFileName = "mkdocs.yml"`, `TemplatesDir = "templates"`
- [x] Replace literal usages across `cmd/init.go:50,61`, `cmd/update.go:64`,
      `cmd/template.go:115`, `cmd/wiki.go:216,292`,
      `internal/config/config.go:134,163,169`,
      `internal/template/template.go:72,98`
- [x] Define `const defaultMinHeadings = 3` and reference it from
      `internal/config/config.go:141`
- [x] Document why `maxSlugLength = 64` (filesystem path limit) inline at
      `internal/template/template.go:31`

#### Success Criteria

- `grep -rn '0o644\|0o750' cmd/ internal/` returns only the constant
  declarations
- `grep -rn '"\.docz\.yaml"\|"README\.md"\|"mkdocs\.yml"' cmd/ internal/`
  returns only the constant declarations
- `go build ./...` succeeds
- `make test` passes with zero golden-file diffs

---

### Phase 2: Modernize Go stdlib idioms

Replace legacy patterns with the modern equivalents available in Go 1.25.7.
Each substitution is local; no APIs change.

#### Tasks

- [x] Replace `os.IsNotExist(err)` with `errors.Is(err, fs.ErrNotExist)` at:
  - `cmd/wiki.go:120`
  - `internal/index/index.go:36`
  - `internal/index/index.go:95`
  - `internal/index/index.go:120`
- [x] Add `io/fs` imports where needed
- [x] Replace `sort.Slice` with `slices.SortFunc` at:
  - `internal/index/index.go:64`
  - `internal/wiki/wiki.go:104`
  - `internal/wiki/wiki.go:109`
  - `internal/wiki/wiki.go:114`
  - `internal/wiki/wiki.go:148`
  - `internal/wiki/mkdocs.go:123`
- [x] Drop `sort` imports where they become unused; add `slices` imports
- [x] Replace `bufio.NewScanner(strings.NewReader(string(data)))` with
      `bufio.NewScanner(bytes.NewReader(data))` at `internal/wiki/titles.go:57`
- [x] Add `scanner.Err()` check after the `firstH1` loop (currently missing)
- [x] Replace path string-concat with `filepath.Join` at
      `internal/template/template.go:72` (note: `internal/template/embed.go`
      uses must stay forward-slash for `embed.FS`)
- [x] Replace path string-concat with `filepath.Join` at
      `internal/template/template.go:98`
- [x] Delete `internal/toc/toc.go:itoa()` (lines 209-219); add `strconv` import
      to `toc.go`; replace call site at `toc.go:133` with `strconv.Itoa(...)`
- [x] Replace `currentDate()` body with `timeNow().Format(time.DateOnly)` at
      `internal/document/create.go:118-121`

#### Success Criteria

- `grep -rn 'os\.IsNotExist\|sort\.Slice\|strings\.NewReader(string(' cmd/ internal/`
  returns no results
- `grep -rn 'func itoa' internal/toc/` returns no results
- `internal/wiki/titles.go` no longer imports `strings` solely for `NewReader`
- `make ci` passes
- `make test` passes with zero golden-file diffs

---

### Phase 3: Cobra and style polish

Small Cobra hygiene and a handful of cosmetic style fixes.

#### Tasks

- [x] Add `defer enc.Close()` in `cmd/config.go:24-30`; remove the explicit
      `return enc.Close()` so the deferred call runs on all return paths
- [x] Change `cmd/version.go:21` from `Run:` to `RunE:` with a `func() error`
      that returns `nil`
- [x] Set `SilenceUsage: true` on `rootCmd` at `cmd/root.go:36`
- [x] Move the `// Package wiki ...` doc-comment from
      `internal/wiki/titles.go:1` to a new comment block at the top of
      `internal/wiki/wiki.go`
- [x] Convert `func (c *Config) Validate() (warnings []string, err error)` at
      `internal/config/config.go:236` to `(c *Config) Validate() ([]string, error)`
      with a local `var warnings []string`; replace all `err =` assignments
      with `return warnings, fmt.Errorf(...)`
- [x] Use `errors.New` for static-string errors at
      `internal/config/config.go:238` and `:252`,
      `internal/document/document.go:44` (sentinel candidates flagged in
      INV-0002 deferred to IMPL-0006). `:252` keeps `fmt.Errorf` because it
      includes a `%q` verb; only the truly static strings were swapped.

#### Success Criteria

- `cmd/config.go` does not leak a YAML encoder on error
- Running `docz somecommand --bogus-flag` no longer prints the full usage
  block on a `RunE` error (only the error message)
- `go vet ./...` is clean
- `make ci` passes

---

### Phase 4: Verify and ship

Run the full quality gate and confirm zero behavioral change.

#### Tasks

- [ ] Run `make fmt` to normalize formatting
- [ ] Run `make lint` and resolve any new warnings
- [ ] Run `make test` and confirm all golden files unchanged
- [ ] Run `make ci` end-to-end
- [ ] Smoke-test on this repo: `docz list`, `docz create inv "smoke test"`,
      `docz update`, `docz config`, `docz wiki update`
- [ ] Open PR with label `dont-release` (style cleanup only, no user-visible
      change)
- [ ] Confirm no `.docz.yaml` user-config files would break

#### Success Criteria

- `make ci` passes
- All golden files identical (no `-update` regeneration needed)
- Manual smoke test succeeds for the five core commands
- PR diff contains zero behavior changes (only renames, imports, and
  constant references)

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/config/config.go` or new `internal/doczfs/fs.go` | Create or modify | Define `FileMode`, `DirMode`, and filename constants |
| `cmd/init.go` | Modify | Use centralized constants; doc comment cleanup |
| `cmd/update.go` | Modify | Use centralized constants |
| `cmd/template.go` | Modify | Use centralized constants |
| `cmd/wiki.go` | Modify | Use centralized constants; `errors.Is` |
| `cmd/config.go` | Modify | `defer enc.Close()` |
| `cmd/root.go` | Modify | `SilenceUsage: true` |
| `cmd/version.go` | Modify | `Run` → `RunE` |
| `internal/index/index.go` | Modify | `errors.Is`, `slices.SortFunc`, constants |
| `internal/wiki/wiki.go` | Modify | `slices.SortFunc`, package doc comment |
| `internal/wiki/titles.go` | Modify | `bytes.NewReader`, package doc removed |
| `internal/wiki/mkdocs.go` | Modify | `slices.SortFunc` |
| `internal/template/template.go` | Modify | `filepath.Join` |
| `internal/document/create.go` | Modify | `time.DateOnly`; capture `timeNow()` |
| `internal/document/document.go` | Modify | `errors.New` for static error |
| `internal/toc/toc.go` | Modify | Delete `itoa`; use `strconv.Itoa` |
| `internal/config/config.go` | Modify | Drop named returns; named constants |

## Testing Plan

- [ ] Run existing test suite (`make test`) and confirm zero golden-file diffs
- [ ] Add a single regression test that asserts `currentDate` returns a
      stable `YYYY-MM-DD` string when `timeNow` is pinned (verifies the
      `time.DateOnly` swap)
- [ ] Add a regression test for `firstH1` that exercises a file with CRLF
      line endings (sanity-checks the `bytes.NewReader` swap; full CRLF
      handling deferred to IMPL-0006)
- [ ] Manual smoke test against the docz repo itself

## Decisions

Resolved during INV-0002 planning review.

1. **Centralized constants home:** `internal/config`. No new package.
2. **`os.IsNotExist` swap scope:** rewrite the four `os.IsNotExist` sites to
   `errors.Is(err, fs.ErrNotExist)`. Leave `err == nil` "file-exists"
   idioms at `cmd/wiki.go:85,102,218,294` as-is.
3. **`sort` import audit:** confirmed as an implementation-time grep step,
   not a design decision.
4. **Date layout:** `time.DateOnly` (stdlib constant; Go 1.20+).
5. **`scanner.Err()` in `firstH1`:** add the check, log via the existing
   verbose path; never return an error from `firstH1` (preserves current
   contract).
6. **`//nolint:funlen` directive on `writeDefaultConfig`:** delete it in
   this wave. The function will be rewritten in IMPL-0006; the directive
   is already stale.

## Dependencies

- None. This wave has no design prerequisites and no external blockers.
- Must merge before IMPL-0006 (which expects the modernized idioms as the
  baseline).

## References

- INV-0002: Architectural Review and Cleanup Opportunities — Wave 1
  recommendation, findings F9, F27, F28–F37, F43–F45
- Uber Go Style Guide — `errors.Is`, `slices`, time formatting, named returns
- Effective Go — package doc comment placement
