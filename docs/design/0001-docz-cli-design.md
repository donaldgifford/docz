---
id: DESIGN-0001
title: "docz CLI Tool"
status: Implemented
author: Donald Gifford
created: 2026-02-22
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0001: docz CLI Tool

## Problem Statement

Managing standardized documentation across repositories is painful. The previous
approach used bash scripts (`create-adr.sh`, `create-rfc.sh`, `update-*-readme.sh`)
that had to be copied into every repository's `tools/` directory. This created
several problems:

1. **Duplication** -- Every repo needed its own copy of the scripts and templates.
   Template changes required updating every repo individually.
2. **Fragile parsing** -- Bash `awk`/`sed`/`grep` chains for extracting YAML
   frontmatter and status fields are brittle and hard to maintain.
3. **Limited extensibility** -- Adding a new document type (e.g., DESIGN or IMPL)
   meant writing another pair of create/update scripts with largely duplicated
   logic.
4. **No Claude Code integration** -- The scripts couldn't serve as Claude Code
   skills, so AI-assisted document creation required manually copying template
   contents into prompts.

## Proposed Solution

Build `docz` as a single Go binary that:

- Ships with **embedded default templates** for each document type (RFC, ADR,
  DESIGN, IMPL) so repos need zero boilerplate files.
- Allows **per-repo template overrides** via a `.docz.yaml` config file and/or a
  local `docs/templates/` directory.
- **Creates documents** from templates with auto-incremented IDs, placeholder
  substitution, and YAML frontmatter.
- **Generates and updates index pages** (`README.md`) in each type's directory
  with a table of documents, their statuses, and links.
- **Exports templates** for use by Claude Code skills, enabling AI-assisted
  document authoring that calls `docz` under the hood.

## Document Types

| Type | Directory | Purpose | Status Values |
|------|-----------|---------|---------------|
| RFC | `docs/rfc/` | High-level proposals for major features or system changes | Draft, Proposed, Accepted, Rejected, Superseded |
| ADR | `docs/adr/` | Architecture decision records for specific technical decisions | Proposed, Accepted, Deprecated, Superseded |
| DESIGN | `docs/design/` | Detailed design documents for feature implementation | Draft, In Review, Approved, Implemented, Abandoned |
| IMPL | `docs/impl/` | Implementation plans with concrete tasks and milestones | Draft, In Progress, Completed, Paused, Cancelled |

## CLI Interface

### Command Structure

```
docz
  init                          Initialize docz in current repo
  create <type> <title>         Create a new document
  update [type]                 Update index/README for type (or all)
  list [type]                   List documents, optionally filtered by type
  template <subcommand>         Template management
    show <type>                 Print the template for a type to stdout
    export <type> [path]        Export a template to a file
    override <type>             Copy default template to local overrides dir
  config                        Show resolved configuration
  version                       Print version information

Types: rfc, adr, design, impl
```

### Examples

```bash
# Initialize docz in a repo (creates .docz.yaml, dirs, and README indexes)
docz init

# Re-initialize, overwriting existing README index files
docz init --force

# Create documents
docz create rfc "API Rate Limiting Strategy"
docz create adr "Use PostgreSQL for Primary Storage"
docz create design "User Authentication Flow"
docz create impl "Migrate to gRPC"

# Update index pages
docz update          # Update all type indexes
docz update rfc      # Update only RFC index

# List documents
docz list            # List all documents across all types
docz list adr        # List only ADRs
docz list --status accepted  # Filter by status

# Template operations
docz template show rfc           # Print RFC template to stdout
docz template export design .    # Export design template to current dir
```

### Flags

```
Global flags:
  --config string    Config file (default: .docz.yaml in repo root)
  --docs-dir string  Base documentation directory (default: docs)
  --verbose          Enable verbose output

init flags:
  --force            Overwrite existing README index files

create flags:
  --status string    Initial status (default varies by type)
  --author string    Document author (default: git user.name)
  --no-update        Skip automatic index update after creation

list flags:
  --status string    Filter by status
  --format string    Output format: table, json, csv (default: table)

update flags:
  --dry-run          Show what would change without writing
```

## Configuration

### `.docz.yaml`

Configuration is loaded from two locations, with repo root taking precedence:

1. **Repo root** `.docz.yaml` -- Per-repo configuration. Created by
   `docz init`. Values here override global config.
