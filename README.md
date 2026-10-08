# docz

A CLI tool for generating and managing standardized documentation files in
software repositories. `docz` creates documents (RFCs, ADRs, design docs,
implementation plans) from embedded templates, auto-increments IDs, and keeps
README index tables up to date.

## Features

- **Six built-in document types:** RFC, ADR, DESIGN, IMPL, INV, and RUNBOOK (disabled by default)
- **Custom document types:** define your own types in `.docz.yaml` — own prefix, statuses, aliases, and templates — invoked by name, alias, or `id_prefix`
- **Auto-incremented IDs:** documents are numbered sequentially within their type directory
- **YAML frontmatter:** every document carries structured metadata (id, title, status, author, created)
- **Auto-generated index tables:** README files in each type directory are updated automatically after each `create`
- **Template overrides:** customize any template per-repository without forking
- **Configuration:** repo-level `.docz.yaml` deep-merged with global `~/.docz.yaml`
- **Multiple output formats:** `list` supports table, JSON, and CSV
- **Table of contents:** automatic ToC generation in documents using `<!--toc:start-->` / `<!--toc:end-->` markers
- **MkDocs/TechDocs integration:** `wiki` commands generate and maintain `mkdocs.yml` for Backstage TechDocs
- **Changelog awareness:** opt-in `changelog:` config block plus a parser for git-cliff / Keep a Changelog files
- **Publishing declaration:** opt-in `api:` config block naming the repo's landing page, exclusions, and markdown outside `docs_dir`, for tools that render a docz repo

## Getting Started

### Installation

```bash
# Build from source
git clone https://github.com/donaldgifford/docz.git
cd docz
just build
# Binary: build/bin/docz

# Or install directly
go install github.com/donaldgifford/docz/v2/cmd/docz@latest
```

The `/v2` path is the current development line. Released v1.x tags stay on
the unversioned path, so `go install github.com/donaldgifford/docz/cmd/docz@v1.2.2`
is how you pin the last v1 release.

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
- `docs/investigation/README.md`

