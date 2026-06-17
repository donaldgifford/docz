---
id: DESIGN-0006
title: "Custom Document Type Support"
status: Draft
author: Donald Gifford
created: 2026-06-17
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0006: Custom Document Type Support

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-06-17

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
  - [Output side: template vs index-header resolution](#output-side-template-vs-index-header-resolution)
  - [Input side: how <type> arguments resolve](#input-side-how-type-arguments-resolve)
- [Detailed Design](#detailed-design)
  - [Output: index-header resolution (template.ResolveIndexHeader)](#output-index-header-resolution-templateresolveindexheader)
    - [Decoupling internal/index from internal/template](#decoupling-internalindex-from-internaltemplate)
  - [Input: type-argument resolution by id_prefix](#input-type-argument-resolution-by-idprefix)
    - [Prefix collisions and precedence](#prefix-collisions-and-precedence)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. Where should index-header resolution live?](#1-where-should-index-header-resolution-live)
  - [2. What should the generic fallback header contain?](#2-what-should-the-generic-fallback-header-contain)
  - [3. How to label a custom type that omits plural_label?](#3-how-to-label-a-custom-type-that-omits-plurallabel)
  - [4. Should built-ins also accept id_prefix shorthands?](#4-should-built-ins-also-accept-idprefix-shorthands)
  - [5. Duplicate id_prefix — hard error or warning?](#5-duplicate-idprefix--hard-error-or-warning)
  - [6. Add an explicit per-type aliases: field to TypeConfig?](#6-add-an-explicit-per-type-aliases-field-to-typeconfig)
  - [7. Add an explicit per-type index_template: config key?](#7-add-an-explicit-per-type-indextemplate-config-key)
  - [8. Soften the non-built-in type (typo?) warning for declared custom types?](#8-soften-the-non-built-in-type-typo-warning-for-declared-custom-types)
- [References](#references)
<!--toc:end-->

## Overview

docz lets a repository declare extra document types in `.docz.yaml` beyond
the six built-ins, but two rough edges keep a custom type from being
first-class:

1. **Output side — index header.** The first `docz update` / `docz init`
   for a custom type (e.g. `frameworks`) fails with
   `no embedded index header for type "frameworks"`, because index-header
   resolution is embedded-only while body-template resolution has a
   disk-override → embedded fallback chain.
2. **Input side — invocation.** A custom type can only be invoked by its
   canonical config-map name (`docz create frameworks "…"`). Its
   `id_prefix` (e.g. `FW`) is not accepted as a CLI shorthand, so
   `docz create FW "…"` / `docz create fw "…"` errors.

This design closes both: index-header resolution gains the same three-tier
behavior body templates already have, and type-argument resolution gains an
`id_prefix` tier so `docz create FW "…"` (or `fw`) resolves to the
`frameworks` type and uses the template at `docs/templates/frameworks.md`.

## Goals and Non-Goals

### Goals

- **Index headers resolve for any enabled type.** `docz update` /
  `docz init` succeed without an embedded `index_<type>.md`; honor an
  on-disk override at `<docs_dir>/templates/index_<type>.md`; fall back to
  a generated header using the type's `plural_label`.
- **Invoke any enabled type by its `id_prefix`,** case-insensitively
  (`FW` or `fw`), on every subcommand that takes a `<type>` argument
  (`create`, `update`, `list`, `status`, `template`).
- **Resolution precedence never lets a prefix shadow a real type
  name/alias** — name beats alias beats prefix.
- **A custom type whose template lives at `docs/templates/<type>.md`
  works end to end:** create (body template) + update (index header) +
  invoke-by-prefix, with no embedded assets and no fork of docz.
- **Preserve built-in output byte-for-byte** — no golden churn for
  `rfc`/`adr`/`design`/`impl`/`plan`/`investigation`. Built-ins
  *additionally* gain prefix shorthands (`RFC`, `ADR`, …) for consistency.

### Non-Goals

- An explicit per-type `aliases:` field on `TypeConfig`. Implicit
  `id_prefix` resolution covers the requested shorthand; arbitrary
  user-declared aliases are deferred (see Open Questions §6).
- An explicit per-type `index_template:` config key (Open Questions §7).
- Auto-discovering or auto-registering types from the filesystem — a
  custom type must still be declared in `.docz.yaml`.
- Changing body-template resolution — `template.Resolve` already handles
  custom types correctly and is the model this design copies.
- Validating or templating the contents of user-authored index headers (a
  disk override is emitted verbatim, exactly as today).

## Background

A custom type touches two independent resolution paths, and each has a
rough edge.

### Output side: template vs index-header resolution

docz resolves two artifacts when it materializes a type:

1. **Body template** — `template.Resolve(docType, configPath, docsDir)`
   (`internal/template/template.go:83`), three tiers:
   1. explicit `configPath` (the type's `template:` field), then
   2. on-disk override `<docsDir>/templates/<docType>.md`, then
   3. embedded default `templates/<docType>.md`.
2. **Index header** — the prose above the auto-generated table in a type's
   `README.md`. Resolved by `template.EmbeddedIndexHeader(docType)`
   (`internal/template/embed.go:29`), which reads
   `templates/index_<docType>.md` from the **embedded FS only** — no
   `configPath` tier, no on-disk tier.

That asymmetry is the output-side bug. The embedded FS ships
`index_<type>.md` for the six built-ins and nothing else, so for a custom
type the single lookup misses and returns `no embedded index header for
type %q`. It surfaces at three call sites:

- `internal/index/index.go:153` — `createNewReadme` (`docz update`).
- `internal/index/index.go:111` — `DryRunReadme` (`docz update --dry-run`).
- `cmd/init.go:117` — `writeIndexReadme` (`docz init`).

A user reasonably expects the same disk convention the body template
honors: drop a file under `docs/templates/` and have docz pick it up.
Today that works for `docs/templates/frameworks.md` (body) but not
`docs/templates/index_frameworks.md` (header).

Two facts shape the index-header fix:

- **Index headers are static markdown, not `text/template`.** The built-in
  `index_*.md` files contain no `{{ }}` actions; they are read verbatim and
  concatenated with the marker block and table (`index.go:158`). Rendering
  existing files through a template engine would risk reinterpreting a
  literal `{{` in user prose, so verbatim emission must stay the default.
- **A custom type already carries a display label.** `TypeConfig` has
  `PluralLabel` (`internal/config/config.go:34`), and `cmd/update.go:84`
  already builds the table heading from `tc.PluralLabel`.

### Input side: how `<type>` arguments resolve

Every `<type>` argument flows through one chokepoint,
`Config.ValidateType` (`internal/config/config.go:196`):

```
ValidateType(arg) → ResolveTypeAlias(lower(arg)) → look up c.Types[canonical]
```

`ResolveTypeAlias` only knows the **built-in registry's** aliases
(`DocTypeDef.Aliases` — `inv`, `implementation`); `TypeConfig` has no alias
field. A custom `frameworks` declared in `.docz.yaml` lands in `c.Types`,
so:

- `docz create frameworks "…"` **already resolves today**, and because
  `document.Create` passes `tc.Template` into `template.Resolve`
  (`cmd/create.go:99-110`), it **already uses `docs/templates/frameworks.md`**
  via `Resolve` tier 2. The *only* thing that broke was the index-header
  write afterward (the output-side bug above).
- `docz create FW "…"` / `fw "…"` → `ResolveTypeAlias("fw")` finds nothing
  → `c.Types["fw"]` misses (the map key is `frameworks`) → `ErrUnknownType`.

So `id_prefix` is data on `TypeConfig` (`config.go:30`) but is **not a
resolution key**. Two further facts:

- `Config.Validate` currently emits a `non-built-in type %q (typo?)`
  warning for any custom type, and **does not enforce `id_prefix`
  uniqueness** across types.
- Because all subcommands route through `ValidateType`, a change there is
  uniform across `create` / `update` / `list` / `status` / `template`.

## Detailed Design

### Output: index-header resolution (`template.ResolveIndexHeader`)

Add a sibling to `Resolve` that mirrors its tiered lookup, specialized for
index headers:

```go
// IndexHeaderData is the render context for the generic fallback index
// header (templates/index_default.md). Type-specific embedded headers and
// on-disk overrides are emitted verbatim and ignore this data.
type IndexHeaderData struct {
    TypeName    string // canonical type name, e.g. "frameworks"
    PluralLabel string // display label, e.g. "Frameworks"
}

// ResolveIndexHeader returns the index-header prose for docType, checking
// override sources in order:
//  1. On-disk override at <docsDir>/templates/index_<docType>.md (verbatim)
//  2. Embedded type-specific header templates/index_<docType>.md (verbatim)
//  3. Embedded generic header templates/index_default.md (rendered with data)
func ResolveIndexHeader(docType, docsDir string, data IndexHeaderData) (string, error)
```

Tiers:

1. **On-disk override** — `filepath.Join(docsDir, config.TemplatesDir,
   "index_"+docType+".md")`. If readable, return its bytes verbatim,
   identical to `Resolve` tier 2.
2. **Embedded type-specific** — `templates/index_<docType>.md` via the
   existing `templateFS`. Hits for the six built-ins; returned verbatim, so
   built-in output is unchanged.
3. **Embedded generic fallback** — a new embedded `templates/index_default.md`,
   parsed with `text/template` and executed with `data`. This is the only
   rendered tier, so the `{{ }}`-reinterpretation risk is confined to a file
   docz ships and controls; it runs only when neither a disk override nor an
   embedded type-specific header exists — i.e. exclusively for custom types
   with no override.

#### Decoupling `internal/index` from `internal/template`

Today `internal/index` reaches into `internal/template` for the header
(`index.go:111`, `:153`) — the only reason index depends on templates at
all, and what fails for custom types. Rather than thread `docsDir` + label
*into* index, **lift header resolution out of index** so the package
becomes a pure marker-splicer:

- `UpdateReadme(readmePath, typeName, tableContent string)` →
  `UpdateReadme(readmePath, header, tableContent string)`.
- `DryRunReadme(readmePath, typeName, tableContent string)` →
  `DryRunReadme(readmePath, header, tableContent string)`.
- `createNewReadme` and the `DryRunReadme` create branch use the passed-in
  `header` directly and drop the `doctemplate` import.

The caller resolves the header once and passes the string in. This matches
the layering already documented in `CLAUDE.md` — *"this package depends on
`document.DocEntry` for its input type"* — index should not also depend on
`internal/template`. All template resolution (body, wiki index, index
header) then lives in one package.

`cmd/update.go` `updateType` already holds `tc` and the resolved `docsDir`;
`cmd/init.go` `writeIndexReadme` looks up `r.Cfg.Types[typeName]`. Both call
`ResolveIndexHeader(typeName, r.Cfg.DocsDir, IndexHeaderData{...})` and pass
the result to index. A tiny `indexLabel(tc, typeName)` helper returns
`tc.PluralLabel` when set, else a Title-cased `typeName` (Open Questions
§3). `EmbeddedIndexHeader` is removed once the three call sites migrate.

### Input: type-argument resolution by `id_prefix`

Extend the single chokepoint, `Config.ValidateType`, with a third
resolution tier. Precedence (highest first):

1. **Canonical name** — `c.Types[lower(arg)]`.
2. **Alias** — `ResolveTypeAlias(lower(arg))` then `c.Types[…]`.
3. **`id_prefix`** — the unique enabled type whose
   `lower(IDPrefix) == lower(arg)`.

Factor the lookup into an unexported helper so `ValidateType` keeps its
single error site:

```go
// resolveType maps a user-supplied token (case-insensitive) to a canonical
// Types key via name, then alias, then id_prefix. ok is false if none match.
func (c *Config) resolveType(name string) (canonical string, ok bool)
```

Tier 3 scans `c.Types` once; the match is case-insensitive on both sides so
`FW` and `fw` both resolve the type whose `IDPrefix` is `"FW"`. Because all
`<type>`-taking subcommands call `ValidateType`, prefix invocation is
uniform — `docz create FW "…"`, `docz list fw`, `docz status set FW
FW-0003 Approved` — and built-ins gain it too (`docz create RFC "…"`).

Once a token resolves to the canonical type, **nothing downstream changes**:
`document.Create` already passes `tc.IDPrefix` and `tc.Template` into
`template.Resolve` (`create.go:99-110`, `template.go:83`), so the body
template at `docs/templates/<type>.md` is used exactly as for the
by-name path. This tier only widens *which inputs resolve to the type*.

#### Prefix collisions and precedence

- **Precedence guarantees no shadowing.** A prefix is consulted only after
  name and alias miss, so a custom type with `id_prefix: PLAN` can never
  hijack the `plan` type — `plan` resolves by name (tier 1) first.
- **Ambiguous prefixes must be rejected.** Two enabled types sharing an
  `id_prefix` (case-insensitive) make tier 3 ambiguous. Add a
  `Config.Validate` rule that returns an error on duplicate enabled
  `id_prefix` values. This is unvalidated today; the rule is new but only
  fires on a config that was already broken for ID generation.

## API / Interface Changes

- **`internal/template`** — `ResolveIndexHeader(docType, docsDir string,
  data IndexHeaderData) (string, error)` and `type IndexHeaderData` added;
  `EmbeddedIndexHeader` **removed** (internal; the three callers migrate in
  the same change).
- **`internal/index`** — `UpdateReadme` / `DryRunReadme` take a resolved
  `header string` instead of `typeName`; the package drops its
  `internal/template` import.
- **`internal/config`**:
  - `ValidateType` gains an `id_prefix` resolution tier (behavioral; the
    signature is unchanged). New unexported `resolveType` helper.
  - `Validate` gains a duplicate-`id_prefix` error (across enabled types,
    case-insensitive).
- **CLI** — no new flags. New *accepted inputs*: any enabled type's
  `id_prefix` (case-insensitive) as the `<type>` argument on `create` /
  `update` / `list` / `status` / `template`. New *override path*:
  `docs/templates/index_<type>.md`. Net effect: custom types are fully
  usable and built-ins gain prefix shorthands.

## Data Model

- **New embedded template** `internal/template/templates/index_default.md`,
  a `text/template` referencing `IndexHeaderData`. Sketch:

  ```markdown
  # {{ .PluralLabel }}

  This directory contains {{ .PluralLabel }}.

  ## Creating a New Document

  ```bash
  docz create {{ .TypeName }} "Your Title"
  ```
  ```

  It is the only index header rendered through the engine; the six built-in
  `index_*.md` files stay static and are emitted verbatim.

- **On-disk conventions (paths, not schema):**
  - `<docs_dir>/templates/index_<type>.md` — index-header override (new),
    parallel to the body override `<docs_dir>/templates/<type>.md`
    (existing, already honored).
- **`id_prefix` gains a uniqueness constraint** (case-insensitive, across
  enabled types) — validated, not stored differently. No frontmatter,
  README-marker, or persisted-state changes.

## Testing Strategy

- **`internal/template`** — `ResolveIndexHeader` tiers: (1) a temp-dir
  `templates/index_x.md` is returned verbatim, including a literal `{{`
  that must *not* render; (2) a built-in (`rfc`) returns the embedded
  header byte-for-byte; (3) an unknown type with no override renders
  `index_default.md` and contains the `PluralLabel` and the `docz create
  <type>` line; (3-empty) an empty `PluralLabel` still yields a non-empty,
  well-formed header.
- **`internal/index`** — update tests for the `header`-string parameter;
  add a case proving an arbitrary header string is spliced above the table
  unchanged.
- **`internal/config`** — `ValidateType` resolves by prefix (`FW` and `fw`
  → `frameworks`); precedence (a colliding prefix never shadows a name);
  an unknown token still errors with the type list; a config with two
  enabled types sharing an `id_prefix` fails `Validate`.
- **`cmd` integration** (serial, `Runner` + `bytes.Buffer`): the
  end-to-end regression — `docz create FW "X"` in a repo whose `.docz.yaml`
  enables `frameworks` with `id_prefix: FW` and a
  `docs/templates/frameworks.md` body template creates
  `docs/frameworks/FW-0001-x.md` from that template **and** writes the
  index header; an `index_frameworks.md` override wins when present;
  `docz list fw` and `docz status set FW …` resolve.
- **Golden stability** — `go test ./... -update` must change **no** built-in
  golden files (tier 2 verbatim path is a no-op for the six built-ins).

## Migration / Rollout Plan

Purely additive, with one guardrail to call out:

- Built-in types resolve their index header through tier 2 exactly as
  before — identical bytes, no golden churn.
- Built-ins gain `id_prefix` shorthands (`RFC`, `ADR`, …) — new accepted
  inputs; nothing removed.
- **The duplicate-`id_prefix` rule is the only potentially-breaking
  change.** A config that previously declared two types with the same
  prefix would now fail `Validate`. That config was already ambiguous for
  ID generation, so surfacing it is a correctness win — note it in release
  notes.
- Existing repos with no custom types and no `index_*.md` overrides see no
  behavior change; hand-authored READMEs with markers still update in
  place, and a `docs/templates/index_<type>.md` override is picked up on
  the next `docz update`.
- Consider softening the `non-built-in type %q (typo?)` warning now that
  custom types are first-class (Open Questions §8).
- Ships in a normal minor release; no config migration.

## Open Questions

### 1. Where should index-header resolution live?

- **a (recommended).** Lift resolution into `internal/template`
  (`ResolveIndexHeader`) and pass the resolved `header` string into
  `index.UpdateReadme`/`DryRunReadme`, dropping index's `template`
  dependency. Cleanest layering; index becomes a pure splicer.
- **b.** Keep `internal/index` calling into `internal/template`, swapping
  `EmbeddedIndexHeader` for `ResolveIndexHeader` and threading `docsDir` +
  label through index's signatures. Smaller diff, retains the coupling.
- **c.** Resolve in `cmd` only, inlining the tiers at each call site.
  Rejected — duplicates logic across `update.go` and `init.go`.
- **d.** Other.

### 2. What should the generic fallback header contain?

- **a (recommended).** Ship an embedded `index_default.md` rendered with
  `text/template` using `PluralLabel`/`TypeName` — a titled, type-aware
  header with a correct `docz create <type>` example.
- **b.** A static, type-agnostic header (no interpolation) — simplest, but
  the heading can't name the type.
- **c.** No fallback: require a `docs/templates/index_<type>.md` override
  and fail with a help message pointing at that path.
- **d.** Other.

### 3. How to label a custom type that omits `plural_label`?

- **a (recommended).** Derive a Title-cased label from the canonical type
  name (`frameworks` → `Frameworks`) when `PluralLabel` is empty.
- **b.** Use the bare type name verbatim (`frameworks`).
- **c.** Treat an empty `plural_label` for a custom type as a `Validate`
  warning nudging the user to set one.
- **d.** Other.

### 4. Should built-ins also accept `id_prefix` shorthands?

- **a (recommended).** Yes — uniform behavior; `docz create RFC "…"` works
  alongside `docz create rfc "…"`. Simplest mental model.
- **b.** Restrict prefix resolution to non-built-in (custom) types only.
- **c.** Gate prefix resolution behind a config flag.
- **d.** Other.

### 5. Duplicate `id_prefix` — hard error or warning?

- **a (recommended).** Hard error in `Validate` — the config is ambiguous
  for both invocation and ID generation; fail fast at startup.
- **b.** Warning + first-declared-wins for prefix resolution.
- **c.** Warning + refuse *prefix* resolution for the collided prefix but
  still allow name/alias resolution.
- **d.** Other.

### 6. Add an explicit per-type `aliases:` field to `TypeConfig`?

- **a (recommended).** Defer — implicit `id_prefix` covers the requested
  shorthand; arbitrary aliases can be added later as tier 2.5 without
  breaking this design.
- **b.** Add `aliases: [fw, …]` now so custom types declare shorthands like
  built-ins do.
- **c.** Add it but mark it experimental/undocumented.
- **d.** Other.

### 7. Add an explicit per-type `index_template:` config key?

- **a (recommended).** Not now. The on-disk convention
  (`templates/index_<type>.md`) plus the generated fallback covers the
  need; an explicit path knob is YAGNI and additive later.
- **b.** Add `index_template:` to `TypeConfig` now, mirroring `template:`,
  as tier 0 of `ResolveIndexHeader`.
- **c.** Add it but mark it experimental/undocumented.
- **d.** Other.

### 8. Soften the `non-built-in type (typo?)` warning for declared custom types?

- **a (recommended).** Keep the warning — it still catches genuine typos
  and is cheap.
- **b.** Drop it now that custom types are first-class.
- **c.** Make it opt-in/quiet via a flag or a config opt-out list.
- **d.** Other.

## References

- Body-template resolver this design mirrors:
  `template.Resolve` — `internal/template/template.go:83`
- Wiki-index resolver (existing disk-override precedent):
  `template.ResolveWikiIndex` — `internal/template/template.go:119`
- Embedded-only header lookup (output-side gap):
  `template.EmbeddedIndexHeader` — `internal/template/embed.go:29`
- Failing index call sites: `internal/index/index.go:111`,
  `internal/index/index.go:153`, `cmd/init.go:117`
- Header consumer / heading from `PluralLabel`: `cmd/update.go:84`,
  `internal/index/index.go:158`
- Type-argument chokepoint (input-side gap):
  `Config.ValidateType` — `internal/config/config.go:196`;
  `ResolveTypeAlias` — `internal/config/config.go:230`
- Create flow that already honors `docs/templates/<type>.md`:
  `cmd/create.go:76`, `cmd/create.go:99-110`
- `TypeConfig` (`IDPrefix`, `Template`, `PluralLabel`):
  `internal/config/config.go:26`
- Built-in alias registry (`DocTypeDef.Aliases`):
  `internal/config/doctype.go:38`
- Typed `DocType`/`Status` boundary conventions: DESIGN-0004 §F
