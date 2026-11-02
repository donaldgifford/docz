# CI Pipeline (rust-docz)

How the `rust-docz.yml` workflow runs and enforces
freshness. Mirrors the `go-docz` pattern.

## Workflow shape

```yaml
name: Rust Docs

on:
  push:
    branches: [main]
    paths:
      - '**/*.rs'
      - 'Cargo.toml'
      - 'Cargo.lock'
  pull_request:
    paths:
      - '**/*.rs'
      - 'Cargo.toml'
      - 'Cargo.lock'

permissions:
  contents: write
  pull-requests: read

jobs:
  drift-check:
    if: github.event_name == 'pull_request'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dtolnay/rust-toolchain@stable
      - uses: actions/cache@v4
        with:
          path: |
            ~/.cargo/registry
            ~/.cargo/git
            target
          key: rust-docz-${{ runner.os }}-${{ hashFiles('**/Cargo.lock') }}
      - run: docz rust-docz
      - name: Check for drift
        run: |
          if [ -n "$(git status --porcelain docs/api/rust)" ]; then
            echo "::error::Rust docs are out of date."
            echo "Run 'docz rust-docz' locally and commit."
            git diff --stat docs/api/rust
            git diff docs/api/rust | head -200
            exit 1
          fi

  regenerate:
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          token: ${{ secrets.DOCS_PUSH_TOKEN }}
      - uses: dtolnay/rust-toolchain@stable
      - uses: actions/cache@v4
        with:
          path: |
            ~/.cargo/registry
            ~/.cargo/git
            target
          key: rust-docz-${{ runner.os }}-${{ hashFiles('**/Cargo.lock') }}
      - run: docz rust-docz
      - name: Auto-commit if drift exists
        run: |
          if [ -n "$(git status --porcelain docs/api/rust)" ]; then
            git config user.name "docz-bot"
            git config user.email "docz-bot@example.com"
            git add docs/api/rust
            git commit -m "chore(changelog): Auto-regenerate Rust docs"
            git push
          fi
```

Same two-job pattern as `go-docz`:

- **drift-check** on PRs.
- **regenerate** on `main` push, with auto-commit using
  the `chore(changelog):` prefix.

## Cache discipline

Rust builds are slow. The workflow caches:

- `~/.cargo/registry` — crate index and tarballs.
- `~/.cargo/git` — git dependencies.
- `target/` — incremental compilation artifacts.

Cache key is hashed against `Cargo.lock`, so changes to
dependencies invalidate the cache appropriately. Without
the cache, a typical run takes 5-10 minutes; with it,
30-60 seconds.

## Token requirements

Same as `go-docz`:

- `DOCS_PUSH_TOKEN` — PAT or fine-grained token with
  `Contents: write`. Default `GITHUB_TOKEN` can't trigger
  further workflows.

## Path filter

The workflow triggers on:

- `**/*.rs` — any Rust source change.
- `Cargo.toml` / `Cargo.lock` — dependency or feature
  flag changes (which affect what gets documented).

Reference / markdown / asset changes don't trigger.

## When drift detection is wrong

Same caveats as `go-docz`:

- Deletion PRs require the author to regenerate locally
  and commit the trimmed `docs/api/rust/` tree.
- Emergency PRs can skip via `docz: skip-drift` in the
  PR body.

## mdbook coexistence

If `book.toml` exists at the repo root, the skill exits
early (see the SKILL.md). The workflow still runs the
skill, but the skill reports "mdbook detected; skipping"
and exits 0 — no drift, no failure.

If the user wants both API docs and mdbook, they must:

1. Add `--output docs/api/rust --force` to the workflow's
   `docz rust-docz` invocation.
2. Manually maintain the nav reconciling the two trees.

## Workspace builds

`--workspace --no-deps` is implicit. The skill builds
every member crate's docs in one `cargo doc` invocation
and groups output per crate. Workspace members are
discovered from the `[workspace.members]` section of the
root `Cargo.toml`.

## See also

- `cargo-doc-vs-mdbook.md` — tool choice rationale
- `../go-docz/references/ci-pipeline.md` — the precedent
  workflow this mirrors
- DESIGN-0014 OQ-6a — drift detection design decision