2. **Home directory** `~/.docz.yaml` -- Global defaults (e.g., author name).
   Applied when not overridden by repo config.

```yaml
# .docz.yaml
docs_dir: docs          # Base directory for all documentation

types:
  rfc:
    enabled: true
    dir: rfc             # Relative to docs_dir
    template: ""         # Empty = use embedded default; path = override
    id_prefix: "RFC"     # Prefix shown in index table
    id_width: 4          # Zero-pad width (0001, 0002, ...)
    statuses:            # Allowed status values (first = default)
      - Draft
      - Proposed
      - Accepted
      - Rejected
      - Superseded
    status_field: "status"  # YAML frontmatter field name for status

  adr:
    enabled: true
    dir: adr
    template: ""
    id_prefix: "ADR"
    id_width: 4
    statuses:
      - Proposed
      - Accepted
      - Deprecated
      - Superseded
    status_field: "status"

  design:
    enabled: true
    dir: design
    template: ""
    id_prefix: "DESIGN"
    id_width: 4
    statuses:
      - Draft
      - In Review
      - Approved
      - Implemented
      - Abandoned
    status_field: "status"

  impl:
    enabled: true
    dir: impl
    template: ""
    id_prefix: "IMPL"
    id_width: 4
    statuses:
      - Draft
      - In Progress
      - Completed
      - Paused
      - Cancelled
    status_field: "status"

index:
  auto_update: true      # Auto-update index after create
  preserve_header: true  # Keep custom content above auto-generated marker

author:
  from_git: true         # Default author from git config user.name
  default: ""            # Fallback if git is not available
```

### Template Override Resolution

Templates are resolved in this order (first match wins):

1. Explicit path in `.docz.yaml` `types.<type>.template`
2. Local file at `<docs_dir>/templates/<type>.md`
3. Embedded default template compiled into the binary

This means a repo can override just one type's template while using defaults
for the rest.

## Template System

### Placeholder Variables

Templates use Go `text/template` syntax for variable substitution:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{ .Number }}` | Zero-padded document ID | `0001` |
| `{{ .Title }}` | Document title as provided | `API Rate Limiting` |
| `{{ .Date }}` | Creation date (YYYY-MM-DD) | `2026-02-22` |
| `{{ .Author }}` | Author name | `Donald Gifford` |
| `{{ .Status }}` | Initial status | `Draft` |
| `{{ .Type }}` | Document type | `RFC` |
| `{{ .Prefix }}` | ID prefix from config | `RFC` |
| `{{ .Slug }}` | Kebab-case title | `api-rate-limiting` |
| `{{ .Filename }}` | Generated filename | `0001-api-rate-limiting.md` |

### Default Template: RFC

```markdown
---
id: {{ .Prefix }}-{{ .Number }}
title: "{{ .Title }}"
status: {{ .Status }}
author: {{ .Author }}
created: {{ .Date }}
---

# RFC {{ .Number }}: {{ .Title }}

**Status:** {{ .Status }}
**Author:** {{ .Author }}
**Date:** {{ .Date }}

## Summary

<!-- Brief 2-3 sentence summary of the proposal -->

## Problem Statement

<!-- What problem does this RFC address? Include evidence and impact. -->

## Proposed Solution

<!-- High-level description of the proposed approach -->

## Design

<!-- Detailed design including architecture, data flow, APIs, etc. -->

## Alternatives Considered

<!-- What other approaches were evaluated and why were they rejected? -->

## Implementation Phases

<!-- Break the implementation into phases/milestones -->

### Phase 1: ...

### Phase 2: ...

## Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
|      |        |            |            |

## Success Criteria

<!-- How will we measure whether this RFC achieved its goals? -->

## References

<!-- Links to related ADRs, RFCs, issues, external docs -->
```

### Default Template: ADR

```markdown
---
id: {{ .Prefix }}-{{ .Number }}
title: "{{ .Title }}"
status: {{ .Status }}
author: {{ .Author }}
created: {{ .Date }}
---

# {{ .Number }}. {{ .Title }}

## Status

{{ .Status }}

## Context

<!-- What is the issue that we're seeing that motivates this decision? -->

## Decision

<!-- What is the change that we're proposing and/or doing? -->

## Consequences

<!-- What becomes easier or more difficult to do because of this change? -->

### Positive

-

### Negative

-

### Neutral

-

## Alternatives Considered

<!-- What other options were evaluated? -->

## References

