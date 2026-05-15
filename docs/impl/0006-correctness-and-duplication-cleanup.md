---
id: IMPL-0006
title: "Correctness and Duplication Cleanup"
status: Draft
author: Donald Gifford
created: 2026-05-15
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0006: Correctness and Duplication Cleanup

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-05-15

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Derive .docz.yaml write from DefaultConfig()](#phase-1-derive-doczyaml-write-from-defaultconfig)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Audit and fix setDefaults coverage](#phase-2-audit-and-fix-setdefaults-coverage)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Propagate config validation error at startup](#phase-3-propagate-config-validation-error-at-startup)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Distinguish missing vs. unparseable config files](#phase-4-distinguish-missing-vs-unparseable-config-files)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Wrap bare return err sites with context](#phase-5-wrap-bare-return-err-sites-with-context)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Extract ValidateType helper and EnabledTypes() method](#phase-6-extract-validatetype-helper-and-enabledtypes-method)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: Add TypeConfig.PluralLabel; remove "adr" magic-string](#phase-7-add-typeconfigplurallabel-remove-adr-magic-string)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
  - [Phase 8: Frontmatter CRLF tolerance](#phase-8-frontmatter-crlf-tolerance)
    - [Tasks](#tasks-7)
    - [Success Criteria](#success-criteria-7)
  - [Phase 9: Verify and ship](#phase-9-verify-and-ship)
    - [Tasks](#tasks-8)
    - [Success Criteria](#success-criteria-8)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Decisions](#decisions)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

## Objective

Eliminate the active bug class behind INV-0002: three-way duplication of
defaults (which already produced PRs #30 and #31), silent error swallowing,
and the four copy-paste sites of "unknown document type" error handling.
After this wave, adding a new field to `Config` should require exactly one
edit and `.docz.yaml` validation errors should be visible.

**Implements:** INV-0002 (Wave 2 — Correctness and duplication)

## Scope

### In Scope

- Derive `cmd/init.go:writeDefaultConfig` from `config.DefaultConfig()` (F12)
- Audit and fix `setDefaults` field coverage (F49)
- Propagate config validation errors via `PersistentPreRunE` (F7)
- Distinguish missing vs. unparseable config files in `mergeConfigFile` (F8)
- Wrap all bare `return err` sites with context (F10)
- Extract `config.ValidateType` helper to collapse 4 duplicated error sites
  (F38, F39)
- Add `Config.EnabledTypes()` method to collapse 3 guard-block sites (F40)
- Add `TypeConfig.PluralLabel` field to remove the `"adr"` magic-string
  special case (F34)
- Frontmatter CRLF tolerance (F48)

### Out of Scope

- Moving `writeMkDocsYAML` or `updateToCs` (IMPL-0008)
- Introducing typed `DocType` or `Status` (IMPL-0009)
- Restructuring command output / logging (IMPL-0009)
- Performance changes around double-reads (IMPL-0007)

## Implementation Phases

---

### Phase 1: Derive `.docz.yaml` write from `DefaultConfig()`

Eliminate the 100-line hardcoded YAML in `writeDefaultConfig` by marshalling
`config.DefaultConfig()` directly. This removes the three-way duplication
at its root.

#### Tasks

- [ ] Audit `cmd/init.go:60-183` against `config.DefaultConfig()` to confirm
      they produce identical output today (baseline parity)
- [ ] Replace the literal YAML string with `yaml.Marshal(config.DefaultConfig())`
      using `go.yaml.in/yaml/v3` (same lib used elsewhere)
- [ ] Verify the marshalled output round-trips through `config.Load()` back
      to a `Config` equal to the original `DefaultConfig()`
- [ ] Remove the `//nolint:funlen` directive on `writeDefaultConfig`
- [ ] If preserving comments/header is desirable, prepend a fixed header
      block (see Decisions §1)

#### Success Criteria

- `writeDefaultConfig` is under 30 lines
- A round-trip test (`marshal → unmarshal → DeepEqual`) passes
- Running `docz init` in a fresh directory produces a `.docz.yaml` that
  parses cleanly under `docz config`

---

### Phase 2: Audit and fix `setDefaults` coverage

`setDefaults` (config.go:291-319) was hand-written to mirror `DefaultConfig`
but has drifted: it is missing `MarkdownExtensions`, `DocsDir`, `RepoURL`,
`SiteURL`, `Theme` (added later). Decide between reflection-driven or
removal.

#### Tasks

- [ ] Diff `setDefaults` against `DefaultConfig` and document every missing
      field
- [ ] **Option A** (reflection): rewrite `setDefaults` to walk
      `DefaultConfig()` via reflection and call `v.SetDefault(path, value)`
      for every leaf field
- [ ] **Option B** (removal): delete `setDefaults` entirely; rely on Viper
      unmarshalling onto a pre-populated `Config` struct (see Decisions §2)
- [ ] Add a test that asserts `setDefaults` parity for every field in
      `Config` via reflection (regardless of which option is chosen)

#### Success Criteria

- A user `.docz.yaml` that sets only `wiki.repo_url:` correctly merges over
  defaults — verified by a test
- Adding a new field to `Config` does not require a corresponding edit to
  `setDefaults` (or `setDefaults` no longer exists)

---

### Phase 3: Propagate config validation error at startup

Currently `cmd/root.go:initConfig` prints the validation error to stderr and
continues with the broken config. Convert to a hard failure.

#### Tasks

- [ ] Move config loading + validation out of `cobra.OnInitialize` into a
      `PersistentPreRunE` on `rootCmd`
- [ ] Have the `PreRunE` return the validation error so Cobra exits non-zero
- [ ] Keep printing warnings (non-fatal) before returning the error
- [ ] Remove the `appCfg = cfg` assignment on the error path
- [ ] Add a test: a `.docz.yaml` with `statuses: []` causes `docz create rfc "x"`
      to exit non-zero with the validation message

#### Success Criteria

- `docz` with a broken `.docz.yaml` exits 1 immediately with a clear message
- `docz --help` still works even with a broken `.docz.yaml` (help should not
  require config) — see Decisions §3

---

### Phase 4: Distinguish missing vs. unparseable config files

`mergeConfigFile` currently swallows both "file does not exist" and "YAML
parse error" silently. Surface the parse error.

#### Tasks

- [ ] In `internal/config/config.go:262-272`, change the second `return nil`
      (after `ReadInConfig`) to return a wrapped error
- [ ] Update the first `return nil` to check specifically for
      `errors.Is(err, fs.ErrNotExist)`; surface other stat errors (permission
      denied, etc.)
- [ ] Update both call sites of `mergeConfigFile` to handle the error
- [ ] Add tests for: (a) missing file → returns defaults silently, (b)
      malformed YAML → returns error with path in message, (c) permission
      denied → returns error

#### Success Criteria

- A `.docz.yaml` containing `not: valid: yaml: :` causes `docz` to exit 1
  with a clear "parse error in .docz.yaml: ..." message
- Tests for all three cases above pass

---

### Phase 5: Wrap bare `return err` sites with context

Walk every flagged site and add `fmt.Errorf("doing X: %w", err)` wrapping.

#### Tasks

- [ ] Wrap `cmd/update.go:54` — `runUpdate` loop
- [ ] Wrap `cmd/create.go:89` — `document.Create` call
- [ ] Wrap `cmd/init.go:51` — `writeIndexReadme` call
- [ ] Wrap `cmd/wiki.go:103` — `writeMkDocsYAML` call
- [ ] Wrap `cmd/wiki.go:109` — `ensureDocsIndex` call
- [ ] Wrap `cmd/wiki.go:154-156` — `ReadMkDocs`
- [ ] Wrap `cmd/wiki.go:178-180` — `WriteMkDocs`
- [ ] Wrap `cmd/wiki.go:197-199` — dry-run `ReadMkDocs`
- [ ] Wrap `internal/wiki/wiki.go:58` — recursive `scanDir`
- [ ] Audit any sites missed by INV-0002 with `grep -rn 'return err$' cmd/ internal/`

#### Success Criteria

- `grep -rn 'return err$' cmd/ internal/` returns no results (or each
  remaining instance has an explicit code comment justifying it)
- Every error message includes enough context to identify the failing
  operation

---

### Phase 6: Extract `ValidateType` helper and `EnabledTypes()` method

Collapse the four "unknown document type" sites and three enabled-type
guard blocks into single helpers.

#### Tasks

- [ ] Add `config.ValidateType(name string) (canonical string, err error)`
      that internally: lowercases, resolves alias, looks up in valid types,
      returns canonical name or a wrapped error
- [ ] Define `var ErrUnknownType = errors.New("unknown document type")`
      sentinel (or a typed error) so callers can `errors.Is` on it
- [ ] Replace the duplicated block at `cmd/create.go:50-57` with a single
      `ValidateType` call
- [ ] Replace the duplicated block at `cmd/list.go:54-58`
- [ ] Replace the duplicated block at `cmd/update.go:37-42`
- [ ] Replace the three sites in `cmd/template.go:71,86,110` + `validateType()`
      at line 139 (delete the local helper)
- [ ] Add `func (c *Config) EnabledTypes() []string` that returns sorted
      enabled canonical type names (intersection of `ValidTypes()` and
      `appCfg.Types` with `Enabled: true`)
- [ ] Replace iteration at `cmd/init.go:36-43` with `appCfg.EnabledTypes()`
- [ ] Replace iteration at `cmd/update.go:35-52` with `appCfg.EnabledTypes()`
- [ ] Replace iteration at `cmd/wiki.go:307-321` with `appCfg.EnabledTypes()`

#### Success Criteria

- "unknown document type" string appears in exactly one source location
- `EnabledTypes()` returns sorted, deterministic output (verified by test)
- `make test` passes
- Behavior unchanged: existing CLI invocations produce identical output

---

### Phase 7: Add `TypeConfig.PluralLabel`; remove `"adr"` magic-string

Replace the special-case at `cmd/update.go:85-87` with config-driven
pluralization.

#### Tasks

- [ ] Add `PluralLabel string` field to `TypeConfig` with `yaml:"plural_label,omitempty"`
- [ ] Default values in `DefaultConfig()`:
      `rfc: "RFCs"`, `adr: "ADRs"`, `design: "Designs"`,
      `impl: "Implementation Plans"`, `plan: "Plans"`,
      `investigation: "Investigations"`
- [ ] Replace `cmd/update.go:84-87` with `heading := "All " + tc.PluralLabel`
- [ ] Audit `cmd/wiki.go:312-315` — the nav-title fallback that does
      `strings.ToUpper(typeName)` — and use `PluralLabel` here too if it
      conceptually maps (see Decisions §4)
- [ ] Re-run golden file regeneration; verify only the `design` index
      heading changes (`"DESIGNs"` → `"Design"` — see Decisions §5)
- [ ] Update the embedded index header templates if they reference the
      old heading

#### Success Criteria

- No magic strings remain in `cmd/update.go`
- README index headings come from config, not string manipulation
- Existing repos with `.docz.yaml` files continue to work (back-compat:
  `PluralLabel` is optional and falls back to the old behavior if absent)

---

### Phase 8: Frontmatter CRLF tolerance

`document.ParseFrontmatter` currently rejects `---\r\n` line endings.

#### Tasks

- [ ] In `internal/document/document.go:30-39`, normalize `\r\n` to `\n` in
      the prefix-check region, or relax `rest[0] != '\n'` to accept
      `'\r'` followed by `'\n'`
- [ ] Add a table-driven test covering `\n`, `\r\n`, and mixed line endings

#### Success Criteria

- A docs file authored on Windows (CRLF) is parsed successfully
- Test covers all three line-ending variants

---

### Phase 9: Verify and ship

#### Tasks

- [ ] Run `make ci`
- [ ] Smoke test: bootstrap a fresh repo, run `docz init`, verify
      `.docz.yaml` matches a `DefaultConfig` round-trip
- [ ] Smoke test: corrupt `.docz.yaml` and confirm clear error message
- [ ] Open PR with `dont-release` label
- [ ] Update INV-0002 status to "In Progress" if not already

#### Success Criteria

- `make ci` passes
- Hand-crafted "broken config" gives a clear error
- Hand-crafted "config missing a field added later" still works because
  defaults merge over

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `cmd/init.go` | Modify | Derive `.docz.yaml` from `DefaultConfig()` |
| `cmd/root.go` | Modify | Move config load/validate to `PersistentPreRunE` |
| `cmd/create.go` | Modify | Use `ValidateType`; wrap errors |
| `cmd/list.go` | Modify | Use `ValidateType` |
| `cmd/update.go` | Modify | Use `ValidateType`, `EnabledTypes`, `PluralLabel`; wrap errors |
| `cmd/template.go` | Modify | Use `ValidateType`; delete local `validateType` |
| `cmd/wiki.go` | Modify | Use `EnabledTypes`; wrap errors |
| `internal/config/config.go` | Modify | Add `ValidateType`, `EnabledTypes`, `PluralLabel`, `ErrUnknownType`; reflective or simplified `setDefaults`; fix `mergeConfigFile` |
| `internal/document/document.go` | Modify | CRLF tolerance |
| `internal/wiki/wiki.go` | Modify | Wrap `scanDir` errors |

## Testing Plan

- [ ] Round-trip test: `DefaultConfig() → yaml.Marshal → yaml.Unmarshal →
      deep-equal`
- [ ] Reflective test: every leaf field on `Config` is covered by
      `setDefaults` (or `setDefaults` no longer exists)
- [ ] Error-surfacing tests: missing config, malformed config, permission-denied
      config
- [ ] Validation-error tests: empty `docs_dir`, type with empty `statuses` —
      both should cause non-zero exit
- [ ] `ValidateType` table-driven test: canonical names, aliases, unknown
      names, empty string, uppercase
- [ ] `EnabledTypes` test: order is sorted, disabled types excluded
- [ ] `PluralLabel` test: golden-file regeneration only changes the headings
- [ ] CRLF frontmatter test

## Decisions

Resolved during INV-0002 planning review.

1. **`.docz.yaml` generation:** use an embedded `.docz.yaml.tmpl`
   (`//go:embed` + `text/template`) with a header comment block at the
   top and structured per-section comments throughout. `DefaultConfig()`
   remains the data source supplied to the template.
2. **`setDefaults` fate:** try removal first (half-day timebox). Verify
   with a round-trip test that Viper's `MergeConfigMap` handles nested
   maps correctly without `SetDefault`-registered keys. If the test
   fails, fall back to reflection-driven `setDefaults`. Either way,
   ship a reflection-based parity test so the function (if present)
   can't drift again.
3. **`docz --help` with a broken config:** skip validation when
   `--help` / `-h` is present in args, or when no subcommand is given.
   Detection happens in `PersistentPreRunE` before the validation call.
4. **`PluralLabel` + `NavTitles` unification:** `PluralLabel` becomes
   the single source. `WikiConfig.NavTitles` is deprecated and wins
   over `PluralLabel` when present (back-compat for one version);
   remove `NavTitles` in a future release with a migration note.
5. **`design` plural label:** `"Design"` (no plural). Matches the
   existing `NavTitles["design"]` value and reads naturally as a
   section heading.
6. **`ErrUnknownType` shape:** sentinel error
   (`var ErrUnknownType = errors.New("unknown document type")`).
   Defer typed-error / "did you mean…?" enhancements until user
   feedback requests them.

## Dependencies

- Builds on IMPL-0005 (assumes idiom modernization has landed; we use
  `errors.Is(err, fs.ErrNotExist)` and similar in this wave)
- Must merge before IMPL-0008 (which assumes `EnabledTypes` and
  `ValidateType` are available)

## References

- INV-0002 — Wave 2, findings F7–F12, F34, F38–F40, F48, F49
- PR #30 — `markdown_extensions` defaults-drift case study
- PR #31 — disabled-types defaults-drift case study
- Viper `MergeConfigMap` semantics — relevant to Decisions §2
