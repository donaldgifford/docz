# Changelog

All notable changes to this project are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/).
## [unreleased]

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

