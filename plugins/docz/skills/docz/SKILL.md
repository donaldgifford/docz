---
name: docz
description: >
  Documentation lifecycle management with the docz CLI — RFCs, ADRs,
  design docs, implementation plans, investigations, and runbooks.
when_to_use: >
  Creating, organizing, validating, or reviewing project documentation.
  Triggers on: docz, ADR, RFC, DESIGN doc, IMPL doc, INV doc, runbook,
  architecture decision record, "create a doc", "document this decision",
  "write a proposal", "validate the docs", "export the docs".
---

# Docz

Workflow guidance for managing project documentation with the `docz` CLI.

## Core Principles

- **Right type for the scope** — RFC for broad proposals, ADR for decisions, DESIGN for implementation detail, IMPL for phase tracking, INV for research, RUNBOOK for operational procedures
- **Auto-IDs and indexes** — docz increments IDs and maintains README index tables; never hand-edit the `<!-- BEGIN DOCZ AUTO-GENERATED -->` sections
- **Regions are docz's** — every section a template writes sits between `<!--docz:<kind>:start-->` / `<!--docz:<kind>:end-->` markers. Write inside them; never delete, rename, or reorder a marker
- **Config at the repo level** — `.docz.yaml` controls directories, statuses, and which types are enabled; read it (or `docz config`) before creating docs
- **Status discipline** — move documents through their lifecycle with `docz status set`, then `docz update`; never hand-edit the `status:` key
- **Validate before you hand off** — `docz validate <id>` after writing a document; fix what it reports

## Doc Types at a Glance

| Type | When to use | Statuses |
|------|-------------|---------|
| RFC | High-level proposals for significant changes | Draft, Proposed, Accepted, Rejected, Superseded |
| ADR | Lightweight records of architectural decisions | Proposed, Accepted, Deprecated, Superseded |
| DESIGN | Implementation detail after a decision is made | Draft, In Review, Approved, Implemented, Abandoned |
| IMPL | Phase-based tracking with checkboxes | Draft, In Progress, Completed, Paused, Cancelled |
| INV | Time-boxed research and validation experiments | Open, In Progress, Concluded, Inconclusive, Abandoned |
| RUNBOOK [^optin] | Onboarding, operations, and troubleshooting procedures | Draft, Active, Needs Review, Deprecated |

[^optin]: RUNBOOK ships **disabled**: `types.runbook.enabled: false` is in every generated `.docz.yaml`, and `docz create runbook` fails naming that key until it is set to `true`.

There is no PLAN type in docz v2. A repository whose `.docz.yaml` still carries a `types.plan` block keeps working: `plan` becomes a **custom** type rendered from the generic template.

## Inline Rules

### Creating documents

