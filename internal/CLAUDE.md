# CLAUDE.md: the server (`internal/`, `cmd/docz-api`)

Loaded when a session works under this directory. The repository's map is the root `CLAUDE.md`.

Per-feature notes load with their package: `api/CLAUDE.md` (the
OpenAPI contract), `internal/httpapi/CLAUDE.md` (index, changelog,
and pages endpoints), `internal/queue/CLAUDE.md` (failure logging and
queue self-heal), `internal/export/CLAUDE.md` (Confluence export),
`internal/oauth/CLAUDE.md` (scoped access and tokens), and
`internal/mcp/CLAUDE.md`. How each package was built, phase by phase
(ingest, search, queue, webhook, auth, sessions, telemetry, and the
old chart), with its gotchas, is `docs/archive/api/build-history.md`:
read the phase for a package before changing it. CI is in
`.github/CLAUDE.md` and the chart in `charts/docz/CLAUDE.md`.

The docz-api server arrived with its full history in IMPL-0019 Phase 2
(DESIGN-0016, ADR-0004): `cmd/docz-api/` is its binary, `internal/` its
library code, `api/` its OpenAPI contract, `charts/docz` its Helm chart (shared
with the site since IMPL-0021),
`deploy/api/` its compose stacks, and `Dockerfile.api` its image. Its recipes
live in `api.just` and run as `just api <recipe>` (`test`, `lint`, and `fmt` scoped to the server's
packages; release tagging, the licence check, and the gates are the root's;
the buildx recipes that were `docker.just` are its `[group('docker')]`). Its
planning documents are archived under `docs/archive/api/` and are **not
rewritten**: `test/archive` pins both that no tracked file outside `docs/`,
`testdata/`, and `CHANGELOG.md` names docz-api's old module path, and
that the five archived documents citing it still do (IMPL-0019 Phase 3).
`internal/ingest` parses a fetched `.docz.yaml` with `config.ParseBytes` — no
temp dir, no `$HOME` merge — and `internal/doczcontract` is gone: its one
surviving clause is `TestConfigLoadsFixtureManifest` in `internal/ingest`, and
`pkg/doczcore`'s own tests pin the rest. `.cliffignore` keeps the grafted
history out of `git-cliff`. The material below is docz-api's own `CLAUDE.md`
from the graft, headings demoted one level; where it names `make`, the
`Makefile`, `internal/doczcontract`, or `docs/` paths, read `just api`,
nothing, nothing, and `docs/archive/api/`.

## Go-specific conventions

- **`go.mod` go directive matches `mise.toml`** (currently `go 1.26.10`, which `Dockerfile.api`, `ui/go.mod`, and `test/consumer/go.mod` also pin). Bump
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
- **docz parsing library** is pinned at `github.com/donaldgifford/docz v1.2.2`
  (a plain `require`, no `replace`; bumped from `v1.2.0` per IMPL-0008 for
  the json-tagged config structs — every config field now carries a `json`
  tag mirroring its `yaml` tag in name and `omitempty`, so the marshaled
  `config_snapshot` serves `.docz.yaml` key spellings; guarded by contract
  clause **R11**, mirroring upstream DESIGN-0008 R11. The crossed `v1.2.1`
  is config-only — docz's own api-block dogfood merge. The v1.1.0→v1.2.0
  bump added the `api:` block surface — `APIConfig` + `ErrInvalidAPIPath` +
  `docparse.Title`, contract clause R10 per IMPL-0007; the v1.0.0→v1.1.0
  bump added the changelog surface — `ChangelogConfig` + `ParseChangelog`,
  contract clause R6 per IMPL-0005; the v0.5.0→v1.0.0 bump was INV-0001).
  The v1.2.0 bump crossed
  `v1.1.1`, which flipped the built-in PLAN type default to disabled — the
  IMPL-0007 fleet check confirmed every fleet manifest pins an explicit
  `types:` block, so no repo was exposed. `pkg/doczcore/docparse` imports
  with the alias `doczparse` (joining `doczcfg`/`doczdoc`). As of `v1.0.0` docz **no longer pulls
  `spf13/viper` transitively** (it moved `config.Load` to `yaml.v3`); `viper`
  (`v1.21.0`) is now a **standalone direct dep** used only by `internal/config`,
  so DESIGN-0001 Decision 2's "reuse viper from docz" rationale no longer holds.
  Import its packages with the **aliases `doczcfg` / `doczdoc`** everywhere, per
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
- **Secrets use `config.Secret`** (a `string` type that redacts on slog, `%s`,
  `%v`, `%+v`, `%#v`). Read the real value only via `.Reveal()` — every unwrap
  is explicit and greppable. Never log a raw credential.
- **`internal/config` is env-only** (`spf13/viper` + `AutomaticEnv`, no config
  file). `Load()` returns one `ErrInvalidConfig` listing **all** problems.
  Config value receivers are heavy — `AuthEnabled` uses a pointer receiver on
  purpose (gocritic `hugeParam`); don't flip it back to a value receiver.

## Renovate

- `go.mod` updates are PR'd by Renovate's Go module manager.
- Container base images in `Dockerfile` are PR'd by the Docker manager.
- `mise.toml` versions are handled by a custom regex manager configured upstream
  in `donaldgifford/renovate-config`.
