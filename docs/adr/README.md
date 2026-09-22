# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records documenting significant
technical decisions.

## What are ADRs?

ADRs document **technical implementation decisions** for specific architectural
components. Each ADR focuses on a single decision and includes:

- **Context**: The problem or constraint that led to this decision
- **Decision**: What was chosen and why
- **Consequences**: Trade-offs, pros, and cons
- **Alternatives**: Other options that were considered

## Creating a New ADR

```bash
docz create adr "Your ADR Title"
```

## ADR Status

- **Proposed**: Under discussion, not yet approved
- **Accepted**: Approved and being implemented or already implemented
- **Deprecated**: No longer relevant or superseded
- **Superseded by ADR-XXXX**: Replaced by another ADR

<!-- BEGIN DOCZ AUTO-GENERATED -->
## All ADRs

| ID | Title | Status | Date | Author | Link |
|----|-------|--------|------|--------|------|
| ADR-0001 | pkg/doczcore as the single public core; cmd as a thin CLI shell (v1.0.0) | Accepted | 2026-07-03 | Donald Gifford | [0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md](0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md) |
| ADR-0002 | docz is an API package whose first consumer is the CLI | Accepted | 2026-09-14 | Donald Gifford | [0002-docz-is-an-api-package-whose-first-consumer-is-the-cli.md](0002-docz-is-an-api-package-whose-first-consumer-is-the-cli.md) |
| ADR-0003 | Remove plan from the built-in document types | Accepted | 2026-09-14 | Donald Gifford | [0003-remove-plan-from-the-built-in-document-types.md](0003-remove-plan-from-the-built-in-document-types.md) |
| ADR-0004 | One repository: docz, docz-api, and docz-site as a single Go module | Accepted | 2026-09-21 | Donald Gifford | [0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md](0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md) |
<!-- END DOCZ AUTO-GENERATED -->
