# Canonical mkdocs.yml

The shape this skill writes.

## Default `mkdocs.yml`

```yaml
site_name: {{ .SiteName }}
site_description: {{ .SiteDescription }}
site_url: {{ .SiteURL }}
repo_url: {{ .RepoURL }}
edit_uri: blob/main/docs/

theme:
  name: material
  features:
    - navigation.instant
    - navigation.tracking
    - navigation.tabs
    - navigation.sections
    - navigation.expand
    - navigation.indexes
    - navigation.top
    - search.suggest
    - search.highlight
    - content.code.copy
  palette:
    - media: "(prefers-color-scheme: light)"
      scheme: default
      primary: indigo
      toggle:
        icon: material/brightness-7
        name: Switch to dark mode
    - media: "(prefers-color-scheme: dark)"
      scheme: slate
      primary: indigo
      toggle:
        icon: material/brightness-4
        name: Switch to light mode

plugins:
  - search
  - mermaid2

markdown_extensions:
  - admonition
  - pymdownx.details
  - pymdownx.superfences:
      custom_fences:
        - name: mermaid
          class: mermaid
          format: !!python/name:mermaid2.fence_mermaid_custom
  - pymdownx.tabbed:
      alternate_style: true
  - toc:
      permalink: true

nav:
  - Home: index.md
  # nav is generated; see `docz wiki update`
```

## Requirements

`requirements.txt`:

```
mkdocs>=1.5
mkdocs-material>=9.5
mkdocs-mermaid2-plugin>=1.1
```

For Backstage TechDocs, add:

```
mkdocs-techdocs-core>=1.4
```

## Workflow: `.github/workflows/wiki.yml`

```yaml
name: Wiki

on:
  push:
    branches: [main]
    paths:
      - 'docs/**'
      - 'mkdocs.yml'

permissions:
  contents: write

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: '3.12'
      - run: pip install -r requirements.txt
      - run: mkdocs gh-deploy --force
```

## Nav generation contract

`docz wiki update` regenerates *only* the `nav:` block.
The skill:

1. Walks `docs/` recursively.
2. Groups files by their top-level directory (matching
   docz doc types: `adr/`, `rfc/`, `design/`, `impl/`,
   `investigation/`, `runbook/`).
3. Applies `wiki.nav_titles` overrides from `.docz.yaml`
   for display names.
4. Respects `wiki.exclude` to skip directories.
5. Sorts files within each group by filename (matches
   docz's numbered prefix convention).
6. Writes back without touching any other field in
   `mkdocs.yml`.

This preserves hand-edited theme/plugins/markdown_extensions.

## Edge cases

- **No `docs/index.md`?** The skill writes one referencing
  the project README.
- **Mermaid blocks?** Already enabled via `mermaid2`
  plugin — no extra config needed.
- **Versioned docs?** Out of scope for v1. Use `mike` if
  you need versioning; the skill won't fight you.
