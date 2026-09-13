---
id: IMPL-0010
title: "Ship timestamped and sortable search hits"
status: Draft
author: Donald Gifford
created: 2026-09-12
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0010: Ship timestamped and sortable search hits

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-09-12

<!--toc:start-->
- [Objective](#objective)
- [Background](#background)
  - [What changes, concretely](#what-changes-concretely)
  - [What deliberately does not change](#what-deliberately-does-not-change)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Dated hits and the UTC pin](#phase-1-dated-hits-and-the-utc-pin)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: The sort parameter and ranking rules](#phase-2-the-sort-parameter-and-ranking-rules)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: The source filter](#phase-3-the-source-filter)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Proof against real backends](#phase-4-proof-against-real-backends)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Docs, dependency gate, live smoke, close-out](#phase-5-docs-dependency-gate-live-smoke-close-out)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Rollout](#rollout)
- [Follow-ups](#follow-ups)
- [Open Questions](#open-questions)
  - [1. Where does the grpc CVE bump go?](#1-where-does-the-grpc-cve-bump-go)
  - [2. Validate the source filter value, or pass it through?](#2-validate-the-source-filter-value-or-pass-it-through)
  - [3. Which semver label does the PR carry?](#3-which-semver-label-does-the-pr-carry)
  - [4. Run a live smoke against the compose stack?](#4-run-a-live-smoke-against-the-compose-stack)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

## Objective

Land [DESIGN-0005] as one PR: every search hit carries `updated_at`
(RFC3339 UTC) and `created` (`YYYY-MM-DD`), `Document.updated_at` is pinned
to UTC, `searchDocs` accepts `sort` (four enumerated tokens, total order,
implicit secondary key, `400` on anything else) and a `source` filter, and
the OpenAPI contract bumps `1.4.2 → 1.5.0`. Closes issue #34.

**Implements:** [DESIGN-0005] (Approved, all five OQs `a`), grounded by
[INV-0009] (Concluded, all five OQs decided).

## Background

INV-0009 confirmed the change is read-side only: the index has carried
`updated_at` and `created` on every record since Phase 3, and the read path
drops them at three sites — the explicit `AttributesToRetrieve` list, the
`rawHit` decode target, and the `decodeHits` copy. DESIGN-0005 added the
sort (with the `sort` ranking rule moved first so a requested sort is a
total order), an implicit secondary key because every record a reconcile
touches shares one transaction timestamp, and the `source` filter the
`1.4.0` facet never got a query parameter for.

### What changes, concretely

- `internal/search`: retrieve list, `rawHit`, `decodeHits` + `formatUnix`,
  `SearchHit` gains `Created`/`UpdatedAt`, `SearchParams` gains
  `Sort`/`Source`, `ParseSort` + `ErrInvalidSort` + four `Sort*` tokens,
  `sortKeys` (secondary key), `sortableAttributes` shared with
  `EnsureIndex`, ranking rules reordered to `sort, words, typo, proximity,
  attribute, exactness`, `buildFilter` gains `source`.
- `internal/httpapi`: `searchDocs` parses `sort` (400 on
  `ErrInvalidSort`) and `source`; `nullTimestamp` renders `.UTC()`.
- `api/openapi.yaml`: `SearchHit` + `created`/`updated_at` (properties and
  `required`); `Document.updated_at` reworded; `sort` + `source` query
  parameters; a `BadRequest` response component and `400` on `searchDocs`;
  `info.version: 1.5.0`.
- Tests at every seam (unit, contract, integration, e2e); `api/README.md`
  and `CLAUDE.md` notes.

### What deliberately does not change

- No ingest, store, migration, or `IndexDoc` schema change; no reindex.
- Unsorted requests rank exactly as today (the `sort` rule is inert without
  the parameter).
- The pages endpoints (`Page`/`PageSummary`) stay undated — a recorded
  follow-up.
- Commit-dated history stays INV-0003 F3.

## Scope

### In Scope

- Everything in the DESIGN-0005 "API / Interface Changes" table.
- The tests DESIGN-0005's Testing Strategy names, at all four seams.
- `api/README.md` current-version paragraph + the versioning clarification;
  `CLAUDE.md` Phase 3 gotchas.
- Marking DESIGN-0005 Implemented and noting the landing in INV-0009.
- The `google.golang.org/grpc` CVE bump as Phase 1's prerequisite task
  (OQ-1), so the PR's Security Scan is never the blocker — landed early in
  the docs PR #35 for the same reason.
- Opening the docz-site follow-up issue once the PR merges (Phase 5's last
  task) — the site work itself stays out of scope.

### Out of Scope

- docz-site work (re-vendor `1.5.0`, drop the defensive read, directory
  default sort, fixtures) — a separate repo, tracked in Follow-ups.
- `updated_at` on the pages endpoints; `last_commit_at`-style history.
- The stale chart `appVersion` (`0.6.0` vs app `v0.9.0`) — unrelated,
  still awaiting a decision.

## Implementation Phases

Each phase builds on the previous one and leaves the tree green (`just
lint`, `just test`, `just lint-openapi`). A phase is complete when all its
tasks are checked off and its success criteria are met. Commits are
conventional and per numbered task; the changelog sync commit is always the
last one before a push.

---

### Phase 1: Dated hits and the UTC pin

The read-side fix for issue #34 proper, plus the second date and the zone
pin, landed together with their spec so the contract test never sees a
struct/spec mismatch mid-branch. The version bump happens here and holds
for the rest of the PR. The phase opens with the dependency bump the PR's
Security Scan would otherwise fail on (OQ-1), so every later push is
judged on its own changes.

#### Tasks

- [x] **Prerequisite (OQ-1):** bump `google.golang.org/grpc` to `v1.83.2`
      via `go get google.golang.org/grpc@v1.83.2` + `go mod edit -fmt`
      (never a bare `go mod tidy` — staged indirect deps get pruned); run
      the local Trivy scan the CI Security job mirrors
      (`trivy fs --scanners vuln --severity HIGH,CRITICAL --exit-code 1 .`)
      and `go build ./...`; commit on its own
      (`chore(deps): bump google.golang.org/grpc to v1.83.2`).
      **Landed early, in the docs PR #35 (2026-09-12):** the CI Security
      Scan job has no path filter, so the docs-only PR tripped on the
      same CVE (local Trivy: `Total: 1 (HIGH: 1)`, CVE-2026-84445). The
      implementation branch inherits the bump from `main`; re-run the
      local scan before Phase 1's first push to confirm nothing new
      appeared.
- [ ] `internal/search/search.go`: add `"created"` and `"updated_at"` to
      `AttributesToRetrieve`; add `Created string` and `UpdatedAt int64` to
      `rawHit`; copy both in `decodeHits`, the stamp through a new
      `formatUnix(sec int64) string` (`0 → ""`, else
      `time.Unix(sec, 0).UTC().Format(time.RFC3339)`), with a doc comment
      naming the retrieve list as the first drop site (INV-0009 F2).
- [ ] `internal/search/types.go`: `SearchHit` gains `Created string`
      (`json:"created"`) and `UpdatedAt string` (`json:"updated_at"`)
      placed before `Snippet`; the type comment states the ingest-observed
      semantics and the `""` conventions (page hits: `created` empty,
      `updated_at` real).
- [ ] `internal/httpapi/dto.go`: `nullTimestamp` renders
      `t.Time.UTC().Format(time.RFC3339)`; comment cites DESIGN-0005 (pgx
      scans in the process zone).
- [ ] `api/openapi.yaml`: `SearchHit.properties.created` + `.updated_at`
      and both in `required`; replace `Document.updated_at`'s description
      with the DESIGN-0005 wording and use the same text on the hit;
      `info.version: 1.5.0`.
- [ ] Unit tests: `formatUnix` table (zero, a value with `Z` suffix);
      `decodeHits` round-trips both fields from a canned Meilisearch hit;
      `nullTimestamp` renders `Z` for a non-UTC `time.Time`; the httpapi
      `TestSearchEndpoint` wire struct asserts `created`/`updated_at`.
- [ ] Contract test: `contractSearcher` returns `Created: "2026-01-15"`
      and `UpdatedAt: "2026-09-10T17:52:00Z"` so the schema's string type
      is exercised by real values.
- [ ] `just fmt`, `just lint`, `just lint-openapi`, `just test` green;
      commit (`feat(search): expose created and updated_at on search hits`).

#### Success Criteria

- The local Trivy scan reports no HIGH/CRITICAL findings and Dependabot
  alert #5 closes on push; `go build ./...` and `go vet ./...` clean after
  the bump.
- `go test ./internal/search/ ./internal/httpapi/` green, including the
  kin-openapi contract test against the `1.5.0` spec with the two new
  required properties.
- `just lint-openapi` stays 100/100.
- A hand-decoded Meilisearch hit with `updated_at: 1750615451` renders
  `"2026-06-22T18:04:11Z"` on the wire, matching what `nullTimestamp`
  renders for the same instant.

---

### Phase 2: The sort parameter and ranking rules

The `sort` query parameter, its allowlist, the implicit secondary key, the
ranking-rule move, and the first documented `400` on the read surface.

#### Tasks

- [ ] `internal/search/client.go`: hoist the sortable list to
      `var sortableAttributes = []string{"created", "updated_at"}` and use
      it in `EnsureIndex`; reorder `RankingRules` to
      `sort, words, typo, proximity, attribute, exactness` with a comment
      stating why (inert without `sort`; a requested sort is a total
      order; DESIGN-0005).
- [ ] `internal/search/types.go`: the four `Sort*` token constants,
      `ErrInvalidSort`, `ParseSort(s string) (string, error)` (`""` → `""`;
      accepted token → itself; else `ErrInvalidSort`); `SearchParams` gains
      `Sort string` documented as "a `ParseSort`-validated token or `""`".
- [ ] `internal/search/search.go`: `sortKeys(token) []string` returning
      the requested key plus its fixed secondary (the DESIGN-0005 table);
      `Search` sets `req.Sort = sortKeys(p.Sort)` when `p.Sort != ""`.
- [ ] `internal/httpapi/search.go`: parse `sort` via `search.ParseSort`
      before the search; on `ErrInvalidSort` →
      `writeError(w, http.StatusBadRequest, "invalid sort")` and return
      without calling the searcher; otherwise set `params.Sort`.
- [ ] `api/openapi.yaml`: `sort` query parameter on `searchDocs` with the
      four-value `enum` and the DESIGN-0005 description (total order,
      relevance breaks ties, `created` string-sort note);
      `components.responses.BadRequest` (`Error` envelope); `"400"` on
      `searchDocs` referencing it.
- [ ] Unit tests: `ParseSort` table (`""`, each token, `updated_at`,
      `UPDATED_AT:desc`, `updated_at:down`, `body:desc`, a token with
      surrounding whitespace); `sortKeys` table (each token → its pair); a
      guard that every token's attribute is in `sortableAttributes` and
      every sortable attribute has both direction tokens; httpapi:
      `sort=updated_at:desc` reaches the fake searcher as `Sort`,
      `sort=bogus` → `400 {"error":"invalid sort"}` with the searcher never
      called, absent `sort` → `Sort == ""`.
- [ ] Contract test: add `searchDocsSorted` to `TestOpenAPIContract`
      (`/api/v1/search?q=intro&sort=updated_at:desc`) — request validation
      proves the enum, response validation the `200`. No `400` case: an
      out-of-enum value fails kin-openapi's request validation before the
      handler runs (DESIGN-0005 Testing Strategy).
- [ ] `just fmt`, `just lint`, `just lint-openapi`, `just test` green;
      commit (`feat(search): sort parameter with a total-order ranking`).

#### Success Criteria

- `go test ./internal/search/ ./internal/httpapi/` green; the contract
  test validates the sorted request against the enum.
- `sort=bogus` is a `400` with the JSON error envelope and never reaches
  Meilisearch; an absent `sort` produces a request byte-identical to
  today's (no `sort` key in the Meilisearch request body).
- `just lint-openapi` 100/100 with the new parameter and response
  component.

---

### Phase 3: The source filter

The one-line-per-layer companion DESIGN-0005 OQ-5a pulled in: callers can
ask for documents or pages only, with accurate totals and offsets.

#### Tasks

- [ ] `internal/search/types.go`: `SearchParams` gains `Source string`
      (doc comment: `"doc"`/`"page"`, `""` for both).
- [ ] `internal/search/search.go`: `buildFilter` appends
      `appendEq(parts, "source", p.Source)` after `author`, keeping the
      documented clause order.
- [ ] `internal/httpapi/search.go`: `Source: q.Get("source")`, passed
      through unvalidated like the four existing facet filters (OQ-2a); a
      comment says why `source` is not a `400` while `sort` is.
- [ ] `api/openapi.yaml`: `source` query parameter on `searchDocs`,
      `enum: [doc, page]`, description "Filter by record kind."
- [ ] Unit tests: `TestBuildFilter` gains a `source` case and an
      all-facets-in-order case including it; httpapi asserts `source=page`
      reaches the searcher and that `source=bogus` is passed through (a
      `200` with whatever the searcher returns — no `400`).
- [ ] Contract test: `searchDocsSource` case
      (`/api/v1/search?q=intro&source=doc`).
- [ ] `just fmt`, `just lint`, `just lint-openapi`, `just test` green;
      commit (`feat(search): source filter on searchDocs`).

#### Success Criteria

- `buildFilter` emits `… AND source = "page"` exactly once and in the
  documented position; the contract test validates the `source` enum.
- The unfiltered request is unchanged (no `source` clause when the
  parameter is absent).

---

### Phase 4: Proof against real backends

The retrieve list and the ranking-rule placement are invisible to the
unit tests (which fake the searcher); the shared-transaction timestamp is
invisible to the search integration test (which seeds the index directly).
This phase proves each claim at the seam that can see it.

#### Tasks

- [ ] `internal/search/search_integration_test.go` (real Meilisearch):
  - [ ] a hit's `UpdatedAt` is a non-empty RFC3339 string ending in `Z`,
        and `Created` is the seeded `YYYY-MM-DD` on doc hits / `""` on
        page hits (proves the retrieve list);
  - [ ] empty query + `Sort: SortUpdatedDesc` over both repos → the exact
        reverse of the seed order (all six records);
  - [ ] `Query: "logging"` + `SortUpdatedDesc` → `2_p_99…`, `2_ADR-0001`,
        `1_RFC-0001` — a total order in which the most relevant hit is
        last (proves `sort` leads the ranking rules);
  - [ ] `Sort: SortCreatedDesc` → the three page records last;
        `SortCreatedAsc` → first (the `""` behavior the spec promises);
  - [ ] `Source: SourceDoc` yields three hits and no page; `SourcePage`
        the converse;
  - [ ] after a second `EnsureIndex` on the existing index,
        `GetRankingRulesWithContext` returns the new order (proves the
        settings migrate on an already-populated index, the deploy path).
- [ ] `internal/e2e/search_integration_test.go` (real Postgres +
      Meilisearch through the ingest pipeline):
  - [ ] the wire struct gains `created`/`updated_at`; after onboarding,
        `updated_at` parses as RFC3339 with a `Z` suffix and is byte-equal
        to `Document.updated_at` for the same doc via `getDoc`;
  - [ ] `created` on the hit equals the fixture's frontmatter date;
  - [ ] add a created-parameterized fixture helper beside `doc()` (which
        hardcodes `2026-07-01`) and give the two search fixture docs
        distinct dates, then assert `sort=created:desc` orders them — the
        `updated_at` order cannot be proven here because one reconcile
        stamps both docs identically (INV-0009 F4 addendum), and the test
        says so in a comment;
  - [ ] `sort=bogus` through the real router → `400`.
- [ ] `just test-integration` green locally (Docker required).

#### Success Criteria

- Every integration and e2e case above passes against
  `getmeili/meilisearch:v1.12` and the testcontainers Postgres.
- The ranking-rule assertion fails by name if `sort` is moved back
  (revert-drilled once, not kept as a test).
- CI's `Test Go` job (unit + contract) stays green; the integration tag
  runs locally as today.

---

### Phase 5: Docs, dependency gate, live smoke, close-out

Everything a reader or operator needs, the CI gate the PR must clear, and
the docz-side status flips.

#### Tasks

- [ ] `api/README.md`: current-version paragraph → `1.5.0` (dated hits,
      `sort`, `source`, first `400`), inserting the skipped `1.4.2` entry
      (the `groups` description, PR #32); clarify the **major** rule's
      "newly required field" as request-side, citing the `1.4.0` precedent
      for response properties.
- [ ] `CLAUDE.md` Phase 3 search bullets: a GOTCHA for the explicit
      `AttributesToRetrieve` list (a new attribute must be added there or
      it never reaches the decoder), one for the `sort` ranking-rule
      placement (inert without the parameter; first = total order), and a
      line on the shared-transaction `updated_at` and the implicit
      secondary key.
- [ ] Live smoke (OQ-4a): `docker compose up -d`, `just run`, `-onboard`
      this repo (it dogfoods the `api:` block, so both record kinds exist),
      then `curl` the four sorts, `source=doc`, `source=page`, a bogus
      sort (`400`), and Meilisearch's
      `GET /indexes/documents/settings/ranking-rules`; record the evidence
      in this phase's status block.
- [ ] DESIGN-0005: status `Implemented` + a dated landing note; INV-0009:
      a one-line "landed in IMPL-0010" under the Recommendation.
- [ ] `docz update` (then revert its underscore-anchor mangling in older
      docs), `just ci` green, `mise exec -- git-cliff -o CHANGELOG.md` +
      `chore(changelog): Auto-sync` as the last commit; open the PR with
      the `minor` label (OQ-3a), body ending with the Claude Code footer.
- [ ] **After the PR merges and the release tags** — open a GitHub issue
      in `donaldgifford/docz-site` describing what docz-api changed and
      what the site must do to use it. Title
      `docz-api v0.10.0 / spec 1.5.0: dated, sortable, source-filterable
      search hits`. Body sections:
  - **What changed in docz-api** (link the release, DESIGN-0005, and
    issue #34): `SearchHit` gains `created` (`YYYY-MM-DD`, `""` on page
    hits) and `updated_at` (RFC3339 UTC, ingest-observed change time, real
    on page hits too); `searchDocs` accepts `sort` (enum
    `updated_at:desc|asc`, `created:desc|asc`; total order; unknown →
    `400 {"error":"invalid sort"}`) and `source` (`doc|page`);
    `Document.updated_at` is now always `Z`-suffixed.
  - **Required to support it**: re-vendor `api/openapi.yaml` at `1.5.0`
    and regenerate the client (the union types for `sort`/`source` and the
    two new properties); pass `sort=updated_at:desc` as the directory's
    default order; update `src/mocks/fixtures.ts` so page hits carry a
    real `updated_at` and doc hits carry `created`.
  - **Optional cleanups**: drop the defensive cast in `hitUpdatedAt`
    (`src/lib/updatedAt.ts`) now that the property is typed; render
    `created` on the directory card if wanted; use `source=doc` for a
    documents-only listing instead of client-side filtering.
  - **Nothing breaks without action**: the column lights up on deploy via
    the existing defensive read; everything else is additive.

#### Success Criteria

- `just ci` passes locally; the PR's Lint, Test Go, Security Scan, Build,
  and changelog drift checks are green.
- `api/README.md` and the served `/openapi.yaml` agree on `1.5.0`.
- Every DESIGN-0005 "API / Interface Changes" row is traceable to a
  commit in the PR.
- The live-smoke evidence is recorded in this phase's status block.
- The docz-site issue exists, links the docz-api release, and its
  "required" list matches the shipped contract (the post-merge task is
  the one item that cannot close before the merge; mark it `deferred —
  human required` only if the merge itself is pending).

---

## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `internal/search/search.go` | Modify | retrieve list; `rawHit`/`decodeHits` + `formatUnix`; `sortKeys`; `req.Sort`; `buildFilter` `source` |
| `internal/search/types.go` | Modify | `SearchHit.Created`/`UpdatedAt`; `SearchParams.Sort`/`Source`; `Sort*` tokens; `ErrInvalidSort`; `ParseSort` |
| `internal/search/client.go` | Modify | `sortableAttributes`; ranking rules `sort` first |
| `internal/search/filter_test.go` | Modify | `source` cases |
| `internal/search/search_test.go` | Create | `formatUnix`, `decodeHits`, `ParseSort`, `sortKeys`, sortable-guard tables |
| `internal/search/search_integration_test.go` | Modify | dated hits, sort order, total order, `created` edges, `source`, ranking-rule read-back |
| `internal/httpapi/search.go` | Modify | `sort` parse + `400`; `source` |
| `internal/httpapi/search_test.go` | Modify | sort passthrough, `400`, `source` |
| `internal/httpapi/dto.go` | Modify | `nullTimestamp` `.UTC()` |
| `internal/httpapi/openapi_contract_test.go` | Modify | fixture dates; `searchDocsSorted`, `searchDocsSource` cases |
| `internal/e2e/search_integration_test.go` | Modify | wire struct; RFC3339 + byte-equality; `created` sort; `400` |
| `internal/e2e/onboard_integration_test.go` | Modify | created-parameterized fixture helper |
| `api/openapi.yaml` | Modify | `SearchHit` fields; `Document.updated_at` wording; `sort`/`source` params; `BadRequest`; `1.5.0` |
| `api/README.md` | Modify | current version; versioning clarification |
| `CLAUDE.md` | Modify | Phase 3 gotchas |
| `go.mod` / `go.sum` | Modify | `grpc v1.83.2` (Phase 1 prerequisite, OQ-1) |
| `docs/design/0005-*.md`, `docs/investigation/0009-*.md` | Modify | status / landing notes |
| `CHANGELOG.md` | Modify | git-cliff sync |

## Testing Plan

- [ ] Unit (`just test`): `formatUnix`, `decodeHits`, `nullTimestamp` UTC,
      `ParseSort`, `sortKeys`, the sortable-attribute guard, `buildFilter`
      `source`, httpapi sort/source/400.
- [ ] Contract (`just test`, no tag): fixture carries real dates;
      `searchDocsSorted` + `searchDocsSource` validate the enums and the
      `200`s; `doc.Validate` covers `BadRequest`.
- [ ] Integration (`just test-integration`): retrieve list, exact sort
      orders, total order under a query, `created` string-sort edges,
      `source` filter, ranking rules read back after a re-`EnsureIndex`.
- [ ] e2e (`just test-integration`): RFC3339 `Z` + byte-equality with
      `Document.updated_at`, `created` passthrough and sort, `400` through
      the real router.
- [ ] Spec (`just lint-openapi`): vacuum 100/100, yamlfmt canonical.
- [ ] Live smoke (OQ-4a): compose stack + `just run`, this repo onboarded,
      `curl` the four sorts, `source=doc`, a bogus sort, and the
      Meilisearch `GET /indexes/documents/settings/ranking-rules`.

## Rollout

- One PR, one `minor` release (OQ-3a). On deploy the new binary's `EnsureIndex`
  applies the ranking-rule order before serving; during a rolling update
  old pods never send `sort`, so their results are unchanged.
- `Document.updated_at` gains a `Z` suffix only on hosts whose process zone
  was not UTC; the container already was.
- No migration, no reindex, no chart change, no config change.
- docz-site's directory column lights up on deploy via its defensive read;
  everything else there is a follow-up at its own pace.

## Follow-ups

- **docz-site**: tracked by the issue Phase 5's last task opens after the
  merge (re-vendor `1.5.0`; drop the `hitUpdatedAt` cast; pass
  `sort=updated_at:desc` as the directory default; render `created`;
  fixtures emit real page stamps).
- **Pages endpoints**: `updated_at` on `PageSummary`/`Page` (own minor
  bump).
- **Staleness automation** (e.g. "not updated in 180 days"): builds on
  `updated_at` once it persists across deployments; a re-onboard resets
  the clock, so commit-dated history (INV-0003 F3) is the robust source
  when that lands. A docz-side generated "last content update" frontmatter
  field is a possible upstream alternative.
- **Chart `appVersion`** is still `0.6.0` against app `v0.9.0`.

## Open Questions

Each question is numbered; option **a** is the recommendation, later
letters are alternatives, and **Other** is free-form. **All four answered
2026-09-12**; the decision line under each question is authoritative and
the phases above already reflect it.

### 1. Where does the grpc CVE bump go?

The open Dependabot alert on `google.golang.org/grpc` (`>= 1.83.0,
< 1.83.2`, high) will fail the PR's Security Scan job — Trivy gates on
`HIGH,CRITICAL` with `exit-code: 1` — regardless of anything this PR does.

- **a (recommended): same PR, as its own first `chore(deps)` commit.** The
  PR #29 precedent (x/crypto + grpc rode with the fix), one green CI run,
  and the bump is one `go get google.golang.org/grpc@v1.83.2` +
  `go mod edit -fmt`. Own commit keeps the changelog honest.
- **b: a separate one-line chore PR merged first.** Cleaner history and a
  `patch` release of its own, at the cost of a second review cycle before
  this PR can go green.
- **c: wait for Renovate.** Its next run may or may not precede this PR;
  CI stays red until it does.
- Other: \_\_\_\_\_

**Answered `1a`, positioned as Phase 1's prerequisite task** (same
branch, own commit, first thing pushed) so the Security Scan never blocks
the implementation PR. **In practice it landed one PR earlier:** the docs
PR (#35) hit the same scan, so the bump rode there as its own
`chore(deps)` commit (the PR #18 precedent) and the prerequisite is
already checked off above.

### 2. Validate the source filter value, or pass it through?

`sort` is validated with a `400` because a silently ignored sort changes
the order the caller sees. `source` is a facet filter like `repo`, `type`,
`status`, and `author`, none of which are validated today — an unknown
value simply matches nothing.

- **a (recommended): pass it through, like the other facet filters.** A
  bogus `source` returns zero hits, which is self-evident; the spec's
  `enum: [doc, page]` documents the values and the contract test's request
  validation enforces them for spec-conformant clients. No new error path.
- **b: validate and `400`, like `sort`.** Consistent with the new
  parameter's strictness, but inconsistent with the four existing filters,
  and it adds a second allowlist for two values.
- Other: \_\_\_\_\_

**Answered `2a`.** Pass-through; the spec enum documents the values.

### 3. Which semver label does the PR carry?

- **a (recommended): `minor`** → `v0.10.0`. New API surface (two response
  properties, two query parameters, a new response code) and a contract
  minor bump; matches how #32 (configurable scopes) and the pages PRs were
  labeled.
- **b: `patch`** → `v0.9.1`. Treats #34 as a bug fix ("the field was
  always meant to be there"); undersells the sort and source additions
  that consumers will pin against.
- Other: \_\_\_\_\_

**Answered `3a`.** `minor` → `v0.10.0`.

### 4. Run a live smoke against the compose stack?

- **a (recommended): yes, short and recorded.** `docker compose up -d`,
  `just run`, `-onboard` this repo (it dogfoods the `api:` block, so both
  record kinds exist), then `curl` the four sorts, `source=doc`, a bogus
  sort, and read `GET /indexes/documents/settings/ranking-rules` from
  Meilisearch. Ten minutes; catches anything the fakes and the seeded
  index cannot (the IMPL-0006 "reconstruct it from evidence" standard),
  and the evidence goes in Phase 5's status block.
- **b: tests only.** Phase 4's integration and e2e cases cover every
  claim; skip the manual step.
- Other: \_\_\_\_\_

**Answered `4a`.** Live smoke runs in Phase 5 and its evidence is recorded
there.

## Dependencies

- None new. `meilisearch-go v0.36.3` already exposes `SearchRequest.Sort`
  and `GetRankingRulesWithContext`; `kin-openapi v0.144.0` validates enum
  query parameters.
- The grpc bump is Phase 1's first commit on the implementation branch
  (OQ-1); nothing external gates the PR.
- The docz-site work is a separate repo: this plan's only obligation is
  the follow-up issue Phase 5 opens after the merge. The site changes
  themselves are **deferred — human required**.

## References

- [DESIGN-0005] — the design this plan implements (Detailed Design,
  Testing Strategy, decisions 1a–5a)
- [INV-0009] — findings F1–F8, decisions 1–5, the shared-transaction
  timestamp addendum
- Issue #34 — `searchDocs: expose the already-indexed updated_at on SearchHit`
- [DESIGN-0002] / [IMPL-0002] — the contract regime (spec, kin-openapi
  test, SemVer, `just lint-openapi`)
- [IMPL-0009] — the most recent single-PR precedent for phase and
  commit shape
- `internal/httpapi/openapi_contract_test.go:324-353` — the case table the
  new cases join; `internal/search/search_integration_test.go:86-128` —
  the seed corpus with strictly increasing `UpdatedAt`;
  `internal/e2e/onboard_integration_test.go:103` — the `doc()` fixture
  helper (hardcoded `created: 2026-07-01`)
- `.github/workflows/ci.yml` — Security Scan (Trivy `HIGH,CRITICAL`,
  `exit-code: 1`); Dependabot alert #5 (`grpc` → `1.83.2`)

[DESIGN-0005]: ../design/0005-timestamped-and-sortable-search-hits.md
[INV-0009]: ../investigation/0009-expose-the-indexed-updated-timestamp-on-search-hits.md
[DESIGN-0002]: ../design/0002-openapi-contract-for-docz-api-and-the-docz-site.md
[IMPL-0002]: 0002-openapi-contract-for-docz-api-and-the-docz-site.md
[IMPL-0009]: 0009-stop-publishing-type-dir-readmes-as-directory-pages.md
