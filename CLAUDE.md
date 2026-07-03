# CLAUDE.md

Per-repo orientation for `donaldgifford/docz-api`. This file is a Go-shaped
overlay on top of the universal homelab `CLAUDE.md` (see
[homelab/docs](https://github.com/donaldgifford/docs)); the universals apply
here too — only Go-specific guidance is captured below.

## What this is

`docz-api` is a Go binary maintained as part of the homelab fleet:

- Single binary under `cmd/docz-api/`; library code under `internal/` (private
  to the module).
- Built into a distroless container via `Dockerfile`; released as multi-arch
  (linux+darwin × amd64+arm64) archives via `goreleaser`.
- Lives on Forgejo (`github.com/donaldgifford/docz-api`); a `.github/workflows/`
  mirror exists so the repo can also build on GitHub once it's mirrored.

## Layout

```text
cmd/docz-api/    # main package — keep thin, parse flags + call into internal/
internal/               # library code; not importable outside this module
Dockerfile              # multi-stage distroless build, cached layers
.goreleaser.yml         # release config (multi-arch archives + checksums)
mise.toml               # pinned go + golangci-lint + goreleaser + universal tools
justfile                # `just` task runner — `just` for the menu
.forgejo/workflows/     # CI (Forgejo Actions) — primary
.github/workflows/      # CI (GitHub Actions) — mirror
```

## Workflows

### Build + run

- `just build` — `go build -o bin/docz-api ./cmd/docz-api`
- `just run -- <args>` — runs via `go run` without building
- `just test` — race detector + coverage to `coverage.txt`

### Lint + format

- `just lint` — `golangci-lint run` + yamllint + markdownlint + prettier (covers
  the universal linters too).
- `just fmt` — `go fmt ./...` + yamlfmt + prettier `--write`.

### Release

- `just release v0.1.0` — tag + push. CI picks up the `v*` tag and runs
  `goreleaser release --clean`, producing multi-arch archives and a release
  entry on Forgejo (via `GITEA_TOKEN`) or GitHub (via `GITHUB_TOKEN`).
- Version metadata is injected into the binary via `-ldflags`: `main.version`,
  `main.commit`, `main.date`. `--version`-style output should print these.

### Container build

Built locally with:

```bash
docker build -t docz-api:dev \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) .
```

The Dockerfile uses BuildKit `--mount=type=cache` for `/go/pkg/mod` and
`/root/.cache/go-build` — first build is cold, subsequent builds reuse the cache
layers.

## Go-specific conventions

- **`go.mod` go directive matches `mise.toml`** (currently `go 1.26.4`). Bump
  both together — Renovate's Go updater handles `go.mod`; bump `mise.toml` in
  the same commit.
- **No `vendor/`**. Modules are resolved at build time; the Docker cache mount
  handles offline-ish builds.
- **`internal/` is a hard wall** — packages there can't be imported by other
  modules. Use it liberally; promote to a separate module only when something
  outside this repo actually needs it.
- **`slog` for structured logs**, not `log` or third-party loggers. Set the
  default handler in `main()` so library code doesn't have to thread loggers.
- **No `init()` for behavior**. `init()` runs at import time — it breaks test
  isolation and surprises future-you. Wire dependencies in `main()`.
- **Tests live next to the code** (`foo_test.go` alongside `foo.go`).
  Integration tests that need external services go under a
  `// +build integration` (or `//go:build integration`) tag and run via
  `go test -tags=integration ./...`.
- **Errors wrap with `%w`**: `fmt.Errorf("loading config: %w", err)`. Top of the
  call stack handles via `errors.Is` / `errors.As`.

## CI matrix

- `.forgejo/workflows/ci.yml` runs on every push/PR — `just test` + `just lint`.
- `.github/workflows/ci.yml` is the mirror; identical jobs, runs on the GitHub
  mirror if/when one exists.
- Release workflows fire only on `v*` tag push; `goreleaser` consumes
  `.goreleaser.yml` and the appropriate token (`GITEA_TOKEN` for Forgejo,
  `GITHUB_TOKEN` for GitHub).

## Gotchas

- **`go mod tidy` on first scaffold**: the post-create hook runs it
  automatically. If you skip hooks (`--no-hooks`), run it manually before the
  first `just build` or imports will be unresolved.
- **`goreleaser` v2 config**: the v1 → v2 migration moved `archives[].format` to
  `archives[].formats` (slice). If you copy a pre-v2 `.goreleaser.yml` from
  elsewhere, validate with `goreleaser check`.
- **Distroless `nonroot` UID is 65532**. If the binary needs to write state,
  mount a writable volume — the rootfs is read-only.
- **goreleaser + Forgejo**: the v6 action defaults to GitHub-shaped release
  URLs. The `gitea_urls` block in `.goreleaser.yml` is commented by default —
  uncomment for Forgejo releases, and ensure `GITEA_TOKEN` is set in repo
  Secrets (PAT with `write:repository`).

## Implementation (DESIGN-0001 / IMPL-0001)

The service is being built out per `docs/design/0001-*.md` (Approved) and
`docs/impl/0001-*.md` (the phased plan). Conventions established as the build
progresses:

- **Build/lint/test entry points are `just`** — `just build`, `just test`,
  `just lint`, `just fmt`. There is no `Makefile`; any "`make lint`/`make fmt`"
  instruction maps to the corresponding `just` recipe.
- **docz parsing library** is pinned at `github.com/donaldgifford/docz v0.5.0`
  (a plain `require`, no `replace`; brings `spf13/viper` transitively). Import
  its packages with the **aliases `doczcfg` / `doczdoc`** everywhere, per
  DESIGN-0001 — this repo has its own `internal/config`, so the alias keeps "the
  docz library" unambiguous at every call site even in files that don't
  currently import `internal/config`. This deliberately overrides the generic
  "avoid import aliases" style rule.
- **`internal/doczcontract`** is a runtime-code-free package whose tests guard
  the pinned docz surface (R1–R5). If a docz bump breaks the contract, it fails
  here, not deep in ingest. Re-run after any docz version change.
- **Tests use the standard-library `testing` package only** — no testify or
  other assertion deps. Prefer table-driven tests; positional struct literals
  are fine for tables with ≤3 fields.

### Phase progress

- **Phase 0 — Foundations:** docz `v0.5.0` pinned + `internal/doczcontract`
  smoke test ✅. Remaining: core deps, `internal/config`, `main()` wiring,
  `compose.yaml`.

## Renovate

- `go.mod` updates are PR'd by Renovate's Go module manager.
- Container base images in `Dockerfile` are PR'd by the Docker manager.
- `mise.toml` versions are handled by a custom regex manager configured upstream
  in `donaldgifford/renovate-config`.
