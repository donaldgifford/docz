# CLAUDE.md

This file is loaded into every session, so it is a map and the rules that
apply everywhere. **Keep it short.** Detail about one area goes in that
area's own `CLAUDE.md`, which loads only when a session works there (see
"Where the detail lives"); history goes under `docs/`.

## Project

`docz` is a Go **library** for generating and managing standardized
documentation (RFCs, ADRs, design docs, implementation plans,
investigations, runbooks) whose first consumer is its own CLI (ADR-0002).
One module, `github.com/donaldgifford/docz/v2`, holds four things:

- **The library and CLI.** `pkg/` is the product; `cmd/` is flags,
  printing, and exit codes over it.
- **docz-api**, the server (ADR-0004, DESIGN-0016): `cmd/docz-api/` is its
  binary, `internal/` is the server's and not the library's, and `api/`
  holds its OpenAPI contract. `pkg/` never imports `internal/`.
- **docz-site**, the frontend (DESIGN-0017): `ui/` is its Bun/Vite/React
  project, with docz-review's page and local server beside it.
- **Deployment and plugins**: one Helm chart, `charts/docz`
  (DESIGN-0018), compose stacks under `deploy/`, `Dockerfile.api` and
  `Dockerfile.ui`, and the `docz` plugin marketplace under `plugins/`.

docz-api's and docz-site's planning records from before they moved in are
archived under `docs/archive/api/` and `docs/archive/ui/`, where an ID
means that project's own. They are not rewritten, and `wiki.exclude` and
`api.exclude` both carry `archive` (pinned by `test/archive`).

## Build & Test

```bash
just build          # build to build/bin/docz
just test           # run all tests (root module only)
just test-consumer  # the external-module smoke test (test/consumer, own go.mod)
just parity         # replay the v1.2.2 CLI goldens against build/bin/docz
just validate       # docz validate over this repo's own docs/, non-strict
just lint-generated # markdownlint over a repository docz scaffolds
just lint           # golangci-lint + golines
just fmt            # gofmt + goimports
just ci             # every gate: the Go chain, api::{lint,test}, chart::lint, the ui:: chain
just api build      # the server binary to build/bin/docz-api (api.just, a module)
just api test       # the server's packages only: ./cmd/docz-api/... ./internal/... ./api/...
just vendor-mermaid # gzip ui/node_modules' mermaid into pkg/export/html/assets (after just ui install)
just gen-charcoal   # write pkg/theme/tokens/charcoal.css from ui/node_modules' design system (same)
just ui ci          # the frontend's CI parity
just chart lint     # charts/docz: helm lint with ci/ci-values.yaml (also template, unittest, docs)
```

- There is no `Makefile`. `make build` and `make test` **exit 0 and do
  nothing** (`build/` and `test/` are real directories), so never reach
  for `make`. `just --list` lists recipes and `just --dry-run <recipe>`
  shows a recipe's commands; recipes echo nothing.
- `docz.just` holds docz's recipes at the root namespace; `api.just`,
  `ui.just`, and `chart.just` are modules (`just api <recipe>`, `api::lint`
  from a root gate). `just release vX` and `just bin=<path> parity` take
  their arguments that way. `docs/development/build-notes.md` has the
  rest, and what `just lint-generated` checks.
- Go's version is pinned in `go.mod`, `mise.toml`, `Dockerfile.api`,
  `ui/go.mod`, and `test/consumer/go.mod`; bump all five together.

## Code Conventions

