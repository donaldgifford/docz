---
id: {{ .Prefix }}-{{ .Number }}
title: "{{ .Title }}"
status: {{ .Status }}
author: {{ .Author }}
created: {{ .Date }}
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL {{ .Number }}: {{ .Title }}

**Status:** {{ .Status }}
**Author:** {{ .Author }}
**Date:** {{ .Date }}

## Objective

<!-- What is being implemented? Link to the RFC/DESIGN/PLAN it implements. -->

**Implements:** <!-- RFC-XXXX / DESIGN-XXXX / PLAN-XXXX -->

## Scope

### In Scope

-

### Out of Scope

-

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

### Phase 1: <!-- Foundation / Setup / Core -->

<!-- Describe what this phase establishes. Focus on the internal
     building blocks that later phases depend on. -->

#### Tasks

- [ ] Task description
- [ ] Task description
- [ ] Write unit tests for ...

#### Success Criteria

- `go build ./...` succeeds with no errors
- ...

---

### Phase 2: <!-- Core Feature / Primary Commands -->

<!-- Describe what this phase delivers to users. -->

#### Tasks

- [ ] Task description
- [ ] Task description
- [ ] Write integration tests for ...

#### Success Criteria

- Feature X works end-to-end
- ...

---

### Phase 3: <!-- Polish / Edge Cases / CI Readiness -->

<!-- Harden, test, and prepare for release. -->

#### Tasks

- [ ] Audit error messages for consistency
- [ ] Ensure `make ci` passes
- [ ] Review test coverage (target: >80%)
- [ ] Clean up any TODO/FIXME comments

#### Success Criteria

- `make ci` passes with zero errors
- Test coverage >80% for all packages
- All error paths produce clear, actionable messages

---

## File Changes

<!-- Key files that will be created or modified -->

| File | Action | Description |
|------|--------|-------------|
|      | Create |             |
|      | Modify |             |

## Testing Plan

- [ ] Unit tests for all exported functions
- [ ] Integration tests using `t.TempDir()` for filesystem operations
- [ ] Table-driven tests for functions with multiple input variations

## Dependencies

<!-- External dependencies, blocking work, prerequisites -->

## References

<!-- Links to related RFCs, ADRs, designs, plans, issues -->
