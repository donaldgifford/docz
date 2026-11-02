---
name: docz
description: >
  Documentation lifecycle management with the docz CLI — RFCs, ADRs,
  design docs, implementation plans, investigations.
when_to_use: >
  Creating, organizing, or reviewing project documentation. Triggers on:
  docz, ADR, RFC, DESIGN doc, IMPL doc, PLAN doc, INV doc, architecture
  decision record, "create a doc", "document this decision", "write a
  proposal".
---

# Docz

Workflow guidance for managing project documentation with the `docz` CLI.

## Core Principles

- **Right type for the scope** — use RFC for broad proposals, ADR for decisions, DESIGN for implementation detail, PLAN for mid-level planning, IMPL for phase tracking, INV for research
- **Auto-IDs and indexes** — docz increments IDs and maintains README index tables automatically; never hand-edit the `<!-- BEGIN DOCZ AUTO-GENERATED -->` sections
- **Config at the repo level** — `.docz.yaml` controls directories, statuses, and author resolution; read it before creating docs
- **Status discipline** — move documents through their lifecycle (Draft → Proposed → Accepted/Rejected) with `docz status set`, then `docz update`; never hand-edit the `status:` key

## Doc Types at a Glance

| Type | When to use | Statuses |
|------|-------------|---------|
| RFC | High-level proposals for significant changes | Draft, Proposed, Accepted, Rejected, Superseded |
| ADR | Lightweight records of architectural decisions | Proposed, Accepted, Deprecated, Superseded |
| DESIGN | Implementation detail after a decision is made | Draft, In Review, Approved, Implemented, Abandoned |
| IMPL | Phase-based tracking with checkboxes | Draft, In Progress, Completed, Paused, Cancelled |
| PLAN [^optin] | Mid-level planning between RFC and IMPL | Draft, In Progress, Completed, Cancelled |
| INV | Time-boxed research and validation experiments | Open, In Progress, Concluded, Inconclusive, Abandoned |

[^optin]: PLAN is **opt-in** since docz v1.1.1 — `types.plan.enabled` defaults to `false`, `docz init` does not scaffold `docs/plan/`, and `docz create plan` fails until it is enabled in `.docz.yaml`.

## Inline Rules

### Creating documents

- **`docz create <type> <title>`** — type is lowercase (`rfc`, `adr`, `design`, `impl`, `plan`, `inv`)
- **Title becomes the slug** — spaces become hyphens, lowercase; keep titles descriptive but concise
- **`--author`** — override author if not set in config; use full name or handle consistently
- **`--status`** — set initial status; defaults to the first status in the type's list
- **`--no-update`** — skip README regeneration; useful in scripts that batch-create docs

### Config

- **`.docz.yaml`** at repo root — controls `docs_dir`, per-type `dir`, `id_prefix`, `id_width`, `statuses`
- **`~/.docz.yaml`** (global) — author defaults and personal overrides; repo config takes precedence per-key
- **`docz config`** — prints the fully resolved config; run this to debug unexpected behavior, to check whether a type is enabled, or to look up a custom type
- **Custom types** — any type can be defined in `.docz.yaml` with its own `id_prefix`, `aliases`, and `statuses`; it needs a template at `<docs_dir>/templates/<name>.md`. An unfamiliar type word may be custom, not a typo
- **`changelog:` / `api:` blocks** — optional declarations for tools that read a docz repo; dormant unless enabled. Enabling `api:` publishes every `.md` under `docs_dir`
- **`--repo-root`** — run docz against a repo without `cd`-ing into it

### Status

- **`docz status set <type> <id> <status>`** — the canonical mutator; validates against the type's `statuses`. Never hand-edit the `status:` key
- **Always follow with `docz update <type>`** — `status set` does not touch the README index table
- **Case-sensitive** — ids (`ADR-0003`) and status values (`Accepted`) both. Wrong-case id exits `1`; wrong-case status exits `2`
- **`--dry-run`** to preview, **`--format json`** for scripts, **`--quiet`** to suppress stdout without changing the exit code

### Updating indexes

- **`docz update`** — regenerates all README tables; safe to run repeatedly
- **`docz update <type>`** — update a single type's index
- **`--dry-run`** — preview changes without writing; use in CI to detect drift
- Run `docz update` after bulk edits or when README tables look stale

### Templates

- **`docz template show <type>`** — the CLI embeds every default template; this prints the resolved one. Works even for disabled types
- **`docz template override <type>`** — copy template into `docs/templates/` for customization (built-in types only)
- Template variables: `{{ .Number }}`, `{{ .Title }}`, `{{ .Slug }}`, `{{ .Filename }}`, `{{ .Date }}`, `{{ .Author }}`, `{{ .Status }}`, `{{ .Type }}`, `{{ .Prefix }}`

## CLI Requirement

This plugin drives the `docz` binary; it does not reimplement it. **Requires `docz` ≥ v1.2.0 on `PATH`** (`status set` landed in v1.1.1, the `api:` block in v1.2.0).

- **Templates come from the CLI** — `docz template show <type>` is the single source of truth, so an override or a CLI upgrade is reflected immediately. This plugin ships no template copies.
- **`docz:create` / `docz:show`** — CLI-only. If the binary is missing, stop and point the user at the install path; there is no offline path.
- **`docz:list`** — can read `docs/<type>/*.md` frontmatter directly if the CLI is unavailable.
- **`docz:update` / `docz:status`** — CLI-only. If unavailable, say so and note that indexes stay stale until the user runs `docz update`.

```bash
mise use github:donaldgifford/docz@latest   # or download from the releases page
docz version                                 # confirm ≥ 1.2.0
```

## Reference Files

Read `references/<file>.md` when you encounter the specific topic.

| Topic | File | When to read |
|---|---|---|
| **Doc Types** | `doc-types` | Detailed guidance on choosing and filling each doc type |
| **Configuration** | `config` | `.docz.yaml` fields, author resolution, per-type overrides, wiki config |
| **Workflow** | `workflow` | End-to-end workflows: proposal, decision, review, archiving |

## Quick Reference

```bash
# Initialize docz in a repo
docz init

# Create docs (type is lowercase)
docz create rfc "Switch to gRPC for internal services"
docz create adr "Use PostgreSQL for primary storage"
docz create design "Auth token refresh flow"
docz create plan "Q3 API migration"     # requires types.plan.enabled: true
docz create impl "Phase 1: Database migration"
docz create inv "Evaluate message queue options"

# With flags
docz create adr "Use Redis for caching" --author "Jane Smith" --status Proposed

# List and inspect
docz list               # all docs
docz list adr           # filtered by type
docz list --status Draft --format json

# Update indexes
docz update             # regenerate all README tables
docz update --dry-run   # preview changes

# Change a status (always pair with update)
docz status set adr ADR-0003 Accepted && docz update adr

# Config and templates
docz config             # print resolved config
docz template show adr  # print template
```

## Agents

- `docz:doc-recommender` — recommend the right doc type given a description of the work
- `docz:doc-reviewer` — review a draft document for completeness and clarity

## Skills

- `docz:create` — create a new document from natural language ("create an ADR for...")
- `docz:list` — list documents ("what RFCs do we have?", "show me open investigations")
- `docz:status` — set a document's status ("mark ADR-0003 accepted")
- `docz:update` — regenerate README index tables ("update the doc indexes")
- `docz:show` — show a doc type's template ("what sections does an ADR have?")
- `docz:wiki` — manage MkDocs/Backstage TechDocs integration ("set up mkdocs", "update wiki nav")
