---
id: IMPL-0007
title: "Server observability — logs and probes, then metrics and traces"
status: Completed
author: Donald Gifford
created: 2026-09-18
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0007: Server observability — logs and probes, then metrics and traces

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [PR 1 — logs, probes, error boundary](#pr-1--logs-probes-error-boundary)
    - [Phase 1: Shared primitives](#phase-1-shared-primitives)
      - [Tasks](#tasks)
      - [Success Criteria](#success-criteria)
    - [Phase 2: Structured logging](#phase-2-structured-logging)
      - [Tasks](#tasks-1)
      - [Success Criteria](#success-criteria-1)
    - [Phase 3: Split probes](#phase-3-split-probes)
      - [Tasks](#tasks-2)
      - [Success Criteria](#success-criteria-2)
    - [Phase 4: The React error boundary](#phase-4-the-react-error-boundary)
      - [Tasks](#tasks-3)
      - [Success Criteria](#success-criteria-3)
    - [Phase 5: PR 1 close-out and release](#phase-5-pr-1-close-out-and-release)
      - [Tasks](#tasks-4)
      - [Success Criteria](#success-criteria-4)
  - [PR 2 — metrics and traces](#pr-2--metrics-and-traces)
    - [Phase 6: Packaging (no-op enabler)](#phase-6-packaging-no-op-enabler)
      - [Tasks](#tasks-5)
      - [Success Criteria](#success-criteria-5)
    - [Phase 7: Prometheus metrics](#phase-7-prometheus-metrics)
      - [Tasks](#tasks-6)
      - [Success Criteria](#success-criteria-6)
    - [Phase 8: OpenTelemetry tracing](#phase-8-opentelemetry-tracing)
      - [Tasks](#tasks-7)
      - [Success Criteria](#success-criteria-7)
    - [Phase 9: PR 2 close-out and release](#phase-9-pr-2-close-out-and-release)
      - [Tasks](#tasks-8)
      - [Success Criteria](#success-criteria-8)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions — all resolved](#open-questions--all-resolved)
- [References](#references)
<!--toc:end-->

## Objective

Implement DESIGN-0006's nine components across two PRs: structured
logging, split liveness/readiness probes, and a React error boundary
first; Prometheus metrics and OpenTelemetry tracing second.

**Implements:**
[DESIGN-0006](../design/0006-server-observability-structured-logging-split-probes-prometheus.md)
(Approved), which builds on
[INV-0006](../investigation/0006-server-observability-logging-health-probes-metrics-and-the-otel.md)
(Concluded). Closes [issue #18](https://github.com/donaldgifford/docz-site/issues/18).

## Scope

### In Scope

- `server/route-class.ts`, `server/redact.ts`, `server/logger.ts` — new
  modules (Components 1–3).
- `/readyz`, and the chart's readiness probe moving off `/healthz`
  (Component 4).
- `/metrics` via `prom-client`, OTel tracing over OTLP/HTTP
  (Components 5–6).
- Bundling the server so it can carry dependencies at all
  (Component 7).
- The request-pipeline wrapper (Component 8).
- An error boundary below `AppShell` reusing `ErrorPanel`'s chrome
  (Component 9).
- Chart values, a `ServiceMonitor`, and `values.schema.json` enums.

### Out of Scope

- Browser telemetry **export** — no collector, no `sendBeacon`, no
  `window.onerror` (DESIGN-0006 OQ-6a). The error boundary catches and
  displays only.
- `PrometheusRule` starter alerts (OQ-8a — follow-up).
- Any change to docz-api. It already extracts `traceparent`; nothing
  upstream is needed.
- Log shipping or forwarder configuration.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its
tasks are checked off and its success criteria are met.

**Ordering is required, not preferred.** Phases 7 and 8 consume Phase 1's
classifier and redaction module, and Phase 6 must land before any
dependency is imported or the Docker image breaks.

Per-task discipline (repo convention): check the box, update any
technical documentation the task changes, and commit with a conventional
commit.

---

### PR 1 — logs, probes, error boundary

Zero new runtime dependencies, no Dockerfile change. Reviewable as a
behaviour change with no build-system risk.

---

#### Phase 1: Shared primitives

The two modules every signal depends on. Deliberately first and
deliberately pure — no I/O, no globals — so they are fully testable
before anything consumes them, and so the label set is exercised by
logging (Phase 2) before metrics depend on its cardinality properties.

##### Tasks

- [x] Create `server/route-class.ts` with the closed `RouteClass` union
      (`asset`, `static`, `spa`, `proxy:api`, `proxy:auth`,
      `proxy:webhooks`, `proxy:openapi`, `probe`).
- [x] Add `normalizeMethod()` collapsing anything outside
      `GET HEAD POST PUT PATCH DELETE OPTIONS` to `other` — `fetch()`
      accepts arbitrary method tokens, so this is attacker-controlled
      input, not a formality.
- [x] Create `server/redact.ts` implementing DESIGN-0006's three rules:
      headers never recorded; query keys kept and every value replaced
      with `<redacted>` via an **allowlist**; `Location` reduced to host.
- [x] Write `server/route-class.test.ts` — every class, unknown methods,
      hostile paths staying in-class.
- [x] Write `server/redact.test.ts` — including that an *unknown* query
      key is redacted by default (proves allowlist, not denylist).

##### Success Criteria

- `bun test server/` passes; both modules are pure and import nothing
  from `serve.ts`.
- A hostile path (`/a/../../etc/passwd`, 2 KB of junk, a NUL byte)
  classifies to a member of the union and never appears in the label.
- `just ci` green — the modules exist but nothing imports them yet, so
  behaviour is unchanged.

---

#### Phase 2: Structured logging

Component 3, and the one that closes issue #18. The highest-value single
change in the whole document is binding the `catch` in `proxy()`
(`serve.ts:223`), which currently destroys the upstream failure cause at
the language level.

##### Tasks

- [x] Create `server/logger.ts` — levels `debug < info < warn < error`,
      JSON and `text` modes, one object per line to stdout.
- [x] Add `resolveLogLevel()` and `resolveLogFormat()` to `serve.ts`,
      following the existing whitelist-with-fallback pattern.
- [x] Convert the startup banner to a structured `server.start` event.
- [x] **Bind the proxy `catch`** and emit `proxy.error` with
      `err_name`/`err_message`/`target_host`/`duration_ms`.
- [x] Give `502 DOCZ_API_URL is not configured` a distinct
      `not_configured` reason so a config fault is distinguishable from
      a network fault.
- [x] Add `proxy.request` (debug) carrying `upstream_status` and
      `location_host` — the OAuth-journey line the issue asks for.
- [x] Add `http.request` (debug) using Phase 1's classifier.
- [x] Skip probe paths at every level.
- [x] Write `server/logger.test.ts` — level filtering, both formats,
      unknown level falls back.
- [x] Write the **redaction gate**: drive a realistic OAuth callback
      through the logger at *every* level, asserting on captured stdout
      that `code`/`state` values never appear while their keys do.
      Parameterise over levels so a future level cannot bypass it.

##### Success Criteria

- Running with `DOCZ_API_URL` pointing nowhere produces an error-level
  line naming the cause; today it produces silence.
- Default level emits the startup line and proxy failures, nothing more.
- `DOCZ_LOG_LEVEL=debug` emits request lines and the `/auth/*` flow.
- No probe path appears in output at any level.
- The redaction gate passes, and is demonstrated to *fail* when
  redaction is deliberately disabled — verify the guard fires before
  trusting it green.
- `just ci` green.

---

#### Phase 3: Split probes

Component 4. `/healthz` keeps its exact current behaviour; readiness
becomes a real check.

##### Tasks

- [x] Export `handleRequest` so tests can drive the request path by
      calling it with a `Request` and asserting on the `Response`
      (OQ-1a). `import.meta.main` already prevents startup on import,
      so no port is bound.
- [x] Add a pure `checkReady(distDir)` returning per-check status, so
      the logic is testable without touching the module-level `DIST`
      (which is read once at import and cannot be varied afterwards).
- [x] Add the `/readyz` route: 200 `{"status":"ready","checks":{…}}`,
      503 naming the offender.
- [x] Emit `readyz.fail` (warn) when a check fails.
- [x] Confirm `/healthz` is byte-identical to today and still
      unconditional.
- [x] Point the chart's `readinessProbe` at `/readyz`; leave
      `livenessProbe` and the Dockerfile `HEALTHCHECK` on `/healthz`.
- [x] Add `config.logLevel` / `config.logFormat` chart values, env
      wiring, and `values.schema.json` enums.
- [x] Update `charts/docz-site/tests/deployment_test.yaml` — readiness
      path, new env, defaults.
- [x] Run `just helm-docs`.
- [x] Write `/readyz` tests: ready when dist present, 503 naming `dist`
      when absent, **and that it makes no network call to docz-api**.

##### Success Criteria

- `helm unittest` passes with the readiness path asserted as `/readyz`.
- A container started against an empty dist directory reports `503` on
  `/readyz` while `/healthz` still returns `200` — the distinction that
  justifies the second endpoint.
- No test or runtime path issues a request to docz-api from `/readyz`.
- `just ci` and `just helm-unittest` green.

---

#### Phase 4: The React error boundary

Component 9 — the one browser-side change. Catches and displays; does
not export.

##### Tasks

- [x] Add an error boundary in `src/app/router.tsx` on a **pathless
      layout route directly below `AppShell`**, wrapping every real
      route. NOT on the root route: a boundary replaces the element of
      the route that owns it, so a root-level one swaps out `AppShell`
      and takes the topbar with it. (Corrected here and in DESIGN-0006
      Component 9, which described the root placement; a test pins both
      behaviours.)
- [x] Render via the existing `ErrorPanel` from
      `src/components/query-states.tsx` — no new visual design.
- [x] Offer a link to `/` as the only recovery affordance (OQ-5a) — no
      reset button, which risks an immediate re-throw loop.
- [x] Keep `console.error`; the boundary changes what the *user* sees,
      not what a developer can observe.
- [x] Confirm `window.onerror` / `unhandledrejection` / any beacon
      wiring is **absent** — those are export-shaped and out of scope.
- [x] Add a route test: mount with `createMemoryRouter`, throw from a
      child, assert the panel renders and the topbar survives.
- [x] Add an entry to `src/a11y/axe.test.tsx` rendering a throwing route,
      asserting zero serious/critical violations.
- [x] Re-run `just bundle-budget` and record the delta. **Measured:
      122.5 -> 122.8 KB gz (+0.3 KB), 7.2 KB under the 130 KB budget.**

##### Success Criteria

- A route that throws renders the error panel inside the shell instead
  of a blank page, and the user can navigate away.
- The axe sweep passes on the error state — a panel with no heading or
  an unfocusable recovery control would fail it.
- Eager bundle stays under 130 KB gz (expected delta ≈ 0, since
  `ErrorPanel` already ships).
- `just ci` green.

---

#### Phase 5: PR 1 close-out and release

##### Tasks

- [x] Update `CLAUDE.md` — the logging/redaction rules, the reserved
      server paths, and the liveness-vs-readiness distinction.
- [x] Update `README.md` and the chart README with the new env and
      values.
- [x] Tick every Phase 1–4 box in this document.
- [x] Bump the chart to **0.1.9** and `appVersion` to the release
      (bare semver — metadata-action strips the `v`). **Done: chart
      `0.1.9`, `appVersion: "0.9.0"`.**
- [x] Regenerate `CHANGELOG.md` after `git fetch --tags`; the
      `chore(changelog): Auto-sync` commit must be **last**.
- [x] Open the PR with exactly one release label (`minor`) and
      `Closes #18` in the body (OQ-3a). **[#35](https://github.com/donaldgifford/docz-site/pull/35).**

##### Success Criteria

- `just ci` fully green, including `format:check`.
- All CI checks pass on the PR, including Helm jobs and trufflehog.
- Chart version bumped — otherwise the publish job's idempotency
  precheck silently skips and the chart changes never ship.
- Issue #18's requested behaviour is demonstrable from the PR
  description.

---

### PR 2 — metrics and traces

Adds the server's first runtime dependencies, so all packaging risk
lives here.

---

#### Phase 6: Packaging (no-op enabler)

Deliberately a **no-op change shipped first**: switch the image to a
bundled server while it still has zero dependencies, so that if bundling
breaks anything it breaks in isolation, with nothing else in flight.

Verified feasible before writing this phase: a file using `Bun.serve`,
`Bun.file`, `process.env`, and `import.meta.main` bundles with
`bun build --target=bun` and behaves identically when run.

##### Tasks

- [x] Add a `bun build server/serve.ts --target=bun --outfile=dist-server/serve.js`
      step to the Dockerfile build stage.
- [x] Change the runtime stage to copy `dist-server/serve.js` instead of
      `server/serve.ts`; update `CMD`.
- [x] Add `dist-server/` to `.gitignore`.
- [x] Add a `just` recipe for building and running the bundle locally.
- [x] Confirm `server/serve.test.ts` still imports the **source**, not
      the bundle.
- [x] Rebuild the image and verify `/healthz`, `/readyz`, the SPA
      fallback, and the API proxy all behave identically.
- [x] Note the image size delta. **Measured: 284 MB before and
      after — the bundle is 14.16 KB against four source files of
      comparable size, so there is no delta to speak of.**

##### Success Criteria

- The image runs from the bundle with no behaviour change whatsoever.
- `import.meta.main` still gates startup (tests do not bind a port).
- `just local-up` works against the rebuilt image.
- `just ci` green.

---

#### Phase 7: Prometheus metrics

Component 5.

##### Tasks

- [x] Add `prom-client` as a dependency.
- [x] Create `server/metrics.ts` — the four instruments plus
      `collectDefaultMetrics()`, on an explicit `Registry`.
- [x] Add `resolveMetricsEnabled()` and the `/metrics` route.
- [x] **When disabled, `/metrics` must return an explicit 404** — if the
      route is simply not registered it falls through to the SPA
      handler and a scraper receives `index.html` with a `200`
      (OQ-4a).
- [x] Record `docz_site_proxy_errors_total` in both 502 paths, with
      `unreachable` and `not_configured` reasons.
- [x] Wire HTTP metrics into the Phase 8 pipeline wrapper using Phase 1
      labels only.
- [x] Add chart `metrics.enabled` + `serviceMonitor.*` values, and
      `templates/servicemonitor.yaml` gated on **both** flags, mirroring
      docz-api's.
- [x] Note in the chart README that `nodejs_gc_duration_seconds` never
      samples under Bun, so Node dashboards will show empty panels.
- [x] Write metrics tests: exposition parses, labels bounded, probe
      paths absent, disabled returns 404 (not HTML).
- [x] Write a **cardinality regression test**: drive many distinct
      hostile paths and methods through the pipeline and assert the
      registry's series count stays bounded.
- [x] Add `helm unittest` cases for the ServiceMonitor gating.

##### Success Criteria

- `/metrics` returns valid exposition including the four instruments.
- `DOCZ_METRICS_ENABLED=false` returns 404 — verified explicitly not to
  return the SPA shell.
- 10 000 distinct request paths produce no more series than the bounded
  maximum.
- ServiceMonitor renders only when metrics **and** serviceMonitor are
  both enabled.
- `just ci` and `just helm-unittest` green.

---

#### Phase 8: OpenTelemetry tracing

Component 6, plus the Component 8 pipeline wrapper that unifies all
three signals.

##### Tasks

- [x] Add the OTel dependencies (`@opentelemetry/sdk-trace-node`,
      `@opentelemetry/api`, the OTLP/HTTP exporter).
- [x] Create `server/tracing.ts` — `NodeTracerProvider.register()`,
      resource `service.name`, clamped head sampler, batch exporter.
      Empty endpoint means no export and no overhead.
- [x] Add `resolveOtelEndpoint/ServiceName/SampleRate` with the usual
      whitelist-and-clamp discipline.
- [x] Implement the pipeline wrapper (Component 8): probe paths
      short-circuit before any signal; everything else emits log,
      metric, and span from **one** place.
- [x] Start the server span with allowlisted attributes only —
      `http.request.method`, `http.route`, `http.response.status_code`,
      redacted `url.path`. **Never** `url.full`, never headers.
- [x] Add the `proxy.upstream` child span and inject `traceparent` on
      the fetch to docz-api.
- [x] Set span status `ERROR` on 5xx only.
- [x] Confirm **no** auto-instrumentation package is installed — that is
      the mechanism that would ship OAuth codes to a collector.
- [x] Write tracing tests with an in-memory exporter: parent/child
      linkage, `traceparent` well-formed, attributes allowlisted,
      no span for probe paths, nothing exported when unconfigured.
- [x] Add an attribute-redaction test mirroring Phase 2's log gate — no
      `code`/`state` value on any span.
- [x] Add chart `otel.*` values, env wiring, schema, helm tests.

##### Success Criteria

- With a local collector, a single browser request produces one trace
  spanning docz-site **and** docz-api, joined by our injected
  `traceparent`, with no docz-api change.
  **VERIFIED against a live stack**, 2026-09-19. Both services were
  pointed at one `otel/opentelemetry-collector`; docz-api needed only
  its own `OTEL_*` env, no code change. One proxied request produced a
  single trace, `9aab035c7e6828fa3715ad353bcabac3`:

  | Span | Service | ID | Parent |
  | --- | --- | --- | --- |
  | `GET proxy:api` | docz-site | `2666e2b9de221a09` | (root) |
  | `proxy.upstream` | docz-site | `5f374598ffdd7670` | `2666e2b9de221a09` |
  | `GET /api/v1/*` | **docz-api** | `31382ce24371ef75` | `5f374598ffdd7670` |

  docz-api's server span is parented by our proxy span, so INV-0006 F5
  is now observed rather than assumed. The two exporters disagree on
  the wire — docz-api posts protobuf to a `host:port`, our SDK posts
  JSON to a full URL — which a standards-compliant collector absorbs,
  but it is worth knowing before blaming a config.
- With no endpoint configured, nothing is exported and no network call
  is attempted.
- No span attribute anywhere contains a `code` or `state` value.
- `just ci` and `just helm-unittest` green.

---

#### Phase 9: PR 2 close-out and release

##### Tasks

- [x] Update `CLAUDE.md` — the metrics/tracing surface, the bundling
      step, and the no-auto-instrumentation rule.
- [x] Update `README.md` and the chart README.
- [x] Tick every Phase 6–8 box.
- [x] Bump the chart to **0.1.10** (done: chart 0.1.10, appVersion 0.10.0) and `appVersion` to the release.
- [x] Regenerate `CHANGELOG.md`; `chore(changelog): Auto-sync` last.
- [x] Flip DESIGN-0006 to `Implemented` and this document to
      `Completed`.
- [x] Open the PR with one release label (`minor`) — **#36**, opened
      against the PR 1 branch so its diff is only Phases 6–9. GitHub
      retargets it to `main` when #35 merges. Stacked for *review*
      only; they are merged one at a time, never in a loop.

##### Success Criteria

- `just ci` green including `format:check`.
- All CI checks pass, Helm jobs included. While #36 was stacked the
  security workflows did not fire on it — they trigger on PRs targeting
  `main`, so it ran 8 checks where #35 ran 10, leaving PR 2's server
  code without static analysis. #35 merging retargeted #36 to `main`
  and CodeQL + Analyze TypeScript now run on it; **both must be green
  before merging**, since this is the PR touching OAuth-bearing proxy
  traffic. Worth knowing for any future stack: the retarget is also
  what turns the branch `DIRTY`, because the repo squash-merges and
  git cannot match PR 1's individual commits against the single squash
  on main. The fix is `git rebase --onto main <pr1-tip>`, which replays
  only PR 2's commits — not a merge, which would resurrect them.
- Chart version bumped and, after merge, the publish job actually
  publishes (`SLSA provenance (chart)` must **not** show `skipped`).
  **VERIFIED for BOTH PRs** on merge (2026-09-20). In each case
  `SLSA provenance (chart)` ran `success` rather than `skipped`, and
  the artefact was then pulled back from GHCR — a green Release run is
  precisely the signal this trap counterfeits, so the job status alone
  was not treated as proof.

  | PR | Chart | Digest | appVersion | Tag |
  | --- | --- | --- | --- | --- |
  | #35 | 0.1.9 | `sha256:b05c3f74…` | 0.9.0 | `v0.9.0` |
  | #36 | 0.1.10 | `sha256:f0d742ec…` | 0.10.0 | `v0.10.0` |

  `appVersion` was read out of the *published* chart, not the repo, and
  `ghcr.io/donaldgifford/docz-site:0.10.0` confirmed present — the two
  move together because `tests/deployment_test.yaml` pins the image tag
  to `appVersion`.
- Both PRs' behaviour verified against a real docz-api, not only MSW.
  **VERIFIED** 2026-09-19 against the docz-api local stack, with the
  shipped container (both PRs' code). Beyond the trace join above:
  - Real upstream 401 and a real GitHub OAuth `302` both proxied, the
    latter logged as `location_host="github.com"` with
    `path="/auth/login?provider=<redacted>"` — the redaction rules hold
    on live credential-bearing traffic, not just synthetic fixtures.
  - `/healthz` and `/readyz` produced **zero** log lines at `debug` and
    **zero** upstream requests: `/readyz` makes no docz-api call.
  - A real browse yielded exactly four route-class labels
    (`spa`, `proxy:api`, `proxy:auth`, `proxy:openapi`), unknown SPA
    paths included — the closed set holds outside tests.
  - **The outage-amplification claim (INV-0006 F3) was tested by
    actually killing docz-api.** `/readyz` stayed `200 ready` and the
    SPA kept serving while the API hop returned `502` and
    `docz_site_proxy_errors_total{reason="unreachable"}` incremented.
    A readiness gate on the upstream would have evicted the pod here;
    this is the alertable signal that replaces it, and it is now
    demonstrated rather than argued.
  - `url.full` appeared **zero** times in exported spans; the attribute
    set was exactly the allowlist.

## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `server/route-class.ts` | Create | Closed route-class union + method normalisation |
| `server/redact.ts` | Create | Header/query/Location redaction rules |
| `server/logger.ts` | Create | Levelled structured logger |
| `server/metrics.ts` | Create | prom-client registry and instruments |
| `server/tracing.ts` | Create | OTel provider, sampler, exporter |
| `server/serve.ts` | Modify | Resolvers, `/readyz`, `/metrics`, pipeline wrapper, bound `catch` |
| `server/*.test.ts` | Create | Per-module suites + the redaction gate |
| `src/app/router.tsx` | Modify | Root `errorElement` |
| `src/components/error-boundary.tsx` | Create | Boundary rendering `ErrorPanel` |
| `src/a11y/axe.test.tsx` | Modify | Error-state sweep entry |
| `Dockerfile` | Modify | Bundle step; copy the bundle |
| `charts/docz-site/values.yaml` | Modify | log/metrics/otel/serviceMonitor values |
| `charts/docz-site/values.schema.json` | Modify | Enums for closed sets |
| `charts/docz-site/templates/deployment.yaml` | Modify | New env; readiness → `/readyz` |
| `charts/docz-site/templates/servicemonitor.yaml` | Create | Gated on metrics + serviceMonitor |
| `charts/docz-site/tests/*.yaml` | Modify | Probe path, env, ServiceMonitor gating |
| `charts/docz-site/Chart.yaml` | Modify | Version + appVersion, **once per PR** |
| `CLAUDE.md`, `README.md` | Modify | Guidance and operator docs |

## Testing Plan

- [x] Unit tests for every new pure module (`bun test server/`) — 141
      tests across 10 files.
- [x] The redaction gate, parameterised over all levels, asserting on
      captured stdout — and demonstrated to fail when redaction is
      disabled. The self-check runs in the suite, so the gate cannot
      pass vacuously.
- [x] Span-attribute redaction test mirroring it.
- [x] Cardinality regression test over many hostile paths and methods —
      10 000 paths × 5 methods must stay under 100 series, with a
      companion test proving the guard **fails** on an unbounded label.
- [x] `/readyz` behaviour with dist present and absent, and proof it
      makes no upstream call (`globalThis.fetch` stubbed, `calls === 0`;
      re-confirmed against a live docz-api, which logged no probe hits).
- [x] `/metrics` disabled returns 404, explicitly not the SPA shell.
      Needs a child process: `bun test` shares one module registry, so
      re-importing after setting env would test nothing.
- [x] Route test for the error boundary + axe entry for the error state,
      including a test pinning *why* the boundary sits on a pathless
      route rather than `path: "/"` (owning the root would swap out
      AppShell and take the topbar with it).
- [x] Helm unit tests for every new value, the probe change, and
      ServiceMonitor gating — 48 tests.
- [x] Manual verification against a real docz-api, including one
      end-to-end trace spanning both services — see Phase 8/9 above for
      the span table and the killed-upstream result.

## Dependencies

| Dependency | Phase | Notes |
| --- | --- | --- |
| `prom-client` | 7 | Verified working under Bun (INV-0006 F11) |
| `@opentelemetry/sdk-trace-node` + API + OTLP/HTTP exporter | 8 | Context propagation verified under Bun 1.3.14 (F10) |
| Bundling | 6 | Prerequisite for both — the runtime image carries no `node_modules` |
| docz-api | 8 (verify only) | Already extracts `traceparent`; no upstream change needed |

No blocking external work. Nothing here waits on another repo.

## Open Questions — all resolved

**All six are decided (2026-09-19): every one takes option (a).** They
are kept with their reasoning rather than deleted, so the plan records
why. Each was lettered with **a** as the recommendation.

| OQ | Decision |
| --- | --- |
| OQ-1 request-path testing | **a** — export `handleRequest`, call it with a `Request` |
| OQ-2 chart version | **a** — bump once per PR (0.1.9, then 0.1.10) |
| OQ-3 closes #18 | **a** — PR 1 |
| OQ-4 `/metrics` disabled | **a** — explicit 404, never the SPA shell |
| OQ-5 error recovery | **a** — a link to `/`, no reset button |
| OQ-6 e2e coverage | **a** — none; `bun test server/` plus the jsdom suite |

---

**OQ-1 — How do we exercise the request path in tests? — DECIDED:
(a) export `handleRequest`.**
`handleRequest` is not exported and every existing test in
`server/serve.test.ts` covers pure helpers only — nothing today
constructs a request or binds a port. Phases 3, 7, and 8 all need it.

- **a (recommended)** — Export `handleRequest` and call it directly with
  a `Request`, asserting on the returned `Response`. No port binding, no
  async teardown, and `import.meta.main` already prevents startup on
  import. It is the smallest extension of the existing pattern.
- **b** — Bind `Bun.serve` on port 0 per test and `fetch` it. More
  faithful to production, at the cost of lifecycle management and
  slower tests.
- **c** — Extract routing into `server/router.ts` and test that, leaving
  `serve.ts` as thin wiring. Cleanest separation, largest diff, and it
  moves code that is not otherwise being touched.
- **other** — _______

---

**OQ-2 — One chart version bump or two? — DECIDED: (a) one per PR.**
Each PR changes the chart. The publish job reads `Chart.yaml` `.version`
and skips silently if that version is already published — the trap that
required PR #32.

- **a (recommended)** — Bump once per PR (0.1.9 and 0.1.10). Each PR is
  independently installable, and each release's chart matches its image.
- **b** — Bump only in PR 2. Fewer chart releases, but PR 1's readiness
  probe change would sit unpublished, so a chart user upgrading between
  the two PRs gets an image serving `/readyz` and a chart not using it.
- **other** — _______

---

**OQ-3 — Which PR closes issue #18? — DECIDED: (a) PR 1.**
The issue is specifically about logging.

- **a (recommended)** — PR 1, via `Closes #18`. Its content is delivered
  there in full; metrics and traces are beyond what it asked for.
- **b** — PR 2, so the issue stays open until the whole design ships.
- **other** — _______

---

**OQ-4 — What does `/metrics` return when metrics are disabled? —
DECIDED: (a) explicit 404.**
This one has a trap. If the route is simply not registered, `/metrics`
falls through to the SPA handler and a scraper gets `index.html` with a
`200` — which looks like success and would silently poison a dashboard.

- **a (recommended)** — Explicit `404` with a plain-text body. Correct
  semantics, and unambiguous to a scraper.
- **b** — `200` with an empty exposition body. Scrapers stay "healthy"
  but report nothing, which is arguably worse — it hides the misconfig.
- **c** — Keep `/metrics` always on and drop the flag entirely,
  restricting access at the ingress instead.
- **other** — _______

---

**OQ-5 — Does the error boundary offer a recovery action? — DECIDED:
(a) a link to `/`.**
`ErrorPanel` exists; whether the boundary adds an affordance beyond it
is a UI decision.

- **a (recommended)** — A link to `/` only. `react-router`'s reset
  semantics after a render throw are subtle, and a plain navigation is
  predictable and trivially accessible.
- **b** — A "try again" button re-rendering the failed route. Nicer when
  the error was transient; risks an immediate re-throw loop.
- **c** — Panel only, no action. Smallest change; leaves the user
  relying on the topbar.
- **other** — _______

---

**OQ-6 — Do the new endpoints get e2e coverage? — DECIDED: (a) no
e2e.**
The Playwright suite runs against an MSW preview build, not the Bun
production server, so `/readyz` and `/metrics` are not reachable there
today.

- **a (recommended)** — No e2e. Cover them in `bun test server/`, and
  cover the error boundary in the jsdom suite. Adding a production-server
  e2e target is a meaningful new fixture for two endpoints with no UI.
- **b** — Add a small smoke test that boots the real server on a port
  and curls both endpoints, as a separate `just` recipe outside
  Playwright.
- **c** — Extend the e2e harness to run the real server. Most faithful,
  most infrastructure.
- **other** — _______

## References

- [DESIGN-0006](../design/0006-server-observability-structured-logging-split-probes-prometheus.md) — what this implements
- [INV-0006](../investigation/0006-server-observability-logging-health-probes-metrics-and-the-otel.md) — the measured premises
- [Issue #18](https://github.com/donaldgifford/docz-site/issues/18) — the trigger
- `server/serve.ts` — the subject
- `Dockerfile` — the no-`node_modules` constraint driving Phase 6
- docz-api `internal/telemetry/` — the reference implementation
