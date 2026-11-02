# Platform Decision Matrix

How to pick between `mkdocs`, `backstage`, `gh`, and
`forgejo`.

| Question | mkdocs | backstage | gh | forgejo |
|---|---|---|---|---|
| Do you have a Backstage instance? | — | **yes** | — | — |
| Internal-only docs hosted in the SCM? | — | — | **yes** | **yes** |
| Want a public dev portal? | **yes** | — | — | — |
| Open-source repo with PRs and issues? | **yes** | — | sometimes | sometimes |
| Allergic to JS toolchains? | **yes** | **yes** | **yes** | **yes** |
| Want zero ops (push to publish)? | **yes** | — | **yes** | **yes** |

## Decision tree

```
Backstage catalog exists?
├── yes  → backstage
└── no
    SCM is Forgejo / Gitea / Codeberg?
    ├── yes  → forgejo
    └── no
        Want a public site separate from the SCM?
        ├── yes  → mkdocs
        └── no   → gh
```

## Why MkDocs is the default

- **Lowest setup cost.** Pure Python; one `mkdocs.yml`.
- **No JS toolchain required.** Important for backend
  teams.
- **Material theme is excellent** without customization.
- **Plugin ecosystem** covers mermaid, search, redirects,
  versioning.
- **GitHub Pages publishes** via a 30-line workflow.

The trade-off: less interactivity than a Docusaurus or
Zensical site (no React components in markdown). For
internal docs, this is usually a feature.

## When MkDocs is the wrong default

- **You need MDX / React components** → use the
  `website` skill with `docusaurus`.
- **You need offline-first single-page docs** → use the
  `website` skill with `zensical`.
- **You're already using Backstage** → use the `backstage`
  platform here.
- **Docs need to live alongside the code, in the SCM's
  wiki** → use `gh` or `forgejo`.

## Migration between platforms

The skill is idempotent: re-running `init <platform>`
writes the new platform's files alongside (or replacing,
with `--force`) the existing ones. Migration steps:

1. `docz wiki init <new-platform> --force`
2. Manually delete the previous platform's files (e.g.
   `mkdocs.yml` when moving to `gh`).
3. Update `.docz.yaml` `wiki.platform` to the new key.

Don't keep two platforms active simultaneously — they'll
fight over the nav structure.
