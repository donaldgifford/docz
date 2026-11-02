---
name: docz:wiki
description: Scaffold and maintain a documentation wiki for the repo across multiple platforms — MkDocs (default), Backstage TechDocs, GitHub wiki, Forgejo wiki.
when_to_use: User says "set up the wiki", "init mkdocs", "wire up Backstage TechDocs", "sync to GitHub wiki", "/docz:wiki init <platform>", or "rebuild the nav".
allowed-tools: Read, Write, Bash(docz:*), Bash(mkdocs:*), Bash(gh:*)
argument-hint: "init|update <platform>"
---

# docz:wiki

Scaffold and maintain a documentation wiki for the repo.
Picks a target platform and writes the configuration files
that platform needs to publish the `docs/` tree.

## Platforms

| Platform | When to use |
|---|---|
| `mkdocs` (default) | Best general-purpose docs portal. Material theme, mkdocs-mermaid2, drift-free nav rebuild. |
| `backstage` | Backstage TechDocs (MkDocs + `techdocs-core` plugin). Use when the team has a Backstage instance. |
| `gh` | Mirror `docs/wiki/` into GitHub's per-repo `<repo>.wiki.git` for the GH Wiki UI. |
| `forgejo` | Same as `gh` but for Forgejo / Codeberg / Gitea wikis via the Forgejo API. |

## Arguments

| Argument | Behavior |
|---|---|
| `init <platform>` | Write platform-specific scaffolding (yml, workflow, scripts). Refuses to overwrite without `--force`. |
| `update <platform>` | Re-run the scaffolder idempotently. Rebuilds the nav from current `docs/` contents. |
| `--site-name <name>` | Override the site title. Defaults to the repo directory name. |
| `--site-description <desc>` | Add a site description. |
| `--force` | Overwrite existing config files. Required to clobber a hand-edited `mkdocs.yml`. |

## Pipeline (`init`)

1. **Detect platform.** Refuse unknown platforms with the
   list above.
2. **Pre-flight checks.** Confirm `docs/` exists and
   contains at least one `.md` file. If not, prompt the
   user to run `docz init` first.
3. **Write platform files** — see the platform-specific
   sections below.
4. **Build nav** from `docs/` contents, honoring
   `wiki.exclude` and `wiki.nav_titles` from `.docz.yaml`.
5. **Report.** Print the files written and the next step
   (e.g. `mkdocs serve` for local preview).

## Pipeline (`update`)

Same as `init` but with `--force` semantics scoped to nav
sections only. Hand-edited theme/plugins config is
preserved; only the `nav:` block is regenerated.

## Platform: `mkdocs` (default)

**Files written:**

- `mkdocs.yml` — Material theme, mermaid2 plugin, search,
  nav generated from `docs/`.
- `docs/index.md` — references the docz README if absent.
- `requirements.txt` — `mkdocs`, `mkdocs-material`,
  `mkdocs-mermaid2-plugin`.
- `.github/workflows/wiki.yml` — builds on push to `main`
  and publishes to `gh-pages` branch.

See `references/mkdocs-config.md` for the canonical
`mkdocs.yml` shape.

## Platform: `backstage`

Backstage TechDocs is MkDocs + a few extra parts (the
`techdocs-core` plugin, monorepo-aware paths). This skill
scaffolds the MkDocs side and then **detects** Backstage
catalog state.

**Files written:**

- `mkdocs.yml` — same as the `mkdocs` platform but with
  `techdocs-core` added under `plugins:`.
- `requirements.txt` — adds
  `mkdocs-techdocs-core` to the standard set.

**Detection step (no files written):**

- Look for `catalog-info.yaml` at the repo root.
  - If present: parse `metadata.annotations` and verify the
    `backstage.io/techdocs-ref` key points at `dir:.` (or
    the docs directory). Report any mismatch as a warning.
  - If absent: warn that Backstage catalog import is a
    separate manual step. Proceed with the MkDocs-only
    setup.

**Does not write `catalog-info.yaml`.** Catalog ownership
lives with the platform/team, not the docs tool.

See `references/backstage-techdocs.md`.

## Platform: `gh`

**Files written:**

- `scripts/sync-wiki.sh` — clones `<repo>.wiki.git`,
  mirrors `docs/wiki/` into it, pushes. Idempotent.
- `.github/workflows/wiki-sync.yml` — runs `sync-wiki.sh`
  on push to `main` against paths matching `docs/wiki/**`.

GitHub Wikis don't support nested directories well; the
sync script flattens with `--`-separated paths
(`adr--0001-foo.md`).

See `references/gh-wiki-sync.md`.

## Platform: `forgejo`

Same shape as `gh` but uses the Forgejo API
(`/api/v1/repos/{owner}/{repo}/wiki/page/{page}`).
Requires `FORGEJO_TOKEN` env var in the workflow.

## Configuration

The `wiki` section in `.docz.yaml`:

```yaml
wiki:
  platform: mkdocs            # platform key
  auto_update: true           # update nav on `docz create`
  mkdocs_path: mkdocs.yml
  exclude:
    - templates
    - examples
  nav_titles:
    adr: ADRs
    design: Design
    impl: Implementation Plans
    investigation: Investigations
    plan: Plans
    rfc: RFCs
```

## What this skill does not do

- **Doesn't run `mkdocs build`** — the workflow does that
  after merge. Local preview is `mkdocs serve`.
- **Doesn't write `catalog-info.yaml`** — Backstage
  catalog ownership is the platform team's responsibility.
- **Doesn't clobber hand-edited theme/plugins.** `update`
  only rebuilds nav.
- **Doesn't support multi-repo wikis.** One platform per
  repo.

## References

| Topic | File |
|---|---|
| Platform decision matrix | `references/platforms.md` |
| Canonical mkdocs.yml | `references/mkdocs-config.md` |
| Backstage TechDocs setup | `references/backstage-techdocs.md` |
| GitHub wiki sync mechanics | `references/gh-wiki-sync.md` |
