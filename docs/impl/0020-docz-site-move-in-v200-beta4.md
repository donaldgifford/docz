---
id: IMPL-0020
title: "docz-site move-in: v2.0.0-beta.4"
status: Draft
author: Donald Gifford
created: 2026-09-23
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0020: docz-site move-in: v2.0.0-beta.4

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 0: Root preparation](#phase-0-root-preparation)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 1: The docz-site status sweep](#phase-1-the-docz-site-status-sweep)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 2: The graft](#phase-2-the-graft)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 3: One spec, the ui image, and the loose ends](#phase-3-one-spec-the-ui-image-and-the-loose-ends)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 4: Archive, docs, and the publish job](#phase-4-archive-docs-and-the-publish-job)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 5: v2.0.0-beta.4](#phase-5-v200-beta4)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. How are the six phases branched?](#1-how-are-the-six-phases-branched)
  - [2. How is Phase 0's parameterised publish proven before a tag exists?](#2-how-is-phase-0s-parameterised-publish-proven-before-a-tag-exists)
  - [3. How is the npm-tree fence pinned?](#3-how-is-the-npm-tree-fence-pinned)
  - [4. Does trufflehog take docz-site's stricter setting?](#4-does-trufflehog-take-docz-sites-stricter-setting)
  - [5. How does the docz-site sweep avoid cutting a docz-site release?](#5-how-does-the-docz-site-sweep-avoid-cutting-a-docz-site-release)
  - [6. What happens to docz-site's "bump the chart in the same PR" rule?](#6-what-happens-to-docz-sites-bump-the-chart-in-the-same-pr-rule)
- [References](#references)
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Move docz-site into this repository as `ui/`, with its chart at
`charts/docz-site`, its 148 commits grafted on, and its 25 docz files
archived verbatim under `docs/archive/ui/`. orval then reads
`api/openapi.yaml` directly, so the spec has one copy. `node_modules` is
fenced off from Go tooling, and both images and both charts publish from
one hand-cut `v2.0.0-beta.4`. Six phases, one PR each.

**Implements:** DESIGN-0017 (Approved; all nine open questions resolved (a)), which
implements ADR-0004 Decisions 2, 4, 7, and 8 for the frontend half.

Tasks marked **(human)** need a person: a commit to another repository, a
merge, a tag push, or a GitHub settings change. An automated run marks them
`deferred - human required` and continues.

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- Root preparation that is safe before `ui/` exists: `.dockerignore`, a
  two-component `docker-bake.hcl`, `ghcr.yml`/`ecr.yml` parameterised by
  `component`, the chart-changelog include-path fix, Bun and Node in
  `mise.toml`, and the `ui` path-filter output (DESIGN-0017 §6–§8, OQ 5)
- The status sweep of four documents in docz-site, and closing its PR #33
  (DESIGN-0017 §9, OQ 9)
- The `filter-repo` graft with seven renames, a `--no-tags` fetch, and the
  `--allow-unrelated-histories` merge. Green on arrival (DESIGN-0017 §2)
- `ui/go.mod`, the fence around `node_modules`, and the checks that pin it
  (§3, OQ 2)
- Folding docz-site's repository-level files and `.github/` into ours, and
  the `ui.just` edits (§5, §6)
- The `ui` and `ui-e2e` CI jobs, CodeQL for `javascript-typescript`, and
  `helm-unittest` over both charts (§7)
- orval on `../api/openapi.yaml`, with the vendored spec and `spec-drift.yml`
  deleted (§4)
- `Dockerfile.ui` with a named `spec` build context, and bake `-ui` targets
  (§8, OQ 3)
- MSW fixtures snapshotted into `ui/src/mocks/content/` (OQ 4)
- `deploy/ui/` contexts and the local-network check (§10)
- Repository URL rewrites, and the `test/archive` additions
- `docs/archive/ui/README.md`, chart `0.2.0`, the `prerelease.yml` ui publish
  job, CLAUDE.md and the other root docs, and INV-0012 concluded
- `v2.0.0-beta.4`

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Any change to the application: routes, the markdown pipeline, the Bun
  server, the runtime config contract, or chart values
- Any Go API change. `pkg/` and `internal/` are untouched apart from new
  tests under `test/archive`
- Embedding `ui/dist` in docz-api (DESIGN-0017 Non-Goals)
- A JavaScript licence check (a follow-up; docz-site's PR #33 proposed one)
- INV-0013's deferred features
- #127 and its markdownlint gate. That work is independent and can land
  before or after this
- Archiving the docz-site repository read-only, which follows beta.4
  verification
- v2.0.0 proper, restoring the `latest` image tag, and removing the
  `EXPERIMENTAL` markers

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met. Phases 0 and 1 touch
different repositories and can merge in either order. Everything from
Phase 2 on is strictly sequential. Every PR in this repository carries
`dont-release`.

---

<!--docz:phase:start-->
### Phase 0: Root preparation

Every change the site will need that is safe while `ui/` does not exist,
proven against docz-api alone. When the graft arrives, the publish path
already knows about two components and has been exercised with one.

<!--docz:tasks:start-->
#### Tasks

- [x] `.dockerignore`: add `ui/`, so `Dockerfile.api`'s repository-root
  context never uploads `ui/node_modules/`
- [x] `docker-bake.hcl`: rename `_common` → `_common_api` and the targets to
  `dev-api`, `ci-api`, `release-api`, with `IMAGE_NAME` scoped per target;
  add `group "default" { targets = ["dev-api"] }` and
  `group "ci" { targets = ["ci-api"] }` so the bare `docker buildx bake` and
  `docker buildx bake ci` spellings keep working
- [x] `api.just`: update the bake recipes (`docker buildx bake release` →
  `release-api`, and so on); `just --dry-run api <recipe>` shows each
  resolved command
- [x] `ghcr.yml` and `ecr.yml`: add a `component` input (`api`|`ui`, default
  `api` so a bare `workflow_dispatch` behaves as today). A first step resolves
  it to `IMAGE_REPO`, `BAKE_TARGET`, `CHART_NAME`, and `CHART_DIR` from one
  table, as in DESIGN-0017 §8. Every hardcoded `docz-api`, `charts/docz-api`,
  and `targets: release` reads from those values
- [x] Chart-changelog step: `--include-path "charts/**"` →
  `"${CHART_DIR}/**"`, with the config at `${CHART_DIR}/cliff.toml`
- [x] `prerelease.yml` and `release.yml`: pass `component: api` to both
  publish calls
- [x] `ci.yml` `changes`: add the `ui` output (`ui/**`, `api/openapi.yaml`,
  `Dockerfile.ui`). No job reads it until Phase 2
- [x] `mise.toml`: `bun = "1.3.14"`, `node = "24.14.0"`
- [x] `renovate.json5`: add `github>donaldgifford/renovate-config:node`
- [x] `just api lint-actions` clean
- [x] Prove the parameterised publish without a tag (Open Question 2):
  `gh workflow run ghcr.yml --ref <branch> -f component=api -f tag=v0.0.0-phase0 -f dry_run=true`,
  and the same for `ecr.yml` if `ECR_PUBLISH_ENABLED` is set. Record the run
  URLs here.
  Run with `-f tag=v2.0.0-beta.3` rather than `v0.0.0-phase0`, because the
  image job checks out `inputs.tag` and a tag that does not exist fails the
  checkout before `dry_run` can skip anything:
  [run 35950181641](https://github.com/donaldgifford/docz/actions/runs/35950181641),
  all three jobs green. The resolve job logged
  `component=api -> donaldgifford/docz-api, release-api, charts/docz-api`;
  the image job skipped bake and pushed nothing; the chart job found chart
  0.9.0 already published and took the `helm pull` skip path. `ecr.yml` was
  not run: `ECR_PUBLISH_ENABLED` is not set

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just ci` passes, and `just parity` shows zero golden diffs
- `docker buildx bake --print ci` resolves to one target, `ci-api`, whose
  context, Dockerfile, args, and labels equal the old `ci` target's
- CI's `docker-build` job is green on the Phase 0 PR
- The `ghcr.yml` dry run resolves `component=api` to `donaldgifford/docz-api`,
  `release-api`, and `charts/docz-api`, packages the chart, and pushes nothing
- `grep -n 'docz-api' .github/workflows/ghcr.yml .github/workflows/ecr.yml`
  matches only the resolution table

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 1: The docz-site status sweep

Record-keeping in the home repository, before the clone, so the archive's
frontmatter is honest and the correction lives in that repository's
history (ADR-0004 OQ 1, DESIGN-0017 §9).

<!--docz:tasks:start-->
#### Tasks

- [x] **(human)** In docz-site, on a branch: `docz status set impl IMPL-0001 Completed`,
  `docz status set impl IMPL-0002 Completed`,
  `docz status set design DESIGN-0001 Implemented`,
  `docz status set design DESIGN-0005 Implemented`, then `docz update`
  so the README indexes follow. Done in docz-site PR #39 (`chore/move`)
- [x] **(human)** Open it as a PR labelled `dont-release` (Open Question 5)
  and merge it. docz-site's `pr-semver-bump` lists `dont-release` in
  `noop-labels`. docz-site #39, merged as `f17ba51`
- [x] Confirm the merge cut no tag and published no image: `git ls-remote --tags`
  unchanged at 13, and the `publish-ghcr` chart job, which runs even on
  `dont-release`, skipped on its `helm pull` idempotency check because
  `Chart.yaml` is unchanged at `0.1.10`. Confirmed: 13 tags, unchanged. The Release run published no image, and its chart job logged
  `Chart 0.1.10 already published, skipping.`
- [ ] **(human)** Close docz-site PR #33 with a comment that the move to
  `donaldgifford/docz` supersedes it, and that a JavaScript licence check
  is a follow-up there
- [x] Record docz-site's `main` SHA after the sweep and its auto-sync
  changelog commit settle. Phase 2 clones exactly that SHA: **`f1203c91d1a9f69ccdae140d9eabc3185f1bce8f`** (`chore(changelog): Auto-sync`
  after #39). The four documents read Completed/Implemented at that SHA

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The four documents carry their corrected statuses on docz-site `main`
- docz-site has no new tag, image, or chart version
- PR #33 is closed
- The SHA Phase 2 will clone is written in this document

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: The graft

The irreversible step, and unlike IMPL-0019's it is **green on arrival**:
no Go file arrives, the vendored spec keeps orval working, and
`ui/.github/` is inert until it is folded. No `graft` label.

<!--docz:tasks:start-->
#### Tasks

- [x] `git clone --no-local https://github.com/donaldgifford/docz-site /tmp/docz-site-graft`
  at the Phase 1 SHA
- [x] Run `git filter-repo`, dropping `scripts/labels.sh` (byte-identical
  to ours) and `ct.yaml` (identical bar one blank line) with `--invert-paths`,
  then `--path-rename` in order: `:ui/`, `ui/docs/:docs/archive/ui/`,
  `ui/CHANGELOG.md:docs/archive/ui/CHANGELOG.md`,
  `ui/charts/docz-site/:charts/docz-site/`, `ui/Dockerfile:Dockerfile.ui`,
  `ui/deploy/:deploy/ui/`, `ui/justfile:ui.just`. Record the exact
  invocation in the Phase 2 notes below
- [x] Verify the rewrite before merging:
  `git ls-tree --name-only HEAD` lists exactly `Dockerfile.ui`, `charts`,
  `deploy`, `docs`, `ui`, and `ui.just`, and `git ls-tree -r --name-only HEAD charts`
  holds only `charts/docz-site/` plus `charts/.yamllint.yml`
- [x] `git remote add site-local /tmp/docz-site-graft`,
  `git fetch --no-tags site-local`, then on a branch off `main`
  `git merge --allow-unrelated-histories --no-commit site-local/main`.
  `git tag | wc -l` is unchanged afterwards
- [x] Resolve `charts/.yamllint.yml` as the union of both (the only expected
  conflict; hashes `2be451f` and `e423206` differ)
- [x] Add `ui/go.mod`: `module github.com/donaldgifford/docz/v2/ui`, with a
  comment saying it exists only to keep `node_modules` out of the root
  module's `./...` (DESIGN-0017 §3). Verify with a real `just ui install`
  followed by `go list ./... | grep -c /ui/` printing `0`
- [x] Fold the repository-level files per DESIGN-0017 §6, then delete them
  from `ui/`: `mise.toml` and `renovate.json5` (already done in Phase 0),
  `catalog-info.yaml` (a third `Component`, `docz-site`, source
  `…/tree/main/ui`), `.claude/settings.json` (union of `allow`), and drop
  `.codecov.yml`, `.docz.yaml`, `cliff.toml`, `.yamllint.yml`,
  `.yamlfmt.yml`, `LICENSE`, and `.forge-lock.hcl`. The `allow` union
  added nothing: four of docz-site's five extra entries (`git commit -m ' *`,
  `make lint *`, `make lint-md *`, `make test *`) are already covered by
  ours, and the fifth, a blanket `Bash(git *)`, was left out on purpose
  because it would auto-approve `git push --force` and `git reset --hard`
- [x] Keep in `ui/` (DESIGN-0017 OQ 1): `CLAUDE.md`, `README.md`,
  `CONTRIBUTING.md`, `.markdownlint.yaml`, `.gitignore`, `.dockerignore`,
  `.prettierrc.yaml`, `.prettierignore`, and every JS tool config
- [x] Fold `ui/.github/` and delete it whole, `spec-drift.yml` included,
  since nothing under `ui/.github/` runs: `ci.yml` becomes the `ui` and
  `ui-e2e` jobs below; CodeQL's matrix gains `javascript-typescript` with
  `security-extended`; root `labeler.yml` gains a `ui` rule on `ui/**`;
  trufflehog per Open Question 4. `ghcr.yml`, `release.yml`, `changelog*.yml`,
  `pr-labels.yml`, `labeler.yml`, `actionlint.yml`, and `CODEOWNERS` drop.
  Trufflehog: the local `--results=verified,unknown` scan over the merged
  history (trufflehog 3.95.9) found 0 verified and 7 unknown, every one a
  placeholder Postgres DSN in docz's own history (none in `ui/`). The four
  paths are excluded in `.github/trufflehog-exclude.txt`, each with its
  reason, and the re-scan finds nothing
- [x] `ci.yml`: a `ui` job (`needs: changes`, `if: needs.changes.outputs.ui == 'true'`,
  `defaults.run.working-directory: ui`, `oven-sh/setup-bun` pinned to
  1.3.14, `extractions/setup-just`) running `bun install --frozen-lockfile`,
  then `just ui` `gen-api`, `lint`, `fmt-check`, `typecheck`, `test`,
  `test-server` (before `build`: the server tests assume no `dist/`), `build`,
  `bundle-budget`, and `gen-api-check`, one step each. Then
  the fence check (Open Question 3). A `ui-e2e` job does the same install
  plus `bunx playwright install --with-deps chromium` and `just ui e2e`.
  `ui-e2e` also runs `just ui gen-api` first: the generated client is
  gitignored and the MSW preview build imports it
- [x] `ci.yml` `helm-unittest`: loop over `charts/*/` instead of naming
  `charts/docz-api`
- [x] `ui.just`: `set working-directory := "ui"`; helm recipes →
  `../charts/docz-site`; `local-up`/`local-down` →
  `../deploy/ui/compose.local.yaml`, with the hint naming `just api local-up`.
  `default` became `just --list ui`: `ui/` has no justfile of its own, so
  the inherited bare `just --list` climbed to the root's and listed that.
  `helm-docs` searches `../charts/docz-site` only, not both charts
- [ ] Root `justfile` `ci`: append `ui::install ui::gen-api ui::lint
  ui::fmt-check ui::typecheck ui::test ui::test-server ui::build
  ui::bundle-budget ui::gen-api-check` (DESIGN-0017 OQ 6)
- [ ] Commit the merge, then append `git rev-list <merge>^2` to
  `.cliffignore`. Run `git-cliff` locally twice: a control run without the
  new lines shows docz-site commits under `[unreleased]`, and with them it
  shows none. Regenerate `CHANGELOG.md`
- [ ] `just ci` and `just ui ci` locally
- [ ] Open the PR with `dont-release`; the body lists the folds and cites
  DESIGN-0017 §2 and §6
- [ ] **(human)** Review and merge **with a merge commit** (squashing destroys
  the graft)

<!--docz:tasks:end-->

**Phase 2 notes**, the exact commands, filled in as run:

```bash
git clone --no-local https://github.com/donaldgifford/docz-site /tmp/docz-site-graft
cd /tmp/docz-site-graft
git checkout -B main f1203c91d1a9f69ccdae140d9eabc3185f1bce8f

# Two passes: the drop, then the renames (applied in order).
git filter-repo --force --refs main --invert-paths \
  --path scripts/labels.sh --path ct.yaml
git filter-repo --force --refs main \
  --path-rename :ui/ \
  --path-rename ui/docs/:docs/archive/ui/ \
  --path-rename ui/CHANGELOG.md:docs/archive/ui/CHANGELOG.md \
  --path-rename ui/charts/docz-site/:charts/docz-site/ \
  --path-rename ui/charts/.yamllint.yml:charts/.yamllint.yml \
  --path-rename ui/Dockerfile:Dockerfile.ui \
  --path-rename ui/deploy/:deploy/ui/ \
  --path-rename ui/justfile:ui.just
# -> 150 commits, rewritten tip 9458076735e23b0b9f7a001bbbddf42e71b7e9a3

cd ~/code/docz   # on feat/docz-site-graft, cut from main after #129
git remote add site-local /tmp/docz-site-graft
git fetch --no-tags site-local
git merge --allow-unrelated-histories --no-commit site-local/main
# one conflict, charts/.yamllint.yml: union of the two ignore lists
git commit    # 3630665; git tag | wc -l is 26 before and after
```

The list above lacked one rename: docz-site's `charts/.yamllint.yml` sits
beside `charts/docz-site/`, not inside it, so without
`ui/charts/.yamllint.yml:charts/.yamllint.yml` it landed at
`ui/charts/.yamllint.yml` and the expected conflict never happened. The
first attempt was thrown away and re-cloned. `ct.yaml` differed from ours
by one line, a stray helm-testsuite `$schema` comment, not a blank line.
The merge was committed on its own, with only the conflict resolved, and
the folds below are separate commits on top of it, so the merge commit is
the graft and nothing else.

<!--docz:criteria:start-->
#### Success Criteria

- Every CI job is green on the PR, including the new `ui` and `ui-e2e` jobs
- `just ci` (now with the ui gates) and `just ui ci` pass locally
- `git log --oneline -- ui/src/markdown | wc -l` and
  `git log --oneline -- Dockerfile.ui | wc -l` are both non-zero, so history
  followed the renames
- `git tag` is unchanged, and no docz-site commit appears in a regenerated
  `CHANGELOG.md`
- After `just ui install`, `go list ./...` lists no package under `ui/`
- `just parity` shows zero golden diffs
- `ui/.github/` does not exist

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: One spec, the ui image, and the loose ends

Everything that changes behaviour rather than location: the spec swap, the
image build, the fixtures, the local stacks, and the URLs.

<!--docz:tasks:start-->
#### Tasks

- [ ] `ui/orval.config.ts`: `input: "../api/openapi.yaml"`. Run
  `just ui gen-api-check` with `ui/api/` still present and again after
  deleting it. Both pass, which proves the swap is byte-neutral
  (DESIGN-0017 §4)
- [ ] Delete `ui/api/`
- [ ] Drift drill, recorded here and not kept: on a scratch branch, remove
  a response field the UI reads from `api/openapi.yaml`. `just ui typecheck`
  fails, and the contract test fails. Record both outputs
- [ ] `Dockerfile.ui`: `COPY --from=spec openapi.yaml` to the path that makes
  orval's `../api/openapi.yaml` resolve in the build stage (DESIGN-0017 OQ 3)
- [ ] `docker-bake.hcl`: `_common_ui` (image `donaldgifford/docz-site`,
  `context = "ui"`, `dockerfile = "../Dockerfile.ui"`,
  `contexts = { spec = "api" }`), with `dev-ui`, `ci-ui`, and `release-ui`;
  `group "ci"` gains `ci-ui`
- [ ] `ci.yml` `docker-build`: run when `docker` **or** `ui` changed
- [ ] `ui.just`: a `docker-build` recipe spelling
  `docker build -f ../Dockerfile.ui --build-context spec=../api .`
- [ ] `docker buildx bake dev-ui`, then run the image with `DOCZ_API_URL`
  pointed at nothing: `/healthz` returns 200, `/readyz` behaves as the
  site's CLAUDE.md says (it never calls docz-api), and `/` serves
  `index.html` with `window.__DOCZ_CONFIG__` injected
- [ ] MSW fixtures (DESIGN-0017 OQ 4): copy the seven archived files that
  `ui/src/mocks/fixtures.ts` imports into `ui/src/mocks/content/`
  (`docz-site-changelog.md`, `docz-site-design-0001.md`,
  `docz-site-design-index.md`, `docz-site-guides-markdown-specimen.md`,
  `docz-site-impl-0001.md`, `docz-site-impl-index.md`, `docz-site-input.md`),
  repoint the imports, keep `README.md` live, and reword the "always current"
  comment. `just ui e2e` passes
- [ ] `deploy/ui/compose.yaml`: docz-api builds from `context: ../..`,
  `dockerfile: Dockerfile.api`, and the site from `../../ui` with
  `additional_contexts: { spec: ../../api }`. `docker compose config` is valid
- [ ] `deploy/ui/compose.local.yaml`: run `just api local-up` and read the
  network name it creates. If it is no longer `docz-api-local_default`,
  pin `name:` in the api local stack rather than renaming the site's
  reference. `just ui local-up` then starts
- [ ] Rewrite `github.com/donaldgifford/docz-site` → the monorepo (`…/docz`,
  `…/docz/tree/main/ui` where a path is meant) in `ui/README.md`,
  `ui/CLAUDE.md`, `ui/CONTRIBUTING.md`, `charts/docz-site/Chart.yaml`
  `home:`, `charts/docz-site/README.md.gotmpl` (then
  `just ui helm-docs`), and `charts/docz-site/cliff.toml`. Also retarget
  `ui/README.md`'s links to docz-api, which now points at this repository.
  The demo-org slug `donaldgifford/docz-site` in fixtures and e2e specs stays
- [ ] `test/archive`: `TestSiteRepositoryURLGone`. No tracked file outside
  `docs/`, `testdata/`, `CHANGELOG.md`, and `test/archive/` names
  `github.com/donaldgifford/docz-site`
- [ ] `test/archive`: the fence test (Open Question 3)

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just ci`, `just ui ci`, and `just parity` pass
- `find ui -name openapi.yaml -not -path '*/node_modules/*'` finds nothing,
  and `spec-drift` appears nowhere under `.github/`
- `docker buildx bake --print ci` lists `ci-api` and `ci-ui`, and CI's
  `docker-build` builds both
- The ui image serves `/healthz`, `/readyz`, and `/` as described
- The drift drill's two failures are recorded in this document
- `TestSiteRepositoryURLGone` and the fence test pass

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: Archive, docs, and the publish job

What the outside world sees at beta.4, and the notes the next person needs.

<!--docz:tasks:start-->
#### Tasks

- [ ] `docs/archive/ui/README.md`: the namespace rule in one line (*inside
  this directory, an ID means docz-site's*), mirroring
  `docs/archive/api/README.md`. Add `docs/archive/README.md` listing both
  trees
- [ ] Confirm `wiki.exclude` and `api.exclude` already hide
  `docs/archive/ui/` with no edit: `TestExcludesAgreeOnArchive` passes
  unchanged, and `docz wiki update --dry-run` names no `archive` path
- [ ] `charts/docz-site/Chart.yaml`: `version: 0.2.0`,
  `appVersion: "2.0.0-beta.4"` (bare; DESIGN-0017 OQ 8). Regenerate with
  `just ui helm-docs`. The chart's bare-semver unit test passes
- [ ] Update the chart-bump rule in `ui/CLAUDE.md` per Open Question 6
- [ ] `prerelease.yml`: add `publish-image-ui` (`ghcr.yml`, `component: ui`,
  `tag: ${{ github.ref_name }}`) and `publish-ecr-ui` (gated on
  `vars.ECR_PUBLISH_ENABLED`), with the same permissions ceiling as the api
  jobs. Add the same pair to `release.yml`
- [ ] Root `CLAUDE.md`: a "Frontend (`ui/`)" section pointing at
  `ui/CLAUDE.md`, covering `ui/go.mod` and why it exists, the single spec
  and its four readers, the named `spec` build context, two-component
  publishing and the per-package GHCR grant, and the path-filter revisit
  condition (DESIGN-0017 OQ 7). Build & Test gains `just ui ci`. The opening
  paragraph names `ui/`
- [ ] `README.md`, `DEVELOPMENT.md`, and `CONTRIBUTING.md`: a frontend
  section (Bun via `mise install`, `just ui install`, `just ui dev`,
  `just ui ci`) and the two-chart note
- [ ] INV-0012: fill in Findings and Conclusion (**Answer:** confirmed,
  generation straight from `api/openapi.yaml` per DESIGN-0017 §4, with
  the typecheck and `gen-api-check` arms), then
  `docz status set investigation INV-0012 Concluded`
- [ ] **(human)** GHCR → Packages → `docz-site` → Manage Actions access:
  add `donaldgifford/docz` with **Write**. Do the same for
  `charts/docz-site`. Both are separate grants from docz-api's (IMPL-0019
  Phase 5: the chart publish failed `403 write_package` without one)
- [ ] `just validate`, `just api lint-actions`

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just ci`, `just ui ci`, `just validate`, and `just parity` pass
- `prerelease.yml` has an api pair and a ui pair of publish jobs, and
  actionlint is clean
- Both GHCR grants are in place before the tag
- INV-0012 is Concluded
- The archive does not appear in the wiki nav or the `api:` listing

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: v2.0.0-beta.4

<!--docz:tasks:start-->
#### Tasks

- [ ] `just release-check` and `just api release-check` pass
- [ ] `docker buildx bake ci` builds both images locally
- [ ] **(human)** `just release v2.0.0-beta.4` from **Phase 4's merge
  commit**, the last commit that changes a workflow. A tag runs the
  workflows as they exist in the tagged commit (IMPL-0019 Phase 5)
- [ ] Confirm `prerelease.yml`: the GitHub release is a pre-release carrying
  the `docz_*` and `docz-api_*` archives; `publish-image` pushed
  `docz-api:2.0.0-beta.4` (chart `docz-api` `0.9.0` skipped as already
  published); `publish-image-ui` pushed `docz-site:2.0.0-beta.4` and chart
  `docz-site` `0.2.0`
- [ ] `cosign tree ghcr.io/donaldgifford/docz-site:2.0.0-beta.4` shows a
  signature and SLSA provenance. `helm show chart
  oci://ghcr.io/donaldgifford/charts/docz-site --version 0.2.0` reports
  `appVersion: 2.0.0-beta.4`
- [ ] **(human)** `docker run` the published docz-site image, confirm
  `/healthz`, and point it at a docz-api to confirm the proxy (`/api/v1/repos`
  through the site). Upgrading an existing docz-site release to chart
  `0.2.0` needs no values change
- [ ] Closing notes below: amend ADR-0004's two claims that DESIGN-0017's
  Background disproved (the "no `go.mod`" line and the successor count)
- [ ] `docz status set impl IMPL-0020 Completed` and
  `docz status set design DESIGN-0017 Implemented`, then `docz update`
- [ ] File the follow-ups as issues: archive the docz-site repository
  read-only with a README pointing at `ui/`; a Bun licence check; v2.0.0
  proper with `latest` restored for both images

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The GitHub release for `v2.0.0-beta.4` is a pre-release with both
  binaries' archives
- `ghcr.io/donaldgifford/docz-site:2.0.0-beta.4` and
  `ghcr.io/donaldgifford/docz-api:2.0.0-beta.4` exist, are signed, and carry
  provenance. Neither is tagged `latest`
- `charts/docz-site` `0.2.0` is published with `appVersion: 2.0.0-beta.4`
- IMPL-0020 is Completed and DESIGN-0017 Implemented

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| --- | --- | --- |
| `.dockerignore` | Modify | exclude `ui/` |
| `docker-bake.hcl` | Modify | `-api`/`-ui` targets, per-target image, spec context |
| `.github/workflows/ghcr.yml`, `ecr.yml` | Modify | `component` input; include-path fix |
| `.github/workflows/prerelease.yml`, `release.yml` | Modify | api and ui publish pairs |
| `.github/workflows/ci.yml` | Modify | `ui` filter; `ui`, `ui-e2e` jobs; helm loop; docker on ui |
| `.github/workflows/codeql.yml` | Modify | `javascript-typescript` |
| `.github/labeler.yml` | Modify | `ui` rule |
| `mise.toml`, `renovate.json5`, `catalog-info.yaml`, `.claude/settings.json` | Modify | Bun/Node, `:node`, third component, allow union |
| `api.just` | Modify | bake target names |
| `justfile` | Modify | `ci` gains the ui gates |
| `ui/**` | Create | graft (≈220 files) |
| `ui/go.mod` | Create | the `node_modules` fence |
| `ui.just` | Create | graft of docz-site's `justfile`, with edits |
| `Dockerfile.ui` | Create | graft, plus the spec context |
| `charts/docz-site/**` | Create | graft; `0.2.0`; URLs |
| `charts/.yamllint.yml` | Modify | union |
| `deploy/ui/**` | Create | graft; contexts |
| `docs/archive/ui/**` | Create | 25 files and `CHANGELOG.md`, verbatim, plus the namespace README |
| `docs/archive/README.md` | Create | lists both archive trees |
| `ui/api/`, `spec-drift.yml` | Delete | one spec |
| `ui/src/mocks/content/*` | Create | seven snapshotted fixtures |
| `.cliffignore` | Modify | the graft's second-parent rev-list |
| `test/archive/*_test.go` | Modify | site URL test, fence test |
| `CLAUDE.md`, `README.md`, `DEVELOPMENT.md`, `CONTRIBUTING.md` | Modify | frontend sections |
| `docs/investigation/0012-*.md` | Modify | Concluded |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Phase 0's parameterised `ghcr.yml` dry run resolves `component=api`
  exactly as the hardcoded version did (Phase 0)
- [ ] The graft's history follows the renames, and no tag or changelog entry
  leaks (Phase 2)
- [ ] `go list ./...` lists nothing under `ui/` with `node_modules`
  populated, locally and in CI (Phases 2–3)
- [ ] orval's swap to `../api/openapi.yaml` is byte-neutral (Phase 3)
- [ ] A spec change that breaks the client fails `just ui typecheck`, drilled
  once (Phase 3)
- [ ] The ui image builds from bake and serves its probes (Phase 3)
- [ ] `TestSiteRepositoryURLGone` and `TestExcludesAgreeOnArchive` (Phases 3–4)
- [ ] `just parity` zero diffs at the end of every phase
- [ ] Published images signed with provenance, charts at the right
  `appVersion` (Phase 5)

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- **The PR carrying DESIGN-0017 and this document** merged before Phase 0
  branches from `main`
- **`git-filter-repo`** installed (used for IMPL-0019)
- **docz-site write access** for Phase 1
- **Bun 1.3.14** locally once Phase 0 lands (`mise install`), since the root
  `just ci` runs the ui gates from Phase 2 on
- **GHCR Actions access** for `docz-site` and `charts/docz-site`, granted
  to `donaldgifford/docz` before the tag (Phase 4)
- **Docker** for the image, compose, and drill tasks (Phases 3 and 5)

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

> Each question is numbered; option `a` is my recommendation, later letters are
> alternatives, and the last is a free-form "other".

### 1. How are the six phases branched?

> **Resolved 2026-09-23: (a).** One branch per phase, each cut from `main` after the
> previous phase merges, merged with a merge commit. Phases 0 and 1 may run in
> parallel. Phase 0 branches after the PR carrying DESIGN-0017 and this
> document merges.

- a. **One branch per phase, each cut from `main` after the previous phase
  merges**, with merge commits, as IMPL-0018 and IMPL-0019 did (#106–#111,
  #120–#124). Every PR diffs against real `main`, and a merged phase cannot
  orphan the next. Phases 0 and 1 can run in parallel because they touch
  different repositories. *(recommendation)*
- b. Stacked branches, so work continues while review lags. The stacked-PR
  auto-close gotcha applies: merging a base with `--delete-branch` closes
  its child.
- c. One PR for everything. Buries the graft's review under the spec swap and
  the workflow changes, which is what DESIGN-0017 §2's ordering exists to
  avoid.
- d. Other.

### 2. How is Phase 0's parameterised publish proven before a tag exists?

DESIGN-0017 said "the next docz-api publish proves it", but on the v2 line
publishes happen only on a beta tag, and the next one is beta.4, which
also introduces `ui`. `ghcr.yml`'s `dry_run` skips the bake step entirely,
so it proves input resolution, metadata, and chart packaging, but not that
the bake target names are right.

> **Resolved 2026-09-23: (a).** A `workflow_dispatch` dry run of `ghcr.yml` with
> `component=api` (and `ecr.yml` if enabled) proves resolution and chart
> packaging, and CI's `docker-build` job proves the renamed bake targets. Both
> run URLs are recorded in Phase 0.

- a. **Both halves of the proof, from two places.** A `workflow_dispatch`
  dry run of `ghcr.yml` with `component=api` proves resolution and chart
  packaging, and CI's `docker-build` job proves the renamed bake targets
  build (`ci` → `ci-api`). Together they cover every line Phase 0 changes,
  and neither pushes anything. *(recommendation)*
- b. **A real publish under a throwaway tag** (dispatch with
  `tag=v0.0.0-phase0`, not dry). End to end, including signing and
  provenance; leaves a junk image and attestation in GHCR that has to be
  deleted by hand, and the chart push is idempotent on version, so it
  proves nothing about the chart.
- c. **No proof until the beta.4 tag.** The cheapest; a refactor of the
  release path is then first exercised by the release, which is the
  failure mode IMPL-0019 Phase 5 already paid for once (the chart 403).
- d. Other.

### 3. How is the npm-tree fence pinned?

DESIGN-0017's testing strategy asks for a check on the *property* ("no Go
package under `ui/`, even with `node_modules` populated"), not just the
file. But `just test` runs where `ui/node_modules` usually does not exist,
and CI's Go jobs never install the UI, so a Go test alone would pass
vacuously.

> **Resolved 2026-09-23: (a).** A Go test in `test/archive` asserts `ui/go.mod`
> exists and `go list` returns nothing under `ui/`, and the `ui` CI job runs
> `go list ./... | grep /ui/` after `bun install` and fails on a match.

- a. **Two checks, one per half.** A Go test in `test/archive` asserts that
  `ui/go.mod` exists and that `go list <modulePath>/...` returns nothing
  under `ui/`, which is cheap and runs in `just test`. The `ui` CI job runs
  `go list ./... | grep /ui/` after `bun install` and fails on a match,
  which is the only place `flatted` is guaranteed present. The first catches
  a deleted `go.mod`; the second catches a fence that stopped working.
  *(recommendation)*
- b. **One Go test that plants its own `.go` file** under
  `ui/node_modules/<tmp>/` with `t.Cleanup`, then runs `go list`. Proves the
  property everywhere with no Node; writes into the working tree from a test,
  which the repository's tests otherwise never do, and a killed test leaves
  the file behind.
- c. **The file-existence test only.** Simplest; proves the mechanism is
  present rather than that it works.
- d. Other.

### 4. Does trufflehog take docz-site's stricter setting?

docz-site ran `--results=verified,unknown`, and ours runs `--results=verified`.
DESIGN-0017 §6 left the choice here. `unknown` reports credential-shaped
strings trufflehog could not verify either way, so it catches more and is
noisier. docz-api's test fixtures (fake PEMs, HMAC secrets) are the likely
noise.

> **Resolved 2026-09-23: (a).** `--results=verified,unknown` repository-wide,
> landed in Phase 2 after a local scan of the full merged history. Every
> finding is fixed or excluded with a written reason in the same PR.

- a. **Adopt `verified,unknown` in Phase 2**, after running it locally over
  the full merged history first. Any finding is fixed or excluded with a
  written reason in the same PR. The site's CLAUDE.md already treats an
  unverified finding as a failure, so the repository keeps the stricter of
  its two rules instead of silently dropping one. *(recommendation)*
- b. **Keep `verified`.** No new noise; the frontend loses a gate it had.
- c. **Split by path**: `verified,unknown` over `ui/`, `verified` elsewhere.
  Keeps each half's rule; two trufflehog configurations for one repository.
- d. Other.

### 5. How does the docz-site sweep avoid cutting a docz-site release?

docz-site releases through `pr-semver-bump` on every merge to `main`. Its
`publish-ghcr` chart job also runs on `dont-release`, deliberately, so that
chart-only changes can ship.

> **Resolved 2026-09-23: (a).** A docz-site PR labelled `dont-release`. Phase 1
> confirms from the run that no tag or image was cut and that the chart job
> skipped on `helm pull`.

- a. **A PR labelled `dont-release`**, which `pr-semver-bump` lists in its
  `noop-labels`, so no tag and no image. The chart job still runs and
  skips on `helm pull`, since `Chart.yaml` is unchanged at `0.1.10`. Phase 1
  confirms both from the run. *(recommendation)*
- b. **Push directly to docz-site `main`.** One commit and no PR; bypasses
  the repository's own rule and still triggers `release.yml` on push.
- c. **Skip the sweep** and fix the four statuses inside
  `docs/archive/ui/` after the graft. Edits the archive, and contradicts
  ADR-0004 OQ 1 ("in the home repositories, before the merge").
- d. Other.

### 6. What happens to docz-site's "bump the chart in the same PR" rule?

docz-site's CLAUDE.md requires a `Chart.yaml` `version` bump in any PR that
touches `charts/`, because there the chart published on every merge to
`main` and an unbumped change silently never shipped. Here charts publish
only from a beta tag, and `ct.yaml` has `check-version-increment: false`.
Phase 3 touches the chart (URLs) and Phase 4 bumps it.

> **Resolved 2026-09-23: (a).** One bump per release: `charts/docz-site` goes to
> `0.2.0` once, in Phase 4, and `ui/CLAUDE.md`'s rule is rewritten to "bump
> before the tag that should publish it", matching `charts/docz-api`.

- a. **One bump per release, not per PR.** `charts/docz-site` goes to
  `0.2.0` once, in Phase 4, and `ui/CLAUDE.md`'s rule is rewritten to say
  so: bump before the tag that should publish it. That matches how
  `charts/docz-api` has worked since IMPL-0019. *(recommendation)*
- b. **Keep the per-PR rule**: bump to `0.1.11` in Phase 3 and `0.2.0` in
  Phase 4. Honours the old rule literally; produces a version that is never
  published.
- c. **Turn on `check-version-increment`** in `ct.yaml` for both charts.
  Enforced rather than documented; fails every PR that touches a chart
  between releases, which is most of them.
- d. Other.

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0017](../design/0017-move-docz-site-in-ui-chartsdocz-site-and-orval-on-the-one-spec.md)
  — the design this implements, and all nine of its resolved questions
- [ADR-0004](../adr/0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md)
  — Decisions 2, 4, 7, 8; Open Questions 1, 2, 4
- [IMPL-0019](0019-docz-api-move-in-v200-beta3.md) — the phase shape and its
  Phase 2 notes, which this graft's invocation follows
- [INV-0012](../investigation/0012-docz-site-consumes-the-openapi-contract-from-the-same-repository.md)
  — concluded in Phase 4
- [INV-0013](../investigation/0013-docz-site-deferred-features-after-the-move-link-graph-lifecycle.md)
  — out of scope
- [#127](https://github.com/donaldgifford/docz/issues/127) — markdownlint on
  generated output, independent of this move
- [docz-site](https://github.com/donaldgifford/docz-site) — the source
  repository; PR #33 closed in Phase 1
- [git filter-repo](https://github.com/newren/git-filter-repo)

<!--docz:references:end-->
