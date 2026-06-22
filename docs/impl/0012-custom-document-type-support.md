---
id: IMPL-0012
title: "Custom Document Type Support"
status: Draft
author: Donald Gifford
created: 2026-06-18
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0012: Custom Document Type Support

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-06-18

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: internal/template — index-header resolution](#phase-1-internaltemplate--index-header-resolution)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: decouple internal/index + wire callers (output axis ships)](#phase-2-decouple-internalindex--wire-callers-output-axis-ships)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: internal/config — TypeConfig.Aliases + resolveType (input axis: single-type)](#phase-3-internalconfig--typeconfigaliases--resolvetype-input-axis-single-type)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: internal/config — EnabledTypes() inclusion + Validate collisions (input axis: iteration & safety)](#phase-4-internalconfig--enabledtypes-inclusion--validate-collisions-input-axis-iteration--safety)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: verify and ship](#phase-5-verify-and-ship)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Decisions](#decisions)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

## Objective

Implement DESIGN-0006 — make a user-declared custom document type (e.g.
`frameworks` with `id_prefix: FW`) a first-class citizen across docz. Today
a custom type declared in `.docz.yaml` is half-supported: `docz create
frameworks "…"` resolves and renders its body template from
`docs/templates/frameworks.md`, but the index-header write that follows
fails (`no embedded index header for type "frameworks"`), the type cannot
be invoked by its `id_prefix` (`docz create FW "…"`), and every no-argument
command (`docz update`, `docz init`, `docz list`, wiki nav) silently skips
it because `Config.EnabledTypes()` only walks the built-in registry.

This implements both axes of the design:

- **Output** — index-header resolution gains the disk-override → embedded
  type-specific → generated-fallback tiers body templates already have, via
  a new `template.ResolveIndexHeader`. `internal/index` is decoupled from
  `internal/template`.
- **Input** — type-argument resolution gains alias and `id_prefix` tiers
  (`Config.resolveType`), a new `TypeConfig.Aliases` field, the
  `EnabledTypes()` fix so custom types appear in no-arg iteration, and
  `Validate` collision rules guarding the new resolution keys.

**Implements:** DESIGN-0006 — Custom Document Type Support. All eight
decisions in that doc's §Decisions table are locked and inherited here
(notably **6b**: add an explicit per-type `aliases:` field on top of
implicit `id_prefix` matching). Implementation-level questions not settled
by the design are recorded in the [Decisions](#decisions) table.

## Scope

### In Scope

- New embedded `internal/template/templates/index_default.md` — the generic
  fallback index header, the only header rendered through `text/template`
- New `template.ResolveIndexHeader(docType, docsDir string, data
  IndexHeaderData) (string, error)` + `IndexHeaderData{TypeName,
  PluralLabel}`; removal of `template.EmbeddedIndexHeader`
- `internal/index` decoupled from `internal/template`:
  `UpdateReadme`/`DryRunReadme` take a resolved `header string` instead of
  `typeName`
- `cmd/update.go` and `cmd/init.go` resolve the header and pass it in; a
  shared `indexLabel(tc, typeName)` helper (PluralLabel → Title-cased name)
- New `TypeConfig.Aliases []string` field for per-type shorthands
- New `Config.resolveType` helper and refactor of `ValidateType` to the
  name → alias (registry + per-type) → `id_prefix` precedence
- `Config.EnabledTypes()` fixed to include enabled custom types in a
  deterministic order
- `Config.Validate` collision rules: duplicate `id_prefix` and ambiguous
  alias across enabled types (both hard errors per Decision 5)
- Tests at every layer (template tiers, index splice, config resolution +
  validation, cmd integration with a real custom type) and an end-to-end
  smoke

### Out of Scope

- An explicit per-type `index_template:` config key — Decision 7 defers it
- Auto-discovering/registering custom types from the filesystem — they must
  still be declared in `.docz.yaml`
- Changing body-template resolution (`template.Resolve`) — already correct
- Validating or templating the *contents* of user-authored index headers —
  a disk override is emitted verbatim
- Scaffolding editable `docs/templates/<type>.md` / `index_<type>.md` stubs
  on `docz init` — the generated fallback removes the need (Decision 7)
- Per-type custom *status lifecycles* beyond what `TypeConfig.Statuses`
  already supports — unchanged

## Implementation Phases

Each phase produces a self-contained, lint-clean, compiling commit, in the
IMPL-0011 style. Phases 1–2 deliver the output axis; Phases 3–4 deliver the
input axis; Phase 5 verifies and ships. Phases 1→2 and 3→4 are ordered by
dependency; the output and input axes are otherwise independent.

---

### Phase 1: `internal/template` — index-header resolution

Add the resolver and the generic fallback template **additively** —
`EmbeddedIndexHeader` stays in place so `internal/index` and `cmd/init`
keep compiling; it is removed in Phase 2 once callers migrate. Only the
generic-fallback tier is rendered through `text/template`; type-specific
embedded headers and disk overrides are returned verbatim (Decision 2),
which keeps the six built-in headers byte-identical.

#### Tasks

- [x] Create `internal/template/templates/index_default.md` as a
      `text/template` referencing `{{ .PluralLabel }}` and `{{ .TypeName }}`
      (title heading, one-line description, and a `docz create {{ .TypeName }}`
      example). Match the trailing-newline shape of the built-in
      `index_*.md` files so the spliced marker block spacing is identical
- [x] Add `type IndexHeaderData struct { TypeName, PluralLabel string }` in
      `internal/template/template.go`, next to `Resolve`/`Data`
- [x] Implement `ResolveIndexHeader(docType, docsDir string, data
      IndexHeaderData) (string, error)` with three tiers:
  1. on-disk override `filepath.Join(docsDir, config.TemplatesDir,
     "index_"+docType+".md")` → return verbatim if readable
  2. embedded `templates/index_<docType>.md` → return verbatim if present
  3. embedded `templates/index_default.md` → parse with `text/template`,
     execute with `data`, return rendered
- [x] Only tier 3 is rendered; tiers 1–2 return raw bytes (a literal `{{`
      in a user override or built-in header must survive untouched)
- [x] Leave `EmbeddedIndexHeader` in place for this phase (removed Phase 2)
- [x] Write `internal/template` tests:
  - tier 1: a `t.TempDir()` `templates/index_x.md` (containing a literal
    `{{ raw }}`) is returned byte-for-byte
  - tier 2: `ResolveIndexHeader("rfc", …)` equals the current
    `EmbeddedIndexHeader("rfc")` output byte-for-byte (golden-stability
    guard for all six built-ins, table-driven)
  - tier 3: an unknown type with no override renders `index_default.md` and
    the output contains the `PluralLabel` and the `docz create <type>` line
  - tier 3 empty label: `IndexHeaderData{PluralLabel: ""}` still yields a
    non-empty, well-formed header

#### Success Criteria

- `go build ./...` and `go vet ./...` clean; `make lint` zero issues
- `go test -race -shuffle=on -count=3 ./internal/template/...` green
- `ResolveIndexHeader` returns each built-in header byte-identical to the
  pre-change `EmbeddedIndexHeader`
- No change yet to `internal/index` or `cmd/`; `EmbeddedIndexHeader` still
  present and used

---

### Phase 2: decouple `internal/index` + wire callers (output axis ships)

Make `internal/index` a pure marker-splicer that receives a resolved header
string, move header resolution to the cmd callers, and delete
`EmbeddedIndexHeader` (Decision 1). After this phase, `docz create
frameworks "…"` and `docz update frameworks` succeed end-to-end for a
custom type (the type already resolves by its canonical name; only the
header was missing).

#### Tasks

- [x] Change `index.UpdateReadme(readmePath, header, tableContent string)`
      and `index.DryRunReadme(readmePath, header, tableContent string)` —
      replace the `typeName` parameter with the resolved `header`
- [x] `createNewReadme` and the `DryRunReadme` not-exist branch use the
      passed-in `header` directly; remove the `doctemplate` import and the
      `EmbeddedIndexHeader` calls (`index.go:111`, `:153`)
- [x] Add a shared `indexLabel(tc config.TypeConfig, typeName string)
      string` cmd helper: `tc.PluralLabel` when set, else a Title-cased
      `typeName` (Decision 3). Place it where both `update.go` and
      `init.go` can use it
- [x] `cmd/update.go` `updateType`: build `IndexHeaderData{TypeName:
      typeName, PluralLabel: indexLabel(tc, typeName)}`, call
      `doctemplate.ResolveIndexHeader(typeName, r.Cfg.DocsDir, data)`, and
      pass the resulting `header` into `DryRunReadme`/`UpdateReadme`
- [x] `cmd/init.go` `writeIndexReadme`: look up `tc := r.Cfg.Types[typeName]`,
      resolve the header via `ResolveIndexHeader`, drop the
      `EmbeddedIndexHeader` call (`init.go:117`)
- [x] Remove `EmbeddedIndexHeader` from `internal/template/embed.go`
- [x] Update `internal/index` tests for the new signature; add a case
      proving an arbitrary header string is spliced above the table verbatim
- [x] Add cmd integration tests (constructed `Runner`, `RepoRoot:
      t.TempDir()`): a repo whose `.docz.yaml` enables `frameworks` →
      `docz update frameworks` creates `docs/frameworks/README.md` with the
      generated header; with a `docs/templates/index_frameworks.md`
      override present, the override wins
- [x] Update CLAUDE.md: `internal/index` no longer depends on
      `internal/template`; note `ResolveIndexHeader` + `index_default.md` +
      `indexLabel`

> **Implementation note (intentional deviation):** `indexLabel` takes
> `(pluralLabel, typeName string)` rather than `(tc config.TypeConfig,
> typeName string)`. golangci-lint's `gocritic hugeParam` flags passing the
> 120-byte `TypeConfig` by value; the helper only needs the label, so callers
> pass `tc.PluralLabel`. The registry-coupling test
> `TestDocTypeRegistry_AllHaveEmbeddedIndexHeader` moved from
> `internal/config` to `internal/template`'s
> `TestResolveIndexHeader_EmbeddedBuiltin` (it now reads the embedded FS
> directly and asserts byte-identity for every registry type).

#### Success Criteria

- `docz update frameworks` and `docz create frameworks "X"` succeed
  end-to-end against a repo with a `frameworks` type, writing a README with
  a header (generated fallback, or override when present)
- Built-in index READMEs are byte-unchanged: `docz update` on this repo
  produces no diff to the six built-in `README.md` files, and Phase 1's
  tier-2 golden guard still passes
- `go test -race -shuffle=on -count=3 ./internal/index/... ./cmd/...` green
- `make lint` clean

---

### Phase 3: `internal/config` — `TypeConfig.Aliases` + `resolveType` (input axis: single-type)

Add the per-type alias field and the new resolution precedence so a custom
type resolves by its `id_prefix` or a declared alias on every
single-`<type>` command (`create`, `update <type>`, `list <type>`,
`status set`, `template`). Validation of collisions lands in Phase 4.

#### Tasks

- [x] Add `Aliases []string` to `TypeConfig`
      (`mapstructure:"aliases" yaml:"aliases,omitempty"`), documented as
      per-type shorthands (Decision 6)
- [x] Implement `(c *Config) resolveType(name string) (canonical string,
      ok bool)` with case-insensitive precedence:
  1. canonical name — `c.Types[lower(name)]`
  2. alias — built-in registry alias (`ResolveTypeAlias`) **or** any enabled
     type whose `TypeConfig.Aliases` contains `lower(name)`
  3. `id_prefix` — the type whose `strings.EqualFold(tc.IDPrefix, name)`
- [x] Refactor `ValidateType` to delegate to `resolveType`, keeping its
      single `ErrUnknownType` error site. The "valid types" hint lists the
      configured enabled types (via `EnabledTypes()` — see Phase 4) rather
      than only the built-in registry
- [x] Honor `TypeConfig.Aliases` for any type, built-in or custom (union
      with registry aliases) per Decision 6
- [x] Write `internal/config` tests (table-driven, `t.Parallel()`):
  - prefix resolution: `FW` and `fw` → `frameworks`
  - per-type alias resolution: a declared `aliases: [fw]` → `frameworks`
  - precedence: a custom type with `id_prefix: PLAN` does not shadow the
    built-in `plan` (name tier wins)
  - built-ins still resolve by name and registry alias (`inv`,
    `implementation`); `RFC`/`rfc` both resolve
  - an unknown token returns `ErrUnknownType`

> **Implementation notes:** `resolveType` resolves by name/alias/prefix
> regardless of a type's `enabled` flag — matching the pre-existing name-tier
> behavior, where the `enabled` gate lives in the cmd layer
> (`Create` rejects a disabled type). The alias and prefix loops range
> `c.Types` keys only (not value) because adding `Aliases` pushed
> `TypeConfig` to 144 bytes, over gocritic's `rangeValCopy` threshold; the
> two pre-existing value-range loops in `config.go` (`Validate`,
> `fillTypeFieldDefaults`) were converted the same way in this phase.

#### Success Criteria

- `docz create FW "X"`, `docz list fw`, `docz status set FW FW-0003
  <status>`, `docz update FW` all resolve to the `frameworks` type
- All existing built-in resolution behavior is preserved (no test
  regressions); `RFC`/`ADR`/… shorthands now also resolve (Decision 4)
- `go test -race -shuffle=on -count=3 ./internal/config/... ./cmd/...` green
- `make lint` clean

---

### Phase 4: `internal/config` — `EnabledTypes()` inclusion + `Validate` collisions (input axis: iteration & safety)

Close the no-argument iteration gap and add the guardrails the new
resolution keys require.

#### Tasks

- [x] Fix `EnabledTypes()` to include enabled custom types. Built-in types
      stay in registry-declaration order; enabled custom keys (those not in
      the built-in registry) are appended in a deterministic order
      (sorted — Decision 1) so map iteration never makes
      `docz update`/`list`/wiki output unstable
- [x] Add `Validate` rule: duplicate `id_prefix` (case-insensitive) across
      enabled types is a hard error (Decision 5), naming the prefix and the
      colliding types
- [x] Add `Validate` rule: a `TypeConfig.Aliases` entry that collides
      (case-insensitive) with another enabled type's canonical name, its
      registry alias, its `id_prefix`, or another type's alias is a hard
      error (Decision 5 fixes the exact collision domain)
- [x] Keep the existing `non-built-in type %q (typo?)` warning (Decision 8)
- [x] Write tests:
  - `EnabledTypes()` includes an enabled custom type at the expected
    position; a disabled custom type is excluded; ordering is stable across
    runs
  - cmd integration: no-arg `docz update` processes the custom type
    (`TestUpdate_NoArgIncludesCustomType`); `docz list`/`init` share the
    same `EnabledTypes()` iteration
  - `Validate` errors on duplicate `id_prefix`
  - `Validate` errors on an alias colliding with a name / registry alias /
    prefix / another alias
  - a well-formed custom-type config passes `Validate` (only the existing
    typo warning)
- [x] Update CLAUDE.md: `EnabledTypes()` now includes custom types;
      `TypeConfig.Aliases`; `resolveType` precedence; new `Validate` rules

> **Implementation note:** the `Validate` collision rules live in a
> `validateResolution` helper called at the end of `Validate`; the
> collision domain is built by "claiming" each token (name, per-type alias,
> id_prefix, and built-in registry aliases for enabled built-ins) into a
> `map[token]owner`, where a token claimed by two different owners is the
> error. A token claimed twice by the *same* type (e.g. a built-in whose
> name and `id_prefix` both lower-case to `rfc`) is allowed.

#### Success Criteria

- No-arg `docz update` creates/updates the custom type's README; `docz
  list` shows it; `docz init` scaffolds it; `docz wiki` nav includes it
  (title via the existing `NavTitles → PluralLabel → ToUpper` cascade)
- A config with duplicate `id_prefix` or an ambiguous alias fails at
  startup (`PersistentPreRunE`) with a clear, actionable message
- `go test -race -shuffle=on -count=3 ./...` green
- `make lint` clean

---

### Phase 5: verify and ship

#### Tasks

- [x] Full `make ci` green (lint + test + build + license-check)
- [x] `go test -race -shuffle=on -count=3 ./...` green
- [x] End-to-end smoke in a scratch repo (`/tmp`): a `.docz.yaml` enabling a
      `frameworks` type with `id_prefix: FW`, `aliases: [fw]`, and a
      `docs/templates/frameworks.md` body template. Verify, in order:
  - `docz create FW "First Framework"` → `docs/frameworks/0001-first-framework.md`
    with frontmatter `id: FW-0001` from the body template, and
    `docs/frameworks/README.md` written with a header
  - `docz create fw "Second"` resolves the same type
  - `docz list fw` and no-arg `docz update` include the type
  - `docz status set FW FW-0001 <status>` mutates one frontmatter line
  - a `docs/templates/index_frameworks.md` override is honored when the
    README is (re)generated
- [x] Confirm built-in goldens unchanged: `docz update` on this repo shows
      no diff to the six built-in `README.md` files
- [x] CLAUDE.md fully reflects the final architecture (consolidate the
      per-phase edits)
- [x] Run `docz update` to refresh the impl/design index READMEs and nav
- [ ] Flip DESIGN-0006 frontmatter status `Approved` → `Implemented` after
      merge to main *(post-merge)*
- [ ] Flip this doc's frontmatter status `Draft` → `Completed` after merge
      to main *(post-merge)*
- [x] Open the PR with the `minor` release label (new user-facing feature,
      additive) — single PR per Decision 8

> **Smoke-test findings (verified 2026-06-22):** created filenames follow
> the existing `<number>-<slug>.md` convention (`0001-first-framework.md`),
> not a prefix-bearing name — the `id_prefix` lands in the frontmatter `id:`
> (`FW-0001`), matching the built-in types. The index `index_<type>.md`
> override is applied by `internal/index` only when the README is created
> (no markers yet); on an existing README, `UpdateReadme` is a pure
> marker-splicer and preserves the user-editable header prose above the
> markers — so an override added after first generation takes effect on the
> next regeneration (e.g. after the README is removed), which is the
> intended marker semantics, not a regression.

#### Success Criteria

- `make ci` green on the final commit
- The scratch-repo smoke exercises create-by-prefix, create-by-alias,
  no-arg update/list, status set, and the index override — all pass
- No diff to built-in index READMEs or golden fixtures
- CLAUDE.md mentions custom-type support in the relevant `internal/template`,
  `internal/index`, `internal/config`, and `cmd/` lines
- Branch ready to merge with the standard squash flow

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/template/templates/index_default.md` | Create | Generic fallback index header (`text/template`) |
| `internal/template/template.go` | Modify | Add `ResolveIndexHeader` + `IndexHeaderData` |
| `internal/template/embed.go` | Modify | Remove `EmbeddedIndexHeader` (Phase 2) |
| `internal/template/template_test.go` | Modify/Create | `ResolveIndexHeader` tier + golden-stability tests |
| `internal/index/index.go` | Modify | `UpdateReadme`/`DryRunReadme` take `header string`; drop `template` import |
| `internal/index/index_test.go` | Modify | New header-param signature + verbatim-splice case |
| `cmd/update.go` | Modify | Resolve header via `ResolveIndexHeader`; `indexLabel` helper |
| `cmd/init.go` | Modify | `writeIndexReadme` resolves header via `ResolveIndexHeader` |
| `cmd/*_test.go` | Modify | Custom-type integration: update/init/create/list/status |
| `internal/config/config.go` | Modify | `TypeConfig.Aliases`; `resolveType`; `ValidateType`; `EnabledTypes`; `Validate` collisions |
| `internal/config/config_test.go` | Modify | resolution, `EnabledTypes`, `Validate` tests |
| `CLAUDE.md` | Modify | Architecture updates across the phases |
| `docs/design/0006-custom-document-type-support.md` | Modify | Flip `Approved` → `Implemented` on merge |
| `docs/impl/0012-custom-document-type-support.md` | Modify | Flip `Draft` → `Completed` on merge |
| `docs/impl/README.md`, `docs/design/README.md` | Modify | Auto-regenerated by `docz update` |

## Testing Plan

- [ ] `internal/template` — tier-1 verbatim (incl. literal `{{`), tier-2
      byte-equality for all six built-ins, tier-3 render with and without a
      `PluralLabel`
- [ ] `internal/index` — new `header`-string signature; arbitrary header
      spliced verbatim; existing marker/no-marker/dry-run cases preserved
- [ ] `internal/config` — `resolveType` precedence matrix (name / registry
      alias / per-type alias / prefix / unknown); `EnabledTypes` inclusion +
      stable ordering; `Validate` duplicate-prefix and alias-collision errors
- [ ] `cmd` integration (constructed `Runner`, `RepoRoot: t.TempDir()`) —
      custom type through `create` (by prefix and alias), no-arg `update`,
      `list`, `init`, `status set`; index override precedence
- [ ] Golden stability — `go test ./... -update` changes **no** built-in
      golden files
- [ ] `go test -race -shuffle=on -count=3 ./...` green at the end of every
      phase
- [ ] One end-to-end scratch-repo smoke captured in Phase 5

## Decisions

Resolved by user review on 2026-06-18. All recommendations accepted.
These are implementation-level choices; DESIGN-0006 §Decisions remains
authoritative for the design-level choices this plan inherits.

| # | Topic | Choice | Rationale |
|---|-------|--------|-----------|
| 1 | `EnabledTypes()` ordering | (a) Built-ins in registry-declaration order, then enabled custom types sorted alphabetically | Stable and predictable; built-ins keep their familiar order; Go maps need a deterministic sort |
| 2 | Scope of the `EnabledTypes()` fix | (a) Fix it in this IMPL (Phase 4) | The design's "succeed for any enabled type" is unmet without it — the line between usable and first-class |
| 3 | Placement of `ResolveIndexHeader` | (a) `internal/template/template.go`, next to `Resolve`/`ResolveWikiIndex` | Keeps all resolvers together; `embed.go` stays a pure embedded-FS accessor |
| 4 | `index_default.md` content | (a) Minimal — `# {{ .PluralLabel }}` heading, one-line description, `docz create {{ .TypeName }}` example | Safe for a type of unknown semantics; no dependence on `TypeConfig.Statuses` |
| 5 | `Validate` collision domain | (a) Union over enabled types of {canonical name, registry alias, per-type `aliases:`, `id_prefix`}, case-insensitive | Mirrors exactly what `resolveType` can reach, so any in-set duplicate is a real ambiguity |
| 6 | `TypeConfig.Aliases` on built-ins | (a) Yes — union a built-in's `aliases:` with its registry aliases | One code path; lets users add e.g. `r` for `rfc` |
| 7 | `docz init` template stubs | (a) No — rely on the generated fallback + optional `docs/templates/` overrides | A custom type works with zero scaffolded files; avoids littering the tree (a body template is still required to `create`) |
| 8 | PR strategy | (a) One PR for all five phases, label `minor` | Cohesive feature, ~<600 LOC; matches the IMPL-0009/0011 single-PR precedent |

## Dependencies

- **Blocking:** none. DESIGN-0006 is `Approved`; the IMPL-0009 Runner
  pattern (`Runner`, `inRepo`, `RepoRoot`, the DocType registry) is on main
  and is the foundation every cmd change builds on
- **Builds on:** `template.Resolve` (body-template 3-tier, the model
  `ResolveIndexHeader` copies) and `template.ResolveWikiIndex` (disk-override
  precedent)
- **Roadmap:** follows the v1 sequence after IMPL-0011 (status set);
  unrelated to the rfc-api consumer that drove IMPL-0011

## References

- [DESIGN-0006](../design/0006-custom-document-type-support.md) — the design
  this implements; its §Decisions table (1a 2a 3a 4a 5a 6b 7a 8a) is the
  authoritative answer for each locked choice
- [IMPL-0009](0009-runner-pattern-and-doctype-registry-refactor.md) — Runner
  pattern + DocType registry this builds on
- `internal/template/template.go:83` — `Resolve`, the body-template resolver
- `internal/template/template.go:119` — `ResolveWikiIndex`, disk-override
  precedent
- `internal/template/embed.go:29` — `EmbeddedIndexHeader` (to be removed)
- `internal/index/index.go:111`, `:153` — header call sites being decoupled
- `cmd/update.go:84`, `cmd/init.go:117` — caller wiring points
- `internal/config/config.go:196` — `ValidateType` chokepoint;
  `EnabledTypes` and `Validate` in the same file
- `internal/config/config.go:26` — `TypeConfig` (gains `Aliases`)
- `internal/config/doctype.go:38` — `DocTypeDef.Aliases` registry (unioned
  in resolution tier 2)
- `cmd/wiki.go:299` — existing `NavTitles → PluralLabel → ToUpper` nav-title
  cascade that custom types reuse once in `EnabledTypes()`