- Go modules with Cobra for the CLI and `text/template` for rendering.
  Config merging is plain yaml.v3 plus a raw-map deep merge (no viper in
  the library; the server's `internal/config` uses it, env-only).
- Every in-repo import carries the `/v2` path element. `docz.just`
  derives the version ldflags from `module_path`, because a `-X` naming a
  path no package has is ignored silently.
- Six built-in doc types: rfc, adr, design, impl, investigation, runbook.
  `runbook` ships disabled (`types.runbook.enabled`). `plan` was removed
  on the v2 line (ADR-0003); a repo that keeps a `types.plan` block gets a
  custom type. Aliases: `implementation` → `impl`, `inv` →
  `investigation`, `rb` → `runbook`.
- Embedded templates live in `pkg/doczcore/doctemplate/templates/`, with
  marker skeletons in `templates/schema/`. A template carries
  `<!-- markdownlint-disable-file MD025 MD041 -->` after its frontmatter
  (`MD024 MD025 MD041` for the IMPL and runbook templates), always a
  disable and never a `markdownlint-configure-file` or
  `markdownlint-enable`, and every line fits MD013's 80 outside tables
  and fences (`TestEmbeddedTemplatesFitMD013`).
- The public packages under `pkg/` are semver-governed. `config`,
  `document`, `docparse`, `docwrite`, and `toc` are frozen at their v1
  shapes. Bytes in, values out; no library interfaces; the core never
  imports a type package (`pkg/doczcore/layer_test.go`).
- Lint: `golangci-lint` with `golines` for line length.
- Tests: `t.TempDir()` for filesystem tests and golden files under
  `testdata/`, regenerated with `go test ./... -update` and never
  hand-edited. `pkg/*` tests run in parallel (`t.Parallel()` on every
  top-level test and subtest); `cmd/` tests stay serial and build a
  `Runner` with `Out: &bytes.Buffer{}` and `RepoRoot: t.TempDir()` rather
  than using `os.Pipe` or `os.Chdir`. The server's tests use the standard
  library's `testing` only.
- Parity goldens (`test/parity`) are captured from the v1.2.2 release
  binary and from nowhere else; never re-capture them from a v2 build.
- Never commit a credential-shaped string, even a fake one in a fixture
  or a doc: trufflehog scans the whole PR range and unverified findings
  fail it.

## Git Workflow

- Always create feature branches and PRs; never push directly to main.
- Branch naming: `feat/`, `fix/`, `chore/`, `docs/` prefixes.
- Conventional commits are required (`feat(wiki):`, `fix(config):`,
  `docs:`).
- Every PR needs one semver label: `major`, `minor`, `patch`, or
  `dont-release`. Releases are cut by label through `release.yml`.
- `CHANGELOG.md` is generated: `git-cliff -o CHANGELOG.md`, committed
  last as `chore(changelog): …`. A drift check fails a PR without it.
- `docs/examples/plans/` contains user reference material; never commit
  these files.
- A new test that reads the real tree (`docs/`, tracked files,
  `.docz.yaml`) goes in CI's "Docs and repository checks" job, and a Go
  job that starts reading a new path means widening the `go` path filter
  (`.github/CLAUDE.md`).

## Where the detail lives

Read the file for an area before changing that area. Each loads on its
own when a session reads or edits files under its directory.

| Area | File |
| --- | --- |
| CLI commands, exit codes, the CLI's JSON contract (`api/cli/`) | `cmd/CLAUDE.md` |
| Core packages: `config`, `document`, `docparse`, `kinds`, `validate`, `docwrite`, `index`, `doctemplate`, `toc`, `repo` | `pkg/doczcore/CLAUDE.md` |
| Type packages (`pkg/rfc` … `pkg/runbook`), `pkg/wiki`, `pkg/theme` | `pkg/CLAUDE.md` |
| `pkg/export/html` and `pkg/export/confluence` | `pkg/export/CLAUDE.md` |
| The parity suite and the consumer module | `test/CLAUDE.md` |
| The server: conventions, config, secrets, dependencies | `internal/CLAUDE.md` |
| The OpenAPI contract and its test | `api/CLAUDE.md` |
| Repo index, changelog, and pages endpoints | `internal/httpapi/CLAUDE.md` |
| Failure logging, queue self-heal, boot checks, `AUTH_PROVIDERS=none` | `internal/queue/CLAUDE.md` |
| The server's Confluence export | `internal/export/CLAUDE.md` |
| Scoped access, tokens, the authorization server | `internal/oauth/CLAUDE.md` |
| The MCP server | `internal/mcp/CLAUDE.md` |
| How each server package was built, with its gotchas | `docs/archive/api/build-history.md` |
| The frontend, docz-review's page and local server | `ui/CLAUDE.md` |
| Plugins, the marketplace, docz-review, docz-html | `plugins/CLAUDE.md` |
| The Helm chart | `charts/docz/CLAUDE.md` |
| CI jobs, path filters, publishing, releases | `.github/CLAUDE.md` |

When a change teaches something worth keeping, add it to the area's file,
not to this one.
