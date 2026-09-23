---
id: IMPL-0019
title: "docz-api move-in: v2.0.0-beta.3"
status: In Progress
author: Donald Gifford
created: 2026-09-22
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0019: docz-api move-in: v2.0.0-beta.3

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 0: Toolchain and just composition](#phase-0-toolchain-and-just-composition)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 1: config.ParseBytes](#phase-1-configparsebytes)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 2: The graft](#phase-2-the-graft)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 3: Make it build: imports, api.just, lint](#phase-3-make-it-build-imports-apijust-lint)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 4: Boundaries, archive, and publishing](#phase-4-boundaries-archive-and-publishing)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 5: v2.0.0-beta.3](#phase-5-v200-beta3)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. How are the six phases branched?](#1-how-are-the-six-phases-branched)
  - [2. How does the docz-api status sweep avoid cutting a docz-api release?](#2-how-does-the-docz-api-status-sweep-avoid-cutting-a-docz-api-release)
  - [3. Where does the exact graft invocation live?](#3-where-does-the-exact-graft-invocation-live)
  - [4. What happens to docker.just?](#4-what-happens-to-dockerjust)
  - [5. How do the image and chart versions continue?](#5-how-do-the-image-and-chart-versions-continue)
  - [6. When are the INV-0002 and INV-0003 successors written?](#6-when-are-the-inv-0002-and-inv-0003-successors-written)
- [References](#references)
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Move docz-api into this repository as part of the same Go module: `internal/`,
`cmd/docz-api`, `api/`, `charts/docz-api`, `contrib/`, and `deploy/api/`, with
its 161 commits grafted on, its 24 documents archived verbatim, and
`config.ParseBytes` added. Six phases, one PR each, ending in a hand-cut
`v2.0.0-beta.3`.

**Implements:** DESIGN-0016 (Approved), which implements ADR-0004 Decisions 2,
3, 4, 6, 7, and 8.

Tasks marked **(human)** need a person — a merge on red, a commit to another
repository, a tag push, or a GitHub settings change. An automated run marks them
`deferred - human required` and continues.

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- `go 1.26.5` across `go.mod`, `test/consumer/go.mod`, and `mise.toml`
- The root `justfile` reshaped to `import 'docz.just'` + `mod? api` + `mod? ui`
  (DESIGN-0016 OQ 1)
- `config.ParseBytes` and the bytes variant of the INV-0003 types-replace rule
- The status sweep and two forward pointers in docz-api, then the `filter-repo`
  graft and `--allow-unrelated-histories` merge
- Reconciling 20 root files, 5 directories, and 17 `.github/` files
- The import rewrite (44 + 11 Go files), scoped outside `docs/archive/`
- `internal/doczcontract` deleted, one test relocated
- One `.golangci.yml`, driven to a zero-finding baseline (OQ 2)
- `TestLayerRules_PkgNeverImportsInternal`
- `docs/archive/api/` with its namespace README and docz-api's CHANGELOG;
  `wiki.exclude` + `api.exclude` as a pair (OQ 4)
- `prerelease.yml` publishing the server image on a beta tag
- Successor documents for docz-api INV-0002 and INV-0003
- `v2.0.0-beta.3`

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- docz-site / `ui/` / `ui.just` contents (beta.4)
- Any change to the server's HTTP surface, schema, queue, or search index
- Renumbering or editing archived documents beyond the sweep and pointers
- Archiving the docz-api repository read-only (after beta.3 is verified)
- Removing the eleven `EXPERIMENTAL` markers (the v2.0.0 cut)

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met. Phases 0 and 1 touch only
this repository and can merge in either order; everything from Phase 2 on is
strictly sequential.

---

<!--docz:phase:start-->
### Phase 0: Toolchain and just composition

Everything the graft will need in place before any incoming file arrives, so
Phase 2's diff is the graft and nothing else.

<!--docz:tasks:start-->
#### Tasks

- [x] Bump `go` to `1.26.5` in `go.mod`, `test/consumer/go.mod`, and
  `mise.toml`; run `go mod tidy` in both modules
- [x] Delete `.checkmake.ini`
- [x] Rewrite the root `justfile` composition: `import 'docz.just'`,
  `mod? api 'api.just'`, `mod? ui 'ui.just'` (replacing the two `import?`
  lines)
- [x] Update `CLAUDE.md`'s just paragraph: the two optional files are modules
  addressed as `just api <recipe>` / `just ui <recipe>`, and `ui.just` will sit
  at the root carrying `set working-directory := "ui"`
- [x] Update `DEVELOPMENT.md` and `CONTRIBUTING.md` wherever they describe the
  composition
- [x] Create the `graft` label (`gh label create graft`), described as "skips
  the Go CI jobs; DESIGN-0016 OQ 6"
- [x] Add `if: ${{ !contains(github.event.pull_request.labels.*.name, 'graft') }}`
  to `ci.yml`'s `lint`, `test-go`, and `build` jobs, with a comment citing
  DESIGN-0016 OQ 6
- [x] Run `govulncheck ./...` and confirm GO-2026-4970 no longer reports

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just ci` passes; `just --list` shows no `api ...` or `ui ...` entry and
  raises no error (both modules optional and absent)
- `go version` inside the repo reports 1.26.5 via mise; CI's `setup-go` resolves
  1.26.5 from `go.mod`
- `just parity` shows zero golden diffs
- A PR carrying `graft` skips `lint`, `test-go`, `build`, and still runs
  `Check Required Labels`

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 1: config.ParseBytes

The only API addition, landed in docz alone so it is reviewed against the
library rather than inside a 160-commit merge.

<!--docz:tasks:start-->
#### Tasks

- [x] Split `userListedTypeNames(path)` into a path wrapper over
  `userListedTypeNamesIn(data []byte) []string` — *the path form ended up
  uncalled once `applyTypesReplaceOnPresence(path)` read the file itself, so it
  was removed rather than kept for the `unused` linter to flag; the path
  wrapper lives one level up*
- [x] Split `applyTypesReplaceOnPresence` the same way:
  `applyTypesReplaceOnPresenceIn(cfg, data)` with the path version reading and
  delegating
- [x] Extract `parseBytes(data []byte, defaults *Config) (Config, error)` from
  `loadFromFile`'s body; `loadFromFile` becomes `os.ReadFile` + `parseBytes`
- [x] Add exported `ParseBytes(b []byte) (Config, error)` with the doc comment
  from DESIGN-0016 (reads no file, merges no global config, normalises exactly
  as `Load`, validation is the caller's)
- [x] Table-driven `TestParseBytes_MatchesLoad`: `types:` block, explicit
  `changelog.file: ""`, `api:` block with a trailing-slash `exclude`, a custom
  type with aliases, and an empty file — each asserting
  `ParseBytes(b) == Load("", dir)` with `t.Setenv("HOME", t.TempDir())`
  (serial, because of `Setenv`)
- [x] `TestParseBytes_ReadsNoFilesystem`: a global `~/.docz.yaml` present under
  a temp `HOME` that would change the result, asserting `ParseBytes` ignores it
- [x] `TestParseBytes_DoesNotModifyInput`: the input slice is byte-identical
  after the call
- [x] Add `ParseBytes` to `test/consumer` so the new symbol is proven reachable
  from outside the module
- [x] Update the `pkg/doczcore/config` entry in `CLAUDE.md` to name
  `ParseBytes` and the shared byte core

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just ci` and `just test-consumer` pass
- `Load(path, "")` and `ParseBytes` share one code path (visible in the diff:
  `loadFromFile` is two statements)
- No existing test in `pkg/doczcore/config` changed
- `just parity` shows zero golden diffs

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: The graft

The irreversible step. Merges **red by decision** (DESIGN-0016 OQ 6): after it,
44 Go files import `github.com/donaldgifford/docz-api/…`, which no longer
exists as a module. Phase 3 fixes that.

<!--docz:tasks:start-->
#### Tasks

- [x] **(human)** In docz-api: `docz status set impl IMPL-0001 Completed`,
  `IMPL-0002 Completed`, `IMPL-0006 Completed`, via a PR that does not trigger a
  release (Open Question 2) — done in docz-api #43 (`dont-release`)
- [x] **(human)** In the same docz-api PR: a dated one-line forward pointer at
  the top of `INV-0002` and `INV-0003`, naming the successor IDs this repository
  will allocate (INV-0012, INV-0013 — reserve them by noting it here) — done in docz-api #44 (`dont-release`)
- [x] Clone docz-api fresh with `git clone --no-local` into a temp directory,
  **after** the sweep PR merges
- [x] Run `git filter-repo` with: `docs/` → `docs/archive/api/`, `deploy/` →
  `deploy/api/`, `justfile` → `api.just`, `Dockerfile` → `Dockerfile.api`,
  `CHANGELOG.md` → `docs/archive/api/CHANGELOG.md`, and `scripts/labels.sh`
  dropped (`--invert-paths`)
- [x] Record the exact `filter-repo` invocation in this document under
  Phase 2 notes below (Open Question 3)
- [x] `git remote add api-local <clone>`, `git fetch`, and
  `git merge --allow-unrelated-histories api-local/main` on a branch off `main`
- [x] Resolve the 20 root-file conflicts per DESIGN-0016 §3: union `go.mod`
  requires (drop the `docz` self-requirement), keep ours for `.docz.yaml`,
  `.prettierrc.yaml`, `LICENSE`, `CODEOWNERS`, `licenses-csv.tpl`; union
  `mise.toml`, `.markdownlint.yaml`, `.yamllint.yml`, `.codecov.yml`,
  `renovate.json5`, `catalog-info.yaml` (two components); `.gitignore` gains
  `.idea/`, `.vscode/`, `coverage.html`, `coverage.txt`,
  `.claude/donald-loop.local.md`
- [x] Fold docz-api's `CLAUDE.md` server material into ours as a new
  `## Server (internal/, cmd/docz-api)` section; fold `README.md` and
  `DEVELOPMENT.md` the same way
- [x] Merge `.goreleaser.yml`: two `builds:` entries (`docz`, `docz-api`),
  keep our `signs:`, keep docz-api's syft SBOM
- [x] `.claude/settings.json`: add `Bash(just --list)`; drop docz-api's three
  `make` entries and `Bash(git *)`
- [x] Merge `.github/`: `ci.yml` absorbs `changes`, `lint-alerts`, `security`,
  `docker-build`, `helm-unittest`, `helm-test` (Go jobs keep the `graft`
  guard); `release.yml` absorbs `publish-ghcr`/`publish-ecr` and keeps GPG
  import; `license-check.yml`, `pr-labels.yml`, `labeler.yml` unioned; the ten
  new workflows arrive as they are
- [x] Update the four `Dockerfile` references to `Dockerfile.api`
  (`docker-bake.hcl`, `compose.yaml`, `ci.yml` `docker-build`, `deploy/api/`)
  and check `.dockerignore`
- [x] `cliff.toml`: add the commit-parser rule that skips commits reachable
  only through the grafted parent; run `git-cliff` locally against the merge
  and confirm no docz-api commit appears under a docz version heading —
  **deviation:** git-cliff's parsers cannot express reachability, so the rule
  is `.cliffignore` (the 164 SHAs of `git rev-list 9616673^2`), which git-cliff
  reads natively. A control run without it shows the docz-api commits under
  `[unreleased]`; with it, none. docz's `CHANGELOG.md` is regenerated under the
  arriving config, and the imported docz-api tags were deleted locally so they
  are never pushed
- [x] Merge `.golangci.yml` as the union of enabled linters (findings fixed in
  Phase 3)
- [x] Leave `docker.just` in place for Phase 3 to fold into `api.just` (Open Question 4)
- [x] Open the PR with `graft` and `dont-release`; the body states that the Go
  jobs are skipped by decision and cites DESIGN-0016 OQ 6
- [ ] **(human)** Review and merge the red PR with a merge commit (not squash —
  squashing destroys the graft)

<!--docz:tasks:end-->

**Phase 2 notes** (Open Question 3) — the exact commands, filled in as run:

```bash
# 2026-09-23. docz-api main at 153bdc0 (#44, the forward pointers), 164 commits.
git clone --no-local https://github.com/donaldgifford/docz-api /tmp/docz-api-graft
cd /tmp/docz-api-graft
git filter-repo --force \
  --invert-paths --path scripts/labels.sh \
  --path-rename docs/:docs/archive/api/ \
  --path-rename deploy/:deploy/api/ \
  --path-rename justfile:api.just \
  --path-rename Dockerfile:Dockerfile.api \
  --path-rename CHANGELOG.md:docs/archive/api/CHANGELOG.md
# rewritten HEAD: 4e9eda6

# in docz, on chore/docz-api-phase-2-graft off main
git remote add api-local /tmp/docz-api-graft
git fetch api-local
git merge --allow-unrelated-histories --no-commit api-local/main
```

<!--docz:criteria:start-->
#### Success Criteria

- `git log docs/archive/api/` and `git log internal/store/` both reach
  commits authored in docz-api
- `git log Dockerfile.api` reaches docz-api history without `--follow`
- `git show --stat HEAD` on `main` is a merge with two parents
- No tracked file matches `git check-ignore` rules it should not, and
  `git check-ignore deploy/api/secrets/x.pem` reports ignored
- `git-cliff` output for the unreleased section contains only docz commits
- Non-Go CI (`Check Required Labels`, `Label PR`, license check) green

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: Make it build: imports, api.just, lint

The phase that turns `main` green again and is the first time both halves are
tested together.

<!--docz:tasks:start-->
#### Tasks

- [x] Rewrite `github.com/donaldgifford/docz-api/` →
  `github.com/donaldgifford/docz/v2/` in tracked `*.go` files outside
  `docs/archive/` (44 files, 107 lines)
- [x] Rewrite `github.com/donaldgifford/docz/pkg/` →
  `github.com/donaldgifford/docz/v2/pkg/` (11 files, 20 lines)
- [x] Rewrite the module path in the non-Go files: `docker-bake.hcl`
  (including `org.opencontainers.image.source`), `cliff.toml`,
  `.goreleaser.yml`, `catalog-info.yaml`, and the four `charts/docz-api/` files
- [x] `go mod tidy`; `go build ./...`
- [x] `api.just`: confirm it loads as a module (`just --list` shows `api ...`),
  fix `build-core` to write `build/bin/docz-api`
- [x] Fold `docker.just`'s four recipes into `api.just` under `[group('docker')]`,
  updated for `Dockerfile.api`, and delete `docker.just` (Open Question 4)
- [x] Wire `api::lint`, `api::test`, and `api::helm-lint` into the root `ci`
  gate
- [ ] Swap `internal/ingest/parse.go`'s `loadConfig` for
  `doczcfg.ParseBytes`; drop the `HOME` neutralisation from ingest tests that
  only needed it for `Load`
- [ ] Move `TestConfigLoadsFixtureManifest` into `internal/ingest`, then delete
  `internal/doczcontract/`
- [ ] Capture the lint baseline: `golangci-lint run ./... > /tmp/baseline.txt`
  and record the count in this document
- [ ] Commit the `golines`/`gofumpt` reflow of `internal/` and `cmd/docz-api/`
  on its own, before any finding fix
- [ ] Fix or exclude every finding; each `exclude-rules` entry carries a comment
  naming the reason (OQ 2 of DESIGN-0016)
- [ ] Add the archive-spared test: no tracked file outside `docs/archive/`
  names `github.com/donaldgifford/docz-api`, and the five archived documents
  that did still do
- [ ] Record `test/consumer/go.sum` line count before (12) and after
- [ ] Open this PR **without** the `graft` label, so the Go jobs run again

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just ci` passes, including the Go jobs, with the `graft` guard inert
- `just api test` and `just api lint` pass; `golangci-lint run ./...` reports 0
- `just parity` shows zero golden diffs
- `go list ./internal/doczcontract/...` fails (package gone)
- `grep -rn 'MkdirTemp' internal/ingest/*.go` returns nothing outside tests

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: Boundaries, archive, and publishing

Everything that makes the result enforceable and visible correctly from outside.

<!--docz:tasks:start-->
#### Tasks

- [ ] Add `TestLayerRules_PkgNeverImportsInternal` to
  `pkg/doczcore/layer_test.go`, walking `go list <modulePath>/pkg/...` and
  failing on any dependency prefixed `modulePath + "/internal/"`
- [ ] Prove the rule fires: a temporary `pkg/` import of an `internal/` package
  fails the test (not committed)
- [ ] Write `docs/archive/api/README.md`: one paragraph stating that inside
  this directory an ID means docz-api's, the archive date, and the source
  repository
- [ ] Add `docs/archive/` to `wiki.exclude` and `api.exclude` in `.docz.yaml`,
  in one commit
- [ ] Test that `wiki.exclude` and `api.exclude` agree on `docs/archive/`
- [ ] `docz wiki update` and confirm no archived page enters `mkdocs.yml`
- [ ] Add a `publish-image` job to `prerelease.yml` calling `ghcr.yml` with
  `tag: ${{ github.ref_name }}`; ECR stays gated on `vars.ECR_PUBLISH_ENABLED`
- [ ] **(human)** In GitHub package settings for `ghcr.io/donaldgifford/docz-api`,
  grant the `docz` repository write access under "Manage Actions access"
- [ ] `charts/docz-api/Chart.yaml`: `version: 0.9.0`, `appVersion: "v2.0.0-beta.3"` (Open Question 5); regenerate its README with `just api helm-docs`
- [ ] Create INV-0012 (successor to docz-api INV-0002) and INV-0013 (successor
  to docz-api INV-0003) per Open Question 6, each citing the archived original
  by path
- [ ] Update `CLAUDE.md`: the Project paragraph (`internal/` is the server's,
  no longer empty), the layer rule, the `api::` recipes, and the archive

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just ci` and `just validate` pass
- `pkg/doczcore/layer_test.go` has five `TestLayerRules_*` tests, all green
- `mkdocs.yml` nav contains no `archive/` path
- `prerelease.yml` passes `actionlint`

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: v2.0.0-beta.3

<!--docz:tasks:start-->
#### Tasks

- [ ] Local run of `just api test-integration` against `compose.yaml` passes
  (not in CI, not claimed to be)
- [ ] `just release-check` and `just api release-check` pass
- [ ] **(human)** `just release v2.0.0-beta.3` from the Phase 4 merge commit
- [ ] **(human)** Confirm `prerelease.yml` produced both binaries and the image
- [ ] **(human)** `docker pull ghcr.io/donaldgifford/docz-api:v2.0.0-beta.3`
  succeeds and `docker run … --version` reports it
- [ ] **(human)** Confirm the rendered wiki and the `api:` listing return no
  archived document
- [ ] Flip IMPL-0019 to `Completed` and DESIGN-0016 to `Implemented`

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The GitHub release for `v2.0.0-beta.3` is marked pre-release and carries
  `docz` and `docz-api` archives
- The image exists at the beta tag, signed
- `go install github.com/donaldgifford/docz/v2/cmd/docz-api@v2.0.0-beta.3`
  succeeds from a clean `GOPATH`

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `go.mod`, `go.sum` | Modify | 1.26.5; union with docz-api's 120 requirements |
| `test/consumer/go.mod`, `consumer_test.go` | Modify | 1.26.5; `ParseBytes` |
| `justfile` | Modify | `import` + two `mod?` |
| `api.just` | Create | docz-api's `justfile`, via the graft |
| `pkg/doczcore/config/config.go` | Modify | byte core, `ParseBytes` |
| `pkg/doczcore/config/parsebytes_test.go` | Create | equivalence and no-filesystem tests |
| `pkg/doczcore/layer_test.go` | Modify | `PkgNeverImportsInternal` |
| `internal/**`, `cmd/docz-api/**`, `api/**` | Create | graft; import paths rewritten |
| `internal/doczcontract/` | Delete | ADR-0004 OQ 3 |
| `charts/docz-api/**`, `contrib/**`, `deploy/api/**` | Create | graft |
| `docs/archive/api/**` | Create | 24 documents, READMEs, CHANGELOG, namespace README |
| `Dockerfile.api`, `docker-bake.hcl`, `compose.yaml`, `sqlc.yaml`, `ct.yaml` | Create | graft |
| `.github/workflows/{ci,release,prerelease}.yml` | Modify | merged jobs, `graft` guard, beta image |
| `.github/workflows/*` (10) | Create | graft |
| `.golangci.yml`, `.goreleaser.yml`, `cliff.toml`, `.gitignore`, `.docz.yaml`, `mise.toml` | Modify | reconciled |
| `CLAUDE.md`, `README.md`, `DEVELOPMENT.md`, `CONTRIBUTING.md` | Modify | server sections, just modules |
| `.checkmake.ini` | Delete | Makefile-era leftover |
| `docs/investigation/0012-*.md`, `0013-*.md` | Create | successors |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] `ParseBytes` ≡ `Load` table test, `HOME` neutralised (Phase 1)
- [ ] `ParseBytes` reads no filesystem and does not modify its input (Phase 1)
- [ ] `ParseBytes` reachable from `test/consumer` (Phase 1)
- [ ] `TestConfigLoadsFixtureManifest` relocated to ingest (Phase 3)
- [ ] Import rewrite spared the archive (Phase 3)
- [ ] `TestLayerRules_PkgNeverImportsInternal`, proven to fire (Phase 4)
- [ ] `wiki.exclude` / `api.exclude` agreement (Phase 4)
- [ ] `just parity` zero diffs at the end of every phase
- [ ] `just api test-integration` locally before the tag (Phase 5)

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- **PR #119** (DESIGN-0016 and this document) merged before Phase 0 branches
  from `main`
- **`git-filter-repo`** installed (`/opt/homebrew/bin/git-filter-repo` present)
- **docz-api write access** for the Phase 2 sweep PR
- **GHCR package permission**: `ghcr.io/donaldgifford/docz-api` is linked to
  the docz-api repository; `ghcr.yml` hardcodes `IMAGE_REPO:
  donaldgifford/docz-api`, so the image name survives the move, but a push from
  the docz repository fails until the package grants it write access (Phase 4)
- **Docker** running locally for `just api test-integration` (Phase 5)

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

> Each question is numbered; option `a` is my recommendation, later letters are
> alternatives, and the last is a free-form "other".

### 1. How are the six phases branched?


> **Resolved 2026-09-22: (a).** One branch per phase, each cut from `main` after the previous phase merges, merged with a merge commit. Phase 0 branches after PR #119 (this document and DESIGN-0016) merges.

- a. **One branch per phase, each cut from `main` after the previous phase
  merges** — the IMPL-0018 shape (#106–#111). Every PR diffs against real
  `main`, and a merged phase can't orphan the next. Costs waiting on each merge.
  *(recommendation)*
- b. Stacked branches, each on the one before, so work continues while review
  lags. The stacked-PR auto-close gotcha applies: merging a base with
  `--delete-branch` closes its child.
- c. Everything on `feat/docz-api-consolidation` as one PR. Contradicts
  DESIGN-0016's rollout and makes the graft unreviewable.
- d. Other.

### 2. How does the docz-api status sweep avoid cutting a docz-api release?

docz-api releases through `pr-semver-bump` on every merge to `main`, so a
docs-only PR there would publish a `v0.10.2` nobody wants the day before the
repository stops being where docz-api is built.


> **Resolved 2026-09-22: (a).** A normal PR in docz-api carrying the label its `pr-semver-bump` treats as no-release, so `bump-version` reports skipped and `release.yml`'s `release` and `publish-*` jobs do not run.

- a. **A PR in docz-api labelled with whatever its `pr-semver-bump` treats as
  no-release** (its `release.yml` skips when `bump-version` reports skipped),
  merged normally. Keeps docz-api's own rule that `main` only changes through a
  PR. *(recommendation)*
- b. Push directly to docz-api `main`. One commit, no release — but only if
  `release.yml`'s trigger ignores a push with no PR, which has to be checked
  first.
- c. Skip the sweep and set the three statuses in `docs/archive/api/` after the
  graft. Contradicts ADR-0004 OQ 1 ("in the home repositories, before the
  merge") and edits the archive.
- d. Other.

### 3. Where does the exact graft invocation live?


> **Resolved 2026-09-22: (a).** In this document: the Phase 2 notes block below the Phase 2 tasks holds the exact commands as run, so docz-site's IMPL can copy them with its own renames.

- a. **In this document, as a Phase 2 notes block with the exact commands**,
  filled in as they are run. The record lives beside the plan, and docz-site's
  IMPL can copy it with its own renames. *(recommendation)*
- b. A committed `scripts/graft.sh`, parameterised for both services.
  Reproducible, but it is a script run twice in the repository's life and then
  dead code.
- c. The PR description only. Discoverable from the merge commit, invisible
  from the docs.
- d. Other.

### 4. What happens to `docker.just`?

Nothing references it today, and its four recipes (`docker-build`,
`docker-buildx`, `docker-push`, `docker-ci`) name `Dockerfile`, which becomes
`Dockerfile.api`.


> **Resolved 2026-09-22: (a).** Its four recipes fold into `api.just` under `[group('docker')]`, updated for `Dockerfile.api`, and `docker.just` is deleted in Phase 3.

- a. **Fold its four recipes into `api.just` under a `[group('docker')]`**,
  updated for `Dockerfile.api` and `docker-bake.hcl`. They become
  `just api docker-build`, reachable for the first time. *(recommendation)*
- b. Drop it. `docker buildx bake` is the real interface and CI calls it
  directly; four wrappers nobody has run are not worth carrying.
- c. Keep the file and add `import 'docker.just'` inside `api.just`. Honours the
  original intent; one more file at the root for four recipes.
- d. Other.

### 5. How do the image and chart versions continue?

docz-api's last tag is `v0.10.1`, `Chart.yaml` is `version: 0.8.0`,
`appVersion: "0.10.0"`. After the move the image is tagged from this
repository's tags, so the next image is `v2.0.0-beta.3`.


> **Resolved 2026-09-22: (a).** Image tags follow the repository tag (`v2.0.0-beta.3`); `charts/docz-api` keeps its own semver and goes `0.8.0` → `0.9.0` with `appVersion: "v2.0.0-beta.3"`.

- a. **Image tags follow the repository tag (`v2.0.0-beta.3`); the chart keeps
  its own semver and bumps minor (`0.9.0`) with `appVersion: "v2.0.0-beta.3"`.**
  A chart version describes the chart, and its consumers should not see a 0.8
  → 2.0 jump because the source moved. *(recommendation)*
- b. Chart version tracks the repository tag too (`2.0.0-beta.3`). One number
  everywhere; a chart major jump with no chart change.
- c. Keep publishing images as `v0.x` from a separate tag namespace
  (`docz-api/v0.11.0`). Preserves continuity; contradicts ADR-0004 Decision 1's
  one version line.
- d. Other.

### 6. When are the INV-0002 and INV-0003 successors written?

Both are about docz-site — INV-0002 auto-generating the OpenAPI client, INV-0003
the docz-site features blocked on the API surface — and DESIGN-0016 places the
successors in Phase 4.


> **Resolved 2026-09-22: (a).** Phase 4, as DESIGN-0016 says: INV-0012 and INV-0013, each stating what was still open and citing the archived original by path.

- a. **Phase 4, as DESIGN-0016 says**, each a short successor that states what
  was still open and cites the archived original. They become the entry point
  docz-site's design reads, which is exactly when it needs them.
  *(recommendation)*
- b. Defer both to the docz-site move and write them there, where the work they
  carry actually happens. The forward pointers in docz-api would then name IDs
  allocated later.
- c. One combined successor, since both concern the same consumer.
- d. Other.

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0016](../design/0016-move-docz-api-in-internal-cmddocz-api-api-charts-and.md)
  — the design this implements; §2 (just), §3 (root files), §4 (`.github/`),
  §5 (imports), §6 (graft), §7 (layer rule), §8 (archive), and its six resolved
  open questions.
- [ADR-0004](../adr/0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md)
  — the consolidation decision.
- [IMPL-0018](../impl/0018-v200-beta1-the-docz-api-as-one-unit-structured-regions-and-the.md)
  — the per-phase-PR shape this follows.
- [INV-0011](../investigation/0011-consolidating-docz-api-and-docz-site-into-one-repo-layout.md)
  — the inventory.
- [`git filter-repo`](https://github.com/newren/git-filter-repo)
- [just modules](https://just.systems/man/en/modules.html)

<!--docz:references:end-->
