---
id: {{ .Prefix }}-{{ .Number }}
title: "{{ .Title }}"
status: {{ .Status }}
author: {{ .Author }}
created: {{ .Date }}
---

<!-- markdownlint-disable-file MD025 MD041 -->

# {{ .Prefix }}-{{ .Number }}: {{ .Title }}

<!--toc:start-->
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
