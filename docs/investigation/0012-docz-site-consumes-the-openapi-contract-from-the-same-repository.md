---
id: INV-0012
title: "docz-site consumes the OpenAPI contract from the same repository"
status: Open
author: Donald Gifford
created: 2026-09-23
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0012: docz-site consumes the OpenAPI contract from the same repository

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Now that `api/openapi.yaml` lives in the same repository as the site will, how
does docz-site generate its client from it — straight from the spec at build
time, or from a vendored copy as before — and what checks that the generated
client and the served spec never disagree?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Generation straight from `api/openapi.yaml` replaces vendor-and-generate
outright: one repository means one copy of the spec, so the vendoring step and
the drift it guarded against both go away, and the kin-openapi contract test on
the server plus a generated-client typecheck on the site cover the rest.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** docz-api INV-0002, archived at
[`docs/archive/api/investigation/0002-auto-generate-an-openapi-contract-for-the-docz-site.md`](../archive/api/investigation/0002-auto-generate-an-openapi-contract-for-the-docz-site.md),
which this continues (ADR-0004, DESIGN-0016 OQ 6, IMPL-0019 Phase 4).

That investigation answered its server half: the spec-first contract it
recommended was built (docz-api DESIGN-0002 and IMPL-0002, both archived
beside it), with a hand-authored `api/openapi.yaml`, an in-process kin-openapi
contract test, and the spec served at `/openapi.yaml`. It stayed Open on the
consumer half, docz-site's vendor-and-generate client, which was designed for
two repositories. After the move there is one, and docz-site is next to arrive
(ADR-0004's order: just, docz-api, docz-site).

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Inventory how docz-site generates its client today: the vendored spec's
   path, the generator, and where `src/api/__generated__/` is produced (the
   root `.gitignore` already ignores it at any depth).
2. Point the generator at `api/openapi.yaml` and confirm the generated client
   is byte-identical to the vendored one at the same spec version.
3. Decide what runs in CI: the server's contract test already pins the spec to
   the handlers; name the site-side check that pins the client to the spec.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value                                   |
| --------- | ------------------------------------------------- |
| Spec      | `api/openapi.yaml` (docz-api, moved in IMPL-0019) |
| Site      | docz-site, not yet moved                          |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

Not started: this successor records what was still open when docz-api's
investigation was archived. The work begins with the docz-site move.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Inconclusive — not yet investigated. This successor carries the
question forward; the verdict comes with the docz-site move.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Take this up in the design for the docz-site move, before `ui/` lands.

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- docz-api INV-0002 (archived):
  [`docs/archive/api/investigation/0002-auto-generate-an-openapi-contract-for-the-docz-site.md`](../archive/api/investigation/0002-auto-generate-an-openapi-contract-for-the-docz-site.md)
- docz-api DESIGN-0002 and IMPL-0002 (archived):
  [`docs/archive/api/design/`](../archive/api/design/),
  [`docs/archive/api/impl/`](../archive/api/impl/)
- [ADR-0004](../adr/0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md)
- [DESIGN-0016](../design/0016-move-docz-api-in-internal-cmddocz-api-api-charts-and.md)
- [IMPL-0019](../impl/0019-docz-api-move-in-v200-beta3.md)

<!--docz:references:end-->
