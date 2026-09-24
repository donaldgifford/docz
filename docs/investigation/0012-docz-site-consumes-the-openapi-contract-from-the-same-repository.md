---
id: INV-0012
title: "docz-site consumes the OpenAPI contract from the same repository"
status: Concluded
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
| Site      | docz-site at `ui/`, moved in IMPL-0020            |
| Generator | orval, `ui/orval.config.ts`                       |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

Worked through in IMPL-0020 Phase 3, against the design in DESIGN-0017 §4.

1. **Before the move** docz-site kept a hand-copied `api/openapi.yaml`, orval
   read `./api/openapi.yaml` into `src/api/__generated__/` (gitignored, never
   committed), and `spec-drift.yml` curled docz-api's `main` copy and diffed
   it against the vendored one, on pull requests and weekly. It was
   informational by design: a pull request got a warning annotation and never
   failed, and a scheduled run opened a tracking issue. So drift was noticed
   only after the server change had merged, and the site was never blocked
   by it.
2. **The swap is byte-neutral.** At the move, the vendored file and
   `api/openapi.yaml` were byte-identical. With orval's input pointed at
   `../api/openapi.yaml`, `just ui gen-api-check` printed `generated client
   is current.` with `ui/api/` still present and again after deleting it, so
   the generated client did not change by a byte. `ui/api/` and
   `spec-drift.yml` are gone.
3. **Drift now fails in the pull request that causes it**, and in both halves.
   The drill removed `author` from `Document` in `api/openapi.yaml`:
   `just ui typecheck` exited 2 with eight `TS2339`/`TS2353` errors, including
   `src/routes/doc.tsx(132,20): error TS2339: Property 'author' does not exist
   on type 'Document'.`, and the server's `TestOpenAPIContract` failed on
   `listDocs` and `getDoc` with `property "author" is unsupported`. The change
   was reverted rather than kept.
4. **CI wiring.** The Go jobs are unfiltered, and the `ui` job's path filter
   includes `api/openapi.yaml`, so a spec change always runs both the contract
   test and the site's `typecheck` and `gen-api-check` (DESIGN-0017 OQ 7).
   The image build sees the same file through bake's named `spec` context, so
   the published image is generated from the spec it ships beside.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes, the hypothesis holds. docz-site generates its client straight from
`api/openapi.yaml` (DESIGN-0017 §4), so there is no vendored copy left to
drift. Three checks keep the client and the served spec in agreement: the
server's kin-openapi contract test pins the spec to the handlers, the site's
`typecheck` fails when a spec change breaks code that reads the client, and
`gen-api-check` fails when the generated client is stale. The drill in
IMPL-0020 Phase 3 showed the first two failing on the same one-field change.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Nothing further. Keep `api/openapi.yaml` in the `ui` path filter for as long as
CI is path-filtered, since that arm is what runs the site's checks on a spec
change.

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
- [DESIGN-0017](../design/0017-move-docz-site-in-ui-chartsdocz-site-and-orval-on-the-one-spec.md)
- [IMPL-0020](../impl/0020-docz-site-move-in-v200-beta4.md)

<!--docz:references:end-->
