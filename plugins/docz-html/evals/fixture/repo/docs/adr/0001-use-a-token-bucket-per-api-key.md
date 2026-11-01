---
id: ADR-0001
title: "Use a token bucket per API key"
status: Proposed
author: Sam Rivera
created: 2026-10-20
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0001: Use a token bucket per API key

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Summary](#summary)
- [Context](#context)
- [Decision](#decision)
  - [Supporting Data](#supporting-data)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:summary:start-->
## Summary

Limit each API key with a token bucket held in Redis. It allows short
bursts while holding the average rate, and it costs one round trip.

<!--docz:summary:end-->

<!--docz:context:start-->
## Context

DESIGN-0001 needs a limiting algorithm. Partner traffic is bursty: a
batch job sends hundreds of requests in a second, then nothing.

<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

Use a token bucket per key, refilled continuously, with the take and
the refill in one Lua script.

### Supporting Data

A prototype added 0.6 ms at p99 against the staging Redis.

<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!-- What becomes easier or more difficult to do because of this change? -->

<!--docz:positive:start-->
### Positive

- Bursts up to the bucket size are allowed.
- One Redis round trip per request.

<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- Redis becomes a dependency of every API request.

<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- The rate is per key, not per account.

<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

A fixed window counter is simpler but lets a client send twice the
limit across a window boundary. A sliding log is exact but stores every
request's timestamp.

<!--docz:alternatives:end-->

<!--docz:references:start-->
## References

- [DESIGN-0001: Rate limiting for the public API](../design/0001-rate-limiting-for-the-public-api.md)

<!--docz:references:end-->
