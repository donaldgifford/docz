# docz-api OpenAPI contract

`openapi.yaml` is the **hand-authored, machine-readable contract** for docz-api's
HTTP surface (OpenAPI 3.1). It is the single source of truth for the wire shapes
docz-api ships, and it is the artifact the **docz-site** vendors to generate a
typed client.

## What is here

- **`openapi.yaml`** — the OAS 3.1 contract: the `/api/v1` read + search routes,
  the site auth endpoints (`/api/v1/auth/*`, public `/auth/login` + `/auth/callback`),
  and the GitHub webhook receiver (`/webhooks/github`).
- **`spec.go`** — a one-line `//go:embed openapi.yaml` exposing `var Spec []byte`,
  so the runtime server and the contract test consume the exact same bytes.
- **`vacuum-ruleset.yaml`** — the linter ruleset used by `just lint-openapi`.

## How it stays honest

An in-process `kin-openapi` contract test
(`internal/httpapi/openapi_contract_test.go`) loads this file, drives the real
chi handler stack in-memory, and validates every request and response against it
on every `go test ./...` / CI run. Response schemas are `additionalProperties:
false`, so any added, renamed, or retyped wire field fails the contract test —
code and spec cannot silently drift. `just lint-openapi` additionally runs
`vacuum` (100/100) and `yamlfmt` so the file stays standards-clean and canonical.

## Versioning

`info.version` is **SemVer, starting at `1.0.0`** (independent of the binary's
release version). Bump it by hand on any change to a specced wire shape:

- **patch** — editorial only (descriptions, examples); no wire change.
- **minor** — additive, backward-compatible (a new endpoint, a new optional
  field, a new enum value).
- **major** — breaking (a removed or renamed field, a changed type, a removed
  endpoint, a newly required field or header).

The version is the signal consumers pin against, so a wire change without a bump
is a contract bug.

## Consuming it (the docz-site)

The docz-site vendors-and-generates, mirroring the `rfc-site` model:

1. **Vendor** this `openapi.yaml` into the site repo (pinning a known version),
   **or** fetch it at runtime from `GET /openapi.yaml` (served verbatim from the
   embed, public — no session required).
2. **Generate** a typed client from the vendored spec with the site's toolchain
   (e.g. `openapi-typescript` or `orval`).
3. **Re-vendor** when `info.version` bumps; a major bump signals a breaking
   change to reconcile before upgrading.

There is **no bundled Swagger/Scalar UI** in docz-api (IMPL-0002 OQ-3d) — the
served `/openapi.yaml` is the machine contract; human browsing is left to the
consumer's tooling or an optional docz mkdocs render.

## References

- [DESIGN-0002](../docs/design/0002-openapi-contract-for-docz-api-and-the-docz-site.md)
  — the design this contract implements.
- [IMPL-0002](../docs/impl/0002-openapi-contract-for-docz-api-and-the-docz-site.md)
  — the phased implementation plan.