<!-- Links to related RFCs, ADRs, issues, external docs -->
```

### Default Template: DESIGN

```markdown
---
id: {{ .Prefix }}-{{ .Number }}
title: "{{ .Title }}"
status: {{ .Status }}
author: {{ .Author }}
created: {{ .Date }}
---

# DESIGN {{ .Number }}: {{ .Title }}

**Status:** {{ .Status }}
**Author:** {{ .Author }}
**Date:** {{ .Date }}

## Overview

<!-- What is being designed and why? 2-3 sentences. -->

## Goals and Non-Goals

### Goals

-

### Non-Goals

-

## Background

<!-- Context, prior art, and any prerequisites -->

## Detailed Design

<!-- The core of the document. Architecture diagrams, API contracts,
     data models, component interactions, etc. -->

## API / Interface Changes

<!-- Public API surface changes, CLI changes, config changes -->

## Data Model

<!-- Schema changes, storage considerations -->

## Testing Strategy

<!-- How will this be tested? Unit, integration, e2e? -->

## Migration / Rollout Plan

<!-- How do we get from here to there safely? -->

## Open Questions

<!-- Unresolved decisions or areas needing further investigation -->

## References

<!-- Links to related RFCs, ADRs, issues, external docs -->
```

### Default Template: IMPL

```markdown
---
id: {{ .Prefix }}-{{ .Number }}
title: "{{ .Title }}"
status: {{ .Status }}
author: {{ .Author }}
created: {{ .Date }}
---

# IMPL {{ .Number }}: {{ .Title }}

**Status:** {{ .Status }}
**Author:** {{ .Author }}
**Date:** {{ .Date }}

## Objective

<!-- What is being implemented? Link to the RFC/DESIGN it implements. -->

**Implements:** <!-- RFC-XXXX / DESIGN-XXXX -->

## Scope

<!-- Specific scope of this implementation plan -->

### In Scope

-

### Out of Scope

-

## Implementation Steps

### Step 1: ...

- [ ] Task description
- [ ] Task description

### Step 2: ...

- [ ] Task description

## File Changes

<!-- Key files that will be created or modified -->

| File | Action | Description |
|------|--------|-------------|
|      |        |             |

## Testing Plan

- [ ] Unit tests for ...
- [ ] Integration tests for ...

## Rollback Plan

<!-- How to revert if something goes wrong -->

## Dependencies

<!-- External dependencies, blocking work, prerequisites -->

## References

<!-- Links to related RFCs, ADRs, designs, issues -->
```

## Index Generation

### Index Format

Each document type directory gets a `README.md` with an auto-generated table.
The update command preserves any custom content above the auto-generated marker.

**Markers:**

```markdown
<!-- BEGIN DOCZ AUTO-GENERATED -->
...generated content...
<!-- END DOCZ AUTO-GENERATED -->
```

**Generated table example (`docs/rfc/README.md`):**

```markdown
## All RFCs

| ID | Title | Status | Date | Author | Link |
|----|-------|--------|------|--------|------|
| 0001 | API Rate Limiting Strategy | Accepted | 2026-01-15 | dgifford | [0001-api-rate-limiting-strategy.md](0001-api-rate-limiting-strategy.md) |
| 0002 | Migrate to Event Sourcing | Draft | 2026-02-10 | dgifford | [0002-migrate-to-event-sourcing.md](0002-migrate-to-event-sourcing.md) |
```

### Default Index Header

If no `README.md` exists yet, `docz` generates a default header above the
marker that describes the document type, its purpose, how to create new ones,
and the status lifecycle. This header is only generated once -- subsequent
updates preserve whatever is above the marker.

### Index Parsing

The index generator reads YAML frontmatter from each document file to extract:

- `id` -- Document identifier
- `title` -- Document title
- `status` -- Current status
- `created` -- Creation date
- `author` -- Author name

This is a significant improvement over the bash scripts, which used fragile
regex/awk parsing of markdown content. YAML frontmatter provides a structured,
reliable data source.

## Claude Code Skills Integration

> **Note:** The Claude Code plugin is deferred to a later implementation phase.
> This section documents the intended design so the CLI is built with skill
> integration in mind from the start.

### How It Works

`docz` provides a `template show <type>` command that outputs a template to
stdout. Claude Code skills can call this command to get the template, then use
it to guide the AI in writing the document content.

The skill workflow:

1. User invokes a skill (e.g., `/design "Authentication Flow"`)
2. The skill calls `docz template show design` to get the template
3. The skill prompt instructs Claude to fill in the template sections
4. Claude writes the content to the appropriate file
5. The skill calls `docz update design` to refresh the index

### Plugin Structure

The skills will be published as a Claude Code plugin following Anthropic's
official plugin setup, bundled in the docz repo. The plugin will be registered
under a namespace (e.g., `docz@donaldgifford-claude-skills`) and will contain
skill definitions for each document type.

### Skill Definition Pattern

Skills reference `docz` as a CLI dependency. Example skill prompt structure:

```
You are writing a {type} document. Use the following template:

