---
name: docz:go-docz
description: Generate Go API documentation (pkg.go.dev-style) for the current module and integrate with the docz wiki or website.
when_to_use: User says "generate Go API docs", "publish godoc to the wiki", "set up go-docz", "/docz:go-docz", or invokes the skill from a Go module.
allowed-tools: Read, Write, Bash(go:*), Bash(godoc:*), Bash(pkgsite:*)
argument-hint: "[--output <dir>] [--module <name>] [--public-only]"
paths: "go.mod"
---

# docz:go-docz

Generate Go API documentation using `pkgsite` (the
renderer behind pkg.go.dev) and publish into the repo's
wiki or website. Composes with `docz:wiki` and
`docz:website` — the published docs include the
`docs/api/go/` tree.

## Pipeline

1. **Locate module.** Walk up to find `go.mod`. Fail if
   none.
2. **Detect output target** (per OQ-5a):
   - `mkdocs.yml` at repo root → write to `docs/api/go/`.
   - `docusaurus.config.ts` at repo root → write to
     `docs/api/go/`.
   - Neither → write to `_generated/docs/go/` and warn.
3. **Run pkgsite.** Boot in offline mode against the
   module: `pkgsite -http=:8080 -gopath_mode=false .`.
4. **Render pages.** For each package in the module,
   fetch `/<module>/<pkg>` and convert HTML to markdown.
5. **Write artifacts** to the output dir. One file per
   package: `pkg-<name>.md`. Index at `index.md`.
6. **Write workflow** at `.github/workflows/go-docz.yml`
   so docs regenerate on every push to `main` and a drift
   check fails CI when source changes ship without
   regenerated docs.

## Arguments

| Argument | Behavior |
|---|---|
| `--output <dir>` | Override the auto-detected output target. |
| `--module <name>` | Limit to a specific module (default: all under `go.mod`). |
| `--public-only` | Skip unexported symbols. Default is to include them. |

## Why pkgsite over rolling our own

`pkgsite` is the same renderer pkg.go.dev uses, so the
output matches what users see online. Trying to parse
godoc text into custom markdown loses:

- Identifier cross-links across packages.
- Example function rendering.
- Deprecation notices.
- Heading and code-block detection.

See `references/pkgsite-vs-godoc.md`.

## Workflow: drift detection

`.github/workflows/go-docz.yml`:

```yaml
name: Go Docs

on:
  push:
    branches: [main]
    paths: ['**/*.go', 'go.mod', 'go.sum']
  pull_request:
    paths: ['**/*.go', 'go.mod', 'go.sum']

jobs:
  generate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: go install golang.org/x/pkgsite/cmd/pkgsite@latest
      - run: |
          docz go-docz
          if [ -n "$(git status --porcelain docs/api/go)" ]; then
            echo "::error::Generated Go docs are out of date."
            echo "Run 'docz go-docz' locally and commit."
            git diff docs/api/go
            exit 1
          fi
```

Honors the `chore(changelog)` skip pattern: PRs that only
touch `chore(changelog)`-tagged commits skip the drift
check (mirrors this repo's `changelog-regen.yml`).

See `references/ci-pipeline.md` for the full workflow
shape.

## What this skill does not do

- **Doesn't run `mkdocs build` or `docusaurus build`.**
  That's the host platform's job.
- **Doesn't publish to pkg.go.dev.** Public modules
  publish there automatically; this skill is for
  *internal* wiki/website embedding.
- **Doesn't override the doc.go convention.** Use
  `go-development:go-doc` for `doc.go` generation
  before this skill runs.

## References

| Topic | File |
|---|---|
| pkgsite vs raw godoc | `references/pkgsite-vs-godoc.md` |
| CI pipeline with drift detection | `references/ci-pipeline.md` |
| go-doc (write `doc.go` first) | `../../go-development/skills/go-doc/` (cross-plugin) |
