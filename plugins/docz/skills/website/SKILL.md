---
name: docz:website
description: Scaffold a public-facing documentation website — MkDocs Material (default), Docusaurus, or Zensical.
when_to_use: User says "set up a docs website", "scaffold docusaurus", "build a dev portal", "/docz:website init <platform>", or "publish a docs site".
allowed-tools: Read, Write, Bash(docz:*), Bash(npm:*), Bash(mkdocs:*), Bash(gh:*)
argument-hint: "init|update <platform>"
---

# docz:website

Scaffold a public-facing documentation site. Higher
production-value than `wiki` — picks a platform, writes
the scaffold, configures a deploy workflow.

## Platforms

| Platform | When to use |
|---|---|
| `mkdocs` (default) | Best balance of zero JS toolchain and decent customization. Material theme. |
| `docusaurus` | React/MDX components in markdown, internationalization, versioning, blog. |
| `zensical` | Single-page docs platform; less ops than Docusaurus, more design polish than MkDocs. |

## Arguments

| Argument | Behavior |
|---|---|
| `init <platform>` | Scaffold the platform's project tree. |
| `update <platform>` | Re-run the scaffolder; preserves user edits. |
| `--site-name <name>` | Site title (default: repo name). |
| `--site-url <url>` | Canonical URL for the published site. |
| `--force` | Overwrite existing config. |

## Pipeline (`init`)

1. **Detect platform.** Refuse unknown platforms.
2. **Pre-flight.** Confirm `docs/` exists with at least
   one `.md` file.
3. **Scaffold** — see per-platform sections below.
4. **Write deploy workflow** under `.github/workflows/`.
5. **Report next steps** — local preview command, deploy
   target URL.

## Platform: `mkdocs` (default)

Same scaffold as `docz:wiki init mkdocs` but tuned for a
public portal:

- `site_url` set in `mkdocs.yml` (Material requires it for
  canonical links).
- Custom palette: brand color from `--brand-color` flag,
  defaults to indigo.
- `nav.tabs` enabled for top-level navigation.
- `social-cards` plugin enabled for OG image generation.
- Workflow publishes to `gh-pages` (or `--target netlify`
  / `cloudflare` / `s3` if specified).

## Platform: `docusaurus`

**Files written / commands run:**

- `npx create-docusaurus@latest <name> classic --typescript`.
- Rewrites `docusaurus.config.ts` to point `docs:`
  preset at the repo's `docs/` tree.
- Updates `sidebars.ts` to reflect docz doc-type groups.
- Adds `.github/workflows/website.yml` building on push
  to `main`, deploying to GitHub Pages or
  `--target netlify`.

**When Docusaurus shines:**

- MDX (React components in markdown).
- Versioned docs across releases.
- i18n for multiple languages.
- Blog alongside docs.

**Trade-offs:**

- Node toolchain required for any local preview.
- Higher LOC of generated boilerplate.
- Customizing the theme means React knowledge.

See `references/docusaurus-setup.md`.

## Platform: `zensical`

**Files written:**

- `zensical.config.ts` configured against the repo's
  `docs/` tree.
- `package.json` with `zensical` as a dependency.
- `.github/workflows/website.yml` building and deploying.

Zensical lands between MkDocs and Docusaurus: less ops
than Docusaurus (no plugin ecosystem to learn), more
design freedom than MkDocs (Tailwind + component slots).

See `references/zensical-setup.md`.

## Configuration

The `website` section in `.docz.yaml`:

```yaml
website:
  platform: mkdocs            # or docusaurus, zensical
  site_url: https://docs.example.com
  brand_color: indigo
  deploy_target: gh-pages     # or netlify, cloudflare, s3
```

## What this skill does not do

- **Doesn't run `npm install` or `mkdocs build`** —
  scaffolding only. Document the next command in the
  report.
- **Doesn't write the deploy secrets.** GitHub Pages
  works with the default token; Netlify/Cloudflare/S3
  require manual secret setup.
- **Doesn't sync with `wiki` skill.** If the repo has
  both a wiki (`docz:wiki`) and a website (`docz:website`),
  they live side by side; users decide which one to push.

## References

| Topic | File |
|---|---|
| Platform decision (websites) | `references/platforms.md` |
| Docusaurus scaffolder details | `references/docusaurus-setup.md` |
| Zensical setup notes | `references/zensical-setup.md` |
| MkDocs canonical config | `../wiki/references/mkdocs-config.md` (shared) |