- **`docz create <type> <title>`** — type is lowercase (`rfc`, `adr`, `design`, `impl`, `inv`, `runbook`/`rb`) or a custom type's name, alias, or id prefix
- **Title becomes the slug** — spaces become hyphens, lowercase; keep titles descriptive but concise
- **`--author`** / **`--status`** — override the resolved author or the initial status (default: the type's first status)
- **`--no-update`** — skip README regeneration; useful in scripts that batch-create docs
- **`--format json`** — one JSON object with the new document's `id`, `type`, and `path`; read the path from it rather than guessing
- **Custom type without a template** — `docz create` renders the generic template and says so on stderr; `docz template override <type>` writes one to customise

### Validating

- **`docz validate`** — every enabled type: regions each schema requires, per-region content rules, frontmatter, a stale ToC, and README index drift
- **`docz validate <id|path>`** — one document, read once; the fast check after editing
- **`--fix`** — marks the regions docz inferred from headings and canonicalises marker spellings, then reports what is left. It never invents a section
- **`--strict`** — warnings and index drift fail too (exit 1); use it in CI. **`--format json`** for scripts
- Findings print as `path:line code detail`; the **code** is stable, the wording is not

### Inspecting

- **`docz inspect <id|path>`** — one document's frontmatter, regions, headings, open questions, tasks, and allowed statuses. `--format json` includes the document's bytes and SHA-256. Use it instead of parsing markdown yourself

### Config

- **`.docz.yaml`** at repo root — `docs_dir`, per-type `dir`, `id_prefix`, `id_width`, `statuses`, `enabled`
- **`~/.docz.yaml`** (global) — author defaults and personal overrides; repo config wins per key
- **`docz config`** — prints the resolved config; run it to debug, to check whether a type is enabled, or to look up a custom type
- **Optional blocks** — `changelog:`, `api:`, `sync.confluence:` are dormant until enabled; `html:` (the export theme) and `access:` (who may read the repo through docz-api) always have a value. See `references/config.md`
- **`--repo-root`** — run docz against a repo without `cd`-ing into it

### Status

- **`docz status set <type> <id> <status>`** — the canonical mutator; validates against the type's `statuses`
- **Always follow with `docz update <type>`** — `status set` does not touch the README index table
- **Case-sensitive** — ids (`ADR-0003`) and statuses (`Accepted`). Wrong-case id exits `1`; wrong-case status exits `2`
- **`--dry-run`** to preview, **`--format json`** for scripts, **`--quiet`** to suppress stdout

### Updating indexes

- **`docz update`** — regenerates every README table and ToC; safe to run repeatedly
- **`docz update <type>`** — one type's index
- **`--dry-run`** — preview without writing; `docz validate --strict` is the CI check for drift

### Templates

- **`docz template show <type>`** — prints the resolved template (repo override, else embedded). Works for disabled types
- **`docz template override <type>`** — copies the template into `docs/templates/` for customisation; for a custom type with none it scaffolds the generic template and its schema
- Template variables: `{{ .Number }}`, `{{ .Title }}`, `{{ .Slug }}`, `{{ .Filename }}`, `{{ .Date }}`, `{{ .Author }}`, `{{ .Status }}`, `{{ .Type }}`, `{{ .Prefix }}`

### Exporting

The repository stays the source of truth; an export only writes outward.

- **`docz export html [id|path...]`** — standalone HTML pages in the repository's `html.theme` (`paper`, `charcoal`, `classic`), under `build/html/`. `--stdout` prints one page; `--theme` is a one-off. The **docz-html** plugin drives this end to end
- **`docz export confluence [id|path...]`** — pages in the Confluence space `sync.confluence` names. Credentials only from `ATLASSIAN_EMAIL` / `ATLASSIAN_API_TOKEN`; `--dry-run` first. A page edited in Confluence is skipped unless `--force`. IMPLs are exported only when `sync.confluence.types` lists `impl`

## CLI Requirement

This plugin drives the `docz` binary; it does not reimplement it. **Requires `docz` ≥ v2.0.0 on `PATH`.** Later features: `docz inspect`, one-document `validate`, and `create --format json` in v2.2.0; `export html` in v2.4.0; `html.theme` in v2.5.0.

- **Templates come from the CLI** — `docz template show <type>` is the single source of truth; this plugin ships no template copies
- **`docz:create` / `docz:show` / `docz:status` / `docz:update`** — CLI-only. If the binary is missing, stop and point the user at the install path
- **`docz:list`** — can read `docs/<type>/*.md` frontmatter directly if the CLI is unavailable

```bash
mise use github:donaldgifford/docz@latest   # or download from the releases page
docz version                                 # confirm ≥ 2.0.0
```

## Reference Files

Read `references/<file>.md` when you encounter the specific topic.

| Topic | File | When to read |
|---|---|---|
| **Doc Types** | `doc-types` | Choosing and filling each doc type |
| **Configuration** | `config` | `.docz.yaml` fields, author resolution, per-type overrides, optional blocks |
| **Workflow** | `workflow` | End-to-end workflows: proposal, decision, review, validation, export |

## Quick Reference

```bash
# Initialize docz in a repo
docz init

# Create docs (type is lowercase)
docz create rfc "Switch to gRPC for internal services"
docz create adr "Use PostgreSQL for primary storage"
docz create design "Auth token refresh flow"
docz create impl "Database migration"
docz create inv "Evaluate message queue options"
docz create runbook "Rotate the signing key"   # requires types.runbook.enabled: true

# Check what you wrote
docz validate ADR-0004
docz validate --fix          # mark inferred regions, report the rest
docz inspect ADR-0004 --format json

# List
docz list                    # all docs
docz list adr --status Accepted --format json

# Change a status (always pair with update)
docz status set adr ADR-0003 Accepted && docz update adr

# Config and templates
docz config
docz template show adr

# Export
docz export html ADR-0004 --stdout
docz export confluence --dry-run
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
- `docz:website`, `docz:go-docz`, `docz:rust-docz` — documentation sites and API docs

## Companion plugins

From the same marketplace (`/plugin install <name>@docz`):

- **docz-html** — turn a document into one standalone HTML page in the repository's theme
- **docz-review** — hold the end of a turn for a browser review of the documents queued in it
