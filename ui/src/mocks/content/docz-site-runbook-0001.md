---
id: RUNBOOK-0001
title: "Rotate the session secret"
status: Active
author: Donald Gifford
created: 2026-09-25
---

<!-- markdownlint-disable-file MD025 MD041 -->

# RUNBOOK-0001: Rotate the session secret

<!--docz:last-verified:start-->
## Last Verified

| Date | PR | Commit | Verified by |
| ---- | -- | ------ | ----------- |
| 2026-09-25 | #137 | e41203e | @donaldgifford |

**Notes:** Rotated on staging.

<!--docz:last-verified:end-->

## Flow

```mermaid
flowchart LR
  A[Write the secret] --> B[Restart the site] --> C[Sign in again]
```

```docz-flow
title: A rotation
stages:
  - name: Write
    does: The new secret goes to the cluster.
  - name: Restart
    does: The rollout picks it up.
  - name: Sign in
    does: Old sessions end; a new one starts.
gates: [2]
```

<!--docz:procedure:start-->
### Procedure 1: Rotate

<!--docz:steps:start-->
#### Steps

1. Write the new secret to the cluster.

   ```sh
   kubectl -n web rollout restart deploy/site
   ```

   **Expected:** the rollout completes
2. Sign in again.

<!--docz:steps:end-->
<!--docz:procedure:end-->
