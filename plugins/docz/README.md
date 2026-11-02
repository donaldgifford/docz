# docz

Documentation lifecycle management with the [docz CLI](https://github.com/donaldgifford/docz) — create, organize, and review RFCs, ADRs, design docs, implementation plans, investigations, runbooks, and custom types.

## Requires the docz CLI ≥ v2.0.0

The skills drive the `docz` binary rather than reimplementing it — templates, ID numbering, index generation, and validation all come from the CLI. `docz inspect` and one-document `validate` need v2.2.0, `docz export html` v2.4.0.

```bash
mise use github:donaldgifford/docz@latest
docz version   # confirm >= 2.0.0
```

## Install

From docz's own marketplace, where the plugin is written:

```text
/plugin marketplace add donaldgifford/docz
/plugin install docz@docz
```

or from [claude-skills](https://github.com/donaldgifford/claude-skills), which carries a copy:

```text
/plugin marketplace add donaldgifford/claude-skills
/plugin install docz@donaldgifford-claude-skills
```

docz's marketplace also carries **docz-html** (a document as one standalone HTML page) and **docz-review** (a browser review of the documents queued in a turn).

## Skills

Skills trigger from natural language — no slash commands needed.

### `docz:create`

Create a new document. Resolves doc type from natural language.

> "create an ADR for using PostgreSQL"
> "write up an RFC about switching to gRPC"
> "start an investigation into message queue options"

### `docz:list`

List documents, filtered by type or status.

> "what RFCs do we have?"
> "show me open investigations"
> "list our accepted ADRs"

### `docz:status`

Set a document's status with `docz status set`, validated against the type's lifecycle, then refresh the index.

> "mark ADR-0003 accepted"
> "this design is implemented now"
> "supersede ADR-0005"

### `docz:update`

Regenerate README index tables.

> "update the doc indexes"
> "regenerate the README tables"
> "the doc index looks stale"

### `docz:show`

Show the resolved template for a document type.

> "show me the RFC template"
> "what sections does an ADR have?"
> "what does an IMPL doc look like?"

### `docz:wiki`

Scaffold and maintain a documentation wiki across multiple platforms — MkDocs (default), Backstage TechDocs, GitHub wiki, or Forgejo wiki. Picks a platform and writes the configs that platform needs to publish the `docs/` tree.

> "set up the wiki with mkdocs"
> "wire up Backstage TechDocs"
> "sync to the GitHub wiki"
> "rebuild the nav"

### `docz:website`

Scaffold a public-facing documentation website — MkDocs Material (default), Docusaurus, or Zensical.

> "scaffold a docusaurus site"
> "set up a docs website"
> "build a dev portal"

### `docz:go-docz`

Generate Go API documentation using `pkgsite` (the renderer behind pkg.go.dev) and write it to `docs/api/go/`. Composes with `docz:wiki` and `docz:website`. Ships a drift-detecting CI workflow.

> "generate Go API docs"
> "publish godoc to the wiki"

### `docz:rust-docz`

Generate Rust API documentation with `cargo doc` and write it to `docs/api/rust/`. Same output-target detection and drift-detection workflow as `go-docz`. Steps aside if `book.toml` (mdbook) is present.

> "generate Rust API docs"
> "publish rustdoc to the wiki"

## Doc Types

| Type | Purpose |
|------|---------|
| RFC | High-level proposals for significant changes requiring team alignment |
| ADR | Lightweight, permanent records of architectural decisions |
| DESIGN | Implementation detail after a decision is made |
| IMPL | Phase-based execution tracking with checkboxes |
| INV | Time-boxed research and validation experiments |
| RUNBOOK | Onboarding, operations, and troubleshooting procedures — **disabled by default**, set `types.runbook.enabled: true` |

Custom types are supported too: give a type its own `id_prefix`, `aliases`, and `statuses` in `.docz.yaml`, and docz creates its documents from its generic template until you add one at `<docs_dir>/templates/<name>.md`.

## Quick Start

```bash
# Initialize docz in your repo
docz init

# Create your first ADR
docz create adr "Use PostgreSQL for primary storage"

# List all documents
docz list

# Check it, then update README index tables
docz validate
docz update

# View resolved configuration
docz config
```
