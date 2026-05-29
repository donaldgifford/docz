---
id: IMPL-0009
title: "Runner Pattern and DocType Registry Refactor"
status: Draft
author: Donald Gifford
created: 2026-05-15
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0009: Runner Pattern and DocType Registry Refactor

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-05-15

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Author DESIGN doc](#phase-1-author-design-doc)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Introduce Runner struct (no behavior change)](#phase-2-introduce-runner-struct-no-behavior-change)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Migrate handlers to Runner methods + output writers](#phase-3-migrate-handlers-to-runner-methods--output-writers)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Introduce log/slog logger; eliminate if verbose](#phase-4-introduce-logslog-logger-eliminate-if-verbose)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Inject time into document.CreateOptions](#phase-5-inject-time-into-documentcreateoptions)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Inject git resolution; propagate cmd.Context()](#phase-6-inject-git-resolution-propagate-cmdcontext)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: Add repoRoot to config.Load](#phase-7-add-reporoot-to-configload)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
  - [Phase 8: Introduce DocType registry](#phase-8-introduce-doctype-registry)
    - [Tasks](#tasks-7)
    - [Success Criteria](#success-criteria-7)
  - [Phase 9: Drive iteration from EnabledTypes()](#phase-9-drive-iteration-from-enabledtypes)
    - [Tasks](#tasks-8)
    - [Success Criteria](#success-criteria-8)
  - [Phase 10: Introduce typed DocType and Status strings](#phase-10-introduce-typed-doctype-and-status-strings)
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

The largest and most architectural of the INV-0002 waves. Eliminate the
two systemic blockers to clean tests and clean extension:

1. **Package-level globals in `cmd/`** — every flag and `appCfg` lives as
   a `var`, blocking parallel tests and forcing `os.Pipe` tricks for
   output capture.
2. **`DocType` scattered across 6+ locations** — no single source of
   truth for the closed set of document types.

Introduce a `Runner` struct that holds resolved config and writer
dependencies, convert command handlers to methods on `Runner`, and replace
the ad-hoc type definitions with a `DocType` registry. Add `log/slog`
logging, injectable time + git resolution, and an explicit `repoRoot`
parameter for `config.Load`.

This wave is gated on a **DESIGN doc** that aligns on the Runner shape and
DocType registry API before implementation begins.

**Implements:** INV-0002 (Wave 5 — Architecture refactor)

## Scope

### In Scope

- Author a DESIGN doc covering Runner pattern + DocType registry
- Introduce `cmd.Runner` struct (config + `io.Writer` for stdout/stderr +
  injected dependencies)
- Convert all command `runFoo` functions into `(*Runner).Foo()` methods
- Migrate `fmt.Printf` / `os.Stdout` calls to `cmd.Println` /
  `cmd.OutOrStdout()` (50+ sites, F2)
- Introduce `log/slog` logger; replace 20+ `if verbose { ... }` blocks (F3)
- Inject `time.Time` into `document.CreateOptions`; delete the package
  `timeNow` global (F4)
- Make `gitUserName` injectable; propagate `cmd.Context()` (F5)
- Add `repoRoot string` parameter to `config.Load`; eliminate `os.Chdir`
  in tests (F6)
- Introduce `DocType` registry: single struct bundling name, aliases,
  default `TypeConfig`, nav title, template name (F14)
- Drive type iteration from the registry instead of `ValidTypes()`
  hardcoded slice (F15)
- Introduce `type DocType string` and `type Status string` typed strings
  for compile-time signal (F16)

### Out of Scope

- Adding new user-facing commands or flags
- Changing the YAML config schema (typed strings should not break
  user-written `.docz.yaml` files)
- Migrating from Cobra to a different CLI framework
- Internationalization / structured logging consumers

## Implementation Phases

---

### Phase 1: Author DESIGN doc

Before writing any code, align on the architectural shape via a DESIGN
document. This is the prerequisite gate.

#### Tasks

- [x] Create DESIGN doc: `docz create design "Runner Pattern and DocType Registry"`
      — DESIGN-0004 scaffolded 2026-05-27
- [x] DESIGN doc must cover:
  - Runner struct shape and lifecycle (constructed where? scoped per
    command vs. per process?) — DESIGN-0004 §A
  - How flags bind to per-command options structs vs. Runner fields —
    DESIGN-0004 §B
  - Output writers — single writer or stdout+stderr split, how `--quiet`
    integrates — DESIGN-0004 §C
  - Logger handler: text or JSON, configurable level, where it's threaded
    — DESIGN-0004 §D
  - DocType registry API: registration model (init-time or explicit?),
    aliasing model, lookup model — DESIGN-0004 §E
  - Typed `DocType` / `Status` migration: where the alias is enforced,
    YAML tag compatibility, validation surface — DESIGN-0004 §F
    (revises this IMPL's Decision §3 — no custom YAML unmarshaler
    required)
  - Library/pattern evaluation:
    - `charmbracelet/fang` — DESIGN-0004 §G: rejected (experimental;
      no payoff for current scope)
    - Bubble Tea v2 alongside Cobra — DESIGN-0004 §G: rejected (no TUI
      requirement today)
    - `localstack/lstk` — DESIGN-0004 §G: pattern reference for
      per-command options structs, not a dependency
  - Test strategy: how does a typical handler test look post-refactor?
    — DESIGN-0004 §H
  - Migration plan: can the refactor land in one PR or must be split?
    — DESIGN-0004 §Migration: single PR for phases 2–11 with an 11-commit
    sequence
- [x] DESIGN doc reviewed and accepted (status: Approved) — flipped
      from `In Review` after the full Phases 2–11 implementation
      shipped lint-clean and race-clean, validating the design's
      load-bearing claims (Runner construction, DocType registry,
      `repoRoot` parameter, stacked-PR split). Frontmatter and
      narrative status updated together.

#### Success Criteria

- DESIGN doc status is `Approved`
- Any new open questions raised by the DESIGN doc are resolved or
  explicitly deferred (IMPL-0009's own decisions are already fixed in
  the Decisions section below)
- Approved by repository owner

---

### Phase 2: Introduce `Runner` struct (no behavior change)

Establish the Runner shape with no functional change to handlers yet.

#### Tasks

- [x] Define `cmd.Runner` struct per the DESIGN doc, e.g.:

  ```
  type Runner struct {
      Cfg    config.Config
      Out    io.Writer
      Err    io.Writer
      Logger *slog.Logger
      Now    func() time.Time
      Git    GitResolver
  }
  ```

- [x] Add `NewRunner(cfg *config.Config) *Runner` with defaults
      (`os.Stdout`, `os.Stderr`, `slog.TextHandler` at `LevelInfo`,
      `time.Now`, `realGit{}`). Signature deviates from the DESIGN
      sketch — takes `*config.Config` per `gocritic hugeParam` (Config
      is ~240B); semantically equivalent (Runner stores `*cfg` as a
      value copy).
- [x] Add `GitResolver` interface plus `realGit`/`staticGit`
      implementations in `cmd/git.go`. (Bundled into Phase 2 because
      `Runner.Git` requires the type to compile; Phase 6 still owns
      the conversion of `cmd/create.go:gitUserName` callers.)
- [x] Add a single root-level `runner *Runner` global initialized in
      `PersistentPreRunE` — this is a temporary scaffolding step; later
      phases convert handlers one at a time.
- [x] Confirm all existing tests pass with no changes. New tests added:
      `cmd/runner_test.go` (`TestNewRunner_Defaults`,
      `TestRunner_DirectConstruction`, `TestPackageRunner_AssignedFromNewRunner`)
      and `cmd/git_test.go` (`TestStaticGit_UserName`,
      `TestRealGit_UserName_Smoke`).

#### Success Criteria

- [x] `Runner` defined and importable
- [x] No handler converted yet — pure plumbing
- [x] `make ci` green

---

### Phase 3: Migrate handlers to Runner methods + output writers

Convert command handlers from package-level functions to `Runner` methods.
Per DESIGN-0004 §C, handlers write to `r.Out` / `r.Err` (NOT
`cmd.OutOrStdout()` — the task wording below predates the DESIGN and is
superseded).

#### Tasks

- [x] Convert `runCreate` → `(*Runner).Create` accepting context and
      args; output through `r.Out` (Phase 3e)
- [x] Convert `runUpdate`, `runList`, `runInit`, `runTemplateShow`,
      `runTemplateExport`, `runTemplateOverride`, `runWikiInit`,
      `runWikiUpdate`, `runConfig`, `runVersion` similarly (Phases 3a/3b/3c/3d/3f)
- [x] Replace `fmt.Printf` / `os.Stdout` sites with writes through
      `r.Out` (single residual at `cmd/root.go:79` is in the
      bootstrap path before the Runner exists — acceptable)
- [x] Replace `fmt.Fprintf(os.Stderr, ...)` sites with `r.Err` writes
      or `r.Logger.Debug` (Phase 4 work folded in)
- [x] Update tests to construct a Runner with `bytes.Buffer` writers
      instead of `os.Pipe` tricks — finished in the cleanup commit:
      `installListRunner`, `setupWikiTestDir`, `newTemplateTestRunner`,
      `BenchmarkCmdUpdate`, and `config_test.go` all assemble a Runner
      with `Out` pointed at a `bytes.Buffer` or `io.Discard`

#### Success Criteria

- [x] `grep -rn 'fmt\.Printf\|fmt\.Println\|os\.Stdout' cmd/*.go | grep -v _test.go`
      returns only `cmd/root.go:79` (bootstrap path)
- [x] No test uses `os.Pipe` to capture output — verified with
      `grep -n 'os.Pipe' cmd/*_test.go`
- [x] Tests can run `t.Parallel()` (where the underlying handler is
      side-effect-free) — every cmd test that does not touch the
      package-level `runner`/`appCfg`/flag globals now calls
      `t.Parallel()`: `TestStaticGit_UserName` (+ subtests),
      `TestRealGit_UserName_{Smoke,CtxCancel}`,
      `TestFilterByStatus`, `TestOutputTable/JSON/CSV`, the
      `TestBuildLogger_*` family, `TestRunner_resolveAuthor`
      (+ subtests), and `TestRunner_Create_Parallel`. The remaining
      cmd tests still go through `runUpdate`/`runCreate`/`rootCmd.Execute`
      and therefore touch the shared globals, so they stay serial
      until per-command opts structs land in a follow-up RFC.

---

### Phase 4: Introduce `log/slog` logger; eliminate `if verbose`

Replace verbose-guard blocks with structured logging. (Note: the
mechanical replacements landed alongside the Phase 3 conversions;
the `--log-level` / `--log-format` flag wiring remains.)

#### Tasks

- [x] In `Runner`, wire `Logger *slog.Logger` from the `--verbose` flag
      (verbose → debug level; default → info level). `NewRunner` still
      installs the safe default (TextHandler at LevelInfo, stderr); the
      flag-driven swap happens in `loadAndValidateConfig` via
      `buildLogger`, which then overwrites `r.Logger` before the global
      `runner` is published. This keeps `NewRunner` callable from tests
      with no flag plumbing.
- [x] Replace every `if verbose { fmt.Fprintf(os.Stderr, ...) }` block
      with `r.Logger.Debug(msg, "key", value)` — done as part of
      Phase 3 conversions
- [x] Internal packages remain quiet (no logger handle plumbed in —
      see DESIGN-0004 §D)
- [x] Decision §1 locked: `slog.TextHandler` default
- [x] Add `--log-level` flag (debug/info/warn/error) and `--log-format`
      flag (text/json) with the JSON handler swap. Resolution order:
      explicit `--log-level` wins, else `--verbose`→debug, else info.
      Invalid values surface a startup error rather than silently
      defaulting.

#### Success Criteria

- [x] `grep -rn 'if verbose' cmd/*.go | grep -v _test.go` returns no
      matches
- [x] `grep -rn '\bverbose\b' cmd/*.go | grep -v _test.go` returns
      only the `cmd/root.go` flag declaration and the buildLogger call
      site that consumes it
- [x] Tests can capture log output by configuring a buffer-backed
      handler — `TestBuildLogger_*` cases use a `bytes.Buffer` as the
      slog Writer and assert on emitted records (text and JSON)

---

### Phase 5: Inject time into `document.CreateOptions`

Eliminate the `internal/document/time.go` package global.

#### Tasks

- [x] Add `CreatedAt time.Time` to `document.CreateOptions`
- [x] In `document.Create`, use `opts.CreatedAt` (with zero-value fallback
      to `time.Now()`) and remove the call to `currentDate()` /
      `timeNow()`
- [x] Delete `internal/document/time.go` and the `timeNow` package
      variable
- [x] In `cmd/create.go`, populate `opts.CreatedAt = r.Now()` inside
      `(*Runner).Create`
- [x] Update `internal/document/create_test.go` to pass `CreatedAt`
      directly; remove `t.Cleanup` time-restore patterns; add
      `TestCreate_ZeroCreatedAtFallsBackToNow` to cover the
      zero-value path

#### Success Criteria

- [x] `grep -rn 'timeNow' internal/` returns no matches
- [x] Tests no longer mutate package globals to control time

---

### Phase 6: Inject git resolution; propagate `cmd.Context()`

#### Tasks

- [x] Define `type GitResolver interface { UserName(ctx context.Context) string }`
      in `cmd/git.go` (Phase 2)
- [x] Implement `realGit struct{}` that calls
      `exec.CommandContext(ctx, "git", "config", "user.name")` (Phase 2)
- [x] Add a test-friendly `staticGit{Name string}` implementation (Phase 2)
- [x] In `Runner`, hold `Git GitResolver` (Phase 2)
- [x] In `(*Runner).resolveAuthor`, call `r.Git.UserName(ctx)`
      instead of the package-level `gitUserName()` (Phase 3e)
- [x] Delete `cmd/create.go:gitUserName` (Phase 3e)
- [x] Author-resolution unit test that passes `staticGit{Name: "Test User"}`.
      `TestRunner_resolveAuthor` (cmd/runner_test.go) is a five-row
      table covering: flag wins over everything; config default wins
      over git; git wins when both are empty; `from_git=false` skips
      git; git returning empty falls through to "Unknown".

#### Success Criteria

- [x] `gitUserName` is gone
- [x] Author resolution is fully unit-testable (Runner.resolveAuthor
      takes ctx + flagAuthor and reads only r.Cfg/r.Git)
- [x] `Ctrl+C` during `docz create` cancels the git lookup.
      `TestRealGit_UserName_CtxCancel` (cmd/git_test.go) passes an
      already-cancelled context and asserts the call returns "" within
      2s — a future regression that drops the ctx would time out
      instead of hanging the suite.

---

### Phase 7: Add `repoRoot` to `config.Load`

Eliminate `os.Chdir` in tests.

#### Tasks

- [x] Change `config.Load(configFile string) (Config, error)` to
      `config.Load(configFile, repoRoot string) (Config, error)`
- [x] `repoRoot` is the directory to scan for `.docz.yaml` (empty string
      = current working directory for back-compat default)
- [x] In `loadAndValidateConfig` (cmd/root.go), pass `os.Getwd()`
      explicitly
- [x] Update tests in `internal/config/config_test.go` and
      `internal/config/parity_baseline_test.go` to pass `t.TempDir()`
      directly via the new repoRoot param; remove every `os.Chdir` +
      `t.Cleanup(os.Chdir)` pattern in these files
- [x] Remove `os.Chdir` from `cmd/init_test.go`, `cmd/wiki_test.go`,
      and `cmd/template_test.go` — done by adding `Runner.RepoRoot`
      and an `inRepo(name)` helper for cwd-relative path resolution,
      plus a `--repo-root` Cobra flag with precedence
      `--repo-root > dir(--config) > os.Getwd()` so `root_test.go` and
      `inv0003_test.go` can drive `rootCmd.Execute()` without
      `os.Chdir`
- [x] Verify tests can run `t.Parallel()` now — `internal/*` already
      do (134 sites); the side-effect-free cmd tests
      (`git_test.go`, the `outputTable/JSON/CSV` table tests,
      `TestFilterByStatus`, the `TestBuildLogger_*` family,
      `TestRunner_resolveAuthor`, `TestRunner_Create_Parallel`) now
      do as well. The remaining cmd tests that exercise package-level
      `runner`/`appCfg`/flag globals stay serial pending the
      per-command opts struct refactor.

#### Success Criteria

- `grep -rn 'os\.Chdir' .` returns no matches in test code
- Tests run with `t.Parallel()` and pass
- `make test` wall-clock time decreases noticeably

---

### Phase 8: Introduce `DocType` registry

Replace the scattered type definitions with a single registration list.

#### Tasks

- [x] Define `internal/config/doctype.go` with the registry struct.
      Named `DocTypeDef` (not `DocType`) to leave `DocType` free for the
      typed-string in Phase 10 (DESIGN-0004 Open Question 2); `DefaultConfig`
      is a `func() TypeConfig` constructor so each lookup yields a fresh
      `Statuses` slice (DESIGN-0004 §E):

  ```go
  type DocTypeDef struct {
      Name          string
      Aliases       []string
      DefaultConfig func() TypeConfig
      NavTitle      string
      PluralLabel   string
      TemplateName  string
  }
  ```

- [x] Define `var allDocTypes = []DocTypeDef{ ... }` listing all 6 types
      with their full metadata in one place
- [x] Add helpers: `AllDocTypes() []DocTypeDef` (returns `slices.Clone`),
      `LookupDocType(name string) (DocTypeDef, bool)` (case-insensitive,
      whitespace-trimmed, canonical + alias), `DocTypeNames() []string`
      (registry-declaration order — `ValidateType`'s error message and
      the existing `TestValidTypes` depend on this ordering)
- [x] Derive `DefaultConfig().Types` from `allDocTypes` via `defaultTypesMap()`
- [x] Derive `ValidTypes()` from `allDocTypes` (now delegates to `DocTypeNames()`)
- [x] Derive `DefaultNavTitles()` from `allDocTypes` via `defaultNavTitlesMap()`
- [x] Derive `typeAliases` from `allDocTypes` via `defaultTypeAliases()`
- [x] Derive `TypesHelp()` text from `allDocTypes` — done by adding
      a `HelpDescription` field to `DocTypeDef` and rewriting
      `TypesHelp` as a registry walk that auto-appends `(alias: …)`
      from each entry's `Aliases`. The byte-for-byte output is
      unchanged.
      `TestDocTypeRegistry_TypesHelpDerivedFromRegistry` replaces the
      previous static-string guard with a registry-derived assertion
      that every name, every `HelpDescription`, and every alias
      reaches the rendered help.
- [x] Add a test: every registered `DocTypeDef` has a corresponding
      `internal/template/templates/<TemplateName>.md` embedded file
      (`TestDocTypeRegistry_AllHaveEmbeddedTemplate`)
- [x] Add a test: every registered `DocTypeDef` has a corresponding
      `internal/template/templates/index_<TemplateName>.md`
      (`TestDocTypeRegistry_AllHaveEmbeddedIndexHeader`)
- [x] Add the rest of the DESIGN-0004 §E consistency invariant tests:
      no duplicate canonical names, no alias collides with a canonical
      name, `DefaultConfig()` validates, every entry's `DefaultConfig()`
      ships non-empty `Statuses`, `DefaultConfig()` hands back a fresh
      `Statuses` backing array per call, `LookupDocType` resolves
      canonical/aliases case-insensitively, and the derived
      `DefaultConfig()` matches the registry literal field-for-field

#### Success Criteria

- Adding a new doc type requires editing exactly one location:
  `allDocTypes`, plus creating the two template files
- A test catches a registered type missing its embedded templates
- All existing behavior unchanged (golden files green)

---

### Phase 9: Drive iteration from `EnabledTypes()`

Now that the registry is in place, `EnabledTypes()` should iterate the
registry rather than the hardcoded `ValidTypes()` slice.

#### Tasks

- [x] Update `Config.EnabledTypes()` (from IMPL-0006) to iterate
      `allDocTypes` (via `DocTypeNames`) and filter by
      `Config.Types[name].Enabled`. The old sort step is dropped —
      registry-declaration order is now the canonical order.
- [x] Audit all remaining `ValidTypes()` call sites and replace with
      `EnabledTypes()` or `DocTypeNames()` as appropriate:
      - `cmd/list.go` (`docz list`) → `r.Cfg.EnabledTypes()` so disabled
        types are skipped instead of attempting to scan a directory
        the user removed
      - `cmd/template.go` (three help strings) → `config.DocTypeNames()`
      - `config.ValidateType` error message → `config.DocTypeNames()`
      - `config.Validate` membership set → `config.DocTypeNames()`
- [x] Delete `ValidTypes()` — no callers remain
- [x] Update `TestValidTypes` → `TestDocTypeNames`; update
      `TestEnabledTypes` and `TestDefaultConfig` to assert registry order

#### Success Criteria

- `grep -rn 'ValidTypes' .` returns only one match: a historical comment
  in `doctype.go` documenting the consolidation
- All iteration goes through the registry

---

### Phase 10: Introduce typed `DocType` and `Status` strings

Add typed-string definitions for compile-time signal at API boundaries.

#### Tasks

- [x] Define `type DocType string` (typed wrapper, not the struct)
      in `internal/config/doctype.go`
- [x] Define `type Status string` alongside `DocType`
- [x] No registry rename needed — Phase 8 already named the struct
      `DocTypeDef`, so `DocType` is free for the typed-string wrapper
      (DESIGN-0004 Decision §2 resolves Open Question 2)
- [x] Update `document.CreateOptions.Type` to `config.DocType`
- [x] Update `template.Data.Type` (renamed from `TemplateData` in
      IMPL-0008) to `config.DocType` and `template.Data.Status` to
      `config.Status`
- [x] Update `template.EmbeddedDocumentTemplate(docType config.DocType)`
      — `template.Resolve` keeps `string` (internal resolution path,
      not in §F's enforcement surface) and casts once at the embed
      call site
- [x] Update `document.Frontmatter.Status` to `config.Status`. No custom
      YAML marshaler needed — `go.yaml.in/yaml/v3` round-trips typed
      strings whose underlying kind is `string` transparently. DESIGN-0004
      §F revises Decision §3.
- [x] Verify YAML round-trip back-compat with two tests in
      `document_test.go`: `TestFrontmatter_TypedStatus_YAMLRoundTrip`
      pins the bare-scalar emit and field-level round trip;
      `TestFrontmatter_TypedStatus_LegacyYAMLParses` pins parsing of a
      pre-typed-string YAML fixture

#### Success Criteria

- [x] `DocType` and `Status` typed wrappers exist
- [x] Function signatures across `internal/` use the typed forms
- [x] `.docz.yaml` files written by any prior docz version still parse
      (no top-level field changed type — only Frontmatter.Status and
      template.Data fields, neither of which appear in `.docz.yaml`)

---

### Phase 11: Verify and ship

#### Tasks

- [x] Full `make ci` green
- [x] Manual smoke test across all CLI commands —
      `version`, `--help`, `init`, `create rfc/adr/inv` (canonical
      and alias paths), `list` (no filter, type filter, `--status`),
      `update`, `template show`, `config`, `wiki update` all behave
      as expected against a `/tmp/docz-smoke` working directory
- [x] Verify all tests can run with `t.Parallel()`. Every top-level
      `Test*` in `internal/*` plus their table-driven subtests now
      call `t.Parallel()` (134 sites). Three iterations of
      `go test -race -shuffle=on -count=3 ./...` are green. cmd/
      tests stay serial because they share package-level `runner` /
      `appCfg` / flag globals, but the pipe-discarding pattern
      (`_, w, _ := os.Pipe()`) that previously flaked under shuffle
      is fixed by capturing the read end and deferring its close —
      so even without parallelism the cmd/ suite is now
      order-independent
- [x] Update INV-0002 status — flipped to `Concluded`; Wave 4 marked
      done (PRs #44/#45/#46) and Wave 5 marked done (DESIGN-0004 +
      IMPL-0009 Phases 2-10) with item-by-item references
- [x] Update the architecture section of CLAUDE.md to reflect the new
      structure (Runner pattern, DocType registry, typed strings,
      logger flag wiring all noted)
- [x] Document the DocType registration pattern in CONTRIBUTING.md
      and DEVELOPMENT.md (single-file edit + two templates,
      pointers to the Phase 8 consistency tests)
- [x] Open final PR(s) with `dont-release` label. PR #48 (this PR)
      stacks on PR #47 (DESIGN-0004) per Decisions §6. The
      `--log-level` and `--log-format` flags are new user-visible
      surface but ship behind their existing defaults (text + info),
      so the runtime behavior matches the prior release and the PR
      keeps the `dont-release` label. When PR #47 merges, GitHub will
      auto-rebase PR #48's base onto main.

#### Success Criteria

- [x] `make ci` green
- [x] A new contributor can add a doc type by editing one file plus two
      template files (CONTRIBUTING.md "Adding a New Built-In Document
      Type" and DEVELOPMENT.md walkthrough both pin this)
- [x] Tests run in parallel — all internal/* tests, top-level and
      subtests, call `t.Parallel()`. `go test -race -shuffle=on
      -count=3 ./...` is green. cmd/ tests stay serial intentionally
      until the package-level globals are removed (out of scope here)
- [x] No `cmd/` package-level globals remain except the threaded
      `*Runner` and the bound Cobra flag values
      (`cfgFile`, `docsDir`, `verbose`, `logLevel`, `logFormat`, and
      per-command flag vars like `createStatus`); the bound globals
      are CLI-flag plumbing rather than runtime state

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `docs/design/00NN-runner-pattern-and-doctype-registry.md` | Create | Prerequisite DESIGN doc |
| `cmd/runner.go` | Create | `Runner` struct, `NewRunner`, helpers |
| `cmd/git.go` | Create | `GitResolver` interface, `realGit`, tests use fakes |
| `cmd/root.go` | Modify | Wire `Runner` into `PersistentPreRunE` |
| `cmd/*.go` | Modify | Convert handlers to `(*Runner).Foo` methods |
| `cmd/*_test.go` | Modify | Use `cmd.SetOut(&buf)`; pass test Runner |
| `internal/document/time.go` | Delete | `timeNow` global no longer needed |
| `internal/document/create.go` | Modify | Use `opts.CreatedAt` |
| `internal/config/config.go` | Modify | `Load(configFile, repoRoot string)` |
| `internal/config/doctype.go` | Create | `DocType` registry struct + `allDocTypes` table |
| `internal/template/template.go` | Modify | Use typed `DocType` |
| All test files | Modify | Remove `os.Chdir`; remove `os.Pipe` tricks |

## Testing Plan

- [x] Every Runner method has a focused unit test using a constructed
      `Runner` with `bytes.Buffer` writers (cmd/runner_test.go:
      `TestNewRunner_Defaults`, `TestRunner_DirectConstruction`,
      `TestRunner_resolveAuthor`, `TestRunner_Create_Parallel`,
      `TestPackageRunner_AssignedFromNewRunner`, plus the
      `TestBuildLogger_*` family for the slog wiring)
- [x] DocType registry consistency tests
      (`internal/config/doctype_test.go`):
  - Every `DocType.Name` matches an embedded template
    (`TestDocTypeRegistry_AllHaveEmbeddedTemplate`)
  - Every `DocType.Name` matches an embedded index header
    (`TestDocTypeRegistry_AllHaveEmbeddedIndexHeader`)
  - No alias collides with a canonical name
    (`TestDocTypeRegistry_NoAliasCollidesWithCanonical`)
  - `DefaultConfig().Types` is fully derivable from `allDocTypes`
    (`TestDocTypeRegistry_DerivedDefaultConfigMatchesHardcoded`)
- [x] `t.Parallel()` regression test:
      `TestRunner_Create_Parallel` (cmd/runner_test.go) fires 10
      concurrent `(*Runner).Create` calls against distinct temp dirs,
      asserts all succeed, and spot-checks that each produced
      `docs/rfc/0001-concurrent-title.md` in its own dir — proving
      the handler itself is parallel-safe; only the cmd/ package
      globals block `t.Parallel()` at the wrapper layer.
- [x] Slog handler test: `TestBuildLogger_{VerboseSelectsDebug,
      DefaultDropsDebug, LogLevelOverridesVerbose, JSONFormat,
      InvalidFlagsError}` (Phase 4) capture log records into a
      bytes.Buffer and assert on emitted records.
- [x] Git resolver test: `TestRealGit_UserName_CtxCancel` (above).
- [x] YAML back-compat test: `TestFrontmatter_TypedStatus_YAMLRoundTrip`
      and `TestFrontmatter_TypedStatus_LegacyYAMLParses` (Phase 10)
      cover both the new on-disk emit and parsing of legacy YAML.
      The pre-existing `TestParseFrontmatter` table also covers the
      `.docz.yaml` Config-level round trip indirectly through the
      golden-driven `internal/config` tests.

## Decisions

Resolved during INV-0002 planning review.

1. **`slog` default handler:** `slog.TextHandler` for human-interactive
   use. Add `--log-format=json` as opt-in for users piping to log
   aggregators.
2. **`DocType` naming collision:** registry struct is `config.DocTypeDef`;
   typed-string wrapper is `config.DocType`. Locked in via the DESIGN
   doc in Phase 1.
3. **`Status` typing at the YAML boundary:** typed `Status string` at
   the in-memory boundary; plain string in YAML via a custom unmarshaler
   that wraps. Verify round-trip back-compat with a fixture test.
4. **`Runner` lifetime:** per-process singleton, matching the current
   `appCfg` model. Revisit per-command construction if docz grows a
   daemon mode.
5. **Library evaluation (`fang`, Bubble Tea v2, `localstack/lstk`):**
   investigate during the DESIGN doc in Phase 1 (see the Phase 1 task
   list for the explicit comparison points). Note: at last check
   `fang` depended on Bubble Tea v1; consider using v2 components
   directly with Cobra. `localstack/lstk` is a useful reference for
   Runner pattern and command organization.
6. **Wave 5 PR split:** ship as a **single PR** (all phases 2–11
   together). Large but cohesive; full CI provides a clean revert
   target. Phase 1 (the DESIGN doc) ships separately as a prerequisite.
7. **`.docz.yaml` back-compat verification:** collect 3–5 real-world
   fixtures (this repo's `.docz.yaml`, plus 2–3 synthetic edge cases)
   and add them as golden inputs to the parse test.
8. **External `cmd/` importers:** confirm none exist; document the
   assumption in the DESIGN doc. No build-tag fences or `internal/cmd`
   move needed.
9. **`slog.Attr` vs. shorthand:** shorthand `slog.Info(msg, "key", value)`
   by default. Switch specific call sites to `slog.Attr` only if
   profiling identifies a hot logging path (unlikely for a CLI).
10. **Phase 8 (DocType registry) rollback strategy:** revert the
    single Wave 5 PR if a compatibility issue surfaces. Branch
    protection plus the consistency tests should catch most issues
    pre-merge.

## Dependencies

- **Prerequisite:** DESIGN doc (Phase 1)
- Builds on IMPL-0005, IMPL-0006, IMPL-0007, IMPL-0008 — each makes the
  surface area smaller and the refactor easier
- This is the final wave; nothing depends on it landing

## References

- INV-0002 — Wave 5, findings F1–F6, F14–F16
- IMPL-0008 — provides the slim `cmd/` and `internal/document` shape
  this refactor builds on
- IMPL-0006 — provides `EnabledTypes()` and `ValidateType()` helpers
- Effective Go — package design, interface design
- `log/slog` package docs (Go 1.21+) — structured logging API
- Cobra `Command.OutOrStdout` documentation
