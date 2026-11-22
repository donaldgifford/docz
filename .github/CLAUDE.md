# CLAUDE.md: CI and releases

Loaded when a session works under this directory. The repository's map is the root `CLAUDE.md`.

## Releases

The fourteen packages this repo made public on the v2 line were **experimental
through the `v2.0.0-beta.N` tags** (ADR-0002 Decision 7) and are **stable from
v2.0.0**: the cut removed each one's `EXPERIMENTAL` doc-comment paragraph
(#135), so semver covers them as it covers the rest. The five promoted at
v1.0.0 — `config`, `document`, `docparse`, `docwrite`, `toc` — stay frozen at
their v1 shapes, ADR-0003's removal of `plan` from the catalogue excepted.

**v2.0.0 was cut by label** (`major`, through `release.yml`'s
`pr-semver-bump`, which bumps `2.0.0-beta.9` to `2.0.0`), and releases are by
label from there; `release.yml` publishes the images and, when it cut a tag,
`charts/docz`. A non-beta tag pushed by hand triggers no workflow. The
**betas were hand-cut**: every PR carried `dont-release`, and a beta was
`just release v2.0.0-beta.N` from the merge commit. `prerelease.yml` triggers on the `v*-beta.*` tag and
goreleaser's `prerelease: auto` marks the GitHub release. The same tag builds
both binaries (docz-api's archives carry a syft SBOM) and its `publish-image`
job calls `ghcr.yml` to push `ghcr.io/donaldgifford/docz-api` and the chart;
ECR stays behind `vars.ECR_PUBLISH_ENABLED`. The image tag is metadata-action's
`{{version}}`, which strips the `v`, so the chart's `appVersion` is **bare**
(`2.0.0-beta.3`) — a `v`-prefixed default 404s on pull, and the chart's
bare-semver test says so.

## Publishing and path filters

- **Three components publish from one tag.** Bake has `-api` and `-ui` target
  families, and `ghcr.yml`/`ecr.yml` take a `component` input
  (`api`|`ui`|`chart`) resolved in one table. `api` and `ui` are images only,
  with `-` for the chart fields, and `chart` is `charts/docz` alone, with `-`
  for the image fields: the image job skips without a tag and the chart job
  without a chart dir. `prerelease.yml` calls each component, so a
  `v*-beta.*` tag pushes the `docz-api` and `docz-site` images and the `docz`
  chart. **Charts publish only on a release**: `release.yml` calls component
  `chart` only when `bump-version` cut a tag (`skipped != 'true'`), so a
  `dont-release` merge publishes none (IMPL-0021 OQ 2), while a release cut
  by label — v2.0.0 — publishes the chart beside the images (#140).
  **Each GHCR package needs its own Actions access grant** for this repository
  (`docz-site`, `docz-api`, and `charts/docz` each need one); without it the
  push fails `403 write_package`, and granting it after the first 403 and
  re-running the job is the expected path. `charts/docz` is bumped once per
  release, before the tag, and its `appVersion` is bare semver.
- **CI stays path-filtered** (ADR-0004 OQ 4, revisited as DESIGN-0017 OQ 7).
  The `ui` job runs on `ui/**`, `ui.just`, `Dockerfile.ui`,
  `api/openapi.yaml`, `api/cli/**`, and `pkg/theme/**`, so a spec or CLI-schema change runs
  both halves; `ui-e2e` also runs on the `go` filter, since its docz-review
  spec drives a docz it builds. The `go` filter includes
  `ui/e2e/fixtures/review-**`, which `cmd/review_fixture_test.go` reads, and
  the `plugin` job runs on `plugins/**`, `.claude-plugin/**`, and
  `ui/review/**`. **Revisit the filter
  the moment a Go change can break the UI other than through the spec** — an
  `//go:embed` of `ui/dist`, a shared generated file, a Go test that reads
  `ui/`. Until then the UI reaches the server only over the specced HTTP
  surface.
- **The Go jobs are path-filtered on PRs too** (reversing the "Go jobs are
  unfiltered" choice above). `dont-release` is on every v2 PR, Go or not, so
  the paths decide: Lint, Test Go, Security Scan, and Build run when the
  `changes` job's `go` filter matches, and Security Scan also on `ui` (Trivy
  scans its lockfile). The `go` filter is every Go *input*, not just `*.go`:
  `pkg/**` (embedded templates and schemas), `cmd/**`, `internal/**`,
  `api/**`, `test/**` (parity fixtures, goldens), `**/go.{mod,sum}`, the
  justfiles, `.golangci.yml`, `.goreleaser.yml`, `mise.toml`, `sqlc.yaml`,
  and `ci.yml` itself. **Widen it when a Go job starts reading a new
  path.** Every push to `main` runs them unfiltered. One job, **Docs and
  repository checks**, runs on every PR: `just validate` plus `go test
  ./pkg/doczcore/repo/... ./test/archive/...`, because those two read the
  real tree (the migration test runs over `docs/`, `test/archive` scans every
  tracked file and `.docz.yaml`), so a docs-only PR can break them. Add any
  new test that reads the real tree to that job. CodeQL runs on PRs only for
  Go or `ui/` changes, and the licence check only for `go.mod`/`go.sum`. No
  check is required on `main`, so a skipped job never blocks a merge. Every
  job that runs `goreleaser-action` installs cosign first, so the action
  verifies the signature on the GoReleaser it downloads; the release itself
  is still GPG-signed.

## docz-api's CI notes

From docz-api's own `CLAUDE.md`; where it names `make`, read `just api`.

- **GitHub Actions is the only CI** — everything lives under
  `.github/workflows/`. There is **no `.forgejo/workflows/`**; the Forgejo
  mirror is aspirational, so don't assume a Forgejo-primary setup.
- `.github/workflows/ci.yml` runs on every push/PR: `Lint` (golangci-lint via
  the action, **not** `just lint`), `Test Go` (`just test-coverage` + Codecov),
  `Security Scan` (govulncheck + Trivy), and `Build` (goreleaser `--snapshot`
  + SBOM scan). Sibling workflows add CodeQL, license-check (`go-licenses`,
  which needs a root `LICENSE`), trufflehog (verified-only), a changelog drift
  check + auto-regen (`git-cliff` via `cliff.toml`; sync commits match
  `^chore.*changelog` and are skipped to stay idempotent), and a required
  semver label (`major`/`minor`/`patch`/`dont-release`) on PRs.
- `release.yml` fires only on `v*` tag push; `goreleaser` consumes
  `.goreleaser.yml` with `GITHUB_TOKEN` (the `GITEA_TOKEN`/`gitea_urls` path is
  stubbed for a future Forgejo mirror).