Types with `enabled: false` in `.docz.yaml` are skipped — no directory or
README is created for them. That includes `runbook`, which ships disabled;
see [RUNBOOK](#runbook--runbook) to turn it on.

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
| `docz validate [type]` | Check documents against their schema and report drift |
| `docz export confluence [id\|path...]` | Export documents to Confluence Cloud as pages ([Sync](#sync)) |
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

### `docz validate` Flags

| Flag | Description |
|------|-------------|
| `--strict` | Fail on warnings and index drift as well as errors |
| `--format <fmt>` | Output format: `text` (default), `json` |
| `--fix` | Mark the regions inference finds, then report what is left |

### `docz export confluence` Flags

| Flag | Description |
|------|-------------|
| `--type <name>` | Export one type; repeatable (default: `sync.confluence.types`, else every enabled type) |
| `--force` | Overwrite pages edited in Confluence and adopt pages docz did not create (never another repository's) |
| `--dry-run` | Make every read and no write; report what each page would have done |
| `--out <dir>` | Also write each rendered page to `<dir>/<id>.xhtml` |
| `--format <fmt>` | Output format: `text` (default), `json` |
| `--strict` | Exit 1 when any page was skipped |

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

> **Upgrading from v1?** `plan` was a sixth built-in through v1 and is not one
> in v2 (ADR-0003). A repo that keeps its `types.plan` block loses nothing: the
> block now declares a **custom** type, so `init`, `list`, `update`, `status`,
> and the wiki nav all keep working over `docs/plan/`. Only `docz create plan`
> stops, because there is no bundled template left to create from — run
> `docz template override plan` to scaffold one and it works again.

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

### INV — Investigation

Time-boxed research spikes and validation experiments. Use to answer a specific
question before committing to a design or implementation — e.g. proving a
library handles a requirement, reproducing a weird error, or validating a
performance assumption. Design and impl docs can reference investigations by ID
to document how open questions were resolved.

```
docs/investigation/
└── 0001-can-pgvector-handle-concurrent-writes.md
```

### RUNBOOK — Runbook

Step-by-step procedures for onboarding, operating, and troubleshooting a
service or tool. A runbook is **disabled by default**: every generated
`.docz.yaml` carries its block with `enabled: false`, and a repository opts in.

A runbook opens with a **Last Verified** table, one row recording the last
end-to-end run: Date, PR, Commit, and Verified by, plus a one- or two-sentence
`**Notes:**` line. Re-verifying replaces the row, and git keeps the history.
Then come **procedures** (`### Procedure 1: Rotate`), each with steps,
verification, and rollback, and **scenarios** (`### Scenario: <the symptom>`),
each with `**Alert:**`, `**Likely cause:**`, and steps.

Steps are an **ordered list**. Number them, nest sub-steps under them, put a
step's command in a fenced block beneath it, and say what it should show on an
`**Expected:**` line. `docz validate` reports a steps section written as
bullets. Steps have stable IDs: `2.3` is the third step of procedure 2,
`2.3.1` its first sub-step, `S1.2` a scenario's second step, and `2.R1` a
rollback step.

```
docs/runbook/
└── 0001-rotate-the-webhook-secret.md
```

**Enabling it.** How you turn runbooks on depends on whether `.docz.yaml` has
a `types:` block, because a `types:` block keeps exactly the types it lists:

- **It has one** (every repo `docz init` scaffolded does): add
  `runbook: {enabled: true}` to it. The short block gets every other field
  from docz's defaults.
- **It has none:** a `types:` block listing only `runbook` would switch off the
  other five. List all six, or copy the full `types:` block a current
  `docz init` generates and flip the flag.

Then run `docz update` to create `docs/runbook/` and its README. If you run
docz-api, deploy `v2.0.0-beta.6` or later **before** a repository enables
runbooks: an older docz-api has no runbook defaults to fill the short block
from.

## Custom Document Types

Beyond the six built-ins, you can define your own document types entirely in
`.docz.yaml` — no rebuild required. Add an entry under `types:` with a unique
`id_prefix` and a directory:

```yaml
types:
  # ...built-in types...
  frameworks:
    enabled: true
    dir: frameworks
    id_prefix: FW
    id_width: 4
    aliases: [fw]            # optional CLI shorthands
    statuses:
      - Draft
      - Active
      - Deprecated
    status_field: status
    plural_label: Frameworks
```

Once declared, the type behaves exactly like a built-in:

```bash
docz create frameworks "Service Mesh"   # by canonical name
docz create FW "Service Mesh"           # by id_prefix
docz create fw "Service Mesh"           # by alias
docz list fw                            # aliases work wherever a type is accepted
docz update                             # no-arg update includes custom types
docz status set FW FW-0001 Active       # status mutation resolves the type too
```

The file is named with the usual `<number>-<slug>.md` convention
(`docs/frameworks/0001-service-mesh.md`); the `id_prefix` is applied to the
frontmatter `id:` (`FW-0001`), matching the built-in types.

**Resolution precedence.** A type token is matched case-insensitively as
**canonical name → alias → `id_prefix`**, so a canonical name always wins over a
colliding alias or prefix.

**Templates.** A custom type uses the same override resolution as the built-ins:

- **Body template** — `docs/templates/<type>.md` (e.g. `docs/templates/frameworks.md`).
  Without one, creation falls back to the embedded generic document template.
- **Index header** — `docs/templates/index_<type>.md` (the prose above the
  auto-generated table in the type's `README.md`). Without one, `docz` generates
  a generic header from the type's `plural_label`.

**Validation.** Custom types must resolve unambiguously. A duplicate `id_prefix`,
or an `aliases` entry that collides with another type's name, alias, or prefix,
is rejected at startup with a clear error. `docz` also prints a harmless
`config declares non-built-in type "<name>" (typo?)` notice for any type outside
the built-in set, so a genuine typo is easy to spot.

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
  runbook:
    enabled: false           # the one built-in that ships off
    dir: runbook
    id_prefix: RUNBOOK
    id_width: 4
    statuses:
      - Draft
      - Active
      - Needs Review
      - Deprecated

wiki:
  auto_update: true
  mkdocs_path: mkdocs.yml
  plugins:                   # MkDocs plugins for wiki init
    - techdocs-core
  markdown_extensions:       # MkDocs markdown extensions
    - admonition
    - tables
  exclude:
    - templates
    - examples
  nav_titles:
    rfc: "RFCs"
    adr: "ADRs"
    design: "Design"
    impl: "Implementation Plans"
    investigation: "Investigations"
    runbook: "Runbooks"
  # docs_dir: docs           # override MkDocs docs_dir
  # repo_url: https://github.com/org/repo
  # site_url: https://example.com/docs
  # theme: readthedocs

toc:
  enabled: true            # generate ToC during docz update
  min_headings: 3          # minimum headings to generate a ToC

changelog:
  enabled: false           # opt in so tools reading this config find your changelog
  file: CHANGELOG.md       # repo-relative; subpaths work (charts/api/CHANGELOG.md)

api:
  enabled: false           # opt in to publish this repo's docs beyond docz documents
  landing_page: ""         # defaults to <docs_dir>/index.md
  exclude: []              # path prefixes under docs_dir that are never published
  additional_docs: []      # markdown OUTSIDE docs_dir, e.g. CONTRIBUTING.md

sync:
  confluence:
    enabled: false         # opt in to export to Confluence Cloud (see Sync below)
```

Run `docz config` to see the fully resolved configuration.

### Changelog

The `changelog:` block tells tools that read a docz repo — such as a
documentation API or site — where the repo's changelog lives. It is **off by
default** and changes nothing about how the CLI behaves; docz does not
generate changelogs (git-cliff does), it only locates and parses them.

When enabled, `file` must be a clean repo-relative path: absolute paths, `..`
traversal, and trailing slashes are rejected at load time. A disabled block is
never validated, so you can add it before you are ready to turn it on. The
matching parser is `doczcore/document.ParseChangelog`, described in
[Using docz as a Go Library](#using-docz-as-a-go-library).

### API

The `api:` block declares what a documentation API or site should publish for
this repo beyond the docz documents themselves. Like `changelog:`, it is **off
by default**, and no docz command reads it — `docz config` prints it and that
is all. It exists so consumers have one validated, versioned place to look
instead of guessing at a repo's layout.

The governing rule is that **the URL path mirrors the `docs_dir` path**:

| File | Published as |
| ---- | ------------ |
| `docs/index.md` | the repo root |
| `docs/impl/README.md` | `/<owner>/<repo>/impl` |
| `docs/impl/0015-foo.md` | `/<owner>/<repo>/impl/IMPL-0015` |
| `docs/examples/example1.md` | `/<owner>/<repo>/examples/example1.md` |

A directory's `index.md` or `README.md` is that directory's page, which is why
the index tables `docz update` generates *are* the type pages. Everything else
under `docs_dir` is addressed at its `docs_dir`-relative path, so nothing needs
listing.

The four fields:

- **`enabled`** — off by default. A disabled block is never validated, so you
  can commit it before your tooling is ready.
- **`landing_page`** — the repo's front page, repo-relative. Empty resolves to
  `<docs_dir>/index.md`.
- **`exclude`** — path prefixes under `docs_dir` that are never published.
  `<docs_dir>/templates/` is always excluded and need not be listed.
- **`additional_docs`** — markdown *outside* `docs_dir`, such as
  `CONTRIBUTING.md`, each published at its repo-relative path. An entry inside
  `docs_dir` is an error: it is already consumed.

> **Enabling this publishes every `.md` under `docs_dir`**, not just docz
> documents. Look at what is in there first, and use `exclude` for the rest.

When enabled, every path is validated as strictly as `changelog.file`:
absolute paths, `..` traversal, backslashes, and control characters are
rejected at load time, and so is an `additional_docs` entry whose first path
segment would collide with a document type's route. Failures wrap
`config.ErrInvalidAPIPath`.

Files listed in `additional_docs` have no frontmatter to take a title from, so
consumers derive one from the document's H1 via `doczcore/docparse.Title`.

### Sync

The `sync:` block points `docz export confluence`, and docz-api's export, at
a Confluence Cloud space. Like `api:` it is **off by default**, and a
disabled block is never validated:

```yaml
sync:
  confluence:
    enabled: true
    site: https://example.atlassian.net   # the Cloud site; not a secret
    space: DOCZ                           # space key; several repositories may share one
    layout: folder                        # folder (default) | page
    folder: ""                            # the folder's title; empty: the repository's name
    parent: ""                            # optional under folder; the root page under page
    types: []                             # empty: every enabled type
    exclude: [examples]                   # prefixes under docs_dir, as api.exclude
    api_pages: false                      # also export the api: landing page and additional_docs
    mermaid:
      viewer: auto                        # auto | off | <extension key>
```

With **`layout: folder`** (the default, DESIGN-0021) the export builds one
Confluence folder per repository at the top of the space, or under
`parent` when it is set, and prefixes every title with the folder's name so
repositories sharing a space never collide:

- **The repository's page**, titled with the folder's name alone, carries the `api:`
  landing page as its body when `api_pages` is on, otherwise one line
  naming the repository.
- **A page per type** (`<folder>: RFCs`), titled with the type's nav
  title, carries that type's README index. Its table links to the pages
  beneath it.
- **One page per document** (`<folder>: RFC-0001: Title`) sits under its
  type page. A banner names the source file and links to it on GitHub.
- **The `additional_docs`** sit in the folder when `api_pages` is on.

**`layout: page`** is the IMPL-0023 shape for the CLI alone: everything
under the existing `parent` page, titles unprefixed. docz-api refuses it.

A relative link to another exported file becomes a link to its page. A link
to any other file that exists becomes a GitHub blob URL at the default
branch. A link nothing can place prints as an `unresolved link:` line.

Credentials come from the environment and never from the file:

```bash
export ATLASSIAN_EMAIL=you@example.com
export ATLASSIAN_API_TOKEN=…   # never commit this
docz export confluence --dry-run
docz export confluence
```

Prefer a **scoped API token**. Create one at
<https://id.atlassian.com/manage-profile/security/api-tokens> with "Create
API token with scopes", choose Confluence, and grant these six scopes:

- `read:space:confluence`
- `read:page:confluence`
- `write:page:confluence`
- `read:folder:confluence`
- `write:folder:confluence`
- `read:hierarchical-content:confluence`

No delete scope is needed: nothing is ever deleted. Every request goes
through Atlassian's gateway (`api.atlassian.com/ex/confluence/<cloudId>`),
which accepts both scoped and unscoped tokens. An unscoped token therefore
works too. A token Atlassian rejects exits 2, with the response body, which
for a scoped token names the scope it lacks.

Mermaid fences render through Atlassian Labs' **Mermaid diagrams viewer**
app, which a site admin must install from the Marketplace. Each diagram
draws from its source, kept beneath it in a collapsed "Diagram source"
expand. On a site without the app, set `mermaid.viewer: off` and the
fences export as plain code blocks instead.

Each page and folder carries a `docz` content property recording the
document id, the repository, its source, the hash of its rendered body,
and the page version docz wrote. The CLI finds each page by its title;
docz-api finds it by the page id it recorded, so a renamed document's page
is renamed in place. On every run a page takes one of these actions:

- **Unchanged:** the hash and the parent match, so nothing is written.
- **Updated:** the document changed, so a new version is written. Inline
  comments readers left on the page are carried into the new version
  wherever their text survives; one whose text is gone is named in a
  `WARNING: … lost its anchor` line on stderr.
- **Skipped:** somebody edited the page in Confluence since docz wrote it
  (named in a `WARNING: … was edited in Confluence` line), or the page has
  the right title but docz did not create it. `--force` overwrites the
  edit, or adopts the page. A page or folder **another repository** wrote
  is never touched, `--force` included, and is named in a `WARNING` line. A
  skip exits 0 unless `--strict` is set.
- **Archived:** a full run (no `--type`, no ids) found a page whose
  document is gone, and moved it under `<folder>: Archive`. Nothing is
  ever deleted. A document that comes back is moved back.

Content only flows out: nothing in Confluence is read back into the
repository. Exit codes are 0 when no page failed, 1 when a page failed (the
rest are still written), and 2 for configuration problems. Those include a
disabled block, missing or rejected credentials, an unknown type, and an id
that matches nothing.

**docz-api** runs the same export after every ingest of an opted-in
repository, with no checkout, when the server is configured with
`CONFLUENCE_SITE`, `CONFLUENCE_EMAIL`, `CONFLUENCE_API_TOKEN`, and
`CONFLUENCE_SPACES` (the chart's `api.confluence.*`). It writes only into a
space on that list, always overwrites edits made in Confluence, and serves
each repository's last export at `GET /api/v1/repos/{owner}/{name}/confluence`.
[RUNBOOK-0003](docs/runbook/0003-enable-confluence-export-on-docz-api.md)
turns it on.

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

The header prose written above the markers when a `README.md` is first created
can be customized per type with a `docs/templates/index_<type>.md` override
(used verbatim). This resolution mirrors the body-template tiers and works for
both built-in and custom types.

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

**Note:** Documents created before v0.0.8 do not include ToC markers. To add
ToC support to existing documents, manually insert the markers between the
metadata block and the first section heading:

```markdown
**Date:** 2026-01-01

<!--toc:start-->
<!--toc:end-->

## First Section
```

Then run `docz update` to populate the ToC.

## Validation

`docz validate` checks each document against the set of regions its type's
schema requires, the content rules of each region kind, and the drift only a
regeneration can see — a stale table of contents, and a README index table
that is not what `docz update` would write. With no type argument every
enabled type is checked.

Findings come from three tiers, all printed in one format:

- **Generic** — marker well-formedness, frontmatter shape, whether a required
  region is present, and the content rules of the kinds the document carries
  (`marker.*`, `frontmatter.*`, `region.*`, `content.*`, `references.*`,
  `toc.*`).
- **Repository** — index drift, which is `docz update`'s work rather than an
  author's (`index.drift`).
- **Per-type** — the rules only a typed model can check, each prefixed with
  the type name (`adr.decision.empty`, `impl.phase.no-tasks`,
  `inv.conclusion.no-answer`).

Every finding prints as `path:line code detail`. The **code is the stable
part** — filter on it in CI; the wording after it may change.

```bash
docz validate                  # every enabled type
docz validate adr              # only ADRs
docz validate --format json    # one JSON report, for CI
docz validate --strict         # warnings and index drift fail too
docz validate --fix            # mark what inference found, then re-report
```

A run over this repository's own ADRs:

```text
docs/adr/0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md:507 references.no-link reference has no link: **INV-0006** — per-package core requirements audit (docz C…
docs/adr/0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md:509 references.no-link reference has no link: **IMPL-0014** — the implementation plan for this ADR (all …
docs/adr/0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md:513 references.no-link reference has no link: **ADR context / prior art** — DESIGN-0007 (`pkg/doczcore` …
```

Those are warnings, so the command exits `0`. Adding `--strict` prints the
same lines and then fails with a one-line count:

```text
Error: 0 errors, 5 warnings
```

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Nothing found, or warnings only and no `--strict` |
| `1` | Errors were found, or anything was found with `--strict` |
| `2` | The command was used wrongly (unknown type, bad `--format`) |

Errors always fail. Warnings and index drift fail **only** under `--strict`,
which is what makes that flag the CI gate: a stale ToC and a drifted index
are `docz update`'s work, so someone running `docz validate` by hand sees
them without being stopped by them.

### Fixing Documents

`docz validate --fix` writes two things and nothing else:

- **Region markers** for documents whose regions were read by *inference*
  from their headings — the `region.inferred` warning — so the spans become
  explicit `<!--docz:<kind>:start-->` / `<!--docz:<kind>:end-->` pairs.
- **Canonical marker spellings** for markers `docz` already read leniently —
  the `marker.spelling` warning.

A section the document does not have is never invented, so `--fix` cannot
make a document valid on its own. It re-validates afterwards and prints
what is left for you to fix by hand; the exit code is the second pass's. One
line is printed per document it changed, naming the kinds it marked and the
count of markers it canonicalized, and nothing at all for a document it left
alone. A second `--fix` over the same repository writes nothing.

This is the migration path for documents written before the schema existed.
It is not required: inference is permanent, so a repository that never runs
`--fix` keeps validating and parsing forever. Documents created by
`docz create` are marked from birth and need it never.

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
  plugins:                   # MkDocs plugins written by wiki init
    - techdocs-core
  markdown_extensions:       # MkDocs markdown extensions
    - admonition
    - tables
  exclude:                   # directories excluded from nav
    - templates
    - examples
  nav_titles:                # override directory display names
    rfc: "Request for Comments"
  docs_dir: docs             # MkDocs docs_dir
  repo_url: https://github.com/org/repo  # repository URL
  site_url: https://example.com/docs     # published site URL
  theme: readthedocs         # MkDocs theme
```

All optional fields (`plugins`, `markdown_extensions`, `docs_dir`, `repo_url`,
`site_url`, `theme`) are only written to `mkdocs.yml` during `wiki init` when
set in the config. `wiki update` only modifies the `nav` section — manual edits
to other fields are preserved.

### Homepage Template

`docz wiki init` generates `docs/index.md` from an embedded template. To
customize the homepage, place a template at `docs/templates/wiki_index.md`.
The template uses Go `text/template` syntax with these variables:

| Variable | Description |
|----------|-------------|
| `{{ .SiteName }}` | Site name from `--site-name` flag or repo directory |
| `{{ .Types }}` | Slice of enabled types, each with `.Name`, `.NavTitle`, `.Dir` |

Only enabled types (those with `enabled: true` in config) are included.

### Nav Generation

- Docz documents use their frontmatter title (e.g., "RFC-0001: API Rate Limiting")
- Other markdown files use their first H1 heading or filename
- `wiki init` sorts sections alphabetically
- `wiki update` preserves existing section order, appending new sections at the end
- README.md / index.md files become "Overview" entries
- Empty directories and excluded directories are skipped

## Using docz as a Go Library

Since v1.0.0 the parsing and writing core has been a public, semver-governed
Go API. On the v2 line that surface is the whole of docz: eighteen packages
under `pkg/`, and every `docz` command is one call into them plus printing:

```bash
go get github.com/donaldgifford/docz/v2@latest   # the v2 line
go get github.com/donaldgifford/docz@v1.2.2      # the last v1 release
```

The module path carries the major version: the v2 line is
`github.com/donaldgifford/docz/v2`, and every v1.x tag keeps the
unversioned `github.com/donaldgifford/docz`. A consumer that pins v1 is
unaffected by the v2 work and can stay there.

The packages are layered: facts, then mutation, then interpretation, then
whole-repository operations. A layer imports only the layers beneath it.

| Package | What it provides |
|---------|------------------|
| `pkg/doczcore/config` | `.docz.yaml` loading, validation, type resolution, and the `changelog:` / `api:` declarations (`Load`, `Validate`, `EnabledTypes`) |
| `pkg/doczcore/document` | Frontmatter parsing, directory scanning, and changelog parsing (`ParseFrontmatter`, `ScanDocuments`, `ParseChangelog`) |
| `pkg/doczcore/docparse` | Markdown facts: headings with GitHub anchor slugs, checkbox task items, list items, pipe tables, the document title, and the `<!--docz:…-->` region spans (`Headings`, `TaskItems`, `Title`, `Regions`) |
| `pkg/doczcore/docwrite` | The write side: `Create` from templates, byte-preserving `SetStatus`, checkbox `SetTaskState`, and a `…Bytes` core for each so a consumer holding bytes never needs a path |
| `pkg/doczcore/toc` | ToC generation and marker splicing over `docparse` facts (`UpdateToC`, `UpdateFiles`) |
| `pkg/doczcore/kinds` | Region spans to typed fields: the readers every document type shares, plus heading-based inference for documents written before markers (`ResolveRegions`, `RegionBytes`, `OpenQuestions`) |
| `pkg/doczcore/validate` | The generic validator — markers, frontmatter, required regions, per-kind content rules, ToC freshness. It never fails; it returns findings (`Document`, `SchemaFromMarkers`) |
| `pkg/rfc` | An RFC as a typed value: summary, problem, proposal, alternatives, and the risks table (`Parse`, `Validate`) |
| `pkg/adr` | An ADR as a typed value: context, decision, and consequences split positive / negative / neutral (`Parse`, `Validate`) |
| `pkg/design` | A design doc as a typed value: overview, goals, detailed design, decisions, and open questions with their options (`Parse`, `Validate`) |
| `pkg/impl` | An implementation plan as a typed value: phases, tasks with byte-accurate lines to hand straight back to `docwrite`, and per-phase acceptance criteria (`Parse`, `Validate`) |
| `pkg/investigation` | An investigation as a typed value: question, approach, findings, and a conclusion whose answer is also read as a `Verdict` (`Parse`, `Validate`) |
| `pkg/runbook` | A runbook as a typed value: the Last Verified row, procedures and scenarios, and ordered steps with stable IDs, their commands, and what each should show (`Parse`, `Validate`) |
| `pkg/doczcore/doctemplate` | Template and schema resolution (config path → repo override → embedded) and rendering (`Resolve`, `ResolveSchema`, `Render`) |
| `pkg/doczcore/index` | The README index table and the splice between its markers, the latter as a pure function (`GenerateTable`, `Splice`, `UpdateReadme`) |
| `pkg/doczcore/repo` | Whole-repository operations with typed reports and typed errors — what each `docz` command is one call to (`Open`, `Create`, `Update`, `Validate`, `Find`) |
| `pkg/wiki` | MkDocs / Backstage TechDocs integration: write `mkdocs.yml`, then rebuild its nav from the docs tree (`Init`, `UpdateNav`) |
| `pkg/export/confluence` | Confluence Cloud export: a document to storage format with no I/O, a v2 REST client through Atlassian's gateway, and the page-tree reconcile `docz export confluence` is one call to (`Render`, `NewHTTPClient`, `Export`). The one package allowed a third-party markdown parser, goldmark |

> **Stability.** The five packages promoted at v1.0.0 — `config`, `document`,
> `docparse`, `docwrite`, and `toc` — are **frozen** and take additions only
> (ADR-0001 Decision 6); the one break the v2 line makes to them is `plan`
> leaving `DocTypeNames()` (ADR-0003). The other thirteen are
> **experimental until v2.0.0 proper ships** (ADR-0002 Decision 7) and may
> change between `v2.0.0-beta.N` tags. Pin a beta exactly if you depend on
> them.

Semver covers exported identifiers under `pkg/` only; `cmd/`, CLI output
text, and embedded template contents are not part of the contract. There is
no `internal/` left. See `go doc` on each package for the full API.

## Task Runner Integration

After `docz init`, `docz.just` in this repository includes convenience recipes:

```bash
just docs-init    # docz init
just docs-update  # docz update (all types)
just docs-list    # docz list
just docs-config  # docz config
```

## Author Resolution

`docz create` resolves the document author in this order:

1. `--author` flag
2. `author.default` in `.docz.yaml`
3. `git config user.name`
4. `"Unknown"`

## docz-api, the server

This repository also carries `docz-api`, the HTTP service that ingests docz
repositories and serves them to docz-site (ADR-0004, DESIGN-0016). It builds
from `cmd/docz-api/`, its recipes are a just module, and its image is built from
`Dockerfile.api`.

```sh
just api build                # binary at build/bin/docz-api
just api test                 # race detector
just api run                  # build + run the binary
```

The image is distroless and nonroot, published to
`ghcr.io/donaldgifford/docz-api`; the Helm chart is `charts/docz/`, which
deploys the API, the site, and the API's backends together. Running
it locally, with compose, webhooks, and the monitoring stack, is covered in
[DEVELOPMENT.md](DEVELOPMENT.md#developing-docz-api-the-server).

## docz-site, the frontend

docz-site, the web UI that reads docz-api, lives in `ui/` (ADR-0004,
DESIGN-0017). It is a Bun, Vite, and React project whose recipes are the
`ui` just module; its image is built from `Dockerfile.ui`.

```sh
mise install                  # pinned toolchain, Bun and Node included
just ui install               # dependencies
just ui dev                   # dev server, proxied to a docz-api on :8080
just ui dev-msw               # the same app against mock fixtures, no docz-api needed
just ui ci                    # everything the ui CI job runs
```

Its typed client is generated from the same `api/openapi.yaml` docz-api
serves, so there is one spec. The image is published to
`ghcr.io/donaldgifford/docz-site`, and both images deploy with **one chart**,
[`charts/docz/`](charts/docz/README.md), published from the same tag as
`oci://ghcr.io/donaldgifford/charts/docz`.
[ui/README.md](ui/README.md) covers the rest.
