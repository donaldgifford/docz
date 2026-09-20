---
id: DESIGN-0002
title: "Wiki nav generation"
status: Approved
author: Parity Fixture
created: 2026-03-04
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0002: Wiki nav generation

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. Should nested type directories collapse into a single nav group?](#1-should-nested-type-directories-collapse-into-a-single-nav-group)
  - [2. How should title collisions across pages be reported?](#2-how-should-title-collisions-across-pages-be-reported)
- [References](#references)
<!--toc:end-->

<!--docz:overview:start-->
## Overview

`docz wiki` walks the configured docs directory and builds the `nav:`
block of `mkdocs.yml` so contributors never hand-edit that list. The
generator groups pages by their type directory, orders entries within
each group, and resolves a human title for every page it finds, even
when the page itself has no frontmatter at all.

**Status:** this behavior already ships behind `docz wiki`; this
document records the design after the fact so future changes have a
baseline to diff against.
<!--docz:overview:end-->

## Goals and Non-Goals

<!--docz:goals:start-->
### Goals

- Build a complete MkDocs `nav:` tree from the configured `docs_dir`
  without requiring manual upkeep.
- Resolve a readable title for every page, even when frontmatter is
  missing or incomplete.
  - Fall back through frontmatter title, then the first H1, then the
    filename, in that order.
    - Never fail the whole wiki build just because one page lacks a
      title; degrade to the filename and keep going.
- Keep generated output stable across repeated runs so the diff stays
  small when nothing meaningful changed.
<!--docz:goals:end-->

<!--docz:non-goals:start-->
### Non-Goals

- Rewriting the whole `mkdocs.yml` file. Only the `nav:` block is
  owned by the generator; theme and plugin config are left alone.
- Supporting non-MkDocs site generators, such as Hugo or Docusaurus,
  in this iteration.
- Reordering or renaming files on disk to make titles prettier.
<!--docz:non-goals:end-->

<!--docz:background:start-->
## Background

Contributors used to edit the `nav:` block by hand every time a
document was added, renamed, or reclassified. That worked while the
docs tree was small, but the list drifted constantly once `docz
create` started running from CI and from local branches at the same
time, and stale entries quietly pointed at pages that no longer
existed.

The README index table generator already solves a similar drift
problem for the per-type listing pages, so this design reuses that
generator's approach where the shapes line up: read the tree once,
derive a small in-memory model, and splice the output back in rather
than asking anyone to maintain it by hand.
<!--docz:background:end-->

<!--docz:detailed-design:start-->
## Detailed Design

The generator makes two passes over the docs directory. The first
pass walks every enabled type directory and every top-level page, and
builds a flat list of candidate nav entries. The second pass resolves
a title for each entry and assembles the nested tree that `mkdocs.yml`
expects.

Title resolution is the part most likely to surprise someone reading
the output, so it is deliberately linear and cheap to reason about:

```go
func resolveTitle(page Page) string {
    if page.Frontmatter.Title != "" {
        return page.Frontmatter.Title
    }
    if h1 := docparse.Title(page.Content); h1 != "" {
        return h1
    }
    return FilenameTitle(page.Path)
}
```

Frontmatter wins when it is present because it is the most explicit
signal an author can give. The first H1 comes next since most pages
already have one, and it is usually a better title than the raw
filename. The filename is the last resort, cleaned up by replacing
dashes and underscores with spaces and title-casing the result, so a
page never ends up with no nav label at all.
<!--docz:detailed-design:end-->

<!--docz:api-changes:start-->
## API / Interface Changes

No new CLI flags are introduced. `docz wiki` keeps its existing
signature, and the title resolver described above is purely internal:
callers only ever see the resolved string, never which of the three
sources produced it. The only externally visible change is that pages
without frontmatter now show up in the nav at all, instead of being
silently skipped as they were before this generator existed.
<!--docz:api-changes:end-->

<!--docz:data-model:start-->
## Data Model

The generator's in-memory representation is a small tree. Nothing here
is persisted; it exists only for the duration of a single `docz wiki`
run and is discarded once `mkdocs.yml` has been written.

| Field      | Type         | Notes                                        |
| ---------- | ------------ | --------------------------------------------- |
| `Title`    | `string`     | Resolved via frontmatter, H1, then filename   |
| `Path`     | `string`     | Repo-relative path to the rendered page       |
| `Children` | `[]NavEntry` | Populated for type directories with subpages  |
<!--docz:data-model:end-->

<!--docz:testing:start-->
## Testing Strategy

Title resolution precedence is covered by table-driven unit tests that
pin the three fallback tiers independently: frontmatter present,
frontmatter absent but an H1 present, and neither present. Golden
fixture tests assemble a small synthetic docs tree and assert the
generated `nav:` block byte for byte, so a change in ordering or
indentation is caught immediately rather than surfacing as a review
comment on a real `mkdocs.yml` diff.
<!--docz:testing:end-->

<!--docz:rollout:start-->
## Migration / Rollout Plan

The generator only ever adds or updates the `nav:` block it owns, so
existing repos do not need a migration step. On first run against a
repo whose `mkdocs.yml` predates this feature, the block is inserted
once and left under normal splice management afterward.
<!--docz:rollout:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Should nested type directories collapse into a single nav group?

- a. Yes, flatten one level so the nav stays shallow even as more
  custom types are added. *(recommendation)*
- b. No, mirror the on-disk directory structure exactly.
- c. Make it configurable per type via `.docz.yaml`.

### 2. How should title collisions across pages be reported?

- a. Warn during `docz wiki` and continue with the filename-derived
  title for the later entry. *(recommendation)*
- b. Fail the build so authors must resolve every collision by hand
  before the nav can regenerate.
- c. Silently dedupe by appending a numeric suffix to the title.
<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [MkDocs navigation configuration](https://www.mkdocs.org/user-guide/configuration/#nav)
- [Custom Document Type Support](0006-custom-document-type-support.md)
- [docz issue tracker](https://github.com/donaldgifford/docz/issues)
<!--docz:references:end-->
