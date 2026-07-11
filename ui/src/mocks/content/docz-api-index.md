# docz-api

Cross-repo **docz** registry and ingestion service: a GitHub App ingests
`docs/` trees on push, a Postgres registry keeps the canonical document
set, and Meilisearch serves faceted full-text search over all of it.

## What lives here

| Directory | Contents |
| --- | --- |
| `design/` | Service, contract, and endpoint designs |
| `rfc/` | Cross-cutting proposals |

## Start here

- [DESIGN-0001 — cross-repo docz registry and ingestion service](design/0001-docz-api-cross-repo-docz-registry-and-ingestion-service.md)
- [DESIGN-0002 — OpenAPI contract for docz-api and the docz-site](design/0002-openapi-contract-for-docz-api-and-the-docz-site.md)

## Operating notes

The API is same-origin only (no CORS) and serves its own spec at
`GET /openapi.yaml`. Sessions ride the `docz_session` cookie; the GitHub
App owns ingest via webhooks.
