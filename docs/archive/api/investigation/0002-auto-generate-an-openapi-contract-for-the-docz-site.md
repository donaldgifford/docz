---
id: INV-0002
title: "Auto-generate an OpenAPI contract for the docz-site"
status: Open
author: Donald Gifford
created: 2026-07-07
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0002: Auto-generate an OpenAPI contract for the docz-site

**Status:** Open
**Author:** Donald Gifford
**Date:** 2026-07-07

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [The rfc-api reference solution](#the-rfc-api-reference-solution)
  - [How it maps onto docz-api](#how-it-maps-onto-docz-api)
  - [Approach options](#approach-options)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

## Question

docz-api's HTTP API (`/api/v1`, chi) is currently contracted only by byte-frozen
golden fixtures (`internal/httpapi/contract_test.go` +
`testdata/contract/*.json`). How do we produce and maintain a **machine-readable
OpenAPI document** that the **docz-site** — and any other consumer — can treat
as the authoritative contract, and keep it from drifting from the running
server? Can we reuse the approach that **`rfc-api`** (our prior attempt at this
service) already shipped?

## Hypothesis

The right shape is **spec-first, not spec-generated-from-code**. `rfc-api`
already solved exactly this: a hand-authored `api/openapi.yaml` as the single
source of truth, kept honest by an in-process `kin-openapi` contract test on
every CI run, and consumed downstream by the site via *vendor-and-generate*.
Because docz-api is the same fleet and architecturally a near-twin (a chi HTTP
layer with response DTOs, a per-type + cross-type surface, and an existing
contract test), that pattern should port with minimal friction — plausibly the
**same implementation**.

The phrase "auto-generate an OpenAPI doc" resolves to three distinct automations,
only two of which are truly automatic: author the spec **once**, auto-**validate**
the server against it in CI, and let consumers auto-**generate** their client
from it — rather than reverse-engineering a spec from handler annotations.

## Context

- docz-api Phase 7 froze the read + search wire shape with golden JSON fixtures
  (`internal/httpapi/contract_test.go`). That pins exact bytes but is **not** a
  machine-readable, consumer-facing contract: it does not describe schemas,
  parameters, or status codes in a standard a frontend can codegen against.
- The **docz-site** needs a stable contract to build its client on. An OpenAPI
  doc is the fleet-standard way to provide one (`rfc-api` → `rfc-site`).
- `rfc-api` (`github.com/donaldgifford/rfc-api`) is the previous attempt at this
  same service and **already implemented** the OpenAPI contract. The ask is to
  reuse it as a reference or, if it fits, wholesale.

**Triggered by:** the docz-site integration need, DESIGN-0001 (the read API
shape), and the existing `internal/httpapi` golden-fixture contract test.

## Approach

1. **Study rfc-api's implementation.** (Done — see Findings.)
2. **Author `api/openapi.yaml`** (OAS 3.1) for docz-api's `/api/v1` surface — the
   five DESIGN-0001 read endpoints (repos, doc types, docs-by-type, doc-by-id) +
   `/search` + the error envelope; decide whether to include auth / webhook /
   health.
3. **Port the kin-openapi contract test** (`test/contract` or under
   `internal/httpapi`): load the spec, build a spec router, drive the real chi
   handler (`Handler.Mount`) in-process with in-memory fakes, and validate
   request + response via `openapi3filter`. Wire it into `just test` + CI so
   spec↔code drift fails the build.
4. **Decide consumption + surfacing:** ship `api/openapi.yaml` as a repo artifact
   for vendor-and-generate (baseline, matches `rfc-site`); optionally also
   `//go:embed` + serve it at `/openapi.yaml` with a Scalar/Redoc/Swagger-UI
   page, and/or render it in the docz mkdocs wiki via a plugin.
5. **Reconcile with the golden-fixture test** — keep both (byte lock + schema
   contract) or fold one into the other.
6. **Scope the build** as an IMPL (and a short DESIGN if the consume/serve model
   needs a recorded decision).

## Environment

| Component | Version / Value |
| --- | --- |
| docz-api HTTP layer | chi v5 (`internal/httpapi`), response DTOs, `authorize` seam |
| Current contract | golden byte fixtures (`internal/httpapi/contract_test.go`, `testdata/contract/*.json`) |
| Candidate library | `github.com/getkin/kin-openapi` (rfc-api pins `v0.135.0`) |
| Spec dialect | OpenAPI **3.1.0** |
| Reference spec | rfc-api `api/openapi.yaml` — 519 lines, ~13 paths, `servers: [{url: "/"}]` |
| Reference harness | rfc-api `test/contract/contract_test.go` (kin-openapi in-process validator) |
| docz-site consumption | vendor-and-generate (per `rfc-site`) |

## Findings

### The rfc-api reference solution

- **Source of truth — `api/openapi.yaml`:** hand-authored **OAS 3.1.0** (~519
  lines, ~13 paths), `servers: [{url: "/"}]` (same-origin), a per-type
  (`/api/v1/{type}/{id}`) surface plus a small cross-type aggregation
  (`/docs`, `/search`, `/types`). Adding a document type is a registry config
  change that does **not** alter the spec's shape. It is **not** generated from
  code.
- **Drift guard — `test/contract/contract_test.go`** (`kin-openapi v0.135.0`):
  - `openapi3.NewLoader().LoadFromFile("api/openapi.yaml")` then `doc.Validate()`
    (the spec is self-checked — no separate spectral/vacuum lint).
  - `gorillamux.NewRouter(doc)` — routing is derived **from the spec**, so it is
    independent of the server's own router (framework-agnostic).
  - builds the **real** handler in-process (`server.BuildMainHandler` with
    in-memory `store`/`search` fakes — not `httptest.NewServer`).
  - per endpoint: `openapi3filter.ValidateRequest` **and** `ValidateResponse`
    (with `MultiError: true`) against the spec's schemas — covering happy-path
    endpoints plus the 404 and 400 problem envelopes.
  - runs on **every CI/PR** — in their words, "how we keep the spec and the
    server's actual behavior in sync … the spec is hand-authored, so we need a
    live check that claims match reality."
- **Consumption:** the frontend **`rfc-site` consumes `api/openapi.yaml` via
  vendor-and-generate** (vendors the file, generates a typed client from it). The
  spec is the cross-repo contract. The mkdocs wiki itself uses `techdocs-core`
  and does **not** swagger-render the spec — the "site" consumption is the
  separate frontend repo.
- **Gotchas (rfc-api `CLAUDE.md`):** kin-openapi is strict about OAS 3.1 —
  `info.summary` is rejected ("extra sibling fields") and `const: X` must be
  written `enum: [X]`; run `go test ./test/contract/...` immediately after any
  spec edit. The spec is a **repo artifact**, not served at an HTTP endpoint.

### How it maps onto docz-api

- **chi is a non-issue.** kin-openapi routes off the spec via `gorillamux`, so
  chi-vs-`net/http` is irrelevant; the contract test drives docz-api's
  `httpapi.Handler.Mount` exactly as rfc-api drives `BuildMainHandler`.
- **Endpoints to spec:** the five DESIGN-0001 read endpoints + `GET
  /api/v1/search` + the error envelope. Auth (`/api/v1/auth/session`,
  `/logout`), `/webhooks/github`, and `/healthz` `/readyz` `/metrics` are
  candidates to include or scope out.
- **Authoring is mechanical.** `internal/httpapi` already returns purpose-built
  response **DTOs** (own structs mapping `pgtype` nullables to clean
  `string`/`YYYY-MM-DD`), so the spec schemas map ~1:1 to those DTOs.
- **Relationship to the golden-fixture test — complementary.** Golden fixtures
  freeze exact bytes (a regression lock); the OpenAPI + kin-openapi test asserts
  **schema** conformance and produces a **consumer-facing** artifact. Keep both,
  or fold the fixtures under the OpenAPI test.
- **A step beyond rfc-api (optional):** docz-api could `//go:embed` the spec and
  **serve** it at `/openapi.yaml` with a Scalar/Redoc page, and/or render it in
  the docz mkdocs wiki (`neoteroi-mkdocs` / `mkdocs-swagger-ui-tag`) so the
  docz-site shows live API docs rather than only vendoring the file.

### Approach options

| Option | What | Pros | Cons |
| --- | --- | --- | --- |
| **A. Spec-first + kin-openapi** (rfc-api) | hand-author `api/openapi.yaml`; CI contract test validates code ↔ spec | proven in-fleet; framework-agnostic; reviewable/diffable spec; consumer-ready artifact | spec authored by hand; kin-openapi's OAS 3.1 quirks |
| **B. Code-first generation** | annotate handlers (swaggo) or adopt a spec-emitting framework (Huma / ogen) and generate the spec | spec cannot drift (it is generated) | swaggo = fragile comment-soup; Huma/ogen = adopt a new HTTP framework or rewrite handlers — heavy for a working chi service |
| **C. Status quo** | golden byte fixtures only | already in place | not machine-readable, not consumer-facing, no schema/param contract |

## Conclusion

**Answer:** **Yes — reuse rfc-api's spec-first pattern (Option A).** The
"auto-generate an OpenAPI doc" goal is best met by hand-authoring
`api/openapi.yaml` as the contract and **auto-validating** the server against it
with a `kin-openapi` in-process contract test on every CI run, exactly as
rfc-api does; the docz-site then **auto-generates** its client from that spec
(vendor-and-generate, like `rfc-site`). docz-api is architecturally a near-twin —
chi + response DTOs + an existing contract test — so the implementation ports
with little friction, plausibly the same code.

This investigation stays **Open** only on the *build* decision — whether to also
serve/render the spec, and how to reconcile the golden fixtures — which should
land as a short DESIGN and/or an IMPL.

## Recommendation

- **Adopt Option A.** Port rfc-api's `api/openapi.yaml` shape and its
  `test/contract` kin-openapi harness to docz-api; add `github.com/getkin/kin-openapi`
  as a direct dep and wire the contract test into `just test` + CI.
- **Spec scope first pass:** the five DESIGN-0001 read endpoints + `/search` +
  the shared error envelope; add auth / webhook coverage in a later pass.
- **Decide consume + surface (short DESIGN):** repo artifact for
  vendor-and-generate is the baseline (matches `rfc-site`); **plus** optionally
  `//go:embed` + serve at `/openapi.yaml` + a Scalar page and/or a wiki render —
  a deliberate step beyond rfc-api that makes the docz-site show live API docs.
- **Keep the golden-fixture test** for byte-level regressions; let OpenAPI own
  the schema/parameter/status contract.
- **Mind the kin-openapi 3.1 quirks** (`info.summary`, `const` → `enum`) and run
  the contract test on every spec edit.
- **Follow-up docs:** an **IMPL** to build it, and a **DESIGN** if the
  serve/consume model warrants a recorded decision.

## References

- [DESIGN-0002](../design/0002-openapi-contract-for-docz-api-and-the-docz-site.md)
  — the detailed design derived from this investigation (with open questions)
- rfc-api `api/openapi.yaml` — hand-authored OAS 3.1 contract (the source of truth)
- rfc-api `test/contract/contract_test.go` — `kin-openapi` in-process request/response validator
- rfc-api `CLAUDE.md` — "the spec is hand-authored; `test/contract/` validates every handler against it via `kin-openapi` on every CI run"; OAS 3.1 gotchas
- `rfc-site` — consumes `api/openapi.yaml` via vendor-and-generate (the fleet consumption model)
- docz-api `internal/httpapi/contract_test.go` + `testdata/contract/` — current golden-fixture contract
- [DESIGN-0001](../design/0001-docz-api-cross-repo-docz-registry-and-ingestion-service.md) — the docz-api read + search API shape
- getkin/kin-openapi — <https://github.com/getkin/kin-openapi>
- OpenAPI Specification 3.1.0 — <https://spec.openapis.org/oas/v3.1.0>
