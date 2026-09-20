---
id: IMPL-0042
title: "The canonical IMPL shape"
status: In Progress
author: Fixture
created: 2026-03-04
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0042: The canonical IMPL shape

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
<!--toc:end-->

<!--docz:objective:start-->
## Objective

**Implements:** DESIGN-0042
<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- The walker.
<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Everything else.
<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Intro prose outside every region.

<!--docz:phase:start-->
### Phase 0: Groundwork

<!--docz:tasks:start-->
#### Tasks

- [x] Land the walker.
- [ ] Land the goldens.
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `go test ./...` passes.
<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 1: The rest

<!--docz:tasks:start-->
#### Tasks

- [ ] Do the rest.
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Nothing left.
<!--docz:criteria:end-->
<!--docz:phase:end-->

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `regions.go` | Add | The walker |
<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Goldens.
<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- None.
<!--docz:dependencies:end-->

<!--docz:references:start-->
## References

- [DESIGN-0015](../../../../docs/design/0015-structured-regions-and-docz-validate.md)
<!--docz:references:end-->
