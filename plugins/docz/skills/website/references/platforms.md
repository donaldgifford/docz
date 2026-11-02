# Website Platform Decision

How to pick between `mkdocs`, `docusaurus`, and `zensical`
for a public documentation site.

| Question | mkdocs | docusaurus | zensical |
|---|---|---|---|
| Allergic to JS toolchain? | **yes** | — | — |
| Need MDX (React in markdown)? | — | **yes** | — |
| Need versioned docs? | with `mike` | **yes** (native) | — |
| Need i18n? | partial | **yes** (native) | — |
| Need a blog? | with plugin | **yes** (native) | partial |
| Need single-page / scroll-spy navigation? | — | — | **yes** |
| OK with React knowledge for theming? | — | yes | yes |
| Want zero ops (push to publish)? | **yes** | yes | yes |
| Brand-heavy design? | limited | yes | **yes** |

## Decision tree

```
Need MDX / versioning / i18n / blog?
├── yes  → docusaurus
└── no
    Want single-page / scroll-spy layout?
    ├── yes  → zensical
    └── no   → mkdocs (default)
```

## Why MkDocs is the default

- Lowest barrier. No JS toolchain. One YAML file.
- Material theme covers 95% of design needs without
  custom CSS.
- Deploy in 30-line workflow to GitHub Pages.
- Most teams' docs don't need React components, blogs, or
  versioning — and adding those later is straightforward.

## When to escalate to Docusaurus

- You **need** to embed React components inside markdown.
- You publish docs **per release version** and old versions
  must remain navigable.
- You publish in **multiple languages**.
- The docs site doubles as a **blog or marketing surface**.

Docusaurus is the heaviest option. Don't pick it for
ergonomics — pick it because one of the above is a
genuine requirement.

## When to escalate to Zensical

- You want a **single-page docs landing** with smooth
  scroll-spy navigation.
- You want **brand-heavy design** but don't want to learn
  the Docusaurus theming system.
- You want **less ops than Docusaurus** while keeping
  modern UX.

Zensical is newer and has less ecosystem reach. Use it
when its specific UX is what you want; otherwise
default to MkDocs.

## Wiki vs Website

`docz:wiki` and `docz:website` overlap on the MkDocs
platform. Difference:

| Skill | Audience | Style |
|---|---|---|
| `wiki` | Internal team, contributors | Plain Material theme, ADR/RFC nav structure dominant |
| `website` | External users, customers | Tuned theme, social cards, top-level tabs, marketing-grade |

You can run both — `docs/` is the same source tree, but
the published artifacts differ.

## Migration between platforms

The scaffold is idempotent on its own platform:

- `docz website init mkdocs --force` — replaces MkDocs
  config.
- Switching platforms (mkdocs → docusaurus) is a manual
  step: run the new platform's `init`, then delete the
  previous platform's files (`mkdocs.yml`, etc.).

Don't keep two platforms active.
