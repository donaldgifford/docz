---
id: {{ .Prefix }}-{{ .Number }}
title: "{{ .Title }}"
status: {{ .Status }}
author: {{ .Author }}
created: {{ .Date }}
---
<!-- markdownlint-disable-file MD025 MD041 -->

# PLAN {{ .Number }}: {{ .Title }}

**Status:** {{ .Status }}
**Author:** {{ .Author }}
**Date:** {{ .Date }}

## Goal

<!-- What is the end state? What should be possible after this plan is executed?
     Be concrete — include a code snippet, CLI interaction, or example output
     that demonstrates success. -->

## Context

<!-- Why is this work needed now? What problem does it solve? What breaks or
     becomes harder if we don't do it? Link to relevant RFCs, ADRs, or issues. -->

## Approach

<!-- How will we get there? Break the approach into named parts if the work
     spans multiple concerns. Each part should be independently understandable. -->

### Part 1: <!-- e.g. Infrastructure / Foundation -->

<!-- Describe this part of the approach -->

### Part 2: <!-- e.g. Core Feature -->

<!-- Describe this part of the approach -->

## Components

<!-- What are the major pieces of this work? Use a table for a quick overview. -->

| Component | Purpose |
|-----------|---------|
|           |         |

## File Changes

<!-- Key files that will be created or modified. -->

| File | Action | Description |
|------|--------|-------------|
|      | Create |             |
|      | Modify |             |

## Verification

<!-- How do we know when this plan is complete? Include commands to run,
     outputs to check, or behaviors to verify. -->

```bash
# Example verification steps
```

## Dependencies

<!-- Other work that must be done first, external tools, or blocking issues. -->

## Open Questions

<!-- Unresolved decisions. Move each item to a Resolved section or a decision
     record (ADR) when it is answered. -->

-

## References

<!-- Links to related RFCs, ADRs, designs, issues, external docs -->
