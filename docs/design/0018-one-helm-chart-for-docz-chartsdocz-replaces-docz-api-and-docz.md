---
id: DESIGN-0018
title: "One Helm chart for docz: charts/docz replaces docz-api and docz-site"
status: Draft
author: Donald Gifford
created: 2026-09-25
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0018: One Helm chart for docz: charts/docz replaces docz-api and docz-site

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [1. Layout](#1-layout)
  - [2. Names, labels, and selectors](#2-names-labels-and-selectors)
  - [3. Values](#3-values)
  - [4. Wiring the site to the API](#4-wiring-the-site-to-the-api)
  - [5. The edge](#5-the-edge)
  - [6. Helpers](#6-helpers)
  - [7. From two charts to one, file by file](#7-from-two-charts-to-one-file-by-file)
  - [8. Schema, README, and changelog](#8-schema-readme-and-changelog)
  - [9. Publishing](#9-publishing)
  - [10. Recipes and CI](#10-recipes-and-ci)
  - [11. Retiring the old charts](#11-retiring-the-old-charts)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. What is the chart called?](#1-what-is-the-chart-called)
  - [2. What version does the chart start at?](#2-what-version-does-the-chart-start-at)
  - [3. Where do the backing services live in values?](#3-where-do-the-backing-services-live-in-values)
  - [4. How are resources named?](#4-how-are-resources-named)
  - [5. Which workloads can be turned off?](#5-which-workloads-can-be-turned-off)
  - [6. Is DOCZAPIURL overridable?](#6-is-doczapiurl-overridable)
  - [7. Where do the shared settings live?](#7-where-do-the-shared-settings-live)
  - [8. What does the edge point at?](#8-what-does-the-edge-point-at)
  - [9. Where does the Tailscale sidecar go?](#9-where-does-the-tailscale-sidecar-go)
  - [10. One ServiceAccount or two?](#10-one-serviceaccount-or-two)
  - [11. Does component go into the Deployments' matchLabels?](#11-does-component-go-into-the-deployments-matchlabels)
  - [12. How does the chart publish?](#12-how-does-the-chart-publish)
  - [13. Where do the chart's just recipes live?](#13-where-do-the-charts-just-recipes-live)
  - [14. How are the unit tests laid out?](#14-how-are-the-unit-tests-laid-out)
  - [15. When are the old chart directories deleted?](#15-when-are-the-old-chart-directories-deleted)
  - [16. Does the first cut add a site alert?](#16-does-the-first-cut-add-a-site-alert)
  - [17. Is authRedirectBase derived from the edge?](#17-is-authredirectbase-derived-from-the-edge)
- [References](#references)
<!--toc:end-->

<!--docz:overview:start-->
## Overview

INV-0014 concluded that docz-api and docz-site should ship as one Helm chart.
It resolved the shape as a **single merged chart** (1b), not an umbrella; the
two published charts are **deprecated** (2a); and existing releases are **not
migrated** (3c). This design specifies that chart, `charts/docz`:

- its layout and names;
- its values, which put each workload in its own block, give the backing
  services one shared home, and hold the settings both workloads need in one
  place;
- how it derives the wiring between the site and the API that a user writes
  by hand today;
- where outside traffic enters;
- how it is tested and published;
- how `charts/docz-api` and `charts/docz-site` are retired.

Nothing inside either container changes. Every environment variable the
binaries read today is rendered from the new values, and the image tags are
the ones `v2.0.0-beta.4` already publishes.

<!--docz:overview:end-->

## Goals and Non-Goals

<!--docz:goals:start-->
### Goals

- **One install.** `helm install docz oci://ghcr.io/donaldgifford/charts/docz`
  gives a working product: the API, the site, and baked Postgres, Valkey, and
  Meilisearch.
- **One version.** A single `appVersion` defaults both images. The version
  skew INV-0014 Observation 1 found cannot occur.
- **Wiring derived, not typed.** The site's `DOCZ_API_URL` comes from the
  release's own API Service. The login-provider list and the tracing
  endpoint are each set once and reach both workloads.
- **Feature parity.** Everything `charts/docz-api` 0.9.0 and
  `charts/docz-site` 0.2.0 can render, the new chart can render:
  - the backend modes (`baked`/`cnpg`/`external` for Postgres,
    `baked`/`external` for Valkey and Meilisearch);
  - every login provider and `none`;
  - `existingSecret`;
  - the Tailscale sidecar;
  - both ServiceMonitors, the PrometheusRule, HPA, Ingress, and HTTPRoute.
- **Coverage kept.** The 145 unit tests (96 + 49) are ported, not dropped,
  and new tests pin the wiring this design adds.
- **Same supply chain.** The chart is signed with cosign and carries build
  provenance through the existing `ghcr.yml` path.

<!--docz:goals:end-->

<!--docz:non-goals:start-->
### Non-Goals

- **Migrating existing releases** (INV-0014 3c). A docz-api or docz-site
  release keeps working on its old chart until someone reinstalls with the
  new one. No adoption or data-move procedure is written or tested.
- **An umbrella chart** (INV-0014 1a, rejected).
- **Changing either binary** or the name of any environment variable.
- **New runtime features**, such as a site alert or pod disruption budgets.
  Open Question 16 asks whether a site alert belongs in the first cut.
- **Enabling ECR publishing.** It stays behind `vars.ECR_PUBLISH_ENABLED`;
  this design only keeps `ecr.yml` in step with `ghcr.yml`.

<!--docz:non-goals:end-->

<!--docz:background:start-->
## Background

The two charts today, as INV-0014 measured them:

| | `charts/docz-api` | `charts/docz-site` |
| --- | --- | --- |
| Version | `0.9.0`, `appVersion: 2.0.0-beta.3` | `0.2.0`, `appVersion: 2.0.0-beta.4` |
| `values.yaml` | 503 lines | 234 lines |
| Templates | 22 + a `helm test` hook | 8 + a `helm test` hook |
| Helpers | 23 (`docz-api.*`) | 6 (`docz-site.*`), a subset |
| Unit tests | 96 across 10 suites | 49 across 6 suites |
| State | 3 StatefulSets with PVCs (baked) | none |
| Front door | own Ingress/HTTPRoute; Tailscale sidecar with Funnel | own Ingress/HTTPRoute |
| Needs from the other | nothing | `config.doczApiUrl` (required), matching `authProviders` |

Four facts from the investigation shape this design:

1. **The site is the front door.** `ui/server/route-class.ts` proxies
   `/api`, `/auth`, `/webhooks`, and `/openapi.yaml` to the API, so the
   browser and GitHub can reach everything through the site's origin. The
   API does not need its own edge.
2. **Selector overlap is a known hazard.** The API chart's pods share
   `name`/`instance` labels with its baked backends, and chart 0.2.2 added
   `app.kubernetes.io/component: server` to the pod template and Service
   selector to stop the Service enrolling Meilisearch. That fix could not go
   into `spec.selector.matchLabels`, which is immutable, so it lives only in
   the Service. Because the new chart is new installs only, it can put
   `component` into every selector from the start.
3. **The Tailscale sidecar exposes the API with Funnel.** Its serve config
   proxies `/` to the API container and sets `AllowFunnel`. That is how a
   homelab install receives GitHub webhooks from the public internet.
4. **The pipeline is per component.** `ghcr.yml` and `ecr.yml` resolve
   `component: api|ui` into an image, a bake target, and a chart. The chart
   job is idempotent (`helm pull` precheck), signed, and attested.

<!--docz:background:end-->

<!--docz:detailed-design:start-->
## Detailed Design

### 1. Layout

```text
charts/docz/
├── Chart.yaml               name: docz, one appVersion
├── values.yaml              §3
├── values.schema.json       merged, permissive (additionalProperties: true)
├── README.md.gotmpl         → README.md via helm-docs
├── CHANGELOG.md             chart changes only
├── cliff.toml               --include-path 'charts/docz/**'
├── ci/ci-values.yaml        busybox for both workloads, a dummy per required value
├── templates/
│   ├── _helpers.tpl         shared: name, fullname, labels, selectors (§2)
│   ├── _api.tpl             the 17 backend/secret/auth helpers, renamed docz.api.*
│   ├── NOTES.txt
│   ├── api-deployment.yaml  api-service.yaml  api-serviceaccount.yaml
│   ├── api-secret.yaml      api-hpa.yaml      api-servicemonitor.yaml
│   ├── api-prometheusrule.yaml
│   ├── site-deployment.yaml site-service.yaml site-serviceaccount.yaml
│   ├── site-hpa.yaml        site-servicemonitor.yaml
│   ├── ingress.yaml         httproute.yaml    (one edge, §5)
│   ├── tailscale-configmap.yaml tailscale-rbac.yaml
│   ├── store-postgres.yaml  store-postgres-secret.yaml
│   ├── store-cnpg-cluster.yaml store-cnpg-pooler.yaml store-cnpg-pooler-service.yaml
│   ├── queue-valkey.yaml    queue-valkey-secret.yaml
│   ├── search-meili.yaml    search-meili-secret.yaml search-meili-servicemonitor.yaml
│   └── tests/test-connection.yaml   one hook, checks both /healthz
└── tests/
    ├── api/…_test.yaml      the 10 docz-api suites, ported
    ├── site/…_test.yaml     the 6 docz-site suites, ported
    └── chart/…_test.yaml    new: wiring, names, selectors, edge
```

Backend templates keep their file names. Only the workload templates gain an
`api-`/`site-` prefix, so a reader can see which workload a file belongs to.

### 2. Names, labels, and selectors

`docz.fullname` is the standard Helm rule over chart name `docz`: a release
called `docz` renders `docz`, and a release called `prod` renders
`prod-docz`. Each workload and backend then takes a suffix:

| Resource | Name | `app.kubernetes.io/component` |
| --- | --- | --- |
| API Deployment, Service, ServiceAccount, Secret, HPA, ServiceMonitor | `<fullname>-api` | `api` |
| Site Deployment, Service, ServiceAccount, HPA, ServiceMonitor | `<fullname>-site` | `site` |
| Baked Postgres StatefulSet, Service, Secret | `<fullname>-postgres` | `postgres` |
| CNPG Cluster (and `-app` Secret it writes) | `<fullname>-postgres` | `postgres` |
| Baked Valkey | `<fullname>-valkey` | `valkey` |
| Baked Meilisearch | `<fullname>-meilisearch` | `meilisearch` |
| Ingress / HTTPRoute | `<fullname>` | the edge target's |
| Tailscale serve ConfigMap, Role, state Secret | `<fullname>-tailscale-…` | the edge target's |

Every resource carries the common labels (`helm.sh/chart`,
`app.kubernetes.io/name: docz`, `instance`, `version`, `managed-by`) plus its
`component`. **Every selector includes `component`**: both Deployments'
`spec.selector.matchLabels`, every Service's `selector`, and every
ServiceMonitor's `matchLabels`. `docz.selectorLabels` takes the component as
an argument, so a selector without one does not render:

```yaml
{{- define "docz.selectorLabels" -}}
app.kubernetes.io/name: {{ include "docz.name" .ctx }}
app.kubernetes.io/instance: {{ .ctx.Release.Name }}
app.kubernetes.io/component: {{ required "docz.selectorLabels: component" .component }}
{{- end }}
```

It is called as `include "docz.selectorLabels" (dict "ctx" $ "component" "api")`.
`docz.labels` and `docz.componentFullname` take the same `dict`. This
settles the hazard from Background fact 2 once for all six workloads rather
than one at a time.

### 3. Values

The values file has four kinds of block:

- **one block per workload** (`api`, `site`): everything that is about one
  Deployment;
- **backing services** (`store`, `queue`, `search`): top level, keeping their
  current shape;
- **shared settings** (`auth`, `otel`, `metrics`, `serviceMonitor`): set once
  and read by both workloads;
- **the edge** (`ingress`, `httpRoute`, `tailscale`): one set, pointed at the
  front door.

```yaml
nameOverride: ""
fullnameOverride: ""
imagePullSecrets: []

auth:
  providers: "github"            # → AUTH_PROVIDERS (api) and DOCZ_AUTH_PROVIDERS (site)

otel:
  endpoint: ""                   # empty = tracing off in both workloads
  sampleRate: 1
  # serviceName is per workload: api.otel.serviceName / site.otel.serviceName

metrics:
  enabled: true                  # both /metrics endpoints
serviceMonitor:
  enabled: false                 # one per workload, plus Meilisearch's
  interval: 30s
  labels: {}
prometheusRule:
  enabled: false                 # API alerts, as today
  labels: {}

api:
  replicaCount: 1
  revisionHistoryLimit: 3
  image: { repository: ghcr.io/donaldgifford/docz-api, pullPolicy: IfNotPresent, tag: "" }
  serviceAccount: { create: true, annotations: {}, name: "" }
  podAnnotations: {}
  podLabels: {}
  podSecurityContext: { runAsUser: 65532, … }     # unchanged defaults
  securityContext: { readOnlyRootFilesystem: true, … }
  service: { type: ClusterIP, port: 80 }
  resources: { … }                                # unchanged defaults
  livenessProbe: { … }
  readinessProbe: { … }
  config:                        # every docz-api config key except authProviders
    appId: ""
    port: 8080
    logLevel: info
    logFormat: json
    authRedirectBase: ""
    githubOAuthClientID: ""
    oktaIssuer: ""               # …and the rest of the provider keys, unchanged
    githubApiBase: ""
    sessionTTL: ""
    ingestDebounce: ""
  otel: { serviceName: "docz-api" }
  secrets: { create: true, existingSecret: "", … }   # unchanged shape
  autoscaling: { enabled: false, … }
  extraEnv: []
  extraVolumes: []
  extraVolumeMounts: []
  nodeSelector: {}
  tolerations: []
  affinity: {}

site:
  enabled: true
  replicaCount: 1
  revisionHistoryLimit: 3
  image: { repository: ghcr.io/donaldgifford/docz-site, pullPolicy: IfNotPresent, tag: "" }
  serviceAccount: { create: true, annotations: {}, name: "" }
  # …the same per-workload keys as api…
  config:
    port: 8080
    doczApiUrl: ""               # empty = derived (§4)
    mermaidLayout: elk
    navLinks: []
    logLevel: info
    logFormat: json
  otel: { serviceName: "docz-site" }

store:  { backend: postgres, postgres: { mode: baked, … }, external: { … } }   # unchanged
queue:  { backend: valkey,   valkey:   { mode: baked, … }, external: { … } }   # unchanged
search: { meili: { mode: baked, … } }                                         # unchanged

ingress:   { enabled: false, className: "", annotations: {}, hosts: [], tls: [] }
httpRoute: { enabled: false, annotations: {}, parentRefs: [], hostnames: [], rules: [] }
tailscale: { enabled: false, hostname: docz, … }    # unchanged shape but hostname
```

Both images default their tag to `.Chart.AppVersion`, so one bump moves both.
A user carrying values over from the old charts nests them as in the Data
Model table, and the only keys that move out of a workload are
`authProviders` (to `auth.providers`) and the OTel endpoint and sample rate
(to `otel`).

### 4. Wiring the site to the API

```mermaid
flowchart LR
  V["values.yaml"] --> AP["auth.providers"]
  V --> OT["otel.endpoint / sampleRate"]
  V --> SU["site.config.doczApiUrl"]
  AP -->|AUTH_PROVIDERS| API["api Deployment"]
  AP -->|DOCZ_AUTH_PROVIDERS| SITE["site Deployment"]
  OT -->|OTEL_EXPORTER_OTLP_ENDPOINT| API
  OT -->|OTEL_EXPORTER_OTLP_ENDPOINT| SITE
  SU -->|"set: used verbatim"| SITE
  SU -->|"empty: http://&lt;fullname&gt;-api.&lt;ns&gt;.svc:&lt;api.service.port&gt;"| SITE
```

- **`DOCZ_API_URL`** renders from `site.config.doczApiUrl` when set, and
  otherwise from `docz.api.internalUrl`:
  `http://<fullname>-api.<namespace>.svc.cluster.local:<api.service.port>`.
  The `required` check is gone. The override stays, for a site pointed at an
  API outside the release (Open Question 6).
- **Login providers** are one list. `docz.api.authProviders` and
  `docz.api.authDisabled` read `auth.providers`, and the site gets the same
  string. With `none`, the site shows no providers, which is what the API
  serves.
- **Tracing** is on for both workloads or for neither. Each workload keeps
  its own `OTEL_SERVICE_NAME`, so traces still name the hop.
- **`AUTH_REDIRECT_BASE`** stays `api.config.authRedirectBase`, required
  unless `auth.providers` is `none`. Its comment now says it must be the
  site's public URL, because the site proxies `/auth/callback`
  (Open Question 17).

### 5. The edge

```mermaid
flowchart LR
  U["Browser"] --> E
  GH["GitHub webhooks"] --> E
  subgraph E["Edge (one of)"]
    I["Ingress / HTTPRoute<br/>&lt;fullname&gt;"]
    T["Tailscale sidecar<br/>Serve + Funnel"]
  end
  E --> SS["Service &lt;fullname&gt;-site"]
  SS --> SP["site pod<br/>SPA · /healthz · /readyz · /metrics<br/>proxy /api /auth /webhooks /openapi.yaml"]
  SP --> AS["Service &lt;fullname&gt;-api"]
  AS --> AP["api pod"]
  AP --> PG["&lt;fullname&gt;-postgres"]
  AP --> VK["&lt;fullname&gt;-valkey"]
  AP --> MS["&lt;fullname&gt;-meilisearch"]
```

There is **one** `ingress` block and **one** `httpRoute` block, and both
target the **front door**: the site Service when `site.enabled`, otherwise
the API Service. `docz.edgeTarget` returns that component, and the edge
templates take their backend name and port from it. The API's Service stays
`ClusterIP` and gets no edge of its own. An API-only install
(`site.enabled: false`) still has an edge, because the target falls back to
the API.

The HTTPRoute renders a default rule to the target when `rules` is empty.
That was docz-site's behaviour. docz-api's template rendered a route with no
rules, which attaches nothing, and is not carried forward.

**Tailscale** attaches its sidecar to the front-door pod, and its serve
config proxies to that pod's `config.port`. With the site in front, Funnel
exposes the site, the site proxies `/webhooks` to the API, and GitHub
delivery keeps working without the API being published. The RBAC Role and
state Secret move with the sidecar. `tailscale.hostname` defaults to `docz`
(Open Question 9).

### 6. Helpers

`_helpers.tpl` holds only what every template needs:

- `docz.name`, `docz.fullname`, `docz.chart`;
- `docz.componentFullname` (`dict ctx component`);
- `docz.labels` and `docz.selectorLabels` (`dict ctx component`);
- `docz.serviceAccountName` (`dict ctx component`), which reads
  `.Values.<component>.serviceAccount`;
- `docz.edgeTarget`;
- `docz.image` (`dict ctx component`), which renders
  `repository:tag|default AppVersion`.

`_api.tpl` holds the 17 helpers that describe the API's dependencies. Each
is renamed from `docz-api.X` to `docz.api.X`, with the same body except that
`.Values.config.authProviders` becomes `.Values.auth.providers` and fullname
calls go through `docz.fullname`:

- `secretName`;
- `postgresFullname`, `valkeyFullname`, `meiliFullname`;
- `storeSecretName`, `storeSecretKey`;
- `queueSecretName`, `queueSecretKey`;
- `valkeyPasswordSecretName`, `valkeyPasswordSecretKey`, `valkeyBakedDsn`;
- `meiliHost`, `searchSecretName`, `searchSecretKey`;
- `authProviders`, `authDisabled`;
- `tailscaleStateSecret`.

It also gains one new helper, `internalUrl`, for §4.

The two charts' `-name`/`-fullname`/`-chart`/`-labels`/`-selectorLabels`/
`-serviceAccountName` helpers are duplicates of one another and collapse
into the shared set.

### 7. From two charts to one, file by file

| Today | In `charts/docz` |
| --- | --- |
| `docz-api/templates/deployment.yaml` | `api-deployment.yaml`; the Tailscale container and volumes move to whichever deployment is the edge target, via a `docz.tailscale.container` helper both deployments include |
| `docz-site/templates/deployment.yaml` | `site-deployment.yaml`, gated on `site.enabled` |
| both `service.yaml` | `api-service.yaml`, `site-service.yaml` |
| both `serviceaccount.yaml` | `api-serviceaccount.yaml`, `site-serviceaccount.yaml` |
| both `hpa.yaml` (identical bar names) | `api-hpa.yaml`, `site-hpa.yaml` over one `docz.hpa` helper |
| both `servicemonitor.yaml` | `api-servicemonitor.yaml`, `site-servicemonitor.yaml` |
| both `ingress.yaml` (identical bar names) | one `ingress.yaml` → `docz.edgeTarget` |
| both `httproute.yaml` | one `httproute.yaml`, docz-site's default-rule behaviour |
| `docz-api/templates/secret.yaml` | `api-secret.yaml`, keys unchanged |
| `docz-api/templates/prometheusrule.yaml` | `api-prometheusrule.yaml`; `up{job=…}` names `<fullname>-api` |
| `store-*`, `queue-*`, `search-*` | same names, helpers renamed |
| `tailscale-configmap.yaml`, `tailscale-rbac.yaml` | same names, keyed to the edge target |
| both `NOTES.txt` | one; prints the front-door URL, and what is still required |
| both `templates/tests/test-connection.yaml` | one hook pod that `wget`s both Services' `/healthz` |

### 8. Schema, README, and changelog

- **`values.schema.json`** merges the two schemas under their new paths, and
  keeps their permissive style (`additionalProperties: true` everywhere). It
  also keeps every existing enum: the three backend `mode`s, both workloads'
  `logLevel`/`logFormat`, and `site.config.mermaidLayout`. `auth.providers`
  gets a pattern that allows only `github`, `okta`, `keycloak`, and `none`,
  comma-separated.
- **`README.md`** is generated by `helm-docs` from `README.md.gotmpl`. The
  template is written fresh: install, the edge, backend modes, login
  providers, Tailscale, and a section titled "Coming from docz-api or
  docz-site" that points at the Data Model table and says no in-place
  upgrade exists.
- **`CHANGELOG.md`** starts at the chart's first version, and `cliff.toml`
  scopes git-cliff to `charts/docz/**`.

### 9. Publishing

```mermaid
flowchart TB
  TAG["tag v2.0.0-beta.N"] --> PR["prerelease.yml"]
  PR --> GR["goreleaser: docz_*, docz-api_* archives"]
  PR --> GA["ghcr.yml component: api<br/>image docz-api"]
  PR --> GU["ghcr.yml component: ui<br/>image docz-site"]
  PR --> GC["ghcr.yml component: chart<br/>chart charts/docz"]
  GC --> SIGN["cosign sign + attest-build-provenance"]
```

`ghcr.yml` and `ecr.yml` gain a third `component`, `chart`, whose resolve row
has no image and a chart directory of `charts/docz`. The `image` job is gated
on the row having an image, and the `chart` job on the row having a chart.
`prerelease.yml` and `release.yml` each call it once more
(`publish-chart`, `publish-ecr-chart`) under the same permissions ceiling.
The `api` and `ui` rows keep their chart columns only for the release that
publishes the deprecated final versions (§11), then lose them.

The OCI address is `oci://ghcr.io/donaldgifford/charts/docz`, a new GHCR
package. Like every package before it, it needs its own Actions access grant
before the first publish, or the push fails `403 write_package`.

### 10. Recipes and CI

- **just.** The chart is neither the server's nor the frontend's, so its
  recipes leave `api.just` and `ui.just`. They go to a new optional module,
  `chart.just` (`mod? chart`): `just chart lint`, `template`, `unittest`, and
  `docs`, each scoped to `charts/docz`. The root `ci` gate replaces
  `api::helm-lint` with `chart::lint` (Open Question 13).
- **helm-unittest** runs as
  `helm unittest -f 'tests/**/*_test.yaml' charts/docz`, because the suites
  sit in subdirectories (Open Question 14). CI's `for chart in charts/*/`
  loop needs no change while the old charts exist, apart from passing that
  glob for `charts/docz`.
- **chart-testing.** `ct lint` and `ct install` on kind pick the new chart
  up from `ct.yaml`'s `chart-dirs: [charts]` with no edit. `ci/ci-values.yaml`
  runs busybox for both workloads and keeps the ≥16-byte Meilisearch
  dummy key `ct install` needs.
- **Path filter.** The `helm` filter already covers `charts/**`.

### 11. Retiring the old charts

```mermaid
flowchart LR
  A["beta.N<br/>charts/docz 0.1.0 first publish<br/>docz-api 0.10.0 + docz-site 0.3.0<br/>deprecated: true"] --> B["betas after N<br/>charts/docz only"]
  B --> C["v2.0.0<br/>charts/docz-api and charts/docz-site<br/>directories deleted"]
```

In the release that first publishes `charts/docz`, each old chart publishes
one final version: `docz-api` 0.10.0 and `docz-site` 0.3.0. That version:

- sets `deprecated: true` in `Chart.yaml`, which Helm and Artifact Hub
  surface;
- sets `appVersion` to that release, which fixes INV-0014's skew on the way
  out;
- opens its `NOTES.txt` and README with the move to `charts/docz`, and says
  there is no in-place upgrade.

After that release the old directories stop changing and are not published
again. They are deleted at the v2.0.0 cut, along with the `api`/`ui` chart
columns, `api.just`/`ui.just`'s helm recipes, and every doc line that names
them. The published versions stay in GHCR.

<!--docz:detailed-design:end-->

<!--docz:api-changes:start-->
## API / Interface Changes

The interface a user sees is the chart address and its values. The
containers' interface does not change.

| Surface | Before | After |
| --- | --- | --- |
| Install | two `helm install`s, site after API | `helm install docz oci://ghcr.io/donaldgifford/charts/docz` |
| Chart address | `charts/docz-api`, `charts/docz-site` | `charts/docz`; the old two deprecated |
| Resource names | `<release>-docz-api`, `<release>-docz-site`, `<release>-docz-api-postgres`… | `<fullname>-api`, `<fullname>-site`, `<fullname>-postgres`… (§2) |
| `DOCZ_API_URL` | required, hand-written | derived; override optional |
| Login providers | set twice, must match | `auth.providers`, once |
| Edge | one per chart | one, to the front door |
| Tailscale | API pod | front-door pod |
| Alert `DoczAPIDown` | `up{job="<release>-docz-api"}` | `up{job="<fullname>-api"}` |
| Environment variables | — | unchanged in name and meaning |

<!--docz:api-changes:end-->

<!--docz:data-model:start-->
## Data Model

Where each old key goes. Anything not listed moves under the workload's
block unchanged: for example, docz-api's `resources` becomes
`api.resources`, and docz-site's `affinity` becomes `site.affinity`.

| `charts/docz-api` key | `charts/docz` key |
| --- | --- |
| `replicaCount`, `revisionHistoryLimit`, `image`, `serviceAccount`, `pod*`, `securityContext`, `service`, `resources`, `*Probe`, `extra*`, `nodeSelector`, `tolerations`, `affinity` | `api.<same>` |
| `imagePullSecrets`, `nameOverride`, `fullnameOverride` | top level, shared |
| `config.authProviders` | `auth.providers` |
| `config.*` (every other key) | `api.config.*` |
| `otel.endpoint`, `otel.sampleRate` | `otel.endpoint`, `otel.sampleRate` |
| `otel.serviceName` | `api.otel.serviceName` (default `docz-api`) |
| `secrets.*` | `api.secrets.*` |
| `autoscaling.*` | `api.autoscaling.*` |
| `metrics`, `serviceMonitor`, `prometheusRule` | top level, shared |
| `store`, `queue`, `search` | top level, unchanged |
| `tailscale.*` | `tailscale.*`; `hostname` defaults to `docz` |
| `ingress`, `httpRoute` | top level, targets the front door |

| `charts/docz-site` key | `charts/docz` key |
| --- | --- |
| per-workload keys, as above | `site.<same>` |
| `config.authProviders` | `auth.providers` |
| `config.doczApiUrl` | `site.config.doczApiUrl`, optional |
| `config.*` (every other key) | `site.config.*` |
| `otel.endpoint`, `otel.sampleRate` | `otel.*` |
| `otel.serviceName` | `site.otel.serviceName` (default `docz-site`) |
| `autoscaling.*` | `site.autoscaling.*` |
| `metrics`, `serviceMonitor` | top level, shared |
| `ingress`, `httpRoute` | top level |

<!--docz:data-model:end-->

<!--docz:testing:start-->
## Testing Strategy

**Ported suites.** All 16 suites move under `tests/api/` and `tests/site/`,
with `set:` paths rewritten to the new keys and expected names rewritten to
§2. A ported suite keeps its assertions. A test that no longer applies,
such as docz-api's HTTPRoute rendering zero rules, is deleted with a comment
in its commit saying why. It is never edited until it passes. The target is
at least 145 tests across the ported suites before the new ones are counted.

**New suites under `tests/chart/`:**

- **wiring**:
  - `DOCZ_API_URL` renders the in-cluster API URL when unset and the
    override when set;
  - `auth.providers` reaches both `AUTH_PROVIDERS` and `DOCZ_AUTH_PROVIDERS`;
  - an empty `otel.endpoint` omits the tracing variables from both
    workloads;
- **selectors**: every Deployment `matchLabels`, Service `selector`, and
  ServiceMonitor `matchLabels` carries a `component`, and no two workloads
  share one;
- **edge**:
  - Ingress, HTTPRoute, and the Tailscale sidecar target the site;
  - with `site.enabled: false` they target the API, and no site resource
    renders;
- **version**: both images default to `.Chart.AppVersion`, which matches
  bare semver (the existing guard, now over both images).

**Render parity**, run once during implementation and recorded in the IMPL,
not kept. Render the old charts and the new one with equivalent values: each
`ci-values.yaml`, plus a CNPG case, an all-external case, a Tailscale case,
and a `none` auth case. Normalise resource names to the §2 scheme and diff
every container spec, environment variable, volume, and Secret key. The
only differences allowed are names, labels, `DOCZ_API_URL`, the edge target,
and the Tailscale host. This check is what shows the port kept feature
parity.

**Install.** `ct install` on kind with `ci/ci-values.yaml`. Once, by hand,
install the real images with `auth.providers: none` and a port-forward, and
confirm that `/api/v1/repos` answers through the site. That is the proxy
round trip IMPL-0020 left open, and the derived `DOCZ_API_URL` is what makes
it work without typing a URL.

**Gate.** `just ci`, which now runs `chart::lint`, plus CI's
`helm-unittest` and `helm-test`.

<!--docz:testing:end-->

<!--docz:rollout:start-->
## Migration / Rollout Plan

There is no migration for existing releases (INV-0014 3c). The rollout is
about the repository and the registry:

1. Build `charts/docz` beside the old charts. Neither old chart changes
   until step 3.
2. Wire `component: chart` into `ghcr.yml`/`ecr.yml`, then into
   `prerelease.yml` and `release.yml`. The first `release.yml` run after the
   merge would publish `charts/docz` from `main`, because the chart job
   publishes any version not yet in GHCR. That is acceptable only after the
   GHCR grant exists, so **the grant for `charts/docz` is made before this
   merges** (human).
3. In the same PR, publish `docz-api` 0.10.0 and `docz-site` 0.3.0 as
   `deprecated: true`, with `appVersion` set to the next beta.
4. Cut the next beta by hand, as every v2 beta is. Check the release:
   - `helm show chart oci://ghcr.io/donaldgifford/charts/docz` reports the
     beta's bare `appVersion`;
   - `cosign tree` shows a signature and provenance;
   - both old charts report `deprecated: true`.
5. Update README, DEVELOPMENT, CONTRIBUTING, CLAUDE.md, `deploy/api/README.md`,
   and `ui/CLAUDE.md` to name `charts/docz`.
6. At the v2.0.0 cut, delete `charts/docz-api/` and `charts/docz-site/`, the
   `api`/`ui` chart columns, and their recipes.

<!--docz:rollout:end-->

<!--docz:open-questions:start-->
## Open Questions

> Each question is numbered. Option `a` is my recommendation, the later
> letters are alternatives, and the last is "other" for your own answer.

### 1. What is the chart called?

- a. **`docz`** at `charts/docz`, published as
  `oci://ghcr.io/donaldgifford/charts/docz`. It is named for the product,
  it is short, and it matches the repository.
- b. `docz-platform` or `docz-stack`, which says "the whole thing" but is a
  name nothing else uses.
- c. Keep `docz-api` and fold the site into it. This avoids a new GHCR
  package, but the name no longer describes the chart.
- d. Other.

### 2. What version does the chart start at?

- a. **`0.1.0` now, `1.0.0` at the v2.0.0 cut.** Chart and app versions
  stay independent, as both charts do today, and `1.0.0` marks the values
  interface as stable.
- b. Track the app: `2.0.0-beta.N` on each beta. This is simple to read,
  but a chart-only fix then needs an app version.
- c. `1.0.0` now.
- d. Other.

### 3. Where do the backing services live in values?

- a. **Top level (`store`, `queue`, `search`), shape unchanged.** They are
  the product's infrastructure rather than the API's settings, and an
  existing values file copies across with no nesting.
- b. Under `api` (`api.store`…), because only the API uses them. This is
  truer to ownership but deepens every path.
- c. Other.

### 4. How are resources named?

- a. **`<fullname>-api`, `<fullname>-site`, `<fullname>-postgres`…**
  (§2). The names are symmetric, and each one says what it is.
- b. The API takes the bare `<fullname>` and only the site gets a suffix, on
  the grounds that the API is the "main" workload.
- c. Other.

### 5. Which workloads can be turned off?

- a. **`site.enabled` (default `true`); the API always renders.** A site
  without an API is useless, but an API without a site serves headless use
  (the API and search only).
- b. `api.enabled` and `site.enabled` both. This allows a site-only install
  pointed at an external API, at the cost of a second set of conditionals.
- c. Neither: both always render.
- d. Other.

### 6. Is `DOCZ_API_URL` overridable?

- a. **Derived by default, overridable with `site.config.doczApiUrl`.**
  The override costs one `if` and keeps the external-API case open.
- b. Always derived. This is simplest, and a site that needs another API
  uses its own chart.
- c. Other.

### 7. Where do the shared settings live?

- a. **Plain top-level blocks (`auth.providers`, `otel`), read by both
  workloads.** Nothing about them needs Helm's `global` semantics, because
  there are no subcharts.
- b. A `global:` block, the usual Helm convention for shared values, but a
  convention meant for subcharts this chart does not have.
- c. Keep them per workload, and fail the render when the two disagree.
- d. Other.

### 8. What does the edge point at?

- a. **One Ingress/HTTPRoute, to the site; to the API when the site is
  off** (§5). The site already proxies everything the API serves publicly.
- b. One edge per workload, as today. This allows exposing the API on its
  own host, but reintroduces two front doors.
- c. One edge to the site, plus an optional path rule sending `/webhooks`
  straight to the API and skipping the proxy hop.
- d. Other.

### 9. Where does the Tailscale sidecar go?

- a. **On the front-door pod, with `hostname` defaulting to `docz`.** Funnel
  then publishes the site, and webhooks reach the API through the proxy. A
  homelab install behind Tailscale has one tailnet name.
- b. Stay on the API pod, as today. This keeps webhook delivery independent
  of the site, but the browser-facing site then needs a separate way in.
- c. Per-workload sidecars (`api.tailscale`, `site.tailscale`).
- d. Other.

### 10. One ServiceAccount or two?

- a. **One per workload**, each with its own `serviceAccount` block. The
  Tailscale Role binds only to the edge pod's account, which gives least
  privilege.
- b. One shared account. This is simpler, but the Tailscale Secret access
  extends to both workloads.
- c. Other.

### 11. Does `component` go into the Deployments' `matchLabels`?

- a. **Yes, everywhere** (§2). New installs only means no immutable-selector
  constraint, so the 0.2.2 compromise can end.
- b. Follow the old pattern: component on pods and Services only. This keeps
  the charts alike, but carries forward a workaround for a constraint this
  chart does not have.
- c. Other.

### 12. How does the chart publish?

- a. **A third `component: chart` row in `ghcr.yml`/`ecr.yml`**, with the
  image job skipped for a row without an image (§9). There is one table, one
  chart job, and one more call per workflow.
- b. A separate `chart.yml` reusable workflow. This is cleaner in isolation,
  but it duplicates the signing and attestation steps `ghcr.yml` already has.
- c. Keep the chart on the `api` row. This is the fewest changes, but it
  couples the product chart to one image.
- d. Other.

### 13. Where do the chart's `just` recipes live?

- a. **A new optional module `chart.just`** (`just chart lint`), which the
  root `ci` gate calls as `chart::lint`. The chart is neither the server's
  nor the frontend's.
- b. A `helm` group in `docz.just`, the root namespace. This saves a file,
  but `docz.just` is the library's.
- c. Keep them in `api.just` as `just api helm-*`.
- d. Other.

### 14. How are the unit tests laid out?

- a. **`tests/api/`, `tests/site/`, `tests/chart/`**, run with
  `-f 'tests/**/*_test.yaml'`. Ported suites stay recognisable, and the new
  ones are grouped.
- b. Flat, with prefixed names (`api-deployment_test.yaml`), which keeps the
  default glob.
- c. Other.

### 15. When are the old chart directories deleted?

- a. **At the v2.0.0 cut**, after one deprecated final version of each ships
  in the first beta with `charts/docz` (§11).
- b. In the same PR that adds `charts/docz`, with no final deprecated
  versions published.
- c. Keep them indefinitely as deprecated, never publishing again.
- d. Other.

### 16. Does the first cut add a site alert?

- a. **No.** Carry the five API alerts over unchanged and file a follow-up
  for a `DoczSiteDown`/proxy-error alert, which needs its own thresholds.
- b. Yes: add `DoczSiteDown` (`up{job="<fullname>-site"} == 0`) now, since
  the PrometheusRule is being rewritten anyway.
- c. Other.

### 17. Is `authRedirectBase` derived from the edge?

- a. **No, it stays explicit and required** (unless `none`). The chart
  cannot tell the scheme a TLS-terminating proxy presents, and a wrong
  callback URL fails at the identity provider, far from the chart.
- b. Default it to `https://<first ingress host or httpRoute hostname>` when
  unset, and keep the override.
- c. Other.

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [INV-0014](../investigation/0014-consolidate-the-docz-api-and-docz-site-helm-charts-into-one.md):
  the consolidation investigation and its three resolutions
- [ADR-0004](../adr/0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md):
  one repository, one release
- [DESIGN-0017](0017-move-docz-site-in-ui-chartsdocz-site-and-orval-on-the-one-spec.md)
  and [IMPL-0020](../impl/0020-docz-site-move-in-v200-beta4.md): the
  docz-site move-in and the per-component publish path
- [Helm: Chart.yaml `deprecated`](https://helm.sh/docs/topics/charts/#the-chartyaml-file)
- [helm-unittest](https://github.com/helm-unittest/helm-unittest)

<!--docz:references:end-->
