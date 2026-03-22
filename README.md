# docz

A CLI tool for generating and managing standardized documentation files in
software repositories. `docz` creates documents (RFCs, ADRs, design docs,
implementation plans) from embedded templates, auto-increments IDs, and keeps
README index tables up to date.

## Features

- **Six built-in document types:** RFC, ADR, DESIGN, IMPL, PLAN, INV
- **Auto-incremented IDs:** documents are numbered sequentially within their type directory
- **YAML frontmatter:** every document carries structured metadata (id, title, status, author, created)
- **Auto-generated index tables:** README files in each type directory are updated automatically after each `create`
- **Template overrides:** customize any template per-repository without forking
- **Configuration:** repo-level `.docz.yaml` deep-merged with global `~/.docz.yaml`
- **Multiple output formats:** `list` supports table, JSON, and CSV
- **Table of contents:** automatic ToC generation in documents using `<!--toc:start-->` / `<!--toc:end-->` markers
- **MkDocs/TechDocs integration:** `wiki` commands generate and maintain `mkdocs.yml` for Backstage TechDocs

## Getting Started

### Installation

```bash
# Build from source
git clone https://github.com/donaldgifford/docz.git
cd docz
make build
# Binary: build/bin/docz

# Or install directly
go install github.com/donaldgifford/docz/cmd/docz@latest
```

### Initialize a repository

```bash
cd your-repo
docz init
```

This creates:
- `.docz.yaml` — repo configuration
- `docs/rfc/README.md`
- `docs/adr/README.md`
- `docs/design/README.md`
- `docs/impl/README.md`
- `docs/plan/README.md`
- `docs/investigation/README.md`

### Create your first document

```bash
docz create rfc "Use OpenTelemetry for Distributed Tracing"
# → docs/rfc/0001-use-opentelemetry-for-distributed-tracing.md

docz create adr "Adopt PostgreSQL as Primary Database"
# → docs/adr/0001-adopt-postgresql-as-primary-database.md
```

The document is created with YAML frontmatter and a type-appropriate template
structure. The README index for that type is updated automatically.

### List documents

```bash
docz list                        # all types, table format
docz list rfc                    # only RFCs
docz list --status draft         # filter by status (case-insensitive)
docz list --format json          # JSON output
docz list --format csv           # CSV output
```

### Update indexes manually

```bash
docz update          # regenerate README for all types
docz update rfc      # only the RFC index
docz update --dry-run  # preview changes without writing
```

## Commands

| Command | Description |
|---------|-------------|
| `docz init` | Initialize docz in the current repository |
| `docz create <type> <title>` | Create a new document from a template |
| `docz update [type]` | Regenerate README index tables |
| `docz list [type]` | List documents, optionally filtered by type |
| `docz template show <type>` | Print the resolved template to stdout |
| `docz template export <type> [path]` | Write the resolved template to a file |
| `docz template override <type>` | Copy the template into the local overrides directory |
| `docz wiki init` | Create `mkdocs.yml` with TechDocs defaults |
| `docz wiki update` | Rebuild the MkDocs nav from docs/ contents |
| `docz config` | Print the fully resolved configuration as YAML |
| `docz version` | Print version and commit hash |

### Global Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Use a specific config file instead of `.docz.yaml` |
| `--docs-dir <path>` | Override the base docs directory |
| `--verbose` | Print additional context during operations |

### `docz create` Flags

| Flag | Description |
|------|-------------|
| `--author <name>` | Override the document author |
| `--status <status>` | Set the initial status (defaults to the first configured status) |
| `--no-update` | Skip the automatic index update after creation |

### `docz init` Flags

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing README index files |

### `docz update` Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Preview changes without writing any files |

### `docz list` Flags

| Flag | Description |
|------|-------------|
| `--status <status>` | Filter documents by status (case-insensitive) |
| `--format <fmt>` | Output format: `table` (default), `json`, `csv` |

### `docz wiki init` Flags

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing `mkdocs.yml` |
| `--site-name <name>` | Set `site_name` (default: repo directory name) |
| `--site-description <desc>` | Set `site_description` |

### `docz wiki update` Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Print the generated nav without modifying `mkdocs.yml` |

