# Configuration Reference

Docz is configured via `.docz.yaml` at the repository root. A global config at `~/.docz.yaml` provides defaults that repo-level config overrides per-key.

See `plugins/docz/example.docz.yaml` for a complete working example.

## Config Precedence

```
.docz.yaml (repo)        highest priority
~/.docz.yaml (global)    author defaults, personal overrides
built-in defaults        lowest priority
```

Run `docz config` to see the fully resolved configuration.

## Root Fields

```yaml
# Base directory for all documentation (default: docs)
docs_dir: docs

# Author resolution
author:
  from_git: true    # use git config user.name when --author not specified
  default: ""       # fallback if git config is not set

# README index generation
index:
  auto_update: true       # regenerate README tables on create/update
  preserve_header: true   # keep content above the auto-generated section
```

## Per-Type Configuration

Each type is configured under `types`. The config key for INV is `investigation`.

```yaml
types:
  rfc:
    enabled: true
    dir: rfc
    template: ""          # path to custom template; empty uses built-in
    id_prefix: "RFC"
    id_width: 4           # RFC-0001, RFC-0002, ...
    status_field: "status"
    statuses:
      - Draft
      - Proposed
      - Accepted
      - Rejected
      - Superseded

  adr:
    enabled: true
    dir: adr
    template: ""
    id_prefix: "ADR"
    id_width: 4
    status_field: "status"
    statuses:
      - Proposed
      - Accepted
      - Deprecated
      - Superseded

  design:
    enabled: true
    dir: design
    template: ""
    id_prefix: "DESIGN"
    id_width: 4
    status_field: "status"
    statuses:
      - Draft
      - In Review
      - Approved
      - Implemented
      - Abandoned

  impl:
    enabled: true
    dir: impl
    template: ""
    id_prefix: "IMPL"
    id_width: 4
    status_field: "status"
    statuses:
      - Draft
      - In Progress
      - Completed
      - Paused
      - Cancelled

  investigation:          # CLI type arg is "inv"; config key is "investigation"
    enabled: true
    dir: investigation
    template: ""
    id_prefix: "INV"
    id_width: 4
    status_field: "status"
    plural_label: "Investigations"
    statuses:
      - Open
      - In Progress
      - Concluded
      - Inconclusive
      - Abandoned

  runbook:                # CLI type arg is "runbook" or "rb"
    enabled: false        # ships disabled; see "Enable RUNBOOK" below
    dir: runbook
    template: ""
    id_prefix: "RUNBOOK"
    id_width: 4
    status_field: "status"
    plural_label: "Runbooks"
    statuses:
      - Draft
      - Active
      - Needs Review
      - Deprecated
```

There is no `plan` type in docz v2. A repository that keeps a `types.plan` block gets a **custom** type named `plan`, so its documents keep working; see Custom Document Types.

## Status Field

`status_field` specifies the YAML frontmatter key used to track status. Defaults to `"status"`. Changing this is rarely needed. Mutate the value with `docz status set` rather than editing frontmatter by hand.

## Plural Label

`plural_label` is the display name used for the type in generated index headers and wiki nav (e.g. `ADRs`, `Implementation Plans`). Defaults to a title-cased form of the type name.

## Template Field

`template` is a path to a custom template file. Empty string (`""`) uses the built-in template for that type. Resolution order:

1. **Config path** — `types.<type>.template`
2. **Local override** — `<docs_dir>/templates/<type>.md`
3. **Embedded default** — built into the binary; a type with none (a custom type) gets the embedded **generic** template

```bash
docz template override adr   # copies built-in to docs/templates/adr.md
docz template show adr       # print the resolved template
```

Then set `template: docs/templates/adr.md` in `.docz.yaml`, or leave it empty — docz detects the override automatically. Template resolution ignores `enabled`, so `docz template show` works on a disabled type.

### Index header overrides

The prose *above* the auto-generated table in a type's `README.md` comes from `<docs_dir>/templates/index_<type>.md` if present, using the same tiers as body templates. Without one, docz generates a header from the type's `plural_label`. Works for built-in and custom types alike.

## Custom Document Types

Beyond the six built-ins you can define your own types entirely in `.docz.yaml` — no rebuild needed:

```yaml
types:
  frameworks:
    enabled: true
    dir: frameworks
    template: ""
    id_prefix: "FW"           # must be unique across all types
    id_width: 4
    aliases: ["fw"]           # optional CLI shorthands
    status_field: "status"
    plural_label: "Frameworks"
    statuses:
      - Draft
      - Active
      - Deprecated
```

Once declared, the type behaves like a built-in — `create`, `list`, `update`, and `status set` all accept it:

```bash
docz create frameworks "Service Mesh"   # by canonical name
docz create FW "Service Mesh"           # by id_prefix
docz create fw "Service Mesh"           # by alias
docz status set FW FW-0001 Active       # aliases work wherever a type is accepted
```

