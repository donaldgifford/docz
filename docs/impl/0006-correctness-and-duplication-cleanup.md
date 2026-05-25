---
id: IMPL-0006
title: "Correctness and Duplication Cleanup"
status: In Progress
author: Donald Gifford
created: 2026-05-15
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0006: Correctness and Duplication Cleanup

**Status:** In Progress
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
  - [Phase 5: Honor user-listed types only (INV-0003 fix)](#phase-5-honor-user-listed-types-only-inv-0003-fix)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Wrap bare return err sites with context](#phase-6-wrap-bare-return-err-sites-with-context)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: Extract ValidateType helper and EnabledTypes() method](#phase-7-extract-validatetype-helper-and-enabledtypes-method)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
  - [Phase 8: Add TypeConfig.PluralLabel; remove "adr" magic-string](#phase-8-add-typeconfigplurallabel-remove-adr-magic-string)
    - [Tasks](#tasks-7)
    - [Success Criteria](#success-criteria-7)
  - [Phase 9: Frontmatter CRLF tolerance](#phase-9-frontmatter-crlf-tolerance)
    - [Tasks](#tasks-8)
    - [Success Criteria](#success-criteria-8)
  - [Phase 10: Verify and ship](#phase-10-verify-and-ship)
    - [Tasks](#tasks-9)
    - [Success Criteria](#success-criteria-9)
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

Eliminate the 100-line hardcoded YAML in `writeDefaultConfig` by rendering
an embedded `.docz.yaml.tmpl` template with `config.DefaultConfig()` as
the template data. This removes the three-way duplication at its root
and lets us keep human-readable comments and a header block in the
generated file.

The `//nolint:funlen` directive on `writeDefaultConfig` was already
removed as part of IMPL-0005 (per its Decisions §6), so it is not on
this phase's task list.

#### Tasks

- [x] Audit `cmd/init.go:writeDefaultConfig` against
      `config.DefaultConfig()` to confirm they produce semantically
      identical output today (baseline parity) — confirmed drift:
      hardcoded YAML omitted `wiki.nav_titles`, exactly the F12 bug
- [x] Add `internal/template/templates/docz_yaml.tmpl` containing the
      `.docz.yaml` template: a header comment block at the top, then
      `text/template` directives that iterate `.Types`, `.Index`,
      `.Author`, `.Wiki`, `.ToC` from the template data with structured
      per-section comments
- [x] `//go:embed` the new template alongside the existing embedded
      doc templates in `internal/template/embed.go`; expose a
      `EmbeddedDoczYAML() (string, error)` helper
- [x] Replace the literal YAML string in `cmd/init.go:writeDefaultConfig`
      with a `text/template` render of `EmbeddedDoczYAML()` passed
      `config.DefaultConfig()`
- [x] Verify the rendered output round-trips through `config.Load()`
      back to a `Config` deep-equal to the original `DefaultConfig()`
      (guarded by `TestDoczYAMLTemplate_RoundTripsToDefaultConfig` in
      `internal/config/parity_baseline_test.go`)

#### Success Criteria

- `writeDefaultConfig` is under 30 lines
- A round-trip test (`render → parse → DeepEqual(DefaultConfig())`) passes
- Running `docz init` in a fresh directory produces a `.docz.yaml` that
  parses cleanly under `docz config`
- The generated `.docz.yaml` retains its header comment block and
  per-section comments (regression guard against the "marshal loses
  comments" failure mode)

---

### Phase 2: Audit and fix `setDefaults` coverage

`setDefaults` (config.go:291-319) was hand-written to mirror `DefaultConfig`
but has drifted: it is missing `MarkdownExtensions`, `DocsDir`, `RepoURL`,
`SiteURL`, `Theme` (added later). Decide between reflection-driven or
removal.

#### Tasks

- [x] Diff `setDefaults` against `DefaultConfig` and document every missing
      field — confirmed drift: `MarkdownExtensions`, `Wiki.DocsDir`,
      `RepoURL`, `SiteURL`, `Theme` missing; `toc.enabled` was wired to
      `cfg.Wiki.AutoUpdate` (copy-paste bug)
- [x] **Option B** (removal): delete `setDefaults` entirely; rely on Viper
      unmarshalling onto a pre-populated `Config` struct (see Decisions §2).
      Verified by full test suite passing without the function, plus three
      new parity tests
- [x] Add a parity test that exercises the removal: `Load("")` with no
      config files must deep-equal `DefaultConfig()`
      (`TestLoad_DefaultsParity` in `internal/config/parity_baseline_test.go`)
- [x] Add a partial-override test: setting only `wiki.repo_url` and
      `toc.enabled` does not drop sibling defaults
      (`TestLoad_PartialOverridesPreserveSiblingDefaults`)
- [x] Add a repo-root-config partial-override test covering the
      `MergeConfigMap` path (`TestLoad_RepoConfigPartialOverridesPreserveSiblingDefaults`)

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

- [x] Move config loading + validation out of `cobra.OnInitialize` into a
      `PersistentPreRunE` on `rootCmd` (`loadAndValidateConfig`)
- [x] Have the `PreRunE` return the validation error so Cobra exits non-zero
- [x] Keep printing warnings (non-fatal) before returning the error
- [x] Remove the `appCfg = cfg` assignment on the error path (load failure
      and validation failure both return before any assignment)
- [x] Add a test: a `.docz.yaml` with `statuses: []` causes a subcommand
      (`docz list rfc`) to fail with the validation message
      (`TestPersistentPreRunE_ValidationErrorFailsCommand` in `cmd/root_test.go`)
- [x] Add a test that `docz --help` still works with a broken config
      (`TestPersistentPreRunE_HelpWorksWithBrokenConfig`) — guards the
      Cobra-short-circuit behavior promised by Decisions §3

#### Success Criteria

- `docz` with a broken `.docz.yaml` exits 1 immediately with a clear message
- `docz --help` still works even with a broken `.docz.yaml` (help should not
  require config) — see Decisions §3

---

### Phase 4: Distinguish missing vs. unparseable config files

`mergeConfigFile` currently swallows both "file does not exist" and "YAML
parse error" silently. Surface the parse error.

#### Tasks

- [x] In `internal/config/config.go:mergeConfigFile`, change the second
      `return nil` (after `ReadInConfig`) to return a wrapped
      `parsing config file ...` error
- [x] Update the first `return nil` to check specifically for
      `errors.Is(err, fs.ErrNotExist)`; surface other stat errors
      (permission denied, etc.)
- [x] Call sites of `mergeConfigFile` already propagate the error
      (`return cfg, mergeErr`) — no change required
- [x] Add tests for: (a) missing file → returns defaults silently
      (`TestLoad_MissingRepoConfigSilent`), (b) malformed YAML → returns
      error with path in message (`TestLoad_MalformedRepoConfigReturnsError`),
      (c) permission denied → returns error
      (`TestLoad_UnreadableRepoConfigReturnsError`)

#### Success Criteria

- A `.docz.yaml` containing `not: valid: yaml: :` causes `docz` to exit 1
  with a clear "parse error in .docz.yaml: ..." message
- Tests for all three cases above pass

---

### Phase 5: Honor user-listed types only (INV-0003 fix)

Implement INV-0003's recommended Option A: when `.docz.yaml` includes a
top-level `types:` block, treat it as a *replacement* of the defaults map
rather than a merge target. Omission of a type means "not configured →
skipped". Absent `types:` block → fall through to the full default set.

This phase has to land before Phase 7's `EnabledTypes()` helper, because
the helper's return value is only meaningful once the config truly
reflects the user's intent.

#### Tasks

- [x] Add `userListedTypeNames(path)` in `internal/config/config.go` that
      parses the YAML at path into a raw `map[string]any` and returns the
      keys of the top-level `types:` map (nil if absent or unparseable)
- [x] Add `applyTypesReplaceOnPresence(cfg, path)` and invoke it from both
      `Load()` (with `ConfigFileName`) and `loadFromFile()` (with the
      explicit path). Global `~/.docz.yaml` is intentionally NOT considered
      for this check — repo file is the override boundary.
- [x] Document the new contract on `Load`'s doc comment and in the
      `docz init` long-help text
- [x] Add the five e2e tests INV-0003 enumerated in `cmd/inv0003_test.go`:
  1. `TestINV0003_RFCOnlyConfig_InitScaffoldsOnlyRFC`
  2. `TestINV0003_RFCOnlyConfig_UpdateTouchesOnlyRFC`
  3. `TestINV0003_NoConfig_InitScaffoldsAllSix`
  4. `TestINV0003_DisabledADRListed_OnlyRFCScaffolded`
  5. `TestINV0003_IncrementalAddType_PreservesExistingFiles`

#### Success Criteria

- The reproduction from INV-0003 — an rfc-only `.docz.yaml` + bare
  `docz init` — now scaffolds *only* `docs/rfc/`
- The green-field UX (no `.docz.yaml` present → all six dirs) is
  preserved, with a regression test guarding it
- Status of INV-0003 can be flipped to `Concluded` and linked to the
  PR for this wave

---

### Phase 6: Wrap bare `return err` sites with context

Walk every flagged site and add `fmt.Errorf("doing X: %w", err)` wrapping.

#### Tasks

- [x] Wrap `cmd/update.go` runUpdate loop (`updating %s: %w`) and
      `updateType` dry-run / readme paths
- [x] Wrap `cmd/create.go` `document.Create` call (`creating %s document`)
- [x] Wrap `cmd/init.go` `writeIndexReadme` call (`writing index readme for %s`),
      `writeDefaultConfig` call (`writing default config`), and
      `renderDefaultConfig` invocation (`rendering default config`)
- [x] Wrap `cmd/wiki.go` `writeMkDocsYAML`, `ensureDocsIndex`,
      `ensureDoczInit`, both `ReadMkDocs`, and `WriteMkDocs` sites with
      path-bearing wrappers
- [x] Wrap `cmd/list.go` table-writer / csv-writer stdout writes with
      `writing table header/separator/row %d` and `writing csv header/row %d`
- [x] `internal/wiki/wiki.go` recursive scanDir already returns wrapped
      errors at every site; verified via grep -- no bare `return err`
      remains in `internal/`
- [x] Audit sites missed by INV-0002: `grep -rn 'return err$' cmd/ internal/`
      now shows only three sites in `cmd/template.go`, each documented with
      a code comment explaining the unwrapped propagation (validateType
      already returns a fully-formatted error). Phase 7 collapses them
      further.

#### Success Criteria

- `grep -rn 'return err$' cmd/ internal/` returns no results (or each
  remaining instance has an explicit code comment justifying it)
- Every error message includes enough context to identify the failing
  operation

---

### Phase 7: Extract `ValidateType` helper and `EnabledTypes()` method

Collapse the four "unknown document type" sites and three enabled-type
guard blocks into single helpers.

#### Tasks

- [x] Add `(c *Config) ValidateType(name string) (canonical, err)` to
      `internal/config/config.go`: lowercases, resolves alias, looks up
      in `c.Types`, returns canonical name or wrapped `ErrUnknownType`
- [x] Define `var ErrUnknownType = errors.New("unknown document type")`
      sentinel so callers can `errors.Is` on it
- [x] Replace `cmd/create.go` lookup-and-format block with `ValidateType`
- [x] Replace `cmd/list.go` lookup-and-format block with `ValidateType`
- [x] Replace `cmd/update.go` lookup-and-format block with `ValidateType`
- [x] Replace the three sites in `cmd/template.go` with `ValidateType`
      and delete the local `validateType()` helper (plus its now-duplicate
      test in `cmd/template_test.go`)
- [x] Add `func (c *Config) EnabledTypes() []string` returning a sorted
      slice of canonical type names with `Enabled: true`
- [x] Replace iteration at `cmd/init.go:runInit` with `appCfg.EnabledTypes()`
- [x] Replace iteration at `cmd/update.go:runUpdate` with
      `appCfg.EnabledTypes()` (default branch when no positional arg)
- [x] Replace iteration at `cmd/wiki.go:ensureDocsIndex` with
      `appCfg.EnabledTypes()` (preallocates the slice with `len(enabled)`)
- [x] Reword the `Validate()` warning string for "type listed in config
      but not built-in" so the "unknown document type" phrase lives only
      on the `ErrUnknownType` sentinel

#### Success Criteria

- "unknown document type" string appears in exactly one source location
- `EnabledTypes()` returns sorted, deterministic output (verified by test)
- `make test` passes
- Behavior unchanged: existing CLI invocations produce identical output

---

### Phase 8: Add `TypeConfig.PluralLabel`; remove `"adr"` magic-string

Replace the special-case at `cmd/update.go:85-87` with config-driven
pluralization.

#### Tasks

- [x] Add `PluralLabel string` field to `TypeConfig` with
      `yaml:"plural_label,omitempty"`; document the precedence rule on
      the struct
- [x] Defaults in `DefaultConfig()` per Decisions §5:
      `rfc:"RFCs"`, `adr:"ADRs"`, `design:"Design"`,
      `impl:"Implementation Plans"`, `plan:"Plans"`,
      `investigation:"Investigations"`
- [x] Replace the magic-string block in `cmd/update.go:updateType` with
      `heading := "All " + tc.PluralLabel`; the `if typeName == "adr"`
      special case is gone
- [x] Update `cmd/wiki.go:ensureDocsIndex` nav-title fallback chain to
      `Wiki.NavTitles[name]` -> `tc.PluralLabel` -> `strings.ToUpper(name)`
      per Decisions §4
- [x] Add `fillTypeFieldDefaults` to `internal/config/config.go`: a
      reflective post-Unmarshal pass that backfills zero-valued
      string/int/nil-slice fields on each `cfg.Types` entry from
      `DefaultConfig()`. Closes the F49 case where mapstructure's
      map-of-struct decoding allocated fresh entries and dropped
      sibling defaults (the bug that surfaced when PluralLabel was
      added: existing user configs would have rendered "All " with
      no label).
- [x] Add `TestLoad_TypeFieldDefaultsBackfilled` and
      `TestLoad_TypeExplicitEmptyStatusesPreserved` regression tests
      covering the backfill and the nil-vs-empty slice distinction
- [x] Add `plural_label` to `docz_yaml.tmpl` so new `.docz.yaml` files
      include it
- [x] Re-run `docz update` against the repo's own docs; the only heading
      changes are the expected ones (DESIGNs→Design, IMPLs→Implementation
      Plans, INVESTIGATIONs→Investigations) plus the empty headings
      filled in for adr, plan, rfc (which were never rendered before)

#### Success Criteria

- No magic strings remain in `cmd/update.go`
- README index headings come from config, not string manipulation
- Existing repos with `.docz.yaml` files continue to work (back-compat:
  `PluralLabel` is optional and falls back to the old behavior if absent)

---

### Phase 9: Frontmatter CRLF tolerance

`document.ParseFrontmatter` currently rejects `---\r\n` line endings.

#### Tasks

- [x] In `internal/document/document.go`, relax the post-`---` check to
      accept either `\r\n` or `\n` (Windows or Unix line endings); the
      closing `\n---` cut already tolerated CRLF
- [x] Add table-driven cases covering `\n`, `\r\n`, and mixed line endings
      to `TestParseFrontmatter`

#### Success Criteria

- A docs file authored on Windows (CRLF) is parsed successfully
- Test covers all three line-ending variants

---

### Phase 10: Verify and ship

#### Tasks

- [x] Run `make ci` — green
- [x] Smoke test: bootstrap a fresh repo, run `docz init`, confirm
      `.docz.yaml` renders from the embedded template with the header
      comment block + per-section comments, and all six type dirs are
      scaffolded
- [x] Smoke test: corrupt `.docz.yaml` to `not: valid: yaml: : :`,
      confirm exit 1 with `loading config: parsing config file .docz.yaml: ...`
- [x] Smoke test: `.docz.yaml` with `types: rfc: {enabled: true, statuses: []}`,
      confirm `docz list rfc` exits 1 with
      `invalid config: type "rfc" has no statuses defined`
- [x] Open PR — PR #41 (label corrected from `dont-release` to `minor`
      post-review: this wave has user-visible behavior changes
      — hard-fail on broken `.docz.yaml`, README heading text shifts via
      `PluralLabel`, INV-0003 `types:` replace-on-presence semantics,
      parse-error surfacing — so it's not a no-release refactor)
- [x] Update INV-0002 status to "In Progress"

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

- [x] Round-trip test: `DefaultConfig() → template-render → yaml.Unmarshal →
      deep-equal` — `TestDoczYAMLTemplate_RoundTripsToDefaultConfig`
- [x] Reflective parity test: `Load()` with no config files deep-equals
      `DefaultConfig()`; `setDefaults` no longer exists —
      `TestLoad_DefaultsParity`,
      `TestLoad_PartialOverridesPreserveSiblingDefaults`,
      `TestLoad_RepoConfigPartialOverridesPreserveSiblingDefaults`,
      `TestLoad_TypeFieldDefaultsBackfilled`,
      `TestLoad_TypeExplicitEmptyStatusesPreserved`
- [x] Error-surfacing tests: missing config silent, malformed YAML
      surfaces wrapped error, permission-denied surfaces wrapped error
      — `TestLoad_MissingRepoConfigSilent`,
      `TestLoad_MalformedRepoConfigReturnsError`,
      `TestLoad_UnreadableRepoConfigReturnsError`
- [x] Validation-error tests: empty `docs_dir`, type with empty
      `statuses` — both cause non-zero exit via PersistentPreRunE:
      `TestValidate_EmptyDocsDir`, `TestValidate_EmptyStatuses`,
      `TestPersistentPreRunE_ValidationErrorFailsCommand`
- [x] **INV-0003 e2e suite** in `cmd/inv0003_test.go`: rfc-only config
      → only rfc; no-config → all six; disabled adr → only rfc;
      incremental add of adr → adr appears without disturbing rfc
      (five tests, all green)
- [x] `ValidateType` table-driven test: canonical names, aliases,
      unknown names, empty string, uppercase — `TestValidateType`
- [x] `EnabledTypes` test: order is sorted, disabled types excluded —
      `TestEnabledTypes`
- [x] `PluralLabel` smoke: ran `docz update` against the repo's own
      docs; only the expected headings changed (DESIGNs→Design,
      IMPLs→Implementation Plans, INVESTIGATIONs→Investigations) plus
      previously-empty headings filled for adr/plan/rfc
- [x] CRLF frontmatter test — `TestParseFrontmatter` table extended
      with `frontmatter with CRLF line endings` and
      `frontmatter with mixed line endings` subtests

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
- Delivers the resolution for INV-0003 as Phase 5 of this wave; INV-0003
  can be closed when this wave merges
- Must merge before IMPL-0008 (which assumes `EnabledTypes` and
  `ValidateType` are available)
- Must reach `Completed` before any v1 design/implementation work begins,
  per the INV-0004 prerequisite gate (alongside IMPL-0007/0008/0009)

## References

- INV-0002 — Wave 2, findings F7–F12, F34, F38–F40, F48, F49
- INV-0003 — Init and update should respect config-listed types only;
  Phase 5 of this wave implements its recommended Option A
- INV-0004 — v1 release plan; this wave is part of the hard prerequisite
  gate for v1 design/impl
- PR #30 — `markdown_extensions` defaults-drift case study
- PR #31 — disabled-types defaults-drift case study
- Viper `MergeConfigMap` semantics — relevant to Decisions §2
