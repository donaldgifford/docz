# CI Pipeline (go-docz)

How the `go-docz.yml` workflow runs and enforces freshness.

## Workflow shape

```yaml
name: Go Docs

on:
  push:
    branches: [main]
    paths:
      - '**/*.go'
      - 'go.mod'
      - 'go.sum'
  pull_request:
    paths:
      - '**/*.go'
      - 'go.mod'
      - 'go.sum'

permissions:
  contents: write    # for the push job
  pull-requests: read

jobs:
  drift-check:
    if: github.event_name == 'pull_request'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: 'go.mod'
      - run: go install golang.org/x/pkgsite/cmd/pkgsite@latest
      - run: docz go-docz
      - name: Check for drift
        run: |
          if [ -n "$(git status --porcelain docs/api/go)" ]; then
            echo "::error::Go docs are out of date."
            echo "Run 'docz go-docz' locally and commit the result."
            git diff --stat docs/api/go
            git diff docs/api/go | head -200
            exit 1
          fi

  regenerate:
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          token: ${{ secrets.DOCS_PUSH_TOKEN }}
      - uses: actions/setup-go@v5
        with:
          go-version-file: 'go.mod'
      - run: go install golang.org/x/pkgsite/cmd/pkgsite@latest
      - run: docz go-docz
      - name: Auto-commit if drift exists
        run: |
          if [ -n "$(git status --porcelain docs/api/go)" ]; then
            git config user.name "docz-bot"
            git config user.email "docz-bot@example.com"
            git add docs/api/go
            git commit -m "chore(changelog): Auto-regenerate Go docs"
            git push
          fi
```

Two jobs:

- **drift-check** runs on PRs and fails CI if regenerated
  docs differ from what's committed.
- **regenerate** runs on `main` pushes and auto-commits
  any drift with a `chore(changelog):` prefix (so the
  changelog regen workflow doesn't re-cycle on the
  auto-commit).

## Why two jobs

Drift detection alone wouldn't keep the docs current — a
contributor who skipped `docz go-docz` would just see a
red CI. The auto-regenerate job on `main` ensures docs
stay fresh even when PRs land without regeneration.

The `chore(changelog):` prefix matches this repo's
existing skip-list (see `.github/workflows/changelog-regen.yml`):
auto-commits don't trigger changelog regeneration loops.

## Token requirements

- `DOCS_PUSH_TOKEN` — PAT or fine-grained token with
  `Contents: write` on the repo. The default `GITHUB_TOKEN`
  cannot trigger further workflows from its own push,
  which would prevent the changelog workflow from running.
- Set in repo secrets (Settings → Secrets and variables →
  Actions).

## Path filter

The workflow triggers only on Go source / module file
changes. This prevents the docs regen from running on
unrelated commits (markdown edits, CI changes).

Reference / template / fixture files are not Go source,
so they don't trigger. The `paths:` filter is conservative
on purpose.

## When drift detection is wrong

If a PR's purpose is to *delete* a package, drift
detection will fail because the old docs still reference
the deleted symbols. The fix:

1. PR author runs `docz go-docz` locally.
2. Commits the regenerated `docs/api/go/` (which omits
   the deleted package).
3. CI passes.

This is the intended behavior — it forces the docs to
ship in the same PR as the code change.

## Skipping the drift check

For emergency PRs where docs regeneration is impractical:

```yaml
# In the PR description:
# docz: skip-drift
```

The workflow looks for this marker in the PR body and
skips the drift-check job. Use sparingly — every skip
accumulates stale docs.

## See also

- `pkgsite-vs-godoc.md` — why pkgsite is the renderer
- DESIGN-0014 §"Skill: go-docz" — scope and rationale
- This repo's `.github/workflows/changelog-regen.yml` —
  the precedent for `chore(changelog):` skip patterns
