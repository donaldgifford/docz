# contrib/

Contributed assets for operating docz-api in production.

These files are reference starting points, not normative configuration. Adjust
thresholds, panel layouts, and selectors to match your environment.

## Contents

| Path                              | Purpose                                                                                          |
| --------------------------------- | ------------------------------------------------------------------------------------------------ |
| `prometheus/alerts.yaml`          | Alerting rules: availability, HTTP 5xx error rate, request latency, and ingest failures/latency. |
| `grafana/docz-api-dashboard.json` | Import-style Grafana dashboard (RED for the HTTP surface + the async ingest pipeline).           |

## Exposed metrics

docz-api serves Prometheus metrics on the **same port as everything else**,
`:8080/metrics` (there is no separate admin listener). `/metrics` is public — it
sits **outside** the `/api/v1` auth gate so an in-cluster Prometheus can scrape
it without a session. Serving is gated on `METRICS_ENABLED` (default `true`; set
`METRICS_ENABLED=false` to omit the route entirely).

All application metric names are prefixed with `docz_api_`. The default registry
also carries the standard Go-runtime (`go_*`) and process (`process_*`)
collectors, so `/metrics` exposes those for free.

Cardinality is deliberately flat: the `route` label is the **chi route
template** (e.g. `/api/v1/repos`, `/api/v1/search`, `/api/v1/*`), never an
expanded path with ids, and `reason` is a small closed set.

### HTTP surface

| Metric                                   | Type      | Labels                      | Description                                                                             |
| ---------------------------------------- | --------- | --------------------------- | --------------------------------------------------------------------------------------- |
| `docz_api_http_requests_total`           | Counter   | `method`, `route`, `status` | Total HTTP requests by method, matched route, and status code.                          |
| `docz_api_http_request_duration_seconds` | Histogram | `method`, `route`           | Request duration (default Prometheus buckets). Exposes `_bucket{le}`, `_sum`, `_count`. |

The operational probes (`/healthz`, `/readyz`, `/metrics`) are **excluded** from
these instruments, so the series reflect real API traffic only.

Example:

```promql
# 5xx error rate (fraction of all requests)
sum(rate(docz_api_http_requests_total{status=~"5.."}[5m]))
  / sum(rate(docz_api_http_requests_total[5m]))

# p99 request latency (seconds)
histogram_quantile(0.99,
  sum by (le) (rate(docz_api_http_request_duration_seconds_bucket[5m])))

# request rate by route
sum by (route) (rate(docz_api_http_requests_total[1m]))
```

### Ingest pipeline

Ingest runs off the request path on an asynq + Redis queue. The worker records
one observation per completed job.

| Metric                                 | Type      | Labels             | Description                                                                                                               |
| -------------------------------------- | --------- | ------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| `docz_api_ingest_jobs_total`           | Counter   | `reason`, `status` | Ingest jobs processed. `reason` ∈ {`onboard`, `repo_added`, `push`}; `status` ∈ {`success`, `failure`}.                   |
| `docz_api_ingest_job_duration_seconds` | Histogram | `reason`           | Job duration with wide buckets (up to 120s — GitHub-bound jobs are wide-tailed). Exposes `_bucket{le}`, `_sum`, `_count`. |

Example:

```promql
# ingest failure rate by reason
sum by (reason) (rate(docz_api_ingest_jobs_total{status="failure"}[15m]))

# ingest success ratio
sum(rate(docz_api_ingest_jobs_total{status="success"}[15m]))
  / sum(rate(docz_api_ingest_jobs_total[15m]))

# p95 ingest duration by reason (seconds)
histogram_quantile(0.95,
  sum by (le, reason) (rate(docz_api_ingest_job_duration_seconds_bucket[30m])))
```

## Scraping

Point Prometheus at the service's `:8080/metrics`. A minimal static config:

```yaml
scrape_configs:
  - job_name: docz-api
    metrics_path: /metrics
    static_configs:
      - targets: ["docz-api:8080"]
```

Under the Prometheus Operator, the Helm chart ships a `ServiceMonitor`
(`serviceMonitor.enabled=true`), so you do not hand-write a scrape config.

## Importing the dashboard

`grafana/docz-api-dashboard.json` is import-style: it declares a `DS_PROMETHEUS`
input, so Grafana prompts for the Prometheus data source on import. It has no
Loki or Jaeger panels — it is Prometheus-only, for operators who run just a
metrics stack.

```bash
# Via the HTTP API (wrap in the import envelope, choosing your datasource uid)
curl -X POST -H "Content-Type: application/json" \
  -d "$(jq '{dashboard: ., overwrite: true, inputs: [{name: "DS_PROMETHEUS", type: "datasource", pluginId: "prometheus", value: "<your-prometheus-uid>"}]}' \
        contrib/grafana/docz-api-dashboard.json)" \
  https://grafana.example.com/api/dashboards/import
```

Or in the UI: **Dashboards → New → Import**, upload the JSON, and pick your
Prometheus data source when prompted.

## Applying the alerts

`prometheus/alerts.yaml` is the **same alert pack** the Helm chart renders as a
`PrometheusRule` — one file, two delivery paths:

- **Vanilla Prometheus:** add the file to `rule_files` (see the file's
  preamble), or paste the `docz-api` group into a `PrometheusRule` resource
  under the Prometheus Operator.
- **Helm chart:** set `prometheusRule.enabled=true` and the chart installs the
  same five alerts for you (skip this file entirely).

The file carries one extra alert, `DoczAPINoScrapes`
(`absent(up{job="docz-api"}) == 1`), that only makes sense with a static scrape
config owning the `job` label — adjust the `job` selector to match yours.

Validate any edits with:

```bash
promtool check rules contrib/prometheus/alerts.yaml   # or: just lint-alerts
```