## Document Types

### RFC — Request for Comments

High-level proposals for significant changes. Use when you need broader
discussion before committing to a direction.

```
docs/rfc/
└── 0001-use-opentelemetry-for-distributed-tracing.md
```

### ADR — Architecture Decision Record

Lightweight records of architectural decisions and their rationale. Use after a
decision has been made to capture why.

```
docs/adr/
└── 0001-adopt-postgresql-as-primary-database.md
```

### DESIGN — Design Document

Detailed design specifications for a feature or system. Use when the what is
decided and you need to work out the how.

```
docs/design/
└── 0001-telemetry-pipeline-design.md
```

### IMPL — Implementation Plan

Phase-based implementation plans with checkboxes. Use to track execution of a
design across multiple steps.

```
docs/impl/
└── 0001-telemetry-pipeline-implementation.md
```

### PLAN — Plan

Mid-level planning documents that sit between an RFC (what and why) and an IMPL
(step-by-step execution). Use a plan to work out the approach and component
breakdown before writing detailed tasks.

```
docs/plan/
└── 0001-telemetry-pipeline-approach.md
```

### INV — Investigation

Time-boxed research spikes and validation experiments. Use to answer a specific
question before committing to a design or implementation — e.g. proving a
library handles a requirement, reproducing a weird error, or validating a
performance assumption. Design, plan, and impl docs can reference investigations
by ID to document how open questions were resolved.

```
docs/investigation/
└── 0001-can-pgvector-handle-concurrent-writes.md
```

## Configuration

`docz` reads configuration from two locations, deep-merged with repo taking
precedence:

1. `~/.docz.yaml` — global defaults
2. `.docz.yaml` — repo-root config (overrides global)

### Example `.docz.yaml`

```yaml
docs_dir: docs

author:
  from_git: true       # use git config user.name
  default: ""          # fallback if git name unavailable

index:
  auto_update: true    # update README after docz create
  preserve_header: true

types:
  rfc:
    enabled: true
    dir: rfc
    id_prefix: RFC
    id_width: 4
    statuses:
      - Draft
      - Proposed
      - Accepted
      - Rejected
      - Superseded
  adr:
    enabled: true
    dir: adr
    id_prefix: ADR
    id_width: 4
    statuses:
      - Proposed
      - Accepted
      - Deprecated
      - Superseded
  design:
    enabled: true
    dir: design
    id_prefix: DESIGN
    id_width: 4
    statuses:
      - Draft
      - In Review
      - Approved
      - Implemented
      - Abandoned
  impl:
    enabled: true
    dir: impl
    id_prefix: IMPL
    id_width: 4
    statuses:
      - Draft
      - In Progress
      - Completed
      - Paused
      - Cancelled
  plan:
    enabled: true
    dir: plan
    id_prefix: PLAN
    id_width: 4
    statuses:
      - Draft
      - In Progress
      - Completed
      - Cancelled
  investigation:
    enabled: true
    dir: investigation
    id_prefix: INV
    id_width: 4
    statuses:
      - Open
      - In Progress
      - Concluded
      - Inconclusive
      - Abandoned

wiki:
  auto_update: true
  mkdocs_path: mkdocs.yml
  exclude:
    - templates
    - examples
  nav_titles:
    rfc: "RFCs"
    adr: "ADRs"
    design: "Design"
    impl: "Implementation Plans"
    plan: "Plans"
    investigation: "Investigations"

toc:
  enabled: true            # generate ToC during docz update
  min_headings: 3          # minimum headings to generate a ToC
```

Run `docz config` to see the fully resolved configuration.

## Template System

Templates are resolved in this order:

1. **Config path** — `types.<type>.template` key in `.docz.yaml`
2. **Local override** — `<docs_dir>/templates/<type>.md`
3. **Embedded default** — built into the binary

To override a template for your repository:

```bash
# Copy the embedded template into your local overrides directory
docz template override rfc
# → creates docs/templates/rfc.md

# Edit it
$EDITOR docs/templates/rfc.md

# Future creates will use your override automatically
docz create rfc "My Proposal"
```

To preview the resolved template without creating a document:

