---
id: IMPL-0005
title: "Changelog endpoint: pin docz v1.1.0, config-driven fetch, raw serve"
status: Completed
author: Donald Gifford
created: 2026-08-03
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0005: Changelog endpoint: pin docz v1.1.0, config-driven fetch, raw serve

**Status:** Completed **Author:** Donald Gifford **Date:** 2026-08-03

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: docz pin bump + contract clause R6](#phase-1-docz-pin-bump--contract-clause-r6)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Persistence — the resolved changelog path on the repo row](#phase-2-persistence--the-resolved-changelog-path-on-the-repo-row)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Fetch, ingest, and the webhook trigger](#phase-3-fetch-ingest-and-the-webhook-trigger)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Endpoint and contract — handler, spec 1.2.0](#phase-4-endpoint-and-contract--handler-spec-120)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: End-to-end proof and close-out](#phase-5-end-to-end-proof-and-close-out)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Objective

Implement INV-0005's feature 1: the repo changelog becomes a config-driven,
served artifact. Bump the docz pin to v1.1.0 (which ships `ChangelogConfig` +
`ParseChangelog` as docz DESIGN-0010, verified in INV-0005 F6), honor
`changelog.enabled`/`changelog.file` at fetch time, and serve the cached raw
markdown at `GET /api/v1/repos/{owner}/{name}/changelog` (spec 1.1.0 → 1.2.0).
Every INV-0005 open question is answered `a`; this plan encodes those answers
and the `index.md` precedent (DESIGN-0003 / IMPL-0003) it deliberately mirrors.

**Implements:** INV-0005 (feature 1); consumes docz DESIGN-0010 (v1.1.0)

## Scope

### In Scope

- `go.mod` pin bump `docz v1.0.0 → v1.1.0` (INV-0001 procedure) and the new
  **doczcontract clause R6**: `ChangelogConfig` defaults/merge/normalize +
  `ParseChangelog` behavior + `ErrNoVersions`, frozen against fixtures.
- Config-driven fetch in `githubapp` (hint parse + targeted blob, replacing the
  hardcoded root `CHANGELOG.md`), with INV-0005 OQ-3a's **strict semantics**:
  disabled or absent block ⇒ nothing fetched ⇒ the desired-state reconcile
  **nulls** the cached pair ⇒ the endpoint 404s.
- The resolved changelog path persisted on the repo row (per OQ-1a below) so the
  webhook can trigger a re-ingest when a push touches only the changelog file —
  without it the served copy goes stale (the file lives outside `docs_dir`,
  unlike `index.md`).
- The read endpoint + DTO, OpenAPI 1.2.0, contract-test coverage, e2e proof, and
  the docs/rollout close-out.

### Out of Scope

- **Feature 2** (per-document changelog backlinks) — its own INV/DESIGN; blocked
  on commit→file data (INV-0005 F5, compare-API vs PR-joins).
- **Structured/parsed serving** (INV-0005 OQ-1a chose raw-only).
  `ParseChangelog` is contract-pinned in R6 but has **no runtime caller** here
  yet — that is deliberate (OQ-7a), so feature 2 starts on a frozen surface.
- Meilisearch indexing of changelog bodies (OQ-5a: no).
- docz-site rendering (consumes the endpoint; separate repo).
- Narrow blob fetches / fetch-cost optimization (unchanged deferral).

## Implementation Phases

Each phase ends green (`just test` / `just lint` / `just fmt`) and is a
conventional commit on this branch (`inv/0005-changelog-doc-type`). Order is by
dependency, mirroring IMPL-0003: the pin + contract first (everything compiles
against v1.1.0), the column before the mapping that writes it, the fetch before
the endpoint that serves its output. One vertical-slice PR (per OQ-4a below), so
no phase leaves a specced-but-unserved window on `main`.

---

### Phase 1: docz pin bump + contract clause R6

The module moves to docz v1.1.0 and the contract package freezes the new
surface. No runtime behavior changes — `internal/doczcontract` is
runtime-code-free by design.

#### Tasks

- [x] Bump the pin: `go get github.com/donaldgifford/docz@v1.1.0` (**never** a
      bare `go mod tidy` — staged-dep rule), `go mod edit -fmt`; confirm no
      import-path changes (v1.1.0 keeps `pkg/doczcore/config` +
      `pkg/doczcore/document`, verified INV-0005 F6).
- [x] Freeze a **static** fleet-shaped changelog fixture at
      `internal/doczcontract/testdata/` (snapshot of a real git-cliff output:
      preamble + `[unreleased]` + ≥2 released versions with `*(scope)*` bullets
      and PR links). Static copy, not the live `CHANGELOG.md` — contract
      fixtures must not drift.
- [x] Add R6 config tests (`contract_test.go`, temp-dir + `HOME`-override
      pattern already used by the R1–R5 tests):
      `DefaultConfig().Changelog == {false, "CHANGELOG.md"}`; partial block
      (`enabled: true` only) keeps the `File` default; explicit empty `file:`
      backfills; `./` prefix normalized; unknown **sibling** key still tolerated
      (the INV-0005 F2 rollout guarantee).
- [x] Add R6 parser tests: the frozen fixture parses to the exact
      versions/dates/groups/items (bare-semver identity, `unreleased`
      canonicalized, scope markers + PR links intact in items);
      `errors.Is(err, doczdoc.ErrNoVersions)` on a no-headings input; empty
      input errors the same way.
- [x] Update `internal/doczcontract/doc.go` (R1–R5 → R1–R6) and the CLAUDE.md
      docz-pin note (`v1.0.0` → `v1.1.0`).
- [x] `just test` / `just lint` / `just fmt` green; commit
      (`feat(doczcontract): pin docz v1.1.0 + freeze changelog surface (R6)`).

#### Success Criteria

- `go.mod` requires docz `v1.1.0` with no `replace`; build + full unit suite
  green with **zero changes outside** `go.mod`/`go.sum`, `doczcontract`, and
  docs.
- R6 fails loudly if a future docz bump changes any pinned behavior (defaults,
  merge, normalization, parse shape, sentinel identity).

**Status: COMPLETE ✅** — pin at v1.1.0 (plain require, import paths unchanged);
`changelog_test.go` adds R6 over the frozen `testdata/changelog_fleet.md`
fixture (defaults, partial-merge, `./` normalize, unknown-key tolerance,
enabled-only validation, parse shape with v-trim + `ErrNoVersions`); `just test`
/ `just lint` (0 issues) / `just fmt` green; change set confined to
`go.mod`/`go.sum`, `doczcontract`, CLAUDE.md.

---

### Phase 2: Persistence — the resolved changelog path on the repo row

The repo row learns `changelog_file` — the resolved repo-root-relative path when
the block is enabled, NULL when disabled (per OQ-1a; the `DocsDir` column +
IMPL-0003's cached-pair migrations are the precedent). The
`changelog_md`/`changelog_sha` columns already exist (initial schema); nothing
else is stored.

#### Tasks

- [x] Goose migration
      `internal/store/migrations/20260803000000_add_repo_changelog_file.sql`:
      `ALTER TABLE repos ADD COLUMN changelog_file TEXT` (+ mirrored
      `-- +goose Down` drop), comment matching the cached-pair style. Covered by
      the existing `TestMigrateUpDownRoundTrip` (it walks all migrations up →
      down-to-zero → up).
- [x] Add the column to `UpsertRepo` (`internal/store/queries/repos.sql`, INSERT
      list + `DO UPDATE SET`); `just generate`; `just generate-check` clean.
      Reads pick it up via `SELECT *` — no new queries.
- [x] Grow `store.RepoInput` with `ChangelogFile string`, mapped in
      `reconcile.go` with `textOrNull` beside the cached pair. Document the
      OQ-3a consequence on `RepoInput`: a disabled block ⇒ empty
      `ChangelogFile`/`ChangelogMD`/`ChangelogSHA` ⇒ all three columns **null on
      the next reconcile** (pure desired state; presence keys off
      `changelog_sha`, mirroring the `index_sha` gotcha).
- [x] Store integration tests (`//go:build integration`): reconcile with the
      triple persists all three; a follow-up reconcile without them nulls all
      three; empty-body-with-valid-sha keeps the sha (empty changelog file ⇒
      200 + `""` later).
- [x] `just test` / `just lint` / `just fmt` green; commit
      (`feat(store): persist the resolved changelog path on the repo row`).

#### Success Criteria

- Migration up/down round-trips via `TestMigrateUpDownRoundTrip`;
  `generate-check` reports no drift.
- `ReconcileRepo` round-trips set → clear for the changelog triple under
  integration tests.
- No fetch/serve behavior change yet; all existing tests green untouched.

**Status: COMPLETE ✅** — `20260803000000_add_repo_changelog_file.sql` adds
`repos.changelog_file` (+ down); `UpsertRepo` carries it as `$10` with an
`EXCLUDED` update and `generate-check` is clean; `RepoInput.ChangelogFile` maps
through `textOrNull` with the opt-in-desired-state rule documented on the
struct. `TestReconcileRepoChangelogTriple` proves set (subpath) →
empty-body-with-valid-sha → all-three-cleared against real Postgres, with
`TestMigrateUpDownRoundTrip` + `TestReconcileRepoIndexPair` still green.
`just test` / `just lint` (0 issues) green.

---

### Phase 3: Fetch, ingest, and the webhook trigger

`githubapp.Fetch` honors the config; `ingest.Service` maps the resolved path;
the webhook re-ingests on changelog-only pushes. This is the phase where today's
unconditional root-`CHANGELOG.md` fetch becomes opt-in (OQ-3a):
already-onboarded repos without the block stop caching the changelog on their
next reconcile — invisible today since nothing serves it yet.

#### Tasks

- [x] `internal/githubapp`: add
      `changelogHint(configYAML) (enabled bool,     file string)` — fetch-scoped
      one-field `yaml.Unmarshal` of the `changelog:` block, defaults from
      `doczcfg.DefaultConfig().Changelog` / `doczcfg.DefaultChangelogFile`
      (mirrors `docsDirHint`: hint at fetch, authoritative parse stays in
      ingest's `loadConfig`).
- [x] Rework `Client.Fetch`: drop the hardcoded changelog branch from
      `classifyTree` (signature loses `changelogSHA`); after `ConfigYAML` is
      fetched, when the hint says enabled → `findBlobSHA(tree, file)` → fetch
      that blob into `snap.ChangelogMD`/`ChangelogSHA` (at most one extra blob
      request, zero when disabled/absent — the `index.md` pattern exactly).
      Subpaths (`charts/<name>/CHANGELOG.md`) work by construction since
      `findBlobSHA` is exact-path.
- [x] `internal/ingest/service.go`: map
      `RepoInput.ChangelogFile = cfg.Changelog.File` when
      `cfg.Changelog.Enabled`, else `""` — from the **authoritative**
      post-`Load` config (normalized), not the hint. Per OQ-2a, an invalid
      `changelog.file` fails `Validate` and therefore the whole ingest,
      identical to any other malformed `.docz.yaml`.
- [x] `internal/webhook`: extend `shouldIngest(ev, docsDir, changelogFile)` —
      also match a changed path equal to the repo row's resolved
      `changelog_file` (when non-NULL). `handlePush` already holds the full repo
      row (`SELECT *`), so this is one param + one comparison; a `.docz.yaml`
      push already re-ingests, keeping the stored path fresh.
- [x] Tests: `githubapp` stub-RoundTripper fixtures — enabled root file, enabled
      subpath (`charts/x/CHANGELOG.md`), disabled block (no blob request —
      assert request count), absent block (default-off), enabled but file
      missing from tree (empty snapshot fields, no error); `changelogHint` table
      (absent/partial/full/malformed yaml → default-off); ingest unit test
      mapping enabled/disabled → `ChangelogFile` set/empty; webhook
      `shouldIngest` table gains changelog-path hit/miss/NULL cases.
- [x] `just test` / `just lint` / `just fmt` green; commit
      (`feat(ingest): config-driven changelog fetch + webhook trigger`).

#### Success Criteria

- With `changelog.enabled: true`, `Fetch` retrieves exactly the configured file
  (root or subpath); disabled/absent fetches nothing and the next reconcile
  nulls the cached triple (proven by unit + Phase 2 integration tests together).
- A push touching **only** the changelog file on the default branch enqueues a
  re-ingest; one touching neither it, `.docz.yaml`, nor `docs_dir/` does not.
- The five-endpoint read surface is still untouched (endpoint lands next phase).

**Status: COMPLETE ✅** — `changelogHint` mirrors `docsDirHint` (fetch-scoped,
docz defaults, `./` normalization); `Fetch` resolves the configured path via
`findBlobSHA` and `classifyTree` no longer recognizes `CHANGELOG.md` at all;
`ingest.changelogFile` maps the authoritative post-`Load` value (empty when
dormant); `shouldIngest` gained the exact-path changelog match.
`TestFetchRepoChangelog` covers enabled-root / enabled-subpath / disabled /
absent-block / configured-file-missing (the no-fetch cases prove it by
withholding the blob — the stub 404s on an unfetched sha), `TestChangelogHint`

- `TestRunMapsChangelogFile` table the parse and mapping, and `TestShouldIngest`
  gained five changelog cases. All 14 packages green; `just lint` 0 issues.

---

### Phase 4: Endpoint and contract — handler, spec 1.2.0

The serve slice, a near-verbatim mirror of `getRepoIndex`.

#### Tasks

- [x] `internal/httpapi/dto.go`: `repoChangelogDTO` — `repo` (`owner/name`
      label), `changelog_md`, `changelog_sha` (`nullText` mapping, empty strings
      never `null`) + `toRepoChangelog`.
- [x] `internal/httpapi/handlers.go`: `getRepoChangelog` — `resolveRepo`
      (existence hiding) → gate on `repo.ChangelogSha.Valid` → 404
      `{"error":"changelog not found"}` or 200 DTO. Route
      `r.Get("/changelog", …)` beside `/index` in `Mount`.
- [x] httpapi unit tests: 200 envelope (body + sha), empty-file 200 + `""` body
      (valid sha, NULL body), 404 when sha NULL, 404 existence-hiding for a repo
      outside the allowed set.
- [x] Spec (`api/openapi.yaml`): path `/api/v1/repos/{owner}/{name}/changelog`,
      `operationId:     getRepoChangelog`, tag `repos`, `RepoChangelog` schema
      (`additionalProperties: false`, all three fields required), 404 →
      `ErrorResponse`; **`info.version: 1.1.0 → 1.2.0`** (additive minor,
      INV-0005 OQ-6a). `just lint-openapi` (vacuum 100/100 + yamlfmt) clean.
- [x] Contract test (`openapi_contract_test.go`): seed the primary fixture repo
      with a changelog pair; round-trip the 200; the bare fixture repo
      (`acme/bare`) proves the 404 — the exact `getRepoIndex` pattern. Update
      `api/README.md` (consumer guide: new op + version note).
- [x] `just test` / `just lint` / `just fmt` / `just lint-openapi` green; commit
      (`feat(httpapi): serve the repo changelog (spec 1.2.0)`).

#### Success Criteria

- The contract test validates the new op against the served spec bytes in both
  directions (request + response), happy and 404, with
  `additionalProperties: false` as the drift gate.
- Spec scores 100/100 under vacuum; `info.version` is `1.2.0`.
- All prior wire shapes byte-identical (existing contract cases untouched).

**Status: COMPLETE ✅** — `repoChangelogDTO` + `toRepoChangelog` mirror the
index pair; `getRepoChangelog` gates on `ChangelogSha.Valid` behind
`resolveRepo` (existence hiding) and is routed at `/changelog`. Spec is `1.2.0`
with the `getRepoChangelog` op + strict `RepoChangelog` schema, vacuum
**100/100**. `TestGetRepoChangelog` covers 200 / empty-200 / 404 / unknown-repo,
`TestGetRepoChangelogUnauthorizedIs404` the hidden 404, and the contract test
round-trips `getRepoChangelog` + `getRepoChangelogMissing` against the served
bytes. `api/README.md` records the 1.2.0 signal. All 14 packages green;
`just lint` 0 issues.

---

### Phase 5: End-to-end proof and close-out

#### Tasks

- [x] e2e integration test (`internal/e2e`, real Postgres):
      `TestE2ERepoChangelogServeAndDisable` mirroring
      `TestE2ERepoIndexServeAndRemoval` — onboard with the fake fetcher carrying
      a changelog (enabled) → `GET .../changelog` 200 with body + sha →
      re-onboard with the block disabled at HEAD → triple nulled → 404.
- [x] Rollout note (`deploy/README.md`, beside the index.md note): natural
      refresh only — repos serve their changelog after their **next ingest with
      `changelog.enabled: true`**; pre-existing cached copies from the
      pre-config era are **nulled** on the next reconcile of repos that never
      enable the block (OQ-3a; nothing served them, so nothing user-visible
      regresses).
- [x] Update **CLAUDE.md**: pin note v1.1.0 + R6; the new IMPL-0005 section
      (conventions: hint parse, strict-null semantics, `changelog_file` column,
      webhook trigger, spec 1.2.0).
- [x] Dogfood check (per OQ-3a of this doc): this repo's `.docz.yaml` already
      carries `changelog: {enabled: true, file: CHANGELOG.md}` — leave it; once
      deployed and re-ingested, docz-api serves its own changelog.
- [x] `docz update` (index tables); check this plan's boxes as phases land; flip
      IMPL-0005 → In Progress → Completed at the appropriate commits.
- [x] Final gates: `just test`, `just test-integration` (Docker), `just lint`,
      `just lint-openapi`, `just fmt`, changelog sync commit; hand the branch to
      review/PR.

#### Success Criteria

- The e2e test proves ingest → serve → disable-at-HEAD → 404 against a real
  Postgres.
- Local gates all green (`just test` / `test-integration` / `lint` /
  `lint-openapi` / `fmt`); the branch is PR-ready with the INV + IMPL +
  implementation as one reviewable unit.
- Docs (CLAUDE.md, deploy, api/README, docz indexes) reflect the shipped
  behavior.

**Status: COMPLETE ✅** — `TestE2ERepoChangelogServeAndDisable` proves ingest →
serve → disable-at-HEAD → 404 against real Postgres. Full local gates green:
`just test` (14 pkgs), `just test-integration` (16 pkgs, 0 FAIL), `just lint` (0
issues), `just lint-openapi` (vacuum 100/100), `just fmt`. Rollout +
webhook-trigger notes in `deploy/README.md`, conventions in CLAUDE.md, the 1.2.0
signal in `api/README.md`, docz indexes refreshed. This repo keeps
`changelog: enabled: true` (dogfood, OQ-3a).

**IMPL-0005 is COMPLETE** — all five phases done; the branch is one reviewable
vertical slice (OQ-4a).

## File Changes

| File                                                                   | Action | Description                                                     |
| ---------------------------------------------------------------------- | ------ | --------------------------------------------------------------- |
| `go.mod` / `go.sum`                                                    | Modify | docz `v1.0.0 → v1.1.0` (targeted `go get`)                      |
| `internal/doczcontract/` + `testdata/`                                 | Modify | R6 clause + frozen fleet fixture                                |
| `internal/store/migrations/20260803000000_add_repo_changelog_file.sql` | Create | `repos.changelog_file TEXT` (+ down)                            |
| `internal/store/queries/repos.sql` + generated                         | Modify | `UpsertRepo` gains the column                                   |
| `internal/store/reconcile.go`                                          | Modify | `RepoInput.ChangelogFile` + `textOrNull` mapping                |
| `internal/githubapp/client.go` (+ tests)                               | Modify | `changelogHint`, targeted fetch, `classifyTree` sheds changelog |
| `internal/ingest/service.go` (+ tests)                                 | Modify | map `ChangelogFile` from authoritative config                   |
| `internal/webhook/events.go` (+ tests)                                 | Modify | `shouldIngest` matches the stored changelog path                |
| `internal/httpapi/{dto,handlers,handler}.go`                           | Modify | DTO, `getRepoChangelog`, route                                  |
| `api/openapi.yaml` + `api/README.md`                                   | Modify | new op, `RepoChangelog` schema, version 1.2.0                   |
| `internal/httpapi/openapi_contract_test.go`                            | Modify | happy + 404 contract cases                                      |
| `internal/e2e/`                                                        | Modify | `TestE2ERepoChangelogServeAndDisable`                           |
| `CLAUDE.md`, `deploy/README.md`, docz indexes                          | Modify | conventions + rollout notes                                     |

## Testing Plan

- **Contract (R6):** docz surface frozen against static fixtures — the tripwire
  for every future docz bump.
- **Unit:** hint-parse table; fetch fixtures (enabled / disabled / subpath /
  missing file, request-count assertion); ingest mapping; webhook trigger table;
  handler status matrix (200 / empty-200 / 404 / hidden-404).
- **Integration (testcontainers):** store round-trip incl. null-on-disable;
  migration up/down via the existing round-trip test.
- **Wire contract:** kin-openapi round-trips of the new op, strict schemas.
- **e2e:** full pipeline ingest → serve → disable → 404 on real Postgres.

## Dependencies

- docz **v1.1.0** — released 2026-08-03, surface verified (INV-0005 F6).
- No new Go dependencies otherwise; no chart/deploy changes (the endpoint rides
  the existing service and auth seam).

## Open Questions

Numbered for review; `a` is the recommendation. Reply with a letter per
question, or "other: …".

**All answered `a` (2026-08-03).** OQ-1a comes with a design note from review:
the persisted-path-on-the-repo-row shape is expected to become a **more generic
consumed-files pattern later** (other non-doc files served by the API). Keep the
implementation simple but not corner-painting: one plain column now, no bespoke
abstractions that would fight a future `tracked files` generalization — and no
premature generalization either.

1. **Webhook staleness — how does a changelog-only push trigger re-ingest?** The
   changelog lives outside `docs_dir` (unlike `index.md`), so today's
   `shouldIngest` would never refresh it — and the release flow's changelog-sync
   push is exactly this shape, so the served copy would otherwise lag one
   release behind, permanently.
   - **a. Persist the resolved path as `repos.changelog_file`** (NULL when
     disabled) and match it in `shouldIngest` — the `DocsDir`-on-the-row
     precedent; exact per-repo correctness, one tiny migration, zero parse work
     in the webhook. _(Recommended; Phases 2–3 assume it.)_ **Answered: (a)** —
     with the generic-pattern design note above.
   - b. Hint-parse the cached `config_snapshot` inside the webhook per push — no
     migration, but duplicates the hint helper outside `githubapp` and re-parses
     yaml on every push event.
   - c. No trigger — accept staleness until the next `docs_dir` / `.docz.yaml`
     push. Cheapest, but the "changelog lags its own release" failure mode is
     permanent.

2. **Invalid `changelog.file` (enabled) — fail the whole ingest?** docz v1.1.0's
   `Validate` rejects bad paths only when the block is enabled.
   - **a. Yes — let the existing `Validate` gate fail the ingest loudly**,
     identical to any other malformed `.docz.yaml`; the repo owner fixes the
     config and the next push heals it. _(Recommended: one error path, no
     partial-ingest special case.)_
   - b. Catch changelog validation specifically: warn, ingest docs without a
     changelog. Softer, but adds a bespoke partial-failure mode for a config the
     owner explicitly opted into. **Answered: (a).**

3. **Dogfood — keep `changelog.enabled: true` in this repo's `.docz.yaml`?** It
   is already committed on this branch (the INV's live example).
   - **a. Keep it** — docz-api serves its own changelog after deploy +
     re-ingest; the fleet's first consumer is ourselves. _(Recommended.)_
   - b. Revert to disabled until the docz-site renders it, then enable
     fleet-wide in one pass. **Answered: (a).**

4. **PR shape — one vertical slice or split?**
   - **a. One PR off this branch** (INV + IMPL + all five phases): no
     specced-but-unserved window on `main`, matches IMPL-0003's
     one-vertical-slice precedent; the pin bump is exercised by the feature that
     needs it in the same review. _(Recommended.)_
   - b. Two PRs — pin bump + R6 first, feature second. Smaller reviews, but
     `main` briefly pins v1.1.0 with a dormant surface and the second PR carries
     a cross-PR dependency. **Answered: (a).**

## References

- INV-0005 — Changelog as a first-class docz artifact (all OQs = `a`; F6
  verifies v1.1.0)
- docz v1.1.0 / upstream DESIGN-0010 — `ChangelogConfig` + `ParseChangelog`
  (<https://github.com/donaldgifford/docz/releases/tag/v1.1.0>)
- DESIGN-0003 / IMPL-0003 — the `index.md` repo-level-artifact precedent
  mirrored throughout
- INV-0001 — pin-bump procedure (targeted `go get`, contract re-run)
- `internal/doczcontract` — R1–R5 today; gains R6 in Phase 1
