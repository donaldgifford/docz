# GitHub Wiki Sync

How the `gh` and `forgejo` platforms mirror `docs/wiki/`
into the SCM's per-repo wiki.

## Background

GitHub Wikis and Forgejo Wikis are git repositories
shadowing the main repo (`<repo>.wiki.git`). They render
markdown like the main repo but use:

- **Flat file structure.** No nested directories (Forgejo
  is more permissive, but for portability stay flat).
- **`Home.md`** as the landing page (not `README.md`).
- **`_Sidebar.md`** for custom sidebar navigation.
- **Spaces in filenames** become URL parts (e.g.
  `Getting Started.md` → `/wiki/Getting-Started`).

## What the skill writes

`scripts/sync-wiki.sh`:

```bash
#!/usr/bin/env bash
set -Eeuo pipefail

REPO_SLUG="${REPO_SLUG:?REPO_SLUG required (owner/repo)}"
WIKI_URL="https://github.com/${REPO_SLUG}.wiki.git"
WORKDIR="$(mktemp -d)"
trap "rm -rf $WORKDIR" EXIT

git clone "$WIKI_URL" "$WORKDIR/wiki"

# Mirror docs/wiki/ into wiki/, flattening with `--` separators.
rsync -a --delete \
    --exclude '.git' \
    docs/wiki/ "$WORKDIR/wiki/"

cd "$WORKDIR/wiki"
git add -A
if git diff --staged --quiet; then
    echo "No changes to sync"
    exit 0
fi
git commit -m "Sync from docs/wiki/ ($(date -u +%Y-%m-%dT%H:%M:%SZ))"
git push
```

`.github/workflows/wiki-sync.yml`:

```yaml
name: Wiki Sync

on:
  push:
    branches: [main]
    paths:
      - 'docs/wiki/**'

permissions:
  contents: write

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          token: ${{ secrets.WIKI_PUSH_TOKEN }}
      - env:
          REPO_SLUG: ${{ github.repository }}
        run: bash scripts/sync-wiki.sh
```

## Path-flattening convention

GitHub wikis don't support nested directories. The sync
script flattens `docs/wiki/<group>/<page>.md` into
`<group>--<page>.md` in the wiki repo:

```
docs/wiki/adr/0001-pick-stack.md
  → adr--0001-pick-stack.md
  → URL: /wiki/adr--0001-pick-stack
```

Reverse mapping is unambiguous as long as no original
filename contains `--`.

## `_Sidebar.md`

The skill writes a `_Sidebar.md` that mirrors the directory
structure logically:

```markdown
## ADRs
- [[adr--0001-pick-stack]]
- [[adr--0002-add-postgres]]

## Design
- [[design--0001-storage-layout]]
```

The wikilinks (`[[…]]`) resolve to the flattened paths.

## Token requirements

GitHub wikis require:

- `WIKI_PUSH_TOKEN` — a PAT or fine-grained token with
  write access to `Contents` on the wiki repo. Repo
  secrets — not env, not in code.
- GitHub's default `GITHUB_TOKEN` **cannot push to
  wikis**. Document this in the workflow.

Forgejo equivalent:

- `FORGEJO_TOKEN` with `write:repository` scope.
- Workflow uses the Forgejo API instead of git push:
  `curl -X POST -H "Authorization: token $FORGEJO_TOKEN"`
  against `/api/v1/repos/{owner}/{repo}/wiki/page/{page}`.

## Bidirectional sync? No.

This script is one-way: `docs/wiki/` is the source of
truth, the GH wiki is the publish target. Edits made
through the GH wiki UI are **discarded on next sync**.

Bidirectional sync would require conflict resolution
logic that isn't worth the complexity for v1. If users
want to edit in the wiki UI, they should disable the
sync workflow and treat the wiki repo as source of truth
instead.

## When to use this vs `mkdocs`

| Use `gh`/`forgejo` when… | Use `mkdocs` when… |
|---|---|
| Docs are SCM-internal | Public dev portal |
| Team already uses the wiki UI | Team prefers a separate site |
| Single-repo, single-team | Cross-repo navigation matters |
| Don't want a separate workflow to learn | OK with `mkdocs serve` for preview |
