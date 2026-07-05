---
id: IMPL-0099
title: "Golden Fixture: Wire The Widget"
status: In Progress
author: Test Author
created: 2026-07-04
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0099: Golden Fixture: Wire The Widget

**Status:** In Progress
**Author:** Test Author
**Date:** 2026-07-04

<!--toc:start-->
- [Objective](#objective)
<!--toc:end-->

## Objective

Implements the widget wiring described in the design doc.

**Implements:** DESIGN-0042

## Scope

### In Scope

- Widget wiring

### Out of Scope

- Widget invention

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

### Phase 1: Foundation

Establish the core seam.

#### Tasks

- [x] Add the `Widget` struct
- [x] Wire `Widget` into the runner
- [ ] Write unit tests for `Widget`

#### Success Criteria

- `go build ./...` succeeds with no errors

---

### Phase 2: Core Feature

Deliver wiring end-to-end.

#### Tasks

- [ ] Task with **bold** and `code` in it
- [ ] Ensure `make ci` passes

#### Success Criteria

- Feature works end-to-end

```go
// Example — not real facts:
// - [ ] this checkbox is inside a fence
## Not A Heading
```

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| widget.go | Create | The widget |

## Testing Plan

- [ ] Unit tests for all exported functions
- [x] Table-driven tests for variations

## Dependencies

None.

## References

- DESIGN-0042
