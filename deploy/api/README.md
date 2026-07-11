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

The service applies database migrations automatically on startup, so there is no
separate migration step.

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

## GitHub App setup (ingestion)

docz-api ingests repos as a **GitHub App**: install-driven onboarding, HMAC
webhooks, and content fetches over the Git Trees API with short-lived
installation tokens. Create one under _Settings → Developer settings → GitHub
Apps → New GitHub App_ (org or user account both work).

### App configuration

| Setting              | Value                                                            |
| -------------------- | ---------------------------------------------------------------- |
| Homepage URL         | Anything (e.g. this repo).                                       |
| Webhook URL          | `https://<your-host>/webhooks/github`                            |
| Webhook content type | `application/json`                                               |
| Webhook secret       | A strong random value — the same one as `GITHUB_WEBHOOK_SECRET`. |
| SSL verification     | Enabled.                                                         |
| Where installable    | "Only on this account" is fine for a homelab.                    |

The webhook receiver authenticates by HMAC-SHA256 over the raw body
(constant-time compare, fails closed), so the endpoint itself can be public; no
session or extra auth applies to `/webhooks/github`.

For **local development** the webhook URL can be an ngrok tunnel to your machine
— `just dev-tunnel` prints it; see
[DEVELOPMENT.md](../DEVELOPMENT.md#receiving-github-webhooks-locally-ngrok).

### Repository permissions

| Permission | Access    | Why                                                                                                          |
| ---------- | --------- | ------------------------------------------------------------------------------------------------------------ |
| Contents   | Read-only | Git refs/trees/blobs for `.docz.yaml`, docs, `CHANGELOG.md`, `index.md`; also gates the push/release events. |
| Metadata   | Read-only | Mandatory for every GitHub App (repo lookup, default branch).                                                |

Nothing else — no write access of any kind, no account permissions.

### Webhook events

Subscribe to:

- **Push** — a push to the repo's default branch that touches `.docz.yaml` or
  anything under `docs_dir/` triggers a full re-ingest (debounced; content-hash
  gated, so unchanged docs are no-ops). Pushes to other branches or unrelated
  paths are ignored.
- **Release** — received and logged only today; reserved for the future versions
  feature.

**Installation** and **Installation repositories** events are delivered to every
GitHub App automatically (no checkbox): installing the app or adding repos to an
installation onboards and enqueues an ingest per repo; uninstalling or removing
a repo offboards it (rows deleted, search index purged). A repo without a
`.docz.yaml` at HEAD fails its ingest and is logged — add the manifest and push
to onboard it.

### Keys and identifiers

After creating the app:

1. Note the **App ID** (the app's About page) → `GITHUB_APP_ID`.
2. **Generate a private key** (PEM) → save as `secrets/github-app.pem`;
   `GITHUB_APP_PRIVATE_KEY` takes the file path (the compose stack mounts it as
   a Docker secret at `/run/secrets/github_app_key`) or the PEM body itself.
3. Set `GITHUB_WEBHOOK_SECRET` to the webhook secret from above.
4. **Install the app** on the account and select the docz repos — installation
   is the onboarding; there is no separate registration step. The manual
   fallback for a missed installation event is
   `docz-api -onboard owner/name@<installationID>`.

For GitHub Enterprise, point `GITHUB_API_BASE` at your instance's API root;
everything else is unchanged.

### Not the same thing: the site-login OAuth app

Site users log in via a plain **OAuth app** (or Okta/Keycloak), not via the
GitHub App above. When `github` is in `AUTH_PROVIDERS`, create a separate OAuth
app (_Settings → Developer settings → OAuth Apps_) with the authorization
callback URL `<AUTH_REDIRECT_BASE>/auth/callback`, and set
`GITHUB_OAUTH_CLIENT_ID` / `GITHUB_OAUTH_CLIENT_SECRET`. The service requests
the `read:user` and `user:email` scopes and requires the account to have a
**primary, verified email**; no scopes are configured on the app itself.

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
- **Repo index backfill (DESIGN-0003):** the
  `/api/v1/repos/{owner}/{name}/index` endpoint serves the `docs_dir/index.md`
  cached at each repo's **last ingest**. Repos onboarded before this feature
  shipped return 404 until their next default-branch push touching `docs_dir/`
  (or `.docz.yaml`) re-ingests them — or run a manual
  `docz-api -onboard owner/name@installationID` per repo. No migration or
  backfill job is required; the docz-site's metadata fallback covers the gap.
- The images pin major/minor tags (`postgres:17-alpine`, `redis:7.4-alpine`,
  `getmeili/meilisearch:v1.12`); Renovate PRs updates.
- For Kubernetes, translate this to a Deployment (service) plus StatefulSets or
  managed equivalents for the three dependencies, HTTP liveness/readiness probes
  against `/healthz` and `/readyz`, and secrets from a `Secret`/CSI provider.