{output of `docz template show <type>`}

Instructions:
- Fill in every section with substantive content
- Use the project context to inform the content
- Follow the YAML frontmatter format exactly
- Save the file to the path output by `docz create --dry-run <type> "<title>"`
```

### Planned Skills

| Skill | Trigger | Document Type | Description |
|-------|---------|---------------|-------------|
| `docz:rfc` | `/rfc "Title"` | RFC | Write a full RFC proposal |
| `docz:adr` | `/adr "Title"` | ADR | Write an architecture decision record |
| `docz:design` | `/design "Title"` | DESIGN | Write a detailed design document |
| `docz:impl` | `/impl "Title"` | IMPL | Write an implementation plan |

### Skill Execution Strategy

Skills call the `docz` CLI to handle file creation and index updates rather
than writing files directly. This ensures consistent numbering, correct
directory placement, and index updates.

The approach is **create then fill**: the skill calls
`docz create <type> "Title"` to create the file from the template, then uses
Claude to fill in each section of the created file. This keeps `docz` as the
single authority for file naming and ID assignment.

## Project Architecture

### Package Layout

```
docz/
  cmd/                    # Cobra command definitions
    root.go               # Root command, config loading
    create.go             # docz create
    update.go             # docz update
    list.go               # docz list
    init.go               # docz init
    template.go           # docz template (show, export, override)
    config.go             # docz config
    version.go            # docz version
  internal/
    config/               # Configuration loading and validation
      config.go           # Config struct, defaults, loading
    document/             # Document creation and management
      document.go         # Document struct, frontmatter parsing
      create.go           # Document creation logic
    index/                # Index/README generation
      index.go            # Index generation and updating
    template/             # Template management
      template.go         # Template loading, resolution, rendering
      embed.go            # Embedded default templates
      templates/          # Default template files (embedded via //go:embed)
        rfc.md
        adr.md
        design.md
        impl.md
        index_rfc.md      # Default index header for RFC
        index_adr.md      # Default index header for ADR
        index_design.md   # Default index header for DESIGN
        index_impl.md     # Default index header for IMPL
  main.go                 # Entry point
  .docz.yaml              # Example config (also used for self-documentation)
```

### Key Design Decisions

**Embedded templates via `//go:embed`:** Default templates are compiled into
the binary so `docz` works with zero configuration. No external files needed.

**YAML frontmatter as the source of truth:** All metadata (status, author,
date, ID) lives in YAML frontmatter at the top of each document. Index
generation reads frontmatter, never parses markdown body content. This is the
main reliability improvement over the bash approach.

**`text/template` for rendering:** Go's standard `text/template` package
handles variable substitution. Chosen over simple string replacement because
it provides conditionals, loops, and a path to partials/includes in future
versions. No additional dependencies required.

**Fixed type set in v1:** Only RFC, ADR, DESIGN, and IMPL are supported.
Custom user-defined types are deferred to a future release to keep v1
focused and the config validation straightforward.

**Viper for configuration:** Already in the dependency tree. Supports YAML
config files, environment variables, and flag binding.

**Cobra for CLI:** Already in the dependency tree. Industry standard for Go
CLIs.

### Frontmatter Parsing

Use a simple approach: read the file, split on `---` delimiters, unmarshal the
YAML block between them. No need for a heavy library -- `go.yaml.in/yaml/v3`
is already a dependency.

```go
type Frontmatter struct {
    ID      string `yaml:"id"`
    Title   string `yaml:"title"`
    Status  string `yaml:"status"`
    Author  string `yaml:"author"`
    Created string `yaml:"created"`
}
```

### ID Assignment

The create command scans the target directory for existing files matching the
pattern `NNNN-*.md`, extracts the highest number, and increments. This matches
the bash script behavior but with proper integer parsing instead of shell
arithmetic.

### Error Handling

