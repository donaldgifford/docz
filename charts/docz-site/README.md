# docz-site

Helm chart for docz-site

## What this deploys

`docz-site` is the web UI for
[docz-api](https://github.com/donaldgifford/docz) — a cross-repo docz
reader, search directory, and repo pages. It ships as a single-page app
served by a small Bun process that also reverse-proxies the API surface
(`/api`, `/auth`, `/webhooks`, `/openapi.yaml`) to docz-api, so the browser
and API share **one origin**: no CORS, and the httpOnly `docz_session`
cookie is first-party. Everything is served on a single HTTP port
(`config.port`, default `8080`), including the `/healthz` probe endpoint.

The chart renders a `Deployment`, a `Service`, and a `ServiceAccount`, with
optional `Ingress`, Gateway API `HTTPRoute`, and `HorizontalPodAutoscaler`.
The site holds **no secrets** — authentication is entirely docz-api's, over
the session cookie — so there is no Secret in this chart.

The image runs on a read-only rootfs as the `oven/bun` image's non-root
`bun` user (UID/GID 1000).

## Installation

The chart is published as an OCI artifact to GHCR:

```bash
helm install docz-site \
  oci://ghcr.io/donaldgifford/charts/docz-site \
  --version 0.1.10 \
  --namespace docz-site \
  --create-namespace \
  --set config.doczApiUrl=http://docz-api:8080
```

## Prerequisites

- Kubernetes 1.28+
- Helm 3.14+ or 4.x (OCI support)
- A reachable **docz-api** in the cluster (or elsewhere) for
  `config.doczApiUrl` to point at. Deploy it with the
  [docz-api chart](https://github.com/donaldgifford/docz/tree/main/charts/docz-api).

## Configuration

Minimal `values.yaml`:

```yaml
config:
  # In-cluster docz-api Service. Required — the API proxy 502s without it.
  doczApiUrl: "http://docz-api:8080"
```

### The API proxy contract

`config.doczApiUrl` is the single required value. The site process proxies
`/api`, `/auth`, `/webhooks`, and `/openapi.yaml` to it verbatim (OAuth 302s
are passed through, not followed server-side). Point it at the in-cluster
docz-api `Service` so the two share an origin.

### Login providers

`config.authProviders` (comma-separated: `github`, `okta`, `keycloak`)
chooses the login buttons shown on `/login`. The server injects it into the
SPA at runtime (whitelist-validated), so one image serves any combo — no
rebuild. It must match docz-api's own `AUTH_PROVIDERS`, and docz-api owns
the actual OAuth/OIDC exchange plus the GitHub App ingest ("machine
identity"), which is independent of the login provider. Empty or unknown
values fall back to `github`.

```yaml
config:
  authProviders: "keycloak,github" # Keycloak login; GitHub App ingest is docz-api's
```

### Nav pins

`config.navLinks` pins up to six deployment-chosen links into the site
topbar (between Repos and the session menu). The list renders into
`DOCZ_NAV_LINKS` as JSON and the server whitelist-validates every entry
(short label charset, same-origin app-path hrefs); invalid entries
degrade to fewer or no pins — never a broken page. Leave it empty for a
pin-free topbar.

```yaml
config:
  navLinks:
    - label: "RFCs"
      href: "/donaldgifford/rfcs"
    - label: "Team Docs"
      href: "/donaldgifford/docs/docs"
```

### Observability

Logging is structured JSON on stdout by default. `config.logLevel`
takes `debug`, `info`, `warn`, or `error`; `debug` adds a line per
request plus the proxied `/auth/*` flow, which is the level to reach
for when troubleshooting an Okta or Keycloak login. Credential-bearing
values (`code`, `state`, cookies) are redacted at every level.

The probes are deliberately split, because Kubernetes responds to them
differently. `livenessProbe` stays on `/healthz`, which is
unconditional — a failing liveness probe **restarts** the container,
and restarting cannot fix a broken image. `readinessProbe` points at
`/readyz`, which verifies the built assets are servable and the runtime
config validated — a failing readiness probe **holds traffic**, so a
bad deploy stalls the rollout and the previous ReplicaSet keeps
serving. `/readyz` makes no call to docz-api by design: gating
readiness on the API would evict every pod from the Service the moment
it blipped.

`metrics.enabled` (default `true`) exposes Prometheus metrics on
`/metrics`. When disabled the endpoint returns an explicit `404`
rather than falling through to the SPA, so a scraper is told the
endpoint is absent instead of being handed `index.html` with a `200`.
Set `serviceMonitor.enabled: true` for a Prometheus Operator
`ServiceMonitor` — it is gated on **both** flags.

```yaml
metrics:
  enabled: true
serviceMonitor:
  enabled: true
  interval: 30s
  labels:
    release: kube-prometheus-stack
```

Alongside the four `docz_site_*` instruments, `prom-client`'s default
process and runtime metrics are exported. One caveat worth knowing
before you build dashboards: **`nodejs_gc_duration_seconds` is declared
but never samples**, because the server runs on Bun rather than Node
and Bun does not emit the GC performance entries that metric is fed
from. A stock Node.js dashboard will show empty GC panels. Every other
`nodejs_*` metric (event-loop lag, heap size, handles) does report.

## Exposure

The site is a `ClusterIP` Service by default. Front it with one of:

- **Ingress** — set `ingress.enabled: true` with `ingress.hosts` (and
  `ingress.tls` for HTTPS).
- **Gateway API HTTPRoute** — set `httpRoute.enabled: true` with
  `httpRoute.parentRefs` and `httpRoute.hostnames`; an empty `rules` list
  renders a default rule to the Service.

## Scaling

Enable `autoscaling.enabled: true` for a `HorizontalPodAutoscaler` (CPU
target by default; add `autoscaling.targetMemoryUtilizationPercentage` for a
memory metric). The SPA server is stateless, so horizontal scaling is safe.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity rules |
| autoscaling | object | `{"enabled":false,"maxReplicas":3,"minReplicas":1,"targetCPUUtilizationPercentage":80,"targetMemoryUtilizationPercentage":0}` | Horizontal Pod Autoscaler. Off by default. |
| autoscaling.enabled | bool | `false` | Enable a HorizontalPodAutoscaler |
| autoscaling.maxReplicas | int | `3` | Maximum replicas |
| autoscaling.minReplicas | int | `1` | Minimum replicas |
| autoscaling.targetCPUUtilizationPercentage | int | `80` | Target average CPU utilization (percent) |
| autoscaling.targetMemoryUtilizationPercentage | int | `0` | Target average memory utilization (percent). Unset → no memory metric. |
| config | object | `{"authProviders":"github","doczApiUrl":"","logFormat":"json","logLevel":"info","mermaidLayout":"elk","navLinks":[],"port":8080}` | docz-site runtime configuration. The site is a static SPA served by a small Bun process that also reverse-proxies the API surface, so the browser and API share one origin (no CORS, first-party session cookie). |
| config.authProviders | string | `"github"` | Comma-separated login providers to show on /login (DOCZ_AUTH_PROVIDERS): github, okta, keycloak. The server injects this into the SPA at runtime (whitelist-validated), so one image serves any combo — no rebuild. Must match docz-api's own AUTH_PROVIDERS. Empty/unknown → github. The GitHub App ingest ("machine identity") is docz-api's and is independent of this. |
| config.doczApiUrl | string | `""` | Absolute base URL of the docz-api the site proxies to (DOCZ_API_URL). In-cluster this is the docz-api Service, e.g. http://docz-api:8080. Required — without it the API proxy returns 502. |
| config.logFormat | string | `"json"` | Log output format (DOCZ_LOG_FORMAT): `json` (one object per line, for a log aggregator) or `text` (readable, for local runs). |
| config.logLevel | string | `"info"` | Log verbosity (DOCZ_LOG_LEVEL): `debug`, `info`, `warn`, or `error`. `debug` adds a line per request plus the proxied `/auth/*` flow — the level to reach for when troubleshooting an Okta or Keycloak login. Credential-bearing values (`code`, `state`, cookies) are redacted at every level, enforced by a test. The values schema constrains this to the four names so a typo fails at install; at runtime an unrecognized level falls back to `info`, never to something noisier. |
| config.mermaidLayout | string | `"elk"` | Diagram layout engine (DOCZ_MERMAID_LAYOUT): `elk` or `dagre`. mermaid 12 made ELK the default and so is it here; `dagre` restores the pre-12 layout without rebuilding the image. Injected into the SPA at runtime and whitelist-validated at both ends, so anything unrecognized falls back to `elk` — the values schema constrains it to the two names so a typo fails at install rather than silently rendering the default. |
| config.navLinks | list | `[]` | Topbar nav pins (DOCZ_NAV_LINKS): a list of `{label, href}` links rendered between Repos and the session menu. Injected into the SPA at runtime as JSON, whitelist-validated by the server (short label charset, same-origin app-path hrefs, cap 6); invalid entries degrade to fewer/no pins, never a broken page. Empty → no pins and the env var is omitted. |
| config.port | int | `8080` | Container HTTP listen port (drives the PORT env var and the Service targetPort). The SPA, /healthz, /readyz, and the API proxy are all served here. |
| extraEnv | list | `[]` | Additional environment variables |
| extraVolumeMounts | list | `[]` | Additional volume mounts |
| extraVolumes | list | `[]` | Additional volumes |
| fullnameOverride | string | `""` | Override the full release name |
| httpRoute | object | `{"annotations":{},"enabled":false,"hostnames":[],"parentRefs":[],"rules":[]}` | Gateway API HTTPRoute. Off by default. |
| httpRoute.annotations | object | `{}` | HTTPRoute annotations |
| httpRoute.enabled | bool | `false` | Enable an HTTPRoute |
| httpRoute.hostnames | list | `[]` | Hostnames matched by this route. |
| httpRoute.parentRefs | list | `[]` | parentRefs (Gateways) this route attaches to. |
| httpRoute.rules | list | `[]` | Route rules. Empty → the template renders a default rule to the service. |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| image.repository | string | `"ghcr.io/donaldgifford/docz-site"` | Container image repository |
| image.tag | string | `""` | Overrides the image tag (default: chart appVersion) |
| imagePullSecrets | list | `[]` | Image pull secrets |
| ingress | object | `{"annotations":{},"className":"","enabled":false,"hosts":[],"tls":[]}` | Ingress (networking.k8s.io/v1). Off by default. |
| ingress.annotations | object | `{}` | Ingress annotations |
| ingress.className | string | `""` | IngressClass name |
| ingress.enabled | bool | `false` | Enable an Ingress |
| ingress.hosts | list | `[]` | Ingress hosts. Each entry: {host, paths: [{path, pathType}]}. |
| ingress.tls | list | `[]` | TLS blocks. Each entry: {secretName, hosts: []}. |
| livenessProbe | object | `{"httpGet":{"path":"/healthz","port":"http"},"initialDelaySeconds":5,"periodSeconds":15}` | Liveness probe. Stays on /healthz, which is unconditional: a failing liveness probe RESTARTS the container, and restarting cannot fix a broken image or mount — that is just CrashLoopBackOff. |
| metrics.enabled | bool | `true` | Expose Prometheus metrics on /metrics (DOCZ_METRICS_ENABLED). When false the endpoint returns an explicit 404 rather than falling through to the SPA — a scraper is told the endpoint is absent instead of being handed index.html with a 200. |
| nameOverride | string | `""` | Override the chart name |
| nodeSelector | object | `{}` | Node selector |
| otel.endpoint | string | `""` | OTLP/HTTP traces endpoint (OTEL_EXPORTER_OTLP_ENDPOINT), e.g. http://otel-collector:4318/v1/traces. EMPTY MEANS TRACING IS OFF — no provider is registered, no network call is attempted, and the env var is omitted entirely. Only absolute http(s) URLs are accepted; anything else is treated as unset rather than guessed at, because this decides where request telemetry is sent. |
| otel.sampleRate | int | `1` | Head sample rate (OTEL_TRACES_SAMPLER_ARG), clamped to 0..1. docz-site injects `traceparent` on the proxy hop and docz-api already extracts it, so a sampled request produces ONE trace spanning both services with no docz-api configuration. |
| otel.serviceName | string | `"docz-site"` | Service name reported to the collector (OTEL_SERVICE_NAME) |
| podAnnotations | object | `{}` | Pod annotations |
| podLabels | object | `{}` | Pod labels |
| podSecurityContext | object | `{"fsGroup":1000,"runAsGroup":1000,"runAsNonRoot":true,"runAsUser":1000,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod security context. Defaults match the `oven/bun` runtime image, whose `bun` user is UID/GID 1000, and drop to a RuntimeDefault seccomp profile. |
| readinessProbe | object | `{"httpGet":{"path":"/readyz","port":"http"},"initialDelaySeconds":5,"periodSeconds":10}` | Readiness probe. Points at /readyz, which verifies the dist/ is servable and the runtime config validated — a failing readiness probe HOLDS TRAFFIC, so a bad deploy stalls the rollout and the previous ReplicaSet keeps serving. /readyz makes no call to docz-api by design: gating readiness on the API would evict every pod from the Service when the API blipped. |
| replicaCount | int | `1` | Number of replicas |
| resources | object | `{"limits":{"cpu":"250m","memory":"128Mi"},"requests":{"cpu":"25m","memory":"64Mi"}}` | Container resource requests and limits |
| revisionHistoryLimit | int | `3` | Number of old ReplicaSets retained for rollback. Defaults to 3 to keep the kubectl `get rs` view tidy; bump if you need more rollback headroom. Kubernetes default is 10. |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Container security context. The static server reads `dist/` and proxies; it writes nothing to disk, so the rootfs is mounted read-only. |
| service.port | int | `80` | Service port. Targets the container's single `http` port, which serves the SPA, /healthz, and the same-origin API proxy. |
| service.type | string | `"ClusterIP"` | Service type |
| serviceAccount.annotations | object | `{}` | Annotations for the ServiceAccount |
| serviceAccount.create | bool | `true` | Create a ServiceAccount |
| serviceAccount.name | string | `""` | Override the ServiceAccount name |
| serviceMonitor.enabled | bool | `false` | Create a Prometheus Operator ServiceMonitor scraping /metrics. Requires `metrics.enabled`; the template is gated on both. |
| serviceMonitor.interval | string | `"30s"` | Scrape interval |
| serviceMonitor.labels | object | `{}` | Additional labels for the ServiceMonitor (e.g. the `release` label your Prometheus Operator selects on) |
| tolerations | list | `[]` | Tolerations |

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| donaldgifford |  |  |
