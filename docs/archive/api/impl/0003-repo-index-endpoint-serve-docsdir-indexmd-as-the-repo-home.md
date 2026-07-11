---
id: IMPL-0003
title: "Repo index endpoint: serve docs_dir index.md as the repo home"
status: Completed
author: Donald Gifford
created: 2026-07-10
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0003: Repo index endpoint: serve docs_dir index.md as the repo home

**Status:** Completed
**Author:** Donald Gifford
**Date:** 2026-07-10

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Persistence — columns, upsert, reconcile](#phase-1-persistence--columns-upsert-reconcile)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Fetch and ingest — snapshot fields and the targeted blob](#phase-2-fetch-and-ingest--snapshot-fields-and-the-targeted-blob)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Endpoint and contract — handler, spec 1.1.0, contract test](#phase-3-endpoint-and-contract--handler-spec-110-contract-test)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: End-to-end proof and close-out](#phase-4-end-to-end-proof-and-close-out)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. Which YAML library for the docs_dir hint parse?](#1-which-yaml-library-for-the-docsdir-hint-parse)
  - [2. How does the contract test model the no-index 404?](#2-how-does-the-contract-test-model-the-no-index-404)
  - [3. Advertise index presence on existing repo DTOs?](#3-advertise-index-presence-on-existing-repo-dtos)
  - [4. Cap the cached index.md size?](#4-cap-the-cached-indexmd-size)
  - [5. One PR or one PR per phase?](#5-one-pr-or-one-pr-per-phase)
- [References](#references)
<!--toc:end-->

## Objective

Implement **DESIGN-0003** (all six design OQs resolved as option `a`): fetch
the repo's **`docs_dir/index.md`** — the file docz's wiki renders as its
landing page — as one extra targeted blob during ingest, cache it on two new
nullable `repos` columns (the `CHANGELOG.md` precedent), and serve it at
**`GET /api/v1/repos/{owner}/{name}/index`** as a JSON envelope
`{repo, index_md, index_sha}` (404 with the standard error envelope when the
repo has no `index.md`). The OpenAPI contract gains the path + `RepoIndex`
schema and bumps **`1.0.0 → 1.1.0`**. Everything is additive; no existing wire
shape changes (the contract test enforces it).

**Implements:** DESIGN-0003 (from INV-0003 finding F1 — the docz-site repo
home, the top-priority deferred-feature).

## Scope

### In Scope

- Goose migration adding `repos.index_md` / `repos.index_sha` (nullable TEXT),
  `UpsertRepo` + sqlc regeneration, `store.RepoInput` mapping.
- `ingest.RepoSnapshot` + `IndexMD []byte` / `IndexSHA string`;
  `githubapp.Client.Fetch` docs_dir hint parse (DESIGN OQ-2a) + targeted
  `docs_dir/index.md` blob fetch; the ingest `RepoInput` mapping.
- `internal/httpapi`: `getRepoIndex` handler + `repoIndexDTO`, mounted in the
  gated `/api/v1` group behind `resolveRepo` (existence hiding).
- `api/openapi.yaml`: new path + `RepoIndex` schema, `info.version: 1.1.0`;
  contract-test coverage for the happy path and the 404 envelope.
- Unit / integration / e2e tests per the DESIGN-0003 testing strategy;
  rollout note (natural-refresh backfill, DESIGN OQ-4a).

### Out of Scope

- **Changelog endpoint** (versions feature, DESIGN-0001 OQ-12) — this
  endpoint's shape is copyable for it later.
- **HTML rendering, content negotiation, ETag plumbing** — the site renders;
  `index_sha` is the cache key (DESIGN OQ-1a/6a).
- **Backfill machinery** — natural refresh only (DESIGN OQ-4a); a manual
  `-onboard` per repo covers the gap.
- **Link graph, labels, raw-file ingest** — separate INV-0003 items.
- **docz-site changes** — the site swaps its fallback at spec `1.1.0`,
  in its own repo.

## Implementation Phases

Each phase ends green (`just test` / `just lint` / `just fmt`) and is a
conventional commit. Phases 1→3 are ordered by dependency (columns before the
mapping that writes them; snapshot fields before the fetch that fills them;
both before the endpoint that serves them). The whole plan is one vertical
slice shipped on one PR (OQ-5a), so no phase leaves a specced-but-unserved
window on `main`.

---

### Phase 1: Persistence — columns, upsert, reconcile

The store learns to hold the cached index pair. After this phase a reconcile
whose input carries index content persists it; nothing fetches or serves it
yet.

#### Tasks

- [x] Add the second goose migration
      (`internal/store/migrations/20260710000000_add_repo_index.sql`):
      `ALTER TABLE repos ADD COLUMN index_md TEXT, ADD COLUMN index_sha TEXT`
      (+ mirrored `-- +goose Down` drops), comments matching the
      `changelog_md`/`changelog_sha` style. Verify up **and** down against the
      compose Postgres (`-migrate`, then a manual `MigrateDown` check as
      Phase 1 of IMPL-0001 did). _(done — migration added; up/down verified by
      the new **`TestMigrateUpDownRoundTrip`** testcontainers test (own
      container, up → down-to-zero → up) instead of a manual compose check —
      hermetic and permanent regression coverage, strictly better than the
      one-off verification.)_
- [x] Add the two columns to `UpsertRepo` (`internal/store/queries/repos.sql`)
      — INSERT list + `DO UPDATE SET` — and run `just generate`;
      `just generate-check` clean. `store.Repo` picks the fields up via
      `SELECT *` (no new read queries). _(done — `$10`/`$11` params +
      `EXCLUDED` updates; `Repo`/`UpsertRepoParams` regenerated with
      `IndexMd`/`IndexSha`; `generate-check` clean.)_
- [x] Grow `store.RepoInput` with `IndexMD` / `IndexSHA string` and map them
      in `reconcile.go` with `textOrNull`, beside the changelog pair.
      **Gotcha (by design):** `textOrNull("")` → NULL, so an empty-but-present
      `index.md` stores `index_md = NULL` with a **valid `index_sha`** —
      presence therefore keys off `index_sha`, and the DTO's `nullText`
      mapping returns `""` for the body, which is exactly DESIGN OQ-3a's
      "empty file ⇒ 200 + empty string". _(done — fields added + mapped; the
      presence-keys-off-`index_sha` gotcha is documented on `RepoInput`
      itself.)_
- [x] Extend the store integration tests (`//go:build integration`,
      testcontainers): a reconcile with the index pair persists both columns;
      a follow-up reconcile without them (file deleted at HEAD) nulls both; an
      empty-body input with a sha keeps the sha valid. _(done —
      `TestReconcileRepoIndexPair` covers set → empty-body-with-valid-sha →
      cleared against a real Postgres.)_
- [x] `just test` / `just lint` / `just fmt` green; commit
      (`feat(store): cache docs_dir index.md on the repo row`). _(done —
      unit suite, full `just test-integration` (all packages ok, 0 FAIL),
      `just lint` 0 issues, `just fmt` no-op. Landed as four task commits:
      migration, upsert+sqlc, RepoInput mapping, integration tests.)_

#### Success Criteria

- Migration applies and rolls back cleanly; `just generate-check` reports no
  sqlc drift.
- `ReconcileRepo` round-trips the index pair (set → update → clear) under the
  integration tests, including the empty-body/valid-sha case.
- No API or ingest behavior change yet — all existing tests green untouched.

**Status: COMPLETE ✅** — all criteria met. `TestMigrateUpDownRoundTrip`
proves up → down-to-zero → up on a fresh container; `generate-check` is
clean; `TestReconcileRepoIndexPair` round-trips set → empty-body (NULL body,
valid sha) → cleared; the full unit + integration suites pass with zero
changes to any existing test.

---

### Phase 2: Fetch and ingest — snapshot fields and the targeted blob

The pipeline learns to bring the file home. After this phase an ingest run
populates the Phase 1 columns from a live (or stubbed) repo.

#### Tasks

- [x] Add `IndexMD []byte` / `IndexSHA string` to `ingest.RepoSnapshot`
      (`internal/ingest/fetcher.go`), doc comments mirroring the changelog
      pair (nil/"" when absent). _(done — fields + DESIGN-0003 doc comments.)_
- [x] Implement the docs_dir **hint parse** in `internal/githubapp`
      (DESIGN OQ-2a): unexported `docsDirHint(configYAML []byte) string` —
      one-field `yaml.Unmarshal` of `docs_dir`, defaulting to
      `doczcfg.DefaultConfig().DocsDir` on empty/unparseable input (a broken
      `.docz.yaml` still fails ingest at `loadConfig`; the hint never masks
      it). Promote `gopkg.in/yaml.v3` to a direct require per the repo
      convention (go.mod edit / targeted `go get`, **no bare `go mod tidy`**)
      — OQ-1a. _(done — hint parse trims a trailing `/`; yaml.v3 moved to the
      direct require block + `go mod edit -fmt`.)_
- [x] Wire the targeted fetch into `Client.Fetch`: after `ConfigYAML` is
      fetched, look up `docsDirHint(...) + "/" + doczcfg.WikiIndexName` in the
      already-fetched tree entries (exact path match, blob type); when
      present, `fetchBlob` → `snap.IndexMD` / `snap.IndexSHA`. Absent ⇒ both
      zero, **zero extra API calls**. _(done — `findBlobSHA` exact-path/blob
      match over the recursive tree; fetch wired after the changelog blob.)_
- [x] Map the pair in `ingest.Service` (`RepoInput{... IndexMD:
      string(snap.IndexMD), IndexSHA: snap.IndexSHA}`) beside the changelog
      mapping. _(done — plus the `Run` doc comment now names the cached repo
      home.)_
- [x] Tests: `githubapp` stub-RoundTripper fixtures — tree with
      `docs/index.md` (fetched, correct sha), tree without it (zero fields, no
      extra blob request), non-default `docs_dir: notes` targeting
      `notes/index.md`, config without `docs_dir` using the default;
      `docsDirHint` table test (valid, empty, missing key, malformed YAML);
      ingest `service_test` asserts the `RepoInput` carries the pair. _(done —
      `TestFetchRepoIndex` (default dir, custom `notes` beating a decoy
      `docs/index.md`, index.md-as-directory non-match), the absent case
      asserted in `TestFetchClassifiesAndDecodes` (stub 404s unknown shas, so
      a stray blob call would fail the fetch), `TestDocsDirHint` five-case
      table, and the `RepoInput` pair assertion in `service_test`.)_
- [x] `just test` / `just lint` / `just fmt` green; commit
      (`feat(ingest): fetch docs_dir index.md into the repo snapshot`).
      _(done — lint 0 issues, fmt no-op, full unit suite green.)_

#### Success Criteria

- A stubbed fetch of a repo with `docs/index.md` lands the bytes + sha on the
  snapshot; without the file, both fields are zero and the blob endpoint is
  **not** called.
- The hint parse honors a custom `docs_dir`, defaults correctly, and cannot
  fail an otherwise-valid fetch (malformed YAML → default + ingest still
  fails later at `loadConfig`, unchanged).
- `gopkg.in/yaml.v3` is a direct require; `go mod verify` clean.

**Status: COMPLETE ✅** — all criteria met. `TestFetchRepoIndex` proves the
targeted fetch (default and custom `docs_dir`, blob-only match);
`TestFetchClassifiesAndDecodes` proves the absent case makes no extra blob
call (the stub 404s unknown shas); `TestDocsDirHint` pins the fallback
behavior; `go mod verify` reports all modules verified.

---

### Phase 3: Endpoint and contract — handler, spec 1.1.0, contract test

The cached pair goes on the wire. After this phase the endpoint is specced,
served, and contract-gated.

#### Tasks

- [x] Add `repoIndexDTO` (`internal/httpapi/dto.go`): `repo` / `index_md` /
      `index_sha` + a `toRepoIndex(*store.Repo)` mapper (`nullText` for both
      nullable columns). _(done — the mapper's doc comment records the
      presence-keys-off-sha contract.)_
- [x] Add the `getRepoIndex` handler + route (`r.Get("/index", …)` in the
      `/repos/{owner}/{name}` subtree): `resolveRepo` (unknown and
      unauthorized repos both 404, unchanged helper) → **absence check on
      `repo.IndexSha.Valid`** (per the Phase 1 gotcha) → 404
      `{"error": "index not found"}` when absent → 200 `repoIndexDTO`
      otherwise. _(done — mounted between `/` and `/types`.)_
- [x] httpapi unit tests: 200 envelope (body + sha), empty-file 200 + `""`,
      404 for a repo without an index, 404 unknown repo, 404 outside the
      allowed set. _(done — `TestGetRepoIndex` (four subtests, empty-file via
      an `acme/emptyidx` NULL-body/valid-sha row) +
      `TestGetRepoIndexUnauthorizedIs404`.)_
- [x] Spec (`api/openapi.yaml`): new path
      `/api/v1/repos/{owner}/{name}/index` (`get`, tag `repos`,
      `operationId: getRepoIndex`, shared `Owner`/`Name` params,
      `Unauthorized`/`NotFound` responses) + `RepoIndex` schema (all three
      fields `required`, `additionalProperties: false`, descriptions for
      vacuum). Bump **`info.version` → `1.1.0`**. `just lint-openapi` 100/100.
      _(done — the `NotFound` description also gained the index flavor.)_
- [x] Contract test: seed the fixture repo with an index body and add the
      `getRepoIndex` happy-path case; cover the no-index **404** envelope per
      OQ-2a (second bare fixture repo; update the `list repos` count
      assertions it shifts). _(done — `seededStore` grew the `acme/platform`
      index pair + the bare `acme/bare` repo; `getRepoIndex` +
      `getRepoIndexMissing` contract cases; the shifted assertions were the
      `list repos` count and `TestSearchEndpoint`'s allowed-set `[1]`→`[1 2]`.)_
- [x] `just test` / `just lint` / `just fmt` / `just lint-openapi` green;
      commit (`feat(httpapi): serve the repo index at
      /api/v1/repos/{owner}/{name}/index`). _(done — 14 packages ok, lint 0
      issues, vacuum 100/100.)_

#### Success Criteria

- The endpoint returns the DESIGN-0003 wire shapes exactly: 200
  `{repo, index_md, index_sha}`, 404 error envelope for absent index /
  unknown repo / unauthorized repo (indistinguishable, existence hiding).
- The OpenAPI contract test validates request **and** response for both new
  cases; the spec self-validates at `1.1.0`; vacuum stays 100/100.
- **No existing schema changed** — the contract test passes with zero edits
  to any previously specced response (the drift detector proves additivity).

**Status: COMPLETE ✅** — all criteria met. The five 200/404 flavors are
pinned by unit tests; the contract test validates request + response for
`getRepoIndex` and `getRepoIndexMissing` at spec `1.1.0` (vacuum 100/100);
no previously specced schema was edited — only the fixture seed and the
allowed-set count assertions shifted, exactly as OQ-2a predicted.

---

### Phase 4: End-to-end proof and close-out

Prove the whole slice through the real pipeline and record the work.

#### Tasks

- [x] Extend the e2e integration test (`internal/e2e`): the fake
      `RepoFetcher` snapshot gains an `index.md` body; after onboarding
      through the real ingest + store, `GET .../index` serves it; a
      re-onboard with the file removed flips the endpoint to 404 (the
      delete-at-HEAD path). _(done — `TestE2ERepoIndexServeAndRemoval`.)_
- [x] Rollout note (per DESIGN OQ-4a): document in `deploy/` notes or the PR
      body that repos onboarded before this ships serve 404 until their next
      push or a manual `-onboard owner/name@id`; the site's fallback covers
      the gap. _(done — `deploy/README.md` Notes section + repeated in the PR
      body.)_
- [x] Update **`CLAUDE.md`**: the persistence conventions gain the new
      columns/migration; the OpenAPI section records spec `1.1.0` and the new
      endpoint; add an IMPL-0003 phase-progress note. _(done — a dedicated
      "Repo index endpoint" section covers persistence/fetch/serve/rollout,
      and the OpenAPI section records the current `1.1.0`.)_
- [x] Flip **DESIGN-0003 → Implemented**; check off this plan; `docz update`
      for the index tables. _(done.)_
- [x] Final gates: `just test`, `just test-integration` (Docker),
      `just lint`, `just lint-openapi`, `just fmt`; changelog sync commit;
      push + PR (label **minor** — additive endpoint), merge when green.
      _(done — all gates green; PR merged.)_

#### Success Criteria

- The e2e test proves fetch → reconcile → serve for the index, including the
  removal path, against real Postgres.
- All quality gates green; the PR carries the rollout note; DESIGN-0003 is
  **Implemented** and the docs indexes are regenerated.
- The docz-site can pin spec `1.1.0` and swap its repo-home fallback — no
  docz-api follow-up required.

---

## File Changes

| File / package | Action | Description |
| --- | --- | --- |
| `internal/store/migrations/20260710000000_add_repo_index.sql` | Create | Nullable `repos.index_md` / `index_sha` (+ down migration). |
| `internal/store/queries/repos.sql` | Modify | `UpsertRepo` INSERT + `DO UPDATE SET` gain the two columns. |
| `internal/store/` (generated) | Regen | `just generate` refreshes `Repo` / `UpsertRepoParams`. |
| `internal/store/store.go` / `reconcile.go` | Modify | `RepoInput` fields + `textOrNull` mapping. |
| `internal/ingest/fetcher.go` | Modify | `RepoSnapshot` + `IndexMD` / `IndexSHA`. |
| `internal/ingest/service.go` | Modify | Map the pair into `RepoInput`. |
| `internal/githubapp/client.go` | Modify | `docsDirHint` + targeted `docs_dir/index.md` fetch in `Fetch`. |
| `internal/httpapi/dto.go` / `handler.go` | Modify | `repoIndexDTO` + `getRepoIndex` + route. |
| `api/openapi.yaml` | Modify | New path + `RepoIndex` schema; `info.version: 1.1.0`. |
| `internal/httpapi/openapi_contract_test.go` | Modify | Happy-path + no-index-404 cases; fixture updates. |
| `internal/e2e/` | Modify | Fake fetcher index body + serve/removal assertions. |
| `go.mod` / `go.sum` | Modify | `gopkg.in/yaml.v3` indirect → direct (no bare tidy). |
| `CLAUDE.md` / `docs/design/0003-*.md` | Modify | Conventions + phase notes; design → Implemented. |

## Testing Plan

- [x] **Store integration** (testcontainers): index pair set / update / clear
      round-trip through `ReconcileRepo`; empty-body + valid-sha case.
      _(`TestReconcileRepoIndexPair`; migration up/down by
      `TestMigrateUpDownRoundTrip`.)_
- [x] **githubapp unit** (stub RoundTripper + fixtures): present / absent /
      custom `docs_dir` / default fallback; absent ⇒ no extra blob call.
      _(`TestFetchRepoIndex` + the absent assertions in
      `TestFetchClassifiesAndDecodes` — the stub 404s unknown shas, so a
      stray blob call would fail the fetch.)_
- [x] **docsDirHint table test**: valid, empty, missing key, malformed YAML
      (→ default, never an error). _(`TestDocsDirHint`, five cases including
      trailing-slash trim.)_
- [x] **ingest unit**: snapshot pair lands on `RepoInput`.
      _(`TestRunMapsCustomTypeAndSkipsMissingFrontmatter`.)_
- [x] **httpapi unit**: 200 body+sha, empty-file 200 + `""`, three 404
      flavors (no index, unknown repo, unauthorized repo).
      _(`TestGetRepoIndex` + `TestGetRepoIndexUnauthorizedIs404`.)_
- [x] **OpenAPI contract**: `getRepoIndex` request+response validated; 404
      envelope case; spec self-validates at `1.1.0`; existing cases untouched
      (additivity proof). _(`getRepoIndex` + `getRepoIndexMissing` cases;
      only fixture seeds and count assertions shifted.)_
- [x] **e2e integration**: onboard serves the index; removal at HEAD flips to
      404. _(`TestE2ERepoIndexServeAndRemoval` against real Postgres.)_
- [x] No new integration _dependencies_ — everything rides the existing
      Postgres testcontainers + stub-HTTP patterns (Meilisearch/Redis
      untouched). _(Confirmed: the only new module dependency is the yaml.v3
      promotion, already indirect before.)_

## Dependencies

- **`gopkg.in/yaml.v3`** — already in `go.sum` (indirect); promoted to direct
  for `docsDirHint` (OQ-1a). No new module downloads.
- **docz v1.0.0 pin** — `doczcfg.WikiIndexName` + `DefaultConfig().DocsDir`
  are the only convention sources; `internal/doczcontract` already guards the
  pin.
- **Existing tooling** — goose + sqlc (`just generate`), testcontainers
  integration tests, the kin-openapi contract harness, vacuum/yamlfmt
  (`just lint-openapi`).
- **No coordination dependency** — the docz-site consumes at its own pace via
  the spec version (`1.1.0`); 404 keeps its current fallback.

## Open Questions

**Resolved 2026-07-10: all option `a`.** Decisions are recorded inline
(**→ Decision**) and were already reflected in the phase tasks above (which
cite the OQ-`n`a choices at each decision point); the original options are
kept for context. Each question is numbered; option `a` was the
recommendation, later letters alternatives, **Other** free-form. These are
**implementation** choices not already fixed by DESIGN-0003's resolved OQs.

### 1. Which YAML library for the docs_dir hint parse?

`docsDirHint` needs a one-field unmarshal inside `internal/githubapp`.

- **a (recommended):** **`gopkg.in/yaml.v3`**, promoted indirect → direct.
  Already in `go.sum` (docz itself parses `.docz.yaml` with it), so the hint
  parse agrees with the authoritative parser on YAML dialect corner cases;
  zero new downloads; the promotion follows the repo's go.mod convention.
- **b:** **`go.yaml.in/yaml/v3`** (also already indirect, via yamlfmt's
  lineage). Equivalent mechanics, but it is _not_ the library docz uses, so
  dialect drift between hint and authoritative parse becomes possible.
- **c:** **No YAML lib** — a hand-rolled scan for a top-level `docs_dir:`
  line. Zero deps, but wrong on quoting/comments/anchors and embarrassing to
  test; the hint must agree with real YAML semantics.
- Other.

**→ Decision: 1a.** `gopkg.in/yaml.v3`, promoted indirect → direct (the same
library docz parses `.docz.yaml` with — no dialect drift).

### 2. How does the contract test model the no-index 404?

`seededStore()` holds exactly one repo, and `TestReadEndpoints/list repos`
asserts that count. The happy path needs the fixture repo to gain an index;
the 404 case needs a repo **without** one.

- **a (recommended):** **Add a second bare fixture repo** (e.g. `acme/bare`,
  no types/docs/index) to `seededStore` and update the handful of count/shape
  assertions it shifts (`list repos` and any allowed-set tests). Most
  realistic — every test now runs against a store where both index states
  coexist, and future fixtures get a natural "empty repo" to lean on.
- **b:** **A dedicated store variant** (`seededStoreNoIndex()` or a per-case
  fake) used only by the 404 contract case. Zero churn on existing
  assertions, but the fixture forks — two stores to keep in sync as the
  surface grows.
- **c:** **Skip the 404-index case in the contract table** — the unknown-repo
  case already schema-validates the 404 `Error` envelope, and the absent-index
  behavior is covered by httpapi unit tests. Cheapest, but the contract test
  then never drives the new endpoint's 404 branch.
- Other.

**→ Decision: 2a.** Second bare fixture repo (`acme/bare`) in `seededStore`;
update the count/shape assertions it shifts.

### 3. Advertise index presence on existing repo DTOs?

Should the site be able to know a repo has a home document without calling
the index endpoint (e.g. `has_index: true` on `RepoSummary`/`RepoDetail`)?

- **a (recommended):** **No — keep existing DTOs untouched.** The site's
  repo-home page fetches `/index` exactly when it renders it and treats 404
  as "use the fallback" — a presence flag saves nothing on that flow. Purely
  additive discipline stays clean ("no existing schema changed" remains a
  provable success criterion), and a flag can still be added later as a
  compatible minor bump if a real grid/nav use case appears.
- **b:** **Add `has_index` to `RepoDetail` only** — the nav could grey out a
  "Home" entry without a probe request. One boolean, still additive, but it
  edits an existing schema this plan otherwise proves untouched, for a
  consumer feature nobody has asked for.
- **c:** **Add it to both `RepoSummary` and `RepoDetail`** — grid badges too;
  same trade-off, wider blast radius.
- Other.

**→ Decision: 3a.** No presence flag — existing DTOs stay untouched; the
site's fetch-and-404 flow needs nothing more, and a flag remains a compatible
minor bump later.

### 4. Cap the cached index.md size?

GitHub blobs can reach 100 MiB; `index_md` is TEXT and rides the repo row and
the response body. The `changelog_md` precedent has **no** cap.

- **a (recommended):** **No cap — follow the precedent.** These are homelab
  repos whose `index.md` is a human-written landing page (KB, not MB); the
  changelog cache has run capless without incident; and a truncation policy
  invents a new wire semantic (partial content) this design doesn't need.
  Revisit only if a real repo abuses it.
- **b:** **Skip-and-warn over a threshold** (e.g. 1 MiB): don't cache, log at
  warn, endpoint 404s. Bounds the row and response with no partial-content
  semantics, but silently makes a big-but-legitimate home page disappear.
- **c:** **Truncate at a threshold with a marker.** Keeps something
  renderable, but partial markdown can cut mid-construct and the "truncated"
  wire semantic needs spec/DTO surface this feature doesn't otherwise want.
- Other.

**→ Decision: 4a.** No cap — follow the capless `changelog_md` precedent;
revisit only if a real repo abuses it.

### 5. One PR or one PR per phase?

DESIGN-0003's rollout plan says "one PR (a single vertical slice)"; the
phases above are still separately committed.

- **a (recommended):** **One PR, four phase-commits** — matches the design's
  rollout intent (no specced-but-unserved window on `main`), keeps CI/review
  in one place, and the conventional commits preserve the phase story for
  git-cliff.
- **b:** **A PR per phase** — smaller reviews, but Phases 1–2 land columns
  and fetch plumbing that nothing serves, and the spec/endpoint PR (Phase 3)
  still needs them merged first; more churn for a plan this small.
- Other.

**→ Decision: 5a.** One PR (`feat/impl-repo-index`), four phase-commits — the
design's single-vertical-slice rollout, with the phase story preserved for
git-cliff.

## References

- [DESIGN-0003](../design/0003-repo-index-endpoint-serve-docsdir-indexmd-as-the-repo-home.md)
  — the design this plan implements (all six OQs resolved as option `a`)
- [INV-0003](../investigation/0003-docz-site-deferred-features-and-the-docz-api-surface-to-unblock.md)
  — the deferred-features disposition (finding F1)
- [DESIGN-0002](../design/0002-openapi-contract-for-docz-api-and-the-docz-site.md)
  / [IMPL-0002](0002-openapi-contract-for-docz-api-and-the-docz-site.md) — the
  wire-contract regime (spec, contract test, SemVer bump discipline)
- [IMPL-0001](0001-docz-api-cross-repo-docz-registry-and-ingestion-service.md)
  — the phased build this plan's store/ingest/e2e patterns extend
- `internal/store/reconcile.go` (`textOrNull`) — the empty-vs-absent gotcha
  that keys presence off `index_sha`
- `internal/githubapp/client.go` — `Fetch` / `classifyTree` / `fetchBlob`
  (the `CHANGELOG.md` targeted-fetch precedent)
- `internal/httpapi/handler.go` — `Mount` / `resolveRepo` (existence hiding)
- docz v1.0.0 — `config.WikiIndexName`, `DefaultConfig().DocsDir`
