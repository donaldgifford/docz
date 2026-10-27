---
id: IMPL-0001
title: "Ship the cache"
status: Draft
author: Fixture Author
created: 2026-03-04
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0001: Ship the cache

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Foundation](#phase-1-foundation)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Rollout](#phase-2-rollout)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Polish](#phase-3-polish)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:objective:start-->
## Objective

<!-- What is being implemented? Link to the RFC/DESIGN it implements. -->

**Implements:** DESIGN-0001

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

-

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

-

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

<!--docz:phase:start-->
### Phase 1: Foundation

<!-- Describe what this phase establishes. Focus on the internal
     building blocks that later phases depend on. -->

<!--docz:tasks:start-->
#### Tasks

- [x] Add the cache client
- [ ] Wire the cache into the reader
  - [ ] Invalidate on ingest
- [ ] Task description
- [ ] Write unit tests for ...

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `go build ./...` succeeds with no errors
- ...

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: Rollout

<!-- Describe what this phase delivers to users. -->

<!--docz:tasks:start-->
#### Tasks

- [ ] Turn the cache on behind a flag
- [ ] Task description
- [ ] Write integration tests for ...

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Feature X works end-to-end
- ...

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: Polish

<!-- Harden, test, and prepare for release. -->

<!--docz:tasks:start-->
#### Tasks

- [ ] Audit error messages for consistency
- [ ] Ensure `make ci` passes
- [ ] Review test coverage (target: >80%)
- [ ] Clean up any TODO/FIXME comments

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `make ci` passes with zero errors
- Test coverage >80% for all packages
- All error paths produce clear, actionable messages

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

<!-- Key files that will be created or modified -->

| File | Action | Description |
| ---- | ------ | ----------- |
|      | Create |             |
|      | Modify |             |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Unit tests for all exported functions
- [ ] Integration tests using `t.TempDir()` for filesystem operations
- [ ] Table-driven tests for functions with multiple input variations

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

<!-- External dependencies, blocking work, prerequisites -->

<!--docz:dependencies:end-->

<!--docz:references:start-->
## References

<!-- Links to related RFCs, ADRs, designs, plans, issues -->

<!--docz:references:end-->