- Return clear error messages for common mistakes (missing title, invalid type,
  config not found).
- `docz init` is safe to run multiple times: creates directories if missing,
  skips existing README files unless `--force` is passed, creates `.docz.yaml`
  only if it doesn't exist.
- `docz create` should fail fast if a file with the computed name already
  exists.
- `docz update` should handle empty directories gracefully (generate index with
  empty table).
- `docz update` silently skips files without YAML frontmatter (no error, no
  fallback parsing).

## Migration from Bash Scripts

Users migrating from the bash-based approach:

1. Install `docz` (single binary, `go install` or release download).
2. Run `docz init` in the repo root.
3. Existing `docs/rfc/*.md` and `docs/adr/*.md` files are picked up
   automatically by `docz update` -- as long as they have YAML frontmatter.
4. Files without YAML frontmatter are **ignored** by `docz update`. Users must
   manually add frontmatter to existing documents. There is no automatic
   fallback parsing of markdown body content.
5. Remove old `tools/docs/` scripts and `Makefile` targets.

| Old Command | New Command |
|-------------|-------------|
| `./tools/docs/create-adr.sh "Title"` | `docz create adr "Title"` |
| `./tools/docs/create-rfc.sh "Title"` | `docz create rfc "Title"` |
| `./tools/docs/update-adr-readme.sh` | `docz update adr` |
| `./tools/docs/update-rfc-readme.sh` | `docz update rfc` |

## Testing Strategy

- **Unit tests** for each `internal/` package: config parsing, template
  rendering, frontmatter parsing, ID extraction, slug generation.
- **Integration tests** using `afero.MemMapFs` (already a dependency) to test
  file creation and index generation without touching the real filesystem.
- **CLI tests** using Cobra's built-in test helpers to verify command argument
  parsing and flag handling.
- **Golden file tests** for template rendering and index generation: compare
  output against checked-in expected files.

## Future Considerations

These are explicitly **out of scope** for v1 but noted for later:

- **Custom types** -- Allow users to define arbitrary document types beyond
  the four built-in ones via `.docz.yaml`.
- **Status transitions** -- A `docz status <id> <new-status>` command to
  update a document's status and re-generate the index.
- **Cross-references** -- Automatic linking between documents that reference
  each other (e.g., an IMPL referencing its RFC).
- **Git hooks** -- Auto-run `docz update` on commit when doc files change.
- **Template partials and includes** -- Leverage `text/template` capabilities
  to support reusable template fragments across document types.
- **Claude Code plugin** -- Publish `docz` skills as an Anthropic-compatible
  Claude Code plugin, following the official plugin setup. Skills will call
  the `docz` CLI to get templates, create documents, and update indexes.
  Deferred to a later release to focus on the core CLI first.

## Resolved Decisions

1. **Filename format** -- Keep `NNNN-slug.md` without a type prefix in the
   filename. Each type lives in its own directory (`docs/rfc/`, `docs/adr/`,
   etc.), so the directory name provides the type context. No prefix needed.

2. **Frontmatter-less fallback** -- No. `docz update` strictly requires YAML
   frontmatter. Documents without frontmatter are ignored. No fallback parsing
   of markdown body content. Users must add frontmatter manually to legacy
   files.

3. **Skill authoring** -- Skills will be published as a Claude Code plugin
   following Anthropic's official plugin setup, bundled in the docz repo.
   Deferred to a later implementation phase to focus on the core CLI first.

4. **Config location** -- Support both repo root and `~/.docz.yaml`. Repo
   root `.docz.yaml` takes precedence over the global home directory config.
   This allows user-wide defaults (e.g., author name) to be set globally while
   repos can override specific settings.

5. **Index table columns** -- ID, Title, Status, Date, Author, and Link. This
   is sufficient for v1.

6. **Template format** -- Go `text/template`. This gives us conditionals,
   loops, and a path to partials/includes in future versions. Worth the minor
   complexity over simple string replacement.

7. **Type extensibility in v1** -- No. v1 supports only the four built-in
   types: RFC, ADR, DESIGN, IMPL. Custom types are deferred to a future
   release.

8. **`docz init` behavior** -- `docz init` eagerly creates all four type
   directories with their `README.md` index files:
   `docs/{adr,rfc,design,impl}/README.md`. If the README files already exist,
   they are **not overwritten** unless `--force` is passed. Directories are
   always created if missing (mkdir -p semantics).
