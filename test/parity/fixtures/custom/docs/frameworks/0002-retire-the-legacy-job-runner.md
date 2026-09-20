---
id: FW-0002
title: "Retire the legacy job runner"
status: Deprecated
author: Parity Fixture
created: 2026-03-04
---

<!-- markdownlint-disable-file MD025 MD041 -->

# FW-0002: Retire the legacy job runner

<!--toc:start-->
- [Summary](#summary)
- [Owner](#owner)
- [Adoption](#adoption)
- [References](#references)
<!--toc:end-->

## Summary

The legacy job runner is the cron wrapper that predates the scheduler. It
takes a shell command and a crontab line, writes a pid file, and does
nothing else: no retries, no structured logs, no way to ask whether last
night's run succeeded. Two of the five services still on it schedule work
that nobody notices when it stops.

**Replacement:** the internal scheduler framework, [FW-0001](0001-adopt-the-internal-auth-framework.md).

## Owner

Platform team owns the runner until the last service leaves it. After that
the repository is archived rather than deleted, because three runbooks link
to its README.

| Service | Jobs on the runner | Owner | Migrated |
| ------- | ------------------ | ----- | -------- |
| billing-export | 4 | Payments | yes |
| docs-rebuild | 1 | Platform | yes |
| search-reindex | 2 | Search | no |
| mail-digest | 1 | Growth | no |

## Adoption

Nothing new adopts it. The remaining work is subtraction, and it goes in
this order:

- Move the two Search jobs onto the scheduler.
  - Reindex is idempotent, so it can run under both for a week.
  - The delta job is not, and needs a lock before it moves.
- Move the Growth digest, which is the only job that reads the runner's
  pid file directly.
- Delete the crontab entries, then the runner's deploy.

A job moves with one config block:

```yaml
schedule:
  name: search-reindex
  cron: "0 3 * * *"
  retries: 2
  timeout: 45m
```

The pid-file read in the digest job is the one real blocker. It infers
"already running" from a file the scheduler does not write, so it needs the
scheduler's own lease API instead.

<!--docz:references:start-->
## References

- [FW-0001: Adopt the internal auth framework](0001-adopt-the-internal-auth-framework.md)
- [ADR-0001: Keep frameworks in a register](../adr/0001-keep-frameworks-in-a-register.md)
- [Scheduler lease API](https://example.invalid/scheduler/leases)
<!--docz:references:end-->
