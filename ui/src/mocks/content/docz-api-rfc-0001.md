---
id: RFC-0001
title: "Relative doc links resolve in rendered bodies"
status: Accepted
author: Donald Gifford
created: 2026-08-02
---

# RFC 0001: Relative doc links resolve in rendered bodies

**Status:** Accepted **Author:** Donald Gifford **Date:** 2026-08-02

## Summary

Documents cross-reference each other with GitHub-style relative
markdown links. The reader resolves the ones that identify an ingested
document to in-app routes; every other link renders exactly as
written.

## Motivation

A References footer written for GitHub should keep working when the
same file renders in the reader. Doc-id tokens like DESIGN-0001
already linkify; relative file links were the remaining gap.

## Decision

Resolution happens client-side against the repo's own document index.
Only exact path matches rewrite — a link that is wrong on GitHub stays
wrong here, and links to files docz never ingested pass through
untouched.

## References

- [docz-api registry design](../design/0001-docz-api-cross-repo-docz-registry-and-ingestion-service.md#goals)
- [OpenAPI contract design](../design/0002-openapi-contract-for-docz-api-and-the-docz-site.md)
- [an ADR docz-api never ingested](../adr/0001-not-ingested.md)
- [the vendored spec on GitHub](https://github.com/donaldgifford/docz-api/blob/main/api/openapi.yaml)
