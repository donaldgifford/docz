# docz

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 2.0.0-beta.5](https://img.shields.io/badge/AppVersion-2.0.0--beta.5-informational?style=flat-square)

Helm chart for docz — docz-api, docz-site, and their backends in one release

**Homepage:** <https://github.com/donaldgifford/docz>

## What this deploys

One release runs all of docz's server side:

- **docz-api** (`api.*`) ingests [docz](https://github.com/donaldgifford/docz)
  documentation from GitHub repositories, driven by GitHub App webhooks. It
  serves a read and full-text-search API. Everything is on one HTTP port:
  `/api/v1`, `/webhooks/github`, `/auth/*`, `/healthz`, `/readyz`,
  `/metrics`, and `/openapi.yaml`.
- **docz-site** (`site.*`) is the web UI. It is a small Bun process that
  serves the single-page app and reverse-proxies `/api`, `/auth`, `/webhooks`,
  and `/openapi.yaml` to the API, so the browser sees one origin and the
  session cookie is first-party.
- **The API's three backing services**, each baked (run by the chart),
  delegated to an operator, or pointed at an external instance: Postgres
  (`store.*`), Valkey (`queue.*`), and Meilisearch (`search.*`).

Every object is named `<release>-docz-<role>`, and the role is also its
`app.kubernetes.io/component` label, which is in every selector. For release
`docz` the roles give `docz-api`, `docz-site`, `docz-postgres`, `docz-valkey`,
and `docz-meilisearch`. Both images default their tag to the chart's
`appVersion`, so one chart version pins one API and one site.

The site finds the API by itself. With `site.config.doczApiUrl` empty (the
default), it uses this release's API Service:
`http://<release>-docz-api.<namespace>.svc.cluster.local:<api.service.port>`.
Set it only to point the site at an API outside the release.

## Installation

The chart is published as an OCI artifact to GHCR, signed with cosign keyless
and carrying a SLSA v1 build provenance attestation (GitHub artifact
attestations, Build L2).

```bash
helm install docz oci://ghcr.io/donaldgifford/charts/docz \
  --version 0.1.0 \
  --namespace docz \
  --create-namespace \
  -f values.yaml
```

Every install needs a GitHub App, a login provider (or `none`; see
[Login providers](#login-providers)), and a Meilisearch master key if
Meilisearch is baked. A minimal all-baked `values.yaml`:

```yaml
auth:
  providers: "github"

api:
  config:
    appId: "YOUR_APP_ID"
    # The SITE's public URL. See "The two edges".
    authRedirectBase: "https://docz.example.com"
    githubOAuthClientID: "YOUR_OAUTH_CLIENT_ID"
  secrets:
    webhookSecret: "YOUR_WEBHOOK_SECRET"
    sessionSecret: "A_RANDOM_32B_SECRET"
    oauthClientSecret: "YOUR_OAUTH_CLIENT_SECRET"
    privateKey: |
      -----BEGIN RSA PRIVATE KEY-----
      YOUR_PRIVATE_KEY
      -----END RSA PRIVATE KEY-----

search:
  meili:
    masterKey: "A_RANDOM_MEILI_MASTER_KEY"  # at least 16 bytes
```

### Prerequisites

- Kubernetes 1.28+ and Helm 3.14+ or 4.x (OCI support)
- A [GitHub App](https://docs.github.com/en/apps/creating-github-apps) with
  Contents (Read) and Metadata (Read), subscribed to `push` and `release`,
  with a generated private key and webhook secret
- For login, an OAuth client (the GitHub App's own or a separate OAuth app),
  or an Okta or Keycloak OIDC **confidential** client

## The two edges

The two workloads are exposed separately. `api.ingress`, `api.httpRoute`,
`site.ingress`, and `site.httpRoute` are all off by default. Each one renders
as `<release>-docz-api` or `<release>-docz-site` and sends traffic only to its
own workload's Service and port. An HTTPRoute with no `rules` gets one
default rule to its Service.

The usual shape:

- **The site** on an internal network: people use the UI, and the site
  proxies their API calls, so they never need the API's own edge.
- **The API** where GitHub can reach `/webhooks/github`, and where internal
  clients can reach `/api/v1` directly if they need to.

```yaml
site:
  httpRoute:
    enabled: true
    parentRefs: [{name: internal-gateway, namespace: gateway}]
    hostnames: [docz.example.com]
api:
  httpRoute:
    enabled: true
    parentRefs: [{name: webhook-gateway, namespace: gateway}]
    hostnames: [docz-api.example.com]
```

**`api.config.authRedirectBase` is the site's public URL**, not the API's.
The login provider redirects the browser to
`<authRedirectBase>/auth/callback`, and the site proxies that to the API.
Register the same URL as the provider's redirect URI.

To take GitHub webhooks through a Tailscale operator Ingress on the API, see
[`deploy/tailscale-operator.md`](https://github.com/donaldgifford/docz/blob/main/deploy/tailscale-operator.md).

## Login providers

`auth.providers` is one comma-separated list of `github`, `okta`, and
`keycloak`. It becomes the API's `AUTH_PROVIDERS` and the site's
`DOCZ_AUTH_PROVIDERS`, so the login page offers exactly what the API accepts.
Each enabled provider adds its own env and `Secret` key, and its values are
`required` at render time:

| Provider | Values | Secret key |
|----------|--------|------------|
| `github` | `api.config.githubOAuthClientID` | `oauth-client-secret` |
| `okta` | `api.config.oktaIssuer`, `api.config.oktaClientID` | `okta-client-secret` |
| `keycloak` | `api.config.keycloakIssuer`, `api.config.keycloakClientID` | `keycloak-client-secret` |

Okta and Keycloak share one OIDC code path. The client must be
**confidential** (authorization-code grant with a client secret). Copy the
issuer verbatim from the provider's `.well-known/openid-configuration`:
discovery runs at startup, so a wrong issuer fails the boot.

### `none`: first setup without login

`auth.providers: "none"` removes login, so a first install can prove
ingestion works before any login credentials exist. With `none`, the
redirect base, the session secret, and the provider credentials are not
needed, and the chart renders none of them. `none` must be the only entry.

**This leaves the read API open to anyone who can reach the site or the API
Service.** Every request is served as an anonymous identity, and
`/auth/login` and `/auth/callback` are not mounted. Do not expose a `none`
install on a public endpoint. To turn login on, set `auth.providers`, supply
the provider's values and secrets, and upgrade. Nothing else changes.

### Secrets

With `api.secrets.create: true` (the default), the chart renders one `Secret`
from `api.secrets.*`. To manage it with a secret manager (1Password Operator,
External Secrets, sealed-secrets, or `kubectl create secret`), set
`api.secrets.create: false` and `api.secrets.existingSecret: <name>`. The
chart references that Secret by key only:

| Key | Required |
|-----|----------|
| `app-id`, `webhook-secret`, `private-key` | always |
| `session-secret` | unless `auth.providers` is `none` |
| `oauth-client-secret` / `okta-client-secret` / `keycloak-client-secret` | when that provider is enabled |

The GitHub App private key is mounted as a file by default
(`api.secrets.privateKeyAsFile: true`). The backing-service DSNs live in
their own secrets, per backend mode.

## Backend modes

Each backing service is independent. Baked is the default and brings
everything up with no operator.

| Service | `mode` | Renders |
|---------|--------|---------|
| **Postgres** (`store.postgres.mode`) | `baked` (default) | A single-pod `StatefulSet`, a headless `Service`, and a `Secret` whose generated password is preserved across upgrades. |
| | `cnpg` | A [CloudNativePG](https://cloudnative-pg.io/) `Cluster` (and an optional `Pooler`). `DATABASE_URL` reads the CNPG `<name>-app` secret's `uri` key. Requires the operator. |
| | `external` | Nothing. `DATABASE_URL` comes from `store.external.existingSecret`. |
| **Valkey** (`queue.valkey.mode`) | `baked` (default) | A single-pod `StatefulSet`, a `Service`, and a `Secret`. |
| | `external` | Nothing. `REDIS_URL` comes from `queue.external.existingSecret`. |
| **Meilisearch** (`search.meili.mode`) | `baked` (default) | A single-pod `StatefulSet`, a headless `Service`, and a `Secret`. Needs `search.meili.masterKey` or `search.meili.existingSecret`. |
| | `external` | Nothing. `MEILI_HOST` is `search.meili.host`, and `MEILI_API_KEY` comes from `search.meili.external.existingSecret`. |

The baked shapes are sized for a homelab or a small cluster. For production,
run Postgres through CNPG or a managed service, and point Valkey and
Meilisearch at managed instances.

## `extraLabels`

`extraLabels` is a map of labels added to every object's metadata and to
every pod template, for tracking (an owner, a cost centre). It is empty by
default. It never reaches a selector, a `volumeClaimTemplate`, or CNPG's
`inheritedMetadata`, so adding or changing one never breaks an upgrade. A
key the chart sets itself (`app.kubernetes.io/*`, `helm.sh/chart`) fails
the render instead of replacing a label a selector depends on.

```yaml
extraLabels:
  owner: lab
```

## Observability

- **Probes.** Both workloads use `/healthz` for liveness. The API's `/readyz`
  checks Postgres, Valkey, and Meilisearch and names the one that is down.
  The site's `/readyz` checks only itself and never calls the API, so an API
  outage does not remove the UI from its Service.
- **The GitHub App is checked at startup, not by a probe.** Only a 401 or an
  unparseable private key fails the API's boot; anything else warns and
  continues, since GitHub is not a serving dependency.
- **Metrics.** `metrics.enabled` (default `true`) serves `/metrics` on both
  workloads. `serviceMonitor.enabled` renders one ServiceMonitor per
  workload, plus one for a baked Meilisearch, which scrapes with the
  **master key** as its bearer token. Enabling that scrape means trusting
  the Prometheus namespace with the master key.
- **Alerts.** `prometheusRule.enabled` renders the API's starter pack:
  `DoczAPIDown`, `DoczAPIHighErrorRate`, `DoczAPISlowRequests`,
  `DoczAPIIngestFailures`, and `DoczAPISlowIngest`.
- **Tracing.** Set `otel.endpoint` to an OTLP/HTTP collector
  (`http://collector:4318`) to trace both workloads, or leave it empty to
  trace neither. The chart gives each runtime the form it reads: `host:port`
  for the API, and the `/v1/traces` URL for the site. The site forwards
  `traceparent` on the proxy hop, so one sampled request is one trace across
  both. Spans are named by `api.otel.serviceName` and
  `site.otel.serviceName`.

`helm test <release>` checks both Services' `/healthz`.

## Coming from docz-api or docz-site

`charts/docz` replaces the `docz-api` and `docz-site` charts, whose final
versions are marked deprecated. **There is no in-place upgrade**: object
names and selectors differ, so install `docz` as a new release, move the
data you need, and then uninstall the old ones. Values move as follows.
Anything not listed moves under the workload's block unchanged, so
docz-api's `resources` becomes `api.resources`.

| `docz-api` key | `docz` key |
| --- | --- |
| `replicaCount`, `revisionHistoryLimit`, `image`, `serviceAccount`, `pod*`, `securityContext`, `service`, `resources`, `*Probe`, `extra*`, `nodeSelector`, `tolerations`, `affinity` | `api.<same>` |
| `imagePullSecrets`, `nameOverride`, `fullnameOverride` | top level, shared |
| `config.authProviders` | `auth.providers` |
| `config.*` (every other key) | `api.config.*` |
| `otel.endpoint`, `otel.sampleRate` | `otel.endpoint`, `otel.sampleRate` |
| `otel.serviceName` | `api.otel.serviceName` |
| `secrets.*`, `autoscaling.*` | `api.secrets.*`, `api.autoscaling.*` |
| `metrics`, `serviceMonitor`, `prometheusRule` | top level, shared |
| `store`, `queue`, `search` | top level, unchanged |
| `ingress`, `httpRoute` | `api.ingress`, `api.httpRoute` |
| `tailscale.*` | removed |

| `docz-site` key | `docz` key |
| --- | --- |
| per-workload keys, as above | `site.<same>` |
| `config.authProviders` | `auth.providers` |
| `config.doczApiUrl` | `site.config.doczApiUrl`, now optional |
| `config.*` (every other key) | `site.config.*` |
| `otel.endpoint`, `otel.sampleRate` | `otel.*` |
| `otel.serviceName` | `site.otel.serviceName` |
| `metrics`, `serviceMonitor` | top level, shared |
| `ingress`, `httpRoute` | `site.ingress`, `site.httpRoute` |

The backing services are renamed from `<release>-docz-api-<service>` to
`<release>-docz-<service>`. The Tailscale sidecar is gone; see
[`deploy/tailscale-operator.md`](https://github.com/donaldgifford/docz/blob/main/deploy/tailscale-operator.md).

## Verifying the chart

```bash
cosign verify \
  --certificate-identity-regexp '^https://github.com/donaldgifford/docz/.+' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/donaldgifford/charts/docz:0.1.0

gh attestation verify \
  oci://ghcr.io/donaldgifford/charts/docz:0.1.0 \
  --owner donaldgifford
```

## Source Code

* <https://github.com/donaldgifford/docz>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| api | object | `{"affinity":{},"autoscaling":{"enabled":false,"maxReplicas":3,"minReplicas":1,"targetCPUUtilizationPercentage":80,"targetMemoryUtilizationPercentage":0},"config":{"appId":"","authRedirectBase":"","githubApiBase":"","githubOAuthClientID":"","ingestDebounce":"","keycloakClientID":"","keycloakIssuer":"","keycloakScopes":"","logFormat":"json","logLevel":"info","oktaClientID":"","oktaIssuer":"","oktaScopes":"","port":8080,"sessionTTL":""},"extraEnv":[],"extraVolumeMounts":[],"extraVolumes":[],"httpRoute":{"annotations":{},"enabled":false,"hostnames":[],"parentRefs":[],"rules":[]},"image":{"pullPolicy":"IfNotPresent","repository":"ghcr.io/donaldgifford/docz-api","tag":""},"ingress":{"annotations":{},"className":"","enabled":false,"hosts":[],"tls":[]},"livenessProbe":{"httpGet":{"path":"/healthz","port":"http"},"initialDelaySeconds":5,"periodSeconds":15},"nodeSelector":{},"otel":{"serviceName":"docz-api"},"podAnnotations":{},"podLabels":{},"podSecurityContext":{"fsGroup":65532,"runAsGroup":65532,"runAsNonRoot":true,"runAsUser":65532,"seccompProfile":{"type":"RuntimeDefault"}},"readinessProbe":{"httpGet":{"path":"/readyz","port":"http"},"initialDelaySeconds":5,"periodSeconds":10},"replicaCount":1,"resources":{"limits":{"cpu":"500m","memory":"256Mi"},"requests":{"cpu":"100m","memory":"128Mi"}},"revisionHistoryLimit":3,"secrets":{"create":true,"existingSecret":"","keycloakClientSecret":"","oauthClientSecret":"","oktaClientSecret":"","privateKey":"","privateKeyAsFile":true,"sessionSecret":"","webhookSecret":""},"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true},"service":{"port":80,"type":"ClusterIP"},"serviceAccount":{"annotations":{},"create":true,"name":""},"tolerations":[]}` | docz-api: the server. Always installed. |
| api.affinity | object | `{}` | Affinity rules |
| api.autoscaling | object | `{"enabled":false,"maxReplicas":3,"minReplicas":1,"targetCPUUtilizationPercentage":80,"targetMemoryUtilizationPercentage":0}` | Horizontal Pod Autoscaler. Off by default. |
| api.autoscaling.enabled | bool | `false` | Enable a HorizontalPodAutoscaler |
| api.autoscaling.maxReplicas | int | `3` | Maximum replicas |
| api.autoscaling.minReplicas | int | `1` | Minimum replicas |
| api.autoscaling.targetCPUUtilizationPercentage | int | `80` | Target average CPU utilization (percent) |
| api.autoscaling.targetMemoryUtilizationPercentage | int | `0` | Target average memory utilization (percent). Unset → no memory metric. |
| api.config | object | `{"appId":"","authRedirectBase":"","githubApiBase":"","githubOAuthClientID":"","ingestDebounce":"","keycloakClientID":"","keycloakIssuer":"","keycloakScopes":"","logFormat":"json","logLevel":"info","oktaClientID":"","oktaIssuer":"","oktaScopes":"","port":8080,"sessionTTL":""}` | docz-api application configuration (non-secret env vars). Empty strings for the optional tuning knobs mean "emit nothing; let the binary apply its own default". |
| api.config.appId | string | `""` | GitHub App ID (GITHUB_APP_ID) |
| api.config.authRedirectBase | string | `""` | Absolute base URL the OAuth/OIDC provider redirects back to; the chart appends /auth/callback (AUTH_REDIRECT_BASE). Required unless auth.providers is "none". This is the **site's** public URL: the site proxies /auth/callback to the API. |
| api.config.githubApiBase | string | `""` | GitHub API base URL; override for GitHub Enterprise (GITHUB_API_BASE). Empty → api.github.com. |
| api.config.githubOAuthClientID | string | `""` | GitHub OAuth client id for site login (GITHUB_OAUTH_CLIENT_ID). Required while `github` is in auth.providers. |
| api.config.ingestDebounce | string | `""` | Ingest debounce window as a Go duration (INGEST_DEBOUNCE). Empty → 5s. |
| api.config.keycloakClientID | string | `""` | Keycloak client id (KEYCLOAK_CLIENT_ID). The client must be confidential (client authentication on). Required while `keycloak` is in auth.providers. |
| api.config.keycloakIssuer | string | `""` | Keycloak OIDC issuer, e.g. `https://kc.example.com/realms/docz-api` (KEYCLOAK_ISSUER). Required while `keycloak` is in auth.providers. |
| api.config.keycloakScopes | string | `""` | Comma-separated OAuth scopes Keycloak is asked for beyond `openid`, which is always sent (KEYCLOAK_SCOPES). Empty → `profile,email`. Only add a scope assigned to the client (default or optional client scope): an unassigned scope fails the authorize request with `invalid_scope`. Stock Keycloak has no `groups` client scope — add one with a group membership mapper before requesting it here. |
| api.config.logFormat | string | `"json"` | Log format: text or json (LOG_FORMAT) |
| api.config.logLevel | string | `"info"` | Log level: debug, info, warn, error (LOG_LEVEL) |
| api.config.oktaClientID | string | `""` | Okta client id (OKTA_CLIENT_ID). The Okta app must be an OIDC **Web Application** (confidential client, authorization-code flow) — an SPA or Native app has no client secret and cannot complete the exchange. Required while `okta` is in auth.providers. |
| api.config.oktaIssuer | string | `""` | Okta OIDC issuer, copied verbatim from the issuer's own `.well-known/openid-configuration` (OKTA_ISSUER). Discovery runs at startup, so a wrong value fails the boot. Required while `okta` is in auth.providers. e.g. `https://acme.okta.com/oauth2/default` |
| api.config.oktaScopes | string | `""` | Comma-separated OAuth scopes Okta is asked for beyond `openid`, which is always sent (OKTA_SCOPES). Empty → `profile,email`. Only add a scope the Okta app is actually assigned: an unassigned scope fails the whole authorize request with `invalid_scope`, so login breaks entirely rather than degrading. Add `groups` only if the authorization server publishes a groups claim — docz-api serves it on `/api/v1/auth/session` for the site to read, but makes no access decision with it. Prefer setting this in a values file: `--set` splits on commas, so it needs `--set-string 'api.config.oktaScopes=profile\,email'`. |
| api.config.port | int | `8080` | Container HTTP listen port (drives HTTP_ADDR and the Service targetPort). Everything — API, /healthz, /readyz, /metrics — is served here. |
| api.config.sessionTTL | string | `""` | Session lifetime as a Go duration (SESSION_TTL). Empty → 720h. |
| api.extraEnv | list | `[]` | Additional environment variables |
| api.extraVolumeMounts | list | `[]` | Additional volume mounts |
| api.extraVolumes | list | `[]` | Additional volumes |
| api.httpRoute | object | `{"annotations":{},"enabled":false,"hostnames":[],"parentRefs":[],"rules":[]}` | Gateway API HTTPRoute. Off by default. |
| api.httpRoute.annotations | object | `{}` | HTTPRoute annotations |
| api.httpRoute.enabled | bool | `false` | Enable an HTTPRoute |
| api.httpRoute.hostnames | list | `[]` | Hostnames matched by this route. |
| api.httpRoute.parentRefs | list | `[]` | parentRefs (Gateways) this route attaches to. |
| api.httpRoute.rules | list | `[]` | Route rules. Empty → the template renders a default rule to the service. |
| api.image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| api.image.repository | string | `"ghcr.io/donaldgifford/docz-api"` | Container image repository |
| api.image.tag | string | `""` | Overrides the image tag (default: chart appVersion) |
| api.ingress | object | `{"annotations":{},"className":"","enabled":false,"hosts":[],"tls":[]}` | Ingress (networking.k8s.io/v1). Off by default. |
| api.ingress.annotations | object | `{}` | Ingress annotations |
| api.ingress.className | string | `""` | IngressClass name |
| api.ingress.enabled | bool | `false` | Enable an Ingress |
| api.ingress.hosts | list | `[]` | Ingress hosts. Each entry: {host, paths: [{path, pathType}]}. |
| api.ingress.tls | list | `[]` | TLS blocks. Each entry: {secretName, hosts: []}. |
| api.nodeSelector | object | `{}` | Node selector |
| api.otel | object | `{"serviceName":"docz-api"}` | Tracing for the API. The endpoint and sample rate are the shared top-level `otel` block. |
| api.otel.serviceName | string | `"docz-api"` | Service name reported on spans (OTEL_SERVICE_NAME). |
| api.podAnnotations | object | `{}` | Pod annotations |
| api.podLabels | object | `{}` | Pod labels |
| api.podSecurityContext | object | `{"fsGroup":65532,"runAsGroup":65532,"runAsNonRoot":true,"runAsUser":65532,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod security context. Defaults match the distroless `nonroot` image (UID/GID 65532) and drop to a RuntimeDefault seccomp profile. |
| api.replicaCount | int | `1` | Number of replicas |
| api.resources | object | `{"limits":{"cpu":"500m","memory":"256Mi"},"requests":{"cpu":"100m","memory":"128Mi"}}` | Container resource requests and limits |
| api.revisionHistoryLimit | int | `3` | Number of old ReplicaSets retained for rollback. Defaults to 3 to keep the kubectl `get rs` view tidy; bump if you need more rollback headroom. Kubernetes default is 10. |
| api.secrets | object | `{"create":true,"existingSecret":"","keycloakClientSecret":"","oauthClientSecret":"","oktaClientSecret":"","privateKey":"","privateKeyAsFile":true,"sessionSecret":"","webhookSecret":""}` | Application secrets. When `create` is true the chart renders a Secret from the plaintext values below. When `create` is false, `existingSecret` names a Secret you supply by any means (1Password Operator, External Secrets, sealed-secrets, `kubectl create secret`, ...) — the chart only references it by key, so the provider is entirely your choice.  The referenced Secret must carry these keys:    app-id                  always   webhook-secret          always   session-secret          unless auth.providers is "none"   private-key             when secrets.privateKeyAsFile is true, or the                           GitHub App key is not supplied out-of-band   oauth-client-secret     when `github`   is in auth.providers   okta-client-secret      when `okta`     is in auth.providers   keycloak-client-secret  when `keycloak` is in auth.providers  Only the enabled providers' keys are referenced, so a github-only install needs no okta-client-secret, and a "none" install needs no login keys at all. |
| api.secrets.create | bool | `true` | Create secret resource (false = use existing secret) |
| api.secrets.existingSecret | string | `""` | Name of an existing Secret holding the keys listed above (when create=false). Populate it with whatever secret manager you run. |
| api.secrets.keycloakClientSecret | string | `""` | Keycloak client secret (KEYCLOAK_CLIENT_SECRET). Required while `keycloak` is in auth.providers and create=true. Secret key: keycloak-client-secret. |
| api.secrets.oauthClientSecret | string | `""` | GitHub OAuth client secret for site login (GITHUB_OAUTH_CLIENT_SECRET). Required while `github` is in auth.providers and create=true. Secret key: oauth-client-secret. |
| api.secrets.oktaClientSecret | string | `""` | Okta client secret (OKTA_CLIENT_SECRET). Required while `okta` is in auth.providers and create=true. Secret key: okta-client-secret. |
| api.secrets.privateKey | string | `""` | GitHub App private key (PEM format, GITHUB_APP_PRIVATE_KEY) |
| api.secrets.privateKeyAsFile | bool | `true` | Mount the private key as a file (true) or pass it as an env var (false). Either way the binary accepts a path or a PEM body. |
| api.secrets.sessionSecret | string | `""` | Session signing secret (SESSION_SECRET). Required unless auth.providers is "none". |
| api.secrets.webhookSecret | string | `""` | GitHub webhook HMAC secret (GITHUB_WEBHOOK_SECRET) |
| api.securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Container security context. The image runs on a read-only rootfs; nothing is written to disk except the optional mounted private key. |
| api.service.port | int | `80` | Service port. Targets the container's single `http` port, which serves the API, probes, and /metrics. |
| api.service.type | string | `"ClusterIP"` | Service type |
| api.serviceAccount.annotations | object | `{}` | Annotations for the ServiceAccount |
| api.serviceAccount.create | bool | `true` | Create a ServiceAccount |
| api.serviceAccount.name | string | `""` | Override the ServiceAccount name |
| api.tolerations | list | `[]` | Tolerations |
| auth.providers | string | `"github"` | Comma-separated login providers: `github`, `okta`, `keycloak`, or `none` alone (login-free; the read API is open to anyone who can reach it). Rendered as AUTH_PROVIDERS on the API and DOCZ_AUTH_PROVIDERS on the site, so the two always agree. |
| extraLabels | object | `{}` | Labels added to every object's metadata and to every pod template, for tracking (e.g. `owner: lab`). Never added to a selector, a StatefulSet's volumeClaimTemplates, or CNPG's inheritedMetadata, so setting or changing them cannot break an install or an upgrade. A key the chart sets itself (`helm.sh/chart`, `app.kubernetes.io/*`) fails the render. |
| fullnameOverride | string | `""` | Override the full resource-name prefix (`<release>-docz` by default). |
| imagePullSecrets | list | `[]` | Image pull secrets, shared by both workloads. |
| metrics | object | `{"enabled":true}` | Prometheus metrics, served on each workload's HTTP port. |
| metrics.enabled | bool | `true` | Expose /metrics on both workloads. Also turns on the baked Meilisearch's Prometheus route (MEILI_EXPERIMENTAL_ENABLE_METRICS). |
| nameOverride | string | `""` | Override the chart name used in resource names. |
| otel.endpoint | string | `""` | OTLP/HTTP collector, as `http://host:4318` or bare `host:4318`. Empty turns tracing off in both workloads. The two runtimes read OTEL_EXPORTER_OTLP_ENDPOINT differently: the API wants `host:port`, while the site wants the full traces URL. The chart derives each form from this one value. A URL that already has a path goes to the site verbatim. The API always exports over plain HTTP. Each workload names its spans with `<workload>.otel.serviceName`. |
| otel.sampleRate | int | `1` | Trace head-sample rate, 0.0–1.0, for both workloads (the API reads OTEL_SAMPLE_RATE and the site OTEL_TRACES_SAMPLER_ARG). |
| prometheusRule | object | `{"enabled":false,"labels":{}}` | PrometheusRule with the API's starter alerts (RED + ingest). |
| prometheusRule.enabled | bool | `false` | Create the PrometheusRule. |
| prometheusRule.labels | object | `{}` | Additional labels (e.g., to match Prometheus operator `ruleSelector`). |
| queue | object | `{"backend":"valkey","external":{"existingSecret":"","secretKey":"REDIS_URL"},"valkey":{"baked":{"authPasswordLength":32,"image":"valkey/valkey:9.1","storageClassName":"","storageSize":"1Gi"},"existingSecret":"","existingSecretKey":"VALKEY_PASSWORD","mode":"baked"}}` | Work queue + session store (Valkey, Redis-wire-compatible). |
| queue.backend | string | `"valkey"` | Backend implementation. Only "valkey" is supported. |
| queue.external | object | `{"existingSecret":"","secretKey":"REDIS_URL"}` | External Valkey/Redis. Used only when queue.valkey.mode=external. |
| queue.external.existingSecret | string | `""` | Operator-supplied secret holding the REDIS_URL DSN. Required when queue.valkey.mode=external. |
| queue.external.secretKey | string | `"REDIS_URL"` | Key inside existingSecret holding the DSN. Default: REDIS_URL. |
| queue.valkey | object | `{"baked":{"authPasswordLength":32,"image":"valkey/valkey:9.1","storageClassName":"","storageSize":"1Gi"},"existingSecret":"","existingSecretKey":"VALKEY_PASSWORD","mode":"baked"}` | Valkey-specific configuration. Ignored when backend != valkey. |
| queue.valkey.baked | object | `{"authPasswordLength":32,"image":"valkey/valkey:9.1","storageClassName":"","storageSize":"1Gi"}` | Baked Valkey-only configuration. |
| queue.valkey.baked.authPasswordLength | int | `32` | Generated AUTH password length (random alphanumeric). Only used when existingSecret is unset. |
| queue.valkey.baked.image | string | `"valkey/valkey:9.1"` | Pinned image. Bump intentionally. |
| queue.valkey.baked.storageClassName | string | `""` | StorageClass name. Empty → cluster default. |
| queue.valkey.baked.storageSize | string | `"1Gi"` | Persistent volume size. |
| queue.valkey.existingSecret | string | `""` | Existing Secret holding the baked Valkey password. RECOMMENDED under GitOps: the self-generated password above relies on Helm `lookup`, which Argo CD / `helm template` cannot run, so it changes on every render and the running server drifts from its clients (asynq: WRONGPASS). Point this at a stable Secret (e.g. 1Password) and the chart renders no Valkey Secret of its own — the baked Valkey uses the value as `requirepass` and docz-api builds REDIS_URL from it (injected via the container's VALKEY_PASSWORD env, so no plaintext DSN is stored). Ignored when mode=external (use queue.external instead). |
| queue.valkey.existingSecretKey | string | `"VALKEY_PASSWORD"` | Key inside existingSecret holding the raw password. It is interpolated into the redis:// DSN, so keep it URL-safe / alphanumeric (e.g. `openssl rand -hex 32`). |
| queue.valkey.mode | string | `"baked"` | Source of the Valkey deployment. One of: "baked"    — chart renders a single-pod Valkey Deployment; "external" — operator provides REDIS_URL via queue.external. |
| search | object | `{"meili":{"existingSecret":"","existingSecretKey":"MEILI_API_KEY","external":{"existingSecret":"","secretKey":"MEILI_API_KEY"},"host":"","image":"getmeili/meilisearch:v1.12","masterKey":"","mode":"baked","storage":"5Gi","storageClassName":""}}` | Full-text search (Meilisearch). The chart runs a baked single-pod Meilisearch by default; point at an external instance with mode=external. |
| search.meili.existingSecret | string | `""` | Existing Secret holding the baked Meilisearch master key. When set, the chart renders no Secret of its own and both baked Meilisearch and docz-api read the key from here — no plaintext masterKey in values. Ignored when mode=external (use search.meili.external instead). |
| search.meili.existingSecretKey | string | `"MEILI_API_KEY"` | Key inside existingSecret holding the master key. |
| search.meili.external | object | `{"existingSecret":"","secretKey":"MEILI_API_KEY"}` | External Meilisearch secret. Used only when mode=external. |
| search.meili.external.existingSecret | string | `""` | Secret holding the API key. Required when mode=external. |
| search.meili.external.secretKey | string | `"MEILI_API_KEY"` | Key inside existingSecret holding the API key. |
| search.meili.host | string | `""` | External Meilisearch base URL, http(s)://host:port. Required when mode=external. |
| search.meili.image | string | `"getmeili/meilisearch:v1.12"` | Pinned baked image. Bump intentionally. |
| search.meili.masterKey | string | `""` | Meilisearch master key (baked mode). Required unless existingSecret is set; shared by the baked Meilisearch (MEILI_MASTER_KEY) and docz-api (MEILI_API_KEY). Prefer existingSecret over an inline value to avoid a plaintext secret. |
| search.meili.mode | string | `"baked"` | Source of Meilisearch. One of "baked" or "external". |
| search.meili.storage | string | `"5Gi"` | Persistent volume size (baked mode). |
| search.meili.storageClassName | string | `""` | StorageClass name (baked mode). Empty → cluster default. |
| serviceMonitor.enabled | bool | `false` | Create Prometheus Operator ServiceMonitors: one per workload, and in baked mode one for Meilisearch with the master key as its bearer token. |
| serviceMonitor.interval | string | `"30s"` | Scrape interval |
| serviceMonitor.labels | object | `{}` | Additional labels for every ServiceMonitor |
| site | object | `{"affinity":{},"autoscaling":{"enabled":false,"maxReplicas":3,"minReplicas":1,"targetCPUUtilizationPercentage":80,"targetMemoryUtilizationPercentage":0},"config":{"doczApiUrl":"","logFormat":"json","logLevel":"info","mermaidLayout":"elk","navLinks":[],"port":8080},"extraEnv":[],"extraVolumeMounts":[],"extraVolumes":[],"httpRoute":{"annotations":{},"enabled":false,"hostnames":[],"parentRefs":[],"rules":[]},"image":{"pullPolicy":"IfNotPresent","repository":"ghcr.io/donaldgifford/docz-site","tag":""},"ingress":{"annotations":{},"className":"","enabled":false,"hosts":[],"tls":[]},"livenessProbe":{"httpGet":{"path":"/healthz","port":"http"},"initialDelaySeconds":5,"periodSeconds":15},"nodeSelector":{},"otel":{"serviceName":"docz-site"},"podAnnotations":{},"podLabels":{},"podSecurityContext":{"fsGroup":1000,"runAsGroup":1000,"runAsNonRoot":true,"runAsUser":1000,"seccompProfile":{"type":"RuntimeDefault"}},"readinessProbe":{"httpGet":{"path":"/readyz","port":"http"},"initialDelaySeconds":5,"periodSeconds":10},"replicaCount":1,"resources":{"limits":{"cpu":"250m","memory":"128Mi"},"requests":{"cpu":"25m","memory":"64Mi"}},"revisionHistoryLimit":3,"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true},"service":{"port":80,"type":"ClusterIP"},"serviceAccount":{"annotations":{},"create":true,"name":""},"tolerations":[]}` | docz-site: the frontend. Always installed. |
| site.affinity | object | `{}` | Affinity rules |
| site.autoscaling | object | `{"enabled":false,"maxReplicas":3,"minReplicas":1,"targetCPUUtilizationPercentage":80,"targetMemoryUtilizationPercentage":0}` | Horizontal Pod Autoscaler. Off by default. |
| site.autoscaling.enabled | bool | `false` | Enable a HorizontalPodAutoscaler |
| site.autoscaling.maxReplicas | int | `3` | Maximum replicas |
| site.autoscaling.minReplicas | int | `1` | Minimum replicas |
| site.autoscaling.targetCPUUtilizationPercentage | int | `80` | Target average CPU utilization (percent) |
| site.autoscaling.targetMemoryUtilizationPercentage | int | `0` | Target average memory utilization (percent). Unset → no memory metric. |
| site.config | object | `{"doczApiUrl":"","logFormat":"json","logLevel":"info","mermaidLayout":"elk","navLinks":[],"port":8080}` | docz-site runtime configuration. The site is a static SPA served by a small Bun process that also reverse-proxies the API surface, so the browser and API share one origin (no CORS, first-party session cookie). |
| site.config.doczApiUrl | string | `""` | Absolute base URL of the docz-api the site proxies to (DOCZ_API_URL). Empty derives this release's own API Service, http://<fullname>-api.<namespace>.svc.cluster.local:<api.service.port>; set it only to point the site at an API outside this release. |
| site.config.logFormat | string | `"json"` | Log output format (DOCZ_LOG_FORMAT): `json` (one object per line, for a log aggregator) or `text` (readable, for local runs). |
| site.config.logLevel | string | `"info"` | Log verbosity (DOCZ_LOG_LEVEL): `debug`, `info`, `warn`, or `error`. `debug` adds a line per request plus the proxied `/auth/*` flow — the level to reach for when troubleshooting an Okta or Keycloak login. Credential-bearing values (`code`, `state`, cookies) are redacted at every level, enforced by a test. The values schema constrains this to the four names so a typo fails at install; at runtime an unrecognized level falls back to `info`, never to something noisier. |
| site.config.mermaidLayout | string | `"elk"` | Diagram layout engine (DOCZ_MERMAID_LAYOUT): `elk` or `dagre`. mermaid 12 made ELK the default and so is it here; `dagre` restores the pre-12 layout without rebuilding the image. Injected into the SPA at runtime and whitelist-validated at both ends, so anything unrecognized falls back to `elk` — the values schema constrains it to the two names so a typo fails at install rather than silently rendering the default. |
| site.config.navLinks | list | `[]` | Topbar nav pins (DOCZ_NAV_LINKS): a list of `{label, href}` links rendered between Repos and the session menu. Injected into the SPA at runtime as JSON, whitelist-validated by the server (short label charset, same-origin app-path hrefs, cap 6); invalid entries degrade to fewer/no pins, never a broken page. Empty → no pins and the env var is omitted. |
| site.config.port | int | `8080` | Container HTTP listen port (drives the PORT env var and the Service targetPort). The SPA, /healthz, /readyz, and the API proxy are all served here. |
| site.extraEnv | list | `[]` | Additional environment variables |
| site.extraVolumeMounts | list | `[]` | Additional volume mounts |
| site.extraVolumes | list | `[]` | Additional volumes |
| site.httpRoute | object | `{"annotations":{},"enabled":false,"hostnames":[],"parentRefs":[],"rules":[]}` | Gateway API HTTPRoute. Off by default. |
| site.httpRoute.annotations | object | `{}` | HTTPRoute annotations |
| site.httpRoute.enabled | bool | `false` | Enable an HTTPRoute |
| site.httpRoute.hostnames | list | `[]` | Hostnames matched by this route. |
| site.httpRoute.parentRefs | list | `[]` | parentRefs (Gateways) this route attaches to. |
| site.httpRoute.rules | list | `[]` | Route rules. Empty → the template renders a default rule to the service. |
| site.image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| site.image.repository | string | `"ghcr.io/donaldgifford/docz-site"` | Container image repository |
| site.image.tag | string | `""` | Overrides the image tag (default: chart appVersion) |
| site.ingress | object | `{"annotations":{},"className":"","enabled":false,"hosts":[],"tls":[]}` | Ingress (networking.k8s.io/v1). Off by default. |
| site.ingress.annotations | object | `{}` | Ingress annotations |
| site.ingress.className | string | `""` | IngressClass name |
| site.ingress.enabled | bool | `false` | Enable an Ingress |
| site.ingress.hosts | list | `[]` | Ingress hosts. Each entry: {host, paths: [{path, pathType}]}. |
| site.ingress.tls | list | `[]` | TLS blocks. Each entry: {secretName, hosts: []}. |
| site.livenessProbe | object | `{"httpGet":{"path":"/healthz","port":"http"},"initialDelaySeconds":5,"periodSeconds":15}` | Liveness probe. Stays on /healthz, which is unconditional: a failing liveness probe RESTARTS the container, and restarting cannot fix a broken image or mount — that is just CrashLoopBackOff. |
| site.nodeSelector | object | `{}` | Node selector |
| site.otel | object | `{"serviceName":"docz-site"}` | Tracing for the site. The endpoint and sample rate are the shared top-level `otel` block; one sampled request traces across both workloads, since the site injects `traceparent` on the proxy hop. |
| site.otel.serviceName | string | `"docz-site"` | Service name reported on spans (OTEL_SERVICE_NAME). |
| site.podAnnotations | object | `{}` | Pod annotations |
| site.podLabels | object | `{}` | Pod labels |
| site.podSecurityContext | object | `{"fsGroup":1000,"runAsGroup":1000,"runAsNonRoot":true,"runAsUser":1000,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod security context. Defaults match the `oven/bun` runtime image, whose `bun` user is UID/GID 1000, and drop to a RuntimeDefault seccomp profile. |
| site.readinessProbe | object | `{"httpGet":{"path":"/readyz","port":"http"},"initialDelaySeconds":5,"periodSeconds":10}` | Readiness probe. Points at /readyz, which verifies the dist/ is servable and the runtime config validated — a failing readiness probe HOLDS TRAFFIC, so a bad deploy stalls the rollout and the previous ReplicaSet keeps serving. /readyz makes no call to docz-api by design: gating readiness on the API would evict every pod from the Service when the API blipped. |
| site.replicaCount | int | `1` | Number of replicas |
| site.resources | object | `{"limits":{"cpu":"250m","memory":"128Mi"},"requests":{"cpu":"25m","memory":"64Mi"}}` | Container resource requests and limits |
| site.revisionHistoryLimit | int | `3` | Number of old ReplicaSets retained for rollback. Defaults to 3 to keep the kubectl `get rs` view tidy; bump if you need more rollback headroom. Kubernetes default is 10. |
| site.securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Container security context. The static server reads `dist/` and proxies; it writes nothing to disk, so the rootfs is mounted read-only. |
| site.service.port | int | `80` | Service port. Targets the container's single `http` port, which serves the SPA, /healthz, and the same-origin API proxy. |
| site.service.type | string | `"ClusterIP"` | Service type |
| site.serviceAccount.annotations | object | `{}` | Annotations for the ServiceAccount |
| site.serviceAccount.create | bool | `true` | Create a ServiceAccount |
| site.serviceAccount.name | string | `""` | Override the ServiceAccount name |
| site.tolerations | list | `[]` | Tolerations |
| store | object | `{"backend":"postgres","external":{"existingSecret":"","secretKey":"DATABASE_URL"},"postgres":{"baked":{"image":"postgres:18.4","resources":{"limits":{"cpu":"1000m","memory":"1Gi"},"requests":{"cpu":"100m","memory":"256Mi"}},"storageClassName":"","storageSize":"10Gi"},"cnpg":{"imageName":"ghcr.io/cloudnative-pg/postgresql:18.4","instances":1,"pooler":{"enabled":false,"instances":1,"monitoring":{"enablePodMonitor":false},"pgbouncer":{"defaultPoolSize":25,"maxClientConnections":100,"parameters":{},"poolMode":"transaction"},"service":{"annotations":{},"enabled":false,"labels":{"bgp.cilium.io/advertise-service":"default","bgp.cilium.io/ip-pool":"default"},"type":"LoadBalancer"},"type":"rw"},"storage":{"size":"10Gi","storageClass":""}},"maxConns":16,"mode":"baked"}}` | Persistent state store (Postgres). See DESIGN-0012 §Backend modes. |
| store.backend | string | `"postgres"` | Backend implementation. Only "postgres" is supported. |
| store.external | object | `{"existingSecret":"","secretKey":"DATABASE_URL"}` | External Postgres. Used only when store.postgres.mode=external. |
| store.external.existingSecret | string | `""` | Operator-supplied secret holding the DATABASE_URL DSN. Required when store.postgres.mode=external. |
| store.external.secretKey | string | `"DATABASE_URL"` | Key inside existingSecret holding the DSN. Default: DATABASE_URL. |
| store.postgres | object | `{"baked":{"image":"postgres:18.4","resources":{"limits":{"cpu":"1000m","memory":"1Gi"},"requests":{"cpu":"100m","memory":"256Mi"}},"storageClassName":"","storageSize":"10Gi"},"cnpg":{"imageName":"ghcr.io/cloudnative-pg/postgresql:18.4","instances":1,"pooler":{"enabled":false,"instances":1,"monitoring":{"enablePodMonitor":false},"pgbouncer":{"defaultPoolSize":25,"maxClientConnections":100,"parameters":{},"poolMode":"transaction"},"service":{"annotations":{},"enabled":false,"labels":{"bgp.cilium.io/advertise-service":"default","bgp.cilium.io/ip-pool":"default"},"type":"LoadBalancer"},"type":"rw"},"storage":{"size":"10Gi","storageClass":""}},"maxConns":16,"mode":"baked"}` | Postgres-specific configuration. Ignored when backend != postgres. |
| store.postgres.baked | object | `{"image":"postgres:18.4","resources":{"limits":{"cpu":"1000m","memory":"1Gi"},"requests":{"cpu":"100m","memory":"256Mi"}},"storageClassName":"","storageSize":"10Gi"}` | Baked Postgres-only configuration. |
| store.postgres.baked.image | string | `"postgres:18.4"` | Pinned image. Bump intentionally. |
| store.postgres.baked.resources | object | `{"limits":{"cpu":"1000m","memory":"1Gi"},"requests":{"cpu":"100m","memory":"256Mi"}}` | Resource requests/limits for the Postgres container. |
| store.postgres.baked.storageClassName | string | `""` | StorageClass name. Empty → cluster default. |
| store.postgres.baked.storageSize | string | `"10Gi"` | Persistent volume size. |
| store.postgres.cnpg | object | `{"imageName":"ghcr.io/cloudnative-pg/postgresql:18.4","instances":1,"pooler":{"enabled":false,"instances":1,"monitoring":{"enablePodMonitor":false},"pgbouncer":{"defaultPoolSize":25,"maxClientConnections":100,"parameters":{},"poolMode":"transaction"},"service":{"annotations":{},"enabled":false,"labels":{"bgp.cilium.io/advertise-service":"default","bgp.cilium.io/ip-pool":"default"},"type":"LoadBalancer"},"type":"rw"},"storage":{"size":"10Gi","storageClass":""}}` | CloudNativePG-only configuration. |
| store.postgres.cnpg.imageName | string | `"ghcr.io/cloudnative-pg/postgresql:18.4"` | CNPG-managed Postgres image. |
| store.postgres.cnpg.instances | int | `1` | Number of CNPG instances. |
| store.postgres.cnpg.pooler | object | `{"enabled":false,"instances":1,"monitoring":{"enablePodMonitor":false},"pgbouncer":{"defaultPoolSize":25,"maxClientConnections":100,"parameters":{},"poolMode":"transaction"},"service":{"annotations":{},"enabled":false,"labels":{"bgp.cilium.io/advertise-service":"default","bgp.cilium.io/ip-pool":"default"},"type":"LoadBalancer"},"type":"rw"}` | Connection pooler (PgBouncer). Disabled by default. |
| store.postgres.cnpg.pooler.instances | int | `1` | Pooler replica count. |
| store.postgres.cnpg.pooler.monitoring.enablePodMonitor | bool | `false` | Render a `PodMonitor` for the pooler. Requires the Prometheus Operator CRD. |
| store.postgres.cnpg.pooler.pgbouncer.defaultPoolSize | int | `25` | PgBouncer `default_pool_size`. |
| store.postgres.cnpg.pooler.pgbouncer.maxClientConnections | int | `100` | PgBouncer `max_client_conn`. |
| store.postgres.cnpg.pooler.pgbouncer.parameters | object | `{}` | Extra `pgbouncer.ini` parameters (key-value pairs). |
| store.postgres.cnpg.pooler.pgbouncer.poolMode | string | `"transaction"` | PgBouncer pool mode: `session`, `transaction`, or `statement`. |
| store.postgres.cnpg.pooler.service.annotations | object | `{}` | Extra Service annotations. |
| store.postgres.cnpg.pooler.service.enabled | bool | `false` | Enable an external LoadBalancer Service in front of the pooler. |
| store.postgres.cnpg.pooler.service.labels | object | `{"bgp.cilium.io/advertise-service":"default","bgp.cilium.io/ip-pool":"default"}` | Service labels. Defaults wire Cilium BGP IP advertisement. |
| store.postgres.cnpg.pooler.service.type | string | `"LoadBalancer"` | Service `type` (usually `LoadBalancer`). |
| store.postgres.cnpg.pooler.type | string | `"rw"` | Pooler type: `rw` (primary) or `ro` (read-only replicas). |
| store.postgres.cnpg.storage | object | `{"size":"10Gi","storageClass":""}` | Storage block. |
| store.postgres.maxConns | int | `16` | Connection cap for the pgx pool. |
| store.postgres.mode | string | `"baked"` | Source of the Postgres deployment. One of: "baked"    — chart renders a single-pod Postgres Deployment; "cnpg"     — chart renders a CloudNativePG `Cluster` CR; "external" — operator provides DATABASE_URL via store.external. |

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| donaldgifford |  |  |
