---
name: docz:rust-docz
description: Generate Rust API documentation (cargo doc) for the current crate or workspace and integrate with the docz wiki or website.
when_to_use: User says "generate Rust API docs", "publish rustdoc to the wiki", "set up rust-docz", "/docz:rust-docz", or invokes the skill from a Cargo project.
allowed-tools: Read, Write, Bash(cargo:*), Bash(rustdoc:*)
argument-hint: "[--output <dir>] [--crate <name>] [--public-only]"
paths: "Cargo.toml"
---

# docz:rust-docz

Generate Rust API documentation with `cargo doc` and
publish into the repo's wiki or website. Composes with
`docz:wiki` and `docz:website` — the published docs
include the `docs/api/rust/` tree.

## Pipeline

1. **Locate crate / workspace root.** Walk up to find
   `Cargo.toml`. If it's a workspace, enumerate member
   crates. Single-workspace only (per OQ-4a); refuse
   multi-root workspaces with a clear error.
2. **Detect output target** (per OQ-5a):
   - `mkdocs.yml` at repo root → write to
     `docs/api/rust/`.
   - `docusaurus.config.ts` at repo root → write to
     `docs/api/rust/`.
   - Neither → write to `_generated/docs/rust/` and warn.
3. **Detect mdbook** — if `book.toml` exists at repo root,
   skip the conversion. mdbook is the authoring path,
   not the API-extraction path. Report and exit cleanly.
4. **Run `cargo doc`:**
   ```bash
   cargo doc --no-deps --workspace \
       --document-private-items   # unless --public-only
   ```
5. **Post-process** `target/doc/` into a markdown tree.
   `cargo doc` emits HTML; convert per-page.
6. **Write artifacts** to the output dir, one directory
   per crate.
7. **Write workflow** at
   `.github/workflows/rust-docz.yml` with the same
   drift-detection pattern as `go-docz`.

## Arguments

| Argument | Behavior |
|---|---|
| `--output <dir>` | Override the auto-detected output target. |
| `--crate <name>` | Limit to one workspace member. |
| `--public-only` | Drop `--document-private-items`. Defaults to including private. |

## mdbook detection

mdbook is the standard for long-form Rust docs (the Rust
Book, async-book, etc.). If the repo uses mdbook, this
skill steps out of the way:

- Detects `book.toml` at the repo root.
- Reports "mdbook detected at `<path>`; skipping API
  extraction. Use the mdbook authoring workflow instead."
- Exits cleanly with status 0 (not an error).

If users want both API docs (cargo doc output) and prose
docs (mdbook), they should:

1. Author prose in mdbook.
2. Disable mdbook detection: `docz rust-docz
   --output docs/api/rust --force`.
3. Manually reconcile the two trees in their nav.

## CI: drift detection

`.github/workflows/rust-docz.yml`:

```yaml
name: Rust Docs

on:
  push:
    branches: [main]
    paths: ['**/*.rs', 'Cargo.toml', 'Cargo.lock']
  pull_request:
    paths: ['**/*.rs', 'Cargo.toml', 'Cargo.lock']

jobs:
  drift-check:
    if: github.event_name == 'pull_request'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dtolnay/rust-toolchain@stable
      - run: docz rust-docz
      - name: Check for drift
        run: |
          if [ -n "$(git status --porcelain docs/api/rust)" ]; then
            echo "::error::Rust docs are out of date."
            git diff docs/api/rust
            exit 1
          fi
```

See `references/ci-pipeline.md`.

## What this skill does not do

- **Doesn't support multi-root workspaces.** Single
  workspace only (OQ-4a). Multi-root is a follow-up.
- **Doesn't run `mkdocs build` or `docusaurus build`.**
  That's the host platform's job.
- **Doesn't publish to docs.rs.** Public crates publish
  there automatically; this skill is for *internal*
  wiki/website embedding.
- **Doesn't fight mdbook.** If the user has it, the skill
  steps aside.

## References

| Topic | File |
|---|---|
| cargo doc vs mdbook | `references/cargo-doc-vs-mdbook.md` |
| CI pipeline with drift detection | `references/ci-pipeline.md` |