```bash
docz template show rfc
```

### Template Variables

| Variable | Description |
|----------|-------------|
| `{{ .Number }}` | Zero-padded document number (e.g. `0001`) |
| `{{ .Title }}` | Document title as provided |
| `{{ .Slug }}` | Kebab-case slug derived from the title |
| `{{ .Filename }}` | Full filename (e.g. `0001-my-title.md`) |
| `{{ .Date }}` | Creation date (`YYYY-MM-DD`) |
| `{{ .Author }}` | Resolved author name |
| `{{ .Status }}` | Initial status |
| `{{ .Type }}` | Document type (e.g. `rfc`) |
| `{{ .Prefix }}` | ID prefix (e.g. `RFC`) |

## Index Tables

Each type directory contains a `README.md` with an auto-generated table of all
documents. The table is bounded by HTML comments:

```markdown
<!-- BEGIN DOCZ AUTO-GENERATED -->
| ID | Title | Status | Date | Author | Link |
|----|-------|--------|------|--------|------|
| RFC-0001 | My Proposal | Draft | 2026-01-01 | Alice | [0001-my-proposal.md](0001-my-proposal.md) |
<!-- END DOCZ AUTO-GENERATED -->
```

Content outside these markers (headers, descriptions, links) is preserved across
updates. If a README has no markers, `docz update` will warn rather than modify
it — run `docz init --force` or add the markers manually.

## Table of Contents

`docz update` automatically generates a table of contents in documents that
contain `<!--toc:start-->` and `<!--toc:end-->` markers. New documents created
with `docz create` include these markers by default.

```markdown
<!--toc:start-->
- [Summary](#summary)
- [Problem Statement](#problem-statement)
- [Design](#design)
  - [Phase 1: Setup](#phase-1-setup)
  - [Phase 2: Migration](#phase-2-migration)
- [References](#references)
<!--toc:end-->
```

The ToC uses GitHub-compatible anchor links, relative indentation based on
heading depth, and handles duplicate headings with `-1`, `-2` suffixes. Headings
inside fenced code blocks are excluded.

Documents with fewer headings than `toc.min_headings` (default: 3) will have
empty markers. The feature can be disabled with `toc.enabled: false` in
`.docz.yaml`.

The markers are compatible with the
[markdown-toc.nvim](https://github.com/hedyhli/markdown-toc.nvim) plugin, so
documents edited in Neovim/lazyvim will work with both tools.

## MkDocs / Backstage TechDocs Integration

`docz wiki` generates and maintains a `mkdocs.yml` compatible with Backstage's
TechDocs plugin. The nav section is rebuilt from the docs directory contents.

```bash
# Initialize mkdocs.yml and docs/index.md
docz wiki init
docz wiki init --site-name "My Service"

# Rebuild the nav section from docs/ contents
docz wiki update
docz wiki update --dry-run    # preview without writing

# Auto-update: docz create also updates the nav when mkdocs.yml exists
docz create rfc "My Proposal"  # → nav is updated automatically
```

### Configuration

Wiki behavior is controlled by the `wiki` section in `.docz.yaml`:

```yaml
wiki:
  auto_update: true          # auto-run wiki update after docz create
  mkdocs_path: mkdocs.yml    # path to mkdocs.yml
  exclude:                   # directories excluded from nav
    - templates
    - examples
  nav_titles:                # override directory display names
    rfc: "Request for Comments"
```

### Nav Generation

- Docz documents use their frontmatter title (e.g., "RFC-0001: API Rate Limiting")
- Other markdown files use their first H1 heading or filename
- `wiki init` sorts sections alphabetically
- `wiki update` preserves existing section order, appending new sections at the end
- README.md / index.md files become "Overview" entries
- Empty directories and excluded directories are skipped

## Makefile Integration

After `docz init`, the Makefile in this repository includes convenience targets:

```bash
make docs-init    # docz init
make docs-update  # docz update (all types)
make docs-list    # docz list
make docs-config  # docz config
```

## Author Resolution

`docz create` resolves the document author in this order:

1. `--author` flag
2. `author.default` in `.docz.yaml`
3. `git config user.name`
4. `"Unknown"`
