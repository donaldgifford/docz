---
id: IMPL-9002
title: A task inserted mid-run
status: In Progress
author: Fixture
created: 2026-09-20
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-9002: A task inserted mid-run

## Objective

Record what happens to task IDs when someone adds a task to a phase that is
already half done, which is the one case where an ID a consumer wrote down
stops meaning what it meant.

## Implementation Phases

### Phase 1: Already underway

Tasks 1 and 2 were checked off, then "Handle the CRLF case" was inserted
between them and the rest. Every task after the insertion shifts by one, so
what was 1.3 is now 1.4. A consumer that recorded "1.3 is done" is now wrong,
which is why the golden records the IDs and a phase's tasks are addressed by
position rather than by a stored number.

#### Tasks

- [x] Read the file
- [x] Parse the frontmatter
- [ ] Handle the CRLF case
- [ ] Walk the regions
      verify: `go test ./pkg/doczcore/docparse/...`
- [ ] Write the result

#### Success Criteria

- `make ci` is green

### Phase 2: Not started

#### Tasks

- [ ] Wire it into the CLI
- [ ] Update the docs

#### Success Criteria

- `docz validate` reports nothing on this repo's own documents

## File Changes

| File | Action | Description |
| --- | --- | --- |
| `pkg/impl/parse.go` | Modify | Handle CRLF |
| `pkg/impl/walk.go` | Add | The insertion case |

## Testing Plan

- [ ] A golden for the shifted IDs

## Dependencies

None.
