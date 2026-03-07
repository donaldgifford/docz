# Contributing to docz

Thank you for your interest in contributing. This document covers how to report
issues, propose changes, and submit pull requests.

## Reporting Issues

Use [GitHub Issues](https://github.com/donaldgifford/docz/issues) for:

- **Bug reports** — include the `docz version` output, the command you ran, and
  the error you saw
- **Feature requests** — describe the problem you are trying to solve, not just
  the solution you have in mind
- **Template improvements** — open an issue before changing embedded templates,
  as changes affect all existing users

## Development Setup

### Prerequisites

- Go 1.22 or later
- `golangci-lint` (see [installation](https://golangci-lint.run/usage/install/))
- `make`

```bash
git clone https://github.com/donaldgifford/docz.git
cd docz
go mod download
make build   # builds build/bin/docz
make test    # runs all tests
make lint    # runs golangci-lint
```

### Verify your setup

```bash
./build/bin/docz version
make ci   # lint + test + build + license-check must all pass
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

## Code Standards

### Go style

This project follows the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
and enforces it via `golangci-lint`. Run `make lint` before pushing.

Key conventions:

- Table-driven tests with `t.Run`
- `t.TempDir()` for filesystem tests (no mocking the filesystem)
- Return errors; don't log and return
- Package comments on every package (`// Package foo ...`)
- No global state outside of `cmd/` package-level vars

### Testing

Every exported function in `internal/` must have at least one test. The coverage
target for `internal/` packages is >80%.

```bash
make test              # all tests
make test-coverage     # tests with coverage report
go test -run TestXxx   # run a specific test
go test -update        # update golden files
```

### Linting

```bash
make lint        # run golangci-lint (must pass before merging)
make lint-fix    # auto-fix what can be auto-fixed
make fmt         # run gofmt + goimports
```

## Adding a New Built-In Document Type

See [DEVELOPMENT.md](DEVELOPMENT.md#adding-a-built-in-document-type) for the
step-by-step guide.

New built-in types require:

1. A document template under `internal/template/templates/<type>.md`
2. An index header template under `internal/template/templates/index_<type>.md`
3. Default config entry in `internal/config/config.go`
4. Updated `ValidTypes()` in `internal/config/config.go`
5. Tests covering the new type

## Changing Embedded Templates

Embedded templates are part of the public interface — users who have run
`docz init` will have existing documents. Template changes should:

- Preserve all existing template variables (`{{ .Title }}`, etc.)
- Be additive where possible (add new sections, don't remove existing ones)
- Be documented in the PR description

If a template change is breaking (removes or renames a section), it requires a
minor version bump and a note in the changelog.

## CI

All PRs must pass `make ci`:

```
lint     → golangci-lint (0 issues)
test     → go test -race ./... (all green)
build    → go build ./... (no errors)
license  → go-licenses check
```

CI runs on every push to a PR branch. Fix failures before requesting review.

## License

By contributing you agree that your contributions will be licensed under the
[Apache 2.0 License](LICENSE).
