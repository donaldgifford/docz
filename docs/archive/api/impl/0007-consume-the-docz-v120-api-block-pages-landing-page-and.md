---
id: IMPL-0007
title: "Consume the docz v1.2.0 api block: pages, landing page, and additional docs"
status: Draft
author: Donald Gifford
created: 2026-08-28
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0007: Consume the docz v1.2.0 api block: pages, landing page, and additional docs

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-08-28

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: docz pin bump + contract clause R10](#phase-1-docz-pin-bump--contract-clause-r10)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Persistence — the pages table and the api columns](#phase-2-persistence--the-pages-table-and-the-api-columns)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Fetch and ingest — the hint, the widened filter, the classifier](#phase-3-fetch-and-ingest--the-hint-the-widened-filter-the-classifier)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Serve — the pages endpoints and spec 1.3.0](#phase-4-serve--the-pages-endpoints-and-spec-130)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Webhook — matching files outside the docs dir](#phase-5-webhook--matching-files-outside-the-docs-dir)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Search — the source facet](#phase-6-search--the-source-facet)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: End-to-end proof, docs, and close-out](#phase-7-end-to-end-proof-docs-and-close-out)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. Shipping shape: how do the phases land on main?](#1-shipping-shape-how-do-the-phases-land-on-main)
- [References](#references)
<!--toc:end-->

## Objective

Implement [DESIGN-0004] (Approved; all OQs `a`): bump the docz pin to
`v1.2.0`, freeze contract clause R10, and grow the second record type —
**pages** — through every layer: fetch, classify, store, serve, webhook,
and search. When this closes, a repo that enables the `api:` block gets its
markdown published at `GET /api/v1/repos/{owner}/{name}/pages[/{path}]` and
findable in search under a `source` facet, while every repo without the
block is untouched byte-for-byte.

**Implements:** [DESIGN-0004] (grounded by [INV-0008], all six OQs answered
1a 2a 3a 4a 5a-amended 6a; design OQs answered 1a 2a 3a)

## Scope

### In Scope

- docz `v1.1.0 → v1.2.0` pin bump; `internal/doczcontract` clause **R10**
  (`Config.API`, `ErrInvalidAPIPath`, `docparse.Title`); the PLAN-flip
  fleet check as the bump's deploy gate.
- `repo_pages` table + `repos.api_landing_page`/`api_additional_docs`
  columns; `reconcileRepoPages` with the content-hash gate and
  desired-state deletion.
- `apiHint`, the enabled-only widened tree filter, per-entry
  `additional_docs` lookups, landing-page fetch following the config.
- The `buildPages` classifier: six-rule consumption, directory-page
  precedence (5a-amended), collision rule (design OQ-1a), title fallbacks.
- `GET .../pages` + `GET .../pages/{path}` with decoded-path re-validation;
  spec `1.2.1 → 1.3.0`; contract-test coverage.
- Webhook exact-path matching for the persisted landing page and
  additional docs.
- Search: `source` facet, page index docs with hashed PKs, `SearchHit`
  `source`/`path` fields.
- Docs: `deploy/README.md` + `api/README.md` consumer notes, CLAUDE.md
  section, docz-site coordination issue, upstream follow-ups to docz.

### Out of Scope

- Assets/images, the link graph, lifecycle dates, labels, rendering, and
  docz-site's page-tree UI — DESIGN-0004 Non-Goals, unchanged.
- Sha-gated blob fetching (design OQ-2a: deferred, recorded as the cost
  lever).
- Any authorization change — pages ride the same repo-scoped
  existence-hiding gate as every other repo surface.

## Implementation Phases

### Phase 1: docz pin bump + contract clause R10

The module moves to docz `v1.2.0` and the contract package freezes the new
surface before any runtime caller exists — the R6 pattern. The bump crosses
`v1.1.1` (PLAN default flip) and `v1.2.0` (changelog trailing-period
narrowing), so the existing suite doubles as the regression gate.

#### Tasks

- [ ] Run the PLAN-flip fleet check (DESIGN-0004's SQL) against the
      deployed database; for each hit, commit `types.plan.enabled: true`
      upstream in that repo or record the accepted deletion **here**. This
      gates the *deploy* of the bump, not the merge.
- [ ] Bump the pin: `go get github.com/donaldgifford/docz@v1.2.0` (**never**
      a bare `go mod tidy`), `go mod edit -fmt`; confirm no import-path
      changes and no new transitive requirements.
- [ ] R1–R6 green unchanged — the changelog trailing-period narrowing must
      not touch any pinned case (INV-0008 Observation 3 says it doesn't;
      prove it).
- [ ] Add `internal/doczcontract/api_test.go` (R10, the R6 mold: own file,
      doc-comment header, temp-dir + `HOME`-override loader): dormancy
      (absent block zero values; disabled block with hostile paths loads +
      validates clean), normalization (`./` strip, trailing-`/` collapse,
      landing-page backfill tracking a non-default docs dir), enabled
      validation (traversal / absolute / percent-encoded separator / Win32
      trailing period / under-docs-dir additional doc / reserved first
      segment → all `errors.Is(err, doczcfg.ErrInvalidAPIPath)`), and
      `docparse.Title` (ATX + inline strip, setext, frontmatter skipped,
      frontmatter `title:` ignored → `""`, prose-only → `""`).
- [ ] Update `internal/doczcontract/doc.go` (R1–R6 → R1–R6+R10) and the
      CLAUDE.md docz-pin note (`v1.1.0` → `v1.2.0`).
- [ ] `just test` / `just lint` / `just fmt` green; commit
      (`feat(doczcontract): pin docz v1.2.0 + freeze the api surface (R10)`).

#### Success Criteria

- `go.mod` requires docz `v1.2.0`, no `replace`; change set confined to
  `go.mod`/`go.sum`, `doczcontract`, CLAUDE.md, and this doc.
- R10 fails loudly if a future docz bump changes any pinned behavior;
  verified by revert drill (flip one pinned normalization case and watch
  it fail).
- The fleet-check outcome is recorded here before the release containing
  the bump deploys.

### Phase 2: Persistence — the pages table and the api columns

The second record type lands in Postgres with the same reconcile
discipline documents have; nothing reads it yet.

#### Tasks

- [ ] Migration `repo_pages` (DESIGN-0004 Data Model verbatim: id / repo
      FK CASCADE / path / repo path / title / git sha / content hash /
      raw md / updated at, `UNIQUE (repo_id, path)`, repo index) +
      `repos.api_landing_page` TEXT NULL + `repos.api_additional_docs`
      JSONB NULL; verified up/down.
- [ ] sqlc queries: `UpsertRepoPage`, `DeleteRepoPage`,
      `ListRepoPageHashes` (path + content hash, the gate read),
      `ListRepoPages` (no raw md), `GetRepoPageByPath` (with raw md);
      `just generate` / `generate-check` clean.
- [ ] `PageInput{Path, RepoPath, Title, GitSHA, ContentHash, RawMD}`;
      `ReconcileInput.Pages`; `RepoInput` gains `APILandingPage string` +
      `APIAdditionalDocs []string` (empty ⇒ NULL — desired state);
      `UpsertRepo` writes both columns.
- [ ] `reconcileRepoPages` inside the existing transaction, mirroring
      `reconcileDocuments`: hash-map read, content-hash gate, upsert
      changed, delete absent; `ReconcileResult` gains
      `PagesUpserted/PagesDeleted/PagesUnchanged` +
      `UpsertedPagePaths/DeletedPagePaths`.
- [ ] Store integration tests (`//go:build integration`): round-trip, gate
      no-op on unchanged hash, delete-absent, disable-at-HEAD (empty
      inputs) wipes rows and nulls both columns; migration up/down.
- [ ] `just test` / `just lint` / `just fmt` green; commit.

#### Success Criteria

- A `ReconcileInput` with pages round-trips; re-reconciling unchanged
  input reports all-unchanged and writes nothing; removing a page deletes
  its row; empty pages + empty api fields leave the repo exactly as a
  never-opted-in repo.

### Phase 3: Fetch and ingest — the hint, the widened filter, the classifier

The pipeline learns to see pages. The dormant-block invariant is the
headline: a repo without the block fetches and ingests byte-for-byte as
today.

#### Tasks

- [ ] `internal/githubapp` `apiHint(configYAML)` (third hint beside
      `docsDirHint`/`changelogHint`): enabled / landing page / exclude /
      additional docs, docz defaults on malformed yaml, `./` +
      trailing-`/` trimmed; unit table mirroring `TestChangelogHint`.
- [ ] `classifyTree`: when the hint is enabled, keep every `.md` under the
      docs dir (exclusion pruning stays in ingest); landing-page
      `findBlobSHA` at the hint's path (fallback `docs_dir/index.md`);
      per-entry `additional_docs` `findBlobSHA` (absent ⇒ zero requests).
      When dormant: today's keep-set, provably (withheld-blob stub tests —
      the `TestFetchRepoChangelog` technique).
- [ ] `internal/ingest`: `buildPages(cfg, blobs)` implementing the six
      classifier rules (DESIGN-0004): landing-page skip, over-fetch guard,
      templates/exclude pruning, type-dir discrimination (README kept as
      the directory page; `IsDoczFile` + frontmatter → document; stray →
      skip + Warn), directory-page precedence (README wins; lone index
      serves; loser path-addressed), additional-docs mapping. Collision
      rule (design OQ-1a): the docs-dir page wins deterministically; the
      additional doc is skipped with a Warn naming both files.
- [ ] Title mapping: `docparse.Title(content)`; fallback title-cased
      basename for file pages, directory name for directory pages (new
      small helper + tests).
- [ ] `Service.Run` wires pages into `ReconcileInput` and the api fields
      into `RepoInput` from the **post-Load** config; an invalid enabled
      block still fails the whole ingest via the existing `Validate` path
      (no new error path).
- [ ] Unit tests: the classifier table (every rule above gets a row — the
      type-dir README kept is its own named case), hint tables, dormant
      byte-for-byte fetch.
- [ ] `just test` / `just lint` / `just fmt` green; commit.

#### Success Criteria

- A fixture repo with an enabled block produces DESIGN-0004's
  published-path table exactly (directory pages at directory paths, file
  pages with extensions, additional docs repo-relative).
- A dormant-block fixture produces snapshots and reconcile inputs
  identical to today's — proven by tests, not asserted.
- The collision case is deterministic and Warn-logged; the type-dir
  README survives.

### Phase 4: Serve — the pages endpoints and spec 1.3.0

#### Tasks

- [ ] `httpapi.storeReader` gains `ListRepoPages` + `GetRepoPageByPath`;
      DTOs `pageSummaryDTO{path, title, git_sha}` /
      `pageDTO{repo, path, title, raw_md, git_sha}` (never expose sqlc
      types).
- [ ] `GET /api/v1/repos/{owner}/{name}/pages` — `{"pages":[…]}` ordered
      by path; empty set (including never-opted-in) ⇒ `200 {"pages":[]}`.
- [ ] `GET /api/v1/repos/{owner}/{name}/pages/*` — chi wildcard; decoded
      path re-validated before lookup (non-empty, no `..`/`.` segments, no
      leading `/`, no `\`, no control bytes) ⇒ reject as 404; exact-byte
      lookup; miss ⇒ 404 `{"error":"page not found"}`.
- [ ] Spec `1.3.0`: the two ops + `PageList`/`PageSummary`/`Page` schemas,
      `additionalProperties: false`; `{path}` documented as a
      slash-containing repo path (percent-encoded as one segment by
      clients); `getRepoIndex` description notes the configurable landing
      page. `just lint-openapi` (vacuum 100/100) + `yamlfmt` clean.
- [ ] Contract tests: list happy + empty, page happy (the percent-encoded
      spelling) + 404; traversal/undecodable paths 404; existing fixtures
      untouched.
- [ ] `just test` / `just lint` / `just fmt` green; commit.

#### Success Criteria

- Both endpoints validate against the served spec in the in-process
  contract test; a repo without pages serves an empty list and 404s every
  page path; no existing route or schema changed shape.

### Phase 5: Webhook — matching files outside the docs dir

#### Tasks

- [ ] `shouldIngest` gains the two exact-path checks from the repo row:
      `p == api_landing_page` (when set) and `p ∈ api_additional_docs`
      (when set); NULL columns match nothing.
- [ ] Unit tests: a landing page outside the docs dir triggers; an
      additional doc triggers; an unrelated root file still skips; NULL
      columns behave exactly as today.
- [ ] `just test` / `just lint` / `just fmt` green; commit.

#### Success Criteria

- A push touching only a root `CONTRIBUTING.md` listed in additional docs
  re-ingests; the same push against a never-opted-in repo is skipped —
  both proven at the `shouldIngest` table.

### Phase 6: Search — the source facet

Pages join the palette (INV-0008 OQ-1a; the site's acceptance bar). Doc
PKs are unchanged; pages hash (design OQ-3a).

#### Tasks

- [ ] `search.IndexDoc` gains `Source string`; `EnsureIndex` adds `source`
      to the filterable attributes (idempotent settings update).
- [ ] Page index mapping in `internal/ingest`: PK
      `<repo_id>_p_<hex(sha256(published_path))[:16]>`; fields source /
      repo / repo id / path / title / body / updated at; the doc mapping
      gains `Source: "doc"` + `Path`.
- [ ] `syncIndex` extends to `UpsertedPagePaths`/`DeletedPagePaths`
      (fetch changed rows via a `GetRepoPagesByPaths` query, delete by
      hashed PK); still best-effort, still after the Postgres commit.
- [ ] `SearchParams`/`buildFilter` untouched (repo scoping already covers
      pages via `repo_id`); `SearchHit` gains `source` + `path`; the spec
      bump rides this phase's PR per OQ-1 (a: `1.4.0`).
- [ ] Search integration tests: page indexed + searchable, `source` facet
      counts, page-hit shape (`""` doc fields), page deletion, repo-scope
      filter still applied, offboard purge covers pages.
- [ ] `just test` / `just lint` / `just fmt` green; commit.

#### Success Criteria

- The design's headline: a fixture repo's `CONTRIBUTING.md` and
  `docs/examples/example1.md` are findable via `GET /api/v1/search`, carry
  `source: "page"` and their published paths, and disappear from the index
  when removed at HEAD.

### Phase 7: End-to-end proof, docs, and close-out

#### Tasks

- [ ] `internal/e2e` integration test (the
      `TestE2ERepoChangelogServeAndDisable` shape): onboard a fixture repo
      with an enabled block → pages listed + served (a directory page, a
      file page, an additional doc) → push disabling the block → rows
      gone, list empty, 404s, index purged.
- [ ] Dogfood: enable the `api:` block in this repo's `.docz.yaml` (and
      docz's, via an upstream PR) — the rollout's first real traffic.
- [ ] Docs: `deploy/README.md` + `api/README.md` consumer notes
      (**enabling publishes every `.md` under the docs dir**; `exclude` is
      the guard rail); CLAUDE.md gains the DESIGN-0004/IMPL-0007 section.
- [ ] File the docz-site coordination issue (the list shape for `byPath`,
      the `source` facet, the reserved-word note, spec re-vendor).
- [ ] File the upstream docz follow-ups: close #81 against `v1.2.0`; the
      directory-page precedence ratification (5a-amended); the
      cross-namespace uniqueness validation (design OQ-1a's hardening
      half); the IMPL-0016 Phase-4 leftovers (status flips, release-notes
      extraction).
- [ ] Flip [DESIGN-0004] → Implemented and this doc → Completed; `docz
      update`; final `just ci` green.

#### Success Criteria

- Every DESIGN-0004 testing-strategy row has a named, passing test; the
  e2e run proves serve-and-disable end to end against real Postgres +
  Meilisearch; both follow-up issue sets exist; the docs say what ships.

## File Changes

| Area | Files |
| ---- | ----- |
| Contract | `go.mod`/`go.sum`; `internal/doczcontract/{api_test.go,doc.go}` |
| Store | `internal/store/migrations/2026…_add_repo_pages.sql`; `internal/store/queries/pages.sql`; `internal/store/{store.go,reconcile.go}` + generated |
| Fetch | `internal/githubapp/client.go` (+ tests, testdata) |
| Ingest | `internal/ingest/{service.go,pages.go,pagemap.go,indexmap.go}` (+ tests) |
| Serve | `internal/httpapi/{handler.go,handlers.go,dto.go}` (+ contract tests); `api/openapi.yaml`; `api/README.md` |
| Webhook | `internal/webhook/events.go` (+ tests) |
| Search | `internal/search/{types.go,client.go,search.go}` (+ integration tests) |
| e2e / docs | `internal/e2e/…`; `deploy/README.md`; `CLAUDE.md`; `.docz.yaml` |

## Testing Plan

DESIGN-0004's Testing Strategy is the checklist; each phase above names
its slice. Standards carried from prior IMPLs: table-driven, stdlib
`testing` only; integration behind `//go:build integration`
(testcontainers); the wire frozen by the in-process contract test with
`additionalProperties: false`; new guard tests verified by **revert
drill** (break the production rule, watch the test fail, restore); the
dormant-block invariant proven by withheld-blob stubs, not asserted.

## Dependencies

- `github.com/donaldgifford/docz v1.2.0` — the only dependency change in
  the entire feature. No new modules at any layer.
- Deploy-time: the Phase 1 fleet check runs against the production
  database before the release containing the bump rolls out.

## Open Questions

Answer each with a letter — **a is the recommendation**, b onward are
alternatives; write in your own option if none fits.

**Answered `1a` (2026-08-28).** The feature ships as two PRs: PR-1 =
Phases 1–5 (spec `1.3.0`, the pages endpoints), PR-2 = Phases 6–7 (spec
`1.4.0`, the `SearchHit` additions). This doc stays In Progress until
PR-2 merges.

### 1. Shipping shape: how do the phases land on main?

IMPL-0005 shipped its whole vertical as one PR (its OQ-4a: no
specced-but-unserved window on `main` — the served spec must never
describe endpoints that don't exist). This feature is roughly three times
that size, and its search phase is separable: the pages endpoints are
complete and correct before search lands.

- **a. (Recommendation) Two PRs, each a complete vertical: PR-1 = Phases
  1–5 (spec `1.3.0`, the pages endpoints only), PR-2 = Phases 6–7 (spec
  `1.4.0`, the `SearchHit` additions).** Each PR leaves `main` serving
  exactly what its spec says — which is also why the `SearchHit` fields
  cannot ride PR-1's bump (specced-but-unserved is the thing OQ-4a
  forbids). Reviews stay tractable at every layer; the IMPL stays open —
  and pages stay off the "done" list — until search lands, honoring
  INV-0008 OQ-1a's "not done until pages search".
- b. One PR for everything, IMPL-0005 style. Maximum coherence and a
  single spec bump — and a review spanning seven phases across every
  layer of the service.
- c. A PR per phase. Smallest reviews, but Phases 2–3 alone put
  stored-but-unserved data on `main`, and seven PRs means seven changelog
  drift cycles for one feature.

## References

- [DESIGN-0004] — the design this implements (Approved, all OQs `a`)
- [INV-0008] — the grounding investigation (Concluded, 1a 2a 3a 4a
  5a-amended 6a)
- IMPL-0005 — the changelog IMPL whose phase shape and one-PR precedent
  this follows and adapts
- docz DESIGN-0008 clause R10 / DESIGN-0011 — the upstream contract and
  consumption rule

[DESIGN-0004]: ../design/0004-consume-the-docz-v120-api-block-pages-landing-page-and.md
[INV-0008]: ../investigation/0008-adopt-the-docz-v120-api-block-additional-docs-landing-page-and.md
