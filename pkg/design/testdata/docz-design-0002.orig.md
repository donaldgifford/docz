---
id: DESIGN-0002
title: "Wiki Command for MkDocs TechDocs Integration"
status: Implemented
author: Donald Gifford
created: 2026-03-11
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0002: Wiki Command for MkDocs TechDocs Integration

**Status:** Implemented
**Author:** Donald Gifford
**Date:** 2026-03-11

  <!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
  - [Backstage TechDocs mkdocs.yml Format](#backstage-techdocs-mkdocsyml-format)
- [CLI Interface](#cli-interface)
  - [Command Structure](#command-structure)
  - [docz wiki init](#docz-wiki-init)
    - [Generated mkdocs.yml](#generated-mkdocsyml)
    - [Flags](#flags)
  - [docz wiki update](#docz-wiki-update)
    - [Flags](#flags-1)
  - [Integration with docz create](#integration-with-docz-create)
- [Nav Generation](#nav-generation)
  - [Directory Scanning](#directory-scanning)
  - [Nav Structure](#nav-structure)
  - [Directory Title Mapping](#directory-title-mapping)
  - [Ordering](#ordering)
  - [Handling docs/index.md](#handling-docsindexmd)
  - [Edge Cases](#edge-cases)
- [Configuration](#configuration)
  - [New Config Keys](#new-config-keys)
  - [WikiConfig Struct](#wikiconfig-struct)
- [Implementation](#implementation)
  - [Package Layout](#package-layout)
  - [Key Types](#key-types)
  - [MkDocs YAML Handling](#mkdocs-yaml-handling)
  - [Nav Serialization](#nav-serialization)
  - [Title Extraction from Documents](#title-extraction-from-documents)
- [Testing Strategy](#testing-strategy)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Overview

Add a `docz wiki` command group that generates and maintains a `mkdocs.yml`
file compatible with Backstage's TechDocs plugin. The `wiki update` command
scans the `docs/` directory and rebuilds the MkDocs `nav:` section to reflect
the current file structure — including docz-managed documents and any other
markdown files in the docs tree.

## Goals and Non-Goals

### Goals

- Generate a `mkdocs.yml` that works out of the box with `mkdocs serve` and
  Backstage TechDocs
- Automatically maintain the `nav:` section as documents are added or removed
- Include all markdown files in the docs directory, not just docz-managed types
- Integrate with the existing `docz create` workflow so the nav stays current
- Support a reasonable default nav structure with zero configuration

### Non-Goals

- Building or serving the MkDocs site (use `mkdocs serve` / `mkdocs build`)
- Managing MkDocs themes, plugins, or extensions beyond `techdocs-core`
- Supporting non-markdown content (images, PDFs are referenced but not tracked)
- Replacing MkDocs as the site generator

## Background

Backstage's TechDocs plugin reads a `mkdocs.yml` at the repo root to generate
documentation sites. The nav section must be maintained manually, which creates
drift — documents get created via `docz create` but the nav doesn't update.
Teams either forget to update the nav or maintain it by hand, leading to stale
or missing pages.

Since `docz` already knows the full structure of the docs directory (it manages
document types, scans for frontmatter, and maintains README indexes), it's in
the best position to keep the MkDocs nav in sync automatically.

### Backstage TechDocs `mkdocs.yml` Format

A typical TechDocs-compatible `mkdocs.yml`:

```yaml
site_name: My Service
site_description: Documentation for My Service

plugins:
  - techdocs-core

nav:
  - Home: index.md
  - RFCs:
      - Overview: rfc/README.md
      - "RFC-0001: API Rate Limiting": rfc/0001-api-rate-limiting.md
  - ADRs:
      - Overview: adr/README.md
      - "ADR-0001: Use PostgreSQL": adr/0001-use-postgresql.md
  - Architecture:
      - Overview: architecture/README.md
      - System Diagram: architecture/system-diagram.md
```

Key constraints:

- `plugins: [techdocs-core]` is required for Backstage
- Nav paths are relative to the `docs_dir` (defaults to `docs/`)
- Nav entries can be nested for grouping
- The title in nav can differ from the document title

## CLI Interface

### Command Structure

```
docz wiki
  init                    Create mkdocs.yml with TechDocs defaults
  update                  Rebuild the nav section from docs/ contents
```

### `docz wiki init`

Creates a `mkdocs.yml` at the repo root with sensible TechDocs defaults.
If the project hasn't been initialized with `docz init`, it runs `docz init`
automatically before creating the mkdocs file.

```bash
docz wiki init
# → Initialized docz (if needed)
# → Created mkdocs.yml

docz wiki init --site-name "My Service"
# → Created mkdocs.yml with site_name: My Service

docz wiki init --force
# → Overwrote existing mkdocs.yml
```

If `mkdocs.yml` already exists and `--force` is not passed, the command fails
with a clear error message.

#### Generated `mkdocs.yml`

```yaml
site_name: <repo-name>
site_description: Documentation for <repo-name>

plugins:
  - techdocs-core

nav:
  - Home: index.md
  # nav is populated by `docz wiki update`
```

The `site_name` defaults to the directory name of the repository root (e.g.,
`my-service`). It can be overridden with `--site-name`.

#### Flags

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing `mkdocs.yml` |
| `--site-name <name>` | Set `site_name` (default: repo directory name) |
| `--site-description <desc>` | Set `site_description` |

### `docz wiki update`

Scans the `docs/` directory and regenerates the `nav:` section of `mkdocs.yml`.
All other fields (`site_name`, `plugins`, `theme`, etc.) are preserved.

```bash
docz wiki update
# → Updated nav in mkdocs.yml (23 pages)

docz wiki update --dry-run
# → Prints what the nav would look like without writing
```

#### Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Print the generated nav to stdout without modifying `mkdocs.yml` |

### Integration with `docz create`

When `docz create` runs and auto-updates the type index, it should also call
`docz wiki update` if a `mkdocs.yml` exists at the repo root. This keeps the
nav current without requiring a separate command.

This is controlled by a new config key:

```yaml
wiki:
  auto_update: true   # default: true if mkdocs.yml exists
```

## Nav Generation

### Directory Scanning

`wiki update` walks the docs directory recursively. For each directory and
file it encounters:

1. **Directories** become nav groups. The group title is derived from:
   - The directory name, title-cased (e.g., `rfc` → `RFCs`, `design` → `Design`)
   - Docz-managed types use their configured display name

2. **`README.md` / `index.md`** files in a directory become the "Overview"
   entry for that group

3. **Docz documents** (`NNNN-*.md`) use their frontmatter title as the nav
   entry title (e.g., `"RFC-0001: API Rate Limiting"`)

4. **Other markdown files** use their filename converted to title case, or
   their first H1 heading if present

5. **Non-markdown files** are ignored

### Nav Structure

The nav is organized by top-level directories under `docs/`:

```yaml
nav:
  - Home: index.md
  - RFCs:
      - Overview: rfc/README.md
      - "RFC-0001: API Rate Limiting": rfc/0001-api-rate-limiting.md
      - "RFC-0002: Event Sourcing": rfc/0002-event-sourcing.md
  - ADRs:
      - Overview: adr/README.md
      - "ADR-0001: Use PostgreSQL": adr/0001-use-postgresql.md
  - Design:
      - Overview: design/README.md
      - "DESIGN-0001: Auth Flow": design/0001-auth-flow.md
  - Implementation Plans:
      - Overview: impl/README.md
      - "IMPL-0001: Auth Implementation": impl/0001-auth-implementation.md
  - Plans:
      - Overview: plan/README.md
      - "PLAN-0001: Q1 Roadmap": plan/0001-q1-roadmap.md
  - Investigations:
      - Overview: investigation/README.md
      - "INV-0001: pgvector Performance": investigation/0001-pgvector-performance.md
  - Architecture:
      - System Overview: architecture/system-overview.md
      - Deployment: architecture/deployment.md
```

### Directory Title Mapping

Docz-managed type directories use human-friendly pluralized names:

| Directory | Nav Title |
|-----------|-----------|
| `rfc` | RFCs |
| `adr` | ADRs |
| `design` | Design |
| `impl` | Implementation Plans |
| `plan` | Plans |
| `investigation` | Investigations |

Non-docz directories use title-cased directory names. A directory named
`architecture` becomes `Architecture`, `getting-started` becomes
`Getting Started`.

These defaults can be overridden in config via `wiki.nav_titles`.

### Ordering

Within each section:

1. `README.md` / `index.md` always comes first (as "Overview")
2. Docz documents are sorted by their numeric ID (ascending)
3. Other files are sorted alphabetically

Top-level sections on `wiki init` are ordered alphabetically:

1. `index.md` (Home) always first
2. All other sections sorted alphabetically

On `wiki update`, existing section order is preserved. New sections are
appended alphabetically at the end.

### Handling `docs/index.md`

If `docs/index.md` does not exist, `wiki init` creates a minimal one:

```markdown
# <site_name>

Welcome to the documentation for <site_name>.

## Document Types

- [RFCs](rfc/README.md) — High-level proposals
- [ADRs](adr/README.md) — Architecture decisions
- [Design](design/README.md) — Detailed designs
- [Implementation](impl/README.md) — Implementation plans
- [Plans](plan/README.md) — Planning documents
- [Investigations](investigation/README.md) — Research spikes
```

### Edge Cases

- **Empty directories** — Skipped in the nav (no group created)
- **Deeply nested directories** — Supported at arbitrary depth
- **Files without frontmatter** — Use filename-derived titles
- **`mkdocs.yml` missing** — `wiki update` errors with a message to run
  `wiki init` first
- **`docs/` directory missing** — `wiki init` runs `docz init` automatically;
  `wiki update` errors with a message to run `wiki init` first
- **Excluded directories** — `templates/` and `examples/` are excluded by
  default (configurable)

## Configuration

### New Config Keys

```yaml
wiki:
  auto_update: true           # auto-run wiki update after docz create
  mkdocs_path: mkdocs.yml     # path to mkdocs.yml (relative to repo root)
  exclude:                    # directories to exclude from nav
    - templates
    - examples
  nav_titles:                 # override directory-to-nav-title mapping
    rfc: "Request for Comments"
    architecture: "System Architecture"
```

### `WikiConfig` Struct

```go
type WikiConfig struct {
    AutoUpdate bool              `mapstructure:"auto_update" yaml:"auto_update"`
    MkDocsPath string            `mapstructure:"mkdocs_path" yaml:"mkdocs_path"`
    Exclude    []string          `mapstructure:"exclude"     yaml:"exclude"`
    NavTitles  map[string]string `mapstructure:"nav_titles"  yaml:"nav_titles"`
}
```

## Implementation

### Package Layout

```
internal/
  wiki/
    wiki.go       # NavEntry, ScanDocs(), BuildNav()
    mkdocs.go     # ReadMkDocs(), WriteMkDocs(), UpdateNav()
    titles.go     # Directory title mapping, frontmatter title extraction
cmd/
  wiki.go         # docz wiki init, docz wiki update
```

### Key Types

```go
// NavEntry represents a single entry in the MkDocs nav.
type NavEntry struct {
    Title    string
    Path     string     // relative to docs_dir, e.g. "rfc/0001-my-rfc.md"
    Children []NavEntry // non-nil for directory groups
}
```

### MkDocs YAML Handling

Reading and writing `mkdocs.yml` must preserve fields that `docz` doesn't
manage (theme, extra plugins, markdown_extensions, etc.). The approach:

1. Read the file as a `map[string]interface{}`
2. Replace only the `nav` key
3. Write back the full map, preserving key order where possible

This avoids defining a struct for every possible MkDocs field. The `nav` key
is the only one `docz wiki update` modifies.

### Nav Serialization

MkDocs nav entries are serialized as a list of single-key maps in YAML:

```yaml
nav:
  - Home: index.md
  - RFCs:
      - Overview: rfc/README.md
```

In Go, this is `[]interface{}` where each element is either:

- `map[string]string` for a leaf: `{"Home": "index.md"}`
- `map[string][]interface{}` for a group: `{"RFCs": [...]}`

### Title Extraction from Documents

For docz-managed documents, read frontmatter and construct the title as
`"<PREFIX>-<ID>: <Title>"` (e.g., `"RFC-0001: API Rate Limiting"`).

For non-docz markdown files, use a two-pass approach:

1. Try to read the first `# Heading` from the file
2. Fall back to converting the filename to title case
   (e.g., `system-overview.md` → `System Overview`)

## Testing Strategy

- **Unit tests** for nav generation from a mock directory tree
- **Unit tests** for title extraction (frontmatter, H1, filename fallback)
- **Unit tests** for MkDocs YAML read/write (preserve unknown fields)
- **Integration tests** for `wiki init` (creates mkdocs.yml and index.md)
- **Integration tests** for `wiki update` (scans real directories, writes nav)
- **Golden file tests** for nav output given a known directory structure

## Decisions

1. **Ordering behavior** — `wiki init` generates the nav in alphabetical order.
   `wiki update` preserves the existing section order in `mkdocs.yml` and
   appends new sections alphabetically at the end. This lets users manually
   reorder sections and have that order survive updates.

2. **`wiki init` runs `docz init`** — If the project hasn't been initialized
   with `docz init`, `wiki init` runs it automatically rather than erroring.
   This reduces friction for new setups.

3. **Arbitrary nesting depth** — Deeply nested non-docz directories are
   supported at arbitrary depth. No flattening or depth limit is imposed.

4. **Config-based exclusion list** — The `wiki.exclude` key in `.docz.yaml`
   controls which directories are excluded from the nav. Defaults to
   `[templates, examples]`.

5. **Nav titles for `impl` and `investigation`** — `impl` maps to
   "Implementation Plans" and `investigation` maps to "Investigations".

## References

- [Backstage TechDocs](https://backstage.io/docs/features/techdocs/)
- [MkDocs Configuration](https://www.mkdocs.org/user-guide/configuration/)
- [MkDocs Nav](https://www.mkdocs.org/user-guide/writing-your-docs/#configure-pages-and-navigation)
- DESIGN-0001: docz CLI Tool