**Resolution precedence** — a type word is matched case-insensitively as **canonical name → alias → `id_prefix`**, so a canonical name always wins over a colliding alias or prefix. A duplicate `id_prefix`, or an alias that collides with another type's name, alias, or prefix, is rejected when the config loads.

**A custom type needs no template file.** With none at `<docs_dir>/templates/<name>.md` (named after the type's **canonical name**), docz renders the embedded generic template and says so on stderr:

```
note: frameworks has no template; used the generic one. Run "docz template override frameworks" to write one.
```

`docz template override <custom type>` scaffolds that generic template and its marker schema into `docs/templates/` to customise. A `template:` path in the config that names a missing file is still an error.

**Harmless warning.** docz prints `Warning: config declares non-built-in type "<name>" (typo?)` on every command for any type outside the built-in set. It is informational — it exists so a genuine typo is easy to spot — and does not affect the exit code.

**Built-in aliases**: `implementation` → `impl`, `inv` → `investigation`, `rb` → `runbook`.

If a user names a type you do not recognize, run `docz config` and check the `types:` block before assuming it is a typo.

## Changelog Block

Declares where the repo's changelog lives, for tools that read a docz repo (docz-api, docz-site). Added in docz v1.1.0.

```yaml
changelog:
  enabled: false        # dormant by default
  file: CHANGELOG.md    # repo-relative; subpaths allowed (charts/api/CHANGELOG.md)
```

No docz command reads it — `docz config` prints it and that is all. **docz does not generate changelogs**; git-cliff does. This block only declares where the file lives so consumers can find and parse it.

A disabled block is never validated, so you can commit it before your tooling is ready.

## API Block

Declares what a documentation API or site should publish for this repo beyond the docz documents themselves. Added in docz v1.2.0.

```yaml
api:
  enabled: false          # dormant by default
  landing_page: ""        # defaults to <docs_dir>/index.md
  exclude: []             # path prefixes under docs_dir that are never published
  additional_docs: []     # markdown OUTSIDE docs_dir, e.g. CONTRIBUTING.md
```

> **Enabling this publishes every `.md` under `docs_dir`**, not just docz documents. Look at what is in there first and use `exclude` for anything that should stay private.

The governing rule is that **the URL path mirrors the `docs_dir` path**:

| File | Published as |
|------|--------------|
| `docs/index.md` | the repo root |
| `docs/impl/README.md` | `/<owner>/<repo>/impl` |
| `docs/impl/0015-foo.md` | `/<owner>/<repo>/impl/IMPL-0015` |
| `docs/examples/example1.md` | `/<owner>/<repo>/examples/example1.md` |

A directory's `index.md` or `README.md` is that directory's page — which is why the index tables `docz update` generates *are* the type pages.

The fields:

- **`enabled`** — off by default; a disabled block is never validated
- **`landing_page`** — the repo's front page, repo-relative; empty resolves to `<docs_dir>/index.md`
- **`exclude`** — path prefixes under `docs_dir` never published. `<docs_dir>/templates/` is always excluded and need not be listed
- **`additional_docs`** — markdown **outside** `docs_dir`, each published at its repo-relative path. An entry inside `docs_dir` is an error: `is already under docs_dir "docs", which is consumed automatically`

Files in `additional_docs` have no frontmatter, so consumers derive their title from the document's H1 — give them a real one.

## Sync Block

Configures `docz export confluence` (and docz-api's export, when the server has a Confluence credential). Dormant and never validated until enabled.

```yaml
sync:
  confluence:
    enabled: false
    site: ""              # https://<name>.atlassian.net, no path
    space: ""             # the space key; several repos may share one
    layout: folder        # folder (one folder per repo, titles prefixed) or page
    parent: ""            # the page the folder sits under; required for layout: page
    types: []             # empty = every enabled type except impl
    exclude: []           # path prefixes under docs_dir never exported
    api_pages: false      # also export the api: landing page and additional docs
    mermaid:
      viewer: auto        # auto, off, or an extension key
```

Credentials are **never** in the file: `ATLASSIAN_EMAIL` and `ATLASSIAN_API_TOKEN` come from the environment. IMPLs are opt-in: list `impl` in `types` to export them. Run `docz export confluence --dry-run` before the first real export.

## Access Block

Who may read the repository through docz-api. Always present; the default is every signed-in reader.

```yaml
access:
  scopes:
    - public:read
```

A scope is `<audience>:read`; a team audience such as `docs-team:read` hides the repository from anyone without that scope. The `write` action is reserved and rejected (the one exception is `confluence:write`, which lets the export run on a repository whose scopes grant nothing public). No local docz command reads this block.

## HTML Block

The theme `docz export html` pages and the docz-review page render in. Always present.

```yaml
html:
  theme: paper    # paper (light and dark), charcoal (dark), or classic (dark)
```

An unknown name is a **warning**, not an error, and falls back to `paper`. `docz export html --theme <name>` overrides it for one run.

## Path Validation

`changelog.file` and all three `api:` path fields are validated as clean repo-relative paths when their block is enabled. Both blocks are dormant while `enabled: false`, so an invalid path in a disabled block loads fine.

| Input | Error |
|-------|-------|
| `/etc/CHANGELOG.md` | must be relative to the repo root |
| `../CHANGELOG.md` | must not traverse outside the repo root |
| `charts\api\CHANGELOG.md` | must use forward slashes to separate directories |
| `docs /CHANGELOG.md` | must not have a path component ending in a space or period |
| `docs./index.md` | same — Windows trims trailing dots, so `docs...` would resolve as `..` |
| `docs/inside.md` in `additional_docs` | already under `docs_dir`, which is consumed automatically |

API path failures wrap `config.ErrInvalidAPIPath`.

## Common Customizations

### Enable RUNBOOK

RUNBOOK ships disabled. Turn it on in the existing `types:` block:

```yaml
types:
  runbook:
    enabled: true
```

A short `runbook: {enabled: true}` gets every other field from the defaults. **Add it to the `types:` block rather than replacing the block with it**: a `types:` block that lists only `runbook` switches the other five built-ins off. Then run `docz init` to scaffold `docs/runbook/` with its index README.

### Disable unused types

```yaml
types:
  investigation:
    enabled: false
```

A disabled type is skipped by `docz init`, no-arg `docz update`, and wiki nav
generation. `docz create` on it errors; `docz list` returns an empty table
rather than an error; `docz template show` still prints its template, since
template resolution ignores `enabled`.

### Custom statuses

```yaml
types:
  adr:
    statuses:
      - Draft
      - Under Review
      - Accepted
      - Rejected
      - Superseded
```

### Narrower ID width

```yaml
types:
  adr:
    id_width: 3   # ADR-001 instead of ADR-0001
```

### Custom docs directory

```yaml
docs_dir: documentation
```

## Global Config (`~/.docz.yaml`)

Personal defaults applied across all repos:

```yaml
author:
  from_git: true
  default: "Jane Smith"
```

## CLI Overrides

Six global flags, accepted by every subcommand:

| Flag | Purpose |
|------|---------|
| `--config <path>` | explicit config file (default: `.docz.yaml` in the repo root) |
| `--docs-dir <path>` | override the base documentation directory |
| `--repo-root <path>` | repo root to scan for `.docz.yaml` and scaffold against (default: working directory) |
| `--verbose` | shorthand for `--log-level=debug` |
| `--log-level <level>` | `debug`, `info`, `warn`, `error` — overrides `--verbose` |
| `--log-format <fmt>` | `text` or `json` |

```bash
docz --config /path/to/custom.yaml create adr "Title"
docz --docs-dir ./documentation create rfc "Title"
docz --verbose config

# Run from a subdirectory or a worktree without cd-ing first
docz --repo-root /path/to/repo list

# Structured logs for CI
docz --log-format json --log-level debug update --dry-run
```

`--repo-root` is the one to reach for when the working directory is not the repo root.

## Wiki Configuration

The `wiki` section controls MkDocs/Backstage TechDocs integration:

```yaml
wiki:
  auto_update: true           # update mkdocs.yml nav automatically on docz create
  mkdocs_path: mkdocs.yml     # path to the mkdocs.yml file
  exclude:                    # directories under docs/ to skip in nav generation
    - templates
    - examples
  nav_titles:                 # custom display names for doc type sections in nav
    adr: ADRs
    design: Design
    impl: Implementation Plans
    investigation: Investigations
    rfc: RFCs
    runbook: Runbooks
```

### Wiki fields

- **`auto_update`** — when `true`, `docz create` automatically rebuilds the `nav` section of `mkdocs.yml`
- **`mkdocs_path`** — path to the MkDocs config file (default: `mkdocs.yml` at repo root)
- **`exclude`** — list of directory names under `docs/` to skip when building navigation
- **`nav_titles`** — map of directory names to display names in the MkDocs nav; keys match the `dir` field in each type's config

## Auto-Generated Index Sections

Docz maintains bounded sections in README files:

```markdown
# Architecture Decision Records

Content above the markers is preserved when preserve_header is true.

<!-- BEGIN DOCZ AUTO-GENERATED -->
## All ADRs

<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
| ID | Title | Status | Date | Author | Link |
| --- | --- | --- | --- | --- | --- |
| ADR-0001 | Use PostgreSQL | Accepted | 2026-03-04 | Jane Smith | [0001-use-postgresql.md](0001-use-postgresql.md) |
<!-- markdownlint-restore -->
<!-- END DOCZ AUTO-GENERATED -->
```

Never hand-edit between the `BEGIN`/`END` markers — run `docz update` instead.
