---
id: IMPL-0001
title: Grammar fixture
status: In Progress
author: Test Author
created: 2026-09-20
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0001: Grammar fixture

<!--docz:objective:start-->
## Objective

**Implements:** DESIGN-0014 / DESIGN-0015

Exercise every rule of the DESIGN-0014 section 3 task grammar in one
document, so the parser's tolerances are pinned by something a person can
read.
<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- The phase, task, and criteria grammar
- The shared field rules
<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- The CLI
<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

<!--docz:phase:start-->
### Phase 1: Foundations

Sets up the packages the later phases build on.

<!--docz:tasks:start-->
#### Tasks

- [x] Write the parser
      verify: `go test ./pkg/impl/...`
- [ ] Write the walker with a task description that
      wraps onto a second line
  - [ ] a nested note, which is not a task
- [ ] Bump the toolchain — deferred - human required: needs a release owner
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `go test ./...` passes
- The parser reads every document in the corpus
<!--docz:criteria:end-->
<!--docz:phase:end-->

<!--docz:phase:start-->
### Phase 2B: Cleanup

<!--docz:tasks:start-->
#### Tasks

- [x] ~~Delete the compatibility shim~~ — skipped: the shim shipped
- [ ] Update the docs
      **verify:** run the linter by hand
<!--docz:tasks:end-->
<!--docz:phase:end-->

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| --- | --- | --- |
| `pkg/impl/parse.go` | Add | The parser |
| `pkg/impl/walk.go` | Add | The task grammar |
|  |  |  |
<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Unit tests for the grammar
- [x] Golden fixtures
<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

None.
<!--docz:dependencies:end-->

<!--docz:references:start-->
## References

- [DESIGN-0014](../design/0014-v200-the-docz-api-as-one-unit.md)
<!--docz:references:end-->
