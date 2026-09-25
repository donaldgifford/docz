# docz

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 2.0.0-beta.5](https://img.shields.io/badge/AppVersion-2.0.0--beta.5-informational?style=flat-square)

Helm chart for docz — docz-api, docz-site, and their backends in one release

**Homepage:** <https://github.com/donaldgifford/docz>

## Source Code

* <https://github.com/donaldgifford/docz>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| api | object | `{}` | docz-api: the server. Always installed. |
| auth.providers | string | `"github"` | Comma-separated login providers: `github`, `okta`, `keycloak`, or `none` alone (login-free; the read API is open to anyone who can reach it). Rendered as AUTH_PROVIDERS on the API and DOCZ_AUTH_PROVIDERS on the site, so the two always agree. |
| extraLabels | object | `{}` | Labels added to every object's metadata and to every pod template, for tracking (e.g. `owner: lab`). Never added to a selector, a StatefulSet's volumeClaimTemplates, or CNPG's inheritedMetadata, so setting or changing them cannot break an install or an upgrade. A key the chart sets itself (`helm.sh/chart`, `app.kubernetes.io/*`) fails the render. |
| fullnameOverride | string | `""` | Override the full resource-name prefix (`<release>-docz` by default). |
| imagePullSecrets | list | `[]` | Image pull secrets, shared by both workloads. |
| metrics | object | `{"enabled":true}` | Prometheus metrics, served on each workload's HTTP port. |
| metrics.enabled | bool | `true` | Expose /metrics on both workloads. Also turns on the baked Meilisearch's Prometheus route (MEILI_EXPERIMENTAL_ENABLE_METRICS). |
| nameOverride | string | `""` | Override the chart name used in resource names. |
| otel.endpoint | string | `""` | OTLP/HTTP collector endpoint, host:port or URL (OTEL_EXPORTER_OTLP_ENDPOINT). Empty → tracing off in both workloads. Each workload names its own spans with `<workload>.otel.serviceName`. |
| otel.sampleRate | int | `1` | Trace sample rate 0.0–1.0 (OTEL_SAMPLE_RATE), for both workloads. |
| prometheusRule | object | `{"enabled":false,"labels":{}}` | PrometheusRule with the API's starter alerts (RED + ingest). |
| prometheusRule.enabled | bool | `false` | Create the PrometheusRule. |
| prometheusRule.labels | object | `{}` | Additional labels (e.g., to match Prometheus operator `ruleSelector`). |
| serviceMonitor.enabled | bool | `false` | Create Prometheus Operator ServiceMonitors: one per workload, and in baked mode one for Meilisearch with the master key as its bearer token. |
| serviceMonitor.interval | string | `"30s"` | Scrape interval |
| serviceMonitor.labels | object | `{}` | Additional labels for every ServiceMonitor |
| site | object | `{}` | docz-site: the frontend. Always installed. |
