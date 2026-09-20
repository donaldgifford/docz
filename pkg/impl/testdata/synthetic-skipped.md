---
id: IMPL-9001
title: A plan with abandoned work
status: In Progress
author: Fixture
created: 2026-09-20
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-9001: A plan with abandoned work

<!--docz:objective:start-->
## Objective

Record every spelling of an abandoned task, so the IDs a consumer has already
written down keep pointing at the same work.
<!--docz:objective:end-->

## Implementation Phases

<!--docz:phase:start-->
### Phase 1: Abandon some work

The interesting property is the numbering: tasks 1.2 and 1.4 are skipped, and
1.3 and 1.5 keep the IDs they had before anyone gave up on their neighbours.

<!--docz:tasks:start-->
#### Tasks

- [x] Land the parser
- [ ] ~~Land the second parser~~ — skipped: the first one covers both cases
- [x] Land the walker
- [ ] ~~Land the cache~~ - skipped:
- [ ] Land the writer
      verify: `go test ./...`
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `go test ./...` passes
- No task ID changed when a task was skipped
<!--docz:criteria:end-->
<!--docz:phase:end-->

<!--docz:phase:start-->
### Phase 2: Defer the rest

<!--docz:tasks:start-->
#### Tasks

- [ ] Cut the release — deferred - human required: needs a release owner
- [ ] **Deferred** — waiting on the upstream fix
- [ ] Announce it
- [ ] ~~Backport to v1~~ — skipped: v1 is frozen
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The release is tagged
<!--docz:criteria:end-->
<!--docz:phase:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] The skipped tasks are in neither progress count
<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

None.
<!--docz:dependencies:end-->

<!--docz:references:start-->
## References

- [DESIGN-0014](../design/0014.md)
<!--docz:references:end-->
