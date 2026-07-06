# Deploying docz-api

A reference single-host deployment: the `docz-api` service plus Postgres, Redis,
and Meilisearch on a private Docker network. For local development of the
service itself, use the repo-root `compose.yaml` (dependencies only) with
`just run`.

## Layout

- `compose.yaml` — the full stack (service + three dependencies).
- `.env.production.example` — configuration template; copy to `.env.production`
  (gitignored) and fill in from your secret manager.
- `secrets/github-app.pem` — the GitHub App private key, mounted into the
  service as a Docker secret (gitignored; you create it).

## Bring-up

```sh
cd deploy
cp .env.production.example .env.production      # then fill in real values
mkdir -p secrets && cp /path/to/your/app.pem secrets/github-app.pem
docker compose up -d --build
docker compose ps
```

The service applies database migrations automatically on startup, so there is
no separate migration step.

## Configuration and secrets

All configuration is read from the environment. `compose.yaml` loads
`.env.production` as the service's env store; source that file from your secret
manager (SOPS, Vault, 1Password, ...) rather than committing it.

`compose.yaml` overrides the networking values (`DATABASE_URL`, `REDIS_URL`,
`MEILI_HOST`) so the service reaches its dependencies by their compose service
names, and delivers the GitHub App private key as a mounted secret file
(`GITHUB_APP_PRIVATE_KEY=/run/secrets/github_app_key`). Everything else —
webhook secret, session secret, OAuth/OIDC credentials, Meili key — comes from
`.env.production`.

## Health and observability

- **Liveness:** `GET /healthz` — process is up.
- **Readiness:** `GET /readyz` — Postgres, Redis, and Meilisearch are all
  reachable (503 with a per-dependency body otherwise). Point your
  orchestrator's probes here; the distroless image has no shell, so there is no
  in-container healthcheck for the service.
- **Metrics:** `GET /metrics` — Prometheus exposition (disable with
  `METRICS_ENABLED=false`). Scrape it on the internal network; it is not behind
  the auth gate.
- **Tracing:** set `OTEL_EXPORTER_OTLP_ENDPOINT` (host:port, OTLP/HTTP) to a
  collector to export traces; unset, tracing is a no-op.

## Notes

- The service listens on `:8080`; only that port is published. Postgres, Redis,
  and Meilisearch stay on the private network.
- The images pin major/minor tags (`postgres:17-alpine`, `redis:7.4-alpine`,
  `getmeili/meilisearch:v1.12`); Renovate PRs updates.
- For Kubernetes, translate this to a Deployment (service) plus StatefulSets or
  managed equivalents for the three dependencies, HTTP liveness/readiness probes
  against `/healthz` and `/readyz`, and secrets from a `Secret`/CSI provider.
