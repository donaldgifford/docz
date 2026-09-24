---
id: INV-0006
title: "Server observability — logging, health probes, metrics, and the OTel-vs-Prometheus split"
status: Concluded
author: Donald Gifford
created: 2026-09-18
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0006: Server observability — logging, health probes, metrics, and the OTel-vs-Prometheus split

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [F1 — The server has one log line, and the 502 path throws its cause away](#f1--the-server-has-one-log-line-and-the-502-path-throws-its-cause-away)
  - [F2 — /healthz exists but is a liveness probe wired to both slots](#f2--healthz-exists-but-is-a-liveness-probe-wired-to-both-slots)
  - [F3 — There is no /readyz, and the obvious /readyz would amplify outages](#f3--there-is-no-readyz-and-the-obvious-readyz-would-amplify-outages)
  - [F4 — There is no metrics endpoint and no observability dependency at all](#f4--there-is-no-metrics-endpoint-and-no-observability-dependency-at-all)
  - [F5 — docz-api already runs the split, and it already accepts our trace context](#f5--docz-api-already-runs-the-split-and-it-already-accepts-our-trace-context)
  - [F6 — This app is two runtimes, and only one of them is the server](#f6--this-app-is-two-runtimes-and-only-one-of-them-is-the-server)
  - [F7 — Auto-instrumentation is a redaction hazard on exactly our hottest path](#f7--auto-instrumentation-is-a-redaction-hazard-on-exactly-our-hottest-path)
  - [F8 — SPA paths are unbounded, so naive route labels explode cardinality](#f8--spa-paths-are-unbounded-so-naive-route-labels-explode-cardinality)
  - [F9 — The browser has no error handling to report from](#f9--the-browser-has-no-error-handling-to-report-from)
  - [F10 — OTel context propagation works under Bun (measured)](#f10--otel-context-propagation-works-under-bun-measured)
  - [F11 — prom-client works under Bun; browser OTel does not fit the budget](#f11--prom-client-works-under-bun-browser-otel-does-not-fit-the-budget)
- [Options](#options)
- [Open questions — resolved](#open-questions--resolved)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

## Question

Issue #18 asks a narrow question — how should `server/serve.ts` log, at
what levels, in what format, with what redaction. This investigation
widens it to the whole operational surface and one architectural
decision:

1. **Logging.** What does `server/serve.ts` need to emit so that an
   IdP misconfiguration is debuggable from the site container alone?
2. **Probes.** Is today's `/healthz` sufficient, and should a `/readyz`
   exist? If so, what may it legitimately check?
3. **Metrics.** Should the site expose a metrics endpoint at all, and
   what would actually be on it that docz-api does not already report?
4. **The architectural question.** Do we do all four telemetry signals
   (logs, metrics, traces, and browser RUM) through **OpenTelemetry**,
   or do we **split** — Prometheus for metrics, OTel for traces, plain
   structured stdout for logs — the way docz-api already does?

Question 4 is the one that constrains the others, so it is the real
subject. Questions 1–3 are answered differently depending on how it
lands.

## Hypothesis

The split wins for the server, and the all-OTel case is stronger in
theory than in our practice. Specifically I expect to find that:

- the split is not a new decision but an **existing house standard**,
  since docz-api already runs Prometheus metrics alongside OTLP traces,
  and divergence would cost more than it buys;
- the marginal value of server-side *metrics* here is **low**, because
  this process is a static file server and a reverse proxy, and
  docz-api already meters the far side of that proxy;
- the marginal value of server-side *logs* is **high**, because the
  proxy hop is the one place in the OAuth journey that nothing else can
  see — which is precisely issue #18's complaint;
- the genuinely unserved surface is the **browser**, and that is where
  OTel's story is weakest for us, because of the bundle budget.

If that holds, the answer to "OTel or Prometheus" is "both, split by
signal" — but the more useful finding would be that the question is
scoped wrong, and the real fork is *server vs browser*, not *OTel vs
Prometheus*.

## Context

**Triggered by:** [issue #18](https://github.com/donaldgifford/docz-site/issues/18)
(“Server has no logging — wire up `DOCZ_LOG_LEVEL` with a debug mode
for IdP troubleshooting”), whose own follow-up checkbox asks for this
document before any implementation. Scope extended at the user's
request to cover a metrics endpoint, `/healthz`, `/readyz`, and the
OTel-vs-Prometheus split.

The immediate operational pain is inherited from
[DESIGN-0003](../design/0003-support-docz-apis-no-auth-mode-and-the-session-unavailable-503.md)
and issue #17: configuring Okta or Keycloak means debugging an OAuth
round trip that transits this proxy, and `AUTH_REDIRECT_BASE` mistakes
bite precisely at the hop that is currently invisible.

## Approach

1. Read `server/serve.ts` end to end and inventory every observable
   emission and every silently discarded error.
2. Inventory the probe surface: what the server answers, what the chart
   wires, and what a probe failure would actually do to traffic.
3. Check the dependency manifest for any existing telemetry library.
4. Read docz-api's `internal/telemetry` package and chart to establish
   the house precedent and — critically — whether it would *accept*
   trace context from us without changes.
5. Reason about the two runtimes separately (Bun server, browser SPA)
   rather than treating "the app" as one thing.
6. Identify the constraints that kill options outright: the bundle
   budget, the redaction rule, and metric cardinality.
7. Record what still needs measuring rather than asserting it.

Steps 1–6 are done and recorded below. Step 7 is the open-questions
list; nothing there is claimed as fact.

## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz-site | `main` @ `1cd7887` (v0.8.0, chart 0.1.8) |
| Server runtime | Bun (`Bun.serve`), `server/serve.ts`, 335 lines |
| Observability deps | **none** — no OTel, Prometheus, pino, winston |
| Eager bundle | ~122.5 KB gz against a 130 KB budget (`scripts/bundle-budget.ts:24`) |
| docz-api | `/Users/donaldgifford/code/docz-api`, `internal/telemetry` |
| docz-api metrics | `prometheus/client_golang` v1.22.0 |
| docz-api traces | `go.opentelemetry.io/otel` v1.44.0, OTLP/HTTP |

## Findings

### F1 — The server has one log line, and the 502 path throws its cause away

Confirmed as issue #18 describes. `server/serve.ts` has exactly one
`console.log`, the startup banner at line 328, guarded by
`import.meta.main`. There is no request logging, no proxy logging.

The sharpest instance is the proxy catch block:

```ts
} catch {
  return new Response("upstream unreachable", { status: 502 });
}
```

`server/serve.ts:223-225`. The `catch` binds nothing, so the error — DNS
failure, connection refused, TLS error, timeout — is not merely unlogged
but **unrecoverable**, discarded at the language level. An operator
seeing `502 upstream unreachable` in a browser has no way to learn which
of those it was from inside the container.

There is a second, quieter one: `proxy()` returns
`502 DOCZ_API_URL is not configured` (line 203) — a configuration fault
that looks identical to a network fault from the outside, and is equally
silent inside.

This is the highest-value, lowest-cost fix in the whole investigation,
and it does not depend on the architectural question at all.

### F2 — `/healthz` exists but is a liveness probe wired to both slots

The endpoint is real (`server/serve.ts:298-302`) and returns `ok`
unconditionally with `cache-control: no-store`. It checks nothing — it
reports only that the process is scheduling requests, which is the
correct semantic for *liveness*.

The chart, however, points **both** probes at it:

```yaml
livenessProbe.httpGet.path:  /healthz
readinessProbe.httpGet.path: /healthz
```

(`charts/docz-site/tests/deployment_test.yaml:58-65`, asserted, so this
is intentional and pinned.) The practical effect is that readiness is
currently a synonym for liveness. That is defensible today precisely
because there is nothing else to check — but it means the site
advertises readiness the instant the process boots, including before
`dist/` is verified present. `serveIndex()` returns
`500 dist/index.html missing` (line 274), a condition a readiness probe
*could* legitimately catch and currently does not.

### F3 — There is no `/readyz`, and the obvious `/readyz` would amplify outages

There is no `/readyz` route. The request was for one that goes green
"when all connections are up, like api comm is working" — and this is
the place where I think the requested design is actively harmful, so I
want to make the argument explicitly rather than quietly implementing
something narrower.

**The site is designed to work while docz-api is down.** That is not an
accident; it is
[DESIGN-0003](../design/0003-support-docz-apis-no-auth-mode-and-the-session-unavailable-503.md),
implemented and shipped in v0.5.0. `classifySession` has a dedicated
`unavailable` state for exactly this; a 503 from the session endpoint is
explicitly *never* a logout; `SessionMenu` renders an inert placeholder
and re-polls every ~30 s; `SessionUnavailableError` is documented in
`CLAUDE.md` as "transient, NEVER a logout". We built a careful,
tested degradation path for docz-api being unreachable.

Now consider a `/readyz` that checks docz-api reachability. docz-api
goes down. Every docz-site pod fails its readiness probe. Kubernetes
removes every pod from the Service endpoints. The Ingress has nothing to
route to. Users get a connection failure or a 503 from the ingress
controller — **instead of** the designed UI that says the session is
temporarily unavailable and quietly recovers when the API returns.

A partial degradation we handle gracefully becomes a total outage, and
the blast radius grows from one feature to the whole site. The static
SPA, the reader shell, cached content, and the login page were all still
serveable.

docz-api's own codebase makes this argument, in a comment I did not
expect to find:

> `/readyz` deliberately checks serving dependencies only (GitHub being
> down must not pull the read API out of rotation).
>
> — `internal/githubapp/selfcheck.go:37`

The same reasoning applies one layer up. docz-site's *serving*
dependency is `dist/`, not docz-api. docz-api is a **downstream**
dependency of a *feature*, not of serving.

This does not mean API reachability should be unobservable — it means
the right surface is a **metric or a log, not a probe**. Something that
pages a human without evicting the pod. A possible middle path is a
`/readyz` that checks only serving dependencies (dist present, config
resolved) and reports API reachability in its *body* as informational,
without letting it set the status code; that is captured in OQ-3.

### F4 — There is no metrics endpoint and no observability dependency at all

`package.json` contains no OTel, Prometheus, pino, winston, or bunyan
dependency, direct or transitive-by-intent. The chart has no
ServiceMonitor (docz-api's chart has one; ours does not). The only
`metrics` string anywhere in our chart is in `hpa.yaml`, which is
Kubernetes resource metrics, unrelated.

So this is genuinely greenfield — there is nothing to migrate, and no
sunk cost pulling in either direction. Whatever we choose, we choose
from zero.

It is worth being honest about what server-side metrics would contain.
This process does three things: serve static files from disk, fall back
to `index.html`, and proxy to docz-api. For the proxy path — the only
interesting one — **docz-api already meters every request from the other
side**, with better labels than we could produce, because it has the
matched route template (`/api/v1/repos/{owner}/{name}`) and we have only
an opaque prefix. Our proxy metrics would substantially double-count
docz-api's, minus the routing detail.

What we would add that docz-api genuinely cannot see:

- requests that **never reached** docz-api (the F1 502s — connection
  failures, DNS, config faults);
- the **proxy hop's own** added latency;
- static asset and SPA-fallback traffic, which docz-api never sees at
  all;
- cache-hit shape (`.gz` sibling served vs not).

That is a real but modest list, and it argues for a *small, hand-picked*
instrument set rather than broad auto-instrumentation.

### F5 — docz-api already runs the split, and it already accepts our trace context

This is the most decision-relevant finding. docz-api is not
observability-naive; it is a worked reference implementation of exactly
the split under consideration:

| Signal | docz-api's choice | Surface |
| ------ | ----------------- | ------- |
| Metrics | **Prometheus** (`client_golang`, `promauto`) | `/metrics`, scraped by a ServiceMonitor |
| Traces | **OpenTelemetry** | OTLP/HTTP, `OTEL_EXPORTER_OTLP_ENDPOINT` |
| Logs | **`log/slog`**, structured, to stdout | `LOG_LEVEL`, `LOG_FORMAT` (text\|json) |

Note that `go.opentelemetry.io/otel/metric` appears in `go.mod` only as
an *indirect* dependency — docz-api deliberately does **not** use OTel
for metrics. The split is a choice, not an accident of history.

Three details are directly reusable:

**It degrades to nothing when unconfigured.** From
`internal/telemetry/telemetry.go:7-12`: with no OTLP endpoint the global
tracer stays the OTel no-op, "so a homelab install without a collector
pays no overhead and needs no telemetry configuration." Any design here
should inherit that property — this project's users include
single-operator homelab installs.

**It already extracts W3C trace context.** `Setup()` installs a
composite `TraceContext`/`Baggage` propagator unconditionally — before
the endpoint check, so even with tracing disabled — and `Instrument()`
calls `propagator.Extract` on every request
(`internal/telemetry/middleware.go:77`). **Consequence: if docz-site
injects a `traceparent` header on its proxy hop, docz-api will join the
trace with zero changes on its side.** End-to-end traces across the
proxy are available for the cost of the client half alone. That is a
genuinely strong argument for OTel traces specifically.

**Probe paths are excluded from logs and traces.** `skipPaths` covers
`/healthz`, `/readyz`, `/metrics` — "high-frequency, low-signal, and
self-referential for `/metrics`"
(`internal/telemetry/middleware.go:29-34`). We should mirror this; a
kubelet probing every 10 s would otherwise dominate our log volume,
which for a low-traffic internal docs site could be *most* of it.

Env naming precedent to match: `LOG_LEVEL`, `LOG_FORMAT`,
`METRICS_ENABLED`, `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`,
`OTEL_SAMPLE_RATE`. Note issue #18 proposes `DOCZ_LOG_LEVEL`; docz-api
uses bare `LOG_LEVEL`. Our own runtime config is consistently
`DOCZ_`-prefixed (`DOCZ_API_URL`, `DOCZ_AUTH_PROVIDERS`,
`DOCZ_NAV_LINKS`, `DOCZ_MERMAID_LAYOUT`) — but the `OTEL_*` names are
an ecosystem standard that SDKs read automatically and should **not** be
prefixed. That tension needs a decision (OQ-6).

### F6 — This app is two runtimes, and only one of them is the server

The framing "observability for docz-site" hides that there are two
independent processes with different constraints:

| | Bun server | Browser SPA |
| --- | --- | --- |
| What it does | static files + reverse proxy | the entire product |
| Who sees failures | operator | user |
| Instrument cost | dependency size irrelevant | **counts against 130 KB budget** |
| Export path | direct to collector | must egress via our origin |
| Current state | 1 log line | nothing at all |

Almost everything a *user* experiences — render failures, a mermaid
diagram that throws, a route that 404s, slow search, the reader
crashing on a malformed doc — happens in the browser and is invisible to
both the server and docz-api. The server sees a 200 for `index.html` and
nothing more.

So the server work (issue #18) and browser observability are **separate
decisions with separate constraints**, and I think bundling them under
one "observability" umbrella is what makes the OTel-vs-Prometheus
question feel harder than it is. Prometheus is not a browser telemetry
option at all; OTel's browser SDK is, but see the budget constraint in
OQ-4. For the server the two are genuine alternatives; for the browser
the comparison never applies.

### F7 — Auto-instrumentation is a redaction hazard on exactly our hottest path

Issue #18 states redaction is non-negotiable: `code`/`state` query
params and all cookie headers are credential-bearing.

The proxy forwards `/auth/*` verbatim, including the OAuth callback
carrying `?code=…&state=…`, and copies **every** request header —
`new Headers(req.headers)` at `server/serve.ts:206` — which includes the
`docz_session` cookie. It also returns upstream headers verbatim
(line 221), including `set-cookie`.

Standard OTel HTTP auto-instrumentation records `url.full` on client
spans by default. Pointed at this proxy, it would ship OAuth
authorization codes to the collector as span attributes, and depending
on configuration, headers too. The same hazard applies to a naive
request logger that prints `req.url`.

This cuts against auto-instrumentation and toward **hand-written
instrumentation with an allowlist** — record `url.path`, never
`url.search`; never record headers; emit a redacted form
(`/auth/callback?code=<redacted>&state=<redacted>`) if the query string
is wanted at all. An allowlist, not a denylist: a denylist fails open
the first time a provider adds a parameter.

This also constrains the "all-OTel" option meaningfully. All-OTel's main
practical draw is that you get instrumentation for free; on our most
security-sensitive path, free instrumentation is the thing we must not
have.

### F8 — SPA paths are unbounded, so naive route labels explode cardinality

Every SPA route falls through to `serveIndex()`. Real paths include
`/donaldgifford/docz-site/design/DESIGN-0001` and
`/donaldgifford/docz-site/pages/guides/markdown-specimen.md`. Path is
therefore unbounded in owner × repo × type × doc-id — thousands of
values against a live docz-api (the v0.7.0 deployment reported 856
documents), and attacker-controllable besides, since any URL a browser
requests becomes a label value.

A `path` label would be a cardinality bomb in Prometheus and an
expensive one in any trace backend. Labels must be **route classes**
computed before the value is recorded, not raw paths — the same
discipline docz-api gets from chi's `RoutePattern()` and its
`unmatchedRoute` fallback. A workable bucketing for this server:

`asset` (`/assets/` immutable) · `static` (other real file) ·
`spa` (index.html fallback) · `proxy:api` · `proxy:auth` ·
`proxy:webhooks` · `proxy:openapi` · `probe`

Eight values, bounded by construction, and they map exactly onto the
branches `handleRequest` already takes.

### F9 — The browser has no error handling to report from

`src/app/router.tsx` defines no `errorElement`, there is no React error
boundary anywhere in `src/`, and `src/main.tsx` installs no
`window.onerror` or `unhandledrejection` handler.

So a render throw inside a route today unmounts to a blank page with a
console message, and nothing anywhere records it. Before any browser
telemetry can be *exported*, there has to be something *catching*. That
is a prerequisite, and it has standalone user-facing value independent
of this investigation — arguably it should be split out as its own
piece of work regardless of what we decide here (OQ-8).

### F10 — OTel context propagation works under Bun (measured)

OQ-2 is answered, by experiment rather than by reading. Bun 1.3.14,
`@opentelemetry/sdk-trace-node` 2.11.0.

The failure mode this guards against is silent: spans still get created,
but a child started after an `await` detaches from its parent and every
trace becomes a pile of orphaned roots. So the spike asserts *linkage*,
not merely that spans exist.

```text
{ "bun": "1.3.14", "spanCount": 2, "parentFound": true,
  "childFound": true, "sameTrace": true, "childLinkedToParent": true }
RESULT: PASS
```

A second spike exercised the ergonomic path real code would take —
`NodeTracerProvider.register()`, which installs the AsyncLocalStorage
context manager and the W3C propagator without naming either — and
confirmed the header we would put on the proxy hop is produced and
well-formed:

```text
traceparent: 00-36e1c1a1ada75466049b114f912b0a10-23f15ff8885f834c-01
RESULT: PASS
```

Combined with F5 (docz-api already extracts `traceparent`), end-to-end
tracing across the proxy costs us only the client half and needs no
upstream change. This removes the main risk that would have argued
against an OTel-bearing option.

### F11 — prom-client works under Bun; browser OTel does not fit the budget

**Server-side metrics: viable.** `prom-client` 15.1.3 runs under Bun.
Custom counters and histograms export correct exposition text, and
`collectDefaultMetrics` yields 26 metric families (~5.5 KB) with
`process_*` and most `nodejs_*` populated for free.

One gap worth knowing: `nodejs_gc_duration_seconds` registers a HELP
line but produces **no samples** under Bun, as do the non-`_total`
variants of `nodejs_active_handles`/`resources`/`requests`. Bun does not
expose Node's GC hooks. Nothing we would alert on, but a dashboard
copied from a Node service will show gaps.

**Browser OTel: does not fit.** Measured, rather than guessed as the
first draft of this document did. `@opentelemetry/sdk-trace-web` +
`exporter-trace-otlp-http` + `instrumentation-fetch`, bundled for the
browser and minified:

| | Size |
| --- | --- |
| Minified | 74.8 KB |
| **Gzipped** | **23.4 KB** |
| Current eager bundle | 122.5 KB gz |
| Budget | 130 KB gz |
| **Headroom** | **7.5 KB gz** |

23.4 KB against 7.5 KB of headroom — **3.1× over**. Browser OTel cannot
be an eager import. It could still be lazy-loaded, since the budget
measures the entry chunk plus its modulepreload closure and a deferred
import falls outside both; that is a real option, not a dead end, but it
is a deliberate decision rather than a detail.

## Options

**Option A — All-OTel.** Logs, metrics, and traces from both runtimes as
OTLP to a collector; the collector fans out to Prometheus, Loki, Tempo.
One SDK, one config, one wire protocol, vendor-portable.
*Against:* requires a collector to exist before anything is observable,
which breaks the homelab-degrades-to-nothing property (F5); the OTel JS
logs API is the least mature of the three signals; pulls a
comparatively heavy dependency into a process whose entire job is
serving files; and the auto-instrumentation that justifies it is the
part we must disable (F7). Prometheus users still need the collector's
exporter, so "one system" is partly an illusion in our topology.

**Option B — Split, mirroring docz-api.** Prometheus `/metrics` +
OTel traces over OTLP + structured stdout logs.
*For:* identical operational shape to docz-api — one scrape config, one
collector, one log convention across both services; each signal degrades
independently; trace context already interoperates (F5). *Against:* two
dependencies and two config surfaces; some conceptual duplication.

**Option C — Logs and probes only.** Structured stdout logging plus
`/healthz` and a serving-only `/readyz`. No metrics endpoint, no traces.
*For:* answers issue #18 completely and directly; smallest possible
change; no new runtime dependency at all; F4 shows server-side metrics
are genuinely thin here. *Against:* no latency distribution, no scrape
target, no end-to-end trace — and the proxy hop is the only place an
end-to-end trace could ever be stitched.

**Option D — Split the decision by runtime.** Adopt Option B or C for
the server on its own merits, and treat browser observability as a
separate, later decision gated on the bundle budget and on F9's
prerequisite work.

These are not fully exclusive: D is a meta-choice that composes with B
or C, and C is a proper subset of B — C now does not foreclose B later,
provided log field names are chosen with metric labels in mind.

## Open questions — resolved

All nine are closed: OQ-2 and OQ-4 by experiment (F10, F11), the rest by
decision on 2026-09-18. Recorded here as the inputs DESIGN-0006 builds
on.

| OQ | Resolution |
| --- | --- |
| OQ-1 staging | **Overruled — one PR.** I recommended shipping logs first and deciding on metrics later; the decision is to land the whole observability surface in a single PR and version. See the note below. |
| OQ-2 OTel under Bun | **Works** — measured, F10. Context survives `await`; `register()` and `traceparent` injection both verified. |
| OQ-3 `/readyz` scope | **Serving-only**, accepted as "a small addition for low failure coverage". Never gates on docz-api. |
| OQ-4 browser OTel cost | **23.4 KB gz against 7.5 KB headroom** — measured, F11. Cannot be eager. |
| OQ-5 browser telemetry now? | Deferred; carried into DESIGN-0006 as an open question with options. |
| OQ-6 env naming | `DOCZ_*` for ours, bare `OTEL_*` for the SDK-native names. |
| OQ-7 log format | Carried into DESIGN-0006 as an open question. |
| OQ-8 error boundary | Carried into DESIGN-0006 as an open question (scope). |
| OQ-9 `traceparent` without a tracer | Moot — F10 makes the real tracer cheap, so the fallback is unnecessary. |

**On OQ-1.** My staged recommendation was overruled with a rationale
worth recording, because it is a better reading of the users than mine
was: the split is only annoying if you assume everyone runs both
backends. In practice logs are needed regardless and go to stdout; a
deployment with no Prometheus and no collector simply gets nothing extra
and configures nothing; a deployment with a forwarder already owns that
plumbing and it is not our config surface. That argument makes the
metrics half cheap enough that staging buys little, and one coherent
chart surface beats two partial ones.

## Conclusion

**Answer: the split — Option B. Not all-OTel.**

Concretely, for the Bun server: **Prometheus for metrics** (`/metrics`,
`prom-client`), **OpenTelemetry for traces** (OTLP/HTTP), **structured
stdout for logs**. This is docz-api's shape, and the two empirical risks
that could have overturned it are now measured and did not (F10, F11).

All-OTel is rejected on three grounds: it requires a collector to exist
before anything at all is observable, which breaks the
degrades-to-nothing property a single-operator install depends on (F5);
its main practical draw is free auto-instrumentation, which is precisely
what we must disable on the proxy path (F7); and Prometheus users would
still need a collector exporter, so the "one system" simplification is
partly illusory in our topology.

Two corrections to the question as posed matter more than the choice
itself:

**First, "OTel or Prometheus" is not one question, it is two runtimes.**
For the Bun server they are genuine alternatives and the split wins,
mostly on F5: docz-api has already made this choice, already exposes
`/metrics` for Prometheus, and already accepts our trace context without
modification. For the browser the comparison never applies — Prometheus
is not a browser option, and OTel's browser SDK is gated on a budget
question nobody has measured (OQ-4). Deciding "all-OTel" globally would
be deciding the browser question by accident.

**Second, the requested `/readyz` semantics would make outages worse,
not more visible.** F3 is the finding I most want reviewed. Gating
readiness on docz-api reachability would convert a degradation the app
was deliberately built to survive — DESIGN-0003, shipped, tested — into
a total site outage, and docz-api's own code contains the counter-
argument for its own layer.

Two further findings hold regardless of which option is chosen: the
redaction hazard (F7) rules out naive auto-instrumentation on the proxy
path whatever the SDK, and route-class labels (F8) are mandatory before
any metric or span records a path.

## Recommendation

Land the whole server observability surface in **one PR and one
version** (OQ-1 as decided, overriding this document's original staged
proposal):

1. **Structured stdout logging.** Level from env, probe paths skipped
   (F5), allowlist redaction (F7). Bind that `catch` (F1) — two lines,
   and the single highest-value fix here.
2. **Split the probes.** Keep `/healthz` unconditional for liveness; add
   a serving-only `/readyz` and point the chart's readiness probe at it
   (F2, F3). Liveness and readiness fail differently: a missing `dist/`
   must hold traffic, not restart the container into CrashLoopBackOff.
3. **Prometheus `/metrics`** via `prom-client` (F11), route-class labels
   only (F8), behind an enable flag, with a chart ServiceMonitor
   mirroring docz-api's.
4. **OTel traces** over OTLP/HTTP, hand-instrumented rather than
   auto (F7), injecting `traceparent` on the proxy hop so traces stitch
   to docz-api for free (F5, F10). No collector configured means no
   export and no overhead.
5. **Browser observability stays a separate decision**, now with a
   number attached: 23.4 KB gz against 7.5 KB of headroom (F11). Carried
   into DESIGN-0006 as scoped open questions rather than assumed in.

Next artifact: **DESIGN-0006**, covering the logging surface, probe
semantics, metric set and labels, trace spans, redaction rules, env and
chart plumbing, and the test strategy.

## References

- [Issue #18 — Server has no logging](https://github.com/donaldgifford/docz-site/issues/18) (trigger)
- [Issue #17 — context for the IdP debugging pain](https://github.com/donaldgifford/docz-site/issues/17)
- [DESIGN-0003 — no-auth mode and the session-unavailable 503](../design/0003-support-docz-apis-no-auth-mode-and-the-session-unavailable-503.md) — the degradation path F3 protects
- `server/serve.ts` — the subject
- `charts/docz-site/templates/deployment.yaml`, `tests/deployment_test.yaml` — probe wiring (F2)
- `scripts/bundle-budget.ts` — the 130 KB constraint (OQ-4)
- docz-api `internal/telemetry/{telemetry,metrics,middleware}.go` — the split, worked (F5)
- docz-api `internal/githubapp/selfcheck.go:37` — the readiness argument (F3)
- docz-api `charts/docz-api/templates/servicemonitor.yaml` — scrape precedent
- [W3C Trace Context](https://www.w3.org/TR/trace-context/) — the propagation format docz-api already extracts
