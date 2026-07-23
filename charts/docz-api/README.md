# docz-api

Helm chart for docz-api

## What this deploys

`docz-api` ingests [docz](https://github.com/donaldgifford/docz)-format
documentation from GitHub repositories and serves a read + full-text-search
API over it, driven by GitHub App webhooks and site login. Everything is
served on a single HTTP port (`config.port`, default `8080`): the `/api/v1`
surface, the `/webhooks/github` receiver, `/auth/*` login, and the
operational `/healthz`, `/readyz`, `/metrics`, and `/openapi.yaml` endpoints.

The chart renders the service `Deployment` plus its three backing
dependencies, each of which can be baked (chart-managed), delegated to an
operator, or pointed at an external instance:

- **Postgres** — the persistent store (`store.postgres.mode`).
- **Valkey** (Redis-wire-compatible) — the ingest work queue and session
  store (`queue.valkey.mode`).
- **Meilisearch** — the full-text search index (`search.meili.mode`).

The image runs on a read-only rootfs as the distroless `nonroot` user
(UID/GID 65532).

## Installation

The chart is published as an OCI artifact to two registries — pick whichever
you prefer. Both are signed with cosign keyless and ship SLSA Level 3
provenance attestations.

**GHCR (public, anonymous pull):**

```bash
helm install docz-api \
  oci://ghcr.io/donaldgifford/charts/docz-api \
  --version 0.2.1 \
  --namespace docz-api \
  --create-namespace \
  -f values.yaml
```

**ECR (private; requires AWS auth):**

```bash
aws ecr get-login-password --region <region> | \
  helm registry login <account>.dkr.ecr.<region>.amazonaws.com \
    --username AWS --password-stdin

helm install docz-api \
  oci://<account>.dkr.ecr.<region>.amazonaws.com/docz-api \
  --version 0.2.1 \
  --namespace docz-api \
  --create-namespace \
  -f values.yaml
```

## Prerequisites

- Kubernetes 1.28+
- Helm 3.14+ or 4.x (OCI support)
- A registered [GitHub App](https://docs.github.com/en/apps/creating-github-apps) with:
  - **Permissions:** Contents (Read), Metadata (Read)
  - **Events:** `installation`, `installation_repositories`, `push`, `release`
  - A generated **private key** (PEM) and **webhook secret**
- An OAuth client (the GitHub App's own client id/secret, or a separate
  OAuth app) for site login

## Configuration

Minimal `values.yaml` for the default all-baked shape:

```yaml
config:
  appId: "YOUR_APP_ID"
  authRedirectBase: "https://docz-api.example.com"
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
    masterKey: "A_RANDOM_MEILI_MASTER_KEY"  # required in baked mode
```

### Secrets contract

When `secrets.create: true` (the default) the chart renders one `Secret`
carrying five keys: `app-id`, `webhook-secret`, `private-key`,
`session-secret`, and `oauth-client-secret`. The GitHub App private key is
mounted as a file by default (`secrets.privateKeyAsFile: true`); set it to
`false` to pass the key as an env var instead.

To manage the secret out of band, set `secrets.create: false` and
`secrets.existingSecret: <name>`. **That secret must carry all five keys**
(`app-id`, `webhook-secret`, `private-key`, `session-secret`,
`oauth-client-secret`).

The backing-service DSNs (`DATABASE_URL`, `REDIS_URL`, `MEILI_API_KEY`) live
in their own secrets, sourced per dependency mode (see below).

## Choosing a deployment shape

Each dependency is independent — mix and match to fit your cluster. The baked
shape is the default and brings the StatefulSets up out of the box.

| Dependency | `mode` | Renders |
|------------|--------|---------|
| **Postgres** (`store.postgres.mode`) | `baked` (default) | Single-pod Postgres `StatefulSet` + headless `Service` + `Secret` (auto-generated password, preserved across upgrades). |
| | `cnpg` | A [CloudNativePG](https://cloudnative-pg.io/) `Cluster` CR (and optional `Pooler`); `DATABASE_URL` reads the CNPG `<name>-app` secret's `uri` key. Requires the operator. |
| | `external` | Nothing — `DATABASE_URL` comes from `store.external.existingSecret`. |
| **Valkey** (`queue.valkey.mode`) | `baked` (default) | Single-pod Valkey `StatefulSet` + `Service` + `Secret`. |
| | `external` | Nothing — `REDIS_URL` comes from `queue.external.existingSecret`. |
| **Meilisearch** (`search.meili.mode`) | `baked` (default) | Single-pod Meilisearch `StatefulSet` + headless `Service` + `Secret`. Requires `search.meili.masterKey`. |
| | `external` | Nothing — `MEILI_HOST` = `search.meili.host`, `MEILI_API_KEY` from `search.meili.external.existingSecret`. |

The baked shapes are sized for homelab / small dev clusters; for production,
run Postgres via CNPG or a managed service and point Valkey/Meilisearch at
managed instances.

## Observability

- Set `metrics.enabled: true` (default) to serve Prometheus metrics on
  `/metrics`; enable `serviceMonitor.enabled: true` to have a Prometheus
  Operator scrape the `http` port at `/metrics`.
- Enable `prometheusRule.enabled: true` for the starter alert pack
  (`DoczAPIDown`, `DoczAPIHighErrorRate`, `DoczAPISlowRequests`,
  `DoczAPIIngestFailures`, `DoczAPISlowIngest`).
- Set `otel.endpoint` to an OTLP/HTTP collector to export traces; leave it
  empty to disable tracing.

## Verifying the chart

Every published version is signed with cosign (Sigstore keyless) via the
workflow's OIDC identity, plus a SLSA Level 3 provenance attestation. Both
can be verified offline against the public Sigstore transparency log.

### Cosign signature

```bash
cosign verify \
  --certificate-identity-regexp \
    '^https://github.com/donaldgifford/docz-api/.+' \
  --certificate-oidc-issuer \
    'https://token.actions.githubusercontent.com' \
  ghcr.io/donaldgifford/charts/docz-api:0.2.1
```

### SLSA provenance

```bash
cosign verify-attestation --type slsaprovenance \
  --certificate-identity-regexp \
    '^https://github.com/slsa-framework/slsa-github-generator/.+' \
  --certificate-oidc-issuer \
    'https://token.actions.githubusercontent.com' \
  ghcr.io/donaldgifford/charts/docz-api:0.2.1
```

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
| config | object | `{"appId":"","authProviders":"github","authRedirectBase":"","githubApiBase":"","githubOAuthClientID":"","ingestDebounce":"","logFormat":"json","logLevel":"info","port":8080,"sessionTTL":""}` | docz-api application configuration (non-secret env vars). Empty strings for the optional tuning knobs mean "emit nothing; let the binary apply its own default". |
| config.appId | string | `""` | GitHub App ID (GITHUB_APP_ID) |
| config.authProviders | string | `"github"` | Comma-separated login providers: github, okta, keycloak (AUTH_PROVIDERS). |
| config.authRedirectBase | string | `""` | Absolute base URL the OAuth/OIDC provider redirects back to; the chart appends /auth/callback (AUTH_REDIRECT_BASE). Required. |
| config.githubApiBase | string | `""` | GitHub API base URL; override for GitHub Enterprise (GITHUB_API_BASE). Empty → api.github.com. |
| config.githubOAuthClientID | string | `""` | GitHub OAuth client id for site login (GITHUB_OAUTH_CLIENT_ID). Required while `github` is in authProviders. |
| config.ingestDebounce | string | `""` | Ingest debounce window as a Go duration (INGEST_DEBOUNCE). Empty → 5s. |
| config.logFormat | string | `"json"` | Log format: text or json (LOG_FORMAT) |
| config.logLevel | string | `"info"` | Log level: debug, info, warn, error (LOG_LEVEL) |
| config.port | int | `8080` | Container HTTP listen port (drives HTTP_ADDR and the Service targetPort). Everything — API, /healthz, /readyz, /metrics — is served here. |
| config.sessionTTL | string | `""` | Session lifetime as a Go duration (SESSION_TTL). Empty → 720h. |
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
| image.repository | string | `"ghcr.io/donaldgifford/docz-api"` | Container image repository |
| image.tag | string | `""` | Overrides the image tag (default: chart appVersion) |
| imagePullSecrets | list | `[]` | Image pull secrets |
| ingress | object | `{"annotations":{},"className":"","enabled":false,"hosts":[],"tls":[]}` | Ingress (networking.k8s.io/v1). Off by default. |
| ingress.annotations | object | `{}` | Ingress annotations |
| ingress.className | string | `""` | IngressClass name |
| ingress.enabled | bool | `false` | Enable an Ingress |
| ingress.hosts | list | `[]` | Ingress hosts. Each entry: {host, paths: [{path, pathType}]}. |
| ingress.tls | list | `[]` | TLS blocks. Each entry: {secretName, hosts: []}. |
| livenessProbe.httpGet.path | string | `"/healthz"` |  |
| livenessProbe.httpGet.port | string | `"http"` |  |
| livenessProbe.initialDelaySeconds | int | `5` |  |
| livenessProbe.periodSeconds | int | `15` |  |
| metrics | object | `{"enabled":true}` | Prometheus metrics. The /metrics endpoint is served on the main HTTP port; scrape it with the ServiceMonitor below. |
| metrics.enabled | bool | `true` | Expose /metrics (METRICS_ENABLED) |
| nameOverride | string | `""` | Override the chart name |
| nodeSelector | object | `{}` | Node selector |
| otel | object | `{"endpoint":"","sampleRate":"","serviceName":""}` | OpenTelemetry tracing. Traces export over OTLP/HTTP; leave the endpoint empty to disable tracing (spans are created but not sent). |
| otel.endpoint | string | `""` | OTLP/HTTP collector endpoint, host:port or URL (OTEL_EXPORTER_OTLP_ENDPOINT). Empty → tracing off. |
| otel.sampleRate | string | `""` | Trace sample rate 0.0–1.0 (OTEL_SAMPLE_RATE). Empty → 1.0. |
| otel.serviceName | string | `""` | Service name reported on spans (OTEL_SERVICE_NAME). Empty → docz-api. |
| podAnnotations | object | `{}` | Pod annotations |
| podLabels | object | `{}` | Pod labels |
| podSecurityContext | object | `{"fsGroup":65532,"runAsGroup":65532,"runAsNonRoot":true,"runAsUser":65532,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod security context. Defaults match the distroless `nonroot` image (UID/GID 65532) and drop to a RuntimeDefault seccomp profile. |
| prometheusRule | object | `{"enabled":false,"labels":{}}` | Prometheus PrometheusRule with starter alerts (docz-api RED + ingest). Alert expressions are defined in the template. |
| prometheusRule.enabled | bool | `false` | Create PrometheusRule with starter alerts. |
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
| readinessProbe.httpGet.path | string | `"/readyz"` |  |
| readinessProbe.httpGet.port | string | `"http"` |  |
| readinessProbe.initialDelaySeconds | int | `5` |  |
| readinessProbe.periodSeconds | int | `10` |  |
| replicaCount | int | `1` | Number of replicas |
| resources | object | `{"limits":{"cpu":"500m","memory":"256Mi"},"requests":{"cpu":"100m","memory":"128Mi"}}` | Container resource requests and limits |
| revisionHistoryLimit | int | `3` | Number of old ReplicaSets retained for rollback. Defaults to 3 to keep the kubectl `get rs` view tidy; bump if you need more rollback headroom. Kubernetes default is 10. |
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
| secrets | object | `{"create":true,"existingSecret":"","oauthClientSecret":"","privateKey":"","privateKeyAsFile":true,"sessionSecret":"","webhookSecret":""}` | Application secrets. When `create` is true the chart renders a Secret carrying every key below; when false, `existingSecret` must name a Secret that already holds app-id, webhook-secret, private-key, session-secret, and oauth-client-secret. |
| secrets.create | bool | `true` | Create secret resource (false = use existing secret) |
| secrets.existingSecret | string | `""` | Name of existing secret (when create=false) |
| secrets.oauthClientSecret | string | `""` | GitHub OAuth client secret for site login (GITHUB_OAUTH_CLIENT_SECRET) |
| secrets.privateKey | string | `""` | GitHub App private key (PEM format, GITHUB_APP_PRIVATE_KEY) |
| secrets.privateKeyAsFile | bool | `true` | Mount the private key as a file (true) or pass it as an env var (false). Either way the binary accepts a path or a PEM body. |
| secrets.sessionSecret | string | `""` | Session signing secret (SESSION_SECRET) |
| secrets.webhookSecret | string | `""` | GitHub webhook HMAC secret (GITHUB_WEBHOOK_SECRET) |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Container security context. The image runs on a read-only rootfs; nothing is written to disk except the optional mounted private key. |
| service.port | int | `80` | Service port. Targets the container's single `http` port, which serves the API, probes, and /metrics. |
| service.type | string | `"ClusterIP"` | Service type |
| serviceAccount.annotations | object | `{}` | Annotations for the ServiceAccount |
| serviceAccount.create | bool | `true` | Create a ServiceAccount |
| serviceAccount.name | string | `""` | Override the ServiceAccount name |
| serviceMonitor.enabled | bool | `false` | Create a Prometheus Operator ServiceMonitor scraping /metrics |
| serviceMonitor.interval | string | `"30s"` | Scrape interval |
| serviceMonitor.labels | object | `{}` | Additional labels for ServiceMonitor |
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
| tailscale | object | `{"authKeySecret":"tailscale-auth","enabled":false,"hostname":"docz-api","image":"ghcr.io/tailscale/tailscale:latest","rbac":{"create":true},"userspace":true}` | Tailscale Funnel sidecar. Off by default; the chart does not require tailscale — homelab infra fronts the service with it. |
| tailscale.authKeySecret | string | `"tailscale-auth"` | Name of existing secret containing 'authkey' |
| tailscale.enabled | bool | `false` | Enable Tailscale sidecar container |
| tailscale.hostname | string | `"docz-api"` | Tailscale hostname (becomes <hostname>.<tailnet>.ts.net) |
| tailscale.image | string | `"ghcr.io/tailscale/tailscale:latest"` | Tailscale container image |
| tailscale.rbac | object | `{"create":true}` | Create RBAC for Tailscale state management |
| tailscale.userspace | bool | `true` | Use userspace networking (no CAP_NET_ADMIN needed) |
| tolerations | list | `[]` | Tolerations |

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| donaldgifford |  |  |
