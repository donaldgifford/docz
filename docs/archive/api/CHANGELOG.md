# Changelog

All notable changes to this project are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/).
## [0.4.0] - 2026-07-11

### Features

- *(store)* Add repos.index_md/index_sha migration
- *(store)* Carry index_md/index_sha through UpsertRepo
- *(store)* Map RepoInput index pair through ReconcileRepo
- *(ingest)* Add the index pair to the repo snapshot
- *(githubapp)* Fetch docs_dir index.md via a targeted blob lookup
- *(ingest)* Map the cached index pair into the reconcile input
- *(httpapi)* Serve the repo index at /api/v1/repos/{owner}/{name}/index

### Documentation

- *(repo-index)* Add INV-0003 and DESIGN-0003 for the repo index endpoint
- *(repo-index)* Add IMPL-0003 with resolved open questions
- *(repo-index)* Complete IMPL-0003 Phase 1 (persistence)
- *(repo-index)* Complete IMPL-0003 Phase 2 (fetch + ingest)
- *(repo-index)* Complete IMPL-0003 Phase 3 (endpoint + contract)
- *(repo-index)* Close out IMPL-0003

### Styling

- *(githubapp)* Join the index path with path.Join

### Testing

- *(store)* Cover the index pair lifecycle and migration round-trip
- *(e2e)* Prove the repo index serve and removal path

## [0.3.0] - 2026-07-10

### Features

- *(openapi)* Add kin-openapi v0.135.0 dependency
- *(openapi)* Add api package embedding the OpenAPI spec
- *(openapi)* Add spec header, servers, tags, security scheme
- *(openapi)* Author component schemas from the response DTOs
- *(openapi)* Author responses, parameters, and the six read paths
- *(openapi)* Add kin-openapi contract test harness
- *(openapi)* Add vacuum spec lint + yamlfmt tooling
- *(openapi)* Spec the auth + webhook surface with security overrides
- *(openapi)* Embed and serve the spec at GET /openapi.yaml

### Refactor

- *(openapi)* Retire golden fixtures at parity

### Documentation

- *(investigation)* Add INV-0002 OpenAPI contract investigation
- *(design)* Add DESIGN-0002 OpenAPI contract design
- *(impl)* Add IMPL-0002 OpenAPI contract implementation plan
- *(openapi)* Complete IMPL-0002 Phase 1
- *(openapi)* Complete IMPL-0002 Phase 2
- *(openapi)* Version, document consumption, close out
- *(openapi)* Check off the IMPL-0002 testing plan

### Testing

- *(openapi)* Drive the auth + webhook endpoints in the contract test

### Miscellaneous Tasks

- *(settings)* Allow markdownlint-cli in Claude Code permissions

## [0.2.0] - 2026-07-08

### Features

- Upgrade to docz v1.0.0

## [0.1.0] - 2026-07-07

### Features

- *(config)* Add typed env configuration with validation
- *(cmd)* Wire main with config, slog, and graceful HTTP server
- *(dev)* Add local compose stack for Postgres, Redis, Meilisearch
- *(store)* Add initial schema migration
- *(store)* Embed migrations and run them on startup
- *(store)* Generate typed queries with sqlc
- *(store)* Add transactional ReconcileRepo and store layer
- *(api)* Add /readyz probe and wire runtime pgxpool
- *(githubapp)* Add App-authenticated repo fetcher
- *(ingest)* Add synchronous fetch→parse→map→reconcile pipeline
- *(authorize)* Add read-endpoint authorization seam
- *(httpapi)* Add /api/v1 read endpoints behind the authorize seam
- *(cmd)* Add -onboard flag for manual repo ingest
- *(search)* Configure Meilisearch documents index
- *(search)* Index documents after reconcile via content-hash gate
- *(search)* Add GET /api/v1/search with facets and authz filter
- *(health)* Report Meilisearch reachability in /readyz
- *(queue)* Add Redis-backed async ingest queue (asynq)
- *(queue)* Run worker in-process; -onboard enqueues; graceful drain
- *(webhook)* Add GitHub App onboarding + HMAC-verified webhooks
- *(auth)* Site-user authentication with pluggable providers + Redis sessions
- *(telemetry)* Full observability stack — slog logs, Prometheus, OTel traces

### Refactor

- *(search)* Apply Uber style-guide review fixes
- *(ingest)* Wrap Run's doc-build errors for consistency (Phase 7 task 5)

### Documentation

- Add DESIGN-0001 + IMPL-0001 for docz-api
- *(phase-4)* Mark async ingestion complete; add queue integration tests
- *(impl-0001)* Confirm docz v0.5.0 pin (Phase 7 task 1)
- *(impl-0001)* Check off the Testing Plan checklist
- *(impl-0001)* Add explicit Status blocks for Phases 0-2
- Correct CI matrix to GitHub-only (no Forgejo workflows)

### Testing

- *(store)* Add testcontainers integration tests for reconcile
- *(e2e)* Add hermetic Phase 2 onboarding integration test
- *(search)* Meilisearch integration tests via testcontainers
- *(e2e)* Prove ingest->index->search end-to-end; mark Phase 3 complete
- *(httpapi)* Freeze the read + search wire contract with golden fixtures
- Raise coverage across auth/session/search/webhook/httpapi (Phase 7 task 6)

### Miscellaneous Tasks

- Claude settings
- *(deps)* Pin docz v0.5.0 and guard pkg/doczcore with contract tests
- Close out Phase 0 — skeleton green
- *(deploy)* Reference deployment stack + confirm distroless image
- Repair template leftovers and add Apache-2.0 license
- Trufflehog fails only on verified secrets

