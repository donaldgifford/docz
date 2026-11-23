# Contributing to docz-site

Thank you for your interest in contributing. This document covers how to report
issues, propose changes, and submit pull requests.

## Quick start

```bash
mise install                               # toolchain pinned in the root mise.toml
NPM_TOKEN=$(gh auth token) just ui install # bun install, in ui/
just ui lint                               # lint
```

Installing needs a GitHub token with the `read:packages` scope in
`NPM_TOKEN`, because `@donaldgifford/design-system` comes from GitHub
Packages. `gh auth token` prints one; add the scope with
`gh auth refresh --scopes read:packages` if the install answers 401 or
403. The token stays in the environment: never write it into `.npmrc`
or any other file.

### Pull requests from a fork

A fork's pull request runs with a read-only token that may not be able
to read the package, in which case the UI jobs fail at the install. A
maintainer then pushes the branch to this repository so the jobs run
there. No gate is skipped for a fork.

`just ui` enumerates the frontend's recipes; `just --list` shows the root's.

## Reporting Issues

Use [GitHub Issues](https://github.com/donaldgifford/docz/issues) for:

- **Bug reports** — include the `docz version` output, the command you ran, and
  the error you saw
- **Feature requests** — describe the problem you are trying to solve, not just
  the solution you have in mind
- **Template improvements** — open an issue before changing embedded templates,
  as changes affect all existing users
- For larger proposals, open an RFC document via `docz create rfc` and
  link it from the issue.

## Development Setup

### Prerequisites

- [docz](https://github.com/donaldgifford/docz)
- `just`

```bash
git clone https://github.com/donaldgifford/docz.git
cd docz
# the frontend lives in ui/; its recipes are `just ui <recipe>`
NPM_TOKEN=$(gh auth token) just ui install
just ui lint
```

## Making Changes

### 1. Create a branch

Branch names follow the pattern `<type>/<short-description>`:

```bash
git checkout -b feat/plan-doc-type
git checkout -b fix/slug-truncation
git checkout -b docs/contributing-guide
```

Types: `feat`, `fix`, `docs`, `chore`, `refactor`

### 2. Make your changes

- Keep changes focused. One logical change per PR.
- Add or update tests for any code you change.
- Run `make lint` and `make test` before pushing.

### 3. Commit

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(cmd): add plan document type
fix(template): truncate slug at word boundary
docs: update README with configuration reference
test(index): add dry-run edge case tests
```

Format:
```
<type>(<scope>): <imperative subject>

<optional body explaining why, not what>
```

### 4. Open a pull request

Push your branch and open a PR against `main`. The PR description should:

- Explain what changed and why
- Link to the related issue (if any) with `Fixes #123` or `Refs #123`
- Include a test plan or describe how you verified the change

## License

By contributing you agree that your contributions will be licensed under the
[Apache 2.0 License](LICENSE).
