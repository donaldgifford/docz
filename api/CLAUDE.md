# CLAUDE.md: `api/`

Loaded when a session works under this directory. The repository's map is the root `CLAUDE.md`. The CLI's JSON contract under
`api/cli/` is described in `cmd/CLAUDE.md`.

## OpenAPI contract (DESIGN-0002 / IMPL-0002)

A machine-readable OpenAPI 3.1 contract for the `/api/v1` surface, kept honest by
an in-process `kin-openapi` test and served at `GET /openapi.yaml`. Built per
`docs/design/0002-*.md` (**Implemented**) and `docs/impl/0002-*.md` (the phased
plan) — **all four phases COMPLETE ✅** (spec foundation + read/search contract
test; full auth/webhook surface + golden fixtures retired; serve `/openapi.yaml`;
version + document consumption). `api/README.md` is the consumer-facing guide.

- **Spec at `api/openapi.yaml`** (OAS 3.1.0), hand-authored (not generated). It is
  embedded by the tiny **`api` package** (`api/spec.go`: `//go:embed openapi.yaml`
  → `var Spec []byte`) so both the runtime server and the contract test consume
  the **same bytes**. Root-level `api/` is deliberate (fleet convention + the file
  the docz-site vendors); it is not under `internal/` because the wall governs
  Go-import visibility, and this artifact is consumed by file path/HTTP.
  It covers the **whole consumer-facing surface**: the read/search `/api/v1`
  routes, the auth endpoints (`/api/v1/auth/session` + `/logout`, public
  `/auth/login` + `/auth/callback` 302s), and the HMAC `/webhooks/github`
  receiver. Operational routes (`/openapi.yaml`, `/healthz`, `/readyz`,
  `/metrics`) stay **out** (OQ-6a).
- **Contract test** `internal/httpapi/openapi_contract_test.go` (`kin-openapi
  v0.135.0`, a **direct test-path dep**) — the **sole** wire-contract gate (the
  byte-frozen golden fixtures were retired at parity in Phase 2):
  `loadContractSpec` (`LoadFromData(api.Spec)` → `doc.Validate` →
  `gorillamux.NewRouter`) + `buildContractHandler` (wires the **full** stack
  exactly as `runServer` — read/search + gated auth behind the real
  `session.Middleware` ∘ `authorize.Middleware`, the public redirects, and the
  webhook — with in-package fakes) + `validateRoundTrip` (`openapi3filter`
  request + response, `MultiError`, snapshots the body so it survives both). **No
  build tag** — it rides `just test` / CI `Test Go`. Security is a no-op via
  `openapi3filter.NoopAuthenticationFunc` (the middleware runs for real with a
  fake session; the test asserts schemas, not the auth mechanism).
- **Schemas mirror the DTOs 1:1** (`internal/httpapi/dto.go`,
  `internal/search/types.go`, `internal/authhttp/handler.go`'s `sessionDTO`) with
  **`additionalProperties: false`** everywhere — that strictness is the drift
  detector (an added/renamed field fails the test). Nullable columns are empty
  strings (never `null`); JSONB arrays are `[]`. The read/search error envelope
  stays `{"error": string}` and list responses stay envelope objects
  (`{"repos":[…]}`); the **webhook + logout** use a separate `{"status": string}`
  envelope (`StatusResponse`). RFC 7807 + bare arrays are deferred (FU-1/FU-2).
- **Security model:** the top-level default is `sessionCookie` (apiKey in the
  `docz_session` cookie); `/api/v1/*` inherits it, and the three public routes
  (`login`, `callback`, `githubWebhook`) override with `security: []`. The webhook
  HMAC-SHA256 scheme is documented in the op `description` (OpenAPI has no
  first-class HMAC-body scheme).
- **Served + versioned:** `GET /openapi.yaml` serves `api.Spec` verbatim
  (`handleOpenAPISpec` in `newRouter`, `application/yaml`, **public** — outside
  the `/api/v1` gate, so the docz-site can fetch it without a session). No `/docs`
  UI (OQ-3d). `info.version` is **SemVer from `1.0.0`** (OQ-5a), bumped by hand on
  any specced wire change (patch = editorial, minor = additive, major = breaking),
  independent of the binary version — the consumer-pin signal. **Currently
  `1.8.0`** (IMPL-0032: the `html` route serving a document as a
  docz-html fragment; `1.7.0` was IMPL-0026's bearer scheme and the
  OAuth routes; `1.6.2` was INV-0021: `comments_lost` counts every
  comment without an anchor after the last update, remembered ones
  included; `1.6.1` was IMPL-0025: `git_sha` on `Document` described as
  a blob SHA, a cache key and not a commit, which is why docz-site's
  reader no longer shows it, #169). The docz-site
  vendors the file (or fetches the served spec at a pinned version) and generates
  a typed client; see `api/README.md`.
- **Spec lint/format (OQ-7b):** `just lint-openapi` runs **`vacuum`** (`aqua:
  daveshanley/vacuum`, pinned in `mise.toml`) against `api/vacuum-ruleset.yaml`
  (`-n warn`, fails on warnings) + `yamlfmt -lint`; `just fmt` yamlfmt-
  canonicalizes the spec. The ruleset disables **`camel-case-properties`** (the
  wire contract is snake_case by design) plus two over-strict-for-us rules, with
  rationale in-file; the spec scores **100/100**. CI's Lint job runs
  `just lint-openapi` behind a mise step.
- **Dep-settling gotcha:** `go get kin-openapi@v0.135.0` alone does **not** pull
  the `openapi3`/`openapi3filter`/`routers/gorillamux` subpackages' transitive
  `go.sum` entries (nothing imports them yet). Settle them with targeted `go get
  <subpkg>@v0.135.0` (or a `go mod tidy` once the harness imports them, which also
  promotes `kin-openapi` to a direct require).
