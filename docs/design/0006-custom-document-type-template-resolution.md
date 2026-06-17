---
id: DESIGN-0006
title: "Custom Document Type Template Resolution"
status: Draft
author: Donald Gifford
created: 2026-06-17
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0006: Custom Document Type Template Resolution

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-06-17

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [New resolver: template.ResolveIndexHeader](#new-resolver-templateresolveindexheader)
  - [Decoupling internal/index from internal/template](#decoupling-internalindex-from-internaltemplate)
  - [Wiring the callers](#wiring-the-callers)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. Where should index-header resolution live?](#1-where-should-index-header-resolution-live)
  - [2. What should the generic fallback header contain?](#2-what-should-the-generic-fallback-header-contain)
  - [3. Add an explicit per-type index_template: config key?](#3-add-an-explicit-per-type-indextemplate-config-key)
  - [4. How to label a custom type that omits plural_label?](#4-how-to-label-a-custom-type-that-omits-plurallabel)
- [References](#references)
<!--toc:end-->

## Overview

A user who enables a **custom document type** (e.g. `frameworks`) in
`.docz.yaml` can create documents, but the first `docz update` or
`docz init` fails with `no embedded index header for type "frameworks"`.
This design closes the gap by giving index-header resolution the same
three-tier disk-override → embedded fallback behavior that body templates
already have, so a custom type is first-class without the user having to
fork the tool or hand-author every index README.

## Goals and Non-Goals

### Goals

- Make `docz update` and `docz init` succeed for any enabled type,
  built-in or custom, without an embedded `index_<type>.md`.
- Let a user override an index header on disk at
  `<docs_dir>/templates/index_<type>.md`, mirroring the existing
  body-template override at `<docs_dir>/templates/<type>.md`.
- Provide a sensible generated header for a custom type that has no
  on-disk override, using the type's configured `plural_label`.
- Preserve byte-for-byte output for the six built-in types — no golden
  churn for `rfc`/`adr`/`design`/`impl`/`plan`/`investigation`.

### Non-Goals

- Adding a per-type `index_template:` config key (an explicit path knob).
  The on-disk convention plus a generated fallback covers the need; an
  explicit path field is deferred (see Open Questions §3).
- Changing how *body* templates resolve — `template.Resolve` already
  handles custom types correctly and is the model this design copies.
- Validating or templating the contents of user-authored index headers
  (a disk override is emitted verbatim, exactly as today).
- Auto-registering or discovering custom types from the filesystem; the
  type must still be declared in `.docz.yaml`.

## Background

docz resolves two distinct template artifacts when it materializes a
document type:

1. **The body template** — the scaffold for a new document. Resolved by
   `template.Resolve(docType, configPath, docsDir)`
   (`internal/template/template.go:83`) through three tiers:
   1. explicit `configPath` (the type's `template:` field), then
   2. on-disk override `<docsDir>/templates/<docType>.md`, then
   3. the embedded default `templates/<docType>.md`.

2. **The index header** — the prose that sits above the auto-generated
   table in each type's `README.md`. Resolved by
   `template.EmbeddedIndexHeader(docType)` (`internal/template/embed.go:29`),
   which reads `templates/index_<docType>.md` from the **embedded FS only**.
   There is no `configPath` tier and no on-disk tier.

That asymmetry is the bug. The embedded FS ships `index_<type>.md` for the
six built-ins and nothing else, so for any custom type tier (2)'s single
lookup misses and the call returns
`no embedded index header for type %q`. The error surfaces at three call
sites:

- `internal/index/index.go:153` — `createNewReadme` (the `docz update`
  path that first creates a type's README).
- `internal/index/index.go:111` — `DryRunReadme` (the `docz update
  --dry-run` create branch).
- `cmd/init.go:117` — `writeIndexReadme` (the `docz init` scaffolding
  path).

A user hitting this reasonably expects the same disk convention the body
template already honors: drop a file under `docs/templates/` and have docz
pick it up. Today that works for `docs/templates/frameworks.md` (body) but
not `docs/templates/index_frameworks.md` (header).

Two facts shape the design:

- **Index headers are static markdown, not `text/template`.** The
  built-in `index_*.md` files contain no `{{ }}` actions; they are read
  verbatim and concatenated with the marker block and table
  (`index.go:158`). Rendering existing files through a template engine
  would risk reinterpreting a literal `{{` in user prose, so verbatim
  emission must stay the default.
- **A custom type already carries a display label.** `TypeConfig` has
  `PluralLabel` (`internal/config/config.go:34`), and `cmd/update.go:84`
  already builds the table heading from `tc.PluralLabel`. A generated
  fallback header can use that label to produce a non-generic title.

## Detailed Design

### New resolver: `template.ResolveIndexHeader`

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

1. **On-disk override.** `filepath.Join(docsDir, config.TemplatesDir,
   "index_"+docType+".md")`. If readable, return its bytes verbatim —
   identical treatment to `Resolve` tier 2.
2. **Embedded type-specific.** `templates/index_<docType>.md` via the
   existing `templateFS`. Hits for the six built-ins; returned verbatim.
   This is exactly today's `EmbeddedIndexHeader` behavior, so built-in
   output is unchanged.
3. **Embedded generic fallback.** A new embedded
   `templates/index_default.md`, parsed with `text/template` and executed
   with `data`. Only this tier is rendered, so the `{{ }}`-reinterpretation
   risk is confined to a file docz ships and controls.

Tiers 1 and 2 keep index headers static; tier 3 is the only rendered path
and only runs when neither a disk override nor an embedded type-specific
header exists — i.e. exclusively for custom types with no override.

### Decoupling `internal/index` from `internal/template`

Today `internal/index` reaches into `internal/template` for the header
(`index.go:111`, `:153`). That cross-package call is the only reason index
knows about templates at all, and it is what fails for custom types.
Rather than thread `docsDir` + label *into* index, **lift header
resolution out of index** so the package becomes a pure marker-splicer:

- Change `UpdateReadme(readmePath, typeName, tableContent string)` →
  `UpdateReadme(readmePath, header, tableContent string)`.
- Change `DryRunReadme(readmePath, typeName, tableContent string)` →
  `DryRunReadme(readmePath, header, tableContent string)`.
- `createNewReadme` and the `DryRunReadme` create branch use the passed-in
  `header` directly and drop the `doctemplate` import.

The caller resolves the header once and passes the string in. This matches
the layering already documented in `CLAUDE.md` — *"Scanning lives in
`internal/document`; this package depends on `document.DocEntry` for its
input type"* — index should not also depend on `internal/template`. All
template resolution (body, wiki index, and now index header) then lives in
one package.

### Wiring the callers

`cmd/update.go` `updateType` already holds `tc` (the `TypeConfig`) and the
resolved `docsDir`, so it resolves the header before calling index:

```go
header, err := template.ResolveIndexHeader(typeName, r.Cfg.DocsDir, template.IndexHeaderData{
    TypeName:    typeName,
    PluralLabel: indexLabel(tc, typeName),
})
if err != nil {
    return fmt.Errorf("resolving index header for %s: %w", typeName, err)
}
// ... dryRun ? index.DryRunReadme(readmePath, header, tableContent)
//            : index.UpdateReadme(readmePath, header, tableContent)
```

`cmd/init.go` `writeIndexReadme` swaps its `doctemplate.EmbeddedIndexHeader`
call for the same `ResolveIndexHeader`, looking up `r.Cfg.Types[typeName]`
for the label.

`indexLabel` is a tiny cmd-level helper: return `tc.PluralLabel` when set,
else a Title-cased form of `typeName` so a custom type that omits
`plural_label` still gets a readable heading (see Open Questions §4).

`EmbeddedIndexHeader` is removed once these three call sites migrate; it
has no other callers.

## API / Interface Changes

- **`internal/template`** (new public surface):
  - `func ResolveIndexHeader(docType, docsDir string, data IndexHeaderData) (string, error)`
  - `type IndexHeaderData struct { TypeName, PluralLabel string }`
  - `EmbeddedIndexHeader` is **removed** (internal package; the three
    callers migrate in the same change).
- **`internal/index`** (signature change, internal package):
  - `UpdateReadme(readmePath, header, tableContent string)` — second
    parameter changes meaning from `typeName` to the resolved `header`.
  - `DryRunReadme(readmePath, header, tableContent string)` — same.
  - The package drops its `internal/template` import.
- **CLI / config:** no user-facing flag or `.docz.yaml` schema change. The
  only new *behavior* is that `<docs_dir>/templates/index_<type>.md` is now
  honored as an override, and custom types no longer error.

## Data Model

- **New embedded template:** `internal/template/templates/index_default.md`,
  a `text/template` whose actions reference `IndexHeaderData`. Sketch:

  ```markdown
  # {{ .PluralLabel }}

  This directory contains {{ .PluralLabel }}.

  ## Creating a New Document

  ```bash
  docz create {{ .TypeName }} "Your Title"
  ```
  ```

  It is the only index header rendered through the template engine; the
  six built-in `index_*.md` files remain static and are emitted verbatim.

- **New on-disk override path (convention, not schema):**
  `<docs_dir>/templates/index_<type>.md`. Parallels the existing body
  override `<docs_dir>/templates/<type>.md`. No config key references it;
  it is discovered by path like `Resolve` tier 2 and `ResolveWikiIndex`.

- No frontmatter, README marker, or persisted-state changes.

## Testing Strategy

- **`internal/template` unit tests** for `ResolveIndexHeader`:
  - tier 1 — a file under `t.TempDir()/templates/index_x.md` is returned
    verbatim (including any `{{` that must *not* be rendered);
  - tier 2 — a built-in type (`rfc`) returns the embedded header byte-for-byte
    (guards against silently rendering static headers);
  - tier 3 — an unknown type with no override renders `index_default.md`
    and the output contains the `PluralLabel` and the
    `docz create <type>` line;
  - tier-3 empty label — `IndexHeaderData{PluralLabel: ""}` still produces
    a non-empty, well-formed header.
- **`internal/index` tests** updated for the new `header`-string parameter;
  add a case proving an arbitrary header string is spliced above the table
  unchanged (the package no longer cares where the header came from).
- **`cmd` integration tests** (serial, `Runner` + `bytes.Buffer`):
  - `docz update` against a repo whose `.docz.yaml` enables a custom type
    `frameworks` creates `docs/frameworks/README.md` with a generated
    header — the regression test for the reported bug;
  - the same with a `docs/templates/index_frameworks.md` override present
    asserts the override wins;
  - `docz init` scaffolds a custom type's README without error.
- **Golden stability:** re-run `go test ./... -update` and confirm **no**
  built-in golden files change — tier 2's verbatim path must be a no-op
  for the six built-ins.

## Migration / Rollout Plan

Purely additive and backward-compatible:

- Built-in types resolve through tier 2 exactly as before — identical
  bytes, no golden churn.
- Existing repos with no custom types and no `index_*.md` overrides see no
  behavior change.
- The fix is what *unblocks* custom types, so there is nothing to migrate;
  users who previously worked around the error (committing a README by
  hand) keep working — a hand-authored README with markers still updates
  in place, and a `docs/templates/index_<type>.md` override is picked up
  on the next `docz update`.
- Ships in a normal minor release; no deprecation, no config migration.

## Open Questions

### 1. Where should index-header resolution live?

How do we close the cross-package call that fails for custom types?

- **a (recommended).** Lift resolution into `internal/template`
  (`ResolveIndexHeader`) and pass the resolved `header` string into
  `index.UpdateReadme`/`DryRunReadme`, dropping index's `template`
  dependency. Cleanest layering; index becomes a pure splicer; all
  resolution lives in one package.
- **b.** Keep `internal/index` calling into `internal/template`, but swap
  `EmbeddedIndexHeader` for `ResolveIndexHeader` and thread `docsDir` +
  label through the existing index signatures. Smaller diff, but retains
  the cross-package coupling and widens index's API with template concerns.
- **c.** Resolve in `cmd` only (no new `template` function); inline the
  three tiers at each call site. Rejected — duplicates logic across
  `update.go` and `init.go`.
- **d.** Other.

### 2. What should the generic fallback header contain?

For a custom type with no embedded header and no on-disk override:

- **a (recommended).** Ship an embedded `index_default.md` rendered with
  `text/template` using the type's `PluralLabel`/`TypeName` — yields a
  titled, type-aware header with a correct `docz create <type>` example.
- **b.** Emit a static, type-agnostic header (no interpolation) — simplest,
  but the heading can't name the type and the create example is generic.
- **c.** No fallback: require a `docs/templates/index_<type>.md` override
  and fail with a help message pointing at that path. Most explicit, but
  custom types don't "just work" out of the box.
- **d.** Other.

### 3. Add an explicit per-type `index_template:` config key?

Body templates have a `template:` path field; index headers do not.

- **a (recommended).** Not now. The on-disk convention
  (`templates/index_<type>.md`) plus the generated fallback covers the
  need; an explicit path knob is YAGNI and can be added later without
  breaking this design.
- **b.** Add `index_template:` to `TypeConfig` now, mirroring `template:`,
  and make it tier 0 of `ResolveIndexHeader`. Full symmetry with body
  templates at the cost of more surface area to document and test.
- **c.** Add it but mark it experimental/undocumented.
- **d.** Other.

### 4. How to label a custom type that omits `plural_label`?

The fallback header's heading comes from a display label.

- **a (recommended).** Derive a Title-cased label from the canonical type
  name (`frameworks` → `Frameworks`) when `PluralLabel` is empty, so the
  header is always readable.
- **b.** Use the bare type name verbatim (`frameworks`).
- **c.** Treat an empty `plural_label` for a custom type as a config
  validation warning, nudging the user to set one.
- **d.** Other.

## References

- Body-template resolver this design mirrors:
  `template.Resolve` — `internal/template/template.go:83`
- Wiki-index resolver (existing disk-override precedent):
  `template.ResolveWikiIndex` — `internal/template/template.go:119`
- Embedded-only header lookup (the gap):
  `template.EmbeddedIndexHeader` — `internal/template/embed.go:29`
- Failing call sites: `internal/index/index.go:111`,
  `internal/index/index.go:153`, `cmd/init.go:117`
- Header consumer / table heading from `PluralLabel`:
  `cmd/update.go:84`, `internal/index/index.go:158`
- `TypeConfig` (`PluralLabel`, `Template`):
  `internal/config/config.go:26`
- Typed `DocType`/`Status` boundary conventions: DESIGN-0004 §F
