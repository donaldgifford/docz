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

  plan:
    enabled: false        # opt-in since docz v1.1.1; init does not scaffold docs/plan/
    dir: plan
    template: ""
    id_prefix: "PLAN"
    id_width: 4
    status_field: "status"
    plural_label: "Plans"
    statuses:
      - Draft
      - In Progress
      - Completed
      - Cancelled

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
```

## Status Field

`status_field` specifies the YAML frontmatter key used to track status. Defaults to `"status"`. Changing this is rarely needed. Mutate the value with `docz status set` rather than editing frontmatter by hand.

## Plural Label

`plural_label` is the display name used for the type in generated index headers and wiki nav (e.g. `ADRs`, `Implementation Plans`). Defaults to a title-cased form of the type name.

## Template Field

`template` is a path to a custom template file. Empty string (`""`) uses the built-in template for that type. Resolution order:

1. **Config path** — `types.<type>.template`
2. **Local override** — `<docs_dir>/templates/<type>.md`
3. **Embedded default** — built into the binary (built-in types only)

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

**A custom type requires a template file.** Unlike the built-ins, custom types have no embedded default. Create `<docs_dir>/templates/<name>.md` — named after the type's **canonical name**, so `frameworks` needs `docs/templates/frameworks.md` — before running any command that resolves the type. Without it every command fails with:

```
Error: creating frameworks document: resolving template: no embedded template
for type "frameworks": open templates/frameworks.md: file does not exist
```

`docz template override` only works for built-in types, so start from an existing template:

```bash
mkdir -p docs/templates
docz template show design > docs/templates/frameworks.md   # then edit
```

> The upstream docz README states that a custom type without a template "falls back to the embedded generic document template." Verified against v1.2.0–v1.2.2, it does not — there is no generic embedded template. Tracked as [donaldgifford/docz#92](https://github.com/donaldgifford/docz/issues/92). The same inaccurate comment appears in `docz init` output (and therefore in `example.docz.yaml`, which is a verbatim copy).

**Harmless warning.** docz prints `Warning: config declares non-built-in type "<name>" (typo?)` on every command for any type outside the built-in set. It is informational — it exists so a genuine typo is easy to spot — and does not affect the exit code.

**Built-in aliases**: `implementation` → `impl`, `inv` → `investigation`.

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

### Enable PLAN

PLAN ships disabled (docz v1.1.1+). Turn it on explicitly:

```yaml
types:
  plan:
    enabled: true
```

Then run `docz init` to scaffold `docs/plan/` with its index README.

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
    plan: Plans
    rfc: RFCs
```

### Wiki fields

- **`auto_update`** — when `true`, `docz create` automatically rebuilds the `nav` section of `mkdocs.yml`
- **`mkdocs_path`** — path to the MkDocs config file (default: `mkdocs.yml` at repo root)
- **`exclude`** — list of directory names under `docs/` to skip when building navigation
- **`nav_titles`** — map of directory names to display names in the MkDocs nav; keys match the `dir` field in each type's config

## Auto-Generated Index Sections

Docz maintains bounded sections in README files:

```markdown
## ADRs

<!-- BEGIN DOCZ AUTO-GENERATED -->
| ID | Title | Status | Date |
|----|-------|--------|------|
| ADR-0001 | Use PostgreSQL | Accepted | 2024-01-15 |
<!-- END DOCZ AUTO-GENERATED -->

Content here is preserved when preserve_header is true.
```

Never hand-edit between the `BEGIN`/`END` markers — run `docz update` instead.
