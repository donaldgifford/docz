---
id: DESIGN-0001
title: "Review cache"
status: Draft
author: Fixture Author
created: 2026-03-04
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0001: Review cache

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
  - [Notes](#notes)
- [Detailed Design](#detailed-design)
  - [Notes](#notes-1)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. Which cache?](#1-which-cache)
  - [2. How long a TTL?](#2-how-long-a-ttl)
  - [3. Where does the cache live?](#3-where-does-the-cache-live)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:overview:start-->
## Overview

The reader re-renders every document on every visit. We use
**Postgres** for the store and keep a cache in front of it, see
[the reader](https://example.com/reader) and `docz inspect`.

The cache is warm. The cache is cold. The cache is warm.

<!--docz:overview:end-->

## Goals and Non-Goals

<!--docz:goals:start-->
### Goals

- Serve a rendered page in under 50 ms.
- Keep Tom &amp; Jerry's café notes ☕ intact: naïve résumé 🚀 done.

<!--docz:goals:end-->

<!--docz:non-goals:start-->
### Non-Goals

- Caching search results.

<!--docz:non-goals:end-->

<!--docz:background:start-->
## Background

### Notes

The first notes section.

<p data-line="1">A forged paragraph claiming line one.</p>

<!--docz:background:end-->

<!--docz:detailed-design:start-->
## Detailed Design

### Notes

The second notes section, under a heading the background also uses.

```go
func Render(doc []byte) []byte { return doc }
```

<!--docz:detailed-design:end-->

<!--docz:api-changes:start-->
## API / Interface Changes

<!-- Public API surface changes, CLI changes, config changes -->

<!--docz:api-changes:end-->

<!--docz:data-model:start-->
## Data Model

<!-- Schema changes, storage considerations -->

<!--docz:data-model:end-->

<!--docz:testing:start-->
## Testing Strategy

<!-- How will this be tested? Unit, integration, e2e? -->

<!--docz:testing:end-->

<!--docz:rollout:start-->
## Migration / Rollout Plan

<!-- How do we get from here to there safely? -->

<!--docz:rollout:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Which cache?

- a. **Valkey.** *(recommendation)*
- b. Memcached.
- c. An in-process LRU.

### 2. How long a TTL?

- a. **One hour.** *(recommendation)*
- b. One day.

### 3. Where does the cache live?

- a. Beside the API.

> **Resolved 2026-03-04: (a).**

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0002](0002-unmarked-notes.md)

<!--docz:references:end-->
