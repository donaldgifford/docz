# Backstage TechDocs

Backstage TechDocs is MkDocs + a few extra parts. This
skill scaffolds the MkDocs side and **detects** Backstage
catalog state — it does not write `catalog-info.yaml`.

## What the skill writes

`mkdocs.yml` with the `techdocs-core` plugin:

```yaml
site_name: {{ .SiteName }}
plugins:
  - techdocs-core   # the magic ingredient

# ...the rest matches the standard mkdocs config
```

`requirements.txt` adds:

```
mkdocs-techdocs-core>=1.4
```

That's it on the docs side. TechDocs handles the rest at
publish time — Backstage's `@backstage/plugin-techdocs`
backend invokes `mkdocs build` against the repo and serves
the output.

## What the skill detects

After writing the MkDocs config, the skill looks for
`catalog-info.yaml` at the repo root and verifies the
TechDocs annotation:

```yaml
apiVersion: backstage.io/v1alpha1
kind: Component
metadata:
  name: my-service
  annotations:
    backstage.io/techdocs-ref: dir:.    # docs at repo root
spec:
  type: service
  owner: my-team
```

**Detection outcomes:**

| Catalog state | Skill behavior |
|---|---|
| `catalog-info.yaml` present, `techdocs-ref` correct | Report ✓ "TechDocs catalog ready" |
| `catalog-info.yaml` present, `techdocs-ref` missing | Warn: add the annotation manually |
| `catalog-info.yaml` present, `techdocs-ref` wrong path | Warn with the expected value |
| `catalog-info.yaml` absent | Warn: Backstage catalog import is a separate manual step |

## Why the skill doesn't write `catalog-info.yaml`

Catalog ownership lives with the platform/team running
Backstage, not with the docs tool. The catalog file:

- Declares component **kind** (service, library, website,
  resource, etc.) — domain decision.
- Declares **ownership** (a team) — org decision.
- Often references **other entities** (system, domain,
  API specs) — cross-team coupling.

A docs skill that wrote a default `catalog-info.yaml`
would either guess wrong on these or scatter
near-correct catalog files across the org. The platform
team imports the catalog; the docs skill ensures the
TechDocs side is ready when they do.

## Backstage TechDocs reference

| Resource | Use |
|---|---|
| [TechDocs overview](https://backstage.io/docs/features/techdocs/techdocs-overview) | What TechDocs is, how it integrates |
| [`techdocs-core` mkdocs plugin](https://backstage.io/docs/features/techdocs/configuration#supported-options) | Configuration knobs |
| [Catalog YAML schema](https://backstage.io/docs/features/software-catalog/descriptor-format) | `catalog-info.yaml` reference |
| [TechDocs `techdocs-ref` formats](https://backstage.io/docs/features/techdocs/configuring-ci-cd#techdocs-ref-annotation) | `dir:.`, `url:`, etc. |

## Monorepo note

For monorepos with multiple components under different
sub-trees, each component's `catalog-info.yaml` should
have its own `techdocs-ref` pointing at its sub-tree. This
skill operates on one directory at a time — re-run per
component.

## Publish path

TechDocs publishes through one of:

- **Local publisher** (default in dev) — builds and serves
  from the Backstage host's filesystem.
- **Cloud publisher** (S3, GCS, Azure Blob) — recommended
  for prod. Configured in the Backstage app config, not
  here.
- **External publisher** (`mkdocs gh-deploy`) — bypasses
  TechDocs publishing entirely and serves from
  GitHub Pages. Falls back to the standard `wiki.yml`
  workflow.

This skill defaults to the **local/cloud publisher** path
(no extra workflow); the team's Backstage instance handles
the build trigger. If your Backstage instance is configured
for the external publisher, you'll need the standard
`wiki.yml` workflow as well — re-run `docz wiki init
mkdocs` to add it.
