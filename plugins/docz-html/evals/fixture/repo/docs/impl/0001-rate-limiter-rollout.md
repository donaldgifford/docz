---
id: IMPL-0001
title: "Rate limiter rollout"
status: Draft
author: Sam Rivera
created: 2026-10-20
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0001: Rate limiter rollout

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Limiter core](#phase-1-limiter-core)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Report-only mode](#phase-2-report-only-mode)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Enforcement](#phase-3-enforcement)
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

Build the per-key rate limiter and roll it out in three steps.

**Implements:** DESIGN-0001

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- The gateway middleware and its Lua script.
- The response headers.

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- The admin console's rate editor.

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

<!--docz:phase:start-->
### Phase 1: Limiter core

The bucket arithmetic and the Redis script.

<!--docz:tasks:start-->
#### Tasks

- [x] Write the Lua take-and-refill script
- [x] Unit tests for the refill arithmetic
- [ ] Integration test against a real Redis

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The script refuses the 101st request in a burst

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: Report-only mode

Wire the limiter into the gateway without refusing anything.

<!--docz:tasks:start-->
#### Tasks

- [ ] Gateway middleware that calls the script
- [ ] Log each request that would have been refused
- [ ] Add the `RateLimit-*` headers

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- A week of logs with no added p99 latency over 2 ms

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: Enforcement

Refuse requests over the limit, new keys first.

<!--docz:tasks:start-->
#### Tasks


<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every key is limited

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

<!-- Key files that will be created or modified -->

| File | Action | Description |
| ---- | ------ | ----------- |
| `gateway/ratelimit.lua` | Create | The take-and-refill script |
| `gateway/middleware.go` | Modify | Call the limiter |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Load test at three times the default rate

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

The gateway's Redis connection.

<!--docz:dependencies:end-->

<!--docz:references:start-->
## References

- [DESIGN-0001: Rate limiting for the public API](../design/0001-rate-limiting-for-the-public-api.md)
- [ADR-0001: Use a token bucket per API key](../adr/0001-use-a-token-bucket-per-api-key.md)

<!--docz:references:end-->
