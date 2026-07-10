---
id: DESIGN-0002
title: "OpenAPI contract for docz-api and the docz-site"
status: Implemented
author: Donald Gifford
created: 2026-07-08
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0002: OpenAPI contract for docz-api and the docz-site

**Status:** Implemented
**Author:** Donald Gifford
**Date:** 2026-07-08

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [The spec artifact: api/openapi.yaml](#the-spec-artifact-apiopenapiyaml)
  - [Endpoints in scope](#endpoints-in-scope)
  - [Component schemas (from the DTOs)](#component-schemas-from-the-dtos)
  - [Security schemes](#security-schemes)
  - [The contract test (kin-openapi harness)](#the-contract-test-kin-openapi-harness)
  - [Serving and surfacing the spec](#serving-and-surfacing-the-spec)
  - [Dependency and CI](#dependency-and-ci)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
- [Follow-ups](#follow-ups)
- [References](#references)
<!--toc:end-->

## Overview

docz-api will gain a hand-authored **`api/openapi.yaml`** (OpenAPI 3.1) that is
the machine-readable contract for its `/api/v1` HTTP surface. The spec is the
single source of truth; a **`kin-openapi` in-process contract test** drives the
real chi handler on every CI run and validates each request/response against the
spec, so the two can never silently drift. The **docz-site** consumes the spec
via *vendor-and-generate* to produce its typed client. This is the spec-first
pattern proven in **`rfc-api`** and recommended by **INV-0002** — adopted here as
the fleet standard.

## Goals and Non-Goals

### Goals

- Publish an **OpenAPI 3.1** document describing docz-api's `/api/v1` surface
  that the docz-site (and any consumer) can generate a typed client from.
- **Prevent drift**: a CI-enforced check fails the build whenever the server's
  actual request/response shapes diverge from the spec.
- **Reuse the fleet pattern** (rfc-api: spec-first + `kin-openapi`) with minimal
  new machinery — plausibly the same harness code.
- Keep the change **additive and non-breaking**: the spec documents the wire
  shapes docz-api already ships and that Phase 7's golden fixtures froze.

### Non-Goals

- **Generating the spec from code** (swaggo annotations) or adopting a
  spec-emitting HTTP framework (Huma / ogen). INV-0002 rules these out.
- **Redesigning the HTTP API.** This documents the surface as it is; wire
  reshaping (RFC 7807 errors, bare-array lists) is deferred to follow-up
  investigations — see [Follow-ups](#follow-ups) (FU-1, FU-2).
- **Building the docz-site client.** That lives in the docz-site repo
  (vendor-and-generate); this repo's only job is keeping `api/openapi.yaml`
  accurate.
- Auth/authorization redesign (the `authorize` seam and session model are
  unchanged).

## Background

**INV-0002** investigated how to give docz-api an OpenAPI contract and found
that `rfc-api` — the prior attempt at this same service — already solved it
**spec-first**: a hand-authored `api/openapi.yaml` (OAS 3.1, ~13 paths) kept
honest by a `kin-openapi` (`v0.135.0`) contract test that loads the spec, builds
a spec-derived router (`gorillamux`), runs the real handler in-process with
in-memory fakes, and validates request **and** response via `openapi3filter` on
every CI run. `rfc-site` (the frontend) vendors `api/openapi.yaml` and generates
a typed client from it.

docz-api is architecturally a near-twin of rfc-api, so the pattern ports
directly — but its **current wire shapes differ from rfc-api's conventions** in
ways that become explicit design decisions:

| Concern | docz-api today | rfc-api convention |
| --- | --- | --- |
| Error body | `{"error": "message"}` | RFC 7807 `application/problem+json` |
| List responses | envelope object: `{"repos":[…]}`, `{"types":[…]}`, `{"docs":[…]}` | bare JSON array + pagination in headers (`X-Total-Count`, `Link`) |
| Pagination | none on reads; `offset`/`limit` on `/search` only | opaque `cursor` + `Limit`, header `Link` |
| Search shape | flat `SearchHit` + separate `facets` map | `SearchResult` wraps a full `Document` + `score`/`snippet` |
| Auth in spec | real session cookie (`docz_session`) + webhook HMAC | none modeled |

Phase 7 already froze docz-api's read+search wire shape with **golden JSON
fixtures** (`internal/httpapi/contract_test.go`, `testdata/contract/*.json`).
Those lock exact bytes but are not a machine-readable, consumer-facing contract
and describe no schemas/params/status codes — which is the gap this design fills.

## Detailed Design

### The spec artifact: api/openapi.yaml

A single hand-authored file at **`api/openapi.yaml`** (matching rfc-api's
layout), `openapi: 3.1.0`, organized `info → servers → tags → paths →
components`. `servers` is same-origin (`- url: "/"`) so the contract-test router
resolves `http://localhost/api/v1/...` requests. `info` carries only
`title`/`version`/`description` — **not** `info.summary` (kin-openapi rejects it,
see [Testing Strategy](#testing-strategy)). `operationId`s are lowerCamelCase
verb+noun (`listRepos`, `getRepo`, `listTypes`, `listDocs`, `getDoc`,
`searchDocs`, …).

### Endpoints in scope

The current surface (from `cmd/docz-api/main.go` + `internal/httpapi`):

| Method | Path | operationId | Auth | Notes |
| --- | --- | --- | --- | --- |
| GET | `/api/v1/repos` | `listRepos` | session | `{"repos":[RepoSummary]}`, filtered to allowed set |
| GET | `/api/v1/repos/{owner}/{name}` | `getRepo` | session | `RepoDetail`; 404 hides unauthorized |
| GET | `/api/v1/repos/{owner}/{name}/types` | `listTypes` | session | `{"types":[DocType]}` |
| GET | `/api/v1/repos/{owner}/{name}/types/{type}/docs` | `listDocs` | session | `{"docs":[Document]}` (no `raw_md`); `{type}` resolves by name/prefix/alias |
| GET | `/api/v1/repos/{owner}/{name}/types/{type}/docs/{doc_id}` | `getDoc` | session | single `Document` **with** `raw_md`; 404 on type mismatch |
| GET | `/api/v1/search` | `searchDocs` | session | `SearchResult`; `q`,`repo`,`type`,`status`,`author`,`offset`,`limit` |
| GET | `/api/v1/auth/session` | `getSession` | session | `Session` or 401 |
| POST | `/api/v1/auth/logout` | `logout` | session | `{"status":"logged out"}` |
| GET | `/auth/login` | `login` | public | 302 → provider; `provider` query param |
| GET | `/auth/callback` | `authCallback` | public (signed state) | 302 → `/` |
| POST | `/webhooks/github` | `githubWebhook` | HMAC | `X-Hub-Signature-256`, `X-GitHub-Event`, `X-GitHub-Delivery`; 202/400/401 |
| GET | `/healthz` | `healthLive` | none | `{"status":"ok"}` |
| GET | `/readyz` | `healthReady` | none | per-dep map, 200/503 |
| GET | `/metrics` | `metrics` | none | Prometheus text |

The exact scope to spec in the first pass is **OQ-3**.

### Component schemas (from the DTOs)

Schemas map ~1:1 from `internal/httpapi/dto.go` and `internal/search/types.go`.
All nullable columns serialize as empty strings (never `null`); JSONB string
arrays serialize as `[]` (never `null`) — the schemas reflect that.

Error envelope (docz-api's current shape; RFC 7807 is **OQ-1**):

```yaml
Error:
  type: object
  required: [error]
  additionalProperties: false
  properties:
    error: { type: string, description: Human-readable error message. }
```

`Document` (the single richest schema; `raw_md` present only on `getDoc`):

```yaml
Document:
  type: object
  required: [repo, doc_id, type, title, status, author, created,
             path, git_sha, content_hash, updated_at]
  properties:
    repo:         { type: string, example: "acme/platform" }
    doc_id:       { type: string, example: "FW-0001" }
    type:         { type: string, description: Canonical type name. }
    title:        { type: string }
    status:       { type: string, description: '"" when unset.' }
    author:       { type: string, description: '"" when unset.' }
    created:      { type: string, description: 'YYYY-MM-DD, or "" when unset.' }
    path:         { type: string }
    git_sha:      { type: string }
    content_hash: { type: string }
    updated_at:   { type: string, description: 'RFC3339, or "" when unset.' }
    raw_md:       { type: string, description: Full Markdown; only on the single-document endpoint. }
```

Representative list endpoint (envelope shape; bare-array is **OQ-2**):

```yaml
/api/v1/repos:
  get:
    tags: [repos]
    operationId: listRepos
    responses:
      "200":
        description: Repositories the caller may see.
        content:
          application/json:
            schema:
              type: object
              required: [repos]
              properties:
                repos:
                  type: array
                  items: { $ref: "#/components/schemas/RepoSummary" }
      "401": { $ref: "#/components/responses/Unauthorized" }
```

Search endpoint (docz-api's `offset`/`limit` + facet map):

```yaml
/api/v1/search:
  get:
    tags: [search]
    operationId: searchDocs
    parameters:
      - { name: q,      in: query, schema: { type: string } }
      - { name: repo,   in: query, schema: { type: string } }
      - { name: type,   in: query, schema: { type: string } }
      - { name: status, in: query, schema: { type: string } }
      - { name: author, in: query, schema: { type: string } }
      - { name: offset, in: query, schema: { type: integer, format: int64, minimum: 0, default: 0 } }
      - { name: limit,  in: query, schema: { type: integer, format: int64, minimum: 0, default: 20 } }
    responses:
      "200":
        description: Search hits with facet counts.
        content:
          application/json: { schema: { $ref: "#/components/schemas/SearchResult" } }
      "401": { $ref: "#/components/responses/Unauthorized" }
```

Other schemas: `RepoSummary` (`repo`,`default_branch`,`docs_dir`,
`last_synced_sha`), `RepoDetail` (adds `config_snapshot` as a free-form
`object`, `types: [DocType]`), `DocType`
(`name`,`dir`,`id_prefix`,`plural_label`,`statuses[]`,`aliases[]`),
`SearchResult` (`query`, `estimated_total_hits`, `hits: [SearchHit]`, `facets:
{string: {string: integer}}`), `SearchHit`
(`repo`,`doc_id`,`type`,`title`,`status`,`author`,`snippet`), `Session`
(`provider`,`subject`,`email?`,`login?`,`groups?`).

### Security schemes

docz-api has real auth, unlike rfc-api's spec. Proposed (see **OQ-5**):

```yaml
components:
  securitySchemes:
    sessionCookie: { type: apiKey, in: cookie, name: docz_session }
security:
  - sessionCookie: []          # default: all /api/v1 operations
```

`/auth/login`, `/auth/callback`, `/healthz`, `/readyz`, `/metrics` override with
`security: []` (public). `/webhooks/github` is HMAC-authenticated — modeled as
required headers (`X-Hub-Signature-256`, `X-GitHub-Event`) with a documented
signature scheme, since OpenAPI has no first-class HMAC-body scheme.

### The contract test (kin-openapi harness)

A hermetic Go test (placement is **OQ-6**) ports rfc-api's three-function
harness:

1. **`loadSpec`** — `openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")`
   → `doc.Validate(ctx)` (enforces OAS 3.1 strictness) → `gorillamux.NewRouter(doc)`.
2. **`buildHandler`** — construct the real `httpapi.Handler` via
   `NewHandlerWithSearch(store, searcher)` + `Handler.Mount`, fed the **same
   in-memory fakes the existing `contract_test.go` already uses** (fake
   `storeReader`, fake `Searcher`), plus a fake `authorize`/`session` context so
   the gate passes. No network, no `httptest.NewServer`.
3. **`validate`** — `router.FindRoute(req)` → `openapi3filter.ValidateRequest`
   → serve in-process → `openapi3filter.ValidateResponse` (both with
   `MultiError: true`), against the spec schemas. Table-drive the happy-path
   endpoints plus the 401/404/400 envelopes.

Because it drives the production `Mount`/handlers/serialization with fakes only
for I/O, a green test means "spec == reality." It rides the normal `go test
./...` run (no build tag) — the same way rfc-api's does.

### Serving and surfacing the spec

Per **OQ-4a**: the spec is a **repo artifact** the docz-site vendors, **and**
docz-api `//go:embed`s `api/openapi.yaml` and serves `GET /openapi.yaml` plus a
**Scalar** HTML page at `/docs` (a step beyond rfc-api, so browsing and consuming
are both live). Optionally it can also render in the docz **mkdocs wiki** via a
plugin (`mkdocs-swagger-ui-tag`). Serving is `//go:embed` + a static page — no
OpenAPI library at runtime.

### Dependency and CI

- New **direct** dependency `github.com/getkin/kin-openapi` (decision **OQ-8a**;
  rfc-api pins `v0.135.0`). Add via `go get`, settle `go.sum` per the repo
  convention (no bare `go mod tidy`). See **FU-3** for the libopenapi fast-follow.
- The contract test runs under the existing `just test` / CI `Test Go` job — no
  new workflow. Any spec edit must keep it green.

## API / Interface Changes

- **New file** `api/openapi.yaml` (the contract).
- **New dependency** `getkin/kin-openapi` — **test-path only** (serving the spec
  is `//go:embed` + a static Scalar page; no OpenAPI library at runtime).
- **No change** to existing endpoint request/response shapes (additive). Reshaping
  is deferred to FU-1 (errors) and FU-2 (list envelopes).
- **New public routes** (OQ-4a): `GET /openapi.yaml` (embedded spec) and `GET
  /docs` (Scalar UI).
- CLAUDE.md gains an "API contract" note; a new IMPL will track the build.

## Data Model

None. There are no database or storage changes. The OpenAPI "schemas" are the
existing wire DTOs (`internal/httpapi/dto.go`, `internal/search/types.go`);
nothing new is persisted.

## Testing Strategy

- **kin-openapi contract test** (above) — hermetic, in-memory fakes, rides `go
  test ./...`; validates request + response against the spec for every specced
  endpoint including error envelopes.
- **Spec self-validation** — `doc.Validate` in the test catches malformed specs
  and the OAS-3.1 gotchas: `info.summary` is rejected; `const: X` must be written
  `enum: [X]`. Run the contract test immediately after any spec edit.
- **Golden fixtures retire** (decision **OQ-7b**) — once the OpenAPI contract
  test reaches endpoint parity, `internal/httpapi/contract_test.go` +
  `testdata/contract/*.json` are removed so the spec is the single wire-contract
  owner (no double-maintenance). Both run side by side until parity; retiring the
  fixtures is the final step of the rollout.
- **Coverage of new fields/endpoints** — the invariant "any change to
  `api/openapi.yaml` (or a handler) must keep the contract test green" makes the
  test the forcing function for spec accuracy.

## Migration / Rollout Plan

Additive and reversible; no runtime behavior change in the baseline.

1. **Phase 1 — spec + contract test (read + search).** Author `api/openapi.yaml`
   for the five read endpoints + `/search` + the error envelope; add
   `kin-openapi`; port the harness; wire into CI. Exit: contract test green in CI.
2. **Phase 2 — auth + webhook + security schemes (OQ-3a/OQ-5a).** Add
   `/api/v1/auth/*`, `/webhooks/github`, and the `securitySchemes`. Once the
   contract test reaches endpoint parity, **retire the golden-fixture test**
   (`internal/httpapi/contract_test.go` + `testdata/contract/`) per **OQ-7b** so
   the spec is the single wire-contract owner. Exit: full current surface specced
   and validated; fixtures removed.
3. **Phase 3 — surface the spec (OQ-4a).** `//go:embed` + serve `GET
   /openapi.yaml` and add a **Scalar** page at `/docs`; optionally also render in
   the mkdocs wiki.
4. **docz-site coordination.** The docz-site vendors `api/openapi.yaml` and
   generates its client; version the spec (`info.version`) so consumers pin a
   known shape.

Rollback: delete `api/openapi.yaml` + the contract test + the dep; nothing else
depends on it in the baseline.

## Open Questions

**Resolved 2026-07-08.** Decisions are recorded inline (**→ Decision**); the
deferred items are tracked in [Follow-ups](#follow-ups) as a condition of
accepting this design. The original options are kept for context.

**OQ-1 — Error response envelope.** docz-api currently emits `{"error":"…"}`;
the fleet reference (rfc-api) uses RFC 7807 `application/problem+json`.

- **a (recommended):** Spec the existing `{"error": string}` envelope as-is.
  Keeps the design additive, respects the frozen golden fixtures, and treats
  error-shape convergence as separable future work.
- **b:** Adopt RFC 7807 `application/problem+json`
  (`type`/`title`/`status`/`detail`/`request_id`) to match rfc-api and surface
  the request ID docz-api's middleware already generates. Breaking: rewrites
  `writeError`, the middleware 401/500 bodies, and the golden fixtures.
- **other:** \_\_\_

**→ Decision: 1a.** Spec the existing `{"error": string}` envelope. Migrating to
RFC 7807 is logged as **FU-1** — to be investigated as a condition of accepting
this design.

**OQ-2 — List response shape.** Reads return envelope objects
(`{"repos":[…]}`, `{"types":[…]}`, `{"docs":[…]}`); rfc-api returns bare arrays
with pagination in headers.

- **a (recommended):** Keep the envelope objects. They're shipped and frozen,
  and give room to add sibling metadata later without another breaking change.
- **b:** Switch to bare JSON arrays + `X-Total-Count`/`Link` headers (rfc-api
  parity). Breaking to the wire and the fixtures.
- **other:** \_\_\_

**→ Decision: 2a** (for now). Keep the envelope objects to land the spec without
a breaking change. Converging on rfc-api's bare-array + header-pagination shape
is logged as **FU-2** — a likely-needed migration to schedule soon after.

**OQ-3 — Spec scope (first pass).** Which endpoints does the initial
`api/openapi.yaml` cover?

- **a (recommended):** The full current `/api/v1` surface **plus** auth +
  webhook — everything a consumer touches — in one spec, built across Phases 1–2.
- **b:** Read + `/search` only first (Phase 1); auth/webhook later. Smallest
  first step; the docz-site's primary need.
- **c:** Everything including `/healthz` `/readyz` `/metrics`.
- **other:** \_\_\_

**→ Decision: 3a.** Spec the full `/api/v1` surface plus auth + webhook across
Phases 1–2.

**OQ-4 — Serve/surface the spec at runtime.** rfc-api ships the spec as a repo
artifact only (not served).

- **a (recommended):** Repo artifact **plus** `//go:embed` + `GET /openapi.yaml`
  + a **Scalar** page at `/docs`. Low cost, and it makes the docz-site's
  consumption *and* human browsing live — the point of "a doc the docz-site can
  consume."
- **b:** Repo artifact only (strict rfc-api parity; docz-site vendors the file).
- **c:** Repo artifact + render in the mkdocs wiki (`mkdocs-swagger-ui-tag`),
  no runtime endpoint.
- **d:** Both b's endpoint *and* c's wiki render.
- **other:** \_\_\_

**→ Decision: 4a.** Ship the spec as a repo artifact **and** `//go:embed`-serve
it at `/openapi.yaml` with a Scalar page at `/docs`.

**OQ-5 — Model auth in the spec.** rfc-api models no security schemes.

- **a (recommended):** Model `sessionCookie` (`apiKey`/cookie/`docz_session`) as
  the default `security`, with public overrides and the webhook HMAC documented.
  docz-api genuinely has auth, and a typed client benefits from knowing it.
- **b:** Omit security schemes (rfc-api parity); document auth in prose only.
- **other:** \_\_\_

**→ Decision: 5a.** Model `sessionCookie` + the webhook HMAC in the spec. If it
complicates Phase 2 it can be descoped to a follow-up rather than blocking the
read + search spec.

**OQ-6 — Contract-test placement.** rfc-api uses a top-level `test/contract/`
package.

- **a (recommended):** `internal/httpapi/openapi_contract_test.go`, beside the
  existing `contract_test.go`. Matches docz-api's "tests live next to the code"
  rule and reuses the fakes already there.
- **b:** A new top-level `test/contract/` package (rfc-api parity), for a clear
  home if more cross-cutting contract tests arrive.
- **other:** \_\_\_

**→ Decision: 6a.** Place the harness at
`internal/httpapi/openapi_contract_test.go`, beside the existing test.

**OQ-7 — Existing golden-fixture test.** `internal/httpapi/contract_test.go`
freezes exact response bytes today.

- **a (recommended):** Keep both — golden fixtures for byte-level regressions,
  OpenAPI for the schema/param/status contract. They catch different failures.
- **b:** Retire the golden fixtures once the OpenAPI contract test covers the
  same endpoints, to avoid double-maintenance.
- **other:** \_\_\_

**→ Decision: 7b.** Retire `internal/httpapi/contract_test.go` +
`testdata/contract/*.json` once the OpenAPI contract test reaches endpoint
parity, so the spec is the single wire-contract owner (no double-maintenance).

**OQ-8 — OpenAPI library.** The validator that enforces the contract.

- **a (recommended):** `getkin/kin-openapi` (rfc-api pins `v0.135.0`). Fleet
  parity, proven harness, request+response validation out of the box.
- **b:** `pb33f/libopenapi` (+ `libopenapi-validator`) — more modern, stronger
  native OAS 3.1 support, but new to the fleet and a different API.
- **other:** \_\_\_

**→ Decision: 8a** (for now). Use `getkin/kin-openapi` for fleet parity with
rfc-api's proven harness — the fastest path to a green contract gate. Evaluating
`pb33f/libopenapi` (+ `libopenapi-validator`) is logged as **FU-3**: a
fast-follow spike, because its stronger native OAS 3.1 support and richer feature
set could serve our use case better than kin-openapi.

## Follow-ups

Deferred investigations logged as a **condition of accepting DESIGN-0002**. Each
becomes its own follow-up **INV** (and, if it lands a change, an IMPL) — they are
recorded here so the decisions above don't quietly become permanent. None blocks
shipping the baseline spec + contract test.

- **FU-1 — Error envelope → RFC 7807** (from **OQ-1**). Investigate migrating the
  `{"error": string}` envelope to `application/problem+json`
  (`type`/`title`/`status`/`detail`/`request_id`) for rfc-api parity, surfacing
  the request ID the middleware already generates. Breaking to `writeError`, the
  middleware 401/500 bodies, and any consumers — so it needs its own spike +
  rollout, not an inline change here.
- **FU-2 — List responses → rfc-api convention** (from **OQ-2**). Investigate
  moving the read envelopes (`{"repos":[…]}`, `{"types":[…]}`, `{"docs":[…]}`) to
  bare JSON arrays with pagination in headers (`X-Total-Count`/`Link`), matching
  rfc-api. Flagged as a **likely-needed** migration to schedule soon after the
  baseline lands, since fleet convergence is the goal.
- **FU-3 — Evaluate `pb33f/libopenapi` (+ `libopenapi-validator`)** (from
  **OQ-8**). Fast-follow spike comparing it against `getkin/kin-openapi` on our
  contract-test use case: native OAS 3.1 fidelity, request/response validation
  ergonomics, performance, and the migration cost off kin-openapi. If it wins,
  swap the harness; otherwise record why we stay on kin-openapi.

## References

- [INV-0002](../investigation/0002-auto-generate-an-openapi-contract-for-the-docz-site.md)
  — the spec-first investigation and rfc-api findings this design builds on
- [DESIGN-0001](../design/0001-docz-api-cross-repo-docz-registry-and-ingestion-service.md)
  — the docz-api read + search API shape
- `internal/httpapi/dto.go`, `internal/httpapi/search.go`,
  `internal/search/types.go` — the response DTOs the schemas mirror
- `api/openapi.yaml` + `internal/httpapi/openapi_contract_test.go` — the
  implemented contract and its kin-openapi test (the byte-frozen golden fixtures
  were retired at parity in IMPL-0002 Phase 2)
- rfc-api `api/openapi.yaml` + `test/contract/contract_test.go` — the reference
  spec and kin-openapi harness
- getkin/kin-openapi — <https://github.com/getkin/kin-openapi>
- OpenAPI Specification 3.1.0 — <https://spec.openapis.org/oas/v3.1.0>
- RFC 7807 (Problem Details for HTTP APIs) — <https://www.rfc-editor.org/rfc/rfc7807>
