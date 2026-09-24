---
id: INV-0002
title: "Evaluate Effect migration and docz-api consolidation"
status: Concluded
author: Donald Gifford
created: 2026-07-25
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0002: Evaluate Effect migration and docz-api consolidation

**Status:** Concluded
**Author:** Donald Gifford
**Date:** 2026-07-25

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: what Effect actually is, and its 4.0 inflection point](#observation-1-what-effect-actually-is-and-its-40-inflection-point)
  - [Observation 2: what an Effect-based docz-site would look like](#observation-2-what-an-effect-based-docz-site-would-look-like)
  - [Observation 3: the eager bundle budget is the hard constraint](#observation-3-the-eager-bundle-budget-is-the-hard-constraint)
  - [Observation 4: consolidation — three shapes, one real question](#observation-4-consolidation--three-shapes-one-real-question)
  - [Observation 5: the docz Go library is the anchor](#observation-5-the-docz-go-library-is-the-anchor)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

## Question

Two questions, evaluated together because the second changes the answer
to the first:

1. What would docz-site look like migrated to
   [Effect](https://effect.website/) — which modules change, which
   don't, and what does the repo gain or lose?
2. Does it make sense to consolidate docz-site and docz-api into a
   single Effect-based codebase (i.e. rewrite the Go API in TypeScript
   on Effect)? Is it possible, and what are the pros and cons?

## Hypothesis

Going in: Effect's server-side story (typed errors, dependency
injection via Layers, built-in OTel, contract-first `HttpApi`) looked
like it might collapse a lot of hand-built machinery in both repos —
the typed error classes in `src/api/fetcher.ts`, the `arr()` wire-null
normalization, the vendored-spec + orval + drift-check pipeline, and
docz-api's hand-wired otel/prometheus plumbing. Expected outcome: an
attractive-on-paper migration whose value concentrates server-side,
with the SPA itself mostly untouched.

## Context

**Triggered by:** post-v0.1.2 exploration. Both products are shipped
and stable (docz-site v0.1.2, image + chart on GHCR; docz-api serving
a SemVer'd OpenAPI 3.1 contract). This is the right moment to ask
"should the next architectural era look different?" — before new
feature work accumulates on either side, not after.

Current shape of the two codebases:

|                | docz-site                          | docz-api                                          |
| -------------- | ---------------------------------- | ------------------------------------------------- |
| Language       | TypeScript (strict)                | Go                                                |
| Production LOC | ~5.8k                              | ~5.9k                                             |
| Test LOC       | ~3.7k (320 unit + 11 e2e)          | ~5.3k                                             |
| HTTP           | Bun static server + proxy          | chi router, 11 public ops                         |
| Data           | — (SPA)                            | Postgres (pgx + goose), Meilisearch, Redis        |
| Async jobs     | —                                  | asynq (Redis task queue) for ingest               |
| Auth           | renders login buttons only         | OAuth + OIDC, httpOnly sessions, GitHub App       |
| Contract       | vendored spec → orval client       | hand-authored OpenAPI 3.1 + kin-openapi test      |
| Observability  | —                                  | OpenTelemetry + Prometheus                        |
| Doc parsing    | —                                  | `github.com/donaldgifford/docz` v1.0.0 (Go lib)   |

## Approach

Desk investigation — no code experiments:

1. Survey Effect's current state from effect.website and the 4.0 beta
   release post (version strategy, packaging, bundle numbers,
   ecosystem modules).
2. Map every docz-site module to its Effect equivalent and classify
   the change as win / wash / churn.
3. Check the one hard numeric constraint: the 130 KB gz eager-JS
   budget vs Effect's own published size numbers.
4. Enumerate consolidation shapes (monorepo-only, BFF-growth, full
   rewrite) and map every docz-api Go dependency to its TS/Effect
   path to establish feasibility, then weigh cost/risk.

## Environment

| Component       | Version / Value                                       |
| --------------- | ----------------------------------------------------- |
| docz-site       | v0.1.2 (main @ 3304248)                               |
| docz-api        | main @ 2a5a43f, spec v1.x                             |
| Effect          | 4.0 **beta** (checked 2026-07-25); 3.x in maintenance |
| TypeScript      | pinned 5.9 series                                     |
| Runtime         | Bun (mise-pinned)                                     |
| Eager JS budget | 130 KB gz limit, ~121.4 KB used (~8.6 KB headroom)    |

## Findings

### Observation 1: what Effect actually is, and its 4.0 inflection point

Effect is a batteries-included TypeScript runtime library: effects as
values typed `Effect<Success, Error, Requirements>`, so failures and
dependencies appear in signatures. The pillars relevant here: typed
errors, Layer-based dependency injection, structured (fiber)
concurrency, `Schedule` retry/backoff, built-in OTel tracing/metrics,
and `Schema` (runtime validation + serialization + API contract
generation). Ecosystem: `@effect/platform-*` (HTTP servers/clients
incl. contract-first `HttpApi` that derives server handlers, a typed
client, **and** an OpenAPI document from one definition),
`@effect/sql-*`, `@effect/rpc`, `@effect/cluster`, `@effect/vitest`,
and `effect-atom` for React state integration.

The timing wrinkle is significant: **Effect 4.0 is in beta**
(2026-07-25). Per the release post:

- All ecosystem packages move to a single shared version (v3's
  `effect@3.x` + `@effect/platform@0.x` version skew goes away).
- The fiber runtime was rewritten; a minimal program drops from
  ~70 kB (v3) to ~20 kB (v4) — Effect's own numbers, pre-gzip.
- Schema was rewritten (it has a dedicated migration guide).
- New APIs land under `effect/unstable/*` without semver commitment.
- v3 gets maintenance only; **new features are v4-exclusive**.
  Codemods/migration tooling "forthcoming".

Practical consequence: adopting v3 today guarantees a second
migration; adopting v4 today means building on a beta whose HTTP and
Schema surfaces are still settling. Either way, "wait for 4.0 LTS" is
the only calm entry point for anything load-bearing.

### Observation 2: what an Effect-based docz-site would look like

Module-by-module mapping of the SPA:

| Current | Effect shape | Verdict |
| --- | --- | --- |
| `src/api/fetcher.ts` typed errors (`SessionRequiredError`, `NotFoundError`, `ApiError`) | `Schema.TaggedError` classes on `HttpApi` endpoints; the derived client returns them in the error channel | **win**, but small — the hand-rolled version is ~100 LOC and already works |
| `src/lib/wire.ts` `arr()` null-normalization (docz-api marshals nil slices as `null`) | `Schema.NullOr(Schema.Array(T))` with a decode-time default — handled once at the boundary instead of at every use site | **win** — the single best Effect fit in the repo |
| orval + vendored spec + `gen-api-check` + spec-drift workflow | deleted **only if** the contract is authored as `HttpApi`/Schema on the server side — i.e. only under consolidation. Standalone, the SPA still consumes someone else's OpenAPI, so codegen stays | wash standalone / win consolidated |
| TanStack Query (caching, retry policy, prefetch) | `effect-atom` (atom-react) or a hand-rolled query layer | **churn** — TanStack Query is battle-tested here; every route, the palette, and `usePrefetchDoc` get rewritten for equivalent behavior |
| Markdown pipeline (`src/markdown/`: unified/rehype, sanitize, Shiki, mermaid, xrefs) | unchanged — synchronous transform code with no async orchestration and no error channels worth typing | none — and this is the majority of the repo's real complexity |
| Router, tokens.css, cmdk palette, URL-state (`searchParams.ts`) | unchanged | none |
| `server/serve.ts` (Bun static + proxy + runtime config injection) | `@effect/platform-bun` `HttpApp`: routes as composable values, `Config` module for `DOCZ_API_URL` / `DOCZ_AUTH_PROVIDERS` (typed, validated, self-documenting), graceful shutdown built in | **win** — bounded, server-side, zero bundle impact |
| Vitest + MSW + Playwright | keep; `@effect/vitest` adds `TestClock`/Layer injection where Effect is used | wash |

Illustrative contract sketch (v4 API surface still settling — shape,
not gospel):

```ts
class SessionRequired extends Schema.TaggedError<SessionRequired>()(
  "SessionRequired",
  {},
) {}

const Docs = HttpApiGroup.make("docs").add(
  HttpApiEndpoint.get("getDoc", "/api/v1/repos/:repo/docs/:docId")
    .addSuccess(Doc) // Schema — wire nulls normalized at decode
    .addError(SessionRequired, { status: 401 })
    .addError(DocNotFound, { status: 404 }),
);
// One definition derives: server handlers, typed client, OpenAPI doc.
```

The honest summary: the SPA's complexity lives in rendering, URL
state, and a11y — places Effect has nothing to say. Effect's wins
concentrate in exactly the two smallest files (`fetcher.ts`,
`wire.ts`) and the one file that isn't in the browser at all
(`server/serve.ts`).

### Observation 3: the eager bundle budget is the hard constraint

CI fails above 130 KB gz for the entry chunk + modulepreload closure;
we sit at ~121.4 KB (~8.6 KB headroom). An Effect-based data layer is
eager by nature — the fetch/query machinery loads with the shell and
cannot hide in a lazy chunk.

Effect's own published numbers: a minimal program is ~70 kB on v3 and
~20 kB on v4, pre-gzip. Even at an optimistic 3:1 gzip ratio that is
roughly ~23 kB gz (v3) / ~7 kB gz (v4) for the *minimal* case —
before Schema definitions for eleven operations and an HttpApi
client. v3 is flatly impossible inside 8.6 KB; v4 might squeeze the
minimal runtime in but not realistic usage. An SPA migration
therefore means **raising the budget we deliberately set** — spending
CI-protected headroom on plumbing the user never sees. (For scale:
TanStack Query costs ~12 KB gz and is already paid for inside the
121.4.)

### Observation 4: consolidation — three shapes, one real question

**(a) Monorepo without rewrite** (Go API + TS site in one repo).
Possible; orthogonal to Effect. Loses the independent release trains
that just got built (per-repo tags → image + chart publish,
cosign/SLSA, changelog automation — all duplicated then re-merged)
and buys little: the contract is already shared via the vendored spec
plus drift workflow. Cost > benefit today.

**(b) BFF growth** — Effect-ize `server/serve.ts` and let it absorb
site-side server concerns over time. Bounded (~220 LOC with an
existing Bun test suite), zero bundle impact, real DX win (`Config`,
typed routes, tracing if ever wanted). The only shape cheap enough to
simply try.

**(c) Full rewrite** — docz-api reimplemented in Effect TS, one
codebase serving API + SPA statics. Feasibility map:

| docz-api (Go) | Effect/TS path | Assessment |
| --- | --- | --- |
| chi + hand-authored OpenAPI + kin-openapi contract test | `HttpApi` — the contract *generates* the OpenAPI instead of being tested against it | genuine win; drift machinery deleted in both repos |
| pgx + goose | `@effect/sql-pg` + its Migrator | mature enough |
| meilisearch-go | official `meilisearch-js`, hand-wrapped as an Effect service | fine, but new code |
| go-redis sessions | redis client wrapped as a service | fine, but new code |
| **asynq** (ingest queue: retries, scheduling) | no TS twin — BullMQ (different semantics) or `@effect/cluster` workflows (young) | **forced queue redesign**, not a port |
| ghinstallation + go-github (GitHub App) | octokit App auth | fine |
| go-oidc + oauth2 | openid-client | fine |
| otel + prometheus (hand-wired) | Effect-native tracing/metrics → OTLP | genuine win |
| viper config | `Config` module | win |
| testcontainers-go | testcontainers-node | works, heavier |
| **`github.com/donaldgifford/docz` v1.0.0** | none — see Observation 5 | **blocker-grade** |

Pros of (c): one language and toolchain; Schema contract shared
end-to-end (delete orval, the vendored spec, `gen-api-check`, and the
spec-drift workflow); typed errors over the wire; the nil-slice class
of bug becomes structurally impossible; one deploy artifact — the API
serves the SPA, so the proxy hop, one of two Helm charts, and one
compose service disappear; Effect-native observability.

Cons of (c): a full rewrite of a mature, *released*, SemVer'd public
API — ~5.9k LOC plus 5.3k test LOC to port while staying
byte-compatible for existing consumers (even the nil-slice quirk is
now documented, worked-around behavior; deviation is a compat risk);
a forced queue redesign; every integration re-wrapped by hand; the Go
artifact's boring ops profile (static binary, tiny RSS) traded for a
Bun service; built on a beta or on a maintenance branch
(Observation 1); realistically 4–8 focused weeks for zero
user-visible features. And the anchor:

### Observation 5: the docz Go library is the anchor

docz-api does not own the docz document contract — it imports it.
`internal/doczcontract` wraps `github.com/donaldgifford/docz` v1.0.0,
the same Go library that powers the docz CLI. The CLI stays Go
regardless of what happens here.

Today the parsing/contract logic exists in exactly **one** place, in
one language, versioned once. A TS rewrite of docz-api must
reimplement it, after which the contract lives in two languages that
can drift — the exact failure mode the current architecture exists to
avoid. This is the strongest single argument against consolidation,
and it is independent of Effect's merits: it holds for any non-Go
rewrite. It flips only if the docz contract ever becomes a
language-neutral spec (schema + conformance fixtures) rather than a
Go library.

## Conclusion

**Answer:**

1. *What would an Effect docz-site look like?* Largely identical
   where it matters. The markdown pipeline, routing, URL-state, and
   a11y machinery — the repo's real complexity — are untouched.
   Effect's wins land in the three smallest server-adjacent pieces
   (typed errors, wire-null normalization, `serve.ts`), while its
   costs land on the biggest working surface (TanStack Query → churn
   across every route) and on a CI budget with ~8.6 KB gz of headroom
   that v3 cannot fit and v4-beta only barely might. **Migrating the
   SPA proper is low-value, high-churn.**
2. *Consolidation:* **possible — yes; sensible now — no.** Every Go
   dependency has a TS path, but the sum is a 4–8 week
   byte-compatible rewrite of released software, a forced queue
   redesign (asynq has no twin), a beta-or-maintenance version
   dilemma, and — decisively — forking the docz document contract
   into a second language while the Go CLI keeps the first
   (Observation 5). The pros (shared Schema contract, deleted drift
   machinery, one artifact, native OTel) are real, but they are
   *rewrite rewards*, collectable only after paying full price for
   features users already have.

The hypothesis held: value concentrates server-side, and the
attractive-on-paper version dissolves against the bundle budget, the
4.0 beta timing, and the contract anchor.

## Recommendation

1. **Do not consolidate now.** Record the revisit triggers: (a) the
   docz contract becomes a language-neutral spec instead of a Go
   library; (b) docz-api needs a rewrite-scale new surface anyway;
   (c) Effect 4.0 reaches LTS with a stable `HttpApi`. Any two of the
   three make this worth reopening.
2. **Do not Effect-ize the SPA.** Keep orval + TanStack Query; the
   eager-budget math alone settles it.
3. **Optional, timeboxed (1 day):** port `server/serve.ts` to
   `@effect/platform-bun` on a branch under the 4.0 beta — the one
   bounded surface with zero bundle impact and an existing test suite
   to diff against. Treat it as a taste-test for the triggers in (1),
   not a commitment; throw it away freely.
4. **Steal the one idea with no dependency:** if wire-shape bugs like
   the nil-slice incident recur, consider standalone `Schema` decode
   at the fetcher boundary before reaching for the whole runtime.
5. Re-evaluate at Effect 4.0 LTS; this doc is the baseline.

## References

- [Effect website](https://effect.website/) — checked 2026-07-25
- [Effect 4.0 beta release post](https://effect.website/blog/releases/effect/40-beta/)
- [effect-atom](https://github.com/tim-smart/effect-atom) — React
  integration
- [docz-api](https://github.com/donaldgifford/docz-api) — internal
  packages surveyed at 2a5a43f
- [docz (Go library + CLI)](https://github.com/donaldgifford/docz) —
  the contract anchor in Observation 5
- DESIGN-0001 — the stack decisions this INV re-examines
  (`docs/design/0001-docz-site-cross-repo-docz-reader-and-search-ui.md`)
- `scripts/bundle-budget.ts` — the eager-JS gate in Observation 3
- `src/lib/wire.ts`, `src/api/fetcher.ts` — the hand-built pieces
  Effect would subsume
- INV-0001 — prior investigation, format precedent
