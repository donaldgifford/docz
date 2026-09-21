---
id: IMPL-0001
title: "Phase out the legacy scanner"
status: In Progress
author: Parity Fixture
created: 2026-03-04
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0001: Phase out the legacy scanner

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: <!-- Foundation / Setup / Core -->](#phase-1----foundation--setup--core)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: <!-- Core Feature / Primary Commands -->](#phase-2----core-feature--primary-commands)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: <!-- Polish / Edge Cases / CI Readiness -->](#phase-3----polish--edge-cases--ci-readiness)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

<!--docz:objective:start-->
## Objective

<!-- What is being implemented? Link to the RFC/DESIGN/PLAN it implements. -->

**Implements:** <!-- RFC-XXXX / DESIGN-XXXX / PLAN-XXXX -->
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
### Phase 1: <!-- Foundation / Setup / Core -->

<!-- Describe what this phase establishes. Focus on the internal
     building blocks that later phases depend on. -->

<!--docz:tasks:start-->
#### Tasks

- [ ] Task description
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
### Phase 2: <!-- Core Feature / Primary Commands -->

<!-- Describe what this phase delivers to users. -->

<!--docz:tasks:start-->
#### Tasks

- [ ] Task description
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
### Phase 3: <!-- Polish / Edge Cases / CI Readiness -->

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
