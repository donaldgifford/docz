---
id: IMPL-0002
title: "Registry store migration"
status: In Progress
author: Parity Fixture
created: 2026-03-04
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0002: Registry store migration

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 0: Schema and migrations](#phase-0-schema-and-migrations)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 1: Dual write](#phase-1-dual-write)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 2: Cut over and delete the file path](#phase-2-cut-over-and-delete-the-file-path)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

<!--docz:objective:start-->
## Objective

This implements moving the document registry from flat files on disk to a
PostgreSQL-backed store. The current file-based registry does not scale
past a few thousand documents and makes concurrent writes from multiple
docz-api instances unsafe. The new schema supports transactional writes,
indexed lookups by id and type, and row-level locking during status
transitions.

**Implements:** DESIGN-0002
<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- Schema design for the `documents` and `document_events` tables
- A dual-write shim so the file store and Postgres stay in sync during
  rollout
- Backfill tooling to import existing flat-file documents into Postgres
- The cutover switch and removal of the flat-file code path
<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Any change to the public docz-api HTTP contract
- Multi-region replication, tracked separately in INV-0006
- Migrating the wiki nav cache, which stays file-based for now
<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

<!--docz:phase:start-->
### Phase 0: Schema and migrations

Stand up the Postgres schema and the migration tooling that later phases
depend on. No production traffic touches Postgres yet, so this phase is
purely additive and can ship without a feature flag.

```sql
CREATE TABLE documents (
    id         TEXT PRIMARY KEY,
    doc_type   TEXT NOT NULL,
    status     TEXT NOT NULL,
    path       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

<!--docz:tasks:start-->
#### Tasks

- [x] Write the `documents` table migration (id, type, status, path, created_at)
- [x] Write the `document_events` table migration for audit history
- [x] Add a `migrate` subcommand that runs migrations via golang-migrate
- [x] Write unit tests for the migration runner against a throwaway database
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `go build ./...` succeeds with no errors
- `docz-api migrate up` applies cleanly against a fresh Postgres 15 instance
- `docz-api migrate down` leaves the database in its prior state
<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 1: Dual write

Write to both the flat-file registry and Postgres on every create, update,
and status change. Reads still come from the flat-file store, so a bug in
the Postgres path cannot take down the API.

<!--docz:tasks:start-->
#### Tasks

- [x] Add a `dualWriter` wrapper around the existing file store
- [x] Wire `dualWriter` into `docz create`, `docz update`, and `docz status set`
- [ ] Add a reconciliation job that diffs file and Postgres state nightly
  - [ ] Alert when the reconciliation job finds more than 1% drift
- [ ] Write integration tests for dual-write failure modes (Postgres down,
      file write fails, partial write)
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every write path exercises both stores in the same request
- The reconciliation job runs in CI against a seeded fixture set
- No increase in p99 write latency versus the file-only baseline
<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: Cut over and delete the file path

Flip reads to Postgres, watch error budgets for a full release cycle, and
then delete the flat-file registry code entirely.

<!--docz:tasks:start-->
#### Tasks

- [ ] Flip the read path to Postgres behind the `DOCZ_REGISTRY_BACKEND` flag
- [ ] Monitor p99 latency and error rate for two weeks post-flip
- [ ] Remove `internal/registry/filestore` and its tests
- [ ] Update the operator runbook to drop the flat-file recovery steps
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `make ci` passes with zero errors
- No references to `filestore` remain outside git history
- The on-call runbook reflects the Postgres-only path
<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `internal/registry/postgres/schema.sql` | Create | Initial schema for `documents` and `document_events` |
| `internal/registry/postgres/store.go` | Create | Postgres-backed implementation of the `registry.Store` interface |
| `internal/registry/dualwrite.go` | Create | Wrapper that writes to both backends during rollout |
| `internal/registry/filestore/filestore.go` | Modify | Trimmed in Phase 2, deleted once cutover is verified |
<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Unit tests for the Postgres store against a real database, not mocks
- [ ] Integration tests for the dual-write path covering partial failures
- [ ] Load test the migration and backfill tooling against a snapshot of
      production-sized data
- [ ] Manual verification of the cutover flag in staging before flipping it
      in production
<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- A Postgres 15 instance provisioned in staging and production
- `golang-migrate` added as a build dependency
- Sign-off from the on-call rotation before flipping the cutover flag in
  Phase 2
<!--docz:dependencies:end-->

<!--docz:references:start-->
## References

- [DESIGN-0002: Registry storage backend](../design/0002-registry-storage-backend.md)
- [golang-migrate documentation](https://github.com/golang-migrate/migrate)
- [INV-0006: Multi-region replication options](../investigation/0006-multi-region-replication-options.md)
<!--docz:references:end-->
