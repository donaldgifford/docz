# CLAUDE.md: `charts/docz`

Loaded when a session works under this directory. The repository's map is the root `CLAUDE.md`.

One chart deploys everything (IMPL-0021, DESIGN-0018): `api.*` and `site.*`
are the two workloads, and `store`, `queue`, and `search` are the API's
backends, unchanged from docz-api's chart. `auth`, `otel`, `metrics`,
`serviceMonitor`, `prometheusRule`, and `extraLabels` are shared at the top
level. `chart.just` drives it, and its README is the operator's guide.

- **Names and selectors.** Every object is `<fullname>-<role>` (api, site,
  postgres, postgres-pooler, valkey, meilisearch, test), and the role is
  `app.kubernetes.io/component`, which is in **every** selector, Deployment
  `matchLabels` included. It is safe there because the chart takes new
  installs only (DESIGN-0018 OQ 3c); there is no in-place upgrade from the old
  charts. The workload helpers take `dict "ctx" $ "component" "api"` and read
  `index .ctx.Values .component`, so `docz.hpa`, `docz.ingress`, and
  `docz.httpRoute` serve both workloads as one-line includes.
- **`extraLabels`** reaches metadata and pod templates, never a selector, a
  `volumeClaimTemplate`, or CNPG `inheritedMetadata`, and a key the chart sets
  fails the render.
- **Wiring.** `DOCZ_API_URL` derives from `docz.api.internalUrl` unless
  `site.config.doczApiUrl` is set. `auth.providers` feeds both `AUTH_PROVIDERS`
  and `DOCZ_AUTH_PROVIDERS`, except with `none`: docz-site's whitelist has no
  `none`, and its `/readyz` fails on it, so the site's variable is omitted.
  `otel.endpoint` is one collector URL rendered two ways, because the runtimes
  read it differently: `host:port` for the API's
  `otlptracehttp.WithEndpoint`, and the full `/v1/traces` URL for the site's
  exporter.
- **MCP.** `api.mcp.{enabled, issuer, siteUrl, trustedIssuers, rateLimit}`
  and `api.authz.{scopeMap, cacheTtl}` (IMPL-0026, DESIGN-0023) render
  `MCP_ENABLED`, `MCP_RATE_LIMIT`, `OAUTH_ISSUER`, `DOCZ_SITE_URL`,
  `OAUTH_TRUSTED_ISSUERS`, `OAUTH_SIGNING_KEY`, `AUTHZ_SCOPE_MAP`, and
  `AUTHZ_CACHE_TTL` **only while `enabled`**, the authz block included. The
  issuer defaults to `https://` on the API's first ingress host or HTTPRoute
  hostname (`docz.api.externalUrl` over `docz.edgeHost`) and the site URL to
  the site's (`docz.site.externalUrl`); with neither set the render fails
  naming the value. `DOCZ_SITE_URL` is omitted under `auth.providers: none`,
  where nobody can consent. The Secret's `oauth-signing-key` is `required`
  while enabled, through the `existingSecret` seam, and the scope map is
  rendered with sorted keys (`docz.api.scopeMap`) so the value is stable.
  `DoczAPITokenFailures` is the seventh alert in the PrometheusRule and
  `contrib/prometheus/alerts.yaml`; `tests/api/prometheusrule_test.yaml`
  indexes rules by position, so a new alert goes last. RUNBOOK-0004 is the
  operator's path.
- **Edges.** There is one Ingress and one HTTPRoute per workload, each routed
  to its own Service. No Tailscale is in the chart:
  `deploy/tailscale-operator.md` is the only Tailscale documentation, and a
  single shared route is a follow-up (DESIGN-0018 Rollout step 7).
- **Tests.** `tests/api/` and `tests/site/` are the old charts' suites ported;
  `tests/chart/` covers what only a merged chart has (wiring, selectors,
  edges, extraLabels, no Tailscale, versions). helm-unittest runs with
  `-f 'tests/**/*_test.yaml'`. The `.helmignore` anchors `/tests/` and `/ci/`,
  because an unanchored `tests/` also drops `templates/tests/`: the old
  charts never packaged their `helm test` hook. The ci-values therefore run
  busybox `httpd` serving `/healthz`, since `ct install` now runs the hook.
