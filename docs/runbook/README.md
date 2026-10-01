# Runbooks

This directory contains runbooks: the procedures for operating a service or
tool, and the steps for troubleshooting it.

## What are Runbooks?

A runbook is **a procedure someone runs, again and again**. It is never
"done", so its steps are numbered rather than checked off. Each runbook
includes:

- **Last Verified**: When it was last run end to end, against which PR and
  commit, and by whom
- **Overview**: What it is for, the service it covers, and who owns it
- **When to Use** and **Prerequisites**: The trigger, and the access and
  tools it needs
- **Procedures**: Ordered steps, with the commands to run and what to
  expect, plus verification and rollback
- **Troubleshooting**: One scenario per symptom, with diagnosis and
  resolution steps
- **Escalation**: Who to call when the runbook does not work

## Creating a New Runbook

```bash
docz create runbook "Your Runbook Title"
```

Runbooks ship disabled. Turn them on with `types.runbook.enabled: true` in
`.docz.yaml`.

## Runbook Status

- **Draft**: Being written; not yet run end to end
- **Active**: Verified and safe to follow
- **Needs Review**: A step is known to be wrong or stale
- **Deprecated**: No longer applies
<!-- BEGIN DOCZ AUTO-GENERATED -->
## All Runbooks

| ID | Title | Status | Date | Author | Link |
|----|-------|--------|------|--------|------|
| RUNBOOK-0001 | Cut a v2 beta release | Active | 2026-10-01 | Donald Gifford | [0001-cut-a-v2-beta-release.md](0001-cut-a-v2-beta-release.md) |
| RUNBOOK-0002 | docz-api webhook deliveries fail | Draft | 2026-10-01 | Donald Gifford | [0002-docz-api-webhook-deliveries-fail.md](0002-docz-api-webhook-deliveries-fail.md) |
<!-- END DOCZ AUTO-GENERATED -->
