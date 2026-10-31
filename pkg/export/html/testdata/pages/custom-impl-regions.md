---
id: FW-0001
title: A framework rollout carrying IMPL regions
status: Draft
author: Test
created: 2026-03-04
---

# FW-0001: A framework rollout carrying IMPL regions

<!--docz:objective:start-->
## Objective

A custom type whose document carries IMPL regions gets phase cards,
because the layout reads region kinds and never the type name.

<!--docz:objective:end-->

<!--docz:phase:start-->
### Phase 1: Groundwork

Lay the foundation.

<!--docz:tasks:start-->
#### Tasks

- [x] Write the config.
- [ ] Wire the loader.
- [ ] Port the old flag. **Deferred** — waits on the CLI rework.
- [ ] ~~Drop the shim~~ — skipped: nothing uses it.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The loader reads the config.

<!--docz:criteria:end-->
<!--docz:phase:end-->

<!--docz:phase:start-->
### Phase 2: Rollout

<!--docz:tasks:start-->
#### Tasks

- [x] Ship it.
- [x] Announce it.

<!--docz:tasks:end-->
<!--docz:phase:end-->
