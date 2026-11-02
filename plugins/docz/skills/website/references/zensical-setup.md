# Zensical Setup

Scaffolder details for `docz website init zensical`.

## What is Zensical

[Zensical](https://zensical.dev/) is a single-page docs
platform that sits between MkDocs and Docusaurus on the
ergonomics/customization spectrum:

| Aspect | MkDocs | Zensical | Docusaurus |
|---|---|---|---|
| Toolchain | Python | Node | Node |
| LOC of generated boilerplate | ~30 | ~100 | ~500 |
| Customization | YAML + CSS | Tailwind + slots | React |
| Versioning | with `mike` | — (single version) | native |
| i18n | partial | — | native |
| Layout | multi-page | single-page scroll-spy | multi-page |

It's the right choice when you want a **brand-heavy
single-page docs landing** without committing to the
Docusaurus toolchain.

## What the skill writes

`package.json`:

```json
{
  "name": "{{ .RepoName }}-docs",
  "private": true,
  "scripts": {
    "dev": "zensical dev",
    "build": "zensical build",
    "preview": "zensical preview"
  },
  "dependencies": {
    "zensical": "^1.0.0"
  }
}
```

`zensical.config.ts`:

```ts
import { defineConfig } from "zensical";

export default defineConfig({
  title: "{{ .SiteName }}",
  description: "{{ .SiteDescription }}",
  url: "{{ .SiteURL }}",

  docs: {
    root: "../docs",            // point at repo docs tree
    sections: [
      { label: "Architecture", path: "adr/" },
      { label: "Design", path: "design/" },
      { label: "Implementation", path: "impl/" },
      { label: "RFCs", path: "rfc/" },
    ],
  },

  theme: {
    brandColor: "{{ .BrandColor }}",
    font: "Inter, system-ui",
  },
});
```

`.github/workflows/website.yml`:

```yaml
name: Website

on:
  push:
    branches: [main]
    paths:
      - 'docs/**'
      - 'website/**'

permissions:
  contents: write
  pages: write
  id-token: write

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: website/package-lock.json
      - working-directory: website
        run: |
          npm ci
          npm run build
      - uses: actions/upload-pages-artifact@v3
        with:
          path: website/dist
      - uses: actions/deploy-pages@v4
```

## Sectioning convention

Zensical's single-page layout works best with 3-5
sections. The skill maps docz doc types onto sections
1:1, but you'll likely want to consolidate:

- **Architecture** — adr/, design/
- **How to** — runbooks, guides
- **Reference** — API docs, config schemas

Edit `zensical.config.ts` `sections` to taste after the
scaffold writes.

## When Zensical isn't right

- **You need multi-page navigation.** Zensical is
  single-page. Use Docusaurus or MkDocs.
- **You publish per-version docs.** Zensical doesn't have
  versioning. Use Docusaurus.
- **Your team doesn't have anyone who writes Tailwind /
  CSS.** The customization story leans on those.

For internal team docs, MkDocs is almost always the
better default.

## See also

- [Zensical docs](https://zensical.dev/) — full reference
- DESIGN-0014 §"Skill: website" — scope and trade-offs
