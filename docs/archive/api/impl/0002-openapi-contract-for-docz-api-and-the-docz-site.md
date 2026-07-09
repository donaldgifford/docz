---
id: IMPL-0002
title: "OpenAPI contract for docz-api and the docz-site"
status: Draft
author: Donald Gifford
created: 2026-07-08
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0002: OpenAPI contract for docz-api and the docz-site

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-07-08

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Spec foundation and the read and search contract test](#phase-1-spec-foundation-and-the-read-and-search-contract-test)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Full-surface spec (auth, webhook, security) and retiring golden fixtures](#phase-2-full-surface-spec-auth-webhook-security-and-retiring-golden-fixtures)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Serving and surfacing the spec](#phase-3-serving-and-surfacing-the-spec)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Consumer coordination and close-out](#phase-4-consumer-coordination-and-close-out)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. kin-openapi version pin?](#1-kin-openapi-version-pin)
  - [2. Where does the spec live, and how is it embedded?](#2-where-does-the-spec-live-and-how-is-it-embedded)
  - [3. Scalar delivery for the /docs page?](#3-scalar-delivery-for-the-docs-page)
  - [4. Driving the authed + HMAC endpoints in the hermetic test?](#4-driving-the-authed--hmac-endpoints-in-the-hermetic-test)
  - [5. info.version scheme for consumer pinning?](#5-infoversion-scheme-for-consumer-pinning)
  - [6. Does the spec document its own meta/serving routes?](#6-does-the-spec-document-its-own-metaserving-routes)
  - [7. Spec linting in CI beyond doc.Validate?](#7-spec-linting-in-ci-beyond-docvalidate)
- [References](#references)
<!--toc:end-->

## Objective

Give docz-api a hand-authored **`api/openapi.yaml`** (OpenAPI 3.1) that is the
machine-readable contract for its `/api/v1` HTTP surface, kept honest by a
**`kin-openapi` in-process contract test** that drives the real chi handler on
every CI run and validates request + response against the spec — so code and
spec can never silently drift. Serve the spec at `GET /openapi.yaml` (no bundled
UI — IMPL OQ-3d) and hand the **docz-site** a stable artifact to
vendor-and-generate a typed client from. This is the spec-first pattern proven
in `rfc-api` (INV-0002) and recorded in DESIGN-0002.

**Implements:** DESIGN-0002 (from INV-0002). This plan realizes DESIGN-0002's
resolved decisions: OQ-1a error envelope as-is, OQ-2a list envelopes as-is,
OQ-3a full `/api/v1` + auth + webhook, OQ-4a serve the spec, OQ-5a model auth,
OQ-6a harness beside the existing test, OQ-7b retire the golden fixtures at
parity, OQ-8a `getkin/kin-openapi`. This plan's own implementation OQs (resolved
2026-07-09) further narrow DESIGN-0002 OQ-4a to **serve `/openapi.yaml` only —
no Scalar UI** (IMPL OQ-3d) and add **spec lint + format** tooling (IMPL OQ-7b).
The deferred wire-reshaping work is FU-1 (RFC 7807 errors), FU-2 (bare-array
lists), and FU-3 (`pb33f/libopenapi`).

## Scope

### In Scope

- A single hand-authored **`api/openapi.yaml`** (OAS 3.1.0) covering the current
  `/api/v1` read + search surface, the auth endpoints, and the GitHub webhook
  receiver — documenting the wire shapes docz-api **already ships** (Phase 7's
  golden fixtures froze them; this is additive, non-breaking).
- A hermetic **`kin-openapi` contract test** at
  `internal/httpapi/openapi_contract_test.go` (OQ-6a) that loads the spec,
  derives a router from it, drives the production handler in-process with the
  in-memory fakes already in the package, and validates request **and** response
  via `openapi3filter` — riding the normal `go test ./...` / CI `Test Go` job.
- **Serving** the spec: `//go:embed` the file (`api.Spec`) and expose `GET
  /openapi.yaml`, public — the machine contract only, **no bundled UI/Scalar
  page** (OQ-4a serve, narrowed by IMPL OQ-3d).
- **Retiring** the golden-fixture contract (`internal/httpapi/contract_test.go`
  + `testdata/contract/*.json`) once the OpenAPI test reaches endpoint parity
  (OQ-7b), so the spec is the single wire-contract owner.
- The new **`getkin/kin-openapi`** dependency (test-path only) and the CLAUDE.md
  "API contract" note.

### Out of Scope

- **Reshaping the wire** to match rfc-api conventions — RFC 7807 errors (**FU-1**)
  and bare-array list responses with header pagination (**FU-2**). This plan
  documents the surface **as it is**; convergence is deferred to those follow-up
  investigations.
- **Building the docz-site client** — that lives in the docz-site repo
  (vendor-and-generate). This repo's only job is keeping `api/openapi.yaml`
  accurate and served.
- **Evaluating `pb33f/libopenapi` (+ `libopenapi-validator`)** — the fast-follow
  library spike (**FU-3**); this plan ships on `kin-openapi` for fleet parity.
- **Code-first generation** (swaggo) or a spec-emitting framework (Huma / ogen) —
  ruled out by INV-0002.
- Auth/authorization redesign — the `authorize` seam and session model are
  unchanged; the spec merely **describes** them.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met. Phase 1 delivers the
docz-site's primary need (the read + search contract, validated). Phase 2
completes the surface and collapses the two contract tests into one. Phase 3
makes the spec live at runtime. Phase 4 versions it and hands it to consumers.

---

### Phase 1: Spec foundation and the read and search contract test

Stand up the artifact and its drift guard for the endpoints the docz-site
consumes first — the five DESIGN-0001 read endpoints plus `/search` and the
error envelope. This is the whole spec-first machine end to end on the smallest
useful surface; later phases only widen it.

#### Tasks

- [x] Add **`github.com/getkin/kin-openapi@v0.135.0`** as a direct dependency
      (**OQ-1a**, rfc-api parity): `go get github.com/getkin/kin-openapi@v0.135.0`,
      then settle `go.sum` per the repo convention (**no bare `go mod tidy`** —
      deps are staged; settle via `go get` / targeted `GOPROXY=off go get`), run
      `go mod edit -fmt`, and confirm `go mod verify`. The dep is **test-path
      only** (runtime serving is `//go:embed`, not a library). _(done — added at
      `v0.135.0`; `go mod edit -fmt`; `go mod verify` clean. Staged `// indirect`
      per the repo convention until the contract test imports it in Task 6, then
      it promotes to direct; `go build ./...` + `just lint` green.)_
- [x] Create the **`api` package** (`api/spec.go`, `package api`) with
      `//go:embed openapi.yaml` exposing `var Spec []byte` (**OQ-2a**) — the
      single embedded copy consumed by both the runtime server (Phase 3) and the
      contract test (`LoadFromData(api.Spec)`), so served bytes provably equal
      tested bytes and there are no relative `../../` load paths. _(done —
      go-architect confirmed the root-level `api/` home + leaf-package shape;
      `var Spec []byte` (not `embed.FS`/func) is the idiomatic single-file embed.
      A minimal valid `openapi.yaml` skeleton lands in the same commit so the
      embed compiles; Task 3+ expand it. `go build ./...` + `go vet` + `just
      lint` green.)_
- [x] Create **`api/openapi.yaml`** header: `openapi: 3.1.0`; `info` with
      `title` / `version` / `description` **only** (NOT `info.summary` —
      kin-openapi rejects it); `servers: [{ url: "/" }]` (same-origin, so the
      test router resolves `http://localhost/api/v1/...`); `tags`
      (`repos`, `docs`, `search`); `components.securitySchemes.sessionCookie`
      (`type: apiKey`, `in: cookie`, `name: docz_session`) with a top-level
      `security: [{ sessionCookie: [] }]` default. _(done — header + servers +
      3 tags + `sessionCookie` scheme + default `security`; no `info.summary`.
      YAML syntax-checked; full OAS `doc.Validate` runs with the harness in
      Task 6.)_
- [x] Author `components.schemas` mirroring the DTOs
      (`internal/httpapi/dto.go`, `internal/search/types.go`) **1:1**: `Error`
      (`required: [error]`, `additionalProperties: false`), `RepoSummary`
      (`repo`, `default_branch`, `docs_dir`, `last_synced_sha`), `RepoDetail`
      (adds `config_snapshot` as a free-form `object`, `types: [DocType]`),
      `DocType` (`name`, `dir`, `id_prefix`, `plural_label`, `statuses[]`,
      `aliases[]`), `Document` (all read fields; `raw_md` **optional** — present
      only on `getDoc`), `SearchHit`
      (`repo`, `doc_id`, `type`, `title`, `status`, `author`, `snippet`),
      `SearchResult` (`query`, `estimated_total_hits`, `hits: [SearchHit]`,
      `facets`). Reflect the serialization invariants: nullable columns are
      **empty strings, never `null`**; JSONB string arrays are **`[]`, never
      `null`**; `facets` is `object` with `additionalProperties` an object of
      `additionalProperties: { type: integer }`. _(done — all 7 schemas authored
      with `additionalProperties: false` for strict drift detection, verified
      1:1 against the golden fixtures; every response field is `required` except
      `Document.raw_md`. Proven to pass kin-openapi `doc.Validate` via a
      throwaway loader test, removed before commit — the real harness in Task 6
      revalidates in CI.)_
- [x] Author `components.responses` (`Unauthorized` → 401 `Error`, `NotFound` →
      404 `Error`) and the six paths with lowerCamelCase `operationId`s:
      `listRepos` (`{repos:[…]}`), `getRepo` (`RepoDetail`; 404), `listTypes`
      (`{types:[…]}`), `listDocs` (`{docs:[…]}`, no `raw_md`; 404), `getDoc`
      (`Document` **with** `raw_md`; 404), `searchDocs` (`SearchResult`; query
      params `q`/`repo`/`type`/`status`/`author` + `offset` default 0 / `limit`
      default 20, both `integer` `format: int64` `minimum: 0`). Path params:
      `owner`, `name`, `type`, `doc_id`. **List responses stay envelope
      objects** (OQ-2a); **error stays `{"error": string}`** (OQ-1a). _(done —
      shared `Owner`/`Name`/`Type`/`DocID` path parameters + `Unauthorized`/
      `NotFound` responses; the same `Document` schema serves `listDocs` (raw_md
      absent) and `getDoc` (raw_md present) since raw_md is optional. Proven:
      `doc.Validate` passes and `gorillamux.FindRoute` resolves all six paths.)_
- [x] Port rfc-api's three-function harness to
      **`internal/httpapi/openapi_contract_test.go`** (OQ-6a): `loadSpec`
      (`openapi3.NewLoader().LoadFromData(api.Spec)` per **OQ-2a** →
      `doc.Validate(ctx)` → `gorillamux.NewRouter(doc)`); `buildHandler`
      **reusing the fakes already in the package** — `seededStore()` +
      `contractSearcher{}` + `NewHandlerWithSearch(st, …).Mount(r,
      authorize.Middleware(authorize.NewAllReposAuthorizer(st)))`; `validate`
      (`router.FindRoute` → `openapi3filter.ValidateRequest` → serve in-process
      → `openapi3filter.ValidateResponse`, both `Options{MultiError: true}`). No
      build tag, no `httptest.NewServer`. _(done — `loadContractSpec` /
      `buildContractHandler` / `validateRoundTrip`. Security is validated with
      `openapi3filter.NoopAuthenticationFunc` so the spec's `sessionCookie`
      requirement doesn't fail request validation — the contract test asserts
      schemas, auth is covered by its own tests. `getkin/kin-openapi` promoted to
      a direct dep. go-style reviewed; passes `just lint`.)_
- [x] Table-drive the cases: the six read/search happy paths (`/api/v1/repos`,
      `/repos/acme/platform`, `/types`, `/types/frameworks/docs`,
      `/types/FW/docs/FW-0001`, `/search?q=intro`) plus the **404** envelope
      (`/repos/acme/missing`). Confirm the search fixture exercises `facets` and
      the `snippet`. _(done — 7 subtests, all green; the `searchDocs` case
      exercises the `facets` map and the `<em>` `snippet` via `contractSearcher`.)_
- [x] Add **spec lint + format** so the hand-authored file is standards-clean
      from its first commit (**OQ-7b**): wire an OpenAPI linter — **`vacuum`**
      (Go-native, `mise`-installable, pb33f/`libopenapi` lineage per **FU-3**;
      `spectral` the alternative) — over `api/openapi.yaml` via a `just`
      recipe (e.g. `just lint-openapi`) **and** a CI step, plus a YAML
      format/consistency pass (`yamlfmt` or `prettier`) hooked into `just fmt` /
      the fmt check. Pin the tool in `mise.toml`. _(done — `vacuum@0.29.9`
      pinned in `mise.toml`; `api/vacuum-ruleset.yaml` extends the recommended
      set and disables three rules with justifications (`camel-case-properties`
      because the wire contract is snake_case by design;
      `oas3-missing-example` / `description-duplication` as over-strict for a
      DTO-mirroring spec). Added schema + parameter descriptions → **100/100**.
      `just lint-openapi` runs vacuum (`-n warn`, fails on warnings) +
      `yamlfmt -lint`; `just fmt` now yamlfmt-canonicalizes the spec; CI's Lint
      job gained a mise step + `just lint-openapi`.)_
- [ ] Run `just test`, `just lint`, `just lint-openapi`, `just fmt`; confirm the
      contract test runs in the CI `Test Go` job with no new workflow. Check off
      and commit (`feat(openapi): ...`).

#### Success Criteria

- `api/openapi.yaml` **self-validates**: `doc.Validate` passes and kin-openapi
  loads it with no OAS-3.1 errors (no `info.summary`, no bare `const`).
- The contract test validates **request and response** against the spec for all
  six read/search endpoints **and** the 404 envelope, green under `just test`
  and in CI's `Test Go` job.
- `getkin/kin-openapi` is pinned at **`v0.135.0`**, `go.sum` settled without a
  bare `go mod tidy`; `go mod verify` clean; `just lint` green.
- The spec passes the **OpenAPI linter** (`vacuum`/`spectral`) and the YAML
  formatter in CI — standards-clean and canonical, not just structurally valid.
- **Drift is caught:** changing a DTO field (name/type) without updating the
  spec — or vice-versa — fails the contract test. Demonstrate once (temporarily,
  reverted) to prove the guard bites.

---

### Phase 2: Full-surface spec (auth, webhook, security) and retiring golden fixtures

Widen the spec to the rest of the surface a consumer touches — the session auth
endpoints, the public OAuth redirects, and the HMAC webhook receiver — model the
security schemes, then collapse the two contract tests into one by retiring the
byte-frozen golden fixtures (OQ-7b).

#### Tasks

- [ ] Add the auth paths: `GET /api/v1/auth/session` (`getSession` →
      `Session` schema, or 401 via `Unauthorized`); `POST /api/v1/auth/logout`
      (`logout` → `{ "status": "logged out" }` object); `GET /auth/login`
      (`login` → **302** with `Location`, `provider` query param,
      `security: []`); `GET /auth/callback` (`authCallback` → **302**,
      `security: []`). Add the `Session` schema
      (`provider`, `subject`, `email?`, `login?`, `groups?`) mirroring
      `internal/authhttp/handler.go`.
- [ ] Add the webhook path `POST /webhooks/github` (`githubWebhook`): required
      headers `X-Hub-Signature-256`, `X-GitHub-Event`, `X-GitHub-Delivery`;
      request body `application/json` (opaque `object` — the payload is
      GitHub's, validated by `go-github` at runtime, not the spec); responses
      **202** / **400** / **401**; `security: []`. Document the HMAC scheme in
      the operation `description` (OpenAPI has no first-class HMAC-body scheme).
- [ ] Apply `security` overrides across the spec: the `sessionCookie` default
      holds for `/api/v1/*`; the public routes (`/auth/login`, `/auth/callback`,
      `/webhooks/github`, and any meta routes per OQ-6) override with
      `security: []`.
- [ ] **Acceptance gate (OQ-4a):** before implementing, review rfc-api's
      `test/contract/contract_test.go` and mirror its harness style. Note (already
      verified) rfc-api models **no** auth/HMAC and tests only happy-path + 404 +
      400 — so the auth/webhook driving is **net-new** here; the review confirms
      we don't diverge from the shared harness shape, not that we copy a pattern
      it lacks.
- [ ] Extend the contract harness to drive the new endpoints hermetically
      (**OQ-4a**): inject a `session.Session` into the request context for
      `getSession`; build `authhttp.New(...)` with fake `sessionStore` /
      `userUpserter` and a **stub provider registry** so `login` asserts a 302;
      compute a **valid HMAC** over a fixture webhook body with a test secret and
      a fake delivery store so `githubWebhook` validates its required headers and
      returns 202. Add these to the validation table.
- [ ] Reach **endpoint parity** with the golden-fixture test, then **retire it
      (OQ-7b):** delete `internal/httpapi/contract_test.go` and
      `internal/httpapi/testdata/contract/*.json`. The OpenAPI contract test is
      now the single wire-contract owner. Keep the shared fakes (`seededStore`,
      `contractSearcher`) — they move to / stay in the OpenAPI test's file or a
      small shared test helper.
- [ ] `just test` / `just lint` / `just fmt` green; CI green. Check off and
      commit (`refactor(openapi): retire golden fixtures at parity`).

#### Success Criteria

- Every endpoint in DESIGN-0002's surface table that is in scope (per OQ-6) is
  specced and exercised by the contract test: read + search + `auth/session` +
  `auth/logout` + `auth/login` + `auth/callback` + `webhooks/github`.
- The webhook operation validates its **required headers** against the spec; the
  auth endpoints validate their response schemas; the OAuth redirects assert
  **302**.
- `internal/httpapi/contract_test.go` and `testdata/contract/` are **removed**;
  no wire shape is double-maintained; `just test` is green with the OpenAPI test
  as the **sole** contract gate.

---

### Phase 3: Serving and surfacing the spec

Make the contract live at runtime: serve the embedded spec so the docz-site can
fetch it at a pinned version (as well as vendoring the file). **No Scalar / UI
page** (OQ-3d) — the machine contract is `/openapi.yaml`; human browsing is left
to the optional docz mkdocs render (Phase 4) or the consumer's own tooling. The
`api` package embed already exists from Phase 1 (OQ-2a).

#### Tasks

- [ ] Add `GET /openapi.yaml`: serve `api.Spec` (the Phase-1 embed) with
      `Content-Type: application/yaml`, **public** (outside the `/api/v1` auth
      gate), mounted in `newRouter` / `runServer` alongside the `/healthz`,
      `/readyz`, `/metrics` probes in `cmd/docz-api/main.go`. **No `/docs`
      route** (OQ-3d).
- [ ] Tests: `GET /openapi.yaml` returns 200 + the embedded bytes + the right
      content-type, and the **served bytes re-parse and `doc.Validate`** (so the
      embedded copy provably equals the source `api/openapi.yaml` and can't
      drift); the route **bypasses** the auth gate (no session → still 200).
- [ ] `just test` / `just lint` / `just fmt` green. Check off and commit
      (`feat(openapi): embed and serve the spec`).

#### Success Criteria

- `GET /openapi.yaml` serves the embedded, self-validating spec with **no
  filesystem dependency** — it works in the distroless image (no shell, no
  mounted file).
- The route is **public** (no session required), consistent with the spec's
  `security: []` overrides; **no `/docs` UI route exists** (OQ-3d).
- A test proves the **served spec byte-equals `api/openapi.yaml`** and validates,
  closing the embed-drift gap.

---

### Phase 4: Consumer coordination and close-out

Version the contract, document how the docz-site consumes it, and record the
work — flipping DESIGN-0002 to Implemented and logging the three follow-ups.

#### Tasks

- [ ] Set `info.version` (scheme per **OQ-5**) and document the bump discipline:
      a wire change to any specced endpoint bumps the version so consumers can
      pin a known shape.
- [ ] Document the **docz-site consumption path**: vendor `api/openapi.yaml` (or
      fetch `GET /openapi.yaml` at a pinned version) and generate a typed client
      — a short note under `api/README.md` or the design, matching the `rfc-site`
      model.
- [ ] Update **`CLAUDE.md`**: an "API contract" note (spec at `api/openapi.yaml`;
      contract test at `internal/httpapi/openapi_contract_test.go` on every CI
      run; spec lint via `vacuum`; golden fixtures **retired**; the spec is served
      at `/openapi.yaml`) and IMPL-0002 phase-progress lines.
- [ ] (Optional) render the spec in the docz **mkdocs wiki** via
      `mkdocs-swagger-ui-tag` — the only human-browsable API reference, since
      there is no in-app `/docs` page (OQ-3d).
- [ ] Flip **DESIGN-0002 → Implemented**; open the follow-up INVs for **FU-1**
      (RFC 7807 errors), **FU-2** (bare-array lists + header pagination), and
      **FU-3** (`pb33f/libopenapi` evaluation) — logged, not built here.
- [ ] Final `just lint` / `just test` / `just fmt` green; commit
      (`docs(openapi): version, document consumption, close out`).

#### Success Criteria

- The spec carries a consumer-pinnable `info.version` and a documented bump rule.
- The docz-site has a **documented, stable consumption path** (vendor the file /
  fetch the served spec → generate a typed client).
- `CLAUDE.md` documents the contract and its CI gate; DESIGN-0002 is marked
  **Implemented**; FU-1 / FU-2 / FU-3 are logged as follow-up investigations.

---

## File Changes

| File / package | Action | Description |
| -------------- | ------ | ----------- |
| `api/openapi.yaml` | Create | The hand-authored OAS 3.1 contract (source of truth). |
| `api/spec.go` | Create | `package api` with `//go:embed openapi.yaml` → `var Spec []byte`, shared by runtime + test (**OQ-2a**). |
| `internal/httpapi/openapi_contract_test.go` | Create | kin-openapi in-process request/response validator (OQ-6a); loads `api.Spec`; reuses `seededStore` / `contractSearcher`. |
| `cmd/docz-api/main.go` | Modify | Serve `GET /openapi.yaml` (from `api.Spec`) in `newRouter` / `runServer`, public, outside the auth gate (**OQ-3d** — no `/docs`). |
| `internal/httpapi/contract_test.go` | Delete | Retired at Phase 2 parity (OQ-7b). |
| `internal/httpapi/testdata/contract/*.json` | Delete | Golden fixtures retired with the test. |
| `go.mod` / `go.sum` | Modify | Add `getkin/kin-openapi@v0.135.0` (test-path direct dep; settle via `go get`, no bare tidy). |
| `justfile` / `.github/workflows/` / `mise.toml` | Modify | `just lint-openapi` + a CI step (`vacuum`); pin the linter + `yamlfmt` in `mise.toml` (**OQ-7b**). |
| `CLAUDE.md` | Modify | "API contract" note + IMPL-0002 phase progress. |
| `docs/design/0002-*.md` | Modify | Status → Implemented at close-out. |
| `api/README.md` (optional) | Create | docz-site vendor-and-generate note. |

## Testing Plan

- [ ] **kin-openapi contract test** — request + response validation for the full
      specced surface, hermetic (in-memory fakes), on the normal `go test ./...`
      run with **no build tag** (rides CI's `Test Go` job; no new workflow).
- [ ] **Spec self-validation** — `doc.Validate` in the test catches malformed
      specs and the OAS-3.1 gotchas (`info.summary` rejected; `const: X` must be
      `enum: [X]`). Run the contract test immediately after any spec edit.
- [ ] **Drift detection** — a DTO change without a matching spec change (or the
      reverse) fails the contract test; proven once during Phase 1.
- [ ] **Spec lint** — `vacuum` (or `spectral`) passes over `api/openapi.yaml` in
      CI, on top of `doc.Validate`'s structural check (Phase 1, **OQ-7b**).
- [ ] **Authed + HMAC endpoints** — session injected into context; a valid HMAC
      computed over a fixture webhook body; a stub provider registry for the
      OAuth redirects (Phase 2, **OQ-4a**).
- [ ] **Serve test** — `GET /openapi.yaml` served bytes re-validate and
      byte-equal the source; the route is public (Phase 3). No `/docs` route
      (OQ-3d).
- [ ] No new **integration** dependencies — the whole plan is hermetic; nothing
      here needs Postgres / Redis / Meilisearch / testcontainers.

## Dependencies

- **`github.com/getkin/kin-openapi@v0.135.0`** (OQ-8a; version **OQ-1a**, rfc-api
  parity) — the spec loader + `openapi3filter` request/response validator +
  `gorillamux` spec router. **Test-path only** — runtime serving is `//go:embed`
  (`api.Spec`), no OpenAPI library in the binary.
- **`vacuum`** (OpenAPI linter, **OQ-7b**; `spectral` the alternative) + a YAML
  formatter (`yamlfmt` / `prettier`) — new toolchain entries pinned in
  `mise.toml`, wired into `just` + CI so the hand-authored spec stays
  standards-clean.
- **Existing `internal/httpapi` test fakes** — `seededStore()`,
  `contractSearcher{}`, `authorize.Middleware` / `NewAllReposAuthorizer` — reused
  by the new harness (no new test scaffolding for the read/search paths).
- **No UI dependency** — `/docs`/Scalar dropped (**OQ-3d**); only the raw
  `/openapi.yaml` is served.
- **Deferred (not this plan):** `pb33f/libopenapi` (+ `libopenapi-validator`) —
  the FU-3 fast-follow evaluation (shares lineage with the chosen `vacuum`
  linter).
- **Existing repo tooling** — `mise`, `just`, `golangci-lint`, the GitHub
  Actions `Test Go` job (already present; the contract test rides it).

## Open Questions

**Resolved 2026-07-09.** Decisions are recorded inline (**→ Decision**) and folded
into the phases above; the original options are kept for context. Each question is
numbered; option `a` was the recommendation, later letters alternatives, **Other**
free-form. These are **implementation** choices not already fixed by DESIGN-0002's
resolved OQs.

### 1. kin-openapi version pin?

Which `getkin/kin-openapi` version to pin (DESIGN-0002 chose the library, not
the version).

- **a (recommended):** Pin **`v0.135.0`** — exact fleet parity with rfc-api's
  proven harness, so the ported code compiles unchanged; let Renovate propose
  bumps later.
- **b:** Pin the **latest** released version — newest OAS-3.1 fixes, but may
  differ from rfc-api's API and need small harness tweaks.
- **c:** Pin the latest **`v0.135.x`** patch — rfc-api's minor line plus bug
  fixes, minimal API risk.
- Other.

**→ Decision: 1a.** Pin `v0.135.0` for exact fleet parity with rfc-api's harness;
Renovate proposes bumps later.

### 2. Where does the spec live, and how is it embedded?

`api/openapi.yaml` sits at the repo root, outside `internal/`. Go's `//go:embed`
**cannot reach across or above a package directory**, so the contract test (in
`internal/httpapi`, OQ-6a) and the runtime server (in `cmd/docz-api`) can't both
`//go:embed ../../api/openapi.yaml`. This needs a single, un-duplicated strategy.

- **a (recommended):** Add a tiny **`api` package** (`api/spec.go`, `package
  api`) with `//go:embed openapi.yaml` exposing `var Spec []byte`. The runtime
  server serves `api.Spec`; the contract test loads it via
  `openapi3.NewLoader().LoadFromData(api.Spec)`. One embedded copy, one import,
  no relative-path fragility, and the served bytes provably equal the tested
  bytes.
- **b:** Contract test uses **`LoadFromFile("../../api/openapi.yaml")`** (rfc-api
  parity — its test is at `test/contract/`), and the runtime server embeds via a
  **separate** `api` package. Two load paths; the test reads the source file, the
  server reads the embed — a subtle drift seam the Phase 3 byte-equal test must
  cover.
- **c:** Relocate the spec **under `internal/httpapi/`** and `//go:embed` it
  there. Simplest embed, but breaks the `api/openapi.yaml` fleet-convention
  layout the docz-site expects to vendor.
- Other.

**→ Decision: 2a.** Add an `api` package (`api/spec.go`, `package api`) with
`//go:embed openapi.yaml` exposing `var Spec []byte`. The runtime server serves
`api.Spec`; the contract test loads it via `LoadFromData(api.Spec)`. One embedded
copy, one import, no relative paths — served bytes provably equal tested bytes.

### 3. Scalar delivery for the /docs page?

How to serve the Scalar UI at `GET /docs` (the spec itself is served separately
at `/openapi.yaml`).

- **a (recommended):** A single **static HTML page referencing Scalar's CDN**
  (`@scalar/api-reference` via jsDelivr) pointed at `/openapi.yaml`. Zero build,
  a few lines, trivial to embed — the page needs outbound CDN at **view** time,
  which is fine for a browsable docs page (the machine contract is `/openapi.yaml`,
  which is fully self-contained).
- **b:** **Vendor + embed** the Scalar JS bundle so `/docs` is fully offline /
  air-gapped. Self-contained, but adds a vendored front-end asset and an update
  burden.
- **c:** Use **Redoc** or **Swagger-UI** instead (same CDN-vs-vendor trade-off).
- **d:** **Drop `/docs`**; serve only `/openapi.yaml` and rely on the docz mkdocs
  wiki render (OQ-6 / Phase 4) for human browsing.
- Other.

**→ Decision: 3d.** **No Scalar page.** Serve only `GET /openapi.yaml` (the
machine contract, fully self-contained). Human browsing is left to the optional
docz mkdocs wiki render (Phase 4) or the consumer's own tooling — no bundled UI,
no CDN dependency. Phase 3 drops the `/docs` route entirely.

### 4. Driving the authed + HMAC endpoints in the hermetic test?

Phase 2 must validate `/api/v1/auth/*` (session-gated), the OAuth redirects, and
`/webhooks/github` (HMAC). How far does the hermetic test go?

- **a (recommended):** **Full request + response validation with purpose-built
  fakes** — inject a `session.Session` into context for `getSession`; a stub
  provider registry + fake session/user stores for the redirects (assert 302);
  a **computed valid HMAC** over a fixture body + fake delivery store for the
  webhook (validate the required headers, assert 202). Highest fidelity —
  "spec == reality" for the whole surface.
- **b:** **Response-only** validation for the webhook (skip `ValidateRequest`, so
  no need to model/verify the HMAC headers precisely) while keeping full
  validation for auth.
- **c:** **Document-only** for webhook + redirects — spec them but exclude from
  the live test (only read/search/session are validated), deferring the harder
  fakes.
- Other.

**→ Decision: 4a**, with an acceptance gate: **study rfc-api's
`test/contract/contract_test.go` first** and mirror its harness patterns before we
accept the Phase 2 implementation. **Caveat (verified):** rfc-api's contract test
models **no** security schemes and drives only happy-path + 404 + 400 envelopes —
it has **no** session-auth or HMAC-webhook cases. So rfc-api is the reference for
the *harness shape* (`loadSpec` / `buildHandler` / `validate`, `MultiError`, the
`gorillamux` router), but **driving the authed + HMAC endpoints is net-new** to
docz-api. Phase 2 must design those fakes ourselves (injected session context,
computed HMAC over a fixture body, stub provider registry) — the rfc-api review is
to confirm we're not diverging from the shared harness style, not to copy an
auth/HMAC pattern it doesn't have.

### 5. info.version scheme for consumer pinning?

The docz-site pins a known spec shape via `info.version`; what scheme?

- **a (recommended):** **SemVer starting `1.0.0`**, bumped manually on any wire
  change (minor for additive, major for breaking). Standard OpenAPI practice;
  clear signal to consumers; independent of the binary's release version.
- **b:** **Date-based** (`YYYY-MM-DD`) — simple, monotonic, but conveys no
  compatibility semantics.
- **c:** **Mirror the binary version** (`main.version` / git tag) — one number to
  reason about, but couples the contract's cadence to release tags even when the
  wire is unchanged.
- Other.

**→ Decision: 5a.** SemVer starting `1.0.0`, bumped manually on any specced wire
change (minor = additive, major = breaking), independent of the binary's release
version.

### 6. Does the spec document its own meta/serving routes?

DESIGN-0002 OQ-3a scoped the spec to `/api/v1` + auth + webhook — it did **not**
include the operational routes. Does the new `GET /openapi.yaml` (and the
existing `/healthz` / `/readyz` / `/metrics`) appear in the spec?

- **a (recommended):** **No** — keep the spec to the consumer-facing surface
  (`/api/v1` + auth + webhook, per OQ-3a). `/openapi.yaml` and the health/metrics
  probes are operational/meta and stay out; the contract test never asserts them.
- **b:** **Include `/openapi.yaml`** (self-referential, so a client can discover
  it) but still exclude health/metrics.
- **c:** **Include everything**, health and metrics included (OQ-3c's fuller
  scope), for a complete operational description.
- Other.

**→ Decision: 6a.** The spec stays the consumer-facing surface (`/api/v1` + auth +
webhook). `/openapi.yaml` and the health/metrics probes are operational and stay
out; the contract test never asserts them.

### 7. Spec linting in CI beyond doc.Validate?

`doc.Validate` in the contract test is the only structural check rfc-api runs. Do
we add a dedicated OpenAPI linter?

- **a (recommended):** **No** — `doc.Validate` (OAS-3.1 structural correctness)
  in the contract test is sufficient and matches rfc-api; no new CI tooling or
  Node dependency.
- **b:** Add **`spectral`** (or `vacuum`) with a ruleset for style/consistency
  (operationId casing, description presence, example coverage) as a CI step.
- **c:** Add **`redocly lint`** — similar linting, tied to the Redoc toolchain.
- Other.

**→ Decision: 7b.** A hand-authored spec still gets **lint + format** so downstream
tooling that expects standards can consume it cleanly. Add:

- an **OpenAPI linter** — **`vacuum`** recommended (Go-native, `mise`-installable,
  no Node runtime, and shares the pb33f/`libopenapi` lineage the **FU-3** spike
  targets), with **`spectral`** as the alternative — run over `api/openapi.yaml`
  in CI and a `just` recipe (e.g. `just lint-openapi`), enforcing consistency
  rules (operationId casing, description presence, response coverage);
- a **format/consistency** pass so the YAML stays canonical — `yamlfmt` (already
  in the fleet toolchain) or `prettier`, wired into `just fmt` / the fmt check.

`doc.Validate` in the contract test still guards structural OAS-3.1 correctness;
the linter adds style/consistency on top. This is a **Phase 1 task** so the spec
is standards-clean from its first commit.

## References

- [DESIGN-0002](../design/0002-openapi-contract-for-docz-api-and-the-docz-site.md)
  — the design this plan implements (resolved OQs + FU-1/FU-2/FU-3)
- [INV-0002](../investigation/0002-auto-generate-an-openapi-contract-for-the-docz-site.md)
  — the spec-first investigation and rfc-api findings
- [DESIGN-0001](../design/0001-docz-api-cross-repo-docz-registry-and-ingestion-service.md)
  — the docz-api read + search API shape
- `internal/httpapi/dto.go`, `internal/httpapi/search.go`,
  `internal/search/types.go` — the response DTOs the schemas mirror
- `internal/httpapi/handler.go` (`Mount`, `NewHandlerWithSearch`) +
  `internal/httpapi/handler_test.go` (`seededStore`) +
  `internal/httpapi/contract_test.go` (`contractSearcher`) — the wiring + fakes
  the harness reuses
- `cmd/docz-api/main.go` (`newRouter`, `runServer`) — where the public spec
  routes mount
- rfc-api `api/openapi.yaml` + `test/contract/contract_test.go` — the reference
  spec and kin-openapi harness
- getkin/kin-openapi — <https://github.com/getkin/kin-openapi>
- OpenAPI Specification 3.1.0 — <https://spec.openapis.org/oas/v3.1.0>
- Scalar API Reference — <https://github.com/scalar/scalar>
